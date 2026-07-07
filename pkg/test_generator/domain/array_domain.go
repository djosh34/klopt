package domain

import (
	"bytes"
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
	"fmt"
)

type ArrayDomain struct {
	Nullable bool `json:"nullable"`

	Items types.Domain `json:"items"`

	MinItems int  `json:"minItems"`
	MaxItems *int `json:"maxItems"`
}

func (a *ArrayDomain) AllOfMerge(domain types.Domain) (types.Domain, error) {
	if allOfDomain, ok := domain.(*AllOfDomain); ok {
		return allOfDomain.AllOfMerge(a)
	}
	if _, ok := domain.(*ArrayDomain); !ok {
		return nil, errors.New("domain is not ArrayDomain")
	}

	return nil, errors.New("NOT IMPLEMENTED")
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
	if node == nil {
		return ArrayDomain{}, errors.New("schema node is nil")
	}

	decoder := json.NewDecoder(bytes.NewReader(*node))
	decoder.UseNumber()
	jsonKV := JSONKV{}
	if err := decoder.Decode(&jsonKV); err != nil {
		return ArrayDomain{}, err
	}

	var raw map[string]any
	decoder = json.NewDecoder(bytes.NewReader(*node))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return ArrayDomain{}, err
	}

	schemaType, err := requiredString(raw, "type")
	if err != nil {
		return ArrayDomain{}, err
	}
	if schemaType != "array" {
		return ArrayDomain{}, fmt.Errorf("array domain type must be array, got %q", schemaType)
	}

	domain := ArrayDomain{}
	if value, ok := raw["nullable"]; ok {
		nullable, ok := value.(bool)
		if !ok {
			return ArrayDomain{}, errors.New("nullable must be boolean")
		}
		domain.Nullable = nullable
	}

	itemsRaw, ok := jsonKV["items"]
	if !ok {
		return ArrayDomain{}, errors.New("items is required")
	}
	if string(itemsRaw) == "null" {
		return ArrayDomain{}, errors.New("items cannot be null")
	}
	if _, ok := raw["items"].([]any); ok {
		return ArrayDomain{}, errors.New("items cannot be an array")
	}
	itemsObject, ok := raw["items"].(map[string]any)
	if !ok {
		return ArrayDomain{}, errors.New("items must be object")
	}

	if _, ok := raw["uniqueItems"]; ok {
		return ArrayDomain{}, errors.New("uniqueItems is unsupported")
	}
	if value, ok := raw["minItems"]; ok {
		minItems, err := parseNonNegativeInteger(value, "minItems")
		if err != nil {
			return ArrayDomain{}, err
		}
		domain.MinItems = minItems
	}
	if value, ok := raw["maxItems"]; ok {
		maxItems, err := parseNonNegativeInteger(value, "maxItems")
		if err != nil {
			return ArrayDomain{}, err
		}
		domain.MaxItems = &maxItems
	}
	if domain.MaxItems != nil && domain.MinItems > *domain.MaxItems {
		return ArrayDomain{}, errors.New("minItems cannot exceed maxItems")
	}

	deleteAllowableKeys(jsonKV)
	for _, key := range []string{"items", "minItems", "maxItems"} {
		delete(jsonKV, key)
	}
	if len(jsonKV) != 0 {
		for key := range jsonKV {
			return ArrayDomain{}, fmt.Errorf("unsupported array schema field %q", key)
		}
	}

	if len(itemsObject) != 0 {
		itemsDomain, err := dc.Parse(&itemsRaw)
		if err != nil {
			return ArrayDomain{}, fmt.Errorf("items: %w", err)
		}
		domain.Items = itemsDomain
	}

	return domain, nil
}
