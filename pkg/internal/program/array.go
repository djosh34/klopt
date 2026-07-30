//nolint:cyclop,gocognit,godoclint,mnd // Array obligations are an explicit, private theory dispatch.
package program

import (
	"math/big"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type arrayFaultPlan struct {
	groups  [][]goal
	weight  uint64
	maximum uint64
}

func (program *Program) sampleArray(
	goals []goal,
	excluded []jsonvalue.Value,
	reader *tapeReader,
	work *decodeWork,
	depth uint64,
) (jsonvalue.Value, bool, error) {
	minimum := uint64(0)
	maximum := ^uint64(0)
	positiveItems := make([]goal, 0)
	negativeItems := make([]goal, 0)

	for _, current := range goals {
		item := program.nodes[current.node].atom
		switch item.kind {
		case atomArrayMinItems:
			if current.want {
				minimum = max(minimum, item.count)
			} else if item.count == 0 {
				return jsonvalue.Value{}, false, nil
			} else {
				maximum = min(maximum, item.count-1)
			}
		case atomArrayMaxItems:
			if current.want {
				maximum = min(maximum, item.count)
			} else if item.count == ^uint64(0) {
				return jsonvalue.Value{}, false, nil
			} else {
				minimum = max(minimum, item.count+1)
			}
		case atomArrayItems:
			childGoal := goal{node: item.child, want: current.want}
			if current.want {
				positiveItems = append(positiveItems, childGoal)
			} else {
				negativeItems = append(negativeItems, childGoal)
			}
		}
	}

	if minimum > maximum {
		return jsonvalue.Value{}, false, nil
	}

	plans, err := program.productiveArrayPlans(
		positiveItems, negativeItems, excluded, minimum, maximum, work,
	)
	if err != nil {
		return jsonvalue.Value{}, false, err
	}

	if len(plans) == 0 {
		return jsonvalue.Value{}, false, nil
	}

	selectedPlan := plans[0]
	if reader != nil {
		selectedPlan = plans[weightedArrayPlan(reader.word(), plans)]
	}

	minimum = max(minimum, uint64(len(selectedPlan.groups)))
	maximum = min(maximum, selectedPlan.maximum)

	count, possible, err := program.chooseProductiveArrayCount(
		minimum, maximum, positiveItems, selectedPlan.groups, excluded, reader, work,
	)
	if err != nil {
		return jsonvalue.Value{}, false, err
	}

	if !possible {
		return jsonvalue.Value{}, false, nil
	}

	if count > uint64(int(^uint(0)>>1)) || count+2 > work.limits.MaxOutputBytes {
		return jsonvalue.Value{}, false, &LimitError{
			Resource: "array items", Limit: work.limits.MaxOutputBytes, Observed: count + 2,
		}
	}

	items, err := program.decodeArrayItems(
		count, positiveItems, selectedPlan.groups, excluded, reader, work, depth,
	)
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

func (program *Program) productiveArrayPlans(
	positive []goal,
	negative []goal,
	excluded []jsonvalue.Value,
	minimum uint64,
	maximum uint64,
	work *decodeWork,
) ([]arrayFaultPlan, error) {
	if len(negative) == 0 {
		return program.productiveArrayPlanWithoutFaults(
			positive, excluded, minimum, maximum, work,
		)
	}

	plans := []arrayFaultPlan{
		{groups: [][]goal{appendCopy(negative)}, weight: 8, maximum: maximum},
	}
	if len(negative) > 1 {
		separate := make([][]goal, len(negative))
		for index, current := range negative {
			separate[index] = []goal{current}
		}

		plans = append(plans, arrayFaultPlan{groups: separate, weight: 1, maximum: maximum})
	}

	productivePlans := make([]arrayFaultPlan, 0, len(plans))
	for _, plan := range plans {
		if uint64(len(plan.groups)) > maximum {
			continue
		}

		productive := true

		for _, group := range plan.groups {
			itemGoals := appendCopy(positive, group...)

			possible, err := program.productive(canonicalState(itemGoals), work)
			if err != nil {
				return nil, err
			}

			if !possible {
				productive = false

				break
			}
		}

		if !productive {
			continue
		}

		if max(minimum, uint64(len(plan.groups))) < plan.maximum {
			possible, err := program.productive(canonicalState(positive), work)
			if err != nil {
				return nil, err
			}

			if !possible {
				plan.maximum = uint64(len(plan.groups))
			}
		}

		if max(minimum, uint64(len(plan.groups))) <= plan.maximum {
			_, possible, err := program.chooseProductiveArrayCount(
				max(minimum, uint64(len(plan.groups))),
				plan.maximum,
				positive,
				plan.groups,
				excluded,
				nil,
				work,
			)
			if err != nil {
				return nil, err
			}

			if possible {
				productivePlans = append(productivePlans, plan)
			}
		}
	}

	return productivePlans, nil
}

func (program *Program) productiveArrayPlanWithoutFaults(
	positive []goal,
	excluded []jsonvalue.Value,
	minimum uint64,
	maximum uint64,
	work *decodeWork,
) ([]arrayFaultPlan, error) {
	productive, err := program.productive(canonicalState(positive), work)
	if err != nil {
		return nil, err
	}

	if !productive {
		if minimum == 0 {
			return []arrayFaultPlan{{weight: 1, maximum: 0}}, nil
		}

		return nil, nil
	}

	plan := arrayFaultPlan{weight: 1, maximum: maximum}

	_, possible, err := program.chooseProductiveArrayCount(
		minimum, maximum, positive, plan.groups, excluded, nil, work,
	)
	if err != nil || !possible {
		return nil, err
	}

	return []arrayFaultPlan{plan}, nil
}

func chooseCount(
	minimum uint64,
	maximum uint64,
	reader *tapeReader,
	work *decodeWork,
) (uint64, error) {
	rank, err := readNatural(reader, work)
	if err != nil {
		return 0, err
	}

	if maximum != ^uint64(0) {
		span := new(big.Int).SetUint64(maximum - minimum)
		span.Add(span, big.NewInt(1))
		rank.Mod(rank, span)

		return minimum + rank.Uint64(), nil
	}

	if !rank.IsUint64() || rank.Uint64() > ^uint64(0)-minimum {
		return 0, &LimitError{
			Resource: "container length", Limit: ^uint64(0), Observed: ^uint64(0),
		}
	}

	return minimum + rank.Uint64(), nil
}

func weightedArrayPlan(word uint64, plans []arrayFaultPlan) int {
	total := uint64(0)
	for _, plan := range plans {
		total += plan.weight
	}

	selected := word % total
	for index, plan := range plans {
		if selected < plan.weight {
			return index
		}

		selected -= plan.weight
	}

	panic("array plan weights must be positive")
}
