package domain

import (
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.

	"github.com/stretchr/testify/require"
)

// TestParseNumberParsesValidNumberSchemas covers supported numeric constraints.
func TestParseNumberParsesValidNumberSchemas(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		yamlString string
		expected   NumberDomain
	}{
		"minimal number": {
			yamlString: `
type: number
`,
			expected: NumberDomain{Type: "number"},
		},
		"title and description are allowed documentation fields": {
			yamlString: `
type: number
title: Amount
description: A decimal amount.
`,
			expected: NumberDomain{Type: "number"},
		},
		"specification extension is ignored": {
			yamlString: `
type: number
x-extra: 1
`,
			expected: NumberDomain{Type: "number"},
		},
		"nullable true": {
			yamlString: `
type: number
nullable: true
`,
			expected: NumberDomain{Type: "number", Nullable: true},
		},
		"nullable false": {
			yamlString: `
type: number
nullable: false
`,
			expected: NumberDomain{Type: "number"},
		},
		"enum numbers": {
			yamlString: `
type: number
enum:
  - 1
  - 2.5
`,
			expected: NumberDomain{
				Type: "number",
				Enum: []types.Enum{types.Enum("1"), types.Enum("2.5")},
			},
		},
		"enum filters incompatible values and duplicates": {
			yamlString: `
type: number
enum:
  - 2.5
  - "2.5"
  - null
  - 1.0
  - 1
`,
			expected: NumberDomain{
				Type: "number",
				Enum: []types.Enum{types.Enum("1"), types.Enum("2.5")},
			},
		},
		"integer enum filters fractional numbers": {
			yamlString: `
type: integer
enum:
  - 1.5
  - 1000
  - 0.001
  - 1
`,
			expected: NumberDomain{
				Type: "integer",
				Enum: []types.Enum{types.Enum("1"), types.Enum("1e3")},
			},
		},
		"nullable number enum retains null": {
			yamlString: `
type: number
nullable: true
enum:
  - null
  - 1
`,
			expected: NumberDomain{
				Type:     "number",
				Nullable: true,
				Enum:     []types.Enum{types.Enum("1"), types.Enum("null")},
			},
		},
		"minimum maximum and exclusive bounds": {
			yamlString: `
type: number
minimum: 1.5
exclusiveMinimum: true
maximum: 9.5
exclusiveMaximum: true
`,
			expected: NumberDomain{
				Type:             "number",
				Minimum:          new(Number("1.5")),
				Maximum:          new(Number("9.5")),
				ExclusiveMinimum: true,
				ExclusiveMaximum: true,
			},
		},
		"multipleOf": {
			yamlString: `
type: number
multipleOf: 2.5
`,
			expected: NumberDomain{Type: "number", MultipleOf: new(Number("2.5"))},
		},
		"format float": {
			yamlString: `
type: number
format: float
`,
			expected: NumberDomain{Type: "number", Format: new("float")},
		},
		"format double": {
			yamlString: `
type: number
format: double
`,
			expected: NumberDomain{Type: "number", Format: new("double")},
		},
		"arbitrary format": {
			yamlString: `
type: number
format: decimal128
`,
			expected: NumberDomain{Type: "number", Format: new("decimal128")},
		},
		"integer int32": {
			yamlString: `
type: integer
format: int32
enum:
  - 2
minimum: -10
maximum: 10
multipleOf: 2
`,
			expected: NumberDomain{
				Type:       "integer",
				Enum:       []types.Enum{types.Enum("2")},
				Minimum:    new(Number("-10")),
				Maximum:    new(Number("10")),
				MultipleOf: new(Number("2")),
				Format:     new("int32"),
			},
		},
		"integer int64": {
			yamlString: `
type: integer
format: int64
`,
			expected: NumberDomain{Type: "integer", Format: new("int64")},
		},
		"integer accepts numeric constraint values with fractions and exponents": {
			yamlString: `
type: integer
minimum: 1.5
maximum: 1.0e+2
multipleOf: 2.5
format: float
`,
			expected: NumberDomain{
				Type:       "integer",
				Minimum:    new(Number("1.5")),
				Maximum:    new(Number("100")),
				MultipleOf: new(Number("2.5")),
				Format:     new("float"),
			},
		},
		"all supported fields together": {
			yamlString: `
type: number
nullable: true
enum:
  - 2.5
minimum: 0.5
exclusiveMinimum: true
maximum: 10.5
exclusiveMaximum: false
multipleOf: 2.5
format: double
`,
			expected: NumberDomain{
				Type:             "number",
				Nullable:         true,
				Enum:             []types.Enum{types.Enum("2.5")},
				Minimum:          new(Number("0.5")),
				Maximum:          new(Number("10.5")),
				ExclusiveMinimum: true,
				MultipleOf:       new(Number("2.5")),
				Format:           new("double"),
			},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			node := rawObjectFromYAML(t, tt.yamlString)
			dc := Context{domainStore: domainStore{}}
			numberDomain, err := dc.ParseNumber(node)
			require.NoError(t, err)
			require.Equal(t, tt.expected, numberDomain)
		})
	}
}

