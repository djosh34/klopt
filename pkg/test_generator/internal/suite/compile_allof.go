package suite

import (
	"encoding/json"
	"errors"
	"fmt"

	//nolint:depguard // allOf compilation intentionally depends on internal/jsonvalue.
	"decode_and_validate_generator/pkg/test_generator/internal/jsonvalue"
	//nolint:depguard // allOf compilation intentionally consumes located internal/oas schemas.
	"decode_and_validate_generator/pkg/test_generator/internal/oas"
)

// compileAllOf folds each allOf child into the local sibling Domain.
func (compiler *Compiler) compileAllOf(
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	active map[string]struct{},
	result DomainID,
	constraints []ConstraintSource,
	examples GenerationExamples,
) (DomainID, []ConstraintSource, GenerationExamples, error) {
	raw, ok := members["allOf"]
	if !ok {
		return result, constraints, examples, nil
	}

	if isJSONNull(raw) {
		return NoDomain, nil, GenerationExamples{}, compiler.failure(
			"compile", "malformed", schema.Pointer, "allOf", errors.New("allOf must be a non-empty array"),
		)
	}

	var children []json.RawMessage
	if err := json.Unmarshal(raw, &children); err != nil {
		return NoDomain, nil, GenerationExamples{}, compiler.failure(
			"compile", "malformed", schema.Pointer, "allOf", err,
		)
	}

	if len(children) == 0 {
		return NoDomain, nil, GenerationExamples{}, compiler.failure(
			"compile",
			"malformed",
			schema.Pointer,
			"allOf",
			errors.New("allOf must contain at least one Schema Object"),
		)
	}

	schemaWideValid := cloneJSONValues(examples.Valid)

	for index := range children {
		childResult, childConstraints, mergedExamples, err := compiler.compileAllOfChild(
			schema,
			index,
			active,
			result,
			examples,
			schemaWideValid,
		)
		if err != nil {
			return NoDomain, nil, GenerationExamples{}, err
		}

		result = childResult
		examples = mergedExamples

		constraints = append(constraints, childConstraints...)
	}

	return result, constraints, examples, nil
}

// compileAllOfChild intersects one allOf child and merges its source metadata.
func (compiler *Compiler) compileAllOfChild(
	schema oas.LocatedSchema,
	index int,
	active map[string]struct{},
	result DomainID,
	examples GenerationExamples,
	schemaWideValid []jsonvalue.Value,
) (DomainID, []ConstraintSource, GenerationExamples, error) {
	child, err := compiler.Source.Child(schema, "allOf", fmt.Sprintf("%d", index))
	if err != nil {
		return NoDomain, nil, GenerationExamples{}, compiler.failure(
			"compile", "malformed", schema.Pointer, "allOf", err,
		)
	}

	childID, err := compiler.compileSchema(child, active)
	if err != nil {
		return NoDomain, nil, GenerationExamples{}, err
	}

	childConstraints, childExamples := compiler.metadataForPointer(child.Pointer)
	leftDomain, leftOK := compiler.Domains.Domain(result)

	rightDomain, rightOK := compiler.Domains.Domain(childID)
	if !leftOK || !rightOK {
		return NoDomain, nil, GenerationExamples{}, compiler.failure(
			"compile", "malformed", schema.Pointer, "allOf", errors.New("compiled allOf Domain does not exist"),
		)
	}

	enumNeedsStringExamples := enumCrossesStringLanguage(leftDomain, rightDomain)
	mergedExamples := mergeGenerationExamples(examples, childExamples)
	compatibleExamples, needsStringExamples := compatibleStringExamples(
		leftDomain,
		rightDomain,
		examples.Valid,
		childExamples.Valid,
	)

	if len(schemaWideValid) > 0 {
		mergedExamples.Valid = cloneJSONValues(schemaWideValid)
	} else if needsStringExamples {
		mergedExamples.Valid = compatibleExamples
	}

	result, mergedDomain, err := compiler.intersectAllOfDomains(schema.Pointer, result, childID)
	if err != nil {
		return NoDomain, nil, GenerationExamples{}, err
	}

	result, mergedDomain, err = compiler.refineAllOfEnum(
		schema.Pointer,
		result,
		mergedDomain,
		mergedExamples.Valid,
		enumNeedsStringExamples,
	)
	if err != nil {
		return NoDomain, nil, GenerationExamples{}, err
	}

	if err := compiler.validateAllOfStringExamples(
		schema.Pointer,
		mergedDomain,
		needsStringExamples,
		mergedExamples.Valid,
	); err != nil {
		return NoDomain, nil, GenerationExamples{}, err
	}

	return result, childConstraints, mergedExamples, nil
}

// intersectAllOfDomains intersects two Domains and returns the merged Domain.
func (compiler *Compiler) intersectAllOfDomains(
	pointer string,
	left DomainID,
	right DomainID,
) (DomainID, Domain, error) {
	result, err := compiler.Domains.IntersectDomains(left, right)
	if err != nil {
		code := "malformed"
		if errors.Is(err, errUnconstructible) {
			code = "unconstructible"
		}

		return NoDomain, Domain{}, compiler.failure("compile", code, pointer, "allOf", err)
	}

	mergedDomain, ok := compiler.Domains.Domain(result)
	if !ok {
		return NoDomain, Domain{}, compiler.failure(
			"compile", "malformed", pointer, "allOf", errors.New("merged allOf Domain does not exist"),
		)
	}

	return result, mergedDomain, nil
}

