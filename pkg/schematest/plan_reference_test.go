//nolint:godoclint,lll // Reference tables pin occurrence identity without exposing planner types.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakePlanKeepsRepeatedReferenceUseSitesDistinct(t *testing.T) {
	t.Parallel()

	document := `{
		"openapi":"3.0.4",
		"components":{"schemas":{"Shared":{"type":"string","enum":["a","b"]}}},
		"paths":{
			"/":{
				"post":{
					"operationId":"selected",
					"requestBody":{
						"content":{
							"application/json":{
								"schema":{
									"type":"object",
									"properties":{
										"first":{"$ref":"#/components/schemas/Shared"},
										"second":{"$ref":"#/components/schemas/Shared"}
									}
								}
							}
						}
					}
				}
			}
		}
	}`

	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	ids := plan.validObligationIDs()
	require.Contains(t, ids, "#/paths/~1/post/requestBody/content/application~1json/schema/properties/first|#/first|type|level:string")
	require.Contains(t, ids, "#/paths/~1/post/requestBody/content/application~1json/schema/properties/second|#/second|type|level:string")
	require.Contains(t, ids, "#/paths/~1/post/requestBody/content/application~1json/schema/properties/first|#/first|enum|level:member:0")
	require.Contains(t, ids, "#/paths/~1/post/requestBody/content/application~1json/schema/properties/second|#/second|enum|level:member:0")

	first := model.root.properties["first"].occurrence
	second := model.root.properties["second"].occurrence

	require.True(t, first.reference)
	require.True(t, second.reference)
	require.Equal(t, "#/components/schemas/Shared", first.targetPointer)
	require.Equal(t, first.targetPointer, second.targetPointer)
}

func TestMakePlanIsStableForShuffledObjectConstruction(t *testing.T) {
	t.Parallel()

	first, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object",
		"properties":{"z":{"type":"number","minimum":2},"a":{"type":"string","pattern":"^a"}},
		"required":["z","a"],
		"additionalProperties":false
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	second, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"additionalProperties":false,
		"required":["a","z"],
		"properties":{"a":{"pattern":"^a","type":"string"},"z":{"minimum":2,"type":"number"}},
		"type":"object"
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	firstPlan, err := makePlan(first)
	require.NoError(t, err)
	secondPlan, err := makePlan(second)
	require.NoError(t, err)

	require.Equal(t, firstPlan.obligationIDs(), secondPlan.obligationIDs())
}
