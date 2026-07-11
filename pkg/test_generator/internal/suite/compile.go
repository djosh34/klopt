package suite

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"unicode/utf8"

	//nolint:depguard // Internal suite architecture intentionally depends on internal/jsonvalue.
	"decode_and_validate_generator/pkg/test_generator/internal/jsonvalue"
	//nolint:depguard // Internal suite architecture intentionally depends on internal/oas.
	"decode_and_validate_generator/pkg/test_generator/internal/oas"
)

// exactDecimalRadix is the JSON number radix used for symbolic exponent comparison.
const exactDecimalRadix = 10

// Compiler compiles located Schema Objects into canonical DomainIDs.
type Compiler struct {
	Source               oas.Source
	Domains              *DomainRegistry
	DomainByPointer      map[string]DomainID
	LocalDomainByPointer map[string]DomainID
	SchemaUses           []SchemaUse
}

// NewCompiler creates a Compiler for one located OpenAPI source.
func NewCompiler(source oas.Source) *Compiler {
	return &Compiler{
		Source:               source,
		Domains:              NewDomainRegistry(),
		DomainByPointer:      make(map[string]DomainID),
		LocalDomainByPointer: make(map[string]DomainID),
		SchemaUses:           make([]SchemaUse, 0),
	}
}

// Compile compiles the selected request Schema Object.
func (compiler *Compiler) Compile() (DomainID, error) {
	return compiler.CompileSchema(compiler.Source.RequestSchema)
}

// CompileSchema compiles one inline Schema Object or Reference Object.
func (compiler *Compiler) CompileSchema(schema oas.LocatedSchema) (DomainID, error) {
	return compiler.compileSchema(schema, make(map[string]struct{}))
}

// compileSchema resolves and compiles one schema occurrence.
func (compiler *Compiler) compileSchema(
	occurrence oas.LocatedSchema,
	active map[string]struct{},
) (DomainID, error) {
	if id, ok := compiler.DomainByPointer[occurrence.Pointer]; ok {
		return id, nil
	}

	resolved, err := compiler.Source.Resolve(occurrence)
	if err != nil {
		return NoDomain, compiler.failure("compile", "unsupported", occurrence.Pointer, "$ref", err)
	}

	if id, ok := compiler.DomainByPointer[resolved.Pointer]; ok {
		compiler.recordResolvedUse(occurrence.Pointer, resolved.Pointer, id)

		return id, nil
	}

	if _, recursive := active[resolved.Pointer]; recursive {
		return NoDomain, compiler.recursiveReferenceFailure(resolved.Pointer)
	}

	return compiler.compileResolvedSchema(occurrence, resolved, active)
}

// recordResolvedUse records an occurrence that reused a resolved schema Domain.
func (compiler *Compiler) recordResolvedUse(occurrencePointer string, resolvedPointer string, id DomainID) {
	compiler.DomainByPointer[occurrencePointer] = id
	compiler.LocalDomainByPointer[occurrencePointer] = compiler.LocalDomainByPointer[resolvedPointer]
	constraints, examples := compiler.metadataForPointer(resolvedPointer)
	compiler.addSchemaUse(occurrencePointer, id, constraints, examples)
}

// recursiveReferenceFailure reports a reference cycle unsupported by this compiler step.
func (compiler *Compiler) recursiveReferenceFailure(pointer string) *Error {
	return compiler.failure(
		"compile",
		"unsupported",
		pointer,
		"$ref",
		errors.New("recursive schema references are unsupported"),
	)
}

// compileResolvedSchema compiles a resolved schema and records both of its pointers.
func (compiler *Compiler) compileResolvedSchema(
	occurrence oas.LocatedSchema,
	resolved oas.LocatedSchema,
	active map[string]struct{},
) (DomainID, error) {
	active[resolved.Pointer] = struct{}{}
	defer delete(active, resolved.Pointer)

	members, err := schemaMembers(resolved)
	if err != nil {
		return NoDomain, compiler.failure("compile", "malformed", resolved.Pointer, "", err)
	}

	if validationErr := validateSchemaKeywords(members); validationErr != nil {
		return NoDomain, compiler.failure("compile", "malformed", resolved.Pointer, "", validationErr)
	}

	if keyword := unsupportedKeyword(members); keyword != "" {
		return NoDomain, compiler.unsupportedKeywordFailure(resolved.Pointer, keyword)
	}

	domain, constraints, examples, err := compiler.compileSchemaDomain(resolved, members, active)
	if err != nil {
		return NoDomain, err
	}

	id := compiler.Domains.FindOrAddEquivalentDomain(domain)
	compiler.LocalDomainByPointer[resolved.Pointer] = id
	compiler.LocalDomainByPointer[occurrence.Pointer] = id

	id, constraints, examples, err = compiler.compileAllOf(resolved, members, active, id, constraints, examples)
	if err != nil {
		return NoDomain, err
	}

	compiler.DomainByPointer[resolved.Pointer] = id
	compiler.DomainByPointer[occurrence.Pointer] = id
	compiler.addSchemaUse(occurrence.Pointer, id, constraints, examples)

	if occurrence.Pointer != resolved.Pointer {
		compiler.addSchemaUse(resolved.Pointer, id, constraints, examples)
	}

	return id, nil
}

// unsupportedKeywordFailure reports a known Schema Object keyword not supported by this step.
func (compiler *Compiler) unsupportedKeywordFailure(pointer string, keyword string) *Error {
	return compiler.failure(
		"compile",
		"unsupported",
		pointer,
		keyword,
		fmt.Errorf("%s is unsupported", keyword),
	)
}

// compileSchemaDomain compiles all schema keywords into a Domain and source metadata.
func (compiler *Compiler) compileSchemaDomain(
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	active map[string]struct{},
) (Domain, []ConstraintSource, GenerationExamples, error) {
	examples, err := compileGenerationExamples(members)
	if err != nil {
		return Domain{}, nil, GenerationExamples{}, compiler.failure("compile", "malformed", schema.Pointer, "", err)
	}

	domain := anyJSONDomain()
	constraints := make([]ConstraintSource, 0, len(members))

	hasType, err := applyTypeAndNullable(&domain, members)
	if err != nil {
		return Domain{}, nil, GenerationExamples{}, compiler.failure("compile", "malformed", schema.Pointer, "type", err)
	}

	if hasType {
		constraints = append(constraints, ConstraintSource{Pointer: schema.Pointer, Keyword: "type"})
	}

	if err := compiler.compileNumber(&domain, members); err != nil {
		return Domain{}, nil, GenerationExamples{}, compiler.failure("compile", "malformed", schema.Pointer, "number", err)
	}

	if err := compileString(&domain, members); err != nil {
		return Domain{}, nil, GenerationExamples{}, compiler.failure("compile", "malformed", schema.Pointer, "string", err)
	}

	if err := compiler.compileArray(&domain, schema, members, active); err != nil {
		return Domain{}, nil, GenerationExamples{}, err
	}

	if err := compiler.compileObject(&domain, schema, members, active); err != nil {
		return Domain{}, nil, GenerationExamples{}, err
	}

	constraints = append(constraints, constraintSources(schema.Pointer, members)...)
	if err := eliminateContradictoryKinds(&domain); err != nil {
		return Domain{}, nil, GenerationExamples{}, compiler.failure("compile", "unconstructible", schema.Pointer, "", err)
	}

	if err := compiler.applyEnum(&domain, schema.Pointer, members, examples); err != nil {
		return Domain{}, nil, GenerationExamples{}, err
	}

	return domain, constraints, examples, nil
}

