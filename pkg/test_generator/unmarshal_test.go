package testgenerator

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestNullableObjectNodeHasOneNullableStringProperty(t *testing.T) {
	openapiPath := testdataOpenAPIPath("nullable_object_with_nullable_string.yaml")

	content, err := os.ReadFile(openapiPath)
	require.NoError(t, err)

	var doc struct {
		Paths map[string]struct {
			Post *struct {
				OperationID string `yaml:"operationId"`
				RequestBody struct {
					Required bool `yaml:"required"`
					Content  map[string]struct {
						Schema SchemaNode `yaml:"schema"`
					} `yaml:"content"`
				} `yaml:"requestBody"`
			} `yaml:"post"`
		} `yaml:"paths"`
	}
	err = yaml.Unmarshal(content, &doc)
	require.NoError(t, err)

	require.Len(t, doc.Paths, 1)

	path := doc.Paths["/nullable-object-keys-additional-properties-false"]
	require.NotNil(t, path.Post)
	require.Equal(t, "nullableObjectKeysAdditionalPropertiesFalse", path.Post.OperationID)
	require.True(t, path.Post.RequestBody.Required)

	mediaTypeNode := path.Post.RequestBody.Content["application/json"]

	objectNode := mediaTypeNode.Schema.Object
	require.Equal(t, "object", mediaTypeNode.Schema.Type)
	require.NotNil(t, objectNode)
	require.Nil(t, mediaTypeNode.Schema.String)

	require.Equal(t, BaseNode{Nullable: true}, objectNode.BaseNode)
	require.Equal(t, []string{"requiredNullableString"}, objectNode.Required)
	require.NotNil(t, objectNode.AdditionalProperties.Allowed)
	require.False(t, *objectNode.AdditionalProperties.Allowed)
	require.Nil(t, objectNode.AdditionalProperties.Schema)
	require.Len(t, objectNode.Properties, 1)

	propertySchema := objectNode.Properties["requiredNullableString"]
	require.Equal(t, "string", propertySchema.Type)

	propertyNode := propertySchema.String
	require.NotNil(t, propertyNode)
	require.Nil(t, propertySchema.Object)

	require.Equal(t, BaseNode{Nullable: true}, propertyNode.BaseNode)
}

func TestAdditionalPropertiesNodeCanBeSchema(t *testing.T) {
	content := []byte(`
type: object
additionalProperties:
  type: string
  nullable: true
`)

	var objectNode ObjectNode
	err := yaml.Unmarshal(content, &objectNode)
	require.NoError(t, err)

	require.Nil(t, objectNode.AdditionalProperties.Allowed)
	require.NotNil(t, objectNode.AdditionalProperties.Schema)

	additionalPropertiesSchema := objectNode.AdditionalProperties.Schema
	require.Equal(t, "string", additionalPropertiesSchema.Type)

	additionalPropertiesNode := additionalPropertiesSchema.String
	require.NotNil(t, additionalPropertiesNode)
	require.Equal(t, BaseNode{Nullable: true}, additionalPropertiesNode.BaseNode)
}

func TestSchemaNodeUnmarshalYAMLDispatchesString(t *testing.T) {
	content := []byte(`
type: string
nullable: true
format: date-time
`)

	var schemaNode SchemaNode
	err := yaml.Unmarshal(content, &schemaNode)
	require.NoError(t, err)

	require.Equal(t, "string", schemaNode.Type)
	require.Nil(t, schemaNode.Object)
	require.NotNil(t, schemaNode.String)
	require.Equal(t, BaseNode{Nullable: true}, schemaNode.String.BaseNode)
	require.Equal(t, "date-time", schemaNode.String.Format)
}

