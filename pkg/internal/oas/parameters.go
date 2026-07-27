//nolint:godoclint // Private OAS parameter merge names are local implementation details.
package oas

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime"
	"strings"
)

type parameterIdentity struct {
	name     string
	location string
}

type locatedParameter struct {
	schema   LocatedSchema
	identity parameterIdentity
}

func (source Source) mergedParameters(
	pathTemplate string,
	pathExpressions []string,
	pathParameters []locatedParameter,
	operationParameters []locatedParameter,
) ([]LocatedSchema, []LocatedSchema, error) {
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

	if err := validatePathParameterCorrespondence(pathTemplate, pathExpressions, merged); err != nil {
		return nil, nil, err
	}

	query := make([]LocatedSchema, 0, len(merged))

	path := make([]LocatedSchema, 0, len(merged))
	for _, parameter := range merged {
		switch parameter.identity.location {
		case "query":
			query = append(query, parameter.schema)
		case "path":
			path = append(path, parameter.schema)
		}
	}

	return query, path, nil
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

//nolint:cyclop // Parameter identity and fixed-field validity form one declaration boundary.
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

	switch identity.location {
	case "query", "header", "path", "cookie":
	default:
		return parameterIdentity{}, fmt.Errorf(
			"parameter at %s in must be one of query, header, path, or cookie",
			parameter.Pointer,
		)
	}

	_, hasSchema := members["schema"]

	_, hasContent := members["content"]
	if hasSchema == hasContent {
		return parameterIdentity{}, fmt.Errorf(
			"parameter %q at %s must contain exactly one of schema or content",
			identity.name,
			parameter.Pointer,
		)
	}

	if err := validateParameterFields(members, identity, parameter.Pointer); err != nil {
		return parameterIdentity{}, err
	}

	if identity.location == "path" {
		required, err := optionalBoolean(members["required"], "required")
		if err != nil {
			return parameterIdentity{}, fmt.Errorf(
				"path parameter %q at %s required: %w",
				identity.name,
				parameter.Pointer,
				err,
			)
		}

		if !required {
			return parameterIdentity{}, fmt.Errorf(
				"path parameter %q at %s required must be true",
				identity.name,
				parameter.Pointer,
			)
		}
	}

	return identity, nil
}

//nolint:cyclop // Fixed fields and the schema/content alternatives form one finite decision table.
func validateParameterFields(
	members map[string]json.RawMessage,
	identity parameterIdentity,
	pointer string,
) error {
	for _, field := range []string{"required", "deprecated", "allowEmptyValue", "allowReserved", "explode"} {
		if _, err := optionalBoolean(members[field], field); err != nil {
			return fmt.Errorf(
				"%s parameter %q at %s %s: %w",
				identity.location,
				identity.name,
				pointer,
				field,
				err,
			)
		}
	}

	for _, field := range []string{"description", "style"} {
		raw, ok := members[field]
		if !ok {
			continue
		}

		var value string
		if err := json.Unmarshal(raw, &value); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf(
				"%s parameter %q at %s %s must be a string",
				identity.location,
				identity.name,
				pointer,
				field,
			)
		}
	}

	contentRaw, hasContent := members["content"]
	if !hasContent {
		return validateSchemaObjectShape(identity.name, appendPointer(pointer, "schema"), members["schema"])
	}

	for _, field := range []string{"allowReserved", "style", "explode", "example", "examples"} {
		if _, present := members[field]; present {
			return fmt.Errorf(
				"parameter %q content cannot be combined with %s at %s",
				identity.name,
				field,
				appendPointer(pointer, field),
			)
		}
	}

	contentPointer := appendPointer(pointer, "content")

	var content map[string]json.RawMessage
	if err := json.Unmarshal(contentRaw, &content); err != nil || content == nil {
		return fmt.Errorf("parameter %q content at %s must be an object", identity.name, contentPointer)
	}

	if len(content) != 1 {
		return fmt.Errorf(
			"parameter %q content at %s must contain exactly one media type",
			identity.name,
			contentPointer,
		)
	}

	for mediaTypeName, mediaTypeRaw := range content {
		mediaTypePointer := appendPointer(contentPointer, mediaTypeName)

		parsedMediaType, _, err := mime.ParseMediaType(mediaTypeName)
		if err != nil || strings.Count(parsedMediaType, "/") != 1 {
			return fmt.Errorf(
				"%s parameter %q content at %s media type %q is malformed",
				identity.location,
				identity.name,
				contentPointer,
				mediaTypeName,
			)
		}

		var mediaType map[string]json.RawMessage
		if err := json.Unmarshal(mediaTypeRaw, &mediaType); err != nil || mediaType == nil {
			return fmt.Errorf(
				"parameter %q Media Type Object %q at %s must be an object",
				identity.name,
				mediaTypeName,
				mediaTypePointer,
			)
		}

		if schemaRaw, ok := mediaType["schema"]; ok {
			return validateSchemaObjectShape(
				identity.name,
				appendPointer(mediaTypePointer, "schema"),
				schemaRaw,
			)
		}
	}

	return nil
}

func validateSchemaObjectShape(name string, pointer string, raw json.RawMessage) error {
	var schema map[string]json.RawMessage
	if err := json.Unmarshal(raw, &schema); err != nil || schema == nil {
		return fmt.Errorf("parameter %q Schema Object must be an object at %s", name, pointer)
	}

	return nil
}
