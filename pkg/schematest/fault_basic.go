package schematest

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// errFaultNotFound leaves a planned fault without an isolated derivative uncovered.
var errFaultNotFound = errors.New("schematest: planned fault has no isolated derivative")

const (
	// faultParentDimension selects the valid-parent rank.
	faultParentDimension = iota
	// faultClosureDimension selects the declared failure closure.
	faultClosureDimension
	// faultOccurrenceDimension selects one concrete instance path.
	faultOccurrenceDimension
	// faultMutationDimension selects one mutation alternative.
	faultMutationDimension
	// faultProductDimensions is the fixed fault-product arity.
	faultProductDimensions
)

// streamFaults visits fault programs in their deterministic execution order.
func streamFaults(
	plan *searchPlan,
	s *search,
	covered map[string]bool,
	yield func(Case) error,
) (StopReason, error) {
	for _, index := range plan.faultExecution {
		if index < 0 || index >= len(plan.faultSchedule) {
			return "", errors.New("schematest: invalid fault execution order")
		}

		if err := streamFault(plan, plan.faultSchedule[index], s, covered, yield); err != nil {
			if errors.Is(err, errMaxSteps) {
				return MaxStepsReached, nil
			}

			return "", err
		}
	}

	return SpaceExhausted, nil
}

// faultSearchMachines owns the directly addressed row machines used by the sole fault continuation.
type faultSearchMachines struct {
	search                 *search
	parentRows             parentRowMachine
	scalarCandidates       scalarCandidateMachine
	compositionAssignments *compositionAssignmentMachine
}

// newFaultSearchMachines gives the outer continuation sole ownership of its live row machines.
func newFaultSearchMachines(s *search) *faultSearchMachines {
	return &faultSearchMachines{
		search:                 s,
		parentRows:             parentRowMachine{search: s},
		scalarCandidates:       scalarCandidateMachine{search: s},
		compositionAssignments: &compositionAssignmentMachine{search: s},
	}
}

// parentAtRank resumes parent replay through the row machine owned by the continuation.
func (machines faultSearchMachines) parentAtRank(
	plan *searchPlan,
	fault faultProgram,
	rank uint64,
) (*jsonValue, bool, bool, error) {
	return machines.directParentAtRank(plan, fault, rank)
}

// directParentAtRank decodes one aggregate mask/row coordinate without a nested replay.
//
//nolint:cyclop // Mask, enum, and finite scalar endpoints share one coordinate advance.
func (machines faultSearchMachines) directParentAtRank(
	plan *searchPlan,
	fault faultProgram,
	rank uint64,
) (*jsonValue, bool, bool, error) {
	if plan == nil {
		return nil, false, false, errors.New("schematest: nil search plan")
	}

	if machines.search == nil || machines.search.model == nil || machines.search.model.root == nil {
		return nil, false, false, errors.New("schematest: parent replay has no model")
	}

	groups := parentReplayGroups(fault.requirements)

	decoder, ok := newDirectRankTupleDecoder(len(groups)+1, rank)
	if !ok {
		return nil, false, false, nil
	}

	maskRanks := make([]uint64, len(groups))
	for index, group := range groups {
		maskRank, exists := decoder.Next()
		if !exists {
			return nil, false, false, errors.New("schematest: parent mask rank ended early")
		}

		if _, maskExists := parentReplayMaskAtOrdinal(
			group.count, new(big.Int).SetUint64(maskRank),
		); !maskExists {
			return nil, false, false, nil
		}

		maskRanks[index] = maskRank
	}

	rowRank, exists := decoder.Next()
	if !exists {
		return nil, false, false, errors.New("schematest: parent row rank ended early")
	}

	requirements := parentReplayRequirementsAt(fault, groups, maskRanks)

	parent, found, err := machines.parentRows.CandidateAtRank(requirements, rowRank)
	if err != nil || found || parent != nil {
		return parent, found, false, err
	}

	firstEnumRank := uint64(0)
	if _, hasEnum := activeEnumValueAtRank(
		machines.search.model.root,
		machines.search.model.root.occurrence,
		requirements,
		&firstEnumRank,
	); hasEnum {
		requested := rank
		_, rankExists := activeEnumValueAtRank(
			machines.search.model.root,
			machines.search.model.root.occurrence,
			requirements,
			&requested,
		)

		return nil, false, !rankExists, nil
	}

	exhausted, err := finiteScalarFaultRows(
		machines.search.model.root,
		machines.search.model.root.occurrence,
		requirements,
	)

	return nil, false, exhausted, err
}

// faultProductExhaustion retains conditional finite endpoints for reachable nested cursors.
type faultProductExhaustion struct {
	closureFinite bool
	closureSize   uint64
	closures      map[uint64]*faultClosureExhaustion
}

