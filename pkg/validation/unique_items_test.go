package validation

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUniqueItemsTraversalPropagatesSchemaDecodeErrors preserves malformed-schema diagnostics.
func TestUniqueItemsTraversalPropagatesSchemaDecodeErrors(t *testing.T) {
	t.Parallel()

	walker := authoredSchemaWalker{visited: make(map[string]struct{})}
	err := walker.schema(json.RawMessage(`{"items":]}`), "#/components/schemas/Broken")

	require.ErrorContains(t, err, "decode schema at #/components/schemas/Broken")

	var syntaxErr *json.SyntaxError
	require.ErrorAs(t, err, &syntaxErr)
}
