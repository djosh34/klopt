package testgenerator

type AdditionalPolicyKind int

const (
	AdditionalTrue AdditionalPolicyKind = iota
	AdditionalFalse
	AdditionalSchema
)

type ObjectDomain struct {
	Properties map[string]Domain
	Required   map[string]bool

	AdditionalPropertyKind   AdditionalPolicyKind
	AdditionalPropertyDomain Domain

	MinProps int
	MaxProps *int
}
