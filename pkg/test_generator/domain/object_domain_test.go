package domain

import (
	"encoding/json"
	"errors"
	"testing"

	testgenerator "decode_and_validate_generator/pkg/test_generator"

	"github.com/stretchr/testify/require"
)

type fakeObjectTestDomain struct {
	hash Hash
}

func (f fakeObjectTestDomain) GenerateHash() (Hash, error) {
	return f.hash, nil
}

func rawObjectFromYAML(t *testing.T, yamlString string) *json.RawMessage {
	t.Helper()

	node, err := testgenerator.YAMLBytesToJSONRawMessage([]byte(yamlString))
	require.NoError(t, err)
	return node
}

func requireDomainStoreDomains(t *testing.T, dc *DomainContext, expectedDomains ...Domain) {
	t.Helper()

	storedDomains := make([]Domain, 0, len(dc.domainStore))
	for storedDomain := range dc.domainStore {
		storedDomains = append(storedDomains, storedDomain)
	}

	require.ElementsMatch(t, expectedDomains, storedDomains)
}

func TestParseObjectParsesValidObjectSchemas(t *testing.T) {
	propertyNameHash := Hash{1}
	propertyAgeHash := Hash{2}
	additionalPropertyHash := Hash{3}
	propertyNameDomain := fakeObjectTestDomain{hash: propertyNameHash}
	propertyAgeDomain := fakeObjectTestDomain{hash: propertyAgeHash}
	additionalPropertyDomain := fakeObjectTestDomain{hash: additionalPropertyHash}
	ageProperty := &Property{Key: "age", Domain: propertyAgeDomain}
	nameProperty := &Property{Key: "name", Domain: propertyNameDomain}
	requiredNameProperty := &Property{Key: "name", Domain: propertyNameDomain, Required: true}
	tests := map[string]struct {
		yamlString    string
		parseDomains  []Domain
		expectedStore []Domain
		expected      ObjectDomain
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
			parseDomains:  []Domain{propertyNameDomain, propertyAgeDomain},
			expectedStore: []Domain{propertyNameDomain, propertyAgeDomain, ageProperty, nameProperty},
			expected: ObjectDomain{
				Properties:             []Domain{ageProperty, nameProperty},
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
			parseDomains:  []Domain{propertyNameDomain},
			expectedStore: []Domain{propertyNameDomain, requiredNameProperty},
			expected: ObjectDomain{
				Properties:             []Domain{requiredNameProperty},
				AdditionalPropertyKind: AdditionalTrue,
			},
		},
		"additionalProperties true": {
			yamlString: `
type: object
additionalProperties: true
`,
			expected: ObjectDomain{
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
			parseDomains:  []Domain{additionalPropertyDomain},
			expectedStore: []Domain{additionalPropertyDomain},
			expected: ObjectDomain{
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: additionalPropertyDomain,
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
				domainStore: domainStore{},
				parse: func(node *json.RawMessage) (Domain, error) {
					require.Less(t, parseCall, len(tt.parseDomains))
					domain := tt.parseDomains[parseCall]
					parseCall++
					return domain, nil
				},
			}

			objectDomain, err := dc.ParseObject(node)
			require.NoError(t, err)
			require.Equal(t, len(tt.parseDomains), parseCall)
			requireDomainStoreDomains(t, &dc, tt.expectedStore...)
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
		parse: func(node *json.RawMessage) (Domain, error) {
			require.Fail(t, "ParseObject should return before parsing properties")
			return nil, nil
		},
	}

	objectDomain, err := dc.ParseObject(node)
	require.NoError(t, err)
	require.Len(t, objectDomain.Enum, 2)
	require.Len(t, dc.domainStore, 2)

	for _, enumDomain := range objectDomain.Enum {
		require.Contains(t, dc.domainStore, enumDomain)
		require.IsType(t, new(EnumDomain), enumDomain)
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
			dc := DomainContext{domainStore: domainStore{}}

			objectDomain, err := dc.ParseObject(node)
			require.Error(t, err)
			require.Empty(t, objectDomain)
			require.Empty(t, dc.domainStore)
		})
	}
}

func TestObjectDomainHashAndPropertyErrors(t *testing.T) {
	_, err := (*Property)(nil).GenerateHash()
	require.Error(t, err)

	_, err = (*ObjectDomain)(nil).GenerateHash()
	require.Error(t, err)

	_, err = (&ObjectDomain{}).GenerateHash()
	require.NoError(t, err)

	require.EqualError(t, (&PropertyAlreadyExistsError{Key: "name"}), `property "name" already exists in object`)
}

func TestParseObjectErrorBranches(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		node := json.RawMessage(`{`)
		_, err := (&DomainContext{}).ParseObject(&node)
		require.Error(t, err)
	})

	t.Run("property parse error", func(t *testing.T) {
		node := rawObjectFromYAML(t, `
type: object
properties:
  name:
    type: string
`)
		dc := DomainContext{parse: func(node *json.RawMessage) (Domain, error) {
			return nil, errors.New("parse failed")
		}}
		_, err := dc.ParseObject(node)
		require.Error(t, err)
	})

	t.Run("invalid property schema", func(t *testing.T) {
		node := rawObjectFromYAML(t, `
type: object
properties:
  name: bad
`)
		_, err := (&DomainContext{}).ParseObject(node)
		require.Error(t, err)
	})

	t.Run("additionalProperties null", func(t *testing.T) {
		node := rawObjectFromYAML(t, `
type: object
additionalProperties: null
`)
		_, err := (&DomainContext{}).ParseObject(node)
		require.Error(t, err)
	})

	t.Run("additionalProperties parse error", func(t *testing.T) {
		node := rawObjectFromYAML(t, `
type: object
additionalProperties:
  type: string
`)
		dc := DomainContext{parse: func(node *json.RawMessage) (Domain, error) {
			return nil, errors.New("parse failed")
		}}
		_, err := dc.ParseObject(node)
		require.Error(t, err)
	})
}

func TestParseObjectInitializesNilDomainStoreForPropertiesAndAdditionalProperties(t *testing.T) {
	t.Run("properties", func(t *testing.T) {
		node := rawObjectFromYAML(t, `
type: object
properties:
  name:
    type: string
`)
		hash := Hash{1}
		dc := DomainContext{parse: func(node *json.RawMessage) (Domain, error) { return fakeObjectTestDomain{hash: hash}, nil }}
		objectDomain, err := dc.ParseObject(node)
		require.NoError(t, err)
		require.Len(t, objectDomain.Properties, 1)
		require.NotNil(t, dc.domainStore)
	})

	t.Run("additionalProperties", func(t *testing.T) {
		node := rawObjectFromYAML(t, `
type: object
additionalProperties:
  type: string
`)
		hash := Hash{1}
		dc := DomainContext{parse: func(node *json.RawMessage) (Domain, error) { return fakeObjectTestDomain{hash: hash}, nil }}
		objectDomain, err := dc.ParseObject(node)
		require.NoError(t, err)
		require.Equal(t, AdditionalSchema, objectDomain.AdditionalPropertyKind)
		require.NotNil(t, dc.domainStore)
	})
}

func TestParseObjectRequiredWithoutPropertySchema(t *testing.T) {
	node := rawObjectFromYAML(t, `
type: object
required:
  - name
`)
	objectDomain, err := (&DomainContext{}).ParseObject(node)
	require.NoError(t, err)
	require.Len(t, objectDomain.Properties, 1)
}
