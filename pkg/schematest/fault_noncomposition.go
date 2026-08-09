//nolint:godoclint // Private fault helpers stay behind Build.
package schematest

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// applyNonCompositionFault builds one isolated non-composition derivative.
func applyNonCompositionFault(parent *jsonValue, fault faultProgram, s *search) (*jsonValue, error) {
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
func findNonCompositionDerivative(parent *jsonValue, fault faultProgram, s *search) (*jsonValue, bool, error) {
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
		return findTypeDerivative(parent, fault, s)
	case oracleRuleEnum:
		return findEnumDerivative(parent, fault, s)
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

func findTypeDerivative(
	parent *jsonValue,
	fault faultProgram,
	s *search,
) (*jsonValue, bool, error) {
	return findKindDirectedDerivative(parent, fault, oracleRuleType, s)
}

func findEnumDerivative(
	parent *jsonValue,
	fault faultProgram,
	s *search,
) (*jsonValue, bool, error) {
	return findKindDirectedDerivative(parent, fault, oracleRuleEnum, s)
}

func findKindDirectedDerivative(
	parent *jsonValue,
	fault faultProgram,
	rule string,
	s *search,
) (*jsonValue, bool, error) {
	root := cloneWithoutFaultRule(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence, rule,
	)

	_, containerOccurrence, found := resolveFaultValueContainer(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence,
	)
	if !found {
		return nil, false, fmt.Errorf(
			"schematest: %s fault target was not found: %s", rule, fault.obligation.String(),
		)
	}

	for _, kind := range canonicalJSONKinds() {
		if instanceTemplateMatches(
			fault.obligation.occurrence.instanceTemplate,
			s.model.root.occurrence.instanceTemplate,
		) && !activeSchemaAllowsKind(
			root, s.model.root.occurrence, fault.requirements, kind, make(map[*schemaNode]bool),
		) {
			continue
		}

		directed := fault
		directed.requirements = appendPlanRequirements(
			copyPlanRequirements(fault.requirements),
			kindRequirement(containerOccurrence, kind),
		)

		if rule == oracleRuleEnum {
			derivative, matched, err := findProjectedScalarDerivative(parent, directed, root, kind, s)
			if err != nil || matched {
				return derivative, matched, err
			}
		}

		derivative, matched, err := findScalarDerivativeFromRows(parent, directed, root, s)
		if err != nil || matched {
			return derivative, matched, err
		}
	}

	return nil, false, nil
}

func findProjectedScalarDerivative(
	parent *jsonValue,
	fault faultProgram,
	root *schemaNode,
	kind jsonKind,
	s *search,
) (*jsonValue, bool, error) {
	cursor := newRowProjectionCursor(root, s.model.root.occurrence, fault.requirements)
	defer cursor.Close()

	for {
		view, ok, err := cursor.Next()
		if err != nil || !ok {
			return nil, false, err
		}

		projected := func(yield func(*jsonValue) bool) error {
			return view.eachDirectValue(func(_ rowSchemaSource, value *jsonValue) bool {
				return value.kind != kind || yield(value)
			})
		}

		derivative, matched, err := firstReplacementDerivative(parent, fault, projected, s.model, s)
		if err != nil || matched {
			return derivative, matched, err
		}
	}
}

//nolint:cyclop // Local, allOf, and constrained anyOf kind constraints form one conjunction.
func activeSchemaAllowsKind(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	kind jsonKind,
	visiting map[*schemaNode]bool,
) bool {
	if node == nil || node.schemaShape == nil || visiting[node] {
		return false
	}

	visiting[node] = true
	defer delete(visiting, node)

	if node.kind != schemaAny && !nodeAcceptsKindForTarget(node, kind) {
		return false
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if !activeSchemaAllowsKind(child, childOccurrence, requirements, kind, visiting) {
			return false
		}
	}

	if len(node.anyOf) == 0 {
		return true
	}

	states, constrained := rowCompositionTruthStates(requirements, occurrence, "anyOf", len(node.anyOf))
	for index, child := range node.anyOf {
		if constrained && !states[index] {
			continue
		}

		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/anyOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if activeSchemaAllowsKind(child, childOccurrence, requirements, kind, visiting) {
			return true
		}
	}

	return false
}

//nolint:cyclop // Structural and composition children use one identity-preserving clone traversal.
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

	if node.items != nil {
		childOccurrence := rebasePlanOccurrence(
			node.items, occurrence, occurrence.usePointer+"/items",
			appendInstanceToken(occurrence.instanceTemplate, "*"),
		)
		if faultTargetWithin(target, childOccurrence) {
			shape.items = cloneWithoutFaultRule(node.items, childOccurrence, target, rule)
		}
	}

	shape.properties = make(map[string]*schemaNode, len(node.properties))
	for name, child := range node.properties {
		childOccurrence := rebasePlanOccurrence(
			child, occurrence, occurrence.usePointer+"/properties/"+escapePointerToken(name),
			appendInstanceToken(occurrence.instanceTemplate, name),
		)
		if faultTargetWithin(target, childOccurrence) {
			child = cloneWithoutFaultRule(child, childOccurrence, target, rule)
		}

		shape.properties[name] = child
	}

	if node.additionalProperties != nil {
		childOccurrence := rebasePlanOccurrence(
			node.additionalProperties, occurrence, occurrence.usePointer+"/additionalProperties",
			appendInstanceToken(occurrence.instanceTemplate, "*"),
		)
		if faultTargetWithin(target, childOccurrence) {
			shape.additionalProperties = cloneWithoutFaultRule(
				node.additionalProperties, childOccurrence, target, rule,
			)
		}
	}

	shape.allOf = append([]*schemaNode(nil), node.allOf...)
	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child, occurrence, occurrence.usePointer+"/allOf/"+itoa(index), occurrence.instanceTemplate,
		)
		if faultTargetWithin(target, childOccurrence) {
			shape.allOf[index] = cloneWithoutFaultRule(child, childOccurrence, target, rule)
		}
	}

	shape.anyOf = append([]*schemaNode(nil), node.anyOf...)
	for index, child := range node.anyOf {
		childOccurrence := rebasePlanOccurrence(
			child, occurrence, occurrence.usePointer+"/anyOf/"+itoa(index), occurrence.instanceTemplate,
		)
		if faultTargetWithin(target, childOccurrence) {
			shape.anyOf[index] = cloneWithoutFaultRule(child, childOccurrence, target, rule)
		}
	}

	return &schemaNode{schemaShape: &shape, occurrence: node.occurrence}
}

