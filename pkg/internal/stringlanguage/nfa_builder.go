//nolint:godoclint,mnd // Private Thompson graph mechanics are kept separate from AST lowering.
package stringlanguage

import (
	"unicode/utf16"

	"github.com/djosh34/klopt/pkg/internal/patternsyntax"
)

//nolint:nestif // Bounded and unbounded Thompson construction share the required prefix.
func (builder *nfaBuilder) repeat(
	build func() (fragment, error),
	minimum int,
	maximum int,
	unbounded bool,
) (fragment, error) {
	parts := make([]fragment, 0, max(minimum, maximum))
	for range minimum {
		part, err := build()
		if err != nil {
			return fragment{}, err
		}

		parts = append(parts, part)
	}

	if unbounded {
		part, err := build()
		if err != nil {
			return fragment{}, err
		}

		starred, err := builder.star(part)
		if err != nil {
			return fragment{}, err
		}

		parts = append(parts, starred)
	} else {
		for range maximum - minimum {
			part, err := build()
			if err != nil {
				return fragment{}, err
			}

			optional, err := builder.optional(part)
			if err != nil {
				return fragment{}, err
			}

			parts = append(parts, optional)
		}
	}

	return builder.concatenate(parts)
}

func (builder *nfaBuilder) newState() (int, error) {
	if err := builder.budget.add(
		&builder.budget.nfaStates, 1, builder.budget.limits.nfaStates,
		"NFA construction", "NFA states",
	); err != nil {
		return 0, err
	}

	state := len(builder.machine.states)
	builder.machine.states = append(builder.machine.states, nfaState{})

	return state, nil
}

func (builder *nfaBuilder) addEdge(from int, edge nfaEdge) error {
	if err := builder.budget.add(
		&builder.budget.nfaEdges, 1, builder.budget.limits.nfaEdges,
		"NFA construction", "NFA edges",
	); err != nil {
		return err
	}

	builder.machine.states[from].edges = append(builder.machine.states[from].edges, edge)

	return nil
}

func (builder *nfaBuilder) empty() (fragment, error) {
	state, err := builder.newState()
	if err != nil {
		return fragment{}, err
	}

	return fragment{start: state, end: state}, nil
}

func (builder *nfaBuilder) noMatch() (fragment, error) {
	start, err := builder.newState()
	if err != nil {
		return fragment{}, err
	}

	end, err := builder.newState()
	if err != nil {
		return fragment{}, err
	}

	return fragment{start: start, end: end}, nil
}

func (builder *nfaBuilder) characters(set runeSet) (fragment, error) {
	start, err := builder.newState()
	if err != nil {
		return fragment{}, err
	}

	end, err := builder.newState()
	if err != nil {
		return fragment{}, err
	}

	if err := builder.addEdge(start, nfaEdge{
		to: end, kind: edgeCharacters, characters: normalizeRuneSet(set),
	}); err != nil {
		return fragment{}, err
	}

	return fragment{start: start, end: end}, nil
}

func (builder *nfaBuilder) assertion(kind edgeKind) (fragment, error) {
	start, err := builder.newState()
	if err != nil {
		return fragment{}, err
	}

	end, err := builder.newState()
	if err != nil {
		return fragment{}, err
	}

	if err := builder.addEdge(start, nfaEdge{to: end, kind: kind}); err != nil {
		return fragment{}, err
	}

	return fragment{start: start, end: end}, nil
}

func (builder *nfaBuilder) concatenate(parts []fragment) (fragment, error) {
	if len(parts) == 0 {
		return builder.empty()
	}

	result := parts[0]
	for _, part := range parts[1:] {
		if err := builder.addEdge(result.end, nfaEdge{to: part.start, kind: edgeEpsilon}); err != nil {
			return fragment{}, err
		}

		result.end = part.end
	}

	return result, nil
}

func (builder *nfaBuilder) alternate(parts []fragment) (fragment, error) {
	if len(parts) == 0 {
		return builder.empty()
	}

	start, err := builder.newState()
	if err != nil {
		return fragment{}, err
	}

	end, err := builder.newState()
	if err != nil {
		return fragment{}, err
	}

	for _, part := range parts {
		if err := builder.addEdge(start, nfaEdge{to: part.start, kind: edgeEpsilon}); err != nil {
			return fragment{}, err
		}

		if err := builder.addEdge(part.end, nfaEdge{to: end, kind: edgeEpsilon}); err != nil {
			return fragment{}, err
		}
	}

	return fragment{start: start, end: end}, nil
}

