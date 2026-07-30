//nolint:godoclint // Container residual construction stays behind graph lowering.
package suite

import (
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

//nolint:cyclop,gocognit // Signed array atoms normalize to all-item and outstanding some-item residuals.
func (finder *valueFinder) arrayValues(assignment map[AtomID]bool) ([]jsonvalue.Value, error) {
	allowed := finder.arena.True()
	requirements := make([]SetRef, 0)

	for identifier, want := range assignment {
		switch value := finder.arena.Atoms[identifier].(type) {
		case arrayItemsAtom:
			if want {
				var err error

				allowed, err = finder.arena.Intersect(allowed, value.Values)
				if err != nil {
					return nil, err
				}
			} else {
				requirements = append(requirements, Complement(value.Values))
			}
		case arraySomeItemsAtom:
			if want {
				requirements = append(requirements, value.Values)
			} else {
				var err error

				allowed, err = finder.arena.Intersect(allowed, Complement(value.Values))
				if err != nil {
					return nil, err
				}
			}
		}
	}

	required := make([]jsonvalue.Value, 0, len(requirements))
	for _, requirement := range requirements {
		combined, err := finder.arena.Intersect(allowed, requirement)
		if err != nil {
			return nil, err
		}

		value, err := finder.find(combined)
		if err != nil {
			return nil, err
		}

		required = append(required, value)
	}

	counts := finder.countCandidates(assignment, true, len(required))

	result := make([]jsonvalue.Value, 0, len(counts))
	for _, count := range counts {
		if count < len(required) || !finder.countAssignmentMatches(assignment, true, count) {
			continue
		}

		items := append([]jsonvalue.Value(nil), required...)
		if count > len(items) {
			value, err := finder.find(allowed)
			if err != nil {
				continue
			}

			for len(items) < count {
				items = append(items, value)
			}
		}

		result = append(result, jsonvalue.Array(items))
	}

	return result, nil
}

//nolint:cyclop,gocognit,gocyclo,nestif,maintidx // Object residuals normalize together.
func (finder *valueFinder) objectValues(assignment map[AtomID]bool) ([]jsonvalue.Value, error) {
	required := make(map[string]struct{})
	forbidden := make(map[string]struct{})
	known := make(map[string]struct{})
	constraints := make(map[string][]SetRef)
	allowedNames := []string(nil)
	hasAllowed := false
	extraRequirements := make([]objectValueRule, 0)
	additionalRules := make([]objectValueRule, 0)
	needForbiddenName := false

	for identifier, want := range assignment {
		switch value := finder.arena.Atoms[identifier].(type) {
		case requiredPropertyAtom:
			known[value.Name] = struct{}{}
			if want {
				required[value.Name] = struct{}{}
			} else {
				forbidden[value.Name] = struct{}{}
			}
		case allowedPropertyNamesAtom:
			for _, name := range value.Names {
				known[name] = struct{}{}
			}

			if want {
				if !hasAllowed {
					allowedNames = append([]string(nil), value.Names...)
					hasAllowed = true
				} else {
					allowedNames = intersectNames(allowedNames, value.Names)
				}
			} else {
				needForbiddenName = true
			}
		case propertyValuesAtom:
			known[value.Name] = struct{}{}
			if want {
				constraints[value.Name] = append(constraints[value.Name], value.Values)
			} else {
				required[value.Name] = struct{}{}
				constraints[value.Name] = append(constraints[value.Name], Complement(value.Values))
			}
		case additionalPropertyValuesAtom:
			for _, name := range value.Names {
				known[name] = struct{}{}
			}

			if want {
				additionalRules = append(additionalRules, objectValueRule{
					declaredNames: value.Names,
					values:        value.Values,
				})
			} else {
				extraRequirements = append(extraRequirements, objectValueRule{
					declaredNames: value.Names,
					values:        Complement(value.Values),
				})
			}
		case additionalSomePropertyAtom:
			for _, name := range value.Names {
				known[name] = struct{}{}
			}

			if want {
				extraRequirements = append(extraRequirements, objectValueRule{
					declaredNames: value.Names,
					values:        value.Values,
				})
			} else {
				additionalRules = append(additionalRules, objectValueRule{
					declaredNames: value.Names,
					values:        Complement(value.Values),
				})
			}
		}
	}

	names := make([]string, 0, len(required)+len(extraRequirements)+1)
	for name := range required {
		if _, absent := forbidden[name]; absent || hasAllowed && !slices.Contains(allowedNames, name) {
			return nil, nil
		}

		names = append(names, name)
	}

	extraName := nextAdditionalName(known)
	if len(extraRequirements) != 0 || needForbiddenName {
		if hasAllowed && !slices.Contains(allowedNames, extraName) {
			if needForbiddenName {
				names = append(names, extraName)
				known[extraName] = struct{}{}
			} else {
				return nil, nil
			}
		} else {
			names = append(names, extraName)
			known[extraName] = struct{}{}
		}
	}

	minimumCount := len(names)
	counts := finder.countCandidates(assignment, false, minimumCount)
	result := make([]jsonvalue.Value, 0, len(counts))

	for _, count := range counts {
		selected := append([]string(nil), names...)
		for _, name := range slices.Sorted(maps.Keys(known)) {
			if len(selected) >= count {
				break
			}

			if _, absent := forbidden[name]; absent || slices.Contains(selected, name) ||
				hasAllowed && !slices.Contains(allowedNames, name) {
				continue
			}

			selected = append(selected, name)
		}

		for len(selected) < count && !hasAllowed {
			name := nextAdditionalName(known)
			known[name] = struct{}{}
			selected = append(selected, name)
		}

		if len(selected) != count || !finder.countAssignmentMatches(assignment, false, count) {
			continue
		}

		sort.Strings(selected)
		members := make([]jsonvalue.Member, 0, len(selected))
		valid := true

		for _, name := range selected {
			refs := objectValueConstraints(constraints[name], name, additionalRules)
			if name == extraName {
				for _, requirement := range extraRequirements {
					if !slices.Contains(requirement.declaredNames, name) {
						refs = append(refs, requirement.values)
					}
				}
			}

			values, err := finder.arena.Intersect(refs...)
			if err != nil {
				return nil, err
			}

			child, err := finder.find(values)
			if err != nil {
				valid = false

				break
			}

			members = append(members, jsonvalue.Member{Name: name, Value: child})
		}

		if !valid {
			continue
		}

		object, err := jsonvalue.Object(members)
		if err != nil {
			return nil, err
		}

		result = append(result, object)
	}

	return result, nil
}

//nolint:cyclop // Array and object count atoms share one exact boundary catalogue.
func (finder *valueFinder) countCandidates(
	assignment map[AtomID]bool,
	array bool,
	minimum int,
) []int {
	result := []int{minimum, 0, 1, 2}

	for identifier := range assignment {
		var bounds []*jsonvalue.Number

		switch value := finder.arena.Atoms[identifier].(type) {
		case arrayLengthAtom:
			if array {
				bounds = []*jsonvalue.Number{value.Minimum, value.Maximum}
			}
		case objectCountAtom:
			if !array {
				bounds = []*jsonvalue.Number{value.Minimum, value.Maximum}
			}
		}

		for _, bound := range bounds {
			if bound == nil || bound.Rational == nil || !bound.Rational.IsInt() || !bound.Rational.Num().IsInt64() {
				continue
			}

			value := int(bound.Rational.Num().Int64())
			for _, candidate := range []int{value - 1, value, value + 1} {
				if candidate >= minimum && candidate >= 0 {
					result = append(result, candidate)
				}
			}
		}
	}

	sort.Ints(result)

	return slices.Compact(result)
}

func (finder *valueFinder) countAssignmentMatches(
	assignment map[AtomID]bool,
	array bool,
	count int,
) bool {
	for identifier, want := range assignment {
		var matches bool

		switch value := finder.arena.Atoms[identifier].(type) {
		case arrayLengthAtom:
			if !array {
				continue
			}

			matches = countMatches(count, value.Minimum, value.Maximum)
		case objectCountAtom:
			if array {
				continue
			}

			matches = countMatches(count, value.Minimum, value.Maximum)
		default:
			continue
		}

		if matches != want {
			return false
		}
	}

	return true
}

func nextAdditionalName(known map[string]struct{}) string {
	for index := 0; ; index++ {
		name := fmt.Sprintf("extra%d", index)
		if _, exists := known[name]; !exists {
			return name
		}
	}
}
