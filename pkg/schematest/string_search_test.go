//nolint:godoclint // Test names state the locked product-graph behavior.
package schematest

import (
	"testing"
	"unicode/utf8"

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
	require.Equal(t, [][]int{{5, 7, 8}, {5}}, basicStringProductStateIDs(afterA))
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
		{low: 0x202a, high: 0xd7ff},
		{low: 0xd800, high: 0xdbff},
		{low: 0xdc00, high: 0xdfff},
		{low: 0xe000, high: 0xffff},
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

func TestBasicStringProductFindsGroupedAlternationIntersection(t *testing.T) {
	t.Parallel()

	patterns := parseBasicSearchPatterns(t, `^(a|b)c$`, `^(?:a)c$`)
	searchState := &search{maxSteps: 100}

	witness, found, err := searchState.findBasicStringWitness(patterns)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "ac", witness)
	require.Equal(t, uint64(7), searchState.steps)
}

func TestBasicStringProductPreservesLiteralUTF16Units(t *testing.T) {
	t.Parallel()

	patterns := parseBasicSearchPatterns(t, `^😀x$`)
	searchState := &search{maxSteps: 100}

	witness, found, err := searchState.findBasicStringWitness(patterns)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "😀x", witness)
	require.Equal(t, uint64(10), searchState.steps)
}

func TestBasicStringProductSearchesSimultaneousUnanchoredPatterns(t *testing.T) {
	t.Parallel()

	patterns := parseBasicSearchPatterns(t, `a`, `(b)`)
	searchState := &search{maxSteps: 100}

	witness, found, err := searchState.findBasicStringWitness(patterns)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "ab", witness)
	require.Equal(t, uint64(55), searchState.steps)
}

func TestBasicStringProductPadsContextualPatterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pattern string
		want    string
	}{
		{pattern: `\Ba\B`, want: "0a0"},
		{pattern: `^a\B`, want: "a0"},
		{pattern: `\Ba`, want: "0a"},
		{pattern: `^(?!a$)a`, want: "a\x00"},
		{pattern: `^(?![^]{0,2}$)a`, want: "a𐀀"},
	}

	for _, test := range tests {
		t.Run(test.pattern, func(t *testing.T) {
			t.Parallel()

			searchState := &search{maxSteps: 100_000}
			witness, found, err := searchState.findBasicStringWitness(
				parseBasicSearchPatterns(t, test.pattern),
			)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, test.want, witness)
		})
	}
}

func TestBasicStringProductFindsAstralRuneThroughDotUnits(t *testing.T) {
	t.Parallel()

	searchState := &search{maxSteps: 1_000}
	witness, found, err := searchState.findBasicStringWitnessAtLength(
		parseBasicSearchPatterns(t, `^..$`), 1,
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 1, utf8.RuneCountInString(witness))
	require.True(t, utf8.ValidString(witness))
	require.Greater(t, []rune(witness)[0], rune(0xffff))
}

func TestBasicStringProductSearchesLengthAbove4096(t *testing.T) {
	t.Parallel()

	const length = 5_000

	searchState := &search{maxSteps: length + 1}

	var witness string

	found, err := searchState.walkBasicStringProductForLengths(
		&basicStringProduct{unbounded: true, needsPadding: true, directedFormat: -1},
		basicStringLengths{boundaries: []uint64{length}, minimum: length},
		basicStringLengthObjective{},
		0,
		func(value *jsonValue) (bool, error) {
			witness = value.text

			return true, nil
		},
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, witness, length)
	require.Equal(t, uint64(length+1), searchState.steps)
}

