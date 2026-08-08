package schematest

import (
	"errors"
	"fmt"
	"math/big"
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
	if err := validateJSONValue(left); err != nil {
		return false, fmt.Errorf("left JSON value: %w", err)
	}

	if err := validateJSONValue(right); err != nil {
		return false, fmt.Errorf("right JSON value: %w", err)
	}

	return jsonValidatedSemanticEqual(left, right)
}

// jsonValidatedSemanticEqual compares values already validated at their ownership boundary.
//
//nolint:cyclop,gocognit // The pair stack handles each of JSON's six semantic kinds directly.
func jsonValidatedSemanticEqual(left, right *jsonValue) (bool, error) {
	work := []jsonValuePair{{left: left, right: right}}
	for len(work) > 0 {
		pair := work[len(work)-1]
		work = work[:len(work)-1]

		if pair.left.kind != pair.right.kind {
			return false, nil
		}

		switch pair.left.kind {
		case jsonNull:
		case jsonBoolean:
			if pair.left.boolean != pair.right.boolean {
				return false, nil
			}
		case jsonNumber:
			comparison, err := pair.left.number.compare(pair.right.number)
			if err != nil {
				return false, err
			}

			if comparison != 0 {
				return false, nil
			}
		case jsonString:
			if pair.left.text != pair.right.text {
				return false, nil
			}
		case jsonArray:
			if len(pair.left.array) != len(pair.right.array) {
				return false, nil
			}

			for index := len(pair.left.array) - 1; index >= 0; index-- {
				work = append(work, jsonValuePair{left: pair.left.array[index], right: pair.right.array[index]})
			}
		case jsonObject:
			if len(pair.left.object) != len(pair.right.object) {
				return false, nil
			}

			for _, name := range sortedObjectNames(pair.left.object) {
				rightMember, exists := pair.right.object[name]
				if !exists {
					return false, nil
				}

				work = append(work, jsonValuePair{left: pair.left.object[name], right: rightMember})
			}
		default:
			return false, fmt.Errorf("unknown JSON kind %d", pair.left.kind)
		}
	}

	return true, nil
}

// jsonValuePair is one pending semantic comparison.
type jsonValuePair struct {
	left  *jsonValue
	right *jsonValue
}

// jsonActivePath tracks only ancestors of the current traversal occurrence.
type jsonActivePath map[*jsonValue]bool

// validateJSONValue rejects malformed private model state and cycles.
//
//nolint:cyclop // Validation keeps payload and container checks in one iterative traversal.
func validateJSONValue(value *jsonValue) error {
	active := make(jsonActivePath)
	stack := []jsonValidationFrame{{value: value}}

	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if frame.exit {
			delete(active, frame.value)

			continue
		}

		if frame.value == nil {
			return contextualJSONError(frame.context, errors.New("JSON value is nil"))
		}

		if active[frame.value] {
			return contextualJSONError(frame.context, errors.New("JSON value contains a cycle"))
		}

		if err := validateJSONPayload(frame.value); err != nil {
			return contextualJSONError(frame.context, err)
		}

		active[frame.value] = true
		stack = append(stack, jsonValidationFrame{value: frame.value, exit: true})

		switch frame.value.kind {
		case jsonArray:
			for index := len(frame.value.array) - 1; index >= 0; index-- {
				stack = append(stack, jsonValidationFrame{
					value:   frame.value.array[index],
					context: fmt.Sprintf("array element %d", index),
				})
			}
		case jsonObject:
			names := sortedObjectNames(frame.value.object)
			for index := len(names) - 1; index >= 0; index-- {
				name := names[index]
				if !utf8.ValidString(name) {
					return errors.New("object member name is not valid UTF-8")
				}

				stack = append(stack, jsonValidationFrame{
					value:   frame.value.object[name],
					context: fmt.Sprintf("object member %q", name),
				})
			}
		}
	}

	return nil
}

// jsonValidationFrame enters or exits one value occurrence.
type jsonValidationFrame struct {
	value   *jsonValue
	context string
	exit    bool
}

// contextualJSONError adds the immediate parent edge to an invariant error.
func contextualJSONError(context string, err error) error {
	if context == "" {
		return err
	}

	return fmt.Errorf("%s: %w", context, err)
}

// cloneJSONValue makes an independent tree copy and rejects malformed state and cycles.
//
//nolint:cyclop // Copy and cycle checks share one occurrence-oriented iterative traversal.
func cloneJSONValue(value *jsonValue) (*jsonValue, error) {
	if value == nil {
		return nil, errors.New("JSON value is nil")
	}

	clone := &jsonValue{}
	active := make(jsonActivePath)
	stack := []jsonCloneFrame{{source: value, clone: clone}}

	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if frame.exit {
			delete(active, frame.source)

			continue
		}

		if frame.source == nil {
			return nil, contextualJSONError(frame.context, errors.New("JSON value is nil"))
		}

		if active[frame.source] {
			return nil, contextualJSONError(frame.context, errors.New("JSON value contains a cycle"))
		}

		if err := validateJSONPayload(frame.source); err != nil {
			return nil, contextualJSONError(frame.context, err)
		}

		active[frame.source] = true
		stack = append(stack, jsonCloneFrame{source: frame.source, exit: true})

		frame.clone.kind = frame.source.kind
		frame.clone.boolean = frame.source.boolean
		frame.clone.text = frame.source.text

		if frame.source.number != nil {
			frame.clone.number = &exactNumber{
				numerator:   new(big.Int).Set(frame.source.number.numerator),
				denominator: new(big.Int).Set(frame.source.number.denominator),
				exponent:    new(big.Int).Set(frame.source.number.exponent),
				scale:       new(big.Int).Set(frame.source.number.scale),
			}
		}

		switch frame.source.kind {
		case jsonArray:
			frame.clone.array = make([]*jsonValue, len(frame.source.array))
			for index := len(frame.source.array) - 1; index >= 0; index-- {
				child := &jsonValue{}
				frame.clone.array[index] = child
				stack = append(stack, jsonCloneFrame{
					source:  frame.source.array[index],
					clone:   child,
					context: fmt.Sprintf("array element %d", index),
				})
			}
		case jsonObject:
			frame.clone.object = make(map[string]*jsonValue, len(frame.source.object))

			names := sortedObjectNames(frame.source.object)
			for index := len(names) - 1; index >= 0; index-- {
				name := names[index]
				if !utf8.ValidString(name) {
					return nil, errors.New("object member name is not valid UTF-8")
				}

				child := &jsonValue{}
				frame.clone.object[name] = child
				stack = append(stack, jsonCloneFrame{
					source:  frame.source.object[name],
					clone:   child,
					context: fmt.Sprintf("object member %q", name),
				})
			}
		}
	}

	return clone, nil
}

// jsonCloneFrame pairs one source occurrence with its fresh destination.
type jsonCloneFrame struct {
	source  *jsonValue
	clone   *jsonValue
	context string
	exit    bool
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
