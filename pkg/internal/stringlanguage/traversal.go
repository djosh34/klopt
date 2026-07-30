package stringlanguage

import "slices"

// State identifies one immutable state in a compiled string language.
type State uint32

// ScalarRange is one productive Unicode-scalar transition.
type ScalarRange struct {
	First rune
	Last  rune
	Next  State

	distance uint64
}

// Matches reports whether one exact language accepts value.
func (language Language) Matches(value string) bool {
	if len(language.dfa.states) == 0 {
		return false
	}

	state := uint32(0)
	for _, scalar := range value {
		state = advanceScalar(&language.dfa, state, scalar)
	}

	return language.dfa.states[state].accepting
}

// Start returns the initial state of a compiled language.
func (set *Set) Start() State {
	return 0
}

// Accepting reports whether stopping at state produces a member.
func (set *Set) Accepting(state State) bool {
	return set != nil && int(state) < len(set.product.states) && set.product.states[state].accepting
}

// ProductiveRanges returns immediate scalar ranges that can still reach acceptance.
// A shortest-completion edge is first so a zero-filled tape always terminates.
func (set *Set) ProductiveRanges(state State) []ScalarRange {
	if set == nil || int(state) >= len(set.product.states) {
		return nil
	}

	currentDistance := set.product.shortest[state]
	result := make([]ScalarRange, 0)

	for _, edge := range set.product.states[state].edges {
		if set.product.shortest[edge.next] < 0 {
			continue
		}

		for _, scalarRange := range edge.ranges {
			result = append(result, ScalarRange{
				First: scalarRange.first, Last: scalarRange.last, Next: State(edge.next),
			})
		}
	}

	slices.SortStableFunc(result, func(left ScalarRange, right ScalarRange) int {
		leftCompletes := set.product.shortest[left.Next] < currentDistance

		rightCompletes := set.product.shortest[right.Next] < currentDistance
		if leftCompletes != rightCompletes {
			if leftCompletes {
				return -1
			}

			return 1
		}

		if left.First < right.First {
			return -1
		}

		if left.First > right.First {
			return 1
		}

		return 0
	})

	return result
}
