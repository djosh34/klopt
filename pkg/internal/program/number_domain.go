//nolint:godoclint,mnd // Private exact-decimal domain operations have no policy.
package program

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type numberInterval struct {
	lower          *big.Rat
	upper          *big.Rat
	lowerExclusive bool
	upperExclusive bool
}

type integerDomain struct {
	lower *big.Int
	upper *big.Int
}

func materializedRat(number jsonvalue.Number, work *decodeWork) (*big.Rat, error) {
	if number.Rational == nil {
		return nil, &ResourceError{
			Resource: "exact decimal", Limit: work.limits.MaxSolverBytes, Observed: ^uint64(0),
		}
	}

	bytes, err := exactValueBytes(jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: number})
	if err != nil {
		return nil, err
	}

	if err := work.chargeSolver("exact decimal", "exact decimal bytes", 1, bytes); err != nil {
		return nil, err
	}

	return new(big.Rat).Set(number.Rational), nil
}

func (interval *numberInterval) addLower(value *big.Rat, exclusive bool) {
	if interval.lower == nil || value.Cmp(interval.lower) > 0 ||
		value.Cmp(interval.lower) == 0 && exclusive && !interval.lowerExclusive {
		interval.lower = new(big.Rat).Set(value)
		interval.lowerExclusive = exclusive
	}
}

func (interval *numberInterval) addUpper(value *big.Rat, exclusive bool) {
	if interval.upper == nil || value.Cmp(interval.upper) < 0 ||
		value.Cmp(interval.upper) == 0 && exclusive && !interval.upperExclusive {
		interval.upper = new(big.Rat).Set(value)
		interval.upperExclusive = exclusive
	}
}

func (interval numberInterval) empty() bool {
	if interval.lower == nil || interval.upper == nil {
		return false
	}

	comparison := interval.lower.Cmp(interval.upper)

	return comparison > 0 || comparison == 0 && (interval.lowerExclusive || interval.upperExclusive)
}

func (interval numberInterval) integerDomain(unit *big.Rat) integerDomain {
	domain := integerDomain{}

	if interval.lower != nil {
		ratio := new(big.Rat).Quo(interval.lower, unit)

		domain.lower = ceilRat(ratio)
		if interval.lowerExclusive && ratio.IsInt() {
			domain.lower.Add(domain.lower, big.NewInt(1))
		}
	}

	if interval.upper != nil {
		ratio := new(big.Rat).Quo(interval.upper, unit)

		domain.upper = floorRat(ratio)
		if interval.upperExclusive && ratio.IsInt() {
			domain.upper.Sub(domain.upper, big.NewInt(1))
		}
	}

	return domain
}

func (domain integerDomain) empty() bool {
	return domain.lower != nil && domain.upper != nil && domain.lower.Cmp(domain.upper) > 0
}

func (domain integerDomain) count() *big.Int {
	if domain.lower == nil || domain.upper == nil || domain.empty() {
		return nil
	}

	return new(big.Int).Add(new(big.Int).Sub(domain.upper, domain.lower), big.NewInt(1))
}

func (domain integerDomain) at(rank *big.Int) *big.Int {
	if count := domain.count(); count != nil {
		index := new(big.Int).Mod(new(big.Int).Set(rank), count)

		return index.Add(index, domain.lower)
	}

	if domain.lower != nil {
		return new(big.Int).Add(domain.lower, rank)
	}

	if domain.upper != nil {
		return new(big.Int).Sub(domain.upper, rank)
	}

	if rank.Sign() == 0 {
		return new(big.Int)
	}

	magnitude := new(big.Int).Add(rank, big.NewInt(1))
	magnitude.Quo(magnitude, big.NewInt(2))

	if rank.Bit(0) == 0 {
		magnitude.Neg(magnitude)
	}

	return magnitude
}

func readNatural(reader *tapeReader, work *decodeWork) (*big.Int, error) {
	if reader == nil {
		return new(big.Int), nil
	}

	value, err := reader.natural(work.step, work.solver)
	if err != nil {
		return nil, err
	}

	return value, nil
}

func floorRat(value *big.Rat) *big.Int {
	quotient := new(big.Int)
	remainder := new(big.Int)
	quotient.QuoRem(value.Num(), value.Denom(), remainder)

	if value.Sign() < 0 && remainder.Sign() != 0 {
		quotient.Sub(quotient, big.NewInt(1))
	}

	return quotient
}

func ceilRat(value *big.Rat) *big.Int {
	negated := new(big.Rat).Neg(value)

	return new(big.Int).Neg(floorRat(negated))
}

func rationalLCM(left *big.Rat, right *big.Rat) *big.Rat {
	if left == nil {
		return new(big.Rat).Set(right)
	}

	numeratorGCD := new(big.Int).GCD(nil, nil, left.Num(), right.Num())
	numerator := new(big.Int).Quo(new(big.Int).Set(left.Num()), numeratorGCD)
	numerator.Mul(numerator, right.Num())
	numerator.Abs(numerator)

	denominator := new(big.Int).GCD(nil, nil, left.Denom(), right.Denom())

	return new(big.Rat).SetFrac(numerator, denominator)
}

func finiteScale(value *big.Rat) (int, error) {
	remaining := new(big.Int).Set(value.Denom())
	twos := 0

	for remaining.Bit(0) == 0 {
		remaining.Quo(remaining, big.NewInt(2))

		twos++
	}

	fives := 0

	for new(big.Int).Mod(remaining, big.NewInt(5)).Sign() == 0 {
		remaining.Quo(remaining, big.NewInt(5))

		fives++
	}

	if remaining.Cmp(big.NewInt(1)) != 0 {
		return 0, fmt.Errorf("rational is not a finite decimal")
	}

	return max(twos, fives), nil
}

func finiteDecimal(value *big.Rat, work *decodeWork) (string, error) {
	scale, err := finiteScale(value)
	if err != nil {
		return "", err
	}

	if uint64(scale) > work.limits.MaxSolverBytes {
		return "", &ResourceError{
			Resource: "decimal digits", Limit: work.limits.MaxSolverBytes, Observed: uint64(scale),
		}
	}

	text := value.FloatString(scale)
	if strings.Contains(text, ".") {
		text = strings.TrimRight(text, "0")
		text = strings.TrimRight(text, ".")
	}

	if text == "-0" {
		return "0", nil
	}

	return text, nil
}
