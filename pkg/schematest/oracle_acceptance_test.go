//nolint:godoclint,lll // Acceptance tables keep clean-model expectations explicit beside corpus operations.
package schematest

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	anyOfBodySchemaPointer = "#/paths/~1any-of~1{id}/post/requestBody/content/application~1json/schema"
	alphaSchemaPointer     = "#/paths/~1alpha/post/requestBody/content/application~1json/schema"
	closedSchemaPointer    = "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema"
)

func TestCleanOracleAnyOfBodyAndParametersUsesOnlyBodyComposition(t *testing.T) {
	t.Parallel()

	model := cleanCorpusModel(t, "../../resources/openapi.yaml", "anyOfBodyAndParameters")

	tests := []struct {
		name       string
		value      string
		valid      bool
		allOf      [][]bool
		anyOf      [][]bool
		observed   []string
		failures   []string
		applicable []string
	}{
		{
			name:  "first and second body branches",
			value: `"az"`,
			valid: true,
			allOf: [][]bool{{true}},
			anyOf: [][]bool{{true, true}},
			applicable: []string{
				anyOfBodySchemaPointer + "|#|type",
				anyOfBodySchemaPointer + "/allOf/0|#|type",
				anyOfBodySchemaPointer + "/allOf/0|#|pattern",
				anyOfBodySchemaPointer + "/anyOf/0|#|type",
				anyOfBodySchemaPointer + "/anyOf/0|#|pattern",
				anyOfBodySchemaPointer + "/anyOf/1|#|type",
				anyOfBodySchemaPointer + "/anyOf/1|#|pattern",
			},
			observed: []string{
				"string", "string", "valid", "string", "valid", "string", "valid",
			},
		},
		{
			name:  "only first body branch",
			value: `"a"`,
			valid: true,
			allOf: [][]bool{{true}},
			anyOf: [][]bool{{true, false}},
			applicable: []string{
				anyOfBodySchemaPointer + "|#|type",
				anyOfBodySchemaPointer + "/allOf/0|#|type",
				anyOfBodySchemaPointer + "/allOf/0|#|pattern",
				anyOfBodySchemaPointer + "/anyOf/0|#|type",
				anyOfBodySchemaPointer + "/anyOf/0|#|pattern",
				anyOfBodySchemaPointer + "/anyOf/1|#|type",
				anyOfBodySchemaPointer + "/anyOf/1|#|pattern",
			},
			observed: []string{"string", "string", "valid", "string", "valid", "string"},
		},
		{
			name:  "all body branches fail",
			value: `"x"`,
			allOf: [][]bool{{false}},
			anyOf: [][]bool{{false, false}},
			applicable: []string{
				anyOfBodySchemaPointer + "|#|type",
				anyOfBodySchemaPointer + "/allOf/0|#|type",
				anyOfBodySchemaPointer + "/allOf/0|#|pattern",
				anyOfBodySchemaPointer + "/anyOf/0|#|type",
				anyOfBodySchemaPointer + "/anyOf/0|#|pattern",
				anyOfBodySchemaPointer + "/anyOf/1|#|type",
				anyOfBodySchemaPointer + "/anyOf/1|#|pattern",
			},
			observed: []string{"string", "string", "string", "string"},
			failures: []string{
				anyOfBodySchemaPointer + "/allOf/0|#|pattern",
				anyOfBodySchemaPointer + "/anyOf/0|#|pattern",
				anyOfBodySchemaPointer + "/anyOf/1|#|pattern",
				anyOfBodySchemaPointer + "|#|anyOf",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			value, err := parseStrictJSON([]byte(test.value))
			require.NoError(t, err)

			result := evaluate(model, value)
			require.NoError(t, result.err)
			require.Equal(t, test.valid, result.valid)
			require.Equal(t, test.allOf, compositionTruthVectorsForTest(result.allOf))
			require.Equal(t, test.anyOf, compositionTruthVectorsForTest(result.anyOf))
			require.Equal(t, test.applicable, identityStrings(result.applicable))
			require.Equal(t, test.observed, observedLevels(result.observed))
			require.Equal(t, test.failures, identityStrings(result.failures))
		})
	}
}

