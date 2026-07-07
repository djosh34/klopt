package generate

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"strings"
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
	Schemas     []Schema
	UsesRFC3339 bool
}

type modelsTestTemplateContext struct {
	OpenAPI       string
	Operations    []JSONRequestBodyOperation
	ObjectSchemas []modelObjectSchemaTest
}

type modelObjectSchemaTest struct {
	OperationID string
	TypeName    string
}

func (m modelObjectSchemaTest) TestName() string {
	return m.TypeName + "MalformedObjectJSON"
}

func renderModelsFile(schemas []Schema) ([]byte, error) {
	templates, err := parsedGenerateTemplates()
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer

	err = templates.ExecuteTemplate(&out, "file.go.tmpl", fileTemplateContext{
		Schemas:     schemas,
		UsesRFC3339: usesRFC3339(schemas),
	})
	if err != nil {
		return nil, err
	}

	formatted, err := format.Source(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated models.go: %w", err)
	}

	return formatted, nil
}

func usesRFC3339(schemas []Schema) bool {
	for _, schema := range schemas {
		stringSchema, ok := schema.(*StringSchema)
		if ok && stringSchema.Format == "date-time" {
			return true
		}
	}

	return false
}

func renderModelsTestFile(openAPI []byte, operations []JSONRequestBodyOperation, schemas []Schema) ([]byte, error) {
	if bytes.Contains(openAPI, []byte("`")) {
		return nil, fmt.Errorf("openapi source contains backtick")
	}

	templates, err := parsedGenerateTemplates()
	if err != nil {
		return nil, err
	}

	objectTests, err := objectSchemaTests(operations, schemas)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer

	err = templates.ExecuteTemplate(&out, "models_test.go.tmpl", modelsTestTemplateContext{
		OpenAPI:       string(openAPI),
		Operations:    operations,
		ObjectSchemas: objectTests,
	})
	if err != nil {
		return nil, err
	}

	formatted, err := format.Source(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated models_test.go: %w", err)
	}

	return formatted, nil
}

func objectSchemaTests(operations []JSONRequestBodyOperation, schemas []Schema) ([]modelObjectSchemaTest, error) {
	tests := make([]modelObjectSchemaTest, 0, len(schemas))
	for _, schema := range schemas {
		if _, ok := schema.(*ObjectSchema); !ok {
			continue
		}

		operationID, err := schemaOperationID(operations, schema.SchemaTypeName())
		if err != nil {
			return nil, err
		}

		tests = append(tests, modelObjectSchemaTest{
			OperationID: operationID,
			TypeName:    schema.SchemaTypeName(),
		})
	}

	return tests, nil
}

func schemaOperationID(operations []JSONRequestBodyOperation, schemaTypeName string) (string, error) {
	var match JSONRequestBodyOperation

	for _, operation := range operations {
		if schemaTypeName != operation.TypeName && !strings.HasPrefix(schemaTypeName, operation.TypeName) {
			continue
		}

		if len(operation.TypeName) <= len(match.TypeName) {
			continue
		}

		match = operation
	}

	if match.OperationID == "" {
		return "", fmt.Errorf("object schema %q has no source operation", schemaTypeName)
	}

	return match.OperationID, nil
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
