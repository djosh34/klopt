//nolint:cyclop,godoclint,mnd,nestif // Private grammar productions and normative limits are intentionally explicit.
package schematest

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	patternSourceByteLimit       = 65_536
	patternNestingLimit          = 100
	patternNodeLimit             = 10_000
	patternLeadingAssertionLimit = 64
	patternRepeatEndpointLimit   = 1_000
	patternNestedRepeatLimit     = 1_000
	patternMatcherByteLimit      = 1_048_576
)

type patternAtomKind uint8

const (
	patternLiteral patternAtomKind = iota
	patternDot
	patternClassAtom
	patternStart
	patternEnd
	patternWordBoundary
	patternNotWordBoundary
	patternGroup
)

type patternRange struct {
	low  uint16
	high uint16
}

type patternClassPart struct {
	ranges  []patternRange
	negated bool
}

type patternClass struct {
	parts   []patternClassPart
	negated bool
}

type patternExpression struct {
	alternatives []*patternSequence
}

type patternSequence struct {
	terms []*patternTerm
}

type patternAtom struct {
	kind       patternAtomKind
	literal    uint16
	class      patternClass
	expression *patternExpression
}

type patternTerm struct {
	atom       *patternAtom
	quantified bool
	counted    bool
	minimum    uint64
	maximum    uint64
	unbounded  bool
	greedy     bool
}

type patternLookahead struct {
	positive   bool
	expression *patternExpression
}

type patternAST struct {
	source            string
	leadingAssertions []patternLookahead
	expression        *patternExpression
	nodeCount         int
	matcherBytes      int
}

type ecmaPatternParser struct {
	units                    []uint16
	position                 int
	nesting                  int
	nodeCount                int
	lowSurrogateContinuation bool
}

func parseECMAPattern(source string) (*patternAST, error) {
	if !utf8.ValidString(source) {
		return nil, errors.New("pattern is not valid UTF-8")
	}

	if len(source) > patternSourceByteLimit {
		return nil, fmt.Errorf("pattern source exceeds %d bytes", patternSourceByteLimit)
	}

	parser := ecmaPatternParser{units: utf16.Encode([]rune(source))}
	ast := &patternAST{source: source}

	if parser.hasLeadingLookahead() {
		start, err := parser.newAtom(patternStart)
		if err != nil {
			return nil, err
		}

		parser.position++

		for parser.nextIsLookahead() {
			assertion, parseErr := parser.parseLeadingLookahead()
			if parseErr != nil {
				return nil, parseErr
			}

			ast.leadingAssertions = append(ast.leadingAssertions, assertion)
			if len(ast.leadingAssertions) > patternLeadingAssertionLimit {
				return nil, fmt.Errorf("leading assertions exceed %d", patternLeadingAssertionLimit)
			}
		}

		expression, err := parser.parseExpression(0)
		if err != nil {
			return nil, err
		}

		if len(expression.alternatives) != 1 {
			return nil, errors.New("leading assertions require one top-level alternative")
		}

		expression.alternatives[0].terms = append(
			[]*patternTerm{{atom: start, minimum: 1, maximum: 1, greedy: true}},
			expression.alternatives[0].terms...,
		)
		ast.expression = expression
	} else {
		expression, err := parser.parseExpression(0)
		if err != nil {
			return nil, err
		}

		ast.expression = expression
	}

	if parser.position != len(parser.units) {
		return nil, fmt.Errorf("unexpected pattern token at UTF-16 unit %d", parser.position)
	}

	ast.nodeCount = parser.nodeCount
	if len(ast.leadingAssertions) == 0 {
		ast.matcherBytes = patternExpressionBytes(ast.expression)
		if ast.matcherBytes > patternMatcherByteLimit {
			return nil, fmt.Errorf("translated matcher source exceeds %d bytes", patternMatcherByteLimit)
		}
	} else {
		for _, assertion := range ast.leadingAssertions {
			matcherBytes := 6 + patternExpressionBytes(assertion.expression)
			if matcherBytes > patternMatcherByteLimit {
				return nil, fmt.Errorf("translated matcher source exceeds %d bytes", patternMatcherByteLimit)
			}

			ast.matcherBytes = max(ast.matcherBytes, matcherBytes)
		}

		remainderBytes := 6
		for _, term := range ast.expression.alternatives[0].terms[1:] {
			remainderBytes += patternTermBytes(term)
		}

		if remainderBytes > patternMatcherByteLimit {
			return nil, fmt.Errorf("translated matcher source exceeds %d bytes", patternMatcherByteLimit)
		}

		ast.matcherBytes = max(ast.matcherBytes, remainderBytes)
	}

	return ast, nil
}

