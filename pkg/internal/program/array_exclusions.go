//nolint:godoclint // Exact array exclusion productivity stays private to array theory.
package program

import "github.com/djosh34/klopt/pkg/jsonvalue"

const arrayCountSolverBytes = 16

func (program *Program) chooseProductiveArrayCount(
	minimum uint64,
	maximum uint64,
	positive []goal,
	groups [][]goal,
	excluded []jsonvalue.Value,
	reader *tapeReader,
	work *decodeWork,
) (uint64, bool, error) {
	count, err := chooseCount(minimum, maximum, reader, work)
	if err != nil {
		return 0, false, err
	}

	start := count
	maximumAttempts := exactKindCount(excluded, jsonvalue.KindArray) + 1

	for attempts := 0; attempts < maximumAttempts; attempts++ {
		possible, possibleErr := program.arrayItemsProductive(
			count, positive, groups, excluded, work,
		)
		if possibleErr != nil {
			return 0, false, possibleErr
		}

		if possible {
			return count, true, nil
		}

		if err := work.solver(arrayCountSolverBytes); err != nil {
			return 0, false, err
		}

		if maximum == ^uint64(0) {
			if count == ^uint64(0) {
				return 0, false, &LimitError{
					Resource: "array items", Limit: ^uint64(0), Observed: ^uint64(0),
				}
			}

			count++

			continue
		}

		if count == maximum {
			count = minimum
		} else {
			count++
		}

		if count == start {
			return 0, false, nil
		}
	}

	return 0, false, nil
}

func (program *Program) arrayItemsProductive(
	count uint64,
	positive []goal,
	groups [][]goal,
	excluded []jsonvalue.Value,
	work *decodeWork,
) (bool, error) {
	relevant := make([]jsonvalue.Value, 0)

	for _, exact := range excluded {
		if exact.Kind == jsonvalue.KindArray && uint64(len(exact.Array)) == count {
			relevant = append(relevant, exact)
		}
	}

	return program.arraySuffixProductive(0, count, positive, groups, relevant, work)
}

func (program *Program) arraySuffixProductive(
	index uint64,
	count uint64,
	positive []goal,
	groups [][]goal,
	relevant []jsonvalue.Value,
	work *decodeWork,
) (bool, error) {
	if index == count {
		return len(relevant) == 0, nil
	}

	if err := work.solver(uint64(len(relevant)) + 1); err != nil {
		return false, err
	}

	itemGoals := arrayItemGoals(index, positive, groups)
	if len(relevant) == 0 {
		return program.arrayRegularSuffixProductive(index, count, positive, groups, work)
	}

	exactGroups := groupExactArrayItems(relevant, int(index))

	outside, err := program.arrayOutsidePrefixProductive(
		index, count, itemGoals, positive, groups, exactGroups, work,
	)
	if err != nil {
		return false, err
	}

	if outside {
		return true, nil
	}

	return program.arrayExactPrefixProductive(
		index, count, itemGoals, positive, groups, exactGroups, work,
	)
}

func (program *Program) arrayOutsidePrefixProductive(
	index uint64,
	count uint64,
	itemGoals []goal,
	positive []goal,
	groups [][]goal,
	exactGroups []exactArrayGroup,
	work *decodeWork,
) (bool, error) {
	exactItems := make([]jsonvalue.Value, len(exactGroups))
	for exactIndex, group := range exactGroups {
		exactItems[exactIndex] = group.value
	}

	outside, err := program.productive(
		canonicalStateWithExclusions(itemGoals, exactItems), work,
	)
	if err != nil || !outside {
		return outside, err
	}

	return program.arrayRegularSuffixProductive(index+1, count, positive, groups, work)
}

func (program *Program) arrayExactPrefixProductive(
	index uint64,
	count uint64,
	itemGoals []goal,
	positive []goal,
	groups [][]goal,
	exactGroups []exactArrayGroup,
	work *decodeWork,
) (bool, error) {
	for _, group := range exactGroups {
		matches, matchErr := program.valueMatchesGoals(itemGoals, group.value)
		if matchErr != nil {
			return false, matchErr
		}

		if !matches {
			continue
		}

		possible, possibleErr := program.arraySuffixProductive(
			index+1, count, positive, groups, group.arrays, work,
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

func (program *Program) arrayRegularSuffixProductive(
	index uint64,
	count uint64,
	positive []goal,
	groups [][]goal,
	work *decodeWork,
) (bool, error) {
	for current := index; current < count; current++ {
		possible, err := program.productive(
			canonicalState(arrayItemGoals(current, positive, groups)), work,
		)
		if err != nil || !possible {
			return possible, err
		}
	}

	return true, nil
}
