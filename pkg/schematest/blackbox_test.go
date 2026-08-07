package schematest_test

import (
	"os"
	"testing"

	"github.com/djosh34/klopt/pkg/schematest" //nolint:depguard // This external harness exercises the public Build seam.
	"github.com/djosh34/klopt/pkg/validation"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/stretchr/testify/require"
)

// TestCorpusRuntimeVerdictsMatchBuild validates every streamed corpus case through production Parse.
func TestCorpusRuntimeVerdictsMatchBuild(t *testing.T) {
	t.Parallel()

	for _, fixture := range corpus {
		t.Run(fixture.path, func(t *testing.T) {
			t.Parallel()

			document, err := os.ReadFile(fixture.path)
			require.NoError(t, err)

			requests, err := validation.Parse(document, validation.PatternOptions())
			require.NoError(t, err)

			for _, operationID := range fixture.operations {
				request, exists := requests[operationID]
				require.True(t, exists, operationID)
				require.NotNil(t, request.Body, operationID)

				emitted := 0
				report, buildErr := schematest.Build(
					schematest.Input{OpenAPI: document, OperationID: operationID, MaxSteps: 10_000},
					func(testCase schematest.Case) error {
						require.True(t, jsontext.Value(testCase.JSON).IsValid(), operationID)
						require.Equal(t, testCase.Valid, len(request.Body.Validate(testCase.JSON)) == 0, operationID)

						emitted++

						return nil
					},
				)

				require.NoError(t, buildErr, operationID)
				require.Contains(
					t,
					[]schematest.StopReason{schematest.SpaceExhausted, schematest.MaxStepsReached},
					report.Stop,
					operationID,
				)
				require.Positive(t, emitted, operationID)
			}
		})
	}
}
