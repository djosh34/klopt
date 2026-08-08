//nolint:godoclint // Fault-planning tests requirement the private declarative seam.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakePlanCompilesTypedObjectFaultRolesWithoutWitnesses(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object",
		"required":["id","name"],
		"properties":{"id":{"type":"number"},"name":{"type":"string"}},
		"additionalProperties":false
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	required := findFaultTarget(t, plan, "|#/id|required|fault:required")
	requireRequirement(t, required.requirements, "#/id", requirementAbsent)
	requireRequirement(t, required.requirements, "#/name", requirementPresent)
	requireKindRequirement(t, required.requirements, model.root.occurrence, jsonObject)
	require.Equal(t, failureSet{failureIdentity(required.obligation.ruleIdentity)}, required.expected)
	require.Nil(t, required.alternatives)

	additional := findFaultTarget(t, plan, "|#/*|additionalProperties|fault:additionalProperties")
	requireRequirement(t, additional.requirements, "#/*", requirementPresent)
	requireKindRequirement(t, additional.requirements, model.root.occurrence, jsonObject)
	require.Equal(t, failureSet{failureIdentity(additional.obligation.ruleIdentity)}, additional.expected)
	require.Nil(t, additional.alternatives)
}

func TestMakePlanCompilesNestedAnyOfClosureDomainsWithoutInheritedKindRequirements(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"allOf":[{"type":"string"}],
		"anyOf":[
			{"allOf":[{"type":"number"}]},
			{"type":"boolean","enum":[true,false]}
		]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	aggregate := findFaultTarget(t, plan, "|anyOf|fault:anyOf")
	require.Equal(t, failureSet{failureIdentity(aggregate.obligation.ruleIdentity)}, aggregate.expected)
	requireNoKindRequirement(t, aggregate.requirements, aggregate.obligation.occurrence)
	requireCompositionRequirement(t, aggregate.requirements, "anyOf", 0, false)
	requireCompositionRequirement(t, aggregate.requirements, "anyOf", 1, false)

	first := aggregate.alternatives
	require.NotNil(t, first)
	require.NotNil(t, first.alternatives)
	require.Contains(t, identityStrings(first.alternatives.expected),
		aggregate.obligation.occurrence.usePointer+"/anyOf/0/allOf/0|#|type")

	second := first.next
	require.NotNil(t, second)
	require.NotNil(t, second.alternatives)
	require.Contains(t, identityStrings(second.alternatives.expected),
		aggregate.obligation.occurrence.usePointer+"/anyOf/1|#|type")
	require.NotNil(t, second.alternatives.next, "every branch-local fault remains an alternative")
	require.Nil(t, second.next)
}

func TestMakePlanKeepsSymbolicAggregateWithEmptyBranchDomain(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{}, {"type":"string"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	aggregate := findFaultTarget(t, plan, "|anyOf|fault:anyOf")
	require.NotNil(t, aggregate.alternatives)
	require.Nil(t, aggregate.alternatives.alternatives)
	require.NotNil(t, aggregate.alternatives.next)
	require.NotNil(t, aggregate.alternatives.next.alternatives)
}

func TestMakePlanRetainsFaultsWithoutFiniteRealizabilityProof(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"boolean",
		"nullable":true,
		"enum":[false,true,null],
		"anyOf":[{"minLength":2,"maxLength":1}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	findFaultTarget(t, plan, "|type|fault:type")
	findFaultTarget(t, plan, "|enum|fault:enum")
	findFaultTarget(t, plan, "|anyOf|fault:anyOf")
	findValidTarget(t, plan, "|anyOf|level:mask:1")
}

func requireNoKindRequirement(t *testing.T, requirements []requirement, occurrence schemaOccurrence) {
	t.Helper()

	for _, requirement := range requirements {
		require.False(t, requirement.hasKind && rowOccurrenceMatches(requirement.occurrence, occurrence), requirements)
	}
}
