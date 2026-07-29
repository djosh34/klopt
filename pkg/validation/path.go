//nolint:godoclint,lll // Private path runtime names and diagnostics are local implementation details.
package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"mime"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type pathWireKind uint8

type pathShape uint8

const (
	pathWireSimplePrimitive pathWireKind = iota
	pathWireSimpleArray
	pathWireSimpleObject
	pathWireLabelPrimitive
	pathWireLabelArray
	pathWireLabelObject
	pathWireMatrixPrimitive
	pathWireMatrixArray
	pathWireMatrixObject
	pathWireJSONContent
)

const (
	pathShapePrimitive pathShape = iota
	pathShapeArray
	pathShapeObject
	pathShapeCount
)

type styleScalarType string

type pathParameter struct {
	name              string
	wire              pathWireKind
	explode           bool
	validation        *Validation
	scalarType        styleScalarType
	dynamicType       styleScalarType
	itemValidation    *Validation
	dynamicValidation *Validation
	properties        []pathProperty
	propertyByName    map[string]int
	conversions       []pathConversion
}

type pathConversion struct {
	validation        *Validation
	scalarType        styleScalarType
	itemValidation    *Validation
	dynamicType       styleScalarType
	dynamicValidation *Validation
	properties        []pathProperty
	propertyByName    map[string]int
}

type pathProperty struct {
	name       string
	scalarType styleScalarType
	validation *Validation
}

func compilePathDecoder(
	operationID string,
	source oas.Source,
	compiler *schemaCompiler,
) (*PathDecoder, error) {
	parameters := make([]pathParameter, 0, len(source.PathParameters))
	for _, located := range source.PathParameters {
		parameter, err := compilePathParameter(located, compiler)
		if err != nil {
			return nil, fmt.Errorf("operationId %q compile path parameter: %w", operationID, err)
		}

		parameters = append(parameters, parameter)
	}

	decoder, err := newPathDecoder(operationID, source.PathTemplate, parameters)
	if err != nil {
		return nil, fmt.Errorf("operationId %q compile path decoder: %w", operationID, err)
	}

	return decoder, nil
}

func compilePathParameter(located oas.LocatedSchema, compiler *schemaCompiler) (pathParameter, error) {
	members, err := parameterMembers(located)
	if err != nil {
		return pathParameter{}, err
	}

	name, hasContent, err := pathParameterDeclaration(located.Pointer, members)
	if err != nil {
		return pathParameter{}, err
	}

	if hasContent {
		return compileJSONPathParameter(name, located, members, compiler)
	}

	return compileSchemaPathParameter(name, located, members, compiler)
}

func pathParameterDeclaration(
	pointer string,
	members map[string]json.RawMessage,
) (string, bool, error) {
	name, err := decodeString(members["name"], "name")
	if err != nil || name == "" || strings.ContainsRune(name, '/') {
		return "", false, fmt.Errorf("path parameter at %s name is invalid", pointer)
	}

	required, err := decodeOptionalBoolean(members, "required")
	if err != nil || !required {
		return "", false, fmt.Errorf("path parameter %q required must be true", name)
	}

	_, hasSchema := members["schema"]

	_, hasContent := members["content"]
	if hasSchema == hasContent {
		return "", false, fmt.Errorf("path parameter %q must contain exactly one of schema or content", name)
	}

	return name, hasContent, nil
}

func compileSchemaPathParameter(
	name string,
	located oas.LocatedSchema,
	members map[string]json.RawMessage,
	compiler *schemaCompiler,
) (pathParameter, error) {
	schema := locatedRawChild(located, members["schema"], "schema")

	validation, err := compiler.compile(schema)
	if err != nil {
		return pathParameter{}, fmt.Errorf("path parameter %q schema: %w", name, err)
	}

	if policyErr := rejectPathOnlyFields(name, members); policyErr != nil {
		return pathParameter{}, policyErr
	}

	if len(validation.Validate(json.RawMessage("null"))) == 0 {
		return pathParameter{}, fmt.Errorf("schema-style path parameter %q accepts JSON null", name)
	}

	if binaryErr := rejectPathBinary(validation); binaryErr != nil {
		return pathParameter{}, fmt.Errorf("path parameter %q: %w", name, binaryErr)
	}

	styleOffset, explode, err := compilePathStyle(name, members)
	if err != nil {
		return pathParameter{}, err
	}

	return compileSchemaPathMetadata(name, styleOffset, explode, validation)
}