// TestParseNumberChecksExactSatisfiability covers numeric-set existence and enum filtering.
func TestParseNumberChecksExactSatisfiability(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		yamlString string
		expected   NumberDomain
	}{
		"nullable rescues an impossible numeric range": {
			yamlString: `
type: number
nullable: true
minimum: 1
exclusiveMinimum: true
maximum: 1
`,
			expected: NumberDomain{
				Type:             "number",
				Nullable:         true,
				Minimum:          new(Number("1")),
				Maximum:          new(Number("1")),
				ExclusiveMinimum: true,
			},
		},
		"nullable null enum rescues an impossible numeric range": {
			yamlString: `
type: number
nullable: true
enum:
  - null
minimum: 1
exclusiveMinimum: true
maximum: 1
`,
			expected: NumberDomain{
				Type:             "number",
				Nullable:         true,
				Enum:             []types.Enum{types.Enum("null")},
				Minimum:          new(Number("1")),
				Maximum:          new(Number("1")),
				ExclusiveMinimum: true,
			},
		},
		"integer exact value satisfies decimal multiple": {
			yamlString: `
type: integer
minimum: 5
maximum: 5
multipleOf: 2.5
`,
			expected: NumberDomain{
				Type:       "integer",
				Minimum:    new(Number("5")),
				Maximum:    new(Number("5")),
				MultipleOf: new(Number("2.5")),
			},
		},
		"integer exact value satisfies subunit multiple": {
			yamlString: `
type: integer
minimum: 1
maximum: 1
multipleOf: 0.25
`,
			expected: NumberDomain{
				Type:       "integer",
				Minimum:    new(Number("1")),
				Maximum:    new(Number("1")),
				MultipleOf: new(Number("0.25")),
			},
		},
		"fractional bounds contain an integer": {
			yamlString: `
type: integer
minimum: 1.1
maximum: 2
`,
			expected: NumberDomain{
				Type:    "integer",
				Minimum: new(Number("1.1")),
				Maximum: new(Number("2")),
			},
		},
		"open interval contains a decimal multiple": {
			yamlString: `
type: number
minimum: 0
exclusiveMinimum: true
maximum: 1
exclusiveMaximum: true
multipleOf: 0.5
`,
			expected: NumberDomain{
				Type:             "number",
				Minimum:          new(Number("0")),
				Maximum:          new(Number("1")),
				ExclusiveMinimum: true,
				ExclusiveMaximum: true,
				MultipleOf:       new(Number("0.5")),
			},
		},
		"negative fractional upper bound has a preceding multiple": {
			yamlString: `
type: number
maximum: -1.1
multipleOf: 1
`,
			expected: NumberDomain{
				Type:       "number",
				Maximum:    new(Number("-1.1")),
				MultipleOf: new(Number("1")),
			},
		},
		"enum filters values excluded by numeric constraints": {
			yamlString: `
type: number
enum:
  - 1
  - 2
  - 3
minimum: 2
multipleOf: 2
`,
			expected: NumberDomain{
				Type:       "number",
				Enum:       []types.Enum{types.Enum("2")},
				Minimum:    new(Number("2")),
				MultipleOf: new(Number("2")),
			},
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			node := rawObjectFromYAML(t, testCase.yamlString)
			domainContext := Context{domainStore: domainStore{}}
			numberDomain, err := domainContext.ParseNumber(node)
			require.NoError(t, err)
			require.Equal(t, testCase.expected, numberDomain)
		})
	}
}

