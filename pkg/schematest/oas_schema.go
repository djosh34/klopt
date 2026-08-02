//nolint:cyclop,godoclint,lll,nilnil // Optional OAS fields use nil to represent absence in the private model.
package schematest

import (
	"errors"
	"fmt"
)

var schemaKinds = map[string]schemaKind{
	"boolean": schemaBoolean,
	"integer": schemaInteger,
	"number":  schemaNumber,
	"string":  schemaString,
	"array":   schemaArray,
	"object":  schemaObject,
}

var schemaKeywords = map[string]bool{
	"title":                true,
	"multipleOf":           true,
	"maximum":              true,
	"exclusiveMaximum":     true,
	"minimum":              true,
	"exclusiveMinimum":     true,
	"maxLength":            true,
	"minLength":            true,
	"pattern":              true,
	"maxItems":             true,
	"minItems":             true,
	"uniqueItems":          true,
	"maxProperties":        true,
	"minProperties":        true,
	"required":             true,
	"enum":                 true,
	"type":                 true,
	"allOf":                true,
	"oneOf":                true,
	"anyOf":                true,
	"not":                  true,
	"items":                true,
	"properties":           true,
	"additionalProperties": true,
	"description":          true,
	"format":               true,
	"default":              true,
	"nullable":             true,
	"discriminator":        true,
	"readOnly":             true,
	"writeOnly":            true,
	"xml":                  true,
	"externalDocs":         true,
	"example":              true,
	"deprecated":           true,
}

var schemaFormats = map[string]schemaFormat{
	"int32":     schemaFormatInt32,
	"int64":     schemaFormatInt64,
	"float":     schemaFormatFloat,
	"double":    schemaFormatDouble,
	"byte":      schemaFormatByte,
	"date":      schemaFormatDate,
	"date-time": schemaFormatDateTime,
	"email":     schemaFormatEmail,
	"ipv4":      schemaFormatIPv4,
	"uuid":      schemaFormatUUID,
	"uuidv4":    schemaFormatUUIDv4,
	"uuid-v4":   schemaFormatUUIDDashV4,
	"cidr":      schemaFormatCIDR,
	"ipv4-cidr": schemaFormatIPv4CIDR,
	"password":  schemaFormatPassword,
}

func (parser *oasParser) parseSchemaObject(
	node *schemaNode,
	object map[string]*jsonValue,
	pointer string,
) error {
	if err := validateSchemaKeywords(object, pointer); err != nil {
		return err
	}

	if err := validateInertAnnotations(object, pointer); err != nil {
		return err
	}

	if err := parseScalarSchemaFields(node, object, pointer); err != nil {
		return err
	}

	var err error
	if node.readOnly, err = optionalBoolean(object, "readOnly", pointer); err != nil {
		return err
	}

	if node.writeOnly, err = optionalBoolean(object, "writeOnly", pointer); err != nil {
		return err
	}

	if err := parseStringFields(node, object, pointer); err != nil {
		return err
	}

	if err := parser.parseArrayFields(node, object, pointer); err != nil {
		return err
	}

	if err := parser.parseObjectFields(node, object, pointer); err != nil {
		return err
	}

	return parser.parseCompositionFields(node, object, pointer)
}

func validateSchemaKeywords(object map[string]*jsonValue, pointer string) error {
	for _, name := range sortedObjectNames(object) {
		if name == "oneOf" || name == "not" || name == "discriminator" || name == "uniqueItems" {
			return fmt.Errorf("%s/%s: authored %s is outside the schematest profile", pointer, name, name)
		}

		if schemaKeywords[name] || len(name) > 2 && name[:2] == "x-" {
			continue
		}

		return fmt.Errorf("%s/%s: unknown OAS 3.0 Schema Object keyword", pointer, escapePointerToken(name))
	}

	return nil
}

