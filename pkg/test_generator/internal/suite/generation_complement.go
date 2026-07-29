//nolint:godoclint // The finite-complement helpers form one private implementation unit.
package suite

import (
	"errors"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

const maximumFiniteComplementValues = 10_000

func finiteDomainValues(registry *DomainRegistry, id DomainID) ([]jsonvalue.Value, bool, error) {
	return finiteDomainValuesAt(registry, id, make(map[DomainID]bool))
}

//nolint:cyclop // Every JSON kind has a distinct finiteness proof.
func finiteDomainValuesAt(
	registry *DomainRegistry,
	id DomainID,
	active map[DomainID]bool,
) ([]jsonvalue.Value, bool, error) {
	domain, ok := registry.Domain(id)
	if !ok {
		return nil, false, errors.New("finite complement Domain does not exist")
	}

	if domain.Status == DomainEmpty {
		return nil, true, nil
	}

	if domain.Enum != nil {
		return cloneJSONValues(domain.Enum.Values), true, nil
	}

	if active[id] {
		return nil, false, nil
	}

	active[id] = true
	defer delete(active, id)

	values := make([]jsonvalue.Value, 0)
	if domain.Null != KindExcluded {
		values = append(values, jsonvalue.Null())
	}

	if domain.Boolean != KindExcluded {
		values = append(values, jsonvalue.Bool(false), jsonvalue.Bool(true))
	}

	if domain.Number.State != KindExcluded {
		if !exactSingletonInterval(domain.Number) {
			return nil, false, nil
		}

		values = append(values, jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: domain.Number.Minimum.Value})
	}

	if domain.String.State != KindExcluded {
		return nil, false, nil
	}

	if domain.Array.State != KindExcluded {
		arrays, finite, err := finiteArrayValues(registry, domain.Array, active)
		if err != nil || !finite {
			return nil, finite, err
		}

		values = append(values, arrays...)
	}

	if domain.Object.State != KindExcluded {
		objects, finite, err := finiteObjectValues(registry, domain.Object, active)
		if err != nil || !finite {
			return nil, finite, err
		}

		values = append(values, objects...)
	}

	if len(values) > maximumFiniteComplementValues {
		return nil, false, nil
	}

	return values, true, nil
}

func finiteArrayValues(
	registry *DomainRegistry,
	constraints ArrayConstraints,
	active map[DomainID]bool,
) ([]jsonvalue.Value, bool, error) {
	if constraints.MaxItems == nil {
		return nil, false, nil
	}

	items, finite, err := finiteDomainValuesAt(registry, constraints.Items, active)
	if err != nil || !finite {
		return nil, finite, err
	}

	result := make([]jsonvalue.Value, 0)
	for count := constraints.MinItems; count <= *constraints.MaxItems; count++ {
		arrays, complete := finiteArrayProducts(items, count, maximumFiniteComplementValues-len(result))
		if !complete {
			return nil, false, nil
		}

		result = append(result, arrays...)
	}

	return result, true, nil
}

func finiteArrayProducts(items []jsonvalue.Value, count int, limit int) ([]jsonvalue.Value, bool) {
	if count == 0 {
		return []jsonvalue.Value{jsonvalue.Array(nil)}, limit > 0
	}

	if len(items) == 0 || limit <= 0 {
		return nil, len(items) == 0
	}

	indexes := make([]int, count)

	result := make([]jsonvalue.Value, 0)
	for {
		if len(result) == limit {
			return nil, false
		}

		values := make([]jsonvalue.Value, count)
		for index, item := range indexes {
			values[index] = items[item]
		}

		result = append(result, jsonvalue.Array(values))

		position := count - 1
		for position >= 0 {
			indexes[position]++
			if indexes[position] < len(items) {
				break
			}

			indexes[position] = 0
			position--
		}

		if position < 0 {
			return result, true
		}
	}
}

type finiteObjectProperty struct {
	name     string
	required bool
	values   []jsonvalue.Value
}

func finiteObjectValues(
	registry *DomainRegistry,
	constraints ObjectConstraints,
	active map[DomainID]bool,
) ([]jsonvalue.Value, bool, error) {
	if constraints.Additional.Values != EmptyDomainID {
		return nil, false, nil
	}

	properties := make([]finiteObjectProperty, 0, len(constraints.Properties))
	for _, property := range constraints.Properties {
		if property.State == PropertyForbidden {
			if property.Required {
				return nil, true, nil
			}

			continue
		}

		values, finite, err := finiteDomainValuesAt(registry, property.Values, active)
		if err != nil || !finite {
			return nil, finite, err
		}

		if len(values) == 0 {
			if property.Required {
				return nil, true, nil
			}

			continue
		}

		properties = append(properties, finiteObjectProperty{
			name: property.Name, required: property.Required, values: values,
		})
	}

	result := make([]jsonvalue.Value, 0)
	if !appendFiniteObjects(&result, properties, constraints, 0, nil) {
		return nil, false, nil
	}

	return result, true, nil
}

//nolint:cyclop // Optional-property recursion directly enumerates every finite object shape.
func appendFiniteObjects(
	result *[]jsonvalue.Value,
	properties []finiteObjectProperty,
	constraints ObjectConstraints,
	index int,
	members []jsonvalue.Member,
) bool {
	if len(*result) > maximumFiniteComplementValues {
		return false
	}

	if index == len(properties) {
		if len(members) < constraints.MinProps || constraints.MaxProps != nil && len(members) > *constraints.MaxProps {
			return true
		}

		value, err := jsonvalue.Object(members)
		if err != nil {
			return false
		}

		*result = append(*result, value)

		return true
	}

	property := properties[index]
	if !property.required && !appendFiniteObjects(result, properties, constraints, index+1, members) {
		return false
	}

	for _, value := range property.values {
		next := append(append([]jsonvalue.Member(nil), members...), jsonvalue.Member{Name: property.name, Value: value})
		if !appendFiniteObjects(result, properties, constraints, index+1, next) {
			return false
		}
	}

	return true
}
