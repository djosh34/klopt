package example

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
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

type StringNoFormatNullable struct {
	Value *string
}

var _ json.Unmarshaler = new(StringNoFormatNullable)

func (s *StringNoFormatNullable) UnmarshalJSON(data []byte) error {
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

type StringNoFormatNotNullable string

var _ json.Unmarshaler = new(StringNoFormatNotNullable)

func (s *StringNoFormatNotNullable) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = StringNoFormatNotNullable(value)
	return nil
}

type RefStressObjectPut struct {
	RefStressObjectPutAllOf1
	RefStressObjectPutAllOf2
	RefStressObjectPutAllOf3
}

var _ json.Unmarshaler = (*RefStressObjectPut)(nil)

func (a *RefStressObjectPut) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectPutAllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectPutAllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectPutAllOf3.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectPutAllOf1 struct {
	RefStressObjectPutAllOf1AllOf1
	RefStressObjectPutAllOf1AllOf2
	RefStressObjectPutAllOf1AllOf3
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1)(nil)

func (a *RefStressObjectPutAllOf1) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectPutAllOf1AllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectPutAllOf1AllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectPutAllOf1AllOf3.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectPutAllOf1AllOf1 struct {
	FinalCode      RefStressObjectPutAllOf1AllOf1FinalCode       `json:"finalCode"`
	Nested         *RefStressObjectPutAllOf1AllOf1Nested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectPutAllOf1AllOf1OptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectPutAllOf1AllOf1SharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf1)(nil)

func (o *RefStressObjectPutAllOf1AllOf1) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectPutAllOf1AllOf1FinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectPutAllOf1AllOf1Nested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectPutAllOf1AllOf1OptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectPutAllOf1AllOf1SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf1FinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf1FinalCode)