func parseScalarSchemaFields(node *schemaNode, object map[string]*jsonValue, pointer string) error {
	if err := parseSchemaType(node, object, pointer); err != nil {
		return err
	}

	var err error
	if node.nullable, err = optionalBoolean(object, "nullable", pointer); err != nil {
		return err
	}

	if node.enum, err = parseSchemaEnum(object, pointer); err != nil {
		return err
	}

	if node.minimum, err = optionalExactNumber(object, "minimum", pointer); err != nil {
		return err
	}

	if node.maximum, err = optionalExactNumber(object, "maximum", pointer); err != nil {
		return err
	}

	if node.multipleOf, err = optionalExactNumber(object, "multipleOf", pointer); err != nil {
		return err
	}

	if node.multipleOf != nil {
		zero, parseErr := parseExactNumber("0")
		if parseErr != nil {
			return parseErr
		}

		comparison, compareErr := node.multipleOf.compare(zero)
		if compareErr != nil {
			return compareErr
		}

		if comparison <= 0 {
			return fmt.Errorf("%s/multipleOf: must be greater than zero", pointer)
		}
	}

	if node.exclusiveMinimum, err = optionalBoolean(object, "exclusiveMinimum", pointer); err != nil {
		return err
	}

	if node.exclusiveMaximum, err = optionalBoolean(object, "exclusiveMaximum", pointer); err != nil {
		return err
	}

	if node.format, err = parseSchemaFormat(object, node.kind, pointer); err != nil {
		return err
	}

	node.defaultValue, err = parseSchemaDefault(object, node.kind, node.nullable, pointer)

	return err
}

func parseSchemaType(node *schemaNode, object map[string]*jsonValue, pointer string) error {
	value, exists := object["type"]
	if !exists {
		node.kind = schemaAny

		return nil
	}

	if value.kind != jsonString {
		return fmt.Errorf("%s/type: must be a string", pointer)
	}

	kind, supported := schemaKinds[value.text]
	if !supported {
		return fmt.Errorf("%s/type: unknown OAS 3.0 type %q", pointer, value.text)
	}

	node.kind = kind

	return nil
}

func parseSchemaEnum(object map[string]*jsonValue, pointer string) ([]*jsonValue, error) {
	value, exists := object["enum"]
	if !exists {
		return nil, nil
	}

	if value.kind != jsonArray {
		return nil, fmt.Errorf("%s/enum: must be an array", pointer)
	}

	if len(value.array) == 0 {
		return nil, fmt.Errorf("%s/enum: empty enum is outside the schematest profile", pointer)
	}

	members := make([]*jsonValue, 0, len(value.array))
	for index, candidate := range value.array {
		duplicate := false

		for _, member := range members {
			equal, err := jsonSemanticEqual(candidate, member)
			if err != nil {
				return nil, fmt.Errorf("%s/enum/%d: compare enum member: %w", pointer, index, err)
			}

			if equal {
				duplicate = true

				break
			}
		}

		if !duplicate {
			members = append(members, candidate)
		}
	}

	return members, nil
}

func optionalExactNumber(object map[string]*jsonValue, name, pointer string) (*exactNumber, error) {
	value, exists := object[name]
	if !exists {
		return nil, nil
	}

	if value.kind != jsonNumber {
		return nil, fmt.Errorf("%s/%s: must be a number", pointer, name)
	}

	return value.number, nil
}

func optionalBoolean(object map[string]*jsonValue, name, pointer string) (bool, error) {
	value, exists := object[name]
	if !exists {
		return false, nil
	}

	if value.kind != jsonBoolean {
		return false, fmt.Errorf("%s/%s: must be a boolean", pointer, name)
	}

	return value.boolean, nil
}

func parseSchemaFormat(object map[string]*jsonValue, kind schemaKind, pointer string) (schemaFormat, error) {
	value, exists := object["format"]
	if !exists {
		return schemaFormatNone, nil
	}

	if value.kind != jsonString {
		return schemaFormatNone, fmt.Errorf("%s/format: must be a string", pointer)
	}

	format, supported := schemaFormats[value.text]
	if !supported {
		return schemaFormatNone, fmt.Errorf("%s/format: format %q is legal OAS but outside the schematest profile", pointer, value.text)
	}

	if !formatAllowedForKind(format, kind) {
		return schemaFormatNone, fmt.Errorf("%s/format: format %q is outside the profile for type %q", pointer, value.text, schemaKindName(kind))
	}

	return format, nil
}

func formatAllowedForKind(format schemaFormat, kind schemaKind) bool {
	if kind == schemaAny {
		return true
	}

	switch format {
	case schemaFormatInt32, schemaFormatInt64:
		return kind == schemaInteger || kind == schemaNumber
	case schemaFormatFloat, schemaFormatDouble:
		return kind == schemaNumber
	case schemaFormatByte, schemaFormatDate, schemaFormatDateTime, schemaFormatEmail, schemaFormatIPv4,
		schemaFormatUUID, schemaFormatUUIDv4, schemaFormatUUIDDashV4, schemaFormatCIDR,
		schemaFormatIPv4CIDR, schemaFormatPassword:
		return kind == schemaString
	case schemaFormatNone:
		return true
	default:
		return false
	}
}

