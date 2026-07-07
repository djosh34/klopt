package domain

import (
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
	"testing"

	testgenerator "decode_and_validate_generator/pkg/test_generator"

	"github.com/stretchr/testify/require"
)

type failingToHasherDomain struct{}

func (f failingToHasherDomain) ToHasher() (types.Hasher, error) {
	return nil, errors.New("to hasher failed")
}

type fakeObjectTestDomain struct {
	hash types.Hash
}

func (f fakeObjectTestDomain) GenerateHash() (types.Hash, error) {
	return f.hash, nil
}

func (f fakeObjectTestDomain) ToHasher() (types.Hasher, error) {
	return f, nil
}

func rawObjectFromYAML(t *testing.T, yamlString string) *json.RawMessage {
	t.Helper()

	node, err := testgenerator.YAMLBytesToJSONRawMessage([]byte(yamlString))
	require.NoError(t, err)
	return node
}

func requireDomainStoreDomains(t *testing.T, dc *DomainContext, expectedDomains ...types.Domain) {
	t.Helper()

	storedDomains := make([]types.Domain, 0, len(dc.domainStore))
	for storedDomain := range dc.domainStore {
		storedDomains = append(storedDomains, storedDomain)
	}

	require.ElementsMatch(t, expectedDomains, storedDomains)
}

