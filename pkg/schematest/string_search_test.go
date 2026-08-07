//nolint:godoclint,lll,paralleltest,wsl_v5 // These focused cases pin the private string-search seam.
package schematest

import (
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestBuildUsesOneProductForSimultaneousStringPatterns(t *testing.T) {
	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"allOf":[
			{"pattern":"^(?=ab)a"},
			{"pattern":"^a.$"}
		]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.NotEmpty(t, cases)
	require.Equal(t, `"ab"`, string(cases[0].JSON))
	require.NotEmpty(t, report.Covered)
}

func TestBuildAllowsAnchoredPatternPrefixWithSiblingLength(t *testing.T) {
	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"allOf":[{"pattern":"^a"},{"pattern":"^ab$"}]
	}`))

	var cases []Case
	_, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000}, func(testCase Case) error {
		cases = append(cases, testCase)

		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, cases)
	require.Equal(t, `"ab"`, string(cases[0].JSON))
}

func TestBuildDoesNotPinUnanchoredPatternLength(t *testing.T) {
	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"allOf":[{"pattern":"a$"},{"pattern":"^ba$"}]
	}`))

	var cases []Case
	_, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000}, func(testCase Case) error {
		cases = append(cases, testCase)

		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, cases)
	require.Equal(t, `"ba"`, string(cases[0].JSON))
}

func TestBuildContradictoryFiniteStringProductExhausts(t *testing.T) {
	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"allOf":[{"pattern":"^a$"},{"pattern":"^b*$"}]
	}`))

	var cases []Case
	report, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000}, func(testCase Case) error {
		cases = append(cases, testCase)

		return nil
	})

	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Empty(t, cases)
}

func TestBuildContradictoryFiniteLeadingAssertionProductExhausts(t *testing.T) {
	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"allOf":[{"pattern":"^(?=a$).*"},{"pattern":"^b*$"}]
	}`))

	var cases []Case
	report, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1_000}, func(testCase Case) error {
		cases = append(cases, testCase)

		return nil
	})
	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Empty(t, cases)
}

func TestBuildPreservesPinnedFalseAnyOfStringBranches(t *testing.T) {
	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"anyOf":[{"pattern":"^[ac-z]$"},{"pattern":"^[a-z]$"}]
	}`))

	var cases []Case
	report, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000}, func(testCase Case) error {
		cases = append(cases, testCase)

		return nil
	})

	require.NoError(t, err)
	require.Contains(t, cases, Case{JSON: []byte(`"b"`), Valid: true})
	require.Contains(t, report.Covered, "#/paths/~1/post/requestBody/content/application~1json/schema|#|anyOf|level:mask:2")
}

func TestBuildReachesEmailAddressLiteralDelimiters(t *testing.T) {
	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"format":"email",
		"minLength":7,
		"maxLength":7,
		"pattern":"^a@\\[[ -~]{4}$"
	}`))

	var cases []Case
	_, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000}, func(testCase Case) error {
		cases = append(cases, testCase)

		return nil
	})

	require.NoError(t, err)
	require.NotEmpty(t, cases)
	value, err := parseStrictJSON(cases[0].JSON)
	require.NoError(t, err)
	require.Equal(t, 7, len([]rune(value.text)))
	model, err := parseInput(Input{OpenAPI: document, OperationID: "selected"})
	require.NoError(t, err)
	result := evaluate(model, value)
	require.NoError(t, result.err)
	require.True(t, result.valid)
	formatMatched, err := cleanStringFormatMatches(value.text, schemaFormatEmail)
	require.NoError(t, err)
	require.True(t, formatMatched)
}

func TestStringProductPinnedFalseRulesFindMaskTwoWitnesses(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		want   string
		length int
	}{
		{
			name:   "length and pattern",
			schema: `{"type":"string","anyOf":[{"minLength":2},{"pattern":"^a$"}]}`,
			want:   "a",
			length: 1,
		},
		{
			name:   "sibling lengths",
			schema: `{"type":"string","anyOf":[{"minLength":2},{"minLength":1}]}`,
			length: 1,
		},
		{
			name:   "retained format",
			schema: `{"type":"string","anyOf":[{"format":"email"},{"format":"date"}]}`,
			want:   "0000-01-01",
			length: 10,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model, err := parseInput(Input{
				OpenAPI:     []byte(documentWithJSONSchema(test.schema)),
				OperationID: "selected",
			})
			require.NoError(t, err)

			pins := anyOfMaskPins(model.root.occurrence, 2, big.NewInt(2))
			product, err := buildStringProduct(model.root, model.root.occurrence, pins)
			require.NoError(t, err)
			state := &search{model: model, maxSteps: 10000}
			var value *jsonValue
			complete, err := state.searchStringObjective(
				product,
				firstStringAllTrueObjective(t, product, oracleRuleAnyOf, planLevelMask+"2"),
				func(candidate *jsonValue) (bool, error) {
					value = candidate

					return true, nil
				},
			)
			require.NoError(t, err)
			require.True(t, complete)
			require.NotNil(t, value)
			if test.want != "" {
				require.Equal(t, test.want, value.text)
			}
			require.Equal(t, test.length, len([]rune(value.text)))

			result := evaluate(model, value)
			require.NoError(t, result.err)
			require.True(t, result.valid)
			require.True(t, compositionLevelWasObserved(result, levelIdentity{
				ruleIdentity: makeRuleIdentity(model.root.occurrence, oracleRuleAnyOf),
				level:        planLevelMask + "2",
			}))
		})
	}
}

