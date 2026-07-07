package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
)

type ArrayDomain struct {
	Nullable bool `json:"nullable"`

	Enum []types.Enum `json:"enum"`

	Items types.Domain `json:"items"`

	MinItems int  `json:"minItems"`
	MaxItems *int `json:"maxItems"`
}

func (a *ArrayDomain) AllOfMerge(domain types.Domain) (types.Domain, error) {
	if a == nil {
		return nil, errors.New("array domain cannot be nil")
	}

	if allOfDomain, ok := domain.(*AllOfDomain); ok {
		mergedAllOf := &AllOfDomain{}
		if _, err := mergedAllOf.AllOfMerge(a); err != nil {
			return nil, err
		}

		return mergedAllOf.AllOfMerge(allOfDomain)
	}

	otherArray, ok := domain.(*ArrayDomain)
	if !ok || otherArray == nil {
		return nil, errors.New("domain is not ArrayDomain")
	}

	merged := *a
	merged.Nullable = a.Nullable && otherArray.Nullable

	enums, err := mergeEnums(a.Enum, otherArray.Enum)
	if err != nil {
		return nil, err
	}

	merged.Enum = enums

	if a.Items == nil {
		merged.Items = otherArray.Items
	} else if otherArray.Items == nil {
		merged.Items = a.Items
	} else {
		items, err := a.Items.AllOfMerge(otherArray.Items)
		if err != nil {
			return nil, err
		}

		merged.Items = items
	}

	if otherArray.MinItems > merged.MinItems {
		merged.MinItems = otherArray.MinItems
	}

	if merged.MaxItems == nil || (otherArray.MaxItems != nil && *otherArray.MaxItems < *merged.MaxItems) {
		merged.MaxItems = otherArray.MaxItems
	}

	return &merged, nil
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
		Enum:     a.Enum,
		Items:    itemsHasher,
		MinItems: a.MinItems,
		MaxItems: a.MaxItems,
	}, nil
}

type arraySchema struct {
	Type     *string          `json:"type"`
	Nullable *bool            `json:"nullable"`
	Items    *json.RawMessage `json:"items"`
	MinItems *int             `json:"minItems"`
	MaxItems *int             `json:"maxItems"`
}

func (dc *DomainContext) ParseArray(node *json.RawMessage) (ArrayDomain, error) {
	if node == nil {
		return ArrayDomain{}, errors.New("schema node is nil")
	}

	jsonKV := JSONKV{}
	if err := json.Unmarshal(*node, &jsonKV); err != nil {
		return ArrayDomain{}, err
	}

	schema := arraySchema{}
	if err := json.Unmarshal(*node, &schema); err != nil {
		return ArrayDomain{}, err
	}

	schemaType, err := requiredSchemaType(jsonKV, schema.Type)
	if err != nil {
		return ArrayDomain{}, err
	}

	if schemaType != "array" {
		return ArrayDomain{}, fmt.Errorf("array domain type must be array, got %q", schemaType)
	}

	domain := ArrayDomain{}

	if _, ok := jsonKV["nullable"]; ok {
		if schema.Nullable == nil {
			return ArrayDomain{}, errors.New("nullable must be boolean")
		}

		domain.Nullable = *schema.Nullable
	}

	enums, _, err := parseEnums(jsonKV)
	if err != nil {
		return ArrayDomain{}, err
	}

	domain.Enum = enums

	if _, ok := jsonKV["items"]; !ok {
		return ArrayDomain{}, errors.New("items is required")
	}

	if schema.Items == nil {
		return ArrayDomain{}, errors.New("items cannot be null")
	}

	itemsRaw := *schema.Items

	trimmedItemsRaw := strings.TrimSpace(string(itemsRaw))
	if trimmedItemsRaw != "" && trimmedItemsRaw[0] == '[' {
		return ArrayDomain{}, errors.New("items cannot be an array")
	}

	itemsObject := JSONKV{}
	if err := json.Unmarshal(itemsRaw, &itemsObject); err != nil {
		return ArrayDomain{}, errors.New("items must be object")
	}

	if itemsObject == nil {
		return ArrayDomain{}, errors.New("items must be object")
	}

	if _, ok := jsonKV["uniqueItems"]; ok {
		return ArrayDomain{}, errors.New("uniqueItems is unsupported")
	}

	if _, ok := jsonKV["minItems"]; ok {
		if schema.MinItems == nil {
			return ArrayDomain{}, errors.New("minItems cannot be null")
		}

		if *schema.MinItems < 0 {
			return ArrayDomain{}, errors.New("minItems cannot be negative")
		}

		domain.MinItems = *schema.MinItems
	}

	if _, ok := jsonKV["maxItems"]; ok {
		if schema.MaxItems == nil {
			return ArrayDomain{}, errors.New("maxItems cannot be null")
		}

		if *schema.MaxItems < 0 {
			return ArrayDomain{}, errors.New("maxItems cannot be negative")
		}

		domain.MaxItems = schema.MaxItems
	}

	if domain.MaxItems != nil && domain.MinItems > *domain.MaxItems {
		return ArrayDomain{}, errors.New("minItems cannot exceed maxItems")
	}

	deleteAllowableKeys(jsonKV)

	for _, key := range []string{"enum", "items", "minItems", "maxItems"} {
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
