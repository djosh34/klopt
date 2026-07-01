package testgenerator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchemaNodeMergeObjectsCombinesRequiredPropertiesAndAdditionalProperties(t *testing.T) {
	left := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			BaseNode: BaseNode{Nullable: true},
			Required: []string{
				"first",
				"shared",
			},
			AdditionalProperties: AdditionalPropertiesNode{Allowed: new(true)},
			Properties: map[string]SchemaNode{
				"first":  stringSchema(false),
				"shared": stringSchema(true),
			},
		},
	}
	right := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			BaseNode:             BaseNode{Nullable: false},
			Required:             []string{"second"},
			AdditionalProperties: AdditionalPropertiesNode{Allowed: new(false)},
			Properties: map[string]SchemaNode{
				"second": boolSchema(false),
				"shared": stringSchema(false),
			},
		},
	}

	merged, err := left.Merge(right)
	require.NoError(t, err)

	require.Equal(t, "object", merged.Type)
	require.NotNil(t, merged.Object)
	require.Equal(t, BaseNode{Nullable: false}, merged.Object.BaseNode)
	require.ElementsMatch(t, []string{"first", "shared", "second"}, merged.Object.Required)
	require.NotNil(t, merged.Object.AdditionalProperties.Allowed)
	require.False(t, *merged.Object.AdditionalProperties.Allowed)
	require.Contains(t, merged.Object.Properties, "first")
	require.Contains(t, merged.Object.Properties, "second")

	shared := merged.Object.Properties["shared"]
	require.Equal(t, "string", shared.Type)
	require.NotNil(t, shared.String)
	require.Equal(t, BaseNode{Nullable: false}, shared.String.BaseNode)

	require.Equal(t, BaseNode{Nullable: true}, left.Object.Properties["shared"].String.BaseNode)
}

func TestSchemaNodeMergeObjectAdditionalPropertiesSchemas(t *testing.T) {
	leftAdditionalProperties := stringSchema(true)
	rightAdditionalProperties := stringSchema(false)
	left := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			AdditionalProperties: AdditionalPropertiesNode{
				Schema: &leftAdditionalProperties,
			},
		},
	}
	right := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			AdditionalProperties: AdditionalPropertiesNode{
				Schema: &rightAdditionalProperties,
			},
		},
	}

	merged, err := left.Merge(right)
	require.NoError(t, err)

	require.NotNil(t, merged.Object.AdditionalProperties.Schema)
	require.Equal(t, "string", merged.Object.AdditionalProperties.Schema.Type)
	require.NotNil(t, merged.Object.AdditionalProperties.Schema.String)
	require.Equal(t, BaseNode{Nullable: false}, merged.Object.AdditionalProperties.Schema.String.BaseNode)
	require.Equal(t, BaseNode{Nullable: true}, left.Object.AdditionalProperties.Schema.String.BaseNode)
}

func TestSchemaNodeMergeObjectAdditionalPropertiesFalseWinsOverSchema(t *testing.T) {
	additionalProperties := stringSchema(false)
	left := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			AdditionalProperties: AdditionalPropertiesNode{Allowed: new(false)},
		},
	}
	right := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			AdditionalProperties: AdditionalPropertiesNode{
				Schema: &additionalProperties,
			},
		},
	}

	merged, err := left.Merge(right)
	require.NoError(t, err)

	require.NotNil(t, merged.Object.AdditionalProperties.Allowed)
	require.False(t, *merged.Object.AdditionalProperties.Allowed)
	require.Nil(t, merged.Object.AdditionalProperties.Schema)
}

func TestSchemaNodeMergeObjectAdditionalPropertiesFalseWinsEvenWhenNodeAlsoHasSchema(t *testing.T) {
	additionalProperties := stringSchema(false)
	left := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			AdditionalProperties: AdditionalPropertiesNode{
				Allowed: new(false),
				Schema:  &additionalProperties,
			},
		},
	}
	right := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			AdditionalProperties: AdditionalPropertiesNode{Allowed: new(true)},
		},
	}

	merged, err := left.Merge(right)
	require.NoError(t, err)

	require.NotNil(t, merged.Object.AdditionalProperties.Allowed)
	require.False(t, *merged.Object.AdditionalProperties.Allowed)
	require.Nil(t, merged.Object.AdditionalProperties.Schema)
}

