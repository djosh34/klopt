//nolint:godoclint,mnd // Private Thompson construction mirrors the supported regexp ASTs.
package stringlanguage

import (
	"errors"
	regexpsyntax "regexp/syntax"
	"unicode"
	"unicode/utf16"

	"github.com/djosh34/klopt/pkg/internal/patternsyntax"
)

type edgeKind uint8

const (
	edgeEpsilon edgeKind = iota
	edgeCharacters
	edgeBeginText
	edgeEndText
	edgeBeginLine
	edgeEndLine
	edgeWordBoundary
	edgeNotWordBoundary
)

type nfaEdge struct {
	to         int
	kind       edgeKind
	characters runeSet
}

type nfaState struct{ edges []nfaEdge }

type nfa struct {
	states   []nfaState
	start    int
	accept   int
	universe runeSet
	utf16    bool
}

type fragment struct{ start, end int }

type nfaBuilder struct {
	machine nfa
}

func newNFABuilder(utf16Mode bool) *nfaBuilder {
	universe := scalarUniverse
	if utf16Mode {
		universe = codeUnitUniverse()
	}

	return &nfaBuilder{machine: nfa{universe: universe, utf16: utf16Mode}}
}

func compileESPattern(tree *patternsyntax.Tree) (*dfa, error) {
	root := tree.Nodes[tree.Root]
	if len(root.Children) == 1 {
		alternative := tree.Nodes[root.Children[0]]
		if len(alternative.Children) >= 2 &&
			tree.Nodes[alternative.Children[0]].Kind == patternsyntax.KindBeginInput &&
			isESLookahead(tree.Nodes[alternative.Children[1]].Kind) {
			return compileESLookaheadPattern(tree, alternative)
		}
	}

	return compileESLeaf(tree, []patternsyntax.NodeID{tree.Root}, false)
}

func compileESLookaheadPattern(_ *patternsyntax.Tree, _ patternsyntax.Node) (*dfa, error) {
	return nil, errors.New("leading lookahead patterns are not supported by direct language compilation")
}

func compileESLeaf(
	tree *patternsyntax.Tree,
	nodes []patternsyntax.NodeID,
	prependBegin bool,
) (*dfa, error) {
	builder := newNFABuilder(true)
	parts := make([]fragment, 0, len(nodes)+1)

	if prependBegin {
		begin, err := builder.assertion(edgeBeginText)
		if err != nil {
			return nil, err
		}

		parts = append(parts, begin)
	}

	for _, node := range nodes {
		built, err := builder.buildESNode(tree, node)
		if err != nil {
			return nil, err
		}

		parts = append(parts, built)
	}

	root, err := builder.concatenate(parts)
	if err != nil {
		return nil, err
	}

	if err := builder.wrapSearch(root); err != nil {
		return nil, err
	}

	return determinize(&builder.machine)
}

func compileRawPattern(expression *regexpsyntax.Regexp) (*dfa, error) {
	builder := newNFABuilder(false)

	root, err := builder.buildRawNode(expression)
	if err != nil {
		return nil, err
	}

	if err := builder.wrapSearch(root); err != nil {
		return nil, err
	}

	return determinize(&builder.machine)
}

//nolint:cyclop // The closed AST-kind dispatch mirrors patternsyntax directly.
func (builder *nfaBuilder) buildESNode(tree *patternsyntax.Tree, nodeID patternsyntax.NodeID) (fragment, error) {
	node := tree.Nodes[nodeID]
	switch node.Kind {
	case patternsyntax.KindExpression:
		return builder.buildESChildren(tree, node.Children, true)
	case patternsyntax.KindAlternative:
		return builder.buildESChildren(tree, node.Children, false)
	case patternsyntax.KindLiteral:
		return builder.esLiteral(node.Value)
	case patternsyntax.KindDot:
		return builder.characters(complementRuneSet(runeSet{
			{first: '\r', last: '\r'},
			{first: '\n', last: '\n'},
			{first: 0x2028, last: 0x2029},
		}, builder.machine.universe))
	case patternsyntax.KindClass:
		return builder.characters(esClassSet(node))
	case patternsyntax.KindDigit:
		return builder.characters(digitSet())
	case patternsyntax.KindNotDigit:
		return builder.characters(complementRuneSet(digitSet(), builder.machine.universe))
	case patternsyntax.KindSpace:
		return builder.characters(spaceSet())
	case patternsyntax.KindNotSpace:
		return builder.characters(complementRuneSet(spaceSet(), builder.machine.universe))
	case patternsyntax.KindWord:
		return builder.characters(wordSet())
	case patternsyntax.KindNotWord:
		return builder.characters(complementRuneSet(wordSet(), builder.machine.universe))
	case patternsyntax.KindBeginInput:
		return builder.assertion(edgeBeginText)
	case patternsyntax.KindEndInput:
		return builder.assertion(edgeEndText)
	case patternsyntax.KindWordBoundary:
		return builder.assertion(edgeWordBoundary)
	case patternsyntax.KindNotWordBoundary:
		return builder.assertion(edgeNotWordBoundary)
	case patternsyntax.KindCapture, patternsyntax.KindGroup:
		return builder.buildESNode(tree, node.Children[0])
	case patternsyntax.KindRepeat:
		child := tree.Nodes[node.Children[0]]
		if child.Kind == patternsyntax.KindLiteral && child.Value > maximumCodeUnit {
			return builder.repeatAstralLiteral(child.Value, node)
		}

		return builder.repeat(
			func() (fragment, error) { return builder.buildESNode(tree, node.Children[0]) },
			node.Repeat.Minimum,
			node.Repeat.Maximum,
			node.Repeat.Unbounded,
		)
	case patternsyntax.KindPositiveLookahead, patternsyntax.KindNegativeLookahead:
		return fragment{}, errors.New("internal error: lookahead reached ordinary NFA construction")
	default:
		return fragment{}, errors.New("internal error: unknown ES5.1 AST node")
	}
}

