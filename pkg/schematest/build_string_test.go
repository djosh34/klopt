//nolint:godoclint // Tests requirement the public string-search schedule.
package schematest

import (
	"errors"
	"strconv"
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
		{JSON: []byte(`null`), Valid: false},
		{JSON: []byte(`"c"`), Valid: false},
		{JSON: []byte(`"a"`), Valid: false},
		{JSON: []byte(`"bb"`), Valid: false},
	}, firstCases)
	require.Equal(t, Report{
		Stop:  SpaceExhausted,
		Steps: 95,
		Covered: []string{
			schemaPointer + "|#|type|level:string",
			schemaPointer + "|#|type|fault:type",
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
			schemaPointer + "|#|minLength|fault:minLength",
			schemaPointer + "/allOf/0|#|type|level:boolean",
			schemaPointer + "/allOf/0|#|type|level:null",
			schemaPointer + "/allOf/0|#|type|level:number",
			schemaPointer + "/allOf/0|#|type|level:array",
			schemaPointer + "/allOf/0|#|type|level:object",
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
		{JSON: []byte(`"255.255.255.255"`), Valid: true},
	}, validCasesOnly(cases))
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
	}.Covered, validCoveredOnly(report.Covered))
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
			want:   `"0000-01-01T00:00:00.000Z"`,
		},
		{name: "email 4", schema: `{"type":"string","format":"email","minLength":4,"maxLength":4}`, want: `"!!@0"`},
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

func TestBuildSearchesEveryExactFormatTransitionClass(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
		want   string
	}{
		{
			name:   "base64 significant padding bits",
			schema: `{"type":"string","format":"byte","pattern":"^A[B-R]==$"}`,
			want:   `"AQ=="`,
		},
		{
			name:   "Gregorian leap day",
			schema: `{"type":"string","format":"date","pattern":"^190[0-5]-02-29$"}`,
			want:   `"1904-02-29"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cases, report := buildStringCases(
				t, []byte(documentWithJSONSchema(test.schema)), 100_000,
			)
			require.Contains(t, cases, Case{JSON: []byte(test.want), Valid: true})
			require.Equal(t, SpaceExhausted, report.Stop)
		})
	}
}

func TestBuildEnumeratesUnconstrainedAnyOfStringRules(t *testing.T) {
	t.Parallel()

	cases, _ := buildStringCases(t, []byte(documentWithJSONSchema(`{
		"type":"string",
		"anyOf":[
			{"pattern":"^a$"},
			{"pattern":"^b$"}
		]
	}`)), 1_000)

	require.Equal(t, []Case{
		{JSON: []byte(`"a"`), Valid: true},
		{JSON: []byte(`"a"`), Valid: true},
	}, cases)
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

func TestBuildFormatProductCoversLockedLanguage(t *testing.T) {
	t.Parallel()

	emails := []struct {
		name    string
		pattern string
		want    string
	}{
		{name: "uppercase", pattern: `^A@B$`, want: `A@B`},
		{name: "digit", pattern: `^0@a$`, want: `0@a`},
		{name: "mixed mailbox", pattern: `^John@x$`, want: `John@x`},
		{name: "dot atom", pattern: `^a\.b@x$`, want: `a.b@x`},
		{name: "quoted local", pattern: `^"a"@x$`, want: `"a"@x`},
		{name: "quoted escape", pattern: `^"a\\""@x$`, want: `"a\""@x`},
		{name: "hyphenated domain", pattern: `^a@a-b$`, want: `a@a-b`},
		{name: "IPv4 literal", pattern: `^a@\[1\.2\.3\.4\]$`, want: `a@[1.2.3.4]`},
		{name: "IPv6 literal", pattern: `^a@\[IPv6:::1\]$`, want: `a@[IPv6:::1]`},
		{name: "general literal", pattern: `^a@\[tag:value\]$`, want: `a@[tag:value]`},
	}
	for _, test := range emails {
		t.Run("email "+test.name, func(t *testing.T) {
			t.Parallel()

			schema := `{"type":"string","format":"email","pattern":` + strconv.Quote(test.pattern) + `}`
			cases, _ := buildStringCases(t, []byte(documentWithJSONSchema(schema)), 100_000)
			require.Contains(t, cases, Case{JSON: []byte(strconv.Quote(test.want)), Valid: true})
		})
	}

	for _, test := range []struct {
		name   string
		schema string
		want   string
	}{
		{
			name:   "date year zero",
			schema: `{"type":"string","format":"date","allOf":[{"pattern":"^0000-02-29$"}]}`,
			want:   `"0000-02-29"`,
		},
		{
			name: "date-time year zero",
			schema: `{"type":"string","format":"date-time",` +
				`"allOf":[{"pattern":"^0000-02-29T00:00:00Z$"}]}`,
			want: `"0000-02-29T00:00:00Z"`,
		},
		{
			name:   "uuid",
			schema: `{"type":"string","format":"uuid","pattern":"^00000000-0000-4000-8000-000000000000$"}`,
			want:   `"00000000-0000-4000-8000-000000000000"`,
		},
		{
			name:   "uuidv4",
			schema: `{"type":"string","format":"uuidv4","pattern":"^00000000-0000-4000-8000-000000000000$"}`,
			want:   `"00000000-0000-4000-8000-000000000000"`,
		},
		{
			name:   "uuid-v4",
			schema: `{"type":"string","format":"uuid-v4","pattern":"^00000000-0000-4000-8000-000000000000$"}`,
			want:   `"00000000-0000-4000-8000-000000000000"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cases, _ := buildStringCases(t, []byte(documentWithJSONSchema(test.schema)), 100_000)
			require.Contains(t, cases, Case{JSON: []byte(test.want), Valid: true})
		})
	}
}

