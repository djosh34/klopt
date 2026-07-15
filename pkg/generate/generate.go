// Package generate writes compiled request-body validations and query decoders as Go source.
package generate

import (
	"fmt"
	"go/token"
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

// Generate parses one OpenAPI document and writes validate.go and validate_test.go.
func Generate(dir string, packageName string, openAPI []byte) error {
	parsed, queryDecoders, err := validation.Parse(openAPI)
	if err != nil {
		return err
	}

	for operationID := range parsed {
		if !isSafeOperationIdentifier(operationID) {
			return fmt.Errorf("operation ID %q cannot be used as a generated Go identifier", operationID)
		}
	}

	files, err := render(packageName, openAPI, parsed, queryDecoders)
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

// isSafeOperationIdentifier reports whether an operation ID can name a generated package variable.
func isSafeOperationIdentifier(operationID string) bool {
	if !token.IsIdentifier(operationID) || operationID == "_" || operationID == "init" {
		return false
	}

	switch operationID {
	case "byte", "error", "errors", "json", "jsonvalue", "openAPI", "regexp", "string", "testing",
		"testgenerator", "TestValidations", "true", "validation", "validations":
		return false
	default:
		return true
	}
}
