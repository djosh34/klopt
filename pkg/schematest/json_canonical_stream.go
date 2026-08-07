//nolint:godoclint // Private streaming helpers are used only by deterministic seeding.
package schematest

import (
	"fmt"
	"io"
	"sort"
	"unicode/utf8"
)

// writeCanonicalJSON writes one complete JSON value without retaining its canonical bytes.
func writeCanonicalJSON(writer io.Writer, value *jsonValue) error {
	if writer == nil {
		return fmt.Errorf("canonical JSON writer is nil")
	}

	return writeCanonicalJSONValue(writer, value, make(map[*jsonValue]bool))
}

func writeCanonicalJSONValue(writer io.Writer, value *jsonValue, visiting map[*jsonValue]bool) error {
	if err := enterCanonicalJSONValue(value, visiting); err != nil {
		return err
	}
	defer delete(visiting, value)

	return writeCanonicalJSONKind(writer, value, visiting)
}

func enterCanonicalJSONValue(value *jsonValue, visiting map[*jsonValue]bool) error {
	if value == nil {
		return fmt.Errorf("JSON value is nil")
	}

	if visiting[value] {
		return fmt.Errorf("JSON value contains a cycle")
	}

	if err := validateJSONPayload(value); err != nil {
		return err
	}

	visiting[value] = true

	return nil
}

func writeCanonicalJSONKind(writer io.Writer, value *jsonValue, visiting map[*jsonValue]bool) error {
	switch value.kind {
	case jsonNull:
		return writeCanonicalString(writer, "null")
	case jsonBoolean:
		return writeCanonicalBoolean(writer, value.boolean)
	case jsonNumber:
		return writeCanonicalNumber(writer, value.number)
	case jsonString:
		return writeCanonicalJSONString(writer, value.text)
	case jsonArray:
		return writeCanonicalJSONArray(writer, value.array, visiting)
	case jsonObject:
		return writeCanonicalJSONObject(writer, value.object, visiting)
	default:
		return fmt.Errorf("unknown JSON kind %d", value.kind)
	}
}

func writeCanonicalBoolean(writer io.Writer, value bool) error {
	if value {
		return writeCanonicalString(writer, "true")
	}

	return writeCanonicalString(writer, "false")
}

func writeCanonicalNumber(writer io.Writer, value *exactNumber) error {
	decimal, err := value.canonicalDecimal()
	if err != nil {
		return err
	}

	return writeCanonicalString(writer, decimal)
}

func writeCanonicalJSONArray(writer io.Writer, values []*jsonValue, visiting map[*jsonValue]bool) error {
	if err := writeCanonicalString(writer, "["); err != nil {
		return err
	}

	for index, value := range values {
		if index > 0 {
			if err := writeCanonicalString(writer, ","); err != nil {
				return err
			}
		}

		if err := writeCanonicalJSONValue(writer, value, visiting); err != nil {
			return err
		}
	}

	return writeCanonicalString(writer, "]")
}

func writeCanonicalJSONObject(writer io.Writer, values map[string]*jsonValue, visiting map[*jsonValue]bool) error {
	names, err := canonicalJSONObjectNames(values)
	if err != nil {
		return err
	}

	sort.Strings(names)

	if err := writeCanonicalString(writer, "{"); err != nil {
		return err
	}

	for index, name := range names {
		if index > 0 {
			if err := writeCanonicalString(writer, ","); err != nil {
				return err
			}
		}

		if err := writeCanonicalJSONString(writer, name); err != nil {
			return err
		}

		if err := writeCanonicalString(writer, ":"); err != nil {
			return err
		}

		if err := writeCanonicalJSONValue(writer, values[name], visiting); err != nil {
			return err
		}
	}

	return writeCanonicalString(writer, "}")
}

func canonicalJSONObjectNames(values map[string]*jsonValue) ([]string, error) {
	names := make([]string, 0, len(values))
	for name, value := range values {
		if !utf8.ValidString(name) {
			return nil, fmt.Errorf("object member name is not valid UTF-8")
		}

		if value == nil {
			return nil, fmt.Errorf("object member %q is nil", name)
		}

		names = append(names, name)
	}

	return names, nil
}

func writeCanonicalJSONString(writer io.Writer, value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("JSON string is not valid UTF-8")
	}

	if err := writeCanonicalString(writer, "\""); err != nil {
		return err
	}

	for _, character := range value {
		if err := writeCanonicalJSONCharacter(writer, character); err != nil {
			return err
		}
	}

	return writeCanonicalString(writer, "\"")
}

func writeCanonicalJSONCharacter(writer io.Writer, character rune) error {
	switch character {
	case '"':
		return writeCanonicalString(writer, "\\\"")
	case '\\':
		return writeCanonicalString(writer, "\\\\")
	case '\b':
		return writeCanonicalString(writer, "\\b")
	case '\f':
		return writeCanonicalString(writer, "\\f")
	case '\n':
		return writeCanonicalString(writer, "\\n")
	case '\r':
		return writeCanonicalString(writer, "\\r")
	case '\t':
		return writeCanonicalString(writer, "\\t")
	default:
		if character >= jsonControlLimit {
			var encoded [utf8.UTFMax]byte

			length := utf8.EncodeRune(encoded[:], character)

			return writeCanonicalBytes(writer, encoded[:length])
		}

		escaped := [6]byte{'\\', 'u', '0', '0', jsonHexDigits[byte(character)>>4], jsonHexDigits[byte(character)&0x0f]}

		return writeCanonicalBytes(writer, escaped[:])
	}
}

func writeCanonicalString(writer io.Writer, value string) error {
	return writeCanonicalBytes(writer, []byte(value))
}

func writeCanonicalBytes(writer io.Writer, value []byte) error {
	for len(value) > 0 {
		count, err := writer.Write(value)
		if err != nil {
			return err
		}

		if count <= 0 {
			return io.ErrShortWrite
		}

		value = value[count:]
	}

	return nil
}
