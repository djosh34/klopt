//nolint:cyclop,godoclint // Array theory translates signed atoms into one small cursor.
package program

import "github.com/djosh34/klopt/pkg/jsonvalue"

type arrayRules struct {
	cacheKey string
	minimum  uint64
	maximum  uint64
	items    []goal
	faults   []goal
	excluded []jsonvalue.Value
}

func (program *Program) sampleArray(
	goals []goal,
	excluded []jsonvalue.Value,
	reader *tapeReader,
	work *decodeWork,
	depth uint64,
) (jsonvalue.Value, bool, error) {
	rules, possible, err := program.arrayRules(goals, excluded)
	if err != nil || !possible {
		return jsonvalue.Value{}, possible, err
	}

	groups, possible, err := program.chooseArrayFaultGrouping(rules, reader, work)
	if err != nil || !possible {
		return jsonvalue.Value{}, possible, err
	}

	initial := arrayState{pending: groups, relevant: rules.excluded}
	if reader == nil {
		finishPossible, finishErr := program.arrayCanFinish(initial, rules, work)

		return jsonvalue.Array(nil), finishPossible, finishErr
	}

	items, err := program.walkArray(initial, rules, reader, work, depth)
	if err != nil {
		return jsonvalue.Value{}, false, err
	}

	value := jsonvalue.Array(items)

	matches, err := program.valueAllowed(goals, excluded, value)
	if err != nil {
		return jsonvalue.Value{}, false, err
	}

	return value, matches, nil
}

func (program *Program) arrayRules(
	goals []goal,
	excluded []jsonvalue.Value,
) (arrayRules, bool, error) {
	rules := arrayRules{maximum: ^uint64(0)}

	cacheKey, err := stateKey(canonicalStateWithExclusions(goals, excluded))
	if err != nil {
		return arrayRules{}, false, err
	}

	rules.cacheKey = cacheKey

	for _, exact := range excluded {
		if exact.Kind == jsonvalue.KindArray {
			rules.excluded = append(rules.excluded, exact)
		}
	}

	for _, current := range goals {
		item := program.nodes[current.node].atom
		switch item.kind {
		case atomArrayMinItems:
			if current.want {
				rules.minimum = max(rules.minimum, item.count)
			} else if item.count == 0 {
				return arrayRules{}, false, nil
			} else {
				rules.maximum = min(rules.maximum, item.count-1)
			}
		case atomArrayMaxItems:
			if current.want {
				rules.maximum = min(rules.maximum, item.count)
			} else if item.count == ^uint64(0) {
				return arrayRules{}, false, nil
			} else {
				rules.minimum = max(rules.minimum, item.count+1)
			}
		case atomArrayItems:
			child := goal{node: item.child, want: current.want}
			if current.want {
				rules.items = append(rules.items, child)
			} else {
				rules.faults = append(rules.faults, child)
			}
		}
	}

	return rules, rules.minimum <= rules.maximum, nil
}

func (program *Program) chooseArrayFaultGrouping(
	rules arrayRules,
	reader *tapeReader,
	work *decodeWork,
) ([][]goal, bool, error) {
	if len(rules.faults) == 0 {
		possible, err := program.arrayCanFinish(
			arrayState{relevant: rules.excluded}, rules, work,
		)

		return nil, possible, err
	}

	combined := [][]goal{appendCopy(rules.faults)}

	separate := make([][]goal, len(rules.faults))
	for index, fault := range rules.faults {
		separate[index] = []goal{fault}
	}

	preferSeparate := work.fault.budget > 1
	if reader != nil && work.fault.style == faultStructural {
		preferSeparate = true
	}

	first, second := combined, separate
	if preferSeparate {
		first, second = separate, combined
	}

	possible, err := program.arrayCanFinish(
		arrayState{pending: first, relevant: rules.excluded}, rules, work,
	)
	if err != nil {
		return nil, false, err
	}

	if possible {
		return first, true, nil
	}

	if sameGoalGrouping(first, second) {
		return nil, false, nil
	}

	possible, err = program.arrayCanFinish(
		arrayState{pending: second, relevant: rules.excluded}, rules, work,
	)
	if err != nil || !possible {
		return nil, possible, err
	}

	return second, true, nil
}

func sameGoalGrouping(left [][]goal, right [][]goal) bool {
	if len(left) != len(right) {
		return false
	}

	for index := range left {
		if len(left[index]) != len(right[index]) {
			return false
		}

		for goalIndex := range left[index] {
			if left[index][goalIndex] != right[index][goalIndex] {
				return false
			}
		}
	}

	return true
}
