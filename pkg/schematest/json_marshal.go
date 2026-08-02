package schematest

import (
	"fmt"
	"sort"
	"unicode/utf8"
)

const (
	// jsonHexDigits is the fixed lowercase alphabet for control escapes.
	jsonHexDigits = "0123456789abcdef"
	// jsonControlLimit is the first Unicode scalar that need not be escaped.
	jsonControlLimit = 0x20
)

// marshalStrict serializes one complete JSON value with deterministic bytes.
func marshalStrict(value *jsonValue) ([]byte, error) {
	if err := validateJSONValue(value, make(map[*jsonValue]bool)); err != nil {
		return nil, err
	}

	encoded, err := appendStrictJSON(nil, value)
	if err != nil {
		return nil, err
	}

	return encoded, nil
}

// appendStrictJSON appends one already-validated complete JSON value.
func appendStrictJSON(encoded []byte, value *jsonValue) ([]byte, error) {
	switch value.kind {
	case jsonNull:
		return append(encoded, "null"...), nil
	case jsonBoolean:
		return appendJSONBoolean(encoded, value.boolean), nil
	case jsonNumber:
		decimal, err := value.number.canonicalDecimal()
		if err != nil {
			return nil, err
		}

		return append(encoded, decimal...), nil
	case jsonString:
		return appendJSONString(encoded, value.text), nil
	case jsonArray:
		return appendJSONArray(encoded, value.array)
	case jsonObject:
		return appendJSONObject(encoded, value.object)
	default:
		return nil, fmt.Errorf("unknown JSON kind %d", value.kind)
	}
}

// appendJSONBoolean appends one JSON boolean.
func appendJSONBoolean(encoded []byte, value bool) []byte {
	if value {
		return append(encoded, "true"...)
	}

	return append(encoded, "false"...)
}

// appendJSONArray appends one already-validated JSON array.
func appendJSONArray(encoded []byte, elements []*jsonValue) ([]byte, error) {
	encoded = append(encoded, '[')

	for index, element := range elements {
		if index > 0 {
			encoded = append(encoded, ',')
		}

		var err error

		encoded, err = appendStrictJSON(encoded, element)
		if err != nil {
			return nil, err
		}
	}

	return append(encoded, ']'), nil
}

// appendJSONObject appends one already-validated JSON object in UTF-8 key order.
func appendJSONObject(encoded []byte, members map[string]*jsonValue) ([]byte, error) {
	names := make([]string, 0, len(members))
	for name := range members {
		names = append(names, name)
	}

	sort.Strings(names)

	encoded = append(encoded, '{')

	for index, name := range names {
		if index > 0 {
			encoded = append(encoded, ',')
		}

		encoded = appendJSONString(encoded, name)
		encoded = append(encoded, ':')

		var err error

		encoded, err = appendStrictJSON(encoded, members[name])
		if err != nil {
			return nil, err
		}
	}

	return append(encoded, '}'), nil
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
