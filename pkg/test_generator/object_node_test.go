package testgenerator

import (
	"encoding/json"
	"testing"
	"time"

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
		`{"requiredNotNullableString":"valid-string","requiredNullableString":123}`,
		`{"requiredNotNullableString":"valid-string","requiredNullableString":true}`,
		`{"requiredNotNullableString":"valid-string","requiredNullableString":{}}`,
		`{"requiredNotNullableString":"valid-string","requiredNullableString":[]}`,
		`{"requiredNotNullableString":null,"requiredNullableString":"valid-string"}`,
		`{"requiredNotNullableString":123,"requiredNullableString":"valid-string"}`,
		`{"requiredNotNullableString":true,"requiredNullableString":"valid-string"}`,
		`{"requiredNotNullableString":{},"requiredNullableString":"valid-string"}`,
		`{"requiredNotNullableString":[],"requiredNullableString":"valid-string"}`,
		`{"optionalNotNullableString":null,"requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
		`{"optionalNotNullableString":123,"requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
		`{"optionalNotNullableString":true,"requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
		`{"optionalNotNullableString":{},"requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
		`{"optionalNotNullableString":[],"requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
		`{"optionalNullableString":123,"requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
		`{"optionalNullableString":true,"requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
		`{"optionalNullableString":{},"requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
		`{"optionalNullableString":[],"requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
		`{"` + additionalPropertyCaseKey + `":"not-allowed","requiredNotNullableString":"valid-string","requiredNullableString":"valid-string"}`,
	}, rawMessages(node.InvalidCases()))

}

func TestObjectNodeValidCasesIncludeEachRequiredPropertyValidCaseAdditively(t *testing.T) {
	node := ObjectNode{
		Required: []string{"left", "right"},
		AdditionalProperties: AdditionalPropertiesNode{
			Allowed: new(false),
		},
		Properties: map[string]SchemaNode{
			"left":  stringSchema(true),
			"right": stringSchema(true),
		},
	}

	require.Equal(t, []string{
		`{"left":"valid-string","right":"valid-string"}`,
		`{"left":null,"right":"valid-string"}`,
		`{"left":"valid-string","right":null}`,
	}, rawMessages(node.ValidCases()))
	require.NotContains(t, rawMessages(node.ValidCases()), `{"left":null,"right":null}`)
	require.Equal(t, []string{
		"required properties",
		"required properties",
		"required properties",
	}, caseNames(node.ValidCases()))
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
		"invalid property id number",
		"invalid property id boolean",
		"invalid property id object",
		"invalid property id array",
		"invalid property name null",
		"invalid property name number",
		"invalid property name boolean",
		"invalid property name object",
		"invalid property name array",
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

func TestObjectNodeRequiredValidCasesCatchRejectedNonBaselineRequiredVariant(t *testing.T) {
	node := ObjectNode{
		Required: []string{"id"},
		AdditionalProperties: AdditionalPropertiesNode{
			Allowed: new(false),
		},
		Properties: map[string]SchemaNode{
			"id": stringSchema(true),
		},
	}

	var rejected []string
	for _, testCase := range node.ValidCases() {
		object := rawObject(t, testCase.Value)
		if string(object["id"]) == "null" {
			rejected = append(rejected, string(testCase.Value))
		}
	}

	require.Equal(t, []string{`{"id":null}`}, rejected)
}

func TestObjectNodeOptionalCasesUseSingleRequiredBaseline(t *testing.T) {
	node := ObjectNode{
		Required: []string{"id"},
		AdditionalProperties: AdditionalPropertiesNode{
			Allowed: new(false),
		},
		Properties: map[string]SchemaNode{
			"id":       stringSchema(true),
			"optional": stringSchema(true),
		},
	}

	require.Subset(t, rawMessages(node.ValidCases()), []string{
		`{"id":"valid-string","optional":"valid-string"}`,
		`{"id":"valid-string","optional":null}`,
	})
	require.NotContains(t, rawMessages(node.ValidCases()), `{"id":null,"optional":"valid-string"}`)
	require.NotContains(t, rawMessages(node.ValidCases()), `{"id":null,"optional":null}`)
}

func TestObjectNodeRequiredObjectCasesAreAdditive(t *testing.T) {
	node := ObjectNode{
		Required: []string{"text", "flag", "count"},
		AdditionalProperties: AdditionalPropertiesNode{
			Allowed: new(false),
		},
		Properties: map[string]SchemaNode{
			"text":  stringSchema(true),
			"flag":  boolSchema(true),
			"count": numberSchema(true),
		},
	}

	start := time.Now()
	requiredNames, _, _, requiredObjects := node.objectCaseData()
	elapsed := time.Since(start)

	validCaseCounts := make(map[string]int, len(requiredNames))
	cartesianRequiredObjects := 1
	additiveRequiredObjects := 1
	for _, name := range requiredNames {
		schema := node.Properties[name]
		validCaseCount := len(schema.ValidCases())
		validCaseCounts[name] = validCaseCount
		cartesianRequiredObjects *= validCaseCount
		additiveRequiredObjects += validCaseCount - 1
	}

	t.Logf(
		"required valid case counts=%v cartesian required objects=%d additive required objects=%d generated required objects=%d elapsed=%s",
		validCaseCounts,
		cartesianRequiredObjects,
		additiveRequiredObjects,
		len(requiredObjects),
		elapsed,
	)

	require.Equal(t, []string{"text", "flag", "count"}, requiredNames)
	require.Equal(t, 66, cartesianRequiredObjects)
	require.Equal(t, 14, additiveRequiredObjects)
	require.Len(t, requiredObjects, additiveRequiredObjects)
}

func TestObjectNodeRequiredObjectsDoNotAlias(t *testing.T) {
	node := ObjectNode{
		Required: []string{"left", "right"},
		AdditionalProperties: AdditionalPropertiesNode{
			Allowed: new(false),
		},
		Properties: map[string]SchemaNode{
			"left":  stringSchema(true),
			"right": stringSchema(true),
		},
	}

	_, _, _, requiredObjects := node.objectCaseData()
	require.Len(t, requiredObjects, 3)

	before := string(requiredObjects[1]["right"])
	requiredObjects[0]["right"] = json.RawMessage(`"changed"`)

	require.Equal(t, `"valid-string"`, before)
	require.Equal(t, before, string(requiredObjects[1]["right"]))
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

func boolSchema(nullable bool) SchemaNode {
	return SchemaNode{
		Type: "boolean",
		Bool: &BoolNode{
			BaseNode: BaseNode{Nullable: nullable},
		},
	}
}

func numberSchema(nullable bool) SchemaNode {
	return SchemaNode{
		Type: "number",
		Number: &NumberNode{
			BaseNode: BaseNode{Nullable: nullable},
		},
	}
}
