//nolint:cyclop,gocognit,godoclint,mnd // Signed normalization and keys are explicit state machines.
package program

import (
	"encoding/binary"
	"fmt"
	"slices"
	"strings"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type goal struct {
	node nodeID
	want bool
}

type state struct {
	goals    []goal
	excluded []jsonvalue.Value
}

type branch struct {
	base state
	goal goal
}

func (program *Program) normalize(
	input state,
	work *decodeWork,
) (state, *branch, bool, error) {
	inputBytes, err := stateKeyBytes(input)
	if err != nil {
		return state{}, nil, false, err
	}

	if err := work.chargeSolver("normalize work", "normalize bytes", 1, inputBytes); err != nil {
		return state{}, nil, false, err
	}

	pending := slices.Clone(input.goals)
	atoms := make([]goal, 0, len(pending))
	excluded := slices.Clone(input.excluded)

	for len(pending) != 0 {
		last := len(pending) - 1
		current := pending[last]
		pending = pending[:last]
		item := program.nodes[current.node]

		switch item.kind {
		case nodeAtom:
			if item.atom.kind == atomEnum && !current.want {
				if err := work.chargeExactValues("normalize enum bytes", item.atom.values); err != nil {
					return state{}, nil, false, err
				}

				excluded = append(excluded, item.atom.values...)

				continue
			}

			atoms = append(atoms, current)
		case nodeAnd:
			if current.want {
				if err := work.chargeGoals("normalize goals", len(item.children)); err != nil {
					return state{}, nil, false, err
				}

				for _, child := range item.children {
					pending = append(pending, goal{node: child, want: true})
				}

				continue
			}

			if len(item.children) == 0 {
				return state{}, nil, false, nil
			}

			if len(item.children) == 1 {
				if err := work.chargeGoals("normalize goals", 1); err != nil {
					return state{}, nil, false, err
				}

				pending = append(pending, goal{node: item.children[0], want: false})

				continue
			}

			base := canonicalStateWithExclusions(append(atoms, pending...), excluded)

			return state{}, &branch{base: base, goal: current}, true, nil
		case nodeOr:
			if !current.want {
				if err := work.chargeGoals("normalize goals", len(item.children)); err != nil {
					return state{}, nil, false, err
				}

				for _, child := range item.children {
					pending = append(pending, goal{node: child, want: false})
				}

				continue
			}

			if len(item.children) == 0 {
				return state{}, nil, false, nil
			}

			if len(item.children) == 1 {
				if err := work.chargeGoals("normalize goals", 1); err != nil {
					return state{}, nil, false, err
				}

				pending = append(pending, goal{node: item.children[0], want: true})

				continue
			}

			base := canonicalStateWithExclusions(append(atoms, pending...), excluded)

			return state{}, &branch{base: base, goal: current}, true, nil
		}
	}

	terminal := canonicalStateWithExclusions(atoms, excluded)
	for index := 1; index < len(terminal.goals); index++ {
		if terminal.goals[index-1].node == terminal.goals[index].node &&
			terminal.goals[index-1].want != terminal.goals[index].want {
			return state{}, nil, false, nil
		}
	}

	return terminal, nil, true, nil
}

func canonicalState(goals []goal) state {
	return canonicalStateWithExclusions(goals, nil)
}

func canonicalStateWithExclusions(goals []goal, excluded []jsonvalue.Value) state {
	result := slices.Clone(goals)
	slices.SortFunc(result, func(left goal, right goal) int {
		if left.node != right.node {
			return int(left.node) - int(right.node)
		}

		if left.want == right.want {
			return 0
		}

		if !left.want {
			return -1
		}

		return 1
	})
	result = slices.Compact(result)

	exact := slices.Clone(excluded)
	slices.SortFunc(exact, compareExactValues)
	exact = slices.CompactFunc(exact, func(left jsonvalue.Value, right jsonvalue.Value) bool {
		return left.Equal(right)
	})

	return state{goals: result, excluded: exact}
}

func stateKey(value state) (string, error) {
	encoded := appendUint64(nil, uint64(len(value.goals)))
	for _, item := range value.goals {
		var goalBytes [5]byte
		binary.LittleEndian.PutUint32(goalBytes[:], uint32(item.node))

		if item.want {
			goalBytes[4] = 1
		}

		encoded = append(encoded, goalBytes[:]...)
	}

	encoded = appendUint64(encoded, uint64(len(value.excluded)))
	for index, excluded := range value.excluded {
		raw, err := excluded.MarshalJSON()
		if err != nil {
			return "", fmt.Errorf("encode exact exclusion %d: %w", index, err)
		}

		encoded = appendBytes(encoded, raw)
	}

	return string(encoded), nil
}

func stateKeyBytes(value state) (uint64, error) {
	goalBytes, ok := checkedMul(uint64(len(value.goals)), 5)
	if !ok {
		return 0, &ResourceError{
			Resource: "state key bytes", Limit: ^uint64(0), Observed: ^uint64(0),
		}
	}

	result, ok := checkedAdd(16, goalBytes)
	if !ok {
		return 0, &ResourceError{
			Resource: "state key bytes", Limit: ^uint64(0), Observed: ^uint64(0),
		}
	}

	for _, excluded := range value.excluded {
		size, err := exactValueBytes(excluded)
		if err != nil {
			return 0, err
		}

		framed, framedOK := checkedAdd(size, 8)
		if !framedOK {
			return 0, &ResourceError{
				Resource: "state key bytes", Limit: ^uint64(0), Observed: ^uint64(0),
			}
		}

		result, ok = checkedAdd(result, framed)
		if !ok {
			return 0, &ResourceError{
				Resource: "state key bytes", Limit: ^uint64(0), Observed: ^uint64(0),
			}
		}
	}

	return result, nil
}

func compareExactValues(left jsonvalue.Value, right jsonvalue.Value) int {
	if left.Kind != right.Kind {
		return int(left.Kind) - int(right.Kind)
	}

	switch left.Kind {
	case jsonvalue.KindNull:
		return 0
	case jsonvalue.KindBoolean:
		if left.Boolean == right.Boolean {
			return 0
		}

		if !left.Boolean {
			return -1
		}

		return 1
	case jsonvalue.KindNumber:
		return left.Number.Compare(right.Number)
	case jsonvalue.KindString:
		return strings.Compare(left.String, right.String)
	case jsonvalue.KindArray:
		return compareExactArrays(left.Array, right.Array)
	case jsonvalue.KindObject:
		return compareExactObjects(left.Object, right.Object)
	default:
		return 0
	}
}

func compareExactArrays(left []jsonvalue.Value, right []jsonvalue.Value) int {
	for index := 0; index < min(len(left), len(right)); index++ {
		if comparison := compareExactValues(left[index], right[index]); comparison != 0 {
			return comparison
		}
	}

	return len(left) - len(right)
}

func compareExactObjects(left []jsonvalue.Member, right []jsonvalue.Member) int {
	for index := 0; index < min(len(left), len(right)); index++ {
		if comparison := strings.Compare(left[index].Name, right[index].Name); comparison != 0 {
			return comparison
		}

		if comparison := compareExactValues(left[index].Value, right[index].Value); comparison != 0 {
			return comparison
		}
	}

	return len(left) - len(right)
}

func memoizedAnyProductive[T any](
	key string,
	known map[string]bool,
	results map[string]bool,
	candidates []T,
	test func(T) (bool, error),
) (bool, error) {
	if known[key] {
		return results[key], nil
	}

	for _, candidate := range candidates {
		possible, err := test(candidate)
		if err != nil {
			return false, err
		}

		if possible {
			known[key] = true
			results[key] = true

			return true, nil
		}
	}

	known[key] = true
	results[key] = false

	return false, nil
}
