//nolint:godoclint // Focused private fault tests use behavior names.
package schematest

import (
	"errors"
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

			for _, fault := range plan.faultSchedule {
				if fault.obligation.rule == oracleRuleAllOf || fault.obligation.rule == oracleRuleAnyOf {
					continue
				}

				if fault.alternatives != nil {
					continue
				}

				parent, found, replayErr := regenerateParent(plan, fault, searchState)
				require.NoError(t, replayErr, fault.obligation.String())
				require.True(t, found, fault.obligation.String())
				parentJSON := marshalFaultTestValue(t, parent)

				derivative, applyErr := applyFault(parent, fault, searchState)
				if errors.Is(applyErr, errFaultNotFound) {
					continue
				}

				require.NoError(t, applyErr, fault.obligation.String())
				require.Equal(t, parentJSON, marshalFaultTestValue(t, parent), fault.obligation.String())

				result := evaluate(model, derivative)
				require.NoError(t, result.err, fault.obligation.String())
				require.False(t, result.valid, fault.obligation.String())
				matches, matchErr := faultFailureClosureMatches(result, fault)
				require.NoError(t, matchErr)
				require.True(
					t, matches, "%s: actual=%v expected=%v",
					fault.obligation.String(),
					identityStrings(result.failureRecords()),
					identityStrings(fault.expected),
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

func TestObjectFaultRejectsAnImpossibleRootKindBeforeCharging(t *testing.T) {
	t.Parallel()

	model, _ := compositionFaultModel(t, `{"maxProperties":0,"allOf":[{"type":"string"}]}`)
	identity := makeRuleIdentity(model.root.occurrence, oracleRuleMaxProperties)
	fault := faultProgram{
		obligation: makeFaultObligation(identity, oracleRuleMaxProperties),
		expected:   failureSet{failureIdentity(identity)},
	}
	searchState := &search{model: model, maxSteps: 10}

	derivative, found, err := findNonCompositionDerivative(
		&jsonValue{kind: jsonString}, fault, searchState,
	)
	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, derivative)
	require.Zero(t, searchState.steps)
}

func TestMaxPropertiesFaultAdvancesPastAParentKeyCollision(t *testing.T) {
	t.Parallel()

	requireFaultDerivative(
		t,
		`{"type":"object","maxProperties":1,"default":{"__schematest_extra__":false}}`,
		"|maxProperties|fault:maxProperties",
		`{"__schematest_extra__":false,"__schematest_extra___1":null}`,
	)
}

func TestObjectCountFaultMutatesOnlyOneConcreteOccurrence(t *testing.T) {
	t.Parallel()

	requireFaultDerivative(
		t,
		`{"type":"array","minItems":2,"maxItems":2,"items":{"type":"object","maxProperties":0}}`,
		"/items|#/*|maxProperties|fault:maxProperties",
		`[{"__schematest_extra__":null},{}]`,
	)
}

func TestArrayCountFaultsPreserveWholeArrayEnums(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		faultID    string
		derivative string
	}{
		{
			name: "minimum deletes a non-prefix position",
			schema: `{"type":"array","minItems":2,"enum":[["a","b"],["b"]],` +
				`"items":{"type":"string"}}`,
			faultID:    "|minItems|fault:minItems",
			derivative: `["b"]`,
		},
		{
			name: "maximum inserts the enum item at its authored position",
			schema: `{"type":"array","maxItems":1,"enum":[["x"],["x","special"]],` +
				`"items":{"type":"string"}}`,
			faultID:    "|maxItems|fault:maxItems",
			derivative: `["x","special"]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			requireFaultDerivative(t, test.schema, test.faultID, test.derivative)
		})
	}
}

func TestArrayCountFaultPositionsAdvanceCanonically(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		faultID    string
		derivative string
	}{
		{
			name: "deletion starts at the first position",
			schema: `{"type":"array","minItems":2,"maxItems":2,"default":["a","b"],` +
				`"items":{"type":"string"}}`,
			faultID:    "|minItems|fault:minItems",
			derivative: `["b"]`,
		},
		{
			name: "insertion starts at the first position",
			schema: `{"type":"array","maxItems":1,"default":["x"],` +
				`"items":{"type":"string"}}`,
			faultID:    "|maxItems|fault:maxItems",
			derivative: `["","x"]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			requireFaultDerivative(t, test.schema, test.faultID, test.derivative)
		})
	}
}

