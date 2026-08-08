package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMakePlanEmitsOneBaselineForOneLevelRules proves single-level collapse.
func TestMakePlanEmitsOneBaselineForOneLevelRules(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string","minLength":1,"maxLength":3,"format":"email"
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	require.Len(t, plan.validCatalog, 4)
	require.Len(t, plan.validSchedule, 1)
	require.Len(t, plan.validSchedule[0].targets, 4)
	require.Equal(t, []string{
		"minLength|valid", "maxLength|valid", "format|valid",
	}, levelRuleNames(plan.stringObjectives))
	require.Len(t, plan.faultExecution, len(plan.faultSchedule))

	for index := range plan.faultExecution {
		require.Equal(t, index, plan.faultExecution[index])
	}
}

// TestMakePlanEmitsAdditiveValidSchedule proves the mixed-radix formula.
func TestMakePlanEmitsAdditiveValidSchedule(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string","enum":["a","b"],
		"anyOf":[{"type":"string"},{"type":"string"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	// type has one level, enum has two, and anyOf has three masks.
	require.Equal(t, 4, len(plan.validSchedule))

	baseline := plan.validSchedule[0]
	require.Equal(t, []string{
		"type|string",
		"enum|member:0",
		"anyOf|mask:3",
		"type|string",
		"type|string",
	}, validRequestLevels(baseline))

	for _, request := range plan.validSchedule[1:] {
		require.Equal(t, 1, changedValidRequestLevels(baseline, request))
	}

	require.Equal(t, []string{
		"enum|member:1",
		"anyOf|mask:1",
		"anyOf|mask:2",
	}, validAlternativeLevels(plan.validSchedule[1:]))
}

// TestMakePlanTypelessBaselineStartsBoolean locks the non-null canonical kind.
func TestMakePlanTypelessOrderIgnoresEnumSiblingCompatibility(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{"enum":[1]}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)
	require.Equal(t, []string{
		"type|boolean", "type|null", "type|number", "type|string", "type|array", "type|object",
	}, validTypeLevels(plan.validCatalog, model.root.occurrence))
}

// validTypeLevels renders root type levels in catalog order.
func validTypeLevels(catalog []validIntent, occurrence schemaOccurrence) []string {
	var levels []string

	for _, target := range catalog {
		if target.expected.rule == oracleRuleType && target.expected.occurrence == occurrence {
			levels = append(levels, target.expected.rule+"|"+target.expected.level)
		}
	}

	return levels
}

// TestMakePlanTypelessBaselineStartsBoolean locks the first additive type assignment.
func TestMakePlanTypelessBaselineStartsBoolean(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)
	require.Len(t, plan.validSchedule, 6)
	require.Equal(t, "boolean", plan.validSchedule[0].targets[0].expected.level)
	require.Equal(t, []string{
		"type|null", "type|number", "type|string", "type|array", "type|object",
	}, validAlternativeLevels(plan.validSchedule[1:]))
}

// TestValidStringObjectiveConsumesExplicitExecutionOrder proves production consumes compiled order.
func TestValidStringObjectiveConsumesExplicitExecutionOrder(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string","minLength":1,"maxLength":3,"pattern":"^a+$","format":"email"
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	baseline := plan.validSchedule[0]
	objective := validStringObjective(&baseline, model.root, model.root.occurrence)
	require.NotNil(t, objective)
	require.Equal(t, oracleRuleMinLength, objective.identity.rule)
	require.Same(t, model.root, objective.node)

	focus := -1

	for index, target := range baseline.targets {
		if target.expected.rule == oracleRulePattern {
			focus = index

			break
		}
	}

	require.NotEqual(t, -1, focus)

	focused, err := makeValidRequest(baseline.targets, focus, plan.stringObjectives)
	require.NoError(t, err)

	objective = validStringObjective(&focused, model.root, model.root.occurrence)
	require.NotNil(t, objective)
	require.Equal(t, oracleRulePattern, objective.identity.rule)
	require.Equal(t, oracleRulePattern, focused.stringObjectives[0].rule)
}

// TestMakePlanCachesCanonicalObligationOrderKeys proves one-time order parsing.
func TestMakePlanCachesCanonicalObligationOrderKeys(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object","properties":{"value":{"type":"string"}}
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	for _, obligation := range plan.obligations {
		require.NotNil(t, obligation.orderKey)
	}

	for _, target := range plan.validCatalog {
		require.NotNil(t, target.obligation.orderKey)
	}

	for _, fault := range plan.faultSchedule {
		require.NotNil(t, fault.obligation.orderKey)
	}
}

// levelRuleNames renders test-only string objective identities.
func levelRuleNames(levels []levelIdentity) []string {
	result := make([]string, 0, len(levels))
	for _, level := range levels {
		result = append(result, level.rule+"|"+level.level)
	}

	return result
}

// validRequestLevels renders one test-only assignment vector.
func validRequestLevels(request validRequest) []string {
	levels := make([]string, 0, len(request.targets))
	for _, target := range request.targets {
		levels = append(levels, target.expected.rule+"|"+target.expected.level)
	}

	return levels
}

// changedValidRequestLevels counts replacements against the baseline.
func changedValidRequestLevels(baseline, request validRequest) int {
	changed := 0

	for index := range baseline.targets {
		if baseline.targets[index].expected != request.targets[index].expected {
			changed++
		}
	}

	return changed
}

// validAlternativeLevels renders focused replacements in execution order.
func validAlternativeLevels(requests []validRequest) []string {
	levels := make([]string, 0, len(requests))
	for _, request := range requests {
		requirement := request.targets[request.focus]
		levels = append(levels, requirement.expected.rule+"|"+requirement.expected.level)
	}

	return levels
}