func TestSchemaNodeMergeObjectsDeduplicatesRequiredProperties(t *testing.T) {
	left := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			Required: []string{"id", "id", "shared"},
			Properties: map[string]SchemaNode{
				"id":     stringSchema(false),
				"shared": boolSchema(false),
			},
		},
	}
	right := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			Required: []string{"shared", "right", "right"},
			Properties: map[string]SchemaNode{
				"right": numberSchema(false),
			},
		},
	}

	merged, err := left.Merge(right)
	require.NoError(t, err)

	require.Equal(t, []string{"id", "shared", "right"}, merged.Object.Required)
}

func TestSchemaNodeMergeObjectsRecursivelyMergesNestedObjectProperties(t *testing.T) {
	left := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			Properties: map[string]SchemaNode{
				"child": {
					Type: "object",
					Object: &ObjectNode{
						BaseNode: BaseNode{Nullable: true},
						Required: []string{"name"},
						Properties: map[string]SchemaNode{
							"name": stringSchema(true),
						},
					},
				},
			},
		},
	}
	right := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			Properties: map[string]SchemaNode{
				"child": {
					Type: "object",
					Object: &ObjectNode{
						BaseNode:             BaseNode{Nullable: false},
						Required:             []string{"enabled"},
						AdditionalProperties: AdditionalPropertiesNode{Allowed: new(false)},
						Properties: map[string]SchemaNode{
							"enabled": boolSchema(false),
							"name":    stringSchema(false),
						},
					},
				},
			},
		},
	}

	merged, err := left.Merge(right)
	require.NoError(t, err)

	child := merged.Object.Properties["child"]
	require.Equal(t, "object", child.Type)
	require.NotNil(t, child.Object)
	require.Equal(t, BaseNode{Nullable: false}, child.Object.BaseNode)
	require.Equal(t, []string{"name", "enabled"}, child.Object.Required)
	require.Equal(t, BaseNode{Nullable: false}, child.Object.Properties["name"].String.BaseNode)
	require.NotNil(t, child.Object.AdditionalProperties.Allowed)
	require.False(t, *child.Object.AdditionalProperties.Allowed)
}

func TestSchemaNodeMergeObjectsRecursivelyMergesArrayPropertyItems(t *testing.T) {
	left := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			Properties: map[string]SchemaNode{
				"tags": {
					Type: "array",
					Array: &ArrayNode{
						BaseNode: BaseNode{Nullable: true},
						Items:    stringSchema(true),
					},
				},
			},
		},
	}
	right := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			Properties: map[string]SchemaNode{
				"tags": {
					Type: "array",
					Array: &ArrayNode{
						BaseNode: BaseNode{Nullable: false},
						Items:    stringSchema(false),
					},
				},
			},
		},
	}

	merged, err := left.Merge(right)
	require.NoError(t, err)

	tags := merged.Object.Properties["tags"]
	require.Equal(t, "array", tags.Type)
	require.NotNil(t, tags.Array)
	require.Equal(t, BaseNode{Nullable: false}, tags.Array.BaseNode)
	require.Equal(t, BaseNode{Nullable: false}, tags.Array.Items.String.BaseNode)
}

