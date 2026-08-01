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
			matches, err := language.Matches(test.valid)
			require.NoError(t, err)
			require.True(t, matches)
			matches, err = language.Matches(test.invalid)
			require.NoError(t, err)
			require.False(t, matches)
		})
	}
}

func TestLanguagePatternRetainsRE2CaseFolding(t *testing.T) {
	t.Parallel()

	language, err := Pattern(`(?i)^a+$`, patternvalidator.UseRE2)
	require.NoError(t, err)
	matches, err := language.Matches("AaA")
	require.NoError(t, err)
	require.True(t, matches)
	matches, err = language.Matches("b")
	require.NoError(t, err)
	require.False(t, matches)
}

func TestLanguageMatchesRejectsMalformedDFAState(t *testing.T) {
	t.Parallel()

	malformed := []Language{
		{},
		{dfa: dfa{states: []dfaState{{edges: []dfaEdge{{first: 0, last: 1, target: 1}}}}}},
		{dfa: dfa{states: []dfaState{{edges: []dfaEdge{{first: 1, last: 0}}}}}},
	}
	for _, language := range malformed {
		_, err := language.Matches("value")
		require.Error(t, err)
	}

	_, err := minimizeDFA(&malformed[1].dfa)
	require.Error(t, err)
	_, err = advanceDFAState(&malformed[1].dfa, 0, 0)
	require.Error(t, err)
}
