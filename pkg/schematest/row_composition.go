package schematest

import "strings"

// rowSchemaSource identifies one composed child schema and its planner occurrence.
type rowSchemaSource struct {
	node       *schemaNode
	occurrence schemaOccurrence
}

// rowItemSchemaSource returns one direct item schema with its rebased occurrence.
func rowItemSchemaSource(node *schemaNode, occurrence schemaOccurrence) (rowSchemaSource, bool) {
	if node == nil || node.schemaShape == nil || node.items == nil {
		return rowSchemaSource{}, false
	}

	childOccurrence := rebasePlanOccurrence(
		node.items,
		occurrence,
		occurrence.usePointer+"/items",
		appendInstanceToken(occurrence.instanceTemplate, "*"),
	)

	return rowSchemaSource{node: node.items, occurrence: childOccurrence}, true
}

// rowPropertySchemaSource returns one direct property schema with its rebased occurrence.
func rowPropertySchemaSource(
	node *schemaNode,
	occurrence schemaOccurrence,
	name string,
) (rowSchemaSource, bool) {
	if node == nil || node.schemaShape == nil {
		return rowSchemaSource{}, false
	}

	property, exists := node.properties[name]
	if !exists || property == nil {
		return rowSchemaSource{}, false
	}

	childOccurrence := rebasePlanOccurrence(
		property,
		occurrence,
		occurrence.usePointer+"/properties/"+escapePointerToken(name),
		appendInstanceToken(occurrence.instanceTemplate, name),
	)

	return rowSchemaSource{node: property, occurrence: childOccurrence}, true
}

// rowItemOccurrence supplies the item path used by generic values.
func rowItemOccurrence(node *schemaNode, occurrence schemaOccurrence) schemaOccurrence {
	if source, exists := rowItemSchemaSource(node, occurrence); exists {
		return source.occurrence
	}

	return schemaOccurrence{
		usePointer:       occurrence.usePointer + "/items",
		targetPointer:    occurrence.targetPointer,
		instanceTemplate: appendInstanceToken(occurrence.instanceTemplate, "*"),
	}
}

// rowPreferredSchemaSources puts a target-constrained source first before composition merging.
func rowPreferredSchemaSources(sources []rowSchemaSource, requirements []requirement) []rowSchemaSource {
	ordered := append([]rowSchemaSource(nil), sources...)

	for index, source := range ordered {
		preferred := false

		for _, requirement := range requirements {
			if (requirement.hasKind || requirement.presence != requirementNoPresence) &&
				rowOccurrenceMatches(requirement.occurrence, source.occurrence) {
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

// rowCompositionTruthStates reads branch requirements using the requirement template as wildcard pattern.
func rowCompositionTruthStates(
	requirements []requirement,
	occurrence schemaOccurrence,
	composition string,
	count int,
) ([]bool, bool) {
	states := make([]bool, count)
	constrained := false

	for index := 0; index < count; index++ {
		branchUsePointer := occurrence.usePointer + "/" + composition + "/" + itoa(index)
		for _, requirement := range requirements {
			if !requirement.hasBranch || requirement.composition != composition || requirement.branch != index ||
				requirement.occurrence.usePointer != branchUsePointer ||
				!instanceTemplateMatches(requirement.occurrence.instanceTemplate, occurrence.instanceTemplate) {
				continue
			}

			states[index] = requirement.truth
			constrained = true

			break
		}
	}

	return states, constrained
}
