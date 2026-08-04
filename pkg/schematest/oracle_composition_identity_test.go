//nolint:godoclint // Composition failure identity is pinned at the private oracle seam.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluateAnyOfAggregateFailureHasParentIdentityAfterBranchFailures(t *testing.T) {
	t.Parallel()

	result := evaluateSchemaValue(
		t,
		`{"anyOf":[{"type":"string","pattern":"^a"},{"type":"number","minimum":2}]}`,
		`1`,
	)

	require.NoError(t, result.err)
	require.False(t, result.valid)
	require.Equal(
		t,
		[]string{
			"#/paths/~1/post/requestBody/content/application~1json/schema/anyOf/0|#|type",
			"#/paths/~1/post/requestBody/content/application~1json/schema/anyOf/1|#|minimum",
			"#/paths/~1/post/requestBody/content/application~1json/schema|#|anyOf",
		},
		identityStrings(result.failures),
	)
}
