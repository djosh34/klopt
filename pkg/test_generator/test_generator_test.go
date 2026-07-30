//nolint:godoclint // Test names state the public runner behavior.
package testgenerator_test

import (
	"errors"
	"testing"

	"github.com/djosh34/klopt/pkg/test_generator"
	"github.com/djosh34/klopt/pkg/validation"
)

var runnerSpec = []byte(`
openapi: 3.0.4
info: {title: runner, version: 1.0.0}
paths:
  /demo:
    post:
      operationId: demo
      requestBody:
        content:
          application/json:
            schema:
              anyOf:
                - {type: string, pattern: '^a+$'}
                - {type: integer, minimum: 10}
      responses: {'204': {description: ok}}
`)

func TestCheckJSONRequestBodiesUsesFinalPrograms(t *testing.T) {
	t.Parallel()

	validators, err := validation.Parse(runnerSpec)
	if err != nil {
		t.Fatal(err)
	}

	testgenerator.CheckJSONRequestBodies(t, runnerSpec, func(operationID string, body []byte) error {
		return errors.Join(validators[operationID].Body.Validate(body)...)
	}, validation.PatternOptions())
}

func FuzzJSONRequestBodiesUsesFinalPrograms(f *testing.F) {
	validators, err := validation.Parse(runnerSpec)
	if err != nil {
		f.Fatal(err)
	}

	testgenerator.FuzzJSONRequestBodies(f, runnerSpec, func(operationID string, body []byte) error {
		return errors.Join(validators[operationID].Body.Validate(body)...)
	}, validation.PatternOptions())
}
