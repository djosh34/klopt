package schematest

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildFullStreamDeterminism pins cases, reports, and normal stops across representative schemas.
func TestBuildFullStreamDeterminism(t *testing.T) {
	t.Parallel()

	corpus, err := os.ReadFile("testdata/alpha_zeta.yaml")
	require.NoError(t, err)

	tests := []struct {
		name        string
		document    []byte
		operationID string
		maxSteps    uint64
	}{
		{
			name:        "primitive",
			document:    []byte(documentWithJSONSchema(`{"type":"boolean"}`)),
			operationID: "selected",
			maxSteps:    100,
		},
		{
			name: "structural",
			document: []byte(documentWithJSONSchema(`{
				"type":"object",
				"required":["items"],
				"properties":{"items":{"type":"array","minItems":1,"items":{"type":"boolean"}}},
				"additionalProperties":false
			}`)),
			operationID: "selected",
			maxSteps:    1_000,
		},
		{
			name: "composition",
			document: []byte(documentWithJSONSchema(`{
				"anyOf":[
					{"type":"string","enum":["a"]},
					{"type":"number","minimum":1,"maximum":2}
				]
			}`)),
			operationID: "selected",
			maxSteps:    1_000,
		},
		{
			name:        "string scalar",
			document:    []byte(documentWithJSONSchema(`{"type":"string","minLength":1,"pattern":"^a$"}`)),
			operationID: "selected",
			maxSteps:    100,
		},
		{
			name:        "number scalar",
			document:    []byte(documentWithJSONSchema(`{"type":"number","minimum":1,"maximum":2}`)),
			operationID: "selected",
			maxSteps:    100,
		},
		{
			name:        "corpus",
			document:    corpus,
			operationID: "alphaRequest",
			maxSteps:    10_000,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			input := Input{OpenAPI: test.document, OperationID: test.operationID, MaxSteps: test.maxSteps}
			firstCases, firstReport, firstErr := collectDeterministicRun(input, nil)
			require.NoError(t, firstErr)
			require.NotEmpty(t, firstCases)
			require.Contains(t, []StopReason{SpaceExhausted, MaxStepsReached}, firstReport.Stop)

			for range 2 {
				cases, report, buildErr := collectDeterministicRun(input, nil)
				require.NoError(t, buildErr)
				require.Equal(t, firstCases, cases)
				require.Equal(t, firstReport, report)
			}
		})
	}
}

