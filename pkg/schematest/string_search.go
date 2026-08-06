//nolint:cyclop,gocognit,godoclint,mnd,wsl_v5 // The private product search keeps objective and frontier order explicit.
package schematest

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"unicode"
	"unicode/utf16"
)

const maxStringSearchUint64 = ^uint64(0)

type stringRuleKind uint8

const (
	stringRuleMinLength stringRuleKind = iota
	stringRuleMaxLength
)

type stringPatternAssertionConstraint struct {
	positive bool
	graph    *stringPatternGraph
}

type stringPatternConstraint struct {
	identity   ruleIdentity
	pattern    *patternAST
	graph      *stringPatternGraph
	assertions []stringPatternAssertionConstraint

	finite  bool
	maximum uint64
}

type stringFormatConstraint struct {
	identity ruleIdentity
	format   schemaFormat
}

type stringLengthConstraint struct {
	identity ruleIdentity
	kind     stringRuleKind
	bound    *exactCount
}

type stringPinnedLength struct {
	value uint64
	kind  stringObjectiveKind
	index int
}

type stringProduct struct {
	model *schemaModel

	patterns []stringPatternConstraint
	formats  []stringFormatConstraint
	lengths  []stringLengthConstraint

	fixedLengths []stringPinnedLength
	seedUnits    []uint16
}

type stringObjectiveKind uint8

const (
	stringObjectiveAllTrue stringObjectiveKind = iota
	stringObjectivePatternFalse
	stringObjectiveFormatFalse
	stringObjectiveLengthFalse
)

type stringObjective struct {
	kind  stringObjectiveKind
	index int
	rule  string
	level string
}

type stringProductRuntime struct {
	patterns []stringPatternRuntime
}

type stringProductTransition struct {
	interval         stringUnitInterval
	targets          [][]int
	assertionTargets [][][]int
}

// buildStringProduct collects every string rule active in one target state.
func buildStringProduct(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
) (stringProduct, error) {
	if node == nil || node.schemaShape == nil {
		return stringProduct{}, errors.New("string product has no schema shape")
	}

	product := stringProduct{}
	if err := collectStringProductRules(
		&product, node, occurrence, pins, make(map[*schemaNode]bool),
	); err != nil {
		return stringProduct{}, err
	}

	for index := range product.patterns {
		pattern := &product.patterns[index]
		graph, err := compileStringPatternGraph(pattern.pattern)
		if err != nil {
			return stringProduct{}, fmt.Errorf("compile string pattern %s: %w", pattern.identity, err)
		}

		pattern.graph = graph
		for _, assertion := range pattern.pattern.leadingAssertions {
			assertionGraph, assertionErr := compileStringPatternExpressionGraph(assertion.expression, true)
			if assertionErr != nil {
				return stringProduct{}, fmt.Errorf("compile string assertion %s: %w", pattern.identity, assertionErr)
			}

			pattern.assertions = append(pattern.assertions, stringPatternAssertionConstraint{
				positive: assertion.positive,
				graph:    assertionGraph,
			})
		}

		product.seedUnits = appendUniqueUint16(
			product.seedUnits,
			stringPatternSeedUnits(product.patterns[index].pattern)...,
		)
	}

	for index, format := range product.formats {
		for _, length := range stringFormatPinnedLengths(format.format) {
			product.fixedLengths = append(product.fixedLengths, stringPinnedLength{
				value: length,
				kind:  stringObjectiveFormatFalse,
				index: index,
			})
		}
		product.seedUnits = appendUniqueUint16(
			product.seedUnits,
			stringFormatSeedUnits(format.format)...,
		)
	}

	for index, pattern := range product.patterns {
		if maximum, finite := stringPatternMaximumRuneLength(pattern.pattern); finite {
			product.patterns[index].finite = true
			product.patterns[index].maximum = maximum
		}

		if lengths, exact := stringPatternExactRuneLengths(pattern.pattern); exact {
			for _, length := range lengths {
				product.fixedLengths = append(product.fixedLengths, stringPinnedLength{
					value: length,
					kind:  stringObjectivePatternFalse,
					index: index,
				})
			}
		}
	}

	product.fixedLengths = uniqueStringPinnedLengths(product.fixedLengths)

	return product, nil
}

