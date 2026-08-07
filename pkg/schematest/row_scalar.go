package schematest

import (
	"errors"
	"fmt"
	"strings"
)

// walkScalar tries deterministic primitive witnesses for one assigned kind.
//
//nolint:cyclop // Enum, number, string, and finite primitive paths share one scalar seam.
func (s *search) walkScalar(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	context rowSearchContext,
	kind jsonKind,
	visit rowVisit,
) (bool, error) {
	if node.enum == nil && kind == jsonNumber {
		return s.walkActiveScalarPinAlternatives(
			node,
			occurrence,
			append([]applicabilityPin(nil), pins...),
			func(activePins []applicabilityPin) (bool, error) {
				return s.walkActiveNumberRules(
					node, occurrence, activePins, context.validTarget, visit,
				)
			},
		)
	}

	candidates, err := rowScalarValues(node, kind)
	if err != nil {
		return false, err
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

	if node.enum != nil {
		return false, nil
	}

	switch kind {
	case jsonString:
		return s.walkActiveScalarPinAlternatives(
			node,
			occurrence,
			append([]applicabilityPin(nil), pins...),
			func(activePins []applicabilityPin) (bool, error) {
				return s.walkActiveStringRules(
					node, occurrence, activePins, context.validTarget, visit,
				)
			},
		)
	default:
		return false, nil
	}
}

// walkActiveStringRules searches one canonical applicable rule view.
//
//nolint:cyclop // Rule collection, target seeding, and product construction form one search phase.
func (s *search) walkActiveStringRules(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	target *validTarget,
	visit rowVisit,
) (bool, error) {
	rules, err := activeStringRulesFor(node, occurrence, pins, nil)
	if err != nil {
		return false, err
	}

	if !rules.supported {
		return false, nil
	}

	patterns := make([]*patternAST, 0, len(rules.patterns))
	for _, pattern := range rules.patterns {
		patterns = append(patterns, pattern.pattern)
	}

	lengths, err := basicStringLengthsFromActive(rules.lengths)
	if err != nil {
		return false, err
	}

	if len(patterns) == 0 && len(rules.formats) == 0 && len(lengths.boundaries) == 0 {
		return false, nil
	}

	seedNode := node
	rule := oracleRulePattern
	level := oracleStringValidLevel

	seedPointer := occurrence.usePointer
	if target != nil {
		if targetNode, found := scalarTargetNode(node, occurrence, target.expected.occurrence); found {
			seedNode = targetNode
			rule = target.expected.rule
			level = target.expected.level
			seedPointer = target.expected.occurrence.usePointer
		}
	}

	canonicalSchemaJSON, err := marshalStrict(seedNode.schemaJSON)
	if err != nil {
		return false, fmt.Errorf("schematest: canonicalize string search schema: %w", err)
	}

	product, err := newBasicStringProduct(patterns)
	if err != nil {
		return false, err
	}

	if err := product.addFormats(rules.formats, -1); err != nil {
		return false, err
	}

	return s.walkBasicStringProductForLengths(
		product,
		lengths,
		basicStringLengthObjective{},
		searchSeed(seedPointer, canonicalSchemaJSON, rule, level),
		visit,
	)
}

// scalarTargetNode resolves the valid target occurrence used to lock a scalar seed.
func scalarTargetNode(
	node *schemaNode,
	occurrence schemaOccurrence,
	target schemaOccurrence,
) (*schemaNode, bool) {
	if ruleOccurrenceMatches(occurrence, target) {
		return node, true
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child, occurrence.usePointer+"/allOf/"+itoa(index), occurrence.instanceTemplate,
		)
		if found, ok := scalarTargetNode(child, childOccurrence, target); ok {
			return found, true
		}
	}

	for index, child := range node.anyOf {
		childOccurrence := rebasePlanOccurrence(
			child, occurrence.usePointer+"/anyOf/"+itoa(index), occurrence.instanceTemplate,
		)
		if found, ok := scalarTargetNode(child, childOccurrence, target); ok {
			return found, true
		}
	}

	return nil, false
}

