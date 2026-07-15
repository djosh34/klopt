//nolint:godoclint // Private OAS query merge names are local implementation details.
package oas

import (
	"encoding/json"
	"fmt"
)

type parameterIdentity struct {
	name     string
	location string
}

type locatedParameter struct {
	schema   LocatedSchema
	identity parameterIdentity
}

func (source Source) mergedQueryParameters(pathItem LocatedSchema, operation LocatedSchema) ([]LocatedSchema, error) {
	pathParameters, err := source.parameterList(pathItem)
	if err != nil {
		return nil, fmt.Errorf("path item parameters: %w", err)
	}

	operationParameters, err := source.parameterList(operation)
	if err != nil {
		return nil, fmt.Errorf("operation parameters: %w", err)
	}

	merged := append([]locatedParameter(nil), pathParameters...)

	positions := make(map[parameterIdentity]int, len(merged))
	for index, parameter := range merged {
		positions[parameter.identity] = index
	}

	for _, parameter := range operationParameters {
		if index, ok := positions[parameter.identity]; ok {
			merged[index] = parameter

			continue
		}

		positions[parameter.identity] = len(merged)
		merged = append(merged, parameter)
	}

	query := make([]LocatedSchema, 0, len(merged))
	for _, parameter := range merged {
		if parameter.identity.location == "query" {
			query = append(query, parameter.schema)
		}
	}

	return query, nil
}

func (source Source) parameterList(parent LocatedSchema) ([]locatedParameter, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(parent.Raw, &members); err != nil {
		return nil, fmt.Errorf("parse object at %s: %w", parent.Pointer, err)
	}

	raw, ok := members["parameters"]
	if !ok {
		return nil, nil
	}

	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("parameters at %s must be an array: %w", parent.Pointer, err)
	}

	if values == nil {
		return nil, fmt.Errorf("parameters at %s must be an array", parent.Pointer)
	}

	parameters := make([]locatedParameter, 0, len(values))

	identities := make(map[parameterIdentity]string, len(values))
	for index, value := range values {
		occurrence := LocatedSchema{
			Raw:     value,
			Pointer: appendPointer(parent.Pointer, "parameters", fmt.Sprint(index)),
		}

		resolved, err := source.Resolve(occurrence)
		if err != nil {
			return nil, fmt.Errorf("parameter at %s: %w", occurrence.Pointer, err)
		}

		identity, err := parameterObjectIdentity(resolved)
		if err != nil {
			return nil, err
		}

		if first, duplicate := identities[identity]; duplicate {
			return nil, fmt.Errorf(
				"parameter (%q, %q) is duplicated at %s and %s",
				identity.name,
				identity.location,
				first,
				resolved.Pointer,
			)
		}

		identities[identity] = resolved.Pointer
		parameters = append(parameters, locatedParameter{schema: resolved, identity: identity})
	}

	return parameters, nil
}

func parameterObjectIdentity(parameter LocatedSchema) (parameterIdentity, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(parameter.Raw, &members); err != nil || members == nil {
		return parameterIdentity{}, fmt.Errorf("parameter at %s must be an object", parameter.Pointer)
	}

	var identity parameterIdentity
	if err := json.Unmarshal(members["name"], &identity.name); err != nil || identity.name == "" {
		return parameterIdentity{}, fmt.Errorf("parameter at %s name must be a non-empty string", parameter.Pointer)
	}

	if err := json.Unmarshal(members["in"], &identity.location); err != nil || identity.location == "" {
		return parameterIdentity{}, fmt.Errorf("parameter at %s in must be a non-empty string", parameter.Pointer)
	}

	return identity, nil
}