func TestArrayCountFaultCutoffPrecedesEachAtomicAssignment(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{"type":"array","minItems":2,`+
		`"enum":[["a","b"],["b"]],"items":{"type":"string"}}`)
	fault := findFaultTarget(t, plan, "|minItems|fault:minItems")
	searchState := &search{model: model, maxSteps: 100_000}
	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	beforeMutation := searchState.steps
	searchState.maxSteps = beforeMutation + 3
	derivative, err := applyFault(parent, fault, searchState)
	require.ErrorIs(t, err, errMaxSteps)
	require.Nil(t, derivative)
	require.Equal(t, searchState.maxSteps, searchState.steps)

	searchState = &search{model: model, maxSteps: 100_000}
	parent, found, err = regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	beforeMutation = searchState.steps
	derivative, err = applyFault(parent, fault, searchState)
	require.NoError(t, err)
	require.Equal(t, `["b"]`, string(marshalFaultTestValue(t, derivative)))
	require.Equal(t, uint64(4), searchState.steps-beforeMutation)
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
			derivative: `{"a":false}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			requireFaultDerivative(t, test.schema, test.faultID, test.derivative)
		})
	}
}

func requireFaultDerivative(t *testing.T, schema, faultID, expected string) {
	t.Helper()

	model, plan := compositionFaultModel(t, schema)
	fault := findFaultTarget(t, plan, faultID)
	searchState := &search{model: model, maxSteps: 100_000}
	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	derivative, err := applyFault(parent, fault, searchState)
	require.NoError(t, err)
	require.Equal(t, expected, string(marshalFaultTestValue(t, derivative)))
}

func TestComposedEnumFaultPreservesSiblingType(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{"allOf":[{"type":"string"},{"enum":["ok",7]}]}`)
	fault := findFaultTarget(t, plan, "/allOf/1|#|enum|fault:enum")
	searchState := &search{model: model, maxSteps: 10_000}
	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	derivative, err := applyFault(parent, fault, searchState)
	require.NoError(t, err)
	require.Equal(t, `""`, string(marshalFaultTestValue(t, derivative)))
}

func TestBuildTypeFaultUsesActiveSiblingEnumWitness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		derivative string
	}{
		{
			name:       "number enum witness",
			schema:     `{"allOf":[{"type":"string"},{"enum":["ok",7]}]}`,
			derivative: `7`,
		},
		{
			name:       "large integer enum witness",
			schema:     `{"allOf":[{"type":"boolean"},{"enum":[true,123456789]}]}`,
			derivative: `123456789`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var cases []Case

			report, err := Build(Input{
				OpenAPI:     []byte(documentWithJSONSchema(test.schema)),
				OperationID: "selected", MaxSteps: 1_000_000,
			}, func(testCase Case) error {
				cases = append(cases, testCase)

				return nil
			})
			require.NoError(t, err)
			require.Contains(t, cases, Case{JSON: []byte(test.derivative), Valid: false})
			require.Equal(t, SpaceExhausted, report.Stop)
			require.Positive(t, report.Steps)
			require.Contains(t, report.Covered,
				"#/paths/~1/post/requestBody/content/application~1json/schema/allOf/0|#|type|fault:type")
		})
	}
}

func TestAdditionalPropertyFaultSkipsNamesDeclaredByTheTargetOccurrence(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"object",
		"additionalProperties":{"type":"boolean"},
		"allOf":[{
			"properties":{"__schematest_extra__":{"type":"string"}},
			"additionalProperties":false
		}]
	}`)
	fault := findFaultTarget(t, plan,
		"/allOf/0|#/*|additionalProperties|fault:additionalProperties")
	searchState := &search{model: model, maxSteps: 100_000}
	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	derivative, err := applyFault(parent, fault, searchState)
	require.NoError(t, err)
	require.Equal(t, `{"__schematest_extra___1":false}`, string(marshalFaultTestValue(t, derivative)))
}

