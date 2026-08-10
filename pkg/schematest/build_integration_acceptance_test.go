package schematest

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// publicSchemaPointer is the selected request schema's canonical use pointer.
const publicSchemaPointer = "#/paths/~1/post/requestBody/content/application~1json/schema"

// TestBuildCompoundReferenceDirectionStreamAndCutoffAreExact locks repeated references and request direction.
func TestBuildCompoundReferenceDirectionStreamAndCutoffAreExact(t *testing.T) {
	t.Parallel()

	document := []byte(`openapi: 3.0.4
components:
  schemas:
    Flag: {type: boolean}
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [left, right, server]
              additionalProperties: false
              properties:
                left: {$ref: '#/components/schemas/Flag'}
                right: {$ref: '#/components/schemas/Flag'}
                server: {type: boolean, readOnly: true}
`)

	fullCases, fullReport, err := collectDeterministicRun(Input{
		OpenAPI: document, OperationID: "selected", MaxSteps: 10_000,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, []Case{
		{JSON: []byte(`{"left":false,"right":false,"server":false}`), Valid: true},
		{JSON: []byte(`null`), Valid: false},
		{JSON: []byte(`{"__schematest_extra__":null,"left":false,"right":false}`), Valid: false},
		{JSON: []byte(`{"right":false}`), Valid: false},
		{JSON: []byte(`{"left":false}`), Valid: false},
		{JSON: []byte(`{"left":null,"right":false}`), Valid: false},
		{JSON: []byte(`{"left":false,"right":null}`), Valid: false},
		{JSON: []byte(`{"left":false,"right":false,"server":null}`), Valid: false},
	}, fullCases)
	require.Equal(t, SpaceExhausted, fullReport.Stop)
	require.Equal(t, uint64(1_910), fullReport.Steps)
	require.Empty(t, fullReport.Uncovered)
	require.Equal(t, []string{
		publicSchemaPointer + "|#|type|level:object",
		publicSchemaPointer + "|#|type|fault:type",
		publicSchemaPointer + "|#/*|additionalProperties|fault:additionalProperties",
		publicSchemaPointer + "|#/left|required|level:present",
		publicSchemaPointer + "|#/left|required|fault:required",
		publicSchemaPointer + "|#/right|required|level:present",
		publicSchemaPointer + "|#/right|required|fault:required",
		publicSchemaPointer + "/properties/left|#/left|type|level:boolean",
		publicSchemaPointer + "/properties/left|#/left|type|fault:type",
		publicSchemaPointer + "/properties/right|#/right|type|level:boolean",
		publicSchemaPointer + "/properties/right|#/right|type|fault:type",
		publicSchemaPointer + "/properties/server|#/server|type|level:boolean",
		publicSchemaPointer + "/properties/server|#/server|type|fault:type",
	}, fullReport.Covered)

	for range 2 {
		cases, report, buildErr := collectDeterministicRun(Input{
			OpenAPI: document, OperationID: "selected", MaxSteps: 100,
		}, nil)
		require.NoError(t, buildErr)
		require.Equal(t, fullCases[:2], cases)
		require.Equal(t, MaxStepsReached, report.Stop)
		require.Equal(t, uint64(100), report.Steps)
		require.Equal(t, []string{
			fullReport.Covered[0],
			fullReport.Covered[1],
			fullReport.Covered[3],
			fullReport.Covered[5],
			fullReport.Covered[7],
			fullReport.Covered[9],
			fullReport.Covered[11],
		}, report.Covered)
		require.Equal(t, []string{
			fullReport.Covered[2],
			fullReport.Covered[4],
			fullReport.Covered[6],
			fullReport.Covered[8],
			fullReport.Covered[10],
			fullReport.Covered[12],
		}, report.Uncovered)
	}
}

// TestBuildCompoundAnyOfClosureAndMasksAreExact locks additive masks and the all-false aggregate closure.
func TestBuildCompoundAnyOfClosureAndMasksAreExact(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{"anyOf":[{"enum":["a"]},{"enum":[1]}]}`))
	firstCases, firstReport, err := collectDeterministicRun(Input{
		OpenAPI: document, OperationID: "selected", MaxSteps: 1_000,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, []Case{
		{JSON: []byte(`1`), Valid: true},
		{JSON: []byte(`"a"`), Valid: true},
		{JSON: []byte(`"a"`), Valid: true},
		{JSON: []byte(`1`), Valid: true},
		{JSON: []byte(`false`), Valid: false},
		{JSON: []byte(`""`), Valid: false},
		{JSON: []byte(`""`), Valid: false},
	}, firstCases)
	require.Equal(t, Report{
		Stop:  SpaceExhausted,
		Steps: 286,
		Covered: []string{
			publicSchemaPointer + "|#|type|level:number",
			publicSchemaPointer + "|#|type|level:string",
			publicSchemaPointer + "|#|anyOf|level:mask:1",
			publicSchemaPointer + "|#|anyOf|level:mask:2",
			publicSchemaPointer + "|#|anyOf|fault:anyOf",
			publicSchemaPointer + "/anyOf/0|#|type|level:number",
			publicSchemaPointer + "/anyOf/0|#|type|level:string",
			publicSchemaPointer + "/anyOf/0|#|enum|level:member:0",
			publicSchemaPointer + "/anyOf/0|#|enum|fault:enum",
			publicSchemaPointer + "/anyOf/1|#|type|level:number",
			publicSchemaPointer + "/anyOf/1|#|type|level:string",
			publicSchemaPointer + "/anyOf/1|#|enum|level:member:0",
			publicSchemaPointer + "/anyOf/1|#|enum|fault:enum",
		},
		Uncovered: []string{
			publicSchemaPointer + "|#|type|level:boolean",
			publicSchemaPointer + "|#|type|level:null",
			publicSchemaPointer + "|#|type|level:array",
			publicSchemaPointer + "|#|type|level:object",
			publicSchemaPointer + "|#|anyOf|level:mask:3",
			publicSchemaPointer + "/anyOf/0|#|type|level:boolean",
			publicSchemaPointer + "/anyOf/0|#|type|level:null",
			publicSchemaPointer + "/anyOf/0|#|type|level:array",
			publicSchemaPointer + "/anyOf/0|#|type|level:object",
			publicSchemaPointer + "/anyOf/1|#|type|level:boolean",
			publicSchemaPointer + "/anyOf/1|#|type|level:null",
			publicSchemaPointer + "/anyOf/1|#|type|level:array",
			publicSchemaPointer + "/anyOf/1|#|type|level:object",
		},
	}, firstReport)

	secondCases, secondReport, err := collectDeterministicRun(Input{
		OpenAPI: document, OperationID: "selected", MaxSteps: 1_000,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, firstCases, secondCases)
	require.Equal(t, firstReport, secondReport)
}

// TestBuildSkipsOpenFirstFaultAlternative proves a genuinely open,
// unproductive item product cannot starve the later boolean mutation.
func TestBuildSkipsOpenFirstFaultAlternative(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"array","maxItems":0,
		"items":{"anyOf":[
			{"type":"string","allOf":[{"pattern":"^a+$"},{"pattern":"^b+$"}]},
			{"enum":[false]}
		]}
	}`))

	cases, report, err := collectDeterministicRun(Input{
		OpenAPI: document, OperationID: "selected", MaxSteps: 20_000,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, []Case{
		{JSON: []byte(`null`), Valid: false},
		{JSON: []byte(`[false]`), Valid: false},
	}, cases)

	item := publicSchemaPointer + "/items"

	uncovered := []string{
		publicSchemaPointer + "|#|type|level:array",
		publicSchemaPointer + "|#|maxItems|level:valid",
		item + "|#/*|type|level:boolean",
		item + "|#/*|type|level:null",
		item + "|#/*|type|level:number",
		item + "|#/*|type|level:string",
		item + "|#/*|type|level:array",
		item + "|#/*|type|level:object",
		item + "|#/*|anyOf|level:mask:1",
		item + "|#/*|anyOf|level:mask:2",
		item + "|#/*|anyOf|level:mask:3",
		item + "|#/*|anyOf|fault:anyOf",
		item + "/anyOf/0|#/*|type|level:string",
		item + "/anyOf/0|#/*|type|fault:type",
		item + "/anyOf/0|#/*|allOf|level:all-true",
	}
	for branch := range 2 {
		pointer := item + "/anyOf/0/allOf/" + strconv.Itoa(branch)
		uncovered = append(
			uncovered,
			pointer+"|#/*|type|level:boolean",
			pointer+"|#/*|type|level:null",
			pointer+"|#/*|type|level:number",
			pointer+"|#/*|type|level:string",
			pointer+"|#/*|type|level:array",
			pointer+"|#/*|type|level:object",
			pointer+"|#/*|pattern|level:valid",
			pointer+"|#/*|pattern|fault:pattern",
		)
	}

	uncovered = append(
		uncovered,
		item+"/anyOf/1|#/*|type|level:boolean",
		item+"/anyOf/1|#/*|type|level:null",
		item+"/anyOf/1|#/*|type|level:number",
		item+"/anyOf/1|#/*|type|level:string",
		item+"/anyOf/1|#/*|type|level:array",
		item+"/anyOf/1|#/*|type|level:object",
		item+"/anyOf/1|#/*|enum|level:member:0",
		item+"/anyOf/1|#/*|enum|fault:enum",
	)
	require.Equal(t, Report{
		Stop:  MaxStepsReached,
		Steps: 20_000,
		Covered: []string{
			publicSchemaPointer + "|#|type|fault:type",
			publicSchemaPointer + "|#|maxItems|fault:maxItems",
		},
		Uncovered: uncovered,
	}, report)
}

