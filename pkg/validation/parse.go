package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"decode_and_validate_generator/pkg/internal/jsonvalue"
	"decode_and_validate_generator/pkg/internal/oas"
)

// Parse selects one operation's JSON request body and compiles its reachable schema graph.
func Parse(spec []byte, operationID string) (*Validation, error) {
	source, err := oas.Parse(spec, operationID)
	if err != nil {
		return nil, err
	}

	compiler := schemaCompiler{
		source:   source,
		bySchema: make(map[string]*Validation),
		active:   make(map[string]int),
	}

	parsed, err := compiler.compile(source.RequestSchema, 0)
	if err != nil {
		return nil, err
	}

	parsed.BodyRequired = source.RequestBodyRequired

	return parsed, nil
}

// schemaCompiler owns call-local compilation state for one Parse.
type schemaCompiler struct {
	source   oas.Source
	bySchema map[string]*Validation
	active   map[string]int
}

// compile resolves and compiles one reachable schema occurrence.
func (compiler *schemaCompiler) compile(occurrence oas.LocatedSchema, instanceDepth int) (*Validation, error) {
	resolved, err := compiler.source.Resolve(occurrence)
	if err != nil {
		return nil, fmt.Errorf("resolve schema at %s: %w", occurrence.Pointer, err)
	}

	if activeDepth, ok := compiler.active[resolved.Pointer]; ok {
		if instanceDepth == activeDepth {
			return nil, fmt.Errorf("compile schema at %s: non-consuming schema cycle", resolved.Pointer)
		}

		return compiler.bySchema[resolved.Pointer], nil
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

	compiler.active[resolved.Pointer] = instanceDepth
	defer delete(compiler.active, resolved.Pointer)

	if err := compiler.compileKeywords(validation, resolved, members, instanceDepth); err != nil {
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
	for _, keyword := range []string{"oneOf", "anyOf", "not", "readOnly", "writeOnly", "discriminator"} {
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
		"xml": {}, "externalDocs": {},
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
	instanceDepth int,
) error {
	if err := compileDocumentation(schema.Pointer, members); err != nil {
		return err
	}

	if err := compileKind(validation, schema.Pointer, members); err != nil {
		return err
	}

	if err := compileEnum(validation, schema.Pointer, members); err != nil {
		return err
	}

	if err := compileNumber(validation, schema.Pointer, members); err != nil {
		return err
	}

	if err := compileString(validation, schema.Pointer, members); err != nil {
		return err
	}

	if err := compiler.compileArray(validation, schema, members, instanceDepth); err != nil {
		return err
	}

	if err := compiler.compileObject(validation, schema, members, instanceDepth); err != nil {
		return err
	}

	return compiler.compileAllOf(validation, schema, members, instanceDepth)
}

// compileDocumentation validates documentation-only field shapes.
func compileDocumentation(pointer string, members map[string]json.RawMessage) error {
	for _, keyword := range []string{"title", "description"} {
		if raw, ok := members[keyword]; ok {
			if _, err := decodeString(raw, keyword); err != nil {
				return keywordError(pointer, keyword, err)
			}
		}
	}

	if raw, ok := members["deprecated"]; ok {
		if _, err := decodeBoolean(raw, "deprecated"); err != nil {
			return keywordError(pointer, "deprecated", err)
		}
	}

	for _, keyword := range []string{"xml", "externalDocs"} {
		if raw, ok := members[keyword]; ok {
			var object map[string]json.RawMessage
			if err := json.Unmarshal(raw, &object); err != nil || object == nil {
				return keywordError(pointer, keyword, errors.New("must be an object"))
			}
		}
	}

	return nil
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

	validation.EnumValidation.exactValues = make([]jsonvalue.Value, len(values))
	for index, value := range values {
		exact, err := jsonvalue.Parse(value)
		if err != nil {
			return keywordError(pointer, "enum", fmt.Errorf("member %d: %w", index, err))
		}

		validation.EnumValidation.Values[index] = append(json.RawMessage(nil), value...)
		validation.EnumValidation.exactValues[index] = exact
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
			Value: minimum.Lexeme, Exclusive: exclusiveMinimum, exactValue: *minimum,
		}
	}

	if maximum != nil {
		validation.NumberValidation.Maximum = &NumberBound{
			Value: maximum.Lexeme, Exclusive: exclusiveMaximum, exactValue: *maximum,
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
		validation.NumberValidation.exactMultipleOf = &multiple
	}

	validation.NumberValidation.Integer = validation.KindValidation.Type == "integer"

	return nil
}

// compileString compiles string length, pattern, and format constraints.
func compileString(validation *Validation, pointer string, members map[string]json.RawMessage) error {
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

		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return keywordError(pointer, "pattern", fmt.Errorf("unsupported RE2 pattern: %w", err))
		}

		validation.StringValidation.Pattern = pattern
		validation.StringValidation.compiledPattern = compiled
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
	instanceDepth int,
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

		validation.ArrayValidation.Items, err = compiler.compile(child, instanceDepth+1)
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
	instanceDepth int,
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

	if raw, ok := members["properties"]; ok {
		var properties map[string]json.RawMessage
		if err := json.Unmarshal(raw, &properties); err != nil || properties == nil {
			return keywordError(schema.Pointer, "properties", errors.New("must be an object"))
		}

		names := make([]string, 0, len(properties))
		for name := range properties {
			names = append(names, name)
		}

		sort.Strings(names)

		validation.ObjectValidation.Properties = make([]PropertyValidation, 0, len(names))
		for _, name := range names {
			child, err := compiler.source.Child(schema, "properties", name)
			if err != nil {
				return keywordError(schema.Pointer, "properties", err)
			}

			parsed, err := compiler.compile(child, instanceDepth+1)
			if err != nil {
				return err
			}

			validation.ObjectValidation.Properties = append(validation.ObjectValidation.Properties, PropertyValidation{
				Name: name, Validation: parsed,
			})
		}
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

			validation.ObjectValidation.AdditionalPropertiesValidation, err = compiler.compile(child, instanceDepth+1)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// compileAllOf preserves composition source order without flattening.
func (compiler *schemaCompiler) compileAllOf(
	validation *Validation,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	instanceDepth int,
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

		parsed, err := compiler.compile(child, instanceDepth)
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
func decodeOptionalNonNegativeInteger(members map[string]json.RawMessage, keyword string) (*int, error) {
	raw, ok := members[keyword]
	if !ok {
		//nolint:nilnil // A nil value is the explicit representation of an absent optional keyword.
		return nil, nil
	}

	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("%s must be a non-negative integer", keyword)
	}

	var value int
	if err := json.Unmarshal(raw, &value); err != nil || value < 0 {
		return nil, fmt.Errorf("%s must be a non-negative integer", keyword)
	}

	return &value, nil
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
