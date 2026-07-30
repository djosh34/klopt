//nolint:godoclint // Behavior tests use readable names instead of redundant prose comments.
package suite_test

import (
	"testing"

	"github.com/djosh34/klopt/pkg/internal/program" //nolint:depguard // Suite tests configure the private program limit seam.
	"github.com/djosh34/klopt/pkg/test_generator/internal/suite"
	"github.com/stretchr/testify/require"
)

func TestCompileSuiteBuildsOneDocumentProgram(t *testing.T) {
	t.Parallel()

	compiled, err := suite.CompileSuite([]byte(`
openapi: 3.0.4
info: {title: test, version: 1.0.0}
paths:
  /first:
    post:
      operationId: first
      requestBody:
        content:
          application/json:
            schema: {type: string, minLength: 1}
      responses: {"204": {description: ok}}
  /second:
    post:
      operationId: second
      requestBody:
        content:
          application/json:
            schema: {type: integer, minimum: 2}
      responses: {"204": {description: ok}}
`))
	require.NoError(t, err)
	require.Equal(t, []suite.OperationInfo{
		{ID: "first", Method: "post", Path: "/first"},
		{ID: "second", Method: "post", Path: "/second"},
	}, compiled.Operations)

	limits := program.Limits{
		MaxSteps: 100_000, MaxOutputBytes: 1_000_000, MaxDepth: 64,
		MaxSolverWork: 100_000, MaxSolverBytes: 16_000_000,
	}
	first, err := compiled.Program.Decode(nil, limits)
	require.NoError(t, err)
	require.Equal(t, program.OperationID(0), first.Operation)

	secondInput := make([]byte, 8)
	secondInput[0] = 1
	second, err := compiled.Program.Decode(secondInput, limits)
	require.NoError(t, err)
	require.Equal(t, program.OperationID(1), second.Operation)
}
