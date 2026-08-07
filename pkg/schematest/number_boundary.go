package schematest

import (
	"errors"
	"math/big"
)

// numberDeterministicCandidates returns exact boundary, divisibility, and integer-format candidates.
func numberDeterministicCandidates(node *schemaNode) ([]*jsonValue, error) {
	if node == nil || node.schemaShape == nil {
		return nil, errors.New("schematest: number boundary schema has no shape")
	}

	quantum, err := numberBoundaryQuantum(node)
	if err != nil {
		return nil, err
	}

	var candidates []*jsonValue

	err = appendNumberBoundaryTriplet(&candidates, node.minimum, quantum, true)
	if err != nil {
		return nil, err
	}

	err = appendNumberBoundaryTriplet(&candidates, node.maximum, quantum, false)
	if err != nil {
		return nil, err
	}

	zero, err := parseExactNumber("0")
	if err != nil {
		return nil, err
	}

	candidates, err = appendUniqueJSONWitness(
		candidates,
		&jsonValue{kind: jsonNumber, number: zero},
	)
	if err != nil {
		return nil, err
	}

	if err := appendNumberMultipleCandidates(&candidates, node.multipleOf, quantum); err != nil {
		return nil, err
	}

	if err := appendNumberFormatCandidates(&candidates, node.format); err != nil {
		return nil, err
	}

	return candidates, nil
}

// appendNumberMultipleCandidates appends the divisor and its directed exact neighbors.
func appendNumberMultipleCandidates(
	candidates *[]*jsonValue,
	divisor, quantum *exactNumber,
) error {
	if divisor == nil {
		return nil
	}

	negativeDivisor, err := newExactNumber(
		new(big.Int).Neg(divisor.numerator),
		divisor.denominator,
		divisor.exponent,
		divisor.scale,
	)
	if err != nil {
		return err
	}

	positiveNeighbor, err := addExactNumbers(divisor, quantum)
	if err != nil {
		return err
	}

	negativeQuantum, err := newExactNumber(
		new(big.Int).Neg(quantum.numerator),
		quantum.denominator,
		quantum.exponent,
		quantum.scale,
	)
	if err != nil {
		return err
	}

	negativeNeighbor, err := addExactNumbers(divisor, negativeQuantum)
	if err != nil {
		return err
	}

	for _, number := range []*exactNumber{divisor, negativeDivisor, positiveNeighbor, negativeNeighbor} {
		var appendErr error

		*candidates, appendErr = appendUniqueJSONWitness(
			*candidates,
			&jsonValue{kind: jsonNumber, number: number},
		)
		if appendErr != nil {
			return appendErr
		}
	}

	return nil
}

