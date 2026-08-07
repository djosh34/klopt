//nolint:godoclint // Focused private composition-fault tests use behavior names.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllOfFaultKeepsSiblingBranchesTrue(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"object",
		"allOf":[
			{"required":["a"],"properties":{"a":{"type":"string"}}},
			{"required":["b"],"properties":{"b":{"type":"string"}}}
		]
	}`)
	fault := findFaultTarget(t, plan, "/allOf/0|#/a|required|fault:required")
	searchState := &search{model: model, maxSteps: 100_000}

	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, `{"a":"","b":""}`, string(marshalFaultTestValue(t, parent)))

	derivative, err := applyFault(parent, fault, searchState)
	require.NoError(t, err)
	require.Equal(t, `{"b":""}`, string(marshalFaultTestValue(t, derivative)))
	require.Equal(t, `{"a":"","b":""}`, string(marshalFaultTestValue(t, parent)))

	result := evaluate(model, derivative)
	require.Equal(t, identityStrings(fault.closure), identityStrings(result.failures))
	require.Equal(t, [][]bool{{false, true}}, compositionTruthVectorsForTest(result.allOf))
}

func TestAnyOfAggregateFaultAtomicallyMakesEveryBranchFalse(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"object",
		"properties":{"a":{"type":"string"},"b":{"type":"string"}},
		"anyOf":[{"required":["a"]},{"required":["b"]}]
	}`)
	fault := findFaultTarget(t, plan, "|anyOf|fault:anyOf")
	searchState := &search{model: model, maxSteps: 100_000}

	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, `{"a":"","b":""}`, string(marshalFaultTestValue(t, parent)))

	stepsBeforeFault := searchState.steps
	derivative, err := applyFault(parent, fault, searchState)
	require.NoError(t, err)
	require.Equal(t, `{}`, string(marshalFaultTestValue(t, derivative)))
	require.Greater(t, searchState.steps, stepsBeforeFault)
	require.Equal(t, `{"a":"","b":""}`, string(marshalFaultTestValue(t, parent)))

	result := evaluate(model, derivative)
	matches, err := exactFailureClosure(result.failures, fault.closure)
	require.NoError(t, err)
	require.True(t, matches)
	require.Equal(t, [][]bool{{false, false}}, compositionTruthVectorsForTest(result.anyOf))
}

func TestBuildCompositionFaultGoldenStream(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"properties":{"a":{"type":"string"},"b":{"type":"string"}},
		"anyOf":[{"required":["a"]},{"required":["b"]}]
	}`))
	collect := func(maxSteps uint64) ([]Case, Report) {
		var cases []Case

		report, err := Build(
			Input{OpenAPI: document, OperationID: "selected", MaxSteps: maxSteps},
			func(testCase Case) error {
				cases = append(cases, testCase)

				return nil
			},
		)
		require.NoError(t, err)

		return cases, report
	}

	cases, report := collect(1_000_000)
	require.Equal(t, []Case{
		{JSON: []byte(`{"a":""}`), Valid: true},
		{JSON: []byte(`{"a":""}`), Valid: true},
		{JSON: []byte(`{"a":""}`), Valid: true},
		{JSON: []byte(`{"a":""}`), Valid: true},
		{JSON: []byte(`{"b":""}`), Valid: true},
		{JSON: []byte(`{"b":""}`), Valid: true},
		{JSON: []byte(`{"a":""}`), Valid: true},
		{JSON: []byte(`{}`), Valid: false},
		{JSON: []byte(`{}`), Valid: false},
		{JSON: []byte(`{}`), Valid: false},
	}, cases)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Equal(t, uint64(512), report.Steps)
	require.Len(t, report.Covered, 11)
	require.Len(t, report.Uncovered, 13)

	cutoffCases, cutoffReport := collect(report.Steps - 1)
	require.Equal(t, cases[:len(cases)-1], cutoffCases)
	require.Equal(t, MaxStepsReached, cutoffReport.Stop)
	require.Equal(t, report.Steps-1, cutoffReport.Steps)
	require.Contains(t, cutoffReport.Uncovered,
		"#/paths/~1/post/requestBody/content/application~1json/schema/anyOf/1|#/b|required|fault:required")
}

func compositionFaultModel(t *testing.T, schema string) (*schemaModel, *searchPlan) {
	t.Helper()

	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(schema)), OperationID: "selected",
	})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	return model, plan
}