// TestBuildBeyondUint64ArrayYieldsToLaterProjection proves one open current
// row advances fairly without starving a reachable sibling projection.
func TestBuildBeyondUint64ArrayYieldsToLaterProjection(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"anyOf":[
			{"type":"array","minItems":18446744073709551616,
			 "maxItems":18446744073709551616,"items":{"enum":[false]}},
			{"enum":[0]}
		]
	}`))

	all := []string{
		publicSchemaPointer + "|#|type|level:boolean",
		publicSchemaPointer + "|#|type|level:null",
		publicSchemaPointer + "|#|type|level:number",
		publicSchemaPointer + "|#|type|level:string",
		publicSchemaPointer + "|#|type|level:array",
		publicSchemaPointer + "|#|type|level:object",
		publicSchemaPointer + "|#|anyOf|level:mask:1",
		publicSchemaPointer + "|#|anyOf|level:mask:2",
		publicSchemaPointer + "|#|anyOf|level:mask:3",
		publicSchemaPointer + "|#|anyOf|fault:anyOf",
		publicSchemaPointer + "/anyOf/0|#|type|level:array",
		publicSchemaPointer + "/anyOf/0|#|type|fault:type",
		publicSchemaPointer + "/anyOf/0|#|minItems|level:valid",
		publicSchemaPointer + "/anyOf/0|#|minItems|fault:minItems",
		publicSchemaPointer + "/anyOf/0|#|maxItems|level:valid",
		publicSchemaPointer + "/anyOf/0|#|maxItems|fault:maxItems",
		publicSchemaPointer + "/anyOf/0/items|#/*|type|level:boolean",
		publicSchemaPointer + "/anyOf/0/items|#/*|type|level:null",
		publicSchemaPointer + "/anyOf/0/items|#/*|type|level:number",
		publicSchemaPointer + "/anyOf/0/items|#/*|type|level:string",
		publicSchemaPointer + "/anyOf/0/items|#/*|type|level:array",
		publicSchemaPointer + "/anyOf/0/items|#/*|type|level:object",
		publicSchemaPointer + "/anyOf/0/items|#/*|enum|level:member:0",
		publicSchemaPointer + "/anyOf/0/items|#/*|enum|fault:enum",
		publicSchemaPointer + "/anyOf/1|#|type|level:boolean",
		publicSchemaPointer + "/anyOf/1|#|type|level:null",
		publicSchemaPointer + "/anyOf/1|#|type|level:number",
		publicSchemaPointer + "/anyOf/1|#|type|level:string",
		publicSchemaPointer + "/anyOf/1|#|type|level:array",
		publicSchemaPointer + "/anyOf/1|#|type|level:object",
		publicSchemaPointer + "/anyOf/1|#|enum|level:member:0",
		publicSchemaPointer + "/anyOf/1|#|enum|fault:enum",
	}
	covered := []string{all[2], all[7], all[26], all[30]}
	uncovered := append([]string(nil), all[:2]...)
	uncovered = append(uncovered, all[3:7]...)
	uncovered = append(uncovered, all[8:26]...)
	uncovered = append(uncovered, all[27:30]...)
	uncovered = append(uncovered, all[31])

	for range 2 {
		cases, report, err := collectDeterministicRun(Input{
			OpenAPI: document, OperationID: "selected", MaxSteps: 200,
		}, nil)
		require.NoError(t, err)
		require.Equal(t, []Case{{JSON: []byte(`0`), Valid: true}}, cases)
		require.Equal(t, Report{
			Stop: MaxStepsReached, Steps: 200, Covered: covered, Uncovered: uncovered,
		}, report)
	}

	cases, report, err := collectDeterministicRun(Input{
		OpenAPI: document, OperationID: "selected", MaxSteps: 2,
	}, nil)
	require.NoError(t, err)
	require.Empty(t, cases)
	require.Equal(t, Report{
		Stop: MaxStepsReached, Steps: 2, Uncovered: all,
	}, report)
}

// TestBuildHugeContainerBoundsStopBeforeFirstChildAssignment proves huge authored bounds remain lazy.
func TestBuildHugeContainerBoundsStopBeforeFirstChildAssignment(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
	}{
		{name: "array", schema: `{"type":"array","minItems":1000000000,"items":{"type":"boolean"}}`},
		{name: "object", schema: `{"type":"object","minProperties":1000000000,"additionalProperties":{"type":"boolean"}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cases, report, err := collectDeterministicRun(Input{
				OpenAPI: []byte(documentWithJSONSchema(test.schema)), OperationID: "selected", MaxSteps: 2,
			}, nil)
			require.NoError(t, err)
			require.Empty(t, cases)

			countRule := "minItems"
			childPointer := "/items"
			kind := "array"

			if test.name == "object" {
				countRule = "minProperties"
				childPointer = "/additionalProperties"
				kind = "object"
			}

			require.Equal(t, Report{
				Stop:  MaxStepsReached,
				Steps: 2,
				Uncovered: []string{
					publicSchemaPointer + "|#|type|level:" + kind,
					publicSchemaPointer + "|#|type|fault:type",
					publicSchemaPointer + "|#|" + countRule + "|level:valid",
					publicSchemaPointer + "|#|" + countRule + "|fault:" + countRule,
					publicSchemaPointer + childPointer + "|#/*|type|level:boolean",
					publicSchemaPointer + childPointer + "|#/*|type|fault:type",
				},
			}, report)
		})
	}
}

