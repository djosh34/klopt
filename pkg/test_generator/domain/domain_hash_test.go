package domain

import (
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Tests assert the shared domain contract.

	"github.com/stretchr/testify/require"
)

// TestDomainsGenerateHash verifies the deterministic hash input for every domain implementation.
func TestDomainsGenerateHash(t *testing.T) {
	t.Parallel()

	minimum := Number("1")
	maximum := Number("10")
	multipleOf := Number("2")

	stringHash, err := (&StringDomain{}).GenerateHash()
	require.NoError(t, err)

	objectHash, err := (&ObjectDomain{}).GenerateHash()
	require.NoError(t, err)

	tests := map[string]struct {
		domain   types.Domain
		hashType string
		value    any
	}{
		"array": {
			domain: &ArrayDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum(`["item"]`)},
				Items:    &StringDomain{},
				MinItems: 1,
				MaxItems: new(3),
			},
			hashType: "array",
			value: arrayHashValue{
				Nullable: true,
				Enum:     []types.Enum{types.Enum(`["item"]`)},
				Items:    &stringHash,
				MinItems: 1,
				MaxItems: new(3),
			},
		},
		"array with nil items": {
			domain:   &ArrayDomain{Nullable: true},
			hashType: "array",
			value:    arrayHashValue{Nullable: true},
		},
		"bool": {
			domain:   &BoolDomain{Nullable: true, Enum: []types.Enum{types.Enum("true")}},
			hashType: "bool",
			value:    BoolDomain{Nullable: true, Enum: []types.Enum{types.Enum("true")}},
		},
		"number": {
			domain: &NumberDomain{
				Type:             "number",
				Nullable:         true,
				Enum:             []types.Enum{types.Enum("2")},
				Minimum:          &minimum,
				Maximum:          &maximum,
				ExclusiveMinimum: true,
				ExclusiveMaximum: true,
				MultipleOf:       &multipleOf,
				Format:           new("double"),
			},
			hashType: "number",
			value: NumberDomain{
				Type:             "number",
				Nullable:         true,
				Enum:             []types.Enum{types.Enum("2")},
				Minimum:          &minimum,
				Maximum:          &maximum,
				ExclusiveMinimum: true,
				ExclusiveMaximum: true,
				MultipleOf:       &multipleOf,
				Format:           new("double"),
			},
		},
		"integer": {
			domain: &NumberDomain{
				Type:     "integer",
				Nullable: true,
				Enum:     []types.Enum{types.Enum("1")},
				Minimum:  &minimum,
				Format:   new("int32"),
			},
			hashType: "integer",
			value: NumberDomain{
				Type:     "integer",
				Nullable: true,
				Enum:     []types.Enum{types.Enum("1")},
				Minimum:  &minimum,
				Format:   new("int32"),
			},
		},
		"allOf": {
			domain: &AllOfDomain{
				Domains:      []types.Domain{&StringDomain{}},
				MergedDomain: &ObjectDomain{},
			},
			hashType: "allOf",
			value: allOfHashValue{
				Domains:      []*types.Hash{&stringHash},
				MergedDomain: &objectHash,
			},
		},
		"allOf with nil domain": {
			domain:   &AllOfDomain{Domains: []types.Domain{nil}},
			hashType: "allOf",
			value:    allOfHashValue{Domains: []*types.Hash{nil}},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.domain.GenerateHash()
			require.NoError(t, err)
			require.Equal(t, requireGeneratedHash(t, tt.hashType, tt.value), got)
		})
	}
}

// TestDomainsGenerateHashNil verifies that typed nil receivers return errors instead of panicking.
func TestDomainsGenerateHashNil(t *testing.T) {
	t.Parallel()

	_, err := (*ArrayDomain)(nil).GenerateHash()
	require.Error(t, err)
	_, err = (*BoolDomain)(nil).GenerateHash()
	require.Error(t, err)
	_, err = (*NumberDomain)(nil).GenerateHash()
	require.Error(t, err)
	_, err = (*AllOfDomain)(nil).GenerateHash()
	require.Error(t, err)
}
