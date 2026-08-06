package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildMergesAllOfArrayItemSchemas verifies composed item witnesses.
func TestBuildMergesAllOfArrayItemSchemas(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"array",
		"items":{"type":"string"},
		"allOf":[{"items":{"enum":["z"]}}]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.NotEmpty(t, cases)
	require.Contains(t, cases, Case{JSON: []byte(`["z"]`), Valid: true})
}

// TestBuildMergesNestedAllOfArrayItemSchemas verifies nested composed item witnesses.
func TestBuildMergesNestedAllOfArrayItemSchemas(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"array",
		"minItems":1,
		"items":{"type":"string"},
		"allOf":[{"allOf":[{"items":{"enum":["z"]}}]}]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Contains(t, cases, Case{JSON: []byte(`["z"]`), Valid: true})
}

// TestBuildMergesAllOfObjectPropertySchemas verifies composed property witnesses.
func TestBuildMergesAllOfObjectPropertySchemas(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"required":["x"],
		"properties":{"x":{"type":"string"}},
		"allOf":[{"properties":{"x":{"enum":["z"]}}}]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.NotEmpty(t, cases)
	require.Contains(t, cases, Case{JSON: []byte(`{"x":"z"}`), Valid: true})
}

// TestBuildUsesNestedAllOfObjectBounds verifies composed lower-bound witnesses.
func TestBuildUsesNestedAllOfObjectBounds(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"allOf":[{"allOf":[{"minProperties":3}]}]
	}`))

	cases := make([]Case, 0)
	_, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)

	found := false
	for _, testCase := range cases {
		value, parseErr := parseStrictJSON(testCase.JSON)
		require.NoError(t, parseErr)
		if value.kind == jsonObject && len(value.object) >= 3 {
			found = true

			break
		}
	}

	require.True(t, found)
}

// TestBuildUsesComposedAdditionalPropertySchemas verifies wildcard value witnesses.
func TestBuildUsesComposedAdditionalPropertySchemas(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"minProperties":1,
		"allOf":[{"additionalProperties":{"enum":["z"]}}]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)

	found := false
	for _, testCase := range cases {
		value, parseErr := parseStrictJSON(testCase.JSON)
		require.NoError(t, parseErr)
		if value.kind != jsonObject || len(value.object) != 1 {
			continue
		}

		for _, member := range value.object {
			if member.kind == jsonString && member.text == "z" {
				found = true
			}
		}
	}

	require.True(t, found)
	require.Contains(t, report.Covered, "#/paths/~1/post/requestBody/content/application~1json/schema/allOf/0/additionalProperties|#/*|enum|level:member:0")
}

// TestBuildTargetsComposedAdditionalProperties verifies nested wildcard applicability.
func TestBuildTargetsComposedAdditionalProperties(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"allOf":[{"additionalProperties":{"type":"string"}}]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Contains(t, report.Covered, "#/paths/~1/post/requestBody/content/application~1json/schema/allOf/0/additionalProperties|#/*|type|level:string")

	found := false
	for _, testCase := range cases {
		value, parseErr := parseStrictJSON(testCase.JSON)
		require.NoError(t, parseErr)
		if value.kind == jsonObject && len(value.object) > 0 {
			found = true

			break
		}
	}

	require.True(t, found)
}

// TestBuildKeepsAnyOfSiblingPropertiesAsAlternatives verifies sibling property masks.
func TestBuildKeepsAnyOfSiblingPropertiesAsAlternatives(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"anyOf":[
			{"required":["x"],"properties":{"x":{"type":"string"}}},
			{"required":["x"],"properties":{"x":{"type":"number"}}}
		]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Contains(t, cases, Case{JSON: []byte(`{"x":""}`), Valid: true})
	require.Contains(t, cases, Case{JSON: []byte(`{"x":-1}`), Valid: true})
	require.Contains(t, report.Covered, "#/paths/~1/post/requestBody/content/application~1json/schema/anyOf/0/properties/x|#/x|type|level:string")
	require.Contains(t, report.Covered, "#/paths/~1/post/requestBody/content/application~1json/schema/anyOf/1/properties/x|#/x|type|level:number")
}

// TestBuildKeepsAnyOfArrayBranchesAsAlternatives verifies exact array masks.
func TestBuildKeepsAnyOfArrayBranchesAsAlternatives(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"array",
		"items":{},
		"anyOf":[
			{"type":"array","items":{},"maxItems":0},
			{"type":"array","items":{},"minItems":1}
		]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Contains(t, cases, Case{JSON: []byte(`[]`), Valid: true})
	require.Contains(t, cases, Case{JSON: []byte(`[null]`), Valid: true})
}

// TestBuildCompositionGoldenLocksCasesAndReport verifies the exact composed stream.
func TestBuildCompositionGoldenLocksCasesAndReport(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"array",
		"items":{},
		"anyOf":[
			{"type":"array","items":{},"maxItems":0},
			{"type":"array","items":{},"minItems":1}
		]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	const schemaPointer = "#/paths/~1/post/requestBody/content/application~1json/schema"

	require.NoError(t, err)
	require.Equal(t, []Case{
		{JSON: []byte(`[]`), Valid: true},
		{JSON: []byte(`[]`), Valid: true},
		{JSON: []byte(`[null]`), Valid: true},
		{JSON: []byte(`[]`), Valid: true},
		{JSON: []byte(`[]`), Valid: true},
		{JSON: []byte(`[null]`), Valid: true},
		{JSON: []byte(`[null]`), Valid: true},
		{JSON: []byte(`[null]`), Valid: true},
		{JSON: []byte(`[false]`), Valid: true},
		{JSON: []byte(`[-1]`), Valid: true},
		{JSON: []byte(`[""]`), Valid: true},
		{JSON: []byte(`[[]]`), Valid: true},
		{JSON: []byte(`[{}]`), Valid: true},
	}, cases)
	require.Equal(t, Report{
		Stop:  SpaceExhausted,
		Steps: 1186,
		Covered: []string{
			schemaPointer + "|#|type|level:array",
			schemaPointer + "|#|anyOf|level:mask:1",
			schemaPointer + "|#|anyOf|level:mask:2",
			schemaPointer + "/anyOf/0|#|type|level:array",
			schemaPointer + "/anyOf/0|#|maxItems|level:valid",
			schemaPointer + "/anyOf/0/items|#/*|type|level:null",
			schemaPointer + "/anyOf/0/items|#/*|type|level:boolean",
			schemaPointer + "/anyOf/0/items|#/*|type|level:number",
			schemaPointer + "/anyOf/0/items|#/*|type|level:string",
			schemaPointer + "/anyOf/0/items|#/*|type|level:array",
			schemaPointer + "/anyOf/0/items|#/*|type|level:object",
			schemaPointer + "/anyOf/1|#|type|level:array",
			schemaPointer + "/anyOf/1|#|minItems|level:valid",
			schemaPointer + "/anyOf/1/items|#/*|type|level:null",
			schemaPointer + "/anyOf/1/items|#/*|type|level:boolean",
			schemaPointer + "/anyOf/1/items|#/*|type|level:number",
			schemaPointer + "/anyOf/1/items|#/*|type|level:string",
			schemaPointer + "/anyOf/1/items|#/*|type|level:array",
			schemaPointer + "/anyOf/1/items|#/*|type|level:object",
			schemaPointer + "/items|#/*|type|level:null",
			schemaPointer + "/items|#/*|type|level:boolean",
			schemaPointer + "/items|#/*|type|level:number",
			schemaPointer + "/items|#/*|type|level:string",
			schemaPointer + "/items|#/*|type|level:array",
			schemaPointer + "/items|#/*|type|level:object",
		},
		Uncovered: []string{
			schemaPointer + "|#|anyOf|fault:anyOf",
			schemaPointer + "/anyOf/0|#|type|fault:type",
			schemaPointer + "/anyOf/0|#|maxItems|fault:maxItems",
			schemaPointer + "/anyOf/1|#|type|fault:type",
			schemaPointer + "/anyOf/1|#|minItems|fault:minItems",
		},
	}, report)
}

// TestCompositionPinsMatchConcreteArrayInstances verifies wildcard pin matching.
func TestCompositionPinsMatchConcreteArrayInstances(t *testing.T) {
	t.Parallel()

	parent := schemaOccurrence{usePointer: "#/items", instanceTemplate: "#/*"}
	pin := anyOfValidPins(parent, 0)[0]
	concrete := schemaOccurrence{usePointer: "#/items", instanceTemplate: "#/0"}

	require.True(t, rowHasCompositionPins([]applicabilityPin{pin}, concrete, "anyOf"))
}

// TestCompositionCoverageScansAllWildcardTruthVectors verifies wildcard coverage.
func TestCompositionCoverageScansAllWildcardTruthVectors(t *testing.T) {
	t.Parallel()

	expectedOccurrence := schemaOccurrence{
		usePointer:       "#/schema",
		targetPointer:    "#/schema",
		instanceTemplate: "#/*",
	}
	expected := makeLevelIdentity(
		makeRuleIdentity(expectedOccurrence, oracleRuleAnyOf),
		planLevelMask+"2",
	)

	result := evaluation{anyOf: []compositionTruth{
		{
			ruleIdentity: makeRuleIdentity(
				schemaOccurrence{
					usePointer:       "#/schema",
					targetPointer:    "#/schema",
					instanceTemplate: "#/0",
				},
				oracleRuleAnyOf,
			),
			branches: []bool{true, false},
		},
		{
			ruleIdentity: makeRuleIdentity(
				schemaOccurrence{
					usePointer:       "#/schema",
					targetPointer:    "#/schema",
					instanceTemplate: "#/1",
				},
				oracleRuleAnyOf,
			),
			branches: []bool{false, true},
		},
	}}

	require.True(t, compositionLevelWasObserved(result, expected))
}

// TestBuildKeepsAnyOfRequiredMembersInTheirBranch verifies branch-local requiredness.
func TestBuildKeepsAnyOfRequiredMembersInTheirBranch(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"anyOf":[
			{"required":["a"],"properties":{"a":{"type":"string"}}},
			{"required":["b"],"properties":{"b":{"type":"number"}}}
		]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Contains(t, cases, Case{JSON: []byte(`{"a":""}`), Valid: true})
	require.Contains(t, cases, Case{JSON: []byte(`{"b":-1}`), Valid: true})
}
