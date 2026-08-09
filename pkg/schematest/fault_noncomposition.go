//nolint:godoclint // Private fault helpers stay behind Build.
package schematest

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const objectFaultProductDimensions = 4

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

// findObjectCountDerivative directs the selected count rule false and lets the
// ordinary object projection search names, presence states, and complete values.
//
//nolint:cyclop // Kind pruning, exact count direction, and the one-member cursor meet here.
func findObjectCountDerivative(
	parent *jsonValue,
	fault faultProgram,
	node *schemaNode,
	s *search,
) (*jsonValue, bool, error) {
	container, containerOccurrence, found := resolveFaultContainer(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence, jsonObject,
	)
	if !found || !activeSchemaAllowsKind(
		container, containerOccurrence, fault.requirements, jsonObject, make(map[*schemaNode]bool),
	) {
		return nil, false, nil
	}

	bound := node.minProperties
	direction := int64(-1)

	if fault.obligation.rule == oracleRuleMaxProperties {
		bound = node.maxProperties
		direction = 1
	}

	if bound == nil || bound.number == nil {
		return nil, false, errors.New("schematest: object count fault has no bound")
	}

	one, err := parseExactNumber("1")
	if err != nil {
		return nil, false, err
	}

	desired, err := addSignedExactNumbers(bound.number, one, direction)
	if err != nil {
		return nil, false, err
	}

	if desired.numerator.Sign() < 0 {
		return nil, false, nil
	}

	root := cloneWithoutFaultRule(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence, fault.obligation.rule,
	)
	if fault.obligation.rule == oracleRuleMaxProperties {
		count, fits, countErr := exactCountUint64(&exactCount{number: desired})
		if countErr != nil {
			return nil, false, countErr
		}

		if fits {
			derivative, matched, searchErr := findSingleObjectGrowthDerivative(
				parent, fault, root, count, s,
			)
			if searchErr != nil || matched {
				return derivative, matched, searchErr
			}
		}
	}

	directed := fault
	directed.requirements = appendPlanRequirements(
		copyPlanRequirements(fault.requirements),
		requirement{
			tag: requirementExactCount, occurrence: fault.obligation.occurrence,
			count: &exactCount{number: desired},
		},
	)

	return findObjectReplacementFromRows(parent, directed, root, s)
}

// findSingleObjectGrowthDerivative shares one diagonal projection/key/value
// cursor when the current parent is exactly one member below the directed count.
//
//nolint:cyclop,gocognit // One fixed product decodes path, projection, key, and value ranks.
func findSingleObjectGrowthDerivative(
	parent *jsonValue,
	fault faultProgram,
	root *schemaNode,
	desired uint64,
	s *search,
) (*jsonValue, bool, error) {
	container, occurrence, found := resolveFaultContainer(
		root, s.model.root.occurrence, fault.obligation.occurrence, jsonObject,
	)
	if !found {
		return nil, false, nil
	}

	projectionCount, err := rowProjectionNodeCount(container, occurrence, fault.requirements)
	if err != nil {
		return nil, false, err
	}

	paths := matchingValuePaths(parent, fault.obligation.occurrence.instanceTemplate)
	if len(paths) == 0 {
		return nil, false, nil
	}

	frontier, err := newRankProductCursor(objectFaultProductDimensions)
	if err != nil {
		return nil, false, err
	}

	if err := frontier.SetFinite(0, uint64(len(paths))); err != nil {
		return nil, false, err
	}

	if err := frontier.SetFinite(1, projectionCount); err != nil {
		return nil, false, err
	}

	for {
		ranks, ok := frontier.Next()
		if !ok {
			return nil, false, nil
		}

		path := paths[ranks[0]]

		current := valueAtPath(parent, path)
		if current == nil || current.kind != jsonObject || uint64(len(current.object))+1 != desired {
			continue
		}

		view, exists, decodeErr := rowProjectionAt(
			container, occurrence, fault.requirements, ranks[1],
		)
		if decodeErr != nil {
			return nil, false, decodeErr
		}

		if !exists {
			continue
		}

		active, activeErr := view.appendBranchRequirements(
			copyPlanRequirements(fault.requirements), s.assign,
		)
		if activeErr != nil {
			return nil, false, activeErr
		}

		shape, shapeErr := newRowProjectedObject(view, active, occurrence)
		if shapeErr != nil {
			return nil, false, shapeErr
		}

		if err := s.assign(); err != nil {
			return nil, false, err
		}

		member, memberExists, memberErr := objectGrowthMemberAtRank(shape, current, active, ranks[2])
		if memberErr != nil {
			return nil, false, memberErr
		}

		if !memberExists {
			continue
		}

		value, valueExists, usable, _, valueErr := s.rowConjunctionValueAt(
			member.schemas, active, rowSearchContext{}, ranks[3],
		)
		if valueErr != nil {
			return nil, false, valueErr
		}

		if !valueExists || !usable {
			continue
		}

		replacement, cloneErr := cloneJSONValue(current)
		if cloneErr != nil {
			return nil, false, cloneErr
		}

		replacement.object[member.name] = value

		candidate, matched, candidateErr := tryObjectReplacement(
			parent, path, current, replacement, fault, s,
		)
		if candidateErr != nil || matched {
			return candidate, matched, candidateErr
		}
	}
}

