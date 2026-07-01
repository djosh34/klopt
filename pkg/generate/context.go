package generate

import (
	"fmt"
	"maps"
	"slices"

	"github.com/getkin/kin-openapi/openapi3"
)

type GenerateContext struct {
	Document *openapi3.T
}

type SchemaObject struct {
	Generatable
	TypeName string
	Nullable bool
}

type Generatable interface {
	Generate() (string, error)
}

var _ Generatable = new(ObjectContext)
var _ Generatable = new(StringContext)
var _ Generatable = new(ArrayContext)

type ObjectContext struct {
	AdditionalProperties       bool
	AdditionalPropertiesSchema SchemaObject
	Properties                 []ObjectFieldContext
}

type ObjectFieldContext struct {
	PropertyName string
	Schema       SchemaObject
	Required     bool
}

type StringContext struct {
}
type ArrayContext struct {
	Items SchemaObject
}

// TODO, put in tmpl
func (p ObjectFieldContext) FieldType() string {
	if p.Required {
		return p.Schema.TypeName
	}

	return "*" + p.Schema.TypeName
}

// TODO, put in tmpl
func (p ObjectFieldContext) JSONTag() string {
	if p.Required {
		return fmt.Sprintf("`json:%q`", p.PropertyName)
	}

	return fmt.Sprintf("`json:%q`", p.PropertyName+",omitzero")
}

// TODO, we could decide to not care, and auto gen some valid var name
func (p ObjectFieldContext) LocalName() string {
	return unexportedName(p.Schema.TypeName)
}

func (o ObjectContext) Generate() (string, error) {
	return executeGoTemplate("object.go.tmpl", o)
}

func (o StringContext) Generate() (string, error) {
	return executeGoTemplate("string.go.tmpl", o)
}

func (o ArrayContext) Generate() (string, error) {
	return executeGoTemplate("array.go.tmpl", o)
}

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

// TODO, this seems very duplicative. Why can't this be integrated in the above function?? For each operation, do convert to SchemaObject
func (c *GenerateContext) JSONRequestBodySchemaObjects() ([]SchemaObject, error) {
	var schemaObjects []SchemaObject

	if c.Document == nil || c.Document.Paths == nil {
		return nil, fmt.Errorf("openapi document has no paths")
	}

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

			schema := jsonBody.Schema.Value
			if schema.Title == "" && operation.OperationID != "" {
				schema.Title = operation.OperationID
			}

			schemaObject, err := SchemaObjectFromOpenAPISchema(schema)
			if err != nil {
				return nil, fmt.Errorf("operation %q request body schema: %w", operation.OperationID, err)
			}

			schemaObjects = append(schemaObjects, schemaObject)
		}
	}

	return schemaObjects, nil
}

func SchemaObjectFromOpenAPISchema(schema *openapi3.Schema) (SchemaObject, error) {
	var schemaObject SchemaObject

	if schema == nil {
		return schemaObject, fmt.Errorf("openapi schema is nil")
	}

	schemaType, err := schemaObjectType(schema)
	if err != nil {
		return schemaObject, err
	}

	schemaObject.Nullable = schema.PermitsNull()

	switch schemaType {
	case openapi3.TypeObject:
		schemaObject.Generatable, err = objectContextFromOpenAPISchema(schema)
		if err != nil {
			return schemaObject, err
		}
	case openapi3.TypeArray:
		schemaObject.Generatable, err = arrayContextFromOpenAPISchema(schema)
		if err != nil {
			return schemaObject, err
		}
	case openapi3.TypeString:
		schemaObject.Generatable = &StringContext{}
	default:
		return schemaObject, fmt.Errorf("unsupported schema type %q", schemaType)
	}

	if schema.Title != "" {
		schemaObject.TypeName = exportedName(schema.Title)
	}

	return schemaObject, nil
}

func objectContextFromOpenAPISchema(schema *openapi3.Schema) (*ObjectContext, error) {
	objectContext := &ObjectContext{
		AdditionalProperties: true,
		Properties:           make([]ObjectFieldContext, 0, len(schema.Properties)),
	}

	// TODO, I keep seeing this overly double and bad verbose ways. Why not just one if check, like i can't phathom why double if check is needed??
	if hasAdditionalProperties := schema.AdditionalProperties.Has; hasAdditionalProperties != nil {
		objectContext.AdditionalProperties = *hasAdditionalProperties
	}

	if additionalPropertiesSchema := schema.AdditionalProperties.Schema; additionalPropertiesSchema != nil {
		additionalPropertiesObject, err := schemaObjectFromOpenAPISchemaRef(additionalPropertiesSchema)
		if err != nil {
			return nil, fmt.Errorf("additionalProperties schema: %w", err)
		}

		objectContext.AdditionalProperties = true
		objectContext.AdditionalPropertiesSchema = additionalPropertiesObject
	}

	required := make(map[string]struct{}, len(schema.Required))
	for _, propertyName := range schema.Required {
		required[propertyName] = struct{}{}
	}

	for _, propertyName := range slices.Sorted(maps.Keys(schema.Properties)) {
		propertySchema := schema.Properties[propertyName]
		propertyObject, err := schemaObjectFromOpenAPISchemaRef(propertySchema)
		if err != nil {
			return nil, fmt.Errorf("property %q schema: %w", propertyName, err)
		}

		_, isRequired := required[propertyName]
		objectContext.Properties = append(objectContext.Properties, ObjectFieldContext{
			PropertyName: propertyName,
			Schema:       propertyObject,
			Required:     isRequired,
		})
	}

	return objectContext, nil
}

func arrayContextFromOpenAPISchema(schema *openapi3.Schema) (*ArrayContext, error) {
	if schema.Items == nil {
		return nil, fmt.Errorf("array schema has no items")
	}

	items, err := schemaObjectFromOpenAPISchemaRef(schema.Items)
	if err != nil {
		return nil, fmt.Errorf("array items schema: %w", err)
	}

	return &ArrayContext{
		Items: items,
	}, nil
}

// TODO, I have high concern for this function. But we would need first to get better testing than this. It looks to me that it doesn't try to find the reffed value at all
func schemaObjectFromOpenAPISchemaRef(schemaRef *openapi3.SchemaRef) (SchemaObject, error) {
	if schemaRef == nil {
		return SchemaObject{}, fmt.Errorf("openapi schema ref is nil")
	}

	if schemaRef.Value == nil {
		if schemaRef.Ref != "" {
			return SchemaObject{}, fmt.Errorf("openapi schema ref %q has no value", schemaRef.Ref)
		}

		return SchemaObject{}, fmt.Errorf("openapi schema ref has no value")
	}

	return SchemaObjectFromOpenAPISchema(schemaRef.Value)
}

// TODO, Concerned about this as well. Wouldn't we want a better inferring of type method
// Perhaps just one traversal over the whole schema, and setting Type once. From then on you just read out the 'Type'
// I thought that openapi3.Schema would already do that for us, but perhaps not
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

// TODO, validate if this is true. I thought that strings for instance could also be inferred
func inferredSchemaObjectType(schema *openapi3.Schema) (string, error) {
	if len(schema.Properties) != 0 || len(schema.Required) != 0 || schema.AdditionalProperties.Has != nil || schema.AdditionalProperties.Schema != nil {
		return openapi3.TypeObject, nil
	}

	if schema.Items != nil {
		return openapi3.TypeArray, nil
	}

	return "", fmt.Errorf("openapi schema has no type")
}
