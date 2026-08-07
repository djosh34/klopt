//nolint:godoclint // Private fault helpers stay behind Build.
package schematest

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// applyNonCompositionFault builds one isolated non-composition derivative.
func applyNonCompositionFault(parent *jsonValue, fault faultTarget, s *search) (*jsonValue, error) {
	if parent == nil {
		return nil, errors.New("schematest: nil fault parent")
	}

	if s == nil || s.model == nil || s.model.root == nil {
		return nil, errors.New("schematest: fault application has no model")
	}

	if fault.obligation.rule == oracleRuleAllOf || fault.obligation.rule == oracleRuleAnyOf {
		return nil, fmt.Errorf("schematest: composition fault requires aggregate handling: %s", fault.obligation.String())
	}

	candidate, found, err := findNonCompositionDerivative(parent, fault, s)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, fmt.Errorf("%w: %s", errFaultNotFound, fault.obligation.String())
	}

	return candidate, nil
}

// findNonCompositionDerivative selects a deterministic witness without retaining candidates.
//
//nolint:cyclop // Closed fault-family dispatch is intentionally explicit.
func findNonCompositionDerivative(parent *jsonValue, fault faultTarget, s *search) (*jsonValue, bool, error) {
	if fault.obligation.rule == oracleRuleRequired {
		return findRequiredDerivative(parent, fault, s)
	}

	if fault.obligation.rule == oracleRuleAdditionalProperties {
		return findAdditionalPropertyDerivative(parent, fault, s)
	}

	node, _, found := resolveExactFaultTarget(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence,
	)
	if !found {
		return nil, false, fmt.Errorf("schematest: fault target was not found: %s", fault.obligation.String())
	}

	switch fault.obligation.rule {
	case oracleRuleType:
		return findTypeDerivative(parent, fault, node, s.model, s)
	case oracleRuleEnum:
		return findEnumDerivative(parent, fault, node, s.model, s)
	case oracleRuleMinimum, oracleRuleExclusiveMinimum, oracleRuleMaximum,
		oracleRuleExclusiveMaximum, oracleRuleMultipleOf:
		return findNumberDerivative(parent, fault, s)
	case oracleRuleFormat:
		if formatHasNumericSemantics(node.format) {
			return findNumberDerivative(parent, fault, s)
		}

		return findStringDerivative(parent, fault, s)
	case oracleRuleMinLength, oracleRuleMaxLength, oracleRulePattern:
		return findStringDerivative(parent, fault, s)
	case oracleRuleMinItems, oracleRuleMaxItems:
		return findArrayCountDerivative(parent, fault, node, s)
	case oracleRuleMinProperties, oracleRuleMaxProperties:
		return findObjectCountDerivative(parent, fault, node, s)
	default:
		return nil, false, fmt.Errorf("schematest: unsupported fault rule %q", fault.obligation.rule)
	}
}

func formatHasNumericSemantics(format schemaFormat) bool {
	switch format {
	case schemaFormatInt32, schemaFormatInt64, schemaFormatFloat, schemaFormatDouble:
		return true
	default:
		return false
	}
}

//nolint:cyclop // Seeded witnesses and active-conjunction fallback share one search.
func findTypeDerivative(
	parent *jsonValue,
	fault faultTarget,
	node *schemaNode,
	model *schemaModel,
	s *search,
) (*jsonValue, bool, error) {
	withoutLocalType := schemaNodeWithoutLocalRule(node, oracleRuleType)
	for _, kind := range canonicalJSONKinds() {
		if !typeFaultCanUseKind(node, kind) {
			continue
		}

		seeded, seedErr := canonicalAnyOfWitnesses(withoutLocalType, kind)
		if seedErr != nil {
			return nil, false, seedErr
		}

		derivative, found, seedErr := firstReplacementDerivative(parent, fault, seeded, model, s)
		if seedErr != nil || found {
			return derivative, found, seedErr
		}

		container, occurrence, found := resolveFaultValueContainer(
			model.root, model.root.occurrence, fault.obligation.occurrence,
		)
		if !found {
			continue
		}

		withoutType := cloneWithoutFaultRule(container, occurrence, fault.obligation.occurrence, oracleRuleType)
		derivative = nil

		complete, err := s.walkNode(
			withoutType, occurrence, fault.pins, rowSearchContext{}, func(candidate *jsonValue) (bool, error) {
				selected, matched, selectErr := firstReplacementDerivative(
					parent, fault, []*jsonValue{candidate}, model, s,
				)
				if selectErr != nil || !matched {
					return false, selectErr
				}

				derivative = selected

				return true, nil
			},
		)
		if err != nil || complete {
			return derivative, complete, err
		}
	}

	return nil, false, nil
}

