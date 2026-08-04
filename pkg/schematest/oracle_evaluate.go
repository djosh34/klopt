package schematest

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// oracleRuleType identifies the Schema Object's type rule.
	oracleRuleType = "type"
	// oracleRuleEnum identifies the Schema Object's enum rule.
	oracleRuleEnum = "enum"
	// oracleRuleMinimum identifies an inclusive numeric lower bound.
	oracleRuleMinimum = "minimum"
	// oracleRuleExclusiveMinimum identifies an exclusive numeric lower bound.
	oracleRuleExclusiveMinimum = "exclusiveMinimum"
	// oracleRuleMaximum identifies an inclusive numeric upper bound.
	oracleRuleMaximum = "maximum"
	// oracleRuleExclusiveMaximum identifies an exclusive numeric upper bound.
	oracleRuleExclusiveMaximum = "exclusiveMaximum"
	// oracleRuleMultipleOf identifies exact numeric divisibility.
	oracleRuleMultipleOf = "multipleOf"
	// oracleRuleFormat identifies a format constraint.
	oracleRuleFormat = "format"
	// oracleRuleMinLength identifies a string lower-length bound.
	oracleRuleMinLength = "minLength"
	// oracleRuleMaxLength identifies a string upper-length bound.
	oracleRuleMaxLength = "maxLength"
	// oracleRulePattern identifies a string pattern constraint.
	oracleRulePattern = "pattern"
	// oracleRuleAllOf identifies an allOf composition occurrence.
	oracleRuleAllOf = "allOf"
	// oracleRuleAnyOf identifies an anyOf composition occurrence and aggregate failure.
	oracleRuleAnyOf = "anyOf"

	// oracleLevelPrefix separates a rule identity from an observed level.
	oracleLevelPrefix = "level:"
	// oracleEnumLevelPrefix identifies an observed authored enum member.
	oracleEnumLevelPrefix = "member:"
)

// ruleIdentity locates one clean schema rule occurrence.
type ruleIdentity struct {
	occurrence schemaOccurrence
	rule       string
}

// failureIdentity is the rule identity used by an exact failure closure.
type failureIdentity = ruleIdentity

// levelIdentity identifies one observed valid local level.
type levelIdentity struct {
	ruleIdentity

	level string
}

// compositionTruth records the authored branch verdicts for one composition occurrence.
type compositionTruth struct {
	ruleIdentity

	branches []bool
}

// evaluation is the complete clean evaluation of one JSON value.
type evaluation struct {
	valid        bool
	applicable   []ruleIdentity
	observed     []levelIdentity
	allOf        []compositionTruth
	anyOf        []compositionTruth
	failures     []failureIdentity
	failed       bool
	fromCache    bool
	materialized bool
	records      *evaluationRecords
	err          error
}

// evaluationCacheKey identifies one shared shape/value/instance evaluation.
type evaluationCacheKey struct {
	shape            *schemaShape
	value            *jsonValue
	instanceTemplate string
}

// evaluationCacheEntry retains a result and the occurrence that authored its paths.
type evaluationCacheEntry struct {
	occurrence schemaOccurrence
	result     evaluation
}

// evaluationContext carries one complete evaluation's shared-shape cache.
type evaluationContext struct {
	cache map[evaluationCacheKey]evaluationCacheEntry
}

// String renders the stable rule portion of an obligation identity.
func (identity ruleIdentity) String() string {
	return strings.Join([]string{
		identity.occurrence.usePointer,
		identity.occurrence.instanceTemplate,
		identity.rule,
	}, "|")
}

// String renders the stable observed-level portion of an obligation identity.
func (identity levelIdentity) String() string {
	return identity.ruleIdentity.String() + "|" + oracleLevelPrefix + identity.level
}

// evaluate independently checks one complete clean JSON value against the
// primitive portion of the admitted schema model.
func evaluate(model *schemaModel, value *jsonValue) evaluation {
	result := evaluation{}

	if model == nil || model.root == nil {
		result.err = errors.New("schema model has no root")

		return result
	}

	if err := validateJSONValue(value, make(map[*jsonValue]bool)); err != nil {
		result.err = fmt.Errorf("evaluate JSON value: %w", err)

		return result
	}

	context := evaluationContext{cache: make(map[evaluationCacheKey]evaluationCacheEntry)}
	result = context.evaluateNode(model.root, value, model.root.occurrence)
	result.valid = result.err == nil && !result.failed

	return result
}

// evaluateNode evaluates one schema occurrence and its compositions in canonical order.
func evaluateNode(node *schemaNode, value *jsonValue, occurrence schemaOccurrence) evaluation {
	context := evaluationContext{cache: make(map[evaluationCacheKey]evaluationCacheEntry)}

	return context.evaluateNode(node, value, occurrence)
}

