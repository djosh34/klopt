// Package example contains generated request-body validations.
//
//nolint:dupl,lll // Generated validation graphs may contain repeated schemas and long pointers.
package example

import (
	"github.com/djosh34/decode_and_validate_generator/pkg/validation"
)

// allOfObject is a compiled request-body validation.
var allOfObject = func() *validation.Validation {
	validations := []*validation.Validation{
		{SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema", BodyRequired: true, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/0", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"first"}, Properties: []validation.PropertyValidation{{Name: "first"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/0/properties/first", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/1", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"second"}, Properties: []validation.PropertyValidation{{Name: "second"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/1/properties/second", KindValidation: validation.KindValidation{Type: "boolean"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/2", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"last"}, Properties: []validation.PropertyValidation{{Name: "last"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/2/properties/last", KindValidation: validation.KindValidation{Type: "number"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
	}

	validations[0].AllOfValidations = append(validations[0].AllOfValidations, validations[1])
	validations[0].AllOfValidations = append(validations[0].AllOfValidations, validations[3])
	validations[0].AllOfValidations = append(validations[0].AllOfValidations, validations[5])
	validations[1].ObjectValidation.Properties[0].Validation = validations[2]
	validations[3].ObjectValidation.Properties[0].Validation = validations[4]
	validations[5].ObjectValidation.Properties[0].Validation = validations[6]

	return validations[0]
}()

// arrayNotNullable is a compiled request-body validation.
var arrayNotNullable = func() *validation.Validation {
	validations := []*validation.Validation{
		{SchemaPointer: "#/paths/~1array-not-nullable/post/requestBody/content/application~1json/schema", BodyRequired: true, KindValidation: validation.KindValidation{Type: "array"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1array-not-nullable/post/requestBody/content/application~1json/schema/items", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
	}

	validations[0].ArrayValidation.Items = validations[1]

	return validations[0]
}()

// arrayNullable is a compiled request-body validation.
var arrayNullable = func() *validation.Validation {
	validations := []*validation.Validation{
		{SchemaPointer: "#/paths/~1array-nullable/post/requestBody/content/application~1json/schema", BodyRequired: true, KindValidation: validation.KindValidation{Type: "array", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1array-nullable/post/requestBody/content/application~1json/schema/items", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
	}

	validations[0].ArrayValidation.Items = validations[1]

	return validations[0]
}()

// compositeObject is a compiled request-body validation.
var compositeObject = func() *validation.Validation {
	validations := []*validation.Validation{
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema", BodyRequired: true, KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"arrayNotNullableItemsNotNullable", "arrayNotNullableItemsNullable", "arrayNullableItemsNotNullable", "arrayNullableItemsNullable", "boolNotNullable", "boolNullable", "numberNotNullable", "numberNullable", "objectAdditionalPropertiesImplicit", "objectAdditionalPropertiesSchema", "objectAdditionalPropertiesTrue", "stringFormatNotNullable", "stringFormatNullable"}, Properties: []validation.PropertyValidation{{Name: "arrayNotNullableItemsNotNullable"}, {Name: "arrayNotNullableItemsNullable"}, {Name: "arrayNullableItemsNotNullable"}, {Name: "arrayNullableItemsNullable"}, {Name: "boolNotNullable"}, {Name: "boolNullable"}, {Name: "numberNotNullable"}, {Name: "numberNullable"}, {Name: "objectAdditionalPropertiesImplicit"}, {Name: "objectAdditionalPropertiesSchema"}, {Name: "objectAdditionalPropertiesTrue"}, {Name: "stringFormatNotNullable"}, {Name: "stringFormatNullable"}}}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNotNullableItemsNotNullable", KindValidation: validation.KindValidation{Type: "array"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNotNullableItemsNotNullable/items", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNotNullableItemsNullable", KindValidation: validation.KindValidation{Type: "array"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNotNullableItemsNullable/items", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNullableItemsNotNullable", KindValidation: validation.KindValidation{Type: "array", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNullableItemsNotNullable/items", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNullableItemsNullable", KindValidation: validation.KindValidation{Type: "array", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNullableItemsNullable/items", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/boolNotNullable", KindValidation: validation.KindValidation{Type: "boolean"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/boolNullable", KindValidation: validation.KindValidation{Type: "boolean", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/numberNotNullable", KindValidation: validation.KindValidation{Type: "number"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/numberNullable", KindValidation: validation.KindValidation{Type: "number", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesImplicit", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Properties: []validation.PropertyValidation{{Name: "known"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesImplicit/properties/known", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesSchema", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Properties: []validation.PropertyValidation{{Name: "known"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesSchema/properties/known", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesSchema/additionalProperties", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesTrue", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Properties: []validation.PropertyValidation{{Name: "known"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesTrue/properties/known", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/stringFormatNotNullable", KindValidation: validation.KindValidation{Type: "string"}, StringValidation: validation.StringValidation{Format: "date-time"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/stringFormatNullable", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, StringValidation: validation.StringValidation{Format: "date-time"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
	}

	validations[0].ObjectValidation.Properties[0].Validation = validations[1]
	validations[0].ObjectValidation.Properties[1].Validation = validations[3]
	validations[0].ObjectValidation.Properties[2].Validation = validations[5]
	validations[0].ObjectValidation.Properties[3].Validation = validations[7]
	validations[0].ObjectValidation.Properties[4].Validation = validations[9]
	validations[0].ObjectValidation.Properties[5].Validation = validations[10]
	validations[0].ObjectValidation.Properties[6].Validation = validations[11]
	validations[0].ObjectValidation.Properties[7].Validation = validations[12]
	validations[0].ObjectValidation.Properties[8].Validation = validations[13]
	validations[0].ObjectValidation.Properties[9].Validation = validations[15]
	validations[0].ObjectValidation.Properties[10].Validation = validations[18]
	validations[0].ObjectValidation.Properties[11].Validation = validations[20]
	validations[0].ObjectValidation.Properties[12].Validation = validations[21]
	validations[1].ArrayValidation.Items = validations[2]
	validations[3].ArrayValidation.Items = validations[4]
	validations[5].ArrayValidation.Items = validations[6]
	validations[7].ArrayValidation.Items = validations[8]
	validations[13].ObjectValidation.Properties[0].Validation = validations[14]
	validations[15].ObjectValidation.Properties[0].Validation = validations[16]
	validations[15].ObjectValidation.AdditionalPropertiesValidation = validations[17]
	validations[18].ObjectValidation.Properties[0].Validation = validations[19]

	return validations[0]
}()

// nullableObjectKeysAdditionalPropertiesFalse is a compiled request-body validation.
var nullableObjectKeysAdditionalPropertiesFalse = func() *validation.Validation {
	validations := []*validation.Validation{
		{SchemaPointer: "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema", BodyRequired: true, KindValidation: validation.KindValidation{Type: "object", Nullable: true}, ObjectValidation: validation.ObjectValidation{Required: []string{"requiredNotNullableString", "requiredNullableString"}, Properties: []validation.PropertyValidation{{Name: "optionalNotNullableString"}, {Name: "optionalNullableString"}, {Name: "requiredNotNullableString"}, {Name: "requiredNullableString"}}}},
		{SchemaPointer: "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNotNullableString", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNullableString", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNotNullableString", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNullableString", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
	}

	validations[0].ObjectValidation.Properties[0].Validation = validations[1]
	validations[0].ObjectValidation.Properties[1].Validation = validations[2]
	validations[0].ObjectValidation.Properties[2].Validation = validations[3]
	validations[0].ObjectValidation.Properties[3].Validation = validations[4]

	return validations[0]
}()

// objectKeysAdditionalPropertiesFalse is a compiled request-body validation.
var objectKeysAdditionalPropertiesFalse = func() *validation.Validation {
	validations := []*validation.Validation{
		{SchemaPointer: "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema", BodyRequired: true, KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"requiredNotNullableString", "requiredNullableString"}, Properties: []validation.PropertyValidation{{Name: "optionalNotNullableString"}, {Name: "optionalNullableString"}, {Name: "requiredNotNullableString"}, {Name: "requiredNullableString"}}}},
		{SchemaPointer: "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNotNullableString", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNullableString", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNotNullableString", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNullableString", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
	}

	validations[0].ObjectValidation.Properties[0].Validation = validations[1]
	validations[0].ObjectValidation.Properties[1].Validation = validations[2]
	validations[0].ObjectValidation.Properties[2].Validation = validations[3]
	validations[0].ObjectValidation.Properties[3].Validation = validations[4]

	return validations[0]
}()

// optionalArrayNullable is a compiled request-body validation.
var optionalArrayNullable = func() *validation.Validation {
	validations := []*validation.Validation{
		{SchemaPointer: "#/paths/~1optional-array-nullable/post/requestBody/content/application~1json/schema", KindValidation: validation.KindValidation{Type: "array", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1optional-array-nullable/post/requestBody/content/application~1json/schema/items", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
	}

	validations[0].ArrayValidation.Items = validations[1]

	return validations[0]
}()

// refObject is a compiled request-body validation.
var refObject = func() *validation.Validation {
	validations := []*validation.Validation{
		{SchemaPointer: "#/components/schemas/RefObjectRequest", BodyRequired: true, KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"refRequiredString"}, Properties: []validation.PropertyValidation{{Name: "refOptionalBool"}, {Name: "refRequiredString"}}}},
		{SchemaPointer: "#/components/schemas/RefObjectRequest/properties/refOptionalBool", KindValidation: validation.KindValidation{Type: "boolean", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefObjectRequest/properties/refRequiredString", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
	}

	validations[0].ObjectValidation.Properties[0].Validation = validations[1]
	validations[0].ObjectValidation.Properties[1].Validation = validations[2]

	return validations[0]
}()

// refStressObject is a compiled request-body validation.
var refStressObject = func() *validation.Validation {
	validations := []*validation.Validation{
		{SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema", BodyRequired: true, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFirstAllOf", ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFinal", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"finalCode", "sharedName"}, Properties: []validation.PropertyValidation{{Name: "finalCode"}, {Name: "nested"}, {Name: "optionalShared"}, {Name: "sharedName"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFinal/properties/finalCode", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedBase", KindValidation: validation.KindValidation{Type: "object", Nullable: true}, ObjectValidation: validation.ObjectValidation{Required: []string{"sameName"}, Properties: []validation.PropertyValidation{{Name: "leaf"}, {Name: "sameName"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressMetadataValue", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedBase/properties/sameName", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFinal/properties/optionalShared", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFinal/properties/sharedName", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressViaMiddle", ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressMiddleAllOf", ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressMiddleAllOf/allOf/1", KindValidation: validation.KindValidation{Type: "object", Nullable: true}, ObjectValidation: validation.ObjectValidation{Required: []string{"sharedName"}, Properties: []validation.PropertyValidation{{Name: "optionalCode"}, {Name: "sharedName"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressMiddleAllOf/allOf/1/properties/optionalCode", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressMiddleAllOf/allOf/1/properties/sharedName", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressViaMiddle/allOf/1", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"middleFlag", "sharedName"}, Properties: []validation.PropertyValidation{{Name: "middleFlag"}, {Name: "nested"}, {Name: "sharedName"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressViaMiddle/allOf/1/properties/middleFlag", KindValidation: validation.KindValidation{Type: "boolean"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedCombined", ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedOverlay", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"sameName"}, Properties: []validation.PropertyValidation{{Name: "leaf"}, {Name: "sameName"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedOverlay/properties/sameName", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"sameName", "sealed"}, Properties: []validation.PropertyValidation{{Name: "sameName"}, {Name: "sealed"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sameName", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"locked"}, Properties: []validation.PropertyValidation{{Name: "locked"}}}},
		{SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed/properties/locked", KindValidation: validation.KindValidation{Type: "boolean"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressViaMiddle/allOf/1/properties/sharedName", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"final", "nested", "nullableRequired"}, Properties: []validation.PropertyValidation{{Name: "final"}, {Name: "nested"}, {Name: "nullableRequired"}, {Name: "optionalShared"}, {Name: "sharedName"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/nullableRequired", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/optionalShared", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/sharedName", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressSecondAllOf", ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressOtherMiddle", ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressOtherMiddle/allOf/1", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"metadata", "rootFlag"}, Properties: []validation.PropertyValidation{{Name: "final"}, {Name: "metadata"}, {Name: "rootFlag"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressOtherMiddle/allOf/1/properties/metadata", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressOtherMiddle/allOf/1/properties/rootFlag", KindValidation: validation.KindValidation{Type: "boolean"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"count", "finals", "metadata", "rootFlag"}, Properties: []validation.PropertyValidation{{Name: "count"}, {Name: "finals"}, {Name: "metadata"}, {Name: "rootFlag"}, {Name: "sharedName"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/count", KindValidation: validation.KindValidation{Type: "number"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/finals", KindValidation: validation.KindValidation{Type: "array"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/metadata", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/rootFlag", KindValidation: validation.KindValidation{Type: "boolean"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/sharedName", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"count", "final", "finalCode", "finals", "metadata", "middleFlag", "nested", "nullableRequired", "rootFlag", "sharedName"}, Properties: []validation.PropertyValidation{{Name: "count"}, {Name: "final"}, {Name: "finalCode"}, {Name: "finals"}, {Name: "metadata"}, {Name: "middleFlag"}, {Name: "nested"}, {Name: "nullableRequired"}, {Name: "optionalCode"}, {Name: "optionalShared"}, {Name: "rootFlag"}, {Name: "sharedName"}}}},
		{SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/count", KindValidation: validation.KindValidation{Type: "number"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/finalCode", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/finals", KindValidation: validation.KindValidation{Type: "array"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/metadata", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/middleFlag", KindValidation: validation.KindValidation{Type: "boolean"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/nullableRequired", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/optionalCode", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/optionalShared", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/rootFlag", KindValidation: validation.KindValidation{Type: "boolean"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/sharedName", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
	}

	validations[0].AllOfValidations = append(validations[0].AllOfValidations, validations[1])
	validations[0].AllOfValidations = append(validations[0].AllOfValidations, validations[28])
	validations[0].AllOfValidations = append(validations[0].AllOfValidations, validations[39])
	validations[1].AllOfValidations = append(validations[1].AllOfValidations, validations[2])
	validations[1].AllOfValidations = append(validations[1].AllOfValidations, validations[9])
	validations[1].AllOfValidations = append(validations[1].AllOfValidations, validations[24])
	validations[2].ObjectValidation.Properties[0].Validation = validations[3]
	validations[2].ObjectValidation.Properties[1].Validation = validations[4]
	validations[2].ObjectValidation.Properties[2].Validation = validations[7]
	validations[2].ObjectValidation.Properties[3].Validation = validations[8]
	validations[4].ObjectValidation.Properties[0].Validation = validations[5]
	validations[4].ObjectValidation.Properties[1].Validation = validations[6]
	validations[9].AllOfValidations = append(validations[9].AllOfValidations, validations[10])
	validations[9].AllOfValidations = append(validations[9].AllOfValidations, validations[14])
	validations[10].AllOfValidations = append(validations[10].AllOfValidations, validations[2])
	validations[10].AllOfValidations = append(validations[10].AllOfValidations, validations[11])
	validations[11].ObjectValidation.Properties[0].Validation = validations[12]
	validations[11].ObjectValidation.Properties[1].Validation = validations[13]
	validations[14].ObjectValidation.Properties[0].Validation = validations[15]
	validations[14].ObjectValidation.Properties[1].Validation = validations[16]
	validations[14].ObjectValidation.Properties[2].Validation = validations[23]
	validations[16].AllOfValidations = append(validations[16].AllOfValidations, validations[4])
	validations[16].AllOfValidations = append(validations[16].AllOfValidations, validations[17])
	validations[16].AllOfValidations = append(validations[16].AllOfValidations, validations[19])
	validations[17].ObjectValidation.Properties[0].Validation = validations[5]
	validations[17].ObjectValidation.Properties[1].Validation = validations[18]
	validations[19].ObjectValidation.Properties[0].Validation = validations[20]
	validations[19].ObjectValidation.Properties[1].Validation = validations[21]
	validations[21].ObjectValidation.Properties[0].Validation = validations[22]
	validations[24].ObjectValidation.Properties[0].Validation = validations[2]
	validations[24].ObjectValidation.Properties[1].Validation = validations[16]
	validations[24].ObjectValidation.Properties[2].Validation = validations[25]
	validations[24].ObjectValidation.Properties[3].Validation = validations[26]
	validations[24].ObjectValidation.Properties[4].Validation = validations[27]
	validations[28].AllOfValidations = append(validations[28].AllOfValidations, validations[29])
	validations[28].AllOfValidations = append(validations[28].AllOfValidations, validations[33])
	validations[29].AllOfValidations = append(validations[29].AllOfValidations, validations[2])
	validations[29].AllOfValidations = append(validations[29].AllOfValidations, validations[30])
	validations[30].ObjectValidation.Properties[0].Validation = validations[2]
	validations[30].ObjectValidation.Properties[1].Validation = validations[31]
	validations[30].ObjectValidation.Properties[2].Validation = validations[32]
	validations[31].ObjectValidation.AdditionalPropertiesValidation = validations[5]
	validations[33].ObjectValidation.Properties[0].Validation = validations[34]
	validations[33].ObjectValidation.Properties[1].Validation = validations[35]
	validations[33].ObjectValidation.Properties[2].Validation = validations[36]
	validations[33].ObjectValidation.Properties[3].Validation = validations[37]
	validations[33].ObjectValidation.Properties[4].Validation = validations[38]
	validations[35].ArrayValidation.Items = validations[2]
	validations[36].ObjectValidation.AdditionalPropertiesValidation = validations[5]
	validations[39].ObjectValidation.Properties[0].Validation = validations[40]
	validations[39].ObjectValidation.Properties[1].Validation = validations[2]
	validations[39].ObjectValidation.Properties[2].Validation = validations[41]
	validations[39].ObjectValidation.Properties[3].Validation = validations[42]
	validations[39].ObjectValidation.Properties[4].Validation = validations[43]
	validations[39].ObjectValidation.Properties[5].Validation = validations[44]
	validations[39].ObjectValidation.Properties[6].Validation = validations[16]
	validations[39].ObjectValidation.Properties[7].Validation = validations[45]
	validations[39].ObjectValidation.Properties[8].Validation = validations[46]
	validations[39].ObjectValidation.Properties[9].Validation = validations[47]
	validations[39].ObjectValidation.Properties[10].Validation = validations[48]
	validations[39].ObjectValidation.Properties[11].Validation = validations[49]
	validations[42].ArrayValidation.Items = validations[2]
	validations[43].ObjectValidation.AdditionalPropertiesValidation = validations[5]

	return validations[0]
}()

// refStressObjectPut is a compiled request-body validation.
var refStressObjectPut = func() *validation.Validation {
	validations := []*validation.Validation{
		{SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema", BodyRequired: true, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFirstAllOf", ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFinal", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"finalCode", "sharedName"}, Properties: []validation.PropertyValidation{{Name: "finalCode"}, {Name: "nested"}, {Name: "optionalShared"}, {Name: "sharedName"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFinal/properties/finalCode", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedBase", KindValidation: validation.KindValidation{Type: "object", Nullable: true}, ObjectValidation: validation.ObjectValidation{Required: []string{"sameName"}, Properties: []validation.PropertyValidation{{Name: "leaf"}, {Name: "sameName"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressMetadataValue", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedBase/properties/sameName", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFinal/properties/optionalShared", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFinal/properties/sharedName", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressViaMiddle", ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressMiddleAllOf", ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressMiddleAllOf/allOf/1", KindValidation: validation.KindValidation{Type: "object", Nullable: true}, ObjectValidation: validation.ObjectValidation{Required: []string{"sharedName"}, Properties: []validation.PropertyValidation{{Name: "optionalCode"}, {Name: "sharedName"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressMiddleAllOf/allOf/1/properties/optionalCode", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressMiddleAllOf/allOf/1/properties/sharedName", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressViaMiddle/allOf/1", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"middleFlag", "sharedName"}, Properties: []validation.PropertyValidation{{Name: "middleFlag"}, {Name: "nested"}, {Name: "sharedName"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressViaMiddle/allOf/1/properties/middleFlag", KindValidation: validation.KindValidation{Type: "boolean"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedCombined", ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedOverlay", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"sameName"}, Properties: []validation.PropertyValidation{{Name: "leaf"}, {Name: "sameName"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedOverlay/properties/sameName", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"sameName", "sealed"}, Properties: []validation.PropertyValidation{{Name: "sameName"}, {Name: "sealed"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sameName", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"locked"}, Properties: []validation.PropertyValidation{{Name: "locked"}}}},
		{SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed/properties/locked", KindValidation: validation.KindValidation{Type: "boolean"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressViaMiddle/allOf/1/properties/sharedName", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"final", "nested", "nullableRequired"}, Properties: []validation.PropertyValidation{{Name: "final"}, {Name: "nested"}, {Name: "nullableRequired"}, {Name: "optionalShared"}, {Name: "sharedName"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/nullableRequired", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/optionalShared", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/sharedName", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressSecondAllOf", ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressOtherMiddle", ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressOtherMiddle/allOf/1", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"metadata", "rootFlag"}, Properties: []validation.PropertyValidation{{Name: "final"}, {Name: "metadata"}, {Name: "rootFlag"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressOtherMiddle/allOf/1/properties/metadata", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressOtherMiddle/allOf/1/properties/rootFlag", KindValidation: validation.KindValidation{Type: "boolean"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"count", "finals", "metadata", "rootFlag"}, Properties: []validation.PropertyValidation{{Name: "count"}, {Name: "finals"}, {Name: "metadata"}, {Name: "rootFlag"}, {Name: "sharedName"}}, AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/count", KindValidation: validation.KindValidation{Type: "number"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/finals", KindValidation: validation.KindValidation{Type: "array"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/metadata", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/rootFlag", KindValidation: validation.KindValidation{Type: "boolean"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/sharedName", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"count", "final", "finalCode", "finals", "metadata", "middleFlag", "nested", "nullableRequired", "rootFlag", "sharedName"}, Properties: []validation.PropertyValidation{{Name: "count"}, {Name: "final"}, {Name: "finalCode"}, {Name: "finals"}, {Name: "metadata"}, {Name: "middleFlag"}, {Name: "nested"}, {Name: "nullableRequired"}, {Name: "optionalCode"}, {Name: "optionalShared"}, {Name: "rootFlag"}, {Name: "sharedName"}}}},
		{SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/count", KindValidation: validation.KindValidation{Type: "number"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/finalCode", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/finals", KindValidation: validation.KindValidation{Type: "array"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/metadata", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/middleFlag", KindValidation: validation.KindValidation{Type: "boolean"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/nullableRequired", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/optionalCode", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/optionalShared", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/rootFlag", KindValidation: validation.KindValidation{Type: "boolean"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		{SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/sharedName", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
	}

	validations[0].AllOfValidations = append(validations[0].AllOfValidations, validations[1])
	validations[0].AllOfValidations = append(validations[0].AllOfValidations, validations[28])
	validations[0].AllOfValidations = append(validations[0].AllOfValidations, validations[39])
	validations[1].AllOfValidations = append(validations[1].AllOfValidations, validations[2])
	validations[1].AllOfValidations = append(validations[1].AllOfValidations, validations[9])
	validations[1].AllOfValidations = append(validations[1].AllOfValidations, validations[24])
	validations[2].ObjectValidation.Properties[0].Validation = validations[3]
	validations[2].ObjectValidation.Properties[1].Validation = validations[4]
	validations[2].ObjectValidation.Properties[2].Validation = validations[7]
	validations[2].ObjectValidation.Properties[3].Validation = validations[8]
	validations[4].ObjectValidation.Properties[0].Validation = validations[5]
	validations[4].ObjectValidation.Properties[1].Validation = validations[6]
	validations[9].AllOfValidations = append(validations[9].AllOfValidations, validations[10])
	validations[9].AllOfValidations = append(validations[9].AllOfValidations, validations[14])
	validations[10].AllOfValidations = append(validations[10].AllOfValidations, validations[2])
	validations[10].AllOfValidations = append(validations[10].AllOfValidations, validations[11])
	validations[11].ObjectValidation.Properties[0].Validation = validations[12]
	validations[11].ObjectValidation.Properties[1].Validation = validations[13]
	validations[14].ObjectValidation.Properties[0].Validation = validations[15]
	validations[14].ObjectValidation.Properties[1].Validation = validations[16]
	validations[14].ObjectValidation.Properties[2].Validation = validations[23]
	validations[16].AllOfValidations = append(validations[16].AllOfValidations, validations[4])
	validations[16].AllOfValidations = append(validations[16].AllOfValidations, validations[17])
	validations[16].AllOfValidations = append(validations[16].AllOfValidations, validations[19])
	validations[17].ObjectValidation.Properties[0].Validation = validations[5]
	validations[17].ObjectValidation.Properties[1].Validation = validations[18]
	validations[19].ObjectValidation.Properties[0].Validation = validations[20]
	validations[19].ObjectValidation.Properties[1].Validation = validations[21]
	validations[21].ObjectValidation.Properties[0].Validation = validations[22]
	validations[24].ObjectValidation.Properties[0].Validation = validations[2]
	validations[24].ObjectValidation.Properties[1].Validation = validations[16]
	validations[24].ObjectValidation.Properties[2].Validation = validations[25]
	validations[24].ObjectValidation.Properties[3].Validation = validations[26]
	validations[24].ObjectValidation.Properties[4].Validation = validations[27]
	validations[28].AllOfValidations = append(validations[28].AllOfValidations, validations[29])
	validations[28].AllOfValidations = append(validations[28].AllOfValidations, validations[33])
	validations[29].AllOfValidations = append(validations[29].AllOfValidations, validations[2])
	validations[29].AllOfValidations = append(validations[29].AllOfValidations, validations[30])
	validations[30].ObjectValidation.Properties[0].Validation = validations[2]
	validations[30].ObjectValidation.Properties[1].Validation = validations[31]
	validations[30].ObjectValidation.Properties[2].Validation = validations[32]
	validations[31].ObjectValidation.AdditionalPropertiesValidation = validations[5]
	validations[33].ObjectValidation.Properties[0].Validation = validations[34]
	validations[33].ObjectValidation.Properties[1].Validation = validations[35]
	validations[33].ObjectValidation.Properties[2].Validation = validations[36]
	validations[33].ObjectValidation.Properties[3].Validation = validations[37]
	validations[33].ObjectValidation.Properties[4].Validation = validations[38]
	validations[35].ArrayValidation.Items = validations[2]
	validations[36].ObjectValidation.AdditionalPropertiesValidation = validations[5]
	validations[39].ObjectValidation.Properties[0].Validation = validations[40]
	validations[39].ObjectValidation.Properties[1].Validation = validations[2]
	validations[39].ObjectValidation.Properties[2].Validation = validations[41]
	validations[39].ObjectValidation.Properties[3].Validation = validations[42]
	validations[39].ObjectValidation.Properties[4].Validation = validations[43]
	validations[39].ObjectValidation.Properties[5].Validation = validations[44]
	validations[39].ObjectValidation.Properties[6].Validation = validations[16]
	validations[39].ObjectValidation.Properties[7].Validation = validations[45]
	validations[39].ObjectValidation.Properties[8].Validation = validations[46]
	validations[39].ObjectValidation.Properties[9].Validation = validations[47]
	validations[39].ObjectValidation.Properties[10].Validation = validations[48]
	validations[39].ObjectValidation.Properties[11].Validation = validations[49]
	validations[42].ArrayValidation.Items = validations[2]
	validations[43].ObjectValidation.AdditionalPropertiesValidation = validations[5]

	return validations[0]
}()

// stringNoFormatNotNullable is a compiled request-body validation.
var stringNoFormatNotNullable = func() *validation.Validation {
	validations := []*validation.Validation{
		{SchemaPointer: "#/paths/~1string-no-format-not-nullable/post/requestBody/content/application~1json/schema", BodyRequired: true, KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
	}

	return validations[0]
}()

// stringNoFormatNullable is a compiled request-body validation.
var stringNoFormatNullable = func() *validation.Validation {
	validations := []*validation.Validation{
		{SchemaPointer: "#/paths/~1string-no-format-nullable/post/requestBody/content/application~1json/schema", BodyRequired: true, KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
	}

	return validations[0]
}()
