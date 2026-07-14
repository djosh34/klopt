package testgenerator

import (
	"encoding/json"
	"errors"
	"testing"

	"decode_and_validate_generator/pkg/validation"
)

// runtimeValidationName identifies the repository runtime validator.
const runtimeValidationName = "pkg/validation"

// runtimeValidationAdapter adapts one parsed validation graph to the private test seam.
type runtimeValidationAdapter struct {
	validation *validation.Validation
}

// newRuntimeValidationRequestBodyValidator parses one operation once per fixture.
func newRuntimeValidationRequestBodyValidator(spec []byte) (validatorAdapter, error) {
	parsed, err := validation.Parse(spec, "checkThing")
	if err != nil {
		return validatorAdapter{}, err
	}

	return validatorAdapter{
		name:      runtimeValidationName,
		validator: runtimeValidationAdapter{validation: parsed},
	}, nil
}

// Validate returns joined runtime rule failures as one body rejection.
func (adapter runtimeValidationAdapter) Validate(body []byte) error {
	errs := adapter.validation.Validate(json.RawMessage(body))
	if len(errs) == 0 {
		return nil
	}

	return bodyRejectionError{err: errors.Join(errs...)}
}

// TestRuntimeValidationCharacterizations pins independent runtime setup and body verdicts.
func TestRuntimeValidationCharacterizations(t *testing.T) {
	t.Parallel()

	runValidatorCharacterizations(t, newRuntimeValidationRequestBodyValidator, runtimeValidationName)
}
