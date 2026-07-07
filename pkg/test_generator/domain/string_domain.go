package domain

import (
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"errors"
)

type StringDomain struct {
	Nullable bool `json:"nullable"`

	Enum []string `json:"enum"`

	Pattern *string `json:"pattern"`
	Format  *string `json:"format"`

	XValidExamples   []string `json:"x-valid-examples"`
	XInvalidExamples []string `json:"x-invalid-examples"`

	MinLength int  `json:"minLength"`
	MaxLength *int `json:"maxLength"`
}

func (domain *StringDomain) ToHasher() (types.Hasher, error) {
	if domain == nil {
		return nil, errors.New("domain of string cannot be nil")
	}

	return &hashables.StringHashable{
		Nullable:         domain.Nullable,
		Enum:             domain.Enum,
		Pattern:          domain.Pattern,
		Format:           domain.Format,
		XValidExamples:   domain.XValidExamples,
		XInvalidExamples: domain.XInvalidExamples,
		MinLength:        domain.MinLength,
		MaxLength:        domain.MaxLength,
	}, nil
}
