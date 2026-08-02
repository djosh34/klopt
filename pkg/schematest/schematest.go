// Package schematest builds schema-aware JSON request-body test cases.
package schematest

import "errors"

// errBuildNotImplemented marks the temporary non-authoritative scaffold.
var errBuildNotImplemented = errors.New("schematest: Build is not implemented")

// Input selects one operation from an OpenAPI document and limits search work.
type Input struct {
	OpenAPI     []byte
	OperationID string
	MaxSteps    uint64
}

// Case is one complete JSON request body and its expected validity.
type Case struct {
	JSON  []byte
	Valid bool
}

// StopReason describes why a successful Build stopped.
type StopReason string

const (
	// SpaceExhausted means Build consumed its finite search space.
	SpaceExhausted StopReason = "space_exhausted"
	// MaxStepsReached means Build could not charge its next assignment.
	MaxStepsReached StopReason = "max_steps_reached"
)

// Report summarizes a successful Build.
type Report struct {
	Stop      StopReason
	Steps     uint64
	Covered   []string
	Uncovered []string
}

// Build streams test cases for one OpenAPI request body.
func Build(input Input, yield func(Case) error) (Report, error) {
	_ = input
	_ = yield

	return Report{}, errBuildNotImplemented
}