// appendNumberFormatCandidates appends exact signed format edges and outside values.
func appendNumberFormatCandidates(candidates *[]*jsonValue, format schemaFormat) error {
	var sources []string

	switch format {
	case schemaFormatInt32:
		sources = []string{"-2147483648", "-2147483649", "2147483647", "2147483648"}
	case schemaFormatInt64:
		sources = []string{
			"-9223372036854775808", "-9223372036854775809",
			"9223372036854775807", "9223372036854775808",
		}
	case schemaFormatFloat, schemaFormatDouble:
		return appendNumberFloatFormatCandidates(candidates, format)
	default:
		return nil
	}

	for _, source := range sources {
		number, err := parseExactNumber(source)
		if err != nil {
			return err
		}

		*candidates, err = appendUniqueJSONWitness(
			*candidates,
			&jsonValue{kind: jsonNumber, number: number},
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// appendNumberFloatFormatCandidates uses the exact finite-overflow cutoff.
func appendNumberFloatFormatCandidates(candidates *[]*jsonValue, format schemaFormat) error {
	limit, err := exactBinaryFloatOverflowLimit(format)
	if err != nil {
		return err
	}

	one, err := parseExactNumber("1")
	if err != nil {
		return err
	}

	negativeOne, err := parseExactNumber("-1")
	if err != nil {
		return err
	}

	positiveInside, err := addExactNumbers(limit, negativeOne)
	if err != nil {
		return err
	}

	negativeLimit, err := newExactRational(new(big.Int).Neg(limit.numerator), limit.denominator)
	if err != nil {
		return err
	}

	negativeInside, err := addExactNumbers(negativeLimit, one)
	if err != nil {
		return err
	}

	for _, number := range []*exactNumber{negativeInside, negativeLimit, positiveInside, limit} {
		*candidates, err = appendUniqueJSONWitness(
			*candidates,
			&jsonValue{kind: jsonNumber, number: number},
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// numberBoundaryQuantum returns one for integer schemas and the finest authored numeric unit otherwise.
func numberBoundaryQuantum(node *schemaNode) (*exactNumber, error) {
	if node.kind == schemaInteger {
		return parseExactNumber("1")
	}

	var numbers []*exactNumber
	if node.minimum != nil {
		numbers = append(numbers, node.minimum)
	}

	if node.maximum != nil {
		numbers = append(numbers, node.maximum)
	}

	if node.multipleOf != nil {
		numbers = append(numbers, node.multipleOf)
	}

	if len(numbers) == 0 {
		return parseExactNumber("1")
	}

	return exactQuantum(numbers...)
}

// appendNumberBoundaryTriplet appends one bound and its two directed quantum neighbors.
func appendNumberBoundaryTriplet(
	candidates *[]*jsonValue,
	bound, quantum *exactNumber,
	minimum bool,
) error {
	if bound == nil {
		return nil
	}

	positive, err := addExactNumbers(bound, quantum)
	if err != nil {
		return err
	}

	negativeQuantum, err := newExactNumber(
		new(big.Int).Neg(quantum.numerator),
		quantum.denominator,
		quantum.exponent,
		quantum.scale,
	)
	if err != nil {
		return err
	}

	negative, err := addExactNumbers(bound, negativeQuantum)
	if err != nil {
		return err
	}

	ordered := []*exactNumber{bound, positive, negative}
	if !minimum {
		ordered = []*exactNumber{bound, negative, positive}
	}

	for _, number := range ordered {
		var appendErr error

		*candidates, appendErr = appendUniqueJSONWitness(
			*candidates,
			&jsonValue{kind: jsonNumber, number: number},
		)
		if appendErr != nil {
			return appendErr
		}
	}

	return nil
}

// addExactNumbers adds scaled rationals without binary floating point.
func addExactNumbers(left, right *exactNumber) (*exactNumber, error) {
	if err := left.validate(); err != nil {
		return nil, err
	}

	if err := right.validate(); err != nil {
		return nil, err
	}

	exponent := new(big.Int).Set(left.exponent)
	if right.exponent.Cmp(exponent) < 0 {
		exponent.Set(right.exponent)
	}

	leftShift := new(big.Int).Sub(left.exponent, exponent)

	rightShift := new(big.Int).Sub(right.exponent, exponent)
	if !leftShift.IsUint64() || !rightShift.IsUint64() {
		return nil, errors.New("schematest: number boundary exponent difference does not fit uint64")
	}

	leftNumerator := new(big.Int).Mul(left.numerator, right.denominator)
	leftNumerator.Mul(leftNumerator, decimalPower(leftShift.Uint64()))

	rightNumerator := new(big.Int).Mul(right.numerator, left.denominator)
	rightNumerator.Mul(rightNumerator, decimalPower(rightShift.Uint64()))

	numerator := new(big.Int).Add(leftNumerator, rightNumerator)
	denominator := new(big.Int).Mul(left.denominator, right.denominator)

	scale := new(big.Int).Set(left.scale)
	if right.scale.Cmp(scale) > 0 {
		scale.Set(right.scale)
	}

	return newExactNumber(numerator, denominator, exponent, scale)
}
