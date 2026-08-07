//nolint:cyclop,gocognit,godoclint,mnd,wsl_v5 // The private product search keeps objective and frontier order explicit.
package schematest

import (
	"errors"
	"fmt"
	"strconv"
	"unicode"
	"unicode/utf16"
)

const maxStringSearchUint64 = ^uint64(0)

// walkString searches the ordered string objectives for one row target.
func (s *search) walkString(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	visit rowVisit,
) (bool, error) {
	product, err := buildStringProduct(node, occurrence, pins)
	if err != nil {
		return false, err
	}

	if !product.hasStringRules() {
		candidates, candidateErr := rowScalarValues(node, jsonString)
		if candidateErr != nil {
			return false, candidateErr
		}
		for _, enumValue := range product.enumValues {
			candidates, candidateErr = appendUniqueJSONWitness(candidates, enumValue)
			if candidateErr != nil {
				return false, candidateErr
			}
		}

		for _, candidate := range candidates {
			assignErr := s.assign()
			if assignErr != nil {
				return false, assignErr
			}

			complete, visitErr := visit(candidate)
			if visitErr != nil || complete {
				return complete, visitErr
			}
		}

		return false, nil
	}
	if !s.hasStringTarget {
		return false, errors.New("string search has no target")
	}

	owner, err := product.ownerFor(s.stringTarget)
	if err != nil {
		return false, err
	}
	last, err := stringProductTargetIsLast(product, s.stringTarget)
	if err != nil {
		return false, err
	}

	return s.runStringObjectiveSchedule(
		product,
		owner,
		s.stringRule,
		s.stringLevel,
		last,
		func(objective stringObjective, value *jsonValue) (bool, error) {
			if objective.kind != stringObjectiveAllTrue {
				return true, nil
			}

			return visit(value)
		},
	)
}

// searchStringObjectives streams the ordered clean objectives without retaining witnesses.
func (s *search) searchStringObjectives(
	product stringProduct,
	rule, level string,
	visit func(stringObjective, *jsonValue) (bool, error),
) error {
	_, err := s.runStringObjectiveSchedule(product, product.defaultOwner, rule, level, true, visit)

	return err
}

func (s *search) searchStringObjective(
	product stringProduct,
	objective stringObjective,
	visit rowVisit,
) (bool, error) {
	seed, err := stringSearchSeed(objective)
	if err != nil {
		return false, err
	}

	impossible, err := stringObjectiveIsImpossible(product, objective)
	if err != nil {
		return false, err
	}
	if impossible {
		return false, nil
	}

	pinnedLengths, err := stringPinnedLengths(product, objective)
	if err != nil {
		return false, err
	}

	tryLength := func(length uint64) (bool, error) {
		matches, lengthErr := stringLengthMatchesObjective(product, objective, length)
		if lengthErr != nil {
			return false, lengthErr
		}

		if !matches {
			return false, nil
		}

		if assignErr := s.assign(); assignErr != nil {
			return false, assignErr
		}

		runtime := stringProductRuntime{
			patterns: make([]stringPatternRuntime, 0, len(product.patterns)),
		}
		for _, pattern := range product.patterns {
			patternRuntime := newStringPatternRuntime(pattern.graph)
			for _, assertion := range pattern.assertions {
				assertionRuntime := stringPatternAssertionRuntime{
					graph:    assertion.graph,
					raw:      []int{assertion.graph.start},
					positive: assertion.positive,
				}
				assertionRuntime.matched = assertionRuntime.acceptsAt(
					0, 0, false, nil, length == 0,
				)
				patternRuntime.assertions = append(patternRuntime.assertions, assertionRuntime)
			}

			runtime.patterns = append(runtime.patterns, patternRuntime)
		}

		return s.walkStringLength(product, objective, runtime, length, seed, visit)
	}

	for _, candidate := range product.enumValues {
		matches, matchErr := stringObjectiveMatches(product, objective, candidate.text)
		if matchErr != nil {
			return false, matchErr
		}
		if !matches {
			continue
		}

		oracleMatches, oracleErr := stringObjectiveMatchesOracle(objective, candidate)
		if oracleErr != nil {
			return false, oracleErr
		}
		if !oracleMatches {
			continue
		}

		if assignErr := s.assign(); assignErr != nil {
			return false, assignErr
		}

		complete, visitErr := visit(candidate)
		if visitErr != nil || complete {
			return complete, visitErr
		}
	}

	for _, length := range pinnedLengths {
		complete, tryErr := tryLength(length)
		if tryErr != nil || complete {
			return complete, tryErr
		}
	}

	maximum, finite, err := stringObjectiveMaximumLength(product, objective)
	if err != nil {
		return false, err
	}

	for length := uint64(0); ; length++ {
		if finite && length > maximum {
			return false, nil
		}

		if containsUint64(pinnedLengths, length) {
			if length == maxStringSearchUint64 {
				return false, nil
			}

			continue
		}

		complete, lengthErr := tryLength(length)
		if lengthErr != nil || complete {
			return complete, lengthErr
		}

		if length == maxStringSearchUint64 {
			return false, nil
		}
	}
}

