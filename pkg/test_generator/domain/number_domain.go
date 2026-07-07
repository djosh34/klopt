package domain

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"errors"
)

type NumberDomain struct {
	Nullable bool      `json:"nullable"`
	Enum     []float64 `json:"enum"`

	Minimum          *float64 `json:"minimum"`
	Maximum          *float64 `json:"maximum"`
	ExclusiveMinimum bool     `json:"exclusiveMinimum"`
	ExclusiveMaximum bool     `json:"exclusiveMaximum"`
	MultipleOf       *float64 `json:"multipleOf"`
	Format           *string  `json:"format"`
}

func (n *NumberDomain) ToHasher() (types.Hasher, error) {
	if n == nil {
		return nil, errors.New("domain of number cannot be nil")
	}

	panic("TO DO")
}
