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

func TestCountFaultRepairsUseActiveComposedSchemas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		faultID    string
		derivative string
	}{
		{
			name: "alternate composed item witness",
			schema: `{"type":"array","maxItems":0,"items":{"enum":["bad","ok"]},` +
				`"allOf":[{"items":{"enum":["ok"]}}]}`,
			faultID:    "|maxItems|fault:maxItems",
			derivative: `["ok"]`,
		},
		{
			name: "composed required member survives shrinking",
			schema: `{"type":"object","minProperties":2,"properties":{"a":{},"b":{}},` +
				`"allOf":[{"required":["a"]}]}`,
			faultID:    "|minProperties|fault:minProperties",
			derivative: `{"a":null}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, plan := compositionFaultModel(t, test.schema)
			fault := findFaultTarget(t, plan, test.faultID)
			searchState := &search{model: model, maxSteps: 100_000}
			parent, found, err := regenerateParent(plan, fault, searchState)
			require.NoError(t, err)
			require.True(t, found)

			derivative, err := applyFault(parent, fault, searchState)
			require.NoError(t, err)
			require.Equal(t, test.derivative, string(marshalFaultTestValue(t, derivative)))
		})
	}
}

func TestAdditionalPropertyFaultUsesActiveSiblingValueSchema(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"object",
		"allOf":[
			{"additionalProperties":false},
			{"additionalProperties":{"type":"string"}}
		]
	}`)
	fault := findFaultTarget(t, plan, "/allOf/0|#/*|additionalProperties|fault:additionalProperties")
	searchState := &search{model: model, maxSteps: 100_000}
	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	derivative, err := applyFault(parent, fault, searchState)
	require.NoError(t, err)
	require.Equal(t, `{"__schematest_extra__":""}`, string(marshalFaultTestValue(t, derivative)))
}

func TestLargeCountFaultsStopBeforeMaterialization(t *testing.T) {
	t.Parallel()

	for name, schema := range map[string]string{
		"array":  `{"type":"array","maxItems":1000000000000,"items":{}}`,
		"object": `{"type":"object","maxProperties":1000000000000}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			report, err := Build(Input{
				OpenAPI: []byte(documentWithJSONSchema(schema)), OperationID: "selected", MaxSteps: 20,
			}, func(Case) error { return nil })
			require.NoError(t, err)
			require.Equal(t, MaxStepsReached, report.Stop)
			require.Equal(t, uint64(20), report.Steps)
		})
	}
}

func TestBuildFindsActiveConjunctionFaultWitnesses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		faultID    string
		derivative string
	}{
		{
			name: "intersecting item patterns",
			schema: `{"maxItems":0,"items":{"type":"string","pattern":"^[a-z]+$"},` +
				`"allOf":[{"items":{"pattern":"^z+$"}}]}`,
			faultID:    "|maxItems|fault:maxItems",
			derivative: `["z"]`,
		},
		{
			name: "intersecting numeric additional property",
			schema: `{"allOf":[{"additionalProperties":false},` +
				`{"additionalProperties":{"type":"number","minimum":5}},` +
				`{"additionalProperties":{"multipleOf":2}}]}`,
			faultID:    "/allOf/0|#/*|additionalProperties|fault:additionalProperties",
			derivative: `{"__schematest_extra__":6}`,
		},
		{
			name: "composed string max property expansion",
			schema: `{"maxProperties":0,` +
				`"allOf":[{"additionalProperties":{"type":"string","minLength":2}}]}`,
			faultID:    "|maxProperties|fault:maxProperties",
			derivative: `{"__schematest_extra__":"text"}`,
		},
		{
			name: "named property backtracking",
			schema: `{"maxProperties":0,"properties":{"x":{"enum":["bad","good"]}},` +
				`"allOf":[{"properties":{"x":{"enum":["good"]}}}]}`,
			faultID:    "|maxProperties|fault:maxProperties",
			derivative: `{"x":"good"}`,
		},
		{
			name:       "enum fault from active sibling",
			schema:     `{"type":"string","allOf":[{"enum":["bad"]},{"enum":["bad","good"]}]}`,
			faultID:    "/allOf/0|#|enum|fault:enum",
			derivative: `"good"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, plan := compositionFaultModel(t, test.schema)
			fault := findFaultTarget(t, plan, test.faultID)
			searchState := &search{model: model, maxSteps: 1_000_000}
			parent, found, err := regenerateParent(plan, fault, searchState)
			require.NoError(t, err)
			require.True(t, found)

			derivative, err := applyFault(parent, fault, searchState)
			require.NoError(t, err)
			require.Equal(t, test.derivative, string(marshalFaultTestValue(t, derivative)))
		})
	}
}

func TestBuildTypelessNumericFormatFaults(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"float", "double"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			var cases []Case

			report, err := Build(Input{
				OpenAPI:     []byte(documentWithJSONSchema(`{"format":"` + format + `"}`)),
				OperationID: "selected",
				MaxSteps:    1_000_000,
			}, func(testCase Case) error {
				cases = append(cases, testCase)

				return nil
			})
			require.NoError(t, err)
			require.Equal(t, SpaceExhausted, report.Stop)

			foundNumericFault := false

			for _, testCase := range cases {
				if testCase.Valid || len(testCase.JSON) == 0 || testCase.JSON[0] == '"' ||
					string(testCase.JSON) == "null" || string(testCase.JSON) == "true" ||
					string(testCase.JSON) == "false" {
					continue
				}

				foundNumericFault = true
			}

			require.True(t, foundNumericFault)
		})
	}
}

func TestOversizedCountFaultAdvancesToCutoffAnalytically(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"array","maxItems":184467440737095516160,"items":{}
	}`)
	fault := findFaultTarget(t, plan, "|maxItems|fault:maxItems")
	searchState := &search{model: model, maxSteps: 1 << 60}
	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	_, err = applyFault(parent, fault, searchState)
	require.ErrorIs(t, err, errMaxSteps)
	require.Equal(t, searchState.maxSteps, searchState.steps)
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
