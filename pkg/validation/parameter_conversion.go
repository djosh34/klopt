package validation

import (
	"encoding/json"
	"errors"

	"github.com/go-json-experiment/json/jsontext"
)

// convertParameterValue tries direct anyOf alternatives in authored order.
// Each conversion remains private until the selected schema accepts it.
func convertParameterValue(
	validation *Validation,
	convert func(*Validation) (jsontext.Value, error),
) (jsontext.Value, error) {
	if validation == nil || len(validation.AnyOfValidations) == 0 {
		return convert(validation)
	}

	var (
		firstConversionError error
		firstValidationError error
	)

	for _, child := range validation.AnyOfValidations {
		candidate := anyOfCandidate(validation, child)

		value, err := convert(candidate)
		if err != nil {
			if firstConversionError == nil {
				firstConversionError = err
			}

			continue
		}

		if errs := validateRaw(candidate, json.RawMessage(value), "#"); len(errs) != 0 {
			if firstValidationError == nil {
				firstValidationError = errors.Join(errs...)
			}

			continue
		}

		return value, nil
	}

	if firstValidationError != nil {
		return nil, firstValidationError
	}

	if firstConversionError != nil {
		return nil, firstConversionError
	}

	return nil, newValidationError(validation, "#", "anyOf", "value must validate against at least one alternative")
}

// anyOfCandidate combines one authored alternative with its active parent siblings.
func anyOfCandidate(parent *Validation, child *Validation) *Validation {
	candidate := *parent
	candidate.AnyOfValidations = nil

	candidate.AllOfValidations = append(append([]*Validation(nil), parent.AllOfValidations...), child)

	return &candidate
}

// conjunctiveValidation combines already-compiled schemas without flattening them.
func conjunctiveValidation(validations ...*Validation) *Validation {
	if len(validations) == 0 {
		return nil
	}

	if len(validations) == 1 {
		return validations[0]
	}

	return &Validation{
		SchemaPointer:    validations[0].SchemaPointer,
		AllOfValidations: append([]*Validation(nil), validations...),
		ObjectValidation: ObjectValidation{AdditionalPropertiesAllowed: true},
	}
}
