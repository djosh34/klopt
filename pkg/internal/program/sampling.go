//nolint:cyclop,godoclint,mnd // Kind selection explicitly mirrors JSON's six kinds.
package program

import (
	"fmt"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

func (program *Program) sampleAtoms(
	goals []goal,
	excluded []jsonvalue.Value,
	reader *tapeReader,
	work *decodeWork,
	depth uint64,
) (jsonvalue.Value, bool, error) {
	if candidates, restricted := program.enumCandidates(goals); restricted {
		return program.sampleEnum(goals, excluded, candidates, reader)
	}

	allowed := program.allowedKindsForGoals(goals)

	kinds := make([]jsonvalue.Kind, 0, len(allowed))
	for kind, enabled := range allowed {
		if !enabled {
			continue
		}

		_, possible, err := program.sampleKind(
			goals, excluded, jsonvalue.Kind(kind), nil, work, depth,
		)
		if err != nil {
			return jsonvalue.Value{}, false, err
		}

		if possible {
			kinds = append(kinds, jsonvalue.Kind(kind))
		}
	}

	if len(kinds) == 0 {
		return jsonvalue.Value{}, false, nil
	}

	selected := 0
	if reader != nil {
		selected = int(reader.word() % uint64(len(kinds)))
	}

	return program.sampleKind(goals, excluded, kinds[selected], reader, work, depth)
}

func (program *Program) sampleEnum(
	goals []goal,
	excluded []jsonvalue.Value,
	candidates []jsonvalue.Value,
	reader *tapeReader,
) (jsonvalue.Value, bool, error) {
	matching := make([]jsonvalue.Value, 0, len(candidates))
	for _, candidate := range candidates {
		matches, err := program.valueAllowed(goals, excluded, candidate)
		if err != nil {
			return jsonvalue.Value{}, false, err
		}

		if matches {
			matching = append(matching, candidate)
		}
	}

	if len(matching) == 0 {
		return jsonvalue.Value{}, false, nil
	}

	selected := 0
	if reader != nil {
		selected = int(reader.word() % uint64(len(matching)))
	}

	return matching[selected], true, nil
}

func (program *Program) allowedKindsForGoals(goals []goal) [6]bool {
	allowed := [6]bool{true, true, true, true, true, true}

	for _, current := range goals {
		item := program.nodes[current.node].atom
		switch item.kind {
		case atomKinds:
			for kind := range allowed {
				if item.integer && jsonvalue.Kind(kind) == jsonvalue.KindNumber {
					continue
				}

				allowed[kind] = allowed[kind] && item.allowed[kind] == current.want
			}
		case atomNumberMinimum, atomNumberMaximum, atomNumberMultipleOf, atomNumberFormat:
			restrictWhenFalse(&allowed, current, jsonvalue.KindNumber)
		case atomStringMinLength, atomStringMaxLength, atomStringLanguage:
			restrictWhenFalse(&allowed, current, jsonvalue.KindString)
		case atomArrayMinItems, atomArrayMaxItems, atomArrayItems:
			restrictWhenFalse(&allowed, current, jsonvalue.KindArray)
		case atomObjectMinProperties, atomObjectMaxProperties,
			atomObjectRequired, atomObjectProperty, atomObjectAdditional:
			restrictWhenFalse(&allowed, current, jsonvalue.KindObject)
		}
	}

	return allowed
}

func restrictWhenFalse(allowed *[6]bool, current goal, kind jsonvalue.Kind) {
	if !current.want {
		restrictToKind(allowed, kind)
	}
}

func (program *Program) sampleKind(
	goals []goal,
	excluded []jsonvalue.Value,
	kind jsonvalue.Kind,
	reader *tapeReader,
	work *decodeWork,
	depth uint64,
) (jsonvalue.Value, bool, error) {
	switch kind {
	case jsonvalue.KindNull:
		value := jsonvalue.Null()
		matches, err := program.valueAllowed(goals, excluded, value)

		return value, matches, err
	case jsonvalue.KindBoolean:
		return program.sampleBoolean(goals, excluded, reader)
	case jsonvalue.KindNumber:
		number, possible, err := program.sampleNumber(goals, excluded, reader, work)

		return jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: number}, possible, err
	case jsonvalue.KindString:
		return program.sampleString(goals, excluded, reader, work)
	case jsonvalue.KindArray:
		return program.sampleArray(goals, excluded, reader, work, depth)
	case jsonvalue.KindObject:
		return program.sampleObject(goals, excluded, reader, work, depth)
	default:
		return jsonvalue.Value{}, false, fmt.Errorf("unknown JSON kind")
	}
}

func (program *Program) sampleBoolean(
	goals []goal,
	excluded []jsonvalue.Value,
	reader *tapeReader,
) (jsonvalue.Value, bool, error) {
	candidates := make([]jsonvalue.Value, 0, 2)

	for _, boolean := range []bool{false, true} {
		candidate := jsonvalue.Bool(boolean)

		matches, err := program.valueAllowed(goals, excluded, candidate)
		if err != nil {
			return jsonvalue.Value{}, false, err
		}

		if matches {
			candidates = append(candidates, candidate)
		}
	}

	if len(candidates) == 0 {
		return jsonvalue.Value{}, false, nil
	}

	selected := 0
	if reader != nil {
		selected = int(reader.word() % uint64(len(candidates)))
	}

	return candidates[selected], true, nil
}

func (program *Program) enumCandidates(goals []goal) ([]jsonvalue.Value, bool) {
	for _, current := range goals {
		item := program.nodes[current.node].atom
		if item.kind == atomEnum && current.want {
			return item.values, true
		}
	}

	return nil, false
}

func restrictToKind(allowed *[6]bool, selected jsonvalue.Kind) {
	for kind := range allowed {
		allowed[kind] = jsonvalue.Kind(kind) == selected && allowed[kind]
	}
}

func weightedIndex(word uint64, edges []productiveEdge) (int, error) {
	total := uint64(0)

	for _, edge := range edges {
		var ok bool

		total, ok = checkedAdd(total, edge.weight)
		if !ok {
			return 0, &ResourceError{
				Resource: "sampling weight", Limit: ^uint64(0), Observed: ^uint64(0),
			}
		}
	}

	if total == 0 {
		return 0, fmt.Errorf("productive edges have no sampling weight")
	}

	selected := word % total
	for index, edge := range edges {
		if selected < edge.weight {
			return index, nil
		}

		selected -= edge.weight
	}

	return 0, fmt.Errorf("sampling weight exceeds productive edges")
}

func appendCopy[T any](source []T, values ...T) []T {
	result := append([]T(nil), source...)

	return append(result, values...)
}
