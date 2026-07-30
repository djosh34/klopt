//nolint:godoclint // Test names document the public behavior under characterization.
package stringlanguage_test

import (
	"encoding/binary"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/djosh34/klopt/pkg/internal/program"        //nolint:depguard // Public test of the confirmed lowering seam.
	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // Public-seam test of the required shared module.
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/stretchr/testify/require"
)

func TestLengthCountsUnicodeCodePoints(t *testing.T) {
	t.Parallel()

	set, err := stringlanguage.Compile(nil, stringlanguage.Length{Min: 1, Max: new(1)})
	require.NoError(t, err)

	for _, value := range []string{"a", "é", "界", "😀"} {
		require.True(t, set.Matches(value), "%q", value)
	}

	for _, value := range []string{"", "a😀", string([]byte{0xff})} {
		require.False(t, set.Matches(value), "%q", value)
	}
}

func TestPatternMatchesUnicodeWithECMAScriptUTF16Progress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{pattern: "^é$", value: "é", want: true},
		{pattern: "^😀$", value: "😀", want: true},
		{pattern: "^.$", value: "😀", want: false},
		{pattern: "^..$", value: "😀", want: true},
		{pattern: `^\s$`, value: "\u2028", want: true},
		{pattern: `^\S$`, value: "界", want: true},
	}

	for _, test := range tests {
		language, err := stringlanguage.Pattern(test.pattern)
		require.NoError(t, err)

		set, err := stringlanguage.Compile(
			[]stringlanguage.Requirement{{Language: language, WantMatch: true}},
			stringlanguage.Length{Max: new(1)},
		)
		if test.want {
			require.NoError(t, err)
			require.True(t, set.Matches(test.value), "pattern %q", test.pattern)
		} else if err == nil {
			require.False(t, set.Matches(test.value), "pattern %q", test.pattern)
		} else {
			var emptyError *stringlanguage.EmptyError
			require.ErrorAs(t, err, &emptyError)
		}
	}
}

func TestUnicodePatternEdgeCasesMatchECMAScriptAndRawGoSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		option  patternvalidator.Option
		values  map[string]bool
	}{
		{
			name:    "astral quantifier applies to the trailing UTF-16 code unit",
			pattern: "^😀+$",
			values:  map[string]bool{"😀": true, "😀😀": false},
		},
		{
			name:    "astral class contains its two UTF-16 code units",
			pattern: "^[😀][😀]$",
			values:  map[string]bool{"😀": true, "😀😀": false},
		},
		{
			name:    "ECMAScript dot excludes line separator",
			pattern: "^.$",
			values:  map[string]bool{"\u2028": false, "界": true},
		},
		{
			name:    "raw Go dot advances by scalar",
			pattern: "^.$",
			option:  patternvalidator.UseRE2,
			values:  map[string]bool{"😀": true},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			options := []patternvalidator.Option(nil)
			if test.option != nil {
				options = append(options, test.option)
			}

			language, err := stringlanguage.Pattern(test.pattern, options...)
			require.NoError(t, err)

			set, err := stringlanguage.Compile(
				[]stringlanguage.Requirement{{Language: language, WantMatch: true}},
				stringlanguage.Length{Max: new(3)},
			)
			if err != nil {
				var emptyError *stringlanguage.EmptyError
				require.ErrorAs(t, err, &emptyError)
			}

			for value, want := range test.values {
				got := set != nil && set.Matches(value)
				require.Equal(t, want, got, "pattern %q value %q", test.pattern, value)
			}
		})
	}
}

func TestSignedPatternComplementIncludesAllUnicodeScalars(t *testing.T) {
	t.Parallel()

	ascii, err := stringlanguage.Pattern(`^[\x00-\x7f]$`)
	require.NoError(t, err)
	set, err := stringlanguage.Compile(
		[]stringlanguage.Requirement{{Language: ascii, WantMatch: false}},
		stringlanguage.Length{Min: 1, Max: new(1)},
	)
	require.NoError(t, err)

	for _, value := range []string{"\u0080", "é", "界", "😀"} {
		require.True(t, set.Matches(value), "%q", value)
	}

	for _, value := range []string{"a", "\x7f"} {
		require.False(t, set.Matches(value), "%q", value)
	}
}

