//nolint:godoclint // Tests pin private directed string-search behavior.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindStringFaultRowDirectsEachPatternInAuthoredOrder(t *testing.T) {
	t.Parallel()

	model, plan := parseStringFaultPlan(t, `{
		"type":"string",
		"pattern":"^[a-b]$",
		"allOf":[{"pattern":"^a$"}]
	}`)

	first := findFaultTarget(t, plan, "|pattern|fault:pattern")
	firstSearch := &search{model: model, maxSteps: 100}
	row, found, err := findStringFaultRow(first, firstSearch)
	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, row)

	second := findFaultTarget(t, plan, "/allOf/0|#|pattern|fault:pattern")
	secondSearch := &search{model: model, maxSteps: 100}
	row, found, err = findStringFaultRow(second, secondSearch)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "b", row.text)
	require.Equal(t, identityStrings(second.closure), identityStrings(evaluate(model, row).failureRecords()))
}

func TestFindStringFaultRowTriesLeadingAssertionFailureAlternatives(t *testing.T) {
	t.Parallel()

	model, plan := parseStringFaultPlan(t, `{
		"type":"string",
		"pattern":"^(?!a)[a-b]$",
		"allOf":[{"pattern":"^[a-b]$"}]
	}`)
	target := findFaultTarget(t, plan, "|pattern|fault:pattern")
	searchState := &search{model: model, maxSteps: 100}

	row, found, err := findStringFaultRow(target, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "a", row.text)
	require.Equal(t, identityStrings(target.closure), identityStrings(evaluate(model, row).failureRecords()))
}

func TestFindStringFaultRowDirectsLengthBoundsAndPreservesPatterns(t *testing.T) {
	t.Parallel()

	model, plan := parseStringFaultPlan(t, `{
		"type":"string",
		"minLength":2,
		"maxLength":3,
		"pattern":"^a+$"
	}`)

	tests := []struct {
		rule string
		want string
	}{
		{rule: oracleRuleMinLength, want: "a"},
		{rule: oracleRuleMaxLength, want: "aaaa"},
	}

	for _, test := range tests {
		t.Run(test.rule, func(t *testing.T) {
			t.Parallel()

			target := findFaultTarget(t, plan, "|"+test.rule+"|fault:"+test.rule)
			searchState := &search{model: model, maxSteps: 1000}
			row, found, err := findStringFaultRow(target, searchState)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, test.want, row.text)
			require.Equal(t, identityStrings(target.closure), identityStrings(evaluate(model, row).failureRecords()))
		})
	}
}

func TestFindStringFaultRowPinsMaxLengthFailureAtSiblingMinimum(t *testing.T) {
	t.Parallel()

	model, plan := parseStringFaultPlan(t, `{
		"type":"string",
		"maxLength":2,
		"allOf":[{"minLength":5}]
	}`)
	target := findFaultTarget(t, plan, "|maxLength|fault:maxLength")
	searchState := &search{model: model, maxSteps: 100}

	row, found, err := findStringFaultRow(target, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "\x00\x00\x00\x00\x00", row.text)
}

func TestFindStringFaultRowRejectsEmptyMaxLengthFailureRange(t *testing.T) {
	t.Parallel()

	model, plan := parseStringFaultPlan(t, `{
		"type":"string",
		"maxLength":2,
		"allOf":[{"minLength":5,"maxLength":4}]
	}`)
	target := findFaultTarget(t, plan, "|maxLength|fault:maxLength")
	searchState := &search{model: model, maxSteps: 100}

	row, found, err := findStringFaultRow(target, searchState)
	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, row)
}

func TestFindStringFaultRowResolvesNestedScalarTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		fault  string
		want   string
	}{
		{
			name: "object property pattern",
			schema: `{"type":"object","properties":{"value":{` +
				`"type":"string","pattern":"^a$","maxLength":1}}}`,
			fault: "/properties/value|#/value|pattern|fault:pattern",
			want:  "",
		},
		{
			name: "array item format",
			schema: `{"type":"array","items":{"type":"string","format":"ipv4",` +
				`"pattern":"^(?:1\\.2\\.3\\.4|999\\.999\\.999\\.999)$"}}`,
			fault: "/items|#/*|format|fault:format",
			want:  "999.999.999.999",
		},
		{
			name: "additional property length",
			schema: `{"type":"object","additionalProperties":{` +
				`"type":"string","maxLength":2,"pattern":"^...$"}}`,
			fault: "/additionalProperties|#/*|maxLength|fault:maxLength",
			want:  "\x00\x00\x00",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, plan := parseStringFaultPlan(t, test.schema)
			target := findFaultTarget(t, plan, test.fault)
			searchState := &search{model: model, maxSteps: 100_000}

			row, found, err := findStringFaultRow(target, searchState)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, test.want, row.text)

			node, occurrence, resolved := resolveStringFaultTarget(
				model.root, model.root.occurrence, target.obligation.occurrence,
			)
			require.True(t, resolved)

			result := evaluateNode(node, row, occurrence)
			matches, err := exactFailureClosure(result.failureRecords(), target.closure)
			require.NoError(t, err)
			require.True(t, matches)
		})
	}
}

