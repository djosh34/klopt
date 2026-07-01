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
	NullForNotNullableBoolError   = errors.New("null for not nullable bool")
	NonBoolForBoolSchemaError     = errors.New("non-bool for bool schema")
	NullForNotNullableNumberError = errors.New("null for not nullable number")
	NonNumberForNumberSchemaError = errors.New("non-number for number schema")
	NullForNotNullableStringError = errors.New("null for not nullable string")
	NonStringForStringSchemaError = errors.New("non-string for string schema")
)

var jsonNull = []byte("null")

type OptionalArrayNullable struct {
	Value []OptionalArrayNullableItem
}

var _ json.Unmarshaler = new(OptionalArrayNullable)

func (a *OptionalArrayNullable) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if bytes.Equal(data, jsonNull) {
		a.Value = nil
		return nil
	}

	var value []OptionalArrayNullableItem
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	a.Value = value
	return nil
}

type OptionalArrayNullableItem string

var _ json.Unmarshaler = new(OptionalArrayNullableItem)

func (s *OptionalArrayNullableItem) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = OptionalArrayNullableItem(value)
	return nil
}

type ObjectKeysAdditionalPropertiesFalse struct {
	OptionalNotNullableString *ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString `json:"optionalNotNullableString,omitzero"`
	OptionalNullableString    *ObjectKeysAdditionalPropertiesFalseOptionalNullableString    `json:"optionalNullableString,omitzero"`
	RequiredNotNullableString ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString  `json:"requiredNotNullableString"`
	RequiredNullableString    ObjectKeysAdditionalPropertiesFalseRequiredNullableString     `json:"requiredNullableString"`
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

		name := nameTok.(string)

		var value json.RawMessage
		err = d.Decode(&value)
		if err != nil {
			return err
		}

		switch name {
		case "optionalNotNullableString":

			var optionalNotNullableString ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString
			err = json.Unmarshal(value, &optionalNotNullableString)
			if err != nil {
				return err
			}
			o.OptionalNotNullableString = &optionalNotNullableString
		case "optionalNullableString":

			var optionalNullableString ObjectKeysAdditionalPropertiesFalseOptionalNullableString
			err = json.Unmarshal(value, &optionalNullableString)
			if err != nil {
				return err
			}
			o.OptionalNullableString = &optionalNullableString
		case "requiredNotNullableString":
			hasRequiredNotNullableString = true

			var requiredNotNullableString ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString
			err = json.Unmarshal(value, &requiredNotNullableString)
			if err != nil {
				return err
			}
			o.RequiredNotNullableString = requiredNotNullableString
		case "requiredNullableString":
			hasRequiredNullableString = true

			var requiredNullableString ObjectKeysAdditionalPropertiesFalseRequiredNullableString
			err = json.Unmarshal(value, &requiredNullableString)
			if err != nil {
				return err
			}
			o.RequiredNullableString = requiredNullableString
		default:
			return fmt.Errorf("%w: %s", AdditionalPropertyError, name)
		}
	}
	if _, err := d.Token(); err != nil {
		return err
	}
	if len(bytes.TrimSpace(data[d.InputOffset():])) != 0 {
		return NotAnObjectError
	}
	if !hasRequiredNotNullableString {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "requiredNotNullableString")
	}
	if !hasRequiredNullableString {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "requiredNullableString")
	}

	return nil
}

type ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString string

var _ json.Unmarshaler = new(ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString)

func (s *ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString(value)
	return nil
}

type ObjectKeysAdditionalPropertiesFalseOptionalNullableString struct {
	Value *string
}

var _ json.Unmarshaler = new(ObjectKeysAdditionalPropertiesFalseOptionalNullableString)

func (s *ObjectKeysAdditionalPropertiesFalseOptionalNullableString) UnmarshalJSON(data []byte) error {
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

type ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString string

var _ json.Unmarshaler = new(ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString)

func (s *ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString(value)
	return nil
}

type ObjectKeysAdditionalPropertiesFalseRequiredNullableString struct {
	Value *string
}

var _ json.Unmarshaler = new(ObjectKeysAdditionalPropertiesFalseRequiredNullableString)

func (s *ObjectKeysAdditionalPropertiesFalseRequiredNullableString) UnmarshalJSON(data []byte) error {
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

type ArrayNullable struct {
	Value []ArrayNullableItem
}

var _ json.Unmarshaler = new(ArrayNullable)

func (a *ArrayNullable) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		a.Value = nil
		return nil
	}

	var value []ArrayNullableItem
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	a.Value = value
	return nil
}

type ArrayNullableItem string

var _ json.Unmarshaler = new(ArrayNullableItem)

func (s *ArrayNullableItem) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = ArrayNullableItem(value)
	return nil
}

type ArrayNotNullable []ArrayNotNullableItem

