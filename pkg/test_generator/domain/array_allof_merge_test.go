package domain

import (
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.

	"github.com/stretchr/testify/require"
)

// TestArrayDomainAllOfMergeValidPlanCases covers array domain intersections.
func TestArrayDomainAllOfMergeValidPlanCases(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		left  *ArrayDomain
		right types.Domain
		want  types.Domain
	}{
		"nullable true": {
			left:  &ArrayDomain{Nullable: true},
			right: &ArrayDomain{Nullable: true},
			want:  &ArrayDomain{Nullable: true},
		},
		"nullable different types intersect as null": {
			left:  &ArrayDomain{Nullable: true},
			right: &StringDomain{Nullable: true},
			want: &BoolDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum(`null`)},
			},
		},
		"enum nil right": {
			left: &ArrayDomain{},
			right: &ArrayDomain{
				Enum: []types.Enum{types.Enum(`["a"]`), types.Enum(`["b"]`)},
			},
			want: &ArrayDomain{
				Enum: []types.Enum{types.Enum(`["a"]`), types.Enum(`["b"]`)},
			},
		},
		"enum intersection": {
			left: &ArrayDomain{
				Enum: []types.Enum{
					types.Enum(`["a"]`),
					types.Enum(`["b"]`),
					types.Enum(`["c"]`),
				},
			},
			right: &ArrayDomain{
				Enum: []types.Enum{
					types.Enum(`["b"]`),
					types.Enum(`["c"]`),
					types.Enum(`["d"]`),
				},
			},
			want: &ArrayDomain{
				Enum: []types.Enum{types.Enum(`["b"]`), types.Enum(`["c"]`)},
			},
		},
		"enum uses canonical order": {
			left: &ArrayDomain{
				Enum: []types.Enum{types.Enum(`["b"]`), types.Enum(`["a"]`)},
			},
			right: &ArrayDomain{
				Enum: []types.Enum{types.Enum(`["a"]`), types.Enum(`["b"]`)},
			},
			want: &ArrayDomain{
				Enum: []types.Enum{types.Enum(`["a"]`), types.Enum(`["b"]`)},
			},
		},
		"enum compares nested JSON semantically": {
			left: &ArrayDomain{
				Enum: []types.Enum{types.Enum(`[{"a":1,"b":2}]`)},
			},
			right: &ArrayDomain{
				Enum: []types.Enum{types.Enum(`[{"b":2.0,"a":1e0}]`)},
			},
			want: &ArrayDomain{
				Enum: []types.Enum{types.Enum(`[{"a":1,"b":2}]`)},
			},
		},
		"enum is filtered by merged item-count bounds": {
			left: &ArrayDomain{
				Enum: []types.Enum{
					types.Enum(`[]`),
					types.Enum(`["a"]`),
					types.Enum(`["a","b"]`),
				},
				MinItems: 1,
			},
			right: &ArrayDomain{MaxItems: new(1)},
			want: &ArrayDomain{
				Enum:     []types.Enum{types.Enum(`["a"]`)},
				MinItems: 1,
				MaxItems: new(1),
			},
		},
		"items nil": {
			left:  &ArrayDomain{},
			right: &ArrayDomain{},
			want:  &ArrayDomain{},
		},
		"items nil domain": {
			left:  &ArrayDomain{},
			right: &ArrayDomain{Items: &StringDomain{MinLength: 1}},
			want:  &ArrayDomain{Items: &StringDomain{MinLength: 1}},
		},
		"items domain nil": {
			left:  &ArrayDomain{Items: &StringDomain{MinLength: 1}},
			right: &ArrayDomain{},
			want:  &ArrayDomain{Items: &StringDomain{MinLength: 1}},
		},
		"items string merge": {
			left:  &ArrayDomain{Items: &StringDomain{MinLength: 1}},
			right: &ArrayDomain{Items: &StringDomain{MaxLength: new(5)}},
			want: &ArrayDomain{
				Items: &StringDomain{MinLength: 1, MaxLength: new(5)},
			},
		},
		"items number merge": {
			left:  &ArrayDomain{Items: &NumberDomain{Type: "number"}},
			right: &ArrayDomain{Items: &NumberDomain{Type: "integer"}},
			want:  &ArrayDomain{Items: &NumberDomain{Type: "integer"}},
		},
		"incompatible items permit only an empty array": {
			left:  &ArrayDomain{Items: &StringDomain{}},
			right: &ArrayDomain{Items: &BoolDomain{}},
			want:  &ArrayDomain{MaxItems: new(0)},
		},
		"incompatible items tighten an existing maximum": {
			left:  &ArrayDomain{Items: &StringDomain{}, MaxItems: new(3)},
			right: &ArrayDomain{Items: &BoolDomain{}, MaxItems: new(2)},
			want:  &ArrayDomain{MaxItems: new(0)},
		},
		"incompatible items filter nonempty enum arrays": {
			left: &ArrayDomain{
				Enum:  []types.Enum{types.Enum(`["a"]`), types.Enum(`[]`)},
				Items: &StringDomain{},
			},
			right: &ArrayDomain{Items: &BoolDomain{}},
			want: &ArrayDomain{
				Enum:     []types.Enum{types.Enum(`[]`)},
				MaxItems: new(0),
			},
		},
		"nullable permits incompatible items with positive minimum": {
			left:  &ArrayDomain{Nullable: true, Items: &StringDomain{}, MinItems: 1},
			right: &ArrayDomain{Nullable: true, Items: &BoolDomain{}},
			want: &ArrayDomain{
				Nullable: true,
				MinItems: 1,
				MaxItems: new(0),
			},
		},
		"nullable permits contradictory item-count bounds": {
			left:  &ArrayDomain{Nullable: true, MinItems: 10},
			right: &ArrayDomain{Nullable: true, MaxItems: new(5)},
			want:  &ArrayDomain{Nullable: true, MinItems: 10, MaxItems: new(5)},
		},
		"nullable enum null permits contradictory item-count bounds": {
			left: &ArrayDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum(`null`), types.Enum(`["a"]`)},
				MinItems: 2,
			},
			right: &ArrayDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum(`["a"]`), types.Enum(`null`)},
				MaxItems: new(1),
			},
			want: &ArrayDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum(`null`)},
				MinItems: 2,
				MaxItems: new(1),
			},
		},
		"all fields": {
			left: &ArrayDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum(`["a","b"]`), types.Enum(`["b","c"]`)},
				Items:    &StringDomain{MinLength: 1},
				MinItems: 1,
				MaxItems: new(10),
			},
			right: &ArrayDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum(`["b","c"]`), types.Enum(`["c","d"]`)},
				Items:    &StringDomain{MaxLength: new(5)},
				MinItems: 2,
				MaxItems: new(8),
			},
			want: &ArrayDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum(`["b","c"]`)},
				Items:    &StringDomain{MinLength: 1, MaxLength: new(5)},
				MinItems: 2,
				MaxItems: new(8),
			},
		},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := testCase.left.AllOfMerge(testCase.right)
			require.NoError(t, err)
			require.Equal(t, testCase.want, got)
		})
	}
}

