//nolint:godoclint // Private product-graph state is documented at its search seams.
package schematest

import (
	"errors"
	"fmt"
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
	basicStringRepeatInit
	basicStringRepeatBody
	basicStringRepeatComplete
	basicStringRepeatExit
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
	frame  int
}

type basicStringInterval struct {
	low  uint16
	high uint16
}

type basicStringRepeat struct {
	minimum   uint64
	maximum   uint64
	unbounded bool
}

type basicStringRepeatState struct {
	count     uint64
	enteredAt int
	blockedAt int
}

type basicStringConfiguration struct {
	state   int
	repeats []basicStringRepeatState
}

type basicStringMachine struct {
	states        [][]basicStringEdge
	repeats       []basicStringRepeat
	start         int
	accept        int
	minUnits      uint64
	maxUnits      uint64
	unbounded     bool
	hasBoundary   bool
	hasSurrogate  bool
	restartStates []int
	expected      bool
	required      bool
	wholeBound    bool
}

type basicStringPatternState struct {
	active  []basicStringConfiguration
	matched bool
}

type basicStringProductState struct {
	patterns     []basicStringPatternState
	formats      []stringFormatProgramState
	position     int
	length       int
	previousWord bool
	pendingHigh  bool
}

type basicStringProduct struct {
	machines         []basicStringMachine
	formatPrograms   []*stringFormatProgram
	maxUnits         uint64
	unbounded        bool
	hasSurrogate     bool
	surrogatePadding bool
	needsPadding     bool
	formats          []activeStringFormat
	directedFormat   int
	guidance         *stringFormatBoundary
	objective        *stringFormatBoundary
}

