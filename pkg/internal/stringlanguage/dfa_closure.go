//nolint:godoclint,mnd // Private assertion closure tracks ECMAScript and RE2 positions.
package stringlanguage

import (
	"encoding/binary"
	"slices"
)

func unconditionalClosure(machine *nfa, seeds []int) []int {
	seen := make([]bool, len(machine.states))

	stack := append([]int(nil), seeds...)

	result := make([]int, 0, len(seeds))
	for len(stack) > 0 {
		last := len(stack) - 1
		state := stack[last]
		stack = stack[:last]

		if seen[state] {
			continue
		}

		seen[state] = true

		result = append(result, state)
		for _, edge := range machine.states[state].edges {
			if edge.kind == edgeEpsilon {
				stack = append(stack, edge.to)
			}
		}
	}

	slices.Sort(result)

	return result
}

func assertionClosure(
	machine *nfa,
	seeds []int,
	atStart bool,
	previousWord bool,
	previousLF bool,
	next *rune,
) []int {
	seen := make([]bool, len(machine.states))

	stack := append([]int(nil), seeds...)

	result := make([]int, 0, len(seeds))
	for len(stack) > 0 {
		last := len(stack) - 1
		state := stack[last]
		stack = stack[:last]

		if seen[state] {
			continue
		}

		seen[state] = true

		result = append(result, state)
		for _, edge := range machine.states[state].edges {
			if assertionEnabled(edge.kind, atStart, previousWord, previousLF, next) {
				stack = append(stack, edge.to)
			}
		}
	}

	slices.Sort(result)

	return result
}

//nolint:cyclop // Assertion kinds form a closed semantic table.
func assertionEnabled(kind edgeKind, atStart bool, previousWord bool, previousLF bool, next *rune) bool {
	atEnd := next == nil
	nextWord := !atEnd && isWordRune(*next)

	switch kind {
	case edgeEpsilon:
		return true
	case edgeBeginText:
		return atStart
	case edgeEndText:
		return atEnd
	case edgeBeginLine:
		return atStart || previousLF
	case edgeEndLine:
		return atEnd || *next == '\n'
	case edgeWordBoundary:
		return previousWord != nextWord
	case edgeNotWordBoundary:
		return previousWord == nextWord
	case edgeCharacters:
		return false
	default:
		return false
	}
}

func moveSubset(machine *nfa, current subsetState, value rune) subsetState {
	active := assertionClosure(
		machine, current.states, current.atStart, current.previousWord, current.previousLF, &value,
	)
	destinations := make([]int, 0)

	for _, state := range active {
		for _, edge := range machine.states[state].edges {
			if edge.kind == edgeCharacters && edge.characters.contains(value) {
				destinations = append(destinations, edge.to)
			}
		}
	}

	return subsetState{
		states:       unconditionalClosure(machine, destinations),
		previousWord: isWordRune(value), previousLF: value == '\n',
	}
}

func acceptsAtEnd(machine *nfa, state subsetState) bool {
	active := assertionClosure(
		machine, state.states, state.atStart, state.previousWord, state.previousLF, nil,
	)

	return slices.Contains(active, machine.accept)
}

func subsetKey(state subsetState) string {
	flags := byte(0)
	if state.atStart {
		flags |= 1
	}

	if state.previousWord {
		flags |= 2
	}

	if state.previousLF {
		flags |= 4
	}

	key := []byte{flags}
	for _, item := range state.states {
		key = binary.AppendUvarint(key, uint64(item)+1)
	}

	return string(key)
}

func stateTupleKey(states []uint32) string {
	key := make([]byte, 0, len(states)*2)
	for _, state := range states {
		key = binary.AppendUvarint(key, uint64(state)+1)
	}

	return string(key)
}

func isWordRune(value rune) bool {
	return value >= '0' && value <= '9' ||
		value >= 'A' && value <= 'Z' ||
		value >= 'a' && value <= 'z' ||
		value == '_'
}