func TestSchemaNodeUnmarshalYAMLDispatchesArray(t *testing.T) {
	content := []byte(`
type: array
nullable: true
items:
  type: string
  nullable: false
`)

	var schemaNode SchemaNode
	err := yaml.Unmarshal(content, &schemaNode)
	require.NoError(t, err)

	require.Equal(t, "array", schemaNode.Type)
	require.NotNil(t, schemaNode.Array)
	require.Equal(t, BaseNode{Nullable: true}, schemaNode.Array.BaseNode)
	require.Equal(t, "string", schemaNode.Array.Items.Type)
	require.NotNil(t, schemaNode.Array.Items.String)
}

func TestSchemaNodeUnmarshalYAMLDispatchesNumber(t *testing.T) {
	content := []byte(`
type: number
nullable: true
`)

	var schemaNode SchemaNode
	err := yaml.Unmarshal(content, &schemaNode)
	require.NoError(t, err)

	require.Equal(t, "number", schemaNode.Type)
	require.NotNil(t, schemaNode.Number)
	require.Equal(t, BaseNode{Nullable: true}, schemaNode.Number.BaseNode)
}

func TestSchemaNodeUnmarshalYAMLDispatchesBoolean(t *testing.T) {
	content := []byte(`
type: boolean
nullable: true
`)

	var schemaNode SchemaNode
	err := yaml.Unmarshal(content, &schemaNode)
	require.NoError(t, err)

	require.Equal(t, "boolean", schemaNode.Type)
	require.NotNil(t, schemaNode.Bool)
	require.Equal(t, BaseNode{Nullable: true}, schemaNode.Bool.BaseNode)
}

func TestSchemaNodeUnmarshalYAMLDispatchesObjectWithNestedProperties(t *testing.T) {
	content := []byte(`
type: object
nullable: false
required:
  - child
additionalProperties: false
properties:
  child:
    type: object
    nullable: true
    additionalProperties: true
    properties:
      name:
        type: string
        nullable: false
`)

	var schemaNode SchemaNode
	err := yaml.Unmarshal(content, &schemaNode)
	require.NoError(t, err)

	objectNode := schemaNode.Object
	require.Equal(t, "object", schemaNode.Type)
	require.NotNil(t, objectNode)
	require.Nil(t, schemaNode.String)
	require.Equal(t, BaseNode{Nullable: false}, objectNode.BaseNode)
	require.Equal(t, []string{"child"}, objectNode.Required)
	require.NotNil(t, objectNode.AdditionalProperties.Allowed)
	require.False(t, *objectNode.AdditionalProperties.Allowed)

	childSchema := objectNode.Properties["child"]
	require.Equal(t, "object", childSchema.Type)

	childNode := childSchema.Object
	require.NotNil(t, childNode)
	require.Equal(t, BaseNode{Nullable: true}, childNode.BaseNode)
	require.NotNil(t, childNode.AdditionalProperties.Allowed)
	require.True(t, *childNode.AdditionalProperties.Allowed)

	nameSchema := childNode.Properties["name"]
	require.Equal(t, "string", nameSchema.Type)

	nameNode := nameSchema.String
	require.NotNil(t, nameNode)
	require.Equal(t, BaseNode{Nullable: false}, nameNode.BaseNode)
}

func TestSchemaNodeUnmarshalYAMLDispatchesAllOf(t *testing.T) {
	content := []byte(`
allOf:
  - type: object
    required:
      - first
    properties:
      first:
        type: string
  - type: object
    required:
      - second
    properties:
      second:
        type: boolean
`)

	var schemaNode SchemaNode
	err := yaml.Unmarshal(content, &schemaNode)
	require.NoError(t, err)

	require.Equal(t, "object", schemaNode.Type)
	require.NotNil(t, schemaNode.Object)
	require.Equal(t, []string{"first", "second"}, schemaNode.Object.Required)
	require.Contains(t, schemaNode.Object.Properties, "first")
	require.Contains(t, schemaNode.Object.Properties, "second")
}

