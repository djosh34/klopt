//nolint:godoclint,lll // Complete JSON/YAML fixtures keep authored pointers visible beside expectations.
package schematest

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseInputAdmitsRepresentativeCorpus(t *testing.T) {
	t.Parallel()

	fixtures := []struct {
		path       string
		operations []string
	}{
		{path: "testdata/alpha_zeta.yaml", operations: []string{"alphaRequest", "zetaRequest"}},
		{path: "testdata/request_bodies.yaml", operations: []string{"referencedRequest"}},
		{path: "../../resources/openapi.yaml", operations: []string{
			"allOfObject",
			"anyOfBodyAndParameters",
			"arrayNotNullable",
			"arrayNullable",
			"compositeObject",
			"nullableObjectKeysAdditionalPropertiesFalse",
			"objectKeysAdditionalPropertiesFalse",
			"optionalArrayNullable",
			"refObject",
			"refStressObject",
			"refStressObjectPut",
			"stringNoFormatNotNullable",
			"stringNoFormatNullable",
		}},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.path, func(t *testing.T) {
			t.Parallel()

			document, err := os.ReadFile(fixture.path)
			require.NoError(t, err)

			for _, operation := range fixture.operations {
				model, parseErr := parseInput(Input{OpenAPI: document, OperationID: operation})
				require.NoError(t, parseErr, operation)
				require.NotNil(t, model.root)
			}
		})
	}
}

func TestParseInputRejectsMalformedJSONDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		document string
		message  string
	}{
		{
			name: "duplicate decoded key",
			document: `{
				"openapi":"3.0.4",
				"\u006fpenapi":"3.0.4",
				"paths":{}
			}`,
			message: "duplicate object member",
		},
		{
			name: "unpaired surrogate",
			document: `{
				"openapi":"3.0.4",
				"paths":{},
				"x-bad":"\uD800"
			}`,
			message: "surrogate",
		},
		{
			name: "trailing value", document: `{"openapi":"3.0.4","paths":{}} {}`,
			message: "trailing data",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(test.document), OperationID: "selected"})
			require.ErrorContains(t, err, test.message)
		})
	}
}

func TestParseInputAdmitsYAMLJSONSchemaPrimitivesAndAliases(t *testing.T) {
	t.Parallel()

	document := `
openapi: !!str 3.0.4
components:
  schemas:
    First: &shape {type: string}
    Second: &shape {type: integer}
    Selected: *shape
paths: !!map
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema: {$ref: '#/components/schemas/Selected'}
`

	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	require.Equal(t, schemaInteger, model.root.kind)
}

func TestParseInputAcceptsExplicitStringTaggedScalarMappingKey(t *testing.T) {
	t.Parallel()

	document := `openapi: 3.0.4
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                !!str 1: {type: string}
`

	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	require.Equal(t, schemaString, model.root.properties["1"].kind)
}

func TestParseInputRejectsYAMLOutsideJSONShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		document string
		message  string
	}{
		{
			name: "non JSON null spelling",
			document: `openapi: 3.0.4
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema: {default: ~}
`,
			message: "outside the YAML JSON schema",
		},
		{
			name: "custom tag",
			document: `openapi: 3.0.4
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema: {default: !custom value}
`,
			message: "outside the YAML JSON schema",
		},
		{
			name: "non string mapping key",
			document: `openapi: 3.0.4
paths:
  1: {}
`,
			message: "must be a scalar string",
		},
		{
			name: "tagged non scalar mapping key",
			document: `? !!str [openapi]
: 3.0.4
paths: {}
`,
			message: "invalid key",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(test.document), OperationID: "selected"})
			require.ErrorContains(t, err, test.message)
		})
	}
}

func TestParseInputResolvesLocalPathItemReference(t *testing.T) {
	t.Parallel()

	document := `{
		"openapi":"3.0.4",
		"x-path-item":{"post":{
			"operationId":"selected",
			"requestBody":{"content":{"application/json":{"schema":{"type":"string"}}}}
		}},
		"paths":{"/things":{"$ref":"#/x-path-item"}}
	}`

	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	require.Equal(t, schemaString, model.root.kind)
	require.Equal(t, "#/x-path-item/post/requestBody/content/application~1json/schema", model.root.occurrence.usePointer)
}

func TestParseInputRejectsUnsupportedPathItemReferenceForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		document string
		pointer  string
	}{
		{
			name:     "external",
			document: `{"openapi":"3.0.4","paths":{"/":{"$ref":"other.yaml"}}}`,
			pointer:  "#/paths/~1/$ref",
		},
		{
			name: "active sibling",
			document: `{
				"openapi":"3.0.4",
				"x-path-item":{},
				"paths":{"/":{"$ref":"#/x-path-item","post":{}}}
			}`,
			pointer: "#/paths/~1/$ref",
		},
		{
			name: "cycle",
			document: `{
				"openapi":"3.0.4",
				"x-a":{"$ref":"#/x-b"},
				"x-b":{"$ref":"#/x-a"},
				"paths":{"/":{"$ref":"#/x-a"}}
			}`,
			pointer: "#/x-b/$ref",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(test.document), OperationID: "selected"})
			require.ErrorContains(t, err, test.pointer)
		})
	}
}

func TestParseInputRejectsEquivalentTemplatedPaths(t *testing.T) {
	t.Parallel()

	documents := map[string]string{
		"json": `{"openapi":"3.0.4","paths":{"/pets/{id}":{},"/pets/{name}":{}}}`,
		"yaml": `openapi: 3.0.4
paths:
  /pets/{id}: {}
  /pets/{name}: {}
`,
	}

	for encoding, document := range documents {
		t.Run(encoding, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
			require.ErrorContains(t, err, "#/paths/~1pets~1{name}: templated path is identical to #/paths/~1pets~1{id}")
		})
	}
}

func TestParseInputSelectsJSONRequestSchema(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{
		OpenAPI: []byte(`{
			"openapi":"3.0.4",
			"paths":{
				"/things":{
					"post":{
						"operationId":"createThing",
						"requestBody":{"content":{"application/json":{"schema":{"type":"string"}}}}
					}
				}
			}
		}`),
		OperationID: "createThing",
	})
	require.NoError(t, err)
	require.NotNil(t, model)
	require.NotNil(t, model.root)
	require.Equal(t, schemaString, model.root.kind)
	require.Equal(t, "#/paths/~1things/post/requestBody/content/application~1json/schema", model.root.occurrence.usePointer)
	require.Equal(t, model.root.occurrence.usePointer, model.root.occurrence.targetPointer)
	require.Equal(t, "#", model.root.occurrence.instanceTemplate)
}

