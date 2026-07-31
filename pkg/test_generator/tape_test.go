//nolint:godoclint // Package-private semantic tests document the tape model.
package testgenerator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTapeEmptyAndShortWordsAreZeroFilled(t *testing.T) {
	t.Parallel()

	short := []byte{0x01, 0x02, 0x03}
	explicit := []byte{0x01, 0x02, 0x03, 0, 0, 0, 0, 0}

	shortCursor := newTapeCursor(short)
	explicitCursor := newTapeCursor(explicit)
	require.Equal(t, explicitCursor.takeWord(), shortCursor.takeWord())
	require.Equal(t, uint64(0), shortCursor.takeWord())
	require.Equal(t, uint64(0), shortCursor.takeWord())

	emptyCursor := newTapeCursor(nil)
	zeroCursor := newTapeCursor(make([]byte, tapeWordBytes*2))
	require.Equal(t, zeroCursor.takeWord(), emptyCursor.takeWord())
	require.Equal(t, zeroCursor.takeWord(), emptyCursor.takeWord())
}

func TestTapeChooseRejectsNoAlternatives(t *testing.T) {
	t.Parallel()

	_, err := newTapeCursor(nil).choose(0)
	require.EqualError(t, err, "choose called with no alternatives")
}
