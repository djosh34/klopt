//nolint:godoclint // The private template helpers are local implementation details.
package generate

import (
	"bytes"
	"embed"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"text/template"

	"github.com/djosh34/klopt/pkg/names"
	"github.com/djosh34/klopt/pkg/validation"
	"golang.org/x/tools/imports"
)

//go:embed templates/*.go.tmpl
var templateFiles embed.FS

type patternSettings struct {
	RejectNonASCII bool
	UseRE2         bool
}

type operationRender struct {
	OperationID string
	Name        string
	Body        *validation.Validation
	Query       *validation.QueryDecoder
	Path        *validation.PathDecoder
}

func render(
	packageName string,
	openAPI []byte,
	parsed map[string]validation.RequestValidation,
	settings patternSettings,
) (map[string][]byte, error) {
	templates, err := template.ParseFS(templateFiles, "templates/*.go.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	operations := make([]operationRender, 0, len(parsed))
	hasQuery := false
	hasPath := false

	for _, operationID := range slices.Sorted(maps.Keys(parsed)) {
		name, nameErr := names.RequestValidation(operationID)
		if nameErr != nil {
			return nil, fmt.Errorf("render operation ID %q: %w", operationID, nameErr)
		}

		request := parsed[operationID]
		operations = append(operations, operationRender{
			OperationID: operationID,
			Name:        name,
			Body:        request.Body,
			Query:       request.Query,
			Path:        request.Path,
		})
		hasQuery = hasQuery || request.Query != nil
		hasPath = hasPath || request.Path != nil
	}

	data := struct {
		Package    string
		OpenAPI    string
		Operations []operationRender
		HasQuery   bool
		HasPath    bool
		Pattern    patternSettings
	}{
		Package:    packageName,
		OpenAPI:    strconv.Quote(string(openAPI)),
		Operations: operations,
		HasQuery:   hasQuery,
		HasPath:    hasPath,
		Pattern:    settings,
	}

	validate, err := executeTemplate(templates, "validate.go.tmpl", data)
	if err != nil {
		return nil, err
	}

	validateTest, err := executeTemplate(templates, "validate_test.go.tmpl", data)
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		"validate.go":      validate,
		"validate_test.go": validateTest,
	}, nil
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
