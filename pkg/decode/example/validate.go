// Package example contains generated request-body validations.
//
//nolint:dupl,lll // Generated validation graphs may contain repeated schemas and long pointers.
package example

import (
	"github.com/djosh34/decode_and_validate_generator/pkg/validation"
)

// validations contains every compiled request-body validation by exact operation ID.
var validations = map[string]*validation.Validation{
	"allOfObject": func() *validation.Validation {
		nodes := []*validation.Validation{
			{SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema", BodyRequired: true, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/0", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"first"}, Properties: []validation.PropertyValidation{{Name: "first"}}, AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/0/properties/first", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/1", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"second"}, Properties: []validation.PropertyValidation{{Name: "second"}}, AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/1/properties/second", KindValidation: validation.KindValidation{Type: "boolean"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/2", KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"last"}, Properties: []validation.PropertyValidation{{Name: "last"}}, AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/2/properties/last", KindValidation: validation.KindValidation{Type: "number"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		}

		nodes[0].AllOfValidations = append(nodes[0].AllOfValidations, nodes[1])
		nodes[0].AllOfValidations = append(nodes[0].AllOfValidations, nodes[3])
		nodes[0].AllOfValidations = append(nodes[0].AllOfValidations, nodes[5])
		nodes[1].ObjectValidation.Properties[0].Validation = nodes[2]
		nodes[3].ObjectValidation.Properties[0].Validation = nodes[4]
		nodes[5].ObjectValidation.Properties[0].Validation = nodes[6]

		return nodes[0]
	}(),
	"arrayNotNullable": func() *validation.Validation {
		nodes := []*validation.Validation{
			{SchemaPointer: "#/paths/~1array-not-nullable/post/requestBody/content/application~1json/schema", BodyRequired: true, KindValidation: validation.KindValidation{Type: "array"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/paths/~1array-not-nullable/post/requestBody/content/application~1json/schema/items", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		}

		nodes[0].ArrayValidation.Items = nodes[1]

		return nodes[0]
	}(),
	"arrayNullable": func() *validation.Validation {
		nodes := []*validation.Validation{
			{SchemaPointer: "#/paths/~1array-nullable/post/requestBody/content/application~1json/schema", BodyRequired: true, KindValidation: validation.KindValidation{Type: "array", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/paths/~1array-nullable/post/requestBody/content/application~1json/schema/items", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		}

		nodes[0].ArrayValidation.Items = nodes[1]

		return nodes[0]
	}(),
	"compositeObject": func() *validation.Validation {
		nodes := []*validation.Validation{
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

		nodes[0].ObjectValidation.Properties[0].Validation = nodes[1]
		nodes[0].ObjectValidation.Properties[1].Validation = nodes[3]
		nodes[0].ObjectValidation.Properties[2].Validation = nodes[5]
		nodes[0].ObjectValidation.Properties[3].Validation = nodes[7]
		nodes[0].ObjectValidation.Properties[4].Validation = nodes[9]
		nodes[0].ObjectValidation.Properties[5].Validation = nodes[10]
		nodes[0].ObjectValidation.Properties[6].Validation = nodes[11]
		nodes[0].ObjectValidation.Properties[7].Validation = nodes[12]
		nodes[0].ObjectValidation.Properties[8].Validation = nodes[13]
		nodes[0].ObjectValidation.Properties[9].Validation = nodes[15]
		nodes[0].ObjectValidation.Properties[10].Validation = nodes[18]
		nodes[0].ObjectValidation.Properties[11].Validation = nodes[20]
		nodes[0].ObjectValidation.Properties[12].Validation = nodes[21]
		nodes[1].ArrayValidation.Items = nodes[2]
		nodes[3].ArrayValidation.Items = nodes[4]
		nodes[5].ArrayValidation.Items = nodes[6]
		nodes[7].ArrayValidation.Items = nodes[8]
		nodes[13].ObjectValidation.Properties[0].Validation = nodes[14]
		nodes[15].ObjectValidation.Properties[0].Validation = nodes[16]
		nodes[15].ObjectValidation.AdditionalPropertiesValidation = nodes[17]
		nodes[18].ObjectValidation.Properties[0].Validation = nodes[19]

		return nodes[0]
	}(),
	"nullableObjectKeysAdditionalPropertiesFalse": func() *validation.Validation {
		nodes := []*validation.Validation{
			{SchemaPointer: "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema", BodyRequired: true, KindValidation: validation.KindValidation{Type: "object", Nullable: true}, ObjectValidation: validation.ObjectValidation{Required: []string{"requiredNotNullableString", "requiredNullableString"}, Properties: []validation.PropertyValidation{{Name: "optionalNotNullableString"}, {Name: "optionalNullableString"}, {Name: "requiredNotNullableString"}, {Name: "requiredNullableString"}}}},
			{SchemaPointer: "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNotNullableString", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNullableString", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNotNullableString", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNullableString", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		}

		nodes[0].ObjectValidation.Properties[0].Validation = nodes[1]
		nodes[0].ObjectValidation.Properties[1].Validation = nodes[2]
		nodes[0].ObjectValidation.Properties[2].Validation = nodes[3]
		nodes[0].ObjectValidation.Properties[3].Validation = nodes[4]

		return nodes[0]
	}(),
	"objectKeysAdditionalPropertiesFalse": func() *validation.Validation {
		nodes := []*validation.Validation{
			{SchemaPointer: "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema", BodyRequired: true, KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"requiredNotNullableString", "requiredNullableString"}, Properties: []validation.PropertyValidation{{Name: "optionalNotNullableString"}, {Name: "optionalNullableString"}, {Name: "requiredNotNullableString"}, {Name: "requiredNullableString"}}}},
			{SchemaPointer: "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNotNullableString", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNullableString", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNotNullableString", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNullableString", KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		}

		nodes[0].ObjectValidation.Properties[0].Validation = nodes[1]
		nodes[0].ObjectValidation.Properties[1].Validation = nodes[2]
		nodes[0].ObjectValidation.Properties[2].Validation = nodes[3]
		nodes[0].ObjectValidation.Properties[3].Validation = nodes[4]

		return nodes[0]
	}(),
	"optionalArrayNullable": func() *validation.Validation {
		nodes := []*validation.Validation{
			{SchemaPointer: "#/paths/~1optional-array-nullable/post/requestBody/content/application~1json/schema", KindValidation: validation.KindValidation{Type: "array", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/paths/~1optional-array-nullable/post/requestBody/content/application~1json/schema/items", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		}

		nodes[0].ArrayValidation.Items = nodes[1]

		return nodes[0]
	}(),
	"refObject": func() *validation.Validation {
		nodes := []*validation.Validation{
			{SchemaPointer: "#/components/schemas/RefObjectRequest", BodyRequired: true, KindValidation: validation.KindValidation{Type: "object"}, ObjectValidation: validation.ObjectValidation{Required: []string{"refRequiredString"}, Properties: []validation.PropertyValidation{{Name: "refOptionalBool"}, {Name: "refRequiredString"}}}},
			{SchemaPointer: "#/components/schemas/RefObjectRequest/properties/refOptionalBool", KindValidation: validation.KindValidation{Type: "boolean", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
			{SchemaPointer: "#/components/schemas/RefObjectRequest/properties/refRequiredString", KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		}

		nodes[0].ObjectValidation.Properties[0].Validation = nodes[1]
		nodes[0].ObjectValidation.Properties[1].Validation = nodes[2]

		return nodes[0]
	}(),
	"refStressObject": func() *validation.Validation {
		nodes := []*validation.Validation{
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

		nodes[0].AllOfValidations = append(nodes[0].AllOfValidations, nodes[1])
		nodes[0].AllOfValidations = append(nodes[0].AllOfValidations, nodes[28])
		nodes[0].AllOfValidations = append(nodes[0].AllOfValidations, nodes[39])
		nodes[1].AllOfValidations = append(nodes[1].AllOfValidations, nodes[2])
		nodes[1].AllOfValidations = append(nodes[1].AllOfValidations, nodes[9])
		nodes[1].AllOfValidations = append(nodes[1].AllOfValidations, nodes[24])
		nodes[2].ObjectValidation.Properties[0].Validation = nodes[3]
		nodes[2].ObjectValidation.Properties[1].Validation = nodes[4]
		nodes[2].ObjectValidation.Properties[2].Validation = nodes[7]
		nodes[2].ObjectValidation.Properties[3].Validation = nodes[8]
		nodes[4].ObjectValidation.Properties[0].Validation = nodes[5]
		nodes[4].ObjectValidation.Properties[1].Validation = nodes[6]
		nodes[9].AllOfValidations = append(nodes[9].AllOfValidations, nodes[10])
		nodes[9].AllOfValidations = append(nodes[9].AllOfValidations, nodes[14])
		nodes[10].AllOfValidations = append(nodes[10].AllOfValidations, nodes[2])
		nodes[10].AllOfValidations = append(nodes[10].AllOfValidations, nodes[11])
		nodes[11].ObjectValidation.Properties[0].Validation = nodes[12]
		nodes[11].ObjectValidation.Properties[1].Validation = nodes[13]
		nodes[14].ObjectValidation.Properties[0].Validation = nodes[15]
		nodes[14].ObjectValidation.Properties[1].Validation = nodes[16]
		nodes[14].ObjectValidation.Properties[2].Validation = nodes[23]
		nodes[16].AllOfValidations = append(nodes[16].AllOfValidations, nodes[4])
		nodes[16].AllOfValidations = append(nodes[16].AllOfValidations, nodes[17])
		nodes[16].AllOfValidations = append(nodes[16].AllOfValidations, nodes[19])
		nodes[17].ObjectValidation.Properties[0].Validation = nodes[5]
		nodes[17].ObjectValidation.Properties[1].Validation = nodes[18]
		nodes[19].ObjectValidation.Properties[0].Validation = nodes[20]
		nodes[19].ObjectValidation.Properties[1].Validation = nodes[21]
		nodes[21].ObjectValidation.Properties[0].Validation = nodes[22]
		nodes[24].ObjectValidation.Properties[0].Validation = nodes[2]
		nodes[24].ObjectValidation.Properties[1].Validation = nodes[16]
		nodes[24].ObjectValidation.Properties[2].Validation = nodes[25]
		nodes[24].ObjectValidation.Properties[3].Validation = nodes[26]
		nodes[24].ObjectValidation.Properties[4].Validation = nodes[27]
		nodes[28].AllOfValidations = append(nodes[28].AllOfValidations, nodes[29])
		nodes[28].AllOfValidations = append(nodes[28].AllOfValidations, nodes[33])
		nodes[29].AllOfValidations = append(nodes[29].AllOfValidations, nodes[2])
		nodes[29].AllOfValidations = append(nodes[29].AllOfValidations, nodes[30])
		nodes[30].ObjectValidation.Properties[0].Validation = nodes[2]
		nodes[30].ObjectValidation.Properties[1].Validation = nodes[31]
		nodes[30].ObjectValidation.Properties[2].Validation = nodes[32]
		nodes[31].ObjectValidation.AdditionalPropertiesValidation = nodes[5]
		nodes[33].ObjectValidation.Properties[0].Validation = nodes[34]
		nodes[33].ObjectValidation.Properties[1].Validation = nodes[35]
		nodes[33].ObjectValidation.Properties[2].Validation = nodes[36]
		nodes[33].ObjectValidation.Properties[3].Validation = nodes[37]
		nodes[33].ObjectValidation.Properties[4].Validation = nodes[38]
		nodes[35].ArrayValidation.Items = nodes[2]
		nodes[36].ObjectValidation.AdditionalPropertiesValidation = nodes[5]
		nodes[39].ObjectValidation.Properties[0].Validation = nodes[40]
		nodes[39].ObjectValidation.Properties[1].Validation = nodes[2]
		nodes[39].ObjectValidation.Properties[2].Validation = nodes[41]
		nodes[39].ObjectValidation.Properties[3].Validation = nodes[42]
		nodes[39].ObjectValidation.Properties[4].Validation = nodes[43]
		nodes[39].ObjectValidation.Properties[5].Validation = nodes[44]
		nodes[39].ObjectValidation.Properties[6].Validation = nodes[16]
		nodes[39].ObjectValidation.Properties[7].Validation = nodes[45]
		nodes[39].ObjectValidation.Properties[8].Validation = nodes[46]
		nodes[39].ObjectValidation.Properties[9].Validation = nodes[47]
		nodes[39].ObjectValidation.Properties[10].Validation = nodes[48]
		nodes[39].ObjectValidation.Properties[11].Validation = nodes[49]
		nodes[42].ArrayValidation.Items = nodes[2]
		nodes[43].ObjectValidation.AdditionalPropertiesValidation = nodes[5]

		return nodes[0]
	}(),
	"refStressObjectPut": func() *validation.Validation {
		nodes := []*validation.Validation{
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

		nodes[0].AllOfValidations = append(nodes[0].AllOfValidations, nodes[1])
		nodes[0].AllOfValidations = append(nodes[0].AllOfValidations, nodes[28])
		nodes[0].AllOfValidations = append(nodes[0].AllOfValidations, nodes[39])
		nodes[1].AllOfValidations = append(nodes[1].AllOfValidations, nodes[2])
		nodes[1].AllOfValidations = append(nodes[1].AllOfValidations, nodes[9])
		nodes[1].AllOfValidations = append(nodes[1].AllOfValidations, nodes[24])
		nodes[2].ObjectValidation.Properties[0].Validation = nodes[3]
		nodes[2].ObjectValidation.Properties[1].Validation = nodes[4]
		nodes[2].ObjectValidation.Properties[2].Validation = nodes[7]
		nodes[2].ObjectValidation.Properties[3].Validation = nodes[8]
		nodes[4].ObjectValidation.Properties[0].Validation = nodes[5]
		nodes[4].ObjectValidation.Properties[1].Validation = nodes[6]
		nodes[9].AllOfValidations = append(nodes[9].AllOfValidations, nodes[10])
		nodes[9].AllOfValidations = append(nodes[9].AllOfValidations, nodes[14])
		nodes[10].AllOfValidations = append(nodes[10].AllOfValidations, nodes[2])
		nodes[10].AllOfValidations = append(nodes[10].AllOfValidations, nodes[11])
		nodes[11].ObjectValidation.Properties[0].Validation = nodes[12]
		nodes[11].ObjectValidation.Properties[1].Validation = nodes[13]
		nodes[14].ObjectValidation.Properties[0].Validation = nodes[15]
		nodes[14].ObjectValidation.Properties[1].Validation = nodes[16]
		nodes[14].ObjectValidation.Properties[2].Validation = nodes[23]
		nodes[16].AllOfValidations = append(nodes[16].AllOfValidations, nodes[4])
		nodes[16].AllOfValidations = append(nodes[16].AllOfValidations, nodes[17])
		nodes[16].AllOfValidations = append(nodes[16].AllOfValidations, nodes[19])
		nodes[17].ObjectValidation.Properties[0].Validation = nodes[5]
		nodes[17].ObjectValidation.Properties[1].Validation = nodes[18]
		nodes[19].ObjectValidation.Properties[0].Validation = nodes[20]
		nodes[19].ObjectValidation.Properties[1].Validation = nodes[21]
		nodes[21].ObjectValidation.Properties[0].Validation = nodes[22]
		nodes[24].ObjectValidation.Properties[0].Validation = nodes[2]
		nodes[24].ObjectValidation.Properties[1].Validation = nodes[16]
		nodes[24].ObjectValidation.Properties[2].Validation = nodes[25]
		nodes[24].ObjectValidation.Properties[3].Validation = nodes[26]
		nodes[24].ObjectValidation.Properties[4].Validation = nodes[27]
		nodes[28].AllOfValidations = append(nodes[28].AllOfValidations, nodes[29])
		nodes[28].AllOfValidations = append(nodes[28].AllOfValidations, nodes[33])
		nodes[29].AllOfValidations = append(nodes[29].AllOfValidations, nodes[2])
		nodes[29].AllOfValidations = append(nodes[29].AllOfValidations, nodes[30])
		nodes[30].ObjectValidation.Properties[0].Validation = nodes[2]
		nodes[30].ObjectValidation.Properties[1].Validation = nodes[31]
		nodes[30].ObjectValidation.Properties[2].Validation = nodes[32]
		nodes[31].ObjectValidation.AdditionalPropertiesValidation = nodes[5]
		nodes[33].ObjectValidation.Properties[0].Validation = nodes[34]
		nodes[33].ObjectValidation.Properties[1].Validation = nodes[35]
		nodes[33].ObjectValidation.Properties[2].Validation = nodes[36]
		nodes[33].ObjectValidation.Properties[3].Validation = nodes[37]
		nodes[33].ObjectValidation.Properties[4].Validation = nodes[38]
		nodes[35].ArrayValidation.Items = nodes[2]
		nodes[36].ObjectValidation.AdditionalPropertiesValidation = nodes[5]
		nodes[39].ObjectValidation.Properties[0].Validation = nodes[40]
		nodes[39].ObjectValidation.Properties[1].Validation = nodes[2]
		nodes[39].ObjectValidation.Properties[2].Validation = nodes[41]
		nodes[39].ObjectValidation.Properties[3].Validation = nodes[42]
		nodes[39].ObjectValidation.Properties[4].Validation = nodes[43]
		nodes[39].ObjectValidation.Properties[5].Validation = nodes[44]
		nodes[39].ObjectValidation.Properties[6].Validation = nodes[16]
		nodes[39].ObjectValidation.Properties[7].Validation = nodes[45]
		nodes[39].ObjectValidation.Properties[8].Validation = nodes[46]
		nodes[39].ObjectValidation.Properties[9].Validation = nodes[47]
		nodes[39].ObjectValidation.Properties[10].Validation = nodes[48]
		nodes[39].ObjectValidation.Properties[11].Validation = nodes[49]
		nodes[42].ArrayValidation.Items = nodes[2]
		nodes[43].ObjectValidation.AdditionalPropertiesValidation = nodes[5]

		return nodes[0]
	}(),
	"stringNoFormatNotNullable": func() *validation.Validation {
		nodes := []*validation.Validation{
			{SchemaPointer: "#/paths/~1string-no-format-not-nullable/post/requestBody/content/application~1json/schema", BodyRequired: true, KindValidation: validation.KindValidation{Type: "string"}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		}

		return nodes[0]
	}(),
	"stringNoFormatNullable": func() *validation.Validation {
		nodes := []*validation.Validation{
			{SchemaPointer: "#/paths/~1string-no-format-nullable/post/requestBody/content/application~1json/schema", BodyRequired: true, KindValidation: validation.KindValidation{Type: "string", Nullable: true}, ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true}},
		}

		return nodes[0]
	}(),
}