func stringPinnedLengthConstraint(pinned stringPinnedLength) stringConstraintRef {
	switch pinned.kind {
	case stringObjectivePatternFalse:
		return stringConstraintRef{kind: stringConstraintPattern, index: pinned.index}
	case stringObjectiveFormatFalse:
		return stringConstraintRef{kind: stringConstraintFormat, index: pinned.index}
	default:
		return stringConstraintRef{kind: stringConstraintLength, index: pinned.index}
	}
}

type stringDFSFrame struct {
	runtime      stringProductRuntime
	position     uint64
	runeLength   uint64
	previous     uint16
	hasPrevious  bool
	pendingHigh  bool
	prefixLength int

	initialized     bool
	transitions     []stringProductTransition
	transitionIndex int
	candidates      []uint16
	candidateIndex  int
}

func (s *search) walkStringLength(
	product stringProduct,
	objective stringObjective,
	runtime stringProductRuntime,
	targetLength, seed uint64,
	visit rowVisit,
) (bool, error) {
	units := make([]uint16, 0)
	frames := []stringDFSFrame{{runtime: runtime}}

	pop := func() bool {
		frames = frames[:len(frames)-1]
		if len(frames) == 0 {
			return false
		}
		units = units[:frames[len(frames)-1].prefixLength]

		return true
	}

	for len(frames) > 0 {
		frame := &frames[len(frames)-1]
		if !frame.initialized {
			complete, shouldPop, err := s.initializeStringDFSFrame(
				frame,
				product,
				objective,
				targetLength,
				units,
				visit,
			)
			if err != nil || complete {
				return complete, err
			}
			if shouldPop {
				if !pop() {
					return false, nil
				}

				continue
			}
		}

		for frame.transitionIndex < len(frame.transitions) {
			if frame.candidates == nil {
				frame.candidates = stringIntervalCandidates(
					frame.transitions[frame.transitionIndex].interval,
					stringSearchSeedForInterval(seed, frame.transitions[frame.transitionIndex].interval),
					product.seedUnits,
				)
			}
			if frame.candidateIndex < len(frame.candidates) {
				break
			}
			frame.transitionIndex++
			frame.candidates = nil
			frame.candidateIndex = 0
		}
		if frame.transitionIndex == len(frame.transitions) {
			if !pop() {
				return false, nil
			}

			continue
		}

		transition := frame.transitions[frame.transitionIndex]
		unit := frame.candidates[frame.candidateIndex]
		frame.candidateIndex++
		if !stringUTF16UnitAllowed(frame.pendingHigh, unit) {
			continue
		}
		if err := s.assign(); err != nil {
			return false, err
		}

		nextRuneLength := frame.runeLength
		if !frame.pendingHigh {
			nextRuneLength++
		}
		nextPendingHigh := unit >= 0xd800 && unit <= 0xdbff
		nextAtEnd := nextRuneLength == targetLength && !nextPendingHigh
		nextRuntime, valid := stringNextProductRuntime(
			objective,
			frame.runtime,
			transition,
			unit,
			frame.position,
			nextAtEnd,
		)
		if !valid {
			continue
		}

		units = append(units, unit)
		frames = append(frames, stringDFSFrame{
			runtime:      nextRuntime,
			position:     frame.position + 1,
			runeLength:   nextRuneLength,
			previous:     unit,
			hasPrevious:  true,
			pendingHigh:  nextPendingHigh,
			prefixLength: len(units),
		})
	}

	return false, nil
}

func (s *search) initializeStringDFSFrame(
	frame *stringDFSFrame,
	product stringProduct,
	objective stringObjective,
	targetLength uint64,
	units []uint16,
	visit rowVisit,
) (bool, bool, error) {
	frame.initialized = true
	if frame.runeLength == targetLength && !frame.pendingHigh {
		complete, err := s.finishStringCandidate(
			product,
			objective,
			frame.runtime,
			frame.position,
			frame.previous,
			frame.hasPrevious,
			units,
			visit,
		)

		return complete, true, err
	}
	if frame.runeLength > targetLength {
		return false, true, nil
	}

	frame.transitions = stringProductTransitions(
		product,
		objective,
		frame.runtime,
		units,
		int(frame.position),
		frame.previous,
		frame.hasPrevious,
	)

	return false, false, nil
}

func stringUTF16UnitAllowed(pendingHigh bool, unit uint16) bool {
	if pendingHigh {
		return unit >= 0xdc00 && unit <= 0xdfff
	}

	return unit < 0xdc00 || unit > 0xdfff
}

