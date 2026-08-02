// Package generate writes compiled request validations as Go source.
package generate

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/validation"
)

const (
	// directoryMode is used for the generated directory.
	directoryMode = 0o755
	// fileMode is used for generated Go files.
	fileMode = 0o644
)

// ErrNilPatternOption reports a nil pattern option.
var ErrNilPatternOption = errors.New("generate: nil pattern option")

// Generate parses one OpenAPI document and writes validate.go.
func Generate(
	dir string,
	packageName string,
	openAPI []byte,
	patternOption patternvalidator.Option,
) error {
	files, err := GenerateInMemory(packageName, openAPI, patternOption)
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

// GenerateInMemory parses one OpenAPI document and returns validate.go.
//
//nolint:revive // GenerateInMemory is the required public API name.
func GenerateInMemory(
	packageName string,
	openAPI []byte,
	patternOption patternvalidator.Option,
) (map[string][]byte, error) {
	if patternOption == nil {
		return nil, ErrNilPatternOption
	}

	parsed, err := validation.Parse(openAPI, patternOption)
	if err != nil {
		return nil, err
	}

	files, err := render(packageName, parsed)
	if err != nil {
		return nil, err
	}

	return files, nil
}
