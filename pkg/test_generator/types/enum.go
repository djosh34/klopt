package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"unicode/utf8"
)

const (
	// decimalRadix is the JSON number radix.
	decimalRadix = 10
	// hexadecimalOffset is the value of the first hexadecimal letter.
	hexadecimalOffset = 10
	// unicodeEscapeWidth is the hexadecimal digit count in a Unicode escape.
	unicodeEscapeWidth = 4
	// unicodeEscapePrefixWidth counts the slash and u in a Unicode escape.
	unicodeEscapePrefixWidth = 2
)

// Enum is a canonical JSON value used by an enum constraint.
type Enum json.RawMessage

// enumHashJSON separates enum hashes from hashes of other domain values.
type enumHashJSON struct {
	Type  string `json:"type"`
	Value Enum   `json:"value"`
}

// canonicalNumber marshals an exact JSON number without changing it to float64.
type canonicalNumber string

var _ Hasher = Enum{}

// CanonicalEnum returns a deterministic representation of a JSON enum value.
func CanonicalEnum(value json.RawMessage) (Enum, error) {
	if value == nil {
		return nil, errors.New("enum raw value cannot be nil")
	}

	if !utf8.Valid(value) {
		return nil, errors.New("enum raw value must contain valid UTF-8")
	}

	if err := validateJSONStringEscapes(value); err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()

	decoded, err := decodeCanonicalJSONValue(decoder)
	if err != nil {
		return nil, fmt.Errorf("enum raw value must be valid JSON: %w", err)
	}

	if _, trailingErr := decoder.Token(); !errors.Is(trailingErr, io.EOF) {
		if trailingErr == nil {
			return nil, errors.New("enum raw value must contain one JSON value")
		}

		return nil, fmt.Errorf("enum raw value must be valid JSON: %w", trailingErr)
	}

	canonical, err := json.Marshal(decoded)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical enum: %w", err)
	}

	return Enum(canonical), nil
}

// GenerateHash returns a hash of the enum's semantic JSON value.
func (e Enum) GenerateHash() (Hash, error) {
	jsonBytes, err := json.Marshal(enumHashJSON{Type: "enum", Value: e})
	if err != nil {
		return Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}

// MarshalJSON returns the enum as canonical JSON.
func (e Enum) MarshalJSON() ([]byte, error) {
	if e == nil {
		return nil, errors.New("enum raw value cannot be nil")
	}

	canonical, err := CanonicalEnum(json.RawMessage(e))
	if err != nil {
		return nil, err
	}

	return canonical, nil
}

// MarshalJSON writes an already validated canonical number.
func (number canonicalNumber) MarshalJSON() ([]byte, error) {
	return []byte(number), nil
}

// decodeCanonicalJSONValue decodes one strict JSON value and normalizes its numbers.
func decodeCanonicalJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	delimiter, ok := token.(json.Delim)
	if !ok {
		switch value := token.(type) {
		case nil, bool, string:
			return value, nil
		case json.Number:
			number, err := normalizeJSONNumber(value)
			if err != nil {
				return nil, err
			}

			return canonicalNumber(number), nil
		default:
			return nil, fmt.Errorf("unsupported decoded enum value %T", token)
		}
	}

	switch delimiter {
	case '[':
		return decodeCanonicalJSONArray(decoder)
	case '{':
		return decodeCanonicalJSONObject(decoder)
	default:
		return nil, fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
}

// decodeCanonicalJSONArray decodes an array while retaining element order.
func decodeCanonicalJSONArray(decoder *json.Decoder) ([]any, error) {
	values := make([]any, 0)

	for decoder.More() {
		value, err := decodeCanonicalJSONValue(decoder)
		if err != nil {
			return nil, err
		}

		values = append(values, value)
	}

	if _, err := decoder.Token(); err != nil {
		return nil, err
	}

	return values, nil
}

// decodeCanonicalJSONObject decodes an object and rejects duplicate decoded names.
func decodeCanonicalJSONObject(decoder *json.Decoder) (map[string]any, error) {
	object := make(map[string]any)

	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}

		key, ok := keyToken.(string)
		if !ok {
			return nil, errors.New("JSON object name must be string")
		}

		if _, exists := object[key]; exists {
			return nil, fmt.Errorf("duplicate JSON object name %q", key)
		}

		value, err := decodeCanonicalJSONValue(decoder)
		if err != nil {
			return nil, err
		}

		object[key] = value
	}

	if _, err := decoder.Token(); err != nil {
		return nil, err
	}

	return object, nil
}

