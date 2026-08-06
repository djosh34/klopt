//nolint:cyclop,gocognit,godoclint,mnd,wsl_v5 // The private graph mirrors the admitted clean pattern AST directly.
package schematest

import (
	"errors"
	"fmt"
	"sort"
)

const stringUTF16UnitCount = 1 << 16

type stringUnitInterval struct {
	low  uint16
	high uint16
}

type stringPatternAssertion uint8

const (
	stringAssertionNone stringPatternAssertion = iota
	stringAssertionStart
	stringAssertionEnd
	stringAssertionWordBoundary
	stringAssertionNotWordBoundary
)

type stringPatternEpsilon struct {
	target    int
	assertion stringPatternAssertion
}

type stringPatternEdge struct {
	target    int
	intervals []stringUnitInterval
}

type stringPatternState struct {
	eps    []stringPatternEpsilon
	edges  []stringPatternEdge
	accept bool
}

type stringPatternGraph struct {
	states []stringPatternState
	start  int
}

type stringPatternFragment struct {
	start int
	end   int
}

type stringPatternGraphBuilder struct {
	graph stringPatternGraph
}

// compileStringPatternGraph compiles one admitted AST into a clean Thompson graph.
func compileStringPatternGraph(pattern *patternAST) (*stringPatternGraph, error) {
	if pattern == nil || pattern.expression == nil {
		return nil, errors.New("string pattern has no expression")
	}

	return compileStringPatternExpressionGraph(pattern.expression, false)
}

// compileStringPatternExpressionGraph compiles one expression with optional input anchoring.
func compileStringPatternExpressionGraph(expression *patternExpression, anchored bool) (*stringPatternGraph, error) {
	if expression == nil {
		return nil, errors.New("string pattern expression is nil")
	}

	builder := stringPatternGraphBuilder{}
	fragment, err := builder.compileExpression(expression)
	if err != nil {
		return nil, err
	}

	builder.graph.start = fragment.start
	if anchored {
		start := builder.newState()
		builder.addEpsilon(start, fragment.start, stringAssertionStart)
		builder.graph.start = start
	}

	builder.graph.states[fragment.end].accept = true

	if !anchored && stringPatternStartsAnywhere(expression) {
		builder.addEdge(builder.graph.start, builder.graph.start, stringAllUnitIntervals())
	}

	builder.addEdge(fragment.end, fragment.end, stringAllUnitIntervals())

	return &builder.graph, nil
}

func (builder *stringPatternGraphBuilder) newState() int {
	builder.graph.states = append(builder.graph.states, stringPatternState{})

	return len(builder.graph.states) - 1
}

func (builder *stringPatternGraphBuilder) addEpsilon(
	from, to int,
	assertion stringPatternAssertion,
) {
	builder.graph.states[from].eps = append(
		builder.graph.states[from].eps,
		stringPatternEpsilon{target: to, assertion: assertion},
	)
}

func (builder *stringPatternGraphBuilder) addEdge(
	from, to int,
	intervals []stringUnitInterval,
) {
	if len(intervals) == 0 {
		return
	}

	builder.graph.states[from].edges = append(
		builder.graph.states[from].edges,
		stringPatternEdge{target: to, intervals: intervals},
	)
}

func (builder *stringPatternGraphBuilder) compileExpression(
	expression *patternExpression,
) (stringPatternFragment, error) {
	if expression == nil {
		return stringPatternFragment{}, errors.New("string pattern expression is nil")
	}

	start := builder.newState()
	end := builder.newState()

	for _, alternative := range expression.alternatives {
		fragment, err := builder.compileSequence(alternative)
		if err != nil {
			return stringPatternFragment{}, err
		}

		builder.addEpsilon(start, fragment.start, stringAssertionNone)
		builder.addEpsilon(fragment.end, end, stringAssertionNone)
	}

	return stringPatternFragment{start: start, end: end}, nil
}

func (builder *stringPatternGraphBuilder) compileSequence(
	sequence *patternSequence,
) (stringPatternFragment, error) {
	if sequence == nil {
		return stringPatternFragment{}, errors.New("string pattern sequence is nil")
	}

	start := builder.newState()
	current := start

	for _, term := range sequence.terms {
		fragment, err := builder.compileTerm(term)
		if err != nil {
			return stringPatternFragment{}, err
		}

		builder.addEpsilon(current, fragment.start, stringAssertionNone)
		current = fragment.end
	}

	return stringPatternFragment{start: start, end: current}, nil
}

