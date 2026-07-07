package domain

import (
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
)

type ArrayDomain struct {
	Nullable bool `json:"nullable"`

	Items types.Domain `json:"items"`

	MinItems int  `json:"minItems"`
	MaxItems *int `json:"maxItems"`
}

func (a *ArrayDomain) ToHasher() (types.Hasher, error) {
	if a == nil {
		return nil, errors.New("domain of array cannot be nil")
	}

	var itemsHasher types.Hasher
	if a.Items != nil {
		hasher, err := a.Items.ToHasher()
		if err != nil {
			return nil, err
		}
		itemsHasher = hasher
	}

	return &hashables.ArrayHashable{
		Nullable: a.Nullable,
		Items:    itemsHasher,
		MinItems: a.MinItems,
		MaxItems: a.MaxItems,
	}, nil
}

func (dc *DomainContext) ParseArray(node *json.RawMessage) (ArrayDomain, error) {
	return ArrayDomain{}, errors.New("NOT IMPLEMENTED")
}
