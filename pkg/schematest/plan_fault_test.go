//nolint:godoclint // Fault-planning tables pin the private planner seam.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakePlanMinLengthZeroTerminates(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"anyOf":[{"minLength":0}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)
	findValidTarget(t, plan, "|minLength|level:valid")
}

func TestMakePlanRetainsAnyOfObligationBeyondStringWitnessGuard(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"anyOf":[{"minLength":4097}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)
	findValidTarget(t, plan, "|anyOf|level:mask:1")
}

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
		"pattern":"^z$",
		"anyOf":[{"pattern":"^z$"}, {"pattern":"^y$"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	fault := findFaultTarget(t, plan, "|pattern|fault:pattern")
	requireCompositionPin(t, fault.pins, "anyOf", 0, false)
	requireCompositionPin(t, fault.pins, "anyOf", 1, true)
}

func TestMakePlanMaxLengthFaultUsesExactAnyOfApplicability(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"maxLength":0,
		"anyOf":[
			{"type":"string","pattern":"^z$","maxLength":0},
			{"type":"string","pattern":"^q$"}
		]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	fault := findFaultTarget(t, plan, "|maxLength|fault:maxLength")
	requireCompositionPin(t, fault.pins, "anyOf", 0, false)
	requireCompositionPin(t, fault.pins, "anyOf", 1, true)
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

func TestFaultCandidateSupportsKindRejectsConflictingSameInstancePins(t *testing.T) {
	t.Parallel()

	occurrence := schemaOccurrence{usePointer: "#/branch", instanceTemplate: "#"}
	candidate := faultTarget{pins: []applicabilityPin{
		kindPin(occurrence, jsonString),
		kindPin(occurrence, jsonNumber),
	}}

	branch := &schemaNode{schemaShape: &schemaShape{}}
	require.False(t, faultCandidateSupportsKind(candidate, branch, occurrence, jsonString))
}

func TestMakePlanPreservesNestedAllOfFaultsInAnyOfAggregate(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"anyOf":[{"allOf":[{"type":"number"}]}, {"type":"number"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	aggregate := findFaultTarget(t, plan, "|anyOf|fault:anyOf")
	require.Equal(t, []string{
		aggregate.obligation.ruleIdentity.String(),
		aggregate.obligation.occurrence.usePointer + "/anyOf/0/allOf/0|#|type",
		aggregate.obligation.occurrence.usePointer + "/anyOf/1|#|type",
	}, identityStrings(aggregate.closure))
	requireKindPin(t, aggregate.pins, aggregate.obligation.occurrence, jsonString)
}

func TestMakePlanAnyOfAggregateUsesInheritedKindPins(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"allOf":[{"type":"string"}],
		"anyOf":[{"type":"number"}, {"type":"boolean"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	aggregate := findFaultTarget(t, plan, "|anyOf|fault:anyOf")
	requireKindPin(t, aggregate.pins, aggregate.obligation.occurrence, jsonString)
	requireCompositionPin(t, aggregate.pins, "anyOf", 0, false)
	requireCompositionPin(t, aggregate.pins, "anyOf", 1, false)
	require.Equal(t, []string{
		aggregate.obligation.ruleIdentity.String(),
		aggregate.obligation.occurrence.usePointer + "/anyOf/0|#|type",
		aggregate.obligation.occurrence.usePointer + "/anyOf/1|#|type",
	}, identityStrings(aggregate.closure))
}

func TestMakePlanMaximumFaultUsesDirectedBoundWitness(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"number",
		"maximum":100,
		"anyOf":[{"type":"number"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	fault := findFaultTarget(t, plan, "|maximum|fault:maximum")
	requireCompositionPin(t, fault.pins, "anyOf", 0, true)
	require.Equal(t, []string{fault.obligation.ruleIdentity.String()}, identityStrings(fault.closure))
}

func TestMakePlanRequiredFaultsPopulateUnaffectedSiblings(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object",
		"required":["id","name"],
		"anyOf":[{}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	idFault := findFaultTarget(t, plan, "|#/id|required|fault:required")
	requirePin(t, idFault.pins, "#/id", planPinAbsent)
	requirePin(t, idFault.pins, "#/name", planPinPresent)
	requireCompositionPin(t, idFault.pins, "anyOf", 0, true)

	nameFault := findFaultTarget(t, plan, "|#/name|required|fault:required")
	requirePin(t, nameFault.pins, "#/name", planPinAbsent)
	requirePin(t, nameFault.pins, "#/id", planPinPresent)
	requireCompositionPin(t, nameFault.pins, "anyOf", 0, true)
}

func TestMakePlanEnumFaultUsesStringOutsideCannedEnum(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"enum":["","a","b","text"]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	fault := findFaultTarget(t, plan, "|enum|fault:enum")
	requireKindPin(t, fault.pins, fault.obligation.occurrence, jsonString)
	require.Equal(t, []string{fault.obligation.ruleIdentity.String()}, identityStrings(fault.closure))
}

func TestMakePlanOmitsContradictoryAnyOfScalarLevel(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"anyOf":[{"minLength":2,"maxLength":1}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	for _, target := range plan.validTargets {
		require.NotEqual(t, "|anyOf|level:mask:1", target.obligation.String())
	}
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

func TestMakePlanAdditionalPropertyWitnessAvoidsAuthoredName(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object",
		"properties":{"__schematest_extra__":{"type":"boolean"},"a":{"type":"string"}},
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

func TestMakePlanEnumFaultUsesNullableKindWhenSiblingRulesExhaustStrings(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"nullable":true,
		"enum":[""],
		"maxLength":0
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	fault := findFaultTarget(t, plan, "|enum|fault:enum")
	requireKindPin(t, fault.pins, fault.obligation.occurrence, jsonNull)
	require.Equal(t, []string{fault.obligation.ruleIdentity.String()}, identityStrings(fault.closure))
}

func TestMakePlanOmitsExhaustiveNullableBooleanEnumFault(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"boolean",
		"nullable":true,
		"enum":[false,true,null]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	for _, target := range plan.faultTargets {
		require.NotEqual(t, oracleRuleEnum, target.obligation.rule)
	}
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
