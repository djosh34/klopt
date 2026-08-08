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
	validTarget *validTarget
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
func findTargetRow(plan *searchPlan, target validTarget, s *search) (*jsonValue, bool, error) {
	if plan == nil {
		return nil, false, errors.New("schematest: nil search plan")
	}

	if s == nil || s.model == nil || s.model.root == nil {
		return nil, false, errors.New("schematest: search has no model")
	}

	forbidden, err := targetPresenceForbiddenByActiveSchema(s.model.root, target)
	if err != nil || forbidden {
		return nil, false, err
	}

	var found *jsonValue

	visit := func(value *jsonValue) (bool, error) {
		result := evaluate(s.model, value)
		if result.err != nil {
			return false, fmt.Errorf("evaluate generated row: %w", result.err)
		}

		if !result.valid || !targetRowMatches(result, target, value) {
			return false, nil
		}

		found = value

		return true, nil
	}

	complete, err := s.walkNode(
		s.model.root,
		s.model.root.occurrence,
		target.pins,
		rowSearchContext{validTarget: &target},
		visit,
	)
	if err != nil {
		return nil, false, err
	}

	return found, complete, nil
}

// targetPresenceForbiddenByActiveSchema rejects impossible active member targets.
//
//nolint:cyclop // Root-kind and concrete-member contradictions share one preflight.
func targetPresenceForbiddenByActiveSchema(root *schemaNode, target validTarget) (bool, error) {
	tokens, ok := rowPointerTokens(target.expected.occurrence.instanceTemplate)
	if !ok {
		return false, nil
	}

	if len(tokens) == 0 {
		collision, err := activeSchemaHasForbiddenDeclaration(
			root, root.occurrence, target.pins, make(map[*schemaNode]bool),
		)
		if err != nil || !collision {
			return false, err
		}

		for _, pin := range target.pins {
			if pin.hasKind && rowOccurrenceMatches(pin.occurrence, target.expected.occurrence) &&
				root.kind != schemaAny && !nodeAcceptsKindForTarget(root, pin.kind) {
				return true, nil
			}
		}

		return false, nil
	}

	present := false

	for _, pin := range target.pins {
		if pin.presence == planPinPresent &&
			instanceTemplateMatches(pin.occurrence.instanceTemplate, target.expected.occurrence.instanceTemplate) {
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
		container, occurrence, target.pins, tokens[len(tokens)-1], make(map[*schemaNode]bool),
	)
}

// activeSchemaHasForbiddenDeclaration detects a declared name rejected by an active sibling.
func activeSchemaHasForbiddenDeclaration(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	visiting map[*schemaNode]bool,
) (bool, error) {
	names := make(map[string]bool)
	if err := collectActiveSameInstancePropertyNames(node, occurrence, visiting, names); err != nil {
		return false, err
	}

	for name := range names {
		forbidden, err := activeSchemaForbidsMember(
			node, occurrence, pins, name, make(map[*schemaNode]bool),
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
//nolint:cyclop // Local, allOf, and pinned anyOf member rules form one conjunction.
func activeSchemaForbidsMember(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
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

		forbidden, err := activeSchemaForbidsMember(child, childOccurrence, pins, name, visiting)
		if err != nil || forbidden {
			return forbidden, err
		}
	}

	states, pinned := rowCompositionTruthStates(pins, occurrence, "anyOf", len(node.anyOf))
	if !pinned {
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

		forbidden, err := activeSchemaForbidsMember(child, childOccurrence, pins, name, visiting)
		if err != nil || forbidden {
			return forbidden, err
		}
	}

	return false, nil
}

// targetRowMatches requires a complete valid value and the target's exact pins.
//
//nolint:cyclop // Validity, levels, and the three pin dimensions are one acceptance pass.
func targetRowMatches(result evaluation, target validTarget, value *jsonValue) bool {
	if !result.valid || (!levelWasObserved(result.observedRecords(), target.expected) &&
		!compositionLevelWasObserved(result, target.expected)) {
		return false
	}

	for _, pin := range target.pins {
		switch {
		case pin.presence != planPinNoPresence && !pin.canonical && !presencePinWasSatisfied(value, pin):
			return false
		case pin.hasKind && !kindWasObserved(result.observedRecords(), pin.occurrence, pin.kind):
			return false
		case pin.hasBranch && !branchTruthWasObserved(result, pin):
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

// presencePinWasSatisfied checks one required or optional data path.
func presencePinWasSatisfied(value *jsonValue, pin applicabilityPin) bool {
	if strings.HasSuffix(pin.occurrence.usePointer, "/additionalProperties") {
		return true
	}

	present, known := rowValuePathPresent(value, pin.occurrence.instanceTemplate)
	if !known {
		return true
	}

	return present == (pin.presence == planPinPresent)
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

// kindWasObserved checks the clean type observation for one pinned occurrence.
func kindWasObserved(observed iter.Seq[levelIdentity], occurrence schemaOccurrence, kind jsonKind) bool {
	return levelWasObserved(observed, levelIdentity{
		ruleIdentity: makeRuleIdentity(occurrence, oracleRuleType),
		level:        jsonKindName(kind),
	})
}

// branchTruthWasObserved checks one exact allOf or anyOf truth bit.
func branchTruthWasObserved(result evaluation, pin applicabilityPin) bool {
	truths := result.compositionRecords(pin.composition)

	parentUsePointer := strings.TrimSuffix(
		strings.TrimSuffix(pin.occurrence.usePointer, "/"+itoa(pin.branch)),
		"/"+pin.composition,
	)

	for truth := range truths {
		if truth.rule != pin.composition || truth.occurrence.usePointer != parentUsePointer ||
			!instanceTemplateMatches(pin.occurrence.instanceTemplate, truth.occurrence.instanceTemplate) {
			continue
		}

		if pin.branch < 0 || pin.branch >= len(truth.branches) {
			return false
		}

		return truth.branches[pin.branch] == pin.truth
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

// rowOccurrenceMatches compares planner pins, which intentionally omit target identity.
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
