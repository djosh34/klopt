// Package example contains generated OpenAPI request-body models.
package example

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"
)

var (
	// NotAnObjectError reports that an object schema received another JSON type.
	//nolint:errname,revive,staticcheck // Existing generated API keeps its original exported error name.
	NotAnObjectError = errors.New("not an object")
	// AdditionalPropertyError reports a property forbidden by an object schema.
	//nolint:errname,revive,staticcheck // Existing generated API keeps its original exported error name.
	AdditionalPropertyError = errors.New("additional property")
	// MissingRequiredPropertyError reports a missing required object property.
	//nolint:errname,revive,staticcheck // Existing generated API keeps its original exported error name.
	MissingRequiredPropertyError = errors.New("missing required property")
	// NullForNotNullableBoolError reports null for a non-nullable boolean.
	//nolint:errname,revive,staticcheck // Existing generated API keeps its original exported error name.
	NullForNotNullableBoolError = errors.New("null for not nullable bool")
	// NonBoolForBoolSchemaError reports a non-boolean value for a boolean schema.
	//nolint:errname,revive,staticcheck // Existing generated API keeps its original exported error name.
	NonBoolForBoolSchemaError = errors.New("non-bool for bool schema")
	// NullForNotNullableNumberError reports null for a non-nullable number.
	//nolint:errname,revive,staticcheck // Existing generated API keeps its original exported error name.
	NullForNotNullableNumberError = errors.New("null for not nullable number")
	// NonNumberForNumberSchemaError reports a non-number value for a number schema.
	//nolint:errname,revive,staticcheck // Existing generated API keeps its original exported error name.
	NonNumberForNumberSchemaError = errors.New("non-number for number schema")
	// NullForNotNullableStringError reports null for a non-nullable string.
	//nolint:errname,revive,staticcheck // Existing generated API keeps its original exported error name.
	NullForNotNullableStringError = errors.New("null for not nullable string")
	// NonStringForStringSchemaError reports a non-string value for a string schema.
	//nolint:errname,revive,staticcheck // Existing generated API keeps its original exported error name.
	NonStringForStringSchemaError = errors.New("non-string for string schema")
)

// jsonNull is the JSON null literal.
var jsonNull = []byte("null")

// objectPropertyDecoder decodes one declared object property.
type objectPropertyDecoder struct {
	name     string
	required bool
	seen     bool
	decode   func([]byte) error
}

// requiredObjectProperty describes a required generated object property.
func requiredObjectProperty[T any](name string, target *T) objectPropertyDecoder {
	return objectPropertyDecoder{
		name:     name,
		required: true,
		decode: func(data []byte) error {
			var value T
			if err := json.Unmarshal(data, &value); err != nil {
				return err
			}

			*target = value

			return nil
		},
	}
}

// optionalObjectProperty describes an optional generated object property.
func optionalObjectProperty[T any](name string, target **T) objectPropertyDecoder {
	return objectPropertyDecoder{
		name: name,
		decode: func(data []byte) error {
			var value T
			if err := json.Unmarshal(data, &value); err != nil {
				return err
			}

			*target = &value

			return nil
		},
	}
}

// unmarshalObject decodes an object while preserving input-order error behavior.
func unmarshalObject(
	data []byte,
	emptyBodyAllowed bool,
	nullable bool,
	properties []objectPropertyDecoder,
	additionalProperties bool,
	decodeAdditionalProperty func([]byte) error,
) error {
	decoder, skipped, err := objectDecoder(data, emptyBodyAllowed, nullable)
	if err != nil || skipped {
		return err
	}

	err = unmarshalObjectProperties(
		decoder,
		properties,
		additionalProperties,
		decodeAdditionalProperty,
	)
	if err != nil {
		return err
	}

	if err = finishObjectDecoder(decoder, data); err != nil {
		return err
	}

	return requireObjectProperties(properties)
}

// objectDecoder starts decoding an object or reports an allowed skipped body.
func objectDecoder(data []byte, emptyBodyAllowed bool, nullable bool) (*json.Decoder, bool, error) {
	if emptyBodyAllowed && len(data) == 0 {
		return nil, true, nil
	}

	if nullable && bytes.Equal(data, jsonNull) {
		return nil, true, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	token, err := decoder.Token()
	if err != nil {
		return nil, false, err
	}

	if token != json.Delim('{') {
		return nil, false, NotAnObjectError
	}

	return decoder, false, nil
}

// unmarshalObjectProperties decodes properties in input order.
func unmarshalObjectProperties(
	decoder *json.Decoder,
	properties []objectPropertyDecoder,
	additionalProperties bool,
	decodeAdditionalProperty func([]byte) error,
) error {
	for decoder.More() {
		nameToken, err := decoder.Token()
		if err != nil {
			return err
		}

		name, ok := nameToken.(string)
		if !ok {
			return NotAnObjectError
		}

		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil {
			return err
		}

		err = unmarshalObjectProperty(
			properties,
			name,
			value,
			additionalProperties,
			decodeAdditionalProperty,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// finishObjectDecoder consumes the closing delimiter and rejects trailing data.
func finishObjectDecoder(decoder *json.Decoder, data []byte) error {
	if _, err := decoder.Token(); err != nil {
		return err
	}

	if len(bytes.TrimSpace(data[decoder.InputOffset():])) != 0 {
		return NotAnObjectError
	}

	return nil
}

// requireObjectProperties checks that every required property was seen.
func requireObjectProperties(properties []objectPropertyDecoder) error {
	for i := range properties {
		if properties[i].required && !properties[i].seen {
			return fmt.Errorf("%w: %s", MissingRequiredPropertyError, properties[i].name)
		}
	}

	return nil
}

// unmarshalObjectProperty decodes one declared or additional property.
func unmarshalObjectProperty(
	properties []objectPropertyDecoder,
	name string,
	value []byte,
	additionalProperties bool,
	decodeAdditionalProperty func([]byte) error,
) error {
	property := findObjectProperty(properties, name)
	if property != nil {
		property.seen = true

		return property.decode(value)
	}

	if !additionalProperties {
		return fmt.Errorf("%w: %s", AdditionalPropertyError, name)
	}

	if decodeAdditionalProperty != nil {
		return decodeAdditionalProperty(value)
	}

	return nil
}

// findObjectProperty returns the decoder for name, if name is declared.
func findObjectProperty(properties []objectPropertyDecoder, name string) *objectPropertyDecoder {
	low, high := 0, len(properties)
	for low < high {
		middle := low + ((high - low) >> 1)
		switch {
		case properties[middle].name < name:
			low = middle + 1
		case properties[middle].name > name:
			high = middle
		default:
			return &properties[middle]
		}
	}

	return nil
}

// jsonField is one candidate field in a generated object's JSON representation.
type jsonField struct {
	name    string
	value   any
	depth   int
	omitted bool
	tagged  bool
}

// jsonFieldSource provides the fields promoted by a generated model.
type jsonFieldSource interface {
	jsonFields() []jsonField
}

// requiredJSONField describes a required property for marshaling.
func requiredJSONField(name string, value any) jsonField {
	return jsonField{name: name, value: value}
}

// objectJSONFields reads generated object fields in declaration order.
func objectJSONFields(object any, names []string, optional []bool) []jsonField {
	value := reflect.ValueOf(object)

	fields := make([]jsonField, 0, value.NumField())
	for index, name := range names {
		field := value.Field(index)
		omitted := optional[index] && field.IsNil()
		fields = append(fields, jsonField{
			name: name, value: field.Interface(), omitted: omitted, tagged: true,
		})
	}

	return fields
}

// appendEmbeddedJSONFields adds the fields promoted by one embedded allOf member.
func appendEmbeddedJSONFields(fields []jsonField, source jsonFieldSource) []jsonField {
	embeddedStruct := reflect.ValueOf(source).Kind() == reflect.Struct
	for _, field := range source.jsonFields() {
		if embeddedStruct {
			field.depth++
		}

		fields = append(fields, field)
	}

	return fields
}

// marshalJSONFields applies encoding/json's dominant embedded-field rules.
func marshalJSONFields(fields []jsonField) ([]byte, error) {
	byName := make(map[string][]int)
	for index, field := range fields {
		byName[field.name] = append(byName[field.name], index)
	}

	selected := make(map[int]struct{}, len(byName))
	for _, candidates := range byName {
		index, ok := dominantJSONField(fields, candidates)
		if ok && !fields[index].omitted {
			selected[index] = struct{}{}
		}
	}

	data := []byte{'{'}

	for index, field := range fields {
		if _, ok := selected[index]; !ok {
			continue
		}

		if len(data) != 1 {
			data = append(data, ',')
		}

		name, err := json.Marshal(field.name)
		if err != nil {
			return nil, err
		}

		value, err := json.Marshal(field.value)
		if err != nil {
			return nil, err
		}

		data = append(data, name...)
		data = append(data, ':')
		data = append(data, value...)
	}

	return append(data, '}'), nil
}

// dominantJSONField selects the shallowest non-conflicting field.
func dominantJSONField(fields []jsonField, candidates []int) (int, bool) {
	selected := candidates[0]
	conflict := false

	for _, candidate := range candidates[1:] {
		switch {
		case fields[candidate].depth < fields[selected].depth:
			selected = candidate
			conflict = false
		case fields[candidate].depth > fields[selected].depth:
			continue
		case fields[candidate].tagged && !fields[selected].tagged:
			selected = candidate
			conflict = false
		case !fields[candidate].tagged && fields[selected].tagged:
			continue
		default:
			conflict = true
		}
	}

	return selected, !conflict
}

// StringNoFormatNullable is generated.
type StringNoFormatNullable struct {
	Value *string
}

var _ json.Unmarshaler = new(StringNoFormatNullable)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s StringNoFormatNullable) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// StringNoFormatNotNullable is generated.
type StringNoFormatNotNullable string

var _ json.Unmarshaler = new(StringNoFormatNotNullable)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s StringNoFormatNotNullable) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("StringNoFormatNotNullable", s)}
}

// RefStressObjectPut is generated.
type RefStressObjectPut struct {
	RefStressObjectPutAllOf1
	RefStressObjectPutAllOf2
	RefStressObjectPutAllOf3
}

var (
	_ json.Unmarshaler = (*RefStressObjectPut)(nil)
	_ json.Marshaler   = RefStressObjectPut{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectPut) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectPut) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf2)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf3)

	return fields
}

// RefStressObjectPutAllOf1 is generated.
type RefStressObjectPutAllOf1 struct {
	RefStressObjectPutAllOf1AllOf1
	RefStressObjectPutAllOf1AllOf2
	RefStressObjectPutAllOf1AllOf3
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectPutAllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectPutAllOf1) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf1AllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf1AllOf2)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf1AllOf3)

	return fields
}

// RefStressObjectPutAllOf1AllOf1 is generated.
type RefStressObjectPutAllOf1AllOf1 struct {
	FinalCode      RefStressObjectPutAllOf1AllOf1FinalCode
	Nested         *RefStressObjectPutAllOf1AllOf1Nested
	OptionalShared *RefStressObjectPutAllOf1AllOf1OptionalShared
	SharedName     RefStressObjectPutAllOf1AllOf1SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf1)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf1{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf1) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf1) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf1FinalCode is generated.
type RefStressObjectPutAllOf1AllOf1FinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf1FinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf1FinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf1FinalCode", s)}
}

// RefStressObjectPutAllOf1AllOf1Nested is generated.
type RefStressObjectPutAllOf1AllOf1Nested struct {
	Leaf     *RefStressObjectPutAllOf1AllOf1NestedLeaf
	SameName RefStressObjectPutAllOf1AllOf1NestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf1Nested)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf1Nested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf1Nested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf1Nested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf1Nested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf1NestedLeaf is generated.
type RefStressObjectPutAllOf1AllOf1NestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf1NestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf1NestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf1NestedLeaf", s)}
}

// RefStressObjectPutAllOf1AllOf1NestedSameName is generated.
type RefStressObjectPutAllOf1AllOf1NestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf1NestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf1NestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf1AllOf1OptionalShared is generated.
type RefStressObjectPutAllOf1AllOf1OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf1OptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf1OptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf1AllOf1SharedName is generated.
type RefStressObjectPutAllOf1AllOf1SharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf1SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf1SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf1SharedName", s)}
}

// RefStressObjectPutAllOf1AllOf2 is generated.
type RefStressObjectPutAllOf1AllOf2 struct {
	RefStressObjectPutAllOf1AllOf2AllOf1
	RefStressObjectPutAllOf1AllOf2AllOf2
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf2{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectPutAllOf1AllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectPutAllOf1AllOf2) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf1AllOf2AllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf1AllOf2AllOf2)

	return fields
}

// RefStressObjectPutAllOf1AllOf2AllOf1 is generated.
type RefStressObjectPutAllOf1AllOf2AllOf1 struct {
	RefStressObjectPutAllOf1AllOf2AllOf1AllOf1
	RefStressObjectPutAllOf1AllOf2AllOf1AllOf2
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf1)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf2AllOf1{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectPutAllOf1AllOf2AllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectPutAllOf1AllOf2AllOf1) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf1AllOf2AllOf1AllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf1AllOf2AllOf1AllOf2)

	return fields
}

// RefStressObjectPutAllOf1AllOf2AllOf1AllOf1 is generated.
type RefStressObjectPutAllOf1AllOf2AllOf1AllOf1 struct {
	FinalCode      RefStressObjectPutAllOf1AllOf2AllOf1AllOf1FinalCode
	Nested         *RefStressObjectPutAllOf1AllOf2AllOf1AllOf1Nested
	OptionalShared *RefStressObjectPutAllOf1AllOf2AllOf1AllOf1OptionalShared
	SharedName     RefStressObjectPutAllOf1AllOf2AllOf1AllOf1SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf1AllOf1)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf2AllOf1AllOf1{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf2AllOf1AllOf1) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf2AllOf1AllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf2AllOf1AllOf1) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf2AllOf1AllOf1FinalCode is generated.
type RefStressObjectPutAllOf1AllOf2AllOf1AllOf1FinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf1AllOf1FinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf2AllOf1AllOf1FinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf2AllOf1AllOf1FinalCode", s)}
}

// RefStressObjectPutAllOf1AllOf2AllOf1AllOf1Nested is generated.
type RefStressObjectPutAllOf1AllOf2AllOf1AllOf1Nested struct {
	Leaf     *RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedLeaf
	SameName RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf1AllOf1Nested)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf2AllOf1AllOf1Nested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf2AllOf1AllOf1Nested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf2AllOf1AllOf1Nested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf2AllOf1AllOf1Nested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedLeaf is generated.
type RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedLeaf", s)}
}

// RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedSameName is generated.
type RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf2AllOf1AllOf1NestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf1AllOf2AllOf1AllOf1OptionalShared is generated.
type RefStressObjectPutAllOf1AllOf2AllOf1AllOf1OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf1AllOf1OptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf2AllOf1AllOf1OptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf1AllOf2AllOf1AllOf1SharedName is generated.
type RefStressObjectPutAllOf1AllOf2AllOf1AllOf1SharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf1AllOf1SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf2AllOf1AllOf1SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf2AllOf1AllOf1SharedName", s)}
}

// RefStressObjectPutAllOf1AllOf2AllOf1AllOf2 is generated.
type RefStressObjectPutAllOf1AllOf2AllOf1AllOf2 struct {
	OptionalCode *RefStressObjectPutAllOf1AllOf2AllOf1AllOf2OptionalCode
	SharedName   RefStressObjectPutAllOf1AllOf2AllOf1AllOf2SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf1AllOf2)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf2AllOf1AllOf2{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf2AllOf1AllOf2) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("optionalCode", &o.OptionalCode),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf2AllOf1AllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf2AllOf1AllOf2) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"optionalCode",
			"sharedName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf2AllOf1AllOf2OptionalCode is generated.
type RefStressObjectPutAllOf1AllOf2AllOf1AllOf2OptionalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf1AllOf2OptionalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf2AllOf1AllOf2OptionalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf2AllOf1AllOf2OptionalCode", s)}
}

// RefStressObjectPutAllOf1AllOf2AllOf1AllOf2SharedName is generated.
type RefStressObjectPutAllOf1AllOf2AllOf1AllOf2SharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf1AllOf2SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf2AllOf1AllOf2SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf2AllOf1AllOf2SharedName", s)}
}

// RefStressObjectPutAllOf1AllOf2AllOf2 is generated.
type RefStressObjectPutAllOf1AllOf2AllOf2 struct {
	MiddleFlag RefStressObjectPutAllOf1AllOf2AllOf2MiddleFlag
	Nested     *RefStressObjectPutAllOf1AllOf2AllOf2Nested
	SharedName RefStressObjectPutAllOf1AllOf2AllOf2SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf2)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf2AllOf2{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf2AllOf2) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("middleFlag", &o.MiddleFlag),
			optionalObjectProperty("nested", &o.Nested),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf2AllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf2AllOf2) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"middleFlag",
			"nested",
			"sharedName",
		},
		[]bool{
			false,
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf2AllOf2MiddleFlag is generated.
type RefStressObjectPutAllOf1AllOf2AllOf2MiddleFlag bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2MiddleFlag)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectPutAllOf1AllOf2AllOf2MiddleFlag) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf2AllOf2MiddleFlag", b)}
}

// RefStressObjectPutAllOf1AllOf2AllOf2Nested is generated.
type RefStressObjectPutAllOf1AllOf2AllOf2Nested struct {
	RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1
	RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2
	RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf2Nested)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf2AllOf2Nested{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectPutAllOf1AllOf2AllOf2Nested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectPutAllOf1AllOf2AllOf2Nested) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3)

	return fields
}

// RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1 is generated.
type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1 struct {
	Leaf     *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1Leaf
	SameName RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1SameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1Leaf is generated.
type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1Leaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1Leaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1Leaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1Leaf", s)}
}

// RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1SameName is generated.
type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1SameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf1SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2 is generated.
type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2 struct {
	Leaf     *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2Leaf
	SameName RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2SameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2Leaf is generated.
type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2Leaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2Leaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2Leaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2Leaf", s)}
}

// RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2SameName is generated.
type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2SameName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf2SameName", s)}
}

// RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3 is generated.
type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3 struct {
	SameName RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SameName
	Sealed   RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3Sealed
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("sameName", &o.SameName),
			requiredObjectProperty("sealed", &o.Sealed),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"sameName",
			"sealed",
		},
		[]bool{
			false,
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SameName is generated.
type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SameName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SameName", s)}
}

// RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3Sealed is generated.
type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3Sealed struct {
	Locked RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SealedLocked
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3Sealed)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3Sealed{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3Sealed) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("locked", &o.Locked),
		},
		false,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3Sealed) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3Sealed) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"locked",
		},
		[]bool{
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SealedLocked is generated.
type RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SealedLocked bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SealedLocked)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SealedLocked) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf2AllOf2NestedAllOf3SealedLocked", b)}
}

// RefStressObjectPutAllOf1AllOf2AllOf2SharedName is generated.
type RefStressObjectPutAllOf1AllOf2AllOf2SharedName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf2AllOf2SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf2AllOf2SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf1AllOf3 is generated.
type RefStressObjectPutAllOf1AllOf3 struct {
	Final            RefStressObjectPutAllOf1AllOf3Final
	Nested           RefStressObjectPutAllOf1AllOf3Nested
	NullableRequired RefStressObjectPutAllOf1AllOf3NullableRequired
	OptionalShared   *RefStressObjectPutAllOf1AllOf3OptionalShared
	SharedName       *RefStressObjectPutAllOf1AllOf3SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf3{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf3) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("final", &o.Final),
			requiredObjectProperty("nested", &o.Nested),
			requiredObjectProperty("nullableRequired", &o.NullableRequired),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			optionalObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf3) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf3) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"final",
			"nested",
			"nullableRequired",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			false,
			false,
			true,
			true,
		},
	)
}

// RefStressObjectPutAllOf1AllOf3Final is generated.
type RefStressObjectPutAllOf1AllOf3Final struct {
	FinalCode      RefStressObjectPutAllOf1AllOf3FinalFinalCode
	Nested         *RefStressObjectPutAllOf1AllOf3FinalNested
	OptionalShared *RefStressObjectPutAllOf1AllOf3FinalOptionalShared
	SharedName     RefStressObjectPutAllOf1AllOf3FinalSharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3Final)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf3Final{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf3Final) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf3Final) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf3Final) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf3FinalFinalCode is generated.
type RefStressObjectPutAllOf1AllOf3FinalFinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3FinalFinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf3FinalFinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf3FinalFinalCode", s)}
}

// RefStressObjectPutAllOf1AllOf3FinalNested is generated.
type RefStressObjectPutAllOf1AllOf3FinalNested struct {
	Leaf     *RefStressObjectPutAllOf1AllOf3FinalNestedLeaf
	SameName RefStressObjectPutAllOf1AllOf3FinalNestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3FinalNested)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf3FinalNested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf3FinalNested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf3FinalNested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf3FinalNested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf3FinalNestedLeaf is generated.
type RefStressObjectPutAllOf1AllOf3FinalNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3FinalNestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf3FinalNestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf3FinalNestedLeaf", s)}
}

// RefStressObjectPutAllOf1AllOf3FinalNestedSameName is generated.
type RefStressObjectPutAllOf1AllOf3FinalNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3FinalNestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf3FinalNestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf1AllOf3FinalOptionalShared is generated.
type RefStressObjectPutAllOf1AllOf3FinalOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3FinalOptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf3FinalOptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf1AllOf3FinalSharedName is generated.
type RefStressObjectPutAllOf1AllOf3FinalSharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3FinalSharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf3FinalSharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf3FinalSharedName", s)}
}

// RefStressObjectPutAllOf1AllOf3Nested is generated.
type RefStressObjectPutAllOf1AllOf3Nested struct {
	RefStressObjectPutAllOf1AllOf3NestedAllOf1
	RefStressObjectPutAllOf1AllOf3NestedAllOf2
	RefStressObjectPutAllOf1AllOf3NestedAllOf3
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3Nested)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf3Nested{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectPutAllOf1AllOf3Nested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectPutAllOf1AllOf3Nested) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf1AllOf3NestedAllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf1AllOf3NestedAllOf2)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf1AllOf3NestedAllOf3)

	return fields
}

// RefStressObjectPutAllOf1AllOf3NestedAllOf1 is generated.
type RefStressObjectPutAllOf1AllOf3NestedAllOf1 struct {
	Leaf     *RefStressObjectPutAllOf1AllOf3NestedAllOf1Leaf
	SameName RefStressObjectPutAllOf1AllOf3NestedAllOf1SameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3NestedAllOf1)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf3NestedAllOf1{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf3NestedAllOf1) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf3NestedAllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf3NestedAllOf1) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf3NestedAllOf1Leaf is generated.
type RefStressObjectPutAllOf1AllOf3NestedAllOf1Leaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3NestedAllOf1Leaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf3NestedAllOf1Leaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf3NestedAllOf1Leaf", s)}
}

// RefStressObjectPutAllOf1AllOf3NestedAllOf1SameName is generated.
type RefStressObjectPutAllOf1AllOf3NestedAllOf1SameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3NestedAllOf1SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf3NestedAllOf1SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf1AllOf3NestedAllOf2 is generated.
type RefStressObjectPutAllOf1AllOf3NestedAllOf2 struct {
	Leaf     *RefStressObjectPutAllOf1AllOf3NestedAllOf2Leaf
	SameName RefStressObjectPutAllOf1AllOf3NestedAllOf2SameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3NestedAllOf2)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf3NestedAllOf2{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf3NestedAllOf2) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf3NestedAllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf3NestedAllOf2) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf3NestedAllOf2Leaf is generated.
type RefStressObjectPutAllOf1AllOf3NestedAllOf2Leaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3NestedAllOf2Leaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf3NestedAllOf2Leaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf3NestedAllOf2Leaf", s)}
}

// RefStressObjectPutAllOf1AllOf3NestedAllOf2SameName is generated.
type RefStressObjectPutAllOf1AllOf3NestedAllOf2SameName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3NestedAllOf2SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf3NestedAllOf2SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf3NestedAllOf2SameName", s)}
}

// RefStressObjectPutAllOf1AllOf3NestedAllOf3 is generated.
type RefStressObjectPutAllOf1AllOf3NestedAllOf3 struct {
	SameName RefStressObjectPutAllOf1AllOf3NestedAllOf3SameName
	Sealed   RefStressObjectPutAllOf1AllOf3NestedAllOf3Sealed
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3NestedAllOf3)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf3NestedAllOf3{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf3NestedAllOf3) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("sameName", &o.SameName),
			requiredObjectProperty("sealed", &o.Sealed),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf3NestedAllOf3) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf3NestedAllOf3) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"sameName",
			"sealed",
		},
		[]bool{
			false,
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf3NestedAllOf3SameName is generated.
type RefStressObjectPutAllOf1AllOf3NestedAllOf3SameName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3NestedAllOf3SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf3NestedAllOf3SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf3NestedAllOf3SameName", s)}
}

// RefStressObjectPutAllOf1AllOf3NestedAllOf3Sealed is generated.
type RefStressObjectPutAllOf1AllOf3NestedAllOf3Sealed struct {
	Locked RefStressObjectPutAllOf1AllOf3NestedAllOf3SealedLocked
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf1AllOf3NestedAllOf3Sealed)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf1AllOf3NestedAllOf3Sealed{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf1AllOf3NestedAllOf3Sealed) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("locked", &o.Locked),
		},
		false,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf1AllOf3NestedAllOf3Sealed) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf1AllOf3NestedAllOf3Sealed) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"locked",
		},
		[]bool{
			false,
		},
	)
}

// RefStressObjectPutAllOf1AllOf3NestedAllOf3SealedLocked is generated.
type RefStressObjectPutAllOf1AllOf3NestedAllOf3SealedLocked bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3NestedAllOf3SealedLocked)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectPutAllOf1AllOf3NestedAllOf3SealedLocked) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf1AllOf3NestedAllOf3SealedLocked", b)}
}

// RefStressObjectPutAllOf1AllOf3NullableRequired is generated.
type RefStressObjectPutAllOf1AllOf3NullableRequired struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3NullableRequired)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf3NullableRequired) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf1AllOf3OptionalShared is generated.
type RefStressObjectPutAllOf1AllOf3OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3OptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf3OptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf1AllOf3SharedName is generated.
type RefStressObjectPutAllOf1AllOf3SharedName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf1AllOf3SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf1AllOf3SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf2 is generated.
type RefStressObjectPutAllOf2 struct {
	RefStressObjectPutAllOf2AllOf1
	RefStressObjectPutAllOf2AllOf2
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf2)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf2{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectPutAllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectPutAllOf2) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf2AllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf2AllOf2)

	return fields
}

// RefStressObjectPutAllOf2AllOf1 is generated.
type RefStressObjectPutAllOf2AllOf1 struct {
	RefStressObjectPutAllOf2AllOf1AllOf1
	RefStressObjectPutAllOf2AllOf1AllOf2
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf1)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf2AllOf1{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectPutAllOf2AllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectPutAllOf2AllOf1) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf2AllOf1AllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf2AllOf1AllOf2)

	return fields
}

// RefStressObjectPutAllOf2AllOf1AllOf1 is generated.
type RefStressObjectPutAllOf2AllOf1AllOf1 struct {
	FinalCode      RefStressObjectPutAllOf2AllOf1AllOf1FinalCode
	Nested         *RefStressObjectPutAllOf2AllOf1AllOf1Nested
	OptionalShared *RefStressObjectPutAllOf2AllOf1AllOf1OptionalShared
	SharedName     RefStressObjectPutAllOf2AllOf1AllOf1SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf1AllOf1)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf2AllOf1AllOf1{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf2AllOf1AllOf1) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf2AllOf1AllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf2AllOf1AllOf1) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf2AllOf1AllOf1FinalCode is generated.
type RefStressObjectPutAllOf2AllOf1AllOf1FinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf1FinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf1AllOf1FinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf1AllOf1FinalCode", s)}
}

// RefStressObjectPutAllOf2AllOf1AllOf1Nested is generated.
type RefStressObjectPutAllOf2AllOf1AllOf1Nested struct {
	Leaf     *RefStressObjectPutAllOf2AllOf1AllOf1NestedLeaf
	SameName RefStressObjectPutAllOf2AllOf1AllOf1NestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf1AllOf1Nested)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf2AllOf1AllOf1Nested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf2AllOf1AllOf1Nested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf2AllOf1AllOf1Nested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf2AllOf1AllOf1Nested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf2AllOf1AllOf1NestedLeaf is generated.
type RefStressObjectPutAllOf2AllOf1AllOf1NestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf1NestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf1AllOf1NestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf1AllOf1NestedLeaf", s)}
}

// RefStressObjectPutAllOf2AllOf1AllOf1NestedSameName is generated.
type RefStressObjectPutAllOf2AllOf1AllOf1NestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf1NestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf1AllOf1NestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf2AllOf1AllOf1OptionalShared is generated.
type RefStressObjectPutAllOf2AllOf1AllOf1OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf1OptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf1AllOf1OptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf2AllOf1AllOf1SharedName is generated.
type RefStressObjectPutAllOf2AllOf1AllOf1SharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf1SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf1AllOf1SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf1AllOf1SharedName", s)}
}

