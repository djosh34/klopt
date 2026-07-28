//nolint:godoclint // Test names document the public behavior under characterization.
package stringlanguage_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // Public-seam test of the required shared module.
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/stretchr/testify/require"
)

func TestCompileGeneratesDeterministicSignedPatternValues(t *testing.T) {
	t.Parallel()

	startsWithA, err := stringlanguage.Pattern("^A")
	require.NoError(t, err)
	endsWithZ, err := stringlanguage.Pattern("Z$")
	require.NoError(t, err)

	set, err := stringlanguage.Compile(
		[]stringlanguage.Requirement{
			{Language: startsWithA, WantMatch: true},
			{Language: endsWithZ, WantMatch: true},
		},
		stringlanguage.Length{Min: 2, Max: new(4)},
	)
	require.NoError(t, err)

	value := set.Generate(42)
	require.Equal(t, value, set.Generate(42))
	require.GreaterOrEqual(t, len(value), 2)
	require.LessOrEqual(t, len(value), 4)
	require.True(t, set.Matches(value))
}

func TestGeneratedValuesSatisfySignedPatternsAndByteLengths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		patterns []struct {
			source    string
			wantMatch bool
		}
		length  stringlanguage.Length
		options []patternvalidator.Option
	}{
		{
			name: "unanchored patterns match at different positions",
			patterns: []struct {
				source    string
				wantMatch bool
			}{
				{source: "^A", wantMatch: true},
				{source: "Z$", wantMatch: true},
			},
			length: stringlanguage.Length{Min: 3, Max: new(6)},
		},
		{
			name: "isolated first failure",
			patterns: []struct {
				source    string
				wantMatch bool
			}{
				{source: "^[A-Z]+$", wantMatch: false},
				{source: "^A", wantMatch: true},
			},
			length: stringlanguage.Length{Min: 2, Max: new(4)},
		},
		{
			name: "positive and negative leading lookahead",
			patterns: []struct {
				source    string
				wantMatch bool
			}{
				{source: "^(?=a)(?!ab)a", wantMatch: true},
			},
			length: stringlanguage.Length{Min: 2, Max: new(3)},
		},
		{
			name: "raw Go multiline anchors",
			patterns: []struct {
				source    string
				wantMatch bool
			}{
				{source: `(?m)^a$`, wantMatch: true},
			},
			length:  stringlanguage.Length{Min: 2, Max: new(4)},
			options: []patternvalidator.Option{patternvalidator.UseRE2},
		},
		{
			name:   "length only",
			length: stringlanguage.Length{Min: 3, Max: new(3)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			requirements := make([]stringlanguage.Requirement, 0, len(test.patterns))
			for _, pattern := range test.patterns {
				language, err := stringlanguage.Pattern(pattern.source, test.options...)
				require.NoError(t, err)

				requirements = append(requirements, stringlanguage.Requirement{
					Language: language, WantMatch: pattern.wantMatch,
				})
			}

			set, err := stringlanguage.Compile(requirements, test.length)
			require.NoError(t, err)

			values := make(map[string]struct{})

			for seed := range uint64(100) {
				value := set.Generate(seed)
				require.True(t, set.Matches(value), "%q", value)
				require.GreaterOrEqual(t, len(value), test.length.Min, "%q", value)

				if test.length.Max != nil {
					require.LessOrEqual(t, len(value), *test.length.Max, "%q", value)
				}

				values[value] = struct{}{}
			}

			if test.length.Max == nil || *test.length.Max > test.length.Min {
				require.Greater(t, len(values), 1)
			}
		})
	}
}

func TestPatternMatchesExistingValidatorSemantics(t *testing.T) {
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

			options := []patternvalidator.Option(nil)
			if test.option != nil {
				options = append(options, test.option)
			}

			language, err := stringlanguage.Pattern(test.pattern, options...)
			require.NoError(t, err)

			set, err := stringlanguage.Compile(
				[]stringlanguage.Requirement{{Language: language, WantMatch: true}},
				stringlanguage.Length{Max: new(2)},
			)
			if errors.As(err, new(*stringlanguage.EmptyError)) {
				set = nil
			} else {
				require.NoError(t, err)
			}

			validation, err := patternvalidator.Parse(test.pattern, options...)
			require.NoError(t, err)

			forEachString([]byte{'\x00', '\n', ' ', '1', 'A', '_', 'a', 'b'}, 2, func(value string) {
				actual := false
				if set != nil {
					actual = set.Matches(value)
				}

				require.Equal(t, validation.Validate(value), actual, "value %q", value)
			})
		})
	}
}

