package schematest

import (
	"math/big"
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

// TestSeededNumberFrontierSeedsLengthAndExponentShells pins all private choice dimensions.
func TestSeededNumberFrontierSeedsLengthAndExponentShells(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		seed      uint64
		wantEdges []string
		wantSteps uint64
	}{
		{
			name:      "format seed",
			seed:      0xdf93600b4a4e0f49,
			wantEdges: []string{"7", "0.7", "70", "7.7"},
			wantSteps: 70,
		},
		{
			name:      "schema seed",
			seed:      0xe35d5f9a218f8214,
			wantEdges: []string{"5", "50", "0.5", "51"},
			wantSteps: 71,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			state := &search{maxSteps: 1000}
			seen := make([]string, 0, 55)
			complete, err := state.walkSeededNumberFrontier(
				test.seed,
				func(value *jsonValue) (bool, error) {
					encoded, marshalErr := marshalStrict(value)
					if marshalErr != nil {
						return false, marshalErr
					}

					seen = append(seen, string(encoded))

					return len(seen) == 55, nil
				},
			)
			require.NoError(t, err)
			require.True(t, complete)
			require.Equal(t, test.wantEdges, []string{seen[0], seen[18], seen[36], seen[54]})
			require.Equal(t, test.wantSteps, state.steps)
			require.GreaterOrEqual(t, new(big.Int).Abs(requireExactNumber(t, seen[54]).numerator).Int64(), int64(10))
		})
	}
}

// TestSeededNumberShellSeedsCandidateLengthOrder pins schema-dependent length selection.
func TestSeededNumberShellSeedsCandidateLengthOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		seed      uint64
		want      string
		wantSteps uint64
	}{
		{seed: 0, want: "1100", wantSteps: 6},
		{seed: 256, want: "500", wantSteps: 4},
	}

	for _, test := range tests {
		state := &search{maxSteps: 100}

		var seen string

		complete, err := state.walkSeededNumberShell(
			3,
			test.seed,
			func(value *jsonValue) (bool, error) {
				encoded, marshalErr := marshalStrict(value)
				if marshalErr != nil {
					return false, marshalErr
				}

				seen = string(encoded)

				return true, nil
			},
		)
		require.NoError(t, err)
		require.True(t, complete)
		require.Equal(t, test.want, seen)
		require.Equal(t, test.wantSteps, state.steps)
	}
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
