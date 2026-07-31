//nolint:godoclint // Private array construction is covered by semantic tests.
package testgenerator

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

//nolint:cyclop,gocognit // Array count and repetition rules are one fixed traversal.
func buildArray(selected []demand, tape *tapeCursor) buildResult {
	if tape == nil {
		return failedBuild(errors.New("build array with nil tape cursor"))
	}

	enumValues, hasEnum, err := selectedEnumValues(selected)
	if err != nil {
		return failedBuild(err)
	}

	if hasEnum {
		arrays := make([]jsonvalue.Value, 0, len(enumValues))
		for _, value := range enumValues {
			if value.Kind == jsonvalue.KindArray {
				arrays = append(arrays, value)
			}
		}

		if len(arrays) == 0 {
			return missedBuild()
		}

		candidate := arrays[0]
		if len(arrays) > 1 {
			candidate = arrays[tape.takeWord()%uint64(len(arrays))]
		}

		return buildSelectedValue(selected, candidate)
	}

	minimum, maximum, item, negativeItem, err := collectArrayDomain(selected)
	if err != nil {
		return failedBuild(err)
	}

	if maximum != nil && (maximum.Sign() < 0 || minimum.Cmp(maximum) > 0) {
		return missedBuild()
	}

	values := make([]jsonvalue.Value, 0)

	for count := big.NewInt(0); count.Cmp(minimum) < 0; count.Add(count, big.NewInt(1)) {
		if negativeItem {
			return missedBuild()
		}

		built := buildPositiveChild(item, tape)
		if built.err != nil || built.state != buildComplete {
			return built
		}

		values = append(values, built.value)
	}

	for maximum == nil || big.NewInt(int64(len(values))).Cmp(maximum) < 0 {
		if tape.takeByte() == 0 {
			break
		}

		if negativeItem {
			return missedBuild()
		}

		built := buildPositiveChild(item, tape)
		if built.err != nil || built.state != buildComplete {
			return built
		}

		values = append(values, built.value)
	}

	return buildSelectedValue(selected, jsonvalue.Array(values))
}

//nolint:cyclop,gocognit // Array count constraints map directly to one fixed domain.
func collectArrayDomain(selected []demand) (*big.Int, *big.Int, *expression, bool, error) {
	minimum := big.NewInt(0)

	var (
		maximum *big.Int
		item    *expression
	)

	negativeItem := false

	for index, selectedDemand := range selected {
		if selectedDemand.expression == nil || selectedDemand.expression.kind != expressionAtom {
			return nil, nil, nil, false, fmt.Errorf("array demand %d is not an atom", index)
		}

		rule := selectedDemand.expression.atom
		switch rule.kind {
		case atomArrayMinItems:
			count, err := countBigInt(rule.count, "minItems")
			if err != nil {
				return nil, nil, nil, false, err
			}

			if selectedDemand.wantPass {
				if count.Cmp(minimum) > 0 {
					minimum = count
				}
			} else {
				upper := new(big.Int).Sub(count, big.NewInt(1))
				if maximum == nil || upper.Cmp(maximum) < 0 {
					maximum = upper
				}
			}
		case atomArrayMaxItems:
			count, err := countBigInt(rule.count, "maxItems")
			if err != nil {
				return nil, nil, nil, false, err
			}

			if selectedDemand.wantPass {
				if maximum == nil || count.Cmp(maximum) < 0 {
					maximum = count
				}
			} else {
				lower := new(big.Int).Add(count, big.NewInt(1))
				if lower.Cmp(minimum) > 0 {
					minimum = lower
				}
			}
		case atomArrayItems:
			if item != nil && selectedDemand.wantPass {
				return nil, nil, nil, false, errors.New("multiple array item atoms")
			}

			if selectedDemand.wantPass {
				item = rule.child
			} else {
				negativeItem = true
			}
		}
	}

	return minimum, maximum, item, negativeItem, nil
}

func buildPositiveChild(child *expression, tape *tapeCursor) buildResult {
	if child == nil {
		return completeBuild(jsonvalue.Null())
	}

	selected := make([]demand, 0)
	if err := selectPassingDemands(child, tape, &selected); err != nil {
		return failedBuild(err)
	}

	built := buildValue(selected, tape)
	if built.err != nil || built.state != buildComplete {
		return built
	}

	holds, err := demandsHold(selected, built.value)
	if err != nil {
		return failedBuild(fmt.Errorf("check child demands: %w", err))
	}

	if !holds {
		return missedBuild()
	}

	return built
}