// RefStressObjectPutAllOf2AllOf1AllOf2 is generated.
type RefStressObjectPutAllOf2AllOf1AllOf2 struct {
	Final    *RefStressObjectPutAllOf2AllOf1AllOf2Final
	Metadata RefStressObjectPutAllOf2AllOf1AllOf2Metadata
	RootFlag RefStressObjectPutAllOf2AllOf1AllOf2RootFlag
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf1AllOf2)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf2AllOf1AllOf2{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf2AllOf1AllOf2) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			optionalObjectProperty("final", &o.Final),
			requiredObjectProperty("metadata", &o.Metadata),
			requiredObjectProperty("rootFlag", &o.RootFlag),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf2AllOf1AllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf2AllOf1AllOf2) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"final",
			"metadata",
			"rootFlag",
		},
		[]bool{
			true,
			false,
			false,
		},
	)
}

// RefStressObjectPutAllOf2AllOf1AllOf2Final is generated.
type RefStressObjectPutAllOf2AllOf1AllOf2Final struct {
	FinalCode      RefStressObjectPutAllOf2AllOf1AllOf2FinalFinalCode
	Nested         *RefStressObjectPutAllOf2AllOf1AllOf2FinalNested
	OptionalShared *RefStressObjectPutAllOf2AllOf1AllOf2FinalOptionalShared
	SharedName     RefStressObjectPutAllOf2AllOf1AllOf2FinalSharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf1AllOf2Final)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf2AllOf1AllOf2Final{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf2AllOf1AllOf2Final) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf2AllOf1AllOf2Final) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf2AllOf1AllOf2Final) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf2AllOf1AllOf2FinalFinalCode is generated.
type RefStressObjectPutAllOf2AllOf1AllOf2FinalFinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf2FinalFinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf1AllOf2FinalFinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf1AllOf2FinalFinalCode", s)}
}

// RefStressObjectPutAllOf2AllOf1AllOf2FinalNested is generated.
type RefStressObjectPutAllOf2AllOf1AllOf2FinalNested struct {
	Leaf     *RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedLeaf
	SameName RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf1AllOf2FinalNested)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf2AllOf1AllOf2FinalNested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf2AllOf1AllOf2FinalNested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf2AllOf1AllOf2FinalNested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf2AllOf1AllOf2FinalNested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedLeaf is generated.
type RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedLeaf", s)}
}

// RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedSameName is generated.
type RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf1AllOf2FinalNestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf2AllOf1AllOf2FinalOptionalShared is generated.
type RefStressObjectPutAllOf2AllOf1AllOf2FinalOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf2FinalOptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf1AllOf2FinalOptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf2AllOf1AllOf2FinalSharedName is generated.
type RefStressObjectPutAllOf2AllOf1AllOf2FinalSharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf2FinalSharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf1AllOf2FinalSharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf1AllOf2FinalSharedName", s)}
}

// RefStressObjectPutAllOf2AllOf1AllOf2Metadata is generated.
type RefStressObjectPutAllOf2AllOf1AllOf2Metadata struct{}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf1AllOf2Metadata)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf2AllOf1AllOf2Metadata{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf2AllOf1AllOf2Metadata) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{},
		true,
		func(value []byte) error {
			var additionalProperty RefStressObjectPutAllOf2AllOf1AllOf2MetadataAdditionalProperty

			return json.Unmarshal(value, &additionalProperty)
		},
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf2AllOf1AllOf2Metadata) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf2AllOf1AllOf2Metadata) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{},
		[]bool{},
	)
}

// RefStressObjectPutAllOf2AllOf1AllOf2MetadataAdditionalProperty is generated.
type RefStressObjectPutAllOf2AllOf1AllOf2MetadataAdditionalProperty string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf2MetadataAdditionalProperty)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf1AllOf2MetadataAdditionalProperty) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf1AllOf2MetadataAdditionalProperty", s)}
}

// RefStressObjectPutAllOf2AllOf1AllOf2RootFlag is generated.
type RefStressObjectPutAllOf2AllOf1AllOf2RootFlag bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf1AllOf2RootFlag)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectPutAllOf2AllOf1AllOf2RootFlag) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf1AllOf2RootFlag", b)}
}

// RefStressObjectPutAllOf2AllOf2 is generated.
type RefStressObjectPutAllOf2AllOf2 struct {
	Count      RefStressObjectPutAllOf2AllOf2Count
	Finals     RefStressObjectPutAllOf2AllOf2Finals
	Metadata   RefStressObjectPutAllOf2AllOf2Metadata
	RootFlag   RefStressObjectPutAllOf2AllOf2RootFlag
	SharedName *RefStressObjectPutAllOf2AllOf2SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf2)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf2AllOf2{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf2AllOf2) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("count", &o.Count),
			requiredObjectProperty("finals", &o.Finals),
			requiredObjectProperty("metadata", &o.Metadata),
			requiredObjectProperty("rootFlag", &o.RootFlag),
			optionalObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf2AllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf2AllOf2) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"count",
			"finals",
			"metadata",
			"rootFlag",
			"sharedName",
		},
		[]bool{
			false,
			false,
			false,
			false,
			true,
		},
	)
}

// RefStressObjectPutAllOf2AllOf2Count is generated.
type RefStressObjectPutAllOf2AllOf2Count json.Number

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2Count)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (n RefStressObjectPutAllOf2AllOf2Count) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf2Count", n)}
}

// RefStressObjectPutAllOf2AllOf2Finals is generated.
type RefStressObjectPutAllOf2AllOf2Finals []RefStressObjectPutAllOf2AllOf2FinalsItem

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2Finals)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (a RefStressObjectPutAllOf2AllOf2Finals) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf2Finals", a)}
}

// RefStressObjectPutAllOf2AllOf2FinalsItem is generated.
type RefStressObjectPutAllOf2AllOf2FinalsItem struct {
	FinalCode      RefStressObjectPutAllOf2AllOf2FinalsItemFinalCode
	Nested         *RefStressObjectPutAllOf2AllOf2FinalsItemNested
	OptionalShared *RefStressObjectPutAllOf2AllOf2FinalsItemOptionalShared
	SharedName     RefStressObjectPutAllOf2AllOf2FinalsItemSharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf2FinalsItem)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf2AllOf2FinalsItem{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf2AllOf2FinalsItem) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf2AllOf2FinalsItem) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf2AllOf2FinalsItem) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf2AllOf2FinalsItemFinalCode is generated.
type RefStressObjectPutAllOf2AllOf2FinalsItemFinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2FinalsItemFinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf2FinalsItemFinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf2FinalsItemFinalCode", s)}
}

// RefStressObjectPutAllOf2AllOf2FinalsItemNested is generated.
type RefStressObjectPutAllOf2AllOf2FinalsItemNested struct {
	Leaf     *RefStressObjectPutAllOf2AllOf2FinalsItemNestedLeaf
	SameName RefStressObjectPutAllOf2AllOf2FinalsItemNestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf2FinalsItemNested)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf2AllOf2FinalsItemNested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf2AllOf2FinalsItemNested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf2AllOf2FinalsItemNested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf2AllOf2FinalsItemNested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf2AllOf2FinalsItemNestedLeaf is generated.
type RefStressObjectPutAllOf2AllOf2FinalsItemNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2FinalsItemNestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf2FinalsItemNestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf2FinalsItemNestedLeaf", s)}
}

// RefStressObjectPutAllOf2AllOf2FinalsItemNestedSameName is generated.
type RefStressObjectPutAllOf2AllOf2FinalsItemNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2FinalsItemNestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf2FinalsItemNestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf2AllOf2FinalsItemOptionalShared is generated.
type RefStressObjectPutAllOf2AllOf2FinalsItemOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2FinalsItemOptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf2FinalsItemOptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf2AllOf2FinalsItemSharedName is generated.
type RefStressObjectPutAllOf2AllOf2FinalsItemSharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2FinalsItemSharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf2FinalsItemSharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf2FinalsItemSharedName", s)}
}

// RefStressObjectPutAllOf2AllOf2Metadata is generated.
type RefStressObjectPutAllOf2AllOf2Metadata struct{}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf2AllOf2Metadata)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf2AllOf2Metadata{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf2AllOf2Metadata) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{},
		true,
		func(value []byte) error {
			var additionalProperty RefStressObjectPutAllOf2AllOf2MetadataAdditionalProperty

			return json.Unmarshal(value, &additionalProperty)
		},
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf2AllOf2Metadata) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf2AllOf2Metadata) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{},
		[]bool{},
	)
}

// RefStressObjectPutAllOf2AllOf2MetadataAdditionalProperty is generated.
type RefStressObjectPutAllOf2AllOf2MetadataAdditionalProperty string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2MetadataAdditionalProperty)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf2MetadataAdditionalProperty) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf2MetadataAdditionalProperty", s)}
}

// RefStressObjectPutAllOf2AllOf2RootFlag is generated.
type RefStressObjectPutAllOf2AllOf2RootFlag bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2RootFlag)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectPutAllOf2AllOf2RootFlag) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf2RootFlag", b)}
}

// RefStressObjectPutAllOf2AllOf2SharedName is generated.
type RefStressObjectPutAllOf2AllOf2SharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf2AllOf2SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf2AllOf2SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf2AllOf2SharedName", s)}
}

// RefStressObjectPutAllOf3 is generated.
type RefStressObjectPutAllOf3 struct {
	Count            RefStressObjectPutAllOf3Count
	Final            RefStressObjectPutAllOf3Final
	FinalCode        RefStressObjectPutAllOf3FinalCode
	Finals           RefStressObjectPutAllOf3Finals
	Metadata         RefStressObjectPutAllOf3Metadata
	MiddleFlag       RefStressObjectPutAllOf3MiddleFlag
	Nested           RefStressObjectPutAllOf3Nested
	NullableRequired RefStressObjectPutAllOf3NullableRequired
	OptionalCode     *RefStressObjectPutAllOf3OptionalCode
	OptionalShared   *RefStressObjectPutAllOf3OptionalShared
	RootFlag         RefStressObjectPutAllOf3RootFlag
	SharedName       RefStressObjectPutAllOf3SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf3)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf3{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf3) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("count", &o.Count),
			requiredObjectProperty("final", &o.Final),
			requiredObjectProperty("finalCode", &o.FinalCode),
			requiredObjectProperty("finals", &o.Finals),
			requiredObjectProperty("metadata", &o.Metadata),
			requiredObjectProperty("middleFlag", &o.MiddleFlag),
			requiredObjectProperty("nested", &o.Nested),
			requiredObjectProperty("nullableRequired", &o.NullableRequired),
			optionalObjectProperty("optionalCode", &o.OptionalCode),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("rootFlag", &o.RootFlag),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		false,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf3) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf3) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"count",
			"final",
			"finalCode",
			"finals",
			"metadata",
			"middleFlag",
			"nested",
			"nullableRequired",
			"optionalCode",
			"optionalShared",
			"rootFlag",
			"sharedName",
		},
		[]bool{
			false,
			false,
			false,
			false,
			false,
			false,
			false,
			false,
			true,
			true,
			false,
			false,
		},
	)
}

// RefStressObjectPutAllOf3Count is generated.
type RefStressObjectPutAllOf3Count json.Number

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3Count)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (n RefStressObjectPutAllOf3Count) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3Count", n)}
}

// RefStressObjectPutAllOf3Final is generated.
type RefStressObjectPutAllOf3Final struct {
	FinalCode      RefStressObjectPutAllOf3FinalFinalCode
	Nested         *RefStressObjectPutAllOf3FinalNested
	OptionalShared *RefStressObjectPutAllOf3FinalOptionalShared
	SharedName     RefStressObjectPutAllOf3FinalSharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf3Final)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf3Final{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf3Final) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf3Final) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf3Final) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf3FinalFinalCode is generated.
type RefStressObjectPutAllOf3FinalFinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalFinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3FinalFinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3FinalFinalCode", s)}
}

// RefStressObjectPutAllOf3FinalNested is generated.
type RefStressObjectPutAllOf3FinalNested struct {
	Leaf     *RefStressObjectPutAllOf3FinalNestedLeaf
	SameName RefStressObjectPutAllOf3FinalNestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf3FinalNested)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf3FinalNested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf3FinalNested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf3FinalNested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf3FinalNested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf3FinalNestedLeaf is generated.
type RefStressObjectPutAllOf3FinalNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalNestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3FinalNestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3FinalNestedLeaf", s)}
}

// RefStressObjectPutAllOf3FinalNestedSameName is generated.
type RefStressObjectPutAllOf3FinalNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalNestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3FinalNestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf3FinalOptionalShared is generated.
type RefStressObjectPutAllOf3FinalOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalOptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3FinalOptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf3FinalSharedName is generated.
type RefStressObjectPutAllOf3FinalSharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalSharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3FinalSharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3FinalSharedName", s)}
}

// RefStressObjectPutAllOf3FinalCode is generated.
type RefStressObjectPutAllOf3FinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3FinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3FinalCode", s)}
}

// RefStressObjectPutAllOf3Finals is generated.
type RefStressObjectPutAllOf3Finals []RefStressObjectPutAllOf3FinalsItem

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3Finals)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (a RefStressObjectPutAllOf3Finals) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3Finals", a)}
}

// RefStressObjectPutAllOf3FinalsItem is generated.
type RefStressObjectPutAllOf3FinalsItem struct {
	FinalCode      RefStressObjectPutAllOf3FinalsItemFinalCode
	Nested         *RefStressObjectPutAllOf3FinalsItemNested
	OptionalShared *RefStressObjectPutAllOf3FinalsItemOptionalShared
	SharedName     RefStressObjectPutAllOf3FinalsItemSharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf3FinalsItem)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf3FinalsItem{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf3FinalsItem) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf3FinalsItem) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf3FinalsItem) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf3FinalsItemFinalCode is generated.
type RefStressObjectPutAllOf3FinalsItemFinalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalsItemFinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3FinalsItemFinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3FinalsItemFinalCode", s)}
}

// RefStressObjectPutAllOf3FinalsItemNested is generated.
type RefStressObjectPutAllOf3FinalsItemNested struct {
	Leaf     *RefStressObjectPutAllOf3FinalsItemNestedLeaf
	SameName RefStressObjectPutAllOf3FinalsItemNestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf3FinalsItemNested)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf3FinalsItemNested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf3FinalsItemNested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf3FinalsItemNested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf3FinalsItemNested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf3FinalsItemNestedLeaf is generated.
type RefStressObjectPutAllOf3FinalsItemNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalsItemNestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3FinalsItemNestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3FinalsItemNestedLeaf", s)}
}

