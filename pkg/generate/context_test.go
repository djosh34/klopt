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

func TestJSONRequestBodyModelSchemasConvertsRequestBodySchemas(t *testing.T) {
	openapiExamplePath := filepath.Join(GetRepoRoot(t), "pkg", "decode", "example", "openapi.yaml")
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	err = generateContext.FilterOperations("objectKeysAdditionalPropertiesFalse")
	require.NoError(t, err)

	schemas, err := generateContext.JSONRequestBodyModelSchemas()
	require.NoError(t, err)

	require.Equal(t, []Schema{
		&ObjectSchema{
			BaseSchema:           BaseSchema{TypeName: "ObjectKeysAdditionalPropertiesFalse"},
			AdditionalProperties: false,
			Properties: []ObjectFieldContext{
				{
					PropertyName: "optionalNotNullableString",
					Schema: &StringSchema{
						BaseSchema: BaseSchema{TypeName: "OptionalNotNullableString"},
					},
				},
				{
					PropertyName: "optionalNullableString",
					Schema: &StringSchema{
						BaseSchema: BaseSchema{TypeName: "OptionalNullableString", Nullable: true},
					},
				},
				{
					PropertyName: "requiredNotNullableString",
					Schema: &StringSchema{
						BaseSchema: BaseSchema{TypeName: "RequiredNotNullableString"},
					},
					Required: true,
				},
				{
					PropertyName: "requiredNullableString",
					Schema: &StringSchema{
						BaseSchema: BaseSchema{TypeName: "RequiredNullableString", Nullable: true},
					},
					Required: true,
				},
			},
		},
	}, schemas)
}

func TestSchemaFromOpenAPISchemaRecursesObjectProperties(t *testing.T) {
	nestedSchema := openapi3.NewObjectSchema()
	nestedSchema.WithProperty("child", openapi3.NewStringSchema().WithNullable())

	schema := openapi3.NewObjectSchema()
	schema.Required = []string{"name", "nested"}
	schema.WithoutAdditionalProperties()
	schema.WithProperty("name", openapi3.NewStringSchema())
	schema.WithProperty("nested", nestedSchema)

	generatedSchema, err := SchemaFromOpenAPISchema(schema)
	require.NoError(t, err)

	require.Equal(t, &ObjectSchema{
		AdditionalProperties: false,
		Properties: []ObjectFieldContext{
			{
				PropertyName: "name",
				Schema:       &StringSchema{},
				Required:     true,
			},
			{
				PropertyName: "nested",
				Required:     true,
				Schema: &ObjectSchema{
					AdditionalProperties: true,
					Properties: []ObjectFieldContext{
						{
							PropertyName: "child",
							Schema: &StringSchema{
								BaseSchema: BaseSchema{Nullable: true},
							},
						},
					},
				},
			},
		},
	}, generatedSchema)
}

func TestSchemaFromOpenAPISchemaConvertsArrayItems(t *testing.T) {
	schema := openapi3.NewArraySchema().WithNullable()
	schema.WithItems(openapi3.NewStringSchema())

	generatedSchema, err := SchemaFromOpenAPISchema(schema)
	require.NoError(t, err)

	require.Equal(t, &ArraySchema{
		BaseSchema: BaseSchema{Nullable: true},
		Items:      &StringSchema{},
	}, generatedSchema)
}

func TestSchemaFromOpenAPISchemaConvertsAdditionalPropertiesSchema(t *testing.T) {
	schema := openapi3.NewObjectSchema()
	schema.WithAdditionalProperties(openapi3.NewStringSchema().WithNullable())

	generatedSchema, err := SchemaFromOpenAPISchema(schema)
	require.NoError(t, err)

	require.Equal(t, &ObjectSchema{
		AdditionalProperties: true,
		AdditionalPropertiesSchema: &StringSchema{
			BaseSchema: BaseSchema{Nullable: true},
		},
		Properties: []ObjectFieldContext{},
	}, generatedSchema)
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