func findEnumDerivative(
	parent *jsonValue,
	fault faultTarget,
	node *schemaNode,
	model *schemaModel,
	s *search,
) (*jsonValue, bool, error) {
	for _, kind := range enumFaultKinds(node) {
		seeded, seedErr := canonicalEnumFaultWitnesses(node, kind)
		if seedErr != nil {
			return nil, false, seedErr
		}

		derivative, found, seedErr := firstReplacementDerivative(parent, fault, seeded, model, s)
		if seedErr != nil || found {
			return derivative, found, seedErr
		}

		container, occurrence, found := resolveFaultContainer(
			model.root, model.root.occurrence, fault.obligation.occurrence, kind,
		)
		if !found {
			continue
		}

		withoutEnum := cloneWithoutFaultRule(container, occurrence, fault.obligation.occurrence, oracleRuleEnum)
		derivative = nil

		complete, err := s.walkNode(
			withoutEnum, occurrence, fault.pins, rowSearchContext{}, func(candidate *jsonValue) (bool, error) {
				selected, matched, selectErr := firstReplacementDerivative(
					parent, fault, []*jsonValue{candidate}, model, s,
				)
				if selectErr != nil || !matched {
					return false, selectErr
				}

				derivative = selected

				return true, nil
			},
		)
		if err != nil || complete {
			return derivative, complete, err
		}
	}

	return nil, false, nil
}

func cloneWithoutFaultRule(
	node *schemaNode,
	occurrence schemaOccurrence,
	target schemaOccurrence,
	rule string,
) *schemaNode {
	if ruleOccurrenceMatches(occurrence, target) {
		return schemaNodeWithoutLocalRule(node, rule)
	}

	shape := *node.schemaShape

	shape.allOf = append([]*schemaNode(nil), node.allOf...)
	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child, occurrence.usePointer+"/allOf/"+itoa(index), occurrence.instanceTemplate,
		)
		if target.usePointer == childOccurrence.usePointer ||
			strings.HasPrefix(target.usePointer, childOccurrence.usePointer+"/") {
			shape.allOf[index] = cloneWithoutFaultRule(child, childOccurrence, target, rule)
		}
	}

	shape.anyOf = append([]*schemaNode(nil), node.anyOf...)
	for index, child := range node.anyOf {
		childOccurrence := rebasePlanOccurrence(
			child, occurrence.usePointer+"/anyOf/"+itoa(index), occurrence.instanceTemplate,
		)
		if target.usePointer == childOccurrence.usePointer ||
			strings.HasPrefix(target.usePointer, childOccurrence.usePointer+"/") {
			shape.anyOf[index] = cloneWithoutFaultRule(child, childOccurrence, target, rule)
		}
	}

	return &schemaNode{schemaShape: &shape, occurrence: node.occurrence}
}

