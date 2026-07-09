package domain

import (
	"encoding/json"
	"errors"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.

	testgenerator "decode_and_validate_generator/pkg/test_generator" //nolint:depguard // Package under test.

	"github.com/stretchr/testify/require"
)

// failingGenerateHashDomain returns a deterministic hashing failure.
type failingGenerateHashDomain struct{}

// GenerateHash returns the configured test failure.
func (failingGenerateHashDomain) GenerateHash() (types.Hash, error) {
	return types.Hash{}, errors.New("generate hash failed")
}

// AllOfMerge is not implemented by this hashing test double.
func (failingGenerateHashDomain) AllOfMerge(types.Domain) (types.Domain, error) {
	return nil, errors.New("NOT IMPLEMENTED")
}

// fakeObjectTestDomain returns a configured hash.
type fakeObjectTestDomain struct {
	hash types.Hash
}

// GenerateHash returns the configured hash.
func (f fakeObjectTestDomain) GenerateHash() (types.Hash, error) {
	return f.hash, nil
}

// AllOfMerge is not implemented by this parser test double.
func (fakeObjectTestDomain) AllOfMerge(types.Domain) (types.Domain, error) {
	return nil, errors.New("NOT IMPLEMENTED")
}

// rawObjectFromYAML converts a YAML test fixture to raw JSON.
func rawObjectFromYAML(t *testing.T, yamlString string) *json.RawMessage {
	t.Helper()

	node, err := testgenerator.YAMLBytesToJSONRawMessage([]byte(yamlString))
	require.NoError(t, err)

	return node
}

// requireDomainStoreDomains compares the domains committed by a parser.
func requireDomainStoreDomains(t *testing.T, dc *Context, expectedDomains ...types.Domain) {
	t.Helper()

	storedDomains := make([]types.Domain, 0, len(dc.domainStore))
	for storedDomain := range dc.domainStore {
		storedDomains = append(storedDomains, storedDomain)
	}

	require.ElementsMatch(t, expectedDomains, storedDomains)
}

