package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRowArrayLengthCursorUsesExactBoundaryOrder pins the lazy cursor's first-occurrence order.
func TestRowArrayLengthCursorUsesExactBoundaryOrder(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array",
		"items":{},
		"minItems":3,
		"maxItems":5
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	requirements := []requirement{{
		tag:        requirementExactCount,
		occurrence: model.root.occurrence,
		count:      model.root.maxItems,
	}}
	view := rowProjectionView{sources: []rowSchemaSource{{
		node:       model.root,
		occurrence: model.root.occurrence,
	}}}

	cursor, err := newRowArrayLengthCursor(view, requirements)
	require.NoError(t, err)

	var lengths []uint64

	for {
		length, ok := cursor.Next()
		if !ok {
			break
		}

		lengths = append(lengths, length)
	}

	require.Equal(t, []uint64{5, 3, 0, 1, 2, 4}, lengths)
}

// TestRowArrayOpenLengthCursorHasNoLocalEndpoint proves only the global budget ends an open domain.
func TestRowArrayOpenLengthCursorHasNoLocalEndpoint(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array",
		"items":{},
		"minItems":2
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	view := rowProjectionView{sources: []rowSchemaSource{{
		node:       model.root,
		occurrence: model.root.occurrence,
	}}}
	cursor, err := newRowArrayLengthCursor(view, nil)
	require.NoError(t, err)

	lengths := make([]uint64, 0, 8)

	for range 8 {
		length, ok := cursor.Next()
		require.True(t, ok)

		lengths = append(lengths, length)
	}

	require.Equal(t, []uint64{2, 0, 1, 3, 4, 5, 6, 7}, lengths)
}

// TestWalkArrayChargesBeforeHugeMinimumAllocation proves authored bounds do not allocate containers.
func TestWalkArrayChargesBeforeHugeMinimumAllocation(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array",
		"items":{},
		"minItems":1000000000
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1}
	emitted := 0
	_, err = searchState.walkArray(
		model.root,
		model.root.occurrence,
		nil,
		rowSearchContext{},
		func(*jsonValue) (bool, error) {
			emitted++

			return false, nil
		},
	)

	require.ErrorIs(t, err, errMaxSteps)
	require.Equal(t, uint64(1), searchState.steps)
	require.Zero(t, emitted)
}

// TestWalkArrayPositionsOwnIndependentValues proves item occurrences never alias authored values.
func TestWalkArrayPositionsOwnIndependentValues(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array",
		"items":{"enum":[{"x":true}]},
		"minItems":2,
		"maxItems":2
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 10000}
	itemOccurrence := rebasePlanOccurrence(
		model.root.items,
		model.root.occurrence,
		model.root.occurrence.usePointer+"/items",
		appendInstanceToken(model.root.occurrence.instanceTemplate, "*"),
	)
	requirements := []requirement{kindRequirement(itemOccurrence, jsonObject)}

	var row *jsonValue

	complete, err := searchState.walkArray(
		model.root,
		model.root.occurrence,
		requirements,
		rowSearchContext{},
		func(value *jsonValue) (bool, error) {
			row = value

			return true, nil
		},
	)

	require.NoError(t, err)
	require.True(t, complete)
	require.Len(t, row.array, 2)
	require.NotSame(t, row.array[0], row.array[1])
	require.NotSame(t, row.array[0].object["x"], row.array[1].object["x"])
}
