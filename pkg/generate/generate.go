// Package generate writes compiled request-body validations as Go source.
package generate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/djosh34/decode_and_validate_generator/pkg/validation"
)

const (
	// directoryMode is used for the generated directory.
	directoryMode = 0o755
	// fileMode is used for generated Go files.
	fileMode = 0o644
)

// Operation names one validation and its generated test.
type Operation struct {
	OperationID string
	Variable    string
	Test        string
}

// Generate parses every operation and writes validate.go and validate_test.go.
func Generate(dir string, packageName string, openAPI []byte, operations []Operation) error {
	parsed := make([]*validation.Validation, len(operations))
	for index, operation := range operations {
		compiled, err := validation.Parse(openAPI, operation.OperationID)
		if err != nil {
			return fmt.Errorf("parse operation %q: %w", operation.OperationID, err)
		}

		parsed[index] = compiled
	}

	files, err := render(packageName, openAPI, operations, parsed)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, directoryMode); err != nil {
		return err
	}

	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), contents, fileMode); err != nil {
			return err
		}
	}

	return nil
}
