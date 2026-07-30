//nolint:godoclint,mnd // Private product-graph vocabulary is local to certification.
package stringlanguage

import (
	"encoding/binary"
	"slices"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	productStateBaseBytes = 48
	productEdgeBytes      = 40
)

type productTuple struct {
	patterns []uint32
	length   int
}

type productEdge struct {
	next   uint32
	ranges runeSet
}

type productState struct {
	edges     []productEdge
	accepting bool
}

type product struct {
	states   []productState
	shortest []int
	minimum  int
	maximum  *int
}

type graphBudget struct {
	limits      constructionLimits
	states      uint64
	transitions uint64
	bytes       uint64
	work        uint64
}

//nolint:cyclop // Exhaustive exploration keeps every resource check at its allocation site.
func buildProduct(
	machines []*dfa,
	requirements []Requirement,
	length Length,
	limits constructionLimits,
) (*product, error) {
	work := &graphBudget{limits: limits}
	initial := productTuple{patterns: make([]uint32, len(machines))}
	tuples := []productTuple{initial}
	keys := map[string]uint32{productKey(initial): 0}
	graph := &product{minimum: length.Min, maximum: length.Max}

	if err := work.addState(len(machines)); err != nil {
		return nil, err
	}

	graph.states = append(graph.states, productState{
		accepting: productAccepts(initial, machines, requirements, length.Min, length.Max),
	})

	for current := 0; current < len(tuples); current++ {
		tuple := tuples[current]
		if length.Max != nil && tuple.length == *length.Max {
			continue
		}

		alphabet := productAlphabet(machines, tuple.patterns)
		if err := work.addTransitions(uint64(len(alphabet))); err != nil {
			return nil, err
		}

		if err := work.addCertificationWork(uint64(len(alphabet))); err != nil {
			return nil, err
		}

		byTarget := make(map[uint32]runeSet)
		for _, scalarClass := range alphabet {
			next := productTuple{
				patterns: make([]uint32, len(machines)),
				length:   nextLength(tuple.length, length.Min, length.Max),
			}
			for index, machine := range machines {
				next.patterns[index] = advanceScalar(machine, tuple.patterns[index], scalarClass.first)
			}

			key := productKey(next)

			nextID, ok := keys[key]
			if !ok {
				nextID = uint32(len(tuples))
				if err := work.addState(len(machines)); err != nil {
					return nil, err
				}

				keys[key] = nextID

				tuples = append(tuples, next)
				graph.states = append(graph.states, productState{
					accepting: productAccepts(next, machines, requirements, length.Min, length.Max),
				})
			}

			byTarget[nextID] = append(byTarget[nextID], scalarClass)
		}

		targets := make([]uint32, 0, len(byTarget))
		for target := range byTarget {
			targets = append(targets, target)
		}

		slices.SortFunc(targets, func(left uint32, right uint32) int {
			return int(byTarget[left][0].first - byTarget[right][0].first)
		})

		for _, target := range targets {
			graph.states[current].edges = append(graph.states[current].edges, productEdge{
				next: target, ranges: normalizeRuneSet(byTarget[target]),
			})
		}

		if err := work.addGraphBytes(uint64(len(graph.states[current].edges)) * productEdgeBytes); err != nil {
			return nil, err
		}
	}

	if err := graph.certify(work); err != nil {
		return nil, err
	}

	if graph.shortest[0] < 0 {
		return nil, &EmptyError{}
	}

	return graph, nil
}

func productAlphabet(machines []*dfa, states []uint32) runeSet {
	if len(machines) == 0 {
		return scalarUniverse
	}

	sources := make([]runeSet, 0, len(machines))
	for index, machine := range machines {
		sources = append(sources, machine.scalarRanges(states[index]))
	}

	return partitionRuneSets(scalarUniverse, sources...)
}

func (machine *dfa) scalarRanges(state uint32) runeSet {
	edges := machine.scalarEdges(state)

	set := make(runeSet, 0, len(edges))
	for _, edge := range edges {
		set = append(set, runeRange{first: edge.first, last: edge.last})
	}

	return set
}

