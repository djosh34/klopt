package schematest

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

const (
	// decimalRadix is JSON's number base.
	decimalRadix = 10
	// binaryFactor is the first prime factor of decimalRadix.
	binaryFactor = 2
	// quinaryFactor is the second prime factor of decimalRadix.
	quinaryFactor = 5
	// decimalPrefixLength is the length of "0.".
	decimalPrefixLength = 2
)

// exactNumber stores a rational multiplied by an arbitrary integral power of ten.
type exactNumber struct {
	numerator   *big.Int
	denominator *big.Int
	exponent    *big.Int
	scale       *big.Int
}

// exactNumberParts is one syntactically scanned JSON number.
type exactNumberParts struct {
	end              int
	digits           string
	fractionDigits   int
	authoredExponent *big.Int
	negative         bool
}

// parseExactNumber parses one complete JSON number without binary floating point.
func parseExactNumber(source string) (*exactNumber, error) {
	parts, err := scanExactNumber(source)
	if err != nil {
		return nil, err
	}

	if parts.end != len(source) {
		return nil, fmt.Errorf("number has trailing data at byte %d", parts.end)
	}

	fraction := new(big.Int).SetUint64(uint64(parts.fractionDigits))

	scale := new(big.Int).Sub(fraction, parts.authoredExponent)
	if scale.Sign() < 0 {
		scale.SetInt64(0)
	}

	normalizedDigits := strings.TrimRight(parts.digits, "0")

	trailingZeros := len(parts.digits) - len(normalizedDigits)
	if normalizedDigits == "" {
		normalizedDigits = "0"
	}

	coefficient := new(big.Int)
	if _, ok := coefficient.SetString(normalizedDigits, decimalRadix); !ok {
		return nil, errors.New("number has invalid decimal digits")
	}

	if parts.negative {
		coefficient.Neg(coefficient)
	}

	exponent := new(big.Int).Sub(parts.authoredExponent, fraction)
	exponent.Add(exponent, new(big.Int).SetUint64(uint64(trailingZeros)))

	return newExactNumber(coefficient, big.NewInt(1), exponent, scale)
}

// scanExactNumber checks JSON number grammar and returns its decimal parts.
func scanExactNumber(source string) (exactNumberParts, error) {
	if source == "" {
		return exactNumberParts{}, errors.New("expected JSON number")
	}

	position, negative, err := scanExactSign(source)
	if err != nil {
		return exactNumberParts{}, err
	}

	position, integerDigits, err := scanExactInteger(source, position)
	if err != nil {
		return exactNumberParts{}, err
	}

	position, fractionDigits, err := scanExactFraction(source, position)
	if err != nil {
		return exactNumberParts{}, err
	}

	position, authoredExponent, err := scanExactExponent(source, position)
	if err != nil {
		return exactNumberParts{}, err
	}

	return exactNumberParts{
		end:              position,
		digits:           integerDigits + fractionDigits,
		fractionDigits:   len(fractionDigits),
		authoredExponent: authoredExponent,
		negative:         negative,
	}, nil
}

// scanExactSign scans an optional leading minus.
func scanExactSign(source string) (int, bool, error) {
	if source[0] != '-' {
		return 0, false, nil
	}

	if len(source) == 1 {
		return 0, false, errors.New("number is missing its integer part")
	}

	return 1, true, nil
}

// scanExactInteger scans the required JSON integer part.
func scanExactInteger(source string, position int) (int, string, error) {
	start := position
	switch {
	case source[position] == '0':
		position++
	case source[position] >= '1' && source[position] <= '9':
		position++
		for position < len(source) && isDecimalDigit(source[position]) {
			position++
		}
	default:
		return 0, "", errors.New("number has an invalid integer part")
	}

	return position, source[start:position], nil
}

