package domain

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"errors"
)

type IntegerDomain struct {
	Nullable bool    `json:"nullable"`
	Enum     []int64 `json:"enum"`

	Minimum          *int64  `json:"minimum"`
	Maximum          *int64  `json:"maximum"`
	ExclusiveMinimum bool    `json:"exclusiveMinimum"`
	ExclusiveMaximum bool    `json:"exclusiveMaximum"`
	MultipleOf       *int64  `json:"multipleOf"`
	Format           *string `json:"format"`
}

func (i *IntegerDomain) ToHasher() (types.Hasher, error) {
	if i == nil {
		return nil, errors.New("domain of integer cannot be nil")
	}

	panic("TO DO")
}
