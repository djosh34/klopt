package generate

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestJSONRequestBodySchemasKeepsOnlyOperationsWithJSONBodySchema(t *testing.T) {
	jsonSchema := openapi3.NewStringSchema()
	jsonOperation := operationWithContent("jsonBody", openapi3.NewContentWithJSONSchema(jsonSchema))
	noRequestBodyOperation := &openapi3.Operation{OperationID: "noRequestBody"}
	noJSONOperation := operationWithContent("noJSON", openapi3.NewContentWithSchema(openapi3.NewStringSchema(), []string{"text/plain"}))
	noSchemaOperation := operationWithContent("noSchema", openapi3.Content{
		"application/json": openapi3.NewMediaType(),
	})

	generateContext := &GenerateContext{
		Document: &openapi3.T{
			Paths: openapi3.NewPaths(
				openapi3.WithPath("/json", &openapi3.PathItem{Post: jsonOperation}),
				openapi3.WithPath("/no-request-body", &openapi3.PathItem{Post: noRequestBodyOperation}),
				openapi3.WithPath("/no-json", &openapi3.PathItem{Post: noJSONOperation}),
				openapi3.WithPath("/no-schema", &openapi3.PathItem{Post: noSchemaOperation}),
			),
		},
	}

	schemas, err := generateContext.JSONRequestBodySchemas()
	require.NoError(t, err)

	require.Equal(t, map[*openapi3.Operation]*openapi3.Schema{
		jsonOperation: jsonSchema,
	}, schemas)
}

func TestJSONRequestBodySchemaObjectsConvertsRequestBodySchemas(t *testing.T) {
	openapiExamplePath := filepath.Join(GetRepoRoot(t), "pkg", "decode", "example", "openapi.yaml")
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	err = generateContext.FilterOperations("objectKeysAdditionalPropertiesFalse")
	require.NoError(t, err)

	schemaObjects, err := generateContext.JSONRequestBodySchemaObjects()
	require.NoError(t, err)

	require.Equal(t, []SchemaObject{
		{
			Generatable: &ObjectContext{
				AdditionalProperties: false,
				Properties: []ObjectFieldContext{
					{
						PropertyName: "optionalNotNullableString",
						Schema: SchemaObject{
							Generatable: &StringContext{},
						},
					},
					{
						PropertyName: "optionalNullableString",
						Schema: SchemaObject{
							Generatable: &StringContext{},
							Nullable:    true,
						},
					},
					{
						PropertyName: "requiredNotNullableString",
						Schema: SchemaObject{
							Generatable: &StringContext{},
						},
						Required: true,
					},
					{
						PropertyName: "requiredNullableString",
						Schema: SchemaObject{
							Generatable: &StringContext{},
							Nullable:    true,
						},
						Required: true,
					},
				},
			},
		},
	}, schemaObjects)
}

func TestSchemaObjectFromOpenAPISchemaRecursesObjectProperties(t *testing.T) {
	nestedSchema := openapi3.NewObjectSchema()
	nestedSchema.WithProperty("child", openapi3.NewStringSchema().WithNullable())

	schema := openapi3.NewObjectSchema()
	schema.Required = []string{"name", "nested"}
	schema.WithoutAdditionalProperties()
	schema.WithProperty("name", openapi3.NewStringSchema())
	schema.WithProperty("nested", nestedSchema)

	schemaObject, err := SchemaObjectFromOpenAPISchema(schema)
	require.NoError(t, err)

	require.Equal(t, SchemaObject{
		Generatable: &ObjectContext{
			AdditionalProperties: false,
			Properties: []ObjectFieldContext{
				{
					PropertyName: "name",
					Schema: SchemaObject{
						Generatable: &StringContext{},
					},
					Required: true,
				},
				{
					PropertyName: "nested",
					Required:     true,
					Schema: SchemaObject{
						Generatable: &ObjectContext{
							AdditionalProperties: true,
							Properties: []ObjectFieldContext{
								{
									PropertyName: "child",
									Schema: SchemaObject{
										Generatable: &StringContext{},
										Nullable:    true,
									},
								},
							},
						},
					},
				},
			},
		},
	}, schemaObject)
}

func TestSchemaObjectFromOpenAPISchemaConvertsArrayItems(t *testing.T) {
	schema := openapi3.NewArraySchema().WithNullable()
	schema.WithItems(openapi3.NewStringSchema())

	schemaObject, err := SchemaObjectFromOpenAPISchema(schema)
	require.NoError(t, err)

	require.Equal(t, SchemaObject{
		Generatable: &ArrayContext{
			Items: SchemaObject{
				Generatable: &StringContext{},
			},
		},
		Nullable: true,
	}, schemaObject)
}

func TestSchemaObjectFromOpenAPISchemaConvertsAdditionalPropertiesSchema(t *testing.T) {
	schema := openapi3.NewObjectSchema()
	schema.WithAdditionalProperties(openapi3.NewStringSchema().WithNullable())

	schemaObject, err := SchemaObjectFromOpenAPISchema(schema)
	require.NoError(t, err)

	require.Equal(t, SchemaObject{
		Generatable: &ObjectContext{
			AdditionalProperties: true,
			AdditionalPropertiesSchema: SchemaObject{
				Generatable: &StringContext{},
				Nullable:    true,
			},
			Properties: []ObjectFieldContext{},
		},
	}, schemaObject)
}

func operationWithContent(operationID string, content openapi3.Content) *openapi3.Operation {
	return &openapi3.Operation{
		OperationID: operationID,
		RequestBody: &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Content: content,
			},
		},
	}
}

func TestJSONRequestBodySchemasSupportsHTTPMethods(t *testing.T) {
	schema := openapi3.NewStringSchema()
	operation := operationWithContent("putBody", openapi3.NewContentWithJSONSchema(schema))
	pathItem := new(openapi3.PathItem)
	pathItem.SetOperation(http.MethodPut, operation)

	generateContext := &GenerateContext{
		Document: &openapi3.T{
			Paths: openapi3.NewPaths(openapi3.WithPath("/put", pathItem)),
		},
	}

	schemas, err := generateContext.JSONRequestBodySchemas()
	require.NoError(t, err)

	require.Equal(t, map[*openapi3.Operation]*openapi3.Schema{
		operation: schema,
	}, schemas)
}
