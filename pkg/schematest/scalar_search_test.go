package schematest

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRowScalarValuesConsumeLargeComposedEnumLinearly pins direct admitted-member consumption.
func TestRowScalarValuesConsumeLargeComposedEnumLinearly(t *testing.T) {
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

	values, err := rowScalarValues(root, jsonString)
	require.NoError(t, err)
	canonical, err := canonicalKindWitnesses(jsonString)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(values), len(canonical)+memberCount)

	for index := range memberCount {
		require.Same(t, members[index].value, values[len(canonical)+index])
	}

	again, err := rowScalarValues(root, jsonString)
	require.NoError(t, err)
	require.Equal(t, values, again)
}

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