func TestCompileReportsTypedEmptyAndComplexityErrors(t *testing.T) {
	t.Parallel()

	a, err := stringlanguage.Pattern("^a$")
	require.NoError(t, err)

	_, err = stringlanguage.Compile(
		[]stringlanguage.Requirement{
			{Language: a, WantMatch: true},
			{Language: a, WantMatch: false},
		},
		stringlanguage.Length{Max: new(4)},
	)

	var emptyError *stringlanguage.EmptyError
	require.ErrorAs(t, err, &emptyError)

	requirements := make([]stringlanguage.Requirement, 17)
	for index := range requirements {
		requirements[index] = stringlanguage.Requirement{Language: a, WantMatch: true}
	}

	_, err = stringlanguage.Compile(requirements, stringlanguage.Length{})

	var complexityError *stringlanguage.ComplexityError
	require.ErrorAs(t, err, &complexityError)
	require.Equal(t, uint64(16), complexityError.Limit)
	require.Equal(t, uint64(17), complexityError.Observed)
}

func TestCompileRejectsLanguagesWhoseShortestValueExceedsTheOutputLimit(t *testing.T) {
	t.Parallel()

	language, err := stringlanguage.Pattern("^a{257}$")
	require.NoError(t, err)

	set, err := stringlanguage.Compile(
		[]stringlanguage.Requirement{{Language: language, WantMatch: true}},
		stringlanguage.Length{},
	)
	require.Nil(t, set)

	var complexityError *stringlanguage.ComplexityError
	require.ErrorAs(t, err, &complexityError)
	require.Equal(t, "generated bytes", complexityError.Resource)
	require.Equal(t, uint64(256), complexityError.Limit)
	require.Equal(t, uint64(257), complexityError.Observed)
}

func TestLengthBoundsAreExactAndInvalidBoundsAreTyped(t *testing.T) {
	t.Parallel()

	set, err := stringlanguage.Compile(nil, stringlanguage.Length{Min: 2, Max: new(3)})
	require.NoError(t, err)
	require.False(t, set.Matches("a"))
	require.True(t, set.Matches("ab"))
	require.True(t, set.Matches("abc"))
	require.False(t, set.Matches("abcd"))

	_, err = stringlanguage.Compile(nil, stringlanguage.Length{Min: 2, Max: new(1)})

	var emptyError *stringlanguage.EmptyError
	require.ErrorAs(t, err, &emptyError)

	for _, length := range []stringlanguage.Length{{Min: -1}, {Max: new(-1)}} {
		_, err = stringlanguage.Compile(nil, length)

		var compileError *stringlanguage.CompileError
		require.ErrorAs(t, err, &compileError)
	}
}

func TestPatternOptionsAndErrorsUsePublicTypes(t *testing.T) {
	t.Parallel()

	for _, source := range []string{"é", `\u00e9`, `[a-\u00e9]`} {
		_, err := stringlanguage.Pattern(source, patternvalidator.RejectNonASCII)

		var compileError *stringlanguage.CompileError
		require.ErrorAs(t, err, &compileError, source)
	}

	for _, source := range []string{"[", `(?i)a`} {
		_, err := stringlanguage.Pattern(source, patternvalidator.UseRE2)

		var compileError *stringlanguage.CompileError
		require.ErrorAs(t, err, &compileError, source)
	}

	_, err := stringlanguage.Pattern("a", nil)

	var compileError *stringlanguage.CompileError
	require.ErrorAs(t, err, &compileError)

	_, err = stringlanguage.Format("hostname")
	require.ErrorAs(t, err, &compileError)
	require.ErrorContains(t, err, "unsupported string format")
}

func TestNonASCIIValuesAreOutsidePatternLanguages(t *testing.T) {
	t.Parallel()

	language, err := stringlanguage.Pattern("é")
	require.NoError(t, err)
	set, err := stringlanguage.Compile(
		[]stringlanguage.Requirement{{Language: language, WantMatch: false}},
		stringlanguage.Length{Max: new(2)},
	)
	require.NoError(t, err)
	require.False(t, set.Matches("é"))

	for seed := range uint64(50) {
		require.NotContains(t, set.Generate(seed), "é")
	}
}

func forEachString(alphabet []byte, maximumLength int, visit func(string)) {
	visit("")

	for length := 1; length <= maximumLength; length++ {
		count := 1
		for range length {
			count *= len(alphabet)
		}

		value := strings.Repeat("\x00", length)
		bytes := []byte(value)

		for encoded := range count {
			remainder := encoded
			for index := length - 1; index >= 0; index-- {
				bytes[index] = alphabet[remainder%len(alphabet)]
				remainder /= len(alphabet)
			}

			visit(string(bytes))
		}
	}
}
