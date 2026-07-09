package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPatternUnmarshalJSON verifies string decoding and null rejection.
func TestPatternUnmarshalJSON(t *testing.T) {
	t.Parallel()

	var pattern Pattern
	require.NoError(t, json.Unmarshal([]byte(`"^ok$"`), &pattern))
	require.Equal(t, Pattern{"^ok$"}, pattern)
	require.Error(t, json.Unmarshal([]byte(`null`), &pattern))
}

// TestFormatUnmarshalJSON verifies string decoding and null rejection.
func TestFormatUnmarshalJSON(t *testing.T) {
	t.Parallel()

	var format Format
	require.NoError(t, json.Unmarshal([]byte(`"uuid"`), &format))
	require.Equal(t, Format{"uuid"}, format)
	require.Error(t, json.Unmarshal([]byte(`null`), &format))
}
