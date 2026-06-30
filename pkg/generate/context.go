package generate

import (
	"fmt"
	"slices"

	"github.com/getkin/kin-openapi/openapi3"
)

type GenerateContext struct {
	Document   *openapi3.T
	Operations map[string]SchemaObject
}

type SchemaObject interface {
	// Only reason this exists, is to differentiate for SchemaObject and non-SchemaObject
	DummyFunc()
}

var _ SchemaObject = new(ObjectContext)
var _ SchemaObject = new(StringContext)
var _ SchemaObject = new(ArrayContext)

type ObjectContext struct {
	Nullable                   bool
	AdditionalProperties       bool
	AdditionalPropertiesSchema SchemaObject
	Required                   []string
	Properties                 map[string]SchemaObject
}

type StringContext struct {
	Nullable bool
}
type ArrayContext struct {
	Nullable bool
	Items    SchemaObject
}

func (o ObjectContext) DummyFunc() {}
func (o StringContext) DummyFunc() {}
func (o ArrayContext) DummyFunc()  {}

func (c *GenerateContext) JSONRequestBodySchemas() (map[*openapi3.Operation]*openapi3.Schema, error) {
	if c.Document == nil || c.Document.Paths == nil {
		return nil, fmt.Errorf("openapi document has no paths")
	}

	schemas := make(map[*openapi3.Operation]*openapi3.Schema)
	for _, path := range c.Document.Paths.InMatchingOrder() {
		pathItem := c.Document.Paths.Value(path)
		if pathItem == nil {
			continue
		}

		for _, operation := range pathItem.Operations() {
			if operation == nil || operation.RequestBody == nil || operation.RequestBody.Value == nil {
				continue
			}

			jsonBody := operation.RequestBody.Value.Content.Get("application/json")
			if jsonBody == nil || jsonBody.Schema == nil || jsonBody.Schema.Value == nil {
				continue
			}

			schemas[operation] = jsonBody.Schema.Value
		}
	}

	return schemas, nil
}

func (c *GenerateContext) JSONRequestBodySchemaObjects() (map[string]SchemaObject, error) {
	openapiSchemas, err := c.JSONRequestBodySchemas()
	if err != nil {
		return nil, err
	}

	schemaObjects := make(map[string]SchemaObject, len(openapiSchemas))
	for operation, openapiSchema := range openapiSchemas {
		if operation.OperationID == "" {
			return nil, fmt.Errorf("operation with JSON request body schema has no operationId")
		}

		if _, ok := schemaObjects[operation.OperationID]; ok {
			return nil, fmt.Errorf("duplicate operationId %q", operation.OperationID)
		}

		schemaObject, err := SchemaObjectFromOpenAPISchema(openapiSchema)
		if err != nil {
			return nil, fmt.Errorf("operation %q request body schema: %w", operation.OperationID, err)
		}

		schemaObjects[operation.OperationID] = schemaObject
	}

	return schemaObjects, nil
}

func SchemaObjectFromOpenAPISchema(schema *openapi3.Schema) (SchemaObject, error) {
	if schema == nil {
		return nil, fmt.Errorf("openapi schema is nil")
	}

	schemaType, err := schemaObjectType(schema)
	if err != nil {
		return nil, err
	}

	switch schemaType {
	case openapi3.TypeObject:
		return objectContextFromOpenAPISchema(schema)
	case openapi3.TypeArray:
		return arrayContextFromOpenAPISchema(schema)
	case openapi3.TypeString:
		return StringContext{
			Nullable: schema.PermitsNull(),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported schema type %q", schemaType)
	}
}

func objectContextFromOpenAPISchema(schema *openapi3.Schema) (ObjectContext, error) {
	objectContext := ObjectContext{
		Nullable:             schema.PermitsNull(),
		AdditionalProperties: true,
		Required:             slices.Clone(schema.Required),
		Properties:           make(map[string]SchemaObject, len(schema.Properties)),
	}

	if hasAdditionalProperties := schema.AdditionalProperties.Has; hasAdditionalProperties != nil {
		objectContext.AdditionalProperties = *hasAdditionalProperties
	}

	if additionalPropertiesSchema := schema.AdditionalProperties.Schema; additionalPropertiesSchema != nil {
		additionalPropertiesObject, err := schemaObjectFromOpenAPISchemaRef(additionalPropertiesSchema)
		if err != nil {
			return ObjectContext{}, fmt.Errorf("additionalProperties schema: %w", err)
		}

		objectContext.AdditionalProperties = true
		objectContext.AdditionalPropertiesSchema = additionalPropertiesObject
	}

	for propertyName, propertySchema := range schema.Properties {
		propertyObject, err := schemaObjectFromOpenAPISchemaRef(propertySchema)
		if err != nil {
			return ObjectContext{}, fmt.Errorf("property %q schema: %w", propertyName, err)
		}

		objectContext.Properties[propertyName] = propertyObject
	}

	return objectContext, nil
}

func arrayContextFromOpenAPISchema(schema *openapi3.Schema) (ArrayContext, error) {
	if schema.Items == nil {
		return ArrayContext{}, fmt.Errorf("array schema has no items")
	}

	items, err := schemaObjectFromOpenAPISchemaRef(schema.Items)
	if err != nil {
		return ArrayContext{}, fmt.Errorf("array items schema: %w", err)
	}

	return ArrayContext{
		Nullable: schema.PermitsNull(),
		Items:    items,
	}, nil
}

func schemaObjectFromOpenAPISchemaRef(schemaRef *openapi3.SchemaRef) (SchemaObject, error) {
	if schemaRef == nil {
		return nil, fmt.Errorf("openapi schema ref is nil")
	}

	if schemaRef.Value == nil {
		if schemaRef.Ref != "" {
			return nil, fmt.Errorf("openapi schema ref %q has no value", schemaRef.Ref)
		}

		return nil, fmt.Errorf("openapi schema ref has no value")
	}

	return SchemaObjectFromOpenAPISchema(schemaRef.Value)
}

func schemaObjectType(schema *openapi3.Schema) (string, error) {
	if schema.Type == nil || schema.Type.IsEmpty() {
		return inferredSchemaObjectType(schema)
	}

	schemaTypes := schema.Type.Slice()
	nonNullTypes := make([]string, 0, len(schemaTypes))
	for _, schemaType := range schemaTypes {
		if schemaType != openapi3.TypeNull {
			nonNullTypes = append(nonNullTypes, schemaType)
		}
	}

	if len(nonNullTypes) != 1 {
		return "", fmt.Errorf("unsupported schema types %v", schemaTypes)
	}

	return nonNullTypes[0], nil
}

func inferredSchemaObjectType(schema *openapi3.Schema) (string, error) {
	if len(schema.Properties) != 0 || len(schema.Required) != 0 || schema.AdditionalProperties.Has != nil || schema.AdditionalProperties.Schema != nil {
		return openapi3.TypeObject, nil
	}

	if schema.Items != nil {
		return openapi3.TypeArray, nil
	}

	return "", fmt.Errorf("openapi schema has no type")
}
