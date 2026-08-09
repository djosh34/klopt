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
	requirements []requirement,
	context rowSearchContext,
	visit rowVisit,
) (bool, error) {
	if node == nil || node.schemaShape == nil {
		return false, errors.New("schematest: row schema occurrence has no shape")
	}

	if err := s.chargeNodeCompositions(node, occurrence, requirements); err != nil {
		return false, err
	}

	kinds, err := rowKindChoices(node, occurrence, requirements)
	if err != nil {
		return false, err
	}

	for _, kind := range kinds {
		if assignErr := s.assign(); assignErr != nil {
			return false, assignErr
		}

		if kindIsScalar(kind) {
			complete, scalarErr := s.walkScalar(node, occurrence, requirements, context, kind, visit)
			if scalarErr != nil || complete {
				return complete, scalarErr
			}

			continue
		}

		var complete bool
		if kind != jsonArray {
			complete, err = s.walkDirectNodeValues(node, kind, visit)
			if err != nil || complete {
				return complete, err
			}
		}

		switch kind {
		case jsonArray:
			complete, err = s.walkArray(node, occurrence, requirements, context, visit)
		case jsonObject:
			complete, err = s.walkObject(node, occurrence, requirements, context, visit)
		default:
			return false, fmt.Errorf("schematest: unsupported row kind %d", kind)
		}

		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}

// chargeNodeCompositions charges each constrained local composition choice once per occurrence.
func (s *search) chargeNodeCompositions(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
) error {
	return s.chargeNestedCompositions(node, occurrence, requirements, make(map[*schemaNode]bool))
}

// chargeNestedCompositions charges same-instance compositions before structural assignment.
//
//nolint:cyclop // Authored allOf and anyOf paths share one recursive charge pass.
func (s *search) chargeNestedCompositions(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	visiting map[*schemaNode]bool,
) error {
	if node == nil || node.schemaShape == nil {
		return errors.New("schematest: row composition has no shape")
	}

	if visiting[node] {
		return fmt.Errorf("schematest: recursive row composition at %s", occurrence.usePointer)
	}

	visiting[node] = true
	defer delete(visiting, node)

	if len(node.allOf) > 0 && rowHasCompositionRequirements(requirements, occurrence, "allOf") {
		if err := s.assign(); err != nil {
			return err
		}
	}

	if len(node.anyOf) > 0 && rowHasCompositionRequirements(requirements, occurrence, "anyOf") {
		if err := s.assign(); err != nil {
			return err
		}
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if err := s.chargeNestedCompositions(child, childOccurrence, requirements, visiting); err != nil {
			return err
		}
	}

	for index, child := range node.anyOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/anyOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if err := s.chargeNestedCompositions(child, childOccurrence, requirements, visiting); err != nil {
			return err
		}
	}

	return nil
}

// rowKindChoices returns one constrained kind or the planner's canonical kind order.
func rowKindChoices(node *schemaNode, occurrence schemaOccurrence, requirements []requirement) ([]jsonKind, error) {
	var constrained *jsonKind

	for _, requirement := range requirements {
		if !requirement.hasKind || !rowOccurrenceMatches(requirement.occurrence, occurrence) {
			continue
		}

		if constrained != nil && *constrained != requirement.kind {
			return nil, fmt.Errorf("schematest: conflicting kind requirements at %s", occurrence.usePointer)
		}

		kind := requirement.kind
		constrained = &kind
	}

	if constrained != nil {
		if !nodeAcceptsKindForTarget(node, *constrained) {
			return nil, nil
		}

		return []jsonKind{*constrained}, nil
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

// walkDirectNodeValues tries authored enum or default values before structural repair.
func (s *search) walkDirectNodeValues(node *schemaNode, kind jsonKind, visit rowVisit) (bool, error) {
	var (
		complete bool
		visitErr error
	)

	err := directNodeValueSource(node, kind)(func(candidate *jsonValue) bool {
		if visitErr = s.assign(); visitErr != nil {
			return false
		}

		complete, visitErr = visit(candidate)

		return visitErr == nil && !complete
	})
	if err != nil {
		return false, err
	}

	return complete, visitErr
}

// directNodeValueSource yields authored composite enum or default values in place.
func directNodeValueSource(node *schemaNode, kind jsonKind) jsonValueSource {
	return func(yield func(*jsonValue) bool) error {
		if node.enum != nil {
			for _, member := range node.enum {
				if member.value == nil {
					return errors.New("schematest: nil enum row value")
				}

				if member.value.kind == kind && !yield(member.value) {
					return nil
				}
			}

			return nil
		}

		if node.defaultValue != nil && node.defaultValue.kind == kind {
			yield(node.defaultValue)
		}

		return nil
	}
}

// rowChildValueUsable prunes locally invalid children unless a constrained anyOf branch must be false.
func (s *search) rowChildValueUsable(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	value *jsonValue,
) (bool, error) {
	result := evaluateNode(node, value, occurrence)
	if result.err != nil {
		return false, result.err
	}

	if result.valid {
		return true, nil
	}

	for _, requirement := range requirements {
		if !requirement.hasBranch || requirement.composition != "anyOf" || requirement.truth ||
			!rowBranchContainsOccurrence(requirement, occurrence) {
			continue
		}

		return true, nil
	}

	return false, nil
}

// rowBranchContainsOccurrence reports whether a constrained composition branch contains one child.
func rowBranchContainsOccurrence(requirement requirement, occurrence schemaOccurrence) bool {
	prefix := requirement.occurrence.usePointer
	if occurrence.usePointer != prefix && !strings.HasPrefix(occurrence.usePointer, prefix+"/") {
		return false
	}

	return rowInstancePrefixMatches(requirement.occurrence.instanceTemplate, occurrence.instanceTemplate)
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

// rowHasCompositionRequirements reports whether one local composition has an explicit target state.
func rowHasCompositionRequirements(requirements []requirement, occurrence schemaOccurrence, composition string) bool {
	prefix := occurrence.usePointer + "/" + composition + "/"
	for _, requirement := range requirements {
		if requirement.hasBranch && requirement.composition == composition &&
			len(requirement.occurrence.usePointer) > len(prefix) &&
			requirement.occurrence.usePointer[:len(prefix)] == prefix &&
			instanceTemplateMatches(requirement.occurrence.instanceTemplate, occurrence.instanceTemplate) {
			return true
		}
	}

	return false
}
