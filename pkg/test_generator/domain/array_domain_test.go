package domain

import (
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.

	"github.com/stretchr/testify/require"
)

// TestArrayDomainGenerateHashFinalizesConstraints checks hash-time validation without receiver mutation.
func TestArrayDomainGenerateHashFinalizesConstraints(t *testing.T) {
	t.Parallel()

	domain := ArrayDomain{
		Nullable: true,
		Enum: []types.Enum{
			types.Enum(`["a","b"]`),
			types.Enum(`null`),
			types.Enum(`[]`),
			types.Enum(`["a"]`),
		},
		MinItems: 1,
		MaxItems: new(1),
	}
	before := domain

	got, err := domain.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, before, domain)
	require.Equal(t, requireGeneratedHash(t, "array", arrayHashValue{
		Nullable: true,
		Enum:     []types.Enum{types.Enum(`["a"]`), types.Enum(`null`)},
		MinItems: 1,
		MaxItems: new(1),
	}), got)
}

// TestArrayDomainGenerateHashRejectsUnsatisfiableConstraints covers hash-time failures.
func TestArrayDomainGenerateHashRejectsUnsatisfiableConstraints(t *testing.T) {
	t.Parallel()

	tests := map[string]*ArrayDomain{
		"contradictory item-count bounds": {
			MinItems: 2,
			MaxItems: new(1),
		},
		"enum outside item-count bounds": {
			Enum:     []types.Enum{types.Enum(`[]`)},
			MinItems: 1,
		},
		"nullable enum without null": {
			Nullable: true,
			Enum:     []types.Enum{types.Enum(`["a"]`)},
			MinItems: 2,
			MaxItems: new(1),
		},
	}

	for name, domain := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := domain.GenerateHash()
			require.Error(t, err)
		})
	}
}

// TestArrayDomainGenerateHashAllowsNullOnlySatisfiability checks nullable rescue at hash time.
func TestArrayDomainGenerateHashAllowsNullOnlySatisfiability(t *testing.T) {
	t.Parallel()

	domain := ArrayDomain{
		Nullable: true,
		Enum:     []types.Enum{types.Enum(`["a"]`), types.Enum(`null`)},
		MinItems: 2,
		MaxItems: new(1),
	}

	_, err := domain.GenerateHash()
	require.NoError(t, err)
}
