//nolint:godoclint // Composition failure identity is pinned at the private oracle seam.
package schematest

import (
	"fmt"
	"iter"
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
		identityStrings(result.failureRecords()),
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
		identityStrings(result.failureRecords()),
	)
	require.Equal(
		t,
		[]string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|allOf",
			"#/paths/~1/post/requestBody/content/application~1json/schema/allOf/0|#|allOf",
			"#/paths/~1/post/requestBody/content/application~1json/schema/allOf/1|#|allOf",
		},
		compositionIdentityStrings(result.compositionRecords(oracleRuleAllOf)),
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

func TestEvaluateExpandedYAMLCompositionDeterministically(t *testing.T) {
	t.Parallel()

	const depth = 8

	document := sharedYAMLCompositionDocument(depth)
	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)

	value, err := parseStrictJSON([]byte(`"text"`))
	require.NoError(t, err)

	first := evaluate(model, value)
	second := evaluate(model, value)

	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.True(t, first.valid)
	require.Equal(t, evaluationRecordStrings(first.records), evaluationRecordStrings(second.records))
	require.Equal(t, (1<<(depth+1))-1, evaluationRecordSequenceCount(first.applicableRecords()))
	require.Equal(t, (1<<depth)-1, evaluationRecordSequenceCount(first.compositionRecords(oracleRuleAllOf)))
	require.True(t, evaluationRecordSequenceEmpty(first.compositionRecords(oracleRuleAnyOf)))
	require.True(t, evaluationRecordSequenceEmpty(first.failureRecords()))

	expectedTruth := make([][]bool, (1<<depth)-1)
	for index := range expectedTruth {
		expectedTruth[index] = []bool{true, true}
	}

	require.Equal(t, expectedTruth, compositionTruthVectorsForTest(first.compositionRecords(oracleRuleAllOf)))

	firstTruth, ok := evaluationRecordSequenceAt(first.compositionRecords(oracleRuleAllOf), 0)
	require.True(t, ok)
	secondTruth, ok := evaluationRecordSequenceAt(second.compositionRecords(oracleRuleAllOf), 0)
	require.True(t, ok)
	require.Equal(t, firstTruth, secondTruth)

	allOfRecords := first.compositionRecords(oracleRuleAllOf)
	lastTruth, ok := evaluationRecordSequenceAt(
		allOfRecords,
		evaluationRecordSequenceCount(allOfRecords)-1,
	)
	require.True(t, ok)
	require.Equal(
		t,
		"#/paths/~1/post/requestBody/content/application~1json/schema"+
			strings.Repeat("/allOf/1", depth-1)+"|#|allOf",
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
	require.Equal(t, evaluationRecordStrings(firstInvalid.records), evaluationRecordStrings(secondInvalid.records))
	require.Equal(t, 1<<depth, evaluationRecordSequenceCount(firstInvalid.failureRecords()))

	firstFailure, ok := evaluationRecordSequenceAt(firstInvalid.failureRecords(), 0)
	require.True(t, ok)

	failureRecords := firstInvalid.failureRecords()
	lastFailure, ok := evaluationRecordSequenceAt(
		failureRecords,
		evaluationRecordSequenceCount(failureRecords)-1,
	)
	require.True(t, ok)
	require.Equal(
		t,
		"#/paths/~1/post/requestBody/content/application~1json/schema"+
			strings.Repeat("/allOf/0", depth)+"|#|type",
		firstFailure.String(),
	)
	require.Equal(
		t,
		"#/paths/~1/post/requestBody/content/application~1json/schema"+
			strings.Repeat("/allOf/1", depth)+"|#|type",
		lastFailure.String(),
	)
}

