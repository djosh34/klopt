//nolint:godoclint,lll // Private decoder names and contextual errors are local implementation details.
package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/go-json-experiment/json/jsontext"
)

type rawPair struct {
	rawName      string
	name         string
	rawValue     string
	decodedValue string
	property     int
	childName    string
}

// Decode converts one URL query into a validated JSON object.
//
//nolint:cyclop,gocognit // Claiming, ordered emission, and final validation are one request pipeline.
func (decoder *QueryDecoder) Decode(input *url.URL) (json.RawMessage, error) {
	if input == nil {
		return nil, fmt.Errorf("operationId %q decode query: URL is nil", decoder.operationID)
	}

	pairs, err := lexRawQuery(input.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("operationId %q claim query: %w", decoder.operationID, err)
	}

	claimed := make([][]rawPair, len(decoder.parameters))

	for _, pair := range pairs {
		if strings.ContainsAny(pair.rawName, "[]") {
			return nil, fmt.Errorf(
				"operationId %q claim query name %q: brackets must use canonical %%5B and %%5D encoding",
				decoder.operationID,
				pair.name,
			)
		}

		claim, ok := decoder.owners[pair.name]
		if ok {
			if strings.ContainsAny(pair.name, "[]") && !canonicalBracketEncoding(pair) {
				return nil, fmt.Errorf(
					"operationId %q claim query name %q: brackets must use canonical %%5B and %%5D encoding",
					decoder.operationID,
					pair.name,
				)
			}

			pair.property = claim.property
			if claim.property >= 0 {
				pair.childName = decoder.parameters[claim.parameter].properties[claim.property].name
			}

			claimed[claim.parameter] = append(claimed[claim.parameter], pair)

			continue
		}

		deepParameter, child, deepErr := decoder.claimDeepName(pair)
		if deepErr != nil {
			return nil, deepErr
		}

		if deepParameter != -1 {
			pair.childName = child
			claimed[deepParameter] = append(claimed[deepParameter], pair)

			continue
		}

		if decoder.openForm != -1 {
			pair.childName = pair.name
			claimed[decoder.openForm] = append(claimed[decoder.openForm], pair)
		}
	}

	var output bytes.Buffer

	encoder := jsontext.NewEncoder(&output)
	if err := encoder.WriteToken(jsontext.BeginObject); err != nil {
		return nil, fmt.Errorf("operationId %q encode query object: %w", decoder.operationID, err)
	}

	for index := range decoder.parameters {
		parameter := &decoder.parameters[index]

		occurrences := claimed[index]
		if len(occurrences) == 0 {
			if parameter.required {
				return nil, fmt.Errorf("operationId %q decode query parameter %q: required parameter is absent", decoder.operationID, parameter.name)
			}

			if parameter.defaultValue == nil {
				continue
			}
		}

		for _, occurrence := range occurrences {
			if occurrence.decodedValue == "" && !parameter.allowEmpty {
				return nil, fmt.Errorf("operationId %q decode query parameter %q: empty value is not allowed", decoder.operationID, parameter.name)
			}
		}

		if err := encoder.WriteToken(jsontext.String(parameter.name)); err != nil {
			return nil, fmt.Errorf("operationId %q encode query parameter name %q: %w", decoder.operationID, parameter.name, err)
		}

		if len(occurrences) == 0 {
			if err := encoder.WriteValue(parameter.defaultValue); err != nil {
				return nil, fmt.Errorf("operationId %q encode query parameter %q default: %w", decoder.operationID, parameter.name, err)
			}

			continue
		}

		if err := parameter.writeValue(encoder, occurrences); err != nil {
			return nil, fmt.Errorf("operationId %q decode query parameter %q: %w", decoder.operationID, parameter.name, err)
		}
	}

	if err := encoder.WriteToken(jsontext.EndObject); err != nil {
		return nil, fmt.Errorf("operationId %q encode query object: %w", decoder.operationID, err)
	}

	query := json.RawMessage(bytes.TrimSpace(output.Bytes()))
	if errs := decoder.validation.Validate(query); len(errs) != 0 {
		return nil, fmt.Errorf("operationId %q validate query: %w", decoder.operationID, errors.Join(errs...))
	}

	return query, nil
}