func compilePathStyle(
	name string,
	members map[string]json.RawMessage,
) (pathWireKind, bool, error) {
	style := "simple"

	if raw, ok := members["style"]; ok {
		parsed, err := decodeString(raw, "style")
		if err != nil {
			return 0, false, fmt.Errorf("path parameter %q style: %w", name, err)
		}

		style = parsed
	}

	explode := false

	if raw, ok := members["explode"]; ok {
		parsed, err := decodeBoolean(raw, "explode")
		if err != nil {
			return 0, false, fmt.Errorf("path parameter %q explode: %w", name, err)
		}

		explode = parsed
	}

	switch style {
	case "simple":
		return pathWireSimplePrimitive, explode, nil
	case "label":
		return pathWireLabelPrimitive, explode, nil
	case "matrix":
		return pathWireMatrixPrimitive, explode, nil
	default:
		return 0, false, fmt.Errorf("path parameter %q style %q is unsupported", name, style)
	}
}

func compileSchemaPathMetadata(
	name string,
	styleOffset pathWireKind,
	explode bool,
	validation *Validation,
) (pathParameter, error) {
	parameter := pathParameter{name: name, explode: explode, validation: validation}

	var err error

	typeName, err := compiledStyleType(validation)
	if err != nil {
		return pathParameter{}, fmt.Errorf("path parameter %q: %w", name, err)
	}

	switch typeName {
	case "array":
		parameter.wire = styleOffset + pathWireKind(pathShapeArray)

		parameter.scalarType, err = compiledArrayScalarType(validation)
		if err != nil {
			return pathParameter{}, fmt.Errorf("path parameter %q array items: %w", name, err)
		}
	case "object":
		parameter.wire = styleOffset + pathWireKind(pathShapeObject)

		parameter.properties, parameter.propertyByName, err = compiledPathProperties(validation)
		if err != nil {
			return pathParameter{}, fmt.Errorf("path parameter %q object properties: %w", name, err)
		}

		parameter.dynamicType, err = compiledPathScalarType(compiledAdditionalProperties(validation)...)
		if err != nil {
			return pathParameter{}, fmt.Errorf("path parameter %q additionalProperties: %w", name, err)
		}

	default:
		parameter.wire = styleOffset + pathWireKind(pathShapePrimitive)

		parameter.scalarType, err = compiledPathScalarType(validation)
		if err != nil {
			return pathParameter{}, fmt.Errorf("path parameter %q: %w", name, err)
		}
	}

	attachPathConversionValidations(&parameter)

	if err := preparePathConversions(&parameter); err != nil {
		return pathParameter{}, fmt.Errorf("path parameter %q: %w", name, err)
	}

	return parameter, nil
}

func compileJSONPathParameter(
	name string,
	located oas.LocatedSchema,
	members map[string]json.RawMessage,
	compiler *schemaCompiler,
) (pathParameter, error) {
	schema, err := pathJSONContentSchema(name, located, members["content"])
	if err != nil {
		return pathParameter{}, err
	}

	validation, err := compiler.compile(schema)
	if err != nil {
		return pathParameter{}, fmt.Errorf("path parameter %q schema: %w", name, err)
	}

	if policyErr := rejectPathOnlyFields(name, members); policyErr != nil {
		return pathParameter{}, policyErr
	}

	for _, field := range []string{"style", "explode"} {
		if _, present := members[field]; present {
			return pathParameter{}, fmt.Errorf("path parameter %q content cannot be combined with %s", name, field)
		}
	}

	if binaryErr := rejectPathBinary(validation); binaryErr != nil {
		return pathParameter{}, fmt.Errorf("path parameter %q: %w", name, binaryErr)
	}

	return pathParameter{name: name, wire: pathWireJSONContent, validation: validation}, nil
}

func rejectPathOnlyFields(name string, members map[string]json.RawMessage) error {
	for _, field := range []string{"allowEmptyValue", "allowReserved"} {
		if _, present := members[field]; present {
			return fmt.Errorf("path parameter %q cannot declare %s", name, field)
		}
	}

	return nil
}

func pathJSONContentSchema(
	name string,
	located oas.LocatedSchema,
	rawContent json.RawMessage,
) (oas.LocatedSchema, error) {
	var content map[string]json.RawMessage
	if err := json.Unmarshal(rawContent, &content); err != nil || content == nil || len(content) != 1 {
		return oas.LocatedSchema{}, fmt.Errorf("path parameter %q content must contain exactly one media type", name)
	}

	mediaTypeName, rawMediaType, err := pathJSONMediaType(name, content)
	if err != nil {
		return oas.LocatedSchema{}, err
	}

	var mediaType map[string]json.RawMessage
	if unmarshalErr := json.Unmarshal(rawMediaType, &mediaType); unmarshalErr != nil || mediaType == nil {
		return oas.LocatedSchema{}, fmt.Errorf("path parameter %q Media Type Object must be an object", name)
	}

	rawSchema, ok := mediaType["schema"]
	if !ok {
		rawSchema = json.RawMessage(`{}`)
	}

	return locatedRawChild(located, rawSchema, "content", mediaTypeName, "schema"), nil
}

