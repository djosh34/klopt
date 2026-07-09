package domain

import (
	"encoding/json"
	"errors"
	"fmt"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.
)

var _ types.AllOfMerger = new(AllOfDomain)

// AllOfDomain holds the source domains and their merged intersection.
type AllOfDomain struct {
	Domains      []types.Domain
	MergedDomain types.Domain
}

// AllOfMerge returns the intersection of this allOf domain and domain.
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

	if err := mergedAllOf.mergeDomain(domain); err != nil {
		return nil, err
	}

	return mergedAllOf, nil
}

// allOfHashValue is the stable hash representation of an allOf domain.
type allOfHashValue struct {
	Domains      []*types.Hash
	MergedDomain *types.Hash
}

// GenerateHash returns a hash of the source and merged domains.
func (a *AllOfDomain) GenerateHash() (types.Hash, error) {
	if a == nil {
		return types.Hash{}, errors.New("domain of allOf cannot be nil")
	}

	domainHashes := make([]*types.Hash, 0, len(a.Domains))
	for _, allOfDomain := range a.Domains {
		var domainHash *types.Hash

		if allOfDomain != nil {
			hash, err := allOfDomain.GenerateHash()
			if err != nil {
				return types.Hash{}, err
			}

			domainHash = &hash
		}

		domainHashes = append(domainHashes, domainHash)
	}

	var mergedHash *types.Hash

	if a.MergedDomain != nil {
		hash, err := a.MergedDomain.GenerateHash()
		if err != nil {
			return types.Hash{}, err
		}

		mergedHash = &hash
	}

	return generateHash("allOf", allOfHashValue{
		Domains:      domainHashes,
		MergedDomain: mergedHash,
	})
}

// ParseAllOf parses an allOf Schema Object.
func (dc *Context) ParseAllOf(node *json.RawMessage) (AllOfDomain, error) {
	originalStore := cloneDomainStore(dc.domainStore)

	allOfDomain, err := dc.parseAllOf(node)
	if err != nil {
		dc.domainStore = originalStore

		return AllOfDomain{}, err
	}

	return allOfDomain, nil
}

// mergeDomain merges either one domain or the children of another allOf.
func (a *AllOfDomain) mergeDomain(domain types.Domain) error {
	otherAllOf, ok := domain.(*AllOfDomain)
	if !ok {
		return a.mergeOne(domain)
	}

	return a.mergeAllOf(otherAllOf)
}

// mergeAllOf flattens another allOf into this accumulator.
func (a *AllOfDomain) mergeAllOf(otherAllOf *AllOfDomain) error {
	if otherAllOf == nil {
		return errors.New("allOf domain cannot be nil")
	}

	if len(otherAllOf.Domains) == 0 && otherAllOf.MergedDomain != nil {
		return a.mergeOne(otherAllOf.MergedDomain)
	}

	for _, childDomain := range otherAllOf.Domains {
		if err := a.mergeOne(childDomain); err != nil {
			return err
		}
	}

	return nil
}

// parseAllOf parses an allOf schema without managing store rollback.
func (dc *Context) parseAllOf(node *json.RawMessage) (AllOfDomain, error) {
	if node == nil {
		return AllOfDomain{}, errors.New("schema node is nil")
	}

	jsonKV := JSONKV{}
	if err := json.Unmarshal(*node, &jsonKV); err != nil {
		return AllOfDomain{}, err
	}

	allOfRaw, err := validateAllOfSchema(jsonKV)
	if err != nil {
		return AllOfDomain{}, err
	}

	allOfDomain := AllOfDomain{}
	if err := dc.parseAllOfItems(allOfRaw, &allOfDomain); err != nil {
		return AllOfDomain{}, err
	}

	if err := dc.parseAllOfSiblings(allOfSiblingFields(jsonKV), &allOfDomain); err != nil {
		return AllOfDomain{}, err
	}

	return allOfDomain, nil
}

// cloneDomainStore copies a domain store for parser rollback.
func cloneDomainStore(store domainStore) domainStore {
	if store == nil {
		return nil
	}

	cloned := make(domainStore, len(store))
	for domain := range store {
		cloned[domain] = struct{}{}
	}

	return cloned
}

// allOfSiblingFields returns non-documentation fields beside allOf.
func allOfSiblingFields(jsonKV JSONKV) JSONKV {
	siblingKV := make(JSONKV, len(jsonKV))
	for key, value := range jsonKV {
		if key == "allOf" || key == "title" || key == "description" ||
			isSpecificationExtension(key) && !isGeneratorSchemaExtension(key) {
			continue
		}

		siblingKV[key] = value
	}

	return siblingKV
}

// parseAllOfSiblings applies supported sibling constraints.
func (dc *Context) parseAllOfSiblings(siblingKV JSONKV, allOfDomain *AllOfDomain) error {
	parsedNullable, err := parseNullableSibling(siblingKV)
	if err != nil {
		return err
	}

	if parsedNullable {
		return nil
	}

	return dc.parseGeneralSibling(siblingKV, allOfDomain)
}

