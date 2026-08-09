//nolint:godoclint // Focused private fault-stream tests use behavior names.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildFaultProductContinuesPastUnsuitableRequiredParent(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"minProperties":2,
		"required":["id"],
		"properties":{"extra":{"type":"string"},"id":{"type":"string"},"spare":{"type":"string"}},
		"additionalProperties":false
	}`))

	var cases []Case

	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100_000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)
	require.NoError(t, err)
	require.Contains(t, report.Covered,
		"#/paths/~1/post/requestBody/content/application~1json/schema|#/id|required|fault:required")
	require.Contains(t, cases, Case{JSON: []byte(`{"extra":"","spare":""}`), Valid: false})
}

func TestBuildNonCompositionFaultsAreDeterministicAndCutOffAtomically(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{"type":"number","minimum":1,"maximum":2}`))
	collect := func(maxSteps uint64) ([]Case, Report) {
		var cases []Case

		report, err := Build(
			Input{OpenAPI: document, OperationID: "selected", MaxSteps: maxSteps},
			func(testCase Case) error {
				cases = append(cases, testCase)

				return nil
			},
		)
		require.NoError(t, err)

		return cases, report
	}

	firstCases, firstReport := collect(10_000)
	secondCases, secondReport := collect(10_000)

	require.Equal(t, SpaceExhausted, firstReport.Stop)
	require.Equal(t, firstCases, secondCases)
	require.Equal(t, firstReport, secondReport)
	require.NotEmpty(t, firstCases)
	require.False(t, firstCases[len(firstCases)-1].Valid)

	cutoffCases, cutoffReport := collect(firstReport.Steps - 1)
	require.Equal(t, MaxStepsReached, cutoffReport.Stop)
	require.Equal(t, firstReport.Steps-1, cutoffReport.Steps)
	require.Equal(t, firstCases[:len(firstCases)-1], cutoffCases)
}
