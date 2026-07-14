//nolint:godoclint // The private literal helpers are local implementation details.
package generate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/djosh34/decode_and_validate_generator/pkg/jsonvalue"
	"github.com/djosh34/decode_and_validate_generator/pkg/validation"
)

type requiredImports struct {
	json   bool
	regexp bool
	value  bool
}

func renderAssignment(name string, root *validation.Validation) (assignmentTemplate, requiredImports) {
	nodes := collectValidations(root)

	indexes := make(map[*validation.Validation]int, len(nodes))
	for index, node := range nodes {
		indexes[node] = index
	}

	assignment := assignmentTemplate{Name: name}

	var imports requiredImports

	for _, node := range nodes {
		literal, nodeImports := validationLiteral(node)
		assignment.Nodes = append(assignment.Nodes, literal)
		imports.json = imports.json || nodeImports.json
		imports.regexp = imports.regexp || nodeImports.regexp
		imports.value = imports.value || nodeImports.value
	}

	for index, node := range nodes {
		assignment.Links = append(assignment.Links, validationLinks(index, node, indexes)...)
	}

	return assignment, imports
}

func collectValidations(root *validation.Validation) []*validation.Validation {
	seen := make(map[*validation.Validation]struct{})

	var nodes []*validation.Validation

	var collect func(*validation.Validation)

	collect = func(current *validation.Validation) {
		if current == nil {
			return
		}

		if _, ok := seen[current]; ok {
			return
		}

		seen[current] = struct{}{}
		nodes = append(nodes, current)

		collect(current.ArrayValidation.Items)

		for _, property := range current.ObjectValidation.Properties {
			collect(property.Validation)
		}

		collect(current.ObjectValidation.AdditionalPropertiesValidation)

		for _, child := range current.AllOfValidations {
			collect(child)
		}
	}

	collect(root)

	return nodes
}

//nolint:cyclop // Each independent validation family contributes its own literal and imports.
func validationLiteral(compiled *validation.Validation) (string, requiredImports) {
	fields := []string{"SchemaPointer: " + strconv.Quote(compiled.SchemaPointer)}
	if compiled.BodyRequired {
		fields = append(fields, "BodyRequired: true")
	}

	if compiled.KindValidation != (validation.KindValidation{}) {
		fields = append(fields, "KindValidation: "+kindLiteral(compiled.KindValidation))
	}

	var imports requiredImports

	if len(compiled.EnumValidation.Values) != 0 {
		fields = append(fields, "EnumValidation: "+enumLiteral(compiled.EnumValidation))
		imports.json = true
		imports.value = true
	}

	if compiled.NumberValidation != (validation.NumberValidation{}) {
		fields = append(fields, "NumberValidation: "+numberValidationLiteral(compiled.NumberValidation))
		imports.value = true
	}

	if compiled.StringValidation != (validation.StringValidation{}) {
		fields = append(fields, "StringValidation: "+stringValidationLiteral(compiled.StringValidation))
		imports.value = imports.value || compiled.StringValidation.MinLength != nil ||
			compiled.StringValidation.MaxLength != nil
		imports.regexp = compiled.StringValidation.CompiledPattern != nil
	}

	if compiled.ArrayValidation.MinItems != nil || compiled.ArrayValidation.MaxItems != nil ||
		compiled.ArrayValidation.UniqueItems {
		fields = append(fields, "ArrayValidation: "+arrayLiteral(compiled.ArrayValidation))
		imports.value = imports.value || compiled.ArrayValidation.MinItems != nil ||
			compiled.ArrayValidation.MaxItems != nil
	}

	if hasObjectValues(compiled.ObjectValidation) {
		fields = append(fields, "ObjectValidation: "+objectLiteral(compiled.ObjectValidation))
		imports.value = imports.value || compiled.ObjectValidation.MinProperties != nil ||
			compiled.ObjectValidation.MaxProperties != nil
	}

	return "{" + strings.Join(fields, ", ") + "}", imports
}