func findNumberDerivative(parent *jsonValue, fault faultTarget, s *search) (*jsonValue, bool, error) {
	container, occurrence, found := resolveFaultContainer(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence, jsonNumber,
	)
	if !found {
		return nil, false, nil
	}

	var derivative *jsonValue

	complete, err := s.walkActiveNumberRules(
		container, occurrence, fault.pins, nil, func(candidate *jsonValue) (bool, error) {
			selected, matched, selectErr := firstReplacementDerivative(
				parent, fault, []*jsonValue{candidate}, s.model, s,
			)
			if selectErr != nil || !matched {
				return false, selectErr
			}

			derivative = selected

			return true, nil
		},
	)
	if err != nil {
		return nil, false, err
	}

	return derivative, complete, nil
}

func findStringDerivative(parent *jsonValue, fault faultTarget, s *search) (*jsonValue, bool, error) {
	candidate, found, err := findStringFaultRow(fault, s)
	if err != nil || !found {
		return nil, false, err
	}

	return firstReplacementDerivative(parent, fault, []*jsonValue{candidate}, s.model, s)
}

func firstReplacementDerivative(
	parent *jsonValue,
	fault faultTarget,
	candidates []*jsonValue,
	model *schemaModel,
	s *search,
) (*jsonValue, bool, error) {
	paths := matchingValuePaths(parent, fault.obligation.occurrence.instanceTemplate)
	for _, path := range paths {
		for _, candidate := range candidates {
			if err := s.assign(); err != nil {
				return nil, false, err
			}

			derivative, err := copyJSONValue(parent, make(map[*jsonValue]*jsonValue))
			if err != nil {
				return nil, false, err
			}

			copyCandidate, err := copyJSONValue(candidate, make(map[*jsonValue]*jsonValue))
			if err != nil {
				return nil, false, err
			}

			if !replaceValueAtPath(derivative, path, copyCandidate) {
				continue
			}

			matched, err := derivativeHasClosure(model, derivative, fault.closure)
			if err != nil {
				return nil, false, err
			}

			if matched {
				return derivative, true, nil
			}
		}
	}

	return nil, false, nil
}

//nolint:cyclop // Exact count conversion, resize, and item repair are one mutation.
func findArrayCountDerivative(
	parent *jsonValue,
	fault faultTarget,
	node *schemaNode,
	s *search,
) (*jsonValue, bool, error) {
	paths := matchingValuePaths(parent, fault.obligation.occurrence.instanceTemplate)
	for _, path := range paths {
		value := valueAtPath(parent, path)
		if value == nil || value.kind != jsonArray {
			continue
		}

		bound := node.minItems
		if fault.obligation.rule == oracleRuleMaxItems {
			bound = node.maxItems
		}

		count, fits, err := exactCountUint64(bound)
		if err != nil {
			return nil, false, err
		}

		if !fits || count > uint64(maxInt()) {
			return nil, false, exhaustFaultStructuralBudget(s)
		}

		desired := int(count)
		if fault.obligation.rule == oracleRuleMinItems {
			if desired == 0 {
				continue
			}

			desired--
		} else {
			if desired == maxInt() {
				return nil, false, exhaustFaultStructuralBudget(s)
			}

			desired++
		}

		if len(value.array) >= desired {
			return buildArrayCountDerivative(parent, path, desired, nil, fault, s)
		}

		container, containerOccurrence, found := resolveFaultContainer(
			s.model.root, s.model.root.occurrence, fault.obligation.occurrence, jsonArray,
		)
		if !found {
			continue
		}

		var derivative *jsonValue

		complete, walkErr := walkActiveFaultChildValues(
			container, containerOccurrence, fault.pins, rowChildItems, "", s,
			func(witness *jsonValue) (bool, error) {
				candidate, matched, candidateErr := buildArrayCountDerivative(
					parent, path, desired, witness, fault, s,
				)
				if candidateErr != nil || !matched {
					return false, candidateErr
				}

				derivative = candidate

				return true, nil
			},
		)
		if walkErr != nil || complete {
			return derivative, complete, walkErr
		}
	}

	return nil, false, nil
}