// applyEnum replaces a Domain with the compatible finite enum values when enum is present.
func (compiler *Compiler) applyEnum(
	domain *Domain,
	pointer string,
	members map[string]json.RawMessage,
	examples GenerationExamples,
) error {
	raw, ok := members["enum"]
	if !ok {
		return nil
	}

	values, err := compiler.compileEnum(raw, *domain, examples.Valid)
	if err != nil {
		code := "malformed"
		if errors.Is(err, errUnconstructible) {
			code = "unconstructible"
		}

		return compiler.failure("compile", code, pointer, "enum", err)
	}

	*domain = finiteDomain(values)

	return nil
}

// schemaMembers decodes the JSON object members of a located Schema Object.
func schemaMembers(schema oas.LocatedSchema) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(schema.Raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, errors.New("schema object must be a JSON object")
	}

	var members map[string]json.RawMessage
	if err := json.Unmarshal(schema.Raw, &members); err != nil {
		return nil, fmt.Errorf("decode Schema Object: %w", err)
	}

	return members, nil
}

// validateSchemaKeywords rejects unknown non-extension Schema Object keywords.
func validateSchemaKeywords(members map[string]json.RawMessage) error {
	if raw, ok := members["uniqueItems"]; ok {
		if isJSONNull(raw) {
			return errors.New("uniqueItems must be a boolean")
		}

		var unique bool
		if err := json.Unmarshal(raw, &unique); err != nil {
			return fmt.Errorf("uniqueItems must be a boolean: %w", err)
		}
	}

	known := map[string]struct{}{
		"title": {}, "multipleOf": {}, "maximum": {}, "exclusiveMaximum": {},
		"minimum": {}, "exclusiveMinimum": {}, "maxLength": {}, "minLength": {},
		"pattern": {}, "maxItems": {}, "minItems": {}, "uniqueItems": {}, "items": {},
		"maxProperties": {}, "minProperties": {}, "required": {}, "properties": {},
		"additionalProperties": {}, "description": {}, "format": {}, "$ref": {},
		"nullable": {}, "readOnly": {}, "writeOnly": {}, "xml": {}, "externalDocs": {},
		"example": {}, "deprecated": {}, "type": {}, "enum": {}, "default": {},
		"allOf": {}, "oneOf": {}, "anyOf": {}, "not": {}, "discriminator": {},
	}
	for keyword := range members {
		if _, ok := known[keyword]; ok || strings.HasPrefix(keyword, "x-") {
			continue
		}

		return fmt.Errorf("unknown Schema Object keyword %q", keyword)
	}

	return nil
}

// unsupportedKeyword returns the first recognized keyword unsupported by this step.
func unsupportedKeyword(members map[string]json.RawMessage) string {
	for _, keyword := range []string{"oneOf", "anyOf", "not", "discriminator"} {
		if _, ok := members[keyword]; ok {
			return keyword
		}
	}

	if raw, ok := members["uniqueItems"]; ok {
		var unique bool
		if err := json.Unmarshal(raw, &unique); err == nil && unique {
			return "uniqueItems"
		}
	}

	return ""
}

// compileAllOf folds each allOf child into the local sibling Domain.
//
//nolint:cyclop,gocognit // Source metadata and distinct failure outcomes belong at this seam.
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
	if err := json.Unmarshal(raw, &children); err != nil || len(children) == 0 {
		if err == nil {
			err = errors.New("allOf must contain at least one Schema Object")
		}

		return NoDomain, nil, GenerationExamples{}, compiler.failure(
			"compile", "malformed", schema.Pointer, "allOf", err,
		)
	}

	schemaWideValid := cloneJSONValues(examples.Valid)

	for index := range children {
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

		compatibleStringExamples, needsStringExamples := compatibleStringExamples(
			leftDomain,
			rightDomain,
			examples.Valid,
			childExamples.Valid,
		)
		if len(schemaWideValid) > 0 {
			mergedExamples.Valid = cloneJSONValues(schemaWideValid)
		} else if needsStringExamples {
			mergedExamples.Valid = compatibleStringExamples
		}

		result, err = compiler.Domains.IntersectDomains(result, childID)
		if err != nil {
			code := "malformed"
			if errors.Is(err, errUnconstructible) {
				code = "unconstructible"
			}

			return NoDomain, nil, GenerationExamples{}, compiler.failure(
				"compile", code, schema.Pointer, "allOf", err,
			)
		}

		mergedDomain, mergedOK := compiler.Domains.Domain(result)
		if !mergedOK {
			return NoDomain, nil, GenerationExamples{}, compiler.failure(
				"compile", "malformed", schema.Pointer, "allOf", errors.New("merged allOf Domain does not exist"),
			)
		}

		if enumNeedsStringExamples && mergedDomain.Enum != nil {
			constructiveValues, hadString := enumValuesBackedByExamples(mergedDomain.Enum, mergedExamples.Valid)
			if hadString && len(constructiveValues) != len(mergedDomain.Enum.Values) {
				return NoDomain, nil, GenerationExamples{}, compiler.failure(
					"compile",
					"unconstructible",
					schema.Pointer,
					"allOf",
					errors.New("enum string with pattern or format has no compatible trusted valid generation example"),
				)
			}

			result = compiler.Domains.FindOrAddEquivalentDomain(finiteDomain(constructiveValues))

			mergedDomain, mergedOK = compiler.Domains.Domain(result)
			if !mergedOK {
				return NoDomain, nil, GenerationExamples{}, compiler.failure(
					"compile", "malformed", schema.Pointer, "allOf", errors.New("constructive enum Domain does not exist"),
				)
			}
		}

		if mergedDomain.String.State != KindExcluded && needsStringExamples && len(mergedExamples.Valid) == 0 {
			return NoDomain, nil, GenerationExamples{}, compiler.failure(
				"compile",
				"unconstructible",
				schema.Pointer,
				"allOf",
				errors.New("pattern or format conjunction has no compatible trusted valid generation examples"),
			)
		}

		examples = mergedExamples

		constraints = append(constraints, childConstraints...)
	}

	return result, constraints, examples, nil
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

