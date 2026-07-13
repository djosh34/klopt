package decode_tests

import (
	"encoding/json"
	"testing"

	"decode_and_validate_generator/pkg/decode/example"

	"github.com/stretchr/testify/require"
)

// requiredNullableString returns a non-null required nullable-string value.
func requiredNullableString(value string) example.ObjectKeysAdditionalPropertiesFalseRequiredNullableString {
	return example.ObjectKeysAdditionalPropertiesFalseRequiredNullableString{Value: new(value)}
}

// optionalNullableString returns a present non-null optional nullable-string value.
func optionalNullableString(value string) *example.ObjectKeysAdditionalPropertiesFalseOptionalNullableString {
	return new(example.ObjectKeysAdditionalPropertiesFalseOptionalNullableString{Value: new(value)})
}

// nullOptionalNullableString returns a present null optional nullable-string value.
func nullOptionalNullableString() *example.ObjectKeysAdditionalPropertiesFalseOptionalNullableString {
	return new(example.ObjectKeysAdditionalPropertiesFalseOptionalNullableString)
}

// optionalNotNullableString returns a present optional non-nullable string value.
func optionalNotNullableString(value string) *example.ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString {
	converted := example.ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString(value)

	return &converted
}

// TestObjectKeysAdditionalPropertiesFalseDecodeAllowedWays checks accepted closed-object forms.
func TestObjectKeysAdditionalPropertiesFalseDecodeAllowedWays(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		inputJSON      string
		expectedStruct example.ObjectKeysAdditionalPropertiesFalse
		expectedErr    error
	}{
		"required nullable non-null optional nullable omitted optional not nullable omitted": {
			inputJSON: `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString:    requiredNullableString("required-nullable"),
				RequiredNotNullableString: "required-not-nullable",
			},
			expectedErr: nil,
		},
		"required nullable non-null optional nullable omitted optional not nullable non-null": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable"` +
				`,"optionalNotNullableString":"optional-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString:    requiredNullableString("required-nullable"),
				RequiredNotNullableString: "required-not-nullable",
				OptionalNotNullableString: optionalNotNullableString("optional-not-nullable"),
			},
			expectedErr: nil,
		},
		"required nullable non-null optional nullable null optional not nullable omitted": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable","optionalNullableString":null}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString:    requiredNullableString("required-nullable"),
				RequiredNotNullableString: "required-not-nullable",
				OptionalNullableString:    nullOptionalNullableString(),
			},
			expectedErr: nil,
		},
		"required nullable non-null optional nullable null optional not nullable non-null": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable","optionalNullableString":null` +
				`,"optionalNotNullableString":"optional-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString:    requiredNullableString("required-nullable"),
				RequiredNotNullableString: "required-not-nullable",
				OptionalNullableString:    nullOptionalNullableString(),
				OptionalNotNullableString: optionalNotNullableString("optional-not-nullable"),
			},
			expectedErr: nil,
		},
		"required nullable non-null optional nullable non-null optional not nullable omitted": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable"` +
				`,"optionalNullableString":"optional-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString:    requiredNullableString("required-nullable"),
				RequiredNotNullableString: "required-not-nullable",
				OptionalNullableString:    optionalNullableString("optional-nullable"),
			},
			expectedErr: nil,
		},
		"required nullable non-null optional nullable non-null optional not nullable non-null": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable"` +
				`,"optionalNullableString":"optional-nullable"` +
				`,"optionalNotNullableString":"optional-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString:    requiredNullableString("required-nullable"),
				RequiredNotNullableString: "required-not-nullable",
				OptionalNullableString:    optionalNullableString("optional-nullable"),
				OptionalNotNullableString: optionalNotNullableString("optional-not-nullable"),
			},
			expectedErr: nil,
		},
		"required nullable null optional nullable omitted optional not nullable omitted": {
			inputJSON: `{"requiredNullableString":null,"requiredNotNullableString":"required-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNotNullableString: "required-not-nullable",
			},
			expectedErr: nil,
		},
		"required nullable null optional nullable omitted optional not nullable non-null": {
			inputJSON: `{"requiredNullableString":null,"requiredNotNullableString":"required-not-nullable"` +
				`,"optionalNotNullableString":"optional-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNotNullableString: "required-not-nullable",
				OptionalNotNullableString: optionalNotNullableString("optional-not-nullable"),
			},
			expectedErr: nil,
		},
		"required nullable null optional nullable null optional not nullable omitted": {
			inputJSON: `{"requiredNullableString":null,"requiredNotNullableString":"required-not-nullable"` +
				`,"optionalNullableString":null}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNotNullableString: "required-not-nullable",
				OptionalNullableString:    nullOptionalNullableString(),
			},
			expectedErr: nil,
		},
		"required nullable null optional nullable null optional not nullable non-null": {
			inputJSON: `{"requiredNullableString":null,"requiredNotNullableString":"required-not-nullable"` +
				`,"optionalNullableString":null,"optionalNotNullableString":"optional-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNotNullableString: "required-not-nullable",
				OptionalNullableString:    nullOptionalNullableString(),
				OptionalNotNullableString: optionalNotNullableString("optional-not-nullable"),
			},
			expectedErr: nil,
		},
		"required nullable null optional nullable non-null optional not nullable omitted": {
			inputJSON: `{"requiredNullableString":null,"requiredNotNullableString":"required-not-nullable"` +
				`,"optionalNullableString":"optional-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNotNullableString: "required-not-nullable",
				OptionalNullableString:    optionalNullableString("optional-nullable"),
			},
			expectedErr: nil,
		},
		"required nullable null optional nullable non-null optional not nullable non-null": {
			inputJSON: `{"requiredNullableString":null,"requiredNotNullableString":"required-not-nullable"` +
				`,"optionalNullableString":"optional-nullable"` +
				`,"optionalNotNullableString":"optional-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNotNullableString: "required-not-nullable",
				OptionalNullableString:    optionalNullableString("optional-nullable"),
				OptionalNotNullableString: optionalNotNullableString("optional-not-nullable"),
			},
			expectedErr: nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var actualStruct example.ObjectKeysAdditionalPropertiesFalse

			// Act
			err := actualStruct.UnmarshalJSON([]byte(tt.inputJSON))

			// Assert
			if tt.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tt.expectedErr)
			}

			require.Equal(t, tt.expectedStruct, actualStruct)
		})
	}
}

// TestObjectKeysAdditionalPropertiesFalseDecodeRejectsInvalidShapes checks rejected closed-object forms.
func TestObjectKeysAdditionalPropertiesFalseDecodeRejectsInvalidShapes(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		inputJSON   string
		expectedErr error
	}{
		"not object array": {
			inputJSON:   `[]`,
			expectedErr: example.NotAnObjectError,
		},
		"not object string": {
			inputJSON:   `"not-object"`,
			expectedErr: example.NotAnObjectError,
		},
		"not object null": {
			inputJSON:   `null`,
			expectedErr: example.NotAnObjectError,
		},
		"not object number": {
			inputJSON:   `123`,
			expectedErr: example.NotAnObjectError,
		},
		"not object bool": {
			inputJSON:   `true`,
			expectedErr: example.NotAnObjectError,
		},
		"additional property after required properties": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable","extra":"not-allowed"}`,
			expectedErr: example.AdditionalPropertyError,
		},
		"additional property before required properties": {
			inputJSON: `{"extra":"not-allowed","requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable"}`,
			expectedErr: example.AdditionalPropertyError,
		},
		"additional property after optional properties": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable"` +
				`,"optionalNullableString":"optional-nullable"` +
				`,"optionalNotNullableString":"optional-not-nullable","extra":"not-allowed"}`,
			expectedErr: example.AdditionalPropertyError,
		},
		"both required strings missing": {
			inputJSON:   `{}`,
			expectedErr: example.MissingRequiredPropertyError,
		},
		"missing required nullable string": {
			inputJSON:   `{"requiredNotNullableString":"required-not-nullable"}`,
			expectedErr: example.MissingRequiredPropertyError,
		},
		"missing required not nullable string": {
			inputJSON:   `{"requiredNullableString":"required-nullable"}`,
			expectedErr: example.MissingRequiredPropertyError,
		},
		"required not nullable string null": {
			inputJSON:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":null}`,
			expectedErr: example.NullForNotNullableStringError,
		},
		"optional not nullable string null": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable","optionalNotNullableString":null}`,
			expectedErr: example.NullForNotNullableStringError,
		},
		"required nullable string number": {
			inputJSON:   `{"requiredNullableString":123,"requiredNotNullableString":"required-not-nullable"}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"required nullable string bool": {
			inputJSON:   `{"requiredNullableString":true,"requiredNotNullableString":"required-not-nullable"}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"required nullable string object": {
			inputJSON:   `{"requiredNullableString":{},"requiredNotNullableString":"required-not-nullable"}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"required nullable string array": {
			inputJSON:   `{"requiredNullableString":[],"requiredNotNullableString":"required-not-nullable"}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"required not nullable string number": {
			inputJSON:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":123}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"required not nullable string bool": {
			inputJSON:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":false}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"required not nullable string object": {
			inputJSON:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":{}}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"required not nullable string array": {
			inputJSON:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":[]}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional nullable string number": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable","optionalNullableString":123}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional nullable string bool": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable","optionalNullableString":false}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional nullable string object": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable","optionalNullableString":{}}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional nullable string array": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable","optionalNullableString":[]}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional not nullable string number": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable","optionalNotNullableString":123}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional not nullable string bool": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable","optionalNotNullableString":true}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional not nullable string object": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable","optionalNotNullableString":{}}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional not nullable string array": {
			inputJSON: `{"requiredNullableString":"required-nullable"` +
				`,"requiredNotNullableString":"required-not-nullable","optionalNotNullableString":[]}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var actualStruct example.ObjectKeysAdditionalPropertiesFalse

			// Act
			err := actualStruct.UnmarshalJSON([]byte(tt.inputJSON))

			// Assert
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

// TestObjectDecodeChecksEveryDuplicateValue preserves input-order validation.
func TestObjectDecodeChecksEveryDuplicateValue(t *testing.T) {
	t.Parallel()

	var decoded example.ObjectKeysAdditionalPropertiesFalse

	err := decoded.UnmarshalJSON([]byte(
		`{"requiredNullableString":123,"requiredNullableString":"valid",` +
			`"requiredNotNullableString":"required"}`,
	))
	require.ErrorIs(t, err, example.NonStringForStringSchemaError)
}

// TestObjectDecodeKeepsLastDuplicateValue preserves encoding/json assignment behavior.
func TestObjectDecodeKeepsLastDuplicateValue(t *testing.T) {
	t.Parallel()

	var decoded example.ObjectKeysAdditionalPropertiesFalse

	err := decoded.UnmarshalJSON([]byte(
		`{"requiredNullableString":"first","requiredNullableString":"last",` +
			`"requiredNotNullableString":"required"}`,
	))
	require.NoError(t, err)
	require.Equal(t, "last", *decoded.RequiredNullableString.Value)
}

// TestObjectDecodeReplacesRequiredDuplicateObjectValue preserves whole-value assignment behavior.
func TestObjectDecodeReplacesRequiredDuplicateObjectValue(t *testing.T) {
	t.Parallel()

	var decoded example.RefStressObjectPutAllOf1AllOf3

	err := decoded.UnmarshalJSON([]byte(`{
		"final":{"finalCode":"first","sharedName":"first","optionalShared":"stale"},
		"final":{"finalCode":"last","sharedName":"last"},
		"nested":{"sameName":"same","sealed":{"locked":true}},
		"nullableRequired":null
	}`))
	require.NoError(t, err)
	require.Nil(t, decoded.Final.OptionalShared)
}

// TestAllOfMarshalPreservesEmbeddedFieldSelection protects generated JSON wire behavior.
func TestAllOfMarshalPreservesEmbeddedFieldSelection(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(example.RefStressObjectPutAllOf1{})
	require.NoError(t, err)
	require.JSONEq(t, `{
		"finalCode": "",
		"middleFlag": false,
		"final": {"finalCode": "", "sharedName": ""},
		"nullableRequired": {"Value": null}
	}`, string(data))
}
