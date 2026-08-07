//nolint:godoclint // Tests pin the private clean format-search seam.
package schematest

import (
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
	require.Equal(t, uint64(2), searchState.steps)
	require.Equal(t, identityStrings(target.closure), identityStrings(evaluate(model, row).failures))
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
