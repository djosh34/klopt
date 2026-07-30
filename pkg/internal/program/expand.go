//nolint:cyclop,godoclint,mnd // Signed branch expansion is one explicit truth table.
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
	candidates, err := program.branchCandidates(branching, work)
	if err != nil {
		return nil, err
	}

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

func (program *Program) branchCandidates(
	branching branch,
	work *decodeWork,
) ([]productiveEdge, error) {
	item := program.nodes[branching.goal.node]

	childBytes, ok := checkedMul(uint64(len(item.children)), 48)
	if !ok {
		return nil, &ResourceError{
			Resource: "branch bytes", Limit: work.limits.MaxSolverBytes, Observed: ^uint64(0),
		}
	}

	goalBytes, ok := checkedMul(uint64(len(branching.base.goals)), 16)
	if !ok {
		return nil, &ResourceError{
			Resource: "branch bytes", Limit: work.limits.MaxSolverBytes, Observed: ^uint64(0),
		}
	}

	estimated, ok := checkedAdd(childBytes, goalBytes)
	if !ok {
		return nil, &ResourceError{
			Resource: "branch bytes", Limit: work.limits.MaxSolverBytes, Observed: ^uint64(0),
		}
	}

	if err := work.chargeSolver("branch work", "branch bytes", 1, estimated); err != nil {
		return nil, err
	}

	result := make([]productiveEdge, 0, len(item.children))

	if item.kind == nodeOr {
		for _, child := range item.children {
			goals := appendCopy(branching.base.goals, goal{node: child, want: true})
			result = append(result, productiveEdge{
				next:   canonicalStateWithExclusions(goals, branching.base.excluded),
				weight: validAlternativeWeight,
			})
		}

		return result, nil
	}

	for failed, child := range item.children {
		goals := appendCopy(branching.base.goals)
		for sibling, siblingNode := range item.children {
			goals = append(goals, goal{node: siblingNode, want: sibling != failed})
		}

		baseWeight := uint64(preservedSiblingFaultWeight)
		if work.fault.budget > 1 {
			baseWeight = unconstrainedSiblingFaultWeight
		}

		weight := baseWeight *
			program.faultFrontierWeight(child, work.fault.style)
		result = append(result, productiveEdge{
			next: canonicalStateWithExclusions(goals, branching.base.excluded), weight: weight,
		})
	}

	budget := int(max(work.fault.budget, 1))

	budget = min(budget, len(item.children))
	if budget > 1 {
		for first := range item.children {
			failed := make(map[int]struct{}, budget)
			for offset := range budget {
				failed[(first+offset)%len(item.children)] = struct{}{}
			}

			goals := appendCopy(branching.base.goals)

			for index, child := range item.children {
				_, wantFalse := failed[index]
				goals = append(goals, goal{node: child, want: !wantFalse})
			}

			weight := uint64(preservedSiblingFaultWeight) *
				program.faultFrontierWeight(item.children[first], work.fault.style)
			result = append(result, productiveEdge{
				next:   canonicalStateWithExclusions(goals, branching.base.excluded),
				weight: weight,
			})
		}
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

	return result, nil
}

func (program *Program) faultFrontierWeight(identifier nodeID, style faultStyle) uint64 {
	category := program.faultCategory(identifier)

	switch style {
	case faultWrongKind:
		if category == faultCategoryKind {
			return 8
		}
	case faultStructural:
		if category == faultCategoryStructure {
			return 8
		}
	case faultBoundary:
		if category == faultCategoryBoundary {
			return 8
		}
	}

	return 1
}

type faultCategory uint8

const (
	faultCategoryBoundary faultCategory = iota
	faultCategoryKind
	faultCategoryStructure
)

func (program *Program) faultCategory(identifier nodeID) faultCategory {
	item := program.nodes[identifier]
	if item.kind != nodeAtom {
		for _, child := range item.children {
			category := program.faultCategory(child)
			if category != faultCategoryBoundary {
				return category
			}
		}

		return faultCategoryBoundary
	}

	switch item.atom.kind {
	case atomKinds:
		return faultCategoryKind
	case atomArrayMinItems, atomArrayMaxItems, atomArrayItems,
		atomObjectMinProperties, atomObjectMaxProperties, atomObjectRequired,
		atomObjectProperty, atomObjectAdditional:
		return faultCategoryStructure
	default:
		return faultCategoryBoundary
	}
}

//nolint:cyclop // Exact productivity keeps all error and terminal states explicit.
func (program *Program) productive(candidate state, work *decodeWork) (bool, error) {
	if len(candidate.goals) == 0 {
		return true, nil
	}

	keyBytes, err := stateKeyBytes(candidate)
	if err != nil {
		return false, err
	}

	if solverErr := work.solver(keyBytes); solverErr != nil {
		return false, solverErr
	}

	key, err := stateKey(candidate)
	if err != nil {
		return false, err
	}

	if work.known[key] {
		return work.productive[key], nil
	}

	terminal, branching, possible, err := program.normalize(candidate, work)
	if err != nil {
		return false, err
	}

	if !possible {
		work.known[key] = true
		work.productive[key] = false

		return false, nil
	}

	if branching == nil {
		_, possible, sampleErr := program.sampleAtoms(
			terminal.goals, terminal.excluded, nil, work, 0,
		)
		if sampleErr != nil {
			return false, sampleErr
		}

		work.known[key] = true
		work.productive[key] = possible

		return possible, nil
	}

	nextStates, err := program.branchCandidates(*branching, work)
	if err != nil {
		return false, err
	}

	for _, next := range nextStates {
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