// mergeOne adds one domain to an allOf accumulator.
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

// validateAllOfSchema validates the allOf container and returns its raw items.
func validateAllOfSchema(jsonKV JSONKV) (json.RawMessage, error) {
	if jsonKV == nil {
		return nil, errors.New("schema node must be object")
	}

	if err := validateSchemaDocumentation(jsonKV); err != nil {
		return nil, err
	}

	allOfRaw, ok := jsonKV["allOf"]
	if !ok {
		return nil, errors.New("allOf is required")
	}

	for _, key := range []string{"oneOf", "anyOf", "not", "discriminator"} {
		if _, ok := jsonKV[key]; ok {
			return nil, fmt.Errorf("%s is unsupported with allOf", key)
		}
	}

	for _, key := range sortedJSONKeys(jsonKV) {
		if !isAllowedAllOfSiblingKey(key) {
			return nil, fmt.Errorf("unsupported allOf schema field %q", key)
		}
	}

	return allOfRaw, nil
}

// parseAllOfItems parses and merges each allOf item into allOfDomain.
func (dc *Context) parseAllOfItems(allOfRaw json.RawMessage, allOfDomain *AllOfDomain) error {
	if string(allOfRaw) == "null" {
		return errors.New("allOf cannot be null")
	}

	var allOfItems []json.RawMessage
	if err := json.Unmarshal(allOfRaw, &allOfItems); err != nil {
		return errors.New("allOf must be array")
	}

	if len(allOfItems) == 0 {
		return errors.New("allOf cannot be empty")
	}

	for _, allOfItem := range allOfItems {
		if err := validateAllOfItem(allOfItem); err != nil {
			return err
		}

		domain, err := dc.Parse(&allOfItem)
		if err != nil {
			return err
		}

		if domain == nil {
			return errors.New("parsed allOf item cannot be nil")
		}

		if err := mergeIntoAllOf(allOfDomain, domain); err != nil {
			return err
		}
	}

	return nil
}

// validateAllOfItem validates one raw allOf item.
func validateAllOfItem(allOfItem json.RawMessage) error {
	if string(allOfItem) == "null" {
		return errors.New("allOf item cannot be null")
	}

	itemKV := JSONKV{}
	if err := json.Unmarshal(allOfItem, &itemKV); err != nil {
		return errors.New("allOf item must be object")
	}

	if len(itemKV) == 0 {
		return errors.New("allOf item cannot be empty schema")
	}

	for _, key := range []string{"oneOf", "anyOf", "not", "discriminator"} {
		if _, ok := itemKV[key]; ok {
			return fmt.Errorf("allOf item %s is unsupported", key)
		}
	}

	if _, ok := itemKV["$ref"]; ok && len(itemKV) != 1 {
		return errors.New("$ref with siblings is unsupported")
	}

	return nil
}

// parseNullableSibling validates a nullable-only sibling Schema Object.
func parseNullableSibling(siblingKV JSONKV) (bool, error) {
	if len(siblingKV) != 1 {
		return false, nil
	}

	nullableRaw, ok := siblingKV["nullable"]
	if !ok {
		return false, nil
	}

	var nullable *bool
	if err := json.Unmarshal(nullableRaw, &nullable); err != nil {
		return false, errors.New("nullable must be boolean")
	}

	if nullable == nil {
		return false, errors.New("nullable must be boolean")
	}

	return true, nil
}

// parseGeneralSibling parses and merges non-documentation allOf siblings.
func (dc *Context) parseGeneralSibling(siblingKV JSONKV, allOfDomain *AllOfDomain) error {
	if len(siblingKV) == 0 {
		return nil
	}

	siblingRaw, err := json.Marshal(siblingKV)
	if err != nil {
		return err
	}

	raw := json.RawMessage(siblingRaw)

	domain, err := dc.Parse(&raw)
	if err != nil {
		return err
	}

	if domain == nil {
		return errors.New("parsed allOf sibling cannot be nil")
	}

	return mergeIntoAllOf(allOfDomain, domain)
}

// mergeIntoAllOf assigns the non-mutating merge result to a parser accumulator.
func mergeIntoAllOf(allOfDomain *AllOfDomain, domain types.Domain) error {
	mergedDomain, err := allOfDomain.AllOfMerge(domain)
	if err != nil {
		return err
	}

	mergedAllOf, ok := mergedDomain.(*AllOfDomain)
	if !ok {
		return errors.New("allOf merge returned unexpected domain type")
	}

	*allOfDomain = *mergedAllOf

	return nil
}

// isAllowedAllOfSiblingKey reports whether key is supported beside allOf.
func isAllowedAllOfSiblingKey(key string) bool {
	switch key {
	case "allOf", "type", "nullable", "title", "description",
		"enum", "minLength", "maxLength", "pattern", "format", "x-valid-examples", "x-invalid-examples",
		"minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf",
		"items", "minItems", "maxItems",
		"required", "properties", "additionalProperties", "minProperties", "maxProperties":
		return true
	default:
		return isSpecificationExtension(key)
	}
}
