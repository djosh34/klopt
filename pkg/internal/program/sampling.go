package program

import "fmt"

// SemanticSampling derives one deterministic positive table from certified graph facts.
// It cannot add, remove, or redirect a transition.
//
//nolint:cyclop,mnd // The closed action vocabulary maps directly to small static weights.
func (builder *Builder) SemanticSampling(root NodeID) (SamplingTable, error) {
	if err := builder.validateNode(root); err != nil {
		return SamplingTable{}, fmt.Errorf("compile sampling: %w", err)
	}

	costs := builder.completionCosts()
	if costs[root] == unreachableCost {
		return SamplingTable{}, fmt.Errorf("compile sampling: root node %d is not productive", root)
	}

	weights := make([]uint32, len(builder.transitions))
	for identifier, item := range builder.transitions {
		cost := builder.transitionCost(item, costs)
		if cost == unreachableCost {
			return SamplingTable{}, fmt.Errorf("compile sampling: transition %d is not productive", identifier)
		}

		weight := uint32(1)

		switch item.kind {
		case transitionBeginArray, transitionBeginObject, transitionBeginString,
			transitionArraySequence:
			weight++
		case transitionInteger:
			weight += 2
		case transitionExactValue:
			if item.value.Kind == 0 || item.value.Kind >= 4 {
				weight++
			}
		}

		if cost > 1 {
			weight += uint32(min(cost-1, 3))
		}

		weights[identifier] = weight
	}

	return SamplingTable{Weights: weights}, nil
}