func (s *RefStressObjectPutAllOf1AllOf1FinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf1FinalCode(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf1Nested struct {
	Leaf     *RefStressObjectPutAllOf1AllOf1NestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectPutAllOf1AllOf1NestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf1Nested)(nil)

func (o *RefStressObjectPutAllOf1AllOf1Nested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectPutAllOf1AllOf1NestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf1AllOf1NestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf1NestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf1NestedLeaf)

func (s *RefStressObjectPutAllOf1AllOf1NestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf1NestedLeaf(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf1NestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf1NestedSameName)

func (s *RefStressObjectPutAllOf1AllOf1NestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf1AllOf1OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf1OptionalShared)

func (s *RefStressObjectPutAllOf1AllOf1OptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf1AllOf1SharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf1SharedName)

func (s *RefStressObjectPutAllOf1AllOf1SharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf1SharedName(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf2 struct {
	RefStressObjectPutAllOf1AllOf2AllOf1
	RefStressObjectPutAllOf1AllOf2AllOf2
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2)(nil)

func (a *RefStressObjectPutAllOf1AllOf2) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectPutAllOf1AllOf2AllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectPutAllOf1AllOf2AllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectPutAllOf1AllOf2AllOf1 struct {
	RefStressObjectPutAllOf1AllOf2AllOf1AllOf1
	RefStressObjectPutAllOf1AllOf2AllOf1AllOf2
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf1)(nil)

func (a *RefStressObjectPutAllOf1AllOf2AllOf1) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectPutAllOf1AllOf2AllOf1AllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectPutAllOf1AllOf2AllOf1AllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectPutAllOf1AllOf2AllOf1AllOf1 struct {
	FinalCode      RefStressObjectPutAllOf1AllOf2AllOf1AllOf1FinalCode       `json:"finalCode"`
	Nested         *RefStressObjectPutAllOf1AllOf2AllOf1AllOf1Nested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectPutAllOf1AllOf2AllOf1AllOf1OptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectPutAllOf1AllOf2AllOf1AllOf1SharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf1AllOf1)(nil)

func (o *RefStressObjectPutAllOf1AllOf2AllOf1AllOf1) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectPutAllOf1AllOf2AllOf1AllOf1FinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectPutAllOf1AllOf2AllOf1AllOf1Nested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectPutAllOf1AllOf2AllOf1AllOf1OptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectPutAllOf1AllOf2AllOf1AllOf1SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf1AllOf1FinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf1AllOf1FinalCode)

func (s *RefStressObjectPutAllOf1AllOf2AllOf1AllOf1FinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf2AllOf1AllOf1FinalCode(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf1AllOf1Nested struct {
	Leaf     *RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf1AllOf1Nested)(nil)

func (o *RefStressObjectPutAllOf1AllOf2AllOf1AllOf1Nested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedLeaf)

func (s *RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedLeaf(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedSameName)

func (s *RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf1AllOf2AllOf1AllOf1OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf1AllOf1OptionalShared)

func (s *RefStressObjectPutAllOf1AllOf2AllOf1AllOf1OptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf1AllOf2AllOf1AllOf1SharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf1AllOf1SharedName)

func (s *RefStressObjectPutAllOf1AllOf2AllOf1AllOf1SharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf2AllOf1AllOf1SharedName(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf1AllOf2 struct {
	OptionalCode *RefStressObjectPutAllOf1AllOf2AllOf1AllOf2OptionalCode `json:"optionalCode,omitzero"`
	SharedName   RefStressObjectPutAllOf1AllOf2AllOf1AllOf2SharedName    `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf1AllOf2)(nil)

func (o *RefStressObjectPutAllOf1AllOf2AllOf1AllOf2) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSharedName bool

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
		case "optionalCode":

			var optionalCode RefStressObjectPutAllOf1AllOf2AllOf1AllOf2OptionalCode
			err = json.Unmarshal(value, &optionalCode)
			if err != nil {
				return err
			}
			o.OptionalCode = &optionalCode
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectPutAllOf1AllOf2AllOf1AllOf2SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf1AllOf2OptionalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf1AllOf2OptionalCode)

func (s *RefStressObjectPutAllOf1AllOf2AllOf1AllOf2OptionalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf2AllOf1AllOf2OptionalCode(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf1AllOf2SharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf1AllOf2SharedName)

func (s *RefStressObjectPutAllOf1AllOf2AllOf1AllOf2SharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf2AllOf1AllOf2SharedName(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf2 struct {
	MiddleFlag RefStressObjectPutAllOf1AllOf2AllOf2MiddleFlag `json:"middleFlag"`
	Nested     *RefStressObjectPutAllOf1AllOf2AllOf2Nested    `json:"nested,omitzero"`
	SharedName RefStressObjectPutAllOf1AllOf2AllOf2SharedName `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf2)(nil)

func (o *RefStressObjectPutAllOf1AllOf2AllOf2) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasMiddleFlag bool
	var hasSharedName bool

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
		case "middleFlag":
			hasMiddleFlag = true

			var middleFlag RefStressObjectPutAllOf1AllOf2AllOf2MiddleFlag
			err = json.Unmarshal(value, &middleFlag)
			if err != nil {
				return err
			}
			o.MiddleFlag = middleFlag
		case "nested":

			var nested RefStressObjectPutAllOf1AllOf2AllOf2Nested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectPutAllOf1AllOf2AllOf2SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasMiddleFlag {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "middleFlag")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf2MiddleFlag bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2MiddleFlag)

func (b *RefStressObjectPutAllOf1AllOf2AllOf2MiddleFlag) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectPutAllOf1AllOf2AllOf2MiddleFlag(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf2Nested struct {
	RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1
	RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2
	RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf2Nested)(nil)

func (a *RefStressObjectPutAllOf1AllOf2AllOf2Nested) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1 struct {
	Leaf     *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1Leaf    `json:"leaf,omitzero"`
	SameName RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1SameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1)(nil)

func (o *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1Leaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1Leaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1Leaf)

func (s *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1Leaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1Leaf(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1SameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1SameName)

func (s *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1SameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2 struct {
	Leaf     *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2Leaf    `json:"leaf,omitzero"`
	SameName RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2SameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2)(nil)

func (o *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2Leaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2Leaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2Leaf)

func (s *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2Leaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2Leaf(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2SameName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2SameName)

func (s *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2SameName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2SameName(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3 struct {
	SameName RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SameName `json:"sameName"`
	Sealed   RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3Sealed   `json:"sealed"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3)(nil)

func (o *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool
	var hasSealed bool

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
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
		case "sealed":
			hasSealed = true

			var sealed RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3Sealed
			err = json.Unmarshal(value, &sealed)
			if err != nil {
				return err
			}
			o.Sealed = sealed
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}
	if !hasSealed {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sealed")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SameName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SameName)

func (s *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SameName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SameName(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3Sealed struct {
	Locked RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SealedLocked `json:"locked"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3Sealed)(nil)

func (o *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3Sealed) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasLocked bool

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
		case "locked":
			hasLocked = true

			var locked RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SealedLocked
			err = json.Unmarshal(value, &locked)
			if err != nil {
				return err
			}
			o.Locked = locked
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
	if !hasLocked {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "locked")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SealedLocked bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SealedLocked)

func (b *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SealedLocked) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SealedLocked(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf2AllOf2SharedName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2SharedName)

func (s *RefStressObjectPutAllOf1AllOf2AllOf2SharedName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf1AllOf3 struct {
	Final            RefStressObjectPutAllOf1AllOf3Final            `json:"final"`
	Nested           RefStressObjectPutAllOf1AllOf3Nested           `json:"nested"`
	NullableRequired RefStressObjectPutAllOf1AllOf3NullableRequired `json:"nullableRequired"`
	OptionalShared   *RefStressObjectPutAllOf1AllOf3OptionalShared  `json:"optionalShared,omitzero"`
	SharedName       *RefStressObjectPutAllOf1AllOf3SharedName      `json:"sharedName,omitzero"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3)(nil)

func (o *RefStressObjectPutAllOf1AllOf3) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinal bool
	var hasNested bool
	var hasNullableRequired bool

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
		case "final":
			hasFinal = true

			var final RefStressObjectPutAllOf1AllOf3Final
			err = json.Unmarshal(value, &final)
			if err != nil {
				return err
			}
			o.Final = final
		case "nested":
			hasNested = true

			var nested RefStressObjectPutAllOf1AllOf3Nested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = nested
		case "nullableRequired":
			hasNullableRequired = true

			var nullableRequired RefStressObjectPutAllOf1AllOf3NullableRequired
			err = json.Unmarshal(value, &nullableRequired)
			if err != nil {
				return err
			}
			o.NullableRequired = nullableRequired
		case "optionalShared":

			var optionalShared RefStressObjectPutAllOf1AllOf3OptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":

			var sharedName RefStressObjectPutAllOf1AllOf3SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = &sharedName
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
	if !hasFinal {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "final")
	}
	if !hasNested {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "nested")
	}
	if !hasNullableRequired {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "nullableRequired")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf3Final struct {
	FinalCode      RefStressObjectPutAllOf1AllOf3FinalFinalCode       `json:"finalCode"`
	Nested         *RefStressObjectPutAllOf1AllOf3FinalNested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectPutAllOf1AllOf3FinalOptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectPutAllOf1AllOf3FinalSharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3Final)(nil)

func (o *RefStressObjectPutAllOf1AllOf3Final) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectPutAllOf1AllOf3FinalFinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectPutAllOf1AllOf3FinalNested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectPutAllOf1AllOf3FinalOptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectPutAllOf1AllOf3FinalSharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf3FinalFinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3FinalFinalCode)

func (s *RefStressObjectPutAllOf1AllOf3FinalFinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf3FinalFinalCode(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf3FinalNested struct {
	Leaf     *RefStressObjectPutAllOf1AllOf3FinalNestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectPutAllOf1AllOf3FinalNestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3FinalNested)(nil)

func (o *RefStressObjectPutAllOf1AllOf3FinalNested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectPutAllOf1AllOf3FinalNestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf1AllOf3FinalNestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf3FinalNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3FinalNestedLeaf)

func (s *RefStressObjectPutAllOf1AllOf3FinalNestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf3FinalNestedLeaf(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf3FinalNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3FinalNestedSameName)

func (s *RefStressObjectPutAllOf1AllOf3FinalNestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf1AllOf3FinalOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3FinalOptionalShared)

func (s *RefStressObjectPutAllOf1AllOf3FinalOptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf1AllOf3FinalSharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3FinalSharedName)

func (s *RefStressObjectPutAllOf1AllOf3FinalSharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf3FinalSharedName(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf3Nested struct {
	RefStressObjectPutAllOf1AllOf3NestedAllOf1
	RefStressObjectPutAllOf1AllOf3NestedAllOf2
	RefStressObjectPutAllOf1AllOf3NestedAllOf3
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3Nested)(nil)

func (a *RefStressObjectPutAllOf1AllOf3Nested) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectPutAllOf1AllOf3NestedAllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectPutAllOf1AllOf3NestedAllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectPutAllOf1AllOf3NestedAllOf3.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectPutAllOf1AllOf3NestedAllOf1 struct {
	Leaf     *RefStressObjectPutAllOf1AllOf3NestedAllOf1Leaf    `json:"leaf,omitzero"`
	SameName RefStressObjectPutAllOf1AllOf3NestedAllOf1SameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3NestedAllOf1)(nil)

func (o *RefStressObjectPutAllOf1AllOf3NestedAllOf1) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectPutAllOf1AllOf3NestedAllOf1Leaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf1AllOf3NestedAllOf1SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf3NestedAllOf1Leaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3NestedAllOf1Leaf)

func (s *RefStressObjectPutAllOf1AllOf3NestedAllOf1Leaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf3NestedAllOf1Leaf(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf3NestedAllOf1SameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3NestedAllOf1SameName)

func (s *RefStressObjectPutAllOf1AllOf3NestedAllOf1SameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf1AllOf3NestedAllOf2 struct {
	Leaf     *RefStressObjectPutAllOf1AllOf3NestedAllOf2Leaf    `json:"leaf,omitzero"`
	SameName RefStressObjectPutAllOf1AllOf3NestedAllOf2SameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3NestedAllOf2)(nil)

func (o *RefStressObjectPutAllOf1AllOf3NestedAllOf2) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectPutAllOf1AllOf3NestedAllOf2Leaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf1AllOf3NestedAllOf2SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf3NestedAllOf2Leaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3NestedAllOf2Leaf)

func (s *RefStressObjectPutAllOf1AllOf3NestedAllOf2Leaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf3NestedAllOf2Leaf(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf3NestedAllOf2SameName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3NestedAllOf2SameName)

func (s *RefStressObjectPutAllOf1AllOf3NestedAllOf2SameName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf3NestedAllOf2SameName(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf3NestedAllOf3 struct {
	SameName RefStressObjectPutAllOf1AllOf3NestedAllOf3SameName `json:"sameName"`
	Sealed   RefStressObjectPutAllOf1AllOf3NestedAllOf3Sealed   `json:"sealed"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3NestedAllOf3)(nil)

func (o *RefStressObjectPutAllOf1AllOf3NestedAllOf3) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool
	var hasSealed bool

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
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf1AllOf3NestedAllOf3SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
		case "sealed":
			hasSealed = true

			var sealed RefStressObjectPutAllOf1AllOf3NestedAllOf3Sealed
			err = json.Unmarshal(value, &sealed)
			if err != nil {
				return err
			}
			o.Sealed = sealed
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}
	if !hasSealed {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sealed")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf3NestedAllOf3SameName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3NestedAllOf3SameName)

func (s *RefStressObjectPutAllOf1AllOf3NestedAllOf3SameName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf1AllOf3NestedAllOf3SameName(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf3NestedAllOf3Sealed struct {
	Locked RefStressObjectPutAllOf1AllOf3NestedAllOf3SealedLocked `json:"locked"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3NestedAllOf3Sealed)(nil)

func (o *RefStressObjectPutAllOf1AllOf3NestedAllOf3Sealed) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasLocked bool

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
		case "locked":
			hasLocked = true

			var locked RefStressObjectPutAllOf1AllOf3NestedAllOf3SealedLocked
			err = json.Unmarshal(value, &locked)
			if err != nil {
				return err
			}
			o.Locked = locked
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
	if !hasLocked {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "locked")
	}

	return nil
}

type RefStressObjectPutAllOf1AllOf3NestedAllOf3SealedLocked bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3NestedAllOf3SealedLocked)

func (b *RefStressObjectPutAllOf1AllOf3NestedAllOf3SealedLocked) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectPutAllOf1AllOf3NestedAllOf3SealedLocked(value)
	return nil
}

type RefStressObjectPutAllOf1AllOf3NullableRequired struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3NullableRequired)

func (s *RefStressObjectPutAllOf1AllOf3NullableRequired) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf1AllOf3OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3OptionalShared)

func (s *RefStressObjectPutAllOf1AllOf3OptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf1AllOf3SharedName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3SharedName)

func (s *RefStressObjectPutAllOf1AllOf3SharedName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf2 struct {
	RefStressObjectPutAllOf2AllOf1
	RefStressObjectPutAllOf2AllOf2
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf2)(nil)

func (a *RefStressObjectPutAllOf2) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectPutAllOf2AllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectPutAllOf2AllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectPutAllOf2AllOf1 struct {
	RefStressObjectPutAllOf2AllOf1AllOf1
	RefStressObjectPutAllOf2AllOf1AllOf2
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf1)(nil)

func (a *RefStressObjectPutAllOf2AllOf1) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectPutAllOf2AllOf1AllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectPutAllOf2AllOf1AllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectPutAllOf2AllOf1AllOf1 struct {
	FinalCode      RefStressObjectPutAllOf2AllOf1AllOf1FinalCode       `json:"finalCode"`
	Nested         *RefStressObjectPutAllOf2AllOf1AllOf1Nested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectPutAllOf2AllOf1AllOf1OptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectPutAllOf2AllOf1AllOf1SharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf1AllOf1)(nil)

func (o *RefStressObjectPutAllOf2AllOf1AllOf1) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectPutAllOf2AllOf1AllOf1FinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectPutAllOf2AllOf1AllOf1Nested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectPutAllOf2AllOf1AllOf1OptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectPutAllOf2AllOf1AllOf1SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectPutAllOf2AllOf1AllOf1FinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf1FinalCode)

func (s *RefStressObjectPutAllOf2AllOf1AllOf1FinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf2AllOf1AllOf1FinalCode(value)
	return nil
}

type RefStressObjectPutAllOf2AllOf1AllOf1Nested struct {
	Leaf     *RefStressObjectPutAllOf2AllOf1AllOf1NestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectPutAllOf2AllOf1AllOf1NestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf1AllOf1Nested)(nil)

func (o *RefStressObjectPutAllOf2AllOf1AllOf1Nested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectPutAllOf2AllOf1AllOf1NestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf2AllOf1AllOf1NestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectPutAllOf2AllOf1AllOf1NestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf1NestedLeaf)

func (s *RefStressObjectPutAllOf2AllOf1AllOf1NestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf2AllOf1AllOf1NestedLeaf(value)
	return nil
}

type RefStressObjectPutAllOf2AllOf1AllOf1NestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf1NestedSameName)

func (s *RefStressObjectPutAllOf2AllOf1AllOf1NestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf2AllOf1AllOf1OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf1OptionalShared)

func (s *RefStressObjectPutAllOf2AllOf1AllOf1OptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf2AllOf1AllOf1SharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf1SharedName)

func (s *RefStressObjectPutAllOf2AllOf1AllOf1SharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf2AllOf1AllOf1SharedName(value)
	return nil
}

type RefStressObjectPutAllOf2AllOf1AllOf2 struct {
	Final    *RefStressObjectPutAllOf2AllOf1AllOf2Final   `json:"final,omitzero"`
	Metadata RefStressObjectPutAllOf2AllOf1AllOf2Metadata `json:"metadata"`
	RootFlag RefStressObjectPutAllOf2AllOf1AllOf2RootFlag `json:"rootFlag"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf1AllOf2)(nil)

func (o *RefStressObjectPutAllOf2AllOf1AllOf2) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasMetadata bool
	var hasRootFlag bool

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
		case "final":

			var final RefStressObjectPutAllOf2AllOf1AllOf2Final
			err = json.Unmarshal(value, &final)
			if err != nil {
				return err
			}
			o.Final = &final
		case "metadata":
			hasMetadata = true

			var metadata RefStressObjectPutAllOf2AllOf1AllOf2Metadata
			err = json.Unmarshal(value, &metadata)
			if err != nil {
				return err
			}
			o.Metadata = metadata
		case "rootFlag":
			hasRootFlag = true

			var rootFlag RefStressObjectPutAllOf2AllOf1AllOf2RootFlag
			err = json.Unmarshal(value, &rootFlag)
			if err != nil {
				return err
			}
			o.RootFlag = rootFlag
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
	if !hasMetadata {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "metadata")
	}
	if !hasRootFlag {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "rootFlag")
	}

	return nil
}

type RefStressObjectPutAllOf2AllOf1AllOf2Final struct {
	FinalCode      RefStressObjectPutAllOf2AllOf1AllOf2FinalFinalCode       `json:"finalCode"`
	Nested         *RefStressObjectPutAllOf2AllOf1AllOf2FinalNested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectPutAllOf2AllOf1AllOf2FinalOptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectPutAllOf2AllOf1AllOf2FinalSharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf1AllOf2Final)(nil)

func (o *RefStressObjectPutAllOf2AllOf1AllOf2Final) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectPutAllOf2AllOf1AllOf2FinalFinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectPutAllOf2AllOf1AllOf2FinalNested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectPutAllOf2AllOf1AllOf2FinalOptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectPutAllOf2AllOf1AllOf2FinalSharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectPutAllOf2AllOf1AllOf2FinalFinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf2FinalFinalCode)

func (s *RefStressObjectPutAllOf2AllOf1AllOf2FinalFinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf2AllOf1AllOf2FinalFinalCode(value)
	return nil
}

type RefStressObjectPutAllOf2AllOf1AllOf2FinalNested struct {
	Leaf     *RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf1AllOf2FinalNested)(nil)

func (o *RefStressObjectPutAllOf2AllOf1AllOf2FinalNested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedLeaf)

func (s *RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedLeaf(value)
	return nil
}

type RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedSameName)

func (s *RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf2AllOf1AllOf2FinalOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf2FinalOptionalShared)

func (s *RefStressObjectPutAllOf2AllOf1AllOf2FinalOptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf2AllOf1AllOf2FinalSharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf2FinalSharedName)

func (s *RefStressObjectPutAllOf2AllOf1AllOf2FinalSharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf2AllOf1AllOf2FinalSharedName(value)
	return nil
}

type RefStressObjectPutAllOf2AllOf1AllOf2Metadata struct {
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf1AllOf2Metadata)(nil)

func (o *RefStressObjectPutAllOf2AllOf1AllOf2Metadata) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}

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
		default:
			var additionalProperty RefStressObjectPutAllOf2AllOf1AllOf2MetadataAdditionalProperty
			err = json.Unmarshal(value, &additionalProperty)
			if err != nil {
				return err
			}
		}
	}
	if _, err := d.Token(); err != nil {
		return err
	}
	if len(bytes.TrimSpace(data[d.InputOffset():])) != 0 {
		return NotAnObjectError
	}

	return nil
}

type RefStressObjectPutAllOf2AllOf1AllOf2MetadataAdditionalProperty string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf2MetadataAdditionalProperty)

func (s *RefStressObjectPutAllOf2AllOf1AllOf2MetadataAdditionalProperty) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf2AllOf1AllOf2MetadataAdditionalProperty(value)
	return nil
}

type RefStressObjectPutAllOf2AllOf1AllOf2RootFlag bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf2RootFlag)

func (b *RefStressObjectPutAllOf2AllOf1AllOf2RootFlag) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectPutAllOf2AllOf1AllOf2RootFlag(value)
	return nil
}

type RefStressObjectPutAllOf2AllOf2 struct {
	Count      RefStressObjectPutAllOf2AllOf2Count       `json:"count"`
	Finals     RefStressObjectPutAllOf2AllOf2Finals      `json:"finals"`
	Metadata   RefStressObjectPutAllOf2AllOf2Metadata    `json:"metadata"`
	RootFlag   RefStressObjectPutAllOf2AllOf2RootFlag    `json:"rootFlag"`
	SharedName *RefStressObjectPutAllOf2AllOf2SharedName `json:"sharedName,omitzero"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf2)(nil)

func (o *RefStressObjectPutAllOf2AllOf2) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasCount bool
	var hasFinals bool
	var hasMetadata bool
	var hasRootFlag bool

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
		case "count":
			hasCount = true

			var count RefStressObjectPutAllOf2AllOf2Count
			err = json.Unmarshal(value, &count)
			if err != nil {
				return err
			}
			o.Count = count
		case "finals":
			hasFinals = true

			var finals RefStressObjectPutAllOf2AllOf2Finals
			err = json.Unmarshal(value, &finals)
			if err != nil {
				return err
			}
			o.Finals = finals
		case "metadata":
			hasMetadata = true

			var metadata RefStressObjectPutAllOf2AllOf2Metadata
			err = json.Unmarshal(value, &metadata)
			if err != nil {
				return err
			}
			o.Metadata = metadata
		case "rootFlag":
			hasRootFlag = true

			var rootFlag RefStressObjectPutAllOf2AllOf2RootFlag
			err = json.Unmarshal(value, &rootFlag)
			if err != nil {
				return err
			}
			o.RootFlag = rootFlag
		case "sharedName":

			var sharedName RefStressObjectPutAllOf2AllOf2SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = &sharedName
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
	if !hasCount {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "count")
	}
	if !hasFinals {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finals")
	}
	if !hasMetadata {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "metadata")
	}
	if !hasRootFlag {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "rootFlag")
	}

	return nil
}

type RefStressObjectPutAllOf2AllOf2Count json.Number

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2Count)

func (n *RefStressObjectPutAllOf2AllOf2Count) UnmarshalJSON(data []byte) error {
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
	*n = RefStressObjectPutAllOf2AllOf2Count(value)
	return nil
}

type RefStressObjectPutAllOf2AllOf2Finals []RefStressObjectPutAllOf2AllOf2FinalsItem

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2Finals)

func (a *RefStressObjectPutAllOf2AllOf2Finals) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return fmt.Errorf("null for not nullable array")
	}

	var value []RefStressObjectPutAllOf2AllOf2FinalsItem
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	*a = RefStressObjectPutAllOf2AllOf2Finals(value)
	return nil
}

type RefStressObjectPutAllOf2AllOf2FinalsItem struct {
	FinalCode      RefStressObjectPutAllOf2AllOf2FinalsItemFinalCode       `json:"finalCode"`
	Nested         *RefStressObjectPutAllOf2AllOf2FinalsItemNested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectPutAllOf2AllOf2FinalsItemOptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectPutAllOf2AllOf2FinalsItemSharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf2FinalsItem)(nil)

func (o *RefStressObjectPutAllOf2AllOf2FinalsItem) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectPutAllOf2AllOf2FinalsItemFinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectPutAllOf2AllOf2FinalsItemNested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectPutAllOf2AllOf2FinalsItemOptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectPutAllOf2AllOf2FinalsItemSharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectPutAllOf2AllOf2FinalsItemFinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2FinalsItemFinalCode)

func (s *RefStressObjectPutAllOf2AllOf2FinalsItemFinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf2AllOf2FinalsItemFinalCode(value)
	return nil
}

type RefStressObjectPutAllOf2AllOf2FinalsItemNested struct {
	Leaf     *RefStressObjectPutAllOf2AllOf2FinalsItemNestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectPutAllOf2AllOf2FinalsItemNestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf2FinalsItemNested)(nil)

func (o *RefStressObjectPutAllOf2AllOf2FinalsItemNested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectPutAllOf2AllOf2FinalsItemNestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf2AllOf2FinalsItemNestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectPutAllOf2AllOf2FinalsItemNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2FinalsItemNestedLeaf)

func (s *RefStressObjectPutAllOf2AllOf2FinalsItemNestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf2AllOf2FinalsItemNestedLeaf(value)
	return nil
}

type RefStressObjectPutAllOf2AllOf2FinalsItemNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2FinalsItemNestedSameName)

func (s *RefStressObjectPutAllOf2AllOf2FinalsItemNestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf2AllOf2FinalsItemOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2FinalsItemOptionalShared)

func (s *RefStressObjectPutAllOf2AllOf2FinalsItemOptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf2AllOf2FinalsItemSharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2FinalsItemSharedName)

func (s *RefStressObjectPutAllOf2AllOf2FinalsItemSharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf2AllOf2FinalsItemSharedName(value)
	return nil
}

type RefStressObjectPutAllOf2AllOf2Metadata struct {
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf2Metadata)(nil)

func (o *RefStressObjectPutAllOf2AllOf2Metadata) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}

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
		default:
			var additionalProperty RefStressObjectPutAllOf2AllOf2MetadataAdditionalProperty
			err = json.Unmarshal(value, &additionalProperty)
			if err != nil {
				return err
			}
		}
	}
	if _, err := d.Token(); err != nil {
		return err
	}
	if len(bytes.TrimSpace(data[d.InputOffset():])) != 0 {
		return NotAnObjectError
	}

	return nil
}

type RefStressObjectPutAllOf2AllOf2MetadataAdditionalProperty string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2MetadataAdditionalProperty)

func (s *RefStressObjectPutAllOf2AllOf2MetadataAdditionalProperty) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf2AllOf2MetadataAdditionalProperty(value)
	return nil
}

type RefStressObjectPutAllOf2AllOf2RootFlag bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2RootFlag)

func (b *RefStressObjectPutAllOf2AllOf2RootFlag) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectPutAllOf2AllOf2RootFlag(value)
	return nil
}

type RefStressObjectPutAllOf2AllOf2SharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2SharedName)

func (s *RefStressObjectPutAllOf2AllOf2SharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf2AllOf2SharedName(value)
	return nil
}

type RefStressObjectPutAllOf3 struct {
	Count            RefStressObjectPutAllOf3Count            `json:"count"`
	Final            RefStressObjectPutAllOf3Final            `json:"final"`
	FinalCode        RefStressObjectPutAllOf3FinalCode        `json:"finalCode"`
	Finals           RefStressObjectPutAllOf3Finals           `json:"finals"`
	Metadata         RefStressObjectPutAllOf3Metadata         `json:"metadata"`
	MiddleFlag       RefStressObjectPutAllOf3MiddleFlag       `json:"middleFlag"`
	Nested           RefStressObjectPutAllOf3Nested           `json:"nested"`
	NullableRequired RefStressObjectPutAllOf3NullableRequired `json:"nullableRequired"`
	OptionalCode     *RefStressObjectPutAllOf3OptionalCode    `json:"optionalCode,omitzero"`
	OptionalShared   *RefStressObjectPutAllOf3OptionalShared  `json:"optionalShared,omitzero"`
	RootFlag         RefStressObjectPutAllOf3RootFlag         `json:"rootFlag"`
	SharedName       RefStressObjectPutAllOf3SharedName       `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf3)(nil)

func (o *RefStressObjectPutAllOf3) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasCount bool
	var hasFinal bool
	var hasFinalCode bool
	var hasFinals bool
	var hasMetadata bool
	var hasMiddleFlag bool
	var hasNested bool
	var hasNullableRequired bool
	var hasRootFlag bool
	var hasSharedName bool

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
		case "count":
			hasCount = true

			var count RefStressObjectPutAllOf3Count
			err = json.Unmarshal(value, &count)
			if err != nil {
				return err
			}
			o.Count = count
		case "final":
			hasFinal = true

			var final RefStressObjectPutAllOf3Final
			err = json.Unmarshal(value, &final)
			if err != nil {
				return err
			}
			o.Final = final
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectPutAllOf3FinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "finals":
			hasFinals = true

			var finals RefStressObjectPutAllOf3Finals
			err = json.Unmarshal(value, &finals)
			if err != nil {
				return err
			}
			o.Finals = finals
		case "metadata":
			hasMetadata = true

			var metadata RefStressObjectPutAllOf3Metadata
			err = json.Unmarshal(value, &metadata)
			if err != nil {
				return err
			}
			o.Metadata = metadata
		case "middleFlag":
			hasMiddleFlag = true

			var middleFlag RefStressObjectPutAllOf3MiddleFlag
			err = json.Unmarshal(value, &middleFlag)
			if err != nil {
				return err
			}
			o.MiddleFlag = middleFlag
		case "nested":
			hasNested = true

			var nested RefStressObjectPutAllOf3Nested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = nested
		case "nullableRequired":
			hasNullableRequired = true

			var nullableRequired RefStressObjectPutAllOf3NullableRequired
			err = json.Unmarshal(value, &nullableRequired)
			if err != nil {
				return err
			}
			o.NullableRequired = nullableRequired
		case "optionalCode":

			var optionalCode RefStressObjectPutAllOf3OptionalCode
			err = json.Unmarshal(value, &optionalCode)
			if err != nil {
				return err
			}
			o.OptionalCode = &optionalCode
		case "optionalShared":

			var optionalShared RefStressObjectPutAllOf3OptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "rootFlag":
			hasRootFlag = true

			var rootFlag RefStressObjectPutAllOf3RootFlag
			err = json.Unmarshal(value, &rootFlag)
			if err != nil {
				return err
			}
			o.RootFlag = rootFlag
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectPutAllOf3SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasCount {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "count")
	}
	if !hasFinal {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "final")
	}
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasFinals {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finals")
	}
	if !hasMetadata {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "metadata")
	}
	if !hasMiddleFlag {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "middleFlag")
	}
	if !hasNested {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "nested")
	}
	if !hasNullableRequired {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "nullableRequired")
	}
	if !hasRootFlag {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "rootFlag")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectPutAllOf3Count json.Number

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3Count)