func stringNextProductRuntime(
	objective stringObjective,
	runtime stringProductRuntime,
	transition stringProductTransition,
	unit uint16,
	position uint64,
	atEnd bool,
) (stringProductRuntime, bool) {
	next := stringProductRuntime{patterns: make([]stringPatternRuntime, len(runtime.patterns))}
	for index, pattern := range runtime.patterns {
		patternRequired := stringObjectiveRequiresConstraint(objective, stringConstraintRef{
			kind: stringConstraintPattern, index: index,
		})
		nextPattern := stringPatternRuntime{
			graph: pattern.graph,
			raw:   append([]int(nil), transition.targets[index]...),
		}

		for assertionIndex, assertion := range pattern.assertions {
			nextAssertion := stringPatternAssertionRuntime{
				graph:    assertion.graph,
				positive: assertion.positive,
			}
			if assertion.matched {
				nextAssertion.raw = append([]int(nil), assertion.raw...)
				nextAssertion.matched = true
			} else {
				targets := transition.assertionTargets[index]
				if assertionIndex < len(targets) {
					nextAssertion.raw = append([]int(nil), targets[assertionIndex]...)
				}
				nextAssertion.matched = nextAssertion.acceptsAt(int(position+1), unit, true, nil, atEnd)
				if nextAssertion.matched && !nextAssertion.positive && patternRequired {
					return stringProductRuntime{}, false
				}
			}

			nextPattern.assertions = append(nextPattern.assertions, nextAssertion)
		}
		next.patterns[index] = nextPattern
	}

	return next, true
}

func stringProductTransitions(
	product stringProduct,
	objective stringObjective,
	runtime stringProductRuntime,
	prefix []uint16,
	position int,
	previous uint16,
	hasPrevious bool,
) []stringProductTransition {
	patternTransitions, assertionTransitions, boundaries := stringPatternTransitionFrontiers(
		runtime, position, previous, hasPrevious,
	)
	formatTransitions, formatBoundaries := stringFormatTransitionFrontiers(product, objective, prefix)
	boundaries = sortedInts(append(boundaries, formatBoundaries...))

	preserveEmailPartitions := stringProductPreservesEmailPartitions(product, prefix)
	transitions := make([]stringProductTransition, 0, len(boundaries))
	for index := 0; index+1 < len(boundaries); index++ {
		low := boundaries[index]
		high := boundaries[index+1] - 1
		if low > high || low >= stringUTF16UnitCount {
			continue
		}
		if high >= stringUTF16UnitCount {
			high = stringUTF16UnitCount - 1
		}

		transition, allowed := stringProductTransitionForInterval(
			objective, runtime, patternTransitions, assertionTransitions, formatTransitions, low, high,
		)
		if !allowed {
			continue
		}
		if stringTransitionsCanMerge(transitions, transition, preserveEmailPartitions) {
			transitions[len(transitions)-1].interval.high = transition.interval.high

			continue
		}
		transitions = append(transitions, transition)
	}

	return transitions
}

func stringPatternTransitionFrontiers(
	runtime stringProductRuntime,
	position int,
	previous uint16,
	hasPrevious bool,
) ([][]stringPatternTransition, [][][]stringPatternTransition, []int) {
	patterns := make([][]stringPatternTransition, len(runtime.patterns))
	assertions := make([][][]stringPatternTransition, len(runtime.patterns))
	boundaries := []int{0, stringUTF16UnitCount}

	for index, pattern := range runtime.patterns {
		patterns[index] = pattern.outgoing(position, previous, hasPrevious)
		boundaries = appendStringTransitionBoundaries(boundaries, patterns[index])

		assertions[index] = make([][]stringPatternTransition, len(pattern.assertions))
		for assertionIndex, assertion := range pattern.assertions {
			if assertion.matched {
				continue
			}
			assertions[index][assertionIndex] = stringPatternRuntime{
				graph: assertion.graph,
				raw:   assertion.raw,
			}.outgoing(position, previous, hasPrevious)
			boundaries = appendStringTransitionBoundaries(boundaries, assertions[index][assertionIndex])
		}
	}

	return patterns, assertions, boundaries
}

func appendStringTransitionBoundaries(boundaries []int, transitions []stringPatternTransition) []int {
	for _, transition := range transitions {
		boundaries = append(boundaries, int(transition.interval.low), int(transition.interval.high)+1)
	}

	return boundaries
}

func stringFormatTransitionFrontiers(
	product stringProduct,
	objective stringObjective,
	prefix []uint16,
) ([][]stringUnitInterval, []int) {
	transitions := make([][]stringUnitInterval, len(product.formats))
	var boundaries []int
	for index, format := range product.formats {
		if !stringObjectiveRequiresConstraint(objective, stringConstraintRef{
			kind: stringConstraintFormat, index: index,
		}) {
			continue
		}
		transitions[index] = stringFormatIntervals(format.format, prefix)
		for _, interval := range transitions[index] {
			boundaries = append(boundaries, int(interval.low), int(interval.high)+1)
		}
	}

	return transitions, boundaries
}

func stringProductPreservesEmailPartitions(product stringProduct, prefix []uint16) bool {
	if !stringEmailAddressLiteralActive(prefix) {
		return false
	}
	for _, format := range product.formats {
		if format.format == schemaFormatEmail {
			return true
		}
	}

	return false
}

