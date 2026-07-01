package testgenerator

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunJSONRequestBodyOperationCasesResolvesRootRef(t *testing.T) {
	openAPI := []byte(`
openapi: 3.0.3
info:
  title: Ref Test
  version: 1.0.0
paths:
  /ref:
    post:
      operationId: refPayload
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Name'
      responses:
        '204':
          description: No Content
components:
  schemas:
    Name:
      type: string
      nullable: false
`)

	RunJSONRequestBodyOperationCases(t, openAPI, "refPayload", func(data []byte) error {
		var value any
		err := json.Unmarshal(data, &value)
		if err != nil {
			return err
		}
		if _, ok := value.(string); !ok {
			return fmt.Errorf("not a string")
		}

		return nil
	})
}

func TestSchemaRefsResolveNestedSchemaLocations(t *testing.T) {
	schema, required, err := decodeRefTestSchema(`
openapi: 3.0.3
info:
  title: Nested Ref Test
  version: 1.0.0
paths:
  /ref:
    post:
      operationId: refPayload
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Payload'
      responses:
        '204':
          description: No Content
components:
  schemas:
    Payload:
      type: object
      required:
        - name
        - tags
        - meta
      additionalProperties: false
      properties:
        name:
          $ref: '#/components/schemas/Name'
        tags:
          type: array
          items:
            $ref: '#/components/schemas/Name'
        meta:
          type: object
          additionalProperties:
            $ref: '#/components/schemas/Name'
    Name:
      type: string
      nullable: false
`, "refPayload")
	require.NoError(t, err)
	require.True(t, required)

	require.Equal(t, "object", schema.Type)
	require.Equal(t, "string", schema.Object.Properties["name"].Type)
	require.Equal(t, "string", schema.Object.Properties["tags"].Array.Items.Type)
	require.Equal(t, "string", schema.Object.Properties["meta"].Object.AdditionalProperties.Schema.Type)
	require.Contains(t, rawMessages(schema.ValidCases()), `{"meta":{},"name":"valid-string","tags":["valid-string"]}`)
	require.Contains(t, rawMessages(schema.ValidCases()), `{"meta":{"`+additionalPropertyCaseKey+`":"valid-string"},"name":"valid-string","tags":[]}`)
}

func TestSchemaRefsResolveAllOfSchemas(t *testing.T) {
	schema, _, err := decodeRefTestSchema(`
openapi: 3.0.3
info:
  title: AllOf Ref Test
  version: 1.0.0
paths:
  /ref:
    post:
      operationId: refPayload
      requestBody:
        required: true
        content:
          application/json:
            schema:
              allOf:
                - $ref: '#/components/schemas/ID'
                - type: object
                  required:
                    - active
                  properties:
                    active:
                      type: boolean
      responses:
        '204':
          description: No Content
components:
  schemas:
    ID:
      type: object
      required:
        - id
      properties:
        id:
          type: string
`, "refPayload")
	require.NoError(t, err)

	require.Equal(t, []string{"id", "active"}, schema.Object.Required)
	require.Contains(t, rawMessages(schema.ValidCases()), `{"active":true,"id":"valid-string"}`)
}

func TestSchemaRefsResolveEscapedPointerTokens(t *testing.T) {
	schema, _, err := decodeRefTestSchema(`
openapi: 3.0.3
info:
  title: Escaped Ref Test
  version: 1.0.0
paths:
  /ref:
    post:
      operationId: refPayload
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/a~1b~0c'
      responses:
        '204':
          description: No Content
components:
  schemas:
    a/b~c:
      type: string
`, "refPayload")
	require.NoError(t, err)

	require.Equal(t, "string", schema.Type)
	require.NotNil(t, schema.String)
}

func TestSchemaRefsRejectSiblings(t *testing.T) {
	_, _, err := decodeRefTestSchema(`
openapi: 3.0.3
info:
  title: Ref Sibling Test
  version: 1.0.0
paths:
  /ref:
    post:
      operationId: refPayload
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Name'
              nullable: true
      responses:
        '204':
          description: No Content
components:
  schemas:
    Name:
      type: string
`, "refPayload")

	require.ErrorContains(t, err, `schema ref "#/components/schemas/Name" has unsupported sibling "nullable"`)
}

func TestSchemaRefsRejectExternalRefs(t *testing.T) {
	_, _, err := decodeRefTestSchema(`
openapi: 3.0.3
info:
  title: External Ref Test
  version: 1.0.0
paths:
  /ref:
    post:
      operationId: refPayload
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: 'external.yaml#/components/schemas/Name'
      responses:
        '204':
          description: No Content
`, "refPayload")

	require.ErrorContains(t, err, "unsupported non-local schema ref")
}

func TestSchemaRefsRejectMissingTargets(t *testing.T) {
	_, _, err := decodeRefTestSchema(`
openapi: 3.0.3
info:
  title: Missing Ref Test
  version: 1.0.0
paths:
  /ref:
    post:
      operationId: refPayload
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Missing'
      responses:
        '204':
          description: No Content
components:
  schemas:
    Name:
      type: string
`, "refPayload")

	require.ErrorContains(t, err, `resolve schema ref "#/components/schemas/Missing"`)
	require.ErrorContains(t, err, `mapping has no key "Missing"`)
}

func TestSchemaRefsRejectTooMuchDepth(t *testing.T) {
	_, _, err := decodeRefTestSchema(`
openapi: 3.0.3
info:
  title: Cyclic Ref Test
  version: 1.0.0
paths:
  /ref:
    post:
      operationId: refPayload
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Name'
      responses:
        '204':
          description: No Content
components:
  schemas:
    Name:
      $ref: '#/components/schemas/Name'
`, "refPayload")

	require.ErrorContains(t, err, "schema ref depth exceeds 1000")
}

func decodeRefTestSchema(content string, operationID string) (SchemaNode, bool, error) {
	root, schemaNode, required, err := jsonRequestBodySchemaNode([]byte(content), operationID)
	if err != nil {
		return SchemaNode{}, false, err
	}

	schema, err := decodeSchemaNode(root, schemaNode)
	if err != nil {
		return SchemaNode{}, false, err
	}

	return schema, required, nil
}
