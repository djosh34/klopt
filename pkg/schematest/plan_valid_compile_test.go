package schematest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMakePlanDeclaresEveryAnyOfMaskWithoutWitnessProbing verifies declarative mask compilation.
func TestMakePlanDeclaresEveryAnyOfMaskWithoutWitnessProbing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
	}{
		{
			name: "crossed numeric bounds",
			schema: `{"type":"number","anyOf":[
				{"minimum":10},{"maximum":20}
			]}`,
		},
		{
			name: "identical patterns",
			schema: `{"type":"string","anyOf":[
				{"pattern":"^z+$"},{"pattern":"^z+$"}
			]}`,
		},
		{
			name: "contradictory branch",
			schema: `{"type":"number","anyOf":[
				{"minimum":10,"maximum":0},{"minimum":1}
			]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, err := parseInput(Input{
				OpenAPI: []byte(documentWithJSONSchema(test.schema)), OperationID: "selected",
			})
			require.NoError(t, err)

			plan, err := makePlan(model)
			require.NoError(t, err)

			require.Equal(t, []string{"level:mask:1", "level:mask:2", "level:mask:3"}, anyOfLevelComponents(plan))
		})
	}
}

// TestMakePlanExactTargetsDoNotInheritKindSelectedAnyOfMask verifies target-specific requirements.
func TestBuildReportsReachedAndUncoveredAnyOfMasks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		schema    string
		covered   []string
		uncovered []string
	}{
		{
			name: "crossed bounds",
			schema: `{"type":"number","enum":[21,0,10],"anyOf":[
				{"minimum":10},{"maximum":20}
			]}`,
			covered:   []string{"mask:1"},
			uncovered: []string{"mask:2", "mask:3"},
		},
		{
			name: "identical branches",
			schema: `{"type":"string","anyOf":[
				{"pattern":"^z+$"},{"pattern":"^z+$"}
			]}`,
			uncovered: []string{"mask:1", "mask:2", "mask:3"},
		},
		{
			name: "contradictory branch",
			schema: `{"type":"number","anyOf":[
				{"minimum":10,"maximum":0},{"minimum":1}
			]}`,
			uncovered: []string{"mask:1", "mask:2", "mask:3"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			report, err := Build(Input{
				OpenAPI: []byte(documentWithJSONSchema(test.schema)), OperationID: "selected", MaxSteps: 10_000,
			}, func(Case) error { return nil })
			require.NoError(t, err)
			require.Equal(t, test.covered, anyOfReportMasks(report.Covered))
			require.Equal(t, test.uncovered, anyOfReportMasks(report.Uncovered))
		})
	}
}

// TestScheduledAnyOfRequestsEnterChargedTraversal locks the absence of contradiction preflights.
func TestScheduledAnyOfRequestsEnterChargedTraversal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
	}{
		{name: "identical branches", schema: `{"type":"string","anyOf":[{"pattern":"^z+$"},{"pattern":"^z+$"}]}`},
		{name: "contradictory bounds", schema: `{"type":"number","anyOf":[{"minimum":10,"maximum":0},{"minimum":1}]}`},
		{name: "universal false branch", schema: `{"type":"number","anyOf":[{}, {"maximum":0}]}`},
		{name: "integer implies number", schema: `{"anyOf":[{"type":"integer"},{"type":"number"}]}`},
		{name: "kind incompatible", schema: `{"type":"string","anyOf":[{"type":"number"},{"type":"string"}]}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, err := parseInput(Input{
				OpenAPI: []byte(documentWithJSONSchema(test.schema)), OperationID: "selected",
			})
			require.NoError(t, err)
			plan, err := makePlan(model)
			require.NoError(t, err)

			levels := make([]string, 0, 3)

			for _, request := range plan.validSchedule {
				selected := request.focus < 0

				var target validIntent

				if selected {
					for _, candidate := range request.targets {
						if candidate.expected.rule == oracleRuleAnyOf &&
							candidate.expected.occurrence == model.root.occurrence {
							target = candidate

							break
						}
					}
				} else {
					target = request.targets[request.focus]
					selected = target.expected.rule == oracleRuleAnyOf && target.expected.occurrence == model.root.occurrence
				}

				if !selected || target.expected.rule == "" {
					continue
				}

				levels = append(levels, target.expected.level)

				_, _, searchErr := findTargetRow(plan, request, &search{model: model, maxSteps: 0})
				require.ErrorIs(t, searchErr, errMaxSteps)
			}

			require.Equal(t, []string{"mask:1", "mask:2", "mask:3"}, levels)
		})
	}
}

