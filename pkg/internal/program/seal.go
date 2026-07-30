//nolint:godoclint // Private certification values stay behind Builder.Seal.
package program

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"slices"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

const unreachableCost = ^uint64(0)

// Seal validates, certifies, and immutably seals builder with one sampling table.
//
//nolint:cyclop // Sealing validates one transaction before immutable data escapes.
func (builder *Builder) Seal(root NodeID, table SamplingTable) (Program, error) {
	if err := builder.validateNode(root); err != nil {
		return Program{}, fmt.Errorf("seal root: %w", err)
	}

	if len(table.Weights) != len(builder.transitions) {
		return Program{}, fmt.Errorf(
			"sampling table has %d weights for %d transitions",
			len(table.Weights),
			len(builder.transitions),
		)
	}

	for index, weight := range table.Weights {
		if weight == 0 {
			return Program{}, fmt.Errorf("sampling weight %d is zero", index)
		}
	}

	costs := builder.completionCosts()
	if costs[root] == unreachableCost {
		return Program{}, fmt.Errorf("root node %d does not terminate", root)
	}

	reachable := builder.reachable(root)
	for nodeID, isReachable := range reachable {
		if !isReachable {
			continue
		}

		if len(builder.nodes[nodeID].outgoing) == 0 {
			return Program{}, fmt.Errorf("reachable node %d has no transitions", nodeID)
		}

		for _, transitionID := range builder.nodes[nodeID].outgoing {
			if builder.transitionCost(builder.transitions[transitionID], costs) == unreachableCost {
				return Program{}, fmt.Errorf(
					"transition %d from node %d does not terminate", transitionID, nodeID,
				)
			}
		}
	}

	nodeOrder := builder.reachableOrder(root, costs)

	nodeMap := make(map[NodeID]NodeID, len(nodeOrder))
	for newID, oldID := range nodeOrder {
		nodeMap[oldID] = NodeID(newID)
	}

	sealed := Program{
		root: 0, nodes: make([]node, len(nodeOrder)), completion: make([]completionFact, len(nodeOrder)),
	}

	for newNodeID, oldNodeID := range nodeOrder {
		ordered := builder.orderedTransitions(oldNodeID, costs)
		for _, oldTransitionID := range ordered {
			item, err := sealTransition(
				builder.transitions[oldTransitionID], table.Weights[oldTransitionID],
			)
			if err != nil {
				return Program{}, fmt.Errorf("seal transition %d: %w", oldTransitionID, err)
			}

			remapTransitionNodes(&item, nodeMap)

			newTransitionID := uint32(len(sealed.transitions))
			sealed.transitions = append(sealed.transitions, item)
			sealed.nodes[newNodeID].outgoing = append(
				sealed.nodes[newNodeID].outgoing, newTransitionID,
			)
		}

		sealed.completion[newNodeID] = completionFact{
			productive: true,
			cost:       costs[oldNodeID],
			first:      sealed.nodes[newNodeID].outgoing[0],
		}
	}

	sealed.fingerprint = sealed.hash()

	return sealed, nil
}

func remapTransitionNodes(item *transition, nodeMap map[NodeID]NodeID) {
	switch item.kind {
	case transitionScalar, transitionBeginString, transitionBeginArray, transitionBeginObject:
		item.next = nodeMap[item.next]
	case transitionArrayItem, transitionObjectMember:
		item.child = nodeMap[item.child]
		item.resume = nodeMap[item.resume]
	case transitionArraySequence:
		item.child = nodeMap[item.child]
	}
}

