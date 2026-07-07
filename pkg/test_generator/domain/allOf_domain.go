package domain

import (
	"encoding/json"
	"errors"
	"fmt"

	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
)

var _ types.AllOfMerger = new(AllOfDomain)

type AllOfDomain struct {
	Domains      []types.Domain
	MergedDomain types.Domain
}

func (a *AllOfDomain) AllOfMerge(domain types.Domain) (types.Domain, error) {
	if a == nil {
		return nil, errors.New("allOf domain cannot be nil")
	}

	if domain == nil {
		return nil, errors.New("domain cannot be nil")
	}

	mergedAllOf := &AllOfDomain{
		Domains:      append([]types.Domain(nil), a.Domains...),
		MergedDomain: a.MergedDomain,
	}

	if otherAllOf, ok := domain.(*AllOfDomain); ok {
		if otherAllOf == nil {
			return nil, errors.New("allOf domain cannot be nil")
		}

		if len(otherAllOf.Domains) == 0 && otherAllOf.MergedDomain != nil {
			if err := mergedAllOf.mergeOne(otherAllOf.MergedDomain); err != nil {
				return nil, err
			}
		} else {
			for _, childDomain := range otherAllOf.Domains {
				if err := mergedAllOf.mergeOne(childDomain); err != nil {
					return nil, err
				}
			}
		}
	} else if err := mergedAllOf.mergeOne(domain); err != nil {
		return nil, err
	}

	*a = *mergedAllOf

	return a, nil
}

func (a *AllOfDomain) mergeOne(domain types.Domain) error {
	if domain == nil {
		return errors.New("domain cannot be nil")
	}

	a.Domains = append(a.Domains, domain)
	if a.MergedDomain == nil {
		a.MergedDomain = domain

		return nil
	}

	mergedDomain, err := a.MergedDomain.AllOfMerge(domain)
	if err != nil {
		return err
	}

	a.MergedDomain = mergedDomain

	return nil
}

func (a *AllOfDomain) ToHasher() (types.Hasher, error) {
	if a == nil {
		return nil, errors.New("domain of allOf cannot be nil")
	}

	domainHashers := make([]types.Hasher, 0, len(a.Domains))
	for _, allOfDomain := range a.Domains {
		var domainHasher types.Hasher

		if allOfDomain != nil {
			hasher, err := allOfDomain.ToHasher()
			if err != nil {
				return nil, err
			}

			domainHasher = hasher
		}

		domainHashers = append(domainHashers, domainHasher)
	}

	var mergedHasher types.Hasher

	if a.MergedDomain != nil {
		hasher, err := a.MergedDomain.ToHasher()
		if err != nil {
			return nil, err
		}

		mergedHasher = hasher
	}

	return &hashables.AllOfHashable{
		Domains:      domainHashers,
		MergedDomain: mergedHasher,
	}, nil
}

