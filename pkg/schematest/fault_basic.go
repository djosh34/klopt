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

	for _, dimension := range []int{faultOccurrenceDimension, faultMutationDimension} {
		if err := product.SetFinite(dimension, 1); err != nil {
			return err
		}
	}

	for {
		ranks, ok := product.Next()
		if !ok {
			return nil
		}

		selectedFault, closureExists, closureExhausted := faultClosureAtRank(
			fault, ranks[faultClosureDimension],
		)
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

		derivative, faultErr := applyFault(parent, selectedFault, s)
		if errors.Is(faultErr, errFaultNotFound) {
			continue
		}

		if faultErr != nil {
			return faultErr
		}

		result := evaluate(s.model, derivative)
		if result.err != nil {
			return fmt.Errorf("evaluate fault derivative: %w", result.err)
		}

		matches, matchErr := faultFailureClosureMatches(result.failureRecords(), selectedFault)
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

// faultClosureAtRank adapts direct closures to the shared product. Declarative
// aggregate programs have no executable closure cursor in this bounded stage.
func faultClosureAtRank(fault faultProgram, rank uint64) (faultProgram, bool, bool) {
	if fault.alternatives != nil || rank > 0 {
		return faultProgram{}, false, true
	}

	return fault, true, false
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

// copyJSONValue deep-copies one transient parent or fault witness.
func copyJSONValue(value *jsonValue, copied map[*jsonValue]*jsonValue) (*jsonValue, error) {
	if value == nil {
		return nil, errors.New("JSON value is nil")
	}

	if existing, ok := copied[value]; ok {
		return existing, nil
	}

	clone := &jsonValue{kind: value.kind, boolean: value.boolean, text: value.text}
	copied[value] = clone

	if value.number != nil {
		clone.number = &exactNumber{
			numerator:   new(big.Int).Set(value.number.numerator),
			denominator: new(big.Int).Set(value.number.denominator),
			exponent:    new(big.Int).Set(value.number.exponent),
			scale:       new(big.Int).Set(value.number.scale),
		}
	}

	if value.array != nil {
		clone.array = make([]*jsonValue, len(value.array))
		for index, element := range value.array {
			copiedElement, err := copyJSONValue(element, copied)
			if err != nil {
				return nil, err
			}

			clone.array[index] = copiedElement
		}
	}

	if value.object != nil {
		clone.object = make(map[string]*jsonValue, len(value.object))
		for name, member := range value.object {
			copiedMember, err := copyJSONValue(member, copied)
			if err != nil {
				return nil, err
			}

			clone.object[name] = copiedMember
		}
	}

	return clone, nil
}
