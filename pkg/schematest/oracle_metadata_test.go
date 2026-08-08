//nolint:godoclint,lll // Ticket-focused private oracle evidence uses behavior names and inline schemas.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdmissionCanonicalizesEnumMembersAndRequiredNamesOnce(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"enum":[1,1.0,{"a":1,"b":2},{"b":2.0,"a":1}],
			"required":["é","z","a"]
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)
	require.Equal(t, []int{0, 2}, []int{
		model.root.enum[0].authoredIndex,
		model.root.enum[1].authoredIndex,
	})
	require.Equal(t, []string{"a", "z", "é"}, model.root.required)

	result := evaluateSchemaValue(t, `{"enum":[1,1.0,{"a":1,"b":2},{"b":2.0,"a":1}]}`, `{"b":2,"a":1}`)
	require.NoError(t, result.err)
	records := evaluationRecordSlice(result.records)
	require.Equal(t, evaluationRecordObserved, records[3].kind)
	require.Equal(t, oracleRuleEnum, records[3].identity.rule)
	require.Equal(t, "member:2", records[3].level)
}

func TestValidatedEnumObjectEqualityIgnoresOrderAndRejectsMiss(t *testing.T) {
	t.Parallel()

	equal := evaluateSchemaValue(t, `{"enum":[{"a":1,"b":2}]}`, `{"b":2.0,"a":1}`)
	require.NoError(t, equal.err)
	require.True(t, equal.valid)

	miss := evaluateSchemaValue(t, `{"enum":[{"a":1},{"a":2}]}`, `{"a":3}`)
	require.NoError(t, miss.err)
	require.False(t, miss.valid)
	require.Equal(t, []string{oracleRuleEnum}, applicableRules(miss.failureRecords()))
}

func TestOracleRecordsCoverExactNumericEdges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		value  string
		rule   string
	}{
		{name: "above exclusive minimum", schema: `{"type":"number","minimum":1,"exclusiveMinimum":true}`, value: `1.0000000000000000001`, rule: oracleRuleExclusiveMinimum},
		{name: "below exclusive maximum", schema: `{"type":"number","maximum":1,"exclusiveMaximum":true}`, value: `0.9999999999999999999`, rule: oracleRuleExclusiveMaximum},
		{name: "int32 maximum", schema: `{"type":"integer","format":"int32"}`, value: `2147483647`, rule: oracleRuleFormat},
		{name: "int64 maximum", schema: `{"type":"integer","format":"int64"}`, value: `9223372036854775807`, rule: oracleRuleFormat},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, test.schema, test.value)
			require.NoError(t, result.err)
			require.True(t, result.valid)
			records := evaluationRecordSlice(result.records)
			require.Equal(t, evaluationRecordApplicable, records[2].kind)
			require.Equal(t, test.rule, records[2].identity.rule)
			require.Equal(t, evaluationRecordObserved, records[3].kind)
			require.Equal(t, test.rule, records[3].identity.rule)
			require.Equal(t, "valid", records[3].level)
		})
	}
}

func TestOracleRecordsCoverPropertyCountsAndRequestDirection(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
		value  string
		rule   string
	}{
		{name: "under property count", schema: `{"type":"object","minProperties":1}`, value: `{}`, rule: oracleRuleMinProperties},
		{name: "over property count", schema: `{"type":"object","maxProperties":0}`, value: `{"x":1}`, rule: oracleRuleMaxProperties},
		{name: "absent required writeOnly", schema: `{"type":"object","required":["x"],"properties":{"x":{"type":"string","writeOnly":true}}}`, value: `{}`, rule: oracleRuleRequired},
		{name: "allOf local readOnly does not remove sibling requiredness", schema: `{"type":"object","allOf":[{"required":["x"]},{"properties":{"x":{"type":"string","readOnly":true}}}]}`, value: `{}`, rule: oracleRuleRequired},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, test.schema, test.value)
			require.NoError(t, result.err)
			require.False(t, result.valid)
			failures := evaluationRecordsOfKind(result.records, evaluationRecordFailure)
			require.Len(t, failures, 1)
			require.Equal(t, test.rule, failures[0].identity.rule)
		})
	}

	absentReadOnly := evaluateSchemaValue(t, `{"type":"object","required":["x"],"properties":{"x":{"type":"string","readOnly":true}}}`, `{}`)
	require.NoError(t, absentReadOnly.err)
	require.True(t, absentReadOnly.valid)
	require.Empty(t, evaluationRecordsOfKind(absentReadOnly.records, evaluationRecordFailure))

	suppliedReadOnly := evaluateSchemaValue(t, `{"type":"object","required":["x"],"properties":{"x":{"type":"string","readOnly":true}}}`, `{"x":1}`)
	require.NoError(t, suppliedReadOnly.err)
	require.False(t, suppliedReadOnly.valid)
	readOnlyFailures := evaluationRecordsOfKind(suppliedReadOnly.records, evaluationRecordFailure)
	require.Equal(t, oracleRuleType, readOnlyFailures[0].identity.rule)

	suppliedWriteOnly := evaluateSchemaValue(t, `{"type":"object","required":["x"],"properties":{"x":{"type":"string","writeOnly":true}}}`, `{"x":"ok"}`)
	require.NoError(t, suppliedWriteOnly.err)
	require.True(t, suppliedWriteOnly.valid)
	writeOnlyRecords := evaluationRecordSlice(suppliedWriteOnly.records)
	require.Equal(t, evaluationRecordObserved, writeOnlyRecords[3].kind)
	require.Equal(t, oracleRuleRequired, writeOnlyRecords[3].identity.rule)
	require.Equal(t, oracleRequiredPresentLevel, writeOnlyRecords[3].level)
}

