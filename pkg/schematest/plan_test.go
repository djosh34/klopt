//nolint:godoclint // Canonical planner tables requirement private identity vocabulary at its seam.
package schematest

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompareSchemaOccurrencesUsesCanonicalPointerTokenOrder(t *testing.T) {
	t.Parallel()

	occurrences := []schemaOccurrence{
		{usePointer: "#/properties/2", targetPointer: "#/components/schemas/Value", instanceTemplate: "#/2"},
		{usePointer: "#/properties/10", targetPointer: "#/components/schemas/Value", instanceTemplate: "#/10"},
		{usePointer: "#/allOf/10", targetPointer: "#/components/schemas/Value", instanceTemplate: "#"},
		{usePointer: "#/allOf/2", targetPointer: "#/components/schemas/Value", instanceTemplate: "#"},
		{usePointer: "#/properties/a~0b", targetPointer: "#/components/schemas/Value", instanceTemplate: "#"},
		{usePointer: "#/properties/a~1b", targetPointer: "#/components/schemas/Value", instanceTemplate: "#"},
	}

	sort.SliceStable(occurrences, func(left, right int) bool {
		comparison, err := compareSchemaOccurrences(occurrences[left], occurrences[right])
		require.NoError(t, err)

		return comparison < 0
	})

	require.Equal(t, []string{
		"#/allOf/2",
		"#/allOf/10",
		"#/properties/10",
		"#/properties/2",
		"#/properties/a~1b",
		"#/properties/a~0b",
	}, occurrenceUsePointers(occurrences))
}

func TestCompareSchemaOccurrencesUsesTargetThenInstanceAndRetainsReferenceIdentity(t *testing.T) {
	t.Parallel()

	occurrences := []schemaOccurrence{
		{usePointer: "#/use", targetPointer: "#/components/schemas/Z", instanceTemplate: "#/b", reference: true},
		{usePointer: "#/use", targetPointer: "#/components/schemas/A", instanceTemplate: "#/z", reference: true},
		{usePointer: "#/use", targetPointer: "#/components/schemas/A", instanceTemplate: "#/a", reference: true},
		{usePointer: "#/use", targetPointer: "#/components/schemas/A", instanceTemplate: "#/a"},
	}

	sort.SliceStable(occurrences, func(left, right int) bool {
		comparison, err := compareSchemaOccurrences(occurrences[left], occurrences[right])
		require.NoError(t, err)

		return comparison < 0
	})

	require.Equal(t, []schemaOccurrence{
		{usePointer: "#/use", targetPointer: "#/components/schemas/A", instanceTemplate: "#/a"},
		{usePointer: "#/use", targetPointer: "#/components/schemas/A", instanceTemplate: "#/a", reference: true},
		{usePointer: "#/use", targetPointer: "#/components/schemas/A", instanceTemplate: "#/z", reference: true},
		{usePointer: "#/use", targetPointer: "#/components/schemas/Z", instanceTemplate: "#/b", reference: true},
	}, occurrences)
}

func TestMakePlanCanonicalizesPropertyAndEnumObligations(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object",
		"required":["z","a"],
		"properties":{
			"2":{"type":"string"},
			"10":{"type":"string"},
			"a/b":{"type":"string","enum":["first","second"]},
			"a~b":{"type":"string"}
		},
		"additionalProperties":false
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	require.Equal(t, []string{
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:object",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|fault:type",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#/*|additionalProperties|fault:additionalProperties",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#/a|required|level:present",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#/a|required|fault:required",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#/z|required|level:present",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#/z|required|fault:required",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/10|#/10|type|level:string",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/10|#/10|type|fault:type",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/2|#/2|type|level:string",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/2|#/2|type|fault:type",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a~1b|#/a~1b|type|level:string",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a~1b|#/a~1b|type|fault:type",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a~1b|#/a~1b|enum|level:member:0",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a~1b|#/a~1b|enum|level:member:1",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a~1b|#/a~1b|enum|fault:enum",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a~0b|#/a~0b|type|level:string",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a~0b|#/a~0b|type|fault:type",
	}, plan.obligationIDs())
}

func TestMakePlanRequirementsCanonicalKindsAndObjectPresence(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object",
		"required":["required"],
		"properties":{
			"required":{"type":"string"},
			"optional":{"type":"number"}
		}
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	rootType := findValidTarget(t, plan, "|type|level:object")
	requireRequirement(t, rootType.requirements, "#/required", requirementPresent)
	requireRequirement(t, rootType.requirements, "#/optional", requirementAbsent)

	optionalType := findValidTarget(t, plan, "/properties/optional|#/optional|type|level:number")
	requireRequirement(t, optionalType.requirements, "#/optional", requirementPresent)
	requireKindRequirement(t, optionalType.requirements, optionalType.obligation.occurrence, jsonNumber)

	requiredFault := findFaultTarget(t, plan, "|#/required|required|fault:required")
	requireOnlyPresenceRequirement(t, requiredFault.requirements, "#/required", requirementAbsent)
}

