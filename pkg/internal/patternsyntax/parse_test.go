//nolint:godoclint // Test names already state their contracts.
package patternsyntax

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAcceptsClosedGrammar(t *testing.T) {
	t.Parallel()

	patterns := []string{
		"", "|", "a|", "|a", "()", "(?:)", "(a|b)c", ".", "^a$", `\bword\B`,
		"a*", "a+?", "a??", "a{0}", "a{2,3}?", "a{2,}",
		`\f\n\r\t\v\0\x41\u0061\cA\ca`, `\/\-\#\,\.\$`,
		`\d\D\s\S\w\W`, "[]", "[^]", "[-a]", "[a-]", `[a-z]`, `[a-b-c]`, `[A-Za-z0-9-_.]+`,
		`[^\d\sA-Z]`, `[\b\x41\u0062\cC\cz]`, "[^^]",
		"^(?=a)(?!ab)a", "^(?=a|b)",
		"é", `\é`,
	}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			t.Parallel()

			tree, err := Parse(pattern)
			require.NoError(t, err)
			require.NotNil(t, tree)
			require.Less(t, int(tree.Root), len(tree.Nodes))
			require.Equal(t, Span{Start: 0, End: len(pattern)}, tree.Nodes[tree.Root].Span)
		})
	}
}

func TestParseClassifiesRejectionsAtOriginalByte(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		kind    ErrorKind
		offset  int
	}{
		{name: "dangling escape", pattern: `a\`, kind: ErrorInvalidSyntax, offset: 1},
		{name: "unmatched group", pattern: ")", kind: ErrorInvalidSyntax, offset: 0},
		{name: "bare closing bracket", pattern: "]", kind: ErrorInvalidSyntax, offset: 0},
		{name: "open group", pattern: "(a", kind: ErrorInvalidSyntax, offset: 0},
		{name: "reversed range", pattern: "[z-a]", kind: ErrorInvalidSyntax, offset: 2},
		{name: "set endpoint", pattern: `[\d-a]`, kind: ErrorInvalidSyntax, offset: 3},
		{name: "bad hex", pattern: `\x0g`, kind: ErrorInvalidSyntax, offset: 3},
		{name: "zero decimal", pattern: `\01`, kind: ErrorInvalidSyntax, offset: 2},
		{name: "unknown escape", pattern: `\a`, kind: ErrorInvalidSyntax, offset: 0},
		{name: "identifier escape", pattern: `\_`, kind: ErrorInvalidSyntax, offset: 0},
		{name: "backreference", pattern: `\1(a)`, kind: ErrorUnsupported, offset: 0},
		{name: "class decimal reference", pattern: `[\1](a)`, kind: ErrorUnsupported, offset: 1},
		{name: "out of range decimal", pattern: `\2(a)`, kind: ErrorInvalidSyntax, offset: 0},
		{name: "class decimal out of range", pattern: `[\2](a)`, kind: ErrorInvalidSyntax, offset: 1},
		{name: "lookahead placement", pattern: `x(?=a)`, kind: ErrorUnsupported, offset: 1},
		{name: "lookahead branch", pattern: `^(?=a)a|b`, kind: ErrorUnsupported, offset: 1},
		{name: "nested lookahead", pattern: `^(?=(?=a))`, kind: ErrorUnsupported, offset: 1},
		{name: "lookahead quantifier", pattern: `(?=a)*`, kind: ErrorInvalidSyntax, offset: 5},
		{name: "lookbehind", pattern: `(?<=a)`, kind: ErrorForeignSyntax, offset: 0},
		{name: "named group", pattern: `(?<name>a)`, kind: ErrorForeignSyntax, offset: 0},
		{name: "inline mode", pattern: `(?i:a)`, kind: ErrorForeignSyntax, offset: 0},
		{name: "atomic", pattern: `(?>a)`, kind: ErrorForeignSyntax, offset: 0},
		{name: "control verb", pattern: `(*SKIP)`, kind: ErrorForeignSyntax, offset: 0},
		{name: "unicode property", pattern: `\p{L}`, kind: ErrorForeignSyntax, offset: 0},
		{name: "code point", pattern: `\u{61}`, kind: ErrorForeignSyntax, offset: 0},
		{name: "named reference", pattern: `\k<name>`, kind: ErrorForeignSyntax, offset: 0},
		{name: "POSIX class", pattern: `[[:alpha:]]`, kind: ErrorForeignSyntax, offset: 1},
		{name: "set intersection", pattern: `[a&&b]`, kind: ErrorForeignSyntax, offset: 2},
		{name: "possessive", pattern: `a++`, kind: ErrorForeignSyntax, offset: 2},
		{name: "surrogate", pattern: `\ud800`, kind: ErrorUnsupported, offset: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(test.pattern)
			require.Error(t, err)

			var parseError *Error
			require.ErrorAs(t, err, &parseError)
			require.Equal(t, test.kind, parseError.Kind)
			require.Equal(t, test.offset, parseError.Offset)
		})
	}
}

func TestParseEnforcesEveryLimitBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		below   string
		atLimit string
		over    string
		limit   string
	}{
		{
			name:    "source",
			below:   "[" + strings.Repeat("a", MaximumSourceBytes-3) + "]",
			atLimit: "[" + strings.Repeat("a", MaximumSourceBytes-2) + "]",
			over:    "[" + strings.Repeat("a", MaximumSourceBytes-1) + "]",
			limit:   "source bytes",
		},
		{
			name: "depth",
			below: strings.Repeat("(", MaximumNestingDepth-1) + "a" +
				strings.Repeat(")", MaximumNestingDepth-1),
			atLimit: strings.Repeat("(", MaximumNestingDepth) + "a" +
				strings.Repeat(")", MaximumNestingDepth),
			over: strings.Repeat("(", MaximumNestingDepth+1) + "a" +
				strings.Repeat(")", MaximumNestingDepth+1),
			limit: "nesting depth",
		},
		{
			name: "nodes", below: strings.Repeat("a", MaximumNodes-3),
			atLimit: strings.Repeat("a", MaximumNodes-2),
			over:    strings.Repeat("a", MaximumNodes-1), limit: "AST nodes",
		},
		{
			name:    "leading assertions",
			below:   "^" + strings.Repeat("(?=a)", MaximumLeadingAssertions-1),
			atLimit: "^" + strings.Repeat("(?=a)", MaximumLeadingAssertions),
			over:    "^" + strings.Repeat("(?=a)", MaximumLeadingAssertions+1),
			limit:   "leading assertions",
		},
		{
			name: "repeat endpoint", below: "a{999}", atLimit: "a{1000}", over: "a{1001}",
			limit: "repeat endpoint",
		},
		{
			name: "nested repeat product", below: "(a{10}){99}",
			atLimit: "(a{10}){100}", over: "(a{10}){101}",
			limit: "cumulative nested repeat product",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(test.below)
			require.NoError(t, err)

			_, err = Parse(test.atLimit)
			require.NoError(t, err)

			_, err = Parse(test.over)
			require.ErrorIs(t, err, ErrTooComplex)

			var complexity *Error
			require.True(t, errors.As(err, &complexity))
			require.Equal(t, ErrorTooComplex, complexity.Kind)
			require.Equal(t, test.limit, complexity.Limit)
			require.Greater(t, complexity.Observed, complexity.Maximum)
		})
	}
}

func TestParseNamedRobustnessSources(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		source string
	}{
		{name: "empty", source: ""},
		{name: "literal", source: "a"},
		{name: "open class", source: "["},
		{name: "trailing slash", source: `\`},
		{name: "lookahead", source: "^(?=a)a"},
		{name: "non-ASCII", source: "é"},
		{name: "invalid UTF-8", source: string([]byte{0xff})},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			tree, err := Parse(test.source)
			if err == nil {
				require.NotNil(t, tree)
			}
		})
	}
}

func TestParseCoversAcceptedEscapeAndRepeatForms(t *testing.T) {
	t.Parallel()

	for _, pattern := range []string{
		`[\D\S\w\W\f\n\r\t\v\0\x41\u0042\cA\ca\é\-]`,
		`(a{2,}){2}`,
		`\xAF`,
	} {
		tree, err := Parse(pattern)
		require.NoError(t, err, pattern)
		require.NotNil(t, tree)
	}
}

func TestParseRejectsEveryMalformedGrammarForm(t *testing.T) {
	t.Parallel()

	for _, pattern := range []string{
		"*", "+", "?", "{", "}", "(?", "(?#a)", "(?()",
		`\c`, `\c1`, `\k`, `\A`,
		`[a--b]`, `[a-\`, `[\01]`, `[\x]`, `[\u]`, `[\u{41}]`, `[\u12]`,
		`[\c`, `[\c]`, `[\c1]`, `[\p]`, `[\A]`, `[\a]`, `[\ud800]`,
		`\999999999999999999999999(a)`,
		"a{", "a{,}", "a{2,x}", "a{2,1}", "a{2", "a**", "a*?*", "a{999999999999999999999999}",
		"(?=a)",
	} {
		_, err := Parse(pattern)
		require.Error(t, err, pattern)
	}
}

func TestPatternSyntaxErrorsAndByteHelpers(t *testing.T) {
	t.Parallel()

	ordinary := &Error{Kind: ErrorInvalidSyntax, Offset: 2, Message: "bad"}
	require.Contains(t, ordinary.Error(), "byte 2")

	complexity := &Error{Kind: ErrorTooComplex, Offset: 1, Limit: "nodes", Maximum: 2, Observed: 3}
	require.Contains(t, complexity.Error(), "maximum 2")

	require.Equal(t, 1, firstInvalidUTF8("a"+string([]byte{0xff})))
	require.Zero(t, firstInvalidUTF8("é"))

	value, ok := hexValue('F')
	require.True(t, ok)
	require.Equal(t, byte(15), value)
}