// faultClosureExhaustion owns one closure-local parent endpoint.
type faultClosureExhaustion struct {
	parentFinite bool
	parentSize   uint64
	parents      map[uint64]*faultParentExhaustion
}

// faultParentExhaustion owns one parent-local occurrence endpoint.
type faultParentExhaustion struct {
	occurrenceFinite bool
	occurrenceSize   uint64
	occurrences      map[uint64]*faultOccurrenceExhaustion
}

// faultOccurrenceExhaustion owns one occurrence-local mutation endpoint.
type faultOccurrenceExhaustion struct {
	mutationFinite         bool
	mutationSize           uint64
	compositionAssignments *compositionAssignmentMachine
}

// newFaultProductExhaustion starts empty conditional endpoint state.
func newFaultProductExhaustion() *faultProductExhaustion {
	return &faultProductExhaustion{closures: make(map[uint64]*faultClosureExhaustion)}
}

// closure returns the endpoint state for one reachable closure rank.
func (state *faultProductExhaustion) closure(rank uint64) *faultClosureExhaustion {
	closure := state.closures[rank]
	if closure == nil {
		closure = &faultClosureExhaustion{parents: make(map[uint64]*faultParentExhaustion)}
		state.closures[rank] = closure
	}

	return closure
}

// parent returns the endpoint state for one reachable parent rank.
func (state *faultProductExhaustion) parent(
	closureRank uint64,
	parentRank uint64,
) *faultParentExhaustion {
	closure := state.closure(closureRank)

	parent := closure.parents[parentRank]
	if parent == nil {
		parent = &faultParentExhaustion{
			occurrences: make(map[uint64]*faultOccurrenceExhaustion),
		}
		closure.parents[parentRank] = parent
	}

	return parent
}

// occurrence returns the endpoint state for one reachable occurrence rank.
func (state *faultProductExhaustion) occurrence(
	closureRank uint64,
	parentRank uint64,
	occurrenceRank uint64,
) *faultOccurrenceExhaustion {
	parent := state.parent(closureRank, parentRank)

	occurrence := parent.occurrences[occurrenceRank]
	if occurrence == nil {
		occurrence = new(faultOccurrenceExhaustion)
		parent.occurrences[occurrenceRank] = occurrence
	}

	return occurrence
}

// setFaultFiniteEndpoint retains the smallest proven finite upper bound.
func setFaultFiniteEndpoint(finite *bool, current *uint64, size uint64) {
	if !*finite || size < *current {
		*current = size
	}

	*finite = true
}

// complete reports whether every reachable nested finite cursor is exhausted.
//
//nolint:cyclop // Four conditional cursor levels are checked in one proof.
func (state *faultProductExhaustion) complete() bool {
	if !state.closureFinite || uint64(len(state.closures)) != state.closureSize {
		return false
	}

	for _, closure := range state.closures {
		if !closure.parentFinite {
			return false
		}

		for parentRank, parent := range closure.parents {
			if parentRank >= closure.parentSize {
				continue
			}

			if !parent.occurrenceFinite {
				return false
			}

			for occurrenceRank, occurrence := range parent.occurrences {
				if occurrenceRank < parent.occurrenceSize && !occurrence.mutationFinite {
					return false
				}
			}
		}
	}

	return true
}

