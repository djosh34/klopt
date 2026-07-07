package domain

import (
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
)

// nil == null
type EnumDomain struct {
	*json.RawMessage
}

func (e *EnumDomain) AllOfMerge(domain types.Domain) (types.Domain, error) {
	if allOfDomain, ok := domain.(*AllOfDomain); ok {
		return allOfDomain.AllOfMerge(e)
	}
	if _, ok := domain.(*EnumDomain); !ok {
		return nil, errors.New("domain is not EnumDomain")
	}

	return nil, errors.New("NOT IMPLEMENTED")
}

func (e *EnumDomain) ToHasher() (types.Hasher, error) {
	if e == nil {
		return nil, errors.New("domain of enum cannot be nil")
	}

	return &hashables.EnumHashable{RawMessage: e.RawMessage}, nil
}

func NewEnumFromJSON(node *json.RawMessage) EnumDomain {
	return EnumDomain{node}
}
