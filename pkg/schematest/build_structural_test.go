package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildStreamsArrayCountAndExistingIndexTargets verifies array count and item rows.
func TestBuildStreamsArrayCountAndExistingIndexTargets(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"array",
		"maxItems":1,
		"items":{"type":"string"}
	}`))

	collect := func() (Report, []Case, error) {
		cases := make([]Case, 0)
		report, err := Build(
			Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100},
			func(testCase Case) error {
				cases = append(cases, testCase)

				return nil
			},
		)

		return report, cases, err
	}

	expectedReport := Report{
		Stop:  SpaceExhausted,
		Steps: 8,
		Covered: []string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:array",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|maxItems|level:valid",
			"#/paths/~1/post/requestBody/content/application~1json/schema/items|#/*|type|level:string",
		},
		Uncovered: []string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|fault:type",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|maxItems|fault:maxItems",
			"#/paths/~1/post/requestBody/content/application~1json/schema/items|#/*|type|fault:type",
		},
	}
	expectedCases := []Case{
		{JSON: []byte(`[]`), Valid: true},
		{JSON: []byte(`[]`), Valid: true},
		{JSON: []byte(`[""]`), Valid: true},
	}

	firstReport, firstCases, err := collect()
	require.NoError(t, err)
	require.Equal(t, expectedReport, firstReport)
	require.Equal(t, expectedCases, firstCases)

	secondReport, secondCases, err := collect()
	require.NoError(t, err)
	require.Equal(t, firstReport, secondReport)
	require.Equal(t, firstCases, secondCases)
	require.Equal(t, firstReport.Stop, secondReport.Stop)
}

// TestBuildMatchesAnyOfTargetsAtExistingArrayIndices verifies wildcard item applicability.
func TestBuildMatchesAnyOfTargetsAtExistingArrayIndices(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"array",
		"minItems":1,
		"items":{
			"anyOf":[{"type":"string"},{"type":"number"}]
		}
	}`))

	collect := func() (Report, []Case, error) {
		cases := make([]Case, 0)
		report, err := Build(
			Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100},
			func(testCase Case) error {
				cases = append(cases, testCase)

				return nil
			},
		)

		return report, cases, err
	}

	expectedReport := Report{
		Stop:  SpaceExhausted,
		Steps: 76,
		Covered: []string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:array",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|minItems|level:valid",
			"#/paths/~1/post/requestBody/content/application~1json/schema/items|#/*|type|level:number",
			"#/paths/~1/post/requestBody/content/application~1json/schema/items|#/*|type|level:string",
			"#/paths/~1/post/requestBody/content/application~1json/schema/items|#/*|anyOf|level:mask:1",
			"#/paths/~1/post/requestBody/content/application~1json/schema/items|#/*|anyOf|level:mask:2",
			"#/paths/~1/post/requestBody/content/application~1json/schema/items/anyOf/0|#/*|type|level:string",
			"#/paths/~1/post/requestBody/content/application~1json/schema/items/anyOf/1|#/*|type|level:number",
		},
		Uncovered: []string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|fault:type",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|minItems|fault:minItems",
			"#/paths/~1/post/requestBody/content/application~1json/schema/items|#/*|anyOf|fault:anyOf",
			"#/paths/~1/post/requestBody/content/application~1json/schema/items/anyOf/0|#/*|type|fault:type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/items/anyOf/1|#/*|type|fault:type",
		},
	}
	expectedCases := []Case{
		{JSON: []byte(`[0]`), Valid: true},
		{JSON: []byte(`[0]`), Valid: true},
		{JSON: []byte(`[0]`), Valid: true},
		{JSON: []byte(`[""]`), Valid: true},
		{JSON: []byte(`[""]`), Valid: true},
		{JSON: []byte(`[0]`), Valid: true},
		{JSON: []byte(`[""]`), Valid: true},
		{JSON: []byte(`[0]`), Valid: true},
	}

	firstReport, firstCases, err := collect()
	require.NoError(t, err)
	require.Equal(t, expectedReport, firstReport)
	require.Equal(t, expectedCases, firstCases)

	secondReport, secondCases, err := collect()
	require.NoError(t, err)
	require.Equal(t, firstReport, secondReport)
	require.Equal(t, firstCases, secondCases)
	require.Equal(t, firstReport.Stop, secondReport.Stop)
}

