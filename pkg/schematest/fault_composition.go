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
			compositionDifference(parent, value, nil),
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

	stopped, err := visitCompositionEditSizes(directEdits, attempt)
	if err != nil || stopped {
		return derivative, attempted, false, err
	}

	stopped, err = s.walkNode(
		s.model.root,
		s.model.root.occurrence,
		fault.requirements,
		rowSearchContext{},
		compositionAssignmentEditVisitor(parent, attempt),
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
	visit func([]compositionEdit) (bool, error),
) (bool, error) {
	count := compositionEditCount(edits)
	for size := 1; size <= count; size++ {
		stopped, err := visitCompositionEditSubsets(edits, count, size, visit)
		if err != nil || stopped {
			return stopped, err
		}
	}

	return false, nil
}

// compositionAssignmentEditVisitor converts each complete assignment to parent-relative edits.
func compositionAssignmentEditVisitor(
	parent *jsonValue,
	visit func([]compositionEdit) (bool, error),
) rowVisit {
	return func(value *jsonValue) (bool, error) {
		return visitCompositionEditSizes(compositionDifference(parent, value, nil), visit)
	}
}

// compositionEditSource yields transient edits in canonical order.
type compositionEditSource func(func(compositionEdit) bool)

// compositionDirectEdits yields the current parent's directly represented removals.
func compositionDirectEdits(parent *jsonValue, requirements []requirement) compositionEditSource {
	return func(yield func(compositionEdit) bool) {
		for _, requirement := range requirements {
			if requirement.canonical || requirement.presence != requirementAbsent {
				continue
			}

			for path := range matchingValuePathSequence(parent, requirement.occurrence.instanceTemplate) {
				if len(path) == 0 || valueAtPath(parent, path) == nil {
					continue
				}

				if !yield(compositionEdit{path: pathCopy(path), remove: true}) {
					return
				}
			}
		}
	}
}

// applyCompositionRequirementEdits applies directly represented absent-presence requirements.
func applyCompositionRequirementEdits(
	parent *jsonValue,
	fault faultProgram,
	s *search,
) (*jsonValue, bool, error) {
	edits := compositionDirectEdits(parent, fault.requirements)

	count := compositionEditCount(edits)
	for size := 1; size <= count; size++ {
		var derivative *jsonValue

		found, err := visitCompositionEditSubsets(edits, count, size, func(selected []compositionEdit) (bool, error) {
			candidate, matched, candidateErr := tryCompositionEdits(parent, fault, selected, s)
			if candidateErr != nil || !matched {
				return false, candidateErr
			}

			derivative = candidate

			return true, nil
		})
		if err != nil || found {
			return derivative, found, err
		}
	}

	return nil, false, nil
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

	concreteFault, concretizeErr := concretizeCompositionFaultClosure(result, fault, s)
	if concretizeErr != nil {
		return nil, false, concretizeErr
	}

	matches, matchErr := faultFailureClosureMatches(result, concreteFault)
	if matchErr != nil {
		return nil, false, fmt.Errorf("compare composition fault expected: %w", matchErr)
	}

	if result.valid || !matches {
		return nil, false, nil
	}

	return candidate, true, nil
}