func (builder *stringPatternGraphBuilder) compileTerm(
	term *patternTerm,
) (stringPatternFragment, error) {
	if term == nil || term.atom == nil {
		return stringPatternFragment{}, errors.New("string pattern term has no atom")
	}

	if !term.quantified {
		return builder.compileAtom(term.atom)
	}

	start := builder.newState()
	current := start

	for count := uint64(0); count < term.minimum; count++ {
		fragment, err := builder.compileAtom(term.atom)
		if err != nil {
			return stringPatternFragment{}, err
		}

		builder.addEpsilon(current, fragment.start, stringAssertionNone)
		current = fragment.end
	}

	if term.unbounded {
		end := builder.newState()
		fragment, err := builder.compileAtom(term.atom)
		if err != nil {
			return stringPatternFragment{}, err
		}

		builder.addEpsilon(current, end, stringAssertionNone)
		builder.addEpsilon(current, fragment.start, stringAssertionNone)
		builder.addEpsilon(fragment.end, current, stringAssertionNone)

		return stringPatternFragment{start: start, end: end}, nil
	}

	optional := term.maximum - term.minimum
	for count := uint64(0); count < optional; count++ {
		end := builder.newState()
		fragment, err := builder.compileAtom(term.atom)
		if err != nil {
			return stringPatternFragment{}, err
		}

		builder.addEpsilon(current, end, stringAssertionNone)
		builder.addEpsilon(current, fragment.start, stringAssertionNone)
		builder.addEpsilon(fragment.end, end, stringAssertionNone)
		current = end
	}

	return stringPatternFragment{start: start, end: current}, nil
}

func (builder *stringPatternGraphBuilder) compileAtom(
	atom *patternAtom,
) (stringPatternFragment, error) {
	if atom == nil {
		return stringPatternFragment{}, errors.New("string pattern atom is nil")
	}

	start := builder.newState()
	end := builder.newState()

	switch atom.kind {
	case patternLiteral:
		builder.addEdge(start, end, []stringUnitInterval{{low: atom.literal, high: atom.literal}})
	case patternDot:
		builder.addEdge(start, end, stringDotIntervals())
	case patternClassAtom:
		builder.addEdge(start, end, stringClassIntervals(atom.class))
	case patternStart:
		builder.addEpsilon(start, end, stringAssertionStart)
	case patternEnd:
		builder.addEpsilon(start, end, stringAssertionEnd)
	case patternWordBoundary:
		builder.addEpsilon(start, end, stringAssertionWordBoundary)
	case patternNotWordBoundary:
		builder.addEpsilon(start, end, stringAssertionNotWordBoundary)
	case patternGroup:
		fragment, err := builder.compileExpression(atom.expression)
		if err != nil {
			return stringPatternFragment{}, err
		}

		builder.addEpsilon(start, fragment.start, stringAssertionNone)
		builder.addEpsilon(fragment.end, end, stringAssertionNone)
	default:
		return stringPatternFragment{}, fmt.Errorf("unsupported string pattern atom %d", atom.kind)
	}

	return stringPatternFragment{start: start, end: end}, nil
}

// stringPatternAssertionRuntime carries one leading assertion's raw NFA state set.
type stringPatternAssertionRuntime struct {
	graph    *stringPatternGraph
	raw      []int
	positive bool
	matched  bool
}

// stringPatternRuntime carries one path's raw NFA state set.
type stringPatternRuntime struct {
	graph      *stringPatternGraph
	raw        []int
	assertions []stringPatternAssertionRuntime
}

func newStringPatternRuntime(graph *stringPatternGraph) stringPatternRuntime {
	return stringPatternRuntime{graph: graph, raw: []int{graph.start}}
}

func (runtime stringPatternAssertionRuntime) acceptsAt(
	position int,
	previous uint16,
	hasPrevious bool,
	current *uint16,
	atEnd bool,
) bool {
	return stringPatternRuntime{graph: runtime.graph, raw: runtime.raw}.acceptsAt(
		position, previous, hasPrevious, current, atEnd,
	)
}