func buildArrayCountDerivative(
	parent *jsonValue,
	path []string,
	desired int,
	witness *jsonValue,
	fault faultTarget,
	s *search,
) (*jsonValue, bool, error) {
	if assignErr := s.assign(); assignErr != nil {
		return nil, false, assignErr
	}

	candidate, copyErr := copyJSONValue(parent, make(map[*jsonValue]*jsonValue))
	if copyErr != nil {
		return nil, false, copyErr
	}

	array := valueAtPath(candidate, path)
	if len(array.array) > desired {
		if assignErr := s.assign(); assignErr != nil {
			return nil, false, assignErr
		}

		array.array = array.array[:desired]
	}

	for len(array.array) < desired {
		if assignErr := s.assign(); assignErr != nil {
			return nil, false, assignErr
		}

		item, itemErr := copyJSONValue(witness, make(map[*jsonValue]*jsonValue))
		if itemErr != nil {
			return nil, false, itemErr
		}

		array.array = append(array.array, item)
	}

	matched, matchErr := derivativeHasClosure(s.model, candidate, fault.closure)

	return candidate, matched, matchErr
}

//nolint:cyclop // Structural choice and complete child walking share one seam.
func walkActiveFaultChildValues(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	kind rowChildKind,
	name string,
	s *search,
	visit rowVisit,
) (bool, error) {
	if kind == rowChildProperty {
		members, err := rowObjectMembers(node, occurrence, pins)
		if err != nil {
			return false, err
		}

		for _, member := range members {
			if member.name == name {
				return s.walkRowMemberValues(member, pins, rowSearchContext{}, visit)
			}
		}

		return s.walkGenericValue(pins, visit)
	}

	choices, err := rowChildSchemaChoices(node, occurrence, pins, kind, name)
	if err != nil {
		return false, err
	}

	for _, choice := range choices {
		if choice.node == nil {
			complete, walkErr := s.walkGenericValue(pins, visit)
			if walkErr != nil || complete {
				return complete, walkErr
			}

			continue
		}

		complete, walkErr := s.walkNode(
			choice.node, choice.occurrence, pins, rowSearchContext{}, func(value *jsonValue) (bool, error) {
				usable, usableErr := s.rowChildValueUsable(choice.node, choice.occurrence, pins, value)
				if usableErr != nil || !usable {
					return false, usableErr
				}

				return visit(value)
			},
		)
		if walkErr != nil || complete {
			return complete, walkErr
		}
	}

	return false, nil
}

//nolint:cyclop,gocognit // Exact count conversion and active member search are one mutation.
func findObjectCountDerivative(
	parent *jsonValue,
	fault faultTarget,
	node *schemaNode,
	s *search,
) (*jsonValue, bool, error) {
	paths := matchingValuePaths(parent, fault.obligation.occurrence.instanceTemplate)
	if fault.obligation.rule == oracleRuleMinProperties {
		return findObjectShrinkDerivative(parent, fault, node, paths, s)
	}

	count, fits, err := exactCountUint64(node.maxProperties)
	if err != nil {
		return nil, false, err
	}

	if !fits || count >= uint64(maxInt()) {
		return nil, false, exhaustFaultStructuralBudget(s)
	}

	desired := int(count) + 1

	container, containerOccurrence, found := resolveFaultContainer(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence, jsonObject,
	)
	if !found {
		return nil, false, nil
	}

	for _, path := range paths {
		object := valueAtPath(parent, path)
		if object == nil || object.kind != jsonObject {
			continue
		}

		needed := desired - len(object.object)
		if needed <= 0 {
			continue
		}

		if uint64(needed) > s.maxSteps-s.steps {
			return nil, false, exhaustFaultStructuralBudget(s)
		}

		growthPins := append([]applicabilityPin(nil), fault.pins...)

		members, memberErr := rowObjectMembers(container, containerOccurrence, growthPins)
		if memberErr != nil {
			return nil, false, memberErr
		}

		available := 0

		for _, member := range members {
			if _, exists := object.object[member.name]; !exists {
				available++
			}
		}

		base := additionalPropertyWitnessName(container)
		for index := 0; available < needed; index++ {
			name := base
			if index > 0 {
				name = fmt.Sprintf("%s_%d", base, index)
			}

			if _, exists := object.object[name]; exists {
				continue
			}

			growthPins = append(growthPins, faultMemberPresencePin(containerOccurrence, name))
			available++
		}

		if len(growthPins) != len(fault.pins) {
			members, memberErr = rowObjectMembers(container, containerOccurrence, growthPins)
			if memberErr != nil {
				return nil, false, memberErr
			}
		}

		derivative, found, searchErr := findObjectGrowthDerivative(
			parent, path, desired, members, fault, growthPins, s,
		)
		if searchErr != nil || found {
			return derivative, found, searchErr
		}
	}

	return nil, false, nil
}