func schemaKindName(kind schemaKind) string {
	for name, candidate := range schemaKinds {
		if candidate == kind {
			return name
		}
	}

	return "absent"
}

func parseSchemaDefault(
	object map[string]*jsonValue,
	kind schemaKind,
	nullable bool,
	pointer string,
) (*jsonValue, error) {
	value, exists := object["default"]
	if !exists {
		return nil, nil
	}

	if kind == schemaAny {
		return value, nil
	}

	matches, err := valueMatchesSchemaType(value, kind, nullable)
	if err != nil {
		return nil, fmt.Errorf("%s/default: %w", pointer, err)
	}

	if !matches {
		return nil, fmt.Errorf("%s/default: must conform to the explicit type in the same Schema Object", pointer)
	}

	return value, nil
}

func parseStringFields(node *schemaNode, object map[string]*jsonValue, pointer string) error {
	var err error
	if node.minLength, err = optionalCount(object, "minLength", pointer); err != nil {
		return err
	}

	if node.maxLength, err = optionalCount(object, "maxLength", pointer); err != nil {
		return err
	}

	value, exists := object["pattern"]
	if !exists {
		return nil
	}

	if value.kind != jsonString {
		return fmt.Errorf("%s/pattern: must be a string", pointer)
	}

	node.pattern, err = parseECMAPattern(value.text)
	if err != nil {
		return fmt.Errorf("%s/pattern: outside the schematest ECMA profile: %w", pointer, err)
	}

	return nil
}

func (parser *oasParser) parseArrayFields(
	node *schemaNode,
	object map[string]*jsonValue,
	pointer string,
) error {
	var err error
	if node.minItems, err = optionalCount(object, "minItems", pointer); err != nil {
		return err
	}

	if node.maxItems, err = optionalCount(object, "maxItems", pointer); err != nil {
		return err
	}

	items, exists := object["items"]
	if !exists {
		if node.kind == schemaArray {
			return fmt.Errorf("%s/items: required when type is array", pointer)
		}

		return nil
	}

	node.items, err = parser.parseSchemaNode(items, pointer+"/items", appendInstanceToken(node.occurrence.instanceTemplate, "*"))

	return err
}

func (parser *oasParser) parseObjectFields(
	node *schemaNode,
	object map[string]*jsonValue,
	pointer string,
) error {
	var err error
	if node.minProperties, err = optionalCount(object, "minProperties", pointer); err != nil {
		return err
	}

	if node.maxProperties, err = optionalCount(object, "maxProperties", pointer); err != nil {
		return err
	}

	authoredRequired, err := parseRequired(object, pointer)
	if err != nil {
		return err
	}

	node.properties, err = parser.parseProperties(object, pointer, node.occurrence.instanceTemplate)
	if err != nil {
		return err
	}

	for _, name := range authoredRequired {
		property, declared := node.properties[name]
		if declared && property.readOnly {
			continue
		}

		node.required = append(node.required, name)
	}

	return parser.parseAdditionalProperties(node, object, pointer)
}

func (parser *oasParser) parseProperties(
	object map[string]*jsonValue,
	pointer string,
	instanceTemplate string,
) (map[string]*schemaNode, error) {
	value, exists := object["properties"]
	if !exists {
		return make(map[string]*schemaNode), nil
	}

	properties, err := requireJSONObject(value, pointer+"/properties")
	if err != nil {
		return nil, err
	}

	parsed := make(map[string]*schemaNode, len(properties))
	for _, name := range sortedObjectNames(properties) {
		propertyPointer := pointer + "/properties/" + escapePointerToken(name)

		property, parseErr := parser.parseSchemaNode(
			properties[name],
			propertyPointer,
			appendInstanceToken(instanceTemplate, name),
		)
		if parseErr != nil {
			return nil, parseErr
		}

		if property.readOnly && property.writeOnly {
			return nil, fmt.Errorf("%s/writeOnly: a property cannot be both readOnly and writeOnly", propertyPointer)
		}

		parsed[name] = property
	}

	return parsed, nil
}