// TestParseObjectParsesValidObjectSchemas covers supported object schemas.
func TestParseObjectParsesValidObjectSchemas(t *testing.T) {
	t.Parallel()

	propertyNameHash := types.Hash{1}
	propertyAgeHash := types.Hash{2}
	additionalPropertyHash := types.Hash{3}
	refPropertyHash := types.Hash{4}
	refAdditionalPropertyHash := types.Hash{5}
	propertyNameDomain := fakeObjectTestDomain{hash: propertyNameHash}
	propertyAgeDomain := fakeObjectTestDomain{hash: propertyAgeHash}
	additionalPropertyDomain := fakeObjectTestDomain{hash: additionalPropertyHash}
	refPropertyDomain := fakeObjectTestDomain{hash: refPropertyHash}
	refAdditionalPropertyDomain := fakeObjectTestDomain{hash: refAdditionalPropertyHash}
	ageProperty := &Property{Key: "age", Domain: propertyAgeDomain}
	nameProperty := &Property{Key: "name", Domain: propertyNameDomain}
	refProperty := &Property{Key: "thing", Domain: refPropertyDomain}
	requiredNameProperty := &Property{Key: "name", Domain: propertyNameDomain, Required: true}
	requiredAgeOnlyProperty := &Property{Key: "age", Required: true}
	requiredNameOnlyProperty := &Property{Key: "name", Required: true}
	requiredAdditionalProperty := &Property{
		Key:      "label",
		Domain:   additionalPropertyDomain,
		Required: true,
	}
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
				Properties:             []Property{*ageProperty, *nameProperty},
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
				Properties:             []Property{*requiredNameProperty},
				AdditionalPropertyKind: AdditionalTrue,
			},
		},
		"property ref is parsed as resolved target domain": {
			yamlString: `
type: object
properties:
  thing:
    $ref: '#/components/schemas/Thing'
`,
			parseDomains:  []types.Domain{refPropertyDomain},
			expectedStore: []types.Domain{refPropertyDomain, refProperty},
			expected: ObjectDomain{
				Properties:             []Property{*refProperty},
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
				Properties:             []Property{*requiredAgeOnlyProperty, *requiredNameOnlyProperty},
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
		"undeclared required property uses additionalProperties schema": {
			yamlString: `
type: object
required:
  - label
additionalProperties:
  type: string
`,
			parseDomains: []types.Domain{additionalPropertyDomain},
			expectedStore: []types.Domain{
				additionalPropertyDomain,
				requiredAdditionalProperty,
			},
			expected: ObjectDomain{
				Properties:               []Property{*requiredAdditionalProperty},
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: additionalPropertyDomain,
			},
		},
		"additionalProperties empty schema object is free-form": {
			yamlString: `
type: object
additionalProperties: {}
`,
			expected: ObjectDomain{
				AdditionalPropertyKind: AdditionalTrue,
			},
		},
		"additionalProperties ref is parsed as resolved target domain": {
			yamlString: `
type: object
additionalProperties:
  $ref: '#/components/schemas/ThingValue'
`,
			parseDomains:  []types.Domain{refAdditionalPropertyDomain},
			expectedStore: []types.Domain{refAdditionalPropertyDomain},
			expected: ObjectDomain{
				AdditionalPropertyKind:   AdditionalSchema,
				AdditionalPropertyDomain: refAdditionalPropertyDomain,
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
			t.Parallel()

			node := rawObjectFromYAML(t, tt.yamlString)
			parseCall := 0
			dc := Context{
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

// TestParseObjectParsesNestedObjectWithDefaultParser covers nested objects.
func TestParseObjectParsesNestedObjectWithDefaultParser(t *testing.T) {
	t.Parallel()

	const objectSchemaYAML = `
type: object
required:
  - contact_info
properties:
  id:
    type: integer
  contact_info:
    type: object
    properties:
      email:
        type: string
      phone:
        type: string
`

	node := rawObjectFromYAML(t, objectSchemaYAML)
	objectDomain, err := (&Context{}).ParseObject(node)
	require.NoError(t, err)
	require.Len(t, objectDomain.Properties, 2)
	require.Equal(t, "contact_info", objectDomain.Properties[0].Key)
	require.True(t, objectDomain.Properties[0].Required)
	require.IsType(t, new(ObjectDomain), objectDomain.Properties[0].Domain)
	require.Equal(t, "id", objectDomain.Properties[1].Key)
}

// TestObjectDomainAllOfMerge covers a representative object intersection.
func TestObjectDomainAllOfMerge(t *testing.T) {
	t.Parallel()

	first := &ObjectDomain{
		Nullable:               true,
		Properties:             []Property{{Key: "id", Required: true}},
		AdditionalPropertyKind: AdditionalTrue,
		MinProps:               1,
		MaxProps:               new(5),
	}
	second := &ObjectDomain{
		Nullable:               true,
		Properties:             []Property{{Key: "name"}},
		AdditionalPropertyKind: AdditionalTrue,
		MinProps:               2,
		MaxProps:               new(3),
	}

	mergedDomain, err := first.AllOfMerge(second)
	require.NoError(t, err)
	require.Equal(t, &ObjectDomain{
		Nullable:               true,
		Properties:             []Property{{Key: "id", Required: true}, {Key: "name"}},
		AdditionalPropertyKind: AdditionalTrue,
		MinProps:               2,
		MaxProps:               new(3),
	}, mergedDomain)
}

// TestObjectDomainAllOfMergeErrors covers invalid object intersections.
func TestObjectDomainAllOfMergeErrors(t *testing.T) {
	t.Parallel()

	_, err := (*ObjectDomain)(nil).AllOfMerge(&ObjectDomain{})
	require.ErrorContains(t, err, "object domain cannot be nil")

	_, err = (&ObjectDomain{}).AllOfMerge(&StringDomain{})
	require.ErrorContains(t, err, "domain is not ObjectDomain")

	_, err = (&ObjectDomain{Properties: []Property{{Key: "id", Domain: &StringDomain{}}}}).AllOfMerge(
		&ObjectDomain{Properties: []Property{{Key: "id", Domain: &BoolDomain{}}}},
	)
	require.Error(t, err)
}

// TestParseObjectParsesEnumAndOtherConstraints prevents enum short-circuiting.
func TestParseObjectParsesEnumAndOtherConstraints(t *testing.T) {
	t.Parallel()

	const objectSchemaYAML = `
type: object
enum:
  - name: alpha
  - {}
  - name: alpha
    extra: rejected
  - wrong-type
  - null
required:
  - name
properties:
  name:
    type: string
additionalProperties: false
maxProperties: 1
x-extension: true
`

	node := rawObjectFromYAML(t, objectSchemaYAML)
	dc := Context{}

	objectDomain, err := dc.ParseObject(node)
	require.NoError(t, err)
	require.Equal(t, ObjectDomain{
		AdditionalPropertyKind: AdditionalFalse,
		Enum:                   []types.Enum{types.Enum(`{"name":"alpha"}`)},
		Properties: []Property{{
			Key:      "name",
			Domain:   &StringDomain{},
			Required: true,
		}},
		MaxProps: new(1),
	}, objectDomain)
	require.Len(t, dc.domainStore, 2)
}

// TestParseObjectRetainsNullWhenObjectEnumsViolateConstraints covers nullable enum rescue.
func TestParseObjectRetainsNullWhenObjectEnumsViolateConstraints(t *testing.T) {
	t.Parallel()

	node := rawObjectFromYAML(t, `
type: object
nullable: true
enum:
  - null
  - {}
minProperties: 1
`)

	objectDomain, err := (&Context{}).ParseObject(node)
	require.NoError(t, err)
	require.Equal(t, ObjectDomain{
		Nullable:               true,
		Enum:                   []types.Enum{types.Enum(`null`)},
		AdditionalPropertyKind: AdditionalTrue,
		MinProps:               1,
	}, objectDomain)
}

// TestParseObjectRejectsInvalidObjectSchemas covers invalid object schemas.
func TestParseObjectRejectsInvalidObjectSchemas(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		yamlString string
	}{
		"missing type object is invalid": {yamlString: `
properties: {}
`},
		"random key outside OpenAPI schema object": {yamlString: `
type: object
notInTheSpecAtAll: true
`},
		"enum does not bypass unknown key validation": {yamlString: `
type: object
enum:
  - {}
notInTheSpecAtAll: true
`},
		"nonnullable object enum cannot contain only null": {yamlString: `
type: object
enum:
  - null
`},
		"object enum must satisfy minProperties": {yamlString: `
type: object
enum:
  - {}
minProperties: 1
`},
		"object enum must satisfy maxProperties": {yamlString: `
type: object
enum:
  - first: 1
    second: 2
maxProperties: 1
`},
		"object enum must contain required names": {yamlString: `
type: object
enum:
  - {}
required:
  - name
`},
		"closed object enum cannot contain undeclared names": {yamlString: `
type: object
enum:
  - forbidden: true
additionalProperties: false
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
		"nullable must be boolean": {yamlString: `
type: object
nullable: nope
`},
		"nullable cannot be null": {yamlString: `
type: object
nullable: null
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
		"undeclared required property is forbidden by additionalProperties false": {yamlString: `
type: object
required:
  - name
additionalProperties: false
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
		"minProperties cannot be greater than maxProperties": {yamlString: `
type: object
minProperties: 2
maxProperties: 1
`},
		"required property count cannot exceed maxProperties": {yamlString: `
type: object
required:
  - first
  - second
maxProperties: 1
`},
		"closed object cannot satisfy minProperties": {yamlString: `
type: object
properties:
  only:
    type: string
additionalProperties: false
minProperties: 2
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
			t.Parallel()

			node := rawObjectFromYAML(t, tt.yamlString)
			dc := Context{domainStore: domainStore{}}

			objectDomain, err := dc.ParseObject(node)
			require.Error(t, err)
			require.Empty(t, objectDomain)
			require.Empty(t, dc.domainStore)
		})
	}
}

// TestParseObjectAdditionalPropertiesPropagatesDecodeErrors covers malformed raw tokens.
func TestParseObjectAdditionalPropertiesPropagatesDecodeErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"empty token":      " ",
		"malformed object": `{`,
		"malformed true":   `tru`,
		"malformed false":  `fals`,
	}

	for name, rawValue := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			raw := json.RawMessage(rawValue)
			jsonKV := JSONKV{"additionalProperties": raw}
			objectDomain := ObjectDomain{}

			err := (&Context{}).parseObjectAdditionalProperties(jsonKV, &raw, &objectDomain)
			require.Error(t, err)
			require.Empty(t, objectDomain)
		})
	}
}

