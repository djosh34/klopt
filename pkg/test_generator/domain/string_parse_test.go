package domain

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseStringParsesValidStringSchemas(t *testing.T) {
	tests := map[string]struct {
		yamlString string
		expected   StringDomain
	}{
		"minimal string": {
			yamlString: `
type: string
`,
			expected: StringDomain{},
		},
		"title and description are allowed documentation fields": {
			yamlString: `
type: string
title: Display name
description: A display name.
`,
			expected: StringDomain{},
		},
		"nullable true": {
			yamlString: `
type: string
nullable: true
`,
			expected: StringDomain{Nullable: true},
		},
		"nullable false": {
			yamlString: `
type: string
nullable: false
`,
			expected: StringDomain{},
		},
		"enum strings": {
			yamlString: `
type: string
enum:
  - alpha
  - beta
`,
			expected: StringDomain{Enum: []types.Enum{types.Enum("\"alpha\""), types.Enum("\"beta\"")}},
		},
		"minLength and maxLength": {
			yamlString: `
type: string
minLength: 1
maxLength: 5
`,
			expected: StringDomain{MinLength: 1, MaxLength: new(5)},
		},
		"pattern with required examples": {
			yamlString: `
type: string
pattern: '^[a-z]+$'
x-valid-examples:
  - alpha
  - beta
x-invalid-examples:
  - ABC
  - '123'
`,
			expected: StringDomain{Pattern: types.Pattern{"^[a-z]+$"}, XValidExamples: []string{"alpha", "beta"}, XInvalidExamples: []string{"ABC", "123"}},
		},
		"invalid regex pattern is accepted when examples are provided": {
			yamlString: `
type: string
pattern: '['
x-valid-examples:
  - '['
x-invalid-examples:
  - ']'
`,
			expected: StringDomain{Pattern: types.Pattern{"["}, XValidExamples: []string{"["}, XInvalidExamples: []string{"]"}},
		},
		"pattern examples are trusted verbatim without validation": {
			yamlString: `
type: string
pattern: '^a$'
x-valid-examples:
  - does-not-match-pattern
x-invalid-examples:
  - a
`,
			expected: StringDomain{Pattern: types.Pattern{"^a$"}, XValidExamples: []string{"does-not-match-pattern"}, XInvalidExamples: []string{"a"}},
		},
		"format with required examples": {
			yamlString: `
type: string
format: email
x-valid-examples:
  - a@example.com
x-invalid-examples:
  - not-an-email
`,
			expected: StringDomain{Format: types.Format{"email"}, XValidExamples: []string{"a@example.com"}, XInvalidExamples: []string{"not-an-email"}},
		},
		"unknown format is accepted when examples are provided": {
			yamlString: `
type: string
format: made-up-format
x-valid-examples:
  - made-up-valid
x-invalid-examples:
  - made-up-invalid
`,
			expected: StringDomain{Format: types.Format{"made-up-format"}, XValidExamples: []string{"made-up-valid"}, XInvalidExamples: []string{"made-up-invalid"}},
		},
		"format examples are trusted verbatim without validation": {
			yamlString: `
type: string
format: email
x-valid-examples:
  - not-an-email-but-trusted
x-invalid-examples:
  - a@example.com
`,
			expected: StringDomain{Format: types.Format{"email"}, XValidExamples: []string{"not-an-email-but-trusted"}, XInvalidExamples: []string{"a@example.com"}},
		},
		"pattern and format share required examples": {
			yamlString: `
type: string
pattern: '^ID-[0-9]+$'
format: internal-id
x-valid-examples:
  - ID-123
x-invalid-examples:
  - nope
`,
			expected: StringDomain{Pattern: types.Pattern{"^ID-[0-9]+$"}, Format: types.Format{"internal-id"}, XValidExamples: []string{"ID-123"}, XInvalidExamples: []string{"nope"}},
		},
		"all supported fields together": {
			yamlString: `
type: string
nullable: true
enum:
  - ID-123
pattern: '^ID-[0-9]+$'
format: internal-id
x-valid-examples:
  - ID-123
x-invalid-examples:
  - nope
minLength: 2
maxLength: 10
`,
			expected: StringDomain{Nullable: true, Enum: []types.Enum{types.Enum("\"ID-123\"")}, Pattern: types.Pattern{"^ID-[0-9]+$"}, Format: types.Format{"internal-id"}, XValidExamples: []string{"ID-123"}, XInvalidExamples: []string{"nope"}, MinLength: 2, MaxLength: new(10)},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			node := rawObjectFromYAML(t, tt.yamlString)
			dc := DomainContext{domainStore: domainStore{}}
			stringDomain, err := dc.ParseString(node)
			require.NoError(t, err)
			require.Equal(t, tt.expected, stringDomain)
		})
	}
}

