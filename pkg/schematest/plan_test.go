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
		"#/paths/~1/post/requestBody/content/application~1json/schema/properties/a~1b|#/a~1b|type|fault:type",
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
	requireCompositionPin(t, anyOfFault.pins, "anyOf", 1, true)
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

	rootType := findValidTarget(t, anyOfPlan, "|type|level:null")
	requireCompositionPin(t, rootType.pins, "anyOf", 0, true)
	requireCompositionPin(t, rootType.pins, "anyOf", 1, false)

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

func requireKindPin(t *testing.T, pins []applicabilityPin, occurrence schemaOccurrence, kind jsonKind) {
	t.Helper()

	for _, pin := range pins {
		if pin.hasKind && pin.occurrence == occurrence && pin.kind == kind {
			return
		}
	}

	t.Fatalf("kind pin for %s with kind %s not found: %#v", occurrence.usePointer, jsonKindName(kind), pins)
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
