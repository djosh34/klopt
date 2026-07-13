package example

import (
	"testing"

	"decode_and_validate_generator/pkg/test_generator"
)

var exampleOpenAPI = []byte(`
openapi: 3.0.3
info:
  title: Request Body Shape Test
  version: 1.0.0

paths:
  /all-of-object:
    post:
      operationId: allOfObject
      requestBody:
        required: true
        content:
          application/json:
            schema:
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
      responses:
        '204':
          description: No Content

  /composite-object:
    post:
      operationId: compositeObject
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              nullable: false
              required:
                - arrayNullableItemsNullable
                - arrayNullableItemsNotNullable
                - arrayNotNullableItemsNullable
                - arrayNotNullableItemsNotNullable
                - objectAdditionalPropertiesTrue
                - objectAdditionalPropertiesSchema
                - objectAdditionalPropertiesImplicit
                - stringFormatNullable
                - stringFormatNotNullable
                - numberNullable
                - numberNotNullable
                - boolNullable
                - boolNotNullable
              additionalProperties: false
              properties:
                arrayNullableItemsNullable:
                  type: array
                  nullable: true
                  items:
                    type: string
                    nullable: true
                arrayNullableItemsNotNullable:
                  type: array
                  nullable: true
                  items:
                    type: string
                    nullable: false
                arrayNotNullableItemsNullable:
                  type: array
                  nullable: false
                  items:
                    type: string
                    nullable: true
                arrayNotNullableItemsNotNullable:
                  type: array
                  nullable: false
                  items:
                    type: string
                    nullable: false
                objectAdditionalPropertiesTrue:
                  type: object
                  nullable: false
                  additionalProperties: true
                  properties:
                    known:
                      type: string
                      nullable: false
                objectAdditionalPropertiesSchema:
                  type: object
                  nullable: false
                  additionalProperties:
                    type: string
                    nullable: false
                  properties:
                    known:
                      type: string
                      nullable: false
                objectAdditionalPropertiesImplicit:
                  type: object
                  nullable: false
                  properties:
                    known:
                      type: string
                      nullable: false
                stringFormatNullable:
                  type: string
                  format: date-time
                  nullable: true
                  x-valid-examples: ['2024-01-02T03:04:05Z']
                  x-invalid-examples: [not-a-date-time]
                stringFormatNotNullable:
                  type: string
                  format: date-time
                  nullable: false
                  x-valid-examples: ['2024-01-02T03:04:05Z']
                  x-invalid-examples: [not-a-date-time]
                numberNullable:
                  type: number
                  nullable: true
                numberNotNullable:
                  type: number
                  nullable: false
                boolNullable:
                  type: boolean
                  nullable: true
                boolNotNullable:
                  type: boolean
                  nullable: false
      responses:
        '204':
          description: No Content
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

  /nullable-object-keys-additional-properties-false:
    post:
      operationId: nullableObjectKeysAdditionalPropertiesFalse
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              nullable: true
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

  /string-no-format-nullable:
    post:
      operationId: stringNoFormatNullable
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: string
              nullable: true
      responses:
        '204':
          description: No Content

  /string-no-format-not-nullable:
    post:
      operationId: stringNoFormatNotNullable
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: string
              nullable: false
      responses:
        '204':
          description: No Content

  /ref-object:
    post:
      operationId: refObject
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/RefObjectRequest'
      responses:
        '204':
          description: No Content

  /ref-stress-object:
    post:
      operationId: refStressObject
      requestBody:
        required: true
        content:
          application/json:
            schema:
              allOf:
                - $ref: '#/components/schemas/RefStressFirstAllOf'
                - $ref: '#/components/schemas/RefStressSecondAllOf'
                - type: object
                  nullable: false
                  required:
                    - finalCode
                    - sharedName
                    - middleFlag
                    - rootFlag
                    - count
                    - nested
                    - final
                    - finals
                    - metadata
                    - nullableRequired
                  additionalProperties: false
                  properties:
                    finalCode:
                      type: string
                      nullable: false
                    sharedName:
                      type: string
                      nullable: false
                    middleFlag:
                      type: boolean
                      nullable: false
                    rootFlag:
                      type: boolean
                      nullable: false
                    count:
                      type: number
                      nullable: false
                    nested:
                      $ref: '#/components/schemas/RefStressNestedCombined'
                    final:
                      $ref: '#/components/schemas/RefStressFinalAlias'
                    finals:
                      type: array
                      nullable: false
                      items:
                        $ref: '#/components/schemas/RefStressFinalAlias'
                    metadata:
                      type: object
                      nullable: false
                      additionalProperties:
                        $ref: '#/components/schemas/RefStressMetadataValueAlias'
                    nullableRequired:
                      type: string
                      nullable: true
                    optionalShared:
                      type: string
                      nullable: true
                    optionalCode:
                      type: string
                      nullable: false
      responses:
        '204':
          description: No Content

  /ref-stress-object-put:
    put:
      operationId: refStressObjectPut
      requestBody:
        required: true
        content:
          application/json:
            schema:
              allOf:
                - $ref: '#/components/schemas/RefStressFirstAllOf'
                - $ref: '#/components/schemas/RefStressSecondAllOf'
                - type: object
                  nullable: false
                  required:
                    - finalCode
                    - sharedName
                    - middleFlag
                    - rootFlag
                    - count
                    - nested
                    - final
                    - finals
                    - metadata
                    - nullableRequired
                  additionalProperties: false
                  properties:
                    finalCode:
                      type: string
                      nullable: false
                    sharedName:
                      type: string
                      nullable: false
                    middleFlag:
                      type: boolean
                      nullable: false
                    rootFlag:
                      type: boolean
                      nullable: false
                    count:
                      type: number
                      nullable: false
                    nested:
                      $ref: '#/components/schemas/RefStressNestedCombined'
                    final:
                      $ref: '#/components/schemas/RefStressFinalAlias'
                    finals:
                      type: array
                      nullable: false
                      items:
                        $ref: '#/components/schemas/RefStressFinalAlias'
                    metadata:
                      type: object
                      nullable: false
                      additionalProperties:
                        $ref: '#/components/schemas/RefStressMetadataValueAlias'
                    nullableRequired:
                      type: string
                      nullable: true
                    optionalShared:
                      type: string
                      nullable: true
                    optionalCode:
                      type: string
                      nullable: false
      responses:
        '204':
          description: No Content
components:
  schemas:
    RefObjectRequest:
      type: object
      nullable: false
      required:
        - refRequiredString
      additionalProperties: false
      properties:
        refRequiredString:
          type: string
          nullable: false
        refOptionalBool:
          type: boolean
          nullable: true

    RefStressFinalAlias:
      $ref: '#/components/schemas/RefStressFinal'

    RefStressMetadataValueAlias:
      $ref: '#/components/schemas/RefStressMetadataValue'

    RefStressFirstAllOf:
      allOf:
        - $ref: '#/components/schemas/RefStressFinal'
        - $ref: '#/components/schemas/RefStressViaMiddle'
        - type: object
          nullable: false
          required:
            - final
            - nested
            - nullableRequired
          properties:
            sharedName:
              type: string
              nullable: true
            final:
              $ref: '#/components/schemas/RefStressFinalAlias'
            nested:
              $ref: '#/components/schemas/RefStressNestedCombined'
            nullableRequired:
              type: string
              nullable: true
            optionalShared:
              type: string
              nullable: true

    RefStressViaMiddle:
      allOf:
        - $ref: '#/components/schemas/RefStressMiddleRef'
        - type: object
          nullable: false
          required:
            - middleFlag
            - sharedName
          properties:
            middleFlag:
              type: boolean
              nullable: false
            sharedName:
              type: string
              nullable: true
            nested:
              $ref: '#/components/schemas/RefStressNestedAlias'

    RefStressMiddleRef:
      $ref: '#/components/schemas/RefStressMiddleAllOf'

    RefStressMiddleAllOf:
      allOf:
        - $ref: '#/components/schemas/RefStressFinalAlias'
        - type: object
          nullable: true
          required:
            - sharedName
          properties:
            sharedName:
              type: string
              nullable: false
            optionalCode:
              type: string
              nullable: false

    RefStressSecondAllOf:
      allOf:
        - $ref: '#/components/schemas/RefStressOtherMiddle'
        - type: object
          nullable: false
          required:
            - rootFlag
            - count
            - finals
            - metadata
          properties:
            rootFlag:
              type: boolean
              nullable: false
            count:
              type: number
              nullable: false
            sharedName:
              type: string
              nullable: false
            finals:
              type: array
              nullable: false
              items:
                $ref: '#/components/schemas/RefStressFinalAlias'
            metadata:
              type: object
              nullable: false
              additionalProperties:
                $ref: '#/components/schemas/RefStressMetadataValue'

    RefStressOtherMiddle:
      allOf:
        - $ref: '#/components/schemas/RefStressFinalAlias'
        - type: object
          nullable: false
          required:
            - rootFlag
            - metadata
          properties:
            rootFlag:
              type: boolean
              nullable: false
            metadata:
              type: object
              nullable: false
              additionalProperties:
                $ref: '#/components/schemas/RefStressMetadataValueAlias'
            final:
              $ref: '#/components/schemas/RefStressFinalAlias'

    RefStressFinal:
      type: object
      nullable: false
      required:
        - finalCode
        - sharedName
      properties:
        finalCode:
          type: string
          nullable: false
        sharedName:
          type: string
          nullable: false
        nested:
          $ref: '#/components/schemas/RefStressNestedBase'
        optionalShared:
          type: string
          nullable: true

    RefStressNestedAlias:
      $ref: '#/components/schemas/RefStressNestedCombined'

    RefStressNestedCombined:
      allOf:
        - $ref: '#/components/schemas/RefStressNestedBase'
        - $ref: '#/components/schemas/RefStressNestedOverlay'
        - type: object
          nullable: false
          required:
            - sameName
            - sealed
          properties:
            sameName:
              type: string
              nullable: false
            sealed:
              type: object
              nullable: false
              required:
                - locked
              additionalProperties: false
              properties:
                locked:
                  type: boolean
                  nullable: false

    RefStressNestedBase:
      type: object
      nullable: true
      required:
        - sameName
      properties:
        sameName:
          type: string
          nullable: true
        leaf:
          $ref: '#/components/schemas/RefStressMetadataValue'

    RefStressNestedOverlay:
      type: object
      nullable: false
      required:
        - sameName
      properties:
        sameName:
          type: string
          nullable: false
        leaf:
          $ref: '#/components/schemas/RefStressMetadataValueAlias'

    RefStressMetadataValue:
      type: string
      nullable: false
`)

