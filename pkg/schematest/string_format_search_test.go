//nolint:godoclint // Tests pin the private clean format-search seam.
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
			positive: []string{"YQ=="},
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

			require.Equal(t, test.positive, simpleStringFormatWitnesses(test.format, true))
			require.Equal(t, test.negative, simpleStringFormatWitnesses(test.format, false))

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
			cases = append(cases, testCase)

			return nil
		},
	)
	require.NoError(t, err)
	require.Contains(t, cases, Case{JSON: []byte(`"YQ=="`), Valid: true})
	require.Equal(t, SpaceExhausted, report.Stop)
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
	require.Equal(t, uint64(10), searchState.steps)
	require.Equal(t, identityStrings(target.closure), identityStrings(evaluate(model, row).failures))
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
			require.Equal(t, identityStrings(target.closure), identityStrings(evaluate(model, row).failures))
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
			witness: "255.255.255.255", stop: SpaceExhausted,
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

			report, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000}, func(testCase Case) error {
				cases = append(cases, testCase)

				return nil
			})
			require.NoError(t, err)
			require.Contains(t, cases, Case{JSON: []byte(`"` + test.witness + `"`), Valid: true})
			require.Equal(t, test.stop, report.Stop)
		})
	}
}

func TestPasswordAddsNoFormatObjective(t *testing.T) {
	t.Parallel()

	model, plan := parseStringFaultPlan(t, `{"type":"string","format":"password"}`)
	for _, target := range plan.faultTargets {
		require.NotEqual(t, oracleRuleFormat, target.obligation.rule)
	}

	result := evaluate(model, &jsonValue{kind: jsonString, text: "anything"})
	require.True(t, result.valid)
	require.NotContains(t, applicableRules(result.applicable), oracleRuleFormat)
}
