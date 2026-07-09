package testgenerator

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

const openAPIObjectSchemaSpec = `
openapi: 3.0.3
info:
  title: test
  version: 1.0.0
paths:
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

func TestOpenAPIRequestBodySchemaNodeReturnsObjectSchemaNode(t *testing.T) {
	openAPIJSONSpec, err := YAMLBytesToJSONRawMessage([]byte(openAPIObjectSchemaSpec))
	require.NoError(t, err)

	schemaNode, err := OpenAPIRequestBodySchemaNode(openAPIJSONSpec, "createThing")
	require.NoError(t, err)

	require.JSONEq(t, `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`, string(*schemaNode))
}

func TestOpenAPIRequestBodySchemaNodeReturnsRefSchemaNode(t *testing.T) {
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

func TestOpenAPIRequestBodySchemaNodeErrors(t *testing.T) {
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
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			openAPIJSONSpec, err := YAMLBytesToJSONRawMessage([]byte(tt.openAPISpec))
			require.NoError(t, err)

			_, err = OpenAPIRequestBodySchemaNode(openAPIJSONSpec, tt.operationID)
			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantError)
		})
	}
}

func TestGenerateFunctionsParseSchemaAndDoNothing(t *testing.T) {
	tests := map[string]struct {
		generate func([]byte, string, func([]byte) error) error
	}{
		"valid":   {generate: GenerateValid},
		"invalid": {generate: GenerateInvalid},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
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

func TestGenerateFunctionsWrapOpenAPISchemaLookupErrors(t *testing.T) {
	tests := map[string]struct {
		generate func([]byte, string, func([]byte) error) error
	}{
		"valid":   {generate: GenerateValid},
		"invalid": {generate: GenerateInvalid},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.generate([]byte(openAPIObjectSchemaSpec), "missingThing", func(_ []byte) error { return nil })
			require.Error(t, err)
			require.ErrorContains(t, err, "openapi request body schema lookup failed")
			require.ErrorContains(t, err, `operationId "missingThing" not found`)
		})
	}
}
