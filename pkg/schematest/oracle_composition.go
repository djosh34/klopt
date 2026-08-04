package schematest

import (
	"fmt"
	"strconv"
)

// evaluateCompositionRules evaluates all conjunctive and alternative compositions.
func evaluateCompositionRules(
	result *evaluation,
	node *schemaNode,
	occurrence schemaOccurrence,
	value *jsonValue,
) {
	evaluateAllOfRules(result, node, occurrence, value)

	if result.err != nil {
		return
	}

	evaluateAnyOfRules(result, node, occurrence, value)
}

// evaluateAllOfRules evaluates every allOf branch against the same complete value.
func evaluateAllOfRules(
	result *evaluation,
	node *schemaNode,
	occurrence schemaOccurrence,
	value *jsonValue,
) {
	if len(node.allOf) == 0 {
		return
	}

	truthIndex := len(result.allOf)
	result.allOf = append(result.allOf, compositionTruth{
		ruleIdentity: makeRuleIdentity(occurrence, oracleRuleAllOf),
		branches:     make([]bool, len(node.allOf)),
	})

	for index, child := range node.allOf {
		childOccurrence := rebaseCompositionOccurrence(node, child, occurrence, "allOf", index)
		childResult := evaluateNode(child, value, childOccurrence)

		if childResult.err != nil {
			result.err = fmt.Errorf("%s branch %d: %w", occurrence.usePointer+"/allOf", index, childResult.err)

			return
		}

		result.allOf[truthIndex].branches[index] = childResult.valid
		mergeEvaluation(result, childResult)
	}
}

// evaluateAnyOfRules evaluates every anyOf branch and applies the aggregate failure closure.
func evaluateAnyOfRules(
	result *evaluation,
	node *schemaNode,
	occurrence schemaOccurrence,
	value *jsonValue,
) {
	if len(node.anyOf) == 0 {
		return
	}

	truthIndex := len(result.anyOf)
	result.anyOf = append(result.anyOf, compositionTruth{
		ruleIdentity: makeRuleIdentity(occurrence, oracleRuleAnyOf),
		branches:     make([]bool, len(node.anyOf)),
	})

	branchFailures := make([]failureIdentity, 0)
	anyBranchValid := false

	for index, child := range node.anyOf {
		childOccurrence := rebaseCompositionOccurrence(node, child, occurrence, "anyOf", index)
		childResult := evaluateNode(child, value, childOccurrence)

		if childResult.err != nil {
			result.err = fmt.Errorf("%s branch %d: %w", occurrence.usePointer+"/anyOf", index, childResult.err)

			return
		}

		result.anyOf[truthIndex].branches[index] = childResult.valid
		result.applicable = append(result.applicable, childResult.applicable...)
		result.observed = append(result.observed, childResult.observed...)
		result.allOf = append(result.allOf, childResult.allOf...)
		result.anyOf = append(result.anyOf, childResult.anyOf...)

		if childResult.valid {
			anyBranchValid = true

			continue
		}

		branchFailures = append(branchFailures, childResult.failures...)
	}

	if anyBranchValid {
		return
	}

	result.failures = append(result.failures, branchFailures...)
	result.failures = append(
		result.failures,
		makeRuleIdentity(occurrence, oracleRuleAnyOf),
	)
}

// rebaseCompositionOccurrence assigns a composition branch's use site and current instance.
func rebaseCompositionOccurrence(
	parent *schemaNode,
	child *schemaNode,
	occurrence schemaOccurrence,
	composition string,
	index int,
) schemaOccurrence {
	return rebaseChildOccurrence(
		parent,
		child,
		occurrence.usePointer+"/"+composition+"/"+strconv.Itoa(index),
		occurrence.instanceTemplate,
	)
}