func TestParseInputAcceptsFlowStyleYAML(t *testing.T) {
	t.Parallel()

	document := `{openapi: 3.0.4, paths: {/: {post: {operationId: selected, requestBody: {content: {
  application/json: {schema: {type: string}}
}}}}}}`

	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	require.Equal(t, schemaString, model.root.kind)
}

func TestParseInputJSONYAMLSelectionParity(t *testing.T) {
	t.Parallel()

	documents := map[string]string{
		"json": `{"openapi":"3.0.4","paths":{"/things":{"post":{"operationId":"createThing","requestBody":{"content":{"application/json":{"schema":{"type":"string"}}}}}}}}`,
		"yaml": `openapi: 3.0.4
paths:
  /things:
    post:
      operationId: createThing
      requestBody:
        content:
          application/json:
            schema:
              type: string
`,
	}

	for name, document := range documents {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "createThing"})
			require.NoError(t, err)
			require.Equal(t, schemaString, model.root.kind)
			require.Equal(t, "#/paths/~1things/post/requestBody/content/application~1json/schema", model.root.occurrence.usePointer)
		})
	}
}

func TestParseInputSelectsMostSpecificJSONMediaType(t *testing.T) {
	t.Parallel()

	documents := map[string]string{
		"json": `{"openapi":"3.0.4","paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"*/*":{"schema":{"type":"string"}},"application/*":{"schema":{"type":"number"}},"Application/JSON; charset=utf-8":{"schema":{"type":"boolean"}}}}}}}}`,
		"yaml": `openapi: 3.0.4
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          "*/*": {schema: {type: string}}
          "application/*": {schema: {type: number}}
          "Application/JSON; charset=utf-8": {schema: {type: boolean}}
`,
	}

	for encoding, document := range documents {
		t.Run(encoding, func(t *testing.T) {
			t.Parallel()

			model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
			require.NoError(t, err)
			require.Equal(t, schemaBoolean, model.root.kind)
			require.Contains(t, model.root.occurrence.usePointer, "Application~1JSON; charset=utf-8")
		})
	}
}

func TestParseInputRejectsInvalidContentMediaTypeKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		json string
		yaml string
	}{
		{name: "missing subtype", json: `"application"`, yaml: `application`},
		{name: "invalid wildcard placement", json: `"*/json"`, yaml: `"*/json"`},
	}

	for _, test := range tests {
		for encoding, document := range map[string]string{
			"json": fmt.Sprintf(
				`{"openapi":"3.0.4","paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{%s:{},"application/json":{"schema":{}}}}}}}}`,
				test.json,
			),
			"yaml": `openapi: 3.0.4
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          ` + test.yaml + `: {}
          application/json: {schema: {}}
`,
		} {
			t.Run(test.name+"/"+encoding, func(t *testing.T) {
				t.Parallel()

				_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
				require.ErrorContains(t, err, "must be a valid media type or media range")
			})
		}
	}
}

func TestParseInputModelsEveryAdmittedKindWithJSONYAMLParity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		json string
		yaml string
		kind schemaKind
	}{
		{name: "typeless", json: `{}`, yaml: `{}`, kind: schemaAny},
		{name: "boolean", json: `{"type":"boolean"}`, yaml: `type: boolean`, kind: schemaBoolean},
		{name: "integer", json: `{"type":"integer"}`, yaml: `type: integer`, kind: schemaInteger},
		{name: "number", json: `{"type":"number"}`, yaml: `type: number`, kind: schemaNumber},
		{name: "string", json: `{"type":"string"}`, yaml: `type: string`, kind: schemaString},
		{
			name: "array",
			json: `{"type":"array","items":{}}`,
			yaml: "type: array\nitems: {}",
			kind: schemaArray,
		},
		{name: "object", json: `{"type":"object"}`, yaml: `type: object`, kind: schemaObject},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for encoding, document := range map[string]string{
				"json": documentWithJSONSchema(test.json),
				"yaml": documentWithYAMLSchema(test.yaml),
			} {
				t.Run(encoding, func(t *testing.T) {
					t.Parallel()

					model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
					require.NoError(t, err)
					require.Equal(t, test.kind, model.root.kind)
				})
			}
		})
	}
}

func TestParseInputResolvesLocalSchemaReferenceWithOccurrenceMetadata(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{
		OpenAPI: []byte(`{
			"openapi":"3.0.4",
			"components":{"schemas":{"Text":{"type":"string"}}},
			"paths":{"/things":{"post":{
				"operationId":"createThing",
				"requestBody":{"content":{"application/json":{"schema":{
					"$ref":"#/components/schemas/Text",
					"type":"number",
					"discriminator":{"propertyName":"ignoredReferenceSibling"}
				}}}}
			}}}
		}`),
		OperationID: "createThing",
	})
	require.NoError(t, err)
	require.Equal(t, schemaString, model.root.kind)
	require.Equal(t, "#/paths/~1things/post/requestBody/content/application~1json/schema", model.root.occurrence.usePointer)
	require.Equal(t, "#/components/schemas/Text", model.root.occurrence.targetPointer)
	require.Equal(t, "#", model.root.occurrence.instanceTemplate)
}

func TestParseInputSharesRepeatedReferenceTargetStructure(t *testing.T) {
	t.Parallel()

	document := `{
		"openapi":"3.0.4",
		"components":{"schemas":{
			"A":{"type":"string"},
			"B":{"allOf":[{"$ref":"#/components/schemas/A"},{"$ref":"#/components/schemas/A"}]},
			"C":{"allOf":[{"$ref":"#/components/schemas/B"},{"$ref":"#/components/schemas/B"}]}
		}},
		"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
			"$ref":"#/components/schemas/C"
		}}}}}}}
	}`

	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	require.NotSame(t, model.root.allOf[0], model.root.allOf[1])
	require.Same(t, model.root.allOf[0].allOf[0], model.root.allOf[1].allOf[0])
}

