package example

import (
	"testing"

	"decode_and_validate_generator/pkg/test_generator"
)

var exampleOpenAPI = []byte(`
openapi: 3.0.3
info:
  title: Decode Example
  version: 1.0.0
paths:
  /optional-array-nullable:
    post:
      operationId: optionalArrayNullable
      requestBody:
        required: false
        content:
          application/json:
            schema:
              type: array
              nullable: true
              items:
                type: string
                nullable: false
      responses:
        '204':
          description: No Content
  /array-nullable:
    post:
      operationId: arrayNullable
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: array
              nullable: true
              items:
                type: string
                nullable: false
      responses:
        '204':
          description: No Content
  /array-not-nullable:
    post:
      operationId: arrayNotNullable
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: array
              nullable: false
              items:
                type: string
                nullable: false
      responses:
        '204':
          description: No Content
  /object-keys-additional-properties-false:
    post:
      operationId: objectKeysAdditionalPropertiesFalse
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              nullable: false
              required:
                - requiredNullableString
                - requiredNotNullableString
              additionalProperties: false
              properties:
                requiredNullableString:
                  type: string
                  nullable: true
                requiredNotNullableString:
                  type: string
                  nullable: false
                optionalNullableString:
                  type: string
                  nullable: true
                optionalNotNullableString:
                  type: string
                  nullable: false
      responses:
        '204':
          description: No Content
`)

func TestOptionalArrayNullable(t *testing.T) {
	testgenerator.RunJSONRequestBodyOperationCases(t, exampleOpenAPI, "optionalArrayNullable", func(data []byte) error {
		var value OptionalArrayNullable
		return value.UnmarshalJSON(data)
	})
}

func TestArrayNullable(t *testing.T) {
	testgenerator.RunJSONRequestBodyOperationCases(t, exampleOpenAPI, "arrayNullable", func(data []byte) error {
		var value ArrayNullable
		return value.UnmarshalJSON(data)
	})
}

func TestArrayNotNullable(t *testing.T) {
	testgenerator.RunJSONRequestBodyOperationCases(t, exampleOpenAPI, "arrayNotNullable", func(data []byte) error {
		var value ArrayNotNullable
		return value.UnmarshalJSON(data)
	})
}

func TestObjectKeysAdditionalPropertiesFalse(t *testing.T) {
	testgenerator.RunJSONRequestBodyOperationCases(t, exampleOpenAPI, "objectKeysAdditionalPropertiesFalse", func(data []byte) error {
		var value ObjectKeysAdditionalPropertiesFalse
		return value.UnmarshalJSON(data)
	})
}
