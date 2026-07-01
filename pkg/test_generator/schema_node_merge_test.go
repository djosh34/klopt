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
