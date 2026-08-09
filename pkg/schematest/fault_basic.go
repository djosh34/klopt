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

// parentReplayMaskBits is the number of anyOf mask bits addressable by a rank.
const parentReplayMaskBits = 64

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

	if err := product.SetFinite(faultOccurrenceDimension, 1); err != nil {
		return err
	}

	if fault.alternatives == nil && !faultNeedsCompositionSearch(fault) {
		if err := product.SetFinite(faultMutationDimension, 1); err != nil {
			return err
		}
	}

	var (
		diagonal        uint64
		diagonalStarted bool
		mutationLive    bool
	)

	for {
		ranks, ok := product.Next()
		if !ok {
			return nil
		}

		if !diagonalStarted || product.diagonal != diagonal {
			if diagonalStarted && faultProductDiagonalExhausted(product, diagonal, mutationLive) {
				return nil
			}

			diagonal = product.diagonal
			diagonalStarted = true
			mutationLive = false
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
			if err := product.SetFinite(faultParentDimension, ranks[faultParentDimension]); err != nil {
				return err
			}

			continue
		}

		if !found {
			continue
		}

		derivative, attempted, mutationExhausted, faultErr := applyFaultAtRank(
			parent, selectedFault, ranks[faultMutationDimension], s,
		)
		if mutationExhausted {
			continue
		}

		mutationLive = true

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

		matches, matchErr := faultFailureClosureMatches(result, selectedFault)
		if matchErr != nil {
			return fmt.Errorf("compare fault expected: %w", matchErr)
		}

		if result.valid || !matches {
			continue
		}

		encoded, marshalErr := marshalStrict(derivative)
		if marshalErr != nil {
			return fmt.Errorf("serialize fault derivative: %w", marshalErr)
		}

		covered[fault.obligation.String()] = true

		return yield(Case{JSON: encoded, Valid: false})
	}
}

// faultProductDiagonalExhausted recognizes a diagonal beyond every finite parent and closure rank.
func faultProductDiagonalExhausted(product *rankProductCursor, diagonal uint64, mutationLive bool) bool {
	if mutationLive || product == nil ||
		!product.finite[faultParentDimension] || !product.finite[faultClosureDimension] {
		return false
	}

	parentSize := product.finiteSizes[faultParentDimension]

	closureSize := product.finiteSizes[faultClosureDimension]
	if parentSize == 0 || closureSize == 0 {
		return true
	}

	parentMaximum := parentSize - 1

	closureMaximum := closureSize - 1
	if ^uint64(0)-parentMaximum < closureMaximum {
		return false
	}

	return diagonal > parentMaximum+closureMaximum
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

	selected, choices, found := closureSelectionAtRank(fault, rank)
	if !found {
		return faultProgram{}, false, true, nil
	}

	for range choices {
		if err := s.assign(); err != nil {
			return faultProgram{}, false, false, err
		}
	}

	return selected, true, false, nil
}

// closureSelectionAtRank resolves one canonical complete choice and its atomic assignment count.
func closureSelectionAtRank(fault faultProgram, rank uint64) (faultProgram, int, bool) {
	var observed uint64

	selected := faultProgram{}
	selectedChoices := 0
	found := walkFaultClosurePrograms(
		[]*faultClosureProgram{fault.alternatives},
		fault.requirements,
		fault.expected,
		0,
		func(requirements []requirement, expected failureSet, choices int) bool {
			if observed != rank {
				observed++

				return false
			}

			selected = fault
			selected.requirements = copyPlanRequirements(requirements)

			selected.expected = append(failureSet(nil), expected...)
			selected.alternatives = nil
			selectedChoices = choices

			return true
		},
	)

	return selected, selectedChoices, found
}

// walkFaultClosurePrograms traverses only the current declarative product prefix.
func walkFaultClosurePrograms(
	programs []*faultClosureProgram,
	requirements []requirement,
	expected failureSet,
	choices int,
	visit func([]requirement, failureSet, int) bool,
) bool {
	if len(programs) == 0 {
		return visit(requirements, expected, choices)
	}

	program := programs[0]
	if program == nil {
		return walkFaultClosurePrograms(programs[1:], requirements, expected, choices, visit)
	}

	for alternative := program.alternatives; alternative != nil; alternative = alternative.next {
		nextPrograms := make([]*faultClosureProgram, 0, len(programs)+1)
		if alternative.closure != nil {
			nextPrograms = append(nextPrograms, alternative.closure)
		}

		if program.next != nil {
			nextPrograms = append(nextPrograms, program.next)
		}

		nextPrograms = append(nextPrograms, programs[1:]...)

		if walkFaultClosurePrograms(
			nextPrograms,
			appendPlanRequirements(requirements, alternative.requirements...),
			append(append(failureSet(nil), expected...), alternative.expected...),
			choices+1,
			visit,
		) {
			return true
		}
	}

	return false
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
		size, finite := parentReplayMaskCount(group.count)
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
	var (
		parent   *jsonValue
		observed uint64
	)

	complete, err := s.walkNode(
		s.model.root,
		s.model.root.occurrence,
		requirements,
		rowSearchContext{},
		func(value *jsonValue) (bool, error) {
			result := evaluate(s.model, value)
			if result.err != nil {
				return false, fmt.Errorf("evaluate regenerated parent: %w", result.err)
			}

			if !result.valid || !requirementsMatch(result, value, requirements) {
				return false, nil
			}

			if observed != rank {
				observed++

				return false, nil
			}

			parent = value

			return true, nil
		},
	)
	if err != nil {
		return nil, false, err
	}

	return parent, complete, nil
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

		for _, failure := range fault.expected {
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
				requirements[index].presence = requirementAbsent
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
		mask := new(big.Int).SetUint64(maskRanks[groupIndex])
		mask.Add(mask, big.NewInt(1))

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

// parentReplayMaskCount returns the finite number of nonempty uint64-addressable masks.
func parentReplayMaskCount(branches int) (uint64, bool) {
	if branches <= 0 {
		return 0, true
	}

	if branches >= parentReplayMaskBits {
		return 0, false
	}

	return uint64(1)<<branches - 1, true
}

// applyFault copies the current parent, charges one fault choice, and applies one fault.
func applyFault(parent *jsonValue, fault faultProgram, s *search) (*jsonValue, error) {
	if faultNeedsCompositionSearch(fault) {
		return applyCompositionFault(parent, fault, s)
	}

	return applyNonCompositionFault(parent, fault, s)
}

// applyFaultAtRank attempts one mutation rank without draining another closure's frontier.
func applyFaultAtRank(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	if faultNeedsCompositionSearch(fault) {
		return compositionFaultAttemptAtRank(parent, fault, rank, s)
	}

	if rank > 0 {
		return nil, false, true, nil
	}

	derivative, err := applyNonCompositionFault(parent, fault, s)
	if errors.Is(err, errFaultNotFound) {
		return nil, true, false, nil
	}

	if err != nil {
		return nil, false, false, err
	}

	return derivative, true, false, nil
}