func TestCleanOracleRequiredNullableClosedObjectUsesExplicitRequestSemantics(t *testing.T) {
	t.Parallel()

	model := cleanCorpusModel(t, "../../resources/openapi.yaml", "nullableObjectKeysAdditionalPropertiesFalse")

	tests := []struct {
		name       string
		value      string
		valid      bool
		applicable []string
		observed   []string
		failures   []string
	}{
		{
			name:       "nullable root null",
			value:      "null",
			valid:      true,
			applicable: []string{closedSchemaPointer + "|#|type"},
			observed:   []string{"null"},
		},
		{
			name:  "required nullable and nonnullable properties",
			value: `{"requiredNullableString":null,"requiredNotNullableString":"ok"}`,
			valid: true,
			applicable: []string{
				closedSchemaPointer + "|#|type",
				closedSchemaPointer + "|#/requiredNotNullableString|required",
				closedSchemaPointer + "|#/requiredNullableString|required",
				closedSchemaPointer + "/properties/requiredNotNullableString|#/requiredNotNullableString|type",
				closedSchemaPointer + "/properties/requiredNullableString|#/requiredNullableString|type",
			},
			observed: []string{"object", "present", "present", "string", "null"},
		},
		{
			name:  "closed object rejects an undeclared member",
			value: `{"requiredNullableString":null,"requiredNotNullableString":"ok","extra":true}`,
			applicable: []string{
				closedSchemaPointer + "|#|type",
				closedSchemaPointer + "|#/requiredNotNullableString|required",
				closedSchemaPointer + "|#/requiredNullableString|required",
				closedSchemaPointer + "/properties/requiredNotNullableString|#/requiredNotNullableString|type",
				closedSchemaPointer + "/properties/requiredNullableString|#/requiredNullableString|type",
				closedSchemaPointer + "|#/extra|additionalProperties",
			},
			observed: []string{"object", "present", "present", "string", "null"},
			failures: []string{closedSchemaPointer + "|#/extra|additionalProperties"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			value, err := parseStrictJSON([]byte(test.value))
			require.NoError(t, err)

			result := evaluate(model, value)
			require.NoError(t, result.err)
			require.Equal(t, test.valid, result.valid)
			require.Equal(t, test.applicable, identityStrings(result.applicable))
			require.Equal(t, test.observed, observedLevels(result.observed))
			require.Equal(t, test.failures, identityStrings(result.failures))
		})
	}
}

func TestCleanOracleAlphaAndZetaCorpusSemanticsAreAuthored(t *testing.T) {
	t.Parallel()

	alpha := cleanCorpusModel(t, "testdata/alpha_zeta.yaml", "alphaRequest")
	zeta := cleanCorpusModel(t, "testdata/alpha_zeta.yaml", "zetaRequest")

	alphaValue, err := parseStrictJSON([]byte(`{"array":[1],"enum":null,"number":1.5,"text":"a@example.com","closed":{"child":"ok"}}`))
	require.NoError(t, err)

	alphaResult := evaluate(alpha, alphaValue)
	require.NoError(t, alphaResult.err)
	require.True(t, alphaResult.valid)
	require.Empty(t, alphaResult.failures)
	require.Equal(t, [][]bool{{true, true}}, compositionTruthVectorsForTest(alphaResult.allOf))
	require.Empty(t, alphaResult.anyOf)
	require.Contains(t, levelIdentityStrings(alphaResult.observed), alphaSchemaPointer+"/properties/enum|#/enum|enum|level:member:0")
	require.Contains(t, levelIdentityStrings(alphaResult.observed), alphaSchemaPointer+"/properties/text|#/text|format|level:valid")

	for _, test := range []struct {
		name     string
		value    string
		valid    bool
		truth    [][]bool
		observed []string
	}{
		{
			name:     "true selects the first enum branch",
			value:    "true",
			valid:    true,
			truth:    [][]bool{{true, false}},
			observed: []string{"boolean", "boolean", "member:0", "boolean"},
		},
		{
			name:     "false selects the second enum branch",
			value:    "false",
			valid:    true,
			truth:    [][]bool{{false, true}},
			observed: []string{"boolean", "boolean", "boolean", "member:0"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			value, parseErr := parseStrictJSON([]byte(test.value))
			require.NoError(t, parseErr)

			result := evaluate(zeta, value)
			require.NoError(t, result.err)
			require.Equal(t, test.valid, result.valid)
			require.Equal(t, test.truth, compositionTruthVectorsForTest(result.anyOf))
			require.Equal(t, test.observed, observedLevels(result.observed))
			require.Empty(t, result.failures)
		})
	}
}