func TestObjectNodeUnmarshalYAMLAllowsMissingOptionalSections(t *testing.T) {
	content := []byte(`
type: object
nullable: true
`)

	var objectNode ObjectNode
	err := yaml.Unmarshal(content, &objectNode)
	require.NoError(t, err)

	require.Equal(t, BaseNode{Nullable: true}, objectNode.BaseNode)
	require.Nil(t, objectNode.Required)
	require.Nil(t, objectNode.AdditionalProperties.Allowed)
	require.Nil(t, objectNode.AdditionalProperties.Schema)
	require.Nil(t, objectNode.Properties)
}

func TestAdditionalPropertiesNodeUnmarshalYAMLCanBeBool(t *testing.T) {
	for name, tt := range map[string]struct {
		content string
		want    bool
	}{
		"true": {
			content: "true",
			want:    true,
		},
		"false": {
			content: "false",
			want:    false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var additionalPropertiesNode AdditionalPropertiesNode
			err := yaml.Unmarshal([]byte(tt.content), &additionalPropertiesNode)
			require.NoError(t, err)

			require.NotNil(t, additionalPropertiesNode.Allowed)
			require.Equal(t, tt.want, *additionalPropertiesNode.Allowed)
			require.Nil(t, additionalPropertiesNode.Schema)
		})
	}
}

func TestSchemaNodeUnmarshalYAMLRejectsUnsupportedSchemaType(t *testing.T) {
	content := []byte(`
type: integer
nullable: true
`)

	var schemaNode SchemaNode
	err := yaml.Unmarshal(content, &schemaNode)
	require.ErrorContains(t, err, `unsupported schema type "integer"`)
}

func TestSchemaNodeUnmarshalYAMLRejectsMissingSchemaType(t *testing.T) {
	content := []byte(`
nullable: true
`)

	var schemaNode SchemaNode
	err := yaml.Unmarshal(content, &schemaNode)
	require.ErrorContains(t, err, `unsupported schema type ""`)
}

func TestSchemaNodeUnmarshalYAMLRejectsAnyOfAndOneOf(t *testing.T) {
	for _, keyword := range []string{"anyOf", "oneOf"} {
		t.Run(keyword, func(t *testing.T) {
			content := []byte(fmt.Sprintf(`
%s:
  - type: string
`, keyword))

			var schemaNode SchemaNode
			err := yaml.Unmarshal(content, &schemaNode)
			require.ErrorContains(t, err, fmt.Sprintf(`unsupported schema composition %q`, keyword))
		})
	}
}

func TestSchemaNodeUnmarshalYAMLRejectsNestedAnyOfAndOneOf(t *testing.T) {
	for _, keyword := range []string{"anyOf", "oneOf"} {
		t.Run(keyword, func(t *testing.T) {
			content := []byte(fmt.Sprintf(`
type: object
properties:
  unsupported:
    %s:
      - type: string
`, keyword))

			var schemaNode SchemaNode
			err := yaml.Unmarshal(content, &schemaNode)
			require.ErrorContains(t, err, fmt.Sprintf(`unsupported schema composition %q`, keyword))
		})
	}
}

func TestOpenAPISchemaUnmarshalRejectsAnyOfAndOneOf(t *testing.T) {
	for _, keyword := range []string{"anyOf", "oneOf"} {
		t.Run(keyword, func(t *testing.T) {
			content := []byte(fmt.Sprintf(`
openapi: 3.0.3
info:
  title: Unsupported Composition
  version: 1.0.0
paths:
  /unsupported:
    post:
      operationId: unsupported
      requestBody:
        required: true
        content:
          application/json:
            schema:
              %s:
                - type: string
      responses:
        '204':
          description: No Content
`, keyword))

			var doc struct {
				Paths map[string]struct {
					Post *struct {
						RequestBody struct {
							Content map[string]struct {
								Schema SchemaNode `yaml:"schema"`
							} `yaml:"content"`
						} `yaml:"requestBody"`
					} `yaml:"post"`
				} `yaml:"paths"`
			}

			err := yaml.Unmarshal(content, &doc)
			require.ErrorContains(t, err, fmt.Sprintf(`unsupported schema composition %q`, keyword))
		})
	}
}

