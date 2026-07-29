//nolint:godoclint,lll // Private query compiler names and diagnostics are local implementation details.
package validation

import (
	"encoding/json"
	"fmt"
	"maps"
	"mime"
	"slices"
	"sort"
	"strings"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/go-json-experiment/json/jsontext"
)

type wireKind uint8

const (
	wirePrimitive wireKind = iota
	wireFormArrayRepeated
	wireDelimitedArray
	wireFormObjectNamed
	wireFormObjectExploded
	wireDelimitedObject
	wireDeepObject
	wireJSONContent
)

// QueryDecoder decodes and validates one operation's query parameters.
// It is immutable after Parse returns and safe for concurrent use.
type QueryDecoder struct {
	operationID string
	parameters  []queryParameter
	owners      map[string]queryClaim
	openForm    int
	validation  *Validation
}

type queryParameter struct {
	name              string
	wire              wireKind
	separator         string
	required          bool
	allowEmpty        bool
	validation        *Validation
	defaultValue      jsontext.Value
	scalarType        string
	dynamicType       string
	itemValidation    *Validation
	dynamicValidation *Validation
	properties        []queryProperty
	propertyByName    map[string]int
	conversions       []queryConversion
}

type queryConversion struct {
	validation        *Validation
	scalarType        string
	itemValidation    *Validation
	dynamicType       string
	dynamicValidation *Validation
	properties        []queryProperty
	propertyByName    map[string]int
}

type queryProperty struct {
	name           string
	scalarType     string
	array          bool
	validation     *Validation
	itemValidation *Validation
}

type queryClaim struct {
	parameter int
	property  int
}

// QueryDecoderDefinition is the generation-only compiled form of a QueryDecoder.
// Callers should normally use Parse instead.
// Validation pointers in definitions passed to NewQueryDecoderFromGenerated or
// returned by Definition are shared with the decoder.
// Do not mutate a definition while a decoder sharing it is in use;
// concurrent mutation has undefined behavior.
type QueryDecoderDefinition struct {
	OperationID string
	Parameters  []QueryParameterDefinition
}

// QueryParameterDefinition is one generation-only compiled query parameter.
type QueryParameterDefinition struct {
	Name         string
	Wire         uint8
	Separator    string
	Required     bool
	AllowEmpty   bool
	Validation   *Validation
	DefaultValue json.RawMessage
	ScalarType   string
	DynamicType  string
	Properties   []QueryPropertyDefinition
}

// QueryPropertyDefinition is one generation-only compiled object property.
type QueryPropertyDefinition struct {
	Name       string
	ScalarType string
	Array      bool
}

// Definition returns the generation-only compiled form of decoder.
// Its Validation pointers remain shared; see QueryDecoderDefinition.
func (decoder *QueryDecoder) Definition() QueryDecoderDefinition {
	definition := QueryDecoderDefinition{
		OperationID: decoder.operationID,
		Parameters:  make([]QueryParameterDefinition, len(decoder.parameters)),
	}
	for index, parameter := range decoder.parameters {
		compiled := QueryParameterDefinition{
			Name: parameter.name, Wire: uint8(parameter.wire), Separator: parameter.separator,
			Required: parameter.required, AllowEmpty: parameter.allowEmpty, Validation: parameter.validation,
			DefaultValue: append(json.RawMessage(nil), parameter.defaultValue...), ScalarType: parameter.scalarType,
			DynamicType: parameter.dynamicType,
			Properties:  make([]QueryPropertyDefinition, len(parameter.properties)),
		}
		for propertyIndex, property := range parameter.properties {
			compiled.Properties[propertyIndex] = QueryPropertyDefinition{
				Name: property.name, ScalarType: property.scalarType, Array: property.array,
			}
		}

		definition.Parameters[index] = compiled
	}

	return definition
}

