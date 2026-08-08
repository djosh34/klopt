package schematest

import (
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
	requirePin(t, childEnum.requirements, "#/value", requirementPresent)
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
