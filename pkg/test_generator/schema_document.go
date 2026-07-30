//nolint:cyclop,godoclint // Private deterministic document encoding is explicit.
package testgenerator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

func (number generatedNumber) MarshalJSON() ([]byte, error) {
	raw := []byte(number)
	if !json.Valid(append(append([]byte{'['}, raw...), ']')) {
		return nil, fmt.Errorf("generated number %q is not valid JSON", number)
	}

	return raw, nil
}

func generatedOpenAPIDocument(schema generatedSchemaObject) map[string]any {
	return map[string]any{
		"openapi": "3.0.3",
		"info":    map[string]any{"title": "generated", "version": "1"},
		"paths": map[string]any{
			"/things": map[string]any{
				"post": map[string]any{
					"operationId": "checkThing",
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{"schema": schema},
						},
					},
					"responses": map[string]any{"204": map[string]any{"description": "done"}},
				},
			},
		},
		"components": map[string]any{
			"schemas": map[string]any{
				"Leaf":  generatedSchemaObject{"type": "integer"},
				"Lower": generatedSchemaObject{"type": "integer", "minimum": generatedNumber("-100")},
				"Upper": generatedSchemaObject{"type": "integer", "maximum": generatedNumber("100")},
				"Meet": generatedSchemaObject{"allOf": []any{
					generatedSchemaObject{"$ref": "#/components/schemas/Lower"},
					generatedSchemaObject{"$ref": "#/components/schemas/Upper"},
				}},
				"Chain": generatedSchemaObject{"$ref": "#/components/schemas/Meet"},
				"Choice": generatedSchemaObject{"anyOf": []any{
					generatedSchemaObject{"type": "string"},
					generatedSchemaObject{"type": "integer"},
				}},
				"Container": generatedSchemaObject{
					"type": "object",
					"properties": map[string]any{
						"":  generatedSchemaObject{"type": "boolean"},
						"~": generatedSchemaObject{"type": "string"},
						"/": generatedSchemaObject{"type": "number"},
						"λ": generatedSchemaObject{"$ref": "#/components/schemas/Chain"},
					},
				},
			},
		},
	}
}

func marshalGeneratedDocument(document map[string]any) ([]byte, error) {
	var encoded bytes.Buffer
	if err := encodeGeneratedValue(&encoded, document); err != nil {
		return nil, err
	}

	return encoded.Bytes(), nil
}

func encodeGeneratedValue(encoded *bytes.Buffer, value any) error {
	switch typed := value.(type) {
	case generatedNumber:
		raw, err := typed.MarshalJSON()
		if err != nil {
			return err
		}

		_, err = encoded.Write(raw)

		return err
	case map[string]any:
		return encodeGeneratedMap(encoded, typed)
	case generatedSchemaObject:
		members := make(map[string]any, len(typed))
		for key, child := range typed {
			members[key] = child
		}

		return encodeGeneratedMap(encoded, members)
	case []any:
		return encodeGeneratedSlice(encoded, typed)
	case []generatedSchemaObject:
		values := make([]any, len(typed))
		for index, child := range typed {
			values[index] = child
		}

		return encodeGeneratedSlice(encoded, values)
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return err
		}

		_, err = encoded.Write(raw)

		return err
	}
}

func encodeGeneratedMap(encoded *bytes.Buffer, members map[string]any) error {
	keys := make([]string, 0, len(members))
	for key := range members {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	if err := encoded.WriteByte('{'); err != nil {
		return err
	}

	for index, key := range keys {
		if index != 0 {
			if err := encoded.WriteByte(','); err != nil {
				return err
			}
		}

		keyJSON, err := json.Marshal(key)
		if err != nil {
			return err
		}

		if _, err := encoded.Write(keyJSON); err != nil {
			return err
		}

		if err := encoded.WriteByte(':'); err != nil {
			return err
		}

		if err := encodeGeneratedValue(encoded, members[key]); err != nil {
			return err
		}
	}

	if err := encoded.WriteByte('}'); err != nil {
		return err
	}

	return nil
}

func encodeGeneratedSlice(encoded *bytes.Buffer, values []any) error {
	if err := encoded.WriteByte('['); err != nil {
		return err
	}

	for index, value := range values {
		if index != 0 {
			if err := encoded.WriteByte(','); err != nil {
				return err
			}
		}

		if err := encodeGeneratedValue(encoded, value); err != nil {
			return err
		}
	}

	if err := encoded.WriteByte(']'); err != nil {
		return err
	}

	return nil
}

func cloneGeneratedValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		clone := make(map[string]any, len(typed))
		for key, child := range typed {
			clone[key] = cloneGeneratedValue(child)
		}

		return clone
	case generatedSchemaObject:
		clone := make(generatedSchemaObject, len(typed))
		for key, child := range typed {
			clone[key] = cloneGeneratedValue(child)
		}

		return clone
	case []any:
		clone := make([]any, len(typed))
		for index, child := range typed {
			clone[index] = cloneGeneratedValue(child)
		}

		return clone
	case []generatedSchemaObject:
		clone := make([]generatedSchemaObject, len(typed))
		for index, child := range typed {
			clonedChild, ok := cloneGeneratedValue(child).(generatedSchemaObject)
			if !ok {
				panic("clone generated schema: child is not a schema object")
			}

			clone[index] = clonedChild
		}

		return clone
	case []string:
		return append([]string(nil), typed...)
	default:
		return typed
	}
}
