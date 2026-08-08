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
	for _, fault := range plan.faultSchedule {
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
//
//nolint:cyclop // Every error and uncovered outcome remains explicit at the streaming boundary.
func streamBasicFault(
	plan *searchPlan,
	fault faultProgram,
	s *search,
	covered map[string]bool,
	yield func(Case) error,
) error {
	unreachable, err := faultIsSyntacticallyUnreachable(fault, s.model)
	if err != nil {
		return err
	}

	if !closureProgramCanComplete(fault.alternatives) || unreachable {
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

// closureProgramCanComplete reports whether every linked branch has at least
// one recursively complete symbolic alternative.
func closureProgramCanComplete(program *faultClosureProgram) bool {
	if program == nil {
		return true
	}

	for alternative := program.alternatives; alternative != nil; alternative = alternative.next {
		if closureProgramCanComplete(alternative.closure) && closureProgramCanComplete(program.next) {
			return true
		}
	}

	return false
}

// faultIsSyntacticallyUnreachable recognizes finite exhaustive enum domains
// without deleting their declarative obligations from the plan.
//
//nolint:cyclop // Closed fault families have distinct exact syntactic checks.
func faultIsSyntacticallyUnreachable(fault faultProgram, model *schemaModel) (bool, error) {
	if model == nil || model.root == nil {
		return false, nil
	}

	node, _, found := resolveExactFaultTarget(
		model.root,
		model.root.occurrence,
		fault.obligation.occurrence,
	)
	if !found {
		return false, nil
	}

	if !faultClosureCanRespectAnyOfParentType(fault, model) {
		return true, nil
	}

	switch fault.obligation.rule {
	case oracleRuleType:
		if node.enum == nil {
			return false, nil
		}

		for _, member := range node.enum {
			matches, err := valueMatchesNodeKind(member.value, node.kind, node.nullable)
			if err != nil {
				return false, err
			}

			if !matches {
				return false, nil
			}
		}

		return true, nil
	case oracleRuleEnum:
		return booleanEnumIsExhaustive(node), nil
	default:
		return false, nil
	}
}

// faultClosureCanRespectAnyOfParentType checks branch faults against one explicit parent type.
func faultClosureCanRespectAnyOfParentType(fault faultProgram, model *schemaModel) bool {
	for _, failure := range fault.expected {
		if failure.rule != oracleRuleAnyOf {
			continue
		}

		parent, _, found := resolveExactFaultTarget(model.root, model.root.occurrence, failure.occurrence)
		if !found || parent.kind == schemaAny || parent.nullable {
			return true
		}

		kind := schemaNodeJSONKind(parent.kind)

		return failureSetCanUseParentKind(
			fault.expected, fault.obligation.ruleIdentity, failure.occurrence, kind, model,
		) && closureProgramCanUseParentKind(
			fault.alternatives, fault.obligation.ruleIdentity, failure.occurrence, kind, model,
		)
	}

	return true
}

// closureProgramCanUseParentKind finds one symbolic closure path compatible with the parent kind.
func closureProgramCanUseParentKind(
	program *faultClosureProgram,
	directed ruleIdentity,
	parent schemaOccurrence,
	kind jsonKind,
	model *schemaModel,
) bool {
	if program == nil {
		return true
	}

	for alternative := program.alternatives; alternative != nil; alternative = alternative.next {
		if failureSetCanUseParentKind(alternative.expected, directed, parent, kind, model) &&
			closureProgramCanUseParentKind(alternative.closure, directed, parent, kind, model) &&
			closureProgramCanUseParentKind(program.next, directed, parent, kind, model) {
			return true
		}
	}

	return false
}

// failureSetCanUseParentKind rejects type failures that would also break the parent type.
func failureSetCanUseParentKind(
	failures failureSet,
	directed ruleIdentity,
	parent schemaOccurrence,
	kind jsonKind,
	model *schemaModel,
) bool {
	for _, failure := range failures {
		directedAtParent := failure == directed &&
			rowOccurrenceMatches(failure.occurrence, parent)
		if failure.rule != oracleRuleType || directedAtParent ||
			failure.occurrence.instanceTemplate != parent.instanceTemplate {
			continue
		}

		node, _, found := resolveExactFaultTarget(model.root, model.root.occurrence, failure.occurrence)
		if !found {
			continue
		}

		if node.kind == schemaInteger && kind == jsonNumber {
			continue
		}

		if nodeAcceptsKindForTarget(node, kind) {
			return false
		}
	}

	return true
}

// booleanEnumIsExhaustive recognizes the finite valid domain of a boolean schema.
//
//nolint:cyclop // Boolean and nullable coverage are clearer as explicit cases.
func booleanEnumIsExhaustive(node *schemaNode) bool {
	if node == nil || node.kind != schemaBoolean || node.enum == nil {
		return false
	}

	seenFalse := false
	seenTrue := false
	seenNull := false

	for _, member := range node.enum {
		switch member.value.kind {
		case jsonBoolean:
			if member.value.boolean {
				seenTrue = true
			} else {
				seenFalse = true
			}
		case jsonNull:
			seenNull = true
		}
	}

	return seenFalse && seenTrue && (!node.nullable || seenNull)
}

// regenerateParent replays row search for one fresh, complete oracle-valid parent.
func regenerateParent(plan *searchPlan, fault faultProgram, s *search) (*jsonValue, bool, error) {
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
//
//nolint:cyclop // Presence, type, and enum faults translate distinct pin dimensions.
func parentReplayPins(fault faultProgram) []requirement {
	pins := copyPlanPins(fault.requirements)
	for index := range pins {
		if pins[index].hasBranch {
			if pins[index].composition == "anyOf" {
				pins[index].hasBranch = false
			} else {
				pins[index].truth = true
			}
		}

		for _, failure := range fault.expected {
			if !instanceTemplateMatches(
				failure.occurrence.instanceTemplate,
				pins[index].occurrence.instanceTemplate,
			) {
				continue
			}

			switch failure.rule {
			case oracleRuleRequired:
				pins[index].presence = requirementPresent
			case oracleRuleAdditionalProperties:
				pins[index].presence = requirementAbsent
			case oracleRuleType:
				pins[index].hasKind = false
			case oracleRuleEnum:
				if rowOccurrenceMatches(pins[index].occurrence, failure.occurrence) {
					pins[index].hasKind = false
				}
			}
		}
	}

	return pins
}

// faultPinsMatch requires every fault-applicability precondition on a valid parent.
func faultPinsMatch(result evaluation, value *jsonValue, pins []requirement) bool {
	for _, pin := range pins {
		switch {
		case pin.presence != requirementNoPresence && !pin.canonical && !presencePinWasSatisfied(value, pin):
			return false
		case pin.hasKind && !kindWasObserved(result.observedRecords(), pin.occurrence, pin.kind):
			return false
		case pin.hasBranch && !branchTruthWasObserved(result, pin):
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
