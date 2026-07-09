package domain

import (
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.

	"github.com/stretchr/testify/require"
)

// TestParseBoolParsesValidBooleanSchemas covers supported boolean constraints.
func TestParseBoolParsesValidBooleanSchemas(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		yamlString string
		expected   BoolDomain
	}{
		"minimal boolean": {
			yamlString: `
type: boolean
`,
			expected: BoolDomain{},
		},
		"title and description are allowed documentation fields": {
			yamlString: `
type: boolean
title: Enabled
description: Whether it is enabled.
`,
			expected: BoolDomain{},
		},
		"specification extension is ignored": {
			yamlString: `
type: boolean
x-extra: true
`,
			expected: BoolDomain{},
		},
		"nullable true": {
			yamlString: `
type: boolean
nullable: true
`,
			expected: BoolDomain{Nullable: true},
		},
		"nullable false": {
			yamlString: `
type: boolean
nullable: false
`,
			expected: BoolDomain{},
		},
		"enum booleans": {
			yamlString: `
type: boolean
enum:
  - true
  - false
`,
			expected: BoolDomain{Enum: []types.Enum{types.Enum("false"), types.Enum("true")}},
		},
		"enum filters incompatible values and duplicates": {
			yamlString: `
type: boolean
enum:
  - true
  - "true"
  - null
  - true
  - 1
`,
			expected: BoolDomain{Enum: []types.Enum{types.Enum("true")}},
		},
		"nullable enum retains null": {
			yamlString: `
type: boolean
nullable: true
enum:
  - true
  - null
`,
			expected: BoolDomain{Nullable: true, Enum: []types.Enum{types.Enum("null"), types.Enum("true")}},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			node := rawObjectFromYAML(t, tt.yamlString)
			dc := Context{domainStore: domainStore{}}
			boolDomain, err := dc.ParseBool(node)
			require.NoError(t, err)
			require.Equal(t, tt.expected, boolDomain)
		})
	}
}

// TestParseBoolRejectsInvalidBooleanSchemas covers malformed and unsupported fields.
func TestParseBoolRejectsInvalidBooleanSchemas(t *testing.T) {
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
  - boolean
  - string
`,
		"nullable must be boolean": `
type: boolean
nullable: nope
`,
		"title must be string": `
type: boolean
title: 123
`,
		"enum cannot be empty": `
type: boolean
enum: []
`,
		"enum cannot be null": `
type: boolean
enum: null
`,
		"enum must be array": `
type: boolean
enum: true
`,
		"enum must contain a compatible value": `
type: boolean
enum:
  - null
  - "true"
  - 1
`,
		"minimum is not part of BoolDomain": `
type: boolean
minimum: 1
`,
		"maximum is not part of BoolDomain": `
type: boolean
maximum: 1
`,
		"multipleOf is not part of BoolDomain": `
type: boolean
multipleOf: 2
`,
		"minLength is not part of BoolDomain": `
type: boolean
minLength: 1
`,
		"pattern is not part of BoolDomain": `
type: boolean
pattern: '^true$'
`,
		"format is not part of BoolDomain": `
type: boolean
format: flag
`,
		"items is not part of BoolDomain": `
type: boolean
items:
  type: string
`,
		"properties is not part of BoolDomain": `
type: boolean
properties: {}
`,
		"additionalProperties is not part of BoolDomain": `
type: boolean
additionalProperties: false
`,
		"allOf is not part of BoolDomain": `
type: boolean
allOf: []
`,
		"oneOf must be rejected": `
type: boolean
oneOf:
  - type: boolean
`,
		"anyOf must be rejected": `
type: boolean
anyOf:
  - type: boolean
`,
		"not must be rejected": `
type: boolean
not:
  type: string
`,
		"discriminator must be rejected": `
type: boolean
discriminator:
  propertyName: kind
`,
		"default is unsupported": `
type: boolean
default: true
`,
		"readOnly is unsupported": `
type: boolean
readOnly: true
`,
		"writeOnly is unsupported": `
type: boolean
writeOnly: true
`,
		"example is unsupported": `
type: boolean
example: true
`,
		"deprecated is unsupported": `
type: boolean
deprecated: true
`,
	}

	for testName, yamlString := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			node := rawObjectFromYAML(t, yamlString)
			dc := Context{domainStore: domainStore{}}
			boolDomain, err := dc.ParseBool(node)
			require.Error(t, err)
			require.Empty(t, boolDomain)
		})
	}
}