// streamFault is the sole continuation over parent, closure, occurrence, and mutation ranks.
//
//nolint:cyclop,gocognit,gocyclo // Conditional product exhaustion and exact verification meet here.
func streamFault(
	plan *searchPlan,
	fault faultProgram,
	s *search,
	covered map[string]bool,
	yield func(Case) error,
) error {
	product, err := newRankProductCursor(faultProductDimensions)
	if err != nil {
		return err
	}

	machines := newFaultSearchMachines(s)
	exhaustion := newFaultProductExhaustion()

	var (
		diagonal        uint64
		diagonalStarted bool
	)

	for {
		ranks, ok := product.Next()
		if !ok {
			return nil
		}

		if !diagonalStarted || product.diagonal != diagonal {
			if diagonalStarted && exhaustion.complete() {
				return nil
			}

			diagonal = product.diagonal
			diagonalStarted = true
		}

		selectedFault, closureExists, closureExhausted, closureErr := faultClosureAtRank(
			fault, ranks[faultClosureDimension], s,
		)
		if closureErr != nil {
			return closureErr
		}

		if closureExhausted {
			closureSize := ranks[faultClosureDimension]
			if err := product.SetFinite(faultClosureDimension, closureSize); err != nil {
				return err
			}

			setFaultFiniteEndpoint(
				&exhaustion.closureFinite, &exhaustion.closureSize, closureSize,
			)

			continue
		}

		if !closureExists {
			continue
		}

		closureRank := ranks[faultClosureDimension]
		exhaustion.closure(closureRank)

		parentRank := ranks[faultParentDimension]

		parent, found, exhausted, replayErr := machines.parentAtRank(
			plan, selectedFault, ranks[faultParentDimension],
		)
		if replayErr != nil {
			return replayErr
		}

		if exhausted {
			closure := exhaustion.closure(closureRank)
			setFaultFiniteEndpoint(&closure.parentFinite, &closure.parentSize, parentRank)

			continue
		}

		if !found {
			continue
		}

		closure := exhaustion.closure(closureRank)

		trackedParent := !closure.parentFinite || parentRank < closure.parentSize
		if trackedParent {
			exhaustion.parent(closureRank, parentRank)
		}

		occurrenceRank := ranks[faultOccurrenceDimension]
		mutationRank := ranks[faultMutationDimension]

		compositionAssignments := machines.compositionAssignments

		if trackedParent {
			parentState := exhaustion.parent(closureRank, parentRank)
			if !parentState.occurrenceFinite || occurrenceRank < parentState.occurrenceSize {
				occurrenceState := exhaustion.occurrence(closureRank, parentRank, occurrenceRank)
				if occurrenceState.compositionAssignments == nil {
					occurrenceState.compositionAssignments = &compositionAssignmentMachine{search: s}
				}

				compositionAssignments = occurrenceState.compositionAssignments
			}
		}

		derivative, attempted, occurrenceExhausted, mutationExhausted, faultErr := machines.applyFaultAtRank(
			parent,
			selectedFault,
			ranks[faultOccurrenceDimension],
			ranks[faultMutationDimension],
			compositionAssignments,
		)
		if faultErr != nil {
			return faultErr
		}

		if occurrenceExhausted {
			if trackedParent {
				parentState := exhaustion.parent(closureRank, parentRank)
				setFaultFiniteEndpoint(
					&parentState.occurrenceFinite, &parentState.occurrenceSize, occurrenceRank,
				)
			}

			continue
		}

		trackedOccurrence := false

		if trackedParent {
			parentState := exhaustion.parent(closureRank, parentRank)

			trackedOccurrence = !parentState.occurrenceFinite ||
				occurrenceRank < parentState.occurrenceSize
			if trackedOccurrence {
				exhaustion.occurrence(closureRank, parentRank, occurrenceRank)
			}
		}

		if mutationExhausted {
			if trackedOccurrence {
				occurrenceState := exhaustion.occurrence(closureRank, parentRank, occurrenceRank)
				setFaultFiniteEndpoint(
					&occurrenceState.mutationFinite, &occurrenceState.mutationSize, mutationRank,
				)
			}

			continue
		}

		if !attempted || derivative == nil {
			continue
		}

		result := evaluate(s.model, derivative)
		if result.err != nil {
			return fmt.Errorf("evaluate fault derivative: %w", result.err)
		}

		if result.valid {
			return errors.New("schematest: fault adapter returned a valid derivative")
		}

		encoded, marshalErr := marshalStrict(derivative)
		if marshalErr != nil {
			return fmt.Errorf("serialize fault derivative: %w", marshalErr)
		}

		covered[fault.obligation.String()] = true

		return yield(Case{JSON: encoded, Valid: false})
	}
}

// faultClosureAtRank selects one complete declarative closure without storing
// the closure product. Each selected branch-local alternative is charged only
// after the requested complete rank is known.
func faultClosureAtRank(
	fault faultProgram,
	rank uint64,
	s *search,
) (faultProgram, bool, bool, error) {
	if fault.alternatives == nil {
		if rank > 0 {
			return faultProgram{}, false, true, nil
		}

		return fault, true, false, nil
	}

	selected := fault
	selected.alternatives = nil
	selected.requirements = copyPlanRequirements(fault.requirements)
	selected.expected = append(faultClosure(nil), fault.expected...)

	remaining := new(big.Int).SetUint64(rank)

	found, err := selectFaultClosure(
		fault.alternatives, remaining, &selected, s,
	)
	if err != nil {
		return faultProgram{}, false, false, err
	}

	return selected, found, !found, nil
}

// selectFaultClosure decodes one rank from compositional subtree sizes. Counting
// visits each authored program once and never enumerates completed tuples.
func selectFaultClosure(
	program *faultClosureProgram,
	rank *big.Int,
	selected *faultProgram,
	s *search,
) (bool, error) {
	counts := make(map[*faultClosureProgram]*big.Int)
	if rank.Cmp(faultClosureProgramCount(program, counts)) >= 0 {
		return false, nil
	}

	return decodeFaultClosureProgram(program, rank, selected, s, counts)
}

