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

	err = generateContext.Generate(generateOutputDir)
	require.NoError(t, err)

}
