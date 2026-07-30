//nolint:godoclint // Private arithmetic-theory helpers stay behind SetArena.IsEmpty.
package suite

import (
	"math/big"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

//nolint:cyclop,gocognit,gocyclo // Exact ranges and arithmetic lattices are normalized in one pass.
func (arena *SetArena) numberAssignmentProductive(assignment map[AtomID]bool) (bool, error) {
	var (
		minimum *jsonvalue.Number
		maximum *jsonvalue.Number
	)

	exclusiveMinimum := false
	exclusiveMaximum := false
	integer := false
	nonInteger := false

	var step *big.Rat

	excludedSteps := make([]*big.Rat, 0)

	for identifier, want := range assignment {
		if !atomApplies(arena.Atoms[identifier], jsonvalue.KindNumber) ||
			identifierIsEnum(arena.Atoms[identifier]) {
			continue
		}

		switch value := arena.Atoms[identifier].(type) {
		case integerAtom:
			if want {
				integer = true
			} else {
				nonInteger = true
			}
		case numberRangeAtom:
			if !want {
				continue
			}

			minimum, exclusiveMinimum = tighterMinimum(
				minimum, exclusiveMinimum, value.Minimum, value.ExclusiveMinimum,
			)
			maximum, exclusiveMaximum = tighterMaximum(
				maximum, exclusiveMaximum, value.Maximum, value.ExclusiveMaximum,
			)
		case multipleOfAtom:
			if value.Value.Rational == nil {
				return true, nil
			}

			if want {
				step = intersectSteps(step, value.Value.Rational)
			} else {
				excludedSteps = append(excludedSteps, value.Value.Rational)
			}
		case floatFormatAtom:
			// Both the finite float range and its exact complement are productive.
		}
	}

	if minimum != nil && maximum != nil {
		comparison := minimum.Compare(*maximum)
		if comparison > 0 || comparison == 0 && (exclusiveMinimum || exclusiveMaximum) {
			return false, nil
		}

		if comparison == 0 {
			candidate := jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: *minimum}

			return arena.candidateMatchesAssignment(candidate, assignment)
		}
	}

	if integer {
		step = intersectSteps(step, big.NewRat(1, 1))
	}

	if integer && nonInteger {
		return false, nil
	}

	if step == nil {
		return true, nil
	}

	if nonInteger && step.IsInt() {
		return false, nil
	}

	for _, excluded := range excludedSteps {
		if new(big.Rat).Quo(step, excluded).IsInt() {
			return false, nil
		}
	}

	var first *big.Int

	if minimum != nil {
		if minimum.Rational == nil {
			return true, nil
		}

		quotient := new(big.Rat).Quo(minimum.Rational, step)

		first = ceilRat(quotient)
		if exclusiveMinimum && quotient.IsInt() {
			first.Add(first, big.NewInt(1))
		}
	}

	var last *big.Int

	if maximum != nil {
		if maximum.Rational == nil {
			return true, nil
		}

		quotient := new(big.Rat).Quo(maximum.Rational, step)

		last = floorRat(quotient)
		if exclusiveMaximum && quotient.IsInt() {
			last.Sub(last, big.NewInt(1))
		}
	}

	if first != nil && last != nil && first.Cmp(last) > 0 {
		return false, nil
	}

	return true, nil
}

func identifierIsEnum(value atom) bool {
	_, ok := value.(enumAtom)

	return ok
}

func tighterMinimum(
	current *jsonvalue.Number,
	currentExclusive bool,
	candidate *jsonvalue.Number,
	candidateExclusive bool,
) (*jsonvalue.Number, bool) {
	if candidate == nil {
		return current, currentExclusive
	}

	if current == nil {
		return candidate, candidateExclusive
	}

	comparison := candidate.Compare(*current)
	if comparison > 0 {
		return candidate, candidateExclusive
	}

	if comparison == 0 {
		return current, currentExclusive || candidateExclusive
	}

	return current, currentExclusive
}

func tighterMaximum(
	current *jsonvalue.Number,
	currentExclusive bool,
	candidate *jsonvalue.Number,
	candidateExclusive bool,
) (*jsonvalue.Number, bool) {
	if candidate == nil {
		return current, currentExclusive
	}

	if current == nil {
		return candidate, candidateExclusive
	}

	comparison := candidate.Compare(*current)
	if comparison < 0 {
		return candidate, candidateExclusive
	}

	if comparison == 0 {
		return current, currentExclusive || candidateExclusive
	}

	return current, currentExclusive
}

func intersectSteps(current *big.Rat, candidate *big.Rat) *big.Rat {
	if current == nil {
		return new(big.Rat).Set(candidate)
	}

	numerator := lcmBig(current.Num(), candidate.Num())
	denominator := new(big.Int).GCD(nil, nil, current.Denom(), candidate.Denom())

	return new(big.Rat).SetFrac(numerator, denominator)
}

func lcmBig(left *big.Int, right *big.Int) *big.Int {
	gcd := new(big.Int).GCD(nil, nil, left, right)
	result := new(big.Int).Quo(new(big.Int).Set(left), gcd)

	return result.Mul(result, right)
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
