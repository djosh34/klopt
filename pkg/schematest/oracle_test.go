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

func TestEvaluateNumericBoundsUseExactComparisons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   string
		value    string
		valid    bool
		observed []string
		failures []string
	}{
		{
			name:     "decimal boundary is inclusive",
			schema:   `{"type":"number","minimum":0.10,"maximum":2e0}`,
			value:    "0.1",
			valid:    true,
			observed: []string{"number", "valid", "valid"},
		},
		{
			name:     "decimal below exact minimum",
			schema:   `{"type":"number","minimum":0.10}`,
			value:    "0.09999999999999999999",
			valid:    false,
			observed: []string{"number"},
			failures: []string{"minimum"},
		},
		{
			name:     "exponent above exact maximum",
			schema:   `{"type":"number","maximum":2e0}`,
			value:    "2.00000000000000000001",
			valid:    false,
			observed: []string{"number"},
			failures: []string{"maximum"},
		},
		{
			name:     "exclusive lower boundary",
			schema:   `{"type":"number","minimum":1e-1,"exclusiveMinimum":true}`,
			value:    "0.1",
			valid:    false,
			observed: []string{"number"},
			failures: []string{"exclusiveMinimum"},
		},
		{
			name:     "exclusive upper boundary",
			schema:   `{"type":"number","maximum":2E0,"exclusiveMaximum":true}`,
			value:    "2",
			valid:    false,
			observed: []string{"number"},
			failures: []string{"exclusiveMaximum"},
		},
		{
			name:     "single point",
			schema:   `{"type":"number","minimum":1,"maximum":1}`,
			value:    "1.0",
			valid:    true,
			observed: []string{"number", "valid", "valid"},
		},
		{
			name:     "contradictory bounds",
			schema:   `{"type":"number","minimum":2,"maximum":1}`,
			value:    "1.5",
			valid:    false,
			observed: []string{"number"},
			failures: []string{"minimum", "maximum"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, test.schema, test.value)
			require.NoError(t, result.err)
			require.Equal(t, test.valid, result.valid)
			require.Equal(t, test.observed, observedLevels(result.observed))
			require.Equal(t, test.failures, failureRules(result.failures))
		})
	}
}

func TestEvaluateMultipleOfAndIntegerMembershipRemainExact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   string
		value    string
		valid    bool
		failures []string
	}{
		{
			name:   "decimal multiple",
			schema: `{"type":"number","multipleOf":0.1}`,
			value:  "0.3",
			valid:  true,
		},
		{
			name:     "exact decimal nonmultiple",
			schema:   `{"type":"number","multipleOf":0.1}`,
			value:    "0.30000000000000000001",
			valid:    false,
			failures: []string{"multipleOf"},
		},
		{
			name:   "integral decimal satisfies integer",
			schema: `{"type":"integer"}`,
			value:  "100e-2",
			valid:  true,
		},
		{
			name:     "fraction fails integer only",
			schema:   `{"type":"integer","multipleOf":0.1}`,
			value:    "9007199254740992.55",
			valid:    false,
			failures: []string{"type", "multipleOf"},
		},
		{
			name:   "large exact multiple",
			schema: `{"type":"number","multipleOf":3}`,
			value:  "9007199254740993",
			valid:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, test.schema, test.value)
			require.NoError(t, result.err)
			require.Equal(t, test.valid, result.valid)
			require.Equal(t, test.failures, failureRules(result.failures))
		})
	}
}

