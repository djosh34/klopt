// Package suite compiles OpenAPI schemas into canonical constructive domains.
package suite

import (
	"fmt"

	"decode_and_validate_generator/pkg/test_generator/internal/jsonvalue"
	"pgregory.net/rapid"
)

// DomainID identifies one canonical Domain in a DomainRegistry.
type DomainID uint32

const (
	// NoDomain is returned when compilation did not produce a Domain.
	NoDomain DomainID = iota
	// AnyJSONDomainID identifies the Domain containing every JSON value.
	AnyJSONDomainID
	// EmptyDomainID identifies the Domain containing no JSON value.
	EmptyDomainID
)

// KindState says whether a JSON kind is absent, unconstrained, or constrained.
type KindState uint8

const (
	// KindExcluded means no value of the kind is reachable.
	KindExcluded KindState = iota
	// KindUnrestricted means every value of the kind is reachable.
	KindUnrestricted
	// KindRestricted means the kind's constraint set applies.
	KindRestricted
)

// DomainStatus describes whether a Domain can construct values.
type DomainStatus uint8

const (
	// DomainProductive contains at least one constructible JSON value.
	DomainProductive DomainStatus = iota
	// DomainEmpty contains no JSON value.
	DomainEmpty
	// DomainUnsupported uses understood OpenAPI behavior not implemented by this step.
	DomainUnsupported
	// DomainUnconstructible is understood but lacks enough trusted generation input.
	DomainUnconstructible
)

// Domain is one constructible set spanning all JSON value kinds.
type Domain struct {
	Null    KindState
	Boolean KindState
	Number  NumberConstraints
	String  StringConstraints
	Array   ArrayConstraints
	Object  ObjectConstraints
	Enum    *EnumSet
	Status  DomainStatus
}

// NumberBound is one exact inclusive or exclusive number bound.
type NumberBound struct {
	Value     jsonvalue.Number
	Exclusive bool
}

// NumberConstraints constrains JSON numbers.
type NumberConstraints struct {
	State        KindState
	IntegersOnly bool
	Minimum      *NumberBound
	Maximum      *NumberBound
	MultipleOf   *jsonvalue.Number
	Format       *string
}

// StringConstraints constrains JSON strings.
type StringConstraints struct {
	State     KindState
	MinLength int
	MaxLength *int
	Patterns  []string
	Formats   []string
}

// ArrayConstraints constrains JSON arrays and references their item Domain.
type ArrayConstraints struct {
	State    KindState
	Items    DomainID
	MinItems int
	MaxItems *int
}

// PropertyState says whether a named object property may occur.
type PropertyState uint8

const (
	// PropertyAllowed permits the property with Values.
	PropertyAllowed PropertyState = iota
	// PropertyForbidden rejects the property.
	PropertyForbidden
)

// NamedProperty constrains one object property.
type NamedProperty struct {
	Name     string
	Required bool
	State    PropertyState
	Values   DomainID
}

// AdditionalProperties constrains object names without a NamedProperty.
type AdditionalProperties struct {
	Values DomainID
}

// ObjectConstraints constrains JSON objects and references property Domains.
type ObjectConstraints struct {
	State      KindState
	Properties []NamedProperty
	Additional AdditionalProperties
	MinProps   int
	MaxProps   *int
}

// EnumSet is a canonical finite set of exact semantic JSON values.
type EnumSet struct {
	Values []jsonvalue.Value
}

// ExpectedResult is the validator result expected for a CasePlan.
type ExpectedResult uint8

const (
	// ExpectAccepted means every value in the CasePlan Domain must be accepted.
	ExpectAccepted ExpectedResult = iota
	// ExpectRejected means every value in the CasePlan Domain must be rejected.
	ExpectRejected
)

// ObligationOutcome explains why a planned constraint failure has no CasePlan.
type ObligationOutcome uint8

const (
	// ObligationPlanned means at least one isolated failure CasePlan was built.
	ObligationPlanned ObligationOutcome = iota
	// ObligationDominated means the failure space is empty after the other rules pass.
	ObligationDominated
	// ObligationUnconstructible means the rule is understood but has no constructive failure Domain.
	ObligationUnconstructible
)

// ConstraintPlan stores constructive passing and failing Domains for one source rule.
type ConstraintPlan struct {
	Source  ConstraintSource
	Pass    DomainID
	Fail    []DomainID
	Outcome ObligationOutcome
	Reason  string
}

// CasePlan names one semantic partition without materializing a JSON case.
type CasePlan struct {
	Name      string
	Expect    ExpectedResult
	Values    DomainID
	Source    ConstraintSource
	Generator *rapid.Generator[jsonvalue.Value]
}

// CasePlanner builds canonical semantic partitions from a compiled Domain graph.
type CasePlanner struct {
	Domains     *DomainRegistry
	Constraints []ConstraintPlan
}

// CompiledSuite is a planned suite with one constructive generator per CasePlan.
type CompiledSuite struct {
	Root        DomainID
	Domains     *DomainRegistry
	Constraints []ConstraintPlan
	Cases       []CasePlan
}

// DomainPair is an unordered pair used by the intersection cache.
type DomainPair struct {
	First  DomainID
	Second DomainID
}

// ConstraintSource locates one compiled schema keyword.
type ConstraintSource struct {
	Pointer string
	Keyword string
}

// GenerationExamples are trusted generation inputs and not Domain identity.
type GenerationExamples struct {
	Valid   []jsonvalue.Value
	Invalid []jsonvalue.Value
}

// schemaUse preserves one exact schema occurrence separately from its canonical Domain.
type schemaUse struct {
	pointer     string
	domain      DomainID
	localDomain DomainID
	constraints []ConstraintSource
	examples    GenerationExamples
	atomic      map[string]DomainID
	allOf       []*schemaUse
	items       *schemaUse
	properties  []schemaPropertyUse
	additional  *schemaUse
	resolved    *schemaUse
}

// schemaPropertyUse pairs one property name with its exact schema occurrence.
type schemaPropertyUse struct {
	name string
	use  *schemaUse
}

// Error reports a stable compilation outcome with source context.
type Error struct {
	Phase   string
	Code    string
	Pointer string
	Keyword string
	Cause   error
}

// Error formats compilation context.
func (compileError *Error) Error() string {
	location := compileError.Pointer
	if compileError.Keyword != "" {
		location += "/" + compileError.Keyword
	}

	return fmt.Sprintf("%s %s at %s: %v", compileError.Phase, compileError.Code, location, compileError.Cause)
}

// Unwrap returns the underlying failure.
func (compileError *Error) Unwrap() error {
	return compileError.Cause
}
