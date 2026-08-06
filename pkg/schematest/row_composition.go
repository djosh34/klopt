package schematest

import (
	"errors"
	"strings"
)

// rowSchemaSource identifies one composed child schema and its planner occurrence.
type rowSchemaSource struct {
	node       *schemaNode
	occurrence schemaOccurrence
}

// rowSchemaChoice is one schema used to construct a structural child value.
type rowSchemaChoice struct {
	node       *schemaNode
	occurrence schemaOccurrence
}

// rowChildKind identifies the structural child selected from a schema node.
type rowChildKind uint8

const (
	// rowChildItems selects an array item schema.
	rowChildItems rowChildKind = iota
	// rowChildProperty selects an object property schema.
	rowChildProperty
)

// rowChildSchemaChoices returns structural child alternatives for one target state.
func rowChildSchemaChoices(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	kind rowChildKind,
	name string,
) ([]rowSchemaChoice, error) {
	sets, err := rowChildSchemaSourceSets(node, occurrence, pins, kind, name)
	if err != nil {
		return nil, err
	}

	choices := make([]rowSchemaChoice, 0, len(sets))
	fallback := rowSchemaChoice{occurrence: rowChildOccurrence(node, occurrence, kind, name)}

	for _, sources := range sets {
		ordered := rowPreferredSchemaSources(sources, pins)

		choice, exists, err := mergeRowSchemaSources(ordered)
		if err != nil {
			return nil, err
		}

		if !exists {
			choice = fallback
		}

		choices = append(choices, choice)
	}

	if len(choices) == 0 {
		choices = append(choices, fallback)
	}

	return choices, nil
}

// rowChildSchemaSourceSets groups direct, allOf, and selected anyOf child schemas.
//
//nolint:cyclop // Direct, allOf, pinned anyOf, and alternative selection share one seam.
func rowChildSchemaSourceSets(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	kind rowChildKind,
	name string,
) ([][]rowSchemaSource, error) {
	if node == nil || node.schemaShape == nil {
		return nil, errors.New("schematest: composed row child has no shape")
	}

	common := make([]rowSchemaSource, 0, 1+len(node.allOf))
	if source, exists := rowChildSchemaSource(node, occurrence, kind, name); exists {
		common = append(common, source)
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if source, exists := rowChildSchemaSource(child, childOccurrence, kind, name); exists {
			common = append(common, source)
		}
	}

	if len(node.anyOf) == 0 {
		return [][]rowSchemaSource{common}, nil
	}

	states, pinned := rowCompositionTruthStates(pins, occurrence, "anyOf", len(node.anyOf))
	if pinned {
		selected := append([]rowSchemaSource(nil), common...)

		for index, child := range node.anyOf {
			if !states[index] {
				continue
			}

			childOccurrence := rebasePlanOccurrence(
				child,
				occurrence.usePointer+"/anyOf/"+itoa(index),
				occurrence.instanceTemplate,
			)
			if source, exists := rowChildSchemaSource(child, childOccurrence, kind, name); exists {
				selected = append(selected, source)
			}
		}

		return [][]rowSchemaSource{selected}, nil
	}

	alternatives := make([][]rowSchemaSource, 0, len(node.anyOf))
	for index, child := range node.anyOf {
		selected := append([]rowSchemaSource(nil), common...)

		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/anyOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if source, exists := rowChildSchemaSource(child, childOccurrence, kind, name); exists {
			selected = append(selected, source)
		}

		alternatives = append(alternatives, selected)
	}

	return alternatives, nil
}

// rowChildSchemaSource returns one direct child schema with its rebased occurrence.
func rowChildSchemaSource(
	node *schemaNode,
	occurrence schemaOccurrence,
	kind rowChildKind,
	name string,
) (rowSchemaSource, bool) {
	if node == nil || node.schemaShape == nil {
		return rowSchemaSource{}, false
	}

	switch kind {
	case rowChildItems:
		if node.items == nil {
			return rowSchemaSource{}, false
		}

		childOccurrence := rebasePlanOccurrence(
			node.items,
			occurrence.usePointer+"/items",
			appendInstanceToken(occurrence.instanceTemplate, "*"),
		)

		return rowSchemaSource{node: node.items, occurrence: childOccurrence}, true
	case rowChildProperty:
		property, exists := node.properties[name]
		if !exists || property == nil {
			return rowSchemaSource{}, false
		}

		childOccurrence := rebasePlanOccurrence(
			property,
			occurrence.usePointer+"/properties/"+escapePointerToken(name),
			appendInstanceToken(occurrence.instanceTemplate, name),
		)

		return rowSchemaSource{node: property, occurrence: childOccurrence}, true
	default:
		return rowSchemaSource{}, false
	}
}

// rowChildOccurrence supplies the direct structural child path used by generic values.
func rowChildOccurrence(
	node *schemaNode,
	occurrence schemaOccurrence,
	kind rowChildKind,
	name string,
) schemaOccurrence {
	if node != nil && node.schemaShape != nil {
		if source, exists := rowChildSchemaSource(node, occurrence, kind, name); exists {
			return source.occurrence
		}
	}

	if kind == rowChildItems {
		return schemaOccurrence{
			usePointer:       occurrence.usePointer + "/items",
			targetPointer:    occurrence.targetPointer,
			instanceTemplate: appendInstanceToken(occurrence.instanceTemplate, "*"),
		}
	}

	return schemaOccurrence{
		usePointer:       occurrence.usePointer + "/properties/" + escapePointerToken(name),
		targetPointer:    occurrence.targetPointer,
		instanceTemplate: appendInstanceToken(occurrence.instanceTemplate, name),
	}
}