func stringProductTransitionForInterval(
	objective stringObjective,
	runtime stringProductRuntime,
	patternTransitions [][]stringPatternTransition,
	assertionTransitions [][][]stringPatternTransition,
	formatTransitions [][]stringUnitInterval,
	low int,
	high int,
) (stringProductTransition, bool) {
	unit := uint16(low)
	if !stringFormatsAllowUnit(objective, formatTransitions, unit) {
		return stringProductTransition{}, false
	}

	targets := make([][]int, len(patternTransitions))
	assertionTargets := make([][][]int, len(patternTransitions))
	for patternIndex, candidates := range patternTransitions {
		patternRequired := stringObjectiveRequiresConstraint(objective, stringConstraintRef{
			kind: stringConstraintPattern, index: patternIndex,
		})
		targets[patternIndex] = stringPatternTargetsAtUnit(candidates, unit)
		if len(targets[patternIndex]) == 0 && patternRequired {
			return stringProductTransition{}, false
		}

		var allowed bool
		assertionTargets[patternIndex], allowed = stringAssertionTargetsAtUnit(
			runtime.patterns[patternIndex], assertionTransitions[patternIndex], unit, patternRequired,
		)
		if !allowed {
			return stringProductTransition{}, false
		}
	}

	return stringProductTransition{
		interval:         stringUnitInterval{low: unit, high: uint16(high)},
		targets:          copyPatternTargets(targets),
		assertionTargets: copyPatternAssertionTargets(assertionTargets),
	}, true
}

func stringFormatsAllowUnit(
	objective stringObjective,
	transitions [][]stringUnitInterval,
	unit uint16,
) bool {
	for index, intervals := range transitions {
		if !stringObjectiveRequiresConstraint(objective, stringConstraintRef{
			kind: stringConstraintFormat, index: index,
		}) {
			continue
		}
		if !intervalContains(intervals, unit) {
			return false
		}
	}

	return true
}

func stringPatternTargetsAtUnit(transitions []stringPatternTransition, unit uint16) []int {
	for _, transition := range transitions {
		if unit >= transition.interval.low && unit <= transition.interval.high {
			return transition.targets
		}
	}

	return nil
}

func stringAssertionTargetsAtUnit(
	pattern stringPatternRuntime,
	transitions [][]stringPatternTransition,
	unit uint16,
	patternRequired bool,
) ([][]int, bool) {
	targets := make([][]int, len(transitions))
	for index, candidates := range transitions {
		if pattern.assertions[index].matched {
			continue
		}
		targets[index] = stringPatternTargetsAtUnit(candidates, unit)
		if len(targets[index]) == 0 && pattern.assertions[index].positive && patternRequired {
			return nil, false
		}
	}

	return targets, true
}

func stringTransitionsCanMerge(
	transitions []stringProductTransition,
	next stringProductTransition,
	preserveEmailPartitions bool,
) bool {
	if len(transitions) == 0 || preserveEmailPartitions {
		return false
	}
	previous := transitions[len(transitions)-1]

	return previous.interval.high+1 == next.interval.low && patternTargetsEqual(
		previous.targets, next.targets, previous.assertionTargets, next.assertionTargets,
	)
}

func copyPatternTargets(targets [][]int) [][]int {
	copyTargets := make([][]int, len(targets))
	for index, target := range targets {
		copyTargets[index] = append([]int(nil), target...)
	}

	return copyTargets
}

func copyPatternAssertionTargets(targets [][][]int) [][][]int {
	copyTargets := make([][][]int, len(targets))
	for index, patternTargets := range targets {
		copyTargets[index] = copyPatternTargets(patternTargets)
	}

	return copyTargets
}

func patternTargetsEqual(
	left, right [][]int,
	leftAssertions, rightAssertions [][][]int,
) bool {
	if len(left) != len(right) || len(leftAssertions) != len(rightAssertions) {
		return false
	}

	for index := range left {
		if !intSlicesEqual(left[index], right[index]) {
			return false
		}
	}

	for index := range leftAssertions {
		if len(leftAssertions[index]) != len(rightAssertions[index]) {
			return false
		}

		for assertionIndex := range leftAssertions[index] {
			if !intSlicesEqual(
				leftAssertions[index][assertionIndex],
				rightAssertions[index][assertionIndex],
			) {
				return false
			}
		}
	}

	return true
}

func (s *search) finishStringCandidate(
	product stringProduct,
	objective stringObjective,
	runtime stringProductRuntime,
	position uint64,
	previous uint16,
	hasPrevious bool,
	units []uint16,
	visit rowVisit,
) (bool, error) {
	if position > uint64(^uint(0)>>1) {
		return false, errors.New("string graph position overflows int")
	}

	for index, pattern := range runtime.patterns {
		if !stringObjectiveRequiresConstraint(objective, stringConstraintRef{
			kind:  stringConstraintPattern,
			index: index,
		}) {
			continue
		}

		if !pattern.accepts(int(position), previous, hasPrevious) {
			return false, nil
		}

		for _, assertion := range pattern.assertions {
			if assertion.matched != assertion.positive {
				return false, nil
			}
		}
	}

	text := string(utf16.Decode(units))
	matches, err := stringObjectiveMatches(product, objective, text)
	if err != nil {
		return false, err
	}

	if !matches {
		return false, nil
	}

	candidate := &jsonValue{kind: jsonString, text: text}
	oracleMatches, oracleErr := stringObjectiveMatchesOracle(objective, candidate)
	if oracleErr != nil || !oracleMatches {
		return false, oracleErr
	}

	return visit(candidate)
}