func TestBasicStringProductHugeLengthStopsBeforeAllocation(t *testing.T) {
	t.Parallel()

	searchState := &search{maxSteps: 3}
	found, err := searchState.walkBasicStringProductForLengths(
		&basicStringProduct{unbounded: true, needsPadding: true, directedFormat: -1},
		basicStringLengths{boundaries: []uint64{1_000_000_000}, minimum: 1_000_000_000},
		basicStringLengthObjective{},
		0,
		func(*jsonValue) (bool, error) {
			t.Fatal("huge candidate must not complete")

			return false, nil
		},
	)
	require.ErrorIs(t, err, errMaxSteps)
	require.False(t, found)
	require.Equal(t, uint64(3), searchState.steps)
}

func TestBasicStringProductPreservesMixedAlternativeAnchors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		patterns []string
		want     string
	}{
		{name: "restart unanchored alternative", patterns: []string{`^a|b$`, `^xb$`}, want: "xb"},
		{name: "anchored prefix", patterns: []string{`^a|b$`, `^ac$`}, want: "ac"},
		{name: "finite sibling", patterns: []string{`^a|b$`, `^..$`}, want: "\x00b"},
		{name: "boundary restart", patterns: []string{`^z|\Bb`, `^ab$`}, want: "ab"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			searchState := &search{maxSteps: 10_000}
			witness, found, err := searchState.findBasicStringWitness(
				parseBasicSearchPatterns(t, test.patterns...),
			)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, test.want, witness)
		})
	}
}

func TestBasicStringProductNegativeAssertionExpandsSurrogates(t *testing.T) {
	t.Parallel()

	searchState := &search{maxSteps: 10_000}
	witness, found, err := searchState.findBasicStringWitnessAtLength(
		parseBasicSearchPatterns(t, `^(?![^]$)[^]`), 1,
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "𐀀", witness)
}

func TestBasicStringProductPaddingUsesNormativeIntervalOrder(t *testing.T) {
	t.Parallel()

	searchState := &search{maxSteps: 10_000}
	witness, found, err := searchState.findBasicStringWitness(
		parseBasicSearchPatterns(t, `^(?!z$)[^]$`),
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "\x00", witness)
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

func TestBasicStringProductSearchesQuantifiersAnchorsAndBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		patterns []string
		want     string
		found    bool
	}{
		{name: "empty star", patterns: []string{`^a*$`}, want: "", found: true},
		{name: "finite greedy", patterns: []string{`^a{2,3}$`}, want: "aa", found: true},
		{name: "finite lazy", patterns: []string{`^a{2,3}?$`}, want: "aa", found: true},
		{name: "unbounded plus", patterns: []string{`^a+$`, `^aaa$`}, want: "aaa", found: true},
		{name: "unicode repetition", patterns: []string{`^(😀){2}$`}, want: "😀😀", found: true},
		{name: "word boundary", patterns: []string{`^\ba\b$`}, want: "a", found: true},
		{name: "non-word boundary", patterns: []string{`^\Ba\B$`}, found: false},
		{name: "contradictory boundaries", patterns: []string{`^\ba$`, `^\Ba$`}, found: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			searchState := &search{maxSteps: 10_000}
			witness, found, err := searchState.findBasicStringWitness(
				parseBasicSearchPatterns(t, test.patterns...),
			)
			require.NoError(t, err)
			require.Equal(t, test.found, found)
			require.Equal(t, test.want, witness)
		})
	}
}

func TestBasicStringProductSearchesLeadingAssertionsInAuthoredOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		want    string
		steps   uint64
	}{
		{name: "positive", pattern: `^(?=ab)ab$`, want: "ab", steps: 21},
		{name: "negative", pattern: `^(?!a)[a-b]$`, want: "b", steps: 7},
		{name: "mixed consecutive", pattern: `^(?=a)(?!b)[a-c]$`, want: "a", steps: 6},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			searchState := &search{maxSteps: 100}
			witness, found, err := searchState.findBasicStringWitness(
				parseBasicSearchPatterns(t, test.pattern),
			)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, test.want, witness)
			require.Equal(t, test.steps, searchState.steps)
		})
	}
}

