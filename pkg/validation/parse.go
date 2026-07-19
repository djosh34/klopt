package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
)

// Parse compiles every JSON request-body validation and query decoder in one OpenAPI document.
func Parse(
	spec []byte,
	patternOptions ...patternvalidator.Option,
) (map[string]*Validation, map[string]*QueryDecoder, error) {
	sources, err := oas.Parse(spec)
	if err != nil {
		return nil, nil, err
	}

	validations := make(map[string]*Validation, len(sources))

	queryDecoders := make(map[string]*QueryDecoder, len(sources))
	for _, operationID := range slices.Sorted(maps.Keys(sources)) {
		source := sources[operationID]
		compiler := schemaCompiler{
			source:         source,
			bySchema:       make(map[string]*Validation),
			active:         make(map[string]struct{}),
			patternOptions: patternOptions,
		}

		if len(source.RequestSchema.Raw) != 0 {
			root, err := compiler.compile(source.RequestSchema)
			if err != nil {
				return nil, nil, fmt.Errorf("compile operationId %q: %w", operationID, err)
			}

			root.BodyRequired = source.RequestBodyRequired
			validations[operationID] = root
		}

		if len(source.QueryParameters) != 0 {
			decoder, err := compileQueryDecoder(operationID, source, &compiler)
			if err != nil {
				return nil, nil, err
			}

			queryDecoders[operationID] = decoder
		}
	}

	return validations, queryDecoders, nil
}

// schemaCompiler owns compilation state for one operation.
type schemaCompiler struct {
	source         oas.Source
	bySchema       map[string]*Validation
	active         map[string]struct{}
	patternOptions []patternvalidator.Option
}

// PatternOptions composes pattern options for APIs that accept exactly one option.
func PatternOptions(options ...patternvalidator.Option) patternvalidator.Option {
	for _, option := range options {
		if option == nil {
			panic("nil pattern option")
		}
	}

	return func(validation *patternvalidator.PatternValidation) {
		for _, option := range options {
			option(validation)
		}
	}
}

// compile resolves and compiles one reachable schema occurrence.
func (compiler *schemaCompiler) compile(occurrence oas.LocatedSchema) (*Validation, error) {
	resolved, err := compiler.source.Resolve(occurrence)
	if err != nil {
		return nil, fmt.Errorf("resolve schema at %s: %w", occurrence.Pointer, err)
	}

	if _, cyclic := compiler.active[resolved.Pointer]; cyclic {
		return nil, fmt.Errorf("compile schema at %s: recursive schema is unsupported", resolved.Pointer)
	}

	if validation, ok := compiler.bySchema[resolved.Pointer]; ok {
		return validation, nil
	}

	members, err := schemaMembers(resolved)
	if err != nil {
		return nil, err
	}

	if err := rejectUnsupportedKeywords(resolved.Pointer, members); err != nil {
		return nil, err
	}

	validation := &Validation{SchemaPointer: resolved.Pointer}
	validation.ObjectValidation.AdditionalPropertiesAllowed = true
	compiler.bySchema[resolved.Pointer] = validation

	compiler.active[resolved.Pointer] = struct{}{}
	defer delete(compiler.active, resolved.Pointer)

	if err := compiler.compileKeywords(validation, resolved, members); err != nil {
		return nil, err
	}

	return validation, nil
}

// schemaMembers decodes one Schema Object's members.
func schemaMembers(schema oas.LocatedSchema) (map[string]json.RawMessage, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(schema.Raw, &members); err != nil {
		return nil, fmt.Errorf("parse schema at %s: Schema Object must be an object: %w", schema.Pointer, err)
	}

	if members == nil {
		return nil, fmt.Errorf("parse schema at %s: Schema Object must be an object", schema.Pointer)
	}

	return members, nil
}

