package testgenerator

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAllOfValidCasesComeFromMergedSchema(t *testing.T) {
	var node SchemaNode
	err := yaml.Unmarshal(allOfThreeRequiredPropertiesSchema(), &node)
	require.NoError(t, err)

	require.Contains(t, rawMessages(node.ValidCases()), `{"first":"valid-string","last":0,"second":true}`)
	require.Contains(t, caseNames(node.ValidCases()), "required properties")
}

func TestAllOfInvalidCasesComeFromMergedSchema(t *testing.T) {
	var node SchemaNode
	err := yaml.Unmarshal(allOfThreeRequiredPropertiesSchema(), &node)
	require.NoError(t, err)

	require.Contains(t, rawMessages(node.InvalidCases()), `{}`)
	require.Contains(t, rawMessages(node.InvalidCases()), `{"last":0,"second":true}`)
	require.Contains(t, rawMessages(node.InvalidCases()), `{"first":"valid-string","last":0}`)
	require.Contains(t, rawMessages(node.InvalidCases()), `{"first":"valid-string","second":true}`)

	require.Contains(t, caseNames(node.InvalidCases()), "missing required properties")
	require.Contains(t, caseNames(node.InvalidCases()), "missing required property first")
	require.Contains(t, caseNames(node.InvalidCases()), "missing required property second")
	require.Contains(t, caseNames(node.InvalidCases()), "missing required property last")
}

func TestAllOfUnmarshalRejectsImpossibleMerge(t *testing.T) {
	content := []byte(`
allOf:
  - type: string
  - type: number
`)

	var node SchemaNode
	err := yaml.Unmarshal(content, &node)

	require.ErrorContains(t, err, "merge allOf schema 2")
	require.ErrorContains(t, err, `cannot merge schema type "string" with "number"`)
}

func TestAllOfUnmarshalReportsLaterImpossibleMergeIndex(t *testing.T) {
	content := []byte(`
allOf:
  - type: object
    properties:
      first:
        type: string
  - type: object
    properties:
      second:
        type: boolean
  - type: object
    properties:
      first:
        type: number
`)

	var node SchemaNode
	err := yaml.Unmarshal(content, &node)

	require.ErrorContains(t, err, "merge allOf schema 3")
	require.ErrorContains(t, err, `property "first"`)
	require.ErrorContains(t, err, `cannot merge schema type "string" with "number"`)
}

func TestAllOfMergedObjectInvalidCasesUseMergedPropertyRules(t *testing.T) {
	content := []byte(`
allOf:
  - type: object
    required:
      - id
    properties:
      id:
        type: string
        nullable: true
  - type: object
    required:
      - id
    additionalProperties: false
    properties:
      id:
        type: string
        nullable: false
`)

	var node SchemaNode
	err := yaml.Unmarshal(content, &node)
	require.NoError(t, err)

	require.Contains(t, rawMessages(node.ValidCases()), `{"id":"valid-string"}`)
	require.NotContains(t, rawMessages(node.ValidCases()), `{"id":null}`)
	require.Contains(t, rawMessages(node.InvalidCases()), `{"id":null}`)
	require.Contains(t, rawMessages(node.InvalidCases()), `{"`+additionalPropertyCaseKey+`":"not-allowed","id":"valid-string"}`)
	require.Equal(t, []string{"id"}, node.Object.Required)
}

func TestAllOfMergedNestedObjectInvalidCasesUseMergedRequiredProperties(t *testing.T) {
	content := []byte(`
allOf:
  - type: object
    required:
      - child
    properties:
      child:
        type: object
        required:
          - first
        properties:
          first:
            type: string
            nullable: false
  - type: object
    required:
      - child
    properties:
      child:
        type: object
        required:
          - second
        additionalProperties: false
        properties:
          second:
            type: boolean
            nullable: false
`)

	var node SchemaNode
	err := yaml.Unmarshal(content, &node)
	require.NoError(t, err)

	require.Contains(t, rawMessages(node.ValidCases()), `{"child":{"first":"valid-string","second":true}}`)
	require.Contains(t, rawMessages(node.InvalidCases()), `{"child":{}}`)
	require.Contains(t, rawMessages(node.InvalidCases()), `{"child":{"second":true}}`)
	require.Contains(t, rawMessages(node.InvalidCases()), `{"child":{"first":"valid-string"}}`)
	require.Contains(t, caseNames(node.InvalidCases()), "invalid property child missing required property first")
	require.Contains(t, caseNames(node.InvalidCases()), "invalid property child missing required property second")
}

