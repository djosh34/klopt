package domain

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBoolParsesValidBooleanSchemas(t *testing.T) {
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
			expected: BoolDomain{Enum: []types.Enum{types.Enum("true"), types.Enum("false")}},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			node := rawObjectFromYAML(t, tt.yamlString)
			dc := DomainContext{domainStore: domainStore{}}
			boolDomain, err := dc.ParseBool(node)
			require.NoError(t, err)
			require.Equal(t, tt.expected, boolDomain)
		})
	}
}

func TestParseBoolRejectsInvalidBooleanSchemas(t *testing.T) {
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
		"spec extension is unsupported": `
type: boolean
x-extra: true
`,
	}

	for testName, yamlString := range tests {
		t.Run(testName, func(t *testing.T) {
			node := rawObjectFromYAML(t, yamlString)
			dc := DomainContext{domainStore: domainStore{}}
			boolDomain, err := dc.ParseBool(node)
			require.Error(t, err)
			require.Empty(t, boolDomain)
		})
	}
}
