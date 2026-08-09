package schematest

import (
	"errors"
	"fmt"
	"math/big"
)

// errFaultNotFound leaves a planned fault without an isolated derivative uncovered.
var errFaultNotFound = errors.New("schematest: planned fault has no isolated derivative")

// streamBasicFaults visits supported faults in planner order.
func streamBasicFaults(
	plan *searchPlan,
	s *search,
	covered map[string]bool,
	yield func(Case) error,
) (StopReason, error) {
	for _, index := range plan.faultExecution {
		if index < 0 || index >= len(plan.faultSchedule) {
			return "", errors.New("schematest: invalid fault execution order")
		}

		if err := streamBasicFault(plan, plan.faultSchedule[index], s, covered, yield); err != nil {
			if errors.Is(err, errMaxSteps) {
				return MaxStepsReached, nil
			}

			return "", err
		}
	}

	return SpaceExhausted, nil
}

// streamBasicFault replays, applies, verifies, and emits one fault derivative.
//
//nolint:cyclop // Every error and uncovered outcome remains explicit at the streaming boundary.
func streamBasicFault(
	plan *searchPlan,
	fault faultProgram,
	s *search,
	covered map[string]bool,
	yield func(Case) error,
) error {
	// R4 only compiles aggregate closure domains. R7 owns their charged runtime cursor.
	if fault.alternatives != nil {
		return nil
	}

	parent, found, err := regenerateParent(plan, fault, s)
	if err != nil || !found {
		return err
	}

	derivative, err := applyFault(parent, fault, s)
	if errors.Is(err, errFaultNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	result := evaluate(s.model, derivative)
	if result.err != nil {
		return fmt.Errorf("evaluate fault derivative: %w", result.err)
	}

	matches, err := faultFailureClosureMatches(result.failureRecords(), fault)
	if err != nil {
		return fmt.Errorf("compare fault expected: %w", err)
	}

	if result.valid || !matches {
		return nil
	}

	encoded, err := marshalStrict(derivative)
	if err != nil {
		return fmt.Errorf("serialize fault derivative: %w", err)
	}

	covered[fault.obligation.String()] = true

	return yield(Case{JSON: encoded, Valid: false})
}

// regenerateParent replays row search for one fresh, complete oracle-valid parent.
func regenerateParent(plan *searchPlan, fault faultProgram, s *search) (*jsonValue, bool, error) {
	if plan == nil {
		return nil, false, errors.New("schematest: nil search plan")
	}

	if s == nil || s.model == nil || s.model.root == nil {
		return nil, false, errors.New("schematest: parent replay has no model")
	}

	parentRequirements := parentReplayRequirements(fault)

	var parent *jsonValue

	visit := func(value *jsonValue) (bool, error) {
		result := evaluate(s.model, value)
		if result.err != nil {
			return false, fmt.Errorf("evaluate regenerated parent: %w", result.err)
		}

		if !result.valid || !faultRequirementsMatch(result, value, parentRequirements) {
			return false, nil
		}

		parent = value

		return true, nil
	}

	complete, err := s.walkNode(
		s.model.root,
		s.model.root.occurrence,
		parentRequirements,
		rowSearchContext{},
		visit,
	)
	if err != nil {
		return nil, false, err
	}

	return parent, complete, nil
}

// parentReplayRequirements turns mutation-result presence into valid-parent presence.
//
//nolint:cyclop // Presence, type, and enum faults translate distinct requirement dimensions.
func parentReplayRequirements(fault faultProgram) []requirement {
	requirements := copyPlanRequirements(fault.requirements)
	for index := range requirements {
		if requirements[index].hasBranch {
			if requirements[index].composition == "anyOf" {
				requirements[index].hasBranch = false
			} else {
				requirements[index].truth = true
			}
		}

		for _, failure := range fault.expected {
			if !instanceTemplateMatches(
				failure.occurrence.instanceTemplate,
				requirements[index].occurrence.instanceTemplate,
			) {
				continue
			}

			switch failure.rule {
			case oracleRuleRequired:
				requirements[index].presence = requirementPresent
			case oracleRuleAdditionalProperties:
				requirements[index].presence = requirementAbsent
			case oracleRuleType:
				requirements[index].hasKind = false
			case oracleRuleEnum:
				if rowOccurrenceMatches(requirements[index].occurrence, failure.occurrence) {
					requirements[index].hasKind = false
				}
			}
		}
	}

	return requirements
}

// faultRequirementsMatch requires every fault-applicability precondition on a valid parent.
func faultRequirementsMatch(result evaluation, value *jsonValue, requirements []requirement) bool {
	for _, requirement := range requirements {
		switch {
		case requirement.presence != requirementNoPresence && !requirement.canonical &&
			!presenceRequirementWasSatisfied(value, requirement):
			return false
		case requirement.hasKind && !kindWasObserved(result.observedRecords(), requirement.occurrence, requirement.kind):
			return false
		case requirement.hasBranch && !branchTruthWasObserved(result, requirement):
			return false
		}
	}

	return true
}

// applyFault copies the current parent, charges one fault choice, and applies one fault.
func applyFault(parent *jsonValue, fault faultProgram, s *search) (*jsonValue, error) {
	if faultNeedsCompositionSearch(fault) {
		return applyCompositionFault(parent, fault, s)
	}

	return applyNonCompositionFault(parent, fault, s)
}

// copyJSONValue deep-copies one transient parent or fault witness.
func copyJSONValue(value *jsonValue, copied map[*jsonValue]*jsonValue) (*jsonValue, error) {
	if value == nil {
		return nil, errors.New("JSON value is nil")
	}

	if existing, ok := copied[value]; ok {
		return existing, nil
	}

	clone := &jsonValue{kind: value.kind, boolean: value.boolean, text: value.text}
	copied[value] = clone

	if value.number != nil {
		clone.number = &exactNumber{
			numerator:   new(big.Int).Set(value.number.numerator),
			denominator: new(big.Int).Set(value.number.denominator),
			exponent:    new(big.Int).Set(value.number.exponent),
			scale:       new(big.Int).Set(value.number.scale),
		}
	}

	if value.array != nil {
		clone.array = make([]*jsonValue, len(value.array))
		for index, element := range value.array {
			copiedElement, err := copyJSONValue(element, copied)
			if err != nil {
				return nil, err
			}

			clone.array[index] = copiedElement
		}
	}

	if value.object != nil {
		clone.object = make(map[string]*jsonValue, len(value.object))
		for name, member := range value.object {
			copiedMember, err := copyJSONValue(member, copied)
			if err != nil {
				return nil, err
			}

			clone.object[name] = copiedMember
		}
	}

	return clone, nil
}
