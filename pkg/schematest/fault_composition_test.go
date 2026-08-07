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

func TestAnyOfAggregateFaultPreservesUnrelatedParentPaths(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"object",
		"required":["payload","keep"],
		"properties":{
			"keep":{"type":"string"},
			"payload":{
				"type":"object",
				"properties":{"a":{"type":"string"},"b":{"type":"string"}},
				"anyOf":[{"required":["a"]},{"required":["b"]}]
			}
		}
	}`)
	fault := findFaultTarget(t, plan, "/properties/payload|#/payload|anyOf|fault:anyOf")
	searchState := &search{model: model, maxSteps: 100_000}
	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	parent.object["keep"] = &jsonValue{kind: jsonString, text: "untouched"}
	parentJSON := marshalFaultTestValue(t, parent)

	derivative, err := applyFault(parent, fault, searchState)
	require.NoError(t, err)
	require.Equal(t, "untouched", derivative.object["keep"].text)
	require.Equal(t, parentJSON, marshalFaultTestValue(t, parent))
	require.Empty(t, derivative.object["payload"].object)

	cutoff := &search{model: model, maxSteps: searchState.steps}
	cutoff.steps = cutoff.maxSteps
	_, err = applyFault(parent, fault, cutoff)
	require.ErrorIs(t, err, errMaxSteps)
	require.Equal(t, parentJSON, marshalFaultTestValue(t, parent))
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
		{JSON: []byte(`null`), Valid: false},
		{JSON: []byte(`{}`), Valid: false},
		{JSON: []byte(`{}`), Valid: false},
		{JSON: []byte(`{}`), Valid: false},
		{JSON: []byte(`{"a":null}`), Valid: false},
		{JSON: []byte(`{"b":null}`), Valid: false},
	}, cases)

	prefix := "#/paths/~1/post/requestBody/content/application~1json/schema"
	expectedCovered := []string{
		prefix + "|#|type|level:object", prefix + "|#|type|fault:type",
		prefix + "|#|anyOf|level:mask:1", prefix + "|#|anyOf|fault:anyOf",
		prefix + "/anyOf/0|#|type|level:object", prefix + "/anyOf/0|#/a|required|level:present",
		prefix + "/anyOf/0|#/a|required|fault:required",
		prefix + "/anyOf/1|#|type|level:object", prefix + "/anyOf/1|#/b|required|level:present",
		prefix + "/anyOf/1|#/b|required|fault:required",
		prefix + "/properties/a|#/a|type|level:string", prefix + "/properties/a|#/a|type|fault:type",
		prefix + "/properties/b|#/b|type|level:string", prefix + "/properties/b|#/b|type|fault:type",
	}
	expectedUncovered := []string{
		prefix + "/anyOf/0|#|type|level:null", prefix + "/anyOf/0|#|type|level:boolean",
		prefix + "/anyOf/0|#|type|level:number", prefix + "/anyOf/0|#|type|level:string",
		prefix + "/anyOf/0|#|type|level:array", prefix + "/anyOf/1|#|type|level:null",
		prefix + "/anyOf/1|#|type|level:boolean", prefix + "/anyOf/1|#|type|level:number",
		prefix + "/anyOf/1|#|type|level:string", prefix + "/anyOf/1|#|type|level:array",
	}
	require.Equal(t, Report{
		Stop: SpaceExhausted, Steps: 519, Covered: expectedCovered, Uncovered: expectedUncovered,
	}, report)

	cutoffCases, cutoffReport := collect(report.Steps - 1)
	require.Equal(t, cases[:len(cases)-1], cutoffCases)
	require.Equal(t, Report{
		Stop: MaxStepsReached, Steps: 518,
		Covered:   expectedCovered[:len(expectedCovered)-1],
		Uncovered: append(append([]string(nil), expectedUncovered...), expectedCovered[len(expectedCovered)-1]),
	}, cutoffReport)
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
	require.Equal(t, []Case{
		{JSON: []byte(`{"a":"","b":""}`), Valid: true},
		{JSON: []byte(`{"a":"","b":""}`), Valid: true},
		{JSON: []byte(`{"a":"","b":""}`), Valid: true},
		{JSON: []byte(`{"a":"","b":""}`), Valid: true},
		{JSON: []byte(`{"a":"","b":""}`), Valid: true},
		{JSON: []byte(`{"a":"","b":""}`), Valid: true},
		{JSON: []byte(`{"a":"","b":""}`), Valid: true},
		{JSON: []byte(`{"a":"","b":""}`), Valid: true},
		{JSON: []byte(`null`), Valid: false},
		{JSON: []byte(`{"b":""}`), Valid: false},
		{JSON: []byte(`{"a":null,"b":""}`), Valid: false},
		{JSON: []byte(`{"a":""}`), Valid: false},
		{JSON: []byte(`{"a":"","b":null}`), Valid: false},
	}, cases)

	prefix := "#/paths/~1/post/requestBody/content/application~1json/schema"
	expectedCovered := []string{
		prefix + "|#|type|level:object", prefix + "|#|type|fault:type", prefix + "|#|allOf|level:all-true",
		prefix + "/allOf/0|#|type|level:object", prefix + "/allOf/0|#/a|required|level:present",
		prefix + "/allOf/0|#/a|required|fault:required",
		prefix + "/allOf/0/properties/a|#/a|type|level:string",
		prefix + "/allOf/0/properties/a|#/a|type|fault:type",
		prefix + "/allOf/1|#|type|level:object", prefix + "/allOf/1|#/b|required|level:present",
		prefix + "/allOf/1|#/b|required|fault:required",
		prefix + "/allOf/1/properties/b|#/b|type|level:string",
		prefix + "/allOf/1/properties/b|#/b|type|fault:type",
	}
	expectedUncovered := []string{
		prefix + "/allOf/0|#|type|level:null", prefix + "/allOf/0|#|type|level:boolean",
		prefix + "/allOf/0|#|type|level:number", prefix + "/allOf/0|#|type|level:string",
		prefix + "/allOf/0|#|type|level:array", prefix + "/allOf/1|#|type|level:null",
		prefix + "/allOf/1|#|type|level:boolean", prefix + "/allOf/1|#|type|level:number",
		prefix + "/allOf/1|#|type|level:string", prefix + "/allOf/1|#|type|level:array",
	}
	require.Equal(t, Report{
		Stop: SpaceExhausted, Steps: 429, Covered: expectedCovered, Uncovered: expectedUncovered,
	}, report)

	cutoffCases, cutoffReport := collect(report.Steps - 1)
	require.Equal(t, cases[:len(cases)-1], cutoffCases)
	require.Equal(t, Report{
		Stop: MaxStepsReached, Steps: 428,
		Covered:   expectedCovered[:len(expectedCovered)-1],
		Uncovered: append(append([]string(nil), expectedUncovered...), expectedCovered[len(expectedCovered)-1]),
	}, cutoffReport)
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