type basicStringLengthObjective struct {
	length      uint64
	constrained bool
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
//nolint:cyclop // Constrained, boundary, and fair phases share exact first-occurrence deduplication.
func (lengths basicStringLengths) each(
	product *basicStringProduct,
	objective basicStringLengthObjective,
	visit func(uint64) bool,
) {
	special := make([]uint64, 0, len(lengths.boundaries)+1)
	if objective.constrained {
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

func newBasicStringProduct(patterns []*patternAST) (*basicStringProduct, error) {
	return newBasicStringProductForFailure(patterns, -1, -1)
}

//nolint:cyclop // Compilation, one directed alternative, and exact bounds are one product operation.
func newBasicStringProductForFailure(
	patterns []*patternAST,
	directedPattern,
	failureAlternative int,
) (*basicStringProduct, error) {
	product := &basicStringProduct{
		machines:       make([]basicStringMachine, 0, len(patterns)),
		directedFormat: -1,
	}
	if len(patterns) == 0 {
		product.unbounded = true
		product.needsPadding = true

		return product, nil
	}

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

//nolint:cyclop // Whole-string bounds and padding metadata share one machine pass.
func (product *basicStringProduct) setBounds() error {
	bounded := false
	product.maxUnits = 0
	product.unbounded = false
	product.hasSurrogate = false
	product.surrogatePadding = false
	product.needsPadding = len(product.machines) == 0

	for index := range product.machines {
		machine := &product.machines[index]
		product.hasSurrogate = product.hasSurrogate || machine.required && machine.hasSurrogate
		product.surrogatePadding = product.surrogatePadding || machine.required && !machine.expected
		product.needsPadding = product.needsPadding || machine.required && (!machine.expected || !machine.wholeBound)

		if !machine.required || !machine.expected || machine.unbounded || !machine.wholeBound {
			continue
		}

		if !bounded || machine.maxUnits < product.maxUnits {
			product.maxUnits = machine.maxUnits
			bounded = true
		}
	}

	product.unbounded = !bounded

	return nil
}

func compileBasicStringPatternMachines(pattern *patternAST) ([]basicStringMachine, error) {
	if pattern == nil || pattern.expression == nil {
		return nil, errors.New("schematest: pattern is not a basic string expression")
	}

	if pattern.searchMachines == nil {
		return nil, errors.New("schematest: pattern has no compiled search program")
	}

	return append([]basicStringMachine(nil), pattern.searchMachines...), nil
}

func compileBasicStringPatternMachinesUncached(pattern *patternAST) ([]basicStringMachine, error) {
	if pattern == nil || pattern.expression == nil {
		return nil, errors.New("schematest: pattern is not a basic string expression")
	}

	machines := make([]basicStringMachine, 0, len(pattern.leadingAssertions)+1)
	for _, assertion := range pattern.leadingAssertions {
		machine, err := compileBasicStringExpressionMachine(assertion.expression, assertion.positive)
		if err != nil {
			return nil, err
		}

		machines = append(machines, machine)
	}

	machine, err := compileBasicStringTopLevelMachine(pattern.expression)
	if err != nil {
		return nil, err
	}

	return append(machines, machine), nil
}

func compileBasicStringExpressionMachine(
	expression *patternExpression,
	expected bool,
) (basicStringMachine, error) {
	machine, err := newBasicStringMachine(expression, expected)
	if err != nil {
		return basicStringMachine{}, err
	}

	if err := machine.compileExpression(expression, machine.start, machine.accept); err != nil {
		return basicStringMachine{}, err
	}

	machine.wholeBound = expected && basicStringExpressionEndAnchored(expression)

	return machine, nil
}

func compileBasicStringTopLevelMachine(expression *patternExpression) (basicStringMachine, error) {
	machine, err := newBasicStringMachine(expression, true)
	if err != nil {
		return basicStringMachine{}, err
	}

	machine.wholeBound = len(expression.alternatives) > 0
	for _, alternative := range expression.alternatives {
		alternativeStart := machine.newState()
		alternativeEnd := machine.newState()
		machine.addEdge(machine.start, basicStringEdge{kind: basicStringEpsilon, target: alternativeStart})

		if err := machine.compileSequence(alternative, alternativeStart, alternativeEnd); err != nil {
			return basicStringMachine{}, err
		}

		machine.addEdge(alternativeEnd, basicStringEdge{kind: basicStringEpsilon, target: machine.accept})

		startAnchored, endAnchored := basicStringSequenceAnchors(alternative)
		if !startAnchored {
			machine.restartStates = append(machine.restartStates, alternativeStart)
		}

		machine.wholeBound = machine.wholeBound && startAnchored && endAnchored
	}

	return machine, nil
}

func newBasicStringMachine(expression *patternExpression, expected bool) (basicStringMachine, error) {
	bounds, ok := basicStringExpressionBounds(expression)
	if !ok {
		return basicStringMachine{}, errors.New("schematest: pattern is not a searchable string expression")
	}

	machine := basicStringMachine{
		start:     0,
		accept:    1,
		minUnits:  bounds.minimum,
		maxUnits:  bounds.maximum,
		unbounded: bounds.unbounded,
		expected:  expected,
		required:  true,
	}
	machine.states = make([][]basicStringEdge, machine.accept+1)

	return machine, nil
}

func basicStringExpressionEndAnchored(expression *patternExpression) bool {
	_, end := basicStringExpressionAnchors(expression, make(map[*patternAtom][2]bool))

	return end
}

func basicStringSequenceAnchors(sequence *patternSequence) (bool, bool) {
	return basicStringSequenceAnchorsWithMemo(sequence, make(map[*patternAtom][2]bool))
}

func basicStringExpressionAnchors(
	expression *patternExpression,
	memo map[*patternAtom][2]bool,
) (bool, bool) {
	if expression == nil || len(expression.alternatives) == 0 {
		return false, false
	}

	start, end := true, true

	for _, alternative := range expression.alternatives {
		alternativeStart, alternativeEnd := basicStringSequenceAnchorsWithMemo(alternative, memo)
		start = start && alternativeStart
		end = end && alternativeEnd
	}

	return start, end
}

func basicStringSequenceAnchorsWithMemo(
	sequence *patternSequence,
	memo map[*patternAtom][2]bool,
) (bool, bool) {
	if sequence == nil || len(sequence.terms) == 0 {
		return false, false
	}

	start := false

	for _, term := range sequence.terms {
		termStart, _ := basicStringTermAnchors(term, memo)
		if termStart {
			start = true

			break
		}

		if !basicStringTermNullable(term) {
			break
		}
	}

	end := false

	for index := len(sequence.terms) - 1; index >= 0; index-- {
		term := sequence.terms[index]

		_, termEnd := basicStringTermAnchors(term, memo)
		if termEnd {
			end = true

			break
		}

		if !basicStringTermNullable(term) {
			break
		}
	}

	return start, end
}

func basicStringTermNullable(term *patternTerm) bool {
	if term == nil || term.atom == nil || term.quantified && term.minimum == 0 {
		return true
	}

	switch term.atom.kind {
	case patternStart, patternEnd, patternWordBoundary, patternNotWordBoundary:
		return true
	case patternGroup:
		bounds, ok := basicStringExpressionBounds(term.atom.expression)

		return ok && bounds.minimum == 0
	default:
		return false
	}
}

func basicStringTermAnchors(term *patternTerm, memo map[*patternAtom][2]bool) (bool, bool) {
	if term == nil || term.atom == nil || term.quantified && term.minimum == 0 {
		return false, false
	}

	if anchors, exists := memo[term.atom]; exists {
		return anchors[0], anchors[1]
	}

	start, end := term.atom.kind == patternStart, term.atom.kind == patternEnd
	if term.atom.kind == patternGroup {
		start, end = basicStringExpressionAnchors(term.atom.expression, memo)
	}

	memo[term.atom] = [2]bool{start, end}

	return start, end
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

	frame := len(machine.repeats)
	machine.repeats = append(machine.repeats, basicStringRepeat{
		minimum: term.minimum, maximum: term.maximum, unbounded: term.unbounded,
	})

	decision := machine.newState()
	bodyStart := machine.newState()
	bodyEnd := machine.newState()
	machine.addEdge(from, basicStringEdge{kind: basicStringRepeatInit, target: decision, frame: frame})

	body := basicStringEdge{kind: basicStringRepeatBody, target: bodyStart, frame: frame}
	exit := basicStringEdge{kind: basicStringRepeatExit, target: to, frame: frame}

	if term.greedy {
		machine.addEdge(decision, body)
		machine.addEdge(decision, exit)
	} else {
		machine.addEdge(decision, exit)
		machine.addEdge(decision, body)
	}

	if err := machine.compileAtom(term.atom, bodyStart, bodyEnd); err != nil {
		return err
	}

	machine.addEdge(bodyEnd, basicStringEdge{
		kind: basicStringRepeatComplete, target: decision, frame: frame,
	})

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

type basicStringLengthBounds struct {
	minimum   uint64
	maximum   uint64
	unbounded bool
}

func basicStringExpressionLength(expression *patternExpression) (uint64, bool, bool) {
	bounds, ok := basicStringExpressionBounds(expression)
	if !ok {
		return 0, false, false
	}

	return bounds.maximum, bounds.unbounded, true
}

func basicStringExpressionBounds(expression *patternExpression) (basicStringLengthBounds, bool) {
	if expression == nil || len(expression.alternatives) == 0 {
		return basicStringLengthBounds{}, false
	}

	bounds := basicStringLengthBounds{minimum: ^uint64(0)}

	for _, alternative := range expression.alternatives {
		alternativeBounds, ok := basicStringSequenceBounds(alternative)
		if !ok {
			return basicStringLengthBounds{}, false
		}

		bounds.minimum = min(bounds.minimum, alternativeBounds.minimum)
		bounds.maximum = max(bounds.maximum, alternativeBounds.maximum)
		bounds.unbounded = bounds.unbounded || alternativeBounds.unbounded
	}

	return bounds, true
}

func basicStringSequenceLength(sequence *patternSequence) (uint64, bool, bool) {
	bounds, ok := basicStringSequenceBounds(sequence)
	if !ok {
		return 0, false, false
	}

	return bounds.maximum, bounds.unbounded, true
}

//nolint:cyclop // Term validation and each admitted atom are one length calculation.
func basicStringSequenceBounds(sequence *patternSequence) (basicStringLengthBounds, bool) {
	if sequence == nil {
		return basicStringLengthBounds{}, false
	}

	var bounds basicStringLengthBounds

	for _, term := range sequence.terms {
		if term == nil || term.atom == nil {
			return basicStringLengthBounds{}, false
		}

		atomBounds := basicStringLengthBounds{}

		switch term.atom.kind {
		case patternLiteral, patternDot, patternClassAtom:
			atomBounds.minimum, atomBounds.maximum = 1, 1
		case patternStart, patternEnd, patternWordBoundary, patternNotWordBoundary:
		case patternGroup:
			var ok bool

			atomBounds, ok = basicStringExpressionBounds(term.atom.expression)
			if !ok {
				return basicStringLengthBounds{}, false
			}
		default:
			return basicStringLengthBounds{}, false
		}

		minimumFactor, maximumFactor := uint64(1), uint64(1)
		if term.quantified {
			minimumFactor, maximumFactor = term.minimum, term.maximum
		}

		termBounds, ok := multiplyBasicStringBounds(atomBounds, minimumFactor, maximumFactor, term.unbounded)
		if !ok || bounds.minimum > ^uint64(0)-termBounds.minimum ||
			bounds.maximum > ^uint64(0)-termBounds.maximum {
			return basicStringLengthBounds{}, false
		}

		bounds.minimum += termBounds.minimum
		bounds.maximum += termBounds.maximum
		bounds.unbounded = bounds.unbounded || termBounds.unbounded
	}

	return bounds, true
}

func multiplyBasicStringBounds(
	atom basicStringLengthBounds,
	minimum,
	maximum uint64,
	unbounded bool,
) (basicStringLengthBounds, bool) {
	if maximum == 0 && !unbounded {
		return basicStringLengthBounds{}, true
	}

	if atom.minimum != 0 && minimum > ^uint64(0)/atom.minimum {
		return basicStringLengthBounds{}, false
	}

	result := basicStringLengthBounds{minimum: atom.minimum * minimum}

	result.unbounded = atom.unbounded || unbounded && atom.maximum != 0
	if result.unbounded {
		return result, true
	}

	if atom.maximum != 0 && maximum > ^uint64(0)/atom.maximum {
		return basicStringLengthBounds{}, false
	}

	result.maximum = atom.maximum * maximum

	return result, true
}

func (product *basicStringProduct) start(length int) basicStringProductState {
	state := basicStringProductState{
		patterns: make([]basicStringPatternState, len(product.machines)),
		formats:  make([]stringFormatProgramState, len(product.formatPrograms)),
		length:   length,
	}
	for index := range product.machines {
		machine := &product.machines[index]
		active := machine.closure(
			[]basicStringConfiguration{machine.initialConfiguration(machine.start)},
			0,
			length,
			false,
			false,
		)
		state.patterns[index] = basicStringPatternState{
			active:  active,
			matched: containsBasicStringState(active, machine.accept),
		}
	}

	for index, program := range product.formatPrograms {
		if program != nil {
			state.formats[index] = program.start(length)
		}
	}

	return state
}

//nolint:cyclop // Product transition and zero-width closure are one state operation.
func (product *basicStringProduct) advance(
	state basicStringProductState,
	unit uint16,
	position,
	length int,
) basicStringProductState {
	next := basicStringProductState{
		patterns:     make([]basicStringPatternState, len(product.machines)),
		formats:      make([]stringFormatProgramState, len(product.formatPrograms)),
		position:     position + 1,
		length:       length,
		previousWord: isPatternWordUnit(unit),
		pendingHigh:  unit >= 0xd800 && unit <= 0xdbff,
	}

	for index, program := range product.formatPrograms {
		if program != nil {
			next.formats[index] = program.advance(state.formats[index], unit)
		}
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
		targets := make([]basicStringConfiguration, 0)
		matched := patternState.matched || containsBasicStringState(active, machine.accept)

		if !matched {
			for _, activeState := range active {
				for _, edge := range machine.states[activeState.state] {
					if edge.kind == basicStringUnit && unit >= edge.low && unit <= edge.high {
						target := activeState
						target.state = edge.target
						targets = appendUniqueBasicStringConfiguration(targets, target)
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

func (machine *basicStringMachine) initialConfiguration(state int) basicStringConfiguration {
	configuration := basicStringConfiguration{
		state: state, repeats: make([]basicStringRepeatState, len(machine.repeats)),
	}
	for index := range configuration.repeats {
		configuration.repeats[index].enteredAt = -1
		configuration.repeats[index].blockedAt = -1
	}

	return configuration
}

func (machine *basicStringMachine) appendRestartState(
	states []basicStringConfiguration,
) []basicStringConfiguration {
	for _, restartState := range machine.restartStates {
		states = appendUniqueBasicStringConfiguration(states, machine.initialConfiguration(restartState))
	}

	return states
}

func (machine *basicStringMachine) closure(
	states []basicStringConfiguration,
	position,
	length int,
	wordBoundary,
	boundariesKnown bool,
) []basicStringConfiguration {
	closed := append([]basicStringConfiguration(nil), states...)

	for index := 0; index < len(closed); index++ {
		configuration := closed[index]
		for _, edge := range machine.states[configuration.state] {
			next, follow := machine.followZeroWidth(
				configuration, edge, position, length, wordBoundary, boundariesKnown,
			)
			if !follow {
				continue
			}

			closed = appendUniqueBasicStringConfiguration(closed, next)
		}
	}

	return closed
}

//nolint:cyclop // Each edge kind has one exact zero-width transition rule.
func (machine *basicStringMachine) followZeroWidth(
	configuration basicStringConfiguration,
	edge basicStringEdge,
	position,
	length int,
	wordBoundary,
	boundariesKnown bool,
) (basicStringConfiguration, bool) {
	next := configuration
	next.state = edge.target

	switch edge.kind {
	case basicStringEpsilon:
		return next, true
	case basicStringStart:
		return next, position == 0
	case basicStringEnd:
		return next, position == length
	case basicStringWordBoundary:
		return next, boundariesKnown && wordBoundary
	case basicStringNotWordBoundary:
		return next, boundariesKnown && !wordBoundary
	case basicStringRepeatInit:
		next.repeats = cloneBasicStringRepeatStates(configuration.repeats)
		next.repeats[edge.frame] = basicStringRepeatState{enteredAt: -1, blockedAt: -1}

		return next, true
	case basicStringRepeatBody:
		repeat := machine.repeats[edge.frame]

		state := configuration.repeats[edge.frame]
		if !repeat.unbounded && state.count >= repeat.maximum ||
			state.count >= repeat.minimum && state.blockedAt == position {
			return basicStringConfiguration{}, false
		}

		next.repeats = cloneBasicStringRepeatStates(configuration.repeats)
		next.repeats[edge.frame].enteredAt = position

		return next, true
	case basicStringRepeatComplete:
		state := configuration.repeats[edge.frame]
		if state.count == ^uint64(0) {
			return basicStringConfiguration{}, false
		}

		next.repeats = cloneBasicStringRepeatStates(configuration.repeats)

		next.repeats[edge.frame].count++
		if state.enteredAt == position {
			next.repeats[edge.frame].blockedAt = position
		} else {
			next.repeats[edge.frame].blockedAt = -1
		}

		return next, true
	case basicStringRepeatExit:
		return next, configuration.repeats[edge.frame].count >= machine.repeats[edge.frame].minimum
	default:
		return basicStringConfiguration{}, false
	}
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

//nolint:cyclop // Structural, boundary, and active-edge partitions share one ordered endpoint.
func (product *basicStringProduct) intervalHigh(state basicStringProductState, low uint16) uint16 {
	high := basicStringMaxUnit

	boundaries := []uint16{0xd7ff, 0xdbff, 0xdfff}

	for _, machine := range product.machines {
		if machine.required && machine.hasBoundary {
			boundaries = []uint16{47, 57, 64, 90, 94, 95, 96, 122, 0xd7ff, 0xdbff, 0xdfff}

			break
		}
	}

	for _, boundary := range boundaries {
		if low <= boundary {
			high = boundary

			break
		}
	}

	product.eachActiveUnitEdge(state, func(edge basicStringEdge) {
		switch {
		case low >= edge.low && low <= edge.high && edge.high < high:
			high = edge.high
		case edge.low > low && edge.low-1 < high:
			high = edge.low - 1
		}
	})

	for index, program := range product.formatPrograms {
		if program == nil {
			continue
		}

		class := program.transition(state.formats[index], low)
		maximumUnit := uint16(0)

		program.eachUnit(func(unit uint16) {
			maximumUnit = max(maximumUnit, unit)
		})

		for unit := uint32(low) + 1; unit <= uint32(maximumUnit)+1 && unit <= uint32(high); unit++ {
			if program.transition(state.formats[index], uint16(unit)) != class {
				high = uint16(unit - 1)

				break
			}
		}
	}

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
			for _, edge := range machine.states[activeState.state] {
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
		for _, edge := range machine.states[activeState.state] {
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

//nolint:cyclop,gocognit,mnd // UTF-16 structural boundaries and deterministic padding are explicit.
func (product *basicStringProduct) eachTransition(
	state basicStringProductState,
	seed uint64,
	includePadding bool,
	visit func(uint16) bool,
) {
	preferred := uint16(0)

	hasPreferred := product.guidance != nil && state.position < len(product.guidance.witness)
	if hasPreferred {
		preferred = uint16(product.guidance.witness[state.position])
	} else {
		for _, program := range product.formatPrograms {
			if program != nil && program.preferred != nil {
				preferred = program.preferred(state.position, state.length)
				hasPreferred = true

				break
			}
		}
	}

	preferredVisited := false
	visitWellFormed := func(unit uint16) bool {
		if preferredVisited && unit == preferred {
			return false
		}

		preferredVisited = preferredVisited || hasPreferred && unit == preferred
		low := unit >= 0xdc00 && unit <= 0xdfff

		high := unit >= 0xd800 && unit <= 0xdbff
		if state.pendingHigh != low || high && state.position+1 >= state.length {
			return false
		}

		return visit(unit)
	}

	if hasPreferred && visitWellFormed(preferred) {
		return
	}

	visitWellFormed = func(unit uint16) bool {
		if hasPreferred && unit == preferred {
			return false
		}

		low := unit >= 0xdc00 && unit <= 0xdfff

		high := unit >= 0xd800 && unit <= 0xdbff
		if state.pendingHigh != low || high && state.position+1 >= state.length {
			return false
		}

		return visit(unit)
	}

	if includePadding {
		var low uint32
		for low <= uint32(basicStringMaxUnit) {
			high := product.intervalHigh(state, uint16(low))
			if eachBasicStringIntervalCandidate(
				basicStringInterval{low: uint16(low), high: high}, seed, visitWellFormed,
			) {
				return
			}

			low = uint32(high) + 1
		}

		return
	}

	product.eachInterval(state, func(interval basicStringInterval) bool {
		return eachBasicStringIntervalCandidate(interval, seed, visitWellFormed)
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

func (product *basicStringProduct) viable(state basicStringProductState) bool {
	for index, formatState := range state.formats {
		if index != product.directedFormat && product.formatPrograms[index] != nil && !formatState.alive {
			return false
		}
	}

	return product.patternsViable(state.patterns)
}

func (product *basicStringProduct) patternsViable(patterns []basicStringPatternState) bool {
	for index, pattern := range patterns {
		machine := &product.machines[index]
		if !machine.required {
			continue
		}

		if !machine.expected && pattern.matched ||
			machine.expected && !pattern.matched && len(pattern.active) == 0 {
			return false
		}
	}

	return true
}

func (product *basicStringProduct) accepting(state basicStringProductState) bool {
	for index, program := range product.formatPrograms {
		if program != nil && program.accept(state.formats[index]) == (index == product.directedFormat) {
			return false
		}
	}

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

//nolint:cyclop // Rune/unit lengths and cutoff-safe traversal form one cursor boundary.
func (s *search) walkBasicStringRuneLength(
	product *basicStringProduct,
	runeLength uint64,
	seed uint64,
	visit rowVisit,
) (bool, error) {
	if !product.formatsAllowLength(runeLength) {
		return false, nil
	}

	maxInt := uint64(^uint(0) >> 1)
	if runeLength > maxInt {
		return false, nil
	}

	maximumUnits := runeLength
	if product.hasSurrogate || product.surrogatePadding && runeLength == 1 {
		if runeLength > maxInt/basicStringUnitsPerRune {
			maximumUnits = maxInt
		} else {
			maximumUnits = runeLength * basicStringUnitsPerRune
		}
	}

	for unitLength := runeLength; unitLength <= maximumUnits; unitLength++ {
		length := int(unitLength)

		var units []uint16

		includePadding := product.needsPadding || !product.unbounded && unitLength > product.maxUnits

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

		candidate := string(utf16.Decode(units))
		if product.objective != nil && !product.objective.matches(candidate) {
			return false, nil
		}

		return visit(&jsonValue{kind: jsonString, text: candidate})
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

		nextState := product.advance(state, unit, len(units), length)
		if !product.viable(nextState) {
			return false
		}

		nextUnits := append(units, unit)
		complete, walkErr = s.walkBasicStringProduct(
			product,
			nextState,
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

func cloneBasicStringRepeatStates(states []basicStringRepeatState) []basicStringRepeatState {
	return append([]basicStringRepeatState(nil), states...)
}

func appendUniqueBasicStringConfiguration(
	states []basicStringConfiguration,
	state basicStringConfiguration,
) []basicStringConfiguration {
	for _, existing := range states {
		if equalBasicStringConfiguration(existing, state) {
			return states
		}
	}

	state.repeats = cloneBasicStringRepeatStates(state.repeats)

	return append(states, state)
}

func equalBasicStringConfiguration(left, right basicStringConfiguration) bool {
	if left.state != right.state || len(left.repeats) != len(right.repeats) {
		return false
	}

	for index := range left.repeats {
		if left.repeats[index] != right.repeats[index] {
			return false
		}
	}

	return true
}

func containsBasicStringState(states []basicStringConfiguration, wanted int) bool {
	for _, state := range states {
		if state.state == wanted {
			return true
		}
	}

	return false
}
