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

type Schema interface {
	Base() *BaseSchema
	Generate() (string, error)
}

type BaseSchema struct {
	TypeName string
	Nullable bool
}

type ObjectSchema struct {
	BaseSchema
	AdditionalProperties       bool
	AdditionalPropertiesSchema Schema
	Properties                 []ObjectFieldContext
}

var _ Schema = new(ObjectSchema)
var _ Schema = new(StringSchema)
var _ Schema = new(ArraySchema)

type ObjectFieldContext struct {
	PropertyName string
	Schema       Schema
	Required     bool
}

type StringSchema struct {
	BaseSchema
}

type ArraySchema struct {
	BaseSchema
	Items Schema
}

// TODO, put in tmpl
func (p ObjectFieldContext) FieldType() string {
	if p.Required {
		return p.SchemaTypeName()
	}

	return "*" + p.SchemaTypeName()
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
	return unexportedName(p.SchemaTypeName())
}

func (p ObjectFieldContext) SchemaTypeName() string {
	return schemaTypeName(p.Schema)
}

func (o *ObjectSchema) Base() *BaseSchema {
	if o == nil {
		return nil
	}

	return &o.BaseSchema
}

func (o *ObjectSchema) Generate() (string, error) {
	if o == nil {
		return "", fmt.Errorf("nil object schema")
	}

	return executeGoTemplate("object.go.tmpl", o)
}

func (o *ObjectSchema) AdditionalPropertiesTypeName() string {
	return schemaTypeName(o.AdditionalPropertiesSchema)
}

func (s *StringSchema) Base() *BaseSchema {
	if s == nil {
		return nil
	}

	return &s.BaseSchema
}

func (s *StringSchema) Generate() (string, error) {
	if s == nil {
		return "", fmt.Errorf("nil string schema")
	}

	return executeGoTemplate("string.go.tmpl", s)
}

func (a *ArraySchema) Base() *BaseSchema {
	if a == nil {
		return nil
	}

	return &a.BaseSchema
}

func (a *ArraySchema) Generate() (string, error) {
	if a == nil {
		return "", fmt.Errorf("nil array schema")
	}

	return executeGoTemplate("array.go.tmpl", a)
}

func (a *ArraySchema) ItemsTypeName() string {
	return schemaTypeName(a.Items)
}

func schemaTypeName(schema Schema) string {
	if schema == nil {
		return ""
	}

	base := schema.Base()
	if base == nil {
		return ""
	}

	return base.TypeName
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

func (c *GenerateContext) JSONRequestBodyModelSchemas() ([]Schema, error) {
	var schemas []Schema

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

			schema, err := SchemaFromOpenAPISchema(jsonBody.Schema.Value)
			if err != nil {
				return nil, fmt.Errorf("operation %q request body schema: %w", operation.OperationID, err)
			}

			name := jsonBody.Schema.Value.Title
			if name == "" {
				name = operation.OperationID
			}

			err = nameSchema(schema, name)
			if err != nil {
				return nil, fmt.Errorf("operation %q request body schema names: %w", operation.OperationID, err)
			}

			schemas = append(schemas, schema)
		}
	}

	return schemas, nil
}

