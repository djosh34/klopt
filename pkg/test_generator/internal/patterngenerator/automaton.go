//nolint:godoclint,mnd // Private automata vocabulary and bit widths are local implementation details.
package patterngenerator

import (
	"encoding/binary"
	"errors"
	regexpsyntax "regexp/syntax"
	"slices"

	"github.com/djosh34/klopt/pkg/internal/patternsyntax"
)

type byteSet [2]uint64

func fullByteSet() byteSet {
	return byteSet{^uint64(0), ^uint64(0)}
}

func (set *byteSet) add(value byte) {
	set[value/64] |= uint64(1) << (value % 64)
}

func (set *byteSet) addRange(low rune, high rune) {
	low = max(low, 0)

	high = min(high, asciiAlphabetSize-1)
	if low > high {
		return
	}

	for value := low; value <= high; value++ {
		set.add(byte(value))
	}
}

func (set byteSet) contains(value byte) bool {
	return set[value/64]&(uint64(1)<<(value%64)) != 0
}

func (set byteSet) complement() byteSet {
	return byteSet{^set[0], ^set[1]}
}

func (set *byteSet) union(other byteSet) {
	set[0] |= other[0]
	set[1] |= other[1]
}

func (set byteSet) values() []byte {
	values := make([]byte, 0, asciiAlphabetSize)
	for value := range asciiAlphabetSize {
		if set.contains(byte(value)) {
			values = append(values, byte(value))
		}
	}

	return values
}

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
	characters byteSet
}

type nfaState struct {
	edges []nfaEdge
}

type nfa struct {
	states []nfaState
	start  int
	accept int
}

type fragment struct {
	start int
	end   int
}

type nfaBuilder struct {
	machine nfa
	budget  *budget
}

type dfaState struct {
	transitions [asciiAlphabetSize]uint32
	accepting   bool
}

type dfa struct {
	states []dfaState
}

type subsetState struct {
	states       []int
	atStart      bool
	previousWord bool
	previousLF   bool
}

type leafSpecification struct {
	machine   *dfa
	wantMatch bool
}

func compileESPattern(tree *patternsyntax.Tree, work *budget) (*dfa, error) {
	root := tree.Nodes[tree.Root]
	if len(root.Children) == 1 {
		alternative := tree.Nodes[root.Children[0]]
		if len(alternative.Children) >= 2 &&
			tree.Nodes[alternative.Children[0]].Kind == patternsyntax.KindBeginInput &&
			isESLookahead(tree.Nodes[alternative.Children[1]].Kind) {
			return compileESLookaheadPattern(tree, alternative, work)
		}
	}

	machine, err := compileESLeaf(tree, []patternsyntax.NodeID{tree.Root}, false, work)
	if err != nil {
		return nil, err
	}

	return machine, nil
}

func compileESLookaheadPattern(
	tree *patternsyntax.Tree,
	alternative patternsyntax.Node,
	work *budget,
) (*dfa, error) {
	leaves := make([]leafSpecification, 0)
	index := 1

	for index < len(alternative.Children) {
		assertion := tree.Nodes[alternative.Children[index]]
		if !isESLookahead(assertion.Kind) {
			break
		}

		machine, err := compileESLeaf(tree, assertion.Children, true, work)
		if err != nil {
			return nil, err
		}

		leaves = append(leaves, leafSpecification{
			machine: machine, wantMatch: assertion.Kind == patternsyntax.KindPositiveLookahead,
		})
		index++
	}

	remainder, err := compileESLeaf(tree, alternative.Children[index:], true, work)
	if err != nil {
		return nil, err
	}

	leaves = append(leaves, leafSpecification{machine: remainder, wantMatch: true})

	return combineLeaves(leaves, work)
}

func compileESLeaf(
	tree *patternsyntax.Tree,
	nodes []patternsyntax.NodeID,
	prependBegin bool,
	work *budget,
) (*dfa, error) {
	builder := &nfaBuilder{budget: work}
	fragments := make([]fragment, 0, len(nodes)+1)

	if prependBegin {
		begin, err := builder.assertion(edgeBeginText)
		if err != nil {
			return nil, err
		}

		fragments = append(fragments, begin)
	}

	for _, node := range nodes {
		built, err := builder.buildESNode(tree, node)
		if err != nil {
			return nil, err
		}

		fragments = append(fragments, built)
	}

	root, err := builder.concatenate(fragments)
	if err != nil {
		return nil, err
	}

	if err := builder.wrapSearch(root); err != nil {
		return nil, err
	}

	return determinize(&builder.machine, work)
}

func compileRawPattern(expression *regexpsyntax.Regexp, work *budget) (*dfa, error) {
	builder := &nfaBuilder{budget: work}

	root, err := builder.buildRawNode(expression)
	if err != nil {
		return nil, err
	}

	if err := builder.wrapSearch(root); err != nil {
		return nil, err
	}

	return determinize(&builder.machine, work)
}

