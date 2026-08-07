//nolint:godoclint // Tests pin the public string-search schedule.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildStreamsValidStringTargetsInLockedOrder(t *testing.T) {
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
	}, firstCases)
	require.Equal(t, Report{
		Stop:  SpaceExhausted,
		Steps: 95,
		Covered: []string{
			schemaPointer + "|#|type|level:string",
			schemaPointer + "|#|minLength|level:valid",
			schemaPointer + "|#|maxLength|level:valid",
			schemaPointer + "|#|pattern|level:valid",
			schemaPointer + "|#|allOf|level:all-true",
			schemaPointer + "/allOf/0|#|type|level:string",
			schemaPointer + "/allOf/0|#|pattern|level:valid",
		},
		Uncovered: []string{
			schemaPointer + "|#|type|fault:type",
			schemaPointer + "|#|minLength|fault:minLength",
			schemaPointer + "|#|maxLength|fault:maxLength",
			schemaPointer + "|#|pattern|fault:pattern",
			schemaPointer + "/allOf/0|#|type|level:null",
			schemaPointer + "/allOf/0|#|type|level:boolean",
			schemaPointer + "/allOf/0|#|type|level:number",
			schemaPointer + "/allOf/0|#|type|level:array",
			schemaPointer + "/allOf/0|#|type|level:object",
			schemaPointer + "/allOf/0|#|pattern|fault:pattern",
		},
	}, firstReport)
	require.Equal(t, firstCases, secondCases)
	require.Equal(t, firstReport, secondReport)
}

func TestBuildStreamsValidFormatTargets(t *testing.T) {
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
	}, cases)
	require.Equal(t, Report{
		Stop:  SpaceExhausted,
		Steps: 36,
		Covered: []string{
			schemaPointer + "|#|type|level:string",
			schemaPointer + "|#|minLength|level:valid",
			schemaPointer + "|#|maxLength|level:valid",
			schemaPointer + "|#|format|level:valid",
		},
		Uncovered: []string{
			schemaPointer + "|#|type|fault:type",
			schemaPointer + "|#|minLength|fault:minLength",
			schemaPointer + "|#|maxLength|fault:maxLength",
			schemaPointer + "|#|format|fault:format",
		},
	}, report)
}

func TestBuildSearchesFormatsAtActiveLengths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		want   string
	}{
		{
			name:   "date-time 24",
			schema: `{"type":"string","format":"date-time","minLength":24,"maxLength":24}`,
			want:   `"1970-01-01T00:00:00.000Z"`,
		},
		{name: "email 4", schema: `{"type":"string","format":"email","minLength":4,"maxLength":4}`, want: `"a@bb"`},
		{name: "ipv4 8", schema: `{"type":"string","format":"ipv4","minLength":8,"maxLength":8}`, want: `"10.0.0.0"`},
		{name: "ipv4 9", schema: `{"type":"string","format":"ipv4","minLength":9,"maxLength":9}`, want: `"100.0.0.0"`},
		{name: "base64 minimum 5", schema: `{"type":"string","format":"byte","minLength":5}`, want: `"AAAAAA=="`},
		{name: "date pattern", schema: `{"type":"string","format":"date","pattern":"^2024"}`, want: `"2024-01-01"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cases, _ := buildStringCases(
				t, []byte(documentWithJSONSchema(test.schema)), 100_000,
			)
			require.Contains(t, cases, Case{JSON: []byte(test.want), Valid: true})
		})
	}
}

func TestBuildEnumeratesUnpinnedAnyOfStringRules(t *testing.T) {
	t.Parallel()

	cases, _ := buildStringCases(t, []byte(documentWithJSONSchema(`{
		"type":"string",
		"anyOf":[
			{"pattern":"^a$"},
			{"pattern":"^b$"}
		]
	}`)), 1_000)

	require.Contains(t, cases, Case{JSON: []byte(`"a"`), Valid: true})
	require.Contains(t, cases, Case{JSON: []byte(`"b"`), Valid: true})
}

func TestBuildDoesNotStreamDirectedStringFaults(t *testing.T) {
	t.Parallel()

	cases, report := buildStringCases(t, []byte(documentWithJSONSchema(`{
		"type":"string",
		"pattern":"^a$"
	}`)), 1_000)

	for _, testCase := range cases {
		require.True(t, testCase.Valid)
	}

	for _, covered := range report.Covered {
		require.NotContains(t, covered, "|fault:")
	}
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
