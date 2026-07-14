package generate

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGenerateWritesCompiledValidation checks constraints, references, and generated compilation.
func TestGenerateWritesCompiledValidation(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	output, err := os.MkdirTemp(filepath.Join(repo, "pkg"), "generate-fixture-")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(output))
	})

	spec := []byte(`
openapi: 3.0.3
info: {title: generated, version: "1"}
paths:
  /request:
    post:
      operationId: request/path
      requestBody:
        required: true
        content:
          application/json:
            schema: {$ref: '#/components/schemas/Node'}
      responses:
        '204': {description: empty}
components:
  schemas:
    Node:
      type: object
      required: [code, amount, choice]
      additionalProperties: false
      properties:
        code: {type: string, minLength: 2, pattern: '^a+$'}
        amount: {type: number, minimum: 1, multipleOf: 0.5}
        choice: {type: string, enum: [first, second]}
        child: {$ref: '#/components/schemas/Node'}
`)

	err = Generate(output, "generatefixture", spec)
	require.NoError(t, err)

	probe := []byte(`package generatefixture

import "testing"

func TestGeneratedValidation(t *testing.T) {
	valid := []byte(` + "`" + `{
		"code":"aa",
		"amount":1.5,
		"choice":"first",
		"child":{"code":"aaa","amount":2,"choice":"second"}
	}` + "`" + `)
	if errs := validations["request/path"].Validate(valid); len(errs) != 0 {
		t.Fatalf("valid body: %v", errs)
	}

	invalid := []byte(` + "`" + `{"code":"b","amount":1.25,"choice":"third"}` + "`" + `)
	if errs := validations["request/path"].Validate(invalid); len(errs) != 4 {
		t.Fatalf("got %d errors, want 4: %v", len(errs), errs)
	}
}
`)
	require.NoError(t, os.WriteFile(filepath.Join(output, "probe_test.go"), probe, 0o644))

	command := exec.CommandContext(
		t.Context(), "go", "test", "./pkg/"+filepath.Base(output), "-run", "TestGeneratedValidation",
	)
	command.Dir = repo
	result, err := command.CombinedOutput()
	require.NoError(t, err, string(result))
}

// TestGenerateWritesEmptyValidationMap verifies documents without JSON request bodies still generate valid tests.
func TestGenerateWritesEmptyValidationMap(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	output, err := os.MkdirTemp(filepath.Join(repo, "pkg"), "generate-empty-fixture-")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(output))
	})

	err = Generate(output, "generateemptyfixture", []byte(`
openapi: 3.0.3
info: {title: generated, version: "1"}
paths:
  /bodyless:
    get: {}
  /plain:
    post:
      requestBody:
        content:
          text/plain:
            schema: {type: string}
`))
	require.NoError(t, err)

	command := exec.CommandContext(t.Context(), "go", "test", "./pkg/"+filepath.Base(output))
	command.Dir = repo
	result, err := command.CombinedOutput()
	require.NoError(t, err, string(result))
}

// TestGenerateStopsBeforeWritingOnParseError checks the required failure ordering.
func TestGenerateStopsBeforeWritingOnParseError(t *testing.T) {
	t.Parallel()

	output := filepath.Join(t.TempDir(), "output")
	err := Generate(output, "example", []byte("not openapi"))
	require.Error(t, err)

	_, statErr := os.Stat(output)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

// TestGenerateExampleMatchesFixture checks the checked-in generated example.
func TestGenerateExampleMatchesFixture(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	openAPI, err := os.ReadFile(filepath.Join(repo, "resources", "openapi.yaml"))
	require.NoError(t, err)

	output := t.TempDir()
	require.NoError(t, Generate(output, "example", openAPI))

	for _, name := range []string{"validate.go", "validate_test.go"} {
		actual, readErr := os.ReadFile(filepath.Join(output, name))
		require.NoError(t, readErr)

		expected, readErr := os.ReadFile(filepath.Join(repo, "pkg", "decode", "example", name))
		require.NoError(t, readErr)
		require.True(t, bytes.Equal(expected, actual), "%s is stale; run make regen", name)
	}
}

// TestRegenerateExample rewrites the example only through the explicit regen target.
func TestRegenerateExample(t *testing.T) { //nolint:paralleltest // This test explicitly rewrites a shared fixture.
	if os.Getenv("REGENERATE") != "1" {
		t.Skip("set REGENERATE=1")
	}

	repo := repoRoot(t)
	openAPI, err := os.ReadFile(filepath.Join(repo, "resources", "openapi.yaml"))
	require.NoError(t, err)

	require.NoError(t, Generate(
		filepath.Join(repo, "pkg", "decode", "example"),
		"example",
		openAPI,
	))
}

// repoRoot returns the repository root for generator tests.
func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)

	return root
}
