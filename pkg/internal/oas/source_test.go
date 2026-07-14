package oas

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseLocatesRequestSchemaThroughDocumentReferences verifies document-aware selection.
func TestParseLocatesRequestSchemaThroughDocumentReferences(t *testing.T) {
	t.Parallel()

	source, err := Parse([]byte(`
openapi: 3.0.3
paths:
  /things:
    $ref: '#/x-path-items/Things'
x-path-items:
  Things:
    post:
      operationId: createThing
      requestBody:
        $ref: '#/components/requestBodies/CreateThing'
        required: false
components:
  requestBodies:
    CreateThing:
      required: true
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Thing'
            description: ignored
  schemas:
    Thing:
      type: object
      properties:
        child:
          $ref: '#/components/schemas/ChildAlias'
          nullable: true
    ChildAlias:
      $ref: '#/components/schemas/Child'
    Child:
      type: string
`), "createThing")
	require.NoError(t, err)
	require.True(t, source.RequestBodyRequired)
	require.Equal(
		t,
		"#/components/requestBodies/CreateThing/content/application~1json/schema",
		source.RequestSchema.Pointer,
	)
	require.JSONEq(t, `{"$ref":"#/components/schemas/Thing","description":"ignored"}`, string(source.RequestSchema.Raw))

	rootSchema, err := source.Resolve(source.RequestSchema)
	require.NoError(t, err)
	require.Equal(t, "#/components/schemas/Thing", rootSchema.Pointer)

	child, err := source.Child(rootSchema, "properties", "child")
	require.NoError(t, err)
	resolvedChild, err := source.Resolve(child)
	require.NoError(t, err)
	require.Equal(t, "#/components/schemas/Child", resolvedChild.Pointer)
	require.JSONEq(t, `{"type":"string"}`, string(resolvedChild.Raw))

	// Resolution is lazy: neither root Schema Object nor its Reference Object is replaced.
	var document map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source.Document, &document))
	require.Contains(t, string(source.Document), `"nullable": true`)
	require.Contains(t, string(source.Document), `"$ref": "#/components/schemas/ChildAlias"`)
}

// TestNestedSchemaPositionsResolveWithCanonicalPointers verifies properties, items, and allOf positions.
func TestNestedSchemaPositionsResolveWithCanonicalPointers(t *testing.T) {
	t.Parallel()

	source, err := Parse([]byte(`
openapi: 3.0.3
paths:
  /escaped/~things:
    post:
      operationId: nestedThing
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                a/b:
                  type: array
                  items:
                    $ref: '#/components/schemas/Alias'
              allOf:
                - $ref: '#/components/schemas/Base'
              additionalProperties:
                $ref: '#/components/schemas/A~1B~0C'
components:
  schemas:
    Alias:
      $ref: '#/components/schemas/Base'
    Base:
      type: integer
    A/B~C:
      type: boolean
`), "nestedThing")
	require.NoError(t, err)
	require.Equal(
		t,
		"#/paths/~1escaped~1~0things/post/requestBody/content/application~1json/schema",
		source.RequestSchema.Pointer,
	)

	item, err := source.Child(source.RequestSchema, "properties", "a/b", "items")
	require.NoError(t, err)
	require.Contains(t, item.Pointer, "/properties/a~1b/items")
	item, err = source.Resolve(item)
	require.NoError(t, err)
	require.Equal(t, "#/components/schemas/Base", item.Pointer)

	allOfChild, err := source.Child(source.RequestSchema, "allOf", "0")
	require.NoError(t, err)
	allOfChild, err = source.Resolve(allOfChild)
	require.NoError(t, err)
	require.Equal(t, "#/components/schemas/Base", allOfChild.Pointer)

	additional, err := source.Child(source.RequestSchema, "additionalProperties")
	require.NoError(t, err)
	additional, err = source.Resolve(additional)
	require.NoError(t, err)
	require.Equal(t, "#/components/schemas/A~1B~0C", additional.Pointer)
	require.JSONEq(t, `{"type":"boolean"}`, string(additional.Raw))
}

