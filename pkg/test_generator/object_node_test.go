package testgenerator

import (
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
		`{"requiredNullableString":"valid-string","requiredNotNullableString":"valid-string"}`,
		`{"requiredNullableString":null,"requiredNotNullableString":"valid-string"}`,
		`{"requiredNullableString":"valid-string","requiredNotNullableString":"valid-string","optionalNotNullableString":"valid-string"}`,
		`{"requiredNullableString":"valid-string","requiredNotNullableString":"valid-string","optionalNullableString":"valid-string"}`,
		`{"requiredNullableString":"valid-string","requiredNotNullableString":"valid-string","optionalNullableString":null}`,
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
		`{"requiredNullableString":"valid-string","requiredNotNullableString":null}`,
		`{"requiredNullableString":"valid-string","requiredNotNullableString":"valid-string","optionalNotNullableString":null}`,
		`{"requiredNullableString":"valid-string","requiredNotNullableString":"valid-string","extra":"not-allowed"}`,
	}, rawMessages(node.InvalidCases()))

	invalidPropertyCase := findCaseByRawMessage(t, node.InvalidCases(), `{"requiredNullableString":"valid-string","requiredNotNullableString":null}`)
	require.Contains(t, invalidPropertyCase.RequiredValid, "requiredNullableString")
	require.Contains(t, invalidPropertyCase.RequiredInvalid, "requiredNotNullableString")
}

func TestObjectNodeAdditionalPropertiesDefaultAllowsExtraProperties(t *testing.T) {
	node := ObjectNode{
		Properties: map[string]SchemaNode{},
	}

	require.Contains(t, rawMessages(node.ValidCases()), `{"extra":"additional-property"}`)
	require.NotContains(t, rawMessages(node.InvalidCases()), `{"extra":"not-allowed"}`)
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

	require.Contains(t, rawMessages(node.ValidCases()), `{"id":"valid-string","extra":"valid-string"}`)
	require.Contains(t, rawMessages(node.InvalidCases()), `{"id":"valid-string","extra":null}`)
}

func rawMessages(cases []Case) []string {
	rawMessages := make([]string, 0, len(cases))
	for _, testCase := range cases {
		rawMessages = append(rawMessages, string(testCase.GenerateValid(nil, nil)))
	}

	return rawMessages
}

func findCaseByRawMessage(t *testing.T, cases []Case, rawMessage string) Case {
	t.Helper()

	for _, testCase := range cases {
		if string(testCase.GenerateValid(nil, nil)) == rawMessage {
			return testCase
		}
	}

	require.Failf(t, "case not found", "case with raw message %s was not found", rawMessage)
	return Case{}
}

func stringSchema(nullable bool) SchemaNode {
	return SchemaNode{
		Type: "string",
		String: &StringNode{
			BaseNode: BaseNode{Nullable: nullable},
		},
	}
}