func TestBasicStringProductLeadingAssertionObjectivesKeepAuthoredOrder(t *testing.T) {
	t.Parallel()

	product, err := newBasicStringProduct(parseBasicSearchPatterns(t, `^(?=a)(?!b)a$`))
	require.NoError(t, err)
	require.Len(t, product.machines, 3)
	require.Equal(t, []bool{true, false, true}, []bool{
		product.machines[0].expected,
		product.machines[1].expected,
		product.machines[2].expected,
	})
	require.Empty(t, product.machines[0].restartStates)
	require.Empty(t, product.machines[1].restartStates)
	require.Empty(t, product.machines[2].restartStates)
}

func TestBasicStringProductNegativeAssertionPreservesSiblingPattern(t *testing.T) {
	t.Parallel()

	searchState := &search{maxSteps: 100}
	witness, found, err := searchState.findBasicStringWitness(
		parseBasicSearchPatterns(t, `^(?!a)[a-b]$`, `^b$`),
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "b", witness)
	require.Equal(t, uint64(7), searchState.steps)
}

func TestBasicStringProductLeadingAssertionContradictionStopsAtBudget(t *testing.T) {
	t.Parallel()

	searchState := &search{maxSteps: 4}
	witness, found, err := searchState.findBasicStringWitness(
		parseBasicSearchPatterns(t, `^(?=a)(?!a)a$`),
	)
	require.ErrorIs(t, err, errMaxSteps)
	require.False(t, found)
	require.Empty(t, witness)
	require.Equal(t, uint64(4), searchState.steps)
}

func TestBuildSearchesLeadingAssertionsWithoutFallbackCandidates(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"pattern":"^(?=ab)(?!ac)a.$"
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
	require.Contains(t, cases, Case{JSON: []byte(`"ab"`), Valid: true})
	require.Equal(t, SpaceExhausted, report.Stop)
}

func TestBasicStringProductUsesExactES51WhitespaceSet(t *testing.T) {
	t.Parallel()

	whitespace := []uint16{
		0x0009, 0x000a, 0x000b, 0x000c, 0x000d, 0x0020, 0x00a0, 0x1680,
		0x180e, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006,
		0x2007, 0x2008, 0x2009, 0x200a, 0x200b, 0x2028, 0x2029, 0x202f,
		0x3000, 0xfeff,
	}
	pattern := parseBasicSearchPatterns(t, `^\s$`)
	product, err := newBasicStringProduct(pattern)
	require.NoError(t, err)

	state := product.start(1)
	for _, unit := range whitespace {
		require.True(t, product.accepting(product.advance(state, unit, 0, 1)), "U+%04X", unit)
	}

	for _, unit := range []uint16{0x0008, 0x000e, 0x005f, 0x180d, 0x180f, 0x200c, 0x205f, 0xff00} {
		require.False(t, product.accepting(product.advance(state, unit, 0, 1)), "U+%04X", unit)
	}
}

func TestBasicStringLengthOrderPinsBoundariesThenFairLengths(t *testing.T) {
	t.Parallel()

	maximum := uint64(4)
	lengths := basicStringLengths{
		boundaries: []uint64{2, 4, 2},
		maximum:    maximum,
		hasMaximum: true,
	}
	product := &basicStringProduct{maxUnits: 3}
	got := make([]uint64, 0)

	lengths.each(product, basicStringLengthObjective{length: 3, pinned: true}, func(length uint64) bool {
		got = append(got, length)

		return false
	})

	require.Equal(t, []uint64{3, 2, 4, 0, 1}, got)
}

func TestBasicStringProductSearchesExactRuneLength(t *testing.T) {
	t.Parallel()

	searchState := &search{maxSteps: 100}
	witness, found, err := searchState.findBasicStringWitnessAtLength(
		parseBasicSearchPatterns(t, `^😀$`),
		1,
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "😀", witness)
	require.Equal(t, uint64(3), searchState.steps)
}

