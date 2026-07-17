package validation

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/mail"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

// Validate validates one present or absent raw JSON request body.
func (validation *Validation) Validate(body json.RawMessage) []error {
	if len(body) == 0 {
		if validation.BodyRequired {
			return []error{newValidationError(validation, "#", "requestBody", "required body is absent")}
		}

		return nil
	}

	if _, err := jsonvalue.Parse(body); err != nil {
		return []error{newValidationError(validation, "#", "body", err.Error())}
	}

	return validateRaw(validation, body, "#")
}

// instance retains raw JSON and only the decoded data needed by one keyword family.
type instance struct {
	raw    json.RawMessage
	kind   jsonvalue.Kind
	number jsonvalue.Number
	string string
	array  []json.RawMessage
	object []rawMember
}

// rawMember retains one streamed object name/value pair.
type rawMember struct {
	name string
	raw  json.RawMessage
}

// validateRaw applies one compiled schema node to one raw instance node.
func validateRaw(validation *Validation, raw json.RawMessage, pointer string) []error {
	value, err := decodeInstance(raw)
	if err != nil {
		return []error{newValidationError(validation, pointer, "body", err.Error())}
	}

	errs := validation.KindValidation.validate(validation, value, pointer)
	errs = append(errs, validation.EnumValidation.validate(validation, value, pointer)...)
	errs = append(errs, validation.NumberValidation.validate(validation, value, pointer)...)
	errs = append(errs, validation.StringValidation.validate(validation, value, pointer)...)
	errs = append(errs, validation.ArrayValidation.validate(validation, value, pointer)...)
	errs = append(errs, validation.ObjectValidation.validate(validation, value, pointer)...)

	for _, child := range validation.AllOfValidations {
		errs = append(errs, validateRaw(child, raw, pointer)...)
	}

	return errs
}

// decodeInstance classifies one already-strict raw JSON value.
//
//nolint:cyclop // JSON's six value kinds are clearest as one explicit dispatch.
func decodeInstance(raw json.RawMessage) (instance, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return instance{}, errors.New("JSON value is empty")
	}

	result := instance{raw: append(json.RawMessage(nil), raw...)}

	switch trimmed[0] {
	case 'n':
		result.kind = jsonvalue.KindNull
	case 't', 'f':
		result.kind = jsonvalue.KindBoolean
	case '"':
		result.kind = jsonvalue.KindString
		if err := json.Unmarshal(trimmed, &result.string); err != nil {
			return instance{}, err
		}
	case '[':
		result.kind = jsonvalue.KindArray
		if err := json.Unmarshal(trimmed, &result.array); err != nil {
			return instance{}, err
		}
	case '{':
		result.kind = jsonvalue.KindObject

		members, err := decodeObjectMembers(trimmed)
		if err != nil {
			return instance{}, err
		}

		result.object = members
	default:
		result.kind = jsonvalue.KindNumber

		number, err := jsonvalue.ParseNumber(string(trimmed))
		if err != nil {
			return instance{}, err
		}

		result.number = number
	}

	return result, nil
}

// decodeObjectMembers streams and lexically sorts raw object member values.
func decodeObjectMembers(raw []byte) ([]rawMember, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))

	opening, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	if opening != json.Delim('{') {
		return nil, errors.New("JSON object has no opening delimiter")
	}

	members := make([]rawMember, 0)

	for decoder.More() {
		nameToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}

		name, ok := nameToken.(string)
		if !ok {
			return nil, errors.New("JSON object name must be a string")
		}

		var child json.RawMessage
		if err := decoder.Decode(&child); err != nil {
			return nil, err
		}

		members = append(members, rawMember{name: name, raw: child})
	}

	if _, err := decoder.Token(); err != nil {
		return nil, err
	}

	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("JSON object contains trailing input")
		}

		return nil, err
	}

	sort.Slice(members, func(left int, right int) bool {
		return members[left].name < members[right].name
	})

	return members, nil
}

