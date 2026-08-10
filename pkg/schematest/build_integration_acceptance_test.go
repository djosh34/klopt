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

// TestBuildBeyondCountSkipsUnusableFirstChildRank proves the scalar child
// cursor keeps a beyond-count projection live until a later usable rank.
func TestBuildBeyondCountSkipsUnusableFirstChildRank(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"anyOf":[
			{"type":"array","minItems":18446744073709551616,
			 "maxItems":18446744073709551616,
			 "items":{"type":"string","enum":[false,"ok"]}},
			{"enum":[0]}
		]
	}`))
	wantCases := []Case{{JSON: []byte(`0`), Valid: true}}
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
			publicSchemaPointer + `/anyOf/0/items|#/*|enum|level:member:1`,
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
		cases, report, err := collectDeterministicRun(Input{
			OpenAPI: document, OperationID: "selected", MaxSteps: 200,
		}, nil)
		require.NoError(t, err)
		require.Equal(t, wantCases, cases)
		require.Equal(t, wantReport, report)
	}
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

// TestBuildGenuinelyCompoundMatrixIsExact crosses every repaired module
// through complete public-Build runs plus one exact cutoff.
//
//nolint:maintidx // The exact matrix values intentionally remain visible at the public seam.
func TestBuildGenuinelyCompoundMatrixIsExact(t *testing.T) {
	t.Parallel()

	matrix := []struct {
		name     string
		document []byte
		cases    []Case
		report   Report
	}{
		{
			name: "repeated references, object, and request direction",
			document: []byte(`openapi: 3.0.4
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
`),
			cases: []Case{
				{JSON: []byte(`{"left":false,"right":false,"server":false}`), Valid: true},
				{JSON: []byte(`null`), Valid: false},
				{JSON: []byte(`{"__schematest_extra__":null,"left":false,"right":false}`), Valid: false},
				{JSON: []byte(`{"right":false}`), Valid: false},
				{JSON: []byte(`{"left":false}`), Valid: false},
				{JSON: []byte(`{"left":null,"right":false}`), Valid: false},
				{JSON: []byte(`{"left":false,"right":null}`), Valid: false},
				{JSON: []byte(`{"left":false,"right":false,"server":null}`), Valid: false},
			},
			report: Report{Stop: SpaceExhausted, Steps: 1_910, Covered: []string{
				publicSchemaPointer + `|#|type|level:object`,
				publicSchemaPointer + `|#|type|fault:type`,
				publicSchemaPointer + `|#/*|additionalProperties|fault:additionalProperties`,
				publicSchemaPointer + `|#/left|required|level:present`,
				publicSchemaPointer + `|#/left|required|fault:required`,
				publicSchemaPointer + `|#/right|required|level:present`,
				publicSchemaPointer + `|#/right|required|fault:required`,
				publicSchemaPointer + `/properties/left|#/left|type|level:boolean`,
				publicSchemaPointer + `/properties/left|#/left|type|fault:type`,
				publicSchemaPointer + `/properties/right|#/right|type|level:boolean`,
				publicSchemaPointer + `/properties/right|#/right|type|fault:type`,
				publicSchemaPointer + `/properties/server|#/server|type|level:boolean`,
				publicSchemaPointer + `/properties/server|#/server|type|fault:type`,
			}},
		},
		{
			name:     "exact number constraints",
			document: []byte(documentWithJSONSchema(`{"type":"number","minimum":2,"maximum":4,"multipleOf":2}`)),
			cases: []Case{
				{JSON: []byte(`2`), Valid: true},
				{JSON: []byte(`null`), Valid: false},
				{JSON: []byte(`0`), Valid: false},
				{JSON: []byte(`8`), Valid: false},
				{JSON: []byte(`3`), Valid: false},
			}, report: Report{
				Stop: SpaceExhausted, Steps: 4457,
				Covered: []string{
					publicSchemaPointer + `|#|type|level:number`,
					publicSchemaPointer + `|#|type|fault:type`,
					publicSchemaPointer + `|#|minimum|level:valid`,
					publicSchemaPointer + `|#|minimum|fault:minimum`,
					publicSchemaPointer + `|#|maximum|level:valid`,
					publicSchemaPointer + `|#|maximum|fault:maximum`,
					publicSchemaPointer + `|#|multipleOf|level:valid`,
					publicSchemaPointer + `|#|multipleOf|fault:multipleOf`,
				},
			},
		},
		{
			name:     "directed pattern",
			document: []byte(documentWithJSONSchema(`{"type":"string","pattern":"^A$"}`)),
			cases: []Case{
				{JSON: []byte(`"A"`), Valid: true},
				{JSON: []byte(`null`), Valid: false},
				{JSON: []byte(`""`), Valid: false},
			}, report: Report{
				Stop: SpaceExhausted, Steps: 12,
				Covered: []string{
					publicSchemaPointer + `|#|type|level:string`,
					publicSchemaPointer + `|#|type|fault:type`,
					publicSchemaPointer + `|#|pattern|level:valid`,
					publicSchemaPointer + `|#|pattern|fault:pattern`,
				},
			},
		},
		{
			name:     "directed format",
			document: []byte(documentWithJSONSchema(`{"type":"string","format":"byte"}`)),
			cases: []Case{
				{JSON: []byte(`""`), Valid: true},
				{JSON: []byte(`"+A=="`), Valid: true},
				{JSON: []byte(`"++0="`), Valid: true},
				{JSON: []byte(`null`), Valid: false},
				{JSON: []byte(`"\u0000"`), Valid: false},
			}, report: Report{
				Stop: SpaceExhausted, Steps: 66030,
				Covered: []string{
					publicSchemaPointer + `|#|type|level:string`,
					publicSchemaPointer + `|#|type|fault:type`,
					publicSchemaPointer + `|#|format|level:valid`,
					publicSchemaPointer + `|#|format|fault:format`,
				},
			},
		},
		{
			name:     "directed length",
			document: []byte(documentWithJSONSchema(`{"type":"string","maxLength":1}`)),
			cases: []Case{
				{JSON: []byte(`"\u0000"`), Valid: true},
				{JSON: []byte(`null`), Valid: false},
				{JSON: []byte(`"\u0000\u0000"`), Valid: false},
			}, report: Report{
				Stop: SpaceExhausted, Steps: 14,
				Covered: []string{
					publicSchemaPointer + `|#|type|level:string`,
					publicSchemaPointer + `|#|type|fault:type`,
					publicSchemaPointer + `|#|maxLength|level:valid`,
					publicSchemaPointer + `|#|maxLength|fault:maxLength`,
				},
			},
		},
		{
			name:     "array structure",
			document: []byte(documentWithJSONSchema(`{"type":"array","maxItems":1,"items":{"type":"boolean"}}`)),
			cases: []Case{
				{JSON: []byte(`[false]`), Valid: true},
				{JSON: []byte(`null`), Valid: false},
				{JSON: []byte(`[false,false]`), Valid: false},
				{JSON: []byte(`[null]`), Valid: false},
			}, report: Report{
				Stop: SpaceExhausted, Steps: 33,
				Covered: []string{
					publicSchemaPointer + `|#|type|level:array`,
					publicSchemaPointer + `|#|type|fault:type`,
					publicSchemaPointer + `|#|maxItems|level:valid`,
					publicSchemaPointer + `|#|maxItems|fault:maxItems`,
					publicSchemaPointer + `/items|#/*|type|level:boolean`,
					publicSchemaPointer + `/items|#/*|type|fault:type`,
				},
			},
		},
		{
			name:     "allOf composition",
			document: []byte(documentWithJSONSchema(`{"allOf":[{}]}`)),
			cases: []Case{
				{JSON: []byte(`false`), Valid: true},
				{JSON: []byte(`null`), Valid: true},
				{JSON: []byte(`0`), Valid: true},
				{JSON: []byte(`""`), Valid: true},
				{JSON: []byte(`[]`), Valid: true},
				{JSON: []byte(`{}`), Valid: true},
				{JSON: []byte(`null`), Valid: true},
				{JSON: []byte(`0`), Valid: true},
				{JSON: []byte(`""`), Valid: true},
				{JSON: []byte(`[]`), Valid: true},
				{JSON: []byte(`{}`), Valid: true},
			}, report: Report{
				Stop: SpaceExhausted, Steps: 71,
				Covered: []string{
					publicSchemaPointer + `|#|type|level:boolean`,
					publicSchemaPointer + `|#|type|level:null`,
					publicSchemaPointer + `|#|type|level:number`,
					publicSchemaPointer + `|#|type|level:string`,
					publicSchemaPointer + `|#|type|level:array`,
					publicSchemaPointer + `|#|type|level:object`,
					publicSchemaPointer + `|#|allOf|level:all-true`,
					publicSchemaPointer + `/allOf/0|#|type|level:boolean`,
					publicSchemaPointer + `/allOf/0|#|type|level:null`,
					publicSchemaPointer + `/allOf/0|#|type|level:number`,
					publicSchemaPointer + `/allOf/0|#|type|level:string`,
					publicSchemaPointer + `/allOf/0|#|type|level:array`,
					publicSchemaPointer + `/allOf/0|#|type|level:object`,
				},
			},
		},
		{
			name: "anyOf composition",
			document: []byte(documentWithJSONSchema(`{
				"anyOf":[
					{"enum":[false,null,0,"a",[],{}]},
					{"enum":[true,null,1,"b",[0],{"x":0}]}
				]
			}`)),
			cases: []Case{
				{JSON: []byte(`false`), Valid: true},
				{JSON: []byte(`0`), Valid: true},
				{JSON: []byte(`"a"`), Valid: true},
				{JSON: []byte(`[]`), Valid: true},
				{JSON: []byte(`{}`), Valid: true},
				{JSON: []byte(`true`), Valid: true},
				{JSON: []byte(`0`), Valid: true},
				{JSON: []byte(`"a"`), Valid: true},
				{JSON: []byte(`[]`), Valid: true},
				{JSON: []byte(`{}`), Valid: true},
				{JSON: []byte(`0`), Valid: true},
				{JSON: []byte(`"a"`), Valid: true},
				{JSON: []byte(`[]`), Valid: true},
				{JSON: []byte(`{}`), Valid: true},
				{JSON: []byte(`null`), Valid: true},
				{JSON: []byte(`1`), Valid: true},
				{JSON: []byte(`"b"`), Valid: true},
				{JSON: []byte(`[0]`), Valid: true},
				{JSON: []byte(`{"x":0}`), Valid: true},
				{JSON: []byte(`null`), Valid: true},
				{JSON: []byte(`1`), Valid: true},
				{JSON: []byte(`"b"`), Valid: true},
				{JSON: []byte(`[0]`), Valid: true},
				{JSON: []byte(`{"x":0}`), Valid: true},
				{JSON: []byte(`-1`), Valid: false},
				{JSON: []byte(`-1`), Valid: false},
				{JSON: []byte(`-1`), Valid: false},
			}, report: Report{
				Stop: SpaceExhausted, Steps: 1081,
				Covered: []string{
					publicSchemaPointer + `|#|type|level:boolean`,
					publicSchemaPointer + `|#|type|level:null`,
					publicSchemaPointer + `|#|type|level:number`,
					publicSchemaPointer + `|#|type|level:string`,
					publicSchemaPointer + `|#|type|level:array`,
					publicSchemaPointer + `|#|type|level:object`,
					publicSchemaPointer + `|#|anyOf|level:mask:1`,
					publicSchemaPointer + `|#|anyOf|level:mask:2`,
					publicSchemaPointer + `|#|anyOf|level:mask:3`,
					publicSchemaPointer + `|#|anyOf|fault:anyOf`,
					publicSchemaPointer + `/anyOf/0|#|type|level:boolean`,
					publicSchemaPointer + `/anyOf/0|#|type|level:null`,
					publicSchemaPointer + `/anyOf/0|#|type|level:number`,
					publicSchemaPointer + `/anyOf/0|#|type|level:string`,
					publicSchemaPointer + `/anyOf/0|#|type|level:array`,
					publicSchemaPointer + `/anyOf/0|#|type|level:object`,
					publicSchemaPointer + `/anyOf/0|#|enum|level:member:0`,
					publicSchemaPointer + `/anyOf/0|#|enum|level:member:1`,
					publicSchemaPointer + `/anyOf/0|#|enum|level:member:2`,
					publicSchemaPointer + `/anyOf/0|#|enum|level:member:3`,
					publicSchemaPointer + `/anyOf/0|#|enum|level:member:4`,
					publicSchemaPointer + `/anyOf/0|#|enum|level:member:5`,
					publicSchemaPointer + `/anyOf/0|#|enum|fault:enum`,
					publicSchemaPointer + `/anyOf/1|#|type|level:boolean`,
					publicSchemaPointer + `/anyOf/1|#|type|level:null`,
					publicSchemaPointer + `/anyOf/1|#|type|level:number`,
					publicSchemaPointer + `/anyOf/1|#|type|level:string`,
					publicSchemaPointer + `/anyOf/1|#|type|level:array`,
					publicSchemaPointer + `/anyOf/1|#|type|level:object`,
					publicSchemaPointer + `/anyOf/1|#|enum|level:member:0`,
					publicSchemaPointer + `/anyOf/1|#|enum|level:member:1`,
					publicSchemaPointer + `/anyOf/1|#|enum|level:member:2`,
					publicSchemaPointer + `/anyOf/1|#|enum|level:member:3`,
					publicSchemaPointer + `/anyOf/1|#|enum|level:member:4`,
					publicSchemaPointer + `/anyOf/1|#|enum|level:member:5`,
					publicSchemaPointer + `/anyOf/1|#|enum|fault:enum`,
				},
			},
		},
	}

	for _, test := range matrix {
		for range 2 {
			cases, report, err := collectDeterministicRun(Input{
				OpenAPI: test.document, OperationID: "selected", MaxSteps: 100_000,
			}, nil)
			require.NoError(t, err, test.name)
			require.Equal(t, test.cases, cases, test.name)
			require.Equal(t, test.report, report, test.name)
			require.Empty(t, report.Uncovered, test.name)
		}
	}

	wantBoundary := Report{Stop: MaxStepsReached, Steps: 1, Uncovered: []string{
		publicSchemaPointer + `|#|type|level:number`,
		publicSchemaPointer + `|#|type|fault:type`,
		publicSchemaPointer + `|#|minimum|level:valid`,
		publicSchemaPointer + `|#|minimum|fault:minimum`,
		publicSchemaPointer + `|#|maximum|level:valid`,
		publicSchemaPointer + `|#|maximum|fault:maximum`,
		publicSchemaPointer + `|#|multipleOf|level:valid`,
		publicSchemaPointer + `|#|multipleOf|fault:multipleOf`,
	}}

	for range 2 {
		cases, report, err := collectDeterministicRun(Input{
			OpenAPI: matrix[1].document, OperationID: "selected", MaxSteps: 1,
		}, nil)
		require.NoError(t, err)
		require.Empty(t, cases)
		require.Equal(t, wantBoundary, report)
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
