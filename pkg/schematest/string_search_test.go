//nolint:godoclint,lll,paralleltest,wsl_v5 // These focused cases pin the private string-search seam.
package schematest

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildUsesOneProductForSimultaneousStringPatterns(t *testing.T) {
	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"allOf":[
			{"pattern":"^a+$"},
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
	require.Equal(t, `"aa"`, string(cases[0].JSON))
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
			"allOf":[{"pattern":"^a$"},{"pattern":"^b$"}]
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

func objectiveKinds(objectives []stringObjective) []stringObjectiveKind {
	kinds := make([]stringObjectiveKind, 0, len(objectives))
	for _, objective := range objectives {
		kinds = append(kinds, objective.kind)
	}

	return kinds
}