// TestParseRequiresExactlyOneJSONRequestSchema verifies deterministic selection failures.
func TestParseRequiresExactlyOneJSONRequestSchema(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		spec      string
		operation string
		wantError string
	}{
		"missing operation": {
			spec: `
openapi: 3.0.3
paths: {}
`,
			operation: "missing",
			wantError: `operationId "missing" not found`,
		},
		"duplicate operation": {
			spec: `
openapi: 3.0.3
paths:
  /first:
    get:
      operationId: duplicate
      requestBody:
        content:
          application/json:
            schema: {type: string}
  /second:
    post:
      operationId: duplicate
      requestBody:
        content:
          application/json:
            schema: {type: string}
`,
			operation: "duplicate",
			wantError: `operationId "duplicate" found multiple times`,
		},
		"missing JSON media type": {
			spec: `
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: create
      requestBody:
        content:
          text/plain:
            schema: {type: string}
`,
			operation: "create",
			wantError: "has no application/json content",
		},
		"missing schema": {
			spec: `
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: create
      requestBody:
        content:
          application/json: {}
`,
			operation: "create",
			wantError: "schema does not exist",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse([]byte(tt.spec), tt.operation)
			require.ErrorContains(t, err, tt.wantError)
		})
	}
}

// TestParseSelectsApplicationJSONMediaRangesBySpecificity verifies request content matching.
func TestParseSelectsApplicationJSONMediaRangesBySpecificity(t *testing.T) {
	t.Parallel()

	for name, content := range map[string]string{
		"application wildcard": "application/*: {schema: {type: string}}",
		"global wildcard":      "'*/*': {schema: {type: boolean}}",
		"exact wins":           "'*/*': {schema: {type: boolean}}\n          application/json: {schema: {type: string}}",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			source, err := Parse([]byte(`openapi: 3.0.3
paths:
  /things:
    post:
      operationId: create
      requestBody:
        content:
          `+content), "create")
			require.NoError(t, err)

			if name == "exact wins" {
				require.JSONEq(t, `{"type":"string"}`, string(source.RequestSchema.Raw))
			}
		})
	}
}

// TestParseRejectsNullRequestBodyRequired verifies null is not decoded as false.
func TestParseRejectsNullRequestBodyRequired(t *testing.T) {
	t.Parallel()

	_, err := Parse([]byte(`openapi: 3.0.3
paths:
  /things:
    post:
      operationId: create
      requestBody:
        required: null
        content:
          application/json: {schema: {type: string}}`), "create")
	require.ErrorContains(t, err, "required must be a boolean")
}

// TestResolveReportsExternalAndCyclicReferences verifies clear reference failures.
func TestResolveReportsExternalAndCyclicReferences(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		ref       string
		aliases   string
		wantError string
	}{
		"external": {
			ref:       "other.yaml#/Thing",
			wantError: "external reference",
		},
		"cycle": {
			ref: "#/components/schemas/A",
			aliases: `
    A: {$ref: '#/components/schemas/B'}
    B: {$ref: '#/components/schemas/A'}
`,
			wantError: "reference cycle",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			spec := []byte(`
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: create
      requestBody:
        content:
          application/json:
            schema: {$ref: '` + tt.ref + `'}
components:
  schemas:
` + tt.aliases)
			source, err := Parse(spec, "create")
			require.NoError(t, err)

			_, err = source.Resolve(source.RequestSchema)
			require.ErrorContains(t, err, tt.wantError)

			var referenceError *ReferenceError
			require.True(t, errors.As(err, &referenceError))
			require.NotEmpty(t, referenceError.Referrer)
			require.NotEmpty(t, referenceError.Reference)
		})
	}
}

// TestRequestBodyRequiredDefaultsFalse verifies the OpenAPI default.
func TestRequestBodyRequiredDefaultsFalse(t *testing.T) {
	t.Parallel()

	source, err := Parse([]byte(`
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: create
      requestBody:
        content:
          application/json:
            schema: {type: boolean}
`), "create")
	require.NoError(t, err)
	require.False(t, source.RequestBodyRequired)
}
