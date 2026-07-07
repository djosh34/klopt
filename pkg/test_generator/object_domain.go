package testgenerator

import "gopkg.in/yaml.v3"

var _ YamlParser = new(PropertyDomain)
var _ YamlParser = new(ObjectDomain)

type AdditionalPolicyKind int

const (
	AdditionalTrue AdditionalPolicyKind = iota
	AdditionalFalse
	AdditionalSchema
)

type PropertyDomain struct {
	Key string
	*Hash
	Required bool
}

func (p *PropertyDomain) Parse(node yaml.Node) error {
	//TODO implement me
	panic("implement me")
}

type ObjectDomain struct {
	Properties []*Hash

	AdditionalPropertyKind   AdditionalPolicyKind
	AdditionalPropertyDomain *Hash

	MinProps int
	MaxProps *int
}

func (o *ObjectDomain) Parse(node yaml.Node) error {
	//TODO implement me
	panic("implement me")
}
