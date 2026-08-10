package schematest

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
)

// applyCompositionFault searches for the smallest exact aggregate edit relative
// to the regenerated parent. Rejected candidates are transient.
func applyCompositionFault(parent *jsonValue, fault faultProgram, s *search) (*jsonValue, error) {
	if parent == nil {
		return nil, errors.New("schematest: nil composition fault parent")
	}

	if s == nil || s.model == nil || s.model.root == nil {
		return nil, errors.New("schematest: composition fault application has no model")
	}

	derivative, found, err := findCompositionFaultDerivative(parent, fault, s)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, fmt.Errorf("%w: %s", errFaultNotFound, fault.obligation.String())
	}

	return derivative, nil
}

// compositionEdit is one parent-relative aggregate mutation.
type compositionEdit struct {
	path        []string
	replacement *jsonValue
	remove      bool
	append      bool
}

// findCompositionFaultDerivative uses complete-schema assignments only as a
// deterministic source of edits. It applies the smallest useful subset to the
// current parent, so unrelated parent paths are retained.
func findCompositionFaultDerivative(
	parent *jsonValue,
	fault faultProgram,
	s *search,
) (*jsonValue, bool, error) {
	derivative, found, err := applyCompositionRequirementEdits(parent, fault, s)
	if err != nil || found {
		return derivative, found, err
	}

	visit := func(value *jsonValue) (bool, error) {
		return visitCompositionEditSizes(
			compositionDifference(parent, value, nil), s,
			func(selected []compositionEdit) (bool, error) {
				candidate, matched, candidateErr := tryCompositionEdits(parent, fault, selected, s)
				if candidateErr != nil || !matched {
					return false, candidateErr
				}

				derivative = candidate

				return true, nil
			},
		)
	}

	found, err = s.walkNode(
		s.model.root,
		s.model.root.occurrence,
		fault.requirements,
		rowSearchContext{},
		visit,
	)

	return derivative, found, err
}

// compositionFaultAttemptAtRank advances one bounded direct or assignment coordinate.
//
//nolint:cyclop // Prospective and existing aggregate sources meet at one ranked attempt.
func compositionFaultAttemptAtRank(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	if parent == nil {
		return nil, false, false, errors.New("schematest: nil composition fault parent")
	}

	createdEdits, prospectiveSource, prospectiveErr := prospectiveCompositionEditsAtRank(parent, fault, rank)
	if prospectiveErr != nil {
		return nil, false, false, prospectiveErr
	}

	if prospectiveSource && len(createdEdits) > 0 {
		candidate, matched, err := tryCompositionEdits(parent, fault, createdEdits, s)
		if err != nil || matched {
			return candidate, true, false, err
		}
	}

	var (
		observed   uint64
		derivative *jsonValue
		attempted  bool
	)

	attempt := func(edits []compositionEdit) (bool, error) {
		if observed != rank {
			observed++

			return false, nil
		}

		candidate, matched, err := tryCompositionEdits(parent, fault, edits, s)
		if err != nil {
			return false, err
		}

		attempted = true

		if matched {
			derivative = candidate
		}

		return true, nil
	}

	directEdits := compositionDirectEdits(parent, fault.requirements)

	stopped, err := visitCompositionEditSizes(directEdits, s, attempt)
	if err != nil || stopped {
		return derivative, attempted, false, err
	}

	stopped, err = s.walkNode(
		s.model.root,
		s.model.root.occurrence,
		fault.requirements,
		rowSearchContext{},
		compositionAssignmentEditVisitor(parent, s, attempt),
	)
	if err != nil {
		return nil, false, false, err
	}

	if stopped {
		return derivative, attempted, false, nil
	}

	return nil, false, true, nil
}

// prospectiveCompositionSource identifies one missing wildcard container.
type prospectiveCompositionSource struct {
	path      []string
	container *jsonValue
}