// validate applies type and nullable constraints.
//
//nolint:cyclop // The explicit kind dispatch mirrors the six JSON value kinds.
func (kind KindValidation) validate(validation *Validation, value instance, pointer string) []error {
	if kind.Type == "" || value.kind == jsonvalue.KindNull && kind.Nullable {
		return nil
	}

	matches := false

	switch kind.Type {
	case "boolean":
		matches = value.kind == jsonvalue.KindBoolean
	case "integer":
		matches = value.kind == jsonvalue.KindNumber && value.number.IsInteger()
	case "number":
		matches = value.kind == jsonvalue.KindNumber
	case "string":
		matches = value.kind == jsonvalue.KindString
	case "array":
		matches = value.kind == jsonvalue.KindArray
	case "object":
		matches = value.kind == jsonvalue.KindObject
	}

	if matches {
		return nil
	}

	return []error{newValidationError(
		validation, pointer, "type", fmt.Sprintf("got %s, want %s", kindName(value.kind), kind.Type),
	)}
}

// validate applies exact semantic enum membership.
func (enum EnumValidation) validate(validation *Validation, value instance, pointer string) []error {
	if len(enum.ExactValues) == 0 {
		return nil
	}

	candidate, err := jsonvalue.Parse(value.raw)
	if err != nil {
		return []error{newValidationError(validation, pointer, "enum", err.Error())}
	}

	for _, allowed := range enum.ExactValues {
		if allowed.Equal(candidate) {
			return nil
		}
	}

	return []error{newValidationError(validation, pointer, "enum", "value is not an allowed member")}
}

// validate applies exact numeric bounds and divisibility.
//
//nolint:cyclop // Each independent numeric keyword must collect its own failure.
func (number NumberValidation) validate(validation *Validation, value instance, pointer string) []error {
	if value.kind != jsonvalue.KindNumber {
		return nil
	}

	var errs []error

	if number.Minimum != nil {
		comparison := value.number.Compare(number.Minimum.ExactValue)
		if comparison < 0 || comparison == 0 && number.Minimum.Exclusive {
			reason := fmt.Sprintf("value must be greater than or equal to %s", number.Minimum.Value)
			keyword := "minimum"

			if number.Minimum.Exclusive {
				reason = fmt.Sprintf("value must be greater than %s", number.Minimum.Value)
				keyword = "exclusiveMinimum"
			}

			errs = append(errs, newValidationError(validation, pointer, keyword, reason))
		}
	}

	if number.Maximum != nil {
		comparison := value.number.Compare(number.Maximum.ExactValue)
		if comparison > 0 || comparison == 0 && number.Maximum.Exclusive {
			reason := fmt.Sprintf("value must be less than or equal to %s", number.Maximum.Value)
			keyword := "maximum"

			if number.Maximum.Exclusive {
				reason = fmt.Sprintf("value must be less than %s", number.Maximum.Value)
				keyword = "exclusiveMaximum"
			}

			errs = append(errs, newValidationError(validation, pointer, keyword, reason))
		}
	}

	if number.ExactMultipleOf != nil && !value.number.IsMultipleOf(*number.ExactMultipleOf) {
		errs = append(errs, newValidationError(
			validation,
			pointer,
			"multipleOf",
			fmt.Sprintf("value must be an exact multiple of %s", number.MultipleOf),
		))
	}

	return errs
}

