//nolint:godoclint // Composition failure identity is pinned at the private oracle seam.
package schematest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluateAnyOfAggregateFailureHasParentIdentityAfterBranchFailures(t *testing.T) {
	t.Parallel()

	result := evaluateSchemaValue(
		t,
		`{"anyOf":[{"type":"string","pattern":"^a"},{"type":"number","minimum":2}]}`,
		`1`,
	)

	require.NoError(t, result.err)
	require.False(t, result.valid)
	require.Equal(
		t,
		[]string{
			"#/paths/~1/post/requestBody/content/application~1json/schema/anyOf/0|#|type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/anyOf/1|#|minimum",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|anyOf",
		},
		identityStrings(result.failures),
	)
}

func TestEvaluateRepeatedReferencedCompositionBranchesKeepDistinctUseSiteIdentities(t *testing.T) {
	t.Parallel()

	document := `{
		"openapi":"3.0.4",
		"components":{"schemas":{"Shared":{"allOf":[{"type":"string"},{"type":"number","minimum":2}]}}},
		"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{"allOf":[
			{"$ref":"#/components/schemas/Shared"},
			{"$ref":"#/components/schemas/Shared"}
		]}}}}}}}
	}`
	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)

	value, err := parseStrictJSON([]byte(`1`))
	require.NoError(t, err)

	result := evaluate(model, value)
	require.NoError(t, result.err)
	require.False(t, result.valid)
	require.Equal(
		t,
		[]string{
			"#/paths/~1/post/requestBody/content/application~1json/schema/allOf/0/allOf/0|#|type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/allOf/0/allOf/1|#|minimum",
			"#/paths/~1/post/requestBody/content/application~1json/schema/allOf/1/allOf/0|#|type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/allOf/1/allOf/1|#|minimum",
		},
		identityStrings(result.failures),
	)
	require.Equal(
		t,
		[]string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|allOf",
			"#/paths/~1/post/requestBody/content/application~1json/schema/allOf/0|#|allOf",
			"#/paths/~1/post/requestBody/content/application~1json/schema/allOf/1|#|allOf",
		},
		compositionIdentityStrings(result.allOf),
	)
	require.Equal(
		t,
		[]string{
			"#/components/schemas/Shared",
			"#/components/schemas/Shared",
		},
		[]string{
			model.root.allOf[0].occurrence.targetPointer,
			model.root.allOf[1].occurrence.targetPointer,
		},
	)
}

func TestEvaluateSharedYAMLCompositionDAGDeterministicallyWithoutTreeExpansion(t *testing.T) {
	t.Parallel()

	document := sharedYAMLCompositionDocument(24)
	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)

	value, err := parseStrictJSON([]byte(`"text"`))
	require.NoError(t, err)

	first := evaluate(model, value)
	second := evaluate(model, value)

	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.True(t, first.valid)
	require.Equal(t, first, second)
	require.Len(t, first.applicable, 25)
	require.Len(t, first.allOf, 24)
	require.Equal(t, (1<<25)-1, first.records.applicable.count)
	require.Equal(t, (1<<24)-1, first.records.allOf.count)
	require.Empty(t, first.anyOf)
	require.Empty(t, first.failures)

	expectedTruth := make([][]bool, 24)
	for index := range expectedTruth {
		expectedTruth[index] = []bool{true, true}
	}

	require.Equal(t, expectedTruth, compositionTruthVectorsForTest(first.allOf))

	firstTruth, ok := first.records.allOf.at(0)
	require.True(t, ok)
	secondTruth, ok := second.records.allOf.at(0)
	require.True(t, ok)
	require.Equal(t, firstTruth, secondTruth)

	lastTruth, ok := first.records.allOf.at(first.records.allOf.count - 1)
	require.True(t, ok)
	require.Equal(
		t,
		"#/paths/~1/post/requestBody/content/application~1json/schema"+
			strings.Repeat("/allOf/1", 23)+"|#|allOf",
		lastTruth.String(),
	)
	require.Equal(
		t,
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|allOf",
		firstTruth.String(),
	)

	invalidValue, err := parseStrictJSON([]byte(`1`))
	require.NoError(t, err)

	firstInvalid := evaluate(model, invalidValue)
	secondInvalid := evaluate(model, invalidValue)

	require.NoError(t, firstInvalid.err)
	require.NoError(t, secondInvalid.err)
	require.False(t, firstInvalid.valid)
	require.Equal(t, firstInvalid, secondInvalid)
	require.Equal(t, 1<<24, firstInvalid.records.failures.count)

	firstFailure, ok := firstInvalid.records.failures.at(0)
	require.True(t, ok)
	lastFailure, ok := firstInvalid.records.failures.at(firstInvalid.records.failures.count - 1)
	require.True(t, ok)
	require.Equal(
		t,
		"#/paths/~1/post/requestBody/content/application~1json/schema"+
			strings.Repeat("/allOf/0", 24)+"|#|type",
		firstFailure.String(),
	)
	require.Equal(
		t,
		"#/paths/~1/post/requestBody/content/application~1json/schema"+
			strings.Repeat("/allOf/1", 24)+"|#|type",
		lastFailure.String(),
	)
}

func sharedYAMLCompositionDocument(depth int) string {
	anchors := "x-schema-0: &s0 {type: string}\n"
	for index := 1; index <= depth; index++ {
		anchors += fmt.Sprintf("x-schema-%d: &s%d {allOf: [*s%d, *s%d]}\n", index, index, index-1, index-1)
	}

	return "openapi: 3.0.4\n" + anchors + fmt.Sprintf(`paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema: *s%d
`, depth)
}

func compositionIdentityStrings(values []compositionTruth) []string {
	if len(values) == 0 {
		return nil
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}

	return result
}