func TestAllOfMergedArrayCasesUseMergedItemRules(t *testing.T) {
	content := []byte(`
allOf:
  - type: array
    nullable: true
    items:
      type: string
      nullable: true
  - type: array
    nullable: false
    items:
      type: string
      nullable: false
`)

	var node SchemaNode
	err := yaml.Unmarshal(content, &node)
	require.NoError(t, err)

	require.NotContains(t, rawMessages(node.ValidCases()), `null`)
	require.NotContains(t, rawMessages(node.ValidCases()), `[null]`)
	require.Contains(t, rawMessages(node.ValidCases()), `["valid-string"]`)
	require.Contains(t, rawMessages(node.InvalidCases()), `null`)
	require.Contains(t, rawMessages(node.InvalidCases()), `[null]`)
}

func TestAllOfMergedStringFormatCasesUseMergedFormat(t *testing.T) {
	content := []byte(`
allOf:
  - type: string
    nullable: true
  - type: string
    nullable: false
    format: date-time
`)

	var node SchemaNode
	err := yaml.Unmarshal(content, &node)
	require.NoError(t, err)

	require.Contains(t, rawMessages(node.ValidCases()), `"2026-07-01T12:34:56Z"`)
	require.NotContains(t, rawMessages(node.ValidCases()), `null`)
	require.Contains(t, rawMessages(node.InvalidCases()), `null`)
	require.Contains(t, rawMessages(node.InvalidCases()), `"not-date-time"`)
}

func TestAllOfUnmarshalRejectsArrayItemConflict(t *testing.T) {
	content := []byte(`
allOf:
  - type: array
    items:
      type: string
  - type: array
    items:
      type: boolean
`)

	var node SchemaNode
	err := yaml.Unmarshal(content, &node)

	require.ErrorContains(t, err, "merge allOf schema 2")
	require.ErrorContains(t, err, "array items")
	require.ErrorContains(t, err, `cannot merge schema type "string" with "boolean"`)
}

func TestAllOfUnmarshalRejectsStringFormatConflict(t *testing.T) {
	content := []byte(`
allOf:
  - type: string
    format: date-time
  - type: string
    format: uuid
`)

	var node SchemaNode
	err := yaml.Unmarshal(content, &node)

	require.ErrorContains(t, err, "merge allOf schema 2")
	require.ErrorContains(t, err, `cannot merge string format "date-time" with "uuid"`)
}

func TestAllOfUnmarshalRejectsObjectPropertyConflict(t *testing.T) {
	content := []byte(`
allOf:
  - type: object
    properties:
      same:
        type: string
  - type: object
    properties:
      same:
        type: number
`)

	var node SchemaNode
	err := yaml.Unmarshal(content, &node)

	require.ErrorContains(t, err, "merge allOf schema 2")
	require.ErrorContains(t, err, `property "same"`)
	require.ErrorContains(t, err, `cannot merge schema type "string" with "number"`)
}

func TestAllOfUnmarshalRejectsAdditionalPropertiesConflict(t *testing.T) {
	content := []byte(`
allOf:
  - type: object
    additionalProperties:
      type: string
  - type: object
    additionalProperties:
      type: number
`)

	var node SchemaNode
	err := yaml.Unmarshal(content, &node)

	require.ErrorContains(t, err, "merge allOf schema 2")
	require.ErrorContains(t, err, "additionalProperties")
	require.ErrorContains(t, err, `cannot merge schema type "string" with "number"`)
}

func TestAllOfUnmarshalRejectsNestedAdditionalPropertiesConflict(t *testing.T) {
	content := []byte(`
allOf:
  - type: object
    properties:
      child:
        type: object
        additionalProperties:
          type: string
  - type: object
    properties:
      child:
        type: object
        additionalProperties:
          type: number
`)

	var node SchemaNode
	err := yaml.Unmarshal(content, &node)

	require.ErrorContains(t, err, "merge allOf schema 2")
	require.ErrorContains(t, err, `property "child"`)
	require.ErrorContains(t, err, "additionalProperties")
	require.ErrorContains(t, err, `cannot merge schema type "string" with "number"`)
}

func TestAllOfUnmarshalRejectsEmptyAllOf(t *testing.T) {
	content := []byte(`
allOf: []
`)

	var node SchemaNode
	err := yaml.Unmarshal(content, &node)

	require.ErrorContains(t, err, "allOf must contain at least one schema")
}

func allOfThreeRequiredPropertiesSchema() []byte {
	return []byte(`
allOf:
  - type: object
    required:
      - first
    properties:
      first:
        type: string
        nullable: false
  - type: object
    required:
      - second
    properties:
      second:
        type: boolean
        nullable: false
  - type: object
    required:
      - last
    properties:
      last:
        type: number
        nullable: false
`)
}
