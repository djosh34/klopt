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
	require.Equal(t, identityStrings(second.closure), identityStrings(evaluate(model, row).failures))
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
	require.Equal(t, identityStrings(target.closure), identityStrings(evaluate(model, row).failures))
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
			require.Equal(t, identityStrings(target.closure), identityStrings(evaluate(model, row).failures))
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
	require.Equal(t, identityStrings(target.closure), identityStrings(evaluate(model, row).failures))
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

func TestFindStringFaultRowCarriesNestedFailureToCompleteOracleCheck(t *testing.T) {
	t.Parallel()

	model, plan := parseStringFaultPlan(t, `{
		"type":"object",
		"required":["value"],
		"properties":{
			"value":{
				"type":"string",
				"pattern":"^a$",
				"maxLength":1
			}
		}
	}`)
	target := findFaultTarget(t, plan, "/properties/value|#/value|pattern|fault:pattern")
	searchState := &search{model: model, maxSteps: 1000}

	row, found, err := findStringFaultRow(target, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "", row.object["value"].text)
	require.Equal(t, identityStrings(target.closure), identityStrings(evaluate(model, row).failures))
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