func (n *RefStressObjectPutAllOf3Count) UnmarshalJSON(data []byte) error {
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
	*n = RefStressObjectPutAllOf3Count(value)
	return nil
}

type RefStressObjectPutAllOf3Final struct {
	FinalCode      RefStressObjectPutAllOf3FinalFinalCode       `json:"finalCode"`
	Nested         *RefStressObjectPutAllOf3FinalNested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectPutAllOf3FinalOptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectPutAllOf3FinalSharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf3Final)(nil)

func (o *RefStressObjectPutAllOf3Final) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectPutAllOf3FinalFinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectPutAllOf3FinalNested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectPutAllOf3FinalOptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectPutAllOf3FinalSharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectPutAllOf3FinalFinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalFinalCode)

func (s *RefStressObjectPutAllOf3FinalFinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf3FinalFinalCode(value)
	return nil
}

type RefStressObjectPutAllOf3FinalNested struct {
	Leaf     *RefStressObjectPutAllOf3FinalNestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectPutAllOf3FinalNestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf3FinalNested)(nil)

func (o *RefStressObjectPutAllOf3FinalNested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectPutAllOf3FinalNestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf3FinalNestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectPutAllOf3FinalNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalNestedLeaf)

func (s *RefStressObjectPutAllOf3FinalNestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf3FinalNestedLeaf(value)
	return nil
}

type RefStressObjectPutAllOf3FinalNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalNestedSameName)

func (s *RefStressObjectPutAllOf3FinalNestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf3FinalOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalOptionalShared)

func (s *RefStressObjectPutAllOf3FinalOptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf3FinalSharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalSharedName)

func (s *RefStressObjectPutAllOf3FinalSharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf3FinalSharedName(value)
	return nil
}

type RefStressObjectPutAllOf3FinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalCode)

func (s *RefStressObjectPutAllOf3FinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf3FinalCode(value)
	return nil
}

type RefStressObjectPutAllOf3Finals []RefStressObjectPutAllOf3FinalsItem

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3Finals)

func (a *RefStressObjectPutAllOf3Finals) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return fmt.Errorf("null for not nullable array")
	}

	var value []RefStressObjectPutAllOf3FinalsItem
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	*a = RefStressObjectPutAllOf3Finals(value)
	return nil
}

type RefStressObjectPutAllOf3FinalsItem struct {
	FinalCode      RefStressObjectPutAllOf3FinalsItemFinalCode       `json:"finalCode"`
	Nested         *RefStressObjectPutAllOf3FinalsItemNested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectPutAllOf3FinalsItemOptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectPutAllOf3FinalsItemSharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf3FinalsItem)(nil)

func (o *RefStressObjectPutAllOf3FinalsItem) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectPutAllOf3FinalsItemFinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectPutAllOf3FinalsItemNested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectPutAllOf3FinalsItemOptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectPutAllOf3FinalsItemSharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectPutAllOf3FinalsItemFinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalsItemFinalCode)

func (s *RefStressObjectPutAllOf3FinalsItemFinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf3FinalsItemFinalCode(value)
	return nil
}

type RefStressObjectPutAllOf3FinalsItemNested struct {
	Leaf     *RefStressObjectPutAllOf3FinalsItemNestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectPutAllOf3FinalsItemNestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf3FinalsItemNested)(nil)

func (o *RefStressObjectPutAllOf3FinalsItemNested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectPutAllOf3FinalsItemNestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf3FinalsItemNestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectPutAllOf3FinalsItemNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalsItemNestedLeaf)

func (s *RefStressObjectPutAllOf3FinalsItemNestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf3FinalsItemNestedLeaf(value)
	return nil
}

type RefStressObjectPutAllOf3FinalsItemNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalsItemNestedSameName)

func (s *RefStressObjectPutAllOf3FinalsItemNestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf3FinalsItemOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalsItemOptionalShared)

func (s *RefStressObjectPutAllOf3FinalsItemOptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf3FinalsItemSharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalsItemSharedName)

func (s *RefStressObjectPutAllOf3FinalsItemSharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf3FinalsItemSharedName(value)
	return nil
}

type RefStressObjectPutAllOf3Metadata struct {
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf3Metadata)(nil)

func (o *RefStressObjectPutAllOf3Metadata) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}

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
		default:
			var additionalProperty RefStressObjectPutAllOf3MetadataAdditionalProperty
			err = json.Unmarshal(value, &additionalProperty)
			if err != nil {
				return err
			}
		}
	}
	if _, err := d.Token(); err != nil {
		return err
	}
	if len(bytes.TrimSpace(data[d.InputOffset():])) != 0 {
		return NotAnObjectError
	}

	return nil
}

type RefStressObjectPutAllOf3MetadataAdditionalProperty string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3MetadataAdditionalProperty)

func (s *RefStressObjectPutAllOf3MetadataAdditionalProperty) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf3MetadataAdditionalProperty(value)
	return nil
}

type RefStressObjectPutAllOf3MiddleFlag bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3MiddleFlag)

func (b *RefStressObjectPutAllOf3MiddleFlag) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectPutAllOf3MiddleFlag(value)
	return nil
}

type RefStressObjectPutAllOf3Nested struct {
	RefStressObjectPutAllOf3NestedAllOf1
	RefStressObjectPutAllOf3NestedAllOf2
	RefStressObjectPutAllOf3NestedAllOf3
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf3Nested)(nil)

func (a *RefStressObjectPutAllOf3Nested) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectPutAllOf3NestedAllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectPutAllOf3NestedAllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectPutAllOf3NestedAllOf3.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectPutAllOf3NestedAllOf1 struct {
	Leaf     *RefStressObjectPutAllOf3NestedAllOf1Leaf    `json:"leaf,omitzero"`
	SameName RefStressObjectPutAllOf3NestedAllOf1SameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf3NestedAllOf1)(nil)

func (o *RefStressObjectPutAllOf3NestedAllOf1) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectPutAllOf3NestedAllOf1Leaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf3NestedAllOf1SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectPutAllOf3NestedAllOf1Leaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3NestedAllOf1Leaf)

func (s *RefStressObjectPutAllOf3NestedAllOf1Leaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf3NestedAllOf1Leaf(value)
	return nil
}

type RefStressObjectPutAllOf3NestedAllOf1SameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3NestedAllOf1SameName)

func (s *RefStressObjectPutAllOf3NestedAllOf1SameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf3NestedAllOf2 struct {
	Leaf     *RefStressObjectPutAllOf3NestedAllOf2Leaf    `json:"leaf,omitzero"`
	SameName RefStressObjectPutAllOf3NestedAllOf2SameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf3NestedAllOf2)(nil)

func (o *RefStressObjectPutAllOf3NestedAllOf2) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectPutAllOf3NestedAllOf2Leaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf3NestedAllOf2SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectPutAllOf3NestedAllOf2Leaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3NestedAllOf2Leaf)

func (s *RefStressObjectPutAllOf3NestedAllOf2Leaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf3NestedAllOf2Leaf(value)
	return nil
}

type RefStressObjectPutAllOf3NestedAllOf2SameName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3NestedAllOf2SameName)

func (s *RefStressObjectPutAllOf3NestedAllOf2SameName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf3NestedAllOf2SameName(value)
	return nil
}

