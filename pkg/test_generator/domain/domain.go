// Package domain parses supported OpenAPI schema objects into mergeable domains.
package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	//nolint:depguard // Domain contracts are shared within test_generator.
	"decode_and_validate_generator/pkg/test_generator/types"
)

// JSONKV is a decoded JSON object whose values retain their original JSON representation.
type JSONKV map[string]json.RawMessage

// alwaysAllowableKeys are handled identically by every supported typed schema.
var alwaysAllowableKeys = []string{
	"type",
	"nullable",
	"title",
	"description",
}

// deleteAllowableKeys validates and removes fields shared by every supported schema type.
func deleteAllowableKeys(jsonKV JSONKV) error {
	if err := validateSchemaDocumentation(jsonKV); err != nil {
		return err
	}

	for _, key := range alwaysAllowableKeys {
		delete(jsonKV, key)
	}

	for key := range jsonKV {
		if isSpecificationExtension(key) {
			delete(jsonKV, key)
		}
	}

	return nil
}

// isSpecificationExtension reports whether key is an OpenAPI extension field.
func isSpecificationExtension(key string) bool {
	return strings.HasPrefix(key, "x-")
}

// isGeneratorSchemaExtension reports whether the generator interprets an extension field.
func isGeneratorSchemaExtension(key string) bool {
	return key == "x-valid-examples" || key == "x-invalid-examples"
}

// sortedJSONKeys returns object keys in deterministic lexical order.
func sortedJSONKeys(jsonKV JSONKV) []string {
	keys := make([]string, 0, len(jsonKV))
	for key := range jsonKV {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

// validateSchemaDocumentation checks the string-valued schema documentation fields.
func validateSchemaDocumentation(jsonKV JSONKV) error {
	for _, key := range []string{"title", "description"} {
		raw, ok := jsonKV[key]
		if !ok {
			continue
		}

		var value *string
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("%s must be string: %w", key, err)
		}

		if value == nil {
			return fmt.Errorf("%s must be string", key)
		}
	}

	return nil
}

// requiredSchemaType returns a schema's required string type.
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
	// domainStore tracks every domain created while parsing a schema graph.
	domainStore = map[types.Domain]struct{}

	// Context owns the domains created while parsing one schema graph.
	Context struct {
		// Each Domain that is created, must be added here
		domainStore domainStore
		// Exists only for testing, to 'mock'/'inject' wanted parse outputs
		parse func(node *json.RawMessage) (types.Domain, error)
	}
)

// AddDomain records a domain created by this context.
func (dc *Context) AddDomain(domain types.Domain) {
	if dc.domainStore == nil {
		dc.domainStore = make(map[types.Domain]struct{})
	}

	dc.domainStore[domain] = struct{}{}
}

// Parse parses and records one schema domain.
func (dc *Context) Parse(node *json.RawMessage) (types.Domain, error) {
	if dc.parse == nil {
		dc.parse = dc.parseDefault
	}

	parse := dc.parse

	if node != nil {
		jsonKV, err := decodeSchemaObject(node)
		if err != nil {
			return dc.parseAndStore(parse, node)
		}

		if _, ok := jsonKV["allOf"]; ok {
			parse = dc.parseDefault
		}
	}

	return dc.parseAndStore(parse, node)
}

// parseAndStore records a successfully parsed domain.
func (dc *Context) parseAndStore(
	parse func(*json.RawMessage) (types.Domain, error),
	node *json.RawMessage,
) (types.Domain, error) {
	domain, err := parse(node)
	if err != nil {
		return nil, err
	}

	dc.AddDomain(domain)

	return domain, nil
}

// parseDefault dispatches a schema object to its supported domain parser.
func (dc *Context) parseDefault(node *json.RawMessage) (types.Domain, error) {
	jsonKV, err := decodeSchemaObject(node)
	if err != nil {
		return nil, err
	}

	if _, ok := jsonKV["allOf"]; ok {
		allOfDomain, parseErr := dc.ParseAllOf(node)
		if parseErr != nil {
			return nil, parseErr
		}

		return &allOfDomain, nil
	}

	typeRaw, ok := jsonKV["type"]
	if !ok {
		return nil, errors.New("schema does not specify type")
	}

	var schemaType *string
	if err := json.Unmarshal(typeRaw, &schemaType); err != nil {
		return nil, fmt.Errorf("schema type must be string: %w", err)
	}

	if schemaType == nil {
		return nil, errors.New("schema type must be string")
	}

	return dc.parseTypedDomain(node, *schemaType)
}

// decodeSchemaObject decodes a non-nil JSON schema object.
func decodeSchemaObject(node *json.RawMessage) (JSONKV, error) {
	if node == nil {
		return nil, errors.New("schema node is nil")
	}

	var jsonKV JSONKV
	if err := json.Unmarshal(*node, &jsonKV); err != nil {
		return nil, fmt.Errorf("decode schema JSON: %w", err)
	}

	if jsonKV == nil {
		return nil, errors.New("schema node must be object")
	}

	return jsonKV, nil
}

// parseTypedDomain dispatches a schema with a decoded type.
func (dc *Context) parseTypedDomain(node *json.RawMessage, schemaType string) (types.Domain, error) {
	switch schemaType {
	case "object":
		domain, err := dc.ParseObject(node)

		return &domain, err
	case "array":
		domain, err := dc.ParseArray(node)

		return &domain, err
	case "string":
		domain, err := dc.ParseString(node)

		return &domain, err
	case "number", "integer":
		domain, err := dc.ParseNumber(node)

		return &domain, err
	case "boolean":
		domain, err := dc.ParseBool(node)

		return &domain, err
	default:
		return nil, fmt.Errorf("unsupported schema object type %q", schemaType)
	}
}
