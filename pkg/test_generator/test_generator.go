package testgenerator

import "fmt"

func GenerateValid(openAPIYAMLSpec []byte, operationID string, unmarshal func([]byte) error) error {
	_ = unmarshal

	openAPIJSONSpec, err := YAMLBytesToJSONRawMessage(openAPIYAMLSpec)
	if err != nil {
		return fmt.Errorf("openapi yaml spec parse failed: %w", err)
	}

	_, err = OpenAPIRequestBodySchemaNode(openAPIJSONSpec, operationID)
	if err != nil {
		return fmt.Errorf("openapi yaml spec parse failed: %w", err)
	}

	return nil
}

func GenerateInvalid(openAPIYAMLSpec []byte, operationID string, unmarshal func([]byte) error) error {
	_ = unmarshal

	openAPIJSONSpec, err := YAMLBytesToJSONRawMessage(openAPIYAMLSpec)
	if err != nil {
		return fmt.Errorf("openapi yaml spec parse failed: %w", err)
	}

	_, err = OpenAPIRequestBodySchemaNode(openAPIJSONSpec, operationID)
	if err != nil {
		return fmt.Errorf("openapi yaml spec parse failed: %w", err)
	}

	return nil
}