func TestBuildReachesSurrogatePairForDirectedBMPComplement(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		length int
		want   []Case
		steps  uint64
	}{
		{
			name: "one rune", length: 1, steps: 195,
			want: []Case{
				{JSON: []byte(`"\u0000"`), Valid: true},
				{JSON: []byte(`null`), Valid: false},
				{JSON: []byte("\"𐀀\""), Valid: false},
				{JSON: []byte(`"\u0000\u0000"`), Valid: false},
			},
		},
		{
			name: "two runes", length: 2, steps: 111,
			want: []Case{
				{JSON: []byte(`"\u0000\u0000"`), Valid: true},
				{JSON: []byte(`null`), Valid: false},
				{JSON: []byte("\"\\u0000𐀀\""), Valid: false},
				{JSON: []byte(`"\u0000"`), Valid: false},
				{JSON: []byte(`"\u0000\u0000\u0000"`), Valid: false},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			schema := `{"type":"string","minLength":` + itoa(test.length) +
				`,"maxLength":` + itoa(test.length) +
				`,"pattern":"^[\\u0000-\\uD7FF\\uE000-\\uFFFF]+$"}`
			cases, report := buildStringCases(t, []byte(documentWithJSONSchema(schema)), 10_000)

			require.Equal(t, test.want, cases)
			require.Equal(t, SpaceExhausted, report.Stop)
			require.Equal(t, test.steps, report.Steps)
		})
	}
}

func TestBuildPreservesNestedAuthoredStringEnumAlternatives(t *testing.T) {
	t.Parallel()

	cases, _ := buildStringCases(t, []byte(documentWithJSONSchema(
		`{"type":"object","required":["x"],"properties":{"x":{"type":"string","enum":["m"],"pattern":"^[a-z]$"}}}`,
	)), 10_000)

	require.Contains(t, cases, Case{JSON: []byte(`{"x":"m"}`), Valid: true})
}

