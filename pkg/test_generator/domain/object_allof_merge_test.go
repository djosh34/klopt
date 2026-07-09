package domain

import (
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.

	"github.com/stretchr/testify/require"
)

// TestObjectDomainAllOfMergeValidPlanCases covers valid object intersections.
func TestObjectDomainAllOfMergeValidPlanCases(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		left  *ObjectDomain
		right types.Domain
		want  types.Domain
	}{
		"nullable true": {
			left:  &ObjectDomain{Nullable: true},
			right: &ObjectDomain{Nullable: true},
			want:  &ObjectDomain{Nullable: true},
		},
		"enum nil right": {
			left:  &ObjectDomain{},
			right: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1}`), types.Enum(`{"b":2}`)}},
			want:  &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1}`), types.Enum(`{"b":2}`)}},
		},
		"enum intersection": {
			left: &ObjectDomain{
				Enum: []types.Enum{types.Enum(`{"a":1}`), types.Enum(`{"b":2}`), types.Enum(`{"c":3}`)},
			},
			right: &ObjectDomain{
				Enum: []types.Enum{types.Enum(`{"b":2}`), types.Enum(`{"c":3}`), types.Enum(`{"d":4}`)},
			},
			want: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"b":2}`), types.Enum(`{"c":3}`)}},
		},
		"enum uses canonical order": {
			left:  &ObjectDomain{Enum: []types.Enum{types.Enum(`{"b":2}`), types.Enum(`{"a":1}`)}},
			right: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1}`), types.Enum(`{"b":2}`)}},
			want:  &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1}`), types.Enum(`{"b":2}`)}},
		},
		"nullable enum raw null": {
			left: &ObjectDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum(`null`), types.Enum(`{"a":1}`)},
			},
			right: &ObjectDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum(`{"b":2}`), types.Enum(`null`)},
			},
			want: &ObjectDomain{Nullable: true, Enum: []types.Enum{types.Enum(`null`)}},
		},
		"enum filters incompatible types": {
			left: &ObjectDomain{Enum: []types.Enum{
				types.Enum(`{"a":1}`), types.Enum(`"wrong"`), types.Enum(`null`),
			}},
			right: &ObjectDomain{},
			want:  &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1}`)}},
		},
		"enum object key order is insignificant": {
			left:  &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1,"b":2}`)}},
			right: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"b":2,"a":1}`)}},
			want:  &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1,"b":2}`)}},
		},
		"enum filters structural mismatches": {
			left: &ObjectDomain{
				Enum: []types.Enum{
					types.Enum(`{}`),
					types.Enum(`{"a":1}`),
					types.Enum(`{"a":1,"extra":2}`),
				},
				Properties:             []Property{{Key: "a", Required: true}},
				AdditionalPropertyKind: AdditionalFalse,
				MinProps:               1,
				MaxProps:               new(1),
			},
			right: &ObjectDomain{},
			want: &ObjectDomain{
				Enum:                   []types.Enum{types.Enum(`{"a":1}`)},
				Properties:             []Property{{Key: "a", Required: true}},
				AdditionalPropertyKind: AdditionalFalse,
				MinProps:               1,
				MaxProps:               new(1),
			},
		},
		"additional property schema permits undeclared enum names": {
			left: &ObjectDomain{
				Enum:                     []types.Enum{types.Enum(`{"extra":"ok"}`)},
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: &StringDomain{},
			},
			right: &ObjectDomain{},
			want: &ObjectDomain{
				Enum:                     []types.Enum{types.Enum(`{"extra":"ok"}`)},
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: &StringDomain{},
			},
		},
		"nullable retains impossible object constraints": {
			left:  &ObjectDomain{Nullable: true, MinProps: 10},
			right: &ObjectDomain{Nullable: true, MaxProps: new(5)},
			want:  &ObjectDomain{Nullable: true, MinProps: 10, MaxProps: new(5)},
		},
		"nullable required property type mismatch retains null": {
			left: &ObjectDomain{
				Nullable:   true,
				Properties: []Property{{Key: "value", Required: true, Domain: &StringDomain{}}},
			},
			right: &ObjectDomain{
				Nullable:   true,
				Properties: []Property{{Key: "value", Domain: &BoolDomain{}}},
			},
			want: &BoolDomain{Nullable: true, Enum: []types.Enum{types.Enum("null")}},
		},
		"nullable required property forbidden retains null": {
			left: &ObjectDomain{
				Nullable:   true,
				Properties: []Property{{Key: "value", Required: true}},
			},
			right: &ObjectDomain{Nullable: true, AdditionalPropertyKind: AdditionalFalse},
			want:  &BoolDomain{Nullable: true, Enum: []types.Enum{types.Enum("null")}},
		},
		"disjoint props sorted": {
			left:  &ObjectDomain{Properties: []Property{{Key: "b"}}, AdditionalPropertyKind: AdditionalTrue},
			right: &ObjectDomain{Properties: []Property{{Key: "a"}}, AdditionalPropertyKind: AdditionalTrue},
			want: &ObjectDomain{
				Properties:             []Property{{Key: "a"}, {Key: "b"}},
				AdditionalPropertyKind: AdditionalTrue,
			},
		},
		"same prop required or and domain merge": {
			left: &ObjectDomain{
				Properties: []Property{{Key: "a", Required: true, Domain: &StringDomain{MinLength: 1}}},
			},
			right: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MaxLength: new(5)}}}},
			want: &ObjectDomain{
				Properties: []Property{
					{Key: "a", Required: true, Domain: &StringDomain{MinLength: 1, MaxLength: new(5)}},
				},
			},
		},
		"same prop nil concrete": {
			left:  &ObjectDomain{Properties: []Property{{Key: "a"}}},
			right: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MinLength: 1}}}},
			want:  &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MinLength: 1}}}},
		},
		"optional prop dropped by additional false": {
			left:  &ObjectDomain{Properties: []Property{{Key: "a"}}},
			right: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse},
			want:  &ObjectDomain{AdditionalPropertyKind: AdditionalFalse},
		},
		"disjoint optional props both false dropped": {
			left:  &ObjectDomain{Properties: []Property{{Key: "a"}}, AdditionalPropertyKind: AdditionalFalse},
			right: &ObjectDomain{Properties: []Property{{Key: "b"}}, AdditionalPropertyKind: AdditionalFalse},
			want:  &ObjectDomain{AdditionalPropertyKind: AdditionalFalse},
		},
		"prop merged with additional schema": {
			left: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{MinLength: 1}}}},
			right: &ObjectDomain{
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)},
			},
			want: &ObjectDomain{
				Properties: []Property{
					{Key: "a", Domain: &StringDomain{MinLength: 1, MaxLength: new(5)}},
				},
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)},
			},
		},
		"additional true false": {
			left:  &ObjectDomain{AdditionalPropertyKind: AdditionalTrue},
			right: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse},
			want:  &ObjectDomain{AdditionalPropertyKind: AdditionalFalse},
		},
		"additional true schema": {
			left: &ObjectDomain{AdditionalPropertyKind: AdditionalTrue},
			right: &ObjectDomain{
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)},
			},
			want: &ObjectDomain{
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)},
			},
		},
		"additional schema": {
			left: &ObjectDomain{
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: &StringDomain{MinLength: 1},
			},
			right: &ObjectDomain{
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: &StringDomain{MaxLength: new(5)},
			},
			want: &ObjectDomain{
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: &StringDomain{MinLength: 1, MaxLength: new(5)},
			},
		},
		"disjoint additional schemas close the object": {
			left: &ObjectDomain{
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: &StringDomain{},
			},
			right: &ObjectDomain{
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: &BoolDomain{},
			},
			want: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.left.AllOfMerge(tt.right)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestObjectDomainAllOfMergeInvalidPlanCases covers impossible intersections.
func TestObjectDomainAllOfMergeInvalidPlanCases(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		left  *ObjectDomain
		right types.Domain
	}{
		"nil other": {
			left: &ObjectDomain{},
		},
		"string": {
			left:  &ObjectDomain{},
			right: &StringDomain{},
		},
		"number": {
			left:  &ObjectDomain{},
			right: &NumberDomain{Type: "number"},
		},
		"bool": {
			left:  &ObjectDomain{},
			right: &BoolDomain{},
		},
		"array": {
			left:  &ObjectDomain{},
			right: &ArrayDomain{},
		},
		"enum empty": {
			left:  &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":1}`)}},
			right: &ObjectDomain{Enum: []types.Enum{types.Enum(`{"a":2}`)}},
		},
		"enum nil raw message": {
			left:  &ObjectDomain{Enum: []types.Enum{nil}},
			right: &ObjectDomain{Enum: []types.Enum{types.Enum(`{}`)}},
		},
		"nonnullable enum raw null": {
			left:  &ObjectDomain{Enum: []types.Enum{types.Enum(`null`), types.Enum(`{"a":1}`)}},
			right: &ObjectDomain{Enum: []types.Enum{types.Enum(`null`)}},
		},
		"enum forbidden by closed object": {
			left: &ObjectDomain{
				Enum:                   []types.Enum{types.Enum(`{"forbidden":true}`)},
				AdditionalPropertyKind: AdditionalFalse,
			},
			right: &ObjectDomain{},
		},
		"enum misses required property": {
			left: &ObjectDomain{
				Enum:       []types.Enum{types.Enum(`{}`)},
				Properties: []Property{{Key: "a", Required: true}},
			},
			right: &ObjectDomain{},
		},
		"enum violates minimum property count": {
			left:  &ObjectDomain{Enum: []types.Enum{types.Enum(`{}`)}, MinProps: 1},
			right: &ObjectDomain{},
		},
		"same prop incompatible": {
			left:  &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{}}}},
			right: &ObjectDomain{Properties: []Property{{Key: "a", Domain: &BoolDomain{}}}},
		},
		"required forbidden": {
			left:  &ObjectDomain{Properties: []Property{{Key: "a", Required: true}}},
			right: &ObjectDomain{AdditionalPropertyKind: AdditionalFalse},
		},
		"prop additional schema incompatible": {
			left:  &ObjectDomain{Properties: []Property{{Key: "a", Domain: &StringDomain{}}}},
			right: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema, AdditionalPropertyDomain: &BoolDomain{}},
		},
		"additional schema nil left": {
			left:  &ObjectDomain{AdditionalPropertyKind: AdditionalSchema},
			right: &ObjectDomain{AdditionalPropertyKind: AdditionalTrue},
		},
		"additional schema nil right": {
			left:  &ObjectDomain{AdditionalPropertyKind: AdditionalTrue},
			right: &ObjectDomain{AdditionalPropertyKind: AdditionalSchema},
		},
		"incompatible allOf": {
			left:  &ObjectDomain{},
			right: &AllOfDomain{Domains: []types.Domain{&StringDomain{}}, MergedDomain: &StringDomain{}},
		},
		"minProperties exceeds maxProperties": {
			left:  &ObjectDomain{MinProps: 10},
			right: &ObjectDomain{MaxProps: new(5)},
		},
		"required count exceeds maxProperties": {
			left: &ObjectDomain{Properties: []Property{
				{Key: "a", Required: true},
				{Key: "b", Required: true},
			}},
			right: &ObjectDomain{MaxProps: new(1)},
		},
		"closed object cannot satisfy minProperties": {
			left: &ObjectDomain{
				Properties:             []Property{{Key: "a"}},
				AdditionalPropertyKind: AdditionalFalse,
				MinProps:               2,
			},
			right: &ObjectDomain{},
		},
		"negative minProperties on left": {
			left:  &ObjectDomain{MinProps: -1},
			right: &ObjectDomain{},
		},
		"negative maxProperties on right": {
			left:  &ObjectDomain{},
			right: &ObjectDomain{MaxProps: new(-1)},
		},
		"nullable does not hide malformed required property domain": {
			left: &ObjectDomain{
				Nullable:   true,
				Properties: []Property{{Key: "value", Required: true, Domain: &StringDomain{MinLength: -1}}},
			},
			right: &ObjectDomain{
				Nullable:   true,
				Properties: []Property{{Key: "value", Domain: &BoolDomain{}}},
			},
		},
		"nullable does not hide malformed additional property domain": {
			left: &ObjectDomain{
				Nullable:   true,
				Properties: []Property{{Key: "value", Required: true, Domain: &StringDomain{}}},
			},
			right: &ObjectDomain{
				Nullable:                 true,
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: &BoolDomain{Enum: []types.Enum{types.Enum("1")}},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			before := *tt.left
			got, err := tt.left.AllOfMerge(tt.right)
			require.Error(t, err)
			require.Nil(t, got)
			require.Equal(t, before, *tt.left)
		})
	}

	t.Run("nil receiver", func(t *testing.T) {
		t.Parallel()

		var left *ObjectDomain

		got, err := left.AllOfMerge(&ObjectDomain{})
		require.Error(t, err)
		require.Nil(t, got)
	})
}

