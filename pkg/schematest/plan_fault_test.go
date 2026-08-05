//nolint:godoclint // Fault-planning tables pin the private planner seam.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakePlanAnyOfWitnessCountsUseExactValues(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{"minLength":1000}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)
	findValidTarget(t, plan, "|anyOf|level:mask:1")
}

func TestMakePlanOmitsAnyOfFaultsIncompatibleWithParentType(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"anyOf":[{"type":"string"}, {"type":"number"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	for _, target := range plan.faultTargets {
		require.NotEqual(t, oracleRuleAnyOf, target.obligation.rule)
		require.NotContains(t, target.obligation.occurrence.usePointer, "/anyOf/")
	}
}

func TestMakePlanAnyOfUsesWitnessBeyondExclusiveNumberBound(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{"type":"number","minimum":5,"exclusiveMinimum":true}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)
	findValidTarget(t, plan, "|anyOf|level:mask:1")
}

func TestMakePlanDirectedFaultsUseTheirOwnAnyOfApplicability(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"pattern":"^b",
		"anyOf":[{"pattern":"^a"}, {"pattern":"^b"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	fault := findFaultTarget(t, plan, "|pattern|fault:pattern")
	requireCompositionPin(t, fault.pins, "anyOf", 0, true)
	requireCompositionPin(t, fault.pins, "anyOf", 1, false)
}

func TestMakePlanTypeFaultUsesWrongKindAnyOfApplicability(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"anyOf":[{"type":"number"}, {"type":"string"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	fault := findFaultTarget(t, plan, "|type|fault:type")
	requireCompositionPin(t, fault.pins, "anyOf", 0, true)
	requireCompositionPin(t, fault.pins, "anyOf", 1, false)
}

func TestMakePlanAdditionalPropertyFaultUsesAnAddedMemberAnyOfMask(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object",
		"additionalProperties":false,
		"anyOf":[{"type":"object","additionalProperties":false}, {"type":"object"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	fault := findFaultTarget(t, plan, "|#/*|additionalProperties|fault:additionalProperties")
	requireCompositionPin(t, fault.pins, "anyOf", 0, false)
	requireCompositionPin(t, fault.pins, "anyOf", 1, true)
}

func TestMakePlanRequiredFaultUsesAnOmittedMemberAnyOfMask(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object",
		"required":["id"],
		"anyOf":[{"required":["id"]}, {"type":"object"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	fault := findFaultTarget(t, plan, "|#/id|required|fault:required")
	requireCompositionPin(t, fault.pins, "anyOf", 0, false)
	requireCompositionPin(t, fault.pins, "anyOf", 1, true)
}

func TestMakePlanEnumFaultPinsTheDeclaredKind(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"enum":["ok"]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	fault := findFaultTarget(t, plan, "|enum|fault:enum")
	requireKindPin(t, fault.pins, fault.obligation.occurrence, jsonString)
}

func TestMakePlanIntegerTypeFaultUsesFractionalAnyOfApplicability(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"integer",
		"anyOf":[{"type":"integer"}, {"type":"number"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	fault := findFaultTarget(t, plan, "|type|fault:type")
	requireCompositionPin(t, fault.pins, "anyOf", 0, false)
	requireCompositionPin(t, fault.pins, "anyOf", 1, true)
}

func TestMakePlanFaultClosureDoesNotIncludeInapplicableLocalRules(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"minLength":1,
		"format":"date"
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	typeFault := findFaultTarget(t, plan, "|type|fault:type")
	require.Equal(t, []string{typeFault.obligation.ruleIdentity.String()}, identityStrings(typeFault.closure))
}
