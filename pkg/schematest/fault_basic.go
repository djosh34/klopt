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

// streamFault is the sole continuation over parent, closure, occurrence, and mutation ranks.
//
//nolint:cyclop,gocognit // Product exhaustion, exact verification, and callback errors meet here.
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

	var (
		diagonal        uint64
		diagonalStarted bool
		diagonalLive    bool
	)

	for {
		ranks, ok := product.Next()
		if !ok {
			return nil
		}

		if !diagonalStarted || product.diagonal != diagonal {
			if diagonalStarted && !diagonalLive && product.finite[faultClosureDimension] {
				return nil
			}

			diagonal = product.diagonal
			diagonalLive = false
			diagonalStarted = true
		}

		selectedFault, closureExists, closureExhausted, closureErr := faultClosureAtRank(
			fault, ranks[faultClosureDimension], s,
		)
		if closureErr != nil {
			return closureErr
		}

		if closureExhausted {
			if err := product.SetFinite(faultClosureDimension, ranks[faultClosureDimension]); err != nil {
				return err
			}

			continue
		}

		if !closureExists {
			continue
		}

		parent, found, exhausted, replayErr := regenerateParentAtRank(
			plan, selectedFault, ranks[faultParentDimension], s,
		)
		if replayErr != nil {
			return replayErr
		}

		if exhausted {
			continue
		}

		if !found {
			diagonalLive = true

			continue
		}

		derivative, attempted, occurrenceExhausted, mutationExhausted, faultErr := applyFaultAtRank(
			parent,
			selectedFault,
			ranks[faultOccurrenceDimension],
			ranks[faultMutationDimension],
			s,
		)
		if occurrenceExhausted || mutationExhausted {
			continue
		}

		diagonalLive = true

		if faultErr != nil {
			return faultErr
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
	parent, found, _, err := regenerateParentAtRank(plan, fault, 0, s)

	return parent, found, err
}

// regenerateParentAtRank fairly addresses one valid parent across nonempty
// anyOf masks and complete-row ranks. It retains no generated parent corpus.
//
//nolint:cyclop // Finite mask setup and diagonal exhaustion share the replay cursor boundary.
func regenerateParentAtRank(
	plan *searchPlan,
	fault faultProgram,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
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

		parent, found, replayErr := parentCandidateAtRank(
			s, requirements, ranks[len(ranks)-1],
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

// parentCandidateAtRank regenerates one complete row rank under exact parent requirements.
func parentCandidateAtRank(s *search, requirements []requirement, rank uint64) (*jsonValue, bool, error) {
	if candidate, exists := activeEnumValueAtRank(
		s.model.root, s.model.root.occurrence, requirements, new(uint64),
	); exists && candidate != nil {
		return parentEnumCandidateAtRank(s, requirements, rank)
	}

	var (
		candidate *jsonValue
		observed  uint64
	)

	selected, err := s.walkNode(
		s.model.root,
		s.model.root.occurrence,
		requirements,
		rowSearchContext{},
		func(value *jsonValue) (bool, error) {
			if observed < rank {
				observed++

				return false, nil
			}

			candidate = value

			return true, nil
		},
	)
	if err != nil || !selected || candidate == nil {
		return nil, false, err
	}

	result := evaluate(s.model, candidate)
	if result.err != nil {
		return nil, false, fmt.Errorf("evaluate regenerated parent: %w", result.err)
	}

	if !result.valid || !requirementsMatch(result, candidate, requirements) {
		return candidate, false, nil
	}

	return candidate, true, nil
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

// applyFaultAtRank attempts one concrete occurrence and mutation tuple.
func applyFaultAtRank(
	parent *jsonValue,
	fault faultProgram,
	occurrenceRank uint64,
	mutationRank uint64,
	s *search,
) (*jsonValue, bool, bool, bool, error) {
	selected, exists, occurrenceErr := faultAtOccurrenceRank(parent, fault, occurrenceRank, s)
	if occurrenceErr != nil {
		return nil, false, false, false, occurrenceErr
	}

	if !exists {
		return nil, false, true, false, nil
	}

	if faultNeedsCompositionSearch(selected) {
		derivative, attempted, exhausted, err := compositionFaultAttemptAtRank(
			parent, selected, mutationRank, s,
		)

		return derivative, attempted, false, exhausted, err
	}

	if selected.obligation.rule == oracleRuleType {
		derivative, attempted, exhausted, err := rootTypeFaultAttemptAtRank(
			parent, selected, mutationRank, s,
		)

		return derivative, attempted, false, exhausted, err
	}

	derivative, attempted, exhausted, err := nonCompositionFaultAttemptAtRank(
		parent, selected, mutationRank, s,
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