func TestSchemaNodeMergeObjectCasesUseMergedRequiredAndPropertyRules(t *testing.T) {
	left := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			Required: []string{"name"},
			Properties: map[string]SchemaNode{
				"name": stringSchema(true),
			},
		},
	}
	right := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			Required:             []string{"enabled"},
			AdditionalProperties: AdditionalPropertiesNode{Allowed: new(false)},
			Properties: map[string]SchemaNode{
				"enabled": boolSchema(false),
				"name":    stringSchema(false),
			},
		},
	}

	merged, err := left.Merge(right)
	require.NoError(t, err)

	require.Contains(t, rawMessages(merged.ValidCases()), `{"enabled":true,"name":"valid-string"}`)
	require.NotContains(t, rawMessages(merged.ValidCases()), `{"enabled":true,"name":null}`)
	require.Contains(t, rawMessages(merged.InvalidCases()), `{"enabled":true,"name":null}`)
	require.Contains(t, rawMessages(merged.InvalidCases()), `{"name":"valid-string"}`)
	require.Contains(t, rawMessages(merged.InvalidCases()), `{"enabled":true}`)
	require.Contains(t, rawMessages(merged.InvalidCases()), `{"`+additionalPropertyCaseKey+`":"not-allowed","enabled":true,"name":"valid-string"}`)
}

func TestSchemaNodeMergeObjectAdditionalPropertiesSchemaCasesUseMergedSchema(t *testing.T) {
	leftAdditionalProperties := stringSchema(true)
	rightAdditionalProperties := stringSchema(false)
	left := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			AdditionalProperties: AdditionalPropertiesNode{
				Schema: &leftAdditionalProperties,
			},
		},
	}
	right := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			AdditionalProperties: AdditionalPropertiesNode{
				Schema: &rightAdditionalProperties,
			},
		},
	}

	merged, err := left.Merge(right)
	require.NoError(t, err)

	require.Contains(t, rawMessages(merged.ValidCases()), `{"`+additionalPropertyCaseKey+`":"valid-string"}`)
	require.NotContains(t, rawMessages(merged.ValidCases()), `{"`+additionalPropertyCaseKey+`":null}`)
	require.Contains(t, rawMessages(merged.InvalidCases()), `{"`+additionalPropertyCaseKey+`":null}`)
}

func TestSchemaNodeMergeObjectAdditionalPropertiesCombinations(t *testing.T) {
	stringAdditionalProperties := stringSchema(false)

	for name, tt := range map[string]struct {
		left  AdditionalPropertiesNode
		right AdditionalPropertiesNode
		check func(t *testing.T, merged AdditionalPropertiesNode)
	}{
		"implicit and implicit stays implicit": {
			check: func(t *testing.T, merged AdditionalPropertiesNode) {
				t.Helper()

				require.Nil(t, merged.Allowed)
				require.Nil(t, merged.Schema)
			},
		},
		"true and implicit stays true": {
			left: AdditionalPropertiesNode{Allowed: new(true)},
			check: func(t *testing.T, merged AdditionalPropertiesNode) {
				t.Helper()

				require.NotNil(t, merged.Allowed)
				require.True(t, *merged.Allowed)
				require.Nil(t, merged.Schema)
			},
		},
		"schema and implicit keeps schema": {
			left: AdditionalPropertiesNode{Schema: &stringAdditionalProperties},
			check: func(t *testing.T, merged AdditionalPropertiesNode) {
				t.Helper()

				require.Nil(t, merged.Allowed)
				require.NotNil(t, merged.Schema)
				require.Equal(t, "string", merged.Schema.Type)
			},
		},
		"schema and true keeps schema": {
			left:  AdditionalPropertiesNode{Schema: &stringAdditionalProperties},
			right: AdditionalPropertiesNode{Allowed: new(true)},
			check: func(t *testing.T, merged AdditionalPropertiesNode) {
				t.Helper()

				require.Nil(t, merged.Allowed)
				require.NotNil(t, merged.Schema)
				require.Equal(t, "string", merged.Schema.Type)
			},
		},
		"true and false becomes false": {
			left:  AdditionalPropertiesNode{Allowed: new(true)},
			right: AdditionalPropertiesNode{Allowed: new(false)},
			check: func(t *testing.T, merged AdditionalPropertiesNode) {
				t.Helper()

				require.NotNil(t, merged.Allowed)
				require.False(t, *merged.Allowed)
				require.Nil(t, merged.Schema)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			left := SchemaNode{
				Type: "object",
				Object: &ObjectNode{
					AdditionalProperties: tt.left,
				},
			}
			right := SchemaNode{
				Type: "object",
				Object: &ObjectNode{
					AdditionalProperties: tt.right,
				},
			}

			merged, err := left.Merge(right)
			require.NoError(t, err)

			tt.check(t, merged.Object.AdditionalProperties)
		})
	}
}