func stringObjectiveMatches(
	product stringProduct,
	objective stringObjective,
	text string,
) (bool, error) {
	for index, constraint := range product.enums {
		want, constrained := stringObjectiveConstraintTruth(objective, stringConstraintRef{
			kind: stringConstraintEnum, index: index,
		})
		if !constrained {
			continue
		}

		matches, err := stringEnumConstraintMatches(constraint, text)
		if err != nil {
			return false, err
		}
		if matches != want {
			return false, nil
		}
	}

	for index, pattern := range product.patterns {
		want, constrained := stringObjectiveConstraintTruth(objective, stringConstraintRef{
			kind:  stringConstraintPattern,
			index: index,
		})
		if !constrained {
			continue
		}

		matches, err := cleanPatternMatches(pattern.pattern, text)
		if err != nil {
			return false, err
		}

		if matches != want {
			return false, nil
		}
	}

	for index, format := range product.formats {
		want, constrained := stringObjectiveConstraintTruth(objective, stringConstraintRef{
			kind:  stringConstraintFormat,
			index: index,
		})
		if !constrained {
			continue
		}

		matches, err := cleanStringFormatMatches(text, format.format)
		if err != nil {
			return false, err
		}

		if matches != want {
			return false, nil
		}
	}

	length := uint64(len([]rune(text)))
	for index, constraint := range product.lengths {
		want, constrained := stringObjectiveConstraintTruth(objective, stringConstraintRef{
			kind:  stringConstraintLength,
			index: index,
		})
		if !constrained {
			continue
		}

		matches, err := stringLengthConstraintMatches(constraint, length)
		if err != nil {
			return false, err
		}

		if matches != want {
			return false, nil
		}
	}

	return true, nil
}

func stringEnumConstraintMatches(constraint stringEnumConstraint, text string) (bool, error) {
	candidate := &jsonValue{kind: jsonString, text: text}
	for _, member := range constraint.members {
		matches, err := jsonSemanticEqual(candidate, member)
		if err != nil {
			return false, err
		}
		if matches {
			return true, nil
		}
	}

	return false, nil
}

func stringObjectiveMatchesOracle(objective stringObjective, candidate *jsonValue) (bool, error) {
	if len(objective.closure) == 0 {
		return true, nil
	}
	if objective.owner.node == nil || objective.owner.node.schemaShape == nil {
		return false, errors.New("schematest invariant: directed string objective has no owner")
	}

	result := evaluateNode(objective.owner.node, candidate, objective.owner.occurrence)
	if result.err != nil {
		return false, fmt.Errorf("evaluate directed string candidate: %w", result.err)
	}

	return exactFailureClosureMatches(result, objective.closure), nil
}

func exactFailureClosureMatches(result evaluation, expected []failureIdentity) bool {
	if result.records == nil || result.records.failures.count != len(expected) {
		return false
	}

	index := 0
	matches := true
	result.records.failures.forEach(func(actual failureIdentity) {
		if index >= len(expected) || actual != expected[index] {
			matches = false
		}
		index++
	})

	return matches && index == len(expected)
}

func stringLengthConstraintMatches(constraint stringLengthConstraint, length uint64) (bool, error) {
	comparison, err := compareStringLengthToBound(length, constraint.bound)
	if err != nil {
		return false, err
	}

	if constraint.kind == stringRuleMinLength {
		return comparison >= 0, nil
	}

	return comparison <= 0, nil
}

func stringLengthMatchesObjective(product stringProduct, objective stringObjective, length uint64) (bool, error) {
	for index, constraint := range product.lengths {
		want, constrained := stringObjectiveConstraintTruth(objective, stringConstraintRef{
			kind:  stringConstraintLength,
			index: index,
		})
		if !constrained {
			continue
		}

		matches, err := stringLengthConstraintMatches(constraint, length)
		if err != nil {
			return false, err
		}

		if matches != want {
			return false, nil
		}
	}

	return true, nil
}

func compareStringLengthToBound(length uint64, bound *exactCount) (int, error) {
	if bound == nil || bound.number == nil {
		return 0, errors.New("string length bound is nil")
	}

	actual, err := parseExactNumber(strconv.FormatUint(length, 10))
	if err != nil {
		return 0, err
	}

	return actual.compare(bound.number)
}

