//nolint:godoclint // Behavior test names are the semantic compilation specifications.
package suite

import (
	"fmt"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/validation"
	"github.com/stretchr/testify/require"
)

func TestCompileSemanticPlansSimpleAnyOfFromExactRootMeaning(t *testing.T) {
	t.Parallel()

	compiled := compileSchemaForTest(t, `{
		"type":"object",
		"required":["x"],
		"additionalProperties":false,
		"properties":{"x":{
			"enum":[1,2,3,"ok","no"],
			"anyOf":[
				{"type":"number","enum":[1,2]},
				{"type":"string","enum":["ok"]}
			]
		}}
	}`, "")

	require.Equal(t, []caseMeaning{
		{name: "valid root", expect: ExpectAccepted, keyword: "schema"},
		{name: "valid anyOf", expect: ExpectAccepted, keyword: "anyOf"},
		{name: "invalid anyOf", expect: ExpectRejected, keyword: "anyOf"},
	}, caseMeanings(compiled.Cases))

	for _, literal := range []struct {
		body      string
		validRoot bool
		invalid   bool
	}{
		{body: `{"x":1}`, validRoot: true},
		{body: `{"x":"ok"}`, validRoot: true},
		{body: `{"x":3}`, invalid: true},
		{body: `{"x":"no"}`, invalid: true},
		{body: `{}`},
	} {
		value := mustJSONValue(t, literal.body)
		require.Equal(t, literal.validRoot, caseContains(t, compiled, "valid root", value), literal.body)
		require.Equal(t, literal.invalid, caseContains(t, compiled, "invalid anyOf", value), literal.body)
	}

	require.Contains(
		t,
		findCaseAtPointer(t, compiled, "/properties/x/anyOf", ExpectAccepted).source.Pointer,
		"/properties/x/anyOf",
	)
}

func TestCompileSemanticRebuildsNestedPropertyAndItemsOccurrenceContexts(t *testing.T) {
	t.Parallel()

	compiled := compileSchemaForTest(t, `{
		"type":"object",
		"additionalProperties":false,
		"properties":{"rows":{
			"type":"array",
			"items":{"anyOf":[{"type":"string"},{"type":"number"}]}
		}}
	}`, "")

	require.True(t, caseContains(t, compiled, "valid anyOf", mustJSONValue(t, `{"rows":["x"]}`)))
	require.False(t, caseContains(t, compiled, "valid anyOf", mustJSONValue(t, `{}`)))
	require.True(t, caseContains(t, compiled, "invalid anyOf", mustJSONValue(t, `{"rows":[false]}`)))
	require.False(t, caseContains(t, compiled, "invalid anyOf", mustJSONValue(t, `{"rows":[]}`)))
	require.False(t, caseContains(t, compiled, "invalid anyOf", mustJSONValue(t, `{"rows":[false],"extra":1}`)))
}

func TestCompileSemanticPreservesAllOfSiblingsAndLocalReferenceUses(t *testing.T) {
	t.Parallel()

	compiled := compileSchemaForTest(t, `{
		"type":"object",
		"required":["value"],
		"properties":{"value":{"$ref":"#/components/schemas/Choice"}}
	}`, `,"components":{"schemas":{"Choice":{
		"enum":[1,2,3],
		"allOf":[{"minimum":1}],
		"anyOf":[{"enum":[1]},{"enum":[2]}]
	}}}`)

	require.True(t, caseContains(t, compiled, "valid root", mustJSONValue(t, `{"value":2}`)))
	require.True(t, caseContains(t, compiled, "invalid anyOf", mustJSONValue(t, `{"value":3}`)))
	require.False(t, caseContains(t, compiled, "invalid anyOf", mustJSONValue(t, `{"value":0}`)))
	require.Contains(t, findCase(t, compiled, "invalid anyOf").source.Pointer, "/components/schemas/Choice/anyOf")
}

func TestCompileSemanticOmitsOnlyExactlyEmptyAnyOfObligations(t *testing.T) {
	t.Parallel()

	subsumed := compileSchemaForTest(t, `{"anyOf":[{}, {"type":"string"}]}`, "")
	require.Equal(t, []caseMeaning{
		{name: "valid root", expect: ExpectAccepted, keyword: "schema"},
		{name: "valid anyOf", expect: ExpectAccepted, keyword: "anyOf"},
	}, caseMeanings(subsumed.Cases))

	impossible := compileSchemaForTest(t, `{
		"anyOf":[
			{"type":"string","enum":[1]},
			{"type":"number","enum":["x"]}
		]
	}`, "")
	require.Equal(t, []caseMeaning{
		{name: "invalid anyOf", expect: ExpectRejected, keyword: "anyOf"},
	}, caseMeanings(impossible.Cases))
}

type caseMeaning struct {
	name    string
	expect  ExpectedResult
	keyword string
}

func caseMeanings(cases []caseSpec) []caseMeaning {
	result := make([]caseMeaning, 0, len(cases))
	for _, item := range cases {
		if item.source.Keyword != "schema" && item.source.Keyword != "anyOf" {
			continue
		}

		name := item.name
		if item.source.Keyword == "anyOf" {
			if item.expect == ExpectAccepted {
				name = "valid anyOf"
			} else {
				name = "invalid anyOf"
			}
		}

		result = append(result, caseMeaning{name: name, expect: item.expect, keyword: item.source.Keyword})
	}

	return result
}

func compileSchemaForTest(t *testing.T, schema string, components string) *semanticCompilation {
	t.Helper()

	spec := []byte(fmt.Sprintf(`{
		"openapi":"3.0.3",
		"info":{"title":"test","version":"1"},
		"paths":{"/things":{"post":{
			"operationId":"checkThing",
			"requestBody":{"content":{"application/json":{"schema":%s}}},
			"responses":{"204":{"description":"done"}}
		}}}%s
	}`, schema, components))

	sources, err := oas.Parse(spec)
	require.NoError(t, err)

	source := sources["checkThing"]
	admitted, err := validation.AdmitRequestSchema(
		source,
		source.RequestSchema,
		validation.UseRequestGeneration,
	)
	require.NoError(t, err)

	compiled, err := compileSemantic(admitted)
	require.NoError(t, err)

	return compiled
}

func mustJSONValue(t *testing.T, literal string) jsonvalue.Value {
	t.Helper()

	value, err := jsonvalue.Parse([]byte(literal))
	require.NoError(t, err)

	return value
}

func caseContains(t *testing.T, compiled *semanticCompilation, prefix string, value jsonvalue.Value) bool {
	t.Helper()

	item := findCase(t, compiled, prefix)
	matches, err := compiled.Semantic.Sets.Contains(item.values, value)
	require.NoError(t, err)

	return matches
}

func findCase(t *testing.T, compiled *semanticCompilation, prefix string) caseSpec {
	t.Helper()

	for _, item := range compiled.Cases {
		if len(item.name) >= len(prefix) && item.name[:len(prefix)] == prefix {
			return item
		}
	}

	t.Fatalf("case with prefix %q not found", prefix)

	return caseSpec{}
}
