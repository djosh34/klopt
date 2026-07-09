package domain

import (
	"encoding/json"
	"errors"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.

	"github.com/stretchr/testify/require"
)

// TestParseArrayParsesValidArraySchemas covers supported array constraints.
func TestParseArrayParsesValidArraySchemas(t *testing.T) {
	t.Parallel()

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
		"specification extensions are ignored": {
			yamlString: `
type: array
x-internal-metadata:
  enabled: true
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
		"enum is filtered by item-count bounds": {
			yamlString: `
type: array
items:
  type: string
enum:
  - []
  - [alpha]
  - [alpha, beta]
  - [alpha, beta, gamma]
minItems: 1
maxItems: 2
`,
			parseDomain: stringItemsDomain,
			expected: ArrayDomain{
				Enum:     []types.Enum{types.Enum(`["alpha","beta"]`), types.Enum(`["alpha"]`)},
				Items:    stringItemsDomain,
				MinItems: 1,
				MaxItems: new(2),
			},
			expectedStore: []types.Domain{stringItemsDomain},
		},
		"nullable permits contradictory item-count bounds": {
			yamlString: `
type: array
nullable: true
items: {}
minItems: 3
maxItems: 2
`,
			expected: ArrayDomain{Nullable: true, MinItems: 3, MaxItems: new(2)},
		},
		"nullable enum null permits contradictory item-count bounds": {
			yamlString: `
type: array
nullable: true
items: {}
enum:
  - [alpha]
  - null
minItems: 2
maxItems: 1
`,
			expected: ArrayDomain{
				Nullable: true,
				Enum:     []types.Enum{types.Enum(`null`)},
				MinItems: 2,
				MaxItems: new(1),
			},
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
			t.Parallel()

			parseCall := 0
			dc := Context{domainStore: domainStore{}, parse: func(_ *json.RawMessage) (types.Domain, error) {
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

// TestParseArrayParsesEnum checks canonical array enum parsing.
func TestParseArrayParsesEnum(t *testing.T) {
	t.Parallel()

	node := rawObjectFromYAML(t, `
type: array
items:
  type: string
enum:
  - [beta]
  - null
  - {alpha: beta}
  - [alpha]
  - [alpha]
`)

	dc := Context{domainStore: domainStore{}, parse: func(_ *json.RawMessage) (types.Domain, error) {
		return &StringDomain{}, nil
	}}

	arrayDomain, err := dc.ParseArray(node)
	require.NoError(t, err)
	require.Equal(t, []types.Enum{types.Enum(`["alpha"]`), types.Enum(`["beta"]`)}, arrayDomain.Enum)
	require.Len(t, dc.domainStore, 1)
}

// TestParseArrayRejectsInvalidArraySchemas covers malformed and unsupported fields.
func TestParseArrayRejectsInvalidArraySchemas(t *testing.T) {
	t.Parallel()

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
		"enum must contain a compatible value": `
type: array
items:
  type: string
enum:
  - null
  - {alpha: beta}
  - string
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
		"enum must satisfy item-count bounds": `
type: array
items:
  type: string
enum:
  - []
minItems: 1
`,
		"nullable enum without null cannot rescue contradictory bounds": `
type: array
nullable: true
items:
  type: string
enum:
  - [alpha]
minItems: 2
maxItems: 1
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
	}

	for testName, yamlString := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			dc := Context{domainStore: domainStore{}, parse: func(_ *json.RawMessage) (types.Domain, error) {
				return &StringDomain{}, nil
			}}

			node := rawObjectFromYAML(t, yamlString)
			arrayDomain, err := dc.ParseArray(node)
			require.Error(t, err)
			require.Empty(t, arrayDomain)
		})
	}
}

// TestParseArrayUnsupportedFieldErrorIsDeterministic checks lexical unsupported-key selection.
func TestParseArrayUnsupportedFieldErrorIsDeterministic(t *testing.T) {
	t.Parallel()

	node := rawObjectFromYAML(t, `
type: array
items: {}
z-unsupported: true
a-unsupported: true
`)

	_, err := (&Context{domainStore: domainStore{}}).ParseArray(node)
	require.EqualError(t, err, `unsupported array schema field "a-unsupported"`)
}

// TestParseArrayReturnsItemParseErrors propagates nested item failures.
func TestParseArrayReturnsItemParseErrors(t *testing.T) {
	t.Parallel()

	dc := Context{domainStore: domainStore{}, parse: func(_ *json.RawMessage) (types.Domain, error) {
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
