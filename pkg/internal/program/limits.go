//nolint:godoclint,mnd // Private defaults and charging stay behind typed limits.
package program

import "fmt"

// CompileLimits bounds immutable graph construction before Program is allocated.
type CompileLimits struct {
	MaxNodes        uint64
	MaxFacts        uint64
	MaxProgramBytes uint64
}

func defaultCompileLimits() CompileLimits {
	return CompileLimits{
		MaxNodes:        1_000_000,
		MaxFacts:        1_000_000,
		MaxProgramBytes: 64 * 1024 * 1024,
	}
}

// Limits contains the only runtime resource controls used by Decode.
type Limits struct {
	MaxSteps       uint64
	MaxOutputBytes uint64
	MaxDepth       uint64
	MaxSolverWork  uint64
	MaxSolverBytes uint64
}

// LimitError reports transactional runtime resource exhaustion.
type LimitError struct {
	Resource string
	Limit    uint64
	Observed uint64
}

// Error formats one exhausted runtime limit.
func (limitError *LimitError) Error() string {
	return fmt.Sprintf(
		"program decode exceeds %s limit: maximum %d, observed %d",
		limitError.Resource,
		limitError.Limit,
		limitError.Observed,
	)
}

func checkLimit(resource string, maximum uint64, observed uint64) error {
	if observed <= maximum {
		return nil
	}

	return &LimitError{Resource: resource, Limit: maximum, Observed: observed}
}

// ResourceError reports exact solver work that exceeded its configured budget.
type ResourceError struct {
	Resource string
	Limit    uint64
	Observed uint64
}

// Error formats one exhausted solver resource.
func (resourceError *ResourceError) Error() string {
	return fmt.Sprintf(
		"program exceeds %s limit: maximum %d, observed %d",
		resourceError.Resource,
		resourceError.Limit,
		resourceError.Observed,
	)
}

func checkedAdd(left uint64, right uint64) (uint64, bool) {
	if right > ^uint64(0)-left {
		return ^uint64(0), false
	}

	return left + right, true
}

func checkedMul(left uint64, right uint64) (uint64, bool) {
	if left != 0 && right > ^uint64(0)/left {
		return ^uint64(0), false
	}

	return left * right, true
}
