package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

const (
	// float32BitSize selects IEEE-754 binary32 parsing.
	float32BitSize = 32
	// float64BitSize selects IEEE-754 binary64 parsing.
	float64BitSize = 64
)

var (
	// int32Minimum is the exact lower signed binary32 integer bound.
	int32Minimum = jsonvalue.Number{Lexeme: "-2147483648"}
	// int32Maximum is the exact upper signed binary32 integer bound.
	int32Maximum = jsonvalue.Number{Lexeme: "2147483647"}
	// int64Minimum is the exact lower signed binary64 integer bound.
	int64Minimum = jsonvalue.Number{Lexeme: "-9223372036854775808"}
	// int64Maximum is the exact upper signed binary64 integer bound.
	int64Maximum = jsonvalue.Number{Lexeme: "9223372036854775807"}
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

	run := validationRun{anyOfMatches: make(map[validationMemoKey]bool)}

	return validateRaw(&run, validation, body, "#")
}

// validateCompiledState rejects malformed validation graphs at construction boundaries.
func validateCompiledState(root *Validation) error {
	active := make(map[*Validation]struct{})
	validated := make(map[*Validation]struct{})

	var validateNode func(*Validation) error

	validateNode = func(validation *Validation) error {
		if validation == nil {
			return errors.New("validation node is nil")
		}

		if _, cyclic := active[validation]; cyclic {
			return fmt.Errorf("validation cycle reaches schema %q", validation.SchemaPointer)
		}

		if _, done := validated[validation]; done {
			return nil
		}

		active[validation] = struct{}{}
		defer delete(active, validation)

		if err := validateLocalCompiledState(validation); err != nil {
			return fmt.Errorf("schema %q: %w", validation.SchemaPointer, err)
		}

		children := make([]*Validation, 0, 3+len(validation.ObjectValidation.Properties)+
			len(validation.AllOfValidations)+len(validation.AnyOfValidations))
		if validation.ArrayValidation.Items != nil {
			children = append(children, validation.ArrayValidation.Items)
		}

		for _, property := range validation.ObjectValidation.Properties {
			children = append(children, property.Validation)
		}

		if validation.ObjectValidation.AdditionalPropertiesValidation != nil {
			children = append(children, validation.ObjectValidation.AdditionalPropertiesValidation)
		}

		children = append(children, validation.AllOfValidations...)
		children = append(children, validation.AnyOfValidations...)

		for _, child := range children {
			if err := validateNode(child); err != nil {
				return err
			}
		}

		validated[validation] = struct{}{}

		return nil
	}

	return validateNode(root)
}