// scanExactFraction scans an optional JSON fraction.
func scanExactFraction(source string, position int) (int, string, error) {
	if position == len(source) || source[position] != '.' {
		return position, "", nil
	}

	position++

	start := position
	for position < len(source) && isDecimalDigit(source[position]) {
		position++
	}

	if position == start {
		return 0, "", errors.New("number is missing fraction digits")
	}

	return position, source[start:position], nil
}

// scanExactExponent scans an optional signed decimal exponent.
func scanExactExponent(source string, position int) (int, *big.Int, error) {
	if position == len(source) || (source[position] != 'e' && source[position] != 'E') {
		return position, new(big.Int), nil
	}

	position, negative := scanExponentSign(source, position+1)

	start := position
	for position < len(source) && isDecimalDigit(source[position]) {
		position++
	}

	if position == start {
		return 0, nil, errors.New("number is missing exponent digits")
	}

	exponent := new(big.Int)
	if _, ok := exponent.SetString(source[start:position], decimalRadix); !ok {
		return 0, nil, errors.New("number has an invalid exponent")
	}

	if negative {
		exponent.Neg(exponent)
	}

	return position, exponent, nil
}

// scanExponentSign scans the optional exponent sign.
func scanExponentSign(source string, position int) (int, bool) {
	if position == len(source) || (source[position] != '+' && source[position] != '-') {
		return position, false
	}

	return position + 1, source[position] == '-'
}

// isDecimalDigit reports whether value is an ASCII decimal digit.
func isDecimalDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

// newExactRational constructs an exact rational with no authored decimal scale.
func newExactRational(numerator, denominator *big.Int) (*exactNumber, error) {
	return newExactNumber(numerator, denominator, big.NewInt(0), big.NewInt(0))
}

// newExactNumber copies and normalizes an exact scaled rational.
func newExactNumber(numerator, denominator, exponent, scale *big.Int) (*exactNumber, error) {
	if numerator == nil || denominator == nil || exponent == nil || scale == nil {
		return nil, errors.New("exact number has nil state")
	}

	if denominator.Sign() == 0 {
		return nil, errors.New("exact number has a zero denominator")
	}

	if scale.Sign() < 0 {
		return nil, errors.New("exact number has a negative authored scale")
	}

	number := &exactNumber{
		numerator:   new(big.Int).Set(numerator),
		denominator: new(big.Int).Set(denominator),
		exponent:    new(big.Int).Set(exponent),
		scale:       new(big.Int).Set(scale),
	}
	number.normalize()

	return number, nil
}

// normalize reduces an exact number and removes coefficient decimal zeros.
func (number *exactNumber) normalize() {
	if number.denominator.Sign() < 0 {
		number.numerator.Neg(number.numerator)
		number.denominator.Neg(number.denominator)
	}

	if number.numerator.Sign() == 0 {
		number.denominator.SetInt64(1)
		number.exponent.SetInt64(0)

		return
	}

	gcd := new(big.Int).GCD(nil, nil, new(big.Int).Abs(number.numerator), number.denominator)
	number.numerator.Quo(number.numerator, gcd)
	number.denominator.Quo(number.denominator, gcd)

	zeros := decimalTrailingZeros(new(big.Int).Abs(number.numerator))
	if zeros == 0 {
		return
	}

	number.numerator.Quo(number.numerator, decimalPower(zeros))
	number.exponent.Add(number.exponent, new(big.Int).SetUint64(zeros))
}

// validate checks the exact-number representation invariant.
func (number *exactNumber) validate() error {
	if !number.hasCompleteState() {
		return errors.New("exact number has nil state")
	}

	if number.denominator.Sign() <= 0 {
		return errors.New("exact number denominator is not positive")
	}

	if number.scale.Sign() < 0 {
		return errors.New("exact number authored scale is negative")
	}

	if err := number.validateReduced(); err != nil {
		return err
	}

	return number.validateNormalizedCoefficient()
}

// hasCompleteState reports whether every exact-number field is initialized.
func (number *exactNumber) hasCompleteState() bool {
	if number == nil || number.numerator == nil || number.denominator == nil {
		return false
	}

	return number.exponent != nil && number.scale != nil
}

