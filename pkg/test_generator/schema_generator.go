//nolint:cyclop,godoclint,mnd // Private recursive schema choices are clearer inline.
package testgenerator

import (
	"fmt"

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

// GenerateSchemas draws one valid OpenAPI document followed by independently
// mutated invalid copies. It must be called from a Rapid property.
func GenerateSchemas(t *rapid.T) []GeneratedSchema {
	t.Helper()

	root := generatedInlineSchema().Draw(t, "request schema")
	document := generatedOpenAPIDocument(root)

	validJSON, err := marshalGeneratedDocument(document)
	if err != nil {
		t.Fatalf("marshal valid generated OpenAPI document: %v", err)
	}

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

		invalidJSON, err := marshalGeneratedDocument(mutated)
		if err != nil {
			t.Fatalf("marshal invalid generated OpenAPI document %d: %v", index, err)
		}

		generated = append(generated, GeneratedSchema{
			OpenAPIJSON: invalidJSON,
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
		generatedAnyOfSchema(),
		generatedAnyOfSchema(),
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
		generatedAnyOfSchema(),
		generatedAnyOfSchema(),
		generatedAnyOfSchema(),
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
}

func addGeneratedStringKeywords(t *rapid.T, schema generatedSchemaObject) {
	if rapid.Bool().Draw(t, "string enum") {
		schema["enum"] = []any{"", "a", "λ", "line\nfeed"}

		return
	}

	if rapid.Bool().Draw(t, "opaque string") {
		fragment := rapid.SampledFrom(opaqueStringCatalog).Draw(t, "opaque fragment")

		schema["pattern"] = fragment.Pattern
		if fragment.Format != "" {
			schema["format"] = fragment.Format
		}

		schema["minLength"] = 1
		schema["maxLength"] = 128

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
		"#/components/schemas/Choice",
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

func generatedAnyOfSchema() *rapid.Generator[generatedSchemaObject] {
	return rapid.Custom(func(t *rapid.T) generatedSchemaObject {
		children := rapid.SliceOfN(rapid.Deferred(generatedSchema), 2, 3).Draw(t, "anyOf children")
		schema := generatedLeafSchema().Draw(t, "anyOf siblings")
		schema["anyOf"] = children

		return schema
	})
}
