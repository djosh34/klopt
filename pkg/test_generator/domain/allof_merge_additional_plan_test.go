package domain

import (
	"encoding/json"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types"

	"github.com/stretchr/testify/require"
)

func TestParseAllOfAdditionalPlanCases(t *testing.T) {
	tests := map[string]struct {
		raw  string
		want types.Domain
	}{
		"objects": {
			raw: `{"allOf":[{"type":"object","properties":{"a":{"type":"string"}}},{"type":"object","properties":{"b":{"type":"string"}}}]}`,
			want: &AllOfDomain{
				Domains: []types.Domain{
					&ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{}}}, AdditionalPropertyKind: AdditionalTrue},
					&ObjectDomain{Properties: []Property{{Key: "b", Domain: &StringDomain{}}}, AdditionalPropertyKind: AdditionalTrue},
				},
				MergedDomain: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{}}, {Key: "b", Domain: &StringDomain{}}}, AdditionalPropertyKind: AdditionalTrue},
			},
		},
		"wrapper nullable absent keeps nullable child": {
			raw: `{"allOf":[{"type":"string","nullable":true}]}`,
			want: &AllOfDomain{
				Domains:      []types.Domain{&StringDomain{Nullable: true}},
				MergedDomain: &StringDomain{Nullable: true},
			},
		},
		"wrapper nullable true and child true": {
			raw: `{"nullable":true,"allOf":[{"type":"string","nullable":true}]}`,
			want: &AllOfDomain{
				Domains:      []types.Domain{&StringDomain{Nullable: true}, &StringDomain{Nullable: true}},
				MergedDomain: &StringDomain{Nullable: true},
			},
		},
		"wrapper nullable true and child false": {
			raw: `{"nullable":true,"allOf":[{"type":"string","nullable":false}]}`,
			want: &AllOfDomain{
				Domains:      []types.Domain{&StringDomain{}, &StringDomain{Nullable: true}},
				MergedDomain: &StringDomain{},
			},
		},
		"wrapper nullable false and child false": {
			raw: `{"nullable":false,"allOf":[{"type":"string","nullable":false}]}`,
			want: &AllOfDomain{
				Domains:      []types.Domain{&StringDomain{}, &StringDomain{}},
				MergedDomain: &StringDomain{},
			},
		},
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