// rowScalarValues builds a finite deterministic frontier for one primitive kind.
//
//nolint:cyclop // Canonical, derived, numeric, and string witness phases are explicit.
func rowScalarValues(node *schemaNode, kind jsonKind) ([]*jsonValue, error) {
	if node.enum != nil {
		return rowEnumValues(node.enum, kind)
	}

	var candidates []*jsonValue

	candidates, err := canonicalKindWitnesses(kind)
	if err != nil {
		return nil, err
	}

	if node.defaultValue != nil && node.defaultValue.kind == kind {
		candidates, err = appendUniqueJSONWitness(candidates, node.defaultValue)
		if err != nil {
			return nil, err
		}
	}

	derived, err := canonicalAnyOfWitnesses(node, kind)
	if err != nil {
		return nil, err
	}

	for _, candidate := range derived {
		candidates, err = appendUniqueJSONWitness(candidates, candidate)
		if err != nil {
			return nil, err
		}
	}

	if kind == jsonString {
		candidates, err = appendRowStringCandidates(candidates, node)
		if err != nil {
			return nil, err
		}
	}

	return filterRowScalarValues(candidates, node, kind)
}

// rowEnumValues preserves authored enum order and uses already canonical members.
func rowEnumValues(enum []*jsonValue, kind jsonKind) ([]*jsonValue, error) {
	candidates := make([]*jsonValue, 0, len(enum))
	for _, candidate := range enum {
		if candidate == nil {
			return nil, errors.New("schematest: nil enum row value")
		}

		if candidate.kind != kind {
			continue
		}

		var err error

		candidates, err = appendUniqueJSONWitness(candidates, candidate)
		if err != nil {
			return nil, err
		}
	}

	return candidates, nil
}

// filterRowScalarValues keeps only values that can satisfy the node's explicit kind.
func filterRowScalarValues(candidates []*jsonValue, node *schemaNode, kind jsonKind) ([]*jsonValue, error) {
	filtered := make([]*jsonValue, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil {
			return nil, errors.New("schematest: nil scalar row value")
		}

		if candidate.kind != kind {
			continue
		}

		if node.kind == schemaInteger && kind == jsonNumber {
			integer, err := candidate.number.isInteger()
			if err != nil {
				return nil, err
			}

			if !integer {
				continue
			}
		}

		filtered = append(filtered, candidate)
	}

	return filtered, nil
}

// appendRowStringCandidates adds valid format and length witnesses in stable order.
//
//nolint:cyclop // Pattern, format, and length witnesses are one ordered frontier.
func appendRowStringCandidates(candidates []*jsonValue, node *schemaNode) ([]*jsonValue, error) {
	for _, sample := range rowStringFormatSamples(node.format) {
		var err error

		candidates, err = appendUniqueJSONWitness(candidates, &jsonValue{kind: jsonString, text: sample})
		if err != nil {
			return nil, err
		}
	}

	if witness, exists := canonicalStringPatternWitness(node.pattern); exists {
		var err error

		candidates, err = appendUniqueJSONWitness(candidates, witness)
		if err != nil {
			return nil, err
		}
	}

	if node.pattern != nil {
		var err error

		candidates, err = appendUniqueJSONWitness(
			candidates, &jsonValue{kind: jsonString, text: "a@b"},
		)
		if err != nil {
			return nil, err
		}
	}

	for _, count := range []*exactCount{node.minLength, node.maxLength} {
		length, fits, err := exactCountUint64(count)
		if err != nil {
			return nil, err
		}

		if !fits || length > maxPlanStringWitnessLength || length > uint64(^uint(0)>>1) {
			continue
		}

		candidate := &jsonValue{kind: jsonString, text: strings.Repeat("a", int(length))}

		candidates, err = appendUniqueJSONWitness(candidates, candidate)
		if err != nil {
			return nil, err
		}
	}

	return candidates, nil
}

// rowStringFormatSamples provides one small witness for every active string format.
func rowStringFormatSamples(format schemaFormat) []string {
	switch format {
	case schemaFormatByte:
		return []string{"YQ=="}
	case schemaFormatDate:
		return []string{"1970-01-01"}
	case schemaFormatDateTime:
		return []string{"1970-01-01T00:00:00Z"}
	case schemaFormatEmail:
		return []string{"a@b", "a@example.com"}
	case schemaFormatIPv4:
		return []string{"0.0.0.0"}
	case schemaFormatUUID:
		return []string{"00000000-0000-4000-8000-000000000000"}
	case schemaFormatUUIDv4, schemaFormatUUIDDashV4:
		return []string{"00000000-0000-4000-8000-000000000000"}
	case schemaFormatCIDR, schemaFormatIPv4CIDR:
		return []string{"0.0.0.0/0"}
	default:
		return nil
	}
}