func stringPinnedLengths(product stringProduct, objective stringObjective) ([]uint64, error) {
	lengths := make([]uint64, 0, len(product.fixedLengths)+len(product.lengths))
	appendLength := func(length uint64) {
		for _, existing := range lengths {
			if existing == length {
				return
			}
		}

		lengths = append(lengths, length)
	}

	for _, pinned := range product.fixedLengths {
		if stringObjectiveFalseConstraint(objective, stringPinnedLengthConstraint(pinned)) {
			continue
		}

		matches, err := stringLengthMatchesObjective(product, objective, pinned.value)
		if err != nil {
			return nil, err
		}

		if matches {
			appendLength(pinned.value)
		}
	}

	for index, pattern := range product.patterns {
		if !pattern.finite || pattern.maximum == maxStringSearchUint64 || !stringObjectiveFalseConstraint(
			objective,
			stringConstraintRef{kind: stringConstraintPattern, index: index},
		) {
			continue
		}

		candidate := pattern.maximum + 1
		matches, err := stringLengthMatchesObjective(product, objective, candidate)
		if err != nil {
			return nil, err
		}
		if matches {
			appendLength(candidate)
		}
	}

	for index, constraint := range product.lengths {
		if constraint.bound == nil || constraint.bound.number == nil {
			continue
		}

		bound, fits, err := exactCountUint64(constraint.bound)
		if err != nil {
			return nil, err
		}
		if !fits {
			continue
		}

		candidate := bound
		switch {
		case stringObjectiveFalseConstraint(objective, stringConstraintRef{
			kind:  stringConstraintLength,
			index: index,
		}) && constraint.kind == stringRuleMinLength:
			if bound == 0 {
				continue
			}

			candidate = bound - 1
		case stringObjectiveFalseConstraint(objective, stringConstraintRef{
			kind:  stringConstraintLength,
			index: index,
		}) && constraint.kind == stringRuleMaxLength:
			if bound == maxStringSearchUint64 {
				continue
			}

			candidate = bound + 1
		}

		matches, err := stringLengthMatchesObjective(product, objective, candidate)
		if err != nil {
			return nil, err
		}
		if matches {
			appendLength(candidate)
		}
	}

	return lengths, nil
}

func stringObjectiveIsImpossible(product stringProduct, objective stringObjective) (bool, error) {
	for index, pattern := range product.patterns {
		truth, constrained := stringObjectiveConstraintTruth(objective, stringConstraintRef{
			kind:  stringConstraintPattern,
			index: index,
		})
		if constrained && !truth && stringPatternMatchesEveryString(pattern.pattern) {
			return true, nil
		}
	}

	for index, constraint := range product.lengths {
		truth, constrained := stringObjectiveConstraintTruth(objective, stringConstraintRef{
			kind:  stringConstraintLength,
			index: index,
		})
		if !constrained || truth {
			continue
		}

		bound, fits, err := exactCountUint64(constraint.bound)
		if err != nil {
			return false, err
		}
		if !fits {
			continue
		}
		if (constraint.kind == stringRuleMinLength && bound == 0) ||
			(constraint.kind == stringRuleMaxLength && bound == maxStringSearchUint64) {
			return true, nil
		}
	}

	return false, nil
}

func stringPatternMatchesEveryString(pattern *patternAST) bool {
	if pattern == nil || len(pattern.leadingAssertions) != 0 || pattern.expression == nil {
		return false
	}

	for _, alternative := range pattern.expression.alternatives {
		if stringPatternSequenceCanMatchEmpty(alternative) {
			return true
		}
	}

	return false
}

func stringPatternSequenceCanMatchEmpty(sequence *patternSequence) bool {
	if sequence == nil {
		return false
	}

	for _, term := range sequence.terms {
		if !stringPatternTermCanMatchEmpty(term) {
			return false
		}
	}

	return true
}

func stringPatternTermCanMatchEmpty(term *patternTerm) bool {
	if term == nil || term.atom == nil {
		return false
	}
	if term.quantified && term.minimum == 0 {
		return true
	}
	if term.atom.kind != patternGroup || term.atom.expression == nil {
		return false
	}

	for _, alternative := range term.atom.expression.alternatives {
		if stringPatternSequenceCanMatchEmpty(alternative) {
			return true
		}
	}

	return false
}

type stringMaximumLength struct {
	value uint64
	found bool
}

func (maximum *stringMaximumLength) add(value uint64) {
	if !maximum.found || value < maximum.value {
		maximum.value = value
		maximum.found = true
	}
}

func stringObjectiveMaximumLength(
	product stringProduct,
	objective stringObjective,
) (uint64, bool, error) {
	maximum := stringMaximumLength{}
	addStringObjectiveEnumMaximum(&maximum, product, objective)
	addStringObjectivePatternMaximum(&maximum, product, objective)
	addStringObjectiveFormatMaximum(&maximum, product, objective)
	if err := addStringObjectiveLengthMaximum(&maximum, product, objective); err != nil {
		return 0, false, err
	}

	return maximum.value, maximum.found, nil
}

func addStringObjectiveEnumMaximum(
	maximum *stringMaximumLength,
	product stringProduct,
	objective stringObjective,
) {
	for index, constraint := range product.enums {
		if !stringObjectiveRequiresConstraint(objective, stringConstraintRef{
			kind: stringConstraintEnum, index: index,
		}) {
			continue
		}

		enumMaximum := uint64(0)
		for _, member := range constraint.members {
			length := uint64(len([]rune(member.text)))
			if length > enumMaximum {
				enumMaximum = length
			}
		}
		maximum.add(enumMaximum)
	}
}