// prospectiveCompositionEditsAtRank addresses canonical subsets of mutation-created paths.
//
//nolint:cyclop // Source discovery and direct subset decoding share one boundary.
func prospectiveCompositionEditsAtRank(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
) ([]compositionEdit, bool, error) {
	sourceCount, err := prospectiveCompositionSourceCount(parent, fault)
	if err != nil || sourceCount == 0 {
		return nil, false, err
	}

	subsets, finite := parentReplayMaskFiniteSize(sourceCount)
	if !finite {
		return nil, true, nil
	}

	subsetRank, tupleRank, addressed := rowSourceValueRanksAtOrdinal(subsets, rank)
	if !addressed {
		return nil, true, nil
	}

	mask, exists := parentReplayMaskAtOrdinal(sourceCount, new(big.Int).SetUint64(subsetRank))
	if !exists {
		return nil, true, nil
	}

	selected := 0

	for index := range sourceCount {
		if mask.Bit(index) == 1 {
			selected++
		}
	}

	decoder, ok := newDirectRankTupleDecoder(selected*directRowRankDimensions, tupleRank)
	if !ok {
		return nil, true, nil
	}

	edits := make([]compositionEdit, 0, selected)

	for index := range sourceCount {
		if mask.Bit(index) == 0 {
			continue
		}

		source, sourceExists, sourceErr := prospectiveCompositionSourceAt(parent, fault, index)
		if sourceErr != nil || !sourceExists {
			return nil, true, sourceErr
		}

		coordinateRank, _ := decoder.Next()
		valueRank, _ := decoder.Next()

		value, valueExists, _, valueErr := canonicalGenericValueAt(valueRank)
		if valueErr != nil || !valueExists {
			return nil, true, valueErr
		}

		edit, editExists := prospectiveCompositionEdit(source, coordinateRank, value)
		if !editExists {
			return nil, true, nil
		}

		edits = append(edits, edit)
	}

	return edits, true, nil
}

// prospectiveCompositionSourceCount counts eligible identities without retaining coordinates.
func prospectiveCompositionSourceCount(parent *jsonValue, fault faultProgram) (int, error) {
	count := 0

	for index := range fault.expected {
		_, exists, err := prospectiveCompositionSourceForIdentity(parent, fault.expected[index])
		if err != nil {
			return 0, err
		}

		if exists {
			count++
		}
	}

	return count, nil
}

// prospectiveCompositionSourceAt directly addresses one eligible identity.
func prospectiveCompositionSourceAt(
	parent *jsonValue,
	fault faultProgram,
	wanted int,
) (prospectiveCompositionSource, bool, error) {
	for index := range fault.expected {
		source, exists, err := prospectiveCompositionSourceForIdentity(parent, fault.expected[index])
		if err != nil {
			return prospectiveCompositionSource{}, false, err
		}

		if !exists {
			continue
		}

		if wanted == 0 {
			return source, true, nil
		}

		wanted--
	}

	return prospectiveCompositionSource{}, false, nil
}

// prospectiveCompositionSourceForIdentity resolves one missing wildcard container.
//
//nolint:cyclop // Identity filtering and wildcard resolution are one direct lookup.
func prospectiveCompositionSourceForIdentity(
	parent *jsonValue,
	expected evaluationRecordIdentity,
) (prospectiveCompositionSource, bool, error) {
	projected := expected.project()
	if projected.rule != oracleRuleType ||
		!strings.Contains(projected.occurrence.instanceTemplate, "*") ||
		matchingValuePathCount(parent, projected.occurrence.instanceTemplate) > 0 {
		return prospectiveCompositionSource{}, false, nil
	}

	tokens, ok := rowPointerTokens(projected.occurrence.instanceTemplate)
	if !ok {
		return prospectiveCompositionSource{}, false, errors.New("schematest: invalid prospective composition path")
	}

	wildcard := -1

	for index, token := range tokens {
		if token == "*" {
			wildcard = index

			break
		}
	}

	if wildcard < 0 || wildcard+1 != len(tokens) {
		return prospectiveCompositionSource{}, false, nil
	}

	path, exists := matchingValuePathAt(parent, pointerFromTokens(tokens[:wildcard]), 0)
	if !exists {
		return prospectiveCompositionSource{}, false, nil
	}

	container := valueAtPath(parent, path)
	if container == nil || container.kind != jsonArray && container.kind != jsonObject {
		return prospectiveCompositionSource{}, false, nil
	}

	return prospectiveCompositionSource{path: pathCopy(path), container: container}, true, nil
}

// prospectiveCompositionEdit creates one addressed insertion.
func prospectiveCompositionEdit(
	source prospectiveCompositionSource,
	coordinateRank uint64,
	value *jsonValue,
) (compositionEdit, bool) {
	switch source.container.kind {
	case jsonArray:
		if coordinateRank > 0 {
			return compositionEdit{}, false
		}

		return compositionEdit{
			path:        append(pathCopy(source.path), strconv.Itoa(len(source.container.array))),
			replacement: value, append: true,
		}, true
	case jsonObject:
		name := freshObjectMutationName(coordinateRank)
		if _, collision := source.container.object[name]; collision {
			return compositionEdit{}, false
		}

		return compositionEdit{path: append(pathCopy(source.path), name), replacement: value}, true
	default:
		return compositionEdit{}, false
	}
}

