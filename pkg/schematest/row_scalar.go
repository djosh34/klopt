package schematest

import (
	"errors"
	"fmt"
	"strings"
)

// walkScalar tries deterministic primitive witnesses for one assigned kind.
//
//nolint:cyclop // Number, directed-string, finite, and generated phases share one scalar boundary.
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
					node, occurrence, activeRequirements, context.validRequest, context.scalarFault, visit,
				)
			},
		)
	}

	if kind == jsonString {
		if scalarFaultDirectsEnum(context.scalarFault, node, occurrence) {
			return s.walkStringEnumFault(
				node, occurrence, requirements, context.scalarFault, visit,
			)
		}

		if objective := scalarFaultStringObjective(context.scalarFault, node, occurrence); objective != nil {
			_, complete, err := s.walkDirectedStringObjective(
				node, occurrence, requirements, objective, visit,
			)

			return complete, err
		}
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
	if objective := validFalseStringObjective(node, occurrence, requirements); objective != nil {
		handled, complete, err := s.walkDirectedStringObjective(
			node, occurrence, requirements, objective, visit,
		)
		if err != nil || handled {
			return complete, err
		}
	}

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

	if err := product.setFormatObjective(matchingFormatBoundary(request, occurrence)); err != nil {
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

// matchingFormatBoundary returns the semantic format objective targeting this scalar.
func matchingFormatBoundary(
	request *validRequest,
	occurrence schemaOccurrence,
) *formatBoundaryObjective {
	boundary := validRequestFormatBoundary(request)
	if boundary == nil || !stringObjectiveWithin(
		&stringSearchObjective{occurrence: boundary.identity.occurrence}, occurrence,
	) {
		return nil
	}

	return boundary
}

// validFalseStringObjective returns the first selected false-branch pattern direction.
func validFalseStringObjective(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
) *stringSearchObjective {
	for _, requirement := range requirements {
		if !requirement.hasBranch || requirement.composition != oracleRuleAnyOf || requirement.truth {
			continue
		}

		branch, found := scalarTargetNode(node, occurrence, requirement.occurrence)
		if !found {
			continue
		}

		patternOccurrence, found := firstStringPatternOccurrence(branch, requirement.occurrence)
		if found {
			return &stringSearchObjective{
				kind:       stringSearchPatternFalse,
				occurrence: patternOccurrence,
				rule:       oracleRulePattern,
				level:      oracleStringValidLevel,
				closure:    nil,
			}
		}
	}

	return nil
}

// firstStringPatternOccurrence finds one authored pattern in deterministic composition order.
func firstStringPatternOccurrence(
	node *schemaNode,
	occurrence schemaOccurrence,
) (schemaOccurrence, bool) {
	if node.pattern != nil {
		return occurrence, true
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child, occurrence, occurrence.usePointer+"/allOf/"+itoa(index), occurrence.instanceTemplate,
		)
		if foundOccurrence, found := firstStringPatternOccurrence(child, childOccurrence); found {
			return foundOccurrence, true
		}
	}

	return schemaOccurrence{}, false
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

// walkStringEnumFault searches outside one enum while retaining active string siblings.
func (s *search) walkStringEnumFault(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	fault *faultProgram,
	visit rowVisit,
) (bool, error) {
	rules, err := activeStringRulesFor(node, occurrence, requirements, nil)
	if err != nil {
		return false, err
	}

	patterns := make([]*patternAST, 0, len(rules.patterns))
	for _, pattern := range rules.patterns {
		patterns = append(patterns, pattern.pattern)
	}

	lengths, err := basicStringLengthsFromActive(rules.lengths)
	if err != nil {
		return false, err
	}

	product, err := newBasicStringProduct(patterns)
	if err != nil {
		return false, err
	}

	if formatErr := product.addFormats(rules.formats, -1); formatErr != nil {
		return false, formatErr
	}

	target, found := scalarTargetNode(node, occurrence, fault.obligation.occurrence)
	if !found || target.schemaJSON == nil {
		return false, errors.New("schematest: string enum fault target has no canonical schema")
	}

	canonicalSchemaJSON, err := marshalStrict(target.schemaJSON)
	if err != nil {
		return false, fmt.Errorf("schematest: canonicalize string enum fault schema: %w", err)
	}

	return s.walkBasicStringProductForLengths(
		product,
		lengths,
		basicStringLengthObjective{},
		searchSeed(
			fault.obligation.occurrence.usePointer,
			canonicalSchemaJSON,
			oracleRuleEnum,
			fault.obligation.component,
		),
		visit,
	)
}

// scalarFaultDirectsEnum reports whether this scalar owns the directed enum rule.
func scalarFaultDirectsEnum(
	fault *faultProgram,
	node *schemaNode,
	occurrence schemaOccurrence,
) bool {
	return fault != nil && fault.obligation.rule == oracleRuleEnum &&
		scalarTargetNodeMatches(node, occurrence, fault.obligation.occurrence)
}

// scalarFaultStringObjective resolves one directed string-rule program.
func scalarFaultStringObjective(
	fault *faultProgram,
	node *schemaNode,
	occurrence schemaOccurrence,
) *stringSearchObjective {
	if fault == nil {
		return nil
	}

	kind, ok := stringFaultObjectiveKind(fault.obligation.rule)
	if !ok || !scalarTargetNodeMatches(node, occurrence, fault.obligation.occurrence) {
		return nil
	}

	return &stringSearchObjective{
		kind:       kind,
		occurrence: fault.obligation.occurrence,
		closure:    append([]failureIdentity(nil), fault.expected...),
		rule:       fault.obligation.rule,
		level:      fault.obligation.component,
	}
}

// scalarTargetNodeMatches reports whether a scalar subtree contains the target.
func scalarTargetNodeMatches(
	node *schemaNode,
	occurrence schemaOccurrence,
	target schemaOccurrence,
) bool {
	_, found := scalarTargetNode(node, occurrence, target)

	return found
}

// scalarFaultWithin reports whether an occurrence contains the directed target.
func scalarFaultWithin(fault *faultProgram, occurrence schemaOccurrence) bool {
	if fault == nil {
		return false
	}

	target := fault.obligation.occurrence

	return rowInstancePrefixMatches(occurrence.instanceTemplate, target.instanceTemplate) &&
		(target.usePointer == occurrence.usePointer || strings.HasPrefix(target.usePointer, occurrence.usePointer+"/"))
}

// scalarTargetNode resolves the target occurrence used to lock a scalar seed.
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

		if kind == jsonString && nodeHasStringSearchRules(node) {
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
