package domain

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
	parse func(node *json.RawMessage) (Domain, error)
}

func (dc *DomainContext) AddDomain(domain Domain) error {
	hash, hashErr := domain.GenerateHash()
	if hashErr != nil {
		return hashErr
	}

	if dc.domainStore == nil {
		dc.domainStore = make(map[Hash]Domain)
	}
	dc.domainStore[hash] = domain
	return nil
}

func (dc *DomainContext) ParseToHash(node *json.RawMessage) (Hash, error) {
	_, hash, domainErr := dc.Parse(node)
	return hash, domainErr
}

func (dc *DomainContext) Parse(node *json.RawMessage) (Domain, Hash, error) {
	if dc.parse == nil {
		dc.parse = dc.parseDefault
	}

	domain, domainErr := dc.parse(node)
	if domainErr != nil {
		return nil, Hash{}, domainErr
	}

	hash, hashErr := domain.GenerateHash()
	if hashErr != nil {
		return nil, Hash{}, hashErr
	}

	if dc.domainStore == nil {
		dc.domainStore = make(map[Hash]Domain)
	}
	dc.domainStore[hash] = domain

	return domain, hash, nil
}

func (dc *DomainContext) parseDefault(node *json.RawMessage) (Domain, error) {
	_ = node

	return nil, nil
}
