package schematest

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildStreamsDeterministicValidPrimitiveRows verifies the public valid-row stream.
func TestBuildStreamsDeterministicValidPrimitiveRows(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{"type":"string","enum":["a","b"]}`))

	collect := func() (Report, []Case, error) {
		cases := make([]Case, 0)
		report, err := Build(
			Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100},
			func(testCase Case) error {
				cases = append(cases, testCase)

				return nil
			},
		)

		return report, cases, err
	}

	firstReport, firstCases, err := collect()
	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, firstReport.Stop)
	require.NotEmpty(t, firstCases)

	for _, testCase := range firstCases {
		require.True(t, testCase.Valid)
		require.Contains(t, []string{`"a"`, `"b"`}, string(testCase.JSON))
	}

	secondReport, secondCases, err := collect()
	require.NoError(t, err)
	require.Equal(t, firstReport, secondReport)
	require.Equal(t, firstCases, secondCases)
}

// TestBuildEmitsOracleValidUUIDWitnesses verifies UUID format coverage through Build.
func TestBuildEmitsOracleValidUUIDWitnesses(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{"type":"string","format":"uuid"}`))
	cases := make([]Case, 0)

	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)
	require.NoError(t, err)
	require.NotEmpty(t, cases)

	require.Contains(t, cases, Case{
		JSON:  []byte(`"00000000-0000-4000-8000-000000000000"`),
		Valid: true,
	})

	for _, testCase := range cases {
		require.True(t, testCase.Valid)
	}

	const schemaPointer = "#/paths/~1/post/requestBody/content/application~1json/schema"

	require.Equal(t, []string{
		schemaPointer + "|#|type|level:string",
		schemaPointer + "|#|format|level:valid",
	}, report.Covered)
}

// TestBuildAdmitsBeforeZeroBudgetAndEmitsNothing verifies the zero-step stop.
func TestBuildAdmitsBeforeZeroBudgetAndEmitsNothing(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{"type":"boolean"}`))

	emitted := false
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 0},
		func(Case) error {
			emitted = true

			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, MaxStepsReached, report.Stop)
	require.Zero(t, report.Steps)
	require.False(t, emitted)
}

// TestBuildRejectsNilCallback verifies callback misuse is reported.
func TestBuildRejectsNilCallback(t *testing.T) {
	t.Parallel()

	_, err := Build(Input{}, nil)
	require.Error(t, err)
}

// TestBuildReturnsCallbackErrors verifies consumer failures stop the stream.
func TestBuildReturnsCallbackErrors(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{"type":"boolean"}`))
	callbackErr := errors.New("consumer stopped")
	emitted := 0

	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100},
		func(Case) error {
			emitted++

			return callbackErr
		},
	)
	require.ErrorIs(t, err, callbackErr)
	require.Zero(t, report)
	require.Equal(t, 1, emitted)
}
