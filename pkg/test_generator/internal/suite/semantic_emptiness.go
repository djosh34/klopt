//nolint:godoclint // Private SAT helpers stay behind SetArena.IsEmpty.
package suite

import (
	"fmt"
	"slices"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

// IsEmpty proves whether ref has no JSON values using exact signed theory reasoning.
func (arena *SetArena) IsEmpty(ref SetRef) (bool, error) {
	if int(ref.Node) >= len(arena.Nodes) {
		return false, fmt.Errorf("set reference %d is outside arena", ref.Node)
	}

	if _, known := arena.emptyKnown[ref]; known {
		return arena.emptyMemo[ref], nil
	}

	empty, err := arena.isEmpty(ref)
	if err != nil {
		return false, err
	}

	arena.emptyKnown[ref] = struct{}{}
	arena.emptyMemo[ref] = empty

	return empty, nil
}

//nolint:cyclop,nestif // Exact fast paths precede the signed theory solver.
func (arena *SetArena) isEmpty(ref SetRef) (bool, error) {
	if ref == arena.False() {
		return true, nil
	}

	if ref == arena.True() {
		return false, nil
	}

	if !ref.Negated {
		if node, ok := arena.Nodes[ref.Node].(unionNode); ok {
			for _, child := range node.Children {
				empty, err := arena.IsEmpty(child)
				if err != nil {
					return false, err
				}

				if !empty {
					return false, nil
				}
			}

			return true, nil
		}
	}

	solver := newValueFinder(arena)
	for kind := jsonvalue.KindNull; kind <= jsonvalue.KindObject; kind++ {
		assignments := solver.solve(ref, kind, map[AtomID]bool{}, int(^uint(0)>>1))
		for _, assignment := range assignments {
			productive, err := arena.assignmentProductive(kind, assignment)
			if err != nil {
				return false, err
			}

			if productive {
				return false, nil
			}
		}
	}

	return true, nil
}

func (arena *SetArena) referencedAtoms(ref SetRef, seen map[AtomID]struct{}) []AtomID {
	switch node := arena.Nodes[ref.Node].(type) {
	case atomNode:
		if _, kind := arena.Atoms[node.Atom].(kindAtom); !kind {
			seen[node.Atom] = struct{}{}
		}
	case intersectionNode:
		for _, child := range node.Children {
			arena.referencedAtoms(child, seen)
		}
	case unionNode:
		for _, child := range node.Children {
			arena.referencedAtoms(child, seen)
		}
	}

	result := make([]AtomID, 0, len(seen))
	for identifier := range seen {
		result = append(result, identifier)
	}

	slices.Sort(result)

	return result
}

func (arena *SetArena) kindFormulaSatisfiable(
	ref SetRef,
	kind jsonvalue.Kind,
	atoms []AtomID,
	index int,
	assignment map[AtomID]bool,
) (bool, error) {
	if index == len(atoms) {
		if !arena.evaluateFormula(ref, kind, assignment) {
			return false, nil
		}

		return arena.assignmentProductive(kind, assignment)
	}

	identifier := atoms[index]
	assignment[identifier] = false

	productive, err := arena.kindFormulaSatisfiable(ref, kind, atoms, index+1, assignment)
	if err != nil || productive {
		return productive, err
	}

	assignment[identifier] = true

	return arena.kindFormulaSatisfiable(ref, kind, atoms, index+1, assignment)
}

//nolint:cyclop // Each JSON kind delegates to one exact theory owner.
func (arena *SetArena) assignmentProductive(
	kind jsonvalue.Kind,
	assignment map[AtomID]bool,
) (bool, error) {
	for identifier, want := range assignment {
		if !atomApplies(arena.Atoms[identifier], kind) && !want {
			return false, nil
		}
	}

	for identifier, want := range assignment {
		enumeration, ok := arena.Atoms[identifier].(enumAtom)
		if !ok || !want {
			continue
		}

		for _, candidate := range enumeration.Values {
			if candidate.Kind != kind {
				continue
			}

			matches, err := arena.candidateMatchesAssignment(candidate, assignment)
			if err != nil {
				return false, err
			}

			if matches {
				return true, nil
			}
		}

		return false, nil
	}

	switch kind {
	case jsonvalue.KindNull:
		return arena.candidateMatchesAssignment(jsonvalue.Null(), assignment)
	case jsonvalue.KindBoolean:
		matches, err := arena.candidateMatchesAssignment(jsonvalue.Bool(false), assignment)
		if err != nil || matches {
			return matches, err
		}

		return arena.candidateMatchesAssignment(jsonvalue.Bool(true), assignment)
	case jsonvalue.KindString:
		return arena.stringAssignmentProductive(assignment)
	case jsonvalue.KindNumber:
		return arena.numberAssignmentProductive(assignment)
	case jsonvalue.KindArray:
		return arena.arrayAssignmentProductive(assignment)
	case jsonvalue.KindObject:
		return arena.objectAssignmentProductive(assignment)
	default:
		return true, nil
	}
}

func (arena *SetArena) candidateMatchesAssignment(
	candidate jsonvalue.Value,
	assignment map[AtomID]bool,
) (bool, error) {
	for identifier, want := range assignment {
		matches, err := arena.Atoms[identifier].matches(arena, candidate)
		if err != nil {
			return false, err
		}

		if matches != want {
			return false, nil
		}
	}

	return true, nil
}

func atomApplies(value atom, kind jsonvalue.Kind) bool {
	switch value.(type) {
	case enumAtom:
		return true
	case integerAtom, numberRangeAtom, multipleOfAtom, floatFormatAtom:
		return kind == jsonvalue.KindNumber
	case stringAtom:
		return kind == jsonvalue.KindString
	case arrayLengthAtom, arrayItemsAtom, arraySomeItemsAtom:
		return kind == jsonvalue.KindArray
	case objectCountAtom, requiredPropertyAtom, allowedPropertyNamesAtom,
		propertyValuesAtom, additionalPropertyValuesAtom, additionalSomePropertyAtom:
		return kind == jsonvalue.KindObject
	default:
		return false
	}
}

//nolint:cyclop // Exhaustive final node variants have distinct Boolean semantics.
func (arena *SetArena) evaluateFormula(ref SetRef, kind jsonvalue.Kind, assignment map[AtomID]bool) bool {
	var result bool

	switch node := arena.Nodes[ref.Node].(type) {
	case falseNode:
		result = false
	case atomNode:
		if exactKind, ok := arena.Atoms[node.Atom].(kindAtom); ok {
			result = exactKind.Kind == kind
		} else {
			result = assignment[node.Atom]
		}
	case intersectionNode:
		result = true

		for _, child := range node.Children {
			if !arena.evaluateFormula(child, kind, assignment) {
				result = false

				break
			}
		}
	case unionNode:
		for _, child := range node.Children {
			if arena.evaluateFormula(child, kind, assignment) {
				result = true

				break
			}
		}
	}

	if ref.Negated {
		result = !result
	}

	return result
}
