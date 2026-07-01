package generate

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"maps"
	"slices"
	"sync"
	"text/template"
)

//go:embed templates/*.go.tmpl
var templateFS embed.FS

const templatePattern = "templates/*.go.tmpl"

var (
	generateTemplatesOnce sync.Once
	generateTemplates     *template.Template
	generateTemplatesErr  error
)

type fileTemplateContext struct {
	Schemas []Schema
}

func renderModelsFile(schemas []Schema) ([]byte, error) {
	templates, err := parsedGenerateTemplates()
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	err = templates.ExecuteTemplate(&out, "file.go.tmpl", fileTemplateContext{Schemas: schemas})
	if err != nil {
		return nil, err
	}

	formatted, err := format.Source(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated models.go: %w", err)
	}

	return formatted, nil
}

func executeGoTemplate(name string, data any) (string, error) {
	templates, err := parsedGenerateTemplates()
	if err != nil {
		return "", err
	}

	var out bytes.Buffer
	err = templates.ExecuteTemplate(&out, name, data)
	if err != nil {
		return "", err
	}

	return out.String(), nil
}

func parsedGenerateTemplates() (*template.Template, error) {
	generateTemplatesOnce.Do(func() {
		generateTemplates, generateTemplatesErr = template.ParseFS(templateFS, templatePattern)
	})
	if generateTemplatesErr != nil {
		return nil, fmt.Errorf("parse generate templates: %w", generateTemplatesErr)
	}

	return generateTemplates, nil
}

func schemaDefinitions(schemas []Schema) ([]Schema, error) {
	definitionsByName := map[string]Schema{}
	for _, schema := range schemas {
		err := collectSchemaDefinitions(schema, definitionsByName)
		if err != nil {
			return nil, err
		}
	}

	definitions := make([]Schema, 0, len(definitionsByName))
	for _, name := range slices.Sorted(maps.Keys(definitionsByName)) {
		definitions = append(definitions, definitionsByName[name])
	}

	return definitions, nil
}

func collectSchemaDefinitions(schema Schema, definitions map[string]Schema) error {
	if schema == nil {
		return fmt.Errorf("nil schema")
	}

	base := schema.Base()
	if base == nil {
		return fmt.Errorf("schema %T has nil base", schema)
	}
	if base.TypeName == "" {
		return fmt.Errorf("schema %T has no type name", schema)
	}

	if _, exists := definitions[base.TypeName]; exists {
		return nil
	}
	definitions[base.TypeName] = schema

	switch schema := schema.(type) {
	case *ObjectSchema:
		for _, property := range schema.Properties {
			err := collectSchemaDefinitions(property.Schema, definitions)
			if err != nil {
				return fmt.Errorf("property %q schema: %w", property.PropertyName, err)
			}
		}

		if schema.AdditionalPropertiesSchema != nil {
			err := collectSchemaDefinitions(schema.AdditionalPropertiesSchema, definitions)
			if err != nil {
				return fmt.Errorf("additionalProperties schema: %w", err)
			}
		}
	case *ArraySchema:
		err := collectSchemaDefinitions(schema.Items, definitions)
		if err != nil {
			return fmt.Errorf("array items schema: %w", err)
		}
	case *StringSchema:
	default:
		return fmt.Errorf("unsupported schema %T", schema)
	}

	return nil
}
