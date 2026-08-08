// Package schematest builds schema-aware JSON request-body test cases.
package schematest

import (
	"errors"
	"fmt"
)

// errBuildNotImplemented remains for the pre-B1 scaffold regression guard.
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
//
//nolint:cyclop // Admission, planning, streaming, and normal-stop handling are one public seam.
func Build(input Input, yield func(Case) error) (Report, error) {
	if yield == nil {
		return Report{}, errors.New("schematest: nil case callback")
	}

	model, err := parseInput(input)
	if err != nil {
		return Report{}, err
	}

	plan, err := makePlan(model)
	if err != nil {
		return Report{}, fmt.Errorf("plan schema obligations: %w", err)
	}

	covered := make(map[string]bool, len(plan.obligations))
	searchState := &search{
		model:    model,
		maxSteps: input.MaxSteps,
	}

	if input.MaxSteps == 0 {
		return buildReport(plan, searchState.steps, MaxStepsReached, covered), nil
	}

	stop := SpaceExhausted

	for _, target := range plan.validTargets {
		row, found, searchErr := findTargetRow(plan, target, searchState)
		if searchErr != nil {
			if errors.Is(searchErr, errMaxSteps) {
				stop = MaxStepsReached

				break
			}

			return Report{}, searchErr
		}

		if !found {
			continue
		}

		encoded, marshalErr := marshalStrict(row)
		if marshalErr != nil {
			return Report{}, fmt.Errorf("serialize generated row: %w", marshalErr)
		}

		result := evaluate(model, row)
		if result.err != nil || !result.valid {
			if result.err != nil {
				return Report{}, fmt.Errorf("re-evaluate generated row: %w", result.err)
			}

			return Report{}, errors.New("generated row was not oracle-valid")
		}

		markObservedValidTargets(plan, result, covered)

		callbackErr := yield(Case{JSON: encoded, Valid: true})
		if callbackErr != nil {
			return Report{}, callbackErr
		}
	}

	if stop != SpaceExhausted {
		return buildReport(plan, searchState.steps, stop, covered), nil
	}

	stop, err = streamBasicFaults(plan, searchState, covered, yield)
	if err != nil {
		return Report{}, err
	}

	return buildReport(plan, searchState.steps, stop, covered), nil
}

// markObservedValidTargets records every valid obligation observed by one row.
func markObservedValidTargets(plan *searchPlan, result evaluation, covered map[string]bool) {
	for _, target := range plan.validTargets {
		if levelWasObserved(result.observedRecords(), target.expected) ||
			compositionLevelWasObserved(result, target.expected) {
			covered[target.obligation.String()] = true
		}
	}
}

// buildReport renders coverage in the plan's canonical obligation order.
func buildReport(plan *searchPlan, steps uint64, stop StopReason, covered map[string]bool) Report {
	report := Report{Stop: stop, Steps: steps}

	for _, obligation := range plan.obligations {
		identity := obligation.String()
		if covered[identity] {
			report.Covered = append(report.Covered, identity)
		} else {
			report.Uncovered = append(report.Uncovered, identity)
		}
	}

	return report
}
