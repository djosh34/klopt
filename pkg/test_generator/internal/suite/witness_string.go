//nolint:godoclint // Exact string witness construction stays behind graph lowering.
package suite

import (
	"errors"

	"github.com/djosh34/klopt/pkg/internal/program"        //nolint:depguard // Shortest exact string completion uses Program.Decode.
	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // Exact string residuals use the shared opaque language.
	"github.com/djosh34/klopt/pkg/jsonvalue"
)

//nolint:mnd // Candidate sampling remains beside its exact language product.
func (finder *valueFinder) stringValues(assignment map[AtomID]bool) ([]jsonvalue.Value, error) {
	set, err := finder.combinedStringSet(assignment)
	if err != nil {
		var empty *stringlanguage.EmptyError
		if errors.As(err, &empty) {
			return nil, nil
		}

		return nil, err
	}

	var builder program.Builder

	root, err := set.AppendTo(&builder)
	if err != nil {
		return nil, err
	}

	sealed, err := builder.Seal(root, builder.UniformSampling())
	if err != nil {
		return nil, err
	}

	value, err := sealed.Decode(nil, program.Limits{
		MaxSteps: 1_000_000, MaxOutputBytes: 8_000_000, MaxDepth: 1,
	})
	if err != nil {
		return nil, err
	}

	result := []jsonvalue.Value{value}
	for _, candidate := range []string{
		"", "a", "aa", "b", "0", "test@example.com",
		"00000000-0000-0000-0000-000000000000", "127.0.0.1", "2024-01-01",
	} {
		if set.Matches(candidate) && candidate != value.String {
			result = append(result, jsonvalue.String(candidate))
		}
	}

	return result, nil
}

func (finder *valueFinder) combinedStringSet(
	assignment map[AtomID]bool,
) (*stringlanguage.Set, error) {
	requirements := make([]stringlanguage.SetRequirement, 0)

	for identifier, want := range assignment {
		switch value := finder.arena.Atoms[identifier].(type) {
		case stringAtom:
			requirements = append(requirements, stringlanguage.SetRequirement{
				Set: value.Set, WantMatch: want,
			})
		case enumAtom:
			if want {
				continue
			}

			for _, excluded := range value.Values {
				if excluded.Kind != jsonvalue.KindString {
					continue
				}

				language, err := stringlanguage.Literal(excluded.String)
				if err != nil {
					return nil, err
				}

				set, err := stringlanguage.Compile(
					[]stringlanguage.Requirement{{Language: language, WantMatch: true}},
					stringlanguage.Length{},
				)
				if err != nil {
					return nil, err
				}

				requirements = append(requirements, stringlanguage.SetRequirement{
					Set: set, WantMatch: false,
				})
			}
		}
	}

	return stringlanguage.Combine(requirements)
}
