//nolint:cyclop,godoclint // The clean matcher mirrors the admitted pattern AST directly.
package schematest

import (
	"errors"
	"unicode/utf16"
)

// cleanPatternMatches evaluates one admitted pattern over ECMAScript UTF-16 units.
func cleanPatternMatches(pattern *patternAST, value string) (bool, error) {
	if pattern == nil || pattern.expression == nil {
		return false, errors.New("pattern has no expression")
	}

	units := utf16.Encode([]rune(value))
	matcher := cleanPatternMatcher{units: units}

	if len(pattern.leadingAssertions) > 0 {
		if !matcher.matchesLeadingAssertions(pattern.leadingAssertions) {
			return false, nil
		}

		return matcher.matchesExpressionAt(pattern.expression, 0), nil
	}

	for start := 0; start <= len(units); start++ {
		if matcher.matchesExpressionAt(pattern.expression, start) {
			return true, nil
		}
	}

	return false, nil
}

type cleanPatternMatcher struct {
	units []uint16
}

func (matcher *cleanPatternMatcher) matchesLeadingAssertions(assertions []patternLookahead) bool {
	for _, assertion := range assertions {
		if assertion.expression == nil {
			return false
		}

		matched := matcher.matchesExpressionAt(assertion.expression, 0)
		if matched != assertion.positive {
			return false
		}
	}

	return true
}

func (matcher *cleanPatternMatcher) matchesExpressionAt(expression *patternExpression, start int) bool {
	return len(matcher.matchExpressionEnds(expression, start)) > 0
}

func (matcher *cleanPatternMatcher) matchExpressionEnds(expression *patternExpression, start int) []int {
	if expression == nil {
		return nil
	}

	ends := make([]int, 0)
	for _, alternative := range expression.alternatives {
		ends = appendUniquePatternPositions(ends, matcher.matchSequenceEnds(alternative, start)...)
	}

	return ends
}

func (matcher *cleanPatternMatcher) matchSequenceEnds(sequence *patternSequence, start int) []int {
	if sequence == nil {
		return nil
	}

	positions := []int{start}

	for _, term := range sequence.terms {
		if term == nil || term.atom == nil {
			return nil
		}

		positions = matcher.matchTerm(term, positions)
		if len(positions) == 0 {
			return nil
		}
	}

	return positions
}

func (matcher *cleanPatternMatcher) matchTerm(term *patternTerm, starts []int) []int {
	if !term.quantified {
		return matcher.matchAtomFromStarts(term.atom, starts)
	}

	positions := uniquePatternPositions(starts)
	accepted := make([]int, 0, len(positions))

	if term.minimum == 0 {
		accepted = append(accepted, positions...)
	}

	for count := uint64(1); term.unbounded || count <= term.maximum; count++ {
		positions = matcher.matchAtomFromStarts(term.atom, positions)
		if len(positions) == 0 {
			break
		}

		before := len(accepted)
		if count >= term.minimum {
			accepted = appendUniquePatternPositions(accepted, positions...)
		}

		if term.unbounded && count >= term.minimum && len(accepted) == before {
			break
		}
	}

	return uniquePatternPositions(accepted)
}

func (matcher *cleanPatternMatcher) matchAtomFromStarts(atom *patternAtom, starts []int) []int {
	matches := make([]int, 0, len(starts))

	for _, start := range starts {
		matches = appendUniquePatternPositions(matches, matcher.matchAtom(atom, start)...)
	}

	return matches
}

func (matcher *cleanPatternMatcher) matchAtom(atom *patternAtom, position int) []int {
	switch atom.kind {
	case patternLiteral:
		if position < len(matcher.units) && matcher.units[position] == atom.literal {
			return []int{position + 1}
		}

		return nil
	case patternDot:
		if position < len(matcher.units) && !isPatternLineTerminator(matcher.units[position]) {
			return []int{position + 1}
		}

		return nil
	case patternClassAtom:
		if position < len(matcher.units) && patternClassMatches(atom.class, matcher.units[position]) {
			return []int{position + 1}
		}

		return nil
	case patternStart:
		if position == 0 {
			return []int{position}
		}

		return nil
	case patternEnd:
		if position == len(matcher.units) {
			return []int{position}
		}

		return nil
	case patternWordBoundary, patternNotWordBoundary:
		boundary := patternWordBoundaryAt(matcher.units, position)
		if boundary == (atom.kind == patternWordBoundary) {
			return []int{position}
		}

		return nil
	case patternGroup:
		return matcher.matchExpressionEnds(atom.expression, position)
	default:
		return nil
	}
}

func patternClassMatches(class patternClass, value uint16) bool {
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
		if seen[position] {
			continue
		}

		seen[position] = true
		unique = append(unique, position)
	}

	return unique
}

func appendUniquePatternPositions(destination []int, positions ...int) []int {
	seen := make(map[int]bool, len(destination)+len(positions))
	for _, position := range destination {
		seen[position] = true
	}

	for _, position := range positions {
		if seen[position] {
			continue
		}

		seen[position] = true
		destination = append(destination, position)
	}

	return destination
}
