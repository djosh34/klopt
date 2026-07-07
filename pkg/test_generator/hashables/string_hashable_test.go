package hashables

import (
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types"

	"github.com/stretchr/testify/require"
)

func TestStringHashableImplementsHasher(t *testing.T) {
	require.Implements(t, (*types.Hasher)(nil), new(StringHashable))
}

func TestStringHashableGenerateHash(t *testing.T) {
	hashable := StringHashable{
		Nullable:         true,
		Enum:             []types.Enum{types.Enum("\"alpha\""), types.Enum("\"beta\"")},
		Pattern:          types.Pattern{"^[a-z]+$"},
		Format:           types.Format{"email"},
		XValidExamples:   []string{"alpha"},
		XInvalidExamples: []string{"123"},
		MinLength:        2,
		MaxLength:        new(5),
	}

	expectedHash := types.Hash{0x7d, 0x37, 0xd1, 0x12, 0xa8, 0xa4, 0x86, 0xe1, 0xcd, 0xe1, 0xe0, 0xbc, 0xf0, 0x28, 0x44, 0xd1, 0xae, 0x85, 0x43, 0x71, 0xc0, 0xe2, 0x9b, 0x18, 0x48, 0xd1, 0x50, 0xb, 0x27, 0x60, 0x7c, 0xd0}

	gotHash, err := hashable.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, expectedHash, gotHash)
}

func TestStringHashableGenerateHashNil(t *testing.T) {
	_, err := (*StringHashable)(nil).GenerateHash()
	require.Error(t, err)
}
