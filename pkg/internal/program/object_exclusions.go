//nolint:cyclop,gocognit,godoclint,mnd // Object emission follows one canonical prefix state.
package program

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type exactObjectGroup struct {
	value   jsonvalue.Value
	objects []jsonvalue.Value
}

func groupObjectValuesAt(
	exact []jsonvalue.Value,
	index uint64,
	name string,
) []exactObjectGroup {
	result := make([]exactObjectGroup, 0)

	for _, object := range exact {
		members := canonicalObjectMembers(object.Object)
		if uint64(len(members)) <= index || members[index].Name != name {
			continue
		}

		value := members[index].Value

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

func canonicalObjectMembers(source []jsonvalue.Member) []jsonvalue.Member {
	result := slices.Clone(source)
	slices.SortFunc(result, func(left jsonvalue.Member, right jsonvalue.Member) int {
		return compareShortlex(left.Name, right.Name)
	})

	return result
}

func (program *Program) walkObject(
	current objectState,
	rules *objectRules,
	reader *tapeReader,
	work *decodeWork,
	depth uint64,
) ([]jsonvalue.Member, error) {
	members := make([]jsonvalue.Member, 0)
	outputBytes := uint64(2)

	for {
		if err := work.step(); err != nil {
			return nil, err
		}

		actions, err := program.objectImmediate(current, rules, work)
		if err != nil {
			return nil, err
		}

		if len(actions) == 0 {
			return nil, fmt.Errorf("productive object state has no immediate action")
		}

		selected := actions[weightedObjectAction(reader.word(), actions)]
		if selected.stop {
			return members, nil
		}

		if selected.dynamic {
			selected, err = program.materializeDynamicObjectAction(
				current, rules, reader, work, selected.name,
			)
			if err != nil {
				return nil, err
			}
		}

		var value jsonvalue.Value
		if selected.literal != nil {
			value = *selected.literal
		} else {
			var possible bool

			value, possible, err = program.decodeState(selected.value, reader, work, depth+1)
			if err != nil {
				return nil, err
			}

			if !possible {
				return nil, fmt.Errorf("productive object property %q has no completion", selected.name)
			}
		}

		nameJSON, err := json.Marshal(selected.name)
		if err != nil {
			return nil, fmt.Errorf("measure object property name: %w", err)
		}

		valueJSON, err := value.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("measure object property value: %w", err)
		}

		addition := uint64(len(nameJSON) + 1 + len(valueJSON))
		if len(members) != 0 {
			addition++
		}

		observed, ok := checkedAdd(outputBytes, addition)
		if !ok {
			return nil, &LimitError{
				Resource: "output bytes", Limit: work.limits.MaxOutputBytes,
				Observed: ^uint64(0),
			}
		}

		if err := checkLimit("output bytes", work.limits.MaxOutputBytes, observed); err != nil {
			return nil, err
		}

		outputBytes = observed

		members = append(members, jsonvalue.Member{Name: selected.name, Value: value})
		current = selected.next
	}
}

func (program *Program) materializeDynamicObjectAction(
	current objectState,
	rules *objectRules,
	reader *tapeReader,
	work *decodeWork,
	fallback string,
) (objectAction, error) {
	upper := ""
	if len(current.remainingForced) != 0 {
		upper = current.remainingForced[0]
	}

	name, possible, err := program.chooseDynamicObjectName(
		current.previous, current.hasPrevious, upper, rules, reader, work,
	)
	if err != nil {
		return objectAction{}, err
	}

	if !possible {
		name = fallback
	}

	actions, err := program.objectValueActions(current, name, rules, work)
	if err != nil {
		return objectAction{}, err
	}

	productive := make([]objectAction, 0, len(actions))
	for _, action := range actions {
		canFinish, finishErr := program.objectCanFinish(action.next, rules, work)
		if finishErr != nil {
			return objectAction{}, finishErr
		}

		if canFinish {
			action.weight = 1
			productive = append(productive, action)
		}
	}

	if len(productive) == 0 && name != fallback {
		actions, err = program.objectValueActions(current, fallback, rules, work)
		if err != nil {
			return objectAction{}, err
		}

		for _, action := range actions {
			canFinish, finishErr := program.objectCanFinish(action.next, rules, work)
			if finishErr != nil {
				return objectAction{}, finishErr
			}

			if canFinish {
				action.weight = 1
				productive = append(productive, action)
			}
		}
	}

	if len(productive) == 0 {
		return objectAction{}, fmt.Errorf("productive dynamic property has no completion")
	}

	return productive[reader.word()%uint64(len(productive))], nil
}

func weightedObjectAction(word uint64, actions []objectAction) int {
	total := uint64(0)
	for _, action := range actions {
		total += action.weight
	}

	selected := word % total
	for index, action := range actions {
		if selected < action.weight {
			return index
		}

		selected -= action.weight
	}

	panic("object action weights must be positive")
}
