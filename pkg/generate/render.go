//nolint:godoclint // The private template helpers are local implementation details.
package generate

import (
	"bytes"
	"embed"
	"fmt"
	"strconv"
	"text/template"

	"github.com/djosh34/klopt/pkg/validation"
	"golang.org/x/tools/imports"
)

//go:embed templates/*.go.tmpl
var templateFiles embed.FS

type patternSettings struct {
	RejectNonASCII bool
	UseRE2         bool
}

func render(
	packageName string,
	openAPI []byte,
	parsed map[string]*validation.Validation,
	queryDecoders map[string]*validation.QueryDecoder,
	settings patternSettings,
) (map[string][]byte, error) {
	templates, err := template.ParseFS(templateFiles, "templates/*.go.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	data := struct {
		Package       string
		OpenAPI       string
		Validations   map[string]*validation.Validation
		QueryDecoders map[string]*validation.QueryDecoder
		Pattern       patternSettings
	}{
		Package:       packageName,
		OpenAPI:       strconv.Quote(string(openAPI)),
		Validations:   parsed,
		QueryDecoders: queryDecoders,
		Pattern:       settings,
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