// rejectUnsupportedKeywords rejects behavior outside the runtime validator contract.
func rejectUnsupportedKeywords(pointer string, members map[string]json.RawMessage) error {
	for _, keyword := range []string{"oneOf", "anyOf", "not"} {
		if _, ok := members[keyword]; ok {
			return fmt.Errorf("compile schema at %s/%s: unsupported keyword", pointer, keyword)
		}
	}

	supported := map[string]struct{}{
		"$ref": {}, "type": {}, "nullable": {}, "enum": {},
		"minimum": {}, "maximum": {}, "exclusiveMinimum": {}, "exclusiveMaximum": {}, "multipleOf": {},
		"minLength": {}, "maxLength": {}, "pattern": {}, "format": {},
		"minItems": {}, "maxItems": {}, "items": {}, "uniqueItems": {},
		"minProperties": {}, "maxProperties": {}, "required": {}, "properties": {}, "additionalProperties": {},
		"allOf": {}, "title": {}, "description": {}, "default": {}, "example": {}, "deprecated": {},
		"readOnly": {}, "writeOnly": {}, "discriminator": {}, "xml": {}, "externalDocs": {},
	}

	for keyword := range members {
		if _, ok := supported[keyword]; ok || strings.HasPrefix(keyword, "x-") {
			continue
		}

		return fmt.Errorf("compile schema at %s/%s: unsupported Schema Object keyword", pointer, keyword)
	}

	return nil
}

// compileKeywords compiles validation keyword families in validation order.
func (compiler *schemaCompiler) compileKeywords(
	validation *Validation,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
) error {
	if err := compileKind(validation, schema.Pointer, members); err != nil {
		return err
	}

	if err := compileDocumentation(validation, schema.Pointer, members); err != nil {
		return err
	}

	if err := compileEnum(validation, schema.Pointer, members); err != nil {
		return err
	}

	if err := compileNumber(validation, schema.Pointer, members); err != nil {
		return err
	}

	if err := compiler.compileString(validation, schema.Pointer, members); err != nil {
		return err
	}

	if err := compiler.compileArray(validation, schema, members); err != nil {
		return err
	}

	if err := compiler.compileObject(validation, schema, members); err != nil {
		return err
	}

	return compiler.compileAllOf(validation, schema, members)
}

// compileDocumentation validates documentation-only field shapes.
//
//nolint:cyclop // Each independent documentation field needs its own malformed-input diagnostic.
func compileDocumentation(validation *Validation, pointer string, members map[string]json.RawMessage) error {
	for _, keyword := range []string{"title", "description"} {
		if raw, ok := members[keyword]; ok {
			if _, err := decodeString(raw, keyword); err != nil {
				return keywordError(pointer, keyword, err)
			}
		}
	}

	for _, keyword := range []string{"readOnly", "writeOnly", "deprecated"} {
		if raw, ok := members[keyword]; ok {
			if _, err := decodeBoolean(raw, keyword); err != nil {
				return keywordError(pointer, keyword, err)
			}
		}
	}

	if raw, ok := members["default"]; ok {
		if err := validateDefault(raw, validation.KindValidation); err != nil {
			return keywordError(pointer, "default", err)
		}
	}

	if raw, ok := members["xml"]; ok {
		if err := validateXML(raw); err != nil {
			return keywordError(pointer, "xml", err)
		}
	}

	if raw, ok := members["externalDocs"]; ok {
		if err := validateExternalDocs(raw); err != nil {
			return keywordError(pointer, "externalDocs", err)
		}
	}

	if err := validateDiscriminator(pointer, members); err != nil {
		return err
	}

	return nil
}

// validateDiscriminator shape-checks an inert hint. Klopt intentionally accepts
// it without oneOf, anyOf, or allOf and does not store or apply its values.
//
//nolint:cyclop // Discriminator Object fixed fields and mapping values have distinct diagnostics.
func validateDiscriminator(pointer string, members map[string]json.RawMessage) error {
	raw, ok := members["discriminator"]
	if !ok {
		return nil
	}

	discriminatorPointer := pointer + "/discriminator"

	var discriminator map[string]json.RawMessage
	if err := json.Unmarshal(raw, &discriminator); err != nil || discriminator == nil {
		return fmt.Errorf("compile schema at %s: must be an object", discriminatorPointer)
	}

	propertyName, ok := discriminator["propertyName"]
	if !ok {
		return fmt.Errorf("compile schema at %s/propertyName: is required", discriminatorPointer)
	}

	if _, err := decodeString(propertyName, "propertyName"); err != nil {
		return fmt.Errorf("compile schema at %s/propertyName: %w", discriminatorPointer, err)
	}

	mappingRaw, ok := discriminator["mapping"]
	if !ok {
		return nil
	}

	var mapping map[string]json.RawMessage
	if err := json.Unmarshal(mappingRaw, &mapping); err != nil || mapping == nil {
		return fmt.Errorf("compile schema at %s/mapping: must be an object", discriminatorPointer)
	}

	for _, name := range slices.Sorted(maps.Keys(mapping)) {
		if _, err := decodeString(mapping[name], "mapping value"); err != nil {
			escaped := strings.ReplaceAll(name, "~", "~0")
			escaped = strings.ReplaceAll(escaped, "/", "~1")

			return fmt.Errorf("compile schema at %s/mapping/%s: %w", discriminatorPointer, escaped, err)
		}
	}

	return nil
}

