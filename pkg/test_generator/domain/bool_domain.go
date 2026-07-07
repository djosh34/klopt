package domain

import (
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
)

type BoolDomain struct {
	Nullable bool   `json:"nullable"`
	Enum     []bool `json:"enum"`
}

func (b *BoolDomain) ToHasher() (types.Hasher, error) {
	if b == nil {
		return nil, errors.New("domain of bool cannot be nil")
	}

	return &hashables.BoolHashable{
		Nullable: b.Nullable,
		Enum:     b.Enum,
	}, nil
}

func (dc *DomainContext) ParseBool(node *json.RawMessage) (BoolDomain, error) {
	return BoolDomain{}, errors.New("NOT IMPLEMENTED")
}
