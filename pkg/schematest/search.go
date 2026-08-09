package schematest

import (
	"errors"
	"fmt"
	"iter"
	"math/big"
	"strconv"
	"strings"
)

// errMaxSteps stops one search before an assignment can exceed its budget.
var errMaxSteps = errors.New("schematest: maximum steps reached")

// search owns one never-reset assignment budget and the selected clean model.
type search struct {
	model    *schemaModel
	maxSteps uint64
	steps    uint64
}

// rowSearchContext explicitly carries private directed and valid-target search inputs.
type rowSearchContext struct {
	validRequest *validRequest
}

// assign charges one structural, kind, composition, enum, or scalar choice.
func (s *search) assign() error {
	if s == nil {
		return errors.New("schematest: nil search")
	}

	if s.steps == s.maxSteps {
		return errMaxSteps
	}

	s.steps++

	return nil
}

// findTargetRow searches one target without retaining generated rows.
//
//nolint:cyclop // Structural impossibility and row search share one target boundary.
func findTargetRow(plan *searchPlan, request validRequest, s *search) (*jsonValue, bool, error) {
	if plan == nil {
		return nil, false, errors.New("schematest: nil search plan")
	}

	if s == nil || s.model == nil || s.model.root == nil {
		return nil, false, errors.New("schematest: search has no model")
	}

	if requestHasSyntacticKindConflict(request) {
		return nil, false, nil
	}

	conflict, err := requestHasSyntacticCompositionConflict(
		s.model.root, s.model.root.occurrence, request.requirements,
	)
	if err != nil || conflict {
		return nil, false, err
	}

	forbidden, err := requestPresenceForbiddenByActiveSchema(s.model.root, request)
	if err != nil || forbidden {
		return nil, false, err
	}

	var found *jsonValue

	visit := func(value *jsonValue) (bool, error) {
		result := evaluate(s.model, value)
		if result.err != nil {
			return false, fmt.Errorf("evaluate generated row: %w", result.err)
		}

		if !result.valid || !targetRowMatches(result, request, value) {
			return false, nil
		}

		found = value

		return true, nil
	}

	complete, err := s.walkNode(
		s.model.root,
		s.model.root.occurrence,
		request.requirements,
		rowSearchContext{validRequest: &request},
		visit,
	)
	if err != nil {
		return nil, false, err
	}

	return found, complete, nil
}

// requestHasSyntacticKindConflict detects incompatible same-instance kind requirements.
func requestHasSyntacticKindConflict(request validRequest) bool {
	for _, left := range request.requirements {
		if !left.hasKind {
			continue
		}

		for _, right := range request.requirements {
			if right.hasKind && left.occurrence.instanceTemplate == right.occurrence.instanceTemplate &&
				left.kind != right.kind {
				return true
			}
		}
	}

	return false
}

// requestHasSyntacticCompositionConflict rejects explicit selected-branch contradictions.
//
//nolint:cyclop,gocognit,nestif // Kind, bounds, and branch implications are one syntax check.
func requestHasSyntacticCompositionConflict(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
) (bool, error) {
	states, constrained := rowCompositionTruthStates(requirements, occurrence, oracleRuleAnyOf, len(node.anyOf))
	if constrained {
		kind, hasKind := requestKindAtInstance(requirements, occurrence.instanceTemplate)

		for index, child := range node.anyOf {
			if states[index] {
				if hasKind && !nodeAcceptsKindForTarget(child, kind) {
					return true, nil
				}

				contradictory, err := nodeHasContradictoryNumericBounds(child)
				if err != nil || contradictory {
					return contradictory, err
				}
			}
		}

		for trueIndex, trueBranch := range node.anyOf {
			if !states[trueIndex] {
				continue
			}

			for falseIndex, falseBranch := range node.anyOf {
				if states[falseIndex] {
					continue
				}

				if branchAcceptsEveryValueOfKind(falseBranch, kind, hasKind) ||
					branchTypeImplies(trueBranch, falseBranch) {
					return true, nil
				}

				equal, err := jsonValidatedSemanticEqual(trueBranch.schemaJSON, falseBranch.schemaJSON)
				if err != nil {
					return false, err
				}

				if equal {
					return true, nil
				}
			}
		}
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child, occurrence, occurrence.usePointer+"/allOf/"+itoa(index), occurrence.instanceTemplate,
		)

		conflict, err := requestHasSyntacticCompositionConflict(child, childOccurrence, requirements)
		if err != nil || conflict {
			return conflict, err
		}
	}

	for index, child := range node.anyOf {
		childOccurrence := rebasePlanOccurrence(
			child, occurrence, occurrence.usePointer+"/anyOf/"+itoa(index), occurrence.instanceTemplate,
		)

		conflict, err := requestHasSyntacticCompositionConflict(child, childOccurrence, requirements)
		if err != nil || conflict {
			return conflict, err
		}
	}

	return false, nil
}

