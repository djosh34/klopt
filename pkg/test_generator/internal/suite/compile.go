package suite

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
)

// Compiler compiles located Schema Objects into canonical DomainIDs.
type Compiler struct {
	Source                 oas.Source
	Domains                *DomainRegistry
	usesByPointer          map[string]*schemaUse
	rootUse                *schemaUse
	mustHaveAllXValidCases bool
	patternOption          patternvalidator.Option
	nextPatternID          uint64
}

// NewCompiler creates a Compiler for one located OpenAPI source.
func NewCompiler(source oas.Source, options ...patternvalidator.Option) *Compiler {
	if len(options) > 1 {
		panic("suite: NewCompiler accepts at most one pattern option")
	}

	patternOption := patternvalidator.Option(func(*patternvalidator.PatternValidation) {})
	if len(options) == 1 {
		patternOption = options[0]
	}

	return &Compiler{
		Source:        source,
		Domains:       NewDomainRegistry(),
		usesByPointer: make(map[string]*schemaUse),
		patternOption: patternOption,
		nextPatternID: 1,
	}
}

// Compile compiles the selected request Schema Object.
func (compiler *Compiler) Compile() (DomainID, error) {
	use, err := compiler.compileSchema(compiler.Source.RequestSchema, make(map[string]struct{}))
	if err != nil {
		return NoDomain, err
	}

	compiler.rootUse = use

	return use.domain, nil
}

// CompileSchema compiles one inline Schema Object or Reference Object.
func (compiler *Compiler) CompileSchema(schema oas.LocatedSchema) (DomainID, error) {
	use, err := compiler.compileSchema(schema, make(map[string]struct{}))
	if err != nil {
		return NoDomain, err
	}

	return use.domain, nil
}

// compileSchema resolves and compiles one schema occurrence.
func (compiler *Compiler) compileSchema(
	occurrence oas.LocatedSchema,
	active map[string]struct{},
) (*schemaUse, error) {
	if use, ok := compiler.usesByPointer[occurrence.Pointer]; ok {
		return use, nil
	}

	resolved, err := compiler.Source.Resolve(occurrence)
	if err != nil {
		return nil, compiler.failure("compile", "unsupported", occurrence.Pointer, "$ref", err)
	}

	if use, ok := compiler.usesByPointer[resolved.Pointer]; ok {
		return compiler.referenceUse(occurrence.Pointer, use), nil
	}

	if _, recursive := active[resolved.Pointer]; recursive {
		return nil, compiler.recursiveReferenceFailure(resolved.Pointer)
	}

	use, err := compiler.compileResolvedSchema(resolved, active)
	if err != nil {
		return nil, err
	}

	if occurrence.Pointer == resolved.Pointer {
		return use, nil
	}

	return compiler.referenceUse(occurrence.Pointer, use), nil
}

