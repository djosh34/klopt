//nolint:godoclint // Private product-graph state is documented at its search seams.
package schematest

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"unicode/utf16"
)

type basicStringEdgeKind uint8

const (
	basicStringEpsilon basicStringEdgeKind = iota
	basicStringUnit
	basicStringStart
	basicStringEnd
)

const basicStringMaxUnit = uint16(0xffff)

type basicStringEdge struct {
	kind   basicStringEdgeKind
	low    uint16
	high   uint16
	target int
}

type basicStringInterval struct {
	low  uint16
	high uint16
}

type basicStringMachine struct {
	states   [][]basicStringEdge
	start    int
	accept   int
	maxUnits uint64
}

type basicStringPatternState struct {
	active  []int
	matched bool
}

type basicStringProductState struct {
	patterns []basicStringPatternState
}

type basicStringProduct struct {
	machines []basicStringMachine
	maxUnits uint64
}

func activeBasicStringPatterns(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
) ([]*patternAST, bool, error) {
	patterns := make([]*patternAST, 0)

	supported, err := collectActiveBasicStringPatterns(node, occurrence, pins, &patterns)
	if err != nil {
		return nil, false, err
	}

	return patterns, supported, nil
}

//nolint:cyclop // Direct, allOf, and pinned anyOf patterns form one active-schema traversal.
func collectActiveBasicStringPatterns(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	patterns *[]*patternAST,
) (bool, error) {
	if node == nil || node.schemaShape == nil {
		return false, errors.New("schematest: basic string pattern schema has no shape")
	}

	if node.pattern != nil {
		if len(node.pattern.leadingAssertions) != 0 {
			return false, nil
		}

		if _, ok := basicStringExpressionMaxUnits(node.pattern.expression); !ok {
			return false, nil
		}

		*patterns = append(*patterns, node.pattern)
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		supported, err := collectActiveBasicStringPatterns(child, childOccurrence, pins, patterns)
		if err != nil || !supported {
			return supported, err
		}
	}

	states, pinned := rowCompositionTruthStates(pins, occurrence, "anyOf", len(node.anyOf))
	if !pinned {
		return true, nil
	}

	for index, child := range node.anyOf {
		if !states[index] {
			continue
		}

		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/anyOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		supported, err := collectActiveBasicStringPatterns(child, childOccurrence, pins, patterns)
		if err != nil || !supported {
			return supported, err
		}
	}

	return true, nil
}

func newBasicStringProduct(patterns []*patternAST) (*basicStringProduct, error) {
	if len(patterns) == 0 {
		return nil, errors.New("schematest: basic string product has no patterns")
	}

	product := &basicStringProduct{machines: make([]basicStringMachine, 0, len(patterns))}
	for _, pattern := range patterns {
		machine, err := compileBasicStringMachine(pattern)
		if err != nil {
			return nil, err
		}

		if product.maxUnits > ^uint64(0)-machine.maxUnits {
			return nil, errors.New("schematest: basic pattern witness length overflows uint64")
		}

		product.maxUnits += machine.maxUnits
		product.machines = append(product.machines, machine)
	}

	return product, nil
}

func compileBasicStringMachine(pattern *patternAST) (basicStringMachine, error) {
	if pattern == nil || pattern.expression == nil || len(pattern.leadingAssertions) != 0 {
		return basicStringMachine{}, errors.New("schematest: pattern is not a basic string expression")
	}

	maximum, ok := basicStringExpressionMaxUnits(pattern.expression)
	if !ok {
		return basicStringMachine{}, errors.New("schematest: pattern is not a finite basic string expression")
	}

	machine := basicStringMachine{start: 0, accept: 1, maxUnits: maximum}
	machine.states = make([][]basicStringEdge, machine.accept+1)

	if err := machine.compileExpression(pattern.expression, machine.start, machine.accept); err != nil {
		return basicStringMachine{}, err
	}

	return machine, nil
}