func (parser *ecmaPatternParser) hasLeadingLookahead() bool {
	return len(parser.units) >= 4 && parser.units[0] == '^' &&
		parser.units[1] == '(' && parser.units[2] == '?' &&
		(parser.units[3] == '=' || parser.units[3] == '!')
}

func (parser *ecmaPatternParser) nextIsLookahead() bool {
	return parser.position+2 < len(parser.units) && parser.units[parser.position] == '(' &&
		parser.units[parser.position+1] == '?' &&
		(parser.units[parser.position+2] == '=' || parser.units[parser.position+2] == '!')
}

func (parser *ecmaPatternParser) parseLeadingLookahead() (patternLookahead, error) {
	positive := parser.units[parser.position+2] == '='
	parser.position += 3

	if err := parser.enterGroup(); err != nil {
		return patternLookahead{}, err
	}
	defer parser.leaveGroup()

	if err := parser.countNode(); err != nil {
		return patternLookahead{}, err
	}

	expression, err := parser.parseExpression(')')
	if err != nil {
		return patternLookahead{}, err
	}

	if !parser.consume(')') {
		return patternLookahead{}, errors.New("unterminated leading assertion")
	}

	return patternLookahead{positive: positive, expression: expression}, nil
}

func (parser *ecmaPatternParser) parseExpression(stop uint16) (*patternExpression, error) {
	if err := parser.countNode(); err != nil {
		return nil, err
	}

	expression := &patternExpression{}

	for {
		sequence, err := parser.parseSequence(stop)
		if err != nil {
			return nil, err
		}

		expression.alternatives = append(expression.alternatives, sequence)
		if !parser.consume('|') {
			return expression, nil
		}
	}
}

func (parser *ecmaPatternParser) parseSequence(stop uint16) (*patternSequence, error) {
	if err := parser.countNode(); err != nil {
		return nil, err
	}

	sequence := &patternSequence{}

	for parser.position < len(parser.units) {
		unit := parser.units[parser.position]
		if unit == '|' || stop != 0 && unit == stop {
			break
		}

		term, err := parser.parseTerm()
		if err != nil {
			return nil, err
		}

		sequence.terms = append(sequence.terms, term)
	}

	return sequence, nil
}

func (parser *ecmaPatternParser) parseTerm() (*patternTerm, error) {
	atom, assertion, repeatProduct, err := parser.parseAtom()
	if err != nil {
		return nil, err
	}

	term := &patternTerm{atom: atom, minimum: 1, maximum: 1, greedy: true}
	if parser.position == len(parser.units) || !isQuantifierStart(parser.units[parser.position]) {
		return term, nil
	}

	if assertion {
		return nil, errors.New("assertions cannot be quantified")
	}

	factor, counted, err := parser.parseQuantifier(term)
	if err != nil {
		return nil, err
	}

	term.counted = counted
	if counted {
		if repeatProduct == 0 {
			repeatProduct = 1
		}

		if factor == 0 {
			factor = 1
		}

		if repeatProduct > patternNestedRepeatLimit/factor {
			return nil, fmt.Errorf("nested counted-repeat product exceeds %d", patternNestedRepeatLimit)
		}
	}

	if parser.consume('?') {
		term.greedy = false
	} else if parser.position < len(parser.units) && parser.units[parser.position] == '+' {
		return nil, errors.New("possessive quantifiers are outside the pattern profile")
	}

	if parser.position < len(parser.units) && isQuantifierStart(parser.units[parser.position]) {
		return nil, errors.New("multiple quantifiers on one atom")
	}

	if err := parser.countNode(); err != nil {
		return nil, err
	}

	return term, nil
}

