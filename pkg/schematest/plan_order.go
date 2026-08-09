//nolint:godoclint // Planner ordering helpers are private implementation details.
package schematest

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

const planNumericFormatRank = 5

type planPointerToken struct {
	raw        string
	decoded    string
	array      bool
	arrayIndex uint64
}

type planOccurrenceOrderKey struct {
	use       []planPointerToken
	target    []planPointerToken
	instance  []planPointerToken
	reference bool
}

type planOrderKey struct {
	occurrence planOccurrenceOrderKey
	ruleRank   int
	rule       string
	fault      bool
	level      uint64
	component  string
}

// compareSchemaOccurrences applies use-site, target, instance, and reference order.
func compareSchemaOccurrences(left, right schemaOccurrence) (int, error) {
	comparison, err := comparePlanPointers(left.usePointer, right.usePointer, true)
	if err != nil || comparison != 0 {
		return comparison, err
	}

	comparison, err = comparePlanPointers(left.targetPointer, right.targetPointer, true)
	if err != nil || comparison != 0 {
		return comparison, err
	}

	comparison, err = comparePlanPointers(left.instanceTemplate, right.instanceTemplate, false)
	if err != nil || comparison != 0 {
		return comparison, err
	}

	if left.reference == right.reference {
		return 0, nil
	}

	if left.reference {
		return 1, nil
	}

	return -1, nil
}

// comparePlanPointers compares decoded JSON Pointer tokens.
func comparePlanPointers(left, right string, schemaPointer bool) (int, error) {
	leftTokens, err := parsePlanPointer(left, schemaPointer)
	if err != nil {
		return 0, err
	}

	rightTokens, err := parsePlanPointer(right, schemaPointer)
	if err != nil {
		return 0, err
	}

	return comparePlanPointerTokenSlices(leftTokens, rightTokens), nil
}

// comparePlanPointerTokens compares array indices numerically and object tokens by UTF-8 bytes.
func comparePlanPointerTokens(left, right planPointerToken) int {
	if left.array && right.array {
		switch {
		case left.arrayIndex < right.arrayIndex:
			return -1
		case left.arrayIndex > right.arrayIndex:
			return 1
		default:
			return compareRawPointerTokens(left.raw, right.raw)
		}
	}

	comparison := bytes.Compare([]byte(left.decoded), []byte(right.decoded))
	if comparison != 0 {
		return comparison
	}

	return compareRawPointerTokens(left.raw, right.raw)
}

// compareRawPointerTokens gives malformed direct comparator inputs deterministic fallback order.
func compareRawPointerTokens(left, right string) int {
	return strings.Compare(left, right)
}

// parsePlanPointer decodes one canonical pointer and identifies composition array indices.
func parsePlanPointer(pointer string, schemaPointer bool) ([]planPointerToken, error) {
	if pointer == "#" {
		return nil, nil
	}

	if !strings.HasPrefix(pointer, "#/") {
		return nil, fmt.Errorf("invalid JSON Pointer %q", pointer)
	}

	encodedTokens := strings.Split(pointer[2:], "/")

	tokens := make([]planPointerToken, 0, len(encodedTokens))
	for index, raw := range encodedTokens {
		decoded, err := unescapePointerToken(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid JSON Pointer %q token %q: %w", pointer, raw, err)
		}

		token := planPointerToken{raw: raw, decoded: decoded}
		if schemaPointer && index > 0 && isPlanCompositionToken(tokens[index-1].decoded) {
			arrayIndex, isArray := canonicalPlanArrayIndex(decoded)
			if isArray {
				token.array = true
				token.arrayIndex = arrayIndex
			}
		}

		tokens = append(tokens, token)
	}

	return tokens, nil
}

// isPlanCompositionToken identifies the only schema-occurrence arrays in the plan.
func isPlanCompositionToken(token string) bool {
	return token == "allOf" || token == "anyOf"
}

// canonicalPlanArrayIndex parses a canonical non-negative JSON array index.
func canonicalPlanArrayIndex(value string) (uint64, bool) {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return 0, false
	}

	index, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, false
	}

	return index, true
}

// makePlanOrderKey parses canonical pointer and order metadata once.
func makePlanOrderKey(identity obligation) (*planOrderKey, error) {
	use, err := parsePlanPointer(identity.occurrence.usePointer, true)
	if err != nil {
		return nil, err
	}

	target, err := parsePlanPointer(identity.occurrence.targetPointer, true)
	if err != nil {
		return nil, err
	}

	instance, err := parsePlanPointer(identity.occurrence.instanceTemplate, false)
	if err != nil {
		return nil, err
	}

	return &planOrderKey{
		occurrence: planOccurrenceOrderKey{
			use: use, target: target, instance: instance, reference: identity.occurrence.reference,
		},
		ruleRank:  obligationRuleRank(identity),
		rule:      identity.rule,
		fault:     planComponentIsFault(identity.component),
		level:     identity.order,
		component: identity.component,
	}, nil
}