func (builder *nfaBuilder) optional(part fragment) (fragment, error) {
	start, err := builder.newState()
	if err != nil {
		return fragment{}, err
	}

	end, err := builder.newState()
	if err != nil {
		return fragment{}, err
	}

	for _, edge := range []nfaEdge{
		{to: part.start, kind: edgeEpsilon},
		{to: end, kind: edgeEpsilon},
	} {
		if err := builder.addEdge(start, edge); err != nil {
			return fragment{}, err
		}
	}

	if err := builder.addEdge(part.end, nfaEdge{to: end, kind: edgeEpsilon}); err != nil {
		return fragment{}, err
	}

	return fragment{start: start, end: end}, nil
}

func (builder *nfaBuilder) star(part fragment) (fragment, error) {
	start, err := builder.newState()
	if err != nil {
		return fragment{}, err
	}

	end, err := builder.newState()
	if err != nil {
		return fragment{}, err
	}

	for _, edge := range []nfaEdge{
		{to: part.start, kind: edgeEpsilon},
		{to: end, kind: edgeEpsilon},
	} {
		if err := builder.addEdge(start, edge); err != nil {
			return fragment{}, err
		}
	}

	for _, edge := range []nfaEdge{
		{to: part.start, kind: edgeEpsilon},
		{to: end, kind: edgeEpsilon},
	} {
		if err := builder.addEdge(part.end, edge); err != nil {
			return fragment{}, err
		}
	}

	return fragment{start: start, end: end}, nil
}

func (builder *nfaBuilder) wrapSearch(root fragment) error {
	start, err := builder.newState()
	if err != nil {
		return err
	}

	accept, err := builder.newState()
	if err != nil {
		return err
	}

	for _, edge := range []nfaEdge{
		{to: root.start, kind: edgeEpsilon},
		{to: start, kind: edgeCharacters, characters: builder.machine.universe},
	} {
		if err := builder.addEdge(start, edge); err != nil {
			return err
		}
	}

	if err := builder.addEdge(root.end, nfaEdge{to: accept, kind: edgeEpsilon}); err != nil {
		return err
	}

	if err := builder.addEdge(accept, nfaEdge{
		to: accept, kind: edgeCharacters, characters: builder.machine.universe,
	}); err != nil {
		return err
	}

	builder.machine.start = start
	builder.machine.accept = accept

	return nil
}

func digitSet() runeSet {
	return runeSet{{first: '0', last: '9'}}
}

func wordSet() runeSet {
	return runeSet{
		{first: '0', last: '9'},
		{first: 'A', last: 'Z'},
		{first: '_', last: '_'},
		{first: 'a', last: 'z'},
	}
}

func spaceSet() runeSet {
	return runeSet{
		{first: 0x09, last: 0x0d},
		{first: 0x20, last: 0x20},
		{first: 0x00a0, last: 0x00a0},
		{first: 0x1680, last: 0x1680},
		{first: 0x180e, last: 0x180e},
		{first: 0x2000, last: 0x200b},
		{first: 0x2028, last: 0x2029},
		{first: 0x202f, last: 0x202f},
		{first: 0x3000, last: 0x3000},
		{first: 0xfeff, last: 0xfeff},
	}
}

func esClassSet(node patternsyntax.Node) runeSet {
	set := make(runeSet, 0)

	for _, item := range node.ClassItems {
		switch item.Kind {
		case patternsyntax.ClassItemRange:
			set = appendESClassRange(set, item.Low, item.High)
		case patternsyntax.ClassItemDigit:
			set = unionRuneSets(set, digitSet())
		case patternsyntax.ClassItemNotDigit:
			set = unionRuneSets(set, complementRuneSet(digitSet(), codeUnitUniverse()))
		case patternsyntax.ClassItemSpace:
			set = unionRuneSets(set, spaceSet())
		case patternsyntax.ClassItemNotSpace:
			set = unionRuneSets(set, complementRuneSet(spaceSet(), codeUnitUniverse()))
		case patternsyntax.ClassItemWord:
			set = unionRuneSets(set, wordSet())
		case patternsyntax.ClassItemNotWord:
			set = unionRuneSets(set, complementRuneSet(wordSet(), codeUnitUniverse()))
		}
	}

	set = normalizeRuneSet(set)
	if node.Negated {
		return complementRuneSet(set, codeUnitUniverse())
	}

	return set
}

func appendESClassRange(set runeSet, low rune, high rune) runeSet {
	if low == high && low > maximumCodeUnit {
		highSurrogate, lowSurrogate := utf16.EncodeRune(low)

		return append(
			set,
			runeRange{first: highSurrogate, last: highSurrogate},
			runeRange{first: lowSurrogate, last: lowSurrogate},
		)
	}

	if low > maximumCodeUnit {
		return set
	}

	return append(set, runeRange{first: low, last: min(high, maximumCodeUnit)})
}

func isESLookahead(kind patternsyntax.Kind) bool {
	return kind == patternsyntax.KindPositiveLookahead || kind == patternsyntax.KindNegativeLookahead
}
