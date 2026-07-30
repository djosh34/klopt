//nolint:cyclop,godoclint,mnd // Exact exclusions are one explicit prefix-state machine.
package program

import (
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type arrayState struct {
	index    uint64
	pending  [][]goal
	relevant []jsonvalue.Value
}

type arrayAction struct {
	stop     bool
	item     state
	literal  *jsonvalue.Value
	next     arrayState
	weight   uint64
	faulting bool
}

func (program *Program) arrayCanFinish(
	current arrayState,
	rules arrayRules,
	work *decodeWork,
) (bool, error) {
	if program.arrayCanStop(current, rules) {
		return true, nil
	}

	if current.index >= rules.maximum {
		return false, nil
	}

	if len(current.relevant) == 0 {
		return program.arrayRegularCompletion(current, rules, work)
	}

	key, err := arrayStateKey(current, rules, work)
	if err != nil {
		return false, err
	}

	if work.arrayKnown[key] {
		return work.arrayProductive[key], nil
	}

	actions, err := program.arrayAppendCandidates(current, rules, work)
	if err != nil {
		return false, err
	}

	return memoizedAnyProductive(
		key,
		work.arrayKnown,
		work.arrayProductive,
		actions,
		func(action arrayAction) (bool, error) {
			return program.arrayCanFinish(action.next, rules, work)
		},
	)
}

func (program *Program) arrayRegularCompletion(
	current arrayState,
	rules arrayRules,
	work *decodeWork,
) (bool, error) {
	needed := uint64(len(current.pending))

	minimumSlots := uint64(0)
	if current.index < rules.minimum {
		minimumSlots = rules.minimum - current.index
	}

	slots := max(needed, minimumSlots)

	end, ok := checkedAdd(current.index, slots)
	if !ok || end > rules.maximum {
		return false, nil
	}

	for _, fault := range current.pending {
		possible, err := program.productive(canonicalState(appendCopy(rules.items, fault...)), work)
		if err != nil || !possible {
			return possible, err
		}
	}

	if slots > needed {
		possible, err := program.productive(canonicalState(rules.items), work)
		if err != nil || !possible {
			return possible, err
		}
	}

	return true, nil
}

func (program *Program) arrayCanStop(current arrayState, rules arrayRules) bool {
	if current.index < rules.minimum || current.index > rules.maximum || len(current.pending) != 0 {
		return false
	}

	for _, exact := range current.relevant {
		if uint64(len(exact.Array)) == current.index {
			return false
		}
	}

	return true
}

func (program *Program) arrayImmediate(
	current arrayState,
	rules arrayRules,
	work *decodeWork,
) ([]arrayAction, error) {
	actions := make([]arrayAction, 0)
	if program.arrayCanStop(current, rules) {
		actions = append(actions, arrayAction{stop: true, weight: 8})
	}

	if current.index >= rules.maximum {
		return actions, nil
	}

	appendActions, err := program.arrayAppendCandidates(current, rules, work)
	if err != nil {
		return nil, err
	}

	for _, action := range appendActions {
		possible, possibleErr := program.arrayCanFinish(action.next, rules, work)
		if possibleErr != nil {
			return nil, possibleErr
		}

		if possible {
			actions = append(actions, action)
		}
	}

	return actions, nil
}

func (program *Program) arrayAppendCandidates(
	current arrayState,
	rules arrayRules,
	work *decodeWork,
) ([]arrayAction, error) {
	type itemOption struct {
		goals    []goal
		pending  [][]goal
		weight   uint64
		faulting bool
	}

	options := make([]itemOption, 0, len(current.pending))
	for index, fault := range current.pending {
		options = append(options, itemOption{
			goals:   appendCopy(rules.items, fault...),
			pending: removeGoalGroup(current.pending, index),
			weight:  8, faulting: true,
		})
	}

	options = append(options, itemOption{
		goals: rules.items, pending: current.pending, weight: 2,
	})

	result := make([]arrayAction, 0)

	for _, option := range options {
		choices, err := program.arrayItemActions(current, option.goals, work)
		if err != nil {
			return nil, err
		}

		for _, choice := range choices {
			choice.next.pending = option.pending
			choice.weight = option.weight
			choice.faulting = option.faulting
			result = append(result, choice)
		}
	}

	return result, nil
}

func (program *Program) arrayItemActions(
	current arrayState,
	itemGoals []goal,
	work *decodeWork,
) ([]arrayAction, error) {
	groups := groupArrayValuesAt(current.relevant, current.index)

	exactValues := make([]jsonvalue.Value, len(groups))
	for index, group := range groups {
		exactValues[index] = group.value
	}

	nextIndex, ok := checkedAdd(current.index, 1)
	if !ok {
		return nil, &LimitError{
			Resource: "array items", Limit: ^uint64(0), Observed: ^uint64(0),
		}
	}

	result := make([]arrayAction, 0, len(groups))
	outside := canonicalStateWithExclusions(itemGoals, exactValues)

	possible, err := program.productive(outside, work)
	if err != nil {
		return nil, err
	}

	if possible {
		result = append(result, arrayAction{
			item: outside, next: arrayState{index: nextIndex},
		})
	}

	for _, group := range groups {
		matches, matchErr := program.valueMatchesGoals(itemGoals, group.value)
		if matchErr != nil {
			return nil, matchErr
		}

		if !matches {
			continue
		}

		value := group.value
		result = append(result, arrayAction{
			literal: &value,
			next:    arrayState{index: nextIndex, relevant: group.arrays},
		})
	}

	return result, nil
}

type exactArrayGroup struct {
	value  jsonvalue.Value
	arrays []jsonvalue.Value
}

func groupArrayValuesAt(exact []jsonvalue.Value, index uint64) []exactArrayGroup {
	result := make([]exactArrayGroup, 0)

	for _, array := range exact {
		if uint64(len(array.Array)) <= index {
			continue
		}

		value := array.Array[index]

		groupIndex := slices.IndexFunc(result, func(group exactArrayGroup) bool {
			return group.value.Equal(value)
		})
		if groupIndex < 0 {
			result = append(result, exactArrayGroup{
				value: value, arrays: []jsonvalue.Value{array},
			})

			continue
		}

		result[groupIndex].arrays = append(result[groupIndex].arrays, array)
	}

	slices.SortFunc(result, func(left exactArrayGroup, right exactArrayGroup) int {
		return compareExactValues(left.value, right.value)
	})

	return result
}

func removeGoalGroup(source [][]goal, remove int) [][]goal {
	result := make([][]goal, 0, len(source)-1)
	result = append(result, source[:remove]...)
	result = append(result, source[remove+1:]...)

	return result
}

func arrayStateKey(current arrayState, rules arrayRules, work *decodeWork) (string, error) {
	estimated, ok := checkedAdd(24, uint64(len(rules.cacheKey)))
	if !ok {
		return "", &ResourceError{
			Resource: "array state bytes", Limit: work.limits.MaxSolverBytes,
			Observed: ^uint64(0),
		}
	}

	pendingBytes, ok := checkedMul(uint64(len(current.pending)), 8)
	if !ok {
		return "", &ResourceError{
			Resource: "array state bytes", Limit: work.limits.MaxSolverBytes,
			Observed: ^uint64(0),
		}
	}

	estimated, ok = checkedAdd(estimated, pendingBytes)
	if !ok {
		return "", &ResourceError{
			Resource: "array state bytes", Limit: work.limits.MaxSolverBytes,
			Observed: ^uint64(0),
		}
	}

	for _, group := range current.pending {
		groupBytes, groupOK := checkedMul(uint64(len(group)), 5)
		if !groupOK {
			return "", &ResourceError{
				Resource: "array state bytes", Limit: work.limits.MaxSolverBytes,
				Observed: ^uint64(0),
			}
		}

		amount, amountOK := checkedAdd(estimated, groupBytes)
		if !amountOK {
			return "", &ResourceError{
				Resource: "array state bytes", Limit: work.limits.MaxSolverBytes,
				Observed: ^uint64(0),
			}
		}

		estimated = amount
	}

	for _, exact := range current.relevant {
		size, err := exactValueBytes(exact)
		if err != nil {
			return "", err
		}

		amount, ok := checkedAdd(estimated, size)
		if !ok {
			return "", &ResourceError{
				Resource: "array state bytes", Limit: work.limits.MaxSolverBytes,
				Observed: ^uint64(0),
			}
		}

		estimated = amount
	}

	if err := work.chargeSolver("array states", "array state bytes", 1, estimated); err != nil {
		return "", err
	}

	encoded := appendBytes(nil, []byte(rules.cacheKey))
	encoded = binary.AppendUvarint(encoded, current.index)

	encoded = binary.AppendUvarint(encoded, uint64(len(current.pending)))
	for _, group := range current.pending {
		encoded = binary.AppendUvarint(encoded, uint64(len(group)))
		for _, currentGoal := range group {
			encoded = binary.AppendUvarint(encoded, uint64(currentGoal.node))
			if currentGoal.want {
				encoded = append(encoded, 1)
			} else {
				encoded = append(encoded, 0)
			}
		}
	}

	for _, exact := range current.relevant {
		raw, err := exact.MarshalJSON()
		if err != nil {
			return "", fmt.Errorf("encode array state: %w", err)
		}

		encoded = appendBytes(encoded, raw)
	}

	return string(encoded), nil
}