func TestSchemaNodeMergeArraysMergesItems(t *testing.T) {
	left := SchemaNode{
		Type: "array",
		Array: &ArrayNode{
			BaseNode: BaseNode{Nullable: true},
			Items:    stringSchema(true),
		},
	}
	right := SchemaNode{
		Type: "array",
		Array: &ArrayNode{
			BaseNode: BaseNode{Nullable: false},
			Items:    stringSchema(false),
		},
	}

	merged, err := left.Merge(right)
	require.NoError(t, err)

	require.Equal(t, "array", merged.Type)
	require.NotNil(t, merged.Array)
	require.Equal(t, BaseNode{Nullable: false}, merged.Array.BaseNode)
	require.Equal(t, "string", merged.Array.Items.Type)
	require.NotNil(t, merged.Array.Items.String)
	require.Equal(t, BaseNode{Nullable: false}, merged.Array.Items.String.BaseNode)
}

func TestSchemaNodeMergePrimitiveSchemasKeepsIntersectedNullableAndStringFormat(t *testing.T) {
	for name, tt := range map[string]struct {
		left  SchemaNode
		right SchemaNode
		check func(t *testing.T, merged SchemaNode)
	}{
		"string": {
			left: SchemaNode{
				Type:   "string",
				String: &StringNode{BaseNode: BaseNode{Nullable: true}},
			},
			right: SchemaNode{
				Type: "string",
				String: &StringNode{
					BaseNode: BaseNode{Nullable: false},
					Format:   "date-time",
				},
			},
			check: func(t *testing.T, merged SchemaNode) {
				t.Helper()

				require.Equal(t, "string", merged.Type)
				require.NotNil(t, merged.String)
				require.Equal(t, BaseNode{Nullable: false}, merged.String.BaseNode)
				require.Equal(t, "date-time", merged.String.Format)
			},
		},
		"number": {
			left:  numberSchema(true),
			right: numberSchema(false),
			check: func(t *testing.T, merged SchemaNode) {
				t.Helper()

				require.Equal(t, "number", merged.Type)
				require.NotNil(t, merged.Number)
				require.Equal(t, BaseNode{Nullable: false}, merged.Number.BaseNode)
			},
		},
		"boolean": {
			left:  boolSchema(true),
			right: boolSchema(false),
			check: func(t *testing.T, merged SchemaNode) {
				t.Helper()

				require.Equal(t, "boolean", merged.Type)
				require.NotNil(t, merged.Bool)
				require.Equal(t, BaseNode{Nullable: false}, merged.Bool.BaseNode)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			merged, err := tt.left.Merge(tt.right)
			require.NoError(t, err)

			tt.check(t, merged)
		})
	}
}

func TestSchemaNodeMergeRejectsDifferentTypes(t *testing.T) {
	_, err := stringSchema(false).Merge(numberSchema(false))
	require.ErrorContains(t, err, `cannot merge schema type "string" with "number"`)
}

