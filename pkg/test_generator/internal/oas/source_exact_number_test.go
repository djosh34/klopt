package oas

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParsePreservesExactJSONNumbers verifies JSON input does not take a lossy
// trip through a YAML scalar representation.
func TestParsePreservesExactJSONNumbers(t *testing.T) {
	t.Parallel()

	source, err := Parse([]byte(`{
  "openapi":"3.0.3",
  "info":{"title":"exact","version":"1"},
  "paths":{"/things":{"post":{
    "operationId":"create",
    "requestBody":{"content":{"application/json":{"schema":{
      "minimum":1.234567890123456789e-100,
      "maximum":1e400
    }}}},
    "responses":{"204":{"description":"done"}}
  }}}
}`), "create")
	require.NoError(t, err)
	require.True(t, bytes.Contains(source.RequestSchema.Raw, []byte("1.234567890123456789e-100")))
	require.True(t, bytes.Contains(source.RequestSchema.Raw, []byte("1e400")))
}
