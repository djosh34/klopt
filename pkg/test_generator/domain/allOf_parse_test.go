package domain

import (
	"encoding/json"
	"errors"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types"

	"github.com/stretchr/testify/require"
)

func TestParseAllOfParsesAndMergesValidCompositionSchemas(t *testing.T) {
	firstObjectDomain := &ObjectDomain{Properties: []Property{{Key: "first", Required: true}}, AdditionalPropertyKind: AdditionalTrue}
	secondObjectDomain := &ObjectDomain{Nullable: true, Properties: []Property{{Key: "second", Required: true}}, AdditionalPropertyKind: AdditionalTrue}
	objectShapedNoTypeDomain := &ObjectDomain{MinProps: 1, AdditionalPropertyKind: AdditionalTrue}
	refTargetDomain := &ObjectDomain{Properties: []Property{{Key: "id", Required: true}}, AdditionalPropertyKind: AdditionalFalse}
	refCompanionDomain := &ObjectDomain{Properties: []Property{{Key: "name"}}, AdditionalPropertyKind: AdditionalTrue}
	siblingObjectDomain := &ObjectDomain{Properties: []Property{{Key: "name", Domain: &StringDomain{}}}, AdditionalPropertyKind: AdditionalTrue}

	tests := map[string]struct {
		yamlString    string
		parseDomains  []types.Domain
		expected      AllOfDomain
		expectedStore []types.Domain
	}{
		"two object schemas merge into one merged domain": {
			yamlString: `
allOf:
  - type: object
    required:
      - first
  - type: object
    required:
      - second
`,
			parseDomains: []types.Domain{firstObjectDomain, secondObjectDomain},
			expected: AllOfDomain{
				Domains: []types.Domain{firstObjectDomain, secondObjectDomain},
				MergedDomain: &ObjectDomain{
					Nullable:               false,
					Properties:             []Property{{Key: "first", Required: true}, {Key: "second", Required: true}},
					AdditionalPropertyKind: AdditionalTrue,
				},
			},
			expectedStore: []types.Domain{firstObjectDomain, secondObjectDomain},
		},
		"string allOf item is accepted": {
			yamlString: `
allOf:
  - type: string
    minLength: 1
  - type: string
    maxLength: 5
`,
			parseDomains: []types.Domain{&StringDomain{MinLength: 1}, &StringDomain{MaxLength: new(5)}},
			expected: AllOfDomain{
				Domains:      []types.Domain{&StringDomain{MinLength: 1}, &StringDomain{MaxLength: new(5)}},
				MergedDomain: &StringDomain{MinLength: 1, MaxLength: new(5)},
			},
			expectedStore: []types.Domain{&StringDomain{MinLength: 1}, &StringDomain{MaxLength: new(5)}},
		},
		"number and integer allOf items are accepted": {
			yamlString: `
allOf:
  - type: number
  - type: integer
`,
			parseDomains: []types.Domain{&NumberDomain{Type: "number"}, &NumberDomain{Type: "integer"}},
			expected: AllOfDomain{
				Domains:      []types.Domain{&NumberDomain{Type: "number"}, &NumberDomain{Type: "integer"}},
				MergedDomain: &NumberDomain{Type: "integer"},
			},
			expectedStore: []types.Domain{&NumberDomain{Type: "number"}, &NumberDomain{Type: "integer"}},
		},
		"boolean allOf items are accepted": {
			yamlString: `
allOf:
  - type: boolean
    enum:
      - true
      - false
  - type: boolean
    enum:
      - false
`,
			parseDomains: []types.Domain{&BoolDomain{Enum: []types.Enum{types.Enum("true"), types.Enum("false")}}, &BoolDomain{Enum: []types.Enum{types.Enum("false")}}},
			expected: AllOfDomain{
				Domains:      []types.Domain{&BoolDomain{Enum: []types.Enum{types.Enum("true"), types.Enum("false")}}, &BoolDomain{Enum: []types.Enum{types.Enum("false")}}},
				MergedDomain: &BoolDomain{Enum: []types.Enum{types.Enum("false")}},
			},
			expectedStore: []types.Domain{&BoolDomain{Enum: []types.Enum{types.Enum("true"), types.Enum("false")}}, &BoolDomain{Enum: []types.Enum{types.Enum("false")}}},
		},
		"array allOf items are accepted": {
			yamlString: `
allOf:
  - type: array
    items: {}
    minItems: 1
  - type: array
    items: {}
    maxItems: 3
`,
			parseDomains: []types.Domain{&ArrayDomain{MinItems: 1}, &ArrayDomain{MaxItems: new(3)}},
			expected: AllOfDomain{
				Domains:      []types.Domain{&ArrayDomain{MinItems: 1}, &ArrayDomain{MaxItems: new(3)}},
				MergedDomain: &ArrayDomain{MinItems: 1, MaxItems: new(3)},
			},
			expectedStore: []types.Domain{&ArrayDomain{MinItems: 1}, &ArrayDomain{MaxItems: new(3)}},
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
			parseDomains: []types.Domain{objectShapedNoTypeDomain},
			expected: AllOfDomain{
				Domains:      []types.Domain{objectShapedNoTypeDomain},
				MergedDomain: objectShapedNoTypeDomain,
			},
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
			parseDomains: []types.Domain{refTargetDomain, refCompanionDomain},
			expected: AllOfDomain{
				Domains: []types.Domain{refTargetDomain, refCompanionDomain},
				MergedDomain: &ObjectDomain{
					Properties:             []Property{{Key: "id", Required: true}},
					AdditionalPropertyKind: AdditionalFalse,
				},
			},
			expectedStore: []types.Domain{refTargetDomain, refCompanionDomain},
		},
		"title and description are allowed documentation fields": {
			yamlString: `
title: Composed Thing
description: A composed object.
allOf:
  - type: object
`,
			parseDomains: []types.Domain{firstObjectDomain},
			expected: AllOfDomain{
				Domains:      []types.Domain{firstObjectDomain},
				MergedDomain: firstObjectDomain,
			},
			expectedStore: []types.Domain{firstObjectDomain},
		},
		"sibling object constraints are merged after allOf children": {
			yamlString: `
type: object
allOf:
  - type: object
    required:
      - first
properties:
  name:
    type: string
`,
			parseDomains: []types.Domain{firstObjectDomain, siblingObjectDomain},
			expected: AllOfDomain{
				Domains: []types.Domain{firstObjectDomain, siblingObjectDomain},
				MergedDomain: &ObjectDomain{
					Properties:             []Property{{Key: "first", Required: true}, {Key: "name", Domain: &StringDomain{}}},
					AdditionalPropertyKind: AdditionalTrue,
				},
			},
			expectedStore: []types.Domain{firstObjectDomain, siblingObjectDomain},
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
		"parsed allOf item cannot be nil": {yamlString: `
allOf:
  - type: object
`, parseDomain: nil},
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
			if parseDomain == nil && testName != "parsed allOf item cannot be nil" {
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