func (machine *basicStringMachine) compileExpression(expression *patternExpression, from, to int) error {
	if expression == nil {
		return errors.New("schematest: basic pattern expression is nil")
	}

	for _, alternative := range expression.alternatives {
		alternativeStart := machine.newState()
		alternativeEnd := machine.newState()
		machine.addEdge(from, basicStringEdge{kind: basicStringEpsilon, target: alternativeStart})

		if err := machine.compileSequence(alternative, alternativeStart, alternativeEnd); err != nil {
			return err
		}

		machine.addEdge(alternativeEnd, basicStringEdge{kind: basicStringEpsilon, target: to})
	}

	return nil
}

func (machine *basicStringMachine) compileSequence(sequence *patternSequence, from, to int) error {
	if sequence == nil {
		return errors.New("schematest: basic pattern sequence is nil")
	}

	current := from

	for _, term := range sequence.terms {
		if term == nil || term.atom == nil || term.quantified || term.minimum != 1 || term.maximum != 1 {
			return errors.New("schematest: quantified term is not a basic pattern term")
		}

		next := machine.newState()
		if err := machine.compileAtom(term.atom, current, next); err != nil {
			return err
		}

		current = next
	}

	machine.addEdge(current, basicStringEdge{kind: basicStringEpsilon, target: to})

	return nil
}

//nolint:mnd // Dot's four exact ES5.1 line-terminator boundaries are normative.
func (machine *basicStringMachine) compileAtom(atom *patternAtom, from, to int) error {
	var ranges []patternRange

	switch atom.kind {
	case patternLiteral:
		ranges = []patternRange{{low: atom.literal, high: atom.literal}}
	case patternDot:
		ranges = []patternRange{
			{low: 0x0000, high: 0x0009},
			{low: 0x000b, high: 0x000c},
			{low: 0x000e, high: 0x2027},
			{low: 0x202a, high: 0xffff},
		}
	case patternClassAtom:
		ranges = basicStringClassRanges(atom.class)
	case patternStart:
		machine.addEdge(from, basicStringEdge{kind: basicStringStart, target: to})
	case patternEnd:
		machine.addEdge(from, basicStringEdge{kind: basicStringEnd, target: to})
	case patternGroup:
		return machine.compileExpression(atom.expression, from, to)
	default:
		return fmt.Errorf("schematest: pattern atom %d is not a basic string atom", atom.kind)
	}

	for _, characterRange := range ranges {
		machine.addEdge(from, basicStringEdge{
			kind:   basicStringUnit,
			low:    characterRange.low,
			high:   characterRange.high,
			target: to,
		})
	}

	return nil
}

func basicStringClassRanges(class patternClass) []patternRange {
	ranges := make([]patternRange, 0)

	for _, part := range class.parts {
		partRanges := mergePatternRanges(append([]patternRange(nil), part.ranges...))
		if part.negated {
			partRanges = complementBasicStringRanges(partRanges)
		}

		ranges = append(ranges, partRanges...)
	}

	ranges = mergePatternRanges(ranges)
	if class.negated {
		return complementBasicStringRanges(ranges)
	}

	return ranges
}

func complementBasicStringRanges(ranges []patternRange) []patternRange {
	ranges = mergePatternRanges(ranges)
	complement := make([]patternRange, 0, len(ranges)+1)

	var next uint32

	for _, excluded := range ranges {
		if next < uint32(excluded.low) {
			complement = append(complement, patternRange{low: uint16(next), high: excluded.low - 1})
		}

		if excluded.high == basicStringMaxUnit {
			return complement
		}

		next = uint32(excluded.high) + 1
	}

	complement = append(complement, patternRange{low: uint16(next), high: basicStringMaxUnit})

	return complement
}

func (machine *basicStringMachine) newState() int {
	state := len(machine.states)
	machine.states = append(machine.states, nil)

	return state
}

func (machine *basicStringMachine) addEdge(state int, edge basicStringEdge) {
	machine.states[state] = append(machine.states[state], edge)
}

func basicStringExpressionMaxUnits(expression *patternExpression) (uint64, bool) {
	if expression == nil {
		return 0, false
	}

	var maximum uint64

	for _, alternative := range expression.alternatives {
		length, ok := basicStringSequenceMaxUnits(alternative)
		if !ok {
			return 0, false
		}

		maximum = max(maximum, length)
	}

	return maximum, true
}