func TestEvaluateNumericFormatsUseNativeApplicabilityAndExactRanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   string
		value    string
		valid    bool
		observed []string
		failures []string
	}{
		{
			name:     "typeless int32 minimum",
			schema:   `{"format":"int32"}`,
			value:    "-2147483648",
			valid:    true,
			observed: []string{"number", "valid"},
		},
		{
			name:     "typeless int32 below range",
			schema:   `{"format":"int32"}`,
			value:    "-2147483649",
			valid:    false,
			observed: []string{"number"},
			failures: []string{"format"},
		},
		{
			name:     "typeless int32 above range",
			schema:   `{"format":"int32"}`,
			value:    "2147483648",
			valid:    false,
			observed: []string{"number"},
			failures: []string{"format"},
		},
		{
			name:     "typeless int32 fraction",
			schema:   `{"format":"int32"}`,
			value:    "1.5",
			valid:    false,
			observed: []string{"number"},
			failures: []string{"format"},
		},
		{
			name:     "typeless int64 minimum",
			schema:   `{"format":"int64"}`,
			value:    "-9223372036854775808",
			valid:    true,
			observed: []string{"number", "valid"},
		},
		{
			name:     "typeless int64 below range",
			schema:   `{"format":"int64"}`,
			value:    "-9223372036854775809",
			valid:    false,
			observed: []string{"number"},
			failures: []string{"format"},
		},
		{
			name:     "typeless int64 above range",
			schema:   `{"format":"int64"}`,
			value:    "9223372036854775808",
			valid:    false,
			observed: []string{"number"},
			failures: []string{"format"},
		},
		{
			name:     "float just below overflow cutoff",
			schema:   `{"format":"float"}`,
			value:    "340282356779733661637539395458142568447",
			valid:    true,
			observed: []string{"number", "valid"},
		},
		{
			name:     "float overflow cutoff",
			schema:   `{"format":"float"}`,
			value:    "340282356779733661637539395458142568448",
			valid:    false,
			observed: []string{"number"},
			failures: []string{"format"},
		},
		{
			name:     "float negative edge",
			schema:   `{"format":"float"}`,
			value:    "-340282356779733661637539395458142568447",
			valid:    true,
			observed: []string{"number", "valid"},
		},
		{
			name:     "float below range",
			schema:   `{"format":"float"}`,
			value:    "-340282356779733661637539395458142568448",
			valid:    false,
			observed: []string{"number"},
			failures: []string{"format"},
		},
		{
			name:     "double decimal edge",
			schema:   `{"format":"double"}`,
			value:    "1.7976931348623158e308",
			valid:    true,
			observed: []string{"number", "valid"},
		},
		{
			name:     "double overflow",
			schema:   `{"format":"double"}`,
			value:    "1.7976931348623159e308",
			valid:    false,
			observed: []string{"number"},
			failures: []string{"format"},
		},
		{
			name:     "double negative decimal edge",
			schema:   `{"format":"double"}`,
			value:    "-1.7976931348623158e308",
			valid:    true,
			observed: []string{"number", "valid"},
		},
		{
			name:     "double below range",
			schema:   `{"format":"double"}`,
			value:    "-1.7976931348623159e308",
			valid:    false,
			observed: []string{"number"},
			failures: []string{"format"},
		},
		{
			name:     "non-number is inapplicable",
			schema:   `{"format":"int32","minimum":1,"multipleOf":2}`,
			value:    `"text"`,
			valid:    true,
			observed: []string{"string"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, test.schema, test.value)
			require.NoError(t, result.err)
			require.Equal(t, test.valid, result.valid)
			require.Equal(t, test.observed, observedLevels(result.observed))
			require.Equal(t, test.failures, failureRules(result.failures))
		})
	}
}

func TestEvaluateNumericFailureIdentitiesAreDeterministic(t *testing.T) {
	t.Parallel()

	schema := `{"type":"number","minimum":2,"exclusiveMinimum":true,` +
		`"maximum":1,"exclusiveMaximum":true,"multipleOf":0.3,"format":"int32"}`
	first := evaluateSchemaValue(t, schema, "1.6")
	second := evaluateSchemaValue(t, schema, "1.6")

	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.Equal(t, first, second)
	require.False(t, first.valid)
	require.Equal(
		t,
		[]string{"type", "exclusiveMinimum", "exclusiveMaximum", "multipleOf", "format"},
		applicableRules(first.applicable),
	)
	require.Equal(
		t,
		[]string{"exclusiveMinimum", "exclusiveMaximum", "multipleOf", "format"},
		failureRules(first.failures),
	)
	require.Equal(
		t,
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|exclusiveMinimum",
		first.failures[0].String(),
	)
	require.Equal(
		t,
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|format",
		first.failures[3].String(),
	)
}

