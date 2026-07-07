package generate

import (
	"fmt"
	"maps"
	"slices"

	"github.com/getkin/kin-openapi/openapi3"
)

type GenerateContext struct {
	Document                  *openapi3.T
	OpenAPISource             []byte
	JSONRequestBodyOperations []JSONRequestBodyOperation
}

type Schema interface {
	Base() *BaseSchema
	ChildSchemas() []Schema
	Generate() (string, error)
	SchemaTypeName() string
	SetTypeName(string)
}

type BaseSchema struct {
	Name             string
	Nullable         bool
	EmptyBodyAllowed bool
}

type JSONRequestBodyOperation struct {
	OperationID string
	TypeName    string
}

func (o JSONRequestBodyOperation) TestName() string {
	return exportedName(o.OperationID)
}

func (b *BaseSchema) Base() *BaseSchema {
	return b
}

func (b *BaseSchema) SchemaTypeName() string {
	if b == nil {
		return ""
	}

	return b.Name
}

func (b *BaseSchema) SetTypeName(name string) {
	b.Name = name
}

func (b *BaseSchema) LocalName() string {
	return unexportedName(b.SchemaTypeName())
}

type ObjectSchema struct {
	BaseSchema

	AdditionalProperties       bool
	AdditionalPropertiesSchema Schema
	Properties                 []ObjectFieldContext
}

var (
	_ Schema = new(ObjectSchema)
	_ Schema = new(StringSchema)
	_ Schema = new(ArraySchema)
	_ Schema = new(AllOfSchema)
	_ Schema = new(BoolSchema)
	_ Schema = new(NumberSchema)
)

type ObjectFieldContext struct {
	Schema

	PropertyName string
	Required     bool
}

type StringSchema struct {
	BaseSchema

	Format string
}

type BoolSchema struct {
	BaseSchema
}

type NumberSchema struct {
	BaseSchema
}

type ArraySchema struct {
	BaseSchema

	Items Schema
}

type AllOfSchema struct {
	BaseSchema

	Schemas []Schema
}

func (o *ObjectSchema) ChildSchemas() []Schema {
	children := make([]Schema, 0, len(o.Properties)+1)
	for _, property := range o.Properties {
		children = append(children, property.Schema)
	}

	if o.AdditionalPropertiesSchema != nil {
		children = append(children, o.AdditionalPropertiesSchema)
	}

	return children
}

func (s *StringSchema) ChildSchemas() []Schema {
	return nil
}

func (b *BoolSchema) ChildSchemas() []Schema {
	return nil
}

func (n *NumberSchema) ChildSchemas() []Schema {
	return nil
}

func (a *ArraySchema) ChildSchemas() []Schema {
	if a.Items == nil {
		return nil
	}

	return []Schema{a.Items}
}

func (a *AllOfSchema) ChildSchemas() []Schema {
	return a.Schemas
}

// TODO, put in tmpl.
func (p *ObjectFieldContext) FieldType() string {
	if p.Required {
		return p.SchemaTypeName()
	}

	return "*" + p.SchemaTypeName()
}

// TODO, put in tmpl.
func (p *ObjectFieldContext) JSONTag() string {
	if p.Required {
		return fmt.Sprintf("`json:%q`", p.PropertyName)
	}

	return fmt.Sprintf("`json:%q`", p.PropertyName+",omitzero")
}

// TODO, we could decide to not care, and auto gen some valid var name.
func (p *ObjectFieldContext) LocalName() string {
	return unexportedName(p.PropertyName)
}

func (p *ObjectFieldContext) FieldName() string {
	return exportedName(p.PropertyName)
}

func (o *ObjectFieldContext) Generate() (string, error) {
	if o == nil {
		return "", fmt.Errorf("nil object schema")
	}

	return executeGoTemplate("object_property.go.tmpl", o)
}

//func (p ObjectFieldContext) SchemaTypeName() string {
//	return schemaTypeName(p.Schema)
//}

func (o *ObjectSchema) Generate() (string, error) {
	if o == nil {
		return "", fmt.Errorf("nil object schema")
	}

	return executeGoTemplate("object.go.tmpl", o)
}

func (o *ObjectSchema) AdditionalPropertiesTypeName() string {
	return schemaTypeName(o.AdditionalPropertiesSchema)
}

func (s *StringSchema) Generate() (string, error) {
	if s == nil {
		return "", fmt.Errorf("nil string schema")
	}

	return executeGoTemplate("string.go.tmpl", s)
}

func (b *BoolSchema) Generate() (string, error) {
	if b == nil {
		return "", fmt.Errorf("nil bool schema")
	}

	return executeGoTemplate("bool.go.tmpl", b)
}

func (n *NumberSchema) Generate() (string, error) {
	if n == nil {
		return "", fmt.Errorf("nil number schema")
	}

	return executeGoTemplate("number.go.tmpl", n)
}

