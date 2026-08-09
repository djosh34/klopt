package schematest

import (
	"crypto/sha256"
	"encoding/binary"
)

// walkActiveScalarRequirementAlternatives consumes the shared lazy same-instance projection.
func (s *search) walkActiveScalarRequirementAlternatives(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	visit func([]requirement) (bool, error),
) (bool, error) {
	cursor := newRowProjectionCursor(node, occurrence, requirements)
	defer cursor.Close()

	visited := false

	for {
		view, ok, err := cursor.Next()
		if err != nil {
			return false, err
		}

		if !ok {
			if !visited {
				return visit(requirements)
			}

			return false, nil
		}

		visited = true

		activeRequirements, err := view.appendBranchRequirements(
			append([]requirement(nil), requirements...),
			s.assign,
		)
		if err != nil {
			return false, err
		}

		complete, err := visit(activeRequirements)
		if err != nil || complete {
			return complete, err
		}
	}
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
