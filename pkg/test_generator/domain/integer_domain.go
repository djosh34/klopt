package domain

import (
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
)

type IntegerDomain struct {
	Nullable bool     `json:"nullable"`
	Enum     []Number `json:"enum"`

	Minimum          *Number `json:"minimum"`
	Maximum          *Number `json:"maximum"`
	ExclusiveMinimum bool    `json:"exclusiveMinimum"`
	ExclusiveMaximum bool    `json:"exclusiveMaximum"`
	MultipleOf       *Number `json:"multipleOf"`
	Format           *string `json:"format"`
}

func (i *IntegerDomain) AllOfMerge(domain types.Domain) (types.Domain, error) {
	if allOfDomain, ok := domain.(*AllOfDomain); ok {
		return allOfDomain.AllOfMerge(i)
	}
	if _, ok := domain.(*IntegerDomain); !ok {
		return nil, errors.New("domain is not IntegerDomain")
	}

	return nil, errors.New("NOT IMPLEMENTED")
}

func (i *IntegerDomain) ToHasher() (types.Hasher, error) {
	if i == nil {
		return nil, errors.New("domain of integer cannot be nil")
	}

	return &hashables.IntegerHashable{
		Nullable:         i.Nullable,
		Enum:             toHashableNumbers(i.Enum),
		Minimum:          toHashableNumberPtr(i.Minimum),
		Maximum:          toHashableNumberPtr(i.Maximum),
		ExclusiveMinimum: i.ExclusiveMinimum,
		ExclusiveMaximum: i.ExclusiveMaximum,
		MultipleOf:       toHashableNumberPtr(i.MultipleOf),
		Format:           i.Format,
	}, nil
}

func (dc *DomainContext) ParseInteger(node *json.RawMessage) (IntegerDomain, error) {
	return IntegerDomain{}, errors.New("NOT IMPLEMENTED")
}
