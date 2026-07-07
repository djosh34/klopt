package domain

import (
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
)

type NumberDomain struct {
	Nullable bool     `json:"nullable"`
	Enum     []Number `json:"enum"`

	Minimum          *Number `json:"minimum"`
	Maximum          *Number `json:"maximum"`
	ExclusiveMinimum bool    `json:"exclusiveMinimum"`
	ExclusiveMaximum bool    `json:"exclusiveMaximum"`
	MultipleOf       *Number `json:"multipleOf"`
	Format           *string `json:"format"`
}

func (n *NumberDomain) ToHasher() (types.Hasher, error) {
	if n == nil {
		return nil, errors.New("domain of number cannot be nil")
	}

	return &hashables.NumberHashable{
		Nullable:         n.Nullable,
		Enum:             toHashableNumbers(n.Enum),
		Minimum:          toHashableNumberPtr(n.Minimum),
		Maximum:          toHashableNumberPtr(n.Maximum),
		ExclusiveMinimum: n.ExclusiveMinimum,
		ExclusiveMaximum: n.ExclusiveMaximum,
		MultipleOf:       toHashableNumberPtr(n.MultipleOf),
		Format:           n.Format,
	}, nil
}

func (dc *DomainContext) ParseNumber(node *json.RawMessage) (NumberDomain, error) {
	return NumberDomain{}, errors.New("NOT IMPLEMENTED")
}