func TestParseInputSharesInlineYAMLAliasSchemaShapes(t *testing.T) {
	t.Parallel()

	anchors := "x-schema-0: &s0 {type: string}\n"
	for depth := 1; depth <= 24; depth++ {
		anchors += fmt.Sprintf("x-schema-%d: &s%d {allOf: [*s%d, *s%d]}\n", depth, depth, depth-1, depth-1)
	}

	document := "openapi: 3.0.4\n" + anchors + `paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema: *s24
`

	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	require.Same(t, model.root.allOf[0].schemaShape, model.root.allOf[1].schemaShape)
}

func TestParseInputKeepsAliasedSchemaDescendantTemplatesRelative(t *testing.T) {
	t.Parallel()

	document := `openapi: 3.0.4
x-shared: &shared
  type: object
  properties:
    child: {type: string}
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                a: *shared
                b: *shared
`

	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	require.Equal(t, "#/a", model.root.properties["a"].occurrence.instanceTemplate)
	require.Equal(t, "#/b", model.root.properties["b"].occurrence.instanceTemplate)
	require.Same(t, model.root.properties["a"].schemaShape, model.root.properties["b"].schemaShape)
	require.Equal(t, "#/child", model.root.properties["a"].properties["child"].occurrence.instanceTemplate)
}

func TestParseInputSharesReferenceTargetsAcrossInstanceTemplates(t *testing.T) {
	t.Parallel()

	document := `{
		"openapi":"3.0.4",
		"components":{"schemas":{
			"A":{"type":"string"},
			"B":{"type":"object","properties":{
				"a":{"$ref":"#/components/schemas/A"},
				"b":{"$ref":"#/components/schemas/A"}
			}},
			"C":{"type":"object","properties":{
				"a":{"$ref":"#/components/schemas/B"},
				"b":{"$ref":"#/components/schemas/B"}
			}}
		}},
		"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
			"$ref":"#/components/schemas/C"
		}}}}}}}
	}`

	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	require.NotSame(t, model.root.properties["a"], model.root.properties["b"])
	require.Equal(t, "#/a", model.root.properties["a"].occurrence.instanceTemplate)
	require.Equal(t, "#/b", model.root.properties["b"].occurrence.instanceTemplate)
	require.Same(t, model.root.properties["a"].properties["a"], model.root.properties["b"].properties["a"])
}

func TestParseInputPreservesNestedReferenceUseTargetAndInstanceMetadata(t *testing.T) {
	t.Parallel()

	document := `{
		"openapi":"3.0.4",
		"components":{"schemas":{"Value":{"type":"integer"}}},
		"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
			"type":"object",
			"properties":{"a/b":{"$ref":"#/components/schemas/Value"}}
		}}}}}}}
	}`

	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)

	property := model.root.properties["a/b"]
	require.Equal(t, "#/paths/~1/post/requestBody/content/application~1json/schema/properties/a~1b", property.occurrence.usePointer)
	require.Equal(t, "#/components/schemas/Value", property.occurrence.targetPointer)
	require.Equal(t, "#/a~1b", property.occurrence.instanceTemplate)
}

func TestParseInputResolvesReferencedRequestBody(t *testing.T) {
	t.Parallel()

	documents := map[string]string{
		"json": `{
			"openapi":"3.0.4",
			"components":{"requestBodies":{"Payload":{"content":{"application/json":{"schema":{"type":"string"}}}}}},
			"paths":{"/things":{"post":{
				"operationId":"createThing",
				"requestBody":{"$ref":"#/components/requestBodies/Payload","content":"ignored sibling"}
			}}}
		}`,
		"yaml": `openapi: 3.0.4
components:
  requestBodies:
    Payload:
      content:
        application/json:
          schema: {type: string}
paths:
  /things:
    post:
      operationId: createThing
      requestBody:
        $ref: '#/components/requestBodies/Payload'
        content: ignored sibling
`,
	}

	for encoding, document := range documents {
		t.Run(encoding, func(t *testing.T) {
			t.Parallel()

			model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "createThing"})
			require.NoError(t, err)
			require.Equal(t, schemaString, model.root.kind)
			require.Equal(
				t,
				"#/components/requestBodies/Payload/content/application~1json/schema",
				model.root.occurrence.usePointer,
			)
		})
	}
}

func TestParseInputRejectsRecursiveRequestBodyReferences(t *testing.T) {
	t.Parallel()

	documents := map[string]string{
		"json": `{
			"openapi":"3.0.4",
			"components":{"requestBodies":{
				"A":{"$ref":"#/components/requestBodies/B"},
				"B":{"$ref":"#/components/requestBodies/A"}
			}},
			"paths":{"/":{"post":{
				"operationId":"selected",
				"requestBody":{"$ref":"#/components/requestBodies/A"}
			}}}
		}`,
		"yaml": `openapi: 3.0.4
components:
  requestBodies:
    A: {$ref: '#/components/requestBodies/B'}
    B: {$ref: '#/components/requestBodies/A'}
paths:
  /:
    post:
      operationId: selected
      requestBody: {$ref: '#/components/requestBodies/A'}
`,
	}

	for encoding, document := range documents {
		t.Run(encoding, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
			require.ErrorContains(t, err, "#/components/requestBodies/B/$ref")
			require.ErrorContains(t, err, "outside the Klopt profile")
		})
	}
}

func TestParseInputModelsExactNumberEnumAndNullable(t *testing.T) {
	t.Parallel()

	documents := map[string]string{
		"json": `{"openapi":"3.0.4","paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{"type":"number","nullable":true,"enum":[1,1.0,null,{"a":1,"b":[true]},{"b":[true],"a":1.0}],"minimum":0.10,"exclusiveMinimum":true,"maximum":2e0,"exclusiveMaximum":false,"multipleOf":0.05,"format":"double","default":1}}}}}}}}`,
		"yaml": `openapi: 3.0.4
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema:
              type: number
              nullable: true
              enum: [1, 1.0, null, {a: 1, b: [true]}, {b: [true], a: 1.0}]
              minimum: 0.10
              exclusiveMinimum: true
              maximum: 2e0
              exclusiveMaximum: false
              multipleOf: 0.05
              format: double
              default: 1
`,
	}

	for name, document := range documents {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
			require.NoError(t, err)
			require.Equal(t, schemaNumber, model.root.kind)
			require.True(t, model.root.nullable)
			require.Len(t, model.root.enum, 3)
			require.Equal(t, schemaFormatDouble, model.root.format)
			require.True(t, model.root.exclusiveMinimum)
			require.False(t, model.root.exclusiveMaximum)
			requireExactNumberEqual(t, "0.1", model.root.minimum)
			requireExactNumberEqual(t, "2", model.root.maximum)
			requireExactNumberEqual(t, "0.05", model.root.multipleOf)
			require.Equal(t, jsonNumber, model.root.defaultValue.kind)
			requireExactNumberEqual(t, "1", model.root.defaultValue.number)
		})
	}
}

