package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.
)

// ArrayDomain describes the values accepted by an OpenAPI array schema.
type ArrayDomain struct {
	Nullable bool `json:"nullable"`

	Enum []types.Enum `json:"enum"`

	Items types.Domain `json:"items"`

	MinItems int  `json:"minItems"`
	MaxItems *int `json:"maxItems"`
}

// AllOfMerge intersects the array domain with another domain.
func (a *ArrayDomain) AllOfMerge(domain types.Domain) (types.Domain, error) {
	if a == nil {
		return nil, errors.New("array domain cannot be nil")
	}

	if allOfDomain, ok := domain.(*AllOfDomain); ok {
		mergedAllOf := &AllOfDomain{Domains: []types.Domain{a}, MergedDomain: a}

		return mergedAllOf.AllOfMerge(allOfDomain)
	}

	otherArray, ok := domain.(*ArrayDomain)
	if !ok || otherArray == nil {
		return mergeMismatchedArrayDomains(a, domain)
	}

	merged := *a
	merged.Nullable = a.Nullable && otherArray.Nullable

	enums, err := mergeEnumsByType(a.Enum, otherArray.Enum, "array", merged.Nullable)
	if err != nil {
		return nil, err
	}

	merged.Enum = enums

	items, compatibleItems, err := mergeArrayItems(a.Items, otherArray.Items)
	if err != nil {
		return nil, err
	}

	merged.Items = items

	merged.MinItems = max(a.MinItems, otherArray.MinItems)
	merged.MaxItems = tighterMax(a.MaxItems, otherArray.MaxItems)

	if !compatibleItems {
		merged.MaxItems = tighterMax(merged.MaxItems, new(0))
	}

	if err := finalizeArrayDomain(&merged); err != nil {
		return nil, err
	}

	return &merged, nil
}

// mergeMismatchedArrayDomains returns null when differently typed domains both allow it.
func mergeMismatchedArrayDomains(left *ArrayDomain, right types.Domain) (types.Domain, error) {
	nullOnly, merged, err := mergeDomainsAsNullOnly(left, right)
	if err != nil {
		return nil, err
	}

	if merged {
		return nullOnly, nil
	}

	return nil, errors.New("domain is not ArrayDomain")
}

// mergeArrayItems intersects item schemas and reports an empty item domain separately.
func mergeArrayItems(left types.Domain, right types.Domain) (types.Domain, bool, error) {
	if err := validateArrayItemDomain(left, "left"); err != nil {
		return nil, false, err
	}

	if err := validateArrayItemDomain(right, "right"); err != nil {
		return nil, false, err
	}

	if left == nil {
		return right, true, nil
	}

	if right == nil {
		return left, true, nil
	}

	items, compatible := intersectValidArrayItems(left, right)

	return items, compatible, nil
}

// validateArrayItemDomain rejects a malformed non-nil item domain before intersection.
func validateArrayItemDomain(domain types.Domain, side string) error {
	if domain == nil {
		return nil
	}

	if _, err := domain.GenerateHash(); err != nil {
		return fmt.Errorf("%s array items: %w", side, err)
	}

	return nil
}

// intersectValidArrayItems treats a merge failure between valid domains as an empty intersection.
func intersectValidArrayItems(left types.Domain, right types.Domain) (types.Domain, bool) {
	items, err := left.AllOfMerge(right)
	if err != nil {
		return nil, false
	}

	return items, true
}

// arrayHashValue replaces the item domain with its deterministic hash.
type arrayHashValue struct {
	Nullable bool `json:"nullable"`

	Enum []types.Enum `json:"enum"`

	Items *types.Hash `json:"items"`

	MinItems int  `json:"minItems"`
	MaxItems *int `json:"maxItems"`
}

// GenerateHash returns a deterministic hash of the array domain.
func (a *ArrayDomain) GenerateHash() (types.Hash, error) {
	if a == nil {
		return types.Hash{}, errors.New("domain of array cannot be nil")
	}

	value := *a
	if err := finalizeArrayDomain(&value); err != nil {
		return types.Hash{}, err
	}

	var itemsHash *types.Hash

	if value.Items != nil {
		hash, hashErr := value.Items.GenerateHash()
		if hashErr != nil {
			return types.Hash{}, hashErr
		}

		itemsHash = &hash
	}

	return generateHash("array", arrayHashValue{
		Nullable: value.Nullable,
		Enum:     value.Enum,
		Items:    itemsHash,
		MinItems: value.MinItems,
		MaxItems: value.MaxItems,
	})
}