// TestBuildDeclaredMutationNameCollisionReachesSuffix locks the unbounded canonical name stream.
func TestBuildDeclaredMutationNameCollisionReachesSuffix(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"required":["__schematest_extra__","__schematest_extra___1"],
		"properties":{
			"__schematest_extra__":{"type":"boolean"},
			"__schematest_extra___1":{"type":"boolean"}
		},
		"additionalProperties":false
	}`))
	cases, report, err := collectDeterministicRun(Input{
		OpenAPI: document, OperationID: "selected", MaxSteps: 10_000,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, []Case{
		{JSON: []byte(`{"__schematest_extra__":false,"__schematest_extra___1":false}`), Valid: true},
		{JSON: []byte(`null`), Valid: false},
		{
			JSON:  []byte(`{"__schematest_extra__":false,"__schematest_extra___1":false,"__schematest_extra___2":null}`),
			Valid: false,
		},
		{JSON: []byte(`{"__schematest_extra___1":false}`), Valid: false},
		{JSON: []byte(`{"__schematest_extra__":false}`), Valid: false},
		{JSON: []byte(`{"__schematest_extra__":null,"__schematest_extra___1":false}`), Valid: false},
		{JSON: []byte(`{"__schematest_extra__":false,"__schematest_extra___1":null}`), Valid: false},
	}, cases)
	require.Equal(t, Report{
		Stop:  SpaceExhausted,
		Steps: 3_935,
		Covered: []string{
			publicSchemaPointer + "|#|type|level:object",
			publicSchemaPointer + "|#|type|fault:type",
			publicSchemaPointer + "|#/*|additionalProperties|fault:additionalProperties",
			publicSchemaPointer + "|#/__schematest_extra__|required|level:present",
			publicSchemaPointer + "|#/__schematest_extra__|required|fault:required",
			publicSchemaPointer + "|#/__schematest_extra___1|required|level:present",
			publicSchemaPointer + "|#/__schematest_extra___1|required|fault:required",
			publicSchemaPointer + "/properties/__schematest_extra__|#/__schematest_extra__|type|level:boolean",
			publicSchemaPointer + "/properties/__schematest_extra__|#/__schematest_extra__|type|fault:type",
			publicSchemaPointer + "/properties/__schematest_extra___1|#/__schematest_extra___1|type|level:boolean",
			publicSchemaPointer + "/properties/__schematest_extra___1|#/__schematest_extra___1|type|fault:type",
		},
	}, report)
}

// TestBuildUnavailableBeyondCountChildYieldsNormally proves an exhausted
// beyond-count item attempt is ordinary uncovered work, not a Build error.
func TestBuildUnavailableBeyondCountChildYieldsNormally(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"anyOf":[
			{"type":"array","minItems":18446744073709551616,
			 "maxItems":18446744073709551616,
			 "items":{"type":"string","enum":[false]}},
			{"enum":[0]}
		]
	}`))
	wantCases := []Case{{JSON: []byte(`0`), Valid: true}, {JSON: []byte(`0`), Valid: true}}
	wantReport := Report{
		Stop:  MaxStepsReached,
		Steps: 200,
		Covered: []string{
			publicSchemaPointer + `|#|type|level:number`,
			publicSchemaPointer + `|#|anyOf|level:mask:2`,
			publicSchemaPointer + `/anyOf/1|#|type|level:number`,
			publicSchemaPointer + `/anyOf/1|#|enum|level:member:0`,
		},
		Uncovered: []string{
			publicSchemaPointer + `|#|type|level:boolean`,
			publicSchemaPointer + `|#|type|level:null`,
			publicSchemaPointer + `|#|type|level:string`,
			publicSchemaPointer + `|#|type|level:array`,
			publicSchemaPointer + `|#|type|level:object`,
			publicSchemaPointer + `|#|anyOf|level:mask:1`,
			publicSchemaPointer + `|#|anyOf|level:mask:3`,
			publicSchemaPointer + `|#|anyOf|fault:anyOf`,
			publicSchemaPointer + `/anyOf/0|#|type|level:array`,
			publicSchemaPointer + `/anyOf/0|#|type|fault:type`,
			publicSchemaPointer + `/anyOf/0|#|minItems|level:valid`,
			publicSchemaPointer + `/anyOf/0|#|minItems|fault:minItems`,
			publicSchemaPointer + `/anyOf/0|#|maxItems|level:valid`,
			publicSchemaPointer + `/anyOf/0|#|maxItems|fault:maxItems`,
			publicSchemaPointer + `/anyOf/0/items|#/*|type|level:string`,
			publicSchemaPointer + `/anyOf/0/items|#/*|type|fault:type`,
			publicSchemaPointer + `/anyOf/0/items|#/*|enum|level:member:0`,
			publicSchemaPointer + `/anyOf/0/items|#/*|enum|fault:enum`,
			publicSchemaPointer + `/anyOf/1|#|type|level:boolean`,
			publicSchemaPointer + `/anyOf/1|#|type|level:null`,
			publicSchemaPointer + `/anyOf/1|#|type|level:string`,
			publicSchemaPointer + `/anyOf/1|#|type|level:array`,
			publicSchemaPointer + `/anyOf/1|#|type|level:object`,
			publicSchemaPointer + `/anyOf/1|#|enum|fault:enum`,
		},
	}

	for range 2 {
		cases, report, err := collectDeterministicRun(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 200}, nil)
		require.NoError(t, err)
		require.Equal(t, wantCases, cases)
		require.Equal(t, wantReport, report)
	}
}

