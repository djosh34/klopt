//nolint:godoclint // Table-driven private oracle seam tests keep specification matrices together.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluateTypelessSchemaAdmitsEveryJSONKind(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		value string
		kind  string
	}{
		{name: "null", value: "null", kind: "null"},
		{name: "boolean", value: "true", kind: "boolean"},
		{name: "number", value: "1.5", kind: "number"},
		{name: "string", value: `"text"`, kind: "string"},
		{name: "array", value: "[]", kind: "array"},
		{name: "object", value: "{}", kind: "object"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, `{}`, test.value)
			require.NoError(t, result.err)
			require.True(t, result.valid)
			require.Equal(t, []string{"type"}, applicableRules(result.applicable))
			require.Equal(t, []string{test.kind}, observedLevels(result.observed))
			require.Empty(t, result.failures)
		})
	}
}

func TestEvaluateExplicitKindsUseJSONKindAndExactIntegerMembership(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   string
		value    string
		valid    bool
		observed []string
		failures []string
	}{
		{name: "boolean", schema: `{"type":"boolean"}`, value: "false", valid: true, observed: []string{"boolean"}},
		{name: "number", schema: `{"type":"number"}`, value: "1.5", valid: true, observed: []string{"number"}},
		{name: "string", schema: `{"type":"string"}`, value: `"text"`, valid: true, observed: []string{"string"}},
		{name: "array", schema: `{"type":"array","items":{}}`, value: "[]", valid: true, observed: []string{"array"}},
		{name: "object", schema: `{"type":"object"}`, value: "{}", valid: true, observed: []string{"object"}},
		{name: "integer decimal", schema: `{"type":"integer"}`, value: "1.0", valid: true, observed: []string{"number"}},
		{name: "integer exponent", schema: `{"type":"integer"}`, value: "1e0", valid: true, observed: []string{"number"}},
		{name: "integer fraction", schema: `{"type":"integer"}`, value: "1.5", valid: false, failures: []string{"type"}},
		{
			name:     "integer fraction above float64 precision",
			schema:   `{"type":"integer"}`,
			value:    "9007199254740992.5",
			valid:    false,
			failures: []string{"type"},
		},
		{name: "integer wrong kind", schema: `{"type":"integer"}`, value: `"1"`, valid: false, failures: []string{"type"}},
		{name: "explicit non-nullable", schema: `{"type":"string"}`, value: "null", valid: false, failures: []string{"type"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, test.schema, test.value)
			require.NoError(t, result.err)
			require.Equal(t, test.valid, result.valid)
			require.Equal(t, []string{"type"}, applicableRules(result.applicable))
			require.Equal(t, test.observed, observedLevels(result.observed))
			require.Equal(t, test.failures, failureRules(result.failures))
		})
	}
}

func TestEvaluateNullableIsSameObjectAndLeavesEnumActive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   string
		valid    bool
		observed []string
		failures []string
	}{
		{
			name:     "typeless nullable is inert",
			schema:   `{"nullable":true}`,
			valid:    true,
			observed: []string{"null"},
		},
		{
			name:     "typeless enum admits listed null",
			schema:   `{"enum":[null,false]}`,
			valid:    true,
			observed: []string{"null", "member:0"},
		},
		{
			name:     "typeless enum rejects unlisted null",
			schema:   `{"enum":[false]}`,
			valid:    false,
			observed: []string{"null"},
			failures: []string{"enum"},
		},
		{
			name:     "explicit type rejects null",
			schema:   `{"type":"string"}`,
			valid:    false,
			failures: []string{"type"},
		},
		{
			name:     "explicit nullable admits null",
			schema:   `{"type":"string","nullable":true}`,
			valid:    true,
			observed: []string{"null"},
		},
		{
			name:     "explicit nullable keeps enum active",
			schema:   `{"type":"string","nullable":true,"enum":["text"]}`,
			valid:    false,
			observed: []string{"null"},
			failures: []string{"enum"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, test.schema, "null")
			require.NoError(t, result.err)
			require.Equal(t, test.valid, result.valid)
			require.Equal(t, test.observed, observedLevels(result.observed))
			require.Equal(t, test.failures, failureRules(result.failures))

			if test.name == "explicit nullable keeps enum active" {
				require.Equal(t, []string{"type", "enum"}, applicableRules(result.applicable))
			}
		})
	}
}