func TestBuildPreservesNestedAnyOfTruthStructure(t *testing.T) {
	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"anyOf":[
			{"anyOf":[{"pattern":"^a$"},{"pattern":"^b$"}]},
			{"pattern":"^c$"}
		]
	}`))

	var cases []Case
	report, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10_000}, func(testCase Case) error {
		cases = append(cases, testCase)

		return nil
	})

	require.NoError(t, err)
	require.Contains(t, report.Covered, "#/paths/~1/post/requestBody/content/application~1json/schema|#|anyOf|level:mask:1")
	require.Contains(t, report.Covered, "#/paths/~1/post/requestBody/content/application~1json/schema|#|anyOf|level:mask:2")
	require.True(t, containsCaseJSON(cases, `"a"`) || containsCaseJSON(cases, `"b"`))
	require.True(t, containsCaseJSON(cases, `"c"`))
}

func containsCaseJSON(cases []Case, wanted string) bool {
	for _, testCase := range cases {
		if string(testCase.JSON) == wanted {
			return true
		}
	}

	return false
}

func TestBuildAnyOfConjunctiveFalseBranchUsesOneFailure(t *testing.T) {
	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"anyOf":[{"allOf":[{"pattern":"^a$"},{"pattern":"^b$"}]},{"pattern":"^a$"}]
	}`))

	var cases []Case
	report, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000}, func(testCase Case) error {
		cases = append(cases, testCase)

		return nil
	})

	require.NoError(t, err)
	require.Contains(t, cases, Case{JSON: []byte(`"a"`), Valid: true})
	require.Contains(t, report.Covered, "#/paths/~1/post/requestBody/content/application~1json/schema|#|anyOf|level:mask:2")
}

func TestBuildReportsMaxStepsAfterStringObjectives(t *testing.T) {
	var cases []Case
	report, err := Build(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"type":"string","pattern":"^a$"}`)),
		OperationID: "selected",
		MaxSteps:    6,
	}, func(testCase Case) error {
		cases = append(cases, testCase)

		return nil
	})

	require.NoError(t, err)
	require.Equal(t, MaxStepsReached, report.Stop)
	require.Equal(t, uint64(6), report.Steps)
}

func TestBuildEmailAddressLiteralAllowsLeadingZeroIPv4Octets(t *testing.T) {
	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"format":"email",
		"pattern":"^a@\\[001\\.002\\.003\\.004\\]$"
	}`))

	var cases []Case
	_, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000}, func(testCase Case) error {
		cases = append(cases, testCase)

		return nil
	})

	require.NoError(t, err)
	require.Contains(t, cases, Case{JSON: []byte(`"a@[001.002.003.004]"`), Valid: true})
}

