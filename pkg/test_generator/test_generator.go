// Package testgenerator compiles OpenAPI request bodies into deterministic native-fuzz samples.
//
//nolint:godoclint,mnd // Fixed budgets and private wiring stay behind Generator.
package testgenerator

import (
	"errors"
	"fmt"

	"github.com/djosh34/klopt/pkg/internal/program" //nolint:depguard // Public adapter owns the private program seam.
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/test_generator/internal/suite"
	"github.com/djosh34/klopt/pkg/validation"
)

var decodeLimits = program.Limits{
	MaxSteps:       100_000,
	MaxOutputBytes: 1_000_000,
	MaxDepth:       64,
	MaxSolverWork:  100_000,
	MaxSolverBytes: 16_000_000,
}

// Validator checks one operation's generated request body.
type Validator func(operationID string, body []byte) error

// Sample is one decoded request body and its expected validation result.
type Sample struct {
	OperationID string
	Body        []byte
	ExpectValid bool
}

// Generator is one compiled OpenAPI document.
type Generator struct {
	compiled *suite.CompiledSuite
	runtime  map[string]validation.RequestValidation
}

// Compile admits one OpenAPI document and creates one immutable graph program.
func Compile(
	document []byte,
	patternOptions ...patternvalidator.Option,
) (*Generator, error) {
	compiled, err := suite.CompileSuite(document, patternOptions...)
	if err != nil {
		return nil, err
	}

	runtime, err := validation.Parse(document, patternOptions...)
	if err != nil {
		return nil, err
	}

	return &Generator{compiled: compiled, runtime: runtime}, nil
}

// Empty reports whether the document has any JSON request body to generate.
func (generator *Generator) Empty() bool {
	return generator == nil || len(generator.compiled.Operations) == 0
}

// Decode maps arbitrary native fuzz bytes directly to one request body.
func (generator *Generator) Decode(input []byte) (Sample, error) {
	if generator == nil {
		return Sample{}, errors.New("decode with nil request generator")
	}

	if generator.Empty() {
		return Sample{}, errors.New("decode document with no JSON request bodies")
	}

	decoded, err := generator.compiled.Program.Decode(input, decodeLimits)
	if err != nil {
		return Sample{}, err
	}

	if int(decoded.Operation) >= len(generator.compiled.Operations) {
		return Sample{}, fmt.Errorf("decoded unknown operation %d", decoded.Operation)
	}

	body, err := decoded.Value.MarshalJSON()
	if err != nil {
		return Sample{}, fmt.Errorf("marshal decoded request body: %w", err)
	}

	return Sample{
		OperationID: generator.compiled.Operations[decoded.Operation].ID,
		Body:        append([]byte(nil), body...),
		ExpectValid: decoded.Expect == program.ExpectValid,
	}, nil
}

// Check compares the expected result independently with runtime and generated validators.
func (generator *Generator) Check(sample Sample, generated Validator) error {
	if generator == nil {
		return errors.New("check with nil request generator")
	}

	if generated == nil {
		return errors.New("generated validator is nil")
	}

	runtimeValidation, ok := generator.runtime[sample.OperationID]
	if !ok || runtimeValidation.Body == nil {
		return fmt.Errorf("operation %q has no runtime request-body validation", sample.OperationID)
	}

	var mismatches []error

	runtimeAccepted := len(runtimeValidation.Body.Validate(sample.Body)) == 0
	if runtimeAccepted != sample.ExpectValid {
		mismatches = append(mismatches, fmt.Errorf(
			"runtime validator for %q returned valid=%t, want %t for %s",
			sample.OperationID,
			runtimeAccepted,
			sample.ExpectValid,
			sample.Body,
		))
	}

	generatedErr := generated(sample.OperationID, sample.Body)

	generatedAccepted := generatedErr == nil
	if generatedAccepted != sample.ExpectValid {
		message := fmt.Sprintf(
			"generated validator for %q returned valid=%t, want %t for %s",
			sample.OperationID, generatedAccepted, sample.ExpectValid, sample.Body,
		)
		if generatedErr == nil {
			mismatches = append(mismatches, errors.New(message))
		} else {
			mismatches = append(mismatches, fmt.Errorf("%s: %w", message, generatedErr))
		}
	}

	return errors.Join(mismatches...)
}

// Close releases resources held by independent differential validators.
func (generator *Generator) Close() {
	if generator == nil {
		return
	}

}

// ResourceLimited reports whether Decode stopped at an explicit runtime or solver budget.
func ResourceLimited(err error) bool {
	var (
		limit    *program.LimitError
		resource *program.ResourceError
	)

	return errors.As(err, &limit) || errors.As(err, &resource)
}
