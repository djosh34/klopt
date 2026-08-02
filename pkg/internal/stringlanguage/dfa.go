//nolint:godoclint,mnd // Private deterministic automaton vocabulary stays inside stringlanguage.
package stringlanguage

import (
	"fmt"
	"slices"
)

type dfaEdge struct {
	first  rune
	last   rune
	target uint32
}

type dfaState struct {
	edges     []dfaEdge
	accepting bool
}

type dfa struct {
	states []dfaState
	utf16  bool
}

type subsetState struct {
	states       []int
	atStart      bool
	previousWord bool
	previousLF   bool
}

func (machine *dfa) advance(state uint32, value rune) (uint32, error) {
	return advanceDFAState(machine, state, value)
}

func determinize(machine *nfa) (*dfa, error) {
	initial := subsetState{states: unconditionalClosure(machine, []int{machine.start}), atStart: true}
	states := []subsetState{initial}
	keys := map[string]uint32{subsetKey(initial): 0}
	result := &dfa{utf16: machine.utf16}

	addDFAState(result, machine, initial)

	alphabet := nfaAlphabet(machine)
	for current := 0; current < len(states); current++ {
		for _, characterClass := range alphabet {
			next := moveSubset(machine, states[current], characterClass.first)
			key := subsetKey(next)

			nextID, ok := keys[key]
			if !ok {
				nextID = uint32(len(states))

				addDFAState(result, machine, next)

				keys[key] = nextID

				states = append(states, next)
			}

			appendDFAEdge(&result.states[current].edges, dfaEdge{
				first: characterClass.first, last: characterClass.last, target: nextID,
			})
		}
	}

	if err := validateDFA(*result); err != nil {
		return nil, fmt.Errorf("determinize DFA: %w", err)
	}

	return result, nil
}

func addDFAState(result *dfa, machine *nfa, state subsetState) {
	result.states = append(result.states, dfaState{accepting: acceptsAtEnd(machine, state)})
}

func nfaAlphabet(machine *nfa) runeSet {
	sources := make([]runeSet, 0)

	for _, state := range machine.states {
		for _, edge := range state.edges {
			if edge.kind == edgeCharacters {
				sources = append(sources, edge.characters)
			}
		}
	}

	sources = append(sources, wordSet(), runeSet{{first: '\n', last: '\n'}})

	return partitionRuneSets(machine.universe, sources...)
}

//nolint:cyclop // Scalar projection handles direct and UTF-16 DFA alphabets explicitly.
func (machine *dfa) scalarEdges(state uint32) ([]dfaEdge, error) {
	if machine == nil || state >= uint32(len(machine.states)) {
		return nil, fmt.Errorf("invalid DFA state %d", state)
	}

	if !machine.utf16 {
		edges := make([]dfaEdge, 0, len(machine.states[state].edges))
		for _, edge := range machine.states[state].edges {
			for _, item := range intersectRuneSets(
				runeSet{{first: edge.first, last: edge.last}}, scalarUniverse,
			) {
				appendDFAEdge(&edges, dfaEdge{first: item.first, last: item.last, target: edge.target})
			}
		}

		return edges, nil
	}

	edges := make([]dfaEdge, 0, len(machine.states[state].edges))
	for _, edge := range machine.states[state].edges {
		for _, item := range intersectRuneSets(
			runeSet{{first: edge.first, last: edge.last}},
			runeSet{{first: 0, last: firstSurrogate - 1}, {first: lastSurrogate + 1, last: maximumCodeUnit}},
		) {
			appendDFAEdge(&edges, dfaEdge{first: item.first, last: item.last, target: edge.target})
		}
	}

	for high := rune(firstSurrogate); high <= 0xdbff; high++ {
		afterHigh, err := machine.advance(state, high)
		if err != nil {
			return nil, err
		}

		for _, edge := range machine.states[afterHigh].edges {
			first := max(edge.first, rune(0xdc00))

			last := min(edge.last, rune(lastSurrogate))
			if first > last {
				continue
			}

			appendDFAEdge(&edges, dfaEdge{
				first:  utf16Scalar(high, first),
				last:   utf16Scalar(high, last),
				target: edge.target,
			})
		}
	}

	return edges, nil
}

func utf16Scalar(high rune, low rune) rune {
	return 0x10000 + (high-firstSurrogate)*0x400 + low - 0xdc00
}

func partitionRuneSets(universe runeSet, sources ...runeSet) runeSet {
	boundaries := make([]int64, 0)
	for _, allowed := range universe {
		boundaries = append(boundaries, int64(allowed.first), int64(allowed.last)+1)
	}

	for _, source := range sources {
		for _, item := range source {
			boundaries = append(boundaries, int64(item.first), int64(item.last)+1)
		}
	}

	slices.Sort(boundaries)
	boundaries = slices.Compact(boundaries)

	result := make(runeSet, 0, len(boundaries))
	for index := 0; index+1 < len(boundaries); index++ {
		first := rune(boundaries[index])

		last := rune(boundaries[index+1] - 1)
		if first <= last && universe.contains(first) {
			result = append(result, runeRange{first: first, last: last})
		}
	}

	return result
}

func appendDFAEdge(edges *[]dfaEdge, edge dfaEdge) {
	if len(*edges) > 0 {
		last := &(*edges)[len(*edges)-1]
		if last.target == edge.target && last.last+1 == edge.first {
			last.last = edge.last

			return
		}
	}

	*edges = append(*edges, edge)
}
