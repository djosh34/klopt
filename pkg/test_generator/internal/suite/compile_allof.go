package suite

import (
	"encoding/json"
	"errors"
	"fmt"

	"decode_and_validate_generator/pkg/internal/jsonvalue"
	"decode_and_validate_generator/pkg/internal/oas"
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

// allOfMeetFailure preserves exact oracle errors and classifies other meet failures.
func (compiler *Compiler) allOfMeetFailure(pointer string, err error) *Error {
	var overlap *generationOverlapError
	if errors.As(err, &overlap) {
		return compiler.failure(
			"compile", "malformed", overlap.Example.Source.Pointer,
			overlap.Example.Source.Keyword, errors.New(overlap.Error()),
		)
	}

	code := "malformed"
	if errors.Is(err, errUnconstructible) {
		code = "unconstructible"
	}

	return compiler.failure("compile", code, pointer, "allOf", err)
}

// meetGenerationExamples intersects independently declared valid case sets. A case
// declared by only one side is checked only against the other occurrence's Domain.
func (compiler *Compiler) meetGenerationExamples(
	left GenerationExamples,
	leftDomain Domain,
	right GenerationExamples,
	rightDomain Domain,
) (GenerationExamples, error) {
	valid, err := compiler.meetValidGenerationExamples(left, leftDomain, right, rightDomain)
	if err != nil {
		return GenerationExamples{}, err
	}

	result := GenerationExamples{
		Valid:         valid,
		ValidDeclared: left.ValidDeclared || right.ValidDeclared,
	}

	result.Invalid = cloneGenerationExamples(left.Invalid)
	for _, candidate := range right.Invalid {
		appendGenerationExample(&result.Invalid, candidate)
	}

	return result, nil
}

// meetValidGenerationExamples meets exact valid cases at occurrence boundaries.
func (compiler *Compiler) meetValidGenerationExamples(
	left GenerationExamples,
	leftDomain Domain,
	right GenerationExamples,
	rightDomain Domain,
) ([]GenerationExample, error) {
	var result []GenerationExample

	switch {
	case left.ValidDeclared && right.ValidDeclared:
		for _, candidate := range left.Valid {
			if generationExamplesContain(right.Valid, candidate.Value) {
				appendGenerationExample(&result, candidate)
			}
		}
	case left.ValidDeclared:
		return compiler.generationExamplesWithinDomain(left.Valid, rightDomain)
	case right.ValidDeclared:
		return compiler.generationExamplesWithinDomain(right.Valid, leftDomain)
	}

	return result, nil
}

// generationExamplesWithinDomain retains exact cases accepted by a separate occurrence.
func (compiler *Compiler) generationExamplesWithinDomain(
	examples []GenerationExample,
	domain Domain,
) ([]GenerationExample, error) {
	result := make([]GenerationExample, 0, len(examples))
	for _, example := range examples {
		matches, err := compiler.valueFitsDomain(example.Value, domain)
		if err != nil {
			return nil, err
		}

		if matches {
			appendGenerationExample(&result, example)
		}
	}

	return result, nil
}

// generationExamplesContain reports semantic membership in an occurrence case set.
func generationExamplesContain(examples []GenerationExample, candidate jsonvalue.Value) bool {
	for _, example := range examples {
		if example.Value.Equal(candidate) {
			return true
		}
	}

	return false
}
