package testgenerator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

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

	node, err := YAMLBytesToJSONRawMessage([]byte(objectSchemaYAML))
	require.NoError(t, err)

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