// rowPreferredSchemaSources puts a target-pinned source first before composition merging.
func rowPreferredSchemaSources(sources []rowSchemaSource, pins []applicabilityPin) []rowSchemaSource {
	ordered := append([]rowSchemaSource(nil), sources...)

	for index, source := range ordered {
		preferred := false

		for _, pin := range pins {
			if (pin.hasKind || pin.presence != planPinNoPresence) &&
				rowOccurrenceMatches(pin.occurrence, source.occurrence) {
				preferred = true

				break
			}
		}

		if !preferred {
			continue
		}

		if index > 0 {
			selected := ordered[index]
			copy(ordered[1:index+1], ordered[0:index])
			ordered[0] = selected
		}

		break
	}

	return ordered
}

// mergeRowSchemaSources combines direct and simultaneously active composition schemas.
func mergeRowSchemaSources(sources []rowSchemaSource) (rowSchemaChoice, bool, error) {
	if len(sources) == 0 {
		return rowSchemaChoice{}, false, nil
	}

	for _, source := range sources {
		if source.node == nil || source.node.schemaShape == nil {
			return rowSchemaChoice{}, false, errors.New("schematest: composed row child has no shape")
		}
	}

	if len(sources) == 1 {
		return rowSchemaChoice{
			node:       sources[0].node,
			occurrence: sources[0].occurrence,
		}, true, nil
	}

	shape := *sources[0].node.schemaShape
	shape.allOf = append([]*schemaNode(nil), shape.allOf...)

	for _, source := range sources[1:] {
		shape.allOf = append(shape.allOf, source.node)
	}

	return rowSchemaChoice{
		node:       &schemaNode{schemaShape: &shape, occurrence: sources[0].occurrence},
		occurrence: sources[0].occurrence,
	}, true, nil
}

// composeRowMember merges all direct and active composed schemas for one object name.
//
//nolint:cyclop // Common-schema selection and inactive-branch alternatives are one phase.
func composeRowMember(name string, candidates []rowMember, required bool, pins []applicabilityPin) (rowMember, error) {
	common := make([]rowSchemaSource, 0, len(candidates))
	fallback := make([]rowMember, 0, len(candidates))

	for _, candidate := range candidates {
		active, constrained := rowMemberCompositionState(candidate, pins)
		if constrained && !active {
			fallback = append(fallback, candidate)

			continue
		}

		if candidate.node == nil {
			fallback = append(fallback, candidate)

			continue
		}

		common = append(common, rowSchemaSource{
			node:       candidate.node,
			occurrence: candidate.occurrence,
		})
	}

	if len(common) == 0 && len(fallback) == 0 {
		fallback = append(fallback, candidates...)
	}

	if len(common) > 0 {
		ordered := rowPreferredSchemaSources(common, pins)

		choice, _, err := mergeRowSchemaSources(ordered)
		if err != nil {
			return rowMember{}, err
		}

		member := rowMember{
			name:       name,
			node:       choice.node,
			occurrence: choice.occurrence,
			required:   required,
		}
		for _, alternative := range fallback {
			alternative.name = name
			alternative.required = required
			member.alternatives = append(member.alternatives, alternative)
		}

		return member, nil
	}

	if len(fallback) == 0 {
		return rowMember{name: name, required: required}, nil
	}

	member := fallback[0]
	member.name = name
	member.required = required

	for _, alternative := range fallback[1:] {
		alternative.name = name
		alternative.required = required
		member.alternatives = append(member.alternatives, alternative)
	}

	return member, nil
}

// rowMemberCompositionState identifies anyOf branch context for one property schema.
func rowMemberCompositionState(candidate rowMember, pins []applicabilityPin) (bool, bool) {
	active := false
	constrained := false

	for _, pin := range pins {
		if !pin.hasBranch || pin.composition != "anyOf" {
			continue
		}

		if rowBranchContainsOccurrence(pin, candidate.occurrence) {
			constrained = true

			if !pin.truth {
				return false, true
			}

			active = true

			continue
		}

		candidateParent, candidateNested := rowAnyOfParentUsePointer(candidate.occurrence.usePointer)

		pinParent := strings.TrimSuffix(pin.occurrence.usePointer, "/"+itoa(pin.branch))
		if candidateNested && candidateParent == pinParent &&
			instanceTemplateMatches(pin.occurrence.instanceTemplate, candidate.occurrence.instanceTemplate) {
			constrained = true
		}
	}

	return active, constrained
}

// rowAnyOfParentUsePointer returns the nearest authored anyOf parent path.
func rowAnyOfParentUsePointer(usePointer string) (string, bool) {
	index := strings.LastIndex(usePointer, "/anyOf/")
	if index < 0 || index+len("/anyOf/") >= len(usePointer) {
		return "", false
	}

	branchEnd := strings.IndexByte(usePointer[index+len("/anyOf/"):], '/')
	if branchEnd < 0 {
		branchEnd = len(usePointer) - (index + len("/anyOf/"))
	}

	return usePointer[:index], branchEnd > 0
}

// rowCompositionTruthStates reads branch pins using the pin template as wildcard pattern.
func rowCompositionTruthStates(
	pins []applicabilityPin,
	occurrence schemaOccurrence,
	composition string,
	count int,
) ([]bool, bool) {
	states := make([]bool, count)
	pinned := false

	for index := 0; index < count; index++ {
		branchUsePointer := occurrence.usePointer + "/" + composition + "/" + itoa(index)
		for _, pin := range pins {
			if !pin.hasBranch || pin.composition != composition || pin.branch != index ||
				pin.occurrence.usePointer != branchUsePointer ||
				!instanceTemplateMatches(pin.occurrence.instanceTemplate, occurrence.instanceTemplate) {
				continue
			}

			states[index] = pin.truth
			pinned = true

			break
		}
	}

	return states, pinned
}