var _ json.Unmarshaler = new(ArrayNotNullable)

func (a *ArrayNotNullable) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return fmt.Errorf("null for not nullable array")
	}

	var value []ArrayNotNullableItem
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	*a = ArrayNotNullable(value)
	return nil
}

type ArrayNotNullableItem string

var _ json.Unmarshaler = new(ArrayNotNullableItem)

func (s *ArrayNotNullableItem) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = ArrayNotNullableItem(value)
	return nil
}

type AllOfObject struct {
	AllOfObjectAllOf1
	AllOfObjectAllOf2
	AllOfObjectAllOf3
}

var _ json.Unmarshaler = (*AllOfObject)(nil)

func (a *AllOfObject) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.AllOfObjectAllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.AllOfObjectAllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.AllOfObjectAllOf3.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type AllOfObjectAllOf1 struct {
	First AllOfObjectAllOf1First `json:"first"`
}

var _ json.Unmarshaler = (*AllOfObjectAllOf1)(nil)

func (o *AllOfObjectAllOf1) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFirst bool

	for d.More() {
		nameTok, nameErr := d.Token()
		if nameErr != nil {
			return nameErr
		}

		name := nameTok.(string)

		var value json.RawMessage
		err = d.Decode(&value)
		if err != nil {
			return err
		}

		switch name {
		case "first":
			hasFirst = true

			var first AllOfObjectAllOf1First
			err = json.Unmarshal(value, &first)
			if err != nil {
				return err
			}
			o.First = first
		default:
			continue
		}
	}
	if _, err := d.Token(); err != nil {
		return err
	}
	if len(bytes.TrimSpace(data[d.InputOffset():])) != 0 {
		return NotAnObjectError
	}
	if !hasFirst {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "first")
	}

	return nil
}

type AllOfObjectAllOf1First string

var _ json.Unmarshaler = new(AllOfObjectAllOf1First)

func (s *AllOfObjectAllOf1First) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = AllOfObjectAllOf1First(value)
	return nil
}

type AllOfObjectAllOf2 struct {
	Second AllOfObjectAllOf2Second `json:"second"`
}

var _ json.Unmarshaler = (*AllOfObjectAllOf2)(nil)

func (o *AllOfObjectAllOf2) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSecond bool

	for d.More() {
		nameTok, nameErr := d.Token()
		if nameErr != nil {
			return nameErr
		}

		name := nameTok.(string)

		var value json.RawMessage
		err = d.Decode(&value)
		if err != nil {
			return err
		}

		switch name {
		case "second":
			hasSecond = true

			var second AllOfObjectAllOf2Second
			err = json.Unmarshal(value, &second)
			if err != nil {
				return err
			}
			o.Second = second
		default:
			continue
		}
	}
	if _, err := d.Token(); err != nil {
		return err
	}
	if len(bytes.TrimSpace(data[d.InputOffset():])) != 0 {
		return NotAnObjectError
	}
	if !hasSecond {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "second")
	}

	return nil
}

type AllOfObjectAllOf2Second bool

var _ json.Unmarshaler = new(AllOfObjectAllOf2Second)

func (b *AllOfObjectAllOf2Second) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = AllOfObjectAllOf2Second(value)
	return nil
}

type AllOfObjectAllOf3 struct {
	Last AllOfObjectAllOf3Last `json:"last"`
}

var _ json.Unmarshaler = (*AllOfObjectAllOf3)(nil)

func (o *AllOfObjectAllOf3) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasLast bool

	for d.More() {
		nameTok, nameErr := d.Token()
		if nameErr != nil {
			return nameErr
		}

		name := nameTok.(string)

		var value json.RawMessage
		err = d.Decode(&value)
		if err != nil {
			return err
		}

		switch name {
		case "last":
			hasLast = true

			var last AllOfObjectAllOf3Last
			err = json.Unmarshal(value, &last)
			if err != nil {
				return err
			}
			o.Last = last
		default:
			continue
		}
	}
	if _, err := d.Token(); err != nil {
		return err
	}
	if len(bytes.TrimSpace(data[d.InputOffset():])) != 0 {
		return NotAnObjectError
	}
	if !hasLast {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "last")
	}

	return nil
}

type AllOfObjectAllOf3Last json.Number

var _ json.Unmarshaler = new(AllOfObjectAllOf3Last)

func (n *AllOfObjectAllOf3Last) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, jsonNull) {
		return NullForNotNullableNumberError
	}

	if len(trimmed) == 0 || trimmed[0] == '"' {
		return NonNumberForNumberSchemaError
	}

	var value json.Number
	err := json.Unmarshal(trimmed, &value)
	if err != nil {
		return NonNumberForNumberSchemaError
	}
	*n = AllOfObjectAllOf3Last(value)
	return nil
}
