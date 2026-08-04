package schematest

import (
	"fmt"
	"strconv"
)

const (
	// oracleRuleMinItems identifies an inclusive lower bound on array length.
	oracleRuleMinItems = "minItems"
	// oracleRuleMaxItems identifies an inclusive upper bound on array length.
	oracleRuleMaxItems = "maxItems"
	// oracleArrayValidLevel is the single local valid level for one array rule.
	oracleArrayValidLevel = "valid"
)

// evaluateArrayRules applies array counts and evaluates each existing item.
func evaluateArrayRules(
	result *evaluation,
	node *schemaNode,
	occurrence schemaOccurrence,
	value *jsonValue,
) {
	if value.kind != jsonArray {
		return
	}

	if node.minItems != nil {
		if err := evaluateArrayCountRule(
			result, occurrence, len(value.array), node.minItems, oracleRuleMinItems, true,
		); err != nil {
			result.err = err

			return
		}
	}

	if node.maxItems != nil {
		if err := evaluateArrayCountRule(
			result, occurrence, len(value.array), node.maxItems, oracleRuleMaxItems, false,
		); err != nil {
			result.err = err

			return
		}
	}

	if node.items == nil {
		return
	}

	for index, item := range value.array {
		itemOccurrence := rebaseChildOccurrence(
			node,
			node.items,
			occurrence.usePointer+"/items",
			appendInstanceToken(occurrence.instanceTemplate, strconv.Itoa(index)),
		)

		itemResult := evaluateNode(node.items, item, itemOccurrence)
		mergeEvaluation(result, itemResult)

		if result.err != nil {
			return
		}
	}
}

// evaluateArrayCountRule compares an exact existing item count with one bound.
func evaluateArrayCountRule(
	result *evaluation,
	occurrence schemaOccurrence,
	count int,
	bound *exactCount,
	rule string,
	minimum bool,
) error {
	identity := makeRuleIdentity(occurrence, rule)
	result.applicable = append(result.applicable, identity)

	actual, err := parseExactNumber(strconv.Itoa(count))
	if err != nil {
		return fmt.Errorf("%s: parse array length: %w", identity, err)
	}

	comparison, err := actual.compare(bound.number)
	if err != nil {
		return fmt.Errorf("%s: compare array length: %w", identity, err)
	}

	violated := comparison < 0
	if !minimum {
		violated = comparison > 0
	}

	if violated {
		result.failures = append(result.failures, identity)
	} else {
		result.observed = append(result.observed, levelIdentity{
			ruleIdentity: identity,
			level:        oracleArrayValidLevel,
		})
	}

	return nil
}

// mergeEvaluation appends a child evaluation in traversal order.
func mergeEvaluation(result *evaluation, child evaluation) {
	result.applicable = append(result.applicable, child.applicable...)
	result.observed = append(result.observed, child.observed...)
	result.allOf = append(result.allOf, child.allOf...)
	result.anyOf = append(result.anyOf, child.anyOf...)

	result.failures = append(result.failures, child.failures...)
	if child.err != nil {
		result.err = child.err
	}
}
