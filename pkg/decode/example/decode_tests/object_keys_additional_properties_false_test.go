package decode_tests

import (
	"strings"
	"testing"

	"decode_and_validate_generator/pkg/decode/example"
	"decode_and_validate_generator/pkg/peekjson"

	"github.com/stretchr/testify/require"
)

func TestObjectKeysAdditionalPropertiesFalseDecodeAllowedWays(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		inputJson      string
		expectedStruct example.ObjectKeysAdditionalPropertiesFalse
		expectedErr    error
	}{
		"required nullable non-null optional nullable omitted optional not nullable omitted": {
			inputJson: `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNullableString{
					Inner: new("required-nullable"),
				},
				RequiredNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString{
					Inner: "required-not-nullable",
				},
			},
			expectedErr: nil,
		},
		"required nullable non-null optional nullable omitted optional not nullable non-null": {
			inputJson: `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","optionalNotNullableString":"optional-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNullableString{
					Inner: new("required-nullable"),
				},
				RequiredNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString{
					Inner: "required-not-nullable",
				},
				OptionalNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString{
					Inner: "optional-not-nullable",
				},
			},
			expectedErr: nil,
		},
		"required nullable non-null optional nullable null optional not nullable omitted": {
			inputJson: `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","optionalNullableString":null}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNullableString{
					Inner: new("required-nullable"),
				},
				RequiredNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString{
					Inner: "required-not-nullable",
				},
				OptionalNullableString: &example.ObjectKeysAdditionalPropertiesFalseOptionalNullableString{},
			},
			expectedErr: nil,
		},
		"required nullable non-null optional nullable null optional not nullable non-null": {
			inputJson: `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","optionalNullableString":null,"optionalNotNullableString":"optional-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNullableString{
					Inner: new("required-nullable"),
				},
				RequiredNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString{
					Inner: "required-not-nullable",
				},
				OptionalNullableString: &example.ObjectKeysAdditionalPropertiesFalseOptionalNullableString{},
				OptionalNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString{
					Inner: "optional-not-nullable",
				},
			},
			expectedErr: nil,
		},
		"required nullable non-null optional nullable non-null optional not nullable omitted": {
			inputJson: `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","optionalNullableString":"optional-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNullableString{
					Inner: new("required-nullable"),
				},
				RequiredNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString{
					Inner: "required-not-nullable",
				},
				OptionalNullableString: &example.ObjectKeysAdditionalPropertiesFalseOptionalNullableString{
					Inner: new("optional-nullable"),
				},
			},
			expectedErr: nil,
		},
		"required nullable non-null optional nullable non-null optional not nullable non-null": {
			inputJson: `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","optionalNullableString":"optional-nullable","optionalNotNullableString":"optional-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNullableString{
					Inner: new("required-nullable"),
				},
				RequiredNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString{
					Inner: "required-not-nullable",
				},
				OptionalNullableString: &example.ObjectKeysAdditionalPropertiesFalseOptionalNullableString{
					Inner: new("optional-nullable"),
				},
				OptionalNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString{
					Inner: "optional-not-nullable",
				},
			},
			expectedErr: nil,
		},
		"required nullable null optional nullable omitted optional not nullable omitted": {
			inputJson: `{"requiredNullableString":null,"requiredNotNullableString":"required-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNullableString{},
				RequiredNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString{
					Inner: "required-not-nullable",
				},
			},
			expectedErr: nil,
		},
		"required nullable null optional nullable omitted optional not nullable non-null": {
			inputJson: `{"requiredNullableString":null,"requiredNotNullableString":"required-not-nullable","optionalNotNullableString":"optional-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNullableString{},
				RequiredNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString{
					Inner: "required-not-nullable",
				},
				OptionalNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString{
					Inner: "optional-not-nullable",
				},
			},
			expectedErr: nil,
		},
		"required nullable null optional nullable null optional not nullable omitted": {
			inputJson: `{"requiredNullableString":null,"requiredNotNullableString":"required-not-nullable","optionalNullableString":null}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNullableString{},
				RequiredNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString{
					Inner: "required-not-nullable",
				},
				OptionalNullableString: &example.ObjectKeysAdditionalPropertiesFalseOptionalNullableString{},
			},
			expectedErr: nil,
		},
		"required nullable null optional nullable null optional not nullable non-null": {
			inputJson: `{"requiredNullableString":null,"requiredNotNullableString":"required-not-nullable","optionalNullableString":null,"optionalNotNullableString":"optional-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNullableString{},
				RequiredNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString{
					Inner: "required-not-nullable",
				},
				OptionalNullableString: &example.ObjectKeysAdditionalPropertiesFalseOptionalNullableString{},
				OptionalNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString{
					Inner: "optional-not-nullable",
				},
			},
			expectedErr: nil,
		},
		"required nullable null optional nullable non-null optional not nullable omitted": {
			inputJson: `{"requiredNullableString":null,"requiredNotNullableString":"required-not-nullable","optionalNullableString":"optional-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNullableString{},
				RequiredNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString{
					Inner: "required-not-nullable",
				},
				OptionalNullableString: &example.ObjectKeysAdditionalPropertiesFalseOptionalNullableString{
					Inner: new("optional-nullable"),
				},
			},
			expectedErr: nil,
		},
		"required nullable null optional nullable non-null optional not nullable non-null": {
			inputJson: `{"requiredNullableString":null,"requiredNotNullableString":"required-not-nullable","optionalNullableString":"optional-nullable","optionalNotNullableString":"optional-not-nullable"}`,
			expectedStruct: example.ObjectKeysAdditionalPropertiesFalse{
				RequiredNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNullableString{},
				RequiredNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString{
					Inner: "required-not-nullable",
				},
				OptionalNullableString: &example.ObjectKeysAdditionalPropertiesFalseOptionalNullableString{
					Inner: new("optional-nullable"),
				},
				OptionalNotNullableString: &example.ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString{
					Inner: "optional-not-nullable",
				},
			},
			expectedErr: nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			decoder := peekjson.NewDecoder(strings.NewReader(tt.inputJson))
			var actualStruct example.ObjectKeysAdditionalPropertiesFalse

			// Act
			err := actualStruct.Decode(decoder)

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

func TestObjectKeysAdditionalPropertiesFalseDecodeRejectsInvalidShapes(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		inputJson   string
		expectedErr error
	}{
		"not object array": {
			inputJson:   `[]`,
			expectedErr: example.NotAnObjectError,
		},
		"not object string": {
			inputJson:   `"not-object"`,
			expectedErr: example.NotAnObjectError,
		},
		"not object null": {
			inputJson:   `null`,
			expectedErr: example.NotAnObjectError,
		},
		"not object number": {
			inputJson:   `123`,
			expectedErr: example.NotAnObjectError,
		},
		"not object bool": {
			inputJson:   `true`,
			expectedErr: example.NotAnObjectError,
		},
		"additional property after required properties": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","extra":"not-allowed"}`,
			expectedErr: example.AdditionalPropertyError,
		},
		"additional property before required properties": {
			inputJson:   `{"extra":"not-allowed","requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable"}`,
			expectedErr: example.AdditionalPropertyError,
		},
		"additional property after optional properties": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","optionalNullableString":"optional-nullable","optionalNotNullableString":"optional-not-nullable","extra":"not-allowed"}`,
			expectedErr: example.AdditionalPropertyError,
		},
		"both required strings missing": {
			inputJson:   `{}`,
			expectedErr: example.MissingRequiredPropertyError,
		},
		"missing required nullable string": {
			inputJson:   `{"requiredNotNullableString":"required-not-nullable"}`,
			expectedErr: example.MissingRequiredPropertyError,
		},
		"missing required not nullable string": {
			inputJson:   `{"requiredNullableString":"required-nullable"}`,
			expectedErr: example.MissingRequiredPropertyError,
		},
		"required not nullable string null": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":null}`,
			expectedErr: example.NullForNotNullableStringError,
		},
		"optional not nullable string null": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","optionalNotNullableString":null}`,
			expectedErr: example.NullForNotNullableStringError,
		},
		"required nullable string number": {
			inputJson:   `{"requiredNullableString":123,"requiredNotNullableString":"required-not-nullable"}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"required nullable string bool": {
			inputJson:   `{"requiredNullableString":true,"requiredNotNullableString":"required-not-nullable"}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"required nullable string object": {
			inputJson:   `{"requiredNullableString":{},"requiredNotNullableString":"required-not-nullable"}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"required nullable string array": {
			inputJson:   `{"requiredNullableString":[],"requiredNotNullableString":"required-not-nullable"}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"required not nullable string number": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":123}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"required not nullable string bool": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":false}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"required not nullable string object": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":{}}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"required not nullable string array": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":[]}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional nullable string number": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","optionalNullableString":123}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional nullable string bool": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","optionalNullableString":false}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional nullable string object": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","optionalNullableString":{}}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional nullable string array": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","optionalNullableString":[]}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional not nullable string number": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","optionalNotNullableString":123}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional not nullable string bool": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","optionalNotNullableString":true}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional not nullable string object": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","optionalNotNullableString":{}}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
		"optional not nullable string array": {
			inputJson:   `{"requiredNullableString":"required-nullable","requiredNotNullableString":"required-not-nullable","optionalNotNullableString":[]}`,
			expectedErr: example.NonStringForStringSchemaError,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			decoder := peekjson.NewDecoder(strings.NewReader(tt.inputJson))
			var actualStruct example.ObjectKeysAdditionalPropertiesFalse

			err := actualStruct.Decode(decoder)

			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