// requestKindAtInstance returns the selected JSON kind for one instance.
func requestKindAtInstance(requirements []requirement, instance string) (jsonKind, bool) {
	for _, requirement := range requirements {
		if requirement.hasKind && requirement.occurrence.instanceTemplate == instance {
			return requirement.kind, true
		}
	}

	return jsonNull, false
}

// nodeHasContradictoryNumericBounds detects a direct empty numeric interval.
func nodeHasContradictoryNumericBounds(node *schemaNode) (bool, error) {
	if node.minimum == nil || node.maximum == nil {
		return false, nil
	}

	comparison, err := node.minimum.compare(node.maximum)

	return comparison > 0, err
}

// branchAcceptsEveryValueOfKind detects a branch with no applicable validation constraint.
//
//nolint:cyclop,gocyclo // Every schema keyword independently prevents universal acceptance.
func branchAcceptsEveryValueOfKind(node *schemaNode, kind jsonKind, hasKind bool) bool {
	if node.enum != nil || len(node.allOf) > 0 || len(node.anyOf) > 0 {
		return false
	}

	if !hasKind {
		return node.kind == schemaAny && node.minimum == nil && node.maximum == nil && node.multipleOf == nil &&
			node.minLength == nil && node.maxLength == nil && node.pattern == nil && node.format == schemaFormatNone &&
			node.minItems == nil && node.maxItems == nil && node.items == nil && node.minProperties == nil &&
			node.maxProperties == nil && len(node.required) == 0 && len(node.properties) == 0 &&
			node.additionalProperties == nil && node.allowAdditionalProperties
	}

	if !nodeAcceptsKindForTarget(node, kind) {
		return false
	}

	switch kind {
	case jsonNumber:
		return node.kind != schemaInteger && node.minimum == nil && node.maximum == nil && node.multipleOf == nil &&
			!isNumericSchemaFormat(node.format)
	case jsonString:
		return node.minLength == nil && node.maxLength == nil && node.pattern == nil && node.format == schemaFormatNone
	case jsonArray:
		return node.minItems == nil && node.maxItems == nil && node.items == nil
	case jsonObject:
		return node.minProperties == nil && node.maxProperties == nil && len(node.required) == 0 &&
			len(node.properties) == 0 && node.additionalProperties == nil && node.allowAdditionalProperties
	default:
		return true
	}
}

// branchTypeImplies detects the direct integer-subset-of-number relation.
func branchTypeImplies(left, right *schemaNode) bool {
	return left.kind == schemaInteger && right.kind == schemaNumber &&
		branchAcceptsEveryValueOfKind(right, jsonNumber, true)
}

// requestPresenceForbiddenByActiveSchema rejects impossible active member targets.
func requestPresenceForbiddenByActiveSchema(root *schemaNode, request validRequest) (bool, error) {
	objective := 0
	if request.focus >= 0 {
		objective = request.focus
	}

	if objective >= len(request.targets) {
		return false, nil
	}

	target := request.targets[objective]
	target.requirements = request.requirements

	return targetPresenceForbiddenByActiveSchema(root, target)
}

