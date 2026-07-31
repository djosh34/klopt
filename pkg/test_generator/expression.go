//nolint:godoclint // Private immutable expression vocabulary is tested within the package.
package testgenerator

import (
	"fmt"
	"slices"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // Expressions retain the shared compiled language.
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/validation"
)

type expressionKind uint8

const (
	expressionAtom expressionKind = iota
	expressionAll
	expressionAny
)

type expression struct {
	kind     expressionKind
	children []*expression
	atom     atom
}

func newAtomExpression(rule atom) *expression {
	return &expression{kind: expressionAtom, atom: rule}
}

func newAllExpression(children []*expression) *expression {
	return &expression{kind: expressionAll, children: slices.Clone(children)}
}

func newAnyExpression(children []*expression) *expression {
	return &expression{kind: expressionAny, children: slices.Clone(children)}
}

type demand struct {
	expression *expression
	wantPass   bool
}

func newDemand(expression *expression, wantPass bool) demand {
	return demand{expression: expression, wantPass: wantPass}
}

type jsonKind uint8

const (
	kindNull jsonKind = iota
	kindBoolean
	kindNumber
	kindString
	kindArray
	kindObject
	jsonKindCount
)

type atomKind uint8

const (
	atomKinds atomKind = iota
	atomEnum
	atomNumberMinimum
	atomNumberMaximum
	atomNumberMultipleOf
	atomNumberFormat
	atomStringMinLength
	atomStringMaxLength
	atomStringLanguage
	atomArrayMinItems
	atomArrayMaxItems
	atomArrayItems
	atomObjectMinProperties
	atomObjectMaxProperties
	atomObjectRequired
	atomObjectProperty
	atomObjectAdditional
)

type atom struct {
	kind          atomKind
	schemaPointer string

	allowed   [jsonKindCount]bool
	integer   bool
	exclusive bool
	count     jsonvalue.Number
	number    jsonvalue.Number
	text      string
	language  stringlanguage.Language
	values    []jsonvalue.Value

	child             *expression
	name              string
	names             []string
	allowedAdditional bool
	hasChild          bool
}

type expressionLowerer struct {
	byValidation map[*validation.Validation]*expression
}

//nolint:cyclop // Recursive lowering has one explicit branch per expression family.
func (lowerer *expressionLowerer) lower(source *validation.Validation) (*expression, error) {
	if lowerer == nil {
		return nil, fmt.Errorf("lower with nil expression lowerer")
	}

	if source == nil {
		return nil, fmt.Errorf("lower nil validation")
	}

	if lowerer.byValidation == nil {
		return nil, fmt.Errorf("lowerer has nil validation map")
	}

	if existing, ok := lowerer.byValidation[source]; ok {
		return existing, nil
	}

	if source.AllOfValidations != nil && len(source.AllOfValidations) == 0 {
		return nil, fmt.Errorf("schema %q has an empty allOf", source.SchemaPointer)
	}

	if source.AnyOfValidations != nil && len(source.AnyOfValidations) == 0 {
		return nil, fmt.Errorf("schema %q has an empty anyOf", source.SchemaPointer)
	}

	children, err := lowerer.lowerLocalAtoms(source)
	if err != nil {
		return nil, fmt.Errorf("lower schema %q local rules: %w", source.SchemaPointer, err)
	}

	for index, child := range source.AllOfValidations {
		lowered, lowerErr := lowerer.lower(child)
		if lowerErr != nil {
			return nil, fmt.Errorf("lower schema %q allOf child %d: %w", source.SchemaPointer, index, lowerErr)
		}

		children = append(children, lowered)
	}

	if len(source.AnyOfValidations) > 0 {
		alternatives := make([]*expression, 0, len(source.AnyOfValidations))
		for index, child := range source.AnyOfValidations {
			lowered, lowerErr := lowerer.lower(child)
			if lowerErr != nil {
				return nil, fmt.Errorf("lower schema %q anyOf child %d: %w", source.SchemaPointer, index, lowerErr)
			}

			alternatives = append(alternatives, lowered)
		}

		children = append(children, newAnyExpression(alternatives))
	}

	result := newAllExpression(children)
	lowerer.byValidation[source] = result

	return result, nil
}

