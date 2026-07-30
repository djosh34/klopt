//nolint:cyclop,godoclint // Exact object exclusions are a small forward prefix trie.
package program

import (
	"fmt"
	"slices"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type exactObjectGroup struct {
	value   jsonvalue.Value
	objects []jsonvalue.Value
}

type objectValueChoice struct {
	state     state
	value     jsonvalue.Value
	remaining []jsonvalue.Value
	literal   bool
}

func (program *Program) objectValuesProductive(
	names []string,
	rules *objectRules,
	excluded []jsonvalue.Value,
	work *decodeWork,
) (bool, error) {
	relevant := exactObjectsWithNames(excluded, names)

	return program.objectSuffixProductive(0, names, rules, relevant, work)
}

func (program *Program) objectSuffixProductive(
	index int,
	names []string,
	rules *objectRules,
	relevant []jsonvalue.Value,
	work *decodeWork,
) (bool, error) {
	if index == len(names) {
		return len(relevant) == 0, nil
	}

	if err := work.solver(uint64(len(relevant)) + 1); err != nil {
		return false, err
	}

	nameGoals, allowed := program.objectNameGoals(names[index], rules.faults[names[index]], rules)
	if !allowed {
		return false, nil
	}

	if len(relevant) == 0 {
		return program.objectRegularSuffixProductive(index, names, rules, work)
	}

	exactGroups := groupExactObjectValues(relevant, names[index])

	exactValues := make([]jsonvalue.Value, len(exactGroups))
	for exactIndex, group := range exactGroups {
		exactValues[exactIndex] = group.value
	}

	outside, err := program.productive(
		canonicalStateWithExclusions(nameGoals, exactValues), work,
	)
	if err != nil {
		return false, err
	}

	if outside {
		suffix, suffixErr := program.objectRegularSuffixProductive(index+1, names, rules, work)
		if suffixErr != nil {
			return false, suffixErr
		}

		if suffix {
			return true, nil
		}
	}

	for _, group := range exactGroups {
		matches, matchErr := program.valueMatchesGoals(nameGoals, group.value)
		if matchErr != nil {
			return false, matchErr
		}

		if !matches {
			continue
		}

		possible, possibleErr := program.objectSuffixProductive(
			index+1, names, rules, group.objects, work,
		)
		if possibleErr != nil {
			return false, possibleErr
		}

		if possible {
			return true, nil
		}
	}

	return false, nil
}

func (program *Program) objectRegularSuffixProductive(
	index int,
	names []string,
	rules *objectRules,
	work *decodeWork,
) (bool, error) {
	for current := index; current < len(names); current++ {
		nameGoals, allowed := program.objectNameGoals(
			names[current], rules.faults[names[current]], rules,
		)
		if !allowed {
			return false, nil
		}

		possible, err := program.productive(canonicalState(nameGoals), work)
		if err != nil || !possible {
			return possible, err
		}
	}

	return true, nil
}

func (program *Program) decodeObjectMembers(
	names []string,
	rules *objectRules,
	excluded []jsonvalue.Value,
	reader *tapeReader,
	work *decodeWork,
	depth uint64,
) ([]jsonvalue.Member, error) {
	relevant := exactObjectsWithNames(excluded, names)
	members := make([]jsonvalue.Member, len(names))

	for index, name := range names {
		nameGoals, allowed := program.objectNameGoals(name, rules.faults[name], rules)
		if !allowed {
			return nil, fmt.Errorf("productive object selected forbidden name %q", name)
		}

		if len(relevant) == 0 {
			value, possible, err := program.decodeState(
				canonicalState(nameGoals), reader, work, depth+1,
			)
			if err != nil {
				return nil, err
			}

			if !possible {
				return nil, fmt.Errorf("productive object property %q has no completion", name)
			}

			members[index] = jsonvalue.Member{Name: name, Value: value}

			continue
		}

		choices, err := program.objectValueChoices(index, names, nameGoals, rules, relevant, work)
		if err != nil {
			return nil, err
		}

		if len(choices) == 0 {
			return nil, fmt.Errorf("productive object exclusion has no value choice for %q", name)
		}

		selected := 0
		if reader != nil {
			selected = int(reader.word() % uint64(len(choices)))
		}

		choice := choices[selected]

		value := choice.value
		if !choice.literal {
			var possible bool

			value, possible, err = program.decodeState(choice.state, reader, work, depth+1)
			if err != nil {
				return nil, err
			}

			if !possible {
				return nil, fmt.Errorf("productive object exclusion cannot finish for %q", name)
			}
		}

		members[index] = jsonvalue.Member{Name: name, Value: value}
		relevant = choice.remaining
	}

	if len(relevant) != 0 {
		return nil, fmt.Errorf("object completed as an excluded exact value")
	}

	return members, nil
}

func (program *Program) objectValueChoices(
	index int,
	names []string,
	nameGoals []goal,
	rules *objectRules,
	relevant []jsonvalue.Value,
	work *decodeWork,
) ([]objectValueChoice, error) {
	exactGroups := groupExactObjectValues(relevant, names[index])

	exactValues := make([]jsonvalue.Value, len(exactGroups))
	for exactIndex, group := range exactGroups {
		exactValues[exactIndex] = group.value
	}

	choices := make([]objectValueChoice, 0, len(exactGroups)+1)
	outsideState := canonicalStateWithExclusions(nameGoals, exactValues)

	outside, err := program.productive(outsideState, work)
	if err != nil {
		return nil, err
	}

	if outside {
		suffix, suffixErr := program.objectRegularSuffixProductive(index+1, names, rules, work)
		if suffixErr != nil {
			return nil, suffixErr
		}

		if suffix {
			choices = append(choices, objectValueChoice{state: outsideState})
		}
	}

	for _, group := range exactGroups {
		matches, matchErr := program.valueMatchesGoals(nameGoals, group.value)
		if matchErr != nil {
			return nil, matchErr
		}

		if !matches {
			continue
		}

		possible, possibleErr := program.objectSuffixProductive(
			index+1, names, rules, group.objects, work,
		)
		if possibleErr != nil {
			return nil, possibleErr
		}

		if possible {
			choices = append(choices, objectValueChoice{
				value: group.value, remaining: group.objects, literal: true,
			})
		}
	}

	return choices, nil
}

func exactObjectsWithNames(excluded []jsonvalue.Value, names []string) []jsonvalue.Value {
	result := make([]jsonvalue.Value, 0)

	for _, exact := range excluded {
		if exact.Kind != jsonvalue.KindObject || len(exact.Object) != len(names) {
			continue
		}

		matches := true

		for _, name := range names {
			if _, present := objectMember(exact.Object, name); !present {
				matches = false

				break
			}
		}

		if matches {
			result = append(result, exact)
		}
	}

	return result
}

func groupExactObjectValues(exact []jsonvalue.Value, name string) []exactObjectGroup {
	result := make([]exactObjectGroup, 0)

	for _, object := range exact {
		value, present := objectMember(object.Object, name)
		if !present {
			continue
		}

		groupIndex := slices.IndexFunc(result, func(group exactObjectGroup) bool {
			return group.value.Equal(value)
		})
		if groupIndex < 0 {
			result = append(result, exactObjectGroup{
				value: value, objects: []jsonvalue.Value{object},
			})

			continue
		}

		result[groupIndex].objects = append(result[groupIndex].objects, object)
	}

	slices.SortFunc(result, func(left exactObjectGroup, right exactObjectGroup) int {
		return compareExactValues(left.value, right.value)
	})

	return result
}
