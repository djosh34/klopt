package generate

import (
	"fmt"
	"strings"
	"unicode"
)

func namedSchemaDefinitions(schema Schema) ([]Schema, error) {
	var definitions []Schema

	err := nameSchema(schema, "", &definitions)
	if err != nil {
		return nil, err
	}

	return definitions, nil
}

func nameSchema(schema Schema, parentName string, definitions *[]Schema) error {
	if schema == nil {
		return fmt.Errorf("nil schema")
	}

	base := schema.Base()
	if base == nil {
		return fmt.Errorf("schema %T has nil base", schema)
	}

	if base.Name == "" {
		return fmt.Errorf("schema %T has no type name", schema)
	}

	base.Name = exportedName(parentName + base.Name)

	*definitions = append(*definitions, schema)

	for _, child := range schema.ChildSchemas() {
		err := nameSchema(child, base.Name, definitions)
		if err != nil {
			return fmt.Errorf("child schema: %w", err)
		}
	}

	return nil
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