func TestCombineIntersectsAndComplementsCompiledSetsExactly(t *testing.T) {
	t.Parallel()

	startsWithA, err := stringlanguage.Pattern(`^a`)
	require.NoError(t, err)
	first, err := stringlanguage.Compile(
		[]stringlanguage.Requirement{{Language: startsWithA, WantMatch: true}},
		stringlanguage.Length{Min: 1},
	)
	require.NoError(t, err)

	exactlyAB, err := stringlanguage.Pattern(`^ab$`)
	require.NoError(t, err)
	second, err := stringlanguage.Compile(
		[]stringlanguage.Requirement{{Language: exactlyAB, WantMatch: true}},
		stringlanguage.Length{},
	)
	require.NoError(t, err)

	combined, err := stringlanguage.Combine([]stringlanguage.SetRequirement{
		{Set: first, WantMatch: true},
		{Set: second, WantMatch: false},
	})
	require.NoError(t, err)
	require.True(t, combined.Matches("a"))
	require.True(t, combined.Matches("ac"))
	require.False(t, combined.Matches("ab"))
	require.False(t, combined.Matches("x"))

	_, err = stringlanguage.Combine([]stringlanguage.SetRequirement{
		{Set: second, WantMatch: true},
		{Set: second, WantMatch: false},
	})

	var empty *stringlanguage.EmptyError
	require.ErrorAs(t, err, &empty)
}

func TestAppendToDecodesCanonicalUnicodeScalars(t *testing.T) {
	t.Parallel()

	set, err := stringlanguage.Compile(nil, stringlanguage.Length{Min: 1, Max: new(1)})
	require.NoError(t, err)

	var builder program.Builder

	root, err := set.AppendTo(&builder)
	require.NoError(t, err)
	sealed, err := builder.Seal(root, builder.UniformSampling())
	require.NoError(t, err)

	limits := program.Limits{MaxSteps: 10, MaxOutputBytes: 10, MaxDepth: 1}
	tests := []struct {
		rank uint64
		want string
	}{
		{rank: 0, want: "\x00"},
		{rank: 0x80, want: "\u0080"},
		{rank: 0xf800, want: "𐀀"},
	}

	for _, test := range tests {
		tape := make([]byte, 8)
		binary.LittleEndian.PutUint64(tape, test.rank)
		value, decodeErr := sealed.Decode(tape, limits)
		require.NoError(t, decodeErr)
		require.True(t, value.Equal(jsonvalue.String(test.want)), "rank %#x: got %q", test.rank, value.String)
	}
}

func TestAppendToUsesMinimumScalarCompletionAndSupportsLongStrings(t *testing.T) {
	t.Parallel()

	ascii, err := stringlanguage.Pattern(`^[\x00-\x7f]$`)
	require.NoError(t, err)
	complement, err := stringlanguage.Compile(
		[]stringlanguage.Requirement{{Language: ascii, WantMatch: false}},
		stringlanguage.Length{Min: 1, Max: new(1)},
	)
	require.NoError(t, err)

	minimum := decodeSet(t, complement, nil, program.Limits{
		MaxSteps: 2, MaxOutputBytes: 4, MaxDepth: 1,
	})
	require.True(t, minimum.Equal(jsonvalue.String("\u0080")))

	long, err := stringlanguage.Compile(nil, stringlanguage.Length{Min: 300, Max: new(300)})
	require.NoError(t, err)
	value := decodeSet(t, long, nil, program.Limits{
		MaxSteps: 301, MaxOutputBytes: 2000, MaxDepth: 1,
	})
	require.Equal(t, 300, utf8.RuneCountInString(value.String))
	require.True(t, long.Matches(value.String))
}

func TestAppendToPreservesFormatLanguage(t *testing.T) {
	t.Parallel()

	language, err := stringlanguage.Format("uuid")
	require.NoError(t, err)
	set, err := stringlanguage.Compile(
		[]stringlanguage.Requirement{{Language: language, WantMatch: true}},
		stringlanguage.Length{},
	)
	require.NoError(t, err)

	value := decodeSet(t, set, nil, program.Limits{
		MaxSteps: 100, MaxOutputBytes: 100, MaxDepth: 1,
	})
	require.True(t, set.Matches(value.String))
}

func decodeSet(t *testing.T, set *stringlanguage.Set, tape []byte, limits program.Limits) jsonvalue.Value {
	t.Helper()

	var builder program.Builder

	root, err := set.AppendTo(&builder)
	require.NoError(t, err)
	sealed, err := builder.Seal(root, builder.UniformSampling())
	require.NoError(t, err)
	value, err := sealed.Decode(tape, limits)
	require.NoError(t, err)

	return value
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

func TestPatternComplementIncludesNonMatchingUnicode(t *testing.T) {
	t.Parallel()

	language, err := stringlanguage.Pattern("é")
	require.NoError(t, err)
	set, err := stringlanguage.Compile(
		[]stringlanguage.Requirement{{Language: language, WantMatch: false}},
		stringlanguage.Length{Max: new(2)},
	)
	require.NoError(t, err)
	require.False(t, set.Matches("é"))
	require.True(t, set.Matches("a"))
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
