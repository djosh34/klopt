//nolint:godoclint // Behavior test names are the lowering specifications.
package suite

import (
	"fmt"
	"strings"
	"testing"

	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/stretchr/testify/require"
)

func TestLoweringMatchesExactStringPatternFormatAndLengthMeaning(t *testing.T) {
	t.Parallel()

	pattern := compileSchemaForTest(t, `{
		"type":"string","minLength":2,"maxLength":3,"pattern":"^a+$","format":"password"
	}`, "")
	for _, test := range []struct {
		literal string
		want    bool
	}{
		{literal: `"aa"`, want: true},
		{literal: `"aaa"`, want: true},
		{literal: `"a"`},
		{literal: `"aaaa"`},
		{literal: `"ab"`},
		{literal: `7`},
	} {
		require.Equal(
			t,
			test.want,
			caseContains(t, pattern, "valid root", mustJSONValue(t, test.literal)),
			test.literal,
		)
	}

	format := compileSchemaForTest(t, `{"type":"string","format":"uuid"}`, "")
	require.True(t, caseContains(
		t,
		format,
		"valid root",
		mustJSONValue(t, `"a1234567-1234-4234-9234-123456789abc"`),
	))
	require.False(t, caseContains(t, format, "valid root", mustJSONValue(t, `"not-a-uuid"`)))
}

func TestLoweringMatchesExactNumericBoundMultipleAndFormatMeaning(t *testing.T) {
	t.Parallel()

	compiled := compileSchemaForTest(t, `{
		"type":"integer","minimum":0,"exclusiveMinimum":true,
		"maximum":5,"multipleOf":1.5,"format":"int32"
	}`, "")

	for _, test := range []struct {
		literal string
		want    bool
	}{
		{literal: `3`, want: true},
		{literal: `0`},
		{literal: `1.5`},
		{literal: `6`},
		{literal: `"3"`},
	} {
		require.Equal(
			t,
			test.want,
			caseContains(t, compiled, "valid root", mustJSONValue(t, test.literal)),
			test.literal,
		)
	}
}

func TestLoweringMatchesContainerRequiredPropertyAndAdditionalPropertyMeaning(t *testing.T) {
	t.Parallel()

	compiled := compileSchemaForTest(t, `{
		"type":"object","minProperties":1,"maxProperties":2,"required":["id"],
		"properties":{"id":{"type":"integer"}},
		"additionalProperties":{"type":"array","minItems":1,"items":{"type":"boolean"}}
	}`, "")

	for _, test := range []struct {
		literal string
		want    bool
	}{
		{literal: `{"id":1}`, want: true},
		{literal: `{"id":1,"flags":[true,false]}`, want: true},
		{literal: `{}`},
		{literal: `{"id":"1"}`},
		{literal: `{"id":1,"flags":[]}`},
		{literal: `{"id":1,"a":[true],"b":[false]}`},
	} {
		require.Equal(
			t,
			test.want,
			caseContains(t, compiled, "valid root", mustJSONValue(t, test.literal)),
			test.literal,
		)
	}
}

func TestLoweringAppliesAdditionalPropertySchemasToAllOfSiblingProperties(t *testing.T) {
	t.Parallel()

	impossible := compileSchemaForTest(t, `{
		"allOf":[
			{"type":"object","additionalProperties":{"type":"string"}},
			{"type":"object","required":["count"],"properties":{"count":{"type":"integer"}}}
		]
	}`, "")
	requireNoAcceptedCases(t, impossible)

	compatible := compileSchemaForTest(t, `{
		"allOf":[
			{"type":"object","additionalProperties":{"type":"string"}},
			{"type":"object","required":["label"],"properties":{"label":{"type":"string"}}}
		]
	}`, "")
	require.True(t, caseContains(t, compatible, "valid root", mustJSONValue(t, `{"label":"ok"}`)))
	require.False(t, caseContains(t, compatible, "valid root", mustJSONValue(t, `{"label":1}`)))
}

func TestLoweringAppliesNullableRequestDirectionAndReferenceObjectRules(t *testing.T) {
	t.Parallel()

	nullable := compileSchemaForTest(t, `{
		"type":"string","nullable":true
	}`, "")
	require.True(t, caseContains(t, nullable, "valid root", jsonvalue.Null()))
	require.True(t, caseContains(t, nullable, "valid root", jsonvalue.String("x")))
	require.False(t, caseContains(t, nullable, "valid root", jsonvalue.Bool(false)))

	enumerated := compileSchemaForTest(t, `{
		"type":"string","nullable":true,"enum":["x"]
	}`, "")
	require.False(t, caseContains(t, enumerated, "valid root", jsonvalue.Null()))

	readOnly := compileSchemaForTest(t, `{
		"type":"object","required":["server"],
		"properties":{"server":{"type":"string","readOnly":true}}
	}`, "")
	require.True(t, caseContains(t, readOnly, "valid root", mustJSONValue(t, `{}`)))

	reference := compileSchemaForTest(
		t,
		`{"$ref":"#/components/schemas/One","enum":[2]}`,
		`,"components":{"schemas":{"One":{"enum":[1]}}}`,
	)
	require.True(t, caseContains(t, reference, "valid root", mustJSONValue(t, `1`)))
	require.False(t, caseContains(t, reference, "valid root", mustJSONValue(t, `2`)))
}

