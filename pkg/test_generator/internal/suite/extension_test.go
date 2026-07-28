package suite

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSuiteIgnoresGenericSchemaExtensions verifies arbitrary extension values have no suite semantics.
func TestSuiteIgnoresGenericSchemaExtensions(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: string
pattern: ^a$
x-project-metadata:
  arbitrary: [shape]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)
	require.NotEmpty(t, compiled.Cases)
}
