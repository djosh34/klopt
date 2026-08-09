package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildUsesComposedObjectDefaultAsCompleteCandidate preserves authored object witnesses.
func TestBuildUsesComposedObjectDefaultAsCompleteCandidate(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"allOf":[
			{"default":{"x":"z"}},
			{"required":["x"],"properties":{"x":{"enum":["z"]}}}
		]
	}`))

	var cases []Case

	_, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Contains(t, cases, Case{JSON: []byte(`{"x":"z"}`), Valid: true})
}

// TestBuildRepairsMinPropertiesWithoutFalseBranchNames proves inactive names do not fill the lower bound.
func TestBuildRepairsMinPropertiesWithoutFalseBranchNames(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"minProperties":1,
		"anyOf":[
			{"type":"object","required":["x"],"properties":{"x":{"type":"boolean"}}},
			{"type":"object","maxProperties":1}
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

	schemaPointer := "#/paths/~1/post/requestBody/content/application~1json/schema"
	require.Contains(t, report.Covered, schemaPointer+"|#|anyOf|level:mask:2")
	require.Contains(t, cases, Case{JSON: []byte(`{"__schematest_extra__":null}`), Valid: true})
}

// TestProjectionStreamsNestedAnyOfObjectMinima proves each unpinned lower bound stays reachable.
func TestProjectionStreamsNestedAnyOfObjectMinima(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"allOf":[{"anyOf":[
			{"type":"object","minProperties":2},
			{"type":"object","minProperties":3}
		]}]
	}`))
	model, err := parseInput(Input{OpenAPI: document, OperationID: "selected"})
	require.NoError(t, err)

	cursor := newRowProjectionCursor(model.root, model.root.occurrence, nil)
	defer cursor.Close()

	var minima []uint64

	for {
		view, ok, cursorErr := cursor.Next()
		require.NoError(t, cursorErr)

		if !ok {
			break
		}

		shape, shapeErr := newRowProjectedObject(view, nil, model.root.occurrence)
		require.NoError(t, shapeErr)

		minima = append(minima, shape.minimum)
	}

	require.Equal(t, []uint64{2, 3, 3}, minima)
}

// TestProjectedObjectRejectsConflictingBoundsBeforeConstruction proves no over-max partial is needed.
func TestProjectedObjectRejectsConflictingBoundsBeforeConstruction(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object","minProperties":2,"allOf":[{"maxProperties":1}]
	}`))
	model, err := parseInput(Input{OpenAPI: document, OperationID: "selected"})
	require.NoError(t, err)

	cursor := newRowProjectionCursor(model.root, model.root.occurrence, nil)
	defer cursor.Close()

	view, ok, err := cursor.Next()
	require.NoError(t, err)
	require.True(t, ok)

	shape, err := newRowProjectedObject(view, nil, model.root.occurrence)
	require.NoError(t, err)
	require.False(t, shape.feasible())
}

// TestBuildCutsOffHugeMinPropertiesBeforeSyntheticAllocation locks incremental repair.
func TestBuildCutsOffHugeMinPropertiesBeforeSyntheticAllocation(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{"type":"object","minProperties":1000000000}`))
	callbacks := 0
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 3},
		func(Case) error {
			callbacks++

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, MaxStepsReached, report.Stop)
	require.Equal(t, uint64(3), report.Steps)
	require.Zero(t, callbacks)
}