func pathJSONMediaType(
	name string,
	content map[string]json.RawMessage,
) (string, json.RawMessage, error) {
	var (
		mediaTypeName string
		rawMediaType  json.RawMessage
	)
	for mediaTypeName, rawMediaType = range content {
		break
	}

	parsedMediaType, _, err := mime.ParseMediaType(mediaTypeName)
	if err != nil || !strictMediaTypeParameters(mediaTypeName) || parsedMediaType != "application/json" {
		return "", nil, fmt.Errorf(
			"path parameter %q content only application/json is supported, got %q",
			name,
			mediaTypeName,
		)
	}

	return mediaTypeName, rawMediaType, nil
}

//nolint:cyclop // The recursive walk checks every compositional and child schema edge.
func rejectPathBinary(validation *Validation) error {
	if validation.StringValidation.Format == "binary" {
		return fmt.Errorf(
			"schema at %s format %q is unsupported for path parameters",
			validation.SchemaPointer,
			validation.StringValidation.Format,
		)
	}

	if validation.ArrayValidation.Items != nil {
		if err := rejectPathBinary(validation.ArrayValidation.Items); err != nil {
			return err
		}
	}

	for _, property := range validation.ObjectValidation.Properties {
		if err := rejectPathBinary(property.Validation); err != nil {
			return err
		}
	}

	if validation.ObjectValidation.AdditionalPropertiesValidation != nil {
		if err := rejectPathBinary(validation.ObjectValidation.AdditionalPropertiesValidation); err != nil {
			return err
		}
	}

	for _, child := range validation.AllOfValidations {
		if err := rejectPathBinary(child); err != nil {
			return err
		}
	}

	for _, child := range validation.AnyOfValidations {
		if err := rejectPathBinary(child); err != nil {
			return err
		}
	}

	return nil
}

func compiledPathScalarType(validations ...*Validation) (styleScalarType, error) {
	typeName, err := compiledScalarType(validations...)
	if err != nil {
		return "", err
	}

	return styleScalarType(typeName), nil
}

func compiledScalarType(validations ...*Validation) (string, error) {
	if len(validations) == 0 {
		return "string", nil
	}

	alternatives := conversionAlternatives(conjunctiveValidation(validations...))

	types := make([]string, 0, len(alternatives))
	for _, alternative := range alternatives {
		typeName := compiledValidationType(alternative)
		if typeName == "" {
			typeName = "string"
		}

		if !isScalarType(typeName) {
			return "", fmt.Errorf(
				"style scalar slot at %s must have a primitive type; unsupported compiled type %q",
				alternative.SchemaPointer,
				typeName,
			)
		}

		types = append(types, typeName)
	}

	typeName := intersectQuerySchemaTypes(types)
	if typeName == "" {
		return "string", nil
	}

	return typeName, nil
}

func compiledArrayScalarType(validation *Validation) (styleScalarType, error) {
	items := compiledArrayItems(validation)
	if len(items) != 0 {
		return compiledPathScalarType(items...)
	}

	enumItems := make([]jsonvalue.Value, 0)
	collectEnumArrayItems(validation, &enumItems)

	if len(enumItems) == 0 {
		return "string", nil
	}

	typeName := homogeneousEnumType(enumItems)
	if !isScalarType(typeName) {
		return "", fmt.Errorf("enum array items do not have one primitive type")
	}

	return styleScalarType(typeName), nil
}

func collectEnumArrayItems(validation *Validation, items *[]jsonvalue.Value) {
	for _, value := range validation.EnumValidation.ExactValues {
		if value.Kind == jsonvalue.KindArray {
			*items = append(*items, value.Array...)
		}
	}

	for _, child := range validation.AllOfValidations {
		collectEnumArrayItems(child, items)
	}
}

func compiledArrayItems(validation *Validation) []*Validation {
	items := make([]*Validation, 0)
	collectCompiledArrayItems(validation, &items)

	return items
}

func collectCompiledArrayItems(validation *Validation, items *[]*Validation) {
	if validation.ArrayValidation.Items != nil {
		*items = append(*items, validation.ArrayValidation.Items)
	}

	for _, child := range validation.AllOfValidations {
		collectCompiledArrayItems(child, items)
	}
}