func TestAllOfDomainAdditionalAllOfMergePlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *AllOfDomain
		right types.Domain
		want  *AllOfDomain
	}{
		"object props sorted": {
			left:  &AllOfDomain{Domains: []types.Domain{&ObjectDomain{Properties: []Property{{Key: "b"}}, AdditionalPropertyKind: AdditionalTrue}}, MergedDomain: &ObjectDomain{Properties: []Property{{Key: "b"}}, AdditionalPropertyKind: AdditionalTrue}},
			right: &ObjectDomain{Properties: []Property{{Key: "a"}}, AdditionalPropertyKind: AdditionalTrue},
			want: &AllOfDomain{
				Domains:      []types.Domain{&ObjectDomain{Properties: []Property{{Key: "b"}}, AdditionalPropertyKind: AdditionalTrue}, &ObjectDomain{Properties: []Property{{Key: "a"}}, AdditionalPropertyKind: AdditionalTrue}},
				MergedDomain: &ObjectDomain{Properties: []Property{{Key: "a"}, {Key: "b"}}, AdditionalPropertyKind: AdditionalTrue},
			},
		},
		"nullable false true": {
			left:  &AllOfDomain{MergedDomain: &StringDomain{}},
			right: &StringDomain{Nullable: true},
			want:  &AllOfDomain{Domains: []types.Domain{&StringDomain{Nullable: true}}, MergedDomain: &StringDomain{}},
		},
		"nullable false": {
			left:  &AllOfDomain{MergedDomain: &StringDomain{}},
			right: &StringDomain{},
			want:  &AllOfDomain{Domains: []types.Domain{&StringDomain{}}, MergedDomain: &StringDomain{}},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.left.AllOfMerge(tt.right)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestPrimitiveDomainAllOfDelegationPlanCases(t *testing.T) {
	tests := map[string]struct {
		left       types.Domain
		right      *AllOfDomain
		wantMerged types.Domain
	}{
		"string plus allOf": {
			left:       &StringDomain{MinLength: 1},
			right:      &AllOfDomain{Domains: []types.Domain{&StringDomain{MaxLength: new(5)}}, MergedDomain: &StringDomain{MaxLength: new(5)}},
			wantMerged: &StringDomain{MinLength: 1, MaxLength: new(5)},
		},
		"number plus allOf": {
			left:       &NumberDomain{Type: "number", Minimum: new(Number("1"))},
			right:      &AllOfDomain{Domains: []types.Domain{&NumberDomain{Type: "number", Maximum: new(Number("5"))}}, MergedDomain: &NumberDomain{Type: "number", Maximum: new(Number("5"))}},
			wantMerged: &NumberDomain{Type: "number", Minimum: new(Number("1")), Maximum: new(Number("5"))},
		},
		"bool plus allOf": {
			left:       &BoolDomain{},
			right:      &AllOfDomain{Domains: []types.Domain{&BoolDomain{Enum: []types.Enum{types.Enum("true")}}}, MergedDomain: &BoolDomain{Enum: []types.Enum{types.Enum("true")}}},
			wantMerged: &BoolDomain{Enum: []types.Enum{types.Enum("true")}},
		},
		"array plus allOf": {
			left:       &ArrayDomain{MinItems: 1},
			right:      &AllOfDomain{Domains: []types.Domain{&ArrayDomain{MaxItems: new(3)}}, MergedDomain: &ArrayDomain{MaxItems: new(3)}},
			wantMerged: &ArrayDomain{MinItems: 1, MaxItems: new(3)},
		},
		"object plus allOf": {
			left:       &ObjectDomain{MinProps: 1, AdditionalPropertyKind: AdditionalTrue},
			right:      &AllOfDomain{Domains: []types.Domain{&ObjectDomain{MaxProps: new(3), AdditionalPropertyKind: AdditionalTrue}}, MergedDomain: &ObjectDomain{MaxProps: new(3), AdditionalPropertyKind: AdditionalTrue}},
			wantMerged: &ObjectDomain{MinProps: 1, MaxProps: new(3), AdditionalPropertyKind: AdditionalTrue},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.left.AllOfMerge(tt.right)
			require.NoError(t, err)

			allOf, ok := got.(*AllOfDomain)
			require.True(t, ok)
			require.Equal(t, tt.wantMerged, allOf.MergedDomain)
		})
	}
}

func TestStringDomainAdditionalAllOfMergePlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *StringDomain
		right types.Domain
		want  types.Domain
	}{
		"enum nil":                            {left: &StringDomain{}, right: &StringDomain{}, want: &StringDomain{}},
		"pattern nil":                         {left: &StringDomain{}, right: &StringDomain{}, want: &StringDomain{}},
		"pattern nil right":                   {left: &StringDomain{}, right: &StringDomain{Pattern: types.Pattern{"p2"}}, want: &StringDomain{Pattern: types.Pattern{"p2"}}},
		"pattern left nil":                    {left: &StringDomain{Pattern: types.Pattern{"p1"}}, right: &StringDomain{}, want: &StringDomain{Pattern: types.Pattern{"p1"}}},
		"pattern concat":                      {left: &StringDomain{Pattern: types.Pattern{"p1"}}, right: &StringDomain{Pattern: types.Pattern{"p2"}}, want: &StringDomain{Pattern: types.Pattern{"p1", "p2"}}},
		"format nil":                          {left: &StringDomain{}, right: &StringDomain{}, want: &StringDomain{}},
		"format nil right":                    {left: &StringDomain{}, right: &StringDomain{Format: types.Format{"email"}}, want: &StringDomain{Format: types.Format{"email"}}},
		"format left nil":                     {left: &StringDomain{Format: types.Format{"uuid"}}, right: &StringDomain{}, want: &StringDomain{Format: types.Format{"uuid"}}},
		"format concat":                       {left: &StringDomain{Format: types.Format{"email"}}, right: &StringDomain{Format: types.Format{"uuid"}}, want: &StringDomain{Format: types.Format{"email", "uuid"}}},
		"valid examples nil":                  {left: &StringDomain{}, right: &StringDomain{}, want: &StringDomain{}},
		"valid examples left nil":             {left: &StringDomain{XValidExamples: []string{"a", "b"}}, right: &StringDomain{}, want: &StringDomain{XValidExamples: []string{"a", "b"}}},
		"valid examples preserves left order": {left: &StringDomain{XValidExamples: []string{"b", "a"}}, right: &StringDomain{XValidExamples: []string{"a", "b"}}, want: &StringDomain{XValidExamples: []string{"b", "a"}}},
		"enum valid preserves left order":     {left: &StringDomain{Enum: []types.Enum{types.Enum("\"b\""), types.Enum("\"a\"")}}, right: &StringDomain{XValidExamples: []string{"a", "b"}}, want: &StringDomain{Enum: []types.Enum{types.Enum("\"b\""), types.Enum("\"a\"")}, XValidExamples: []string{"b", "a"}}},
		"invalid examples nil":                {left: &StringDomain{}, right: &StringDomain{}, want: &StringDomain{}},
		"invalid examples nil right":          {left: &StringDomain{}, right: &StringDomain{XInvalidExamples: []string{"x"}}, want: &StringDomain{XInvalidExamples: []string{"x"}}},
		"invalid examples left nil":           {left: &StringDomain{XInvalidExamples: []string{"x"}}, right: &StringDomain{}, want: &StringDomain{XInvalidExamples: []string{"x"}}},
		"enum and invalid examples both kept": {left: &StringDomain{Enum: []types.Enum{types.Enum("\"ok\"")}, XInvalidExamples: []string{"ok"}}, right: &StringDomain{}, want: &StringDomain{Enum: []types.Enum{types.Enum("\"ok\"")}, XInvalidExamples: []string{"ok"}}},
		"min length larger right":             {left: &StringDomain{MinLength: 1}, right: &StringDomain{MinLength: 3}, want: &StringDomain{MinLength: 3}},
		"min length larger left":              {left: &StringDomain{MinLength: 3}, right: &StringDomain{MinLength: 1}, want: &StringDomain{MinLength: 3}},
		"max length nil left":                 {left: &StringDomain{}, right: &StringDomain{MaxLength: new(5)}, want: &StringDomain{MaxLength: new(5)}},
		"max length nil right":                {left: &StringDomain{MaxLength: new(5)}, right: &StringDomain{}, want: &StringDomain{MaxLength: new(5)}},
		"max length smaller":                  {left: &StringDomain{MaxLength: new(9)}, right: &StringDomain{MaxLength: new(5)}, want: &StringDomain{MaxLength: new(5)}},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.left.AllOfMerge(tt.right)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNumberDomainAdditionalAllOfMergePlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *NumberDomain
		right types.Domain
		want  types.Domain
	}{
		"nullable false":                        {left: &NumberDomain{Type: "number"}, right: &NumberDomain{Type: "number"}, want: &NumberDomain{Type: "number"}},
		"nullable false true":                   {left: &NumberDomain{Type: "number"}, right: &NumberDomain{Type: "number", Nullable: true}, want: &NumberDomain{Type: "number"}},
		"nullable true false":                   {left: &NumberDomain{Type: "number", Nullable: true}, right: &NumberDomain{Type: "number"}, want: &NumberDomain{Type: "number"}},
		"enum nil":                              {left: &NumberDomain{Type: "number"}, right: &NumberDomain{Type: "number"}, want: &NumberDomain{Type: "number"}},
		"enum left nil":                         {left: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1"), types.Enum("2")}}, right: &NumberDomain{Type: "number"}, want: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1"), types.Enum("2")}}},
		"enum preserves left order":             {left: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("2"), types.Enum("1")}}, right: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1"), types.Enum("2")}}, want: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("2"), types.Enum("1")}}},
		"minimum nil left":                      {left: &NumberDomain{Type: "number"}, right: &NumberDomain{Type: "number", Minimum: new(Number("1"))}, want: &NumberDomain{Type: "number", Minimum: new(Number("1"))}},
		"minimum nil right":                     {left: &NumberDomain{Type: "number", Minimum: new(Number("1"))}, right: &NumberDomain{Type: "number"}, want: &NumberDomain{Type: "number", Minimum: new(Number("1"))}},
		"minimum larger left":                   {left: &NumberDomain{Type: "number", Minimum: new(Number("2"))}, right: &NumberDomain{Type: "number", Minimum: new(Number("1"))}, want: &NumberDomain{Type: "number", Minimum: new(Number("2"))}},
		"minimum equal exclusive left":          {left: &NumberDomain{Type: "number", Minimum: new(Number("1")), ExclusiveMinimum: true}, right: &NumberDomain{Type: "number", Minimum: new(Number("1"))}, want: &NumberDomain{Type: "number", Minimum: new(Number("1")), ExclusiveMinimum: true}},
		"minimum exponent stricter keeps left":  {left: &NumberDomain{Type: "number", Minimum: new(Number("1e2"))}, right: &NumberDomain{Type: "number", Minimum: new(Number("99"))}, want: &NumberDomain{Type: "number", Minimum: new(Number("1e2"))}},
		"minimum negative larger":               {left: &NumberDomain{Type: "number", Minimum: new(Number("-5"))}, right: &NumberDomain{Type: "number", Minimum: new(Number("-2"))}, want: &NumberDomain{Type: "number", Minimum: new(Number("-2"))}},
		"maximum nil left":                      {left: &NumberDomain{Type: "number"}, right: &NumberDomain{Type: "number", Maximum: new(Number("10"))}, want: &NumberDomain{Type: "number", Maximum: new(Number("10"))}},
		"maximum nil right":                     {left: &NumberDomain{Type: "number", Maximum: new(Number("10"))}, right: &NumberDomain{Type: "number"}, want: &NumberDomain{Type: "number", Maximum: new(Number("10"))}},
		"maximum smaller left":                  {left: &NumberDomain{Type: "number", Maximum: new(Number("8"))}, right: &NumberDomain{Type: "number", Maximum: new(Number("10"))}, want: &NumberDomain{Type: "number", Maximum: new(Number("8"))}},
		"maximum equal exclusive left":          {left: &NumberDomain{Type: "number", Maximum: new(Number("1")), ExclusiveMaximum: true}, right: &NumberDomain{Type: "number", Maximum: new(Number("1"))}, want: &NumberDomain{Type: "number", Maximum: new(Number("1")), ExclusiveMaximum: true}},
		"maximum equal keeps left lexeme":       {left: &NumberDomain{Type: "number", Maximum: new(Number("1.0"))}, right: &NumberDomain{Type: "number", Maximum: new(Number("1")), ExclusiveMaximum: true}, want: &NumberDomain{Type: "number", Maximum: new(Number("1.0")), ExclusiveMaximum: true}},
		"maximum exponent compared numerically": {left: &NumberDomain{Type: "number", Maximum: new(Number("1e2"))}, right: &NumberDomain{Type: "number", Maximum: new(Number("99"))}, want: &NumberDomain{Type: "number", Maximum: new(Number("99"))}},
		"exclusive equal range not checked":     {left: &NumberDomain{Type: "number", Minimum: new(Number("5")), ExclusiveMinimum: true}, right: &NumberDomain{Type: "number", Maximum: new(Number("5")), ExclusiveMaximum: true}, want: &NumberDomain{Type: "number", Minimum: new(Number("5")), ExclusiveMinimum: true, Maximum: new(Number("5")), ExclusiveMaximum: true}},
		"multiple nil right":                    {left: &NumberDomain{Type: "number", MultipleOf: new(Number("2"))}, right: &NumberDomain{Type: "number"}, want: &NumberDomain{Type: "number", MultipleOf: new(Number("2"))}},
		"multiple decimal divisibility":         {left: &NumberDomain{Type: "number", MultipleOf: new(Number("0.5"))}, right: &NumberDomain{Type: "number", MultipleOf: new(Number("0.25"))}, want: &NumberDomain{Type: "number", MultipleOf: new(Number("0.5"))}},
		"integer type keeps decimal multiple":   {left: &NumberDomain{Type: "number", MultipleOf: new(Number("2.5"))}, right: &NumberDomain{Type: "integer"}, want: &NumberDomain{Type: "integer", MultipleOf: new(Number("2.5"))}},
		"format nil":                            {left: &NumberDomain{Type: "number"}, right: &NumberDomain{Type: "number"}, want: &NumberDomain{Type: "number"}},
		"format left nil":                       {left: &NumberDomain{Type: "number", Format: new("double")}, right: &NumberDomain{Type: "number"}, want: &NumberDomain{Type: "number", Format: new("double")}},
		"format same":                           {left: &NumberDomain{Type: "number", Format: new("float")}, right: &NumberDomain{Type: "number", Format: new("float")}, want: &NumberDomain{Type: "number", Format: new("float")}},
		"number integer int64 format":           {left: &NumberDomain{Type: "number"}, right: &NumberDomain{Type: "integer", Format: new("int64")}, want: &NumberDomain{Type: "integer", Format: new("int64")}},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.left.AllOfMerge(tt.right)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNumberDomainAdditionalInvalidAllOfMergePlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *NumberDomain
		right types.Domain
	}{
		"right bad type":                           {left: &NumberDomain{Type: "number"}, right: &NumberDomain{Type: "string"}},
		"enum no overlap":                          {left: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1")}}, right: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("2")}}},
		"enum raw null mismatch":                   {left: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1")}}, right: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("null")}}},
		"integer with right float unrepresentable": {left: &NumberDomain{Type: "integer"}, right: &NumberDomain{Type: "number", Format: new("float")}},
		"number double with integer":               {left: &NumberDomain{Type: "number", Format: new("double")}, right: &NumberDomain{Type: "integer"}},
		"integer with right double":                {left: &NumberDomain{Type: "integer"}, right: &NumberDomain{Type: "number", Format: new("double")}},
		"float conflicts with int format":          {left: &NumberDomain{Type: "number", Format: new("float")}, right: &NumberDomain{Type: "integer", Format: new("int32")}},
		"int format conflicts with float":          {left: &NumberDomain{Type: "integer", Format: new("int32")}, right: &NumberDomain{Type: "number", Format: new("float")}},
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
}

func TestArrayDomainAdditionalAllOfMergePlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *ArrayDomain
		right types.Domain
		want  types.Domain
	}{
		"nullable false":         {left: &ArrayDomain{}, right: &ArrayDomain{}, want: &ArrayDomain{}},
		"nullable false true":    {left: &ArrayDomain{}, right: &ArrayDomain{Nullable: true}, want: &ArrayDomain{}},
		"nullable true false":    {left: &ArrayDomain{Nullable: true}, right: &ArrayDomain{}, want: &ArrayDomain{}},
		"enum nil":               {left: &ArrayDomain{}, right: &ArrayDomain{}, want: &ArrayDomain{}},
		"enum left nil":          {left: &ArrayDomain{Enum: []types.Enum{types.Enum(`["a"]`), types.Enum(`["b"]`)}}, right: &ArrayDomain{}, want: &ArrayDomain{Enum: []types.Enum{types.Enum(`["a"]`), types.Enum(`["b"]`)}}},
		"min items larger right": {left: &ArrayDomain{MinItems: 1}, right: &ArrayDomain{MinItems: 3}, want: &ArrayDomain{MinItems: 3}},
		"min items larger left":  {left: &ArrayDomain{MinItems: 3}, right: &ArrayDomain{MinItems: 1}, want: &ArrayDomain{MinItems: 3}},
		"max items nil left":     {left: &ArrayDomain{}, right: &ArrayDomain{MaxItems: new(5)}, want: &ArrayDomain{MaxItems: new(5)}},
		"max items nil right":    {left: &ArrayDomain{MaxItems: new(5)}, right: &ArrayDomain{}, want: &ArrayDomain{MaxItems: new(5)}},
		"max items smaller":      {left: &ArrayDomain{MaxItems: new(9)}, right: &ArrayDomain{MaxItems: new(5)}, want: &ArrayDomain{MaxItems: new(5)}},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.left.AllOfMerge(tt.right)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestArrayDomainAdditionalInvalidAllOfMergePlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *ArrayDomain
		right types.Domain
	}{
		"enum raw null mismatch": {left: &ArrayDomain{Enum: []types.Enum{types.Enum(`["a"]`)}}, right: &ArrayDomain{Enum: []types.Enum{types.Enum(`null`)}}},
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
}

func TestObjectDomainAdditionalAllOfMergePlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *ObjectDomain
		right types.Domain
		want  types.Domain
	}{
		"nullable false":                           {left: &ObjectDomain{}, right: &ObjectDomain{}, want: &ObjectDomain{}},
		"nullable false true":                      {left: &ObjectDomain{}, right: &ObjectDomain{Nullable: true}, want: &ObjectDomain{}},
		"nullable true false":                      {left: &ObjectDomain{Nullable: true}, right: &ObjectDomain{}, want: &ObjectDomain{}},
		"enum nil":                                 {left: &ObjectDomain{}, right: &ObjectDomain{}, want: &ObjectDomain{}},
		"enum left nil":                            {left: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1}`), types.Enum(`{"b":2}`)}}, right: &ObjectDomain{}, want: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1}`), types.Enum(`{"b":2}`)}}},
		"disjoint props already sorted":            {left: &ObjectDomain{Properties: []Property{{Key: "a"}}, AdditionalPropertyKind: AdditionalTrue}, right: &ObjectDomain{Properties: []Property{{Key: "b"}}, AdditionalPropertyKind: AdditionalTrue}, want: &ObjectDomain{Properties: []Property{{Key: "a"}, {Key: "b"}}, AdditionalPropertyKind: AdditionalTrue}},
		"same prop optional nil":                   {left: &ObjectDomain{Properties: []Property{{Key: "a"}}}, right: &ObjectDomain{Properties: []Property{{Key: "a"}}}, want: &ObjectDomain{Properties: []Property{{Key: "a"}}}},
		"same prop required left":                  {left: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}}, right: &ObjectDomain{Properties: []Property{{Key: "a"}}}, want: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}}},
		"same prop required right":                 {left: &ObjectDomain{Properties: []Property{{Key: "a"}}}, right: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}}, want: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}}},
		"same prop required both":                  {left: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}}, right: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}}, want: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}}},
		"same prop concrete nil reverse":           {left: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MinLength: 1}}}}, right: &ObjectDomain{Properties: []Property{{Key: "a"}}}, want: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MinLength: 1}}}}},
		"same prop false additional":               {left: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{}}}, AdditionalPropertyKind: AdditionalFalse}, right: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MinLength: 1}}}, AdditionalPropertyKind: AdditionalFalse}, want: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MinLength: 1}}}, AdditionalPropertyKind: AdditionalFalse}},
		"left required prop additional true":       {left: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalTrue}, want: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}, AdditionalPropertyKind: AdditionalTrue}},
		"right required prop additional true":      {left: &ObjectDomain{AdditionalPropertyKind: AdditionalTrue}, right: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}}, want: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}, AdditionalPropertyKind: AdditionalTrue}},
		"right optional prop dropped by false":     {left: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}, right: &ObjectDomain{Properties: []Property{{Key: "a"}}}, want: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}},
		"required prop merged with schema":         {left: &ObjectDomain{Properties: []Property{{Key: "a", Required: true, Domain: &StringDomain{MinLength: 1}}}}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}, want: &ObjectDomain{Properties: []Property{{Key: "a", Required: true, Domain: &StringDomain{MinLength: 1, MaxLength: new(5)}}}, AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}},
		"optional nil prop merged with schema":     {left: &ObjectDomain{Properties: []Property{{Key: "a"}}}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}, want: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MaxLength: new(5)}}}, AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}},
		"required nil prop merged with schema":     {left: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}, want: &ObjectDomain{Properties: []Property{{Key: "a", Required: true, Domain: &StringDomain{MaxLength: new(5)}}}, AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}},
		"right prop merged with left schema":       {left: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}, right: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MinLength: 1}}}}, want: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MinLength: 1, MaxLength: new(5)}}}, AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}},
		"right optional nil prop with left schema": {left: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}, right: &ObjectDomain{Properties: []Property{{Key: "a"}}}, want: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MaxLength: new(5)}}}, AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}},
		"right required nil prop with left schema": {left: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}, right: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}}, want: &ObjectDomain{Properties: []Property{{Key: "a", Required: true, Domain: &StringDomain{MaxLength: new(5)}}}, AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}},
		"additional true":                          {left: &ObjectDomain{AdditionalPropertyKind: AdditionalTrue}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalTrue}, want: &ObjectDomain{AdditionalPropertyKind: AdditionalTrue}},
		"additional false true":                    {left: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalTrue}, want: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}},
		"additional false":                         {left: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}, want: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}},
		"additional schema true":                   {left: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalTrue}, want: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}},
		"additional false schema":                  {left: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)}}, want: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}},
		"min props larger right":                   {left: &ObjectDomain{MinProps: 1}, right: &ObjectDomain{MinProps: 3}, want: &ObjectDomain{MinProps: 3}},
		"min props larger left":                    {left: &ObjectDomain{MinProps: 3}, right: &ObjectDomain{MinProps: 1}, want: &ObjectDomain{MinProps: 3}},
		"max props nil left":                       {left: &ObjectDomain{}, right: &ObjectDomain{MaxProps: new(5)}, want: &ObjectDomain{MaxProps: new(5)}},
		"max props nil right":                      {left: &ObjectDomain{MaxProps: new(5)}, right: &ObjectDomain{}, want: &ObjectDomain{MaxProps: new(5)}},
		"max props smaller":                        {left: &ObjectDomain{MaxProps: new(9)}, right: &ObjectDomain{MaxProps: new(5)}, want: &ObjectDomain{MaxProps: new(5)}},
		"min props no enough props check":          {left: &ObjectDomain{Properties: []Property{{Key: "a"}}, AdditionalPropertyKind: AdditionalFalse, MinProps: 5}, right: &ObjectDomain{}, want: &ObjectDomain{Properties: []Property{{Key: "a"}}, AdditionalPropertyKind: AdditionalFalse, MinProps: 5}},
		"all fields": {
			left:  &ObjectDomain{Nullable: true, Enum: []types.Enum{types.Enum(`{"a":1}`), types.Enum(`{"b":2}`)}, Properties: []Property{{Key: "a", Required: true, Domain: &StringDomain{MinLength: 1}}, {Key: "b", Domain: &StringDomain{}}}, AdditionalPropertyKind: AdditionalFalse, MinProps: 1, MaxProps: new(10)},
			right: &ObjectDomain{Nullable: true, Enum: []types.Enum{types.Enum(`{"b":2}`), types.Enum(`{"c":3}`)}, Properties: []Property{{Key: "a", Domain: &StringDomain{MaxLength: new(5)}}}, AdditionalPropertyKind: AdditionalFalse, MinProps: 2, MaxProps: new(8)},
			want:  &ObjectDomain{Nullable: true, Enum: []types.Enum{types.Enum(`{"b":2}`)}, Properties: []Property{{Key: "a", Required: true, Domain: &StringDomain{MinLength: 1, MaxLength: new(5)}}}, AdditionalPropertyKind: AdditionalFalse, MinProps: 2, MaxProps: new(8)},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.left.AllOfMerge(tt.right)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestObjectDomainAdditionalInvalidAllOfMergePlanCases(t *testing.T) {
	tests := map[string]struct {
		left  *ObjectDomain
		right types.Domain
	}{
		"enum raw null mismatch":                {left: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1}`)}}, right: &ObjectDomain{Enum: []types.Enum{types.Enum(`null`)}}},
		"required forbidden reverse":            {left: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}, right: &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}}},
		"right prop additional schema mismatch": {left: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &BoolDomain{}}, right: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{}}}}},
		"additional schema merge fails":         {left: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: failingToHasherDomain{}}, right: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &StringDomain{}}},
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
}