func TestParseInputDeduplicatesLargeEnumInLinearWork(t *testing.T) {
	t.Parallel()

	members := make([]string, 20_000)
	for index := range members {
		members[index] = fmt.Sprintf("%q", fmt.Sprintf("member-%d", index))
	}

	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"enum":[` + strings.Join(members, ",") + `]}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)
	require.Len(t, model.root.enum, len(members))
}

func TestParseInputDeduplicatesEnumAliasDAGWithoutExpansion(t *testing.T) {
	t.Parallel()

	anchors := "x-anchor-0: &a0 [0]\n"
	for depth := 1; depth <= 24; depth++ {
		anchors += fmt.Sprintf("x-anchor-%d: &a%d [*a%d, *a%d]\n", depth, depth, depth-1, depth-1)
	}

	document := "openapi: 3.0.4\n" + anchors + `paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema:
              enum: [*a24, *a24]
`

	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	require.Len(t, model.root.enum, 1)
}

func TestParseInputModelsNullableOnlyWithSameObjectType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		json     string
		yaml     string
		nullable bool
	}{
		{name: "typeless", json: `{"nullable":true}`, yaml: "nullable: true"},
		{
			name: "explicit type", json: `{"type":"string","nullable":true}`,
			yaml: "type: string\nnullable: true", nullable: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for encoding, document := range map[string]string{
				"json": documentWithJSONSchema(test.json),
				"yaml": documentWithYAMLSchema(test.yaml),
			} {
				t.Run(encoding, func(t *testing.T) {
					t.Parallel()

					model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
					require.NoError(t, err)
					require.Equal(t, test.nullable, model.root.nullable)
				})
			}
		})
	}
}

func TestParseInputPreservesExactAuthoredNumberScale(t *testing.T) {
	t.Parallel()

	for encoding, document := range map[string]string{
		"json": documentWithJSONSchema(`{"type":"number","minimum":1e-100001}`),
		"yaml": documentWithYAMLSchema("type: number\nminimum: 1e-100001"),
	} {
		t.Run(encoding, func(t *testing.T) {
			t.Parallel()

			model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
			require.NoError(t, err)
			requireExactNumberEqual(t, "1e-100001", model.root.minimum)
		})
	}
}

func TestParseInputModelsLockedFormatMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		typeName string
		format   string
		expected schemaFormat
	}{
		{name: "typeless int32", format: "int32", expected: schemaFormatInt32},
		{name: "typeless float", format: "float", expected: schemaFormatFloat},
		{name: "typeless email", format: "email", expected: schemaFormatEmail},
		{name: "integer int32", typeName: "integer", format: "int32", expected: schemaFormatInt32},
		{name: "integer int64", typeName: "integer", format: "int64", expected: schemaFormatInt64},
		{name: "number int32", typeName: "number", format: "int32", expected: schemaFormatInt32},
		{name: "number int64", typeName: "number", format: "int64", expected: schemaFormatInt64},
		{name: "number float", typeName: "number", format: "float", expected: schemaFormatFloat},
		{name: "number double", typeName: "number", format: "double", expected: schemaFormatDouble},
		{name: "string byte", typeName: "string", format: "byte", expected: schemaFormatByte},
		{name: "string date", typeName: "string", format: "date", expected: schemaFormatDate},
		{name: "string date-time", typeName: "string", format: "date-time", expected: schemaFormatDateTime},
		{name: "string email", typeName: "string", format: "email", expected: schemaFormatEmail},
		{name: "string ipv4", typeName: "string", format: "ipv4", expected: schemaFormatIPv4},
		{name: "string uuid", typeName: "string", format: "uuid", expected: schemaFormatUUID},
		{name: "string uuidv4", typeName: "string", format: "uuidv4", expected: schemaFormatUUIDv4},
		{name: "string uuid-v4", typeName: "string", format: "uuid-v4", expected: schemaFormatUUIDDashV4},
		{name: "string cidr", typeName: "string", format: "cidr", expected: schemaFormatCIDR},
		{name: "string ipv4-cidr", typeName: "string", format: "ipv4-cidr", expected: schemaFormatIPv4CIDR},
		{name: "string password", typeName: "string", format: "password", expected: schemaFormatPassword},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			typeJSON := ""
			typeYAML := ""

			if test.typeName != "" {
				typeJSON = `"type":"` + test.typeName + `",`
				typeYAML = "type: " + test.typeName + "\n"
			}

			for encoding, document := range map[string]string{
				"json": documentWithJSONSchema(`{` + typeJSON + `"format":"` + test.format + `"}`),
				"yaml": documentWithYAMLSchema(typeYAML + "format: " + test.format),
			} {
				t.Run(encoding, func(t *testing.T) {
					t.Parallel()

					model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
					require.NoError(t, err)
					require.Equal(t, test.expected, model.root.format)
				})
			}
		})
	}
}