//nolint:cyclop // Subset search, charging, and closure checking form one bounded repair.
func findObjectShrinkDerivative(
	parent *jsonValue,
	fault faultTarget,
	node *schemaNode,
	paths [][]string,
	s *search,
) (*jsonValue, bool, error) {
	count, fits, countErr := exactCountUint64(node.minProperties)
	if countErr != nil {
		return nil, false, countErr
	}

	if !fits || count > uint64(maxInt()) {
		return nil, false, exhaustFaultStructuralBudget(s)
	}

	if count == 0 {
		return nil, false, nil
	}

	desired := int(count) - 1

	for _, path := range paths {
		object := valueAtPath(parent, path)
		if object == nil || object.kind != jsonObject || len(object.object) < desired {
			continue
		}

		names := sortedObjectNames(object.object)
		removeCount := len(names) - desired

		subsetFound, subsetErr := visitNameSubsets(names, removeCount, func(remove []string) (bool, error) {
			if assignErr := s.assign(); assignErr != nil {
				return false, assignErr
			}

			candidate, copyErr := copyJSONValue(parent, make(map[*jsonValue]*jsonValue))
			if copyErr != nil {
				return false, copyErr
			}

			candidateObject := valueAtPath(candidate, path)

			for _, name := range remove {
				if assignErr := s.assign(); assignErr != nil {
					return false, assignErr
				}

				delete(candidateObject.object, name)
			}

			matched, matchErr := derivativeHasClosure(s.model, candidate, fault.closure)
			if matchErr != nil || !matched {
				return false, matchErr
			}

			object = candidate

			return true, nil
		})
		if subsetErr != nil {
			return nil, false, subsetErr
		}

		if subsetFound {
			return object, true, nil
		}
	}

	return nil, false, nil
}

func visitNameSubsets(names []string, size int, visit func([]string) (bool, error)) (bool, error) {
	selected := make([]string, 0, size)

	var walk func(int) (bool, error)

	walk = func(start int) (bool, error) {
		if len(selected) == size {
			return visit(selected)
		}

		for index := start; index <= len(names)-(size-len(selected)); index++ {
			selected = append(selected, names[index])
			found, err := walk(index + 1)
			selected = selected[:len(selected)-1]

			if err != nil || found {
				return found, err
			}
		}

		return false, nil
	}

	return walk(0)
}

func faultMemberPresencePin(occurrence schemaOccurrence, name string) applicabilityPin {
	return applicabilityPin{
		occurrence: schemaOccurrence{
			usePointer:       occurrence.usePointer + "/additionalProperties",
			targetPointer:    occurrence.targetPointer,
			instanceTemplate: appendInstanceToken(occurrence.instanceTemplate, name),
		},
		presence: planPinPresent,
	}
}