func (parser *oasParser) parseAdditionalProperties(
	node *schemaNode,
	object map[string]*jsonValue,
	pointer string,
) error {
	value, exists := object["additionalProperties"]
	if !exists {
		return nil
	}

	if value.kind == jsonBoolean {
		node.allowAdditionalProperties = value.boolean

		return nil
	}

	additional, err := parser.parseSchemaNode(
		value,
		pointer+"/additionalProperties",
		appendInstanceToken(node.occurrence.instanceTemplate, "*"),
	)
	if err != nil {
		return err
	}

	node.additionalProperties = additional
	node.allowAdditionalProperties = true

	return nil
}

func (parser *oasParser) parseCompositionFields(
	node *schemaNode,
	object map[string]*jsonValue,
	pointer string,
) error {
	var err error
	if node.allOf, err = parser.parseSchemaArray(object, "allOf", pointer, node.occurrence.instanceTemplate); err != nil {
		return err
	}

	if node.anyOf, err = parser.parseSchemaArray(object, "anyOf", pointer, node.occurrence.instanceTemplate); err != nil {
		return err
	}

	return nil
}

func (parser *oasParser) parseSchemaArray(
	object map[string]*jsonValue,
	name string,
	pointer string,
	instanceTemplate string,
) ([]*schemaNode, error) {
	value, exists := object[name]
	if !exists {
		return nil, nil
	}

	fieldPointer := pointer + "/" + name
	if value.kind != jsonArray {
		return nil, fmt.Errorf("%s: must be an array", fieldPointer)
	}

	if len(value.array) == 0 {
		return nil, fmt.Errorf("%s: must contain at least one schema", fieldPointer)
	}

	children := make([]*schemaNode, 0, len(value.array))
	for index, childValue := range value.array {
		childPointer := fmt.Sprintf("%s/%d", fieldPointer, index)

		child, err := parser.parseSchemaNode(childValue, childPointer, instanceTemplate)
		if err != nil {
			return nil, err
		}

		children = append(children, child)
	}

	return children, nil
}

func optionalCount(object map[string]*jsonValue, name, pointer string) (*exactCount, error) {
	value, exists := object[name]
	if !exists {
		return nil, nil
	}

	if value.kind != jsonNumber {
		return nil, fmt.Errorf("%s/%s: must be a non-negative integer", pointer, name)
	}

	integer, err := value.number.isInteger()
	if err != nil {
		return nil, fmt.Errorf("%s/%s: %w", pointer, name, err)
	}

	zero, err := parseExactNumber("0")
	if err != nil {
		return nil, err
	}

	comparison, err := value.number.compare(zero)
	if err != nil {
		return nil, fmt.Errorf("%s/%s: %w", pointer, name, err)
	}

	if !integer || comparison < 0 {
		return nil, fmt.Errorf("%s/%s: must be a non-negative integer", pointer, name)
	}

	return &exactCount{number: value.number}, nil
}

func parseRequired(object map[string]*jsonValue, pointer string) ([]string, error) {
	value, exists := object["required"]
	if !exists {
		return nil, nil
	}

	if value.kind != jsonArray {
		return nil, fmt.Errorf("%s/required: must be an array", pointer)
	}

	if len(value.array) == 0 {
		return nil, fmt.Errorf("%s/required: must contain at least one property name", pointer)
	}

	required := make([]string, 0, len(value.array))

	seen := make(map[string]bool, len(value.array))
	for index, member := range value.array {
		if member.kind != jsonString {
			return nil, fmt.Errorf("%s/required/%d: must be a string", pointer, index)
		}

		if seen[member.text] {
			return nil, fmt.Errorf("%s/required/%d: duplicate property name %q", pointer, index, member.text)
		}

		seen[member.text] = true
		required = append(required, member.text)
	}

	return required, nil
}

func appendInstanceToken(pointer, token string) string {
	return pointer + "/" + escapePointerToken(token)
}

func valueMatchesSchemaType(value *jsonValue, kind schemaKind, nullable bool) (bool, error) {
	if value == nil {
		return false, errors.New("default value is nil")
	}

	if value.kind == jsonNull {
		return nullable, nil
	}

	switch kind {
	case schemaBoolean:
		return value.kind == jsonBoolean, nil
	case schemaInteger:
		if value.kind != jsonNumber {
			return false, nil
		}

		return value.number.isInteger()
	case schemaNumber:
		return value.kind == jsonNumber, nil
	case schemaString:
		return value.kind == jsonString, nil
	case schemaArray:
		return value.kind == jsonArray, nil
	case schemaObject:
		return value.kind == jsonObject, nil
	case schemaAny:
		return true, nil
	default:
		return false, fmt.Errorf("unknown schema kind %d", kind)
	}
}