func lexRawQuery(rawQuery string) ([]rawPair, error) {
	if rawQuery == "" {
		return nil, nil
	}

	parts := strings.Split(rawQuery, "&")

	pairs := make([]rawPair, 0, len(parts))
	for _, part := range parts {
		rawName, rawValue, _ := strings.Cut(part, "=")

		name, err := url.QueryUnescape(rawName)
		if err != nil {
			return nil, fmt.Errorf("decode query name %q: %w", rawName, err)
		}

		value, err := url.QueryUnescape(rawValue)
		if err != nil {
			return nil, fmt.Errorf("decode query value for %q: %w", name, err)
		}

		if !utf8.ValidString(name) {
			return nil, fmt.Errorf("query name %q is not valid UTF-8", rawName)
		}

		if !utf8.ValidString(value) {
			return nil, fmt.Errorf("query value for %q is not valid UTF-8", name)
		}

		pairs = append(pairs, rawPair{
			rawName: rawName, name: name, rawValue: rawValue, decodedValue: value, property: -1,
		})
	}

	return pairs, nil
}

//nolint:cyclop // Canonical one-level deepObject grammar is checked at this ownership seam.
func (decoder *QueryDecoder) claimDeepName(pair rawPair) (int, string, error) {
	for index := range decoder.parameters {
		parameter := &decoder.parameters[index]
		if parameter.wire != wireDeepObject || pair.name != parameter.name &&
			!strings.HasPrefix(pair.name, parameter.name+"[") &&
			!strings.HasPrefix(pair.name, parameter.name+"]") {
			continue
		}

		prefix := parameter.name + "["
		if !strings.HasPrefix(pair.name, prefix) || !strings.HasSuffix(pair.name, "]") {
			return -1, "", decoder.malformedDeepName(parameter.name, pair.name)
		}

		child := strings.TrimSuffix(strings.TrimPrefix(pair.name, prefix), "]")
		if !canonicalBracketEncoding(pair) {
			return -1, "", fmt.Errorf(
				"operationId %q claim query parameter %q: deepObject brackets in %q must use canonical %%5B and %%5D encoding",
				decoder.operationID,
				parameter.name,
				pair.name,
			)
		}

		if strings.ContainsAny(child, "[]") ||
			strings.Count(pair.rawName, "%5B") != 1 || strings.Count(pair.rawName, "%5D") != 1 {
			return -1, "", decoder.malformedDeepName(parameter.name, pair.name)
		}

		if parameter.dynamicType == "" {
			return -1, "", decoder.malformedDeepName(parameter.name, pair.name)
		}

		return index, child, nil
	}

	return -1, "", nil
}

func canonicalBracketEncoding(pair rawPair) bool {
	return !strings.ContainsAny(pair.rawName, "[]") &&
		strings.Count(pair.name, "[") == strings.Count(pair.rawName, "%5B") &&
		strings.Count(pair.name, "]") == strings.Count(pair.rawName, "%5D")
}

func (decoder *QueryDecoder) malformedDeepName(base string, name string) error {
	return fmt.Errorf(
		"operationId %q claim query parameter %q: malformed or unknown deepObject child %q",
		decoder.operationID,
		base,
		name,
	)
}