// validateLocalCompiledState checks every exported keyword family at one validation node.
//
//nolint:cyclop // Every exported compiled keyword family is checked at one node boundary.
func validateLocalCompiledState(validation *Validation) error {
	if typeName := validation.KindValidation.Type; typeName != "" &&
		typeName != "boolean" && typeName != "integer" && typeName != "number" &&
		typeName != "string" && typeName != "array" && typeName != "object" {
		return fmt.Errorf("invalid type %q", typeName)
	}

	if len(validation.EnumValidation.Values) != len(validation.EnumValidation.ExactValues) {
		return errors.New("enum source and exact values differ in length")
	}

	for index, exact := range validation.EnumValidation.ExactValues {
		_, err := exact.MarshalJSON()
		if err != nil {
			return fmt.Errorf("enum member %d: %w", index, err)
		}

		authored, err := jsonvalue.Parse(validation.EnumValidation.Values[index])
		if err != nil {
			return fmt.Errorf("enum member %d source: %w", index, err)
		}

		if !authored.Equal(exact) {
			return fmt.Errorf("enum member %d source and exact value differ", index)
		}
	}

	if err := validateNumberCompiledState(validation.NumberValidation); err != nil {
		return err
	}

	for _, compiled := range []struct {
		name  string
		bound *CountBound
	}{
		{name: "minLength", bound: validation.StringValidation.MinLength},
		{name: "maxLength", bound: validation.StringValidation.MaxLength},
		{name: "minItems", bound: validation.ArrayValidation.MinItems},
		{name: "maxItems", bound: validation.ArrayValidation.MaxItems},
		{name: "minProperties", bound: validation.ObjectValidation.MinProperties},
		{name: "maxProperties", bound: validation.ObjectValidation.MaxProperties},
	} {
		name, bound := compiled.name, compiled.bound
		if bound != nil {
			if err := validateCountBound(bound); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
	}

	for index, property := range validation.ObjectValidation.Properties {
		if property.Name == "" {
			return fmt.Errorf("object property %d has empty name", index)
		}

		if index > 0 && validation.ObjectValidation.Properties[index-1].Name >= property.Name {
			return fmt.Errorf("object property %d name %q is not strictly increasing", index, property.Name)
		}
	}

	if validation.StringValidation.Pattern != "" && validation.StringValidation.CompiledPattern == nil {
		return errors.New("pattern has no compiled validation")
	}

	if validation.StringValidation.CompiledFormat != nil {
		if _, err := validation.StringValidation.CompiledFormat.Matches(""); err != nil {
			return fmt.Errorf("format: %w", err)
		}
	} else if format := validation.StringValidation.Format; format != "" && format != "password" {
		return fmt.Errorf("format %q has no compiled language", format)
	}

	return nil
}

// validateNumberCompiledState checks exact numeric source and compiled forms together.
//
//nolint:cyclop // Exact numeric source and compiled forms are checked together.
func validateNumberCompiledState(number NumberValidation) error {
	for _, compiled := range []struct {
		name  string
		bound *NumberBound
	}{
		{name: "minimum", bound: number.Minimum},
		{name: "maximum", bound: number.Maximum},
	} {
		name, bound := compiled.name, compiled.bound
		if bound == nil {
			continue
		}

		parsed, err := jsonvalue.ParseNumber(bound.Value)
		if err != nil {
			return fmt.Errorf("%s source: %w", name, err)
		}

		comparison, err := parsed.Compare(bound.ExactValue)
		if err != nil {
			return fmt.Errorf("%s exact value: %w", name, err)
		}

		if comparison != 0 {
			return fmt.Errorf("%s source and exact value differ", name)
		}
	}

	if number.ExactMultipleOf == nil {
		if number.MultipleOf != "" {
			return errors.New("multipleOf has no exact value")
		}

		return nil
	}

	parsed, err := jsonvalue.ParseNumber(number.MultipleOf)
	if err != nil {
		return fmt.Errorf("multipleOf source: %w", err)
	}

	comparison, err := parsed.Compare(*number.ExactMultipleOf)
	if err != nil {
		return fmt.Errorf("multipleOf exact value: %w", err)
	}

	if comparison != 0 {
		return errors.New("multipleOf source and exact value differ")
	}

	return validatePositiveNumber(parsed)
}

// validateCountBound checks one exact non-negative integer compiled bound.
func validateCountBound(bound *CountBound) error {
	parsed, err := jsonvalue.ParseNumber(bound.Value)
	if err != nil {
		return err
	}

	comparison, err := parsed.Compare(bound.ExactValue)
	if err != nil {
		return err
	}

	if comparison != 0 {
		return errors.New("source and exact value differ")
	}

	return validateNonNegativeInteger(parsed)
}

// validatePositiveNumber checks the exact-number precondition for multipleOf.
func validatePositiveNumber(number jsonvalue.Number) error {
	comparison, err := number.Compare(jsonvalue.Number{Lexeme: "0"})
	if err != nil {
		return err
	}

	if comparison <= 0 {
		return errors.New("multipleOf is not positive")
	}

	return nil
}

// validateNonNegativeInteger checks the exact-number precondition for count bounds.
func validateNonNegativeInteger(number jsonvalue.Number) error {
	integer, err := number.IsInteger()
	if err != nil {
		return err
	}

	if !integer || strings.HasPrefix(number.Lexeme, "-") {
		return errors.New("bound is not a non-negative integer")
	}

	return nil
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

// validationMemoKey identifies one shared schema evaluation against one request instance.
type validationMemoKey struct {
	validation *Validation
	pointer    string
	raw        string
}

// validationRun owns memoized anyOf verdicts for one Validate request.
type validationRun struct {
	anyOfMatches map[validationMemoKey]bool
}

// validateRaw applies one compiled schema node to one raw instance node.
func validateRaw(run *validationRun, validation *Validation, raw json.RawMessage, pointer string) []error {
	errs := validateLocalAndAllOf(run, validation, raw, pointer)
	if len(validation.AnyOfValidations) == 0 {
		return errs
	}

	for _, child := range validation.AnyOfValidations {
		key := validationMemoKey{validation: child, pointer: pointer, raw: string(raw)}

		matches, cached := run.anyOfMatches[key]
		if !cached {
			matches = len(validateRaw(run, child, raw, pointer)) == 0
			run.anyOfMatches[key] = matches
		}

		if matches {
			return errs
		}
	}

	return append(errs, newValidationError(
		validation,
		pointer,
		"anyOf",
		"value must validate against at least one alternative",
	))
}

// validateLocalAndAllOf applies local rules and every conjunctive child.
func validateLocalAndAllOf(
	run *validationRun,
	validation *Validation,
	raw json.RawMessage,
	pointer string,
) []error {
	value, err := decodeInstance(raw)
	if err != nil {
		return []error{newValidationError(validation, pointer, "body", err.Error())}
	}

	errs := validation.KindValidation.validate(validation, value, pointer)
	errs = append(errs, validation.EnumValidation.validate(validation, value, pointer)...)
	errs = append(errs, validation.NumberValidation.validate(validation, value, pointer)...)
	errs = append(errs, validation.StringValidation.validate(validation, value, pointer)...)
	errs = append(errs, validation.ArrayValidation.validate(run, validation, value, pointer)...)
	errs = append(errs, validation.ObjectValidation.validate(run, validation, value, pointer)...)

	for _, child := range validation.AllOfValidations {
		errs = append(errs, validateRaw(run, child, raw, pointer)...)
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

// objectMemberDecoder is the streaming seam used to decode strict object members.
type objectMemberDecoder interface {
	Token() (json.Token, error)
	More() bool
	Decode(value any) error
}

// decodeObjectMembers streams and lexically sorts raw object member values.
func decodeObjectMembers(raw []byte) ([]rawMember, error) {
	return decodeObjectMembersFrom(json.NewDecoder(bytes.NewReader(raw)))
}

// decodeObjectMembersFrom decodes object members from one token stream.
func decodeObjectMembersFrom(decoder objectMemberDecoder) ([]rawMember, error) {
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
		if value.kind == jsonvalue.KindNumber {
			integer, err := value.number.IsInteger()
			if err != nil {
				return []error{newValidationError(validation, pointer, "type", err.Error())}
			}

			matches = integer
		}
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
		comparison, err := value.number.Compare(number.Minimum.ExactValue)
		if err != nil {
			errs = append(errs, newValidationError(validation, pointer, "minimum", err.Error()))
		} else if comparison < 0 || comparison == 0 && number.Minimum.Exclusive {
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
		comparison, err := value.number.Compare(number.Maximum.ExactValue)
		if err != nil {
			errs = append(errs, newValidationError(validation, pointer, "maximum", err.Error()))
		} else if comparison > 0 || comparison == 0 && number.Maximum.Exclusive {
			reason := fmt.Sprintf("value must be less than or equal to %s", number.Maximum.Value)
			keyword := "maximum"

			if number.Maximum.Exclusive {
				reason = fmt.Sprintf("value must be less than %s", number.Maximum.Value)
				keyword = "exclusiveMaximum"
			}

			errs = append(errs, newValidationError(validation, pointer, keyword, reason))
		}
	}

	if number.ExactMultipleOf != nil {
		multiple, err := value.number.IsMultipleOf(*number.ExactMultipleOf)
		if err != nil {
			errs = append(errs, newValidationError(validation, pointer, "multipleOf", err.Error()))
		} else if !multiple {
			errs = append(errs, newValidationError(
				validation,
				pointer,
				"multipleOf",
				fmt.Sprintf("value must be an exact multiple of %s", number.MultipleOf),
			))
		}
	}

	if err := validateNumberFormat(value.number, number.Format); err != nil {
		errs = append(errs, newValidationError(validation, pointer, "format", err.Error()))
	}

	return errs
}

// validateNumberFormat applies one prevalidated numeric format.
func validateNumberFormat(number jsonvalue.Number, format string) error {
	switch format {
	case "":
		return nil
	case "int32":
		return validateSignedInteger(number, int32Minimum, int32Maximum, format)
	case "int64":
		return validateSignedInteger(number, int64Minimum, int64Maximum, format)
	case "float":
		return validateFloat(number, float32BitSize)
	case "double":
		return validateFloat(number, float64BitSize)
	default:
		return fmt.Errorf("uncompiled numeric format %q", format)
	}
}

// validateSignedInteger checks mathematical integrality and exact inclusive bounds.
func validateSignedInteger(
	number jsonvalue.Number,
	minimum jsonvalue.Number,
	maximum jsonvalue.Number,
	format string,
) error {
	integer, err := number.IsInteger()
	if err != nil {
		return err
	}

	if !integer {
		return fmt.Errorf("value is not an integer in %s", format)
	}

	minimumComparison, err := number.Compare(minimum)
	if err != nil {
		return err
	}

	maximumComparison, err := number.Compare(maximum)
	if err != nil {
		return err
	}

	if minimumComparison < 0 || maximumComparison > 0 {
		return fmt.Errorf("value is outside %s signed range", format)
	}

	return nil
}

// validateFloat delegates the complete floating-point policy to strconv.
func validateFloat(number jsonvalue.Number, bitSize int) error {
	if _, err := strconv.ParseFloat(number.Lexeme, bitSize); err != nil {
		return fmt.Errorf("invalid float%d: %w", bitSize, err)
	}

	return nil
}

// validate applies string length, pattern, and format constraints.
//
//nolint:cyclop // Independent string keywords each propagate malformed compiled state.
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
	if stringValidation.MinLength != nil {
		comparison, err := compareCount(length, stringValidation.MinLength)
		if err != nil {
			errs = append(errs, newValidationError(validation, pointer, "minLength", err.Error()))
		} else if comparison < 0 {
			errs = append(errs, newValidationError(validation, pointer, "minLength", fmt.Sprintf(
				"length %d is less than %s", length, stringValidation.MinLength.Value,
			)))
		}
	}

	if stringValidation.MaxLength != nil {
		comparison, err := compareCount(length, stringValidation.MaxLength)
		if err != nil {
			errs = append(errs, newValidationError(validation, pointer, "maxLength", err.Error()))
		} else if comparison > 0 {
			errs = append(errs, newValidationError(validation, pointer, "maxLength", fmt.Sprintf(
				"length %d is greater than %s", length, stringValidation.MaxLength.Value,
			)))
		}
	}

	if stringValidation.CompiledPattern != nil && !stringValidation.CompiledPattern.Validate(value.string) {
		errs = append(errs, newValidationError(validation, pointer, "pattern", fmt.Sprintf(
			"string does not match %q", stringValidation.Pattern,
		)))
	}

	if stringValidation.CompiledFormat != nil {
		matches, err := stringValidation.CompiledFormat.Matches(value.string)
		if err != nil {
			errs = append(errs, newValidationError(validation, pointer, "format", err.Error()))
		} else if !matches {
			errs = append(errs, newValidationError(validation, pointer, "format", fmt.Sprintf(
				"string does not match %q format", stringValidation.Format,
			)))
		}
	}

	return errs
}

// validate applies array bounds and child schemas.
func (array ArrayValidation) validate(
	run *validationRun,
	validation *Validation,
	value instance,
	pointer string,
) []error {
	if value.kind != jsonvalue.KindArray {
		return nil
	}

	var errs []error

	if array.MinItems != nil {
		comparison, err := compareCount(len(value.array), array.MinItems)
		if err != nil {
			errs = append(errs, newValidationError(validation, pointer, "minItems", err.Error()))
		} else if comparison < 0 {
			errs = append(errs, newValidationError(validation, pointer, "minItems", fmt.Sprintf(
				"item count %d is less than %s", len(value.array), array.MinItems.Value,
			)))
		}
	}

	if array.MaxItems != nil {
		comparison, err := compareCount(len(value.array), array.MaxItems)
		if err != nil {
			errs = append(errs, newValidationError(validation, pointer, "maxItems", err.Error()))
		} else if comparison > 0 {
			errs = append(errs, newValidationError(validation, pointer, "maxItems", fmt.Sprintf(
				"item count %d is greater than %s", len(value.array), array.MaxItems.Value,
			)))
		}
	}

	if array.Items != nil {
		for index, child := range value.array {
			errs = append(errs, validateRaw(
				run, array.Items, child, appendInstancePointer(pointer, fmt.Sprintf("%d", index)),
			)...)
		}
	}

	return errs
}

// validate applies object bounds, required names, properties, and additional properties.
//
//nolint:cyclop // Object keywords collect independent failures in fixed order.
func (object ObjectValidation) validate(
	run *validationRun,
	validation *Validation,
	value instance,
	pointer string,
) []error {
	if value.kind != jsonvalue.KindObject {
		return nil
	}

	var errs []error

	if object.MinProperties != nil {
		comparison, err := compareCount(len(value.object), object.MinProperties)
		if err != nil {
			errs = append(errs, newValidationError(validation, pointer, "minProperties", err.Error()))
		} else if comparison < 0 {
			errs = append(errs, newValidationError(validation, pointer, "minProperties", fmt.Sprintf(
				"property count %d is less than %s", len(value.object), object.MinProperties.Value,
			)))
		}
	}

	if object.MaxProperties != nil {
		comparison, err := compareCount(len(value.object), object.MaxProperties)
		if err != nil {
			errs = append(errs, newValidationError(validation, pointer, "maxProperties", err.Error()))
		} else if comparison > 0 {
			errs = append(errs, newValidationError(validation, pointer, "maxProperties", fmt.Sprintf(
				"property count %d is greater than %s", len(value.object), object.MaxProperties.Value,
			)))
		}
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
			errs = append(errs, validateRaw(run, property.Validation, member.raw, memberPointer)...)

			continue
		}

		if object.AdditionalPropertiesValidation != nil {
			errs = append(errs, validateRaw(
				run, object.AdditionalPropertiesValidation, member.raw, memberPointer,
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
func compareCount(count int, bound *CountBound) (int, error) {
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