func TestStringNoFormatNullable(t *testing.T) {
	testgenerator.CheckJSONRequestBody(t, exampleOpenAPI, "stringNoFormatNullable", func(data []byte) error {
		var value StringNoFormatNullable
		return value.UnmarshalJSON(data)
	}, testgenerator.DefaultOption)
}

func TestStringNoFormatNotNullable(t *testing.T) {
	testgenerator.CheckJSONRequestBody(t, exampleOpenAPI, "stringNoFormatNotNullable", func(data []byte) error {
		var value StringNoFormatNotNullable
		return value.UnmarshalJSON(data)
	}, testgenerator.DefaultOption)
}

func TestRefStressObjectPut(t *testing.T) {
	testgenerator.CheckJSONRequestBody(t, exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPut
		return value.UnmarshalJSON(data)
	}, testgenerator.DefaultOption)
}

func TestRefStressObject(t *testing.T) {
	testgenerator.CheckJSONRequestBody(t, exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObject
		return value.UnmarshalJSON(data)
	}, testgenerator.DefaultOption)
}

func TestRefObject(t *testing.T) {
	testgenerator.CheckJSONRequestBody(t, exampleOpenAPI, "refObject", func(data []byte) error {
		var value RefObject
		return value.UnmarshalJSON(data)
	}, testgenerator.DefaultOption)
}