func TestCleanOracleFailureIdentitiesDistinguishRuleAndBranchOccurrences(t *testing.T) {
	t.Parallel()

	root := "#/paths/~1/post/requestBody/content/application~1json/schema"

	tests := []struct {
		name   string
		schema string
		value  string
		want   []string
	}{
		{name: "local type", schema: `{"type":"string"}`, value: "1", want: []string{root + "|#|type"}},
		{name: "enum", schema: `{"enum":[true]}`, value: "false", want: []string{root + "|#|enum"}},
		{name: "bounds", schema: `{"type":"number","minimum":2,"maximum":1}`, value: "1.5", want: []string{root + "|#|minimum", root + "|#|maximum"}},
		{name: "pattern", schema: `{"type":"string","pattern":"^a"}`, value: `"b"`, want: []string{root + "|#|pattern"}},
		{name: "format", schema: `{"type":"string","format":"date"}`, value: `"not-a-date"`, want: []string{root + "|#|format"}},
		{name: "required", schema: `{"type":"object","required":["name"]}`, value: `{}`, want: []string{root + "|#/name|required"}},
		{name: "additional property", schema: `{"type":"object","additionalProperties":false}`, value: `{"extra":true}`, want: []string{root + "|#/extra|additionalProperties"}},
		{
			name:   "branch and aggregate anyOf",
			schema: `{"anyOf":[{"type":"string","pattern":"^a"},{"type":"number","minimum":2}]}`,
			value:  `1`,
			want: []string{
				root + "/anyOf/0|#|type",
				root + "/anyOf/1|#|minimum",
				root + "|#|anyOf",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, test.schema, test.value)
			require.NoError(t, result.err)
			require.False(t, result.valid)
			require.Equal(t, test.want, identityStrings(result.failures))
		})
	}
}

func TestCleanOracleEvaluationDoesNotDependOnMapConstructionOrder(t *testing.T) {
	t.Parallel()

	firstModel, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"object",
			"required":["z","a"],
			"properties":{"z":{"type":"number","minimum":2},"a":{"type":"string","pattern":"^a"}},
			"additionalProperties":false
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	secondModel, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"additionalProperties":false,
			"properties":{"a":{"pattern":"^a","type":"string"},"z":{"minimum":2,"type":"number"}},
			"required":["z","a"],
			"type":"object"
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	firstValue, err := parseStrictJSON([]byte(`{"z":1,"a":"b","extra":true}`))
	require.NoError(t, err)
	secondValue, err := parseStrictJSON([]byte(`{"extra":true,"a":"b","z":1}`))
	require.NoError(t, err)

	first := evaluate(firstModel, firstValue)
	repeat := evaluate(firstModel, firstValue)
	second := evaluate(secondModel, secondValue)

	require.NoError(t, first.err)
	require.NoError(t, repeat.err)
	require.NoError(t, second.err)
	require.Equal(t, first, repeat)
	require.Equal(t, first, second)
	require.Equal(t, []string{
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a|#/a|pattern",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/z|#/z|minimum",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#/extra|additionalProperties",
	}, identityStrings(first.failures))
}

func levelIdentityStrings(identities []levelIdentity) []string {
	if len(identities) == 0 {
		return nil
	}

	result := make([]string, 0, len(identities))
	for _, identity := range identities {
		result = append(result, identity.String())
	}

	return result
}

func cleanCorpusModel(t *testing.T, path, operationID string) *schemaModel {
	t.Helper()

	document, err := os.ReadFile(path)
	require.NoError(t, err)

	model, err := parseInput(Input{OpenAPI: document, OperationID: operationID})
	require.NoError(t, err)

	return model
}