//nolint:cyclop // Term validation and each admitted basic atom are one finite-length calculation.
func basicStringSequenceMaxUnits(sequence *patternSequence) (uint64, bool) {
	if sequence == nil {
		return 0, false
	}

	var length uint64

	for _, term := range sequence.terms {
		if term == nil || term.atom == nil || term.quantified || term.minimum != 1 || term.maximum != 1 {
			return 0, false
		}

		var atomLength uint64

		switch term.atom.kind {
		case patternLiteral, patternDot, patternClassAtom:
			atomLength = 1
		case patternStart, patternEnd:
		case patternGroup:
			var ok bool

			atomLength, ok = basicStringExpressionMaxUnits(term.atom.expression)
			if !ok {
				return 0, false
			}
		default:
			return 0, false
		}

		if length > ^uint64(0)-atomLength {
			return 0, false
		}

		length += atomLength
	}

	return length, true
}

func (product *basicStringProduct) start(length int) basicStringProductState {
	state := basicStringProductState{patterns: make([]basicStringPatternState, len(product.machines))}
	for index := range product.machines {
		active := product.machines[index].closure([]int{product.machines[index].start}, 0, length)
		state.patterns[index] = basicStringPatternState{
			active:  active,
			matched: containsBasicStringState(active, product.machines[index].accept),
		}
	}

	return state
}

func (product *basicStringProduct) advance(
	state basicStringProductState,
	unit uint16,
	position,
	length int,
) basicStringProductState {
	next := basicStringProductState{patterns: make([]basicStringPatternState, len(product.machines))}

	for index := range product.machines {
		machine := &product.machines[index]
		patternState := state.patterns[index]
		targets := make([]int, 0)

		if !patternState.matched {
			for _, active := range patternState.active {
				for _, edge := range machine.states[active] {
					if edge.kind == basicStringUnit && unit >= edge.low && unit <= edge.high {
						targets = appendUniqueBasicStringState(targets, edge.target)
					}
				}
			}
		}

		targets = appendUniqueBasicStringState(targets, machine.start)
		targets = machine.closure(targets, position+1, length)
		next.patterns[index] = basicStringPatternState{
			active:  targets,
			matched: patternState.matched || containsBasicStringState(targets, machine.accept),
		}
	}

	return next
}

func (machine *basicStringMachine) closure(states []int, position, length int) []int {
	closed := append([]int(nil), states...)

	seen := make([]bool, len(machine.states))
	for _, state := range closed {
		seen[state] = true
	}

	for index := 0; index < len(closed); index++ {
		for _, edge := range machine.states[closed[index]] {
			follow := edge.kind == basicStringEpsilon ||
				edge.kind == basicStringStart && position == 0 ||
				edge.kind == basicStringEnd && position == length
			if !follow || seen[edge.target] {
				continue
			}

			seen[edge.target] = true
			closed = append(closed, edge.target)
		}
	}

	sort.Ints(closed)

	return closed
}

// eachInterval incrementally partitions active edges wherever their transition
// truth can change, without constructing a Cartesian product of machines.
func (product *basicStringProduct) eachInterval(
	state basicStringProductState,
	visit func(basicStringInterval) bool,
) {
	var nextLow uint32

	for nextLow <= uint32(basicStringMaxUnit) {
		low, found := product.nextIntervalLow(state, nextLow)
		if !found {
			return
		}

		high := product.intervalHigh(state, low)
		if visit(basicStringInterval{low: low, high: high}) {
			return
		}

		nextLow = uint32(high) + 1
	}
}

func (product *basicStringProduct) nextIntervalLow(
	state basicStringProductState,
	minimum uint32,
) (uint16, bool) {
	var (
		low   uint16
		found bool
	)

	product.eachActiveUnitEdge(state, func(edge basicStringEdge) {
		candidate := max(minimum, uint32(edge.low))
		if candidate > uint32(edge.high) || found && candidate >= uint32(low) {
			return
		}

		low = uint16(candidate)
		found = true
	})

	return low, found
}

func (product *basicStringProduct) intervalHigh(state basicStringProductState, low uint16) uint16 {
	high := basicStringMaxUnit

	product.eachActiveUnitEdge(state, func(edge basicStringEdge) {
		switch {
		case low >= edge.low && low <= edge.high && edge.high < high:
			high = edge.high
		case edge.low > low && edge.low-1 < high:
			high = edge.low - 1
		}
	})

	return high
}

