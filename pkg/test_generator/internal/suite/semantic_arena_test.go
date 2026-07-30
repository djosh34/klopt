//nolint:godoclint // Behavior test names are the canonical set specifications.
package suite

import (
	"testing"

	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/stretchr/testify/require"
)

func TestSetArenaCanonicalConstructorsPreserveSetLawsWithoutDistribution(t *testing.T) {
	t.Parallel()

	arena := NewSetArena()
	strings, err := arena.Atom(kindAtom{Kind: jsonvalue.KindString})
	require.NoError(t, err)
	numbers, err := arena.Atom(kindAtom{Kind: jsonvalue.KindNumber})
	require.NoError(t, err)
	booleans, err := arena.Atom(kindAtom{Kind: jsonvalue.KindBoolean})
	require.NoError(t, err)

	eitherScalar, err := arena.Union(strings, numbers)
	require.NoError(t, err)
	actual, err := arena.Intersect(booleans, eitherScalar, strings, strings)
	require.NoError(t, err)

	empty, err := arena.IsEmpty(actual)
	require.NoError(t, err)
	require.True(t, empty)

	contradiction, err := arena.Intersect(strings, Complement(strings))
	require.NoError(t, err)
	empty, err = arena.IsEmpty(contradiction)
	require.NoError(t, err)
	require.True(t, empty)

	tautology, err := arena.Union(numbers, Complement(numbers))
	require.NoError(t, err)
	require.True(t, arena.IsUniversal(tautology))
	require.Equal(t, strings, Complement(Complement(strings)))
}

func TestSetArenaMembershipUsesExactLiteralAndContainerAtoms(t *testing.T) {
	t.Parallel()

	arena := NewSetArena()
	stringKind, err := arena.Atom(kindAtom{Kind: jsonvalue.KindString})
	require.NoError(t, err)
	enumerated, err := arena.Atom(enumAtom{Values: []jsonvalue.Value{
		jsonvalue.String("ok"),
		jsonvalue.Bool(true),
	}})
	require.NoError(t, err)
	accepted, err := arena.Intersect(stringKind, enumerated)
	require.NoError(t, err)

	for _, test := range []struct {
		value jsonvalue.Value
		want  bool
	}{
		{value: jsonvalue.String("ok"), want: true},
		{value: jsonvalue.String("no"), want: false},
		{value: jsonvalue.Bool(true), want: false},
	} {
		got, containsErr := arena.Contains(accepted, test.value)
		require.NoError(t, containsErr)
		require.Equal(t, test.want, got)
	}

	objectKind, err := arena.Atom(kindAtom{Kind: jsonvalue.KindObject})
	require.NoError(t, err)
	required, err := arena.Atom(requiredPropertyAtom{Name: "x"})
	require.NoError(t, err)
	property, err := arena.Atom(propertyValuesAtom{Name: "x", Values: accepted})
	require.NoError(t, err)
	objectSet, err := arena.Intersect(objectKind, required, property)
	require.NoError(t, err)

	valid, err := jsonvalue.Object([]jsonvalue.Member{{Name: "x", Value: jsonvalue.String("ok")}})
	require.NoError(t, err)
	invalid, err := jsonvalue.Object([]jsonvalue.Member{{Name: "x", Value: jsonvalue.String("no")}})
	require.NoError(t, err)

	got, err := arena.Contains(objectSet, valid)
	require.NoError(t, err)
	require.True(t, got)
	got, err = arena.Contains(objectSet, invalid)
	require.NoError(t, err)
	require.False(t, got)
}