//nolint:cyclop // Member/value backtracking and closure checking form one DFS.
func findObjectGrowthDerivative(
	parent *jsonValue,
	path []string,
	desired int,
	members []rowMember,
	fault faultTarget,
	pins []applicabilityPin,
	s *search,
) (*jsonValue, bool, error) {
	var derivative *jsonValue

	var walk func(*jsonValue, int) (bool, error)

	walk = func(candidate *jsonValue, index int) (bool, error) {
		object := valueAtPath(candidate, path)
		if object == nil || object.kind != jsonObject {
			return false, nil
		}

		if len(object.object) == desired {
			matched, err := derivativeHasClosure(s.model, candidate, fault.closure)
			if err != nil || !matched {
				return false, err
			}

			derivative = candidate

			return true, nil
		}

		if index == len(members) || len(object.object) > desired {
			return false, nil
		}

		member := members[index]
		if _, exists := object.object[member.name]; !exists {
			complete, err := s.walkRowMemberValues(
				member, pins, rowSearchContext{}, func(value *jsonValue) (bool, error) {
					if assignErr := s.assign(); assignErr != nil {
						return false, assignErr
					}

					next, copyErr := copyJSONValue(candidate, make(map[*jsonValue]*jsonValue))
					if copyErr != nil {
						return false, copyErr
					}

					copiedValue, copyErr := copyJSONValue(value, make(map[*jsonValue]*jsonValue))
					if copyErr != nil {
						return false, copyErr
					}

					valueAtPath(next, path).object[member.name] = copiedValue

					return walk(next, index+1)
				},
			)
			if err != nil || complete {
				return complete, err
			}
		}

		return walk(candidate, index+1)
	}

	found, err := walk(parent, 0)

	return derivative, found, err
}

func findRequiredDerivative(parent *jsonValue, fault faultTarget, s *search) (*jsonValue, bool, error) {
	tokens, ok := rowPointerTokens(fault.obligation.occurrence.instanceTemplate)
	if !ok || len(tokens) == 0 {
		return nil, false, errors.New("schematest: required fault has no member path")
	}

	name := tokens[len(tokens)-1]

	parentPointer := pointerFromTokens(tokens[:len(tokens)-1])
	for _, path := range matchingValuePaths(parent, parentPointer) {
		if err := s.assign(); err != nil {
			return nil, false, err
		}

		candidate, err := copyJSONValue(parent, make(map[*jsonValue]*jsonValue))
		if err != nil {
			return nil, false, err
		}

		object := valueAtPath(candidate, path)
		if object == nil || object.kind != jsonObject {
			continue
		}

		delete(object.object, name)

		matched, err := derivativeHasClosure(s.model, candidate, fault.closure)
		if err != nil || matched {
			return candidate, matched, err
		}
	}

	return nil, false, nil
}

//nolint:cyclop // Path and active-witness retries form one mutation search.
func findAdditionalPropertyDerivative(
	parent *jsonValue,
	fault faultTarget,
	s *search,
) (*jsonValue, bool, error) {
	tokens, ok := rowPointerTokens(fault.obligation.occurrence.instanceTemplate)
	if !ok || len(tokens) == 0 {
		return nil, false, errors.New("schematest: additional-property fault has no member path")
	}

	parentPointer := pointerFromTokens(tokens[:len(tokens)-1])

	containerTarget := fault.obligation.occurrence
	containerTarget.instanceTemplate = parentPointer

	container, containerOccurrence, found := resolveFaultContainer(
		s.model.root, s.model.root.occurrence, containerTarget, jsonObject,
	)
	if !found {
		return nil, false, nil
	}

	name := additionalPropertyWitnessName(container)

	pins := append([]applicabilityPin(nil), fault.pins...)
	pins = append(pins, faultMemberPresencePin(containerOccurrence, name))

	for _, path := range matchingValuePaths(parent, parentPointer) {
		var derivative *jsonValue

		complete, err := walkActiveFaultChildValues(
			container, containerOccurrence, pins, rowChildProperty, name, s,
			func(witness *jsonValue) (bool, error) {
				if assignErr := s.assign(); assignErr != nil {
					return false, assignErr
				}

				candidate, copyErr := copyJSONValue(parent, make(map[*jsonValue]*jsonValue))
				if copyErr != nil {
					return false, copyErr
				}

				value, copyErr := copyJSONValue(witness, make(map[*jsonValue]*jsonValue))
				if copyErr != nil {
					return false, copyErr
				}

				object := valueAtPath(candidate, path)
				if object == nil || object.kind != jsonObject {
					return false, nil
				}

				object.object[name] = value

				matched, matchErr := derivativeHasClosure(s.model, candidate, fault.closure)
				if matchErr != nil || !matched {
					return false, matchErr
				}

				derivative = candidate

				return true, nil
			},
		)
		if err != nil || complete {
			return derivative, complete, err
		}
	}

	return nil, false, nil
}

