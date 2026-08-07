//nolint:godoclint // Tests pin the public string-search schedule.
package schematest

import (
	"strings"
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
		Steps: 28,
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
			want:   `"1000-01-01T00:00:00.000Z"`,
		},
		{name: "email 4", schema: `{"type":"string","format":"email","minLength":4,"maxLength":4}`, want: `"a@aa"`},
		{name: "ipv4 8", schema: `{"type":"string","format":"ipv4","minLength":8,"maxLength":8}`, want: `"0.0.0.10"`},
		{name: "ipv4 9", schema: `{"type":"string","format":"ipv4","minLength":9,"maxLength":9}`, want: `"0.0.0.100"`},
		{name: "base64 minimum 5", schema: `{"type":"string","format":"byte","minLength":5}`, want: `"++++++++"`},
		{name: "date pattern", schema: `{"type":"string","format":"date","pattern":"^2025"}`, want: `"2025-01-01"`},
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

func TestBuildSearchesIncrementalFormatState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		check  func(*testing.T, string)
	}{
		{
			name:   "cidr exact length 10",
			schema: `{"type":"string","format":"cidr","minLength":10,"maxLength":10}`,
			check: func(t *testing.T, value string) {
				require.Len(t, value, 10)
			},
		},
		{
			name:   "email exact length 67",
			schema: `{"type":"string","format":"email","minLength":67,"maxLength":67}`,
			check: func(t *testing.T, value string) {
				require.Len(t, value, 67)
			},
		},
		{
			name:   "email exact length 100",
			schema: `{"type":"string","format":"email","minLength":100,"maxLength":100}`,
			check: func(t *testing.T, value string) {
				require.Len(t, value, 100)
			},
		},
		{
			name:   "date pattern",
			schema: `{"type":"string","format":"date","pattern":"^2025"}`,
			check: func(t *testing.T, value string) {
				require.Equal(t, "2025-01-01", value)
			},
		},
		{
			name:   "byte pattern",
			schema: `{"type":"string","format":"byte","pattern":"^YWI=$"}`,
			check: func(t *testing.T, value string) {
				require.Equal(t, "YWI=", value)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cases, _ := buildStringCases(t, []byte(documentWithJSONSchema(test.schema)), 1_000_000)
			require.NotEmpty(t, cases)
			value, err := parseStrictJSON(cases[0].JSON)
			require.NoError(t, err)
			test.check(t, value.text)
		})
	}
}

func TestBuildChargesUnpinnedAnyOfDFSBeforeRetainingSiblingPaths(t *testing.T) {
	t.Parallel()

	const count = 25

	children := make([]string, count)
	for index := range children {
		children[index] = `{"anyOf":[{"pattern":"^a$"},{"pattern":"^b$"}]}`
	}

	schema := `{"type":"string","allOf":[` + strings.Join(children, ",") + `]}`

	cases, report := buildStringCases(t, []byte(documentWithJSONSchema(schema)), 10)
	for _, testCase := range cases {
		require.True(t, testCase.Valid)
	}

	require.Equal(t, MaxStepsReached, report.Stop)
	require.Equal(t, uint64(10), report.Steps)
}

func TestBuildTriesAuthoredStringEnumBeforeUnboundedProduct(t *testing.T) {
	t.Parallel()

	cases, report := buildStringCases(t, []byte(documentWithJSONSchema(`{
		"type":"string",
		"enum":["z"],
		"pattern":"^.*$"
	}`)), 20)
	require.Contains(t, cases, Case{JSON: []byte(`"z"`), Valid: true})
	require.Less(t, report.Steps, uint64(20))

	rejected, rejectedReport := buildStringCases(t, []byte(documentWithJSONSchema(`{
		"type":"string",
		"enum":["z"],
		"allOf":[{"pattern":"^a$"}]
	}`)), 100)
	require.Empty(t, rejected)
	require.Equal(t, SpaceExhausted, rejectedReport.Stop)
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
