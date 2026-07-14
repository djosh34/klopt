//nolint:cyclop,godoclint,mnd // Private recursive choices and mutation IDs are clearer inline.
package testgenerator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"pgregory.net/rapid"
)

// GeneratedSchema is one syntactically valid JSON OpenAPI document and its
// expected support by the request-body test generator.
type GeneratedSchema struct {
	OpenAPIJSON []byte
	Valid       bool
}

type generatedSchemaObject map[string]any

type generatedNumber string

func (number generatedNumber) MarshalJSON() ([]byte, error) {
	raw := []byte(number)
	if !json.Valid(append(append([]byte{'['}, raw...), ']')) {
		return nil, fmt.Errorf("generated number %q is not valid JSON", number)
	}

	return raw, nil
}

// GenerateSchemas draws one valid OpenAPI document followed by independently
// mutated invalid copies. It must be called from a Rapid property.
func GenerateSchemas(t *rapid.T) []GeneratedSchema {
	t.Helper()

	root := generatedInlineSchema().Draw(t, "request schema")
	document := generatedOpenAPIDocument(root)
	validJSON := marshalGeneratedDocument(t, document)

	mutationIDs := rapid.SliceOfN(
		rapid.IntRange(0, generatedMutationCount-1),
		1,
		-1,
	).Draw(t, "invalid mutations")

	generated := make([]GeneratedSchema, 1, len(mutationIDs)+1)
	generated[0] = GeneratedSchema{OpenAPIJSON: validJSON, Valid: true}

	for index, mutationID := range mutationIDs {
		mutated, ok := cloneGeneratedValue(document).(map[string]any)
		if !ok {
			t.Fatal("clone generated document: root is not an object")
		}

		if err := mutateGeneratedDocument(mutated, mutationID); err != nil {
			t.Fatalf("apply invalid schema mutation %d: %v", index, err)
		}

		generated = append(generated, GeneratedSchema{
			OpenAPIJSON: marshalGeneratedDocument(t, mutated),
			Valid:       false,
		})
	}

	return generated
}

func generatedSchema() *rapid.Generator[generatedSchemaObject] {
	return rapid.OneOf(
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedReferenceSchema(),
		generatedReferenceSchema(),
		generatedReferenceSchema(),
		generatedReferenceSchema(),
		generatedReferenceSchema(),
		generatedArraySchema(),
		generatedObjectSchema(),
		generatedAllOfSchema(),
		generatedAllOfSchema(),
	)
}

func generatedInlineSchema() *rapid.Generator[generatedSchemaObject] {
	return rapid.OneOf(
		generatedLeafSchema(),
		generatedLeafSchema(),
		generatedArraySchema(),
		generatedObjectSchema(),
		generatedAllOfSchema(),
		generatedAllOfSchema(),
		generatedAllOfSchema(),
	)
}

func generatedLeafSchema() *rapid.Generator[generatedSchemaObject] {
	return rapid.Custom(func(t *rapid.T) generatedSchemaObject {
		schema := make(generatedSchemaObject)
		kind := rapid.IntRange(0, 4).Draw(t, "type")

		if kind != 0 {
			schema["type"] = []string{"", "string", "number", "integer", "boolean"}[kind]
		}

		switch rapid.IntRange(0, 2).Draw(t, "nullable") {
		case 1:
			schema["nullable"] = false
		case 2:
			schema["nullable"] = true
		}

		switch kind {
		case 0:
			addGeneratedTypelessKeywords(t, schema)
		case 1:
			addGeneratedStringKeywords(t, schema)
		case 2:
			addGeneratedNumberKeywords(t, schema, false)
		case 3:
			addGeneratedNumberKeywords(t, schema, true)
		case 4:
			if rapid.Bool().Draw(t, "boolean enum") {
				schema["enum"] = []any{rapid.Bool().Draw(t, "boolean enum value")}
			}
		}

		return schema
	})
}

