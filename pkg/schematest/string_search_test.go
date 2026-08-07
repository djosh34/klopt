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

func TestBasicStringProductIntervalsAndSeededCandidates(t *testing.T) {
	t.Parallel()

	patterns := parseBasicSearchPatterns(t, `^[\u0100-\u0105]$`)
	product, err := newBasicStringProduct(patterns)
	require.NoError(t, err)

	state := product.start(1)
	require.Equal(t, []basicStringInterval{{low: 0x0100, high: 0x0105}}, product.intervals(state))
	require.Equal(t, []uint16{0x0100, 0x0105, 0x0104}, basicStringIntervalCandidates(
		basicStringInterval{low: 0x0100, high: 0x0105},
		3,
	))
}

func TestBasicStringProductBuildsClassDotAndEscapeWitnesses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		want    string
		found   bool
	}{
		{name: "range", pattern: `^[a-c]$`, want: "a", found: true},
		{name: "negated", pattern: `^[^a]$`, want: "\x00", found: true},
		{name: "empty", pattern: `^[]$`, found: false},
		{name: "universal", pattern: `^[^]$`, want: "\x00", found: true},
		{name: "digit", pattern: `^\d$`, want: "0", found: true},
		{name: "not digit", pattern: `^\D$`, want: "\x00", found: true},
		{name: "word", pattern: `^\w$`, want: "0", found: true},
		{name: "not word", pattern: `^\W$`, want: "\x00", found: true},
		{name: "whitespace", pattern: `^\s$`, want: "\t", found: true},
		{name: "not whitespace", pattern: `^\S$`, want: "\x00", found: true},
		{name: "dot", pattern: `^.$`, want: "\x00", found: true},
		{name: "escapes", pattern: `^\cA\x42\u0043\.$`, want: "\x01BC.", found: true},
		{name: "class backspace", pattern: `^[\b]$`, want: "\b", found: true},
		{name: "non ASCII", pattern: `^[Ā-Ă]$`, want: "Ā", found: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			searchState := &search{maxSteps: 1000}
			witness, found, err := searchState.findBasicStringWitnessWithSeed(
				parseBasicSearchPatterns(t, test.pattern),
				0,
			)
			require.NoError(t, err)
			require.Equal(t, test.found, found)
			require.Equal(t, test.want, witness)
		})
	}
}

func TestBasicStringProductIntersectsClassBoundaries(t *testing.T) {
	t.Parallel()

	searchState := &search{maxSteps: 100}
	witness, found, err := searchState.findBasicStringWitnessWithSeed(
		parseBasicSearchPatterns(t, `^[a-z]$`, `^[m-z]$`, `^[^n-z]$`),
		0,
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "m", witness)
}

func TestBasicStringProductDotExcludesExactlyFourUTF16Units(t *testing.T) {
	t.Parallel()

	pattern := parseBasicSearchPatterns(t, `^.$`)
	product, err := newBasicStringProduct(pattern)
	require.NoError(t, err)

	require.Equal(t, []basicStringInterval{
		{low: 0x0000, high: 0x0009},
		{low: 0x000b, high: 0x000c},
		{low: 0x000e, high: 0x2027},
		{low: 0x202a, high: 0xffff},
	}, product.intervals(product.start(1)))
}

func TestBasicStringProductEmitsOnlyPairedSurrogates(t *testing.T) {
	t.Parallel()

	searchState := &search{maxSteps: 100}
	witness, found, err := searchState.findBasicStringWitnessWithSeed(
		parseBasicSearchPatterns(t, `^[😀][😀]$`),
		0,
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "😀", witness)
}

func TestBasicStringSeedUsesLockedDomainAndFields(t *testing.T) {
	t.Parallel()

	require.Equal(t, uint64(0x2740e16489cf1844), basicStringSeed(
		"#/schema",
		[]byte(`{"pattern":"^[a-z]$","type":"string"}`),
		"pattern",
		"valid",
	))
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

	product.eachTransition(state, 0, func(unit uint16) bool {
		units = append(units, unit)

		return false
	})

	return units
}

func (product *basicStringProduct) intervals(state basicStringProductState) []basicStringInterval {
	intervals := make([]basicStringInterval, 0)

	product.eachInterval(state, func(interval basicStringInterval) bool {
		intervals = append(intervals, interval)

		return false
	})

	return intervals
}

func (s *search) findBasicStringWitness(patterns []*patternAST) (string, bool, error) {
	return s.findBasicStringWitnessWithSeed(patterns, 0)
}

func (s *search) findBasicStringWitnessWithSeed(patterns []*patternAST, seed uint64) (string, bool, error) {
	var witness string

	found, err := s.walkBasicStringWitnesses(patterns, seed, func(value *jsonValue) (bool, error) {
		witness = value.text

		return true, nil
	})

	return witness, found, err
}

func basicStringIntervalCandidates(interval basicStringInterval, seed uint64) []uint16 {
	candidates := make([]uint16, 0, uint32(interval.high)-uint32(interval.low)+1)
	eachBasicStringIntervalCandidate(interval, seed, func(unit uint16) bool {
		candidates = append(candidates, unit)

		return false
	})

	return candidates
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
