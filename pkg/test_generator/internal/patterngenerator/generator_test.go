//nolint:testpackage,godoclint // Internal graph and limit invariants need direct coverage.
package patterngenerator

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/stretchr/testify/require"
)

var defaultPatternOption patternvalidator.Option = func(*patternvalidator.PatternValidation) {}

func TestGeneratedValuesSatisfySignedConjunctionsAndLengths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		requirements []Requirement
		minimum      int
		maximum      *int
		option       patternvalidator.Option
	}{
		{
			name: "unanchored patterns match at different positions",
			requirements: []Requirement{
				{Source: "^A", WantMatch: true},
				{Source: "Z$", WantMatch: true},
			},
			minimum: 3, maximum: new(6), option: defaultPatternOption,
		},
		{
			name: "isolated first failure",
			requirements: []Requirement{
				{Source: "^[A-Z]+$", WantMatch: false},
				{Source: "^A", WantMatch: true},
			},
			minimum: 2, maximum: new(4), option: defaultPatternOption,
		},
		{
			name: "isolated second failure",
			requirements: []Requirement{
				{Source: "^[A-Z]+$", WantMatch: true},
				{Source: "^A", WantMatch: false},
			},
			minimum: 1, maximum: new(4), option: defaultPatternOption,
		},
		{
			name: "positive and negative leading lookahead",
			requirements: []Requirement{
				{Source: "^(?=a)(?!ab)a", WantMatch: true},
			},
			minimum: 2, maximum: new(3), option: defaultPatternOption,
		},
		{
			name: "word boundary and exact length",
			requirements: []Requirement{
				{Source: `\bword\b`, WantMatch: true},
			},
			minimum: 4, maximum: new(6), option: defaultPatternOption,
		},
		{
			name: "raw Go multiline anchors",
			requirements: []Requirement{
				{Source: `(?m)^a$`, WantMatch: true},
			},
			minimum: 2, maximum: new(4), option: patternvalidator.UseRE2,
		},
		{
			name:    "length only",
			minimum: 3, maximum: new(3), option: defaultPatternOption,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assertGeneratedValues(
				t,
				test.requirements,
				test.minimum,
				test.maximum,
				test.option,
				100,
			)
		})
	}
}

func TestEmptyAndUniversalLanguagesReturnNoValuesOnCorrectSide(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		requirement Requirement
		minimum     int
		maximum     *int
		wantEmpty   bool
	}{
		{name: "empty positive", requirement: Requirement{Source: "[]", WantMatch: true}, wantEmpty: true},
		{name: "empty negative", requirement: Requirement{Source: "[]", WantMatch: false}},
		{name: "universal positive", requirement: Requirement{Source: ".*", WantMatch: true}},
		{name: "universal negative", requirement: Requirement{Source: ".*", WantMatch: false}, wantEmpty: true},
		{
			name: "universal nonempty positive", requirement: Requirement{Source: "[^]", WantMatch: true},
			minimum: 1, maximum: new(2),
		},
		{
			name: "universal nonempty negative", requirement: Requirement{Source: "[^]", WantMatch: false},
			minimum: 1, maximum: new(2), wantEmpty: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			generator, err := Strings(
				[]Requirement{test.requirement},
				test.minimum,
				test.maximum,
				defaultPatternOption,
			)
			if test.wantEmpty {
				require.Nil(t, generator)
				require.ErrorIs(t, err, ErrNoValues)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, generator)
			assertGeneratedValues(
				t,
				[]Requirement{test.requirement},
				test.minimum,
				test.maximum,
				defaultPatternOption,
				30,
			)
		})
	}
}

func TestDuplicateAndImpliedSignedRequirementsAreProvenEmpty(t *testing.T) {
	t.Parallel()

	tests := [][]Requirement{
		{
			{Source: "^a$", WantMatch: true},
			{Source: "^a$", WantMatch: false},
		},
		{
			{Source: "^[a-z]+$", WantMatch: true},
			{Source: "^[a-z]*$", WantMatch: false},
		},
		{
			{Source: "^a$", WantMatch: true},
			{Source: "^b$", WantMatch: true},
		},
	}

	for _, requirements := range tests {
		generator, err := Strings(requirements, 0, new(4), defaultPatternOption)
		require.Nil(t, generator)
		require.ErrorIs(t, err, ErrNoValues)
	}
}

