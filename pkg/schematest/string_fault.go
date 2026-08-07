//nolint:godoclint // Private directed-objective types are documented at their search seams.
package schematest

import (
	"errors"
	"fmt"
)

type stringSearchObjectiveKind uint8

const (
	stringSearchAllTrue stringSearchObjectiveKind = iota
	stringSearchPatternFalse
	stringSearchFormatFalse
	stringSearchMinLengthFalse
	stringSearchMaxLengthFalse
)

type stringSearchObjective struct {
	kind       stringSearchObjectiveKind
	occurrence schemaOccurrence
	closure    []failureIdentity
	rule       string
	level      string
}

type activeStringPattern struct {
	pattern    *patternAST
	node       *schemaNode
	occurrence schemaOccurrence
}

type activeStringLength struct {
	count      *exactCount
	node       *schemaNode
	occurrence schemaOccurrence
	minimum    bool
}

type activeStringRules struct {
	patterns  []activeStringPattern
	lengths   []activeStringLength
	formats   []activeStringFormat
	supported bool
}

func activeStringRulesFor(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	objective *stringSearchObjective,
) (activeStringRules, error) {
	rules := activeStringRules{supported: true}
	if err := collectActiveStringRules(node, occurrence, pins, objective, &rules); err != nil {
		return activeStringRules{}, err
	}

	return rules, nil
}

//nolint:cyclop,nestif // All string rules share one allOf/anyOf applicability traversal.
func collectActiveStringRules(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	objective *stringSearchObjective,
	rules *activeStringRules,
) error {
	if node == nil || node.schemaShape == nil {
		return errors.New("schematest: active string schema has no shape")
	}

	if node.minLength != nil {
		rules.lengths = append(rules.lengths, activeStringLength{
			count: node.minLength, node: node, occurrence: occurrence, minimum: true,
		})
	}

	if node.maxLength != nil {
		rules.lengths = append(rules.lengths, activeStringLength{
			count: node.maxLength, node: node, occurrence: occurrence,
		})
	}

	if node.pattern != nil {
		if _, _, ok := basicStringExpressionLength(node.pattern.expression); !ok {
			rules.supported = false
		} else {
			for _, assertion := range node.pattern.leadingAssertions {
				if _, _, ok := basicStringExpressionLength(assertion.expression); !ok {
					rules.supported = false

					break
				}
			}
		}

		if rules.supported {
			rules.patterns = append(rules.patterns, activeStringPattern{
				pattern: node.pattern, node: node, occurrence: occurrence,
			})
		}
	}

	if len(simpleStringFormatWitnesses(node.format, true)) > 0 {
		rules.formats = append(rules.formats, activeStringFormat{
			format: node.format, node: node, occurrence: occurrence,
		})
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child, occurrence.usePointer+"/allOf/"+itoa(index), occurrence.instanceTemplate,
		)
		if err := collectActiveStringRules(child, childOccurrence, pins, objective, rules); err != nil {
			return err
		}
	}

	states, pinned := rowCompositionTruthStates(pins, occurrence, "anyOf", len(node.anyOf))
	for index, child := range node.anyOf {
		childOccurrence := rebasePlanOccurrence(
			child, occurrence.usePointer+"/anyOf/"+itoa(index), occurrence.instanceTemplate,
		)
		if !pinned || !states[index] && !stringObjectiveWithin(objective, childOccurrence) {
			continue
		}

		if err := collectActiveStringRules(child, childOccurrence, pins, objective, rules); err != nil {
			return err
		}
	}

	return nil
}

//nolint:cyclop // Objective admission, scalar traversal, and exact closure verification form one seam.
func findStringFaultRow(target faultTarget, searchState *search) (*jsonValue, bool, error) {
	if searchState == nil || searchState.model == nil || searchState.model.root == nil {
		return nil, false, errors.New("schematest: string fault search has no model")
	}

	kind, ok := stringFaultObjectiveKind(target.obligation.rule)
	if !ok {
		return nil, false, fmt.Errorf("schematest: %s is not a string fault", target.obligation.rule)
	}

	objective := &stringSearchObjective{
		kind:       kind,
		occurrence: target.obligation.occurrence,
		closure:    append([]failureIdentity(nil), target.closure...),
		rule:       target.obligation.rule,
		level:      target.obligation.component,
	}

	root := searchState.model.root
	if !nodeAcceptsKindForTarget(root, jsonString) {
		return nil, false, nil
	}

	var found *jsonValue

	handled, complete, err := searchState.walkDirectedStringObjective(
		root,
		root.occurrence,
		target.pins,
		objective,
		func(value *jsonValue) (bool, error) {
			result := evaluateNode(root, value, root.occurrence)
			if result.err != nil {
				return false, fmt.Errorf("evaluate directed string fault: %w", result.err)
			}

			matches, matchErr := exactStringFailureClosure(result.failures, target.closure)
			if matchErr != nil || !matches {
				return false, matchErr
			}

			found = value

			return true, nil
		},
	)
	if err != nil {
		return nil, false, err
	}

	if !handled {
		return nil, false, nil
	}

	return found, complete, nil
}

