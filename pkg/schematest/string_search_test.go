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
			product.model = model
			objectives := stringProductObjectives(product, oracleRuleAnyOf, planLevelMask+"2")
			require.NotEmpty(t, objectives)

			state := &search{model: model, maxSteps: 10000}
			var value *jsonValue
			complete, err := state.searchStringObjective(product, objectives[0], func(candidate *jsonValue) (bool, error) {
				value = candidate

				return true, nil
			})
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
	product.model = model

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
	product.model = model

	searchState := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringObjective{kind: stringObjectivePatternFalse, index: 0, rule: "pattern", level: "false"},
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
	product.model = model

	objectives := stringProductObjectives(product, "pattern", "valid")
	for _, test := range []struct {
		name      string
		objective stringObjective
		want      string
	}{
		{
			name:      "minimum",
			objective: objectives[len(objectives)-4],
			want:      "a",
		},
		{
			name:      "maximum",
			objective: objectives[len(objectives)-3],
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
	product.model = model

	searchState := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringObjective{kind: stringObjectivePatternFalse, index: 0, rule: "pattern", level: "false"},
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
	product.model = model

	searchState := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringObjective{kind: stringObjectiveLengthFalse, index: 0, rule: "minLength", level: "false"},
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
	product.model = model

	searchState := &search{model: model, maxSteps: 5}
	complete, err := searchState.searchStringObjective(
		product,
		stringObjective{kind: stringObjectivePatternFalse, index: 0, rule: "pattern", level: "false"},
		func(*jsonValue) (bool, error) {
			t.Fatal("budget cutoff produced a partial directed witness")

			return false, nil
		},
	)
	require.False(t, complete)
	require.ErrorIs(t, err, errMaxSteps)
	require.Equal(t, uint64(5), searchState.steps)
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
	product.model = model

	searchState := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringObjective{kind: stringObjectiveAllTrue, rule: "pattern", level: "valid"},
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
	product.model = model

	searchState := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringObjective{kind: stringObjectiveAllTrue, rule: "pattern", level: "valid"},
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
	product.model = model

	searchState := &search{model: model, maxSteps: 8}
	complete, err := searchState.searchStringObjective(
		product,
		stringObjective{kind: stringObjectiveAllTrue, rule: "pattern", level: "valid"},
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
			product.model = model

			state := &search{model: model, maxSteps: 100}
			var values []string
			complete, err := state.searchStringObjective(
				product,
				stringObjective{kind: stringObjectiveAllTrue, rule: "format", level: "valid"},
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
	product.model = model
	require.Equal(
		t,
		[]stringObjectiveKind{
			stringObjectiveAllTrue,
			stringObjectivePatternFalse,
			stringObjectiveFormatFalse,
			stringObjectiveLengthFalse,
		},
		objectiveKinds(stringProductObjectives(product, "format", "valid")),
	)

	state := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := state.searchStringObjective(
		product,
		stringObjective{kind: stringObjectiveFormatFalse, index: 0, rule: "format", level: "false"},
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
			product.model = model

			state := &search{model: model, maxSteps: 100}
			var values []string
			complete, err := state.searchStringObjective(
				product,
				stringObjective{kind: stringObjectiveFormatFalse, index: 0, rule: "format", level: "false"},
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
	product.model = model
	require.Empty(t, product.formats)
	require.Equal(
		t,
		[]stringObjectiveKind{stringObjectiveAllTrue},
		objectiveKinds(stringProductObjectives(product, "format", "valid")),
	)

	state := &search{model: model, maxSteps: 10}
	var values []string
	complete, err := state.searchStringObjective(
		product,
		stringObjective{kind: stringObjectiveAllTrue, rule: "format", level: "valid"},
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
	product.model = model

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
	product.model = model

	searchState := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringObjective{kind: stringObjectiveAllTrue, rule: "pattern", level: "valid"},
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
	product.model = model

	searchState := &search{model: model, maxSteps: 100}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringObjective{kind: stringObjectiveAllTrue, rule: "pattern", level: "valid"},
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
	product.model = model

	searchState := &search{model: model, maxSteps: 8}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringObjective{kind: stringObjectiveAllTrue, rule: "pattern", level: "valid"},
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
	product.model = model

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
	product.model = model

	searchState := &search{model: model, maxSteps: 1000}
	var lengths []int
	complete, err := searchState.searchStringObjective(
		product,
		stringObjective{kind: stringObjectiveAllTrue, rule: "minLength", level: "valid"},
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
	product.model = model

	searchState := &search{model: model, maxSteps: 1000}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringObjective{kind: stringObjectiveFormatFalse, index: 0, rule: "format", level: "false"},
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
	product.model = model

	searchState := &search{model: model, maxSteps: 1000}
	var values []string
	complete, err := searchState.searchStringObjective(
		product,
		stringObjective{kind: stringObjectiveAllTrue, rule: "format", level: "valid"},
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
	product.model = model

	state := &search{model: model, maxSteps: 100_000}
	var values []string
	complete, err := state.searchStringObjective(
		product,
		stringObjective{kind: stringObjectiveAllTrue, rule: "format", level: "valid"},
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
			product.model = model

			searchState := &search{model: model, maxSteps: 1000}
			var values []string
			complete, err := searchState.searchStringObjective(
				product,
				stringObjective{kind: stringObjectiveAllTrue, rule: "format", level: "valid"},
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

func TestStringSearchSeedUsesCanonicalSchemaInputs(t *testing.T) {
	model := &schemaModel{
		schemaPointer:       "#/schema",
		canonicalSchemaJSON: `{"type":"string"}`,
	}
	objective := stringObjective{rule: "pattern", level: "valid"}

	payload := []byte("schematest-v1\x00#/schema\x00{\"type\":\"string\"}\x00pattern\x00valid")
	digest := sha256.Sum256(payload)
	want := binary.BigEndian.Uint64(digest[:8])

	require.Equal(t, want, stringSearchSeed(model, objective))
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

func objectiveKinds(objectives []stringObjective) []stringObjectiveKind {
	kinds := make([]stringObjectiveKind, 0, len(objectives))
	for _, objective := range objectives {
		kinds = append(kinds, objective.kind)
	}

	return kinds
}