func TestDFAExactlyMatchesIndependentValidatorForAllShortASCIIValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pattern string
		option  patternvalidator.Option
	}{
		{pattern: ""},
		{pattern: "a"},
		{pattern: "^a$"},
		{pattern: `\b(?:a|1)\B`},
		{pattern: "[]"},
		{pattern: "[^]"},
		{pattern: "^[^0-9]+$"},
		{pattern: "^(?=a)(?!ab)a"},
		{pattern: `(?m)^a$`, option: patternvalidator.UseRE2},
		{pattern: `\A(?:a|b){1,2}\z`, option: patternvalidator.UseRE2},
	}

	for _, test := range tests {
		t.Run(test.pattern, func(t *testing.T) {
			t.Parallel()

			option := test.option
			if option == nil {
				option = defaultPatternOption
			}

			settings := new(patternvalidator.PatternValidation)
			option(settings)
			machine, err := compileRequirement(test.pattern, settings, &budget{limits: defaultLimits()})
			require.NoError(t, err)
			validation, err := patternvalidator.Parse(test.pattern, option)
			require.NoError(t, err)

			forEachASCIIString(2, func(value string) {
				require.Equal(
					t,
					validation.Validate(value),
					dfaAccepts(machine, value),
					"pattern %q value %q",
					test.pattern,
					value,
				)
			})
		})
	}
}

func TestCompiledSetMatchesSignedASCIIValues(t *testing.T) {
	t.Parallel()

	set, err := Compile([]string{"^[A-Z]+$", "^A"}, defaultPatternOption)
	require.NoError(t, err)

	for _, test := range []struct {
		value string
		want  []bool
		match bool
	}{
		{value: "ABC", want: []bool{true, true}, match: true},
		{value: "A1", want: []bool{false, true}, match: true},
		{value: "BCD", want: []bool{true, false}, match: true},
		{value: "ABC", want: []bool{false, true}},
		{value: "É", want: []bool{false, false}},
	} {
		matched, matchErr := set.Matches(test.value, test.want)
		require.NoError(t, matchErr)
		require.Equal(t, test.match, matched)
	}

	_, err = set.Matches("ABC", []bool{true})
	require.ErrorContains(t, err, "signed requirements")
}

func TestCertifiedProductMatchesExhaustiveShortLanguage(t *testing.T) {
	t.Parallel()

	tests := [][]Requirement{
		{{Source: "^a$", WantMatch: true}},
		{{Source: "^a$", WantMatch: false}},
		{{Source: ".*", WantMatch: false}},
		{{Source: "[]", WantMatch: false}},
		{
			{Source: "^a", WantMatch: true},
			{Source: "b$", WantMatch: true},
		},
		{
			{Source: "^[ab]+$", WantMatch: true},
			{Source: "^a$", WantMatch: false},
		},
	}

	for _, requirements := range tests {
		validations := make([]*patternvalidator.PatternValidation, 0, len(requirements))
		for _, requirement := range requirements {
			validation, err := patternvalidator.Parse(requirement.Source)
			require.NoError(t, err)

			validations = append(validations, validation)
		}

		hasValue := false

		forEachASCIIString(2, func(value string) {
			if signedValidationsAccept(validations, requirements, value) {
				hasValue = true
			}
		})

		generator, err := Strings(requirements, 0, new(2), defaultPatternOption)
		if !hasValue {
			require.Nil(t, generator)
			require.ErrorIs(t, err, ErrNoValues)

			continue
		}

		require.NoError(t, err)
		require.NotNil(t, generator)

		for seed := range 100 {
			value := generator.Example(seed)
			require.LessOrEqual(t, len(value), 2)
			require.True(t, signedValidationsAccept(validations, requirements, value), "%q", value)
		}
	}
}

