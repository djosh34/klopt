//nolint:godoclint // Canonical planner tables pin private identity vocabulary at its seam.
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
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a~1b|#/a~1b|enum|level:member:0",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a~1b|#/a~1b|enum|level:member:1",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a~1b|#/a~1b|enum|fault:enum",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a~0b|#/a~0b|type|level:string",
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a~0b|#/a~0b|type|fault:type",
	}, plan.obligationIDs())
}

func TestMakePlanPinsCanonicalKindsAndObjectPresence(t *testing.T) {
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
	requirePin(t, rootType.pins, "#/required", planPinPresent)
	requirePin(t, rootType.pins, "#/optional", planPinAbsent)

	optionalType := findValidTarget(t, plan, "/properties/optional|#/optional|type|level:number")
	requirePin(t, optionalType.pins, "#/optional", planPinPresent)
	requireKindPin(t, optionalType.pins, optionalType.obligation.occurrence, jsonNumber)

	requiredFault := findFaultTarget(t, plan, "|#/required|required|fault:required")
	requireOnlyPresencePin(t, requiredFault.pins, "#/required", planPinAbsent)
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
	requireCompositionPin(t, allOfFault.pins, "allOf", 0, false)
	requireCompositionPin(t, allOfFault.pins, "allOf", 1, true)

	anyOfFault := findFaultTarget(t, plan, "/anyOf/0|#|pattern|fault:pattern")
	requireCompositionPin(t, anyOfFault.pins, "anyOf", 0, false)
	requireCompositionPin(t, anyOfFault.pins, "anyOf", 1, false)
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

	for _, target := range plan.validTargets {
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

func TestMakePlanOmitsMasksInapplicableToExplicitType(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"anyOf":[{"enum":[true]}, {"enum":[false]}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	for _, target := range plan.validTargets {
		require.NotEqual(t, oracleRuleAnyOf, target.obligation.rule)
	}
}

func TestMakePlanOmitsUnreachableGenericBooleanMasks(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{"enum":[false,true]}, {"enum":[false,true]}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	findValidTarget(t, plan, "|anyOf|level:mask:3")

	for _, target := range plan.validTargets {
		if target.obligation.rule == oracleRuleAnyOf {
			require.NotContains(t, target.obligation.component, "mask:1")
		}
	}
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

	for _, target := range plan.validTargets {
		if target.obligation.rule == oracleRuleEnum {
			ids = append(ids, target.obligation.String())
		}
	}

	require.Equal(t, []string{
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|enum|level:member:0",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|enum|level:member:1",
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|enum|level:member:2",
	}, ids)
}

func TestMakePlanCanonicalizesRuleLevelsAndAnyOfClosure(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"string",
		"minLength":1,
		"maxLength":4,
		"pattern":"^a",
		"format":"date"
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	root := "#/paths/~1/post/requestBody/content/application~1json/schema|#|"
	require.Equal(t, []string{
		root + "type|level:string",
		root + "type|fault:type",
		root + "minLength|level:valid",
		root + "minLength|fault:minLength",
		root + "maxLength|level:valid",
		root + "maxLength|fault:maxLength",
		root + "pattern|level:valid",
		root + "pattern|fault:pattern",
		root + "format|level:valid",
		root + "format|fault:format",
	}, plan.obligationIDs())

	anyOfModel, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{"type":"string"},{"type":"number"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	anyOfPlan, err := makePlan(anyOfModel)
	require.NoError(t, err)

	rootType := findValidTarget(t, anyOfPlan, "|type|level:number")
	requireCompositionPin(t, rootType.pins, "anyOf", 0, false)
	requireCompositionPin(t, rootType.pins, "anyOf", 1, true)

	aggregate := findFaultTarget(t, anyOfPlan, "|anyOf|fault:anyOf")
	anyOfRoot := "#/paths/~1/post/requestBody/content/application~1json/schema"
	require.Equal(t, []string{
		anyOfRoot + "|#|anyOf",
		anyOfRoot + "/anyOf/0|#|type",
		anyOfRoot + "/anyOf/1|#|type",
	}, identityStrings(aggregate.closure))
	requireCompositionPin(t, aggregate.pins, "anyOf", 0, false)
	requireCompositionPin(t, aggregate.pins, "anyOf", 1, false)
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

func TestMakePlanScalarFaultsPinTheirLocalKinds(t *testing.T) {
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
			requireKindPin(t, fault.pins, fault.obligation.occurrence, test.kind)
		})
	}
}

func TestMakePlanRequiredPresencePinsContainmentAndUndeclaredNames(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object",
		"required":["missing"]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	required := findValidTarget(t, plan, "|#/missing|required|level:present")
	requireKindPin(t, required.pins, model.root.occurrence, jsonObject)
	requirePin(t, required.pins, "#/missing", planPinPresent)
}

func TestMakePlanTypelessRequiredPresenceOnlyAppliesToObjects(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"required":["name"]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	for _, target := range plan.validTargets {
		if target.obligation.rule != oracleRuleType {
			continue
		}

		if target.obligation.component == oracleLevelPrefix+jsonKindName(jsonObject) {
			requirePin(t, target.pins, "#/name", planPinPresent)

			continue
		}

		requireNoPresencePin(t, target.pins, "#/name")
	}
}

func TestMakePlanOmitsBareAnyOfFaultClosures(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{}, {"type":"string"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	for _, target := range plan.faultTargets {
		require.NotEqual(t, oracleRuleAnyOf, target.obligation.rule)
		require.NotContains(t, target.obligation.occurrence.usePointer, "/anyOf/")
	}
}

func TestMakePlanAnyOfStringEnumUsesReachableAggregateRepresentative(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{"type":"string","enum":["a"]}, {"type":"number"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	findFaultTarget(t, plan, "/anyOf/0|#|enum|fault:enum")

	for _, target := range plan.faultTargets {
		require.NotContains(t, target.obligation.String(), "/anyOf/0|#|type|fault:type")
	}

	aggregate := findFaultTarget(t, plan, "|anyOf|fault:anyOf")
	require.Equal(t, []string{
		model.root.occurrence.usePointer + "|#|anyOf",
		model.root.occurrence.usePointer + "/anyOf/0|#|enum",
		model.root.occurrence.usePointer + "/anyOf/1|#|type",
	}, identityStrings(aggregate.closure))
}

func TestMakePlanAnyOfLocalMinimumUsesCompatibleNumericBranch(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"minimum":1,
		"anyOf":[{"type":"string"}, {"type":"number"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	for _, target := range []struct {
		name string
		want string
	}{
		{name: "valid", want: "|minimum|level:valid"},
		{name: "fault", want: "|minimum|fault:minimum"},
	} {
		var pins []applicabilityPin
		if target.name == "valid" {
			pins = findValidTarget(t, plan, target.want).pins
		} else {
			pins = findFaultTarget(t, plan, target.want).pins
		}

		requireCompositionPin(t, pins, "anyOf", 0, false)
		requireCompositionPin(t, pins, "anyOf", 1, true)
	}
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
		requireCompositionPin(t, target.pins, "anyOf", index, true)

		for sibling := 0; sibling < 2; sibling++ {
			if sibling == index {
				continue
			}

			requireNoCompositionPin(t, target.pins, "anyOf", sibling)
		}
	}
}

func TestMakePlanAnyOfPatternFaultClosesEveryBranch(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{"type":"string","pattern":"^a"}, {"type":"number"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	pattern := findFaultTarget(t, plan, "/anyOf/0|#|pattern|fault:pattern")
	requireCompositionPin(t, pattern.pins, "anyOf", 0, false)
	requireCompositionPin(t, pattern.pins, "anyOf", 1, false)
	require.Equal(t, []string{
		model.root.occurrence.usePointer + "|#|anyOf",
		model.root.occurrence.usePointer + "/anyOf/0|#|pattern",
		model.root.occurrence.usePointer + "/anyOf/1|#|type",
	}, identityStrings(pattern.closure))
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

	for _, target := range []validTarget{
		findValidTarget(t, plan, "|type|level:array"),
		findValidTarget(t, plan, "|minItems|level:valid"),
	} {
		requirePin(t, target.pins, "#/*", planPinPresent)
		requireNoPresencePin(t, target.pins, "#/*", planPinAbsent)
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

			for _, target := range plan.faultTargets {
				require.NotEqual(t, test.rule, target.obligation.rule)
			}
		})
	}
}

func TestMakePlanOmitsExhaustiveNonNullableBooleanEnumFault(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"boolean",
		"enum":[true,false]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	for _, target := range plan.faultTargets {
		require.NotEqual(t, oracleRuleEnum, target.obligation.rule)
	}
}

func TestPlanComparisonErrorsPropagate(t *testing.T) {
	t.Parallel()

	invalidOccurrence := schemaOccurrence{
		usePointer:       "not-a-pointer",
		targetPointer:    "#",
		instanceTemplate: "#",
	}
	validOccurrence := schemaOccurrence{
		usePointer:       "#/valid",
		targetPointer:    "#",
		instanceTemplate: "#",
	}
	invalidIdentity := makeRuleIdentity(invalidOccurrence, oracleRuleType)
	validIdentity := makeRuleIdentity(validOccurrence, oracleRuleType)
	faults := []faultTarget{
		{obligation: makeFaultObligation(invalidIdentity, oracleRuleType)},
		{obligation: makeFaultObligation(validIdentity, oracleRuleType)},
	}

	_, _, err := firstCanonicalFault(faults)
	require.Error(t, err)

	_, err = canonicalFailureClosure([]failureIdentity{invalidIdentity, validIdentity})
	require.Error(t, err)

	_, _, err = firstRealizableFault(
		&schemaNode{schemaShape: &schemaShape{}}, invalidOccurrence, faults,
	)
	require.Error(t, err)
}

func TestMakePlanOmitsImpossibleTypedEnumTypeFault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		schema    string
		typeFault bool
	}{
		{name: "exhaustive string enum", schema: `{"type":"string","enum":["a"]}`, typeFault: false},
		{name: "wrong-kind enum witness", schema: `{"type":"string","enum":["a",1]}`, typeFault: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(test.schema)), OperationID: "selected"})
			require.NoError(t, err)

			plan, err := makePlan(model)
			require.NoError(t, err)

			found := false

			for _, target := range plan.faultTargets {
				if strings.HasSuffix(target.obligation.String(), "|type|fault:type") {
					found = true

					break
				}
			}

			require.Equal(t, test.typeFault, found)
		})
	}
}