func TestFindStringFaultRowDirectsLengthWithoutAnAuthoredPattern(t *testing.T) {
	t.Parallel()

	model, plan := parseStringFaultPlan(t, `{
		"type":"string",
		"minLength":1
	}`)
	target := findFaultTarget(t, plan, "|minLength|fault:minLength")
	searchState := &search{model: model, maxSteps: 100}

	row, found, err := findStringFaultRow(target, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.Empty(t, row.text)
	require.Equal(t, identityStrings(target.closure), identityStrings(evaluate(model, row).failureRecords()))
}

func TestFindStringFaultRowLeavesContradictoryObjectiveUncovered(t *testing.T) {
	t.Parallel()

	model, plan := parseStringFaultPlan(t, `{
		"type":"string",
		"minLength":2,
		"pattern":"^aa$"
	}`)
	target := findFaultTarget(t, plan, "|minLength|fault:minLength")
	searchState := &search{model: model, maxSteps: 100}

	row, found, err := findStringFaultRow(target, searchState)
	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, row)
	require.Less(t, searchState.steps, searchState.maxSteps)
}

func TestFindStringFaultRowSearchesFormatsIncrementally(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		fault      string
		checkValue func(*testing.T, string)
	}{
		{
			name: "date pattern false",
			schema: `{"type":"string","format":"date",` +
				`"pattern":"^(?:1970|2000|1900|9999|2024)-[0-9]{2}-[0-9]{2}$"}`,
			fault: "|pattern|fault:pattern",
			checkValue: func(t *testing.T, value string) {
				require.Equal(t, "0000-01-01", value)
			},
		},
		{
			name:   "email pattern false",
			schema: `{"type":"string","format":"email","pattern":"^a@b$"}`,
			fault:  "|pattern|fault:pattern",
			checkValue: func(t *testing.T, value string) {
				require.NotEqual(t, "a@b", value)
			},
		},
		{
			name: "ipv4 format false",
			schema: `{"type":"string","format":"ipv4",` +
				`"pattern":"^(?:1\\.2\\.3\\.4|999\\.999\\.999\\.999)$"}`,
			fault: "|format|fault:format",
			checkValue: func(t *testing.T, value string) {
				require.Equal(t, "999.999.999.999", value)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, plan := parseStringFaultPlan(t, test.schema)
			target := findFaultTarget(t, plan, test.fault)
			searchState := &search{model: model, maxSteps: 1_000_000}
			row, found, err := findStringFaultRow(target, searchState)
			require.NoError(t, err)
			require.True(t, found)
			test.checkValue(t, row.text)
			require.Equal(t, identityStrings(target.closure), identityStrings(evaluate(model, row).failureRecords()))
		})
	}
}

func TestFindStringFaultRowFindsNonLowercaseValidEmail(t *testing.T) {
	t.Parallel()

	model, plan := parseStringFaultPlan(t, `{
		"type":"string",
		"format":"email",
		"pattern":"^[a-z]+@[a-z]+$"
	}`)
	target := findFaultTarget(t, plan, "|pattern|fault:pattern")
	searchState := &search{model: model, maxSteps: 100_000}

	row, found, err := findStringFaultRow(target, searchState)
	require.NoError(t, err)
	require.True(t, found)

	formatMatches, err := cleanStringFormatMatches(row.text, schemaFormatEmail)
	require.NoError(t, err)
	require.True(t, formatMatches)

	patternMatches, err := cleanPatternMatches(model.root.pattern, row.text)
	require.NoError(t, err)
	require.False(t, patternMatches)
	require.Equal(t, identityStrings(target.closure), identityStrings(evaluate(model, row).failureRecords()))
}

func TestFindStringFaultRowDirectedFalseExpandsSurrogates(t *testing.T) {
	t.Parallel()

	model, plan := parseStringFaultPlan(t, `{
		"type":"string",
		"minLength":1,
		"maxLength":1,
		"pattern":"^[^]$"
	}`)
	target := findFaultTarget(t, plan, "|pattern|fault:pattern")
	searchState := &search{model: model, maxSteps: 10_000}

	row, found, err := findStringFaultRow(target, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "𐀀", row.text)
}

func TestFindStringFaultRowChargesRetriesGlobally(t *testing.T) {
	t.Parallel()

	model, plan := parseStringFaultPlan(t, `{
		"type":"string",
		"pattern":"^[a-b]$",
		"allOf":[{"pattern":"^b$"}]
	}`)
	target := findFaultTarget(t, plan, "|pattern|fault:pattern")
	searchState := &search{model: model, maxSteps: 2}

	row, found, err := findStringFaultRow(target, searchState)
	require.ErrorIs(t, err, errMaxSteps)
	require.False(t, found)
	require.Nil(t, row)
	require.Equal(t, uint64(2), searchState.steps)
}

func parseStringFaultPlan(t *testing.T, schema string) (*schemaModel, *searchPlan) {
	t.Helper()

	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(schema)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	return model, plan
}
