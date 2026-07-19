//nolint:godoclint,lll // Private query compiler names and diagnostics are local implementation details.
package validation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
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
	name           string
	wire           wireKind
	separator      string
	required       bool
	allowEmpty     bool
	validation     *Validation
	defaultValue   jsontext.Value
	scalarType     string
	dynamicType    string
	properties     []queryProperty
	propertyByName map[string]int
}

type queryProperty struct {
	name       string
	scalarType string
	array      bool
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

		var content map[string]json.RawMessage
		if unmarshalErr := json.Unmarshal(members["content"], &content); unmarshalErr != nil || len(content) != 1 {
			return queryParameter{}, fmt.Errorf("parameter %q at %s content must contain exactly one media type", name, located.Pointer)
		}

		if _, ok := content["application/json"]; !ok {
			return queryParameter{}, fmt.Errorf("parameter %q at %s only application/json content is supported", name, located.Pointer)
		}

		var mediaType map[string]json.RawMessage
		if unmarshalErr := json.Unmarshal(content["application/json"], &mediaType); unmarshalErr != nil {
			return queryParameter{}, fmt.Errorf(
				"parameter %q at %s application/json schema: %w", name, located.Pointer, unmarshalErr,
			)
		}

		if mediaType["schema"] == nil {
			return queryParameter{}, fmt.Errorf("parameter %q at %s application/json schema does not exist", name, located.Pointer)
		}

		schema = locatedRawChild(located, mediaType["schema"], "content", "application/json", "schema")
		parameter.wire = wireJSONContent
	} else {
		schema = locatedRawChild(located, members["schema"], "schema")
	}

	resolved, directMembers, directType, err := directSchemaType(compiler.source, schema)
	if err != nil {
		return queryParameter{}, fmt.Errorf("parameter %q: %w", name, err)
	}

	if raw, ok := directMembers["default"]; ok {
		parameter.defaultValue = append(jsontext.Value(nil), raw...)
	}

	if hasContent {
		parameter.validation, err = compiler.compile(schema)
		if err != nil {
			return queryParameter{}, fmt.Errorf("parameter %q schema: %w", name, err)
		}

		return parameter, nil
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
		item, childErr := compiler.source.Child(resolved, "items")
		if childErr != nil {
			return queryParameter{}, fmt.Errorf("parameter %q array items: %w", name, childErr)
		}

		_, _, parameter.scalarType, childErr = directSchemaType(compiler.source, item)
		if childErr != nil || !isScalarType(parameter.scalarType) {
			return queryParameter{}, fmt.Errorf("parameter %q style-based array items must have a direct primitive type", name)
		}

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
		parameter.properties, parameter.propertyByName, err = compileQueryProperties(resolved, compiler.source, style == "deepObject")
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

		parameter.dynamicType, err = queryAdditionalPropertiesType(resolved, directMembers, compiler.source)
		if err != nil {
			return queryParameter{}, fmt.Errorf("parameter %q additionalProperties: %w", name, err)
		}
	default:
		return queryParameter{}, fmt.Errorf("parameter %q has unsupported direct type %q", name, directType)
	}

	parameter.validation, err = compiler.compile(schema)
	if err != nil {
		return queryParameter{}, fmt.Errorf("parameter %q schema: %w", name, err)
	}

	return parameter, nil
}

func queryAdditionalPropertiesType(
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
	source oas.Source,
) (string, error) {
	raw, ok := members["additionalProperties"]
	if !ok || string(bytes.TrimSpace(raw)) == "true" {
		return "string", nil
	}

	if string(bytes.TrimSpace(raw)) == "false" {
		return "", nil
	}

	additional := locatedRawChild(schema, raw, "additionalProperties")

	types, err := explicitQuerySchemaTypes(source, additional, make(map[string]struct{}))
	if err != nil {
		return "", err
	}

	typeName := intersectQuerySchemaTypes(types)
	if typeName == "" {
		return "string", nil
	}

	if !isScalarType(typeName) {
		return "", fmt.Errorf("style-based dynamic properties cannot have satisfiable type %q", typeName)
	}

	return typeName, nil
}