func SchemaFromOpenAPISchema(schema *openapi3.Schema) (Schema, error) {
	if schema == nil {
		return nil, fmt.Errorf("openapi schema is nil")
	}

	schemaType, err := schemaType(schema)
	if err != nil {
		return nil, err
	}

	base := BaseSchema{
		Nullable: schema.PermitsNull(),
	}
	if schema.Title != "" {
		base.TypeName = exportedName(schema.Title)
	}

	switch schemaType {
	case openapi3.TypeObject:
		objectSchema, err := objectSchemaFromOpenAPISchema(schema)
		if err != nil {
			return nil, err
		}

		objectSchema.BaseSchema = base
		return objectSchema, nil
	case openapi3.TypeArray:
		arraySchema, err := arraySchemaFromOpenAPISchema(schema)
		if err != nil {
			return nil, err
		}

		arraySchema.BaseSchema = base
		return arraySchema, nil
	case openapi3.TypeString:
		return &StringSchema{
			BaseSchema: base,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported schema type %q", schemaType)
	}
}

func objectSchemaFromOpenAPISchema(schema *openapi3.Schema) (*ObjectSchema, error) {
	objectSchema := &ObjectSchema{
		AdditionalProperties: true,
		Properties:           make([]ObjectFieldContext, 0, len(schema.Properties)),
	}

	// TODO, I keep seeing this overly double and bad verbose ways. Why not just one if check, like i can't phathom why double if check is needed??
	if hasAdditionalProperties := schema.AdditionalProperties.Has; hasAdditionalProperties != nil {
		objectSchema.AdditionalProperties = *hasAdditionalProperties
	}

	if additionalPropertiesSchema := schema.AdditionalProperties.Schema; additionalPropertiesSchema != nil {
		additionalPropertiesObject, err := schemaFromOpenAPISchemaRef(additionalPropertiesSchema)
		if err != nil {
			return nil, fmt.Errorf("additionalProperties schema: %w", err)
		}

		objectSchema.AdditionalProperties = true
		objectSchema.AdditionalPropertiesSchema = additionalPropertiesObject
	}

	required := make(map[string]struct{}, len(schema.Required))
	for _, propertyName := range schema.Required {
		required[propertyName] = struct{}{}
	}

	for _, propertyName := range slices.Sorted(maps.Keys(schema.Properties)) {
		propertySchema := schema.Properties[propertyName]
		propertyObject, err := schemaFromOpenAPISchemaRef(propertySchema)
		if err != nil {
			return nil, fmt.Errorf("property %q schema: %w", propertyName, err)
		}

		_, isRequired := required[propertyName]
		objectSchema.Properties = append(objectSchema.Properties, ObjectFieldContext{
			PropertyName: propertyName,
			Schema:       propertyObject,
			Required:     isRequired,
		})
	}

	return objectSchema, nil
}

func arraySchemaFromOpenAPISchema(schema *openapi3.Schema) (*ArraySchema, error) {
	if schema.Items == nil {
		return nil, fmt.Errorf("array schema has no items")
	}

	items, err := schemaFromOpenAPISchemaRef(schema.Items)
	if err != nil {
		return nil, fmt.Errorf("array items schema: %w", err)
	}

	return &ArraySchema{
		Items: items,
	}, nil
}

// TODO, I have high concern for this function. But we would need first to get better testing than this. It looks to me that it doesn't try to find the reffed value at all
func schemaFromOpenAPISchemaRef(schemaRef *openapi3.SchemaRef) (Schema, error) {
	if schemaRef == nil {
		return nil, fmt.Errorf("openapi schema ref is nil")
	}

	if schemaRef.Value == nil {
		if schemaRef.Ref != "" {
			return nil, fmt.Errorf("openapi schema ref %q has no value", schemaRef.Ref)
		}

		return nil, fmt.Errorf("openapi schema ref has no value")
	}

	return SchemaFromOpenAPISchema(schemaRef.Value)
}

// TODO, Concerned about this as well. Wouldn't we want a better inferring of type method
// Perhaps just one traversal over the whole schema, and setting Type once. From then on you just read out the 'Type'
// I thought that openapi3.Schema would already do that for us, but perhaps not
func schemaType(schema *openapi3.Schema) (string, error) {
	if schema.Type == nil || schema.Type.IsEmpty() {
		return inferredSchemaType(schema)
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
func inferredSchemaType(schema *openapi3.Schema) (string, error) {
	if len(schema.Properties) != 0 || len(schema.Required) != 0 || schema.AdditionalProperties.Has != nil || schema.AdditionalProperties.Schema != nil {
		return openapi3.TypeObject, nil
	}

	if schema.Items != nil {
		return openapi3.TypeArray, nil
	}

	return "", fmt.Errorf("openapi schema has no type")
}
