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
	basicStringWordBoundary
	basicStringNotWordBoundary
)

const (
	basicStringMaxUnit      = uint16(0xffff)
	basicStringUnitsPerRune = uint64(2)
)

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
	states       [][]basicStringEdge
	start        int
	accept       int
	maxUnits     uint64
	unbounded    bool
	hasBoundary  bool
	hasSurrogate bool
	restart      bool
	expected     bool
	required     bool
}

type basicStringPatternState struct {
	active  []int
	matched bool
}

type basicStringProductState struct {
	patterns     []basicStringPatternState
	position     int
	length       int
	previousWord bool
}

type basicStringProduct struct {
	machines     []basicStringMachine
	maxUnits     uint64
	unbounded    bool
	hasSurrogate bool
}

type basicStringLengthObjective struct {
	length uint64
	pinned bool
}

type basicStringLengths struct {
	boundaries      []uint64
	minimum         uint64
	minimumTooLarge bool
	maximum         uint64
	hasMaximum      bool
}

func (lengths *basicStringLengths) addMinimum(count *exactCount) error {
	length, fits, err := exactCountUint64(count)
	if err != nil {
		return err
	}

	if !fits {
		lengths.minimumTooLarge = true

		return nil
	}

	lengths.boundaries = append(lengths.boundaries, length)
	lengths.minimum = max(lengths.minimum, length)

	return nil
}

func (lengths *basicStringLengths) addMaximum(count *exactCount) error {
	length, fits, err := exactCountUint64(count)
	if err != nil {
		return err
	}

	if !fits {
		return nil
	}

	lengths.boundaries = append(lengths.boundaries, length)
	if !lengths.hasMaximum || length < lengths.maximum {
		lengths.maximum = length
		lengths.hasMaximum = true
	}

	return nil
}

func (lengths basicStringLengths) allows(length uint64) bool {
	return !lengths.minimumTooLarge && length >= lengths.minimum &&
		(!lengths.hasMaximum || length <= lengths.maximum)
}

