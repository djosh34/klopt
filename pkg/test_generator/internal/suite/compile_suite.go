package suite

import (
	"fmt"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/internal/program" //nolint:depguard // Final executable CasePlan seam required by the hosted plan.
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/validation"
)

// CasePlan is one final executable validator obligation.
type CasePlan struct {
	Name    string
	Expect  ExpectedResult
	Source  ConstraintSource
	Program program.Program
	Seeds   [][]byte
}

// CompiledSuite is an immutable atomically published request-body suite.
type CompiledSuite struct {
	Cases []CasePlan
}

// CompileSuite admits, lowers, plans, compiles, certifies, weights, and seals one request schema.
func CompileSuite(source oas.Source, options CompilerOptions) (*CompiledSuite, error) {
	if len(source.RequestSchema.Raw) == 0 {
		return &CompiledSuite{}, nil
	}

	patternOptions := []patternvalidator.Option(nil)
	if options.PatternOption != nil {
		patternOptions = append(patternOptions, options.PatternOption)
	}

	admitted, err := validation.AdmitRequestSchema(
		source, source.RequestSchema, validation.UseRequestGeneration, patternOptions...,
	)
	if err != nil {
		return nil, err
	}

	semantic, err := compileSemanticWithPattern(admitted, options.PatternOption)
	if err != nil {
		return nil, err
	}

	const approximateSetEntryBytes = 16

	setBytes := uint64(
		len(semantic.Semantic.Sets.Nodes)+len(semantic.Semantic.Sets.Atoms),
	) * approximateSetEntryBytes
	if err := chargeWork(
		"semantic", "set arena bytes", source.RequestSchema.Pointer,
		options.WorkLimits.SetArenaBytes, setBytes,
	); err != nil {
		return nil, err
	}

	result := make([]CasePlan, 0, len(semantic.Cases))
	for _, specification := range semantic.Cases {
		compiled, compileErr := compileCaseProgram(
			&semantic.Semantic.Sets,
			specification.values,
			specification.source,
			options.WorkLimits,
		)
		if compileErr != nil {
			return nil, fmt.Errorf("compile case %q: %w", specification.name, compileErr)
		}

		result = append(result, CasePlan{
			Name: specification.name, Expect: specification.expect, Source: specification.source,
			Program: compiled, Seeds: [][]byte{nil},
		})
	}

	return &CompiledSuite{Cases: result}, nil
}
