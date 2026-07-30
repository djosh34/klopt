//nolint:godoclint // Private ancestor rebuilding is observed through semantic obligations.
package suite

import (
	"fmt"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

func (lowerer *semanticLowerer) compose(
	identifier OccurrenceID,
	overrideChild *OccurrenceID,
	override SetRef,
	disableOwnAnyOf bool,
) (SetRef, error) {
	return lowerer.composeWithBase(
		identifier,
		overrideChild,
		override,
		disableOwnAnyOf,
		lowerer.occurrences[identifier].base,
	)
}

//nolint:cyclop // One occurrence composition explicitly handles every child slot kind.
func (lowerer *semanticLowerer) composeWithBase(
	identifier OccurrenceID,
	overrideChild *OccurrenceID,
	override SetRef,
	disableOwnAnyOf bool,
	base SetRef,
) (SetRef, error) {
	occurrence := lowerer.occurrences[identifier]
	refs := []SetRef{base}

	childRef := func(child OccurrenceID) SetRef {
		if overrideChild != nil && child == *overrideChild {
			return override
		}

		return lowerer.occurrences[child].Full
	}

	for _, child := range occurrence.AllOf {
		refs = append(refs, childRef(child))
	}

	if len(occurrence.AnyOf) != 0 && !disableOwnAnyOf {
		alternatives := make([]SetRef, len(occurrence.AnyOf))
		for index, child := range occurrence.AnyOf {
			alternatives[index] = childRef(child)
		}

		choice, err := lowerer.arena.Union(alternatives...)
		if err != nil {
			return SetRef{}, err
		}

		refs = append(refs, choice)
	}

	if occurrence.Items != nil {
		items, err := lowerer.arena.Atom(arrayItemsAtom{Values: childRef(*occurrence.Items)})
		if err != nil {
			return SetRef{}, err
		}

		refs = append(refs, items)
	}

	for _, property := range occurrence.Properties {
		values, err := lowerer.arena.Atom(propertyValuesAtom{
			Name: property.Name, Values: childRef(property.Child),
		})
		if err != nil {
			return SetRef{}, err
		}

		refs = append(refs, values)
	}

	if occurrence.Additional != nil {
		additional, err := lowerer.arena.Atom(additionalPropertyValuesAtom{
			Names: occurrence.propertyNames, Values: childRef(*occurrence.Additional),
		})
		if err != nil {
			return SetRef{}, err
		}

		refs = append(refs, additional)
	}

	return lowerer.arena.Intersect(refs...)
}

func rebuildWithoutAnyOf(semantic *SemanticProgram, target OccurrenceID) (SetRef, error) {
	if int(target) >= len(semantic.Occurrences) {
		return SetRef{}, fmt.Errorf("occurrence %d is outside semantic program", target)
	}

	replacement := semantic.Occurrences[target].WithoutOwnAnyOf

	return rebuildFromOccurrence(semantic, target, replacement)
}

func rebuildWithoutConstraint(
	semantic *SemanticProgram,
	target OccurrenceID,
	constraint localConstraint,
) (SetRef, error) {
	lowerer := semanticLowerer{arena: semantic.Sets, occurrences: semantic.Occurrences}

	replacement, err := lowerer.composeWithBase(target, nil, SetRef{}, false, constraint.withoutBase)
	if err != nil {
		return SetRef{}, err
	}

	semantic.Sets = lowerer.arena

	return rebuildFromOccurrence(semantic, target, replacement)
}

func rebuildWithLocalRequirement(
	semantic *SemanticProgram,
	target OccurrenceID,
	requirement SetRef,
) (SetRef, error) {
	replacement, err := semantic.Sets.Intersect(semantic.Occurrences[target].Full, requirement)
	if err != nil {
		return SetRef{}, err
	}

	return rebuildFromOccurrence(semantic, target, replacement)
}

func rebuildFromOccurrence(
	semantic *SemanticProgram,
	target OccurrenceID,
	replacement SetRef,
) (SetRef, error) {
	child := target

	for {
		parent, root, err := parentOf(semantic.Occurrences[child].Parent)
		if err != nil {
			return SetRef{}, err
		}

		if root {
			return replacement, nil
		}

		lowerer := semanticLowerer{arena: semantic.Sets, occurrences: semantic.Occurrences}

		replacement, err = lowerer.compose(parent, new(child), replacement, false)
		if err != nil {
			return SetRef{}, err
		}

		semantic.Sets = lowerer.arena
		semantic.Occurrences = lowerer.occurrences
		child = parent
	}
}

//nolint:cyclop,gocognit // Exact activation ascends each exhaustive parent-slot variant.
func occurrenceReach(semantic *SemanticProgram, target OccurrenceID) (SetRef, error) {
	if int(target) >= len(semantic.Occurrences) {
		return SetRef{}, fmt.Errorf("occurrence %d is outside semantic program", target)
	}

	reach := semantic.Sets.True()
	child := target

	for {
		slot := semantic.Occurrences[child].Parent

		parent, root, err := parentOf(slot)
		if err != nil {
			return SetRef{}, err
		}

		if root {
			return reach, nil
		}

		switch value := slot.(type) {
		case propertySlot:
			objectKind, atomErr := semantic.Sets.Atom(kindAtom{Kind: jsonvalue.KindObject})
			if atomErr != nil {
				return SetRef{}, atomErr
			}

			required, atomErr := semantic.Sets.Atom(requiredPropertyAtom{Name: value.Name})
			if atomErr != nil {
				return SetRef{}, atomErr
			}

			property, atomErr := semantic.Sets.Atom(propertyValuesAtom{Name: value.Name, Values: reach})
			if atomErr != nil {
				return SetRef{}, atomErr
			}

			reach, err = semantic.Sets.Intersect(objectKind, required, property)
		case itemsSlot:
			arrayKind, atomErr := semantic.Sets.Atom(kindAtom{Kind: jsonvalue.KindArray})
			if atomErr != nil {
				return SetRef{}, atomErr
			}

			some, atomErr := semantic.Sets.Atom(arraySomeItemsAtom{Values: reach})
			if atomErr != nil {
				return SetRef{}, atomErr
			}

			reach, err = semantic.Sets.Intersect(arrayKind, some)
		case additionalSlot:
			parentOccurrence := semantic.Occurrences[parent]

			objectKind, atomErr := semantic.Sets.Atom(kindAtom{Kind: jsonvalue.KindObject})
			if atomErr != nil {
				return SetRef{}, atomErr
			}

			some, atomErr := semantic.Sets.Atom(additionalSomePropertyAtom{
				Names: parentOccurrence.propertyNames, Values: reach,
			})
			if atomErr != nil {
				return SetRef{}, atomErr
			}

			reach, err = semantic.Sets.Intersect(objectKind, some)
		case allOfSlot, anyOfSlot:
			// Composition children constrain the same JSON value and need no path activation.
		default:
			return SetRef{}, fmt.Errorf("occurrence %d has unknown parent slot %T", child, slot)
		}

		if err != nil {
			return SetRef{}, err
		}

		child = parent
	}
}

func parentOf(slot parentSlot) (OccurrenceID, bool, error) {
	switch value := slot.(type) {
	case rootSlot:
		return 0, true, nil
	case allOfSlot:
		return value.Parent, false, nil
	case anyOfSlot:
		return value.Parent, false, nil
	case itemsSlot:
		return value.Parent, false, nil
	case propertySlot:
		return value.Parent, false, nil
	case additionalSlot:
		return value.Parent, false, nil
	default:
		return 0, false, fmt.Errorf("unknown parent slot %T", slot)
	}
}