// applyTypeAndNullable applies the type-restricted kinds and its nullable modifier.
func applyTypeAndNullable(domain *Domain, members map[string]json.RawMessage) (bool, error) {
	typeRaw, hasType := members["type"]
	if !hasType {
		return false, validateNullable(members)
	}

	if isJSONNull(typeRaw) {
		return false, errors.New("type must be a string")
	}

	var schemaType string
	if err := json.Unmarshal(typeRaw, &schemaType); err != nil {
		return false, fmt.Errorf("type must be a string: %w", err)
	}

	excludeAllKinds(domain)

	if err := enableSchemaType(domain, schemaType); err != nil {
		return false, err
	}

	return true, applyNullable(domain, members)
}

// validateNullable validates nullable when it cannot affect an untyped schema.
func validateNullable(members map[string]json.RawMessage) error {
	if _, ok := members["nullable"]; !ok {
		return nil
	}

	_, err := nullableValue(members)

	return err
}

// applyNullable enables null when the nullable Schema Object modifier is true.
func applyNullable(domain *Domain, members map[string]json.RawMessage) error {
	nullable, err := nullableValue(members)
	if err != nil || !nullable {
		return err
	}

	domain.Null = KindUnrestricted

	return nil
}

// nullableValue returns nullable, treating its absence as false.
func nullableValue(members map[string]json.RawMessage) (bool, error) {
	raw, ok := members["nullable"]
	if !ok {
		return false, nil
	}

	if isJSONNull(raw) {
		return false, errors.New("nullable must be a boolean")
	}

	var nullable bool
	if err := json.Unmarshal(raw, &nullable); err != nil {
		return false, fmt.Errorf("nullable must be a boolean: %w", err)
	}

	return nullable, nil
}

// excludeAllKinds prevents every JSON kind until type enables its selected kind.
func excludeAllKinds(domain *Domain) {
	domain.Null = KindExcluded
	domain.Boolean = KindExcluded
	domain.Number.State = KindExcluded
	domain.String.State = KindExcluded
	domain.Array.State = KindExcluded
	domain.Object.State = KindExcluded
}

// enableSchemaType enables the JSON kind selected by an OpenAPI Schema Object type.
func enableSchemaType(domain *Domain, schemaType string) error {
	switch schemaType {
	case "boolean":
		domain.Boolean = KindUnrestricted
	case "number":
		domain.Number.State = KindUnrestricted
	case "integer":
		domain.Number = NumberConstraints{State: KindRestricted, IntegersOnly: true}
	case "string":
		domain.String.State = KindUnrestricted
	case "array":
		domain.Array = ArrayConstraints{State: KindUnrestricted, Items: AnyJSONDomainID}
	case "object":
		domain.Object = ObjectConstraints{
			State:      KindUnrestricted,
			Additional: AdditionalProperties{Values: AnyJSONDomainID},
		}
	default:
		return fmt.Errorf("unsupported OpenAPI type %q", schemaType)
	}

	return nil
}

// compileNumber applies numeric Schema Object keywords to a Domain.
func (compiler *Compiler) compileNumber(domain *Domain, members map[string]json.RawMessage) error {
	number := &domain.Number
	if err := compileNumberBounds(number, members); err != nil {
		return err
	}

	if err := compileExclusiveBounds(number, members); err != nil {
		return err
	}

	return compileNumberFormat(number, members)
}

// compileNumberBounds applies minimum, maximum, and multipleOf to number constraints.
func compileNumberBounds(number *NumberConstraints, members map[string]json.RawMessage) error {
	if err := compileMinimum(number, members); err != nil {
		return err
	}

	if err := compileMaximum(number, members); err != nil {
		return err
	}

	return compileMultipleOf(number, members)
}

// compileMinimum applies minimum when present.
func compileMinimum(number *NumberConstraints, members map[string]json.RawMessage) error {
	raw, ok := members["minimum"]
	if !ok {
		return nil
	}

	value, err := parseExactNumber(raw)
	if err != nil {
		return fmt.Errorf("minimum: %w", err)
	}

	number.Minimum = &NumberBound{Value: value}
	restrictNumber(number)

	return nil
}

// compileMaximum applies maximum when present.
func compileMaximum(number *NumberConstraints, members map[string]json.RawMessage) error {
	raw, ok := members["maximum"]
	if !ok {
		return nil
	}

	value, err := parseExactNumber(raw)
	if err != nil {
		return fmt.Errorf("maximum: %w", err)
	}

	number.Maximum = &NumberBound{Value: value}
	restrictNumber(number)

	return nil
}

// compileMultipleOf applies multipleOf when present.
func compileMultipleOf(number *NumberConstraints, members map[string]json.RawMessage) error {
	raw, ok := members["multipleOf"]
	if !ok {
		return nil
	}

	value, err := parseExactNumber(raw)
	if err != nil {
		return fmt.Errorf("multipleOf: %w", err)
	}

	if compareNumberToZero(value) <= 0 {
		return errors.New("multipleOf must be greater than zero")
	}

	number.MultipleOf = &value
	restrictNumber(number)

	return nil
}

// restrictNumber marks number constraints as restricted unless their kind is excluded.
func restrictNumber(number *NumberConstraints) {
	if number.State != KindExcluded {
		number.State = KindRestricted
	}
}

// compileExclusiveBounds applies exclusiveMinimum and exclusiveMaximum when their bounds exist.
func compileExclusiveBounds(number *NumberConstraints, members map[string]json.RawMessage) error {
	if err := compileExclusiveMinimum(number, members); err != nil {
		return err
	}

	return compileExclusiveMaximum(number, members)
}

// compileExclusiveMinimum applies exclusiveMinimum to an existing minimum bound.
func compileExclusiveMinimum(number *NumberConstraints, members map[string]json.RawMessage) error {
	exclusive, present, err := parseOptionalBool(members, "exclusiveMinimum")
	if err != nil || !present || number.Minimum == nil {
		return err
	}

	number.Minimum.Exclusive = exclusive

	return nil
}

// compileExclusiveMaximum applies exclusiveMaximum to an existing maximum bound.
func compileExclusiveMaximum(number *NumberConstraints, members map[string]json.RawMessage) error {
	exclusive, present, err := parseOptionalBool(members, "exclusiveMaximum")
	if err != nil || !present || number.Maximum == nil {
		return err
	}

	number.Maximum.Exclusive = exclusive

	return nil
}

