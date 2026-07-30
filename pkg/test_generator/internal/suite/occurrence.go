//nolint:godoclint // Private parent-slot variants are exhaustive occurrence vocabulary.
package suite

// OccurrenceID identifies one authored schema use, independently of set interning.
type OccurrenceID uint32

type parentSlot interface {
	isParentSlot()
}

type rootSlot struct{}

type allOfSlot struct {
	Parent OccurrenceID
	Index  uint32
}

type anyOfSlot struct {
	Parent OccurrenceID
	Index  uint32
}

type itemsSlot struct {
	Parent OccurrenceID
}

type propertySlot struct {
	Parent OccurrenceID
	Name   string
}

type additionalSlot struct {
	Parent OccurrenceID
}

func (rootSlot) isParentSlot()       {}
func (allOfSlot) isParentSlot()      {}
func (anyOfSlot) isParentSlot()      {}
func (itemsSlot) isParentSlot()      {}
func (propertySlot) isParentSlot()   {}
func (additionalSlot) isParentSlot() {}

// PropertyOccurrence preserves one named property use.
type PropertyOccurrence struct {
	Name  string
	Child OccurrenceID
}

// Occurrence preserves authored source identity and exact parent context.
type Occurrence struct {
	Pointer string
	Keyword string
	Full    SetRef
	Reach   SetRef

	WithoutOwnAnyOf SetRef
	AllOf           []OccurrenceID
	AnyOf           []OccurrenceID
	Items           *OccurrenceID
	Properties      []PropertyOccurrence
	Additional      *OccurrenceID
	Parent          parentSlot

	base          SetRef
	propertyNames []string
	constraints   []localConstraint
}

type localConstraint struct {
	keyword     string
	withoutBase SetRef
	boundary    SetRef
	hasBoundary bool
}

// SemanticProgram is the S2 handoff: one occurrence tree and one canonical arena.
type SemanticProgram struct {
	Root        OccurrenceID
	Occurrences []Occurrence
	Sets        SetArena
}
