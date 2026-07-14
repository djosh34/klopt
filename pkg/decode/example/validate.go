package example

import "github.com/djosh34/decode_and_validate_generator/pkg/validation"

var allOfObject = &validation.Validation{
	SchemaPointer:   "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema",
	BodyRequired:    true,
	ArrayValidation: validation.ArrayValidation{},
	ObjectValidation: validation.ObjectValidation{
		AdditionalPropertiesAllowed: true,
	},
	AllOfValidations: []*validation.Validation{
		{
			SchemaPointer:  "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/0",
			KindValidation: validation.KindValidation{Type: "object"},
			ObjectValidation: validation.ObjectValidation{
				Required: []string{"first"},
				Properties: []validation.PropertyValidation{
					{Name: "first", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/0/properties/first",
						KindValidation: validation.KindValidation{Type: "string"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
				},
				AdditionalPropertiesAllowed: true,
			},
		},
		{
			SchemaPointer:  "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/1",
			KindValidation: validation.KindValidation{Type: "object"},
			ObjectValidation: validation.ObjectValidation{
				Required: []string{"second"},
				Properties: []validation.PropertyValidation{
					{Name: "second", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/1/properties/second",
						KindValidation: validation.KindValidation{Type: "boolean"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
				},
				AdditionalPropertiesAllowed: true,
			},
		},
		{
			SchemaPointer:  "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/2",
			KindValidation: validation.KindValidation{Type: "object"},
			ObjectValidation: validation.ObjectValidation{
				Required: []string{"last"},
				Properties: []validation.PropertyValidation{
					{Name: "last", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1all-of-object/post/requestBody/content/application~1json/schema/allOf/2/properties/last",
						KindValidation: validation.KindValidation{Type: "number"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
				},
				AdditionalPropertiesAllowed: true,
			},
		},
	},
}

var arrayNotNullable = &validation.Validation{
	SchemaPointer:  "#/paths/~1array-not-nullable/post/requestBody/content/application~1json/schema",
	BodyRequired:   true,
	KindValidation: validation.KindValidation{Type: "array"},
	ArrayValidation: validation.ArrayValidation{
		Items: &validation.Validation{
			SchemaPointer:  "#/paths/~1array-not-nullable/post/requestBody/content/application~1json/schema/items",
			KindValidation: validation.KindValidation{Type: "string"},
			ObjectValidation: validation.ObjectValidation{
				AdditionalPropertiesAllowed: true,
			},
		},
	},
	ObjectValidation: validation.ObjectValidation{
		AdditionalPropertiesAllowed: true,
	},
}

var arrayNullable = &validation.Validation{
	SchemaPointer:  "#/paths/~1array-nullable/post/requestBody/content/application~1json/schema",
	BodyRequired:   true,
	KindValidation: validation.KindValidation{Type: "array", Nullable: true},
	ArrayValidation: validation.ArrayValidation{
		Items: &validation.Validation{
			SchemaPointer:  "#/paths/~1array-nullable/post/requestBody/content/application~1json/schema/items",
			KindValidation: validation.KindValidation{Type: "string"},
			ObjectValidation: validation.ObjectValidation{
				AdditionalPropertiesAllowed: true,
			},
		},
	},
	ObjectValidation: validation.ObjectValidation{
		AdditionalPropertiesAllowed: true,
	},
}

var compositeObject = &validation.Validation{
	SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema",
	BodyRequired:   true,
	KindValidation: validation.KindValidation{Type: "object"},
	ObjectValidation: validation.ObjectValidation{
		Required: []string{"arrayNotNullableItemsNotNullable", "arrayNotNullableItemsNullable", "arrayNullableItemsNotNullable", "arrayNullableItemsNullable", "boolNotNullable", "boolNullable", "numberNotNullable", "numberNullable", "objectAdditionalPropertiesImplicit", "objectAdditionalPropertiesSchema", "objectAdditionalPropertiesTrue", "stringFormatNotNullable", "stringFormatNullable"},
		Properties: []validation.PropertyValidation{
			{Name: "arrayNotNullableItemsNotNullable", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNotNullableItemsNotNullable",
				KindValidation: validation.KindValidation{Type: "array"},
				ArrayValidation: validation.ArrayValidation{
					Items: &validation.Validation{
						SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNotNullableItemsNotNullable/items",
						KindValidation: validation.KindValidation{Type: "string"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					},
				},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "arrayNotNullableItemsNullable", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNotNullableItemsNullable",
				KindValidation: validation.KindValidation{Type: "array"},
				ArrayValidation: validation.ArrayValidation{
					Items: &validation.Validation{
						SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNotNullableItemsNullable/items",
						KindValidation: validation.KindValidation{Type: "string", Nullable: true},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					},
				},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "arrayNullableItemsNotNullable", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNullableItemsNotNullable",
				KindValidation: validation.KindValidation{Type: "array", Nullable: true},
				ArrayValidation: validation.ArrayValidation{
					Items: &validation.Validation{
						SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNullableItemsNotNullable/items",
						KindValidation: validation.KindValidation{Type: "string"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					},
				},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "arrayNullableItemsNullable", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNullableItemsNullable",
				KindValidation: validation.KindValidation{Type: "array", Nullable: true},
				ArrayValidation: validation.ArrayValidation{
					Items: &validation.Validation{
						SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/arrayNullableItemsNullable/items",
						KindValidation: validation.KindValidation{Type: "string", Nullable: true},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					},
				},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "boolNotNullable", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/boolNotNullable",
				KindValidation: validation.KindValidation{Type: "boolean"},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "boolNullable", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/boolNullable",
				KindValidation: validation.KindValidation{Type: "boolean", Nullable: true},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "numberNotNullable", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/numberNotNullable",
				KindValidation: validation.KindValidation{Type: "number"},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "numberNullable", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/numberNullable",
				KindValidation: validation.KindValidation{Type: "number", Nullable: true},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "objectAdditionalPropertiesImplicit", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesImplicit",
				KindValidation: validation.KindValidation{Type: "object"},
				ObjectValidation: validation.ObjectValidation{
					Properties: []validation.PropertyValidation{
						{Name: "known", Validation: &validation.Validation{
							SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesImplicit/properties/known",
							KindValidation: validation.KindValidation{Type: "string"},
							ObjectValidation: validation.ObjectValidation{
								AdditionalPropertiesAllowed: true,
							},
						}},
					},
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "objectAdditionalPropertiesSchema", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesSchema",
				KindValidation: validation.KindValidation{Type: "object"},
				ObjectValidation: validation.ObjectValidation{
					Properties: []validation.PropertyValidation{
						{Name: "known", Validation: &validation.Validation{
							SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesSchema/properties/known",
							KindValidation: validation.KindValidation{Type: "string"},
							ObjectValidation: validation.ObjectValidation{
								AdditionalPropertiesAllowed: true,
							},
						}},
					},
					AdditionalPropertiesAllowed: true,
					AdditionalPropertiesValidation: &validation.Validation{
						SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesSchema/additionalProperties",
						KindValidation: validation.KindValidation{Type: "string"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					},
				},
			}},
			{Name: "objectAdditionalPropertiesTrue", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesTrue",
				KindValidation: validation.KindValidation{Type: "object"},
				ObjectValidation: validation.ObjectValidation{
					Properties: []validation.PropertyValidation{
						{Name: "known", Validation: &validation.Validation{
							SchemaPointer:  "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/objectAdditionalPropertiesTrue/properties/known",
							KindValidation: validation.KindValidation{Type: "string"},
							ObjectValidation: validation.ObjectValidation{
								AdditionalPropertiesAllowed: true,
							},
						}},
					},
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "stringFormatNotNullable", Validation: &validation.Validation{
				SchemaPointer:    "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/stringFormatNotNullable",
				KindValidation:   validation.KindValidation{Type: "string"},
				StringValidation: validation.StringValidation{Format: "date-time"},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "stringFormatNullable", Validation: &validation.Validation{
				SchemaPointer:    "#/paths/~1composite-object/post/requestBody/content/application~1json/schema/properties/stringFormatNullable",
				KindValidation:   validation.KindValidation{Type: "string", Nullable: true},
				StringValidation: validation.StringValidation{Format: "date-time"},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
		},
	},
}

var nullableObjectKeysAdditionalPropertiesFalse = &validation.Validation{
	SchemaPointer:  "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema",
	BodyRequired:   true,
	KindValidation: validation.KindValidation{Type: "object", Nullable: true},
	ObjectValidation: validation.ObjectValidation{
		Required: []string{"requiredNotNullableString", "requiredNullableString"},
		Properties: []validation.PropertyValidation{
			{Name: "optionalNotNullableString", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNotNullableString",
				KindValidation: validation.KindValidation{Type: "string"},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "optionalNullableString", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNullableString",
				KindValidation: validation.KindValidation{Type: "string", Nullable: true},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "requiredNotNullableString", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNotNullableString",
				KindValidation: validation.KindValidation{Type: "string"},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "requiredNullableString", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1nullable-object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNullableString",
				KindValidation: validation.KindValidation{Type: "string", Nullable: true},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
		},
	},
}

var objectKeysAdditionalPropertiesFalse = &validation.Validation{
	SchemaPointer:  "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema",
	BodyRequired:   true,
	KindValidation: validation.KindValidation{Type: "object"},
	ObjectValidation: validation.ObjectValidation{
		Required: []string{"requiredNotNullableString", "requiredNullableString"},
		Properties: []validation.PropertyValidation{
			{Name: "optionalNotNullableString", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNotNullableString",
				KindValidation: validation.KindValidation{Type: "string"},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "optionalNullableString", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/optionalNullableString",
				KindValidation: validation.KindValidation{Type: "string", Nullable: true},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "requiredNotNullableString", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNotNullableString",
				KindValidation: validation.KindValidation{Type: "string"},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "requiredNullableString", Validation: &validation.Validation{
				SchemaPointer:  "#/paths/~1object-keys-additional-properties-false/post/requestBody/content/application~1json/schema/properties/requiredNullableString",
				KindValidation: validation.KindValidation{Type: "string", Nullable: true},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
		},
	},
}

var optionalArrayNullable = &validation.Validation{
	SchemaPointer:  "#/paths/~1optional-array-nullable/post/requestBody/content/application~1json/schema",
	KindValidation: validation.KindValidation{Type: "array", Nullable: true},
	ArrayValidation: validation.ArrayValidation{
		Items: &validation.Validation{
			SchemaPointer:  "#/paths/~1optional-array-nullable/post/requestBody/content/application~1json/schema/items",
			KindValidation: validation.KindValidation{Type: "string"},
			ObjectValidation: validation.ObjectValidation{
				AdditionalPropertiesAllowed: true,
			},
		},
	},
	ObjectValidation: validation.ObjectValidation{
		AdditionalPropertiesAllowed: true,
	},
}

var refObject = &validation.Validation{
	SchemaPointer:  "#/components/schemas/RefObjectRequest",
	BodyRequired:   true,
	KindValidation: validation.KindValidation{Type: "object"},
	ObjectValidation: validation.ObjectValidation{
		Required: []string{"refRequiredString"},
		Properties: []validation.PropertyValidation{
			{Name: "refOptionalBool", Validation: &validation.Validation{
				SchemaPointer:  "#/components/schemas/RefObjectRequest/properties/refOptionalBool",
				KindValidation: validation.KindValidation{Type: "boolean", Nullable: true},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
			{Name: "refRequiredString", Validation: &validation.Validation{
				SchemaPointer:  "#/components/schemas/RefObjectRequest/properties/refRequiredString",
				KindValidation: validation.KindValidation{Type: "string"},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			}},
		},
	},
}

var refStressObject = &validation.Validation{
	SchemaPointer:   "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema",
	BodyRequired:    true,
	ArrayValidation: validation.ArrayValidation{},
	ObjectValidation: validation.ObjectValidation{
		AdditionalPropertiesAllowed: true,
	},
	AllOfValidations: []*validation.Validation{
		{
			SchemaPointer:   "#/components/schemas/RefStressFirstAllOf",
			ArrayValidation: validation.ArrayValidation{},
			ObjectValidation: validation.ObjectValidation{
				AdditionalPropertiesAllowed: true,
			},
			AllOfValidations: []*validation.Validation{
				{
					SchemaPointer:  "#/components/schemas/RefStressFinal",
					KindValidation: validation.KindValidation{Type: "object"},
					ObjectValidation: validation.ObjectValidation{
						Required: []string{"finalCode", "sharedName"},
						Properties: []validation.PropertyValidation{
							{Name: "finalCode", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
								KindValidation: validation.KindValidation{Type: "string"},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "nested", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressNestedBase",
								KindValidation: validation.KindValidation{Type: "object", Nullable: true},
								ObjectValidation: validation.ObjectValidation{
									Required: []string{"sameName"},
									Properties: []validation.PropertyValidation{
										{Name: "leaf", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "sameName", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
											KindValidation: validation.KindValidation{Type: "string", Nullable: true},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
									},
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "optionalShared", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
								KindValidation: validation.KindValidation{Type: "string", Nullable: true},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "sharedName", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
								KindValidation: validation.KindValidation{Type: "string"},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
						},
						AdditionalPropertiesAllowed: true,
					},
				},
				{
					SchemaPointer:   "#/components/schemas/RefStressViaMiddle",
					ArrayValidation: validation.ArrayValidation{},
					ObjectValidation: validation.ObjectValidation{
						AdditionalPropertiesAllowed: true,
					},
					AllOfValidations: []*validation.Validation{
						{
							SchemaPointer:   "#/components/schemas/RefStressMiddleAllOf",
							ArrayValidation: validation.ArrayValidation{},
							ObjectValidation: validation.ObjectValidation{
								AdditionalPropertiesAllowed: true,
							},
							AllOfValidations: []*validation.Validation{
								{
									SchemaPointer:  "#/components/schemas/RefStressFinal",
									KindValidation: validation.KindValidation{Type: "object"},
									ObjectValidation: validation.ObjectValidation{
										Required: []string{"finalCode", "sharedName"},
										Properties: []validation.PropertyValidation{
											{Name: "finalCode", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
												KindValidation: validation.KindValidation{Type: "string"},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											}},
											{Name: "nested", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressNestedBase",
												KindValidation: validation.KindValidation{Type: "object", Nullable: true},
												ObjectValidation: validation.ObjectValidation{
													Required: []string{"sameName"},
													Properties: []validation.PropertyValidation{
														{Name: "leaf", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
															KindValidation: validation.KindValidation{Type: "string"},
															ObjectValidation: validation.ObjectValidation{
																AdditionalPropertiesAllowed: true,
															},
														}},
														{Name: "sameName", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
															KindValidation: validation.KindValidation{Type: "string", Nullable: true},
															ObjectValidation: validation.ObjectValidation{
																AdditionalPropertiesAllowed: true,
															},
														}},
													},
													AdditionalPropertiesAllowed: true,
												},
											}},
											{Name: "optionalShared", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
												KindValidation: validation.KindValidation{Type: "string", Nullable: true},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											}},
											{Name: "sharedName", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
												KindValidation: validation.KindValidation{Type: "string"},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											}},
										},
										AdditionalPropertiesAllowed: true,
									},
								},
								{
									SchemaPointer:  "#/components/schemas/RefStressMiddleAllOf/allOf/1",
									KindValidation: validation.KindValidation{Type: "object", Nullable: true},
									ObjectValidation: validation.ObjectValidation{
										Required: []string{"sharedName"},
										Properties: []validation.PropertyValidation{
											{Name: "optionalCode", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressMiddleAllOf/allOf/1/properties/optionalCode",
												KindValidation: validation.KindValidation{Type: "string"},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											}},
											{Name: "sharedName", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressMiddleAllOf/allOf/1/properties/sharedName",
												KindValidation: validation.KindValidation{Type: "string"},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											}},
										},
										AdditionalPropertiesAllowed: true,
									},
								},
							},
						},
						{
							SchemaPointer:  "#/components/schemas/RefStressViaMiddle/allOf/1",
							KindValidation: validation.KindValidation{Type: "object"},
							ObjectValidation: validation.ObjectValidation{
								Required: []string{"middleFlag", "sharedName"},
								Properties: []validation.PropertyValidation{
									{Name: "middleFlag", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressViaMiddle/allOf/1/properties/middleFlag",
										KindValidation: validation.KindValidation{Type: "boolean"},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
									}},
									{Name: "nested", Validation: &validation.Validation{
										SchemaPointer:   "#/components/schemas/RefStressNestedCombined",
										ArrayValidation: validation.ArrayValidation{},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
										AllOfValidations: []*validation.Validation{
											{
												SchemaPointer:  "#/components/schemas/RefStressNestedBase",
												KindValidation: validation.KindValidation{Type: "object", Nullable: true},
												ObjectValidation: validation.ObjectValidation{
													Required: []string{"sameName"},
													Properties: []validation.PropertyValidation{
														{Name: "leaf", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
															KindValidation: validation.KindValidation{Type: "string"},
															ObjectValidation: validation.ObjectValidation{
																AdditionalPropertiesAllowed: true,
															},
														}},
														{Name: "sameName", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
															KindValidation: validation.KindValidation{Type: "string", Nullable: true},
															ObjectValidation: validation.ObjectValidation{
																AdditionalPropertiesAllowed: true,
															},
														}},
													},
													AdditionalPropertiesAllowed: true,
												},
											},
											{
												SchemaPointer:  "#/components/schemas/RefStressNestedOverlay",
												KindValidation: validation.KindValidation{Type: "object"},
												ObjectValidation: validation.ObjectValidation{
													Required: []string{"sameName"},
													Properties: []validation.PropertyValidation{
														{Name: "leaf", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
															KindValidation: validation.KindValidation{Type: "string"},
															ObjectValidation: validation.ObjectValidation{
																AdditionalPropertiesAllowed: true,
															},
														}},
														{Name: "sameName", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressNestedOverlay/properties/sameName",
															KindValidation: validation.KindValidation{Type: "string"},
															ObjectValidation: validation.ObjectValidation{
																AdditionalPropertiesAllowed: true,
															},
														}},
													},
													AdditionalPropertiesAllowed: true,
												},
											},
											{
												SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2",
												KindValidation: validation.KindValidation{Type: "object"},
												ObjectValidation: validation.ObjectValidation{
													Required: []string{"sameName", "sealed"},
													Properties: []validation.PropertyValidation{
														{Name: "sameName", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sameName",
															KindValidation: validation.KindValidation{Type: "string"},
															ObjectValidation: validation.ObjectValidation{
																AdditionalPropertiesAllowed: true,
															},
														}},
														{Name: "sealed", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed",
															KindValidation: validation.KindValidation{Type: "object"},
															ObjectValidation: validation.ObjectValidation{
																Required: []string{"locked"},
																Properties: []validation.PropertyValidation{
																	{Name: "locked", Validation: &validation.Validation{
																		SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed/properties/locked",
																		KindValidation: validation.KindValidation{Type: "boolean"},
																		ObjectValidation: validation.ObjectValidation{
																			AdditionalPropertiesAllowed: true,
																		},
																	}},
																},
															},
														}},
													},
													AdditionalPropertiesAllowed: true,
												},
											},
										},
									}},
									{Name: "sharedName", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressViaMiddle/allOf/1/properties/sharedName",
										KindValidation: validation.KindValidation{Type: "string", Nullable: true},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
									}},
								},
								AdditionalPropertiesAllowed: true,
							},
						},
					},
				},
				{
					SchemaPointer:  "#/components/schemas/RefStressFirstAllOf/allOf/2",
					KindValidation: validation.KindValidation{Type: "object"},
					ObjectValidation: validation.ObjectValidation{
						Required: []string{"final", "nested", "nullableRequired"},
						Properties: []validation.PropertyValidation{
							{Name: "final", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFinal",
								KindValidation: validation.KindValidation{Type: "object"},
								ObjectValidation: validation.ObjectValidation{
									Required: []string{"finalCode", "sharedName"},
									Properties: []validation.PropertyValidation{
										{Name: "finalCode", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "nested", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressNestedBase",
											KindValidation: validation.KindValidation{Type: "object", Nullable: true},
											ObjectValidation: validation.ObjectValidation{
												Required: []string{"sameName"},
												Properties: []validation.PropertyValidation{
													{Name: "leaf", Validation: &validation.Validation{
														SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
														KindValidation: validation.KindValidation{Type: "string"},
														ObjectValidation: validation.ObjectValidation{
															AdditionalPropertiesAllowed: true,
														},
													}},
													{Name: "sameName", Validation: &validation.Validation{
														SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
														KindValidation: validation.KindValidation{Type: "string", Nullable: true},
														ObjectValidation: validation.ObjectValidation{
															AdditionalPropertiesAllowed: true,
														},
													}},
												},
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "optionalShared", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
											KindValidation: validation.KindValidation{Type: "string", Nullable: true},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "sharedName", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
									},
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "nested", Validation: &validation.Validation{
								SchemaPointer:   "#/components/schemas/RefStressNestedCombined",
								ArrayValidation: validation.ArrayValidation{},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
								AllOfValidations: []*validation.Validation{
									{
										SchemaPointer:  "#/components/schemas/RefStressNestedBase",
										KindValidation: validation.KindValidation{Type: "object", Nullable: true},
										ObjectValidation: validation.ObjectValidation{
											Required: []string{"sameName"},
											Properties: []validation.PropertyValidation{
												{Name: "leaf", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "sameName", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
													KindValidation: validation.KindValidation{Type: "string", Nullable: true},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
											},
											AdditionalPropertiesAllowed: true,
										},
									},
									{
										SchemaPointer:  "#/components/schemas/RefStressNestedOverlay",
										KindValidation: validation.KindValidation{Type: "object"},
										ObjectValidation: validation.ObjectValidation{
											Required: []string{"sameName"},
											Properties: []validation.PropertyValidation{
												{Name: "leaf", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "sameName", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressNestedOverlay/properties/sameName",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
											},
											AdditionalPropertiesAllowed: true,
										},
									},
									{
										SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2",
										KindValidation: validation.KindValidation{Type: "object"},
										ObjectValidation: validation.ObjectValidation{
											Required: []string{"sameName", "sealed"},
											Properties: []validation.PropertyValidation{
												{Name: "sameName", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sameName",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "sealed", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed",
													KindValidation: validation.KindValidation{Type: "object"},
													ObjectValidation: validation.ObjectValidation{
														Required: []string{"locked"},
														Properties: []validation.PropertyValidation{
															{Name: "locked", Validation: &validation.Validation{
																SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed/properties/locked",
																KindValidation: validation.KindValidation{Type: "boolean"},
																ObjectValidation: validation.ObjectValidation{
																	AdditionalPropertiesAllowed: true,
																},
															}},
														},
													},
												}},
											},
											AdditionalPropertiesAllowed: true,
										},
									},
								},
							}},
							{Name: "nullableRequired", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/nullableRequired",
								KindValidation: validation.KindValidation{Type: "string", Nullable: true},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "optionalShared", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/optionalShared",
								KindValidation: validation.KindValidation{Type: "string", Nullable: true},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "sharedName", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/sharedName",
								KindValidation: validation.KindValidation{Type: "string", Nullable: true},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
						},
						AdditionalPropertiesAllowed: true,
					},
				},
			},
		},
		{
			SchemaPointer:   "#/components/schemas/RefStressSecondAllOf",
			ArrayValidation: validation.ArrayValidation{},
			ObjectValidation: validation.ObjectValidation{
				AdditionalPropertiesAllowed: true,
			},
			AllOfValidations: []*validation.Validation{
				{
					SchemaPointer:   "#/components/schemas/RefStressOtherMiddle",
					ArrayValidation: validation.ArrayValidation{},
					ObjectValidation: validation.ObjectValidation{
						AdditionalPropertiesAllowed: true,
					},
					AllOfValidations: []*validation.Validation{
						{
							SchemaPointer:  "#/components/schemas/RefStressFinal",
							KindValidation: validation.KindValidation{Type: "object"},
							ObjectValidation: validation.ObjectValidation{
								Required: []string{"finalCode", "sharedName"},
								Properties: []validation.PropertyValidation{
									{Name: "finalCode", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
										KindValidation: validation.KindValidation{Type: "string"},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
									}},
									{Name: "nested", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressNestedBase",
										KindValidation: validation.KindValidation{Type: "object", Nullable: true},
										ObjectValidation: validation.ObjectValidation{
											Required: []string{"sameName"},
											Properties: []validation.PropertyValidation{
												{Name: "leaf", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "sameName", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
													KindValidation: validation.KindValidation{Type: "string", Nullable: true},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
											},
											AdditionalPropertiesAllowed: true,
										},
									}},
									{Name: "optionalShared", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
										KindValidation: validation.KindValidation{Type: "string", Nullable: true},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
									}},
									{Name: "sharedName", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
										KindValidation: validation.KindValidation{Type: "string"},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
									}},
								},
								AdditionalPropertiesAllowed: true,
							},
						},
						{
							SchemaPointer:  "#/components/schemas/RefStressOtherMiddle/allOf/1",
							KindValidation: validation.KindValidation{Type: "object"},
							ObjectValidation: validation.ObjectValidation{
								Required: []string{"metadata", "rootFlag"},
								Properties: []validation.PropertyValidation{
									{Name: "final", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressFinal",
										KindValidation: validation.KindValidation{Type: "object"},
										ObjectValidation: validation.ObjectValidation{
											Required: []string{"finalCode", "sharedName"},
											Properties: []validation.PropertyValidation{
												{Name: "finalCode", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "nested", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressNestedBase",
													KindValidation: validation.KindValidation{Type: "object", Nullable: true},
													ObjectValidation: validation.ObjectValidation{
														Required: []string{"sameName"},
														Properties: []validation.PropertyValidation{
															{Name: "leaf", Validation: &validation.Validation{
																SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
																KindValidation: validation.KindValidation{Type: "string"},
																ObjectValidation: validation.ObjectValidation{
																	AdditionalPropertiesAllowed: true,
																},
															}},
															{Name: "sameName", Validation: &validation.Validation{
																SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
																KindValidation: validation.KindValidation{Type: "string", Nullable: true},
																ObjectValidation: validation.ObjectValidation{
																	AdditionalPropertiesAllowed: true,
																},
															}},
														},
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "optionalShared", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
													KindValidation: validation.KindValidation{Type: "string", Nullable: true},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "sharedName", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
											},
											AdditionalPropertiesAllowed: true,
										},
									}},
									{Name: "metadata", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressOtherMiddle/allOf/1/properties/metadata",
										KindValidation: validation.KindValidation{Type: "object"},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
											AdditionalPropertiesValidation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
												KindValidation: validation.KindValidation{Type: "string"},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											},
										},
									}},
									{Name: "rootFlag", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressOtherMiddle/allOf/1/properties/rootFlag",
										KindValidation: validation.KindValidation{Type: "boolean"},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
									}},
								},
								AdditionalPropertiesAllowed: true,
							},
						},
					},
				},
				{
					SchemaPointer:  "#/components/schemas/RefStressSecondAllOf/allOf/1",
					KindValidation: validation.KindValidation{Type: "object"},
					ObjectValidation: validation.ObjectValidation{
						Required: []string{"count", "finals", "metadata", "rootFlag"},
						Properties: []validation.PropertyValidation{
							{Name: "count", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/count",
								KindValidation: validation.KindValidation{Type: "number"},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "finals", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/finals",
								KindValidation: validation.KindValidation{Type: "array"},
								ArrayValidation: validation.ArrayValidation{
									Items: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressFinal",
										KindValidation: validation.KindValidation{Type: "object"},
										ObjectValidation: validation.ObjectValidation{
											Required: []string{"finalCode", "sharedName"},
											Properties: []validation.PropertyValidation{
												{Name: "finalCode", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "nested", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressNestedBase",
													KindValidation: validation.KindValidation{Type: "object", Nullable: true},
													ObjectValidation: validation.ObjectValidation{
														Required: []string{"sameName"},
														Properties: []validation.PropertyValidation{
															{Name: "leaf", Validation: &validation.Validation{
																SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
																KindValidation: validation.KindValidation{Type: "string"},
																ObjectValidation: validation.ObjectValidation{
																	AdditionalPropertiesAllowed: true,
																},
															}},
															{Name: "sameName", Validation: &validation.Validation{
																SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
																KindValidation: validation.KindValidation{Type: "string", Nullable: true},
																ObjectValidation: validation.ObjectValidation{
																	AdditionalPropertiesAllowed: true,
																},
															}},
														},
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "optionalShared", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
													KindValidation: validation.KindValidation{Type: "string", Nullable: true},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "sharedName", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
											},
											AdditionalPropertiesAllowed: true,
										},
									},
								},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "metadata", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/metadata",
								KindValidation: validation.KindValidation{Type: "object"},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
									AdditionalPropertiesValidation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
										KindValidation: validation.KindValidation{Type: "string"},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
									},
								},
							}},
							{Name: "rootFlag", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/rootFlag",
								KindValidation: validation.KindValidation{Type: "boolean"},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "sharedName", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/sharedName",
								KindValidation: validation.KindValidation{Type: "string"},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
						},
						AdditionalPropertiesAllowed: true,
					},
				},
			},
		},
		{
			SchemaPointer:  "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2",
			KindValidation: validation.KindValidation{Type: "object"},
			ObjectValidation: validation.ObjectValidation{
				Required: []string{"count", "final", "finalCode", "finals", "metadata", "middleFlag", "nested", "nullableRequired", "rootFlag", "sharedName"},
				Properties: []validation.PropertyValidation{
					{Name: "count", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/count",
						KindValidation: validation.KindValidation{Type: "number"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "final", Validation: &validation.Validation{
						SchemaPointer:  "#/components/schemas/RefStressFinal",
						KindValidation: validation.KindValidation{Type: "object"},
						ObjectValidation: validation.ObjectValidation{
							Required: []string{"finalCode", "sharedName"},
							Properties: []validation.PropertyValidation{
								{Name: "finalCode", Validation: &validation.Validation{
									SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
									KindValidation: validation.KindValidation{Type: "string"},
									ObjectValidation: validation.ObjectValidation{
										AdditionalPropertiesAllowed: true,
									},
								}},
								{Name: "nested", Validation: &validation.Validation{
									SchemaPointer:  "#/components/schemas/RefStressNestedBase",
									KindValidation: validation.KindValidation{Type: "object", Nullable: true},
									ObjectValidation: validation.ObjectValidation{
										Required: []string{"sameName"},
										Properties: []validation.PropertyValidation{
											{Name: "leaf", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
												KindValidation: validation.KindValidation{Type: "string"},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											}},
											{Name: "sameName", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
												KindValidation: validation.KindValidation{Type: "string", Nullable: true},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											}},
										},
										AdditionalPropertiesAllowed: true,
									},
								}},
								{Name: "optionalShared", Validation: &validation.Validation{
									SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
									KindValidation: validation.KindValidation{Type: "string", Nullable: true},
									ObjectValidation: validation.ObjectValidation{
										AdditionalPropertiesAllowed: true,
									},
								}},
								{Name: "sharedName", Validation: &validation.Validation{
									SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
									KindValidation: validation.KindValidation{Type: "string"},
									ObjectValidation: validation.ObjectValidation{
										AdditionalPropertiesAllowed: true,
									},
								}},
							},
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "finalCode", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/finalCode",
						KindValidation: validation.KindValidation{Type: "string"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "finals", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/finals",
						KindValidation: validation.KindValidation{Type: "array"},
						ArrayValidation: validation.ArrayValidation{
							Items: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFinal",
								KindValidation: validation.KindValidation{Type: "object"},
								ObjectValidation: validation.ObjectValidation{
									Required: []string{"finalCode", "sharedName"},
									Properties: []validation.PropertyValidation{
										{Name: "finalCode", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "nested", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressNestedBase",
											KindValidation: validation.KindValidation{Type: "object", Nullable: true},
											ObjectValidation: validation.ObjectValidation{
												Required: []string{"sameName"},
												Properties: []validation.PropertyValidation{
													{Name: "leaf", Validation: &validation.Validation{
														SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
														KindValidation: validation.KindValidation{Type: "string"},
														ObjectValidation: validation.ObjectValidation{
															AdditionalPropertiesAllowed: true,
														},
													}},
													{Name: "sameName", Validation: &validation.Validation{
														SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
														KindValidation: validation.KindValidation{Type: "string", Nullable: true},
														ObjectValidation: validation.ObjectValidation{
															AdditionalPropertiesAllowed: true,
														},
													}},
												},
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "optionalShared", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
											KindValidation: validation.KindValidation{Type: "string", Nullable: true},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "sharedName", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
									},
									AdditionalPropertiesAllowed: true,
								},
							},
						},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "metadata", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/metadata",
						KindValidation: validation.KindValidation{Type: "object"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
							AdditionalPropertiesValidation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
								KindValidation: validation.KindValidation{Type: "string"},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							},
						},
					}},
					{Name: "middleFlag", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/middleFlag",
						KindValidation: validation.KindValidation{Type: "boolean"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "nested", Validation: &validation.Validation{
						SchemaPointer:   "#/components/schemas/RefStressNestedCombined",
						ArrayValidation: validation.ArrayValidation{},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
						AllOfValidations: []*validation.Validation{
							{
								SchemaPointer:  "#/components/schemas/RefStressNestedBase",
								KindValidation: validation.KindValidation{Type: "object", Nullable: true},
								ObjectValidation: validation.ObjectValidation{
									Required: []string{"sameName"},
									Properties: []validation.PropertyValidation{
										{Name: "leaf", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "sameName", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
											KindValidation: validation.KindValidation{Type: "string", Nullable: true},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
									},
									AdditionalPropertiesAllowed: true,
								},
							},
							{
								SchemaPointer:  "#/components/schemas/RefStressNestedOverlay",
								KindValidation: validation.KindValidation{Type: "object"},
								ObjectValidation: validation.ObjectValidation{
									Required: []string{"sameName"},
									Properties: []validation.PropertyValidation{
										{Name: "leaf", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "sameName", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressNestedOverlay/properties/sameName",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
									},
									AdditionalPropertiesAllowed: true,
								},
							},
							{
								SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2",
								KindValidation: validation.KindValidation{Type: "object"},
								ObjectValidation: validation.ObjectValidation{
									Required: []string{"sameName", "sealed"},
									Properties: []validation.PropertyValidation{
										{Name: "sameName", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sameName",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "sealed", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed",
											KindValidation: validation.KindValidation{Type: "object"},
											ObjectValidation: validation.ObjectValidation{
												Required: []string{"locked"},
												Properties: []validation.PropertyValidation{
													{Name: "locked", Validation: &validation.Validation{
														SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed/properties/locked",
														KindValidation: validation.KindValidation{Type: "boolean"},
														ObjectValidation: validation.ObjectValidation{
															AdditionalPropertiesAllowed: true,
														},
													}},
												},
											},
										}},
									},
									AdditionalPropertiesAllowed: true,
								},
							},
						},
					}},
					{Name: "nullableRequired", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/nullableRequired",
						KindValidation: validation.KindValidation{Type: "string", Nullable: true},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "optionalCode", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/optionalCode",
						KindValidation: validation.KindValidation{Type: "string"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "optionalShared", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/optionalShared",
						KindValidation: validation.KindValidation{Type: "string", Nullable: true},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "rootFlag", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/rootFlag",
						KindValidation: validation.KindValidation{Type: "boolean"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "sharedName", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object/post/requestBody/content/application~1json/schema/allOf/2/properties/sharedName",
						KindValidation: validation.KindValidation{Type: "string"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
				},
			},
		},
	},
}

var refStressObjectPut = &validation.Validation{
	SchemaPointer:   "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema",
	BodyRequired:    true,
	ArrayValidation: validation.ArrayValidation{},
	ObjectValidation: validation.ObjectValidation{
		AdditionalPropertiesAllowed: true,
	},
	AllOfValidations: []*validation.Validation{
		{
			SchemaPointer:   "#/components/schemas/RefStressFirstAllOf",
			ArrayValidation: validation.ArrayValidation{},
			ObjectValidation: validation.ObjectValidation{
				AdditionalPropertiesAllowed: true,
			},
			AllOfValidations: []*validation.Validation{
				{
					SchemaPointer:  "#/components/schemas/RefStressFinal",
					KindValidation: validation.KindValidation{Type: "object"},
					ObjectValidation: validation.ObjectValidation{
						Required: []string{"finalCode", "sharedName"},
						Properties: []validation.PropertyValidation{
							{Name: "finalCode", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
								KindValidation: validation.KindValidation{Type: "string"},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "nested", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressNestedBase",
								KindValidation: validation.KindValidation{Type: "object", Nullable: true},
								ObjectValidation: validation.ObjectValidation{
									Required: []string{"sameName"},
									Properties: []validation.PropertyValidation{
										{Name: "leaf", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "sameName", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
											KindValidation: validation.KindValidation{Type: "string", Nullable: true},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
									},
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "optionalShared", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
								KindValidation: validation.KindValidation{Type: "string", Nullable: true},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "sharedName", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
								KindValidation: validation.KindValidation{Type: "string"},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
						},
						AdditionalPropertiesAllowed: true,
					},
				},
				{
					SchemaPointer:   "#/components/schemas/RefStressViaMiddle",
					ArrayValidation: validation.ArrayValidation{},
					ObjectValidation: validation.ObjectValidation{
						AdditionalPropertiesAllowed: true,
					},
					AllOfValidations: []*validation.Validation{
						{
							SchemaPointer:   "#/components/schemas/RefStressMiddleAllOf",
							ArrayValidation: validation.ArrayValidation{},
							ObjectValidation: validation.ObjectValidation{
								AdditionalPropertiesAllowed: true,
							},
							AllOfValidations: []*validation.Validation{
								{
									SchemaPointer:  "#/components/schemas/RefStressFinal",
									KindValidation: validation.KindValidation{Type: "object"},
									ObjectValidation: validation.ObjectValidation{
										Required: []string{"finalCode", "sharedName"},
										Properties: []validation.PropertyValidation{
											{Name: "finalCode", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
												KindValidation: validation.KindValidation{Type: "string"},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											}},
											{Name: "nested", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressNestedBase",
												KindValidation: validation.KindValidation{Type: "object", Nullable: true},
												ObjectValidation: validation.ObjectValidation{
													Required: []string{"sameName"},
													Properties: []validation.PropertyValidation{
														{Name: "leaf", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
															KindValidation: validation.KindValidation{Type: "string"},
															ObjectValidation: validation.ObjectValidation{
																AdditionalPropertiesAllowed: true,
															},
														}},
														{Name: "sameName", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
															KindValidation: validation.KindValidation{Type: "string", Nullable: true},
															ObjectValidation: validation.ObjectValidation{
																AdditionalPropertiesAllowed: true,
															},
														}},
													},
													AdditionalPropertiesAllowed: true,
												},
											}},
											{Name: "optionalShared", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
												KindValidation: validation.KindValidation{Type: "string", Nullable: true},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											}},
											{Name: "sharedName", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
												KindValidation: validation.KindValidation{Type: "string"},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											}},
										},
										AdditionalPropertiesAllowed: true,
									},
								},
								{
									SchemaPointer:  "#/components/schemas/RefStressMiddleAllOf/allOf/1",
									KindValidation: validation.KindValidation{Type: "object", Nullable: true},
									ObjectValidation: validation.ObjectValidation{
										Required: []string{"sharedName"},
										Properties: []validation.PropertyValidation{
											{Name: "optionalCode", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressMiddleAllOf/allOf/1/properties/optionalCode",
												KindValidation: validation.KindValidation{Type: "string"},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											}},
											{Name: "sharedName", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressMiddleAllOf/allOf/1/properties/sharedName",
												KindValidation: validation.KindValidation{Type: "string"},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											}},
										},
										AdditionalPropertiesAllowed: true,
									},
								},
							},
						},
						{
							SchemaPointer:  "#/components/schemas/RefStressViaMiddle/allOf/1",
							KindValidation: validation.KindValidation{Type: "object"},
							ObjectValidation: validation.ObjectValidation{
								Required: []string{"middleFlag", "sharedName"},
								Properties: []validation.PropertyValidation{
									{Name: "middleFlag", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressViaMiddle/allOf/1/properties/middleFlag",
										KindValidation: validation.KindValidation{Type: "boolean"},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
									}},
									{Name: "nested", Validation: &validation.Validation{
										SchemaPointer:   "#/components/schemas/RefStressNestedCombined",
										ArrayValidation: validation.ArrayValidation{},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
										AllOfValidations: []*validation.Validation{
											{
												SchemaPointer:  "#/components/schemas/RefStressNestedBase",
												KindValidation: validation.KindValidation{Type: "object", Nullable: true},
												ObjectValidation: validation.ObjectValidation{
													Required: []string{"sameName"},
													Properties: []validation.PropertyValidation{
														{Name: "leaf", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
															KindValidation: validation.KindValidation{Type: "string"},
															ObjectValidation: validation.ObjectValidation{
																AdditionalPropertiesAllowed: true,
															},
														}},
														{Name: "sameName", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
															KindValidation: validation.KindValidation{Type: "string", Nullable: true},
															ObjectValidation: validation.ObjectValidation{
																AdditionalPropertiesAllowed: true,
															},
														}},
													},
													AdditionalPropertiesAllowed: true,
												},
											},
											{
												SchemaPointer:  "#/components/schemas/RefStressNestedOverlay",
												KindValidation: validation.KindValidation{Type: "object"},
												ObjectValidation: validation.ObjectValidation{
													Required: []string{"sameName"},
													Properties: []validation.PropertyValidation{
														{Name: "leaf", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
															KindValidation: validation.KindValidation{Type: "string"},
															ObjectValidation: validation.ObjectValidation{
																AdditionalPropertiesAllowed: true,
															},
														}},
														{Name: "sameName", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressNestedOverlay/properties/sameName",
															KindValidation: validation.KindValidation{Type: "string"},
															ObjectValidation: validation.ObjectValidation{
																AdditionalPropertiesAllowed: true,
															},
														}},
													},
													AdditionalPropertiesAllowed: true,
												},
											},
											{
												SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2",
												KindValidation: validation.KindValidation{Type: "object"},
												ObjectValidation: validation.ObjectValidation{
													Required: []string{"sameName", "sealed"},
													Properties: []validation.PropertyValidation{
														{Name: "sameName", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sameName",
															KindValidation: validation.KindValidation{Type: "string"},
															ObjectValidation: validation.ObjectValidation{
																AdditionalPropertiesAllowed: true,
															},
														}},
														{Name: "sealed", Validation: &validation.Validation{
															SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed",
															KindValidation: validation.KindValidation{Type: "object"},
															ObjectValidation: validation.ObjectValidation{
																Required: []string{"locked"},
																Properties: []validation.PropertyValidation{
																	{Name: "locked", Validation: &validation.Validation{
																		SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed/properties/locked",
																		KindValidation: validation.KindValidation{Type: "boolean"},
																		ObjectValidation: validation.ObjectValidation{
																			AdditionalPropertiesAllowed: true,
																		},
																	}},
																},
															},
														}},
													},
													AdditionalPropertiesAllowed: true,
												},
											},
										},
									}},
									{Name: "sharedName", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressViaMiddle/allOf/1/properties/sharedName",
										KindValidation: validation.KindValidation{Type: "string", Nullable: true},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
									}},
								},
								AdditionalPropertiesAllowed: true,
							},
						},
					},
				},
				{
					SchemaPointer:  "#/components/schemas/RefStressFirstAllOf/allOf/2",
					KindValidation: validation.KindValidation{Type: "object"},
					ObjectValidation: validation.ObjectValidation{
						Required: []string{"final", "nested", "nullableRequired"},
						Properties: []validation.PropertyValidation{
							{Name: "final", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFinal",
								KindValidation: validation.KindValidation{Type: "object"},
								ObjectValidation: validation.ObjectValidation{
									Required: []string{"finalCode", "sharedName"},
									Properties: []validation.PropertyValidation{
										{Name: "finalCode", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "nested", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressNestedBase",
											KindValidation: validation.KindValidation{Type: "object", Nullable: true},
											ObjectValidation: validation.ObjectValidation{
												Required: []string{"sameName"},
												Properties: []validation.PropertyValidation{
													{Name: "leaf", Validation: &validation.Validation{
														SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
														KindValidation: validation.KindValidation{Type: "string"},
														ObjectValidation: validation.ObjectValidation{
															AdditionalPropertiesAllowed: true,
														},
													}},
													{Name: "sameName", Validation: &validation.Validation{
														SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
														KindValidation: validation.KindValidation{Type: "string", Nullable: true},
														ObjectValidation: validation.ObjectValidation{
															AdditionalPropertiesAllowed: true,
														},
													}},
												},
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "optionalShared", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
											KindValidation: validation.KindValidation{Type: "string", Nullable: true},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "sharedName", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
									},
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "nested", Validation: &validation.Validation{
								SchemaPointer:   "#/components/schemas/RefStressNestedCombined",
								ArrayValidation: validation.ArrayValidation{},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
								AllOfValidations: []*validation.Validation{
									{
										SchemaPointer:  "#/components/schemas/RefStressNestedBase",
										KindValidation: validation.KindValidation{Type: "object", Nullable: true},
										ObjectValidation: validation.ObjectValidation{
											Required: []string{"sameName"},
											Properties: []validation.PropertyValidation{
												{Name: "leaf", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "sameName", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
													KindValidation: validation.KindValidation{Type: "string", Nullable: true},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
											},
											AdditionalPropertiesAllowed: true,
										},
									},
									{
										SchemaPointer:  "#/components/schemas/RefStressNestedOverlay",
										KindValidation: validation.KindValidation{Type: "object"},
										ObjectValidation: validation.ObjectValidation{
											Required: []string{"sameName"},
											Properties: []validation.PropertyValidation{
												{Name: "leaf", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "sameName", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressNestedOverlay/properties/sameName",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
											},
											AdditionalPropertiesAllowed: true,
										},
									},
									{
										SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2",
										KindValidation: validation.KindValidation{Type: "object"},
										ObjectValidation: validation.ObjectValidation{
											Required: []string{"sameName", "sealed"},
											Properties: []validation.PropertyValidation{
												{Name: "sameName", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sameName",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "sealed", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed",
													KindValidation: validation.KindValidation{Type: "object"},
													ObjectValidation: validation.ObjectValidation{
														Required: []string{"locked"},
														Properties: []validation.PropertyValidation{
															{Name: "locked", Validation: &validation.Validation{
																SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed/properties/locked",
																KindValidation: validation.KindValidation{Type: "boolean"},
																ObjectValidation: validation.ObjectValidation{
																	AdditionalPropertiesAllowed: true,
																},
															}},
														},
													},
												}},
											},
											AdditionalPropertiesAllowed: true,
										},
									},
								},
							}},
							{Name: "nullableRequired", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/nullableRequired",
								KindValidation: validation.KindValidation{Type: "string", Nullable: true},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "optionalShared", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/optionalShared",
								KindValidation: validation.KindValidation{Type: "string", Nullable: true},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "sharedName", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFirstAllOf/allOf/2/properties/sharedName",
								KindValidation: validation.KindValidation{Type: "string", Nullable: true},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
						},
						AdditionalPropertiesAllowed: true,
					},
				},
			},
		},
		{
			SchemaPointer:   "#/components/schemas/RefStressSecondAllOf",
			ArrayValidation: validation.ArrayValidation{},
			ObjectValidation: validation.ObjectValidation{
				AdditionalPropertiesAllowed: true,
			},
			AllOfValidations: []*validation.Validation{
				{
					SchemaPointer:   "#/components/schemas/RefStressOtherMiddle",
					ArrayValidation: validation.ArrayValidation{},
					ObjectValidation: validation.ObjectValidation{
						AdditionalPropertiesAllowed: true,
					},
					AllOfValidations: []*validation.Validation{
						{
							SchemaPointer:  "#/components/schemas/RefStressFinal",
							KindValidation: validation.KindValidation{Type: "object"},
							ObjectValidation: validation.ObjectValidation{
								Required: []string{"finalCode", "sharedName"},
								Properties: []validation.PropertyValidation{
									{Name: "finalCode", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
										KindValidation: validation.KindValidation{Type: "string"},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
									}},
									{Name: "nested", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressNestedBase",
										KindValidation: validation.KindValidation{Type: "object", Nullable: true},
										ObjectValidation: validation.ObjectValidation{
											Required: []string{"sameName"},
											Properties: []validation.PropertyValidation{
												{Name: "leaf", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "sameName", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
													KindValidation: validation.KindValidation{Type: "string", Nullable: true},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
											},
											AdditionalPropertiesAllowed: true,
										},
									}},
									{Name: "optionalShared", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
										KindValidation: validation.KindValidation{Type: "string", Nullable: true},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
									}},
									{Name: "sharedName", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
										KindValidation: validation.KindValidation{Type: "string"},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
									}},
								},
								AdditionalPropertiesAllowed: true,
							},
						},
						{
							SchemaPointer:  "#/components/schemas/RefStressOtherMiddle/allOf/1",
							KindValidation: validation.KindValidation{Type: "object"},
							ObjectValidation: validation.ObjectValidation{
								Required: []string{"metadata", "rootFlag"},
								Properties: []validation.PropertyValidation{
									{Name: "final", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressFinal",
										KindValidation: validation.KindValidation{Type: "object"},
										ObjectValidation: validation.ObjectValidation{
											Required: []string{"finalCode", "sharedName"},
											Properties: []validation.PropertyValidation{
												{Name: "finalCode", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "nested", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressNestedBase",
													KindValidation: validation.KindValidation{Type: "object", Nullable: true},
													ObjectValidation: validation.ObjectValidation{
														Required: []string{"sameName"},
														Properties: []validation.PropertyValidation{
															{Name: "leaf", Validation: &validation.Validation{
																SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
																KindValidation: validation.KindValidation{Type: "string"},
																ObjectValidation: validation.ObjectValidation{
																	AdditionalPropertiesAllowed: true,
																},
															}},
															{Name: "sameName", Validation: &validation.Validation{
																SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
																KindValidation: validation.KindValidation{Type: "string", Nullable: true},
																ObjectValidation: validation.ObjectValidation{
																	AdditionalPropertiesAllowed: true,
																},
															}},
														},
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "optionalShared", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
													KindValidation: validation.KindValidation{Type: "string", Nullable: true},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "sharedName", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
											},
											AdditionalPropertiesAllowed: true,
										},
									}},
									{Name: "metadata", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressOtherMiddle/allOf/1/properties/metadata",
										KindValidation: validation.KindValidation{Type: "object"},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
											AdditionalPropertiesValidation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
												KindValidation: validation.KindValidation{Type: "string"},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											},
										},
									}},
									{Name: "rootFlag", Validation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressOtherMiddle/allOf/1/properties/rootFlag",
										KindValidation: validation.KindValidation{Type: "boolean"},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
									}},
								},
								AdditionalPropertiesAllowed: true,
							},
						},
					},
				},
				{
					SchemaPointer:  "#/components/schemas/RefStressSecondAllOf/allOf/1",
					KindValidation: validation.KindValidation{Type: "object"},
					ObjectValidation: validation.ObjectValidation{
						Required: []string{"count", "finals", "metadata", "rootFlag"},
						Properties: []validation.PropertyValidation{
							{Name: "count", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/count",
								KindValidation: validation.KindValidation{Type: "number"},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "finals", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/finals",
								KindValidation: validation.KindValidation{Type: "array"},
								ArrayValidation: validation.ArrayValidation{
									Items: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressFinal",
										KindValidation: validation.KindValidation{Type: "object"},
										ObjectValidation: validation.ObjectValidation{
											Required: []string{"finalCode", "sharedName"},
											Properties: []validation.PropertyValidation{
												{Name: "finalCode", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "nested", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressNestedBase",
													KindValidation: validation.KindValidation{Type: "object", Nullable: true},
													ObjectValidation: validation.ObjectValidation{
														Required: []string{"sameName"},
														Properties: []validation.PropertyValidation{
															{Name: "leaf", Validation: &validation.Validation{
																SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
																KindValidation: validation.KindValidation{Type: "string"},
																ObjectValidation: validation.ObjectValidation{
																	AdditionalPropertiesAllowed: true,
																},
															}},
															{Name: "sameName", Validation: &validation.Validation{
																SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
																KindValidation: validation.KindValidation{Type: "string", Nullable: true},
																ObjectValidation: validation.ObjectValidation{
																	AdditionalPropertiesAllowed: true,
																},
															}},
														},
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "optionalShared", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
													KindValidation: validation.KindValidation{Type: "string", Nullable: true},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
												{Name: "sharedName", Validation: &validation.Validation{
													SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
													KindValidation: validation.KindValidation{Type: "string"},
													ObjectValidation: validation.ObjectValidation{
														AdditionalPropertiesAllowed: true,
													},
												}},
											},
											AdditionalPropertiesAllowed: true,
										},
									},
								},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "metadata", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/metadata",
								KindValidation: validation.KindValidation{Type: "object"},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
									AdditionalPropertiesValidation: &validation.Validation{
										SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
										KindValidation: validation.KindValidation{Type: "string"},
										ObjectValidation: validation.ObjectValidation{
											AdditionalPropertiesAllowed: true,
										},
									},
								},
							}},
							{Name: "rootFlag", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/rootFlag",
								KindValidation: validation.KindValidation{Type: "boolean"},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
							{Name: "sharedName", Validation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressSecondAllOf/allOf/1/properties/sharedName",
								KindValidation: validation.KindValidation{Type: "string"},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							}},
						},
						AdditionalPropertiesAllowed: true,
					},
				},
			},
		},
		{
			SchemaPointer:  "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2",
			KindValidation: validation.KindValidation{Type: "object"},
			ObjectValidation: validation.ObjectValidation{
				Required: []string{"count", "final", "finalCode", "finals", "metadata", "middleFlag", "nested", "nullableRequired", "rootFlag", "sharedName"},
				Properties: []validation.PropertyValidation{
					{Name: "count", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/count",
						KindValidation: validation.KindValidation{Type: "number"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "final", Validation: &validation.Validation{
						SchemaPointer:  "#/components/schemas/RefStressFinal",
						KindValidation: validation.KindValidation{Type: "object"},
						ObjectValidation: validation.ObjectValidation{
							Required: []string{"finalCode", "sharedName"},
							Properties: []validation.PropertyValidation{
								{Name: "finalCode", Validation: &validation.Validation{
									SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
									KindValidation: validation.KindValidation{Type: "string"},
									ObjectValidation: validation.ObjectValidation{
										AdditionalPropertiesAllowed: true,
									},
								}},
								{Name: "nested", Validation: &validation.Validation{
									SchemaPointer:  "#/components/schemas/RefStressNestedBase",
									KindValidation: validation.KindValidation{Type: "object", Nullable: true},
									ObjectValidation: validation.ObjectValidation{
										Required: []string{"sameName"},
										Properties: []validation.PropertyValidation{
											{Name: "leaf", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
												KindValidation: validation.KindValidation{Type: "string"},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											}},
											{Name: "sameName", Validation: &validation.Validation{
												SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
												KindValidation: validation.KindValidation{Type: "string", Nullable: true},
												ObjectValidation: validation.ObjectValidation{
													AdditionalPropertiesAllowed: true,
												},
											}},
										},
										AdditionalPropertiesAllowed: true,
									},
								}},
								{Name: "optionalShared", Validation: &validation.Validation{
									SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
									KindValidation: validation.KindValidation{Type: "string", Nullable: true},
									ObjectValidation: validation.ObjectValidation{
										AdditionalPropertiesAllowed: true,
									},
								}},
								{Name: "sharedName", Validation: &validation.Validation{
									SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
									KindValidation: validation.KindValidation{Type: "string"},
									ObjectValidation: validation.ObjectValidation{
										AdditionalPropertiesAllowed: true,
									},
								}},
							},
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "finalCode", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/finalCode",
						KindValidation: validation.KindValidation{Type: "string"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "finals", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/finals",
						KindValidation: validation.KindValidation{Type: "array"},
						ArrayValidation: validation.ArrayValidation{
							Items: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressFinal",
								KindValidation: validation.KindValidation{Type: "object"},
								ObjectValidation: validation.ObjectValidation{
									Required: []string{"finalCode", "sharedName"},
									Properties: []validation.PropertyValidation{
										{Name: "finalCode", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressFinal/properties/finalCode",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "nested", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressNestedBase",
											KindValidation: validation.KindValidation{Type: "object", Nullable: true},
											ObjectValidation: validation.ObjectValidation{
												Required: []string{"sameName"},
												Properties: []validation.PropertyValidation{
													{Name: "leaf", Validation: &validation.Validation{
														SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
														KindValidation: validation.KindValidation{Type: "string"},
														ObjectValidation: validation.ObjectValidation{
															AdditionalPropertiesAllowed: true,
														},
													}},
													{Name: "sameName", Validation: &validation.Validation{
														SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
														KindValidation: validation.KindValidation{Type: "string", Nullable: true},
														ObjectValidation: validation.ObjectValidation{
															AdditionalPropertiesAllowed: true,
														},
													}},
												},
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "optionalShared", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressFinal/properties/optionalShared",
											KindValidation: validation.KindValidation{Type: "string", Nullable: true},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "sharedName", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressFinal/properties/sharedName",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
									},
									AdditionalPropertiesAllowed: true,
								},
							},
						},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "metadata", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/metadata",
						KindValidation: validation.KindValidation{Type: "object"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
							AdditionalPropertiesValidation: &validation.Validation{
								SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
								KindValidation: validation.KindValidation{Type: "string"},
								ObjectValidation: validation.ObjectValidation{
									AdditionalPropertiesAllowed: true,
								},
							},
						},
					}},
					{Name: "middleFlag", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/middleFlag",
						KindValidation: validation.KindValidation{Type: "boolean"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "nested", Validation: &validation.Validation{
						SchemaPointer:   "#/components/schemas/RefStressNestedCombined",
						ArrayValidation: validation.ArrayValidation{},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
						AllOfValidations: []*validation.Validation{
							{
								SchemaPointer:  "#/components/schemas/RefStressNestedBase",
								KindValidation: validation.KindValidation{Type: "object", Nullable: true},
								ObjectValidation: validation.ObjectValidation{
									Required: []string{"sameName"},
									Properties: []validation.PropertyValidation{
										{Name: "leaf", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "sameName", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressNestedBase/properties/sameName",
											KindValidation: validation.KindValidation{Type: "string", Nullable: true},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
									},
									AdditionalPropertiesAllowed: true,
								},
							},
							{
								SchemaPointer:  "#/components/schemas/RefStressNestedOverlay",
								KindValidation: validation.KindValidation{Type: "object"},
								ObjectValidation: validation.ObjectValidation{
									Required: []string{"sameName"},
									Properties: []validation.PropertyValidation{
										{Name: "leaf", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressMetadataValue",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "sameName", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressNestedOverlay/properties/sameName",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
									},
									AdditionalPropertiesAllowed: true,
								},
							},
							{
								SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2",
								KindValidation: validation.KindValidation{Type: "object"},
								ObjectValidation: validation.ObjectValidation{
									Required: []string{"sameName", "sealed"},
									Properties: []validation.PropertyValidation{
										{Name: "sameName", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sameName",
											KindValidation: validation.KindValidation{Type: "string"},
											ObjectValidation: validation.ObjectValidation{
												AdditionalPropertiesAllowed: true,
											},
										}},
										{Name: "sealed", Validation: &validation.Validation{
											SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed",
											KindValidation: validation.KindValidation{Type: "object"},
											ObjectValidation: validation.ObjectValidation{
												Required: []string{"locked"},
												Properties: []validation.PropertyValidation{
													{Name: "locked", Validation: &validation.Validation{
														SchemaPointer:  "#/components/schemas/RefStressNestedCombined/allOf/2/properties/sealed/properties/locked",
														KindValidation: validation.KindValidation{Type: "boolean"},
														ObjectValidation: validation.ObjectValidation{
															AdditionalPropertiesAllowed: true,
														},
													}},
												},
											},
										}},
									},
									AdditionalPropertiesAllowed: true,
								},
							},
						},
					}},
					{Name: "nullableRequired", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/nullableRequired",
						KindValidation: validation.KindValidation{Type: "string", Nullable: true},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "optionalCode", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/optionalCode",
						KindValidation: validation.KindValidation{Type: "string"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "optionalShared", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/optionalShared",
						KindValidation: validation.KindValidation{Type: "string", Nullable: true},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "rootFlag", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/rootFlag",
						KindValidation: validation.KindValidation{Type: "boolean"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
					{Name: "sharedName", Validation: &validation.Validation{
						SchemaPointer:  "#/paths/~1ref-stress-object-put/put/requestBody/content/application~1json/schema/allOf/2/properties/sharedName",
						KindValidation: validation.KindValidation{Type: "string"},
						ObjectValidation: validation.ObjectValidation{
							AdditionalPropertiesAllowed: true,
						},
					}},
				},
			},
		},
	},
}

var stringNoFormatNotNullable = &validation.Validation{
	SchemaPointer:  "#/paths/~1string-no-format-not-nullable/post/requestBody/content/application~1json/schema",
	BodyRequired:   true,
	KindValidation: validation.KindValidation{Type: "string"},
	ObjectValidation: validation.ObjectValidation{
		AdditionalPropertiesAllowed: true,
	},
}

var stringNoFormatNullable = &validation.Validation{
	SchemaPointer:  "#/paths/~1string-no-format-nullable/post/requestBody/content/application~1json/schema",
	BodyRequired:   true,
	KindValidation: validation.KindValidation{Type: "string", Nullable: true},
	ObjectValidation: validation.ObjectValidation{
		AdditionalPropertiesAllowed: true,
	},
}