// anyOfReportMasks extracts root anyOf coverage levels.
func anyOfReportMasks(identities []string) []string {
	var masks []string

	for _, identity := range identities {
		marker := "|anyOf|level:"

		index := strings.Index(identity, marker)
		if index >= 0 {
			masks = append(masks, identity[index+len(marker):])
		}
	}

	return masks
}

// TestBaselineReachesSingletonNestedCanonicalTargets proves complete-vector baseline propagation.
func TestBaselineReachesSingletonNestedCanonicalTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		levels []string
	}{
		{
			name:   "optional exact enum property",
			schema: `{"type":"object","properties":{"value":{"type":"string","enum":["a"]}}}`,
			levels: []string{"#/value|type|level:string", "#/value|enum|level:member:0"},
		},
		{
			name:   "plain optional typed property",
			schema: `{"type":"object","properties":{"value":{"type":"string"}}}`,
			levels: []string{"#/value|type|level:string"},
		},
		{
			name:   "plain optional typeless property",
			schema: `{"type":"object","properties":{"value":{}}}`,
			levels: []string{"#/value|type|level:boolean"},
		},
		{
			name: "two compatible singleton properties",
			schema: `{"type":"object","properties":{
				"a":{"type":"string"},"b":{"type":"number"}
			}}`,
			levels: []string{"#/a|type|level:string", "#/b|type|level:number"},
		},
		{
			name:   "singleton array item",
			schema: `{"type":"array","items":{"type":"boolean"}}`,
			levels: []string{"#/*|type|level:boolean"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			report, err := Build(Input{
				OpenAPI: []byte(documentWithJSONSchema(test.schema)), OperationID: "selected", MaxSteps: 10_000,
			}, func(Case) error { return nil })
			require.NoError(t, err)

			for _, suffix := range test.levels {
				require.Truef(t, reportIdentityHasSuffix(report.Covered, suffix), "%s: %#v", suffix, report)
			}
		})
	}
}

// TestBaselineSchedulesCanonicalAdditionalPropertyWildcard locks the singleton additive request.
func TestBaselineSchedulesCanonicalAdditionalPropertyWildcard(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{"type":"object","additionalProperties":{"type":"boolean"}}`))
	model, err := parseInput(Input{OpenAPI: document, OperationID: "selected"})
	require.NoError(t, err)
	plan, err := makePlan(model)
	require.NoError(t, err)
	require.Len(t, plan.validSchedule, 1)

	for _, requirement := range plan.validSchedule[0].requirements {
		require.NotContains(t, requirement.occurrence.instanceTemplate, "__schematest_extra__")
	}

	cases := make([]Case, 0, 1)
	report, err := Build(Input{
		OpenAPI: document, OperationID: "selected", MaxSteps: 10_000,
	}, func(testCase Case) error {
		cases = append(cases, testCase)

		return nil
	})
	require.NoError(t, err)
	require.True(t, reportIdentityHasSuffix(report.Covered, "#/*|type|level:boolean"))

	validCases := validCasesOnly(cases)
	require.Len(t, validCases, 1)

	value, err := parseStrictJSON(validCases[0].JSON)
	require.NoError(t, err)
	require.Equal(t, jsonObject, value.kind)
	require.Len(t, value.object, 1)

	for _, member := range value.object {
		require.Equal(t, jsonBoolean, member.kind)
	}
}

// TestBaselineDoesNotProvePropertyMaximumConflict locks charged runtime ownership.
func TestBaselineDoesNotProvePropertyMaximumConflict(t *testing.T) {
	t.Parallel()

	report, err := Build(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"object","maxProperties":1,"required":["id"],
			"properties":{"id":{"type":"string"},"value":{"type":"number"}}
		}`)),
		OperationID: "selected",
		MaxSteps:    10_000,
	}, func(Case) error { return nil })
	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Positive(t, report.Steps)
	require.True(t, reportIdentityHasSuffix(report.Uncovered, "#/id|type|level:string"))
	require.True(t, reportIdentityHasSuffix(report.Uncovered, "#/value|type|level:number"))
}

