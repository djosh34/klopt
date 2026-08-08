package schematest

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

// applyCompositionFault searches for the smallest exact aggregate edit relative
// to the regenerated parent. Rejected candidates are transient.
func applyCompositionFault(parent *jsonValue, fault faultTarget, s *search) (*jsonValue, error) {
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
	fault faultTarget,
	s *search,
) (*jsonValue, bool, error) {
	derivative, found, err := applyCompositionPinEdits(parent, fault, s)
	if err != nil || found {
		return derivative, found, err
	}

	visit := func(value *jsonValue) (bool, error) {
		edits := compositionDifference(parent, value, nil)
		for size := 1; size <= len(edits); size++ {
			subsetFound, subsetErr := visitCompositionEditSubsets(
				edits,
				size,
				func(selected []compositionEdit) (bool, error) {
					candidate, matched, candidateErr := tryCompositionEdits(parent, fault, selected, s)
					if candidateErr != nil || !matched {
						return false, candidateErr
					}

					derivative = candidate

					return true, nil
				},
			)
			if subsetErr != nil || subsetFound {
				return subsetFound, subsetErr
			}
		}

		return false, nil
	}

	found, err = s.walkNode(
		s.model.root,
		s.model.root.occurrence,
		fault.pins,
		rowSearchContext{},
		visit,
	)

	return derivative, found, err
}

// applyCompositionPinEdits applies directly represented absent-presence pins.
//
//nolint:cyclop // Pin collection and deterministic subset search are one operation.
func applyCompositionPinEdits(
	parent *jsonValue,
	fault faultTarget,
	s *search,
) (*jsonValue, bool, error) {
	var edits []compositionEdit

	for _, pin := range fault.pins {
		if pin.canonical || pin.presence != planPinAbsent {
			continue
		}

		for _, path := range matchingValuePaths(parent, pin.occurrence.instanceTemplate) {
			if len(path) == 0 || valueAtPath(parent, path) == nil {
				continue
			}

			edit := compositionEdit{path: pathCopy(path), remove: true}
			if !compositionEditExists(edits, edit) {
				edits = append(edits, edit)
			}
		}
	}

	for size := 1; size <= len(edits); size++ {
		var derivative *jsonValue

		found, err := visitCompositionEditSubsets(edits, size, func(selected []compositionEdit) (bool, error) {
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
func tryCompositionEdits(
	parent *jsonValue,
	fault faultTarget,
	edits []compositionEdit,
	s *search,
) (*jsonValue, bool, error) {
	if assignErr := s.assign(); assignErr != nil {
		return nil, false, assignErr
	}

	candidate, copyErr := cloneJSONValue(parent)
	if copyErr != nil {
		return nil, false, fmt.Errorf("copy composition fault parent: %w", copyErr)
	}

	for _, edit := range edits {
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

	matches, matchErr := exactFailureClosure(result.failures, fault.closure)
	if matchErr != nil {
		return nil, false, fmt.Errorf("compare composition fault closure: %w", matchErr)
	}

	if result.valid || !matches {
		return nil, false, nil
	}

	return candidate, true, nil
}

// compositionEditExists reports whether an equivalent edit is already planned.
func compositionEditExists(edits []compositionEdit, candidate compositionEdit) bool {
	for _, edit := range edits {
		if reflect.DeepEqual(edit.path, candidate.path) && edit.remove == candidate.remove {
			return true
		}
	}

	return false
}

// compositionDifference returns deterministic leaf edits between two values.
//
//nolint:cyclop // Object, array, and scalar differences are one recursive operation.
func compositionDifference(parent, assignment *jsonValue, path []string) []compositionEdit {
	if parent == nil || assignment == nil || parent.kind != assignment.kind {
		return []compositionEdit{{path: append([]string(nil), path...), replacement: assignment}}
	}

	switch parent.kind {
	case jsonObject:
		var edits []compositionEdit

		for _, name := range sortedObjectNames(parent.object) {
			assigned, exists := assignment.object[name]
			if !exists {
				edits = append(edits, compositionEdit{path: append(pathCopy(path), name), remove: true})

				continue
			}

			edits = append(edits, compositionDifference(parent.object[name], assigned, append(pathCopy(path), name))...)
		}

		for _, name := range sortedObjectNames(assignment.object) {
			if _, exists := parent.object[name]; !exists {
				edits = append(edits, compositionEdit{
					path: append(pathCopy(path), name), replacement: assignment.object[name],
				})
			}
		}

		return edits
	case jsonArray:
		var edits []compositionEdit

		common := min(len(parent.array), len(assignment.array))
		for index := 0; index < common; index++ {
			token := strconv.Itoa(index)
			edits = append(edits, compositionDifference(
				parent.array[index], assignment.array[index], append(pathCopy(path), token),
			)...)
		}

		for index := len(parent.array) - 1; index >= len(assignment.array); index-- {
			edits = append(edits, compositionEdit{
				path: append(pathCopy(path), strconv.Itoa(index)), remove: true,
			})
		}

		for index := len(parent.array); index < len(assignment.array); index++ {
			edits = append(edits, compositionEdit{
				path:        append(pathCopy(path), strconv.Itoa(index)),
				replacement: assignment.array[index], append: true,
			})
		}

		return edits
	default:
		if jsonValuesEqual(parent, assignment) {
			return nil
		}

		return []compositionEdit{{path: append([]string(nil), path...), replacement: assignment}}
	}
}

// visitCompositionEditSubsets visits fixed-size edit subsets in source order.
func visitCompositionEditSubsets(
	edits []compositionEdit,
	size int,
	visit func([]compositionEdit) (bool, error),
) (bool, error) {
	selected := make([]compositionEdit, 0, size)

	var walk func(int) (bool, error)

	walk = func(start int) (bool, error) {
		if len(selected) == size {
			return visit(selected)
		}

		for index := start; index <= len(edits)-(size-len(selected)); index++ {
			selected = append(selected, edits[index])
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
func faultNeedsCompositionSearch(fault faultTarget) bool {
	if fault.obligation.rule == oracleRuleAllOf || fault.obligation.rule == oracleRuleAnyOf {
		return true
	}

	for _, failure := range fault.closure {
		if failure.rule == oracleRuleAnyOf {
			return true
		}
	}

	return false
}
