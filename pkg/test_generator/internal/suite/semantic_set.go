//nolint:godoclint // Exact private node variants are exhaustive SetArena vocabulary.
package suite

// SetID identifies one immutable interned set node.
type SetID uint32

// AtomID identifies one immutable interned exact atom.
type AtomID uint32

// SetRef identifies a set node and its complement polarity.
type SetRef struct {
	Node    SetID
	Negated bool
}

type setNode interface {
	isSetNode()
}

type falseNode struct{}

type atomNode struct {
	Atom AtomID
}

type intersectionNode struct {
	Children []SetRef
}

type unionNode struct {
	Children []SetRef
}

func (falseNode) isSetNode()        {}
func (atomNode) isSetNode()         {}
func (intersectionNode) isSetNode() {}
func (unionNode) isSetNode()        {}

// Complement returns the exact set complement without allocating a node.
func Complement(ref SetRef) SetRef {
	ref.Negated = !ref.Negated

	return ref
}