// NewQueryDecoderFromGenerated restores a generator-produced decoder definition.
//
//nolint:funcorder // The definition method sits beside its public types above this restoring constructor.
func NewQueryDecoderFromGenerated(definition QueryDecoderDefinition) (*QueryDecoder, error) {
	parameters := make([]queryParameter, len(definition.Parameters))
	for index, compiled := range definition.Parameters {
		if compiled.Validation == nil || wireKind(compiled.Wire) > wireJSONContent {
			return nil, fmt.Errorf("generated query parameter %q is invalid", compiled.Name)
		}

		parameter := queryParameter{
			name: compiled.Name, wire: wireKind(compiled.Wire), separator: compiled.Separator,
			required: compiled.Required, allowEmpty: compiled.AllowEmpty, validation: compiled.Validation,
			defaultValue: append(jsontext.Value(nil), compiled.DefaultValue...), scalarType: compiled.ScalarType,
			dynamicType:    compiled.DynamicType,
			properties:     make([]queryProperty, len(compiled.Properties)),
			propertyByName: make(map[string]int, len(compiled.Properties)),
		}
		for propertyIndex, property := range compiled.Properties {
			parameter.properties[propertyIndex] = queryProperty{
				name: property.Name, scalarType: property.ScalarType, array: property.Array,
			}
			parameter.propertyByName[property.Name] = propertyIndex
		}

		attachQueryConversionValidations(&parameter)

		if err := prepareQueryConversions(&parameter); err != nil {
			return nil, fmt.Errorf("generated query parameter %q: %w", compiled.Name, err)
		}

		parameters[index] = parameter
	}

	return newQueryDecoder(definition.OperationID, parameters)
}

func compileQueryDecoder(operationID string, source oas.Source, compiler *schemaCompiler) (*QueryDecoder, error) {
	parameters := make([]queryParameter, 0, len(source.QueryParameters))
	for _, located := range source.QueryParameters {
		parameter, err := compileQueryParameter(located, compiler)
		if err != nil {
			return nil, fmt.Errorf("operationId %q compile query parameter: %w", operationID, err)
		}

		parameters = append(parameters, parameter)
	}

	return newQueryDecoder(operationID, parameters)
}

//nolint:cyclop // Exact owners and the one open-form namespace are registered in one finite pass.
func newQueryDecoder(operationID string, parameters []queryParameter) (*QueryDecoder, error) {
	decoder := &QueryDecoder{
		operationID: operationID,
		parameters:  parameters,
		owners:      make(map[string]queryClaim),
		openForm:    -1,
	}
	for index, parameter := range decoder.parameters {
		switch parameter.wire {
		case wireFormObjectExploded:
			if parameter.dynamicType != "" {
				if decoder.openForm != -1 {
					return nil, fmt.Errorf(
						"operationId %q compile query parameters %q and %q share an unsupported open form exploded bare-key namespace",
						operationID,
						decoder.parameters[decoder.openForm].name,
						parameter.name,
					)
				}

				decoder.openForm = index
			}

			for propertyIndex, property := range parameter.properties {
				if err := decoder.addOwner(property.name, queryClaim{parameter: index, property: propertyIndex}); err != nil {
					return nil, err
				}
			}
		case wireDeepObject:
			for propertyIndex, property := range parameter.properties {
				name := parameter.name + "[" + property.name + "]"
				if err := decoder.addOwner(name, queryClaim{parameter: index, property: propertyIndex}); err != nil {
					return nil, err
				}
			}
		default:
			if err := decoder.addOwner(parameter.name, queryClaim{parameter: index, property: -1}); err != nil {
				return nil, err
			}
		}
	}

	decoder.validation = syntheticQueryValidation(operationID, decoder.parameters)

	return decoder, nil
}

func (decoder *QueryDecoder) addOwner(name string, claim queryClaim) error {
	if existing, ok := decoder.owners[name]; ok {
		return fmt.Errorf(
			"operationId %q compile query ownership %q collides between parameters %q and %q",
			decoder.operationID,
			name,
			decoder.parameters[existing.parameter].name,
			decoder.parameters[claim.parameter].name,
		)
	}

	decoder.owners[name] = claim

	return nil
}

