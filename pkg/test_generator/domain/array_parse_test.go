package domain

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseArrayParsesValidArraySchemas(t *testing.T) {
	stringItemsDomain := &StringDomain{}
	numberItemsDomain := &NumberDomain{}
	refTargetDomain := &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}

	tests := map[string]struct {
		yamlString         string
		parseDomain        types.Domain
		expected           ArrayDomain
		expectedStore      []types.Domain
		expectedParseCalls int
	}{
		"minimal array": {
			yamlString: `
type: array
items:
  type: string
`,
			parseDomain:   stringItemsDomain,
			expected:      ArrayDomain{Items: stringItemsDomain},
			expectedStore: []types.Domain{stringItemsDomain},
		},
		"title and description are allowed documentation fields": {
			yamlString: `
type: array
title: Tags
description: A list of tags.
items:
  type: string
`,
			parseDomain:   stringItemsDomain,
			expected:      ArrayDomain{Items: stringItemsDomain},
			expectedStore: []types.Domain{stringItemsDomain},
		},
		"nullable true": {
			yamlString: `
type: array
nullable: true
items:
  type: string
`,
			parseDomain:   stringItemsDomain,
			expected:      ArrayDomain{Nullable: true, Items: stringItemsDomain},
			expectedStore: []types.Domain{stringItemsDomain},
		},
		"nullable false": {
			yamlString: `
type: array
nullable: false
items:
  type: string
`,
			parseDomain:   stringItemsDomain,
			expected:      ArrayDomain{Items: stringItemsDomain},
			expectedStore: []types.Domain{stringItemsDomain},
		},
		"minItems and maxItems": {
			yamlString: `
type: array
items:
  type: number
minItems: 1
maxItems: 3
`,
			parseDomain:   numberItemsDomain,
			expected:      ArrayDomain{Items: numberItemsDomain, MinItems: 1, MaxItems: new(3)},
			expectedStore: []types.Domain{numberItemsDomain},
		},
		"items ref is parsed as resolved target domain": {
			yamlString: `
type: array
items:
  $ref: '#/components/schemas/Thing'
`,
			parseDomain:   refTargetDomain,
			expected:      ArrayDomain{Items: refTargetDomain},
			expectedStore: []types.Domain{refTargetDomain},
		},
		"items empty object is arbitrary item schema": {
			yamlString: `
type: array
items: {}
`,
			expected: ArrayDomain{},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			parseCall := 0
			dc := DomainContext{domainStore: domainStore{}, parse: func(node *json.RawMessage) (types.Domain, error) {
				parseCall++
				return tt.parseDomain, nil
			}}

			expectedParseCalls := tt.expectedParseCalls
			if tt.parseDomain != nil {
				expectedParseCalls = 1
			}

			node := rawObjectFromYAML(t, tt.yamlString)
			arrayDomain, err := dc.ParseArray(node)
			require.NoError(t, err)
			require.Equal(t, expectedParseCalls, parseCall)
			require.Equal(t, tt.expected, arrayDomain)
			requireDomainStoreDomains(t, &dc, tt.expectedStore...)
		})
	}
}

func TestParseArrayParsesEnum(t *testing.T) {
	node := rawObjectFromYAML(t, `
type: array
items:
  type: string
enum:
  - [alpha]
  - [beta]
`)

	dc := DomainContext{domainStore: domainStore{}, parse: func(node *json.RawMessage) (types.Domain, error) {
		return &StringDomain{}, nil
	}}

	arrayDomain, err := dc.ParseArray(node)
	require.NoError(t, err)
	require.Len(t, arrayDomain.Enum, 2)
	require.Len(t, dc.domainStore, 3)

	for _, enumDomain := range arrayDomain.Enum {
		require.Contains(t, dc.domainStore, enumDomain)
		require.IsType(t, new(EnumDomain), enumDomain)
	}
}