func TestBuildUsesSoundRuneMinimumsForNestedPatterns(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
		want   string
	}{
		{
			name:   "unanchored ASCII",
			schema: `{"type":"object","required":["x"],"properties":{"x":{"type":"string","pattern":"abc"}}}`,
			want:   `{"x":"abc"}`,
		},
		{
			name:   "astral one rune",
			schema: `{"type":"object","required":["x"],"properties":{"x":{"type":"string","pattern":"^😀$"}}}`,
			want:   `{"x":"😀"}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cases, _ := buildStringCases(t, []byte(documentWithJSONSchema(test.schema)), 10_000)
			require.Contains(t, cases, Case{JSON: []byte(test.want), Valid: true})
		})
	}
}

func TestBuildChargesUnconstrainedAnyOfDFSBeforeRetainingSiblingPaths(t *testing.T) {
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
	require.LessOrEqual(t, report.Steps, uint64(20))

	rejected, rejectedReport := buildStringCases(t, []byte(documentWithJSONSchema(`{
		"type":"string",
		"enum":["z"],
		"pattern":"^a*$"
	}`)), 100)
	require.Empty(t, rejected)
	require.Equal(t, SpaceExhausted, rejectedReport.Stop)
	require.Less(t, rejectedReport.Steps, uint64(100))
}

func TestBuildStreamsDirectedStringFaultAfterValidRows(t *testing.T) {
	t.Parallel()

	cases, report := buildStringCases(t, []byte(documentWithJSONSchema(`{
		"type":"string",
		"pattern":"^a$"
	}`)), 1_000)

	require.NotEmpty(t, cases)
	require.True(t, cases[0].Valid)
	require.Contains(t, cases, Case{JSON: []byte(`""`), Valid: false})
	require.Contains(t, strings.Join(report.Covered, ""), "|pattern|fault:pattern")
}

func TestBuildStreamsRegistryFormatBoundaries(t *testing.T) {
	t.Parallel()

	canonicalUUID := "00000000-0000-4000-8000-000000000000"
	domainLimit := strings.Repeat("!", 64) + "@0" + strings.Repeat("-", 61) + "0.0" +
		strings.Repeat("-", 61) + "0.0" + strings.Repeat("-", 59) + "0"

	for _, test := range []struct {
		name        string
		format      string
		valid       []string
		invalidJSON string
		steps       uint64
	}{
		{name: "byte", format: "byte", valid: []string{"", "+A==", "++0="}, invalidJSON: `"\u0000"`, steps: 66_032},
		{name: "date", format: "date", valid: []string{
			"0000-01-01", "1970-01-01", "0000-02-29", "0100-02-28", "9999-12-31",
		}, invalidJSON: `""`, steps: 1_690},
		{name: "date-time", format: "date-time", valid: []string{
			"0000-01-01T00:00:00Z", "1970-01-01T00:00:00Z", "0000-02-29T00:00:00Z",
			"0100-02-28T00:00:00Z", "9999-12-31T00:00:00Z",
		}, invalidJSON: `""`, steps: 5_796},
		{name: "email", format: "email", valid: []string{
			"!@0", "!@0", strings.Repeat("!", 64) + "@0", domainLimit,
		}, invalidJSON: `""`, steps: 10_972},
		{
			name: "ipv4", format: "ipv4",
			valid: []string{"0.0.0.0", "0.0.0.0", "255.255.255.255"}, invalidJSON: `""`, steps: 247,
		},
		{
			name: "uuid", format: "uuid",
			valid: []string{canonicalUUID, canonicalUUID}, invalidJSON: `""`, steps: 589,
		},
		{
			name: "uuidv4 alias", format: "uuidv4",
			valid: []string{canonicalUUID, canonicalUUID}, invalidJSON: `""`, steps: 589,
		},
		{
			name: "uuid-v4 alias", format: "uuid-v4",
			valid: []string{canonicalUUID, canonicalUUID}, invalidJSON: `""`, steps: 589,
		},
		{
			name: "cidr", format: "cidr",
			valid: []string{"0.0.0.0/0", "0.0.0.0/0", "0.0.0.0/32"}, invalidJSON: `""`, steps: 204,
		},
		{
			name: "ipv4-cidr alias", format: "ipv4-cidr",
			valid: []string{"0.0.0.0/0", "0.0.0.0/0", "0.0.0.0/32"}, invalidJSON: `""`, steps: 204,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cases, report := buildStringCases(t, []byte(documentWithJSONSchema(
				`{"type":"string","format":"`+test.format+`"}`,
			)), 1_000_000)

			want := make([]Case, 0, len(test.valid)+2)
			for _, value := range test.valid {
				want = append(want, Case{JSON: []byte(strconv.Quote(value)), Valid: true})
			}

			want = append(
				want,
				Case{JSON: []byte(`null`), Valid: false},
				Case{JSON: []byte(test.invalidJSON), Valid: false},
			)

			require.Equal(t, want, cases)

			schemaPointer := "#/paths/~1/post/requestBody/content/application~1json/schema"
			require.Equal(t, Report{
				Stop:  SpaceExhausted,
				Steps: test.steps,
				Covered: []string{
					schemaPointer + "|#|type|level:string",
					schemaPointer + "|#|type|fault:type",
					schemaPointer + "|#|format|level:valid",
					schemaPointer + "|#|format|fault:format",
				},
			}, report)
		})
	}
}

func TestBuildExhaustsContradictoryFormatLengthIntersection(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(
		`{"type":"string","allOf":[{"format":"date"},{"format":"uuid"}]}`,
	))
	firstCases, firstReport := buildStringCases(t, document, 100)
	secondCases, secondReport := buildStringCases(t, document, 100)

	require.Empty(t, firstCases)
	require.Equal(t, SpaceExhausted, firstReport.Stop)
	require.Equal(t, uint64(44), firstReport.Steps)
	require.Equal(t, firstCases, secondCases)
	require.Equal(t, firstReport, secondReport)
}

func TestBuildAcceptsPatternFixedMixedIPv6EmailLiterals(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		candidate string
		valid     bool
	}{
		{candidate: "a@[IPv6:::ffff:192.0.2.1]", valid: true},
		{candidate: "a@[IPv6:a:b:c:d:e:f:1.2.3.4]", valid: true},
		{candidate: "a@[IPv6:a:b:c:d:e:1.2.3.4]", valid: false},
		{candidate: "a@[IPv6:a:b:c:d:e:f:1.2.3]", valid: false},
		{candidate: "a@[IPv6:a:b:c:d:e:f:1..2.3.4]", valid: false},
		{candidate: "a@[IPv6:a:b:c:d:e:f:1.2.3.4.5]", valid: false},
		{candidate: "a@[IPv6:a:b:c:d:e:f:256.2.3.4]", valid: false},
	} {
		pattern := "^" + strings.NewReplacer("[", `\[`, "]", `\]`, ".", `\.`).Replace(test.candidate) + "$"
		schema := `{"type":"string","format":"email","pattern":` + strconv.Quote(pattern) + `}`
		cases, _ := buildStringCases(t, []byte(documentWithJSONSchema(schema)), 100_000)

		validCase := Case{JSON: []byte(strconv.Quote(test.candidate)), Valid: true}
		if test.valid {
			require.Contains(t, cases, validCase)
		} else {
			require.NotContains(t, cases, validCase)
		}
	}
}

func TestBuildAcceptsUnboundedDateTimeFraction(t *testing.T) {
	t.Parallel()

	candidate := "1970-01-01T00:00:00." + strings.Repeat("0", 65_536) + "Z"
	schema := `{"type":"string","format":"date-time","minLength":` + strconv.Itoa(len(candidate)) +
		`,"maxLength":` + strconv.Itoa(len(candidate)) + `,"pattern":` +
		strconv.Quote(`^1970-01-01T00:00:00\.0+Z$`) + `}`
	reached := errors.New("reached exact long date-time")
	_, err := Build(
		Input{
			OpenAPI:     []byte(documentWithJSONSchema(schema)),
			OperationID: "selected",
			MaxSteps:    2_000_000,
		},
		func(testCase Case) error {
			if testCase.Valid && string(testCase.JSON) == strconv.Quote(candidate) {
				return reached
			}

			return nil
		},
	)
	require.ErrorIs(t, err, reached)
}

func TestBuildFindsSemanticFormatObjectivesWithSiblingPatterns(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
		want   string
	}{
		{
			name:   "base64 canonical padding",
			schema: `{"type":"string","format":"byte","pattern":"^QUJDRA==$"}`,
			want:   `"QUJDRA=="`,
		},
		{
			name:   "email local limit",
			schema: `{"type":"string","format":"email","pattern":"^b{64}@example\\.com$"}`,
			want:   `"` + strings.Repeat("b", 64) + `@example.com"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cases, _ := buildStringCases(t, []byte(documentWithJSONSchema(test.schema)), 100_000)
			require.Contains(t, cases, Case{JSON: []byte(test.want), Valid: true})
		})
	}
}

func TestBuildFindsSemanticEmailLocalBoundaryWithSiblingPattern(t *testing.T) {
	t.Parallel()

	cases, _ := buildStringCases(t, []byte(documentWithJSONSchema(`{
		"type":"string","format":"email","pattern":"^b{64}@c$"
	}`)), 100_000)

	require.Contains(t, validCasesOnly(cases), Case{
		JSON: []byte(`"` + strings.Repeat("b", 64) + `@c"`), Valid: true,
	})
}

func TestBuildRetainsSiblingRulesForFormatBoundaries(t *testing.T) {
	t.Parallel()

	cases, _ := buildStringCases(t, []byte(documentWithJSONSchema(`{
		"type":"string","format":"date","pattern":"^2000-02-29$"
	}`)), 100_000)

	valid := validCasesOnly(cases)
	require.NotEmpty(t, valid)

	for _, testCase := range valid {
		require.Equal(t, `"2000-02-29"`, string(testCase.JSON))
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
