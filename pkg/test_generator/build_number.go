//nolint:godoclint // Private exact-number construction is covered by semantic tests.
package testgenerator

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

const (
	numberRadix                  = 10
	decimalFactorTwo             = 2
	decimalFactorFive            = 5
	fractionalWitnessDenominator = 2
)

type numberInterval struct {
	lower          *big.Rat
	lowerInclusive bool
	upper          *big.Rat
	upperInclusive bool
}

//nolint:cyclop // Numeric constraints are combined in the fixed schema-keyword order.
func buildNumber(selected []demand, tape *tapeCursor) buildResult {
	if tape == nil {
		return failedBuild(errors.New("build number with nil tape cursor"))
	}

	word := tape.takeWord()

	enumValues, hasEnum, err := selectedEnumValues(selected)
	if err != nil {
		return failedBuild(err)
	}

	if hasEnum {
		numbers := make([]jsonvalue.Value, 0, len(enumValues))
		for _, value := range enumValues {
			if value.Kind == jsonvalue.KindNumber {
				numbers = append(numbers, value)
			}
		}

		if len(numbers) == 0 {
			return missedBuild()
		}

		candidate := numbers[0]
		if len(numbers) > 1 {
			candidate = numbers[word%uint64(len(numbers))]
		}

		return buildSelectedValue(selected, candidate)
	}

	domain, err := collectNumberDomain(selected)
	if err != nil {
		return failedBuild(err)
	}

	candidate, err := chooseNumber(domain, word)
	if err != nil {
		return failedBuild(err)
	}

	forbidden, err := containsNumberEnum(domain.negativeEnums, candidate)
	if err != nil {
		return failedBuild(err)
	}

	if forbidden {
		candidate.Add(candidate, chooseNonzeroStep(domain.step))
	}

	value, err := numberValue(candidate)
	if err != nil {
		return failedBuild(err)
	}

	return buildSelectedValue(selected, value)
}

type numberDomain struct {
	interval        numberInterval
	integer         bool
	negativeInteger bool
	step            *big.Rat
	negativeSteps   []jsonvalue.Number
	negativeFormats []string
	negativeEnums   []jsonvalue.Value
}

//nolint:cyclop,gocognit,nestif // Each numeric atom has one exact domain contribution.
func collectNumberDomain(selected []demand) (numberDomain, error) {
	domain := numberDomain{}

	for index, selectedDemand := range selected {
		if selectedDemand.expression == nil || selectedDemand.expression.kind != expressionAtom {
			return numberDomain{}, fmt.Errorf("number demand %d is not an atom", index)
		}

		rule := selectedDemand.expression.atom
		switch rule.kind {
		case atomKinds:
			if selectedDemand.wantPass && rule.integer {
				domain.integer = true
			} else if !selectedDemand.wantPass && rule.integer {
				domain.negativeInteger = true
			}
		case atomEnum:
			if !selectedDemand.wantPass {
				values, err := cloneValues(rule.values)
				if err != nil {
					return numberDomain{}, err
				}

				domain.negativeEnums = append(domain.negativeEnums, values...)
			}
		case atomNumberMinimum:
			bound, err := rationalNumber(rule.number, "minimum")
			if err != nil {
				return numberDomain{}, err
			}

			if selectedDemand.wantPass {
				domain.interval.addLower(bound, !rule.exclusive)
			} else {
				domain.interval.addUpper(bound, rule.exclusive)
			}
		case atomNumberMaximum:
			bound, err := rationalNumber(rule.number, "maximum")
			if err != nil {
				return numberDomain{}, err
			}

			if selectedDemand.wantPass {
				domain.interval.addUpper(bound, !rule.exclusive)
			} else {
				domain.interval.addLower(bound, rule.exclusive)
			}
		case atomNumberMultipleOf:
			multiple, err := rationalNumber(rule.number, "multipleOf")
			if err != nil {
				return numberDomain{}, err
			}

			if multiple.Sign() <= 0 {
				return numberDomain{}, errors.New("multipleOf atom is not positive")
			}

			if selectedDemand.wantPass {
				if domain.step == nil {
					domain.step = multiple
				} else {
					combined, combineErr := commonMultiple(domain.step, multiple)
					if combineErr != nil {
						return numberDomain{}, fmt.Errorf("combine multipleOf: %w", combineErr)
					}

					domain.step = combined
				}
			} else {
				domain.negativeSteps = append(domain.negativeSteps, rule.number)
			}
		case atomNumberFormat:
			if err := validateNumberFormatName(rule.text); err != nil {
				return numberDomain{}, err
			}

			if selectedDemand.wantPass {
				if err := addPositiveNumberFormat(&domain, rule.text); err != nil {
					return numberDomain{}, err
				}
			} else {
				domain.negativeFormats = append(domain.negativeFormats, rule.text)
			}
		}
	}

	return domain, nil
}