//nolint:cyclop // The finite wire-kind switch is the decoder's central policy.
func (parameter *queryParameter) writeValue(encoder *jsontext.Encoder, occurrences []rawPair) error {
	switch parameter.wire {
	case wirePrimitive:
		if len(occurrences) != 1 {
			return errors.New("duplicate scalar occurrence")
		}

		value, err := convertQueryAlternatives(parameter, func(candidate queryConversion) (jsontext.Value, error) {
			return encodeQueryScalar(candidate.scalarType, occurrences[0].decodedValue, parameter.allowEmpty)
		})
		if err != nil {
			return err
		}

		return encoder.WriteValue(value)
	case wireFormArrayRepeated:
		values := make([]string, 0, len(occurrences))
		for _, occurrence := range occurrences {
			values = append(values, occurrence.decodedValue)
		}

		return writeConvertedArray(encoder, parameter, values)
	case wireDelimitedArray:
		if len(occurrences) != 1 {
			return errors.New("duplicate non-exploded array occurrence")
		}

		values, err := splitStyleValue(occurrences[0], parameter.separator)
		if err != nil {
			return err
		}

		return writeConvertedArray(encoder, parameter, values)
	case wireFormObjectNamed, wireDelimitedObject:
		if len(occurrences) != 1 {
			return errors.New("duplicate non-exploded object occurrence")
		}

		return parameter.writeConvertedObject(encoder, func(candidate *queryParameter, output *jsontext.Encoder) error {
			return candidate.writeNamedObject(output, occurrences[0])
		})
	case wireFormObjectExploded, wireDeepObject:
		return parameter.writeConvertedObject(encoder, func(candidate *queryParameter, output *jsontext.Encoder) error {
			return candidate.writeExplodedObject(output, occurrences)
		})
	case wireJSONContent:
		if len(occurrences) != 1 {
			return errors.New("duplicate JSON content occurrence")
		}

		return encoder.WriteValue(jsontext.Value(occurrences[0].decodedValue))
	default:
		return errors.New("unknown compiled wire kind")
	}
}

func (parameter *queryParameter) writeConvertedObject(
	encoder *jsontext.Encoder,
	write func(*queryParameter, *jsontext.Encoder) error,
) error {
	value, err := convertQueryAlternatives(parameter, func(conversion queryConversion) (jsontext.Value, error) {
		candidate := *parameter
		candidate.validation = conversion.validation
		candidate.dynamicType = conversion.dynamicType
		candidate.dynamicValidation = conversion.dynamicValidation
		candidate.properties = conversion.properties
		candidate.propertyByName = conversion.propertyByName

		var output bytes.Buffer

		candidateEncoder := jsontext.NewEncoder(&output)
		if err := write(&candidate, candidateEncoder); err != nil {
			return nil, err
		}

		return append(jsontext.Value(nil), bytes.TrimSpace(output.Bytes())...), nil
	})
	if err != nil {
		return err
	}

	return encoder.WriteValue(value)
}

//nolint:cyclop // Declared and dynamic packed properties share duplicate detection and ordered emission.
func (parameter *queryParameter) writeNamedObject(encoder *jsontext.Encoder, occurrence rawPair) error {
	tokens, err := splitStyleValue(occurrence, parameter.separator)
	if err != nil {
		return err
	}

	if len(tokens) == 0 || len(tokens)%2 != 0 {
		return errors.New("object serialization must contain name/value pairs")
	}

	if err := encoder.WriteToken(jsontext.BeginObject); err != nil {
		return err
	}

	const tupleWidth = 2

	seen := make(map[string]struct{}, len(tokens)/tupleWidth)
	for index := 0; index < len(tokens); index += 2 {
		name := tokens[index]

		propertyIndex, ok := parameter.propertyByName[name]
		if !ok && parameter.dynamicType == "" {
			return fmt.Errorf("unknown object property %q", name)
		}

		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("duplicate object property %q", name)
		}

		seen[name] = struct{}{}
		if err := encoder.WriteToken(jsontext.String(name)); err != nil {
			return err
		}

		typeName := parameter.dynamicType

		valueValidation := parameter.dynamicValidation
		if ok {
			typeName = parameter.properties[propertyIndex].scalarType
			valueValidation = parameter.properties[propertyIndex].validation
		}

		if err := writeConvertedScalar(
			encoder, valueValidation, typeName, tokens[index+1], parameter.allowEmpty,
		); err != nil {
			return fmt.Errorf("property %q: %w", name, err)
		}
	}

	return encoder.WriteToken(jsontext.EndObject)
}

