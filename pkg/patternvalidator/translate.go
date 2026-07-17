//nolint:godoclint,mnd // Private AST lowering uses the normative ES5.1 Unicode code-point table.
package patternvalidator

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/djosh34/klopt/pkg/internal/patternsyntax"
)

type checkSpecification struct {
	source    string
	wantMatch bool
	span      patternsyntax.Span
}

type translator struct {
	tree    *patternsyntax.Tree
	output  strings.Builder
	failure error
}

type runeRange struct {
	low  rune
	high rune
}

var (
	digitRanges = []runeRange{{low: '0', high: '9'}}
	wordRanges  = []runeRange{
		{low: '0', high: '9'},
		{low: 'A', high: 'Z'},
		{low: '_', high: '_'},
		{low: 'a', high: 'z'},
	}
	spaceRanges = []runeRange{
		{low: 0x09, high: 0x0d},
		{low: 0x20, high: 0x20},
		{low: 0x00a0, high: 0x00a0},
		{low: 0x1680, high: 0x1680},
		{low: 0x180e, high: 0x180e},
		{low: 0x2000, high: 0x200b},
		{low: 0x2028, high: 0x2029},
		{low: 0x202f, high: 0x202f},
		{low: 0x3000, high: 0x3000},
		{low: 0xfeff, high: 0xfeff},
	}
)

func translate(tree *patternsyntax.Tree) ([]checkSpecification, error) {
	root := tree.Nodes[tree.Root]

	alternative := tree.Nodes[root.Children[0]]
	if len(alternative.Children) >= 2 &&
		tree.Nodes[alternative.Children[0]].Kind == patternsyntax.KindBeginInput &&
		isLookahead(tree.Nodes[alternative.Children[1]].Kind) {
		return translateLeadingLookaheads(tree, alternative)
	}

	compiled, err := translateNode(tree, tree.Root)
	if err != nil {
		return nil, err
	}

	return []checkSpecification{{source: compiled, wantMatch: true, span: root.Span}}, nil
}

func translateLeadingLookaheads(
	tree *patternsyntax.Tree,
	alternative patternsyntax.Node,
) ([]checkSpecification, error) {
	checks := make([]checkSpecification, 0)

	index := 1
	for index < len(alternative.Children) {
		node := tree.Nodes[alternative.Children[index]]
		if !isLookahead(node.Kind) {
			break
		}

		translated, err := translateNode(tree, node.Children[0])
		if err != nil {
			return nil, err
		}

		source, err := joinedCheckSource("\\A(?:", translated, ")")
		if err != nil {
			return nil, err
		}

		checks = append(checks, checkSpecification{
			source: source, wantMatch: node.Kind == patternsyntax.KindPositiveLookahead, span: node.Span,
		})
		index++
	}

	translated, err := translateSequence(tree, alternative.Children[index:])
	if err != nil {
		return nil, err
	}

	source, err := joinedCheckSource("\\A(?:", translated, ")")
	if err != nil {
		return nil, err
	}

	checks = append(checks, checkSpecification{
		source: source, wantMatch: true,
		span: patternsyntax.Span{Start: 0, End: alternative.Span.End},
	})

	return checks, nil
}

func translateNode(tree *patternsyntax.Tree, nodeID patternsyntax.NodeID) (string, error) {
	translation := translator{tree: tree}
	translation.writeNode(nodeID)

	if translation.failure != nil {
		return "", translation.failure
	}

	return translation.output.String(), nil
}

func translateSequence(tree *patternsyntax.Tree, nodes []patternsyntax.NodeID) (string, error) {
	translation := translator{tree: tree}
	for _, node := range nodes {
		translation.writeNode(node)
	}

	if translation.failure != nil {
		return "", translation.failure
	}

	return translation.output.String(), nil
}