func collectStringProductRules(
	product *stringProduct,
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	visiting map[*schemaNode]bool,
) error {
	if node == nil || node.schemaShape == nil {
		return errors.New("string product child has no schema shape")
	}

	if visiting[node] {
		return fmt.Errorf("recursive string product at %s", occurrence.usePointer)
	}

	visiting[node] = true
	defer delete(visiting, node)

	if nodeCanHaveKind(node, jsonString) {
		if node.minLength != nil {
			product.lengths = append(product.lengths, stringLengthConstraint{
				identity: makeRuleIdentity(occurrence, oracleRuleMinLength),
				kind:     stringRuleMinLength,
				bound:    node.minLength,
			})
		}

		if node.maxLength != nil {
			product.lengths = append(product.lengths, stringLengthConstraint{
				identity: makeRuleIdentity(occurrence, oracleRuleMaxLength),
				kind:     stringRuleMaxLength,
				bound:    node.maxLength,
			})
		}

		if node.pattern != nil {
			product.patterns = append(product.patterns, stringPatternConstraint{
				identity: makeRuleIdentity(occurrence, oracleRulePattern),
				pattern:  node.pattern,
			})
		}

		if isStringSchemaFormat(node.format) && node.format != schemaFormatPassword {
			product.formats = append(product.formats, stringFormatConstraint{
				identity: makeRuleIdentity(occurrence, oracleRuleFormat),
				format:   node.format,
			})
		}
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if err := collectStringProductRules(product, child, childOccurrence, pins, visiting); err != nil {
			return err
		}
	}

	states, pinned := rowCompositionTruthStates(pins, occurrence, "anyOf", len(node.anyOf))
	for index, child := range node.anyOf {
		if pinned && !states[index] {
			continue
		}

		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/anyOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if err := collectStringProductRules(product, child, childOccurrence, pins, visiting); err != nil {
			return err
		}
	}

	return nil
}

func stringProductObjectives(product stringProduct, rule, level string) []stringObjective {
	objectives := []stringObjective{{kind: stringObjectiveAllTrue, rule: rule, level: level}}

	for index, pattern := range product.patterns {
		objectives = append(objectives, stringObjective{
			kind:  stringObjectivePatternFalse,
			index: index,
			rule:  pattern.identity.rule,
			level: "false",
		})
	}

	for index, format := range product.formats {
		objectives = append(objectives, stringObjective{
			kind:  stringObjectiveFormatFalse,
			index: index,
			rule:  format.identity.rule,
			level: "false",
		})
	}

	for index, length := range product.lengths {
		objectives = append(objectives, stringObjective{
			kind:  stringObjectiveLengthFalse,
			index: index,
			rule:  length.identity.rule,
			level: "false",
		})
	}

	return objectives
}

// walkString searches the all-true string objective for one row target.
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

	if len(product.patterns) == 0 && len(product.formats) == 0 && len(product.lengths) == 0 {
		candidates, candidateErr := rowScalarValues(node, jsonString)
		if candidateErr != nil {
			return false, candidateErr
		}

		for _, candidate := range candidates {
			if err := s.assign(); err != nil {
				return false, err
			}

			complete, visitErr := visit(candidate)
			if visitErr != nil || complete {
				return complete, visitErr
			}
		}

		return false, nil
	}

	if s.model != nil && s.model.canonicalSchemaJSON == "" && s.model.schemaValue != nil {
		canonical, canonicalErr := marshalCanonicalJSON(s.model.schemaValue)
		if canonicalErr != nil {
			return false, fmt.Errorf("canonicalize string schema: %w", canonicalErr)
		}

		s.model.canonicalSchemaJSON = string(canonical)
	}

	product.model = s.model
	objectives := stringProductObjectives(product, s.stringRule, s.stringLevel)
	if len(objectives) == 0 {
		return false, nil
	}

	return s.searchStringObjective(product, objectives[0], visit)
}

