package domain

import (
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMissingDomainsToHasher(t *testing.T) {
	minimum := Number("1")
	maximum := Number("10")
	multipleOf := Number("2")

	tests := map[string]struct {
		domain   types.Domain
		expected types.Hasher
	}{
		"array": {
			domain:   &ArrayDomain{Nullable: true, Items: &StringDomain{}, MinItems: 1, MaxItems: new(3)},
			expected: &hashables.ArrayHashable{Nullable: true, Items: &hashables.StringHashable{}, MinItems: 1, MaxItems: new(3)},
		},
		"array with nil items": {
			domain:   &ArrayDomain{Nullable: true},
			expected: &hashables.ArrayHashable{Nullable: true},
		},
		"bool": {
			domain:   &BoolDomain{Nullable: true, Enum: []bool{true}},
			expected: &hashables.BoolHashable{Nullable: true, Enum: []bool{true}},
		},
		"number": {
			domain:   &NumberDomain{Nullable: true, Enum: []Number{Number("1")}, Minimum: &minimum, Maximum: &maximum, ExclusiveMinimum: true, ExclusiveMaximum: true, MultipleOf: &multipleOf, Format: new("double")},
			expected: &hashables.NumberHashable{Nullable: true, Enum: []hashables.Number{hashables.Number("1")}, Minimum: new(hashables.Number("1")), Maximum: new(hashables.Number("10")), ExclusiveMinimum: true, ExclusiveMaximum: true, MultipleOf: new(hashables.Number("2")), Format: new("double")},
		},
		"integer": {
			domain:   &IntegerDomain{Nullable: true, Enum: []Number{Number("1")}, Minimum: &minimum, Format: new("int32")},
			expected: &hashables.IntegerHashable{Nullable: true, Enum: []hashables.Number{hashables.Number("1")}, Minimum: new(hashables.Number("1")), Format: new("int32")},
		},
		"allOf": {
			domain: &AllOfDomain{Domains: []types.Domain{&StringDomain{}}, MergedDomain: &ObjectDomain{}},
			expected: &hashables.AllOfHashable{Domains: []types.Hasher{&hashables.StringHashable{}}, MergedDomain: &hashables.ObjectHashable{
				Enum:       []types.Hasher{},
				Properties: []types.Hasher{},
			}},
		},
		"allOf with nil domain": {
			domain:   &AllOfDomain{Domains: []types.Domain{nil}},
			expected: &hashables.AllOfHashable{Domains: []types.Hasher{nil}},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.domain.ToHasher()
			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestMissingDomainsToHasherNil(t *testing.T) {
	_, err := (*ArrayDomain)(nil).ToHasher()
	require.Error(t, err)
	_, err = (*BoolDomain)(nil).ToHasher()
	require.Error(t, err)
	_, err = (*NumberDomain)(nil).ToHasher()
	require.Error(t, err)
	_, err = (*IntegerDomain)(nil).ToHasher()
	require.Error(t, err)
	_, err = (*AllOfDomain)(nil).ToHasher()
	require.Error(t, err)
}