// decodeFaultClosureProgram selects only the requested declarative path.
func decodeFaultClosureProgram(
	program *faultClosureProgram,
	rank *big.Int,
	selected *faultProgram,
	s *search,
	counts map[*faultClosureProgram]*big.Int,
) (bool, error) {
	if program == nil {
		return rank.Sign() == 0, nil
	}

	nextCount := faultClosureProgramCount(program.next, counts)
	for alternative := program.alternatives; alternative != nil; alternative = alternative.next {
		closureCount := faultClosureProgramCount(alternative.closure, counts)

		block := new(big.Int).Mul(closureCount, nextCount)
		if rank.Cmp(block) >= 0 {
			rank.Sub(rank, block)

			continue
		}

		if err := s.assign(); err != nil {
			return false, err
		}

		selected.requirements = appendPlanRequirements(
			selected.requirements, alternative.requirements...,
		)
		selected.expected = append(selected.expected, alternative.expected...)

		closureRank := new(big.Int).Quo(new(big.Int).Set(rank), nextCount)
		nextRank := new(big.Int).Mod(new(big.Int).Set(rank), nextCount)

		found, err := decodeFaultClosureProgram(
			alternative.closure, closureRank, selected, s, counts,
		)
		if err != nil || !found {
			return found, err
		}

		return decodeFaultClosureProgram(program.next, nextRank, selected, s, counts)
	}

	return false, nil
}

// faultClosureProgramCount computes a bounded compositional cardinality.
func faultClosureProgramCount(
	program *faultClosureProgram,
	counts map[*faultClosureProgram]*big.Int,
) *big.Int {
	if program == nil {
		return big.NewInt(1)
	}

	if count, exists := counts[program]; exists {
		return count
	}

	alternatives := new(big.Int)
	for alternative := program.alternatives; alternative != nil; alternative = alternative.next {
		alternatives.Add(alternatives, faultClosureProgramCount(alternative.closure, counts))
	}

	count := new(big.Int).Mul(alternatives, faultClosureProgramCount(program.next, counts))
	counts[program] = count

	return count
}

// parentReplayGroup identifies one authored anyOf truth vector.
type parentReplayGroup struct {
	indexes []int
	count   int
}

// regenerateParent replays the first fresh, complete oracle-valid parent.
func regenerateParent(plan *searchPlan, fault faultProgram, s *search) (*jsonValue, bool, error) {
	machines := newFaultSearchMachines(s)

	for rank := uint64(0); ; rank++ {
		parent, found, _, err := machines.directParentAtRank(plan, fault, rank)
		if err != nil || found {
			return parent, found, err
		}

		if rank == ^uint64(0) {
			return nil, false, errors.New("schematest: parent rank overflow")
		}
	}
}

// regenerateParentAtRank fairly addresses one valid parent across nonempty
// anyOf masks and complete-row ranks. It retains no generated parent corpus.
func regenerateParentAtRank(
	plan *searchPlan,
	fault faultProgram,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	return regenerateParentAtRankWithMachine(
		plan, fault, rank, parentRowMachine{search: s},
	)
}

// regenerateParentAtRankWithMachine addresses one valid parent through an owned row machine.
//
//nolint:cyclop // Finite mask setup and diagonal exhaustion share the replay cursor boundary.
func regenerateParentAtRankWithMachine(
	plan *searchPlan,
	fault faultProgram,
	rank uint64,
	rows parentRowMachine,
) (*jsonValue, bool, bool, error) {
	s := rows.search

	if plan == nil {
		return nil, false, false, errors.New("schematest: nil search plan")
	}

	if s == nil || s.model == nil || s.model.root == nil {
		return nil, false, false, errors.New("schematest: parent replay has no model")
	}

	groups := parentReplayGroups(fault.requirements)

	frontier, err := newRankProductCursor(len(groups) + 1)
	if err != nil {
		return nil, false, false, err
	}

	maximumMaskDiagonal := uint64(0)
	allMasksFinite := true

	for index, group := range groups {
		size, finite := parentReplayMaskFiniteSize(group.count)
		if !finite {
			allMasksFinite = false

			continue
		}

		if err := frontier.SetFinite(index, size); err != nil {
			return nil, false, false, err
		}

		addend := size - 1
		if ^uint64(0)-maximumMaskDiagonal < addend {
			maximumMaskDiagonal = ^uint64(0)
		} else {
			maximumMaskDiagonal += addend
		}
	}

	var (
		observed        uint64
		diagonal        uint64
		diagonalLive    bool
		diagonalStarted bool
	)

	for {
		ranks, ok := frontier.Next()
		if !ok {
			return nil, false, true, nil
		}

		if !diagonalStarted || frontier.diagonal != diagonal {
			if diagonalStarted && allMasksFinite && diagonal > maximumMaskDiagonal && !diagonalLive {
				return nil, false, true, nil
			}

			diagonal = frontier.diagonal
			diagonalLive = false
			diagonalStarted = true
		}

		requirements := parentReplayRequirementsAt(fault, groups, ranks[:len(groups)])

		parent, found, replayErr := rows.CandidateAtRank(
			requirements, ranks[len(ranks)-1],
		)
		if replayErr != nil {
			return nil, false, false, replayErr
		}

		if parent != nil {
			diagonalLive = true
		}

		if !found {
			continue
		}

		diagonalLive = true

		if observed == rank {
			return parent, true, false, nil
		}

		observed++
	}
}