func compiledPathProperties(validation *Validation) ([]pathProperty, map[string]int, error) {
	byName := make(map[string][]*Validation)
	collectCompiledObjectProperties(validation, byName)

	valuesByName := make(map[string][]jsonvalue.Value)
	collectEnumObjectProperties(validation, valuesByName)

	for name := range valuesByName {
		if _, declared := byName[name]; !declared {
			byName[name] = nil
		}
	}

	names := slices.Sorted(maps.Keys(byName))

	properties := make([]pathProperty, 0, len(names))

	propertyByName := make(map[string]int, len(names))
	for _, name := range names {
		if name == "" {
			return nil, nil, fmt.Errorf("declared style-object property name must not be empty")
		}

		typeName, err := compiledPathPropertyScalarType(byName[name], valuesByName[name])
		if err != nil {
			return nil, nil, fmt.Errorf("property %q: %w", name, err)
		}

		propertyByName[name] = len(properties)
		properties = append(properties, pathProperty{
			name: name, scalarType: typeName, validation: conjunctiveValidation(byName[name]...),
		})
	}

	return properties, propertyByName, nil
}

func compiledPathPropertyScalarType(
	validations []*Validation,
	enumValues []jsonvalue.Value,
) (styleScalarType, error) {
	for _, validation := range validations {
		if containsAnyOf(validation) {
			return compiledPathScalarType(validations...)
		}
	}

	types := make([]string, 0)
	for _, validation := range validations {
		collectCompiledValidationTypes(validation, &types)
	}

	if len(types) == 0 && len(enumValues) != 0 {
		types = append(types, homogeneousEnumType(enumValues))
	}

	typeName := intersectQuerySchemaTypes(types)
	if !isScalarType(typeName) {
		return "", fmt.Errorf("style scalar slot has unsupported compiled type %q", typeName)
	}

	return styleScalarType(typeName), nil
}

func collectEnumObjectProperties(validation *Validation, valuesByName map[string][]jsonvalue.Value) {
	for _, value := range validation.EnumValidation.ExactValues {
		if value.Kind != jsonvalue.KindObject {
			continue
		}

		for _, member := range value.Object {
			valuesByName[member.Name] = append(valuesByName[member.Name], member.Value)
		}
	}

	for _, child := range validation.AllOfValidations {
		collectEnumObjectProperties(child, valuesByName)
	}
}

func collectCompiledObjectProperties(validation *Validation, byName map[string][]*Validation) {
	for _, property := range validation.ObjectValidation.Properties {
		byName[property.Name] = append(byName[property.Name], property.Validation)
	}

	for _, child := range validation.AllOfValidations {
		collectCompiledObjectProperties(child, byName)
	}
}

func compiledAdditionalProperties(validation *Validation) []*Validation {
	additional := make([]*Validation, 0)
	collectCompiledAdditionalProperties(validation, &additional)

	return additional
}

func collectCompiledAdditionalProperties(validation *Validation, additional *[]*Validation) {
	if validation.ObjectValidation.AdditionalPropertiesValidation != nil {
		*additional = append(*additional, validation.ObjectValidation.AdditionalPropertiesValidation)
	}

	for _, child := range validation.AllOfValidations {
		collectCompiledAdditionalProperties(child, additional)
	}
}

func compiledValidationType(validations ...*Validation) string {
	types := make([]string, 0)
	for _, validation := range validations {
		collectCompiledValidationTypes(validation, &types)
	}

	return intersectQuerySchemaTypes(types)
}

func conjunctiveValidation(validations ...*Validation) *Validation {
	if len(validations) == 0 {
		return nil
	}

	if len(validations) == 1 {
		return validations[0]
	}

	// The wrapper is only a conjunction carrier: children enforce object policy.
	// Its pointer is borrowed from the first child solely for wrapper diagnostics.
	return &Validation{
		SchemaPointer:    validations[0].SchemaPointer,
		AllOfValidations: append([]*Validation(nil), validations...),
		ObjectValidation: ObjectValidation{AdditionalPropertiesAllowed: true},
	}
}

