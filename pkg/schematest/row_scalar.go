package schematest

import (
	"errors"
	"fmt"
)

// walkScalar tries deterministic primitive witnesses for one assigned kind.
func (s *search) walkScalar(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	context rowSearchContext,
	kind jsonKind,
	visit rowVisit,
) (bool, error) {
	if kind == jsonNumber {
		return s.walkActiveScalarRequirementAlternatives(
			node,
			occurrence,
			append([]requirement(nil), requirements...),
			func(activeRequirements []requirement) (bool, error) {
				return s.walkActiveNumberRules(
					node, occurrence, activeRequirements, context.validRequest, visit,
				)
			},
		)
	}

	complete := false

	var visitErr error

	err := rowScalarValueSource(node, kind)(func(candidate *jsonValue) bool {
		if assignErr := s.assign(); assignErr != nil {
			visitErr = assignErr

			return false
		}

		complete, visitErr = visit(candidate)

		return visitErr == nil && !complete
	})
	if err != nil {
		return false, err
	}

	if visitErr != nil || complete {
		return complete, visitErr
	}

	if node.enum != nil {
		return false, nil
	}

	switch kind {
	case jsonString:
		return s.walkActiveScalarRequirementAlternatives(
			node,
			occurrence,
			append([]requirement(nil), requirements...),
			func(activeRequirements []requirement) (bool, error) {
				return s.walkActiveStringRules(
					node, occurrence, activeRequirements, context.validRequest, visit,
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
	requirements []requirement,
	request *validRequest,
	visit rowVisit,
) (bool, error) {
	rules, err := activeStringRulesFor(node, occurrence, requirements, nil)
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
	if objective := validStringObjective(request, node, occurrence); objective != nil {
		seedNode = objective.node
		rule = objective.identity.rule
		level = objective.identity.level
		seedPointer = objective.identity.occurrence.usePointer
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

// resolvedScalarObjective reuses one scalar occurrence traversal result.
type resolvedScalarObjective struct {
	identity levelIdentity
	node     *schemaNode
}

// validStringObjective consumes the request's explicit string execution sequence.
func validStringObjective(
	request *validRequest,
	node *schemaNode,
	occurrence schemaOccurrence,
) *resolvedScalarObjective {
	if request == nil {
		return nil
	}

	for _, objective := range request.stringObjectives {
		resolved, found := scalarTargetNode(node, occurrence, objective.occurrence)
		if found {
			return &resolvedScalarObjective{identity: objective, node: resolved}
		}
	}

	return nil
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
			child,
			occurrence,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if found, ok := scalarTargetNode(child, childOccurrence, target); ok {
			return found, true
		}
	}

	for index, child := range node.anyOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/anyOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if found, ok := scalarTargetNode(child, childOccurrence, target); ok {
			return found, true
		}
	}

	return nil, false
}

// rowScalarValueSource advances exactly one primitive alternative at a time.
//
//nolint:cyclop,gocognit // Error, filtering, stopping, and phased lazy sources share one closure boundary.
func rowScalarValueSource(node *schemaNode, kind jsonKind) jsonValueSource {
	return uniqueJSONValueSource(func(yield func(*jsonValue) bool) error {
		stopped := false

		var sourceErr error

		emit := func(candidate *jsonValue) bool {
			usable, err := rowScalarValueUsable(candidate, node, kind)
			if err != nil {
				sourceErr = err
				stopped = true

				return false
			}

			if !usable {
				return true
			}

			if !yield(candidate) {
				stopped = true
			}

			return !stopped
		}

		if node.enum != nil {
			for _, member := range node.enum {
				if member.value == nil {
					return errors.New("schematest: nil enum row value")
				}

				if member.value.kind == kind && !emit(member.value) {
					return nil
				}
			}

			return nil
		}

		if err := walkCanonicalKindWitnesses(kind, emit); err != nil {
			return err
		}

		if stopped {
			return sourceErr
		}

		if err := canonicalAnyOfWitnesses(node, kind)(emit); err != nil {
			return err
		}

		if stopped {
			return sourceErr
		}

		if kind == jsonString {
			if err := walkRowStringCandidates(node, emit); err != nil {
				return err
			}
		}

		return sourceErr
	})
}

// rowScalarValueUsable applies kind and integer filtering to one yielded value.
func rowScalarValueUsable(candidate *jsonValue, node *schemaNode, kind jsonKind) (bool, error) {
	if candidate == nil {
		return false, errors.New("schematest: nil scalar row value")
	}

	if candidate.kind != kind {
		return false, nil
	}

	if node.kind != schemaInteger || kind != jsonNumber {
		return true, nil
	}

	return candidate.number.isInteger()
}

// walkRowStringCandidates adds only fixed small seeds; directed lengths remain lazy.
func walkRowStringCandidates(node *schemaNode, yield func(*jsonValue) bool) error {
	if !walkRowStringFormatSamples(node.format, func(sample string) bool {
		return yield(&jsonValue{kind: jsonString, text: sample})
	}) {
		return nil
	}

	if witness, exists := canonicalStringPatternWitness(node.pattern); exists && !yield(witness) {
		return nil
	}

	if node.pattern != nil {
		yield(&jsonValue{kind: jsonString, text: "a@b"})
	}

	return nil
}

// walkRowStringFormatSamples emits fixed format seeds without retaining a slice.
func walkRowStringFormatSamples(format schemaFormat, yield func(string) bool) bool {
	switch format {
	case schemaFormatByte:
		return yield("YQ==")
	case schemaFormatDate:
		return yield("1970-01-01")
	case schemaFormatDateTime:
		return yield("1970-01-01T00:00:00Z")
	case schemaFormatEmail:
		return yield("a@b") && yield("a@example.com")
	case schemaFormatIPv4:
		return yield("0.0.0.0")
	case schemaFormatUUID, schemaFormatUUIDv4, schemaFormatUUIDDashV4:
		return yield("00000000-0000-4000-8000-000000000000")
	case schemaFormatCIDR, schemaFormatIPv4CIDR:
		return yield("0.0.0.0/0")
	default:
		return true
	}
}
