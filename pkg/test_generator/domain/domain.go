package domain

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
	"fmt"
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

type domainStore = map[types.Domain]struct{}
type DomainContext struct {
	// Each Domain that is created, must be added here
	domainStore domainStore
	// Exists only for testing, to 'mock'/'inject' wanted parse outputs
	parse func(node *json.RawMessage) (types.Domain, error)
}

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

	domain, domainErr := dc.parse(node)
	if domainErr != nil {
		return nil, domainErr
	}

	dc.AddDomain(domain)

	return domain, nil
}

func (dc *DomainContext) parseDefault(node *json.RawMessage) (types.Domain, error) {
	parseErrors := make([]error, 0, 7)

	allOfDomain, allOfErr := dc.ParseAllOf(node)
	if allOfErr == nil {
		return &allOfDomain, nil
	}
	parseErrors = append(parseErrors, allOfErr)

	objectDomain, objectErr := dc.ParseObject(node)
	if objectErr == nil {
		return &objectDomain, nil
	}
	parseErrors = append(parseErrors, objectErr)

	arrayDomain, arrayErr := dc.ParseArray(node)
	if arrayErr == nil {
		return &arrayDomain, nil
	}
	parseErrors = append(parseErrors, arrayErr)

	stringDomain, stringErr := dc.ParseString(node)
	if stringErr == nil {
		return &stringDomain, nil
	}
	parseErrors = append(parseErrors, stringErr)

	numberDomain, numberErr := dc.ParseNumber(node)
	if numberErr == nil {
		return &numberDomain, nil
	}
	parseErrors = append(parseErrors, numberErr)

	integerDomain, integerErr := dc.ParseInteger(node)
	if integerErr == nil {
		return &integerDomain, nil
	}
	parseErrors = append(parseErrors, integerErr)

	boolDomain, boolErr := dc.ParseBool(node)
	if boolErr == nil {
		return &boolDomain, nil
	}
	parseErrors = append(parseErrors, boolErr)

	return nil, fmt.Errorf("unsupported node type: %w", errors.Join(parseErrors...))
}