// targetPresenceForbiddenByActiveSchema checks one vector component.
//
//nolint:cyclop // Root-kind and concrete-member contradictions share one preflight.
func targetPresenceForbiddenByActiveSchema(root *schemaNode, target validIntent) (bool, error) {
	tokens, ok := rowPointerTokens(target.expected.occurrence.instanceTemplate)
	if !ok {
		return false, nil
	}

	if len(tokens) == 0 {
		collision, err := activeSchemaHasForbiddenDeclaration(
			root, root.occurrence, target.requirements, make(map[*schemaNode]bool),
		)
		if err != nil || !collision {
			return false, err
		}

		for _, requirement := range target.requirements {
			if requirement.hasKind && rowOccurrenceMatches(requirement.occurrence, target.expected.occurrence) &&
				root.kind != schemaAny && !nodeAcceptsKindForTarget(root, requirement.kind) {
				return true, nil
			}
		}

		return false, nil
	}

	present := false

	for _, requirement := range target.requirements {
		if requirement.presence == requirementPresent &&
			instanceTemplateMatches(requirement.occurrence.instanceTemplate, target.expected.occurrence.instanceTemplate) {
			present = true

			break
		}
	}

	if !present {
		return false, nil
	}

	parentPointer := pointerFromTokens(tokens[:len(tokens)-1])
	containerTarget := target.expected.occurrence
	containerTarget.instanceTemplate = parentPointer

	container, occurrence, found := resolveFaultValueContainer(root, root.occurrence, containerTarget)
	if !found {
		return false, nil
	}

	return activeSchemaForbidsMember(
		container, occurrence, target.requirements, tokens[len(tokens)-1], make(map[*schemaNode]bool),
	)
}

// activeSchemaHasForbiddenDeclaration detects a declared name rejected by an active sibling.
func activeSchemaHasForbiddenDeclaration(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	visiting map[*schemaNode]bool,
) (bool, error) {
	names := make(map[string]bool)
	if err := collectActiveSameInstancePropertyNames(node, occurrence, visiting, names); err != nil {
		return false, err
	}

	for name := range names {
		forbidden, err := activeSchemaForbidsMember(
			node, occurrence, requirements, name, make(map[*schemaNode]bool),
		)
		if err != nil || forbidden {
			return forbidden, err
		}
	}

	return false, nil
}

// collectActiveSameInstancePropertyNames collects names across same-instance allOf schemas.
func collectActiveSameInstancePropertyNames(
	node *schemaNode,
	occurrence schemaOccurrence,
	visiting map[*schemaNode]bool,
	names map[string]bool,
) error {
	if node == nil || node.schemaShape == nil {
		return nil
	}

	if visiting[node] {
		return fmt.Errorf("schematest: recursive active declaration schema at %s", occurrence.usePointer)
	}

	visiting[node] = true
	defer delete(visiting, node)

	for name := range node.properties {
		names[name] = true
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if err := collectActiveSameInstancePropertyNames(child, childOccurrence, visiting, names); err != nil {
			return err
		}
	}

	return nil
}

// activeSchemaForbidsMember checks one name against the active conjunction.
//
//nolint:cyclop // Local, allOf, and constrained anyOf member rules form one conjunction.
func activeSchemaForbidsMember(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	name string,
	visiting map[*schemaNode]bool,
) (bool, error) {
	if node == nil || node.schemaShape == nil {
		return false, nil
	}

	if visiting[node] {
		return false, fmt.Errorf("schematest: recursive active member schema at %s", occurrence.usePointer)
	}

	visiting[node] = true
	defer delete(visiting, node)

	if _, declared := node.properties[name]; !declared &&
		!node.allowAdditionalProperties && node.additionalProperties == nil {
		return true, nil
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		forbidden, err := activeSchemaForbidsMember(child, childOccurrence, requirements, name, visiting)
		if err != nil || forbidden {
			return forbidden, err
		}
	}

	states, constrained := rowCompositionTruthStates(requirements, occurrence, "anyOf", len(node.anyOf))
	if !constrained {
		return false, nil
	}

	for index, child := range node.anyOf {
		if !states[index] {
			continue
		}

		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/anyOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		forbidden, err := activeSchemaForbidsMember(child, childOccurrence, requirements, name, visiting)
		if err != nil || forbidden {
			return forbidden, err
		}
	}

	return false, nil
}

