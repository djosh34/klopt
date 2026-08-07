package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSearchSeedUsesLockedDomainAndFields pins the shared private seed contract.
func TestSearchSeedUsesLockedDomainAndFields(t *testing.T) {
	t.Parallel()

	require.Equal(t, uint64(0x2740e16489cf1844), searchSeed(
		"#/schema",
		[]byte(`{"pattern":"^[a-z]$","type":"string"}`),
		"pattern",
		"valid",
	))
}
