//nolint:cyclop,gocognit,gocyclo,godoclint,maintidx,mnd // Lowering maps admitted keywords to atoms.
package program

import (
	"fmt"
	"math/big"
	"slices"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // String atoms retain shared automata.
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/validation"
)

type nodeID uint32

type nodeKind uint8

const (
	nodeAtom nodeKind = iota
	nodeAnd
	nodeOr
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
	kind              atomKind
	allowed           [6]bool
	integer           bool
	count             uint64
	number            jsonvalue.Number
	exclusive         bool
	text              string
	language          stringlanguage.Language
	values            []jsonvalue.Value
	child             nodeID
	name              string
	names             []string
	allowedAdditional bool
	hasChild          bool
}

type node struct {
	kind     nodeKind
	children []nodeID
	atom     atom
}

type graphLowerer struct {
	nodes        []node
	byValidation map[*validation.Validation]nodeID
	byAtom       map[string]nodeID
}

func (lower *graphLowerer) validation(source *validation.Validation) (nodeID, error) {
	if existing, ok := lower.byValidation[source]; ok {
		return existing, nil
	}

	root := lower.add(node{kind: nodeAnd})
	lower.byValidation[source] = root
	children := make([]nodeID, 0)

	if source.KindValidation.Type != "" {
		allowed, integer, err := allowedKinds(source.KindValidation)
		if err != nil {
			return 0, err
		}

		children = append(children, lower.add(node{
			kind: nodeAtom,
			atom: atom{kind: atomKinds, allowed: allowed, integer: integer},
		}))
	}

	if len(source.EnumValidation.ExactValues) != 0 {
		values := make([]jsonvalue.Value, len(source.EnumValidation.ExactValues))
		for index, value := range source.EnumValidation.ExactValues {
			copied, err := copyValue(value)
			if err != nil {
				return 0, fmt.Errorf("schema %s enum member %d: %w", source.SchemaPointer, index, err)
			}

			values[index] = copied
		}

		children = append(children, lower.add(node{
			kind: nodeAtom,
			atom: atom{kind: atomEnum, values: values},
		}))
	}

	if source.NumberValidation.Minimum != nil {
		children = append(children, lower.add(node{
			kind: nodeAtom,
			atom: atom{
				kind:      atomNumberMinimum,
				number:    source.NumberValidation.Minimum.ExactValue,
				exclusive: source.NumberValidation.Minimum.Exclusive,
			},
		}))
	}

	if source.NumberValidation.Maximum != nil {
		children = append(children, lower.add(node{
			kind: nodeAtom,
			atom: atom{
				kind:      atomNumberMaximum,
				number:    source.NumberValidation.Maximum.ExactValue,
				exclusive: source.NumberValidation.Maximum.Exclusive,
			},
		}))
	}

	if source.NumberValidation.ExactMultipleOf != nil {
		children = append(children, lower.add(node{
			kind: nodeAtom,
			atom: atom{
				kind:   atomNumberMultipleOf,
				number: *source.NumberValidation.ExactMultipleOf,
			},
		}))
	}

	if source.NumberValidation.Format != "" {
		children = append(children, lower.add(node{
			kind: nodeAtom,
			atom: atom{kind: atomNumberFormat, text: source.NumberValidation.Format},
		}))
	}

	if source.StringValidation.MinLength != nil {
		minimum, err := countValue(source.StringValidation.MinLength)
		if err != nil {
			return 0, fmt.Errorf("schema %s minLength: %w", source.SchemaPointer, err)
		}

		children = append(children, lower.add(node{
			kind: nodeAtom,
			atom: atom{kind: atomStringMinLength, count: minimum},
		}))
	}

	if source.StringValidation.MaxLength != nil {
		maximum, err := countValue(source.StringValidation.MaxLength)
		if err != nil {
			return 0, fmt.Errorf("schema %s maxLength: %w", source.SchemaPointer, err)
		}

		children = append(children, lower.add(node{
			kind: nodeAtom,
			atom: atom{kind: atomStringMaxLength, count: maximum},
		}))
	}

	if source.StringValidation.CompiledPattern != nil {
		language, err := compilePatternLanguage(
			source.StringValidation.Pattern,
			source.StringValidation.CompiledPattern,
		)
		if err != nil {
			return 0, fmt.Errorf("schema %s pattern: %w", source.SchemaPointer, err)
		}

		children = append(children, lower.add(node{
			kind: nodeAtom,
			atom: atom{
				kind: atomStringLanguage,
				text: fmt.Sprintf(
					"pattern:%t:%t:%s",
					source.StringValidation.CompiledPattern.RejectsNonASCII(),
					source.StringValidation.CompiledPattern.UsesRE2(),
					source.StringValidation.Pattern,
				),
				language: language,
			},
		}))
	}

	if source.StringValidation.CompiledFormat != nil {
		language, err := stringlanguage.Format(source.StringValidation.Format)
		if err != nil {
			return 0, fmt.Errorf("schema %s format: %w", source.SchemaPointer, err)
		}

		children = append(children, lower.add(node{
			kind: nodeAtom,
			atom: atom{
				kind: atomStringLanguage, text: "format:" + source.StringValidation.Format,
				language: language,
			},
		}))
	}

	if source.ArrayValidation.MinItems != nil {
		minimum, err := countValue(source.ArrayValidation.MinItems)
		if err != nil {
			return 0, fmt.Errorf("schema %s minItems: %w", source.SchemaPointer, err)
		}

		children = append(children, lower.add(node{
			kind: nodeAtom, atom: atom{kind: atomArrayMinItems, count: minimum},
		}))
	}

	if source.ArrayValidation.MaxItems != nil {
		maximum, err := countValue(source.ArrayValidation.MaxItems)
		if err != nil {
			return 0, fmt.Errorf("schema %s maxItems: %w", source.SchemaPointer, err)
		}

		children = append(children, lower.add(node{
			kind: nodeAtom, atom: atom{kind: atomArrayMaxItems, count: maximum},
		}))
	}

	if source.ArrayValidation.Items != nil {
		items, err := lower.validation(source.ArrayValidation.Items)
		if err != nil {
			return 0, err
		}

		children = append(children, lower.add(node{
			kind: nodeAtom, atom: atom{kind: atomArrayItems, child: items},
		}))
	}

	if source.ObjectValidation.MinProperties != nil {
		minimum, err := countValue(source.ObjectValidation.MinProperties)
		if err != nil {
			return 0, fmt.Errorf("schema %s minProperties: %w", source.SchemaPointer, err)
		}

		children = append(children, lower.add(node{
			kind: nodeAtom, atom: atom{kind: atomObjectMinProperties, count: minimum},
		}))
	}

	if source.ObjectValidation.MaxProperties != nil {
		maximum, err := countValue(source.ObjectValidation.MaxProperties)
		if err != nil {
			return 0, fmt.Errorf("schema %s maxProperties: %w", source.SchemaPointer, err)
		}

		children = append(children, lower.add(node{
			kind: nodeAtom, atom: atom{kind: atomObjectMaxProperties, count: maximum},
		}))
	}

	for _, name := range source.ObjectValidation.Required {
		children = append(children, lower.add(node{
			kind: nodeAtom, atom: atom{kind: atomObjectRequired, name: name},
		}))
	}

	definedNames := make([]string, 0, len(source.ObjectValidation.Properties))
	for _, property := range source.ObjectValidation.Properties {
		compiled, err := lower.validation(property.Validation)
		if err != nil {
			return 0, err
		}

		definedNames = append(definedNames, property.Name)
		children = append(children, lower.add(node{
			kind: nodeAtom,
			atom: atom{kind: atomObjectProperty, name: property.Name, child: compiled},
		}))
	}

	slices.Sort(definedNames)
	definedNames = slices.Compact(definedNames)

	if !source.ObjectValidation.AdditionalPropertiesAllowed ||
		source.ObjectValidation.AdditionalPropertiesValidation != nil {
		additional := atom{
			kind:              atomObjectAdditional,
			names:             slices.Clone(definedNames),
			allowedAdditional: source.ObjectValidation.AdditionalPropertiesAllowed,
		}
		if source.ObjectValidation.AdditionalPropertiesValidation != nil {
			compiled, err := lower.validation(source.ObjectValidation.AdditionalPropertiesValidation)
			if err != nil {
				return 0, err
			}

			additional.child = compiled
			additional.hasChild = true
		}

		children = append(children, lower.add(node{kind: nodeAtom, atom: additional}))
	}

	for _, child := range source.AllOfValidations {
		compiled, err := lower.validation(child)
		if err != nil {
			return 0, err
		}

		children = append(children, compiled)
	}

	if len(source.AnyOfValidations) != 0 {
		alternatives := make([]nodeID, len(source.AnyOfValidations))
		for index, child := range source.AnyOfValidations {
			compiled, err := lower.validation(child)
			if err != nil {
				return 0, err
			}

			alternatives[index] = compiled
		}

		children = append(children, lower.add(node{kind: nodeOr, children: alternatives}))
	}

	lower.nodes[root].children = children

	return root, nil
}

