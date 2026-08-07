package schematest

import (
	"errors"
	"fmt"
	"math/big"
)

// errFaultNotFound leaves an unrealizable planned fault uncovered.
var errFaultNotFound = errors.New("schematest: planned fault has no isolated derivative")

// streamBasicFaults visits supported faults in planner order.
func streamBasicFaults(
	plan *searchPlan,
	s *search,
	covered map[string]bool,
	yield func(Case) error,
) (StopReason, error) {
	for _, fault := range plan.faultTargets {
		if err := streamBasicFault(plan, fault, s, covered, yield); err != nil {
			if errors.Is(err, errMaxSteps) {
				return MaxStepsReached, nil
			}

			return "", err
		}
	}

	return SpaceExhausted, nil
}

// streamBasicFault replays, applies, verifies, and emits one fault derivative.
func streamBasicFault(
	plan *searchPlan,
	fault faultTarget,
	s *search,
	covered map[string]bool,
	yield func(Case) error,
) error {
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

	matches, err := exactFailureClosure(result.failures, fault.closure)
	if err != nil {
		return fmt.Errorf("compare fault closure: %w", err)
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
func regenerateParent(plan *searchPlan, fault faultTarget, s *search) (*jsonValue, bool, error) {
	if plan == nil {
		return nil, false, errors.New("schematest: nil search plan")
	}

	if s == nil || s.model == nil || s.model.root == nil {
		return nil, false, errors.New("schematest: parent replay has no model")
	}

	parentPins := parentReplayPins(fault)

	var parent *jsonValue

	visit := func(value *jsonValue) (bool, error) {
		result := evaluate(s.model, value)
		if result.err != nil {
			return false, fmt.Errorf("evaluate regenerated parent: %w", result.err)
		}

		if !result.valid || !faultPinsMatch(result, value, parentPins) {
			return false, nil
		}

		parent = value

		return true, nil
	}

	complete, err := s.walkNode(
		s.model.root,
		s.model.root.occurrence,
		parentPins,
		rowSearchContext{},
		visit,
	)
	if err != nil {
		return nil, false, err
	}

	return parent, complete, nil
}

// parentReplayPins turns mutation-result presence into valid-parent presence.
func parentReplayPins(fault faultTarget) []applicabilityPin {
	pins := copyPlanPins(fault.pins)
	for index := range pins {
		if pins[index].hasBranch {
			if pins[index].composition == "anyOf" {
				pins[index].hasBranch = false
			} else {
				pins[index].truth = true
			}
		}

		for _, failure := range fault.closure {
			if !instanceTemplateMatches(
				failure.occurrence.instanceTemplate,
				pins[index].occurrence.instanceTemplate,
			) {
				continue
			}

			switch failure.rule {
			case oracleRuleRequired:
				pins[index].presence = planPinPresent
			case oracleRuleAdditionalProperties:
				pins[index].presence = planPinAbsent
			case oracleRuleType:
				pins[index].hasKind = false
			}
		}
	}

	return pins
}

// faultPinsMatch requires every fault-applicability precondition on a valid parent.
func faultPinsMatch(result evaluation, value *jsonValue, pins []applicabilityPin) bool {
	for _, pin := range pins {
		switch {
		case pin.presence != planPinNoPresence && !pin.canonical && !presencePinWasSatisfied(value, pin):
			return false
		case pin.hasKind && !kindWasObserved(result.observed, pin.occurrence, pin.kind):
			return false
		case pin.hasBranch && !branchTruthWasObserved(result, pin):
			return false
		}
	}

	return true
}

// applyFault copies the current parent, charges one fault choice, and applies one fault.
func applyFault(parent *jsonValue, fault faultTarget, s *search) (*jsonValue, error) {
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
