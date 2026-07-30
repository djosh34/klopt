//nolint:godoclint // Private path runtime names and diagnostics are local implementation details.
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

// DecodePathParams decodes one operation-relative URL path into a validated JSON object.
func (decoder *PathDecoder) DecodePathParams(input *url.URL) (json.RawMessage, error) {
	result, err := decoder.decodePathParams(input)
	if err != nil {
		return nil, fmt.Errorf("decode path parameters for operation %q: %w", decoder.operationID, err)
	}

	return result, nil
}

//nolint:cyclop // Matching, ordered extraction, encoding, and aggregate validation form one request pipeline.
func (decoder *PathDecoder) decodePathParams(input *url.URL) (json.RawMessage, error) {
	if input == nil {
		return nil, errors.New("nil URL")
	}

	parts := strings.Split(input.EscapedPath(), "/")
	if len(parts) != len(decoder.segments) {
		return nil, errors.New("path segment count does not match template")
	}

	values := make([]jsontext.Value, len(decoder.parameters))
	parameterIndex := 0

	for segmentIndex, matcher := range decoder.segments {
		matches := matcher.FindStringSubmatch(parts[segmentIndex])
		if matches == nil {
			return nil, fmt.Errorf("path segment %d does not match template", segmentIndex)
		}

		for _, raw := range matches[1:] {
			if parameterIndex >= len(decoder.parameters) {
				return nil, errors.New("path decoder invariant failed")
			}

			value, err := decoder.parameters[parameterIndex].decodePathValue(raw)
			if err != nil {
				return nil, fmt.Errorf("path parameter %q: %w", decoder.parameters[parameterIndex].name, err)
			}

			values[parameterIndex] = value
			parameterIndex++
		}
	}

	if parameterIndex != len(decoder.parameters) {
		return nil, errors.New("path decoder invariant failed")
	}

	result, err := encodePathObject(decoder.parameters, values)
	if err != nil {
		return nil, err
	}

	if errs := decoder.validation.Validate(result); len(errs) != 0 {
		return nil, errors.Join(errs...)
	}

	return result, nil
}

//nolint:cyclop // The complete-capture empty policy and finite wire-shape dispatch are one codec boundary.
func (parameter *pathParameter) decodePathValue(raw string) (jsontext.Value, error) {
	if len(parameter.anyOf) != 0 {
		return parameter.decodeAnyOfPathValue(raw)
	}

	if raw == "" {
		switch parameter.wire {
		case pathWireSimpleArray:
			return jsontext.Value(`[]`), nil
		case pathWireSimpleObject:
			return jsontext.Value(`{}`), nil
		case pathWireSimplePrimitive:
			if parameter.scalarType == "string" {
				return jsontext.Value(`""`), nil
			}
		}
	}

	if raw == "." {
		switch parameter.wire {
		case pathWireLabelArray:
			return jsontext.Value(`[]`), nil
		case pathWireLabelObject:
			return jsontext.Value(`{}`), nil
		}
	}

	if raw == ";"+url.PathEscape(parameter.name) {
		switch parameter.wire {
		case pathWireMatrixPrimitive:
			if parameter.scalarType == "string" {
				return jsontext.Value(`""`), nil
			}
		case pathWireMatrixArray:
			return jsontext.Value(`[]`), nil
		case pathWireMatrixObject:
			return jsontext.Value(`{}`), nil
		}
	}

	if parameter.wire == pathWireJSONContent {
		return parameter.decodeJSONPathValue(raw)
	}

	switch pathShape(parameter.wire % pathWireKind(pathShapeCount)) {
	case pathShapePrimitive:
		return parameter.decodePathPrimitive(raw)
	case pathShapeArray:
		return parameter.decodePathArray(raw)
	case pathShapeObject:
		return parameter.decodePathObject(raw)
	default:
		return nil, fmt.Errorf("unknown compiled path wire %d", parameter.wire)
	}
}

func (parameter *pathParameter) decodeAnyOfPathValue(raw string) (jsontext.Value, error) {
	var lastErr error

	for index := range parameter.anyOf {
		candidate := &parameter.anyOf[index]

		value, err := candidate.decodePathValue(raw)
		if err == nil {
			errs := candidate.validation.Validate(json.RawMessage(value))
			if len(errs) == 0 {
				return value, nil
			}

			err = errors.Join(errs...)
		}

		lastErr = err
	}

	return nil, lastErr
}

func (parameter *pathParameter) decodeJSONPathValue(raw string) (jsontext.Value, error) {
	decoded, err := decodePathToken(raw)
	if err != nil {
		return nil, err
	}

	value := jsontext.Value(decoded)
	if _, err := jsonvalue.Parse(value); err != nil {
		return nil, fmt.Errorf("decode JSON content: %w", err)
	}

	return append(jsontext.Value(nil), value...), nil
}

