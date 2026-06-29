package decode_tests

import (
	"encoding/json"
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
			backToJson, err := json.MarshalIndent(actualStruct, "", "  ")
			require.NoError(t, err)

			t.Logf("Json input:\n\n%v\n\nJson Back:\n\n%v\n\n", tt.inputJson, string(backToJson))

			if tt.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tt.expectedErr)
			}

			require.Equal(t, tt.expectedStruct, actualStruct)
		})
	}
}
