package schematest

import (
	"fmt"
	"sort"
	"strconv"
)

const (
	// oracleRuleMinProperties identifies an inclusive lower bound on object members.
	oracleRuleMinProperties = "minProperties"
	// oracleRuleMaxProperties identifies an inclusive upper bound on object members.
	oracleRuleMaxProperties = "maxProperties"
	// oracleRuleRequired identifies one required object member.
	oracleRuleRequired = "required"
	// oracleRuleAdditionalProperties identifies one forbidden object member.
	oracleRuleAdditionalProperties = "additionalProperties"

	// oracleObjectValidLevel is the single local valid level for an object count rule.
	oracleObjectValidLevel = "valid"
	// oracleRequiredPresentLevel identifies a supplied required member.
	oracleRequiredPresentLevel = "present"
)

// evaluateObjectRules applies object counts, requiredness, properties, and additional properties.
func evaluateObjectRules(
	context *evaluationContext,
	result *evaluation,
	node *schemaNode,
	occurrence schemaOccurrence,
	value *jsonValue,
) {
	if value.kind != jsonObject {
		return
	}

	if err := evaluateObjectCounts(result, node, occurrence, len(value.object)); err != nil {
		result.err = err

		return
	}

	evaluateRequiredMembers(result, node, occurrence, value.object)

	if result.err != nil {
		return
	}

	memberNames := sortedObjectNames(value.object)
	evaluateDeclaredProperties(context, result, node, occurrence, value.object, memberNames)

	if result.err != nil {
		return
	}

	evaluateAdditionalProperties(context, result, node, occurrence, value.object, memberNames)
}

// evaluateObjectCounts evaluates object member-count constraints in rule order.
func evaluateObjectCounts(
	result *evaluation,
	node *schemaNode,
	occurrence schemaOccurrence,
	count int,
) error {
	if node.minProperties != nil {
		if err := evaluateObjectCountRule(
			result, occurrence, count, node.minProperties, oracleRuleMinProperties, true,
		); err != nil {
			return err
		}
	}

	if node.maxProperties != nil {
		if err := evaluateObjectCountRule(
			result, occurrence, count, node.maxProperties, oracleRuleMaxProperties, false,
		); err != nil {
			return err
		}
	}

	return nil
}

// evaluateRequiredMembers evaluates required members in UTF-8 member order.
func evaluateRequiredMembers(
	result *evaluation,
	node *schemaNode,
	occurrence schemaOccurrence,
	members map[string]*jsonValue,
) {
	required := append([]string(nil), node.required...)
	sort.Strings(required)

	for _, name := range required {
		identity := makeRuleIdentity(
			appendObjectMemberOccurrence(occurrence, name),
			oracleRuleRequired,
		)
		appendApplicable(result, identity)

		if _, supplied := members[name]; !supplied {
			appendFailure(result, identity)

			continue
		}

		appendObserved(result, levelIdentity{
			ruleIdentity: identity,
			level:        oracleRequiredPresentLevel,
		})
	}
}

// evaluateDeclaredProperties evaluates only supplied declared properties.
func evaluateDeclaredProperties(
	context *evaluationContext,
	result *evaluation,
	node *schemaNode,
	occurrence schemaOccurrence,
	members map[string]*jsonValue,
	memberNames []string,
) {
	for _, name := range memberNames {
		property, declared := node.properties[name]
		if !declared {
			continue
		}

		propertyOccurrence := rebaseChildOccurrence(
			property,
			occurrence.usePointer+"/properties/"+escapePointerToken(name),
			appendInstanceToken(occurrence.instanceTemplate, name),
		)
		propertyResult := context.evaluateNode(property, members[name], propertyOccurrence)
		mergeEvaluation(result, propertyResult)

		if result.err != nil {
			return
		}
	}
}

// evaluateAdditionalProperties evaluates supplied undeclared members.
func evaluateAdditionalProperties(
	context *evaluationContext,
	result *evaluation,
	node *schemaNode,
	occurrence schemaOccurrence,
	members map[string]*jsonValue,
	memberNames []string,
) {
	for _, name := range memberNames {
		if _, declared := node.properties[name]; declared {
			continue
		}

		if node.additionalProperties != nil {
			additionalOccurrence := rebaseChildOccurrence(
				node.additionalProperties,
				occurrence.usePointer+"/additionalProperties",
				appendInstanceToken(occurrence.instanceTemplate, name),
			)
			additionalResult := context.evaluateNode(
				node.additionalProperties, members[name], additionalOccurrence,
			)
			mergeEvaluation(result, additionalResult)

			if result.err != nil {
				return
			}

			continue
		}

		if node.allowAdditionalProperties {
			continue
		}

		identity := makeRuleIdentity(
			appendObjectMemberOccurrence(occurrence, name),
			oracleRuleAdditionalProperties,
		)
		appendApplicable(result, identity)
		appendFailure(result, identity)
	}
}

// evaluateObjectCountRule compares an exact existing member count with one bound.
func evaluateObjectCountRule(
	result *evaluation,
	occurrence schemaOccurrence,
	count int,
	bound *exactCount,
	rule string,
	minimum bool,
) error {
	identity := makeRuleIdentity(occurrence, rule)
	appendApplicable(result, identity)

	actual, err := parseExactNumber(strconv.Itoa(count))
	if err != nil {
		return fmt.Errorf("%s: parse object member count: %w", identity, err)
	}

	comparison, err := actual.compare(bound.number)
	if err != nil {
		return fmt.Errorf("%s: compare object member count: %w", identity, err)
	}

	violated := comparison < 0
	if !minimum {
		violated = comparison > 0
	}

	if violated {
		appendFailure(result, identity)
	} else {
		appendObserved(result, levelIdentity{
			ruleIdentity: identity,
			level:        oracleObjectValidLevel,
		})
	}

	return nil
}

// appendObjectMemberOccurrence gives one member rule its concrete instance path.
func appendObjectMemberOccurrence(occurrence schemaOccurrence, name string) schemaOccurrence {
	occurrence.instanceTemplate = appendInstanceToken(occurrence.instanceTemplate, name)

	return occurrence
}
