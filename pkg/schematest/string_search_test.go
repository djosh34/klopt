//nolint:godoclint // Test names state the locked product-graph behavior.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBasicStringProductTransitionsAreDeterministic(t *testing.T) {
	t.Parallel()

	left, err := parseECMAPattern(`^(a|b)c$`)
	require.NoError(t, err)
	right, err := parseECMAPattern(`^ac$`)
	require.NoError(t, err)

	product, err := newBasicStringProduct([]*patternAST{left, right})
	require.NoError(t, err)

	state := product.start(2)
	require.Equal(t, [][]int{{0, 2, 4, 6, 9}, {0, 2, 4}}, basicStringProductStateIDs(state))
	require.Equal(t, []uint16{'a', 'b'}, product.transitions(state, 0, 2))

	afterA := product.advance(state, 'a', 0, 2)
	require.Equal(t, [][]int{{0, 2, 5, 7, 8}, {0, 2, 5}}, basicStringProductStateIDs(afterA))
	require.Equal(t, []uint16{'c'}, product.transitions(afterA, 1, 2))
}

func TestBasicStringProductFindsGroupedAlternationIntersection(t *testing.T) {
	t.Parallel()

	patterns := parseBasicSearchPatterns(t, `^(a|b)c$`, `^(?:a)c$`)
	searchState := &search{maxSteps: 100}

	witness, found, err := searchState.findBasicStringWitness(patterns)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "ac", witness)
	require.Equal(t, uint64(4), searchState.steps)
}

func TestBasicStringProductPreservesLiteralUTF16Units(t *testing.T) {
	t.Parallel()

	patterns := parseBasicSearchPatterns(t, `^😀x$`)
	searchState := &search{maxSteps: 100}

	witness, found, err := searchState.findBasicStringWitness(patterns)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "😀x", witness)
	require.Equal(t, uint64(6), searchState.steps)
}

func TestBasicStringProductSearchesSimultaneousUnanchoredPatterns(t *testing.T) {
	t.Parallel()

	patterns := parseBasicSearchPatterns(t, `a`, `(b)`)
	searchState := &search{maxSteps: 100}

	witness, found, err := searchState.findBasicStringWitness(patterns)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "ab", witness)
	require.Equal(t, uint64(4), searchState.steps)
}

func TestBasicStringProductChargesBeforeEveryEdge(t *testing.T) {
	t.Parallel()

	patterns := parseBasicSearchPatterns(t, `^ab$`)
	searchState := &search{maxSteps: 1}

	witness, found, err := searchState.findBasicStringWitness(patterns)
	require.ErrorIs(t, err, errMaxSteps)
	require.False(t, found)
	require.Empty(t, witness)
	require.Equal(t, uint64(1), searchState.steps)
}

func TestBasicStringProductExhaustsContradictoryFinitePatterns(t *testing.T) {
	t.Parallel()

	patterns := parseBasicSearchPatterns(t, `^a$`, `^b$`)
	searchState := &search{maxSteps: 100}

	witness, found, err := searchState.findBasicStringWitness(patterns)
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, witness)
	require.Equal(t, uint64(4), searchState.steps)
}

func TestBuildUsesOneBasicStringProductForActiveAllOfPatterns(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"allOf":[
			{"pattern":"^(a|b)c$"},
			{"pattern":"^ac$"}
		]
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
	require.Contains(t, cases, Case{JSON: []byte(`"ac"`), Valid: true})
	require.Equal(t, SpaceExhausted, report.Stop)
}

func (product *basicStringProduct) transitions(
	state basicStringProductState,
	_ int,
	_ int,
) []uint16 {
	units := make([]uint16, 0)

	product.eachTransition(state, func(unit uint16) bool {
		units = append(units, unit)

		return false
	})

	return units
}

func (s *search) findBasicStringWitness(patterns []*patternAST) (string, bool, error) {
	var witness string

	found, err := s.walkBasicStringWitnesses(patterns, func(value *jsonValue) (bool, error) {
		witness = value.text

		return true, nil
	})

	return witness, found, err
}

func basicStringProductStateIDs(state basicStringProductState) [][]int {
	identities := make([][]int, len(state.patterns))
	for index := range state.patterns {
		identities[index] = append([]int(nil), state.patterns[index].active...)
	}

	return identities
}

func parseBasicSearchPatterns(t *testing.T, sources ...string) []*patternAST {
	t.Helper()

	patterns := make([]*patternAST, 0, len(sources))
	for _, source := range sources {
		pattern, err := parseECMAPattern(source)
		require.NoError(t, err)

		patterns = append(patterns, pattern)
	}

	return patterns
}
