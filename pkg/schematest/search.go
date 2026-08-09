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
	scalarFault  *faultProgram
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
func findTargetRow(plan *searchPlan, request validRequest, s *search) (*jsonValue, bool, error) {
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

// targetRowMatches requires a complete valid value and the target's exact requirements.
func targetRowMatches(result evaluation, request validRequest, value *jsonValue) bool {
	return result.valid && formatBoundaryWasSatisfied(value, request.formatBoundary) &&
		requirementsMatch(result, value, request.requirements)
}

// requirementsMatch is the sole complete-row applicability matcher used by
// focused valid rows and fault-parent replay.
//
//nolint:cyclop // Levels, presence, kind, and composition are the four requirement dimensions.
func requirementsMatch(result evaluation, value *jsonValue, requirements []requirement) bool {
	for _, requirement := range requirements {
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

// formatBoundaryWasSatisfied checks the requested semantic objective at its instance template.
//
//nolint:cyclop // Objective resolution and exact incremental replay form one check.
func formatBoundaryWasSatisfied(value *jsonValue, objective *formatBoundaryObjective) bool {
	if objective == nil {
		return true
	}

	specification, exists := stringFormatSpecificationFor(objective.format)
	if !exists || specification.program == nil {
		return false
	}

	for _, path := range matchingValuePaths(value, objective.identity.occurrence.instanceTemplate) {
		candidate := valueAtPath(value, path)
		if candidate == nil || candidate.kind != jsonString {
			continue
		}

		state := specification.program.start(len(candidate.text))
		for _, character := range candidate.text {
			if character > rune(basicStringMaxUnit) {
				state = deadStringFormatState(state)

				break
			}

			state = specification.program.advance(state, uint16(character))
		}

		if specification.program.accept(state) && objective.boundary.matches(state) {
			return true
		}
	}

	return false
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
