//nolint:godoclint,mnd // Parser helpers are grammar productions; numeric bases and widths are syntax constants.
package patternsyntax

import (
	"unicode/utf8"
)

type parser struct {
	source       string
	position     int
	tree         Tree
	depth        int
	captureCount uint64
	references   []decimalReference
}

type decimalReference struct {
	offset int
	value  uint64
}

type classAtom struct {
	item      ClassItem
	singleton bool
	span      Span
}

// Parse parses one pattern in the supported ECMAScript 5.1 grammar.
func Parse(source string) (*Tree, error) {
	if len(source) > MaximumSourceBytes {
		return nil, complexityError(0, "source bytes", MaximumSourceBytes, len(source))
	}

	if !utf8.ValidString(source) {
		return nil, syntaxError(firstInvalidUTF8(source), "source is not valid UTF-8")
	}

	state := parser{source: source}

	root, err := state.parseExpression()
	if err != nil {
		return nil, err
	}

	if state.position != len(source) {
		return nil, syntaxError(state.position, "unmatched closing parenthesis")
	}

	state.tree.Root = root
	if err := state.classifyDecimalReferences(); err != nil {
		return nil, err
	}

	if err := state.validateLookaheadPlacement(); err != nil {
		return nil, err
	}

	return &state.tree, nil
}

func (state *parser) parseExpression() (NodeID, error) {
	start := state.position
	alternatives := make([]NodeID, 0, 1)

	for {
		alternative, err := state.parseAlternative()
		if err != nil {
			return 0, err
		}

		alternatives = append(alternatives, alternative)

		if state.position == len(state.source) || state.source[state.position] != '|' {
			break
		}

		state.position++
	}

	return state.addNode(Node{
		Kind: KindExpression, Span: Span{Start: start, End: state.position}, Children: alternatives,
	})
}

func (state *parser) parseAlternative() (NodeID, error) {
	start := state.position
	terms := make([]NodeID, 0)

	for state.position < len(state.source) {
		switch state.source[state.position] {
		case '|', ')':
			return state.addNode(Node{
				Kind: KindAlternative, Span: Span{Start: start, End: state.position}, Children: terms,
			})
		case '*', '+', '?':
			return 0, syntaxError(state.position, "quantifier has no atom")
		case '{', '}':
			return 0, syntaxError(state.position, "unescaped brace is not allowed here")
		}

		term, err := state.parseTerm()
		if err != nil {
			return 0, err
		}

		terms = append(terms, term)
	}

	return state.addNode(Node{
		Kind: KindAlternative, Span: Span{Start: start, End: state.position}, Children: terms,
	})
}

//nolint:cyclop // Term dispatch directly mirrors the closed grammar.
func (state *parser) parseTerm() (NodeID, error) {
	start := state.position

	switch state.source[state.position] {
	case '^':
		state.position++

		return state.addNode(Node{Kind: KindBeginInput, Span: Span{Start: start, End: state.position}})
	case '$':
		state.position++

		return state.addNode(Node{Kind: KindEndInput, Span: Span{Start: start, End: state.position}})
	case '\\':
		if state.position+1 < len(state.source) {
			switch state.source[state.position+1] {
			case 'b':
				state.position += 2

				return state.addNode(Node{
					Kind: KindWordBoundary, Span: Span{Start: start, End: state.position},
				})
			case 'B':
				state.position += 2

				return state.addNode(Node{
					Kind: KindNotWordBoundary, Span: Span{Start: start, End: state.position},
				})
			}
		}
	}

	atom, err := state.parseAtom()
	if err != nil {
		return 0, err
	}

	if state.position == len(state.source) || !isQuantifierStart(state.source[state.position]) {
		return atom, nil
	}

	if kind := state.tree.Nodes[atom].Kind; kind == KindPositiveLookahead || kind == KindNegativeLookahead {
		return 0, syntaxError(state.position, "lookahead cannot be quantified")
	}

	return state.parseQuantifier(atom)
}

