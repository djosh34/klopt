package patterngenerator

import (
	"errors"
	"fmt"
)

// ErrNoValues means exhaustive ASCII reachability proved the requested language empty.
var ErrNoValues = errors.New("pattern requirements have no ASCII values")

// ErrTooComplex identifies a fixed resource-limit failure.
var ErrTooComplex = errors.New("pattern generation is too complex")

// ComplexityError reports one fixed limit and the value that exceeded it.
type ComplexityError struct {
	Phase    string
	Limit    string
	Maximum  uint64
	Observed uint64
}

// Error formats the resource-limit failure.
func (complexityError *ComplexityError) Error() string {
	return fmt.Sprintf(
		"pattern generation %s exceeds %s limit: maximum %d, observed %d",
		complexityError.Phase,
		complexityError.Limit,
		complexityError.Maximum,
		complexityError.Observed,
	)
}

// Is supports errors.Is(err, ErrTooComplex).
func (complexityError *ComplexityError) Is(target error) bool {
	return target == ErrTooComplex
}

// RequirementError retains the original requirement index and source.
type RequirementError struct {
	Index  int
	Source string
	Cause  error
}

// Error formats one per-pattern construction failure.
func (requirementError *RequirementError) Error() string {
	return fmt.Sprintf(
		"pattern requirement %d (%q): %v",
		requirementError.Index,
		requirementError.Source,
		requirementError.Cause,
	)
}

// Unwrap exposes the parser, regexp compiler, capability, or complexity error.
func (requirementError *RequirementError) Unwrap() error {
	return requirementError.Cause
}

// CapabilityError reports raw Go syntax accepted by regexp but unsupported by this backend.
type CapabilityError struct {
	Feature string
}

// Error formats the unsupported raw-generator capability.
func (capabilityError *CapabilityError) Error() string {
	return "raw Go regexp generator does not support " + capabilityError.Feature
}
