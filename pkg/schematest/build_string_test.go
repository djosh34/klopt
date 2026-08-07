//nolint:godoclint // Tests pin the public string-search schedule.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildStreamsDirectedStringObjectivesInLockedOrder(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"minLength":1,
		"maxLength":1,
		"pattern":"^[a-b]+$",
		"allOf":[{"pattern":"^[b-c]+$"}]
	}`))

	firstCases, firstReport := buildStringCases(t, document, 1_000)
	secondCases, secondReport := buildStringCases(t, document, 1_000)

	schemaPointer := "#/paths/~1/post/requestBody/content/application~1json/schema"

	require.Equal(t, []Case{
		{JSON: []byte(`"b"`), Valid: true},
		{JSON: []byte(`"b"`), Valid: true},
		{JSON: []byte(`"b"`), Valid: true},
		{JSON: []byte(`"b"`), Valid: true},
		{JSON: []byte(`"b"`), Valid: true},
		{JSON: []byte(`"b"`), Valid: true},
		{JSON: []byte(`"b"`), Valid: true},
		{JSON: []byte(`"c"`), Valid: false},
		{JSON: []byte(`"a"`), Valid: false},
		{JSON: []byte(`"bb"`), Valid: false},
	}, firstCases)
	require.Equal(t, Report{
		Stop:  SpaceExhausted,
		Steps: 196,
		Covered: []string{
			schemaPointer + "|#|type|level:string",
			schemaPointer + "|#|minLength|level:valid",
			schemaPointer + "|#|maxLength|level:valid",
			schemaPointer + "|#|maxLength|fault:maxLength",
			schemaPointer + "|#|pattern|level:valid",
			schemaPointer + "|#|pattern|fault:pattern",
			schemaPointer + "|#|allOf|level:all-true",
			schemaPointer + "/allOf/0|#|type|level:string",
			schemaPointer + "/allOf/0|#|pattern|level:valid",
			schemaPointer + "/allOf/0|#|pattern|fault:pattern",
		},
		Uncovered: []string{
			schemaPointer + "|#|type|fault:type",
			schemaPointer + "|#|minLength|fault:minLength",
			schemaPointer + "/allOf/0|#|type|level:null",
			schemaPointer + "/allOf/0|#|type|level:boolean",
			schemaPointer + "/allOf/0|#|type|level:number",
			schemaPointer + "/allOf/0|#|type|level:array",
			schemaPointer + "/allOf/0|#|type|level:object",
		},
	}, firstReport)
	require.Equal(t, firstCases, secondCases)
	require.Equal(t, firstReport, secondReport)
}

func TestBuildStreamsFormatObjectiveBeforeLengthObjectives(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"minLength":7,
		"maxLength":15,
		"format":"ipv4"
	}`))
	cases, report := buildStringCases(t, document, 1_000)

	schemaPointer := "#/paths/~1/post/requestBody/content/application~1json/schema"

	require.Equal(t, []Case{
		{JSON: []byte(`"0.0.0.0"`), Valid: true},
		{JSON: []byte(`"0.0.0.0"`), Valid: true},
		{JSON: []byte(`"0.0.0.0"`), Valid: true},
		{JSON: []byte(`"0.0.0.0"`), Valid: true},
		{JSON: []byte(`"00.0.0.0"`), Valid: false},
	}, cases)
	require.Equal(t, Report{
		Stop:  MaxStepsReached,
		Steps: 1_000,
		Covered: []string{
			schemaPointer + "|#|type|level:string",
			schemaPointer + "|#|minLength|level:valid",
			schemaPointer + "|#|maxLength|level:valid",
			schemaPointer + "|#|format|level:valid",
			schemaPointer + "|#|format|fault:format",
		},
		Uncovered: []string{
			schemaPointer + "|#|type|fault:type",
			schemaPointer + "|#|minLength|fault:minLength",
			schemaPointer + "|#|maxLength|fault:maxLength",
		},
	}, report)
}

func buildStringCases(t *testing.T, document []byte, maxSteps uint64) ([]Case, Report) {
	t.Helper()

	cases := make([]Case, 0)
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