func strictMediaTypeParameters(mediaType string) bool {
	separator := strings.IndexByte(mediaType, ';')
	if separator == -1 {
		return true
	}

	seen := make(map[string]struct{})

	for _, parameter := range mediaTypeParameterSegments(mediaType[separator+1:]) {
		parameter = strings.TrimLeft(parameter, " \t")

		equal := strings.IndexByte(parameter, '=')
		if equal <= 0 {
			return false
		}

		if equal+1 >= len(parameter) {
			return false
		}

		if strings.ContainsRune(" \t", rune(parameter[equal-1])) {
			return false
		}

		if strings.ContainsRune(" \t", rune(parameter[equal+1])) {
			return false
		}

		name := strings.ToLower(parameter[:equal])
		if _, duplicate := seen[name]; duplicate {
			return false
		}

		seen[name] = struct{}{}
	}

	return true
}

func mediaTypeParameterSegments(parameters string) []string {
	segments := make([]string, 0, strings.Count(parameters, ";")+1)
	parameterStart := 0

	quoted := false

	escaped := false
	for index := range len(parameters) {
		if escaped {
			escaped = false

			continue
		}

		if quoted && parameters[index] == '\\' {
			escaped = true

			continue
		}

		if parameters[index] == '"' {
			quoted = !quoted

			continue
		}

		if parameters[index] != ';' || quoted {
			continue
		}

		segments = append(segments, parameters[parameterStart:index])
		parameterStart = index + 1
	}

	segments = append(segments, parameters[parameterStart:])

	return segments
}

