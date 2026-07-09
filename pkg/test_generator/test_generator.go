package testgenerator

import (
	"encoding/json"
	"fmt"
)

func GenerateValid(openAPIYAMLSpec []byte, operationID string, unmarshal func([]byte) error) error {
	_ = unmarshal

	_, err := parseOpenAPIRequestBodySchemaNode(openAPIYAMLSpec, operationID)
	if err != nil {
		return err
	}

	return nil
}

func GenerateInvalid(openAPIYAMLSpec []byte, operationID string, unmarshal func([]byte) error) error {
	_ = unmarshal

	_, err := parseOpenAPIRequestBodySchemaNode(openAPIYAMLSpec, operationID)
	if err != nil {
		return err
	}

	return nil
}

func parseOpenAPIRequestBodySchemaNode(openAPIYAMLSpec []byte, operationID string) (*json.RawMessage, error) {
	openAPIJSONSpec, err := YAMLBytesToJSONRawMessage(openAPIYAMLSpec)
	if err != nil {
		return nil, fmt.Errorf("openapi yaml spec parse failed: %w", err)
	}

	schemaNode, err := OpenAPIRequestBodySchemaNode(openAPIJSONSpec, operationID)
	if err != nil {
		return nil, fmt.Errorf("openapi request body schema lookup failed: %w", err)
	}

	return schemaNode, nil
}
