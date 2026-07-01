package testgenerator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBaseNodeValidCasesIncludeNullOnlyWhenNullable(t *testing.T) {
	nullableNode := BaseNode{Nullable: true}
	validCases := nullableNode.ValidCases()
	require.Len(t, validCases, 1)
	require.Equal(t, json.RawMessage(`null`), validCases[0].GenerateValid(nil, nil))
	require.Nil(t, validCases[0].RequiredValid)
	require.Nil(t, validCases[0].RequiredInvalid)

	notNullableNode := BaseNode{Nullable: false}
	require.Empty(t, notNullableNode.ValidCases())
}

func TestBaseNodeInvalidCasesIncludeNullOnlyWhenNotNullable(t *testing.T) {
	nullableNode := BaseNode{Nullable: true}
	require.Empty(t, nullableNode.InvalidCases())

	notNullableNode := BaseNode{Nullable: false}
	invalidCases := notNullableNode.InvalidCases()
	require.Len(t, invalidCases, 1)
	require.Equal(t, json.RawMessage(`null`), invalidCases[0].GenerateValid(nil, nil))
	require.Nil(t, invalidCases[0].RequiredValid)
	require.Nil(t, invalidCases[0].RequiredInvalid)
}
