//nolint:godoclint // Private direct-lowering vocabulary is local to the S2 transaction.
package suite

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // S2 exact string semantics are owned here.
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/validation"
)

type semanticLowerer struct {
	source        oas.Source
	arena         SetArena
	occurrences   []Occurrence
	active        map[string]struct{}
	patternOption patternvalidator.Option
}

func compileSemantic(admitted *validation.AdmittedRequestSchema) (*semanticCompilation, error) {
	return compileSemanticWithPattern(admitted, nil)
}

func compileSemanticWithPattern(
	admitted *validation.AdmittedRequestSchema,
	patternOption patternvalidator.Option,
) (*semanticCompilation, error) {
	if admitted == nil {
		return nil, errors.New("admitted request schema must not be nil")
	}

	lowerer := semanticLowerer{
		source:        admitted.Source,
		arena:         NewSetArena(),
		active:        make(map[string]struct{}),
		patternOption: patternOption,
	}

	root, err := lowerer.lower(admitted.Root, rootSlot{}, "schema")
	if err != nil {
		return nil, err
	}

	semantic := SemanticProgram{
		Root:        root,
		Occurrences: lowerer.occurrences,
		Sets:        lowerer.arena,
	}

	for identifier := range semantic.Occurrences {
		reach, reachErr := occurrenceReach(&semantic, OccurrenceID(identifier))
		if reachErr != nil {
			return nil, reachErr
		}

		semantic.Occurrences[identifier].Reach = reach
	}

	cases, err := planCases(&semantic)
	if err != nil {
		return nil, err
	}

	return &semanticCompilation{Semantic: semantic, Cases: cases}, nil
}

func (lowerer *semanticLowerer) lower(
	authored oas.LocatedSchema,
	parent parentSlot,
	keyword string,
) (OccurrenceID, error) {
	resolved, err := lowerer.source.Resolve(authored)
	if err != nil {
		return 0, fmt.Errorf("resolve schema at %s: %w", authored.Pointer, err)
	}

	if _, recursive := lowerer.active[resolved.Pointer]; recursive {
		return 0, fmt.Errorf("lower schema at %s: recursive schema is unsupported", resolved.Pointer)
	}

	members, err := rawObject(resolved)
	if err != nil {
		return 0, err
	}

	lowerer.active[resolved.Pointer] = struct{}{}
	defer delete(lowerer.active, resolved.Pointer)

	identifier := OccurrenceID(len(lowerer.occurrences))
	lowerer.occurrences = append(lowerer.occurrences, Occurrence{
		Pointer: resolved.Pointer,
		Keyword: keyword,
		Parent:  parent,
		Reach:   lowerer.arena.True(),
	})

	base, names, err := lowerer.lowerLocal(resolved, members)
	if err != nil {
		return 0, err
	}

	occurrence := &lowerer.occurrences[identifier]
	occurrence.base = base
	occurrence.propertyNames = names

	constraints, err := lowerer.localConstraints(resolved, members)
	if err != nil {
		return 0, err
	}

	occurrence.constraints = constraints

	if childErr := lowerer.lowerChildren(identifier, resolved, members); childErr != nil {
		return 0, childErr
	}

	full, err := lowerer.compose(identifier, nil, SetRef{}, false)
	if err != nil {
		return 0, err
	}

	withoutAnyOf, err := lowerer.compose(identifier, nil, SetRef{}, true)
	if err != nil {
		return 0, err
	}

	occurrence = &lowerer.occurrences[identifier]
	occurrence.Full = full

	occurrence.WithoutOwnAnyOf = withoutAnyOf
	if len(occurrence.AnyOf) != 0 {
		occurrence.Keyword = "anyOf"
	}

	return identifier, nil
}