func TestBuildPreservesComposedStringEnumValues(t *testing.T) {
	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"minLength":1,
		"anyOf":[{"enum":["unique-enum-value"]}]
	}`))

	var cases []Case
	report, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000}, func(testCase Case) error {
		cases = append(cases, testCase)

		return nil
	})

	require.NoError(t, err)
	require.Contains(t, cases, Case{JSON: []byte(`"unique-enum-value"`), Valid: true})
	require.Contains(t, report.Covered, "#/paths/~1/post/requestBody/content/application~1json/schema|#|anyOf|level:mask:1")
	require.Contains(t, report.Covered, "#/paths/~1/post/requestBody/content/application~1json/schema/anyOf/0|#|enum|level:member:0")
}

func TestBuildUsesProductStringPatternWithDateFormat(t *testing.T) {
	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"format":"date",
		"pattern":"^2024-02-29$"
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.NotEmpty(t, cases)
	require.Equal(t, `"2024-02-29"`, string(cases[0].JSON))
}

func TestStringProductObjectivesKeepDirectedSiblingsTrue(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"string",
			"allOf":[{"pattern":"^(?=a)a$"},{"pattern":"^b$"}]
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 100}
	var objectives []stringObjective
	var values []string
	err = searchState.searchStringObjectives(product, "pattern", "valid", func(objective stringObjective, value *jsonValue) (bool, error) {
		objectives = append(objectives, objective)
		values = append(values, value.text)

		return false, nil
	})
	require.NoError(t, err)
	require.Equal(t, []stringObjectiveKind{stringObjectivePatternFalse, stringObjectivePatternFalse}, objectiveKinds(objectives))
	require.Equal(t, []string{"b", "a"}, values)
}

func TestStringProductDirectedPatternUsesTargetIntervalsForFalseWitnesses(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"string",
			"allOf":[
				{"pattern":"^(?=[a-m]|[o-z])(?:[a-m]|[o-z])$"},
				{"pattern":"^[a-z]$"}
			]
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		mustStringDirectedObjective(t, product, stringObjectivePatternFalse, 0),
		func(value *jsonValue) (bool, error) {
			values = append(values, value.text)

			return true, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.Equal(t, []string{"n"}, values)

	result := evaluate(model, &jsonValue{kind: jsonString, text: values[0]})
	require.NoError(t, result.err)
	require.False(t, result.valid)
	require.Equal(t, []string{product.patterns[0].identity.String()}, identityStrings(result.failures))
}

func TestStringProductDirectedLengthObjectivesKeepStringClosure(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"string",
			"allOf":[
				{"minLength":2,"maxLength":3,"pattern":"^a{0,4}$"},
				{"minLength":1,"maxLength":4,"pattern":"^a+$"}
			]
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	for _, test := range []struct {
		name      string
		objective stringObjective
		want      string
	}{
		{
			name:      "minimum",
			objective: mustStringDirectedObjective(t, product, stringObjectiveLengthFalse, 0),
			want:      "a",
		},
		{
			name:      "maximum",
			objective: mustStringDirectedObjective(t, product, stringObjectiveLengthFalse, 1),
			want:      "aaaa",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			searchState := &search{model: model, maxSteps: 1_000}
			var values []string
			complete, searchErr := searchState.searchStringObjective(
				product,
				test.objective,
				func(value *jsonValue) (bool, error) {
					values = append(values, value.text)

					return true, nil
				},
			)
			require.NoError(t, searchErr)
			require.True(t, complete)
			require.Equal(t, []string{test.want}, values)

			result := evaluate(model, &jsonValue{kind: jsonString, text: values[0]})
			require.NoError(t, result.err)
			require.False(t, result.valid)
			require.Equal(t, []string{product.lengths[test.objective.index].identity.String()}, identityStrings(result.failures))
		})
	}
}

func TestStringProductDirectedPatternUsesEnumMember(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"string",
			"enum":["a","b"],
			"maxLength":1,
			"pattern":"^a$"
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)
	state := &search{model: model, maxSteps: 100}
	var value string
	complete, err := state.searchStringObjective(
		product,
		mustStringDirectedObjective(t, product, stringObjectivePatternFalse, 0),
		func(candidate *jsonValue) (bool, error) {
			value = candidate.text

			return true, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.Equal(t, "b", value)
}

func TestStringProductDirectedPatternRequiresExactCleanFailureClosure(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"string",
			"enum":["a"],
			"maxLength":1,
			"pattern":"^a$"
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 100}
	complete, err := searchState.searchStringObjective(
		product,
		mustStringDirectedObjective(t, product, stringObjectivePatternFalse, 0),
		func(*jsonValue) (bool, error) { return true, nil },
	)
	require.NoError(t, err)
	require.False(t, complete)
}

func TestStringProductUnsatisfiableDirectedPatternStaysUncovered(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"string",
			"allOf":[{"pattern":"^a+$"},{"pattern":"^aa$"}]
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		mustStringDirectedObjective(t, product, stringObjectivePatternFalse, 0),
		func(value *jsonValue) (bool, error) {
			values = append(values, value.text)

			return true, nil
		},
	)
	require.NoError(t, err)
	require.False(t, complete)
	require.Empty(t, values)
}

func TestStringProductUnsatisfiableDirectedLengthStaysUncovered(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"string",
			"minLength":2,
			"maxLength":2,
			"pattern":"^a{2}$"
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		mustStringDirectedObjective(t, product, stringObjectiveLengthFalse, 0),
		func(value *jsonValue) (bool, error) {
			values = append(values, value.text)

			return true, nil
		},
	)
	require.NoError(t, err)
	require.False(t, complete)
	require.Empty(t, values)
}

func TestStringProductDirectedPatternRetriesUseGlobalAssignments(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"string",
			"allOf":[{"pattern":"^a$"},{"pattern":"^a{2,3}$"}]
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 5}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		mustStringDirectedObjective(t, product, stringObjectivePatternFalse, 0),
		func(value *jsonValue) (bool, error) {
			values = append(values, value.text)

			return false, nil
		},
	)
	require.False(t, complete)
	require.ErrorIs(t, err, errMaxSteps)
	require.Equal(t, uint64(5), searchState.steps)
	require.NotEmpty(t, values)
}

func TestBuildStringFormatWitnessesUseCleanLanguages(t *testing.T) {
	for _, format := range []schemaFormat{
		schemaFormatByte,
		schemaFormatDate,
		schemaFormatDateTime,
		schemaFormatEmail,
		schemaFormatIPv4,
		schemaFormatUUID,
		schemaFormatUUIDv4,
		schemaFormatUUIDDashV4,
		schemaFormatCIDR,
		schemaFormatIPv4CIDR,
	} {
		t.Run(formatName(format), func(t *testing.T) {
			t.Parallel()
			document := []byte(documentWithJSONSchema(`{"type":"string","format":"` + formatName(format) + `"}`))
			var cases []Case
			_, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000}, func(testCase Case) error {
				cases = append(cases, testCase)

				return nil
			})
			require.NoError(t, err)
			require.NotEmpty(t, cases)

			value, err := parseStrictJSON(cases[0].JSON)
			require.NoError(t, err)
			matched, err := cleanStringFormatMatches(value.text, format)
			require.NoError(t, err)
			require.True(t, matched)
		})
	}
}

//nolint:cyclop // The test table spells out every admitted format alias.
func formatName(format schemaFormat) string {
	switch format {
	case schemaFormatByte:
		return "byte"
	case schemaFormatDate:
		return "date"
	case schemaFormatDateTime:
		return "date-time"
	case schemaFormatEmail:
		return "email"
	case schemaFormatIPv4:
		return "ipv4"
	case schemaFormatUUID:
		return "uuid"
	case schemaFormatUUIDv4:
		return "uuidv4"
	case schemaFormatUUIDDashV4:
		return "uuid-v4"
	case schemaFormatCIDR:
		return "cidr"
	case schemaFormatIPv4CIDR:
		return "ipv4-cidr"
	default:
		return ""
	}
}

func TestStringProductLeadingAssertionsConstrainWitnesses(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"type":"string","pattern":"^(?=ab)(?!ac)a"}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)
	require.Equal(t, 22, model.root.pattern.matcherBytes)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)
	require.Len(t, product.patterns, 1)
	require.Len(t, product.patterns[0].assertions, 2)

	searchState := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringAllTrueObjective(product, "pattern", "valid"),
		func(value *jsonValue) (bool, error) {
			values = append(values, value.text)

			return true, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.Equal(t, []string{"ab"}, values)
	require.Equal(t, uint64(6), searchState.steps)
}

func TestStringProductLeadingAssertionsUseTheNextUTF16Unit(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(
			`{"type":"string","minLength":2,"pattern":"^(?=a\\b)a"}`,
		)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringAllTrueObjective(product, "pattern", "valid"),
		func(value *jsonValue) (bool, error) {
			values = append(values, value.text)

			return true, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.Equal(t, []string{"a\x00"}, values)
}

func TestStringProductLeadingAssertionsStopContradictionsAtTheGlobalBudget(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"type":"string","pattern":"^(?=a)(?!a)a"}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 8}
	complete, err := searchState.searchStringObjective(
		product,
		stringAllTrueObjective(product, "pattern", "valid"),
		func(*jsonValue) (bool, error) {
			t.Fatal("contradictory assertions produced a witness")

			return false, nil
		},
	)
	require.False(t, complete)
	require.ErrorIs(t, err, errMaxSteps)
	require.Equal(t, uint64(8), searchState.steps)
}

func TestCleanRetainedStringFormatBoundaries(t *testing.T) {
	emailTotalValid := "a@" + strings.Repeat("b", 63) + "." +
		strings.Repeat("c", 63) + "." + strings.Repeat("d", 63) + "." + strings.Repeat("e", 60)
	tests := []struct {
		name    string
		format  schemaFormat
		valid   string
		invalid string
	}{
		{
			name:    "email local part limit",
			format:  schemaFormatEmail,
			valid:   strings.Repeat("a", 64) + "@x",
			invalid: strings.Repeat("a", 65) + "@x",
		},
		{
			name:    "email total limit",
			format:  schemaFormatEmail,
			valid:   emailTotalValid,
			invalid: emailTotalValid + "e",
		},
		{
			name:    "ipv4 octet boundaries",
			format:  schemaFormatIPv4,
			valid:   "255.255.255.255",
			invalid: "256.255.255.255",
		},
		{
			name:    "ipv4 leading zero",
			format:  schemaFormatIPv4,
			valid:   "0.0.0.0",
			invalid: "00.0.0.0",
		},
		{
			name:    "cidr prefix boundaries",
			format:  schemaFormatCIDR,
			valid:   "255.255.255.255/32",
			invalid: "255.255.255.255/33",
		},
		{
			name:    "cidr preserves host bits",
			format:  schemaFormatIPv4CIDR,
			valid:   "192.0.2.7/24",
			invalid: "192.0.2.07/24",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matched, err := cleanStringFormatMatches(test.valid, test.format)
			require.NoError(t, err)
			require.True(t, matched)

			matched, err = cleanStringFormatMatches(test.invalid, test.format)
			require.NoError(t, err)
			require.False(t, matched)
		})
	}
}

func TestStringFormatProductGoldenWitnesses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		want   string
		steps  uint64
	}{
		{
			name:   "byte",
			schema: `{"type":"string","format":"byte","pattern":"^YQ==$"}`,
			want:   "YQ==",
			steps:  5,
		},
		{
			name:   "date",
			schema: `{"type":"string","format":"date","pattern":"^2024-02-29$"}`,
			want:   "2024-02-29",
			steps:  11,
		},
		{
			name:   "date-time fraction and Z",
			schema: `{"type":"string","format":"date-time","pattern":"^2024-02-29T12:34:56\\.123Z$"}`,
			want:   "2024-02-29T12:34:56.123Z",
			steps:  46,
		},
		{
			name:   "date-time fraction and offset",
			schema: `{"type":"string","format":"date-time","pattern":"^2024-02-29T12:34:56\\.123\\+02:30$"}`,
			want:   "2024-02-29T12:34:56.123+02:30",
			steps:  51,
		},
		{
			name:   "date-time offset",
			schema: `{"type":"string","format":"date-time","pattern":"^2024-02-29T12:34:56\\+02:30$"}`,
			want:   "2024-02-29T12:34:56+02:30",
			steps:  47,
		},
		{
			name:   "uuid version four",
			schema: `{"type":"string","format":"uuid","pattern":"^00000000-0000-4000-8000-000000000000$"}`,
			want:   "00000000-0000-4000-8000-000000000000",
			steps:  37,
		},
		{
			name:   "email shortest mailbox",
			schema: `{"type":"string","format":"email","pattern":"^a@b$"}`,
			want:   "a@b",
			steps:  4,
		},
		{
			name:   "ipv4 lower boundary",
			schema: `{"type":"string","format":"ipv4","pattern":"^0\\.0\\.0\\.0$"}`,
			want:   "0.0.0.0",
			steps:  8,
		},
		{
			name:   "ipv4 upper boundary",
			schema: `{"type":"string","format":"ipv4","pattern":"^255\\.255\\.255\\.255$"}`,
			want:   "255.255.255.255",
			steps:  24,
		},
		{
			name:   "cidr lower prefix",
			schema: `{"type":"string","format":"cidr","pattern":"^0\\.0\\.0\\.0/0$"}`,
			want:   "0.0.0.0/0",
			steps:  10,
		},
		{
			name:   "cidr upper prefix preserves host bits",
			schema: `{"type":"string","format":"ipv4-cidr","pattern":"^255\\.255\\.255\\.255/32$"}`,
			want:   "255.255.255.255/32",
			steps:  29,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model, err := parseInput(Input{
				OpenAPI:     []byte(documentWithJSONSchema(test.schema)),
				OperationID: "selected",
			})
			require.NoError(t, err)

			product, err := buildStringProduct(model.root, model.root.occurrence, nil)
			require.NoError(t, err)

			state := &search{model: model, maxSteps: 100}
			var values []string
			complete, err := state.searchStringObjective(
				product,
				stringAllTrueObjective(product, "format", "valid"),
				func(value *jsonValue) (bool, error) {
					values = append(values, value.text)

					return true, nil
				},
			)
			require.NoError(t, err)
			require.True(t, complete)
			require.Equal(t, []string{test.want}, values)
			require.Equal(t, test.steps, state.steps)
		})
	}
}

func TestStringFormatNegativeKeepsStringSiblingsGolden(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(
			`{"type":"string","format":"byte","minLength":2,"pattern":"^.{2,4}$"}`,
		)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)
	require.Equal(
		t,
		[]stringObjectiveKind{
			stringObjectiveAllTrue,
			stringObjectivePatternFalse,
			stringObjectiveFormatFalse,
			stringObjectiveLengthFalse,
		},
		stringScheduleKinds(t, product),
	)

	state := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := state.searchStringObjective(
		product,
		mustStringDirectedObjective(t, product, stringObjectiveFormatFalse, 0),
		func(value *jsonValue) (bool, error) {
			values = append(values, value.text)

			return true, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.Equal(t, []string{"\x00\x00"}, values)
	require.Equal(t, uint64(3), state.steps)

	result := evaluate(model, &jsonValue{kind: jsonString, text: values[0]})
	require.NoError(t, result.err)
	require.False(t, result.valid)
	require.Equal(t, []string{"format"}, failureRules(result.failures))
}

func TestStringProductRetainedFormatNegativesKeepPatternAndLengths(t *testing.T) {
	for _, format := range []schemaFormat{schemaFormatEmail, schemaFormatIPv4, schemaFormatCIDR} {
		t.Run(formatName(format), func(t *testing.T) {
			model, err := parseInput(Input{
				OpenAPI: []byte(documentWithJSONSchema(
					`{"type":"string","format":` + strconv.Quote(formatName(format)) + `,"minLength":2,"maxLength":2,"pattern":"^..$"}`,
				)),
				OperationID: "selected",
			})
			require.NoError(t, err)

			product, err := buildStringProduct(model.root, model.root.occurrence, nil)
			require.NoError(t, err)

			state := &search{model: model, maxSteps: 100}
			var values []string
			complete, err := state.searchStringObjective(
				product,
				mustStringDirectedObjective(t, product, stringObjectiveFormatFalse, 0),
				func(value *jsonValue) (bool, error) {
					values = append(values, value.text)

					return true, nil
				},
			)
			require.NoError(t, err)
			require.True(t, complete)
			require.Len(t, values, 1)
			require.Len(t, []rune(values[0]), 2)
			matched, formatErr := cleanStringFormatMatches(values[0], format)
			require.NoError(t, formatErr)
			require.False(t, matched)

			result := evaluate(model, &jsonValue{kind: jsonString, text: values[0]})
			require.NoError(t, result.err)
			require.False(t, result.valid)
			require.Equal(t, []string{"format"}, failureRules(result.failures))
		})
	}
}

func TestPasswordFormatDoesNotEnterStringProduct(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"type":"string","format":"password"}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)
	require.Empty(t, product.formats)
	require.Equal(
		t,
		[]stringObjectiveKind{stringObjectiveAllTrue},
		stringScheduleKinds(t, product),
	)

	state := &search{model: model, maxSteps: 10}
	var values []string
	complete, err := state.searchStringObjective(
		product,
		stringAllTrueObjective(product, "format", "valid"),
		func(value *jsonValue) (bool, error) {
			values = append(values, value.text)

			return true, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.Equal(t, []string{""}, values)
	require.Equal(t, uint64(1), state.steps)
}

func TestStringProductObjectivesPreserveFormatAndPatternSiblings(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"type":"string","pattern":"^a$","format":"date"}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1000}
	var objectives []stringObjective
	var values []string
	err = searchState.searchStringObjectives(product, "pattern", "valid", func(objective stringObjective, value *jsonValue) (bool, error) {
		objectives = append(objectives, objective)
		values = append(values, value.text)

		return true, nil
	})
	require.NoError(t, err)
	require.Equal(t, []stringObjectiveKind{stringObjectivePatternFalse, stringObjectiveFormatFalse}, objectiveKinds(objectives))
	require.Equal(t, "a", values[1])
	require.True(t, cleanDateFormatMatches(values[0]))
}

func TestStringBoundedQuantifiersUseFiniteProductFrontiers(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"type":"string","pattern":"^a{2,3}$"}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)
	require.Len(t, product.patterns, 1)
	require.True(t, product.patterns[0].finite)
	require.Equal(t, uint64(3), product.patterns[0].maximum)

	searchState := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringAllTrueObjective(product, "pattern", "valid"),
		func(value *jsonValue) (bool, error) {
			values = append(values, value.text)

			return false, nil
		},
	)
	require.NoError(t, err)
	require.False(t, complete)
	require.Equal(t, []string{"aa", "aaa"}, values)
	require.Equal(t, uint64(10), searchState.steps)
}

func TestStringBoundedQuantifierContradictionEndsAtPatternMaximum(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"type":"string","minLength":4,"pattern":"^a{2,3}$"}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringAllTrueObjective(product, "pattern", "valid"),
		func(value *jsonValue) (bool, error) {
			values = append(values, value.text)

			return false, nil
		},
	)
	require.NoError(t, err)
	require.False(t, complete)
	require.Empty(t, values)
	require.Equal(t, uint64(4), searchState.steps)
}

func TestStringQuantifierEdgesUseTheGlobalAssignmentBudget(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"type":"string","pattern":"^a{2,3}$"}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 8}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringAllTrueObjective(product, "pattern", "valid"),
		func(value *jsonValue) (bool, error) {
			values = append(values, value.text)

			return false, nil
		},
	)
	require.False(t, complete)
	require.ErrorIs(t, err, errMaxSteps)
	require.Equal(t, []string{"aa"}, values)
	require.Equal(t, uint64(8), searchState.steps)
}

func TestBuildSearchesS3QuantifierAnchorBoundaryAndWhitespaceLanguages(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    string
	}{
		{name: "greedy counted", pattern: `^a{1,2}$`, want: "a"},
		{name: "lazy unbounded", pattern: `^a+?$`, want: "a"},
		{name: "absolute anchors", pattern: `^a$`, want: "a"},
		{name: "word boundary", pattern: `^\bA$`, want: "A"},
		{name: "non-word boundary", pattern: `^\B\W$`, want: "\x00"},
		{name: "ES whitespace", pattern: `^\s$`, want: "\t"},
		{name: "ES non-whitespace", pattern: `^\S$`, want: "\x00"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var cases []Case
			_, err := Build(
				Input{
					OpenAPI:     []byte(documentWithJSONSchema(`{"type":"string","pattern":` + strconv.Quote(test.pattern) + `}`)),
					OperationID: "selected",
					MaxSteps:    1_000,
				},
				func(testCase Case) error {
					cases = append(cases, testCase)

					return nil
				},
			)
			require.NoError(t, err)
			require.NotEmpty(t, cases)

			value, err := parseStrictJSON(cases[0].JSON)
			require.NoError(t, err)
			require.Equal(t, test.want, value.text)
		})
	}
}

func TestStringProductObjectivesUseLengthBoundaries(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"type":"string","minLength":2,"maxLength":3}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1000}
	var values []string
	err = searchState.searchStringObjectives(product, "minLength", "valid", func(_ stringObjective, value *jsonValue) (bool, error) {
		values = append(values, value.text)

		return true, nil
	})
	require.NoError(t, err)
	require.Len(t, values, 3)
	require.Len(t, []rune(values[0]), 2)
	require.Len(t, []rune(values[1]), 1)
	require.Len(t, []rune(values[2]), 4)
}

func TestStringProductTriesRelevantLengthBoundariesBeforeFairLengths(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"string",
			"minLength":2,
			"maxLength":100,
			"pattern":"^a{2,100}$"
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1000}
	var lengths []int
	complete, err := searchState.searchStringObjective(
		product,
		stringAllTrueObjective(product, "minLength", "valid"),
		func(value *jsonValue) (bool, error) {
			length := len([]rune(value.text))
			if len(lengths) == 0 || lengths[len(lengths)-1] != length {
				lengths = append(lengths, length)
			}

			return len(lengths) == 3, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.Equal(t, []int{2, 100, 3}, lengths)
}

func TestStringProductDirectedFormatFalseIgnoresItsFiniteMaximum(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"string",
			"format":"date",
			"pattern":"^00000000000$"
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1000}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		mustStringDirectedObjective(t, product, stringObjectiveFormatFalse, 0),
		func(value *jsonValue) (bool, error) {
			values = append(values, value.text)

			return true, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.Len(t, values, 1)
	require.Len(t, []rune(values[0]), 11)
	require.False(t, cleanDateFormatMatches(values[0]))
}

func TestStringProductEmailPrefixKeepsDottedLocalPartsReachable(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"string",
			"format":"email",
			"pattern":"^a\\.b@c$"
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1000}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringAllTrueObjective(product, "format", "valid"),
		func(value *jsonValue) (bool, error) {
			values = append(values, value.text)

			return true, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.Equal(t, []string{"a.b@c"}, values)
}

func TestStringProductEmailEscapedQuotesKeepTheSMTPWitnessReachable(t *testing.T) {
	value := `"a b\"c"@example.com`
	pattern := `^"a b\\"c"@example\.com$`
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(
			`{"type":"string","format":"email","pattern":` + strconv.Quote(pattern) + `}`,
		)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)

	state := &search{model: model, maxSteps: 100_000}
	var values []string
	complete, err := state.searchStringObjective(
		product,
		stringAllTrueObjective(product, "format", "valid"),
		func(candidate *jsonValue) (bool, error) {
			values = append(values, candidate.text)

			return true, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.Equal(t, []string{value}, values)
}

func TestStringProductIPv4PrefixKeepsCompletableOctetsReachable(t *testing.T) {
	for _, test := range []struct {
		format  string
		pattern string
		want    string
	}{
		{format: "ipv4", pattern: `^255\.255\.255\.255$`, want: "255.255.255.255"},
		{format: "cidr", pattern: `^255\.255\.255\.255/32$`, want: "255.255.255.255/32"},
	} {
		t.Run(test.format, func(t *testing.T) {
			model, err := parseInput(Input{
				OpenAPI: []byte(documentWithJSONSchema(
					`{"type":"string","format":` + strconv.Quote(test.format) + `,"pattern":` + strconv.Quote(test.pattern) + `}`,
				)),
				OperationID: "selected",
			})
			require.NoError(t, err)

			product, err := buildStringProduct(model.root, model.root.occurrence, nil)
			require.NoError(t, err)

			searchState := &search{model: model, maxSteps: 1000}
			var values []string
			complete, err := searchState.searchStringObjective(
				product,
				stringAllTrueObjective(product, "format", "valid"),
				func(value *jsonValue) (bool, error) {
					values = append(values, value.text)

					return true, nil
				},
			)
			require.NoError(t, err)
			require.True(t, complete)
			require.Equal(t, []string{test.want}, values)
		})
	}
}

func TestStringObjectiveSeedUsesOwningSchemaOccurrence(t *testing.T) {
	buildObjectives := func(siblingPattern string) (stringObjective, stringObjective) {
		model, err := parseInput(Input{
			OpenAPI: []byte(documentWithJSONSchema(`{
				"type":"string",
				"allOf":[{"pattern":"^a$"},{"pattern":` + strconv.Quote(siblingPattern) + `}]
			}`)),
			OperationID: "selected",
		})
		require.NoError(t, err)

		product, err := buildStringProduct(model.root, model.root.occurrence, nil)
		require.NoError(t, err)

		return mustStringDirectedObjective(t, product, stringObjectivePatternFalse, 0),
			mustStringDirectedObjective(t, product, stringObjectivePatternFalse, 1)
	}

	firstLeft, firstRight := buildObjectives("^b$")
	changedLeft, changedRight := buildObjectives("^c$")
	firstLeftSeed, err := stringSearchSeed(firstLeft)
	require.NoError(t, err)
	firstRightSeed, err := stringSearchSeed(firstRight)
	require.NoError(t, err)
	changedLeftSeed, err := stringSearchSeed(changedLeft)
	require.NoError(t, err)
	changedRightSeed, err := stringSearchSeed(changedRight)
	require.NoError(t, err)

	require.NotEqual(t, firstLeft.owner.occurrence.usePointer, firstRight.owner.occurrence.usePointer)
	require.Equal(t, firstLeftSeed, changedLeftSeed)
	require.NotEqual(t, firstRightSeed, changedRightSeed)
}

func TestStringSearchSeedUsesReferenceUseSite(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(`{
			"openapi":"3.0.0",
			"components":{"schemas":{"shared":{"type":"string","pattern":"^a$"}}},
			"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{"allOf":[
				{"$ref":"#/components/schemas/shared"},
				{"$ref":"#/components/schemas/shared"}
			]}}}}}}}
		}`),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)
	left := mustStringDirectedObjective(t, product, stringObjectivePatternFalse, 0)
	right := mustStringDirectedObjective(t, product, stringObjectivePatternFalse, 1)
	leftSeed, err := stringSearchSeed(left)
	require.NoError(t, err)
	rightSeed, err := stringSearchSeed(right)
	require.NoError(t, err)

	require.Equal(t, left.owner.source, right.owner.source)
	require.NotEqual(t, left.owner.occurrence.usePointer, right.owner.occurrence.usePointer)
	require.NotEqual(t, leftSeed, rightSeed)
}