// TestParseObjectDoesNotCommitDomainsWhenReturningError checks parser rollback.
func TestParseObjectDoesNotCommitDomainsWhenReturningError(t *testing.T) {
	t.Parallel()

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
			parse: func(_ int) (types.Domain, error) {
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
			parse: func(_ int) (types.Domain, error) {
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
additionalProperties:
  type: string
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
			t.Parallel()

			node := rawObjectFromYAML(t, tt.yamlString)
			parseCall := 0
			dc := Context{parse: func(_ *json.RawMessage) (types.Domain, error) {
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

// TestPropertyGenerateHash covers property hashing.
func TestPropertyGenerateHash(t *testing.T) {
	t.Parallel()

	property := Property{Key: "name", Domain: &StringDomain{}, Required: true}
	stringHash, err := (&StringDomain{}).GenerateHash()
	require.NoError(t, err)

	got, err := property.GenerateHash()
	require.NoError(t, err)
	require.Equal(
		t,
		requireGeneratedHash(t, "property", propertyHashValue{Key: "name", Hasher: &stringHash, Required: true}),
		got,
	)

	got, err = (&Property{Key: "nickname", Required: true}).GenerateHash()
	require.NoError(t, err)
	require.Equal(t, requireGeneratedHash(t, "property", propertyHashValue{Key: "nickname", Required: true}), got)
}

// TestPropertyGenerateHashErrors covers property hashing failures.
func TestPropertyGenerateHashErrors(t *testing.T) {
	t.Parallel()

	_, err := (*Property)(nil).GenerateHash()
	require.Error(t, err)

	_, err = (&Property{Domain: failingGenerateHashDomain{}}).GenerateHash()
	require.Error(t, err)
}

// TestObjectDomainGenerateHash covers object hashing.
func TestObjectDomainGenerateHash(t *testing.T) {
	t.Parallel()

	maxProps := new(3)
	object := ObjectDomain{
		Nullable:                 true,
		Enum:                     []types.Enum{types.Enum(`{"name":"alpha"}`)},
		Properties:               []Property{{Key: "name", Domain: &StringDomain{}, Required: true}},
		AdditionalPropertyKind:   AdditionalSchema,
		AdditionalPropertyDomain: &StringDomain{},
		MinProps:                 1,
		MaxProps:                 maxProps,
	}

	propertyHash, err := (&Property{Key: "name", Domain: &StringDomain{}, Required: true}).GenerateHash()
	require.NoError(t, err)
	additionalPropertyHash, err := (&StringDomain{}).GenerateHash()
	require.NoError(t, err)

	got, err := object.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, requireGeneratedHash(t, "object", objectHashValue{
		Nullable:                 true,
		Enum:                     []types.Enum{types.Enum(`{"name":"alpha"}`)},
		Properties:               []*types.Hash{&propertyHash},
		AdditionalPropertyKind:   AdditionalSchema,
		AdditionalPropertyDomain: &additionalPropertyHash,
		MinProps:                 1,
		MaxProps:                 maxProps,
	}), got)
}

// TestObjectDomainGenerateHashCanonicalizesPropertyOrder verifies semantic property hashing.
func TestObjectDomainGenerateHashCanonicalizesPropertyOrder(t *testing.T) {
	t.Parallel()

	ordered := ObjectDomain{Properties: []Property{{Key: "a"}, {Key: "b"}}}
	reversed := ObjectDomain{Properties: []Property{{Key: "b"}, {Key: "a"}}}
	before := append([]Property(nil), reversed.Properties...)

	orderedHash, err := ordered.GenerateHash()
	require.NoError(t, err)
	reversedHash, err := reversed.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, orderedHash, reversedHash)
	require.Equal(t, before, reversed.Properties)
}

// TestObjectDomainGenerateHashFiltersStructuralEnums covers hashing a programmatic domain.
func TestObjectDomainGenerateHashFiltersStructuralEnums(t *testing.T) {
	t.Parallel()

	maxProps := new(1)
	object := ObjectDomain{
		Enum: []types.Enum{
			types.Enum(`{"extra":1}`),
			types.Enum(`{"a":1}`),
			types.Enum(`{}`),
		},
		Properties:             []Property{{Key: "a", Required: true}},
		AdditionalPropertyKind: AdditionalFalse,
		MinProps:               1,
		MaxProps:               maxProps,
	}
	before := object
	before.Enum = append([]types.Enum(nil), object.Enum...)

	propertyHash, err := (&Property{Key: "a", Required: true}).GenerateHash()
	require.NoError(t, err)

	got, err := object.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, before, object)
	require.Equal(t, requireGeneratedHash(t, "object", objectHashValue{
		Enum:                   []types.Enum{types.Enum(`{"a":1}`)},
		Properties:             []*types.Hash{&propertyHash},
		AdditionalPropertyKind: AdditionalFalse,
		MinProps:               1,
		MaxProps:               maxProps,
	}), got)
}

// TestObjectDomainGenerateHashPreservesAllowedNull covers null-only enum rescue.
func TestObjectDomainGenerateHashPreservesAllowedNull(t *testing.T) {
	t.Parallel()

	object := ObjectDomain{
		Nullable: true,
		Enum: []types.Enum{
			types.Enum(`null`),
			types.Enum(`{"extra":1}`),
		},
		AdditionalPropertyKind: AdditionalFalse,
		MinProps:               1,
	}
	before := object
	before.Enum = append([]types.Enum(nil), object.Enum...)

	got, err := object.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, before, object)
	require.Equal(t, requireGeneratedHash(t, "object", objectHashValue{
		Nullable:               true,
		Enum:                   []types.Enum{types.Enum(`null`)},
		Properties:             []*types.Hash{},
		AdditionalPropertyKind: AdditionalFalse,
		MinProps:               1,
	}), got)
}

// TestObjectDomainGenerateHashRejectsStructurallyEmptyEnums covers finite enum failures.
func TestObjectDomainGenerateHashRejectsStructurallyEmptyEnums(t *testing.T) {
	t.Parallel()

	tests := map[string]*ObjectDomain{
		"minimum property count": {
			Enum:     []types.Enum{types.Enum(`{}`)},
			MinProps: 1,
		},
		"maximum property count": {
			Enum:     []types.Enum{types.Enum(`{"a":1,"b":2}`)},
			MaxProps: new(1),
		},
		"required name": {
			Enum:       []types.Enum{types.Enum(`{}`)},
			Properties: []Property{{Key: "a", Required: true}},
		},
		"closed property set": {
			Enum:                   []types.Enum{types.Enum(`{"extra":1}`)},
			AdditionalPropertyKind: AdditionalFalse,
		},
	}

	for name, objectDomain := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := objectDomain.GenerateHash()
			require.Error(t, err)
		})
	}
}