func TestMakePlanFaultsInvertCompositionBranchTruth(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[
			{"type":"string","pattern":"^a"},
			{"type":"number","minimum":2}
		],
		"allOf":[
			{"type":"string","pattern":"^a"},
			{"type":"number","minimum":2}
		]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	allOfFault := findFaultTarget(t, plan, "/allOf/0|#|pattern|fault:pattern")
	requireCompositionRequirement(t, allOfFault.requirements, "allOf", 0, false)
	requireCompositionRequirement(t, allOfFault.requirements, "allOf", 1, true)

	anyOfFault := findFaultTarget(t, plan, "/anyOf/0|#|pattern|fault:pattern")
	requireCompositionRequirement(t, anyOfFault.requirements, "anyOf", 0, false)
	requireCompositionRequirement(t, anyOfFault.requirements, "anyOf", 1, false)
}

func TestMakePlanKeepsTypelessSiblingCompatibleKindFirst(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"enum":[true,"text"]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	ids := make([]string, 0)

	for _, target := range plan.validCatalog {
		if target.obligation.rule == oracleRuleType {
			ids = append(ids, target.obligation.String())
		}
	}

	require.Equal(t, []string{
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:boolean",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:null",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:number",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:string",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:array",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:object",
	}, ids)
}

func TestMakePlanTypelessNullEnumKeepsNonNullKindFirst(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"enum":[null,true]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	ids := make([]string, 0)

	for _, target := range plan.validCatalog {
		if target.obligation.rule == oracleRuleType {
			ids = append(ids, target.obligation.String())
		}
	}

	require.Equal(t, []string{
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:boolean",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:null",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:number",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:string",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:array",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|type|level:object",
	}, ids)
}

func TestMakePlanEnumeratesBooleanAnyOfMasks(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"boolean",
		"anyOf":[{"enum":[true]}, {"enum":[false]}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	findValidTarget(t, plan, "|anyOf|level:mask:1")
	findValidTarget(t, plan, "|anyOf|level:mask:2")
}

func TestMakePlanEnumeratesEveryDistinctStringAnyOfMask(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{"enum":["a"]}, {"enum":["b"]}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	masks := make([]string, 0)

	for _, target := range plan.validCatalog {
		if target.obligation.rule == oracleRuleAnyOf {
			masks = append(masks, target.obligation.component)
		}
	}

	require.Equal(t, []string{"level:mask:1", "level:mask:2", "level:mask:3"}, masks)
}

func TestMakePlanParentEnumRetainsEveryBooleanAnyOfMask(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"boolean",
		"enum":[true],
		"anyOf":[{"enum":[false]}, {"enum":[true]}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	require.Equal(t, []string{"level:mask:1", "level:mask:2", "level:mask:3"}, anyOfLevelComponents(plan))
}

func TestMakePlanDerivesStringAnyOfWitnessFromMinLength(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"anyOf":[{"minLength":5}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	findValidTarget(t, plan, "|anyOf|level:mask:1")
}

func TestMakePlanRetainsMasksInapplicableToExplicitType(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"anyOf":[{"enum":[true]}, {"enum":[false]}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	require.Equal(t, []string{"level:mask:1", "level:mask:2", "level:mask:3"}, anyOfLevelComponents(plan))
}

func TestMakePlanRetainsUnreachableGenericBooleanMasks(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{"enum":[false,true]}, {"enum":[false,true]}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	require.Equal(t, []string{"level:mask:1", "level:mask:2", "level:mask:3"}, anyOfLevelComponents(plan))
}

func TestMakePlanSemanticEnumDedupeKeepsFirstAuthoredMembers(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"enum":[1,1.0,{"a":1,"b":2},{"b":2,"a":1},"kept"]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	ids := make([]string, 0)

	for _, target := range plan.validCatalog {
		if target.obligation.rule == oracleRuleEnum {
			ids = append(ids, target.obligation.String())
		}
	}

	require.Equal(t, []string{
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|enum|level:member:0",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|enum|level:member:2",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|enum|level:member:4",
	}, ids)

	for _, test := range []struct {
		value string
		level string
	}{
		{value: `1`, level: "member:0"},
		{value: `{"b":2,"a":1}`, level: "member:2"},
		{value: `"kept"`, level: "member:4"},
	} {
		value, parseErr := parseStrictJSON([]byte(test.value))
		require.NoError(t, parseErr)

		result := evaluate(model, value)
		require.NoError(t, result.err)

		planTarget := findValidTarget(t, plan, "|enum|level:"+test.level)
		require.Contains(t, levelIdentityStrings(result.observedRecords()), planTarget.expected.String())
	}
}

func TestMakePlanAcceptsSchemaNamesThatMatchCompositionKeywords(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object",
		"properties":{"allOf":{"type":"object","properties":{"child":{"type":"string"}}}}
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	_, err = makePlan(model)
	require.NoError(t, err)
}

func TestMakePlanScalarFaultsConstrainTheirLocalKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		rule   string
		kind   jsonKind
	}{
		{name: "string", schema: `{"minLength":1}`, rule: oracleRuleMinLength, kind: jsonString},
		{name: "array", schema: `{"minItems":1}`, rule: oracleRuleMinItems, kind: jsonArray},
		{name: "object", schema: `{"minProperties":1}`, rule: oracleRuleMinProperties, kind: jsonObject},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(test.schema)), OperationID: "selected"})
			require.NoError(t, err)

			plan, err := makePlan(model)
			require.NoError(t, err)

			fault := findFaultTarget(t, plan, "|"+test.rule+"|fault:"+test.rule)
			requireKindRequirement(t, fault.requirements, fault.obligation.occurrence, test.kind)
		})
	}
}

