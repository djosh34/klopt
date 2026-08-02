//nolint:godoclint // The private template helpers are local implementation details.
package generate

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"text/template"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/validation"
	"golang.org/x/tools/imports"
)

//go:embed templates/*.go.tmpl
var templateFiles embed.FS

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
	return renderWithTemplates(templateFiles, packageName, parsed)
}

func renderWithTemplates(
	templateFS fs.FS,
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

	validations, validationIDs := collectValidations(operations)

	validationTemplates, err := template.New("").Funcs(template.FuncMap{
		"validationID": func(compiled *validation.Validation) (int, error) {
			id, ok := validationIDs[compiled]
			if !ok {
				return 0, errors.New("validation is outside rendered graph")
			}

			return id, nil
		},
	}).ParseFS(templateFS, "templates/*.go.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse validation templates: %w", err)
	}

	data := struct {
		Package     string
		Operations  []operationRender
		Validations []*validation.Validation
		HasQuery    bool
		HasPath     bool
	}{
		Package:     packageName,
		Operations:  operations,
		Validations: validations,
		HasQuery:    hasQuery,
		HasPath:     hasPath,
	}

	validate, err := executeTemplate(validationTemplates, "validate.go.tmpl", data)
	if err != nil {
		return nil, err
	}

	return map[string][]byte{"validate.go": validate}, nil
}

// collectValidations assigns one stable generated slot to each shared graph node.
//
//nolint:cyclop // Each validation child family contributes roots to the same graph walk.
func collectValidations(operations []operationRender) ([]*validation.Validation, map[*validation.Validation]int) {
	ids := make(map[*validation.Validation]int)
	validations := make([]*validation.Validation, 0)

	var collect func(*validation.Validation)

	collect = func(compiled *validation.Validation) {
		if compiled == nil {
			return
		}

		if _, exists := ids[compiled]; exists {
			return
		}

		ids[compiled] = len(validations)
		validations = append(validations, compiled)

		collect(compiled.ArrayValidation.Items)

		for _, property := range compiled.ObjectValidation.Properties {
			collect(property.Validation)
		}

		collect(compiled.ObjectValidation.AdditionalPropertiesValidation)

		for _, child := range compiled.AllOfValidations {
			collect(child)
		}

		for _, child := range compiled.AnyOfValidations {
			collect(child)
		}
	}

	for _, operation := range operations {
		collect(operation.Body)

		if operation.Query != nil {
			for _, parameter := range operation.Query.Parameters {
				collect(parameter.Validation)
			}
		}

		if operation.Path != nil {
			for _, parameter := range operation.Path.Parameters {
				collect(parameter.Validation)
			}
		}
	}

	return validations, ids
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
