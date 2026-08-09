package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRankProductCursorUsesCanonicalDiagonals locks tuple order without a queued frontier.
func TestRankProductCursorUsesCanonicalDiagonals(t *testing.T) {
	t.Parallel()

	cursor, err := newRankProductCursor(3)
	require.NoError(t, err)

	var tuples [][]uint64

	for range 10 {
		tuple, ok := cursor.Next()
		require.True(t, ok)

		tuples = append(tuples, append([]uint64(nil), tuple...))
	}

	require.Equal(t, [][]uint64{
		{0, 0, 0},
		{0, 0, 1},
		{0, 1, 0},
		{1, 0, 0},
		{0, 0, 2},
		{0, 1, 1},
		{0, 2, 0},
		{1, 0, 1},
		{1, 1, 0},
		{2, 0, 0},
	}, tuples)
}

// TestRankProductCursorSkipsExhaustedFiniteDimensions proves exact finite exhaustion.
func TestRankProductCursorSkipsExhaustedFiniteDimensions(t *testing.T) {
	t.Parallel()

	cursor, err := newRankProductCursor(2)
	require.NoError(t, err)
	require.NoError(t, cursor.SetFinite(0, 2))
	require.NoError(t, cursor.SetFinite(1, 3))

	var tuples [][]uint64

	for {
		tuple, ok := cursor.Next()
		if !ok {
			break
		}

		tuples = append(tuples, append([]uint64(nil), tuple...))
	}

	require.Equal(t, [][]uint64{
		{0, 0},
		{0, 1},
		{1, 0},
		{0, 2},
		{1, 1},
		{1, 2},
	}, tuples)
}