//nolint:cyclop // OpenAPI style types require ordered inference across all AnyOf candidates.
func compiledStyleType(validation *Validation) (string, error) {
	alternatives := conversionAlternatives(validation)
	types := make([]string, 0, len(alternatives))
	shape := ""

	for _, alternative := range alternatives {
		typeName := compiledValidationType(alternative)
		if typeName == "" {
			typeName = "string"
		}

		candidateShape := "primitive"
		if typeName == "array" || typeName == "object" {
			candidateShape = typeName
		}

		if shape != "" && candidateShape != shape {
			return "", fmt.Errorf(
				"schema at %s has unrepresentable anyOf wire shape %q beside %q",
				alternative.SchemaPointer, candidateShape, shape,
			)
		}

		if candidateShape == "array" {
			if _, err := compiledArrayScalarType(alternative); err != nil {
				return "", err
			}
		}

		shape = candidateShape

		types = append(types, typeName)
	}

	if shape == "array" || shape == "object" {
		return shape, nil
	}

	typeName := intersectQuerySchemaTypes(types)
	if typeName == "" {
		return "string", nil
	}

	return typeName, nil
}

func conversionAlternatives(validation *Validation) []*Validation {
	if validation == nil || !containsAnyOf(validation) {
		return []*Validation{validation}
	}

	local := *validation
	local.AllOfValidations = nil
	local.AnyOfValidations = nil
	base := []*Validation{&local}

	for _, child := range validation.AllOfValidations {
		childAlternatives := conversionAlternatives(child)

		next := make([]*Validation, 0, len(base)*len(childAlternatives))
		for _, current := range base {
			for _, childAlternative := range childAlternatives {
				next = append(next, conjunctiveValidation(current, childAlternative))
			}
		}

		base = next
	}

	if len(validation.AnyOfValidations) == 0 {
		return base
	}

	result := make([]*Validation, 0)

	for _, branch := range validation.AnyOfValidations {
		for _, branchAlternative := range conversionAlternatives(branch) {
			for _, current := range base {
				combined := conjunctiveValidation(current, branchAlternative)
				combined.SchemaPointer = branchAlternative.SchemaPointer
				result = append(result, combined)
			}
		}
	}

	return result
}

func containsAnyOf(validation *Validation) bool {
	if validation == nil {
		return false
	}

	if len(validation.AnyOfValidations) != 0 {
		return true
	}

	for _, child := range validation.AllOfValidations {
		if containsAnyOf(child) {
			return true
		}
	}

	return false
}

func collectCompiledValidationTypes(validation *Validation, types *[]string) {
	if validation.KindValidation.Type != "" {
		*types = append(*types, validation.KindValidation.Type)
	}

	if enumType := homogeneousEnumType(validation.EnumValidation.ExactValues); enumType != "" {
		*types = append(*types, enumType)
	}

	for _, child := range validation.AllOfValidations {
		collectCompiledValidationTypes(child, types)
	}
}

func homogeneousEnumType(values []jsonvalue.Value) string {
	if len(values) == 0 {
		return ""
	}

	typeName := enumValueType(values[0])
	if typeName == "" {
		return ""
	}

	for _, value := range values[1:] {
		valueType := enumValueType(value)
		if typeName == "integer" && valueType == "number" {
			typeName = "number"

			continue
		}

		if typeName == "number" && valueType == "integer" {
			continue
		}

		if valueType != typeName {
			return ""
		}
	}

	return typeName
}

func enumValueType(value jsonvalue.Value) string {
	switch value.Kind {
	case jsonvalue.KindBoolean:
		return "boolean"
	case jsonvalue.KindNumber:
		if value.Number.IsInteger() {
			return "integer"
		}

		return "number"
	case jsonvalue.KindString:
		return "string"
	case jsonvalue.KindArray:
		return "array"
	case jsonvalue.KindObject:
		return "object"
	case jsonvalue.KindNull:
		return ""
	}

	return ""
}

// PathDecoder decodes and validates one operation's path parameters.
// It is immutable after construction and safe for concurrent use.
type PathDecoder struct {
	operationID  string
	pathTemplate string
	segments     []*regexp.Regexp
	parameters   []pathParameter
	validation   *Validation
}

// PathDecoderDefinition is the generation-only compiled form of a PathDecoder.
// Validation pointers are shared with the decoder and must not be mutated while it is in use.
type PathDecoderDefinition struct {
	OperationID  string
	PathTemplate string
	Parameters   []PathParameterDefinition
}

// PathParameterDefinition is one generation-only compiled path parameter.
type PathParameterDefinition struct {
	Name        string
	Wire        uint8
	Explode     bool
	Validation  *Validation
	ScalarType  string
	DynamicType string
	Properties  []PathPropertyDefinition
}

// PathPropertyDefinition is one generation-only compiled style-object property.
type PathPropertyDefinition struct {
	Name       string
	ScalarType string
}