func (parser *ecmaPatternParser) parseAtom() (*patternAtom, bool, uint64, error) {
	if parser.position == len(parser.units) {
		return nil, false, 0, errors.New("expected pattern atom")
	}

	unit := parser.units[parser.position]
	parser.position++

	switch unit {
	case '.':
		atom, err := parser.newAtom(patternDot)

		return atom, false, 1, err
	case '^':
		atom, err := parser.newAtom(patternStart)

		return atom, true, 1, err
	case '$':
		atom, err := parser.newAtom(patternEnd)

		return atom, true, 1, err
	case '[':
		atom, err := parser.parseClass()

		return atom, false, 1, err
	case '(':
		return parser.parseGroup()
	case '\\':
		return parser.parseEscape(false)
	case ')':
		return nil, false, 0, errors.New("unmatched ')'")
	case ']', '{', '}':
		return nil, false, 0, fmt.Errorf("unescaped %q is outside the pattern grammar", rune(unit))
	case '*', '+', '?':
		return nil, false, 0, errors.New("quantifier has no atom")
	default:
		atom := &patternAtom{kind: patternLiteral}

		if parser.lowSurrogateContinuation {
			parser.lowSurrogateContinuation = false
		} else if err := parser.countNode(); err != nil {
			return nil, false, 0, err
		}

		if unit >= 0xd800 && unit <= 0xdbff && parser.position < len(parser.units) &&
			parser.units[parser.position] >= 0xdc00 && parser.units[parser.position] <= 0xdfff {
			parser.lowSurrogateContinuation = true
		}

		atom.literal = unit

		return atom, false, 1, nil
	}
}

func (parser *ecmaPatternParser) parseGroup() (*patternAtom, bool, uint64, error) {
	if parser.consume('?') {
		if !parser.consume(':') {
			return nil, false, 0, errors.New("unsupported '(?' group or misplaced lookahead")
		}
	}

	if err := parser.enterGroup(); err != nil {
		return nil, false, 0, err
	}
	defer parser.leaveGroup()

	expression, err := parser.parseExpression(')')
	if err != nil {
		return nil, false, 0, err
	}

	if !parser.consume(')') {
		return nil, false, 0, errors.New("unterminated group")
	}

	atom, err := parser.newAtom(patternGroup)
	if err != nil {
		return nil, false, 0, err
	}

	atom.expression = expression

	return atom, false, expressionRepeatProduct(expression), nil
}

func (parser *ecmaPatternParser) parseClass() (*patternAtom, error) {
	class := patternClass{}
	if parser.consume('^') {
		class.negated = true
	}

	if parser.consume(']') {
		atom, err := parser.newAtom(patternClassAtom)
		if err != nil {
			return nil, err
		}

		atom.class = class

		return atom, nil
	}

	for {
		if parser.position == len(parser.units) {
			return nil, errors.New("unterminated character class")
		}

		if parser.classHasForeignSetSyntax() {
			return nil, errors.New("POSIX classes and set operations are outside the pattern profile")
		}

		part, singleton, start, err := parser.parseClassPart()
		if err != nil {
			return nil, err
		}

		if parser.position+1 < len(parser.units) && parser.units[parser.position] == '-' &&
			parser.units[parser.position+1] != ']' {
			if !singleton {
				return nil, errors.New("character-class range endpoint must be one UTF-16 unit")
			}

			parser.position++

			endPart, endSingleton, end, endErr := parser.parseClassPart()
			if endErr != nil {
				return nil, endErr
			}

			if !endSingleton || len(endPart.ranges) != 1 {
				return nil, errors.New("character-class range endpoint must be one UTF-16 unit")
			}

			if start > end {
				return nil, errors.New("character-class range is descending")
			}

			part = patternClassPart{ranges: []patternRange{{low: start, high: end}}}
		}

		class.parts = append(class.parts, part)

		if parser.consume(']') {
			break
		}
	}

	class = normalizePatternClass(class)

	atom, err := parser.newAtom(patternClassAtom)
	if err != nil {
		return nil, err
	}

	atom.class = class

	return atom, nil
}

func normalizePatternClass(class patternClass) patternClass {
	allPositive := true

	var positive []patternRange

	for index := range class.parts {
		if class.parts[index].negated {
			allPositive = false

			continue
		}

		positive = append(positive, class.parts[index].ranges...)
	}

	if allPositive {
		class.parts = []patternClassPart{{ranges: mergePatternRanges(positive)}}

		return class
	}

	for index := range class.parts {
		class.parts[index].ranges = mergePatternRanges(class.parts[index].ranges)
	}

	return class
}

