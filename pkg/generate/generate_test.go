package generate

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGenerateWritesCompiledValidation covers every exported validation field and generated compilation.
func TestGenerateWritesCompiledValidation(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	output, err := os.MkdirTemp(filepath.Join(repo, "pkg"), "generate-fixture-")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(output))
	})

	spec := []byte(strings.Join([]string{
		"openapi: 3.0.3",
		"info: {title: generated, version: \"1\"}",
		"paths:",
		"  /zeta:",
		"    post:",
		"      operationId: zetaRequest",
		"      requestBody:",
		"        content:",
		"          application/json:",
		"            schema: {type: boolean}",
		"      responses:",
		"        '204': {description: empty}",
		"  /alpha:",
		"    post:",
		"      operationId: alphaRequest",
		"      requestBody:",
		"        required: true",
		"        content:",
		"          application/json:",
		"            schema:",
		"              type: object",
		"              nullable: true",
		"              minProperties: 4",
		"              maxProperties: 6",
		"              required: [array, enum, number, text]",
		"              additionalProperties: {type: string, minLength: 1}",
		"              properties:",
		"                array:",
		"                  type: array",
		"                  nullable: true",
		"                  minItems: 1",
		"                  maxItems: 3",
		"                  uniqueItems: true",
		"                  items: {type: integer, minimum: 1, maximum: 5, multipleOf: 1}",
		"                enum:",
		"                  enum:",
		"                    - null",
		"                    - false",
		"                    - true",
		"                    - 0",
		"                    - ''",
		"                    - []",
		"                    - {}",
		"                    - [{nested: [false, 2, x]}]",
		"                    - {nested: [null, {x: []}]}",
		"                number:",
		"                  type: number",
		"                  minimum: 1",
		"                  exclusiveMinimum: true",
		"                  maximum: 10",
		"                  exclusiveMaximum: true",
		"                  multipleOf: 0.5",
		"                text:",
		"                  type: string",
		"                  minLength: 3",
		"                  maxLength: 30",
		"                  pattern: '^[^@]+@[^@]+$'",
		"                  format: email",
		"                closed:",
		"                  type: object",
		"                  additionalProperties: false",
		"                  properties:",
		"                    child: {type: string}",
		"              allOf:",
		"                - {minProperties: 4}",
		"                - properties:",
		"                    flag: {type: boolean}",
		"      responses:",
		"        '204': {description: empty}",
	}, "\n"))

	err = Generate(output, "generatefixture", spec)
	require.NoError(t, err)

	generated, err := os.ReadFile(filepath.Join(output, "validate.go"))
	require.NoError(t, err)

	generatedSource := string(generated)

	for _, field := range []string{
		"SchemaPointer:", "BodyRequired:", "KindValidation:", "Type:", "Nullable:",
		"EnumValidation:", "Values:", "ExactValues:", "ExactValue:", "NumberValidation:", "Minimum:",
		"Maximum:", "Exclusive:", "MultipleOf:", "ExactMultipleOf:", "StringValidation:",
		"MinLength:", "MaxLength:", "Pattern:", "Format:", "CompiledPattern:",
		"ArrayValidation:", "MinItems:", "MaxItems:", "Items:", "UniqueItems:",
		"ObjectValidation:", "MinProperties:", "MaxProperties:", "Required:", "Properties:", "Name:", "Validation:",
		"AdditionalPropertiesAllowed:", "AdditionalPropertiesValidation:", "AllOfValidations:",
	} {
		require.Contains(t, generatedSource, field)
	}

	for _, nestedLocation := range []string{
		"/properties/array/items",
		"/properties/closed/properties/child",
		"/additionalProperties",
		"/allOf/0",
	} {
		require.Contains(t, generatedSource, nestedLocation)
	}

	require.NotContains(t, generatedSource, "func")
	require.NotContains(t, generatedSource, "nodes :=")
	require.NotContains(t, generatedSource, ".Validation =")
	require.Equal(t, 3, strings.Count(generatedSource, "\nvar "))
	require.Less(
		t,
		strings.Index(generatedSource, "var alphaRequest"),
		strings.Index(generatedSource, "var zetaRequest"),
	)
	require.Less(
		t,
		strings.Index(generatedSource, "var zetaRequest"),
		strings.Index(generatedSource, "var validations"),
	)

	probe := []byte(`package generatefixture

import "testing"

func TestGeneratedValidation(t *testing.T) {
	enumValues := []string{
		"null",
		"false",
		"true",
		"0",
		"\"\"",
		"[]",
		"{}",
		"[{\"nested\":[false,2,\"x\"]}]",
		"{\"nested\":[null,{\"x\":[]}]}",
	}
	for _, enumValue := range enumValues {
		body := []byte(
			"{\"array\":[2],\"enum\":" + enumValue +
				",\"number\":1.5,\"text\":\"a@b.co\",\"extra\":\"ok\"}",
		)
		if errs := validations["alphaRequest"].Validate(body); len(errs) != 0 {
			t.Fatalf("valid enum %s: %v", enumValue, errs)
		}
	}

	invalid := []byte(
		"{\"array\":[2,2],\"enum\":\"missing\",\"number\":1,\"text\":\"bad\",\"extra\":1}",
	)
	if errs := validations["alphaRequest"].Validate(invalid); len(errs) == 0 {
		t.Fatal("invalid body passed")
	}
	if errs := validations["alphaRequest"].Validate([]byte("null")); len(errs) != 0 {
		t.Fatalf("nullable body: %v", errs)
	}
	if errs := validations["zetaRequest"].Validate([]byte("true")); len(errs) != 0 {
		t.Fatalf("zeta body: %v", errs)
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

// TestGenerateRejectsUnsafeOperationIdentifiers checks generated package-scope name conflicts.
func TestGenerateRejectsUnsafeOperationIdentifiers(t *testing.T) {
	t.Parallel()

	for _, operationID := range []string{
		"validations",
		"init",
		"_",
		"validation",
		"openAPI",
		"TestValidations",
		"request/path",
		"type",
		"json",
		"regexp",
		"jsonvalue",
		"errors",
		"testing",
		"testgenerator",
		"string",
		"byte",
		"error",
		"true",
	} {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()

			output := filepath.Join(t.TempDir(), "output")
			spec := fmt.Appendf(nil, `
openapi: 3.0.3
info: {title: generated, version: "1"}
paths:
  /request:
    post:
      operationId: %q
      requestBody:
        content:
          application/json:
            schema: {type: string}
`, operationID)
			err := Generate(output, "example", spec)
			require.ErrorContains(t, err, fmt.Sprintf("operation ID %q", operationID))

			_, statErr := os.Stat(output)
			require.ErrorIs(t, statErr, os.ErrNotExist)
		})
	}
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