type RefStressObjectPutAllOf3NestedAllOf3 struct {
	SameName RefStressObjectPutAllOf3NestedAllOf3SameName `json:"sameName"`
	Sealed   RefStressObjectPutAllOf3NestedAllOf3Sealed   `json:"sealed"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf3NestedAllOf3)(nil)

func (o *RefStressObjectPutAllOf3NestedAllOf3) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool
	var hasSealed bool

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
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectPutAllOf3NestedAllOf3SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
		case "sealed":
			hasSealed = true

			var sealed RefStressObjectPutAllOf3NestedAllOf3Sealed
			err = json.Unmarshal(value, &sealed)
			if err != nil {
				return err
			}
			o.Sealed = sealed
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}
	if !hasSealed {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sealed")
	}

	return nil
}

type RefStressObjectPutAllOf3NestedAllOf3SameName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3NestedAllOf3SameName)

func (s *RefStressObjectPutAllOf3NestedAllOf3SameName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf3NestedAllOf3SameName(value)
	return nil
}

type RefStressObjectPutAllOf3NestedAllOf3Sealed struct {
	Locked RefStressObjectPutAllOf3NestedAllOf3SealedLocked `json:"locked"`
}

var _ json.Unmarshaler = (*RefStressObjectPutAllOf3NestedAllOf3Sealed)(nil)

func (o *RefStressObjectPutAllOf3NestedAllOf3Sealed) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasLocked bool

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
		case "locked":
			hasLocked = true

			var locked RefStressObjectPutAllOf3NestedAllOf3SealedLocked
			err = json.Unmarshal(value, &locked)
			if err != nil {
				return err
			}
			o.Locked = locked
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
	if !hasLocked {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "locked")
	}

	return nil
}

type RefStressObjectPutAllOf3NestedAllOf3SealedLocked bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3NestedAllOf3SealedLocked)

func (b *RefStressObjectPutAllOf3NestedAllOf3SealedLocked) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectPutAllOf3NestedAllOf3SealedLocked(value)
	return nil
}

type RefStressObjectPutAllOf3NullableRequired struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3NullableRequired)

func (s *RefStressObjectPutAllOf3NullableRequired) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf3OptionalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3OptionalCode)

func (s *RefStressObjectPutAllOf3OptionalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf3OptionalCode(value)
	return nil
}

type RefStressObjectPutAllOf3OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3OptionalShared)

func (s *RefStressObjectPutAllOf3OptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectPutAllOf3RootFlag bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3RootFlag)

func (b *RefStressObjectPutAllOf3RootFlag) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectPutAllOf3RootFlag(value)
	return nil
}

type RefStressObjectPutAllOf3SharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3SharedName)

func (s *RefStressObjectPutAllOf3SharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectPutAllOf3SharedName(value)
	return nil
}

type RefStressObject struct {
	RefStressObjectAllOf1
	RefStressObjectAllOf2
	RefStressObjectAllOf3
}

var _ json.Unmarshaler = (*RefStressObject)(nil)

func (a *RefStressObject) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectAllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectAllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectAllOf3.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectAllOf1 struct {
	RefStressObjectAllOf1AllOf1
	RefStressObjectAllOf1AllOf2
	RefStressObjectAllOf1AllOf3
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1)(nil)

func (a *RefStressObjectAllOf1) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectAllOf1AllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectAllOf1AllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectAllOf1AllOf3.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectAllOf1AllOf1 struct {
	FinalCode      RefStressObjectAllOf1AllOf1FinalCode       `json:"finalCode"`
	Nested         *RefStressObjectAllOf1AllOf1Nested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectAllOf1AllOf1OptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectAllOf1AllOf1SharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf1)(nil)

func (o *RefStressObjectAllOf1AllOf1) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectAllOf1AllOf1FinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectAllOf1AllOf1Nested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectAllOf1AllOf1OptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectAllOf1AllOf1SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectAllOf1AllOf1FinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf1FinalCode)

func (s *RefStressObjectAllOf1AllOf1FinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf1FinalCode(value)
	return nil
}

type RefStressObjectAllOf1AllOf1Nested struct {
	Leaf     *RefStressObjectAllOf1AllOf1NestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectAllOf1AllOf1NestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf1Nested)(nil)

func (o *RefStressObjectAllOf1AllOf1Nested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectAllOf1AllOf1NestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf1AllOf1NestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectAllOf1AllOf1NestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf1NestedLeaf)

func (s *RefStressObjectAllOf1AllOf1NestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf1NestedLeaf(value)
	return nil
}

type RefStressObjectAllOf1AllOf1NestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf1NestedSameName)

func (s *RefStressObjectAllOf1AllOf1NestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf1AllOf1OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf1OptionalShared)

func (s *RefStressObjectAllOf1AllOf1OptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf1AllOf1SharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf1SharedName)

func (s *RefStressObjectAllOf1AllOf1SharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf1SharedName(value)
	return nil
}

type RefStressObjectAllOf1AllOf2 struct {
	RefStressObjectAllOf1AllOf2AllOf1
	RefStressObjectAllOf1AllOf2AllOf2
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2)(nil)

func (a *RefStressObjectAllOf1AllOf2) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectAllOf1AllOf2AllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectAllOf1AllOf2AllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectAllOf1AllOf2AllOf1 struct {
	RefStressObjectAllOf1AllOf2AllOf1AllOf1
	RefStressObjectAllOf1AllOf2AllOf1AllOf2
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf1)(nil)

func (a *RefStressObjectAllOf1AllOf2AllOf1) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectAllOf1AllOf2AllOf1AllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectAllOf1AllOf2AllOf1AllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectAllOf1AllOf2AllOf1AllOf1 struct {
	FinalCode      RefStressObjectAllOf1AllOf2AllOf1AllOf1FinalCode       `json:"finalCode"`
	Nested         *RefStressObjectAllOf1AllOf2AllOf1AllOf1Nested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectAllOf1AllOf2AllOf1AllOf1OptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectAllOf1AllOf2AllOf1AllOf1SharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf1AllOf1)(nil)

func (o *RefStressObjectAllOf1AllOf2AllOf1AllOf1) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectAllOf1AllOf2AllOf1AllOf1FinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectAllOf1AllOf2AllOf1AllOf1Nested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectAllOf1AllOf2AllOf1AllOf1OptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectAllOf1AllOf2AllOf1AllOf1SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectAllOf1AllOf2AllOf1AllOf1FinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf1AllOf1FinalCode)

func (s *RefStressObjectAllOf1AllOf2AllOf1AllOf1FinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf2AllOf1AllOf1FinalCode(value)
	return nil
}

type RefStressObjectAllOf1AllOf2AllOf1AllOf1Nested struct {
	Leaf     *RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf1AllOf1Nested)(nil)

func (o *RefStressObjectAllOf1AllOf2AllOf1AllOf1Nested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedLeaf)

func (s *RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedLeaf(value)
	return nil
}

type RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedSameName)

func (s *RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf1AllOf2AllOf1AllOf1OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf1AllOf1OptionalShared)

func (s *RefStressObjectAllOf1AllOf2AllOf1AllOf1OptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf1AllOf2AllOf1AllOf1SharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf1AllOf1SharedName)

func (s *RefStressObjectAllOf1AllOf2AllOf1AllOf1SharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf2AllOf1AllOf1SharedName(value)
	return nil
}

type RefStressObjectAllOf1AllOf2AllOf1AllOf2 struct {
	OptionalCode *RefStressObjectAllOf1AllOf2AllOf1AllOf2OptionalCode `json:"optionalCode,omitzero"`
	SharedName   RefStressObjectAllOf1AllOf2AllOf1AllOf2SharedName    `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf1AllOf2)(nil)

func (o *RefStressObjectAllOf1AllOf2AllOf1AllOf2) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSharedName bool

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
		case "optionalCode":

			var optionalCode RefStressObjectAllOf1AllOf2AllOf1AllOf2OptionalCode
			err = json.Unmarshal(value, &optionalCode)
			if err != nil {
				return err
			}
			o.OptionalCode = &optionalCode
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectAllOf1AllOf2AllOf1AllOf2SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectAllOf1AllOf2AllOf1AllOf2OptionalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf1AllOf2OptionalCode)

func (s *RefStressObjectAllOf1AllOf2AllOf1AllOf2OptionalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf2AllOf1AllOf2OptionalCode(value)
	return nil
}

type RefStressObjectAllOf1AllOf2AllOf1AllOf2SharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf1AllOf2SharedName)

func (s *RefStressObjectAllOf1AllOf2AllOf1AllOf2SharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf2AllOf1AllOf2SharedName(value)
	return nil
}

type RefStressObjectAllOf1AllOf2AllOf2 struct {
	MiddleFlag RefStressObjectAllOf1AllOf2AllOf2MiddleFlag `json:"middleFlag"`
	Nested     *RefStressObjectAllOf1AllOf2AllOf2Nested    `json:"nested,omitzero"`
	SharedName RefStressObjectAllOf1AllOf2AllOf2SharedName `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf2)(nil)

func (o *RefStressObjectAllOf1AllOf2AllOf2) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasMiddleFlag bool
	var hasSharedName bool

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
		case "middleFlag":
			hasMiddleFlag = true

			var middleFlag RefStressObjectAllOf1AllOf2AllOf2MiddleFlag
			err = json.Unmarshal(value, &middleFlag)
			if err != nil {
				return err
			}
			o.MiddleFlag = middleFlag
		case "nested":

			var nested RefStressObjectAllOf1AllOf2AllOf2Nested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectAllOf1AllOf2AllOf2SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasMiddleFlag {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "middleFlag")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectAllOf1AllOf2AllOf2MiddleFlag bool

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2MiddleFlag)

func (b *RefStressObjectAllOf1AllOf2AllOf2MiddleFlag) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectAllOf1AllOf2AllOf2MiddleFlag(value)
	return nil
}

type RefStressObjectAllOf1AllOf2AllOf2Nested struct {
	RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1
	RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2
	RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf2Nested)(nil)

func (a *RefStressObjectAllOf1AllOf2AllOf2Nested) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1 struct {
	Leaf     *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1Leaf    `json:"leaf,omitzero"`
	SameName RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1SameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1)(nil)

func (o *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1Leaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1Leaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1Leaf)

func (s *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1Leaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1Leaf(value)
	return nil
}

type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1SameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1SameName)

func (s *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1SameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2 struct {
	Leaf     *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2Leaf    `json:"leaf,omitzero"`
	SameName RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2SameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2)(nil)

func (o *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2Leaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2Leaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2Leaf)

func (s *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2Leaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2Leaf(value)
	return nil
}

type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2SameName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2SameName)

func (s *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2SameName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2SameName(value)
	return nil
}

type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3 struct {
	SameName RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SameName `json:"sameName"`
	Sealed   RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3Sealed   `json:"sealed"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3)(nil)

func (o *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool
	var hasSealed bool

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
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
		case "sealed":
			hasSealed = true

			var sealed RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3Sealed
			err = json.Unmarshal(value, &sealed)
			if err != nil {
				return err
			}
			o.Sealed = sealed
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}
	if !hasSealed {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sealed")
	}

	return nil
}

type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SameName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SameName)

func (s *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SameName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SameName(value)
	return nil
}

type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3Sealed struct {
	Locked RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SealedLocked `json:"locked"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3Sealed)(nil)

func (o *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3Sealed) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasLocked bool

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
		case "locked":
			hasLocked = true

			var locked RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SealedLocked
			err = json.Unmarshal(value, &locked)
			if err != nil {
				return err
			}
			o.Locked = locked
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
	if !hasLocked {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "locked")
	}

	return nil
}

type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SealedLocked bool

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SealedLocked)

func (b *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SealedLocked) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SealedLocked(value)
	return nil
}

type RefStressObjectAllOf1AllOf2AllOf2SharedName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2SharedName)

func (s *RefStressObjectAllOf1AllOf2AllOf2SharedName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf1AllOf3 struct {
	Final            RefStressObjectAllOf1AllOf3Final            `json:"final"`
	Nested           RefStressObjectAllOf1AllOf3Nested           `json:"nested"`
	NullableRequired RefStressObjectAllOf1AllOf3NullableRequired `json:"nullableRequired"`
	OptionalShared   *RefStressObjectAllOf1AllOf3OptionalShared  `json:"optionalShared,omitzero"`
	SharedName       *RefStressObjectAllOf1AllOf3SharedName      `json:"sharedName,omitzero"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3)(nil)

func (o *RefStressObjectAllOf1AllOf3) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinal bool
	var hasNested bool
	var hasNullableRequired bool

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
		case "final":
			hasFinal = true

			var final RefStressObjectAllOf1AllOf3Final
			err = json.Unmarshal(value, &final)
			if err != nil {
				return err
			}
			o.Final = final
		case "nested":
			hasNested = true

			var nested RefStressObjectAllOf1AllOf3Nested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = nested
		case "nullableRequired":
			hasNullableRequired = true

			var nullableRequired RefStressObjectAllOf1AllOf3NullableRequired
			err = json.Unmarshal(value, &nullableRequired)
			if err != nil {
				return err
			}
			o.NullableRequired = nullableRequired
		case "optionalShared":

			var optionalShared RefStressObjectAllOf1AllOf3OptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":

			var sharedName RefStressObjectAllOf1AllOf3SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = &sharedName
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
	if !hasFinal {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "final")
	}
	if !hasNested {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "nested")
	}
	if !hasNullableRequired {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "nullableRequired")
	}

	return nil
}

type RefStressObjectAllOf1AllOf3Final struct {
	FinalCode      RefStressObjectAllOf1AllOf3FinalFinalCode       `json:"finalCode"`
	Nested         *RefStressObjectAllOf1AllOf3FinalNested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectAllOf1AllOf3FinalOptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectAllOf1AllOf3FinalSharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3Final)(nil)

func (o *RefStressObjectAllOf1AllOf3Final) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectAllOf1AllOf3FinalFinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectAllOf1AllOf3FinalNested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectAllOf1AllOf3FinalOptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectAllOf1AllOf3FinalSharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectAllOf1AllOf3FinalFinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3FinalFinalCode)

func (s *RefStressObjectAllOf1AllOf3FinalFinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf3FinalFinalCode(value)
	return nil
}

type RefStressObjectAllOf1AllOf3FinalNested struct {
	Leaf     *RefStressObjectAllOf1AllOf3FinalNestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectAllOf1AllOf3FinalNestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3FinalNested)(nil)

func (o *RefStressObjectAllOf1AllOf3FinalNested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectAllOf1AllOf3FinalNestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf1AllOf3FinalNestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectAllOf1AllOf3FinalNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3FinalNestedLeaf)

func (s *RefStressObjectAllOf1AllOf3FinalNestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf3FinalNestedLeaf(value)
	return nil
}