func TestEvaluateArrayCountsUseExistingArrayLengthAndIgnoreWrongKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		value      string
		valid      bool
		applicable []string
		observed   []string
		failures   []string
	}{
		{
			name:       "empty at both boundaries",
			schema:     `{"type":"array","minItems":0,"maxItems":0,"items":{"type":"string"}}`,
			value:      `[]`,
			valid:      true,
			applicable: []string{"type", "minItems", "maxItems"},
			observed:   []string{"array", "valid", "valid"},
		},
		{
			name:       "short below minimum",
			schema:     `{"type":"array","minItems":2,"maxItems":3,"items":{"type":"string"}}`,
			value:      `["text"]`,
			valid:      false,
			applicable: []string{"type", "minItems", "maxItems", "type"},
			observed:   []string{"array", "valid", "string"},
			failures:   []string{"minItems"},
		},
		{
			name:       "maximum boundary",
			schema:     `{"type":"array","minItems":1,"maxItems":2,"items":{"type":"string"}}`,
			value:      `["a","b"]`,
			valid:      true,
			applicable: []string{"type", "minItems", "maxItems", "type", "type"},
			observed:   []string{"array", "valid", "valid", "string", "string"},
		},
		{
			name:       "over maximum",
			schema:     `{"type":"array","maxItems":1,"items":{"type":"string"}}`,
			value:      `["a","b"]`,
			valid:      false,
			applicable: []string{"type", "maxItems", "type", "type"},
			observed:   []string{"array", "string", "string"},
			failures:   []string{"maxItems"},
		},
		{
			name:       "wrong kind",
			schema:     `{"type":"array","minItems":1,"maxItems":1,"items":{"type":"string"}}`,
			value:      `"text"`,
			valid:      false,
			applicable: []string{"type"},
			failures:   []string{"type"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, test.schema, test.value)
			require.NoError(t, result.err)
			require.Equal(t, test.valid, result.valid)
			require.Equal(t, test.applicable, applicableRules(result.applicable))
			require.Equal(t, test.observed, observedLevels(result.observed))
			require.Equal(t, test.failures, failureRules(result.failures))
		})
	}
}

func TestEvaluateArrayItemsOnlyExistingIndices(t *testing.T) {
	t.Parallel()

	result := evaluateSchemaValue(
		t,
		`{"type":"array","minItems":3,"items":{"type":"integer"}}`,
		`[1]`,
	)

	require.NoError(t, result.err)
	require.False(t, result.valid)
	require.Equal(
		t,
		[]string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|type",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|minItems",
			"#/paths/~1/post/requestBody/content/application~1json/schema/items|#/0|type",
		},
		identityStrings(result.applicable),
	)
	require.Equal(t, []string{"array", "number"}, observedLevels(result.observed))
	require.Equal(
		t,
		[]string{"#/paths/~1/post/requestBody/content/application~1json/schema|#|minItems"},
		identityStrings(result.failures),
	)
}

func TestEvaluateNestedArrayItemFailuresKeepNumericInstanceIdentity(t *testing.T) {
	t.Parallel()

	result := evaluateSchemaValue(
		t,
		`{"type":"array","items":{"type":"array","items":{"type":"integer"}}}`,
		`[[1,1.5],["text"]]`,
	)

	require.NoError(t, result.err)
	require.False(t, result.valid)
	require.Equal(
		t,
		[]string{
			"#/paths/~1/post/requestBody/content/application~1json/schema/items/items|#/0/1|type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/items/items|#/1/0|type",
		},
		identityStrings(result.failures),
	)
}

func TestEvaluateObjectRulesUseStableCountsRequirednessAndMemberOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		value      string
		valid      bool
		applicable []string
		observed   []string
		failures   []string
	}{
		{
			name: "wrong kind makes object rules inapplicable",
			schema: `{"type":"object","minProperties":1,"maxProperties":1,"required":["name"],` +
				`"properties":{"name":{"type":"string"}},"additionalProperties":false}`,
			value:      `"text"`,
			valid:      false,
			applicable: []string{"type"},
			observed:   nil,
			failures:   []string{"type"},
		},
		{
			name:       "missing required differs from supplied null",
			schema:     `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`,
			value:      `{}`,
			valid:      false,
			applicable: []string{"type", "required"},
			observed:   []string{"object"},
			failures:   []string{"required"},
		},
		{
			name:       "supplied null passes required and reaches child",
			schema:     `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`,
			value:      `{"name":null}`,
			valid:      false,
			applicable: []string{"type", "required", "type"},
			observed:   []string{"object", "present"},
			failures:   []string{"type"},
		},
		{
			name: "counts and closed extras are independently evaluated",
			schema: `{"type":"object","minProperties":2,"maxProperties":2,"required":["name"],` +
				`"properties":{"name":{"type":"string"}},"additionalProperties":false}`,
			value: `{"name":"ok","extra":true}`,
			valid: false,
			applicable: []string{
				"type", "minProperties", "maxProperties", "required", "type", "additionalProperties",
			},
			observed: []string{"object", "valid", "valid", "present", "string"},
			failures: []string{"additionalProperties"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, test.schema, test.value)
			require.NoError(t, result.err)
			require.Equal(t, test.valid, result.valid)
			require.Equal(t, test.applicable, applicableRules(result.applicable))
			require.Equal(t, test.observed, observedLevels(result.observed))
			require.Equal(t, test.failures, failureRules(result.failures))
		})
	}
}

