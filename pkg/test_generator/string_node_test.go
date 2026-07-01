package testgenerator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringNodeValidCasesAlwaysIncludeString(t *testing.T) {
	for name, node := range map[string]StringNode{
		"nullable":     {BaseNode: BaseNode{Nullable: true}},
		"not nullable": {BaseNode: BaseNode{Nullable: false}},
	} {
		t.Run(name, func(t *testing.T) {
			cases := node.ValidCases()
			require.NotEmpty(t, cases)
			require.Equal(t, Case{Name: "string", Value: json.RawMessage(`"valid-string"`)}, cases[0])
		})
	}
}

func TestStringNodeValidCasesIncludeNullOnlyWhenNullable(t *testing.T) {
	nullableNode := StringNode{BaseNode: BaseNode{Nullable: true}}
	nullableCases := nullableNode.ValidCases()
	require.Len(t, nullableCases, 2)
	require.Equal(t, Case{Name: "null", Value: json.RawMessage(`null`)}, nullableCases[1])

	notNullableNode := StringNode{BaseNode: BaseNode{Nullable: false}}
	notNullableCases := notNullableNode.ValidCases()
	require.Len(t, notNullableCases, 1)
}

func TestStringNodeInvalidCasesIncludeNullOnlyWhenNotNullable(t *testing.T) {
	nullableNode := StringNode{BaseNode: BaseNode{Nullable: true}}
	require.Empty(t, nullableNode.InvalidCases())

	notNullableNode := StringNode{BaseNode: BaseNode{Nullable: false}}
	invalidCases := notNullableNode.InvalidCases()
	require.Len(t, invalidCases, 1)
	require.Equal(t, Case{Name: "null", Value: json.RawMessage(`null`)}, invalidCases[0])
}