type RefStressObjectAllOf1AllOf3FinalNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3FinalNestedSameName)

func (s *RefStressObjectAllOf1AllOf3FinalNestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf1AllOf3FinalOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3FinalOptionalShared)

func (s *RefStressObjectAllOf1AllOf3FinalOptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf1AllOf3FinalSharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3FinalSharedName)

func (s *RefStressObjectAllOf1AllOf3FinalSharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf3FinalSharedName(value)
	return nil
}

type RefStressObjectAllOf1AllOf3Nested struct {
	RefStressObjectAllOf1AllOf3NestedAllOf1
	RefStressObjectAllOf1AllOf3NestedAllOf2
	RefStressObjectAllOf1AllOf3NestedAllOf3
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3Nested)(nil)

func (a *RefStressObjectAllOf1AllOf3Nested) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectAllOf1AllOf3NestedAllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectAllOf1AllOf3NestedAllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectAllOf1AllOf3NestedAllOf3.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectAllOf1AllOf3NestedAllOf1 struct {
	Leaf     *RefStressObjectAllOf1AllOf3NestedAllOf1Leaf    `json:"leaf,omitzero"`
	SameName RefStressObjectAllOf1AllOf3NestedAllOf1SameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3NestedAllOf1)(nil)

func (o *RefStressObjectAllOf1AllOf3NestedAllOf1) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectAllOf1AllOf3NestedAllOf1Leaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf1AllOf3NestedAllOf1SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectAllOf1AllOf3NestedAllOf1Leaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3NestedAllOf1Leaf)

func (s *RefStressObjectAllOf1AllOf3NestedAllOf1Leaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf3NestedAllOf1Leaf(value)
	return nil
}

type RefStressObjectAllOf1AllOf3NestedAllOf1SameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3NestedAllOf1SameName)

func (s *RefStressObjectAllOf1AllOf3NestedAllOf1SameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf1AllOf3NestedAllOf2 struct {
	Leaf     *RefStressObjectAllOf1AllOf3NestedAllOf2Leaf    `json:"leaf,omitzero"`
	SameName RefStressObjectAllOf1AllOf3NestedAllOf2SameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3NestedAllOf2)(nil)

func (o *RefStressObjectAllOf1AllOf3NestedAllOf2) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectAllOf1AllOf3NestedAllOf2Leaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf1AllOf3NestedAllOf2SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectAllOf1AllOf3NestedAllOf2Leaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3NestedAllOf2Leaf)

func (s *RefStressObjectAllOf1AllOf3NestedAllOf2Leaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf3NestedAllOf2Leaf(value)
	return nil
}

type RefStressObjectAllOf1AllOf3NestedAllOf2SameName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3NestedAllOf2SameName)

func (s *RefStressObjectAllOf1AllOf3NestedAllOf2SameName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf3NestedAllOf2SameName(value)
	return nil
}

type RefStressObjectAllOf1AllOf3NestedAllOf3 struct {
	SameName RefStressObjectAllOf1AllOf3NestedAllOf3SameName `json:"sameName"`
	Sealed   RefStressObjectAllOf1AllOf3NestedAllOf3Sealed   `json:"sealed"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3NestedAllOf3)(nil)

func (o *RefStressObjectAllOf1AllOf3NestedAllOf3) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool
	var hasSealed bool

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
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf1AllOf3NestedAllOf3SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
		case "sealed":
			hasSealed = true

			var sealed RefStressObjectAllOf1AllOf3NestedAllOf3Sealed
			err = json.Unmarshal(value, &sealed)
			if err != nil {
				return err
			}
			o.Sealed = sealed
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}
	if !hasSealed {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sealed")
	}

	return nil
}

type RefStressObjectAllOf1AllOf3NestedAllOf3SameName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3NestedAllOf3SameName)

func (s *RefStressObjectAllOf1AllOf3NestedAllOf3SameName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf1AllOf3NestedAllOf3SameName(value)
	return nil
}

type RefStressObjectAllOf1AllOf3NestedAllOf3Sealed struct {
	Locked RefStressObjectAllOf1AllOf3NestedAllOf3SealedLocked `json:"locked"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3NestedAllOf3Sealed)(nil)

func (o *RefStressObjectAllOf1AllOf3NestedAllOf3Sealed) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasLocked bool

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
		case "locked":
			hasLocked = true

			var locked RefStressObjectAllOf1AllOf3NestedAllOf3SealedLocked
			err = json.Unmarshal(value, &locked)
			if err != nil {
				return err
			}
			o.Locked = locked
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
	if !hasLocked {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "locked")
	}

	return nil
}

type RefStressObjectAllOf1AllOf3NestedAllOf3SealedLocked bool

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3NestedAllOf3SealedLocked)

func (b *RefStressObjectAllOf1AllOf3NestedAllOf3SealedLocked) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectAllOf1AllOf3NestedAllOf3SealedLocked(value)
	return nil
}

type RefStressObjectAllOf1AllOf3NullableRequired struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3NullableRequired)

func (s *RefStressObjectAllOf1AllOf3NullableRequired) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf1AllOf3OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3OptionalShared)

func (s *RefStressObjectAllOf1AllOf3OptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf1AllOf3SharedName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3SharedName)

func (s *RefStressObjectAllOf1AllOf3SharedName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf2 struct {
	RefStressObjectAllOf2AllOf1
	RefStressObjectAllOf2AllOf2
}

var _ json.Unmarshaler = (*RefStressObjectAllOf2)(nil)

func (a *RefStressObjectAllOf2) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectAllOf2AllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectAllOf2AllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectAllOf2AllOf1 struct {
	RefStressObjectAllOf2AllOf1AllOf1
	RefStressObjectAllOf2AllOf1AllOf2
}

var _ json.Unmarshaler = (*RefStressObjectAllOf2AllOf1)(nil)

func (a *RefStressObjectAllOf2AllOf1) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectAllOf2AllOf1AllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectAllOf2AllOf1AllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectAllOf2AllOf1AllOf1 struct {
	FinalCode      RefStressObjectAllOf2AllOf1AllOf1FinalCode       `json:"finalCode"`
	Nested         *RefStressObjectAllOf2AllOf1AllOf1Nested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectAllOf2AllOf1AllOf1OptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectAllOf2AllOf1AllOf1SharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf2AllOf1AllOf1)(nil)

func (o *RefStressObjectAllOf2AllOf1AllOf1) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectAllOf2AllOf1AllOf1FinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectAllOf2AllOf1AllOf1Nested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectAllOf2AllOf1AllOf1OptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectAllOf2AllOf1AllOf1SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectAllOf2AllOf1AllOf1FinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf1FinalCode)

func (s *RefStressObjectAllOf2AllOf1AllOf1FinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf2AllOf1AllOf1FinalCode(value)
	return nil
}

type RefStressObjectAllOf2AllOf1AllOf1Nested struct {
	Leaf     *RefStressObjectAllOf2AllOf1AllOf1NestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectAllOf2AllOf1AllOf1NestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf2AllOf1AllOf1Nested)(nil)

func (o *RefStressObjectAllOf2AllOf1AllOf1Nested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectAllOf2AllOf1AllOf1NestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf2AllOf1AllOf1NestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectAllOf2AllOf1AllOf1NestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf1NestedLeaf)

func (s *RefStressObjectAllOf2AllOf1AllOf1NestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf2AllOf1AllOf1NestedLeaf(value)
	return nil
}

type RefStressObjectAllOf2AllOf1AllOf1NestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf1NestedSameName)

func (s *RefStressObjectAllOf2AllOf1AllOf1NestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf2AllOf1AllOf1OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf1OptionalShared)

func (s *RefStressObjectAllOf2AllOf1AllOf1OptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf2AllOf1AllOf1SharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf1SharedName)

func (s *RefStressObjectAllOf2AllOf1AllOf1SharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf2AllOf1AllOf1SharedName(value)
	return nil
}

type RefStressObjectAllOf2AllOf1AllOf2 struct {
	Final    *RefStressObjectAllOf2AllOf1AllOf2Final   `json:"final,omitzero"`
	Metadata RefStressObjectAllOf2AllOf1AllOf2Metadata `json:"metadata"`
	RootFlag RefStressObjectAllOf2AllOf1AllOf2RootFlag `json:"rootFlag"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf2AllOf1AllOf2)(nil)

func (o *RefStressObjectAllOf2AllOf1AllOf2) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasMetadata bool
	var hasRootFlag bool

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
		case "final":

			var final RefStressObjectAllOf2AllOf1AllOf2Final
			err = json.Unmarshal(value, &final)
			if err != nil {
				return err
			}
			o.Final = &final
		case "metadata":
			hasMetadata = true

			var metadata RefStressObjectAllOf2AllOf1AllOf2Metadata
			err = json.Unmarshal(value, &metadata)
			if err != nil {
				return err
			}
			o.Metadata = metadata
		case "rootFlag":
			hasRootFlag = true

			var rootFlag RefStressObjectAllOf2AllOf1AllOf2RootFlag
			err = json.Unmarshal(value, &rootFlag)
			if err != nil {
				return err
			}
			o.RootFlag = rootFlag
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
	if !hasMetadata {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "metadata")
	}
	if !hasRootFlag {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "rootFlag")
	}

	return nil
}

type RefStressObjectAllOf2AllOf1AllOf2Final struct {
	FinalCode      RefStressObjectAllOf2AllOf1AllOf2FinalFinalCode       `json:"finalCode"`
	Nested         *RefStressObjectAllOf2AllOf1AllOf2FinalNested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectAllOf2AllOf1AllOf2FinalOptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectAllOf2AllOf1AllOf2FinalSharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf2AllOf1AllOf2Final)(nil)

func (o *RefStressObjectAllOf2AllOf1AllOf2Final) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectAllOf2AllOf1AllOf2FinalFinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectAllOf2AllOf1AllOf2FinalNested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectAllOf2AllOf1AllOf2FinalOptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectAllOf2AllOf1AllOf2FinalSharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectAllOf2AllOf1AllOf2FinalFinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf2FinalFinalCode)

func (s *RefStressObjectAllOf2AllOf1AllOf2FinalFinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf2AllOf1AllOf2FinalFinalCode(value)
	return nil
}

type RefStressObjectAllOf2AllOf1AllOf2FinalNested struct {
	Leaf     *RefStressObjectAllOf2AllOf1AllOf2FinalNestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectAllOf2AllOf1AllOf2FinalNestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf2AllOf1AllOf2FinalNested)(nil)

func (o *RefStressObjectAllOf2AllOf1AllOf2FinalNested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectAllOf2AllOf1AllOf2FinalNestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf2AllOf1AllOf2FinalNestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectAllOf2AllOf1AllOf2FinalNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf2FinalNestedLeaf)

func (s *RefStressObjectAllOf2AllOf1AllOf2FinalNestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf2AllOf1AllOf2FinalNestedLeaf(value)
	return nil
}

type RefStressObjectAllOf2AllOf1AllOf2FinalNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf2FinalNestedSameName)

func (s *RefStressObjectAllOf2AllOf1AllOf2FinalNestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf2AllOf1AllOf2FinalOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf2FinalOptionalShared)

func (s *RefStressObjectAllOf2AllOf1AllOf2FinalOptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf2AllOf1AllOf2FinalSharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf2FinalSharedName)

func (s *RefStressObjectAllOf2AllOf1AllOf2FinalSharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf2AllOf1AllOf2FinalSharedName(value)
	return nil
}

type RefStressObjectAllOf2AllOf1AllOf2Metadata struct {
}

var _ json.Unmarshaler = (*RefStressObjectAllOf2AllOf1AllOf2Metadata)(nil)

func (o *RefStressObjectAllOf2AllOf1AllOf2Metadata) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}

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
		default:
			var additionalProperty RefStressObjectAllOf2AllOf1AllOf2MetadataAdditionalProperty
			err = json.Unmarshal(value, &additionalProperty)
			if err != nil {
				return err
			}
		}
	}
	if _, err := d.Token(); err != nil {
		return err
	}
	if len(bytes.TrimSpace(data[d.InputOffset():])) != 0 {
		return NotAnObjectError
	}

	return nil
}

type RefStressObjectAllOf2AllOf1AllOf2MetadataAdditionalProperty string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf2MetadataAdditionalProperty)

func (s *RefStressObjectAllOf2AllOf1AllOf2MetadataAdditionalProperty) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf2AllOf1AllOf2MetadataAdditionalProperty(value)
	return nil
}

type RefStressObjectAllOf2AllOf1AllOf2RootFlag bool

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf2RootFlag)

func (b *RefStressObjectAllOf2AllOf1AllOf2RootFlag) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectAllOf2AllOf1AllOf2RootFlag(value)
	return nil
}

type RefStressObjectAllOf2AllOf2 struct {
	Count      RefStressObjectAllOf2AllOf2Count       `json:"count"`
	Finals     RefStressObjectAllOf2AllOf2Finals      `json:"finals"`
	Metadata   RefStressObjectAllOf2AllOf2Metadata    `json:"metadata"`
	RootFlag   RefStressObjectAllOf2AllOf2RootFlag    `json:"rootFlag"`
	SharedName *RefStressObjectAllOf2AllOf2SharedName `json:"sharedName,omitzero"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf2AllOf2)(nil)

func (o *RefStressObjectAllOf2AllOf2) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasCount bool
	var hasFinals bool
	var hasMetadata bool
	var hasRootFlag bool

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
		case "count":
			hasCount = true

			var count RefStressObjectAllOf2AllOf2Count
			err = json.Unmarshal(value, &count)
			if err != nil {
				return err
			}
			o.Count = count
		case "finals":
			hasFinals = true

			var finals RefStressObjectAllOf2AllOf2Finals
			err = json.Unmarshal(value, &finals)
			if err != nil {
				return err
			}
			o.Finals = finals
		case "metadata":
			hasMetadata = true

			var metadata RefStressObjectAllOf2AllOf2Metadata
			err = json.Unmarshal(value, &metadata)
			if err != nil {
				return err
			}
			o.Metadata = metadata
		case "rootFlag":
			hasRootFlag = true

			var rootFlag RefStressObjectAllOf2AllOf2RootFlag
			err = json.Unmarshal(value, &rootFlag)
			if err != nil {
				return err
			}
			o.RootFlag = rootFlag
		case "sharedName":

			var sharedName RefStressObjectAllOf2AllOf2SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = &sharedName
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
	if !hasCount {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "count")
	}
	if !hasFinals {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finals")
	}
	if !hasMetadata {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "metadata")
	}
	if !hasRootFlag {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "rootFlag")
	}

	return nil
}