// TestBuildDirectsCanonicalAndReplacementAnyOfMasks locks executable selected vectors.
func TestBuildDirectsCanonicalAndReplacementAnyOfMasks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		schema    string
		covered   []string
		uncovered []string
	}{
		{
			name: "overlapping patterns",
			schema: `{"type":"string","anyOf":[
				{"pattern":"^a+$"},{"pattern":"^aa$"}
			]}`,
			covered:   []string{"mask:1", "mask:3"},
			uncovered: []string{"mask:2"},
		},
		{
			name:      "unconstrained and maximum",
			schema:    `{"type":"number","anyOf":[{}, {"maximum":0}]}`,
			covered:   []string{"mask:1"},
			uncovered: []string{"mask:2", "mask:3"},
		},
		{
			name:      "integer and number",
			schema:    `{"anyOf":[{"type":"integer"},{"type":"number"}]}`,
			covered:   []string{"mask:2", "mask:3"},
			uncovered: []string{"mask:1"},
		},
		{
			name: "disjoint enum branches",
			schema: `{"type":"number","enum":[5,7],"anyOf":[
				{"enum":[5]},{"enum":[7]}
			]}`,
			covered:   []string{"mask:1"},
			uncovered: []string{"mask:2", "mask:3"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			report, err := Build(Input{
				OpenAPI: []byte(documentWithJSONSchema(test.schema)), OperationID: "selected", MaxSteps: 10_000,
			}, func(Case) error { return nil })
			require.NoError(t, err)
			require.Equal(t, test.covered, anyOfReportMasks(report.Covered))
			require.Equal(t, test.uncovered, anyOfReportMasks(report.Uncovered))
		})
	}
}

// reportIdentityHasSuffix reports whether coverage contains one focused identity fragment.
func reportIdentityHasSuffix(identities []string, suffix string) bool {
	for _, identity := range identities {
		if strings.Contains(identity, suffix) {
			return true
		}
	}

	return false
}

// TestMakePlanExactTargetsDoNotInheritKindSelectedAnyOfMask locks target-specific requirements.
func TestMakePlanExactTargetsDoNotInheritKindSelectedAnyOfMask(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object",
		"enum":[{"value":"a"}],
		"properties":{"value":{"type":"string","enum":["a"]}},
		"anyOf":[
			{"properties":{"value":{"enum":["a"]}}},
			{"properties":{"value":{"enum":["b"]}}}
		]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	rootEnum := findValidIntent(t, plan, model.root.occurrence, oracleRuleEnum, "member:0")
	requireExactEnumRequirement(t, rootEnum.requirements, &model.root.enum[0])
	requireNoCompositionRequirements(t, rootEnum.requirements, model.root.occurrence, "anyOf")

	child := model.root.properties["value"]
	childEnum := findValidIntent(t, plan, child.occurrence, oracleRuleEnum, "member:0")
	requireExactEnumRequirement(t, childEnum.requirements, &child.enum[0])
	requireRequirement(t, childEnum.requirements, "#/value", requirementPresent)
	requireNoCompositionRequirements(t, childEnum.requirements, model.root.occurrence, "anyOf")
}

// findValidIntent returns one exact compiled valid target.
func findValidIntent(
	t *testing.T,
	plan *searchPlan,
	occurrence schemaOccurrence,
	rule string,
	level string,
) validIntent {
	t.Helper()

	for _, intent := range plan.validCatalog {
		if intent.expected.occurrence == occurrence && intent.expected.rule == rule && intent.expected.level == level {
			return intent
		}
	}

	t.Fatalf("valid intent not found for %s|%s|%s", occurrence.usePointer, rule, level)

	return validIntent{}
}

// anyOfLevelComponents returns compiled anyOf levels in schedule order.
func anyOfLevelComponents(plan *searchPlan) []string {
	levels := make([]string, 0)

	for _, intent := range plan.validCatalog {
		if intent.obligation.rule == oracleRuleAnyOf {
			levels = append(levels, intent.obligation.component)
		}
	}

	return levels
}

// requireExactEnumRequirement checks an authored member constraint.
func requireExactEnumRequirement(t *testing.T, requirements []requirement, member *enumMember) {
	t.Helper()

	for _, requirement := range requirements {
		if requirement.tag == requirementExactEnumMember && requirement.enumMember == member {
			return
		}
	}

	t.Fatalf("exact enum member requirement not found: %#v", requirements)
}

// requireNoCompositionRequirements rejects kind-selected composition truth constraints.
func requireNoCompositionRequirements(
	t *testing.T,
	requirements []requirement,
	occurrence schemaOccurrence,
	composition string,
) {
	t.Helper()

	for _, requirement := range requirements {
		if requirement.tag == requirementBranchTruth && requirement.composition == composition &&
			requirement.occurrence.usePointer == occurrence.usePointer {
			t.Fatalf("unexpected %s requirement for %s: %#v", composition, occurrence.usePointer, requirements)
		}
	}
}
