package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNumberBoundaryCandidates pins exact boundary order, quantum, dedupe, and wire bytes.
func TestNumberBoundaryCandidates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		kind     schemaKind
		minimum  string
		maximum  string
		multiple string
		want     []string
	}{
		{
			name:    "number differing scales and exponent",
			kind:    schemaNumber,
			minimum: "-1.20e-2",
			maximum: "0.010",
			want:    []string{"-0.012", "-0.0119", "-0.0121", "0.01", "0.0099", "0.0101", "0"},
		},
		{
			name:    "integer always uses one",
			kind:    schemaInteger,
			minimum: "1.20",
			maximum: "3.456",
			want:    []string{"1.2", "2.2", "0.2", "3.456", "2.456", "4.456", "0"},
		},
		{
			name:     "multiple follows zero in directed order",
			kind:     schemaNumber,
			minimum:  "1",
			multiple: "0.0001",
			want: []string{
				"1", "1.0001", "0.9999", "0",
				"1e-4", "-1e-4", "2e-4",
			},
		},
		{
			name:     "integer multiple uses unit quantum",
			kind:     schemaInteger,
			multiple: "2.00",
			want:     []string{"0", "2", "-2", "3", "1"},
		},
		{
			name:    "single point deduplicates preserving first",
			kind:    schemaNumber,
			minimum: "0.00",
			maximum: "0e-2",
			want:    []string{"0", "0.01", "-0.01"},
		},
		{
			name:    "absent minimum is omitted",
			kind:    schemaNumber,
			maximum: "-2",
			want:    []string{"-2", "-3", "-1", "0"},
		},
		{
			name: "unbounded starts at zero",
			kind: schemaNumber,
			want: []string{"0"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			node := &schemaNode{schemaShape: &schemaShape{kind: test.kind}}
			if test.minimum != "" {
				node.minimum = requireExactNumber(t, test.minimum)
			}

			if test.maximum != "" {
				node.maximum = requireExactNumber(t, test.maximum)
			}

			if test.multiple != "" {
				node.multipleOf = requireExactNumber(t, test.multiple)
			}

			candidates, err := numberDeterministicCandidates(node)
			require.NoError(t, err)
			require.Equal(t, test.want, marshalNumberCandidates(t, candidates))
		})
	}
}

// TestNumberIntegerFormatCandidates pins exact edges and their one-step outside values.
func TestNumberIntegerFormatCandidates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format schemaFormat
		want   []string
	}{
		{
			name:   "int32",
			format: schemaFormatInt32,
			want: []string{
				"0", "-2147483648", "-2147483649", "2147483647", "2147483648",
			},
		},
		{
			name:   "int64",
			format: schemaFormatInt64,
			want: []string{
				"0", "-9223372036854775808", "-9223372036854775809",
				"9223372036854775807", "9223372036854775808",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			node := &schemaNode{schemaShape: &schemaShape{kind: schemaNumber, format: test.format}}
			candidates, err := numberDeterministicCandidates(node)
			require.NoError(t, err)
			require.Equal(t, test.want, marshalNumberCandidates(t, candidates))
		})
	}
}

// TestNumberFloatFormatCandidates pins the exact finite-overflow edges without float conversion.
func TestNumberFloatFormatCandidates(t *testing.T) {
	t.Parallel()

	for _, format := range []schemaFormat{schemaFormatFloat, schemaFormatDouble} {
		t.Run(itoa(int(format)), func(t *testing.T) {
			t.Parallel()

			node := &schemaNode{schemaShape: &schemaShape{kind: schemaNumber, format: format}}
			candidates, err := numberDeterministicCandidates(node)
			require.NoError(t, err)
			require.Len(t, candidates, 5)
			require.Equal(t, "0", marshalNumberCandidates(t, candidates[:1])[0])

			wantMatches := []bool{true, false, true, false}

			for index, candidate := range candidates[1:] {
				matches, matchErr := numericFormatMatches(candidate.number, format)
				require.NoError(t, matchErr)
				require.Equal(t, wantMatches[index], matches)

				encoded, marshalErr := marshalStrict(candidate)
				require.NoError(t, marshalErr)

				parsed, parseErr := parseStrictJSON(encoded)
				require.NoError(t, parseErr)

				comparison, compareErr := parsed.number.compare(candidate.number)
				require.NoError(t, compareErr)
				require.Zero(t, comparison)
			}
		})
	}
}

