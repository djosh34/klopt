//nolint:godoclint // Private deterministic automaton vocabulary stays inside stringlanguage.
package stringlanguage

import (
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

type leafSpecification struct {
	machine   *dfa
	wantMatch bool
}

func (machine *dfa) advance(state uint32, value rune) uint32 {
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
		panic("DFA transition table does not cover its alphabet")
	}

	return edges[index].target
}

func determinize(machine *nfa, work *budget) (*dfa, error) {
	initial := subsetState{states: unconditionalClosure(machine, []int{machine.start}), atStart: true}
	states := []subsetState{initial}
	keys := map[string]uint32{subsetKey(initial): 0}
	result := &dfa{utf16: machine.utf16}

	if err := addDFAState(work, result, machine, initial); err != nil {
		return nil, err
	}

	alphabet := nfaAlphabet(machine)
	for current := 0; current < len(states); current++ {
		if err := work.add(
			&work.dfaTransitions, uint64(len(alphabet)), work.limits.dfaTransitions,
			"DFA construction", "DFA transitions",
		); err != nil {
			return nil, err
		}

		for _, characterClass := range alphabet {
			next := moveSubset(machine, states[current], characterClass.first)
			key := subsetKey(next)

			nextID, ok := keys[key]
			if !ok {
				nextID = uint32(len(states))

				if err := addDFAState(work, result, machine, next); err != nil {
					return nil, err
				}

				keys[key] = nextID

				states = append(states, next)
			}

			appendDFAEdge(&result.states[current].edges, dfaEdge{
				first: characterClass.first, last: characterClass.last, target: nextID,
			})
		}
	}

	return result, nil
}

func addDFAState(work *budget, result *dfa, machine *nfa, state subsetState) error {
	if err := work.add(
		&work.dfaStates, 1, work.limits.dfaStates,
		"DFA construction", "DFA states",
	); err != nil {
		return err
	}

	result.states = append(result.states, dfaState{accepting: acceptsAtEnd(machine, state)})

	return nil
}

func combineLeaves(leaves []leafSpecification, work *budget) (*dfa, error) {
	if len(leaves) == 1 && leaves[0].wantMatch {
		return leaves[0].machine, nil
	}

	initial := make([]uint32, len(leaves))
	tuples := [][]uint32{initial}
	keys := map[string]uint32{stateTupleKey(initial): 0}

	result := &dfa{utf16: true}
	if err := addCombinedState(result, leaves, initial, work); err != nil {
		return nil, err
	}

	for current := 0; current < len(tuples); current++ {
		alphabet := combinedAlphabet(leaves, tuples[current])
		if err := work.add(
			&work.dfaTransitions, uint64(len(alphabet)), work.limits.dfaTransitions,
			"DFA construction", "DFA transitions",
		); err != nil {
			return nil, err
		}

		for _, characterClass := range alphabet {
			next := make([]uint32, len(leaves))
			for index, leaf := range leaves {
				next[index] = leaf.machine.advance(tuples[current][index], characterClass.first)
			}

			key := stateTupleKey(next)

			nextID, ok := keys[key]
			if !ok {
				nextID = uint32(len(tuples))

				if err := addCombinedState(result, leaves, next, work); err != nil {
					return nil, err
				}

				keys[key] = nextID

				tuples = append(tuples, next)
			}

			appendDFAEdge(&result.states[current].edges, dfaEdge{
				first: characterClass.first, last: characterClass.last, target: nextID,
			})
		}
	}

	return result, nil
}

func addCombinedState(result *dfa, leaves []leafSpecification, tuple []uint32, work *budget) error {
	if err := work.add(
		&work.dfaStates, 1, work.limits.dfaStates,
		"DFA construction", "DFA states",
	); err != nil {
		return err
	}

	accepting := true
	for index, leaf := range leaves {
		if leaf.machine.states[tuple[index]].accepting != leaf.wantMatch {
			accepting = false

			break
		}
	}

	result.states = append(result.states, dfaState{accepting: accepting})

	return nil
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

func combinedAlphabet(leaves []leafSpecification, tuple []uint32) runeSet {
	sources := make([]runeSet, 0, len(leaves))
	for index, leaf := range leaves {
		edges := leaf.machine.states[tuple[index]].edges

		set := make(runeSet, 0, len(edges))
		for _, edge := range edges {
			set = append(set, runeRange{first: edge.first, last: edge.last})
		}

		sources = append(sources, set)
	}

	return partitionRuneSets(codeUnitUniverse(), sources...)
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
