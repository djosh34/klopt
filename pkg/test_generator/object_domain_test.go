package testgenerator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseObjectWithAllObjectSchemaFields(t *testing.T) {
	const objectSchemaYAML = `
type: object
required:
  - name
properties:
  name:
    type: string
additionalProperties:
  type: string
minProperties: 1
maxProperties: 3
`

	node, err := YAMLBytesToJSONRawMessage([]byte(objectSchemaYAML))
	require.NoError(t, err)

	dc := DomainContext{}
	_, err = dc.ParseObject(node)
	require.NoError(t, err)
}