// targetRowMatches requires a complete valid value and the target's exact requirements.
//
//nolint:cyclop // Validity, levels, and the three requirement dimensions are one acceptance pass.
func targetRowMatches(result evaluation, request validRequest, value *jsonValue) bool {
	if !result.valid {
		return false
	}

	for _, requirement := range request.requirements {
		switch {
		case requirement.tag == requirementTargetLevel &&
			!levelWasObserved(result.observedRecords(), requirement.target) &&
			!compositionLevelWasObserved(result, requirement.target):
			return false
		case requirement.presence != requirementNoPresence && !requirement.canonical &&
			!presenceRequirementWasSatisfied(value, requirement):
			return false
		case requirement.hasKind && !kindWasObserved(result.observedRecords(), requirement.occurrence, requirement.kind):
			return false
		case requirement.hasBranch && !branchTruthWasObserved(result, requirement):
			return false
		}
	}

	return true
}

// levelWasObserved matches wildcard instance templates to concrete array members.
func levelWasObserved(observed iter.Seq[levelIdentity], expected levelIdentity) bool {
	for candidate := range observed {
		if candidate.level != expected.level || !ruleOccurrenceMatches(candidate.occurrence, expected.occurrence) {
			continue
		}

		if candidate.rule != expected.rule {
			continue
		}

		return true
	}

	return false
}

// compositionLevelWasObserved derives valid composition levels from truth vectors.
//
//nolint:cyclop // allOf and anyOf truth vectors have deliberately separate locked levels.
func compositionLevelWasObserved(result evaluation, expected levelIdentity) bool {
	var truths iter.Seq[compositionTruth]

	switch expected.rule {
	case oracleRuleAllOf:
		truths = result.compositionRecords(oracleRuleAllOf)
	case oracleRuleAnyOf:
		truths = result.compositionRecords(oracleRuleAnyOf)
	default:
		return false
	}

	for truth := range truths {
		if !ruleOccurrenceMatches(truth.occurrence, expected.occurrence) {
			continue
		}

		if expected.rule == oracleRuleAllOf {
			if expected.level != planLevelAllTrue {
				return false
			}

			allTrue := true

			for _, branch := range truth.branches {
				if !branch {
					allTrue = false

					break
				}
			}

			if allTrue {
				return true
			}

			continue
		}

		mask := new(big.Int)

		for index, branch := range truth.branches {
			if branch {
				mask.SetBit(mask, index, 1)
			}
		}

		if mask.Sign() != 0 && expected.level == planLevelMask+mask.String() {
			return true
		}
	}

	return false
}

// presenceRequirementWasSatisfied checks one required or optional data path.
func presenceRequirementWasSatisfied(value *jsonValue, requirement requirement) bool {
	if strings.HasSuffix(requirement.occurrence.usePointer, "/additionalProperties") {
		return true
	}

	present, known := rowValuePathPresent(value, requirement.occurrence.instanceTemplate)
	if !known {
		return true
	}

	return present == (requirement.presence == requirementPresent)
}

// rowValuePathPresent checks one exact instance-template path.
func rowValuePathPresent(value *jsonValue, pointer string) (bool, bool) {
	tokens, ok := rowPointerTokens(pointer)
	if !ok {
		return false, false
	}

	return rowValuePathPresentTokens(value, tokens), true
}