func TestSchemaNodeMergeRejectsMalformedSameTypeSchemas(t *testing.T) {
	for name, tt := range map[string]struct {
		left  SchemaNode
		right SchemaNode
		want  string
	}{
		"array": {
			left:  SchemaNode{Type: "array"},
			right: SchemaNode{Type: "array", Array: &ArrayNode{}},
			want:  "array schema is missing array node",
		},
		"boolean": {
			left:  SchemaNode{Type: "boolean"},
			right: SchemaNode{Type: "boolean", Bool: &BoolNode{}},
			want:  "boolean schema is missing boolean node",
		},
		"number": {
			left:  SchemaNode{Type: "number"},
			right: SchemaNode{Type: "number", Number: &NumberNode{}},
			want:  "number schema is missing number node",
		},
		"object": {
			left:  SchemaNode{Type: "object"},
			right: SchemaNode{Type: "object", Object: &ObjectNode{}},
			want:  "object schema is missing object node",
		},
		"string": {
			left:  SchemaNode{Type: "string"},
			right: SchemaNode{Type: "string", String: &StringNode{}},
			want:  "string schema is missing string node",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := tt.left.Merge(tt.right)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestSchemaNodeMergeRejectsUnsupportedSameTypeSchema(t *testing.T) {
	_, err := SchemaNode{Type: "integer"}.Merge(SchemaNode{Type: "integer"})
	require.ErrorContains(t, err, `unsupported schema type "integer"`)
}

func TestSchemaNodeMergeRejectsStringFormatConflict(t *testing.T) {
	left := SchemaNode{
		Type: "string",
		String: &StringNode{
			Format: "date-time",
		},
	}
	right := SchemaNode{
		Type: "string",
		String: &StringNode{
			Format: "uuid",
		},
	}

	_, err := left.Merge(right)
	require.ErrorContains(t, err, `cannot merge string format "date-time" with "uuid"`)
}

func TestSchemaNodeMergeRejectsArrayItemConflict(t *testing.T) {
	left := SchemaNode{
		Type: "array",
		Array: &ArrayNode{
			Items: stringSchema(false),
		},
	}
	right := SchemaNode{
		Type: "array",
		Array: &ArrayNode{
			Items: boolSchema(false),
		},
	}

	_, err := left.Merge(right)
	require.ErrorContains(t, err, "array items")
	require.ErrorContains(t, err, `cannot merge schema type "string" with "boolean"`)
}

func TestSchemaNodeMergeRejectsPropertyTypeConflict(t *testing.T) {
	left := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			Properties: map[string]SchemaNode{
				"same": stringSchema(false),
			},
		},
	}
	right := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			Properties: map[string]SchemaNode{
				"same": numberSchema(false),
			},
		},
	}

	_, err := left.Merge(right)
	require.ErrorContains(t, err, `property "same"`)
	require.ErrorContains(t, err, `cannot merge schema type "string" with "number"`)
}

func TestSchemaNodeMergeRejectsNestedPropertyTypeConflict(t *testing.T) {
	left := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			Properties: map[string]SchemaNode{
				"child": {
					Type: "object",
					Object: &ObjectNode{
						Properties: map[string]SchemaNode{
							"same": stringSchema(false),
						},
					},
				},
			},
		},
	}
	right := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			Properties: map[string]SchemaNode{
				"child": {
					Type: "object",
					Object: &ObjectNode{
						Properties: map[string]SchemaNode{
							"same": numberSchema(false),
						},
					},
				},
			},
		},
	}

	_, err := left.Merge(right)
	require.ErrorContains(t, err, `property "child"`)
	require.ErrorContains(t, err, `property "same"`)
	require.ErrorContains(t, err, `cannot merge schema type "string" with "number"`)
}

func TestSchemaNodeMergeRejectsAdditionalPropertiesSchemaConflict(t *testing.T) {
	leftAdditionalProperties := stringSchema(false)
	rightAdditionalProperties := numberSchema(false)
	left := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			AdditionalProperties: AdditionalPropertiesNode{
				Schema: &leftAdditionalProperties,
			},
		},
	}
	right := SchemaNode{
		Type: "object",
		Object: &ObjectNode{
			AdditionalProperties: AdditionalPropertiesNode{
				Schema: &rightAdditionalProperties,
			},
		},
	}

	_, err := left.Merge(right)
	require.ErrorContains(t, err, "additionalProperties")
	require.ErrorContains(t, err, `cannot merge schema type "string" with "number"`)
}
