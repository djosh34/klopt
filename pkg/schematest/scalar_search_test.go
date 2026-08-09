package schematest

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRowScalarValueSourceStopsBeforeTraversingLargeComposition locks lazy first assignment.
func TestRowScalarValueSourceStopsBeforeTraversingLargeComposition(t *testing.T) {
	t.Parallel()

	const memberCount = 20_000

	members := make([]enumMember, memberCount)
	for index := range members {
		members[index] = enumMember{
			value:         &jsonValue{kind: jsonString, text: strconv.Itoa(index)},
			authoredIndex: index,
		}
	}

	leaf := &schemaNode{schemaShape: &schemaShape{enum: members}}
	alternative := &schemaNode{schemaShape: &schemaShape{anyOf: []*schemaNode{leaf}}}
	root := &schemaNode{schemaShape: &schemaShape{allOf: []*schemaNode{alternative}}}

	count := 0
	err := rowScalarValueSource(root, jsonString)(func(value *jsonValue) bool {
		count++

		require.Equal(t, "", value.text)

		return false
	})
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// TestRowAndFaultCandidateSourcesStopBeforeUnrequestedComposition proves first-choice laziness.
func TestRowAndFaultCandidateSourcesStopBeforeUnrequestedComposition(t *testing.T) {
	t.Parallel()

	node := &schemaNode{schemaShape: &schemaShape{
		defaultValue: &jsonValue{kind: jsonString, text: "first"},
		anyOf:        []*schemaNode{nil},
	}}

	rowCount := 0
	err := rowScalarValueSource(node, jsonString)(func(*jsonValue) bool {
		rowCount++

		return false
	})
	require.NoError(t, err)
	require.Equal(t, 1, rowCount)

	faultCount := 0
	err = canonicalEnumFaultWitnesses(node, jsonString)(func(value *jsonValue) bool {
		faultCount++

		require.Equal(t, "first", value.text)

		return false
	})
	require.NoError(t, err)
	require.Equal(t, 1, faultCount)
}

// TestSearchSeedUsesLockedDomainAndFields requirements the shared private seed contract.
func TestSearchSeedUsesLockedDomainAndFields(t *testing.T) {
	t.Parallel()

	require.Equal(t, uint64(0x2740e16489cf1844), searchSeed(
		"#/schema",
		[]byte(`{"pattern":"^[a-z]$","type":"string"}`),
		"pattern",
		"valid",
	))
}
