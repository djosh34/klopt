package schematest

import (
	"crypto/sha256"
	"encoding/binary"
)

// walkActiveScalarRequirementAlternatives requirements one complete scalar composition view.
func (s *search) walkActiveScalarRequirementAlternatives(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	visit func([]requirement) (bool, error),
) (bool, error) {
	anyOfNode, anyOfOccurrence, found := firstUnconstrainedScalarAnyOf(node, occurrence, requirements)
	if !found {
		return visit(requirements)
	}

	for selected := range anyOfNode.anyOf {
		pathLength := len(requirements)

		for branch, child := range anyOfNode.anyOf {
			if err := s.assign(); err != nil {
				return false, err
			}

			branchOccurrence := rebasePlanOccurrence(
				child,
				anyOfOccurrence,
				anyOfOccurrence.usePointer+"/anyOf/"+itoa(branch),
				anyOfOccurrence.instanceTemplate,
			)
			requirements = append(requirements, requirement{
				tag:         requirementBranchTruth,
				occurrence:  branchOccurrence,
				composition: "anyOf",
				branch:      branch,
				truth:       branch == selected,
				hasBranch:   true,
			})
		}

		complete, err := s.walkActiveScalarRequirementAlternatives(node, occurrence, requirements, visit)
		requirements = requirements[:pathLength]

		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}

// firstUnconstrainedScalarAnyOf finds the next composition choice in canonical order.
func firstUnconstrainedScalarAnyOf(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
) (*schemaNode, schemaOccurrence, bool) {
	if len(node.anyOf) > 0 {
		states, constrained := rowCompositionTruthStates(requirements, occurrence, "anyOf", len(node.anyOf))
		if !constrained {
			return node, occurrence, true
		}

		for index, child := range node.anyOf {
			if !states[index] {
				continue
			}

			childOccurrence := rebasePlanOccurrence(
				child,
				occurrence,
				occurrence.usePointer+"/anyOf/"+itoa(index),
				occurrence.instanceTemplate,
			)
			if foundNode, foundOccurrence, found := firstUnconstrainedScalarAnyOf(
				child, childOccurrence, requirements,
			); found {
				return foundNode, foundOccurrence, true
			}
		}
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if foundNode, foundOccurrence, found := firstUnconstrainedScalarAnyOf(
			child, childOccurrence, requirements,
		); found {
			return foundNode, foundOccurrence, true
		}
	}

	return nil, schemaOccurrence{}, false
}

// searchSeed derives the private deterministic scalar-search seed.
func searchSeed(schemaPointer string, canonicalSchemaJSON []byte, rule, level string) uint64 {
	input := []byte("schematest-v1\x00")
	input = append(input, schemaPointer...)
	input = append(input, 0)
	input = append(input, canonicalSchemaJSON...)
	input = append(input, 0)
	input = append(input, rule...)
	input = append(input, 0)
	input = append(input, level...)
	digest := sha256.Sum256(input)

	return binary.BigEndian.Uint64(digest[:8])
}
