//nolint:godoclint // Focused private record-sequence tests use behavior names.
package schematest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluationRecordsPreserveCanonicalFactOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		value  string
		want   []string
	}{
		{
			name:   "primitive",
			schema: `{"type":"string","minLength":1}`,
			value:  `"x"`,
			want: []string{
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type|string",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#|minLength",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#|minLength|valid",
			},
		},
		{
			name:   "array",
			schema: `{"type":"array","minItems":1,"items":{"type":"number"}}`,
			value:  `[1]`,
			want: []string{
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type|array",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#|minItems",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#|minItems|valid",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema/items|#/0|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema/items|#/0|type|number",
			},
		},
		{
			name:   "object",
			schema: `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`,
			value:  `{"name":"x"}`,
			want: []string{
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type|object",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#/name|required",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#/name|required|present",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema/properties/name|#/name|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema/properties/name|#/name|type|string",
			},
		},
		{
			name:   "allOf",
			schema: `{"allOf":[{"type":"string"},{"minLength":1}]}`,
			value:  `"x"`,
			want: []string{
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type|string",
				"allOf|#/paths/~1items/post/requestBody/content/application~1json/schema|#|allOf|true,true",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema/allOf/0|#|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema/allOf/0|#|type|string",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema/allOf/1|#|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema/allOf/1|#|type|string",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema/allOf/1|#|minLength",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema/allOf/1|#|minLength|valid",
			},
		},
		{
			name:   "anyOf",
			schema: `{"anyOf":[{"type":"string"},{"type":"number"}]}`,
			value:  `true`,
			want: []string{
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type|boolean",
				"anyOf|#/paths/~1items/post/requestBody/content/application~1json/schema|#|anyOf|false,false",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema/anyOf/0|#|type",
				"failure|#/paths/~1items/post/requestBody/content/application~1json/schema/anyOf/0|#|type",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema/anyOf/1|#|type",
				"failure|#/paths/~1items/post/requestBody/content/application~1json/schema/anyOf/1|#|type",
				"failure|#/paths/~1items/post/requestBody/content/application~1json/schema|#|anyOf",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, test.schema, test.value)
			require.NoError(t, result.err)

			for index := range test.want {
				test.want[index] = strings.Replace(test.want[index], "#/paths/~1items/", "#/paths/~1/", 1)
			}

			require.Equal(t, test.want, evaluationRecordStrings(result.records))
		})
	}
}

func TestEvaluationRecordsCacheHitKeepsEveryFact(t *testing.T) {
	t.Parallel()

	document := `{
		"openapi":"3.0.4",
		"components":{"schemas":{"Shared":{"type":"string"}}},
		"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
			"allOf":[
				{"$ref":"#/components/schemas/Shared"},
				{"$ref":"#/components/schemas/Shared"}
			]
		}}}}}}}
	}`
	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	value, err := parseStrictJSON([]byte(`"x"`))
	require.NoError(t, err)

	context := evaluationContext{cache: make(map[evaluationCacheKey]evaluationCacheEntry)}
	firstNode := model.root.allOf[0]
	secondNode := model.root.allOf[1]
	first := context.evaluateNode(firstNode, value, firstNode.occurrence)
	second := context.evaluateNode(secondNode, value, secondNode.occurrence)

	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.Equal(t, []string{"type"}, applicableRules(first.applicableRecords()))
	require.Equal(t, []string{"type"}, applicableRules(second.applicableRecords()))
	require.Equal(t, []string{"string"}, observedLevels(first.observedRecords()))
	require.Equal(t, []string{"string"}, observedLevels(second.observedRecords()))
	require.Equal(t, 2, first.records.count)
	require.Equal(t, first.records.count, second.records.count)
	require.Same(t, first.records, second.records.parts[0].nested)
}

func TestEvaluationRecordsShareAppendAndRebaseWithoutMaterializing(t *testing.T) {
	t.Parallel()

	child := newEvaluationRecords()
	child.append(evaluationRecord{
		kind: evaluationRecordApplicable,
		identity: makeRuleIdentity(schemaOccurrence{
			usePointer:       "#/target",
			targetPointer:    "#/target",
			instanceTemplate: "#",
		}, oracleRuleType),
	})

	parent := newEvaluationRecords()
	parent.appendRecords(child)
	parent.append(evaluationRecord{
		kind: evaluationRecordFailure,
		identity: makeRuleIdentity(schemaOccurrence{
			usePointer:       "#/parent",
			targetPointer:    "#/parent",
			instanceTemplate: "#",
		}, oracleRuleAnyOf),
	})

	rebased := parent.rebased(
		schemaOccurrence{usePointer: "#/target", instanceTemplate: "#"},
		schemaOccurrence{usePointer: "#/use", instanceTemplate: "#/member"},
	)

	require.Equal(t, 2, rebased.count)
	require.Equal(t, []string{
		"applicable|#/use|#/member|type",
		"failure|#/parent|#/member|anyOf",
	}, evaluationRecordStrings(rebased))
	require.Same(t, parent, rebased.parts[0].nested)
}

func evaluationRecordStrings(records *evaluationRecords) []string {
	if records == nil || records.count == 0 {
		return nil
	}

	values := make([]string, 0, records.count)
	records.forEach(func(record evaluationRecord) bool {
		name := "unknown"
		suffix := ""

		switch record.kind {
		case evaluationRecordApplicable:
			name = "applicable"
		case evaluationRecordObserved:
			name = "observed"
			suffix = "|" + record.level
		case evaluationRecordComposition:
			name = record.identity.rule
			suffix = "|" + formatTruthVectorForTest(record.branches)
		case evaluationRecordFailure:
			name = "failure"
		}

		values = append(values, name+"|"+record.identity.String()+suffix)

		return true
	})

	return values
}

func formatTruthVectorForTest(branches []bool) string {
	values := make([]string, len(branches))
	for index, branch := range branches {
		values[index] = fmt.Sprintf("%t", branch)
	}

	return strings.Join(values, ",")
}
