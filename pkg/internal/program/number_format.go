//nolint:cyclop,godoclint,mnd // Format boundaries are direct theory transitions.
package program

import (
	"fmt"
	"strconv"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

func (program *Program) sampleFormatFault(
	goals []goal,
	excluded []jsonvalue.Value,
	reader *tapeReader,
) (jsonvalue.Number, bool, error) {
	formats := make([]string, 0)

	for _, current := range goals {
		item := program.nodes[current.node].atom
		if item.kind == atomNumberFormat && !current.want {
			formats = append(formats, item.text)
		}
	}

	if len(formats) == 0 {
		return jsonvalue.Number{}, false, nil
	}

	start := 0
	mode := 0

	if reader != nil {
		start = int(reader.word() % uint64(len(formats)))
		mode = int(reader.word() % 3)
	}

	for formatOffset := range formats {
		format := formats[(start+formatOffset)%len(formats)]
		for modeOffset := range 3 {
			lexeme, ok := formatFaultLexeme(format, (mode+modeOffset)%3)
			if !ok {
				continue
			}

			number, err := jsonvalue.ParseNumber(lexeme)
			if err != nil {
				return jsonvalue.Number{}, false, fmt.Errorf(
					"parse generated %s format fault %q: %w", format, lexeme, err,
				)
			}

			matches, err := program.valueAllowed(goals, excluded, jsonvalue.Value{
				Kind: jsonvalue.KindNumber, Number: number,
			})
			if err != nil {
				return jsonvalue.Number{}, false, err
			}

			if matches {
				return number, true, nil
			}
		}
	}

	return jsonvalue.Number{}, false, nil
}

func formatFaultLexeme(format string, mode int) (string, bool) {
	switch format {
	case "int32":
		switch mode {
		case 0:
			return "2147483648", true
		case 1:
			return "-2147483649", true
		default:
			return "0.1", true
		}
	case "int64":
		switch mode {
		case 0:
			return "9223372036854775808", true
		case 1:
			return "-9223372036854775809", true
		default:
			return "0.1", true
		}
	case "float":
		if mode == 1 {
			return "-1e39", true
		}

		return "1e39", true
	case "double":
		if mode == 1 {
			return "-1e309", true
		}

		return "1e309", true
	default:
		return "", false
	}
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
