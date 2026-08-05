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

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Equal(t, uint64(8), report.Steps)
	require.Equal(t, []Case{
		{JSON: []byte(`[]`), Valid: true},
		{JSON: []byte(`[]`), Valid: true},
		{JSON: []byte(`[""]`), Valid: true},
	}, cases)
	require.Contains(
		t,
		report.Covered,
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|maxItems|level:valid",
	)
	require.Contains(
		t,
		report.Covered,
		"#/paths/~1/post/requestBody/content/application~1json/schema/items|#/*|type|level:string",
	)
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

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Equal(t, uint64(82), report.Steps)
	require.Contains(t, cases, Case{JSON: []byte(`[""]`), Valid: true})
	require.Contains(t, cases, Case{JSON: []byte(`[-1]`), Valid: true})

	for _, identity := range []string{
		"#/paths/~1/post/requestBody/content/application~1json/schema/items/anyOf/0|#/*|type|level:string",
		"#/paths/~1/post/requestBody/content/application~1json/schema/items/anyOf/1|#/*|type|level:number",
	} {
		require.Contains(t, report.Covered, identity)
	}
}

// TestBuildDoesNotCoverNonexistentArrayIndices verifies empty arrays do not cover items.
func TestBuildDoesNotCoverNonexistentArrayIndices(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"array",
		"maxItems":0,
		"items":{"type":"string"}
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Equal(t, []Case{
		{JSON: []byte(`[]`), Valid: true},
		{JSON: []byte(`[]`), Valid: true},
	}, cases)
	require.NotContains(
		t,
		report.Covered,
		"#/paths/~1/post/requestBody/content/application~1json/schema/items|#/*|type|level:string",
	)
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

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Contains(
		t,
		report.Covered,
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/b|#/b|type|level:number",
	)
	require.Contains(t, cases, Case{JSON: []byte(`{"b":-1}`), Valid: true})
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

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Equal(t, uint64(40), report.Steps)
	require.Equal(t, []Case{
		{JSON: []byte(`{"id":""}`), Valid: true},
		{JSON: []byte(`{"id":""}`), Valid: true},
		{JSON: []byte(`{"id":""}`), Valid: true},
		{JSON: []byte(`{"id":""}`), Valid: true},
		{JSON: []byte(`{"__schematest_extra__":false,"id":""}`), Valid: true},
		{JSON: []byte(`{"id":""}`), Valid: true},
		{JSON: []byte(`{"id":"","optional":-1}`), Valid: true},
	}, cases)

	for _, identity := range []string{
		"#/paths/~1/post/requestBody/content/application~1json/schema|#/id|required|level:present",
		"#/paths/~1/post/requestBody/content/application~1json/schema/additionalProperties|#/*|type|level:boolean",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/optional|#/optional|type|level:number",
	} {
		require.Contains(t, report.Covered, identity)
	}
}
