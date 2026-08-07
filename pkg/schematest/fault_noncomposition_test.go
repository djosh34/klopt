//nolint:godoclint // Focused private fault tests use behavior names.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNonCompositionFaultFamiliesHaveExactClosures(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"enum":           `{"type":"string","enum":["ok"]}`,
		"numeric bounds": `{"type":"number","minimum":1,"maximum":3,"multipleOf":1}`,
		"numeric format": `{"type":"number","format":"float"}`,
		"string":         `{"type":"string","minLength":2,"maxLength":4,"pattern":"^a+$"}`,
		"string format":  `{"type":"string","format":"ipv4"}`,
		"nullable type":  `{"type":"string","nullable":true}`,
		"array":          `{"type":"array","minItems":1,"maxItems":2,"items":{"type":"string"}}`,
		"object counts": `{"type":"object","minProperties":1,"maxProperties":3,` +
			`"properties":{"id":{"type":"string"},"name":{"type":"string"}}}`,
		"required property":  `{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}`,
		"forbidden property": `{"type":"object","properties":{"id":{"type":"string"}},"additionalProperties":false}`,
	}

	for name, schema := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			model, err := parseInput(Input{
				OpenAPI: []byte(documentWithJSONSchema(schema)), OperationID: "selected",
			})
			require.NoError(t, err)

			plan, err := makePlan(model)
			require.NoError(t, err)

			searchState := &search{model: model, maxSteps: 1_000_000}

			for _, fault := range plan.faultTargets {
				if fault.obligation.rule == oracleRuleAllOf || fault.obligation.rule == oracleRuleAnyOf {
					continue
				}

				parent, found, replayErr := regenerateParent(plan, fault, searchState)
				require.NoError(t, replayErr, fault.obligation.String())
				require.True(t, found, fault.obligation.String())
				parentJSON := marshalFaultTestValue(t, parent)

				derivative, applyErr := applyFault(parent, fault, searchState)
				require.NoError(t, applyErr, fault.obligation.String())
				require.Equal(t, parentJSON, marshalFaultTestValue(t, parent), fault.obligation.String())

				result := evaluate(model, derivative)
				require.NoError(t, result.err, fault.obligation.String())
				require.False(t, result.valid, fault.obligation.String())
				matches, matchErr := exactFailureClosure(result.failures, fault.closure)
				require.NoError(t, matchErr)
				require.True(
					t, matches, "%s: actual=%v expected=%v",
					fault.obligation.String(),
					identityStrings(result.failures),
					identityStrings(fault.closure),
				)
			}
		})
	}
}

func TestMaxPropertiesFaultBuildsAValidTypedAdditionalMember(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{"type":"object","maxProperties":0,` +
			`"additionalProperties":{"type":"boolean"}}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)
	fault := findFaultTarget(t, plan, "|maxProperties|fault:maxProperties")
	searchState := &search{model: model, maxSteps: 1000}
	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	derivative, err := applyFault(parent, fault, searchState)
	require.NoError(t, err)
	require.Equal(t, `{"__schematest_extra__":false}`, string(marshalFaultTestValue(t, derivative)))
}

func TestNestedFaultsDoNotStackAndUseConcreteInstanceIdentities(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"object",
			"required":["items"],
			"properties":{"items":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}}}
		}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	fault := findFaultTarget(t, plan, "#/items/*|minLength|fault:minLength")
	searchState := &search{model: model, maxSteps: 100_000}
	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	derivative, err := applyFault(parent, fault, searchState)
	require.NoError(t, err)

	result := evaluate(model, derivative)
	require.Equal(t, []string{
		fault.obligation.occurrence.usePointer + "|#/items/0|minLength",
	}, identityStrings(result.failures))
}
