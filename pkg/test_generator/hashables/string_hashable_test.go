package hashables

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringHashableImplementsHasher(t *testing.T) {
	require.Implements(t, (*types.Hasher)(nil), new(StringHashable))
}

func TestStringHashableGenerateHash(t *testing.T) {
	hashable := StringHashable{
		Nullable:         true,
		Enum:             []string{"alpha", "beta"},
		Pattern:          new("^[a-z]+$"),
		Format:           new("email"),
		XValidExamples:   []string{"alpha"},
		XInvalidExamples: []string{"123"},
		MinLength:        2,
		MaxLength:        new(5),
	}

	expectedHash := types.Hash{0x1b, 0x93, 0x23, 0x83, 0x7, 0xa0, 0x69, 0x4e, 0x4, 0x85, 0x42, 0xa4, 0x38, 0xe8, 0x53, 0x3b, 0xf, 0x6, 0xeb, 0xdf, 0x32, 0xcf, 0x99, 0xd2, 0x1a, 0xd7, 0x33, 0xd6, 0x70, 0xa5, 0x83, 0xea}

	gotHash, err := hashable.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, expectedHash, gotHash)
}

func TestStringHashableGenerateHashNil(t *testing.T) {
	_, err := (*StringHashable)(nil).GenerateHash()
	require.Error(t, err)
}
