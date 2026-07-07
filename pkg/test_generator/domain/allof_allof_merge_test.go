package domain

import (
	"encoding/json"
	"errors"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types"

	"github.com/stretchr/testify/require"
)

func TestAllOfDomainAllOfMergeValidPlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *AllOfDomain
		right types.Domain
		want  *AllOfDomain
	}{
		"empty plus string":   {left: &AllOfDomain{}, right: &StringDomain{MinLength: 1}, want: &AllOfDomain{Domains: []types.Domain{&StringDomain{MinLength: 1}}, MergedDomain: &StringDomain{MinLength: 1}}},
		"string max":          {left: &AllOfDomain{Domains: []types.Domain{&StringDomain{MinLength: 1}}, MergedDomain: &StringDomain{MinLength: 1}}, right: &StringDomain{MaxLength: new(5)}, want: &AllOfDomain{Domains: []types.Domain{&StringDomain{MinLength: 1}, &StringDomain{MaxLength: new(5)}}, MergedDomain: &StringDomain{MinLength: 1, MaxLength: new(5)}}},
		"string enum":         {left: &AllOfDomain{MergedDomain: &StringDomain{Enum: []types.Enum{types.Enum(`"a"`), types.Enum(`"b"`)}}}, right: &StringDomain{Enum: []types.Enum{types.Enum(`"b"`), types.Enum(`"c"`)}}, want: &AllOfDomain{Domains: []types.Domain{&StringDomain{Enum: []types.Enum{types.Enum(`"b"`), types.Enum(`"c"`)}}}, MergedDomain: &StringDomain{Enum: []types.Enum{types.Enum(`"b"`)}}}},
		"number integer":      {left: &AllOfDomain{MergedDomain: &NumberDomain{Type: "number"}}, right: &NumberDomain{Type: "integer"}, want: &AllOfDomain{Domains: []types.Domain{&NumberDomain{Type: "integer"}}, MergedDomain: &NumberDomain{Type: "integer"}}},
		"array":               {left: &AllOfDomain{MergedDomain: &ArrayDomain{MinItems: 1}}, right: &ArrayDomain{MaxItems: new(3)}, want: &AllOfDomain{Domains: []types.Domain{&ArrayDomain{MaxItems: new(3)}}, MergedDomain: &ArrayDomain{MinItems: 1, MaxItems: new(3)}}},
		"bool":                {left: &AllOfDomain{MergedDomain: &BoolDomain{Enum: []types.Enum{types.Enum("true"), types.Enum("false")}}}, right: &BoolDomain{Enum: []types.Enum{types.Enum("false")}}, want: &AllOfDomain{Domains: []types.Domain{&BoolDomain{Enum: []types.Enum{types.Enum("false")}}}, MergedDomain: &BoolDomain{Enum: []types.Enum{types.Enum("false")}}}},
		"nullable true":       {left: &AllOfDomain{MergedDomain: &StringDomain{Nullable: true}}, right: &StringDomain{Nullable: true}, want: &AllOfDomain{Domains: []types.Domain{&StringDomain{Nullable: true}}, MergedDomain: &StringDomain{Nullable: true}}},
		"nullable true false": {left: &AllOfDomain{MergedDomain: &StringDomain{Nullable: true}}, right: &StringDomain{}, want: &AllOfDomain{Domains: []types.Domain{&StringDomain{}}, MergedDomain: &StringDomain{}}},
		"allOf plus allOf":    {left: &AllOfDomain{Domains: []types.Domain{&StringDomain{MinLength: 1}}, MergedDomain: &StringDomain{MinLength: 1}}, right: &AllOfDomain{Domains: []types.Domain{&StringDomain{MaxLength: new(5)}, &StringDomain{Enum: []types.Enum{types.Enum(`"abc"`)}}}, MergedDomain: &StringDomain{MaxLength: new(5), Enum: []types.Enum{types.Enum(`"abc"`)}}}, want: &AllOfDomain{Domains: []types.Domain{&StringDomain{MinLength: 1}, &StringDomain{MaxLength: new(5)}, &StringDomain{Enum: []types.Enum{types.Enum(`"abc"`)}}}, MergedDomain: &StringDomain{MinLength: 1, MaxLength: new(5), Enum: []types.Enum{types.Enum(`"abc"`)}}}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.left.AllOfMerge(tt.right)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestAllOfDomainAllOfMergeInvalidPlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *AllOfDomain
		right types.Domain
	}{
		"nil other":               {left: &AllOfDomain{}},
		"type conflict":           {left: &AllOfDomain{MergedDomain: &StringDomain{}}, right: &NumberDomain{Type: "number"}},
		"string enum empty":       {left: &AllOfDomain{MergedDomain: &StringDomain{Enum: []types.Enum{types.Enum(`"a"`)}}}, right: &StringDomain{Enum: []types.Enum{types.Enum(`"b"`)}}},
		"bool enum empty":         {left: &AllOfDomain{MergedDomain: &BoolDomain{Enum: []types.Enum{types.Enum("true")}}}, right: &BoolDomain{Enum: []types.Enum{types.Enum("false")}}},
		"array items conflict":    {left: &AllOfDomain{MergedDomain: &ArrayDomain{Items: &StringDomain{}}}, right: &ArrayDomain{Items: &BoolDomain{}}},
		"nullable no null domain": {left: &AllOfDomain{MergedDomain: &StringDomain{Nullable: true}}, right: &NumberDomain{Type: "integer", Nullable: true}},
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
		var left *AllOfDomain

		got, err := left.AllOfMerge(&StringDomain{})
		require.Error(t, err)
		require.Nil(t, got)
	})
}