//nolint:cyclop,funlen,gocognit,gocyclo,maintidx,nestif // Parameter Object rules form one finite decision table.
func compileQueryParameter(located oas.LocatedSchema, compiler *schemaCompiler) (queryParameter, error) {
	members, err := parameterMembers(located)
	if err != nil {
		return queryParameter{}, err
	}

	name, err := decodeString(members["name"], "name")
	if err != nil || name == "" {
		return queryParameter{}, fmt.Errorf("parameter at %s name must be a non-empty string", located.Pointer)
	}

	required, err := decodeOptionalBoolean(members, "required")
	if err != nil {
		return queryParameter{}, fmt.Errorf("parameter %q at %s required: %w", name, located.Pointer, err)
	}

	allowEmpty, err := decodeOptionalBoolean(members, "allowEmptyValue")
	if err != nil {
		return queryParameter{}, fmt.Errorf("parameter %q at %s allowEmptyValue: %w", name, located.Pointer, err)
	}

	if _, decodeErr := decodeOptionalBoolean(members, "allowReserved"); decodeErr != nil {
		return queryParameter{}, fmt.Errorf("parameter %q at %s allowReserved: %w", name, located.Pointer, decodeErr)
	}

	_, hasSchema := members["schema"]

	_, hasContent := members["content"]
	if hasSchema == hasContent {
		return queryParameter{}, fmt.Errorf("parameter %q at %s must contain exactly one of schema or content", name, located.Pointer)
	}

	parameter := queryParameter{name: name, required: required, allowEmpty: allowEmpty}

	var schema oas.LocatedSchema

	if hasContent {
		if _, ok := members["allowReserved"]; ok {
			return queryParameter{}, fmt.Errorf(
				"parameter %q at %s content cannot be combined with allowReserved", name, located.Pointer,
			)
		}

		if _, ok := members["style"]; ok {
			return queryParameter{}, fmt.Errorf("parameter %q at %s content cannot be combined with style", name, located.Pointer)
		}

		if _, ok := members["explode"]; ok {
			return queryParameter{}, fmt.Errorf("parameter %q at %s content cannot be combined with explode", name, located.Pointer)
		}

		contentPointer := locatedRawChild(located, members["content"], "content").Pointer

		var content map[string]json.RawMessage
		if unmarshalErr := json.Unmarshal(members["content"], &content); unmarshalErr != nil {
			return queryParameter{}, fmt.Errorf(
				"parameter %q content at %s must be an object: %w", name, contentPointer, unmarshalErr,
			)
		}

		if content == nil {
			return queryParameter{}, fmt.Errorf("parameter %q content at %s must be an object", name, contentPointer)
		}

		if len(content) != 1 {
			return queryParameter{}, fmt.Errorf(
				"parameter %q content at %s must contain exactly one media type", name, contentPointer,
			)
		}

		mediaTypeName := slices.Sorted(maps.Keys(content))[0]
		rawMediaType := content[mediaTypeName]

		parsedMediaType, _, parseMediaTypeErr := mime.ParseMediaType(mediaTypeName)
		if parseMediaTypeErr != nil {
			return queryParameter{}, fmt.Errorf(
				"parameter %q content at %s media type %q is malformed: %w",
				name, contentPointer, mediaTypeName, parseMediaTypeErr,
			)
		}

		if !strictMediaTypeParameters(mediaTypeName) {
			return queryParameter{}, fmt.Errorf(
				"parameter %q content at %s media type %q is malformed", name, contentPointer, mediaTypeName,
			)
		}

		if strings.Count(parsedMediaType, "/") != 1 {
			return queryParameter{}, fmt.Errorf(
				"parameter %q content at %s media type %q is malformed", name, contentPointer, mediaTypeName,
			)
		}

		if parsedMediaType != "application/json" {
			return queryParameter{}, fmt.Errorf(
				"parameter %q content at %s only application/json is supported, got %q",
				name, contentPointer, mediaTypeName,
			)
		}

		mediaTypePointer := locatedRawChild(located, rawMediaType, "content", mediaTypeName).Pointer

		var mediaType map[string]json.RawMessage
		if unmarshalErr := json.Unmarshal(rawMediaType, &mediaType); unmarshalErr != nil {
			return queryParameter{}, fmt.Errorf(
				"parameter %q Media Type Object at %s must be an object: %w", name, mediaTypePointer, unmarshalErr,
			)
		}

		if mediaType == nil {
			return queryParameter{}, fmt.Errorf(
				"parameter %q Media Type Object at %s must be an object", name, mediaTypePointer,
			)
		}

		rawSchema, ok := mediaType["schema"]
		if !ok {
			rawSchema = json.RawMessage(`{}`)
		}

		schema = locatedRawChild(located, rawSchema, "content", mediaTypeName, "schema")
		parameter.wire = wireJSONContent
	} else {
		schema = locatedRawChild(located, members["schema"], "schema")
	}

	parameter.validation, parameter.defaultValue, err = compileQueryParameterSchema(name, schema, compiler)
	if err != nil {
		return queryParameter{}, err
	}

	if hasContent {
		return parameter, nil
	}

	directType, err := compiledStyleType(parameter.validation)
	if err != nil {
		return queryParameter{}, fmt.Errorf("parameter %q: %w", name, err)
	}

	style := "form"
	if raw, ok := members["style"]; ok {
		style, err = decodeString(raw, "style")
		if err != nil {
			return queryParameter{}, fmt.Errorf("parameter %q at %s style: %w", name, located.Pointer, err)
		}
	}

	explode := style == "form"
	if raw, ok := members["explode"]; ok {
		explode, err = decodeBoolean(raw, "explode")
		if err != nil {
			return queryParameter{}, fmt.Errorf("parameter %q at %s explode: %w", name, located.Pointer, err)
		}
	}

	switch directType {
	case "boolean", "integer", "number", "string":
		if style != "form" {
			return queryParameter{}, unsupportedQueryStyle(name, style, explode, directType)
		}

		parameter.wire = wirePrimitive
		parameter.scalarType = directType
	case "array":
		arrayScalarType, typeErr := compiledArrayScalarType(parameter.validation)
		if typeErr != nil {
			return queryParameter{}, fmt.Errorf("parameter %q style-based array items: %w", name, typeErr)
		}

		parameter.scalarType = string(arrayScalarType)

		switch {
		case style == "form" && explode:
			parameter.wire = wireFormArrayRepeated
		case style == "form" && !explode:
			parameter.wire, parameter.separator = wireDelimitedArray, ","
		case style == "spaceDelimited" && !explode:
			parameter.wire, parameter.separator = wireDelimitedArray, " "
		case style == "pipeDelimited" && !explode:
			parameter.wire, parameter.separator = wireDelimitedArray, "|"
		default:
			return queryParameter{}, unsupportedQueryStyle(name, style, explode, directType)
		}
	case "object":
		parameter.properties, parameter.propertyByName, err = compileQueryProperties(
			parameter.validation,
			style == "deepObject",
		)
		if err != nil {
			return queryParameter{}, fmt.Errorf("parameter %q: %w", name, err)
		}

		switch {
		case style == "form" && explode:
			parameter.wire = wireFormObjectExploded
		case style == "form" && !explode:
			parameter.wire, parameter.separator = wireFormObjectNamed, ","
		case style == "spaceDelimited" && !explode:
			parameter.wire, parameter.separator = wireDelimitedObject, " "
		case style == "pipeDelimited" && !explode:
			parameter.wire, parameter.separator = wireDelimitedObject, "|"
		case style == "deepObject" && explode:
			if strings.ContainsAny(name, "[]") {
				return queryParameter{}, fmt.Errorf(
					"deepObject parameter name %q has an unsupported non-reversible bracket wire boundary",
					name,
				)
			}

			parameter.wire = wireDeepObject
		default:
			return queryParameter{}, unsupportedQueryStyle(name, style, explode, directType)
		}

		parameter.dynamicType, err = queryAdditionalPropertiesType(parameter.validation)
		if err != nil {
			return queryParameter{}, fmt.Errorf("parameter %q additionalProperties: %w", name, err)
		}

	default:
		return queryParameter{}, fmt.Errorf("parameter %q has unsupported direct type %q", name, directType)
	}

	attachQueryConversionValidations(&parameter)

	if err := prepareQueryConversions(&parameter); err != nil {
		return queryParameter{}, fmt.Errorf("parameter %q: %w", name, err)
	}

	return parameter, nil
}