// TestWalkScalarFindsDirectedExactNonmultiple verifies sibling bounds survive a divisibility objective.
func TestWalkScalarFindsDirectedExactNonmultiple(t *testing.T) {
	t.Parallel()

	node := &schemaNode{schemaShape: &schemaShape{
		kind:       schemaNumber,
		minimum:    requireExactNumber(t, "9007199254740993"),
		maximum:    requireExactNumber(t, "9007199254740993"),
		multipleOf: requireExactNumber(t, "2"),
		format:     schemaFormatInt64,
	}}
	state := &search{maxSteps: 20}

	var found string

	complete, err := state.walkScalar(
		node,
		schemaOccurrence{},
		nil,
		rowSearchContext{},
		jsonNumber,
		func(value *jsonValue) (bool, error) {
			result := evaluateNode(node, value, schemaOccurrence{})
			if result.err != nil {
				return false, result.err
			}

			if len(result.failures) != 1 || result.failures[0].rule != oracleRuleMultipleOf {
				return false, nil
			}

			encoded, marshalErr := marshalStrict(value)
			if marshalErr != nil {
				return false, marshalErr
			}

			found = string(encoded)

			return true, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.Equal(t, "9007199254740993", found)
	require.Equal(t, uint64(1), state.steps)
}

// TestWalkScalarFindsExactBoundIntersections verifies inclusive and exclusive boundary repair.
func TestWalkScalarFindsExactBoundIntersections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		exclusiveMinimum bool
		exclusiveMaximum bool
		want             string
		wantSteps        uint64
	}{
		{name: "inclusive single point", want: "1", wantSteps: 1},
		{
			name:             "exclusive neighbors",
			exclusiveMinimum: true,
			exclusiveMaximum: true,
			want:             "1.01",
			wantSteps:        2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			maximum := "1.00"
			if test.exclusiveMaximum {
				maximum = "1.02"
			}

			node := &schemaNode{schemaShape: &schemaShape{
				kind:             schemaNumber,
				minimum:          requireExactNumber(t, "1.00"),
				maximum:          requireExactNumber(t, maximum),
				exclusiveMinimum: test.exclusiveMinimum,
				exclusiveMaximum: test.exclusiveMaximum,
			}}
			state := &search{maxSteps: 20}

			var found string

			complete, err := state.walkScalar(
				node,
				schemaOccurrence{},
				nil,
				rowSearchContext{},
				jsonNumber,
				func(value *jsonValue) (bool, error) {
					result := evaluateNode(node, value, schemaOccurrence{})
					if result.err != nil || !result.valid {
						return false, result.err
					}

					encoded, marshalErr := marshalStrict(value)
					if marshalErr != nil {
						return false, marshalErr
					}

					found = string(encoded)

					return true, nil
				},
			)
			require.NoError(t, err)
			require.True(t, complete)
			require.Equal(t, test.want, found)
			require.Equal(t, test.wantSteps, state.steps)
		})
	}
}

// TestWalkScalarChargesMultipleCandidates pins one charge for each deduplicated exact candidate.
func TestWalkScalarChargesMultipleCandidates(t *testing.T) {
	t.Parallel()

	node := &schemaNode{schemaShape: &schemaShape{
		kind:       schemaNumber,
		multipleOf: requireExactNumber(t, "0.3"),
	}}
	state := &search{maxSteps: 5}
	seen := make([]string, 0)

	complete, err := state.walkScalar(
		node,
		schemaOccurrence{},
		nil,
		rowSearchContext{},
		jsonNumber,
		func(value *jsonValue) (bool, error) {
			encoded, marshalErr := marshalStrict(value)
			if marshalErr != nil {
				return false, marshalErr
			}

			seen = append(seen, string(encoded))

			return false, nil
		},
	)
	require.ErrorIs(t, err, errMaxSteps)
	require.False(t, complete)
	require.Equal(t, uint64(5), state.steps)
	require.Equal(t, []string{"0", "0.3", "-0.3", "0.4", "0.2"}, seen)
}

// TestWalkScalarChargesBoundaryCandidates pins one charge immediately before each candidate.
func TestWalkScalarChargesBoundaryCandidates(t *testing.T) {
	t.Parallel()

	node := &schemaNode{schemaShape: &schemaShape{
		kind:    schemaNumber,
		minimum: requireExactNumber(t, "1.00"),
		maximum: requireExactNumber(t, "1.00"),
	}}
	state := &search{maxSteps: 2}
	seen := make([]string, 0)

	complete, err := state.walkScalar(
		node,
		schemaOccurrence{},
		nil,
		rowSearchContext{},
		jsonNumber,
		func(value *jsonValue) (bool, error) {
			encoded, marshalErr := marshalStrict(value)
			if marshalErr != nil {
				return false, marshalErr
			}

			seen = append(seen, string(encoded))

			return false, nil
		},
	)
	require.ErrorIs(t, err, errMaxSteps)
	require.False(t, complete)
	require.Equal(t, uint64(2), state.steps)
	require.Equal(t, []string{"1", "1.01"}, seen)
}

// marshalNumberCandidates renders candidate wire bytes for golden assertions.
func marshalNumberCandidates(t *testing.T, candidates []*jsonValue) []string {
	t.Helper()

	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		encoded, err := marshalStrict(candidate)
		require.NoError(t, err)

		result = append(result, string(encoded))
	}

	return result
}