func objectGrowthMemberAtRank(
	shape *rowProjectedObject,
	current *jsonValue,
	requirements []requirement,
	rank uint64,
) (rowMember, bool, error) {
	for _, member := range shape.members {
		if _, present := current.object[member.name]; present {
			continue
		}

		if rank == 0 {
			return member, true, nil
		}

		rank--
	}

	if !shape.allowsExtra {
		return rowMember{}, false, nil
	}

	for freshRank := uint64(0); ; freshRank++ {
		name := freshObjectMutationName(freshRank)
		if _, present := current.object[name]; present {
			continue
		}

		member, allowed, err := projectedAdditionalMember(shape, name, requirements)
		if err != nil {
			return rowMember{}, false, err
		}

		if !allowed {
			continue
		}

		if rank == 0 {
			return member, true, nil
		}

		rank--
	}
}

// findObjectReplacementFromRows installs one complete object produced by the
// shared projection search and accepts only the exact declared failure closure.
//
//nolint:cyclop // Complete row evaluation and concrete occurrence replacement share one callback.
func findObjectReplacementFromRows(
	parent *jsonValue,
	fault faultProgram,
	root *schemaNode,
	s *search,
) (*jsonValue, bool, error) {
	model := *s.model
	model.root = root

	var derivative *jsonValue

	complete, err := s.walkNode(
		root,
		s.model.root.occurrence,
		fault.requirements,
		rowSearchContext{},
		func(row *jsonValue) (bool, error) {
			result := evaluate(&model, row)
			if result.err != nil {
				return false, fmt.Errorf("evaluate object fault witness: %w", result.err)
			}

			if !result.valid {
				return false, nil
			}

			for _, path := range matchingValuePaths(row, fault.obligation.occurrence.instanceTemplate) {
				replacement := valueAtPath(row, path)

				current := valueAtPath(parent, path)
				if replacement == nil || replacement.kind != jsonObject || current == nil || current.kind != jsonObject {
					continue
				}

				candidate, matched, candidateErr := tryObjectReplacement(
					parent, path, current, replacement, fault, s,
				)
				if candidateErr != nil {
					return false, candidateErr
				}

				if matched {
					derivative = candidate

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

// tryObjectReplacement charges one retry and every changed member assignment
// immediately before installing an independently owned replacement.
//
//nolint:cyclop,gocognit // Each changed name, presence, key, and value is charged at its assignment.
func tryObjectReplacement(
	parent *jsonValue,
	path []string,
	current *jsonValue,
	replacement *jsonValue,
	fault faultProgram,
	s *search,
) (*jsonValue, bool, error) {
	if err := s.assign(); err != nil {
		return nil, false, err
	}

	names := make(map[string]bool, len(current.object)+len(replacement.object))
	for name := range current.object {
		names[name] = true
	}

	for name := range replacement.object {
		names[name] = true
	}

	for _, name := range sortedBoolNames(names) {
		before, beforePresent := current.object[name]

		after, afterPresent := replacement.object[name]
		if beforePresent == afterPresent && (!beforePresent || jsonValuesEqual(before, after)) {
			continue
		}

		if err := s.assign(); err != nil {
			return nil, false, err
		}

		if beforePresent != afterPresent {
			if err := s.assign(); err != nil {
				return nil, false, err
			}
		}

		if afterPresent {
			if !beforePresent {
				if err := s.assign(); err != nil {
					return nil, false, err
				}
			}

			if err := s.assign(); err != nil {
				return nil, false, err
			}
		}
	}

	candidate, err := cloneJSONValue(parent)
	if err != nil {
		return nil, false, err
	}

	owned, err := cloneJSONValue(replacement)
	if err != nil {
		return nil, false, err
	}

	if !replaceValueAtPath(candidate, path, owned) {
		return nil, false, errors.New("schematest: object fault path disappeared")
	}

	matched, err := derivativeMatchesFault(s.model, candidate, fault)
	if err != nil || !matched {
		return nil, false, err
	}

	return candidate, true, nil
}

func sortedBoolNames(values map[string]bool) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

//nolint:cyclop // Retry, name, and presence charging must precede each independent deletion.
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

		if err := s.assign(); err != nil {
			return nil, false, err
		}

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

// findAdditionalPropertyDerivative searches names from the targeted closed
// occurrence while obtaining each value from the complete active conjunction.
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
	targetOccurrence := fault.obligation.occurrence
	targetOccurrence.instanceTemplate = parentPointer

	target, targetOccurrence, found := resolveExactFaultTarget(
		s.model.root, s.model.root.occurrence, targetOccurrence,
	)
	if !found {
		return nil, false, nil
	}

	root := cloneWithoutFaultRule(
		s.model.root, s.model.root.occurrence, targetOccurrence, oracleRuleAdditionalProperties,
	)

	container, containerOccurrence, found := resolveFaultContainer(
		root, s.model.root.occurrence, targetOccurrence, jsonObject,
	)
	if !found || !activeSchemaAllowsKind(
		container, containerOccurrence, fault.requirements, jsonObject, make(map[*schemaNode]bool),
	) {
		return nil, false, nil
	}

	return searchAdditionalPropertyNames(
		parent,
		matchingValuePaths(parent, parentPointer),
		fault,
		target,
		container,
		containerOccurrence,
		s,
	)
}

// searchDeclaredAdditionalPropertyNames tries active declared members before
// entering the unbounded fresh-name frontier.
//
//nolint:cyclop,gocognit // Path, projection, member, and value exhaustion form one canonical prefix.
func searchDeclaredAdditionalPropertyNames(
	parent *jsonValue,
	paths [][]string,
	fault faultProgram,
	target *schemaNode,
	container *schemaNode,
	containerOccurrence schemaOccurrence,
	s *search,
) (*jsonValue, bool, error) {
	for _, path := range paths {
		object := valueAtPath(parent, path)
		if object == nil || object.kind != jsonObject {
			continue
		}

		cursor := newRowProjectionCursor(container, containerOccurrence, fault.requirements)
		for {
			view, ok, err := cursor.Next()
			if err != nil {
				cursor.Close()

				return nil, false, err
			}

			if !ok {
				break
			}

			active, activeErr := view.appendBranchRequirements(
				copyPlanRequirements(fault.requirements), s.assign,
			)
			if activeErr != nil {
				cursor.Close()

				return nil, false, activeErr
			}

			shape, shapeErr := newRowProjectedObject(view, active, containerOccurrence)
			if shapeErr != nil {
				cursor.Close()

				return nil, false, shapeErr
			}

			for _, member := range shape.members {
				if assignErr := s.assign(); assignErr != nil {
					cursor.Close()

					return nil, false, assignErr
				}

				if _, declared := target.properties[member.name]; declared {
					continue
				}

				if _, collision := object.object[member.name]; collision {
					continue
				}

				var derivative *jsonValue

				complete, walkErr := s.walkRowMemberValues(
					member, active, rowSearchContext{}, func(witness *jsonValue) (bool, error) {
						candidate, candidateErr := installAdditionalProperty(
							parent, path, member.name, witness, fault, s,
						)
						if candidateErr != nil || candidate == nil {
							return false, candidateErr
						}

						derivative = candidate

						return true, nil
					},
				)
				if walkErr != nil || complete {
					cursor.Close()

					return derivative, complete, walkErr
				}
			}
		}

		cursor.Close()
	}

	return nil, false, nil
}

// searchAdditionalPropertyNames diagonally enumerates occurrence, projection,
// fresh-name, and value ranks without retaining a key or value corpus.
//
//nolint:cyclop // One product decodes occurrence, projection, name, and value ranks.
func searchAdditionalPropertyNames(
	parent *jsonValue,
	paths [][]string,
	fault faultProgram,
	target *schemaNode,
	container *schemaNode,
	containerOccurrence schemaOccurrence,
	s *search,
) (*jsonValue, bool, error) {
	if len(paths) == 0 {
		return nil, false, nil
	}

	derivative, found, err := searchDeclaredAdditionalPropertyNames(
		parent, paths, fault, target, container, containerOccurrence, s,
	)
	if err != nil || found {
		return derivative, found, err
	}

	projectionCount, err := rowProjectionNodeCount(container, containerOccurrence, fault.requirements)
	if err != nil {
		return nil, false, err
	}

	frontier, err := newRankProductCursor(objectFaultProductDimensions)
	if err != nil {
		return nil, false, err
	}

	if err := frontier.SetFinite(0, uint64(len(paths))); err != nil {
		return nil, false, err
	}

	if err := frontier.SetFinite(1, projectionCount); err != nil {
		return nil, false, err
	}

	for {
		ranks, ok := frontier.Next()
		if !ok {
			return nil, false, nil
		}

		path := paths[ranks[0]]

		object := valueAtPath(parent, path)
		if object == nil || object.kind != jsonObject {
			continue
		}

		view, exists, err := rowProjectionAt(
			container, containerOccurrence, fault.requirements, ranks[1],
		)
		if err != nil {
			return nil, false, err
		}

		if !exists {
			continue
		}

		active, err := view.appendBranchRequirements(
			copyPlanRequirements(fault.requirements), s.assign,
		)
		if err != nil {
			return nil, false, err
		}

		shape, err := newRowProjectedObject(view, active, containerOccurrence)
		if err != nil {
			return nil, false, err
		}

		if assignErr := s.assign(); assignErr != nil {
			return nil, false, assignErr
		}

		name := freshObjectMutationName(ranks[2])

		member, exists, err := projectedAdditionalMember(shape, name, active)
		if err != nil {
			return nil, false, err
		}

		if !exists {
			continue
		}

		candidate, matched, candidateErr := tryAdditionalPropertyName(
			parent, path, object, fault, target, member, active, ranks[3], s,
		)
		if candidateErr != nil || matched {
			return candidate, matched, candidateErr
		}
	}
}

func freshObjectMutationName(rank uint64) string {
	const base = "__schematest_extra__"
	if rank == 0 {
		return base
	}

	return fmt.Sprintf("%s_%d", base, rank)
}

// tryAdditionalPropertyName skips only collisions and names declared by the
// directed occurrence, then searches its complete active value requirements.
func tryAdditionalPropertyName(
	parent *jsonValue,
	path []string,
	object *jsonValue,
	fault faultProgram,
	target *schemaNode,
	member rowMember,
	requirements []requirement,
	valueRank uint64,
	s *search,
) (*jsonValue, bool, error) {
	name := member.name
	if _, declared := target.properties[name]; declared {
		return nil, false, nil
	}

	if _, collision := object.object[name]; collision {
		return nil, false, nil
	}

	witness, exists, usable, _, err := s.rowConjunctionValueAt(
		member.schemas, requirements, rowSearchContext{}, valueRank,
	)
	if err != nil || !exists || !usable {
		return nil, false, err
	}

	candidate, err := installAdditionalProperty(parent, path, name, witness, fault, s)
	if err != nil || candidate == nil {
		return nil, false, err
	}

	return candidate, true, nil
}

// installAdditionalProperty charges retry, presence, key, and value choices
// before mutating an independent occurrence-owned clone.
func installAdditionalProperty(
	parent *jsonValue,
	path []string,
	name string,
	witness *jsonValue,
	fault faultProgram,
	s *search,
) (*jsonValue, error) {
	for range 4 {
		if err := s.assign(); err != nil {
			return nil, err
		}
	}

	candidate, err := cloneJSONValue(parent)
	if err != nil {
		return nil, err
	}

	value, err := cloneJSONValue(witness)
	if err != nil {
		return nil, err
	}

	object := valueAtPath(candidate, path)
	if object == nil || object.kind != jsonObject {
		return nil, errors.New("schematest: additional-property fault path disappeared")
	}

	object.object[name] = value

	matched, err := derivativeMatchesFault(s.model, candidate, fault)
	if err != nil || !matched {
		return nil, err
	}

	return candidate, nil
}

func derivativeMatchesFault(model *schemaModel, derivative *jsonValue, fault faultProgram) (bool, error) {
	result := evaluate(model, derivative)
	if result.err != nil {
		return false, fmt.Errorf("evaluate fault derivative: %w", result.err)
	}

	if result.valid {
		return false, nil
	}

	return faultFailureClosureMatches(result, fault)
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

func maxInt() int {
	return int(^uint(0) >> 1)
}
