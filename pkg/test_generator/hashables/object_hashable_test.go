package hashables

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeHasher struct{}

func (f fakeHasher) GenerateHash() (types.Hash, error) {
	return types.Hash{1}, nil
}

func TestObjectHashablesImplementHasher(t *testing.T) {
	require.Implements(t, (*types.Hasher)(nil), new(PropertyHashable))
	require.Implements(t, (*types.Hasher)(nil), new(ObjectHashable))
}

func TestPropertyHashableGenerateHash(t *testing.T) {
	hashable := PropertyHashable{Key: "name", Hasher: fakeHasher{}, Required: true}

	expectedHash := types.Hash{0x80, 0xb6, 0x8e, 0x84, 0xf0, 0xbb, 0x33, 0xbf, 0xff, 0xd6, 0x4, 0xb1, 0x74, 0xae, 0x2c, 0x3e, 0x86, 0x81, 0x70, 0x23, 0x27, 0xc8, 0xfa, 0xf1, 0x6b, 0xbd, 0x90, 0x53, 0x38, 0xfe, 0xa, 0x58}

	gotHash, err := hashable.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, expectedHash, gotHash)
}

func TestObjectHashableGenerateHash(t *testing.T) {
	hashable := ObjectHashable{
		Nullable:                 true,
		Properties:               []PropertyHashable{{Hasher: fakeHasher{}}},
		AdditionalPropertyKind:   AdditionalSchema,
		AdditionalPropertyDomain: fakeHasher{},
		MinProps:                 1,
		MaxProps:                 new(3),
	}

	expectedHash := types.Hash{0x61, 0xf9, 0xa9, 0x46, 0x64, 0x6b, 0x53, 0xda, 0xd0, 0x94, 0x11, 0xb3, 0x59, 0xc2, 0x75, 0x55, 0xf1, 0x50, 0xaf, 0xa0, 0xd1, 0x62, 0xb, 0x5a, 0xf4, 0x9f, 0xa2, 0xd4, 0xe9, 0x3b, 0xf4, 0x73}

	gotHash, err := hashable.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, expectedHash, gotHash)
}

func TestObjectHashableGenerateHashNil(t *testing.T) {
	_, err := (*PropertyHashable)(nil).GenerateHash()
	require.Error(t, err)

	_, err = (*ObjectHashable)(nil).GenerateHash()
	require.Error(t, err)
}