// RefStressObjectPutAllOf3FinalsItemNestedSameName is generated.
type RefStressObjectPutAllOf3FinalsItemNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalsItemNestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3FinalsItemNestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf3FinalsItemOptionalShared is generated.
type RefStressObjectPutAllOf3FinalsItemOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalsItemOptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3FinalsItemOptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf3FinalsItemSharedName is generated.
type RefStressObjectPutAllOf3FinalsItemSharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3FinalsItemSharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3FinalsItemSharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3FinalsItemSharedName", s)}
}

// RefStressObjectPutAllOf3Metadata is generated.
type RefStressObjectPutAllOf3Metadata struct{}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf3Metadata)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf3Metadata{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf3Metadata) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{},
		true,
		func(value []byte) error {
			var additionalProperty RefStressObjectPutAllOf3MetadataAdditionalProperty

			return json.Unmarshal(value, &additionalProperty)
		},
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf3Metadata) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf3Metadata) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{},
		[]bool{},
	)
}

// RefStressObjectPutAllOf3MetadataAdditionalProperty is generated.
type RefStressObjectPutAllOf3MetadataAdditionalProperty string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3MetadataAdditionalProperty)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3MetadataAdditionalProperty) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3MetadataAdditionalProperty", s)}
}

// RefStressObjectPutAllOf3MiddleFlag is generated.
type RefStressObjectPutAllOf3MiddleFlag bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3MiddleFlag)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectPutAllOf3MiddleFlag) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3MiddleFlag", b)}
}

// RefStressObjectPutAllOf3Nested is generated.
type RefStressObjectPutAllOf3Nested struct {
	RefStressObjectPutAllOf3NestedAllOf1
	RefStressObjectPutAllOf3NestedAllOf2
	RefStressObjectPutAllOf3NestedAllOf3
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf3Nested)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf3Nested{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectPutAllOf3Nested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectPutAllOf3Nested) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf3NestedAllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf3NestedAllOf2)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectPutAllOf3NestedAllOf3)

	return fields
}

// RefStressObjectPutAllOf3NestedAllOf1 is generated.
type RefStressObjectPutAllOf3NestedAllOf1 struct {
	Leaf     *RefStressObjectPutAllOf3NestedAllOf1Leaf
	SameName RefStressObjectPutAllOf3NestedAllOf1SameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf3NestedAllOf1)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf3NestedAllOf1{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf3NestedAllOf1) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf3NestedAllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf3NestedAllOf1) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf3NestedAllOf1Leaf is generated.
type RefStressObjectPutAllOf3NestedAllOf1Leaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3NestedAllOf1Leaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3NestedAllOf1Leaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3NestedAllOf1Leaf", s)}
}

// RefStressObjectPutAllOf3NestedAllOf1SameName is generated.
type RefStressObjectPutAllOf3NestedAllOf1SameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3NestedAllOf1SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3NestedAllOf1SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf3NestedAllOf2 is generated.
type RefStressObjectPutAllOf3NestedAllOf2 struct {
	Leaf     *RefStressObjectPutAllOf3NestedAllOf2Leaf
	SameName RefStressObjectPutAllOf3NestedAllOf2SameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf3NestedAllOf2)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf3NestedAllOf2{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf3NestedAllOf2) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf3NestedAllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf3NestedAllOf2) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectPutAllOf3NestedAllOf2Leaf is generated.
type RefStressObjectPutAllOf3NestedAllOf2Leaf string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3NestedAllOf2Leaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3NestedAllOf2Leaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3NestedAllOf2Leaf", s)}
}

// RefStressObjectPutAllOf3NestedAllOf2SameName is generated.
type RefStressObjectPutAllOf3NestedAllOf2SameName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3NestedAllOf2SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3NestedAllOf2SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3NestedAllOf2SameName", s)}
}

// RefStressObjectPutAllOf3NestedAllOf3 is generated.
type RefStressObjectPutAllOf3NestedAllOf3 struct {
	SameName RefStressObjectPutAllOf3NestedAllOf3SameName
	Sealed   RefStressObjectPutAllOf3NestedAllOf3Sealed
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf3NestedAllOf3)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf3NestedAllOf3{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf3NestedAllOf3) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("sameName", &o.SameName),
			requiredObjectProperty("sealed", &o.Sealed),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf3NestedAllOf3) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf3NestedAllOf3) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"sameName",
			"sealed",
		},
		[]bool{
			false,
			false,
		},
	)
}

// RefStressObjectPutAllOf3NestedAllOf3SameName is generated.
type RefStressObjectPutAllOf3NestedAllOf3SameName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3NestedAllOf3SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3NestedAllOf3SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3NestedAllOf3SameName", s)}
}

// RefStressObjectPutAllOf3NestedAllOf3Sealed is generated.
type RefStressObjectPutAllOf3NestedAllOf3Sealed struct {
	Locked RefStressObjectPutAllOf3NestedAllOf3SealedLocked
}

var (
	_ json.Unmarshaler = (*RefStressObjectPutAllOf3NestedAllOf3Sealed)(nil)
	_ json.Marshaler   = RefStressObjectPutAllOf3NestedAllOf3Sealed{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectPutAllOf3NestedAllOf3Sealed) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("locked", &o.Locked),
		},
		false,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectPutAllOf3NestedAllOf3Sealed) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectPutAllOf3NestedAllOf3Sealed) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"locked",
		},
		[]bool{
			false,
		},
	)
}

// RefStressObjectPutAllOf3NestedAllOf3SealedLocked is generated.
type RefStressObjectPutAllOf3NestedAllOf3SealedLocked bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3NestedAllOf3SealedLocked)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectPutAllOf3NestedAllOf3SealedLocked) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3NestedAllOf3SealedLocked", b)}
}

// RefStressObjectPutAllOf3NullableRequired is generated.
type RefStressObjectPutAllOf3NullableRequired struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3NullableRequired)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3NullableRequired) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf3OptionalCode is generated.
type RefStressObjectPutAllOf3OptionalCode string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3OptionalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3OptionalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3OptionalCode", s)}
}

// RefStressObjectPutAllOf3OptionalShared is generated.
type RefStressObjectPutAllOf3OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3OptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3OptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectPutAllOf3RootFlag is generated.
type RefStressObjectPutAllOf3RootFlag bool

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3RootFlag)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectPutAllOf3RootFlag) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3RootFlag", b)}
}

// RefStressObjectPutAllOf3SharedName is generated.
type RefStressObjectPutAllOf3SharedName string

var _ json.Unmarshaler = new(RefStressObjectPutAllOf3SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectPutAllOf3SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectPutAllOf3SharedName", s)}
}

// RefStressObject is generated.
type RefStressObject struct {
	RefStressObjectAllOf1
	RefStressObjectAllOf2
	RefStressObjectAllOf3
}

var (
	_ json.Unmarshaler = (*RefStressObject)(nil)
	_ json.Marshaler   = RefStressObject{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObject) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObject) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf2)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf3)

	return fields
}

// RefStressObjectAllOf1 is generated.
type RefStressObjectAllOf1 struct {
	RefStressObjectAllOf1AllOf1
	RefStressObjectAllOf1AllOf2
	RefStressObjectAllOf1AllOf3
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectAllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectAllOf1) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf1AllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf1AllOf2)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf1AllOf3)

	return fields
}

// RefStressObjectAllOf1AllOf1 is generated.
type RefStressObjectAllOf1AllOf1 struct {
	FinalCode      RefStressObjectAllOf1AllOf1FinalCode
	Nested         *RefStressObjectAllOf1AllOf1Nested
	OptionalShared *RefStressObjectAllOf1AllOf1OptionalShared
	SharedName     RefStressObjectAllOf1AllOf1SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf1)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf1{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf1) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf1) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf1FinalCode is generated.
type RefStressObjectAllOf1AllOf1FinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf1FinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf1FinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf1FinalCode", s)}
}

// RefStressObjectAllOf1AllOf1Nested is generated.
type RefStressObjectAllOf1AllOf1Nested struct {
	Leaf     *RefStressObjectAllOf1AllOf1NestedLeaf
	SameName RefStressObjectAllOf1AllOf1NestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf1Nested)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf1Nested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf1Nested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf1Nested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf1Nested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf1NestedLeaf is generated.
type RefStressObjectAllOf1AllOf1NestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf1NestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf1NestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf1NestedLeaf", s)}
}

// RefStressObjectAllOf1AllOf1NestedSameName is generated.
type RefStressObjectAllOf1AllOf1NestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf1NestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf1NestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf1AllOf1OptionalShared is generated.
type RefStressObjectAllOf1AllOf1OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf1OptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf1OptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf1AllOf1SharedName is generated.
type RefStressObjectAllOf1AllOf1SharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf1SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf1SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf1SharedName", s)}
}

// RefStressObjectAllOf1AllOf2 is generated.
type RefStressObjectAllOf1AllOf2 struct {
	RefStressObjectAllOf1AllOf2AllOf1
	RefStressObjectAllOf1AllOf2AllOf2
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf2{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectAllOf1AllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectAllOf1AllOf2) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf1AllOf2AllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf1AllOf2AllOf2)

	return fields
}

// RefStressObjectAllOf1AllOf2AllOf1 is generated.
type RefStressObjectAllOf1AllOf2AllOf1 struct {
	RefStressObjectAllOf1AllOf2AllOf1AllOf1
	RefStressObjectAllOf1AllOf2AllOf1AllOf2
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf1)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf2AllOf1{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectAllOf1AllOf2AllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectAllOf1AllOf2AllOf1) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf1AllOf2AllOf1AllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf1AllOf2AllOf1AllOf2)

	return fields
}

// RefStressObjectAllOf1AllOf2AllOf1AllOf1 is generated.
type RefStressObjectAllOf1AllOf2AllOf1AllOf1 struct {
	FinalCode      RefStressObjectAllOf1AllOf2AllOf1AllOf1FinalCode
	Nested         *RefStressObjectAllOf1AllOf2AllOf1AllOf1Nested
	OptionalShared *RefStressObjectAllOf1AllOf2AllOf1AllOf1OptionalShared
	SharedName     RefStressObjectAllOf1AllOf2AllOf1AllOf1SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf1AllOf1)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf2AllOf1AllOf1{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf2AllOf1AllOf1) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf2AllOf1AllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf2AllOf1AllOf1) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf2AllOf1AllOf1FinalCode is generated.
type RefStressObjectAllOf1AllOf2AllOf1AllOf1FinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf1AllOf1FinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf2AllOf1AllOf1FinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf2AllOf1AllOf1FinalCode", s)}
}

// RefStressObjectAllOf1AllOf2AllOf1AllOf1Nested is generated.
type RefStressObjectAllOf1AllOf2AllOf1AllOf1Nested struct {
	Leaf     *RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedLeaf
	SameName RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf1AllOf1Nested)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf2AllOf1AllOf1Nested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf2AllOf1AllOf1Nested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf2AllOf1AllOf1Nested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf2AllOf1AllOf1Nested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedLeaf is generated.
type RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedLeaf", s)}
}

// RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedSameName is generated.
type RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf2AllOf1AllOf1NestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf1AllOf2AllOf1AllOf1OptionalShared is generated.
type RefStressObjectAllOf1AllOf2AllOf1AllOf1OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf1AllOf1OptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf2AllOf1AllOf1OptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf1AllOf2AllOf1AllOf1SharedName is generated.
type RefStressObjectAllOf1AllOf2AllOf1AllOf1SharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf1AllOf1SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf2AllOf1AllOf1SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf2AllOf1AllOf1SharedName", s)}
}

// RefStressObjectAllOf1AllOf2AllOf1AllOf2 is generated.
type RefStressObjectAllOf1AllOf2AllOf1AllOf2 struct {
	OptionalCode *RefStressObjectAllOf1AllOf2AllOf1AllOf2OptionalCode
	SharedName   RefStressObjectAllOf1AllOf2AllOf1AllOf2SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf1AllOf2)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf2AllOf1AllOf2{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf2AllOf1AllOf2) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("optionalCode", &o.OptionalCode),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf2AllOf1AllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf2AllOf1AllOf2) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"optionalCode",
			"sharedName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf2AllOf1AllOf2OptionalCode is generated.
type RefStressObjectAllOf1AllOf2AllOf1AllOf2OptionalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf1AllOf2OptionalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf2AllOf1AllOf2OptionalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf2AllOf1AllOf2OptionalCode", s)}
}

// RefStressObjectAllOf1AllOf2AllOf1AllOf2SharedName is generated.
type RefStressObjectAllOf1AllOf2AllOf1AllOf2SharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf1AllOf2SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf2AllOf1AllOf2SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf2AllOf1AllOf2SharedName", s)}
}

// RefStressObjectAllOf1AllOf2AllOf2 is generated.
type RefStressObjectAllOf1AllOf2AllOf2 struct {
	MiddleFlag RefStressObjectAllOf1AllOf2AllOf2MiddleFlag
	Nested     *RefStressObjectAllOf1AllOf2AllOf2Nested
	SharedName RefStressObjectAllOf1AllOf2AllOf2SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf2)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf2AllOf2{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf2AllOf2) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("middleFlag", &o.MiddleFlag),
			optionalObjectProperty("nested", &o.Nested),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf2AllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf2AllOf2) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"middleFlag",
			"nested",
			"sharedName",
		},
		[]bool{
			false,
			true,
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf2AllOf2MiddleFlag is generated.
type RefStressObjectAllOf1AllOf2AllOf2MiddleFlag bool

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2MiddleFlag)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectAllOf1AllOf2AllOf2MiddleFlag) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf2AllOf2MiddleFlag", b)}
}

// RefStressObjectAllOf1AllOf2AllOf2Nested is generated.
type RefStressObjectAllOf1AllOf2AllOf2Nested struct {
	RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1
	RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2
	RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf2Nested)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf2AllOf2Nested{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectAllOf1AllOf2AllOf2Nested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectAllOf1AllOf2AllOf2Nested) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3)

	return fields
}

// RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1 is generated.
type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1 struct {
	Leaf     *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1Leaf
	SameName RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1SameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1Leaf is generated.
type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1Leaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1Leaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1Leaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1Leaf", s)}
}

// RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1SameName is generated.
type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1SameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf2AllOf2NestedAllOf1SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2 is generated.
type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2 struct {
	Leaf     *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2Leaf
	SameName RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2SameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2Leaf is generated.
type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2Leaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2Leaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2Leaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2Leaf", s)}
}

// RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2SameName is generated.
type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2SameName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf2AllOf2NestedAllOf2SameName", s)}
}

// RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3 is generated.
type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3 struct {
	SameName RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SameName
	Sealed   RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3Sealed
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("sameName", &o.SameName),
			requiredObjectProperty("sealed", &o.Sealed),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"sameName",
			"sealed",
		},
		[]bool{
			false,
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SameName is generated.
type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SameName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SameName", s)}
}

// RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3Sealed is generated.
type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3Sealed struct {
	Locked RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SealedLocked
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3Sealed)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3Sealed{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3Sealed) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("locked", &o.Locked),
		},
		false,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3Sealed) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3Sealed) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"locked",
		},
		[]bool{
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SealedLocked is generated.
type RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SealedLocked bool

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SealedLocked)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SealedLocked) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf2AllOf2NestedAllOf3SealedLocked", b)}
}

// RefStressObjectAllOf1AllOf2AllOf2SharedName is generated.
type RefStressObjectAllOf1AllOf2AllOf2SharedName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf2AllOf2SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf2AllOf2SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf1AllOf3 is generated.
type RefStressObjectAllOf1AllOf3 struct {
	Final            RefStressObjectAllOf1AllOf3Final
	Nested           RefStressObjectAllOf1AllOf3Nested
	NullableRequired RefStressObjectAllOf1AllOf3NullableRequired
	OptionalShared   *RefStressObjectAllOf1AllOf3OptionalShared
	SharedName       *RefStressObjectAllOf1AllOf3SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf3{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf3) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("final", &o.Final),
			requiredObjectProperty("nested", &o.Nested),
			requiredObjectProperty("nullableRequired", &o.NullableRequired),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			optionalObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf3) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf3) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"final",
			"nested",
			"nullableRequired",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			false,
			false,
			true,
			true,
		},
	)
}

// RefStressObjectAllOf1AllOf3Final is generated.
type RefStressObjectAllOf1AllOf3Final struct {
	FinalCode      RefStressObjectAllOf1AllOf3FinalFinalCode
	Nested         *RefStressObjectAllOf1AllOf3FinalNested
	OptionalShared *RefStressObjectAllOf1AllOf3FinalOptionalShared
	SharedName     RefStressObjectAllOf1AllOf3FinalSharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3Final)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf3Final{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf3Final) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf3Final) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf3Final) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf3FinalFinalCode is generated.
type RefStressObjectAllOf1AllOf3FinalFinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3FinalFinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf3FinalFinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf3FinalFinalCode", s)}
}

// RefStressObjectAllOf1AllOf3FinalNested is generated.
type RefStressObjectAllOf1AllOf3FinalNested struct {
	Leaf     *RefStressObjectAllOf1AllOf3FinalNestedLeaf
	SameName RefStressObjectAllOf1AllOf3FinalNestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3FinalNested)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf3FinalNested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf3FinalNested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf3FinalNested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf3FinalNested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf3FinalNestedLeaf is generated.
type RefStressObjectAllOf1AllOf3FinalNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3FinalNestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf3FinalNestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf3FinalNestedLeaf", s)}
}

// RefStressObjectAllOf1AllOf3FinalNestedSameName is generated.
type RefStressObjectAllOf1AllOf3FinalNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3FinalNestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf3FinalNestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf1AllOf3FinalOptionalShared is generated.
type RefStressObjectAllOf1AllOf3FinalOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3FinalOptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf3FinalOptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf1AllOf3FinalSharedName is generated.
type RefStressObjectAllOf1AllOf3FinalSharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3FinalSharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf3FinalSharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf3FinalSharedName", s)}
}

// RefStressObjectAllOf1AllOf3Nested is generated.
type RefStressObjectAllOf1AllOf3Nested struct {
	RefStressObjectAllOf1AllOf3NestedAllOf1
	RefStressObjectAllOf1AllOf3NestedAllOf2
	RefStressObjectAllOf1AllOf3NestedAllOf3
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3Nested)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf3Nested{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectAllOf1AllOf3Nested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectAllOf1AllOf3Nested) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf1AllOf3NestedAllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf1AllOf3NestedAllOf2)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf1AllOf3NestedAllOf3)

	return fields
}

// RefStressObjectAllOf1AllOf3NestedAllOf1 is generated.
type RefStressObjectAllOf1AllOf3NestedAllOf1 struct {
	Leaf     *RefStressObjectAllOf1AllOf3NestedAllOf1Leaf
	SameName RefStressObjectAllOf1AllOf3NestedAllOf1SameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3NestedAllOf1)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf3NestedAllOf1{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf3NestedAllOf1) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf3NestedAllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf3NestedAllOf1) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf3NestedAllOf1Leaf is generated.
type RefStressObjectAllOf1AllOf3NestedAllOf1Leaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3NestedAllOf1Leaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf3NestedAllOf1Leaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf3NestedAllOf1Leaf", s)}
}

// RefStressObjectAllOf1AllOf3NestedAllOf1SameName is generated.
type RefStressObjectAllOf1AllOf3NestedAllOf1SameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3NestedAllOf1SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf3NestedAllOf1SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf1AllOf3NestedAllOf2 is generated.
type RefStressObjectAllOf1AllOf3NestedAllOf2 struct {
	Leaf     *RefStressObjectAllOf1AllOf3NestedAllOf2Leaf
	SameName RefStressObjectAllOf1AllOf3NestedAllOf2SameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3NestedAllOf2)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf3NestedAllOf2{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf3NestedAllOf2) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf3NestedAllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf3NestedAllOf2) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf3NestedAllOf2Leaf is generated.
type RefStressObjectAllOf1AllOf3NestedAllOf2Leaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3NestedAllOf2Leaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf3NestedAllOf2Leaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf3NestedAllOf2Leaf", s)}
}

// RefStressObjectAllOf1AllOf3NestedAllOf2SameName is generated.
type RefStressObjectAllOf1AllOf3NestedAllOf2SameName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3NestedAllOf2SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf3NestedAllOf2SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf3NestedAllOf2SameName", s)}
}

// RefStressObjectAllOf1AllOf3NestedAllOf3 is generated.
type RefStressObjectAllOf1AllOf3NestedAllOf3 struct {
	SameName RefStressObjectAllOf1AllOf3NestedAllOf3SameName
	Sealed   RefStressObjectAllOf1AllOf3NestedAllOf3Sealed
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3NestedAllOf3)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf3NestedAllOf3{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf3NestedAllOf3) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("sameName", &o.SameName),
			requiredObjectProperty("sealed", &o.Sealed),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf3NestedAllOf3) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf3NestedAllOf3) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"sameName",
			"sealed",
		},
		[]bool{
			false,
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf3NestedAllOf3SameName is generated.
type RefStressObjectAllOf1AllOf3NestedAllOf3SameName string

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3NestedAllOf3SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf3NestedAllOf3SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf3NestedAllOf3SameName", s)}
}

// RefStressObjectAllOf1AllOf3NestedAllOf3Sealed is generated.
type RefStressObjectAllOf1AllOf3NestedAllOf3Sealed struct {
	Locked RefStressObjectAllOf1AllOf3NestedAllOf3SealedLocked
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf1AllOf3NestedAllOf3Sealed)(nil)
	_ json.Marshaler   = RefStressObjectAllOf1AllOf3NestedAllOf3Sealed{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf1AllOf3NestedAllOf3Sealed) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("locked", &o.Locked),
		},
		false,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf1AllOf3NestedAllOf3Sealed) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf1AllOf3NestedAllOf3Sealed) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"locked",
		},
		[]bool{
			false,
		},
	)
}

// RefStressObjectAllOf1AllOf3NestedAllOf3SealedLocked is generated.
type RefStressObjectAllOf1AllOf3NestedAllOf3SealedLocked bool

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3NestedAllOf3SealedLocked)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectAllOf1AllOf3NestedAllOf3SealedLocked) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf1AllOf3NestedAllOf3SealedLocked", b)}
}

// RefStressObjectAllOf1AllOf3NullableRequired is generated.
type RefStressObjectAllOf1AllOf3NullableRequired struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3NullableRequired)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf3NullableRequired) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf1AllOf3OptionalShared is generated.
type RefStressObjectAllOf1AllOf3OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3OptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf3OptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf1AllOf3SharedName is generated.
type RefStressObjectAllOf1AllOf3SharedName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf1AllOf3SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf1AllOf3SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf2 is generated.
type RefStressObjectAllOf2 struct {
	RefStressObjectAllOf2AllOf1
	RefStressObjectAllOf2AllOf2
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf2)(nil)
	_ json.Marshaler   = RefStressObjectAllOf2{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectAllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectAllOf2) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf2AllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf2AllOf2)

	return fields
}

// RefStressObjectAllOf2AllOf1 is generated.
type RefStressObjectAllOf2AllOf1 struct {
	RefStressObjectAllOf2AllOf1AllOf1
	RefStressObjectAllOf2AllOf1AllOf2
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf2AllOf1)(nil)
	_ json.Marshaler   = RefStressObjectAllOf2AllOf1{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectAllOf2AllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectAllOf2AllOf1) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf2AllOf1AllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf2AllOf1AllOf2)

	return fields
}

// RefStressObjectAllOf2AllOf1AllOf1 is generated.
type RefStressObjectAllOf2AllOf1AllOf1 struct {
	FinalCode      RefStressObjectAllOf2AllOf1AllOf1FinalCode
	Nested         *RefStressObjectAllOf2AllOf1AllOf1Nested
	OptionalShared *RefStressObjectAllOf2AllOf1AllOf1OptionalShared
	SharedName     RefStressObjectAllOf2AllOf1AllOf1SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf2AllOf1AllOf1)(nil)
	_ json.Marshaler   = RefStressObjectAllOf2AllOf1AllOf1{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf2AllOf1AllOf1) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf2AllOf1AllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf2AllOf1AllOf1) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectAllOf2AllOf1AllOf1FinalCode is generated.
type RefStressObjectAllOf2AllOf1AllOf1FinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf1FinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf1AllOf1FinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf1AllOf1FinalCode", s)}
}

// RefStressObjectAllOf2AllOf1AllOf1Nested is generated.
type RefStressObjectAllOf2AllOf1AllOf1Nested struct {
	Leaf     *RefStressObjectAllOf2AllOf1AllOf1NestedLeaf
	SameName RefStressObjectAllOf2AllOf1AllOf1NestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf2AllOf1AllOf1Nested)(nil)
	_ json.Marshaler   = RefStressObjectAllOf2AllOf1AllOf1Nested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf2AllOf1AllOf1Nested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf2AllOf1AllOf1Nested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf2AllOf1AllOf1Nested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectAllOf2AllOf1AllOf1NestedLeaf is generated.
type RefStressObjectAllOf2AllOf1AllOf1NestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf1NestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf1AllOf1NestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf1AllOf1NestedLeaf", s)}
}

// RefStressObjectAllOf2AllOf1AllOf1NestedSameName is generated.
type RefStressObjectAllOf2AllOf1AllOf1NestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf1NestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf1AllOf1NestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf2AllOf1AllOf1OptionalShared is generated.
type RefStressObjectAllOf2AllOf1AllOf1OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf1OptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf1AllOf1OptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf2AllOf1AllOf1SharedName is generated.
type RefStressObjectAllOf2AllOf1AllOf1SharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf1SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf1AllOf1SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf1AllOf1SharedName", s)}
}

// RefStressObjectAllOf2AllOf1AllOf2 is generated.
type RefStressObjectAllOf2AllOf1AllOf2 struct {
	Final    *RefStressObjectAllOf2AllOf1AllOf2Final
	Metadata RefStressObjectAllOf2AllOf1AllOf2Metadata
	RootFlag RefStressObjectAllOf2AllOf1AllOf2RootFlag
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf2AllOf1AllOf2)(nil)
	_ json.Marshaler   = RefStressObjectAllOf2AllOf1AllOf2{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf2AllOf1AllOf2) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			optionalObjectProperty("final", &o.Final),
			requiredObjectProperty("metadata", &o.Metadata),
			requiredObjectProperty("rootFlag", &o.RootFlag),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf2AllOf1AllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf2AllOf1AllOf2) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"final",
			"metadata",
			"rootFlag",
		},
		[]bool{
			true,
			false,
			false,
		},
	)
}

// RefStressObjectAllOf2AllOf1AllOf2Final is generated.
type RefStressObjectAllOf2AllOf1AllOf2Final struct {
	FinalCode      RefStressObjectAllOf2AllOf1AllOf2FinalFinalCode
	Nested         *RefStressObjectAllOf2AllOf1AllOf2FinalNested
	OptionalShared *RefStressObjectAllOf2AllOf1AllOf2FinalOptionalShared
	SharedName     RefStressObjectAllOf2AllOf1AllOf2FinalSharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf2AllOf1AllOf2Final)(nil)
	_ json.Marshaler   = RefStressObjectAllOf2AllOf1AllOf2Final{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf2AllOf1AllOf2Final) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf2AllOf1AllOf2Final) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf2AllOf1AllOf2Final) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectAllOf2AllOf1AllOf2FinalFinalCode is generated.
type RefStressObjectAllOf2AllOf1AllOf2FinalFinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf2FinalFinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf1AllOf2FinalFinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf1AllOf2FinalFinalCode", s)}
}

// RefStressObjectAllOf2AllOf1AllOf2FinalNested is generated.
type RefStressObjectAllOf2AllOf1AllOf2FinalNested struct {
	Leaf     *RefStressObjectAllOf2AllOf1AllOf2FinalNestedLeaf
	SameName RefStressObjectAllOf2AllOf1AllOf2FinalNestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf2AllOf1AllOf2FinalNested)(nil)
	_ json.Marshaler   = RefStressObjectAllOf2AllOf1AllOf2FinalNested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf2AllOf1AllOf2FinalNested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf2AllOf1AllOf2FinalNested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf2AllOf1AllOf2FinalNested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectAllOf2AllOf1AllOf2FinalNestedLeaf is generated.
type RefStressObjectAllOf2AllOf1AllOf2FinalNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf2FinalNestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf1AllOf2FinalNestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf1AllOf2FinalNestedLeaf", s)}
}

// RefStressObjectAllOf2AllOf1AllOf2FinalNestedSameName is generated.
type RefStressObjectAllOf2AllOf1AllOf2FinalNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf2FinalNestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf1AllOf2FinalNestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf2AllOf1AllOf2FinalOptionalShared is generated.
type RefStressObjectAllOf2AllOf1AllOf2FinalOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf2FinalOptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf1AllOf2FinalOptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf2AllOf1AllOf2FinalSharedName is generated.
type RefStressObjectAllOf2AllOf1AllOf2FinalSharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf2FinalSharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf1AllOf2FinalSharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf1AllOf2FinalSharedName", s)}
}