type RefStressObjectAllOf2AllOf2Count json.Number

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2Count)

func (n *RefStressObjectAllOf2AllOf2Count) UnmarshalJSON(data []byte) error {
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
	*n = RefStressObjectAllOf2AllOf2Count(value)
	return nil
}

type RefStressObjectAllOf2AllOf2Finals []RefStressObjectAllOf2AllOf2FinalsItem

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2Finals)

func (a *RefStressObjectAllOf2AllOf2Finals) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return fmt.Errorf("null for not nullable array")
	}

	var value []RefStressObjectAllOf2AllOf2FinalsItem
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	*a = RefStressObjectAllOf2AllOf2Finals(value)
	return nil
}

type RefStressObjectAllOf2AllOf2FinalsItem struct {
	FinalCode      RefStressObjectAllOf2AllOf2FinalsItemFinalCode       `json:"finalCode"`
	Nested         *RefStressObjectAllOf2AllOf2FinalsItemNested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectAllOf2AllOf2FinalsItemOptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectAllOf2AllOf2FinalsItemSharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf2AllOf2FinalsItem)(nil)

func (o *RefStressObjectAllOf2AllOf2FinalsItem) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectAllOf2AllOf2FinalsItemFinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectAllOf2AllOf2FinalsItemNested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectAllOf2AllOf2FinalsItemOptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectAllOf2AllOf2FinalsItemSharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectAllOf2AllOf2FinalsItemFinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2FinalsItemFinalCode)

func (s *RefStressObjectAllOf2AllOf2FinalsItemFinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf2AllOf2FinalsItemFinalCode(value)
	return nil
}

type RefStressObjectAllOf2AllOf2FinalsItemNested struct {
	Leaf     *RefStressObjectAllOf2AllOf2FinalsItemNestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectAllOf2AllOf2FinalsItemNestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf2AllOf2FinalsItemNested)(nil)

func (o *RefStressObjectAllOf2AllOf2FinalsItemNested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectAllOf2AllOf2FinalsItemNestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf2AllOf2FinalsItemNestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectAllOf2AllOf2FinalsItemNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2FinalsItemNestedLeaf)

func (s *RefStressObjectAllOf2AllOf2FinalsItemNestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf2AllOf2FinalsItemNestedLeaf(value)
	return nil
}

type RefStressObjectAllOf2AllOf2FinalsItemNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2FinalsItemNestedSameName)

func (s *RefStressObjectAllOf2AllOf2FinalsItemNestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf2AllOf2FinalsItemOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2FinalsItemOptionalShared)

func (s *RefStressObjectAllOf2AllOf2FinalsItemOptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf2AllOf2FinalsItemSharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2FinalsItemSharedName)

func (s *RefStressObjectAllOf2AllOf2FinalsItemSharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf2AllOf2FinalsItemSharedName(value)
	return nil
}

type RefStressObjectAllOf2AllOf2Metadata struct {
}

var _ json.Unmarshaler = (*RefStressObjectAllOf2AllOf2Metadata)(nil)

func (o *RefStressObjectAllOf2AllOf2Metadata) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}

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
		default:
			var additionalProperty RefStressObjectAllOf2AllOf2MetadataAdditionalProperty
			err = json.Unmarshal(value, &additionalProperty)
			if err != nil {
				return err
			}
		}
	}
	if _, err := d.Token(); err != nil {
		return err
	}
	if len(bytes.TrimSpace(data[d.InputOffset():])) != 0 {
		return NotAnObjectError
	}

	return nil
}

type RefStressObjectAllOf2AllOf2MetadataAdditionalProperty string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2MetadataAdditionalProperty)

func (s *RefStressObjectAllOf2AllOf2MetadataAdditionalProperty) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf2AllOf2MetadataAdditionalProperty(value)
	return nil
}

type RefStressObjectAllOf2AllOf2RootFlag bool

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2RootFlag)

func (b *RefStressObjectAllOf2AllOf2RootFlag) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectAllOf2AllOf2RootFlag(value)
	return nil
}

type RefStressObjectAllOf2AllOf2SharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2SharedName)

func (s *RefStressObjectAllOf2AllOf2SharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf2AllOf2SharedName(value)
	return nil
}

type RefStressObjectAllOf3 struct {
	Count            RefStressObjectAllOf3Count            `json:"count"`
	Final            RefStressObjectAllOf3Final            `json:"final"`
	FinalCode        RefStressObjectAllOf3FinalCode        `json:"finalCode"`
	Finals           RefStressObjectAllOf3Finals           `json:"finals"`
	Metadata         RefStressObjectAllOf3Metadata         `json:"metadata"`
	MiddleFlag       RefStressObjectAllOf3MiddleFlag       `json:"middleFlag"`
	Nested           RefStressObjectAllOf3Nested           `json:"nested"`
	NullableRequired RefStressObjectAllOf3NullableRequired `json:"nullableRequired"`
	OptionalCode     *RefStressObjectAllOf3OptionalCode    `json:"optionalCode,omitzero"`
	OptionalShared   *RefStressObjectAllOf3OptionalShared  `json:"optionalShared,omitzero"`
	RootFlag         RefStressObjectAllOf3RootFlag         `json:"rootFlag"`
	SharedName       RefStressObjectAllOf3SharedName       `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf3)(nil)

func (o *RefStressObjectAllOf3) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasCount bool
	var hasFinal bool
	var hasFinalCode bool
	var hasFinals bool
	var hasMetadata bool
	var hasMiddleFlag bool
	var hasNested bool
	var hasNullableRequired bool
	var hasRootFlag bool
	var hasSharedName bool

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
		case "count":
			hasCount = true

			var count RefStressObjectAllOf3Count
			err = json.Unmarshal(value, &count)
			if err != nil {
				return err
			}
			o.Count = count
		case "final":
			hasFinal = true

			var final RefStressObjectAllOf3Final
			err = json.Unmarshal(value, &final)
			if err != nil {
				return err
			}
			o.Final = final
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectAllOf3FinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "finals":
			hasFinals = true

			var finals RefStressObjectAllOf3Finals
			err = json.Unmarshal(value, &finals)
			if err != nil {
				return err
			}
			o.Finals = finals
		case "metadata":
			hasMetadata = true

			var metadata RefStressObjectAllOf3Metadata
			err = json.Unmarshal(value, &metadata)
			if err != nil {
				return err
			}
			o.Metadata = metadata
		case "middleFlag":
			hasMiddleFlag = true

			var middleFlag RefStressObjectAllOf3MiddleFlag
			err = json.Unmarshal(value, &middleFlag)
			if err != nil {
				return err
			}
			o.MiddleFlag = middleFlag
		case "nested":
			hasNested = true

			var nested RefStressObjectAllOf3Nested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = nested
		case "nullableRequired":
			hasNullableRequired = true

			var nullableRequired RefStressObjectAllOf3NullableRequired
			err = json.Unmarshal(value, &nullableRequired)
			if err != nil {
				return err
			}
			o.NullableRequired = nullableRequired
		case "optionalCode":

			var optionalCode RefStressObjectAllOf3OptionalCode
			err = json.Unmarshal(value, &optionalCode)
			if err != nil {
				return err
			}
			o.OptionalCode = &optionalCode
		case "optionalShared":

			var optionalShared RefStressObjectAllOf3OptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "rootFlag":
			hasRootFlag = true

			var rootFlag RefStressObjectAllOf3RootFlag
			err = json.Unmarshal(value, &rootFlag)
			if err != nil {
				return err
			}
			o.RootFlag = rootFlag
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectAllOf3SharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasCount {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "count")
	}
	if !hasFinal {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "final")
	}
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasFinals {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finals")
	}
	if !hasMetadata {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "metadata")
	}
	if !hasMiddleFlag {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "middleFlag")
	}
	if !hasNested {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "nested")
	}
	if !hasNullableRequired {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "nullableRequired")
	}
	if !hasRootFlag {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "rootFlag")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectAllOf3Count json.Number

var _ json.Unmarshaler = new(RefStressObjectAllOf3Count)

func (n *RefStressObjectAllOf3Count) UnmarshalJSON(data []byte) error {
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
	*n = RefStressObjectAllOf3Count(value)
	return nil
}

type RefStressObjectAllOf3Final struct {
	FinalCode      RefStressObjectAllOf3FinalFinalCode       `json:"finalCode"`
	Nested         *RefStressObjectAllOf3FinalNested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectAllOf3FinalOptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectAllOf3FinalSharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf3Final)(nil)

func (o *RefStressObjectAllOf3Final) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectAllOf3FinalFinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectAllOf3FinalNested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectAllOf3FinalOptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectAllOf3FinalSharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectAllOf3FinalFinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalFinalCode)

func (s *RefStressObjectAllOf3FinalFinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf3FinalFinalCode(value)
	return nil
}

type RefStressObjectAllOf3FinalNested struct {
	Leaf     *RefStressObjectAllOf3FinalNestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectAllOf3FinalNestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf3FinalNested)(nil)

func (o *RefStressObjectAllOf3FinalNested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectAllOf3FinalNestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf3FinalNestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectAllOf3FinalNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalNestedLeaf)

func (s *RefStressObjectAllOf3FinalNestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf3FinalNestedLeaf(value)
	return nil
}

type RefStressObjectAllOf3FinalNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalNestedSameName)

func (s *RefStressObjectAllOf3FinalNestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf3FinalOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalOptionalShared)

func (s *RefStressObjectAllOf3FinalOptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf3FinalSharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalSharedName)

func (s *RefStressObjectAllOf3FinalSharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf3FinalSharedName(value)
	return nil
}

type RefStressObjectAllOf3FinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalCode)

func (s *RefStressObjectAllOf3FinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf3FinalCode(value)
	return nil
}

type RefStressObjectAllOf3Finals []RefStressObjectAllOf3FinalsItem

var _ json.Unmarshaler = new(RefStressObjectAllOf3Finals)

func (a *RefStressObjectAllOf3Finals) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return fmt.Errorf("null for not nullable array")
	}

	var value []RefStressObjectAllOf3FinalsItem
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	*a = RefStressObjectAllOf3Finals(value)
	return nil
}

type RefStressObjectAllOf3FinalsItem struct {
	FinalCode      RefStressObjectAllOf3FinalsItemFinalCode       `json:"finalCode"`
	Nested         *RefStressObjectAllOf3FinalsItemNested         `json:"nested,omitzero"`
	OptionalShared *RefStressObjectAllOf3FinalsItemOptionalShared `json:"optionalShared,omitzero"`
	SharedName     RefStressObjectAllOf3FinalsItemSharedName      `json:"sharedName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf3FinalsItem)(nil)

func (o *RefStressObjectAllOf3FinalsItem) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasFinalCode bool
	var hasSharedName bool

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
		case "finalCode":
			hasFinalCode = true

			var finalCode RefStressObjectAllOf3FinalsItemFinalCode
			err = json.Unmarshal(value, &finalCode)
			if err != nil {
				return err
			}
			o.FinalCode = finalCode
		case "nested":

			var nested RefStressObjectAllOf3FinalsItemNested
			err = json.Unmarshal(value, &nested)
			if err != nil {
				return err
			}
			o.Nested = &nested
		case "optionalShared":

			var optionalShared RefStressObjectAllOf3FinalsItemOptionalShared
			err = json.Unmarshal(value, &optionalShared)
			if err != nil {
				return err
			}
			o.OptionalShared = &optionalShared
		case "sharedName":
			hasSharedName = true

			var sharedName RefStressObjectAllOf3FinalsItemSharedName
			err = json.Unmarshal(value, &sharedName)
			if err != nil {
				return err
			}
			o.SharedName = sharedName
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
	if !hasFinalCode {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "finalCode")
	}
	if !hasSharedName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sharedName")
	}

	return nil
}

type RefStressObjectAllOf3FinalsItemFinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalsItemFinalCode)

func (s *RefStressObjectAllOf3FinalsItemFinalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf3FinalsItemFinalCode(value)
	return nil
}

type RefStressObjectAllOf3FinalsItemNested struct {
	Leaf     *RefStressObjectAllOf3FinalsItemNestedLeaf    `json:"leaf,omitzero"`
	SameName RefStressObjectAllOf3FinalsItemNestedSameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf3FinalsItemNested)(nil)

func (o *RefStressObjectAllOf3FinalsItemNested) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectAllOf3FinalsItemNestedLeaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf3FinalsItemNestedSameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectAllOf3FinalsItemNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalsItemNestedLeaf)

func (s *RefStressObjectAllOf3FinalsItemNestedLeaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf3FinalsItemNestedLeaf(value)
	return nil
}

type RefStressObjectAllOf3FinalsItemNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalsItemNestedSameName)

func (s *RefStressObjectAllOf3FinalsItemNestedSameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf3FinalsItemOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalsItemOptionalShared)

func (s *RefStressObjectAllOf3FinalsItemOptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf3FinalsItemSharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalsItemSharedName)

func (s *RefStressObjectAllOf3FinalsItemSharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf3FinalsItemSharedName(value)
	return nil
}

type RefStressObjectAllOf3Metadata struct {
}

var _ json.Unmarshaler = (*RefStressObjectAllOf3Metadata)(nil)