func addGeneratedTypelessKeywords(t *rapid.T, schema generatedSchemaObject) {
	if rapid.Bool().Draw(t, "typeless string family") {
		addGeneratedStringKeywords(t, schema)
	}

	if rapid.Bool().Draw(t, "typeless number family") {
		addGeneratedNumberKeywords(t, schema, false)
	}

	if rapid.Bool().Draw(t, "typeless array family") {
		schema["minItems"] = rapid.IntRange(0, 2).Draw(t, "typeless minItems")
		schema["maxItems"] = rapid.IntRange(2, 5).Draw(t, "typeless maxItems")
	}

	if rapid.Bool().Draw(t, "typeless object family") {
		schema["minProperties"] = rapid.IntRange(0, 2).Draw(t, "typeless minProperties")
		schema["maxProperties"] = rapid.IntRange(2, 5).Draw(t, "typeless maxProperties")
	}

	if len(schema) == 0 || len(schema) == 1 && schema["nullable"] != nil {
		schema["enum"] = []any{false, generatedNumber("-0"), "", "λ"}
	}

	if _, hasEnum := schema["enum"]; hasEnum {
		delete(schema, "x-valid-examples")
		delete(schema, "x-invalid-examples")
	}
}

func addGeneratedStringKeywords(t *rapid.T, schema generatedSchemaObject) {
	if rapid.Bool().Draw(t, "string enum") {
		schema["enum"] = []any{"", "a", "λ", "line\nfeed"}

		return
	}

	if rapid.Bool().Draw(t, "opaque string") {
		fragment := rapid.SampledFrom(opaqueStringCatalog).Draw(t, "opaque fragment")
		valid := generatedEvidenceSubset(t, fragment.ValidExamples, "valid evidence")
		invalid := generatedEvidenceSubset(t, fragment.InvalidExamples, "invalid evidence")

		schema["pattern"] = fragment.Pattern
		if fragment.Format != "" {
			schema["format"] = fragment.Format
		}

		schema["minLength"] = 1
		schema["maxLength"] = 128
		schema["x-valid-examples"] = valid
		schema["x-invalid-examples"] = invalid

		return
	}

	minimum := rapid.IntRange(0, 4).Draw(t, "minLength")

	maximum := rapid.IntRange(minimum, minimum+6).Draw(t, "maxLength")
	if rapid.Bool().Draw(t, "has minLength") {
		schema["minLength"] = minimum
	}

	if rapid.Bool().Draw(t, "has maxLength") {
		schema["maxLength"] = maximum
	}
}

func generatedEvidenceSubset(t *rapid.T, source []json.RawMessage, label string) []any {
	start := rapid.IntRange(0, len(source)-1).Draw(t, label+" start")
	count := rapid.IntRange(1, min(8, len(source)-start)).Draw(t, label+" count")
	values := make([]any, 0, count)

	for _, raw := range source[start : start+count] {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatalf("decode trusted %s: %v", label, err)
		}

		values = append(values, value)
	}

	return values
}

func addGeneratedNumberKeywords(t *rapid.T, schema generatedSchemaObject, integer bool) {
	if rapid.Bool().Draw(t, "number enum") {
		if integer {
			schema["enum"] = []any{generatedNumber("-0"), generatedNumber("9007199254740993")}
		} else {
			schema["enum"] = []any{
				generatedNumber("-0"), generatedNumber("0.0000000000000000000000000001"),
				generatedNumber("9007199254740993"), generatedNumber("1e400"),
			}
		}

		return
	}

	bounds := []struct {
		minimum generatedNumber
		maximum generatedNumber
	}{
		{minimum: "-100", maximum: "100"},
		{minimum: "-0", maximum: "0.0000000000000000000000000001"},
		{minimum: "9007199254740993", maximum: "9007199254741993"},
		{minimum: "-1e400", maximum: "1e400"},
		{minimum: "1.234567890123456789e-100", maximum: "9.876543210987654321e100"},
	}
	selected := rapid.SampledFrom(bounds).Draw(t, "exact bounds")

	if rapid.Bool().Draw(t, "minimum") {
		schema["minimum"] = selected.minimum
		if rapid.Bool().Draw(t, "exclusiveMinimum") {
			schema["exclusiveMinimum"] = rapid.Bool().Draw(t, "exclusiveMinimum value")
		}
	}

	if rapid.Bool().Draw(t, "maximum") {
		schema["maximum"] = selected.maximum
		if rapid.Bool().Draw(t, "exclusiveMaximum") {
			schema["exclusiveMaximum"] = rapid.Bool().Draw(t, "exclusiveMaximum value")
		}
	}

	if rapid.Bool().Draw(t, "multipleOf") {
		if integer {
			schema["multipleOf"] = generatedNumber("3")
		} else {
			schema["multipleOf"] = rapid.SampledFrom[[]generatedNumber, generatedNumber]([]generatedNumber{
				"0.0000000000000000000000000001", "0.25", "3", "1e300",
			}).Draw(t, "exact multipleOf")
		}
	}
}

