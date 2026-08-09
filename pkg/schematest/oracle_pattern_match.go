//nolint:cyclop,godoclint // The clean compiler and matcher are intentionally independent from search.
package schematest

import (
	"errors"
	"unicode/utf16"
)

type cleanPatternProgram struct {
	assertions []cleanPatternAssertion
	expression *cleanPatternExpression
}

type cleanPatternAssertion struct {
	positive   bool
	expression *cleanPatternExpression
}

type cleanPatternExpression struct {
	alternatives []*cleanPatternSequence
}

type cleanPatternSequence struct {
	terms []*cleanPatternTerm
}

type cleanPatternTerm struct {
	atom       *cleanPatternAtom
	quantified bool
	minimum    uint64
	maximum    uint64
	unbounded  bool
	greedy     bool
}

type cleanPatternAtomKind uint8

const (
	cleanPatternLiteral cleanPatternAtomKind = iota
	cleanPatternDot
	cleanPatternClassAtom
	cleanPatternStart
	cleanPatternEnd
	cleanPatternWordBoundary
	cleanPatternNotWordBoundary
	cleanPatternGroup
)

type cleanPatternAtom struct {
	kind       cleanPatternAtomKind
	literal    uint16
	class      cleanPatternClass
	expression *cleanPatternExpression
}

type cleanPatternClass struct {
	parts   []cleanPatternClassPart
	negated bool
}

type cleanPatternClassPart struct {
	ranges  []cleanPatternRange
	negated bool
}

type cleanPatternRange struct {
	low  uint16
	high uint16
}

func compileCleanPattern(pattern *patternAST) (*cleanPatternProgram, error) {
	if pattern == nil || pattern.expression == nil {
		return nil, errors.New("pattern has no expression")
	}

	program := &cleanPatternProgram{assertions: make([]cleanPatternAssertion, 0, len(pattern.leadingAssertions))}
	for _, assertion := range pattern.leadingAssertions {
		expression, err := compileCleanPatternExpression(assertion.expression)
		if err != nil {
			return nil, err
		}

		program.assertions = append(program.assertions, cleanPatternAssertion{
			positive: assertion.positive, expression: expression,
		})
	}

	expression, err := compileCleanPatternExpression(pattern.expression)
	if err != nil {
		return nil, err
	}

	program.expression = expression

	return program, nil
}

func compileCleanPatternExpression(expression *patternExpression) (*cleanPatternExpression, error) {
	if expression == nil {
		return nil, errors.New("pattern expression is nil")
	}

	compiled := &cleanPatternExpression{alternatives: make([]*cleanPatternSequence, 0, len(expression.alternatives))}
	for _, alternative := range expression.alternatives {
		sequence, err := compileCleanPatternSequence(alternative)
		if err != nil {
			return nil, err
		}

		compiled.alternatives = append(compiled.alternatives, sequence)
	}

	return compiled, nil
}

func compileCleanPatternSequence(sequence *patternSequence) (*cleanPatternSequence, error) {
	if sequence == nil {
		return nil, errors.New("pattern sequence is nil")
	}

	compiled := &cleanPatternSequence{terms: make([]*cleanPatternTerm, 0, len(sequence.terms))}
	for _, term := range sequence.terms {
		if term == nil || term.atom == nil {
			return nil, errors.New("pattern term is nil")
		}

		atom, err := compileCleanPatternAtom(term.atom)
		if err != nil {
			return nil, err
		}

		compiled.terms = append(compiled.terms, &cleanPatternTerm{
			atom: atom, quantified: term.quantified, minimum: term.minimum,
			maximum: term.maximum, unbounded: term.unbounded, greedy: term.greedy,
		})
	}

	return compiled, nil
}

func compileCleanPatternAtom(atom *patternAtom) (*cleanPatternAtom, error) {
	if atom == nil {
		return nil, errors.New("pattern atom is nil")
	}

	compiled := &cleanPatternAtom{literal: atom.literal}
	switch atom.kind {
	case patternLiteral:
		compiled.kind = cleanPatternLiteral
	case patternDot:
		compiled.kind = cleanPatternDot
	case patternClassAtom:
		compiled.kind = cleanPatternClassAtom
	case patternStart:
		compiled.kind = cleanPatternStart
	case patternEnd:
		compiled.kind = cleanPatternEnd
	case patternWordBoundary:
		compiled.kind = cleanPatternWordBoundary
	case patternNotWordBoundary:
		compiled.kind = cleanPatternNotWordBoundary
	case patternGroup:
		compiled.kind = cleanPatternGroup
	default:
		return nil, errors.New("pattern atom kind is unsupported")
	}

	for _, part := range atom.class.parts {
		compiledPart := cleanPatternClassPart{negated: part.negated}
		for _, characterRange := range part.ranges {
			compiledPart.ranges = append(compiledPart.ranges, cleanPatternRange(characterRange))
		}

		compiled.class.parts = append(compiled.class.parts, compiledPart)
	}

	compiled.class.negated = atom.class.negated

	if atom.expression != nil {
		expression, err := compileCleanPatternExpression(atom.expression)
		if err != nil {
			return nil, err
		}

		compiled.expression = expression
	}

	return compiled, nil
}

