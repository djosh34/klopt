//nolint:godoclint // Tests requirement the private clean format-search seam.
package schematest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSimpleStringFormatWitnessesAreCanonicalAndDeterministic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		format   schemaFormat
		positive []string
		negative []string
	}{
		{
			name: "byte", format: schemaFormatByte,
			positive: []string{"YQ==", "YWI="},
			negative: []string{"YQ="},
		},
		{
			name: "date", format: schemaFormatDate,
			positive: []string{"1970-01-01", "2000-02-29", "1900-02-28", "9999-12-31"},
			negative: []string{"2001-02-29", "1900-02-29", "1970-13-01", "1970-01-32"},
		},
		{
			name: "date-time", format: schemaFormatDateTime,
			positive: []string{
				"1970-01-01T00:00:00Z",
				"2000-02-29T23:59:59.0Z",
				"1900-02-28T00:00:00+23:59",
				"9999-12-31T23:59:59-23:59",
			},
			negative: []string{
				"1970-01-01t00:00:00Z",
				"1970-01-01T00:00:60Z",
				"1970-01-01T00:00:00.Z",
				"1970-01-01T00:00:00+24:00",
			},
		},
		{
			name: "uuid", format: schemaFormatUUID,
			positive: []string{"00000000-0000-4000-8000-000000000000"},
			negative: []string{"00000000-0000-1000-8000-000000000000"},
		},
		{
			name: "uuidv4", format: schemaFormatUUIDv4,
			positive: []string{"00000000-0000-4000-8000-000000000000"},
			negative: []string{"00000000-0000-4000-7000-000000000000"},
		},
		{
			name: "uuid-v4", format: schemaFormatUUIDDashV4,
			positive: []string{"00000000-0000-4000-8000-000000000000"},
			negative: []string{"00000000-0000-4000-7000-000000000000"},
		},
		{
			name: "email", format: schemaFormatEmail,
			positive: []string{
				"a@b",
				strings.Repeat("a", 64) + "@b",
				strings.Repeat("a", 64) + "@" + strings.Repeat("b", 63) + "." +
					strings.Repeat("c", 63) + "." + strings.Repeat("d", 61),
			},
			negative: []string{
				"a..b@example.com",
				strings.Repeat("a", 65) + "@b",
				strings.Repeat("a", 64) + "@" + strings.Repeat("b", 63) + "." +
					strings.Repeat("c", 63) + "." + strings.Repeat("d", 62),
				"é@example.com",
			},
		},
		{
			name: "ipv4", format: schemaFormatIPv4,
			positive: []string{"0.0.0.0", "255.255.255.255"},
			negative: []string{"00.0.0.0", "256.255.255.255"},
		},
		{
			name: "cidr", format: schemaFormatCIDR,
			positive: []string{"192.0.2.7/0", "192.0.2.7/32"},
			negative: []string{"192.0.2.7/33", "192.0.2.7/00"},
		},
		{
			name: "ipv4-cidr", format: schemaFormatIPv4CIDR,
			positive: []string{"192.0.2.7/0", "192.0.2.7/32"},
			negative: []string{"192.0.2.7/33", "192.0.2.7/00"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.negative, stringFormatNegativeWitnesses(test.format))

			for _, witness := range test.positive {
				matches, err := cleanStringFormatMatches(witness, test.format)
				require.NoError(t, err)
				require.True(t, matches, witness)
			}

			for _, witness := range test.negative {
				matches, err := cleanStringFormatMatches(witness, test.format)
				require.NoError(t, err)
				require.False(t, matches, witness)
			}
		})
	}
}

func TestStringFormatRegistryOwnsProgramsAliasesBoundsAndObjectives(t *testing.T) {
	t.Parallel()

	for _, format := range []schemaFormat{
		schemaFormatByte, schemaFormatDate, schemaFormatDateTime, schemaFormatEmail,
		schemaFormatIPv4, schemaFormatUUID, schemaFormatCIDR, schemaFormatPassword,
	} {
		specification, exists := stringFormatSpecificationFor(format)
		require.True(t, exists, format)
		require.NotNil(t, specification)
	}

	uuid, _ := stringFormatSpecificationFor(schemaFormatUUID)
	uuidv4, _ := stringFormatSpecificationFor(schemaFormatUUIDv4)
	uuidDashV4, _ := stringFormatSpecificationFor(schemaFormatUUIDDashV4)

	require.Same(t, uuid, uuidv4)
	require.Same(t, uuid, uuidDashV4)

	cidr, _ := stringFormatSpecificationFor(schemaFormatCIDR)
	ipv4CIDR, _ := stringFormatSpecificationFor(schemaFormatIPv4CIDR)
	require.Same(t, cidr, ipv4CIDR)

	date, _ := stringFormatSpecificationFor(schemaFormatDate)
	require.True(t, date.bounds.allows(10))
	require.False(t, date.bounds.allows(9))
	require.NotZero(t, date.objectiveCount)

	password, _ := stringFormatSpecificationFor(schemaFormatPassword)
	require.True(t, password.inert)
	require.Zero(t, password.objectiveCount)
}

