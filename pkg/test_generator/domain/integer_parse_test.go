package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type parseIntegerer interface {
	ParseInteger(node *json.RawMessage) (IntegerDomain, error)
}

func parseIntegerForTest(t *testing.T, yamlString string) (IntegerDomain, error) {
	t.Helper()

	node := rawObjectFromYAML(t, yamlString)
	dc := DomainContext{domainStore: domainStore{}}
	parser, ok := any(&dc).(parseIntegerer)
	require.True(t, ok, "DomainContext.ParseInteger must exist")
	if !ok {
		return IntegerDomain{}, nil
	}

	return parser.ParseInteger(node)
}

func TestParseIntegerParsesValidIntegerSchemas(t *testing.T) {
	tests := map[string]struct {
		yamlString string
		expected   IntegerDomain
	}{
		"minimal integer": {
			yamlString: `
type: integer
`,
			expected: IntegerDomain{},
		},
		"title and description are allowed documentation fields": {
			yamlString: `
type: integer
title: Count
description: Number of things.
`,
			expected: IntegerDomain{},
		},
		"nullable true": {
			yamlString: `
type: integer
nullable: true
`,
			expected: IntegerDomain{Nullable: true},
		},
		"nullable false": {
			yamlString: `
type: integer
nullable: false
`,
			expected: IntegerDomain{},
		},
		"enum integers": {
			yamlString: `
type: integer
enum:
  - 1
  - 2
`,
			expected: IntegerDomain{Enum: []int64{1, 2}},
		},
		"minimum maximum and exclusive bounds": {
			yamlString: `
type: integer
minimum: 1
exclusiveMinimum: true
maximum: 9
exclusiveMaximum: true
`,
			expected: IntegerDomain{Minimum: new(int64(1)), Maximum: new(int64(9)), ExclusiveMinimum: true, ExclusiveMaximum: true},
		},
		"multipleOf": {
			yamlString: `
type: integer
multipleOf: 2
`,
			expected: IntegerDomain{MultipleOf: new(int64(2))},
		},
		"format int32": {
			yamlString: `
type: integer
format: int32
`,
			expected: IntegerDomain{Format: new("int32")},
		},
		"format int64": {
			yamlString: `
type: integer
format: int64
`,
			expected: IntegerDomain{Format: new("int64")},
		},
		"int32 boundary values": {
			yamlString: `
type: integer
format: int32
minimum: -2147483648
maximum: 2147483647
`,
			expected: IntegerDomain{Minimum: new(int64(-2147483648)), Maximum: new(int64(2147483647)), Format: new("int32")},
		},
		"all supported fields together": {
			yamlString: `
type: integer
nullable: true
enum:
  - 4
minimum: 0
exclusiveMinimum: true
maximum: 10
exclusiveMaximum: false
multipleOf: 2
format: int64
`,
			expected: IntegerDomain{Nullable: true, Enum: []int64{4}, Minimum: new(int64(0)), Maximum: new(int64(10)), ExclusiveMinimum: true, MultipleOf: new(int64(2)), Format: new("int64")},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			integerDomain, err := parseIntegerForTest(t, tt.yamlString)
			require.NoError(t, err)
			require.Equal(t, tt.expected, integerDomain)
		})
	}
}

func TestParseIntegerRejectsInvalidIntegerSchemas(t *testing.T) {
	tests := map[string]string{
		"missing type": `
nullable: false
`,
		"wrong type": `
type: number
`,
		"mixed type array": `
type:
  - integer
  - string
`,
		"nullable must be boolean": `
type: integer
nullable: nope
`,
		"enum cannot be empty": `
type: integer
enum: []
`,
		"enum cannot be null": `
type: integer
enum: null
`,
		"enum must be array": `
type: integer
enum: 1
`,
		"enum values must be integers": `
type: integer
enum:
  - 1
  - 1.5
`,
		"enum values must be unique": `
type: integer
enum:
  - 1
  - 1
`,
		"enum values must fit int32 format": `
type: integer
format: int32
enum:
  - 2147483648
`,
		"minimum cannot be null": `
type: integer
minimum: null
`,
		"minimum must be an integer": `
type: integer
minimum: 1.5
`,
		"maximum cannot be null": `
type: integer
maximum: null
`,
		"maximum must be an integer": `
type: integer
maximum: 1.5
`,
		"minimum cannot exceed maximum": `
type: integer
minimum: 2
maximum: 1
`,
		"exclusive equal bounds are impossible": `
type: integer
minimum: 1
exclusiveMinimum: true
maximum: 1
`,
		"exclusiveMinimum must be boolean": `
type: integer
minimum: 1
exclusiveMinimum: nope
`,
		"exclusiveMaximum must be boolean": `
type: integer
maximum: 1
exclusiveMaximum: nope
`,
		"minimum must fit int32 format": `
type: integer
format: int32
minimum: -2147483649
`,
		"maximum must fit int32 format": `
type: integer
format: int32
maximum: 2147483648
`,
		"multipleOf cannot be null": `
type: integer
multipleOf: null
`,
		"multipleOf must be an integer": `
type: integer
multipleOf: 1.5
`,
		"multipleOf must be positive": `
type: integer
multipleOf: 0
`,
		"multipleOf cannot be negative": `
type: integer
multipleOf: -2
`,
		"format must be string": `
type: integer
format: 123
`,
		"unknown format is unsupported": `
type: integer
format: uint32
`,
		"number format is not integer format": `
type: integer
format: float
`,
		"minLength is not part of IntegerDomain": `
type: integer
minLength: 1
`,
		"pattern is not part of IntegerDomain": `
type: integer
pattern: '^[0-9]+$'
`,
		"items is not part of IntegerDomain": `
type: integer
items:
  type: integer
`,
		"properties is not part of IntegerDomain": `
type: integer
properties: {}
`,
		"additionalProperties is not part of IntegerDomain": `
type: integer
additionalProperties: false
`,
		"allOf is not part of IntegerDomain": `
type: integer
allOf: []
`,
		"oneOf must be rejected": `
type: integer
oneOf:
  - type: integer
`,
		"anyOf must be rejected": `
type: integer
anyOf:
  - type: integer
`,
		"not must be rejected": `
type: integer
not:
  type: string
`,
		"discriminator must be rejected": `
type: integer
discriminator:
  propertyName: kind
`,
		"default is unsupported": `
type: integer
default: 1
`,
		"readOnly is unsupported": `
type: integer
readOnly: true
`,
		"writeOnly is unsupported": `
type: integer
writeOnly: true
`,
		"example is unsupported": `
type: integer
example: 1
`,
		"deprecated is unsupported": `
type: integer
deprecated: true
`,
		"spec extension is unsupported": `
type: integer
x-extra: 1
`,
	}

	for testName, yamlString := range tests {
		t.Run(testName, func(t *testing.T) {
			integerDomain, err := parseIntegerForTest(t, yamlString)
			require.Error(t, err)
			require.Empty(t, integerDomain)
		})
	}
}