func TestCommonSubsetFixtureConstructsRequestedSide(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("../../../patternvalidator/testdata/common_subset.json")
	require.NoError(t, err)

	var fixture struct {
		Cases []struct {
			Pattern  string `json:"pattern"`
			Input    string `json:"input"`
			Expected bool   `json:"expected"`
		} `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(contents, &fixture))

	for _, test := range fixture.Cases {
		t.Run(test.Pattern+"/"+test.Input, func(t *testing.T) {
			t.Parallel()

			length := utf8.RuneCountInString(test.Input)
			generator, constructionErr := Strings(
				[]Requirement{{Source: test.Pattern, WantMatch: test.Expected}},
				length,
				new(length),
				defaultPatternOption,
			)
			require.NoError(t, constructionErr)
			require.NotNil(t, generator)

			validation := patternvalidator.MustParse(test.Pattern)

			for seed := range 20 {
				value := generator.Example(seed)
				require.Equal(t, test.Expected, validation.Validate(value), "%q", value)
			}
		})
	}
}

func TestRawGoModeAndStrictASCIIPolicies(t *testing.T) {
	t.Parallel()

	generator, err := Strings(
		[]Requirement{{Source: `\A[[:alpha:]]+\z`, WantMatch: true}},
		1,
		new(4),
		patternvalidator.UseRE2,
	)
	require.NoError(t, err)
	require.NotNil(t, generator)

	for seed := range 50 {
		value := generator.Example(seed)
		require.True(t, patternvalidator.MustParse(
			`\A[[:alpha:]]+\z`,
			patternvalidator.UseRE2,
		).Validate(value))
	}

	for _, source := range []string{`(?i)a`, `(?i)[a]`, `(?i:[^a])`} {
		_, err = Strings(
			[]Requirement{{Source: source, WantMatch: true}},
			0,
			new(2),
			patternvalidator.UseRE2,
		)

		var capabilityError *CapabilityError
		require.ErrorAs(t, err, &capabilityError, source)
	}

	for _, source := range []string{"é", `\u00e9`, `[a-\u00e9]`} {
		_, err = Strings(
			[]Requirement{{Source: source, WantMatch: true}},
			0,
			new(2),
			patternvalidator.RejectNonASCII,
		)
		require.Error(t, err, source)
	}

	generator, err = Strings(
		[]Requirement{{Source: "é", WantMatch: false}},
		0,
		new(2),
		defaultPatternOption,
	)
	require.NoError(t, err)
	require.NotNil(t, generator)
}

func TestInputAndConstructionErrorsNeverBecomeNoValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		requirements []Requirement
		minimum      int
		maximum      *int
		option       patternvalidator.Option
	}{
		{name: "nil option", option: nil},
		{name: "negative minimum", minimum: -1, option: defaultPatternOption},
		{name: "negative maximum", maximum: new(-1), option: defaultPatternOption},
		{
			name: "invalid ES syntax", requirements: []Requirement{{Source: "[", WantMatch: true}},
			option: defaultPatternOption,
		},
		{
			name: "invalid raw syntax", requirements: []Requirement{{Source: "[", WantMatch: true}},
			option: patternvalidator.UseRE2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			generator, err := Strings(
				test.requirements,
				test.minimum,
				test.maximum,
				test.option,
			)
			require.Nil(t, generator)
			require.Error(t, err)
			require.NotErrorIs(t, err, ErrNoValues)
		})
	}
}

func TestCompiledSetReusesComponentMachinesAcrossSignedRequests(t *testing.T) {
	t.Parallel()

	optionCalls := 0
	set, err := Compile([]string{`^[ab]+$`, `^a`}, func(validation *patternvalidator.PatternValidation) {
		optionCalls++

		patternvalidator.RejectNonASCII(validation)
	})
	require.NoError(t, err)

	accepted, err := set.Strings([]bool{true, true}, 1, new(3))
	require.NoError(t, err)
	rejected, err := set.Strings([]bool{true, false}, 1, new(3))
	require.NoError(t, err)
	require.Equal(t, 1, optionCalls)

	for seed := range 20 {
		acceptedValue := accepted.Example(seed)
		require.True(t, patternvalidator.MustParse(`^[ab]+$`).Validate(acceptedValue))
		require.True(t, patternvalidator.MustParse(`^a`).Validate(acceptedValue))

		rejectedValue := rejected.Example(seed)
		require.True(t, patternvalidator.MustParse(`^[ab]+$`).Validate(rejectedValue))
		require.False(t, patternvalidator.MustParse(`^a`).Validate(rejectedValue))
	}
}

func TestEveryFrozenLimitRejectsOnlyAboveBoundary(t *testing.T) {
	t.Parallel()

	limits := defaultLimits()
	tests := []struct {
		name    string
		maximum uint64
	}{
		{name: "requirements", maximum: limits.requirements},
		{name: "cumulative source bytes", maximum: limits.cumulativeSourceBytes},
		{name: "cumulative AST nodes", maximum: limits.cumulativeASTNodes},
		{name: "NFA states", maximum: limits.nfaStates},
		{name: "NFA edges", maximum: limits.nfaEdges},
		{name: "DFA states", maximum: limits.dfaStates},
		{name: "DFA transitions", maximum: limits.dfaTransitions},
		{name: "product states", maximum: limits.productStates},
		{name: "product transitions", maximum: limits.productTransitions},
		{name: "graph bytes", maximum: limits.graphBytes},
		{name: "certification work", maximum: limits.certificationWork},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			counter := uint64(0)
			require.NoError(t, addLimited(
				&counter,
				test.maximum-1,
				test.maximum,
				"test",
				test.name,
			))
			require.NoError(t, addLimited(&counter, 1, test.maximum, "test", test.name))
			err := addLimited(&counter, 1, test.maximum, "test", test.name)
			require.ErrorIs(t, err, ErrTooComplex)

			var complexity *ComplexityError
			require.ErrorAs(t, err, &complexity)
			require.Equal(t, test.maximum, complexity.Maximum)
			require.Equal(t, test.maximum+1, complexity.Observed)
		})
	}

	_, err := outputAllowance(limits.generatedBytes-1, limits)
	require.NoError(t, err)
	extra, err := outputAllowance(limits.generatedBytes, limits)
	require.NoError(t, err)
	require.Zero(t, extra)

	_, err = outputAllowance(limits.generatedBytes+1, limits)
	require.ErrorIs(t, err, ErrTooComplex)

	limits.generatedBytes = MaximumGeneratedBytes
	limits.extraLength = MaximumExtraLength
	extra, err = outputAllowance(MaximumGeneratedBytes-MaximumExtraLength+1, limits)
	require.NoError(t, err)
	require.Equal(t, uint64(MaximumExtraLength-1), extra)

	extra, err = outputAllowance(MaximumGeneratedBytes-MaximumExtraLength, limits)
	require.NoError(t, err)
	require.Equal(t, uint64(MaximumExtraLength), extra)

	extra, err = outputAllowance(MaximumGeneratedBytes-MaximumExtraLength-1, limits)
	require.NoError(t, err)
	require.Equal(t, uint64(MaximumExtraLength), extra)

	atRequirementLimit := make([]Requirement, MaximumRequirements)
	for index := range atRequirementLimit {
		atRequirementLimit[index] = Requirement{Source: ".*", WantMatch: true}
	}

	generator, err := Strings(atRequirementLimit, 0, new(0), defaultPatternOption)
	require.NoError(t, err)
	require.NotNil(t, generator)

	_, err = Strings(
		append(atRequirementLimit, Requirement{Source: ".*", WantMatch: true}),
		0,
		new(0),
		defaultPatternOption,
	)
	require.ErrorIs(t, err, ErrTooComplex)
}

func TestComplexityErrorsOccurBeforeGeneratorIsReturned(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		limits func() constructionLimits
	}{
		{
			name: "source",
			limits: func() constructionLimits {
				limits := defaultLimits()
				limits.cumulativeSourceBytes = 0

				return limits
			},
		},
		{
			name: "AST",
			limits: func() constructionLimits {
				limits := defaultLimits()
				limits.cumulativeASTNodes = 1

				return limits
			},
		},
		{
			name: "NFA",
			limits: func() constructionLimits {
				limits := defaultLimits()
				limits.nfaStates = 1

				return limits
			},
		},
		{
			name: "DFA",
			limits: func() constructionLimits {
				limits := defaultLimits()
				limits.dfaStates = 1

				return limits
			},
		},
		{
			name: "product",
			limits: func() constructionLimits {
				limits := defaultLimits()
				limits.productStates = 1

				return limits
			},
		},
		{
			name: "graph bytes",
			limits: func() constructionLimits {
				limits := defaultLimits()
				limits.graphBytes = 1

				return limits
			},
		},
		{
			name: "certification",
			limits: func() constructionLimits {
				limits := defaultLimits()
				limits.certificationWork = 1

				return limits
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			generator, err := stringsWithLimits(
				[]Requirement{{Source: "a", WantMatch: true}},
				0,
				new(2),
				defaultPatternOption,
				test.limits(),
			)
			require.Nil(t, generator)
			require.ErrorIs(t, err, ErrTooComplex)
		})
	}

	limits := defaultLimits()
	limits.generatedBytes = 2
	generator, err := stringsWithLimits(
		[]Requirement{{Source: "^aaa$", WantMatch: true}},
		0,
		new(3),
		defaultPatternOption,
		limits,
	)
	require.Nil(t, generator)
	require.ErrorIs(t, err, ErrTooComplex)
}

func TestProductGraphNormalizesLengthBoundsAtOutputLimit(t *testing.T) {
	t.Parallel()

	limits := defaultLimits()
	limits.generatedBytes = 2
	limits.productStates = 1

	oversizedMaximum := 10_000
	generator, err := stringsWithLimits(
		nil,
		0,
		&oversizedMaximum,
		defaultPatternOption,
		limits,
	)
	require.NoError(t, err)

	for seed := range 20 {
		require.LessOrEqual(t, len(generator.Example(seed)), 2)
	}

	generator, err = stringsWithLimits(nil, 3, nil, defaultPatternOption, limits)
	require.Nil(t, generator)
	require.ErrorIs(t, err, ErrTooComplex)

	var complexity *ComplexityError
	require.ErrorAs(t, err, &complexity)
	require.Equal(t, "output", complexity.Phase)
	require.Equal(t, "generated bytes", complexity.Limit)
	require.Equal(t, uint64(2), complexity.Maximum)
	require.Equal(t, uint64(3), complexity.Observed)
}

func FuzzStringsNeverPanics(fuzz *testing.F) {
	for _, source := range []string{"", "a", "[", `\`, "^(?=a)a", "é", string([]byte{0xff})} {
		fuzz.Add(source, false)
	}

	fuzz.Fuzz(func(t *testing.T, source string, raw bool) {
		option := defaultPatternOption
		if raw {
			option = patternvalidator.UseRE2
		}

		generator, err := Strings(
			[]Requirement{{Source: source, WantMatch: true}},
			0,
			new(4),
			option,
		)
		if err == nil {
			require.NotNil(t, generator)
			_ = generator.Example(1)
		}
	})
}

func BenchmarkConstructCountedRepeat(benchmark *testing.B) {
	for benchmark.Loop() {
		generator, err := Strings(
			[]Requirement{{Source: "^(?:[A-Z][0-9]){32}$", WantMatch: true}},
			64,
			new(64),
			defaultPatternOption,
		)
		if err != nil {
			benchmark.Fatal(err)
		}

		_ = generator.Example(1)
	}
}

func BenchmarkConstructSignedConjunction(benchmark *testing.B) {
	requirements := []Requirement{
		{Source: "^[A-Za-z0-9_-]{1,32}$", WantMatch: true},
		{Source: "^admin", WantMatch: false},
		{Source: "[0-9]", WantMatch: true},
		{Source: `\s`, WantMatch: false},
	}

	for benchmark.Loop() {
		generator, err := Strings(requirements, 4, new(32), defaultPatternOption)
		if err != nil {
			benchmark.Fatal(err)
		}

		_ = generator.Example(1)
	}
}

func assertGeneratedValues(
	t *testing.T,
	requirements []Requirement,
	minimum int,
	maximum *int,
	option patternvalidator.Option,
	count int,
) {
	t.Helper()

	generator, err := Strings(requirements, minimum, maximum, option)
	require.NoError(t, err)
	require.NotNil(t, generator)

	validations := make([]*patternvalidator.PatternValidation, 0, len(requirements))
	for _, requirement := range requirements {
		validation, parseErr := patternvalidator.Parse(requirement.Source, option)
		require.NoError(t, parseErr)

		validations = append(validations, validation)
	}

	for seed := range count {
		value := generator.Example(seed)
		require.True(t, isASCII(value), "%q", value)
		require.GreaterOrEqual(t, len(value), minimum, "%q", value)

		if maximum != nil {
			require.LessOrEqual(t, len(value), *maximum, "%q", value)
		}

		require.True(t, signedValidationsAccept(validations, requirements, value), "%q", value)
	}
}

func signedValidationsAccept(
	validations []*patternvalidator.PatternValidation,
	requirements []Requirement,
	value string,
) bool {
	for index, validation := range validations {
		if validation.Validate(value) != requirements[index].WantMatch {
			return false
		}
	}

	return true
}

func dfaAccepts(machine *dfa, value string) bool {
	state := uint32(0)
	for index := range len(value) {
		state = machine.states[state].transitions[value[index]]
	}

	return machine.states[state].accepting
}

func forEachASCIIString(maximumLength int, visit func(string)) {
	visit("")

	for length := 1; length <= maximumLength; length++ {
		count := 1
		for range length {
			count *= asciiAlphabetSize
		}

		value := make([]byte, length)

		for encoded := range count {
			remainder := encoded
			for index := length - 1; index >= 0; index-- {
				value[index] = byte(remainder % asciiAlphabetSize)
				remainder /= asciiAlphabetSize
			}

			visit(string(value))
		}
	}
}

func isASCII(value string) bool {
	return strings.IndexFunc(value, func(character rune) bool {
		return character > 0x7f
	}) == -1
}