func compileQueryParameterSchema(
	name string,
	schema oas.LocatedSchema,
	compiler *schemaCompiler,
) (*Validation, jsontext.Value, error) {
	validation, err := compiler.compile(schema)
	if err != nil {
		return nil, nil, fmt.Errorf("parameter %q schema: %w", name, err)
	}

	resolved, err := compiler.source.Resolve(schema)
	if err != nil {
		return nil, nil, fmt.Errorf("parameter %q: resolve schema at %s: %w", name, schema.Pointer, err)
	}

	members, err := schemaMembers(resolved)
	if err != nil {
		return nil, nil, fmt.Errorf("parameter %q: %w", name, err)
	}

	var defaultValue jsontext.Value
	if raw, ok := members["default"]; ok {
		defaultValue = append(jsontext.Value(nil), raw...)
	}

	return validation, defaultValue, nil
}

func compiledQueryScalarType(validations ...*Validation) (string, error) {
	return compiledScalarType(validations...)
}

func queryAdditionalPropertiesType(validation *Validation) (string, error) {
	if !compiledAdditionalPropertiesAllowed(validation) {
		return "", nil
	}

	additional := compiledAdditionalProperties(validation)
	if len(additional) == 0 {
		return "string", nil
	}

	typeName, err := compiledQueryScalarType(additional...)
	if err != nil {
		return "", fmt.Errorf("style-based dynamic properties cannot have satisfiable type: %w", err)
	}

	return typeName, nil
}

func compiledAdditionalPropertiesAllowed(validation *Validation) bool {
	if !validation.ObjectValidation.AdditionalPropertiesAllowed {
		return false
	}

	for _, child := range validation.AllOfValidations {
		if !compiledAdditionalPropertiesAllowed(child) {
			return false
		}
	}

	return true
}

func intersectQuerySchemaTypes(types []string) string {
	if len(types) == 0 {
		return "string"
	}

	intersection := types[0]
	for _, typeName := range types[1:] {
		if intersection == typeName {
			continue
		}

		if intersection == "number" && typeName == "integer" || intersection == "integer" && typeName == "number" {
			intersection = "integer"

			continue
		}

		return ""
	}

	return intersection
}