func TestEvaluateObjectRulesSortRequiredAndAdditionalMembersByUTF8(t *testing.T) {
	t.Parallel()

	result := evaluateSchemaValue(
		t,
		`{"type":"object","required":["é","a"],"additionalProperties":false}`,
		`{"é":true,"z":true,"a":true}`,
	)

	require.NoError(t, result.err)
	require.False(t, result.valid)
	require.Equal(
		t,
		[]string{"type", "required", "required", "additionalProperties", "additionalProperties", "additionalProperties"},
		applicableRules(result.applicable),
	)
	require.Equal(t, []string{"object", "present", "present"}, observedLevels(result.observed))
	require.Equal(
		t,
		[]string{"additionalProperties", "additionalProperties", "additionalProperties"},
		failureRules(result.failures),
	)
	require.Equal(
		t,
		[]string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#/a|additionalProperties",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#/z|additionalProperties",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#/é|additionalProperties",
		},
		identityStrings(result.failures),
	)
}

func TestEvaluateObjectRulesValidateSuppliedReadOnlyWriteOnlyAndNestedValues(t *testing.T) {
	t.Parallel()

	result := evaluateSchemaValue(
		t,
		`{"type":"object","properties":{"read":{"type":"string","readOnly":true},`+
			`"write":{"type":"integer","writeOnly":true},"nested":{"type":"object",`+
			`"required":["child"],"properties":{"child":{"type":"boolean"}}}}}`,
		`{"read":1,"write":2,"nested":{"child":"wrong"}}`,
	)

	require.NoError(t, result.err)
	require.False(t, result.valid)
	require.Equal(
		t,
		[]string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/properties/nested|#/nested|type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/properties/nested|#/nested/child|required",
			"#/paths/~1/post/requestBody/content/application~1json/schema/properties/nested/" +
				"properties/child|#/nested/child|type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/properties/read|#/read|type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/properties/write|#/write|type",
		},
		identityStrings(result.applicable),
	)
	require.Equal(t, []string{"object", "object", "present", "number"}, observedLevels(result.observed))
	require.Equal(
		t,
		[]string{
			"#/paths/~1/post/requestBody/content/application~1json/schema/properties/nested/" +
				"properties/child|#/nested/child|type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/properties/read|#/read|type",
		},
		identityStrings(result.failures),
	)
}

func TestEvaluateObjectAdditionalSchemaUsesOnlySuppliedMembers(t *testing.T) {
	t.Parallel()

	result := evaluateSchemaValue(
		t,
		`{"type":"object","additionalProperties":{"type":"integer"}}`,
		`{"valid":1,"invalid":"text"}`,
	)

	require.NoError(t, result.err)
	require.False(t, result.valid)
	require.Equal(
		t,
		[]string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/additionalProperties|#/invalid|type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/additionalProperties|#/valid|type",
		},
		identityStrings(result.applicable),
	)
	require.Equal(t, []string{"object", "number"}, observedLevels(result.observed))
	require.Equal(
		t,
		[]string{"#/paths/~1/post/requestBody/content/application~1json/schema/additionalProperties|#/invalid|type"},
		identityStrings(result.failures),
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

func identityStrings(identities []ruleIdentity) []string {
	if len(identities) == 0 {
		return nil
	}

	result := make([]string, 0, len(identities))
	for _, identity := range identities {
		result = append(result, identity.String())
	}

	return result
}