// validateReduced checks that the fraction has no common factor.
func (number *exactNumber) validateReduced() error {
	gcd := new(big.Int).GCD(nil, nil, new(big.Int).Abs(number.numerator), number.denominator)
	if gcd.Cmp(big.NewInt(1)) != 0 {
		return errors.New("exact number fraction is not reduced")
	}

	return nil
}

// validateNormalizedCoefficient checks zero and trailing-decimal-zero normalization.
func (number *exactNumber) validateNormalizedCoefficient() error {
	if number.numerator.Sign() == 0 {
		if number.denominator.Cmp(big.NewInt(1)) != 0 || number.exponent.Sign() != 0 {
			return errors.New("exact zero is not normalized")
		}

		return nil
	}

	if new(big.Int).Rem(number.numerator, big.NewInt(decimalRadix)).Sign() == 0 {
		return errors.New("exact number coefficient has a trailing decimal zero")
	}

	return nil
}

// compare returns the mathematical ordering of two exact numbers.
func (number *exactNumber) compare(other *exactNumber) (int, error) {
	if err := number.validate(); err != nil {
		return 0, err
	}

	if err := other.validate(); err != nil {
		return 0, err
	}

	leftSign := number.numerator.Sign()

	rightSign := other.numerator.Sign()
	if leftSign < rightSign {
		return -1, nil
	}

	if leftSign > rightSign {
		return 1, nil
	}

	if leftSign == 0 {
		return 0, nil
	}

	left := new(big.Int).Mul(new(big.Int).Abs(number.numerator), other.denominator)
	right := new(big.Int).Mul(new(big.Int).Abs(other.numerator), number.denominator)
	delta := new(big.Int).Sub(number.exponent, other.exponent)

	comparison, err := comparePositiveScaledIntegers(left, right, delta)
	if err != nil {
		return 0, err
	}

	if leftSign < 0 {
		comparison = -comparison
	}

	return comparison, nil
}

// comparePositiveScaledIntegers compares left*10^delta with right without expanding huge powers.
func comparePositiveScaledIntegers(left, right, delta *big.Int) (int, error) {
	leftMagnitude := new(big.Int).SetInt64(int64(len(left.String())))
	leftMagnitude.Add(leftMagnitude, delta)

	rightMagnitude := big.NewInt(int64(len(right.String())))
	if comparison := leftMagnitude.Cmp(rightMagnitude); comparison != 0 {
		return comparison, nil
	}

	if !delta.IsInt64() {
		return 0, errors.New("equal-magnitude decimal exponent does not fit int64")
	}

	difference := delta.Int64()
	if difference >= 0 {
		left.Mul(left, decimalPower(uint64(difference)))
	} else {
		right.Mul(right, decimalPower(uint64(-difference)))
	}

	return left.Cmp(right), nil
}

// isInteger reports whether the exact number is mathematically integral.
func (number *exactNumber) isInteger() (bool, error) {
	if err := number.validate(); err != nil {
		return false, err
	}

	if number.numerator.Sign() == 0 {
		return true, nil
	}

	if number.exponent.Sign() < 0 {
		if number.denominator.Cmp(big.NewInt(1)) != 0 {
			return false, nil
		}

		zeros := decimalTrailingZeros(new(big.Int).Abs(number.numerator))
		required := new(big.Int).Neg(number.exponent)

		return new(big.Int).SetUint64(zeros).Cmp(required) >= 0, nil
	}

	twos, afterTwos := removeFactor(number.denominator, binaryFactor)

	fives, remainder := removeFactor(afterTwos, quinaryFactor)
	if remainder.Cmp(big.NewInt(1)) != 0 {
		return false, nil
	}

	needed := twos
	if fives > needed {
		needed = fives
	}

	return number.exponent.Cmp(new(big.Int).SetUint64(needed)) >= 0, nil
}