// TestParseNumberRejectsInvalidNumberSchemas covers malformed and unsupported fields.
func TestParseNumberRejectsInvalidNumberSchemas(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"missing type": `
nullable: false
`,
		"wrong type": `
type: string
`,
		"mixed type array": `
type:
  - number
  - string
`,
		"nullable must be boolean": `
type: number
nullable: nope
`,
		"enum cannot be empty": `
type: number
enum: []
`,
		"enum cannot be null": `
type: number
enum: null
`,
		"enum must be array": `
type: number
enum: 1
`,
		"number enum must contain a compatible value": `
type: number
enum:
  - null
  - "1"
  - true
`,
		"integer enum must contain an integral value": `
type: integer
enum:
  - 1.5
  - 0.001
`,
		"minimum cannot be null": `
type: number
minimum: null
`,
		"minimum must be a number": `
type: number
minimum: nope
`,
		"minimum cannot be a quoted number": `
type: number
minimum: "1"
`,
		"maximum cannot be null": `
type: number
maximum: null
`,
		"maximum must be a number": `
type: number
maximum: nope
`,
		"maximum cannot be a quoted number": `
type: number
maximum: "1"
`,
		"minimum cannot exceed maximum": `
type: number
minimum: 2
maximum: 1
`,
		"exclusive equal bounds are impossible": `
type: number
minimum: 1
exclusiveMinimum: true
maximum: 1
`,
		"integer fractional interval contains no integer": `
type: integer
minimum: 1.1
maximum: 1.9
`,
		"integer open unit interval contains no integer": `
type: integer
minimum: 1
exclusiveMinimum: true
maximum: 2
exclusiveMaximum: true
`,
		"exact number is not a multiple": `
type: number
minimum: 1
maximum: 1
multipleOf: 2
`,
		"open interval contains no required multiple": `
type: number
minimum: 0
exclusiveMinimum: true
maximum: 1
exclusiveMaximum: true
multipleOf: 1
`,
		"enum value is below minimum": `
type: number
enum:
  - 1
minimum: 2
`,
		"non-nullable null enum has no numeric value": `
type: number
enum:
  - null
`,
		"integer enum value is not a multiple": `
type: integer
enum:
  - 1
multipleOf: 2
`,
		"exclusiveMinimum must be boolean": `
type: number
minimum: 1
exclusiveMinimum: nope
`,
		"exclusiveMaximum must be boolean": `
type: number
maximum: 1
exclusiveMaximum: nope
`,
		"multipleOf cannot be null": `
type: number
multipleOf: null
`,
		"multipleOf must be a number": `
type: number
multipleOf: nope
`,
		"multipleOf cannot be a quoted number": `
type: number
multipleOf: "1"
`,
		"multipleOf must be positive": `
type: number
multipleOf: 0
`,
		"multipleOf cannot be negative": `
type: number
multipleOf: -2.5
`,
		"format must be string": `
type: number
format: 123
`,
		"integer multipleOf must be positive": `
type: integer
multipleOf: -1
`,
		"integer multipleOf cannot be zero": `
type: integer
multipleOf: 0
`,
		"minLength is not part of NumberDomain": `
type: number
minLength: 1
`,
		"pattern is not part of NumberDomain": `
type: number
pattern: '^[0-9]+$'
`,
		"items is not part of NumberDomain": `
type: number
items:
  type: number
`,
		"properties is not part of NumberDomain": `
type: number
properties: {}
`,
		"additionalProperties is not part of NumberDomain": `
type: number
additionalProperties: false
`,
		"allOf is not part of NumberDomain": `
type: number
allOf: []
`,
		"oneOf must be rejected": `
type: number
oneOf:
  - type: number
`,
		"anyOf must be rejected": `
type: number
anyOf:
  - type: number
`,
		"not must be rejected": `
type: number
not:
  type: string
`,
		"discriminator must be rejected": `
type: number
discriminator:
  propertyName: kind
`,
		"default is unsupported": `
type: number
default: 1
`,
		"readOnly is unsupported": `
type: number
readOnly: true
`,
		"writeOnly is unsupported": `
type: number
writeOnly: true
`,
		"example is unsupported": `
type: number
example: 1
`,
		"deprecated is unsupported": `
type: number
deprecated: true
`,
	}

	for testName, yamlString := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			node := rawObjectFromYAML(t, yamlString)
			dc := Context{domainStore: domainStore{}}
			numberDomain, err := dc.ParseNumber(node)
			require.Error(t, err)
			require.Empty(t, numberDomain)
		})
	}
}

