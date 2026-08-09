package schematest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildMatchesNestedReferenceCoverageAtRepeatedAliasDestinations requirements plan/oracle identity agreement.
func TestBuildMatchesNestedReferenceCoverageAtRepeatedAliasDestinations(t *testing.T) {
	t.Parallel()

	document := []byte(`openapi: 3.0.4
x-shared: &outer
  type: object
  required: [child]
  properties:
    child: {type: string, minLength: 1}
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [a, b]
              properties:
                a: *outer
                b: *outer
`)
	report, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000}, func(Case) error {
		return nil
	})
	require.NoError(t, err)

	rootUse := "#/paths/~1/post/requestBody/content/application~1json/schema"
	for _, branch := range []struct{ use, instance string }{
		{use: "/properties/a/properties/child", instance: "#/a/child"},
		{use: "/properties/b/properties/child", instance: "#/b/child"},
	} {
		require.Contains(t, report.Covered, rootUse+branch.use+"|"+branch.instance+"|type|level:string")
		require.Contains(t, report.Covered, rootUse+branch.use+"|"+branch.instance+"|minLength|level:valid")
		require.Contains(t, report.Covered, rootUse+branch.use+"|"+branch.instance+"|minLength|fault:minLength")
	}

	model, parseErr := parseInput(Input{OpenAPI: document, OperationID: "selected"})
	require.NoError(t, parseErr)

	plan, planErr := makePlan(model)
	require.NoError(t, planErr)

	var targets []string

	for _, fault := range plan.faultSchedule {
		if fault.obligation.rule == oracleRuleMinLength {
			targets = append(targets, fault.obligation.occurrence.targetPointer)
		}
	}

	require.Equal(t, []string{
		rootUse + "/properties/a/properties/child",
		rootUse + "/properties/b/properties/child",
	}, targets)
}

// TestBuildMergesAllOfArrayItemSchemas verifies composed item witnesses.
func TestBuildMergesAllOfArrayItemSchemas(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"array",
		"minItems":1,
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
	require.Equal(t, MaxStepsReached, report.Stop)
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
	require.Equal(t, MaxStepsReached, report.Stop)
	require.Contains(t, cases, Case{JSON: []byte(`["z"]`), Valid: true})
}

// TestBuildMergesNestedAnyOfArrayItemSchemas verifies nested alternative item witnesses.
func TestBuildMergesNestedAnyOfArrayItemSchemas(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"array",
		"minItems":1,
		"items":{},
		"allOf":[{"anyOf":[
			{"items":{"enum":["z"]}},
			{"items":{"enum":["q"]}}
		]}]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100000},
		func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		},
	)

	require.NoError(t, err)

	const schemaPointer = "#/paths/~1/post/requestBody/content/application~1json/schema"

	require.Contains(t, cases, Case{JSON: []byte(`["z"]`), Valid: true})
	require.Contains(t, cases, Case{JSON: []byte(`["q"]`), Valid: true})
	require.Contains(t, report.Covered, schemaPointer+"/allOf/0/anyOf/0/items|#/*|enum|level:member:0")
	require.Contains(t, report.Covered, schemaPointer+"/allOf/0/anyOf/1/items|#/*|enum|level:member:0")
}