// TestBuildGenuinelyCompoundMatrixIsExact crosses repaired modules in one
// unchanged public-Build document at sufficient and boundary budgets.
func TestBuildGenuinelyCompoundMatrixIsExact(t *testing.T) {
	t.Parallel()

	document := []byte(`openapi: 3.0.4
components:
  schemas:
    Flag: {type: boolean}
    Exact: {type: number, enum: [2], minimum: 2, maximum: 2, multipleOf: 2}
    Token:
      allOf:
        - {type: string, format: uuid, pattern: "^[0-9a-f-]+$", minLength: 36, maxLength: 36}
        - {enum: ["00000000-0000-4000-8000-000000000000"]}
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [left, right, exact, token, items, server]
              additionalProperties: false
              properties:
                left: {$ref: "#/components/schemas/Flag"}
                right: {$ref: "#/components/schemas/Flag"}
                exact: {$ref: "#/components/schemas/Exact"}
                token: {$ref: "#/components/schemas/Token"}
                items: {type: array, minItems: 1, maxItems: 1, items: {anyOf: [{enum: [false]}]}}
                server: {type: boolean, readOnly: true}
`)
	valid := Case{
		JSON: []byte(
			`{"exact":2,"items":[false],"left":false,"right":false,"server":false,` +
				`"token":"00000000-0000-4000-8000-000000000000"}`,
		),
		Valid: true,
	}
	wantSufficientCases := []Case{valid, valid, {JSON: []byte(`null`), Valid: false}}
	wantSufficientReport := Report{
		Stop:  MaxStepsReached,
		Steps: 10_000,
		Covered: []string{
			publicSchemaPointer + `|#|type|level:object`,
			publicSchemaPointer + `|#|type|fault:type`,
			publicSchemaPointer + `|#/exact|required|level:present`,
			publicSchemaPointer + `|#/items|required|level:present`,
			publicSchemaPointer + `|#/left|required|level:present`,
			publicSchemaPointer + `|#/right|required|level:present`,
			publicSchemaPointer + `|#/token|required|level:present`,
			publicSchemaPointer + `/properties/exact|#/exact|type|level:number`,
			publicSchemaPointer + `/properties/exact|#/exact|enum|level:member:0`,
			publicSchemaPointer + `/properties/exact|#/exact|minimum|level:valid`,
			publicSchemaPointer + `/properties/exact|#/exact|maximum|level:valid`,
			publicSchemaPointer + `/properties/exact|#/exact|multipleOf|level:valid`,
			publicSchemaPointer + `/properties/items|#/items|type|level:array`,
			publicSchemaPointer + `/properties/items|#/items|minItems|level:valid`,
			publicSchemaPointer + `/properties/items|#/items|maxItems|level:valid`,
			publicSchemaPointer + `/properties/items/items|#/items/*|type|level:boolean`,
			publicSchemaPointer + `/properties/items/items|#/items/*|anyOf|level:mask:1`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|type|level:boolean`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|enum|level:member:0`,
			publicSchemaPointer + `/properties/left|#/left|type|level:boolean`,
			publicSchemaPointer + `/properties/right|#/right|type|level:boolean`,
			publicSchemaPointer + `/properties/server|#/server|type|level:boolean`,
			publicSchemaPointer + `/properties/token|#/token|type|level:string`,
			publicSchemaPointer + `/properties/token|#/token|allOf|level:all-true`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|type|level:string`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|minLength|level:valid`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|maxLength|level:valid`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|pattern|level:valid`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|format|level:valid`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|type|level:string`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|enum|level:member:0`,
		},
		Uncovered: []string{
			publicSchemaPointer + `|#/*|additionalProperties|fault:additionalProperties`,
			publicSchemaPointer + `|#/exact|required|fault:required`,
			publicSchemaPointer + `|#/items|required|fault:required`,
			publicSchemaPointer + `|#/left|required|fault:required`,
			publicSchemaPointer + `|#/right|required|fault:required`,
			publicSchemaPointer + `|#/token|required|fault:required`,
			publicSchemaPointer + `/properties/exact|#/exact|type|fault:type`,
			publicSchemaPointer + `/properties/exact|#/exact|enum|fault:enum`,
			publicSchemaPointer + `/properties/exact|#/exact|minimum|fault:minimum`,
			publicSchemaPointer + `/properties/exact|#/exact|maximum|fault:maximum`,
			publicSchemaPointer + `/properties/exact|#/exact|multipleOf|fault:multipleOf`,
			publicSchemaPointer + `/properties/items|#/items|type|fault:type`,
			publicSchemaPointer + `/properties/items|#/items|minItems|fault:minItems`,
			publicSchemaPointer + `/properties/items|#/items|maxItems|fault:maxItems`,
			publicSchemaPointer + `/properties/items/items|#/items/*|type|level:null`,
			publicSchemaPointer + `/properties/items/items|#/items/*|type|level:number`,
			publicSchemaPointer + `/properties/items/items|#/items/*|type|level:string`,
			publicSchemaPointer + `/properties/items/items|#/items/*|type|level:array`,
			publicSchemaPointer + `/properties/items/items|#/items/*|type|level:object`,
			publicSchemaPointer + `/properties/items/items|#/items/*|anyOf|fault:anyOf`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|type|level:null`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|type|level:number`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|type|level:string`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|type|level:array`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|type|level:object`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|enum|fault:enum`,
			publicSchemaPointer + `/properties/left|#/left|type|fault:type`,
			publicSchemaPointer + `/properties/right|#/right|type|fault:type`,
			publicSchemaPointer + `/properties/server|#/server|type|fault:type`,
			publicSchemaPointer + `/properties/token|#/token|type|level:boolean`,
			publicSchemaPointer + `/properties/token|#/token|type|level:null`,
			publicSchemaPointer + `/properties/token|#/token|type|level:number`,
			publicSchemaPointer + `/properties/token|#/token|type|level:array`,
			publicSchemaPointer + `/properties/token|#/token|type|level:object`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|type|fault:type`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|minLength|fault:minLength`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|maxLength|fault:maxLength`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|pattern|fault:pattern`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|format|fault:format`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|type|level:boolean`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|type|level:null`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|type|level:number`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|type|level:array`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|type|level:object`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|enum|fault:enum`,
		},
	}
	wantBoundaryReport := Report{
		Stop:  MaxStepsReached,
		Steps: 100,
		Uncovered: []string{
			publicSchemaPointer + `|#|type|level:object`,
			publicSchemaPointer + `|#|type|fault:type`,
			publicSchemaPointer + `|#/*|additionalProperties|fault:additionalProperties`,
			publicSchemaPointer + `|#/exact|required|level:present`,
			publicSchemaPointer + `|#/exact|required|fault:required`,
			publicSchemaPointer + `|#/items|required|level:present`,
			publicSchemaPointer + `|#/items|required|fault:required`,
			publicSchemaPointer + `|#/left|required|level:present`,
			publicSchemaPointer + `|#/left|required|fault:required`,
			publicSchemaPointer + `|#/right|required|level:present`,
			publicSchemaPointer + `|#/right|required|fault:required`,
			publicSchemaPointer + `|#/token|required|level:present`,
			publicSchemaPointer + `|#/token|required|fault:required`,
			publicSchemaPointer + `/properties/exact|#/exact|type|level:number`,
			publicSchemaPointer + `/properties/exact|#/exact|type|fault:type`,
			publicSchemaPointer + `/properties/exact|#/exact|enum|level:member:0`,
			publicSchemaPointer + `/properties/exact|#/exact|enum|fault:enum`,
			publicSchemaPointer + `/properties/exact|#/exact|minimum|level:valid`,
			publicSchemaPointer + `/properties/exact|#/exact|minimum|fault:minimum`,
			publicSchemaPointer + `/properties/exact|#/exact|maximum|level:valid`,
			publicSchemaPointer + `/properties/exact|#/exact|maximum|fault:maximum`,
			publicSchemaPointer + `/properties/exact|#/exact|multipleOf|level:valid`,
			publicSchemaPointer + `/properties/exact|#/exact|multipleOf|fault:multipleOf`,
			publicSchemaPointer + `/properties/items|#/items|type|level:array`,
			publicSchemaPointer + `/properties/items|#/items|type|fault:type`,
			publicSchemaPointer + `/properties/items|#/items|minItems|level:valid`,
			publicSchemaPointer + `/properties/items|#/items|minItems|fault:minItems`,
			publicSchemaPointer + `/properties/items|#/items|maxItems|level:valid`,
			publicSchemaPointer + `/properties/items|#/items|maxItems|fault:maxItems`,
			publicSchemaPointer + `/properties/items/items|#/items/*|type|level:boolean`,
			publicSchemaPointer + `/properties/items/items|#/items/*|type|level:null`,
			publicSchemaPointer + `/properties/items/items|#/items/*|type|level:number`,
			publicSchemaPointer + `/properties/items/items|#/items/*|type|level:string`,
			publicSchemaPointer + `/properties/items/items|#/items/*|type|level:array`,
			publicSchemaPointer + `/properties/items/items|#/items/*|type|level:object`,
			publicSchemaPointer + `/properties/items/items|#/items/*|anyOf|level:mask:1`,
			publicSchemaPointer + `/properties/items/items|#/items/*|anyOf|fault:anyOf`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|type|level:boolean`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|type|level:null`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|type|level:number`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|type|level:string`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|type|level:array`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|type|level:object`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|enum|level:member:0`,
			publicSchemaPointer + `/properties/items/items/anyOf/0|#/items/*|enum|fault:enum`,
			publicSchemaPointer + `/properties/left|#/left|type|level:boolean`,
			publicSchemaPointer + `/properties/left|#/left|type|fault:type`,
			publicSchemaPointer + `/properties/right|#/right|type|level:boolean`,
			publicSchemaPointer + `/properties/right|#/right|type|fault:type`,
			publicSchemaPointer + `/properties/server|#/server|type|level:boolean`,
			publicSchemaPointer + `/properties/server|#/server|type|fault:type`,
			publicSchemaPointer + `/properties/token|#/token|type|level:boolean`,
			publicSchemaPointer + `/properties/token|#/token|type|level:null`,
			publicSchemaPointer + `/properties/token|#/token|type|level:number`,
			publicSchemaPointer + `/properties/token|#/token|type|level:string`,
			publicSchemaPointer + `/properties/token|#/token|type|level:array`,
			publicSchemaPointer + `/properties/token|#/token|type|level:object`,
			publicSchemaPointer + `/properties/token|#/token|allOf|level:all-true`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|type|level:string`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|type|fault:type`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|minLength|level:valid`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|minLength|fault:minLength`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|maxLength|level:valid`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|maxLength|fault:maxLength`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|pattern|level:valid`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|pattern|fault:pattern`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|format|level:valid`,
			publicSchemaPointer + `/properties/token/allOf/0|#/token|format|fault:format`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|type|level:boolean`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|type|level:null`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|type|level:number`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|type|level:string`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|type|level:array`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|type|level:object`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|enum|level:member:0`,
			publicSchemaPointer + `/properties/token/allOf/1|#/token|enum|fault:enum`,
		},
	}

	for range 2 {
		cases, report, err := collectDeterministicRun(Input{
			OpenAPI: document, OperationID: "selected", MaxSteps: 10_000,
		}, nil)
		require.NoError(t, err)
		require.Equal(t, wantSufficientCases, cases)
		require.Equal(t, wantSufficientReport, report)

		cases, report, err = collectDeterministicRun(Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100}, nil)
		require.NoError(t, err)
		require.Empty(t, cases)
		require.Equal(t, wantBoundaryReport, report)
	}
}