func (product *basicStringProduct) eachActiveUnitEdge(
	state basicStringProductState,
	visit func(basicStringEdge),
) {
	for patternIndex, machine := range product.machines {
		if state.patterns[patternIndex].matched {
			continue
		}

		for _, active := range state.patterns[patternIndex].active {
			for _, edge := range machine.states[active] {
				if edge.kind == basicStringUnit {
					visit(edge)
				}
			}
		}
	}
}

func (product *basicStringProduct) eachTransition(
	state basicStringProductState,
	seed uint64,
	visit func(uint16) bool,
) {
	product.eachInterval(state, func(interval basicStringInterval) bool {
		return eachBasicStringIntervalCandidate(interval, seed, visit)
	})
}

func eachBasicStringIntervalCandidate(
	interval basicStringInterval,
	seed uint64,
	visit func(uint16) bool,
) bool {
	if visit(interval.low) {
		return true
	}

	if interval.high == interval.low {
		return false
	}

	if visit(interval.high) {
		return true
	}

	if uint32(interval.high)-uint32(interval.low) == 1 {
		return false
	}

	interiorCount := uint32(interval.high) - uint32(interval.low) - 1
	candidate := uint32(interval.low) + 1 + uint32(seed%uint64(interiorCount))

	return visit(uint16(candidate))
}

func basicStringSeed(schemaPointer string, canonicalSchemaJSON []byte, rule, level string) uint64 {
	input := []byte("schematest-v1\x00")
	input = append(input, schemaPointer...)
	input = append(input, 0)
	input = append(input, canonicalSchemaJSON...)
	input = append(input, 0)
	input = append(input, rule...)
	input = append(input, 0)
	input = append(input, level...)
	digest := sha256.Sum256(input)

	return binary.BigEndian.Uint64(digest[:8])
}

func (product *basicStringProduct) accepting(state basicStringProductState) bool {
	for _, pattern := range state.patterns {
		if !pattern.matched {
			return false
		}
	}

	return true
}

func (s *search) walkBasicStringWitnesses(patterns []*patternAST, seed uint64, visit rowVisit) (bool, error) {
	product, err := newBasicStringProduct(patterns)
	if err != nil {
		return false, err
	}

	maxInt := uint64(^uint(0) >> 1)
	if product.maxUnits > maxInt {
		return false, errors.New("schematest: basic pattern witness length overflows int")
	}

	for length := 0; length <= int(product.maxUnits); length++ {
		units := make([]uint16, 0, length)

		complete, walkErr := s.walkBasicStringProduct(product, product.start(length), units, length, seed, visit)
		if walkErr != nil || complete {
			return complete, walkErr
		}
	}

	return false, nil
}

func (s *search) walkBasicStringProduct(
	product *basicStringProduct,
	state basicStringProductState,
	units []uint16,
	length int,
	seed uint64,
	visit rowVisit,
) (bool, error) {
	if len(units) == length {
		if !product.accepting(state) || !validBasicStringUnits(units) {
			return false, nil
		}

		return visit(&jsonValue{kind: jsonString, text: string(utf16.Decode(units))})
	}

	var (
		complete bool
		walkErr  error
	)

	product.eachTransition(state, seed, func(unit uint16) bool {
		if err := s.assign(); err != nil {
			walkErr = err

			return true
		}

		nextUnits := append(units, unit)
		complete, walkErr = s.walkBasicStringProduct(
			product,
			product.advance(state, unit, len(units), length),
			nextUnits,
			length,
			seed,
			visit,
		)

		return walkErr != nil || complete
	})

	return complete, walkErr
}

func validBasicStringUnits(units []uint16) bool {
	decoded := utf16.Decode(units)

	encoded := utf16.Encode(decoded)
	if len(encoded) != len(units) {
		return false
	}

	for index := range units {
		if encoded[index] != units[index] {
			return false
		}
	}

	return true
}

func appendUniqueBasicStringState(states []int, state int) []int {
	if containsBasicStringState(states, state) {
		return states
	}

	return append(states, state)
}

func containsBasicStringState(states []int, wanted int) bool {
	for _, state := range states {
		if state == wanted {
			return true
		}
	}

	return false
}