func generatedReferenceSchema() *rapid.Generator[generatedSchemaObject] {
	return rapid.Map(rapid.SampledFrom([]string{
		"#/components/schemas/Leaf",
		"#/components/schemas/Chain",
		"#/components/schemas/Meet",
		"#/components/schemas/Container",
		"#/components/schemas/Container/properties/",
		"#/components/schemas/Container/properties/~0",
		"#/components/schemas/Container/properties/~1",
		"#/components/schemas/Container/properties/%CE%BB",
	}), func(reference string) generatedSchemaObject {
		return generatedSchemaObject{
			"$ref":        reference,
			"description": "Reference Object siblings are ignored in OpenAPI 3.0.3",
		}
	})
}

func generatedArraySchema() *rapid.Generator[generatedSchemaObject] {
	return rapid.Custom(func(t *rapid.T) generatedSchemaObject {
		minimum := rapid.IntRange(0, 3).Draw(t, "minItems")

		schema := generatedSchemaObject{
			"type":  "array",
			"items": rapid.Deferred(generatedSchema).Draw(t, "items"),
		}
		if rapid.Bool().Draw(t, "array minimum") {
			schema["minItems"] = minimum
		}

		if rapid.Bool().Draw(t, "array maximum") {
			schema["maxItems"] = rapid.IntRange(minimum, minimum+4).Draw(t, "maxItems")
		}

		if rapid.Bool().Draw(t, "array nullable") {
			schema["nullable"] = rapid.Bool().Draw(t, "array nullable value")
		}

		return schema
	})
}

func generatedObjectSchema() *rapid.Generator[generatedSchemaObject] {
	return rapid.Custom(func(t *rapid.T) generatedSchemaObject {
		children := rapid.SliceOfN(rapid.Deferred(generatedSchema), 0, -1).Draw(t, "properties")
		propertyNames := []string{"", "~", "/", "λ", "plain", "additional0", "a/b", "a~b"}
		properties := make(map[string]any, len(children))
		required := make([]string, 0, len(children))

		for index, child := range children {
			name := propertyNames[index%len(propertyNames)]
			if index >= len(propertyNames) {
				name = fmt.Sprintf("property%d", index)
			}

			properties[name] = child
			if rapid.Bool().Draw(t, "required property") {
				required = append(required, name)
			}
		}

		schema := generatedSchemaObject{"type": "object", "properties": properties}

		switch rapid.IntRange(0, 2).Draw(t, "object nullable") {
		case 1:
			schema["nullable"] = false
		case 2:
			schema["nullable"] = true
		}

		if len(required) != 0 {
			schema["required"] = required
		}

		minimum := rapid.IntRange(0, len(properties)).Draw(t, "minProperties")
		if rapid.Bool().Draw(t, "object minimum") {
			schema["minProperties"] = minimum
		}

		if rapid.Bool().Draw(t, "object maximum") {
			schema["maxProperties"] = rapid.IntRange(max(minimum, len(required)), max(minimum, len(required))+4).
				Draw(t, "maxProperties")
		}

		switch rapid.IntRange(0, 2).Draw(t, "additionalProperties") {
		case 0:
			schema["additionalProperties"] = false
		case 1:
			schema["additionalProperties"] = true
		case 2:
			schema["additionalProperties"] = rapid.Deferred(generatedSchema).Draw(t, "additional schema")
		}

		return schema
	})
}