func copyValue(value jsonvalue.Value) (jsonvalue.Value, error) {
	raw, err := value.MarshalJSON()
	if err != nil {
		return jsonvalue.Value{}, err
	}

	return jsonvalue.Parse(raw)
}

func compilePatternLanguage(
	source string,
	compiled *patternvalidator.PatternValidation,
) (stringlanguage.Language, error) {
	options := make([]patternvalidator.Option, 0, 2)
	if compiled.RejectsNonASCII() {
		options = append(options, patternvalidator.RejectNonASCII)
	}

	if compiled.UsesRE2() {
		options = append(options, patternvalidator.UseRE2)
	}

	return stringlanguage.Pattern(source, options...)
}

func (lower *graphLowerer) add(item node) nodeID {
	if item.kind == nodeAtom {
		key := atomKey(item.atom)
		if existing, ok := lower.byAtom[key]; ok {
			return existing
		}

		identifier := nodeID(len(lower.nodes))
		lower.nodes = append(lower.nodes, item)
		lower.byAtom[key] = identifier

		return identifier
	}

	identifier := nodeID(len(lower.nodes))
	lower.nodes = append(lower.nodes, item)

	return identifier
}

func atomKey(item atom) string {
	key := fmt.Sprintf(
		"%d|%v|%t|%d|%q|%t|%q|%d|%q|%q|%t|%t",
		item.kind,
		item.allowed,
		item.integer,
		item.count,
		item.number.Lexeme,
		item.exclusive,
		item.text,
		item.child,
		item.name,
		item.names,
		item.allowedAdditional,
		item.hasChild,
	)
	for _, value := range item.values {
		raw, err := value.MarshalJSON()
		if err != nil {
			panic(fmt.Errorf("marshal compiled enum atom: %w", err))
		}

		key += "|" + string(raw)
	}

	return key
}

