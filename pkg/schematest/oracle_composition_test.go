//nolint:godoclint // Composition tables keep truth-vector and exact-failure behavior together.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluateAllOfKeepsLocalSiblingsConjunctiveAndRecordsEveryBranch(t *testing.T) {
	t.Parallel()

	result := evaluateSchemaValue(
		t,
		`{"type":"string","minLength":3,"allOf":[{"pattern":"^a"},{"pattern":"z$"}]}`,
		`"bz"`,
	)

	require.NoError(t, result.err)
	require.False(t, result.valid)
	require.Equal(t, [][]bool{{false, true}}, compositionTruthVectorsForTest(result.allOf))
	require.True(t, evaluationQueryEmpty(result.anyOf))
	require.Equal(
		t,
		[]string{"type", "minLength", "type", "pattern", "type", "pattern"},
		applicableRules(result.applicable),
	)
	require.Equal(t, []string{"string", "string", "string", "valid"}, observedLevels(result.observed))
	require.Equal(
		t,
		[]string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|minLength",
			"#/paths/~1/post/requestBody/content/application~1json/schema/allOf/0|#|pattern",
		},
		identityStrings(result.failures),
	)
}

func TestEvaluateAnyOfRecordsCompleteTruthMaskAndOnlyFailsWhenAllBranchesFail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		valid    bool
		truth    [][]bool
		observed []string
		failures []string
	}{
		{
			name:     "both branches",
			value:    `"az"`,
			valid:    true,
			truth:    [][]bool{{true, true}},
			observed: []string{"string", "string", "valid", "string", "valid"},
		},
		{
			name:     "first branch only",
			value:    `"a"`,
			valid:    true,
			truth:    [][]bool{{true, false}},
			observed: []string{"string", "string", "valid", "string"},
		},
		{
			name:     "all branches fail",
			value:    `"x"`,
			valid:    false,
			truth:    [][]bool{{false, false}},
			observed: []string{"string", "string", "string"},
			failures: []string{"pattern", "pattern", "anyOf"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(
				t,
				`{"type":"string","anyOf":[{"pattern":"^a"},{"pattern":"z$"}]}`,
				test.value,
			)

			require.NoError(t, result.err)
			require.Equal(t, test.valid, result.valid)
			require.Equal(t, test.truth, compositionTruthVectorsForTest(result.anyOf))
			require.True(t, evaluationQueryEmpty(result.allOf))
			require.Equal(
				t,
				[]string{"type", "type", "pattern", "type", "pattern"},
				applicableRules(result.applicable),
			)
			require.Equal(t, test.observed, observedLevels(result.observed))
			require.Equal(t, test.failures, failureRules(result.failures))
		})
	}
}

func TestEvaluatePatternOnlyAnyOfBranchIsTrueWhenPatternIsInapplicable(t *testing.T) {
	t.Parallel()

	result := evaluateSchemaValue(
		t,
		`{"anyOf":[{"pattern":"^a"},{"type":"string"}]}`,
		`1`,
	)

	require.NoError(t, result.err)
	require.True(t, result.valid)
	require.Equal(t, [][]bool{{true, false}}, compositionTruthVectorsForTest(result.anyOf))
	require.Equal(t, []string{"type", "type", "type"}, applicableRules(result.applicable))
	require.Equal(t, []string{"number", "number"}, observedLevels(result.observed))
	require.True(t, evaluationQueryEmpty(result.failures))
}

func TestEvaluateNestedCompositionPreservesBranchOrderAndIdentity(t *testing.T) {
	t.Parallel()

	result := evaluateSchemaValue(
		t,
		`{"allOf":[{"anyOf":[{"type":"string","pattern":"^a"},{"type":"number"}]},`+
			`{"type":"boolean"}]}`,
		`"b"`,
	)

	require.NoError(t, result.err)
	require.False(t, result.valid)
	require.Equal(t, [][]bool{{false, false}}, compositionTruthVectorsForTest(result.allOf))
	require.Equal(t, [][]bool{{false, false}}, compositionTruthVectorsForTest(result.anyOf))
	require.Equal(
		t,
		[]string{
			"#/paths/~1/post/requestBody/content/application~1json/schema/allOf/0/anyOf/0|#|pattern",
			"#/paths/~1/post/requestBody/content/application~1json/schema/allOf/0/anyOf/1|#|type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/allOf/0|#|anyOf",
			"#/paths/~1/post/requestBody/content/application~1json/schema/allOf/1|#|type",
		},
		identityStrings(result.failures),
	)
}

func compositionTruthVectorsForTest(values any) [][]bool {
	var result [][]bool

	switch truths := values.(type) {
	case []compositionTruth:
		for _, truth := range truths {
			result = append(result, truth.branches)
		}
	case evaluationQuery[compositionTruth]:
		for truth := range truths {
			result = append(result, truth.branches)
		}
	default:
		panic("unsupported composition truth collection")
	}

	return result
}
