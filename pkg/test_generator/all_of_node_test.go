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
