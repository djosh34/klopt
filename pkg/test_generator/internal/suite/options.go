package suite

import (
	"fmt"

	"github.com/djosh34/klopt/pkg/patternvalidator"
)

// WorkLimits bounds exact compile-time representation work only.
type WorkLimits struct {
	SetArenaBytes   uint64
	GraphNodes      uint64
	GraphEdges      uint64
	TransitionBytes uint64
	ProofSteps      uint64
	UnicodeClasses  uint64
	ProgramBytes    uint64
}

// CompilerOptions contains compile-time controls and cannot alter schema meaning.
type CompilerOptions struct {
	WorkLimits    WorkLimits
	PatternOption patternvalidator.Option
}

// ResourceError reports transactional compile-time resource exhaustion.
type ResourceError struct {
	Pass     string
	Resource string
	Pointer  string
	Limit    uint64
	Observed uint64
}

// Error formats the charged compile resource and source location.
func (resourceError *ResourceError) Error() string {
	return fmt.Sprintf(
		"%s at %s exceeds %s limit: maximum %d, observed %d",
		resourceError.Pass,
		resourceError.Pointer,
		resourceError.Resource,
		resourceError.Limit,
		resourceError.Observed,
	)
}