func (o *RefStressObjectAllOf3Metadata) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}

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
		default:
			var additionalProperty RefStressObjectAllOf3MetadataAdditionalProperty
			err = json.Unmarshal(value, &additionalProperty)
			if err != nil {
				return err
			}
		}
	}
	if _, err := d.Token(); err != nil {
		return err
	}
	if len(bytes.TrimSpace(data[d.InputOffset():])) != 0 {
		return NotAnObjectError
	}

	return nil
}

type RefStressObjectAllOf3MetadataAdditionalProperty string

var _ json.Unmarshaler = new(RefStressObjectAllOf3MetadataAdditionalProperty)

func (s *RefStressObjectAllOf3MetadataAdditionalProperty) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf3MetadataAdditionalProperty(value)
	return nil
}

type RefStressObjectAllOf3MiddleFlag bool

var _ json.Unmarshaler = new(RefStressObjectAllOf3MiddleFlag)

func (b *RefStressObjectAllOf3MiddleFlag) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectAllOf3MiddleFlag(value)
	return nil
}

type RefStressObjectAllOf3Nested struct {
	RefStressObjectAllOf3NestedAllOf1
	RefStressObjectAllOf3NestedAllOf2
	RefStressObjectAllOf3NestedAllOf3
}

var _ json.Unmarshaler = (*RefStressObjectAllOf3Nested)(nil)

func (a *RefStressObjectAllOf3Nested) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.RefStressObjectAllOf3NestedAllOf1.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectAllOf3NestedAllOf2.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}
	if err := a.RefStressObjectAllOf3NestedAllOf3.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type RefStressObjectAllOf3NestedAllOf1 struct {
	Leaf     *RefStressObjectAllOf3NestedAllOf1Leaf    `json:"leaf,omitzero"`
	SameName RefStressObjectAllOf3NestedAllOf1SameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf3NestedAllOf1)(nil)

func (o *RefStressObjectAllOf3NestedAllOf1) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectAllOf3NestedAllOf1Leaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf3NestedAllOf1SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectAllOf3NestedAllOf1Leaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf3NestedAllOf1Leaf)

func (s *RefStressObjectAllOf3NestedAllOf1Leaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf3NestedAllOf1Leaf(value)
	return nil
}

type RefStressObjectAllOf3NestedAllOf1SameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf3NestedAllOf1SameName)

func (s *RefStressObjectAllOf3NestedAllOf1SameName) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf3NestedAllOf2 struct {
	Leaf     *RefStressObjectAllOf3NestedAllOf2Leaf    `json:"leaf,omitzero"`
	SameName RefStressObjectAllOf3NestedAllOf2SameName `json:"sameName"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf3NestedAllOf2)(nil)

func (o *RefStressObjectAllOf3NestedAllOf2) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool

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
		case "leaf":

			var leaf RefStressObjectAllOf3NestedAllOf2Leaf
			err = json.Unmarshal(value, &leaf)
			if err != nil {
				return err
			}
			o.Leaf = &leaf
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf3NestedAllOf2SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}

	return nil
}

type RefStressObjectAllOf3NestedAllOf2Leaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf3NestedAllOf2Leaf)

func (s *RefStressObjectAllOf3NestedAllOf2Leaf) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf3NestedAllOf2Leaf(value)
	return nil
}

type RefStressObjectAllOf3NestedAllOf2SameName string

var _ json.Unmarshaler = new(RefStressObjectAllOf3NestedAllOf2SameName)

func (s *RefStressObjectAllOf3NestedAllOf2SameName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf3NestedAllOf2SameName(value)
	return nil
}

type RefStressObjectAllOf3NestedAllOf3 struct {
	SameName RefStressObjectAllOf3NestedAllOf3SameName `json:"sameName"`
	Sealed   RefStressObjectAllOf3NestedAllOf3Sealed   `json:"sealed"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf3NestedAllOf3)(nil)

func (o *RefStressObjectAllOf3NestedAllOf3) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasSameName bool
	var hasSealed bool

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
		case "sameName":
			hasSameName = true

			var sameName RefStressObjectAllOf3NestedAllOf3SameName
			err = json.Unmarshal(value, &sameName)
			if err != nil {
				return err
			}
			o.SameName = sameName
		case "sealed":
			hasSealed = true

			var sealed RefStressObjectAllOf3NestedAllOf3Sealed
			err = json.Unmarshal(value, &sealed)
			if err != nil {
				return err
			}
			o.Sealed = sealed
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
	if !hasSameName {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sameName")
	}
	if !hasSealed {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "sealed")
	}

	return nil
}

type RefStressObjectAllOf3NestedAllOf3SameName string

var _ json.Unmarshaler = new(RefStressObjectAllOf3NestedAllOf3SameName)

func (s *RefStressObjectAllOf3NestedAllOf3SameName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf3NestedAllOf3SameName(value)
	return nil
}

type RefStressObjectAllOf3NestedAllOf3Sealed struct {
	Locked RefStressObjectAllOf3NestedAllOf3SealedLocked `json:"locked"`
}

var _ json.Unmarshaler = (*RefStressObjectAllOf3NestedAllOf3Sealed)(nil)

func (o *RefStressObjectAllOf3NestedAllOf3Sealed) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasLocked bool

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
		case "locked":
			hasLocked = true

			var locked RefStressObjectAllOf3NestedAllOf3SealedLocked
			err = json.Unmarshal(value, &locked)
			if err != nil {
				return err
			}
			o.Locked = locked
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
	if !hasLocked {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "locked")
	}

	return nil
}

type RefStressObjectAllOf3NestedAllOf3SealedLocked bool

var _ json.Unmarshaler = new(RefStressObjectAllOf3NestedAllOf3SealedLocked)

func (b *RefStressObjectAllOf3NestedAllOf3SealedLocked) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectAllOf3NestedAllOf3SealedLocked(value)
	return nil
}

type RefStressObjectAllOf3NullableRequired struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf3NullableRequired)

func (s *RefStressObjectAllOf3NullableRequired) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf3OptionalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf3OptionalCode)

func (s *RefStressObjectAllOf3OptionalCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf3OptionalCode(value)
	return nil
}

type RefStressObjectAllOf3OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf3OptionalShared)

func (s *RefStressObjectAllOf3OptionalShared) UnmarshalJSON(data []byte) error {
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

type RefStressObjectAllOf3RootFlag bool

var _ json.Unmarshaler = new(RefStressObjectAllOf3RootFlag)

func (b *RefStressObjectAllOf3RootFlag) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = RefStressObjectAllOf3RootFlag(value)
	return nil
}

type RefStressObjectAllOf3SharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf3SharedName)

func (s *RefStressObjectAllOf3SharedName) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefStressObjectAllOf3SharedName(value)
	return nil
}

type RefObject struct {
	RefOptionalBool   *RefObjectRefOptionalBool  `json:"refOptionalBool,omitzero"`
	RefRequiredString RefObjectRefRequiredString `json:"refRequiredString"`
}

var _ json.Unmarshaler = (*RefObject)(nil)

func (o *RefObject) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasRefRequiredString bool

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
		case "refOptionalBool":

			var refOptionalBool RefObjectRefOptionalBool
			err = json.Unmarshal(value, &refOptionalBool)
			if err != nil {
				return err
			}
			o.RefOptionalBool = &refOptionalBool
		case "refRequiredString":
			hasRefRequiredString = true

			var refRequiredString RefObjectRefRequiredString
			err = json.Unmarshal(value, &refRequiredString)
			if err != nil {
				return err
			}
			o.RefRequiredString = refRequiredString
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
	if !hasRefRequiredString {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "refRequiredString")
	}

	return nil
}

type RefObjectRefOptionalBool struct {
	Value *bool
}

var _ json.Unmarshaler = new(RefObjectRefOptionalBool)

func (b *RefObjectRefOptionalBool) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		b.Value = nil
		return nil
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	b.Value = new(value)
	return nil
}

type RefObjectRefRequiredString string

var _ json.Unmarshaler = new(RefObjectRefRequiredString)

func (s *RefObjectRefRequiredString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = RefObjectRefRequiredString(value)
	return nil
}

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

type NullableObjectKeysAdditionalPropertiesFalse struct {
	OptionalNotNullableString *NullableObjectKeysAdditionalPropertiesFalseOptionalNotNullableString `json:"optionalNotNullableString,omitzero"`
	OptionalNullableString    *NullableObjectKeysAdditionalPropertiesFalseOptionalNullableString    `json:"optionalNullableString,omitzero"`
	RequiredNotNullableString NullableObjectKeysAdditionalPropertiesFalseRequiredNotNullableString  `json:"requiredNotNullableString"`
	RequiredNullableString    NullableObjectKeysAdditionalPropertiesFalseRequiredNullableString     `json:"requiredNullableString"`
}

var _ json.Unmarshaler = (*NullableObjectKeysAdditionalPropertiesFalse)(nil)

func (o *NullableObjectKeysAdditionalPropertiesFalse) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
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

			var optionalNotNullableString NullableObjectKeysAdditionalPropertiesFalseOptionalNotNullableString
			err = json.Unmarshal(value, &optionalNotNullableString)
			if err != nil {
				return err
			}
			o.OptionalNotNullableString = &optionalNotNullableString
		case "optionalNullableString":

			var optionalNullableString NullableObjectKeysAdditionalPropertiesFalseOptionalNullableString
			err = json.Unmarshal(value, &optionalNullableString)
			if err != nil {
				return err
			}
			o.OptionalNullableString = &optionalNullableString
		case "requiredNotNullableString":
			hasRequiredNotNullableString = true

			var requiredNotNullableString NullableObjectKeysAdditionalPropertiesFalseRequiredNotNullableString
			err = json.Unmarshal(value, &requiredNotNullableString)
			if err != nil {
				return err
			}
			o.RequiredNotNullableString = requiredNotNullableString
		case "requiredNullableString":
			hasRequiredNullableString = true

			var requiredNullableString NullableObjectKeysAdditionalPropertiesFalseRequiredNullableString
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

type NullableObjectKeysAdditionalPropertiesFalseOptionalNotNullableString string

var _ json.Unmarshaler = new(NullableObjectKeysAdditionalPropertiesFalseOptionalNotNullableString)

func (s *NullableObjectKeysAdditionalPropertiesFalseOptionalNotNullableString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = NullableObjectKeysAdditionalPropertiesFalseOptionalNotNullableString(value)
	return nil
}

type NullableObjectKeysAdditionalPropertiesFalseOptionalNullableString struct {
	Value *string
}

var _ json.Unmarshaler = new(NullableObjectKeysAdditionalPropertiesFalseOptionalNullableString)

