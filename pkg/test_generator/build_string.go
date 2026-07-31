//nolint:godoclint // Private string construction is covered by semantic tests.
package testgenerator

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // The builder uses the shared exact walk.
	"github.com/djosh34/klopt/pkg/jsonvalue"
)

//nolint:cyclop // String constraints and language requirements are one fixed combination.
func buildString(selected []demand, tape *tapeCursor) buildResult {
	if tape == nil {
		return failedBuild(errors.New("build string with nil tape cursor"))
	}

	enumValues, hasEnum, err := selectedEnumValues(selected)
	if err != nil {
		return failedBuild(err)
	}

	if hasEnum {
		stringsInEnum := make([]jsonvalue.Value, 0, len(enumValues))
		for _, value := range enumValues {
			if value.Kind == jsonvalue.KindString {
				stringsInEnum = append(stringsInEnum, value)
			}
		}

		if len(stringsInEnum) == 0 {
			return missedBuild()
		}

		candidate := stringsInEnum[0]
		if len(stringsInEnum) > 1 {
			candidate = stringsInEnum[tape.takeWord()%uint64(len(stringsInEnum))]
		}

		return buildSelectedValue(selected, candidate)
	}

	minimum, maximum, requirements, err := collectStringDomain(selected)
	if err != nil {
		return failedBuild(err)
	}

	walk, err := stringlanguage.Begin(requirements)
	if err != nil {
		return failedBuild(fmt.Errorf("begin string requirements: %w", err))
	}

	value, complete, err := walkString(walk, minimum, maximum, tape)
	if err != nil {
		return failedBuild(err)
	}

	if !complete {
		return missedBuild()
	}

	return buildSelectedValue(selected, jsonvalue.String(value))
}

//nolint:cyclop,gocognit // String bounds and language requirements are one fixed domain.
func collectStringDomain(
	selected []demand,
) (*big.Int, *big.Int, []stringlanguage.Requirement, error) {
	minimum := big.NewInt(0)

	var maximum *big.Int

	requirements := make([]stringlanguage.Requirement, 0)

	for index, selectedDemand := range selected {
		if selectedDemand.expression == nil || selectedDemand.expression.kind != expressionAtom {
			return nil, nil, nil, fmt.Errorf("string demand %d is not an atom", index)
		}

		rule := selectedDemand.expression.atom
		switch rule.kind {
		case atomStringMinLength:
			count, err := countBigInt(rule.count, "minLength")
			if err != nil {
				return nil, nil, nil, err
			}

			if selectedDemand.wantPass {
				if count.Cmp(minimum) > 0 {
					minimum = count
				}
			} else {
				upper := new(big.Int).Sub(count, big.NewInt(1))
				if maximum == nil || upper.Cmp(maximum) < 0 {
					maximum = upper
				}
			}
		case atomStringMaxLength:
			count, err := countBigInt(rule.count, "maxLength")
			if err != nil {
				return nil, nil, nil, err
			}

			if selectedDemand.wantPass {
				if maximum == nil || count.Cmp(maximum) < 0 {
					maximum = count
				}
			} else {
				lower := new(big.Int).Add(count, big.NewInt(1))
				if lower.Cmp(minimum) > 0 {
					minimum = lower
				}
			}
		case atomStringLanguage:
			requirements = append(requirements, stringlanguage.Requirement{
				Language:  rule.language,
				WantMatch: selectedDemand.wantPass,
			})
		}
	}

	return minimum, maximum, requirements, nil
}

//nolint:cyclop // Required and optional string units have distinct fixed tape rules.
func walkString(
	walk stringlanguage.Walk,
	minimum *big.Int,
	maximum *big.Int,
	tape *tapeCursor,
) (string, bool, error) {
	if walk == nil {
		return "", false, errors.New("walk string with nil walk")
	}

	if tape == nil {
		return "", false, errors.New("walk string with nil tape cursor")
	}

	if minimum == nil {
		minimum = big.NewInt(0)
	}

	if minimum.Sign() < 0 {
		return "", false, errors.New("string minimum is negative")
	}

	if maximum != nil && maximum.Sign() < 0 {
		return "", false, errors.New("string maximum is negative")
	}

	if maximum != nil && minimum.Cmp(maximum) > 0 {
		return "", false, nil
	}

	count := big.NewInt(0)

	var value strings.Builder

	for {
		if maximum != nil && count.Cmp(maximum) >= 0 {
			return value.String(), count.Cmp(minimum) >= 0 && walk.Accepting(), nil
		}

		if count.Cmp(minimum) < 0 {
			complete, err := appendWalkRune(walk, tape, &value)
			if err != nil {
				return "", false, err
			}

			if !complete {
				return "", false, nil
			}

			count.Add(count, big.NewInt(1))

			continue
		}

		if tape.takeByte() == 0 {
			return value.String(), walk.Accepting(), nil
		}

		complete, err := appendWalkRune(walk, tape, &value)
		if err != nil {
			return "", false, err
		}

		if !complete {
			return "", false, nil
		}

		count.Add(count, big.NewInt(1))
	}
}

func appendWalkRune(
	walk stringlanguage.Walk,
	tape *tapeCursor,
	value *strings.Builder,
) (bool, error) {
	ranges := walk.Ranges()
	if len(ranges) == 0 {
		return false, nil
	}

	word := tape.takeWord()
	rangeIndex := word % uint64(len(ranges))
	selected := ranges[rangeIndex]
	width := uint64(selected.Last-selected.First) + 1

	scalar := selected.First + rune((word/uint64(len(ranges)))%width)
	if err := walk.Advance(scalar); err != nil {
		return false, fmt.Errorf("advance generated string rune U+%04X: %w", scalar, err)
	}

	_, err := value.WriteRune(scalar)
	if err != nil {
		return false, err
	}

	return true, nil
}

func countBigInt(number jsonvalue.Number, keyword string) (*big.Int, error) {
	count, err := exactCount(number, keyword)
	if err != nil {
		return nil, err
	}

	if count.Rational != nil {
		if !count.Rational.IsInt() {
			return nil, fmt.Errorf("%s count is not an exact integer", keyword)
		}

		return new(big.Int).Set(count.Rational.Num()), nil
	}

	rational, err := decimalRat(count.Lexeme)
	if err != nil {
		return nil, fmt.Errorf("%s count: %w", keyword, err)
	}

	if !rational.IsInt() {
		return nil, fmt.Errorf("%s count is not an exact integer", keyword)
	}

	return new(big.Int).Set(rational.Num()), nil
}