// TestObjectDomainGenerateHashErrors covers object hashing failures.
func TestObjectDomainGenerateHashErrors(t *testing.T) {
	t.Parallel()

	_, err := (*ObjectDomain)(nil).GenerateHash()
	require.Error(t, err)

	_, err = (&ObjectDomain{Enum: nil}).GenerateHash()
	require.NoError(t, err)

	_, err = (&ObjectDomain{Properties: nil}).GenerateHash()
	require.NoError(t, err)

	_, err = (&ObjectDomain{AdditionalPropertyKind: AdditionalSchema}).GenerateHash()
	require.Error(t, err)

	_, err = (&ObjectDomain{AdditionalPropertyKind: AdditionalPropertyKind(99)}).GenerateHash()
	require.Error(t, err)

	_, err = (&ObjectDomain{Properties: []Property{{Key: "duplicate"}, {Key: "duplicate"}}}).GenerateHash()
	require.Error(t, err)

	_, err = (&ObjectDomain{MinProps: -1}).GenerateHash()
	require.Error(t, err)

	_, err = (&ObjectDomain{MaxProps: new(-1)}).GenerateHash()
	require.Error(t, err)

	_, err = (&ObjectDomain{AdditionalPropertyDomain: failingGenerateHashDomain{}}).GenerateHash()
	require.Error(t, err)
}

