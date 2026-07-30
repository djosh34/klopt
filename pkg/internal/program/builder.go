//nolint:godoclint // Private draft graph vocabulary stays behind Builder.
package program

import (
	"fmt"
	"math/big"
	"slices"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

const (
	firstSurrogate = 0xd800
	lastSurrogate  = 0xdfff
	maximumScalar  = 0x10ffff
)

type draftTransition struct {
	kind    transitionKind
	next    NodeID
	child   NodeID
	resume  NodeID
	name    string
	ranges  []ScalarRange
	value   jsonvalue.Value
	minimum *big.Int
	maximum *big.Int
}

type draftNode struct {
	outgoing []uint32
}

// Builder incrementally constructs a mutable schema-free graph.
type Builder struct {
	nodes       []draftNode
	transitions []draftTransition
}

// AddNode adds an empty graph node.
func (builder *Builder) AddNode() NodeID {
	id := NodeID(len(builder.nodes))
	builder.nodes = append(builder.nodes, draftNode{})

	return id
}

// AddScalarRanges adds one scalar-emitting string transition.
func (builder *Builder) AddScalarRanges(from NodeID, ranges []ScalarRange, next NodeID) error {
	if err := builder.validateNodes(from, next); err != nil {
		return fmt.Errorf("add scalar transition: %w", err)
	}

	normalized, err := normalizeScalarRanges(ranges)
	if err != nil {
		return err
	}

	return builder.add(from, draftTransition{kind: transitionScalar, next: next, ranges: normalized})
}

// AddStringStop adds one transition that completes the current string.
func (builder *Builder) AddStringStop(from NodeID) error {
	if err := builder.validateNode(from); err != nil {
		return fmt.Errorf("add string stop: %w", err)
	}

	return builder.addUniqueStop(from, transitionStringStop)
}

// AddBeginString starts a JSON string and continues at next.
func (builder *Builder) AddBeginString(from NodeID, next NodeID) error {
	if err := builder.validateNodes(from, next); err != nil {
		return fmt.Errorf("add begin string: %w", err)
	}

	return builder.add(from, draftTransition{kind: transitionBeginString, next: next})
}

// AddExactValue returns one immutable exact JSON value.
func (builder *Builder) AddExactValue(from NodeID, value jsonvalue.Value) error {
	if err := builder.validateNode(from); err != nil {
		return fmt.Errorf("add exact value: %w", err)
	}

	if _, err := value.MarshalJSON(); err != nil {
		return fmt.Errorf("add exact value: %w", err)
	}

	return builder.add(from, draftTransition{kind: transitionExactValue, value: value})
}

// AddIntegerRange returns an exact integer selected without int64 narrowing.
// A nil bound means that side is unbounded.
func (builder *Builder) AddIntegerRange(from NodeID, minimum *big.Int, maximum *big.Int) error {
	if err := builder.validateNode(from); err != nil {
		return fmt.Errorf("add integer range: %w", err)
	}

	if minimum != nil && maximum != nil && minimum.Cmp(maximum) > 0 {
		return fmt.Errorf("add integer range: minimum %s exceeds maximum %s", minimum, maximum)
	}

	return builder.add(from, draftTransition{
		kind: transitionInteger, minimum: cloneBigInt(minimum), maximum: cloneBigInt(maximum),
	})
}

// AddBeginArray starts an array and continues at next.
func (builder *Builder) AddBeginArray(from NodeID, next NodeID) error {
	return builder.addContinue(from, next, transitionBeginArray, "begin array")
}

// AddBeginObject starts an object and continues at next.
func (builder *Builder) AddBeginObject(from NodeID, next NodeID) error {
	return builder.addContinue(from, next, transitionBeginObject, "begin object")
}

// AddArrayItem calls child for one array item and resumes at resume.
func (builder *Builder) AddArrayItem(from NodeID, child NodeID, resume NodeID) error {
	if err := builder.validateNodes(from, child, resume); err != nil {
		return fmt.Errorf("add array item: %w", err)
	}

	return builder.add(from, draftTransition{kind: transitionArrayItem, child: child, resume: resume})
}

// AddArraySequence returns an array whose exact count is selected from the inclusive range.
// A nil maximum means unbounded; minimum must be non-negative.
func (builder *Builder) AddArraySequence(
	from NodeID,
	child NodeID,
	minimum *big.Int,
	maximum *big.Int,
) error {
	if err := builder.validateNodes(from, child); err != nil {
		return fmt.Errorf("add array sequence: %w", err)
	}

	if minimum == nil || minimum.Sign() < 0 {
		return fmt.Errorf("add array sequence: minimum must be non-negative")
	}

	if maximum != nil && minimum.Cmp(maximum) > 0 {
		return fmt.Errorf("add array sequence: minimum %s exceeds maximum %s", minimum, maximum)
	}

	return builder.add(from, draftTransition{
		kind: transitionArraySequence, child: child,
		minimum: cloneBigInt(minimum), maximum: cloneBigInt(maximum),
	})
}

// AddObjectMember calls child for one named object member and resumes at resume.
func (builder *Builder) AddObjectMember(from NodeID, name string, child NodeID, resume NodeID) error {
	if err := builder.validateNodes(from, child, resume); err != nil {
		return fmt.Errorf("add object member: %w", err)
	}

	return builder.add(from, draftTransition{
		kind: transitionObjectMember, name: name, child: child, resume: resume,
	})
}

// AddStop closes the current array or object and returns it.
func (builder *Builder) AddStop(from NodeID) error {
	if err := builder.validateNode(from); err != nil {
		return fmt.Errorf("add stop: %w", err)
	}

	return builder.addUniqueStop(from, transitionStop)
}

// UniformSampling returns a table assigning weight one to every current transition.
func (builder *Builder) UniformSampling() SamplingTable {
	weights := make([]uint32, len(builder.transitions))
	for index := range weights {
		weights[index] = 1
	}

	return SamplingTable{Weights: weights}
}

func (builder *Builder) addContinue(from NodeID, next NodeID, kind transitionKind, label string) error {
	if err := builder.validateNodes(from, next); err != nil {
		return fmt.Errorf("add %s: %w", label, err)
	}

	return builder.add(from, draftTransition{kind: kind, next: next})
}

func (builder *Builder) addUniqueStop(from NodeID, kind transitionKind) error {
	for _, transitionID := range builder.nodes[from].outgoing {
		if builder.transitions[transitionID].kind == kind {
			return fmt.Errorf("node %d already has stop kind %d", from, kind)
		}
	}

	return builder.add(from, draftTransition{kind: kind})
}

func (builder *Builder) add(from NodeID, item draftTransition) error {
	if uint64(len(builder.transitions)) > uint64(^uint32(0)) {
		return fmt.Errorf("program has too many transitions")
	}

	id := uint32(len(builder.transitions))
	builder.transitions = append(builder.transitions, item)
	builder.nodes[from].outgoing = append(builder.nodes[from].outgoing, id)

	return nil
}

func (builder *Builder) validateNodes(ids ...NodeID) error {
	for _, id := range ids {
		if err := builder.validateNode(id); err != nil {
			return err
		}
	}

	return nil
}

func (builder *Builder) validateNode(id NodeID) error {
	if uint64(id) >= uint64(len(builder.nodes)) {
		return fmt.Errorf("node %d does not exist", id)
	}

	return nil
}

func cloneBigInt(value *big.Int) *big.Int {
	if value == nil {
		return nil
	}

	return new(big.Int).Set(value)
}

//nolint:cyclop // Validation and canonicalization are one atomic input gate.
func normalizeScalarRanges(ranges []ScalarRange) ([]ScalarRange, error) {
	if len(ranges) == 0 {
		return nil, fmt.Errorf("scalar transition has no ranges")
	}

	normalized := slices.Clone(ranges)
	for _, item := range normalized {
		if item.First < 0 || item.First > maximumScalar || item.Last < item.First || item.Last > maximumScalar {
			return nil, fmt.Errorf("invalid Unicode scalar range %#x-%#x", item.First, item.Last)
		}

		if item.First <= lastSurrogate && item.Last >= firstSurrogate {
			return nil, fmt.Errorf("unicode scalar range %#x-%#x contains surrogates", item.First, item.Last)
		}
	}

	slices.SortFunc(normalized, func(left ScalarRange, right ScalarRange) int {
		if left.First != right.First {
			return int(left.First - right.First)
		}

		return int(left.Last - right.Last)
	})

	result := normalized[:0]
	for _, item := range normalized {
		if len(result) == 0 || item.First > result[len(result)-1].Last+1 {
			result = append(result, item)

			continue
		}

		result[len(result)-1].Last = max(result[len(result)-1].Last, item.Last)
	}

	return result, nil
}