// comparePlanObligations applies occurrence, rule, level, and fault order.
func comparePlanObligations(left, right obligation) (int, error) {
	leftKey := left.orderKey
	if leftKey == nil {
		var err error

		leftKey, err = makePlanOrderKey(left)
		if err != nil {
			return 0, err
		}
	}

	rightKey := right.orderKey
	if rightKey == nil {
		var err error

		rightKey, err = makePlanOrderKey(right)
		if err != nil {
			return 0, err
		}
	}

	return comparePlanOrderKeys(leftKey, rightKey), nil
}

func comparePlanOrderKeys(left, right *planOrderKey) int {
	if comparison := comparePlanOccurrenceOrderKeys(left.occurrence, right.occurrence); comparison != 0 {
		return comparison
	}

	if left.ruleRank != right.ruleRank {
		return compareInts(left.ruleRank, right.ruleRank)
	}

	if left.rule != right.rule {
		return strings.Compare(left.rule, right.rule)
	}

	if left.fault != right.fault {
		if left.fault {
			return 1
		}

		return -1
	}

	if left.level != right.level {
		return compareUint64(left.level, right.level)
	}

	return strings.Compare(left.component, right.component)
}

func comparePlanOccurrenceOrderKeys(left, right planOccurrenceOrderKey) int {
	if comparison := comparePlanPointerTokenSlices(left.use, right.use); comparison != 0 {
		return comparison
	}

	if comparison := comparePlanPointerTokenSlices(left.target, right.target); comparison != 0 {
		return comparison
	}

	if comparison := comparePlanPointerTokenSlices(left.instance, right.instance); comparison != 0 {
		return comparison
	}

	if left.reference == right.reference {
		return 0
	}

	if left.reference {
		return 1
	}

	return -1
}

func comparePlanPointerTokenSlices(left, right []planPointerToken) int {
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}

	for index := 0; index < limit; index++ {
		if comparison := comparePlanPointerTokens(left[index], right[index]); comparison != 0 {
			return comparison
		}
	}

	return compareInts(len(left), len(right))
}

// obligationRuleRank returns an explicit family rank or the generic rule rank.
func obligationRuleRank(identity obligation) int {
	if identity.ruleRank == 0 {
		return planRuleRank(identity.rule)
	}

	return int(identity.ruleRank) - 1
}

// encodedPlanRuleRank stores a rank without using zero as a valid rank.
func encodedPlanRuleRank(rank int) uint8 {
	return uint8(rank + 1)
}

// planRuleRankForKind places a numeric format before string-family rules.
func planRuleRankForKind(rule string, kind jsonKind) int {
	if rule == oracleRuleFormat && kind == jsonNumber {
		return planNumericFormatRank
	}

	return planRuleRank(rule)
}

// compareRuleIdentities applies canonical occurrence and rule order to failures.
func compareRuleIdentities(left, right ruleIdentity) (int, error) {
	comparison, err := compareSchemaOccurrences(left.occurrence, right.occurrence)
	if err != nil || comparison != 0 {
		return comparison, err
	}

	leftRank := planRuleRank(left.rule)

	rightRank := planRuleRank(right.rule)
	if leftRank != rightRank {
		return compareInts(leftRank, rightRank), nil
	}

	return strings.Compare(left.rule, right.rule), nil
}

// compareInts compares two planner ranks.
func compareInts(left, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

// compareUint64 compares two insertion order values.
func compareUint64(left, right uint64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

// planRuleRank returns the locked local rule sequence.
//
//nolint:cyclop // The switch spells out the specification's rule order.
func planRuleRank(rule string) int {
	const (
		rankType = iota
		rankEnum
		rankMinimum
		rankMaximum
		rankMultipleOf
		rankNumericFormat
		rankLengthMinimum
		rankLengthMaximum
		rankPattern
		rankFormat
		rankItemsMinimum
		rankItemsMaximum
		rankPropertiesMinimum
		rankPropertiesMaximum
		rankRequired
		rankAdditionalProperties
		rankAllOf
		rankAnyOf
		rankUnknown
	)

	switch rule {
	case oracleRuleType:
		return rankType
	case oracleRuleEnum:
		return rankEnum
	case oracleRuleMinimum, oracleRuleExclusiveMinimum:
		return rankMinimum
	case oracleRuleMaximum, oracleRuleExclusiveMaximum:
		return rankMaximum
	case oracleRuleMultipleOf:
		return rankMultipleOf
	case oracleRuleMinLength:
		return rankLengthMinimum
	case oracleRuleMaxLength:
		return rankLengthMaximum
	case oracleRulePattern:
		return rankPattern
	case oracleRuleFormat:
		return rankFormat
	case oracleRuleMinItems:
		return rankItemsMinimum
	case oracleRuleMaxItems:
		return rankItemsMaximum
	case oracleRuleMinProperties:
		return rankPropertiesMinimum
	case oracleRuleMaxProperties:
		return rankPropertiesMaximum
	case oracleRuleRequired:
		return rankRequired
	case oracleRuleAdditionalProperties:
		return rankAdditionalProperties
	case oracleRuleAllOf:
		return rankAllOf
	case oracleRuleAnyOf:
		return rankAnyOf
	default:
		return rankUnknown
	}
}
