//nolint:godoclint // Private fault helpers stay behind Build.
package schematest

import (
	"errors"
	"fmt"
	"iter"
	"sort"
	"strconv"
	"strings"
)

const (
	objectFaultProductDimensions = 4
	arrayFaultProductDimensions  = 3
	directRowRankDimensions      = 2
	objectGrowthRanksPerAddition = 2
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

// nonCompositionFaultAttemptAtRank performs one addressed family mutation.
func nonCompositionFaultAttemptAtRank(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	switch fault.obligation.rule {
	case oracleRuleEnum, oracleRuleMinimum, oracleRuleExclusiveMinimum, oracleRuleMaximum,
		oracleRuleExclusiveMaximum, oracleRuleMultipleOf, oracleRuleFormat,
		oracleRuleMinLength, oracleRuleMaxLength, oracleRulePattern:
		return scalarFaultAttemptAtRank(parent, fault, rank, s)
	case oracleRuleRequired:
		if rank > 0 {
			return nil, false, true, nil
		}

		derivative, _, err := findRequiredDerivative(parent, fault, s)

		return derivative, true, false, err
	case oracleRuleAdditionalProperties:
		return additionalPropertyFaultAttemptAtRank(parent, fault, rank, s)
	case oracleRuleMinItems, oracleRuleMaxItems:
		return arrayCountFaultAttemptAtRank(parent, fault, rank, s)
	case oracleRuleMinProperties, oracleRuleMaxProperties:
		return objectCountFaultAttemptAtRank(parent, fault, rank, s)
	default:
		if rank > 0 {
			return nil, false, true, nil
		}

		derivative, err := applyNonCompositionFault(parent, fault, s)
		if errors.Is(err, errFaultNotFound) {
			return nil, true, false, nil
		}

		return derivative, derivative != nil, false, err
	}
}

// scalarFaultAttemptAtRank obtains one replacement from the complete row conjunction.
//
//nolint:cyclop // Scalar families share one complete-row adapter.
func scalarFaultAttemptAtRank(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	var (
		derivative *jsonValue
		attempted  bool
		observed   uint64
	)

	root := s.model.root
	requirements := copyPlanRequirements(fault.requirements)

	if _, occurrence, found := resolveFaultValueContainer(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence,
	); found {
		for path := range matchingValuePathSequence(
			parent, fault.obligation.occurrence.instanceTemplate,
		) {
			value := valueAtPath(parent, path)
			if value != nil {
				requirements = appendPlanRequirements(
					requirements,
					kindRequirement(occurrence, value.kind),
				)
			}

			break
		}
	}

	if fault.obligation.rule == oracleRuleEnum {
		root = cloneWithoutFaultRule(
			s.model.root,
			s.model.root.occurrence,
			fault.obligation.occurrence,
			fault.obligation.rule,
		)
	}

	candidateRoot := cloneWithoutFaultRule(
		s.model.root,
		s.model.root.occurrence,
		fault.obligation.occurrence,
		fault.obligation.rule,
	)

	wanted := uint64(0)
	if _, exists := activeEnumValueAtRank(
		candidateRoot, s.model.root.occurrence, requirements, &wanted,
	); exists {
		return scalarEnumFaultAttemptAtRank(parent, fault, candidateRoot, requirements, rank, s)
	}

	stringFault := fault.obligation.rule == oracleRuleMinLength ||
		fault.obligation.rule == oracleRuleMaxLength || fault.obligation.rule == oracleRulePattern
	if fault.obligation.rule == oracleRuleFormat {
		target, _, found := resolveExactFaultTarget(
			s.model.root, s.model.root.occurrence, fault.obligation.occurrence,
		)
		stringFault = found && !formatHasNumericSemantics(target.format)
	}

	context := rowSearchContext{scalarFault: &fault}
	if fault.obligation.rule == oracleRuleEnum || stringFault &&
		fault.obligation.occurrence.instanceTemplate != "#" {
		root = candidateRoot
		context.scalarFault = nil
	}

	stopped, err := s.walkNode(
		root,
		s.model.root.occurrence,
		requirements,
		context,
		func(row *jsonValue) (bool, error) {
			for path := range matchingValuePathSequence(
				row, fault.obligation.occurrence.instanceTemplate,
			) {
				candidate := valueAtPath(row, path)
				if candidate == nil {
					continue
				}

				if observed < rank {
					observed++

					continue
				}

				selected, matched, selectErr := firstReplacementDerivative(
					parent, fault, singleJSONValueSource(candidate), s.model, s,
				)
				if selectErr != nil {
					return false, selectErr
				}

				attempted = true

				if matched {
					derivative = selected
				}

				return true, nil
			}

			return false, nil
		},
	)
	if err != nil {
		return nil, false, false, err
	}

	if stopped {
		return derivative, attempted, false, nil
	}

	return nil, false, true, nil
}

// objectCountFaultAttemptAtRank attempts one complete directed object replacement.
//
//nolint:cyclop // Exact count decoding and one row attempt share this boundary.
func objectCountFaultAttemptAtRank(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	node, _, found := resolveExactFaultTarget(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence,
	)
	if !found {
		return nil, false, true, nil
	}

	bound := node.minProperties
	direction := int64(-1)

	if fault.obligation.rule == oracleRuleMaxProperties {
		bound = node.maxProperties
		direction = 1
	}

	if bound == nil || bound.number == nil {
		return nil, false, false, errors.New("schematest: object count fault has no bound")
	}

	one, err := parseExactNumber("1")
	if err != nil {
		return nil, false, false, err
	}

	desired, err := addSignedExactNumbers(bound.number, one, direction)
	if err != nil {
		return nil, false, false, err
	}

	if desired.numerator.Sign() < 0 {
		return nil, false, true, nil
	}

	count, fits, err := exactCountUint64(&exactCount{number: desired})
	if err != nil {
		return nil, false, false, err
	}

	root := cloneWithoutFaultRule(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence, fault.obligation.rule,
	)
	if !fits || count > uint64(^uint(0)>>1) {
		return oversizedObjectFaultAttempt(parent, fault, root, desired, rank, s)
	}

	if fault.obligation.rule == oracleRuleMaxProperties {
		path, exists := matchingValuePathAt(parent, fault.obligation.occurrence.instanceTemplate, 0)
		if !exists {
			return nil, false, true, nil
		}

		current := valueAtPath(parent, path)
		if current == nil || current.kind != jsonObject || uint64(len(current.object)) >= count {
			return nil, false, true, nil
		}

		container, occurrence, containerFound := resolveFaultContainer(
			root, s.model.root.occurrence, fault.obligation.occurrence, jsonObject,
		)
		if !containerFound {
			return nil, false, true, nil
		}

		return objectGrowthFaultAttemptAtRank(
			parent, path, current, fault, container, occurrence, nil,
			int(count-uint64(len(current.object))), rank, s,
		)
	}

	directed := fault
	directed.requirements = appendPlanRequirements(
		copyPlanRequirements(fault.requirements),
		requirement{
			tag: requirementExactCount, occurrence: fault.obligation.occurrence,
			count: &exactCount{number: desired},
		},
	)

	decoder, ok := newDirectRankTupleDecoder(directRowRankDimensions, rank)
	if !ok {
		return nil, false, true, nil
	}

	rowRank, _ := decoder.Next()
	pathRank, _ := decoder.Next()
	conjunction := rowSchemaConjunction{sources: []rowSchemaSource{{
		node: root, occurrence: s.model.root.occurrence,
	}}}

	row, exists, usable, finiteSize, err := s.rowConjunctionValueAt(
		conjunction, directed.requirements, rowSearchContext{}, rowRank,
	)
	if err != nil {
		return nil, false, false, err
	}

	if !exists || !usable || row == nil {
		return nil, false, finiteSize > 0 && rowRank >= finiteSize && pathRank == 0, nil
	}

	path, exists := matchingValuePathAt(row, fault.obligation.occurrence.instanceTemplate, pathRank)
	if !exists {
		return nil, false, false, nil
	}

	replacement := valueAtPath(row, path)

	current := valueAtPath(parent, path)
	if replacement == nil || replacement.kind != jsonObject || current == nil || current.kind != jsonObject {
		return nil, false, false, nil
	}

	candidate, _, err := tryObjectReplacement(parent, path, current, replacement, fault, s)

	return candidate, true, false, err
}

// oversizedObjectFaultAttempt retains an exact monotonic member cursor. It
// shares the projected member/value decoder used by ordinary object growth.
//
//nolint:cyclop,gocognit // Exact count, member, name, and value state form one machine.
func oversizedObjectFaultAttempt(
	parent *jsonValue,
	fault faultProgram,
	root *schemaNode,
	desired *exactNumber,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	container, occurrence, found := resolveFaultContainer(
		root, s.model.root.occurrence, fault.obligation.occurrence, jsonObject,
	)
	if !found {
		return nil, false, true, nil
	}

	path, exists := matchingValuePathAt(parent, fault.obligation.occurrence.instanceTemplate, 0)
	if !exists {
		return nil, false, true, nil
	}

	current := valueAtPath(parent, path)
	if current == nil || current.kind != jsonObject {
		return nil, false, true, nil
	}

	view, exists, err := rowProjectionAt(container, occurrence, fault.requirements, 0)
	if err != nil || !exists {
		return nil, false, !exists, err
	}

	active, err := view.appendBranchRequirements(copyPlanRequirements(fault.requirements), s.assign)
	if err != nil {
		return nil, false, false, err
	}

	shape, err := newRowProjectedObject(view, active, occurrence)
	if err != nil {
		return nil, false, false, err
	}

	working, err := cloneJSONValue(current)
	if err != nil {
		return nil, false, false, err
	}

	if assignErr := s.assign(); assignErr != nil { // exact member count
		return nil, false, false, assignErr
	}

	currentCount, err := parseExactNumber(strconv.Itoa(len(current.object)))
	if err != nil {
		return nil, false, false, err
	}

	one, err := parseExactNumber("1")
	if err != nil {
		return nil, false, false, err
	}

	memberRank := rank

	for {
		comparison, compareErr := currentCount.compare(desired)
		if compareErr != nil {
			return nil, false, false, compareErr
		}

		if comparison >= 0 {
			return nil, false, false, errors.New("schematest: oversized object unexpectedly materialized")
		}

		member, memberExists, memberErr := objectMutationMemberAtRank(
			shape, working, nil, active, memberRank,
		)
		if memberErr != nil {
			return nil, false, false, memberErr
		}

		if memberRank == ^uint64(0) {
			return nil, false, false, errors.New("schematest: object member rank overflow")
		}

		memberRank++

		if !memberExists {
			continue
		}

		if assignErr := s.assign(); assignErr != nil { // selected name
			return nil, false, false, assignErr
		}

		value, valueExists, usable, _, valueErr := s.rowConjunctionValueAt(
			member.schemas, active, rowSearchContext{}, rank,
		)
		if valueErr != nil {
			return nil, false, false, valueErr
		}

		if !valueExists || !usable {
			return nil, false, false, nil
		}

		working.object[member.name] = value

		currentCount, err = addExactNumbers(currentCount, one)
		if err != nil {
			return nil, false, false, err
		}
	}
}

// arrayCountFaultAttemptAtRank preserves exact authored counts through a lazy edit cursor.
//
//nolint:cyclop // Exact count decoding and host-safe dispatch share this boundary.
func arrayCountFaultAttemptAtRank(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	node, _, found := resolveExactFaultTarget(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence,
	)
	if !found {
		return nil, false, true, nil
	}

	bound := node.minItems
	direction := int64(-1)

	if fault.obligation.rule == oracleRuleMaxItems {
		bound = node.maxItems
		direction = 1
	}

	if bound == nil || bound.number == nil {
		return nil, false, false, errors.New("schematest: array count fault has no bound")
	}

	one, err := parseExactNumber("1")
	if err != nil {
		return nil, false, false, err
	}

	desired, err := addSignedExactNumbers(bound.number, one, direction)
	if err != nil {
		return nil, false, false, err
	}

	if desired.numerator.Sign() < 0 {
		return nil, false, true, nil
	}

	count, fits, err := exactCountUint64(&exactCount{number: desired})
	if err != nil {
		return nil, false, false, err
	}

	if !fits || count > uint64(^uint(0)>>1) {
		return oversizedArrayFaultAttempt(parent, fault, desired, rank, s)
	}

	return representableArrayFaultAttemptAtRank(parent, fault, int(count), rank, s)
}

// oversizedArrayFaultAttempt retains an exact monotonic length cursor. Every
// charged step precedes a selected length, index, or item-value assignment.
//
//nolint:cyclop // Exact count, index, and item-value state form one machine.
func oversizedArrayFaultAttempt(
	parent *jsonValue,
	fault faultProgram,
	desired *exactNumber,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	container, occurrence, found := resolveFaultContainer(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence, jsonArray,
	)
	if !found {
		return nil, false, true, nil
	}

	path, exists := matchingValuePathAt(parent, fault.obligation.occurrence.instanceTemplate, 0)
	if !exists {
		return nil, false, true, nil
	}

	current := valueAtPath(parent, path)
	if current == nil || current.kind != jsonArray {
		return nil, false, true, nil
	}

	view, exists, err := rowProjectionAt(container, occurrence, fault.requirements, 0)
	if err != nil || !exists {
		return nil, false, !exists, err
	}

	active, err := view.appendBranchRequirements(copyPlanRequirements(fault.requirements), s.assign)
	if err != nil {
		return nil, false, false, err
	}

	if assignErr := s.assign(); assignErr != nil { // exact length
		return nil, false, false, assignErr
	}

	currentCount, err := parseExactNumber(strconv.Itoa(len(current.array)))
	if err != nil {
		return nil, false, false, err
	}

	items := rowProjectedArrayItems(view, active)

	one, err := parseExactNumber("1")
	if err != nil {
		return nil, false, false, err
	}

	for {
		comparison, compareErr := currentCount.compare(desired)
		if compareErr != nil {
			return nil, false, false, compareErr
		}

		if comparison >= 0 {
			return nil, false, false, errors.New("schematest: oversized array unexpectedly materialized")
		}

		if assignErr := s.assign(); assignErr != nil { // selected index
			return nil, false, false, assignErr
		}

		_, valueExists, usable, _, valueErr := s.rowConjunctionValueAt(
			items, active, rowSearchContext{}, rank,
		)
		if valueErr != nil {
			return nil, false, false, valueErr
		}

		if !valueExists || !usable {
			return nil, false, false, nil
		}

		currentCount, err = addExactNumbers(currentCount, one)
		if err != nil {
			return nil, false, false, err
		}
	}
}

// representableArrayFaultAttemptAtRank attempts one authored or parent-relative array edit.
//
//nolint:cyclop // Authored, deletion, and insertion families share one ranked adapter.
func representableArrayFaultAttemptAtRank(
	parent *jsonValue,
	fault faultProgram,
	desired int,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	container, occurrence, found := resolveFaultContainer(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence, jsonArray,
	)
	if !found {
		return nil, false, true, nil
	}

	path, exists := matchingValuePathAt(parent, fault.obligation.occurrence.instanceTemplate, 0)
	if !exists {
		return nil, false, true, nil
	}

	current := valueAtPath(parent, path)
	if current == nil || current.kind != jsonArray {
		return nil, false, true, nil
	}

	authoredCount, err := authoredArrayFaultCandidateCount(
		container, occurrence, fault.requirements, desired,
	)
	if err != nil {
		return nil, false, false, err
	}

	if rank < authoredCount {
		return authoredArrayFaultAttemptAtRank(
			parent, path, fault, container, occurrence, desired, rank, s,
		)
	}

	rank -= authoredCount

	if len(current.array) > desired {
		removed := len(current.array) - desired

		count := saturatedBinomial(uint64(len(current.array)), uint64(removed))
		if rank >= count {
			return nil, false, true, nil
		}

		indexes, exists := arrayCombinationAt(len(current.array), removed, rank)
		if !exists {
			return nil, false, false, errors.New("schematest: array deletion rank disappeared")
		}

		values := make([]*jsonValue, 0, desired)

		removedIndex := 0
		for index, value := range current.array {
			if removedIndex < len(indexes) && indexes[removedIndex] == index {
				removedIndex++

				continue
			}

			values = append(values, value)
		}

		candidate, matched, err := tryArrayCountCandidate(
			parent, path, values, arrayEditCharges{indexes: pathCopyInts(indexes)}, fault, s,
		)

		return candidate, matched, false, err
	}

	if len(current.array) == desired {
		return nil, false, true, nil
	}

	return arrayInsertionFaultAttemptAtRank(
		parent, path, current, fault, container, occurrence, desired, rank, s,
	)
}

func authoredArrayFaultCandidateCount(
	container *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	desired int,
) (uint64, error) {
	projectionCount, err := rowProjectionNodeCount(container, occurrence, requirements)
	if err != nil {
		return 0, err
	}

	var count uint64

	for rank := uint64(0); rank < projectionCount; rank++ {
		view, exists, err := rowProjectionAt(container, occurrence, requirements, rank)
		if err != nil {
			return 0, err
		}

		if !exists {
			continue
		}

		err = view.eachDirectValue(func(_ rowSchemaSource, value *jsonValue) bool {
			if value.kind == jsonArray && len(value.array) == desired && count < ^uint64(0) {
				count++
			}

			return true
		})
		if err != nil {
			return 0, err
		}
	}

	return count, nil
}

//nolint:cyclop // Projection and authored-value selection share one attempt.
func authoredArrayFaultAttemptAtRank(
	parent *jsonValue,
	path []string,
	fault faultProgram,
	container *schemaNode,
	occurrence schemaOccurrence,
	desired int,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	cursor := newRowProjectionCursor(container, occurrence, fault.requirements)
	defer cursor.Close()

	for {
		view, exists, err := cursor.Next()
		if err != nil || !exists {
			return nil, false, !exists, err
		}

		active, err := view.appendBranchRequirements(copyPlanRequirements(fault.requirements), s.assign)
		if err != nil {
			return nil, false, false, err
		}

		if !rowProjectionAcceptsKind(view, jsonArray) ||
			!rowProjectionRequirementsAcceptKind(view, active, jsonArray) {
			continue
		}

		var selected *jsonValue

		err = view.eachDirectValue(func(_ rowSchemaSource, value *jsonValue) bool {
			if value.kind != jsonArray || len(value.array) != desired {
				return true
			}

			if rank > 0 {
				rank--

				return true
			}

			selected = value

			return false
		})
		if err != nil {
			return nil, false, false, err
		}

		if selected == nil {
			continue
		}

		indexes := make([]int, len(selected.array))
		for index := range indexes {
			indexes[index] = index
		}

		candidate, matched, err := tryArrayCountCandidate(
			parent,
			path,
			selected.array,
			arrayEditCharges{indexes: indexes, itemValues: len(indexes)},
			fault,
			s,
		)

		return candidate, matched, false, err
	}
}

//nolint:cyclop // Projection, layout, and item ranks form one insertion attempt.
func arrayInsertionFaultAttemptAtRank(
	parent *jsonValue,
	path []string,
	current *jsonValue,
	fault faultProgram,
	container *schemaNode,
	occurrence schemaOccurrence,
	desired int,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	insertions := desired - len(current.array)

	projectionCount, err := rowProjectionNodeCount(container, occurrence, fault.requirements)
	if err != nil {
		return nil, false, false, err
	}

	layoutCount := saturatedBinomial(uint64(desired), uint64(insertions))

	frontier, err := newRankProductCursor(arrayFaultProductDimensions)
	if err != nil {
		return nil, false, false, err
	}

	if setErr := frontier.SetFinite(0, projectionCount); setErr != nil {
		return nil, false, false, setErr
	}

	if setErr := frontier.SetFinite(1, layoutCount); setErr != nil {
		return nil, false, false, setErr
	}

	var ranks []uint64

	for currentRank := uint64(0); currentRank <= rank; currentRank++ {
		var exists bool

		ranks, exists = frontier.Next()
		if !exists {
			return nil, false, true, nil
		}
	}

	view, exists, err := rowProjectionAt(container, occurrence, fault.requirements, ranks[0])
	if err != nil || !exists {
		return nil, false, !exists, err
	}

	active, err := view.appendBranchRequirements(copyPlanRequirements(fault.requirements), s.assign)
	if err != nil {
		return nil, false, false, err
	}

	if assignErr := s.assign(); assignErr != nil {
		return nil, false, false, assignErr
	}

	tupleValues, exists, _, _, err := s.rowArrayChildrenForOrdinal(
		rankedArrayStructure{view: view, active: active, length: rowArrayCount{value: uint64(insertions)}},
		active,
		rowSearchContext{},
		ranks[2],
	)
	if err != nil || !exists {
		return nil, false, false, err
	}

	indexes, exists := arrayCombinationAt(desired, insertions, ranks[1])
	if !exists {
		return nil, false, false, errors.New("schematest: array insertion layout rank disappeared")
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

		values = append(values, current.array[parentIndex])
		parentIndex++
	}

	candidate, matched, err := tryArrayCountCandidate(
		parent,
		path,
		values,
		arrayEditCharges{indexes: pathCopyInts(indexes), itemValues: len(indexes)},
		fault,
		s,
	)

	return candidate, matched, false, err
}

// additionalPropertyFaultAttemptAtRank attempts one projection/name/value tuple.
//

func additionalPropertyFaultAttemptAtRank(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	tokens, ok := rowPointerTokens(fault.obligation.occurrence.instanceTemplate)
	if !ok || len(tokens) == 0 {
		return nil, false, false, errors.New("schematest: additional-property fault has no member path")
	}

	parentPointer := pointerFromTokens(tokens[:len(tokens)-1])
	targetOccurrence := fault.obligation.occurrence
	targetOccurrence.instanceTemplate = parentPointer

	target, targetOccurrence, found := resolveExactFaultTarget(
		s.model.root, s.model.root.occurrence, targetOccurrence,
	)
	if !found {
		return nil, false, true, nil
	}

	root := cloneWithoutFaultRule(
		s.model.root, s.model.root.occurrence, targetOccurrence, oracleRuleAdditionalProperties,
	)

	container, containerOccurrence, found := resolveFaultContainer(
		root, s.model.root.occurrence, targetOccurrence, jsonObject,
	)
	if !found {
		return nil, false, true, nil
	}

	path, exists := matchingValuePathAt(parent, parentPointer, 0)
	if !exists {
		return nil, false, true, nil
	}

	object := valueAtPath(parent, path)
	if object == nil || object.kind != jsonObject {
		return nil, false, true, nil
	}

	return objectGrowthFaultAttemptAtRank(
		parent, path, object, fault, container, containerOccurrence, target, 1, rank, s,
	)
}

// objectGrowthFaultAttemptAtRank is the shared projection/key/value decoder for
// max-properties and additional-properties mutations.
//
//nolint:cyclop // Projection, key, and value are one atomic growth attempt.
func objectGrowthFaultAttemptAtRank(
	parent *jsonValue,
	path []string,
	object *jsonValue,
	fault faultProgram,
	container *schemaNode,
	containerOccurrence schemaOccurrence,
	target *schemaNode,
	additions int,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	projectionCount, err := rowProjectionNodeCount(
		container, containerOccurrence, fault.requirements,
	)
	if err != nil {
		return nil, false, false, err
	}

	if projectionCount == 0 || additions <= 0 ||
		additions > (int(^uint(0)>>1)-1)/objectGrowthRanksPerAddition {
		return nil, false, true, nil
	}

	projectionRank := rank % projectionCount

	decoder, ok := newDirectRankTupleDecoder(
		additions*objectGrowthRanksPerAddition, rank/projectionCount,
	)
	if !ok {
		return nil, false, true, nil
	}

	view, exists, err := rowProjectionAt(
		container, containerOccurrence, fault.requirements, projectionRank,
	)
	if err != nil || !exists {
		return nil, false, false, err
	}

	active, err := view.appendBranchRequirements(copyPlanRequirements(fault.requirements), s.assign)
	if err != nil {
		return nil, false, false, err
	}

	shape, err := newRowProjectedObject(view, active, containerOccurrence)
	if err != nil {
		return nil, false, false, err
	}

	working, err := cloneJSONValue(object)
	if err != nil {
		return nil, false, false, err
	}

	for addition := 0; addition < additions; addition++ {
		nameRank, _ := decoder.Next()
		valueRank, _ := decoder.Next()

		if assignErr := s.assign(); assignErr != nil {
			return nil, false, false, assignErr
		}

		member, exists, memberErr := objectMutationMemberAtRank(shape, working, target, active, nameRank)
		if memberErr != nil || !exists {
			return nil, false, false, memberErr
		}

		if target != nil {
			candidate, _, candidateErr := tryAdditionalPropertyName(
				parent, path, working, fault, target, member, active, valueRank, s,
			)

			return candidate, true, false, candidateErr
		}

		value, valueExists, usable, _, valueErr := s.rowConjunctionValueAt(
			member.schemas, active, rowSearchContext{}, valueRank,
		)
		if valueErr != nil || !valueExists || !usable {
			return nil, false, false, valueErr
		}

		working.object[member.name] = value
	}

	candidate, _, err := tryObjectReplacement(parent, path, object, working, fault, s)

	return candidate, true, false, err
}

func finiteFirstRankProductTupleAt(
	dimensions int,
	firstSize uint64,
	rank uint64,
) ([]uint64, bool, error) {
	cursor, err := newRankProductCursor(dimensions)
	if err != nil {
		return nil, false, err
	}

	if err := cursor.SetFinite(0, firstSize); err != nil {
		return nil, false, err
	}

	for current := uint64(0); ; current++ {
		tuple, exists := cursor.Next()
		if !exists {
			return nil, false, nil
		}

		if current == rank {
			return append([]uint64(nil), tuple...), true, nil
		}
	}
}

// scalarEnumFaultAttemptAtRank decodes one finite whole-row enum replacement.
func scalarEnumFaultAttemptAtRank(
	parent *jsonValue,
	fault faultProgram,
	root *schemaNode,
	requirements []requirement,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	model := *s.model
	model.root = root

	var observed uint64

	for candidateRank := uint64(0); ; candidateRank++ {
		wanted := candidateRank

		row, exists := activeEnumValueAtRank(root, s.model.root.occurrence, requirements, &wanted)
		if !exists {
			return nil, false, true, nil
		}

		if err := s.assign(); err != nil {
			return nil, false, false, err
		}

		result := evaluate(&model, row)
		if result.err != nil {
			return nil, false, false, fmt.Errorf("evaluate scalar enum fault row: %w", result.err)
		}

		if !result.valid {
			continue
		}

		for path := range matchingValuePathSequence(row, fault.obligation.occurrence.instanceTemplate) {
			candidate := valueAtPath(row, path)
			if candidate == nil {
				continue
			}

			if observed < rank {
				observed++

				continue
			}

			derivative, _, err := firstReplacementDerivative(
				parent, fault, singleJSONValueSource(candidate), s.model, s,
			)

			return derivative, true, false, err
		}

		if candidateRank == ^uint64(0) {
			return nil, false, false, errors.New("schematest: scalar enum rank overflow")
		}
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

//nolint:cyclop // Authored enums and canonical kinds share one ranked attempt.
func rootTypeFaultAttemptAtRank(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	root := cloneWithoutFaultRule(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence, oracleRuleType,
	)

	target, _, found := resolveExactFaultTarget(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence,
	)
	if !found {
		return nil, false, true, nil
	}

	for enumRank := uint64(0); ; enumRank++ {
		wanted := enumRank

		candidate, exists := activeEnumValueAtRank(
			root, s.model.root.occurrence, fault.requirements, &wanted,
		)
		if !exists {
			break
		}

		if nodeAcceptsKindForTarget(target, candidate.kind) {
			continue
		}

		if rank > 0 {
			rank--

			continue
		}

		derivative, _, err := firstReplacementDerivative(
			parent, fault, singleJSONValueSource(candidate), s.model, s,
		)

		return derivative, true, false, err
	}

	for _, kind := range canonicalJSONKinds() {
		if nodeAcceptsKindForTarget(target, kind) {
			continue
		}

		var candidate *jsonValue

		err := walkCanonicalKindWitnesses(kind, func(value *jsonValue) bool {
			if rank > 0 {
				rank--

				return true
			}

			candidate = value

			return false
		})
		if err != nil {
			return nil, false, false, err
		}

		if candidate == nil {
			continue
		}

		derivative, _, err := firstReplacementDerivative(
			parent, fault, singleJSONValueSource(candidate), s.model, s,
		)

		return derivative, true, false, err
	}

	return nil, false, true, nil
}

func activeEnumValueAtRank(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	rank *uint64,
) (*jsonValue, bool) {
	for _, member := range node.enum {
		if *rank == 0 {
			return member.value, true
		}

		*rank--
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child, occurrence, occurrence.usePointer+"/allOf/"+itoa(index), occurrence.instanceTemplate,
		)
		if value, found := activeEnumValueAtRank(child, childOccurrence, requirements, rank); found {
			return value, true
		}
	}

	states, constrained := rowCompositionTruthStates(requirements, occurrence, oracleRuleAnyOf, len(node.anyOf))
	for index, child := range node.anyOf {
		if constrained && !states[index] {
			continue
		}

		childOccurrence := rebasePlanOccurrence(
			child, occurrence, occurrence.usePointer+"/anyOf/"+itoa(index), occurrence.instanceTemplate,
		)
		if value, found := activeEnumValueAtRank(child, childOccurrence, requirements, rank); found {
			return value, true
		}
	}

	return nil, false
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
	root := s.model.root
	if fault.obligation.rule != oracleRuleFormat {
		root = cloneWithoutFaultRule(
			s.model.root, s.model.root.occurrence, fault.obligation.occurrence, fault.obligation.rule,
		)
	}

	container, occurrence, found := resolveFaultContainer(
		root, s.model.root.occurrence, fault.obligation.occurrence, jsonNumber,
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
			for path := range matchingValuePathSequence(candidateRow, fault.obligation.occurrence.instanceTemplate) {
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
	for path := range matchingValuePathSequence(parent, fault.obligation.occurrence.instanceTemplate) {
		concreteFault := concretizeFaultAtPath(fault, path)

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

			matched, matchErr := derivativeMatchesFault(model, derivative, concreteFault)
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

//nolint:cyclop // Obligation and authoritative closure identities are concretized together.
func concretizeFaultAtPath(fault faultProgram, path []string) faultProgram {
	selected := pointerFromTokens(path)
	templateTokens, templateOK := rowPointerTokens(fault.obligation.occurrence.instanceTemplate)

	selectedTokens, selectedOK := rowPointerTokens(selected)
	if !templateOK || !selectedOK || len(templateTokens) != len(selectedTokens) {
		return fault
	}

	concretize := func(identity failureIdentity) failureIdentity {
		tokens, ok := rowPointerTokens(identity.occurrence.instanceTemplate)
		if !ok {
			return identity
		}

		for index := range tokens {
			if tokens[index] == "*" && index < len(templateTokens) && templateTokens[index] == "*" {
				tokens[index] = selectedTokens[index]
			}
		}

		identity.occurrence.instanceTemplate = pointerFromTokens(tokens)
		identity.occurrence.structured = nil

		return identity
	}

	fault.obligation.ruleIdentity = concretize(fault.obligation.ruleIdentity)

	fault.expected = append(faultClosure(nil), fault.expected...)
	for index := range fault.expected {
		identity := cloneEvaluationRecordIdentity(fault.expected[index])
		for tokenIndex := range identity.occurrence.instance.tokens {
			if identity.occurrence.instance.tokens[tokenIndex] == "*" &&
				tokenIndex < len(templateTokens) && templateTokens[tokenIndex] == "*" {
				identity.occurrence.instance.tokens[tokenIndex] = selectedTokens[tokenIndex]
			}
		}

		fault.expected[index] = identity
	}

	return fault
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
		return nil, false, nil
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

	if desired > uint64(^uint(0)>>1) {
		return nil, false, nil
	}

	container, containerOccurrence, found := resolveFaultContainer(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence, jsonArray,
	)
	if !found {
		return nil, false, nil
	}

	for path := range matchingValuePathSequence(parent, fault.obligation.occurrence.instanceTemplate) {
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

			indexes := make([]int, len(value.array))
			for index := range indexes {
				indexes[index] = index
			}

			derivative, _, candidateErr = tryArrayCountCandidate(
				parent, path, value.array, arrayEditCharges{indexes: indexes, itemValues: len(indexes)}, fault, s,
			)

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

		derivative, matched, err := tryArrayCountCandidate(
			parent, path, values, arrayEditCharges{indexes: pathCopyInts(indexes)}, fault, s,
		)
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

		derivative, matched, err := tryArrayCountCandidate(
			parent,
			path,
			values,
			arrayEditCharges{indexes: pathCopyInts(indexes), itemValues: len(indexes)},
			fault,
			s,
		)
		if err != nil || matched {
			return derivative, matched, err
		}
	}
}

// arrayEditCharges identifies the selected atomic coordinates and item witnesses.
type arrayEditCharges struct {
	indexes    []int
	itemValues int
}

// tryArrayCountCandidate charges the retry, length edit, selected indexes, and
// item values before installing one independently cloned complete array.
//
//nolint:cyclop // Each atomic assignment remains explicit at the apply boundary.
func tryArrayCountCandidate(
	parent *jsonValue,
	path []string,
	values []*jsonValue,
	charges arrayEditCharges,
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

	for range charges.indexes {
		if err := s.assign(); err != nil {
			return nil, false, err
		}
	}

	for range charges.itemValues {
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

	matched, err := derivativeMatchesFault(s.model, candidate, concretizeFaultAtPath(fault, path))
	if err != nil || !matched {
		return nil, false, err
	}

	return candidate, true, nil
}

func pathCopyInts(values []int) []int {
	return append([]int(nil), values...)
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
//nolint:cyclop,gocognit,gocyclo // One fixed product decodes path, projection, key, and value ranks.
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

	pathCount := matchingValuePathCount(parent, fault.obligation.occurrence.instanceTemplate)
	if pathCount == 0 {
		return nil, false, nil
	}

	applicable := false

	for path := range matchingValuePathSequence(parent, fault.obligation.occurrence.instanceTemplate) {
		current := valueAtPath(parent, path)
		if current != nil && current.kind == jsonObject && uint64(len(current.object))+1 == desired {
			applicable = true

			break
		}
	}

	if !applicable {
		return nil, false, nil
	}

	frontier, err := newRankProductCursor(objectFaultProductDimensions)
	if err != nil {
		return nil, false, err
	}

	if err := frontier.SetFinite(0, pathCount); err != nil {
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

		path, exists := matchingValuePathAt(
			parent, fault.obligation.occurrence.instanceTemplate, ranks[0],
		)
		if !exists {
			return nil, false, errors.New("schematest: object occurrence rank disappeared")
		}

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

		member, memberExists, memberErr := objectMutationMemberAtRank(
			shape, current, nil, active, ranks[2],
		)
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

func objectMutationMemberAtRank(
	shape *rowProjectedObject,
	current *jsonValue,
	target *schemaNode,
	requirements []requirement,
	rank uint64,
) (rowMember, bool, error) {
	if rank < uint64(len(shape.members)) {
		member := shape.members[rank]
		if _, present := current.object[member.name]; present {
			return rowMember{}, false, nil
		}

		if target != nil {
			if _, declared := target.properties[member.name]; declared {
				return rowMember{}, false, nil
			}
		}

		return member, true, nil
	}

	if !shape.allowsExtra {
		return rowMember{}, false, nil
	}

	name := freshObjectMutationName(rank - uint64(len(shape.members)))
	if _, present := current.object[name]; present {
		return rowMember{}, false, nil
	}

	if target != nil {
		if _, declared := target.properties[name]; declared {
			return rowMember{}, false, nil
		}
	}

	return projectedAdditionalMember(shape, name, requirements)
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

			for path := range matchingValuePathSequence(row, fault.obligation.occurrence.instanceTemplate) {
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

	matched, err := derivativeMatchesFault(s.model, candidate, concretizeFaultAtPath(fault, path))
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
	for path := range matchingValuePathSequence(parent, parentPointer) {
		object := valueAtPath(parent, path)
		if object == nil || object.kind != jsonObject {
			continue
		}

		for range 3 {
			if err := s.assign(); err != nil {
				return nil, false, err
			}
		}

		candidate, err := cloneJSONValue(parent)
		if err != nil {
			return nil, false, err
		}

		object = valueAtPath(candidate, path)
		delete(object.object, name)

		matched, err := derivativeMatchesFault(
			s.model, candidate, concretizeFaultAtPath(fault, append(pathCopy(path), name)),
		)
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
		parent, parentPointer, fault, target, container, containerOccurrence, s,
	)
}

// searchAdditionalPropertyNames diagonally enumerates occurrence, projection,
// fresh-name, and value ranks without retaining a key or value corpus.
//
//nolint:cyclop // One product decodes occurrence, projection, name, and value ranks.
func searchAdditionalPropertyNames(
	parent *jsonValue,
	instanceTemplate string,
	fault faultProgram,
	target *schemaNode,
	container *schemaNode,
	containerOccurrence schemaOccurrence,
	s *search,
) (*jsonValue, bool, error) {
	pathCount := matchingValuePathCount(parent, instanceTemplate)
	if pathCount == 0 {
		return nil, false, nil
	}

	projectionCount, err := rowProjectionNodeCount(container, containerOccurrence, fault.requirements)
	if err != nil {
		return nil, false, err
	}

	frontier, err := newRankProductCursor(objectFaultProductDimensions)
	if err != nil {
		return nil, false, err
	}

	if err := frontier.SetFinite(0, pathCount); err != nil {
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

		path, exists := matchingValuePathAt(parent, instanceTemplate, ranks[0])
		if !exists {
			return nil, false, errors.New("schematest: additional-property occurrence rank disappeared")
		}

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

		member, exists, err := objectMutationMemberAtRank(
			shape, object, target, active, ranks[2],
		)
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

// tryAdditionalPropertyName searches one charged canonical name's complete active value requirements.
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

	matched, err := derivativeMatchesFault(
		s.model, candidate, concretizeFaultAtPath(fault, append(pathCopy(path), name)),
	)
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

func derivativeHasClosure(model *schemaModel, derivative *jsonValue, closure faultClosure) (bool, error) {
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

// matchingValuePathSequence yields concrete occurrences in canonical order while
// retaining only the recursive cursor stack and the current path.
func matchingValuePathSequence(root *jsonValue, template string) iter.Seq[[]string] {
	return func(yield func([]string) bool) {
		tokens, ok := rowPointerTokens(template)
		if !ok {
			return
		}

		walkMatchingValuePaths(root, tokens, nil, yield)
	}
}

func matchingValuePathCount(root *jsonValue, template string) uint64 {
	var count uint64
	for range matchingValuePathSequence(root, template) {
		if count == ^uint64(0) {
			return count
		}

		count++
	}

	return count
}

func matchingValuePathAt(root *jsonValue, template string, rank uint64) ([]string, bool) {
	for path := range matchingValuePathSequence(root, template) {
		if rank == 0 {
			return path, true
		}

		rank--
	}

	return nil, false
}

//nolint:cyclop // Pointer completion and two container kinds form one traversal.
func walkMatchingValuePaths(
	value *jsonValue,
	tokens, path []string,
	yield func([]string) bool,
) bool {
	if len(tokens) == 0 {
		return yield(pathCopy(path))
	}

	if value == nil {
		return true
	}

	token := tokens[0]

	switch value.kind {
	case jsonArray:
		if token == "*" {
			for index, child := range value.array {
				if !walkMatchingValuePaths(child, tokens[1:], append(path, strconv.Itoa(index)), yield) {
					return false
				}
			}

			return true
		}

		index, err := strconv.Atoi(token)
		if err == nil && index >= 0 && index < len(value.array) {
			return walkMatchingValuePaths(value.array[index], tokens[1:], append(path, token), yield)
		}
	case jsonObject:
		if token == "*" {
			for _, name := range sortedObjectNames(value.object) {
				if !walkMatchingValuePaths(value.object[name], tokens[1:], append(path, name), yield) {
					return false
				}
			}

			return true
		}

		if child, exists := value.object[token]; exists {
			return walkMatchingValuePaths(child, tokens[1:], append(path, token), yield)
		}
	}

	return true
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