// TestWalkProjectedDirectArraysCarriesComposedDefaultsIntoMasks verifies complete authored defaults.
func TestWalkProjectedDirectArraysCarriesComposedDefaultsIntoMasks(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"anyOf":[
			{"default":[true,true,true]},
			{"maxItems":2}
		]
	}`))

	model, err := parseInput(Input{OpenAPI: document, OperationID: "selected"})
	require.NoError(t, err)
	plan, err := makePlan(model)
	require.NoError(t, err)

	var (
		target      validIntent
		targetFound bool
	)

	for _, candidate := range plan.validCatalog {
		if strings.HasSuffix(candidate.obligation.String(), "|anyOf|level:mask:1") {
			target = candidate
			targetFound = true

			break
		}
	}

	require.True(t, targetFound)

	request := makeValidRequest([]validIntent{target}, 0, plan.stringObjectives)
	searchState := &search{model: model, maxSteps: 100}

	var row *jsonValue

	cursor := newRowProjectionCursor(model.root, model.root.occurrence, request.requirements)
	defer cursor.Close()

	view, ok, err := cursor.Next()
	require.NoError(t, err)
	require.True(t, ok)

	_, err = view.appendBranchRequirements(
		append([]requirement(nil), request.requirements...), searchState.assign,
	)
	require.NoError(t, err)

	found, err := searchState.walkProjectedDirectValues(
		view, jsonArray,
		func(candidate *jsonValue) (bool, error) {
			if !targetRowMatches(evaluate(model, candidate), request, candidate) {
				return false, nil
			}

			row = candidate

			return true, nil
		},
	)
	require.NoError(t, err)
	require.True(t, found)

	encoded, err := marshalStrict(row)
	require.NoError(t, err)
	require.Equal(t, `[true,true,true]`, string(encoded))
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

// TestBuildUsesUnconstrainedNestedAnyOfBoundsForOuterAllOf verifies ancestor repair bounds.
func TestBuildUsesUnconstrainedNestedAnyOfBoundsForOuterAllOf(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"array",
		"items":{"enum":[null]},
		"allOf":[{"anyOf":[{"minItems":3},{"minItems":5}]}]
	}`))

	cases := make([]Case, 0)
	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 1000000},
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

		if value.kind == jsonArray && (len(value.array) == 3 || len(value.array) == 5) {
			found = true

			break
		}
	}

	require.True(t, found)

	const schemaPointer = "#/paths/~1/post/requestBody/content/application~1json/schema"
	require.Contains(t, report.Covered, schemaPointer+"|#|allOf|level:all-true")
}

