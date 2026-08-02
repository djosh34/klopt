package schematest

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

// jsonKind identifies one of JSON's six semantic kinds.
type jsonKind uint8

const (
	// jsonNull identifies null.
	jsonNull jsonKind = iota
	// jsonBoolean identifies a boolean.
	jsonBoolean
	// jsonNumber identifies an exact number.
	jsonNumber
	// jsonString identifies a decoded Unicode string.
	jsonString
	// jsonArray identifies an ordered array.
	jsonArray
	// jsonObject identifies an unordered object.
	jsonObject
)

// jsonValue is the package-private exact JSON value model.
type jsonValue struct {
	kind    jsonKind
	boolean bool
	number  *exactNumber
	text    string
	array   []*jsonValue
	object  map[string]*jsonValue
}

// jsonSemanticEqual compares two JSON values using JSON Schema semantics.
func jsonSemanticEqual(left, right *jsonValue) (bool, error) {
	if err := validateJSONValue(left, make(map[*jsonValue]bool)); err != nil {
		return false, fmt.Errorf("left JSON value: %w", err)
	}

	if err := validateJSONValue(right, make(map[*jsonValue]bool)); err != nil {
		return false, fmt.Errorf("right JSON value: %w", err)
	}

	return equalJSONValue(left, right)
}

// equalJSONValue compares already-validated JSON values.
func equalJSONValue(left, right *jsonValue) (bool, error) {
	if left.kind != right.kind {
		return false, nil
	}

	switch left.kind {
	case jsonNull:
		return true, nil
	case jsonBoolean:
		return left.boolean == right.boolean, nil
	case jsonNumber:
		comparison, err := left.number.compare(right.number)
		if err != nil {
			return false, err
		}

		return comparison == 0, nil
	case jsonString:
		return left.text == right.text, nil
	case jsonArray:
		return equalJSONArray(left.array, right.array)
	case jsonObject:
		return equalJSONObject(left.object, right.object)
	default:
		return false, fmt.Errorf("unknown JSON kind %d", left.kind)
	}
}

// equalJSONArray compares two ordered arrays.
func equalJSONArray(left, right []*jsonValue) (bool, error) {
	if len(left) != len(right) {
		return false, nil
	}

	for index := range left {
		equal, err := equalJSONValue(left[index], right[index])
		if err != nil {
			return false, err
		}

		if !equal {
			return false, nil
		}
	}

	return true, nil
}

// equalJSONObject compares two objects without observing member order.
func equalJSONObject(left, right map[string]*jsonValue) (bool, error) {
	if len(left) != len(right) {
		return false, nil
	}

	for name, leftMember := range left {
		rightMember, exists := right[name]
		if !exists {
			return false, nil
		}

		equal, err := equalJSONValue(leftMember, rightMember)
		if err != nil {
			return false, err
		}

		if !equal {
			return false, nil
		}
	}

	return true, nil
}

// validateJSONValue rejects malformed private model state and cycles.
func validateJSONValue(value *jsonValue, visiting map[*jsonValue]bool) error {
	if value == nil {
		return errors.New("JSON value is nil")
	}

	if visiting[value] {
		return errors.New("JSON value contains a cycle")
	}

	visiting[value] = true
	defer delete(visiting, value)

	if err := validateJSONPayload(value); err != nil {
		return err
	}

	switch value.kind {
	case jsonArray:
		return validateJSONArray(value.array, visiting)
	case jsonObject:
		return validateJSONObject(value.object, visiting)
	default:
		return nil
	}
}

// validateJSONArray validates every nested array element.
func validateJSONArray(elements []*jsonValue, visiting map[*jsonValue]bool) error {
	for index, element := range elements {
		if err := validateJSONValue(element, visiting); err != nil {
			return fmt.Errorf("array element %d: %w", index, err)
		}
	}

	return nil
}

// validateJSONObject validates member names and nested values.
func validateJSONObject(members map[string]*jsonValue, visiting map[*jsonValue]bool) error {
	for name, member := range members {
		if !utf8.ValidString(name) {
			return errors.New("object member name is not valid UTF-8")
		}

		if err := validateJSONValue(member, visiting); err != nil {
			return fmt.Errorf("object member %q: %w", name, err)
		}
	}

	return nil
}

// validateJSONPayload checks the discriminated-union invariant.
func validateJSONPayload(value *jsonValue) error {
	switch value.kind {
	case jsonNull:
		return validateNullPayload(value)
	case jsonBoolean:
		return validateBooleanPayload(value)
	case jsonNumber:
		return validateNumberPayload(value)
	case jsonString:
		return validateStringPayload(value)
	case jsonArray:
		return validateArrayPayload(value)
	case jsonObject:
		return validateObjectPayload(value)
	default:
		return fmt.Errorf("unknown JSON kind %d", value.kind)
	}
}

// validateNullPayload checks null carries no payload.
func validateNullPayload(value *jsonValue) error {
	if value.boolean || value.number != nil || value.text != "" || value.array != nil || value.object != nil {
		return errors.New("JSON null has payload")
	}

	return nil
}

// validateBooleanPayload checks a boolean carries no other kind's payload.
func validateBooleanPayload(value *jsonValue) error {
	if value.number != nil || value.text != "" || value.array != nil || value.object != nil {
		return errors.New("JSON boolean has another kind's payload")
	}

	return nil
}

// validateNumberPayload checks an exact number carries no other kind's payload.
func validateNumberPayload(value *jsonValue) error {
	if value.boolean || value.text != "" || value.array != nil || value.object != nil {
		return errors.New("JSON number has another kind's payload")
	}

	return value.number.validate()
}

// validateStringPayload checks a string is Unicode and carries no other kind's payload.
func validateStringPayload(value *jsonValue) error {
	if value.boolean || value.number != nil || value.array != nil || value.object != nil {
		return errors.New("JSON string has another kind's payload")
	}

	if !utf8.ValidString(value.text) {
		return errors.New("JSON string is not valid UTF-8")
	}

	return nil
}

// validateArrayPayload checks an array is initialized and carries no other kind's payload.
func validateArrayPayload(value *jsonValue) error {
	if value.boolean || value.number != nil || value.text != "" || value.object != nil {
		return errors.New("JSON array has another kind's payload")
	}

	if value.array == nil {
		return errors.New("JSON array payload is nil")
	}

	return nil
}

// validateObjectPayload checks an object is initialized and carries no other kind's payload.
func validateObjectPayload(value *jsonValue) error {
	if value.boolean || value.number != nil || value.text != "" || value.array != nil {
		return errors.New("JSON object has another kind's payload")
	}

	if value.object == nil {
		return errors.New("JSON object payload is nil")
	}

	return nil
}