// each streams exact rune-length assignments without retaining the fair frontier.
//
//nolint:cyclop // Pinned, boundary, and fair phases share exact first-occurrence deduplication.
func (lengths basicStringLengths) each(
	product *basicStringProduct,
	objective basicStringLengthObjective,
	visit func(uint64) bool,
) {
	special := make([]uint64, 0, len(lengths.boundaries)+1)
	if objective.pinned {
		special = append(special, objective.length)
	}

	special = append(special, lengths.boundaries...)

	for index, length := range special {
		duplicate := false

		for _, earlier := range special[:index] {
			if earlier == length {
				duplicate = true

				break
			}
		}

		if !duplicate && visit(length) {
			return
		}
	}

	maximum, bounded := uint64(0), false
	if product != nil && !product.unbounded {
		maximum, bounded = product.maxUnits, true
	}

	if lengths.hasMaximum && (!bounded || lengths.maximum < maximum) {
		maximum, bounded = lengths.maximum, true
	}

	for length := uint64(0); ; length++ {
		if bounded && length > maximum {
			return
		}

		duplicate := false

		for _, earlier := range special {
			if earlier == length {
				duplicate = true

				break
			}
		}

		if !duplicate && visit(length) {
			return
		}

		if length == ^uint64(0) {
			return
		}
	}
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

func activeBasicStringLengths(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
) (basicStringLengths, error) {
	var lengths basicStringLengths
	if err := collectActiveBasicStringLengths(node, occurrence, pins, &lengths); err != nil {
		return basicStringLengths{}, err
	}

	return lengths, nil
}

//nolint:cyclop // Direct, allOf, and pinned anyOf lengths form one active-schema traversal.
func collectActiveBasicStringLengths(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	lengths *basicStringLengths,
) error {
	if node == nil || node.schemaShape == nil {
		return errors.New("schematest: basic string length schema has no shape")
	}

	if node.minLength != nil {
		if err := lengths.addMinimum(node.minLength); err != nil {
			return fmt.Errorf("schematest: collect minLength at %s: %w", occurrence.usePointer, err)
		}
	}

	if node.maxLength != nil {
		if err := lengths.addMaximum(node.maxLength); err != nil {
			return fmt.Errorf("schematest: collect maxLength at %s: %w", occurrence.usePointer, err)
		}
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if err := collectActiveBasicStringLengths(child, childOccurrence, pins, lengths); err != nil {
			return err
		}
	}

	states, pinned := rowCompositionTruthStates(pins, occurrence, "anyOf", len(node.anyOf))
	if !pinned {
		return nil
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
		if err := collectActiveBasicStringLengths(child, childOccurrence, pins, lengths); err != nil {
			return err
		}
	}

	return nil
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
		if _, _, ok := basicStringExpressionLength(node.pattern.expression); !ok {
			return false, nil
		}

		for _, assertion := range node.pattern.leadingAssertions {
			if _, _, ok := basicStringExpressionLength(assertion.expression); !ok {
				return false, nil
			}
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
	return newBasicStringProductForFailure(patterns, -1, -1)
}

//nolint:cyclop // Compilation, one directed alternative, and exact bounds are one product operation.
func newBasicStringProductForFailure(
	patterns []*patternAST,
	directedPattern,
	failureAlternative int,
) (*basicStringProduct, error) {
	if len(patterns) == 0 {
		return nil, errors.New("schematest: basic string product has no patterns")
	}

	product := &basicStringProduct{machines: make([]basicStringMachine, 0, len(patterns))}
	for patternIndex, pattern := range patterns {
		machines, err := compileBasicStringPatternMachines(pattern)
		if err != nil {
			return nil, err
		}

		if patternIndex == directedPattern {
			if failureAlternative < 0 || failureAlternative >= len(machines) {
				return nil, errors.New("schematest: pattern failure alternative is out of range")
			}

			for index := range machines {
				switch {
				case index < failureAlternative:
				case index == failureAlternative:
					machines[index].expected = !machines[index].expected
				default:
					machines[index].required = false
				}
			}
		}

		product.machines = append(product.machines, machines...)
	}

	if directedPattern >= len(patterns) {
		return nil, errors.New("schematest: directed pattern is out of range")
	}

	if err := product.setBounds(); err != nil {
		return nil, err
	}

	return product, nil
}

func (product *basicStringProduct) setBounds() error {
	hasPositiveRequirement := false

	for index := range product.machines {
		machine := &product.machines[index]

		product.hasSurrogate = product.hasSurrogate || machine.hasSurrogate
		if !machine.required || !machine.expected {
			continue
		}

		hasPositiveRequirement = true

		if machine.unbounded {
			product.unbounded = true

			continue
		}

		if product.maxUnits > ^uint64(0)-machine.maxUnits {
			return errors.New("schematest: basic pattern witness length overflows uint64")
		}

		product.maxUnits += machine.maxUnits
	}

	if !hasPositiveRequirement {
		product.unbounded = true
	}

	return nil
}

func compileBasicStringPatternMachines(pattern *patternAST) ([]basicStringMachine, error) {
	if pattern == nil || pattern.expression == nil {
		return nil, errors.New("schematest: pattern is not a basic string expression")
	}

	machines := make([]basicStringMachine, 0, len(pattern.leadingAssertions)+1)
	for _, assertion := range pattern.leadingAssertions {
		machine, err := compileBasicStringExpressionMachine(assertion.expression, false, assertion.positive)
		if err != nil {
			return nil, err
		}

		machines = append(machines, machine)
	}

	machine, err := compileBasicStringExpressionMachine(
		pattern.expression,
		len(pattern.leadingAssertions) == 0,
		true,
	)
	if err != nil {
		return nil, err
	}

	return append(machines, machine), nil
}

func compileBasicStringExpressionMachine(
	expression *patternExpression,
	restart,
	expected bool,
) (basicStringMachine, error) {
	maximum, unbounded, ok := basicStringExpressionLength(expression)
	if !ok {
		return basicStringMachine{}, errors.New("schematest: pattern is not a searchable string expression")
	}

	machine := basicStringMachine{
		start:     0,
		accept:    1,
		maxUnits:  maximum,
		unbounded: unbounded,
		restart:   restart,
		expected:  expected,
		required:  true,
	}
	machine.states = make([][]basicStringEdge, machine.accept+1)

	if err := machine.compileExpression(expression, machine.start, machine.accept); err != nil {
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
		if term == nil || term.atom == nil {
			return errors.New("schematest: string pattern term is nil")
		}

		next := machine.newState()
		if err := machine.compileTerm(term, current, next); err != nil {
			return err
		}

		current = next
	}

	machine.addEdge(current, basicStringEdge{kind: basicStringEpsilon, target: to})

	return nil
}

func (machine *basicStringMachine) compileTerm(term *patternTerm, from, to int) error {
	if !term.quantified {
		return machine.compileAtom(term.atom, from, to)
	}

	current := from

	for range term.minimum {
		next := machine.newState()
		if err := machine.compileAtom(term.atom, current, next); err != nil {
			return err
		}

		current = next
	}

	if term.unbounded {
		machine.addEdge(current, basicStringEdge{kind: basicStringEpsilon, target: to})

		return machine.compileAtom(term.atom, current, current)
	}

	for count := term.minimum; count < term.maximum; count++ {
		next := machine.newState()
		machine.addEdge(current, basicStringEdge{kind: basicStringEpsilon, target: next})

		if err := machine.compileAtom(term.atom, current, next); err != nil {
			return err
		}

		current = next
	}

	machine.addEdge(current, basicStringEdge{kind: basicStringEpsilon, target: to})

	return nil
}

//nolint:cyclop,mnd // Admitted atom kinds and dot's exact boundaries are normative.
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
	case patternWordBoundary:
		machine.hasBoundary = true
		machine.addEdge(from, basicStringEdge{kind: basicStringWordBoundary, target: to})
	case patternNotWordBoundary:
		machine.hasBoundary = true
		machine.addEdge(from, basicStringEdge{kind: basicStringNotWordBoundary, target: to})
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
	if edge.kind == basicStringUnit && edge.low <= 0xdfff && edge.high >= 0xd800 {
		machine.hasSurrogate = true
	}
}

func basicStringExpressionLength(expression *patternExpression) (uint64, bool, bool) {
	if expression == nil {
		return 0, false, false
	}

	var (
		maximum   uint64
		unbounded bool
	)

	for _, alternative := range expression.alternatives {
		length, sequenceUnbounded, ok := basicStringSequenceLength(alternative)
		if !ok {
			return 0, false, false
		}

		maximum = max(maximum, length)
		unbounded = unbounded || sequenceUnbounded
	}

	return maximum, unbounded, true
}

//nolint:cyclop // Term validation and each admitted atom are one length calculation.
func basicStringSequenceLength(sequence *patternSequence) (uint64, bool, bool) {
	if sequence == nil {
		return 0, false, false
	}

	var (
		length    uint64
		unbounded bool
	)

	for _, term := range sequence.terms {
		if term == nil || term.atom == nil {
			return 0, false, false
		}

		var (
			atomLength    uint64
			atomUnbounded bool
		)

		switch term.atom.kind {
		case patternLiteral, patternDot, patternClassAtom:
			atomLength = 1
		case patternStart, patternEnd, patternWordBoundary, patternNotWordBoundary:
		case patternGroup:
			var ok bool

			atomLength, atomUnbounded, ok = basicStringExpressionLength(term.atom.expression)
			if !ok {
				return 0, false, false
			}
		default:
			return 0, false, false
		}

		if atomUnbounded || term.unbounded && atomLength != 0 {
			unbounded = true
		}

		factor := term.maximum
		if term.unbounded {
			factor = term.minimum
		}

		if atomLength != 0 && factor > ^uint64(0)/atomLength {
			return 0, false, false
		}

		termLength := atomLength * factor
		if length > ^uint64(0)-termLength {
			return 0, false, false
		}

		length += termLength
	}

	return length, unbounded, true
}

func (product *basicStringProduct) start(length int) basicStringProductState {
	state := basicStringProductState{
		patterns: make([]basicStringPatternState, len(product.machines)),
		length:   length,
	}
	for index := range product.machines {
		active := product.machines[index].closure(
			[]int{product.machines[index].start},
			0,
			length,
			false,
			false,
		)
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
	next := basicStringProductState{
		patterns:     make([]basicStringPatternState, len(product.machines)),
		position:     position + 1,
		length:       length,
		previousWord: isPatternWordUnit(unit),
	}

	for index := range product.machines {
		machine := &product.machines[index]
		patternState := state.patterns[index]
		active := machine.closure(
			patternState.active,
			position,
			length,
			state.previousWord != isPatternWordUnit(unit),
			true,
		)
		targets := make([]int, 0)
		matched := patternState.matched || containsBasicStringState(active, machine.accept)

		if !matched {
			for _, activeState := range active {
				for _, edge := range machine.states[activeState] {
					if edge.kind == basicStringUnit && unit >= edge.low && unit <= edge.high {
						targets = appendUniqueBasicStringState(targets, edge.target)
					}
				}
			}
		}

		targets = machine.appendRestartState(targets)
		targets = machine.closure(targets, position+1, length, false, false)
		next.patterns[index] = basicStringPatternState{
			active:  targets,
			matched: matched || containsBasicStringState(targets, machine.accept),
		}
	}

	return next
}

func (machine *basicStringMachine) appendRestartState(states []int) []int {
	if !machine.restart {
		return states
	}

	return appendUniqueBasicStringState(states, machine.start)
}

//nolint:cyclop // Zero-width edge conditions are deliberately explicit.
func (machine *basicStringMachine) closure(
	states []int,
	position,
	length int,
	wordBoundary,
	boundariesKnown bool,
) []int {
	closed := append([]int(nil), states...)

	seen := make([]bool, len(machine.states))
	for _, state := range closed {
		seen[state] = true
	}

	for index := 0; index < len(closed); index++ {
		for _, edge := range machine.states[closed[index]] {
			follow := edge.kind == basicStringEpsilon ||
				edge.kind == basicStringStart && position == 0 ||
				edge.kind == basicStringEnd && position == length ||
				boundariesKnown && edge.kind == basicStringWordBoundary && wordBoundary ||
				boundariesKnown && edge.kind == basicStringNotWordBoundary && !wordBoundary
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
	for patternIndex := range product.machines {
		machine := &product.machines[patternIndex]
		if !machine.required || state.patterns[patternIndex].matched {
			continue
		}

		if machine.hasBoundary {
			product.eachActiveUnitEdgeForWordState(state, machine, patternIndex, false, visit)
			product.eachActiveUnitEdgeForWordState(state, machine, patternIndex, true, visit)

			continue
		}

		for _, activeState := range state.patterns[patternIndex].active {
			for _, edge := range machine.states[activeState] {
				if edge.kind == basicStringUnit {
					visit(edge)
				}
			}
		}
	}
}

func (product *basicStringProduct) eachActiveUnitEdgeForWordState(
	state basicStringProductState,
	machine *basicStringMachine,
	patternIndex int,
	nextWord bool,
	visit func(basicStringEdge),
) {
	active := machine.closure(
		state.patterns[patternIndex].active,
		state.position,
		state.length,
		state.previousWord != nextWord,
		true,
	)

	for _, activeState := range active {
		for _, edge := range machine.states[activeState] {
			if edge.kind != basicStringUnit {
				continue
			}

			eachBasicStringWordIntersection(edge, nextWord, visit)
		}
	}
}

func eachBasicStringWordIntersection(edge basicStringEdge, word bool, visit func(basicStringEdge)) {
	ranges := []patternRange{
		{low: '0', high: '9'},
		{low: 'A', high: 'Z'},
		{low: '_', high: '_'},
		{low: 'a', high: 'z'},
	}
	if !word {
		ranges = complementBasicStringRanges(ranges)
	}

	for _, characterRange := range ranges {
		low := max(edge.low, characterRange.low)

		high := min(edge.high, characterRange.high)
		if low > high {
			continue
		}

		candidate := edge
		candidate.low = low
		candidate.high = high
		visit(candidate)
	}
}

func (product *basicStringProduct) eachTransition(
	state basicStringProductState,
	seed uint64,
	includePadding bool,
	visit func(uint16) bool,
) {
	if includePadding {
		var low uint32
		for low <= uint32(basicStringMaxUnit) {
			high := product.intervalHigh(state, uint16(low))
			if eachBasicStringIntervalCandidate(
				basicStringInterval{low: uint16(low), high: high}, seed, visit,
			) {
				return
			}

			low = uint32(high) + 1
		}

		return
	}

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
	for index, pattern := range state.patterns {
		if !product.machines[index].required {
			continue
		}

		matched := pattern.matched
		if !matched {
			machine := &product.machines[index]
			active := machine.closure(
				pattern.active,
				state.position,
				state.length,
				state.previousWord,
				true,
			)
			matched = containsBasicStringState(active, machine.accept)
		}

		if matched != product.machines[index].expected {
			return false
		}
	}

	return true
}

func (s *search) walkBasicStringWitnesses(patterns []*patternAST, seed uint64, visit rowVisit) (bool, error) {
	return s.walkBasicStringWitnessesForLengths(
		patterns,
		basicStringLengths{},
		basicStringLengthObjective{},
		seed,
		visit,
	)
}

func (s *search) walkBasicStringWitnessesForLengths(
	patterns []*patternAST,
	lengths basicStringLengths,
	objective basicStringLengthObjective,
	seed uint64,
	visit rowVisit,
) (bool, error) {
	product, err := newBasicStringProduct(patterns)
	if err != nil {
		return false, err
	}

	return s.walkBasicStringProductForLengths(product, lengths, objective, seed, visit)
}

func (s *search) walkBasicStringProductForLengths(
	product *basicStringProduct,
	lengths basicStringLengths,
	objective basicStringLengthObjective,
	seed uint64,
	visit rowVisit,
) (bool, error) {
	var (
		complete bool
		walkErr  error
	)

	lengths.each(product, objective, func(runeLength uint64) bool {
		if err := s.assign(); err != nil {
			walkErr = err

			return true
		}

		if !lengths.allows(runeLength) {
			return false
		}

		complete, walkErr = s.walkBasicStringRuneLength(product, runeLength, seed, visit)

		return walkErr != nil || complete
	})

	return complete, walkErr
}

func (s *search) walkBasicStringRuneLength(
	product *basicStringProduct,
	runeLength uint64,
	seed uint64,
	visit rowVisit,
) (bool, error) {
	maxInt := uint64(^uint(0) >> 1)
	if runeLength > maxInt {
		return false, nil
	}

	maximumUnits := runeLength
	if product.hasSurrogate {
		maximumUnits = runeLength * basicStringUnitsPerRune
		if runeLength > maxInt/basicStringUnitsPerRune {
			maximumUnits = maxInt
		}
	}

	for unitLength := runeLength; unitLength <= maximumUnits; unitLength++ {
		length := int(unitLength)
		units := make([]uint16, 0, length)
		includePadding := product.unbounded || unitLength > product.maxUnits

		complete, err := s.walkBasicStringProduct(
			product,
			product.start(length),
			units,
			length,
			int(runeLength),
			seed,
			includePadding,
			visit,
		)
		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}

func (s *search) walkBasicStringProduct(
	product *basicStringProduct,
	state basicStringProductState,
	units []uint16,
	length int,
	runeLength int,
	seed uint64,
	includePadding bool,
	visit rowVisit,
) (bool, error) {
	if len(units) == length {
		if !product.accepting(state) || !validBasicStringUnits(units) ||
			len(utf16.Decode(units)) != runeLength {
			return false, nil
		}

		return visit(&jsonValue{kind: jsonString, text: string(utf16.Decode(units))})
	}

	var (
		complete bool
		walkErr  error
	)

	product.eachTransition(state, seed, includePadding, func(unit uint16) bool {
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
			runeLength,
			seed,
			includePadding,
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
