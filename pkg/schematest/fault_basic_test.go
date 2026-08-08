//nolint:godoclint // Focused private fault tests use behavior names.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegenerateParentAndApplyBasicTypeFault(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"type":"string"}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	fault := findFaultTarget(t, plan, "|type|fault:type")
	searchState := &search{model: model, maxSteps: 10}

	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, `""`, string(marshalFaultTestValue(t, parent)))
	require.Equal(t, uint64(2), searchState.steps)

	secondParent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.NotSame(t, parent, secondParent)
	require.Equal(t, uint64(4), searchState.steps)

	derivative, err := applyFault(secondParent, fault, searchState)
	require.NoError(t, err)
	require.Equal(t, `null`, string(marshalFaultTestValue(t, derivative)))
	require.Equal(t, `""`, string(marshalFaultTestValue(t, secondParent)))
	require.Equal(t, uint64(5), searchState.steps)

	result := evaluate(model, derivative)
	require.False(t, result.valid)
	require.Equal(t, identityStrings(fault.closure), identityStrings(result.failureRecords()))
}

func TestBuildStreamsBasicTypeFaultAfterValidTargets(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{"type":"string"}`))

	var cases []Case

	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 5},
		func(generated Case) error {
			cases = append(cases, generated)

			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, []Case{
		{JSON: []byte(`""`), Valid: true},
		{JSON: []byte(`null`), Valid: false},
	}, cases)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Equal(t, uint64(5), report.Steps)
	require.Empty(t, report.Uncovered)
}

func TestBuildDiscardsBasicFaultAtCutoff(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{"type":"string"}`))

	for _, maxSteps := range []uint64{3, 4} {
		var cases []Case

		report, err := Build(
			Input{OpenAPI: document, OperationID: "selected", MaxSteps: maxSteps},
			func(generated Case) error {
				cases = append(cases, generated)

				return nil
			},
		)
		require.NoError(t, err)
		require.Equal(t, []Case{{JSON: []byte(`""`), Valid: true}}, cases)
		require.Equal(t, MaxStepsReached, report.Stop)
		require.Equal(t, maxSteps, report.Steps)
		require.Len(t, report.Uncovered, 1)
		require.Contains(t, report.Uncovered[0], "|type|fault:type")
	}
}

func TestRegenerateParentPreservesFaultKindPins(t *testing.T) {
	t.Parallel()

	for name, schema := range map[string]string{
		"typeless minLength":       `{"minLength":2}`,
		"mixed enum and minLength": `{"enum":["ok",1],"minLength":2}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			model, plan := compositionFaultModel(t, schema)
			fault := findFaultTarget(t, plan, "|minLength|fault:minLength")
			searchState := &search{model: model, maxSteps: 100_000}

			first, found, err := regenerateParent(plan, fault, searchState)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, jsonString, first.kind)
			firstJSON := marshalFaultTestValue(t, first)

			second, found, err := regenerateParent(plan, fault, searchState)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, firstJSON, marshalFaultTestValue(t, second))
		})
	}
}

func TestBuildVisitsMaximumFaultInsideAnyOfContext(t *testing.T) {
	t.Parallel()

	var cases []Case

	report, err := Build(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"number","maximum":100,"anyOf":[{"type":"number"}]
		}`)),
		OperationID: "selected",
		MaxSteps:    1_000_000,
	}, func(testCase Case) error {
		cases = append(cases, testCase)

		return nil
	})
	require.NoError(t, err)
	require.Equal(t, []Case{
		{JSON: []byte(`100`), Valid: true},
		{JSON: []byte(`100`), Valid: true},
		{JSON: []byte(`100`), Valid: true},
		{JSON: []byte(`100`), Valid: true},
		{JSON: []byte(`101`), Valid: false},
	}, cases)
	require.Equal(t, Report{
		Stop:  SpaceExhausted,
		Steps: 21,
		Covered: []string{
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:number",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|maximum|level:valid",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|maximum|fault:maximum",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|anyOf|level:mask:1",
			"#/paths/~1/post/requestBody/content/application~1json/schema/anyOf/0|#|type|level:number",
		},
	}, report)
}

func marshalFaultTestValue(t *testing.T, value *jsonValue) []byte {
	t.Helper()

	encoded, err := marshalStrict(value)
	require.NoError(t, err)

	return encoded
}
