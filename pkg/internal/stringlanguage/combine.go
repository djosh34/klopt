package stringlanguage

import (
	"fmt"
	"slices"
	"unicode/utf8"
)

// literalTerminalAndDeadStates accounts for the accepting and dead states.
const literalTerminalAndDeadStates = 2

// SetRequirement says whether a compiled exact set must match.
type SetRequirement struct {
	Set       *Set
	WantMatch bool
}

// Combine compiles the exact signed intersection of already compiled sets.
func Combine(requirements []SetRequirement) (*Set, error) {
	converted := make([]Requirement, len(requirements))
	for index, requirement := range requirements {
		if requirement.Set == nil {
			return nil, &CompileError{
				Operation: "combine compiled sets",
				Err:       fmt.Errorf("requirement %d has a nil set", index),
			}
		}

		converted[index] = Requirement{
			Language:  Language{dfa: productLanguage(&requirement.Set.product)},
			WantMatch: requirement.WantMatch,
		}
	}

	return Compile(converted, Length{})
}

// Literal compiles exactly one Unicode string as a language.
func Literal(value string) (Language, error) {
	if !utf8.ValidString(value) {
		return Language{}, &CompileError{
			Operation: "compile literal",
			Err:       fmt.Errorf("literal contains invalid UTF-8"),
		}
	}

	scalars := []rune(value)
	dead := uint32(len(scalars) + 1)
	machine := dfa{states: make([]dfaState, len(scalars)+literalTerminalAndDeadStates)}
	machine.states[len(scalars)].accepting = true

	for index, scalar := range scalars {
		classes := partitionRuneSets(scalarUniverse, runeSet{{first: scalar, last: scalar}})
		for _, class := range classes {
			target := dead
			if class.first <= scalar && scalar <= class.last {
				target = uint32(index + 1)
			}

			appendDFAEdge(&machine.states[index].edges, dfaEdge{
				first:  class.first,
				last:   class.last,
				target: target,
			})
		}
	}

	for state := len(scalars); state < len(machine.states); state++ {
		for _, scalarRange := range scalarUniverse {
			appendDFAEdge(&machine.states[state].edges, dfaEdge{
				first:  scalarRange.first,
				last:   scalarRange.last,
				target: dead,
			})
		}
	}

	return Language{dfa: machine}, nil
}

// productLanguage converts a certified product into one total scalar DFA.
func productLanguage(source *product) dfa {
	dead := uint32(len(source.states))
	result := dfa{states: make([]dfaState, len(source.states)+1)}

	for state, item := range source.states {
		result.states[state].accepting = item.accepting

		type rangedTarget struct {
			first  rune
			last   rune
			target uint32
		}

		ranges := make([]rangedTarget, 0)

		sets := make([]runeSet, 0, len(item.edges))
		for _, edge := range item.edges {
			sets = append(sets, edge.ranges)
			for _, scalarRange := range edge.ranges {
				ranges = append(ranges, rangedTarget{
					first:  scalarRange.first,
					last:   scalarRange.last,
					target: edge.next,
				})
			}
		}

		slices.SortFunc(ranges, func(left rangedTarget, right rangedTarget) int {
			return int(left.first - right.first)
		})

		for _, scalarClass := range partitionRuneSets(scalarUniverse, sets...) {
			target := dead

			for _, candidate := range ranges {
				if candidate.first <= scalarClass.first && scalarClass.first <= candidate.last {
					target = candidate.target

					break
				}
			}

			appendDFAEdge(&result.states[state].edges, dfaEdge{
				first:  scalarClass.first,
				last:   scalarClass.last,
				target: target,
			})
		}
	}

	for _, scalarRange := range scalarUniverse {
		appendDFAEdge(&result.states[dead].edges, dfaEdge{
			first:  scalarRange.first,
			last:   scalarRange.last,
			target: dead,
		})
	}

	return result
}