func TestParseInputModelsArraysObjectsCompositionAndRequestDirection(t *testing.T) {
	t.Parallel()

	documents := map[string]string{
		"json": `{"openapi":"3.0.4","paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{"type":"object","minProperties":1,"maxProperties":5,"required":["visible","server"],"properties":{"visible":{"type":"array","minItems":1,"maxItems":2,"items":{"type":"string"}},"server":{"type":"string","readOnly":true},"secret":{"type":"string","writeOnly":true}},"additionalProperties":{"type":"integer"},"allOf":[{"nullable":false}],"anyOf":[{"type":"object"},{"type":"object"}]}}}}}}}}`,
		"yaml": `openapi: 3.0.4
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema:
              type: object
              minProperties: 1
              maxProperties: 5
              required: [visible, server]
              properties:
                visible:
                  type: array
                  minItems: 1
                  maxItems: 2
                  items: {type: string}
                server: {type: string, readOnly: true}
                secret: {type: string, writeOnly: true}
              additionalProperties: {type: integer}
              allOf: [{nullable: false}]
              anyOf: [{type: object}, {type: object}]
`,
	}

	for name, document := range documents {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
			require.NoError(t, err)
			require.Equal(t, schemaObject, model.root.kind)
			require.Equal(t, []string{"visible"}, model.root.required)
			require.Len(t, model.root.properties, 3)
			require.True(t, model.root.properties["server"].readOnly)
			require.True(t, model.root.properties["secret"].writeOnly)
			require.Equal(t, "#/visible", model.root.properties["visible"].occurrence.instanceTemplate)
			require.Equal(t, schemaArray, model.root.properties["visible"].kind)
			require.Equal(t, "#/*", model.root.properties["visible"].items.occurrence.instanceTemplate)
			require.NotNil(t, model.root.additionalProperties)
			require.True(t, model.root.allowAdditionalProperties)
			require.Len(t, model.root.allOf, 1)
			require.Len(t, model.root.anyOf, 2)
			requireCountEqual(t, "1", model.root.minProperties)
			requireCountEqual(t, "5", model.root.maxProperties)
			requireCountEqual(t, "1", model.root.properties["visible"].minItems)
			requireCountEqual(t, "2", model.root.properties["visible"].maxItems)
		})
	}
}

func TestParseInputAcceptsRelativeExternalDocumentationURL(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"externalDocs":{"url":"../schema-docs"}}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)
	require.NotNil(t, model.root)
}

func TestParseInputIgnoresPathsSpecificationExtensions(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{
		OpenAPI: []byte(`{
			"openapi":"3.0.4",
			"paths":{
				"x-paths-note":"inert",
				"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{}}}}}}
			}
		}`),
		OperationID: "selected",
	})
	require.NoError(t, err)
	require.NotNil(t, model.root)
}

func TestParseInputAcceptsAndDiscardsWellFormedInertMetadata(t *testing.T) {
	t.Parallel()

	document := `{
		"openapi":"3.0.4",
		"components":{"schemas":{"Unused":{"description":"inert"}}},
		"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
			"title":"Title",
			"description":"Description",
			"default":{"exact":0.100},
			"example":[1],
			"deprecated":true,
			"externalDocs":{"description":"Docs","url":"https://example.test/docs","x-note":1},
			"xml":{"name":"item","namespace":"https://example.test/xml#namespace","prefix":"e","attribute":false,"wrapped":true,"x-note":1},
			"x-extension":{"anything":true}
		}}}}}}}
	}`

	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	require.Equal(t, schemaAny, model.root.kind)
}

func TestBuildRejectsDocumentWideDiscriminatorBeforeSelectedSchemaCompilation(t *testing.T) {
	t.Parallel()

	document := `{
		"openapi":"3.0.4",
		"components":{"schemas":{"Unused":{"discriminator":{"propertyName":"kind"}}}},
		"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{"type":1}}}}}}}
	}`

	report, err := Build(Input{OpenAPI: []byte(document), OperationID: "selected", MaxSteps: 0}, func(Case) error {
		return nil
	})
	require.Equal(t, Report{}, report)
	require.EqualError(
		t,
		err,
		"#/components/schemas/Unused/discriminator: authored discriminator is outside the Klopt profile",
	)
}

func TestBuildRejectsDocumentWideReferenceExclusionsBeforeSelectedSchemaCompilation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		components string
		pointer    string
	}{
		{
			name:       "external",
			components: `{"Unused":{"$ref":"other.yaml#/Thing"}}`,
			pointer:    "#/components/schemas/Unused/$ref",
		},
		{
			name:       "cycle",
			components: `{"A":{"$ref":"#/components/schemas/B"},"B":{"$ref":"#/components/schemas/A"}}`,
			pointer:    "#/components/schemas/A/$ref",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			document := `{"openapi":"3.0.4","components":{"schemas":` + test.components +
				`},"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":` +
				`{"schema":{"type":1}}}}}}}}`

			_, err := Build(Input{OpenAPI: []byte(document), OperationID: "selected", MaxSteps: 0}, func(Case) error {
				return nil
			})
			require.ErrorContains(t, err, test.pointer)
		})
	}
}

func TestBuildKeepsOneOfAttributionAheadOfAdjacentDiscriminator(t *testing.T) {
	t.Parallel()

	_, err := Build(Input{
		OpenAPI: []byte(documentWithJSONSchema(
			`{"oneOf":[{}],"discriminator":{"propertyName":"kind"}}`,
		)),
		OperationID: "selected",
		MaxSteps:    0,
	}, func(Case) error { return nil })
	require.EqualError(
		t,
		err,
		"#/paths/~1/post/requestBody/content/application~1json/schema/oneOf: "+
			"authored oneOf is outside the Klopt profile",
	)
}

func TestParseInputRejectsMalformedSchemaShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		pointer string
	}{
		{name: "type", schema: `{"type":1}`, pointer: "/type"},
		{name: "nullable", schema: `{"nullable":1}`, pointer: "/nullable"},
		{name: "minimum", schema: `{"minimum":"0"}`, pointer: "/minimum"},
		{name: "exclusive", schema: `{"exclusiveMaximum":1}`, pointer: "/exclusiveMaximum"},
		{name: "multiple zero", schema: `{"multipleOf":0}`, pointer: "/multipleOf"},
		{name: "length fractional", schema: `{"minLength":1.5}`, pointer: "/minLength"},
		{name: "pattern", schema: `{"pattern":true}`, pointer: "/pattern"},
		{name: "array items missing", schema: `{"type":"array"}`, pointer: "/items"},
		{name: "array items shape", schema: `{"items":[]}`, pointer: "/items"},
		{name: "negative item count", schema: `{"minItems":-1}`, pointer: "/minItems"},
		{name: "properties", schema: `{"properties":[]}`, pointer: "/properties"},
		{name: "required empty", schema: `{"required":[]}`, pointer: "/required"},
		{name: "required duplicate", schema: `{"required":["x","x"]}`, pointer: "/required/1"},
		{name: "additionalProperties", schema: `{"additionalProperties":1}`, pointer: "/additionalProperties"},
		{name: "allOf empty", schema: `{"allOf":[]}`, pointer: "/allOf"},
		{name: "anyOf shape", schema: `{"anyOf":{}}`, pointer: "/anyOf"},
		{name: "default type", schema: `{"type":"string","default":1}`, pointer: "/default"},
		{name: "request direction", schema: `{"properties":{"x":{"readOnly":true,"writeOnly":true}}}`, pointer: "/properties/x/writeOnly"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(test.schema)), OperationID: "selected"})
			require.Error(t, err)
			require.Contains(t, err.Error(), "#/paths/~1/post/requestBody/content/application~1json/schema"+test.pointer)
		})
	}
}

func TestParseInputShapeChecksInertMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		pointer string
	}{
		{name: "title", schema: `{"title":1}`, pointer: "/title"},
		{name: "deprecated", schema: `{"deprecated":"yes"}`, pointer: "/deprecated"},
		{name: "externalDocs_missing_url", schema: `{"externalDocs":{"description":"missing"}}`, pointer: "/externalDocs/url"},
		{name: "externalDocs_empty_url", schema: `{"externalDocs":{"url":""}}`, pointer: "/externalDocs/url"},
		{name: "externalDocs_invalid_url", schema: `{"externalDocs":{"url":"https://example.test/a|b"}}`, pointer: "/externalDocs/url"},
		{name: "externalDocs_invalid_path_character", schema: `{"externalDocs":{"url":"https://example.test/a[b"}}`, pointer: "/externalDocs/url"},
		{name: "xml_object", schema: `{"xml":"item"}`, pointer: "/xml"},
		{name: "xml_field", schema: `{"xml":{"wrapped":"yes"}}`, pointer: "/xml/wrapped"},
		{name: "xml_namespace", schema: `{"xml":{"namespace":"relative/path"}}`, pointer: "/xml/namespace"},
		{name: "xml_namespace_invalid_URI", schema: `{"xml":{"namespace":"https://example.test/a|b"}}`, pointer: "/xml/namespace"},
		{name: "xml_namespace_invalid_path_character", schema: `{"xml":{"namespace":"https://example.test/a[b"}}`, pointer: "/xml/namespace"},
		{name: "xml_unknown", schema: `{"xml":{"role":"semantic"}}`, pointer: "/xml/role"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(test.schema)), OperationID: "selected"})
			require.Error(t, err)
			require.Contains(t, err.Error(), "#/paths/~1/post/requestBody/content/application~1json/schema"+test.pointer)
		})
	}
}

func TestParseInputRejectsExcludedSchemaCapabilitiesAtAuthoredPointers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		yaml    string
		pointer string
	}{
		{name: "oneOf", json: `{"oneOf":[{}]}`, yaml: "oneOf: [{}]", pointer: "/oneOf"},
		{name: "not", json: `{"not":{}}`, yaml: "not: {}", pointer: "/not"},
		{name: "discriminator", json: `{"discriminator":{"propertyName":"kind"}}`, yaml: "discriminator: {propertyName: kind}", pointer: "/discriminator"},
		{name: "uniqueItems_false", json: `{"uniqueItems":false}`, yaml: "uniqueItems: false", pointer: "/uniqueItems"},
		{name: "empty_enum", json: `{"enum":[]}`, yaml: "enum: []", pointer: "/enum"},
		{name: "unknown_keyword", json: `{"const":1}`, yaml: "const: 1", pointer: "/const"},
		{name: "unknown_format", json: `{"type":"string","format":"hostname"}`, yaml: "type: string\nformat: hostname", pointer: "/format"},
		{name: "excluded_OAS_binary_format", json: `{"type":"string","format":"binary"}`, yaml: "type: string\nformat: binary", pointer: "/format"},
		{name: "boolean_string_format", json: `{"type":"boolean","format":"email"}`, yaml: "type: boolean\nformat: email", pointer: "/format"},
		{name: "string_numeric_format", json: `{"type":"string","format":"int32"}`, yaml: "type: string\nformat: int32", pointer: "/format"},
		{name: "integer_float_format", json: `{"type":"integer","format":"double"}`, yaml: "type: integer\nformat: double", pointer: "/format"},
		{name: "number_string_format", json: `{"type":"number","format":"uuid"}`, yaml: "type: number\nformat: uuid", pointer: "/format"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for encoding, document := range map[string]string{
				"json": documentWithJSONSchema(test.json),
				"yaml": documentWithYAMLSchema(test.yaml),
			} {
				t.Run(encoding, func(t *testing.T) {
					t.Parallel()

					_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
					require.Error(t, err)
					require.Contains(t, err.Error(), "#/paths/~1/post/requestBody/content/application~1json/schema"+test.pointer)
				})
			}
		})
	}
}

func documentWithJSONSchema(schema string) string {
	return `{"openapi":"3.0.4","paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":` + schema + `}}}}}}}`
}

func documentWithYAMLSchema(schema string) string {
	return "openapi: 3.0.4\npaths:\n  /:\n    post:\n      operationId: selected\n      requestBody:\n        content:\n          application/json:\n            schema:\n" + indentLines(schema, "              ") + "\n"
}

func indentLines(source, prefix string) string {
	result := prefix
	for _, character := range source {
		result += string(character)
		if character == '\n' {
			result += prefix
		}
	}

	return result
}

func requireExactNumberEqual(t *testing.T, expected string, actual *exactNumber) {
	t.Helper()

	number, err := parseExactNumber(expected)
	require.NoError(t, err)

	comparison, err := number.compare(actual)
	require.NoError(t, err)
	require.Zero(t, comparison)
}

func requireCountEqual(t *testing.T, expected string, actual *exactCount) {
	t.Helper()
	require.NotNil(t, actual)
	require.Equal(t, expected, actual.String())
}

func TestParseInputReportsOperationAndBodySelectionErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		document  string
		operation string
		message   string
	}{
		{
			name:      "missing operation",
			document:  `{"openapi":"3.0.4","paths":{}}`,
			operation: "missing",
			message:   `operationId "missing" was not found`,
		},
		{
			name:      "missing body",
			document:  `{"openapi":"3.0.4","paths":{"/":{"post":{"operationId":"selected"}}}}`,
			operation: "selected",
			message:   "/requestBody",
		},
		{
			name:      "missing JSON media",
			document:  `{"openapi":"3.0.4","paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"text/plain":{}}}}}}}`,
			operation: "selected",
			message:   "/application~1json",
		},
		{
			name:      "missing schema",
			document:  `{"openapi":"3.0.4","paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{}}}}}}}`,
			operation: "selected",
			message:   "/schema",
		},
		{
			name: "duplicate operation ID",
			document: `{"openapi":"3.0.4","paths":{
				"/a":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{}}}}}},
				"/b":{"put":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{}}}}}}
			}}`,
			operation: "selected",
			message:   "must be unique",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(test.document), OperationID: test.operation})
			require.ErrorContains(t, err, test.message)
		})
	}
}

