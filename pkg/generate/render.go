//nolint:godoclint // The private template helpers are local implementation details.
package generate

import (
	"bytes"
	"embed"
	"fmt"
	"maps"
	"slices"
	"text/template"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/validation"
	"golang.org/x/tools/imports"
)

//go:embed templates/*.go.tmpl
var templateFiles embed.FS

var validationTemplates = template.Must(template.ParseFS(templateFiles, "templates/*.go.tmpl"))

type operationRender struct {
	OperationID string
	Name        string
	Body        *validation.Validation
	Query       *validation.QueryDecoderDefinition
	Path        *validation.PathDecoderDefinition
}

func render(
	packageName string,
	parsed map[string]validation.RequestValidation,
) (map[string][]byte, error) {
	operations := make([]operationRender, 0, len(parsed))
	hasQuery := false
	hasPath := false

	for _, operationID := range slices.Sorted(maps.Keys(parsed)) {
		name, nameErr := oas.RequestValidationName(operationID)
		if nameErr != nil {
			return nil, fmt.Errorf("render operation ID %q: %w", operationID, nameErr)
		}

		request := parsed[operationID]

		operation := operationRender{OperationID: operationID, Name: name, Body: request.Body}
		if request.Query != nil {
			definition, definitionErr := request.Query.Definition()
			if definitionErr != nil {
				return nil, fmt.Errorf("render operation ID %q query decoder: %w", operationID, definitionErr)
			}

			operation.Query = &definition
			hasQuery = true
		}

		if request.Path != nil {
			definition, definitionErr := request.Path.Definition()
			if definitionErr != nil {
				return nil, fmt.Errorf("render operation ID %q path decoder: %w", operationID, definitionErr)
			}

			operation.Path = &definition
			hasPath = true
		}

		operations = append(operations, operation)
	}

	data := struct {
		Package    string
		Operations []operationRender
		HasQuery   bool
		HasPath    bool
	}{
		Package:    packageName,
		Operations: operations,
		HasQuery:   hasQuery,
		HasPath:    hasPath,
	}

	validate, err := executeTemplate(validationTemplates, "validate.go.tmpl", data)
	if err != nil {
		return nil, err
	}

	return map[string][]byte{"validate.go": validate}, nil
}

func executeTemplate(templates *template.Template, name string, data any) ([]byte, error) {
	var output bytes.Buffer
	if err := templates.ExecuteTemplate(&output, name, data); err != nil {
		return nil, fmt.Errorf("execute %s: %w", name, err)
	}

	formatted, err := imports.Process(name, output.Bytes(), nil)
	if err != nil {
		return nil, fmt.Errorf("format %s: %w", name, err)
	}

	return formatted, nil
}