func mergePatternRanges(ranges []patternRange) []patternRange {
	if len(ranges) < 2 {
		return ranges
	}

	sort.Slice(ranges, func(left, right int) bool {
		if ranges[left].low == ranges[right].low {
			return ranges[left].high < ranges[right].high
		}

		return ranges[left].low < ranges[right].low
	})

	merged := []patternRange{ranges[0]}
	for _, candidate := range ranges[1:] {
		last := &merged[len(merged)-1]
		if uint32(candidate.low) <= uint32(last.high)+1 {
			if candidate.high > last.high {
				last.high = candidate.high
			}

			continue
		}

		merged = append(merged, candidate)
	}

	return merged
}

func (parser *ecmaPatternParser) classHasForeignSetSyntax() bool {
	if parser.position+1 >= len(parser.units) {
		return false
	}

	first := parser.units[parser.position]
	second := parser.units[parser.position+1]

	return first == '[' && second == ':' || first == '&' && second == '&' || first == '-' && second == '-'
}

func (parser *ecmaPatternParser) parseClassPart() (patternClassPart, bool, uint16, error) {
	if parser.position == len(parser.units) {
		return patternClassPart{}, false, 0, errors.New("unterminated character class")
	}

	if parser.units[parser.position] != '\\' {
		unit := parser.units[parser.position]
		parser.position++

		return singleClassPart(unit), true, unit, nil
	}

	parser.position++

	atom, assertion, _, err := parser.parseEscape(true)
	if err != nil {
		return patternClassPart{}, false, 0, err
	}

	if assertion {
		return patternClassPart{}, false, 0, errors.New("invalid character-class escape")
	}

	if atom.kind == patternLiteral {
		part := singleClassPart(atom.literal)

		return part, true, atom.literal, nil
	}

	if atom.kind != patternClassAtom {
		return patternClassPart{}, false, 0, errors.New("invalid character-class escape")
	}

	part := atom.class.parts[0]
	if len(part.ranges) == 1 && part.ranges[0].low == part.ranges[0].high && !part.negated {
		return part, true, part.ranges[0].low, nil
	}

	return part, false, 0, nil
}

func (parser *ecmaPatternParser) parseEscape(inClass bool) (*patternAtom, bool, uint64, error) {
	if parser.position == len(parser.units) {
		return nil, false, 0, errors.New("unterminated escape")
	}

	escaped := parser.units[parser.position]
	parser.position++

	if escaped >= '1' && escaped <= '9' {
		return nil, false, 0, errors.New("backreferences are outside the pattern profile")
	}

	switch escaped {
	case 'd', 'D', 'w', 'W', 's', 'S':
		atom, err := parser.newAtom(patternClassAtom)
		if err != nil {
			return nil, false, 0, err
		}

		atom.class.parts = []patternClassPart{predefinedClass(escaped)}

		return atom, false, 1, nil
	case 'b':
		if inClass {
			return parser.literalEscape(0x0008)
		}

		atom, err := parser.newAtom(patternWordBoundary)

		return atom, true, 1, err
	case 'B':
		if inClass {
			return nil, false, 0, errors.New("\\B is not a character-class escape")
		}

		atom, err := parser.newAtom(patternNotWordBoundary)

		return atom, true, 1, err
	case 'f':
		return parser.literalEscape(0x000c)
	case 'n':
		return parser.literalEscape(0x000a)
	case 'r':
		return parser.literalEscape(0x000d)
	case 't':
		return parser.literalEscape(0x0009)
	case 'v':
		return parser.literalEscape(0x000b)
	case '0':
		if parser.position < len(parser.units) &&
			parser.units[parser.position] >= '0' && parser.units[parser.position] <= '9' {
			return nil, false, 0, errors.New("decimal escapes are outside the pattern profile")
		}

		return parser.literalEscape(0)
	case 'x':
		return parser.hexEscape(2)
	case 'u':
		if parser.position < len(parser.units) && parser.units[parser.position] == '{' {
			return nil, false, 0, errors.New("unicode code-point escapes are outside the pattern profile")
		}

		return parser.hexEscape(4)
	case 'c':
		return parser.controlEscape()
	case 'p', 'P', 'k':
		return nil, false, 0, errors.New("unicode properties and named references are outside the pattern profile")
	default:
		if !isPermittedIdentityEscape(escaped) {
			return nil, false, 0, fmt.Errorf("identity escape \\%c is outside the pattern profile", escaped)
		}

		return parser.literalEscape(escaped)
	}
}

