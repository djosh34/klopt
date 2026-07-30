//nolint:godoclint,mnd // Exact numeric witness construction stays behind graph lowering.
package suite

import (
	"fmt"
	"math/big"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

//nolint:cyclop,gocognit,gocyclo,mnd // Signed numeric atoms contribute independent exact boundary candidates.
func (finder *valueFinder) numberValues(assignment map[AtomID]bool) ([]jsonvalue.Value, error) {
	lexemes := []string{"0", "1", "-1", "0.5", "-0.5", "1e400", "-1e400"}
	integer := false
	nonInteger := false

	var (
		minimum *jsonvalue.Number
		maximum *jsonvalue.Number
	)

	exclusiveMinimum := false
	exclusiveMaximum := false

	var step *big.Rat

	for identifier, want := range assignment {
		switch value := finder.arena.Atoms[identifier].(type) {
		case integerAtom:
			integer = integer || want
			nonInteger = nonInteger || !want
		case numberRangeAtom:
			for _, bound := range []*jsonvalue.Number{value.Minimum, value.Maximum} {
				if bound == nil {
					continue
				}

				lexemes = append(lexemes, bound.Lexeme)
				if bound.Rational != nil {
					lexemes = appendRationalOffsets(lexemes, bound.Rational)
				}
			}

			if want {
				minimum, exclusiveMinimum = tighterMinimum(
					minimum, exclusiveMinimum, value.Minimum, value.ExclusiveMinimum,
				)
				maximum, exclusiveMaximum = tighterMaximum(
					maximum, exclusiveMaximum, value.Maximum, value.ExclusiveMaximum,
				)
			}
		case multipleOfAtom:
			lexemes = append(lexemes, value.Value.Lexeme)
			if value.Value.Rational != nil {
				lexemes = appendRationalOffsets(lexemes, value.Value.Rational)
				if want {
					step = intersectSteps(step, value.Value.Rational)
				}
			}
		case floatFormatAtom:
			if !want {
				lexemes = append(lexemes, "1e400", "-1e400")
			}
		}
	}

	if minimum != nil && maximum != nil && minimum.Rational != nil && maximum.Rational != nil {
		midpoint := new(big.Rat).Add(minimum.Rational, maximum.Rational)
		midpoint.Quo(midpoint, big.NewRat(2, 1))
		lexemes = append(lexemes, rationalLexeme(midpoint))
	}

	if step != nil {
		factor := big.NewInt(0)
		if minimum != nil && minimum.Rational != nil {
			factor = ceilRat(new(big.Rat).Quo(minimum.Rational, step))
			if exclusiveMinimum && new(big.Rat).Quo(minimum.Rational, step).IsInt() {
				factor.Add(factor, big.NewInt(1))
			}
		}

		for _, offset := range []int64{-1, 0, 1} {
			candidateFactor := new(big.Int).Add(factor, big.NewInt(offset))
			candidate := new(big.Rat).Mul(step, new(big.Rat).SetInt(candidateFactor))
			lexemes = append(lexemes, rationalLexeme(candidate))
		}
	}

	if integer && minimum != nil && minimum.Rational != nil {
		first := ceilRat(minimum.Rational)
		if exclusiveMinimum && minimum.Rational.IsInt() {
			first.Add(first, big.NewInt(1))
		}

		lexemes = append(lexemes, first.String())
	}

	if nonInteger {
		lexemes = append(lexemes, "0.5")
	}

	result := make([]jsonvalue.Value, 0, len(lexemes))
	seen := make(map[string]struct{})

	for _, lexeme := range lexemes {
		if lexeme == "" {
			continue
		}

		number, err := jsonvalue.ParseNumber(lexeme)
		if err != nil {
			return nil, fmt.Errorf("construct exact number %q: %w", lexeme, err)
		}

		if _, duplicate := seen[number.Lexeme]; duplicate {
			continue
		}

		seen[number.Lexeme] = struct{}{}
		result = append(result, jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: number})
	}

	return result, nil
}

func appendRationalOffsets(result []string, value *big.Rat) []string {
	for _, offset := range []*big.Rat{big.NewRat(-1, 1), big.NewRat(-1, 10), big.NewRat(1, 10), big.NewRat(1, 1)} {
		result = append(result, rationalLexeme(new(big.Rat).Add(value, offset)))
	}

	return result
}

func rationalLexeme(value *big.Rat) string {
	denominator := new(big.Int).Set(value.Denom())
	twos := 0
	fives := 0

	for new(big.Int).Mod(denominator, big.NewInt(2)).Sign() == 0 {
		denominator.Quo(denominator, big.NewInt(2))

		twos++
	}

	for new(big.Int).Mod(denominator, big.NewInt(5)).Sign() == 0 {
		denominator.Quo(denominator, big.NewInt(5))

		fives++
	}

	if denominator.Cmp(big.NewInt(1)) != 0 {
		return ""
	}

	return value.FloatString(max(twos, fives))
}
