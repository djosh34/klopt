//nolint:cyclop,godoclint // Shape selection lazily skips only exhausted exact object shapes.
package program

import (
	"fmt"
	"slices"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

const objectContainerBytes = 2

func (program *Program) chooseProductiveObjectNames(
	minimum uint64,
	maximum uint64,
	forced []string,
	available []string,
	finite bool,
	rules *objectRules,
	excluded []jsonvalue.Value,
	reader *tapeReader,
	work *decodeWork,
) ([]string, bool, error) {
	count, err := chooseCount(minimum, maximum, reader, work)
	if err != nil {
		return nil, false, err
	}

	seenShapes := make(map[string]struct{})
	exhaustedCounts := make(map[uint64]struct{})
	maximumShapes := exactKindCount(excluded, jsonvalue.KindObject) + 1

	var names []string
	for len(seenShapes) < maximumShapes {
		if names == nil {
			names, err = program.selectObjectNames(
				count, forced, available, finite, rules, reader, work,
			)
			if err != nil {
				return nil, false, err
			}
		}

		key := objectShapeKey(names)
		if _, seen := seenShapes[key]; seen {
			exhaustedCounts[count] = struct{}{}
			names = nil

			count, err = nextObjectCount(count, minimum, maximum, exhaustedCounts)
			if err != nil {
				return nil, false, err
			}

			if count == ^uint64(0) {
				return nil, false, nil
			}

			continue
		}

		seenShapes[key] = struct{}{}

		possible, possibleErr := program.objectValuesProductive(names, rules, excluded, work)
		if possibleErr != nil {
			return nil, false, possibleErr
		}

		if possible {
			return names, true, nil
		}

		next, nextErr := program.nextObjectNames(names, forced, available, finite, rules, work)
		if nextErr != nil {
			return nil, false, nextErr
		}

		if next != nil {
			names = next

			continue
		}

		exhaustedCounts[count] = struct{}{}
		names = nil

		count, err = nextObjectCount(count, minimum, maximum, exhaustedCounts)
		if err != nil {
			return nil, false, err
		}

		if count == ^uint64(0) {
			return nil, false, nil
		}
	}

	return nil, false, nil
}

func (program *Program) selectObjectNames(
	count uint64,
	forced []string,
	available []string,
	finite bool,
	rules *objectRules,
	reader *tapeReader,
	work *decodeWork,
) ([]string, error) {
	if count > work.limits.MaxOutputBytes ||
		work.limits.MaxOutputBytes-count < objectContainerBytes {
		observed := count
		if count <= ^uint64(0)-objectContainerBytes {
			observed += objectContainerBytes
		}

		return nil, &LimitError{
			Resource: "object properties", Limit: work.limits.MaxOutputBytes, Observed: observed,
		}
	}

	if count > uint64(int(^uint(0)>>1)) {
		return nil, &LimitError{
			Resource: "object properties", Limit: uint64(int(^uint(0) >> 1)), Observed: count,
		}
	}

	optionalCount := int(count) - len(forced)
	if optionalCount < 0 {
		return nil, fmt.Errorf("object count is smaller than its forced name set")
	}

	names := appendCopy(forced)
	if finite {
		names = append(names, chooseFiniteNames(available, optionalCount, reader)...)
	} else {
		selected, err := program.chooseInfiniteNames(optionalCount, rules, reader, work)
		if err != nil {
			return nil, err
		}

		names = append(names, selected...)
	}

	slices.SortFunc(names, compareShortlex)

	return names, nil
}

func (program *Program) nextObjectNames(
	names []string,
	forced []string,
	available []string,
	finite bool,
	rules *objectRules,
	work *decodeWork,
) ([]string, error) {
	optional := make([]string, 0, len(names)-len(forced))
	for _, name := range names {
		if _, required := rules.forced[name]; !required {
			optional = append(optional, name)
		}
	}

	if len(optional) == 0 {
		return nil, nil
	}

	var next []string
	if finite {
		next = nextFiniteNames(optional, available)
	} else {
		var err error

		next, err = program.nextInfiniteNames(optional, rules, work)
		if err != nil {
			return nil, err
		}
	}

	if next == nil {
		return nil, nil
	}

	result := appendCopy(forced)
	result = append(result, next...)
	slices.SortFunc(result, compareShortlex)

	return result, nil
}

func nextFiniteNames(current []string, available []string) []string {
	indices := make([]int, len(current))
	for index, name := range current {
		position, ok := slices.BinarySearch(available, name)
		if !ok {
			return nil
		}

		indices[index] = position
	}

	for index := len(indices) - 1; index >= 0; index-- {
		maximum := len(available) - len(indices) + index
		if indices[index] >= maximum {
			continue
		}

		indices[index]++
		for suffix := index + 1; suffix < len(indices); suffix++ {
			indices[suffix] = indices[suffix-1] + 1
		}

		result := make([]string, len(indices))
		for resultIndex, availableIndex := range indices {
			result[resultIndex] = available[availableIndex]
		}

		return result
	}

	if len(indices) == 0 || len(indices) > len(available) {
		return nil
	}

	result := make([]string, len(indices))
	copy(result, available[:len(indices)])

	return result
}

func nextObjectCount(
	current uint64,
	minimum uint64,
	maximum uint64,
	exhausted map[uint64]struct{},
) (uint64, error) {
	candidate := current
	for range len(exhausted) + 1 {
		if maximum == ^uint64(0) {
			if candidate == ^uint64(0) {
				return 0, &LimitError{
					Resource: "object properties", Limit: ^uint64(0), Observed: ^uint64(0),
				}
			}

			candidate++
		} else if candidate == maximum {
			candidate = minimum
		} else {
			candidate++
		}

		if _, alreadyExhausted := exhausted[candidate]; !alreadyExhausted {
			return candidate, nil
		}
	}

	return ^uint64(0), nil
}

func objectShapeKey(names []string) string {
	encoded := appendUint64(nil, uint64(len(names)))
	for _, name := range names {
		encoded = appendBytes(encoded, []byte(name))
	}

	return string(encoded)
}