//nolint:cyclop,gocognit // Property grouping and the documented array-child extension stay together.
func (parameter *queryParameter) writeExplodedObject(encoder *jsontext.Encoder, occurrences []rawPair) error {
	if err := encoder.WriteToken(jsontext.BeginObject); err != nil {
		return err
	}

	for _, property := range parameter.properties {
		count := 0

		for _, occurrence := range occurrences {
			if occurrence.childName == property.name {
				count++
			}
		}

		if count == 0 {
			continue
		}

		if !property.array && count != 1 {
			return fmt.Errorf("duplicate scalar object property %q", property.name)
		}

		if err := encoder.WriteToken(jsontext.String(property.name)); err != nil {
			return err
		}

		if property.array {
			if err := encoder.WriteToken(jsontext.BeginArray); err != nil {
				return err
			}
		}

		for _, occurrence := range occurrences {
			if occurrence.childName != property.name {
				continue
			}

			valueValidation := property.validation
			if property.array {
				valueValidation = property.itemValidation
			}

			if err := writeConvertedScalar(
				encoder, valueValidation, property.scalarType,
				occurrence.decodedValue, parameter.allowEmpty,
			); err != nil {
				return fmt.Errorf("property %q: %w", property.name, err)
			}
		}

		if property.array {
			if err := encoder.WriteToken(jsontext.EndArray); err != nil {
				return err
			}
		}
	}

	seen := make(map[string]struct{})

	for _, occurrence := range occurrences {
		if _, declared := parameter.propertyByName[occurrence.childName]; declared {
			continue
		}

		if _, duplicate := seen[occurrence.childName]; duplicate {
			return fmt.Errorf("duplicate scalar object property %q", occurrence.childName)
		}

		seen[occurrence.childName] = struct{}{}
		if err := encoder.WriteToken(jsontext.String(occurrence.childName)); err != nil {
			return err
		}

		if err := writeConvertedScalar(
			encoder, parameter.dynamicValidation, parameter.dynamicType,
			occurrence.decodedValue, parameter.allowEmpty,
		); err != nil {
			return fmt.Errorf("property %q: %w", occurrence.childName, err)
		}
	}

	return encoder.WriteToken(jsontext.EndObject)
}

func splitStyleValue(pair rawPair, separator string) ([]string, error) {
	if separator == "|" {
		if strings.Contains(pair.rawValue, separator) {
			return nil, errors.New(`pipeDelimited separator "|" must be percent-encoded as "%7C"`)
		}

		return strings.Split(pair.decodedValue, separator), nil
	}

	if separator == " " {
		return strings.Split(pair.decodedValue, separator), nil
	}

	rawTokens := strings.Split(pair.rawValue, separator)

	tokens := make([]string, len(rawTokens))
	for index, rawToken := range rawTokens {
		decoded, err := url.QueryUnescape(rawToken)
		if err != nil {
			return nil, fmt.Errorf("decode style token %q: %w", rawToken, err)
		}

		if !utf8.ValidString(decoded) {
			return nil, fmt.Errorf("style token %q is not valid UTF-8", rawToken)
		}

		tokens[index] = decoded
	}

	return tokens, nil
}

//nolint:cyclop // Four explicit OpenAPI scalar kinds are clearer than indirect conversion.
func writeScalar(encoder *jsontext.Encoder, typeName string, value string, allowEmpty bool) error {
	if value == "" && !allowEmpty {
		return errors.New("empty value is not allowed")
	}

	switch typeName {
	case "string":
		return encoder.WriteToken(jsontext.String(value))
	case "boolean":
		switch value {
		case "true":
			return encoder.WriteToken(jsontext.Bool(true))
		case "false":
			return encoder.WriteToken(jsontext.Bool(false))
		default:
			return fmt.Errorf("%q is not a boolean", value)
		}
	case "integer", "number":
		number, err := jsonvalue.ParseNumber(value)
		if err != nil {
			return err
		}

		if typeName == "integer" && !number.IsInteger() {
			return fmt.Errorf("%q is not an integer", value)
		}

		return encoder.WriteValue(jsontext.Value(number.Lexeme))
	default:
		return fmt.Errorf("unsupported scalar type %q", typeName)
	}
}

