//nolint:godoclint // Behavior tests use readable names instead of redundant prose comments.
package testgenerator_test

import (
	"errors"
	"testing"

	testgenerator "github.com/djosh34/klopt/pkg/test_generator"
	"github.com/djosh34/klopt/pkg/validation"
	"github.com/stretchr/testify/require"
)

func TestGeneratorDecodesOneDocumentTapeAndChecksIndependentValidator(t *testing.T) {
	t.Parallel()

	compiled, err := testgenerator.Compile([]byte(`
openapi: 3.0.4
info: {title: test, version: 1.0.0}
paths:
  /things:
    post:
      operationId: createThing
      requestBody:
        content:
          application/json:
            schema: {type: string, minLength: 1}
      responses: {"204": {description: ok}}
`))
	require.NoError(t, err)
	t.Cleanup(compiled.Close)
	require.False(t, compiled.Empty())

	valid, err := compiled.Decode(nil)
	require.NoError(t, err)
	require.Equal(t, "createThing", valid.OperationID)
	require.True(t, valid.ExpectValid)
	require.NoError(t, compiled.Check(valid, func(_ string, body []byte) error {
		if string(body) == `"\u0000"` {
			return nil
		}

		return errors.New("unexpected body")
	}))

	invalidTape := make([]byte, 9)
	invalidTape[8] = 1
	invalid, err := compiled.Decode(invalidTape)
	require.NoError(t, err)
	require.False(t, invalid.ExpectValid)
	require.NoError(t, compiled.Check(invalid, func(_ string, _ []byte) error {
		return errors.New("rejected")
	}))
}

func TestGeneratorKeepsAnyOfSiblingsAndOuterVerdictActive(t *testing.T) {
	t.Parallel()

	document := []byte(`
openapi: 3.0.4
info: {title: test, version: 1.0.0}
paths:
  /things:
    post:
      operationId: createThing
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [x]
              additionalProperties: false
              properties:
                x:
                  anyOf:
                    - {type: integer, minimum: 0, maximum: 9}
                    - {type: string, pattern: '^a+$'}
      responses: {"204": {description: ok}}
`)
	compiled, err := testgenerator.Compile(document)
	require.NoError(t, err)
	t.Cleanup(compiled.Close)
	validations := mustValidations(t, document)

	for verdict := byte(0); verdict < 2; verdict++ {
		for choice := byte(0); choice < 64; choice++ {
			input := make([]byte, 96)

			input[8] = verdict
			for index := 16; index < len(input); index += 8 {
				input[index] = choice
			}

			sample, decodeErr := compiled.Decode(input)
			if testgenerator.ResourceLimited(decodeErr) {
				continue
			}

			require.NoError(t, decodeErr)
			require.NoError(t, compiled.Check(sample, func(operationID string, body []byte) error {
				return errors.Join(validations[operationID].Body.Validate(body)...)
			}))
			require.Equal(t, verdict == 0, sample.ExpectValid)
		}
	}
}

func mustValidations(t *testing.T, document []byte) map[string]validation.RequestValidation {
	t.Helper()

	compiled, err := validation.Parse(document)
	require.NoError(t, err)

	return compiled
}
