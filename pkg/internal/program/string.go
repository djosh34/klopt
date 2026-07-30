//nolint:cyclop,godoclint,mnd // String theory explicitly maps signed length and language atoms.
package program

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // Unified string theory consumes shared automata.
	"github.com/djosh34/klopt/pkg/jsonvalue"
)

func (program *Program) sampleString(
	goals []goal,
	excluded []jsonvalue.Value,
	reader *tapeReader,
	work *decodeWork,
) (jsonvalue.Value, bool, error) {
	minimum, maximum, requirements, err := program.stringConstraints(goals, excluded)
	if err != nil {
		return jsonvalue.Value{}, false, err
	}

	value, possible, err := walkString(requirements, minimum, maximum, reader, work)

	return jsonvalue.String(value), possible, err
}

func (program *Program) stringConstraints(
	goals []goal,
	excluded []jsonvalue.Value,
) (uint64, uint64, []stringlanguage.Requirement, error) {
	minimum := uint64(0)
	maximum := ^uint64(0)
	requirements := make([]stringlanguage.Requirement, 0)

	for _, current := range goals {
		item := program.nodes[current.node].atom
		switch item.kind {
		case atomStringMinLength:
			minimum, maximum = signedMinimum(minimum, maximum, item.count, current.want)
		case atomStringMaxLength:
			minimum, maximum = signedMaximum(minimum, maximum, item.count, current.want)
		case atomStringLanguage:
			requirements = append(requirements, stringlanguage.Requirement{
				Language: item.language, WantMatch: current.want,
			})
		}
	}

	for _, value := range excluded {
		if value.Kind != jsonvalue.KindString {
			continue
		}

		language, err := stringlanguage.Literal(value.String)
		if err != nil {
			return 0, 0, nil, err
		}

		requirements = append(requirements, stringlanguage.Requirement{
			Language: language, WantMatch: false,
		})
	}

	return minimum, maximum, requirements, nil
}

func signedMinimum(minimum uint64, maximum uint64, count uint64, want bool) (uint64, uint64) {
	if want {
		return max(minimum, count), maximum
	}

	if count == 0 {
		return 1, 0
	}

	return minimum, min(maximum, count-1)
}

func signedMaximum(minimum uint64, maximum uint64, count uint64, want bool) (uint64, uint64) {
	if want {
		return minimum, min(maximum, count)
	}

	if count == ^uint64(0) {
		return 1, 0
	}

	return max(minimum, count+1), maximum
}

func walkString(
	requirements []stringlanguage.Requirement,
	minimum uint64,
	maximum uint64,
	reader *tapeReader,
	work *decodeWork,
) (string, bool, error) {
	set, possible, err := compileStringSet(requirements, minimum, maximum)
	if err != nil || !possible {
		return "", possible, err
	}

	state := set.Start()
	result := make([]rune, 0)
	outputBytes := uint64(2)

	for {
		if err := work.step(); err != nil {
			return "", false, err
		}

		ranges := set.ProductiveRanges(state)
		accepting := set.Accepting(state)

		choice := stringChoice(reader, accepting, len(ranges))
		if accepting && choice == 0 {
			return string(result), true, nil
		}

		if accepting {
			choice--
		}

		if choice < 0 || choice >= len(ranges) {
			return "", false, fmt.Errorf("productive string state has no completion")
		}

		selected := ranges[choice]
		scalar := rangeScalar(selected, reader)

		outputBytes += uint64(utf8.RuneLen(scalar))
		if err := checkLimit("output bytes", work.limits.MaxOutputBytes, outputBytes); err != nil {
			return "", false, err
		}

		result = append(result, scalar)
		state = selected.Next
	}
}

func compileStringSet(
	requirements []stringlanguage.Requirement,
	minimum uint64,
	maximum uint64,
) (*stringlanguage.Set, bool, error) {
	maximumInt := int(^uint(0) >> 1)
	if minimum > uint64(maximumInt) || maximum != ^uint64(0) && maximum > uint64(maximumInt) {
		return nil, false, &ResourceError{
			Resource: "string length", Limit: uint64(maximumInt), Observed: max(minimum, maximum),
		}
	}

	length := stringlanguage.Length{Min: int(minimum)}
	if maximum != ^uint64(0) {
		length.Max = new(int(maximum))
	}

	set, err := stringlanguage.Compile(requirements, length)
	if err == nil {
		return set, true, nil
	}

	var empty *stringlanguage.EmptyError
	if errors.As(err, &empty) {
		return nil, false, nil
	}

	var complexity *stringlanguage.ComplexityError
	if errors.As(err, &complexity) {
		return nil, false, &ResourceError{
			Resource: "string " + complexity.Resource,
			Limit:    complexity.Limit,
			Observed: complexity.Observed,
		}
	}

	return nil, false, fmt.Errorf("compile string constraints: %w", err)
}

func stringChoice(reader *tapeReader, accepting bool, rangeCount int) int {
	choiceCount := rangeCount
	if accepting {
		choiceCount++
	}

	if choiceCount == 0 || reader == nil {
		return 0
	}

	return int(reader.word() % uint64(choiceCount))
}

func rangeScalar(selected stringlanguage.ScalarRange, reader *tapeReader) rune {
	scalar := selected.First
	if reader != nil {
		width := uint64(selected.Last-selected.First) + 1
		scalar += rune(reader.word() % width)
	}

	return scalar
}