// searchStringObjectives streams the ordered clean objectives without retaining witnesses.
func (s *search) searchStringObjectives(
	product stringProduct,
	rule, level string,
	visit func(stringObjective, *jsonValue) (bool, error),
) error {
	if visit == nil {
		return errors.New("nil string objective callback")
	}

	for _, objective := range stringProductObjectives(product, rule, level) {
		_, err := s.searchStringObjective(product, objective, func(value *jsonValue) (bool, error) {
			return visit(objective, value)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *search) searchStringObjective(
	product stringProduct,
	objective stringObjective,
	visit rowVisit,
) (bool, error) {
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

		return s.walkStringLength(
			product,
			objective,
			runtime,
			length,
			0,
			0,
			false,
			nil,
			nil,
			visit,
		)
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

func (s *search) walkStringLength(
	product stringProduct,
	objective stringObjective,
	runtime stringProductRuntime,
	targetLength uint64,
	position uint64,
	runeLength uint64,
	hasPrevious bool,
	previous *uint16,
	units []uint16,
	visit rowVisit,
) (bool, error) {
	pendingHigh := len(units) > 0 && units[len(units)-1] >= 0xd800 && units[len(units)-1] <= 0xdbff
	if runeLength == targetLength && !pendingHigh {
		return s.finishStringCandidate(
			product, objective, runtime, position, previous, hasPrevious,
			units, visit,
		)
	}

	if runeLength > targetLength {
		return false, nil
	}

	previousUnit := uint16(0)
	if hasPrevious && previous != nil {
		previousUnit = *previous
	}

	transitions := stringProductTransitions(
		product, objective, runtime, units, int(position), previousUnit, hasPrevious,
	)
	for _, transition := range transitions {
		candidates := stringIntervalCandidates(
			transition.interval,
			s.stringSearchSeedForInterval(product, objective, transition.interval),
			product.seedUnits,
		)

		for _, unit := range candidates {
			if pendingHigh {
				if unit < 0xdc00 || unit > 0xdfff {
					continue
				}
			} else if unit >= 0xdc00 && unit <= 0xdfff {
				continue
			}

			if err := s.assign(); err != nil {
				return false, err
			}

			nextUnits := append(append([]uint16(nil), units...), unit)
			nextRuntime := stringProductRuntime{
				patterns: make([]stringPatternRuntime, len(runtime.patterns)),
			}

			nextRuneLength := runeLength
			if !pendingHigh {
				nextRuneLength++
			}
			nextPendingHigh := unit >= 0xd800 && unit <= 0xdbff
			nextAtEnd := nextRuneLength == targetLength && !nextPendingHigh

			invalidAssertion := false
			for index, pattern := range runtime.patterns {
				nextPattern := stringPatternRuntime{
					graph: pattern.graph,
					raw:   append([]int(nil), transition.targets[index]...),
				}
				if objective.kind == stringObjectivePatternFalse && objective.index == index {
					nextRuntime.patterns[index] = nextPattern

					continue
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
						nextAssertion.matched = nextAssertion.acceptsAt(
							int(position+1), unit, true, nil, nextAtEnd,
						)
						if nextAssertion.matched && !nextAssertion.positive {
							invalidAssertion = true
						}
					}

					nextPattern.assertions = append(nextPattern.assertions, nextAssertion)
				}

				nextRuntime.patterns[index] = nextPattern
			}

			if invalidAssertion {
				continue
			}

			complete, err := s.walkStringLength(
				product,
				objective,
				nextRuntime,
				targetLength,
				position+1,
				nextRuneLength,
				true,
				&unit,
				nextUnits,
				visit,
			)
			if err != nil || complete {
				return complete, err
			}
		}
	}

	return false, nil
}

//nolint:gocyclo // Pattern, assertion, and format frontiers share one interval partition.
func stringProductTransitions(
	product stringProduct,
	objective stringObjective,
	runtime stringProductRuntime,
	prefix []uint16,
	position int,
	previous uint16,
	hasPrevious bool,
) []stringProductTransition {
	patternTransitions := make([][]stringPatternTransition, len(runtime.patterns))
	assertionTransitions := make([][][]stringPatternTransition, len(runtime.patterns))
	boundaries := []int{0, stringUTF16UnitCount}

	for index, pattern := range runtime.patterns {
		if objective.kind == stringObjectivePatternFalse && objective.index == index {
			continue
		}

		patternTransitions[index] = pattern.outgoing(position, previous, hasPrevious)
		for _, transition := range patternTransitions[index] {
			boundaries = append(
				boundaries,
				int(transition.interval.low),
				int(transition.interval.high)+1,
			)
		}

		assertionTransitions[index] = make([][]stringPatternTransition, len(pattern.assertions))
		for assertionIndex, assertion := range pattern.assertions {
			if assertion.matched {
				continue
			}

			assertionTransitions[index][assertionIndex] = stringPatternRuntime{
				graph: assertion.graph,
				raw:   assertion.raw,
			}.outgoing(position, previous, hasPrevious)
			for _, transition := range assertionTransitions[index][assertionIndex] {
				boundaries = append(
					boundaries,
					int(transition.interval.low),
					int(transition.interval.high)+1,
				)
			}
		}
	}

	formatTransitions := make([][]stringUnitInterval, len(product.formats))
	for index, format := range product.formats {
		if objective.kind == stringObjectiveFormatFalse && objective.index == index {
			continue
		}

		formatTransitions[index] = stringFormatIntervals(format.format, prefix)
		for _, interval := range formatTransitions[index] {
			boundaries = append(boundaries, int(interval.low), int(interval.high)+1)
		}
	}

	boundaries = sortedInts(boundaries)
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

		unit := uint16(low)
		allowed := true
		for formatIndex, intervals := range formatTransitions {
			if objective.kind == stringObjectiveFormatFalse && objective.index == formatIndex {
				continue
			}

			if !intervalContains(intervals, unit) {
				allowed = false

				break
			}
		}

		if !allowed {
			continue
		}

		targets := make([][]int, len(patternTransitions))
		assertionTargets := make([][][]int, len(patternTransitions))
		for patternIndex, candidates := range patternTransitions {
			if objective.kind == stringObjectivePatternFalse && objective.index == patternIndex {
				continue
			}

			for _, candidate := range candidates {
				if unit < candidate.interval.low || unit > candidate.interval.high {
					continue
				}

				targets[patternIndex] = candidate.targets

				break
			}

			if len(targets[patternIndex]) == 0 {
				allowed = false

				break
			}

			assertionTargets[patternIndex] = make([][]int, len(assertionTransitions[patternIndex]))
			for assertionIndex, candidates := range assertionTransitions[patternIndex] {
				if runtime.patterns[patternIndex].assertions[assertionIndex].matched {
					continue
				}

				for _, candidate := range candidates {
					if unit < candidate.interval.low || unit > candidate.interval.high {
						continue
					}

					assertionTargets[patternIndex][assertionIndex] = candidate.targets

					break
				}

				if len(assertionTargets[patternIndex][assertionIndex]) == 0 &&
					runtime.patterns[patternIndex].assertions[assertionIndex].positive {
					allowed = false

					break
				}
			}

			if !allowed {
				break
			}
		}

		if !allowed {
			continue
		}

		transition := stringProductTransition{
			interval:         stringUnitInterval{low: unit, high: uint16(high)},
			targets:          copyPatternTargets(targets),
			assertionTargets: copyPatternAssertionTargets(assertionTargets),
		}
		if len(transitions) > 0 &&
			transitions[len(transitions)-1].interval.high+1 == transition.interval.low &&
			patternTargetsEqual(
				transitions[len(transitions)-1].targets,
				transition.targets,
				transitions[len(transitions)-1].assertionTargets,
				transition.assertionTargets,
			) {
			transitions[len(transitions)-1].interval.high = transition.interval.high

			continue
		}

		transitions = append(transitions, transition)
	}

	return transitions
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
	previous *uint16,
	hasPrevious bool,
	units []uint16,
	visit rowVisit,
) (bool, error) {
	previousUnit := uint16(0)
	if hasPrevious && previous != nil {
		previousUnit = *previous
	}

	if position > uint64(^uint(0)>>1) {
		return false, errors.New("string graph position overflows int")
	}

	for index, pattern := range runtime.patterns {
		if objective.kind == stringObjectivePatternFalse && objective.index == index {
			continue
		}

		if !pattern.accepts(int(position), previousUnit, hasPrevious) {
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

	return visit(&jsonValue{kind: jsonString, text: text})
}

func stringObjectiveMatches(
	product stringProduct,
	objective stringObjective,
	text string,
) (bool, error) {
	for index, pattern := range product.patterns {
		matches, err := cleanPatternMatches(pattern.pattern, text)
		if err != nil {
			return false, err
		}

		want := true
		if objective.kind == stringObjectivePatternFalse && objective.index == index {
			want = false
		}

		if matches != want {
			return false, nil
		}
	}

	for index, format := range product.formats {
		matches, err := cleanStringFormatMatches(text, format.format)
		if err != nil {
			return false, err
		}

		want := true
		if objective.kind == stringObjectiveFormatFalse && objective.index == index {
			want = false
		}

		if matches != want {
			return false, nil
		}
	}

	length := uint64(len([]rune(text)))
	for index, constraint := range product.lengths {
		matches, err := stringLengthConstraintMatches(constraint, length)
		if err != nil {
			return false, err
		}

		if objective.kind == stringObjectiveLengthFalse && objective.index == index {
			matches = !matches
		}

		if !matches {
			return false, nil
		}
	}

	return true, nil
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
		matches, err := stringLengthConstraintMatches(constraint, length)
		if err != nil {
			return false, err
		}

		if objective.kind == stringObjectiveLengthFalse && objective.index == index {
			matches = !matches
		}

		if !matches {
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
		if objective.kind == pinned.kind && objective.index == pinned.index {
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

	for index, constraint := range product.lengths {
		if constraint.bound == nil || constraint.bound.number == nil {
			continue
		}

		matches, err := stringLengthMatchesAllExcept(product, objective, index, 0)
		if err != nil {
			return nil, err
		}

		if !matches {
			continue
		}

		bound, fits, err := exactCountUint64(constraint.bound)
		if err != nil {
			return nil, err
		}
		if !fits {
			continue
		}

		switch {
		case objective.kind == stringObjectiveLengthFalse && objective.index == index &&
			constraint.kind == stringRuleMinLength:
			if bound > 0 {
				appendLength(bound - 1)
			}
		case objective.kind == stringObjectiveLengthFalse && objective.index == index &&
			constraint.kind == stringRuleMaxLength:
			if bound < maxStringSearchUint64 {
				appendLength(bound + 1)
			}
		default:
			appendLength(bound)
		}
	}

	return lengths, nil
}

func stringLengthMatchesAllExcept(
	product stringProduct,
	objective stringObjective,
	excluded int,
	length uint64,
) (bool, error) {
	for index, constraint := range product.lengths {
		if index == excluded {
			continue
		}

		matches, err := stringLengthConstraintMatches(constraint, length)
		if err != nil {
			return false, err
		}

		if objective.kind == stringObjectiveLengthFalse && objective.index == index {
			matches = !matches
		}

		if !matches {
			return false, nil
		}
	}

	return true, nil
}

func stringObjectiveMaximumLength(
	product stringProduct,
	objective stringObjective,
) (uint64, bool, error) {
	var maximum uint64
	found := false

	patternMaximum := uint64(0)
	patternFound := false
	patternsBounded := true
	for index, pattern := range product.patterns {
		if objective.kind == stringObjectivePatternFalse && objective.index == index {
			continue
		}

		if !pattern.finite {
			patternsBounded = false

			break
		}

		if !patternFound || pattern.maximum < patternMaximum {
			patternMaximum = pattern.maximum
			patternFound = true
		}
	}

	if patternsBounded && patternFound {
		maximum = patternMaximum
		found = true
	}

	for _, format := range product.formats {
		formatMaximum, formatFinite := stringFormatMaximumLength(format.format)
		if !formatFinite {
			continue
		}

		if !found || formatMaximum < maximum {
			maximum = formatMaximum
			found = true
		}
	}

	for index, constraint := range product.lengths {
		if constraint.kind != stringRuleMaxLength ||
			objective.kind == stringObjectiveLengthFalse && objective.index == index {
			continue
		}

		bound, fits, err := exactCountUint64(constraint.bound)
		if err != nil {
			return 0, false, err
		}

		if !fits {
			return 0, false, nil
		}

		if !found || bound < maximum {
			maximum = bound
			found = true
		}
	}

	return maximum, found, nil
}

func (s *search) stringSearchSeedForInterval(
	product stringProduct,
	objective stringObjective,
	interval stringUnitInterval,
) uint64 {
	seed := stringSearchSeed(product.model, objective)
	seed ^= uint64(interval.low)<<16 | uint64(interval.high)
	seed ^= seed >> 29
	seed *= 0x9e3779b97f4a7c15
	seed ^= seed >> 32

	return seed
}

func stringSearchSeed(model *schemaModel, objective stringObjective) uint64 {
	schemaPointer := ""
	canonicalSchemaJSON := ""
	if model != nil {
		schemaPointer = model.schemaPointer
		canonicalSchemaJSON = model.canonicalSchemaJSON
	}

	payload := []byte("schematest-v1\x00")
	payload = append(payload, schemaPointer...)
	payload = append(payload, 0)
	payload = append(payload, canonicalSchemaJSON...)
	payload = append(payload, 0)
	payload = append(payload, objective.rule...)
	payload = append(payload, 0)
	payload = append(payload, objective.level...)

	digest := sha256.Sum256(payload)

	return binary.BigEndian.Uint64(digest[:8])
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

// stringPatternMaximumRuneLength identifies anchored patterns with bounded repetition.
func stringPatternMaximumRuneLength(pattern *patternAST) (uint64, bool) {
	if pattern == nil || pattern.expression == nil {
		return 0, false
	}

	summary, finite := stringExpressionLengthSummary(pattern.expression)
	if !finite || !summary.startsAtInput || !summary.endsAtInput {
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