// validate applies string length, pattern, and format constraints.
func (stringValidation StringValidation) validate(
	validation *Validation,
	value instance,
	pointer string,
) []error {
	if value.kind != jsonvalue.KindString {
		return nil
	}

	var errs []error

	length := utf8.RuneCountInString(value.string)
	if stringValidation.MinLength != nil && compareCount(length, stringValidation.MinLength) < 0 {
		errs = append(errs, newValidationError(validation, pointer, "minLength", fmt.Sprintf(
			"length %d is less than %s", length, stringValidation.MinLength.Value,
		)))
	}

	if stringValidation.MaxLength != nil && compareCount(length, stringValidation.MaxLength) > 0 {
		errs = append(errs, newValidationError(validation, pointer, "maxLength", fmt.Sprintf(
			"length %d is greater than %s", length, stringValidation.MaxLength.Value,
		)))
	}

	if stringValidation.CompiledPattern != nil && !stringValidation.CompiledPattern.Validate(value.string) {
		errs = append(errs, newValidationError(validation, pointer, "pattern", fmt.Sprintf(
			"string does not match %q", stringValidation.Pattern,
		)))
	}

	if !matchesFormat(value.string, stringValidation.Format) {
		errs = append(errs, newValidationError(validation, pointer, "format", fmt.Sprintf(
			"string does not match %q format", stringValidation.Format,
		)))
	}

	return errs
}

// matchesFormat checks the runtime validator's agreed string formats.
func matchesFormat(value string, format string) bool {
	switch format {
	case "", "binary", "password":
		return true
	case "byte":
		_, err := base64.StdEncoding.Strict().DecodeString(value)

		return err == nil
	case "date":
		parsed, err := time.Parse("2006-01-02", value)

		return err == nil && parsed.Format("2006-01-02") == value
	case "date-time":
		_, err := time.Parse(time.RFC3339, value)

		return err == nil
	case "email":
		address, err := mail.ParseAddress(value)

		return err == nil && address.Address == value
	default:
		return true
	}
}

// validate applies array bounds, child schemas, and semantic uniqueness.
//
//nolint:cyclop // Array keywords collect independent failures in fixed order.
func (array ArrayValidation) validate(validation *Validation, value instance, pointer string) []error {
	if value.kind != jsonvalue.KindArray {
		return nil
	}

	var errs []error
	if array.MinItems != nil && compareCount(len(value.array), array.MinItems) < 0 {
		errs = append(errs, newValidationError(validation, pointer, "minItems", fmt.Sprintf(
			"item count %d is less than %s", len(value.array), array.MinItems.Value,
		)))
	}

	if array.MaxItems != nil && compareCount(len(value.array), array.MaxItems) > 0 {
		errs = append(errs, newValidationError(validation, pointer, "maxItems", fmt.Sprintf(
			"item count %d is greater than %s", len(value.array), array.MaxItems.Value,
		)))
	}

	if array.Items != nil {
		for index, child := range value.array {
			errs = append(errs, validateRaw(
				array.Items, child, appendInstancePointer(pointer, fmt.Sprintf("%d", index)),
			)...)
		}
	}

	if array.UniqueItems {
		parsed := make([]jsonvalue.Value, 0, len(value.array))
		for index, child := range value.array {
			candidate, err := jsonvalue.Parse(child)
			if err != nil {
				errs = append(errs, newValidationError(
					validation,
					appendInstancePointer(pointer, fmt.Sprintf("%d", index)), "uniqueItems", err.Error(),
				))

				continue
			}

			for previous, existing := range parsed {
				if existing.Equal(candidate) {
					errs = append(errs, newValidationError(
						validation,
						appendInstancePointer(pointer, fmt.Sprintf("%d", index)),
						"uniqueItems",
						fmt.Sprintf("item duplicates index %d", previous),
					))

					break
				}
			}

			parsed = append(parsed, candidate)
		}
	}

	return errs
}