func TestAdditionalPropertyFaultUsesOuterDeclarationOnlyAsAValueWitness(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"object",
		"properties":{"outer":{"enum":["needed"]}},
		"additionalProperties":false,
		"allOf":[{"additionalProperties":false}]
	}`)
	fault := findFaultTarget(t, plan,
		"/allOf/0|#/*|additionalProperties|fault:additionalProperties")
	searchState := &search{model: model, maxSteps: 100_000}
	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	derivative, err := applyFault(parent, fault, searchState)
	require.NoError(t, err)
	require.Equal(t, `{"outer":"needed"}`, string(marshalFaultTestValue(t, derivative)))
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

func TestAdditionalPropertyFaultUsesActiveDeclaredPropertySchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		derivative string
	}{
		{
			name: "declared enum",
			schema: `{"type":"object","allOf":[{"additionalProperties":false},` +
				`{"properties":{"__schematest_extra__":{"enum":["needed"]}}}]}`,
			derivative: `{"__schematest_extra__":"needed"}`,
		},
		{
			name: "declared numeric intersection",
			schema: `{"type":"object","allOf":[{"additionalProperties":false},` +
				`{"properties":{"__schematest_extra__":` +
				`{"type":"number","minimum":5,"multipleOf":2}}}]}`,
			derivative: `{"__schematest_extra__":6}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, plan := compositionFaultModel(t, test.schema)
			fault := findFaultTarget(t, plan,
				"/allOf/0|#/*|additionalProperties|fault:additionalProperties")
			searchState := &search{model: model, maxSteps: 100_000}
			parent, found, err := regenerateParent(plan, fault, searchState)
			require.NoError(t, err)
			require.True(t, found)

			derivative, err := applyFault(parent, fault, searchState)
			require.NoError(t, err)
			require.Equal(t, test.derivative, string(marshalFaultTestValue(t, derivative)))
			matches, err := derivativeHasClosure(model, derivative, fault.expected)
			require.NoError(t, err)
			require.True(t, matches)
		})
	}
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
			derivative: `{"__schematest_extra__":"\u0000\u0000"}`,
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

func TestScalarFaultSearchPreservesComposedPropertySiblings(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"object",
		"required":["x"],
		"allOf":[
			{"properties":{"x":{"type":"string","pattern":"^[a-b]$"}}},
			{"properties":{"x":{"pattern":"^[b-c]$"}}}
		]
	}`)
	fault := findFaultTarget(t, plan, "/allOf/0/properties/x|#/x|pattern|fault:pattern")
	searchState := &search{model: model, maxSteps: 100_000}
	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	beforeMutation := searchState.steps
	searchState.maxSteps = beforeMutation + 12
	derivative, err := applyFault(parent, fault, searchState)
	require.ErrorIs(t, err, errMaxSteps)
	require.Nil(t, derivative)
	require.Equal(t, searchState.maxSteps, searchState.steps)

	searchState = &search{model: model, maxSteps: 100_000}
	parent, found, err = regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	beforeMutation = searchState.steps
	derivative, err = applyFault(parent, fault, searchState)
	require.NoError(t, err)
	require.Equal(t, `{"x":"c"}`, string(marshalFaultTestValue(t, derivative)))
	require.Equal(t, uint64(13), searchState.steps-beforeMutation)
}

func TestStringEnumFaultUsesOpenScalarFrontier(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"string",
		"enum":["","a","b","text"]
	}`)
	fault := findFaultTarget(t, plan, "|enum|fault:enum")
	searchState := &search{model: model, maxSteps: 10_000}
	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	derivative, err := applyFault(parent, fault, searchState)
	require.NoError(t, err)
	matches, err := derivativeHasClosure(model, derivative, fault.expected)
	require.NoError(t, err)
	require.True(t, matches)
	require.NotContains(t, []string{`""`, `"a"`, `"b"`, `"text"`}, string(marshalFaultTestValue(t, derivative)))
}

func TestNumericEnumFaultUsesOpenScalarFrontier(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"number",
		"enum":[-1,0,0.5,1,2,3]
	}`)
	fault := findFaultTarget(t, plan, "|enum|fault:enum")
	searchState := &search{model: model, maxSteps: 10_000}
	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)

	derivative, err := applyFault(parent, fault, searchState)
	require.NoError(t, err)
	matches, err := derivativeHasClosure(model, derivative, fault.expected)
	require.NoError(t, err)
	require.True(t, matches)
	require.NotContains(t, []string{"-1", "0", "0.5", "1", "2", "3"}, string(marshalFaultTestValue(t, derivative)))
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

func TestOversizedCountFaultChargesRealFrontierToCutoff(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"array","maxItems":184467440737095516160,"items":{}
	}`)
	fault := findFaultTarget(t, plan, "|maxItems|fault:maxItems")
	searchState := &search{model: model, maxSteps: 25}
	_, found, err := regenerateParent(plan, fault, searchState)
	require.ErrorIs(t, err, errMaxSteps)
	require.False(t, found)
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
	}, identityStrings(result.failureRecords()))
}
