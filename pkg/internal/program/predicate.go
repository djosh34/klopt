//nolint:cyclop,gocognit,gocyclo,godoclint // One atom dispatch keeps predicate meaning exhaustive.
package program

import (
	"errors"
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // Predicate parity uses the shared automata.
	"github.com/djosh34/klopt/pkg/jsonvalue"
)

func (program *Program) valueMatchesAtoms(goals []goal, value jsonvalue.Value) (bool, error) {
	for _, current := range goals {
		matches, err := program.atomMatches(program.nodes[current.node].atom, value)
		if err != nil {
			return false, err
		}

		if matches != current.want {
			return false, nil
		}
	}

	return true, nil
}

func (program *Program) valueAllowed(
	goals []goal,
	excluded []jsonvalue.Value,
	value jsonvalue.Value,
) (bool, error) {
	matches, err := program.valueMatchesAtoms(goals, value)
	if err != nil || !matches {
		return matches, err
	}

	for _, exact := range excluded {
		if exact.Equal(value) {
			return false, nil
		}
	}

	return true, nil
}

func (program *Program) valueMatchesGoals(goals []goal, value jsonvalue.Value) (bool, error) {
	for _, current := range goals {
		matches, err := program.nodeMatches(current.node, value)
		if err != nil {
			return false, err
		}

		if matches != current.want {
			return false, nil
		}
	}

	return true, nil
}

//nolint:cyclop // One explicit dispatch mirrors the atomic graph vocabulary.
func (program *Program) atomMatches(item atom, value jsonvalue.Value) (bool, error) {
	switch item.kind {
	case atomKinds:
		if !item.allowed[value.Kind] {
			return false, nil
		}

		if item.integer && value.Kind == jsonvalue.KindNumber {
			return value.Number.IsInteger(), nil
		}

		return true, nil
	case atomEnum:
		for _, allowed := range item.values {
			if allowed.Equal(value) {
				return true, nil
			}
		}

		return false, nil
	case atomNumberMinimum:
		if value.Kind != jsonvalue.KindNumber {
			return true, nil
		}

		comparison := value.Number.Compare(item.number)

		return comparison > 0 || comparison == 0 && !item.exclusive, nil
	case atomNumberMaximum:
		if value.Kind != jsonvalue.KindNumber {
			return true, nil
		}

		comparison := value.Number.Compare(item.number)

		return comparison < 0 || comparison == 0 && !item.exclusive, nil
	case atomNumberMultipleOf:
		return value.Kind != jsonvalue.KindNumber || value.Number.IsMultipleOf(item.number), nil
	case atomNumberFormat:
		return value.Kind != jsonvalue.KindNumber || numberMatchesFormat(value.Number, item.text), nil
	case atomStringMinLength:
		return value.Kind != jsonvalue.KindString ||
			uint64(utf8.RuneCountInString(value.String)) >= item.count, nil
	case atomStringMaxLength:
		return value.Kind != jsonvalue.KindString ||
			uint64(utf8.RuneCountInString(value.String)) <= item.count, nil
	case atomStringLanguage:
		if value.Kind != jsonvalue.KindString {
			return true, nil
		}

		set, err := stringlanguage.Compile([]stringlanguage.Requirement{{
			Language: item.language, WantMatch: true,
		}}, stringlanguage.Length{})
		if err != nil {
			var empty *stringlanguage.EmptyError
			if errors.As(err, &empty) {
				return false, nil
			}

			return false, fmt.Errorf("compile string atom: %w", err)
		}

		return set.Matches(value.String), nil
	case atomArrayMinItems:
		return value.Kind != jsonvalue.KindArray || uint64(len(value.Array)) >= item.count, nil
	case atomArrayMaxItems:
		return value.Kind != jsonvalue.KindArray || uint64(len(value.Array)) <= item.count, nil
	case atomArrayItems:
		if value.Kind != jsonvalue.KindArray {
			return true, nil
		}

		for _, child := range value.Array {
			matches, err := program.nodeMatches(item.child, child)
			if err != nil {
				return false, err
			}

			if !matches {
				return false, nil
			}
		}

		return true, nil
	case atomObjectMinProperties:
		return value.Kind != jsonvalue.KindObject || uint64(len(value.Object)) >= item.count, nil
	case atomObjectMaxProperties:
		return value.Kind != jsonvalue.KindObject || uint64(len(value.Object)) <= item.count, nil
	case atomObjectRequired:
		if value.Kind != jsonvalue.KindObject {
			return true, nil
		}

		_, present := objectMember(value.Object, item.name)

		return present, nil
	case atomObjectProperty:
		if value.Kind != jsonvalue.KindObject {
			return true, nil
		}

		child, present := objectMember(value.Object, item.name)
		if !present {
			return true, nil
		}

		return program.nodeMatches(item.child, child)
	case atomObjectAdditional:
		if value.Kind != jsonvalue.KindObject {
			return true, nil
		}

		for _, member := range value.Object {
			if slices.Contains(item.names, member.Name) {
				continue
			}

			if !item.allowedAdditional {
				return false, nil
			}

			if item.hasChild {
				matches, err := program.nodeMatches(item.child, member.Value)
				if err != nil {
					return false, err
				}

				if !matches {
					return false, nil
				}
			}
		}

		return true, nil
	default:
		return false, fmt.Errorf("unknown graph atom %d", item.kind)
	}
}

func objectMember(members []jsonvalue.Member, name string) (jsonvalue.Value, bool) {
	for _, member := range members {
		if member.Name == name {
			return member.Value, true
		}
	}

	return jsonvalue.Value{}, false
}

func (program *Program) nodeMatches(identifier nodeID, value jsonvalue.Value) (bool, error) {
	item := program.nodes[identifier]
	switch item.kind {
	case nodeAtom:
		return program.atomMatches(item.atom, value)
	case nodeAnd:
		for _, child := range item.children {
			matches, err := program.nodeMatches(child, value)
			if err != nil {
				return false, err
			}

			if !matches {
				return false, nil
			}
		}

		return true, nil
	case nodeOr:
		for _, child := range item.children {
			matches, err := program.nodeMatches(child, value)
			if err != nil {
				return false, err
			}

			if matches {
				return true, nil
			}
		}

		return false, nil
	default:
		return false, fmt.Errorf("unknown graph node %d", item.kind)
	}
}