func addStringObjectivePatternMaximum(
	maximum *stringMaximumLength,
	product stringProduct,
	objective stringObjective,
) {
	for index, pattern := range product.patterns {
		if !pattern.finite || !stringObjectiveRequiresConstraint(objective, stringConstraintRef{
			kind:  stringConstraintPattern,
			index: index,
		}) {
			continue
		}

		maximum.add(pattern.maximum)
	}
}

func addStringObjectiveFormatMaximum(
	maximum *stringMaximumLength,
	product stringProduct,
	objective stringObjective,
) {
	for index, format := range product.formats {
		if !stringObjectiveRequiresConstraint(objective, stringConstraintRef{
			kind:  stringConstraintFormat,
			index: index,
		}) {
			continue
		}

		formatMaximum, finite := stringFormatMaximumLength(format.format)
		if finite {
			maximum.add(formatMaximum)
		}
	}
}

func addStringObjectiveLengthMaximum(
	maximum *stringMaximumLength,
	product stringProduct,
	objective stringObjective,
) error {
	for index, constraint := range product.lengths {
		truth, constrained := stringObjectiveConstraintTruth(objective, stringConstraintRef{
			kind:  stringConstraintLength,
			index: index,
		})
		if !constrained {
			continue
		}

		bound, fits, err := exactCountUint64(constraint.bound)
		if err != nil {
			return err
		}
		if !fits {
			continue
		}

		switch {
		case !truth && constraint.kind == stringRuleMinLength:
			if bound > 0 {
				maximum.add(bound - 1)
			} else {
				maximum.add(0)
			}
		case truth && constraint.kind == stringRuleMaxLength:
			maximum.add(bound)
		}
	}

	return nil
}

func uniqueStringPinnedLengths(values []stringPinnedLength) []stringPinnedLength {
	result := make([]stringPinnedLength, 0, len(values))
	for _, value := range values {
		found := false
		for _, existing := range result {
			if existing == value {
				found = true

				break
			}
		}

		if !found {
			result = append(result, value)
		}
	}

	return result
}

func containsUint64(values []uint64, wanted uint64) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}

	return false
}

func uniqueUint64(values []uint64) []uint64 {
	result := make([]uint64, 0, len(values))
	for _, value := range values {
		found := false
		for _, existing := range result {
			if existing == value {
				found = true

				break
			}
		}

		if !found {
			result = append(result, value)
		}
	}

	return result
}

type stringPatternLengthSummary struct {
	maximum       uint64
	startsAtInput bool
	endsAtInput   bool
}

// stringPatternMaximumRuneLength identifies full-input and leading-assertion bounds.
func stringPatternMaximumRuneLength(pattern *patternAST) (uint64, bool) {
	if pattern == nil || pattern.expression == nil {
		return 0, false
	}

	maximum, bounded := stringFullExpressionMaximum(pattern.expression)
	for _, assertion := range pattern.leadingAssertions {
		if !assertion.positive {
			continue
		}

		assertionMaximum, assertionBounded := stringLeadingAssertionMaximum(assertion.expression)
		if assertionBounded && (!bounded || assertionMaximum < maximum) {
			maximum = assertionMaximum
			bounded = true
		}
	}

	return maximum, bounded
}

func stringFullExpressionMaximum(expression *patternExpression) (uint64, bool) {
	summary, finite := stringExpressionLengthSummary(expression)
	if !finite || !summary.startsAtInput || !summary.endsAtInput {
		return 0, false
	}

	return summary.maximum, true
}

func stringLeadingAssertionMaximum(expression *patternExpression) (uint64, bool) {
	summary, finite := stringExpressionLengthSummary(expression)
	if !finite || !summary.endsAtInput {
		return 0, false
	}

	return summary.maximum, true
}

func stringExpressionLengthSummary(expression *patternExpression) (stringPatternLengthSummary, bool) {
	if expression == nil || len(expression.alternatives) == 0 {
		return stringPatternLengthSummary{}, false
	}

	summary := stringPatternLengthSummary{
		startsAtInput: true,
		endsAtInput:   true,
	}

	for _, alternative := range expression.alternatives {
		sequence, finite := stringSequenceLengthSummary(alternative)
		if !finite {
			return stringPatternLengthSummary{}, false
		}

		if sequence.maximum > summary.maximum {
			summary.maximum = sequence.maximum
		}

		summary.startsAtInput = summary.startsAtInput && sequence.startsAtInput
		summary.endsAtInput = summary.endsAtInput && sequence.endsAtInput
	}

	return summary, true
}

func stringSequenceLengthSummary(sequence *patternSequence) (stringPatternLengthSummary, bool) {
	if sequence == nil {
		return stringPatternLengthSummary{}, false
	}

	summary := stringPatternLengthSummary{}
	for index, term := range sequence.terms {
		termSummary, finite := stringTermLengthSummary(term)
		if !finite {
			return stringPatternLengthSummary{}, false
		}

		maximum, fits := addStringPatternLengths(summary.maximum, termSummary.maximum)
		if !fits {
			return stringPatternLengthSummary{}, false
		}

		summary.maximum = maximum
		if index == 0 {
			summary.startsAtInput = termSummary.startsAtInput
		}
	}

	for index := len(sequence.terms) - 1; index >= 0; index-- {
		termSummary, finite := stringTermLengthSummary(sequence.terms[index])
		if !finite {
			return stringPatternLengthSummary{}, false
		}

		if !termSummary.endsAtInput {
			continue
		}

		endsAtInput := true
		for _, following := range sequence.terms[index+1:] {
			followingSummary, followingFinite := stringTermLengthSummary(following)
			if !followingFinite || followingSummary.maximum != 0 {
				endsAtInput = false

				break
			}
		}

		if endsAtInput {
			summary.endsAtInput = true

			break
		}
	}

	return summary, true
}

