package domain

import (
	"bytes"
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
	"fmt"
)

type BoolDomain struct {
	Nullable bool         `json:"nullable"`
	Enum     []types.Enum `json:"enum"`
}

func (b *BoolDomain) AllOfMerge(domain types.Domain) (types.Domain, error) {
	if allOfDomain, ok := domain.(*AllOfDomain); ok {
		return allOfDomain.AllOfMerge(b)
	}
	if _, ok := domain.(*BoolDomain); !ok {
		return nil, errors.New("domain is not BoolDomain")
	}

	return nil, errors.New("NOT IMPLEMENTED")
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
	if node == nil {
		return BoolDomain{}, errors.New("schema node is nil")
	}

	decoder := json.NewDecoder(bytes.NewReader(*node))
	decoder.UseNumber()
	jsonKV := JSONKV{}
	if err := decoder.Decode(&jsonKV); err != nil {
		return BoolDomain{}, err
	}

	var raw map[string]any
	decoder = json.NewDecoder(bytes.NewReader(*node))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return BoolDomain{}, err
	}

	schemaType, err := requiredString(raw, "type")
	if err != nil {
		return BoolDomain{}, err
	}
	if schemaType != "boolean" {
		return BoolDomain{}, fmt.Errorf("bool domain type must be boolean, got %q", schemaType)
	}

	domain := BoolDomain{}
	if value, ok := raw["nullable"]; ok {
		nullable, ok := value.(bool)
		if !ok {
			return BoolDomain{}, errors.New("nullable must be boolean")
		}
		domain.Nullable = nullable
	}

	enums, _, err := parseEnums(jsonKV)
	if err != nil {
		return BoolDomain{}, err
	}
	domain.Enum = enums

	deleteAllowableKeys(jsonKV)
	delete(jsonKV, "enum")
	if len(jsonKV) != 0 {
		for key := range jsonKV {
			return BoolDomain{}, fmt.Errorf("unsupported bool schema field %q", key)
		}
	}

	return domain, nil
}