// parseOptionalBool decodes an optional boolean Schema Object keyword.
func parseOptionalBool(members map[string]json.RawMessage, keyword string) (bool, bool, error) {
	raw, ok := members[keyword]
	if !ok {
		return false, false, nil
	}

	if isJSONNull(raw) {
		return false, true, fmt.Errorf("%s must be a boolean", keyword)
	}

	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, true, fmt.Errorf("%s must be a boolean: %w", keyword, err)
	}

	return value, true, nil
}

// compileNumberFormat applies format to a reachable numeric kind.
func compileNumberFormat(number *NumberConstraints, members map[string]json.RawMessage) error {
	raw, ok := members["format"]
	if !ok || number.State == KindExcluded {
		return nil
	}

	format, err := parseString(raw, "format")
	if err != nil {
		return err
	}

	number.Format = &format
	number.State = KindRestricted

	return nil
}

// compileString applies string Schema Object keywords to a Domain.
func compileString(domain *Domain, members map[string]json.RawMessage) error {
	stringConstraints := &domain.String
	if err := compileStringLengths(stringConstraints, members); err != nil {
		return err
	}

	if err := compileStringPattern(stringConstraints, members); err != nil {
		return err
	}

	return compileStringFormat(stringConstraints, members)
}

// compileStringLengths applies minLength and maxLength when present.
func compileStringLengths(stringConstraints *StringConstraints, members map[string]json.RawMessage) error {
	if err := compileMinLength(stringConstraints, members); err != nil {
		return err
	}

	return compileMaxLength(stringConstraints, members)
}

// compileMinLength applies minLength when present.
func compileMinLength(stringConstraints *StringConstraints, members map[string]json.RawMessage) error {
	raw, ok := members["minLength"]
	if !ok {
		return nil
	}

	value, err := parseNonNegativeInt(raw, "minLength")
	if err != nil {
		return err
	}

	stringConstraints.MinLength = value
	restrictString(stringConstraints)

	return nil
}

// compileMaxLength applies maxLength when present.
func compileMaxLength(stringConstraints *StringConstraints, members map[string]json.RawMessage) error {
	raw, ok := members["maxLength"]
	if !ok {
		return nil
	}

	value, err := parseNonNegativeInt(raw, "maxLength")
	if err != nil {
		return err
	}

	stringConstraints.MaxLength = &value
	restrictString(stringConstraints)

	return nil
}

// compileStringPattern applies pattern when present.
func compileStringPattern(stringConstraints *StringConstraints, members map[string]json.RawMessage) error {
	raw, ok := members["pattern"]
	if !ok {
		return nil
	}

	value, err := parseString(raw, "pattern")
	if err != nil {
		return err
	}

	stringConstraints.Patterns = []string{value}
	restrictString(stringConstraints)

	return nil
}

// restrictString marks string constraints as restricted unless their kind is excluded.
func restrictString(stringConstraints *StringConstraints) {
	if stringConstraints.State != KindExcluded {
		stringConstraints.State = KindRestricted
	}
}

// compileStringFormat applies format to a reachable string kind.
func compileStringFormat(stringConstraints *StringConstraints, members map[string]json.RawMessage) error {
	raw, ok := members["format"]
	if !ok || stringConstraints.State == KindExcluded {
		return nil
	}

	value, err := parseString(raw, "format")
	if err != nil {
		return err
	}

	stringConstraints.Formats = []string{value}
	stringConstraints.State = KindRestricted

	return nil
}

// compileArray applies array Schema Object keywords to a Domain.
func (compiler *Compiler) compileArray(
	domain *Domain,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	active map[string]struct{},
) error {
	array := &domain.Array
	if err := compileArrayBounds(array, schema.Pointer, members, compiler); err != nil {
		return err
	}

	return compiler.compileArrayItems(array, schema, members, active)
}

// compileArrayBounds applies minItems and maxItems when present.
func compileArrayBounds(
	array *ArrayConstraints,
	pointer string,
	members map[string]json.RawMessage,
	compiler *Compiler,
) error {
	if err := compileMinItems(array, pointer, members, compiler); err != nil {
		return err
	}

	return compileMaxItems(array, pointer, members, compiler)
}

// compileMinItems applies minItems when present.
func compileMinItems(
	array *ArrayConstraints,
	pointer string,
	members map[string]json.RawMessage,
	compiler *Compiler,
) error {
	raw, ok := members["minItems"]
	if !ok {
		return nil
	}

	value, err := parseNonNegativeInt(raw, "minItems")
	if err != nil {
		return compiler.failure("compile", "malformed", pointer, "minItems", err)
	}

	array.MinItems = value
	restrictArray(array)

	return nil
}

// compileMaxItems applies maxItems when present.
func compileMaxItems(
	array *ArrayConstraints,
	pointer string,
	members map[string]json.RawMessage,
	compiler *Compiler,
) error {
	raw, ok := members["maxItems"]
	if !ok {
		return nil
	}

	value, err := parseNonNegativeInt(raw, "maxItems")
	if err != nil {
		return compiler.failure("compile", "malformed", pointer, "maxItems", err)
	}

	array.MaxItems = &value
	restrictArray(array)

	return nil
}

// restrictArray marks array constraints as restricted unless their kind is excluded.
func restrictArray(array *ArrayConstraints) {
	if array.State != KindExcluded {
		array.State = KindRestricted
	}
}

// compileArrayItems compiles the items schema when present.
func (compiler *Compiler) compileArrayItems(
	array *ArrayConstraints,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	active map[string]struct{},
) error {
	if _, ok := members["items"]; !ok {
		return nil
	}

	child, err := compiler.Source.Child(schema, "items")
	if err != nil {
		return compiler.failure("compile", "malformed", schema.Pointer, "items", err)
	}

	childID, err := compiler.compileSchema(child, active)
	if err != nil {
		return err
	}

	array.Items = childID
	restrictArray(array)

	return nil
}

// compileObject applies object Schema Object keywords to a Domain.
func (compiler *Compiler) compileObject(
	domain *Domain,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	active map[string]struct{},
) error {
	object := &domain.Object

	if err := compiler.compileObjectBounds(object, schema.Pointer, members); err != nil {
		return err
	}

	if err := compiler.compileObjectProperties(object, schema, members, active); err != nil {
		return err
	}

	return compiler.compileAdditionalProperties(object, schema, members, active)
}

// compileObjectBounds applies minProperties and maxProperties when present.
func (compiler *Compiler) compileObjectBounds(
	object *ObjectConstraints,
	pointer string,
	members map[string]json.RawMessage,
) error {
	if err := compiler.compileMinProperties(object, pointer, members); err != nil {
		return err
	}

	return compiler.compileMaxProperties(object, pointer, members)
}

