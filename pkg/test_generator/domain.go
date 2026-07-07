package testgenerator

import "encoding/json"

type Hash [32]byte

type Hasher interface {
	GenerateHash() (Hash, error)
}

//type AllOfMerger interface {
//	MergeAllOf(domain Domain) Domain
//}

type Domain interface {
	Hasher
	//AllOfMerger
}

type DomainContext struct {
	// Each Domain that is created, must be added here
	domainStore map[Hash]Domain
	// Exists only for testing, to 'mock'/'inject' wanted parse outputs
	parse func(node *json.RawMessage) (*Hash, error)
}

func (dc *DomainContext) Parse(node *json.RawMessage) (*Hash, error) {
	if dc.parse != nil {
		return dc.parse(node)
	}

	return dc.ParseDefault(node)
}

func (dc *DomainContext) ParseDefault(node *json.RawMessage) (*Hash, error) {
	_ = node

	return nil, nil
}
