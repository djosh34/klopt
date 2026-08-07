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

// TestBuildPreservesComposedNumericEnums verifies the active finite enum path.
func TestBuildPreservesComposedNumericEnums(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		schema    string
		wantCases []Case
		wantSteps uint64
		covered   []string
	}{
		{
			name: "allOf enum",
			schema: `{
				"type":"number",
				"allOf":[{"enum":[5]}]
			}`,
			wantCases: []Case{
				{JSON: []byte("5"), Valid: true},
				{JSON: []byte("5"), Valid: true},
				{JSON: []byte("5"), Valid: true},
				{JSON: []byte("5"), Valid: true},
				{JSON: []byte("-1"), Valid: false},
			},
			wantSteps: 31,
			covered: []string{
				"/allOf/0|#|enum|level:member:0",
			},
		},
		{
			name: "intersecting enum and minimum",
			schema: `{
				"type":"number",
				"allOf":[{"enum":[4,5]},{"minimum":5}]
			}`,
			wantCases: []Case{
				{JSON: []byte("5"), Valid: true},
				{JSON: []byte("5"), Valid: true},
				{JSON: []byte("5"), Valid: true},
				{JSON: []byte("5"), Valid: true},
				{JSON: []byte("5"), Valid: true},
				{JSON: []byte("5"), Valid: true},
				{JSON: []byte("4"), Valid: false},
			},
			wantSteps: 78,
			covered: []string{
				"/allOf/0|#|enum|level:member:1",
				"/allOf/1|#|minimum|level:valid",
			},
		},
		{
			name: "selected anyOf enums",
			schema: `{
				"type":"number",
				"anyOf":[{"enum":[5]},{"enum":[7]}]
			}`,
			wantCases: []Case{
				{JSON: []byte("5"), Valid: true},
				{JSON: []byte("5"), Valid: true},
				{JSON: []byte("7"), Valid: true},
				{JSON: []byte("5"), Valid: true},
				{JSON: []byte("5"), Valid: true},
				{JSON: []byte("7"), Valid: true},
				{JSON: []byte("7"), Valid: true},
			},
			wantSteps: 51,
			covered: []string{
				"|#|anyOf|level:mask:1",
				"|#|anyOf|level:mask:2",
				"/anyOf/0|#|enum|level:member:0",
				"/anyOf/1|#|enum|level:member:0",
			},
		},
	}

	const schemaPointer = "#/paths/~1/post/requestBody/content/application~1json/schema"

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cases := make([]Case, 0, len(test.wantCases))
			report, err := Build(
				Input{
					OpenAPI:     []byte(documentWithJSONSchema(test.schema)),
					OperationID: "selected",
					MaxSteps:    10000,
				},
				func(testCase Case) error {
					cases = append(cases, testCase)

					return nil
				},
			)
			require.NoError(t, err)
			require.Equal(t, SpaceExhausted, report.Stop)
			require.Equal(t, test.wantSteps, report.Steps)
			require.Equal(t, test.wantCases, cases)

			for _, suffix := range test.covered {
				require.Contains(t, report.Covered, schemaPointer+suffix)
			}
		})
	}
}