func (dc *DomainContext) ParseAllOf(node *json.RawMessage) (allOfDomain AllOfDomain, err error) {
	originalStore := dc.domainStore
	if originalStore != nil {
		originalStore = make(domainStore, len(dc.domainStore))
		for domain := range dc.domainStore {
			originalStore[domain] = struct{}{}
		}
	}
	defer func() {
		if err != nil {
			dc.domainStore = originalStore
		}
	}()

	if node == nil {
		return AllOfDomain{}, errors.New("schema node is nil")
	}

	jsonKV := JSONKV{}
	if err := json.Unmarshal(*node, &jsonKV); err != nil {
		return AllOfDomain{}, err
	}

	if jsonKV == nil {
		return AllOfDomain{}, errors.New("schema node must be object")
	}

	allOfRaw, ok := jsonKV["allOf"]
	if !ok {
		return AllOfDomain{}, errors.New("allOf is required")
	}

	for _, key := range []string{"oneOf", "anyOf", "not", "discriminator"} {
		if _, ok := jsonKV[key]; ok {
			return AllOfDomain{}, fmt.Errorf("%s is unsupported with allOf", key)
		}
	}

	for key := range jsonKV {
		if !isAllowedAllOfSiblingKey(key) {
			return AllOfDomain{}, fmt.Errorf("unsupported allOf schema field %q", key)
		}
	}

	if string(allOfRaw) == "null" {
		return AllOfDomain{}, errors.New("allOf cannot be null")
	}

	var allOfItems []json.RawMessage
	if err := json.Unmarshal(allOfRaw, &allOfItems); err != nil {
		return AllOfDomain{}, errors.New("allOf must be array")
	}

	if len(allOfItems) == 0 {
		return AllOfDomain{}, errors.New("allOf cannot be empty")
	}

	for _, allOfItem := range allOfItems {
		if string(allOfItem) == "null" {
			return AllOfDomain{}, errors.New("allOf item cannot be null")
		}

		itemKV := JSONKV{}
		if err := json.Unmarshal(allOfItem, &itemKV); err != nil {
			return AllOfDomain{}, errors.New("allOf item must be object")
		}

		if itemKV == nil {
			return AllOfDomain{}, errors.New("allOf item must be object")
		}

		if len(itemKV) == 0 {
			return AllOfDomain{}, errors.New("allOf item cannot be empty schema")
		}

		for _, key := range []string{"oneOf", "anyOf", "not", "discriminator"} {
			if _, ok := itemKV[key]; ok {
				return AllOfDomain{}, fmt.Errorf("allOf item %s is unsupported", key)
			}
		}

		if _, ok := itemKV["$ref"]; ok && len(itemKV) != 1 {
			return AllOfDomain{}, errors.New("$ref with siblings is unsupported")
		}

		domain, err := dc.Parse(&allOfItem)
		if err != nil {
			return AllOfDomain{}, err
		}

		if domain == nil {
			return AllOfDomain{}, errors.New("parsed allOf item cannot be nil")
		}

		if _, err := allOfDomain.AllOfMerge(domain); err != nil {
			return AllOfDomain{}, err
		}
	}

	siblingKV := make(JSONKV, len(jsonKV))
	for key, value := range jsonKV {
		if key == "allOf" || key == "title" || key == "description" {
			continue
		}

		siblingKV[key] = value
	}

	if len(siblingKV) == 1 {
		if nullableRaw, ok := siblingKV["nullable"]; ok {
			var nullable bool
			if err := json.Unmarshal(nullableRaw, &nullable); err != nil {
				return AllOfDomain{}, errors.New("nullable must be boolean")
			}

			nullableDomain, err := nullableOnlyDomain(allOfDomain.MergedDomain, nullable)
			if err != nil {
				return AllOfDomain{}, err
			}

			dc.AddDomain(nullableDomain)

			if _, err := allOfDomain.AllOfMerge(nullableDomain); err != nil {
				return AllOfDomain{}, err
			}

			return allOfDomain, nil
		}
	}

	if len(siblingKV) != 0 {
		siblingRaw, err := json.Marshal(siblingKV)
		if err != nil {
			return AllOfDomain{}, err
		}

		raw := json.RawMessage(siblingRaw)

		domain, err := dc.Parse(&raw)
		if err != nil {
			return AllOfDomain{}, err
		}

		if domain == nil {
			return AllOfDomain{}, errors.New("parsed allOf sibling cannot be nil")
		}

		if _, err := allOfDomain.AllOfMerge(domain); err != nil {
			return AllOfDomain{}, err
		}
	}

	return allOfDomain, nil
}

func isAllowedAllOfSiblingKey(key string) bool {
	switch key {
	case "allOf", "type", "nullable", "title", "description",
		"enum", "minLength", "maxLength", "pattern", "format", "x-valid-examples", "x-invalid-examples",
		"minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf",
		"items", "minItems", "maxItems",
		"required", "properties", "additionalProperties", "minProperties", "maxProperties":
		return true
	default:
		return false
	}
}

func nullableOnlyDomain(domain types.Domain, nullable bool) (types.Domain, error) {
	switch typedDomain := domain.(type) {
	case *StringDomain:
		return &StringDomain{Nullable: nullable}, nil
	case *NumberDomain:
		return &NumberDomain{Type: typedDomain.Type, Nullable: nullable}, nil
	case *BoolDomain:
		return &BoolDomain{Nullable: nullable}, nil
	case *ArrayDomain:
		return &ArrayDomain{Nullable: nullable}, nil
	case *ObjectDomain:
		return &ObjectDomain{Nullable: nullable}, nil
	default:
		return nil, errors.New("cannot apply nullable to merged allOf domain")
	}
}