// faultCandidateRanksAtOrdinal keeps the common first-projection/first-path stream dense
// while reserving every fourth rank for the complete direct tuple product.
func faultCandidateRanksAtOrdinal(wanted uint64) (uint64, uint64, uint64, bool) {
	const directTupleInterval = 4

	if wanted%directTupleInterval != directTupleInterval-1 {
		return 0, wanted - wanted/directTupleInterval, 0, true
	}

	decoder, ok := newDirectRankTupleDecoder(faultCandidateRankDimensions, wanted/directTupleInterval)
	if !ok {
		return 0, 0, 0, false
	}

	projectionRank, projectionExists := decoder.Next()
	valueRank, valueExists := decoder.Next()
	tailRank, tailExists := decoder.Next()

	return projectionRank, valueRank, tailRank,
		projectionExists && valueExists && tailExists
}

// faultRowAt directly decodes one projection/value coordinate without a row prefix walk.
func faultRowAt(
	s *search,
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	context rowSearchContext,
	projectionRank uint64,
	valueRank uint64,
) (*jsonValue, bool, bool, uint64, error) {
	view, exists, err := rowProjectionAt(node, occurrence, requirements, projectionRank)
	if err != nil {
		return nil, false, false, 0, err
	}

	active := append([]requirement(nil), requirements...)
	if exists {
		active, err = view.appendBranchRequirements(active, s.assign)
		if err != nil {
			return nil, false, false, 0, err
		}
	} else if projectionRank != 0 {
		return nil, false, false, 0, nil
	}

	return s.rowConjunctionValueAt(
		rowSchemaConjunction{sources: []rowSchemaSource{{
			node: node, occurrence: occurrence,
		}}},
		active,
		context,
		valueRank,
	)
}

// parentRowMachine directly addresses one complete row without replaying earlier rows.
type parentRowMachine struct {
	search *search
}

// CandidateAtRank evaluates one row coordinate and then yields to the fault product.
//
//nolint:cyclop // Enum, projection, row, and oracle checks form one machine advance.
func (machine parentRowMachine) CandidateAtRank(
	requirements []requirement,
	rank uint64,
) (*jsonValue, bool, error) {
	if machine.search == nil || machine.search.model == nil || machine.search.model.root == nil {
		return nil, false, errors.New("schematest: parent row machine has no model")
	}

	if candidate, exists := activeEnumValueAtRank(
		machine.search.model.root,
		machine.search.model.root.occurrence,
		requirements,
		new(uint64),
	); exists && candidate != nil {
		return parentEnumCandidateAtRawRank(machine.search, requirements, rank)
	}

	decoder, ok := newDirectRankTupleDecoder(directRowRankDimensions, rank)
	if !ok {
		return nil, false, nil
	}

	projectionRank, _ := decoder.Next()
	valueRank, _ := decoder.Next()

	view, exists, err := rowProjectionAt(
		machine.search.model.root,
		machine.search.model.root.occurrence,
		requirements,
		projectionRank,
	)
	if err != nil || !exists {
		return nil, false, err
	}

	active, err := view.appendBranchRequirements(
		append([]requirement(nil), requirements...), machine.search.assign,
	)
	if err != nil {
		return nil, false, err
	}

	candidate, exists, usable, _, err := machine.search.rowConjunctionValueAt(
		rowSchemaConjunction{sources: view.sources}, active, rowSearchContext{}, valueRank,
	)
	if err != nil {
		return nil, false, err
	}

	if (!exists || !usable || candidate == nil) &&
		(machine.search.model.root.kind == schemaAny ||
			machine.search.model.root.kind == schemaArray ||
			machine.search.model.root.kind == schemaObject) {
		candidate, exists, usable, _, err = faultRowAt(
			machine.search,
			machine.search.model.root,
			machine.search.model.root.occurrence,
			requirements,
			rowSearchContext{},
			projectionRank,
			valueRank,
		)
		if err != nil || !exists || !usable || candidate == nil {
			return nil, false, err
		}
	}

	if !exists || !usable || candidate == nil {
		return nil, false, nil
	}

	result := evaluate(machine.search.model, candidate)
	if result.err != nil {
		return nil, false, fmt.Errorf("evaluate regenerated parent: %w", result.err)
	}

	if !result.valid || !requirementsMatch(result, candidate, requirements) {
		return nil, false, nil
	}

	return candidate, true, nil
}

// parentCandidateAtRank is the standalone direct parent-row adapter.
func parentCandidateAtRank(s *search, requirements []requirement, rank uint64) (*jsonValue, bool, error) {
	return (parentRowMachine{search: s}).CandidateAtRank(requirements, rank)
}

