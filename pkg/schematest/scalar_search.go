package schematest

import (
	"crypto/sha256"
	"encoding/binary"
)

// walkActiveScalarPinAlternatives pins one complete scalar composition view.
func (s *search) walkActiveScalarPinAlternatives(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	visit func([]applicabilityPin) (bool, error),
) (bool, error) {
	anyOfNode, anyOfOccurrence, found := firstUnpinnedScalarAnyOf(node, occurrence, pins)
	if !found {
		return visit(pins)
	}

	for selected := range anyOfNode.anyOf {
		pathLength := len(pins)

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
			pins = append(pins, applicabilityPin{
				occurrence:  branchOccurrence,
				composition: "anyOf",
				branch:      branch,
				truth:       branch == selected,
				hasBranch:   true,
			})
		}

		complete, err := s.walkActiveScalarPinAlternatives(node, occurrence, pins, visit)
		pins = pins[:pathLength]

		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}

// firstUnpinnedScalarAnyOf finds the next composition choice in canonical order.
func firstUnpinnedScalarAnyOf(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
) (*schemaNode, schemaOccurrence, bool) {
	if len(node.anyOf) > 0 {
		states, pinned := rowCompositionTruthStates(pins, occurrence, "anyOf", len(node.anyOf))
		if !pinned {
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
			if foundNode, foundOccurrence, found := firstUnpinnedScalarAnyOf(
				child, childOccurrence, pins,
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
		if foundNode, foundOccurrence, found := firstUnpinnedScalarAnyOf(
			child, childOccurrence, pins,
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