func containsNumberEnum(values []jsonvalue.Value, candidate *big.Rat) (bool, error) {
	if candidate == nil {
		return false, nil
	}

	value, err := numberValue(candidate)
	if err != nil {
		return false, err
	}

	return containsJSONValue(values, value), nil
}

func addPositiveNumberFormat(domain *numberDomain, format string) error {
	switch format {
	case "int32":
		domain.integer = true

		return addNumberFormatBounds(&domain.interval, "-2147483648", "2147483647")
	case "int64":
		domain.integer = true

		return addNumberFormatBounds(&domain.interval, "-9223372036854775808", "9223372036854775807")
	case "float", "double":
		return nil
	default:
		return fmt.Errorf("unsupported numeric format %q", format)
	}
}

func addNumberFormatBounds(interval *numberInterval, minimum string, maximum string) error {
	minimumRat, err := decimalRat(minimum)
	if err != nil {
		return err
	}

	maximumRat, err := decimalRat(maximum)
	if err != nil {
		return err
	}

	interval.addLower(minimumRat, true)
	interval.addUpper(maximumRat, true)

	return nil
}

func (interval *numberInterval) addLower(bound *big.Rat, inclusive bool) {
	if interval.lower == nil || bound.Cmp(interval.lower) > 0 ||
		bound.Cmp(interval.lower) == 0 && !inclusive && interval.lowerInclusive {
		interval.lower = new(big.Rat).Set(bound)
		interval.lowerInclusive = inclusive
	}
}

func (interval *numberInterval) addUpper(bound *big.Rat, inclusive bool) {
	if interval.upper == nil || bound.Cmp(interval.upper) < 0 ||
		bound.Cmp(interval.upper) == 0 && !inclusive && interval.upperInclusive {
		interval.upper = new(big.Rat).Set(bound)
		interval.upperInclusive = inclusive
	}
}

//nolint:cyclop // Number choice applies one deterministic rank to one exact domain.
func chooseNumber(domain numberDomain, word uint64) (*big.Rat, error) {
	unit := domain.step
	if unit == nil && domain.integer {
		unit = big.NewRat(1, 1)
	}

	var candidate *big.Rat
	if unit != nil {
		candidate = chooseGridNumber(domain.interval, unit, word)
	} else {
		candidate = chooseUnquantizedNumber(domain.interval)
	}

	if candidate == nil {
		return nil, errors.New("numeric demands have no feasible candidate")
	}

	if domain.negativeInteger && candidate.IsInt() {
		candidate.Add(candidate, big.NewRat(1, fractionalWitnessDenominator))
	}

	satisfies, err := satisfiesNegativeMultiples(candidate, domain.negativeSteps)
	if err != nil {
		return nil, err
	}

	if !satisfies {
		candidate = new(big.Rat).Add(candidate, chooseNonzeroStep(unit))
	}

	for _, format := range domain.negativeFormats {
		value, err := numberValue(candidate)
		if err != nil {
			return nil, err
		}

		holds, err := numberMatchesFormat(value.Number, format)
		if err != nil {
			return nil, err
		}

		if holds {
			witness, found, witnessErr := negativeFormatWitness(format, domain.interval, unit)
			if witnessErr != nil {
				return nil, witnessErr
			}

			if found {
				candidate = witness
			} else {
				candidate = new(big.Rat).Add(candidate, chooseNonzeroStep(unit))
			}
		}
	}

	return candidate, nil
}

