package example

import (
	"decode_and_validate_generator/pkg/peekjson"
	"errors"
)

var (
	NotAnObjectError = errors.New("not an object")
)

var _ Decoder = new(ObjectKeysAdditionalPropertiesFalse)
var _ Decoder = new(ObjectKeysAdditionalPropertiesFalseRequiredNullableString)
var _ Decoder = new(ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString)
var _ Decoder = new(ObjectKeysAdditionalPropertiesFalseOptionalNullableString)
var _ Decoder = new(ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString)

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

func (o *ObjectKeysAdditionalPropertiesFalse) Decode(decoder peekjson.Decoder) error {
	nextToken, err := decoder.Token()
	if err != nil {
		return err
	}

	if nextToken != '{' {
		return NotAnObjectError
	}

	nextKey, nextKeyErr := decoder.Token()
	if nextKeyErr != nil {
		return nextKeyErr
	}

	switch nextKey {
	case "requiredNullableString":
		var requiredNullableString *ObjectKeysAdditionalPropertiesFalseRequiredNullableString
		requiredNullableStringErr := requiredNullableString.Decode(decoder)
		if requiredNullableStringErr != nil {
			return requiredNullableStringErr
		}
		// TODO the rest

	}

	return nil
}

func (o *ObjectKeysAdditionalPropertiesFalseRequiredNullableString) Decode(decoder peekjson.Decoder) error {
	//TODO implement me
	panic("implement me")
}

func (o *ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString) Decode(decoder peekjson.Decoder) error {
	//TODO implement me
	panic("implement me")
}

func (o *ObjectKeysAdditionalPropertiesFalseOptionalNullableString) Decode(decoder peekjson.Decoder) error {
	//TODO implement me
	panic("implement me")
}

func (o *ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString) Decode(decoder peekjson.Decoder) error {
	//TODO implement me
	panic("implement me")
}