// TestBuildSearchesNumericFalseBranchObjectives verifies realizable exact anyOf masks.
func TestBuildSearchesNumericFalseBranchObjectives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		schema    string
		wantCases []Case
		masks     []string
	}{
		{
			name:   "unconstrained true branch",
			schema: `{"type":"number","anyOf":[{}, {"maximum":0}]}`,
			wantCases: []Case{
				{JSON: []byte("9"), Valid: true},
				{JSON: []byte("7"), Valid: true},
				{JSON: []byte("0"), Valid: true},
			},
			masks: []string{"1", "3"},
		},
		{
			name:   "true and false numeric rules",
			schema: `{"type":"number","anyOf":[{"minimum":1},{"maximum":0}]}`,
			wantCases: []Case{
				{JSON: []byte("1"), Valid: true},
				{JSON: []byte("1"), Valid: true},
				{JSON: []byte("0"), Valid: true},
				{JSON: []byte("1"), Valid: true},
			},
			masks: []string{"1", "2"},
		},
	}

	const schemaPointer = "#/paths/~1/post/requestBody/content/application~1json/schema"

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			build := func() ([]Case, Report) {
				cases := make([]Case, 0, len(test.wantCases))
				report, err := Build(
					Input{
						OpenAPI:     []byte(documentWithJSONSchema(test.schema)),
						OperationID: "selected",
						MaxSteps:    10000,
					},
					func(testCase Case) error {
						cases = append(cases, testCase)

						return nil
					},
				)
				require.NoError(t, err)

				return cases, report
			}

			firstCases, firstReport := build()
			secondCases, secondReport := build()

			require.Equal(t, test.wantCases, firstCases)
			require.Equal(t, firstCases, secondCases)
			require.Equal(t, firstReport, secondReport)
			require.Equal(t, MaxStepsReached, firstReport.Stop)
			require.Equal(t, uint64(10000), firstReport.Steps)

			for _, mask := range test.masks {
				require.Contains(
					t, firstReport.Covered, schemaPointer+"|#|anyOf|level:mask:"+mask,
				)
			}
		})
	}
}

// TestBuildSearchesIntegerFalseBranchObjectives verifies integer-only skipped branches.
func TestBuildSearchesIntegerFalseBranchObjectives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		schema    string
		wantCases []Case
		wantStop  StopReason
		wantSteps uint64
		masks     []string
	}{
		{
			name:   "explicit number with integer branch",
			schema: `{"type":"number","anyOf":[{}, {"type":"integer"}]}`,
			wantCases: []Case{
				{JSON: []byte("-0.7"), Valid: true},
				{JSON: []byte("-0.7"), Valid: true},
				{JSON: []byte("0"), Valid: true},
			},
			wantStop:  MaxStepsReached,
			wantSteps: 10000,
			masks:     []string{"1", "3"},
		},
		{
			name:   "planner integer and number branches",
			schema: `{"anyOf":[{"type":"integer"},{"type":"number"}]}`,
			wantCases: []Case{
				{JSON: []byte("-0.8"), Valid: true},
				{JSON: []byte("0.2"), Valid: true},
				{JSON: []byte("0"), Valid: true},
				{JSON: []byte("0"), Valid: true},
				{JSON: []byte("0"), Valid: true},
				{JSON: []byte("null"), Valid: false},
				{JSON: []byte("null"), Valid: false},
				{JSON: []byte("null"), Valid: false},
			},
			wantStop:  SpaceExhausted,
			wantSteps: 147,
			masks:     []string{"2", "3"},
		},
		{
			name:   "nested integer branch",
			schema: `{"type":"number","anyOf":[{}, {"allOf":[{"type":"integer"}]}]}`,
			wantCases: []Case{
				{JSON: []byte("-0.2"), Valid: true},
				{JSON: []byte("0.9"), Valid: true},
				{JSON: []byte("0"), Valid: true},
			},
			wantStop:  MaxStepsReached,
			wantSteps: 10000,
			masks:     []string{"1", "3"},
		},
	}

	const schemaPointer = "#/paths/~1/post/requestBody/content/application~1json/schema"

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			build := func() ([]Case, Report) {
				cases := make([]Case, 0, len(test.wantCases))
				report, err := Build(
					Input{
						OpenAPI:     []byte(documentWithJSONSchema(test.schema)),
						OperationID: "selected",
						MaxSteps:    10000,
					},
					func(testCase Case) error {
						cases = append(cases, testCase)

						return nil
					},
				)
				require.NoError(t, err)

				return cases, report
			}

			firstCases, firstReport := build()
			secondCases, secondReport := build()

			require.Equal(t, test.wantCases, firstCases)
			require.Equal(t, firstCases, secondCases)
			require.Equal(t, firstReport, secondReport)
			require.Equal(t, test.wantStop, firstReport.Stop)
			require.Equal(t, test.wantSteps, firstReport.Steps)

			for _, mask := range test.masks {
				require.Contains(
					t, firstReport.Covered, schemaPointer+"|#|anyOf|level:mask:"+mask,
				)
			}
		})
	}
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