func TestStringSearchSeedUsesCanonicalSchemaInputs(t *testing.T) {
	source := &jsonValue{kind: jsonObject, object: map[string]*jsonValue{
		"type": {kind: jsonString, text: "string"},
	}}
	objective := stringObjective{
		rule:  "pattern",
		level: "valid",
		owner: stringObjectiveOwner{
			node:       &schemaNode{schemaShape: &schemaShape{source: source}},
			occurrence: schemaOccurrence{usePointer: "#/schema"},
			source:     source,
		},
	}

	payload := []byte("schematest-v1\x00#/schema\x00{\"type\":\"string\"}\x00pattern\x00valid")
	digest := sha256.Sum256(payload)
	want := binary.BigEndian.Uint64(digest[:8])

	got, err := stringSearchSeed(objective)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestStringUTF16IntervalsKeepTheLockedBoundaries(t *testing.T) {
	require.Equal(t, []stringUnitInterval{
		{low: 0, high: 0x0009},
		{low: 0x000b, high: 0x000c},
		{low: 0x000e, high: 0x2027},
		{low: 0x202a, high: 0xffff},
	}, stringDotIntervals())

	universal := patternClass{
		parts: []patternClassPart{
			{ranges: []patternRange{{low: '0', high: '9'}}},
			{ranges: []patternRange{{low: '0', high: '9'}}, negated: true},
		},
	}
	require.Equal(t, []stringUnitInterval{{low: 0, high: 0xffff}}, stringClassIntervals(universal))
	require.Empty(t, stringClassIntervals(patternClass{}))
	require.Equal(t, []stringUnitInterval{{low: 0, high: 0xffff}}, stringClassIntervals(patternClass{negated: true}))
}

func TestStringIntervalCandidatesUseStableUTF16Order(t *testing.T) {
	interval := stringUnitInterval{low: 10, high: 20}

	require.Equal(t, []uint16{10, 20, 15, 14}, stringIntervalCandidates(interval, 3, []uint16{15, 10, 20}))
}

func TestBuildStringClassesEscapesAndPairedUnicode(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    string
	}{
		{name: "range", pattern: `^[a-c]$`, want: `"a"`},
		{name: "negated", pattern: `^[^a]$`, want: `"\u0000"`},
		{name: "universal", pattern: `^[^]$`, want: `"\u0000"`},
		{name: "class backspace", pattern: `^[\b]$`, want: `"\b"`},
		{name: "hex escape", pattern: `^\x41$`, want: `"A"`},
		{name: "control escape", pattern: `^\ca$`, want: `"\u0001"`},
		{name: "paired unicode", pattern: `^😀$`, want: `"😀"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := []byte(documentWithJSONSchema(`{"type":"string","pattern":` + strconv.Quote(test.pattern) + `}`))
			var cases []Case
			_, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000}, func(testCase Case) error {
				cases = append(cases, testCase)

				return nil
			})
			require.NoError(t, err)
			require.NotEmpty(t, cases)
			require.Equal(t, test.want, string(cases[0].JSON))
			require.True(t, utf8.Valid(cases[0].JSON))
		})
	}
}

func TestMergedPropertyProductRunsDirectedScheduleAtLastAuthoredTarget(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"object",
			"allOf":[
				{"properties":{"x":{"type":"string","pattern":"^a$"}}},
				{"properties":{"x":{"pattern":"^a{2}$"}}}
			]
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	choices, err := rowChildSchemaChoices(
		model.root,
		model.root.occurrence,
		nil,
		rowChildProperty,
		"x",
	)
	require.NoError(t, err)
	require.Len(t, choices, 1)
	product, err := buildStringProduct(choices[0].node, choices[0].occurrence, nil)
	require.NoError(t, err)

	targetNode := model.root.allOf[1].properties["x"]
	targetOccurrence := rebasePlanOccurrence(
		targetNode,
		model.root.occurrence.usePointer+"/allOf/1/properties/x",
		appendInstanceToken(model.root.occurrence.instanceTemplate, "x"),
	)
	last, err := stringProductTargetIsLast(product, makeRuleIdentity(targetOccurrence, oracleRulePattern))
	require.NoError(t, err)
	require.True(t, last)

	state := &search{model: model, maxSteps: 100}
	var kinds []stringObjectiveKind
	_, err = state.runStringObjectiveSchedule(
		product,
		product.defaultOwner,
		oracleRulePattern,
		"valid",
		last,
		func(objective stringObjective, _ *jsonValue) (bool, error) {
			kinds = append(kinds, objective.kind)

			return false, nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, []stringObjectiveKind{
		stringObjectivePatternFalse,
		stringObjectivePatternFalse,
	}, kinds)
}

func TestStringProductAnyOfPartialPinsLeaveSiblingsUnconstrained(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"string",
			"anyOf":[{"pattern":"^a$"},{"pattern":"^a$"}]
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	find := func(pins []applicabilityPin) (string, bool) {
		t.Helper()

		product, productErr := buildStringProduct(model.root, model.root.occurrence, pins)
		require.NoError(t, productErr)
		state := &search{model: model, maxSteps: 100}
		var value string
		complete, searchErr := state.searchStringObjective(
			product,
			firstStringAllTrueObjective(t, product, oracleRuleAnyOf, planLevelMask+"1"),
			func(candidate *jsonValue) (bool, error) {
				value = candidate.text

				return true, nil
			},
		)
		require.NoError(t, searchErr)

		return value, complete
	}

	partial, complete := find(anyOfValidPins(model.root.occurrence, 0))
	require.True(t, complete)
	require.Equal(t, "a", partial)

	_, complete = find(anyOfMaskPins(model.root.occurrence, 2, big.NewInt(1)))
	require.False(t, complete)

	exact, complete := find(anyOfMaskPins(model.root.occurrence, 2, big.NewInt(3)))
	require.True(t, complete)
	require.Equal(t, "a", exact)
}

func TestBuildAnyOfFirstFalseAlternativeDoesNotStarveLaterMask(t *testing.T) {
	for _, pattern := range []string{".*", "(?:a*)"} {
		t.Run(pattern, func(t *testing.T) {
			document := []byte(documentWithJSONSchema(`{
				"type":"string",
				"anyOf":[
					{"allOf":[{"pattern":` + strconv.Quote(pattern) + `},{"pattern":"^b$"}]},
					{"minLength":1}
				]
			}`))

			var cases []Case
			report, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1_000}, func(testCase Case) error {
				cases = append(cases, testCase)

				return nil
			})
			require.NoError(t, err)
			require.NotEmpty(t, cases)
			require.Contains(
				t,
				report.Covered,
				"#/paths/~1/post/requestBody/content/application~1json/schema|#|anyOf|level:mask:2",
			)
		})
	}
}

func TestBuildAnyOfEnumFalseBranchCoversMaskFour(t *testing.T) {
	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"anyOf":[
			{"enum":["b"],"pattern":"^a$"},
			{"pattern":"^b$"},
			{"pattern":"^a$"}
		]
	}`))

	var cases []Case
	report, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10_000}, func(testCase Case) error {
		cases = append(cases, testCase)

		return nil
	})
	require.NoError(t, err)
	require.Contains(t, cases, Case{JSON: []byte(`"a"`), Valid: true})
	require.Contains(t, report.Covered, "#/paths/~1/post/requestBody/content/application~1json/schema|#|anyOf|level:mask:4")
}

