// Package example contains generated request validations.
//
//nolint:dupl,godoclint,lll,mnd // Generated validation trees use operation IDs and graph sizes.
package example

import (
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/validation"
)

var generatedValidations = func() []*validation.Validation {
	compiled := make([]*validation.Validation, 160)
	for index := range compiled {
		compiled[index] = new(validation.Validation)
	}

	*compiled[0] = validation.Validation{
		SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema",
		BodyRequired:  true,

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[1], compiled[3], compiled[5],
		},
	}

	*compiled[1] = validation.Validation{
		SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/0",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"first"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "first",
					Validation: compiled[2],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[2] = validation.Validation{
		SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/0/properties/first",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[3] = validation.Validation{
		SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/1",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"second"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "second",
					Validation: compiled[4],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[4] = validation.Validation{
		SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/1/properties/second",

		KindValidation: validation.KindValidation{
			Type: "boolean",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[5] = validation.Validation{
		SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/2",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"last"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "last",
					Validation: compiled[6],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[6] = validation.Validation{
		SchemaPointer: "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/2/properties/last",

		KindValidation: validation.KindValidation{
			Type: "number",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[7] = validation.Validation{
		SchemaPointer: "#/paths/~1any-of~1{id}/post/requestBody/content/application~1json/schema",
		BodyRequired:  true,

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[8],
		},

		AnyOfValidations: []*validation.Validation{
			compiled[9], compiled[10],
		},
	}

	*compiled[8] = validation.Validation{
		SchemaPointer: "#/paths/~1any-of~1{id}/post/requestBody/content/application~1json/schema/allOf/0",

		StringValidation: validation.StringValidation{
			Pattern: "^[^x]+$",

			CompiledPattern: patternvalidator.MustParse(
				"^[^x]+$",
			),
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[9] = validation.Validation{
		SchemaPointer: "#/paths/~1any-of~1{id}/post/requestBody/content/application~1json/schema/anyOf/0",

		StringValidation: validation.StringValidation{
			Pattern: "^a",

			CompiledPattern: patternvalidator.MustParse(
				"^a",
			),
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[10] = validation.Validation{
		SchemaPointer: "#/paths/~1any-of~1{id}/post/requestBody/content/application~1json/schema/anyOf/1",

		StringValidation: validation.StringValidation{
			Pattern: "z$",

			CompiledPattern: patternvalidator.MustParse(
				"z$",
			),
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[11] = validation.Validation{
		SchemaPointer: "#/paths/~1any-of~1{id}/post/parameters/1/schema",

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AnyOfValidations: []*validation.Validation{
			compiled[12], compiled[13],
		},
	}

	*compiled[12] = validation.Validation{
		SchemaPointer: "#/paths/~1any-of~1{id}/post/parameters/1/schema/anyOf/0",

		KindValidation: validation.KindValidation{
			Type: "integer",
		},

		NumberValidation: validation.NumberValidation{
			Minimum: &validation.NumberBound{
				Value: "10",

				ExactValue: jsonvalue.Number{Lexeme: "10"},
			},
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[13] = validation.Validation{
		SchemaPointer: "#/paths/~1any-of~1{id}/post/parameters/1/schema/anyOf/1",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		StringValidation: validation.StringValidation{
			Pattern: "^7$",

			CompiledPattern: patternvalidator.MustParse(
				"^7$",
			),
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[14] = validation.Validation{
		SchemaPointer: "#/paths/~1any-of~1{id}/post/parameters/0/schema",

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AnyOfValidations: []*validation.Validation{
			compiled[15], compiled[16],
		},
	}

	*compiled[15] = validation.Validation{
		SchemaPointer: "#/paths/~1any-of~1{id}/post/parameters/0/schema/anyOf/0",

		KindValidation: validation.KindValidation{
			Type: "integer",
		},

		NumberValidation: validation.NumberValidation{
			Minimum: &validation.NumberBound{
				Value: "10",

				ExactValue: jsonvalue.Number{Lexeme: "10"},
			},
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[16] = validation.Validation{
		SchemaPointer: "#/paths/~1any-of~1{id}/post/parameters/0/schema/anyOf/1",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		StringValidation: validation.StringValidation{
			Pattern: "^7$",

			CompiledPattern: patternvalidator.MustParse(
				"^7$",
			),
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[17] = validation.Validation{
		SchemaPointer: "#/paths/~1array-not-nullable/post/requestBody/content/application~1json/schema",
		BodyRequired:  true,

		KindValidation: validation.KindValidation{
			Type: "array",
		},

		ArrayValidation: validation.ArrayValidation{
			Items: compiled[18],
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[18] = validation.Validation{
		SchemaPointer: "#/paths/~1array-not-nullable/post/requestBody/content/application~1json/schema/items",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[19] = validation.Validation{
		SchemaPointer: "#/paths/~1array-nullable/post/requestBody/content/application~1json/schema",
		BodyRequired:  true,

		KindValidation: validation.KindValidation{
			Type:     "array",
			Nullable: true,
		},

		ArrayValidation: validation.ArrayValidation{
			Items: compiled[20],
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[20] = validation.Validation{
		SchemaPointer: "#/paths/~1array-nullable/post/requestBody/content/application~1json/schema/items",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[21] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema",
		BodyRequired:  true,

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"arrayNotNullableItemsNotNullable", "arrayNotNullableItemsNullable", "arrayNullableItemsNotNullable", "arrayNullableItemsNullable", "boolNotNullable", "boolNullable", "numberNotNullable", "numberNullable", "objectAdditionalPropertiesImplicit", "objectAdditionalPropertiesSchema", "objectAdditionalPropertiesTrue", "stringFormatNotNullable", "stringFormatNullable"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "arrayNotNullableItemsNotNullable",
					Validation: compiled[22],
				}, {
					Name:       "arrayNotNullableItemsNullable",
					Validation: compiled[24],
				}, {
					Name:       "arrayNullableItemsNotNullable",
					Validation: compiled[26],
				}, {
					Name:       "arrayNullableItemsNullable",
					Validation: compiled[28],
				}, {
					Name:       "boolNotNullable",
					Validation: compiled[30],
				}, {
					Name:       "boolNullable",
					Validation: compiled[31],
				}, {
					Name:       "numberNotNullable",
					Validation: compiled[32],
				}, {
					Name:       "numberNullable",
					Validation: compiled[33],
				}, {
					Name:       "objectAdditionalPropertiesImplicit",
					Validation: compiled[34],
				}, {
					Name:       "objectAdditionalPropertiesSchema",
					Validation: compiled[36],
				}, {
					Name:       "objectAdditionalPropertiesTrue",
					Validation: compiled[39],
				}, {
					Name:       "stringFormatNotNullable",
					Validation: compiled[41],
				}, {
					Name:       "stringFormatNullable",
					Validation: compiled[42],
				},
			},
		},
	}

	*compiled[22] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNotNullableItemsNotNullable",

		KindValidation: validation.KindValidation{
			Type: "array",
		},

		ArrayValidation: validation.ArrayValidation{
			Items: compiled[23],
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[23] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNotNullableItemsNotNullable/items",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[24] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNotNullableItemsNullable",

		KindValidation: validation.KindValidation{
			Type: "array",
		},

		ArrayValidation: validation.ArrayValidation{
			Items: compiled[25],
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[25] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNotNullableItemsNullable/items",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[26] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNullableItemsNotNullable",

		KindValidation: validation.KindValidation{
			Type:     "array",
			Nullable: true,
		},

		ArrayValidation: validation.ArrayValidation{
			Items: compiled[27],
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[27] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNullableItemsNotNullable/items",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[28] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNullableItemsNullable",

		KindValidation: validation.KindValidation{
			Type:     "array",
			Nullable: true,
		},

		ArrayValidation: validation.ArrayValidation{
			Items: compiled[29],
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[29] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNullableItemsNullable/items",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[30] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/boolNotNullable",

		KindValidation: validation.KindValidation{
			Type: "boolean",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[31] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/boolNullable",

		KindValidation: validation.KindValidation{
			Type:     "boolean",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[32] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/numberNotNullable",

		KindValidation: validation.KindValidation{
			Type: "number",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[33] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/numberNullable",

		KindValidation: validation.KindValidation{
			Type:     "number",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[34] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesImplicit",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Properties: []validation.PropertyValidation{
				{
					Name:       "known",
					Validation: compiled[35],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[35] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesImplicit/properties/known",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[36] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesSchema",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Properties: []validation.PropertyValidation{
				{
					Name:       "known",
					Validation: compiled[37],
				},
			},

			AdditionalPropertiesAllowed: true,

			AdditionalPropertiesValidation: compiled[38],
		},
	}

	*compiled[37] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesSchema/properties/known",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[38] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesSchema/additionalProperties",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[39] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesTrue",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Properties: []validation.PropertyValidation{
				{
					Name:       "known",
					Validation: compiled[40],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[40] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesTrue/properties/known",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[41] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/stringFormatNotNullable",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		StringValidation: validation.StringValidation{
			Format: "date-time",

			CompiledFormat: validation.MustCompileStringFormat("date-time"),
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[42] = validation.Validation{
		SchemaPointer: "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/stringFormatNullable",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		StringValidation: validation.StringValidation{
			Format: "date-time",

			CompiledFormat: validation.MustCompileStringFormat("date-time"),
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[43] = validation.Validation{
		SchemaPointer: "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema",
		BodyRequired:  true,

		KindValidation: validation.KindValidation{
			Type:     "object",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"requiredNotNullableString", "requiredNullableString"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "optionalNotNullableString",
					Validation: compiled[44],
				}, {
					Name:       "optionalNullableString",
					Validation: compiled[45],
				}, {
					Name:       "requiredNotNullableString",
					Validation: compiled[46],
				}, {
					Name:       "requiredNullableString",
					Validation: compiled[47],
				},
			},
		},
	}

	*compiled[44] = validation.Validation{
		SchemaPointer: "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNotNullableString",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[45] = validation.Validation{
		SchemaPointer: "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNullableString",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[46] = validation.Validation{
		SchemaPointer: "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNotNullableString",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[47] = validation.Validation{
		SchemaPointer: "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNullableString",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[48] = validation.Validation{
		SchemaPointer: "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema",
		BodyRequired:  true,

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"requiredNotNullableString", "requiredNullableString"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "optionalNotNullableString",
					Validation: compiled[49],
				}, {
					Name:       "optionalNullableString",
					Validation: compiled[50],
				}, {
					Name:       "requiredNotNullableString",
					Validation: compiled[51],
				}, {
					Name:       "requiredNullableString",
					Validation: compiled[52],
				},
			},
		},
	}

	*compiled[49] = validation.Validation{
		SchemaPointer: "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNotNullableString",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[50] = validation.Validation{
		SchemaPointer: "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNullableString",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[51] = validation.Validation{
		SchemaPointer: "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNotNullableString",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[52] = validation.Validation{
		SchemaPointer: "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNullableString",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[53] = validation.Validation{
		SchemaPointer: "#/paths/~1optional-array-nullable/post/requestBody/content/application~1json/schema",

		KindValidation: validation.KindValidation{
			Type:     "array",
			Nullable: true,
		},

		ArrayValidation: validation.ArrayValidation{
			Items: compiled[54],
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[54] = validation.Validation{
		SchemaPointer: "#/paths/~1optional-array-nullable/post/requestBody/content/application~1json/schema/items",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[55] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefObjectRequest",
		BodyRequired:  true,

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"refRequiredString"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "refOptionalBool",
					Validation: compiled[56],
				}, {
					Name:       "refRequiredString",
					Validation: compiled[57],
				},
			},
		},
	}

	*compiled[56] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefObjectRequest/properties/refOptionalBool",

		KindValidation: validation.KindValidation{
			Type:     "boolean",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[57] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefObjectRequest/properties/refRequiredString",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[58] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema",
		BodyRequired:  true,

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[59], compiled[86], compiled[97],
		},
	}

	*compiled[59] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFirstAllOf",

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[60], compiled[67], compiled[82],
		},
	}

	*compiled[60] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFinal",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"finalCode", "sharedName"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "finalCode",
					Validation: compiled[61],
				}, {
					Name:       "nested",
					Validation: compiled[62],
				}, {
					Name:       "optionalShared",
					Validation: compiled[65],
				}, {
					Name:       "sharedName",
					Validation: compiled[66],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[61] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFinal/properties/finalCode",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[62] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedBase",

		KindValidation: validation.KindValidation{
			Type:     "object",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"sameName"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "leaf",
					Validation: compiled[63],
				}, {
					Name:       "sameName",
					Validation: compiled[64],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[63] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressMetadataValue",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[64] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedBase/properties/sameName",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[65] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFinal/properties/optionalShared",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[66] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFinal/properties/sharedName",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[67] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressViaMiddle",

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[68], compiled[72],
		},
	}

	*compiled[68] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressMiddleAllOf",

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[60], compiled[69],
		},
	}

	*compiled[69] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressMiddleAllOf/allOf/1",

		KindValidation: validation.KindValidation{
			Type:     "object",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"sharedName"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "optionalCode",
					Validation: compiled[70],
				}, {
					Name:       "sharedName",
					Validation: compiled[71],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[70] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressMiddleAllOf/allOf/1/properties/optionalCode",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[71] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressMiddleAllOf/allOf/1/properties/sharedName",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[72] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressViaMiddle/allOf/1",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"middleFlag", "sharedName"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "middleFlag",
					Validation: compiled[73],
				}, {
					Name:       "nested",
					Validation: compiled[74],
				}, {
					Name:       "sharedName",
					Validation: compiled[81],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[73] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressViaMiddle/allOf/1/properties/middleFlag",

		KindValidation: validation.KindValidation{
			Type: "boolean",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[74] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedCombined",

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[62], compiled[75], compiled[77],
		},
	}

	*compiled[75] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedOverlay",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"sameName"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "leaf",
					Validation: compiled[63],
				}, {
					Name:       "sameName",
					Validation: compiled[76],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[76] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedOverlay/properties/sameName",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[77] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"sameName", "sealed"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "sameName",
					Validation: compiled[78],
				}, {
					Name:       "sealed",
					Validation: compiled[79],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[78] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sameName",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[79] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"locked"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "locked",
					Validation: compiled[80],
				},
			},
		},
	}

	*compiled[80] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed/properties/locked",

		KindValidation: validation.KindValidation{
			Type: "boolean",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[81] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressViaMiddle/allOf/1/properties/sharedName",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[82] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"final", "nested", "nullableRequired"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "final",
					Validation: compiled[60],
				}, {
					Name:       "nested",
					Validation: compiled[74],
				}, {
					Name:       "nullableRequired",
					Validation: compiled[83],
				}, {
					Name:       "optionalShared",
					Validation: compiled[84],
				}, {
					Name:       "sharedName",
					Validation: compiled[85],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[83] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/nullableRequired",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[84] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/optionalShared",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[85] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/sharedName",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[86] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressSecondAllOf",

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[87], compiled[91],
		},
	}

	*compiled[87] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressOtherMiddle",

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[60], compiled[88],
		},
	}

	*compiled[88] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressOtherMiddle/allOf/1",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"metadata", "rootFlag"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "final",
					Validation: compiled[60],
				}, {
					Name:       "metadata",
					Validation: compiled[89],
				}, {
					Name:       "rootFlag",
					Validation: compiled[90],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[89] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressOtherMiddle/allOf/1/properties/metadata",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,

			AdditionalPropertiesValidation: compiled[63],
		},
	}

	*compiled[90] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressOtherMiddle/allOf/1/properties/rootFlag",

		KindValidation: validation.KindValidation{
			Type: "boolean",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[91] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"count", "finals", "metadata", "rootFlag"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "count",
					Validation: compiled[92],
				}, {
					Name:       "finals",
					Validation: compiled[93],
				}, {
					Name:       "metadata",
					Validation: compiled[94],
				}, {
					Name:       "rootFlag",
					Validation: compiled[95],
				}, {
					Name:       "sharedName",
					Validation: compiled[96],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[92] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/count",

		KindValidation: validation.KindValidation{
			Type: "number",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[93] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/finals",

		KindValidation: validation.KindValidation{
			Type: "array",
		},

		ArrayValidation: validation.ArrayValidation{
			Items: compiled[60],
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[94] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/metadata",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,

			AdditionalPropertiesValidation: compiled[63],
		},
	}

	*compiled[95] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/rootFlag",

		KindValidation: validation.KindValidation{
			Type: "boolean",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[96] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/sharedName",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[97] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"count", "final", "finalCode", "finals", "metadata", "middleFlag", "nested", "nullableRequired", "rootFlag", "sharedName"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "count",
					Validation: compiled[98],
				}, {
					Name:       "final",
					Validation: compiled[60],
				}, {
					Name:       "finalCode",
					Validation: compiled[99],
				}, {
					Name:       "finals",
					Validation: compiled[100],
				}, {
					Name:       "metadata",
					Validation: compiled[101],
				}, {
					Name:       "middleFlag",
					Validation: compiled[102],
				}, {
					Name:       "nested",
					Validation: compiled[74],
				}, {
					Name:       "nullableRequired",
					Validation: compiled[103],
				}, {
					Name:       "optionalCode",
					Validation: compiled[104],
				}, {
					Name:       "optionalShared",
					Validation: compiled[105],
				}, {
					Name:       "rootFlag",
					Validation: compiled[106],
				}, {
					Name:       "sharedName",
					Validation: compiled[107],
				},
			},
		},
	}

	*compiled[98] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/count",

		KindValidation: validation.KindValidation{
			Type: "number",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[99] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/finalCode",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[100] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/finals",

		KindValidation: validation.KindValidation{
			Type: "array",
		},

		ArrayValidation: validation.ArrayValidation{
			Items: compiled[60],
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[101] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/metadata",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,

			AdditionalPropertiesValidation: compiled[63],
		},
	}

	*compiled[102] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/middleFlag",

		KindValidation: validation.KindValidation{
			Type: "boolean",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[103] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/nullableRequired",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[104] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/optionalCode",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[105] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/optionalShared",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[106] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/rootFlag",

		KindValidation: validation.KindValidation{
			Type: "boolean",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[107] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/sharedName",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[108] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema",
		BodyRequired:  true,

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[109], compiled[136], compiled[147],
		},
	}

	*compiled[109] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFirstAllOf",

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[110], compiled[117], compiled[132],
		},
	}

	*compiled[110] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFinal",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"finalCode", "sharedName"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "finalCode",
					Validation: compiled[111],
				}, {
					Name:       "nested",
					Validation: compiled[112],
				}, {
					Name:       "optionalShared",
					Validation: compiled[115],
				}, {
					Name:       "sharedName",
					Validation: compiled[116],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[111] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFinal/properties/finalCode",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[112] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedBase",

		KindValidation: validation.KindValidation{
			Type:     "object",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"sameName"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "leaf",
					Validation: compiled[113],
				}, {
					Name:       "sameName",
					Validation: compiled[114],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[113] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressMetadataValue",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[114] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedBase/properties/sameName",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[115] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFinal/properties/optionalShared",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[116] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFinal/properties/sharedName",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[117] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressViaMiddle",

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[118], compiled[122],
		},
	}

	*compiled[118] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressMiddleAllOf",

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[110], compiled[119],
		},
	}

	*compiled[119] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressMiddleAllOf/allOf/1",

		KindValidation: validation.KindValidation{
			Type:     "object",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"sharedName"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "optionalCode",
					Validation: compiled[120],
				}, {
					Name:       "sharedName",
					Validation: compiled[121],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[120] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressMiddleAllOf/allOf/1/properties/optionalCode",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[121] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressMiddleAllOf/allOf/1/properties/sharedName",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[122] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressViaMiddle/allOf/1",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"middleFlag", "sharedName"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "middleFlag",
					Validation: compiled[123],
				}, {
					Name:       "nested",
					Validation: compiled[124],
				}, {
					Name:       "sharedName",
					Validation: compiled[131],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[123] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressViaMiddle/allOf/1/properties/middleFlag",

		KindValidation: validation.KindValidation{
			Type: "boolean",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[124] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedCombined",

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[112], compiled[125], compiled[127],
		},
	}

	*compiled[125] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedOverlay",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"sameName"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "leaf",
					Validation: compiled[113],
				}, {
					Name:       "sameName",
					Validation: compiled[126],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[126] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedOverlay/properties/sameName",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[127] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"sameName", "sealed"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "sameName",
					Validation: compiled[128],
				}, {
					Name:       "sealed",
					Validation: compiled[129],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[128] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sameName",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[129] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"locked"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "locked",
					Validation: compiled[130],
				},
			},
		},
	}

	*compiled[130] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed/properties/locked",

		KindValidation: validation.KindValidation{
			Type: "boolean",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[131] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressViaMiddle/allOf/1/properties/sharedName",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[132] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"final", "nested", "nullableRequired"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "final",
					Validation: compiled[110],
				}, {
					Name:       "nested",
					Validation: compiled[124],
				}, {
					Name:       "nullableRequired",
					Validation: compiled[133],
				}, {
					Name:       "optionalShared",
					Validation: compiled[134],
				}, {
					Name:       "sharedName",
					Validation: compiled[135],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[133] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/nullableRequired",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[134] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/optionalShared",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[135] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/sharedName",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[136] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressSecondAllOf",

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[137], compiled[141],
		},
	}

	*compiled[137] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressOtherMiddle",

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},

		AllOfValidations: []*validation.Validation{
			compiled[110], compiled[138],
		},
	}

	*compiled[138] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressOtherMiddle/allOf/1",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"metadata", "rootFlag"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "final",
					Validation: compiled[110],
				}, {
					Name:       "metadata",
					Validation: compiled[139],
				}, {
					Name:       "rootFlag",
					Validation: compiled[140],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[139] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressOtherMiddle/allOf/1/properties/metadata",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,

			AdditionalPropertiesValidation: compiled[113],
		},
	}

	*compiled[140] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressOtherMiddle/allOf/1/properties/rootFlag",

		KindValidation: validation.KindValidation{
			Type: "boolean",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[141] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"count", "finals", "metadata", "rootFlag"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "count",
					Validation: compiled[142],
				}, {
					Name:       "finals",
					Validation: compiled[143],
				}, {
					Name:       "metadata",
					Validation: compiled[144],
				}, {
					Name:       "rootFlag",
					Validation: compiled[145],
				}, {
					Name:       "sharedName",
					Validation: compiled[146],
				},
			},

			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[142] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/count",

		KindValidation: validation.KindValidation{
			Type: "number",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[143] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/finals",

		KindValidation: validation.KindValidation{
			Type: "array",
		},

		ArrayValidation: validation.ArrayValidation{
			Items: compiled[110],
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[144] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/metadata",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,

			AdditionalPropertiesValidation: compiled[113],
		},
	}

	*compiled[145] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/rootFlag",

		KindValidation: validation.KindValidation{
			Type: "boolean",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[146] = validation.Validation{
		SchemaPointer: "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/sharedName",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[147] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			Required: []string{"count", "final", "finalCode", "finals", "metadata", "middleFlag", "nested", "nullableRequired", "rootFlag", "sharedName"},

			Properties: []validation.PropertyValidation{
				{
					Name:       "count",
					Validation: compiled[148],
				}, {
					Name:       "final",
					Validation: compiled[110],
				}, {
					Name:       "finalCode",
					Validation: compiled[149],
				}, {
					Name:       "finals",
					Validation: compiled[150],
				}, {
					Name:       "metadata",
					Validation: compiled[151],
				}, {
					Name:       "middleFlag",
					Validation: compiled[152],
				}, {
					Name:       "nested",
					Validation: compiled[124],
				}, {
					Name:       "nullableRequired",
					Validation: compiled[153],
				}, {
					Name:       "optionalCode",
					Validation: compiled[154],
				}, {
					Name:       "optionalShared",
					Validation: compiled[155],
				}, {
					Name:       "rootFlag",
					Validation: compiled[156],
				}, {
					Name:       "sharedName",
					Validation: compiled[157],
				},
			},
		},
	}

	*compiled[148] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/count",

		KindValidation: validation.KindValidation{
			Type: "number",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[149] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/finalCode",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[150] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/finals",

		KindValidation: validation.KindValidation{
			Type: "array",
		},

		ArrayValidation: validation.ArrayValidation{
			Items: compiled[110],
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[151] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/metadata",

		KindValidation: validation.KindValidation{
			Type: "object",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,

			AdditionalPropertiesValidation: compiled[113],
		},
	}

	*compiled[152] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/middleFlag",

		KindValidation: validation.KindValidation{
			Type: "boolean",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[153] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/nullableRequired",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[154] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/optionalCode",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[155] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/optionalShared",

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[156] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/rootFlag",

		KindValidation: validation.KindValidation{
			Type: "boolean",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[157] = validation.Validation{
		SchemaPointer: "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/sharedName",

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[158] = validation.Validation{
		SchemaPointer: "#/paths/~1string-no-format-not-nullable/post/requestBody/content/application~1json/schema",
		BodyRequired:  true,

		KindValidation: validation.KindValidation{
			Type: "string",
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	*compiled[159] = validation.Validation{
		SchemaPointer: "#/paths/~1string-no-format-nullable/post/requestBody/content/application~1json/schema",
		BodyRequired:  true,

		KindValidation: validation.KindValidation{
			Type:     "string",
			Nullable: true,
		},

		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}

	return compiled
}()

var allOfObject = validation.RequestValidation{
	Body: generatedValidations[0],
}

var anyOfBodyAndParameters = validation.RequestValidation{
	Body: generatedValidations[7],
	Query: mustQueryDecoder(validation.QueryDecoderDefinition{
		OperationID: "anyOfBodyAndParameters",
		Parameters: []validation.QueryParameterDefinition{
			{
				Name: "q",
				Wire: 0,

				Validation: generatedValidations[11],

				ScalarType: "integer",
			},
		},
	}),
	Path: mustPathDecoder(validation.PathDecoderDefinition{
		OperationID:  "anyOfBodyAndParameters",
		PathTemplate: "/any-of/{id}",
		Parameters: []validation.PathParameterDefinition{
			{
				Name: "id",
				Wire: 0,

				Validation: generatedValidations[14],
				ScalarType: "integer",
			},
		},
	}),
}

var arrayNotNullable = validation.RequestValidation{
	Body: generatedValidations[17],
}

var arrayNullable = validation.RequestValidation{
	Body: generatedValidations[19],
}

var compositeObject = validation.RequestValidation{
	Body: generatedValidations[21],
}

var nullableObjectKeysAdditionalPropertiesFalse = validation.RequestValidation{
	Body: generatedValidations[43],
}

var objectKeysAdditionalPropertiesFalse = validation.RequestValidation{
	Body: generatedValidations[48],
}

var optionalArrayNullable = validation.RequestValidation{
	Body: generatedValidations[53],
}

var refObject = validation.RequestValidation{
	Body: generatedValidations[55],
}

var refStressObject = validation.RequestValidation{
	Body: generatedValidations[58],
}

var refStressObjectPut = validation.RequestValidation{
	Body: generatedValidations[108],
}

var stringNoFormatNotNullable = validation.RequestValidation{
	Body: generatedValidations[158],
}

var stringNoFormatNullable = validation.RequestValidation{
	Body: generatedValidations[159],
}

// RequestValidations contains every compiled request validation by exact operation ID.
var RequestValidations = map[string]validation.RequestValidation{
	"allOfObject": allOfObject,

	"anyOfBodyAndParameters": anyOfBodyAndParameters,

	"arrayNotNullable": arrayNotNullable,

	"arrayNullable": arrayNullable,

	"compositeObject": compositeObject,

	"nullableObjectKeysAdditionalPropertiesFalse": nullableObjectKeysAdditionalPropertiesFalse,

	"objectKeysAdditionalPropertiesFalse": objectKeysAdditionalPropertiesFalse,

	"optionalArrayNullable": optionalArrayNullable,

	"refObject": refObject,

	"refStressObject": refStressObject,

	"refStressObjectPut": refStressObjectPut,

	"stringNoFormatNotNullable": stringNoFormatNotNullable,

	"stringNoFormatNullable": stringNoFormatNullable,
}

func mustQueryDecoder(definition validation.QueryDecoderDefinition) *validation.QueryDecoder {
	decoder, err := validation.NewQueryDecoderFromGenerated(definition)
	if err != nil {
		panic(err)
	}

	return decoder
}

func mustPathDecoder(definition validation.PathDecoderDefinition) *validation.PathDecoder {
	decoder, err := validation.NewPathDecoderFromGenerated(definition)
	if err != nil {
		panic(err)
	}

	return decoder
}
