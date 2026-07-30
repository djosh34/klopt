//nolint:godoclint // Constructive search stays behind exact SetRef lowering.
package suite

import (
	"fmt"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type valueFinder struct {
	arena      *SetArena
	memo       map[SetRef]jsonvalue.Value
	inProgress map[SetRef]struct{}
}

func newValueFinder(arena *SetArena) *valueFinder {
	return &valueFinder{
		arena: arena, memo: make(map[SetRef]jsonvalue.Value), inProgress: make(map[SetRef]struct{}),
	}
}

func (finder *valueFinder) find(ref SetRef) (jsonvalue.Value, error) {
	if value, ok := finder.memo[ref]; ok {
		return value, nil
	}

	values, err := finder.values(ref, 1)
	if err != nil {
		return jsonvalue.Value{}, err
	}

	if len(values) == 0 {
		return jsonvalue.Value{}, fmt.Errorf("set %d/%t has no constructive value", ref.Node, ref.Negated)
	}

	finder.memo[ref] = values[0]

	return values[0], nil
}

//nolint:cyclop,gocognit // Candidate validation keeps exact assignment, set, and dedup checks transactional.
func (finder *valueFinder) values(ref SetRef, maximum int) ([]jsonvalue.Value, error) {
	if maximum < 1 {
		return nil, nil
	}

	if _, active := finder.inProgress[ref]; active {
		return nil, fmt.Errorf("recursive constructive set %d/%t", ref.Node, ref.Negated)
	}

	finder.inProgress[ref] = struct{}{}
	defer delete(finder.inProgress, ref)

	result := make([]jsonvalue.Value, 0)
	seen := make(map[jsonvalue.Hash]struct{})

	for kind := jsonvalue.KindNull; kind <= jsonvalue.KindObject && len(result) < maximum; kind++ {
		assignments := finder.solve(ref, kind, map[AtomID]bool{}, int(^uint(0)>>1))
		for _, assignment := range assignments {
			if len(result) >= maximum {
				break
			}

			productive, err := finder.arena.assignmentProductive(kind, assignment)
			if err != nil {
				return nil, err
			}

			if !productive {
				continue
			}

			candidates, err := finder.assignmentValues(kind, assignment)
			if err != nil {
				return nil, err
			}

			for _, candidate := range candidates {
				matchesAssignment, matchErr := finder.arena.candidateMatchesAssignment(candidate, assignment)
				if matchErr != nil {
					return nil, matchErr
				}

				matchesSet, setErr := finder.arena.Contains(ref, candidate)
				if setErr != nil {
					return nil, setErr
				}

				if !matchesAssignment || !matchesSet {
					continue
				}

				hash, hashErr := candidate.Hash()
				if hashErr != nil {
					return nil, hashErr
				}

				if _, duplicate := seen[hash]; duplicate {
					continue
				}

				seen[hash] = struct{}{}

				result = append(result, candidate)
				if len(result) >= maximum {
					break
				}
			}
		}
	}

	return result, nil
}

func (finder *valueFinder) solve(
	ref SetRef,
	kind jsonvalue.Kind,
	assignment map[AtomID]bool,
	maximum int,
) []map[AtomID]bool {
	want := !ref.Negated

	return finder.solveNode(ref.Node, want, kind, assignment, maximum)
}

//nolint:cyclop // Positive/negative union/intersection truth tables are deliberately explicit.
func (finder *valueFinder) solveNode(
	identifier SetID,
	want bool,
	kind jsonvalue.Kind,
	assignment map[AtomID]bool,
	maximum int,
) []map[AtomID]bool {
	if maximum < 1 {
		return nil
	}

	switch node := finder.arena.Nodes[identifier].(type) {
	case falseNode:
		if !want {
			return []map[AtomID]bool{cloneAssignment(assignment)}
		}
	case atomNode:
		if exactKind, ok := finder.arena.Atoms[node.Atom].(kindAtom); ok {
			if (exactKind.Kind == kind) == want {
				return []map[AtomID]bool{cloneAssignment(assignment)}
			}

			return nil
		}

		if existing, assigned := assignment[node.Atom]; assigned && existing != want {
			return nil
		}

		result := cloneAssignment(assignment)
		result[node.Atom] = want

		return []map[AtomID]bool{result}
	case intersectionNode:
		if want {
			return finder.solveAll(node.Children, true, kind, assignment, maximum)
		}

		return finder.solveAny(node.Children, false, kind, assignment, maximum)
	case unionNode:
		if want {
			return finder.solveAny(node.Children, true, kind, assignment, maximum)
		}

		return finder.solveAll(node.Children, false, kind, assignment, maximum)
	}

	return nil
}

func (finder *valueFinder) solveAll(
	children []SetRef,
	want bool,
	kind jsonvalue.Kind,
	assignment map[AtomID]bool,
	maximum int,
) []map[AtomID]bool {
	current := []map[AtomID]bool{cloneAssignment(assignment)}

	for _, child := range children {
		next := make([]map[AtomID]bool, 0)
		childWant := want != child.Negated

		for _, candidate := range current {
			next = append(next, finder.solveNode(
				child.Node, childWant, kind, candidate, maximum-len(next),
			)...)
			if len(next) >= maximum {
				break
			}
		}

		current = next
		if len(current) == 0 {
			break
		}
	}

	return current
}

func (finder *valueFinder) solveAny(
	children []SetRef,
	want bool,
	kind jsonvalue.Kind,
	assignment map[AtomID]bool,
	maximum int,
) []map[AtomID]bool {
	result := make([]map[AtomID]bool, 0)

	for _, child := range children {
		childWant := want != child.Negated

		result = append(result, finder.solveNode(
			child.Node, childWant, kind, assignment, maximum-len(result),
		)...)
		if len(result) >= maximum {
			break
		}
	}

	return result
}

func cloneAssignment(source map[AtomID]bool) map[AtomID]bool {
	result := make(map[AtomID]bool, len(source))
	for identifier, value := range source {
		result[identifier] = value
	}

	return result
}

func (finder *valueFinder) assignmentValues(
	kind jsonvalue.Kind,
	assignment map[AtomID]bool,
) ([]jsonvalue.Value, error) {
	if enumeration := finder.positiveEnumeration(kind, assignment); enumeration != nil {
		return enumeration, nil
	}

	switch kind {
	case jsonvalue.KindNull:
		return []jsonvalue.Value{jsonvalue.Null()}, nil
	case jsonvalue.KindBoolean:
		return []jsonvalue.Value{jsonvalue.Bool(false), jsonvalue.Bool(true)}, nil
	case jsonvalue.KindNumber:
		return finder.numberValues(assignment)
	case jsonvalue.KindString:
		return finder.stringValues(assignment)
	case jsonvalue.KindArray:
		return finder.arrayValues(assignment)
	case jsonvalue.KindObject:
		return finder.objectValues(assignment)
	default:
		return nil, fmt.Errorf("unknown JSON kind %d", kind)
	}
}

func (finder *valueFinder) positiveEnumeration(
	kind jsonvalue.Kind,
	assignment map[AtomID]bool,
) []jsonvalue.Value {
	var result []jsonvalue.Value

	for identifier, want := range assignment {
		enumeration, ok := finder.arena.Atoms[identifier].(enumAtom)
		if !ok || !want {
			continue
		}

		if result == nil {
			for _, candidate := range enumeration.Values {
				if candidate.Kind == kind {
					result = append(result, candidate)
				}
			}

			continue
		}

		filtered := result[:0]
		for _, candidate := range result {
			matches, err := enumeration.matches(finder.arena, candidate)
			if err == nil && matches {
				filtered = append(filtered, candidate)
			}
		}

		result = filtered
	}

	return result
}
