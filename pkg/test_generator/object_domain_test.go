package testgenerator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeObjectTestDomain struct{}

func (f fakeObjectTestDomain) GenerateHash() (Hash, error) {
	return Hash{}, nil
}

func rawObjectFromYAML(t *testing.T, yamlString string) *json.RawMessage {
	t.Helper()

	node, err := YAMLBytesToJSONRawMessage([]byte(yamlString))
	require.NoError(t, err)
	return node
}

func requireDomainStoreKeys(t *testing.T, dc *DomainContext, expectedKeys ...Hash) {
	t.Helper()

	require.Len(t, dc.domainStore, len(expectedKeys))
	for _, expectedKey := range expectedKeys {
		require.Contains(t, dc.domainStore, expectedKey)
	}
}

func TestParseObjectParsesValidObjectSchemas(t *testing.T) {
	propertyNameHash := Hash{1}
	propertyAgeHash := Hash{2}
	additionalPropertyHash := Hash{3}

	tests := map[string]struct {
		yamlString       string
		parseHashes      []Hash
		expectedStoreKey []Hash
		expected         ObjectDomain
	}{
		"empty object schema defaults additionalProperties to true": {
			yamlString: `
type: object
`,
			expected: ObjectDomain{
				AdditionalPropertyKind: AdditionalTrue,
			},
		},
		"title and description are allowed documentation fields": {
			yamlString: `
type: object
title: Person
description: A person object.
`,
			expected: ObjectDomain{
				AdditionalPropertyKind: AdditionalTrue,
			},
		},
		"nullable is allowed": {
			yamlString: `
type: object
nullable: true
`,
			expected: ObjectDomain{
				AdditionalPropertyKind: AdditionalTrue,
			},
		},
		"properties are parsed and sorted by property key": {
			yamlString: `
type: object
properties:
  name:
    type: string
  age:
    type: integer
`,
			parseHashes:      []Hash{propertyNameHash, propertyAgeHash},
			expectedStoreKey: []Hash{propertyNameHash, propertyAgeHash},
			expected: ObjectDomain{
				Properties:             []*Hash{&propertyAgeHash, &propertyNameHash},
				AdditionalPropertyKind: AdditionalTrue,
			},
		},
		"required property keeps required marker in property domain": {
			yamlString: `
type: object
required:
  - name
properties:
  name:
    type: string
`,
			parseHashes:      []Hash{propertyNameHash},
			expectedStoreKey: []Hash{propertyNameHash},
			expected: ObjectDomain{
				Properties:             []*Hash{&propertyNameHash},
				AdditionalPropertyKind: AdditionalTrue,
			},
		},
		"additionalProperties false": {
			yamlString: `
type: object
additionalProperties: false
`,
			expected: ObjectDomain{
				AdditionalPropertyKind: AdditionalFalse,
			},
		},
		"additionalProperties schema": {
			yamlString: `
type: object
additionalProperties:
  type: string
`,
			parseHashes:      []Hash{additionalPropertyHash},
			expectedStoreKey: []Hash{additionalPropertyHash},
			expected: ObjectDomain{
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: &additionalPropertyHash,
			},
		},
		"minProperties and maxProperties": {
			yamlString: `
type: object
minProperties: 1
maxProperties: 3
`,
			expected: ObjectDomain{
				AdditionalPropertyKind: AdditionalTrue,
				MinProps:               1,
				MaxProps:               new(3),
			},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			node := rawObjectFromYAML(t, tt.yamlString)
			parseCall := 0
			dc := DomainContext{
				domainStore: map[Hash]Domain{},
				parse: func(node *json.RawMessage) (*Hash, error) {
					require.Less(t, parseCall, len(tt.parseHashes))
					hash := tt.parseHashes[parseCall]
					parseCall++
					return &hash, nil
				},
			}

			objectDomain, err := dc.ParseObject(node)
			require.NoError(t, err)
			require.Equal(t, len(tt.parseHashes), parseCall)
			requireDomainStoreKeys(t, &dc, tt.expectedStoreKey...)
			require.Equal(t, tt.expected, objectDomain)
		})
	}
}

