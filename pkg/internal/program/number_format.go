//nolint:godoclint,mnd // Private format predicates and near faults stay in one small theory helper.
package program

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

func (program *Program) directFormatFaults(
	goals []goal,
	excluded []jsonvalue.Value,
) ([]jsonvalue.Number, error) {
	lexemes := []string{
		"0.1",
		"2147483648",
		"-2147483649",
		"9223372036854775808",
		"-9223372036854775809",
		"1" + strings.Repeat("0", 39),
		"-1" + strings.Repeat("0", 39),
		"1" + strings.Repeat("0", 309),
		"-1" + strings.Repeat("0", 309),
	}
	result := make([]jsonvalue.Number, 0, len(lexemes))

	for _, lexeme := range lexemes {
		number, err := jsonvalue.ParseNumber(lexeme)
		if err != nil {
			return nil, fmt.Errorf("parse internal format fault %q: %w", lexeme, err)
		}

		matches, err := program.valueAllowed(goals, excluded, jsonvalue.Value{
			Kind: jsonvalue.KindNumber, Number: number,
		})
		if err != nil {
			return nil, err
		}

		if matches {
			result = append(result, number)
		}
	}

	return result, nil
}

func numberMatchesFormat(number jsonvalue.Number, format string) bool {
	switch format {
	case "int32":
		return number.IsInteger() &&
			number.Compare(mustInternalNumber("-2147483648")) >= 0 &&
			number.Compare(mustInternalNumber("2147483647")) <= 0
	case "int64":
		return number.IsInteger() &&
			number.Compare(mustInternalNumber("-9223372036854775808")) >= 0 &&
			number.Compare(mustInternalNumber("9223372036854775807")) <= 0
	case "float":
		_, err := strconv.ParseFloat(number.Lexeme, 32)

		return err == nil
	case "double":
		_, err := strconv.ParseFloat(number.Lexeme, 64)

		return err == nil
	default:
		return false
	}
}

func mustInternalNumber(value string) jsonvalue.Number {
	number, err := jsonvalue.ParseNumber(value)
	if err != nil {
		panic(err)
	}

	return number
}
