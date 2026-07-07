package domain

import (
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
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

func (domain *StringDomain) AllOfMerge(otherDomain types.Domain) (types.Domain, error) {
	if allOfDomain, ok := otherDomain.(*AllOfDomain); ok {
		return allOfDomain.AllOfMerge(domain)
	}
	if _, ok := otherDomain.(*StringDomain); !ok {
		return nil, errors.New("domain is not StringDomain")
	}

	return nil, errors.New("NOT IMPLEMENTED")
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

func (dc *DomainContext) ParseString(node *json.RawMessage) (StringDomain, error) {
	return StringDomain{}, errors.New("NOT IMPLEMENTED")
}