// Definition returns the generation-only compiled form of decoder.
func (decoder *PathDecoder) Definition() PathDecoderDefinition {
	definition := PathDecoderDefinition{
		OperationID: decoder.operationID, PathTemplate: decoder.pathTemplate,
		Parameters: make([]PathParameterDefinition, len(decoder.parameters)),
	}
	for index, parameter := range decoder.parameters {
		definition.Parameters[index] = PathParameterDefinition{
			Name: parameter.name, Wire: uint8(parameter.wire), Explode: parameter.explode,
			Validation: parameter.validation, ScalarType: string(parameter.scalarType),
			DynamicType: string(parameter.dynamicType),
			Properties:  make([]PathPropertyDefinition, len(parameter.properties)),
		}
		for propertyIndex, property := range parameter.properties {
			definition.Parameters[index].Properties[propertyIndex] = PathPropertyDefinition{
				Name: property.name, ScalarType: string(property.scalarType),
			}
		}
	}

	return definition
}

// NewPathDecoderFromGenerated restores a generator-produced path decoder definition.
//
//nolint:funcorder // Definition transport types and Definition stay together above the restoring constructor.
func NewPathDecoderFromGenerated(definition PathDecoderDefinition) (*PathDecoder, error) {
	if _, err := oas.RequestValidationName(definition.OperationID); err != nil {
		return nil, fmt.Errorf("generated path decoder operation ID: %w", err)
	}

	parameters := make([]pathParameter, len(definition.Parameters))
	for index, compiled := range definition.Parameters {
		parameter, err := pathParameterFromGenerated(compiled)
		if err != nil {
			return nil, err
		}

		parameters[index] = parameter
	}

	return newPathDecoder(definition.OperationID, definition.PathTemplate, parameters)
}

func pathParameterFromGenerated(compiled PathParameterDefinition) (pathParameter, error) {
	if compiled.Name == "" || strings.ContainsRune(compiled.Name, '/') || compiled.Validation == nil {
		return pathParameter{}, fmt.Errorf("generated path parameter %q is invalid", compiled.Name)
	}

	wire := pathWireKind(compiled.Wire)
	if wire > pathWireJSONContent {
		return pathParameter{}, fmt.Errorf("generated path parameter %q has invalid wire %d", compiled.Name, compiled.Wire)
	}

	parameter := pathParameter{
		name: compiled.Name, wire: wire, explode: compiled.Explode, validation: compiled.Validation,
		scalarType: styleScalarType(compiled.ScalarType), dynamicType: styleScalarType(compiled.DynamicType),
		properties:     make([]pathProperty, len(compiled.Properties)),
		propertyByName: make(map[string]int, len(compiled.Properties)),
	}
	for index, property := range compiled.Properties {
		if _, duplicate := parameter.propertyByName[property.Name]; duplicate {
			return pathParameter{}, fmt.Errorf("generated path parameter %q has duplicate property %q", compiled.Name, property.Name)
		}

		parameter.properties[index] = pathProperty{name: property.Name, scalarType: styleScalarType(property.ScalarType)}
		parameter.propertyByName[property.Name] = index
	}

	attachPathConversionValidations(&parameter)

	if err := validatePathParameterMetadata(parameter); err != nil {
		return pathParameter{}, fmt.Errorf("generated path parameter %q: %w", compiled.Name, err)
	}

	if err := preparePathConversions(&parameter); err != nil {
		return pathParameter{}, fmt.Errorf("generated path parameter %q: %w", compiled.Name, err)
	}

	return parameter, nil
}

func attachPathConversionValidations(parameter *pathParameter) {
	if parameter == nil || parameter.validation == nil {
		return
	}

	parameter.itemValidation = conjunctiveValidation(compiledArrayItems(parameter.validation)...)
	parameter.dynamicValidation = conjunctiveValidation(compiledAdditionalProperties(parameter.validation)...)

	byName := make(map[string][]*Validation)
	collectCompiledObjectProperties(parameter.validation, byName)

	for index := range parameter.properties {
		parameter.properties[index].validation = conjunctiveValidation(byName[parameter.properties[index].name]...)
	}
}