func (parameter *pathParameter) decodePathPrimitive(raw string) (jsontext.Value, error) {
	body, err := parameter.pathStyleBody(raw)
	if err != nil {
		return nil, err
	}

	decoded, err := decodePathToken(body)
	if err != nil {
		return nil, err
	}

	return encodePathScalar(parameter.scalarType, decoded)
}

func (parameter *pathParameter) decodePathArray(raw string) (jsontext.Value, error) {
	var rawValues []string

	switch parameter.wire {
	case pathWireSimpleArray:
		rawValues = strings.Split(raw, ",")
	case pathWireLabelArray:
		if !strings.HasPrefix(raw, ".") {
			return nil, errors.New("label value is missing . prefix")
		}

		separator := ","
		if parameter.explode {
			separator = "."
		}

		rawValues = strings.Split(raw[1:], separator)
	case pathWireMatrixArray:
		if parameter.explode {
			values, err := parameter.matrixArrayValues(raw)
			if err != nil {
				return nil, err
			}

			rawValues = values
		} else {
			body, err := parameter.matrixBody(raw)
			if err != nil {
				return nil, err
			}

			rawValues = strings.Split(body, ",")
		}
	default:
		return nil, fmt.Errorf("unknown compiled array wire %d", parameter.wire)
	}

	return encodePathArray(parameter.scalarType, rawValues)
}

//nolint:cyclop,nestif // The six object style/explode grammars form one finite dispatch.
func (parameter *pathParameter) decodePathObject(raw string) (jsontext.Value, error) {
	var rawPairs [][2]string

	switch parameter.wire {
	case pathWireSimpleObject:
		if parameter.explode {
			pairs, err := splitExplodedPathObject(raw, ",")
			if err != nil {
				return nil, err
			}

			rawPairs = pairs
		} else {
			pairs, err := splitPackedPathObject(raw)
			if err != nil {
				return nil, err
			}

			rawPairs = pairs
		}
	case pathWireLabelObject:
		if !strings.HasPrefix(raw, ".") {
			return nil, errors.New("label value is missing . prefix")
		}

		var err error
		if parameter.explode {
			rawPairs, err = splitExplodedPathObject(raw[1:], ".")
		} else {
			rawPairs, err = splitPackedPathObject(raw[1:])
		}

		if err != nil {
			return nil, err
		}
	case pathWireMatrixObject:
		if parameter.explode {
			if !strings.HasPrefix(raw, ";") {
				return nil, errors.New("matrix value is missing ; prefix")
			}

			pairs, err := splitExplodedPathObject(raw[1:], ";")
			if err != nil {
				return nil, err
			}

			rawPairs = pairs
		} else {
			body, err := parameter.matrixBody(raw)
			if err != nil {
				return nil, err
			}

			pairs, err := splitPackedPathObject(body)
			if err != nil {
				return nil, err
			}

			rawPairs = pairs
		}
	default:
		return nil, fmt.Errorf("unknown compiled object wire %d", parameter.wire)
	}

	return parameter.encodePathObjectValue(rawPairs)
}

func (parameter *pathParameter) pathStyleBody(raw string) (string, error) {
	switch parameter.wire {
	case pathWireSimplePrimitive:
		return raw, nil
	case pathWireLabelPrimitive:
		if !strings.HasPrefix(raw, ".") {
			return "", errors.New("label value is missing . prefix")
		}

		return raw[1:], nil
	case pathWireMatrixPrimitive:
		return parameter.matrixBody(raw)
	default:
		return "", fmt.Errorf("unknown compiled primitive wire %d", parameter.wire)
	}
}

func (parameter *pathParameter) matrixBody(raw string) (string, error) {
	prefix := ";" + url.PathEscape(parameter.name) + "="
	if !strings.HasPrefix(raw, prefix) {
		return "", fmt.Errorf("matrix value must begin with %q", prefix)
	}

	return raw[len(prefix):], nil
}

func (parameter *pathParameter) matrixArrayValues(raw string) ([]string, error) {
	if !strings.HasPrefix(raw, ";") {
		return nil, errors.New("matrix value is missing ; prefix")
	}

	terms := strings.Split(raw[1:], ";")
	escapedName := url.PathEscape(parameter.name)

	values := make([]string, len(terms))
	for index, term := range terms {
		name, value, hasValue := strings.Cut(term, "=")
		if name != escapedName {
			return nil, fmt.Errorf("matrix array term names %q, want %q", name, escapedName)
		}

		if hasValue {
			values[index] = value
		}
	}

	return values, nil
}