// validateDefault enforces OpenAPI's same-Schema-Object type rule for defaults.
func validateDefault(raw json.RawMessage, kind KindValidation) error {
	if kind.Type == "" {
		return nil
	}

	value, err := jsonvalue.Parse(raw)
	if err != nil {
		return fmt.Errorf("must be a valid value: %w", err)
	}

	if value.Kind == jsonvalue.KindNull && kind.Nullable {
		return nil
	}

	matches := map[string]bool{
		"boolean": value.Kind == jsonvalue.KindBoolean,
		"integer": value.Kind == jsonvalue.KindNumber && value.Number.IsInteger(),
		"number":  value.Kind == jsonvalue.KindNumber,
		"string":  value.Kind == jsonvalue.KindString,
		"array":   value.Kind == jsonvalue.KindArray,
		"object":  value.Kind == jsonvalue.KindObject,
	}
	if !matches[kind.Type] {
		return fmt.Errorf("must conform to type %q", kind.Type)
	}

	return nil
}

// validateXML validates the fixed fields of an OpenAPI XML Object.
//
//nolint:cyclop // XML Object fixed fields have distinct required shapes.
func validateXML(raw json.RawMessage) error {
	object, err := documentationObject(raw)
	if err != nil {
		return err
	}

	for name, value := range object {
		switch name {
		case "name", "prefix":
			if _, err := decodeString(value, name); err != nil {
				return err
			}
		case "namespace":
			namespace, err := decodeString(value, name)
			if err != nil {
				return err
			}

			parsed, err := url.Parse(namespace)
			if err != nil || !parsed.IsAbs() {
				return errors.New("namespace must be an absolute URI")
			}
		case "attribute", "wrapped":
			if _, err := decodeBoolean(value, name); err != nil {
				return err
			}
		default:
			if !strings.HasPrefix(name, "x-") {
				return fmt.Errorf("unsupported field %q", name)
			}
		}
	}

	return nil
}

// validateExternalDocs validates the fixed fields of an External Documentation Object.
//
//nolint:cyclop // External Documentation fixed fields have distinct required shapes.
func validateExternalDocs(raw json.RawMessage) error {
	object, err := documentationObject(raw)
	if err != nil {
		return err
	}

	urlRaw, ok := object["url"]
	if !ok {
		return errors.New("url is required")
	}

	for name, value := range object {
		switch name {
		case "description", "url":
			if _, decodeErr := decodeString(value, name); decodeErr != nil {
				return decodeErr
			}
		default:
			if !strings.HasPrefix(name, "x-") {
				return fmt.Errorf("unsupported field %q", name)
			}
		}
	}

	documentationURL, err := decodeString(urlRaw, "url")
	if err != nil {
		return err
	}

	if documentationURL == "" {
		return errors.New("url must not be empty")
	}

	if _, err := url.Parse(documentationURL); err != nil {
		return fmt.Errorf("url must be a URI reference: %w", err)
	}

	return nil
}

// documentationObject decodes one non-null documentation object.
func documentationObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, errors.New("must be an object")
	}

	return object, nil
}

// compileKind compiles type and same-object nullable.
func compileKind(validation *Validation, pointer string, members map[string]json.RawMessage) error {
	typeRaw, hasType := members["type"]
	if hasType {
		typeName, err := decodeString(typeRaw, "type")
		if err != nil {
			return keywordError(pointer, "type", err)
		}

		switch typeName {
		case "boolean", "integer", "number", "string", "array", "object":
			validation.KindValidation.Type = typeName
		default:
			return keywordError(pointer, "type", fmt.Errorf("unsupported type %q", typeName))
		}
	}

	if raw, ok := members["nullable"]; ok {
		nullable, err := decodeBoolean(raw, "nullable")
		if err != nil {
			return keywordError(pointer, "nullable", err)
		}

		validation.KindValidation.Nullable = hasType && nullable
	}

	return nil
}