func TestParseArrayRejectsInvalidArraySchemas(t *testing.T) {
	tests := map[string]string{
		"missing type": `
items:
  type: string
`,
		"wrong type": `
type: object
items:
  type: string
`,
		"mixed type array": `
type:
  - array
  - string
items:
  type: string
`,
		"nullable must be boolean": `
type: array
nullable: nope
items:
  type: string
`,
		"items is required": `
type: array
`,
		"enum cannot be null": `
type: array
items:
  type: string
enum: null
`,
		"enum cannot be empty": `
type: array
items:
  type: string
enum: []
`,
		"enum must be array": `
type: array
items:
  type: string
enum: alpha
`,
		"items cannot be null": `
type: array
items: null
`,
		"items cannot be an array": `
type: array
items:
  - type: string
`,
		"uniqueItems true is unsupported": `
type: array
items:
  type: string
uniqueItems: true
`,
		"uniqueItems false is unsupported": `
type: array
items:
  type: string
uniqueItems: false
`,
		"minItems cannot be null": `
type: array
items:
  type: string
minItems: null
`,
		"minItems cannot be negative": `
type: array
items:
  type: string
minItems: -1
`,
		"minItems must be an integer": `
type: array
items:
  type: string
minItems: 1.5
`,
		"maxItems cannot be null": `
type: array
items:
  type: string
maxItems: null
`,
		"maxItems cannot be negative": `
type: array
items:
  type: string
maxItems: -1
`,
		"maxItems must be an integer": `
type: array
items:
  type: string
maxItems: 1.5
`,
		"minItems cannot exceed maxItems": `
type: array
items:
  type: string
minItems: 3
maxItems: 2
`,
		"minimum is not part of ArrayDomain": `
type: array
items:
  type: string
minimum: 1
`,
		"maxLength is not part of ArrayDomain": `
type: array
items:
  type: string
maxLength: 10
`,
		"pattern is not part of ArrayDomain": `
type: array
items:
  type: string
pattern: '^x$'
`,
		"format is not part of ArrayDomain": `
type: array
items:
  type: string
format: csv
`,
		"properties is not part of ArrayDomain": `
type: array
items:
  type: string
properties: {}
`,
		"required is not part of ArrayDomain": `
type: array
items:
  type: string
required:
  - name
`,
		"additionalProperties is not part of ArrayDomain": `
type: array
items:
  type: string
additionalProperties: false
`,
		"allOf is not part of ArrayDomain": `
type: array
items:
  type: string
allOf: []
`,
		"oneOf must be rejected": `
type: array
items:
  type: string
oneOf:
  - type: array
    items:
      type: string
`,
		"anyOf must be rejected": `
type: array
items:
  type: string
anyOf:
  - type: array
    items:
      type: string
`,
		"not must be rejected": `
type: array
items:
  type: string
not:
  type: string
`,
		"discriminator must be rejected": `
type: array
items:
  type: string
discriminator:
  propertyName: kind
`,
		"default is unsupported": `
type: array
items:
  type: string
default: []
`,
		"readOnly is unsupported": `
type: array
items:
  type: string
readOnly: true
`,
		"writeOnly is unsupported": `
type: array
items:
  type: string
writeOnly: true
`,
		"example is unsupported": `
type: array
items:
  type: string
example: []
`,
		"deprecated is unsupported": `
type: array
items:
  type: string
deprecated: true
`,
		"spec extension is unsupported": `
type: array
items:
  type: string
x-extra: true
`,
	}

	for testName, yamlString := range tests {
		t.Run(testName, func(t *testing.T) {
			dc := DomainContext{domainStore: domainStore{}, parse: func(node *json.RawMessage) (types.Domain, error) {
				return &StringDomain{}, nil
			}}

			node := rawObjectFromYAML(t, yamlString)
			arrayDomain, err := dc.ParseArray(node)
			require.Error(t, err)
			require.Empty(t, arrayDomain)
		})
	}
}

func TestParseArrayReturnsItemParseErrors(t *testing.T) {
	dc := DomainContext{domainStore: domainStore{}, parse: func(node *json.RawMessage) (types.Domain, error) {
		return nil, errors.New("item parse failed")
	}}

	node := rawObjectFromYAML(t, `
type: array
items:
  type: string
`)
	arrayDomain, err := dc.ParseArray(node)
	require.Error(t, err)
	require.ErrorContains(t, err, "item parse failed")
	require.Empty(t, arrayDomain)
	require.Empty(t, dc.domainStore)
}
