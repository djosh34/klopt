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

	truthIndex := len(result.allOf)
	appendAllOfTruth(result, compositionTruth{
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

		result.allOf[truthIndex].branches[index] = childResult.valid
		mergeEvaluation(result, childResult)
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

	truthIndex := len(result.anyOf)
	appendAnyOfTruth(result, compositionTruth{
		ruleIdentity: makeRuleIdentity(occurrence, oracleRuleAnyOf),
		branches:     make([]bool, len(node.anyOf)),
	})

	branchFailures := make([]failureIdentity, 0)
	branchFailureRecords := &evaluationRecordList[failureIdentity]{}
	branchFailed := false
	anyBranchValid := false

	for index, child := range node.anyOf {
		childOccurrence := rebaseCompositionOccurrence(child, occurrence, "anyOf", index)
		childResult := context.evaluateNode(child, value, childOccurrence)

		if childResult.err != nil {
			result.err = fmt.Errorf("%s branch %d: %w", occurrence.usePointer+"/anyOf", index, childResult.err)

			return
		}

		result.anyOf[truthIndex].branches[index] = childResult.valid
		mergeEvaluationRecords(result, childResult)

		if childResult.valid {
			anyBranchValid = true

			continue
		}

		branchFailed = branchFailed || childResult.failed
		branchFailureRecords.appendList(&childResult.records.failures, occurrenceTransform{})

		if !childResult.fromCache || childResult.materialized {
			branchFailures = append(branchFailures, childResult.failures...)
		}
	}

	if anyBranchValid {
		return
	}

	result.failed = result.failed || branchFailed
	ensureEvaluationRecords(result)
	result.records.failures.appendList(branchFailureRecords, occurrenceTransform{})
	result.failures = append(result.failures, branchFailures...)
	appendFailure(result, makeRuleIdentity(occurrence, oracleRuleAnyOf))
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
		occurrence.usePointer+"/"+composition+"/"+strconv.Itoa(index),
		occurrence.instanceTemplate,
	)
}
