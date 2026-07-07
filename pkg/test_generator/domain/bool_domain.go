package domain

import (
	"encoding/json"
	"errors"
	"fmt"

	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
)

type BoolDomain struct {
	Nullable bool         `json:"nullable"`
	Enum     []types.Enum `json:"enum"`
}

func (b *BoolDomain) AllOfMerge(domain types.Domain) (types.Domain, error) {
	if b == nil {
		return nil, errors.New("bool domain cannot be nil")
	}

	if allOfDomain, ok := domain.(*AllOfDomain); ok {
		mergedAllOf := &AllOfDomain{}
		if _, err := mergedAllOf.AllOfMerge(b); err != nil {
			return nil, err
		}

		return mergedAllOf.AllOfMerge(allOfDomain)
	}

	otherBool, ok := domain.(*BoolDomain)
	if !ok || otherBool == nil {
		return nil, errors.New("domain is not BoolDomain")
	}

	merged := *b
	merged.Nullable = b.Nullable && otherBool.Nullable

	enums, err := mergeEnums(b.Enum, otherBool.Enum)
	if err != nil {
		return nil, err
	}

	merged.Enum = enums

	return &merged, nil
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

type boolSchema struct {
	Type     *string `json:"type"`
	Nullable *bool   `json:"nullable"`
}

func (dc *DomainContext) ParseBool(node *json.RawMessage) (BoolDomain, error) {
	if node == nil {
		return BoolDomain{}, errors.New("schema node is nil")
	}

	jsonKV := JSONKV{}
	if err := json.Unmarshal(*node, &jsonKV); err != nil {
		return BoolDomain{}, err
	}

	schema := boolSchema{}
	if err := json.Unmarshal(*node, &schema); err != nil {
		return BoolDomain{}, err
	}

	schemaType, err := requiredSchemaType(jsonKV, schema.Type)
	if err != nil {
		return BoolDomain{}, err
	}

	if schemaType != "boolean" {
		return BoolDomain{}, fmt.Errorf("bool domain type must be boolean, got %q", schemaType)
	}

	domain := BoolDomain{}

	if _, ok := jsonKV["nullable"]; ok {
		if schema.Nullable == nil {
			return BoolDomain{}, errors.New("nullable must be boolean")
		}

		domain.Nullable = *schema.Nullable
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
