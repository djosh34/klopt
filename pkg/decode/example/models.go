package example

import (
	"bytes"
	"errors"
	"fmt"

	"encoding/json"
)

var (
	NotAnObjectError              = errors.New("not an object")
	AdditionalPropertyError       = errors.New("additional property")
	MissingRequiredPropertyError  = errors.New("missing required property")
	NullForNotNullableStringError = errors.New("null for not nullable string")
	NonStringForStringSchemaError = errors.New("non-string for string schema")
)

var jsonNull = []byte("null")

type RequiredNullableString string

var _ json.Unmarshaler = new(RequiredNullableString)

func (s *RequiredNullableString) UnmarshalJSON(data []byte) error {
	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}

	*s = RequiredNullableString(value)
	return nil
}

type RequiredNotNullableString string

var _ json.Unmarshaler = new(RequiredNotNullableString)

func (s *RequiredNotNullableString) UnmarshalJSON(data []byte) error {
	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}

	*s = RequiredNotNullableString(value)
	return nil
}

type OptionalNullableString string

var _ json.Unmarshaler = new(OptionalNullableString)

func (s *OptionalNullableString) UnmarshalJSON(data []byte) error {
	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}

	*s = OptionalNullableString(value)
	return nil
}

type OptionalNotNullableString string

var _ json.Unmarshaler = new(OptionalNotNullableString)

func (s *OptionalNotNullableString) UnmarshalJSON(data []byte) error {
	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}

	*s = OptionalNotNullableString(value)
	return nil
}

type ObjectKeysAdditionalPropertiesFalse struct {
	RequiredNullableString    *RequiredNullableString    `json:"requiredNullableString"`
	RequiredNotNullableString RequiredNotNullableString  `json:"requiredNotNullableString"`
	OptionalNullableString    *OptionalNullableString    `json:"optionalNullableString,omitzero"`
	OptionalNotNullableString *OptionalNotNullableString `json:"optionalNotNullableString,omitzero"`
}

var _ json.Unmarshaler = (*ObjectKeysAdditionalPropertiesFalse)(nil)

func (o *ObjectKeysAdditionalPropertiesFalse) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}

	var hasRequiredNullableString bool
	var hasRequiredNotNullableString bool

	for d.More() {
		nameTok, nameErr := d.Token()
		if nameErr != nil {
			return nameErr
		}

		name, ok := nameTok.(string)
		if !ok {
			return NotAnObjectError
		}

		var value json.RawMessage
		err = d.Decode(&value)
		if err != nil {
			return err
		}

		switch name {
		case "requiredNullableString":
			hasRequiredNullableString = true

			err = json.Unmarshal(value, &o.RequiredNullableString)
			if err != nil {
				return err
			}
		case "requiredNotNullableString":
			hasRequiredNotNullableString = true

			if bytes.Equal(value, jsonNull) {
				return NullForNotNullableStringError
			}

			err = json.Unmarshal(value, &o.RequiredNotNullableString)
			if err != nil {
				return err
			}
		case "optionalNullableString":
			err = json.Unmarshal(value, &o.OptionalNullableString)
			if err != nil {
				return err
			}

		case "optionalNotNullableString":

			if bytes.Equal(value, jsonNull) {
				return NullForNotNullableStringError
			}

			err = json.Unmarshal(value, &o.OptionalNotNullableString)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: %s", AdditionalPropertyError, name)
		}
	}

	if !hasRequiredNullableString {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "requiredNullableString")
	}
	if !hasRequiredNotNullableString {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "requiredNotNullableString")
	}

	return nil
}