//nolint:cyclop // Each AnyOf candidate has separate primitive, array, and object metadata.
func preparePathConversions(parameter *pathParameter) error {
	if parameter == nil || parameter.validation == nil || parameter.wire == pathWireJSONContent {
		return nil
	}

	alternatives := conversionAlternatives(parameter.validation)
	if !containsAnyOf(parameter.validation) {
		parameter.conversions = []pathConversion{{
			validation: parameter.validation, scalarType: parameter.scalarType,
			itemValidation: parameter.itemValidation, dynamicType: parameter.dynamicType,
			dynamicValidation: parameter.dynamicValidation, properties: parameter.properties,
			propertyByName: parameter.propertyByName,
		}}

		return nil
	}

	parameter.conversions = make([]pathConversion, 0, len(alternatives))
	shape := pathShape(parameter.wire % pathWireKind(pathShapeCount))

	for _, validation := range alternatives {
		conversion := pathConversion{validation: validation, scalarType: parameter.scalarType}

		switch shape {
		case pathShapePrimitive:
			typeName, err := compiledPathScalarType(validation)
			if err != nil {
				return err
			}

			conversion.scalarType = typeName
		case pathShapeArray:
			typeName, err := compiledArrayScalarType(validation)
			if err != nil {
				return err
			}

			conversion.scalarType = typeName
			conversion.itemValidation = conjunctiveValidation(compiledArrayItems(validation)...)
		case pathShapeObject:
			properties, byName, err := compiledPathProperties(validation)
			if err != nil {
				return err
			}

			conversion.properties = properties
			conversion.propertyByName = byName

			conversion.dynamicType, err = compiledPathScalarType(compiledAdditionalProperties(validation)...)
			if err != nil {
				return fmt.Errorf("additionalProperties: %w", err)
			}

			conversion.dynamicValidation = conjunctiveValidation(compiledAdditionalProperties(validation)...)
		}

		parameter.conversions = append(parameter.conversions, conversion)
	}

	if shape == pathShapeObject {
		mergePathObjectConversions(parameter)
	}

	return nil
}

func mergePathObjectConversions(parameter *pathParameter) {
	properties := make(map[string]pathProperty)
	parameter.dynamicType = ""
	parameter.dynamicValidation = nil

	for _, conversion := range parameter.conversions {
		if parameter.dynamicType == "" && conversion.dynamicType != "" {
			parameter.dynamicType = conversion.dynamicType
			parameter.dynamicValidation = conversion.dynamicValidation
		}

		for _, property := range conversion.properties {
			if _, ok := properties[property.name]; !ok {
				properties[property.name] = property
			}
		}
	}

	parameter.properties = make([]pathProperty, 0, len(properties))

	parameter.propertyByName = make(map[string]int, len(properties))
	for _, name := range slices.Sorted(maps.Keys(properties)) {
		parameter.propertyByName[name] = len(parameter.properties)
		parameter.properties = append(parameter.properties, properties[name])
	}
}

//nolint:cyclop,gocognit // The finite wire/shape metadata table is clearest at one invariant boundary.
func validatePathParameterMetadata(parameter pathParameter) error {
	if parameter.name == "" || strings.ContainsRune(parameter.name, '/') || parameter.validation == nil {
		return errors.New("name or validation is invalid")
	}

	if parameter.wire > pathWireJSONContent {
		return fmt.Errorf("wire %d is invalid", parameter.wire)
	}

	if parameter.wire == pathWireJSONContent {
		if parameter.explode || parameter.scalarType != "" || parameter.dynamicType != "" || len(parameter.properties) != 0 {
			return fmt.Errorf("JSON content wire metadata is invalid")
		}

		return nil
	}

	switch pathShape(parameter.wire % pathWireKind(pathShapeCount)) {
	case pathShapePrimitive, pathShapeArray:
		if !isPathScalarType(parameter.scalarType) || parameter.dynamicType != "" || len(parameter.properties) != 0 {
			return fmt.Errorf("primitive or array wire metadata is invalid")
		}
	case pathShapeObject:
		if parameter.scalarType != "" || parameter.dynamicType != "" && !isPathScalarType(parameter.dynamicType) {
			return fmt.Errorf("object wire metadata is invalid")
		}

		seen := make(map[string]struct{}, len(parameter.properties))
		for index, property := range parameter.properties {
			if property.name == "" || !isPathScalarType(property.scalarType) {
				return fmt.Errorf("object property %q metadata is invalid", property.name)
			}

			if _, duplicate := seen[property.name]; duplicate {
				return fmt.Errorf("object property %q is duplicated", property.name)
			}

			seen[property.name] = struct{}{}

			if mapped, ok := parameter.propertyByName[property.name]; !ok || mapped != index {
				return fmt.Errorf("object property %q lookup metadata is invalid", property.name)
			}
		}

		if len(parameter.propertyByName) != len(parameter.properties) {
			return errors.New("object property lookup metadata is invalid")
		}
	}

	if len(parameter.validation.Validate(json.RawMessage("null"))) == 0 {
		return fmt.Errorf("schema-style validation accepts JSON null")
	}

	return nil
}

func isPathScalarType(typeName styleScalarType) bool {
	return typeName == "string" || typeName == "boolean" || typeName == "integer" || typeName == "number"
}