// isMultipleOf reports whether number divided by divisor is an integer.
func (number *exactNumber) isMultipleOf(divisor *exactNumber) (bool, error) {
	if err := number.validate(); err != nil {
		return false, err
	}

	if err := divisor.validate(); err != nil {
		return false, err
	}

	if divisor.numerator.Sign() == 0 {
		return false, errors.New("multiple divisor is zero")
	}

	numerator := new(big.Int).Mul(number.numerator, divisor.denominator)
	denominator := new(big.Int).Mul(number.denominator, new(big.Int).Abs(divisor.numerator))
	exponent := new(big.Int).Sub(number.exponent, divisor.exponent)

	quotient, err := newExactNumber(numerator, denominator, exponent, big.NewInt(0))
	if err != nil {
		return false, err
	}

	return quotient.isInteger()
}

// exactQuantum returns 10 to the negative greatest authored scale.
func exactQuantum(numbers ...*exactNumber) (*exactNumber, error) {
	if len(numbers) == 0 {
		return nil, errors.New("exact quantum needs at least one number")
	}

	maximumScale := big.NewInt(0)

	for _, number := range numbers {
		if err := number.validate(); err != nil {
			return nil, err
		}

		if number.scale.Cmp(maximumScale) > 0 {
			maximumScale.Set(number.scale)
		}
	}

	return newExactNumber(big.NewInt(1), big.NewInt(1), new(big.Int).Neg(maximumScale), maximumScale)
}

// canonicalDecimal returns the shortest deterministic legal decimal lexeme.
func (number *exactNumber) canonicalDecimal() (string, error) {
	if err := number.validate(); err != nil {
		return "", err
	}

	if number.numerator.Sign() == 0 {
		return "0", nil
	}

	coefficient, exponent, err := number.finiteDecimal()
	if err != nil {
		return "", err
	}

	negative := coefficient.Sign() < 0
	digits := new(big.Int).Abs(coefficient).String()

	body, err := renderShortestDecimal(digits, exponent)
	if err != nil {
		return "", err
	}

	if negative {
		body = "-" + body
	}

	return body, nil
}

// finiteDecimal converts a terminating scaled rational to a decimal coefficient and exponent.
func (number *exactNumber) finiteDecimal() (*big.Int, *big.Int, error) {
	twos, afterTwos := removeFactor(number.denominator, binaryFactor)

	fives, remainder := removeFactor(afterTwos, quinaryFactor)
	if remainder.Cmp(big.NewInt(1)) != 0 {
		return nil, nil, errors.New("exact rational has no finite decimal representation")
	}

	decimalPlaces := twos
	if fives > decimalPlaces {
		decimalPlaces = fives
	}

	coefficient := new(big.Int).Set(number.numerator)
	if twos < decimalPlaces {
		coefficient.Mul(coefficient, integerPower(binaryFactor, decimalPlaces-twos))
	}

	if fives < decimalPlaces {
		coefficient.Mul(coefficient, integerPower(quinaryFactor, decimalPlaces-fives))
	}

	exponent := new(big.Int).Sub(number.exponent, new(big.Int).SetUint64(decimalPlaces))

	zeros := decimalTrailingZeros(new(big.Int).Abs(coefficient))
	if zeros > 0 {
		coefficient.Quo(coefficient, decimalPower(zeros))
		exponent.Add(exponent, new(big.Int).SetUint64(zeros))
	}

	return coefficient, exponent, nil
}

// renderShortestDecimal chooses the shorter of plain and scientific legal notation.
func renderShortestDecimal(digits string, exponent *big.Int) (string, error) {
	plainLength, position := decimalPlainLength(len(digits), exponent)
	scientificExponent := new(big.Int).Add(exponent, big.NewInt(int64(len(digits)-1)))

	scientificLength := decimalScientificLength(len(digits), scientificExponent)
	if plainLength.Cmp(scientificLength) <= 0 {
		return renderPlainDecimal(digits, position)
	}

	return renderScientificDecimal(digits, scientificExponent), nil
}

