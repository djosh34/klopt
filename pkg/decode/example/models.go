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

//var _ Decoder = new(ObjectKeysAdditionalPropertiesFalse)
//var _ Decoder = new(ObjectKeysAdditionalPropertiesFalseRequiredNullableString)
//var _ Decoder = new(ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString)
//var _ Decoder = new(ObjectKeysAdditionalPropertiesFalseOptionalNullableString)
//var _ Decoder = new(ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString)

type ObjectKeysAdditionalPropertiesFalse struct {
	RequiredNullableString    *ObjectKeysAdditionalPropertiesFalseRequiredNullableString    `json:"requiredNullableString"`
	RequiredNotNullableString *ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString `json:"requiredNotNullableString"`
	OptionalNullableString    *ObjectKeysAdditionalPropertiesFalseOptionalNullableString    `json:"optionalNullableString,omitempty"`
	OptionalNotNullableString *ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString `json:"optionalNotNullableString,omitempty"`
}
type ObjectKeysAdditionalPropertiesFalseRequiredNullableString struct{ Inner *string }
type ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString struct{ Inner string }
type ObjectKeysAdditionalPropertiesFalseOptionalNullableString struct{ Inner *string }
type ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString struct{ Inner string }

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
			requiredNullableStringErr := ObjectKeysAdditionalPropertiesFalseRequiredNullableStringDecode(requiredNullableString, decoder)
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

var ObjectKeysAdditionalPropertiesFalseRequiredNullableStringDecode = func(o *ObjectKeysAdditionalPropertiesFalseRequiredNullableString, decoder *peekjson.Decoder) error {
	inner, null, err := decodeString(decoder)
	if err != nil {
		return err
	}

	if !null {
		o.Inner = new(inner)
	}

	return nil

}

//func (o *ObjectKeysAdditionalPropertiesFalseRequiredNullableString) Decode(decoder *peekjson.Decoder) error {
//}

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