func parameterMembers(parameter oas.LocatedSchema) (map[string]json.RawMessage, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(parameter.Raw, &members); err != nil || members == nil {
		return nil, fmt.Errorf("parameter at %s must be an object", parameter.Pointer)
	}

	return members, nil
}

//nolint:cyclop // Compiled property and narrow array-extension checks form one compile decision.
func compileQueryProperties(
	validation *Validation,
	allowPrimitiveArrays bool,
) ([]queryProperty, map[string]int, error) {
	if validation == nil {
		return nil, nil, fmt.Errorf("compiled object validation is missing")
	}

	compiledByName := make(map[string][]*Validation)
	collectCompiledObjectProperties(validation, compiledByName)

	properties := make([]queryProperty, 0, len(compiledByName))

	byName := make(map[string]int, len(compiledByName))
	for _, name := range slices.Sorted(maps.Keys(compiledByName)) {
		if allowPrimitiveArrays && strings.ContainsAny(name, "[]") {
			return nil, nil, fmt.Errorf(
				"deepObject property name %q has an unsupported non-reversible bracket wire boundary",
				name,
			)
		}

		propertyValidations := compiledByName[name]

		typeName, err := compiledStyleType(conjunctiveValidation(propertyValidations...))
		if err != nil {
			return nil, nil, fmt.Errorf("style-based object property %q: %w", name, err)
		}

		property := queryProperty{
			name: name, scalarType: typeName,
			validation: conjunctiveValidation(propertyValidations...),
		}
		if typeName == "array" && allowPrimitiveArrays {
			items := make([]*Validation, 0)
			for _, propertyValidation := range propertyValidations {
				collectCompiledArrayItems(propertyValidation, &items)
			}

			var childErr error

			property.scalarType, childErr = compiledQueryScalarType(items...)
			if childErr != nil {
				return nil, nil, fmt.Errorf("deepObject array property %q items: %w", name, childErr)
			}

			property.array = true
			property.itemValidation = conjunctiveValidation(items...)
		} else if !isScalarType(typeName) {
			return nil, nil, fmt.Errorf("style-based object property %q must have a direct primitive type", name)
		}

		byName[name] = len(properties)
		properties = append(properties, property)
	}

	return properties, byName, nil
}

func attachQueryConversionValidations(parameter *queryParameter) {
	if parameter == nil || parameter.validation == nil {
		return
	}

	parameter.itemValidation = conjunctiveValidation(compiledArrayItems(parameter.validation)...)
	parameter.dynamicValidation = conjunctiveValidation(compiledAdditionalProperties(parameter.validation)...)

	byName := make(map[string][]*Validation)
	collectCompiledObjectProperties(parameter.validation, byName)

	for index := range parameter.properties {
		property := &parameter.properties[index]

		property.validation = conjunctiveValidation(byName[property.name]...)
		if property.array {
			var items []*Validation
			for _, validation := range byName[property.name] {
				collectCompiledArrayItems(validation, &items)
			}

			property.itemValidation = conjunctiveValidation(items...)
		}
	}
}

