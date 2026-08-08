package schematest

import (
	"fmt"
	"strconv"
)

// evaluateCompositionRules evaluates all conjunctive and alternative compositions.
func evaluateCompositionRules(
	context *evaluationContext,
	result *evaluation,
	node *schemaNode,
	occurrence schemaOccurrence,
	value *jsonValue,
) {
	evaluateAllOfRules(context, result, node, occurrence, value)

	if result.err != nil {
		return
	}

	evaluateAnyOfRules(context, result, node, occurrence, value)
}

// evaluateAllOfRules evaluates every allOf branch against the same complete value.
func evaluateAllOfRules(
	context *evaluationContext,
	result *evaluation,
	node *schemaNode,
	occurrence schemaOccurrence,
	value *jsonValue,
) {
	if len(node.allOf) == 0 {
		return
	}

	truth := appendAllOfTruth(result, compositionTruth{
		ruleIdentity: makeRuleIdentity(occurrence, oracleRuleAllOf),
		branches:     make([]bool, len(node.allOf)),
	})

	for index, child := range node.allOf {
		childOccurrence := rebaseCompositionOccurrence(child, occurrence, "allOf", index)
		childResult := context.evaluateNode(child, value, childOccurrence)

		if childResult.err != nil {
			result.err = fmt.Errorf("%s branch %d: %w", occurrence.usePointer+"/allOf", index, childResult.err)

			return
		}

		truth.branches[index] = childResult.valid
		appendEvaluation(result, childResult)
	}
}

// evaluateAnyOfRules evaluates every anyOf branch and applies the aggregate failure closure.
func evaluateAnyOfRules(
	context *evaluationContext,
	result *evaluation,
	node *schemaNode,
	occurrence schemaOccurrence,
	value *jsonValue,
) {
	if len(node.anyOf) == 0 {
		return
	}

	truth := appendAnyOfTruth(result, compositionTruth{
		ruleIdentity: makeRuleIdentity(occurrence, oracleRuleAnyOf),
		branches:     make([]bool, len(node.anyOf)),
	})

	children := make([]evaluation, len(node.anyOf))
	anyBranchValid := false

	for index, child := range node.anyOf {
		childOccurrence := rebaseCompositionOccurrence(child, occurrence, "anyOf", index)
		childResult := context.evaluateNode(child, value, childOccurrence)

		if childResult.err != nil {
			result.err = fmt.Errorf("%s branch %d: %w", occurrence.usePointer+"/anyOf", index, childResult.err)

			return
		}

		children[index] = childResult
		truth.branches[index] = childResult.valid
		anyBranchValid = anyBranchValid || childResult.valid
	}

	for _, childResult := range children {
		if anyBranchValid {
			appendEvaluationNonFailures(result, childResult)
		} else {
			appendEvaluation(result, childResult)
		}
	}

	if !anyBranchValid {
		appendFailure(result, makeRuleIdentity(occurrence, oracleRuleAnyOf))
	}
}

// rebaseCompositionOccurrence assigns a composition branch's use site and current instance.
func rebaseCompositionOccurrence(
	child *schemaNode,
	occurrence schemaOccurrence,
	composition string,
	index int,
) schemaOccurrence {
	return rebaseChildOccurrence(
		child,
		occurrence,
		occurrence.usePointer+"/"+composition+"/"+strconv.Itoa(index),
		occurrence.instanceTemplate,
	)
}
