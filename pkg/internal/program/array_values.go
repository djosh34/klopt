//nolint:godoclint // Forward array emission stays private to array theory.
package program

import (
	"fmt"
	"slices"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type exactArrayGroup struct {
	value  jsonvalue.Value
	arrays []jsonvalue.Value
}

type arrayItemChoice struct {
	state     state
	value     jsonvalue.Value
	remaining []jsonvalue.Value
	literal   bool
}

func (program *Program) decodeArrayItems(
	count uint64,
	positive []goal,
	groups [][]goal,
	excluded []jsonvalue.Value,
	reader *tapeReader,
	work *decodeWork,
	depth uint64,
) ([]jsonvalue.Value, error) {
	relevant := make([]jsonvalue.Value, 0)

	for _, exact := range excluded {
		if exact.Kind == jsonvalue.KindArray && uint64(len(exact.Array)) == count {
			relevant = append(relevant, exact)
		}
	}

	items := make([]jsonvalue.Value, int(count))
	for index := range items {
		value, remaining, err := program.decodeArrayItem(
			uint64(index), count, positive, groups, relevant, reader, work, depth,
		)
		if err != nil {
			return nil, err
		}

		items[index] = value
		relevant = remaining
	}

	if len(relevant) != 0 {
		return nil, fmt.Errorf("array completed as an excluded exact value")
	}

	return items, nil
}

func (program *Program) decodeArrayItem(
	index uint64,
	count uint64,
	positive []goal,
	groups [][]goal,
	relevant []jsonvalue.Value,
	reader *tapeReader,
	work *decodeWork,
	depth uint64,
) (jsonvalue.Value, []jsonvalue.Value, error) {
	itemGoals := arrayItemGoals(index, positive, groups)
	if len(relevant) == 0 {
		value, possible, err := program.decodeState(
			canonicalState(itemGoals), reader, work, depth+1,
		)
		if err != nil {
			return jsonvalue.Value{}, nil, err
		}

		if !possible {
			return jsonvalue.Value{}, nil, fmt.Errorf("productive array item has no completion")
		}

		return value, nil, nil
	}

	choices, err := program.arrayItemChoices(
		index, count, itemGoals, positive, groups, relevant, work,
	)
	if err != nil {
		return jsonvalue.Value{}, nil, err
	}

	if len(choices) == 0 {
		return jsonvalue.Value{}, nil, fmt.Errorf("productive array exclusion has no item choice")
	}

	selected := 0
	if reader != nil {
		selected = int(reader.word() % uint64(len(choices)))
	}

	value, err := program.decodeArrayItemChoice(choices[selected], reader, work, depth)
	if err != nil {
		return jsonvalue.Value{}, nil, err
	}

	return value, choices[selected].remaining, nil
}

func (program *Program) decodeArrayItemChoice(
	choice arrayItemChoice,
	reader *tapeReader,
	work *decodeWork,
	depth uint64,
) (jsonvalue.Value, error) {
	if choice.literal {
		return choice.value, nil
	}

	value, possible, err := program.decodeState(choice.state, reader, work, depth+1)
	if err != nil {
		return jsonvalue.Value{}, err
	}

	if !possible {
		return jsonvalue.Value{}, fmt.Errorf("productive array exclusion cannot finish")
	}

	return value, nil
}

func (program *Program) arrayItemChoices(
	index uint64,
	count uint64,
	itemGoals []goal,
	positive []goal,
	groups [][]goal,
	relevant []jsonvalue.Value,
	work *decodeWork,
) ([]arrayItemChoice, error) {
	exactGroups := groupExactArrayItems(relevant, int(index))

	exactItems := make([]jsonvalue.Value, len(exactGroups))
	for exactIndex, group := range exactGroups {
		exactItems[exactIndex] = group.value
	}

	choices := make([]arrayItemChoice, 0, len(exactGroups)+1)

	outside, err := program.arrayOutsideItemChoice(
		index, count, itemGoals, positive, groups, exactItems, work,
	)
	if err != nil {
		return nil, err
	}

	if outside != nil {
		choices = append(choices, *outside)
	}

	exactChoices, err := program.arrayExactItemChoices(
		index, count, itemGoals, positive, groups, exactGroups, work,
	)
	if err != nil {
		return nil, err
	}

	return append(choices, exactChoices...), nil
}

func (program *Program) arrayOutsideItemChoice(
	index uint64,
	count uint64,
	itemGoals []goal,
	positive []goal,
	groups [][]goal,
	exactItems []jsonvalue.Value,
	work *decodeWork,
) (*arrayItemChoice, error) {
	outsideState := canonicalStateWithExclusions(itemGoals, exactItems)

	outside, err := program.productive(outsideState, work)
	if err != nil || !outside {
		return nil, err
	}

	suffix, err := program.arrayRegularSuffixProductive(index+1, count, positive, groups, work)
	if err != nil || !suffix {
		return nil, err
	}

	return &arrayItemChoice{state: outsideState}, nil
}

func (program *Program) arrayExactItemChoices(
	index uint64,
	count uint64,
	itemGoals []goal,
	positive []goal,
	groups [][]goal,
	exactGroups []exactArrayGroup,
	work *decodeWork,
) ([]arrayItemChoice, error) {
	choices := make([]arrayItemChoice, 0, len(exactGroups))
	for _, group := range exactGroups {
		matches, matchErr := program.valueMatchesGoals(itemGoals, group.value)
		if matchErr != nil {
			return nil, matchErr
		}

		if !matches {
			continue
		}

		possible, possibleErr := program.arraySuffixProductive(
			index+1, count, positive, groups, group.arrays, work,
		)
		if possibleErr != nil {
			return nil, possibleErr
		}

		if possible {
			choices = append(choices, arrayItemChoice{
				value: group.value, remaining: group.arrays, literal: true,
			})
		}
	}

	return choices, nil
}

func arrayItemGoals(index uint64, positive []goal, groups [][]goal) []goal {
	result := appendCopy(positive)
	if index < uint64(len(groups)) {
		result = append(result, groups[index]...)
	}

	return result
}

func groupExactArrayItems(exact []jsonvalue.Value, index int) []exactArrayGroup {
	result := make([]exactArrayGroup, 0)

	for _, array := range exact {
		item := array.Array[index]

		groupIndex := slices.IndexFunc(result, func(group exactArrayGroup) bool {
			return group.value.Equal(item)
		})
		if groupIndex < 0 {
			result = append(result, exactArrayGroup{value: item, arrays: []jsonvalue.Value{array}})

			continue
		}

		result[groupIndex].arrays = append(result[groupIndex].arrays, array)
	}

	slices.SortFunc(result, func(left exactArrayGroup, right exactArrayGroup) int {
		return compareExactValues(left.value, right.value)
	})

	return result
}