// finalizeArrayDomain filters enum values and rejects unsatisfiable array bounds.
func finalizeArrayDomain(domain *ArrayDomain) error {
	if err := validateArrayConstraintValues(domain); err != nil {
		return err
	}

	enums, err := filterEnumsByType(domain.Enum, "array", domain.Nullable)
	if err != nil {
		return err
	}

	domain.Enum = enums
	if domain.Enum != nil {
		if filterErr := filterArrayEnumsByConstraints(domain); filterErr != nil {
			return filterErr
		}
	}

	return validateArraySatisfiability(domain)
}

// validateArrayConstraintValues rejects invalid item-count keyword values.
func validateArrayConstraintValues(domain *ArrayDomain) error {
	if domain.MinItems < 0 {
		return errors.New("minItems cannot be negative")
	}

	if domain.MaxItems != nil && *domain.MaxItems < 0 {
		return errors.New("maxItems cannot be negative")
	}

	return nil
}

// validateArraySatisfiability permits contradictory bounds only when null remains valid.
func validateArraySatisfiability(domain *ArrayDomain) error {
	if domain.MaxItems == nil || domain.MinItems <= *domain.MaxItems {
		return nil
	}

	allowsNull, err := enumAllowsNull(domain.Nullable, domain.Enum)
	if err != nil {
		return err
	}

	if allowsNull {
		return nil
	}

	return errors.New("minItems cannot exceed maxItems")
}

// filterArrayEnumsByConstraints retains enum arrays satisfying array and item constraints.
func filterArrayEnumsByConstraints(domain *ArrayDomain) error {
	filtered := make([]types.Enum, 0, len(domain.Enum))

	for _, enumValue := range domain.Enum {
		allowed, err := domainAllowsCanonicalJSONValue(domain, enumValue)
		if err != nil {
			return err
		}

		if allowed {
			filtered = append(filtered, enumValue)
		}
	}

	if len(filtered) == 0 {
		return errors.New("enum has no values compatible with array constraints")
	}

	domain.Enum = filtered

	return nil
}

// arraySchema contains the supported array Schema Object fields.
type arraySchema struct {
	Type     *string          `json:"type"`
	Nullable *bool            `json:"nullable"`
	Items    *json.RawMessage `json:"items"`
	MinItems *int             `json:"minItems"`
	MaxItems *int             `json:"maxItems"`
}

// ParseArray parses an OpenAPI array Schema Object.
func (dc *Context) ParseArray(node *json.RawMessage) (ArrayDomain, error) {
	originalStore := cloneDomainStore(dc.domainStore)

	arrayDomain, err := dc.parseArray(node)
	if err != nil {
		dc.domainStore = originalStore

		return ArrayDomain{}, err
	}

	return arrayDomain, nil
}

// parseArray parses an array schema without managing store rollback.
func (dc *Context) parseArray(node *json.RawMessage) (ArrayDomain, error) {
	jsonKV, schema, err := parseArrayNode(node)
	if err != nil {
		return ArrayDomain{}, err
	}

	if typeErr := validateArrayType(jsonKV, schema.Type); typeErr != nil {
		return ArrayDomain{}, typeErr
	}

	domain := ArrayDomain{}
	if nullableErr := parseArrayNullable(jsonKV, schema.Nullable, &domain); nullableErr != nil {
		return ArrayDomain{}, nullableErr
	}

	enums, err := parseEnumsByType(jsonKV, "array", domain.Nullable)
	if err != nil {
		return ArrayDomain{}, err
	}

	domain.Enum = enums

	itemsRaw, parseItems, err := parseArrayItems(jsonKV, schema.Items)
	if err != nil {
		return ArrayDomain{}, err
	}

	if boundsErr := parseArrayBounds(jsonKV, schema, &domain); boundsErr != nil {
		return ArrayDomain{}, boundsErr
	}

	if fieldErr := validateArraySchemaFields(jsonKV); fieldErr != nil {
		return ArrayDomain{}, fieldErr
	}

	if err := dc.parseArrayItem(itemsRaw, parseItems, &domain); err != nil {
		return ArrayDomain{}, err
	}

	if finalizeErr := finalizeArrayDomain(&domain); finalizeErr != nil {
		return ArrayDomain{}, finalizeErr
	}

	return domain, nil
}

