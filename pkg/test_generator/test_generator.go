// Package testgenerator parses OpenAPI request schemas for generated decoder tests.
package testgenerator

import (
	"encoding/json"
	"fmt"
)

// GenerateValid validates that operationID has a request schema.
// Payload generation and decoder invocation are not implemented yet.
func GenerateValid(openAPIYAMLSpec []byte, operationID string, _ func([]byte) error) error {
	_, err := parseOpenAPIRequestBodySchemaNode(openAPIYAMLSpec, operationID)
	if err != nil {
		return err
	}

	return nil
}

// GenerateInvalid validates that operationID has a request schema.
// Payload generation and decoder invocation are not implemented yet.
func GenerateInvalid(openAPIYAMLSpec []byte, operationID string, _ func([]byte) error) error {
	_, err := parseOpenAPIRequestBodySchemaNode(openAPIYAMLSpec, operationID)
	if err != nil {
		return err
	}

	return nil
}

// parseOpenAPIRequestBodySchemaNode converts the document to JSON and finds its request schema.
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