func sealTransition(draft draftTransition, weight uint32) (transition, error) {
	item := transition{
		kind: draft.kind, next: draft.next, child: draft.child, resume: draft.resume,
		name: draft.name, ranges: append([]ScalarRange(nil), draft.ranges...),
		minimum: cloneBigInt(draft.minimum), maximum: cloneBigInt(draft.maximum), weight: weight,
	}

	if draft.kind == transitionExactValue {
		encoded, err := draft.value.MarshalJSON()
		if err != nil {
			return transition{}, fmt.Errorf("encode exact value: %w", err)
		}

		item.valueJSON = append([]byte(nil), encoded...)
		item.valueDepth = valueDepth(draft.value)
	}

	if draft.kind == transitionObjectMember {
		encoded, err := json.Marshal(draft.name)
		if err != nil {
			return transition{}, fmt.Errorf("encode object member name: %w", err)
		}

		item.nameJSON = encoded
	}

	return item, nil
}

func valueDepth(value jsonvalue.Value) uint64 {
	depth := uint64(1)

	switch value.Kind {
	case jsonvalue.KindArray:
		for _, child := range value.Array {
			depth = max(depth, 1+valueDepth(child))
		}
	case jsonvalue.KindObject:
		for _, member := range value.Object {
			depth = max(depth, 1+valueDepth(member.Value))
		}
	}

	return depth
}

func (builder *Builder) completionCosts() []uint64 {
	costs := make([]uint64, len(builder.nodes))
	for index := range costs {
		costs[index] = unreachableCost
	}

	for range len(builder.nodes) {
		changed := false

		for nodeID, item := range builder.nodes {
			best := costs[nodeID]

			for _, transitionID := range item.outgoing {
				candidate := builder.transitionCost(builder.transitions[transitionID], costs)
				if candidate < best {
					best = candidate
				}
			}

			if best != costs[nodeID] {
				costs[nodeID] = best
				changed = true
			}
		}

		if !changed {
			break
		}
	}

	return costs
}

//nolint:cyclop,mnd // The closed action vocabulary has one explicit cost equation per action.
func (builder *Builder) transitionCost(item draftTransition, costs []uint64) uint64 {
	switch item.kind {
	case transitionStringStop, transitionExactValue, transitionInteger, transitionStop:
		return 1
	case transitionScalar, transitionBeginString, transitionBeginArray, transitionBeginObject:
		return addCosts(1, costs[item.next])
	case transitionArrayItem, transitionObjectMember:
		return addCosts(1, costs[item.child], costs[item.resume])
	case transitionArraySequence:
		if item.minimum.Sign() == 0 {
			return 1
		}

		if costs[item.child] == unreachableCost || !item.minimum.IsUint64() {
			return unreachableCost
		}

		count := item.minimum.Uint64()

		perItem := addCosts(costs[item.child], 1)
		if perItem == unreachableCost || count > (^uint64(0)-2)/perItem {
			return unreachableCost
		}

		return 1 + count*perItem - 1
	default:
		return unreachableCost
	}
}

func addCosts(values ...uint64) uint64 {
	result := uint64(0)
	for _, value := range values {
		if value == unreachableCost || value > ^uint64(0)-result {
			return unreachableCost
		}

		result += value
	}

	return result
}

func (builder *Builder) reachable(root NodeID) []bool {
	reachable := make([]bool, len(builder.nodes))
	stack := []NodeID{root}

	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]

		if reachable[current] {
			continue
		}

		reachable[current] = true
		for _, transitionID := range builder.nodes[current].outgoing {
			item := builder.transitions[transitionID]

			switch item.kind {
			case transitionScalar, transitionBeginString, transitionBeginArray, transitionBeginObject:
				stack = append(stack, item.next)
			case transitionArrayItem, transitionObjectMember:
				stack = append(stack, item.child, item.resume)
			case transitionArraySequence:
				stack = append(stack, item.child)
			}
		}
	}

	return reachable
}

func (builder *Builder) orderedTransitions(nodeID NodeID, costs []uint64) []uint32 {
	ordered := append([]uint32(nil), builder.nodes[nodeID].outgoing...)
	slices.SortFunc(ordered, func(left uint32, right uint32) int {
		if builder.transitionLess(left, right, costs) {
			return -1
		}

		if builder.transitionLess(right, left, costs) {
			return 1
		}

		return 0
	})

	return ordered
}