func TestMakePlanEnumeratesReachableIntegerNumberAnyOfMasks(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{"type":"integer"},{"type":"number"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	findValidTarget(t, plan, "|anyOf|level:mask:2")
	findValidTarget(t, plan, "|anyOf|level:mask:3")

	for _, target := range plan.validTargets {
		require.NotContains(t, target.obligation.String(), "|anyOf|level:mask:1")
	}
}

func TestMakePlanPositiveMinPropertiesPinsDeclaredMemberPresent(t *testing.T) {
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

	for _, target := range []validTarget{
		findValidTarget(t, plan, "|type|level:object"),
		findValidTarget(t, plan, "|minProperties|level:valid"),
	} {
		requirePin(t, target.pins, "#/x", planPinPresent)
		requireNoPresencePin(t, target.pins, "#/x", planPinAbsent)
	}
}

func findValidTarget(t *testing.T, plan *searchPlan, suffix string) validTarget {
	t.Helper()

	for _, target := range plan.validTargets {
		if strings.HasSuffix(target.obligation.String(), suffix) {
			return target
		}
	}

	t.Fatalf("valid target with suffix %q not found", suffix)

	return validTarget{}
}

func findFaultTarget(t *testing.T, plan *searchPlan, suffix string) faultTarget {
	t.Helper()

	for _, target := range plan.faultTargets {
		if strings.HasSuffix(target.obligation.String(), suffix) {
			return target
		}
	}

	t.Fatalf("fault target with suffix %q not found", suffix)

	return faultTarget{}
}

func requirePin(t *testing.T, pins []applicabilityPin, instanceTemplate string, presence pinPresence) {
	t.Helper()

	for _, pin := range pins {
		if pin.occurrence.instanceTemplate == instanceTemplate && pin.presence == presence {
			return
		}
	}

	t.Fatalf("pin for instance template %q with presence %d not found: %#v", instanceTemplate, presence, pins)
}

func requireOnlyPresencePin(t *testing.T, pins []applicabilityPin, instanceTemplate string, presence pinPresence) {
	t.Helper()

	count := 0

	for _, pin := range pins {
		if pin.occurrence.instanceTemplate == instanceTemplate && pin.presence != planPinNoPresence {
			count++

			require.Equal(t, presence, pin.presence)
		}
	}

	require.Equal(t, 1, count, "presence pin for %s", instanceTemplate)
}

func requireNoPresencePin(t *testing.T, pins []applicabilityPin, instanceTemplate string, forbidden ...pinPresence) {
	t.Helper()

	for _, pin := range pins {
		if pin.occurrence.instanceTemplate != instanceTemplate || pin.presence == planPinNoPresence {
			continue
		}

		if len(forbidden) == 0 || pin.presence == forbidden[0] {
			t.Fatalf("unexpected presence pin for instance template %q: %#v", instanceTemplate, pins)
		}
	}
}

func requireKindPin(t *testing.T, pins []applicabilityPin, occurrence schemaOccurrence, kind jsonKind) {
	t.Helper()

	for _, pin := range pins {
		if pin.hasKind && pin.occurrence == occurrence && pin.kind == kind {
			return
		}
	}

	t.Fatalf("kind pin for %s with kind %s not found: %#v", occurrence.usePointer, jsonKindName(kind), pins)
}

func requireNoCompositionPin(t *testing.T, pins []applicabilityPin, composition string, branch int) {
	t.Helper()

	for _, pin := range pins {
		if pin.hasBranch && pin.composition == composition && pin.branch == branch {
			t.Fatalf("unexpected composition pin %s[%d]: %#v", composition, branch, pins)
		}
	}
}

func requireCompositionPin(t *testing.T, pins []applicabilityPin, composition string, branch int, truth bool) {
	t.Helper()

	for _, pin := range pins {
		if pin.composition == composition && pin.branch == branch && pin.hasBranch && pin.truth == truth {
			return
		}
	}

	t.Fatalf("composition pin %s[%d]=%t not found: %#v", composition, branch, truth, pins)
}

func occurrenceUsePointers(occurrences []schemaOccurrence) []string {
	result := make([]string, 0, len(occurrences))
	for _, occurrence := range occurrences {
		result = append(result, occurrence.usePointer)
	}

	return result
}