func (parser *ecmaPatternParser) literalEscape(unit uint16) (*patternAtom, bool, uint64, error) {
	atom, err := parser.newAtom(patternLiteral)
	if err != nil {
		return nil, false, 0, err
	}

	atom.literal = unit
	if unit >= 0xd800 && unit <= 0xdfff {
		return nil, false, 0, errors.New("surrogate escapes are outside the pattern profile")
	}

	if parser.nesting > 0 && parser.position > len(parser.units) {
		return nil, false, 0, errors.New("invalid pattern state")
	}

	return atom, false, 1, nil
}

func (parser *ecmaPatternParser) hexEscape(digits int) (*patternAtom, bool, uint64, error) {
	if parser.position+digits > len(parser.units) {
		return nil, false, 0, errors.New("short hexadecimal escape")
	}

	var value uint16

	for range digits {
		digit, ok := patternHexValue(parser.units[parser.position])
		if !ok {
			return nil, false, 0, errors.New("invalid hexadecimal escape")
		}

		value = value*16 + digit
		parser.position++
	}

	return parser.literalEscape(value)
}

func (parser *ecmaPatternParser) controlEscape() (*patternAtom, bool, uint64, error) {
	if parser.position == len(parser.units) {
		return nil, false, 0, errors.New("short control-letter escape")
	}

	letter := parser.units[parser.position]
	parser.position++

	if letter >= 'a' && letter <= 'z' {
		letter -= 'a' - 'A'
	}

	if letter < 'A' || letter > 'Z' {
		return nil, false, 0, errors.New("control escape requires an ASCII letter")
	}

	return parser.literalEscape(letter & 0x1f)
}

func (parser *ecmaPatternParser) parseQuantifier(term *patternTerm) (uint64, bool, error) {
	term.quantified = true

	switch parser.units[parser.position] {
	case '*':
		parser.position++
		term.minimum = 0
		term.unbounded = true

		return 1, false, nil
	case '+':
		parser.position++
		term.minimum = 1
		term.unbounded = true

		return 1, false, nil
	case '?':
		parser.position++
		term.minimum = 0
		term.maximum = 1

		return 1, false, nil
	case '{':
		return parser.parseCountedQuantifier(term)
	default:
		return 0, false, errors.New("unknown quantifier")
	}
}

func (parser *ecmaPatternParser) parseCountedQuantifier(term *patternTerm) (uint64, bool, error) {
	parser.position++

	minimum, err := parser.parseRepeatEndpoint()
	if err != nil {
		return 0, false, err
	}

	term.minimum = minimum
	term.maximum = minimum
	factor := minimum

	if parser.consume(',') {
		if parser.consume('}') {
			term.unbounded = true

			return factor, true, nil
		}

		maximum, endpointErr := parser.parseRepeatEndpoint()
		if endpointErr != nil {
			return 0, false, endpointErr
		}

		if maximum < minimum {
			return 0, false, errors.New("counted-repeat maximum is below its minimum")
		}

		term.maximum = maximum
		factor = maximum
	}

	if !parser.consume('}') {
		return 0, false, errors.New("unterminated counted repeat")
	}

	return factor, true, nil
}

func (parser *ecmaPatternParser) parseRepeatEndpoint() (uint64, error) {
	start := parser.position

	var value uint64

	for parser.position < len(parser.units) {
		unit := parser.units[parser.position]
		if unit < '0' || unit > '9' {
			break
		}

		value = value*10 + uint64(unit-'0')
		if value > patternRepeatEndpointLimit {
			return 0, fmt.Errorf("counted-repeat endpoint exceeds %d", patternRepeatEndpointLimit)
		}

		parser.position++
	}

	if parser.position == start {
		return 0, errors.New("counted repeat is missing an endpoint")
	}

	return value, nil
}

func (parser *ecmaPatternParser) newAtom(kind patternAtomKind) (*patternAtom, error) {
	if err := parser.countNode(); err != nil {
		return nil, err
	}

	return &patternAtom{kind: kind}, nil
}