// TestMergeObjectPropertiesReportsTheFirstPropertyByName verifies deterministic merge failures.
func TestMergeObjectPropertiesReportsTheFirstPropertyByName(t *testing.T) {
	t.Parallel()

	left := &ObjectDomain{Properties: []Property{
		{Key: "z", Domain: &StringDomain{}},
		{Key: "a", Domain: &StringDomain{}},
	}}
	right := &ObjectDomain{Properties: []Property{
		{Key: "z", Domain: &BoolDomain{}},
		{Key: "a", Domain: &BoolDomain{}},
	}}

	merged, err := left.AllOfMerge(right)
	require.Error(t, err)
	require.ErrorContains(t, err, `property "a"`)
	require.Nil(t, merged)
}

// TestObjectDomainAllOfMergeDoesNotMutateNestedAllOf checks non-mutating intersections.
func TestObjectDomainAllOfMergeDoesNotMutateNestedAllOf(t *testing.T) {
	t.Parallel()

	propertyAllOf := &AllOfDomain{
		Domains:      []types.Domain{&StringDomain{MinLength: 1}},
		MergedDomain: &StringDomain{MinLength: 1},
	}
	left := &ObjectDomain{
		Properties:               []Property{{Key: "a", Domain: propertyAllOf}},
		AdditionalPropertyKind:   AdditionalSchema,
		AdditionalPropertyDomain: &StringDomain{},
	}
	right := &ObjectDomain{
		Properties:               []Property{{Key: "a", Domain: &StringDomain{MaxLength: new(5)}}},
		AdditionalPropertyKind:   AdditionalSchema,
		AdditionalPropertyDomain: &BoolDomain{},
	}

	mergedDomain, err := left.AllOfMerge(right)
	require.NoError(t, err)
	require.Equal(t, &ObjectDomain{
		Properties: []Property{{
			Key: "a",
			Domain: &AllOfDomain{
				Domains: []types.Domain{
					&StringDomain{MinLength: 1},
					&StringDomain{MaxLength: new(5)},
				},
				MergedDomain: &StringDomain{MinLength: 1, MaxLength: new(5)},
			},
		}},
		AdditionalPropertyKind: AdditionalFalse,
	}, mergedDomain)
	require.Equal(t, &AllOfDomain{
		Domains:      []types.Domain{&StringDomain{MinLength: 1}},
		MergedDomain: &StringDomain{MinLength: 1},
	}, propertyAllOf)
}

// TestObjectDomainAllOfMergeEnumFilteringDoesNotMutateInputs covers successful filtering.
func TestObjectDomainAllOfMergeEnumFilteringDoesNotMutateInputs(t *testing.T) {
	t.Parallel()

	left := &ObjectDomain{
		Enum: []types.Enum{
			types.Enum(`{"extra":1}`),
			types.Enum(`{"a":1}`),
		},
		Properties:             []Property{{Key: "a", Required: true}},
		AdditionalPropertyKind: AdditionalFalse,
	}
	right := &ObjectDomain{}
	leftBefore := *left
	leftBefore.Enum = append([]types.Enum(nil), left.Enum...)
	rightBefore := *right

	merged, err := left.AllOfMerge(right)
	require.NoError(t, err)
	require.Equal(t, &ObjectDomain{
		Enum:                   []types.Enum{types.Enum(`{"a":1}`)},
		Properties:             []Property{{Key: "a", Required: true}},
		AdditionalPropertyKind: AdditionalFalse,
	}, merged)
	require.Equal(t, leftBefore, *left)
	require.Equal(t, rightBefore, *right)
}