func (a *ArraySchema) Generate() (string, error) {
	if a == nil {
		return "", fmt.Errorf("nil array schema")
	}

	return executeGoTemplate("array.go.tmpl", a)
}

func (a *AllOfSchema) Generate() (string, error) {
	if a == nil {
		return "", fmt.Errorf("nil allOf schema")
	}

	return executeGoTemplate("all_of.go.tmpl", a)
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

	return base.SchemaTypeName()
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

	c.JSONRequestBodyOperations = nil

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

			schema.Base().EmptyBodyAllowed = !operation.RequestBody.Value.Required

			name := jsonBody.Schema.Value.Title
			if name == "" {
				name = operation.OperationID
			}

			if operation.OperationID == "" {
				return nil, fmt.Errorf("json request body operation has no operationId")
			}

			if schema.SchemaTypeName() == "" {
				schema.SetTypeName(exportedName(name))
			}

			definitions, err := namedSchemaDefinitions(schema)
			if err != nil {
				return nil, fmt.Errorf("operation %q request body schema names: %w", operation.OperationID, err)
			}

			schemas = append(schemas, definitions...)
			c.JSONRequestBodyOperations = append(c.JSONRequestBodyOperations, JSONRequestBodyOperation{
				OperationID: operation.OperationID,
				TypeName:    schema.SchemaTypeName(),
			})
		}
	}

	return schemas, nil
}

func SchemaFromOpenAPISchema(schema *openapi3.Schema) (Schema, error) {
	if schema == nil {
		return nil, fmt.Errorf("openapi schema is nil")
	}

	base := BaseSchema{
		Nullable: schema.PermitsNull(),
	}
	if schema.Title != "" {
		base.Name = exportedName(schema.Title)
	}

	if len(schema.AllOf) != 0 {
		allOfSchema, err := allOfSchemaFromOpenAPISchema(schema)
		if err != nil {
			return nil, err
		}

		allOfSchema.BaseSchema = base

		return allOfSchema, nil
	}

	schemaType, err := schemaType(schema)
	if err != nil {
		return nil, err
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
			Format:     schema.Format,
		}, nil
	case openapi3.TypeBoolean:
		return &BoolSchema{
			BaseSchema: base,
		}, nil
	case openapi3.TypeNumber:
		return &NumberSchema{
			BaseSchema: base,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported schema type %q", schemaType)
	}
}

func allOfSchemaFromOpenAPISchema(schema *openapi3.Schema) (*AllOfSchema, error) {
	if len(schema.AllOf) == 0 {
		return nil, fmt.Errorf("allOf schema has no items")
	}

	allOfSchema := &AllOfSchema{
		Schemas: make([]Schema, 0, len(schema.AllOf)),
	}

	for i, schemaRef := range schema.AllOf {
		childSchema, err := schemaFromOpenAPISchemaRef(schemaRef)
		if err != nil {
			return nil, fmt.Errorf("allOf schema %d: %w", i+1, err)
		}

		err = setSchemaTypeNameIfEmpty(childSchema, fmt.Sprintf("AllOf%d", i+1))
		if err != nil {
			return nil, fmt.Errorf("allOf schema %d name: %w", i+1, err)
		}

		allOfSchema.Schemas = append(allOfSchema.Schemas, childSchema)
	}

	return allOfSchema, nil
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

		err = setSchemaTypeNameIfEmpty(additionalPropertiesObject, "AdditionalProperty")
		if err != nil {
			return nil, fmt.Errorf("additionalProperties schema name: %w", err)
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

		err = setSchemaTypeNameIfEmpty(propertyObject, propertyName)
		if err != nil {
			return nil, fmt.Errorf("property %q schema name: %w", propertyName, err)
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

	err = setSchemaTypeNameIfEmpty(items, "Item")
	if err != nil {
		return nil, fmt.Errorf("array items schema name: %w", err)
	}

	return &ArraySchema{
		Items: items,
	}, nil
}

func setSchemaTypeNameIfEmpty(schema Schema, name string) error {
	if schema == nil {
		return fmt.Errorf("nil schema")
	}

	base := schema.Base()
	if base == nil {
		return fmt.Errorf("schema %T has nil base", schema)
	}

	if base.Name == "" {
		base.Name = exportedName(name)
	}

	return nil
}

// TODO, I have high concern for this function. But we would need first to get better testing than this. It looks to me that it doesn't try to find the reffed value at all.
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
// I thought that openapi3.Schema would already do that for us, but perhaps not.
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

// TODO, validate if this is true. I thought that strings for instance could also be inferred.
func inferredSchemaType(schema *openapi3.Schema) (string, error) {
	if len(schema.Properties) != 0 || len(schema.Required) != 0 || schema.AdditionalProperties.Has != nil || schema.AdditionalProperties.Schema != nil {
		return openapi3.TypeObject, nil
	}

	if schema.Items != nil {
		return openapi3.TypeArray, nil
	}

	return "", fmt.Errorf("openapi schema has no type")
}
