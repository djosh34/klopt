package suite

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/djosh34/klopt/pkg/internal/oas"
)

// compileAllOf folds each allOf child into the local sibling occurrence.
func (compiler *Compiler) compileAllOf(
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	active map[string]struct{},
	result *schemaUse,
) (*schemaUse, error) {
	raw, ok := members["allOf"]
	if !ok {
		return result, nil
	}

	if isJSONNull(raw) {
		return nil, compiler.failure(
			"compile", "malformed", schema.Pointer, "allOf", errors.New("allOf must be a non-empty array"),
		)
	}

	var children []json.RawMessage
	if err := json.Unmarshal(raw, &children); err != nil {
		return nil, compiler.failure("compile", "malformed", schema.Pointer, "allOf", err)
	}

	if len(children) == 0 {
		return nil, compiler.failure(
			"compile", "malformed", schema.Pointer, "allOf",
			errors.New("allOf must contain at least one Schema Object"),
		)
	}

	for index := range children {
		child, err := compiler.Source.Child(schema, "allOf", fmt.Sprintf("%d", index))
		if err != nil {
			return nil, compiler.failure("compile", "malformed", schema.Pointer, "allOf", err)
		}

		childUse, err := compiler.compileSchema(child, active)
		if err != nil {
			return nil, err
		}

		result, err = compiler.meet(result, childUse)
		if err != nil {
			return nil, compiler.allOfMeetFailure(schema.Pointer, err)
		}
	}

	return result, nil
}

// allOfMeetFailure classifies a failed semantic meet.
func (compiler *Compiler) allOfMeetFailure(pointer string, err error) *Error {
	code := "malformed"
	if errors.Is(err, errUnconstructible) {
		code = "unconstructible"
	}

	return compiler.failure("compile", code, pointer, "allOf", err)
}
