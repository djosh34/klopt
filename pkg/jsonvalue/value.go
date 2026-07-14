// Package jsonvalue represents exact semantic JSON values.
package jsonvalue

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	// decimalRadix is the JSON number radix.
	decimalRadix = 10
	// decimalFactorTwo is one prime factor of the decimal radix.
	decimalFactorTwo = 2
	// decimalFactorFive is the other prime factor of the decimal radix.
	decimalFactorFive = 5
	// hexadecimalOffset is the first hexadecimal letter's value.
	hexadecimalOffset = 10
	// unicodeEscapeWidth is the digit count in a Unicode escape.
	unicodeEscapeWidth = 4
	// unicodeEscapeBytes counts the slash, u, and digits in a Unicode escape.
	unicodeEscapeBytes = 6
	// maximumMaterializedExponent avoids allocating enormous big integers for valid JSON exponents.
	maximumMaterializedExponent = 100000
)

// Kind identifies one JSON value kind.
type Kind uint8

const (
	// KindNull identifies JSON null.
	KindNull Kind = iota
	// KindBoolean identifies a JSON boolean.
	KindBoolean
	// KindNumber identifies a JSON number.
	KindNumber
	// KindString identifies a JSON string.
	KindString
	// KindArray identifies a JSON array.
	KindArray
	// KindObject identifies a JSON object.
	KindObject
)

// Number stores a canonical exact JSON number.
// Rational is nil only when materializing an exceptionally large exponent would be unsafe.
type Number struct {
	Lexeme   string
	Rational *big.Rat
}

// Compare returns -1, 0, or 1 according to the exact mathematical ordering.
func (number Number) Compare(other Number) int {
	if number.Rational != nil && other.Rational != nil {
		return number.Rational.Cmp(other.Rational)
	}

	negative, digits, exponent := decimalParts(number.Lexeme)
	otherNegative, otherDigits, otherExponent := decimalParts(other.Lexeme)

	if digits == "0" || otherDigits == "0" {
		return compareZero(negative, digits, otherNegative, otherDigits)
	}

	if negative != otherNegative {
		if negative {
			return -1
		}

		return 1
	}

	comparison := compareMagnitudes(digits, exponent, otherDigits, otherExponent)
	if negative {
		return -comparison
	}

	return comparison
}

// IsInteger reports whether the exact mathematical value is an integer.
func (number Number) IsInteger() bool {
	if number.Rational != nil {
		return number.Rational.IsInt()
	}

	_, digits, exponent := decimalParts(number.Lexeme)

	return digits == "0" || exponent.Sign() >= 0
}

// IsMultipleOf reports whether the exact quotient by divisor is an integer.
func (number Number) IsMultipleOf(divisor Number) bool {
	_, digits, exponent := decimalParts(number.Lexeme)

	_, divisorDigits, divisorExponent := decimalParts(divisor.Lexeme)
	if divisorDigits == "0" {
		return false
	}

	if digits == "0" {
		return true
	}

	coefficient, ok := new(big.Int).SetString(digits, decimalRadix)
	if !ok {
		return false
	}

	divisorCoefficient, ok := new(big.Int).SetString(divisorDigits, decimalRadix)
	if !ok {
		return false
	}

	common := new(big.Int).GCD(nil, nil, coefficient, divisorCoefficient)
	coefficient.Quo(coefficient, common)
	divisorCoefficient.Quo(divisorCoefficient, common)

	delta := new(big.Int).Sub(exponent, divisorExponent)

	return factorBalance(coefficient, divisorCoefficient, delta, decimalFactorTwo) >= 0 &&
		factorBalance(coefficient, divisorCoefficient, delta, decimalFactorFive) >= 0 &&
		containsOnlyDecimalFactors(divisorCoefficient)
}

// Member is one JSON object member.
type Member struct {
	Name  string
	Value Value
}

// Value is one exact semantic JSON value.
type Value struct {
	Kind    Kind
	Boolean bool
	Number  Number
	String  string
	Array   []Value
	Object  []Member
}

// Hash is a semantic JSON value hash.
type Hash [sha256.Size]byte

// Null returns JSON null.
func Null() Value {
	return Value{Kind: KindNull}
}

// Bool returns a JSON boolean.
func Bool(value bool) Value {
	return Value{Kind: KindBoolean, Boolean: value}
}

// String returns a JSON string.
func String(value string) Value {
	return Value{Kind: KindString, String: value}
}