func TestFocusedInvalidCannotBeRescuedByAnOuterAnyOfBranch(t *testing.T) {
	t.Parallel()

	compiled := compileSchemaForTest(t, `{
		"anyOf":[
			{"type":"object","required":["x"],"properties":{"x":{
				"anyOf":[{"type":"string"},{"type":"number"}]
			}}},
			{"type":"object","required":["fallback"]}
		]
	}`, "")

	inner := findCaseAtPointer(t, compiled, "/properties/x/anyOf", ExpectRejected)
	rejected := mustJSONValue(t, `{"x":false}`)
	rescued := mustJSONValue(t, `{"x":false,"fallback":true}`)

	matches, err := compiled.Semantic.Sets.Contains(inner.values, rejected)
	require.NoError(t, err)
	require.True(t, matches)
	matches, err = compiled.Semantic.Sets.Contains(inner.values, rescued)
	require.NoError(t, err)
	require.False(t, matches)
}

func TestLoweringHasNoAuthoredAnyOfBranchCap(t *testing.T) {
	t.Parallel()

	branches := make([]string, 300)
	for index := range branches {
		branches[index] = fmt.Sprintf(`{"enum":[%d]}`, index)
	}

	compiled := compileSchemaForTest(t, `{"anyOf":[`+strings.Join(branches, ",")+`]}`, "")
	require.Len(t, compiled.Semantic.Occurrences[compiled.Semantic.Root].AnyOf, 300)
	require.False(t, caseContains(t, compiled, "valid root", jsonvalue.String("not enumerated")))
	require.True(t, caseContains(t, compiled, "valid root", mustJSONValue(t, `299`)))
}

func TestExactScalarProductivityOmitsImpossibleStringAndIntegerRoots(t *testing.T) {
	t.Parallel()

	strings := compileSchemaForTest(t, `{
		"type":"string","allOf":[{"pattern":"^a$"},{"pattern":"^b$"}]
	}`, "")
	requireNoAcceptedCases(t, strings)

	numbers := compileSchemaForTest(t, `{
		"type":"integer","minimum":0,"exclusiveMinimum":true,
		"maximum":1,"exclusiveMaximum":true
	}`, "")
	requireNoAcceptedCases(t, numbers)
}

func TestExactContainerProductivityOmitsImpossibleArrayAndObjectRoots(t *testing.T) {
	t.Parallel()

	array := compileSchemaForTest(t, `{
		"type":"array","minItems":2,"maxItems":1,"items":{}
	}`, "")
	requireNoAcceptedCases(t, array)

	object := compileSchemaForTest(t, `{
		"type":"object","required":["x"],"additionalProperties":false,"properties":{}
	}`, "")
	requireNoAcceptedCases(t, object)
}

func TestOrdinaryBoundaryAndFocusedInvalidCasesKeepSiblingConstraints(t *testing.T) {
	t.Parallel()

	compiled := compileSchemaForTest(t, `{
		"type":"integer","minimum":1,"maximum":2
	}`, "")

	minimumValid := findCaseAtPointer(t, compiled, "/minimum", ExpectAccepted)
	minimumInvalid := findCaseAtPointer(t, compiled, "/minimum", ExpectRejected)
	maximumInvalid := findCaseAtPointer(t, compiled, "/maximum", ExpectRejected)

	for _, test := range []struct {
		item    caseSpec
		literal string
		want    bool
	}{
		{item: minimumValid, literal: `1`, want: true},
		{item: minimumValid, literal: `2`},
		{item: minimumInvalid, literal: `0`, want: true},
		{item: minimumInvalid, literal: `3`},
		{item: maximumInvalid, literal: `3`, want: true},
		{item: maximumInvalid, literal: `0`},
	} {
		matches, err := compiled.Semantic.Sets.Contains(test.item.values, mustJSONValue(t, test.literal))
		require.NoError(t, err)
		require.Equal(t, test.want, matches, test.literal)
	}
}

func TestOrdinaryFocusedInvalidIsOmittedWhenConstraintIsExactlyRedundant(t *testing.T) {
	t.Parallel()

	compiled := compileSchemaForTest(t, `{
		"type":"integer","multipleOf":1
	}`, "")
	for _, item := range compiled.Cases {
		if item.source.Keyword == "multipleOf" {
			require.NotEqual(t, ExpectRejected, item.expect, item.name)
		}
	}
}

func findCaseAtPointer(
	t *testing.T,
	compiled *semanticCompilation,
	pointerSuffix string,
	expect ExpectedResult,
) caseSpec {
	t.Helper()

	for _, item := range compiled.Cases {
		if item.expect == expect && strings.HasSuffix(item.source.Pointer, pointerSuffix) {
			return item
		}
	}

	t.Fatalf("case at pointer suffix %q with expectation %d not found", pointerSuffix, expect)

	return caseSpec{}
}

func requireNoAcceptedCases(t *testing.T, compiled *semanticCompilation) {
	t.Helper()

	for _, item := range compiled.Cases {
		require.NotEqual(t, ExpectAccepted, item.expect, item.name)
	}
}
