package schematest_test

import (
	"os"
	"testing"

	"github.com/djosh34/klopt/pkg/schematest" //nolint:depguard // This external harness exercises the public Build seam.
	"github.com/djosh34/klopt/pkg/validation"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/stretchr/testify/require"
)

// closedObjectObservations records the required closed-object callback cases.
type closedObjectObservations [8]bool

// isClosedObjectOperation reports whether the operation has pinned closed-object cases.
func isClosedObjectOperation(operationID string) bool {
	return operationID == "nullableObjectKeysAdditionalPropertiesFalse" ||
		operationID == "objectKeysAdditionalPropertiesFalse"
}

// record marks the matching JSON and Case.Valid observation without retaining callback bytes.
func (observed *closedObjectObservations) record(testCase schematest.Case) {
	expected := [8]struct {
		body  string
		valid bool
	}{
		{body: `{"requiredNotNullableString":"","requiredNullableString":""}`, valid: true},
		{body: `null`, valid: true},
		{body: `{"requiredNullableString":""}`},
		{body: `{"requiredNotNullableString":""}`},
		{body: `{"optionalNotNullableString":null,"requiredNotNullableString":"","requiredNullableString":""}`},
		{body: `{"optionalNullableString":false,"requiredNotNullableString":"","requiredNullableString":""}`},
		{body: `{"requiredNotNullableString":null,"requiredNullableString":""}`},
		{body: `{"requiredNotNullableString":"","requiredNullableString":false}`},
	}

	body := string(testCase.JSON)
	for index, expectation := range expected {
		observed[index] = observed[index] ||
			testCase.Valid == expectation.valid && body == expectation.body
	}
}

// requireClosedObjectObservations asserts every callback case required for one closed-object operation.
func requireClosedObjectObservations(
	t *testing.T,
	operationID string,
	report schematest.Report,
	observed closedObjectObservations,
) {
	t.Helper()

	require.Equal(t, schematest.SpaceExhausted, report.Stop, operationID)
	require.True(t, observed[0], operationID+": valid closed object")

	if operationID == "nullableObjectKeysAdditionalPropertiesFalse" {
		require.True(t, observed[1], operationID+": nullable null")
	}

	for index := 2; index < len(observed); index++ {
		require.True(t, observed[index], operationID+": isolated fault")
	}
}

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
				observed := closedObjectObservations{}
				anyOfAZObserved := false
				report, buildErr := schematest.Build(
					schematest.Input{OpenAPI: document, OperationID: operationID, MaxSteps: 10_000},
					func(testCase schematest.Case) error {
						require.True(t, jsontext.Value(testCase.JSON).IsValid(), operationID)
						require.Equal(t, testCase.Valid, len(request.Body.Validate(testCase.JSON)) == 0, operationID)

						if isClosedObjectOperation(operationID) {
							observed.record(testCase)
						}

						if operationID == "anyOfBodyAndParameters" {
							anyOfAZObserved = anyOfAZObserved || testCase.Valid && string(testCase.JSON) == `"az"`
						}

						emitted++

						return nil
					},
				)

				require.NoError(t, buildErr, operationID)

				if isClosedObjectOperation(operationID) {
					requireClosedObjectObservations(t, operationID, report, observed)
				} else {
					require.Contains(
						t,
						[]schematest.StopReason{schematest.SpaceExhausted, schematest.MaxStepsReached},
						report.Stop,
						operationID,
					)
				}

				if operationID == "anyOfBodyAndParameters" {
					require.True(t, anyOfAZObserved, operationID+`: valid "az"`)
				}

				require.Positive(t, emitted, operationID)
			}
		})
	}
}