// parentEnumCandidateAtRawRank directly selects one finite same-instance enum coordinate.
func parentEnumCandidateAtRawRank(
	s *search,
	requirements []requirement,
	rank uint64,
) (*jsonValue, bool, error) {
	wanted := rank

	candidate, exists := activeEnumValueAtRank(
		s.model.root, s.model.root.occurrence, requirements, &wanted,
	)
	if !exists {
		return nil, false, nil
	}

	if err := s.assign(); err != nil {
		return nil, false, err
	}

	owned, err := cloneJSONValue(candidate)
	if err != nil {
		return nil, false, err
	}

	result := evaluate(s.model, owned)
	if result.err != nil {
		return nil, false, fmt.Errorf("evaluate regenerated enum parent: %w", result.err)
	}

	if !result.valid || !requirementsMatch(result, owned, requirements) {
		return nil, false, nil
	}

	return owned, true, nil
}

// parentEnumCandidateAtRank replays the finite same-instance enum conjunction.
func parentEnumCandidateAtRank(
	s *search,
	requirements []requirement,
	rank uint64,
) (*jsonValue, bool, error) {
	var observed uint64

	for candidateRank := uint64(0); ; candidateRank++ {
		wanted := candidateRank

		candidate, exists := activeEnumValueAtRank(
			s.model.root, s.model.root.occurrence, requirements, &wanted,
		)
		if !exists {
			return nil, false, nil
		}

		if err := s.assign(); err != nil {
			return nil, false, err
		}

		owned, err := cloneJSONValue(candidate)
		if err != nil {
			return nil, false, err
		}

		result := evaluate(s.model, owned)
		if result.err != nil {
			return nil, false, fmt.Errorf("evaluate regenerated enum parent: %w", result.err)
		}

		if !result.valid || !requirementsMatch(result, owned, requirements) {
			continue
		}

		if observed == rank {
			return owned, true, nil
		}

		observed++

		if candidateRank == ^uint64(0) {
			return nil, false, errors.New("schematest: enum parent rank overflow")
		}
	}
}

// parentReplayRequirements turns mutation-result requirements into one valid-parent state.
func parentReplayRequirements(fault faultProgram) []requirement {
	groups := parentReplayGroups(fault.requirements)

	return parentReplayRequirementsAt(fault, groups, make([]uint64, len(groups)))
}

// parentReplayRequirementsAt converts one fault program to one nonempty anyOf parent state.
//
//nolint:cyclop // Presence, type, enum, and composition faults translate distinct dimensions.
func parentReplayRequirementsAt(
	fault faultProgram,
	groups []parentReplayGroup,
	maskRanks []uint64,
) []requirement {
	requirements := copyPlanRequirements(fault.requirements)
	for index := range requirements {
		if requirements[index].hasBranch && requirements[index].composition == "allOf" {
			requirements[index].truth = true
		}

		for _, expected := range fault.expected {
			failure := expected.project()
			if !instanceTemplateMatches(
				failure.occurrence.instanceTemplate,
				requirements[index].occurrence.instanceTemplate,
			) {
				continue
			}

			switch failure.rule {
			case oracleRuleRequired:
				requirements[index].presence = requirementPresent
			case oracleRuleAdditionalProperties:
				requirements[index].presence = requirementNoPresence
			case oracleRuleType:
				requirements[index].hasKind = false
			case oracleRuleEnum:
				if rowOccurrenceMatches(requirements[index].occurrence, failure.occurrence) {
					requirements[index].hasKind = false
				}
			}
		}
	}

	for groupIndex, group := range groups {
		mask, exists := parentReplayMaskAtOrdinal(
			group.count, new(big.Int).SetUint64(maskRanks[groupIndex]),
		)
		if !exists {
			continue
		}

		for _, requirementIndex := range group.indexes {
			branch := requirements[requirementIndex].branch
			requirements[requirementIndex].truth = mask.Bit(branch) == 1
		}
	}

	return requirements
}

// parentReplayGroups returns authored anyOf vectors in requirement order.
func parentReplayGroups(requirements []requirement) []parentReplayGroup {
	type groupKey struct {
		usePointer       string
		instanceTemplate string
	}

	indexes := make(map[groupKey]int)

	var groups []parentReplayGroup

	for index, requirement := range requirements {
		if !requirement.hasBranch || requirement.composition != "anyOf" || requirement.branch < 0 {
			continue
		}

		suffix := "/anyOf/" + itoa(requirement.branch)
		key := groupKey{
			usePointer:       strings.TrimSuffix(requirement.occurrence.usePointer, suffix),
			instanceTemplate: requirement.occurrence.instanceTemplate,
		}

		groupIndex, exists := indexes[key]
		if !exists {
			groupIndex = len(groups)
			indexes[key] = groupIndex

			groups = append(groups, parentReplayGroup{})
		}

		groups[groupIndex].indexes = append(groups[groupIndex].indexes, index)
		groups[groupIndex].count = max(groups[groupIndex].count, requirement.branch+1)
	}

	return groups
}