func TestEvaluateEnumsUseCleanJSONSemantics(t *testing.T) {
	t.Parallel()

	schema := `{"enum":[1,"a",[1,2],{"a":1,"b":2}]}`

	for _, test := range []struct {
		name  string
		value string
		valid bool
	}{
		{name: "exact number", value: `1.0`, valid: true},
		{name: "decoded string", value: `"\u0061"`, valid: true},
		{name: "ordered array", value: `[1.0,2e0]`, valid: true},
		{name: "object member order", value: `{"b":2.0,"a":1}`, valid: true},
		{name: "array order matters", value: `[2,1]`, valid: false},
		{name: "object member differs", value: `{"a":1}`, valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, schema, test.value)
			require.NoError(t, result.err)
			require.Equal(t, test.valid, result.valid)
			require.Equal(t, test.valid, len(result.failures) == 0)
			require.Equal(t, []string{"type", "enum"}, applicableRules(result.applicable))
		})
	}
}

func TestEvaluateEnumDuplicateMembersKeepFirstAuthoredLevel(t *testing.T) {
	t.Parallel()

	result := evaluateSchemaValue(t, `{"enum":[1,1.0,{"a":1,"b":2},{"b":2.0,"a":1}]}`, `{"b":2,"a":1}`)
	require.NoError(t, result.err)
	require.True(t, result.valid)
	require.Equal(t, []string{"type", "enum"}, applicableRules(result.applicable))
	require.Equal(t, []string{"object", "member:1"}, observedLevels(result.observed))
	require.Empty(t, result.failures)
}

func TestEvaluateKeepsSiblingFailuresAndOrderDeterministic(t *testing.T) {
	t.Parallel()

	schema := `{"type":"string","enum":["expected"]}`
	first := evaluateSchemaValue(t, schema, `1`)
	second := evaluateSchemaValue(t, schema, `1`)

	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.Equal(t, first, second)
	require.False(t, first.valid)
	require.Equal(t, []string{"type", "enum"}, applicableRules(first.applicable))
	require.Empty(t, first.observed)
	require.Equal(t, []string{"type", "enum"}, failureRules(first.failures))
	require.Equal(
		t,
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|type",
		first.failures[0].String(),
	)
	require.Equal(
		t,
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|enum",
		first.failures[1].String(),
	)
}

func TestEvaluateRejectsMalformedPrivateJSONValue(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	result := evaluate(model, &jsonValue{kind: jsonNumber})
	require.Error(t, result.err)
	require.False(t, result.valid)
	require.Empty(t, result.applicable)
}

func evaluateSchemaValue(t *testing.T, schema, value string) evaluation {
	t.Helper()

	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(schema)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	parsed, err := parseStrictJSON([]byte(value))
	require.NoError(t, err)

	return evaluate(model, parsed)
}

func applicableRules(identities []ruleIdentity) []string {
	if len(identities) == 0 {
		return nil
	}

	result := make([]string, 0, len(identities))
	for _, identity := range identities {
		result = append(result, identity.rule)
	}

	return result
}

func observedLevels(identities []levelIdentity) []string {
	if len(identities) == 0 {
		return nil
	}

	result := make([]string, 0, len(identities))
	for _, identity := range identities {
		result = append(result, identity.level)
	}

	return result
}

func failureRules(identities []failureIdentity) []string {
	if len(identities) == 0 {
		return nil
	}

	result := make([]string, 0, len(identities))
	for _, identity := range identities {
		result = append(result, identity.rule)
	}

	return result
}
