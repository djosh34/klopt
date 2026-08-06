package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildRejectsInputBeforeZeroBudget verifies admission precedes execution control.
func TestBuildRejectsInputBeforeZeroBudget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		document string
		want     string
	}{
		{
			name:     "malformed",
			document: "{",
			want:     "parse OpenAPI document",
		},
		{
			name:     "profile excluded",
			document: documentWithJSONSchema(`{"oneOf":[{}]}`),
			want:     "/oneOf",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			emitted := 0
			report, err := Build(
				Input{OpenAPI: []byte(test.document), OperationID: "selected", MaxSteps: 0},
				func(Case) error {
					emitted++

					return nil
				},
			)

			require.Error(t, err)
			require.Contains(t, err.Error(), test.want)
			require.Zero(t, report)
			require.Zero(t, emitted)
		})
	}
}

// TestBuildStopsBeforeScalarAssignment verifies a cutoff cannot emit a partial scalar row.
func TestBuildStopsBeforeScalarAssignment(t *testing.T) {
	t.Parallel()

	emitted := 0
	report, err := Build(
		Input{
			OpenAPI:     []byte(documentWithJSONSchema(`{"type":"string","enum":["a"]}`)),
			OperationID: "selected",
			MaxSteps:    1,
		},
		func(Case) error {
			emitted++

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, MaxStepsReached, report.Stop)
	require.Equal(t, uint64(1), report.Steps)
	require.Zero(t, emitted)
}

// TestBuildStopsBeforeStructuralAssignment verifies a cutoff cannot emit a partial object.
func TestBuildStopsBeforeStructuralAssignment(t *testing.T) {
	t.Parallel()

	emitted := 0
	report, err := Build(
		Input{
			OpenAPI: []byte(documentWithJSONSchema(`{
				"type":"object",
				"required":["name"],
				"properties":{"name":{"type":"string"}}
			}`)),
			OperationID: "selected",
			MaxSteps:    3,
		},
		func(Case) error {
			emitted++

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, MaxStepsReached, report.Stop)
	require.Equal(t, uint64(3), report.Steps)
	require.Zero(t, emitted)
}

// TestBuildChargesNestedCompositionsBeforeAssignment verifies nested cutoff charging.
func TestBuildChargesNestedCompositionsBeforeAssignment(t *testing.T) {
	t.Parallel()

	cases := make([]Case, 0)
	report, err := Build(
		Input{
			OpenAPI:     []byte(documentWithJSONSchema(`{"allOf":[{"allOf":[{}]}]}`)),
			OperationID: "selected",
			MaxSteps:    2,
		},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, MaxStepsReached, report.Stop)
	require.Equal(t, uint64(2), report.Steps)
	require.Empty(t, cases)

	cases = cases[:0]
	report, err = Build(
		Input{
			OpenAPI:     []byte(documentWithJSONSchema(`{"allOf":[{"allOf":[{}]}]}`)),
			OperationID: "selected",
			MaxSteps:    3,
		},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, MaxStepsReached, report.Stop)
	require.Equal(t, uint64(3), report.Steps)
	require.Len(t, cases, 1)
	require.Contains(t, report.Covered, "#/paths/~1/post/requestBody/content/application~1json/schema/allOf/0|#|allOf|level:all-true")
}

// TestBuildUsesOneCounterAcrossTargets verifies retries do not reset the budget.
func TestBuildUsesOneCounterAcrossTargets(t *testing.T) {
	t.Parallel()

	cases := make([]Case, 0)
	report, err := Build(
		Input{
			OpenAPI:     []byte(documentWithJSONSchema(`{"type":"string","enum":["a","b"]}`)),
			OperationID: "selected",
			MaxSteps:    6,
		},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, MaxStepsReached, report.Stop)
	require.Equal(t, uint64(6), report.Steps)
	require.Equal(t, []Case{
		{JSON: []byte(`"a"`), Valid: true},
		{JSON: []byte(`"a"`), Valid: true},
	}, cases)
}