// decimalPlainLength returns the plain notation length and decimal-point position.
func decimalPlainLength(digitCount int, exponent *big.Int) (*big.Int, *big.Int) {
	position := new(big.Int).Add(big.NewInt(int64(digitCount)), exponent)
	switch {
	case position.Sign() <= 0:
		length := new(big.Int).Neg(position)
		length.Add(length, big.NewInt(int64(digitCount+decimalPrefixLength)))

		return length, position
	case position.Cmp(big.NewInt(int64(digitCount))) < 0:
		return big.NewInt(int64(digitCount + 1)), position
	default:
		return new(big.Int).Set(position), position
	}
}

// decimalScientificLength returns the scientific notation length.
func decimalScientificLength(digitCount int, exponent *big.Int) *big.Int {
	length := digitCount
	if digitCount > 1 {
		length++
	}

	if exponent.Sign() == 0 {
		return big.NewInt(int64(length))
	}

	length++
	if exponent.Sign() < 0 {
		length++
	}

	length += len(new(big.Int).Abs(exponent).String())

	return big.NewInt(int64(length))
}

// renderPlainDecimal renders a finite decimal without an exponent.
func renderPlainDecimal(digits string, position *big.Int) (string, error) {
	if !position.IsInt64() {
		return "", errors.New("plain decimal position does not fit int64")
	}

	point := position.Int64()
	switch {
	case point <= 0:
		return "0." + strings.Repeat("0", int(-point)) + digits, nil
	case point < int64(len(digits)):
		return digits[:point] + "." + digits[point:], nil
	default:
		return digits + strings.Repeat("0", int(point)-len(digits)), nil
	}
}

// renderScientificDecimal renders a finite decimal with a canonical exponent.
func renderScientificDecimal(digits string, exponent *big.Int) string {
	body := digits[:1]
	if len(digits) > 1 {
		body += "." + digits[1:]
	}

	if exponent.Sign() != 0 {
		body += "e" + exponent.String()
	}

	return body
}

// removeFactor finds the largest factor power and removes it with one division.
func removeFactor(value *big.Int, factor int64) (uint64, *big.Int) {
	count := factorMultiplicity(value, factor)

	remaining := new(big.Int).Set(value)
	if count > 0 {
		remaining.Quo(remaining, integerPower(factor, count))
	}

	return count, remaining
}

// factorMultiplicity locates the largest dividing factor power by exponential and binary search.
func factorMultiplicity(value *big.Int, factor int64) uint64 {
	if value.Sign() == 0 {
		return 0
	}

	limit := uint64(value.BitLen())
	lower := uint64(0)
	upper := uint64(1)

	for upper <= limit && divisibleByPower(value, factor, upper) {
		lower = upper
		if upper > limit/binaryFactor {
			upper = limit + 1

			break
		}

		upper *= 2
	}

	if upper > limit {
		upper = limit + 1
	}

	for lower+1 < upper {
		middle := lower + (upper-lower)/binaryFactor
		if divisibleByPower(value, factor, middle) {
			lower = middle
		} else {
			upper = middle
		}
	}

	return lower
}

// divisibleByPower reports whether value is divisible by factor^exponent.
func divisibleByPower(value *big.Int, factor int64, exponent uint64) bool {
	power := integerPower(factor, exponent)

	return new(big.Int).Rem(value, power).Sign() == 0
}

// decimalTrailingZeros counts paired factors of two and five in a positive integer.
func decimalTrailingZeros(value *big.Int) uint64 {
	twos := factorMultiplicity(value, binaryFactor)

	fives := factorMultiplicity(value, quinaryFactor)
	if fives < twos {
		return fives
	}

	return twos
}

// integerPower returns base raised to exponent.
func integerPower(base int64, exponent uint64) *big.Int {
	return new(big.Int).Exp(big.NewInt(base), new(big.Int).SetUint64(exponent), nil)
}

// decimalPower returns ten raised to exponent.
func decimalPower(exponent uint64) *big.Int {
	return integerPower(decimalRadix, exponent)
}
