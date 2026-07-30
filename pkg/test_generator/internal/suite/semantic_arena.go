//nolint:godoclint // Private canonicalization helpers stay behind SetArena.
package suite

import (
	"fmt"
	"slices"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

// SetArena owns canonical immutable symbolic set nodes and exact atoms.
type SetArena struct {
	Nodes []setNode
	Atoms []atom

	atomIndex  map[string]AtomID
	nodeIndex  map[string]SetID
	emptyMemo  map[SetRef]bool
	emptyKnown map[SetRef]struct{}
}

// NewSetArena returns an arena whose node zero is false.
func NewSetArena() SetArena {
	return SetArena{
		Nodes:      []setNode{falseNode{}},
		atomIndex:  make(map[string]AtomID),
		nodeIndex:  make(map[string]SetID),
		emptyMemo:  make(map[SetRef]bool),
		emptyKnown: make(map[SetRef]struct{}),
	}
}

// False returns the empty set.
func (arena *SetArena) False() SetRef {
	return SetRef{}
}

// True returns the complete JSON universe.
func (arena *SetArena) True() SetRef {
	return Complement(arena.False())
}

// IsUniversal reports whether normalization reduced ref to the universe.
func (arena *SetArena) IsUniversal(ref SetRef) bool {
	return ref == arena.True()
}

// Atom interns one exact atomic requirement transactionally.
func (arena *SetArena) Atom(value atom) (SetRef, error) {
	if value == nil {
		return SetRef{}, fmt.Errorf("intern nil set atom")
	}

	key, err := value.key()
	if err != nil {
		return SetRef{}, err
	}

	if _, ok := arena.atomIndex[key]; ok {
		return SetRef{Node: arena.nodeIndex["atom:"+key]}, nil
	}

	identifier := AtomID(len(arena.Atoms))
	nodeIdentifier := SetID(len(arena.Nodes))

	arena.Atoms = append(arena.Atoms, value)
	arena.Nodes = append(arena.Nodes, atomNode{Atom: identifier})
	arena.atomIndex[key] = identifier
	arena.nodeIndex["atom:"+key] = nodeIdentifier

	return SetRef{Node: nodeIdentifier}, nil
}

// Intersect returns the canonical non-distributed intersection.
func (arena *SetArena) Intersect(refs ...SetRef) (SetRef, error) {
	return arena.combine(true, refs)
}

// Union returns the canonical non-distributed union.
func (arena *SetArena) Union(refs ...SetRef) (SetRef, error) {
	return arena.combine(false, refs)
}

//nolint:cyclop,funcorder,gocognit // Normalization stays beside public constructors.
func (arena *SetArena) combine(intersection bool, refs []SetRef) (SetRef, error) {
	children := make([]SetRef, 0, len(refs))
	for _, ref := range refs {
		if int(ref.Node) >= len(arena.Nodes) {
			return SetRef{}, fmt.Errorf("set reference %d is outside arena", ref.Node)
		}

		if intersection && ref == arena.False() || !intersection && ref == arena.True() {
			return ref, nil
		}

		if intersection && ref == arena.True() || !intersection && ref == arena.False() {
			continue
		}

		if !ref.Negated {
			switch node := arena.Nodes[ref.Node].(type) {
			case intersectionNode:
				if intersection {
					children = append(children, node.Children...)

					continue
				}
			case unionNode:
				if !intersection {
					children = append(children, node.Children...)

					continue
				}
			}
		}

		children = append(children, ref)
	}

	slices.SortFunc(children, compareSetRef)
	children = slices.Compact(children)

	for index := 1; index < len(children); index++ {
		if children[index-1].Node == children[index].Node &&
			children[index-1].Negated != children[index].Negated {
			if intersection {
				return arena.False(), nil
			}

			return arena.True(), nil
		}
	}

	if len(children) == 0 {
		if intersection {
			return arena.True(), nil
		}

		return arena.False(), nil
	}

	if len(children) == 1 {
		return children[0], nil
	}

	key := nodeKey(intersection, children)
	if identifier, ok := arena.nodeIndex[key]; ok {
		return SetRef{Node: identifier}, nil
	}

	immutable := append([]SetRef(nil), children...)

	identifier := SetID(len(arena.Nodes))
	if intersection {
		arena.Nodes = append(arena.Nodes, intersectionNode{Children: immutable})
	} else {
		arena.Nodes = append(arena.Nodes, unionNode{Children: immutable})
	}

	arena.nodeIndex[key] = identifier

	return SetRef{Node: identifier}, nil
}

func compareSetRef(left SetRef, right SetRef) int {
	if left.Node < right.Node {
		return -1
	}

	if left.Node > right.Node {
		return 1
	}

	if left.Negated == right.Negated {
		return 0
	}

	if !left.Negated {
		return -1
	}

	return 1
}

func nodeKey(intersection bool, children []SetRef) string {
	operator := "union"
	if intersection {
		operator = "intersect"
	}

	key := operator
	for _, child := range children {
		key += fmt.Sprintf(":%d:%t", child.Node, child.Negated)
	}

	return key
}

// Contains reports exact membership in ref.
func (arena *SetArena) Contains(ref SetRef, value jsonvalue.Value) (bool, error) {
	if int(ref.Node) >= len(arena.Nodes) {
		return false, fmt.Errorf("set reference %d is outside arena", ref.Node)
	}

	matches, err := arena.nodeMatches(ref.Node, value)
	if err != nil {
		return false, err
	}

	if ref.Negated {
		matches = !matches
	}

	return matches, nil
}

//nolint:cyclop // Exhaustive node variants belong beside membership traversal.
func (arena *SetArena) nodeMatches(identifier SetID, value jsonvalue.Value) (bool, error) {
	switch node := arena.Nodes[identifier].(type) {
	case falseNode:
		return false, nil
	case atomNode:
		return arena.Atoms[node.Atom].matches(arena, value)
	case intersectionNode:
		for _, child := range node.Children {
			matches, err := arena.Contains(child, value)
			if err != nil {
				return false, err
			}

			if !matches {
				return false, nil
			}
		}

		return true, nil
	case unionNode:
		for _, child := range node.Children {
			matches, err := arena.Contains(child, value)
			if err != nil {
				return false, err
			}

			if matches {
				return true, nil
			}
		}

		return false, nil
	default:
		return false, fmt.Errorf("set node %d has unknown type %T", identifier, node)
	}
}