// parentReplayMaskFiniteSize returns the finite size when rankProduct can address it.
func parentReplayMaskFiniteSize(branches int) (uint64, bool) {
	if branches <= 0 {
		return 0, true
	}

	count := new(big.Int).Lsh(big.NewInt(1), uint(branches))
	count.Sub(count, big.NewInt(1))

	if !count.IsUint64() {
		return 0, false
	}

	return count.Uint64(), true
}

// parentReplayMaskCursor enumerates every nonempty mask without an integer rank ceiling.
type parentReplayMaskCursor struct {
	branches int
	selected int
	indexes  []int
	started  bool
}

// newParentReplayMaskCursor starts canonical nonempty mask traversal.
func newParentReplayMaskCursor(branches int) *parentReplayMaskCursor {
	return &parentReplayMaskCursor{branches: branches}
}

// Next returns the next nonempty mask.
func (cursor *parentReplayMaskCursor) Next() (*big.Int, bool) {
	if cursor == nil || cursor.branches <= 0 {
		return nil, false
	}

	if !cursor.started {
		cursor.started = true
		cursor.selected = 1
		cursor.indexes = []int{0}
	} else if !advanceParentReplayMask(cursor) {
		return nil, false
	}

	mask := new(big.Int)
	for _, index := range cursor.indexes {
		mask.SetBit(mask, index, 1)
	}

	return mask, true
}

// advanceParentReplayMask advances one arbitrary-width combination odometer.
func advanceParentReplayMask(cursor *parentReplayMaskCursor) bool {
	for index := len(cursor.indexes) - 1; index >= 0; index-- {
		maximum := cursor.branches - len(cursor.indexes) + index
		if cursor.indexes[index] == maximum {
			continue
		}

		cursor.indexes[index]++
		for next := index + 1; next < len(cursor.indexes); next++ {
			cursor.indexes[next] = cursor.indexes[next-1] + 1
		}

		return true
	}

	if cursor.selected == cursor.branches {
		return false
	}

	cursor.selected++

	cursor.indexes = make([]int, cursor.selected)
	for index := range cursor.indexes {
		cursor.indexes[index] = index
	}

	return true
}

// parentReplayMaskAtOrdinal directly decodes an arbitrary-precision mask
// ordinal in cardinality and authored-branch order.
func parentReplayMaskAtOrdinal(branches int, ordinal *big.Int) (*big.Int, bool) {
	if branches <= 0 || ordinal == nil || ordinal.Sign() < 0 {
		return nil, false
	}

	remaining := new(big.Int).Set(ordinal)

	selected := 1
	for ; selected <= branches; selected++ {
		block := new(big.Int).Binomial(int64(branches), int64(selected))
		if remaining.Cmp(block) < 0 {
			break
		}

		remaining.Sub(remaining, block)
	}

	if selected > branches {
		return nil, false
	}

	mask := new(big.Int)
	next := 0

	for needed := selected; needed > 0; needed-- {
		maximum := branches - needed
		for candidate := next; candidate <= maximum; candidate++ {
			block := new(big.Int).Binomial(
				int64(branches-candidate-1), int64(needed-1),
			)
			if remaining.Cmp(block) < 0 {
				mask.SetBit(mask, candidate, 1)
				next = candidate + 1

				break
			}

			remaining.Sub(remaining, block)
		}
	}

	return mask, true
}

// applyFault copies the current parent, charges one fault choice, and applies one fault.
func applyFault(parent *jsonValue, fault faultProgram, s *search) (*jsonValue, error) {
	if faultNeedsCompositionSearch(fault) {
		return applyCompositionFault(parent, fault, s)
	}

	return applyNonCompositionFault(parent, fault, s)
}

// applyFaultAtRank is the standalone ranked fault adapter.
func applyFaultAtRank(
	parent *jsonValue,
	fault faultProgram,
	occurrenceRank uint64,
	mutationRank uint64,
	s *search,
) (*jsonValue, bool, bool, bool, error) {
	machines := newFaultSearchMachines(s)

	return machines.applyFaultAtRank(
		parent, fault, occurrenceRank, mutationRank, machines.compositionAssignments,
	)
}

