package domain

import (
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.

	"github.com/stretchr/testify/require"
)

// TestBoolDomainAllOfMergeValidPlanCases covers boolean domain intersections.
func TestBoolDomainAllOfMergeValidPlanCases(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		left  *BoolDomain
		right types.Domain
		want  types.Domain
	}{
		"nullable false": {
			left:  &BoolDomain{},
			right: &BoolDomain{},
			want:  &BoolDomain{},
		},
		"nullable false true": {
			left:  &BoolDomain{},
			right: &BoolDomain{Nullable: true},
			want:  &BoolDomain{},
		},
		"nullable true false": {
			left:  &BoolDomain{Nullable: true},
			right: &BoolDomain{},
			want:  &BoolDomain{},
		},
		"nullable true": {
			left:  &BoolDomain{Nullable: true},
			right: &BoolDomain{Nullable: true},
			want:  &BoolDomain{Nullable: true},
		},
		"enum nil": {
			left:  &BoolDomain{},
			right: &BoolDomain{},
			want:  &BoolDomain{},
		},
		"enum nil right": {
			left: &BoolDomain{},
			right: &BoolDomain{
				Enum: []types.Enum{types.Enum("true"), types.Enum("false")},
			},
			want: &BoolDomain{
				Enum: []types.Enum{types.Enum("false"), types.Enum("true")},
			},
		},
		"enum left nil": {
			left: &BoolDomain{
				Enum: []types.Enum{types.Enum("true"), types.Enum("false")},
			},
			right: &BoolDomain{},
			want: &BoolDomain{
				Enum: []types.Enum{types.Enum("false"), types.Enum("true")},
			},
		},
		"enum intersection": {
			left: &BoolDomain{
				Enum: []types.Enum{types.Enum("true"), types.Enum("false")},
			},
			right: &BoolDomain{Enum: []types.Enum{types.Enum("false")}},
			want:  &BoolDomain{Enum: []types.Enum{types.Enum("false")}},
		},
		"enum preserves left order": {
			left: &BoolDomain{
				Enum: []types.Enum{types.Enum("false"), types.Enum("true")},
			},
			right: &BoolDomain{
				Enum: []types.Enum{types.Enum("true"), types.Enum("false")},
			},
			want: &BoolDomain{
				Enum: []types.Enum{types.Enum("false"), types.Enum("true")},
			},
		},
		"all fields": {
			left: &BoolDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum("true"), types.Enum("false")},
			},
			right: &BoolDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum("false")},
			},
			want: &BoolDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum("false")},
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

// TestBoolDomainAllOfMergeInvalidPlanCases covers invalid boolean intersections.
func TestBoolDomainAllOfMergeInvalidPlanCases(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		left  *BoolDomain
		right types.Domain
	}{
		"nil other": {
			left: &BoolDomain{},
		},
		"string": {
			left:  &BoolDomain{},
			right: &StringDomain{},
		},
		"number": {
			left:  &BoolDomain{},
			right: &NumberDomain{Type: "number"},
		},
		"array": {
			left:  &BoolDomain{},
			right: &ArrayDomain{},
		},
		"object": {
			left:  &BoolDomain{},
			right: &ObjectDomain{},
		},
		"empty enum intersection": {
			left:  &BoolDomain{Enum: []types.Enum{types.Enum("true")}},
			right: &BoolDomain{Enum: []types.Enum{types.Enum("false")}},
		},
		"raw null mismatch": {
			left:  &BoolDomain{Enum: []types.Enum{types.Enum("true")}},
			right: &BoolDomain{Enum: []types.Enum{types.Enum("null")}},
		},
		"non-nullable null intersection": {
			left:  &BoolDomain{Enum: []types.Enum{types.Enum("true"), types.Enum("null")}},
			right: &BoolDomain{Enum: []types.Enum{types.Enum("null"), types.Enum("false")}},
		},
		"incompatible allOf": {
			left: &BoolDomain{},
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

		var left *BoolDomain

		got, err := left.AllOfMerge(&BoolDomain{})
		require.Error(t, err)
		require.Nil(t, got)
	})
}
