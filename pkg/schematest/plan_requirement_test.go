package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPlanRequirementsDescribeAuthoredEnumTargetWithoutCandidate verifies declarative enum intent.
func TestPlanRequirementsDescribeAuthoredEnumTargetWithoutCandidate(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"enum":["a","b"],
		"minLength":1
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	intent := findValidTarget(t, plan, "|enum|level:member:1")
	require.Contains(t, intent.requirements, requirement{
		tag: requirementActiveRules, occurrence: model.root.occurrence, active: model.root,
	})
	require.Contains(t, intent.requirements, requirement{
		tag: requirementTargetLevel, occurrence: intent.expected.occurrence, target: intent.expected,
	})
	require.Contains(t, intent.requirements, requirement{
		tag: requirementExactEnumMember, occurrence: model.root.occurrence, enumMember: &model.root.enum[1],
	})
	require.Contains(t, intent.requirements, kindPin(model.root.occurrence, jsonString))
}

// TestPlanRequirementsCarryExactCountAndFaultProgram verifies count and closure constraints.
func TestPlanRequirementsCarryExactCountAndFaultProgram(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object",
		"minProperties":2,
		"required":["id"]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	intent := findValidTarget(t, plan, "|minProperties|level:valid")
	require.Contains(t, intent.requirements, requirement{
		tag: requirementExactCount, occurrence: model.root.occurrence, count: model.root.minProperties,
	})

	fault := findFaultTarget(t, plan, "|#/id|required|fault:required")
	require.NotNil(t, fault.alternatives)
	require.Equal(t, fault.expected, fault.alternatives.expected)
}
