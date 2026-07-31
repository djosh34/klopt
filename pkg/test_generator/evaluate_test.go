//nolint:godoclint // Package-private semantic tests document evaluator truth.
package testgenerator

import (
	"fmt"
	"testing"

	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/validation"
	"github.com/stretchr/testify/require"
)

func TestEvaluatorMatchesRuntimeFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		value  string
	}{
		{name: "kind valid", schema: `{"type":"integer"}`, value: `1`},
		{name: "kind invalid", schema: `{"type":"integer"}`, value: `"1"`},
		{name: "enum valid", schema: `{"enum":[1,"a"]}`, value: `"a"`},
		{name: "enum invalid", schema: `{"enum":[1,"a"]}`, value: `true`},
		{name: "number valid", schema: `{"type":"number","minimum":1,"maximum":5,"multipleOf":2}`, value: `4`},
		{name: "number invalid", schema: `{"type":"number","minimum":1,"maximum":5,"multipleOf":2}`, value: `3`},
		{name: "string valid", schema: `{"type":"string","minLength":2,"maxLength":3,"pattern":"^a+$"}`, value: `"aa"`},
		{name: "string invalid", schema: `{"type":"string","minLength":2,"pattern":"^a+$"}`, value: `"b"`},
		{name: "array valid", schema: `{"type":"array","minItems":1,"items":{"type":"integer"}}`, value: `[2,4]`},
		{name: "array invalid", schema: `{"type":"array","minItems":1,"items":{"type":"integer"}}`, value: `[2,"4"]`},
		{
			name: "object valid",
			schema: `{"type":"object","required":["name"],
				"properties":{"name":{"type":"string"}},"additionalProperties":false}`,
			value: `{"name":"ok"}`,
		},
		{
			name: "object invalid",
			schema: `{"type":"object","required":["name"],
				"properties":{"name":{"type":"string"}},"additionalProperties":false}`,
			value: `{"name":"ok","extra":true}`,
		},
		{name: "all valid", schema: `{"allOf":[{"type":"string"},{"minLength":1}]}`, value: `"a"`},
		{name: "all invalid", schema: `{"allOf":[{"type":"string"},{"minLength":1}]}`, value: `""`},
		{name: "any valid", schema: `{"anyOf":[{"enum":["a"]},{"enum":["b"]}]}`, value: `"b"`},
		{name: "any invalid", schema: `{"anyOf":[{"enum":["a"]},{"enum":["b"]}]}`, value: `"c"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parsed, expression := mustCompileExpression(t, test.schema)
			value, err := jsonvalue.Parse([]byte(test.value))
			require.NoError(t, err)

			actual, err := expressionHolds(expression, value)
			require.NoError(t, err)

			expected := len(parsed["request"].Body.Validate([]byte(test.value))) == 0
			require.Equal(t, expected, actual)
		})
	}
}

func TestExpressionHoldsUsesAllAndAnyTruthTables(t *testing.T) {
	t.Parallel()

	value := jsonvalue.String("value")
	passing := newAtomExpression(atom{
		kind:    atomKinds,
		allowed: [jsonKindCount]bool{kindString: true},
	})
	failing := newAtomExpression(atom{
		kind:    atomKinds,
		allowed: [jsonKindCount]bool{kindNumber: true},
	})

	all := newAllExpression([]*expression{passing, failing})
	anyExpression := newAnyExpression([]*expression{passing, failing})
	emptyAll := newAllExpression(nil)
	emptyAny := newAnyExpression(nil)

	holds, err := expressionHolds(all, value)
	require.NoError(t, err)
	require.False(t, holds)
	holds, err = expressionHolds(anyExpression, value)
	require.NoError(t, err)
	require.True(t, holds)
	holds, err = expressionHolds(emptyAll, value)
	require.NoError(t, err)
	require.True(t, holds)
	holds, err = expressionHolds(emptyAny, value)
	require.NoError(t, err)
	require.False(t, holds)
}

func TestDemandsHoldRequiresEveryRequestedVerdict(t *testing.T) {
	t.Parallel()

	stringExpression := newAtomExpression(atom{
		kind:    atomKinds,
		allowed: [jsonKindCount]bool{kindString: true},
	})
	numberExpression := newAtomExpression(atom{
		kind:    atomKinds,
		allowed: [jsonKindCount]bool{kindNumber: true},
	})

	holds, err := demandsHold([]demand{
		newDemand(stringExpression, true),
		newDemand(numberExpression, false),
	}, jsonvalue.String("value"))
	require.NoError(t, err)
	require.True(t, holds)

	holds, err = demandsHold([]demand{newDemand(stringExpression, false)}, jsonvalue.String("value"))
	require.NoError(t, err)
	require.False(t, holds)
}

func mustCompileExpression(t *testing.T, schema string) (map[string]validation.RequestValidation, *expression) {
	t.Helper()

	document := []byte(fmt.Sprintf(`{
		"openapi":"3.0.4",
		"info":{"title":"test","version":"1"},
		"paths":{"/things":{"post":{
			"operationId":"request",
			"requestBody":{"content":{"application/json":{"schema":%s}}},
			"responses":{"204":{"description":"ok"}}
		}}}
	}`, schema))
	parsed, err := validation.Parse(document)
	require.NoError(t, err)

	lowerer := expressionLowerer{byValidation: make(map[*validation.Validation]*expression)}
	root, err := lowerer.lower(parsed["request"].Body)
	require.NoError(t, err)

	return parsed, root
}
