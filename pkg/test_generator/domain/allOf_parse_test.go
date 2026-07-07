package domain

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAllOfParsesValidObjectCompositionWithoutMerging(t *testing.T) {
	firstDomain := &ObjectDomain{AdditionalPropertyKind: AdditionalFalse}
	secondDomain := &ObjectDomain{Nullable: true, AdditionalPropertyKind: AdditionalTrue}
	objectShapedNoTypeDomain := &ObjectDomain{MinProps: 1, AdditionalPropertyKind: AdditionalTrue}
	refTargetDomain := &ObjectDomain{Properties: []Property{{Key: "id", Required: true}}, AdditionalPropertyKind: AdditionalFalse}

	tests := map[string]struct {
		yamlString    string
		parseDomains  []types.Domain
		expected      AllOfDomain
		expectedStore []types.Domain
	}{
		"two object schemas stay as two domains": {
			yamlString: `
allOf:
  - type: object
    required:
      - first
    properties:
      first:
        type: string
  - type: object
    required:
      - second
    properties:
      second:
        type: boolean
`,
			parseDomains:  []types.Domain{firstDomain, secondDomain},
			expected:      AllOfDomain{Domains: []types.Domain{firstDomain, secondDomain}},
			expectedStore: []types.Domain{firstDomain, secondDomain},
		},
		"object shaped allOf item without type is accepted": {
			yamlString: `
allOf:
  - required:
      - id
    properties:
      id:
        type: string
    minProperties: 1
`,
			parseDomains:  []types.Domain{objectShapedNoTypeDomain},
			expected:      AllOfDomain{Domains: []types.Domain{objectShapedNoTypeDomain}},
			expectedStore: []types.Domain{objectShapedNoTypeDomain},
		},
		"ref item is parsed as resolved target domain": {
			yamlString: `
allOf:
  - $ref: '#/components/schemas/BaseThing'
  - type: object
    properties:
      name:
        type: string
`,
			parseDomains:  []types.Domain{refTargetDomain, secondDomain},
			expected:      AllOfDomain{Domains: []types.Domain{refTargetDomain, secondDomain}},
			expectedStore: []types.Domain{refTargetDomain, secondDomain},
		},
		"title and description are allowed documentation fields": {
			yamlString: `
title: Composed Thing
description: A composed object.
allOf:
  - type: object
`,
			parseDomains:  []types.Domain{firstDomain},
			expected:      AllOfDomain{Domains: []types.Domain{firstDomain}},
			expectedStore: []types.Domain{firstDomain},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			parseCall := 0
			dc := DomainContext{domainStore: domainStore{}, parse: func(node *json.RawMessage) (types.Domain, error) {
				require.Less(t, parseCall, len(tt.parseDomains))
				domain := tt.parseDomains[parseCall]
				parseCall++
				return domain, nil
			}}

			node := rawObjectFromYAML(t, tt.yamlString)
			allOfDomain, err := dc.ParseAllOf(node)
			require.NoError(t, err)
			require.Equal(t, len(tt.parseDomains), parseCall)
			require.Equal(t, tt.expected, allOfDomain)
			requireDomainStoreDomains(t, &dc, tt.expectedStore...)
		})
	}
}

func TestParseAllOfRejectsInvalidCompositionSchemas(t *testing.T) {
	tests := map[string]struct {
		yamlString  string
		parseDomain types.Domain
	}{
		"missing allOf": {yamlString: `
type: object
`},
		"allOf cannot be null": {yamlString: `
allOf: null
`},
		"allOf must be array": {yamlString: `
allOf:
  type: object
`},
		"allOf cannot be empty": {yamlString: `
allOf: []
`},
		"allOf item cannot be null": {yamlString: `
allOf:
  - null
`},
		"allOf item cannot be string": {yamlString: `
allOf:
  - nope
`},
		"allOf item cannot be array": {yamlString: `
allOf:
  - []
`},
		"allOf item cannot be any-type empty schema": {yamlString: `
allOf:
  - {}
`},
		"allOf item cannot be type string": {yamlString: `
allOf:
  - type: string
`},
		"allOf item cannot be type number": {yamlString: `
allOf:
  - type: number
`},
		"allOf item cannot be type integer": {yamlString: `
allOf:
  - type: integer
`},
		"allOf item cannot be type boolean": {yamlString: `
allOf:
  - type: boolean
`},
		"allOf item cannot be type array": {yamlString: `
allOf:
  - type: array
    items:
      type: string
`},
		"parsed allOf item must be object domain": {yamlString: `
allOf:
  - type: object
`, parseDomain: &StringDomain{}},
		"top-level oneOf must be rejected": {yamlString: `
allOf:
  - type: object
oneOf:
  - type: object
`},
		"top-level anyOf must be rejected": {yamlString: `
allOf:
  - type: object
anyOf:
  - type: object
`},
		"top-level not must be rejected": {yamlString: `
allOf:
  - type: object
not:
  type: string
`},
		"top-level discriminator must be rejected": {yamlString: `
allOf:
  - type: object
discriminator:
  propertyName: kind
`},
		"sibling type is unsupported until object merging exists": {yamlString: `
type: object
allOf:
  - type: object
`},
		"sibling properties are unsupported until object merging exists": {yamlString: `
allOf:
  - type: object
properties:
  name:
    type: string
`},
		"allOf item oneOf must be rejected": {yamlString: `
allOf:
  - oneOf:
      - type: object
`},
		"allOf item anyOf must be rejected": {yamlString: `
allOf:
  - anyOf:
      - type: object
`},
		"allOf item not must be rejected": {yamlString: `
allOf:
  - not:
      type: string
`},
		"allOf item discriminator must be rejected": {yamlString: `
allOf:
  - type: object
    discriminator:
      propertyName: kind
`},
		"ref with siblings is unsupported": {yamlString: `
allOf:
  - $ref: '#/components/schemas/BaseThing'
    description: ignored by Reference Object
`},
		"spec extension is unsupported": {yamlString: `
allOf:
  - type: object
x-extra: true
`},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			parseDomain := tt.parseDomain
			if parseDomain == nil {
				parseDomain = &ObjectDomain{AdditionalPropertyKind: AdditionalTrue}
			}
			dc := DomainContext{domainStore: domainStore{}, parse: func(node *json.RawMessage) (types.Domain, error) {
				return parseDomain, nil
			}}

			node := rawObjectFromYAML(t, tt.yamlString)
			allOfDomain, err := dc.ParseAllOf(node)
			require.Error(t, err)
			require.Empty(t, allOfDomain)
		})
	}
}

func TestParseAllOfReturnsChildParseErrors(t *testing.T) {
	dc := DomainContext{domainStore: domainStore{}, parse: func(node *json.RawMessage) (types.Domain, error) {
		return nil, errors.New("allOf child parse failed")
	}}

	node := rawObjectFromYAML(t, `
allOf:
  - type: object
`)
	allOfDomain, err := dc.ParseAllOf(node)
	require.Error(t, err)
	require.ErrorContains(t, err, "allOf child parse failed")
	require.Empty(t, allOfDomain)
	require.Empty(t, dc.domainStore)
}
