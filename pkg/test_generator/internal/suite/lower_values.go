//nolint:godoclint // Private scalar lowering stays behind compileSemantic.
package suite

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // S2 exact string semantics are owned here.
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
)

const (
	initialKindRefCapacity    = 2
	initialNumberRefCapacity  = 3
	initialStringRuleCapacity = 2
	float32FormatBits         = 32
	float64FormatBits         = 64
)

func (lowerer *semanticLowerer) lowerLocal(
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
) (SetRef, []string, error) {
	refs := make([]SetRef, 0)

	kind, err := lowerer.lowerKind(members)
	if err != nil {
		return SetRef{}, nil, fmt.Errorf("lower schema at %s: %w", schema.Pointer, err)
	}

	refs = append(refs, kind...)

	enumeration, err := lowerer.lowerEnum(members)
	if err != nil {
		return SetRef{}, nil, fmt.Errorf("lower schema at %s/enum: %w", schema.Pointer, err)
	}

	refs = append(refs, enumeration...)

	numbers, err := lowerer.lowerNumber(members)
	if err != nil {
		return SetRef{}, nil, fmt.Errorf("lower schema at %s: %w", schema.Pointer, err)
	}

	refs = append(refs, numbers...)

	strings, err := lowerer.lowerString(members)
	if err != nil {
		var complexity *stringlanguage.ComplexityError
		if errors.As(err, &complexity) {
			return SetRef{}, nil, &ResourceError{
				Pass:     "lower string language",
				Resource: complexity.Resource,
				Pointer:  schema.Pointer,
				Limit:    complexity.Limit,
				Observed: complexity.Observed,
			}
		}

		return SetRef{}, nil, fmt.Errorf("lower schema at %s: %w", schema.Pointer, err)
	}

	refs = append(refs, strings...)

	containers, names, err := lowerer.lowerContainers(schema, members)
	if err != nil {
		return SetRef{}, nil, err
	}

	refs = append(refs, containers...)

	combined, err := lowerer.arena.Intersect(refs...)
	if err != nil {
		return SetRef{}, nil, err
	}

	return combined, names, nil
}

//nolint:cyclop // OpenAPI 3.0 has one closed scalar type enumeration.
func (lowerer *semanticLowerer) lowerKind(members map[string]json.RawMessage) ([]SetRef, error) {
	typeName, hasType, err := rawString(members, "type")
	if err != nil || !hasType {
		return nil, err
	}

	var kind jsonvalue.Kind

	refs := make([]SetRef, 0, initialKindRefCapacity)

	switch typeName {
	case "boolean":
		kind = jsonvalue.KindBoolean
	case "integer":
		kind = jsonvalue.KindNumber

		integer, atomErr := lowerer.arena.Atom(integerAtom{})
		if atomErr != nil {
			return nil, atomErr
		}

		refs = append(refs, integer)
	case "number":
		kind = jsonvalue.KindNumber
	case "string":
		kind = jsonvalue.KindString
	case "array":
		kind = jsonvalue.KindArray
	case "object":
		kind = jsonvalue.KindObject
	default:
		return nil, fmt.Errorf("unsupported admitted type %q", typeName)
	}

	kindRef, err := lowerer.arena.Atom(kindAtom{Kind: kind})
	if err != nil {
		return nil, err
	}

	nullable, _, err := rawBoolean(members, "nullable")
	if err != nil {
		return nil, err
	}

	if nullable {
		nullKind, atomErr := lowerer.arena.Atom(kindAtom{Kind: jsonvalue.KindNull})
		if atomErr != nil {
			return nil, atomErr
		}

		kindRef, err = lowerer.arena.Union(kindRef, nullKind)
		if err != nil {
			return nil, err
		}
	}

	return append([]SetRef{kindRef}, refs...), nil
}

func (lowerer *semanticLowerer) lowerEnum(members map[string]json.RawMessage) ([]SetRef, error) {
	raw, ok := members["enum"]
	if !ok {
		return nil, nil
	}

	var encoded []json.RawMessage
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, err
	}

	values := make([]jsonvalue.Value, len(encoded))
	for index := range encoded {
		value, err := jsonvalue.Parse(encoded[index])
		if err != nil {
			return nil, fmt.Errorf("member %d: %w", index, err)
		}

		values[index] = value
	}

	ref, err := lowerer.arena.Atom(enumAtom{Values: values})
	if err != nil {
		return nil, err
	}

	return []SetRef{ref}, nil
}

//nolint:cyclop // Exact numeric keywords and the closed format policy are independent.
func (lowerer *semanticLowerer) lowerNumber(members map[string]json.RawMessage) ([]SetRef, error) {
	minimum, err := rawNumber(members, "minimum")
	if err != nil {
		return nil, err
	}

	maximum, err := rawNumber(members, "maximum")
	if err != nil {
		return nil, err
	}

	exclusiveMinimum, _, err := rawBoolean(members, "exclusiveMinimum")
	if err != nil {
		return nil, err
	}

	exclusiveMaximum, _, err := rawBoolean(members, "exclusiveMaximum")
	if err != nil {
		return nil, err
	}

	refs := make([]SetRef, 0, initialNumberRefCapacity)

	if minimum != nil || maximum != nil {
		value, atomErr := lowerer.arena.Atom(numberRangeAtom{
			Minimum: minimum, ExclusiveMinimum: exclusiveMinimum,
			Maximum: maximum, ExclusiveMaximum: exclusiveMaximum,
		})
		if atomErr != nil {
			return nil, atomErr
		}

		refs = append(refs, value)
	}

	multiple, err := rawNumber(members, "multipleOf")
	if err != nil {
		return nil, err
	}

	if multiple != nil {
		value, atomErr := lowerer.arena.Atom(multipleOfAtom{Value: *multiple})
		if atomErr != nil {
			return nil, atomErr
		}

		refs = append(refs, value)
	}

	format, hasFormat, err := rawString(members, "format")
	if err != nil || !hasFormat {
		return refs, err
	}

	switch format {
	case "int32":
		return lowerer.appendIntegerFormat(refs, "-2147483648", "2147483647")
	case "int64":
		return lowerer.appendIntegerFormat(refs, "-9223372036854775808", "9223372036854775807")
	case "float":
		value, atomErr := lowerer.arena.Atom(floatFormatAtom{Bits: float32FormatBits})

		return append(refs, value), atomErr
	case "double":
		value, atomErr := lowerer.arena.Atom(floatFormatAtom{Bits: float64FormatBits})

		return append(refs, value), atomErr
	default:
		return refs, nil
	}
}