//nolint:cyclop,gocognit // Authored child kinds are traversed once in deterministic source order.
func (lowerer *semanticLowerer) lowerChildren(
	identifier OccurrenceID,
	schema oas.LocatedSchema,
	members map[string]json.RawMessage,
) error {
	if raw, ok := members["allOf"]; ok {
		var children []json.RawMessage
		if err := json.Unmarshal(raw, &children); err != nil {
			return fmt.Errorf("lower schema at %s/allOf: %w", schema.Pointer, err)
		}

		for index := range children {
			child, err := lowerer.source.Child(schema, "allOf", strconv.Itoa(index))
			if err != nil {
				return err
			}

			childID, err := lowerer.lower(
				child,
				allOfSlot{Parent: identifier, Index: uint32(index)},
				fmt.Sprintf("allOf/%d", index),
			)
			if err != nil {
				return err
			}

			lowerer.occurrences[identifier].AllOf = append(lowerer.occurrences[identifier].AllOf, childID)
		}
	}

	if raw, ok := members["anyOf"]; ok {
		var children []json.RawMessage
		if err := json.Unmarshal(raw, &children); err != nil {
			return fmt.Errorf("lower schema at %s/anyOf: %w", schema.Pointer, err)
		}

		for index := range children {
			child, err := lowerer.source.Child(schema, "anyOf", strconv.Itoa(index))
			if err != nil {
				return err
			}

			childID, err := lowerer.lower(
				child,
				anyOfSlot{Parent: identifier, Index: uint32(index)},
				fmt.Sprintf("anyOf/%d", index),
			)
			if err != nil {
				return err
			}

			lowerer.occurrences[identifier].AnyOf = append(lowerer.occurrences[identifier].AnyOf, childID)
		}
	}

	if _, ok := members["items"]; ok {
		child, err := lowerer.source.Child(schema, "items")
		if err != nil {
			return err
		}

		childID, err := lowerer.lower(child, itemsSlot{Parent: identifier}, "items")
		if err != nil {
			return err
		}

		lowerer.occurrences[identifier].Items = new(childID)
	}

	if raw, ok := members["properties"]; ok {
		var properties map[string]json.RawMessage
		if err := json.Unmarshal(raw, &properties); err != nil {
			return fmt.Errorf("lower schema at %s/properties: %w", schema.Pointer, err)
		}

		for _, name := range slices.Sorted(maps.Keys(properties)) {
			child, err := lowerer.source.Child(schema, "properties", name)
			if err != nil {
				return err
			}

			childID, err := lowerer.lower(child, propertySlot{Parent: identifier, Name: name}, "property")
			if err != nil {
				return err
			}

			lowerer.occurrences[identifier].Properties = append(
				lowerer.occurrences[identifier].Properties,
				PropertyOccurrence{Name: name, Child: childID},
			)
		}
	}

	//nolint:nestif // Boolean and schema forms have distinct admitted lowering paths.
	if raw, ok := members["additionalProperties"]; ok {
		trimmed := bytes.TrimSpace(raw)
		if !bytes.Equal(trimmed, []byte("true")) && !bytes.Equal(trimmed, []byte("false")) {
			child, err := lowerer.source.Child(schema, "additionalProperties")
			if err != nil {
				return err
			}

			childID, err := lowerer.lower(child, additionalSlot{Parent: identifier}, "additionalProperties")
			if err != nil {
				return err
			}

			lowerer.occurrences[identifier].Additional = new(childID)
		}
	}

	return nil
}

func rawObject(schema oas.LocatedSchema) (map[string]json.RawMessage, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(schema.Raw, &members); err != nil || members == nil {
		return nil, fmt.Errorf("lower schema at %s: Schema Object must be an object", schema.Pointer)
	}

	return members, nil
}

func rawNumber(members map[string]json.RawMessage, keyword string) (*jsonvalue.Number, error) {
	raw, ok := members[keyword]
	if !ok {
		return nil, nil //nolint:nilnil // Nil means this admitted optional keyword is absent.
	}

	value, err := jsonvalue.ParseNumber(string(bytes.TrimSpace(raw)))
	if err != nil {
		return nil, fmt.Errorf("lower keyword %s: %w", keyword, err)
	}

	return new(value), nil
}

func rawBoolean(members map[string]json.RawMessage, keyword string) (bool, bool, error) {
	raw, ok := members[keyword]
	if !ok {
		return false, false, nil
	}

	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, true, fmt.Errorf("lower keyword %s: %w", keyword, err)
	}

	return value, true, nil
}

func rawString(members map[string]json.RawMessage, keyword string) (string, bool, error) {
	raw, ok := members[keyword]
	if !ok {
		return "", false, nil
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", true, fmt.Errorf("lower keyword %s: %w", keyword, err)
	}

	return value, true, nil
}

func emptyStringLanguage(err error) bool {
	var empty *stringlanguage.EmptyError

	return errors.As(err, &empty)
}
