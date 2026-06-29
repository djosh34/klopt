package example

import (
	"errors"
	"fmt"

	json "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
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

var _ json.UnmarshalerFrom = (*ObjectKeysAdditionalPropertiesFalse)(nil)

func (o *ObjectKeysAdditionalPropertiesFalse) UnmarshalJSONFrom(d *jsontext.Decoder) error {
	tok, err := d.ReadToken()
	if err != nil {
		return err
	}
	if tok.Kind() != jsontext.KindBeginObject {
		return NotAnObjectError
	}

	var hasRequiredNullableString bool
	var hasRequiredNotNullableString bool

	for d.PeekKind() != jsontext.KindEndObject {
		nameTok, err := d.ReadToken()
		if err != nil {
			return err
		}
		if nameTok.Kind() != jsontext.KindString {
			return NotAnObjectError
		}

		switch name := nameTok.String(); name {
		case "requiredNullableString":
			hasRequiredNullableString = true
			if err := json.UnmarshalDecode(d, &o.RequiredNullableString); err != nil {
				return err
			}
		case "requiredNotNullableString":
			hasRequiredNotNullableString = true
			if d.PeekKind() == jsontext.KindNull {
				return NullForNotNullableStringError
			}
			if err := json.UnmarshalDecode(d, &o.RequiredNotNullableString); err != nil {
				return err
			}
		case "optionalNullableString":
			if err := json.UnmarshalDecode(d, &o.OptionalNullableString); err != nil {
				return err
			}
		case "optionalNotNullableString":
			if d.PeekKind() == jsontext.KindNull {
				return NullForNotNullableStringError
			}
			if err := json.UnmarshalDecode(d, &o.OptionalNotNullableString); err != nil {
				return err
			}
		default:
			if err := d.SkipValue(); err != nil {
				return err
			}
			return fmt.Errorf("%w: %s", AdditionalPropertyError, name)
		}
	}

	if _, err := d.ReadToken(); err != nil {
		return err
	}

	if !hasRequiredNullableString {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "requiredNullableString")
	}
	if !hasRequiredNotNullableString {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "requiredNotNullableString")
	}

	return nil
}
