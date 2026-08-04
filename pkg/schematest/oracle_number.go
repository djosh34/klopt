package schematest

import (
	"fmt"
	"math/big"
)

const (
	// oracleNumericValidLevel is the single local valid level for one numeric rule.
	oracleNumericValidLevel = "valid"

	// oracleFloatPrecision is the IEEE-754 binary32 significand precision.
	oracleFloatPrecision uint = 24
	// oracleDoublePrecision is the IEEE-754 binary64 significand precision.
	oracleDoublePrecision uint = 53
	// oracleFloatOverflowExponent is one past the binary32 maximum exponent.
	oracleFloatOverflowExponent uint = 128
	// oracleDoubleOverflowExponent is one past the binary64 maximum exponent.
	oracleDoubleOverflowExponent uint = 1024
)

// evaluateNumberRules applies numeric constraints only to JSON number values.
func evaluateNumberRules(
	result *evaluation,
	node *schemaNode,
	occurrence schemaOccurrence,
	value *jsonValue,
) {
	if value.kind != jsonNumber {
		return
	}

	if node.minimum != nil {
		if err := evaluateNumberBoundRule(
			result, occurrence, value.number, node.minimum, node.exclusiveMinimum, true,
		); err != nil {
			result.err = err

			return
		}
	}

	if node.maximum != nil {
		if err := evaluateNumberBoundRule(
			result, occurrence, value.number, node.maximum, node.exclusiveMaximum, false,
		); err != nil {
			result.err = err

			return
		}
	}

	if node.multipleOf != nil {
		if err := evaluateNumberMultipleOfRule(result, occurrence, value.number, node.multipleOf); err != nil {
			result.err = err

			return
		}
	}

	if isNumericSchemaFormat(node.format) {
		if err := evaluateNumberFormatRule(result, occurrence, value.number, node.format); err != nil {
			result.err = err
		}
	}
}

// evaluateNumberBoundRule evaluates one inclusive or exclusive numeric bound.
func evaluateNumberBoundRule(
	result *evaluation,
	occurrence schemaOccurrence,
	value, bound *exactNumber,
	exclusive, minimum bool,
) error {
	rule := oracleRuleMaximum
	if minimum {
		rule = oracleRuleMinimum
	}

	if exclusive {
		if minimum {
			rule = oracleRuleExclusiveMinimum
		} else {
			rule = oracleRuleExclusiveMaximum
		}
	}

	identity := makeRuleIdentity(occurrence, rule)
	appendApplicable(result, identity)

	comparison, err := bound.compare(value)
	if err != nil {
		return fmt.Errorf("%s: %w", identity, err)
	}

	violated := comparison == 0 && exclusive
	if minimum {
		violated = violated || comparison > 0
	} else {
		violated = violated || comparison < 0
	}

	if violated {
		appendFailure(result, identity)
	} else {
		appendNumericObservation(result, identity)
	}

	return nil
}

// evaluateNumberMultipleOfRule evaluates exact numeric divisibility.
func evaluateNumberMultipleOfRule(
	result *evaluation,
	occurrence schemaOccurrence,
	value, divisor *exactNumber,
) error {
	identity := makeRuleIdentity(occurrence, oracleRuleMultipleOf)
	appendApplicable(result, identity)

	multiple, err := value.isMultipleOf(divisor)
	if err != nil {
		return fmt.Errorf("%s: %w", identity, err)
	}

	if multiple {
		appendNumericObservation(result, identity)
	} else {
		appendFailure(result, identity)
	}

	return nil
}

// evaluateNumberFormatRule evaluates one admitted numeric format.
func evaluateNumberFormatRule(
	result *evaluation,
	occurrence schemaOccurrence,
	value *exactNumber,
	format schemaFormat,
) error {
	identity := makeRuleIdentity(occurrence, oracleRuleFormat)
	appendApplicable(result, identity)

	matches, err := numericFormatMatches(value, format)
	if err != nil {
		return fmt.Errorf("%s: %w", identity, err)
	}

	if matches {
		appendNumericObservation(result, identity)
	} else {
		appendFailure(result, identity)
	}

	return nil
}

// appendNumericObservation records one successful numeric rule at its stable level.
func appendNumericObservation(result *evaluation, identity ruleIdentity) {
	appendObserved(result, levelIdentity{
		ruleIdentity: identity,
		level:        oracleNumericValidLevel,
	})
}

// isNumericSchemaFormat reports whether format has numeric semantics in the closed profile.
func isNumericSchemaFormat(format schemaFormat) bool {
	switch format {
	case schemaFormatInt32, schemaFormatInt64, schemaFormatFloat, schemaFormatDouble:
		return true
	default:
		return false
	}
}

// numericFormatMatches applies the closed exact numeric format contract.
func numericFormatMatches(value *exactNumber, format schemaFormat) (bool, error) {
	switch format {
	case schemaFormatInt32:
		return exactSignedIntegerFormatMatches(value, "-2147483648", "2147483647")
	case schemaFormatInt64:
		return exactSignedIntegerFormatMatches(value, "-9223372036854775808", "9223372036854775807")
	case schemaFormatFloat, schemaFormatDouble:
		return exactBinaryFloatFormatMatches(value, format)
	default:
		return false, fmt.Errorf("format %d is not numeric", format)
	}
}

// exactSignedIntegerFormatMatches applies mathematical integrality and inclusive bounds.
func exactSignedIntegerFormatMatches(value *exactNumber, minimumSource, maximumSource string) (bool, error) {
	integer, err := value.isInteger()
	if err != nil {
		return false, err
	}

	if !integer {
		return false, nil
	}

	minimum, err := parseExactNumber(minimumSource)
	if err != nil {
		return false, err
	}

	minimumComparison, err := minimum.compare(value)
	if err != nil {
		return false, err
	}

	if minimumComparison > 0 {
		return false, nil
	}

	maximum, err := parseExactNumber(maximumSource)
	if err != nil {
		return false, err
	}

	maximumComparison, err := maximum.compare(value)
	if err != nil {
		return false, err
	}

	return maximumComparison >= 0, nil
}

// exactBinaryFloatFormatMatches mirrors strconv's exact finite-overflow cutoff without parsing floats.
func exactBinaryFloatFormatMatches(value *exactNumber, format schemaFormat) (bool, error) {
	limit, err := exactBinaryFloatOverflowLimit(format)
	if err != nil {
		return false, err
	}

	negativeLimit, err := newExactRational(new(big.Int).Neg(limit.numerator), limit.denominator)
	if err != nil {
		return false, err
	}

	lowerComparison, err := negativeLimit.compare(value)
	if err != nil {
		return false, err
	}

	if lowerComparison >= 0 {
		return false, nil
	}

	upperComparison, err := limit.compare(value)
	if err != nil {
		return false, err
	}

	return upperComparison > 0, nil
}

// exactBinaryFloatOverflowLimit returns the first exact decimal value that rounds to infinity.
func exactBinaryFloatOverflowLimit(format schemaFormat) (*exactNumber, error) {
	precision := oracleFloatPrecision
	exponent := oracleFloatOverflowExponent

	if format == schemaFormatDouble {
		precision = oracleDoublePrecision
		exponent = oracleDoubleOverflowExponent
	}

	limit := new(big.Int).Lsh(big.NewInt(1), exponent)
	halfSpacing := new(big.Int).Lsh(big.NewInt(1), exponent-precision-1)
	limit.Sub(limit, halfSpacing)

	return newExactRational(limit, big.NewInt(1))
}
