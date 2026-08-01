//nolint:godoclint,lll // Internal white-box coverage matrices use compact malformed-state literals.
package stringlanguage

import (
	"errors"
	regexpsyntax "regexp/syntax"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/patternsyntax"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/stretchr/testify/require"
)

func TestPatternCompilationBranches(t *testing.T) {
	t.Parallel()

	for _, source := range []string{
		"", "a|b", ".", `\d`, `\D`, `\s`, `\S`, `\w`, `\W`, `^[\d\D\s\S\w\W]$`, `\bword\B`,
		`(?:a)?b*c+d{2}e{1,2}`, "^😀$", "😀*", "😀{2}", "😀{1,2}", "^(?:😀){2}$", "^[😀]$",
	} {
		language, err := Pattern(source)
		require.NoError(t, err, source)
		_, err = language.Matches("😀word_abcdde")
		require.NoError(t, err)
	}

	for _, source := range []string{
		`(?i:a)`, `[a-z]`, `.`, `(?s:.)`, `(?m:^a$)`, `\Aa\z`, `\bword\B`,
		`a*`, `a+`, `a?`, `a{1,2}`, `(a|b)c`,
	} {
		language, err := Pattern(source, patternvalidator.UseRE2)
		require.NoError(t, err, source)
		_, err = language.Matches("A\nwordabc")
		require.NoError(t, err)
	}

	customError := errors.New("option")

	for _, options := range [][]patternvalidator.Option{
		{nil},
		{func(*patternvalidator.PatternValidation) error { return customError }},
	} {
		_, err := Pattern("a", options...)
		require.Error(t, err)
	}

	for _, test := range []struct {
		source  string
		options []patternvalidator.Option
	}{
		{source: string([]byte{0xff})},
		{source: "é", options: []patternvalidator.Option{patternvalidator.RejectNonASCII}},
		{source: `\u00e9`, options: []patternvalidator.Option{patternvalidator.RejectNonASCII}},
		{source: "[", options: []patternvalidator.Option{patternvalidator.UseRE2}},
		{source: "["},
		{source: "^(?=a)a"},
	} {
		_, err := Pattern(test.source, test.options...)
		require.Error(t, err)
	}

	_, err := Format("unknown")
	require.Error(t, err)
	_, err = formatPattern("[")
	require.Error(t, err)
	_, err = formatDFA("[")
	require.Error(t, err)
}

func TestMalformedAutomataReturnErrors(t *testing.T) {
	t.Parallel()

	full := dfaEdge{first: 0, last: maximumScalar, target: 0}

	malformed := []dfa{
		{},
		{states: []dfaState{{edges: []dfaEdge{{first: 1, last: 0}}}}},
		{states: []dfaState{{edges: []dfaEdge{{first: 0, last: maximumScalar, target: 1}}}}},
		{states: []dfaState{{edges: []dfaEdge{{first: 0, last: 2}, {first: 2, last: maximumScalar}}}}},
		{states: []dfaState{{edges: []dfaEdge{{first: 1, last: maximumScalar}}}}},
		{states: []dfaState{{edges: []dfaEdge{{first: 0, last: maximumScalar - 1}}}}},
		{states: []dfaState{{edges: []dfaEdge{{first: 0, last: maximumScalar + 1}}}}},
	}
	for _, machine := range malformed {
		require.Error(t, validateDFA(machine))
	}

	valid := dfa{states: []dfaState{{edges: []dfaEdge{full}, accepting: true}}}
	_, err := valid.advance(0, 'a')
	require.NoError(t, err)
	_, err = advanceDFAState(nil, 0, 'a')
	require.Error(t, err)
	_, err = advanceDFAState(&valid, 1, 'a')
	require.Error(t, err)

	missing := dfa{states: []dfaState{{edges: []dfaEdge{{first: 0, last: 1}}}}}
	_, err = advanceDFAState(&missing, 0, 2)
	require.Error(t, err)
	_, err = matchDFA(&missing, string(rune(2)))
	require.Error(t, err)

	target := dfa{states: []dfaState{{edges: []dfaEdge{{first: 0, last: maximumScalar, target: 1}}}}}
	_, err = advanceDFAState(&target, 0, 'a')
	require.Error(t, err)

	_, err = advanceScalarChecked(nil, 0, 'a')
	require.Error(t, err)
	_, err = advanceScalarChecked(&valid, 1, 'a')
	require.Error(t, err)

	utfMachine := dfa{utf16: true, states: []dfaState{{edges: []dfaEdge{{first: 0, last: maximumCodeUnit}}}}}
	_, err = advanceScalarChecked(&utfMachine, 0, '😀')
	require.NoError(t, err)
	_, err = utfMachine.scalarEdges(1)
	require.Error(t, err)

	incompleteUTF := dfa{utf16: true, states: []dfaState{{edges: []dfaEdge{{first: 0, last: 1}}}}}
	_, err = incompleteUTF.scalarEdges(0)
	require.Error(t, err)
	_, err = advanceScalarChecked(&incompleteUTF, 0, '😀')
	require.Error(t, err)

	_, err = minimizeDFA(nil)
	require.Error(t, err)
}