func kindLiteral(kind validation.KindValidation) string {
	var fields []string
	if kind.Type != "" {
		fields = append(fields, "Type: "+strconv.Quote(kind.Type))
	}

	if kind.Nullable {
		fields = append(fields, "Nullable: true")
	}

	return "validation.KindValidation{" + strings.Join(fields, ", ") + "}"
}

func enumLiteral(enum validation.EnumValidation) string {
	values := make([]string, len(enum.Values))
	for index, value := range enum.Values {
		values[index] = "json.RawMessage(" + strconv.Quote(string(value)) + ")"
	}

	exact := make([]string, len(enum.ExactValues))
	for index, value := range enum.ExactValues {
		exact[index] = valueLiteral(value)
	}

	return "validation.EnumValidation{Values: []json.RawMessage{" + strings.Join(values, ", ") +
		"}, ExactValues: []jsonvalue.Value{" + strings.Join(exact, ", ") + "}}"
}

func numberValidationLiteral(number validation.NumberValidation) string {
	var fields []string
	if number.Minimum != nil {
		fields = append(fields, "Minimum: "+numberBoundLiteral(number.Minimum))
	}

	if number.Maximum != nil {
		fields = append(fields, "Maximum: "+numberBoundLiteral(number.Maximum))
	}

	if number.ExactMultipleOf != nil {
		fields = append(fields, "MultipleOf: "+strconv.Quote(number.MultipleOf))
		fields = append(fields, "ExactMultipleOf: &jsonvalue.Number{Lexeme: "+
			strconv.Quote(number.ExactMultipleOf.Lexeme)+"}")
	}

	return "validation.NumberValidation{" + strings.Join(fields, ", ") + "}"
}

func numberBoundLiteral(bound *validation.NumberBound) string {
	fields := []string{
		"Value: " + strconv.Quote(bound.Value),
		"ExactValue: jsonvalue.Number{Lexeme: " + strconv.Quote(bound.ExactValue.Lexeme) + "}",
	}
	if bound.Exclusive {
		fields = append(fields, "Exclusive: true")
	}

	return "&validation.NumberBound{" + strings.Join(fields, ", ") + "}"
}

func stringValidationLiteral(value validation.StringValidation) string {
	var fields []string
	if value.MinLength != nil {
		fields = append(fields, "MinLength: "+countBoundLiteral(value.MinLength))
	}

	if value.MaxLength != nil {
		fields = append(fields, "MaxLength: "+countBoundLiteral(value.MaxLength))
	}

	if value.Pattern != "" {
		fields = append(fields, "Pattern: "+strconv.Quote(value.Pattern))
	}

	if value.Format != "" {
		fields = append(fields, "Format: "+strconv.Quote(value.Format))
	}

	if value.CompiledPattern != nil {
		fields = append(fields, "CompiledPattern: regexp.MustCompile("+strconv.Quote(value.Pattern)+")")
	}

	return "validation.StringValidation{" + strings.Join(fields, ", ") + "}"
}

func arrayLiteral(array validation.ArrayValidation) string {
	var fields []string
	if array.MinItems != nil {
		fields = append(fields, "MinItems: "+countBoundLiteral(array.MinItems))
	}

	if array.MaxItems != nil {
		fields = append(fields, "MaxItems: "+countBoundLiteral(array.MaxItems))
	}

	if array.UniqueItems {
		fields = append(fields, "UniqueItems: true")
	}

	return "validation.ArrayValidation{" + strings.Join(fields, ", ") + "}"
}

func hasObjectValues(object validation.ObjectValidation) bool {
	return object.MinProperties != nil || object.MaxProperties != nil || len(object.Required) != 0 ||
		len(object.Properties) != 0 || object.AdditionalPropertiesAllowed
}

