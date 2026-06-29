package generate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func GetRepoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}

		parent := filepath.Dir(wd)
		require.NotEqual(t, wd, parent)

		wd = parent
	}
}

func TestGenerateExample(t *testing.T) {

	openapiExamplePath := filepath.Join(GetRepoRoot(t), "pkg", "decode", "example", "openapi.yaml")
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	generateOutputDir := filepath.Join(GetRepoRoot(t), "pkg", "decode", "example_gen")

	err = generateContext.FilterOperations("objectKeysAdditionalPropertiesFalse")
	require.NoError(t, err)

	err = generateContext.Generate(generateOutputDir)
	require.NoError(t, err)

}

func TestGeneratePopulatesOperationsMap(t *testing.T) {
	openapiExamplePath := filepath.Join(GetRepoRoot(t), "pkg", "decode", "example", "openapi.yaml")
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	err = generateContext.FilterOperations("objectKeysAdditionalPropertiesFalse", "stringNoFormatNullable")
	require.NoError(t, err)

	err = generateContext.Generate(t.TempDir())
	require.NoError(t, err)

	require.Equal(t, map[string]SchemaObject{
		"objectKeysAdditionalPropertiesFalse": ObjectContext{
			AdditionalProperties: false,
			Required: []string{
				"requiredNullableString",
				"requiredNotNullableString",
			},
			Properties: map[string]SchemaObject{
				"requiredNullableString":    StringContext{Nullable: true},
				"requiredNotNullableString": StringContext{},
				"optionalNullableString":    StringContext{Nullable: true},
				"optionalNotNullableString": StringContext{},
			},
		},
		"stringNoFormatNullable": StringContext{Nullable: true},
	}, generateContext.Operations)
}

func TestFilterOperationsKeepsOnlyRequestedOperation(t *testing.T) {
	openapiExamplePath := filepath.Join(GetRepoRoot(t), "pkg", "decode", "example", "openapi.yaml")
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	err = generateContext.FilterOperations("objectKeysAdditionalPropertiesFalse")
	require.NoError(t, err)

	require.Equal(t, []string{"/object-keys-additional-properties-false"}, generateContext.Document.Paths.InMatchingOrder())
	operation := generateContext.Document.Paths.Value("/object-keys-additional-properties-false").Post
	require.NotNil(t, operation)
	require.Equal(t, "objectKeysAdditionalPropertiesFalse", operation.OperationID)
}

func TestFilterOperationsReturnsErrorWhenOperationMissing(t *testing.T) {
	openapiExamplePath := filepath.Join(GetRepoRoot(t), "pkg", "decode", "example", "openapi.yaml")
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	err = generateContext.FilterOperations("notAnOperation")
	require.ErrorContains(t, err, "operation not found: [notAnOperation]")
}