// RefStressObjectAllOf2AllOf1AllOf2Metadata is generated.
type RefStressObjectAllOf2AllOf1AllOf2Metadata struct{}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf2AllOf1AllOf2Metadata)(nil)
	_ json.Marshaler   = RefStressObjectAllOf2AllOf1AllOf2Metadata{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf2AllOf1AllOf2Metadata) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{},
		true,
		func(value []byte) error {
			var additionalProperty RefStressObjectAllOf2AllOf1AllOf2MetadataAdditionalProperty

			return json.Unmarshal(value, &additionalProperty)
		},
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf2AllOf1AllOf2Metadata) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf2AllOf1AllOf2Metadata) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{},
		[]bool{},
	)
}

// RefStressObjectAllOf2AllOf1AllOf2MetadataAdditionalProperty is generated.
type RefStressObjectAllOf2AllOf1AllOf2MetadataAdditionalProperty string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf2MetadataAdditionalProperty)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf1AllOf2MetadataAdditionalProperty) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf1AllOf2MetadataAdditionalProperty", s)}
}

// RefStressObjectAllOf2AllOf1AllOf2RootFlag is generated.
type RefStressObjectAllOf2AllOf1AllOf2RootFlag bool

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf1AllOf2RootFlag)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectAllOf2AllOf1AllOf2RootFlag) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf1AllOf2RootFlag", b)}
}

// RefStressObjectAllOf2AllOf2 is generated.
type RefStressObjectAllOf2AllOf2 struct {
	Count      RefStressObjectAllOf2AllOf2Count
	Finals     RefStressObjectAllOf2AllOf2Finals
	Metadata   RefStressObjectAllOf2AllOf2Metadata
	RootFlag   RefStressObjectAllOf2AllOf2RootFlag
	SharedName *RefStressObjectAllOf2AllOf2SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf2AllOf2)(nil)
	_ json.Marshaler   = RefStressObjectAllOf2AllOf2{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf2AllOf2) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("count", &o.Count),
			requiredObjectProperty("finals", &o.Finals),
			requiredObjectProperty("metadata", &o.Metadata),
			requiredObjectProperty("rootFlag", &o.RootFlag),
			optionalObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf2AllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf2AllOf2) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"count",
			"finals",
			"metadata",
			"rootFlag",
			"sharedName",
		},
		[]bool{
			false,
			false,
			false,
			false,
			true,
		},
	)
}

// RefStressObjectAllOf2AllOf2Count is generated.
type RefStressObjectAllOf2AllOf2Count json.Number

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2Count)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (n RefStressObjectAllOf2AllOf2Count) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf2Count", n)}
}

// RefStressObjectAllOf2AllOf2Finals is generated.
type RefStressObjectAllOf2AllOf2Finals []RefStressObjectAllOf2AllOf2FinalsItem

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2Finals)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (a RefStressObjectAllOf2AllOf2Finals) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf2Finals", a)}
}

// RefStressObjectAllOf2AllOf2FinalsItem is generated.
type RefStressObjectAllOf2AllOf2FinalsItem struct {
	FinalCode      RefStressObjectAllOf2AllOf2FinalsItemFinalCode
	Nested         *RefStressObjectAllOf2AllOf2FinalsItemNested
	OptionalShared *RefStressObjectAllOf2AllOf2FinalsItemOptionalShared
	SharedName     RefStressObjectAllOf2AllOf2FinalsItemSharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf2AllOf2FinalsItem)(nil)
	_ json.Marshaler   = RefStressObjectAllOf2AllOf2FinalsItem{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf2AllOf2FinalsItem) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf2AllOf2FinalsItem) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf2AllOf2FinalsItem) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectAllOf2AllOf2FinalsItemFinalCode is generated.
type RefStressObjectAllOf2AllOf2FinalsItemFinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2FinalsItemFinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf2FinalsItemFinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf2FinalsItemFinalCode", s)}
}

// RefStressObjectAllOf2AllOf2FinalsItemNested is generated.
type RefStressObjectAllOf2AllOf2FinalsItemNested struct {
	Leaf     *RefStressObjectAllOf2AllOf2FinalsItemNestedLeaf
	SameName RefStressObjectAllOf2AllOf2FinalsItemNestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf2AllOf2FinalsItemNested)(nil)
	_ json.Marshaler   = RefStressObjectAllOf2AllOf2FinalsItemNested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf2AllOf2FinalsItemNested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf2AllOf2FinalsItemNested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf2AllOf2FinalsItemNested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectAllOf2AllOf2FinalsItemNestedLeaf is generated.
type RefStressObjectAllOf2AllOf2FinalsItemNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2FinalsItemNestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf2FinalsItemNestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf2FinalsItemNestedLeaf", s)}
}

// RefStressObjectAllOf2AllOf2FinalsItemNestedSameName is generated.
type RefStressObjectAllOf2AllOf2FinalsItemNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2FinalsItemNestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf2FinalsItemNestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf2AllOf2FinalsItemOptionalShared is generated.
type RefStressObjectAllOf2AllOf2FinalsItemOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2FinalsItemOptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf2FinalsItemOptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf2AllOf2FinalsItemSharedName is generated.
type RefStressObjectAllOf2AllOf2FinalsItemSharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2FinalsItemSharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf2FinalsItemSharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf2FinalsItemSharedName", s)}
}

// RefStressObjectAllOf2AllOf2Metadata is generated.
type RefStressObjectAllOf2AllOf2Metadata struct{}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf2AllOf2Metadata)(nil)
	_ json.Marshaler   = RefStressObjectAllOf2AllOf2Metadata{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf2AllOf2Metadata) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{},
		true,
		func(value []byte) error {
			var additionalProperty RefStressObjectAllOf2AllOf2MetadataAdditionalProperty

			return json.Unmarshal(value, &additionalProperty)
		},
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf2AllOf2Metadata) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf2AllOf2Metadata) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{},
		[]bool{},
	)
}

// RefStressObjectAllOf2AllOf2MetadataAdditionalProperty is generated.
type RefStressObjectAllOf2AllOf2MetadataAdditionalProperty string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2MetadataAdditionalProperty)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf2MetadataAdditionalProperty) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf2MetadataAdditionalProperty", s)}
}

// RefStressObjectAllOf2AllOf2RootFlag is generated.
type RefStressObjectAllOf2AllOf2RootFlag bool

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2RootFlag)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectAllOf2AllOf2RootFlag) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf2RootFlag", b)}
}

// RefStressObjectAllOf2AllOf2SharedName is generated.
type RefStressObjectAllOf2AllOf2SharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf2AllOf2SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf2AllOf2SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf2AllOf2SharedName", s)}
}

// RefStressObjectAllOf3 is generated.
type RefStressObjectAllOf3 struct {
	Count            RefStressObjectAllOf3Count
	Final            RefStressObjectAllOf3Final
	FinalCode        RefStressObjectAllOf3FinalCode
	Finals           RefStressObjectAllOf3Finals
	Metadata         RefStressObjectAllOf3Metadata
	MiddleFlag       RefStressObjectAllOf3MiddleFlag
	Nested           RefStressObjectAllOf3Nested
	NullableRequired RefStressObjectAllOf3NullableRequired
	OptionalCode     *RefStressObjectAllOf3OptionalCode
	OptionalShared   *RefStressObjectAllOf3OptionalShared
	RootFlag         RefStressObjectAllOf3RootFlag
	SharedName       RefStressObjectAllOf3SharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf3)(nil)
	_ json.Marshaler   = RefStressObjectAllOf3{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf3) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("count", &o.Count),
			requiredObjectProperty("final", &o.Final),
			requiredObjectProperty("finalCode", &o.FinalCode),
			requiredObjectProperty("finals", &o.Finals),
			requiredObjectProperty("metadata", &o.Metadata),
			requiredObjectProperty("middleFlag", &o.MiddleFlag),
			requiredObjectProperty("nested", &o.Nested),
			requiredObjectProperty("nullableRequired", &o.NullableRequired),
			optionalObjectProperty("optionalCode", &o.OptionalCode),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("rootFlag", &o.RootFlag),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		false,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf3) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf3) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"count",
			"final",
			"finalCode",
			"finals",
			"metadata",
			"middleFlag",
			"nested",
			"nullableRequired",
			"optionalCode",
			"optionalShared",
			"rootFlag",
			"sharedName",
		},
		[]bool{
			false,
			false,
			false,
			false,
			false,
			false,
			false,
			false,
			true,
			true,
			false,
			false,
		},
	)
}

// RefStressObjectAllOf3Count is generated.
type RefStressObjectAllOf3Count json.Number

var _ json.Unmarshaler = new(RefStressObjectAllOf3Count)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (n RefStressObjectAllOf3Count) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3Count", n)}
}

// RefStressObjectAllOf3Final is generated.
type RefStressObjectAllOf3Final struct {
	FinalCode      RefStressObjectAllOf3FinalFinalCode
	Nested         *RefStressObjectAllOf3FinalNested
	OptionalShared *RefStressObjectAllOf3FinalOptionalShared
	SharedName     RefStressObjectAllOf3FinalSharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf3Final)(nil)
	_ json.Marshaler   = RefStressObjectAllOf3Final{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf3Final) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf3Final) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf3Final) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectAllOf3FinalFinalCode is generated.
type RefStressObjectAllOf3FinalFinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalFinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3FinalFinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3FinalFinalCode", s)}
}

// RefStressObjectAllOf3FinalNested is generated.
type RefStressObjectAllOf3FinalNested struct {
	Leaf     *RefStressObjectAllOf3FinalNestedLeaf
	SameName RefStressObjectAllOf3FinalNestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf3FinalNested)(nil)
	_ json.Marshaler   = RefStressObjectAllOf3FinalNested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf3FinalNested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf3FinalNested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf3FinalNested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectAllOf3FinalNestedLeaf is generated.
type RefStressObjectAllOf3FinalNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalNestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3FinalNestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3FinalNestedLeaf", s)}
}

// RefStressObjectAllOf3FinalNestedSameName is generated.
type RefStressObjectAllOf3FinalNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalNestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3FinalNestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf3FinalOptionalShared is generated.
type RefStressObjectAllOf3FinalOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalOptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3FinalOptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf3FinalSharedName is generated.
type RefStressObjectAllOf3FinalSharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalSharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3FinalSharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3FinalSharedName", s)}
}

// RefStressObjectAllOf3FinalCode is generated.
type RefStressObjectAllOf3FinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3FinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3FinalCode", s)}
}

// RefStressObjectAllOf3Finals is generated.
type RefStressObjectAllOf3Finals []RefStressObjectAllOf3FinalsItem

var _ json.Unmarshaler = new(RefStressObjectAllOf3Finals)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (a RefStressObjectAllOf3Finals) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3Finals", a)}
}

// RefStressObjectAllOf3FinalsItem is generated.
type RefStressObjectAllOf3FinalsItem struct {
	FinalCode      RefStressObjectAllOf3FinalsItemFinalCode
	Nested         *RefStressObjectAllOf3FinalsItemNested
	OptionalShared *RefStressObjectAllOf3FinalsItemOptionalShared
	SharedName     RefStressObjectAllOf3FinalsItemSharedName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf3FinalsItem)(nil)
	_ json.Marshaler   = RefStressObjectAllOf3FinalsItem{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf3FinalsItem) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("finalCode", &o.FinalCode),
			optionalObjectProperty("nested", &o.Nested),
			optionalObjectProperty("optionalShared", &o.OptionalShared),
			requiredObjectProperty("sharedName", &o.SharedName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf3FinalsItem) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf3FinalsItem) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"finalCode",
			"nested",
			"optionalShared",
			"sharedName",
		},
		[]bool{
			false,
			true,
			true,
			false,
		},
	)
}

// RefStressObjectAllOf3FinalsItemFinalCode is generated.
type RefStressObjectAllOf3FinalsItemFinalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalsItemFinalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3FinalsItemFinalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3FinalsItemFinalCode", s)}
}

// RefStressObjectAllOf3FinalsItemNested is generated.
type RefStressObjectAllOf3FinalsItemNested struct {
	Leaf     *RefStressObjectAllOf3FinalsItemNestedLeaf
	SameName RefStressObjectAllOf3FinalsItemNestedSameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf3FinalsItemNested)(nil)
	_ json.Marshaler   = RefStressObjectAllOf3FinalsItemNested{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf3FinalsItemNested) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf3FinalsItemNested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf3FinalsItemNested) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectAllOf3FinalsItemNestedLeaf is generated.
type RefStressObjectAllOf3FinalsItemNestedLeaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalsItemNestedLeaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3FinalsItemNestedLeaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3FinalsItemNestedLeaf", s)}
}

// RefStressObjectAllOf3FinalsItemNestedSameName is generated.
type RefStressObjectAllOf3FinalsItemNestedSameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalsItemNestedSameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3FinalsItemNestedSameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf3FinalsItemOptionalShared is generated.
type RefStressObjectAllOf3FinalsItemOptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalsItemOptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3FinalsItemOptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf3FinalsItemSharedName is generated.
type RefStressObjectAllOf3FinalsItemSharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf3FinalsItemSharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3FinalsItemSharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3FinalsItemSharedName", s)}
}

// RefStressObjectAllOf3Metadata is generated.
type RefStressObjectAllOf3Metadata struct{}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf3Metadata)(nil)
	_ json.Marshaler   = RefStressObjectAllOf3Metadata{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf3Metadata) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{},
		true,
		func(value []byte) error {
			var additionalProperty RefStressObjectAllOf3MetadataAdditionalProperty

			return json.Unmarshal(value, &additionalProperty)
		},
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf3Metadata) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf3Metadata) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{},
		[]bool{},
	)
}

// RefStressObjectAllOf3MetadataAdditionalProperty is generated.
type RefStressObjectAllOf3MetadataAdditionalProperty string

var _ json.Unmarshaler = new(RefStressObjectAllOf3MetadataAdditionalProperty)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3MetadataAdditionalProperty) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3MetadataAdditionalProperty", s)}
}

// RefStressObjectAllOf3MiddleFlag is generated.
type RefStressObjectAllOf3MiddleFlag bool

var _ json.Unmarshaler = new(RefStressObjectAllOf3MiddleFlag)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectAllOf3MiddleFlag) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3MiddleFlag", b)}
}

// RefStressObjectAllOf3Nested is generated.
type RefStressObjectAllOf3Nested struct {
	RefStressObjectAllOf3NestedAllOf1
	RefStressObjectAllOf3NestedAllOf2
	RefStressObjectAllOf3NestedAllOf3
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf3Nested)(nil)
	_ json.Marshaler   = RefStressObjectAllOf3Nested{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a RefStressObjectAllOf3Nested) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a RefStressObjectAllOf3Nested) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf3NestedAllOf1)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf3NestedAllOf2)
	fields = appendEmbeddedJSONFields(fields, a.RefStressObjectAllOf3NestedAllOf3)

	return fields
}

