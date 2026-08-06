//nolint:godoclint,lll,paralleltest,wsl_v5 // These focused cases pin the private string-search seam.
package schematest

import (
	"crypto/sha256"
	"encoding/binary"
	"strconv"
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
