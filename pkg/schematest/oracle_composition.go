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

	if result.err == nil {
		evaluateAnyOfRules(context, result, node, occurrence, value)
	}
}

// evaluateAllOfRules constructs the complete truth and children before publishing either.
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

	truth := compositionTruth{
		ruleIdentity: makeRuleIdentity(occurrence, oracleRuleAllOf),
		branches:     make([]bool, len(node.allOf)),
	}

	children := make([]evaluation, len(node.allOf))
	for index, child := range node.allOf {
		childOccurrence := rebaseCompositionOccurrence(child, occurrence, "allOf", index)

		childResult := context.evaluateNode(child, value, childOccurrence)
		if childResult.err != nil {
			result.err = fmt.Errorf("%s branch %d: %w", occurrence.usePointer+"/allOf", index, childResult.err)

			return
		}

		truth.branches[index] = childResult.valid
		children[index] = childResult
	}

	appendCompositionTruth(result, truth)

	for _, child := range children {
		appendEvaluation(result, child)
	}
}

// evaluateAnyOfRules constructs the complete truth and children before publishing either.
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

	truth := compositionTruth{
		ruleIdentity: makeRuleIdentity(occurrence, oracleRuleAnyOf),
		branches:     make([]bool, len(node.anyOf)),
	}
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

	appendCompositionTruth(result, truth)

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

// rebaseCompositionOccurrence applies the shared child provenance policy to a composition branch.
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