//nolint:cyclop // Each AnyOf candidate has separate primitive, array, and object metadata.
func prepareQueryConversions(parameter *queryParameter) error {
	if parameter == nil || parameter.validation == nil || parameter.wire == wireJSONContent {
		return nil
	}

	alternatives := conversionAlternatives(parameter.validation)
	if !containsAnyOf(parameter.validation) {
		parameter.conversions = []queryConversion{{
			validation: parameter.validation, scalarType: parameter.scalarType,
			itemValidation: parameter.itemValidation, dynamicType: parameter.dynamicType,
			dynamicValidation: parameter.dynamicValidation, properties: parameter.properties,
			propertyByName: parameter.propertyByName,
		}}

		return nil
	}

	parameter.conversions = make([]queryConversion, 0, len(alternatives))

	for _, validation := range alternatives {
		if _, possible := compiledValidationTypeState(validation); !possible {
			continue
		}

		conversion := queryConversion{validation: parameter.validation, scalarType: parameter.scalarType}

		switch parameter.wire {
		case wirePrimitive:
			if typeName := compiledValidationType(validation); typeName != "" {
				conversion.scalarType = typeName
			}
		case wireFormArrayRepeated, wireDelimitedArray:
			typeName, err := compiledArrayScalarType(validation)
			if err != nil {
				return err
			}

			conversion.scalarType = string(typeName)
			conversion.itemValidation = conjunctiveValidation(compiledArrayItems(validation)...)
		case wireFormObjectNamed, wireFormObjectExploded, wireDelimitedObject, wireDeepObject:
			properties, byName, err := compileQueryProperties(validation, parameter.wire == wireDeepObject)
			if err != nil {
				return err
			}

			conversion.properties = properties
			conversion.propertyByName = byName

			conversion.dynamicType, err = queryAdditionalPropertiesType(validation)
			if err != nil {
				return fmt.Errorf("additionalProperties: %w", err)
			}

			conversion.dynamicValidation = conjunctiveValidation(compiledAdditionalProperties(validation)...)
		}

		parameter.conversions = append(parameter.conversions, conversion)
	}

	if parameter.wire == wireFormObjectNamed || parameter.wire == wireFormObjectExploded ||
		parameter.wire == wireDelimitedObject || parameter.wire == wireDeepObject {
		return mergeQueryObjectConversions(parameter)
	}

	return nil
}

func mergeQueryObjectConversions(parameter *queryParameter) error {
	properties := make(map[string]queryProperty)
	parameter.dynamicType = ""
	parameter.dynamicValidation = nil

	for _, conversion := range parameter.conversions {
		if parameter.dynamicType == "" && conversion.dynamicType != "" {
			parameter.dynamicType = conversion.dynamicType
			parameter.dynamicValidation = conversion.dynamicValidation
		}

		for _, property := range conversion.properties {
			existing, ok := properties[property.name]
			if ok && existing.array != property.array {
				return fmt.Errorf("style-based object property %q has incompatible anyOf wire shapes", property.name)
			}

			if !ok {
				properties[property.name] = property
			}
		}
	}

	parameter.properties = make([]queryProperty, 0, len(properties))

	parameter.propertyByName = make(map[string]int, len(properties))
	for _, name := range slices.Sorted(maps.Keys(properties)) {
		parameter.propertyByName[name] = len(parameter.properties)
		parameter.properties = append(parameter.properties, properties[name])
	}

	return nil
}

func locatedRawChild(parent oas.LocatedSchema, raw json.RawMessage, tokens ...string) oas.LocatedSchema {
	pointer := parent.Pointer

	for _, token := range tokens {
		pointer += "/" + escapePointerToken(token)
	}

	return oas.LocatedSchema{Raw: append(json.RawMessage(nil), raw...), Pointer: pointer}
}

func syntheticQueryValidation(operationID string, parameters []queryParameter) *Validation {
	root := &Validation{
		SchemaPointer:  fmt.Sprintf("#/operations/%s/query", escapePointerToken(operationID)),
		KindValidation: KindValidation{Type: "object"},
	}

	root.ObjectValidation.Properties = make([]PropertyValidation, 0, len(parameters))
	for _, parameter := range parameters {
		root.ObjectValidation.Properties = append(root.ObjectValidation.Properties, PropertyValidation{
			Name: parameter.name, Validation: parameter.validation,
		})
		if parameter.required {
			root.ObjectValidation.Required = append(root.ObjectValidation.Required, parameter.name)
		}
	}

	sort.Slice(root.ObjectValidation.Properties, func(left int, right int) bool {
		return root.ObjectValidation.Properties[left].Name < root.ObjectValidation.Properties[right].Name
	})
	sort.Strings(root.ObjectValidation.Required)

	return root
}

func escapePointerToken(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")

	return strings.ReplaceAll(token, "/", "~1")
}

func isScalarType(typeName string) bool {
	return typeName == "boolean" || typeName == "integer" || typeName == "number" || typeName == "string"
}

func unsupportedQueryStyle(name string, style string, explode bool, typeName string) error {
	return fmt.Errorf("parameter %q style %q explode %t is unsupported for type %q", name, style, explode, typeName)
}
