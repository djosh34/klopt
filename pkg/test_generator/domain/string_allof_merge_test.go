package domain

import (
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.

	"github.com/stretchr/testify/require"
)

// TestStringDomainAllOfMergeValidPlanCases covers string domain intersections.
func TestStringDomainAllOfMergeValidPlanCases(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		left  *StringDomain
		right types.Domain
		want  types.Domain
	}{
		"nullable false": {
			left:  &StringDomain{},
			right: &StringDomain{},
			want:  &StringDomain{},
		},
		"nullable false true": {
			left:  &StringDomain{},
			right: &StringDomain{Nullable: true},
			want:  &StringDomain{},
		},
		"nullable true false": {
			left:  &StringDomain{Nullable: true},
			right: &StringDomain{},
			want:  &StringDomain{},
		},
		"nullable true": {
			left:  &StringDomain{Nullable: true},
			right: &StringDomain{Nullable: true},
			want:  &StringDomain{Nullable: true},
		},
		"nullable different types intersect as null": {
			left:  &StringDomain{Nullable: true},
			right: &ArrayDomain{Nullable: true},
			want: &BoolDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum(`null`)},
			},
		},
		"enum nil right": {
			left: &StringDomain{},
			right: &StringDomain{
				Enum: []types.Enum{types.Enum(`"a"`), types.Enum(`"b"`)},
			},
			want: &StringDomain{
				Enum: []types.Enum{types.Enum(`"a"`), types.Enum(`"b"`)},
			},
		},
		"enum intersection": {
			left: &StringDomain{
				Enum: []types.Enum{types.Enum(`"a"`), types.Enum(`"b"`), types.Enum(`"c"`)},
			},
			right: &StringDomain{
				Enum: []types.Enum{types.Enum(`"b"`), types.Enum(`"c"`), types.Enum(`"d"`)},
			},
			want: &StringDomain{
				Enum: []types.Enum{types.Enum(`"b"`), types.Enum(`"c"`)},
			},
		},
		"enum uses canonical order": {
			left: &StringDomain{
				Enum: []types.Enum{types.Enum(`"b"`), types.Enum(`"a"`)},
			},
			right: &StringDomain{
				Enum: []types.Enum{types.Enum(`"a"`), types.Enum(`"b"`)},
			},
			want: &StringDomain{
				Enum: []types.Enum{types.Enum(`"a"`), types.Enum(`"b"`)},
			},
		},
		"enum escaped strings compare semantically": {
			left:  &StringDomain{Enum: []types.Enum{types.Enum(`"a"`)}},
			right: &StringDomain{Enum: []types.Enum{types.Enum(`"\u0061"`)}},
			want:  &StringDomain{Enum: []types.Enum{types.Enum(`"a"`)}},
		},
		"pattern concat duplicates": {
			left:  &StringDomain{Pattern: types.Pattern{"p"}},
			right: &StringDomain{Pattern: types.Pattern{"p"}},
			want:  &StringDomain{Pattern: types.Pattern{"p", "p"}},
		},
		"format concat duplicates": {
			left:  &StringDomain{Format: types.Format{"email"}},
			right: &StringDomain{Format: types.Format{"email"}},
			want:  &StringDomain{Format: types.Format{"email", "email"}},
		},
		"valid examples nil right": {
			left:  &StringDomain{},
			right: &StringDomain{XValidExamples: []string{"a", "b"}},
			want:  &StringDomain{XValidExamples: []string{"a", "b"}},
		},
		"valid examples intersection": {
			left:  &StringDomain{XValidExamples: []string{"a", "b", "c"}},
			right: &StringDomain{XValidExamples: []string{"b", "c", "d"}},
			want:  &StringDomain{XValidExamples: []string{"b", "c"}},
		},
		"valid examples empty intersection allowed": {
			left:  &StringDomain{XValidExamples: []string{"a"}},
			right: &StringDomain{XValidExamples: []string{"b"}},
			want:  &StringDomain{XValidExamples: []string{}},
		},
		"enum valid examples intersect": {
			left: &StringDomain{
				Enum: []types.Enum{types.Enum(`"a"`), types.Enum(`"b"`)},
			},
			right: &StringDomain{XValidExamples: []string{"b", "c"}},
			want: &StringDomain{
				Enum:           []types.Enum{types.Enum(`"b"`)},
				XValidExamples: []string{"b"},
			},
		},
		"enum is filtered by merged length and valid examples": {
			left: &StringDomain{
				Enum:           []types.Enum{types.Enum(`"a"`), types.Enum(`"bb"`), types.Enum(`"ccc"`)},
				XValidExamples: []string{"a", "bb", "ccc"},
				MinLength:      2,
			},
			right: &StringDomain{
				XValidExamples: []string{"bb", "ccc"},
				MaxLength:      new(2),
			},
			want: &StringDomain{
				Enum:           []types.Enum{types.Enum(`"bb"`)},
				XValidExamples: []string{"bb"},
				MinLength:      2,
				MaxLength:      new(2),
			},
		},
		"invalid examples union": {
			left:  &StringDomain{XInvalidExamples: []string{"a", "b"}},
			right: &StringDomain{XInvalidExamples: []string{"b", "c"}},
			want:  &StringDomain{XInvalidExamples: []string{"a", "b", "c"}},
		},
		"nullable permits contradictory length bounds": {
			left:  &StringDomain{Nullable: true, MinLength: 10},
			right: &StringDomain{Nullable: true, MaxLength: new(5)},
			want:  &StringDomain{Nullable: true, MinLength: 10, MaxLength: new(5)},
		},
		"nullable enum null permits contradictory length bounds": {
			left: &StringDomain{
				Nullable:  true,
				Enum:      []types.Enum{types.Enum(`"a"`), types.Enum(`null`)},
				MinLength: 2,
			},
			right: &StringDomain{
				Nullable:  true,
				Enum:      []types.Enum{types.Enum(`null`), types.Enum(`"a"`)},
				MaxLength: new(1),
			},
			want: &StringDomain{
				Nullable:  true,
				Enum:      []types.Enum{types.Enum(`null`)},
				MinLength: 2,
				MaxLength: new(1),
			},
		},
		"all fields": {
			left: &StringDomain{
				Pattern:          types.Pattern{"p1"},
				Format:           types.Format{"f1"},
				XValidExamples:   []string{"a", "b"},
				XInvalidExamples: []string{"x"},
				MinLength:        1,
				MaxLength:        new(10),
			},
			right: &StringDomain{
				Pattern:          types.Pattern{"p2"},
				Format:           types.Format{"f2"},
				XValidExamples:   []string{"b", "c"},
				XInvalidExamples: []string{"y"},
				MinLength:        2,
				MaxLength:        new(8),
			},
			want: &StringDomain{
				Pattern:          types.Pattern{"p1", "p2"},
				Format:           types.Format{"f1", "f2"},
				XValidExamples:   []string{"b"},
				XInvalidExamples: []string{"x", "y"},
				MinLength:        2,
				MaxLength:        new(8),
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

// TestStringDomainAllOfMergeInvalidPlanCases covers invalid string intersections.
func TestStringDomainAllOfMergeInvalidPlanCases(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		left  *StringDomain
		right types.Domain
	}{
		"nil other": {
			left: &StringDomain{},
		},
		"bool": {
			left:  &StringDomain{},
			right: &BoolDomain{},
		},
		"number": {
			left:  &StringDomain{},
			right: &NumberDomain{Type: "number"},
		},
		"array": {
			left:  &StringDomain{},
			right: &ArrayDomain{},
		},
		"object": {
			left:  &StringDomain{},
			right: &ObjectDomain{},
		},
		"enum empty": {
			left:  &StringDomain{Enum: []types.Enum{types.Enum(`"a"`)}},
			right: &StringDomain{Enum: []types.Enum{types.Enum(`"b"`)}},
		},
		"enum case sensitive": {
			left:  &StringDomain{Enum: []types.Enum{types.Enum(`"a"`)}},
			right: &StringDomain{Enum: []types.Enum{types.Enum(`"A"`)}},
		},
		"enum raw null mismatch": {
			left:  &StringDomain{Enum: []types.Enum{types.Enum(`"a"`)}},
			right: &StringDomain{Enum: []types.Enum{types.Enum("null")}},
		},
		"non-nullable null intersection": {
			left:  &StringDomain{Enum: []types.Enum{types.Enum(`"a"`), types.Enum("null")}},
			right: &StringDomain{Enum: []types.Enum{types.Enum("null"), types.Enum(`"b"`)}},
		},
		"enum valid examples empty": {
			left:  &StringDomain{Enum: []types.Enum{types.Enum(`"a"`)}},
			right: &StringDomain{XValidExamples: []string{"b"}},
		},
		"valid examples enum empty": {
			left:  &StringDomain{XValidExamples: []string{"a"}},
			right: &StringDomain{Enum: []types.Enum{types.Enum(`"b"`)}},
		},
		"contradictory length bounds": {
			left:  &StringDomain{MinLength: 10},
			right: &StringDomain{MaxLength: new(5)},
		},
		"enum has no values within merged length bounds": {
			left:  &StringDomain{Enum: []types.Enum{types.Enum(`"a"`)}, MinLength: 2},
			right: &StringDomain{},
		},
		"nullable enum without null cannot rescue contradictory bounds": {
			left: &StringDomain{
				Nullable:  true,
				Enum:      []types.Enum{types.Enum(`"a"`)},
				MinLength: 2,
			},
			right: &StringDomain{Nullable: true, MaxLength: new(1)},
		},
		"incompatible allOf": {
			left: &StringDomain{},
			right: &AllOfDomain{
				Domains:      []types.Domain{&BoolDomain{}},
				MergedDomain: &BoolDomain{},
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

		var left *StringDomain

		got, err := left.AllOfMerge(&StringDomain{})
		require.Error(t, err)
		require.Nil(t, got)
	})
}