func allowedKinds(kind validation.KindValidation) ([6]bool, bool, error) {
	var allowed [6]bool

	integer := false

	switch kind.Type {
	case "boolean":
		allowed[jsonvalue.KindBoolean] = true
	case "integer":
		allowed[jsonvalue.KindNumber] = true
		integer = true
	case "number":
		allowed[jsonvalue.KindNumber] = true
	case "string":
		allowed[jsonvalue.KindString] = true
	case "array":
		allowed[jsonvalue.KindArray] = true
	case "object":
		allowed[jsonvalue.KindObject] = true
	default:
		return allowed, false, fmt.Errorf("unsupported type %q", kind.Type)
	}

	if kind.Nullable {
		allowed[jsonvalue.KindNull] = true
	}

	return allowed, integer, nil
}

func countValue(bound *validation.CountBound) (uint64, error) {
	if bound.ExactValue.Rational == nil || !bound.ExactValue.Rational.IsInt() {
		return 0, fmt.Errorf("count %q is not a materialized integer", bound.Value)
	}

	value := bound.ExactValue.Rational.Num()
	if value.Sign() < 0 || !value.IsUint64() {
		return 0, fmt.Errorf("count %q is outside uint64", bound.Value)
	}

	return new(big.Int).Set(value).Uint64(), nil
}