func generatedAllOfSchema() *rapid.Generator[generatedSchemaObject] {
	return rapid.Custom(func(t *rapid.T) generatedSchemaObject {
		children := rapid.SliceOfN(rapid.Deferred(generatedSchema), 2, -1).Draw(t, "allOf children")
		schema := generatedLeafSchema().Draw(t, "allOf siblings")
		schema["allOf"] = children

		return schema
	})
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

func marshalGeneratedDocument(t *rapid.T, document map[string]any) []byte {
	t.Helper()

	var encoded bytes.Buffer
	if err := encodeGeneratedValue(&encoded, document); err != nil {
		t.Fatalf("marshal generated OpenAPI document: %v", err)
	}

	return encoded.Bytes()
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

const generatedMutationCount = 14

func mutateGeneratedDocument(document map[string]any, mutationID int) error {
	schema, err := generatedRequestSchema(document)
	if err != nil {
		return err
	}

	switch mutationID {
	case 0:
		schema["oneOf"] = []any{generatedSchemaObject{}}
	case 1:
		delete(schema, "pattern")
		delete(schema, "format")
		schema["x-valid-examples"] = []any{"x"}
		schema["allOf"] = []any{generatedSchemaObject{
			"pattern": "^x$", "x-valid-examples": []any{"x"},
		}}
	case 2:
		schema["allOf"] = appendGeneratedAllOf(schema, generatedSchemaObject{
			"$ref": "#/components/schemas/Missing",
		})
	case 3:
		schemas, schemasErr := generatedComponentSchemas(document)
		if schemasErr != nil {
			return schemasErr
		}

		schemas["Cycle"] = generatedSchemaObject{"$ref": "#/components/schemas/Cycle"}
		schema["allOf"] = appendGeneratedAllOf(schema, generatedSchemaObject{
			"$ref": "#/components/schemas/Cycle",
		})
	case 4:
		schemas, schemasErr := generatedComponentSchemas(document)
		if schemasErr != nil {
			return schemasErr
		}

		schemas["CycleA"] = generatedSchemaObject{"$ref": "#/components/schemas/CycleB"}
		schemas["CycleB"] = generatedSchemaObject{"$ref": "#/components/schemas/CycleA"}
		schema["allOf"] = appendGeneratedAllOf(schema, generatedSchemaObject{
			"$ref": "#/components/schemas/CycleA",
		})
	case 5:
		schema["minLength"] = "zero"
	case 6:
		schema["nullable"] = nil
	case 7:
		schema["minItems"] = -1
	case 8:
		schema["allOf"] = []any{}
	case 9:
		schema["allOf"] = appendGeneratedAllOf(schema, generatedSchemaObject{"$ref": "#not-a-pointer"})
	case 10:
		schema["required"] = "property"
	case 11:
		delete(schema, "allOf")
		schema["pattern"] = "^x$"
		schema["x-valid-examples"] = []any{"x"}
		schema["x-invalid-examples"] = []any{"x"}
	case 12:
		schema["allOf"] = appendGeneratedAllOf(schema, generatedSchemaObject{"$ref": "#/info"})
	case 13:
		schema["maxLength"] = nil
	default:
		return fmt.Errorf("unknown mutation %d", mutationID)
	}

	return nil
}

func appendGeneratedAllOf(schema generatedSchemaObject, child generatedSchemaObject) []any {
	children, ok := schema["allOf"].([]generatedSchemaObject)
	if ok {
		result := make([]any, 0, len(children)+1)
		for _, existing := range children {
			result = append(result, existing)
		}

		return append(result, child)
	}

	if children, ok := schema["allOf"].([]any); ok {
		return append(append([]any(nil), children...), child)
	}

	return []any{child}
}

func generatedRequestSchema(document map[string]any) (generatedSchemaObject, error) {
	paths, ok := document["paths"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("paths")
	}

	path, ok := paths["/things"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("paths./things")
	}

	post, ok := path["post"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("paths./things.post")
	}

	requestBody, ok := post["requestBody"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("paths./things.post.requestBody")
	}

	content, ok := requestBody["content"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("paths./things.post.requestBody.content")
	}

	mediaType, ok := content["application/json"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("paths./things.post.requestBody.content.application/json")
	}

	schema, ok := mediaType["schema"].(generatedSchemaObject)
	if !ok {
		return nil, errorsForGeneratedPath("paths./things.post.requestBody.content.application/json.schema")
	}

	return schema, nil
}

func generatedComponentSchemas(document map[string]any) (map[string]any, error) {
	components, ok := document["components"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("components")
	}

	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("components.schemas")
	}

	return schemas, nil
}

func errorsForGeneratedPath(path string) error {
	return fmt.Errorf("generated document path %s has the wrong shape", path)
}