func (state *parser) parseAtom() (NodeID, error) {
	start := state.position

	switch state.source[state.position] {
	case '.':
		state.position++

		return state.addNode(Node{Kind: KindDot, Span: Span{Start: start, End: state.position}})
	case '[':
		return state.parseClass()
	case '(':
		return state.parseGroup()
	case '\\':
		return state.parseEscapeNode()
	case ')':
		return 0, syntaxError(start, "unmatched closing parenthesis")
	case ']':
		return 0, syntaxError(start, "unescaped closing bracket is not allowed here")
	}

	character, size := utf8.DecodeRuneInString(state.source[state.position:])
	state.position += size

	return state.addNode(Node{
		Kind: KindLiteral, Span: Span{Start: start, End: state.position}, Value: character,
	})
}

//nolint:cyclop // Group-prefix classification is intentionally explicit.
func (state *parser) parseGroup() (NodeID, error) {
	start := state.position

	state.position++
	if state.position < len(state.source) && state.source[state.position] == '*' {
		return 0, foreignError(start, "control-verb groups are not ECMAScript 5.1 syntax")
	}

	kind := KindCapture

	if state.position < len(state.source) && state.source[state.position] == '?' {
		if state.position+1 >= len(state.source) {
			return 0, syntaxError(start, "incomplete group prefix")
		}

		switch state.source[state.position+1] {
		case ':':
			kind = KindGroup
			state.position += 2
		case '=':
			kind = KindPositiveLookahead
			state.position += 2
		case '!':
			kind = KindNegativeLookahead
			state.position += 2
		case '<':
			return 0, foreignError(start, "lookbehind and named groups are not ECMAScript 5.1 syntax")
		case '>':
			return 0, foreignError(start, "atomic groups are not ECMAScript 5.1 syntax")
		case '#', '(':
			return 0, foreignError(start, "this group form is not ECMAScript 5.1 syntax")
		default:
			return 0, foreignError(start, "inline modes and this group form are not ECMAScript 5.1 syntax")
		}
	} else {
		state.captureCount++
	}

	if state.depth == MaximumNestingDepth {
		return 0, complexityError(start, "nesting depth", MaximumNestingDepth, state.depth+1)
	}

	state.depth++
	child, err := state.parseExpression()
	state.depth--

	if err != nil {
		return 0, err
	}

	if state.position == len(state.source) || state.source[state.position] != ')' {
		return 0, syntaxError(start, "group has no closing parenthesis")
	}

	state.position++

	return state.addNode(Node{
		Kind: kind, Span: Span{Start: start, End: state.position}, Children: []NodeID{child},
	})
}

//nolint:cyclop,gocyclo // Escape classification is a closed, flat grammar table.
func (state *parser) parseEscapeNode() (NodeID, error) {
	start := state.position

	state.position++
	if state.position == len(state.source) {
		return 0, syntaxError(start, "dangling escape")
	}

	escaped := state.source[state.position]
	state.position++

	switch escaped {
	case 'd':
		return state.escapeSetNode(start, KindDigit)
	case 'D':
		return state.escapeSetNode(start, KindNotDigit)
	case 's':
		return state.escapeSetNode(start, KindSpace)
	case 'S':
		return state.escapeSetNode(start, KindNotSpace)
	case 'w':
		return state.escapeSetNode(start, KindWord)
	case 'W':
		return state.escapeSetNode(start, KindNotWord)
	case 'f':
		return state.escapeLiteralNode(start, '\f')
	case 'n':
		return state.escapeLiteralNode(start, '\n')
	case 'r':
		return state.escapeLiteralNode(start, '\r')
	case 't':
		return state.escapeLiteralNode(start, '\t')
	case 'v':
		return state.escapeLiteralNode(start, '\v')
	case '0':
		if state.position < len(state.source) && isDecimal(state.source[state.position]) {
			return 0, syntaxError(state.position, "decimal digit cannot follow \\0")
		}

		return state.escapeLiteralNode(start, 0)
	case 'x':
		return state.parseHexEscapeNode(start, 2)
	case 'u':
		if state.position < len(state.source) && state.source[state.position] == '{' {
			return 0, foreignError(start, "Unicode code-point escapes are not ECMAScript 5.1 syntax")
		}

		return state.parseHexEscapeNode(start, 4)
	case 'c':
		if state.position == len(state.source) {
			return 0, syntaxError(start, "control escape must be followed by a letter")
		}

		value, ok := controlEscapeValue(state.source[state.position])
		if !ok {
			return 0, syntaxError(start, "control escape must be followed by a letter")
		}

		state.position++

		return state.escapeLiteralNode(start, value)
	case 'p', 'P':
		return 0, foreignError(start, "Unicode property escapes are not ECMAScript 5.1 syntax")
	case 'k':
		if state.position < len(state.source) && state.source[state.position] == '<' {
			return 0, foreignError(start, "named backreferences are not ECMAScript 5.1 syntax")
		}

		return 0, syntaxError(start, "unknown or forbidden identity escape")
	case 'A', 'z', 'C', 'Q', 'E':
		return 0, foreignError(start, "foreign regular-expression escape")
	}

	if isDecimal(escaped) {
		state.parseDecimalReference(start, escaped)

		return state.escapeLiteralNode(start, 0)
	}

	character, size := utf8.DecodeRuneInString(state.source[state.position-1:])
	if size > 1 {
		state.position += size - 1
	}

	if !isIdentityEscape(character) {
		return 0, syntaxError(start, "unknown or forbidden identity escape")
	}

	return state.escapeLiteralNode(start, character)
}

