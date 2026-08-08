package schematest

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// hexadecimalRadix is the numeric base of Unicode escape digits.
const hexadecimalRadix = 16

// strictJSONParser parses one exact JSON value from UTF-8 bytes.
type strictJSONParser struct {
	source   []byte
	position int
}

// strictJSONContainerState identifies the next token expected by an open container.
type strictJSONContainerState uint8

const (
	// jsonArrayStart permits an element or the closing bracket.
	jsonArrayStart strictJSONContainerState = iota
	// jsonArrayValue requires an element after a comma.
	jsonArrayValue
	// jsonArrayEnd requires a comma or the closing bracket.
	jsonArrayEnd
	// jsonObjectStart permits a member name or the closing brace.
	jsonObjectStart
	// jsonObjectName requires a member name after a comma.
	jsonObjectName
	// jsonObjectEnd requires a comma or the closing brace.
	jsonObjectEnd
)

// strictJSONContainerFrame holds one open container's parsing state.
type strictJSONContainerFrame struct {
	value *jsonValue
	state strictJSONContainerState
}

// parseStrictJSON parses exactly one complete interoperable JSON value.
//
//nolint:cyclop,gocognit // The explicit container state machine replaces recursive descent.
func parseStrictJSON(source []byte) (*jsonValue, error) {
	if !utf8.Valid(source) {
		return nil, errors.New("JSON input is not valid UTF-8")
	}

	parser := strictJSONParser{source: source}
	parser.skipWhitespace()

	if parser.position == len(parser.source) {
		return nil, errors.New("expected JSON value")
	}

	value, state, container, err := parser.parseValueToken()
	if err != nil {
		return nil, err
	}

	stack := make([]strictJSONContainerFrame, 0)
	if container {
		stack = append(stack, strictJSONContainerFrame{value: value, state: state})
	}

	for len(stack) > 0 {
		frame := &stack[len(stack)-1]

		parser.skipWhitespace()

		switch frame.state {
		case jsonArrayStart:
			if parser.consume(']') {
				stack = stack[:len(stack)-1]

				continue
			}

			frame.state = jsonArrayValue
		case jsonArrayEnd:
			if parser.consume(']') {
				stack = stack[:len(stack)-1]

				continue
			}

			if !parser.consume(',') {
				return nil, fmt.Errorf("expected ',' or ']' at byte %d", parser.position)
			}

			parser.skipWhitespace()

			frame.state = jsonArrayValue
		case jsonObjectStart:
			if parser.consume('}') {
				stack = stack[:len(stack)-1]

				continue
			}

			frame.state = jsonObjectName
		case jsonObjectEnd:
			if parser.consume('}') {
				stack = stack[:len(stack)-1]

				continue
			}

			if !parser.consume(',') {
				return nil, fmt.Errorf("expected ',' or '}' at byte %d", parser.position)
			}

			parser.skipWhitespace()

			frame.state = jsonObjectName
		}

		switch frame.state {
		case jsonArrayValue:
			child, childState, childContainer, parseErr := parser.parseValueToken()
			if parseErr != nil {
				return nil, parseErr
			}

			frame.value.array = append(frame.value.array, child)

			frame.state = jsonArrayEnd
			if childContainer {
				stack = append(stack, strictJSONContainerFrame{value: child, state: childState})
			}
		case jsonObjectName:
			if parser.position == len(parser.source) || parser.source[parser.position] != '"' {
				return nil, fmt.Errorf("expected object member name at byte %d", parser.position)
			}

			name, nameErr := parser.parseString()
			if nameErr != nil {
				return nil, nameErr
			}

			parser.skipWhitespace()

			if !parser.consume(':') {
				return nil, fmt.Errorf("expected ':' at byte %d", parser.position)
			}

			parser.skipWhitespace()

			child, childState, childContainer, parseErr := parser.parseValueToken()
			if parseErr != nil {
				return nil, parseErr
			}

			if _, duplicate := frame.value.object[name]; duplicate {
				return nil, fmt.Errorf("duplicate object member %q at byte %d", name, parser.position)
			}

			frame.value.object[name] = child

			frame.state = jsonObjectEnd
			if childContainer {
				stack = append(stack, strictJSONContainerFrame{value: child, state: childState})
			}
		}
	}

	parser.skipWhitespace()

	if parser.position != len(parser.source) {
		return nil, fmt.Errorf("trailing data after JSON value at byte %d", parser.position)
	}

	return value, nil
}

// parseValueToken parses one scalar or opens one container.
//
//nolint:cyclop // JSON's fixed token alternatives are intentionally explicit.
func (parser *strictJSONParser) parseValueToken() (*jsonValue, strictJSONContainerState, bool, error) {
	if parser.position == len(parser.source) {
		return nil, 0, false, fmt.Errorf("expected JSON value at byte %d", parser.position)
	}

	switch parser.source[parser.position] {
	case 'n':
		value, err := parser.parseLiteral("null", &jsonValue{kind: jsonNull})

		return value, 0, false, err
	case 'f':
		value, err := parser.parseLiteral("false", &jsonValue{kind: jsonBoolean})

		return value, 0, false, err
	case 't':
		value, err := parser.parseLiteral("true", &jsonValue{kind: jsonBoolean, boolean: true})

		return value, 0, false, err
	case '"':
		text, err := parser.parseString()

		return &jsonValue{kind: jsonString, text: text}, 0, false, err
	case '[':
		parser.position++

		return &jsonValue{kind: jsonArray, array: make([]*jsonValue, 0)}, jsonArrayStart, true, nil
	case '{':
		parser.position++

		return &jsonValue{kind: jsonObject, object: make(map[string]*jsonValue)}, jsonObjectStart, true, nil
	default:
		if parser.source[parser.position] != '-' && !isDecimalDigit(parser.source[parser.position]) {
			return nil, 0, false, fmt.Errorf("expected JSON value at byte %d", parser.position)
		}

		value, err := parser.parseNumber()

		return value, 0, false, err
	}
}