// parseArrayItem parses a nonempty item Schema Object.
func (dc *Context) parseArrayItem(itemsRaw json.RawMessage, parseItems bool, domain *ArrayDomain) error {
	if !parseItems {
		return nil
	}

	itemsDomain, err := dc.Parse(&itemsRaw)
	if err != nil {
		return fmt.Errorf("items: %w", err)
	}

	if itemsDomain == nil {
		return errors.New("parsed items domain cannot be nil")
	}

	domain.Items = itemsDomain

	return nil
}

// validateArrayType checks the required array type declaration.
func validateArrayType(jsonKV JSONKV, schemaTypeValue *string) error {
	schemaType, err := requiredSchemaType(jsonKV, schemaTypeValue)
	if err != nil {
		return err
	}

	if schemaType != "array" {
		return fmt.Errorf("array domain type must be array, got %q", schemaType)
	}

	return nil
}

// parseArrayNode decodes an array Schema Object into keyed and typed forms.
func parseArrayNode(node *json.RawMessage) (JSONKV, arraySchema, error) {
	if node == nil {
		return nil, arraySchema{}, errors.New("schema node is nil")
	}

	jsonKV := JSONKV{}
	if err := json.Unmarshal(*node, &jsonKV); err != nil {
		return nil, arraySchema{}, err
	}

	schema := arraySchema{}
	if err := json.Unmarshal(*node, &schema); err != nil {
		return nil, arraySchema{}, err
	}

	return jsonKV, schema, nil
}

// parseArrayNullable applies the optional nullable field.
func parseArrayNullable(jsonKV JSONKV, nullable *bool, domain *ArrayDomain) error {
	if _, ok := jsonKV["nullable"]; ok {
		if nullable == nil {
			return errors.New("nullable must be boolean")
		}

		domain.Nullable = *nullable
	}

	return nil
}

// parseArrayItems validates and returns the required item Schema Object.
func parseArrayItems(jsonKV JSONKV, items *json.RawMessage) (json.RawMessage, bool, error) {
	if _, ok := jsonKV["items"]; !ok {
		return nil, false, errors.New("items is required")
	}

	if items == nil {
		return nil, false, errors.New("items cannot be null")
	}

	itemsRaw := *items

	trimmedItemsRaw := strings.TrimSpace(string(itemsRaw))
	if trimmedItemsRaw != "" && trimmedItemsRaw[0] == '[' {
		return nil, false, errors.New("items cannot be an array")
	}

	itemsObject := JSONKV{}
	if err := json.Unmarshal(itemsRaw, &itemsObject); err != nil {
		return nil, false, errors.New("items must be object")
	}

	return itemsRaw, len(itemsObject) != 0, nil
}

// parseArrayBounds applies and validates minItems and maxItems.
func parseArrayBounds(jsonKV JSONKV, schema arraySchema, domain *ArrayDomain) error {
	if _, ok := jsonKV["minItems"]; ok {
		if schema.MinItems == nil {
			return errors.New("minItems cannot be null")
		}

		if *schema.MinItems < 0 {
			return errors.New("minItems cannot be negative")
		}

		domain.MinItems = *schema.MinItems
	}

	if _, ok := jsonKV["maxItems"]; ok {
		if schema.MaxItems == nil {
			return errors.New("maxItems cannot be null")
		}

		if *schema.MaxItems < 0 {
			return errors.New("maxItems cannot be negative")
		}

		domain.MaxItems = schema.MaxItems
	}

	return nil
}

// validateArraySchemaFields rejects unsupported array Schema Object fields.
func validateArraySchemaFields(jsonKV JSONKV) error {
	if err := deleteAllowableKeys(jsonKV); err != nil {
		return err
	}

	for _, key := range []string{"enum", "items", "minItems", "maxItems"} {
		delete(jsonKV, key)
	}

	if _, ok := jsonKV["uniqueItems"]; ok {
		return errors.New("uniqueItems is unsupported")
	}

	if len(jsonKV) != 0 {
		keys := sortedJSONKeys(jsonKV)

		return fmt.Errorf("unsupported array schema field %q", keys[0])
	}

	return nil
}