func (state *parser) parseHexEscapeNode(start int, digits int) (NodeID, error) {
	value, err := state.parseHexDigits(start, digits)
	if err != nil {
		return 0, err
	}

	if value >= 0xd800 && value <= 0xdfff {
		return 0, unsupportedError(start, "surrogate escapes are unsupported")
	}

	return state.escapeLiteralNode(start, rune(value))
}

//nolint:cyclop,gocognit // Class range parsing keeps source-order errors in one grammar production.
func (state *parser) parseClass() (NodeID, error) {
	start := state.position
	state.position++

	negated := false
	if state.position < len(state.source) && state.source[state.position] == '^' {
		negated = true
		state.position++
	}

	items := make([]ClassItem, 0)

	for {
		if state.position == len(state.source) {
			return 0, syntaxError(start, "character class has no closing bracket")
		}

		if state.source[state.position] == ']' {
			state.position++

			return state.addNode(Node{
				Kind: KindClass, Span: Span{Start: start, End: state.position},
				Negated: negated, ClassItems: items,
			})
		}

		if state.position+1 < len(state.source) && state.source[state.position] == '&' &&
			state.source[state.position+1] == '&' {
			return 0, foreignError(state.position, "character-class set operations are not ECMAScript 5.1 syntax")
		}

		if state.position+1 < len(state.source) && state.source[state.position] == '[' &&
			state.source[state.position+1] == ':' {
			return 0, foreignError(state.position, "POSIX character classes are not ECMAScript 5.1 syntax")
		}

		left, err := state.parseClassAtom()
		if err != nil {
			return 0, err
		}

		if state.position < len(state.source) && state.source[state.position] == '-' &&
			state.position+1 < len(state.source) && state.source[state.position+1] != ']' {
			dash := state.position
			if state.source[state.position+1] == '-' {
				return 0, foreignError(dash, "character-class set operations are not ECMAScript 5.1 syntax")
			}

			state.position++

			right, parseErr := state.parseClassAtom()
			if parseErr != nil {
				return 0, parseErr
			}

			if !left.singleton || !right.singleton {
				return 0, syntaxError(dash, "character-class range endpoints must be single characters")
			}

			if left.item.Low > right.item.Low {
				return 0, syntaxError(dash, "character-class range is reversed")
			}

			items = append(items, ClassItem{
				Kind: ClassItemRange, Low: left.item.Low, High: right.item.Low,
			})

			continue
		}

		items = append(items, left.item)
	}
}

