package schematest

import (
	"errors"
	"fmt"
	"strings"
)

// rowVisit observes one complete candidate row while it is still transient.
type rowVisit func(*jsonValue) (bool, error)

// walkNode assigns one schema occurrence and recursively constructs a complete value.
//
//nolint:cyclop // Kind, direct-value, array, and object dispatch are one assignment phase.
func (s *search) walkNode(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	visit rowVisit,
) (bool, error) {
	if node == nil || node.schemaShape == nil {
		return false, errors.New("schematest: row schema occurrence has no shape")
	}

	if err := s.chargeNodeCompositions(node, occurrence, pins); err != nil {
		return false, err
	}

	kinds, err := rowKindChoices(node, occurrence, pins)
	if err != nil {
		return false, err
	}

	for _, kind := range kinds {
		if err := s.assign(); err != nil {
			return false, err
		}

		if kindIsScalar(kind) {
			complete, scalarErr := s.walkScalar(node, kind, visit)
			if scalarErr != nil || complete {
				return complete, scalarErr
			}

			continue
		}

		complete, err := s.walkDirectNodeValues(node, kind, visit)
		if err != nil || complete {
			return complete, err
		}

		switch kind {
		case jsonArray:
			complete, err = s.walkArray(node, occurrence, pins, visit)
		case jsonObject:
			complete, err = s.walkObject(node, occurrence, pins, visit)
		default:
			return false, fmt.Errorf("schematest: unsupported row kind %d", kind)
		}

		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}

// chargeNodeCompositions charges each pinned local composition choice once per occurrence.
func (s *search) chargeNodeCompositions(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
) error {
	if len(node.allOf) > 0 && rowHasCompositionPins(pins, occurrence, "allOf") {
		if err := s.assign(); err != nil {
			return err
		}
	}

	if len(node.anyOf) > 0 && rowHasCompositionPins(pins, occurrence, "anyOf") {
		if err := s.assign(); err != nil {
			return err
		}
	}

	return nil
}

// rowKindChoices returns one pinned kind or the planner's canonical kind order.
func rowKindChoices(node *schemaNode, occurrence schemaOccurrence, pins []applicabilityPin) ([]jsonKind, error) {
	var pinned *jsonKind

	for _, pin := range pins {
		if !pin.hasKind || !rowOccurrenceMatches(pin.occurrence, occurrence) {
			continue
		}

		if pinned != nil && *pinned != pin.kind {
			return nil, fmt.Errorf("schematest: conflicting kind pins at %s", occurrence.usePointer)
		}

		kind := pin.kind
		pinned = &kind
	}

	if pinned != nil {
		if !nodeAcceptsKindForTarget(node, *pinned) {
			return nil, nil
		}

		return []jsonKind{*pinned}, nil
	}

	return orderedTypeKinds(node), nil
}

// kindIsScalar reports whether a kind gets a scalar witness search.
func kindIsScalar(kind jsonKind) bool {
	switch kind {
	case jsonNull, jsonBoolean, jsonNumber, jsonString:
		return true
	default:
		return false
	}
}

// walkDirectNodeValues tries enum and default values before structural repair.
func (s *search) walkDirectNodeValues(node *schemaNode, kind jsonKind, visit rowVisit) (bool, error) {
	candidates, err := rowDirectValues(node, kind)
	if err != nil {
		return false, err
	}

	for _, candidate := range candidates {
		if err := s.assign(); err != nil {
			return false, err
		}

		complete, visitErr := visit(candidate)
		if visitErr != nil || complete {
			return complete, visitErr
		}
	}

	return false, nil
}

// rowDirectValues returns authored enum or default values for composite kinds.
func rowDirectValues(node *schemaNode, kind jsonKind) ([]*jsonValue, error) {
	candidates := make([]*jsonValue, 0)

	if node.enum != nil {
		for _, candidate := range node.enum {
			if candidate == nil {
				return nil, errors.New("schematest: nil enum row value")
			}

			if candidate.kind != kind {
				continue
			}

			var err error

			candidates, err = appendUniqueJSONWitness(candidates, candidate)
			if err != nil {
				return nil, err
			}
		}

		return candidates, nil
	}

	if node.defaultValue != nil && node.defaultValue.kind == kind {
		var err error

		candidates, err = appendUniqueJSONWitness(candidates, node.defaultValue)
		if err != nil {
			return nil, err
		}
	}

	return candidates, nil
}

// rowChildValueUsable prunes locally invalid children unless a pinned anyOf branch must be false.
func rowChildValueUsable(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	value *jsonValue,
) (bool, error) {
	result := evaluateNode(node, value, occurrence)
	if result.err != nil {
		return false, result.err
	}

	if result.valid {
		return true, nil
	}

	for _, pin := range pins {
		if !pin.hasBranch || pin.composition != "anyOf" || pin.truth ||
			!rowBranchContainsOccurrence(pin, occurrence) {
			continue
		}

		return true, nil
	}

	return false, nil
}

// rowBranchContainsOccurrence reports whether a pinned composition branch contains one child.
func rowBranchContainsOccurrence(pin applicabilityPin, occurrence schemaOccurrence) bool {
	prefix := pin.occurrence.usePointer
	if occurrence.usePointer != prefix && !strings.HasPrefix(occurrence.usePointer, prefix+"/") {
		return false
	}

	return rowInstancePrefixMatches(pin.occurrence.instanceTemplate, occurrence.instanceTemplate)
}

// rowInstancePrefixMatches matches a planner instance prefix to one concrete child path.
func rowInstancePrefixMatches(prefix, value string) bool {
	prefixTokens, prefixOK := rowPointerTokens(prefix)

	valueTokens, valueOK := rowPointerTokens(value)
	if !prefixOK || !valueOK || len(prefixTokens) > len(valueTokens) {
		return false
	}

	for index, token := range prefixTokens {
		if token != "*" && token != valueTokens[index] {
			return false
		}
	}

	return true
}

// rowHasCompositionPins reports whether one local composition has an explicit target state.
func rowHasCompositionPins(pins []applicabilityPin, occurrence schemaOccurrence, composition string) bool {
	prefix := occurrence.usePointer + "/" + composition + "/"
	for _, pin := range pins {
		if pin.hasBranch && pin.composition == composition &&
			len(pin.occurrence.usePointer) > len(prefix) &&
			pin.occurrence.usePointer[:len(prefix)] == prefix &&
			instanceTemplateMatches(pin.occurrence.instanceTemplate, occurrence.instanceTemplate) {
			return true
		}
	}

	return false
}
