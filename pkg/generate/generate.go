// Package generate writes compiled request-body validations as Go source.
package generate

import (
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
	parsed, err := validation.Parse(openAPI)
	if err != nil {
		return err
	}

	files, err := render(packageName, openAPI, parsed)
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
