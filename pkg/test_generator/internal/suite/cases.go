// Package suite compiles exact request-schema case obligations.
package suite

import "fmt"

// ExpectedResult is the validator verdict required by one eventual executable case.
type ExpectedResult uint8

const (
	// ExpectAccepted requires both validators to accept.
	ExpectAccepted ExpectedResult = iota
	// ExpectRejected requires both validators to reject.
	ExpectRejected
)

// ConstraintSource identifies the authored obligation source.
type ConstraintSource struct {
	Pointer string
	Keyword string
}

// caseSpec is one exact semantic obligation awaiting S3 graph compilation.
type caseSpec struct {
	name   string
	expect ExpectedResult
	source ConstraintSource
	values SetRef
}

// semanticCompilation is the package-private atomic S2 result.
type semanticCompilation struct {
	Semantic SemanticProgram
	Cases    []caseSpec
}

// planCases creates all non-empty semantic obligations in source order.
//
//nolint:cyclop,gocognit // Aggregate, ordinary, and anyOf obligations share one source-order pass.
func planCases(semantic *SemanticProgram) ([]caseSpec, error) {
	arena := &semantic.Sets
	root := semantic.Occurrences[semantic.Root]
	cases := make([]caseSpec, 0)

	empty, err := arena.IsEmpty(root.Full)
	if err != nil {
		return nil, err
	}

	if !empty {
		cases = append(cases, caseSpec{
			name:   "valid root",
			expect: ExpectAccepted,
			source: ConstraintSource{Pointer: root.Pointer, Keyword: "schema"},
			values: root.Full,
		})
	}

	for identifier := range semantic.Occurrences {
		occurrence := &semantic.Occurrences[identifier]
		for _, constraint := range occurrence.constraints {
			validRoot := root.Full
			if constraint.hasBoundary {
				validRoot, err = rebuildWithLocalRequirement(
					semantic,
					OccurrenceID(identifier),
					constraint.boundary,
				)
				if err != nil {
					return nil, err
				}
			}

			valid, intersectErr := arena.Intersect(validRoot, occurrence.Reach, root.Full)
			if intersectErr != nil {
				return nil, intersectErr
			}

			empty, err = arena.IsEmpty(valid)
			if err != nil {
				return nil, err
			}

			pointer := occurrence.Pointer + "/" + constraint.keyword
			if !empty {
				cases = append(cases, caseSpec{
					name:   "valid " + constraint.keyword + " at " + pointer,
					expect: ExpectAccepted,
					source: ConstraintSource{Pointer: pointer, Keyword: constraint.keyword},
					values: valid,
				})
			}

			without, rebuildErr := rebuildWithoutConstraint(
				semantic,
				OccurrenceID(identifier),
				constraint,
			)
			if rebuildErr != nil {
				return nil, rebuildErr
			}

			invalid, intersectErr := arena.Intersect(
				without,
				occurrence.Reach,
				Complement(root.Full),
			)
			if intersectErr != nil {
				return nil, intersectErr
			}

			empty, err = arena.IsEmpty(invalid)
			if err != nil {
				return nil, err
			}

			if !empty {
				cases = append(cases, caseSpec{
					name:   "invalid " + constraint.keyword + " at " + pointer,
					expect: ExpectRejected,
					source: ConstraintSource{Pointer: pointer, Keyword: constraint.keyword},
					values: invalid,
				})
			}
		}
	}

	for identifier := range semantic.Occurrences {
		occurrence := &semantic.Occurrences[identifier]
		if len(occurrence.AnyOf) == 0 {
			continue
		}

		pointer := occurrence.Pointer + "/anyOf"

		valid, intersectErr := arena.Intersect(root.Full, occurrence.Reach)
		if intersectErr != nil {
			return nil, intersectErr
		}

		empty, err = arena.IsEmpty(valid)
		if err != nil {
			return nil, err
		}

		if !empty {
			cases = append(cases, caseSpec{
				name:   "valid anyOf at " + pointer,
				expect: ExpectAccepted,
				source: ConstraintSource{Pointer: pointer, Keyword: "anyOf"},
				values: valid,
			})
		}

		without, rebuildErr := rebuildWithoutAnyOf(semantic, OccurrenceID(identifier))
		if rebuildErr != nil {
			return nil, fmt.Errorf("rebuild %s without anyOf: %w", occurrence.Pointer, rebuildErr)
		}

		invalid, intersectErr := arena.Intersect(without, occurrence.Reach, Complement(root.Full))
		if intersectErr != nil {
			return nil, intersectErr
		}

		empty, err = arena.IsEmpty(invalid)
		if err != nil {
			return nil, err
		}

		if !empty {
			cases = append(cases, caseSpec{
				name:   "invalid anyOf at " + pointer,
				expect: ExpectRejected,
				source: ConstraintSource{Pointer: pointer, Keyword: "anyOf"},
				values: invalid,
			})
		}
	}

	return cases, nil
}