// Array returns a JSON array and copies values.
func Array(values []Value) Value {
	copied := make([]Value, len(values))
	for index, value := range values {
		copied[index] = cloneValue(value)
	}

	return Value{Kind: KindArray, Array: copied}
}

// Object returns a JSON object after rejecting duplicate member names.
func Object(members []Member) (Value, error) {
	if err := validateMembers(members); err != nil {
		return Value{}, err
	}

	copied := make([]Member, len(members))
	for index, member := range members {
		copied[index] = Member{Name: member.Name, Value: cloneValue(member.Value)}
	}

	return Value{Kind: KindObject, Object: copied}, nil
}

// cloneValue returns a deep copy of value.
func cloneValue(value Value) Value {
	if value.Number.Rational != nil {
		value.Number.Rational = new(big.Rat).Set(value.Number.Rational)
	}

	if value.Array != nil {
		array := make([]Value, len(value.Array))
		for index, child := range value.Array {
			array[index] = cloneValue(child)
		}

		value.Array = array
	}

	if value.Object != nil {
		object := make([]Member, len(value.Object))
		for index, member := range value.Object {
			object[index] = Member{Name: member.Name, Value: cloneValue(member.Value)}
		}

		value.Object = object
	}

	return value
}

// ParseNumber parses and canonicalizes one exact JSON number lexeme.
func ParseNumber(lexeme string) (Number, error) {
	decoder := json.NewDecoder(strings.NewReader(lexeme))
	decoder.UseNumber()

	token, err := decoder.Token()
	if err != nil {
		return Number{}, fmt.Errorf("parse JSON number %q: %w", lexeme, err)
	}

	number, ok := token.(json.Number)
	if !ok {
		return Number{}, fmt.Errorf("JSON value %q is not a number", lexeme)
	}

	if eofErr := requireDecoderEOF(decoder); eofErr != nil {
		return Number{}, fmt.Errorf("parse JSON number %q: %w", lexeme, eofErr)
	}

	canonical, exponent, err := normalizeJSONNumber(number.String())
	if err != nil {
		return Number{}, err
	}

	var rational *big.Rat

	magnitude := new(big.Int).Abs(exponent)
	if magnitude.Cmp(big.NewInt(maximumMaterializedExponent)) <= 0 {
		parsed, parsedOK := new(big.Rat).SetString(canonical)
		if !parsedOK {
			return Number{}, fmt.Errorf("parse exact JSON number %q", lexeme)
		}

		rational = parsed
	}

	return Number{Lexeme: canonical, Rational: rational}, nil
}