func TestMakePlanRequiredPresenceRequirementsContainmentAndUndeclaredNames(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object",
		"required":["missing"]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	required := findValidTarget(t, plan, "|#/missing|required|level:present")
	requireKindRequirement(t, required.requirements, model.root.occurrence, jsonObject)
	requireRequirement(t, required.requirements, "#/missing", requirementPresent)
}

func TestMakePlanTypelessRequiredPresenceOnlyAppliesToObjects(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"required":["name"]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	for _, target := range plan.validCatalog {
		if target.obligation.rule != oracleRuleType {
			continue
		}

		if target.obligation.component == oracleLevelPrefix+jsonKindName(jsonObject) {
			requireRequirement(t, target.requirements, "#/name", requirementPresent)

			continue
		}

		requireNoPresenceRequirement(t, target.requirements, "#/name")
	}
}

func TestMakePlanAnyOfLocalMinimumLeavesSiblingTruthToSearch(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"minimum":1,
		"anyOf":[{"type":"string"}, {"type":"number"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	requirements := findValidTarget(t, plan, "|minimum|level:valid").requirements
	requireNoCompositionRequirement(t, requirements, "anyOf", 0)
	requireNoCompositionRequirement(t, requirements, "anyOf", 1)
}

func TestMakePlanAnyOfOverlappingChildValidTargetsKeepSiblingsUnconstrained(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{"type":"string"}, {"type":"string"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	for index := 0; index < 2; index++ {
		target := findValidTarget(t, plan, "/anyOf/"+itoa(index)+"|#|type|level:string")
		requireCompositionRequirement(t, target.requirements, "anyOf", index, true)

		for sibling := 0; sibling < 2; sibling++ {
			if sibling == index {
				continue
			}

			requireNoCompositionRequirement(t, target.requirements, "anyOf", sibling)
		}
	}
}

func TestMakePlanPositiveMinItemsSuppliesItsItem(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array",
		"minItems":1,
		"items":{"type":"string"}
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	for _, target := range []validIntent{
		findValidTarget(t, plan, "|type|level:array"),
		findValidTarget(t, plan, "|minItems|level:valid"),
	} {
		requireRequirement(t, target.requirements, "#/*", requirementPresent)
		requireNoPresenceRequirement(t, target.requirements, "#/*", requirementAbsent)
	}
}

func TestMakePlanOmitsZeroLowerBoundFaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		rule   string
	}{
		{name: "minLength", schema: `{"type":"string","minLength":0}`, rule: oracleRuleMinLength},
		{name: "minItems", schema: `{"type":"array","minItems":0,"items":{"type":"string"}}`, rule: oracleRuleMinItems},
		{name: "minProperties", schema: `{"type":"object","minProperties":0}`, rule: oracleRuleMinProperties},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(test.schema)), OperationID: "selected"})
			require.NoError(t, err)

			plan, err := makePlan(model)
			require.NoError(t, err)

			for _, target := range plan.faultSchedule {
				require.NotEqual(t, test.rule, target.obligation.rule)
			}
		})
	}
}

