//nolint:godoclint // Private charging stays behind the exported typed limit.
package program

import "fmt"

// Limits contains the only runtime resource controls used by Decode.
type Limits struct {
	MaxSteps       uint64
	MaxOutputBytes uint64
	MaxDepth       uint64
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
