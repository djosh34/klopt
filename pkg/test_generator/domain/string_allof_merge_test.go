package domain

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringDomainAllOfMergeValidPlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *StringDomain
		right types.Domain
		want  types.Domain
	}{
		"nullable false false":                      {left: &StringDomain{}, right: &StringDomain{}, want: &StringDomain{}},
		"nullable false true":                       {left: &StringDomain{}, right: &StringDomain{Nullable: true}, want: &StringDomain{}},
		"nullable true false":                       {left: &StringDomain{Nullable: true}, right: &StringDomain{}, want: &StringDomain{}},
		"nullable true true":                        {left: &StringDomain{Nullable: true}, right: &StringDomain{Nullable: true}, want: &StringDomain{Nullable: true}},
		"enum nil right":                            {left: &StringDomain{}, right: &StringDomain{Enum: []types.Enum{types.Enum("\"a\""), types.Enum("\"b\"")}}, want: &StringDomain{Enum: []types.Enum{types.Enum("\"a\""), types.Enum("\"b\"")}}},
		"enum left nil":                             {left: &StringDomain{Enum: []types.Enum{types.Enum("\"a\""), types.Enum("\"b\"")}}, right: &StringDomain{}, want: &StringDomain{Enum: []types.Enum{types.Enum("\"a\""), types.Enum("\"b\"")}}},
		"enum intersection":                         {left: &StringDomain{Enum: []types.Enum{types.Enum("\"a\""), types.Enum("\"b\""), types.Enum("\"c\"")}}, right: &StringDomain{Enum: []types.Enum{types.Enum("\"b\""), types.Enum("\"c\""), types.Enum("\"d\"")}}, want: &StringDomain{Enum: []types.Enum{types.Enum("\"b\""), types.Enum("\"c\"")}}},
		"enum preserves left order":                 {left: &StringDomain{Enum: []types.Enum{types.Enum("\"b\""), types.Enum("\"a\"")}}, right: &StringDomain{Enum: []types.Enum{types.Enum("\"a\""), types.Enum("\"b\"")}}, want: &StringDomain{Enum: []types.Enum{types.Enum("\"b\""), types.Enum("\"a\"")}}},
		"enum raw null":                             {left: &StringDomain{Enum: []types.Enum{types.Enum("\"a\""), types.Enum("null")}}, right: &StringDomain{Enum: []types.Enum{types.Enum("null"), types.Enum("\"b\"")}}, want: &StringDomain{Enum: []types.Enum{types.Enum("null")}}},
		"pattern concat duplicates":                 {left: &StringDomain{Pattern: types.Pattern{"p"}}, right: &StringDomain{Pattern: types.Pattern{"p"}}, want: &StringDomain{Pattern: types.Pattern{"p", "p"}}},
		"format concat duplicates":                  {left: &StringDomain{Format: types.Format{"email"}}, right: &StringDomain{Format: types.Format{"email"}}, want: &StringDomain{Format: types.Format{"email", "email"}}},
		"valid examples nil right":                  {left: &StringDomain{}, right: &StringDomain{XValidExamples: []string{"a", "b"}}, want: &StringDomain{XValidExamples: []string{"a", "b"}}},
		"valid examples intersection":               {left: &StringDomain{XValidExamples: []string{"a", "b", "c"}}, right: &StringDomain{XValidExamples: []string{"b", "c", "d"}}, want: &StringDomain{XValidExamples: []string{"b", "c"}}},
		"valid examples empty intersection allowed": {left: &StringDomain{XValidExamples: []string{"a"}}, right: &StringDomain{XValidExamples: []string{"b"}}, want: &StringDomain{XValidExamples: []string{}}},
		"enum valid examples intersect":             {left: &StringDomain{Enum: []types.Enum{types.Enum("\"a\""), types.Enum("\"b\"")}}, right: &StringDomain{XValidExamples: []string{"b", "c"}}, want: &StringDomain{Enum: []types.Enum{types.Enum("\"b\"")}, XValidExamples: []string{"b"}}},
		"invalid examples union":                    {left: &StringDomain{XInvalidExamples: []string{"a", "b"}}, right: &StringDomain{XInvalidExamples: []string{"b", "c"}}, want: &StringDomain{XInvalidExamples: []string{"a", "b", "c"}}},
		"min max no satisfiability check":           {left: &StringDomain{MinLength: 10}, right: &StringDomain{MaxLength: new(5)}, want: &StringDomain{MinLength: 10, MaxLength: new(5)}},
		"all fields":                                {left: &StringDomain{Pattern: types.Pattern{"p1"}, Format: types.Format{"f1"}, XValidExamples: []string{"a", "b"}, XInvalidExamples: []string{"x"}, MinLength: 1, MaxLength: new(10)}, right: &StringDomain{Pattern: types.Pattern{"p2"}, Format: types.Format{"f2"}, XValidExamples: []string{"b", "c"}, XInvalidExamples: []string{"y"}, MinLength: 2, MaxLength: new(8)}, want: &StringDomain{Pattern: types.Pattern{"p1", "p2"}, Format: types.Format{"f1", "f2"}, XValidExamples: []string{"b"}, XInvalidExamples: []string{"x", "y"}, MinLength: 2, MaxLength: new(8)}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.left.AllOfMerge(tt.right)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestStringDomainAllOfMergeInvalidPlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *StringDomain
		right types.Domain
	}{
		"nil other": {left: &StringDomain{}}, "bool": {left: &StringDomain{}, right: &BoolDomain{}}, "number": {left: &StringDomain{}, right: &NumberDomain{Type: "number"}}, "array": {left: &StringDomain{}, right: &ArrayDomain{}}, "object": {left: &StringDomain{}, right: &ObjectDomain{}},
		"enum empty":                {left: &StringDomain{Enum: []types.Enum{types.Enum("\"a\"")}}, right: &StringDomain{Enum: []types.Enum{types.Enum("\"b\"")}}},
		"enum case sensitive":       {left: &StringDomain{Enum: []types.Enum{types.Enum("\"a\"")}}, right: &StringDomain{Enum: []types.Enum{types.Enum("\"A\"")}}},
		"enum raw null mismatch":    {left: &StringDomain{Enum: []types.Enum{types.Enum("\"a\"")}}, right: &StringDomain{Enum: []types.Enum{types.Enum("null")}}},
		"enum valid examples empty": {left: &StringDomain{Enum: []types.Enum{types.Enum("\"a\"")}}, right: &StringDomain{XValidExamples: []string{"b"}}},
		"valid examples enum empty": {left: &StringDomain{XValidExamples: []string{"a"}}, right: &StringDomain{Enum: []types.Enum{types.Enum("\"b\"")}}},
		"incompatible allOf":        {left: &StringDomain{}, right: &AllOfDomain{Domains: []types.Domain{&BoolDomain{}}, MergedDomain: &BoolDomain{}}},
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
		var left *StringDomain
		got, err := left.AllOfMerge(&StringDomain{})
		require.Error(t, err)
		require.Nil(t, got)
	})
}
