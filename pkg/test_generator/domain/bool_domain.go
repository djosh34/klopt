package domain

import (
	"encoding/json"
	"errors"
	"fmt"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.
)

// BoolDomain describes the values accepted by an OpenAPI boolean schema.
type BoolDomain struct {
	Nullable bool         `json:"nullable"`
	Enum     []types.Enum `json:"enum"`
}

// AllOfMerge intersects the boolean domain with another domain.
func (b *BoolDomain) AllOfMerge(domain types.Domain) (types.Domain, error) {
	if b == nil {
		return nil, errors.New("bool domain cannot be nil")
	}

	if allOfDomain, ok := domain.(*AllOfDomain); ok {
		mergedAllOf := &AllOfDomain{Domains: []types.Domain{b}, MergedDomain: b}

		return mergedAllOf.AllOfMerge(allOfDomain)
	}

	otherBool, ok := domain.(*BoolDomain)
	if !ok || otherBool == nil {
		nullOnly, merged, err := mergeDomainsAsNullOnly(b, domain)
		if err != nil {
			return nil, err
		}

		if merged {
			return nullOnly, nil
		}

		return nil, errors.New("domain is not BoolDomain")
	}

	merged := *b
	merged.Nullable = b.Nullable && otherBool.Nullable

	enums, err := mergeEnumsByType(b.Enum, otherBool.Enum, "boolean", merged.Nullable)
	if err != nil {
		return nil, err
	}

	merged.Enum = enums

	return &merged, nil
}

// GenerateHash returns a deterministic hash of the boolean domain.
func (b *BoolDomain) GenerateHash() (types.Hash, error) {
	if b == nil {
		return types.Hash{}, errors.New("domain of bool cannot be nil")
	}

	enums, err := filterEnumsByType(b.Enum, "boolean", b.Nullable)
	if err != nil {
		return types.Hash{}, err
	}

	value := *b
	value.Enum = enums

	return generateHash("bool", value)
}

// boolSchema contains the supported boolean Schema Object fields.
type boolSchema struct {
	Type     *string `json:"type"`
	Nullable *bool   `json:"nullable"`
}

// ParseBool parses an OpenAPI boolean Schema Object.
func (dc *Context) ParseBool(node *json.RawMessage) (BoolDomain, error) {
	jsonKV, schema, err := parseBoolNode(node)
	if err != nil {
		return BoolDomain{}, err
	}

	if typeErr := validateBoolType(jsonKV, schema.Type); typeErr != nil {
		return BoolDomain{}, typeErr
	}

	domain := BoolDomain{}
	if nullableErr := parseBoolNullable(jsonKV, schema.Nullable, &domain); nullableErr != nil {
		return BoolDomain{}, nullableErr
	}

	enums, err := parseEnumsByType(jsonKV, "boolean", domain.Nullable)
	if err != nil {
		return BoolDomain{}, err
	}

	domain.Enum = enums

	if fieldErr := validateBoolSchemaFields(jsonKV); fieldErr != nil {
		return BoolDomain{}, fieldErr
	}

	return domain, nil
}

// parseBoolNode decodes a boolean Schema Object into keyed and typed forms.
func parseBoolNode(node *json.RawMessage) (JSONKV, boolSchema, error) {
	if node == nil {
		return nil, boolSchema{}, errors.New("schema node is nil")
	}

	jsonKV := JSONKV{}
	if err := json.Unmarshal(*node, &jsonKV); err != nil {
		return nil, boolSchema{}, err
	}

	schema := boolSchema{}
	if err := json.Unmarshal(*node, &schema); err != nil {
		return nil, boolSchema{}, err
	}

	return jsonKV, schema, nil
}

// validateBoolType checks the required boolean type declaration.
func validateBoolType(jsonKV JSONKV, schemaTypeValue *string) error {
	schemaType, err := requiredSchemaType(jsonKV, schemaTypeValue)
	if err != nil {
		return err
	}

	if schemaType != "boolean" {
		return fmt.Errorf("bool domain type must be boolean, got %q", schemaType)
	}

	return nil
}

// parseBoolNullable applies the optional nullable field.
func parseBoolNullable(jsonKV JSONKV, nullable *bool, domain *BoolDomain) error {
	if _, ok := jsonKV["nullable"]; ok {
		if nullable == nil {
			return errors.New("nullable must be boolean")
		}

		domain.Nullable = *nullable
	}

	return nil
}

// validateBoolSchemaFields rejects unsupported boolean Schema Object fields.
func validateBoolSchemaFields(jsonKV JSONKV) error {
	if err := deleteAllowableKeys(jsonKV); err != nil {
		return err
	}

	delete(jsonKV, "enum")

	if len(jsonKV) != 0 {
		return fmt.Errorf("unsupported bool schema field %q", sortedJSONKeys(jsonKV)[0])
	}

	return nil
}