// normalizeJSONNumber returns a short exact representation of a JSON number.
func normalizeJSONNumber(number json.Number) (string, error) {
	lexeme := number.String()

	negative := strings.HasPrefix(lexeme, "-")
	if negative {
		lexeme = lexeme[1:]
	}

	exponent := new(big.Int)
	if exponentIndex := strings.IndexAny(lexeme, "eE"); exponentIndex >= 0 {
		parsedExponent, ok := new(big.Int).SetString(lexeme[exponentIndex+1:], decimalRadix)
		if !ok {
			return "", fmt.Errorf("invalid JSON number %q", number)
		}

		exponent.Set(parsedExponent)

		lexeme = lexeme[:exponentIndex]
	}

	fractionLength := 0
	if decimalIndex := strings.IndexByte(lexeme, '.'); decimalIndex >= 0 {
		fractionLength = len(lexeme) - decimalIndex - 1
		lexeme = lexeme[:decimalIndex] + lexeme[decimalIndex+1:]
	}

	digits := strings.TrimLeft(lexeme, "0")
	if digits == "" {
		return "0", nil
	}

	trimmedDigits := strings.TrimRight(digits, "0")
	exponent.Add(exponent, big.NewInt(int64(len(digits)-len(trimmedDigits)-fractionLength)))

	return formatCanonicalNumber(negative, trimmedDigits, exponent), nil
}

// formatCanonicalNumber chooses the shorter bounded plain or scientific form.
func formatCanonicalNumber(negative bool, digits string, exponent *big.Int) string {
	scientific := digits
	if exponent.Sign() != 0 {
		scientific += "e" + exponent.String()
	}

	formatted := scientific
	if plain, ok := formatPlainNumber(digits, exponent, len(scientific)); ok {
		formatted = plain
	}

	if negative {
		return "-" + formatted
	}

	return formatted
}

// formatPlainNumber returns a plain form only when it is no longer than scientific form.
func formatPlainNumber(digits string, exponent *big.Int, maximumLength int) (string, bool) {
	if exponent.Sign() == 0 {
		return digits, true
	}

	magnitude := new(big.Int).Abs(exponent)
	if magnitude.Cmp(big.NewInt(int64(maximumLength))) > 0 {
		return "", false
	}

	places := int(magnitude.Int64())
	if exponent.Sign() > 0 {
		if len(digits)+places > maximumLength {
			return "", false
		}

		return digits + strings.Repeat("0", places), true
	}

	if places < len(digits) {
		point := len(digits) - places

		return digits[:point] + "." + digits[point:], true
	}

	if 2+places > maximumLength {
		return "", false
	}

	return "0." + strings.Repeat("0", places-len(digits)) + digits, true
}

// validateJSONStringEscapes rejects unpaired UTF-16 surrogate escapes.
func validateJSONStringEscapes(value []byte) error {
	inString := false

	for index := 0; index < len(value); index++ {
		if value[index] == '"' {
			inString = !inString

			continue
		}

		if value[index] != '\\' || !inString {
			continue
		}

		nextIndex, err := validateJSONStringEscape(value, index)
		if err != nil {
			return err
		}

		index = nextIndex
	}

	return nil
}

// validateJSONStringEscape validates one escape and returns its last byte index.
func validateJSONStringEscape(value []byte, slashIndex int) (int, error) {
	escapeIndex := slashIndex + 1
	if escapeIndex >= len(value) || value[escapeIndex] != 'u' {
		return escapeIndex, nil
	}

	quadEnd := escapeIndex + unicodeEscapeWidth + 1
	if quadEnd > len(value) {
		return escapeIndex, nil
	}

	code, ok := decodeHexQuad(value[escapeIndex+1 : quadEnd])
	if !ok {
		return escapeIndex, nil
	}

	lastIndex := quadEnd - 1

	if code >= 0xDC00 && code <= 0xDFFF {
		return 0, errors.New("enum raw value contains unpaired UTF-16 surrogate")
	}

	if code < 0xD800 || code > 0xDBFF {
		return lastIndex, nil
	}

	return validateJSONSurrogatePair(value, lastIndex)
}

// validateJSONSurrogatePair validates the low half following a high surrogate.
func validateJSONSurrogatePair(value []byte, highEnd int) (int, error) {
	lowSlash := highEnd + 1

	lowEnd := lowSlash + unicodeEscapeWidth + unicodeEscapePrefixWidth
	if lowEnd > len(value) || value[lowSlash] != '\\' || value[lowSlash+1] != 'u' {
		return 0, errors.New("enum raw value contains unpaired UTF-16 surrogate")
	}

	low, ok := decodeHexQuad(value[lowSlash+2 : lowEnd])
	if !ok || low < 0xDC00 || low > 0xDFFF {
		return 0, errors.New("enum raw value contains unpaired UTF-16 surrogate")
	}

	return lowEnd - 1, nil
}

// decodeHexQuad decodes four hexadecimal bytes.
func decodeHexQuad(value []byte) (uint16, bool) {
	if len(value) != unicodeEscapeWidth {
		return 0, false
	}

	var decoded uint16

	for _, character := range value {
		decoded <<= 4

		switch {
		case character >= '0' && character <= '9':
			decoded += uint16(character - '0')
		case character >= 'a' && character <= 'f':
			decoded += uint16(character-'a') + hexadecimalOffset
		case character >= 'A' && character <= 'F':
			decoded += uint16(character-'A') + hexadecimalOffset
		default:
			return 0, false
		}
	}

	return decoded, true
}
