package testgenerator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestObjectNodeValidCasesReachBaseRequiredAndOptionalPropertyCases(t *testing.T) {
	node := ObjectNode{
		BaseNode: BaseNode{Nullable: true},
		Required: []string{
			"requiredNullableString",
			"requiredNotNullableString",
		},
		AdditionalProperties: AdditionalPropertiesNode{Allowed: new(false)},
		Properties: map[string]SchemaNode{
			"optionalNotNullableString": stringSchema(false),
			"optionalNullableString":    stringSchema(true),
			"requiredNotNullableString": stringSchema(false),
			"requiredNullableString":    stringSchema(true),
		},
	}

	require.Equal(t, []string{
		`null`,
		`{"requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
		`{"requiredNotNullableString":"valid-string","requiredNullableString":null}`,
		`{"optionalNotNullableString":"valid-string","requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
		`{"optionalNullableString":"valid-string","requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
		`{"optionalNullableString":null,"requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
	}, rawMessages(node.ValidCases()))
}

func TestObjectNodeInvalidCasesReachShapeMissingAdditionalAndPropertyCases(t *testing.T) {
	node := ObjectNode{
		BaseNode: BaseNode{Nullable: true},
		Required: []string{
			"requiredNullableString",
			"requiredNotNullableString",
		},
		AdditionalProperties: AdditionalPropertiesNode{Allowed: new(false)},
		Properties: map[string]SchemaNode{
			"optionalNotNullableString": stringSchema(false),
			"optionalNullableString":    stringSchema(true),
			"requiredNotNullableString": stringSchema(false),
			"requiredNullableString":    stringSchema(true),
		},
	}

	require.Equal(t, []string{
		`"not-object"`,
		`123`,
		`true`,
		`[]`,
		`{}`,
		`{"requiredNotNullableString":"valid-string"}`,
		`{"requiredNullableString":"valid-string"}`,
		`{"requiredNotNullableString":null,"requiredNullableString":"valid-string"}`,
		`{"optionalNotNullableString":null,"requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
		`{"` + additionalPropertyCaseKey + `":"not-allowed","requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
	}, rawMessages(node.InvalidCases()))

}

func TestObjectNodeAdditionalPropertiesDefaultAllowsExtraProperties(t *testing.T) {
	node := ObjectNode{
		Properties: map[string]SchemaNode{},
	}

	require.Contains(t, rawMessages(node.ValidCases()), `{"`+additionalPropertyCaseKey+`":"additional-property"}`)
	require.NotContains(t, rawMessages(node.InvalidCases()), `{"`+additionalPropertyCaseKey+`":"not-allowed"}`)
}

func TestObjectNodeAdditionalPropertiesSchemaCases(t *testing.T) {
	additionalPropertySchema := stringSchema(false)
	node := ObjectNode{
		Required: []string{"id"},
		AdditionalProperties: AdditionalPropertiesNode{
			Schema: &additionalPropertySchema,
		},
		Properties: map[string]SchemaNode{
			"id": stringSchema(false),
		},
	}

	require.Contains(t, rawMessages(node.ValidCases()), `{"`+additionalPropertyCaseKey+`":"valid-string","id":"valid-string"}`)
	require.Contains(t, rawMessages(node.InvalidCases()), `{"`+additionalPropertyCaseKey+`":null,"id":"valid-string"}`)
}

func TestObjectNodeCasesHaveNames(t *testing.T) {
	additionalPropertySchema := stringSchema(false)
	node := ObjectNode{
		Required: []string{"id"},
		AdditionalProperties: AdditionalPropertiesNode{
			Schema: &additionalPropertySchema,
		},
		Properties: map[string]SchemaNode{
			"id":    stringSchema(false),
			"label": stringSchema(false),
		},
	}

	cases := append(node.ValidCases(), node.InvalidCases()...)
	for _, testCase := range cases {
		require.NotEmpty(t, testCase.Name)
		require.NotEmpty(t, testCase.Value)
	}

	require.Contains(t, caseNames(node.ValidCases()), "required properties")
	require.Contains(t, caseNames(node.ValidCases()), "optional property label string")
	require.Contains(t, caseNames(node.ValidCases()), "additional property string")
	require.Contains(t, caseNames(node.InvalidCases()), "invalid property id null")
	require.Contains(t, caseNames(node.InvalidCases()), "invalid property label null")
	require.Contains(t, caseNames(node.InvalidCases()), "invalid additional property null")
}

func TestObjectNodeInvalidCasesDeduplicateRequiredNames(t *testing.T) {
	node := ObjectNode{
		Required: []string{"id", "id", "name"},
		Properties: map[string]SchemaNode{
			"id":   stringSchema(false),
			"name": stringSchema(false),
		},
	}

	require.Equal(t, []string{
		"null",
		"string",
		"number",
		"boolean",
		"array",
		"missing required properties",
		"missing required property id",
		"missing required property name",
		"invalid property id null",
		"invalid property name null",
	}, caseNames(node.InvalidCases()))
}

func TestObjectNodeAdditionalPropertyKeyDoesNotOverwriteExistingProperty(t *testing.T) {
	node := ObjectNode{
		Required: []string{additionalPropertyCaseKey},
		Properties: map[string]SchemaNode{
			additionalPropertyCaseKey: stringSchema(false),
		},
	}

	additionalPropertyCase := findCaseByName(t, node.ValidCases(), "additional property")
	require.Equal(t, map[string]json.RawMessage{
		additionalPropertyCaseKey:        json.RawMessage(`"valid-string"`),
		additionalPropertyCaseKey + "_2": json.RawMessage(`"additional-property"`),
	}, rawObject(t, additionalPropertyCase.Value))
}

func rawMessages(cases []Case) []string {
	rawMessages := make([]string, 0, len(cases))
	for _, testCase := range cases {
		rawMessages = append(rawMessages, string(testCase.Value))
	}

	return rawMessages
}

func caseNames(cases []Case) []string {
	names := make([]string, 0, len(cases))
	for _, testCase := range cases {
		names = append(names, testCase.Name)
	}

	return names
}

func findCaseByName(t *testing.T, cases []Case, name string) Case {
	t.Helper()

	for _, testCase := range cases {
		if testCase.Name == name {
			return testCase
		}
	}

	require.Failf(t, "case not found", "case with name %q was not found", name)
	return Case{}
}

func rawObject(t *testing.T, value json.RawMessage) map[string]json.RawMessage {
	t.Helper()

	var object map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(value, &object))

	return object
}

func stringSchema(nullable bool) SchemaNode {
	return SchemaNode{
		Type: "string",
		String: &StringNode{
			BaseNode: BaseNode{Nullable: nullable},
		},
	}
}
