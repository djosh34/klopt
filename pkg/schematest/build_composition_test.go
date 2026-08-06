package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildMergesAllOfArrayItemSchemas verifies composed item witnesses.
func TestBuildMergesAllOfArrayItemSchemas(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"array",
		"items":{"type":"string"},
		"allOf":[{"items":{"enum":["z"]}}]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.NotEmpty(t, cases)
	require.Contains(t, cases, Case{JSON: []byte(`["z"]`), Valid: true})
}

// TestBuildMergesAllOfObjectPropertySchemas verifies composed property witnesses.
func TestBuildMergesAllOfObjectPropertySchemas(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"required":["x"],
		"properties":{"x":{"type":"string"}},
		"allOf":[{"properties":{"x":{"enum":["z"]}}}]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.NotEmpty(t, cases)
	require.Contains(t, cases, Case{JSON: []byte(`{"x":"z"}`), Valid: true})
}

// TestBuildKeepsAnyOfArrayBranchesAsAlternatives verifies exact array masks.
func TestBuildKeepsAnyOfArrayBranchesAsAlternatives(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"array",
		"items":{},
		"anyOf":[
			{"type":"array","items":{},"maxItems":0},
			{"type":"array","items":{},"minItems":1}
		]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Contains(t, cases, Case{JSON: []byte(`[]`), Valid: true})
	require.Contains(t, cases, Case{JSON: []byte(`[null]`), Valid: true})
}

// TestCompositionPinsMatchConcreteArrayInstances verifies wildcard pin matching.
func TestCompositionPinsMatchConcreteArrayInstances(t *testing.T) {
	t.Parallel()

	parent := schemaOccurrence{usePointer: "#/items", instanceTemplate: "#/*"}
	pin := anyOfValidPins(parent, 0)[0]
	concrete := schemaOccurrence{usePointer: "#/items", instanceTemplate: "#/0"}

	require.True(t, rowHasCompositionPins([]applicabilityPin{pin}, concrete, "anyOf"))
}

// TestCompositionCoverageScansAllWildcardTruthVectors verifies wildcard coverage.
func TestCompositionCoverageScansAllWildcardTruthVectors(t *testing.T) {
	t.Parallel()

	expectedOccurrence := schemaOccurrence{
		usePointer:       "#/schema",
		targetPointer:    "#/schema",
		instanceTemplate: "#/*",
	}
	expected := makeLevelIdentity(
		makeRuleIdentity(expectedOccurrence, oracleRuleAnyOf),
		planLevelMask+"2",
	)

	result := evaluation{anyOf: []compositionTruth{
		{
			ruleIdentity: makeRuleIdentity(
				schemaOccurrence{
					usePointer:       "#/schema",
					targetPointer:    "#/schema",
					instanceTemplate: "#/0",
				},
				oracleRuleAnyOf,
			),
			branches: []bool{true, false},
		},
		{
			ruleIdentity: makeRuleIdentity(
				schemaOccurrence{
					usePointer:       "#/schema",
					targetPointer:    "#/schema",
					instanceTemplate: "#/1",
				},
				oracleRuleAnyOf,
			),
			branches: []bool{false, true},
		},
	}}

	require.True(t, compositionLevelWasObserved(result, expected))
}

// TestBuildKeepsAnyOfRequiredMembersInTheirBranch verifies branch-local requiredness.
func TestBuildKeepsAnyOfRequiredMembersInTheirBranch(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"anyOf":[
			{"required":["a"],"properties":{"a":{"type":"string"}}},
			{"required":["b"],"properties":{"b":{"type":"number"}}}
		]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Contains(t, cases, Case{JSON: []byte(`{"a":""}`), Valid: true})
	require.Contains(t, cases, Case{JSON: []byte(`{"b":-1}`), Valid: true})
}
