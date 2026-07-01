package generate

import (
	"fmt"
	"strings"
	"unicode"
)

func nameSchema(schema Schema, name string) error {
	if schema == nil {
		return fmt.Errorf("nil schema")
	}

	base := schema.Base()
	if base == nil {
		return fmt.Errorf("schema %T has nil base", schema)
	}
	if base.TypeName == "" {
		base.TypeName = exportedName(name)
	}

	switch schema := schema.(type) {
	case *ObjectSchema:
		for _, property := range schema.Properties {
			err := nameSchema(property.Schema, property.PropertyName)
			if err != nil {
				return fmt.Errorf("property %q schema: %w", property.PropertyName, err)
			}
		}

		if schema.AdditionalPropertiesSchema != nil {
			err := nameSchema(schema.AdditionalPropertiesSchema, base.TypeName+"AdditionalProperty")
			if err != nil {
				return fmt.Errorf("additionalProperties schema: %w", err)
			}
		}
	case *ArraySchema:
		err := nameSchema(schema.Items, base.TypeName+"Item")
		if err != nil {
			return fmt.Errorf("array items schema: %w", err)
		}
	case *StringSchema:
	default:
		return fmt.Errorf("unsupported schema %T", schema)
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