func (parser *ecmaPatternParser) countNode() error {
	parser.nodeCount++
	if parser.nodeCount > patternNodeLimit {
		return fmt.Errorf("pattern AST exceeds %d nodes", patternNodeLimit)
	}

	return nil
}

func (parser *ecmaPatternParser) enterGroup() error {
	parser.nesting++
	if parser.nesting > patternNestingLimit {
		return fmt.Errorf("pattern nesting exceeds %d", patternNestingLimit)
	}

	return nil
}

func (parser *ecmaPatternParser) leaveGroup() {
	parser.nesting--
}

func (parser *ecmaPatternParser) consume(expected uint16) bool {
	if parser.position == len(parser.units) || parser.units[parser.position] != expected {
		return false
	}

	parser.position++

	return true
}

func isQuantifierStart(unit uint16) bool {
	return unit == '*' || unit == '+' || unit == '?' || unit == '{'
}

func isPermittedIdentityEscape(unit uint16) bool {
	return unit <= 0x7f && (unit < '0' || unit > '9') && (unit < 'A' || unit > 'Z') &&
		(unit < 'a' || unit > 'z') && unit != '_'
}

func patternHexValue(unit uint16) (uint16, bool) {
	switch {
	case unit >= '0' && unit <= '9':
		return unit - '0', true
	case unit >= 'a' && unit <= 'f':
		return unit - 'a' + 10, true
	case unit >= 'A' && unit <= 'F':
		return unit - 'A' + 10, true
	default:
		return 0, false
	}
}

func singleClassPart(unit uint16) patternClassPart {
	return patternClassPart{ranges: []patternRange{{low: unit, high: unit}}}
}

func predefinedClass(escaped uint16) patternClassPart {
	part := patternClassPart{}

	switch escaped {
	case 'd', 'D':
		part.ranges = []patternRange{{low: '0', high: '9'}}
		part.negated = escaped == 'D'
	case 'w', 'W':
		part.ranges = []patternRange{
			{low: '0', high: '9'},
			{low: 'A', high: 'Z'},
			{low: '_', high: '_'},
			{low: 'a', high: 'z'},
		}
		part.negated = escaped == 'W'
	case 's', 'S':
		part.ranges = []patternRange{
			{low: 0x0009, high: 0x000d},
			{low: 0x0020, high: 0x0020},
			{low: 0x00a0, high: 0x00a0},
			{low: 0x1680, high: 0x1680},
			{low: 0x180e, high: 0x180e},
			{low: 0x2000, high: 0x200b},
			{low: 0x2028, high: 0x2029},
			{low: 0x202f, high: 0x202f},
			{low: 0x3000, high: 0x3000},
			{low: 0xfeff, high: 0xfeff},
		}
		part.negated = escaped == 'S'
	}

	return part
}

func expressionRepeatProduct(expression *patternExpression) uint64 {
	maximum := uint64(1)

	for _, alternative := range expression.alternatives {
		product := uint64(1)

		for _, term := range alternative.terms {
			child := uint64(1)
			if term.atom.kind == patternGroup {
				child = expressionRepeatProduct(term.atom.expression)
			}

			factor := uint64(1)
			if term.counted {
				factor = term.maximum
				if term.unbounded {
					factor = term.minimum
				}

				if factor == 0 {
					factor = 1
				}
			}

			if child > patternNestedRepeatLimit/factor {
				return patternNestedRepeatLimit + 1
			}

			candidate := child * factor
			if candidate > product {
				product = candidate
			}
		}

		if product > maximum {
			maximum = product
		}
	}

	return maximum
}

func patternExpressionBytes(expression *patternExpression) int {
	bytes := 4

	for alternativeIndex, alternative := range expression.alternatives {
		if alternativeIndex > 0 {
			bytes++
		}

		for _, term := range alternative.terms {
			bytes += patternTermBytes(term)
		}
	}

	return bytes
}

func patternTermBytes(term *patternTerm) int {
	bytes := patternAtomBytes(term.atom)
	if !term.quantified {
		return bytes
	}

	return 4 + bytes + patternRepeatBytes(term)
}

func patternAtomBytes(atom *patternAtom) int {
	switch atom.kind {
	case patternLiteral:
		return patternCodeUnitBytes(atom.literal)
	case patternDot:
		return len(`[^\r\n\x{2028}\x{2029}]`)
	case patternClassAtom:
		return patternClassBytes(atom.class)
	case patternStart, patternEnd, patternWordBoundary, patternNotWordBoundary:
		return 2
	case patternGroup:
		return 4 + patternExpressionBytes(atom.expression)
	default:
		return 0
	}
}