// rowValuePathPresentTokens traverses object members and array indices.
//
//nolint:cyclop // Object, wildcard, and indexed array traversal form one path operation.
func rowValuePathPresentTokens(value *jsonValue, tokens []string) bool {
	if len(tokens) == 0 {
		return true
	}

	if value == nil {
		return false
	}

	token := tokens[0]

	switch value.kind {
	case jsonObject:
		if token == "*" {
			for _, member := range value.object {
				if rowValuePathPresentTokens(member, tokens[1:]) {
					return true
				}
			}

			return false
		}

		member, exists := value.object[token]

		return exists && rowValuePathPresentTokens(member, tokens[1:])
	case jsonArray:
		if token == "*" {
			for _, element := range value.array {
				if rowValuePathPresentTokens(element, tokens[1:]) {
					return true
				}
			}

			return false
		}

		index, err := strconv.Atoi(token)

		return err == nil && index >= 0 && index < len(value.array) &&
			rowValuePathPresentTokens(value.array[index], tokens[1:])
	default:
		return false
	}
}

// kindWasObserved checks the clean type observation for one constrained occurrence.
func kindWasObserved(observed iter.Seq[levelIdentity], occurrence schemaOccurrence, kind jsonKind) bool {
	return levelWasObserved(observed, levelIdentity{
		ruleIdentity: makeRuleIdentity(occurrence, oracleRuleType),
		level:        jsonKindName(kind),
	})
}

// branchTruthWasObserved checks one exact allOf or anyOf truth bit.
func branchTruthWasObserved(result evaluation, requirement requirement) bool {
	truths := result.compositionRecords(requirement.composition)

	parentUsePointer := strings.TrimSuffix(
		strings.TrimSuffix(requirement.occurrence.usePointer, "/"+itoa(requirement.branch)),
		"/"+requirement.composition,
	)

	for truth := range truths {
		if truth.rule != requirement.composition || truth.occurrence.usePointer != parentUsePointer ||
			!instanceTemplateMatches(requirement.occurrence.instanceTemplate, truth.occurrence.instanceTemplate) {
			continue
		}

		if requirement.branch < 0 || requirement.branch >= len(truth.branches) {
			return false
		}

		return truth.branches[requirement.branch] == requirement.truth
	}

	return false
}

// ruleOccurrenceMatches compares authored use sites and wildcard instance paths.
func ruleOccurrenceMatches(actual, expected schemaOccurrence) bool {
	return actual.usePointer == expected.usePointer &&
		actual.targetPointer == expected.targetPointer &&
		actual.reference == expected.reference &&
		instanceTemplateMatches(expected.instanceTemplate, actual.instanceTemplate)
}

// rowOccurrenceMatches compares planner requirements, which intentionally omit target identity.
func rowOccurrenceMatches(left, right schemaOccurrence) bool {
	return left.usePointer == right.usePointer &&
		instanceTemplateMatches(left.instanceTemplate, right.instanceTemplate)
}

// instanceTemplateMatches lets a planner wildcard stand for one concrete JSON token.
func instanceTemplateMatches(pattern, value string) bool {
	patternTokens, patternOK := rowPointerTokens(pattern)

	valueTokens, valueOK := rowPointerTokens(value)
	if !patternOK || !valueOK || len(patternTokens) != len(valueTokens) {
		return false
	}

	for index, token := range patternTokens {
		if token != "*" && token != valueTokens[index] {
			return false
		}
	}

	return true
}

// rowPointerTokens decodes the small JSON Pointer vocabulary used by instance templates.
func rowPointerTokens(pointer string) ([]string, bool) {
	if pointer == "#" {
		return nil, true
	}

	if !strings.HasPrefix(pointer, "#/") {
		return nil, false
	}

	rawTokens := strings.Split(pointer[2:], "/")

	decoded := make([]string, 0, len(rawTokens))
	for _, raw := range rawTokens {
		value, err := unescapePointerToken(raw)
		if err != nil {
			return nil, false
		}

		decoded = append(decoded, value)
	}

	return decoded, true
}
