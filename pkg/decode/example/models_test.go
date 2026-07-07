package example

import (
	"testing"

	"decode_and_validate_generator/pkg/test_generator"

	"github.com/stretchr/testify/require"
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
                stringFormatNotNullable:
                  type: string
                  format: date-time
                  nullable: false
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
	err := testgenerator.GenerateValid(exampleOpenAPI, "stringNoFormatNullable", func(data []byte) error {
		var value StringNoFormatNullable

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestStringNoFormatNotNullable(t *testing.T) {
	err := testgenerator.GenerateValid(exampleOpenAPI, "stringNoFormatNotNullable", func(data []byte) error {
		var value StringNoFormatNotNullable

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPut(t *testing.T) {
	err := testgenerator.GenerateValid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPut

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObject(t *testing.T) {
	err := testgenerator.GenerateValid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObject

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefObject(t *testing.T) {
	err := testgenerator.GenerateValid(exampleOpenAPI, "refObject", func(data []byte) error {
		var value RefObject

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestOptionalArrayNullable(t *testing.T) {
	err := testgenerator.GenerateValid(exampleOpenAPI, "optionalArrayNullable", func(data []byte) error {
		var value OptionalArrayNullable

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestObjectKeysAdditionalPropertiesFalse(t *testing.T) {
	err := testgenerator.GenerateValid(exampleOpenAPI, "objectKeysAdditionalPropertiesFalse", func(data []byte) error {
		var value ObjectKeysAdditionalPropertiesFalse

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestNullableObjectKeysAdditionalPropertiesFalse(t *testing.T) {
	err := testgenerator.GenerateValid(exampleOpenAPI, "nullableObjectKeysAdditionalPropertiesFalse", func(data []byte) error {
		var value NullableObjectKeysAdditionalPropertiesFalse

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestCompositeObject(t *testing.T) {
	err := testgenerator.GenerateValid(exampleOpenAPI, "compositeObject", func(data []byte) error {
		var value CompositeObject

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestArrayNullable(t *testing.T) {
	err := testgenerator.GenerateValid(exampleOpenAPI, "arrayNullable", func(data []byte) error {
		var value ArrayNullable

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestArrayNotNullable(t *testing.T) {
	err := testgenerator.GenerateValid(exampleOpenAPI, "arrayNotNullable", func(data []byte) error {
		var value ArrayNotNullable

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestAllOfObject(t *testing.T) {
	err := testgenerator.GenerateValid(exampleOpenAPI, "allOfObject", func(data []byte) error {
		var value AllOfObject

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf1MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf1

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf1NestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf1Nested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf2AllOf1AllOf1MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf2AllOf1AllOf1

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf2AllOf1AllOf1Nested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf2AllOf1AllOf2MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf2AllOf1AllOf2

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf2AllOf2MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf2AllOf2

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SealedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3Sealed

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf3MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf3

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf3FinalMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf3Final

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf3FinalNestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf3FinalNested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf3NestedAllOf1MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf3NestedAllOf1

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf3NestedAllOf2MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf3NestedAllOf2

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf3NestedAllOf3MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf3NestedAllOf3

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf1AllOf3NestedAllOf3SealedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf1AllOf3NestedAllOf3Sealed

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf2AllOf1AllOf1MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf2AllOf1AllOf1

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf2AllOf1AllOf1NestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf2AllOf1AllOf1Nested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf2AllOf1AllOf2MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf2AllOf1AllOf2

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf2AllOf1AllOf2FinalMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf2AllOf1AllOf2Final

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf2AllOf1AllOf2FinalNestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf2AllOf1AllOf2FinalNested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf2AllOf1AllOf2MetadataMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf2AllOf1AllOf2Metadata

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf2AllOf2MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf2AllOf2

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf2AllOf2FinalsItemMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf2AllOf2FinalsItem

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf2AllOf2FinalsItemNestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf2AllOf2FinalsItemNested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf2AllOf2MetadataMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf2AllOf2Metadata

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf3MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf3

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf3FinalMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf3Final

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf3FinalNestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf3FinalNested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf3FinalsItemMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf3FinalsItem

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf3FinalsItemNestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf3FinalsItemNested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf3MetadataMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf3Metadata

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf3NestedAllOf1MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf3NestedAllOf1

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf3NestedAllOf2MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf3NestedAllOf2

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf3NestedAllOf3MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf3NestedAllOf3

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectPutAllOf3NestedAllOf3SealedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObjectPut", func(data []byte) error {
		var value RefStressObjectPutAllOf3NestedAllOf3Sealed

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf1MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf1

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf1NestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf1Nested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf2AllOf1AllOf1MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf2AllOf1AllOf1

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf2AllOf1AllOf1NestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf2AllOf1AllOf1Nested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf2AllOf1AllOf2MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf2AllOf1AllOf2

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf2AllOf2MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf2AllOf2

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf2AllOf2NestedAllOf1MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf2AllOf2NestedAllOf2MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf2AllOf2NestedAllOf3MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SealedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3Sealed

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf3MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf3

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf3FinalMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf3Final

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf3FinalNestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf3FinalNested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf3NestedAllOf1MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf3NestedAllOf1

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf3NestedAllOf2MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf3NestedAllOf2

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf3NestedAllOf3MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf3NestedAllOf3

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf1AllOf3NestedAllOf3SealedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf1AllOf3NestedAllOf3Sealed

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf2AllOf1AllOf1MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf2AllOf1AllOf1

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf2AllOf1AllOf1NestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf2AllOf1AllOf1Nested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf2AllOf1AllOf2MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf2AllOf1AllOf2

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf2AllOf1AllOf2FinalMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf2AllOf1AllOf2Final

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf2AllOf1AllOf2FinalNestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf2AllOf1AllOf2FinalNested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf2AllOf1AllOf2MetadataMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf2AllOf1AllOf2Metadata

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf2AllOf2MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf2AllOf2

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf2AllOf2FinalsItemMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf2AllOf2FinalsItem

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf2AllOf2FinalsItemNestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf2AllOf2FinalsItemNested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf2AllOf2MetadataMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf2AllOf2Metadata

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf3MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf3

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf3FinalMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf3Final

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf3FinalNestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf3FinalNested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf3FinalsItemMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf3FinalsItem

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf3FinalsItemNestedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf3FinalsItemNested

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf3MetadataMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf3Metadata

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf3NestedAllOf1MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf3NestedAllOf1

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf3NestedAllOf2MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf3NestedAllOf2

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf3NestedAllOf3MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf3NestedAllOf3

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefStressObjectAllOf3NestedAllOf3SealedMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refStressObject", func(data []byte) error {
		var value RefStressObjectAllOf3NestedAllOf3Sealed

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestRefObjectMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "refObject", func(data []byte) error {
		var value RefObject

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestObjectKeysAdditionalPropertiesFalseMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "objectKeysAdditionalPropertiesFalse", func(data []byte) error {
		var value ObjectKeysAdditionalPropertiesFalse

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestNullableObjectKeysAdditionalPropertiesFalseMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "nullableObjectKeysAdditionalPropertiesFalse", func(data []byte) error {
		var value NullableObjectKeysAdditionalPropertiesFalse

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestCompositeObjectMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "compositeObject", func(data []byte) error {
		var value CompositeObject

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestCompositeObjectObjectAdditionalPropertiesImplicitMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "compositeObject", func(data []byte) error {
		var value CompositeObjectObjectAdditionalPropertiesImplicit

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestCompositeObjectObjectAdditionalPropertiesSchemaMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "compositeObject", func(data []byte) error {
		var value CompositeObjectObjectAdditionalPropertiesSchema

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestCompositeObjectObjectAdditionalPropertiesTrueMalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "compositeObject", func(data []byte) error {
		var value CompositeObjectObjectAdditionalPropertiesTrue

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestAllOfObjectAllOf1MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "allOfObject", func(data []byte) error {
		var value AllOfObjectAllOf1

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestAllOfObjectAllOf2MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "allOfObject", func(data []byte) error {
		var value AllOfObjectAllOf2

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}

func TestAllOfObjectAllOf3MalformedObjectJSON(t *testing.T) {
	err := testgenerator.GenerateInvalid(exampleOpenAPI, "allOfObject", func(data []byte) error {
		var value AllOfObjectAllOf3

		return value.UnmarshalJSON(data)
	})
	require.NoError(t, err)
}