//nolint:cyclop,gocyclo,gocognit,maintidx // Each retained validation family has a distinct exact atom.
func (lowerer *expressionLowerer) lowerLocalAtoms(source *validation.Validation) ([]*expression, error) {
	children := make([]*expression, 0)

	kindRule, err := lowerKindAtom(source)
	if err != nil {
		return nil, err
	}

	children = append(children, newAtomExpression(kindRule))

	if source.EnumValidation.Values != nil && len(source.EnumValidation.Values) == 0 {
		return nil, fmt.Errorf("enum has no values")
	}

	if len(source.EnumValidation.Values) != len(source.EnumValidation.ExactValues) {
		return nil, fmt.Errorf("enum values and exact values have different lengths")
	}

	if len(source.EnumValidation.Values) > 0 {
		values := make([]jsonvalue.Value, len(source.EnumValidation.ExactValues))
		for index, value := range source.EnumValidation.ExactValues {
			cloned, cloneErr := cloneJSONValue(value)
			if cloneErr != nil {
				return nil, fmt.Errorf("enum member %d: %w", index, cloneErr)
			}

			values[index] = cloned
		}

		children = append(children, newAtomExpression(atom{
			kind:          atomEnum,
			schemaPointer: source.SchemaPointer,
			values:        values,
		}))
	}

	number := source.NumberValidation
	if number.Minimum != nil {
		value, numberErr := cloneNumber(number.Minimum.ExactValue)
		if numberErr != nil {
			return nil, fmt.Errorf("minimum: %w", numberErr)
		}

		children = append(children, newAtomExpression(atom{
			kind:          atomNumberMinimum,
			schemaPointer: source.SchemaPointer,
			exclusive:     number.Minimum.Exclusive,
			number:        value,
		}))
	}

	if number.Maximum != nil {
		value, numberErr := cloneNumber(number.Maximum.ExactValue)
		if numberErr != nil {
			return nil, fmt.Errorf("maximum: %w", numberErr)
		}

		children = append(children, newAtomExpression(atom{
			kind:          atomNumberMaximum,
			schemaPointer: source.SchemaPointer,
			exclusive:     number.Maximum.Exclusive,
			number:        value,
		}))
	}

	if number.ExactMultipleOf != nil {
		value, numberErr := cloneNumber(*number.ExactMultipleOf)
		if numberErr != nil {
			return nil, fmt.Errorf("multipleOf: %w", numberErr)
		}

		children = append(children, newAtomExpression(atom{
			kind:          atomNumberMultipleOf,
			schemaPointer: source.SchemaPointer,
			number:        value,
		}))
	}

	if number.Format != "" {
		children = append(children, newAtomExpression(atom{
			kind:          atomNumberFormat,
			schemaPointer: source.SchemaPointer,
			text:          number.Format,
		}))
	}

	stringValidation := source.StringValidation
	if stringValidation.MinLength != nil {
		count, countErr := cloneNumber(stringValidation.MinLength.ExactValue)
		if countErr != nil {
			return nil, fmt.Errorf("minLength: %w", countErr)
		}

		children = append(children, newAtomExpression(atom{
			kind:          atomStringMinLength,
			schemaPointer: source.SchemaPointer,
			count:         count,
		}))
	}

	if stringValidation.MaxLength != nil {
		count, countErr := cloneNumber(stringValidation.MaxLength.ExactValue)
		if countErr != nil {
			return nil, fmt.Errorf("maxLength: %w", countErr)
		}

		children = append(children, newAtomExpression(atom{
			kind:          atomStringMaxLength,
			schemaPointer: source.SchemaPointer,
			count:         count,
		}))
	}

	if stringValidation.CompiledPatternLanguage != nil {
		children = append(children, newAtomExpression(atom{
			kind:          atomStringLanguage,
			schemaPointer: source.SchemaPointer,
			language:      *stringValidation.CompiledPatternLanguage,
			text:          stringValidation.Pattern,
		}))
	} else if stringValidation.CompiledPattern != nil {
		return nil, fmt.Errorf("pattern has no compiled language")
	}

	if stringValidation.CompiledFormat != nil {
		children = append(children, newAtomExpression(atom{
			kind:          atomStringLanguage,
			schemaPointer: source.SchemaPointer,
			language:      *stringValidation.CompiledFormat,
			text:          stringValidation.Format,
		}))
	}

	array := source.ArrayValidation
	if array.MinItems != nil {
		count, countErr := cloneNumber(array.MinItems.ExactValue)
		if countErr != nil {
			return nil, fmt.Errorf("minItems: %w", countErr)
		}

		children = append(children, newAtomExpression(atom{
			kind:          atomArrayMinItems,
			schemaPointer: source.SchemaPointer,
			count:         count,
		}))
	}

	if array.MaxItems != nil {
		count, countErr := cloneNumber(array.MaxItems.ExactValue)
		if countErr != nil {
			return nil, fmt.Errorf("maxItems: %w", countErr)
		}

		children = append(children, newAtomExpression(atom{
			kind:          atomArrayMaxItems,
			schemaPointer: source.SchemaPointer,
			count:         count,
		}))
	}

	if array.Items != nil {
		itemExpression, lowerErr := lowerer.lower(array.Items)
		if lowerErr != nil {
			return nil, fmt.Errorf("items: %w", lowerErr)
		}

		children = append(children, newAtomExpression(atom{
			kind:          atomArrayItems,
			schemaPointer: source.SchemaPointer,
			child:         itemExpression,
		}))
	}

	object := source.ObjectValidation

	propertyNames := make([]string, 0, len(object.Properties))
	for _, property := range object.Properties {
		propertyNames = append(propertyNames, property.Name)
	}

	if object.MinProperties != nil {
		count, countErr := cloneNumber(object.MinProperties.ExactValue)
		if countErr != nil {
			return nil, fmt.Errorf("minProperties: %w", countErr)
		}

		children = append(children, newAtomExpression(atom{
			kind:          atomObjectMinProperties,
			schemaPointer: source.SchemaPointer,
			count:         count,
		}))
	}

	if object.MaxProperties != nil {
		count, countErr := cloneNumber(object.MaxProperties.ExactValue)
		if countErr != nil {
			return nil, fmt.Errorf("maxProperties: %w", countErr)
		}

		children = append(children, newAtomExpression(atom{
			kind:          atomObjectMaxProperties,
			schemaPointer: source.SchemaPointer,
			count:         count,
		}))
	}

	if object.Required != nil && len(object.Required) == 0 {
		return nil, fmt.Errorf("required has no names")
	}

	if len(object.Required) > 0 {
		if err := validateUniqueNames(object.Required); err != nil {
			return nil, fmt.Errorf("required: %w", err)
		}

		children = append(children, newAtomExpression(atom{
			kind:          atomObjectRequired,
			schemaPointer: source.SchemaPointer,
			names:         slices.Clone(object.Required),
		}))
	}

	for index, property := range object.Properties {
		if property.Validation == nil {
			return nil, fmt.Errorf("property %d %q has nil validation", index, property.Name)
		}

		propertyExpression, lowerErr := lowerer.lower(property.Validation)
		if lowerErr != nil {
			return nil, fmt.Errorf("property %q: %w", property.Name, lowerErr)
		}

		if index > 0 && object.Properties[index-1].Name >= property.Name {
			return nil, fmt.Errorf("properties are not sorted and unique")
		}

		children = append(children, newAtomExpression(atom{
			kind:          atomObjectProperty,
			schemaPointer: source.SchemaPointer,
			name:          property.Name,
			child:         propertyExpression,
		}))
	}

	if object.AdditionalPropertiesValidation != nil {
		additionalExpression, lowerErr := lowerer.lower(object.AdditionalPropertiesValidation)
		if lowerErr != nil {
			return nil, fmt.Errorf("additionalProperties: %w", lowerErr)
		}

		children = append(children, newAtomExpression(atom{
			kind:              atomObjectAdditional,
			schemaPointer:     source.SchemaPointer,
			child:             additionalExpression,
			names:             slices.Clone(propertyNames),
			allowedAdditional: object.AdditionalPropertiesAllowed,
			hasChild:          true,
		}))
	} else if !object.AdditionalPropertiesAllowed {
		children = append(children, newAtomExpression(atom{
			kind:              atomObjectAdditional,
			schemaPointer:     source.SchemaPointer,
			names:             slices.Clone(propertyNames),
			allowedAdditional: false,
		}))
	}

	return children, nil
}

