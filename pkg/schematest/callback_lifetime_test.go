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

	var (
		retainedCopies  []Case
		callbackAliases [][]byte
	)

	report, err := Build(input, func(testCase Case) error {
		retainedCopies = append(retainedCopies, Case{
			JSON:  bytes.Clone(testCase.JSON),
			Valid: testCase.Valid,
		})
		callbackAliases = append(callbackAliases, testCase.JSON)

		// Simulate callback-lifetime storage becoming unusable immediately on return.
		// Later search, fault generation, and reporting must not consult these bytes.
		for index := range testCase.JSON {
			testCase.JSON[index] = 0xff
		}

		return nil
	})
	require.NoError(t, err)
	require.Equal(t, wantReport, report)
	require.Equal(t, wantCases, retainedCopies)

	for _, alias := range callbackAliases {
		require.NotEmpty(t, alias)

		for _, value := range alias {
			require.Equal(t, byte(0xff), value, "uncopied callback bytes are not retained case storage")
		}
	}
}