// TestBuildSixtyFiveAllOfBranchesHasExactCutoffReport is supplementary public
// branch-count evidence; arbitrary-width anyOf masks use their sanctioned unit seam.
func TestBuildSixtyFiveAllOfBranchesHasExactCutoffReport(t *testing.T) {
	t.Parallel()

	branches := make([]string, 65)
	uncovered := []string{
		publicSchemaPointer + "|#|type|level:boolean",
		publicSchemaPointer + "|#|type|level:null",
		publicSchemaPointer + "|#|type|level:number",
		publicSchemaPointer + "|#|type|level:string",
		publicSchemaPointer + "|#|type|level:array",
		publicSchemaPointer + "|#|type|level:object",
		publicSchemaPointer + "|#|allOf|level:all-true",
	}

	for index := range branches {
		branches[index] = `{"type":"boolean"}`
		pointer := publicSchemaPointer + "/allOf/" + strconv.Itoa(index)
		uncovered = append(
			uncovered,
			pointer+"|#|type|level:boolean",
			pointer+"|#|type|fault:type",
		)
	}

	document := []byte(documentWithJSONSchema(`{"allOf":[` + strings.Join(branches, ",") + `]}`))
	wantReport := Report{Stop: MaxStepsReached, Steps: 1, Uncovered: uncovered}

	for range 2 {
		cases, report, err := collectDeterministicRun(Input{
			OpenAPI: document, OperationID: "selected", MaxSteps: 1,
		}, nil)
		require.NoError(t, err)
		require.Empty(t, cases)
		require.Equal(t, wantReport, report)
	}
}
