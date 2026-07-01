package example

import (
	"bytes"
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

var jsonNull = []byte("null")

type ObjectKeysAdditionalPropertiesFalse struct {
	OptionalNotNullableString *OptionalNotNullableString `json:"optionalNotNullableString,omitzero"`
	OptionalNullableString    *OptionalNullableString    `json:"optionalNullableString,omitzero"`
	RequiredNotNullableString RequiredNotNullableString  `json:"requiredNotNullableString"`
	RequiredNullableString    RequiredNullableString     `json:"requiredNullableString"`
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
	var hasRequiredNotNullableString bool
	var hasRequiredNullableString bool

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
		case "optionalNotNullableString":

			var optionalNotNullableString OptionalNotNullableString
			err = json.Unmarshal(value, &optionalNotNullableString)
			if err != nil {
				return err
			}
			o.OptionalNotNullableString = &optionalNotNullableString
		case "optionalNullableString":

			var optionalNullableString OptionalNullableString
			err = json.Unmarshal(value, &optionalNullableString)
			if err != nil {
				return err
			}
			o.OptionalNullableString = &optionalNullableString
		case "requiredNotNullableString":
			hasRequiredNotNullableString = true

			var requiredNotNullableString RequiredNotNullableString
			err = json.Unmarshal(value, &requiredNotNullableString)
			if err != nil {
				return err
			}
			o.RequiredNotNullableString = requiredNotNullableString
		case "requiredNullableString":
			hasRequiredNullableString = true

			var requiredNullableString RequiredNullableString
			err = json.Unmarshal(value, &requiredNullableString)
			if err != nil {
				return err
			}
			o.RequiredNullableString = requiredNullableString
		default:
			return fmt.Errorf("%w: %s", AdditionalPropertyError, name)
		}
	}
	if !hasRequiredNotNullableString {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "requiredNotNullableString")
	}
	if !hasRequiredNullableString {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "requiredNullableString")
	}

	return nil
}

type OptionalNotNullableString string

var _ json.Unmarshaler = new(OptionalNotNullableString)

func (s *OptionalNotNullableString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = OptionalNotNullableString(value)
	return nil
}

type OptionalNullableString struct {
	Value *string
}

var _ json.Unmarshaler = new(OptionalNullableString)

func (s *OptionalNullableString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		s.Value = nil
		return nil
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	s.Value = new(value)
	return nil
}

type RequiredNotNullableString string

var _ json.Unmarshaler = new(RequiredNotNullableString)

func (s *RequiredNotNullableString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RequiredNotNullableString(value)
	return nil
}

type RequiredNullableString struct {
	Value *string
}

var _ json.Unmarshaler = new(RequiredNullableString)

func (s *RequiredNullableString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		s.Value = nil
		return nil
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	s.Value = new(value)
	return nil
}
