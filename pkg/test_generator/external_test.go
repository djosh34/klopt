//nolint:godoclint // Adapter characterizations use descriptive test names.
package testgenerator

import (
	"fmt"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestLibopenapiOpenAPI30SchemaCharacterization(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		properties string
		required   string
		body       string
		supported  bool
	}{
		{
			name:       "escaped-property-name",
			properties: `"~": {type: integer, maximum: 100}`,
			required:   `"~"`,
			body:       `{"~":100}`,
			supported:  true,
		},
		{
			name:       "nullable",
			properties: `value: {type: integer, nullable: true}`,
			required:   `value`,
			body:       `{"value":null}`,
			supported:  true,
		},
		{
			name:       "nullable-enum",
			properties: `value: {type: integer, nullable: true, enum: [0, 1]}`,
			required:   `value`,
			body:       `{"value":0}`,
			supported:  true,
		},
		{
			name:       "empty-name-nullable-enum",
			properties: `"": {type: integer, nullable: true, enum: [0, 1]}`,
			required:   `""`,
			body:       `{"":0}`,
			supported:  false,
		},
		{
			name:       "empty-name-integer",
			properties: `"": {type: integer}`,
			required:   `""`,
			body:       `{"":0}`,
			supported:  false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			document := []byte(fmt.Sprintf(`
openapi: 3.0.4
info: {title: test, version: "1"}
paths:
  /things:
    post:
      operationId: checkThing
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [%[2]s]
              additionalProperties: false
              properties: {%[1]s}
      responses: {"204": {description: ok}}
`, test.properties, test.required))
			validator, err := newLibValidator(document)
			require.NoError(t, err)
			t.Cleanup(validator.close)

			accepted, reason, err := validator.validate("checkThing", []byte(test.body))
			if !test.supported {
				require.ErrorIs(t, err, errExternalUnsupported)
				require.NotEmpty(t, reason)

				operation := validator.operations["checkThing"]
				operation.unsupported = ""
				validator.operations["checkThing"] = operation

				accepted, reason, err = validator.validate("checkThing", []byte(test.body))
				require.NoError(t, err)
				require.False(t, accepted)
				require.Contains(t, reason, "failed schema compilation")

				return
			}

			require.NoError(t, err)
			require.True(t, accepted, reason)
		})
	}
}

func TestKinOpenAPIFloatRangeCharacterization(t *testing.T) {
	t.Parallel()

	document := []byte(`{
  "openapi": "3.0.4",
  "info": {"title": "test", "version": "1"},
  "paths": {
    "/things": {
      "post": {
        "operationId": "checkThing",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"type": "number", "maximum": 1e400}
            }
          }
        },
        "responses": {"204": {"description": "ok"}}
      }
    }
  }
}`)
	_, err := openapi3.NewLoader().LoadFromData(document)
	require.ErrorContains(t, err, "type float64")

	validator, err := newKinValidator(document)
	require.NoError(t, err)

	_, reason, err := validator.validate("checkThing", []byte(`0`))
	require.ErrorIs(t, err, errExternalUnsupported)
	require.NotEmpty(t, reason)
}
