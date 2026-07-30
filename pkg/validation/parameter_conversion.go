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

	cursor := newParameterCandidateCursor(validation)

	var (
		firstConversionError error
		firstValidationError error
	)

	for {
		candidate, ok := cursor.next()
		if !ok {
			break
		}

		if validationImpossible(candidate) {
			continue
		}

		value, conversionError, validationError := attemptParameterCandidate(candidate, convert)
		if conversionError != nil {
			if firstConversionError == nil {
				firstConversionError = conversionError
			}

			continue
		}

		if validationError != nil {
			if firstValidationError == nil {
				firstValidationError = validationError
			}

			continue
		}

		return value, nil
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

// parameterCandidateCursor lazily enumerates depth-first composition choices.
// It retains only one choice index per currently reachable anyOf occurrence.
type parameterCandidateCursor struct {
	root    *Validation
	choices []int
	arities []int
	started bool
	done    bool
}

// newParameterCandidateCursor starts before the first source-ordered choice.
func newParameterCandidateCursor(root *Validation) *parameterCandidateCursor {
	return &parameterCandidateCursor{root: root}
}

// next returns the next complete source-ordered composition selection.
func (cursor *parameterCandidateCursor) next() (*Validation, bool) {
	if cursor.done {
		return nil, false
	}

	if cursor.started && !cursor.advance() {
		cursor.done = true

		return nil, false
	}

	cursor.started = true
	cursor.arities = cursor.arities[:0]

	position := 0
	candidate := cursor.selectCandidate(cursor.root, &position)
	cursor.choices = cursor.choices[:position]

	return candidate, true
}

// advance increments the deepest choice that has a remaining alternative.
func (cursor *parameterCandidateCursor) advance() bool {
	for position := len(cursor.arities) - 1; position >= 0; position-- {
		if cursor.choices[position]+1 >= cursor.arities[position] {
			continue
		}

		cursor.choices[position]++
		cursor.choices = cursor.choices[:position+1]

		return true
	}

	return false
}

// selectCandidate builds only the current choice and retains every conjunctive rule.
func (cursor *parameterCandidateCursor) selectCandidate(
	validation *Validation,
	position *int,
) *Validation {
	local := *validation
	local.AllOfValidations = nil
	local.AnyOfValidations = nil

	parts := []*Validation{&local}
	pointer := local.SchemaPointer

	if len(validation.AnyOfValidations) != 0 {
		choice := cursor.choice(len(validation.AnyOfValidations), position)
		selected := cursor.selectCandidate(validation.AnyOfValidations[choice], position)
		parts = append(parts, selected)
		pointer = selected.SchemaPointer
	}

	for _, child := range validation.AllOfValidations {
		selected := cursor.selectCandidate(child, position)

		parts = append(parts, selected)
		if containsAnyOf(child) {
			pointer = selected.SchemaPointer
		}
	}

	candidate := conjunctiveValidation(parts...)
	candidate.SchemaPointer = pointer

	return candidate
}

// choice consumes one deterministic cursor position.
func (cursor *parameterCandidateCursor) choice(arity int, position *int) int {
	if *position == len(cursor.choices) {
		cursor.choices = append(cursor.choices, 0)
	}

	choice := cursor.choices[*position]
	cursor.arities = append(cursor.arities, arity)
	(*position)++

	return choice
}
