package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSeededNumberFrontierPinsChoicesAndCharges pins length, exponent, sign, and digit assignments.
func TestSeededNumberFrontierPinsChoicesAndCharges(t *testing.T) {
	t.Parallel()

	seed := searchSeed(
		"#/schema",
		[]byte(`{"format":"double","type":"number"}`),
		oracleRuleFormat,
		oracleNumericValidLevel,
	)
	require.Equal(t, uint64(0xdf93600b4a4e0f49), seed)

	state := &search{maxSteps: 10}
	seen := make([]string, 0, 2)
	complete, err := state.walkSeededNumberFrontier(seed, func(value *jsonValue) (bool, error) {
		encoded, marshalErr := marshalStrict(value)
		if marshalErr != nil {
			return false, marshalErr
		}

		seen = append(seen, string(encoded))

		return len(seen) == 2, nil
	})
	require.NoError(t, err)
	require.True(t, complete)
	require.Equal(t, []string{"7", "8"}, seen)
	require.Equal(t, uint64(5), state.steps)
}

// TestSeededNumberFrontierStopsOnlyAtGlobalBudget verifies its unbounded normal stop.
func TestSeededNumberFrontierStopsOnlyAtGlobalBudget(t *testing.T) {
	t.Parallel()

	state := &search{maxSteps: 30}
	seen := 0
	complete, err := state.walkSeededNumberFrontier(0, func(value *jsonValue) (bool, error) {
		encoded, marshalErr := marshalStrict(value)
		if marshalErr != nil {
			return false, marshalErr
		}

		parsed, parseErr := parseStrictJSON(encoded)
		if parseErr != nil {
			return false, parseErr
		}

		require.Equal(t, jsonNumber, parsed.kind)

		seen++

		return false, nil
	})
	require.ErrorIs(t, err, errMaxSteps)
	require.False(t, complete)
	require.Equal(t, uint64(30), state.steps)
	require.Positive(t, seen)
}

// TestBuildFindsSeededExactNumberAcrossAllOf verifies complete-oracle Build integration.
func TestBuildFindsSeededExactNumberAcrossAllOf(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"number",
		"allOf":[
			{"minimum":10},
			{"maximum":20},
			{"multipleOf":7},
			{"format":"double"}
		]
	}`))
	cases := make([]Case, 0)

	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 20_000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)
	require.NoError(t, err)
	require.Contains(t, cases, Case{JSON: []byte("14"), Valid: true})
	require.Equal(t, MaxStepsReached, report.Stop)
}