func TestStringFormatProgramsAcceptExactRetainedLanguages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format  schemaFormat
		valid   []string
		invalid []string
	}{
		{schemaFormatByte, []string{"", "YQ==", "YWI="}, []string{"YR==", "YWJ=", "YQ="}},
		{schemaFormatDate, []string{"0000-02-29", "2000-02-29", "1900-02-28"}, []string{"1900-02-29", "2001-02-29"}},
		{
			schemaFormatDateTime,
			[]string{"2000-02-29T23:59:59.0Z", "1900-02-28T00:00:00+23:59"},
			[]string{"2001-02-29T00:00:00Z", "1970-01-01t00:00:00Z"},
		},
		{schemaFormatEmail, []string{"a@b", "a.b@example.com"}, []string{"a..b@example.com", "é@example.com"}},
		{schemaFormatIPv4, []string{"0.0.0.0", "255.255.255.255"}, []string{"00.0.0.0", "256.0.0.0"}},
		{schemaFormatCIDR, []string{"0.0.0.0/0", "255.255.255.255/32"}, []string{"0.0.0.0/00", "0.0.0.0/33"}},
		{
			schemaFormatUUID,
			[]string{"00000000-0000-4000-8000-000000000000"},
			[]string{"00000000-0000-1000-8000-000000000000"},
		},
	}

	for _, test := range tests {
		specification, exists := stringFormatSpecificationFor(test.format)
		require.True(t, exists)

		for _, candidate := range test.valid {
			require.True(t, specification.program.accepts(candidate), "%d accepts %q", test.format, candidate)
		}

		for _, candidate := range test.invalid {
			require.False(t, specification.program.accepts(candidate), "%d rejects %q", test.format, candidate)
		}
	}
}

func TestEmailIPv6MixedSuffixRequiresAuthoredCompression(t *testing.T) {
	t.Parallel()

	valid := []string{
		"a@[IPv6:::ffff:192.0.2.1]",
		"a@[IPv6:a:b:c:d:e:f:1.2.3.4]",
		"a@[IPv6:a:b::1.2.3.4]",
	}
	invalid := []string{
		"a@[IPv6:a:b:1.2.3.4]",
		"a@[IPv6:a:b:c:d:e:1.2.3.4]",
		"a@[IPv6:a:b:c:d:e:f:1.2.3]",
		"a@[IPv6:a:b:c:d:e:f:1..2.3.4]",
		"a@[IPv6:a:b:c:d:e:f:1.2.3.4.5]",
		"a@[IPv6:a:b:c:d:e:f:256.2.3.4]",
	}

	specification, exists := stringFormatSpecificationFor(schemaFormatEmail)
	require.True(t, exists)

	for _, test := range []struct {
		candidates []string
		want       bool
	}{
		{candidates: valid, want: true},
		{candidates: invalid, want: false},
	} {
		for _, candidate := range test.candidates {
			cleanMatches, err := cleanStringFormatMatches(candidate, schemaFormatEmail)
			require.NoError(t, err)
			require.Equal(t, test.want, cleanMatches, candidate)
			require.Equal(t, cleanMatches, searchEmailFormatMatches(candidate), candidate)
			require.Equal(t, cleanMatches, specification.program.accepts(candidate), candidate)
		}
	}
}

func TestDateTimeFractionDoesNotOverflowIncrementalState(t *testing.T) {
	t.Parallel()

	specification, exists := stringFormatSpecificationFor(schemaFormatDateTime)
	require.True(t, exists)

	for _, suffix := range []string{"Z", "+00:00"} {
		candidate := "1970-01-01T00:00:00." + strings.Repeat("0", 65_536) + suffix
		cleanMatches, err := cleanStringFormatMatches(candidate, schemaFormatDateTime)
		require.NoError(t, err)
		require.True(t, cleanMatches)
		require.True(t, searchDateTimeFormatMatches(candidate))
		require.True(t, specification.program.accepts(candidate))
	}

	require.False(t, specification.program.accepts("1970-01-01T00:00:00.Z"))
}