// refineAllOfEnum keeps only enum strings backed by trusted compatible examples.
func (compiler *Compiler) refineAllOfEnum(
	pointer string,
	result DomainID,
	domain Domain,
	validExamples []jsonvalue.Value,
	needsStringExamples bool,
) (DomainID, Domain, error) {
	if !needsStringExamples || domain.Enum == nil {
		return result, domain, nil
	}

	constructiveValues, hadString := enumValuesBackedByExamples(domain.Enum, validExamples)
	if hadString && len(constructiveValues) != len(domain.Enum.Values) {
		return NoDomain, Domain{}, compiler.failure(
			"compile",
			"unconstructible",
			pointer,
			"allOf",
			errors.New("enum string with pattern or format has no compatible trusted valid generation example"),
		)
	}

	result = compiler.Domains.FindOrAddEquivalentDomain(finiteDomain(constructiveValues))

	refinedDomain, ok := compiler.Domains.Domain(result)
	if !ok {
		return NoDomain, Domain{}, compiler.failure(
			"compile", "malformed", pointer, "allOf", errors.New("constructive enum Domain does not exist"),
		)
	}

	return result, refinedDomain, nil
}

// validateAllOfStringExamples requires a trusted example for an unmodeled string conjunction.
func (compiler *Compiler) validateAllOfStringExamples(
	pointer string,
	domain Domain,
	needsStringExamples bool,
	validExamples []jsonvalue.Value,
) error {
	if domain.String.State == KindExcluded || !needsStringExamples || len(validExamples) > 0 {
		return nil
	}

	return compiler.failure(
		"compile",
		"unconstructible",
		pointer,
		"allOf",
		errors.New("pattern or format conjunction has no compatible trusted valid generation examples"),
	)
}

// enumCrossesStringLanguage reports whether an enum meets an unmodeled string language.
func enumCrossesStringLanguage(left Domain, right Domain) bool {
	leftLanguage := len(left.String.Patterns) > 0 || len(left.String.Formats) > 0
	rightLanguage := len(right.String.Patterns) > 0 || len(right.String.Formats) > 0

	return left.Enum != nil && rightLanguage || right.Enum != nil && leftLanguage
}

// enumValuesBackedByExamples retains non-strings and trusted example-backed strings.
func enumValuesBackedByExamples(enum *EnumSet, examples []jsonvalue.Value) ([]jsonvalue.Value, bool) {
	values := make([]jsonvalue.Value, 0, len(enum.Values))
	hadString := false

	for _, value := range enum.Values {
		if value.Kind != jsonvalue.KindString {
			values = append(values, cloneJSONValue(value))

			continue
		}

		hadString = true

		if jsonValuesContain(examples, value) {
			values = append(values, cloneJSONValue(value))
		}
	}

	return values, hadString
}

// compatibleStringExamples chooses trusted inputs for a newly strengthened pattern/format conjunction.
func compatibleStringExamples(
	left Domain,
	right Domain,
	leftExamples []jsonvalue.Value,
	rightExamples []jsonvalue.Value,
) ([]jsonvalue.Value, bool) {
	if left.String.State == KindExcluded || right.String.State == KindExcluded {
		return nil, false
	}

	leftLanguages := stringLanguages(left.String)

	rightLanguages := stringLanguages(right.String)
	if len(leftLanguages) == 0 || len(rightLanguages) == 0 ||
		stringsContainAll(leftLanguages, rightLanguages) && stringsContainAll(rightLanguages, leftLanguages) {
		return nil, false
	}

	if stringsContainAll(leftLanguages, rightLanguages) {
		return cloneJSONValues(leftExamples), true
	}

	if stringsContainAll(rightLanguages, leftLanguages) {
		return cloneJSONValues(rightExamples), true
	}

	return intersectJSONValues(leftExamples, rightExamples), true
}

// stringLanguages gives patterns and formats distinct set keys.
func stringLanguages(constraints StringConstraints) []string {
	languages := make([]string, 0, len(constraints.Patterns)+len(constraints.Formats))
	for _, pattern := range constraints.Patterns {
		languages = append(languages, "pattern:"+pattern)
	}

	for _, format := range constraints.Formats {
		languages = append(languages, "format:"+format)
	}

	return languages
}

// stringsContainAll reports set inclusion for a small string slice.
func stringsContainAll(values []string, wanted []string) bool {
	for _, candidate := range wanted {
		found := false

		for _, value := range values {
			if value == candidate {
				found = true

				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

// mergeGenerationExamples unions trusted examples semantically.
func mergeGenerationExamples(left GenerationExamples, right GenerationExamples) GenerationExamples {
	valid := cloneJSONValues(left.Valid)
	for _, candidate := range right.Valid {
		if !jsonValuesContain(valid, candidate) {
			valid = append(valid, cloneJSONValue(candidate))
		}
	}

	invalid := cloneJSONValues(left.Invalid)
	for _, candidate := range right.Invalid {
		if !jsonValuesContain(invalid, candidate) {
			invalid = append(invalid, cloneJSONValue(candidate))
		}
	}

	return GenerationExamples{Valid: valid, Invalid: invalid}
}

// intersectJSONValues intersects exact semantic JSON values in left order.
func intersectJSONValues(left []jsonvalue.Value, right []jsonvalue.Value) []jsonvalue.Value {
	result := make([]jsonvalue.Value, 0, min(len(left), len(right)))
	for _, candidate := range left {
		if jsonValuesContain(right, candidate) && !jsonValuesContain(result, candidate) {
			result = append(result, cloneJSONValue(candidate))
		}
	}

	return result
}

// jsonValuesContain reports semantic JSON set membership.
func jsonValuesContain(values []jsonvalue.Value, candidate jsonvalue.Value) bool {
	for _, value := range values {
		if value.Equal(candidate) {
			return true
		}
	}

	return false
}
