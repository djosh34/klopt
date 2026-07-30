// Package suite adapts one OpenAPI document to one graph program.
package suite

import (
	"maps"
	"slices"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/internal/program" //nolint:depguard // The suite is the OpenAPI adapter for the deep program module.
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/validation"
)

// OperationInfo maps a program root back to its OpenAPI operation.
type OperationInfo struct {
	ID     string
	Method string
	Path   string
}

// CompiledSuite is one document program plus stable operation metadata.
type CompiledSuite struct {
	Program    program.Program
	Operations []OperationInfo
}

// CompileSuite parses and lowers every JSON request body in one document.
func CompileSuite(
	document []byte,
	patternOptions ...patternvalidator.Option,
) (*CompiledSuite, error) {
	requestValidations, err := validation.Parse(document, patternOptions...)
	if err != nil {
		return nil, err
	}

	sources, err := oas.Parse(document)
	if err != nil {
		return nil, err
	}

	roots := make([]*validation.Validation, 0, len(sources))

	operations := make([]OperationInfo, 0, len(sources))
	for _, operationID := range slices.Sorted(maps.Keys(sources)) {
		source := sources[operationID]

		body := requestValidations[operationID].Body
		if body == nil {
			continue
		}

		roots = append(roots, body)
		operations = append(operations, OperationInfo{
			ID: operationID, Method: source.Method, Path: source.PathTemplate,
		})
	}

	compiled, err := program.Compile(roots)
	if err != nil {
		return nil, err
	}

	return &CompiledSuite{Program: *compiled, Operations: operations}, nil
}