// referenceUse records one Reference Object occurrence without copying its resolved occurrence graph.
func (compiler *Compiler) referenceUse(pointer string, resolved *schemaUse) *schemaUse {
	use := &schemaUse{
		pointer:     pointer,
		domain:      resolved.domain,
		localDomain: resolved.localDomain,
		arrayShape:  resolved.arrayShape,
		objectShape: resolved.objectShape,
		constraints: append([]ConstraintSource(nil), resolved.constraints...),
		patterns:    append([]patternOccurrence(nil), resolved.patterns...),
		examples: GenerationExamples{
			Valid:         cloneGenerationExamples(resolved.examples.Valid),
			Invalid:       cloneGenerationExamples(resolved.examples.Invalid),
			ValidDeclared: resolved.examples.ValidDeclared,
		},
		atomic:     resolved.atomic,
		items:      resolved.items,
		properties: append([]schemaPropertyUse(nil), resolved.properties...),
		additional: resolved.additional,
		resolved:   resolved,
	}
	compiler.usesByPointer[pointer] = use

	return use
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

// compileResolvedSchema compiles and records the resolved schema occurrence.
// referenceUse records any Reference Object occurrence pointer.
func (compiler *Compiler) compileResolvedSchema(
	resolved oas.LocatedSchema,
	active map[string]struct{},
) (*schemaUse, error) {
	active[resolved.Pointer] = struct{}{}
	defer delete(active, resolved.Pointer)

	members, err := schemaMembers(resolved)
	if err != nil {
		return nil, compiler.failure("compile", "malformed", resolved.Pointer, "", err)
	}

	if validationErr := validateSchemaKeywords(members); validationErr != nil {
		return nil, compiler.failure("compile", "malformed", resolved.Pointer, "", validationErr)
	}

	if discriminatorErr := compiler.validateDiscriminator(resolved.Pointer, members); discriminatorErr != nil {
		return nil, discriminatorErr
	}

	if keyword := unsupportedKeyword(members); keyword != "" {
		return nil, compiler.unsupportedKeywordFailure(resolved.Pointer, keyword)
	}

	use := &schemaUse{pointer: resolved.Pointer, atomic: make(map[string]DomainID)}

	domain, constraints, examples, err := compiler.compileSchemaDomain(use, resolved, members, active)
	if err != nil {
		return nil, err
	}

	use.domain = compiler.Domains.FindOrAddEquivalentDomain(domain)
	use.localDomain = use.domain
	use.constraints = constraints

	use.patterns, err = compiler.localPatternOccurrences(resolved.Pointer, members)
	if err != nil {
		return nil, compiler.failure("compile", "malformed", resolved.Pointer, "pattern", err)
	}

	use.examples = examples

	use, err = compiler.compileAllOf(resolved, members, active, use)
	if err != nil {
		return nil, err
	}

	compiler.usesByPointer[resolved.Pointer] = use

	return use, nil
}

// validateDiscriminator independently shape-checks an inert hint. Klopt intentionally accepts
// it without requiring oneOf, anyOf, or allOf and does not store its values.
//
//nolint:cyclop // Discriminator Object fixed fields and mapping values have distinct diagnostics.
func (compiler *Compiler) validateDiscriminator(pointer string, members map[string]json.RawMessage) error {
	raw, ok := members["discriminator"]
	if !ok {
		return nil
	}

	discriminatorPointer := pointer + "/discriminator"

	var discriminator map[string]json.RawMessage
	if err := json.Unmarshal(raw, &discriminator); err != nil || discriminator == nil {
		return compiler.failure("compile", "malformed", discriminatorPointer, "", errors.New("must be an object"))
	}

	propertyName, ok := discriminator["propertyName"]
	if !ok {
		return compiler.failure(
			"compile", "malformed", discriminatorPointer+"/propertyName", "", errors.New("is required"),
		)
	}

	if _, err := parseString(propertyName, "propertyName"); err != nil {
		return compiler.failure("compile", "malformed", discriminatorPointer+"/propertyName", "", err)
	}

	mappingRaw, ok := discriminator["mapping"]
	if !ok {
		return nil
	}

	var mapping map[string]json.RawMessage
	if err := json.Unmarshal(mappingRaw, &mapping); err != nil || mapping == nil {
		return compiler.failure(
			"compile", "malformed", discriminatorPointer+"/mapping", "", errors.New("must be an object"),
		)
	}

	names := make([]string, 0, len(mapping))
	for name := range mapping {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		if _, err := parseString(mapping[name], "mapping value"); err != nil {
			escaped := strings.ReplaceAll(name, "~", "~0")
			escaped = strings.ReplaceAll(escaped, "/", "~1")

			return compiler.failure(
				"compile", "malformed", discriminatorPointer+"/mapping/"+escaped, "", err,
			)
		}
	}

	return nil
}

// localPatternOccurrences records a direct declaration before allOf composition.
func (compiler *Compiler) localPatternOccurrences(
	pointer string,
	members map[string]json.RawMessage,
) ([]patternOccurrence, error) {
	raw, ok := members["pattern"]
	if !ok {
		return nil, nil
	}

	pattern, err := parseString(raw, "pattern")
	if err != nil {
		return nil, err
	}

	occurrence := patternOccurrence{
		id: compiler.nextPatternID,
		source: ConstraintSource{
			Pointer: pointer,
			Keyword: "pattern",
		},
		value: pattern,
	}
	compiler.nextPatternID++

	return []patternOccurrence{occurrence}, nil
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
	use *schemaUse,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	active map[string]struct{},
) (Domain, []ConstraintSource, GenerationExamples, error) {
	examples, err := compiler.compileLocalGenerationExamples(schema.Pointer, members)
	if err != nil {
		return Domain{}, nil, GenerationExamples{}, err
	}

	domain := anyJSONDomain()
	constraints := make([]ConstraintSource, 0, len(members))

	if validationErr := compiler.validateFormat(schema.Pointer, members); validationErr != nil {
		return Domain{}, nil, GenerationExamples{}, validationErr
	}

	hasType, err := applyTypeAndNullable(&domain, members)
	if err != nil {
		return Domain{}, nil, GenerationExamples{}, compiler.failure("compile", "malformed", schema.Pointer, "type", err)
	}

	if hasType {
		constraints = append(constraints, ConstraintSource{Pointer: schema.Pointer, Keyword: "type"})
	}

	if err := compiler.compileScalarConstraints(&domain, schema.Pointer, members); err != nil {
		return Domain{}, nil, GenerationExamples{}, err
	}

	if err := compiler.compileArray(&domain, use, schema, members, active); err != nil {
		return Domain{}, nil, GenerationExamples{}, err
	}

	if err := compiler.compileObject(&domain, use, schema, members, active); err != nil {
		return Domain{}, nil, GenerationExamples{}, err
	}

	constraints = append(constraints, constraintSources(schema.Pointer, members)...)
	if err := eliminateContradictoryKinds(&domain); err != nil {
		return Domain{}, nil, GenerationExamples{}, compiler.failure("compile", "unconstructible", schema.Pointer, "", err)
	}

	if err := compiler.applyLocalOracles(&domain, use, schema.Pointer, members, &examples); err != nil {
		return Domain{}, nil, GenerationExamples{}, err
	}

	return domain, constraints, examples, nil
}

// compileLocalGenerationExamples parses extension values and locates malformed input.
func (compiler *Compiler) compileLocalGenerationExamples(
	pointer string,
	members map[string]json.RawMessage,
) (GenerationExamples, error) {
	examples, err := compileGenerationExamples(pointer, members)
	if err == nil {
		return examples, nil
	}

	var exampleError *generationExampleError
	if errors.As(err, &exampleError) {
		return GenerationExamples{}, compiler.failure(
			"compile", "malformed", pointer, exampleError.Keyword, exampleError.Cause,
		)
	}

	return GenerationExamples{}, compiler.failure("compile", "malformed", pointer, "", err)
}

// applyLocalOracles compiles enum cases and validates exact local oracle evidence.
func (compiler *Compiler) applyLocalOracles(
	domain *Domain,
	use *schemaUse,
	pointer string,
	members map[string]json.RawMessage,
	examples *GenerationExamples,
) error {
	if err := compiler.applyEnum(domain, use, pointer, members, examples); err != nil {
		return err
	}

	if overlap := generationExampleOverlap(*examples); overlap != nil {
		return compiler.failure(
			"compile", "malformed", overlap.Source.Pointer, overlap.Source.Keyword,
			errors.New("trusted value is declared both valid and invalid"),
		)
	}

	if !hasUnconstructibleStringDomain(*domain) || examples.ValidDeclared &&
		(len(examples.Valid) > 0 || hasReachableNonStringKind(*domain)) {
		return nil
	}

	keyword := "pattern"
	if _, ok := members[keyword]; !ok {
		keyword = "format"
	}

	return compiler.failure(
		"compile", "unconstructible", pointer, keyword,
		errors.New("pattern or format has no trusted valid example declared locally"),
	)
}

// compileScalarConstraints applies number and string keyword families.
func (compiler *Compiler) compileScalarConstraints(
	domain *Domain,
	pointer string,
	members map[string]json.RawMessage,
) error {
	if err := compiler.compileNumber(domain, members); err != nil {
		return compiler.failure("compile", "malformed", pointer, "number", err)
	}

	if err := compileString(domain, members); err != nil {
		return compiler.failure("compile", "malformed", pointer, "string", err)
	}

	return nil
}

// validateFormat validates format even when it is inapplicable to every reachable kind.
func (compiler *Compiler) validateFormat(pointer string, members map[string]json.RawMessage) error {
	raw, ok := members["format"]
	if !ok {
		return nil
	}

	if _, err := parseString(raw, "format"); err != nil {
		return compiler.failure("compile", "malformed", pointer, "format", err)
	}

	return nil
}

// applyEnum replaces a Domain with the local enum while retaining constructive patterns.
//
//nolint:cyclop // Enum decoding and occurrence-oracle bookkeeping stay in source order.
func (compiler *Compiler) applyEnum(
	domain *Domain,
	use *schemaUse,
	pointer string,
	members map[string]json.RawMessage,
	examples *GenerationExamples,
) error {
	raw, ok := members["enum"]
	if !ok {
		return nil
	}

	enumMembers, err := decodeEnumMembers(raw)
	if err != nil {
		return compiler.failure("compile", "malformed", pointer, "enum", err)
	}

	atomicValues := make([]jsonvalue.Value, 0, len(enumMembers))
	for _, member := range enumMembers {
		value, parseErr := jsonvalue.Parse(member)
		if parseErr != nil {
			return compiler.failure("compile", "malformed", pointer, "enum", parseErr)
		}

		if !jsonValuesContain(atomicValues, value) {
			atomicValues = append(atomicValues, value)
		}
	}

	preEnum := compiler.Domains.FindOrAddEquivalentDomain(*domain)

	for _, source := range constraintSources(pointer, members) {
		if source.Keyword != "enum" {
			if _, preserved := use.atomic[source.Keyword]; !preserved {
				use.atomic[source.Keyword] = preEnum
			}
		}
	}

	use.atomic["enum"] = compiler.Domains.FindOrAddEquivalentDomain(finiteDomain(atomicValues))
	stringConstraints := domain.String

	*domain = finiteDomain(atomicValues)
	if domain.String.State != KindExcluded && len(stringConstraints.Patterns) > 0 {
		domain.String = stringConstraints
	}

	for _, value := range atomicValues {
		appendGenerationExample(&examples.Valid, GenerationExample{
			Value:  value,
			Source: ConstraintSource{Pointer: pointer, Keyword: "enum"},
		})
	}

	examples.ValidDeclared = true

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
	if err := validateReadWriteOnlyShape(members); err != nil {
		return err
	}

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

// validateReadWriteOnlyShape validates the request/response annotation shapes.
func validateReadWriteOnlyShape(members map[string]json.RawMessage) error {
	_, _, err := parseOptionalBool(members, "readOnly")
	if err != nil {
		return err
	}

	_, _, err = parseOptionalBool(members, "writeOnly")

	return err
}

// unsupportedKeyword returns the first recognized keyword unsupported by this step.
func unsupportedKeyword(members map[string]json.RawMessage) string {
	for _, keyword := range []string{"oneOf", "anyOf", "not"} {
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

	return nil
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
	use *schemaUse,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	active map[string]struct{},
) error {
	if rawType, ok := members["type"]; ok {
		var schemaType string
		if err := json.Unmarshal(rawType, &schemaType); err == nil && schemaType == "array" {
			if _, hasItems := members["items"]; !hasItems {
				return compiler.failure(
					"compile", "malformed", schema.Pointer, "items", errors.New("items must be present when type is array"),
				)
			}
		}
	}

	array := &domain.Array
	if err := compileArrayBounds(array, schema.Pointer, members, compiler); err != nil {
		return err
	}

	bounds := cloneDomain(*domain).Array

	if err := compiler.compileArrayItems(array, use, schema, members, active); err != nil {
		return err
	}

	compiler.recordArrayBoundDomains(domain, use, members, bounds)

	return nil
}

// recordArrayBoundDomains preserves count rules before contradictory items
// normalize the effective array kind away.
func (compiler *Compiler) recordArrayBoundDomains(
	domain *Domain,
	use *schemaUse,
	members map[string]json.RawMessage,
	bounds ArrayConstraints,
) {
	if domain.Array.State != KindExcluded && hasArrayPlanningRule(members) {
		planning := cloneDomain(*domain).Array
		if use.items != nil && use.items.domain == EmptyDomainID {
			planning.Items = AnyJSONDomainID
		}

		use.arrayShape = compiler.Domains.FindOrAddEquivalentDomain(arrayRuleDomain(planning))
	}

	for _, keyword := range []string{"minItems", "maxItems"} {
		if _, declared := members[keyword]; !declared {
			continue
		}

		atomic := cloneDomain(*domain)
		atomic.Array.MinItems = bounds.MinItems

		atomic.Array.MaxItems = bounds.MaxItems
		if atomic.Array.Items == EmptyDomainID {
			atomic.Array.Items = AnyJSONDomainID
		}

		if atomic.Array.State == KindExcluded {
			atomic.Array.State = bounds.State
			atomic.Array.Items = AnyJSONDomainID
		}

		use.atomic[keyword] = compiler.Domains.FindOrAddEquivalentDomain(atomic)
	}
}

// hasArrayPlanningRule reports whether an occurrence declares an array keyword.
func hasArrayPlanningRule(members map[string]json.RawMessage) bool {
	for _, keyword := range []string{"minItems", "maxItems", "items"} {
		if _, declared := members[keyword]; declared {
			return true
		}
	}

	return false
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
	use *schemaUse,
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

	childUse, err := compiler.compileSchema(child, active)
	if err != nil {
		return err
	}

	use.items = childUse
	array.Items = childUse.domain
	restrictArray(array)

	return nil
}

// compileObject applies object Schema Object keywords to a Domain.
func (compiler *Compiler) compileObject(
	domain *Domain,
	use *schemaUse,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	active map[string]struct{},
) error {
	object := &domain.Object

	if err := compiler.compileObjectBounds(object, schema.Pointer, members); err != nil {
		return err
	}

	if err := compiler.compileAdditionalProperties(object, use, schema, members, active); err != nil {
		return err
	}

	if err := compiler.compileObjectProperties(object, use, schema, members, active); err != nil {
		return err
	}

	compiler.recordObjectPlanningDomains(domain, use, members)

	return nil
}

// recordObjectPlanningDomains preserves occurrence-local object rules before
// contradictory children and positive cardinality normalize the object kind away.
func (compiler *Compiler) recordObjectPlanningDomains(
	domain *Domain,
	use *schemaUse,
	members map[string]json.RawMessage,
) {
	if domain.Object.State == KindExcluded || !hasObjectPlanningRule(members) {
		return
	}

	source := cloneDomain(*domain).Object
	planning := relaxedObjectPlanningShape(source, use)
	use.objectShape = compiler.Domains.FindOrAddEquivalentDomain(objectRuleDomain(planning))
	compiler.recordObjectAtomicPlanningDomains(*domain, source, planning, use, members)
}

// hasObjectPlanningRule reports whether an occurrence declares an object keyword.
func hasObjectPlanningRule(members map[string]json.RawMessage) bool {
	for _, keyword := range []string{
		"minProperties", "maxProperties", "required", "properties", "additionalProperties",
	} {
		if _, declared := members[keyword]; declared {
			return true
		}
	}

	return false
}

// relaxedObjectPlanningShape opens only contradictory child seams.
func relaxedObjectPlanningShape(source ObjectConstraints, use *schemaUse) ObjectConstraints {
	planning := source
	for index := range planning.Properties {
		child := use.property(planning.Properties[index].Name)
		if child != nil && child.domain == EmptyDomainID {
			planning.Properties[index].State = PropertyAllowed
			planning.Properties[index].Values = AnyJSONDomainID
		}
	}

	if use.additional != nil && use.additional.domain == EmptyDomainID {
		planning.Additional.Values = AnyJSONDomainID
	}

	return planning
}

// recordObjectAtomicPlanningDomains preserves each declared object rule independently.
func (compiler *Compiler) recordObjectAtomicPlanningDomains(
	domain Domain,
	source ObjectConstraints,
	planning ObjectConstraints,
	use *schemaUse,
	members map[string]json.RawMessage,
) {
	for _, keyword := range []string{"minProperties", "maxProperties", "required", "additionalProperties"} {
		if _, declared := members[keyword]; !declared {
			continue
		}

		atomic := objectAtomicPlanningDomain(domain, source, planning, keyword)
		use.atomic[keyword] = compiler.Domains.FindOrAddEquivalentDomain(atomic)
	}
}

// objectAtomicPlanningDomain isolates one object rule from contradictory siblings.
func objectAtomicPlanningDomain(
	domain Domain,
	source ObjectConstraints,
	planning ObjectConstraints,
	keyword string,
) Domain {
	atomic := cloneDomain(domain)
	atomic.Object = planning

	switch keyword {
	case "minProperties":
		atomic.Object.MaxProps = nil
		atomic.Object.Additional.Values = AnyJSONDomainID
	case "maxProperties":
		atomic.Object.MinProps = 0
		atomic.Object.Additional.Values = AnyJSONDomainID
	case "required":
		atomic.Object.MinProps = 0
		atomic.Object.MaxProps = nil
		atomic.Object.Additional.Values = AnyJSONDomainID
	case "additionalProperties":
		atomic.Object.MinProps = 0
		atomic.Object.MaxProps = nil
		atomic.Object.Additional = source.Additional
	}

	return atomic
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
	use *schemaUse,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	active map[string]struct{},
) error {
	properties, err := compiler.compileProperties(use, schema, members, active)
	if err != nil {
		return err
	}

	readOnly, err := compiler.readOnlyProperties(schema, members)
	if err != nil {
		return err
	}

	required, err := parseRequired(members["required"])
	if err != nil {
		return compiler.failure("compile", "malformed", schema.Pointer, "required", err)
	}

	for name := range readOnly {
		delete(required, name)
	}

	applyRequiredProperties(properties, required, object.Additional)
	object.Properties = mapProperties(properties)

	if len(properties) > 0 || len(required) > 0 {
		restrictObject(object)
	}

	return nil
}

// readOnlyProperties returns declared properties whose resolved Schema Object is read-only.
func (compiler *Compiler) readOnlyProperties(
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
) (map[string]struct{}, error) {
	result := make(map[string]struct{})

	var properties map[string]json.RawMessage
	if raw, ok := members["properties"]; !ok {
		return result, nil
	} else if err := json.Unmarshal(raw, &properties); err != nil {
		return nil, compiler.failure("compile", "malformed", schema.Pointer, "properties", err)
	}

	for name := range properties {
		child, err := compiler.Source.Child(schema, "properties", name)
		if err != nil {
			return nil, compiler.failure("compile", "malformed", schema.Pointer, "properties", err)
		}

		resolved, err := compiler.Source.Resolve(child)
		if err != nil {
			return nil, compiler.failure("compile", "unsupported", child.Pointer, "$ref", err)
		}

		readOnly, err := compiler.propertyReadOnly(resolved)
		if err != nil {
			return nil, err
		}

		if readOnly {
			result[name] = struct{}{}
		}
	}

	return result, nil
}

// propertyReadOnly applies request direction rules to one resolved property Schema Object.
func (compiler *Compiler) propertyReadOnly(resolved oas.LocatedSchema) (bool, error) {
	members, err := schemaMembers(resolved)
	if err != nil {
		return false, compiler.failure("compile", "malformed", resolved.Pointer, "", err)
	}

	readOnly, _, err := parseOptionalBool(members, "readOnly")
	if err != nil {
		return false, compiler.failure("compile", "malformed", resolved.Pointer, "readOnly", err)
	}

	writeOnly, _, err := parseOptionalBool(members, "writeOnly")
	if err != nil {
		return false, compiler.failure("compile", "malformed", resolved.Pointer, "writeOnly", err)
	}

	if readOnly && writeOnly {
		return false, compiler.failure(
			"compile", "malformed", resolved.Pointer, "readOnly",
			errors.New("readOnly and writeOnly must not both be true"),
		)
	}

	return readOnly, nil
}

// applyRequiredProperties marks declared and implicit required properties.
func applyRequiredProperties(
	properties map[string]NamedProperty,
	required map[string]struct{},
	additional AdditionalProperties,
) {
	for name := range required {
		property, ok := properties[name]
		if !ok {
			property = NamedProperty{Name: name, State: PropertyAllowed, Values: additional.Values}
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
	use *schemaUse,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	active map[string]struct{},
) error {
	raw, ok := members["additionalProperties"]
	if !ok {
		return nil
	}

	values, childUse, err := compiler.additionalPropertyDomain(raw, schema, active)
	if err != nil {
		return err
	}

	use.additional = childUse
	object.Additional.Values = values
	restrictObject(object)

	return nil
}

// additionalPropertyDomain returns the Domain allowed for unnamed object properties.
func (compiler *Compiler) additionalPropertyDomain(
	raw json.RawMessage,
	schema oas.LocatedSchema,
	active map[string]struct{},
) (DomainID, *schemaUse, error) {
	if isJSONNull(raw) {
		return NoDomain, nil, compiler.failure(
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
			return AnyJSONDomainID, nil, nil
		}

		return EmptyDomainID, nil, nil
	}

	child, err := compiler.Source.Child(schema, "additionalProperties")
	if err != nil {
		return NoDomain, nil, compiler.failure("compile", "malformed", schema.Pointer, "additionalProperties", err)
	}

	use, err := compiler.compileSchema(child, active)
	if err != nil {
		return NoDomain, nil, err
	}

	return use.domain, use, nil
}

// compileProperties compiles each declared property Schema Object.
func (compiler *Compiler) compileProperties(
	use *schemaUse,
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

		childUse, err := compiler.compileSchema(child, active)
		if err != nil {
			return nil, err
		}

		use.properties = append(use.properties, schemaPropertyUse{name: name, use: childUse})
		properties[name] = NamedProperty{Name: name, State: PropertyAllowed, Values: childUse.domain}
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

// compileGenerationExamples parses trusted valid and invalid generation examples.
func compileGenerationExamples(pointer string, members map[string]json.RawMessage) (GenerationExamples, error) {
	var examples GenerationExamples

	if err := validateGenerationExamplePlacement(members); err != nil {
		return GenerationExamples{}, err
	}

	for _, keyword := range []string{"x-valid-examples", "x-invalid-examples"} {
		if err := compileGenerationExampleKeyword(pointer, members, keyword, &examples); err != nil {
			return GenerationExamples{}, err
		}
	}

	return examples, nil
}

// generationExampleError locates malformed extension input at its keyword.
type generationExampleError struct {
	Keyword string
	Cause   error
}

// Error returns the malformed extension cause.
func (exampleError *generationExampleError) Error() string {
	return exampleError.Cause.Error()
}

// validateGenerationExamplePlacement requires an opaque rule on the same Schema Object.
func validateGenerationExamplePlacement(members map[string]json.RawMessage) error {
	if hasOpaqueStringRule(members) {
		return nil
	}

	for _, keyword := range []string{"x-valid-examples", "x-invalid-examples"} {
		if _, ok := members[keyword]; ok {
			return &generationExampleError{
				Keyword: keyword,
				Cause:   errors.New("extension requires a pattern or format on the same Schema Object"),
			}
		}
	}

	return nil
}

// compileGenerationExampleKeyword parses one valid or invalid extension array.
func compileGenerationExampleKeyword(
	pointer string,
	members map[string]json.RawMessage,
	keyword string,
	examples *GenerationExamples,
) error {
	raw, ok := members[keyword]
	if !ok {
		return nil
	}

	target := &examples.Invalid
	if keyword == "x-valid-examples" {
		target = &examples.Valid
		examples.ValidDeclared = true
	}

	if isJSONNull(raw) {
		return &generationExampleError{Keyword: keyword, Cause: fmt.Errorf("%s must be an array", keyword)}
	}

	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return &generationExampleError{
			Keyword: keyword, Cause: fmt.Errorf("%s must be an array: %w", keyword, err),
		}
	}

	for _, valueRaw := range values {
		value, err := jsonvalue.Parse(valueRaw)
		if err != nil {
			return &generationExampleError{Keyword: keyword, Cause: fmt.Errorf("parse %s: %w", keyword, err)}
		}

		appendGenerationExample(target, GenerationExample{
			Value: value, Source: ConstraintSource{Pointer: pointer, Keyword: keyword},
		})
	}

	return nil
}

// hasOpaqueStringRule reports whether a direct pattern or format is declared.
func hasOpaqueStringRule(members map[string]json.RawMessage) bool {
	_, hasPattern := members["pattern"]
	_, hasFormat := members["format"]

	return hasPattern || hasFormat
}

// hasUnconstructibleStringDomain reports whether a reachable string format needs oracle evidence.
func hasUnconstructibleStringDomain(domain Domain) bool {
	return domain.String.State != KindExcluded &&
		len(domain.String.Formats) > 0
}

// hasReachableNonStringKind reports whether generation has another kind to attempt.
func hasReachableNonStringKind(domain Domain) bool {
	return domain.Null != KindExcluded || domain.Boolean != KindExcluded ||
		domain.Number.State != KindExcluded || domain.Array.State != KindExcluded ||
		domain.Object.State != KindExcluded
}

// appendGenerationExample appends one semantically distinct exact case.
func appendGenerationExample(examples *[]GenerationExample, candidate GenerationExample) {
	for _, example := range *examples {
		if example.Value.Equal(candidate.Value) {
			return
		}
	}

	*examples = append(*examples, GenerationExample{
		Value:  cloneJSONValue(candidate.Value),
		Source: candidate.Source,
	})
}

// generationExampleOverlap returns an invalid case also declared valid.
func generationExampleOverlap(examples GenerationExamples) *GenerationExample {
	for index := range examples.Invalid {
		for _, valid := range examples.Valid {
			if examples.Invalid[index].Value.Equal(valid.Value) {
				return &examples.Invalid[index]
			}
		}
	}

	return nil
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

// cloneJSONValues returns deep copies of exact JSON values.
func cloneJSONValues(values []jsonvalue.Value) []jsonvalue.Value {
	result := make([]jsonvalue.Value, len(values))
	for index, value := range values {
		result[index] = cloneJSONValue(value)
	}

	return result
}

// cloneGenerationExamples deep-copies exact values and preserves their source.
func cloneGenerationExamples(examples []GenerationExample) []GenerationExample {
	result := make([]GenerationExample, len(examples))
	for index, example := range examples {
		result[index] = GenerationExample{Value: cloneJSONValue(example.Value), Source: example.Source}
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
