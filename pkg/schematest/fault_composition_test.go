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
	require.Equal(t, identityStrings(fault.expected), identityStrings(result.failureRecords()))
	require.Equal(t, [][]bool{{false, true}}, compositionTruthVectorsForTest(result.compositionRecords(oracleRuleAllOf)))
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
	require.Contains(t, cases, Case{JSON: []byte(`{"a":""}`), Valid: true})
	require.Contains(t, cases, Case{JSON: []byte(`{"b":""}`), Valid: true})
	require.Contains(t, cases, Case{JSON: []byte(`{"a":"","b":""}`), Valid: true})
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Equal(t, []string{"mask:1", "mask:2", "mask:3"}, anyOfReportMasks(report.Covered))
	require.True(t, reportIdentityHasSuffix(report.Uncovered, "|anyOf|fault:anyOf"))
}

func TestBuildAllOfCompositionFaultGoldenStream(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"allOf":[
			{"required":["a"],"properties":{"a":{"type":"string"}}},
			{"required":["b"],"properties":{"b":{"type":"string"}}}
		]
	}`))
	collect := func(maxSteps uint64) ([]Case, Report) {
		var cases []Case

		report, err := Build(Input{
			OpenAPI: document, OperationID: "selected", MaxSteps: maxSteps,
		}, func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		})
		require.NoError(t, err)

		return cases, report
	}

	cases, report := collect(1_000_000)
	require.Len(t, cases, 12)

	for _, testCase := range cases[:7] {
		require.Equal(t, Case{JSON: []byte(`{"a":"","b":""}`), Valid: true}, testCase)
	}

	require.Equal(t, []Case{
		{JSON: []byte(`null`), Valid: false},
		{JSON: []byte(`{"b":""}`), Valid: false},
		{JSON: []byte(`{"a":null,"b":""}`), Valid: false},
		{JSON: []byte(`{"a":""}`), Valid: false},
		{JSON: []byte(`{"a":"","b":null}`), Valid: false},
	}, cases[7:])
	require.Equal(t, SpaceExhausted, report.Stop)
	require.True(t, reportIdentityHasSuffix(report.Covered, "|allOf|level:all-true"))
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
