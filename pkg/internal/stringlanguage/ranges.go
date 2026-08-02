//nolint:godoclint // Private canonical range vocabulary stays inside stringlanguage.
package stringlanguage

import "slices"

const (
	maximumCodeUnit = 0xffff
	maximumScalar   = 0x10ffff
	firstSurrogate  = 0xd800
	lastSurrogate   = 0xdfff
)

type runeRange struct {
	first rune
	last  rune
}

type runeSet []runeRange

var scalarUniverse = runeSet{
	{first: 0, last: firstSurrogate - 1},
	{first: lastSurrogate + 1, last: maximumScalar},
}

func codeUnitUniverse() runeSet {
	return runeSet{{first: 0, last: maximumCodeUnit}}
}

func (set runeSet) contains(value rune) bool {
	_, ok := slices.BinarySearchFunc(set, value, func(item runeRange, target rune) int {
		switch {
		case item.last < target:
			return -1
		case item.first > target:
			return 1
		default:
			return 0
		}
	})

	return ok
}

func normalizeRuneSet(set runeSet) runeSet {
	if len(set) == 0 {
		return nil
	}

	set = slices.Clone(set)
	slices.SortFunc(set, func(left runeRange, right runeRange) int {
		if left.first != right.first {
			return int(left.first - right.first)
		}

		return int(left.last - right.last)
	})

	result := set[:0]
	for _, candidate := range set {
		if len(result) == 0 || candidate.first > result[len(result)-1].last+1 {
			result = append(result, candidate)

			continue
		}

		result[len(result)-1].last = max(result[len(result)-1].last, candidate.last)
	}

	return result
}

func unionRuneSets(sets ...runeSet) runeSet {
	var combined runeSet
	for _, set := range sets {
		combined = append(combined, set...)
	}

	return normalizeRuneSet(combined)
}

func complementRuneSet(set runeSet, universe runeSet) runeSet {
	set = normalizeRuneSet(set)
	result := make(runeSet, 0, len(set)+len(universe))

	for _, allowed := range universe {
		next := allowed.first
		for _, excluded := range set {
			if excluded.last < next || excluded.first > allowed.last {
				continue
			}

			if next < excluded.first {
				result = append(result, runeRange{first: next, last: excluded.first - 1})
			}

			if excluded.last >= allowed.last {
				next = allowed.last + 1

				break
			}

			next = excluded.last + 1
		}

		if next <= allowed.last {
			result = append(result, runeRange{first: next, last: allowed.last})
		}
	}

	return result
}

func intersectRuneSets(left runeSet, right runeSet) runeSet {
	result := make(runeSet, 0)
	leftIndex := 0
	rightIndex := 0

	for leftIndex < len(left) && rightIndex < len(right) {
		first := max(left[leftIndex].first, right[rightIndex].first)

		last := min(left[leftIndex].last, right[rightIndex].last)
		if first <= last {
			result = append(result, runeRange{first: first, last: last})
		}

		if left[leftIndex].last < right[rightIndex].last {
			leftIndex++
		} else {
			rightIndex++
		}
	}

	return result
}