func TestParseObjectParsesEnumAndReturnsEarly(t *testing.T) {
	const objectSchemaYAML = `
type: object
enum:
  - name: alpha
  - name: beta
properties:
  shouldNotParse:
    type: string
`

	node := rawObjectFromYAML(t, objectSchemaYAML)

	dc := DomainContext{
		parse: func(node *json.RawMessage) (*Hash, error) {
			require.Fail(t, "ParseObject should return before parsing properties")
			return nil, nil
		},
	}

	objectDomain, err := dc.ParseObject(node)
	require.NoError(t, err)
	require.Len(t, objectDomain.Enum, 2)
	require.Len(t, dc.domainStore, 2)

	for _, enumHash := range objectDomain.Enum {
		domain, ok := dc.domainStore[*enumHash]
		require.True(t, ok)
		require.IsType(t, new(EnumDomain), domain)
	}
}

func TestParseObjectRejectsInvalidObjectSchemas(t *testing.T) {
	tests := map[string]struct {
		yamlString string
	}{
		"random key outside OpenAPI schema object": {yamlString: `
type: object
notInTheSpecAtAll: true
`},
		"multipleOf is not part of ObjectDomain": {yamlString: `
type: object
multipleOf: 2
`},
		"maximum is not part of ObjectDomain": {yamlString: `
type: object
maximum: 9
`},
		"exclusiveMaximum is not part of ObjectDomain": {yamlString: `
type: object
exclusiveMaximum: true
`},
		"minimum is not part of ObjectDomain": {yamlString: `
type: object
minimum: 1
`},
		"exclusiveMinimum is not part of ObjectDomain": {yamlString: `
type: object
exclusiveMinimum: true
`},
		"maxLength is not part of ObjectDomain": {yamlString: `
type: object
maxLength: 8
`},
		"minLength is not part of ObjectDomain": {yamlString: `
type: object
minLength: 1
`},
		"pattern is not part of ObjectDomain": {yamlString: `
type: object
pattern: ^x$
`},
		"maxItems is not part of ObjectDomain": {yamlString: `
type: object
maxItems: 2
`},
		"minItems is not part of ObjectDomain": {yamlString: `
type: object
minItems: 1
`},
		"uniqueItems is not part of ObjectDomain": {yamlString: `
type: object
uniqueItems: true
`},
		"allOf is not part of ObjectDomain": {yamlString: `
type: object
allOf: []
`},
		"oneOf is not part of ObjectDomain": {yamlString: `
type: object
oneOf: []
`},
		"anyOf is not part of ObjectDomain": {yamlString: `
type: object
anyOf: []
`},
		"not is not part of ObjectDomain": {yamlString: `
type: object
not:
  type: string
`},
		"items is not part of ObjectDomain": {yamlString: `
type: object
items:
  type: string
`},
		"format is not part of ObjectDomain": {yamlString: `
type: object
format: uuid
`},
		"default is not part of ObjectDomain": {yamlString: `
type: object
default: {}
`},
		"discriminator is not part of ObjectDomain": {yamlString: `
type: object
discriminator:
  propertyName: kind
`},
		"readOnly is not part of ObjectDomain": {yamlString: `
type: object
readOnly: true
`},
		"writeOnly is not part of ObjectDomain": {yamlString: `
type: object
writeOnly: true
`},
		"xml is not part of ObjectDomain": {yamlString: `
type: object
xml:
  name: person
`},
		"externalDocs is not part of ObjectDomain": {yamlString: `
type: object
externalDocs:
  url: https://example.com
`},
		"example is not part of ObjectDomain": {yamlString: `
type: object
example: {}
`},
		"deprecated is not part of ObjectDomain": {yamlString: `
type: object
deprecated: true
`},
		"spec extension is not part of ObjectDomain": {yamlString: `
type: object
x-extension: true
`},
		"required empty array is invalid": {yamlString: `
type: object
required: []
`},
		"readOnly is not allowed in property schemas": {yamlString: `
type: object
properties:
  name:
    type: string
    readOnly: true
`},
		"writeOnly is not allowed in property schemas": {yamlString: `
type: object
properties:
  name:
    type: string
    writeOnly: true
`},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			node := rawObjectFromYAML(t, tt.yamlString)
			dc := DomainContext{domainStore: map[Hash]Domain{}}

			objectDomain, err := dc.ParseObject(node)
			require.Error(t, err)
			require.Empty(t, objectDomain)
			require.Empty(t, dc.domainStore)
		})
	}
}