func (runtime stringPatternRuntime) outgoing(
	position int,
	previous uint16,
	hasPrevious bool,
) []stringPatternTransition {
	if runtime.graph == nil {
		return nil
	}

	boundaries := []int{0, stringUTF16UnitCount}
	for _, state := range runtime.graph.states {
		for _, edge := range state.edges {
			for _, interval := range edge.intervals {
				boundaries = append(boundaries, int(interval.low), int(interval.high)+1)
			}
		}
	}

	// Word-boundary epsilon transitions change only at ASCII word-category edges.
	boundaries = append(boundaries, 48, 58, 65, 91, 95, 97, 123)
	sort.Ints(boundaries)
	boundaries = uniqueInts(boundaries)

	transitions := make([]stringPatternTransition, 0, len(boundaries))
	for index := 0; index+1 < len(boundaries); index++ {
		low := boundaries[index]
		high := boundaries[index+1] - 1
		if low > high || low >= stringUTF16UnitCount {
			continue
		}

		if high >= stringUTF16UnitCount {
			high = stringUTF16UnitCount - 1
		}

		unit := uint16(low)
		closure := runtime.closure(position, previous, hasPrevious, &unit, false)
		targets := make([]int, 0)

		for _, stateID := range closure {
			if stateID < 0 || stateID >= len(runtime.graph.states) {
				continue
			}

			for _, edge := range runtime.graph.states[stateID].edges {
				if intervalContains(edge.intervals, unit) {
					targets = appendUniqueInt(targets, edge.target)
				}
			}
		}

		if len(targets) == 0 {
			continue
		}

		transition := stringPatternTransition{
			interval: stringUnitInterval{low: unit, high: uint16(high)},
			targets:  sortedInts(targets),
		}

		if len(transitions) > 0 &&
			transitions[len(transitions)-1].interval.high+1 == transition.interval.low &&
			intSlicesEqual(transitions[len(transitions)-1].targets, transition.targets) {
			transitions[len(transitions)-1].interval.high = transition.interval.high

			continue
		}

		transitions = append(transitions, transition)
	}

	return transitions
}

func (runtime stringPatternRuntime) closure(
	position int,
	previous uint16,
	hasPrevious bool,
	current *uint16,
	atEnd bool,
) []int {
	if runtime.graph == nil {
		return nil
	}

	states := make([]int, 0, len(runtime.raw)+1)
	for _, stateID := range runtime.raw {
		states = appendUniqueInt(states, stateID)
	}
	states = appendUniqueInt(states, runtime.graph.start)

	for index := 0; index < len(states); index++ {
		stateID := states[index]
		if stateID < 0 || stateID >= len(runtime.graph.states) {
			continue
		}

		for _, epsilon := range runtime.graph.states[stateID].eps {
			if !stringAssertionAllows(epsilon.assertion, position, previous, hasPrevious, current, atEnd) {
				continue
			}

			states = appendUniqueInt(states, epsilon.target)
		}
	}

	return sortedInts(states)
}

func (runtime stringPatternRuntime) accepts(
	position int,
	previous uint16,
	hasPrevious bool,
) bool {
	return runtime.acceptsAt(position, previous, hasPrevious, nil, true)
}

func (runtime stringPatternRuntime) acceptsAt(
	position int,
	previous uint16,
	hasPrevious bool,
	current *uint16,
	atEnd bool,
) bool {
	if runtime.graph == nil {
		return false
	}

	for _, stateID := range runtime.closure(position, previous, hasPrevious, current, atEnd) {
		if stateID >= 0 && stateID < len(runtime.graph.states) && runtime.graph.states[stateID].accept {
			return true
		}
	}

	return false
}

type stringPatternTransition struct {
	interval stringUnitInterval
	targets  []int
}

func stringPatternStartsAnywhere(expression *patternExpression) bool {
	if expression == nil || len(expression.alternatives) == 0 {
		return true
	}

	for _, alternative := range expression.alternatives {
		if alternative == nil || len(alternative.terms) == 0 || alternative.terms[0] == nil ||
			alternative.terms[0].atom == nil || alternative.terms[0].atom.kind != patternStart {
			return true
		}
	}

	return false
}

func stringAssertionAllows(
	assertion stringPatternAssertion,
	position int,
	previous uint16,
	hasPrevious bool,
	current *uint16,
	atEnd bool,
) bool {
	switch assertion {
	case stringAssertionNone:
		return true
	case stringAssertionStart:
		return position == 0
	case stringAssertionEnd:
		return atEnd
	case stringAssertionWordBoundary, stringAssertionNotWordBoundary:
		if current == nil && !atEnd {
			return false
		}

		previousWord := hasPrevious && isPatternWordUnit(previous)
		currentWord := current != nil && isPatternWordUnit(*current)
		boundary := previousWord != currentWord

		return boundary == (assertion == stringAssertionWordBoundary)
	default:
		return false
	}
}

