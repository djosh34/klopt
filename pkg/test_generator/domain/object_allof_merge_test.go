package domain

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestObjectDomainAllOfMergeValidPlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *ObjectDomain
		right types.Domain
		want  types.Domain
	}{
		"nullable true true":                         {left: &ObjectDomain{Nullable: true}, right: &ObjectDomain{Nullable: true}, want: &ObjectDomain{Nullable: true}},
		"enum nil right":                             {left: &ObjectDomain{}, right: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1}`), types.Enum(`{"b":2}`)}}, want: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1}`), types.Enum(`{"b":2}`)}}},
		"enum intersection":                          {left: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1}`), types.Enum(`{"b":2}`), types.Enum(`{"c":3}`)}}, right: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"b":2}`), types.Enum(`{"c":3}`), types.Enum(`{"d":4}`)}}, want: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"b":2}`), types.Enum(`{"c":3}`)}}},
		"enum preserves order":                       {left: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"b":2}`), types.Enum(`{"a":1}`)}}, right: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1}`), types.Enum(`{"b":2}`)}}, want: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"b":2}`), types.Enum(`{"a":1}`)}}},
		"enum raw null":                              {left: &ObjectDomain{Enum: []types.Enum{types.Enum(`null`), types.Enum(`{"a":1}`)}}, right: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"b":2}`), types.Enum(`null`)}}, want: &ObjectDomain{Enum: []types.Enum{types.Enum(`null`)}}},
		"enum not filtered":                          {left: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"forbidden":true}`)}, AdditionalPropertyKind: AdditionalFalse}, right: &ObjectDomain{}, want: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"forbidden":true}`)}, AdditionalPropertyKind: AdditionalFalse}},
		"disjoint props sorted":                      {left: &ObjectDomain{Properties: []Property{{Key: "b"}}, AdditionalPropertyKind: AdditionalTrue}, right: &ObjectDomain{Properties: []Property{{Key: "a"}}, AdditionalPropertyKind: AdditionalTrue}, want: &ObjectDomain{Properties: []Property{{Key: "a"}, {Key: "b"}}, AdditionalPropertyKind: AdditionalTrue}},
		"same prop required or and domain merge":     {left: &ObjectDomain{Properties: []Property{{Key: "a", Required: true, Domain: &StringDomain{MinLength: 1}}}}, right: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MaxLength: new(5)}}}}, want: &ObjectDomain{Properties: []Property{{Key: "a", Required: true, Domain: &StringDomain{MinLength: 1, MaxLength: new(5)}}}}},
		"same prop nil concrete":                     {left: &ObjectDomain{Properties: []Property{{Key: "a"}}}, right: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MinLength: 1}}}}, want: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MinLength: 1}}}}},
		"optional prop dropped by additional false":  {left: &ObjectDomain{Properties: []Property{{Key: "a"}}}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}, want: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}},
		"disjoint optional props both false dropped": {left: &ObjectDomain{Properties: []Property{{Key: "a"}}, AdditionalPropertyKind: AdditionalFalse}, right: &ObjectDomain{Properties: []Property{{Key: "b"}}, AdditionalPropertyKind: AdditionalFalse}, want: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}},
		"prop merged with additional schema":         {left: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MinLength: 1}}}}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}, want: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MinLength: 1, MaxLength: new(5)}}}, AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}},
		"additional true false":                      {left: &ObjectDomain{AdditionalPropertyKind: AdditionalTrue}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}, want: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}},
		"additional true schema":                     {left: &ObjectDomain{AdditionalPropertyKind: AdditionalTrue}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}, want: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}},
		"additional schema schema":                   {left: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MinLength: 1}}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}, want: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MinLength: 1, MaxLength: new(5)}}},
		"min max no satisfiability check":            {left: &ObjectDomain{MinProps: 10}, right: &ObjectDomain{MaxProps: new(5)}, want: &ObjectDomain{MinProps: 10, MaxProps: new(5)}},
		"required count not checked":                 {left: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}, {Key: "b", Required: true}}}, right: &ObjectDomain{MaxProps: new(1)}, want: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}, {Key: "b", Required: true}}, MaxProps: new(1)}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.left.AllOfMerge(tt.right)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestObjectDomainAllOfMergeInvalidPlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *ObjectDomain
		right types.Domain
	}{
		"nil other": {left: &ObjectDomain{}}, "string": {left: &ObjectDomain{}, right: &StringDomain{}}, "number": {left: &ObjectDomain{}, right: &NumberDomain{Type: "number"}}, "bool": {left: &ObjectDomain{}, right: &BoolDomain{}}, "array": {left: &ObjectDomain{}, right: &ArrayDomain{}},
		"enum empty":                          {left: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1}`)}}, right: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":2}`)}}},
		"enum raw order differs":              {left: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1,"b":2}`)}}, right: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"b":2,"a":1}`)}}},
		"enum nil raw message":                {left: &ObjectDomain{Enum: []types.Enum{nil}}, right: &ObjectDomain{Enum: []types.Enum{types.Enum(`{}`)}}},
		"same prop incompatible":              {left: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{}}}}, right: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &BoolDomain{}}}}},
		"required forbidden":                  {left: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}},
		"prop additional schema incompatible": {left: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{}}}}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &BoolDomain{}}},
		"additional schema nil left":          {left: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalTrue}},
		"additional schema nil right":         {left: &ObjectDomain{AdditionalPropertyKind: AdditionalTrue}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema}},
		"additional schema incompatible":      {left: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{}}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &BoolDomain{}}},
		"incompatible allOf":                  {left: &ObjectDomain{}, right: &AllOfDomain{Domains: []types.Domain{&StringDomain{}}, MergedDomain: &StringDomain{}}},
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
		var left *ObjectDomain
		got, err := left.AllOfMerge(&ObjectDomain{})
		require.Error(t, err)
		require.Nil(t, got)
	})
}