//nolint:cyclop,gocyclo // Class escapes are a closed, flat grammar table.
func (state *parser) parseClassAtom() (classAtom, error) {
	start := state.position
	if state.source[state.position] == '-' {
		state.position++

		return singletonClassAtom('-', start, state.position), nil
	}

	if state.source[state.position] != '\\' {
		character, size := utf8.DecodeRuneInString(state.source[state.position:])
		state.position += size

		return singletonClassAtom(character, start, state.position), nil
	}

	state.position++
	if state.position == len(state.source) {
		return classAtom{}, syntaxError(start, "dangling character-class escape")
	}

	escaped := state.source[state.position]
	state.position++

	switch escaped {
	case 'd':
		return setClassAtom(ClassItemDigit, start, state.position), nil
	case 'D':
		return setClassAtom(ClassItemNotDigit, start, state.position), nil
	case 's':
		return setClassAtom(ClassItemSpace, start, state.position), nil
	case 'S':
		return setClassAtom(ClassItemNotSpace, start, state.position), nil
	case 'w':
		return setClassAtom(ClassItemWord, start, state.position), nil
	case 'W':
		return setClassAtom(ClassItemNotWord, start, state.position), nil
	case 'b':
		return singletonClassAtom('\b', start, state.position), nil
	case 'f':
		return singletonClassAtom('\f', start, state.position), nil
	case 'n':
		return singletonClassAtom('\n', start, state.position), nil
	case 'r':
		return singletonClassAtom('\r', start, state.position), nil
	case 't':
		return singletonClassAtom('\t', start, state.position), nil
	case 'v':
		return singletonClassAtom('\v', start, state.position), nil
	case '0':
		if state.position < len(state.source) && isDecimal(state.source[state.position]) {
			return classAtom{}, syntaxError(state.position, "decimal digit cannot follow \\0")
		}

		return singletonClassAtom(0, start, state.position), nil
	case 'x':
		value, err := state.parseHexDigits(start, 2)
		if err != nil {
			return classAtom{}, err
		}

		return state.checkedClassEscape(start, value)
	case 'u':
		if state.position < len(state.source) && state.source[state.position] == '{' {
			return classAtom{}, foreignError(start, "Unicode code-point escapes are not ECMAScript 5.1 syntax")
		}

		value, err := state.parseHexDigits(start, 4)
		if err != nil {
			return classAtom{}, err
		}

		return state.checkedClassEscape(start, value)
	case 'c':
		if state.position == len(state.source) {
			return classAtom{}, syntaxError(start, "control escape must be followed by a letter")
		}

		value, ok := controlEscapeValue(state.source[state.position])
		if !ok {
			return classAtom{}, syntaxError(start, "control escape must be followed by a letter")
		}

		state.position++

		return singletonClassAtom(value, start, state.position), nil
	case 'p', 'P':
		return classAtom{}, foreignError(start, "Unicode property escapes are not ECMAScript 5.1 syntax")
	case 'A', 'z', 'C', 'Q', 'E', 'k':
		return classAtom{}, foreignError(start, "foreign regular-expression escape")
	}

	if isDecimal(escaped) {
		state.parseDecimalReference(start, escaped)

		return singletonClassAtom(0, start, state.position), nil
	}

	character, size := utf8.DecodeRuneInString(state.source[state.position-1:])
	if size > 1 {
		state.position += size - 1
	}

	if !isIdentityEscape(character) {
		return classAtom{}, syntaxError(start, "unknown or forbidden identity escape")
	}

	return singletonClassAtom(character, start, state.position), nil
}

func controlEscapeValue(value byte) (rune, bool) {
	switch {
	case value >= 'A' && value <= 'Z':
		return rune(value - 'A' + 1), true
	case value >= 'a' && value <= 'z':
		return rune(value - 'a' + 1), true
	default:
		return 0, false
	}
}

func (state *parser) checkedClassEscape(start int, value uint64) (classAtom, error) {
	if value >= 0xd800 && value <= 0xdfff {
		return classAtom{}, unsupportedError(start, "surrogate escapes are unsupported")
	}

	return singletonClassAtom(rune(value), start, state.position), nil
}

func (state *parser) parseHexDigits(start int, count int) (uint64, error) {
	if len(state.source)-state.position < count {
		return 0, syntaxError(start, "incomplete hexadecimal escape")
	}

	var value uint64

	for range count {
		digit, ok := hexValue(state.source[state.position])
		if !ok {
			return 0, syntaxError(state.position, "invalid hexadecimal escape digit")
		}

		value = value*16 + uint64(digit)
		state.position++
	}

	return value, nil
}

func (state *parser) parseDecimalReference(start int, first byte) {
	value := uint64(first - '0')

	for state.position < len(state.source) && isDecimal(state.source[state.position]) {
		digit := uint64(state.source[state.position] - '0')
		if value > (^uint64(0)-digit)/10 {
			value = ^uint64(0)
		} else {
			value = value*10 + digit
		}

		state.position++
	}

	state.references = append(state.references, decimalReference{offset: start, value: value})
}

