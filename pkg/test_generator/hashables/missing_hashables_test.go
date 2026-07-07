package hashables

import (
	"crypto/sha256"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMissingHashablesImplementHasher(t *testing.T) {
	require.Implements(t, (*types.Hasher)(nil), new(ArrayHashable))
	require.Implements(t, (*types.Hasher)(nil), new(BoolHashable))
	require.Implements(t, (*types.Hasher)(nil), new(NumberHashable))
	require.Implements(t, (*types.Hasher)(nil), new(AllOfHashable))
}

func TestArrayHashableGenerateHash(t *testing.T) {
	hashable := ArrayHashable{Nullable: true, Items: &StringHashable{}, MinItems: 1, MaxItems: new(3)}
	jsonBytes, err := json.Marshal(arrayHashableHashJSON{Type: "array", Value: hashable})
	require.NoError(t, err)

	gotHash, err := hashable.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, types.Hash(sha256.Sum256(jsonBytes)), gotHash)
}

func TestBoolHashableGenerateHash(t *testing.T) {
	hashable := BoolHashable{Nullable: true, Enum: []types.Enum{types.Enum("true"), types.Enum("false")}}
	jsonBytes, err := json.Marshal(boolHashableHashJSON{Type: "bool", Value: hashable})
	require.NoError(t, err)

	gotHash, err := hashable.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, types.Hash(sha256.Sum256(jsonBytes)), gotHash)
}

func TestNumberHashableGenerateHash(t *testing.T) {
	minimum := Number("1")
	maximum := Number("10")
	multipleOf := Number("0.5")
	hashable := NumberHashable{Type: "number", Nullable: true, Enum: []types.Enum{types.Enum("1"), types.Enum("2")}, Minimum: &minimum, Maximum: &maximum, ExclusiveMinimum: true, ExclusiveMaximum: true, MultipleOf: &multipleOf, Format: new("float")}
	jsonBytes, err := json.Marshal(numberHashableHashJSON{Type: "number", Value: hashable})
	require.NoError(t, err)

	gotHash, err := hashable.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, types.Hash(sha256.Sum256(jsonBytes)), gotHash)
}

func TestAllOfHashableGenerateHash(t *testing.T) {
	hashable := AllOfHashable{Domains: []types.Hasher{&StringHashable{}}, MergedDomain: &ObjectHashable{}}
	jsonBytes, err := json.Marshal(allOfHashableHashJSON{Type: "allOf", Value: hashable})
	require.NoError(t, err)

	gotHash, err := hashable.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, types.Hash(sha256.Sum256(jsonBytes)), gotHash)
}

func TestMissingHashablesGenerateHashNil(t *testing.T) {
	_, err := (*ArrayHashable)(nil).GenerateHash()
	require.Error(t, err)
	_, err = (*BoolHashable)(nil).GenerateHash()
	require.Error(t, err)
	_, err = (*NumberHashable)(nil).GenerateHash()
	require.Error(t, err)
	_, err = (*AllOfHashable)(nil).GenerateHash()
	require.Error(t, err)
}