func negativeFormatWitness(
	format string,
	interval numberInterval,
	unit *big.Rat,
) (*big.Rat, bool, error) {
	switch format {
	case "int32":
		return integerFormatWitness(interval, unit, "2147483647", "-2147483648")
	case "int64":
		return integerFormatWitness(interval, unit, "9223372036854775807", "-9223372036854775808")
	case "float":
		candidate, err := decimalRat("1e40")

		return formatWitnessInInterval(candidate, interval, err)
	case "double":
		candidate, err := decimalRat("1e400")

		return formatWitnessInInterval(candidate, interval, err)
	default:
		return nil, false, fmt.Errorf("unsupported negative numeric format %q", format)
	}
}

func integerFormatWitness(
	interval numberInterval,
	unit *big.Rat,
	maximum string,
	minimum string,
) (*big.Rat, bool, error) {
	upper, err := decimalRat(maximum)
	if err != nil {
		return nil, false, err
	}

	lower, err := decimalRat(minimum)
	if err != nil {
		return nil, false, err
	}

	delta := chooseNonzeroStep(unit)

	above := new(big.Rat).Add(upper, delta)
	if ratInInterval(above, interval) {
		return above, true, nil
	}

	below := new(big.Rat).Sub(lower, delta)
	if ratInInterval(below, interval) {
		return below, true, nil
	}

	return nil, false, nil
}

func formatWitnessInInterval(
	candidate *big.Rat,
	interval numberInterval,
	err error,
) (*big.Rat, bool, error) {
	if err != nil {
		return nil, false, err
	}

	if !ratInInterval(candidate, interval) {
		return nil, false, nil
	}

	return candidate, true, nil
}

func chooseGridNumber(interval numberInterval, unit *big.Rat, word uint64) *big.Rat {
	if unit.Sign() <= 0 {
		return nil
	}

	first, hasFirst := firstGridRank(interval.lower, interval.lowerInclusive, unit)
	last, hasLast := lastGridRank(interval.upper, interval.upperInclusive, unit)

	switch {
	case hasFirst && hasLast:
		if last.Cmp(first) < 0 {
			return nil
		}

		count := new(big.Int).Add(new(big.Int).Sub(last, first), big.NewInt(1))
		rank := new(big.Int).SetUint64(word)
		rank.Mod(rank, count)
		rank.Add(rank, first)

		return new(big.Rat).Mul(unit, new(big.Rat).SetInt(rank))
	case hasFirst:
		rank := new(big.Int).SetUint64(word)
		rank.Add(rank, first)

		return new(big.Rat).Mul(unit, new(big.Rat).SetInt(rank))
	case hasLast:
		rank := new(big.Int).SetUint64(word)
		rank.Sub(last, rank)

		return new(big.Rat).Mul(unit, new(big.Rat).SetInt(rank))
	default:
		rank := new(big.Int).SetUint64(word / booleanValueCount)
		if word&1 != 0 {
			rank.Neg(rank)
		}

		return new(big.Rat).Mul(unit, new(big.Rat).SetInt(rank))
	}
}

func chooseUnquantizedNumber(interval numberInterval) *big.Rat {
	zero := big.NewRat(0, 1)
	if ratInInterval(zero, interval) {
		return zero
	}

	if interval.lower != nil {
		candidate := new(big.Rat).Set(interval.lower)
		if !interval.lowerInclusive {
			candidate.Add(candidate, big.NewRat(1, 1))
		}

		if ratInInterval(candidate, interval) {
			return candidate
		}
	}

	if interval.upper != nil {
		candidate := new(big.Rat).Set(interval.upper)
		if !interval.upperInclusive {
			candidate.Sub(candidate, big.NewRat(1, 1))
		}

		if ratInInterval(candidate, interval) {
			return candidate
		}
	}

	return nil
}