// TestObjectDomainHashAndPropertyErrors covers remaining object hash branches.
func TestObjectDomainHashAndPropertyErrors(t *testing.T) {
	t.Parallel()

	_, err := (&ObjectDomain{}).GenerateHash()
	require.NoError(t, err)

	require.EqualError(t, (&PropertyAlreadyExistsError{Key: "name"}), `property "name" already exists in object`)
}

// TestParseObjectErrorBranches covers parser error propagation.
func TestParseObjectErrorBranches(t *testing.T) {
	t.Parallel()

	t.Run("nil node", func(t *testing.T) {
		t.Parallel()

		_, err := (&Context{}).ParseObject(nil)
		require.Error(t, err)
	})

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()

		node := json.RawMessage(`{`)
		_, err := (&Context{}).ParseObject(&node)
		require.Error(t, err)
	})

	t.Run("object struct decode error", func(t *testing.T) {
		t.Parallel()

		node := json.RawMessage(`{"type":{}}`)
		_, err := (&Context{}).ParseObject(&node)
		require.Error(t, err)
	})

	t.Run("property parse error", func(t *testing.T) {
		t.Parallel()

		node := rawObjectFromYAML(t, `
type: object
properties:
  name:
    type: string
`)
		dc := Context{parse: func(_ *json.RawMessage) (types.Domain, error) {
			return nil, errors.New("parse failed")
		}}
		_, err := dc.ParseObject(node)
		require.Error(t, err)
	})

	t.Run("invalid property schema", func(t *testing.T) {
		t.Parallel()

		node := rawObjectFromYAML(t, `
type: object
properties:
  name: bad
`)
		_, err := (&Context{}).ParseObject(node)
		require.Error(t, err)
	})

	t.Run("additionalProperties null", func(t *testing.T) {
		t.Parallel()

		node := rawObjectFromYAML(t, `
type: object
additionalProperties: null
`)
		_, err := (&Context{}).ParseObject(node)
		require.Error(t, err)
	})

	t.Run("additionalProperties parse error", func(t *testing.T) {
		t.Parallel()

		node := rawObjectFromYAML(t, `
type: object
additionalProperties:
  type: string
`)
		dc := Context{parse: func(_ *json.RawMessage) (types.Domain, error) {
			return nil, errors.New("parse failed")
		}}
		_, err := dc.ParseObject(node)
		require.Error(t, err)
	})
}