// visitCompositionEditSizes traverses edit subsets in increasing size and source order.
func visitCompositionEditSizes(
	edits compositionEditSource,
	s *search,
	visit func([]compositionEdit) (bool, error),
) (bool, error) {
	count, err := edits.Count(s)
	if err != nil {
		return false, err
	}

	for size := 1; size <= count; size++ {
		cursor := newCompositionEditSubsetCursor(edits, count, size, s)
		for {
			selected, exists, cursorErr := cursor.Next()
			if cursorErr != nil || !exists {
				if cursorErr != nil {
					return false, cursorErr
				}

				break
			}

			stopped, visitErr := visit(selected)
			if visitErr != nil || stopped {
				return stopped, visitErr
			}
		}
	}

	return false, nil
}

// compositionAssignmentEditVisitor converts each complete assignment to parent-relative edits.
func compositionAssignmentEditVisitor(
	parent *jsonValue,
	s *search,
	visit func([]compositionEdit) (bool, error),
) rowVisit {
	return func(value *jsonValue) (bool, error) {
		return visitCompositionEditSizes(compositionDifference(parent, value, nil), s, visit)
	}
}

// compositionEditSource directly addresses edits without retaining an edit corpus.
type compositionEditSource struct {
	count func(*search) (int, error)
	at    func(int, *search) (compositionEdit, bool, error)
}

// Count charges before advancing across each discovered source coordinate.
func (source compositionEditSource) Count(s *search) (int, error) {
	return source.count(s)
}

// At charges immediately before selecting one edit coordinate.
func (source compositionEditSource) At(index int, s *search) (compositionEdit, bool, error) {
	return source.at(index, s)
}

// compositionDirectEdits addresses the current parent's represented removals.
//
//nolint:cyclop // Counting and addressing use the same direct requirement filter.
func compositionDirectEdits(parent *jsonValue, requirements []requirement) compositionEditSource {
	eligible := func(requirement requirement) bool {
		return !requirement.canonical && requirement.presence == requirementAbsent
	}

	return compositionEditSource{
		count: func(s *search) (int, error) {
			total := 0

			for _, requirement := range requirements {
				if !eligible(requirement) {
					continue
				}

				for range matchingValuePathSequence(parent, requirement.occurrence.instanceTemplate) {
					if err := s.assign(); err != nil {
						return 0, err
					}

					total++
				}
			}

			return total, nil
		},
		at: func(wanted int, s *search) (compositionEdit, bool, error) {
			if wanted < 0 {
				return compositionEdit{}, false, nil
			}

			if err := s.assign(); err != nil {
				return compositionEdit{}, false, err
			}

			for _, requirement := range requirements {
				if !eligible(requirement) {
					continue
				}

				paths := matchingValuePathCount(parent, requirement.occurrence.instanceTemplate)
				if wanted >= int(paths) {
					wanted -= int(paths)

					continue
				}

				path, exists := matchingValuePathAt(
					parent, requirement.occurrence.instanceTemplate, uint64(wanted),
				)
				if !exists || len(path) == 0 || valueAtPath(parent, path) == nil {
					return compositionEdit{}, false, nil
				}

				return compositionEdit{path: pathCopy(path), remove: true}, true, nil
			}

			return compositionEdit{}, false, nil
		},
	}
}

// applyCompositionRequirementEdits applies directly represented absent-presence requirements.
func applyCompositionRequirementEdits(
	parent *jsonValue,
	fault faultProgram,
	s *search,
) (*jsonValue, bool, error) {
	var derivative *jsonValue

	found, err := visitCompositionEditSizes(
		compositionDirectEdits(parent, fault.requirements), s,
		func(selected []compositionEdit) (bool, error) {
			candidate, matched, candidateErr := tryCompositionEdits(parent, fault, selected, s)
			if candidateErr != nil || !matched {
				return false, candidateErr
			}

			derivative = candidate

			return true, nil
		},
	)

	return derivative, found, err
}

