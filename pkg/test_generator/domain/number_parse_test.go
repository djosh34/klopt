package domain

import (
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types"

	"github.com/stretchr/testify/require"
)

func TestParseNumberParsesValidNumberSchemas(t *testing.T) {
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
			expected: NumberDomain{Type: "number", Enum: []types.Enum{types.Enum("1"), types.Enum("2.5")}},
		},
		"minimum maximum and exclusive bounds": {
			yamlString: `
type: number
minimum: 1.5
exclusiveMinimum: true
maximum: 9.5
exclusiveMaximum: true
`,
			expected: NumberDomain{Type: "number", Minimum: new(Number("1.5")), Maximum: new(Number("9.5")), ExclusiveMinimum: true, ExclusiveMaximum: true},
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
		"integer int32": {
			yamlString: `
type: integer
format: int32
enum:
  - 1
minimum: -10
maximum: 10
multipleOf: 2
`,
			expected: NumberDomain{Type: "integer", Enum: []types.Enum{types.Enum("1")}, Minimum: new(Number("-10")), Maximum: new(Number("10")), MultipleOf: new(Number("2")), Format: new("int32")},
		},
		"integer int64": {
			yamlString: `
type: integer
format: int64
`,
			expected: NumberDomain{Type: "integer", Format: new("int64")},
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
			expected: NumberDomain{Type: "number", Nullable: true, Enum: []types.Enum{types.Enum("2.5")}, Minimum: new(Number("0.5")), Maximum: new(Number("10.5")), ExclusiveMinimum: true, MultipleOf: new(Number("2.5")), Format: new("double")},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			node := rawObjectFromYAML(t, tt.yamlString)
			dc := DomainContext{domainStore: domainStore{}}
			numberDomain, err := dc.ParseNumber(node)
			require.NoError(t, err)
			require.Equal(t, tt.expected, numberDomain)
		})
	}
}

func TestParseNumberRejectsInvalidNumberSchemas(t *testing.T) {
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
		"minimum cannot be null": `
type: number
minimum: null
`,
		"minimum must be a number": `
type: number
minimum: nope
`,
		"maximum cannot be null": `
type: number
maximum: null
`,
		"maximum must be a number": `
type: number
maximum: nope
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
		"unknown format is unsupported": `
type: number
format: decimal
`,
		"integer format is not number format": `
type: number
format: int32
`,
		"number format is not integer format": `
type: integer
format: float
`,
		"integer minimum must be integer": `
type: integer
minimum: 1.5
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
		"spec extension is unsupported": `
type: number
x-extra: 1
`,
	}

	for testName, yamlString := range tests {
		t.Run(testName, func(t *testing.T) {
			node := rawObjectFromYAML(t, yamlString)
			dc := DomainContext{domainStore: domainStore{}}
			numberDomain, err := dc.ParseNumber(node)
			require.Error(t, err)
			require.Empty(t, numberDomain)
		})
	}
}
