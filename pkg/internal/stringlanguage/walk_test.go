//nolint:godoclint // Walk tests describe the internal generator seam directly.
package stringlanguage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWalkAcceptsPositiveAndRejectsNegativeRequirements(t *testing.T) {
	t.Parallel()

	positive, err := Pattern(`a+`)
	require.NoError(t, err)
	negative, err := Pattern(`z`)
	require.NoError(t, err)

	walk, err := Begin([]Requirement{
		{Language: positive, WantMatch: true},
		{Language: negative, WantMatch: false},
	})
	require.NoError(t, err)
	require.False(t, walk.Accepting())

	for range 2 {
		require.NoError(t, walk.Advance('a'))
	}

	require.True(t, walk.Accepting())
}

func TestWalkRangesIncludeDeadTransitions(t *testing.T) {
	t.Parallel()

	positive, err := Pattern(`^a$`)
	require.NoError(t, err)
	negative, err := Pattern(`^a$`)
	require.NoError(t, err)
	walk, err := Begin([]Requirement{
		{Language: positive, WantMatch: true},
		{Language: negative, WantMatch: false},
	})
	require.NoError(t, err)

	ranges := walk.Ranges()
	require.NotEmpty(t, ranges)
	require.True(t, scalarRangesContain(ranges, 'b'))

	require.NoError(t, walk.Advance('a'))
	require.True(t, scalarRangesContain(walk.Ranges(), 'b'))
}

func TestWalkAdvanceUsesOneNonBMPScalar(t *testing.T) {
	t.Parallel()

	language, err := Pattern(`^😀$`)
	require.NoError(t, err)
	walk, err := Begin([]Requirement{{Language: language, WantMatch: true}})
	require.NoError(t, err)

	require.NoError(t, walk.Advance('😀'))
	require.Equal(t, language.Matches("😀"), walk.Accepting())
}

func TestWalkEmptyRequirementsAcceptAndExposeScalarUniverse(t *testing.T) {
	t.Parallel()

	walk, err := Begin(nil)
	require.NoError(t, err)
	require.True(t, walk.Accepting())
	require.Equal(t, []ScalarRange{
		{First: 0, Last: firstSurrogate - 1},
		{First: lastSurrogate + 1, Last: maximumScalar},
	}, walk.Ranges())
}

func scalarRangesContain(ranges []ScalarRange, value rune) bool {
	for _, item := range ranges {
		if item.First <= value && value <= item.Last {
			return true
		}
	}

	return false
}
