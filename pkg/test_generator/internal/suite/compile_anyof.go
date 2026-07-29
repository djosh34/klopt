//nolint:godoclint // Private compiler helpers are documented by their focused behavior tests.
package suite

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/djosh34/klopt/pkg/internal/oas"
)

func (compiler *Compiler) compileAnyOf(
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	active map[string]struct{},
	result *schemaUse,
) error {
	raw, ok := members["anyOf"]
	if !ok {
		return nil
	}

	if isJSONNull(raw) {
		return compiler.failure(
			"compile", "malformed", schema.Pointer, "anyOf", errors.New("anyOf must be a non-empty array"),
		)
	}

	var children []json.RawMessage
	if err := json.Unmarshal(raw, &children); err != nil {
		return compiler.failure("compile", "malformed", schema.Pointer, "anyOf", err)
	}

	if len(children) == 0 {
		return compiler.failure(
			"compile", "malformed", schema.Pointer, "anyOf",
			errors.New("anyOf must contain at least one Schema Object"),
		)
	}

	result.anyOf = make([]*schemaUse, 0, len(children))
	for index := range children {
		child, err := compiler.Source.Child(schema, "anyOf", strconv.Itoa(index))
		if err != nil {
			return compiler.failure("compile", "malformed", schema.Pointer, "anyOf", err)
		}

		childUse, err := compiler.compileSchema(child, active)
		if err != nil {
			return err
		}

		result.anyOf = append(result.anyOf, childUse)
	}

	return nil
}
