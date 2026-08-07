package schematest

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildCallbackBytesRequireCallerCopyAndAreNotRetained proves emitted bytes leave execution state at return.
func TestBuildCallbackBytesRequireCallerCopyAndAreNotRetained(t *testing.T) {
	t.Parallel()

	input := Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"type":"boolean"}`)),
		OperationID: "selected",
		MaxSteps:    1_000,
	}

	wantCases, wantReport, err := collectDeterministicRun(input, nil)
	require.NoError(t, err)
	require.Greater(t, len(wantCases), 1)

	var retainedCopies []Case

	report, err := Build(input, func(testCase Case) error {
		retainedCopies = append(retainedCopies, Case{
			JSON:  bytes.Clone(testCase.JSON),
			Valid: testCase.Valid,
		})

		// Callback bytes may be overwritten or reused immediately after return.
		// Mutating them while they are valid proves later search and reporting do
		// not consult prior callback storage; only caller-owned copies are read later.
		for index := range testCase.JSON {
			testCase.JSON[index] = 0xff
		}

		return nil
	})
	require.NoError(t, err)
	require.Equal(t, wantReport, report)
	require.Equal(t, wantCases, retainedCopies)
}