func (lowerer *semanticLowerer) appendIntegerFormat(
	refs []SetRef,
	minimum string,
	maximum string,
) ([]SetRef, error) {
	minValue, err := jsonvalue.ParseNumber(minimum)
	if err != nil {
		return nil, err
	}

	maxValue, err := jsonvalue.ParseNumber(maximum)
	if err != nil {
		return nil, err
	}

	integer, err := lowerer.arena.Atom(integerAtom{})
	if err != nil {
		return nil, err
	}

	ranged, err := lowerer.arena.Atom(numberRangeAtom{Minimum: new(minValue), Maximum: new(maxValue)})
	if err != nil {
		return nil, err
	}

	return append(refs, integer, ranged), nil
}

//nolint:cyclop // Length, pattern, and format are one exact string-language product.
func (lowerer *semanticLowerer) lowerString(members map[string]json.RawMessage) ([]SetRef, error) {
	minimum, err := rawNumber(members, "minLength")
	if err != nil {
		return nil, err
	}

	maximum, err := rawNumber(members, "maxLength")
	if err != nil {
		return nil, err
	}

	pattern, hasPattern, err := rawString(members, "pattern")
	if err != nil {
		return nil, err
	}

	format, hasFormat, err := rawString(members, "format")
	if err != nil {
		return nil, err
	}

	stringFormat := hasFormat && isStringFormat(format) && format != "password"
	if minimum == nil && maximum == nil && !hasPattern && !stringFormat {
		return nil, nil
	}

	requirements := make([]stringlanguage.Requirement, 0, initialStringRuleCapacity)

	if hasPattern {
		options := []patternvalidator.Option(nil)
		if lowerer.patternOption != nil {
			options = append(options, lowerer.patternOption)
		}

		language, compileErr := stringlanguage.Pattern(pattern, options...)
		if compileErr != nil {
			return nil, compileErr
		}

		requirements = append(requirements, stringlanguage.Requirement{Language: language, WantMatch: true})
	}

	if stringFormat {
		language, compileErr := stringlanguage.Format(format)
		if compileErr != nil {
			return nil, compileErr
		}

		requirements = append(requirements, stringlanguage.Requirement{Language: language, WantMatch: true})
	}

	length, err := stringLength(minimum, maximum)
	if err != nil {
		return nil, err
	}

	set, err := stringlanguage.Compile(requirements, length)
	if err != nil {
		if emptyStringLanguage(err) {
			return []SetRef{lowerer.arena.False()}, nil
		}

		return nil, err
	}

	key := fmt.Sprintf(
		"pattern=%t:%s;format=%t:%s;min=%s;max=%s",
		hasPattern, pattern, stringFormat, format, numberLexeme(minimum), numberLexeme(maximum),
	)

	ref, err := lowerer.arena.Atom(stringAtom{Key: key, Set: set})
	if err != nil {
		return nil, err
	}

	return []SetRef{ref}, nil
}

func stringLength(minimum *jsonvalue.Number, maximum *jsonvalue.Number) (stringlanguage.Length, error) {
	result := stringlanguage.Length{}

	if minimum != nil {
		value, err := countInt(*minimum)
		if err != nil {
			return stringlanguage.Length{}, fmt.Errorf("minLength: %w", err)
		}

		result.Min = value
	}

	if maximum != nil {
		value, err := countInt(*maximum)
		if err != nil {
			return stringlanguage.Length{}, fmt.Errorf("maxLength: %w", err)
		}

		result.Max = new(value)
	}

	return result, nil
}

func countInt(value jsonvalue.Number) (int, error) {
	if value.Rational == nil || !value.Rational.IsInt() {
		return 0, fmt.Errorf("exact count %s is not representable", value.Lexeme)
	}

	integer := value.Rational.Num()
	if !integer.IsInt64() {
		return 0, fmt.Errorf("exact count %s is not representable", value.Lexeme)
	}

	parsed := integer.Int64()

	converted := int(parsed)
	if int64(converted) != parsed || converted < 0 {
		return 0, fmt.Errorf("exact count %s is not representable", value.Lexeme)
	}

	return converted, nil
}

func isStringFormat(format string) bool {
	switch format {
	case "uuid", "uuidv4", "uuid-v4", "ipv4", "cidr", "ipv4-cidr", "email", "byte", "date", "date-time", "password":
		return true
	default:
		return false
	}
}

func numberLexeme(value *jsonvalue.Number) string {
	if value == nil {
		return ""
	}

	return value.Lexeme
}