// compileMinProperties applies minProperties when present.
func (compiler *Compiler) compileMinProperties(
	object *ObjectConstraints,
	pointer string,
	members map[string]json.RawMessage,
) error {
	raw, ok := members["minProperties"]
	if !ok {
		return nil
	}

	value, err := parseNonNegativeInt(raw, "minProperties")
	if err != nil {
		return compiler.failure("compile", "malformed", pointer, "minProperties", err)
	}

	object.MinProps = value
	restrictObject(object)

	return nil
}

// compileMaxProperties applies maxProperties when present.
func (compiler *Compiler) compileMaxProperties(
	object *ObjectConstraints,
	pointer string,
	members map[string]json.RawMessage,
) error {
	raw, ok := members["maxProperties"]
	if !ok {
		return nil
	}

	value, err := parseNonNegativeInt(raw, "maxProperties")
	if err != nil {
		return compiler.failure("compile", "malformed", pointer, "maxProperties", err)
	}

	object.MaxProps = &value
	restrictObject(object)

	return nil
}

// restrictObject marks object constraints as restricted unless their kind is excluded.
func restrictObject(object *ObjectConstraints) {
	if object.State != KindExcluded {
		object.State = KindRestricted
	}
}

// compileObjectProperties compiles properties and required names into object constraints.
func (compiler *Compiler) compileObjectProperties(
	object *ObjectConstraints,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	active map[string]struct{},
) error {
	properties, err := compiler.compileProperties(schema, members, active)
	if err != nil {
		return err
	}

	required, err := parseRequired(members["required"])
	if err != nil {
		return compiler.failure("compile", "malformed", schema.Pointer, "required", err)
	}

	applyRequiredProperties(properties, required)
	object.Properties = mapProperties(properties)

	if len(properties) > 0 || len(required) > 0 {
		restrictObject(object)
	}

	return nil
}

// applyRequiredProperties marks declared and implicit required properties.
func applyRequiredProperties(properties map[string]NamedProperty, required map[string]struct{}) {
	for name := range required {
		property, ok := properties[name]
		if !ok {
			property = NamedProperty{Name: name, State: PropertyAllowed, Values: AnyJSONDomainID}
		}

		property.Required = true
		properties[name] = property
	}
}

// mapProperties converts named properties to the Domain representation.
func mapProperties(properties map[string]NamedProperty) []NamedProperty {
	result := make([]NamedProperty, 0, len(properties))
	for _, property := range properties {
		result = append(result, property)
	}

	return result
}

// compileAdditionalProperties applies additionalProperties when present.
func (compiler *Compiler) compileAdditionalProperties(
	object *ObjectConstraints,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	active map[string]struct{},
) error {
	raw, ok := members["additionalProperties"]
	if !ok {
		return nil
	}

	values, err := compiler.additionalPropertyDomain(raw, schema, active)
	if err != nil {
		return err
	}

	object.Additional.Values = values
	restrictObject(object)

	return nil
}

// additionalPropertyDomain returns the Domain allowed for unnamed object properties.
func (compiler *Compiler) additionalPropertyDomain(
	raw json.RawMessage,
	schema oas.LocatedSchema,
	active map[string]struct{},
) (DomainID, error) {
	if isJSONNull(raw) {
		return NoDomain, compiler.failure(
			"compile",
			"malformed",
			schema.Pointer,
			"additionalProperties",
			errors.New("additionalProperties must be a boolean or Schema Object"),
		)
	}

	var allowed bool
	if err := json.Unmarshal(raw, &allowed); err == nil {
		if allowed {
			return AnyJSONDomainID, nil
		}

		return EmptyDomainID, nil
	}

	child, err := compiler.Source.Child(schema, "additionalProperties")
	if err != nil {
		return NoDomain, compiler.failure("compile", "malformed", schema.Pointer, "additionalProperties", err)
	}

	return compiler.compileSchema(child, active)
}

// compileProperties compiles each declared property Schema Object.
func (compiler *Compiler) compileProperties(
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	active map[string]struct{},
) (map[string]NamedProperty, error) {
	properties := make(map[string]NamedProperty)

	raw, ok := members["properties"]
	if !ok {
		return properties, nil
	}

	var rawProperties map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawProperties); err != nil || rawProperties == nil {
		if err == nil {
			err = errors.New("properties must be an object")
		}

		return nil, compiler.failure("compile", "malformed", schema.Pointer, "properties", err)
	}

	names := make([]string, 0, len(rawProperties))
	for name := range rawProperties {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		child, err := compiler.Source.Child(schema, "properties", name)
		if err != nil {
			return nil, compiler.failure("compile", "malformed", schema.Pointer, "properties", err)
		}

		childID, err := compiler.compileSchema(child, active)
		if err != nil {
			return nil, err
		}

		properties[name] = NamedProperty{Name: name, State: PropertyAllowed, Values: childID}
	}

	return properties, nil
}

// parseRequired validates and indexes required property names.
func parseRequired(raw json.RawMessage) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	if raw == nil {
		return result, nil
	}

	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		return nil, fmt.Errorf("required must be an array of strings: %w", err)
	}

	if len(names) == 0 {
		return nil, errors.New("required must contain at least one property name")
	}

	for _, name := range names {
		if _, duplicate := result[name]; duplicate {
			return nil, fmt.Errorf("required contains duplicate property %q", name)
		}

		result[name] = struct{}{}
	}

	return result, nil
}

// eliminateContradictoryKinds excludes kinds whose constraints cannot admit a value.
func eliminateContradictoryKinds(domain *Domain) error {
	if err := eliminateContradictoryNumbers(domain); err != nil {
		return err
	}

	eliminateContradictoryStrings(domain)
	eliminateContradictoryArrays(domain)
	eliminateContradictoryObjects(domain)

	if allKindsExcluded(*domain) {
		*domain = emptyDomain()
	}

	return nil
}

// eliminateContradictoryNumbers excludes exact ranges with no reachable lattice value.
func eliminateContradictoryNumbers(domain *Domain) error {
	if domain.Number.State == KindExcluded {
		return nil
	}

	productive, err := numberConstraintsAreProductive(domain.Number)
	if err != nil {
		return err
	}

	if !productive {
		domain.Number = NumberConstraints{State: KindExcluded}
	}

	return nil
}

// eliminateContradictoryStrings excludes strings whose minimum length exceeds their maximum length.
func eliminateContradictoryStrings(domain *Domain) {
	stringConstraints := &domain.String
	if stringConstraints.State != KindExcluded && stringConstraints.MaxLength != nil &&
		stringConstraints.MinLength > *stringConstraints.MaxLength {
		domain.String = StringConstraints{State: KindExcluded}
	}
}