// RefStressObjectAllOf3NestedAllOf1 is generated.
type RefStressObjectAllOf3NestedAllOf1 struct {
	Leaf     *RefStressObjectAllOf3NestedAllOf1Leaf
	SameName RefStressObjectAllOf3NestedAllOf1SameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf3NestedAllOf1)(nil)
	_ json.Marshaler   = RefStressObjectAllOf3NestedAllOf1{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf3NestedAllOf1) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf3NestedAllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf3NestedAllOf1) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectAllOf3NestedAllOf1Leaf is generated.
type RefStressObjectAllOf3NestedAllOf1Leaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf3NestedAllOf1Leaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3NestedAllOf1Leaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3NestedAllOf1Leaf", s)}
}

// RefStressObjectAllOf3NestedAllOf1SameName is generated.
type RefStressObjectAllOf3NestedAllOf1SameName struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf3NestedAllOf1SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3NestedAllOf1SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf3NestedAllOf2 is generated.
type RefStressObjectAllOf3NestedAllOf2 struct {
	Leaf     *RefStressObjectAllOf3NestedAllOf2Leaf
	SameName RefStressObjectAllOf3NestedAllOf2SameName
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf3NestedAllOf2)(nil)
	_ json.Marshaler   = RefStressObjectAllOf3NestedAllOf2{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf3NestedAllOf2) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			optionalObjectProperty("leaf", &o.Leaf),
			requiredObjectProperty("sameName", &o.SameName),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf3NestedAllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf3NestedAllOf2) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"leaf",
			"sameName",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefStressObjectAllOf3NestedAllOf2Leaf is generated.
type RefStressObjectAllOf3NestedAllOf2Leaf string

var _ json.Unmarshaler = new(RefStressObjectAllOf3NestedAllOf2Leaf)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3NestedAllOf2Leaf) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3NestedAllOf2Leaf", s)}
}

// RefStressObjectAllOf3NestedAllOf2SameName is generated.
type RefStressObjectAllOf3NestedAllOf2SameName string

var _ json.Unmarshaler = new(RefStressObjectAllOf3NestedAllOf2SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3NestedAllOf2SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3NestedAllOf2SameName", s)}
}

// RefStressObjectAllOf3NestedAllOf3 is generated.
type RefStressObjectAllOf3NestedAllOf3 struct {
	SameName RefStressObjectAllOf3NestedAllOf3SameName
	Sealed   RefStressObjectAllOf3NestedAllOf3Sealed
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf3NestedAllOf3)(nil)
	_ json.Marshaler   = RefStressObjectAllOf3NestedAllOf3{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf3NestedAllOf3) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("sameName", &o.SameName),
			requiredObjectProperty("sealed", &o.Sealed),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf3NestedAllOf3) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf3NestedAllOf3) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"sameName",
			"sealed",
		},
		[]bool{
			false,
			false,
		},
	)
}

// RefStressObjectAllOf3NestedAllOf3SameName is generated.
type RefStressObjectAllOf3NestedAllOf3SameName string

var _ json.Unmarshaler = new(RefStressObjectAllOf3NestedAllOf3SameName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3NestedAllOf3SameName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3NestedAllOf3SameName", s)}
}

// RefStressObjectAllOf3NestedAllOf3Sealed is generated.
type RefStressObjectAllOf3NestedAllOf3Sealed struct {
	Locked RefStressObjectAllOf3NestedAllOf3SealedLocked
}

var (
	_ json.Unmarshaler = (*RefStressObjectAllOf3NestedAllOf3Sealed)(nil)
	_ json.Marshaler   = RefStressObjectAllOf3NestedAllOf3Sealed{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefStressObjectAllOf3NestedAllOf3Sealed) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("locked", &o.Locked),
		},
		false,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefStressObjectAllOf3NestedAllOf3Sealed) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefStressObjectAllOf3NestedAllOf3Sealed) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"locked",
		},
		[]bool{
			false,
		},
	)
}

// RefStressObjectAllOf3NestedAllOf3SealedLocked is generated.
type RefStressObjectAllOf3NestedAllOf3SealedLocked bool

var _ json.Unmarshaler = new(RefStressObjectAllOf3NestedAllOf3SealedLocked)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectAllOf3NestedAllOf3SealedLocked) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3NestedAllOf3SealedLocked", b)}
}

// RefStressObjectAllOf3NullableRequired is generated.
type RefStressObjectAllOf3NullableRequired struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf3NullableRequired)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3NullableRequired) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf3OptionalCode is generated.
type RefStressObjectAllOf3OptionalCode string

var _ json.Unmarshaler = new(RefStressObjectAllOf3OptionalCode)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3OptionalCode) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3OptionalCode", s)}
}

// RefStressObjectAllOf3OptionalShared is generated.
type RefStressObjectAllOf3OptionalShared struct {
	Value *string
}

var _ json.Unmarshaler = new(RefStressObjectAllOf3OptionalShared)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3OptionalShared) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// RefStressObjectAllOf3RootFlag is generated.
type RefStressObjectAllOf3RootFlag bool

var _ json.Unmarshaler = new(RefStressObjectAllOf3RootFlag)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefStressObjectAllOf3RootFlag) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3RootFlag", b)}
}

// RefStressObjectAllOf3SharedName is generated.
type RefStressObjectAllOf3SharedName string

var _ json.Unmarshaler = new(RefStressObjectAllOf3SharedName)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefStressObjectAllOf3SharedName) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefStressObjectAllOf3SharedName", s)}
}

// RefObject is generated.
type RefObject struct {
	RefOptionalBool   *RefObjectRefOptionalBool
	RefRequiredString RefObjectRefRequiredString
}

var (
	_ json.Unmarshaler = (*RefObject)(nil)
	_ json.Marshaler   = RefObject{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *RefObject) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			optionalObjectProperty("refOptionalBool", &o.RefOptionalBool),
			requiredObjectProperty("refRequiredString", &o.RefRequiredString),
		},
		false,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o RefObject) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o RefObject) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"refOptionalBool",
			"refRequiredString",
		},
		[]bool{
			true,
			false,
		},
	)
}

// RefObjectRefOptionalBool is generated.
type RefObjectRefOptionalBool struct {
	Value *bool
}

var _ json.Unmarshaler = new(RefObjectRefOptionalBool)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b RefObjectRefOptionalBool) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", b.Value)}
}

// RefObjectRefRequiredString is generated.
type RefObjectRefRequiredString string

var _ json.Unmarshaler = new(RefObjectRefRequiredString)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RefObjectRefRequiredString) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RefObjectRefRequiredString", s)}
}

// OptionalArrayNullable is generated.
type OptionalArrayNullable struct {
	Value []OptionalArrayNullableItem
}

var _ json.Unmarshaler = new(OptionalArrayNullable)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (a OptionalArrayNullable) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", a.Value)}
}

// OptionalArrayNullableItem is generated.
type OptionalArrayNullableItem string

var _ json.Unmarshaler = new(OptionalArrayNullableItem)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s OptionalArrayNullableItem) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("OptionalArrayNullableItem", s)}
}

// ObjectKeysAdditionalPropertiesFalse is generated.
type ObjectKeysAdditionalPropertiesFalse struct {
	OptionalNotNullableString *ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString
	OptionalNullableString    *ObjectKeysAdditionalPropertiesFalseOptionalNullableString
	RequiredNotNullableString ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString
	RequiredNullableString    ObjectKeysAdditionalPropertiesFalseRequiredNullableString
}

var (
	_ json.Unmarshaler = (*ObjectKeysAdditionalPropertiesFalse)(nil)
	_ json.Marshaler   = ObjectKeysAdditionalPropertiesFalse{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *ObjectKeysAdditionalPropertiesFalse) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			optionalObjectProperty("optionalNotNullableString", &o.OptionalNotNullableString),
			optionalObjectProperty("optionalNullableString", &o.OptionalNullableString),
			requiredObjectProperty("requiredNotNullableString", &o.RequiredNotNullableString),
			requiredObjectProperty("requiredNullableString", &o.RequiredNullableString),
		},
		false,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o ObjectKeysAdditionalPropertiesFalse) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o ObjectKeysAdditionalPropertiesFalse) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"optionalNotNullableString",
			"optionalNullableString",
			"requiredNotNullableString",
			"requiredNullableString",
		},
		[]bool{
			true,
			true,
			false,
			false,
		},
	)
}

// ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString is generated.
type ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString string

var _ json.Unmarshaler = new(ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString", s)}
}

// ObjectKeysAdditionalPropertiesFalseOptionalNullableString is generated.
type ObjectKeysAdditionalPropertiesFalseOptionalNullableString struct {
	Value *string
}

var _ json.Unmarshaler = new(ObjectKeysAdditionalPropertiesFalseOptionalNullableString)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s ObjectKeysAdditionalPropertiesFalseOptionalNullableString) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString is generated.
type ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString string

var _ json.Unmarshaler = new(ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString", s)}
}

// ObjectKeysAdditionalPropertiesFalseRequiredNullableString is generated.
type ObjectKeysAdditionalPropertiesFalseRequiredNullableString struct {
	Value *string
}

var _ json.Unmarshaler = new(ObjectKeysAdditionalPropertiesFalseRequiredNullableString)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s ObjectKeysAdditionalPropertiesFalseRequiredNullableString) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// NullableObjectKeysAdditionalPropertiesFalse is generated.
type NullableObjectKeysAdditionalPropertiesFalse struct {
	OptionalNotNullableString *NullableObjectKeysAdditionalPropertiesFalseOptionalNotNullableString
	OptionalNullableString    *NullableObjectKeysAdditionalPropertiesFalseOptionalNullableString
	RequiredNotNullableString NullableObjectKeysAdditionalPropertiesFalseRequiredNotNullableString
	RequiredNullableString    NullableObjectKeysAdditionalPropertiesFalseRequiredNullableString
}

var (
	_ json.Unmarshaler = (*NullableObjectKeysAdditionalPropertiesFalse)(nil)
	_ json.Marshaler   = NullableObjectKeysAdditionalPropertiesFalse{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *NullableObjectKeysAdditionalPropertiesFalse) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		true,
		[]objectPropertyDecoder{
			optionalObjectProperty("optionalNotNullableString", &o.OptionalNotNullableString),
			optionalObjectProperty("optionalNullableString", &o.OptionalNullableString),
			requiredObjectProperty("requiredNotNullableString", &o.RequiredNotNullableString),
			requiredObjectProperty("requiredNullableString", &o.RequiredNullableString),
		},
		false,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o NullableObjectKeysAdditionalPropertiesFalse) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o NullableObjectKeysAdditionalPropertiesFalse) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"optionalNotNullableString",
			"optionalNullableString",
			"requiredNotNullableString",
			"requiredNullableString",
		},
		[]bool{
			true,
			true,
			false,
			false,
		},
	)
}

// NullableObjectKeysAdditionalPropertiesFalseOptionalNotNullableString is generated.
type NullableObjectKeysAdditionalPropertiesFalseOptionalNotNullableString string

var _ json.Unmarshaler = new(NullableObjectKeysAdditionalPropertiesFalseOptionalNotNullableString)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s NullableObjectKeysAdditionalPropertiesFalseOptionalNotNullableString) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("NullableObjectKeysAdditionalPropertiesFalseOptionalNotNullableString", s)}
}

// NullableObjectKeysAdditionalPropertiesFalseOptionalNullableString is generated.
type NullableObjectKeysAdditionalPropertiesFalseOptionalNullableString struct {
	Value *string
}

var _ json.Unmarshaler = new(NullableObjectKeysAdditionalPropertiesFalseOptionalNullableString)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s NullableObjectKeysAdditionalPropertiesFalseOptionalNullableString) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// NullableObjectKeysAdditionalPropertiesFalseRequiredNotNullableString is generated.
type NullableObjectKeysAdditionalPropertiesFalseRequiredNotNullableString string

var _ json.Unmarshaler = new(NullableObjectKeysAdditionalPropertiesFalseRequiredNotNullableString)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s NullableObjectKeysAdditionalPropertiesFalseRequiredNotNullableString) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("NullableObjectKeysAdditionalPropertiesFalseRequiredNotNullableString", s)}
}

// NullableObjectKeysAdditionalPropertiesFalseRequiredNullableString is generated.
type NullableObjectKeysAdditionalPropertiesFalseRequiredNullableString struct {
	Value *string
}

var _ json.Unmarshaler = new(NullableObjectKeysAdditionalPropertiesFalseRequiredNullableString)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s NullableObjectKeysAdditionalPropertiesFalseRequiredNullableString) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// CompositeObject is generated.
type CompositeObject struct {
	ArrayNotNullableItemsNotNullable   CompositeObjectArrayNotNullableItemsNotNullable
	ArrayNotNullableItemsNullable      CompositeObjectArrayNotNullableItemsNullable
	ArrayNullableItemsNotNullable      CompositeObjectArrayNullableItemsNotNullable
	ArrayNullableItemsNullable         CompositeObjectArrayNullableItemsNullable
	BoolNotNullable                    CompositeObjectBoolNotNullable
	BoolNullable                       CompositeObjectBoolNullable
	NumberNotNullable                  CompositeObjectNumberNotNullable
	NumberNullable                     CompositeObjectNumberNullable
	ObjectAdditionalPropertiesImplicit CompositeObjectObjectAdditionalPropertiesImplicit
	ObjectAdditionalPropertiesSchema   CompositeObjectObjectAdditionalPropertiesSchema
	ObjectAdditionalPropertiesTrue     CompositeObjectObjectAdditionalPropertiesTrue
	StringFormatNotNullable            CompositeObjectStringFormatNotNullable
	StringFormatNullable               CompositeObjectStringFormatNullable
}