func stringFaultObjectiveKind(rule string) (stringSearchObjectiveKind, bool) {
	switch rule {
	case oracleRulePattern:
		return stringSearchPatternFalse, true
	case oracleRuleFormat:
		return stringSearchFormatFalse, true
	case oracleRuleMinLength:
		return stringSearchMinLengthFalse, true
	case oracleRuleMaxLength:
		return stringSearchMaxLengthFalse, true
	default:
		return stringSearchAllTrue, false
	}
}

func exactStringFailureClosure(actual, expected []failureIdentity) (bool, error) {
	canonicalActual, err := canonicalFailureClosure(actual)
	if err != nil {
		return false, err
	}

	canonicalExpected, err := canonicalFailureClosure(expected)
	if err != nil {
		return false, err
	}

	if len(canonicalActual) != len(canonicalExpected) {
		return false, nil
	}

	matched := make([]bool, len(canonicalActual))

	for _, expectedFailure := range canonicalExpected {
		found := false

		for index, actualFailure := range canonicalActual {
			if matched[index] || actualFailure.rule != expectedFailure.rule ||
				!ruleOccurrenceMatches(actualFailure.occurrence, expectedFailure.occurrence) {
				continue
			}

			matched[index] = true
			found = true

			break
		}

		if !found {
			return false, nil
		}
	}

	return true, nil
}

//nolint:cyclop // Pattern, length, seed, and failure-alternative phases share one objective seam.
func (s *search) walkDirectedStringObjective(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	objective *stringSearchObjective,
	visit rowVisit,
) (bool, bool, error) {
	if objective == nil {
		return false, false, nil
	}

	rules, rulesErr := activeStringRulesFor(node, occurrence, pins, objective)
	if rulesErr != nil {
		return true, false, rulesErr
	}

	if !rules.supported {
		return true, false, errors.New("schematest: directed pattern is not searchable")
	}

	patterns := rules.patterns
	lengthConstraints := rules.lengths
	formats := rules.formats

	patternIndex := directedStringPatternIndex(patterns, objective)
	formatIndex := directedStringFormatIndex(formats, objective)

	lengthIndex := directedStringLengthIndex(lengthConstraints, objective)
	switch objective.kind {
	case stringSearchPatternFalse:
		if patternIndex < 0 {
			return false, false, nil
		}
	case stringSearchFormatFalse:
		if formatIndex < 0 {
			return false, false, nil
		}
	default:
		if lengthIndex < 0 {
			return false, false, nil
		}
	}

	patternASTs := make([]*patternAST, 0, len(patterns))
	for _, pattern := range patterns {
		patternASTs = append(patternASTs, pattern.pattern)
	}

	var (
		lengths         basicStringLengths
		lengthObjective basicStringLengthObjective
		possible        bool
		err             error
	)
	if objective.kind == stringSearchFormatFalse {
		lengths, err = basicStringLengthsFromActive(lengthConstraints)
		possible = err == nil
	} else {
		lengths, lengthObjective, possible, err = directedBasicStringLengths(
			lengthConstraints, lengthIndex, objective,
		)
	}

	if err != nil || !possible {
		return true, false, err
	}

	var targetNode *schemaNode

	switch objective.kind {
	case stringSearchPatternFalse:
		targetNode = patterns[patternIndex].node
	case stringSearchFormatFalse:
		targetNode = formats[formatIndex].node
	default:
		targetNode = lengthConstraints[lengthIndex].node
	}

	canonicalSchemaJSON, err := marshalStrict(targetNode.schemaJSON)
	if err != nil {
		return true, false, fmt.Errorf("schematest: canonicalize directed string schema: %w", err)
	}

	seed := basicStringSeed(
		objective.occurrence.usePointer,
		canonicalSchemaJSON,
		objective.rule,
		objective.level,
	)

	if objective.kind != stringSearchPatternFalse {
		product, productErr := newDirectedLengthProduct(patternASTs)
		if productErr != nil {
			return true, false, productErr
		}

		if formatErr := product.addFormats(formats, formatIndex); formatErr != nil {
			return true, false, formatErr
		}

		complete, walkErr := s.walkBasicStringProductForLengths(
			product,
			lengths,
			lengthObjective,
			seed,
			visit,
		)

		return true, complete, walkErr
	}

	alternativeCount := len(patterns[patternIndex].pattern.leadingAssertions) + 1
	for alternative := 0; alternative < alternativeCount; alternative++ {
		product, productErr := newBasicStringProductForFailure(patternASTs, patternIndex, alternative)
		if productErr != nil {
			return true, false, productErr
		}

		if formatErr := product.addFormats(formats, -1); formatErr != nil {
			return true, false, formatErr
		}

		complete, walkErr := s.walkBasicStringProductForLengths(
			product,
			lengths,
			lengthObjective,
			seed,
			visit,
		)
		if walkErr != nil || complete {
			return true, complete, walkErr
		}
	}

	return true, false, nil
}