func (machine *dfa) scalarEdges(state uint32) []dfaEdge {
	if !machine.utf16 {
		edges := make([]dfaEdge, 0, len(machine.states[state].edges))
		for _, edge := range machine.states[state].edges {
			for _, item := range intersectRuneSets(
				runeSet{{first: edge.first, last: edge.last}}, scalarUniverse,
			) {
				appendDFAEdge(&edges, dfaEdge{first: item.first, last: item.last, target: edge.target})
			}
		}

		return edges
	}

	edges := make([]dfaEdge, 0, len(machine.states[state].edges)+1024)
	for _, edge := range machine.states[state].edges {
		for _, item := range intersectRuneSets(
			runeSet{{first: edge.first, last: edge.last}},
			runeSet{{first: 0, last: firstSurrogate - 1}, {first: lastSurrogate + 1, last: maximumCodeUnit}},
		) {
			appendDFAEdge(&edges, dfaEdge{first: item.first, last: item.last, target: edge.target})
		}
	}

	for high := rune(0xd800); high <= 0xdbff; high++ {
		afterHigh := machine.advance(state, high)
		for _, edge := range machine.states[afterHigh].edges {
			first := max(edge.first, rune(0xdc00))

			last := min(edge.last, rune(0xdfff))
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

	return edges
}

func advanceScalar(machine *dfa, state uint32, scalar rune) uint32 {
	if !machine.utf16 || scalar <= maximumCodeUnit {
		return machine.advance(state, scalar)
	}

	high, low := utf16.EncodeRune(scalar)

	return machine.advance(machine.advance(state, high), low)
}

func utf16Scalar(high rune, low rune) rune {
	return 0x10000 + (high-0xd800)*0x400 + low - 0xdc00
}

//nolint:cyclop // Reverse reachability and shortest completion are one certification pass.
func (graph *product) certify(work *graphBudget) error {
	reverse := make([][]uint32, len(graph.states))
	for source, state := range graph.states {
		for _, edge := range state.edges {
			if err := work.addCertificationWork(1); err != nil {
				return err
			}

			reverse[edge.next] = append(reverse[edge.next], uint32(source))
		}
	}

	graph.shortest = make([]int, len(graph.states))
	for index := range graph.shortest {
		graph.shortest[index] = -1
	}

	queue := make([]uint32, 0)

	for state, item := range graph.states {
		if !item.accepting {
			continue
		}

		if err := work.addCertificationWork(1); err != nil {
			return err
		}

		graph.shortest[state] = 0
		queue = append(queue, uint32(state))
	}

	for head := 0; head < len(queue); head++ {
		state := queue[head]
		for _, predecessor := range reverse[state] {
			if err := work.addCertificationWork(1); err != nil {
				return err
			}

			if graph.shortest[predecessor] >= 0 {
				continue
			}

			graph.shortest[predecessor] = graph.shortest[state] + 1
			queue = append(queue, predecessor)
		}
	}

	return nil
}

func (graph *product) matches(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}

	length := utf8.RuneCountInString(value)
	if length < graph.minimum || graph.maximum != nil && length > *graph.maximum {
		return false
	}

	state := uint32(0)

	for _, scalar := range value {
		matched := false

		for _, edge := range graph.states[state].edges {
			if edge.ranges.contains(scalar) {
				state = edge.next
				matched = true

				break
			}
		}

		if !matched {
			return false
		}
	}

	return graph.states[state].accepting
}

func productAccepts(
	tuple productTuple,
	machines []*dfa,
	requirements []Requirement,
	minLength int,
	maxLength *int,
) bool {
	if tuple.length < minLength || maxLength != nil && tuple.length > *maxLength {
		return false
	}

	for index, machine := range machines {
		if machine.states[tuple.patterns[index]].accepting != requirements[index].WantMatch {
			return false
		}
	}

	return true
}

func nextLength(current int, minLength int, maxLength *int) int {
	if maxLength != nil {
		return current + 1
	}

	if current >= minLength {
		return minLength
	}

	return current + 1
}

func productKey(tuple productTuple) string {
	key := binary.AppendUvarint(nil, uint64(tuple.length)+1)
	for _, state := range tuple.patterns {
		key = binary.AppendUvarint(key, uint64(state)+1)
	}

	return string(key)
}

func (work *graphBudget) addState(patterns int) error {
	if err := addLimited(
		&work.states, 1, work.limits.productStates,
		"product construction", "reachable product states",
	); err != nil {
		return err
	}

	return work.addGraphBytes(productStateBaseBytes + uint64(patterns)*4)
}

func (work *graphBudget) addTransitions(amount uint64) error {
	return addLimited(
		&work.transitions, amount, work.limits.productTransitions,
		"product construction", "reachable product transitions",
	)
}

func (work *graphBudget) addGraphBytes(amount uint64) error {
	return addLimited(
		&work.bytes, amount, work.limits.graphBytes,
		"product construction", "graph bytes",
	)
}

func (work *graphBudget) addCertificationWork(amount uint64) error {
	return addLimited(
		&work.work, amount, work.limits.certificationWork,
		"certification", "certification work",
	)
}

func addLimited(counter *uint64, amount uint64, maximum uint64, phase string, limit string) error {
	if amount > ^uint64(0)-*counter {
		return limitError(phase, limit, maximum, ^uint64(0))
	}

	observed := *counter + amount
	if observed > maximum {
		return limitError(phase, limit, maximum, observed)
	}

	*counter = observed

	return nil
}