// eliminateContradictoryArrays excludes arrays whose minimum length exceeds their maximum length.
func eliminateContradictoryArrays(domain *Domain) {
	array := &domain.Array
	if array.State != KindExcluded && array.MaxItems != nil && array.MinItems > *array.MaxItems {
		domain.Array = ArrayConstraints{State: KindExcluded}
	}
}

// eliminateContradictoryObjects excludes objects whose minimum property count exceeds its maximum.
func eliminateContradictoryObjects(domain *Domain) {
	object := &domain.Object
	if object.State != KindExcluded && object.MaxProps != nil && object.MinProps > *object.MaxProps {
		domain.Object = ObjectConstraints{State: KindExcluded}
	}
}

// errUnconstructible marks constraints that require trusted generation inputs.
var errUnconstructible = errors.New("unconstructible")

// compileEnum parses and filters enum values compatible with a Domain.
//
//nolint:cyclop // Enum parsing keeps malformed, unconstructible, duplicate, and excluded outcomes distinct.
func (compiler *Compiler) compileEnum(
	raw json.RawMessage,
	domain Domain,
	validExamples []jsonvalue.Value,
) ([]jsonvalue.Value, error) {
	var members []json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return nil, fmt.Errorf("enum must be an array: %w", err)
	}

	if len(members) == 0 {
		return nil, errors.New("enum must contain at least one value")
	}

	values := make([]jsonvalue.Value, 0, len(members))
	unconstructibleString := false

	for _, member := range members {
		value, err := jsonvalue.Parse(member)
		if err != nil {
			return nil, fmt.Errorf("parse enum member: %w", err)
		}

		duplicate := false

		for _, existing := range values {
			if existing.Equal(value) {
				duplicate = true

				break
			}
		}

		if duplicate {
			continue
		}

		if value.Kind == jsonvalue.KindString &&
			(len(domain.String.Patterns) > 0 || len(domain.String.Formats) > 0) {
			if jsonValuesContain(validExamples, value) {
				values = append(values, value)
			} else {
				unconstructibleString = true
			}

			continue
		}

		matches, err := compiler.valueFitsDomain(value, domain)
		if err != nil {
			return nil, err
		}

		if matches {
			values = append(values, value)
		}
	}

	if unconstructibleString {
		return nil, fmt.Errorf(
			"%w: enum string with pattern or format needs a trusted compatible valid example",
			errUnconstructible,
		)
	}

	return values, nil
}

// finiteDomain returns a Domain containing exactly the supplied enum values.
func finiteDomain(values []jsonvalue.Value) Domain {
	if len(values) == 0 {
		return emptyDomain()
	}

	domain := emptyDomain()
	domain.Status = DomainProductive

	domain.Enum = &EnumSet{Values: values}
	for _, value := range values {
		switch value.Kind {
		case jsonvalue.KindNull:
			domain.Null = KindUnrestricted
		case jsonvalue.KindBoolean:
			domain.Boolean = KindUnrestricted
		case jsonvalue.KindNumber:
			domain.Number.State = KindUnrestricted
		case jsonvalue.KindString:
			domain.String.State = KindUnrestricted
		case jsonvalue.KindArray:
			domain.Array = ArrayConstraints{State: KindUnrestricted, Items: AnyJSONDomainID}
		case jsonvalue.KindObject:
			domain.Object = ObjectConstraints{
				State: KindUnrestricted,
				Additional: AdditionalProperties{
					Values: AnyJSONDomainID,
				},
			}
		}
	}

	return domain
}

// valueFitsDomain reports whether a JSON value satisfies a Domain.
func (compiler *Compiler) valueFitsDomain(value jsonvalue.Value, domain Domain) (bool, error) {
	if !domainCanContainValue(domain, value) {
		return false, nil
	}

	switch value.Kind {
	case jsonvalue.KindNull:
		return domain.Null != KindExcluded, nil
	case jsonvalue.KindBoolean:
		return domain.Boolean != KindExcluded, nil
	case jsonvalue.KindNumber:
		return numberFits(value.Number, domain.Number)
	case jsonvalue.KindString:
		return stringFits(value.String, domain.String)
	case jsonvalue.KindArray:
		return compiler.arrayFits(value.Array, domain.Array)
	case jsonvalue.KindObject:
		return compiler.objectFits(value.Object, domain.Object)
	default:
		return false, errors.New("invalid enum value kind")
	}
}

// domainCanContainValue checks whole-Domain constraints shared by all JSON kinds.
func domainCanContainValue(domain Domain, value jsonvalue.Value) bool {
	return domain.Status != DomainEmpty && (domain.Enum == nil || enumContains(domain.Enum, value))
}

// enumContains reports semantic membership in a finite enum Domain.
func enumContains(enum *EnumSet, candidate jsonvalue.Value) bool {
	for _, value := range enum.Values {
		if value.Equal(candidate) {
			return true
		}
	}

	return false
}

// numberFits reports whether a JSON number satisfies number constraints.
func numberFits(value jsonvalue.Number, constraints NumberConstraints) (bool, error) {
	if constraints.State == KindExcluded || !fitsIntegerConstraint(value, constraints) {
		return false, nil
	}

	matches, err := fitsNumberBounds(value, constraints)
	if err != nil || !matches {
		return matches, err
	}

	return fitsMultipleOf(value, constraints.MultipleOf)
}

// fitsIntegerConstraint reports whether a value satisfies an integer-only constraint.
func fitsIntegerConstraint(value jsonvalue.Number, constraints NumberConstraints) bool {
	if !constraints.IntegersOnly {
		return true
	}

	if value.Rational != nil {
		return value.Rational.IsInt()
	}

	return !strings.Contains(value.Lexeme, "e-") && !strings.Contains(value.Lexeme, ".")
}

// fitsNumberBounds reports whether a value is inside its minimum and maximum bounds.
func fitsNumberBounds(value jsonvalue.Number, constraints NumberConstraints) (bool, error) {
	matches, err := fitsMinimum(value, constraints.Minimum)
	if err != nil || !matches {
		return matches, err
	}

	return fitsMaximum(value, constraints.Maximum)
}

// fitsMinimum reports whether a value satisfies a minimum bound.
func fitsMinimum(value jsonvalue.Number, bound *NumberBound) (bool, error) {
	if bound == nil {
		return true, nil
	}

	comparison, ok := compareExactNumbers(value, bound.Value)
	if !ok {
		return false, fmt.Errorf("%w: enum number is too large to compare", errUnconstructible)
	}

	return comparison > 0 || comparison == 0 && !bound.Exclusive, nil
}