func TestParseInputRejectsNonPathPathsField(t *testing.T) {
	t.Parallel()

	_, err := parseInput(Input{
		OpenAPI: []byte(`{
			"openapi":"3.0.4",
			"paths":{"things":{"post":{
				"operationId":"selected",
				"requestBody":{"content":{"application/json":{"schema":{}}}}
			}}}
		}`),
		OperationID: "selected",
	})
	require.ErrorContains(t, err, "#/paths/things")
	require.ErrorContains(t, err, "must begin with '/'")
}

func TestParseInputRejectsMalformedSelectedRequestBodyFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		pointer string
	}{
		{
			name: "description", body: `{"description":1,"content":{"application/json":{"schema":{}}}}`,
			pointer: "/description",
		},
		{
			name: "required", body: `{"required":"yes","content":{"application/json":{"schema":{}}}}`,
			pointer: "/required",
		},
		{
			name: "unknown field", body: `{"unknown":true,"content":{"application/json":{"schema":{}}}}`,
			pointer: "/unknown",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			document := `{"openapi":"3.0.4","paths":{"/":{"post":{` +
				`"operationId":"selected","requestBody":` + test.body + `}}}}`
			_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
			require.ErrorContains(t, err, "#/paths/~1/post/requestBody"+test.pointer)
		})
	}
}

func TestParseInputRejectsInvalidSelectedMediaTypeFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		yaml    string
		pointer string
	}{
		{name: "unknown", json: `"bogus":1`, yaml: "bogus: 1", pointer: "/bogus"},
		{name: "encoding", json: `"encoding":{}`, yaml: "encoding: {}", pointer: "/encoding"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for encoding, document := range map[string]string{
				"json": `{"openapi":"3.0.4","paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{},` + test.json + `}}}}}}}`,
				"yaml": `openapi: 3.0.4
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema: {}
            ` + test.yaml + "\n",
			} {
				t.Run(encoding, func(t *testing.T) {
					t.Parallel()

					_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
					require.ErrorContains(t, err, "/content/application~1json"+test.pointer)
				})
			}
		})
	}
}

func TestParseInputRejectsMutuallyExclusiveMediaTypeExamples(t *testing.T) {
	t.Parallel()

	for encoding, document := range map[string]string{
		"json": `{"openapi":"3.0.4","paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{},"example":1,"examples":{}}}}}}}}`,
		"yaml": `openapi: 3.0.4
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema: {}
            example: 1
            examples: {}
`,
	} {
		t.Run(encoding, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
			require.ErrorContains(t, err, "/content/application~1json/examples")
			require.ErrorContains(t, err, "mutually exclusive")
		})
	}
}

func TestParseInputRejectsMalformedMediaTypeExamplesMap(t *testing.T) {
	t.Parallel()

	for encoding, document := range map[string]string{
		"json": `{"openapi":"3.0.4","paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{},"examples":[]}}}}}}}`,
		"yaml": `openapi: 3.0.4
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema: {}
            examples: []
`,
	} {
		t.Run(encoding, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
			require.ErrorContains(t, err, "/content/application~1json/examples")
			require.ErrorContains(t, err, "must be an object")
		})
	}
}

func TestParseInputRejectsMalformedMediaTypeExampleEntry(t *testing.T) {
	t.Parallel()

	for encoding, document := range map[string]string{
		"json": `{"openapi":"3.0.4","paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{},"examples":{"bad":1}}}}}}}}`,
		"yaml": `openapi: 3.0.4
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema: {}
            examples: {bad: 1}
`,
	} {
		t.Run(encoding, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
			require.ErrorContains(t, err, "/content/application~1json/examples/bad")
			require.ErrorContains(t, err, "must be an object")
		})
	}
}

func TestParseInputRejectsMalformedMediaTypeExampleObject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		yaml    string
		pointer string
	}{
		{name: "summary", json: `{"summary":1}`, yaml: `{summary: 1}`, pointer: "/summary"},
		{name: "reference", json: `{"$ref":1}`, yaml: `{$ref: 1}`, pointer: "/$ref"},
		{
			name: "reference raw space", json: `{"$ref":"#/components/examples/a b"}`,
			yaml: `{$ref: "#/components/examples/a b"}`, pointer: "/$ref",
		},
		{
			name: "reference malformed percent", json: `{"$ref":"#/components/examples/a%2"}`,
			yaml: `{$ref: "#/components/examples/a%2"}`, pointer: "/$ref",
		},
		{
			name: "external reference", json: `{"$ref":"example.yaml#/example"}`,
			yaml: `{$ref: "example.yaml#/example"}`, pointer: "/$ref",
		},
		{
			name: "mutually exclusive value", json: `{"value":1,"externalValue":"example.json"}`,
			yaml: `{value: 1, externalValue: example.json}`, pointer: "/externalValue",
		},
		{
			name: "external value invalid URI", json: `{"externalValue":"https://example.test/a|b"}`,
			yaml: `{externalValue: "https://example.test/a|b"}`, pointer: "/externalValue",
		},
		{
			name: "external value invalid path character", json: `{"externalValue":"https://example.test/a[b"}`,
			yaml: `{externalValue: "https://example.test/a[b"}`, pointer: "/externalValue",
		},
		{name: "unknown field", json: `{"unknown":1}`, yaml: `{unknown: 1}`, pointer: "/unknown"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			documents := map[string]string{
				"json": `{"openapi":"3.0.4","paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{},"examples":{"bad":` + test.json + `}}}}}}}}`,
				"yaml": `openapi: 3.0.4
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema: {}
            examples:
              bad: ` + test.yaml + "\n",
			}

			for encoding, document := range documents {
				t.Run(encoding, func(t *testing.T) {
					t.Parallel()

					_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
					require.ErrorContains(t, err, "/content/application~1json/examples/bad"+test.pointer)
				})
			}
		})
	}
}