// concretizeCompositionFaultClosure selects every repeated closure-local occurrence.
//
//nolint:cyclop // Structured wildcard selection checks every identity coordinate.
func concretizeCompositionFaultClosure(
	result evaluation,
	fault faultProgram,
	s *search,
) (faultProgram, error) {
	fault.expected = append(faultClosure(nil), fault.expected...)
	for index, expected := range fault.expected {
		repeated := false
		for _, token := range expected.occurrence.instance.tokens {
			repeated = repeated || token == "*"
		}

		if !repeated {
			continue
		}

		projected := expected.project()
		selected := false

		for actual := range result.failureRecords() {
			if actual.rule != projected.rule ||
				actual.occurrence.usePointer != projected.occurrence.usePointer ||
				actual.occurrence.targetPointer != projected.occurrence.targetPointer ||
				actual.occurrence.reference != projected.occurrence.reference ||
				!instanceTemplateMatches(
					projected.occurrence.instanceTemplate, actual.occurrence.instanceTemplate,
				) {
				continue
			}

			if err := s.assign(); err != nil {
				return faultProgram{}, err
			}

			fault.expected[index] = newEvaluationRecordIdentity(actual)
			selected = true

			break
		}

		if !selected {
			return fault, nil
		}
	}

	return fault, nil
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

// compositionDifference lazily yields deterministic leaf edits between two values.
func compositionDifference(parent, assignment *jsonValue, path []string) compositionEditSource {
	return func(yield func(compositionEdit) bool) {
		yieldCompositionDifference(parent, assignment, path, yield)
	}
}

// yieldCompositionDifference emits recursive object, array, and scalar edits.
//
//nolint:cyclop,gocognit // Object, array, and scalar differences are one recursive operation.
func yieldCompositionDifference(
	parent, assignment *jsonValue,
	path []string,
	yield func(compositionEdit) bool,
) bool {
	if parent == nil || assignment == nil || parent.kind != assignment.kind {
		return yield(compositionEdit{path: pathCopy(path), replacement: assignment})
	}

	switch parent.kind {
	case jsonObject:
		for _, name := range sortedObjectNames(parent.object) {
			assigned, exists := assignment.object[name]
			if !exists {
				if !yield(compositionEdit{path: append(pathCopy(path), name), remove: true}) {
					return false
				}

				continue
			}

			if !yieldCompositionDifference(
				parent.object[name], assigned, append(pathCopy(path), name), yield,
			) {
				return false
			}
		}

		for _, name := range sortedObjectNames(assignment.object) {
			if _, exists := parent.object[name]; !exists && !yield(compositionEdit{
				path: append(pathCopy(path), name), replacement: assignment.object[name],
			}) {
				return false
			}
		}
	case jsonArray:
		common := min(len(parent.array), len(assignment.array))
		for index := 0; index < common; index++ {
			if !yieldCompositionDifference(
				parent.array[index],
				assignment.array[index],
				append(pathCopy(path), strconv.Itoa(index)),
				yield,
			) {
				return false
			}
		}

		for index := len(parent.array) - 1; index >= len(assignment.array); index-- {
			if !yield(compositionEdit{
				path: append(pathCopy(path), strconv.Itoa(index)), remove: true,
			}) {
				return false
			}
		}

		for index := len(parent.array); index < len(assignment.array); index++ {
			if !yield(compositionEdit{
				path:        append(pathCopy(path), strconv.Itoa(index)),
				replacement: assignment.array[index], append: true,
			}) {
				return false
			}
		}
	default:
		if !jsonValuesEqual(parent, assignment) {
			return yield(compositionEdit{path: pathCopy(path), replacement: assignment})
		}
	}

	return true
}

// compositionEditCount counts a lazy source without retaining its edits.
func compositionEditCount(edits compositionEditSource) int {
	count := 0

	edits(func(compositionEdit) bool {
		count++

		return true
	})

	return count
}

// visitCompositionEditSubsets visits one current fixed-size edit tuple at a time.
func visitCompositionEditSubsets(
	edits compositionEditSource,
	count int,
	size int,
	visit func([]compositionEdit) (bool, error),
) (bool, error) {
	cursor := newArrayCombinationCursor(count, size)
	for indexes, exists := cursor.Next(); exists; indexes, exists = cursor.Next() {
		selected := make([]compositionEdit, 0, size)
		wanted := 0
		position := 0

		edits(func(edit compositionEdit) bool {
			if wanted < len(indexes) && indexes[wanted] == position {
				selected = append(selected, edit)
				wanted++
			}

			position++

			return wanted < len(indexes)
		})

		found, err := visit(selected)
		if err != nil || found {
			return found, err
		}
	}

	return false, nil
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