// evaluateNode evaluates one schema occurrence with one evaluation-local cache.
//
//nolint:cyclop // Canonical rule phases intentionally remain explicit and ordered.
func (context *evaluationContext) evaluateNode(
	node *schemaNode,
	value *jsonValue,
	occurrence schemaOccurrence,
) evaluation {
	result := evaluation{records: newEvaluationRecords()}
	if node == nil || node.schemaShape == nil {
		result.err = errors.New("schema occurrence has no shape")

		return result
	}

	key := evaluationCacheKey{
		shape:            node.schemaShape,
		value:            value,
		instanceTemplate: occurrence.instanceTemplate,
	}
	if cached, exists := context.cache[key]; exists {
		result = cached.result
		result.records = result.records.rebased(cached.occurrence, occurrence)

		result.fromCache = true
		if occurrence.reference && result.recordsMaterialized() {
			result.materializeRecords()
		}

		return result
	}

	evaluateTypeRule(&result, node, occurrence, value)

	if result.err != nil {
		return result
	}

	evaluateEnumRule(&result, node, occurrence, value)

	if result.err != nil {
		return result
	}

	evaluateNumberRules(&result, node, occurrence, value)

	if result.err == nil {
		evaluateStringRules(&result, node, occurrence, value)
	}

	if result.err == nil {
		evaluateArrayRules(context, &result, node, occurrence, value)
	}

	if result.err == nil {
		evaluateObjectRules(context, &result, node, occurrence, value)
	}

	if result.err == nil {
		evaluateCompositionRules(context, &result, node, occurrence, value)
	}

	result.valid = result.err == nil && !result.failed

	if result.err == nil {
		result.fromCache = false
		context.cache[key] = evaluationCacheEntry{
			occurrence: occurrence,
			result:     result,
		}
	}

	return result
}

// rebaseChildOccurrence carries a direct schema shape's evaluated use site and instance path to a child.
func rebaseChildOccurrence(
	child *schemaNode,
	usePointer string,
	instanceTemplate string,
) schemaOccurrence {
	childOccurrence := child.occurrence
	childOccurrence.usePointer = usePointer
	childOccurrence.instanceTemplate = instanceTemplate

	return childOccurrence
}

// evaluateTypeRule applies the explicit kind and same-object nullable contract.
// A typeless schema still has an applicable type rule: it observes the actual
// JSON kind while admitting every kind, including null.
func evaluateTypeRule(result *evaluation, node *schemaNode, occurrence schemaOccurrence, value *jsonValue) {
	identity := makeRuleIdentity(occurrence, oracleRuleType)
	appendApplicable(result, identity)

	matches, err := valueMatchesNodeKind(value, node.kind, node.nullable)
	if err != nil {
		result.err = fmt.Errorf("%s: %w", identity, err)

		return
	}

	if !matches {
		appendFailure(result, identity)

		return
	}

	appendObserved(result, levelIdentity{
		ruleIdentity: identity,
		level:        jsonKindName(value.kind),
	})
}

// valueMatchesNodeKind applies only the Schema Object's own explicit type.
func valueMatchesNodeKind(value *jsonValue, kind schemaKind, nullable bool) (bool, error) {
	if value == nil {
		return false, errors.New("JSON value is nil")
	}

	if kind == schemaAny {
		return true, nil
	}

	if value.kind == jsonNull {
		return nullable, nil
	}

	return matchesNonNullNodeKind(value, kind)
}

// matchesNonNullNodeKind applies an explicit type after null handling.
func matchesNonNullNodeKind(value *jsonValue, kind schemaKind) (bool, error) {
	switch kind {
	case schemaBoolean:
		return value.kind == jsonBoolean, nil
	case schemaInteger:
		if value.kind != jsonNumber {
			return false, nil
		}

		return value.number.isInteger()
	case schemaNumber:
		return value.kind == jsonNumber, nil
	case schemaString:
		return value.kind == jsonString, nil
	case schemaArray:
		return value.kind == jsonArray, nil
	case schemaObject:
		return value.kind == jsonObject, nil
	default:
		return false, fmt.Errorf("unknown schema kind %d", kind)
	}
}

// evaluateEnumRule applies enum to every JSON kind using clean semantic equality.
func evaluateEnumRule(result *evaluation, node *schemaNode, occurrence schemaOccurrence, value *jsonValue) {
	if node.enum == nil {
		return
	}

	identity := makeRuleIdentity(occurrence, oracleRuleEnum)
	appendApplicable(result, identity)

	for index, member := range node.enum {
		equal, err := jsonSemanticEqual(value, member)
		if err != nil {
			result.err = fmt.Errorf("%s member %d: %w", identity, index, err)

			return
		}

		if equal {
			appendObserved(result, levelIdentity{
				ruleIdentity: identity,
				level:        fmt.Sprintf("%s%d", oracleEnumLevelPrefix, index),
			})

			return
		}
	}

	appendFailure(result, identity)
}

// appendFailure records one exact failure and its validity contribution.
func appendFailure(result *evaluation, identity failureIdentity) {
	result.failures = append(result.failures, identity)
	ensureEvaluationRecords(result)
	result.records.failures.append(identity)
	result.failed = true
}

// makeRuleIdentity creates a stable clean occurrence/rule identity.
func makeRuleIdentity(occurrence schemaOccurrence, rule string) ruleIdentity {
	return ruleIdentity{occurrence: occurrence, rule: rule}
}

// jsonKindName returns the deterministic JSON spelling used by kind levels.
func jsonKindName(kind jsonKind) string {
	switch kind {
	case jsonNull:
		return "null"
	case jsonBoolean:
		return "boolean"
	case jsonNumber:
		return "number"
	case jsonString:
		return "string"
	case jsonArray:
		return "array"
	case jsonObject:
		return "object"
	default:
		return fmt.Sprintf("kind-%d", kind)
	}
}