func derivativeHasClosure(model *schemaModel, derivative *jsonValue, closure []failureIdentity) (bool, error) {
	result := evaluate(model, derivative)
	if result.err != nil {
		return false, fmt.Errorf("evaluate fault derivative: %w", result.err)
	}

	if result.valid {
		return false, nil
	}

	return exactFailureClosure(result.failures, closure)
}

// resolveExactFaultTarget follows canonical schema children to one authored rule occurrence.
func resolveExactFaultTarget(
	node *schemaNode,
	occurrence schemaOccurrence,
	target schemaOccurrence,
) (*schemaNode, schemaOccurrence, bool) {
	if ruleOccurrenceMatches(occurrence, target) {
		return node, occurrence, true
	}

	for _, child := range faultSchemaChildren(node, occurrence) {
		if target.usePointer != child.occurrence.usePointer &&
			!strings.HasPrefix(target.usePointer, child.occurrence.usePointer+"/") {
			continue
		}

		if foundNode, foundOccurrence, found := resolveExactFaultTarget(child.node, child.occurrence, target); found {
			return foundNode, foundOccurrence, true
		}
	}

	return nil, schemaOccurrence{}, false
}

func resolveFaultValueContainer(
	node *schemaNode,
	occurrence schemaOccurrence,
	target schemaOccurrence,
) (*schemaNode, schemaOccurrence, bool) {
	if instanceTemplateMatches(occurrence.instanceTemplate, target.instanceTemplate) &&
		(target.usePointer == occurrence.usePointer || strings.HasPrefix(target.usePointer, occurrence.usePointer+"/")) {
		return node, occurrence, true
	}

	for _, child := range faultSchemaChildren(node, occurrence) {
		if foundNode, foundOccurrence, found := resolveFaultValueContainer(
			child.node, child.occurrence, target,
		); found {
			return foundNode, foundOccurrence, true
		}
	}

	return nil, schemaOccurrence{}, false
}

func resolveFaultContainer(
	node *schemaNode,
	occurrence schemaOccurrence,
	target schemaOccurrence,
	kind jsonKind,
) (*schemaNode, schemaOccurrence, bool) {
	if instanceTemplateMatches(occurrence.instanceTemplate, target.instanceTemplate) &&
		nodeAcceptsKindForTarget(node, kind) &&
		(target.usePointer == occurrence.usePointer || strings.HasPrefix(target.usePointer, occurrence.usePointer+"/")) {
		return node, occurrence, true
	}

	for _, child := range faultSchemaChildren(node, occurrence) {
		if foundNode, foundOccurrence, found := resolveFaultContainer(child.node, child.occurrence, target, kind); found {
			return foundNode, foundOccurrence, true
		}
	}

	return nil, schemaOccurrence{}, false
}

type faultSchemaChild struct {
	node       *schemaNode
	occurrence schemaOccurrence
}