func encodeQueryScalar(typeName string, value string, allowEmpty bool) (jsontext.Value, error) {
	var output bytes.Buffer

	encoder := jsontext.NewEncoder(&output)
	if err := writeScalar(encoder, typeName, value, allowEmpty); err != nil {
		return nil, err
	}

	return append(jsontext.Value(nil), bytes.TrimSpace(output.Bytes())...), nil
}

func writeConvertedScalar(
	encoder *jsontext.Encoder,
	validation *Validation,
	fallbackType string,
	value string,
	allowEmpty bool,
) error {
	if validation == nil {
		return writeScalar(encoder, fallbackType, value, allowEmpty)
	}

	converted, err := convertAnyOfValue(validation, func(candidate *Validation) (jsontext.Value, error) {
		typeName := fallbackType

		if containsAnyOf(validation) {
			candidateType := compiledValidationType(candidate)
			if candidateType != "" {
				typeName = candidateType
			}
		}

		return encodeQueryScalar(typeName, value, allowEmpty)
	})
	if err != nil {
		return err
	}

	return encoder.WriteValue(converted)
}

func writeConvertedArray(
	encoder *jsontext.Encoder,
	parameter *queryParameter,
	values []string,
) error {
	converted, err := convertQueryAlternatives(parameter, func(candidate queryConversion) (jsontext.Value, error) {
		items := candidate.itemValidation
		typeName := candidate.scalarType

		var output bytes.Buffer

		arrayEncoder := jsontext.NewEncoder(&output)
		if err := arrayEncoder.WriteToken(jsontext.BeginArray); err != nil {
			return nil, err
		}

		for _, value := range values {
			if err := writeConvertedScalar(arrayEncoder, items, typeName, value, parameter.allowEmpty); err != nil {
				return nil, err
			}
		}

		if err := arrayEncoder.WriteToken(jsontext.EndArray); err != nil {
			return nil, err
		}

		return append(jsontext.Value(nil), bytes.TrimSpace(output.Bytes())...), nil
	})
	if err != nil {
		return err
	}

	return encoder.WriteValue(converted)
}

func convertQueryAlternatives(
	parameter *queryParameter,
	convert func(queryConversion) (jsontext.Value, error),
) (jsontext.Value, error) {
	if !containsAnyOf(parameter.validation) {
		if len(parameter.conversions) != 1 {
			return nil, errors.New("query conversion metadata is missing")
		}

		return convert(parameter.conversions[0])
	}

	var firstErr error

	for _, candidate := range parameter.conversions {
		value, err := convert(candidate)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}

			continue
		}

		if errs := validateRaw(candidate.validation, json.RawMessage(value), "#"); len(errs) != 0 {
			if firstErr == nil {
				firstErr = errors.Join(errs...)
			}

			continue
		}

		return value, nil
	}

	if firstErr != nil {
		return nil, firstErr
	}

	return nil, errors.New("value does not match anyOf")
}

// convertAnyOfValue tries ordered branch conversion and acceptance.
func convertAnyOfValue(
	validation *Validation,
	convert func(*Validation) (jsontext.Value, error),
) (jsontext.Value, error) {
	if !containsAnyOf(validation) {
		return convert(validation)
	}

	return convertValidationProfiles(validation, convert)
}

func convertValidationProfiles(
	validation *Validation,
	convert func(*Validation) (jsontext.Value, error),
) (jsontext.Value, error) {
	profiles, err := boundedConversionProfiles(validation)
	if err != nil {
		return nil, err
	}

	var firstErr error

	for _, profile := range profiles {
		value, err := convert(profile)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}

			continue
		}

		if errs := validateRaw(profile, json.RawMessage(value), "#"); len(errs) != 0 {
			if firstErr == nil {
				firstErr = errors.Join(errs...)
			}

			continue
		}

		return value, nil
	}

	if firstErr != nil {
		return nil, firstErr
	}

	return nil, errors.New("value does not match anyOf")
}