func TestReferencedArrayItemFailureKeepsCompleteIdentity(t *testing.T) {
	t.Parallel()

	document := `{
		"openapi":"3.0.4",
		"components":{"schemas":{"Item":{"type":"string"}}},
		"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
			"type":"array","items":{"$ref":"#/components/schemas/Item"}
		}}}}}}}
	}`
	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	value, err := parseStrictJSON([]byte(`[1]`))
	require.NoError(t, err)

	result := evaluate(model, value)
	require.NoError(t, result.err)
	failures := evaluationRecordsOfKind(result.records, evaluationRecordFailure)
	require.Len(t, failures, 1)
	require.Equal(t, ruleIdentity{
		occurrence: schemaOccurrence{
			usePointer:       model.root.occurrence.usePointer + "/items",
			targetPointer:    "#/components/schemas/Item",
			instanceTemplate: "#/0",
			reference:        true,
		},
		rule: oracleRuleType,
	}, failures[0].identity.project())
}

func TestNestedReferencedArrayItemFailureKeepsOrderedDescendantIdentity(t *testing.T) {
	t.Parallel()

	document := `{
		"openapi":"3.0.4",
		"components":{"schemas":{
			"Leaf":{"type":"string"},
			"Item":{"type":"object","properties":{"field":{"$ref":"#/components/schemas/Leaf"}}}
		}},
		"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
			"type":"array","items":{"$ref":"#/components/schemas/Item"}
		}}}}}}}
	}`
	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	value, err := parseStrictJSON([]byte(`[{"field":1}]`))
	require.NoError(t, err)

	result := evaluate(model, value)
	require.NoError(t, result.err)
	records := evaluationRecordSlice(result.records)
	rootUse := model.root.occurrence.usePointer
	require.Equal(t, []string{
		"applicable|" + rootUse + "|#|type",
		"observed|" + rootUse + "|#|type|array",
		"applicable|" + rootUse + "/items|#/0|type",
		"observed|" + rootUse + "/items|#/0|type|object",
		"applicable|" + rootUse + "/items/properties/field|#/0/field|type",
		"failure|" + rootUse + "/items/properties/field|#/0/field|type",
	}, evaluationRecordStrings(result.records))
	failure := records[len(records)-1].identity.project()
	require.Equal(t, schemaOccurrence{
		usePointer:       rootUse + "/items/properties/field",
		targetPointer:    "#/components/schemas/Leaf",
		instanceTemplate: "#/0/field",
		reference:        true,
	}, failure.occurrence)
	require.Equal(t, oracleRuleType, failure.rule)
}

func evaluationRecordsOfKind(records *evaluationRecords, kind evaluationRecordKind) []evaluationRecord {
	result := make([]evaluationRecord, 0)

	records.forEach(func(record evaluationRecord) bool {
		if record.kind == kind {
			result = append(result, record)
		}

		return true
	})

	return result
}

func evaluationRecordSlice(records *evaluationRecords) []evaluationRecord {
	result := make([]evaluationRecord, 0)

	records.forEach(func(record evaluationRecord) bool {
		result = append(result, record)

		return true
	})

	return result
}

func TestInertSchemaMetadataDoesNotChangeAuthoritativeRecords(t *testing.T) {
	t.Parallel()

	plain := evaluateSchemaValue(t, `{"type":"string"}`, `"x"`)
	annotated := evaluateSchemaValue(t, `{
		"type":"string",
		"default":"default",
		"title":"Title",
		"description":"Documentation",
		"xml":{"name":"item","namespace":"https://example.test/xml","prefix":"p","attribute":false,"wrapped":false}
	}`, `"x"`)
	require.NoError(t, plain.err)
	require.NoError(t, annotated.err)
	require.Equal(t, evaluationRecordSlice(plain.records), evaluationRecordSlice(annotated.records))
}
