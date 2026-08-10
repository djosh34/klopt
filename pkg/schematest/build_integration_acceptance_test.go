package schematest

import (
	"math/big"
	"os"
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
	require.Equal(t, SpaceExhausted, firstReport.Stop)
	require.Equal(t, uint64(286), firstReport.Steps)
	require.Contains(t, firstReport.Covered, publicSchemaPointer+"|#|anyOf|level:mask:1")
	require.Contains(t, firstReport.Covered, publicSchemaPointer+"|#|anyOf|level:mask:2")
	require.Contains(t, firstReport.Uncovered, publicSchemaPointer+"|#|anyOf|level:mask:3")
	require.Contains(t, firstReport.Covered, publicSchemaPointer+"|#|anyOf|fault:anyOf")

	secondCases, secondReport, err := collectDeterministicRun(Input{
		OpenAPI: document, OperationID: "selected", MaxSteps: 1_000,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, firstCases, secondCases)
	require.Equal(t, firstReport, secondReport)
}

// TestBuildSkipsOpenFirstStructuralAndFaultAlternatives proves later viable alternatives remain reachable.
func TestBuildSkipsOpenFirstStructuralAndFaultAlternatives(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"anyOf":[
			{"type":"string","minLength":2,"maxLength":1},
			{"enum":[0]}
		]
	}`))
	cases, report, err := collectDeterministicRun(Input{
		OpenAPI: document, OperationID: "selected", MaxSteps: 1_000,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, []Case{
		{JSON: []byte(`0`), Valid: true},
		{JSON: []byte(`0`), Valid: true},
		{JSON: []byte(`false`), Valid: false},
	}, cases)
	require.Equal(t, MaxStepsReached, report.Stop)
	require.Equal(t, uint64(1_000), report.Steps)
	require.Contains(t, report.Covered, publicSchemaPointer+"|#|anyOf|level:mask:2")
	require.Contains(t, report.Covered, publicSchemaPointer+"|#|anyOf|fault:anyOf")
	require.Contains(t, report.Uncovered, publicSchemaPointer+"|#|anyOf|level:mask:1")
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
			require.Equal(t, MaxStepsReached, report.Stop)
			require.Equal(t, uint64(2), report.Steps)
			require.Empty(t, report.Covered)
			require.NotEmpty(t, report.Uncovered)
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
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Contains(t, cases, Case{
		JSON:  []byte(`{"__schematest_extra__":false,"__schematest_extra___1":false,"__schematest_extra___2":null}`),
		Valid: false,
	})
	require.Contains(t, report.Covered, publicSchemaPointer+"|#/*|additionalProperties|fault:additionalProperties")
}

// TestCompositionMasksAndReplayRemainWiderThanUint64 guards mask width in execution and source.
func TestCompositionMasksAndReplayRemainWiderThanUint64(t *testing.T) {
	t.Parallel()

	mask, exists := parentReplayMaskAtOrdinal(65, big.NewInt(64))
	require.True(t, exists)
	require.Equal(t, uint(1), mask.Bit(64))
	require.Equal(t, 65, mask.BitLen())

	var source strings.Builder

	for _, filename := range productionGoFiles(t) {
		contents, err := os.ReadFile(filename)
		require.NoError(t, err)
		_, err = source.Write(contents)
		require.NoError(t, err)
	}

	text := source.String()
	require.NotContains(t, text, "uint64(mask")
	require.NotContains(t, text, "mask.Uint64()")
	require.NotContains(t, text, "1 << len(node.anyOf)")
	require.Contains(t, strings.ReplaceAll(text, "\t", " "), "parentReplayMaskAtOrdinal")
}