func TestDFAProjectionAndClosureBranches(t *testing.T) {
	t.Parallel()

	language, err := Pattern("^😀$")
	require.NoError(t, err)
	edges, err := language.dfa.scalarEdges(0)
	require.NoError(t, err)
	require.NotEmpty(t, edges)

	raw, err := Pattern("a", patternvalidator.UseRE2)
	require.NoError(t, err)
	edges, err = raw.dfa.scalarEdges(0)
	require.NoError(t, err)
	require.NotEmpty(t, edges)
	require.Equal(t, rune(0x10000), utf16Scalar(0xd800, 0xdc00))

	nextNewline := rune('\n')

	nextWord := rune('a')
	for _, test := range []struct {
		kind         edgeKind
		atStart      bool
		previousWord bool
		previousLF   bool
		next         *rune
	}{
		{kind: edgeEpsilon},
		{kind: edgeBeginText, atStart: true},
		{kind: edgeEndText, next: nil},
		{kind: edgeBeginLine, previousLF: true, next: &nextWord},
		{kind: edgeEndLine, next: &nextNewline},
		{kind: edgeWordBoundary, next: &nextWord},
		{kind: edgeNotWordBoundary, previousWord: true, next: &nextWord},
		{kind: edgeCharacters},
		{kind: edgeKind(255)},
	} {
		_ = assertionEnabled(test.kind, test.atStart, test.previousWord, test.previousLF, test.next)
	}
}