// tryCompositionEdits charges and verifies one transient edit set.
//
//nolint:cyclop // Atomic charging, application, and exact verification meet here.
func tryCompositionEdits(
	parent *jsonValue,
	fault faultProgram,
	edits []compositionEdit,
	s *search,
) (*jsonValue, bool, error) {
	if assignErr := s.assign(); assignErr != nil {
		return nil, false, assignErr
	}

	selectedFault, concretizeErr := concretizeProspectiveCompositionOccurrences(parent, fault, edits, s)
	if concretizeErr != nil {
		return nil, false, concretizeErr
	}

	candidate, copyErr := cloneJSONValue(parent)
	if copyErr != nil {
		return nil, false, fmt.Errorf("clone composition fault parent: %w", copyErr)
	}

	for _, edit := range edits {
		if chargeErr := chargeCompositionEdit(candidate, edit, s); chargeErr != nil {
			return nil, false, chargeErr
		}

		if applyErr := applyCompositionEdit(candidate, edit); applyErr != nil {
			if errors.Is(applyErr, errCompositionEditInapplicable) {
				return nil, false, nil
			}

			return nil, false, applyErr
		}
	}

	result := evaluate(s.model, candidate)
	if result.err != nil {
		return nil, false, fmt.Errorf("evaluate composition fault derivative: %w", result.err)
	}

	matches, matchErr := faultFailureClosureMatches(result, selectedFault)
	if matchErr != nil {
		return nil, false, fmt.Errorf("compare composition fault expected: %w", matchErr)
	}

	if result.valid || !matches {
		return nil, false, nil
	}

	return candidate, true, nil
}

// concretizeProspectiveCompositionOccurrences ranks paths created by the selected edit.
func concretizeProspectiveCompositionOccurrences(
	parent *jsonValue,
	fault faultProgram,
	edits []compositionEdit,
	s *search,
) (faultProgram, error) {
	selected := fault
	selected.expected = append(faultClosure(nil), fault.expected...)

	for index := range selected.expected {
		template := selected.expected[index].project().occurrence.instanceTemplate
		if !strings.Contains(template, "*") || matchingValuePathCount(parent, template) > 0 {
			continue
		}

		var createdPath []string

		for _, edit := range edits {
			path, exists := prospectiveCompositionPath(edit, template)
			if exists {
				createdPath = path

				break
			}
		}

		if createdPath == nil {
			continue
		}

		if err := s.assign(); err != nil {
			return faultProgram{}, err
		}

		identity := cloneEvaluationRecordIdentity(selected.expected[index])
		identity.occurrence.instance.tokens = pathCopy(createdPath)
		selected.expected[index] = identity
	}

	return selected, nil
}

// prospectiveCompositionPath resolves one mutation-created template without oracle output.
func prospectiveCompositionPath(edit compositionEdit, template string) ([]string, bool) {
	if edit.remove || edit.replacement == nil {
		return nil, false
	}

	tokens, ok := rowPointerTokens(template)
	if !ok || len(tokens) < len(edit.path) {
		return nil, false
	}

	for index := range edit.path {
		if tokens[index] != "*" && tokens[index] != edit.path[index] {
			return nil, false
		}
	}

	relativeTemplate := pointerFromTokens(tokens[len(edit.path):])

	relative, exists := matchingValuePathAt(edit.replacement, relativeTemplate, 0)
	if !exists {
		return nil, false
	}

	return append(pathCopy(edit.path), relative...), true
}

// chargeCompositionEdit charges every atomic part of one selected edit.
//
//nolint:cyclop // Presence, index, and replacement charges are intentionally explicit.
func chargeCompositionEdit(candidate *jsonValue, edit compositionEdit, s *search) error {
	if err := s.assign(); err != nil { // selected path
		return err
	}

	if len(edit.path) == 0 {
		return s.assign() // root replacement
	}

	parent := valueAtPath(candidate, edit.path[:len(edit.path)-1])
	if parent == nil {
		return nil
	}

	if edit.remove || edit.append {
		if err := s.assign(); err != nil { // presence
			return err
		}
	}

	if parent.kind == jsonArray {
		if err := s.assign(); err != nil { // selected index
			return err
		}
	}

	if !edit.remove {
		if err := s.assign(); err != nil { // replacement value
			return err
		}
	}

	return nil
}