func TestParseInputIgnoresMediaTypeExampleReferenceSiblingsAndTarget(t *testing.T) {
	t.Parallel()

	for encoding, document := range map[string]string{
		"json": `{"openapi":"3.0.4","paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{},"examples":{"kept-inert":{"$ref":"#/components/examples/Missing","valeu":1}}}}}}}}}`,
		"yaml": `openapi: 3.0.4
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema: {}
            examples:
              kept-inert: {$ref: "#/components/examples/Missing", valeu: 1}
`,
	} {
		t.Run(encoding, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
			require.NoError(t, err)
		})
	}
}

func TestBuildParsesProfileBeforeConsideringMaxSteps(t *testing.T) {
	t.Parallel()

	report, err := Build(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"oneOf":[{}]}`)),
		OperationID: "selected",
		MaxSteps:    0,
	}, func(Case) error { return nil })
	require.Error(t, err)
	require.ErrorContains(t, err, "/oneOf")
	require.NotErrorIs(t, err, errBuildNotImplemented)
	require.Zero(t, report)
}

func TestParseInputRejectsExternalMissingAndRecursiveReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		yaml    string
		pointer string
	}{
		{
			name: "external", json: `{"$ref":"other.yaml#/Thing"}`,
			yaml: `$ref: 'other.yaml#/Thing'`, pointer: "/$ref",
		},
		{
			name: "missing", json: `{"$ref":"#/components/schemas/Missing"}`,
			yaml: `$ref: '#/components/schemas/Missing'`, pointer: "/$ref",
		},
		{
			name: "malformed pointer", json: `{"$ref":"#/components/schemas/Bad~2Token"}`,
			yaml: `$ref: '#/components/schemas/Bad~2Token'`, pointer: "/$ref",
		},
		{
			name: "self cycle", json: `{"$ref":"#/paths/~1/post/requestBody/content/application~1json/schema"}`,
			yaml: `$ref: '#/paths/~1/post/requestBody/content/application~1json/schema'`, pointer: "/$ref",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for encoding, document := range map[string]string{
				"json": documentWithJSONSchema(test.json),
				"yaml": documentWithYAMLSchema(test.yaml),
			} {
				t.Run(encoding, func(t *testing.T) {
					t.Parallel()

					_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
					require.Error(t, err)
					require.Contains(
						t,
						err.Error(),
						"#/paths/~1/post/requestBody/content/application~1json/schema"+test.pointer,
					)
				})
			}
		})
	}
}

func TestParseInputRequiresURIEncodedLocalReferenceCharacters(t *testing.T) {
	t.Parallel()

	for encoding, document := range map[string]string{
		"json": `{
			"openapi":"3.0.4",
			"components":{"schemas":{"a b":{"type":"string"}}},
			"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
				"$ref":"#/components/schemas/a b"
			}}}}}}}
		}`,
		"yaml": `openapi: 3.0.4
components:
  schemas:
    a b: {type: string}
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema: {$ref: '#/components/schemas/a b'}
`,
	} {
		t.Run(encoding, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
			require.ErrorContains(t, err, "/schema/$ref")
			require.ErrorContains(t, err, "URI-reference")
		})
	}

	model, err := parseInput(Input{
		OpenAPI: []byte(`{
			"openapi":"3.0.4",
			"components":{"schemas":{"a b":{"type":"string"}}},
			"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
				"$ref":"#/components/schemas/a%20b"
			}}}}}}}
		}`),
		OperationID: "selected",
	})
	require.NoError(t, err)
	require.Equal(t, schemaString, model.root.kind)
}

func TestParseInputRejectsNonUTF8LocalReferenceFragment(t *testing.T) {
	t.Parallel()

	_, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"$ref":"#/%FF"}`)),
		OperationID: "selected",
	})
	require.ErrorContains(t, err, "#/paths/~1/post/requestBody/content/application~1json/schema/$ref")
	require.ErrorContains(t, err, "valid UTF-8")
}

func TestParseInputOpenAPIVersionProfile(t *testing.T) {
	t.Parallel()

	for _, version := range []string{
		"3.0.0",
		"3.0.4",
		"3.0.999",
		"3.0.4-alpha.1",
		"3.0.4-0.3.7+001",
		"3.0.4+build.9",
		"3.0.4-alpha.1+build.9",
		"3.0.4-x-y-z.--+exp.sha.5114f85",
	} {
		t.Run("accept_"+version, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(fmt.Sprintf(`{
				"openapi":%q,
				"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{}}}}}}}
			}`, version)), OperationID: "selected"})
			require.NoError(t, err)
		})
	}

	for _, test := range []struct {
		version   string
		wantError string
	}{
		{version: "3.0", wantError: "must be a valid Semantic Version"},
		{version: "03.0.1", wantError: "must be a valid Semantic Version"},
		{version: "3.0.01", wantError: "must be a valid Semantic Version"},
		{version: "3.0.0-", wantError: "must be a valid Semantic Version"},
		{version: "3.0.0-01", wantError: "must be a valid Semantic Version"},
		{version: "3.0.0+", wantError: "must be a valid Semantic Version"},
		{version: "3.0.0+bad_meta", wantError: "must be a valid Semantic Version"},
		{version: "2.0.0", wantError: "feature set 2.0 is outside the Klopt 3.0 profile"},
		{version: "3.1.0", wantError: "feature set 3.1 is outside the Klopt 3.0 profile"},
		{version: "4.0.0", wantError: "feature set 4.0 is outside the Klopt 3.0 profile"},
	} {
		t.Run("reject_"+test.version, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(fmt.Sprintf(`{
				"openapi":%q,
				"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{}}}}}}}
			}`, test.version)), OperationID: "selected"})
			require.ErrorContains(t, err, "#/openapi")
			require.ErrorContains(t, err, test.wantError)
		})
	}
}

func TestParseInputRejectsOpenAPIVersionBeforeRequestSelection(t *testing.T) {
	t.Parallel()

	_, err := parseInput(Input{OpenAPI: []byte(`{"openapi":"3.1.0"}`), OperationID: "missing"})
	require.ErrorContains(t, err, "#/openapi: feature set 3.1 is outside the Klopt 3.0 profile")
	require.NotContains(t, err.Error(), "operationId")
}