//nolint:cyclop // The OpenAPI type alternatives are an explicit closed table.
func lowerKindAtom(source *validation.Validation) (atom, error) {
	allowed := [jsonKindCount]bool{
		kindNull: true, kindBoolean: true, kindNumber: true,
		kindString: true, kindArray: true, kindObject: true,
	}

	integer := false

	switch source.KindValidation.Type {
	case "":
	case "boolean":
		allowed = [jsonKindCount]bool{kindBoolean: true}
	case "integer":
		allowed = [jsonKindCount]bool{kindNumber: true}
		integer = true
	case "number":
		allowed = [jsonKindCount]bool{kindNumber: true}
	case "string":
		allowed = [jsonKindCount]bool{kindString: true}
	case "array":
		allowed = [jsonKindCount]bool{kindArray: true}
	case "object":
		allowed = [jsonKindCount]bool{kindObject: true}
	default:
		return atom{}, fmt.Errorf("unsupported type %q", source.KindValidation.Type)
	}

	if source.KindValidation.Type != "" && source.KindValidation.Nullable {
		allowed[kindNull] = true
	}

	return atom{
		kind:          atomKinds,
		schemaPointer: source.SchemaPointer,
		allowed:       allowed,
		integer:       integer,
	}, nil
}

func cloneNumber(number jsonvalue.Number) (jsonvalue.Number, error) {
	if number.Lexeme == "" {
		return jsonvalue.Number{}, fmt.Errorf("number has an empty lexeme")
	}

	cloned, err := jsonvalue.ParseNumber(number.Lexeme)
	if err != nil {
		return jsonvalue.Number{}, err
	}

	return cloned, nil
}

func cloneJSONValue(value jsonvalue.Value) (jsonvalue.Value, error) {
	encoded, err := value.MarshalJSON()
	if err != nil {
		return jsonvalue.Value{}, err
	}

	cloned, err := jsonvalue.Parse(encoded)
	if err != nil {
		return jsonvalue.Value{}, err
	}

	return cloned, nil
}
