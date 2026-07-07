package domain

import (
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types"

	"github.com/stretchr/testify/require"
)

func TestBoolDomainAllOfMergeValidPlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *BoolDomain
		right types.Domain
		want  types.Domain
	}{
		"nullable false":            {left: &BoolDomain{}, right: &BoolDomain{}, want: &BoolDomain{}},
		"nullable false true":       {left: &BoolDomain{}, right: &BoolDomain{Nullable: true}, want: &BoolDomain{}},
		"nullable true false":       {left: &BoolDomain{Nullable: true}, right: &BoolDomain{}, want: &BoolDomain{}},
		"nullable true":             {left: &BoolDomain{Nullable: true}, right: &BoolDomain{Nullable: true}, want: &BoolDomain{Nullable: true}},
		"enum nil":                  {left: &BoolDomain{}, right: &BoolDomain{}, want: &BoolDomain{}},
		"enum nil right":            {left: &BoolDomain{}, right: &BoolDomain{Enum: []types.Enum{types.Enum("true"), types.Enum("false")}}, want: &BoolDomain{Enum: []types.Enum{types.Enum("true"), types.Enum("false")}}},
		"enum left nil":             {left: &BoolDomain{Enum: []types.Enum{types.Enum("true"), types.Enum("false")}}, right: &BoolDomain{}, want: &BoolDomain{Enum: []types.Enum{types.Enum("true"), types.Enum("false")}}},
		"enum intersection":         {left: &BoolDomain{Enum: []types.Enum{types.Enum("true"), types.Enum("false")}}, right: &BoolDomain{Enum: []types.Enum{types.Enum("false")}}, want: &BoolDomain{Enum: []types.Enum{types.Enum("false")}}},
		"enum preserves left order": {left: &BoolDomain{Enum: []types.Enum{types.Enum("false"), types.Enum("true")}}, right: &BoolDomain{Enum: []types.Enum{types.Enum("true"), types.Enum("false")}}, want: &BoolDomain{Enum: []types.Enum{types.Enum("false"), types.Enum("true")}}},
		"enum raw null":             {left: &BoolDomain{Enum: []types.Enum{types.Enum("true"), types.Enum("null")}}, right: &BoolDomain{Enum: []types.Enum{types.Enum("null"), types.Enum("false")}}, want: &BoolDomain{Enum: []types.Enum{types.Enum("null")}}},
		"all fields":                {left: &BoolDomain{Nullable: true, Enum: []types.Enum{types.Enum("true"), types.Enum("false")}}, right: &BoolDomain{Nullable: true, Enum: []types.Enum{types.Enum("false")}}, want: &BoolDomain{Nullable: true, Enum: []types.Enum{types.Enum("false")}}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.left.AllOfMerge(tt.right)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBoolDomainAllOfMergeInvalidPlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *BoolDomain
		right types.Domain
	}{
		"nil other":               {left: &BoolDomain{}, right: nil},
		"string":                  {left: &BoolDomain{}, right: &StringDomain{}},
		"number":                  {left: &BoolDomain{}, right: &NumberDomain{Type: "number"}},
		"array":                   {left: &BoolDomain{}, right: &ArrayDomain{}},
		"object":                  {left: &BoolDomain{}, right: &ObjectDomain{}},
		"empty enum intersection": {left: &BoolDomain{Enum: []types.Enum{types.Enum("true")}}, right: &BoolDomain{Enum: []types.Enum{types.Enum("false")}}},
		"raw null mismatch":       {left: &BoolDomain{Enum: []types.Enum{types.Enum("true")}}, right: &BoolDomain{Enum: []types.Enum{types.Enum("null")}}},
		"incompatible allOf":      {left: &BoolDomain{}, right: &AllOfDomain{Domains: []types.Domain{&StringDomain{}}, MergedDomain: &StringDomain{}}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			before := *tt.left
			got, err := tt.left.AllOfMerge(tt.right)
			require.Error(t, err)
			require.Nil(t, got)
			require.Equal(t, before, *tt.left)
		})
	}

	t.Run("nil receiver", func(t *testing.T) {
		var left *BoolDomain

		got, err := left.AllOfMerge(&BoolDomain{})
		require.Error(t, err)
		require.Nil(t, got)
	})
}
