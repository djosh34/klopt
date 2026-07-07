package domain

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
)

type domainStore = map[types.Domain]struct{}
type DomainContext struct {
	// Each Domain that is created, must be added here
	domainStore domainStore
	// Exists only for testing, to 'mock'/'inject' wanted parse outputs
	parse func(node *json.RawMessage) (types.Domain, error)
}

func (dc *DomainContext) AddDomain(domain types.Domain) {
	if dc.domainStore == nil {
		dc.domainStore = make(map[types.Domain]struct{})
	}
	dc.domainStore[domain] = struct{}{}
}

func (dc *DomainContext) Parse(node *json.RawMessage) (types.Domain, error) {
	if dc.parse == nil {
		dc.parse = dc.parseDefault
	}

	domain, domainErr := dc.parse(node)
	if domainErr != nil {
		return nil, domainErr
	}

	dc.AddDomain(domain)

	return domain, nil
}

func (dc *DomainContext) parseDefault(node *json.RawMessage) (types.Domain, error) {
	_ = node

	return nil, nil
}