// TestBuildDoesNotCoverNonexistentArrayIndices verifies empty arrays do not cover items.
func TestBuildDoesNotCoverNonexistentArrayIndices(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"array",
		"maxItems":0,
		"items":{"type":"string"}
	}`))

	collect := func() (Report, []Case, error) {
		cases := make([]Case, 0)
		report, err := Build(
			Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100},
			func(testCase Case) error {
				cases = append(cases, testCase)

				return nil
			},
		)

		return report, cases, err
	}

	expectedReport := Report{
		Stop:  SpaceExhausted,
		Steps: 11,
		Covered: []string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:array",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|maxItems|level:valid",
		},
		Uncovered: []string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|fault:type",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|maxItems|fault:maxItems",
			"#/paths/~1/post/requestBody/content/application~1json/schema/items|#/*|type|level:string",
			"#/paths/~1/post/requestBody/content/application~1json/schema/items|#/*|type|fault:type",
		},
	}
	expectedCases := []Case{
		{JSON: []byte(`[]`), Valid: true},
		{JSON: []byte(`[]`), Valid: true},
	}

	firstReport, firstCases, err := collect()
	require.NoError(t, err)
	require.Equal(t, expectedReport, firstReport)
	require.Equal(t, expectedCases, firstCases)

	secondReport, secondCases, err := collect()
	require.NoError(t, err)
	require.Equal(t, firstReport, secondReport)
	require.Equal(t, firstCases, secondCases)
	require.Equal(t, firstReport.Stop, secondReport.Stop)
}

// TestBuildRepairsCanonicalObjectPresenceForSuppliedProperty verifies presence backtracking.
func TestBuildRepairsCanonicalObjectPresenceForSuppliedProperty(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"minProperties":1,
		"maxProperties":1,
		"properties":{
			"a":{"type":"string"},
			"b":{"type":"number"}
		}
	}`))

	collect := func() (Report, []Case, error) {
		cases := make([]Case, 0)
		report, err := Build(
			Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000},
			func(testCase Case) error {
				cases = append(cases, testCase)

				return nil
			},
		)

		return report, cases, err
	}

	expectedReport := Report{
		Stop:  SpaceExhausted,
		Steps: 43,
		Covered: []string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:object",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|minProperties|level:valid",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|maxProperties|level:valid",
			"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a|#/a|type|level:string",
			"#/paths/~1/post/requestBody/content/application~1json/schema/properties/b|#/b|type|level:number",
		},
		Uncovered: []string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|fault:type",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|minProperties|fault:minProperties",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|maxProperties|fault:maxProperties",
			"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a|#/a|type|fault:type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/properties/b|#/b|type|fault:type",
		},
	}
	expectedCases := []Case{
		{JSON: []byte(`{"a":""}`), Valid: true},
		{JSON: []byte(`{"a":""}`), Valid: true},
		{JSON: []byte(`{"a":""}`), Valid: true},
		{JSON: []byte(`{"a":""}`), Valid: true},
		{JSON: []byte(`{"b":0}`), Valid: true},
	}

	firstReport, firstCases, err := collect()
	require.NoError(t, err)
	require.Equal(t, expectedReport, firstReport)
	require.Equal(t, expectedCases, firstCases)

	secondReport, secondCases, err := collect()
	require.NoError(t, err)
	require.Equal(t, firstReport, secondReport)
	require.Equal(t, firstCases, secondCases)
	require.Equal(t, firstReport.Stop, secondReport.Stop)
}

// TestBuildStreamsObjectPresenceAndPropertyTargets verifies object structural rows.
func TestBuildStreamsObjectPresenceAndPropertyTargets(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"minProperties":1,
		"maxProperties":2,
		"required":["id"],
		"properties":{
			"id":{"type":"string"},
			"optional":{"type":"number"}
		},
		"additionalProperties":{"type":"boolean"}
	}`))

	collect := func() (Report, []Case, error) {
		cases := make([]Case, 0)
		report, err := Build(
			Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100},
			func(testCase Case) error {
				cases = append(cases, testCase)

				return nil
			},
		)

		return report, cases, err
	}

	expectedReport := Report{
		Stop:  SpaceExhausted,
		Steps: 40,
		Covered: []string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:object",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|minProperties|level:valid",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|maxProperties|level:valid",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#/id|required|level:present",
			"#/paths/~1/post/requestBody/content/application~1json/schema/additionalProperties|#/*|type|level:boolean",
			"#/paths/~1/post/requestBody/content/application~1json/schema/properties/id|#/id|type|level:string",
			"#/paths/~1/post/requestBody/content/application~1json/schema/properties/optional|#/optional|type|level:number",
		},
		Uncovered: []string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|fault:type",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|minProperties|fault:minProperties",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|maxProperties|fault:maxProperties",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#/id|required|fault:required",
			"#/paths/~1/post/requestBody/content/application~1json/schema/additionalProperties|#/*|type|fault:type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/properties/id|#/id|type|fault:type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/properties/optional|#/optional|type|fault:type",
		},
	}
	expectedCases := []Case{
		{JSON: []byte(`{"id":""}`), Valid: true},
		{JSON: []byte(`{"id":""}`), Valid: true},
		{JSON: []byte(`{"id":""}`), Valid: true},
		{JSON: []byte(`{"id":""}`), Valid: true},
		{JSON: []byte(`{"__schematest_extra__":false,"id":""}`), Valid: true},
		{JSON: []byte(`{"id":""}`), Valid: true},
		{JSON: []byte(`{"id":"","optional":0}`), Valid: true},
	}

	firstReport, firstCases, err := collect()
	require.NoError(t, err)
	require.Equal(t, expectedReport, firstReport)
	require.Equal(t, expectedCases, firstCases)

	secondReport, secondCases, err := collect()
	require.NoError(t, err)
	require.Equal(t, firstReport, secondReport)
	require.Equal(t, firstCases, secondCases)
	require.Equal(t, firstReport.Stop, secondReport.Stop)
}
