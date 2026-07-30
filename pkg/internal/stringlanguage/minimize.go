//nolint:godoclint // Private DFA minimization supports the bounded format grammars.
package stringlanguage

import "encoding/binary"

func minimizeDFA(machine *dfa) *dfa {
	classes := make([]uint32, len(machine.states))
	for index := range machine.states {
		if machine.states[index].accepting {
			classes[index] = 1
		}
	}

	for {
		ids := make(map[string]uint32)

		nextClasses := make([]uint32, len(machine.states))
		for index, state := range machine.states {
			signature := dfaClassKey(state, classes)

			class, ok := ids[signature]
			if !ok {
				class = uint32(len(ids))
				ids[signature] = class
			}

			nextClasses[index] = class
		}

		if equalDFAClasses(classes, nextClasses) {
			return rebuildDFAClasses(machine, classes)
		}

		classes = nextClasses
	}
}

func dfaClassKey(state dfaState, classes []uint32) string {
	key := []byte{0}
	if state.accepting {
		key[0] = 1
	}

	classEdges := make([]dfaEdge, 0, len(state.edges))
	for _, edge := range state.edges {
		appendDFAEdge(&classEdges, dfaEdge{
			first: edge.first, last: edge.last, target: classes[edge.target],
		})
	}

	for _, edge := range classEdges {
		key = binary.AppendUvarint(key, uint64(edge.first))
		key = binary.AppendUvarint(key, uint64(edge.last))
		key = binary.AppendUvarint(key, uint64(edge.target))
	}

	return string(key)
}

func equalDFAClasses(left []uint32, right []uint32) bool {
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}

func rebuildDFAClasses(machine *dfa, classes []uint32) *dfa {
	classCount := uint32(0)
	for _, class := range classes {
		classCount = max(classCount, class+1)
	}

	result := &dfa{states: make([]dfaState, classCount), utf16: machine.utf16}

	initialized := make([]bool, classCount)
	for index, class := range classes {
		if initialized[class] {
			continue
		}

		initialized[class] = true

		result.states[class].accepting = machine.states[index].accepting
		for _, edge := range machine.states[index].edges {
			appendDFAEdge(&result.states[class].edges, dfaEdge{
				first: edge.first, last: edge.last, target: classes[edge.target],
			})
		}
	}

	return result
}
