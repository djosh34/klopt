package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWalkArrayStreamsNestedAnyOfItems proves open counts do not starve later projections.
func TestWalkArrayStreamsNestedAnyOfItems(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array",
		"minItems":1,
		"items":{},
		"allOf":[{"anyOf":[
			{"items":{"enum":["z"]}},
			{"items":{"enum":["q"]}}
		]}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 100}

	var rows []string

	_, err = searchState.walkArray(
		model.root,
		model.root.occurrence,
		nil,
		rowSearchContext{},
		func(value *jsonValue) (bool, error) {
			encoded, marshalErr := marshalStrict(value)
			if marshalErr != nil {
				return false, marshalErr
			}

			rows = append(rows, string(encoded))

			return len(rows) == 2, nil
		},
	)

	require.NoError(t, err)
	require.Contains(t, rows, `["z"]`)
	require.Contains(t, rows, `["q"]`)
}