func objectLiteral(object validation.ObjectValidation) string {
	var fields []string
	if object.MinProperties != nil {
		fields = append(fields, "MinProperties: "+countBoundLiteral(object.MinProperties))
	}

	if object.MaxProperties != nil {
		fields = append(fields, "MaxProperties: "+countBoundLiteral(object.MaxProperties))
	}

	if len(object.Required) != 0 {
		fields = append(fields, "Required: "+stringSliceLiteral(object.Required))
	}

	if len(object.Properties) != 0 {
		properties := make([]string, len(object.Properties))
		for index, property := range object.Properties {
			properties[index] = "{Name: " + strconv.Quote(property.Name) + "}"
		}

		fields = append(fields, "Properties: []validation.PropertyValidation{"+
			strings.Join(properties, ", ")+"}")
	}

	if object.AdditionalPropertiesAllowed {
		fields = append(fields, "AdditionalPropertiesAllowed: true")
	}

	return "validation.ObjectValidation{" + strings.Join(fields, ", ") + "}"
}

func countBoundLiteral(bound *validation.CountBound) string {
	return "&validation.CountBound{Value: " + strconv.Quote(bound.Value) +
		", ExactValue: jsonvalue.Number{Lexeme: " + strconv.Quote(bound.ExactValue.Lexeme) + "}}"
}

func stringSliceLiteral(values []string) string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = strconv.Quote(value)
	}

	return "[]string{" + strings.Join(quoted, ", ") + "}"
}

func valueLiteral(value jsonvalue.Value) string {
	var fields []string
	if value.Kind != jsonvalue.KindNull {
		fields = append(fields, "Kind: "+kindName(value.Kind))
	}

	if value.Boolean {
		fields = append(fields, "Boolean: true")
	}

	if value.Number.Lexeme != "" {
		fields = append(fields, "Number: jsonvalue.Number{Lexeme: "+strconv.Quote(value.Number.Lexeme)+"}")
	}

	if value.String != "" {
		fields = append(fields, "String: "+strconv.Quote(value.String))
	}

	if value.Array != nil {
		children := make([]string, len(value.Array))
		for index, child := range value.Array {
			children[index] = valueLiteral(child)
		}

		fields = append(fields, "Array: []jsonvalue.Value{"+strings.Join(children, ", ")+"}")
	}

	if value.Object != nil {
		members := make([]string, len(value.Object))
		for index, member := range value.Object {
			members[index] = "{Name: " + strconv.Quote(member.Name) + ", Value: " + valueLiteral(member.Value) + "}"
		}

		fields = append(fields, "Object: []jsonvalue.Member{"+strings.Join(members, ", ")+"}")
	}

	return "jsonvalue.Value{" + strings.Join(fields, ", ") + "}"
}

func kindName(kind jsonvalue.Kind) string {
	names := map[jsonvalue.Kind]string{
		jsonvalue.KindBoolean: "jsonvalue.KindBoolean",
		jsonvalue.KindNumber:  "jsonvalue.KindNumber",
		jsonvalue.KindString:  "jsonvalue.KindString",
		jsonvalue.KindArray:   "jsonvalue.KindArray",
		jsonvalue.KindObject:  "jsonvalue.KindObject",
	}

	return names[kind]
}

func validationLinks(
	index int,
	compiled *validation.Validation,
	indexes map[*validation.Validation]int,
) []string {
	var links []string
	if compiled.ArrayValidation.Items != nil {
		links = append(links, fmt.Sprintf("validations[%d].ArrayValidation.Items = validations[%d]", index,
			indexes[compiled.ArrayValidation.Items]))
	}

	for propertyIndex, property := range compiled.ObjectValidation.Properties {
		links = append(links, fmt.Sprintf("validations[%d].ObjectValidation.Properties[%d].Validation = validations[%d]",
			index, propertyIndex, indexes[property.Validation]))
	}

	if compiled.ObjectValidation.AdditionalPropertiesValidation != nil {
		links = append(links, fmt.Sprintf(
			"validations[%d].ObjectValidation.AdditionalPropertiesValidation = validations[%d]",
			index, indexes[compiled.ObjectValidation.AdditionalPropertiesValidation],
		))
	}

	for _, child := range compiled.AllOfValidations {
		links = append(links, fmt.Sprintf(
			"validations[%d].AllOfValidations = append(validations[%d].AllOfValidations, validations[%d])",
			index, index, indexes[child],
		))
	}

	return links
}