func (builder *Builder) reachableOrder(root NodeID, costs []uint64) []NodeID {
	result := make([]NodeID, 0)
	seen := make(map[NodeID]struct{})
	queue := []NodeID{root}

	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]

		if _, ok := seen[current]; ok {
			continue
		}

		seen[current] = struct{}{}
		result = append(result, current)

		for _, transitionID := range builder.orderedTransitions(current, costs) {
			item := builder.transitions[transitionID]
			switch item.kind {
			case transitionScalar, transitionBeginString, transitionBeginArray, transitionBeginObject:
				queue = append(queue, item.next)
			case transitionArrayItem, transitionObjectMember:
				queue = append(queue, item.child, item.resume)
			case transitionArraySequence:
				queue = append(queue, item.child)
			}
		}
	}

	return result
}

//nolint:cyclop // Canonical ordering compares each closed transition payload explicitly.
func (builder *Builder) transitionLess(leftID uint32, rightID uint32, costs []uint64) bool {
	left := builder.transitions[leftID]
	right := builder.transitions[rightID]
	leftCost := builder.transitionCost(left, costs)
	rightCost := builder.transitionCost(right, costs)

	if leftCost != rightCost {
		return leftCost < rightCost
	}

	if left.kind != right.kind {
		return left.kind < right.kind
	}

	if left.kind == transitionScalar && left.ranges[0].First != right.ranges[0].First {
		return left.ranges[0].First < right.ranges[0].First
	}

	if left.kind == transitionObjectMember && left.name != right.name {
		return left.name < right.name
	}

	if left.kind == transitionExactValue {
		leftJSON, leftErr := left.value.MarshalJSON()

		rightJSON, rightErr := right.value.MarshalJSON()
		if leftErr == nil && rightErr == nil {
			comparison := slices.Compare(leftJSON, rightJSON)
			if comparison != 0 {
				return comparison < 0
			}
		}
	}

	return leftID < rightID
}

func (program *Program) hash() [32]byte {
	encoded := binary.AppendUvarint(nil, uint64(program.root))
	encoded = binary.AppendUvarint(encoded, uint64(len(program.nodes)))

	for _, item := range program.nodes {
		encoded = binary.AppendUvarint(encoded, uint64(len(item.outgoing)))
		for _, transitionID := range item.outgoing {
			encoded = binary.AppendUvarint(encoded, uint64(transitionID))
		}
	}

	encoded = binary.AppendUvarint(encoded, uint64(len(program.transitions)))
	for _, item := range program.transitions {
		encoded = append(encoded, byte(item.kind))
		encoded = binary.AppendUvarint(encoded, uint64(item.next))
		encoded = binary.AppendUvarint(encoded, uint64(item.child))
		encoded = binary.AppendUvarint(encoded, uint64(item.resume))
		encoded = binary.AppendUvarint(encoded, uint64(item.weight))
		encoded = appendBytes(encoded, []byte(item.name))
		encoded = appendBytes(encoded, item.valueJSON)

		encoded = binary.AppendUvarint(encoded, uint64(len(item.ranges)))
		for _, scalarRange := range item.ranges {
			encoded = binary.AppendUvarint(encoded, uint64(scalarRange.First))
			encoded = binary.AppendUvarint(encoded, uint64(scalarRange.Last))
		}

		encoded = appendBigInt(encoded, item.minimum)
		encoded = appendBigInt(encoded, item.maximum)
	}

	return sha256.Sum256(encoded)
}

func appendBytes(destination []byte, value []byte) []byte {
	destination = binary.AppendUvarint(destination, uint64(len(value)))

	return append(destination, value...)
}

func appendBigInt(destination []byte, value *big.Int) []byte {
	if value == nil {
		return append(destination, 0)
	}

	destination = append(destination, 1)

	return appendBytes(destination, []byte(value.String()))
}