// TestBuildCutoffsChargeBeforeEveryAssignmentPhase pins public cutoff boundaries.
func TestBuildCutoffsChargeBeforeEveryAssignmentPhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		schema    string
		maxSteps  uint64
		wantCases []Case
	}{
		{
			name: "structural choice",
			schema: `{
				"type":"object",
				"required":["name"],
				"properties":{"name":{"type":"string"}}
			}`,
			maxSteps: 3,
		},
		{
			name:     "number edge",
			schema:   `{"type":"number","minimum":1,"maximum":2}`,
			maxSteps: 1,
		},
		{
			name:      "parent replay",
			schema:    `{"type":"boolean"}`,
			maxSteps:  3,
			wantCases: []Case{{JSON: []byte("false"), Valid: true}},
		},
		{
			name:      "fault application",
			schema:    `{"type":"boolean"}`,
			maxSteps:  4,
			wantCases: []Case{{JSON: []byte("false"), Valid: true}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			input := Input{
				OpenAPI:     []byte(documentWithJSONSchema(test.schema)),
				OperationID: "selected",
				MaxSteps:    test.maxSteps,
			}
			cases, report, err := collectDeterministicRun(input, nil)
			require.NoError(t, err)
			require.Equal(t, MaxStepsReached, report.Stop)
			require.Equal(t, test.maxSteps, report.Steps)
			require.Equal(t, test.wantCases, cases)
		})
	}

	boolean := Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"type":"boolean"}`)),
		OperationID: "selected",
		MaxSteps:    5,
	}
	cases, report, err := collectDeterministicRun(boolean, nil)
	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Equal(t, uint64(5), report.Steps)
	require.Equal(t, []Case{
		{JSON: []byte("false"), Valid: true},
		{JSON: []byte("null"), Valid: false},
	}, cases)
}

// TestBuildStringProductEdgeCutoff pins the assignment immediately before the
// sole product transition. A character class prevents the finite scalar
// frontier from supplying the satisfying witness.
func TestBuildStringProductEdgeCutoff(t *testing.T) {
	t.Parallel()

	input := Input{
		OpenAPI: []byte(documentWithJSONSchema(
			`{"type":"string","minLength":1,"maxLength":1,"pattern":"^[q]$"}`,
		)),
		OperationID: "selected",
		MaxSteps:    7,
	}

	cutoffCases, cutoffReport, err := collectDeterministicRun(input, nil)
	require.NoError(t, err)
	require.Empty(t, cutoffCases, "a cutoff before the product edge emits no partial case")
	require.Equal(t, Report{Stop: MaxStepsReached, Steps: 7, Uncovered: cutoffReport.Uncovered}, cutoffReport)

	input.MaxSteps++
	adjacentCases, adjacentReport, err := collectDeterministicRun(input, nil)
	require.NoError(t, err)
	require.Equal(t, MaxStepsReached, adjacentReport.Stop)
	require.Equal(t, uint64(8), adjacentReport.Steps)
	require.Equal(t, []Case{{JSON: []byte(`"q"`), Valid: true}}, adjacentCases)
}

// TestBuildAdmissionErrorsAreDeterministic pins non-authoritative malformed and selection failures.
func TestBuildAdmissionErrorsAreDeterministic(t *testing.T) {
	t.Parallel()

	const (
		operationWithoutBody = `{
		"openapi":"3.0.3",
		"info":{"title":"test","version":"1"},
		"paths":{"/":{"post":{"operationId":"selected","responses":{"204":{"description":"ok"}}}}}
	}`
		operationWithoutJSONMedia = `{
		"openapi":"3.0.3",
		"info":{"title":"test","version":"1"},
		"paths":{"/":{"post":{
			"operationId":"selected",
			"requestBody":{"content":{"text/plain":{"schema":{"type":"string"}}}},
			"responses":{"204":{"description":"ok"}}
		}}}
	}`
		operationWithoutSchema = `{
		"openapi":"3.0.3",
		"info":{"title":"test","version":"1"},
		"paths":{"/":{"post":{
			"operationId":"selected",
			"requestBody":{"content":{"application/json":{}}},
			"responses":{"204":{"description":"ok"}}
		}}}
	}`
	)

	tests := []struct {
		name        string
		document    string
		operationID string
	}{
		{name: "malformed", document: `{`, operationID: "selected"},
		{
			name:        "missing operation",
			document:    documentWithJSONSchema(`{"type":"boolean"}`),
			operationID: "absent",
		},
		{name: "missing body", document: operationWithoutBody, operationID: "selected"},
		{name: "missing media", document: operationWithoutJSONMedia, operationID: "selected"},
		{name: "missing schema", document: operationWithoutSchema, operationID: "selected"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			input := Input{OpenAPI: []byte(test.document), OperationID: test.operationID, MaxSteps: 0}
			firstCases, firstReport, firstErr := collectDeterministicRun(input, nil)
			require.Error(t, firstErr)
			require.Empty(t, firstCases)
			require.Zero(t, firstReport)

			secondCases, secondReport, secondErr := collectDeterministicRun(input, nil)
			require.EqualError(t, secondErr, firstErr.Error())
			require.Empty(t, secondCases)
			require.Zero(t, secondReport)
		})
	}
}

// TestBuildZeroBudgetAndCallbackFailureAreDeterministic pins execution-boundary behavior.
func TestBuildZeroBudgetAndCallbackFailureAreDeterministic(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{"type":"boolean"}`))
	zeroInput := Input{OpenAPI: document, OperationID: "selected", MaxSteps: 0}
	firstCases, firstReport, err := collectDeterministicRun(zeroInput, nil)
	require.NoError(t, err)
	require.Empty(t, firstCases)
	require.Equal(t, MaxStepsReached, firstReport.Stop)
	require.Zero(t, firstReport.Steps)

	secondCases, secondReport, err := collectDeterministicRun(zeroInput, nil)
	require.NoError(t, err)
	require.Equal(t, firstCases, secondCases)
	require.Equal(t, firstReport, secondReport)

	callbackErr := errors.New("deterministic callback failure")

	callbackInput := Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100}
	for range 2 {
		cases, report, buildErr := collectDeterministicRun(callbackInput, callbackErr)
		require.ErrorIs(t, buildErr, callbackErr)
		require.Equal(t, []Case{{JSON: []byte("false"), Valid: true}}, cases)
		require.Zero(t, report)
	}
}

// collectDeterministicRun copies callback-lifetime bytes into one comparable test run.
func collectDeterministicRun(input Input, callbackErr error) ([]Case, Report, error) {
	var cases []Case

	report, err := Build(input, func(testCase Case) error {
		cases = append(cases, Case{JSON: append([]byte(nil), testCase.JSON...), Valid: testCase.Valid})

		return callbackErr
	})

	return cases, report, err
}