// TestParseObjectInitializesNilDomainStoreForPropertiesAndAdditionalProperties checks lazy storage.
func TestParseObjectInitializesNilDomainStoreForPropertiesAndAdditionalProperties(t *testing.T) {
	t.Parallel()

	t.Run("properties", func(t *testing.T) {
		t.Parallel()

		node := rawObjectFromYAML(t, `
type: object
properties:
  name:
    type: string
`)
		hash := types.Hash{1}
		dc := Context{
			parse: func(_ *json.RawMessage) (types.Domain, error) { return fakeObjectTestDomain{hash: hash}, nil },
		}
		objectDomain, err := dc.ParseObject(node)
		require.NoError(t, err)
		require.Len(t, objectDomain.Properties, 1)
		require.NotNil(t, dc.domainStore)
	})

	t.Run("additionalProperties", func(t *testing.T) {
		t.Parallel()

		node := rawObjectFromYAML(t, `
type: object
additionalProperties:
  type: string
`)
		hash := types.Hash{1}
		dc := Context{
			parse: func(_ *json.RawMessage) (types.Domain, error) { return fakeObjectTestDomain{hash: hash}, nil },
		}
		objectDomain, err := dc.ParseObject(node)
		require.NoError(t, err)
		require.Equal(t, AdditionalSchema, objectDomain.AdditionalPropertyKind)
		require.NotNil(t, dc.domainStore)
	})
}

// TestParseObjectRequiredWithoutPropertySchema covers unconstrained required names.
func TestParseObjectRequiredWithoutPropertySchema(t *testing.T) {
	t.Parallel()

	node := rawObjectFromYAML(t, `
type: object
required:
  - name
`)
	dc := Context{}
	expectedProperty := &Property{Key: "name", Required: true}

	objectDomain, err := dc.ParseObject(node)
	require.NoError(t, err)
	require.Equal(
		t,
		ObjectDomain{Properties: []Property{*expectedProperty}, AdditionalPropertyKind: AdditionalTrue},
		objectDomain,
	)
	requireDomainStoreDomains(t, &dc, expectedProperty)
}
