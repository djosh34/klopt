package validation

import (
	"encoding/json"
	"errors"

	"github.com/go-json-experiment/json/jsontext"
)

// convertParameterValue tries one complete composition choice at a time.
// Conversion output is private to an attempt until its selected schema accepts it.
func convertParameterValue(
	validation *Validation,
	convert func(*Validation) (jsontext.Value, error),
) (jsontext.Value, error) {
	if !containsAnyOf(validation) {
		return convert(validation)
	}

	var (
		firstConversionError error
		firstValidationError error
		result               jsontext.Value
	)

	walkParameterCandidates(validation, func(candidate *Validation) bool {
		if validationImpossible(candidate) {
			return true
		}

		value, conversionError, validationError := attemptParameterCandidate(candidate, convert)
		if conversionError != nil {
			if firstConversionError == nil {
				firstConversionError = conversionError
			}

			return true
		}

		if validationError != nil {
			if firstValidationError == nil {
				firstValidationError = validationError
			}

			return true
		}

		result = value

		return false
	})

	if result != nil {
		return result, nil
	}

	return nil, parameterConversionFailure(firstValidationError, firstConversionError)
}

// attemptParameterCandidate isolates conversion output and validates it before commit.
func attemptParameterCandidate(
	candidate *Validation,
	convert func(*Validation) (jsontext.Value, error),
) (jsontext.Value, error, error) {
	value, err := convert(candidate)
	if err != nil {
		return nil, err, nil
	}

	if errs := validateRaw(candidate, json.RawMessage(value), "#"); len(errs) != 0 {
		return nil, nil, errors.Join(errs...)
	}

	return value, nil, nil
}

// parameterConversionFailure applies deterministic all-candidate error precedence.
func parameterConversionFailure(validationError error, conversionError error) error {
	if validationError != nil {
		return validationError
	}

	if conversionError != nil {
		return conversionError
	}

	return errors.New("value does not match anyOf")
}

// walkParameterCandidates tries source-ordered choices transactionally and retains
// only the active recursion path. Returning false stops after one accepted conversion.
func walkParameterCandidates(validation *Validation, visit func(*Validation) bool) bool {
	local := *validation
	local.AllOfValidations = nil
	local.AnyOfValidations = nil

	if len(validation.AnyOfValidations) != 0 {
		for _, alternative := range validation.AnyOfValidations {
			keepGoing := walkParameterCandidates(alternative, func(selected *Validation) bool {
				return walkParameterAllOf(
					validation,
					0,
					[]*Validation{&local, selected},
					selected.SchemaPointer,
					visit,
				)
			})
			if !keepGoing {
				return false
			}
		}

		return true
	}

	return walkParameterAllOf(
		validation, 0, []*Validation{&local}, local.SchemaPointer, visit,
	)
}

// walkParameterAllOf visits the active allOf path without materializing sibling combinations.
func walkParameterAllOf(
	validation *Validation,
	index int,
	parts []*Validation,
	pointer string,
	visit func(*Validation) bool,
) bool {
	if index == len(validation.AllOfValidations) {
		candidate := conjunctiveValidation(parts...)
		candidate.SchemaPointer = pointer

		return visit(candidate)
	}

	child := validation.AllOfValidations[index]

	return walkParameterCandidates(child, func(selected *Validation) bool {
		selectedPointer := pointer
		if containsAnyOf(child) {
			selectedPointer = selected.SchemaPointer
		}

		return walkParameterAllOf(
			validation,
			index+1,
			append(parts, selected),
			selectedPointer,
			visit,
		)
	})
}
