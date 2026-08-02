//nolint:godoclint // Test names already state their contracts.
package patternvalidator

import (
	"encoding/json"
	"errors"
	"os"
	regexpsyntax "regexp/syntax"
	"strings"
	"sync"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/patternsyntax"

	"github.com/stretchr/testify/require"
)

func TestValidateECMAScript51Semantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{name: "empty", pattern: "", value: "anything", want: true},
		{name: "unanchored", pattern: "a", value: "cat", want: true},
		{name: "unanchored miss", pattern: "a", value: "BCD", want: false},
		{name: "anchored", pattern: "^a$", value: "a", want: true},
		{name: "start", pattern: "^a", value: "ab", want: true},
		{name: "start miss", pattern: "^a", value: "ba", want: false},
		{name: "end", pattern: "a$", value: "ba", want: true},
		{name: "end before newline is false", pattern: "a$", value: "a\n", want: false},
		{name: "alternation", pattern: "^(a|b)c$", value: "bc", want: true},
		{name: "empty branch", pattern: "^(?:a|)$", value: "", want: true},
		{name: "counted lazy", pattern: "^a{2,3}?$", value: "aaa", want: true},
		{name: "dot ordinary", pattern: "^.$", value: "x", want: true},
		{name: "dot carriage return", pattern: "^.$", value: "\r", want: false},
		{name: "dot line feed", pattern: "^.$", value: "\n", want: false},
		{name: "dot line separator", pattern: "^.$", value: "\u2028", want: false},
		{name: "dot paragraph separator", pattern: "^.$", value: "\u2029", want: false},
		{name: "dot counts UTF-16 code units", pattern: "^.$", value: "😀", want: false},
		{name: "two dots count UTF-16 code units", pattern: "^..$", value: "😀", want: true},
		{name: "counted dots count UTF-16 code units", pattern: "^.{2}$", value: "😀", want: true},
		{name: "space tab", pattern: `^\s$`, value: "\t", want: true},
		{name: "space Mongolian vowel separator", pattern: `^\s$`, value: "\u180e", want: true},
		{name: "space zero width", pattern: `^\s$`, value: "\u200b", want: true},
		{name: "space excludes modern addition", pattern: `^\s$`, value: "\u205f", want: false},
		{name: "not space", pattern: `^\S$`, value: "x", want: true},
		{name: "digit", pattern: `^\d$`, value: "7", want: true},
		{name: "digit ASCII only", pattern: `^\d$`, value: "٧", want: false},
		{name: "word ASCII only", pattern: `^\w$`, value: "_", want: true},
		{name: "not word", pattern: `^\W$`, value: "-", want: true},
		{name: "word boundary", pattern: `\bcat\b`, value: "cat!", want: true},
		{name: "word boundary miss", pattern: `\bcat\b`, value: "scatter", want: false},
		{name: "not boundary", pattern: `a\Bb`, value: "ab", want: true},
		{name: "class range", pattern: "^[a-c]+$", value: "cab", want: true},
		{name: "class literal hyphen after ranges", pattern: "^[A-Za-z0-9-_.]+$", value: "A-_.9", want: true},
		{name: "astral class literal", pattern: "[😀]", value: "😀", want: true},
		{name: "astral class literal consumes one unit", pattern: "^[😀]$", value: "😀", want: false},
		{name: "astral class literals consume both units", pattern: "^[😀][😀]$", value: "😀", want: true},
		{name: "negated astral class literal", pattern: "[^😀]", value: "😀", want: false},
		{name: "astral quantifier applies to low surrogate", pattern: "^😀{2}$", value: "😀😀", want: false},
		{name: "astral optional keeps high surrogate required", pattern: "^😀?$", value: "", want: false},
		{name: "astral optional accepts one scalar", pattern: "^😀?$", value: "😀", want: true},
		{name: "grouped astral repeat applies to scalar pair", pattern: "^(?:😀){2}$", value: "😀😀", want: true},
		{name: "class backspace", pattern: `^[\b]$`, value: "\b", want: true},
		{name: "class union complement", pattern: `^[\D_]$`, value: "_", want: true},
		{name: "class union complement non-digit", pattern: `^[\D_]$`, value: "x", want: true},
		{name: "class union complement digit", pattern: `^[\D_]$`, value: "7", want: false},
		{name: "empty class", pattern: "[]", value: "x", want: false},
		{name: "universal class newline", pattern: "^[^]$", value: "\n", want: true},
		{name: "universal class nul", pattern: "^[^]$", value: "\x00", want: true},
		{name: "hex escapes", pattern: `^\x41\u0062\cC\0$`, value: "Ab\x03\x00", want: true},
		{name: "lowercase control escape", pattern: `^\ca$`, value: "\x01", want: true},
		{name: "lowercase class control escape", pattern: `^[\cz]$`, value: "\x1a", want: true},
		{name: "identity escapes", pattern: `^\/\-\#\,$`, value: "/-# ,", want: false},
		{name: "identity escapes exact", pattern: `^\/\-\#\,$`, value: "/-#,", want: true},
		{name: "positive lookahead", pattern: `^(?=a)a`, value: "ab", want: true},
		{name: "positive lookahead miss", pattern: `^(?=a)a`, value: "ba", want: false},
		{name: "negative lookahead", pattern: `^(?!ab)a`, value: "ac", want: true},
		{name: "negative lookahead miss", pattern: `^(?!ab)a`, value: "ab", want: false},
		{name: "multiple lookaheads", pattern: `^(?=a)(?!ab)a`, value: "ax", want: true},
		{name: "lookahead only accepts empty", pattern: `^(?!a)`, value: "", want: true},
		{name: "lookahead only rejects", pattern: `^(?!a)`, value: "a", want: false},
		{name: "capture not exposed", pattern: `^(a)+$`, value: "aaa", want: true},
		{name: "non-ASCII best effort literal", pattern: "^é$", value: "é", want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			validation, err := Parse(test.pattern)
			require.NoError(t, err)
			require.Equal(t, test.want, validation.Validate(test.value))
		})
	}
}

