package domain

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArrayDomainAllOfMergeValidPlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *ArrayDomain
		right types.Domain
		want  types.Domain
	}{
		"nullable true true":              {left: &ArrayDomain{Nullable: true}, right: &ArrayDomain{Nullable: true}, want: &ArrayDomain{Nullable: true}},
		"enum nil right":                  {left: &ArrayDomain{}, right: &ArrayDomain{Enum: []types.Enum{types.Enum(`["a"]`), types.Enum(`["b"]`)}}, want: &ArrayDomain{Enum: []types.Enum{types.Enum(`["a"]`), types.Enum(`["b"]`)}}},
		"enum intersection":               {left: &ArrayDomain{Enum: []types.Enum{types.Enum(`["a"]`), types.Enum(`["b"]`), types.Enum(`["c"]`)}}, right: &ArrayDomain{Enum: []types.Enum{types.Enum(`["b"]`), types.Enum(`["c"]`), types.Enum(`["d"]`)}}, want: &ArrayDomain{Enum: []types.Enum{types.Enum(`["b"]`), types.Enum(`["c"]`)}}},
		"enum preserves order":            {left: &ArrayDomain{Enum: []types.Enum{types.Enum(`["b"]`), types.Enum(`["a"]`)}}, right: &ArrayDomain{Enum: []types.Enum{types.Enum(`["a"]`), types.Enum(`["b"]`)}}, want: &ArrayDomain{Enum: []types.Enum{types.Enum(`["b"]`), types.Enum(`["a"]`)}}},
		"enum raw null":                   {left: &ArrayDomain{Enum: []types.Enum{types.Enum(`null`), types.Enum(`["a"]`)}}, right: &ArrayDomain{Enum: []types.Enum{types.Enum(`["b"]`), types.Enum(`null`)}}, want: &ArrayDomain{Enum: []types.Enum{types.Enum(`null`)}}},
		"enum not filtered":               {left: &ArrayDomain{Enum: []types.Enum{types.Enum(`["too-long"]`)}, MinItems: 99}, right: &ArrayDomain{MaxItems: new(0)}, want: &ArrayDomain{Enum: []types.Enum{types.Enum(`["too-long"]`)}, MinItems: 99, MaxItems: new(0)}},
		"items nil nil":                   {left: &ArrayDomain{}, right: &ArrayDomain{}, want: &ArrayDomain{}},
		"items nil domain":                {left: &ArrayDomain{}, right: &ArrayDomain{Items: &StringDomain{MinLength: 1}}, want: &ArrayDomain{Items: &StringDomain{MinLength: 1}}},
		"items domain nil":                {left: &ArrayDomain{Items: &StringDomain{MinLength: 1}}, right: &ArrayDomain{}, want: &ArrayDomain{Items: &StringDomain{MinLength: 1}}},
		"items string merge":              {left: &ArrayDomain{Items: &StringDomain{MinLength: 1}}, right: &ArrayDomain{Items: &StringDomain{MaxLength: new(5)}}, want: &ArrayDomain{Items: &StringDomain{MinLength: 1, MaxLength: new(5)}}},
		"items number merge":              {left: &ArrayDomain{Items: &NumberDomain{Type: "number"}}, right: &ArrayDomain{Items: &NumberDomain{Type: "integer"}}, want: &ArrayDomain{Items: &NumberDomain{Type: "integer"}}},
		"min max no satisfiability check": {left: &ArrayDomain{MinItems: 10}, right: &ArrayDomain{MaxItems: new(5)}, want: &ArrayDomain{MinItems: 10, MaxItems: new(5)}},
		"all fields":                      {left: &ArrayDomain{Nullable: true, Enum: []types.Enum{types.Enum(`["a"]`), types.Enum(`["b"]`)}, Items: &StringDomain{MinLength: 1}, MinItems: 1, MaxItems: new(10)}, right: &ArrayDomain{Nullable: true, Enum: []types.Enum{types.Enum(`["b"]`), types.Enum(`["c"]`)}, Items: &StringDomain{MaxLength: new(5)}, MinItems: 2, MaxItems: new(8)}, want: &ArrayDomain{Nullable: true, Enum: []types.Enum{types.Enum(`["b"]`)}, Items: &StringDomain{MinLength: 1, MaxLength: new(5)}, MinItems: 2, MaxItems: new(8)}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.left.AllOfMerge(tt.right)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestArrayDomainAllOfMergeInvalidPlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *ArrayDomain
		right types.Domain
	}{
		"nil other": {left: &ArrayDomain{}}, "string": {left: &ArrayDomain{}, right: &StringDomain{}}, "number": {left: &ArrayDomain{}, right: &NumberDomain{Type: "number"}}, "bool": {left: &ArrayDomain{}, right: &BoolDomain{}}, "object": {left: &ArrayDomain{}, right: &ObjectDomain{}},
		"enum numeric raw mismatch":     {left: &ArrayDomain{Enum: []types.Enum{types.Enum(`[1]`)}}, right: &ArrayDomain{Enum: []types.Enum{types.Enum(`[1.0]`)}}},
		"enum empty":                    {left: &ArrayDomain{Enum: []types.Enum{types.Enum(`["a"]`)}}, right: &ArrayDomain{Enum: []types.Enum{types.Enum(`["b"]`)}}},
		"enum object raw order differs": {left: &ArrayDomain{Enum: []types.Enum{types.Enum(`{"a":1,"b":2}`)}}, right: &ArrayDomain{Enum: []types.Enum{types.Enum(`{"b":2,"a":1}`)}}},
		"enum nil raw message":          {left: &ArrayDomain{Enum: []types.Enum{nil}}, right: &ArrayDomain{Enum: []types.Enum{types.Enum(`[]`)}}},
		"items incompatible":            {left: &ArrayDomain{Items: &StringDomain{}}, right: &ArrayDomain{Items: &BoolDomain{}}},
		"incompatible allOf":            {left: &ArrayDomain{}, right: &AllOfDomain{Domains: []types.Domain{&StringDomain{}}, MergedDomain: &StringDomain{}}},
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
		var left *ArrayDomain
		got, err := left.AllOfMerge(&ArrayDomain{})
		require.Error(t, err)
		require.Nil(t, got)
	})
}