// fitsMaximum reports whether a value satisfies a maximum bound.
func fitsMaximum(value jsonvalue.Number, bound *NumberBound) (bool, error) {
	if bound == nil {
		return true, nil
	}

	comparison, ok := compareExactNumbers(value, bound.Value)
	if !ok {
		return false, fmt.Errorf("%w: enum number is too large to compare", errUnconstructible)
	}

	return comparison < 0 || comparison == 0 && !bound.Exclusive, nil
}

// fitsMultipleOf reports whether a value is an exact multiple of its divisor.
func fitsMultipleOf(value jsonvalue.Number, multipleOf *jsonvalue.Number) (bool, error) {
	if multipleOf == nil {
		return true, nil
	}

	if value.Rational == nil || multipleOf.Rational == nil {
		return false, fmt.Errorf("%w: enum multipleOf is too large to compare", errUnconstructible)
	}

	quotient := new(big.Rat).Quo(value.Rational, multipleOf.Rational)

	return quotient.IsInt(), nil
}

// stringFits reports whether a JSON string satisfies string constraints.
func stringFits(value string, constraints StringConstraints) (bool, error) {
	if constraints.State == KindExcluded {
		return false, nil
	}

	length := utf8.RuneCountInString(value)
	if length < constraints.MinLength || constraints.MaxLength != nil && length > *constraints.MaxLength {
		return false, nil
	}

	if len(constraints.Patterns) > 0 || len(constraints.Formats) > 0 {
		return false, fmt.Errorf("%w: enum with pattern or format needs trusted compatible examples", errUnconstructible)
	}

	return true, nil
}

// arrayFits reports whether a JSON array satisfies array constraints.
func (compiler *Compiler) arrayFits(values []jsonvalue.Value, constraints ArrayConstraints) (bool, error) {
	if constraints.State == KindExcluded || len(values) < constraints.MinItems ||
		constraints.MaxItems != nil && len(values) > *constraints.MaxItems {
		return false, nil
	}

	for _, value := range values {
		child, ok := compiler.Domains.Domain(constraints.Items)
		if !ok {
			return false, errors.New("array item Domain does not exist")
		}

		matches, err := compiler.valueFitsDomain(value, child)
		if err != nil || !matches {
			return matches, err
		}
	}

	return true, nil
}

// objectFits reports whether a JSON object satisfies object constraints.
func (compiler *Compiler) objectFits(members []jsonvalue.Member, constraints ObjectConstraints) (bool, error) {
	if !fitsObjectSize(members, constraints) {
		return false, nil
	}

	byName := membersByName(members)
	properties := propertyConstraintsByName(constraints.Properties)

	if !hasRequiredProperties(byName, properties) {
		return false, nil
	}

	return compiler.objectMembersFit(byName, properties, constraints.Additional)
}

// fitsObjectSize reports whether an object has an allowed number of members.
func fitsObjectSize(members []jsonvalue.Member, constraints ObjectConstraints) bool {
	return constraints.State != KindExcluded && len(members) >= constraints.MinProps &&
		(constraints.MaxProps == nil || len(members) <= *constraints.MaxProps)
}

// membersByName indexes object members by name.
func membersByName(members []jsonvalue.Member) map[string]jsonvalue.Value {
	result := make(map[string]jsonvalue.Value, len(members))
	for _, member := range members {
		result[member.Name] = member.Value
	}

	return result
}

// propertyConstraintsByName indexes named property constraints by name.
func propertyConstraintsByName(properties []NamedProperty) map[string]NamedProperty {
	result := make(map[string]NamedProperty, len(properties))
	for _, property := range properties {
		result[property.Name] = property
	}

	return result
}

// hasRequiredProperties reports whether all required named properties are present.
func hasRequiredProperties(members map[string]jsonvalue.Value, properties map[string]NamedProperty) bool {
	for name, property := range properties {
		if property.Required {
			if _, ok := members[name]; !ok {
				return false
			}
		}
	}

	return true
}

// objectMembersFit reports whether each object member satisfies its selected child Domain.
func (compiler *Compiler) objectMembersFit(
	members map[string]jsonvalue.Value,
	properties map[string]NamedProperty,
	additional AdditionalProperties,
) (bool, error) {
	for name, value := range members {
		childID, allowed := propertyDomain(name, properties, additional)
		if !allowed {
			return false, nil
		}

		child, ok := compiler.Domains.Domain(childID)
		if !ok {
			return false, errors.New("object property Domain does not exist")
		}

		matches, err := compiler.valueFitsDomain(value, child)
		if err != nil || !matches {
			return matches, err
		}
	}

	return true, nil
}

// propertyDomain returns the child Domain and permission for one object member name.
func propertyDomain(
	name string,
	properties map[string]NamedProperty,
	additional AdditionalProperties,
) (DomainID, bool) {
	property, named := properties[name]
	if !named {
		return additional.Values, true
	}

	if property.State == PropertyForbidden {
		return NoDomain, false
	}

	return property.Values, true
}

// compileGenerationExamples parses trusted valid and invalid generation examples.
func compileGenerationExamples(members map[string]json.RawMessage) (GenerationExamples, error) {
	var examples GenerationExamples
	for keyword, target := range map[string]*[]jsonvalue.Value{
		"x-valid-examples":   &examples.Valid,
		"x-invalid-examples": &examples.Invalid,
	} {
		raw, ok := members[keyword]
		if !ok {
			continue
		}

		if isJSONNull(raw) {
			return GenerationExamples{}, fmt.Errorf("%s must be an array", keyword)
		}

		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return GenerationExamples{}, fmt.Errorf("%s must be an array: %w", keyword, err)
		}

		for _, valueRaw := range values {
			value, err := jsonvalue.Parse(valueRaw)
			if err != nil {
				return GenerationExamples{}, fmt.Errorf("parse %s: %w", keyword, err)
			}

			*target = append(*target, value)
		}
	}

	return examples, nil
}

// constraintSources records the supported constraint keywords in source order.
func constraintSources(pointer string, members map[string]json.RawMessage) []ConstraintSource {
	keywords := make([]string, 0, len(members))
	for keyword := range members {
		switch keyword {
		case "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf",
			"minLength", "maxLength", "pattern", "format", "minItems", "maxItems", "items",
			"minProperties", "maxProperties", "required", "properties", "additionalProperties", "enum", "nullable", "allOf":
			keywords = append(keywords, keyword)
		}
	}

	sort.Strings(keywords)

	result := make([]ConstraintSource, 0, len(keywords))
	for _, keyword := range keywords {
		result = append(result, ConstraintSource{Pointer: pointer, Keyword: keyword})
	}

	return result
}