// compositionDifference exposes deterministic leaf edits without retaining an edit corpus.
func compositionDifference(parent, assignment *jsonValue, path []string) compositionEditSource {
	counted := false
	count := 0

	return compositionEditSource{
		count: func(s *search) (int, error) {
			if counted {
				return count, nil
			}

			var err error

			count, err = countCompositionDifferences(parent, assignment, s)
			if err != nil {
				return 0, err
			}

			counted = true

			return count, nil
		},
		at: func(index int, s *search) (compositionEdit, bool, error) {
			if index < 0 {
				return compositionEdit{}, false, nil
			}

			if err := s.assign(); err != nil {
				return compositionEdit{}, false, err
			}

			remaining := index
			edit, exists := compositionDifferenceAt(parent, assignment, path, &remaining)

			return edit, exists, nil
		},
	}
}

// countCompositionDifferences charges before each discovered source coordinate.
//
//nolint:cyclop,gocognit // Object, array, and scalar cardinalities share one recursion.
func countCompositionDifferences(parent, assignment *jsonValue, s *search) (int, error) {
	if parent == nil || assignment == nil || parent.kind != assignment.kind {
		if err := s.assign(); err != nil {
			return 0, err
		}

		return 1, nil
	}

	count := 0

	switch parent.kind {
	case jsonObject:
		for _, name := range sortedObjectNames(parent.object) {
			assigned, exists := assignment.object[name]
			if !exists {
				if err := s.assign(); err != nil {
					return 0, err
				}

				count++

				continue
			}

			childCount, err := countCompositionDifferences(parent.object[name], assigned, s)
			if err != nil {
				return 0, err
			}

			count += childCount
		}

		for _, name := range sortedObjectNames(assignment.object) {
			if _, exists := parent.object[name]; exists {
				continue
			}

			if err := s.assign(); err != nil {
				return 0, err
			}

			count++
		}
	case jsonArray:
		common := min(len(parent.array), len(assignment.array))
		for index := range common {
			childCount, err := countCompositionDifferences(parent.array[index], assignment.array[index], s)
			if err != nil {
				return 0, err
			}

			count += childCount
		}

		for range max(len(parent.array), len(assignment.array)) - common {
			if err := s.assign(); err != nil {
				return 0, err
			}

			count++
		}
	default:
		if !jsonValuesEqual(parent, assignment) {
			if err := s.assign(); err != nil {
				return 0, err
			}

			count = 1
		}
	}

	return count, nil
}

// compositionDifferenceAt advances one bounded depth-first cursor to the requested edit.
//
//nolint:cyclop,gocognit // Containers share one canonical traversal.
func compositionDifferenceAt(
	parent, assignment *jsonValue,
	path []string,
	remaining *int,
) (compositionEdit, bool) {
	selectEdit := func(edit compositionEdit) (compositionEdit, bool) {
		if *remaining == 0 {
			return edit, true
		}

		*remaining--

		return compositionEdit{}, false
	}

	if parent == nil || assignment == nil || parent.kind != assignment.kind {
		return selectEdit(compositionEdit{path: pathCopy(path), replacement: assignment})
	}

	switch parent.kind {
	case jsonObject:
		for _, name := range sortedObjectNames(parent.object) {
			assigned, exists := assignment.object[name]
			if !exists {
				if edit, selected := selectEdit(compositionEdit{
					path: append(pathCopy(path), name), remove: true,
				}); selected {
					return edit, true
				}

				continue
			}

			if edit, selected := compositionDifferenceAt(
				parent.object[name], assigned, append(pathCopy(path), name), remaining,
			); selected {
				return edit, true
			}
		}

		for _, name := range sortedObjectNames(assignment.object) {
			if _, exists := parent.object[name]; exists {
				continue
			}

			if edit, selected := selectEdit(compositionEdit{
				path: append(pathCopy(path), name), replacement: assignment.object[name],
			}); selected {
				return edit, true
			}
		}
	case jsonArray:
		common := min(len(parent.array), len(assignment.array))
		for index := range common {
			if edit, selected := compositionDifferenceAt(
				parent.array[index], assignment.array[index],
				append(pathCopy(path), strconv.Itoa(index)), remaining,
			); selected {
				return edit, true
			}
		}

		for index := len(parent.array) - 1; index >= len(assignment.array); index-- {
			if edit, selected := selectEdit(compositionEdit{
				path: append(pathCopy(path), strconv.Itoa(index)), remove: true,
			}); selected {
				return edit, true
			}
		}

		for index := len(parent.array); index < len(assignment.array); index++ {
			if edit, selected := selectEdit(compositionEdit{
				path:        append(pathCopy(path), strconv.Itoa(index)),
				replacement: assignment.array[index], append: true,
			}); selected {
				return edit, true
			}
		}
	default:
		if !jsonValuesEqual(parent, assignment) {
			return selectEdit(compositionEdit{path: pathCopy(path), replacement: assignment})
		}
	}

	return compositionEdit{}, false
}

