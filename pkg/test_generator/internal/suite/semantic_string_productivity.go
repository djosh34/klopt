//nolint:godoclint // Private string-theory helpers stay behind SetArena.IsEmpty.
package suite

import (
	"errors"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // Exact signed string products are shared.
	"github.com/djosh34/klopt/pkg/jsonvalue"
)

//nolint:cyclop // Signed string and enum facts become one stringlanguage product.
func (arena *SetArena) stringAssignmentProductive(assignment map[AtomID]bool) (bool, error) {
	requirements := make([]stringlanguage.SetRequirement, 0)

	for identifier, want := range assignment {
		switch value := arena.Atoms[identifier].(type) {
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
					return false, err
				}

				set, err := stringlanguage.Compile(
					[]stringlanguage.Requirement{{Language: language, WantMatch: true}},
					stringlanguage.Length{},
				)
				if err != nil {
					return false, err
				}

				requirements = append(requirements, stringlanguage.SetRequirement{
					Set: set, WantMatch: false,
				})
			}
		}
	}

	set, err := stringlanguage.Combine(requirements)
	if err == nil {
		return set != nil, nil
	}

	var empty *stringlanguage.EmptyError
	if errors.As(err, &empty) {
		return false, nil
	}

	return false, err
}