// metadataForPointer returns source metadata already recorded for a schema pointer.
func (compiler *Compiler) metadataForPointer(pointer string) ([]ConstraintSource, GenerationExamples) {
	for _, use := range compiler.SchemaUses {
		if use.Pointer == pointer {
			return use.Constraints, use.Examples
		}
	}

	return nil, GenerationExamples{}
}

// addSchemaUse records source metadata once for a schema pointer.
func (compiler *Compiler) addSchemaUse(
	pointer string,
	domain DomainID,
	constraints []ConstraintSource,
	examples GenerationExamples,
) {
	for _, use := range compiler.SchemaUses {
		if use.Pointer == pointer {
			return
		}
	}

	compiler.SchemaUses = append(compiler.SchemaUses, SchemaUse{
		Pointer:     pointer,
		Domain:      domain,
		Constraints: append([]ConstraintSource(nil), constraints...),
		Examples: GenerationExamples{
			Valid:   cloneJSONValues(examples.Valid),
			Invalid: cloneJSONValues(examples.Invalid),
		},
	})
}

// cloneJSONValues returns deep copies of exact JSON values.
func cloneJSONValues(values []jsonvalue.Value) []jsonvalue.Value {
	result := make([]jsonvalue.Value, len(values))
	for index, value := range values {
		result[index] = cloneJSONValue(value)
	}

	return result
}

// failure creates a contextual compilation Error.
func (compiler *Compiler) failure(phase string, code string, pointer string, keyword string, cause error) *Error {
	return &Error{Phase: phase, Code: code, Pointer: pointer, Keyword: keyword, Cause: cause}
}

// parseExactNumber parses a JSON number without losing precision.
func parseExactNumber(raw json.RawMessage) (jsonvalue.Number, error) {
	return jsonvalue.ParseNumber(string(bytes.TrimSpace(raw)))
}

// parseNonNegativeInt parses a non-negative integer keyword value.
func parseNonNegativeInt(raw json.RawMessage, keyword string) (int, error) {
	if isJSONNull(raw) {
		return 0, fmt.Errorf("%s must be an integer", keyword)
	}

	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", keyword, err)
	}

	if value < 0 {
		return 0, fmt.Errorf("%s must not be negative", keyword)
	}

	return value, nil
}

// parseString parses a string keyword value.
func parseString(raw json.RawMessage, keyword string) (string, error) {
	if isJSONNull(raw) {
		return "", fmt.Errorf("%s must be a string", keyword)
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string: %w", keyword, err)
	}

	return value, nil
}

// isJSONNull reports whether a raw keyword value is JSON null.
func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// compareNumberToZero compares an exact JSON number with zero.
func compareNumberToZero(number jsonvalue.Number) int {
	if number.Rational != nil {
		return number.Rational.Sign()
	}

	if strings.HasPrefix(number.Lexeme, "-") {
		return -1
	}

	if number.Lexeme == "0" {
		return 0
	}

	return 1
}

// compareExactNumbers compares arbitrary-size canonical JSON decimals without materializing their exponents.
func compareExactNumbers(left jsonvalue.Number, right jsonvalue.Number) (int, bool) {
	if left.Rational != nil && right.Rational != nil {
		return left.Rational.Cmp(right.Rational), true
	}

	leftNegative, leftDigits, leftExponent, leftOK := exactDecimalParts(left.Lexeme)

	rightNegative, rightDigits, rightExponent, rightOK := exactDecimalParts(right.Lexeme)
	if !leftOK || !rightOK {
		return 0, false
	}

	if leftDigits == "0" || rightDigits == "0" {
		return compareExactZero(leftNegative, leftDigits, rightNegative, rightDigits), true
	}

	if leftNegative != rightNegative {
		if leftNegative {
			return -1, true
		}

		return 1, true
	}

	comparison := compareDecimalMagnitudes(leftDigits, leftExponent, rightDigits, rightExponent)
	if leftNegative {
		comparison = -comparison
	}

	return comparison, true
}

// exactDecimalParts splits a canonical JSON decimal into sign, digits, and a base-ten exponent.
func exactDecimalParts(lexeme string) (bool, string, *big.Int, bool) {
	negative := strings.HasPrefix(lexeme, "-")
	if negative {
		lexeme = lexeme[1:]
	}

	exponent := new(big.Int)
	if exponentIndex := strings.IndexByte(lexeme, 'e'); exponentIndex >= 0 {
		parsed, ok := new(big.Int).SetString(lexeme[exponentIndex+1:], exactDecimalRadix)
		if !ok {
			return false, "", nil, false
		}

		exponent = parsed
		lexeme = lexeme[:exponentIndex]
	}

	if decimalIndex := strings.IndexByte(lexeme, '.'); decimalIndex >= 0 {
		fractionLength := len(lexeme) - decimalIndex - 1
		lexeme = lexeme[:decimalIndex] + lexeme[decimalIndex+1:]

		exponent.Sub(exponent, big.NewInt(int64(fractionLength)))
	}

	digits := strings.TrimLeft(lexeme, "0")
	if digits == "" {
		return false, "0", new(big.Int), true
	}

	return negative, digits, exponent, true
}

// compareExactZero handles sign while canonical zero remains unsigned.
func compareExactZero(leftNegative bool, leftDigits string, rightNegative bool, rightDigits string) int {
	if leftDigits == "0" && rightDigits == "0" {
		return 0
	}

	if leftDigits == "0" {
		if rightNegative {
			return 1
		}

		return -1
	}

	if leftNegative {
		return -1
	}

	return 1
}

// compareDecimalMagnitudes compares positive digit/exponent pairs exactly.
func compareDecimalMagnitudes(
	leftDigits string,
	leftExponent *big.Int,
	rightDigits string,
	rightExponent *big.Int,
) int {
	leftMagnitude := new(big.Int).Add(leftExponent, big.NewInt(int64(len(leftDigits))))

	rightMagnitude := new(big.Int).Add(rightExponent, big.NewInt(int64(len(rightDigits))))
	if comparison := leftMagnitude.Cmp(rightMagnitude); comparison != 0 {
		return comparison
	}

	width := max(len(leftDigits), len(rightDigits))
	for index := range width {
		leftDigit := byte('0')
		if index < len(leftDigits) {
			leftDigit = leftDigits[index]
		}

		rightDigit := byte('0')
		if index < len(rightDigits) {
			rightDigit = rightDigits[index]
		}

		if leftDigit < rightDigit {
			return -1
		}

		if leftDigit > rightDigit {
			return 1
		}
	}

	return 0
}