//nolint:cyclop // The closed AST-kind dispatch mirrors patternsyntax directly.
func (builder *nfaBuilder) buildESNode(
	tree *patternsyntax.Tree,
	nodeID patternsyntax.NodeID,
) (fragment, error) {
	node := tree.Nodes[nodeID]

	switch node.Kind {
	case patternsyntax.KindExpression:
		alternatives := make([]fragment, 0, len(node.Children))
		for _, child := range node.Children {
			built, err := builder.buildESNode(tree, child)
			if err != nil {
				return fragment{}, err
			}

			alternatives = append(alternatives, built)
		}

		return builder.alternate(alternatives)
	case patternsyntax.KindAlternative:
		parts := make([]fragment, 0, len(node.Children))
		for _, child := range node.Children {
			built, err := builder.buildESNode(tree, child)
			if err != nil {
				return fragment{}, err
			}

			parts = append(parts, built)
		}

		return builder.concatenate(parts)
	case patternsyntax.KindLiteral:
		return builder.characters(singleRuneSet(node.Value))
	case patternsyntax.KindDot:
		set := fullByteSet()
		set[0] &^= uint64(1) << ('\r' % 64)
		set[0] &^= uint64(1) << ('\n' % 64)

		return builder.characters(set)
	case patternsyntax.KindClass:
		return builder.characters(esClassSet(node))
	case patternsyntax.KindDigit:
		return builder.characters(digitSet())
	case patternsyntax.KindNotDigit:
		return builder.characters(digitSet().complement())
	case patternsyntax.KindSpace:
		return builder.characters(spaceSet())
	case patternsyntax.KindNotSpace:
		return builder.characters(spaceSet().complement())
	case patternsyntax.KindWord:
		return builder.characters(wordSet())
	case patternsyntax.KindNotWord:
		return builder.characters(wordSet().complement())
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
			built, err := builder.characters(singleRuneSet(value))
			if err != nil {
				return fragment{}, err
			}

			parts = append(parts, built)
		}

		return builder.concatenate(parts)
	case regexpsyntax.OpCharClass:
		set := byteSet{}
		for index := 0; index+1 < len(expression.Rune); index += 2 {
			set.addRange(expression.Rune[index], expression.Rune[index+1])
		}

		return builder.characters(set)
	case regexpsyntax.OpAnyCharNotNL:
		set := fullByteSet()
		set[0] &^= uint64(1) << ('\n' % 64)

		return builder.characters(set)
	case regexpsyntax.OpAnyChar:
		return builder.characters(fullByteSet())
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
	case regexpsyntax.OpConcat:
		parts := make([]fragment, 0, len(expression.Sub))
		for _, child := range expression.Sub {
			built, err := builder.buildRawNode(child)
			if err != nil {
				return fragment{}, err
			}

			parts = append(parts, built)
		}

		return builder.concatenate(parts)
	case regexpsyntax.OpAlternate:
		parts := make([]fragment, 0, len(expression.Sub))
		for _, child := range expression.Sub {
			built, err := builder.buildRawNode(child)
			if err != nil {
				return fragment{}, err
			}

			parts = append(parts, built)
		}

		return builder.alternate(parts)
	case regexpsyntax.OpStar:
		return builder.repeat(func() (fragment, error) {
			return builder.buildRawNode(expression.Sub[0])
		}, 0, 0, true)
	case regexpsyntax.OpPlus:
		return builder.repeat(func() (fragment, error) {
			return builder.buildRawNode(expression.Sub[0])
		}, 1, 0, true)
	case regexpsyntax.OpQuest:
		return builder.repeat(func() (fragment, error) {
			return builder.buildRawNode(expression.Sub[0])
		}, 0, 1, false)
	case regexpsyntax.OpRepeat:
		return builder.repeat(func() (fragment, error) {
			return builder.buildRawNode(expression.Sub[0])
		}, expression.Min, expression.Max, expression.Max < 0)
	default:
		return fragment{}, &CapabilityError{Feature: "regexp operation"}
	}
}

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
		&builder.budget.nfaStates,
		1,
		builder.budget.limits.nfaStates,
		"NFA construction",
		"NFA states",
	); err != nil {
		return 0, err
	}

	state := len(builder.machine.states)
	builder.machine.states = append(builder.machine.states, nfaState{})

	return state, nil
}