func TestNFABuilderAndMalformedASTBranches(t *testing.T) {
	t.Parallel()

	builder := newNFABuilder(false)
	failure := errors.New("build")
	_, err := builder.repeat(func() (fragment, error) { return fragment{}, failure }, 1, 1, false)
	require.ErrorIs(t, err, failure)
	_, err = builder.repeat(func() (fragment, error) { return fragment{}, failure }, 0, 0, true)
	require.ErrorIs(t, err, failure)

	calls := 0
	_, err = builder.repeat(func() (fragment, error) {
		calls++
		if calls == 2 {
			return fragment{}, failure
		}

		return builder.empty(), nil
	}, 0, 2, false)
	require.ErrorIs(t, err, failure)

	for _, expression := range []*regexpsyntax.Regexp{
		{Op: regexpsyntax.OpNoMatch},
		{Op: regexpsyntax.OpEmptyMatch},
		{Op: regexpsyntax.OpLiteral, Rune: []rune{'a'}, Flags: regexpsyntax.FoldCase},
		{Op: regexpsyntax.OpCharClass, Rune: []rune{'a', 'z'}},
		{Op: regexpsyntax.OpAnyCharNotNL},
		{Op: regexpsyntax.OpAnyChar},
		{Op: regexpsyntax.OpBeginLine},
		{Op: regexpsyntax.OpEndLine},
		{Op: regexpsyntax.OpBeginText},
		{Op: regexpsyntax.OpEndText},
		{Op: regexpsyntax.OpWordBoundary},
		{Op: regexpsyntax.OpNoWordBoundary},
		{Op: regexpsyntax.Op(255)},
	} {
		_, err = builder.buildRawNode(expression)
		if expression.Op == regexpsyntax.Op(255) {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}

	tree := &patternsyntax.Tree{Root: 0, Nodes: []patternsyntax.Node{{Kind: patternsyntax.KindPositiveLookahead}}}
	_, err = builder.buildESNode(tree, tree.Root)
	require.Error(t, err)

	tree.Nodes[0].Kind = patternsyntax.Kind(255)
	_, err = builder.buildESNode(tree, tree.Root)
	require.Error(t, err)

	_, err = compileRawPattern(&regexpsyntax.Regexp{Op: regexpsyntax.Op(255)})
	require.Error(t, err)

	invalidExpression := &regexpsyntax.Regexp{Op: regexpsyntax.Op(255)}
	_, err = builder.buildRawNode(&regexpsyntax.Regexp{
		Op:  regexpsyntax.OpConcat,
		Sub: []*regexpsyntax.Regexp{invalidExpression},
	})
	require.Error(t, err)
	_, err = compileFormatExpression(invalidExpression)
	require.Error(t, err)
	_, err = compileRawExpression(invalidExpression)
	require.Error(t, err)
	builder.alternate(nil)

	parsed, err := patternsyntax.Parse("a")
	require.NoError(t, err)
	_, err = compileESLeaf(parsed, []patternsyntax.NodeID{parsed.Root}, true)
	require.NoError(t, err)

	malformedTree := &patternsyntax.Tree{Root: 0, Nodes: []patternsyntax.Node{{Kind: patternsyntax.Kind(255)}}}
	_, err = compileESLeaf(malformedTree, []patternsyntax.NodeID{0}, false)
	require.Error(t, err)
	_, err = builder.buildESChildren(malformedTree, []patternsyntax.NodeID{0}, false)
	require.Error(t, err)

	invalidNFA := &nfa{states: []nfaState{{}}, start: 0, accept: 0}
	_, err = determinize(invalidNFA)
	require.Error(t, err)
}

func TestRangeAndEmailHelpers(t *testing.T) {
	t.Parallel()

	require.Nil(t, normalizeRuneSet(nil))
	set := normalizeRuneSet(runeSet{{first: 3, last: 4}, {first: 1, last: 2}, {first: 1, last: 5}})
	require.Equal(t, runeSet{{first: 1, last: 5}}, set)
	require.Equal(t, runeSet{{first: 1, last: 3}}, unionRuneSets(
		runeSet{{first: 1, last: 2}}, runeSet{{first: 3, last: 3}},
	))
	_ = complementRuneSet(runeSet{{first: 2, last: 4}}, runeSet{{first: 1, last: 4}})
	_ = intersectRuneSets(
		runeSet{{first: 1, last: 2}, {first: 5, last: 8}},
		runeSet{{first: 2, last: 6}},
	)

	for _, node := range []patternsyntax.Node{
		{ClassItems: []patternsyntax.ClassItem{{Kind: patternsyntax.ClassItemDigit}}},
		{ClassItems: []patternsyntax.ClassItem{{Kind: patternsyntax.ClassItemNotDigit}}},
		{ClassItems: []patternsyntax.ClassItem{{Kind: patternsyntax.ClassItemSpace}}},
		{ClassItems: []patternsyntax.ClassItem{{Kind: patternsyntax.ClassItemNotSpace}}},
		{ClassItems: []patternsyntax.ClassItem{{Kind: patternsyntax.ClassItemWord}}},
		{ClassItems: []patternsyntax.ClassItem{{Kind: patternsyntax.ClassItemNotWord}}},
		{Negated: true},
	} {
		_ = esClassSet(node)
	}

	_ = appendESClassRange(nil, '😀', '😀')
	_ = appendESClassRange(nil, maximumCodeUnit+1, maximumCodeUnit+2)
	_ = appendESClassRange(nil, 'a', maximumCodeUnit+2)

	require.Empty(t, repeatedColonGroups("x", 0))
	require.NotEmpty(t, compressedIPv6Patterns("x", 1, "suffix"))
	require.NotEmpty(t, charactersExcept("abc", 'a'))

	state := emailLengthState{over: true}
	require.Equal(t, state, advanceEmailLength(state, 'a'))

	_, err := finishEmailLanguage(nil, errors.New("initial"))
	require.Error(t, err)
	_, err = finishEmailLanguage(&dfa{}, nil)
	require.Error(t, err)
	_, err = limitEmailPartLengths(&dfa{
		utf16:  true,
		states: []dfaState{{edges: []dfaEdge{{first: 0, last: 1}}}},
	})
	require.Error(t, err)
	_, err = limitEmailPartLengths(&dfa{
		states: []dfaState{{edges: []dfaEdge{{first: 0, last: 1}}}},
	})
	require.Error(t, err)

	require.Error(t, validateRawCapabilities(&regexpsyntax.Regexp{Op: regexpsyntax.Op(255)}))
	require.Error(t, validateRawCapabilities(&regexpsyntax.Regexp{
		Op:  regexpsyntax.OpConcat,
		Sub: []*regexpsyntax.Regexp{{Op: regexpsyntax.Op(255)}},
	}))
	require.False(t, hasNonASCIIExpression(&patternsyntax.Tree{Nodes: []patternsyntax.Node{{Kind: patternsyntax.KindLiteral, Value: 'a'}}}))
	require.True(t, hasNonASCIIExpression(&patternsyntax.Tree{Nodes: []patternsyntax.Node{{Kind: patternsyntax.KindLiteral, Value: 'é'}}}))
	require.False(t, hasNonASCIIExpression(&patternsyntax.Tree{Nodes: []patternsyntax.Node{{Kind: patternsyntax.KindClass}}}))
	require.True(t, hasNonASCIIExpression(&patternsyntax.Tree{Nodes: []patternsyntax.Node{{
		Kind:       patternsyntax.KindClass,
		ClassItems: []patternsyntax.ClassItem{{Kind: patternsyntax.ClassItemRange, Low: 'a', High: 'é'}},
	}}}))
}

func TestCompileErrorMethods(t *testing.T) {
	t.Parallel()

	cause := errors.New("cause")
	err := &CompileError{Operation: "compile", Err: cause}
	require.Contains(t, err.Error(), "compile")
	require.ErrorIs(t, err, cause)
}