// compositionEditSubsetCursor retains only the current subset indexes and edits.
type compositionEditSubsetCursor struct {
	edits        compositionEditSource
	combinations *arrayCombinationCursor
	s            *search
}

// newCompositionEditSubsetCursor starts one fixed-size subset traversal.
func newCompositionEditSubsetCursor(
	edits compositionEditSource,
	count int,
	size int,
	s *search,
) *compositionEditSubsetCursor {
	return &compositionEditSubsetCursor{
		edits: edits, combinations: newArrayCombinationCursor(count, size), s: s,
	}
}

// Next selects one subset without recounting or replaying the source prefix.
func (cursor *compositionEditSubsetCursor) Next() ([]compositionEdit, bool, error) {
	indexes, exists := cursor.combinations.Next()
	if !exists {
		return nil, false, nil
	}

	selected := make([]compositionEdit, 0, len(indexes))
	for _, index := range indexes {
		edit, editExists, err := cursor.edits.At(index, cursor.s)
		if err != nil || !editExists {
			return nil, false, err
		}

		selected = append(selected, edit)
	}

	return selected, true, nil
}

// errCompositionEditInapplicable rejects a structurally incomplete edit subset.
var errCompositionEditInapplicable = errors.New("schematest: composition edit is inapplicable")

// applyCompositionEdit applies one edit to a transient parent copy.
//
//nolint:cyclop // Root, object, and array edits share one mutation boundary.
func applyCompositionEdit(root *jsonValue, edit compositionEdit) error {
	if len(edit.path) == 0 {
		if edit.remove || edit.replacement == nil {
			return errors.New("schematest: cannot remove composition root")
		}

		replacement, err := cloneJSONValue(edit.replacement)
		if err != nil {
			return err
		}

		*root = *replacement

		return nil
	}

	parent := valueAtPath(root, edit.path[:len(edit.path)-1])
	if parent == nil {
		return errors.New("schematest: composition edit parent was not found")
	}

	token := edit.path[len(edit.path)-1]
	if edit.remove {
		switch parent.kind {
		case jsonObject:
			delete(parent.object, token)
		case jsonArray:
			index, parseErr := strconv.Atoi(token)
			if parseErr != nil || index != len(parent.array)-1 {
				return errCompositionEditInapplicable
			}

			parent.array = parent.array[:index]
		default:
			return errors.New("schematest: composition removal parent is not a container")
		}

		return nil
	}

	replacement, err := cloneJSONValue(edit.replacement)
	if err != nil {
		return err
	}

	switch parent.kind {
	case jsonObject:
		parent.object[token] = replacement
	case jsonArray:
		index, parseErr := strconv.Atoi(token)
		if parseErr != nil || index < 0 {
			return errors.New("schematest: composition array edit index is invalid")
		}

		if edit.append {
			if index != len(parent.array) {
				return errCompositionEditInapplicable
			}

			parent.array = append(parent.array, replacement)
		} else {
			if index >= len(parent.array) {
				return errCompositionEditInapplicable
			}

			parent.array[index] = replacement
		}
	default:
		return errors.New("schematest: composition edit parent is not a container")
	}

	return nil
}

// pathCopy returns an independent edit path.
func pathCopy(path []string) []string {
	return append([]string(nil), path...)
}

// jsonValuesEqual compares complete internal JSON values.
func jsonValuesEqual(left, right *jsonValue) bool {
	return reflect.DeepEqual(left, right)
}

// faultNeedsCompositionSearch reports whether one fault must make an anyOf
// closure false atomically. Pure allOf branch faults use the ordinary isolated
// mutation path so unaffected branches remain untouched.
func faultNeedsCompositionSearch(fault faultProgram) bool {
	if fault.obligation.rule == oracleRuleAllOf || fault.obligation.rule == oracleRuleAnyOf {
		return true
	}

	if fault.obligation.rule != oracleRuleRequired && fault.obligation.rule != oracleRuleAdditionalProperties {
		return false
	}

	for _, failure := range fault.expected {
		if failure.rule == oracleRuleAnyOf {
			return true
		}
	}

	return false
}