func stringTermLengthSummary(term *patternTerm) (stringPatternLengthSummary, bool) {
	if term == nil || term.atom == nil {
		return stringPatternLengthSummary{}, false
	}

	atomSummary, finite := stringAtomLengthSummary(term.atom)
	if !finite {
		return stringPatternLengthSummary{}, false
	}

	if !term.quantified {
		return atomSummary, true
	}

	if term.unbounded {
		if atomSummary.maximum != 0 {
			return stringPatternLengthSummary{}, false
		}

		atomSummary.startsAtInput = false
		atomSummary.endsAtInput = false

		return atomSummary, true
	}

	maximum, fits := multiplyStringPatternLengths(atomSummary.maximum, term.maximum)
	if !fits {
		return stringPatternLengthSummary{}, false
	}

	return stringPatternLengthSummary{
		maximum:       maximum,
		startsAtInput: atomSummary.startsAtInput && term.minimum == 1 && term.maximum == 1,
		endsAtInput:   atomSummary.endsAtInput && term.minimum == 1 && term.maximum == 1,
	}, true
}

func stringAtomLengthSummary(atom *patternAtom) (stringPatternLengthSummary, bool) {
	if atom == nil {
		return stringPatternLengthSummary{}, false
	}

	summary := stringPatternLengthSummary{}
	switch atom.kind {
	case patternLiteral, patternDot, patternClassAtom:
		summary.maximum = 1
	case patternStart:
		summary.startsAtInput = true
	case patternEnd:
		summary.endsAtInput = true
	case patternWordBoundary, patternNotWordBoundary:
	case patternGroup:
		return stringExpressionLengthSummary(atom.expression)
	default:
		return stringPatternLengthSummary{}, false
	}

	return summary, true
}

func addStringPatternLengths(left, right uint64) (uint64, bool) {
	if right > maxStringSearchUint64-left {
		return 0, false
	}

	return left + right, true
}

func multiplyStringPatternLengths(left, right uint64) (uint64, bool) {
	if left != 0 && right > maxStringSearchUint64/left {
		return 0, false
	}

	return left * right, true
}

// stringPatternExactRuneLengths identifies finite literal-only pattern lengths.
func stringPatternExactRuneLengths(pattern *patternAST) ([]uint64, bool) {
	if pattern == nil || pattern.expression == nil {
		return nil, false
	}

	lengths, exact := stringExpressionExactRuneLengths(pattern.expression)
	if !exact {
		return nil, false
	}

	return uniqueUint64(lengths), true
}

func stringExpressionExactRuneLengths(expression *patternExpression) ([]uint64, bool) {
	if expression == nil {
		return nil, false
	}

	lengths := make([]uint64, 0, len(expression.alternatives))
	for _, alternative := range expression.alternatives {
		length, exact := stringSequenceExactRuneLength(alternative)
		if !exact {
			return nil, false
		}

		lengths = append(lengths, length)
	}

	return lengths, true
}

func stringSequenceExactRuneLength(sequence *patternSequence) (uint64, bool) {
	if sequence == nil {
		return 0, false
	}

	hasStart := false
	hasEnd := false
	units := make([]uint16, 0)
	for _, term := range sequence.terms {
		if term == nil || term.atom == nil {
			return 0, false
		}

		if term.quantified && (term.unbounded || term.minimum != term.maximum) {
			return 0, false
		}

		count := uint64(1)
		if term.quantified {
			count = term.minimum
		}

		switch term.atom.kind {
		case patternStart:
			if count != 1 {
				return 0, false
			}

			hasStart = true
		case patternEnd:
			if count != 1 {
				return 0, false
			}

			hasEnd = true
		case patternWordBoundary, patternNotWordBoundary:
			if count != 1 {
				return 0, false
			}
		case patternLiteral:
			for range count {
				units = append(units, term.atom.literal)
			}
		default:
			return 0, false
		}
	}

	if !hasStart || !hasEnd {
		return 0, false
	}

	decoded := utf16.Decode(units)
	for _, decodedRune := range decoded {
		if decodedRune == unicode.ReplacementChar && !stringUnitsRoundTrip(units) {
			return 0, false
		}
	}

	return uint64(len(decoded)), true
}

func stringUnitsRoundTrip(units []uint16) bool {
	encoded := utf16.Encode([]rune(utf16.Decode(units)))
	if len(encoded) != len(units) {
		return false
	}

	for index := range encoded {
		if encoded[index] != units[index] {
			return false
		}
	}

	return true
}
