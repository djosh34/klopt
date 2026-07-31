//nolint:godoclint // Format cases document the direct matching seam.
package stringlanguage

import (
	"testing"

	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/stretchr/testify/require"
)

func TestLanguageMatchesRetainsFormatSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format  string
		valid   string
		invalid string
	}{
		{format: "date", valid: "2024-02-29", invalid: "2024-02-30"},
		{format: "date-time", valid: "2026-07-14T12:30:00Z", invalid: "2026-07-14Z"},
		{format: "email", valid: "first.last@example.com", invalid: "a..b@example.com"},
		{format: "uuid", valid: "a1234567-1234-4234-9234-123456789abc", invalid: "a1234567-1234-1234-9234-123456789abc"},
		{format: "ipv4", valid: "192.0.2.1", invalid: "256.0.2.1"},
		{format: "cidr", valid: "192.0.2.7/24", invalid: "192.0.2.7/33"},
		{format: "byte", valid: "YQ==", invalid: "%%%"},
	}

	for _, test := range tests {
		t.Run(test.format, func(t *testing.T) {
			t.Parallel()

			language, err := Format(test.format)
			require.NoError(t, err)
			require.True(t, language.Matches(test.valid))
			require.False(t, language.Matches(test.invalid))
		})
	}
}

func TestLanguagePatternRetainsRE2CaseFolding(t *testing.T) {
	t.Parallel()

	language, err := Pattern(`(?i)^a+$`, patternvalidator.UseRE2)
	require.NoError(t, err)
	require.True(t, language.Matches("AaA"))
	require.False(t, language.Matches("b"))
}