// parseLiteral parses one fixed JSON literal.
func (parser *strictJSONParser) parseLiteral(literal string, value *jsonValue) (*jsonValue, error) {
	if !strings.HasPrefix(string(parser.source[parser.position:]), literal) {
		return nil, fmt.Errorf("invalid JSON literal at byte %d", parser.position)
	}

	parser.position += len(literal)

	return value, nil
}

// parseNumber parses one exact JSON number.
func (parser *strictJSONParser) parseNumber() (*jsonValue, error) {
	start := parser.position

	parts, err := scanExactNumber(string(parser.source[start:]))
	if err != nil {
		return nil, fmt.Errorf("JSON number at byte %d: %w", start, err)
	}

	parser.position += parts.end

	number, err := parseExactNumber(string(parser.source[start:parser.position]))
	if err != nil {
		return nil, fmt.Errorf("JSON number at byte %d: %w", start, err)
	}

	return &jsonValue{kind: jsonNumber, number: number}, nil
}

// parseString parses and decodes one JSON string.
func (parser *strictJSONParser) parseString() (string, error) {
	start := parser.position
	parser.position++
	decoded := make([]byte, 0)

	for parser.position < len(parser.source) {
		value := parser.source[parser.position]
		switch {
		case value == '"':
			parser.position++

			return string(decoded), nil
		case value == '\\':
			parser.position++

			escaped, err := parser.parseEscape()
			if err != nil {
				return "", err
			}

			decoded = append(decoded, escaped...)
		case value < jsonControlLimit:
			return "", fmt.Errorf("unescaped control character at byte %d", parser.position)
		default:
			_, size := utf8.DecodeRune(parser.source[parser.position:])
			decoded = append(decoded, parser.source[parser.position:parser.position+size]...)
			parser.position += size
		}
	}

	return "", fmt.Errorf("unterminated JSON string at byte %d", start)
}

// parseEscape parses one JSON string escape after its backslash.
func (parser *strictJSONParser) parseEscape() ([]byte, error) {
	if parser.position == len(parser.source) {
		return nil, errors.New("unterminated JSON escape")
	}

	escapePosition := parser.position
	escaped := parser.source[parser.position]
	parser.position++

	switch escaped {
	case '"', '\\', '/':
		return []byte{escaped}, nil
	case 'b':
		return []byte{'\b'}, nil
	case 'f':
		return []byte{'\f'}, nil
	case 'n':
		return []byte{'\n'}, nil
	case 'r':
		return []byte{'\r'}, nil
	case 't':
		return []byte{'\t'}, nil
	case 'u':
		return parser.parseUnicodeEscape(escapePosition)
	default:
		return nil, fmt.Errorf("invalid JSON escape at byte %d", escapePosition)
	}
}

// parseUnicodeEscape parses one Unicode scalar, including a required surrogate pair.
func (parser *strictJSONParser) parseUnicodeEscape(escapePosition int) ([]byte, error) {
	first, err := parser.parseHexQuad()
	if err != nil {
		return nil, err
	}

	if first >= 0xdc00 && first <= 0xdfff {
		return nil, fmt.Errorf("unpaired low surrogate at byte %d", escapePosition)
	}

	if first < 0xd800 || first > 0xdbff {
		return utf8.AppendRune(nil, rune(first)), nil
	}

	value, err := parser.parseSurrogatePair(first, escapePosition)
	if err != nil {
		return nil, err
	}

	return utf8.AppendRune(nil, value), nil
}

// parseSurrogatePair parses the low half required after a high surrogate.
func (parser *strictJSONParser) parseSurrogatePair(first uint16, escapePosition int) (rune, error) {
	missingLow := parser.position+2 > len(parser.source) || parser.source[parser.position] != '\\'
	if missingLow || parser.source[parser.position+1] != 'u' {
		return 0, fmt.Errorf("unpaired high surrogate at byte %d", escapePosition)
	}

	parser.position += 2

	second, err := parser.parseHexQuad()
	if err != nil {
		return 0, err
	}

	if second < 0xdc00 || second > 0xdfff {
		return 0, fmt.Errorf("invalid surrogate pair at byte %d", escapePosition)
	}

	return utf16.DecodeRune(rune(first), rune(second)), nil
}

// parseHexQuad parses exactly four hexadecimal digits.
func (parser *strictJSONParser) parseHexQuad() (uint16, error) {
	if parser.position+4 > len(parser.source) {
		return 0, fmt.Errorf("short Unicode escape at byte %d", parser.position)
	}

	var value uint16

	for range 4 {
		digit, ok := hexadecimalValue(parser.source[parser.position])
		if !ok {
			return 0, fmt.Errorf("invalid Unicode escape at byte %d", parser.position)
		}

		value = value*hexadecimalRadix + uint16(digit)
		parser.position++
	}

	return value, nil
}

// hexadecimalValue decodes one ASCII hexadecimal digit.
func hexadecimalValue(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + decimalRadix, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + decimalRadix, true
	default:
		return 0, false
	}
}

// skipWhitespace advances past JSON's four whitespace bytes.
func (parser *strictJSONParser) skipWhitespace() {
	for parser.position < len(parser.source) {
		switch parser.source[parser.position] {
		case ' ', '\t', '\n', '\r':
			parser.position++
		default:
			return
		}
	}
}

// consume advances past expected when it is next.
func (parser *strictJSONParser) consume(expected byte) bool {
	if parser.position == len(parser.source) || parser.source[parser.position] != expected {
		return false
	}

	parser.position++

	return true
}