// validate applies object bounds, required names, properties, and additional properties.
//
//nolint:cyclop // Object keywords collect independent failures in fixed order.
func (object ObjectValidation) validate(validation *Validation, value instance, pointer string) []error {
	if value.kind != jsonvalue.KindObject {
		return nil
	}

	var errs []error
	if object.MinProperties != nil && compareCount(len(value.object), object.MinProperties) < 0 {
		errs = append(errs, newValidationError(validation, pointer, "minProperties", fmt.Sprintf(
			"property count %d is less than %s", len(value.object), object.MinProperties.Value,
		)))
	}

	if object.MaxProperties != nil && compareCount(len(value.object), object.MaxProperties) > 0 {
		errs = append(errs, newValidationError(validation, pointer, "maxProperties", fmt.Sprintf(
			"property count %d is greater than %s", len(value.object), object.MaxProperties.Value,
		)))
	}

	for _, required := range object.Required {
		if !hasObjectMember(value.object, required) {
			errs = append(errs, newValidationError(
				validation,
				appendInstancePointer(pointer, required), "required", "required property is absent",
			))
		}
	}

	for _, member := range value.object {
		property := object.property(member.name)

		memberPointer := appendInstancePointer(pointer, member.name)
		if property != nil {
			errs = append(errs, validateRaw(property.Validation, member.raw, memberPointer)...)

			continue
		}

		if object.AdditionalPropertiesValidation != nil {
			errs = append(errs, validateRaw(
				object.AdditionalPropertiesValidation, member.raw, memberPointer,
			)...)

			continue
		}

		if !object.AdditionalPropertiesAllowed {
			errs = append(errs, newValidationError(
				validation, memberPointer, "additionalProperties", "property is not allowed",
			))
		}
	}

	return errs
}

// compareCount compares one in-memory count with an exact schema bound.
func compareCount(count int, bound *CountBound) int {
	value := jsonvalue.Number{
		Lexeme:   fmt.Sprintf("%d", count),
		Rational: new(big.Rat).SetInt64(int64(count)),
	}

	return value.Compare(bound.ExactValue)
}

// hasObjectMember searches lexically sorted raw members.
func hasObjectMember(members []rawMember, name string) bool {
	index := sort.Search(len(members), func(index int) bool {
		return members[index].name >= name
	})

	return index < len(members) && members[index].name == name
}

// property searches lexically sorted compiled property validations.
func (object ObjectValidation) property(name string) *PropertyValidation {
	index := sort.Search(len(object.Properties), func(index int) bool {
		return object.Properties[index].Name >= name
	})
	if index == len(object.Properties) || object.Properties[index].Name != name {
		return nil
	}

	return &object.Properties[index]
}

// validationError carries stable instance, schema, keyword, and reason context.
type validationError struct {
	instancePointer string
	schemaPointer   string
	keyword         string
	reason          string
}

// Error formats stable validation context.
func (validationError validationError) Error() string {
	return fmt.Sprintf(
		"instance %s schema %s keyword %s: %s",
		validationError.instancePointer,
		validationError.schemaPointer,
		validationError.keyword,
		validationError.reason,
	)
}

// newValidationError locates a rule failure at one schema node.
func newValidationError(
	validation *Validation,
	instancePointer string,
	keyword string,
	reason string,
) error {
	return validationError{
		instancePointer: instancePointer,
		schemaPointer:   validation.SchemaPointer,
		keyword:         keyword,
		reason:          reason,
	}
}

// appendInstancePointer appends one RFC 6901-escaped token.
func appendInstancePointer(pointer string, token string) string {
	escaped := bytes.ReplaceAll([]byte(token), []byte("~"), []byte("~0"))
	escaped = bytes.ReplaceAll(escaped, []byte("/"), []byte("~1"))

	return pointer + "/" + string(escaped)
}

// kindName returns the JSON spelling of a value kind.
func kindName(kind jsonvalue.Kind) string {
	switch kind {
	case jsonvalue.KindNull:
		return "null"
	case jsonvalue.KindBoolean:
		return "boolean"
	case jsonvalue.KindNumber:
		return "number"
	case jsonvalue.KindString:
		return "string"
	case jsonvalue.KindArray:
		return "array"
	case jsonvalue.KindObject:
		return "object"
	default:
		return "unknown"
	}
}