func stringDotIntervals() []stringUnitInterval {
	return []stringUnitInterval{
		{low: 0, high: 0x0009},
		{low: 0x000b, high: 0x000c},
		{low: 0x000e, high: 0x2028 - 1},
		{low: 0x2029 + 1, high: 0xffff},
	}
}

func stringClassIntervals(class patternClass) []stringUnitInterval {
	boundaries := []int{0, stringUTF16UnitCount}
	for _, part := range class.parts {
		for _, characterRange := range part.ranges {
			boundaries = append(boundaries, int(characterRange.low), int(characterRange.high)+1)
		}
	}

	sort.Ints(boundaries)
	boundaries = uniqueInts(boundaries)

	intervals := make([]stringUnitInterval, 0)
	for index := 0; index+1 < len(boundaries); index++ {
		low := boundaries[index]
		high := boundaries[index+1] - 1
		if low > high || low >= stringUTF16UnitCount {
			continue
		}

		if !patternClassMatches(class, uint16(low)) {
			continue
		}

		candidate := stringUnitInterval{low: uint16(low), high: uint16(high)}
		if len(intervals) > 0 && intervals[len(intervals)-1].high+1 == candidate.low {
			intervals[len(intervals)-1].high = candidate.high

			continue
		}

		intervals = append(intervals, candidate)
	}

	return intervals
}

func intervalContains(intervals []stringUnitInterval, unit uint16) bool {
	for _, interval := range intervals {
		if unit >= interval.low && unit <= interval.high {
			return true
		}
	}

	return false
}

func uniqueInts(values []int) []int {
	if len(values) < 2 {
		return values
	}

	result := make([]int, 0, len(values))
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}

	return result
}

func sortedInts(values []int) []int {
	result := append([]int(nil), values...)
	sort.Ints(result)

	return uniqueInts(result)
}

func appendUniqueInt(values []int, wanted int) []int {
	for _, value := range values {
		if value == wanted {
			return values
		}
	}

	return append(values, wanted)
}

func intSlicesEqual(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}

	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}

// stringPatternSeedUnits collects schema-authored units used for interval interiors.
func stringPatternSeedUnits(pattern *patternAST) []uint16 {
	if pattern == nil {
		return nil
	}

	units := make([]uint16, 0)
	var collectExpression func(*patternExpression)
	collectExpression = func(expression *patternExpression) {
		if expression == nil {
			return
		}

		for _, alternative := range expression.alternatives {
			if alternative == nil {
				continue
			}

			for _, term := range alternative.terms {
				if term == nil || term.atom == nil {
					continue
				}

				switch term.atom.kind {
				case patternLiteral:
					units = appendUniqueUint16(units, term.atom.literal)
				case patternClassAtom:
					for _, part := range term.atom.class.parts {
						for _, characterRange := range part.ranges {
							units = appendUniqueUint16(units, characterRange.low)
							units = appendUniqueUint16(units, characterRange.high)
						}
					}
				case patternGroup:
					collectExpression(term.atom.expression)
				}
			}
		}
	}

	for _, assertion := range pattern.leadingAssertions {
		collectExpression(assertion.expression)
	}
	collectExpression(pattern.expression)

	return units
}

func appendUniqueUint16(values []uint16, wanted ...uint16) []uint16 {
	for _, candidate := range wanted {
		found := false
		for _, value := range values {
			if value == candidate {
				found = true

				break
			}
		}

		if !found {
			values = append(values, candidate)
		}
	}

	return values
}

// stringIntervalCandidates returns the locked low/high/schema-seed order for one edge.
func stringIntervalCandidates(
	interval stringUnitInterval,
	seed uint64,
	seedUnits []uint16,
) []uint16 {
	candidates := make([]uint16, 0, 4+len(seedUnits))
	appendCandidate := func(candidate uint16) {
		if candidate < interval.low || candidate > interval.high {
			return
		}

		candidates = appendUniqueUint16(candidates, candidate)
	}

	appendCandidate(interval.low)
	appendCandidate(interval.high)

	for _, unit := range seedUnits {
		if unit > interval.low && unit < interval.high {
			appendCandidate(unit)
		}
	}

	if interval.high-interval.low > 1 {
		span := uint64(interval.high - interval.low - 1)
		interior := interval.low + 1 + uint16(seed%span)
		appendCandidate(interior)
	}

	return candidates
}