func TestBasicStringProductRetriesChargeTheGlobalCounter(t *testing.T) {
	t.Parallel()

	searchState := &search{maxSteps: 4}
	witness, found, err := searchState.findBasicStringWitnessAtLength(
		parseBasicSearchPatterns(t, `^[a-b]$`, `^b$`),
		1,
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "b", witness)
	require.Equal(t, uint64(3), searchState.steps)
}

func TestBuildSearchesPatternsAtActiveLengthBoundaries(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"minLength":3,
		"maxLength":3,
		"allOf":[
			{"pattern":"^a..$"},
			{"pattern":"^..z$"}
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
	require.Contains(t, cases, Case{JSON: []byte(`"a\u0000z"`), Valid: true})
	require.Equal(t, MaxStepsReached, report.Stop)
}

func TestBasicStringProductDeduplicatesEqualCandidates(t *testing.T) {
	t.Parallel()

	searchState := &search{maxSteps: 100}
	witnessCount := 0
	found, err := searchState.walkBasicStringWitnesses(
		parseBasicSearchPatterns(t, `^(a|a)$`),
		0,
		func(value *jsonValue) (bool, error) {
			require.Equal(t, "a", value.text)

			witnessCount++

			return false, nil
		},
	)
	require.NoError(t, err)
	require.False(t, found)
	require.Equal(t, 1, witnessCount)
}

func TestBasicStringProductExhaustsContradictoryFiniteLengths(t *testing.T) {
	t.Parallel()

	searchState := &search{maxSteps: 100}
	found, err := searchState.walkBasicStringWitnessesForLengths(
		parseBasicSearchPatterns(t, `^a*$`),
		basicStringLengths{
			boundaries: []uint64{2, 1},
			minimum:    2,
			maximum:    1,
			hasMaximum: true,
		},
		basicStringLengthObjective{},
		0,
		func(*jsonValue) (bool, error) { return false, nil },
	)
	require.NoError(t, err)
	require.False(t, found)
	require.Equal(t, uint64(3), searchState.steps)
}

func TestBasicStringProductLeavesUnboundedContradictionToGlobalBudget(t *testing.T) {
	t.Parallel()

	searchState := &search{maxSteps: 20}
	witness, found, err := searchState.findBasicStringWitness(
		parseBasicSearchPatterns(t, `^a+$`, `^b+$`),
	)
	require.ErrorIs(t, err, errMaxSteps)
	require.False(t, found)
	require.Empty(t, witness)
	require.Equal(t, uint64(20), searchState.steps)
}

func TestBasicStringProductChargesLengthAndCountedRepeatEdges(t *testing.T) {
	t.Parallel()

	patterns := parseBasicSearchPatterns(t, `^a{2}$`)
	searchState := &search{maxSteps: 5}

	witness, found, err := searchState.findBasicStringWitness(patterns)
	require.ErrorIs(t, err, errMaxSteps)
	require.False(t, found)
	require.Empty(t, witness)
	require.Equal(t, uint64(5), searchState.steps)
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

	product.eachTransition(state, 0, false, func(unit uint16) bool {
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
	return s.findBasicStringWitnessForObjective(patterns, basicStringLengthObjective{}, seed)
}

func (s *search) findBasicStringWitnessAtLength(patterns []*patternAST, length uint64) (string, bool, error) {
	return s.findBasicStringWitnessForObjective(
		patterns,
		basicStringLengthObjective{length: length, pinned: true},
		0,
	)
}

func (s *search) findBasicStringWitnessForObjective(
	patterns []*patternAST,
	objective basicStringLengthObjective,
	seed uint64,
) (string, bool, error) {
	var witness string

	found, err := s.walkBasicStringWitnessesForLengths(
		patterns,
		basicStringLengths{},
		objective,
		seed,
		func(value *jsonValue) (bool, error) {
			witness = value.text

			return true, nil
		},
	)

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