func (builder *nfaBuilder) repeatAstralLiteral(value rune, node patternsyntax.Node) (fragment, error) {
	high, low := utf16.EncodeRune(value)

	prefix, err := builder.characters(runeSet{{first: high, last: high}})
	if err != nil {
		return fragment{}, err
	}

	repeated, err := builder.repeat(
		func() (fragment, error) {
			return builder.characters(runeSet{{first: low, last: low}})
		},
		node.Repeat.Minimum,
		node.Repeat.Maximum,
		node.Repeat.Unbounded,
	)
	if err != nil {
		return fragment{}, err
	}

	return builder.concatenate([]fragment{prefix, repeated})
}

func (builder *nfaBuilder) buildESChildren(
	tree *patternsyntax.Tree,
	children []patternsyntax.NodeID,
	alternate bool,
) (fragment, error) {
	parts := make([]fragment, 0, len(children))
	for _, child := range children {
		built, err := builder.buildESNode(tree, child)
		if err != nil {
			return fragment{}, err
		}

		parts = append(parts, built)
	}

	if alternate {
		return builder.alternate(parts)
	}

	return builder.concatenate(parts)
}

func (builder *nfaBuilder) esLiteral(value rune) (fragment, error) {
	if value <= maximumCodeUnit {
		return builder.characters(runeSet{{first: value, last: value}})
	}

	high, low := utf16.EncodeRune(value)

	first, err := builder.characters(runeSet{{first: high, last: high}})
	if err != nil {
		return fragment{}, err
	}

	second, err := builder.characters(runeSet{{first: low, last: low}})
	if err != nil {
		return fragment{}, err
	}

	return builder.concatenate([]fragment{first, second})
}

//nolint:cyclop // The closed regexp/syntax operation dispatch is intentionally explicit.
func (builder *nfaBuilder) buildRawNode(expression *regexpsyntax.Regexp) (fragment, error) {
	switch expression.Op {
	case regexpsyntax.OpNoMatch:
		return builder.noMatch()
	case regexpsyntax.OpEmptyMatch:
		return builder.empty()
	case regexpsyntax.OpLiteral:
		parts := make([]fragment, 0, len(expression.Rune))
		for _, value := range expression.Rune {
			characters := runeSet{{first: value, last: value}}
			if expression.Flags&regexpsyntax.FoldCase != 0 {
				for folded := unicode.SimpleFold(value); folded != value; folded = unicode.SimpleFold(folded) {
					characters = append(characters, runeRange{first: folded, last: folded})
				}
			}

			built, err := builder.characters(normalizeRuneSet(characters))
			if err != nil {
				return fragment{}, err
			}

			parts = append(parts, built)
		}

		return builder.concatenate(parts)
	case regexpsyntax.OpCharClass:
		set := make(runeSet, 0, len(expression.Rune)/2)
		for index := 0; index+1 < len(expression.Rune); index += 2 {
			set = append(set, runeRange{first: expression.Rune[index], last: expression.Rune[index+1]})
		}

		return builder.characters(intersectRuneSets(normalizeRuneSet(set), scalarUniverse))
	case regexpsyntax.OpAnyCharNotNL:
		return builder.characters(complementRuneSet(runeSet{{first: '\n', last: '\n'}}, scalarUniverse))
	case regexpsyntax.OpAnyChar:
		return builder.characters(scalarUniverse)
	case regexpsyntax.OpBeginLine:
		return builder.assertion(edgeBeginLine)
	case regexpsyntax.OpEndLine:
		return builder.assertion(edgeEndLine)
	case regexpsyntax.OpBeginText:
		return builder.assertion(edgeBeginText)
	case regexpsyntax.OpEndText:
		return builder.assertion(edgeEndText)
	case regexpsyntax.OpWordBoundary:
		return builder.assertion(edgeWordBoundary)
	case regexpsyntax.OpNoWordBoundary:
		return builder.assertion(edgeNotWordBoundary)
	case regexpsyntax.OpCapture:
		return builder.buildRawNode(expression.Sub[0])
	case regexpsyntax.OpStar:
		return builder.repeat(func() (fragment, error) { return builder.buildRawNode(expression.Sub[0]) }, 0, 0, true)
	case regexpsyntax.OpPlus:
		return builder.repeat(func() (fragment, error) { return builder.buildRawNode(expression.Sub[0]) }, 1, 0, true)
	case regexpsyntax.OpQuest:
		return builder.repeat(func() (fragment, error) { return builder.buildRawNode(expression.Sub[0]) }, 0, 1, false)
	case regexpsyntax.OpRepeat:
		return builder.repeat(
			func() (fragment, error) { return builder.buildRawNode(expression.Sub[0]) },
			expression.Min,
			expression.Max,
			expression.Max < 0,
		)
	case regexpsyntax.OpConcat, regexpsyntax.OpAlternate:
		parts := make([]fragment, 0, len(expression.Sub))
		for _, child := range expression.Sub {
			built, err := builder.buildRawNode(child)
			if err != nil {
				return fragment{}, err
			}

			parts = append(parts, built)
		}

		if expression.Op == regexpsyntax.OpAlternate {
			return builder.alternate(parts)
		}

		return builder.concatenate(parts)
	default:
		return fragment{}, errors.New("raw Go regexp generator does not support regexp operation")
	}
}