func TestMakePlanEnumeratesEveryIntegerNumberAnyOfMask(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{"type":"integer"},{"type":"number"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	require.Equal(t, []string{"level:mask:1", "level:mask:2", "level:mask:3"}, anyOfLevelComponents(plan))
}

func TestMakePlanPositiveMinPropertiesRequirementsDeclaredMemberPresent(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object",
		"minProperties":1,
		"properties":{"x":{"type":"string"}},
		"additionalProperties":false
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	for _, target := range []validIntent{
		findValidTarget(t, plan, "|type|level:object"),
		findValidTarget(t, plan, "|minProperties|level:valid"),
	} {
		requireRequirement(t, target.requirements, "#/x", requirementPresent)
		requireNoPresenceRequirement(t, target.requirements, "#/x", requirementAbsent)
	}
}

func findValidTarget(t *testing.T, plan *searchPlan, suffix string) validIntent {
	t.Helper()

	for _, target := range plan.validCatalog {
		if strings.HasSuffix(target.obligation.String(), suffix) {
			return target
		}
	}

	t.Fatalf("valid target with suffix %q not found", suffix)

	return validIntent{}
}

func findFaultTarget(t *testing.T, plan *searchPlan, suffix string) faultProgram {
	t.Helper()

	for _, target := range plan.faultSchedule {
		if strings.HasSuffix(target.obligation.String(), suffix) {
			return target
		}
	}

	t.Fatalf("fault target with suffix %q not found", suffix)

	return faultProgram{}
}

func requireRequirement(
	t *testing.T,
	requirements []requirement,
	instanceTemplate string,
	presence requirementPresence,
) {
	t.Helper()

	for _, requirement := range requirements {
		if requirement.occurrence.instanceTemplate == instanceTemplate && requirement.presence == presence {
			return
		}
	}

	t.Fatalf(
		"requirement for instance template %q with presence %d not found: %#v",
		instanceTemplate, presence, requirements,
	)
}

func requireOnlyPresenceRequirement(
	t *testing.T,
	requirements []requirement,
	instanceTemplate string,
	presence requirementPresence,
) {
	t.Helper()

	count := 0

	for _, requirement := range requirements {
		if requirement.occurrence.instanceTemplate == instanceTemplate &&
			requirement.presence != requirementNoPresence {
			count++

			require.Equal(t, presence, requirement.presence)
		}
	}

	require.Equal(t, 1, count, "presence requirement for %s", instanceTemplate)
}

func requireNoPresenceRequirement(
	t *testing.T,
	requirements []requirement,
	instanceTemplate string,
	forbidden ...requirementPresence,
) {
	t.Helper()

	for _, requirement := range requirements {
		if requirement.occurrence.instanceTemplate != instanceTemplate ||
			requirement.presence == requirementNoPresence {
			continue
		}

		if len(forbidden) == 0 || requirement.presence == forbidden[0] {
			t.Fatalf("unexpected presence requirement for %q: %#v", instanceTemplate, requirements)
		}
	}
}

func requireKindRequirement(
	t *testing.T,
	requirements []requirement,
	occurrence schemaOccurrence,
	kind jsonKind,
) {
	t.Helper()

	for _, requirement := range requirements {
		if requirement.hasKind && requirement.occurrence == occurrence && requirement.kind == kind {
			return
		}
	}

	t.Fatalf(
		"kind requirement for %s with kind %s not found: %#v",
		occurrence.usePointer, jsonKindName(kind), requirements,
	)
}

func requireNoCompositionRequirement(
	t *testing.T,
	requirements []requirement,
	composition string,
	branch int,
) {
	t.Helper()

	for _, requirement := range requirements {
		if requirement.hasBranch && requirement.composition == composition && requirement.branch == branch {
			t.Fatalf("unexpected composition requirement %s[%d]: %#v", composition, branch, requirements)
		}
	}
}

func requireCompositionRequirement(
	t *testing.T,
	requirements []requirement,
	composition string,
	branch int,
	truth bool,
) {
	t.Helper()

	for _, requirement := range requirements {
		if requirement.composition == composition && requirement.branch == branch &&
			requirement.hasBranch && requirement.truth == truth {
			return
		}
	}

	t.Fatalf("composition requirement %s[%d]=%t not found: %#v", composition, branch, truth, requirements)
}

func occurrenceUsePointers(occurrences []schemaOccurrence) []string {
	result := make([]string, 0, len(occurrences))
	for _, occurrence := range occurrences {
		result = append(result, occurrence.usePointer)
	}

	return result
}