//nolint:cyclop // Constructor validation, declaration ordering, and segment compilation form one fixed pipeline.
func newPathDecoder(operationID string, pathTemplate string, parameters []pathParameter) (*PathDecoder, error) {
	if _, err := oas.RequestValidationName(operationID); err != nil {
		return nil, fmt.Errorf("path decoder operation ID: %w", err)
	}

	byName := make(map[string]pathParameter, len(parameters))
	for _, parameter := range parameters {
		if err := validatePathParameterMetadata(parameter); err != nil {
			return nil, fmt.Errorf("path parameter %q metadata: %w", parameter.name, err)
		}

		parameter = clonePathParameter(parameter)
		if _, duplicate := byName[parameter.name]; duplicate {
			return nil, fmt.Errorf("path parameter %q is duplicated", parameter.name)
		}

		byName[parameter.name] = parameter
	}

	_, expressions, err := oas.ParsePathTemplate(pathTemplate)
	if err != nil {
		return nil, err
	}

	ordered := make([]pathParameter, 0, len(expressions))
	for _, expression := range expressions {
		parameter, ok := byName[expression]
		if !ok {
			return nil, fmt.Errorf("path template %q expression %q has no parameter declaration", pathTemplate, expression)
		}

		ordered = append(ordered, parameter)

		delete(byName, expression)
	}

	for _, parameter := range parameters {
		if _, unused := byName[parameter.name]; unused {
			return nil, fmt.Errorf("path parameter %q has no expression in path template %q", parameter.name, pathTemplate)
		}
	}

	segments := make([]*regexp.Regexp, 0, strings.Count(pathTemplate, "/")+1)
	for _, segment := range strings.Split(pathTemplate, "/") {
		matcher, compileErr := compilePathSegment(segment)
		if compileErr != nil {
			return nil, compileErr
		}

		segments = append(segments, matcher)
	}

	return &PathDecoder{
		operationID: strings.Clone(operationID), pathTemplate: strings.Clone(pathTemplate), segments: segments,
		parameters: ordered, validation: syntheticPathValidation(operationID, ordered),
	}, nil
}

func clonePathParameter(parameter pathParameter) pathParameter {
	cloned := parameter
	cloned.name = strings.Clone(parameter.name)
	cloned.scalarType = styleScalarType(strings.Clone(string(parameter.scalarType)))
	cloned.dynamicType = styleScalarType(strings.Clone(string(parameter.dynamicType)))

	cloned.properties = append([]pathProperty(nil), parameter.properties...)
	for index := range cloned.properties {
		cloned.properties[index].name = strings.Clone(cloned.properties[index].name)
		cloned.properties[index].scalarType = styleScalarType(strings.Clone(string(cloned.properties[index].scalarType)))
	}

	cloned.propertyByName = make(map[string]int, len(parameter.propertyByName))
	for name, index := range parameter.propertyByName {
		cloned.propertyByName[strings.Clone(name)] = index
	}

	return cloned
}

func compilePathSegment(segment string) (*regexp.Regexp, error) {
	pattern := "^"

	for index := 0; index < len(segment); {
		opening := strings.IndexByte(segment[index:], '{')
		if opening == -1 {
			pattern += regexp.QuoteMeta(segment[index:])

			break
		}

		opening += index
		pattern += regexp.QuoteMeta(segment[index:opening])
		closing := strings.IndexByte(segment[opening+1:], '}') + opening + 1

		pattern += `((?:%[0-9A-Fa-f]{2}|[^%/])*?)`

		index = closing + 1
	}

	pattern += "$"

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile path segment %q: %w", segment, err)
	}

	return compiled, nil
}

func syntheticPathValidation(operationID string, parameters []pathParameter) *Validation {
	root := &Validation{
		SchemaPointer:  fmt.Sprintf("#/operations/%s/path", escapePointerToken(operationID)),
		KindValidation: KindValidation{Type: "object"},
	}
	root.ObjectValidation.Properties = make([]PropertyValidation, len(parameters))

	root.ObjectValidation.Required = make([]string, len(parameters))
	for index, parameter := range parameters {
		root.ObjectValidation.Properties[index] = PropertyValidation{Name: parameter.name, Validation: parameter.validation}
		root.ObjectValidation.Required[index] = parameter.name
	}

	sort.Slice(root.ObjectValidation.Properties, func(left int, right int) bool {
		return root.ObjectValidation.Properties[left].Name < root.ObjectValidation.Properties[right].Name
	})
	sort.Strings(root.ObjectValidation.Required)

	return root
}
