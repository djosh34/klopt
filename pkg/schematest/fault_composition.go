package schematest

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
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

// compositionFaultAttemptAtRank performs at most one independently cloned
// aggregate mutation. A mismatch is an attempted rank, not exhaustion.
func compositionFaultAttemptAtRank(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	if parent == nil {
		return nil, false, false, errors.New("schematest: nil composition fault parent")
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
	count func() int
	at    func(int) (compositionEdit, bool)
}

// Count charges each discovered source coordinate once.
func (source compositionEditSource) Count(s *search) (int, error) {
	count := source.count()
	for range count {
		if err := s.assign(); err != nil {
			return 0, err
		}
	}

	return count, nil
}

// At charges immediately before selecting one edit coordinate.
func (source compositionEditSource) At(index int, s *search) (compositionEdit, bool, error) {
	edit, exists := source.at(index)
	if !exists {
		return compositionEdit{}, false, nil
	}

	if err := s.assign(); err != nil {
		return compositionEdit{}, false, err
	}

	return edit, true, nil
}

// compositionDirectEdits addresses the current parent's represented removals.
func compositionDirectEdits(parent *jsonValue, requirements []requirement) compositionEditSource {
	eligible := func(requirement requirement) bool {
		return !requirement.canonical && requirement.presence == requirementAbsent
	}

	return compositionEditSource{
		count: func() int {
			total := 0

			for _, requirement := range requirements {
				if eligible(requirement) {
					total += int(matchingValuePathCount(parent, requirement.occurrence.instanceTemplate))
				}
			}

			return total
		},
		at: func(wanted int) (compositionEdit, bool) {
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
					return compositionEdit{}, false
				}

				return compositionEdit{path: pathCopy(path), remove: true}, true
			}

			return compositionEdit{}, false
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

	matches, matchErr := faultFailureClosureMatches(result, fault)
	if matchErr != nil {
		return nil, false, fmt.Errorf("compare composition fault expected: %w", matchErr)
	}

	if result.valid || !matches {
		return nil, false, nil
	}

	return candidate, true, nil
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

// compositionDifference exposes deterministic leaf edits by direct ordinal.
func compositionDifference(parent, assignment *jsonValue, path []string) compositionEditSource {
	return compositionEditSource{
		count: func() int {
			return compositionDifferenceCount(parent, assignment)
		},
		at: func(index int) (compositionEdit, bool) {
			return compositionDifferenceAt(parent, assignment, path, index)
		},
	}
}

// compositionDifferenceCount returns the number of leaf edits without enumerating subsets.
//
//nolint:cyclop // Object, array, and scalar cardinalities share one recursion.
func compositionDifferenceCount(parent, assignment *jsonValue) int {
	if parent == nil || assignment == nil || parent.kind != assignment.kind {
		return 1
	}

	count := 0

	switch parent.kind {
	case jsonObject:
		for _, name := range sortedObjectNames(parent.object) {
			assigned, exists := assignment.object[name]
			if !exists {
				count++
			} else {
				count += compositionDifferenceCount(parent.object[name], assigned)
			}
		}

		for _, name := range sortedObjectNames(assignment.object) {
			if _, exists := parent.object[name]; !exists {
				count++
			}
		}
	case jsonArray:
		common := min(len(parent.array), len(assignment.array))
		for index := range common {
			count += compositionDifferenceCount(parent.array[index], assignment.array[index])
		}

		count += max(len(parent.array), len(assignment.array)) - common
	default:
		if !jsonValuesEqual(parent, assignment) {
			count = 1
		}
	}

	return count
}

// compositionDifferenceAt decodes one edit by subtree cardinalities.
//
//nolint:cyclop,gocognit // Containers share one canonical ordinal decoder.
func compositionDifferenceAt(
	parent, assignment *jsonValue,
	path []string,
	wanted int,
) (compositionEdit, bool) {
	if wanted < 0 {
		return compositionEdit{}, false
	}

	if parent == nil || assignment == nil || parent.kind != assignment.kind {
		return compositionEdit{path: pathCopy(path), replacement: assignment}, wanted == 0
	}

	switch parent.kind {
	case jsonObject:
		for _, name := range sortedObjectNames(parent.object) {
			assigned, exists := assignment.object[name]
			if !exists {
				if wanted == 0 {
					return compositionEdit{path: append(pathCopy(path), name), remove: true}, true
				}

				wanted--

				continue
			}

			count := compositionDifferenceCount(parent.object[name], assigned)
			if wanted < count {
				return compositionDifferenceAt(
					parent.object[name], assigned, append(pathCopy(path), name), wanted,
				)
			}

			wanted -= count
		}

		for _, name := range sortedObjectNames(assignment.object) {
			if _, exists := parent.object[name]; exists {
				continue
			}

			if wanted == 0 {
				return compositionEdit{
					path: append(pathCopy(path), name), replacement: assignment.object[name],
				}, true
			}

			wanted--
		}
	case jsonArray:
		common := min(len(parent.array), len(assignment.array))
		for index := range common {
			count := compositionDifferenceCount(parent.array[index], assignment.array[index])
			if wanted < count {
				return compositionDifferenceAt(
					parent.array[index], assignment.array[index],
					append(pathCopy(path), strconv.Itoa(index)), wanted,
				)
			}

			wanted -= count
		}

		for index := len(parent.array) - 1; index >= len(assignment.array); index-- {
			if wanted == 0 {
				return compositionEdit{
					path: append(pathCopy(path), strconv.Itoa(index)), remove: true,
				}, true
			}

			wanted--
		}

		for index := len(parent.array); index < len(assignment.array); index++ {
			if wanted == 0 {
				return compositionEdit{
					path:        append(pathCopy(path), strconv.Itoa(index)),
					replacement: assignment.array[index], append: true,
				}, true
			}

			wanted--
		}
	default:
		if wanted == 0 && !jsonValuesEqual(parent, assignment) {
			return compositionEdit{path: pathCopy(path), replacement: assignment}, true
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
