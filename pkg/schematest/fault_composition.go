package schematest

import (
	"errors"
	"fmt"
)

// applyCompositionFault searches one exact aggregate derivative, then applies it
// as one charged assignment to a fresh copy of the current parent.
func applyCompositionFault(parent *jsonValue, fault faultTarget, s *search) (*jsonValue, error) {
	if parent == nil {
		return nil, errors.New("schematest: nil composition fault parent")
	}

	if s == nil || s.model == nil || s.model.root == nil {
		return nil, errors.New("schematest: composition fault application has no model")
	}

	derivative, err := copyJSONValue(parent, make(map[*jsonValue]*jsonValue))
	if err != nil {
		return nil, fmt.Errorf("copy composition fault parent: %w", err)
	}

	assignment, found, err := findCompositionFaultAssignment(fault, s)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, fmt.Errorf("%w: %s", errFaultNotFound, fault.obligation.String())
	}

	assignErr := s.assign()
	if assignErr != nil {
		return nil, assignErr
	}

	assigned, err := copyJSONValue(assignment, make(map[*jsonValue]*jsonValue))
	if err != nil {
		return nil, fmt.Errorf("copy composition fault assignment: %w", err)
	}

	*derivative = *assigned

	return derivative, nil
}

// findCompositionFaultAssignment searches without retaining rejected candidates.
func findCompositionFaultAssignment(fault faultTarget, s *search) (*jsonValue, bool, error) {
	var assignment *jsonValue

	visit := func(value *jsonValue) (bool, error) {
		result := evaluate(s.model, value)
		if result.err != nil {
			return false, fmt.Errorf("evaluate composition fault derivative: %w", result.err)
		}

		matches, err := exactFailureClosure(result.failures, fault.closure)
		if err != nil {
			return false, fmt.Errorf("compare composition fault closure: %w", err)
		}

		if result.valid || !matches || !faultPinsMatch(result, value, fault.pins) {
			return false, nil
		}

		assignment = value

		return true, nil
	}

	found, err := s.walkNode(
		s.model.root,
		s.model.root.occurrence,
		fault.pins,
		rowSearchContext{},
		visit,
	)

	return assignment, found, err
}

// faultNeedsCompositionSearch reports whether one fault must make an anyOf
// closure false atomically. Pure allOf branch faults use the ordinary isolated
// mutation path so unaffected branches remain untouched.
func faultNeedsCompositionSearch(fault faultTarget) bool {
	if fault.obligation.rule == oracleRuleAllOf || fault.obligation.rule == oracleRuleAnyOf {
		return true
	}

	for _, failure := range fault.closure {
		if failure.rule == oracleRuleAnyOf {
			return true
		}
	}

	return false
}