// TestBuildUsesNestedConstrainedAnyOfArrayBounds verifies selected nested bounds.
func TestBuildUsesNestedConstrainedAnyOfArrayBounds(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"array",
		"items":{},
		"allOf":[{"anyOf":[{"minItems":3},{"maxItems":0}]}]
	}`))

	model, err := parseInput(Input{OpenAPI: document, OperationID: "selected"})
	require.NoError(t, err)
	plan, err := makePlan(model)
	require.NoError(t, err)

	var target validIntent

	foundTarget := false

	for _, candidate := range plan.validCatalog {
		if strings.Contains(candidate.obligation.String(), "/allOf/0/anyOf/0|#|minItems|level:valid") {
			target = candidate
			foundTarget = true

			break
		}
	}

	require.True(t, foundTarget)

	request := makeValidRequest([]validIntent{target}, 0, plan.stringObjectives)

	searchState := &search{model: model, maxSteps: 1000}
	row, found, err := findTargetRow(plan, request, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, row)
	require.Equal(t, jsonArray, row.kind)
	require.Len(t, row.array, 3)

	result := evaluate(model, row)
	require.NoError(t, result.err)
	require.True(t, targetRowMatches(result, request, row))
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

	const schemaPointer = "#/paths/~1/post/requestBody/content/application~1json/schema"
	require.Contains(t, report.Covered, schemaPointer+"/allOf/0/additionalProperties|#/*|enum|level:member:0")
}

// TestBuildTargetsComposedAdditionalProperties verifies nested wildcard applicability.
func TestBuildTargetsComposedAdditionalProperties(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"minProperties":1,
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

	const schemaPointer = "#/paths/~1/post/requestBody/content/application~1json/schema"
	require.Contains(t, report.Covered, schemaPointer+"/allOf/0/additionalProperties|#/*|type|level:string")

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

// TestBuildAppliesWildcardsAcrossComposedMemberDeclarations verifies both owner forms.
func TestBuildAppliesWildcardsAcrossComposedMemberDeclarations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		path   string
	}{
		{
			name: "root property and composed wildcard",
			schema: `{
				"type":"object",
				"required":["x"],
				"properties":{"x":{"type":"string"}},
				"allOf":[{"additionalProperties":{"enum":["z"]}}]
			}`,
			path: "#/paths/~1/post/requestBody/content/application~1json/schema/allOf/0/" +
				"additionalProperties|#/*|enum|level:member:0",
		},
		{
			name: "root wildcard and composed property",
			schema: `{
				"type":"object",
				"additionalProperties":{"enum":["z"]},
				"allOf":[{"required":["x"],"properties":{"x":{"type":"string"}}}]
			}`,
			path: "#/paths/~1/post/requestBody/content/application~1json/schema/additionalProperties|#/*|enum|level:member:0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cases := make([]Case, 0)
			report, err := Build(
				Input{OpenAPI: []byte(documentWithJSONSchema(test.schema)), OperationID: "selected", MaxSteps: 10000},
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

				found = found || testCase.Valid && value.kind == jsonObject &&
					value.object["x"] != nil && value.object["x"].text == "z"
			}

			require.True(t, found)
			require.Contains(t, report.Covered, test.path)
		})
	}
}

// TestBuildPreservesUnconstrainedNestedAnyOfWildcards verifies wildcard alternatives.
func TestBuildPreservesUnconstrainedNestedAnyOfWildcards(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"required":["x"],
		"properties":{"x":{}},
		"allOf":[{"anyOf":[
			{"additionalProperties":{"enum":["z"]}},
			{"additionalProperties":{"enum":["q"]}}
		]}]
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

		if testCase.Valid && value.kind == jsonObject && value.object["x"] != nil &&
			(value.object["x"].text == "z" || value.object["x"].text == "q") {
			found = true

			break
		}
	}

	require.True(t, found)

	const schemaPointer = "#/paths/~1/post/requestBody/content/application~1json/schema"
	require.Contains(t, report.Covered, schemaPointer+"|#|allOf|level:all-true")
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
	require.Equal(t, []Case{
		{JSON: []byte(`{"x":""}`), Valid: true},
		{JSON: []byte(`{"x":""}`), Valid: true},
		{JSON: []byte(`null`), Valid: false},
		{JSON: []byte(`{}`), Valid: false},
		{JSON: []byte(`{}`), Valid: false},
	}, cases)

	const schemaPointer = "#/paths/~1/post/requestBody/content/application~1json/schema"
	require.Contains(t, report.Covered, schemaPointer+"/anyOf/0/properties/x|#/x|type|level:string")
	require.Contains(t, report.Uncovered, schemaPointer+"/anyOf/1/properties/x|#/x|type|level:number")
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
	require.Equal(t, MaxStepsReached, report.Stop)
	require.Equal(t, uint64(10000), report.Steps)
	require.Equal(t, []Case{{JSON: []byte(`[false]`), Valid: true}}, cases)
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
	require.Equal(t, []Case{{JSON: []byte(`[false]`), Valid: true}}, cases)

	for _, testCase := range cases {
		require.True(t, testCase.Valid)
	}

	require.Equal(t, MaxStepsReached, report.Stop)
	require.Equal(t, uint64(10000), report.Steps)
	require.Contains(t, report.Uncovered, schemaPointer+"|#|anyOf|level:mask:1")
	require.Contains(t, report.Covered, schemaPointer+"|#|anyOf|level:mask:2")
	require.Contains(t, report.Uncovered, schemaPointer+"|#|anyOf|level:mask:3")
	require.Contains(t, report.Uncovered, schemaPointer+"|#|anyOf|fault:anyOf")
}

// TestCompositionRequirementsMatchConcreteArrayInstances verifies wildcard requirement matching.
func TestCompositionRequirementsMatchConcreteArrayInstances(t *testing.T) {
	t.Parallel()

	parent := schemaOccurrence{usePointer: "#/items", instanceTemplate: "#/*"}
	constraint := anyOfValidRequirements(parent, 0)[0]
	concrete := schemaOccurrence{usePointer: "#/items", instanceTemplate: "#/0"}

	require.True(t, rowHasCompositionRequirements([]requirement{constraint}, concrete, "anyOf"))
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

	result := evaluation{records: newEvaluationRecords()}
	appendAnyOfTruth(&result, compositionTruth{
		ruleIdentity: makeRuleIdentity(
			schemaOccurrence{
				usePointer:       "#/schema",
				targetPointer:    "#/schema",
				instanceTemplate: "#/0",
			},
			oracleRuleAnyOf,
		),
		branches: []bool{true, false},
	})
	appendAnyOfTruth(&result, compositionTruth{
		ruleIdentity: makeRuleIdentity(
			schemaOccurrence{
				usePointer:       "#/schema",
				targetPointer:    "#/schema",
				instanceTemplate: "#/1",
			},
			oracleRuleAnyOf,
		),
		branches: []bool{false, true},
	})
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
	require.Equal(t, MaxStepsReached, report.Stop)
	require.Equal(t, uint64(10000), report.Steps)
	require.Contains(t, cases, Case{JSON: []byte(`{"a":""}`), Valid: true})
	require.Contains(t, cases, Case{JSON: []byte(`{"a":"","b":0}`), Valid: true})
}