func (builder *nfaBuilder) addEdge(from int, edge nfaEdge) error {
	if err := builder.budget.add(
		&builder.budget.nfaEdges,
		1,
		builder.budget.limits.nfaEdges,
		"NFA construction",
		"NFA edges",
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

func (builder *nfaBuilder) characters(set byteSet) (fragment, error) {
	start, err := builder.newState()
	if err != nil {
		return fragment{}, err
	}

	end, err := builder.newState()
	if err != nil {
		return fragment{}, err
	}

	if err := builder.addEdge(start, nfaEdge{to: end, kind: edgeCharacters, characters: set}); err != nil {
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
		{to: start, kind: edgeCharacters, characters: fullByteSet()},
	} {
		if err := builder.addEdge(start, edge); err != nil {
			return err
		}
	}

	if err := builder.addEdge(root.end, nfaEdge{to: accept, kind: edgeEpsilon}); err != nil {
		return err
	}

	if err := builder.addEdge(accept, nfaEdge{
		to: accept, kind: edgeCharacters, characters: fullByteSet(),
	}); err != nil {
		return err
	}

	builder.machine.start = start
	builder.machine.accept = accept

	return nil
}

func determinize(machine *nfa, work *budget) (*dfa, error) {
	initialSubset := unconditionalClosure(machine, []int{machine.start})
	initial := subsetState{states: initialSubset, atStart: true}
	states := []subsetState{initial}
	keys := map[string]uint32{subsetKey(initial): 0}
	result := &dfa{}

	if err := addDFAState(work, result, machine, initial); err != nil {
		return nil, err
	}

	for current := 0; current < len(states); current++ {
		if err := work.add(
			&work.dfaTransitions,
			asciiAlphabetSize,
			work.limits.dfaTransitions,
			"DFA construction",
			"DFA transitions",
		); err != nil {
			return nil, err
		}

		for value := range asciiAlphabetSize {
			next := moveSubset(machine, states[current], byte(value))
			key := subsetKey(next)

			nextID, ok := keys[key]
			if !ok {
				nextID = uint32(len(states))

				if err := addDFAState(work, result, machine, next); err != nil {
					return nil, err
				}

				keys[key] = nextID

				states = append(states, next)
			}

			result.states[current].transitions[value] = nextID
		}
	}

	return result, nil
}

func addDFAState(work *budget, result *dfa, machine *nfa, state subsetState) error {
	if err := work.add(
		&work.dfaStates,
		1,
		work.limits.dfaStates,
		"DFA construction",
		"DFA states",
	); err != nil {
		return err
	}

	result.states = append(result.states, dfaState{accepting: acceptsAtEnd(machine, state)})

	return nil
}

func combineLeaves(leaves []leafSpecification, work *budget) (*dfa, error) {
	if len(leaves) == 1 && leaves[0].wantMatch {
		return leaves[0].machine, nil
	}

	initial := make([]uint32, len(leaves))
	tuples := [][]uint32{initial}
	keys := map[string]uint32{stateTupleKey(initial): 0}
	result := &dfa{}

	if err := addCombinedState(result, leaves, initial, work); err != nil {
		return nil, err
	}

	for current := 0; current < len(tuples); current++ {
		if err := work.add(
			&work.dfaTransitions,
			asciiAlphabetSize,
			work.limits.dfaTransitions,
			"DFA construction",
			"DFA transitions",
		); err != nil {
			return nil, err
		}

		for value := range asciiAlphabetSize {
			next := make([]uint32, len(leaves))
			for index, leaf := range leaves {
				next[index] = leaf.machine.states[tuples[current][index]].transitions[value]
			}

			key := stateTupleKey(next)

			nextID, ok := keys[key]
			if !ok {
				nextID = uint32(len(tuples))

				if err := addCombinedState(result, leaves, next, work); err != nil {
					return nil, err
				}

				keys[key] = nextID

				tuples = append(tuples, next)
			}

			result.states[current].transitions[value] = nextID
		}
	}

	return result, nil
}

func addCombinedState(
	result *dfa,
	leaves []leafSpecification,
	tuple []uint32,
	work *budget,
) error {
	if err := work.add(
		&work.dfaStates,
		1,
		work.limits.dfaStates,
		"DFA construction",
		"DFA states",
	); err != nil {
		return err
	}

	accepting := true
	for index, leaf := range leaves {
		if leaf.machine.states[tuple[index]].accepting != leaf.wantMatch {
			accepting = false

			break
		}
	}

	result.states = append(result.states, dfaState{accepting: accepting})

	return nil
}

func unconditionalClosure(machine *nfa, seeds []int) []int {
	seen := make([]bool, len(machine.states))

	stack := append([]int(nil), seeds...)
	result := make([]int, 0, len(seeds))

	for len(stack) > 0 {
		last := len(stack) - 1
		state := stack[last]
		stack = stack[:last]

		if seen[state] {
			continue
		}

		seen[state] = true
		result = append(result, state)

		for _, edge := range machine.states[state].edges {
			if edge.kind == edgeEpsilon {
				stack = append(stack, edge.to)
			}
		}
	}

	slices.Sort(result)

	return result
}

func assertionClosure(
	machine *nfa,
	seeds []int,
	atStart bool,
	previousWord bool,
	previousLF bool,
	next *byte,
) []int {
	seen := make([]bool, len(machine.states))

	stack := append([]int(nil), seeds...)
	result := make([]int, 0, len(seeds))

	for len(stack) > 0 {
		last := len(stack) - 1
		state := stack[last]
		stack = stack[:last]

		if seen[state] {
			continue
		}

		seen[state] = true
		result = append(result, state)

		for _, edge := range machine.states[state].edges {
			if assertionEnabled(edge.kind, atStart, previousWord, previousLF, next) {
				stack = append(stack, edge.to)
			}
		}
	}

	slices.Sort(result)

	return result
}

//nolint:cyclop // Assertion kinds form a closed semantic table.
func assertionEnabled(
	kind edgeKind,
	atStart bool,
	previousWord bool,
	previousLF bool,
	next *byte,
) bool {
	atEnd := next == nil
	nextWord := !atEnd && isWordByte(*next)

	switch kind {
	case edgeEpsilon:
		return true
	case edgeBeginText:
		return atStart
	case edgeEndText:
		return atEnd
	case edgeBeginLine:
		return atStart || previousLF
	case edgeEndLine:
		return atEnd || *next == '\n'
	case edgeWordBoundary:
		return previousWord != nextWord
	case edgeNotWordBoundary:
		return previousWord == nextWord
	case edgeCharacters:
		return false
	default:
		return false
	}
}

func moveSubset(machine *nfa, current subsetState, value byte) subsetState {
	active := assertionClosure(
		machine,
		current.states,
		current.atStart,
		current.previousWord,
		current.previousLF,
		&value,
	)
	destinations := make([]int, 0)

	for _, state := range active {
		for _, edge := range machine.states[state].edges {
			if edge.kind == edgeCharacters && edge.characters.contains(value) {
				destinations = append(destinations, edge.to)
			}
		}
	}

	return subsetState{
		states:       unconditionalClosure(machine, destinations),
		previousWord: isWordByte(value),
		previousLF:   value == '\n',
	}
}

func acceptsAtEnd(machine *nfa, state subsetState) bool {
	active := assertionClosure(
		machine,
		state.states,
		state.atStart,
		state.previousWord,
		state.previousLF,
		nil,
	)

	return slices.Contains(active, machine.accept)
}

func subsetKey(state subsetState) string {
	flags := byte(0)
	if state.atStart {
		flags |= 1
	}

	if state.previousWord {
		flags |= 2
	}

	if state.previousLF {
		flags |= 4
	}

	key := []byte{flags}
	for _, item := range state.states {
		key = binary.AppendUvarint(key, uint64(item)+1)
	}

	return string(key)
}

func stateTupleKey(states []uint32) string {
	key := make([]byte, 0, len(states)*2)
	for _, state := range states {
		key = binary.AppendUvarint(key, uint64(state)+1)
	}

	return string(key)
}

func singleRuneSet(value rune) byteSet {
	set := byteSet{}
	if value >= 0 && value < asciiAlphabetSize {
		set.add(byte(value))
	}

	return set
}

func digitSet() byteSet {
	set := byteSet{}
	set.addRange('0', '9')

	return set
}

func wordSet() byteSet {
	set := digitSet()
	set.addRange('A', 'Z')
	set.addRange('a', 'z')
	set.add('_')

	return set
}

func spaceSet() byteSet {
	set := byteSet{}
	set.addRange('\t', '\r')
	set.add(' ')

	return set
}

func esClassSet(node patternsyntax.Node) byteSet {
	set := byteSet{}

	for _, item := range node.ClassItems {
		switch item.Kind {
		case patternsyntax.ClassItemRange:
			set.addRange(item.Low, item.High)
		case patternsyntax.ClassItemDigit:
			set.union(digitSet())
		case patternsyntax.ClassItemNotDigit:
			set.union(digitSet().complement())
		case patternsyntax.ClassItemSpace:
			set.union(spaceSet())
		case patternsyntax.ClassItemNotSpace:
			set.union(spaceSet().complement())
		case patternsyntax.ClassItemWord:
			set.union(wordSet())
		case patternsyntax.ClassItemNotWord:
			set.union(wordSet().complement())
		}
	}

	if node.Negated {
		return set.complement()
	}

	return set
}

func isESLookahead(kind patternsyntax.Kind) bool {
	return kind == patternsyntax.KindPositiveLookahead || kind == patternsyntax.KindNegativeLookahead
}

func isWordByte(value byte) bool {
	return value >= '0' && value <= '9' ||
		value >= 'A' && value <= 'Z' ||
		value >= 'a' && value <= 'z' ||
		value == '_'
}