//nolint:cyclop,gocognit,nestif // Counted-repeat forms share ordering-sensitive source parsing.
func (state *parser) parseQuantifier(child NodeID) (NodeID, error) {
	start := state.tree.Nodes[child].Span.Start
	quantifierStart := state.position
	repeat := Repeat{}

	switch state.source[state.position] {
	case '*':
		repeat.Unbounded = true
		state.position++
	case '+':
		repeat.Minimum = 1
		repeat.Unbounded = true
		state.position++
	case '?':
		repeat.Maximum = 1
		state.position++
	case '{':
		repeat.Counted = true
		state.position++

		minimum, err := state.parseRepeatEndpoint(quantifierStart)
		if err != nil {
			return 0, err
		}

		repeat.Minimum = minimum
		repeat.Maximum = minimum

		if state.position < len(state.source) && state.source[state.position] == ',' {
			state.position++
			if state.position < len(state.source) && state.source[state.position] == '}' {
				repeat.Unbounded = true
			} else {
				maximum, err := state.parseRepeatEndpoint(quantifierStart)
				if err != nil {
					return 0, err
				}

				repeat.Maximum = maximum
				if minimum > maximum {
					return 0, syntaxError(quantifierStart, "repeat minimum exceeds maximum")
				}
			}
		}

		if state.position == len(state.source) || state.source[state.position] != '}' {
			return 0, syntaxError(quantifierStart, "counted repeat has no closing brace")
		}

		state.position++
	}

	if state.position < len(state.source) && state.source[state.position] == '?' {
		repeat.Lazy = true
		state.position++
	}

	if state.position < len(state.source) && isQuantifierStart(state.source[state.position]) {
		if state.source[state.position] == '+' {
			return 0, foreignError(state.position, "possessive quantifiers are not ECMAScript 5.1 syntax")
		}

		return 0, syntaxError(state.position, "duplicated quantifier")
	}

	if repeat.Counted {
		factor := repeat.Maximum
		if repeat.Unbounded {
			factor = repeat.Minimum
		}

		if factor < 1 {
			factor = 1
		}

		childProduct := state.maximumRepeatProduct(child)
		if childProduct > MaximumRepeatProduct/factor {
			return 0, complexityError(
				quantifierStart,
				"cumulative nested repeat product",
				MaximumRepeatProduct,
				childProduct*factor,
			)
		}
	}

	return state.addNode(Node{
		Kind: KindRepeat, Span: Span{Start: start, End: state.position},
		Children: []NodeID{child}, Repeat: repeat,
	})
}

func (state *parser) parseRepeatEndpoint(start int) (int, error) {
	if state.position == len(state.source) || !isDecimal(state.source[state.position]) {
		return 0, syntaxError(start, "counted repeat endpoint is missing")
	}

	var value uint64

	for state.position < len(state.source) && isDecimal(state.source[state.position]) {
		digit := uint64(state.source[state.position] - '0')
		if value > (^uint64(0)-digit)/10 {
			value = ^uint64(0)
		} else {
			value = value*10 + digit
		}

		state.position++
	}

	if value > MaximumRepeatEndpoint {
		return 0, complexityErrorUint(start, "repeat endpoint", MaximumRepeatEndpoint, value)
	}

	return int(value), nil
}

func (state *parser) maximumRepeatProduct(nodeID NodeID) int {
	node := state.tree.Nodes[nodeID]

	maximum := 1
	for _, child := range node.Children {
		maximum = max(maximum, state.maximumRepeatProduct(child))
	}

	if node.Kind != KindRepeat || !node.Repeat.Counted {
		return maximum
	}

	factor := node.Repeat.Maximum
	if node.Repeat.Unbounded {
		factor = node.Repeat.Minimum
	}

	return maximum * max(factor, 1)
}

func (state *parser) classifyDecimalReferences() error {
	if len(state.references) == 0 {
		return nil
	}

	reference := state.references[0]
	if reference.value <= state.captureCount {
		return unsupportedError(reference.offset, "backreferences are unsupported")
	}

	return syntaxError(reference.offset, "decimal escape exceeds the pattern's capture count")
}