func TestStringFormatTransitionPartitionsSeparateExactSemantics(t *testing.T) {
	t.Parallel()

	base64Specification, _ := stringFormatSpecificationFor(schemaFormatByte)

	base64State := base64Specification.program.start(4)
	for _, unit := range []uint16{'Y', 'Q'} {
		base64State = base64Specification.program.advance(base64State, unit)
	}

	require.NotEqual(
		t,
		base64Specification.program.transition(base64State, 'A'),
		base64Specification.program.transition(base64State, 'R'),
	)

	dateSpecification, _ := stringFormatSpecificationFor(schemaFormatDate)

	dateState := dateSpecification.program.start(10)
	for _, unit := range []uint16{'2', '0', '0', '0', '-', '0', '2', '-'} {
		dateState = dateSpecification.program.advance(dateState, unit)
	}

	require.NotEqual(
		t,
		dateSpecification.program.transition(dateState, '2'),
		dateSpecification.program.transition(dateState, '3'),
	)
}

func TestStringFormatTransitionClassesHaveEqualSuccessors(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		format    schemaFormat
		candidate string
	}{
		{schemaFormatByte, "YQ=="},
		{schemaFormatDate, "2000-02-29"},
		{schemaFormatDateTime, "2000-02-29T23:59:59Z"},
		{schemaFormatEmail, "a@example.com"},
		{schemaFormatIPv4, "255.0.0.0"},
		{schemaFormatUUID, "00000000-0000-4000-8000-000000000000"},
		{schemaFormatCIDR, "192.0.2.7/32"},
	} {
		specification, exists := stringFormatSpecificationFor(test.format)
		require.True(t, exists)

		state := specification.program.start(len(test.candidate))
		for _, selected := range test.candidate {
			classes := make(map[uint32]stringFormatProgramState)

			specification.program.eachUnit(func(unit uint16) {
				class := specification.program.transition(state, unit)
				if class == 0 {
					return
				}

				successor := specification.program.advance(state, unit)
				if previous, found := classes[class]; found {
					require.Equal(t, previous, successor)
				} else {
					classes[class] = successor
				}
			})

			state = specification.program.advance(state, uint16(selected))
		}
	}
}

func TestBuildFindsContinuationDistinctDateAndIPv4Units(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
		want   string
	}{
		{
			name:   "date month tens",
			schema: `{"type":"string","format":"date","pattern":"^[0-9]{4}-[0-9]0-01$"}`,
			want:   `"0000-10-01"`,
		},
		{
			name:   "ipv4 octet value",
			schema: `{"type":"string","format":"ipv4","pattern":"^[0-9]55\\.0\\.0\\.0$"}`,
			want:   `"155.0.0.0"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cases, _ := buildStringCases(t, []byte(documentWithJSONSchema(test.schema)), 100_000)
			require.Contains(t, cases, Case{JSON: []byte(test.want), Valid: true})
		})
	}
}

func TestBuildSearchesSimpleFormatAcrossActiveAllOfConstraints(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"string",
		"pattern":"^YQ==$",
		"allOf":[{"format":"byte"}]
	}`))
	cases := make([]Case, 0)

	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000},
		func(testCase Case) error {
			cases = append(cases, retainCase(testCase))

			return nil
		},
	)
	require.NoError(t, err)
	require.Contains(t, cases, Case{JSON: []byte(`"YQ=="`), Valid: true})
	require.Equal(t, SpaceExhausted, report.Stop)
}

func TestBuildFindsExactBase64AndGregorianIntersections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		matches func(string) bool
	}{
		{
			name:    "base64 significant padding bits",
			schema:  `{"type":"string","format":"byte","pattern":"^Y[B-R]==$"}`,
			matches: searchByteFormatMatches,
		},
		{
			name:    "Gregorian leap boundary",
			schema:  `{"type":"string","format":"date","pattern":"^19[0-9][0-9]-02-29$"}`,
			matches: searchDateFormatMatches,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var witness string

			_, err := Build(
				Input{OpenAPI: []byte(documentWithJSONSchema(test.schema)), OperationID: "selected", MaxSteps: 10000},
				func(testCase Case) error {
					if testCase.Valid {
						value, parseErr := parseStrictJSON(testCase.JSON)
						require.NoError(t, parseErr)

						if value.kind == jsonString && test.matches(value.text) {
							witness = value.text
						}
					}

					return nil
				},
			)
			require.NoError(t, err)
			require.NotEmpty(t, witness)
		})
	}
}