// applyFaultAtRank attempts one concrete occurrence and mutation tuple with owned machines.
func (machines faultSearchMachines) applyFaultAtRank(
	parent *jsonValue,
	fault faultProgram,
	occurrenceRank uint64,
	mutationRank uint64,
	compositionAssignments *compositionAssignmentMachine,
) (*jsonValue, bool, bool, bool, error) {
	selected, exists, occurrenceErr := faultAtOccurrenceRank(
		parent, fault, occurrenceRank, machines.search,
	)
	if occurrenceErr != nil {
		return nil, false, false, false, occurrenceErr
	}

	if !exists {
		return nil, false, true, false, nil
	}

	if faultNeedsCompositionSearch(selected) {
		derivative, attempted, exhausted, err := compositionFaultAttemptWithMachine(
			parent, selected, mutationRank, compositionAssignments,
		)

		return derivative, attempted, false, exhausted, err
	}

	if selected.obligation.rule == oracleRuleType {
		derivative, attempted, exhausted, err := rootTypeFaultAttemptAtRank(
			parent, selected, mutationRank, machines.search,
		)

		return derivative, attempted, false, exhausted, err
	}

	switch selected.obligation.rule {
	case oracleRuleEnum, oracleRuleMinimum, oracleRuleExclusiveMinimum, oracleRuleMaximum,
		oracleRuleExclusiveMaximum, oracleRuleMultipleOf, oracleRuleFormat,
		oracleRuleMinLength, oracleRuleMaxLength, oracleRulePattern:
		derivative, attempted, exhausted, err := machines.scalarCandidates.AttemptAtRank(
			parent, selected, mutationRank,
		)

		return derivative, attempted, false, exhausted, err
	}

	derivative, attempted, exhausted, err := nonCompositionFaultAttemptAtRank(
		parent, selected, mutationRank, machines.search,
	)

	return derivative, attempted, false, exhausted, err
}

// faultAtOccurrenceRank resolves every closure-local path before mutation.
// Expected identities are never selected from oracle output.
//
//nolint:cyclop // Obligation and closure-local occurrence dimensions meet here.
func faultAtOccurrenceRank(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
	s *search,
) (faultProgram, bool, error) {
	template := fault.obligation.occurrence.instanceTemplate
	appendRequiredName := false

	if fault.obligation.rule == oracleRuleRequired || fault.obligation.rule == oracleRuleAdditionalProperties {
		tokens, ok := rowPointerTokens(template)
		if !ok || len(tokens) == 0 {
			return faultProgram{}, false, nil
		}

		template = pointerFromTokens(tokens[:len(tokens)-1])
		appendRequiredName = fault.obligation.rule == oracleRuleRequired
	}

	count := matchingValuePathCount(parent, template)
	if count == 0 {
		return faultProgram{}, false, nil
	}

	selectors := []string{template}
	selectedTemplates := map[string]bool{template: true}

	for _, expected := range fault.expected {
		projected := expected.project()
		if selectedTemplates[projected.occurrence.instanceTemplate] ||
			projected.occurrence.instanceTemplate == fault.obligation.occurrence.instanceTemplate ||
			!strings.Contains(projected.occurrence.instanceTemplate, "*") {
			continue
		}

		if matchingValuePathCount(parent, projected.occurrence.instanceTemplate) > 0 {
			selectors = append(selectors, projected.occurrence.instanceTemplate)
			selectedTemplates[projected.occurrence.instanceTemplate] = true
		}
	}

	ordinals := make([]uint64, len(selectors))
	for index := len(selectors) - 1; index >= 0; index-- {
		size := matchingValuePathCount(parent, selectors[index])
		if size == 0 {
			return faultProgram{}, false, nil
		}

		ordinals[index] = rank % size
		rank /= size
	}

	if rank > 0 {
		return faultProgram{}, false, nil
	}

	path, exists := matchingValuePathAt(parent, selectors[0], ordinals[0])
	if !exists {
		return faultProgram{}, false, errors.New("schematest: fault occurrence rank disappeared")
	}

	if err := s.assign(); err != nil {
		return faultProgram{}, false, err
	}

	tokens, _ := rowPointerTokens(fault.obligation.occurrence.instanceTemplate)
	if appendRequiredName {
		path = append(pathCopy(path), tokens[len(tokens)-1])
	} else if fault.obligation.rule == oracleRuleAdditionalProperties {
		path = append(pathCopy(path), "*")
	}

	selected := concretizeFaultAtPath(fault, path)

	for selectorIndex := 1; selectorIndex < len(selectors); selectorIndex++ {
		selectedPath, selectedExists := matchingValuePathAt(
			parent, selectors[selectorIndex], ordinals[selectorIndex],
		)
		if !selectedExists {
			return faultProgram{}, false, errors.New("schematest: closure occurrence rank disappeared")
		}

		if err := s.assign(); err != nil {
			return faultProgram{}, false, err
		}

		for expectedIndex := range selected.expected {
			projected := selected.expected[expectedIndex].project()
			if projected.occurrence.instanceTemplate != selectors[selectorIndex] {
				continue
			}

			identity := cloneEvaluationRecordIdentity(selected.expected[expectedIndex])
			identity.occurrence.instance.tokens = pathCopy(selectedPath)
			selected.expected[expectedIndex] = identity
		}
	}

	return selected, true, nil
}
