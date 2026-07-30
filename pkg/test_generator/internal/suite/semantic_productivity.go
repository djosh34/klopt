//nolint:godoclint // Private theory-productivity vocabulary is local to exact emptiness.
package suite

import (
	"math/big"
	"slices"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type countInterval struct {
	minimum *big.Int
	maximum *big.Int
}

type objectValueRule struct {
	declaredNames []string
	values        SetRef
}

func appendObjectValueRules(
	arena *SetArena,
	assignment map[AtomID]bool,
) ([]objectValueRule, bool) {
	rules := make([]objectValueRule, 0)
	complete := true

	for identifier, want := range assignment {
		switch value := arena.Atoms[identifier].(type) {
		case additionalPropertyValuesAtom:
			if !want {
				complete = false

				continue
			}

			rules = append(rules, objectValueRule{
				declaredNames: value.Names,
				values:        value.Values,
			})
		case additionalSomePropertyAtom:
			if want {
				complete = false

				continue
			}

			rules = append(rules, objectValueRule{
				declaredNames: value.Names,
				values:        Complement(value.Values),
			})
		}
	}

	return rules, complete
}

func objectValueConstraints(
	direct []SetRef,
	name string,
	rules []objectValueRule,
) []SetRef {
	result := append([]SetRef(nil), direct...)

	for _, rule := range rules {
		if !slices.Contains(rule.declaredNames, name) {
			result = append(result, rule.values)
		}
	}

	return result
}

//nolint:cyclop // Signed array facts are normalized into count, all-item, and some-item facts.
func (arena *SetArena) arrayAssignmentProductive(assignment map[AtomID]bool) (bool, error) {
	lower, upper, excluded, exact := collectionCountFacts(arena, assignment, true)
	if !exact {
		return true, nil
	}

	allowed := arena.True()
	some := make([]SetRef, 0)

	for identifier, want := range assignment {
		switch value := arena.Atoms[identifier].(type) {
		case arrayItemsAtom:
			if want {
				var err error

				allowed, err = arena.Intersect(allowed, value.Values)
				if err != nil {
					return false, err
				}
			} else {
				some = append(some, Complement(value.Values))
			}
		case arraySomeItemsAtom:
			if want {
				some = append(some, value.Values)
			} else {
				var err error

				allowed, err = arena.Intersect(allowed, Complement(value.Values))
				if err != nil {
					return false, err
				}
			}
		}
	}

	allowedEmpty, err := arena.IsEmpty(allowed)
	if err != nil {
		return false, err
	}

	minimumItems := big.NewInt(0)
	if lower != nil && lower.Sign() > 0 {
		minimumItems.Set(lower)
	}

	if allowedEmpty {
		if len(some) != 0 {
			return false, nil
		}

		upper = minimumBigInt(upper, big.NewInt(0))
	} else if len(some) != 0 {
		bins, binsErr := arena.minimumItemBins(allowed, some)
		if binsErr != nil {
			return false, binsErr
		}

		if bins < 0 {
			return false, nil
		}

		minimumItems = maximumBigInt(minimumItems, big.NewInt(int64(bins)))
	}

	return countExists(minimumItems, upper, excluded), nil
}

//nolint:cyclop // Exact set-compatible bin search avoids value or route enumeration.
func (arena *SetArena) minimumItemBins(allowed SetRef, requirements []SetRef) (int, error) {
	best := len(requirements) + 1

	var search func(int, []SetRef) error

	search = func(index int, bins []SetRef) error {
		if len(bins) >= best {
			return nil
		}

		if index == len(requirements) {
			best = len(bins)

			return nil
		}

		for binIndex := range bins {
			combined, err := arena.Intersect(bins[binIndex], requirements[index])
			if err != nil {
				return err
			}

			empty, err := arena.IsEmpty(combined)
			if err != nil {
				return err
			}

			if empty {
				continue
			}

			next := append([]SetRef(nil), bins...)

			next[binIndex] = combined
			if err := search(index+1, next); err != nil {
				return err
			}
		}

		newBin, err := arena.Intersect(allowed, requirements[index])
		if err != nil {
			return err
		}

		empty, err := arena.IsEmpty(newBin)
		if err != nil {
			return err
		}

		if !empty {
			return search(index+1, append(append([]SetRef(nil), bins...), newBin))
		}

		return nil
	}

	if err := search(0, nil); err != nil {
		return 0, err
	}

	if best > len(requirements) {
		return -1, nil
	}

	return best, nil
}

//nolint:cyclop,gocognit // Signed object name, value, and count facts are proved together.
func (arena *SetArena) objectAssignmentProductive(assignment map[AtomID]bool) (bool, error) {
	lower, upper, excluded, exact := collectionCountFacts(arena, assignment, false)
	if !exact {
		return true, nil
	}

	required := make(map[string]struct{})
	absent := make(map[string]struct{})
	constraints := make(map[string][]SetRef)

	var allowed []string

	hasAllowed := false

	additionalRules, _ := appendObjectValueRules(arena, assignment)

	for identifier, want := range assignment {
		switch value := arena.Atoms[identifier].(type) {
		case requiredPropertyAtom:
			if want {
				required[value.Name] = struct{}{}
			} else {
				absent[value.Name] = struct{}{}
			}
		case propertyValuesAtom:
			if want {
				constraints[value.Name] = append(constraints[value.Name], value.Values)
			} else {
				required[value.Name] = struct{}{}
				constraints[value.Name] = append(
					constraints[value.Name], Complement(value.Values),
				)
			}
		case allowedPropertyNamesAtom:
			if want {
				if !hasAllowed {
					allowed = append([]string(nil), value.Names...)
					hasAllowed = true
				} else {
					allowed = intersectNames(allowed, value.Names)
				}
			}
		}
	}

	for name := range required {
		if _, forbidden := absent[name]; forbidden || hasAllowed && !slices.Contains(allowed, name) {
			return false, nil
		}

		values, err := arena.Intersect(objectValueConstraints(constraints[name], name, additionalRules)...)
		if err != nil {
			return false, err
		}

		empty, err := arena.IsEmpty(values)
		if err != nil {
			return false, err
		}

		if empty {
			return false, nil
		}
	}

	minimum := big.NewInt(int64(len(required)))
	if lower != nil && lower.Cmp(minimum) > 0 {
		minimum = new(big.Int).Set(lower)
	}

	if hasAllowed {
		upper = minimumBigInt(upper, big.NewInt(int64(len(allowed))))
	}

	return countExists(minimum, upper, excluded), nil
}

func collectionCountFacts(
	arena *SetArena,
	assignment map[AtomID]bool,
	array bool,
) (*big.Int, *big.Int, []countInterval, bool) {
	lower := big.NewInt(0)

	var upper *big.Int

	excluded := make([]countInterval, 0)

	for identifier, want := range assignment {
		var (
			minimumNumber *jsonvalue.Number
			maximumNumber *jsonvalue.Number
		)

		switch value := arena.Atoms[identifier].(type) {
		case arrayLengthAtom:
			if !array {
				continue
			}

			minimumNumber, maximumNumber = value.Minimum, value.Maximum
		case objectCountAtom:
			if array {
				continue
			}

			minimumNumber, maximumNumber = value.Minimum, value.Maximum
		default:
			continue
		}

		minimum, ok := countBigInt(minimumNumber)
		if !ok {
			return nil, nil, nil, false
		}

		maximum, ok := countBigInt(maximumNumber)
		if !ok {
			return nil, nil, nil, false
		}

		if want {
			lower = maximumBigInt(lower, minimum)
			upper = minimumBigInt(upper, maximum)
		} else {
			excluded = append(excluded, countInterval{minimum: minimum, maximum: maximum})
		}
	}

	return lower, upper, excluded, true
}

func countBigInt(value *jsonvalue.Number) (*big.Int, bool) {
	if value == nil {
		return nil, true
	}

	if value.Rational == nil || !value.Rational.IsInt() {
		return nil, false
	}

	return new(big.Int).Set(value.Rational.Num()), true
}

//nolint:cyclop // Exact interval subtraction advances across sorted excluded intervals.
func countExists(lower *big.Int, upper *big.Int, excluded []countInterval) bool {
	if upper != nil && lower.Cmp(upper) > 0 {
		return false
	}

	slices.SortFunc(excluded, func(left countInterval, right countInterval) int {
		if left.minimum == nil {
			return -1
		}

		if right.minimum == nil {
			return 1
		}

		return left.minimum.Cmp(right.minimum)
	})

	candidate := new(big.Int).Set(lower)
	for _, interval := range excluded {
		if interval.minimum != nil && candidate.Cmp(interval.minimum) < 0 {
			break
		}

		if interval.maximum == nil {
			return false
		}

		if candidate.Cmp(interval.maximum) <= 0 {
			candidate.Add(interval.maximum, big.NewInt(1))
		}

		if upper != nil && candidate.Cmp(upper) > 0 {
			return false
		}
	}

	return upper == nil || candidate.Cmp(upper) <= 0
}

func maximumBigInt(left *big.Int, right *big.Int) *big.Int {
	if left == nil {
		if right == nil {
			return nil
		}

		return new(big.Int).Set(right)
	}

	if right == nil || left.Cmp(right) >= 0 {
		return new(big.Int).Set(left)
	}

	return new(big.Int).Set(right)
}

func minimumBigInt(left *big.Int, right *big.Int) *big.Int {
	if left == nil {
		if right == nil {
			return nil
		}

		return new(big.Int).Set(right)
	}

	if right == nil || left.Cmp(right) <= 0 {
		return new(big.Int).Set(left)
	}

	return new(big.Int).Set(right)
}

func intersectNames(left []string, right []string) []string {
	result := make([]string, 0)

	for _, name := range left {
		if slices.Contains(right, name) {
			result = append(result, name)
		}
	}

	return result
}
