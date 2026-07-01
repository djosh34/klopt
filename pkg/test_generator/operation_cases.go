package testgenerator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func RunJSONRequestBodyOperationCases(t *testing.T, openAPI []byte, operationID string, unmarshal func([]byte) error) {
	t.Helper()

	if unmarshal == nil {
		t.Fatal("nil unmarshal function")
	}

	schemaNode, err := jsonRequestBodySchemaNode(openAPI, operationID)
	require.NoError(t, err)

	var schema SchemaNode
	err = schemaNode.Decode(&schema)
	require.NoError(t, err)

	t.Run("valid", func(t *testing.T) {
		for _, testCase := range schema.ValidCases() {
			t.Run(testCase.Name, func(t *testing.T) {
				err := unmarshal(testCase.Value)
				if err != nil {
					t.Fatalf("expected valid case %q to decode: %v", testCase.Value, err)
				}
			})
		}
	})

	t.Run("invalid", func(t *testing.T) {
		for _, testCase := range schema.InvalidCases() {
			t.Run(testCase.Name, func(t *testing.T) {
				err := unmarshal(testCase.Value)
				if err == nil {
					t.Fatalf("expected invalid case %q to fail", testCase.Value)
				}
			})
		}
	})
}

func jsonRequestBodySchemaNode(openAPI []byte, operationID string) (*yaml.Node, error) {
	var document yaml.Node
	err := yaml.Unmarshal(openAPI, &document)
	if err != nil {
		return nil, fmt.Errorf("unmarshal openapi yaml: %w", err)
	}

	if len(document.Content) != 1 {
		return nil, fmt.Errorf("openapi yaml must contain one document")
	}

	pathsNode := operationMappingValue(document.Content[0], "paths")
	if pathsNode == nil {
		return nil, fmt.Errorf("openapi document has no paths")
	}

	operationNode, err := operationNodeByID(pathsNode, operationID)
	if err != nil {
		return nil, err
	}

	requestBodyNode := operationMappingValue(operationNode, "requestBody")
	if requestBodyNode == nil {
		return nil, fmt.Errorf("operation %q has no requestBody", operationID)
	}

	contentNode := operationMappingValue(requestBodyNode, "content")
	if contentNode == nil {
		return nil, fmt.Errorf("operation %q requestBody has no content", operationID)
	}

	jsonNode := operationMappingValue(contentNode, "application/json")
	if jsonNode == nil {
		return nil, fmt.Errorf("operation %q requestBody has no application/json content", operationID)
	}

	schemaNode := operationMappingValue(jsonNode, "schema")
	if schemaNode == nil {
		return nil, fmt.Errorf("operation %q application/json content has no schema", operationID)
	}

	return schemaNode, nil
}

func operationNodeByID(pathsNode *yaml.Node, operationID string) (*yaml.Node, error) {
	var found *yaml.Node
	for i := 0; i < len(pathsNode.Content)-1; i += 2 {
		pathItemNode := pathsNode.Content[i+1]
		if pathItemNode.Kind != yaml.MappingNode {
			continue
		}

		for j := 0; j < len(pathItemNode.Content)-1; j += 2 {
			method := pathItemNode.Content[j].Value
			if !isOpenAPIOperationMethod(method) {
				continue
			}

			operationNode := pathItemNode.Content[j+1]
			operationIDNode := operationMappingValue(operationNode, "operationId")
			if operationIDNode == nil || operationIDNode.Value != operationID {
				continue
			}

			if found != nil {
				return nil, fmt.Errorf("duplicate operationId %q", operationID)
			}
			found = operationNode
		}
	}

	if found == nil {
		return nil, fmt.Errorf("operationId %q not found", operationID)
	}

	return found, nil
}

func isOpenAPIOperationMethod(method string) bool {
	switch method {
	case "delete", "get", "head", "options", "patch", "post", "put", "trace":
		return true
	default:
		return false
	}
}

func operationMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}

	return nil
}