// cleanPatternMatches evaluates one admitted pattern over ECMAScript UTF-16 units.
func cleanPatternMatches(pattern *patternAST, value string) (bool, error) {
	if pattern == nil || pattern.cleanProgram == nil || pattern.cleanProgram.expression == nil {
		return false, errors.New("pattern has no compiled clean program")
	}

	matcher := cleanPatternMatcher{
		units:          utf16.Encode([]rune(value)),
		expressionMemo: make(map[cleanExpressionMemoKey][]int),
		sequenceMemo:   make(map[cleanSequenceMemoKey][]int),
		atomMemo:       make(map[cleanAtomMemoKey][]int),
	}

	for _, assertion := range pattern.cleanProgram.assertions {
		matched := len(matcher.matchExpressionEnds(assertion.expression, 0)) > 0
		if matched != assertion.positive {
			return false, nil
		}
	}

	if len(pattern.cleanProgram.assertions) > 0 {
		return len(matcher.matchExpressionEnds(pattern.cleanProgram.expression, 0)) > 0, nil
	}

	starts := make([]int, len(matcher.units)+1)
	for position := range starts {
		starts[position] = position
	}

	return len(matcher.matchExpressionFromStarts(pattern.cleanProgram.expression, starts)) > 0, nil
}

type cleanExpressionMemoKey struct {
	expression *cleanPatternExpression
	start      int
}

type cleanSequenceMemoKey struct {
	sequence *cleanPatternSequence
	start    int
}

type cleanAtomMemoKey struct {
	atom     *cleanPatternAtom
	position int
}

type cleanPatternMatcher struct {
	units          []uint16
	expressionMemo map[cleanExpressionMemoKey][]int
	sequenceMemo   map[cleanSequenceMemoKey][]int
	atomMemo       map[cleanAtomMemoKey][]int
}

func (matcher *cleanPatternMatcher) matchExpressionFromStarts(
	expression *cleanPatternExpression,
	starts []int,
) []int {
	ends := make([]int, 0)
	for _, alternative := range expression.alternatives {
		ends = appendUniquePatternPositions(ends, matcher.matchSequenceFromStarts(alternative, starts)...)
	}

	return ends
}

func (matcher *cleanPatternMatcher) matchSequenceFromStarts(
	sequence *cleanPatternSequence,
	starts []int,
) []int {
	positions := uniquePatternPositions(starts)
	for _, term := range sequence.terms {
		positions = matcher.matchTerm(term, positions)
		if len(positions) == 0 {
			break
		}
	}

	return positions
}

func (matcher *cleanPatternMatcher) matchExpressionEnds(expression *cleanPatternExpression, start int) []int {
	key := cleanExpressionMemoKey{expression: expression, start: start}
	if ends, exists := matcher.expressionMemo[key]; exists {
		return ends
	}

	ends := make([]int, 0)
	for _, alternative := range expression.alternatives {
		ends = appendUniquePatternPositions(ends, matcher.matchSequenceEnds(alternative, start)...)
	}

	matcher.expressionMemo[key] = ends

	return ends
}

func (matcher *cleanPatternMatcher) matchSequenceEnds(sequence *cleanPatternSequence, start int) []int {
	key := cleanSequenceMemoKey{sequence: sequence, start: start}
	if ends, exists := matcher.sequenceMemo[key]; exists {
		return ends
	}

	positions := []int{start}
	for _, term := range sequence.terms {
		positions = matcher.matchTerm(term, positions)
		if len(positions) == 0 {
			break
		}
	}

	matcher.sequenceMemo[key] = positions

	return positions
}