func TestParseObjectParsesValidObjectSchemas(t *testing.T) {
	propertyNameHash := types.Hash{1}
	propertyAgeHash := types.Hash{2}
	additionalPropertyHash := types.Hash{3}
	propertyNameDomain := fakeObjectTestDomain{hash: propertyNameHash}
	propertyAgeDomain := fakeObjectTestDomain{hash: propertyAgeHash}
	additionalPropertyDomain := fakeObjectTestDomain{hash: additionalPropertyHash}
	ageProperty := &Property{Key: "age", Domain: propertyAgeDomain}
	nameProperty := &Property{Key: "name", Domain: propertyNameDomain}
	requiredNameProperty := &Property{Key: "name", Domain: propertyNameDomain, Required: true}
	requiredAgeOnlyProperty := &Property{Key: "age", Required: true}
	requiredNameOnlyProperty := &Property{Key: "name", Required: true}
	tests := map[string]struct {
		yamlString    string
		parseDomains  []types.Domain
		expectedStore []types.Domain
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
		"nullable is parsed": {
			yamlString: `
type: object
nullable: true
`,
			expected: ObjectDomain{
				Nullable:               true,
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
			parseDomains:  []types.Domain{propertyNameDomain, propertyAgeDomain},
			expectedStore: []types.Domain{propertyNameDomain, propertyAgeDomain, ageProperty, nameProperty},
			expected: ObjectDomain{
				Properties:             []types.Domain{ageProperty, nameProperty},
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
			parseDomains:  []types.Domain{propertyNameDomain},
			expectedStore: []types.Domain{propertyNameDomain, requiredNameProperty},
			expected: ObjectDomain{
				Properties:             []types.Domain{requiredNameProperty},
				AdditionalPropertyKind: AdditionalTrue,
			},
		},
		"required properties without schemas are parsed and sorted by key": {
			yamlString: `
type: object
required:
  - name
  - age
`,
			expectedStore: []types.Domain{requiredAgeOnlyProperty, requiredNameOnlyProperty},
			expected: ObjectDomain{
				Properties:             []types.Domain{requiredAgeOnlyProperty, requiredNameOnlyProperty},
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
			parseDomains:  []types.Domain{additionalPropertyDomain},
			expectedStore: []types.Domain{additionalPropertyDomain},
			expected: ObjectDomain{
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: additionalPropertyDomain,
			},
		},
		"additionalProperties empty schema object": {
			yamlString: `
type: object
additionalProperties: {}
`,
			parseDomains:  []types.Domain{additionalPropertyDomain},
			expectedStore: []types.Domain{additionalPropertyDomain},
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
		"minProperties and maxProperties allow zero": {
			yamlString: `
type: object
minProperties: 0
maxProperties: 0
`,
			expected: ObjectDomain{
				AdditionalPropertyKind: AdditionalTrue,
				MaxProps:               new(0),
			},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			node := rawObjectFromYAML(t, tt.yamlString)
			parseCall := 0
			dc := DomainContext{
				domainStore: domainStore{},
				parse: func(node *json.RawMessage) (types.Domain, error) {
					require.Less(t, parseCall, len(tt.parseDomains))
					var propertyJSONKV JSONKV
					require.NoError(t, json.Unmarshal(*node, &propertyJSONKV))
					if len(tt.parseDomains) > 1 {
						if propertyTypeJSON, ok := propertyJSONKV["type"]; ok {
							var propertyType string
							require.NoError(t, json.Unmarshal(propertyTypeJSON, &propertyType))
							switch propertyType {
							case "string":
								parseCall++
								return propertyNameDomain, nil
							case "integer":
								parseCall++
								return propertyAgeDomain, nil
							}
						}
					}
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
		parse: func(node *json.RawMessage) (types.Domain, error) {
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
		"nullable must be boolean": {yamlString: `
type: object
nullable: nope
`},
		"top-level readOnly false is still not part of ObjectDomain": {yamlString: `
type: object
readOnly: false
`},
		"top-level writeOnly false is still not part of ObjectDomain": {yamlString: `
type: object
writeOnly: false
`},
		"properties cannot be null": {yamlString: `
type: object
properties: null
`},
		"properties must be an object": {yamlString: `
type: object
properties: []
`},
		"property schema cannot be null": {yamlString: `
type: object
properties:
  name: null
`},
		"required empty array is invalid": {yamlString: `
type: object
required: []
`},
		"required null is invalid": {yamlString: `
type: object
required: null
`},
		"required must be an array": {yamlString: `
type: object
required: name
`},
		"required values must be strings": {yamlString: `
type: object
required:
  - 1
`},
		"required entries must be unique": {yamlString: `
type: object
required:
  - name
  - name
`},
		"additionalProperties string is invalid": {yamlString: `
type: object
additionalProperties: nope
`},
		"additionalProperties number is invalid": {yamlString: `
type: object
additionalProperties: 123
`},
		"additionalProperties array is invalid": {yamlString: `
type: object
additionalProperties: []
`},
		"minProperties cannot be null": {yamlString: `
type: object
minProperties: null
`},
		"minProperties cannot be negative": {yamlString: `
type: object
minProperties: -1
`},
		"minProperties must be an integer": {yamlString: `
type: object
minProperties: 1.5
`},
		"maxProperties cannot be null": {yamlString: `
type: object
maxProperties: null
`},
		"maxProperties cannot be negative": {yamlString: `
type: object
maxProperties: -1
`},
		"maxProperties must be an integer": {yamlString: `
type: object
maxProperties: 1.5
`},
		"readOnly true is not allowed in property schemas": {yamlString: `
type: object
properties:
  name:
    type: string
    readOnly: true
`},
		"readOnly false is not allowed in property schemas": {yamlString: `
type: object
properties:
  name:
    type: string
    readOnly: false
`},
		"writeOnly true is not allowed in property schemas": {yamlString: `
type: object
properties:
  name:
    type: string
    writeOnly: true
`},
		"writeOnly false is not allowed in property schemas": {yamlString: `
type: object
properties:
  name:
    type: string
    writeOnly: false
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

func TestParseObjectDoesNotCommitDomainsWhenReturningError(t *testing.T) {
	propertyDomain := fakeObjectTestDomain{hash: types.Hash{1}}
	tests := map[string]struct {
		yamlString     string
		parse          func(parseCall int) (types.Domain, error)
		wantParseCalls int
	}{
		"validation error after property parse": {
			yamlString: `
type: object
properties:
  name:
    type: string
minProperties: -1
`,
			parse: func(parseCall int) (types.Domain, error) {
				_ = parseCall
				return propertyDomain, nil
			},
			wantParseCalls: 1,
		},
		"unsupported key after property parse": {
			yamlString: `
type: object
properties:
  name:
    type: string
notInTheSpecAtAll: true
`,
			parse: func(parseCall int) (types.Domain, error) {
				_ = parseCall
				return propertyDomain, nil
			},
			wantParseCalls: 1,
		},
		"additionalProperties parse error after property parse": {
			yamlString: `
type: object
properties:
  name:
    type: string
additionalProperties: {}
`,
			parse: func(parseCall int) (types.Domain, error) {
				if parseCall == 0 {
					return propertyDomain, nil
				}
				return nil, errors.New("parse failed")
			},
			wantParseCalls: 2,
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			node := rawObjectFromYAML(t, tt.yamlString)
			parseCall := 0
			dc := DomainContext{parse: func(node *json.RawMessage) (types.Domain, error) {
				_ = node
				domain, err := tt.parse(parseCall)
				parseCall++
				return domain, err
			}}

			objectDomain, err := dc.ParseObject(node)
			require.Error(t, err)
			require.Empty(t, objectDomain)
			require.Equal(t, tt.wantParseCalls, parseCall)
			require.Empty(t, dc.domainStore)
		})
	}
}

func TestPropertyToHasher(t *testing.T) {
	property := Property{Key: "name", Domain: &StringDomain{}, Required: true}

	hasher, err := property.ToHasher()
	require.NoError(t, err)
	require.Equal(t, &hashables.PropertyHashable{Key: "name", Hasher: &hashables.StringHashable{}, Required: true}, hasher)

	hasher, err = (&Property{Key: "nickname", Required: true}).ToHasher()
	require.NoError(t, err)
	require.Equal(t, &hashables.PropertyHashable{Key: "nickname", Required: true}, hasher)
}

func TestPropertyToHasherErrors(t *testing.T) {
	_, err := (*Property)(nil).ToHasher()
	require.Error(t, err)

	_, err = (&Property{Domain: failingToHasherDomain{}}).ToHasher()
	require.Error(t, err)
}

func TestObjectDomainToHasher(t *testing.T) {
	maxProps := new(3)
	object := ObjectDomain{
		Nullable:                 true,
		Enum:                     []types.Domain{&EnumDomain{}},
		Properties:               []types.Domain{&Property{Key: "name", Domain: &StringDomain{}, Required: true}},
		AdditionalPropertyKind:   AdditionalSchema,
		AdditionalPropertyDomain: &StringDomain{},
		MinProps:                 1,
		MaxProps:                 maxProps,
	}

	hasher, err := object.ToHasher()
	require.NoError(t, err)
	require.Equal(t, &hashables.ObjectHashable{
		Nullable:                 true,
		Enum:                     []types.Hasher{&hashables.EnumHashable{}},
		Properties:               []types.Hasher{&hashables.PropertyHashable{Key: "name", Hasher: &hashables.StringHashable{}, Required: true}},
		AdditionalPropertyKind:   hashables.AdditionalSchema,
		AdditionalPropertyDomain: &hashables.StringHashable{},
		MinProps:                 1,
		MaxProps:                 maxProps,
	}, hasher)
}

func TestObjectDomainToHasherErrors(t *testing.T) {
	_, err := (*ObjectDomain)(nil).ToHasher()
	require.Error(t, err)

	_, err = (&ObjectDomain{Enum: nil}).ToHasher()
	require.NoError(t, err)

	_, err = (&ObjectDomain{Properties: nil}).ToHasher()
	require.NoError(t, err)

	_, err = (&ObjectDomain{AdditionalPropertyKind: AdditionalSchema}).ToHasher()
	require.Error(t, err)

	_, err = (&ObjectDomain{AdditionalPropertyDomain: failingToHasherDomain{}}).ToHasher()
	require.Error(t, err)
}

func TestObjectDomainHashAndPropertyErrors(t *testing.T) {
	hasher, err := (&ObjectDomain{}).ToHasher()
	require.NoError(t, err)
	_, err = hasher.GenerateHash()
	require.NoError(t, err)

	require.EqualError(t, (&PropertyAlreadyExistsError{Key: "name"}), `property "name" already exists in object`)
}

func TestParseObjectErrorBranches(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		node := json.RawMessage(`{`)
		_, err := (&DomainContext{}).ParseObject(&node)
		require.Error(t, err)
	})

	t.Run("object struct decode error", func(t *testing.T) {
		node := json.RawMessage(`{"type":{}}`)
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
		dc := DomainContext{parse: func(node *json.RawMessage) (types.Domain, error) {
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
		dc := DomainContext{parse: func(node *json.RawMessage) (types.Domain, error) {
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
		hash := types.Hash{1}
		dc := DomainContext{parse: func(node *json.RawMessage) (types.Domain, error) { return fakeObjectTestDomain{hash: hash}, nil }}
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
		hash := types.Hash{1}
		dc := DomainContext{parse: func(node *json.RawMessage) (types.Domain, error) { return fakeObjectTestDomain{hash: hash}, nil }}
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
	dc := DomainContext{}
	expectedProperty := &Property{Key: "name", Required: true}

	objectDomain, err := dc.ParseObject(node)
	require.NoError(t, err)
	require.Equal(t, ObjectDomain{Properties: []types.Domain{expectedProperty}, AdditionalPropertyKind: AdditionalTrue}, objectDomain)
	requireDomainStoreDomains(t, &dc, expectedProperty)
}