func firstGridRank(bound *big.Rat, inclusive bool, unit *big.Rat) (*big.Int, bool) {
	if bound == nil {
		return nil, false
	}

	ratio := new(big.Rat).Quo(bound, unit)

	rank := ratCeil(ratio)
	if !inclusive && new(big.Rat).SetInt(rank).Cmp(ratio) == 0 {
		rank.Add(rank, big.NewInt(1))
	}

	return rank, true
}

func lastGridRank(bound *big.Rat, inclusive bool, unit *big.Rat) (*big.Int, bool) {
	if bound == nil {
		return nil, false
	}

	ratio := new(big.Rat).Quo(bound, unit)

	rank := ratFloor(ratio)
	if !inclusive && new(big.Rat).SetInt(rank).Cmp(ratio) == 0 {
		rank.Sub(rank, big.NewInt(1))
	}

	return rank, true
}

func ratInInterval(value *big.Rat, interval numberInterval) bool {
	if interval.lower != nil {
		comparison := value.Cmp(interval.lower)
		if comparison < 0 || comparison == 0 && !interval.lowerInclusive {
			return false
		}
	}

	if interval.upper != nil {
		comparison := value.Cmp(interval.upper)
		if comparison > 0 || comparison == 0 && !interval.upperInclusive {
			return false
		}
	}

	return true
}

func satisfiesNegativeMultiples(value *big.Rat, rules []jsonvalue.Number) (bool, error) {
	for _, rule := range rules {
		multiple, err := rationalNumber(rule, "multipleOf")
		if err != nil {
			return false, err
		}

		if rationalIsMultiple(value, multiple) {
			return false, nil
		}
	}

	return true, nil
}

func rationalIsMultiple(value *big.Rat, divisor *big.Rat) bool {
	if divisor == nil || divisor.Sign() == 0 {
		return false
	}

	quotient := new(big.Rat).Quo(value, divisor)

	return quotient.IsInt()
}

func chooseNonzeroStep(unit *big.Rat) *big.Rat {
	if unit != nil && unit.Sign() != 0 {
		return new(big.Rat).Set(unit)
	}

	return big.NewRat(1, 1)
}

func commonMultiple(left *big.Rat, right *big.Rat) (*big.Rat, error) {
	if left.Sign() <= 0 || right.Sign() <= 0 {
		return nil, errors.New("multipleOf must be positive")
	}

	leftNumerator := new(big.Int).Abs(left.Num())
	rightNumerator := new(big.Int).Abs(right.Num())
	numerator := new(big.Int).Mul(leftNumerator, rightNumerator)
	numerator.Quo(numerator, new(big.Int).GCD(nil, nil, leftNumerator, rightNumerator))
	denominator := new(big.Int).GCD(nil, nil, left.Denom(), right.Denom())

	return new(big.Rat).SetFrac(numerator, denominator), nil
}

func ratFloor(value *big.Rat) *big.Int {
	quotient, remainder := new(big.Int).QuoRem(value.Num(), value.Denom(), new(big.Int))
	if value.Sign() < 0 && remainder.Sign() != 0 {
		quotient.Sub(quotient, big.NewInt(1))
	}

	return quotient
}

func ratCeil(value *big.Rat) *big.Int {
	quotient, remainder := new(big.Int).QuoRem(value.Num(), value.Denom(), new(big.Int))
	if value.Sign() > 0 && remainder.Sign() != 0 {
		quotient.Add(quotient, big.NewInt(1))
	}

	return quotient
}

func rationalNumber(number jsonvalue.Number, keyword string) (*big.Rat, error) {
	if number.Lexeme == "" {
		return nil, fmt.Errorf("%s atom has an empty number", keyword)
	}

	rat, err := decimalRat(number.Lexeme)
	if err != nil {
		return nil, fmt.Errorf("%s atom number: %w", keyword, err)
	}

	return rat, nil
}

