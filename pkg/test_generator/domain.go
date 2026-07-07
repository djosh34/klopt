package testgenerator

import "gopkg.in/yaml.v3"

type Hash [32]byte

type Hasher interface {
	Hash() (Hash, error)
}

//type AllOfMerger interface {
//	MergeAllOf(domain Domain) Domain
//}

type YamlParser interface {
	Parse(node yaml.Node) error
}

type Domain interface {
	Hasher
	//AllOfMerger
	YamlParser
}

type DomainContext struct {
	domainStore map[Hash]Domain
}

func (dc *DomainContext) Parse(node yaml.Node) (*Hash, error) {

	return nil, nil
}
