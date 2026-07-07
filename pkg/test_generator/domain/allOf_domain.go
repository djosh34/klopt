package domain

import (
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
)

type AllOfDomain struct {
	Domains      []types.Domain
	MergedDomain types.Domain
}

func (a *AllOfDomain) ToHasher() (types.Hasher, error) {
	if a == nil {
		return nil, errors.New("domain of allOf cannot be nil")
	}

	domainHashers := make([]types.Hasher, 0, len(a.Domains))
	for _, allOfDomain := range a.Domains {
		var domainHasher types.Hasher
		if allOfDomain != nil {
			hasher, err := allOfDomain.ToHasher()
			if err != nil {
				return nil, err
			}
			domainHasher = hasher
		}
		domainHashers = append(domainHashers, domainHasher)
	}

	var mergedHasher types.Hasher
	if a.MergedDomain != nil {
		hasher, err := a.MergedDomain.ToHasher()
		if err != nil {
			return nil, err
		}
		mergedHasher = hasher
	}

	return &hashables.AllOfHashable{
		Domains:      domainHashers,
		MergedDomain: mergedHasher,
	}, nil
}

func (dc *DomainContext) ParseAllOf(node *json.RawMessage) (AllOfDomain, error) {

	return AllOfDomain{}, errors.New("NOT IMPLEMENTED")
}