//nolint:cyclop // Root and allOf type restrictions need one recursive schema traversal.
func explicitQuerySchemaTypes(
	source oas.Source,
	schema oas.LocatedSchema,
	active map[string]struct{},
) ([]string, error) {
	resolved, err := source.Resolve(schema)
	if err != nil {
		return nil, err
	}

	if _, cycle := active[resolved.Pointer]; cycle {
		return nil, fmt.Errorf("schema at %s has a recursive allOf reference", resolved.Pointer)
	}

	active[resolved.Pointer] = struct{}{}
	defer delete(active, resolved.Pointer)

	members, err := schemaMembers(resolved)
	if err != nil {
		return nil, err
	}

	types := make([]string, 0, 1)

	if raw, ok := members["type"]; ok {
		typeName, typeErr := decodeString(raw, "type")
		if typeErr != nil {
			return nil, fmt.Errorf("query schema at %s type: %w", resolved.Pointer, typeErr)
		}

		types = append(types, typeName)
	}

	rawAllOf, ok := members["allOf"]
	if !ok {
		return types, nil
	}

	var allOf []json.RawMessage
	if err := json.Unmarshal(rawAllOf, &allOf); err != nil || allOf == nil {
		return nil, fmt.Errorf("query schema at %s allOf must be an array", resolved.Pointer)
	}

	for index, raw := range allOf {
		childTypes, childErr := explicitQuerySchemaTypes(
			source,
			locatedRawChild(resolved, raw, "allOf", fmt.Sprint(index)),
			active,
		)
		if childErr != nil {
			return nil, childErr
		}

		types = append(types, childTypes...)
	}

	return types, nil
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

func directSchemaType(source oas.Source, schema oas.LocatedSchema) (
	oas.LocatedSchema,
	map[string]json.RawMessage,
	string,
	error,
) {
	resolved, err := source.Resolve(schema)
	if err != nil {
		return oas.LocatedSchema{}, nil, "", fmt.Errorf("resolve schema at %s: %w", schema.Pointer, err)
	}

	members, err := schemaMembers(resolved)
	if err != nil {
		return oas.LocatedSchema{}, nil, "", err
	}

	raw, ok := members["type"]
	if !ok {
		return oas.LocatedSchema{}, nil, "", fmt.Errorf("query schema at %s must have a direct type", resolved.Pointer)
	}

	typeName, err := decodeString(raw, "type")
	if err != nil {
		return oas.LocatedSchema{}, nil, "", fmt.Errorf("query schema at %s type: %w", resolved.Pointer, err)
	}

	return resolved, members, typeName, nil
}

//nolint:cyclop // Direct property and narrow array-extension checks form one compile decision.
func compileQueryProperties(
	schema oas.LocatedSchema,
	source oas.Source,
	allowPrimitiveArrays bool,
) ([]queryProperty, map[string]int, error) {
	var rawProperties map[string]json.RawMessage

	members, err := schemaMembers(schema)
	if err != nil {
		return nil, nil, err
	}

	if raw, ok := members["properties"]; ok {
		if err := json.Unmarshal(raw, &rawProperties); err != nil || rawProperties == nil {
			return nil, nil, fmt.Errorf("object properties at %s must be an object", schema.Pointer)
		}
	}

	properties := make([]queryProperty, 0, len(rawProperties))

	byName := make(map[string]int, len(rawProperties))
	for _, name := range slices.Sorted(maps.Keys(rawProperties)) {
		if allowPrimitiveArrays && strings.ContainsAny(name, "[]") {
			return nil, nil, fmt.Errorf(
				"deepObject property name %q has an unsupported non-reversible bracket wire boundary",
				name,
			)
		}

		child := locatedRawChild(schema, rawProperties[name], "properties", name)

		var childErr error

		resolved, _, typeName, childErr := directSchemaType(source, child)
		if childErr != nil {
			return nil, nil, childErr
		}

		property := queryProperty{name: name, scalarType: typeName}
		if typeName == "array" && allowPrimitiveArrays {
			items, itemsErr := source.Child(resolved, "items")
			if itemsErr != nil {
				return nil, nil, fmt.Errorf("deepObject array property %q items: %w", name, itemsErr)
			}

			_, _, property.scalarType, itemsErr = directSchemaType(source, items)
			if itemsErr != nil || !isScalarType(property.scalarType) {
				return nil, nil, fmt.Errorf("deepObject array property %q items must have a direct primitive type", name)
			}

			property.array = true
		} else if !isScalarType(typeName) {
			return nil, nil, fmt.Errorf("style-based object property %q must have a direct primitive type", name)
		}

		byName[name] = len(properties)
		properties = append(properties, property)
	}

	return properties, byName, nil
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