func (s *NullableObjectKeysAdditionalPropertiesFalseOptionalNullableString) UnmarshalJSON(data []byte) error {
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

type NullableObjectKeysAdditionalPropertiesFalseRequiredNotNullableString string

var _ json.Unmarshaler = new(NullableObjectKeysAdditionalPropertiesFalseRequiredNotNullableString)

func (s *NullableObjectKeysAdditionalPropertiesFalseRequiredNotNullableString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = NullableObjectKeysAdditionalPropertiesFalseRequiredNotNullableString(value)
	return nil
}

type NullableObjectKeysAdditionalPropertiesFalseRequiredNullableString struct {
	Value *string
}

var _ json.Unmarshaler = new(NullableObjectKeysAdditionalPropertiesFalseRequiredNullableString)

func (s *NullableObjectKeysAdditionalPropertiesFalseRequiredNullableString) UnmarshalJSON(data []byte) error {
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

type CompositeObject struct {
	ArrayNotNullableItemsNotNullable   CompositeObjectArrayNotNullableItemsNotNullable   `json:"arrayNotNullableItemsNotNullable"`
	ArrayNotNullableItemsNullable      CompositeObjectArrayNotNullableItemsNullable      `json:"arrayNotNullableItemsNullable"`
	ArrayNullableItemsNotNullable      CompositeObjectArrayNullableItemsNotNullable      `json:"arrayNullableItemsNotNullable"`
	ArrayNullableItemsNullable         CompositeObjectArrayNullableItemsNullable         `json:"arrayNullableItemsNullable"`
	BoolNotNullable                    CompositeObjectBoolNotNullable                    `json:"boolNotNullable"`
	BoolNullable                       CompositeObjectBoolNullable                       `json:"boolNullable"`
	NumberNotNullable                  CompositeObjectNumberNotNullable                  `json:"numberNotNullable"`
	NumberNullable                     CompositeObjectNumberNullable                     `json:"numberNullable"`
	ObjectAdditionalPropertiesImplicit CompositeObjectObjectAdditionalPropertiesImplicit `json:"objectAdditionalPropertiesImplicit"`
	ObjectAdditionalPropertiesSchema   CompositeObjectObjectAdditionalPropertiesSchema   `json:"objectAdditionalPropertiesSchema"`
	ObjectAdditionalPropertiesTrue     CompositeObjectObjectAdditionalPropertiesTrue     `json:"objectAdditionalPropertiesTrue"`
	StringFormatNotNullable            CompositeObjectStringFormatNotNullable            `json:"stringFormatNotNullable"`
	StringFormatNullable               CompositeObjectStringFormatNullable               `json:"stringFormatNullable"`
}

var _ json.Unmarshaler = (*CompositeObject)(nil)

func (o *CompositeObject) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasArrayNotNullableItemsNotNullable bool
	var hasArrayNotNullableItemsNullable bool
	var hasArrayNullableItemsNotNullable bool
	var hasArrayNullableItemsNullable bool
	var hasBoolNotNullable bool
	var hasBoolNullable bool
	var hasNumberNotNullable bool
	var hasNumberNullable bool
	var hasObjectAdditionalPropertiesImplicit bool
	var hasObjectAdditionalPropertiesSchema bool
	var hasObjectAdditionalPropertiesTrue bool
	var hasStringFormatNotNullable bool
	var hasStringFormatNullable bool

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
		case "arrayNotNullableItemsNotNullable":
			hasArrayNotNullableItemsNotNullable = true

			var arrayNotNullableItemsNotNullable CompositeObjectArrayNotNullableItemsNotNullable
			err = json.Unmarshal(value, &arrayNotNullableItemsNotNullable)
			if err != nil {
				return err
			}
			o.ArrayNotNullableItemsNotNullable = arrayNotNullableItemsNotNullable
		case "arrayNotNullableItemsNullable":
			hasArrayNotNullableItemsNullable = true

			var arrayNotNullableItemsNullable CompositeObjectArrayNotNullableItemsNullable
			err = json.Unmarshal(value, &arrayNotNullableItemsNullable)
			if err != nil {
				return err
			}
			o.ArrayNotNullableItemsNullable = arrayNotNullableItemsNullable
		case "arrayNullableItemsNotNullable":
			hasArrayNullableItemsNotNullable = true

			var arrayNullableItemsNotNullable CompositeObjectArrayNullableItemsNotNullable
			err = json.Unmarshal(value, &arrayNullableItemsNotNullable)
			if err != nil {
				return err
			}
			o.ArrayNullableItemsNotNullable = arrayNullableItemsNotNullable
		case "arrayNullableItemsNullable":
			hasArrayNullableItemsNullable = true

			var arrayNullableItemsNullable CompositeObjectArrayNullableItemsNullable
			err = json.Unmarshal(value, &arrayNullableItemsNullable)
			if err != nil {
				return err
			}
			o.ArrayNullableItemsNullable = arrayNullableItemsNullable
		case "boolNotNullable":
			hasBoolNotNullable = true

			var boolNotNullable CompositeObjectBoolNotNullable
			err = json.Unmarshal(value, &boolNotNullable)
			if err != nil {
				return err
			}
			o.BoolNotNullable = boolNotNullable
		case "boolNullable":
			hasBoolNullable = true

			var boolNullable CompositeObjectBoolNullable
			err = json.Unmarshal(value, &boolNullable)
			if err != nil {
				return err
			}
			o.BoolNullable = boolNullable
		case "numberNotNullable":
			hasNumberNotNullable = true

			var numberNotNullable CompositeObjectNumberNotNullable
			err = json.Unmarshal(value, &numberNotNullable)
			if err != nil {
				return err
			}
			o.NumberNotNullable = numberNotNullable
		case "numberNullable":
			hasNumberNullable = true

			var numberNullable CompositeObjectNumberNullable
			err = json.Unmarshal(value, &numberNullable)
			if err != nil {
				return err
			}
			o.NumberNullable = numberNullable
		case "objectAdditionalPropertiesImplicit":
			hasObjectAdditionalPropertiesImplicit = true

			var objectAdditionalPropertiesImplicit CompositeObjectObjectAdditionalPropertiesImplicit
			err = json.Unmarshal(value, &objectAdditionalPropertiesImplicit)
			if err != nil {
				return err
			}
			o.ObjectAdditionalPropertiesImplicit = objectAdditionalPropertiesImplicit
		case "objectAdditionalPropertiesSchema":
			hasObjectAdditionalPropertiesSchema = true

			var objectAdditionalPropertiesSchema CompositeObjectObjectAdditionalPropertiesSchema
			err = json.Unmarshal(value, &objectAdditionalPropertiesSchema)
			if err != nil {
				return err
			}
			o.ObjectAdditionalPropertiesSchema = objectAdditionalPropertiesSchema
		case "objectAdditionalPropertiesTrue":
			hasObjectAdditionalPropertiesTrue = true

			var objectAdditionalPropertiesTrue CompositeObjectObjectAdditionalPropertiesTrue
			err = json.Unmarshal(value, &objectAdditionalPropertiesTrue)
			if err != nil {
				return err
			}
			o.ObjectAdditionalPropertiesTrue = objectAdditionalPropertiesTrue
		case "stringFormatNotNullable":
			hasStringFormatNotNullable = true

			var stringFormatNotNullable CompositeObjectStringFormatNotNullable
			err = json.Unmarshal(value, &stringFormatNotNullable)
			if err != nil {
				return err
			}
			o.StringFormatNotNullable = stringFormatNotNullable
		case "stringFormatNullable":
			hasStringFormatNullable = true

			var stringFormatNullable CompositeObjectStringFormatNullable
			err = json.Unmarshal(value, &stringFormatNullable)
			if err != nil {
				return err
			}
			o.StringFormatNullable = stringFormatNullable
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
	if !hasArrayNotNullableItemsNotNullable {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "arrayNotNullableItemsNotNullable")
	}
	if !hasArrayNotNullableItemsNullable {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "arrayNotNullableItemsNullable")
	}
	if !hasArrayNullableItemsNotNullable {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "arrayNullableItemsNotNullable")
	}
	if !hasArrayNullableItemsNullable {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "arrayNullableItemsNullable")
	}
	if !hasBoolNotNullable {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "boolNotNullable")
	}
	if !hasBoolNullable {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "boolNullable")
	}
	if !hasNumberNotNullable {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "numberNotNullable")
	}
	if !hasNumberNullable {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "numberNullable")
	}
	if !hasObjectAdditionalPropertiesImplicit {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "objectAdditionalPropertiesImplicit")
	}
	if !hasObjectAdditionalPropertiesSchema {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "objectAdditionalPropertiesSchema")
	}
	if !hasObjectAdditionalPropertiesTrue {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "objectAdditionalPropertiesTrue")
	}
	if !hasStringFormatNotNullable {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "stringFormatNotNullable")
	}
	if !hasStringFormatNullable {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "stringFormatNullable")
	}

	return nil
}

type CompositeObjectArrayNotNullableItemsNotNullable []CompositeObjectArrayNotNullableItemsNotNullableItem

var _ json.Unmarshaler = new(CompositeObjectArrayNotNullableItemsNotNullable)

func (a *CompositeObjectArrayNotNullableItemsNotNullable) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return fmt.Errorf("null for not nullable array")
	}

	var value []CompositeObjectArrayNotNullableItemsNotNullableItem
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	*a = CompositeObjectArrayNotNullableItemsNotNullable(value)
	return nil
}

type CompositeObjectArrayNotNullableItemsNotNullableItem string

var _ json.Unmarshaler = new(CompositeObjectArrayNotNullableItemsNotNullableItem)

func (s *CompositeObjectArrayNotNullableItemsNotNullableItem) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = CompositeObjectArrayNotNullableItemsNotNullableItem(value)
	return nil
}

type CompositeObjectArrayNotNullableItemsNullable []CompositeObjectArrayNotNullableItemsNullableItem

var _ json.Unmarshaler = new(CompositeObjectArrayNotNullableItemsNullable)

func (a *CompositeObjectArrayNotNullableItemsNullable) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return fmt.Errorf("null for not nullable array")
	}

	var value []CompositeObjectArrayNotNullableItemsNullableItem
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	*a = CompositeObjectArrayNotNullableItemsNullable(value)
	return nil
}

type CompositeObjectArrayNotNullableItemsNullableItem struct {
	Value *string
}

var _ json.Unmarshaler = new(CompositeObjectArrayNotNullableItemsNullableItem)

func (s *CompositeObjectArrayNotNullableItemsNullableItem) UnmarshalJSON(data []byte) error {
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

type CompositeObjectArrayNullableItemsNotNullable struct {
	Value []CompositeObjectArrayNullableItemsNotNullableItem
}

var _ json.Unmarshaler = new(CompositeObjectArrayNullableItemsNotNullable)

func (a *CompositeObjectArrayNullableItemsNotNullable) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		a.Value = nil
		return nil
	}

	var value []CompositeObjectArrayNullableItemsNotNullableItem
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	a.Value = value
	return nil
}

type CompositeObjectArrayNullableItemsNotNullableItem string

var _ json.Unmarshaler = new(CompositeObjectArrayNullableItemsNotNullableItem)

func (s *CompositeObjectArrayNullableItemsNotNullableItem) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = CompositeObjectArrayNullableItemsNotNullableItem(value)
	return nil
}

type CompositeObjectArrayNullableItemsNullable struct {
	Value []CompositeObjectArrayNullableItemsNullableItem
}

var _ json.Unmarshaler = new(CompositeObjectArrayNullableItemsNullable)

func (a *CompositeObjectArrayNullableItemsNullable) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		a.Value = nil
		return nil
	}

	var value []CompositeObjectArrayNullableItemsNullableItem
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	a.Value = value
	return nil
}

type CompositeObjectArrayNullableItemsNullableItem struct {
	Value *string
}

var _ json.Unmarshaler = new(CompositeObjectArrayNullableItemsNullableItem)

func (s *CompositeObjectArrayNullableItemsNullableItem) UnmarshalJSON(data []byte) error {
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

type CompositeObjectBoolNotNullable bool

var _ json.Unmarshaler = new(CompositeObjectBoolNotNullable)

func (b *CompositeObjectBoolNotNullable) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableBoolError
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	*b = CompositeObjectBoolNotNullable(value)
	return nil
}

type CompositeObjectBoolNullable struct {
	Value *bool
}

var _ json.Unmarshaler = new(CompositeObjectBoolNullable)

func (b *CompositeObjectBoolNullable) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		b.Value = nil
		return nil
	}

	var value bool
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonBoolForBoolSchemaError
	}
	b.Value = new(value)
	return nil
}

type CompositeObjectNumberNotNullable json.Number

var _ json.Unmarshaler = new(CompositeObjectNumberNotNullable)

func (n *CompositeObjectNumberNotNullable) UnmarshalJSON(data []byte) error {
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
	*n = CompositeObjectNumberNotNullable(value)
	return nil
}

type CompositeObjectNumberNullable struct {
	Value *json.Number
}

var _ json.Unmarshaler = new(CompositeObjectNumberNullable)

func (n *CompositeObjectNumberNullable) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, jsonNull) {
		n.Value = nil
		return nil
	}

	if len(trimmed) == 0 || trimmed[0] == '"' {
		return NonNumberForNumberSchemaError
	}

	var value json.Number
	err := json.Unmarshal(trimmed, &value)
	if err != nil {
		return NonNumberForNumberSchemaError
	}
	n.Value = new(value)
	return nil
}

type CompositeObjectObjectAdditionalPropertiesImplicit struct {
	Known *CompositeObjectObjectAdditionalPropertiesImplicitKnown `json:"known,omitzero"`
}

var _ json.Unmarshaler = (*CompositeObjectObjectAdditionalPropertiesImplicit)(nil)

func (o *CompositeObjectObjectAdditionalPropertiesImplicit) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}

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
		case "known":

			var known CompositeObjectObjectAdditionalPropertiesImplicitKnown
			err = json.Unmarshal(value, &known)
			if err != nil {
				return err
			}
			o.Known = &known
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

	return nil
}

type CompositeObjectObjectAdditionalPropertiesImplicitKnown string

var _ json.Unmarshaler = new(CompositeObjectObjectAdditionalPropertiesImplicitKnown)

func (s *CompositeObjectObjectAdditionalPropertiesImplicitKnown) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = CompositeObjectObjectAdditionalPropertiesImplicitKnown(value)
	return nil
}

type CompositeObjectObjectAdditionalPropertiesSchema struct {
	Known *CompositeObjectObjectAdditionalPropertiesSchemaKnown `json:"known,omitzero"`
}

var _ json.Unmarshaler = (*CompositeObjectObjectAdditionalPropertiesSchema)(nil)

func (o *CompositeObjectObjectAdditionalPropertiesSchema) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}

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
		case "known":

			var known CompositeObjectObjectAdditionalPropertiesSchemaKnown
			err = json.Unmarshal(value, &known)
			if err != nil {
				return err
			}
			o.Known = &known
		default:
			var additionalProperty CompositeObjectObjectAdditionalPropertiesSchemaAdditionalProperty
			err = json.Unmarshal(value, &additionalProperty)
			if err != nil {
				return err
			}
		}
	}
	if _, err := d.Token(); err != nil {
		return err
	}
	if len(bytes.TrimSpace(data[d.InputOffset():])) != 0 {
		return NotAnObjectError
	}

	return nil
}

type CompositeObjectObjectAdditionalPropertiesSchemaKnown string

var _ json.Unmarshaler = new(CompositeObjectObjectAdditionalPropertiesSchemaKnown)

func (s *CompositeObjectObjectAdditionalPropertiesSchemaKnown) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = CompositeObjectObjectAdditionalPropertiesSchemaKnown(value)
	return nil
}

type CompositeObjectObjectAdditionalPropertiesSchemaAdditionalProperty string

var _ json.Unmarshaler = new(CompositeObjectObjectAdditionalPropertiesSchemaAdditionalProperty)

func (s *CompositeObjectObjectAdditionalPropertiesSchemaAdditionalProperty) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = CompositeObjectObjectAdditionalPropertiesSchemaAdditionalProperty(value)
	return nil
}

type CompositeObjectObjectAdditionalPropertiesTrue struct {
	Known *CompositeObjectObjectAdditionalPropertiesTrueKnown `json:"known,omitzero"`
}

var _ json.Unmarshaler = (*CompositeObjectObjectAdditionalPropertiesTrue)(nil)

func (o *CompositeObjectObjectAdditionalPropertiesTrue) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}

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
		case "known":

			var known CompositeObjectObjectAdditionalPropertiesTrueKnown
			err = json.Unmarshal(value, &known)
			if err != nil {
				return err
			}
			o.Known = &known
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

	return nil
}

type CompositeObjectObjectAdditionalPropertiesTrueKnown string

var _ json.Unmarshaler = new(CompositeObjectObjectAdditionalPropertiesTrueKnown)

func (s *CompositeObjectObjectAdditionalPropertiesTrueKnown) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	*s = CompositeObjectObjectAdditionalPropertiesTrueKnown(value)
	return nil
}

type CompositeObjectStringFormatNotNullable string

var _ json.Unmarshaler = new(CompositeObjectStringFormatNotNullable)

func (s *CompositeObjectStringFormatNotNullable) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	_, err = time.Parse(time.RFC3339, value)
	if err != nil {
		return err
	}
	*s = CompositeObjectStringFormatNotNullable(value)
	return nil
}

type CompositeObjectStringFormatNullable struct {
	Value *string
}

var _ json.Unmarshaler = new(CompositeObjectStringFormatNullable)

func (s *CompositeObjectStringFormatNullable) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		s.Value = nil
		return nil
	}

	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}
	_, err = time.Parse(time.RFC3339, value)
	if err != nil {
		return err
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
