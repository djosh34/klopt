package schematest

import (
	"fmt"
	"unicode/utf8"
)

const (
	// jsonHexDigits is the fixed lowercase alphabet for control escapes.
	jsonHexDigits = "0123456789abcdef"
	// jsonControlLimit is the first Unicode scalar that need not be escaped.
	jsonControlLimit = 0x20
)

// jsonMarshalFrame holds the progress through one value being serialized.
type jsonMarshalFrame struct {
	value   *jsonValue
	index   int
	names   []string
	entered bool
}

// marshalStrict serializes one complete JSON value with deterministic bytes.
//
//nolint:cyclop // The explicit traversal handles each of JSON's six kinds in one state machine.
func marshalStrict(value *jsonValue) ([]byte, error) {
	if err := validateJSONValue(value); err != nil {
		return nil, err
	}

	encoded := make([]byte, 0)

	stack := []jsonMarshalFrame{{value: value}}
	for len(stack) > 0 {
		frame := &stack[len(stack)-1]

		switch frame.value.kind {
		case jsonNull:
			encoded = append(encoded, "null"...)
			stack = stack[:len(stack)-1]
		case jsonBoolean:
			encoded = appendJSONBoolean(encoded, frame.value.boolean)
			stack = stack[:len(stack)-1]
		case jsonNumber:
			decimal, err := frame.value.number.canonicalDecimal()
			if err != nil {
				return nil, err
			}

			encoded = append(encoded, decimal...)
			stack = stack[:len(stack)-1]
		case jsonString:
			encoded = appendJSONString(encoded, frame.value.text)
			stack = stack[:len(stack)-1]
		case jsonArray:
			if !frame.entered {
				encoded = append(encoded, '[')
				frame.entered = true
			}

			if frame.index == len(frame.value.array) {
				encoded = append(encoded, ']')
				stack = stack[:len(stack)-1]

				continue
			}

			if frame.index > 0 {
				encoded = append(encoded, ',')
			}

			child := frame.value.array[frame.index]
			frame.index++

			stack = append(stack, jsonMarshalFrame{value: child})
		case jsonObject:
			if !frame.entered {
				encoded = append(encoded, '{')
				frame.names = sortedObjectNames(frame.value.object)
				frame.entered = true
			}

			if frame.index == len(frame.names) {
				encoded = append(encoded, '}')
				stack = stack[:len(stack)-1]

				continue
			}

			if frame.index > 0 {
				encoded = append(encoded, ',')
			}

			name := frame.names[frame.index]
			encoded = appendJSONString(encoded, name)
			encoded = append(encoded, ':')
			frame.index++
			stack = append(stack, jsonMarshalFrame{value: frame.value.object[name]})
		default:
			return nil, fmt.Errorf("unknown JSON kind %d", frame.value.kind)
		}
	}

	return encoded, nil
}

// appendJSONBoolean appends one JSON boolean.
func appendJSONBoolean(encoded []byte, value bool) []byte {
	if value {
		return append(encoded, "true"...)
	}

	return append(encoded, "false"...)
}

// appendJSONString appends one valid UTF-8 string with fixed JSON escaping.
func appendJSONString(encoded []byte, value string) []byte {
	encoded = append(encoded, '"')
	for _, character := range value {
		encoded = appendJSONCharacter(encoded, character)
	}

	return append(encoded, '"')
}

// appendJSONCharacter appends one JSON string character.
func appendJSONCharacter(encoded []byte, character rune) []byte {
	switch character {
	case '"':
		return append(encoded, '\\', '"')
	case '\\':
		return append(encoded, '\\', '\\')
	case '\b':
		return append(encoded, '\\', 'b')
	case '\f':
		return append(encoded, '\\', 'f')
	case '\n':
		return append(encoded, '\\', 'n')
	case '\r':
		return append(encoded, '\\', 'r')
	case '\t':
		return append(encoded, '\\', 't')
	default:
		if character >= jsonControlLimit {
			return utf8.AppendRune(encoded, character)
		}

		encoded = append(encoded, '\\', 'u', '0', '0')

		return append(encoded, jsonHexDigits[byte(character)>>4], jsonHexDigits[byte(character)&0x0f])
	}
}
