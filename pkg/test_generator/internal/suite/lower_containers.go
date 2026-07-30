//nolint:godoclint // Private container lowering stays behind compileSemantic.
package suite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/djosh34/klopt/pkg/internal/oas"
)

//nolint:cyclop // Independent OpenAPI container keywords are lowered explicitly.
func (lowerer *semanticLowerer) lowerContainers(
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
) ([]SetRef, []string, error) {
	refs := make([]SetRef, 0)

	minItems, err := rawNumber(members, "minItems")
	if err != nil {
		return nil, nil, err
	}

	maxItems, err := rawNumber(members, "maxItems")
	if err != nil {
		return nil, nil, err
	}

	if minItems != nil || maxItems != nil {
		value, atomErr := lowerer.arena.Atom(arrayLengthAtom{Minimum: minItems, Maximum: maxItems})
		if atomErr != nil {
			return nil, nil, atomErr
		}

		refs = append(refs, value)
	}

	minProperties, err := rawNumber(members, "minProperties")
	if err != nil {
		return nil, nil, err
	}

	maxProperties, err := rawNumber(members, "maxProperties")
	if err != nil {
		return nil, nil, err
	}

	if minProperties != nil || maxProperties != nil {
		value, atomErr := lowerer.arena.Atom(objectCountAtom{Minimum: minProperties, Maximum: maxProperties})
		if atomErr != nil {
			return nil, nil, atomErr
		}

		refs = append(refs, value)
	}

	readOnlyNames, err := lowerer.readOnlyProperties(schema, members)
	if err != nil {
		return nil, nil, err
	}

	var required []string
	if raw, ok := members["required"]; ok {
		if err := json.Unmarshal(raw, &required); err != nil {
			return nil, nil, err
		}
	}

	for _, name := range required {
		if slices.Contains(readOnlyNames, name) {
			continue
		}

		value, atomErr := lowerer.arena.Atom(requiredPropertyAtom{Name: name})
		if atomErr != nil {
			return nil, nil, atomErr
		}

		refs = append(refs, value)
	}

	propertyNames := make([]string, 0)

	if raw, ok := members["properties"]; ok {
		var properties map[string]json.RawMessage
		if err := json.Unmarshal(raw, &properties); err != nil {
			return nil, nil, err
		}

		propertyNames = slices.Sorted(maps.Keys(properties))
	}

	if raw, ok := members["additionalProperties"]; ok {
		trimmed := bytes.TrimSpace(raw)
		if bytes.Equal(trimmed, []byte("false")) {
			value, atomErr := lowerer.arena.Atom(allowedPropertyNamesAtom{Names: propertyNames})
			if atomErr != nil {
				return nil, nil, atomErr
			}

			refs = append(refs, value)
		}
	}

	return refs, propertyNames, nil
}

func (lowerer *semanticLowerer) readOnlyProperties(
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
) ([]string, error) {
	raw, ok := members["properties"]
	if !ok {
		return nil, nil
	}

	var properties map[string]json.RawMessage
	if err := json.Unmarshal(raw, &properties); err != nil {
		return nil, err
	}

	result := make([]string, 0)

	for _, name := range slices.Sorted(maps.Keys(properties)) {
		child, err := lowerer.source.Child(schema, "properties", name)
		if err != nil {
			return nil, err
		}

		resolved, err := lowerer.source.Resolve(child)
		if err != nil {
			return nil, err
		}

		childMembers, err := rawObject(resolved)
		if err != nil {
			return nil, err
		}

		readOnly, _, err := rawBoolean(childMembers, "readOnly")
		if err != nil {
			return nil, fmt.Errorf("lower schema at %s/readOnly: %w", resolved.Pointer, err)
		}

		if readOnly {
			result = append(result, name)
		}
	}

	return result, nil
}
