package schematest

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// errMaxSteps stops one search before an assignment can exceed its budget.
var errMaxSteps = errors.New("schematest: maximum steps reached")

// search owns one never-reset assignment budget and the selected clean model.
type search struct {
	model           *schemaModel
	maxSteps        uint64
	steps           uint64
	stringObjective *stringSearchObjective
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
func findTargetRow(plan *searchPlan, target validTarget, s *search) (*jsonValue, bool, error) {
	if plan == nil {
		return nil, false, errors.New("schematest: nil search plan")
	}

	if s == nil || s.model == nil || s.model.root == nil {
		return nil, false, errors.New("schematest: search has no model")
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

	complete, err := s.walkNode(s.model.root, s.model.root.occurrence, target.pins, visit)
	if err != nil {
		return nil, false, err
	}

	return found, complete, nil
}

// targetRowMatches requires a complete valid value and the target's exact pins.
//
//nolint:cyclop // Validity, levels, and the three pin dimensions are one acceptance pass.
func targetRowMatches(result evaluation, target validTarget, value *jsonValue) bool {
	if !result.valid || (!levelWasObserved(result.observed, target.expected) &&
		!compositionLevelWasObserved(result, target.expected)) {
		return false
	}

	for _, pin := range target.pins {
		switch {
		case pin.presence != planPinNoPresence && !pin.canonical && !presencePinWasSatisfied(value, pin):
			return false
		case pin.hasKind && !kindWasObserved(result.observed, pin.occurrence, pin.kind):
			return false
		case pin.hasBranch && !branchTruthWasObserved(result, pin):
			return false
		}
	}

	return true
}

// levelWasObserved matches wildcard instance templates to concrete array members.
func levelWasObserved(observed []levelIdentity, expected levelIdentity) bool {
	for _, candidate := range observed {
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
	var truths []compositionTruth

	switch expected.rule {
	case oracleRuleAllOf:
		truths = result.allOf
	case oracleRuleAnyOf:
		truths = result.anyOf
	default:
		return false
	}

	for _, truth := range truths {
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
func kindWasObserved(observed []levelIdentity, occurrence schemaOccurrence, kind jsonKind) bool {
	return levelWasObserved(observed, levelIdentity{
		ruleIdentity: makeRuleIdentity(occurrence, oracleRuleType),
		level:        jsonKindName(kind),
	})
}

// branchTruthWasObserved checks one exact allOf or anyOf truth bit.
func branchTruthWasObserved(result evaluation, pin applicabilityPin) bool {
	var truths []compositionTruth
	if pin.composition == "allOf" {
		truths = result.allOf
	} else {
		truths = result.anyOf
	}

	parentUsePointer := strings.TrimSuffix(
		strings.TrimSuffix(pin.occurrence.usePointer, "/"+itoa(pin.branch)),
		"/"+pin.composition,
	)

	for _, truth := range truths {
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
