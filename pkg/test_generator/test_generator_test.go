//nolint:godoclint // The Stage 3 test checks the valid public seam.
package testgenerator_test

import (
	"testing"

	testgenerator "github.com/djosh34/klopt/pkg/test_generator"
	"github.com/stretchr/testify/require"
)

func TestGeneratorCompilesAndGeneratesValidBody(t *testing.T) {
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
            schema: {type: string}
      responses: {"204": {description: ok}}
`))
	require.NoError(t, err)

	sample, status, err := compiled.Decode(nil)
	require.NoError(t, err)
	require.Equal(t, testgenerator.Generated, status)
	require.Equal(t, "createThing", sample.OperationID)
	require.Equal(t, []byte(`""`), sample.Body)
	require.True(t, sample.ExpectValid)
}
