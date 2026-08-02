//nolint:cyclop,godoclint // Matcher validation is the complete DFA verdict path.
package stringlanguage

import (
	"errors"
	"fmt"
	"slices"
	"unicode/utf16"
)

// Matches reports whether value belongs to one exact language.
func (language Language) Matches(value string) (bool, error) {
	return matchDFA(&language.dfa, value)
}

// Equal reports whether two compiled languages have the same canonical DFA.
func (language Language) Equal(other Language) bool {
	if language.dfa.utf16 != other.dfa.utf16 || len(language.dfa.states) != len(other.dfa.states) {
		return false
	}

	for index, state := range language.dfa.states {
		otherState := other.dfa.states[index]
		if state.accepting != otherState.accepting || !slices.Equal(state.edges, otherState.edges) {
			return false
		}
	}

	return true
}

func matchDFA(machine *dfa, value string) (bool, error) {
	if machine == nil || len(machine.states) == 0 {
		return false, errors.New("DFA has no states")
	}

	state := uint32(0)

	for _, scalar := range value {
		var err error

		state, err = advanceScalarChecked(machine, state, scalar)
		if err != nil {
			return false, err
		}
	}

	return machine.states[state].accepting, nil
}

//nolint:gocognit // DFA validation checks every state, edge, and alphabet segment.
func validateDFA(machine dfa) error {
	if len(machine.states) == 0 {
		return errors.New("DFA has no states")
	}

	universe := scalarUniverse
	if machine.utf16 {
		universe = codeUnitUniverse()
	}

	for stateIndex, state := range machine.states {
		for edgeIndex, edge := range state.edges {
			if edge.first > edge.last {
				return fmt.Errorf("DFA state %d edge %d has an inverted range", stateIndex, edgeIndex)
			}

			if edge.target >= uint32(len(machine.states)) {
				return fmt.Errorf("DFA state %d edge %d has target %d outside state table", stateIndex, edgeIndex, edge.target)
			}

			if edgeIndex > 0 && state.edges[edgeIndex-1].last >= edge.first {
				return fmt.Errorf("DFA state %d edges overlap or are unsorted", stateIndex)
			}
		}

		for _, allowed := range universe {
			next := allowed.first
			for _, edge := range state.edges {
				if edge.last < allowed.first {
					continue
				}

				if edge.first > allowed.last {
					break
				}

				if edge.first > next || edge.last > allowed.last {
					return fmt.Errorf("DFA state %d does not cover its alphabet", stateIndex)
				}

				next = edge.last + 1
			}

			if next <= allowed.last {
				return fmt.Errorf("DFA state %d does not cover its alphabet", stateIndex)
			}
		}
	}

	return nil
}

func advanceScalarChecked(machine *dfa, state uint32, scalar rune) (uint32, error) {
	if machine == nil || state >= uint32(len(machine.states)) {
		return 0, errors.New("invalid DFA state")
	}

	if !machine.utf16 || scalar <= maximumCodeUnit {
		return advanceDFAState(machine, state, scalar)
	}

	high, low := utf16.EncodeRune(scalar)

	state, err := advanceDFAState(machine, state, high)
	if err != nil {
		return 0, err
	}

	return advanceDFAState(machine, state, low)
}

func advanceDFAState(machine *dfa, state uint32, value rune) (uint32, error) {
	if machine == nil || state >= uint32(len(machine.states)) {
		return 0, errors.New("invalid DFA state")
	}

	edges := machine.states[state].edges

	index, found := slices.BinarySearchFunc(edges, value, func(edge dfaEdge, target rune) int {
		switch {
		case edge.last < target:
			return -1
		case edge.first > target:
			return 1
		default:
			return 0
		}
	})
	if !found {
		return 0, fmt.Errorf("DFA state %d has no transition for U+%04X", state, value)
	}

	target := edges[index].target
	if target >= uint32(len(machine.states)) {
		return 0, fmt.Errorf("DFA state %d transition target %d is outside state table", state, target)
	}

	return target, nil
}