//nolint:cyclop // The explicit kind dispatch is the backend's complete lowering table.
func (translation *translator) writeNode(nodeID patternsyntax.NodeID) {
	if translation.failure != nil {
		return
	}

	node := translation.tree.Nodes[nodeID]

	switch node.Kind {
	case patternsyntax.KindExpression:
		translation.write("(?:", node.Span)

		for index, child := range node.Children {
			if index != 0 {
				translation.write("|", node.Span)
			}

			translation.writeNode(child)
		}

		translation.write(")", node.Span)
	case patternsyntax.KindAlternative:
		for _, child := range node.Children {
			translation.writeNode(child)
		}
	case patternsyntax.KindLiteral:
		translation.writeLiteral(node.Value, node.Span)
	case patternsyntax.KindDot:
		translation.write("[^\\r\\n\\x{2028}\\x{2029}]", node.Span)
	case patternsyntax.KindClass:
		translation.writeClass(node)
	case patternsyntax.KindDigit:
		translation.writeRuneSet(digitRanges, node.Span)
	case patternsyntax.KindNotDigit:
		translation.writeRuneSet(complementRuneSet(digitRanges), node.Span)
	case patternsyntax.KindSpace:
		translation.writeRuneSet(spaceRanges, node.Span)
	case patternsyntax.KindNotSpace:
		translation.writeRuneSet(complementRuneSet(spaceRanges), node.Span)
	case patternsyntax.KindWord:
		translation.writeRuneSet(wordRanges, node.Span)
	case patternsyntax.KindNotWord:
		translation.writeRuneSet(complementRuneSet(wordRanges), node.Span)
	case patternsyntax.KindBeginInput:
		translation.write("\\A", node.Span)
	case patternsyntax.KindEndInput:
		translation.write("\\z", node.Span)
	case patternsyntax.KindWordBoundary:
		translation.write("\\b", node.Span)
	case patternsyntax.KindNotWordBoundary:
		translation.write("\\B", node.Span)
	case patternsyntax.KindCapture, patternsyntax.KindGroup:
		translation.write("(?:", node.Span)
		translation.writeNode(node.Children[0])
		translation.write(")", node.Span)
	case patternsyntax.KindRepeat:
		translation.writeRepeatedNode(node)
	case patternsyntax.KindPositiveLookahead, patternsyntax.KindNegativeLookahead:
		translation.failure = &ParseError{
			Kind: ParseErrorInternalTranslation, Offset: node.Span.Start,
			Cause: fmt.Errorf("unexpected lookahead node in ordinary translation"),
		}
	}
}

func (translation *translator) writeLiteral(value rune, span patternsyntax.Span) {
	if value <= 0xffff {
		translation.writeCodeUnit(value, span)

		return
	}

	high, low := utf16.EncodeRune(value)
	translation.writeCodeUnit(high, span)
	translation.writeCodeUnit(low, span)
}

func mapSurrogate(value rune) rune {
	return 0x10000 + value - 0xd800
}

func (translation *translator) writeCodeUnit(value rune, span patternsyntax.Span) {
	if value >= 0xd800 && value <= 0xdfff {
		value = mapSurrogate(value)
	}

	translation.write(fmt.Sprintf("\\x{%x}", value), span)
}

func (translation *translator) writeRepeatedNode(node patternsyntax.Node) {
	child := translation.tree.Nodes[node.Children[0]]
	if child.Kind == patternsyntax.KindLiteral && child.Value > 0xffff {
		high, low := utf16.EncodeRune(child.Value)
		translation.writeCodeUnit(high, child.Span)
		translation.write("(?:", node.Span)
		translation.writeCodeUnit(low, child.Span)
	} else {
		translation.write("(?:", node.Span)
		translation.writeNode(node.Children[0])
	}

	translation.write(")", node.Span)
	translation.writeRepeat(node)
}

func (translation *translator) writeClass(node patternsyntax.Node) {
	ranges := make([]runeRange, 0, len(node.ClassItems))
	for _, item := range node.ClassItems {
		switch item.Kind {
		case patternsyntax.ClassItemRange:
			ranges = appendLiteralClassRange(ranges, item.Low, item.High)
		case patternsyntax.ClassItemDigit:
			ranges = append(ranges, digitRanges...)
		case patternsyntax.ClassItemNotDigit:
			ranges = append(ranges, complementRuneSet(digitRanges)...)
		case patternsyntax.ClassItemSpace:
			ranges = append(ranges, spaceRanges...)
		case patternsyntax.ClassItemNotSpace:
			ranges = append(ranges, complementRuneSet(spaceRanges)...)
		case patternsyntax.ClassItemWord:
			ranges = append(ranges, wordRanges...)
		case patternsyntax.ClassItemNotWord:
			ranges = append(ranges, complementRuneSet(wordRanges)...)
		}
	}

	ranges = normalizeRuneSet(ranges)
	if node.Negated {
		ranges = complementRuneSet(ranges)
	}

	translation.writeRuneSet(ranges, node.Span)
}