func TestDirectedStringObjectiveRejectsExtraFailureClosure(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"string",
			"enum":["a"],
			"minLength":1,
			"maxLength":1,
			"pattern":"^a$"
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)
	objective := mustStringDirectedObjective(t, product, stringObjectivePatternFalse, 0)
	objective.term = &stringTruthTerm{assignments: []stringTruthAssignment{
		{constraint: stringConstraintRef{kind: stringConstraintEnum, index: 0}, truth: false},
		{constraint: stringConstraintRef{kind: stringConstraintLength, index: 0}, truth: true},
		{constraint: stringConstraintRef{kind: stringConstraintLength, index: 1}, truth: true},
		{constraint: stringConstraintRef{kind: stringConstraintPattern, index: 0}, truth: false},
	}}

	state := &search{model: model, maxSteps: 100}
	complete, err := state.searchStringObjective(product, objective, func(*jsonValue) (bool, error) {
		return true, nil
	})
	require.NoError(t, err)
	require.False(t, complete)
}

func TestDirectedStringObjectiveUsesOwnerOracle(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"object",
			"properties":{"x":{"type":"string","minLength":1,"maxLength":1,"pattern":"^a$"}}
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	node := model.root.properties["x"]
	occurrence := rebasePlanOccurrence(
		node,
		model.root.occurrence.usePointer+"/properties/x",
		appendInstanceToken(model.root.occurrence.instanceTemplate, "x"),
	)
	product, err := buildStringProduct(node, occurrence, nil)
	require.NoError(t, err)
	state := &search{model: model, maxSteps: 100}
	var value string
	complete, err := state.searchStringObjective(
		product,
		mustStringDirectedObjective(t, product, stringObjectivePatternFalse, 0),
		func(candidate *jsonValue) (bool, error) {
			value = candidate.text

			return true, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.Equal(t, "\x00", value)
}

func TestStringObjectiveMaximumKeepsTighterEnumBound(t *testing.T) {
	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"string",
			"enum":["a"],
			"pattern":"^.{0,100}$"
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)
	maximum, finite, err := stringObjectiveMaximumLength(
		product,
		firstStringAllTrueObjective(t, product, oracleRuleEnum, "valid"),
	)
	require.NoError(t, err)
	require.True(t, finite)
	require.Equal(t, uint64(1), maximum)
}

func TestStringObjectiveSeedRejectsMissingOwner(t *testing.T) {
	_, err := stringSearchSeed(stringObjective{rule: "pattern", level: "valid"})
	require.Error(t, err)
}

func TestBuildStringTruthSetupIsLazyForSharedYAMLDAG(t *testing.T) {
	const choice = "                - &choice\n                    anyOf:\n                      - pattern: ^a$\n                      - pattern: ^a$\n"
	const alias = "                - *choice\n"
	document := "openapi: 3.0.0\npaths:\n  /:\n    post:\n      operationId: selected\n      requestBody:\n        content:\n          application/json:\n            schema:\n              type: string\n              allOf:\n" + choice + strings.Repeat(alias, 20)

	report, err := Build(Input{OpenAPI: []byte(document), OperationID: "selected", MaxSteps: 1}, func(Case) error {
		t.Fatal("tiny shared-DAG budget emitted a case")

		return nil
	})
	require.NoError(t, err)
	require.Equal(t, MaxStepsReached, report.Stop)
	require.Equal(t, uint64(1), report.Steps)
}

func TestStringProductIterativeLargeExactLength(t *testing.T) {
	const length = 4096
	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"type":"string","minLength":4096,"maxLength":4096}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	product, err := buildStringProduct(model.root, model.root.occurrence, nil)
	require.NoError(t, err)
	state := &search{model: model, maxSteps: length + 1}
	var value string
	complete, err := state.searchStringObjective(
		product,
		stringAllTrueObjective(product, "minLength", "valid"),
		func(candidate *jsonValue) (bool, error) {
			value = candidate.text

			return true, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.Len(t, []rune(value), length)
	require.Equal(t, uint64(length+1), state.steps)
}

func stringAllTrueObjective(product stringProduct, rule, level string) stringObjective {
	return stringObjective{
		kind:  stringObjectiveAllTrue,
		rule:  rule,
		level: level,
		owner: product.defaultOwner,
	}
}

func firstStringAllTrueObjective(
	t *testing.T,
	product stringProduct,
	rule, level string,
) stringObjective {
	t.Helper()

	current := stringTruthTerm{}
	var selected []stringTruthAssignment
	stopped, err := enumerateStringTruth(product.truth, true, &current, func(term *stringTruthTerm) (bool, error) {
		selected = append([]stringTruthAssignment(nil), term.assignments...)

		return true, nil
	})
	require.NoError(t, err)
	require.True(t, stopped)

	return stringObjective{
		kind:  stringObjectiveAllTrue,
		rule:  rule,
		level: level,
		owner: product.defaultOwner,
		term:  &stringTruthTerm{assignments: selected},
	}
}

func mustStringDirectedObjective(
	t *testing.T,
	product stringProduct,
	kind stringObjectiveKind,
	index int,
) stringObjective {
	t.Helper()

	objective, err := newStringDirectedObjective(product, kind, index)
	require.NoError(t, err)

	return objective
}

func stringScheduleKinds(t *testing.T, product stringProduct) []stringObjectiveKind {
	t.Helper()

	kinds := make([]stringObjectiveKind, 0)
	_, err := enumerateStringTruth(product.truth, true, &stringTruthTerm{}, func(*stringTruthTerm) (bool, error) {
		kinds = append(kinds, stringObjectiveAllTrue)

		return false, nil
	})
	require.NoError(t, err)
	for range product.patterns {
		kinds = append(kinds, stringObjectivePatternFalse)
	}
	for range product.formats {
		kinds = append(kinds, stringObjectiveFormatFalse)
	}
	for range product.lengths {
		kinds = append(kinds, stringObjectiveLengthFalse)
	}

	return kinds
}

func objectiveKinds(objectives []stringObjective) []stringObjectiveKind {
	kinds := make([]stringObjectiveKind, 0, len(objectives))
	for _, objective := range objectives {
		kinds = append(kinds, objective.kind)
	}

	return kinds
}
