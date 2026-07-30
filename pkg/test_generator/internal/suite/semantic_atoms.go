//nolint:godoclint // Exact private atom variants are exhaustive SetArena vocabulary.
package suite

import (
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // S2's exact shared string-language seam.
	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type atom interface {
	key() (string, error)
	matches(arena *SetArena, value jsonvalue.Value) (bool, error)
}

type kindAtom struct {
	Kind jsonvalue.Kind
}

func (value kindAtom) key() (string, error) {
	return fmt.Sprintf("kind:%d", value.Kind), nil
}

func (value kindAtom) matches(_ *SetArena, candidate jsonvalue.Value) (bool, error) {
	return candidate.Kind == value.Kind, nil
}

type enumAtom struct {
	Values []jsonvalue.Value
}

func (value enumAtom) key() (string, error) {
	parts := make([]string, len(value.Values))
	for index := range value.Values {
		hash, err := value.Values[index].Hash()
		if err != nil {
			return "", fmt.Errorf("hash enum member %d: %w", index, err)
		}

		parts[index] = hex.EncodeToString(hash[:])
	}

	slices.Sort(parts)
	parts = slices.Compact(parts)

	return "enum:" + strings.Join(parts, "\x00"), nil
}

func (value enumAtom) matches(_ *SetArena, candidate jsonvalue.Value) (bool, error) {
	for _, allowed := range value.Values {
		if candidate.Equal(allowed) {
			return true, nil
		}
	}

	return false, nil
}

type integerAtom struct{}

func (integerAtom) key() (string, error) {
	return "number:integer", nil
}

func (integerAtom) matches(_ *SetArena, candidate jsonvalue.Value) (bool, error) {
	return candidate.Kind != jsonvalue.KindNumber || candidate.Number.IsInteger(), nil
}

type numberRangeAtom struct {
	Minimum          *jsonvalue.Number
	ExclusiveMinimum bool
	Maximum          *jsonvalue.Number
	ExclusiveMaximum bool
}

func (value numberRangeAtom) key() (string, error) {
	minimum := ""
	if value.Minimum != nil {
		minimum = value.Minimum.Lexeme
	}

	maximum := ""
	if value.Maximum != nil {
		maximum = value.Maximum.Lexeme
	}

	return fmt.Sprintf(
		"number:range:%s:%t:%s:%t",
		minimum,
		value.ExclusiveMinimum,
		maximum,
		value.ExclusiveMaximum,
	), nil
}

func (value numberRangeAtom) matches(_ *SetArena, candidate jsonvalue.Value) (bool, error) {
	if candidate.Kind != jsonvalue.KindNumber {
		return true, nil
	}

	if value.Minimum != nil {
		comparison := candidate.Number.Compare(*value.Minimum)
		if comparison < 0 || comparison == 0 && value.ExclusiveMinimum {
			return false, nil
		}
	}

	if value.Maximum != nil {
		comparison := candidate.Number.Compare(*value.Maximum)
		if comparison > 0 || comparison == 0 && value.ExclusiveMaximum {
			return false, nil
		}
	}

	return true, nil
}

type multipleOfAtom struct {
	Value jsonvalue.Number
}

type floatFormatAtom struct {
	Bits int
}

func (value floatFormatAtom) key() (string, error) {
	return fmt.Sprintf("number:float:%d", value.Bits), nil
}

func (value floatFormatAtom) matches(_ *SetArena, candidate jsonvalue.Value) (bool, error) {
	if candidate.Kind != jsonvalue.KindNumber {
		return true, nil
	}

	_, err := strconv.ParseFloat(candidate.Number.Lexeme, value.Bits)

	return err == nil, nil
}

func (value multipleOfAtom) key() (string, error) {
	return "number:multipleOf:" + value.Value.Lexeme, nil
}

func (value multipleOfAtom) matches(_ *SetArena, candidate jsonvalue.Value) (bool, error) {
	return candidate.Kind != jsonvalue.KindNumber || candidate.Number.IsMultipleOf(value.Value), nil
}

type stringAtom struct {
	Key string
	Set *stringlanguage.Set
}

func (value stringAtom) key() (string, error) {
	if value.Set == nil {
		return "", fmt.Errorf("string atom %q has no language", value.Key)
	}

	return "string:" + value.Key, nil
}

func (value stringAtom) matches(_ *SetArena, candidate jsonvalue.Value) (bool, error) {
	return candidate.Kind != jsonvalue.KindString || value.Set.Matches(candidate.String), nil
}

type arrayLengthAtom struct {
	Minimum *jsonvalue.Number
	Maximum *jsonvalue.Number
}

func (value arrayLengthAtom) key() (string, error) {
	return countKey("array:length", value.Minimum, value.Maximum), nil
}

func (value arrayLengthAtom) matches(_ *SetArena, candidate jsonvalue.Value) (bool, error) {
	return candidate.Kind != jsonvalue.KindArray ||
		countMatches(len(candidate.Array), value.Minimum, value.Maximum), nil
}

type arrayItemsAtom struct {
	Values SetRef
}

func (value arrayItemsAtom) key() (string, error) {
	return fmt.Sprintf("array:items:%d:%t", value.Values.Node, value.Values.Negated), nil
}

func (value arrayItemsAtom) matches(arena *SetArena, candidate jsonvalue.Value) (bool, error) {
	if candidate.Kind != jsonvalue.KindArray {
		return true, nil
	}

	for _, item := range candidate.Array {
		matches, err := arena.Contains(value.Values, item)
		if err != nil {
			return false, err
		}

		if !matches {
			return false, nil
		}
	}

	return true, nil
}

type arraySomeItemsAtom struct {
	Values SetRef
}

func (value arraySomeItemsAtom) key() (string, error) {
	return fmt.Sprintf("array:someItems:%d:%t", value.Values.Node, value.Values.Negated), nil
}

func (value arraySomeItemsAtom) matches(arena *SetArena, candidate jsonvalue.Value) (bool, error) {
	if candidate.Kind != jsonvalue.KindArray {
		return true, nil
	}

	for _, item := range candidate.Array {
		matches, err := arena.Contains(value.Values, item)
		if err != nil {
			return false, err
		}

		if matches {
			return true, nil
		}
	}

	return false, nil
}

type objectCountAtom struct {
	Minimum *jsonvalue.Number
	Maximum *jsonvalue.Number
}

func (value objectCountAtom) key() (string, error) {
	return countKey("object:count", value.Minimum, value.Maximum), nil
}

func (value objectCountAtom) matches(_ *SetArena, candidate jsonvalue.Value) (bool, error) {
	return candidate.Kind != jsonvalue.KindObject ||
		countMatches(len(candidate.Object), value.Minimum, value.Maximum), nil
}

type requiredPropertyAtom struct {
	Name string
}

func (value requiredPropertyAtom) key() (string, error) {
	return "object:required:" + value.Name, nil
}

func (value requiredPropertyAtom) matches(_ *SetArena, candidate jsonvalue.Value) (bool, error) {
	if candidate.Kind != jsonvalue.KindObject {
		return true, nil
	}

	_, found := objectProperty(candidate, value.Name)

	return found, nil
}

type allowedPropertyNamesAtom struct {
	Names []string
}

func (value allowedPropertyNamesAtom) key() (string, error) {
	names := append([]string(nil), value.Names...)
	slices.Sort(names)
	names = slices.Compact(names)

	return "object:allowed:" + encodeStrings(names), nil
}

func (value allowedPropertyNamesAtom) matches(_ *SetArena, candidate jsonvalue.Value) (bool, error) {
	if candidate.Kind != jsonvalue.KindObject {
		return true, nil
	}

	for _, member := range candidate.Object {
		if !slices.Contains(value.Names, member.Name) {
			return false, nil
		}
	}

	return true, nil
}

type propertyValuesAtom struct {
	Name   string
	Values SetRef
}

func (value propertyValuesAtom) key() (string, error) {
	return fmt.Sprintf("object:property:%q:%d:%t", value.Name, value.Values.Node, value.Values.Negated), nil
}

func (value propertyValuesAtom) matches(arena *SetArena, candidate jsonvalue.Value) (bool, error) {
	if candidate.Kind != jsonvalue.KindObject {
		return true, nil
	}

	property, found := objectProperty(candidate, value.Name)
	if !found {
		return true, nil
	}

	return arena.Contains(value.Values, property)
}

type additionalPropertyValuesAtom struct {
	Names  []string
	Values SetRef
}

func (value additionalPropertyValuesAtom) key() (string, error) {
	names := append([]string(nil), value.Names...)
	slices.Sort(names)
	names = slices.Compact(names)

	return fmt.Sprintf(
		"object:additional:%s:%d:%t",
		encodeStrings(names),
		value.Values.Node,
		value.Values.Negated,
	), nil
}

func (value additionalPropertyValuesAtom) matches(arena *SetArena, candidate jsonvalue.Value) (bool, error) {
	if candidate.Kind != jsonvalue.KindObject {
		return true, nil
	}

	for _, member := range candidate.Object {
		if slices.Contains(value.Names, member.Name) {
			continue
		}

		matches, err := arena.Contains(value.Values, member.Value)
		if err != nil {
			return false, err
		}

		if !matches {
			return false, nil
		}
	}

	return true, nil
}

type additionalSomePropertyAtom struct {
	Names  []string
	Values SetRef
}

func (value additionalSomePropertyAtom) key() (string, error) {
	names := append([]string(nil), value.Names...)
	slices.Sort(names)
	names = slices.Compact(names)

	return fmt.Sprintf(
		"object:someAdditional:%s:%d:%t",
		encodeStrings(names),
		value.Values.Node,
		value.Values.Negated,
	), nil
}

func (value additionalSomePropertyAtom) matches(arena *SetArena, candidate jsonvalue.Value) (bool, error) {
	if candidate.Kind != jsonvalue.KindObject {
		return true, nil
	}

	for _, member := range candidate.Object {
		if slices.Contains(value.Names, member.Name) {
			continue
		}

		matches, err := arena.Contains(value.Values, member.Value)
		if err != nil {
			return false, err
		}

		if matches {
			return true, nil
		}
	}

	return false, nil
}

func objectProperty(value jsonvalue.Value, name string) (jsonvalue.Value, bool) {
	for _, member := range value.Object {
		if member.Name == name {
			return member.Value, true
		}
	}

	return jsonvalue.Value{}, false
}

func countKey(prefix string, minimum *jsonvalue.Number, maximum *jsonvalue.Number) string {
	minimumLexeme := ""
	if minimum != nil {
		minimumLexeme = minimum.Lexeme
	}

	maximumLexeme := ""
	if maximum != nil {
		maximumLexeme = maximum.Lexeme
	}

	return prefix + ":" + minimumLexeme + ":" + maximumLexeme
}

func encodeStrings(values []string) string {
	result := ""
	for _, value := range values {
		result += strconv.Itoa(len(value)) + ":" + value
	}

	return result
}

func countMatches(count int, minimum *jsonvalue.Number, maximum *jsonvalue.Number) bool {
	exact, err := jsonvalue.ParseNumber(fmt.Sprintf("%d", count))
	if err != nil {
		panic(fmt.Sprintf("encode in-memory count: %v", err))
	}

	return (minimum == nil || exact.Compare(*minimum) >= 0) &&
		(maximum == nil || exact.Compare(*maximum) <= 0)
}
