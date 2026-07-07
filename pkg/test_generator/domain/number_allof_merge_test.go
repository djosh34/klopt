package domain

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNumberDomainAllOfMergeValidPlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *NumberDomain
		right types.Domain
		want  types.Domain
	}{
		"number number":                   {left: &NumberDomain{Type: "number"}, right: &NumberDomain{Type: "number"}, want: &NumberDomain{Type: "number"}},
		"integer integer":                 {left: &NumberDomain{Type: "integer"}, right: &NumberDomain{Type: "integer"}, want: &NumberDomain{Type: "integer"}},
		"number integer":                  {left: &NumberDomain{Type: "number"}, right: &NumberDomain{Type: "integer"}, want: &NumberDomain{Type: "integer"}},
		"integer number":                  {left: &NumberDomain{Type: "integer"}, right: &NumberDomain{Type: "number"}, want: &NumberDomain{Type: "integer"}},
		"nullable true true":              {left: &NumberDomain{Type: "number", Nullable: true}, right: &NumberDomain{Type: "number", Nullable: true}, want: &NumberDomain{Type: "number", Nullable: true}},
		"enum nil right":                  {left: &NumberDomain{Type: "number"}, right: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1"), types.Enum("2")}}, want: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1"), types.Enum("2")}}},
		"enum exact lexeme intersection":  {left: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1"), types.Enum("2"), types.Enum("1.0")}}, right: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("2"), types.Enum("1.0"), types.Enum("3")}}, want: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("2"), types.Enum("1.0")}}},
		"enum raw null":                   {left: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1"), types.Enum("null")}}, right: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("null"), types.Enum("2")}}, want: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("null")}}},
		"minimum larger":                  {left: &NumberDomain{Type: "number", Minimum: new(Number("1"))}, right: &NumberDomain{Type: "number", Minimum: new(Number("2"))}, want: &NumberDomain{Type: "number", Minimum: new(Number("2"))}},
		"minimum equal exclusive":         {left: &NumberDomain{Type: "number", Minimum: new(Number("1"))}, right: &NumberDomain{Type: "number", Minimum: new(Number("1")), ExclusiveMinimum: true}, want: &NumberDomain{Type: "number", Minimum: new(Number("1")), ExclusiveMinimum: true}},
		"minimum equal keeps left lexeme": {left: &NumberDomain{Type: "number", Minimum: new(Number("1.0"))}, right: &NumberDomain{Type: "number", Minimum: new(Number("1")), ExclusiveMinimum: true}, want: &NumberDomain{Type: "number", Minimum: new(Number("1.0")), ExclusiveMinimum: true}},
		"maximum smaller":                 {left: &NumberDomain{Type: "number", Maximum: new(Number("10"))}, right: &NumberDomain{Type: "number", Maximum: new(Number("8"))}, want: &NumberDomain{Type: "number", Maximum: new(Number("8"))}},
		"maximum equal exclusive":         {left: &NumberDomain{Type: "number", Maximum: new(Number("1"))}, right: &NumberDomain{Type: "number", Maximum: new(Number("1")), ExclusiveMaximum: true}, want: &NumberDomain{Type: "number", Maximum: new(Number("1")), ExclusiveMaximum: true}},
		"range no satisfiability check":   {left: &NumberDomain{Type: "number", Minimum: new(Number("10"))}, right: &NumberDomain{Type: "number", Maximum: new(Number("5"))}, want: &NumberDomain{Type: "number", Minimum: new(Number("10")), Maximum: new(Number("5"))}},
		"multiple nil right":              {left: &NumberDomain{Type: "number"}, right: &NumberDomain{Type: "number", MultipleOf: new(Number("2"))}, want: &NumberDomain{Type: "number", MultipleOf: new(Number("2"))}},
		"multiple lcm integers":           {left: &NumberDomain{Type: "number", MultipleOf: new(Number("2"))}, right: &NumberDomain{Type: "number", MultipleOf: new(Number("3"))}, want: &NumberDomain{Type: "number", MultipleOf: new(Number("6"))}},
		"multiple rationals":              {left: &NumberDomain{Type: "number", MultipleOf: new(Number("1.5"))}, right: &NumberDomain{Type: "number", MultipleOf: new(Number("2.5"))}, want: &NumberDomain{Type: "number", MultipleOf: new(Number("7.5"))}},
		"format nil right":                {left: &NumberDomain{Type: "number"}, right: &NumberDomain{Type: "number", Format: new("float")}, want: &NumberDomain{Type: "number", Format: new("float")}},
		"integer format":                  {left: &NumberDomain{Type: "integer", Format: new("int32")}, right: &NumberDomain{Type: "integer", Format: new("int32")}, want: &NumberDomain{Type: "integer", Format: new("int32")}},
		"all fields":                      {left: &NumberDomain{Type: "number", Nullable: true, Enum: []types.Enum{types.Enum("1"), types.Enum("2")}, Minimum: new(Number("0")), Maximum: new(Number("10")), MultipleOf: new(Number("2"))}, right: &NumberDomain{Type: "integer", Nullable: true, Enum: []types.Enum{types.Enum("2"), types.Enum("3")}, Minimum: new(Number("1")), Maximum: new(Number("8")), MultipleOf: new(Number("4")), Format: new("int64")}, want: &NumberDomain{Type: "integer", Nullable: true, Enum: []types.Enum{types.Enum("2")}, Minimum: new(Number("1")), Maximum: new(Number("8")), MultipleOf: new(Number("4")), Format: new("int64")}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.left.AllOfMerge(tt.right)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNumberDomainAllOfMergeInvalidPlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *NumberDomain
		right types.Domain
	}{
		"nil other": {left: &NumberDomain{Type: "number"}}, "string": {left: &NumberDomain{Type: "number"}, right: &StringDomain{}}, "bool": {left: &NumberDomain{Type: "number"}, right: &BoolDomain{}}, "array": {left: &NumberDomain{Type: "number"}, right: &ArrayDomain{}}, "object": {left: &NumberDomain{Type: "number"}, right: &ObjectDomain{}},
		"left empty type": {left: &NumberDomain{}, right: &NumberDomain{Type: "number"}}, "right empty type": {left: &NumberDomain{Type: "number"}, right: &NumberDomain{}}, "left bad type": {left: &NumberDomain{Type: "string"}, right: &NumberDomain{Type: "number"}},
		"enum exact mismatch":                {left: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1")}}, right: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1.0")}}},
		"bad minimum":                        {left: &NumberDomain{Type: "number", Minimum: new(Number("bad"))}, right: &NumberDomain{Type: "number", Minimum: new(Number("1"))}},
		"bad maximum":                        {left: &NumberDomain{Type: "number", Maximum: new(Number("bad"))}, right: &NumberDomain{Type: "number", Maximum: new(Number("1"))}},
		"bad multiple":                       {left: &NumberDomain{Type: "number", MultipleOf: new(Number("bad"))}, right: &NumberDomain{Type: "number", MultipleOf: new(Number("2"))}},
		"format conflict":                    {left: &NumberDomain{Type: "number", Format: new("float")}, right: &NumberDomain{Type: "number", Format: new("double")}},
		"integer format conflict":            {left: &NumberDomain{Type: "integer", Format: new("int32")}, right: &NumberDomain{Type: "integer", Format: new("int64")}},
		"integer with float unrepresentable": {left: &NumberDomain{Type: "number", Format: new("float")}, right: &NumberDomain{Type: "integer"}},
		"incompatible allOf":                 {left: &NumberDomain{Type: "number"}, right: &AllOfDomain{Domains: []types.Domain{&StringDomain{}}, MergedDomain: &StringDomain{}}},
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
		var left *NumberDomain
		got, err := left.AllOfMerge(&NumberDomain{Type: "number"})
		require.Error(t, err)
		require.Nil(t, got)
	})
}
