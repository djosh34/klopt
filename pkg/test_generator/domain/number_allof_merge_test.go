package domain

import (
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.

	"github.com/stretchr/testify/require"
)

// TestNumberDomainAllOfMergeValidPlanCases covers numeric domain intersections.
func TestNumberDomainAllOfMergeValidPlanCases(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		left  *NumberDomain
		right types.Domain
		want  types.Domain
	}{
		"number": {
			left:  &NumberDomain{Type: "number"},
			right: &NumberDomain{Type: "number"},
			want:  &NumberDomain{Type: "number"},
		},
		"integer": {
			left:  &NumberDomain{Type: "integer"},
			right: &NumberDomain{Type: "integer"},
			want:  &NumberDomain{Type: "integer"},
		},
		"number integer": {
			left:  &NumberDomain{Type: "number"},
			right: &NumberDomain{Type: "integer"},
			want:  &NumberDomain{Type: "integer"},
		},
		"integer number": {
			left:  &NumberDomain{Type: "integer"},
			right: &NumberDomain{Type: "number"},
			want:  &NumberDomain{Type: "integer"},
		},
		"nullable true": {
			left:  &NumberDomain{Type: "number", Nullable: true},
			right: &NumberDomain{Type: "number", Nullable: true},
			want:  &NumberDomain{Type: "number", Nullable: true},
		},
		"enum nil right": {
			left: &NumberDomain{Type: "number"},
			right: &NumberDomain{
				Type: "number",
				Enum: []types.Enum{types.Enum("1"), types.Enum("2")},
			},
			want: &NumberDomain{
				Type: "number",
				Enum: []types.Enum{types.Enum("1"), types.Enum("2")},
			},
		},
		"enum numeric values compare semantically": {
			left: &NumberDomain{
				Type: "number",
				Enum: []types.Enum{types.Enum("1"), types.Enum("2")},
			},
			right: &NumberDomain{
				Type: "number",
				Enum: []types.Enum{types.Enum("2.0"), types.Enum("1e0")},
			},
			want: &NumberDomain{
				Type: "number",
				Enum: []types.Enum{types.Enum("1"), types.Enum("2")},
			},
		},
		"minimum larger": {
			left:  &NumberDomain{Type: "number", Minimum: new(Number("1"))},
			right: &NumberDomain{Type: "number", Minimum: new(Number("2"))},
			want:  &NumberDomain{Type: "number", Minimum: new(Number("2"))},
		},
		"minimum equal exclusive": {
			left: &NumberDomain{Type: "number", Minimum: new(Number("1"))},
			right: &NumberDomain{
				Type:             "number",
				Minimum:          new(Number("1")),
				ExclusiveMinimum: true,
			},
			want: &NumberDomain{
				Type:             "number",
				Minimum:          new(Number("1")),
				ExclusiveMinimum: true,
			},
		},
		"minimum equal is canonicalized": {
			left: &NumberDomain{Type: "number", Minimum: new(Number("1.0"))},
			right: &NumberDomain{
				Type:             "number",
				Minimum:          new(Number("1")),
				ExclusiveMinimum: true,
			},
			want: &NumberDomain{
				Type:             "number",
				Minimum:          new(Number("1")),
				ExclusiveMinimum: true,
			},
		},
		"maximum smaller": {
			left:  &NumberDomain{Type: "number", Maximum: new(Number("10"))},
			right: &NumberDomain{Type: "number", Maximum: new(Number("8"))},
			want:  &NumberDomain{Type: "number", Maximum: new(Number("8"))},
		},
		"maximum equal exclusive": {
			left: &NumberDomain{Type: "number", Maximum: new(Number("1"))},
			right: &NumberDomain{
				Type:             "number",
				Maximum:          new(Number("1")),
				ExclusiveMaximum: true,
			},
			want: &NumberDomain{
				Type:             "number",
				Maximum:          new(Number("1")),
				ExclusiveMaximum: true,
			},
		},
		"multiple nil right": {
			left:  &NumberDomain{Type: "number"},
			right: &NumberDomain{Type: "number", MultipleOf: new(Number("2"))},
			want:  &NumberDomain{Type: "number", MultipleOf: new(Number("2"))},
		},
		"multiple lcm integers": {
			left:  &NumberDomain{Type: "number", MultipleOf: new(Number("2"))},
			right: &NumberDomain{Type: "number", MultipleOf: new(Number("3"))},
			want:  &NumberDomain{Type: "number", MultipleOf: new(Number("6"))},
		},
		"multiple rationals": {
			left:  &NumberDomain{Type: "number", MultipleOf: new(Number("1.5"))},
			right: &NumberDomain{Type: "number", MultipleOf: new(Number("2.5"))},
			want:  &NumberDomain{Type: "number", MultipleOf: new(Number("7.5"))},
		},
		"format nil right": {
			left:  &NumberDomain{Type: "number"},
			right: &NumberDomain{Type: "number", Format: new("float")},
			want:  &NumberDomain{Type: "number", Format: new("float")},
		},
		"integer format": {
			left:  &NumberDomain{Type: "integer", Format: new("int32")},
			right: &NumberDomain{Type: "integer", Format: new("int32")},
			want:  &NumberDomain{Type: "integer", Format: new("int32")},
		},
		"different formats fall back to type": {
			left:  &NumberDomain{Type: "number", Format: new("float")},
			right: &NumberDomain{Type: "number", Format: new("double")},
			want:  &NumberDomain{Type: "number"},
		},
		"integer formats fall back to type": {
			left:  &NumberDomain{Type: "integer", Format: new("int32")},
			right: &NumberDomain{Type: "integer", Format: new("int64")},
			want:  &NumberDomain{Type: "integer"},
		},
		"number-only format falls back after integer intersection": {
			left:  &NumberDomain{Type: "number", Format: new("float")},
			right: &NumberDomain{Type: "integer"},
			want:  &NumberDomain{Type: "integer"},
		},
		"nullable intersection permits null despite impossible numeric range": {
			left: &NumberDomain{
				Type:     "number",
				Nullable: true,
				Minimum:  new(Number("10")),
			},
			right: &NumberDomain{
				Type:     "number",
				Nullable: true,
				Maximum:  new(Number("5")),
			},
			want: &NumberDomain{
				Type:     "number",
				Nullable: true,
				Minimum:  new(Number("10")),
				Maximum:  new(Number("5")),
			},
		},
		"nullable enum intersection permits only null": {
			left: &NumberDomain{
				Type:     "number",
				Nullable: true,
				Enum:     []types.Enum{types.Enum("null"), types.Enum("1")},
			},
			right: &NumberDomain{
				Type:     "number",
				Nullable: true,
				Enum:     []types.Enum{types.Enum("null"), types.Enum("2")},
			},
			want: &NumberDomain{
				Type:     "number",
				Nullable: true,
				Enum:     []types.Enum{types.Enum("null")},
			},
		},
		"integer decimal multiple has a bounded solution": {
			left: &NumberDomain{
				Type:    "integer",
				Minimum: new(Number("4")),
				Maximum: new(Number("6")),
			},
			right: &NumberDomain{Type: "number", MultipleOf: new(Number("2.5"))},
			want: &NumberDomain{
				Type:       "integer",
				Minimum:    new(Number("4")),
				Maximum:    new(Number("6")),
				MultipleOf: new(Number("2.5")),
			},
		},
		"all fields": {
			left: &NumberDomain{
				Type:       "number",
				Nullable:   true,
				Enum:       []types.Enum{types.Enum("1"), types.Enum("4")},
				Minimum:    new(Number("0")),
				Maximum:    new(Number("10")),
				MultipleOf: new(Number("2")),
			},
			right: &NumberDomain{
				Type:       "integer",
				Nullable:   true,
				Enum:       []types.Enum{types.Enum("3"), types.Enum("4")},
				Minimum:    new(Number("1")),
				Maximum:    new(Number("8")),
				MultipleOf: new(Number("4")),
				Format:     new("int64"),
			},
			want: &NumberDomain{
				Type:       "integer",
				Nullable:   true,
				Enum:       []types.Enum{types.Enum("4")},
				Minimum:    new(Number("1")),
				Maximum:    new(Number("8")),
				MultipleOf: new(Number("4")),
				Format:     new("int64"),
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

// TestNumberDomainAllOfMergeInvalidPlanCases covers invalid numeric intersections.
func TestNumberDomainAllOfMergeInvalidPlanCases(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		left  *NumberDomain
		right types.Domain
	}{
		"nil other": {
			left: &NumberDomain{Type: "number"},
		},
		"string": {
			left:  &NumberDomain{Type: "number"},
			right: &StringDomain{},
		},
		"bool": {
			left:  &NumberDomain{Type: "number"},
			right: &BoolDomain{},
		},
		"array": {
			left:  &NumberDomain{Type: "number"},
			right: &ArrayDomain{},
		},
		"object": {
			left:  &NumberDomain{Type: "number"},
			right: &ObjectDomain{},
		},
		"left empty type": {
			left:  &NumberDomain{},
			right: &NumberDomain{Type: "number"},
		},
		"right empty type": {
			left:  &NumberDomain{Type: "number"},
			right: &NumberDomain{},
		},
		"left bad type": {
			left:  &NumberDomain{Type: "string"},
			right: &NumberDomain{Type: "number"},
		},
		"enum mismatch": {
			left:  &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1")}},
			right: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("2")}},
		},
		"non-nullable null intersection": {
			left:  &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1"), types.Enum("null")}},
			right: &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("null"), types.Enum("2")}},
		},
		"minimum exceeds maximum": {
			left:  &NumberDomain{Type: "number", Minimum: new(Number("10"))},
			right: &NumberDomain{Type: "number", Maximum: new(Number("5"))},
		},
		"equal range excludes its only value": {
			left: &NumberDomain{
				Type:             "number",
				Minimum:          new(Number("1")),
				ExclusiveMinimum: true,
			},
			right: &NumberDomain{Type: "number", Maximum: new(Number("1"))},
		},
		"integer intersection has no integer in fractional range": {
			left: &NumberDomain{
				Type:    "number",
				Minimum: new(Number("1.1")),
				Maximum: new(Number("1.9")),
			},
			right: &NumberDomain{Type: "integer"},
		},
		"exact range value is not a multiple": {
			left: &NumberDomain{
				Type:    "number",
				Minimum: new(Number("1")),
				Maximum: new(Number("1")),
			},
			right: &NumberDomain{Type: "number", MultipleOf: new(Number("2"))},
		},
		"fractional enum is excluded by integer intersection": {
			left:  &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1.5")}},
			right: &NumberDomain{Type: "integer"},
		},
		"enum is excluded by merged minimum": {
			left:  &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1")}},
			right: &NumberDomain{Type: "number", Minimum: new(Number("2"))},
		},
		"enum is excluded by merged multiple": {
			left:  &NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1")}},
			right: &NumberDomain{Type: "number", MultipleOf: new(Number("2"))},
		},
		"bad minimum": {
			left:  &NumberDomain{Type: "number", Minimum: new(Number("bad"))},
			right: &NumberDomain{Type: "number", Minimum: new(Number("1"))},
		},
		"bad maximum": {
			left:  &NumberDomain{Type: "number", Maximum: new(Number("bad"))},
			right: &NumberDomain{Type: "number", Maximum: new(Number("1"))},
		},
		"bad multiple": {
			left:  &NumberDomain{Type: "number", MultipleOf: new(Number("bad"))},
			right: &NumberDomain{Type: "number", MultipleOf: new(Number("2"))},
		},
		"incompatible allOf": {
			left: &NumberDomain{Type: "number"},
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

		var left *NumberDomain

		got, err := left.AllOfMerge(&NumberDomain{Type: "number"})
		require.Error(t, err)
		require.Nil(t, got)
	})
}