func patternRepeatBytes(term *patternTerm) int {
	bytes := 1

	if term.counted {
		minimum := strconv.FormatUint(term.minimum, 10)
		switch {
		case term.unbounded:
			bytes = 3 + len(minimum)
		case term.minimum == term.maximum:
			bytes = 2 + len(minimum)
		default:
			bytes = 3 + len(minimum) + len(strconv.FormatUint(term.maximum, 10))
		}
	}

	if !term.greedy {
		bytes++
	}

	return bytes
}

func patternCodeUnitBytes(unit uint16) int {
	value := uint32(unit)
	if unit >= 0xd800 && unit <= 0xdfff {
		value = 0x10000 + value - 0xd800
	}

	return 4 + len(strconv.FormatUint(uint64(value), 16))
}

type patternMatcherRange struct {
	low  uint32
	high uint32
}

func patternClassBytes(class patternClass) int {
	var ranges []patternMatcherRange
	for _, part := range class.parts {
		partRanges := patternMatcherRanges(part.ranges)
		if part.negated {
			partRanges = complementPatternMatcherRanges(partRanges)
		}

		ranges = append(ranges, partRanges...)
	}

	ranges = normalizePatternMatcherRanges(ranges)
	if class.negated {
		ranges = complementPatternMatcherRanges(ranges)
	}

	if len(ranges) == 0 {
		return len(`(?:\b\B)`)
	}

	bytes := 2
	for _, characterRange := range ranges {
		bytes += patternCodePointBytes(characterRange.low)
		if characterRange.low != characterRange.high {
			bytes += 1 + patternCodePointBytes(characterRange.high)
		}
	}

	return bytes
}

func patternMatcherRanges(ranges []patternRange) []patternMatcherRange {
	mapped := make([]patternMatcherRange, 0, len(ranges)+1)
	for _, characterRange := range ranges {
		low := uint32(characterRange.low)
		high := uint32(characterRange.high)

		if low <= 0xd7ff {
			mapped = append(mapped, patternMatcherRange{low: low, high: min(high, 0xd7ff)})
		}

		if high >= 0xd800 && low <= 0xdfff {
			surrogateLow := max(low, uint32(0xd800))
			surrogateHigh := min(high, uint32(0xdfff))
			mapped = append(mapped, patternMatcherRange{
				low:  0x10000 + surrogateLow - 0xd800,
				high: 0x10000 + surrogateHigh - 0xd800,
			})
		}

		if high >= 0xe000 {
			mapped = append(mapped, patternMatcherRange{low: max(low, uint32(0xe000)), high: high})
		}
	}

	return normalizePatternMatcherRanges(mapped)
}

func normalizePatternMatcherRanges(ranges []patternMatcherRange) []patternMatcherRange {
	if len(ranges) < 2 {
		return ranges
	}

	sort.Slice(ranges, func(left, right int) bool {
		if ranges[left].low == ranges[right].low {
			return ranges[left].high < ranges[right].high
		}

		return ranges[left].low < ranges[right].low
	})

	merged := []patternMatcherRange{ranges[0]}
	for _, candidate := range ranges[1:] {
		last := &merged[len(merged)-1]
		if candidate.low <= last.high+1 {
			last.high = max(last.high, candidate.high)

			continue
		}

		merged = append(merged, candidate)
	}

	return merged
}

func complementPatternMatcherRanges(ranges []patternMatcherRange) []patternMatcherRange {
	ranges = normalizePatternMatcherRanges(ranges)
	complement := make([]patternMatcherRange, 0, len(ranges)+1)
	next := uint32(0)

	for _, excluded := range ranges {
		if next < excluded.low {
			complement = append(complement, patternMatcherRange{low: next, high: excluded.low - 1})
		}

		if excluded.high == unicode.MaxRune {
			return complement
		}

		next = excluded.high + 1
	}

	if next <= unicode.MaxRune {
		complement = append(complement, patternMatcherRange{low: next, high: unicode.MaxRune})
	}

	return complement
}

func patternCodePointBytes(value uint32) int {
	return 4 + len(strconv.FormatUint(uint64(value), 16))
}
