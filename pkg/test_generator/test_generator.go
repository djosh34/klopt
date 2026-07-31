// Package testgenerator exposes the request-test generator boundary.
package testgenerator

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/validation"
)

// Status reports whether Decode produced a sample.
type Status uint8

const (
	// Exhausted means this construction attempt produced no sample.
	Exhausted Status = iota
	// Generated means Decode produced a sample.
	Generated
)

// Sample is one decoded request body and its expected validation result.
type Sample struct {
	OperationID string
	Body        []byte
	ExpectValid bool
}

// Validator checks one operation's generated request body.
type Validator func(operationID string, body []byte) (accepted bool, err error)

// operation retains one parsed request-body validation.
type operation struct {
	id         string
	root       *expression
	faultPlans []faultPlan
	runtime    validation.RequestValidation
}

// Generator is one compiled OpenAPI document.
type Generator struct {
	operations []operation
	byID       map[string]int
}

// Compile admits one OpenAPI document and lowers its request bodies to expressions.
func Compile(
	document []byte,
	patternOptions ...patternvalidator.Option,
) (*Generator, error) {
	parsed, err := validation.Parse(document, patternOptions...)
	if err != nil {
		return nil, err
	}

	lowerer := expressionLowerer{byValidation: make(map[*validation.Validation]*expression)}
	ids := slices.Sorted(maps.Keys(parsed))

	operations := make([]operation, 0, len(ids))
	for _, id := range ids {
		runtime := parsed[id]
		if runtime.Body == nil {
			continue
		}

		root, lowerErr := lowerer.lower(runtime.Body)
		if lowerErr != nil {
			return nil, fmt.Errorf("lower operation %q: %w", id, lowerErr)
		}

		faultPlans, planErr := enumerateFaultPlans(root)
		if planErr != nil {
			return nil, fmt.Errorf("enumerate operation %q fault plans: %w", id, planErr)
		}

		operations = append(operations, operation{
			id: id, root: root, faultPlans: faultPlans, runtime: runtime,
		})
	}

	byID := make(map[string]int, len(operations))
	for index := range operations {
		byID[operations[index].id] = index
	}

	return &Generator{operations: operations, byID: byID}, nil
}

// Decode makes one valid or exhausted construction attempt from one zero-tailed tape.
func (generator *Generator) Decode(tape []byte) (Sample, Status, error) {
	if generator == nil {
		return Sample{}, Exhausted, errors.New("decode with nil request generator")
	}

	if len(generator.operations) == 0 {
		return Sample{}, Exhausted, nil
	}

	cursor := newTapeCursor(tape)
	operationWord := cursor.takeWord()
	verdictWord := cursor.takeWord()
	selectedOperation := generator.operations[operationWord%uint64(len(generator.operations))]

	wantValid := verdictWord&1 == 0

	var built buildResult
	if wantValid {
		built = buildValid(selectedOperation.root, cursor)
	} else {
		planWord := cursor.takeWord()

		if len(selectedOperation.faultPlans) == 0 {
			return Sample{}, Exhausted, nil
		}

		planIndex := planWord % uint64(len(selectedOperation.faultPlans))
		plan := &selectedOperation.faultPlans[planIndex]
		built = buildFocusedInvalid(selectedOperation.root, plan, cursor)
	}

	if built.err != nil {
		return Sample{}, Exhausted, fmt.Errorf("build operation %q: %w", selectedOperation.id, built.err)
	}

	if built.state != buildComplete {
		return Sample{}, Exhausted, nil
	}

	return finishSample(selectedOperation, built.value, wantValid)
}

//nolint:godoclint // The private finalizer is covered by public Decode semantics.
func finishSample(
	operation operation,
	value jsonvalue.Value,
	wantValid bool,
) (Sample, Status, error) {
	holds, err := expressionHolds(operation.root, value)
	if err != nil {
		return Sample{}, Exhausted, fmt.Errorf(
			"evaluate operation %q root: %w",
			operation.id,
			err,
		)
	}

	if holds != wantValid {
		return Sample{}, Exhausted, nil
	}

	body, err := value.MarshalJSON()
	if err != nil {
		return Sample{}, Exhausted, fmt.Errorf(
			"marshal generated body for operation %q: %w",
			operation.id,
			err,
		)
	}

	return Sample{
		OperationID: operation.id,
		Body:        append([]byte(nil), body...),
		ExpectValid: wantValid,
	}, Generated, nil
}

// Check independently compares runtime and generated validator verdicts.
func (generator *Generator) Check(sample Sample, generated Validator) error {
	if generator == nil {
		return errors.New("check with nil request generator")
	}

	if generated == nil {
		return errors.New("check with nil generated validator")
	}

	operationIndex, ok := generator.byID[sample.OperationID]
	if !ok {
		return fmt.Errorf("check unknown operation %q", sample.OperationID)
	}

	operation := generator.operations[operationIndex]
	if operation.runtime.Body == nil {
		return fmt.Errorf("check operation %q without body validation", operation.id)
	}

	runtimeAccepted := len(operation.runtime.Body.Validate(sample.Body)) == 0
	generatedAccepted, generatedErr := generated(sample.OperationID, sample.Body)

	var errs []error
	if runtimeAccepted != sample.ExpectValid {
		errs = append(errs, fmt.Errorf(
			"runtime validator verdict for operation %q is %t, expected %t",
			operation.id, runtimeAccepted, sample.ExpectValid,
		))
	}

	if generatedErr != nil {
		errs = append(errs, fmt.Errorf(
			"generated validator for operation %q: %w",
			operation.id, generatedErr,
		))
	} else if generatedAccepted != sample.ExpectValid {
		errs = append(errs, fmt.Errorf(
			"generated validator verdict for operation %q is %t, expected %t",
			operation.id, generatedAccepted, sample.ExpectValid,
		))
	}

	return errors.Join(errs...)
}