func TestValidateMatchesNodeAndBunCommonSubsetFixture(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("testdata/common_subset.json")
	require.NoError(t, err)

	var fixture struct {
		Cases []struct {
			Pattern  string `json:"pattern"`
			Input    string `json:"input"`
			Expected bool   `json:"expected"`
		} `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(contents, &fixture))
	require.NotEmpty(t, fixture.Cases)

	for _, test := range fixture.Cases {
		validation, parseErr := Parse(test.Pattern)
		require.NoError(t, parseErr, "pattern %q", test.Pattern)
		require.Equal(t, test.Expected, validation.Validate(test.Input), "pattern %q", test.Pattern)
	}
}

func TestUseRE2RetainsRawGoSemantics(t *testing.T) {
	t.Parallel()

	validation, err := Parse(`(?i)^é$`, UseRE2)
	require.NoError(t, err)
	require.True(t, validation.UsesRE2())
	require.True(t, validation.Validate("É"))
	require.False(t, validation.RejectsNonASCII())

	_, err = Parse(`(?i)^é$`)
	assertParseError(t, err, ParseErrorForeignSyntax, 0)

	_, err = Parse(`(?=a)`, UseRE2)
	assertParseError(t, err, ParseErrorRawGoSyntax, 0)
}

func TestRejectNonASCIIAppliesToPatternValuesAndSubjects(t *testing.T) {
	t.Parallel()

	for _, pattern := range []string{"é", `\u00e9`, `[a-\u00e9]`} {
		_, err := Parse(pattern, RejectNonASCII)
		assertParseError(t, err, ParseErrorPolicy, 0)
	}

	validation, err := Parse(`^[a-z]+$`, RejectNonASCII)
	require.NoError(t, err)
	require.True(t, validation.RejectsNonASCII())
	require.True(t, validation.Validate("ascii"))
	require.False(t, validation.Validate("é"))
	require.False(t, validation.Validate(string([]byte{0xff})))

	raw, err := Parse(`^.$`, RejectNonASCII, UseRE2)
	require.NoError(t, err)
	require.False(t, raw.Validate("é"))

	for _, test := range []struct {
		pattern string
		offset  int
	}{
		{pattern: `\x{e9}`, offset: 0},
		{pattern: `[a-\x{e9}]`, offset: 3},
		{pattern: `\303`, offset: 0},
	} {
		_, err = Parse(test.pattern, RejectNonASCII, UseRE2)
		assertParseError(t, err, ParseErrorPolicy, test.offset)
	}

	for _, pattern := range []string{`\Q\x{e9}\E`, `\\x{e9}`} {
		_, err = Parse(pattern, RejectNonASCII, UseRE2)
		require.NoError(t, err)
	}
}

func TestParseClassifiesPublicErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		options []Option
		kind    ParseErrorKind
		offset  int
	}{
		{name: "invalid", pattern: `[z-a]`, kind: ParseErrorInvalidSyntax, offset: 2},
		{name: "unsupported", pattern: `\1(a)`, kind: ParseErrorUnsupported, offset: 0},
		{name: "foreign", pattern: `(?<=a)`, kind: ParseErrorForeignSyntax, offset: 0},
		{name: "policy", pattern: `aé`, options: []Option{RejectNonASCII}, kind: ParseErrorPolicy, offset: 1},
		{name: "raw", pattern: `prefix[`, options: []Option{UseRE2}, kind: ParseErrorRawGoSyntax, offset: 6},
		{name: "invalid utf8", pattern: string([]byte{'a', 0xff}), kind: ParseErrorInvalidSyntax, offset: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(test.pattern, test.options...)
			assertParseError(t, err, test.kind, test.offset)
		})
	}
}

func TestParseAndMustParseOptionsAndSealing(t *testing.T) {
	t.Parallel()

	validation, err := Parse("a", RejectNonASCII, RejectNonASCII, UseRE2, UseRE2)
	require.NoError(t, err)
	require.True(t, validation.RejectsNonASCII())
	require.True(t, validation.UsesRE2())
	require.True(t, validation.Validate("a"))

	must := MustParse("a", RejectNonASCII, UseRE2)
	require.Equal(t, validation.Validate("a"), must.Validate("a"))

	_, parseErr := Parse("[")
	require.Error(t, parseErr)
	require.PanicsWithError(t, parseErr.Error(), func() { MustParse("[") })

	defaultValidation := MustParse("a")

	require.Error(t, RejectNonASCII(defaultValidation))
	require.False(t, defaultValidation.RejectsNonASCII())
	require.Error(t, UseRE2(defaultValidation))
	require.False(t, defaultValidation.UsesRE2())
	require.Error(t, RejectNonASCII(nil))
	require.Error(t, UseRE2(nil))

	_, err = Parse("a", nil)
	require.Error(t, err)

	customErr := errors.New("custom option failed")
	_, err = Parse("a", func(*PatternValidation) error { return customErr })
	require.ErrorIs(t, err, customErr)
}

func TestZeroAndNilValidationFailClosed(t *testing.T) {
	t.Parallel()

	var nilValidation *PatternValidation
	require.False(t, nilValidation.Validate(""))
	require.False(t, nilValidation.RejectsNonASCII())
	require.False(t, nilValidation.UsesRE2())

	zero := new(PatternValidation)
	require.False(t, zero.Validate(""))
}

func TestCompiledValidationIsSafeForConcurrentReads(t *testing.T) {
	t.Parallel()

	validation := MustParse(`^(?=a)(?!ab)a`)

	const (
		workers    = 32
		iterations = 100
	)

	type result struct {
		value string
		want  bool
		got   bool
	}

	results := make(chan result, workers*iterations*2)

	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()

			for range iterations {
				results <- result{value: "ac", want: true, got: validation.Validate("ac")}

				results <- result{value: "ab", want: false, got: validation.Validate("ab")}
			}
		}()
	}

	wait.Wait()
	close(results)

	for result := range results {
		require.Equal(t, result.want, result.got, "value %q", result.value)
	}
}

func TestParseReportsComplexity(t *testing.T) {
	t.Parallel()

	_, err := Parse(strings.Repeat(`\S`, 9_998))
	require.ErrorIs(t, err, ErrTooComplex)

	var complexity *ComplexityError
	require.True(t, errors.As(err, &complexity))
	require.Equal(t, "translation", complexity.Phase)
	require.Equal(t, "generated Go regexp bytes", complexity.Limit)
	require.Greater(t, complexity.Observed, complexity.Maximum)

	_, err = Parse(strings.Repeat("a", 64*1024+1), UseRE2)
	require.ErrorIs(t, err, ErrTooComplex)
	require.True(t, errors.As(err, &complexity))
	require.Equal(t, "input", complexity.Phase)
}

func TestGeneratedRegexpLimitBoundaries(t *testing.T) {
	t.Parallel()

	for _, length := range []int{maximumGeneratedRegexpBytes - 1, maximumGeneratedRegexpBytes} {
		translation := new(translator)
		translation.write(strings.Repeat("a", length), patternsyntax.Span{})
		require.NoError(t, translation.failure)
		require.Len(t, translation.output, length)
	}

	translation := new(translator)
	translation.write(strings.Repeat("a", maximumGeneratedRegexpBytes+1), patternsyntax.Span{})
	require.ErrorIs(t, translation.failure, ErrTooComplex)
	require.Empty(t, translation.output)
}

func TestParseAndValidateNamedRobustnessMatrix(t *testing.T) {
	t.Parallel()

	sources := []struct {
		name  string
		value string
	}{
		{name: "empty", value: ""},
		{name: "literal", value: "a"},
		{name: "open class", value: "["},
		{name: "trailing slash", value: `\`},
		{name: "lookahead", value: "^(?=a)a"},
		{name: "non-ASCII", value: "é"},
		{name: "invalid UTF-8", value: string([]byte{0xff})},
	}
	values := []struct {
		name  string
		value string
	}{
		{name: "empty", value: ""},
		{name: "ASCII", value: "ascii"},
		{name: "non-ASCII", value: "é"},
		{name: "invalid UTF-8", value: string([]byte{0xff})},
	}
	optionSets := []struct {
		name    string
		options []Option
	}{
		{name: "default"},
		{name: "ASCII", options: []Option{RejectNonASCII}},
		{name: "RE2", options: []Option{UseRE2}},
		{name: "ASCII and RE2", options: []Option{RejectNonASCII, UseRE2}},
	}

	for _, source := range sources {
		for _, value := range values {
			for _, optionSet := range optionSets {
				t.Run(source.name+"/"+value.name+"/"+optionSet.name, func(t *testing.T) {
					t.Parallel()

					validation, err := Parse(source.value, optionSet.options...)
					if err == nil {
						require.NotNil(t, validation)
						_ = validation.Validate(value.value)
					}
				})
			}
		}
	}
}

func TestErrorFormattingAndCompilationFailures(t *testing.T) {
	t.Parallel()

	cause := errors.New("cause")
	parseError := &ParseError{Kind: ParseErrorPolicy, Offset: 2, Cause: cause}
	require.Contains(t, parseError.Error(), "byte 2")
	require.ErrorIs(t, parseError, cause)

	for kind := ParseErrorInvalidSyntax; kind <= ParseErrorInternalTranslation+1; kind++ {
		require.NotEmpty(t, kind.String())
	}

	complexity := &ComplexityError{Phase: "parse", Limit: "nodes", Maximum: 1, Observed: 2}
	require.Contains(t, complexity.Error(), "maximum 1")

	validation, err := compiledPatternValidation(
		new(PatternValidation),
		[]checkSpecification{{source: "[", span: patternsyntax.Span{Start: 3}}},
	)
	require.Error(t, err)
	require.Nil(t, validation)
}

func TestTranslationBranchesAndMalformedTrees(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		pattern string
		value   string
	}{
		{pattern: `^\D$`, value: "x"},
		{pattern: `^[\d\s\S\w\W]$`, value: "_"},
		{pattern: `^a*$`, value: "aaa"},
	} {
		validation, err := Parse(test.pattern)
		require.NoError(t, err)
		require.True(t, validation.Validate(test.value))
	}

	lookahead := patternsyntax.Tree{Root: 0, Nodes: []patternsyntax.Node{{Kind: patternsyntax.KindPositiveLookahead}}}
	_, err := translateNode(&lookahead, lookahead.Root)
	require.Error(t, err)
	_, err = translateSequence(&lookahead, []patternsyntax.NodeID{lookahead.Root})
	require.Error(t, err)

	malformedPrefix := leadingLookaheadTree(lookahead.Nodes, []patternsyntax.NodeID{0})
	_, err = translateLeadingLookaheads(&malformedPrefix, malformedPrefix.Nodes[1])
	require.Error(t, err)

	malformedRemainder := leadingLookaheadTree(
		[]patternsyntax.Node{
			{Kind: patternsyntax.KindAlternative},
			{Kind: patternsyntax.KindPositiveLookahead},
		},
		[]patternsyntax.NodeID{0, 1},
	)
	_, err = translateLeadingLookaheads(&malformedRemainder, malformedRemainder.Nodes[1])
	require.Error(t, err)

	normalized := normalizeRuneSet([]runeRange{{low: 2, high: 3}, {low: 2, high: 4}})
	require.Equal(t, []runeRange{{low: 2, high: 4}}, normalized)
	require.Empty(t, complementRuneSet([]runeRange{{low: 0, high: 0x10ffff}}))
}

func TestRawEscapeAndErrorOffsetHelpers(t *testing.T) {
	t.Parallel()

	require.Zero(t, rawGoErrorOffset("x", errors.New("ordinary")))
	require.Zero(t, rawGoErrorOffset("source", &regexpsyntax.Error{Expr: "missing"}))

	for _, source := range []string{`\Qquoted`, `\xAF`, `\x{Af}`, `\377`, `\a`} {
		_, _, _ = rawEscapeValue(source, 0)
	}

	require.Equal(t, rune(0xaf), hexadecimalValue("aF"))
	require.Zero(t, firstInvalidUTF8("valid"))

	internal := publicSyntaxError(errors.New("internal"))

	var parseError *ParseError
	require.ErrorAs(t, internal, &parseError)
	require.Equal(t, ParseErrorInternalTranslation, parseError.Kind)

	complexity := publicSyntaxError(&patternsyntax.Error{
		Kind: patternsyntax.ErrorTooComplex, Limit: "nodes", Maximum: 1, Observed: 2,
	})
	require.ErrorIs(t, complexity, ErrTooComplex)
}

func leadingLookaheadTree(
	children []patternsyntax.Node,
	remainder []patternsyntax.NodeID,
) patternsyntax.Tree {
	nodes := []patternsyntax.Node{
		{Kind: patternsyntax.KindExpression, Children: []patternsyntax.NodeID{1}},
		{Kind: patternsyntax.KindAlternative},
		{Kind: patternsyntax.KindBeginInput},
		{Kind: patternsyntax.KindPositiveLookahead, Children: []patternsyntax.NodeID{4}},
	}
	offset := patternsyntax.NodeID(len(nodes))

	shifted := make([]patternsyntax.Node, len(children))
	for index := range children {
		shifted[index] = children[index]

		shifted[index].Children = append([]patternsyntax.NodeID(nil), children[index].Children...)
		for childIndex := range shifted[index].Children {
			shifted[index].Children[childIndex] += offset
		}
	}

	nodes = append(nodes, shifted...)

	nodes[1].Children = []patternsyntax.NodeID{2, 3}
	for _, child := range remainder {
		nodes[1].Children = append(nodes[1].Children, child+offset)
	}

	return patternsyntax.Tree{Root: 0, Nodes: nodes}
}

func assertParseError(t *testing.T, err error, kind ParseErrorKind, offset int) {
	t.Helper()
	require.Error(t, err)

	var parseError *ParseError
	require.ErrorAs(t, err, &parseError)
	require.Equal(t, kind, parseError.Kind)
	require.Equal(t, offset, parseError.Offset)
}
