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

type ObjectKeysAdditionalPropertiesFalse struct {
	RequiredNullableString    *string `json:"requiredNullableString"`
	RequiredNotNullableString string  `json:"requiredNotNullableString"`
	OptionalNullableString    *string `json:"optionalNullableString,omitzero"`
	OptionalNotNullableString *string `json:"optionalNotNullableString,omitzero"`
}

var _ json.Unmarshaler = (*ObjectKeysAdditionalPropertiesFalse)(nil)

var jsonNull = []byte("null")

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
