package domain

import (
	"encoding/json"
	"errors"
	"fmt"

	"decode_and_validate_generator/pkg/test_generator/types"
)

type JSONKV map[string]json.RawMessage

var alwaysAllowableKeys = []string{
	"type",
	"nullable",
	"title",
	"description",
}

func deleteAllowableKeys(jsonKV JSONKV) {
	for _, key := range alwaysAllowableKeys {
		delete(jsonKV, key)
	}
}

func requiredSchemaType(jsonKV JSONKV, schemaType *string) (string, error) {
	if _, ok := jsonKV["type"]; !ok {
		return "", errors.New("type is required")
	}

	if schemaType == nil {
		return "", errors.New("type must be string")
	}

	return *schemaType, nil
}

type (
	domainStore   = map[types.Domain]struct{}
	DomainContext struct {
		// Each Domain that is created, must be added here
		domainStore domainStore
		// Exists only for testing, to 'mock'/'inject' wanted parse outputs
		parse func(node *json.RawMessage) (types.Domain, error)
	}
)

func (dc *DomainContext) AddDomain(domain types.Domain) {
	if dc.domainStore == nil {
		dc.domainStore = make(map[types.Domain]struct{})
	}

	dc.domainStore[domain] = struct{}{}
}

func (dc *DomainContext) Parse(node *json.RawMessage) (types.Domain, error) {
	if dc.parse == nil {
		dc.parse = dc.parseDefault
	}

	parse := dc.parse

	if node != nil {
		jsonKV := JSONKV{}
		if err := json.Unmarshal(*node, &jsonKV); err == nil && jsonKV != nil {
			if _, ok := jsonKV["allOf"]; ok {
				parse = dc.parseDefault
			}
		}
	}

	domain, domainErr := parse(node)
	if domainErr != nil {
		return nil, domainErr
	}

	dc.AddDomain(domain)

	return domain, nil
}

func (dc *DomainContext) parseDefault(node *json.RawMessage) (types.Domain, error) {
	if node == nil {
		return nil, errors.New("schema node is nil")
	}

	allOfDomain, allOfErr := dc.ParseAllOf(node)
	if allOfErr == nil {
		return &allOfDomain, nil
	}

	type someSchema struct {
		Type string `json:"type"`
	}

	schemaItem := someSchema{}

	err := json.Unmarshal(*node, &schemaItem)
	if err != nil {
		return nil, errors.New("Schema does not specify type")
	}

	schemaType := schemaItem.Type
	if schemaType == "" {
		jsonKV := JSONKV{}
		if err := json.Unmarshal(*node, &jsonKV); err != nil {
			return nil, err
		}

		for _, key := range []string{"required", "properties", "additionalProperties", "minProperties", "maxProperties"} {
			if _, ok := jsonKV[key]; ok {
				objectDomain, err := dc.ParseObject(node)
				if err != nil {
					return nil, err
				}

				return &objectDomain, nil
			}
		}
	}

	switch schemaType {
	case "object":
		objectDomain, err := dc.ParseObject(node)
		if err != nil {
			return nil, err
		}

		return &objectDomain, nil
	case "array":
		arrayDomain, err := dc.ParseArray(node)
		if err != nil {
			return nil, err
		}

		return &arrayDomain, nil
	case "string":
		stringDomain, err := dc.ParseString(node)
		if err != nil {
			return nil, err
		}

		return &stringDomain, nil
	case "number", "integer":
		numberDomain, err := dc.ParseNumber(node)
		if err != nil {
			return nil, err
		}

		return &numberDomain, nil
	case "boolean":
		boolDomain, err := dc.ParseBool(node)
		if err != nil {
			return nil, err
		}

		return &boolDomain, nil
	default:
		return nil, fmt.Errorf("unsupported schema object type %q", schemaType)
	}
}
