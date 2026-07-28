//nolint:godoclint // Integration test name states the public behavior.
package testgenerator

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/djosh34/klopt/pkg/validation"
	"github.com/stretchr/testify/require"
)

func TestNativeFormatsGenerateEndToEndThroughRuntimeValidation(t *testing.T) {
	t.Parallel()

	spec := []byte(`
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: checkThing
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [uuid, cidr, date, timestamp, payload, mailboxes, count, approximate]
              properties:
                uuid:
                  type: string
                  format: uuid
                  pattern: ^a
                cidr:
                  allOf:
                    - {$ref: '#/components/schemas/CIDR'}
                    - {pattern: '^192\.0\.2\.7/24$'}
                date:
                  type: string
                  format: date
                  pattern: ^2024-
                timestamp:
                  type: string
                  format: date-time
                  pattern: Z$
                payload: {type: string, format: byte}
                mailboxes:
                  type: array
                  minItems: 1
                  maxItems: 2
                  items: {type: string, format: email, pattern: '@example\.com$'}
                count: {type: number, format: int32}
                approximate: {type: number, format: float}
components:
  schemas:
    CIDR: {type: string, format: cidr}
`)

	parsed, err := validation.Parse(spec)
	require.NoError(t, err)

	CheckJSONRequestBodies(t, spec, func(operationID string, body []byte) error {
		require.Equal(t, "checkThing", operationID)

		errs := parsed[operationID].Body.Validate(json.RawMessage(body))
		if len(errs) == 0 {
			return nil
		}

		return errors.Join(errs...)
	}, validation.PatternOptions())
}