// compileEnum compiles exact semantic enum values.
func compileEnum(validation *Validation, pointer string, members map[string]json.RawMessage) error {
	raw, ok := members["enum"]
	if !ok {
		return nil
	}

	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil || values == nil || len(values) == 0 {
		return keywordError(pointer, "enum", errors.New("must be a non-empty array"))
	}

	validation.EnumValidation.Values = make([]json.RawMessage, len(values))

	validation.EnumValidation.ExactValues = make([]jsonvalue.Value, len(values))
	for index, value := range values {
		exact, err := jsonvalue.Parse(value)
		if err != nil {
			return keywordError(pointer, "enum", fmt.Errorf("member %d: %w", index, err))
		}

		validation.EnumValidation.Values[index] = append(json.RawMessage(nil), value...)
		validation.EnumValidation.ExactValues[index] = exact
	}

	return nil
}

// compileNumber compiles exact numeric constraints.
//
//nolint:cyclop // OpenAPI numeric keywords are independent and intentionally compiled together.
func compileNumber(validation *Validation, pointer string, members map[string]json.RawMessage) error {
	minimum, err := decodeOptionalNumber(members, "minimum")
	if err != nil {
		return keywordError(pointer, "minimum", err)
	}

	maximum, err := decodeOptionalNumber(members, "maximum")
	if err != nil {
		return keywordError(pointer, "maximum", err)
	}

	exclusiveMinimum, err := decodeOptionalBoolean(members, "exclusiveMinimum")
	if err != nil {
		return keywordError(pointer, "exclusiveMinimum", err)
	}

	exclusiveMaximum, err := decodeOptionalBoolean(members, "exclusiveMaximum")
	if err != nil {
		return keywordError(pointer, "exclusiveMaximum", err)
	}

	if minimum != nil {
		validation.NumberValidation.Minimum = &NumberBound{
			Value: minimum.Lexeme, Exclusive: exclusiveMinimum, ExactValue: *minimum,
		}
	}

	if maximum != nil {
		validation.NumberValidation.Maximum = &NumberBound{
			Value: maximum.Lexeme, Exclusive: exclusiveMaximum, ExactValue: *maximum,
		}
	}

	if raw, ok := members["multipleOf"]; ok {
		multiple, err := decodeNumber(raw, "multipleOf")
		if err != nil {
			return keywordError(pointer, "multipleOf", err)
		}

		zero, zeroErr := jsonvalue.ParseNumber("0")
		if zeroErr != nil {
			return keywordError(pointer, "multipleOf", zeroErr)
		}

		if multiple.Compare(zero) <= 0 {
			return keywordError(pointer, "multipleOf", errors.New("must be greater than zero"))
		}

		validation.NumberValidation.MultipleOf = multiple.Lexeme
		validation.NumberValidation.ExactMultipleOf = &multiple
	}

	return nil
}

// compileString compiles string length, pattern, and format constraints.
func (compiler *schemaCompiler) compileString(
	validation *Validation,
	pointer string,
	members map[string]json.RawMessage,
) error {
	minimum, err := decodeOptionalNonNegativeInteger(members, "minLength")
	if err != nil {
		return keywordError(pointer, "minLength", err)
	}

	maximum, err := decodeOptionalNonNegativeInteger(members, "maxLength")
	if err != nil {
		return keywordError(pointer, "maxLength", err)
	}

	validation.StringValidation.MinLength = minimum
	validation.StringValidation.MaxLength = maximum

	if raw, ok := members["pattern"]; ok {
		pattern, err := decodeString(raw, "pattern")
		if err != nil {
			return keywordError(pointer, "pattern", err)
		}

		compiled, err := patternvalidator.Parse(pattern, compiler.patternOptions...)
		if err != nil {
			return keywordError(pointer, "pattern", err)
		}

		validation.StringValidation.Pattern = pattern
		validation.StringValidation.CompiledPattern = compiled
	}

	if raw, ok := members["format"]; ok {
		format, err := decodeString(raw, "format")
		if err != nil {
			return keywordError(pointer, "format", err)
		}

		validation.StringValidation.Format = format
	}

	return nil
}

