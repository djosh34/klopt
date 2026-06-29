package example

import (
	"decode_and_validate_generator/pkg/peekjson"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	NotAnObjectError              = errors.New("not an object")
	AdditionalPropertyError       = errors.New("additional property")
	MissingRequiredPropertyError  = errors.New("missing required property")
	NullForNotNullableStringError = errors.New("null for not nullable string")
	NonStringForStringSchemaError = errors.New("non-string for string schema")
)

type ObjectKeysAdditionalPropertiesFalse struct {
	RequiredNullableString    *ObjectKeysAdditionalPropertiesFalseRequiredNullableString    `json:"requiredNullableString"`
	RequiredNotNullableString *ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString `json:"requiredNotNullableString"`
	OptionalNullableString    *ObjectKeysAdditionalPropertiesFalseOptionalNullableString    `json:"optionalNullableString,omitempty"`
	OptionalNotNullableString *ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString `json:"optionalNotNullableString,omitempty"`
}

var _ json.Marshaler = new(ObjectKeysAdditionalPropertiesFalseRequiredNullableString)
var _ json.Marshaler = new(ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString)
var _ json.Marshaler = new(ObjectKeysAdditionalPropertiesFalseOptionalNullableString)
var _ json.Marshaler = new(ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString)

type ObjectKeysAdditionalPropertiesFalseRequiredNullableString struct{ Inner *string }

func (o *ObjectKeysAdditionalPropertiesFalseRequiredNullableString) MarshalJSON() ([]byte, error) {
	if o.Inner == nil {
		return []byte("null"), nil
	}

	return []byte(fmt.Sprintf("\"%v\"", *o.Inner)), nil
}

type ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString struct{ Inner string }

func (o *ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%v\"", o.Inner)), nil
}

type ObjectKeysAdditionalPropertiesFalseOptionalNullableString struct{ Inner *string }

func (o *ObjectKeysAdditionalPropertiesFalseOptionalNullableString) MarshalJSON() ([]byte, error) {
	if o == nil {
		return nil, nil
	}

	if o.Inner == nil {
		return []byte("null"), nil
	}

	return []byte(fmt.Sprintf("\"%v\"", *o.Inner)), nil
}

type ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString struct{ Inner string }

func (o *ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString) MarshalJSON() ([]byte, error) {
	if o == nil {
		return nil, nil
	}

	return []byte(fmt.Sprintf("\"%v\"", o.Inner)), nil
}

func (o *ObjectKeysAdditionalPropertiesFalse) Decode(decoder *peekjson.Decoder) error {
	nextToken, err := decoder.Token()
	if err != nil {
		return err
	}

	if nextToken != json.Delim('{') {
		return NotAnObjectError
	}

	requiredProperties := map[string]struct{}{
		"requiredNullableString":    {},
		"requiredNotNullableString": {},
	}

	for decoder.More() {
		nextKeyToken, nextKeyErr := decoder.Token()
		if nextKeyErr != nil {
			return nextKeyErr
		}

		nextKey, ok := nextKeyToken.(string)
		if !ok {
			return NotAnObjectError
		}

		if _, required := requiredProperties[nextKey]; required {
			delete(requiredProperties, nextKey)
		}

		switch nextKey {
		case "requiredNullableString":
			requiredNullableString := new(ObjectKeysAdditionalPropertiesFalseRequiredNullableString)
			requiredNullableStringErr := requiredNullableString.Decode(decoder)
			if requiredNullableStringErr != nil {
				return requiredNullableStringErr
			}

			o.RequiredNullableString = requiredNullableString
		case "requiredNotNullableString":
			requiredNotNullableString := new(ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString)
			requiredNotNullableStringErr := requiredNotNullableString.Decode(decoder)
			if requiredNotNullableStringErr != nil {
				return requiredNotNullableStringErr
			}

			o.RequiredNotNullableString = requiredNotNullableString
		case "optionalNullableString":
			optionalNullableString := new(ObjectKeysAdditionalPropertiesFalseOptionalNullableString)
			optionalNullableStringErr := optionalNullableString.Decode(decoder)
			if optionalNullableStringErr != nil {
				return optionalNullableStringErr
			}

			o.OptionalNullableString = optionalNullableString
		case "optionalNotNullableString":
			optionalNotNullableString := new(ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString)
			optionalNotNullableStringErr := optionalNotNullableString.Decode(decoder)
			if optionalNotNullableStringErr != nil {
				return optionalNotNullableStringErr
			}

			o.OptionalNotNullableString = optionalNotNullableString
		default:
			return fmt.Errorf("%w: %v", AdditionalPropertyError, nextKey)
		}
	}

	nextToken, err = decoder.Token()
	if err != nil {
		return err
	}

	if nextToken != json.Delim('}') {
		return NotAnObjectError
	}

	for missingRequiredProperty := range requiredProperties {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, missingRequiredProperty)
	}

	return nil
}

func (o *ObjectKeysAdditionalPropertiesFalseRequiredNullableString) Decode(decoder *peekjson.Decoder) error {
	inner, null, err := decodeString(decoder)
	if err != nil {
		return err
	}

	if !null {
		o.Inner = new(inner)
	}

	return nil
}

func (o *ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString) Decode(decoder *peekjson.Decoder) error {
	inner, null, err := decodeString(decoder)
	if err != nil {
		return err
	}

	if null {
		return NullForNotNullableStringError
	}

	o.Inner = inner

	return nil
}

func (o *ObjectKeysAdditionalPropertiesFalseOptionalNullableString) Decode(decoder *peekjson.Decoder) error {
	inner, null, err := decodeString(decoder)
	if err != nil {
		return err
	}

	if null {
		o.Inner = nil
	} else {
		o.Inner = new(inner)
	}

	return nil
}

func (o *ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString) Decode(decoder *peekjson.Decoder) error {
	inner, null, err := decodeString(decoder)
	if err != nil {
		return err
	}

	if null {
		return NullForNotNullableStringError
	}

	o.Inner = inner

	return nil
}

func decodeString(decoder *peekjson.Decoder) (string, bool, error) {
	nextToken, err := decoder.Token()
	if err != nil {
		return "", false, err
	}

	if nextToken == nil {
		return "", true, nil
	}

	inner, ok := nextToken.(string)
	if !ok {
		return "", false, fmt.Errorf("%w: %v", NonStringForStringSchemaError, nextToken)
	}

	return inner, false, nil
}