var (
	_ json.Unmarshaler = (*CompositeObject)(nil)
	_ json.Marshaler   = CompositeObject{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *CompositeObject) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("arrayNotNullableItemsNotNullable", &o.ArrayNotNullableItemsNotNullable),
			requiredObjectProperty("arrayNotNullableItemsNullable", &o.ArrayNotNullableItemsNullable),
			requiredObjectProperty("arrayNullableItemsNotNullable", &o.ArrayNullableItemsNotNullable),
			requiredObjectProperty("arrayNullableItemsNullable", &o.ArrayNullableItemsNullable),
			requiredObjectProperty("boolNotNullable", &o.BoolNotNullable),
			requiredObjectProperty("boolNullable", &o.BoolNullable),
			requiredObjectProperty("numberNotNullable", &o.NumberNotNullable),
			requiredObjectProperty("numberNullable", &o.NumberNullable),
			requiredObjectProperty("objectAdditionalPropertiesImplicit", &o.ObjectAdditionalPropertiesImplicit),
			requiredObjectProperty("objectAdditionalPropertiesSchema", &o.ObjectAdditionalPropertiesSchema),
			requiredObjectProperty("objectAdditionalPropertiesTrue", &o.ObjectAdditionalPropertiesTrue),
			requiredObjectProperty("stringFormatNotNullable", &o.StringFormatNotNullable),
			requiredObjectProperty("stringFormatNullable", &o.StringFormatNullable),
		},
		false,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o CompositeObject) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o CompositeObject) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"arrayNotNullableItemsNotNullable",
			"arrayNotNullableItemsNullable",
			"arrayNullableItemsNotNullable",
			"arrayNullableItemsNullable",
			"boolNotNullable",
			"boolNullable",
			"numberNotNullable",
			"numberNullable",
			"objectAdditionalPropertiesImplicit",
			"objectAdditionalPropertiesSchema",
			"objectAdditionalPropertiesTrue",
			"stringFormatNotNullable",
			"stringFormatNullable",
		},
		[]bool{
			false,
			false,
			false,
			false,
			false,
			false,
			false,
			false,
			false,
			false,
			false,
			false,
			false,
		},
	)
}

// CompositeObjectArrayNotNullableItemsNotNullable is generated.
type CompositeObjectArrayNotNullableItemsNotNullable []CompositeObjectArrayNotNullableItemsNotNullableItem

var _ json.Unmarshaler = new(CompositeObjectArrayNotNullableItemsNotNullable)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (a CompositeObjectArrayNotNullableItemsNotNullable) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("CompositeObjectArrayNotNullableItemsNotNullable", a)}
}

// CompositeObjectArrayNotNullableItemsNotNullableItem is generated.
type CompositeObjectArrayNotNullableItemsNotNullableItem string

var _ json.Unmarshaler = new(CompositeObjectArrayNotNullableItemsNotNullableItem)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s CompositeObjectArrayNotNullableItemsNotNullableItem) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("CompositeObjectArrayNotNullableItemsNotNullableItem", s)}
}

// CompositeObjectArrayNotNullableItemsNullable is generated.
type CompositeObjectArrayNotNullableItemsNullable []CompositeObjectArrayNotNullableItemsNullableItem

var _ json.Unmarshaler = new(CompositeObjectArrayNotNullableItemsNullable)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (a CompositeObjectArrayNotNullableItemsNullable) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("CompositeObjectArrayNotNullableItemsNullable", a)}
}

// CompositeObjectArrayNotNullableItemsNullableItem is generated.
type CompositeObjectArrayNotNullableItemsNullableItem struct {
	Value *string
}

var _ json.Unmarshaler = new(CompositeObjectArrayNotNullableItemsNullableItem)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s CompositeObjectArrayNotNullableItemsNullableItem) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// CompositeObjectArrayNullableItemsNotNullable is generated.
type CompositeObjectArrayNullableItemsNotNullable struct {
	Value []CompositeObjectArrayNullableItemsNotNullableItem
}

var _ json.Unmarshaler = new(CompositeObjectArrayNullableItemsNotNullable)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (a CompositeObjectArrayNullableItemsNotNullable) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", a.Value)}
}

// CompositeObjectArrayNullableItemsNotNullableItem is generated.
type CompositeObjectArrayNullableItemsNotNullableItem string

var _ json.Unmarshaler = new(CompositeObjectArrayNullableItemsNotNullableItem)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s CompositeObjectArrayNullableItemsNotNullableItem) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("CompositeObjectArrayNullableItemsNotNullableItem", s)}
}

// CompositeObjectArrayNullableItemsNullable is generated.
type CompositeObjectArrayNullableItemsNullable struct {
	Value []CompositeObjectArrayNullableItemsNullableItem
}

var _ json.Unmarshaler = new(CompositeObjectArrayNullableItemsNullable)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (a CompositeObjectArrayNullableItemsNullable) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", a.Value)}
}

// CompositeObjectArrayNullableItemsNullableItem is generated.
type CompositeObjectArrayNullableItemsNullableItem struct {
	Value *string
}

var _ json.Unmarshaler = new(CompositeObjectArrayNullableItemsNullableItem)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s CompositeObjectArrayNullableItemsNullableItem) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// CompositeObjectBoolNotNullable is generated.
type CompositeObjectBoolNotNullable bool

var _ json.Unmarshaler = new(CompositeObjectBoolNotNullable)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b CompositeObjectBoolNotNullable) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("CompositeObjectBoolNotNullable", b)}
}

// CompositeObjectBoolNullable is generated.
type CompositeObjectBoolNullable struct {
	Value *bool
}

var _ json.Unmarshaler = new(CompositeObjectBoolNullable)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b CompositeObjectBoolNullable) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", b.Value)}
}

// CompositeObjectNumberNotNullable is generated.
type CompositeObjectNumberNotNullable json.Number

var _ json.Unmarshaler = new(CompositeObjectNumberNotNullable)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (n CompositeObjectNumberNotNullable) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("CompositeObjectNumberNotNullable", n)}
}

// CompositeObjectNumberNullable is generated.
type CompositeObjectNumberNullable struct {
	Value *json.Number
}

var _ json.Unmarshaler = new(CompositeObjectNumberNullable)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (n CompositeObjectNumberNullable) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", n.Value)}
}

// CompositeObjectObjectAdditionalPropertiesImplicit is generated.
type CompositeObjectObjectAdditionalPropertiesImplicit struct {
	Known *CompositeObjectObjectAdditionalPropertiesImplicitKnown
}

var (
	_ json.Unmarshaler = (*CompositeObjectObjectAdditionalPropertiesImplicit)(nil)
	_ json.Marshaler   = CompositeObjectObjectAdditionalPropertiesImplicit{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *CompositeObjectObjectAdditionalPropertiesImplicit) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			optionalObjectProperty("known", &o.Known),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o CompositeObjectObjectAdditionalPropertiesImplicit) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o CompositeObjectObjectAdditionalPropertiesImplicit) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"known",
		},
		[]bool{
			true,
		},
	)
}

// CompositeObjectObjectAdditionalPropertiesImplicitKnown is generated.
type CompositeObjectObjectAdditionalPropertiesImplicitKnown string

var _ json.Unmarshaler = new(CompositeObjectObjectAdditionalPropertiesImplicitKnown)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s CompositeObjectObjectAdditionalPropertiesImplicitKnown) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("CompositeObjectObjectAdditionalPropertiesImplicitKnown", s)}
}

// CompositeObjectObjectAdditionalPropertiesSchema is generated.
type CompositeObjectObjectAdditionalPropertiesSchema struct {
	Known *CompositeObjectObjectAdditionalPropertiesSchemaKnown
}

var (
	_ json.Unmarshaler = (*CompositeObjectObjectAdditionalPropertiesSchema)(nil)
	_ json.Marshaler   = CompositeObjectObjectAdditionalPropertiesSchema{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *CompositeObjectObjectAdditionalPropertiesSchema) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			optionalObjectProperty("known", &o.Known),
		},
		true,
		func(value []byte) error {
			var additionalProperty CompositeObjectObjectAdditionalPropertiesSchemaAdditionalProperty

			return json.Unmarshal(value, &additionalProperty)
		},
	)
}

// MarshalJSON encodes the model as JSON.
func (o CompositeObjectObjectAdditionalPropertiesSchema) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o CompositeObjectObjectAdditionalPropertiesSchema) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"known",
		},
		[]bool{
			true,
		},
	)
}

// CompositeObjectObjectAdditionalPropertiesSchemaKnown is generated.
type CompositeObjectObjectAdditionalPropertiesSchemaKnown string

var _ json.Unmarshaler = new(CompositeObjectObjectAdditionalPropertiesSchemaKnown)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s CompositeObjectObjectAdditionalPropertiesSchemaKnown) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("CompositeObjectObjectAdditionalPropertiesSchemaKnown", s)}
}

// CompositeObjectObjectAdditionalPropertiesSchemaAdditionalProperty is generated.
type CompositeObjectObjectAdditionalPropertiesSchemaAdditionalProperty string

var _ json.Unmarshaler = new(CompositeObjectObjectAdditionalPropertiesSchemaAdditionalProperty)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s CompositeObjectObjectAdditionalPropertiesSchemaAdditionalProperty) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("CompositeObjectObjectAdditionalPropertiesSchemaAdditionalProperty", s)}
}

// CompositeObjectObjectAdditionalPropertiesTrue is generated.
type CompositeObjectObjectAdditionalPropertiesTrue struct {
	Known *CompositeObjectObjectAdditionalPropertiesTrueKnown
}

var (
	_ json.Unmarshaler = (*CompositeObjectObjectAdditionalPropertiesTrue)(nil)
	_ json.Marshaler   = CompositeObjectObjectAdditionalPropertiesTrue{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *CompositeObjectObjectAdditionalPropertiesTrue) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			optionalObjectProperty("known", &o.Known),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o CompositeObjectObjectAdditionalPropertiesTrue) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o CompositeObjectObjectAdditionalPropertiesTrue) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"known",
		},
		[]bool{
			true,
		},
	)
}

// CompositeObjectObjectAdditionalPropertiesTrueKnown is generated.
type CompositeObjectObjectAdditionalPropertiesTrueKnown string

var _ json.Unmarshaler = new(CompositeObjectObjectAdditionalPropertiesTrueKnown)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s CompositeObjectObjectAdditionalPropertiesTrueKnown) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("CompositeObjectObjectAdditionalPropertiesTrueKnown", s)}
}

// CompositeObjectStringFormatNotNullable is generated.
type CompositeObjectStringFormatNotNullable string

var _ json.Unmarshaler = new(CompositeObjectStringFormatNotNullable)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s CompositeObjectStringFormatNotNullable) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("CompositeObjectStringFormatNotNullable", s)}
}

// CompositeObjectStringFormatNullable is generated.
type CompositeObjectStringFormatNullable struct {
	Value *string
}

var _ json.Unmarshaler = new(CompositeObjectStringFormatNullable)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s CompositeObjectStringFormatNullable) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", s.Value)}
}

// ArrayNullable is generated.
type ArrayNullable struct {
	Value []ArrayNullableItem
}

var _ json.Unmarshaler = new(ArrayNullable)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (a ArrayNullable) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("Value", a.Value)}
}

// ArrayNullableItem is generated.
type ArrayNullableItem string

var _ json.Unmarshaler = new(ArrayNullableItem)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s ArrayNullableItem) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("ArrayNullableItem", s)}
}

// ArrayNotNullable is generated.
type ArrayNotNullable []ArrayNotNullableItem

var _ json.Unmarshaler = new(ArrayNotNullable)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (a ArrayNotNullable) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("ArrayNotNullable", a)}
}

// ArrayNotNullableItem is generated.
type ArrayNotNullableItem string

var _ json.Unmarshaler = new(ArrayNotNullableItem)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s ArrayNotNullableItem) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("ArrayNotNullableItem", s)}
}

// AllOfObject is generated.
type AllOfObject struct {
	AllOfObjectAllOf1
	AllOfObjectAllOf2
	AllOfObjectAllOf3
}

var (
	_ json.Unmarshaler = (*AllOfObject)(nil)
	_ json.Marshaler   = AllOfObject{}
)

// UnmarshalJSON decodes JSON into every allOf member.
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

// MarshalJSON encodes every member as one JSON object.
func (a AllOfObject) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a AllOfObject) jsonFields() []jsonField {
	var fields []jsonField

	fields = appendEmbeddedJSONFields(fields, a.AllOfObjectAllOf1)
	fields = appendEmbeddedJSONFields(fields, a.AllOfObjectAllOf2)
	fields = appendEmbeddedJSONFields(fields, a.AllOfObjectAllOf3)

	return fields
}

// AllOfObjectAllOf1 is generated.
type AllOfObjectAllOf1 struct {
	First AllOfObjectAllOf1First
}

var (
	_ json.Unmarshaler = (*AllOfObjectAllOf1)(nil)
	_ json.Marshaler   = AllOfObjectAllOf1{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *AllOfObjectAllOf1) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("first", &o.First),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o AllOfObjectAllOf1) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o AllOfObjectAllOf1) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"first",
		},
		[]bool{
			false,
		},
	)
}

// AllOfObjectAllOf1First is generated.
type AllOfObjectAllOf1First string

var _ json.Unmarshaler = new(AllOfObjectAllOf1First)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s AllOfObjectAllOf1First) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("AllOfObjectAllOf1First", s)}
}

// AllOfObjectAllOf2 is generated.
type AllOfObjectAllOf2 struct {
	Second AllOfObjectAllOf2Second
}

var (
	_ json.Unmarshaler = (*AllOfObjectAllOf2)(nil)
	_ json.Marshaler   = AllOfObjectAllOf2{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *AllOfObjectAllOf2) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("second", &o.Second),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o AllOfObjectAllOf2) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o AllOfObjectAllOf2) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"second",
		},
		[]bool{
			false,
		},
	)
}

// AllOfObjectAllOf2Second is generated.
type AllOfObjectAllOf2Second bool

var _ json.Unmarshaler = new(AllOfObjectAllOf2Second)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (b AllOfObjectAllOf2Second) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("AllOfObjectAllOf2Second", b)}
}

// AllOfObjectAllOf3 is generated.
type AllOfObjectAllOf3 struct {
	Last AllOfObjectAllOf3Last
}

var (
	_ json.Unmarshaler = (*AllOfObjectAllOf3)(nil)
	_ json.Marshaler   = AllOfObjectAllOf3{}
)

// UnmarshalJSON decodes JSON into the model.
func (o *AllOfObjectAllOf3) UnmarshalJSON(data []byte) error {
	return unmarshalObject(
		data,
		false,
		false,
		[]objectPropertyDecoder{
			requiredObjectProperty("last", &o.Last),
		},
		true,
		nil,
	)
}

// MarshalJSON encodes the model as JSON.
func (o AllOfObjectAllOf3) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(o.jsonFields())
}

// jsonFields returns the JSON fields declared by the model.
func (o AllOfObjectAllOf3) jsonFields() []jsonField {
	return objectJSONFields(
		o,
		[]string{
			"last",
		},
		[]bool{
			false,
		},
	)
}

// AllOfObjectAllOf3Last is generated.
type AllOfObjectAllOf3Last json.Number

var _ json.Unmarshaler = new(AllOfObjectAllOf3Last)

// UnmarshalJSON decodes JSON into the model.
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

// jsonFields returns fields promoted when the model is embedded in allOf.
func (n AllOfObjectAllOf3Last) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("AllOfObjectAllOf3Last", n)}
}