func TestCompositionTruthQueriesAreMutationIsolatedAcrossCachedViews(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{
		OpenAPI:     []byte(sharedYAMLCompositionDocument(3)),
		OperationID: "selected",
	})
	require.NoError(t, err)
	value, err := parseStrictJSON([]byte(`"text"`))
	require.NoError(t, err)

	result := evaluate(model, value)
	require.NoError(t, result.err)
	before := compositionTruthVectorsForTest(result.compositionRecords(oracleRuleAllOf))
	truth, ok := evaluationRecordSequenceAt(result.compositionRecords(oracleRuleAllOf), 0)
	require.True(t, ok)

	truth.branches[0] = false

	result.records.forEach(func(record evaluationRecord) bool {
		if record.kind == evaluationRecordComposition {
			record.branches[0] = false
		}

		return true
	})

	require.Equal(t, before, compositionTruthVectorsForTest(result.compositionRecords(oracleRuleAllOf)))

	again := evaluate(model, value)
	require.NoError(t, again.err)
	require.Equal(t, before, compositionTruthVectorsForTest(again.compositionRecords(oracleRuleAllOf)))
	require.Equal(t, evaluationRecordStrings(result.records), evaluationRecordStrings(again.records))
}

func TestFilteredPersistentCompositionRetainsConstantTimeCounts(t *testing.T) {
	t.Parallel()

	child := newEvaluationRecords()
	child.append(makeEvaluationRecord(evaluationRecordApplicable, makeRuleIdentity(
		schemaOccurrence{usePointer: "#/leaf", targetPointer: "#/leaf", instanceTemplate: "#"},
		oracleRuleType,
	)))
	child.append(makeEvaluationRecord(evaluationRecordFailure, makeRuleIdentity(
		schemaOccurrence{usePointer: "#/leaf", targetPointer: "#/leaf", instanceTemplate: "#"},
		oracleRuleType,
	)))

	const depth = 20
	for range depth {
		parent := newEvaluationRecords()
		parent.appendNonFailures(child)
		parent.appendNonFailures(child)
		child = parent
	}

	require.Equal(t, 1<<depth, child.count)
	require.Equal(t, child.count, child.nonFailureCount)
	require.Len(t, child.parts, 2)
	require.Same(t, child.parts[0].nested, child.parts[1].nested)
	require.Equal(t, depth+1, physicalEvaluationRecordNodeCount(child, make(map[*evaluationRecords]bool)))
}

func physicalEvaluationRecordNodeCount(records *evaluationRecords, seen map[*evaluationRecords]bool) int {
	if records == nil || seen[records] {
		return 0
	}

	seen[records] = true

	count := 1
	for _, part := range records.parts {
		count += physicalEvaluationRecordNodeCount(part.nested, seen)
	}

	return count
}

func TestEvaluateExpandedYAMLCompositionRejectingLeafRemainsInvalid(t *testing.T) {
	t.Parallel()

	const depth = 10

	model, err := parseInput(Input{
		OpenAPI:     []byte(sharedYAMLCompositionDocument(depth)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	value, err := parseStrictJSON([]byte(`1`))
	require.NoError(t, err)

	first := evaluate(model, value)
	second := evaluate(model, value)

	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.False(t, first.valid)
	require.Equal(t, evaluationRecordStrings(first.records), evaluationRecordStrings(second.records))
	require.Equal(t, (1<<(depth+1))-1, evaluationRecordSequenceCount(first.applicableRecords()))
	require.Equal(t, (1<<depth)-1, evaluationRecordSequenceCount(first.compositionRecords(oracleRuleAllOf)))
	require.False(t, evaluationRecordSequenceEmpty(first.failureRecords()))

	failure, ok := evaluationRecordSequenceAt(first.failureRecords(), 0)
	require.True(t, ok)
	require.Equal(
		t,
		"#/paths/~1/post/requestBody/content/application~1json/schema"+
			strings.Repeat("/allOf/0", depth)+"|#|type",
		failure.String(),
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

func compositionIdentityStrings(values iter.Seq[compositionTruth]) []string {
	var result []string
	for value := range values {
		result = append(result, value.String())
	}

	return result
}
