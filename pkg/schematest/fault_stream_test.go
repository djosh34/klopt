//nolint:godoclint // Focused private fault-stream tests use behavior names.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFaultProductExhaustionWaitsForEveryReachableNestedCursor(t *testing.T) {
	t.Parallel()

	exhaustion := newFaultProductExhaustion()
	setFaultFiniteEndpoint(&exhaustion.closureFinite, &exhaustion.closureSize, 1)
	closure := exhaustion.closure(0)
	setFaultFiniteEndpoint(&closure.parentFinite, &closure.parentSize, 2)

	first := exhaustion.parent(0, 0)
	setFaultFiniteEndpoint(&first.occurrenceFinite, &first.occurrenceSize, 0)

	second := exhaustion.parent(0, 1)
	setFaultFiniteEndpoint(&second.occurrenceFinite, &second.occurrenceSize, 1)
	exhaustion.occurrence(0, 1, 0)
	require.False(t, exhaustion.complete())

	mutation := exhaustion.occurrence(0, 1, 0)
	setFaultFiniteEndpoint(&mutation.mutationFinite, &mutation.mutationSize, 1)
	require.True(t, exhaustion.complete())
}

func TestParentRowMachineYieldsPastAnUnproductiveFirstMask(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"anyOf":[
			{"type":"string","minLength":2,"maxLength":1},
			{"enum":[0]}
		]
	}`)
	fault := findFaultTarget(t, plan, "|anyOf|fault:anyOf")
	searchState := &search{model: model, maxSteps: 1_000}
	machines := newFaultSearchMachines(searchState)

	var parent *jsonValue

	for rank := uint64(0); rank < 128; rank++ {
		candidate, found, _, err := machines.parentAtRank(plan, fault, rank)
		require.NoError(t, err)

		if found {
			parent = candidate

			break
		}
	}

	require.NotNil(t, parent)
	require.Equal(t, `0`, string(marshalFaultTestValue(t, parent)))
	require.Less(t, searchState.steps, searchState.maxSteps)
}

func TestScalarCandidateMachineYieldsBetweenDirectAddresses(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"string",
		"minLength":2,
		"maxLength":4
	}`)
	fault := findFaultTarget(t, plan, "|minLength|fault:minLength")
	searchState := &search{model: model, maxSteps: 10_000}
	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	machine := scalarCandidateMachine{search: searchState}

	var derivative *jsonValue

	for rank := uint64(0); rank < 128; rank++ {
		candidate, attempted, exhausted, attemptErr := machine.AttemptAtRank(parent, fault, rank)
		require.NoError(t, attemptErr)
		require.False(t, exhausted)

		if attempted && candidate != nil {
			derivative = candidate

			break
		}
	}

	require.NotNil(t, derivative)
	result := evaluate(model, derivative)
	require.NoError(t, result.err)
	require.False(t, result.valid)
	matches, err := faultFailureClosureMatches(result, fault)
	require.NoError(t, err)
	require.True(t, matches)
	require.Less(t, searchState.steps, searchState.maxSteps)
}

func TestBuildFaultProductContinuesPastUnsuitableRequiredParent(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"minProperties":2,
		"required":["id"],
		"properties":{"extra":{"type":"string"},"id":{"type":"string"},"spare":{"type":"string"}},
		"additionalProperties":false
	}`))

	var cases []Case

	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100_000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)
	require.NoError(t, err)
	require.Contains(t, report.Covered,
		"#/paths/~1/post/requestBody/content/application~1json/schema|#/id|required|fault:required")
	require.Contains(t, cases, Case{JSON: []byte(`{"extra":"","spare":""}`), Valid: false})
}

func TestScalarFaultAdvancesToParentWithExactWholeContainerClosure(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"array",
		"enum":[["aa","xx"],["aa","yy"],["b","yy"]],
		"items":{"type":"string","minLength":2}
	}`)
	fault := findFaultTarget(t, plan, "/items|#/*|minLength|fault:minLength")
	searchState := &search{model: model, maxSteps: 100_000}

	var generated Case

	err := streamFault(plan, fault, searchState, make(map[string]bool), func(testCase Case) error {
		generated = testCase

		return nil
	})
	require.NoError(t, err)
	require.JSONEq(t, `["b","yy"]`, string(generated.JSON))
}

func TestAdditionalPropertyFaultAdvancesToLaterParent(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"object",
		"enum":[{}, {"state":0}, {"state":0,"x":1}],
		"properties":{"x":{"enum":[1]}},
		"allOf":[{
			"properties":{"state":{"enum":[0]}},
			"additionalProperties":false
		}]
	}`)
	fault := findFaultTarget(t, plan, "/allOf/0|#/*|additionalProperties|fault:additionalProperties")
	searchState := &search{model: model, maxSteps: 100_000}

	var generated Case

	err := streamFault(plan, fault, searchState, make(map[string]bool), func(testCase Case) error {
		generated = testCase

		return nil
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"state":0,"x":1}`, string(generated.JSON))
}

func TestBuildNonCompositionFaultsAreDeterministicAndCutOffAtomically(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{"type":"number","minimum":1,"maximum":2}`))
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

	firstCases, firstReport := collect(10_000)
	secondCases, secondReport := collect(10_000)

	require.Equal(t, SpaceExhausted, firstReport.Stop)
	require.Equal(t, firstCases, secondCases)
	require.Equal(t, firstReport, secondReport)
	require.NotEmpty(t, firstCases)
	require.False(t, firstCases[len(firstCases)-1].Valid)

	cutoffCases, cutoffReport := collect(firstReport.Steps - 1)
	require.Equal(t, MaxStepsReached, cutoffReport.Stop)
	require.Equal(t, firstReport.Steps-1, cutoffReport.Steps)
	require.Equal(t, firstCases[:len(firstCases)-1], cutoffCases)
}
