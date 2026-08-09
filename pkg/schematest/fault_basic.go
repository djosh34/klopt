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
//nolint:cyclop,gocognit,gocyclo // Product exhaustion, exact verification, and callback errors meet here.
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

	var endpoints *faultProductEndpoint

	var (
		diagonal             uint64
		diagonalStarted      bool
		currentParent        *jsonValue
		currentParentClosure uint64
		currentParentRank    uint64
		currentParentSet     bool
	)

	for {
		ranks, ok := product.Next()
		if !ok {
			return nil
		}

		if !diagonalStarted || product.diagonal != diagonal {
			if diagonalStarted && faultProductStateExhausted(
				product, diagonal, endpoints,
			) {
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
			if err := product.SetFinite(faultClosureDimension, ranks[faultClosureDimension]); err != nil {
				return err
			}

			continue
		}

		if !closureExists {
			continue
		}

		if _, known := faultProductEndpointAt(
			endpoints, faultEndpointParent, ranks[faultClosureDimension], 0, 0,
		); !known && faultMutationIsParentIndependent(selectedFault.obligation.rule) {
			recordFaultProductEndpoint(
				&endpoints, faultEndpointParent, ranks[faultClosureDimension], 0, 0, 1,
			)
		}

		if parentSize, known := faultProductEndpointAt(
			endpoints, faultEndpointParent, ranks[faultClosureDimension], 0, 0,
		); known && ranks[faultParentDimension] >= parentSize {
			continue
		}

		parent := currentParent
		found := currentParentSet && currentParentClosure == ranks[faultClosureDimension] &&
			currentParentRank == ranks[faultParentDimension]
		exhausted := false

		if !found {
			var replayErr error

			parent, found, exhausted, replayErr = regenerateParentAtRank(
				plan, selectedFault, ranks[faultParentDimension], s,
			)
			if replayErr != nil {
				return replayErr
			}

			if found {
				currentParent = parent
				currentParentClosure = ranks[faultClosureDimension]
				currentParentRank = ranks[faultParentDimension]
				currentParentSet = true
			}
		}

		if exhausted {
			recordFaultProductEndpoint(
				&endpoints,
				faultEndpointParent,
				ranks[faultClosureDimension],
				0,
				0,
				ranks[faultParentDimension],
			)

			continue
		}

		if !found {
			continue
		}

		if occurrenceSize, known := faultProductEndpointAt(
			endpoints,
			faultEndpointOccurrence,
			ranks[faultClosureDimension],
			ranks[faultParentDimension],
			0,
		); known && ranks[faultOccurrenceDimension] >= occurrenceSize {
			continue
		}

		if mutationSize, known := faultProductEndpointAt(
			endpoints,
			faultEndpointMutation,
			ranks[faultClosureDimension],
			ranks[faultParentDimension],
			ranks[faultOccurrenceDimension],
		); known && ranks[faultMutationDimension] >= mutationSize {
			continue
		}

		derivative, attempted, occurrenceExhausted, mutationExhausted, faultErr := applyFaultAtRank(
			parent,
			selectedFault,
			ranks[faultOccurrenceDimension],
			ranks[faultMutationDimension],
			s,
		)
		if occurrenceExhausted {
			recordFaultProductEndpoint(
				&endpoints,
				faultEndpointOccurrence,
				ranks[faultClosureDimension],
				ranks[faultParentDimension],
				0,
				ranks[faultOccurrenceDimension],
			)

			continue
		}

		if mutationExhausted {
			recordFaultProductEndpoint(
				&endpoints,
				faultEndpointMutation,
				ranks[faultClosureDimension],
				ranks[faultParentDimension],
				ranks[faultOccurrenceDimension],
				ranks[faultMutationDimension],
			)

			if faultMutationIsParentIndependent(selectedFault.obligation.rule) {
				recordFaultProductEndpoint(
					&endpoints,
					faultEndpointParent,
					ranks[faultClosureDimension],
					0,
					0,
					ranks[faultParentDimension]+1,
				)
			}

			continue
		}

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

// faultProductStateExhausted recognizes a diagonal beyond every closure-local
// parent and mutation endpoint. No closure-local endpoint is promoted to a
// shared product bound.
//
//nolint:cyclop // Conditional endpoints form one fixed product exhaustion check.
func faultProductStateExhausted(
	product *rankProductCursor,
	diagonal uint64,
	endpoints *faultProductEndpoint,
) bool {
	if product == nil || !product.finite[faultClosureDimension] {
		return false
	}

	var maximum uint64

	for closure := uint64(0); closure < product.finiteSizes[faultClosureDimension]; closure++ {
		parentSize, known := faultProductEndpointAt(endpoints, faultEndpointParent, closure, 0, 0)
		if !known {
			return false
		}

		for parent := uint64(0); parent < parentSize; parent++ {
			occurrenceSize, known := faultProductEndpointAt(
				endpoints, faultEndpointOccurrence, closure, parent, 0,
			)
			if !known {
				return false
			}

			for occurrence := uint64(0); occurrence < occurrenceSize; occurrence++ {
				mutationSize, mutationKnown := faultProductEndpointAt(
					endpoints, faultEndpointMutation, closure, parent, occurrence,
				)
				if !mutationKnown {
					return false
				}

				if mutationSize == 0 {
					continue
				}

				candidate := closure + parent + occurrence + mutationSize - 1
				if candidate < closure || candidate < parent || candidate < occurrence {
					return false
				}

				maximum = max(maximum, candidate)
			}
		}
	}

	return diagonal > maximum
}

// faultEndpointKind identifies one conditional product endpoint.
type faultEndpointKind uint8

const (
	// faultEndpointParent identifies a closure-local parent endpoint.
	faultEndpointParent faultEndpointKind = iota
	// faultEndpointOccurrence identifies a parent-local occurrence endpoint.
	faultEndpointOccurrence
	// faultEndpointMutation identifies an occurrence-local mutation endpoint.
	faultEndpointMutation
)

// faultProductEndpoint retains one finite conditional domain endpoint.
type faultProductEndpoint struct {
	kind                        faultEndpointKind
	closure, parent, occurrence uint64
	size                        uint64
	next                        *faultProductEndpoint
}

// recordFaultProductEndpoint records or updates one conditional endpoint.
func recordFaultProductEndpoint(
	endpoints **faultProductEndpoint,
	kind faultEndpointKind,
	closure, parent, occurrence, size uint64,
) {
	for endpoint := *endpoints; endpoint != nil; endpoint = endpoint.next {
		if endpoint.kind == kind && endpoint.closure == closure && endpoint.parent == parent &&
			endpoint.occurrence == occurrence {
			endpoint.size = size

			return
		}
	}

	*endpoints = &faultProductEndpoint{
		kind: kind, closure: closure, parent: parent, occurrence: occurrence,
		size: size, next: *endpoints,
	}
}

// faultProductEndpointAt returns one recorded conditional endpoint.
func faultProductEndpointAt(
	endpoints *faultProductEndpoint,
	kind faultEndpointKind,
	closure, parent, occurrence uint64,
) (uint64, bool) {
	for endpoint := endpoints; endpoint != nil; endpoint = endpoint.next {
		if endpoint.kind == kind && endpoint.closure == closure && endpoint.parent == parent &&
			endpoint.occurrence == occurrence {
			return endpoint.size, true
		}
	}

	return 0, false
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
		mask, exists := parentReplayMaskAtRank(group.count, maskRanks[groupIndex])
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

// parentReplayMaskCount returns the exact finite size when it is addressable by uint64 ranks.
func parentReplayMaskCount(branches int) (uint64, bool) {
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

// parentReplayMaskAtRank enumerates nonempty branch sets by cardinality and
// authored branch order. Singleton ranks therefore reach every authored branch,
// including indexes at and beyond 64, before wider masks.
func parentReplayMaskAtRank(branches int, rank uint64) (*big.Int, bool) {
	if branches <= 0 {
		return nil, false
	}

	for selected := 1; selected <= branches; selected++ {
		count := saturatedBinomial(uint64(branches), uint64(selected))
		if rank >= count {
			if count == ^uint64(0) {
				return nil, false
			}

			rank -= count

			continue
		}

		indexes, exists := arrayCombinationAt(branches, selected, rank)
		if !exists {
			return nil, false
		}

		mask := new(big.Int)
		for _, index := range indexes {
			mask.SetBit(mask, index, 1)
		}

		return mask, true
	}

	return nil, false
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
	selected, exists := faultAtOccurrenceRank(parent, fault, occurrenceRank)
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
		derivative, attempted, exhausted, err := typeFaultAttemptAtRank(
			parent, selected, mutationRank, s,
		)

		return derivative, attempted, false, exhausted, err
	}

	if mutationRank > 0 {
		return nil, false, false, true, nil
	}

	derivative, err := applyNonCompositionFault(parent, selected, s)
	if errors.Is(err, errFaultNotFound) {
		return nil, true, false, false, nil
	}

	if err != nil {
		return nil, false, false, false, err
	}

	return derivative, true, false, false, nil
}

// faultMutationIsParentIndependent reports scalar replacements that do not depend on parent contents.
func faultMutationIsParentIndependent(rule string) bool {
	switch rule {
	case oracleRuleType, oracleRuleEnum, oracleRuleMinimum, oracleRuleExclusiveMinimum,
		oracleRuleMaximum, oracleRuleExclusiveMaximum, oracleRuleMultipleOf,
		oracleRuleFormat, oracleRuleMinLength, oracleRuleMaxLength, oracleRulePattern:
		return true
	default:
		return false
	}
}

// faultAtOccurrenceRank resolves one concrete parent occurrence.
func faultAtOccurrenceRank(parent *jsonValue, fault faultProgram, rank uint64) (faultProgram, bool) {
	template := fault.obligation.occurrence.instanceTemplate
	appendRequiredName := false

	if fault.obligation.rule == oracleRuleRequired || fault.obligation.rule == oracleRuleAdditionalProperties {
		tokens, ok := rowPointerTokens(template)
		if !ok || len(tokens) == 0 {
			return faultProgram{}, false
		}

		template = pointerFromTokens(tokens[:len(tokens)-1])
		appendRequiredName = fault.obligation.rule == oracleRuleRequired
	}

	for path := range matchingValuePathSequence(parent, template) {
		if rank > 0 {
			rank--

			continue
		}

		if appendRequiredName {
			tokens, _ := rowPointerTokens(fault.obligation.occurrence.instanceTemplate)
			path = append(pathCopy(path), tokens[len(tokens)-1])
		}

		return concretizeFaultAtPath(fault, path), true
	}

	return faultProgram{}, false
}
