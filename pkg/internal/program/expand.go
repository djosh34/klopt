//nolint:godoclint,mnd // Private edge weights are local sampling policy, never semantics.
package program

import "fmt"

const (
	validAlternativeWeight          = 4
	preservedSiblingFaultWeight     = 8
	unconstrainedSiblingFaultWeight = 2
	coordinatedFaultWeight          = 1
)

type productiveEdge struct {
	next   state
	weight uint64
}

func (program *Program) chooseVerdict(
	root nodeID,
	word uint64,
	work *decodeWork,
) (ExpectedResult, error) {
	verdicts := make([]ExpectedResult, 0, 2)

	for _, candidate := range []ExpectedResult{ExpectValid, ExpectInvalid} {
		possible, err := program.productive(canonicalState([]goal{{
			node: root, want: candidate == ExpectValid,
		}}), work)
		if err != nil {
			return 0, err
		}

		if possible {
			verdicts = append(verdicts, candidate)
		}
	}

	if len(verdicts) == 0 {
		return 0, fmt.Errorf("operation root has neither valid nor invalid JSON values")
	}

	return verdicts[word%uint64(len(verdicts))], nil
}

func (program *Program) expand(branching branch, work *decodeWork) ([]productiveEdge, error) {
	candidates := program.branchCandidates(branching)
	edges := make([]productiveEdge, 0, len(candidates))

	for _, candidate := range candidates {
		productive, err := program.productive(candidate.next, work)
		if err != nil {
			return nil, err
		}

		if productive {
			edges = append(edges, candidate)
		}
	}

	return edges, nil
}

func (program *Program) branchCandidates(branching branch) []productiveEdge {
	item := program.nodes[branching.goal.node]
	result := make([]productiveEdge, 0, len(item.children)*2+1)

	if item.kind == nodeOr {
		for _, child := range item.children {
			goals := appendCopy(branching.base.goals, goal{node: child, want: true})
			result = append(result, productiveEdge{
				next:   canonicalStateWithExclusions(goals, branching.base.excluded),
				weight: validAlternativeWeight,
			})
		}

		return result
	}

	for failed, child := range item.children {
		goals := appendCopy(branching.base.goals, goal{node: child, want: false})

		for sibling, siblingNode := range item.children {
			if sibling != failed {
				goals = append(goals, goal{node: siblingNode, want: true})
			}
		}

		result = append(result, productiveEdge{
			next:   canonicalStateWithExclusions(goals, branching.base.excluded),
			weight: preservedSiblingFaultWeight,
		})
	}

	for _, child := range item.children {
		goals := appendCopy(branching.base.goals, goal{node: child, want: false})
		result = append(result, productiveEdge{
			next:   canonicalStateWithExclusions(goals, branching.base.excluded),
			weight: unconstrainedSiblingFaultWeight,
		})
	}

	allFailed := appendCopy(branching.base.goals)
	for _, child := range item.children {
		allFailed = append(allFailed, goal{node: child, want: false})
	}

	result = append(result, productiveEdge{
		next:   canonicalStateWithExclusions(allFailed, branching.base.excluded),
		weight: coordinatedFaultWeight,
	})

	return result
}

//nolint:cyclop // Exact productivity keeps all error and terminal states explicit.
func (program *Program) productive(candidate state, work *decodeWork) (bool, error) {
	if len(candidate.goals) == 0 {
		return true, nil
	}

	key, err := stateKey(candidate)
	if err != nil {
		return false, err
	}

	if work.known[key] {
		return work.productive[key], nil
	}

	if err := work.solver(uint64(len(key))); err != nil {
		return false, err
	}

	terminal, branching, possible := program.normalize(candidate)
	if !possible {
		work.known[key] = true
		work.productive[key] = false

		return false, nil
	}

	if branching == nil {
		_, possible, err := program.sampleAtoms(
			terminal.goals, terminal.excluded, nil, work, 0,
		)
		if err != nil {
			return false, err
		}

		work.known[key] = true
		work.productive[key] = possible

		return possible, nil
	}

	for _, next := range program.branchCandidates(*branching) {
		childPossible, err := program.productive(next.next, work)
		if err != nil {
			return false, err
		}

		if childPossible {
			work.known[key] = true
			work.productive[key] = true

			return true, nil
		}
	}

	work.known[key] = true
	work.productive[key] = false

	return false, nil
}