func splitPackedPathObject(raw string) ([][2]string, error) {
	const pairWidth = 2

	tokens := strings.Split(raw, ",")
	if len(tokens)%pairWidth != 0 {
		return nil, errors.New("object serialization must contain name/value pairs")
	}

	pairs := make([][2]string, 0, len(tokens)/pairWidth)
	for index := 0; index < len(tokens); index += pairWidth {
		pairs = append(pairs, [2]string{tokens[index], tokens[index+1]})
	}

	return pairs, nil
}

func splitExplodedPathObject(raw string, separator string) ([][2]string, error) {
	terms := strings.Split(raw, separator)

	pairs := make([][2]string, 0, len(terms))
	for _, term := range terms {
		name, value, ok := strings.Cut(term, "=")
		if !ok {
			return nil, fmt.Errorf("object term %q must contain =", term)
		}

		pairs = append(pairs, [2]string{name, value})
	}

	return pairs, nil
}

func encodePathArray(typeName styleScalarType, rawValues []string) (jsontext.Value, error) {
	var output bytes.Buffer

	encoder := jsontext.NewEncoder(&output)
	if err := encoder.WriteToken(jsontext.BeginArray); err != nil {
		return nil, err
	}

	for _, raw := range rawValues {
		decoded, err := decodePathToken(raw)
		if err != nil {
			return nil, err
		}

		value, err := encodePathScalar(typeName, decoded)
		if err != nil {
			return nil, err
		}

		if err := encoder.WriteValue(value); err != nil {
			return nil, err
		}
	}

	if err := encoder.WriteToken(jsontext.EndArray); err != nil {
		return nil, err
	}

	return append(jsontext.Value(nil), bytes.TrimSpace(output.Bytes())...), nil
}

//nolint:cyclop // Decode, policy, type selection, duplicate checks, and encoding meet at this object boundary.
func (parameter *pathParameter) encodePathObjectValue(rawPairs [][2]string) (jsontext.Value, error) {
	var output bytes.Buffer

	encoder := jsontext.NewEncoder(&output)
	if err := encoder.WriteToken(jsontext.BeginObject); err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(rawPairs))
	for _, pair := range rawPairs {
		name, err := decodePathToken(pair[0])
		if err != nil {
			return nil, err
		}

		if name == "" {
			return nil, errors.New("dynamic object key is empty")
		}

		if _, duplicate := seen[name]; duplicate {
			return nil, fmt.Errorf("duplicate object property %q", name)
		}

		seen[name] = struct{}{}

		value, err := decodePathToken(pair[1])
		if err != nil {
			return nil, err
		}

		typeName := parameter.dynamicType
		if propertyIndex, declared := parameter.propertyByName[name]; declared {
			typeName = parameter.properties[propertyIndex].scalarType
		} else if typeName == "" {
			typeName = "string"
		}

		encoded, err := encodePathScalar(typeName, value)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", name, err)
		}

		if err := encoder.WriteToken(jsontext.String(name)); err != nil {
			return nil, err
		}

		if err := encoder.WriteValue(encoded); err != nil {
			return nil, err
		}
	}

	if err := encoder.WriteToken(jsontext.EndObject); err != nil {
		return nil, err
	}

	return append(jsontext.Value(nil), bytes.TrimSpace(output.Bytes())...), nil
}

func decodePathToken(raw string) (string, error) {
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", fmt.Errorf("unescape path token %q: %w", raw, err)
	}

	if !utf8.ValidString(decoded) {
		return "", fmt.Errorf("path token %q is not valid UTF-8", raw)
	}

	return decoded, nil
}

func encodePathScalar(typeName styleScalarType, value string) (jsontext.Value, error) {
	var output bytes.Buffer

	encoder := jsontext.NewEncoder(&output)
	if err := writeScalar(encoder, string(typeName), value, true); err != nil {
		return nil, err
	}

	return append(jsontext.Value(nil), bytes.TrimSpace(output.Bytes())...), nil
}

func encodePathObject(parameters []pathParameter, values []jsontext.Value) (json.RawMessage, error) {
	var output bytes.Buffer

	encoder := jsontext.NewEncoder(&output)
	if err := encoder.WriteToken(jsontext.BeginObject); err != nil {
		return nil, fmt.Errorf("encode path object: %w", err)
	}

	for index, parameter := range parameters {
		if err := encoder.WriteToken(jsontext.String(parameter.name)); err != nil {
			return nil, fmt.Errorf("encode path parameter name %q: %w", parameter.name, err)
		}

		if err := encoder.WriteValue(values[index]); err != nil {
			return nil, fmt.Errorf("encode path parameter %q: %w", parameter.name, err)
		}
	}

	if err := encoder.WriteToken(jsontext.EndObject); err != nil {
		return nil, fmt.Errorf("encode path object: %w", err)
	}

	return append(json.RawMessage(nil), bytes.TrimSpace(output.Bytes())...), nil
}