func faultTargetWithin(target, candidate schemaOccurrence) bool {
	return rowInstancePrefixMatches(candidate.instanceTemplate, target.instanceTemplate) &&
		(target.usePointer == candidate.usePointer || strings.HasPrefix(target.usePointer, candidate.usePointer+"/"))
}

func findNumberDerivative(parent *jsonValue, fault faultProgram, s *search) (*jsonValue, bool, error) {
	container, occurrence, found := resolveFaultContainer(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence, jsonNumber,
	)
	if !found {
		return nil, false, nil
	}

	var derivative *jsonValue

	complete, err := s.walkActiveNumberRules(
		container,
		occurrence,
		fault.requirements,
		nil,
		&fault,
		func(candidate *jsonValue) (bool, error) {
			selected, matched, selectErr := firstReplacementDerivative(
				parent, fault, singleJSONValueSource(candidate), s.model, s,
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

func findStringDerivative(parent *jsonValue, fault faultProgram, s *search) (*jsonValue, bool, error) {
	node, occurrence, found := resolveStringFaultTarget(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence,
	)
	if !found {
		return nil, false, fmt.Errorf("schematest: string fault target was not found: %s", fault.obligation.String())
	}

	objective := scalarFaultStringObjective(&fault, node, occurrence)
	if objective == nil {
		return nil, false, fmt.Errorf("schematest: %s is not a string fault", fault.obligation.rule)
	}

	var derivative *jsonValue

	_, complete, err := s.walkDirectedStringObjective(
		node,
		occurrence,
		fault.requirements,
		objective,
		func(candidate *jsonValue) (bool, error) {
			selected, matched, selectErr := firstReplacementDerivative(
				parent, fault, singleJSONValueSource(candidate), s.model, s,
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

// findScalarDerivativeFromRows obtains replacement values from the ordinary
// complete-row scalar search while directing only the selected rule false.
func findScalarDerivativeFromRows(
	parent *jsonValue,
	fault faultProgram,
	root *schemaNode,
	s *search,
) (*jsonValue, bool, error) {
	var derivative *jsonValue

	complete, err := s.walkNode(
		root,
		s.model.root.occurrence,
		fault.requirements,
		rowSearchContext{scalarFault: &fault},
		func(candidateRow *jsonValue) (bool, error) {
			for _, path := range matchingValuePaths(candidateRow, fault.obligation.occurrence.instanceTemplate) {
				candidate := valueAtPath(candidateRow, path)
				if candidate == nil {
					continue
				}

				selected, matched, selectErr := firstReplacementDerivative(
					parent, fault, singleJSONValueSource(candidate), s.model, s,
				)
				if selectErr != nil {
					return false, selectErr
				}

				if matched {
					derivative = selected

					return true, nil
				}
			}

			return false, nil
		},
	)
	if err != nil {
		return nil, false, err
	}

	return derivative, complete, nil
}

//nolint:cyclop // Charging, copying, replacement, and matching share one lazy candidate boundary.
func firstReplacementDerivative(
	parent *jsonValue,
	fault faultProgram,
	candidates jsonValueSource,
	model *schemaModel,
	s *search,
) (*jsonValue, bool, error) {
	paths := matchingValuePaths(parent, fault.obligation.occurrence.instanceTemplate)
	for _, path := range paths {
		var (
			selected     *jsonValue
			candidateErr error
		)

		err := candidates(func(candidate *jsonValue) bool {
			if assignErr := s.assign(); assignErr != nil {
				candidateErr = assignErr

				return false
			}

			derivative, copyErr := cloneJSONValue(parent)
			if copyErr != nil {
				candidateErr = copyErr

				return false
			}

			copyCandidate, copyErr := cloneJSONValue(candidate)
			if copyErr != nil {
				candidateErr = copyErr

				return false
			}

			if !replaceValueAtPath(derivative, path, copyCandidate) {
				return true
			}

			matched, matchErr := derivativeMatchesFault(model, derivative, fault)
			if matchErr != nil {
				candidateErr = matchErr

				return false
			}

			if matched {
				selected = derivative

				return false
			}

			return true
		})
		if err != nil {
			return nil, false, err
		}

		if candidateErr != nil || selected != nil {
			return selected, selected != nil, candidateErr
		}
	}

	return nil, false, nil
}

func singleJSONValueSource(candidate *jsonValue) jsonValueSource {
	return func(yield func(*jsonValue) bool) error {
		yield(candidate)

		return nil
	}
}

// findArrayCountDerivative searches complete authored arrays first, then every
// order-preserving deletion or insertion alternative without retaining candidates.
//
//nolint:cyclop // Exact count direction, occurrence search, and mutation family dispatch meet here.
func findArrayCountDerivative(
	parent *jsonValue,
	fault faultProgram,
	node *schemaNode,
	s *search,
) (*jsonValue, bool, error) {
	bound := node.minItems
	if fault.obligation.rule == oracleRuleMaxItems {
		bound = node.maxItems
	}

	count, fits, err := exactCountUint64(bound)
	if err != nil {
		return nil, false, err
	}

	if !fits || fault.obligation.rule == oracleRuleMaxItems && count == ^uint64(0) {
		return nil, false, chargeArrayInsertionsToCutoff(s)
	}

	desired := count
	if fault.obligation.rule == oracleRuleMinItems {
		if desired == 0 {
			return nil, false, nil
		}

		desired--
	} else {
		desired++
	}

	if desired > uint64(maxInt()) {
		return nil, false, chargeArrayInsertionsToCutoff(s)
	}

	container, containerOccurrence, found := resolveFaultContainer(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence, jsonArray,
	)
	if !found {
		return nil, false, nil
	}

	for _, path := range matchingValuePaths(parent, fault.obligation.occurrence.instanceTemplate) {
		value := valueAtPath(parent, path)
		if value == nil || value.kind != jsonArray {
			continue
		}

		derivative, matched, candidateErr := findAuthoredArrayCountDerivative(
			parent, path, int(desired), fault, container, containerOccurrence, s,
		)
		if candidateErr != nil || matched {
			return derivative, matched, candidateErr
		}

		if len(value.array) > int(desired) {
			derivative, matched, candidateErr = findArrayDeletionDerivative(
				parent, path, int(desired), fault, s,
			)
		} else if len(value.array) < int(desired) {
			derivative, matched, candidateErr = findArrayInsertionDerivative(
				parent, path, int(desired), fault, container, containerOccurrence, s,
			)
		}

		if candidateErr != nil || matched {
			return derivative, matched, candidateErr
		}
	}

	return nil, false, nil
}

//nolint:cyclop // Projection traversal and callback error propagation share one lazy boundary.
func findAuthoredArrayCountDerivative(
	parent *jsonValue,
	path []string,
	desired int,
	fault faultProgram,
	container *schemaNode,
	occurrence schemaOccurrence,
	s *search,
) (*jsonValue, bool, error) {
	cursor := newRowProjectionCursor(container, occurrence, fault.requirements)
	defer cursor.Close()

	for {
		view, ok, err := cursor.Next()
		if err != nil || !ok {
			return nil, false, err
		}

		active, err := view.appendBranchRequirements(
			append([]requirement(nil), fault.requirements...), s.assign,
		)
		if err != nil {
			return nil, false, err
		}

		if !rowProjectionAcceptsKind(view, jsonArray) ||
			!rowProjectionRequirementsAcceptKind(view, active, jsonArray) {
			continue
		}

		var (
			derivative   *jsonValue
			candidateErr error
		)

		err = view.eachDirectValue(func(_ rowSchemaSource, value *jsonValue) bool {
			if value.kind != jsonArray || len(value.array) != desired {
				return true
			}

			derivative, _, candidateErr = tryArrayCountCandidate(parent, path, value.array, fault, s)

			return candidateErr == nil && derivative == nil
		})
		if err != nil {
			return nil, false, err
		}

		if candidateErr != nil || derivative != nil {
			return derivative, derivative != nil, candidateErr
		}
	}
}

func findArrayDeletionDerivative(
	parent *jsonValue,
	path []string,
	desired int,
	fault faultProgram,
	s *search,
) (*jsonValue, bool, error) {
	array := valueAtPath(parent, path)
	removed := len(array.array) - desired
	cursor := newArrayCombinationCursor(len(array.array), removed)

	for indexes, ok := cursor.Next(); ok; indexes, ok = cursor.Next() {
		values := make([]*jsonValue, 0, desired)
		removedIndex := 0

		for index, value := range array.array {
			if removedIndex < len(indexes) && indexes[removedIndex] == index {
				removedIndex++

				continue
			}

			values = append(values, value)
		}

		derivative, matched, err := tryArrayCountCandidate(parent, path, values, fault, s)
		if err != nil || matched {
			return derivative, matched, err
		}
	}

	return nil, false, nil
}

//nolint:cyclop,gocognit // Three fair ranks meet at one independently owned insertion attempt.
func findArrayInsertionDerivative(
	parent *jsonValue,
	path []string,
	desired int,
	fault faultProgram,
	container *schemaNode,
	occurrence schemaOccurrence,
	s *search,
) (*jsonValue, bool, error) {
	array := valueAtPath(parent, path)

	insertions := desired - len(array.array)
	if uint64(insertions) > s.maxSteps-s.steps {
		return nil, false, chargeArrayInsertionsToCutoff(s)
	}

	projectionCount, err := rowProjectionNodeCount(container, occurrence, fault.requirements)
	if err != nil {
		return nil, false, err
	}

	layoutCount := saturatedBinomial(uint64(desired), uint64(insertions))

	const productDimensions = 3

	frontier, err := newRankProductCursor(productDimensions)
	if err != nil {
		return nil, false, err
	}

	if err := frontier.SetFinite(0, projectionCount); err != nil {
		return nil, false, err
	}

	if err := frontier.SetFinite(1, layoutCount); err != nil {
		return nil, false, err
	}

	var diagonal uint64

	diagonalLive := true

	for {
		ranks, ok := frontier.Next()
		if !ok {
			return nil, false, nil
		}

		if frontier.diagonal != diagonal {
			if !diagonalLive {
				return nil, false, nil
			}

			diagonal = frontier.diagonal
			diagonalLive = false
		}

		view, exists, err := rowProjectionAt(container, occurrence, fault.requirements, ranks[0])
		if err != nil {
			return nil, false, err
		}

		if !exists {
			continue
		}

		if ranks[0] == diagonal {
			diagonalLive = true
		}

		active, err := view.appendBranchRequirements(
			append([]requirement(nil), fault.requirements...), s.assign,
		)
		if err != nil {
			return nil, false, err
		}

		if ranks[1] == diagonal {
			diagonalLive = true
		}

		tupleValues, exists, _, _, err := s.rowArrayChildrenForOrdinal(
			rankedArrayStructure{view: view, active: active, length: rowArrayCount{value: uint64(insertions)}},
			active,
			rowSearchContext{},
			ranks[2],
		)
		if err != nil {
			return nil, false, err
		}

		if !exists {
			continue
		}

		diagonalLive = true

		indexes, exists := arrayCombinationAt(desired, insertions, ranks[1])
		if !exists {
			return nil, false, errors.New("schematest: array insertion layout rank disappeared")
		}

		values := make([]*jsonValue, 0, desired)
		parentIndex := 0
		insertedIndex := 0

		for index := 0; index < desired; index++ {
			if insertedIndex < len(indexes) && indexes[insertedIndex] == index {
				values = append(values, tupleValues[insertedIndex])
				insertedIndex++

				continue
			}

			values = append(values, array.array[parentIndex])
			parentIndex++
		}

		derivative, matched, err := tryArrayCountCandidate(parent, path, values, fault, s)
		if err != nil || matched {
			return derivative, matched, err
		}
	}
}

// tryArrayCountCandidate charges the retry, length edit, changed indexes, and
// replacement values before installing one independently cloned complete array.
//
//nolint:cyclop // Charging must remain immediately adjacent to each atomic assignment.
func tryArrayCountCandidate(
	parent *jsonValue,
	path []string,
	values []*jsonValue,
	fault faultProgram,
	s *search,
) (*jsonValue, bool, error) {
	if err := s.assign(); err != nil {
		return nil, false, err
	}

	current := valueAtPath(parent, path)
	if current == nil || current.kind != jsonArray {
		return nil, false, errors.New("schematest: array fault path is not an array")
	}

	if err := s.assign(); err != nil {
		return nil, false, err
	}

	for index, value := range values {
		if index < len(current.array) && jsonValuesEqual(current.array[index], value) {
			continue
		}

		if err := s.assign(); err != nil {
			return nil, false, err
		}

		if err := s.assign(); err != nil {
			return nil, false, err
		}
	}

	candidate, err := cloneJSONValue(parent)
	if err != nil {
		return nil, false, err
	}

	owned := &jsonValue{kind: jsonArray, array: make([]*jsonValue, 0, len(values))}
	for _, value := range values {
		item, cloneErr := cloneJSONValue(value)
		if cloneErr != nil {
			return nil, false, cloneErr
		}

		owned.array = append(owned.array, item)
	}

	if !replaceValueAtPath(candidate, path, owned) {
		return nil, false, errors.New("schematest: array fault path disappeared")
	}

	matched, err := derivativeMatchesFault(s.model, candidate, fault)
	if err != nil || !matched {
		return nil, false, err
	}

	return candidate, true, nil
}

func chargeArrayInsertionsToCutoff(s *search) error {
	for index := uint64(0); ; index++ {
		if err := s.assign(); err != nil {
			return err
		}

		if err := s.assign(); err != nil {
			return err
		}

		if err := s.assign(); err != nil {
			return err
		}
	}
}

type arrayCombinationCursor struct {
	indexes []int
	length  int
	started bool
}

func newArrayCombinationCursor(length, selected int) *arrayCombinationCursor {
	indexes := make([]int, selected)
	for index := range indexes {
		indexes[index] = index
	}

	return &arrayCombinationCursor{indexes: indexes, length: length}
}

func (cursor *arrayCombinationCursor) Next() ([]int, bool) {
	if cursor == nil || len(cursor.indexes) > cursor.length {
		return nil, false
	}

	if !cursor.started {
		cursor.started = true

		return cursor.indexes, true
	}

	for index := len(cursor.indexes) - 1; index >= 0; index-- {
		maximum := cursor.length - len(cursor.indexes) + index
		if cursor.indexes[index] == maximum {
			continue
		}

		cursor.indexes[index]++
		for next := index + 1; next < len(cursor.indexes); next++ {
			cursor.indexes[next] = cursor.indexes[next-1] + 1
		}

		return cursor.indexes, true
	}

	return nil, false
}

func arrayCombinationAt(length, selected int, rank uint64) ([]int, bool) {
	if selected < 0 || selected > length || rank >= saturatedBinomial(uint64(length), uint64(selected)) {
		return nil, false
	}

	indexes := make([]int, 0, selected)
	next := 0

	for remaining := selected; remaining > 0; remaining-- {
		maximum := length - remaining
		for candidate := next; candidate <= maximum; candidate++ {
			block := saturatedBinomial(uint64(length-candidate-1), uint64(remaining-1))
			if rank < block {
				indexes = append(indexes, candidate)
				next = candidate + 1

				break
			}

			rank -= block
		}
	}

	return indexes, true
}

//nolint:cyclop // Structural choice and complete child walking share one seam.
func walkActiveFaultChildValues(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	kind rowChildKind,
	name string,
	s *search,
	visit rowVisit,
) (bool, error) {
	if kind == rowChildProperty {
		members, err := rowObjectMembers(node, occurrence, requirements)
		if err != nil {
			return false, err
		}

		for _, member := range members {
			if member.name == name {
				return s.walkRowMemberValues(member, requirements, rowSearchContext{}, visit)
			}
		}

		return s.walkGenericValue(requirements, visit)
	}

	cursor := newRowProjectionCursor(node, occurrence, requirements)
	defer cursor.Close()

	for {
		view, ok, err := cursor.Next()
		if err != nil || !ok {
			return false, err
		}

		active, err := view.appendBranchRequirements(
			append([]requirement(nil), requirements...), s.assign,
		)
		if err != nil {
			return false, err
		}

		complete, walkErr := s.walkRowSchemaConjunction(
			rowProjectedArrayItems(view, active), active, rowSearchContext{}, visit,
		)
		if walkErr != nil || complete {
			return complete, walkErr
		}
	}
}

//nolint:cyclop,gocognit // Exact count conversion and active member search are one mutation.
func findObjectCountDerivative(
	parent *jsonValue,
	fault faultProgram,
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
		return nil, false, chargeObjectInsertionsToCutoff(s)
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
			return nil, false, chargeObjectInsertionsToCutoff(s)
		}

		growthRequirements := append([]requirement(nil), fault.requirements...)

		members, memberErr := rowObjectMembers(container, containerOccurrence, growthRequirements)
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

			growthRequirements = append(growthRequirements, faultMemberPresenceRequirement(containerOccurrence, name))
			available++
		}

		if len(growthRequirements) != len(fault.requirements) {
			members, memberErr = rowObjectMembers(container, containerOccurrence, growthRequirements)
			if memberErr != nil {
				return nil, false, memberErr
			}
		}

		derivative, found, searchErr := findObjectGrowthDerivative(
			parent, path, desired, members, fault, growthRequirements, s,
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
	fault faultProgram,
	node *schemaNode,
	paths [][]string,
	s *search,
) (*jsonValue, bool, error) {
	count, fits, countErr := exactCountUint64(node.minProperties)
	if countErr != nil {
		return nil, false, countErr
	}

	if !fits || count > uint64(maxInt()) {
		return nil, false, chargeObjectDeletionsToCutoff(s)
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

			candidate, copyErr := cloneJSONValue(parent)
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

			matched, matchErr := derivativeMatchesFault(s.model, candidate, fault)
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

func faultMemberPresenceRequirement(occurrence schemaOccurrence, name string) requirement {
	return requirement{
		tag: requirementPresenceState,
		occurrence: schemaOccurrence{
			usePointer:       occurrence.usePointer + "/additionalProperties",
			targetPointer:    occurrence.targetPointer,
			instanceTemplate: appendInstanceToken(occurrence.instanceTemplate, name),
		},
		presence: requirementPresent,
	}
}

//nolint:cyclop // Member/value backtracking and closure checking form one DFS.
func findObjectGrowthDerivative(
	parent *jsonValue,
	path []string,
	desired int,
	members []rowMember,
	fault faultProgram,
	requirements []requirement,
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
			matched, err := derivativeMatchesFault(s.model, candidate, fault)
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
				member, requirements, rowSearchContext{}, func(value *jsonValue) (bool, error) {
					if assignErr := s.assign(); assignErr != nil {
						return false, assignErr
					}

					next, copyErr := cloneJSONValue(candidate)
					if copyErr != nil {
						return false, copyErr
					}

					copiedValue, copyErr := cloneJSONValue(value)
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

func findRequiredDerivative(parent *jsonValue, fault faultProgram, s *search) (*jsonValue, bool, error) {
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

		candidate, err := cloneJSONValue(parent)
		if err != nil {
			return nil, false, err
		}

		object := valueAtPath(candidate, path)
		if object == nil || object.kind != jsonObject {
			continue
		}

		delete(object.object, name)

		matched, err := derivativeMatchesFault(s.model, candidate, fault)
		if err != nil || matched {
			return candidate, matched, err
		}
	}

	return nil, false, nil
}

//nolint:cyclop // Path and active-witness retries form one mutation search.
func findAdditionalPropertyDerivative(
	parent *jsonValue,
	fault faultProgram,
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

	requirements := append([]requirement(nil), fault.requirements...)
	requirements = append(requirements, faultMemberPresenceRequirement(containerOccurrence, name))

	for _, path := range matchingValuePaths(parent, parentPointer) {
		var derivative *jsonValue

		complete, err := walkActiveFaultChildValues(
			container, containerOccurrence, requirements, rowChildProperty, name, s,
			func(witness *jsonValue) (bool, error) {
				if assignErr := s.assign(); assignErr != nil {
					return false, assignErr
				}

				candidate, copyErr := cloneJSONValue(parent)
				if copyErr != nil {
					return false, copyErr
				}

				value, copyErr := cloneJSONValue(witness)
				if copyErr != nil {
					return false, copyErr
				}

				object := valueAtPath(candidate, path)
				if object == nil || object.kind != jsonObject {
					return false, nil
				}

				object.object[name] = value

				matched, matchErr := derivativeMatchesFault(s.model, candidate, fault)
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

func derivativeMatchesFault(model *schemaModel, derivative *jsonValue, fault faultProgram) (bool, error) {
	result := evaluate(model, derivative)
	if result.err != nil {
		return false, fmt.Errorf("evaluate fault derivative: %w", result.err)
	}

	if result.valid {
		return false, nil
	}

	return faultFailureClosureMatches(result.failureRecords(), fault)
}

func derivativeHasClosure(model *schemaModel, derivative *jsonValue, closure []failureIdentity) (bool, error) {
	return derivativeMatchesFault(model, derivative, faultProgram{expected: closure})
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
			node.items,
			occurrence,
			occurrence.usePointer+"/items",
			appendInstanceToken(occurrence.instanceTemplate, "*"),
		)})
	}

	for _, name := range sortedSchemaPropertyNames(node.properties) {
		property := node.properties[name]
		children = append(children, faultSchemaChild{property, rebasePlanOccurrence(
			property,
			occurrence,
			occurrence.usePointer+"/properties/"+escapePointerToken(name),
			appendInstanceToken(occurrence.instanceTemplate, name),
		)})
	}

	if node.additionalProperties != nil {
		children = append(children, faultSchemaChild{node.additionalProperties, rebasePlanOccurrence(
			node.additionalProperties,
			occurrence,
			occurrence.usePointer+"/additionalProperties",
			appendInstanceToken(occurrence.instanceTemplate, "*"),
		)})
	}

	for index, child := range node.allOf {
		children = append(children, faultSchemaChild{child, rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)})
	}

	for index, child := range node.anyOf {
		children = append(children, faultSchemaChild{child, rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/anyOf/"+itoa(index),
			occurrence.instanceTemplate,
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

func chargeObjectInsertionsToCutoff(s *search) error {
	for {
		if err := s.assign(); err != nil {
			return err
		}

		if err := s.assign(); err != nil {
			return err
		}

		if err := s.assign(); err != nil {
			return err
		}
	}
}

func chargeObjectDeletionsToCutoff(s *search) error {
	for {
		if err := s.assign(); err != nil {
			return err
		}

		if err := s.assign(); err != nil {
			return err
		}
	}
}

func maxInt() int {
	return int(^uint(0) >> 1)
}