// compileArray compiles array bounds, uniqueness, and item recursion.
func (compiler *schemaCompiler) compileArray(
	validation *Validation,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
) error {
	minimum, err := decodeOptionalNonNegativeInteger(members, "minItems")
	if err != nil {
		return keywordError(schema.Pointer, "minItems", err)
	}

	maximum, err := decodeOptionalNonNegativeInteger(members, "maxItems")
	if err != nil {
		return keywordError(schema.Pointer, "maxItems", err)
	}

	validation.ArrayValidation.MinItems = minimum
	validation.ArrayValidation.MaxItems = maximum

	if validation.KindValidation.Type == "array" {
		if _, ok := members["items"]; !ok {
			return keywordError(schema.Pointer, "items", errors.New("must be present when type is array"))
		}
	}

	if raw, ok := members["uniqueItems"]; ok {
		unique, err := decodeBoolean(raw, "uniqueItems")
		if err != nil {
			return keywordError(schema.Pointer, "uniqueItems", err)
		}

		validation.ArrayValidation.UniqueItems = unique
	}

	if _, ok := members["items"]; ok {
		child, err := compiler.source.Child(schema, "items")
		if err != nil {
			return keywordError(schema.Pointer, "items", err)
		}

		validation.ArrayValidation.Items, err = compiler.compile(child)
		if err != nil {
			return err
		}
	}

	return nil
}

// compileObject compiles object bounds, names, and child schemas.
//
//nolint:cyclop // Object keywords require distinct malformed-input diagnostics.
func (compiler *schemaCompiler) compileObject(
	validation *Validation,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
) error {
	minimum, err := decodeOptionalNonNegativeInteger(members, "minProperties")
	if err != nil {
		return keywordError(schema.Pointer, "minProperties", err)
	}

	maximum, err := decodeOptionalNonNegativeInteger(members, "maxProperties")
	if err != nil {
		return keywordError(schema.Pointer, "maxProperties", err)
	}

	validation.ObjectValidation.MinProperties = minimum
	validation.ObjectValidation.MaxProperties = maximum

	required, err := decodeRequired(members["required"])
	if err != nil {
		return keywordError(schema.Pointer, "required", err)
	}

	validation.ObjectValidation.Required = required

	if err := compiler.compileObjectProperties(validation, schema, members); err != nil {
		return err
	}

	//nolint:nestif // Boolean and schema forms need separate diagnostics and compilation paths.
	if raw, ok := members["additionalProperties"]; ok {
		trimmed := bytes.TrimSpace(raw)
		if bytes.Equal(trimmed, []byte("true")) || bytes.Equal(trimmed, []byte("false")) {
			allowed, err := decodeBoolean(raw, "additionalProperties")
			if err != nil {
				return keywordError(schema.Pointer, "additionalProperties", err)
			}

			validation.ObjectValidation.AdditionalPropertiesAllowed = allowed
		} else {
			child, err := compiler.source.Child(schema, "additionalProperties")
			if err != nil {
				return keywordError(schema.Pointer, "additionalProperties", err)
			}

			validation.ObjectValidation.AdditionalPropertiesValidation, err = compiler.compile(child)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// compileObjectProperties compiles named properties and applies request direction semantics.
func (compiler *schemaCompiler) compileObjectProperties(
	validation *Validation,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
) error {
	raw, ok := members["properties"]
	if !ok {
		return nil
	}

	var properties map[string]json.RawMessage
	if err := json.Unmarshal(raw, &properties); err != nil || properties == nil {
		return keywordError(schema.Pointer, "properties", errors.New("must be an object"))
	}

	names := slices.Sorted(maps.Keys(properties))

	validation.ObjectValidation.Properties = make([]PropertyValidation, 0, len(names))
	for _, name := range names {
		child, err := compiler.source.Child(schema, "properties", name)
		if err != nil {
			return keywordError(schema.Pointer, "properties", err)
		}

		parsed, err := compiler.compile(child)
		if err != nil {
			return err
		}

		readOnly, err := compiler.requestPropertyReadOnly(child)
		if err != nil {
			return err
		}

		if readOnly {
			validation.ObjectValidation.Required = slices.DeleteFunc(
				validation.ObjectValidation.Required,
				func(required string) bool { return required == name },
			)
		}

		validation.ObjectValidation.Properties = append(validation.ObjectValidation.Properties, PropertyValidation{
			Name: name, Validation: parsed,
		})
	}

	return nil
}

// requestPropertyReadOnly applies request direction rules after local reference resolution.
func (compiler *schemaCompiler) requestPropertyReadOnly(property oas.LocatedSchema) (bool, error) {
	resolved, err := compiler.source.Resolve(property)
	if err != nil {
		return false, fmt.Errorf("resolve schema at %s: %w", property.Pointer, err)
	}

	members, err := schemaMembers(resolved)
	if err != nil {
		return false, err
	}

	readOnly, err := decodeOptionalBoolean(members, "readOnly")
	if err != nil {
		return false, keywordError(resolved.Pointer, "readOnly", err)
	}

	writeOnly, err := decodeOptionalBoolean(members, "writeOnly")
	if err != nil {
		return false, keywordError(resolved.Pointer, "writeOnly", err)
	}

	if readOnly && writeOnly {
		return false, fmt.Errorf(
			"compile schema at %s: readOnly and writeOnly must not both be true",
			resolved.Pointer,
		)
	}

	return readOnly, nil
}

// compileAllOf preserves composition source order without flattening.
func (compiler *schemaCompiler) compileAllOf(
	validation *Validation,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
) error {
	raw, ok := members["allOf"]
	if !ok {
		return nil
	}

	var children []json.RawMessage
	if err := json.Unmarshal(raw, &children); err != nil || len(children) == 0 {
		return keywordError(schema.Pointer, "allOf", errors.New("must be a non-empty array"))
	}

	validation.AllOfValidations = make([]*Validation, 0, len(children))
	for index := range children {
		child, err := compiler.source.Child(schema, "allOf", fmt.Sprintf("%d", index))
		if err != nil {
			return keywordError(schema.Pointer, "allOf", err)
		}

		parsed, err := compiler.compile(child)
		if err != nil {
			return err
		}

		validation.AllOfValidations = append(validation.AllOfValidations, parsed)
	}

	return nil
}

// decodeOptionalNumber decodes an absent-or-exact-number keyword.
func decodeOptionalNumber(members map[string]json.RawMessage, keyword string) (*jsonvalue.Number, error) {
	raw, ok := members[keyword]
	if !ok {
		//nolint:nilnil // A nil value is the explicit representation of an absent optional keyword.
		return nil, nil
	}

	value, err := decodeNumber(raw, keyword)
	if err != nil {
		return nil, err
	}

	return &value, nil
}

// decodeNumber decodes one exact JSON numeric keyword.
func decodeNumber(raw json.RawMessage, keyword string) (jsonvalue.Number, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return jsonvalue.Number{}, fmt.Errorf("%s must be a number", keyword)
	}

	value, err := jsonvalue.ParseNumber(string(trimmed))
	if err != nil {
		return jsonvalue.Number{}, fmt.Errorf("%s must be a number: %w", keyword, err)
	}

	return value, nil
}

// decodeOptionalBoolean decodes an absent-or-boolean keyword.
func decodeOptionalBoolean(members map[string]json.RawMessage, keyword string) (bool, error) {
	raw, ok := members[keyword]
	if !ok {
		return false, nil
	}

	return decodeBoolean(raw, keyword)
}

// decodeBoolean decodes one required boolean keyword value.
func decodeBoolean(raw json.RawMessage, keyword string) (bool, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false, fmt.Errorf("%s must be a boolean", keyword)
	}

	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", keyword, err)
	}

	return value, nil
}

