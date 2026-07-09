package testgenerator

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// openAPIObjectSchemaSpec is the shared inline-object request schema fixture.
const openAPIObjectSchemaSpec = `
openapi: 3.0.3
info:
  title: test
  version: 1.0.0
paths:
  x-test-extension: any value is allowed
  /things:
    post:
      operationId: createThing
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required:
                - name
              properties:
                name:
                  type: string
`

// TestOpenAPIRequestBodySchemaNodeReturnsObjectSchemaNode verifies inline schema lookup.
func TestOpenAPIRequestBodySchemaNodeReturnsObjectSchemaNode(t *testing.T) {
	t.Parallel()

	openAPIJSONSpec, err := YAMLBytesToJSONRawMessage([]byte(openAPIObjectSchemaSpec))
	require.NoError(t, err)

	schemaNode, err := OpenAPIRequestBodySchemaNode(openAPIJSONSpec, "createThing")
	require.NoError(t, err)

	require.JSONEq(t, `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`, string(*schemaNode))
}

// TestOpenAPIRequestBodySchemaNodeReturnsRefSchemaNode verifies schema references stay unresolved.
func TestOpenAPIRequestBodySchemaNodeReturnsRefSchemaNode(t *testing.T) {
	t.Parallel()

	openAPISpec := []byte(`
openapi: 3.0.3
info:
  title: test
  version: 1.0.0
paths:
  /things/{id}:
    put:
      operationId: updateThing
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Thing'
components:
  schemas:
    Thing:
      type: object
`)

	openAPIJSONSpec, err := YAMLBytesToJSONRawMessage(openAPISpec)
	require.NoError(t, err)

	schemaNode, err := OpenAPIRequestBodySchemaNode(openAPIJSONSpec, "updateThing")
	require.NoError(t, err)

	require.JSONEq(t, `{"$ref":"#/components/schemas/Thing"}`, string(*schemaNode))
}

// TestOpenAPIRequestBodySchemaNodeResolvesPathItemAndRequestBodyRefs verifies lazy reference lookup.
func TestOpenAPIRequestBodySchemaNodeResolvesPathItemAndRequestBodyRefs(t *testing.T) {
	t.Parallel()

	openAPISpec := []byte(`
openapi: 3.0.3
info:
  title: test
  version: 1.0.0
paths:
  /things:
    $ref: '#/x-path-items/ThingAlias'
x-path-items:
  ThingAlias:
    $ref: '#/x-path-items/Thing'
  Thing:
    post:
      operationId: createThing
      requestBody:
        $ref: '#/components/requestBodies/ThingAlias'
components:
  requestBodies:
    ThingAlias:
      $ref: '#/components/requestBodies/Thing'
    Thing:
      content:
        application/json:
          schema:
            type: object
            example:
              $ref: literal value
              sibling: true
x-unrelated:
  $ref: 123
  sibling: true
`)

	openAPIJSONSpec, err := YAMLBytesToJSONRawMessage(openAPISpec)
	require.NoError(t, err)

	schemaNode, err := OpenAPIRequestBodySchemaNode(openAPIJSONSpec, "createThing")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"type": "object",
		"example": {"$ref": "literal value", "sibling": true}
	}`, string(*schemaNode))
}

// TestOpenAPIRequestBodySchemaNodeErrors verifies invalid lookups are rejected.
func TestOpenAPIRequestBodySchemaNodeErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		openAPISpec string
		operationID string
		wantError   string
	}{
		"operation not found": {
			openAPISpec: openAPIObjectSchemaSpec,
			operationID: "missingThing",
			wantError:   `operationId "missingThing" not found`,
		},
		"duplicate operation id": {
			openAPISpec: `
openapi: 3.0.3
paths:
  /first:
    post:
      operationId: duplicateThing
      requestBody:
        content:
          application/json:
            schema:
              type: string
  /second:
    patch:
      operationId: duplicateThing
      requestBody:
        content:
          application/json:
            schema:
              type: object
`,
			operationID: "duplicateThing",
			wantError:   `operationId "duplicateThing" found multiple times`,
		},
		"content type does not exist": {
			openAPISpec: `
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: createThing
      requestBody: {}