func decimalRat(lexeme string) (*big.Rat, error) {
	negative := strings.HasPrefix(lexeme, "-")
	if negative {
		lexeme = lexeme[1:]
	}

	exponent := int64(0)

	if position := strings.IndexAny(lexeme, "eE"); position >= 0 {
		parsed, err := strconv.ParseInt(lexeme[position+1:], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number exponent: %w", err)
		}

		exponent = parsed
		lexeme = lexeme[:position]
	}

	fractionDigits := int64(0)
	if position := strings.IndexByte(lexeme, '.'); position >= 0 {
		fractionDigits = int64(len(lexeme) - position - 1)
		lexeme = lexeme[:position] + lexeme[position+1:]
	}

	if lexeme == "" {
		return nil, errors.New("number has no digits")
	}

	numerator, ok := new(big.Int).SetString(lexeme, numberRadix)
	if !ok {
		return nil, fmt.Errorf("invalid decimal number %q", lexeme)
	}

	if negative {
		numerator.Neg(numerator)
	}

	denominator := big.NewInt(1)
	if exponent < fractionDigits {
		denominator.Exp(big.NewInt(numberRadix), big.NewInt(fractionDigits-exponent), nil)
	} else if exponent > fractionDigits {
		numerator.Mul(numerator, new(big.Int).Exp(big.NewInt(numberRadix), big.NewInt(exponent-fractionDigits), nil))
	}

	return new(big.Rat).SetFrac(numerator, denominator), nil
}

func numberValue(value *big.Rat) (jsonvalue.Value, error) {
	lexeme, err := finiteDecimal(value)
	if err != nil {
		return jsonvalue.Value{}, err
	}

	number, err := jsonvalue.ParseNumber(lexeme)
	if err != nil {
		return jsonvalue.Value{}, err
	}

	return jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: number}, nil
}

//nolint:cyclop // Decimal factorization follows one finite-decimal algorithm.
func finiteDecimal(value *big.Rat) (string, error) {
	if value == nil {
		return "", errors.New("nil rational number")
	}

	denominator := new(big.Int).Set(value.Denom())
	scale := int64(0)

	for denominator.Bit(0) == 0 {
		denominator.Rsh(denominator, 1)

		scale = max(scale, int64(1))
	}

	for new(big.Int).Mod(denominator, big.NewInt(decimalFactorFive)).Sign() == 0 {
		denominator.Quo(denominator, big.NewInt(decimalFactorFive))

		scale = max(scale, int64(1))
	}

	if denominator.Cmp(big.NewInt(1)) != 0 {
		return "", fmt.Errorf("rational %s is not a finite decimal", value.RatString())
	}

	// Recompute the decimal scale from the original denominator factors.
	originalDenominator := value.Denom()
	twoCount := int64(0)
	fiveCount := int64(0)

	remaining := new(big.Int).Set(originalDenominator)
	for new(big.Int).Mod(remaining, big.NewInt(decimalFactorTwo)).Sign() == 0 {
		remaining.Quo(remaining, big.NewInt(decimalFactorTwo))

		twoCount++
	}

	for new(big.Int).Mod(remaining, big.NewInt(decimalFactorFive)).Sign() == 0 {
		remaining.Quo(remaining, big.NewInt(decimalFactorFive))

		fiveCount++
	}

	scale = max(twoCount, fiveCount)

	multiplier := new(big.Int).Exp(big.NewInt(numberRadix), big.NewInt(scale), nil)
	numerator := new(big.Int).Mul(value.Num(), multiplier)
	quotient := new(big.Int).Quo(numerator, value.Denom())
	negative := quotient.Sign() < 0
	quotient.Abs(quotient)
	digits := quotient.String()

	if scale == 0 {
		if negative {
			return "-" + digits, nil
		}

		return digits, nil
	}

	places := int(scale)
	if len(digits) <= places {
		digits = strings.Repeat("0", places-len(digits)+1) + digits
	}

	position := len(digits) - places
	result := digits[:position] + "." + digits[position:]

	result = strings.TrimRight(strings.TrimRight(result, "0"), ".")
	if result == "" {
		result = "0"
	}

	if negative && result != "0" {
		result = "-" + result
	}

	return result, nil
}