// TestNumberDomainGenerateHashFinalizesConstraints covers programmatic numeric domains.
func TestNumberDomainGenerateHashFinalizesConstraints(t *testing.T) {
	t.Parallel()

	t.Run("canonicalizes equivalent constraint numbers", func(t *testing.T) {
		t.Parallel()

		plain := NumberDomain{
			Type:       "number",
			Minimum:    new(Number("100")),
			Maximum:    new(Number("100")),
			MultipleOf: new(Number("0.5")),
		}
		equivalent := NumberDomain{
			Type:       "number",
			Minimum:    new(Number("1e2")),
			Maximum:    new(Number("100.0")),
			MultipleOf: new(Number("5e-1")),
		}
		before := equivalent

		plainHash, err := plain.GenerateHash()
		require.NoError(t, err)
		equivalentHash, err := equivalent.GenerateHash()
		require.NoError(t, err)
		require.Equal(t, plainHash, equivalentHash)
		require.Equal(t, before, equivalent)
	})

	t.Run("filters enum without mutating the domain", func(t *testing.T) {
		t.Parallel()

		domain := NumberDomain{
			Type:       "number",
			Enum:       []types.Enum{types.Enum("1"), types.Enum("2"), types.Enum("3")},
			Minimum:    new(Number("2")),
			MultipleOf: new(Number("2")),
		}
		before := domain

		got, err := domain.GenerateHash()
		require.NoError(t, err)
		require.Equal(t, before, domain)

		expected, err := generateHash("number", NumberDomain{
			Type:       "number",
			Enum:       []types.Enum{types.Enum("2")},
			Minimum:    new(Number("2")),
			MultipleOf: new(Number("2")),
		})
		require.NoError(t, err)
		require.Equal(t, expected, got)
	})

	t.Run("ignores exclusivity without a corresponding bound", func(t *testing.T) {
		t.Parallel()

		plain := NumberDomain{Type: "number"}
		withInertFlags := NumberDomain{
			Type:             "number",
			ExclusiveMinimum: true,
			ExclusiveMaximum: true,
		}
		before := withInertFlags

		plainHash, err := plain.GenerateHash()
		require.NoError(t, err)
		flaggedHash, err := withInertFlags.GenerateHash()
		require.NoError(t, err)
		require.Equal(t, plainHash, flaggedHash)
		require.Equal(t, before, withInertFlags)

		node := rawObjectFromYAML(t, `
type: number
exclusiveMinimum: true
exclusiveMaximum: true
`)
		parsed, err := new(Context).ParseNumber(node)
		require.NoError(t, err)
		require.Equal(t, plain, parsed)
	})

	t.Run("rejects impossible numeric constraints", func(t *testing.T) {
		t.Parallel()

		tests := map[string]*NumberDomain{
			"empty range": {
				Type:    "number",
				Minimum: new(Number("2")),
				Maximum: new(Number("1")),
			},
			"enum excluded by multiple": {
				Type:       "number",
				Enum:       []types.Enum{types.Enum("1")},
				MultipleOf: new(Number("2")),
			},
			"integer lattice misses range": {
				Type:    "integer",
				Minimum: new(Number("1.1")),
				Maximum: new(Number("1.9")),
			},
		}

		for name, domain := range tests {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				_, err := domain.GenerateHash()
				require.Error(t, err)
			})
		}
	})

	t.Run("nullable permits an empty numeric set", func(t *testing.T) {
		t.Parallel()

		domain := NumberDomain{
			Type:             "number",
			Nullable:         true,
			Minimum:          new(Number("1")),
			Maximum:          new(Number("1")),
			ExclusiveMinimum: true,
		}

		_, err := domain.GenerateHash()
		require.NoError(t, err)
	})
}
