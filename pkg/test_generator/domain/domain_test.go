package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestContextParseRejectsMissingType verifies that untyped schemas are outside the supported subset.
func TestContextParseRejectsMissingType(t *testing.T) {
	t.Parallel()

	node := json.RawMessage(`{"nullable":true}`)
	dc := Context{}

	domain, err := dc.Parse(&node)
	require.Error(t, err)
	require.Nil(t, domain)
	require.Empty(t, dc.domainStore)
}

// TestContextParseRejectsUnknownType verifies that unsupported schema types fail parsing.
func TestContextParseRejectsUnknownType(t *testing.T) {
	t.Parallel()

	node := json.RawMessage(`{"type":"unknown"}`)
	dc := Context{}

	domain, err := dc.Parse(&node)
	require.Error(t, err)
	require.Nil(t, domain)
	require.Empty(t, dc.domainStore)
}

// TestContextParseRejectsMixedTypeArray verifies that OpenAPI 3.0's string-only type field is enforced.
func TestContextParseRejectsMixedTypeArray(t *testing.T) {
	t.Parallel()

	node := json.RawMessage(`{"type":["string","integer"]}`)
	dc := Context{}

	domain, err := dc.Parse(&node)
	require.Error(t, err)
	require.Nil(t, domain)
	require.Empty(t, dc.domainStore)
}

// TestContextParseRejectsNullType verifies that null is not treated as an empty type string.
func TestContextParseRejectsNullType(t *testing.T) {
	t.Parallel()

	node := json.RawMessage(`{"type":null}`)
	dc := Context{}

	domain, err := dc.Parse(&node)
	require.ErrorContains(t, err, "schema type must be string")
	require.Nil(t, domain)
	require.Empty(t, dc.domainStore)
}