func TestOptionalArrayNullable(t *testing.T) {
	testgenerator.CheckJSONRequestBody(t, exampleOpenAPI, "optionalArrayNullable", func(data []byte) error {
		var value OptionalArrayNullable
		return value.UnmarshalJSON(data)
	}, testgenerator.DefaultOption)
}

func TestObjectKeysAdditionalPropertiesFalse(t *testing.T) {
	testgenerator.CheckJSONRequestBody(t, exampleOpenAPI, "objectKeysAdditionalPropertiesFalse", func(data []byte) error {
		var value ObjectKeysAdditionalPropertiesFalse
		return value.UnmarshalJSON(data)
	}, testgenerator.DefaultOption)
}

func TestNullableObjectKeysAdditionalPropertiesFalse(t *testing.T) {
	testgenerator.CheckJSONRequestBody(t, exampleOpenAPI, "nullableObjectKeysAdditionalPropertiesFalse", func(data []byte) error {
		var value NullableObjectKeysAdditionalPropertiesFalse
		return value.UnmarshalJSON(data)
	}, testgenerator.DefaultOption)
}

func TestCompositeObject(t *testing.T) {
	testgenerator.CheckJSONRequestBody(t, exampleOpenAPI, "compositeObject", func(data []byte) error {
		var value CompositeObject
		return value.UnmarshalJSON(data)
	}, testgenerator.DefaultOption)
}

func TestArrayNullable(t *testing.T) {
	testgenerator.CheckJSONRequestBody(t, exampleOpenAPI, "arrayNullable", func(data []byte) error {
		var value ArrayNullable
		return value.UnmarshalJSON(data)
	}, testgenerator.DefaultOption)
}

func TestArrayNotNullable(t *testing.T) {
	testgenerator.CheckJSONRequestBody(t, exampleOpenAPI, "arrayNotNullable", func(data []byte) error {
		var value ArrayNotNullable
		return value.UnmarshalJSON(data)
	}, testgenerator.DefaultOption)
}

func TestAllOfObject(t *testing.T) {
	testgenerator.CheckJSONRequestBody(t, exampleOpenAPI, "allOfObject", func(data []byte) error {
		var value AllOfObject
		return value.UnmarshalJSON(data)
	}, testgenerator.DefaultOption)
}
