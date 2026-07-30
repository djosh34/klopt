//nolint:cyclop,godoclint,mnd // Array emission is one forward action loop.
package program

import (
	"fmt"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

func (program *Program) walkArray(
	current arrayState,
	rules arrayRules,
	reader *tapeReader,
	work *decodeWork,
	depth uint64,
) ([]jsonvalue.Value, error) {
	items := make([]jsonvalue.Value, 0)
	outputBytes := uint64(2)

	for {
		if err := work.step(); err != nil {
			return nil, err
		}

		actions, err := program.arrayImmediate(current, rules, work)
		if err != nil {
			return nil, err
		}

		if len(actions) == 0 {
			return nil, fmt.Errorf("productive array state has no immediate action")
		}

		selected := actions[weightedArrayAction(reader.word(), actions)]
		if selected.stop {
			return items, nil
		}

		var value jsonvalue.Value
		if selected.literal != nil {
			value = *selected.literal
		} else {
			var possible bool

			value, possible, err = program.decodeState(selected.item, reader, work, depth+1)
			if err != nil {
				return nil, err
			}

			if !possible {
				return nil, fmt.Errorf("productive array item has no completion")
			}
		}

		encoded, err := value.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("measure array item: %w", err)
		}

		addition := uint64(len(encoded))
		if len(items) != 0 {
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

		items = append(items, value)
		current = selected.next
	}
}

func weightedArrayAction(word uint64, actions []arrayAction) int {
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

	panic("array action weights must be positive")
}