func faultSchemaChildren(node *schemaNode, occurrence schemaOccurrence) []faultSchemaChild {
	children := make([]faultSchemaChild, 0)
	if node.items != nil {
		children = append(children, faultSchemaChild{node.items, rebasePlanOccurrence(
			node.items, occurrence.usePointer+"/items", appendInstanceToken(occurrence.instanceTemplate, "*"),
		)})
	}

	for _, name := range sortedSchemaPropertyNames(node.properties) {
		property := node.properties[name]
		children = append(children, faultSchemaChild{property, rebasePlanOccurrence(
			property, occurrence.usePointer+"/properties/"+escapePointerToken(name),
			appendInstanceToken(occurrence.instanceTemplate, name),
		)})
	}

	if node.additionalProperties != nil {
		children = append(children, faultSchemaChild{node.additionalProperties, rebasePlanOccurrence(
			node.additionalProperties, occurrence.usePointer+"/additionalProperties",
			appendInstanceToken(occurrence.instanceTemplate, "*"),
		)})
	}

	for index, child := range node.allOf {
		children = append(children, faultSchemaChild{child, rebasePlanOccurrence(
			child, occurrence.usePointer+"/allOf/"+itoa(index), occurrence.instanceTemplate,
		)})
	}

	for index, child := range node.anyOf {
		children = append(children, faultSchemaChild{child, rebasePlanOccurrence(
			child, occurrence.usePointer+"/anyOf/"+itoa(index), occurrence.instanceTemplate,
		)})
	}

	return children
}

func matchingValuePaths(root *jsonValue, template string) [][]string {
	tokens, ok := rowPointerTokens(template)
	if !ok {
		return nil
	}

	var result [][]string
	collectMatchingValuePaths(root, tokens, nil, &result)

	return result
}

//nolint:cyclop // Pointer completion and two container kinds form one traversal.
func collectMatchingValuePaths(value *jsonValue, tokens, path []string, result *[][]string) {
	if len(tokens) == 0 {
		*result = append(*result, append([]string(nil), path...))

		return
	}

	if value == nil {
		return
	}

	token := tokens[0]

	switch value.kind {
	case jsonArray:
		if token == "*" {
			for index, child := range value.array {
				collectMatchingValuePaths(child, tokens[1:], append(path, strconv.Itoa(index)), result)
			}

			return
		}

		index, err := strconv.Atoi(token)
		if err == nil && index >= 0 && index < len(value.array) {
			collectMatchingValuePaths(value.array[index], tokens[1:], append(path, token), result)
		}
	case jsonObject:
		if token == "*" {
			names := sortedObjectNames(value.object)
			for _, name := range names {
				collectMatchingValuePaths(value.object[name], tokens[1:], append(path, name), result)
			}

			return
		}

		if child, exists := value.object[token]; exists {
			collectMatchingValuePaths(child, tokens[1:], append(path, token), result)
		}
	}
}

func valueAtPath(root *jsonValue, path []string) *jsonValue {
	value := root
	for _, token := range path {
		if value == nil {
			return nil
		}

		switch value.kind {
		case jsonArray:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(value.array) {
				return nil
			}

			value = value.array[index]
		case jsonObject:
			value = value.object[token]
		default:
			return nil
		}
	}

	return value
}

func replaceValueAtPath(root *jsonValue, path []string, replacement *jsonValue) bool {
	if len(path) == 0 {
		*root = *replacement

		return true
	}

	parent := valueAtPath(root, path[:len(path)-1])
	if parent == nil {
		return false
	}

	last := path[len(path)-1]

	switch parent.kind {
	case jsonArray:
		index, err := strconv.Atoi(last)
		if err != nil || index < 0 || index >= len(parent.array) {
			return false
		}

		parent.array[index] = replacement
	case jsonObject:
		if _, exists := parent.object[last]; !exists {
			return false
		}

		parent.object[last] = replacement
	default:
		return false
	}

	return true
}

func pointerFromTokens(tokens []string) string {
	if len(tokens) == 0 {
		return "#"
	}

	encoded := make([]string, len(tokens))
	for index, token := range tokens {
		encoded[index] = escapePointerToken(token)
	}

	return "#/" + strings.Join(encoded, "/")
}

func exhaustFaultStructuralBudget(s *search) error {
	if s == nil {
		return errors.New("schematest: nil search")
	}

	var frontier uint64

	for {
		if err := s.assign(); err != nil {
			return err
		}

		frontier++
	}
}

func maxInt() int {
	return int(^uint(0) >> 1)
}
