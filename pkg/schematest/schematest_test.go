package schematest_test

import (
	"os"
	"sort"
	"testing"

	"github.com/djosh34/klopt/pkg/schematest" //nolint:depguard // The external contract test must cross the package seam.
	"github.com/djosh34/klopt/pkg/validation"

	"github.com/stretchr/testify/require"
)

var _ func(schematest.Input, func(schematest.Case) error) (schematest.Report, error) = schematest.Build

// corpus is the checked-in document and JSON request-body operation metadata.
var corpus = []struct {
	path       string
	operations []string
}{
	{
		path:       "testdata/alpha_zeta.yaml",
		operations: []string{"alphaRequest", "zetaRequest"},
	},
	{
		path: "testdata/request_bodies.yaml",
		// bodylessRequest and plainRequest are deliberately outside the JSON request-body corpus.
		operations: []string{"referencedRequest"},
	},
	{
		path: "../../resources/openapi.yaml",
		operations: []string{
			"allOfObject",
			"anyOfBodyAndParameters",
			"arrayNotNullable",
			"arrayNullable",
			"compositeObject",
			"nullableObjectKeysAdditionalPropertiesFalse",
			"objectKeysAdditionalPropertiesFalse",
			"optionalArrayNullable",
			"refObject",
			"refStressObject",
			"refStressObjectPut",
			"stringNoFormatNotNullable",
			"stringNoFormatNullable",
		},
	},
}

// TestCorpusDocumentsAreAdmittedWithExpectedJSONRequestBodies verifies corpus admission and metadata.
func TestCorpusDocumentsAreAdmittedWithExpectedJSONRequestBodies(t *testing.T) {
	t.Parallel()

	for _, fixture := range corpus {
		t.Run(fixture.path, func(t *testing.T) {
			t.Parallel()

			document, err := os.ReadFile(fixture.path)
			require.NoError(t, err)

			requests, err := validation.Parse(document, validation.PatternOptions())
			require.NoError(t, err)

			operations := make([]string, 0, len(requests))
			for operationID, request := range requests {
				if request.Body != nil {
					operations = append(operations, operationID)
				}
			}

			sort.Strings(operations)

			require.Equal(t, fixture.operations, operations)
		})
	}
}

// TestStopReasonValuesMatchContract verifies the locked normal-stop wire values.
func TestStopReasonValuesMatchContract(t *testing.T) {
	t.Parallel()

	_ = schematest.Input{OpenAPI: nil, OperationID: "", MaxSteps: 0}
	_ = schematest.Case{JSON: nil, Valid: false}
	_ = schematest.Report{Stop: "", Steps: 0, Covered: nil, Uncovered: nil}

	require.Equal(t, schematest.StopReason("space_exhausted"), schematest.SpaceExhausted)
	require.Equal(t, schematest.StopReason("max_steps_reached"), schematest.MaxStepsReached)
}

// TestBuildScaffoldReturnsAnErrorWithoutEmitting verifies the temporary result is never authoritative.
func TestBuildScaffoldReturnsAnErrorWithoutEmitting(t *testing.T) {
	t.Parallel()

	emitted := false
	report, err := schematest.Build(
		schematest.Input{OpenAPI: []byte("openapi: 3.0.3"), OperationID: "request", MaxSteps: 1},
		func(_ schematest.Case) error {
			emitted = true

			return nil
		},
	)

	require.Error(t, err)
	require.False(t, emitted)
	require.Zero(t, report)
}
