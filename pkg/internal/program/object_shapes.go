//nolint:cyclop,godoclint,mnd // Object expansion exposes stop or one canonical property.
package program

import (
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type objectState struct {
	count           uint64
	previous        string
	hasPrevious     bool
	remainingForced []string
	relevant        []jsonvalue.Value
}

type objectAction struct {
	stop    bool
	dynamic bool
	name    string
	value   state
	literal *jsonvalue.Value
	next    objectState
	weight  uint64
}

func (program *Program) objectCanFinish(
	current objectState,
	rules *objectRules,
	work *decodeWork,
) (bool, error) {
	if program.objectCanStop(current, rules) {
		return true, nil
	}

	if current.count >= rules.maximum {
		return false, nil
	}

	if len(current.relevant) == 0 {
		return program.objectRegularCompletion(current, rules, work)
	}

	key, err := objectStateKey(current, rules, work)
	if err != nil {
		return false, err
	}

	if work.objectKnown[key] {
		return work.objectProductive[key], nil
	}

	actions, err := program.objectAddCandidates(current, rules, work)
	if err != nil {
		return false, err
	}

	return memoizedAnyProductive(
		key,
		work.objectKnown,
		work.objectProductive,
		actions,
		func(action objectAction) (bool, error) {
			return program.objectCanFinish(action.next, rules, work)
		},
	)
}

func (program *Program) objectCanStop(current objectState, rules *objectRules) bool {
	if current.count < rules.minimum || current.count > rules.maximum ||
		len(current.remainingForced) != 0 {
		return false
	}

	for _, exact := range current.relevant {
		if uint64(len(exact.Object)) == current.count {
			return false
		}
	}

	return true
}

func (program *Program) objectRegularCompletion(
	current objectState,
	rules *objectRules,
	work *decodeWork,
) (bool, error) {
	forcedCount := uint64(len(current.remainingForced))

	minimumEnd, ok := checkedAdd(current.count, forcedCount)
	if !ok {
		return false, nil
	}

	minimumEnd = max(minimumEnd, rules.minimum)
	if minimumEnd > rules.maximum {
		return false, nil
	}

	for _, name := range current.remainingForced {
		possible, err := program.objectNamePossible(name, rules.faults[name], rules, work)
		if err != nil || !possible {
			return possible, err
		}
	}

	extraNeeded := minimumEnd - current.count - forcedCount
	if extraNeeded == 0 {
		return true, nil
	}

	optional, infinite, err := program.objectOptionalCapacity(current, rules, work)
	if err != nil {
		return false, err
	}

	return infinite || optional >= extraNeeded, nil
}

func (program *Program) objectImmediate(
	current objectState,
	rules *objectRules,
	work *decodeWork,
) ([]objectAction, error) {
	actions := make([]objectAction, 0)
	if program.objectCanStop(current, rules) {
		actions = append(actions, objectAction{stop: true, weight: 8})
	}

	if current.count >= rules.maximum {
		return actions, nil
	}

	additions, err := program.objectAddCandidates(current, rules, work)
	if err != nil {
		return nil, err
	}

	for _, action := range additions {
		possible, possibleErr := program.objectCanFinish(action.next, rules, work)
		if possibleErr != nil {
			return nil, possibleErr
		}

		if possible {
			actions = append(actions, action)
		}
	}

	return actions, nil
}

func (program *Program) objectAddCandidates(
	current objectState,
	rules *objectRules,
	work *decodeWork,
) ([]objectAction, error) {
	names, err := program.objectImmediateNames(current, rules, work)
	if err != nil {
		return nil, err
	}

	result := make([]objectAction, 0)

	for _, name := range names {
		if name.dynamic {
			valueActions, valueErr := program.objectValueActions(
				current, name.name, rules, work,
			)
			if valueErr != nil {
				return nil, valueErr
			}

			productive := false

			for _, action := range valueActions {
				possible, possibleErr := program.objectCanFinish(action.next, rules, work)
				if possibleErr != nil {
					return nil, possibleErr
				}

				if possible {
					productive = true

					break
				}
			}

			if productive {
				result = append(result, objectAction{
					dynamic: true, name: name.name, weight: name.weight,
				})
			}

			continue
		}

		valueActions, valueErr := program.objectValueActions(current, name.name, rules, work)
		if valueErr != nil {
			return nil, valueErr
		}

		for _, action := range valueActions {
			action.weight = name.weight
			result = append(result, action)
		}
	}

	return result, nil
}

type objectNameChoice struct {
	name    string
	weight  uint64
	dynamic bool
}

func (program *Program) objectImmediateNames(
	current objectState,
	rules *objectRules,
	work *decodeWork,
) ([]objectNameChoice, error) {
	upper := ""
	if len(current.remainingForced) != 0 {
		upper = current.remainingForced[0]
	}

	known, finite := program.objectCandidateNames(rules)

	result := make([]objectNameChoice, 0, len(known))
	for _, name := range known {
		if current.hasPrevious && compareShortlex(name, current.previous) <= 0 {
			continue
		}

		if upper != "" && compareShortlex(name, upper) > 0 {
			continue
		}

		if _, absent := rules.absent[name]; absent {
			continue
		}

		possible, err := program.objectNamePossible(name, rules.faults[name], rules, work)
		if err != nil {
			return nil, err
		}

		if !possible {
			continue
		}

		weight := uint64(2)
		if upper == name {
			weight = 8
		}

		result = append(result, objectNameChoice{name: name, weight: weight})
	}

	if !finite {
		name, possible, err := program.chooseDynamicObjectName(
			current.previous, current.hasPrevious, upper, rules, nil, work,
		)
		if err != nil {
			return nil, err
		}

		if possible {
			result = append(result, objectNameChoice{name: name, weight: 2, dynamic: true})
		}
	}

	slices.SortStableFunc(result, func(left objectNameChoice, right objectNameChoice) int {
		return compareShortlex(left.name, right.name)
	})

	return result, nil
}

func (program *Program) objectValueActions(
	current objectState,
	name string,
	rules *objectRules,
	work *decodeWork,
) ([]objectAction, error) {
	goals, allowed := program.objectNameGoals(name, rules.faults[name], rules)
	if !allowed {
		return nil, nil
	}

	groups := groupObjectValuesAt(current.relevant, current.count, name)

	exactValues := make([]jsonvalue.Value, len(groups))
	for index, group := range groups {
		exactValues[index] = group.value
	}

	nextCount, ok := checkedAdd(current.count, 1)
	if !ok {
		return nil, &LimitError{
			Resource: "object properties", Limit: ^uint64(0), Observed: ^uint64(0),
		}
	}

	next := objectState{
		count: nextCount, previous: name, hasPrevious: true,
		remainingForced: removeForcedObjectName(current.remainingForced, name),
	}

	result := make([]objectAction, 0, len(groups))
	outside := canonicalStateWithExclusions(goals, exactValues)

	possible, err := program.productive(outside, work)
	if err != nil {
		return nil, err
	}

	if possible {
		result = append(result, objectAction{name: name, value: outside, next: next})
	}

	for _, group := range groups {
		matches, matchErr := program.valueMatchesGoals(goals, group.value)
		if matchErr != nil {
			return nil, matchErr
		}

		if !matches {
			continue
		}

		value := group.value
		exactNext := next
		exactNext.relevant = group.objects
		result = append(result, objectAction{
			name: name, literal: &value, next: exactNext,
		})
	}

	return result, nil
}

func removeForcedObjectName(forced []string, name string) []string {
	if len(forced) != 0 && forced[0] == name {
		return forced[1:]
	}

	return forced
}

func objectStateKey(current objectState, rules *objectRules, work *decodeWork) (string, error) {
	estimated, ok := checkedAdd(32, uint64(len(rules.cacheKey)))
	if !ok {
		return "", &ResourceError{
			Resource: "object state bytes", Limit: work.limits.MaxSolverBytes,
			Observed: ^uint64(0),
		}
	}

	estimated, ok = checkedAdd(estimated, uint64(len(current.previous)))
	if !ok {
		return "", &ResourceError{
			Resource: "object state bytes", Limit: work.limits.MaxSolverBytes,
			Observed: ^uint64(0),
		}
	}

	for _, name := range current.remainingForced {
		nameBytes, nameOK := checkedAdd(uint64(len(name)), 8)
		if !nameOK {
			return "", &ResourceError{
				Resource: "object state bytes", Limit: work.limits.MaxSolverBytes,
				Observed: ^uint64(0),
			}
		}

		amount, ok := checkedAdd(estimated, nameBytes)
		if !ok {
			return "", &ResourceError{
				Resource: "object state bytes", Limit: work.limits.MaxSolverBytes,
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
				Resource: "object state bytes", Limit: work.limits.MaxSolverBytes,
				Observed: ^uint64(0),
			}
		}

		estimated = amount
	}

	if err := work.chargeSolver("object states", "object state bytes", 1, estimated); err != nil {
		return "", err
	}

	encoded := appendBytes(nil, []byte(rules.cacheKey))
	encoded = binary.AppendUvarint(encoded, current.count)

	encoded = appendBytes(encoded, []byte(current.previous))
	if current.hasPrevious {
		encoded = append(encoded, 1)
	} else {
		encoded = append(encoded, 0)
	}

	for _, name := range current.remainingForced {
		encoded = appendBytes(encoded, []byte(name))
	}

	for _, exact := range current.relevant {
		raw, err := exact.MarshalJSON()
		if err != nil {
			return "", fmt.Errorf("encode object state: %w", err)
		}

		encoded = appendBytes(encoded, raw)
	}

	return string(encoded), nil
}