func TestSchemaNodeUnmarshalYAMLRejectsMissingSchemaNode(t *testing.T) {
	var schemaNode SchemaNode

	require.ErrorContains(t, schemaNode.UnmarshalYAML(nil), "missing schema")
	require.ErrorContains(t, schemaNode.UnmarshalYAML(new(yaml.Node)), "missing schema")
}

func TestSchemaNodeUnmarshalYAMLRejectsNonMappingSchema(t *testing.T) {
	var schemaNode SchemaNode
	err := yaml.Unmarshal([]byte(`"not-schema"`), &schemaNode)
	require.ErrorContains(t, err, "cannot unmarshal !!str")
}

func TestSchemaNodeUnmarshalYAMLRejectsInvalidObjectFields(t *testing.T) {
	content := []byte(`
type: object
required: not-a-list
`)

	var schemaNode SchemaNode
	err := yaml.Unmarshal(content, &schemaNode)
	require.ErrorContains(t, err, "cannot unmarshal !!str `not-a-list` into []string")
}

func TestSchemaNodeUnmarshalYAMLRejectsInvalidStringFields(t *testing.T) {
	content := []byte(`
type: string
nullable: not-bool
`)

	var schemaNode SchemaNode
	err := yaml.Unmarshal(content, &schemaNode)
	require.ErrorContains(t, err, "cannot unmarshal !!str `not-bool` into bool")
}

func TestAdditionalPropertiesNodeUnmarshalYAMLRejectsNonBoolScalar(t *testing.T) {
	var additionalPropertiesNode AdditionalPropertiesNode
	err := yaml.Unmarshal([]byte("123"), &additionalPropertiesNode)
	require.ErrorContains(t, err, "unsupported scalar !!int")
}

func TestAdditionalPropertiesNodeUnmarshalYAMLRejectsUnsupportedSchema(t *testing.T) {
	content := []byte(`
type: integer
nullable: true
`)

	var additionalPropertiesNode AdditionalPropertiesNode
	err := yaml.Unmarshal(content, &additionalPropertiesNode)
	require.ErrorContains(t, err, `unsupported schema type "integer"`)
}

func TestAdditionalPropertiesNodeUnmarshalYAMLAllowsMissingNode(t *testing.T) {
	var additionalPropertiesNode AdditionalPropertiesNode

	require.NoError(t, additionalPropertiesNode.UnmarshalYAML(nil))
	require.Nil(t, additionalPropertiesNode.Allowed)
	require.Nil(t, additionalPropertiesNode.Schema)

	require.NoError(t, additionalPropertiesNode.UnmarshalYAML(new(yaml.Node)))
	require.Nil(t, additionalPropertiesNode.Allowed)
	require.Nil(t, additionalPropertiesNode.Schema)
}

func TestAdditionalPropertiesNodeUnmarshalYAMLAllowsExplicitNull(t *testing.T) {
	var additionalPropertiesNode AdditionalPropertiesNode
	err := yaml.Unmarshal([]byte("null"), &additionalPropertiesNode)
	require.NoError(t, err)
	require.Nil(t, additionalPropertiesNode.Allowed)
	require.Nil(t, additionalPropertiesNode.Schema)
}

func TestAdditionalPropertiesNodeUnmarshalYAMLRejectsSequence(t *testing.T) {
	var additionalPropertiesNode AdditionalPropertiesNode
	err := yaml.Unmarshal([]byte("[true]"), &additionalPropertiesNode)
	require.ErrorContains(t, err, "unsupported yaml node kind")
}

func TestAdditionalPropertiesNodeUnmarshalYAMLRejectsInvalidBoolNode(t *testing.T) {
	var additionalPropertiesNode AdditionalPropertiesNode
	err := additionalPropertiesNode.UnmarshalYAML(&yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!bool",
		Value: "not-bool",
	})
	require.ErrorContains(t, err, "cannot decode !!str `not-bool` as a !!bool")
}