`,
			operationID: "createThing",
			wantError:   `operationId "createThing" request body content type does not exist`,
		},
		"content type is not json": {
			openAPISpec: `
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: createThing
      requestBody:
        content:
          text/plain:
            schema:
              type: string
`,
			operationID: "createThing",
			wantError:   `operationId "createThing" request body content type is not json`,
		},
		"schema does not exist": {
			openAPISpec: `
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: createThing
      requestBody:
        content:
          application/json: {}
`,
			operationID: "createThing",
			wantError:   `operationId "createThing" application/json schema does not exist`,
		},
		"path item ref target does not exist": {
			openAPISpec: `
openapi: 3.0.3
paths:
  /things:
    $ref: '#/x-path-items/Missing'
x-path-items: {}
`,
			operationID: "createThing",
			wantError:   `resolve openapi path item "/things"`,
		},
		"path item ref cycle": {
			openAPISpec: `
openapi: 3.0.3
paths:
  /things:
    $ref: '#/x-path-items/First'
x-path-items:
  First:
    $ref: '#/x-path-items/Second'
  Second:
    $ref: '#/x-path-items/First'
`,
			operationID: "createThing",
			wantError:   "reference cycle",
		},
		"request body ref target does not exist": {
			openAPISpec: `
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: createThing
      requestBody:
        $ref: '#/components/requestBodies/Missing'
components:
  requestBodies: {}
`,
			operationID: "createThing",
			wantError:   `resolve operationId "createThing" request body`,
		},
		"request body ref cycle": {
			openAPISpec: `
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: createThing
      requestBody:
        $ref: '#/components/requestBodies/First'
components:
  requestBodies:
    First:
      $ref: '#/components/requestBodies/Second'
    Second:
      $ref: '#/components/requestBodies/First'
`,
			operationID: "createThing",
			wantError:   "reference cycle",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			openAPIJSONSpec, err := YAMLBytesToJSONRawMessage([]byte(tt.openAPISpec))
			require.NoError(t, err)

			_, err = OpenAPIRequestBodySchemaNode(openAPIJSONSpec, tt.operationID)
			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantError)
		})
	}
}

// TestOpenAPIRequestBodySchemaNodeRejectsInvalidInputs verifies public input guards.
func TestOpenAPIRequestBodySchemaNodeRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	_, err := OpenAPIRequestBodySchemaNode(nil, "createThing")
	require.Error(t, err)
	require.ErrorContains(t, err, "openapi json spec is nil")

	openAPIJSONSpec, err := YAMLBytesToJSONRawMessage([]byte(openAPIObjectSchemaSpec))
	require.NoError(t, err)

	_, err = OpenAPIRequestBodySchemaNode(openAPIJSONSpec, "")
	require.Error(t, err)
	require.ErrorContains(t, err, "operationId must not be empty")
}

// TestGenerateFunctionsParseSchemaAndDoNothing documents the current no-generation behavior.
func TestGenerateFunctionsParseSchemaAndDoNothing(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		generate func([]byte, string, func([]byte) error) error
	}{
		"valid":   {generate: GenerateValid},
		"invalid": {generate: GenerateInvalid},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			called := false
			err := tt.generate([]byte(openAPIObjectSchemaSpec), "createThing", func(_ []byte) error {
				called = true

				return errors.New("unmarshal must not be called yet")
			})
			require.NoError(t, err)
			require.False(t, called)
		})
	}
}

// TestGenerateFunctionsWrapOpenAPISchemaLookupErrors verifies entry-point error context.
func TestGenerateFunctionsWrapOpenAPISchemaLookupErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		generate func([]byte, string, func([]byte) error) error
	}{
		"valid":   {generate: GenerateValid},
		"invalid": {generate: GenerateInvalid},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tt.generate([]byte(openAPIObjectSchemaSpec), "missingThing", func(_ []byte) error { return nil })
			require.Error(t, err)
			require.ErrorContains(t, err, "openapi request body schema lookup failed")
			require.ErrorContains(t, err, `operationId "missingThing" not found`)
		})
	}
}