func TestParseAllOfMergePlanCases(t *testing.T) {
	tests := map[string]struct {
		raw  string
		want types.Domain
	}{
		"strings":                {raw: `{"allOf":[{"type":"string","minLength":1},{"type":"string","maxLength":5}]}`, want: &AllOfDomain{Domains: []types.Domain{&StringDomain{MinLength: 1}, &StringDomain{MaxLength: new(5)}}, MergedDomain: &StringDomain{MinLength: 1, MaxLength: new(5)}}},
		"numbers":                {raw: `{"allOf":[{"type":"number"},{"type":"integer"}]}`, want: &AllOfDomain{Domains: []types.Domain{&NumberDomain{Type: "number"}, &NumberDomain{Type: "integer"}}, MergedDomain: &NumberDomain{Type: "integer"}}},
		"booleans":               {raw: `{"allOf":[{"type":"boolean","enum":[true,false]},{"type":"boolean","enum":[true]}]}`, want: &AllOfDomain{Domains: []types.Domain{&BoolDomain{Enum: []types.Enum{types.Enum("true"), types.Enum("false")}}, &BoolDomain{Enum: []types.Enum{types.Enum("true")}}}, MergedDomain: &BoolDomain{Enum: []types.Enum{types.Enum("true")}}}},
		"arrays":                 {raw: `{"allOf":[{"type":"array","items":{},"minItems":1},{"type":"array","items":{},"maxItems":3}]}`, want: &AllOfDomain{Domains: []types.Domain{&ArrayDomain{MinItems: 1}, &ArrayDomain{MaxItems: new(3)}}, MergedDomain: &ArrayDomain{MinItems: 1, MaxItems: new(3)}}},
		"nested":                 {raw: `{"allOf":[{"type":"string","minLength":1},{"allOf":[{"type":"string","maxLength":5},{"type":"string","enum":["abc"]}]}]}`, want: &AllOfDomain{Domains: []types.Domain{&StringDomain{MinLength: 1}, &StringDomain{MaxLength: new(5)}, &StringDomain{Enum: []types.Enum{types.Enum(`"abc"`)}}}, MergedDomain: &StringDomain{MinLength: 1, MaxLength: new(5), Enum: []types.Enum{types.Enum(`"abc"`)}}}},
		"sibling constraints":    {raw: `{"type":"string","maxLength":5,"allOf":[{"type":"string","minLength":1}]}`, want: &AllOfDomain{Domains: []types.Domain{&StringDomain{MinLength: 1}, &StringDomain{MaxLength: new(5)}}, MergedDomain: &StringDomain{MinLength: 1, MaxLength: new(5)}}},
		"nullable wrapper false": {raw: `{"nullable":false,"allOf":[{"type":"string","nullable":true}]}`, want: &AllOfDomain{Domains: []types.Domain{&StringDomain{Nullable: true}, &StringDomain{}}, MergedDomain: &StringDomain{}}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			raw := json.RawMessage(tt.raw)
			got, err := new(DomainContext).Parse(&raw)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseAllOfInvalidPlanCases(t *testing.T) {
	tests := map[string]struct{ raw string }{
		"incompatible primitive children": {raw: `{"allOf":[{"type":"string"},{"type":"boolean"}]}`},
		"enum empty intersection":         {raw: `{"allOf":[{"type":"string","enum":["a"]},{"type":"string","enum":["b"]}]}`},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			raw := json.RawMessage(tt.raw)
			got, err := new(DomainContext).Parse(&raw)
			require.Error(t, err)
			require.Nil(t, got)
		})
	}

	t.Run("child parse fails no partial store commit", func(t *testing.T) {
		raw := json.RawMessage(`{"allOf":[{"type":"string"},{"type":"nope"}]}`)
		dc := new(DomainContext)
		got, err := dc.Parse(&raw)
		require.Error(t, err)
		require.Nil(t, got)
		require.Empty(t, dc.domainStore)
	})

	t.Run("injected child parse fails no partial store commit", func(t *testing.T) {
		raw := json.RawMessage(`{"allOf":[{},{}]}`)
		dc := &DomainContext{}
		calls := 0
		dc.parse = func(node *json.RawMessage) (types.Domain, error) {
			calls++
			if calls == 2 {
				return nil, errors.New("boom")
			}

			return &StringDomain{}, nil
		}
		got, err := dc.Parse(&raw)
		require.Error(t, err)
		require.Nil(t, got)
		require.Empty(t, dc.domainStore)
	})
}