func (matcher *cleanPatternMatcher) matchTerm(term *cleanPatternTerm, starts []int) []int {
	if !term.quantified {
		return matcher.matchAtomFromStarts(term.atom, starts)
	}

	levels := [][]int{uniquePatternPositions(starts)}
	for count := uint64(1); term.unbounded || count <= term.maximum; count++ {
		next := matcher.matchAtomFromStarts(term.atom, levels[len(levels)-1])
		if len(next) == 0 {
			break
		}

		stable := equalPatternPositions(next, levels[len(levels)-1])
		levels = append(levels, next)

		if stable && count >= term.minimum {
			break
		}
	}

	accepted := make([]int, 0)

	if term.greedy {
		for level := len(levels) - 1; level >= 0; level-- {
			if uint64(level) >= term.minimum {
				accepted = appendUniquePatternPositions(accepted, levels[level]...)
			}
		}
	} else {
		for level, positions := range levels {
			if uint64(level) >= term.minimum {
				accepted = appendUniquePatternPositions(accepted, positions...)
			}
		}
	}

	return accepted
}

func (matcher *cleanPatternMatcher) matchAtomFromStarts(atom *cleanPatternAtom, starts []int) []int {
	matches := make([]int, 0, len(starts))
	for _, start := range starts {
		matches = appendUniquePatternPositions(matches, matcher.matchAtom(atom, start)...)
	}

	return matches
}

func (matcher *cleanPatternMatcher) matchAtom(atom *cleanPatternAtom, position int) []int {
	key := cleanAtomMemoKey{atom: atom, position: position}
	if matches, exists := matcher.atomMemo[key]; exists {
		return matches
	}

	var matches []int

	switch atom.kind {
	case cleanPatternLiteral:
		if position < len(matcher.units) && matcher.units[position] == atom.literal {
			matches = []int{position + 1}
		}
	case cleanPatternDot:
		if position < len(matcher.units) && !isPatternLineTerminator(matcher.units[position]) {
			matches = []int{position + 1}
		}
	case cleanPatternClassAtom:
		if position < len(matcher.units) && cleanPatternClassMatches(atom.class, matcher.units[position]) {
			matches = []int{position + 1}
		}
	case cleanPatternStart:
		if position == 0 {
			matches = []int{position}
		}
	case cleanPatternEnd:
		if position == len(matcher.units) {
			matches = []int{position}
		}
	case cleanPatternWordBoundary, cleanPatternNotWordBoundary:
		boundary := patternWordBoundaryAt(matcher.units, position)
		if boundary == (atom.kind == cleanPatternWordBoundary) {
			matches = []int{position}
		}
	case cleanPatternGroup:
		matches = matcher.matchExpressionEnds(atom.expression, position)
	}

	matcher.atomMemo[key] = matches

	return matches
}

func cleanPatternClassMatches(class cleanPatternClass, value uint16) bool {
	matched := false

	for _, part := range class.parts {
		partMatched := false

		for _, characterRange := range part.ranges {
			if value >= characterRange.low && value <= characterRange.high {
				partMatched = true

				break
			}
		}

		if part.negated {
			partMatched = !partMatched
		}

		matched = matched || partMatched
	}

	if class.negated {
		return !matched
	}

	return matched
}

func patternWordBoundaryAt(units []uint16, position int) bool {
	previous := position > 0 && isPatternWordUnit(units[position-1])
	next := position < len(units) && isPatternWordUnit(units[position])

	return previous != next
}

func isPatternWordUnit(value uint16) bool {
	return value >= '0' && value <= '9' || value >= 'A' && value <= 'Z' ||
		value >= 'a' && value <= 'z' || value == '_'
}

func isPatternLineTerminator(value uint16) bool {
	return value == '\n' || value == '\r' || value == 0x2028 || value == 0x2029
}

func uniquePatternPositions(positions []int) []int {
	unique := make([]int, 0, len(positions))

	seen := make(map[int]bool, len(positions))
	for _, position := range positions {
		if !seen[position] {
			seen[position] = true
			unique = append(unique, position)
		}
	}

	return unique
}

func appendUniquePatternPositions(destination []int, positions ...int) []int {
	seen := make(map[int]bool, len(destination)+len(positions))
	for _, position := range destination {
		seen[position] = true
	}

	for _, position := range positions {
		if !seen[position] {
			seen[position] = true
			destination = append(destination, position)
		}
	}

	return destination
}

func equalPatternPositions(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}

	positions := make(map[int]bool, len(left))
	for _, position := range left {
		positions[position] = true
	}

	for _, position := range right {
		if !positions[position] {
			return false
		}
	}

	return true
}
