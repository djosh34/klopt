package schematest

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWalkProjectedDirectArraysUsesComposedDefaultContents preserves complete authored witnesses.
func TestWalkProjectedDirectArraysUsesComposedDefaultContents(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[
			{"default":[true,true,true]},
			{"maxItems":2}
		]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	requirements := anyOfMaskRequirements(model.root.occurrence, 2, big.NewInt(1))
	searchState := &search{model: model, maxSteps: 100}

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
	require.Equal(t, uint64(1), searchState.steps)
	require.Len(t, row.array, 3)
}