func TestFindStringFaultRowDirectsFormatAndPreservesSiblingPattern(t *testing.T) {
	t.Parallel()

	model, plan := parseStringFaultPlan(t, `{
		"type":"string",
		"format":"byte",
		"pattern":"^YQ=$"
	}`)
	target := findFaultTarget(t, plan, "|format|fault:format")
	searchState := &search{model: model, maxSteps: 100}

	row, found, err := findStringFaultRow(target, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "YQ=", row.text)
	require.Equal(t, uint64(4), searchState.steps)
	require.Equal(t, identityStrings(target.expected), identityStrings(evaluate(model, row).failureRecords()))
}

func TestFindStringFaultRowDirectsRemainingFormatsAndPreservesSiblings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		witness string
		steps   uint64
	}{
		{
			name: "email",
			schema: `{"type":"string","format":"email","pattern":"^a\\.\\.b@example\\.com$",` +
				`"minLength":16,"maxLength":16}`,
			witness: "a..b@example.com",
			steps:   17,
		},
		{
			name:    "ipv4",
			schema:  `{"type":"string","format":"ipv4","pattern":"^00\\.0\\.0\\.0$","minLength":8,"maxLength":8}`,
			witness: "00.0.0.0",
			steps:   9,
		},
		{
			name: "cidr",
			schema: `{"type":"string","format":"cidr","pattern":"^192\\.0\\.2\\.7/33$",` +
				`"minLength":12,"maxLength":12}`,
			witness: "192.0.2.7/33",
			steps:   13,
		},
		{
			name: "ipv4-cidr",
			schema: `{"type":"string","format":"ipv4-cidr","pattern":"^192\\.0\\.2\\.7/33$",` +
				`"minLength":12,"maxLength":12}`,
			witness: "192.0.2.7/33",
			steps:   13,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, plan := parseStringFaultPlan(t, test.schema)
			target := findFaultTarget(t, plan, "|format|fault:format")
			searchState := &search{model: model, maxSteps: 100}

			row, found, err := findStringFaultRow(target, searchState)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, test.witness, row.text)
			require.Equal(t, test.steps, searchState.steps)
			require.Equal(t, identityStrings(target.expected), identityStrings(evaluate(model, row).failureRecords()))
		})
	}
}

func TestBuildSearchesRemainingFormatsAcrossActiveSiblingConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		format  string
		pattern string
		length  int
		witness string
		stop    StopReason
	}{
		{
			name: "email", format: "email", pattern: `^a@b$`, length: 3,
			witness: "a@b", stop: SpaceExhausted,
		},
		{
			name: "ipv4", format: "ipv4", pattern: `^255\\.255\\.255\\.255$`, length: 15,
			witness: "255.255.255.255", stop: MaxStepsReached,
		},
		{
			name: "cidr", format: "cidr", pattern: `^192\\.0\\.2\\.7/32$`, length: 12,
			witness: "192.0.2.7/32", stop: SpaceExhausted,
		},
		{
			name: "ipv4-cidr", format: "ipv4-cidr", pattern: `^192\\.0\\.2\\.7/32$`, length: 12,
			witness: "192.0.2.7/32", stop: SpaceExhausted,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			document := []byte(documentWithJSONSchema(`{"type":"string","format":"` + test.format +
				`","pattern":"` + test.pattern + `","minLength":` + itoa(test.length) +
				`,"maxLength":` + itoa(test.length) + `}`))
			cases := make([]Case, 0)

			report, err := Build(
				Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100_000},
				func(testCase Case) error {
					cases = append(cases, retainCase(testCase))

					return nil
				},
			)
			require.NoError(t, err)
			require.Contains(t, cases, Case{JSON: []byte(`"` + test.witness + `"`), Valid: true})
			require.Equal(t, test.stop, report.Stop)
		})
	}
}

func TestPasswordAddsNoFormatObjective(t *testing.T) {
	t.Parallel()

	model, plan := parseStringFaultPlan(t, `{"type":"string","format":"password"}`)
	for _, target := range plan.faultSchedule {
		require.NotEqual(t, oracleRuleFormat, target.obligation.rule)
	}

	result := evaluate(model, &jsonValue{kind: jsonString, text: "anything"})
	require.True(t, result.valid)
	require.NotContains(t, applicableRules(result.applicableRecords()), oracleRuleFormat)
}