// decodeString decodes one required string keyword value.
func decodeString(raw json.RawMessage, keyword string) (string, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be a string", keyword)
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string: %w", keyword, err)
	}

	return value, nil
}

// decodeOptionalNonNegativeInteger decodes an optional collection bound.
func decodeOptionalNonNegativeInteger(members map[string]json.RawMessage, keyword string) (*CountBound, error) {
	raw, ok := members[keyword]
	if !ok {
		//nolint:nilnil // A nil value is the explicit representation of an absent optional keyword.
		return nil, nil
	}

	value, err := decodeNumber(raw, keyword)
	if err != nil || !value.IsInteger() {
		return nil, fmt.Errorf("%s must be a non-negative integer", keyword)
	}

	zero, err := jsonvalue.ParseNumber("0")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", keyword, err)
	}

	if value.Compare(zero) < 0 {
		return nil, fmt.Errorf("%s must be a non-negative integer", keyword)
	}

	return &CountBound{Value: value.Lexeme, ExactValue: value}, nil
}

// decodeRequired decodes and lexically sorts unique required property names.
func decodeRequired(raw json.RawMessage) ([]string, error) {
	if raw == nil {
		return nil, nil
	}

	var names []string
	if err := json.Unmarshal(raw, &names); err != nil || names == nil || len(names) == 0 {
		return nil, errors.New("must be a non-empty array of unique strings")
	}

	sort.Strings(names)

	for index := 1; index < len(names); index++ {
		if names[index] == names[index-1] {
			return nil, errors.New("must contain unique strings")
		}
	}

	return names, nil
}

// keywordError adds stable schema location and keyword context.
func keywordError(pointer string, keyword string, err error) error {
	return fmt.Errorf("compile schema at %s/%s: %w", pointer, keyword, err)
}