// TestArrayDomainAllOfMergeInvalidPlanCases covers invalid array intersections.
func TestArrayDomainAllOfMergeInvalidPlanCases(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		left  *ArrayDomain
		right types.Domain
	}{
		"nil other": {
			left: &ArrayDomain{},
		},
		"string": {
			left:  &ArrayDomain{},
			right: &StringDomain{},
		},
		"number": {
			left:  &ArrayDomain{},
			right: &NumberDomain{Type: "number"},
		},
		"bool": {
			left:  &ArrayDomain{},
			right: &BoolDomain{},
		},
		"object": {
			left:  &ArrayDomain{},
			right: &ObjectDomain{},
		},
		"enum empty": {
			left:  &ArrayDomain{Enum: []types.Enum{types.Enum(`["a"]`)}},
			right: &ArrayDomain{Enum: []types.Enum{types.Enum(`["b"]`)}},
		},
		"enum nil raw message": {
			left:  &ArrayDomain{Enum: []types.Enum{nil}},
			right: &ArrayDomain{Enum: []types.Enum{types.Enum(`[]`)}},
		},
		"non-nullable null intersection": {
			left:  &ArrayDomain{Enum: []types.Enum{types.Enum(`null`), types.Enum(`["a"]`)}},
			right: &ArrayDomain{Enum: []types.Enum{types.Enum(`["b"]`), types.Enum(`null`)}},
		},
		"incompatible items conflict with positive minimum": {
			left:  &ArrayDomain{Items: &StringDomain{}, MinItems: 1},
			right: &ArrayDomain{Items: &BoolDomain{}},
		},
		"invalid left items domain": {
			left:  &ArrayDomain{Items: failingGenerateHashDomain{}},
			right: &ArrayDomain{},
		},
		"invalid right items domain": {
			left:  &ArrayDomain{},
			right: &ArrayDomain{Items: failingGenerateHashDomain{}},
		},
		"contradictory item-count bounds": {
			left:  &ArrayDomain{MinItems: 10},
			right: &ArrayDomain{MaxItems: new(5)},
		},
		"enum has no values within merged item-count bounds": {
			left: &ArrayDomain{
				Enum:     []types.Enum{types.Enum(`[]`)},
				MinItems: 1,
			},
			right: &ArrayDomain{},
		},
		"nullable enum without null cannot rescue contradictory bounds": {
			left: &ArrayDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum(`["a"]`)},
				MinItems: 2,
			},
			right: &ArrayDomain{
				Nullable: true,
				MaxItems: new(1),
			},
		},
		"incompatible allOf": {
			left: &ArrayDomain{},
			right: &AllOfDomain{
				Domains:      []types.Domain{&StringDomain{}},
				MergedDomain: &StringDomain{},
			},
		},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			before := *testCase.left
			got, err := testCase.left.AllOfMerge(testCase.right)
			require.Error(t, err)
			require.Nil(t, got)
			require.Equal(t, before, *testCase.left)
		})
	}

	t.Run("nil receiver", func(t *testing.T) {
		t.Parallel()

		var left *ArrayDomain

		got, err := left.AllOfMerge(&ArrayDomain{})
		require.Error(t, err)
		require.Nil(t, got)
	})
}
