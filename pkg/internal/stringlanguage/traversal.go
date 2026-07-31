//nolint:cyclop,godoclint // Tuple validation and traversal are the complete walk state machine.
package stringlanguage

import (
	"errors"
	"fmt"
	"slices"
	"unicode/utf16"
)

// Requirement says whether the result must match or not match Language.
type Requirement struct {
	Language  Language
	WantMatch bool
}

// ScalarRange is one range of Unicode scalar values.
type ScalarRange struct {
	First rune
	Last  rune
}

// Walk advances every requirement over one shared rune stream.
type Walk interface {
	Accepting() bool
	Ranges() []ScalarRange
	Advance(value rune) error
}

type tupleWalk struct {
	machines []dfa
	states   []uint32
	wants    []bool
}

// Begin starts a simultaneous walk over one-language DFAs.
func Begin(requirements []Requirement) (Walk, error) {
	machines := make([]dfa, len(requirements))
	states := make([]uint32, len(requirements))
	wants := make([]bool, len(requirements))

	for index, requirement := range requirements {
		if err := validateDFA(requirement.Language.dfa); err != nil {
			return nil, &CompileError{
				Operation: "begin string walk",
				Err:       fmt.Errorf("requirement %d has an invalid language: %w", index, err),
			}
		}

		machines[index] = requirement.Language.dfa
		wants[index] = requirement.WantMatch
	}

	return &tupleWalk{machines: machines, states: states, wants: wants}, nil
}

// Matches reports whether value belongs to one exact language.
func (language Language) Matches(value string) bool {
	if err := validateDFA(language.dfa); err != nil {
		return false
	}

	state := uint32(0)

	for _, scalar := range value {
		var err error

		state, err = advanceScalarChecked(&language.dfa, state, scalar)
		if err != nil {
			return false
		}
	}

	return language.dfa.states[state].accepting
}

func (walk *tupleWalk) Accepting() bool {
	if walk == nil || len(walk.states) != len(walk.machines) || len(walk.wants) != len(walk.machines) {
		return false
	}

	for index, machine := range walk.machines {
		state := walk.states[index]
		if state >= uint32(len(machine.states)) {
			return false
		}

		if machine.states[state].accepting != walk.wants[index] {
			return false
		}
	}

	return true
}

func (walk *tupleWalk) Ranges() []ScalarRange {
	if walk == nil || len(walk.states) != len(walk.machines) || len(walk.wants) != len(walk.machines) {
		return nil
	}

	sources := make([]runeSet, 0, len(walk.machines))
	for index, machine := range walk.machines {
		if err := validateDFA(machine); err != nil {
			return nil
		}

		state := walk.states[index]
		if state >= uint32(len(machine.states)) {
			return nil
		}

		edges := machine.scalarEdges(state)

		set := make(runeSet, 0, len(edges))
		for _, edge := range edges {
			set = append(set, runeRange{first: edge.first, last: edge.last})
		}

		sources = append(sources, set)
	}

	if len(sources) == 0 {
		ranges := make([]ScalarRange, 0, len(scalarUniverse))
		for _, item := range scalarUniverse {
			ranges = append(ranges, ScalarRange{First: item.first, Last: item.last})
		}

		return ranges
	}

	partition := partitionRuneSets(scalarUniverse, sources...)

	ranges := make([]ScalarRange, 0, len(partition))
	for _, item := range partition {
		ranges = append(ranges, ScalarRange{First: item.first, Last: item.last})
	}

	return ranges
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

func (walk *tupleWalk) Advance(value rune) error {
	if walk == nil {
		return errors.New("advance nil string walk")
	}

	if value < 0 || value > maximumScalar || value >= firstSurrogate && value <= lastSurrogate {
		return fmt.Errorf("invalid Unicode scalar U+%04X", value)
	}

	if len(walk.states) != len(walk.machines) || len(walk.wants) != len(walk.machines) {
		return errors.New("invalid string walk state")
	}

	next := make([]uint32, len(walk.states))
	for index, machine := range walk.machines {
		state := walk.states[index]
		if state >= uint32(len(machine.states)) {
			return fmt.Errorf("invalid DFA state %d for requirement %d", state, index)
		}

		advanced, err := advanceScalarChecked(&machine, state, value)
		if err != nil {
			return fmt.Errorf("advance requirement %d: %w", index, err)
		}

		next[index] = advanced
	}

	copy(walk.states, next)

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

	return edges[index].target, nil
}
