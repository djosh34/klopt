package generate

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"
)

func namedOperationSchemas(operations map[string]SchemaObject) ([]SchemaObject, error) {
	operationIDs := slices.Sorted(maps.Keys(operations))
	schemas := make([]SchemaObject, 0, len(operationIDs))
	for _, operationID := range operationIDs {
		schema, err := namedSchemaObject(operations[operationID], operationID)
		if err != nil {
			return nil, fmt.Errorf("operation %q schema: %w", operationID, err)
		}

		schemas = append(schemas, schema)
	}

	return schemas, nil
}

func namedSchemaObject(schemaObject SchemaObject, name string) (SchemaObject, error) {
	switch schema := schemaObject.(type) {
	case ObjectContext:
		properties, err := namedObjectProperties(schema.Properties)
		if err != nil {
			return nil, err
		}

		schema.ContextName = exportedName(name)
		schema.Properties = properties
		if schema.AdditionalPropertiesSchema != nil {
			additionalPropertiesSchema, err := namedSchemaObject(schema.AdditionalPropertiesSchema, schema.ContextName+"AdditionalProperty")
			if err != nil {
				return nil, fmt.Errorf("additionalProperties schema: %w", err)
			}

			schema.AdditionalPropertiesSchema = additionalPropertiesSchema
		}

		return schema, nil
	case *ObjectContext:
		if schema == nil {
			return nil, fmt.Errorf("nil object schema context")
		}

		return namedSchemaObject(*schema, name)
	case StringContext:
		schema.ContextName = exportedName(name)
		return schema, nil
	case *StringContext:
		if schema == nil {
			return nil, fmt.Errorf("nil string schema context")
		}

		return namedSchemaObject(*schema, name)
	case ArrayContext:
		if schema.Items == nil {
			return nil, fmt.Errorf("array schema has nil items")
		}

		schema.ContextName = exportedName(name)
		items, err := namedSchemaObject(schema.Items, schema.ContextName+"Item")
		if err != nil {
			return nil, fmt.Errorf("array items schema: %w", err)
		}

		schema.Items = items
		return schema, nil
	case *ArrayContext:
		if schema == nil {
			return nil, fmt.Errorf("nil array schema context")
		}

		return namedSchemaObject(*schema, name)
	default:
		return nil, fmt.Errorf("unsupported schema context %T", schemaObject)
	}
}

func namedObjectProperties(properties map[string]ObjectFieldContext) (map[string]ObjectFieldContext, error) {
	namedProperties := make(map[string]ObjectFieldContext, len(properties))
	for propertyName, property := range properties {
		schema, err := namedSchemaObject(property.Schema, propertyName)
		if err != nil {
			return nil, fmt.Errorf("property %q schema: %w", propertyName, err)
		}

		property.PropertyName = propertyName
		property.Schema = schema
		namedProperties[propertyName] = property
	}

	return namedProperties, nil
}

func exportedName(name string) string {
	return identifierName(name, true)
}

func unexportedName(name string) string {
	return identifierName(name, false)
}

func identifierName(name string, exported bool) string {
	var out strings.Builder
	upperNext := exported
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			upperNext = true
			continue
		}

		if out.Len() == 0 && unicode.IsDigit(r) {
			out.WriteString("Schema")
		}

		if upperNext {
			out.WriteRune(unicode.ToUpper(r))
			upperNext = false
			continue
		}

		out.WriteRune(r)
	}

	if out.Len() == 0 {
		if exported {
			return "Schema"
		}

		return "schema"
	}

	if exported {
		return out.String()
	}

	runes := []rune(out.String())
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}