// Parse decodes exactly one JSON value without losing number precision.
func Parse(raw []byte) (Value, error) {
	if raw == nil {
		return Value{}, errors.New("JSON value cannot be nil")
	}

	if !utf8.Valid(raw) {
		return Value{}, errors.New("JSON value must contain valid UTF-8")
	}

	if err := validateJSONStringEscapes(raw); err != nil {
		return Value{}, err
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	value, err := decodeValue(decoder)
	if err != nil {
		return Value{}, fmt.Errorf("parse JSON value: %w", err)
	}

	if err := requireDecoderEOF(decoder); err != nil {
		return Value{}, err
	}

	return value, nil
}

// Equal reports semantic JSON equality.
func (value Value) Equal(other Value) bool {
	if value.Kind != other.Kind {
		return false
	}

	switch value.Kind {
	case KindNull:
		return true
	case KindBoolean:
		return value.Boolean == other.Boolean
	case KindNumber:
		return value.Number.Lexeme == other.Number.Lexeme
	case KindString:
		return value.String == other.String
	case KindArray:
		return arraysEqual(value.Array, other.Array)
	case KindObject:
		return objectsEqual(value.Object, other.Object)
	default:
		return false
	}
}

// Hash returns a hash of the semantic JSON value.
func (value Value) Hash() (Hash, error) {
	encoded, err := value.MarshalJSON()
	if err != nil {
		return Hash{}, err
	}

	return sha256.Sum256(encoded), nil
}

// MarshalJSON returns deterministic canonical JSON.
func (value Value) MarshalJSON() ([]byte, error) {
	var encoded bytes.Buffer
	if err := encodeValue(&encoded, value); err != nil {
		return nil, err
	}

	return encoded.Bytes(), nil
}

// decodeValue decodes one value from decoder.
func decodeValue(decoder *json.Decoder) (Value, error) {
	token, err := decoder.Token()
	if err != nil {
		return Value{}, err
	}

	if delimiter, ok := token.(json.Delim); ok {
		return decodeDelimitedValue(decoder, delimiter)
	}

	return decodeScalarValue(token)
}

// decodeDelimitedValue dispatches one array or object value.
func decodeDelimitedValue(decoder *json.Decoder, delimiter json.Delim) (Value, error) {
	switch delimiter {
	case '[':
		return decodeArray(decoder)
	case '{':
		return decodeObject(decoder)
	default:
		return Value{}, fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
}

// decodeScalarValue decodes one non-container token.
func decodeScalarValue(token any) (Value, error) {
	switch scalar := token.(type) {
	case nil:
		return Null(), nil
	case bool:
		return Bool(scalar), nil
	case string:
		return String(scalar), nil
	case json.Number:
		number, err := ParseNumber(scalar.String())
		if err != nil {
			return Value{}, err
		}

		return Value{Kind: KindNumber, Number: number}, nil
	default:
		return Value{}, fmt.Errorf("unsupported decoded JSON value %T", token)
	}
}

// decodeArray decodes an array after its opening delimiter.
func decodeArray(decoder *json.Decoder) (Value, error) {
	values := make([]Value, 0)

	for decoder.More() {
		value, err := decodeValue(decoder)
		if err != nil {
			return Value{}, err
		}

		values = append(values, value)
	}

	if _, err := decoder.Token(); err != nil {
		return Value{}, err
	}

	return Array(values), nil
}

// decodeObject decodes an object after its opening delimiter.
func decodeObject(decoder *json.Decoder) (Value, error) {
	members := make([]Member, 0)
	seen := make(map[string]struct{})

	for decoder.More() {
		nameToken, err := decoder.Token()
		if err != nil {
			return Value{}, err
		}

		name, ok := nameToken.(string)
		if !ok {
			return Value{}, errors.New("JSON object name must be a string")
		}

		if _, duplicate := seen[name]; duplicate {
			return Value{}, fmt.Errorf("duplicate JSON object name %q", name)
		}

		child, err := decodeValue(decoder)
		if err != nil {
			return Value{}, err
		}

		seen[name] = struct{}{}
		members = append(members, Member{Name: name, Value: child})
	}

	if _, err := decoder.Token(); err != nil {
		return Value{}, err
	}

	return Object(members)
}

// requireDecoderEOF rejects trailing JSON values or invalid trailing bytes.
func requireDecoderEOF(decoder *json.Decoder) error {
	_, err := decoder.Token()
	if errors.Is(err, io.EOF) {
		return nil
	}

	if err == nil {
		return errors.New("JSON input must contain exactly one value")
	}

	return fmt.Errorf("parse trailing JSON input: %w", err)
}

// arraysEqual compares ordered array values.
func arraysEqual(left []Value, right []Value) bool {
	if len(left) != len(right) {
		return false
	}

	for index := range left {
		if !left[index].Equal(right[index]) {
			return false
		}
	}

	return true
}

// objectsEqual compares object members without considering their order.
func objectsEqual(left []Member, right []Member) bool {
	if len(left) != len(right) {
		return false
	}

	leftByName := membersByName(left)
	if leftByName == nil {
		return false
	}

	rightByName := membersByName(right)
	if rightByName == nil {
		return false
	}

	for name, leftValue := range leftByName {
		rightValue, ok := rightByName[name]
		if !ok || !leftValue.Equal(rightValue) {
			return false
		}
	}

	return true
}

// membersByName indexes members and returns nil for duplicate names.
func membersByName(members []Member) map[string]Value {
	byName := make(map[string]Value, len(members))
	for _, member := range members {
		if _, duplicate := byName[member.Name]; duplicate {
			return nil
		}

		byName[member.Name] = member.Value
	}

	return byName
}

// encodeValue writes one deterministic JSON value.
func encodeValue(encoded *bytes.Buffer, value Value) error {
	switch value.Kind {
	case KindNull:
		return writeString(encoded, "null")
	case KindBoolean:
		return writeString(encoded, fmt.Sprintf("%t", value.Boolean))
	case KindNumber:
		canonical, err := ParseNumber(value.Number.Lexeme)
		if err != nil {
			return err
		}

		return writeString(encoded, canonical.Lexeme)
	case KindString:
		return encodeString(encoded, value.String)
	case KindArray:
		return encodeArray(encoded, value.Array)
	case KindObject:
		return encodeObject(encoded, value.Object)
	default:
		return fmt.Errorf("invalid JSON value kind %d", value.Kind)
	}
}

// encodeString writes one valid UTF-8 JSON string.
func encodeString(encoded *bytes.Buffer, value string) error {
	if !utf8.ValidString(value) {
		return errors.New("JSON string must contain valid UTF-8")
	}

	stringJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode JSON string: %w", err)
	}

	return writeBytes(encoded, stringJSON)
}

// encodeArray writes an ordered JSON array.
func encodeArray(encoded *bytes.Buffer, values []Value) error {
	if err := encoded.WriteByte('['); err != nil {
		return err
	}

	for index, value := range values {
		if index > 0 {
			if err := encoded.WriteByte(','); err != nil {
				return err
			}
		}

		if err := encodeValue(encoded, value); err != nil {
			return fmt.Errorf("encode array index %d: %w", index, err)
		}
	}

	return encoded.WriteByte(']')
}

// encodeObject writes object members in lexical name order.
func encodeObject(encoded *bytes.Buffer, members []Member) error {
	if err := validateMembers(members); err != nil {
		return err
	}

	sorted := append([]Member(nil), members...)
	sort.Slice(sorted, func(left int, right int) bool {
		return sorted[left].Name < sorted[right].Name
	})

	if err := encoded.WriteByte('{'); err != nil {
		return err
	}

	for index, member := range sorted {
		if index > 0 {
			if err := encoded.WriteByte(','); err != nil {
				return err
			}
		}

		nameJSON, err := json.Marshal(member.Name)
		if err != nil {
			return fmt.Errorf("encode object name %q: %w", member.Name, err)
		}

		if err := writeBytes(encoded, nameJSON); err != nil {
			return err
		}

		if err := encoded.WriteByte(':'); err != nil {
			return err
		}

		if err := encodeValue(encoded, member.Value); err != nil {
			return fmt.Errorf("encode object member %q: %w", member.Name, err)
		}
	}

	return encoded.WriteByte('}')
}

// writeString handles the writer error even though bytes.Buffer currently cannot fail.
func writeString(encoded *bytes.Buffer, value string) error {
	_, err := encoded.WriteString(value)

	return err
}

// writeBytes handles the writer error even though bytes.Buffer currently cannot fail.
func writeBytes(encoded *bytes.Buffer, value []byte) error {
	_, err := encoded.Write(value)

	return err
}

// validateMembers rejects duplicate object names.
func validateMembers(members []Member) error {
	seen := make(map[string]struct{}, len(members))
	for _, member := range members {
		if !utf8.ValidString(member.Name) {
			return errors.New("JSON object name must contain valid UTF-8")
		}

		if _, duplicate := seen[member.Name]; duplicate {
			return fmt.Errorf("duplicate JSON object name %q", member.Name)
		}

		seen[member.Name] = struct{}{}
	}

	return nil
}

// normalizeJSONNumber returns a short exact representation and its decimal exponent.
func normalizeJSONNumber(number string) (string, *big.Int, error) {
	lexeme := number

	negative := strings.HasPrefix(lexeme, "-")
	if negative {
		lexeme = lexeme[1:]
	}

	exponent := new(big.Int)
	if exponentIndex := strings.IndexAny(lexeme, "eE"); exponentIndex >= 0 {
		parsedExponent, ok := new(big.Int).SetString(lexeme[exponentIndex+1:], decimalRadix)
		if !ok {
			return "", nil, fmt.Errorf("invalid JSON number %q", number)
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
		return "0", new(big.Int), nil
	}

	trimmedDigits := strings.TrimRight(digits, "0")
	exponent.Add(exponent, big.NewInt(int64(len(digits)-len(trimmedDigits)-fractionLength)))

	return formatCanonicalNumber(negative, trimmedDigits, exponent), exponent, nil
}

// decimalParts splits a canonical number into sign, coefficient digits, and exponent.
func decimalParts(lexeme string) (bool, string, *big.Int) {
	negative := strings.HasPrefix(lexeme, "-")
	if negative {
		lexeme = lexeme[1:]
	}

	exponent := new(big.Int)
	if exponentIndex := strings.IndexByte(lexeme, 'e'); exponentIndex >= 0 {
		parsedExponent, ok := new(big.Int).SetString(lexeme[exponentIndex+1:], decimalRadix)
		if !ok {
			panic("jsonvalue.Number contains an invalid canonical exponent")
		}

		exponent.Set(parsedExponent)

		lexeme = lexeme[:exponentIndex]
	}

	if decimalIndex := strings.IndexByte(lexeme, '.'); decimalIndex >= 0 {
		fractionLength := len(lexeme) - decimalIndex - 1
		lexeme = lexeme[:decimalIndex] + lexeme[decimalIndex+1:]

		exponent.Sub(exponent, big.NewInt(int64(fractionLength)))
	}

	digits := strings.TrimLeft(lexeme, "0")
	if digits == "" {
		return false, "0", new(big.Int)
	}

	return negative, digits, exponent
}

// compareZero compares a zero operand without losing the other operand's sign.
func compareZero(leftNegative bool, leftDigits string, rightNegative bool, rightDigits string) int {
	if leftDigits == "0" && rightDigits == "0" {
		return 0
	}

	if leftDigits == "0" {
		if rightNegative {
			return 1
		}

		return -1
	}

	if leftNegative {
		return -1
	}

	return 1
}

// compareMagnitudes compares positive coefficient/exponent pairs.
func compareMagnitudes(leftDigits string, leftExponent *big.Int, rightDigits string, rightExponent *big.Int) int {
	leftMagnitude := new(big.Int).Add(leftExponent, big.NewInt(int64(len(leftDigits))))

	rightMagnitude := new(big.Int).Add(rightExponent, big.NewInt(int64(len(rightDigits))))
	if comparison := leftMagnitude.Cmp(rightMagnitude); comparison != 0 {
		return comparison
	}

	for index := range max(len(leftDigits), len(rightDigits)) {
		leftDigit := byte('0')
		if index < len(leftDigits) {
			leftDigit = leftDigits[index]
		}

		rightDigit := byte('0')
		if index < len(rightDigits) {
			rightDigit = rightDigits[index]
		}

		if leftDigit < rightDigit {
			return -1
		}

		if leftDigit > rightDigit {
			return 1
		}
	}

	return 0
}

// factorBalance returns the signed exponent of factor in an exact quotient.
func factorBalance(numerator *big.Int, denominator *big.Int, decimalExponent *big.Int, factor int64) int {
	balance := new(big.Int).Set(decimalExponent)
	balance.Add(balance, big.NewInt(int64(factorCount(numerator, factor))))
	balance.Sub(balance, big.NewInt(int64(factorCount(denominator, factor))))

	return balance.Sign()
}

// factorCount counts one prime factor without changing value.
func factorCount(value *big.Int, factor int64) int {
	remaining := new(big.Int).Set(value)
	divisor := big.NewInt(factor)
	remainder := new(big.Int)
	count := 0

	for {
		quotient, modulus := new(big.Int).QuoRem(remaining, divisor, remainder)
		if modulus.Sign() != 0 {
			return count
		}

		count++
		remaining = quotient
	}
}

// containsOnlyDecimalFactors reports whether value's only prime factors are 2 and 5.
func containsOnlyDecimalFactors(value *big.Int) bool {
	remaining := new(big.Int).Set(value)

	for _, factor := range []int64{2, 5} {
		divisor := big.NewInt(factor)
		remainder := new(big.Int)

		for {
			quotient, modulus := new(big.Int).QuoRem(remaining, divisor, remainder)
			if modulus.Sign() != 0 {
				break
			}

			remaining = quotient
		}
	}

	return remaining.Cmp(big.NewInt(1)) == 0
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
			backslashes := 0
			for previous := index - 1; previous >= 0 && value[previous] == '\\'; previous-- {
				backslashes++
			}

			if backslashes%2 == 0 {
				inString = !inString
			}

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

// validateJSONStringEscape validates one Unicode escape and returns its final index.
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
		return 0, errors.New("JSON value contains an unpaired UTF-16 surrogate")
	}

	if code < 0xD800 || code > 0xDBFF {
		return lastIndex, nil
	}

	return validateJSONSurrogatePair(value, lastIndex)
}

// validateJSONSurrogatePair validates the low half following a high surrogate.
func validateJSONSurrogatePair(value []byte, highEnd int) (int, error) {
	lowSlash := highEnd + 1
	lowEnd := lowSlash + unicodeEscapeBytes

	if lowEnd > len(value) || value[lowSlash] != '\\' || value[lowSlash+1] != 'u' {
		return 0, errors.New("JSON value contains an unpaired UTF-16 surrogate")
	}

	low, ok := decodeHexQuad(value[lowSlash+2 : lowEnd])
	if !ok || low < 0xDC00 || low > 0xDFFF {
		return 0, errors.New("JSON value contains an unpaired UTF-16 surrogate")
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