//nolint:cyclop // The exact accepted root shape is clearer as direct structural checks.
func (state *parser) validateLookaheadPlacement() error {
	firstLookahead := -1
	total := 0

	for _, node := range state.tree.Nodes {
		if node.Kind != KindPositiveLookahead && node.Kind != KindNegativeLookahead {
			continue
		}

		total++

		if firstLookahead == -1 || node.Span.Start < firstLookahead {
			firstLookahead = node.Span.Start
		}
	}

	if total == 0 {
		return nil
	}

	root := state.tree.Nodes[state.tree.Root]
	if len(root.Children) != 1 {
		return unsupportedError(firstLookahead, "lookahead is only supported in the leading anchored form")
	}

	alternative := state.tree.Nodes[root.Children[0]]
	if len(alternative.Children) < 2 {
		return unsupportedError(firstLookahead, "lookahead is only supported in the leading anchored form")
	}

	first := state.tree.Nodes[alternative.Children[0]]
	if first.Kind != KindBeginInput || first.Span.Start != 0 {
		return unsupportedError(firstLookahead, "lookahead is only supported immediately after an initial ^")
	}

	prefixCount := 0

	for _, child := range alternative.Children[1:] {
		node := state.tree.Nodes[child]
		if node.Kind != KindPositiveLookahead && node.Kind != KindNegativeLookahead {
			break
		}

		prefixCount++
		if prefixCount > MaximumLeadingAssertions {
			return complexityError(
				node.Span.Start,
				"leading assertions",
				MaximumLeadingAssertions,
				prefixCount,
			)
		}
	}

	if prefixCount == 0 || prefixCount != total {
		return unsupportedError(firstLookahead, "lookaheads must be consecutive top-level assertions after ^")
	}

	return nil
}

func (state *parser) addNode(node Node) (NodeID, error) {
	observed := len(state.tree.Nodes) + 1
	if observed > MaximumNodes {
		return 0, complexityError(node.Span.Start, "AST nodes", MaximumNodes, observed)
	}

	id := NodeID(len(state.tree.Nodes))
	state.tree.Nodes = append(state.tree.Nodes, node)

	return id, nil
}

func (state *parser) escapeSetNode(start int, kind Kind) (NodeID, error) {
	return state.addNode(Node{Kind: kind, Span: Span{Start: start, End: state.position}})
}

func (state *parser) escapeLiteralNode(start int, value rune) (NodeID, error) {
	return state.addNode(Node{
		Kind: KindLiteral, Span: Span{Start: start, End: state.position}, Value: value,
	})
}

func singletonClassAtom(value rune, start int, end int) classAtom {
	return classAtom{
		item:      ClassItem{Kind: ClassItemRange, Low: value, High: value},
		singleton: true,
		span:      Span{Start: start, End: end},
	}
}

func setClassAtom(kind ClassItemKind, start int, end int) classAtom {
	return classAtom{item: ClassItem{Kind: kind}, span: Span{Start: start, End: end}}
}

func syntaxError(offset int, message string) *Error {
	return &Error{Kind: ErrorInvalidSyntax, Offset: offset, Message: message}
}

func unsupportedError(offset int, message string) *Error {
	return &Error{Kind: ErrorUnsupported, Offset: offset, Message: message}
}

func foreignError(offset int, message string) *Error {
	return &Error{Kind: ErrorForeignSyntax, Offset: offset, Message: message}
}

func complexityError(offset int, limit string, maximum int, observed int) *Error {
	return complexityErrorUint(offset, limit, uint64(maximum), uint64(observed))
}

func complexityErrorUint(offset int, limit string, maximum uint64, observed uint64) *Error {
	return &Error{
		Kind: ErrorTooComplex, Offset: offset, Message: "resource limit exceeded",
		Limit: limit, Maximum: maximum, Observed: observed,
	}
}

func firstInvalidUTF8(source string) int {
	for index := 0; index < len(source); {
		_, size := utf8.DecodeRuneInString(source[index:])
		if size == 1 && source[index] >= utf8.RuneSelf {
			return index
		}

		index += size
	}

	return 0
}

func isQuantifierStart(character byte) bool {
	return character == '*' || character == '+' || character == '?' || character == '{'
}

func isDecimal(character byte) bool {
	return character >= '0' && character <= '9'
}

func hexValue(character byte) (byte, bool) {
	switch {
	case character >= '0' && character <= '9':
		return character - '0', true
	case character >= 'a' && character <= 'f':
		return character - 'a' + 10, true
	case character >= 'A' && character <= 'F':
		return character - 'A' + 10, true
	default:
		return 0, false
	}
}

func isIdentityEscape(character rune) bool {
	if character > 0x7f {
		return true
	}

	if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' || character == '_' {
		return false
	}

	return true
}
