package domain

import (
	"decode_and_validate_generator/pkg/test_generator/types"
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

	panic("TO DO")
}