func appendLiteralClassRange(ranges []runeRange, low rune, high rune) []runeRange {
	if low != high || low <= 0xffff {
		return append(ranges, runeRange{low: low, high: high})
	}

	highSurrogate, lowSurrogate := utf16.EncodeRune(low)

	return append(
		ranges,
		runeRange{low: mapSurrogate(highSurrogate), high: mapSurrogate(highSurrogate)},
		runeRange{low: mapSurrogate(lowSurrogate), high: mapSurrogate(lowSurrogate)},
	)
}

func (translation *translator) writeRuneSet(ranges []runeRange, span patternsyntax.Span) {
	if len(ranges) == 0 {
		translation.write("(?:\\b\\B)", span)

		return
	}

	translation.write("[", span)

	for _, characterRange := range ranges {
		translation.write(fmt.Sprintf("\\x{%x}", characterRange.low), span)

		if characterRange.low != characterRange.high {
			translation.write(fmt.Sprintf("-\\x{%x}", characterRange.high), span)
		}
	}

	translation.write("]", span)
}

//nolint:cyclop // Repeat forms map directly to their canonical Go regexp spellings.
func (translation *translator) writeRepeat(node patternsyntax.Node) {
	repeat := node.Repeat

	var suffix string

	switch {
	case !repeat.Counted && repeat.Minimum == 0 && repeat.Unbounded:
		suffix = "*"
	case !repeat.Counted && repeat.Minimum == 1 && repeat.Unbounded:
		suffix = "+"
	case !repeat.Counted && repeat.Minimum == 0 && repeat.Maximum == 1:
		suffix = "?"
	case repeat.Unbounded:
		suffix = fmt.Sprintf("{%d,}", repeat.Minimum)
	case repeat.Minimum == repeat.Maximum:
		suffix = fmt.Sprintf("{%d}", repeat.Minimum)
	default:
		suffix = fmt.Sprintf("{%d,%d}", repeat.Minimum, repeat.Maximum)
	}

	if repeat.Lazy {
		suffix += "?"
	}

	translation.write(suffix, node.Span)
}

func (translation *translator) write(piece string, span patternsyntax.Span) {
	if translation.failure != nil {
		return
	}

	observed := translation.output.Len() + len(piece)
	if observed > maximumGeneratedRegexpBytes {
		translation.failure = &ComplexityError{
			Phase: "translation", Limit: "generated Go regexp bytes",
			Maximum: maximumGeneratedRegexpBytes, Observed: uint64(observed),
		}

		return
	}

	if _, err := translation.output.WriteString(piece); err != nil {
		translation.failure = &ParseError{
			Kind: ParseErrorInternalTranslation, Offset: span.Start, Cause: err,
		}
	}
}

func normalizeRuneSet(ranges []runeRange) []runeRange {
	if len(ranges) == 0 {
		return nil
	}

	slices.SortFunc(ranges, func(left runeRange, right runeRange) int {
		if left.low != right.low {
			return int(left.low - right.low)
		}

		return int(left.high - right.high)
	})

	result := make([]runeRange, 0, len(ranges))
	for _, candidate := range ranges {
		if len(result) == 0 || candidate.low > result[len(result)-1].high+1 {
			result = append(result, candidate)

			continue
		}

		result[len(result)-1].high = max(result[len(result)-1].high, candidate.high)
	}

	return result
}

func complementRuneSet(ranges []runeRange) []runeRange {
	normalized := normalizeRuneSet(slices.Clone(ranges))
	result := make([]runeRange, 0, len(normalized)+1)
	next := rune(0)

	for _, excluded := range normalized {
		if next < excluded.low {
			result = append(result, runeRange{low: next, high: excluded.low - 1})
		}

		if excluded.high == unicode.MaxRune {
			return result
		}

		next = excluded.high + 1
	}

	if next <= unicode.MaxRune {
		result = append(result, runeRange{low: next, high: unicode.MaxRune})
	}

	return result
}

func joinedCheckSource(parts ...string) (string, error) {
	length := 0
	for _, part := range parts {
		length += len(part)
		if length > maximumGeneratedRegexpBytes {
			return "", &ComplexityError{
				Phase: "translation", Limit: "generated Go regexp bytes",
				Maximum: maximumGeneratedRegexpBytes, Observed: uint64(length),
			}
		}
	}

	return strings.Join(parts, ""), nil
}

func isLookahead(kind patternsyntax.Kind) bool {
	return kind == patternsyntax.KindPositiveLookahead || kind == patternsyntax.KindNegativeLookahead
}