func newDirectedLengthProduct(patterns []*patternAST) (*basicStringProduct, error) {
	return newBasicStringProduct(patterns)
}

func directedStringPatternIndex(patterns []activeStringPattern, objective *stringSearchObjective) int {
	for index, pattern := range patterns {
		if rowOccurrenceMatches(pattern.occurrence, objective.occurrence) {
			return index
		}
	}

	return -1
}

func directedStringLengthIndex(lengths []activeStringLength, objective *stringSearchObjective) int {
	minimum := objective.kind == stringSearchMinLengthFalse
	for index, length := range lengths {
		if length.minimum == minimum && rowOccurrenceMatches(length.occurrence, objective.occurrence) {
			return index
		}
	}

	return -1
}

//nolint:cyclop // Exact lower and upper failure ranges require distinct overflow handling.
func directedBasicStringLengths(
	constraints []activeStringLength,
	directed int,
	objective *stringSearchObjective,
) (basicStringLengths, basicStringLengthObjective, bool, error) {
	var lengths basicStringLengths

	for index, constraint := range constraints {
		if index == directed {
			continue
		}

		var err error
		if constraint.minimum {
			err = lengths.addMinimum(constraint.count)
		} else {
			err = lengths.addMaximum(constraint.count)
		}

		if err != nil {
			return basicStringLengths{}, basicStringLengthObjective{}, false, err
		}
	}

	if objective.kind == stringSearchPatternFalse {
		return lengths, basicStringLengthObjective{}, true, nil
	}

	if directed < 0 {
		return basicStringLengths{}, basicStringLengthObjective{}, false, nil
	}

	bound, fits, err := exactCountUint64(constraints[directed].count)
	if err != nil {
		return basicStringLengths{}, basicStringLengthObjective{}, false, err
	}

	lengthObjective := basicStringLengthObjective{pinned: true}

	switch objective.kind {
	case stringSearchMinLengthFalse:
		if fits {
			if bound == 0 {
				return basicStringLengths{}, basicStringLengthObjective{}, false, nil
			}

			lengths.maximum = bound - 1
			lengths.hasMaximum = true
			lengthObjective.length = bound - 1
		} else {
			lengths.maximum = ^uint64(0)
			lengths.hasMaximum = true
			lengthObjective.length = lengths.minimum
		}
	case stringSearchMaxLengthFalse:
		if !fits || bound == ^uint64(0) {
			return basicStringLengths{}, basicStringLengthObjective{}, false, nil
		}

		lengths.minimum = max(bound+1, lengths.minimum)
		if lengths.hasMaximum && lengths.minimum > lengths.maximum {
			return basicStringLengths{}, basicStringLengthObjective{}, false, nil
		}

		lengthObjective.length = lengths.minimum
	default:
		return basicStringLengths{}, basicStringLengthObjective{}, false,
			fmt.Errorf("schematest: unknown string objective %d", objective.kind)
	}

	return lengths, lengthObjective, true, nil
}

func stringObjectiveWithin(objective *stringSearchObjective, occurrence schemaOccurrence) bool {
	if objective == nil {
		return false
	}

	return rowInstancePrefixMatches(occurrence.instanceTemplate, objective.occurrence.instanceTemplate) &&
		(objective.occurrence.usePointer == occurrence.usePointer ||
			len(objective.occurrence.usePointer) > len(occurrence.usePointer) &&
				objective.occurrence.usePointer[:len(occurrence.usePointer)+1] == occurrence.usePointer+"/")
}
