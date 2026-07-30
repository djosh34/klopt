//nolint:godoclint // Private constraint-rebuild vocabulary is local to semantic planning.
package suite

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // Boundaries reuse the exact string-language module.
	"github.com/djosh34/klopt/pkg/jsonvalue"
)

var ordinaryKeywords = []string{
	"type", "enum",
	"minimum", "exclusiveMinimum", "maximum", "exclusiveMaximum", "multipleOf", "format",
	"minLength", "maxLength", "pattern",
	"minItems", "maxItems",
	"minProperties", "maxProperties", "required", "additionalProperties",
}

func (lowerer *semanticLowerer) localConstraints(
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
) ([]localConstraint, error) {
	result := make([]localConstraint, 0)

	for _, keyword := range ordinaryKeywords {
		if !restrictsLocally(members, keyword) {
			continue
		}

		withoutMembers := make(map[string]json.RawMessage, len(members)-1)
		for name, raw := range members {
			if name != keyword {
				withoutMembers[name] = raw
			}
		}

		withoutBase, _, err := lowerer.lowerLocal(schema, withoutMembers)
		if err != nil {
			return nil, fmt.Errorf("lower schema at %s without %s: %w", schema.Pointer, keyword, err)
		}

		boundary, hasBoundary, err := lowerer.constraintBoundary(members, keyword)
		if err != nil {
			return nil, fmt.Errorf("lower schema at %s/%s boundary: %w", schema.Pointer, keyword, err)
		}

		result = append(result, localConstraint{
			keyword: keyword, withoutBase: withoutBase, boundary: boundary, hasBoundary: hasBoundary,
		})
	}

	return result, nil
}

//nolint:cyclop // Each admitted OpenAPI keyword has one explicit capability rule.
func restrictsLocally(members map[string]json.RawMessage, keyword string) bool {
	raw, ok := members[keyword]
	if !ok {
		return false
	}

	switch keyword {
	case "exclusiveMinimum":
		value, _, err := rawBoolean(members, keyword)

		return err == nil && value && members["minimum"] != nil
	case "exclusiveMaximum":
		value, _, err := rawBoolean(members, keyword)

		return err == nil && value && members["maximum"] != nil
	case "format":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return true
		}

		return value != "password"
	case "additionalProperties":
		return bytes.Equal(bytes.TrimSpace(raw), []byte("false"))
	default:
		return true
	}
}

//nolint:cyclop // Boundary construction is an explicit keyword matrix.
func (lowerer *semanticLowerer) constraintBoundary(
	members map[string]json.RawMessage,
	keyword string,
) (SetRef, bool, error) {
	switch keyword {
	case "type":
		refs, err := lowerer.lowerKind(members)
		if err != nil {
			return SetRef{}, false, err
		}

		boundary, err := lowerer.arena.Intersect(refs...)

		return boundary, true, err
	case "enum":
		refs, err := lowerer.lowerEnum(members)
		if err != nil {
			return SetRef{}, false, err
		}

		return refs[0], true, nil
	case "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum":
		boundKeyword := keyword
		if keyword == "exclusiveMinimum" {
			boundKeyword = "minimum"
		}

		if keyword == "exclusiveMaximum" {
			boundKeyword = "maximum"
		}

		value, err := rawNumber(members, boundKeyword)
		if err != nil {
			return SetRef{}, false, err
		}

		return lowerer.literalBoundary(jsonvalue.KindNumber, jsonvalue.Value{
			Kind: jsonvalue.KindNumber, Number: *value,
		})
	case "multipleOf":
		zero, err := jsonvalue.ParseNumber("0")
		if err != nil {
			return SetRef{}, false, err
		}

		return lowerer.literalBoundary(jsonvalue.KindNumber, jsonvalue.Value{
			Kind: jsonvalue.KindNumber, Number: zero,
		})
	case "minLength", "maxLength":
		return lowerer.stringCountBoundary(members, keyword)
	case "minItems", "maxItems":
		return lowerer.collectionBoundary(members, keyword, jsonvalue.KindArray)
	case "minProperties", "maxProperties":
		return lowerer.collectionBoundary(members, keyword, jsonvalue.KindObject)
	case "pattern", "format":
		kind, err := lowerer.arena.Atom(kindAtom{Kind: jsonvalue.KindString})

		return kind, true, err
	default:
		return SetRef{}, false, nil
	}
}

func (lowerer *semanticLowerer) literalBoundary(
	kind jsonvalue.Kind,
	value jsonvalue.Value,
) (SetRef, bool, error) {
	kindRef, err := lowerer.arena.Atom(kindAtom{Kind: kind})
	if err != nil {
		return SetRef{}, false, err
	}

	literal, err := lowerer.arena.Atom(enumAtom{Values: []jsonvalue.Value{value}})
	if err != nil {
		return SetRef{}, false, err
	}

	boundary, err := lowerer.arena.Intersect(kindRef, literal)

	return boundary, true, err
}

func (lowerer *semanticLowerer) stringCountBoundary(
	members map[string]json.RawMessage,
	keyword string,
) (SetRef, bool, error) {
	count, err := rawNumber(members, keyword)
	if err != nil {
		return SetRef{}, false, err
	}

	exact, err := countInt(*count)
	if err != nil {
		return SetRef{}, false, err
	}

	set, err := stringlanguage.Compile(nil, stringlanguage.Length{Min: exact, Max: new(exact)})
	if err != nil {
		return SetRef{}, false, err
	}

	strings, err := lowerer.arena.Atom(stringAtom{
		Key: fmt.Sprintf("exact-length:%d", exact), Set: set,
	})
	if err != nil {
		return SetRef{}, false, err
	}

	kind, err := lowerer.arena.Atom(kindAtom{Kind: jsonvalue.KindString})
	if err != nil {
		return SetRef{}, false, err
	}

	boundary, err := lowerer.arena.Intersect(kind, strings)

	return boundary, true, err
}

func (lowerer *semanticLowerer) collectionBoundary(
	members map[string]json.RawMessage,
	keyword string,
	kind jsonvalue.Kind,
) (SetRef, bool, error) {
	count, err := rawNumber(members, keyword)
	if err != nil {
		return SetRef{}, false, err
	}

	var countRef SetRef
	if kind == jsonvalue.KindArray {
		countRef, err = lowerer.arena.Atom(arrayLengthAtom{Minimum: count, Maximum: count})
	} else {
		countRef, err = lowerer.arena.Atom(objectCountAtom{Minimum: count, Maximum: count})
	}

	if err != nil {
		return SetRef{}, false, err
	}

	kindRef, err := lowerer.arena.Atom(kindAtom{Kind: kind})
	if err != nil {
		return SetRef{}, false, err
	}

	boundary, err := lowerer.arena.Intersect(kindRef, countRef)

	return boundary, true, err
}