func TestParseStringRejectsInvalidStringSchemas(t *testing.T) {
	tests := map[string]string{
		"missing type": `
nullable: false
`,
		"wrong type": `
type: integer
`,
		"mixed type array": `
type:
  - string
  - integer
`,
		"nullable must be boolean": `
type: string
nullable: nope
`,
		"enum cannot be empty": `
type: string
enum: []
`,
		"enum cannot be null": `
type: string
enum: null
`,
		"enum must be array": `
type: string
enum: alpha
`,
		"minLength cannot be null": `
type: string
minLength: null
`,
		"minLength cannot be negative": `
type: string
minLength: -1
`,
		"minLength must be an integer": `
type: string
minLength: 1.5
`,
		"maxLength cannot be null": `
type: string
maxLength: null
`,
		"maxLength cannot be negative": `
type: string
maxLength: -1
`,
		"maxLength must be an integer": `
type: string
maxLength: 1.5
`,
		"minLength cannot exceed maxLength": `
type: string
minLength: 5
maxLength: 4
`,
		"pattern must be string": `
type: string
pattern: 123
x-valid-examples:
  - abc
x-invalid-examples:
  - '123'
`,
		"format must be string": `
type: string
format: 123
x-valid-examples:
  - abc
x-invalid-examples:
  - '123'
`,
		"x-valid-examples must be array": `
type: string
pattern: '^[a-z]+$'
x-valid-examples: alpha
x-invalid-examples:
  - '123'
`,
		"x-invalid-examples must be array": `
type: string
pattern: '^[a-z]+$'
x-valid-examples:
  - alpha
x-invalid-examples: '123'
`,
		"x-valid-examples cannot be empty": `
type: string
pattern: '^[a-z]+$'
x-valid-examples: []
x-invalid-examples:
  - '123'
`,
		"x-invalid-examples cannot be empty": `
type: string
pattern: '^[a-z]+$'
x-valid-examples:
  - alpha
x-invalid-examples: []
`,
		"x-valid-examples values must be strings": `
type: string
pattern: '^[a-z]+$'
x-valid-examples:
  - 123
x-invalid-examples:
  - '123'
`,
		"x-invalid-examples values must be strings": `
type: string
pattern: '^[a-z]+$'
x-valid-examples:
  - alpha
x-invalid-examples:
  - 123
`,
		"x-valid-examples without pattern or format is unsupported": `
type: string
x-valid-examples:
  - alpha
x-invalid-examples:
  - '123'
`,
		"x-invalid-examples without pattern or format is unsupported": `
type: string
x-invalid-examples:
  - '123'
`,
		"minimum is not part of StringDomain": `
type: string
minimum: 1
`,
		"maximum is not part of StringDomain": `
type: string
maximum: 1
`,
		"multipleOf is not part of StringDomain": `
type: string
multipleOf: 2
`,
		"items is not part of StringDomain": `
type: string
items:
  type: string
`,
		"properties is not part of StringDomain": `
type: string
properties: {}
`,
		"additionalProperties is not part of StringDomain": `
type: string
additionalProperties: false
`,
		"allOf is not part of StringDomain": `
type: string
allOf: []
`,
		"oneOf must be rejected": `
type: string
oneOf:
  - type: string
`,
		"anyOf must be rejected": `
type: string
anyOf:
  - type: string
`,
		"not must be rejected": `
type: string
not:
  type: integer
`,
		"discriminator must be rejected": `
type: string
discriminator:
  propertyName: kind
`,
		"default is unsupported": `
type: string
default: abc
`,
		"readOnly is unsupported": `
type: string
readOnly: true
`,
		"writeOnly is unsupported": `
type: string
writeOnly: true
`,
		"example is unsupported": `
type: string
example: abc
`,
		"deprecated is unsupported": `
type: string
deprecated: true
`,
		"spec extension is unsupported": `
type: string
x-extra: abc
`,
	}

	for testName, yamlString := range tests {
		t.Run(testName, func(t *testing.T) {
			node := rawObjectFromYAML(t, yamlString)
			dc := DomainContext{domainStore: domainStore{}}
			stringDomain, err := dc.ParseString(node)
			require.Error(t, err)
			require.Empty(t, stringDomain)
		})
	}
}

func TestParseStringRejectsPatternFormatWithoutBothExampleLists(t *testing.T) {
	tests := map[string]string{
		"pattern without examples": `
type: string
pattern: '^[a-z]+$'
`,
		"pattern with valid examples only": `
type: string
pattern: '^[a-z]+$'
x-valid-examples:
  - alpha
`,
		"pattern with invalid examples only": `
type: string
pattern: '^[a-z]+$'
x-invalid-examples:
  - '123'
`,
		"format without examples": `
type: string
format: email
`,
		"format with valid examples only": `
type: string
format: email
x-valid-examples:
  - a@example.com
`,
		"format with invalid examples only": `
type: string
format: email
x-invalid-examples:
  - not-an-email
`,
		"pattern and format with no examples": `
type: string
pattern: '^[a-z]+$'
format: email
`,
		"pattern and format with valid examples only": `
type: string
pattern: '^[a-z]+$'
format: email
x-valid-examples:
  - a@example.com
`,
		"pattern and format with invalid examples only": `
type: string
pattern: '^[a-z]+$'
format: email
x-invalid-examples:
  - not-an-email
`,
	}

	for testName, yamlString := range tests {
		t.Run(testName, func(t *testing.T) {
			node := rawObjectFromYAML(t, yamlString)
			dc := DomainContext{domainStore: domainStore{}}
			_, err := dc.ParseString(node)
			require.Error(t, err)
		})
	}
}
