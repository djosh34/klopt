package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.
)

// AdditionalPropertyKind identifies how an object handles undeclared properties.
type AdditionalPropertyKind int

// errObjectValuesIncompatible marks an empty intersection of non-null object values.
var errObjectValuesIncompatible = errors.New("object domains have no compatible object value")

const (
	// AdditionalTrue allows undeclared properties without constraints.
	AdditionalTrue AdditionalPropertyKind = iota
	// AdditionalFalse forbids undeclared properties.
	AdditionalFalse
	// AdditionalSchema validates undeclared properties against a schema domain.
	AdditionalSchema
)

// Property describes a named object property.
type Property struct {
	types.Domain

	Key      string
	Required bool
}

// propertyHashValue is the stable hash representation of a property.
type propertyHashValue struct {
	Key      string
	Hasher   *types.Hash
	Required bool
}

// GenerateHash returns a hash of the property and its domain.
func (p *Property) GenerateHash() (types.Hash, error) {
	if p == nil {
		return types.Hash{}, errors.New("property cannot be nil")
	}

	var domainHash *types.Hash

	if p.Domain != nil {
		hash, err := p.Domain.GenerateHash()
		if err != nil {
			return types.Hash{}, err
		}

		domainHash = &hash
	}

	return generateHash("property", propertyHashValue{
		Key:      p.Key,
		Hasher:   domainHash,
		Required: p.Required,
	})
}

// ObjectDomain describes the supported constraints for object values.
type ObjectDomain struct {
	AdditionalPropertyKind

	Nullable bool

	Enum []types.Enum

	Properties []Property

	AdditionalPropertyDomain types.Domain

	MinProps int
	MaxProps *int
}

// AllOfMerge returns the intersection of this object domain and domain.
func (o *ObjectDomain) AllOfMerge(domain types.Domain) (types.Domain, error) {
	if o == nil {
		return nil, errors.New("object domain cannot be nil")
	}

	if allOfDomain, ok := domain.(*AllOfDomain); ok {
		mergedAllOf := &AllOfDomain{Domains: []types.Domain{o}, MergedDomain: o}

		return mergedAllOf.AllOfMerge(allOfDomain)
	}

	otherObject, ok := domain.(*ObjectDomain)
	if !ok || otherObject == nil {
		nullOnly, merged, err := mergeDomainsAsNullOnly(o, domain)
		if err != nil {
			return nil, err
		}

		if merged {
			return nullOnly, nil
		}

		return nil, errors.New("domain is not ObjectDomain")
	}

	merged, err := mergeObjectDomains(o, otherObject)
	if !errors.Is(err, errObjectValuesIncompatible) {
		return merged, err
	}

	nullOnly, allowsNull, nullErr := mergeDomainsAsNullOnly(o, otherObject)
	if nullErr != nil {
		return nil, nullErr
	}

	if allowsNull {
		return nullOnly, nil
	}

	return nil, err
}

// mergeObjectDomains intersects two concrete object domains.
func mergeObjectDomains(left *ObjectDomain, right *ObjectDomain) (types.Domain, error) {
	if err := validateObjectMergeInputs(left, right); err != nil {
		return nil, err
	}

	merged := *left
	merged.Nullable = left.Nullable && right.Nullable

	enums, err := mergeEnums(left.Enum, right.Enum)
	if err != nil {
		return nil, err
	}

	merged.Enum, err = filterEnumsByType(enums, "object", merged.Nullable)
	if err != nil {
		return nil, err
	}

	merged.MinProps = max(left.MinProps, right.MinProps)
	merged.MaxProps = tighterMax(left.MaxProps, right.MaxProps)

	merged.Properties, err = mergeObjectProperties(left, right)
	if err != nil {
		return nil, err
	}

	additionalKind, additionalDomain, err := mergeAdditionalProperties(left, right)
	if err != nil {
		return nil, err
	}

	merged.AdditionalPropertyKind = additionalKind
	merged.AdditionalPropertyDomain = additionalDomain

	if err := finalizeObjectDomain(&merged); err != nil {
		return nil, err
	}

	return &merged, nil
}

// validateObjectMergeInputs rejects malformed programmatic domains before intersection.
func validateObjectMergeInputs(left *ObjectDomain, right *ObjectDomain) error {
	if err := validateObjectPropertyBounds(left); err != nil {
		return err
	}

	if err := validateObjectPropertyBounds(right); err != nil {
		return err
	}

	if err := validateAdditionalPropertySchema(left); err != nil {
		return err
	}

	leftValue := *left
	if err := canonicalizeObjectProperties(&leftValue); err != nil {
		return err
	}

	rightValue := *right
	if err := canonicalizeObjectProperties(&rightValue); err != nil {
		return err
	}

	return validateAdditionalPropertySchema(right)
}

// validateAdditionalPropertySchema checks an additionalProperties schema domain.
func validateAdditionalPropertySchema(objectDomain *ObjectDomain) error {
	switch objectDomain.AdditionalPropertyKind {
	case AdditionalTrue, AdditionalFalse:
		return nil
	case AdditionalSchema:
		if objectDomain.AdditionalPropertyDomain == nil {
			return errors.New("additional property schema domain cannot be nil")
		}

		if _, err := objectDomain.AdditionalPropertyDomain.GenerateHash(); err != nil {
			return fmt.Errorf("additional property schema domain: %w", err)
		}

		return nil
	default:
		return errors.New("unknown additionalProperties kind")
	}
}

// tighterMax returns the smaller non-nil maximum.
func tighterMax(first *int, second *int) *int {
	if first == nil {
		return second
	}

	if second == nil || *first <= *second {
		return first
	}

	return second
}

// mergeObjectProperties intersects the named properties of two objects.
func mergeObjectProperties(leftObject *ObjectDomain, rightObject *ObjectDomain) ([]Property, error) {
	leftProperties := propertiesByKey(leftObject.Properties)
	rightProperties := propertiesByKey(rightObject.Properties)
	mergedProperties := make(map[string]Property, len(leftObject.Properties)+len(rightObject.Properties))

	for _, leftProperty := range sortedProperties(leftProperties) {
		key := leftProperty.Key

		if rightProperty, ok := rightProperties[key]; ok {
			property, mergeErr := mergeMatchedProperty(key, leftProperty, rightProperty)
			if mergeErr != nil {
				return nil, mergeErr
			}

			mergedProperties[key] = property

			continue
		}

		if mergeErr := mergeUnmatchedProperty(key, leftProperty, rightObject, mergedProperties); mergeErr != nil {
			return nil, mergeErr
		}
	}

	for _, rightProperty := range sortedProperties(rightProperties) {
		key := rightProperty.Key

		if _, exists := leftProperties[key]; exists {
			continue
		}

		if mergeErr := mergeUnmatchedProperty(key, rightProperty, leftObject, mergedProperties); mergeErr != nil {
			return nil, mergeErr
		}
	}

	return sortedProperties(mergedProperties), nil
}

// propertiesByKey indexes properties by name.
func propertiesByKey(properties []Property) map[string]Property {
	mappedProperties := make(map[string]Property, len(properties))
	for _, property := range properties {
		mappedProperties[property.Key] = property
	}

	return mappedProperties
}

// mergeMatchedProperty intersects two declarations for the same property.
func mergeMatchedProperty(key string, leftProperty Property, rightProperty Property) (Property, error) {
	property := Property{Key: key, Required: leftProperty.Required || rightProperty.Required}

	switch {
	case leftProperty.Domain == nil:
		property.Domain = rightProperty.Domain
	case rightProperty.Domain == nil:
		property.Domain = leftProperty.Domain
	default:
		return mergeMatchedPropertyDomains(property, leftProperty.Domain, rightProperty.Domain)
	}

	return property, nil
}

// mergeMatchedPropertyDomains intersects two present property domains.
func mergeMatchedPropertyDomains(property Property, left types.Domain, right types.Domain) (Property, error) {
	domain, err := left.AllOfMerge(right)
	if err == nil {
		property.Domain = domain

		return property, nil
	}

	if property.Required && domainsHaveDifferentConcreteTypes(left, right) {
		return Property{}, fmt.Errorf(
			"%w: property %q: %w",
			errObjectValuesIncompatible,
			property.Key,
			err,
		)
	}

	return Property{}, fmt.Errorf("property %q: %w", property.Key, err)
}

// domainsHaveDifferentConcreteTypes reports a definitely empty non-null type intersection.
func domainsHaveDifferentConcreteTypes(left types.Domain, right types.Domain) bool {
	if _, ok := right.(*AllOfDomain); ok {
		return false
	}

	switch left.(type) {
	case *ArrayDomain:
		_, same := right.(*ArrayDomain)

		return !same
	case *BoolDomain:
		_, same := right.(*BoolDomain)

		return !same
	case *NumberDomain:
		_, same := right.(*NumberDomain)

		return !same
	case *ObjectDomain:
		_, same := right.(*ObjectDomain)

		return !same
	case *StringDomain:
		_, same := right.(*StringDomain)

		return !same
	default:
		return false
	}
}

// mergeUnmatchedProperty applies the other object's additionalProperties policy.
func mergeUnmatchedProperty(
	key string,
	property Property,
	otherObject *ObjectDomain,
	propertiesByKey map[string]Property,
) error {
	mergedProperty, keep, mergeErr := mergePropertyWithAdditional(property, otherObject)
	if mergeErr != nil {
		return mergeErr
	}

	if keep {
		propertiesByKey[key] = mergedProperty
	}

	return nil
}

// sortedProperties returns properties ordered by key.
func sortedProperties(propertiesByKey map[string]Property) []Property {
	propertyKeys := make([]string, 0, len(propertiesByKey))
	for propertyKey := range propertiesByKey {
		propertyKeys = append(propertyKeys, propertyKey)
	}

	sort.Strings(propertyKeys)

	if len(propertyKeys) == 0 {
		return nil
	}

	properties := make([]Property, 0, len(propertyKeys))
	for _, propertyKey := range propertyKeys {
		properties = append(properties, propertiesByKey[propertyKey])
	}

	return properties
}

// mergePropertyWithAdditional intersects a property with additionalProperties.
func mergePropertyWithAdditional(property Property, otherObject *ObjectDomain) (Property, bool, error) {
	switch otherObject.AdditionalPropertyKind {
	case AdditionalTrue:
		return property, true, nil
	case AdditionalFalse:
		if property.Required {
			return Property{}, false, fmt.Errorf(
				"%w: required property %q is forbidden by additionalProperties false",
				errObjectValuesIncompatible,
				property.Key,
			)
		}

		return Property{}, false, nil
	case AdditionalSchema:
		return mergePropertyWithAdditionalSchema(property, otherObject.AdditionalPropertyDomain)
	default:
		return Property{}, false, errors.New("unknown additionalProperties kind")
	}
}

// mergePropertyWithAdditionalSchema applies a schema-valued additionalProperties constraint.
func mergePropertyWithAdditionalSchema(
	property Property,
	additionalDomain types.Domain,
) (Property, bool, error) {
	if additionalDomain == nil {
		return Property{}, false, errors.New("additional property schema domain cannot be nil")
	}

	if property.Domain == nil {
		property.Domain = additionalDomain

		return property, true, nil
	}

	domain, err := property.AllOfMerge(additionalDomain)
	if err == nil {
		property.Domain = domain

		return property, true, nil
	}

	if property.Required && domainsHaveDifferentConcreteTypes(property.Domain, additionalDomain) {
		return Property{}, false, fmt.Errorf(
			"%w: required property %q: %w",
			errObjectValuesIncompatible,
			property.Key,
			err,
		)
	}

	return Property{}, false, fmt.Errorf("property %q: %w", property.Key, err)
}

// mergeAdditionalProperties intersects two additionalProperties policies.
func mergeAdditionalProperties(
	first *ObjectDomain,
	second *ObjectDomain,
) (AdditionalPropertyKind, types.Domain, error) {
	if first.AdditionalPropertyKind == AdditionalFalse || second.AdditionalPropertyKind == AdditionalFalse {
		return AdditionalFalse, nil, nil
	}

	if first.AdditionalPropertyKind == AdditionalSchema && second.AdditionalPropertyKind == AdditionalSchema {
		domain, mergeErr := first.AdditionalPropertyDomain.AllOfMerge(second.AdditionalPropertyDomain)
		if mergeErr != nil {
			if domainsHaveDifferentConcreteTypes(
				first.AdditionalPropertyDomain,
				second.AdditionalPropertyDomain,
			) {
				return AdditionalFalse, nil, nil
			}

			return AdditionalSchema, nil, mergeErr
		}

		return AdditionalSchema, domain, nil
	}

	if first.AdditionalPropertyKind == AdditionalSchema {
		return AdditionalSchema, first.AdditionalPropertyDomain, nil
	}

	if second.AdditionalPropertyKind == AdditionalSchema {
		return AdditionalSchema, second.AdditionalPropertyDomain, nil
	}

	return AdditionalTrue, nil, nil
}

// objectHashValue is the stable hash representation of an object domain.
type objectHashValue struct {
	AdditionalPropertyKind

	Nullable bool

	Enum []types.Enum

	Properties []*types.Hash

	AdditionalPropertyDomain *types.Hash

	MinProps int
	MaxProps *int
}

// GenerateHash returns a hash of the object domain.
func (o *ObjectDomain) GenerateHash() (types.Hash, error) {
	if o == nil {
		return types.Hash{}, errors.New("object domain cannot be nil")
	}

	value := *o
	if err := finalizeObjectDomain(&value); err != nil {
		return types.Hash{}, err
	}

	propertyHashes := make([]*types.Hash, 0, len(value.Properties))
	for _, property := range value.Properties {
		hash, err := property.GenerateHash()
		if err != nil {
			return types.Hash{}, err
		}

		propertyHashes = append(propertyHashes, &hash)
	}

	var additionalPropertyHash *types.Hash

	if value.AdditionalPropertyDomain != nil {
		hash, err := value.AdditionalPropertyDomain.GenerateHash()
		if err != nil {
			return types.Hash{}, err
		}

		additionalPropertyHash = &hash
	}

	return generateHash("object", objectHashValue{
		Nullable:                 value.Nullable,
		Enum:                     value.Enum,
		Properties:               propertyHashes,
		AdditionalPropertyKind:   value.AdditionalPropertyKind,
		AdditionalPropertyDomain: additionalPropertyHash,
		MinProps:                 value.MinProps,
		MaxProps:                 value.MaxProps,
	})
}

// JSONObject is the decoding shape for supported object Schema Object fields.
type JSONObject struct {
	Type                 string           `json:"type"`
	Nullable             *bool            `json:"nullable"`
	Required             []string         `json:"required"`
	Properties           JSONKV           `json:"properties"`
	AdditionalProperties *json.RawMessage `json:"additionalProperties"`
	MinProperties        *int             `json:"minProperties"`
	MaxProperties        *int             `json:"maxProperties"`
}

// PropertyAlreadyExistsError reports a duplicate property declaration.
type PropertyAlreadyExistsError struct {
	Key string
}

// Error returns the duplicate-property message.
func (p *PropertyAlreadyExistsError) Error() string {
	return fmt.Sprintf("property %q already exists in object", p.Key)
}

// ParseObject parses an object Schema Object.
func (dc *Context) ParseObject(node *json.RawMessage) (ObjectDomain, error) {
	originalStore := cloneDomainStore(dc.domainStore)

	objectDomain, err := dc.parseObject(node)
	if err != nil {
		dc.domainStore = originalStore

		return ObjectDomain{}, err
	}

	return objectDomain, nil
}

// parseObject parses an object schema without managing store rollback.
func (dc *Context) parseObject(node *json.RawMessage) (ObjectDomain, error) {
	jsonKV, jsonObject, objectDomain, err := parseObjectHeader(node)
	if err != nil {
		return ObjectDomain{}, err
	}

	properties, err := dc.parseObjectProperties(jsonKV, jsonObject.Properties)
	if err != nil {
		return ObjectDomain{}, err
	}

	if err := dc.parseObjectAdditionalProperties(
		jsonKV,
		jsonObject.AdditionalProperties,
		&objectDomain,
	); err != nil {
		return ObjectDomain{}, err
	}

	if err := applyRequiredObjectProperties(jsonKV, jsonObject.Required, properties, &objectDomain); err != nil {
		return ObjectDomain{}, err
	}

	objectDomain.Properties = dc.storeSortedObjectProperties(properties)
	if err := parseObjectBounds(jsonKV, jsonObject, &objectDomain); err != nil {
		return ObjectDomain{}, err
	}

	if err := finalizeObjectDomain(&objectDomain); err != nil {
		return ObjectDomain{}, err
	}

	if err := deleteAllowableKeys(jsonKV); err != nil {
		return ObjectDomain{}, err
	}

	if err := rejectUnsupportedObjectKeys(jsonKV); err != nil {
		return ObjectDomain{}, err
	}

	return objectDomain, nil
}

// parseObjectHeader parses fields shared by every object schema.
func parseObjectHeader(node *json.RawMessage) (JSONKV, JSONObject, ObjectDomain, error) {
	if node == nil {
		return nil, JSONObject{}, ObjectDomain{}, errors.New("schema node is nil")
	}

	jsonKV, jsonObject, err := decodeJSONObject(node)
	if err != nil {
		return nil, JSONObject{}, ObjectDomain{}, err
	}

	if jsonObject.Type != "object" {
		return nil, JSONObject{}, ObjectDomain{}, fmt.Errorf(
			"object schema type must be object, got %q",
			jsonObject.Type,
		)
	}

	enums, _, err := parseEnums(jsonKV)
	if err != nil {
		return nil, JSONObject{}, ObjectDomain{}, err
	}

	objectDomain := ObjectDomain{Enum: enums}

	if _, ok := jsonKV["nullable"]; ok {
		if jsonObject.Nullable == nil {
			return nil, JSONObject{}, ObjectDomain{}, errors.New("nullable must be boolean")
		}

		objectDomain.Nullable = *jsonObject.Nullable
	}

	objectDomain.Enum, err = filterEnumsByType(enums, "object", objectDomain.Nullable)
	if err != nil {
		return nil, JSONObject{}, ObjectDomain{}, err
	}

	delete(jsonKV, "enum")

	return jsonKV, jsonObject, objectDomain, nil
}

// decodeJSONObject decodes both the raw field map and typed object fields.
func decodeJSONObject(node *json.RawMessage) (JSONKV, JSONObject, error) {
	jsonKV := make(JSONKV)
	if err := json.Unmarshal(*node, &jsonKV); err != nil {
		return nil, JSONObject{}, err
	}

	jsonObject := JSONObject{}
	if err := json.Unmarshal(*node, &jsonObject); err != nil {
		return nil, JSONObject{}, err
	}

	return jsonKV, jsonObject, nil
}

// parseObjectProperties parses the explicitly declared property schemas.
func (dc *Context) parseObjectProperties(jsonKV JSONKV, rawProperties JSONKV) (map[string]Property, error) {
	properties := make(map[string]Property, len(rawProperties))
	if _, ok := jsonKV["properties"]; !ok {
		return properties, nil
	}

	delete(jsonKV, "properties")

	if rawProperties == nil {
		return nil, errors.New("properties must be an object")
	}

	for _, propertyKey := range sortedJSONKeys(rawProperties) {
		propertyValue := rawProperties[propertyKey]

		property, err := dc.parseObjectProperty(propertyKey, propertyValue)
		if err != nil {
			return nil, err
		}

		properties[propertyKey] = property
	}

	return properties, nil
}

// parseObjectProperty parses one explicitly declared property schema.
func (dc *Context) parseObjectProperty(propertyKey string, propertyValue json.RawMessage) (Property, error) {
	propertyJSONKV := make(JSONKV)
	if err := json.Unmarshal(propertyValue, &propertyJSONKV); err != nil {
		return Property{}, err
	}

	if propertyJSONKV == nil {
		return Property{}, fmt.Errorf("property %q schema must be an object", propertyKey)
	}

	if _, ok := propertyJSONKV["readOnly"]; ok {
		return Property{}, errors.New("readOnly is not allowed in object properties")
	}

	if _, ok := propertyJSONKV["writeOnly"]; ok {
		return Property{}, errors.New("writeOnly is not allowed in object properties")
	}

	propertyDomain, err := dc.Parse(&propertyValue)
	if err != nil {
		return Property{}, err
	}

	if propertyDomain == nil {
		return Property{}, fmt.Errorf("property %q parsed domain cannot be nil", propertyKey)
	}

	return Property{Key: propertyKey, Domain: propertyDomain}, nil
}

// parseObjectAdditionalProperties parses the additionalProperties policy.
func (dc *Context) parseObjectAdditionalProperties(
	jsonKV JSONKV,
	raw *json.RawMessage,
	objectDomain *ObjectDomain,
) error {
	if _, ok := jsonKV["additionalProperties"]; !ok {
		return nil
	}

	delete(jsonKV, "additionalProperties")

	if raw == nil {
		return errors.New("additionalProperties cannot be null")
	}

	trimmed := bytes.TrimSpace(*raw)
	if len(trimmed) == 0 {
		return errors.New("additionalProperties cannot be empty")
	}

	switch trimmed[0] {
	case '{':
		return dc.parseObjectAdditionalPropertySchema(raw, trimmed, objectDomain)
	case 't', 'f':
		return parseObjectAdditionalPropertyBool(trimmed, objectDomain)
	default:
		return errors.New("additionalProperties must be boolean or schema object")
	}
}

// parseObjectAdditionalPropertySchema parses an object-shaped additionalProperties value.
func (dc *Context) parseObjectAdditionalPropertySchema(
	raw *json.RawMessage,
	trimmed json.RawMessage,
	objectDomain *ObjectDomain,
) error {
	var additionalPropertiesKV JSONKV
	if err := json.Unmarshal(trimmed, &additionalPropertiesKV); err != nil {
		return err
	}

	if len(additionalPropertiesKV) == 0 {
		objectDomain.AdditionalPropertyKind = AdditionalTrue

		return nil
	}

	additionalPropertyDomain, err := dc.Parse(raw)
	if err != nil {
		return err
	}

	if additionalPropertyDomain == nil {
		return errors.New("parsed additionalProperties domain cannot be nil")
	}

	objectDomain.AdditionalPropertyKind = AdditionalSchema
	objectDomain.AdditionalPropertyDomain = additionalPropertyDomain

	return nil
}

// parseObjectAdditionalPropertyBool parses a boolean additionalProperties value.
func parseObjectAdditionalPropertyBool(trimmed json.RawMessage, objectDomain *ObjectDomain) error {
	var boolValue bool
	if err := json.Unmarshal(trimmed, &boolValue); err != nil {
		return err
	}

	if boolValue {
		objectDomain.AdditionalPropertyKind = AdditionalTrue
	} else {
		objectDomain.AdditionalPropertyKind = AdditionalFalse
	}

	return nil
}

// applyRequiredObjectProperties marks declared and undeclared required names.
func applyRequiredObjectProperties(
	jsonKV JSONKV,
	required []string,
	properties map[string]Property,
	objectDomain *ObjectDomain,
) error {
	if _, ok := jsonKV["required"]; !ok {
		return nil
	}

	delete(jsonKV, "required")

	if len(required) == 0 {
		return errors.New("required cannot be empty")
	}

	requiredKeys := make(map[string]struct{}, len(required))
	for _, requiredKey := range required {
		if _, ok := requiredKeys[requiredKey]; ok {
			return fmt.Errorf("required property %q listed more than once", requiredKey)
		}

		requiredKeys[requiredKey] = struct{}{}

		property, ok := properties[requiredKey]
		if !ok {
			undeclaredProperty, err := undeclaredRequiredProperty(requiredKey, objectDomain)
			if err != nil {
				return err
			}

			property = undeclaredProperty
		}

		property.Required = true
		properties[requiredKey] = property
	}

	return nil
}

// undeclaredRequiredProperty applies additionalProperties to a required name.
func undeclaredRequiredProperty(key string, objectDomain *ObjectDomain) (Property, error) {
	switch objectDomain.AdditionalPropertyKind {
	case AdditionalTrue:
		return Property{Key: key}, nil
	case AdditionalFalse:
		return Property{}, fmt.Errorf(
			"required property %q is forbidden by additionalProperties false",
			key,
		)
	case AdditionalSchema:
		if objectDomain.AdditionalPropertyDomain == nil {
			return Property{}, errors.New("additional property schema domain cannot be nil")
		}

		return Property{Key: key, Domain: objectDomain.AdditionalPropertyDomain}, nil
	default:
		return Property{}, errors.New("unknown additionalProperties kind")
	}
}

// storeSortedObjectProperties stores property domains in deterministic order.
func (dc *Context) storeSortedObjectProperties(properties map[string]Property) []Property {
	propertyKeys := make([]string, 0, len(properties))
	for propertyKey := range properties {
		propertyKeys = append(propertyKeys, propertyKey)
	}

	sort.Strings(propertyKeys)

	if len(propertyKeys) == 0 {
		return nil
	}

	sorted := make([]Property, 0, len(propertyKeys))
	for _, propertyKey := range propertyKeys {
		property := properties[propertyKey]
		dc.AddDomain(&property)
		sorted = append(sorted, property)
	}

	return sorted
}

// parseObjectBounds parses and validates minProperties and maxProperties.
func parseObjectBounds(jsonKV JSONKV, jsonObject JSONObject, objectDomain *ObjectDomain) error {
	if _, ok := jsonKV["minProperties"]; ok {
		delete(jsonKV, "minProperties")

		if jsonObject.MinProperties == nil {
			return errors.New("minProperties cannot be null")
		}

		if *jsonObject.MinProperties < 0 {
			return errors.New("minProperties cannot be negative")
		}

		objectDomain.MinProps = *jsonObject.MinProperties
	}

	if _, ok := jsonKV["maxProperties"]; ok {
		delete(jsonKV, "maxProperties")

		if jsonObject.MaxProperties == nil {
			return errors.New("maxProperties cannot be null")
		}

		if *jsonObject.MaxProperties < 0 {
			return errors.New("maxProperties cannot be negative")
		}

		objectDomain.MaxProps = jsonObject.MaxProperties
	}

	return nil
}

// finalizeObjectDomain canonicalizes finite enums and rejects empty object domains.
func finalizeObjectDomain(objectDomain *ObjectDomain) error {
	if objectDomain == nil {
		return errors.New("object domain cannot be nil")
	}

	if err := validateObjectPropertyBounds(objectDomain); err != nil {
		return err
	}

	if err := validateAdditionalPropertySchema(objectDomain); err != nil {
		return err
	}

	if err := canonicalizeObjectProperties(objectDomain); err != nil {
		return err
	}

	enums, err := filterEnumsByType(objectDomain.Enum, "object", objectDomain.Nullable)
	if err != nil {
		return err
	}

	objectDomain.Enum = enums
	if err := filterObjectEnumsByConstraints(objectDomain); err != nil {
		return err
	}

	return validateObjectSatisfiability(objectDomain)
}

// canonicalizeObjectProperties sorts named properties and rejects duplicates.
func canonicalizeObjectProperties(objectDomain *ObjectDomain) error {
	properties := append([]Property(nil), objectDomain.Properties...)
	sort.Slice(properties, func(first int, second int) bool {
		return properties[first].Key < properties[second].Key
	})

	for index := 1; index < len(properties); index++ {
		if properties[index-1].Key == properties[index].Key {
			return fmt.Errorf("property %q appears more than once", properties[index].Key)
		}
	}

	for index := range properties {
		if _, err := properties[index].GenerateHash(); err != nil {
			return fmt.Errorf("property %q: %w", properties[index].Key, err)
		}
	}

	objectDomain.Properties = properties

	return nil
}

// validateObjectPropertyBounds rejects negative programmatic property counts.
func validateObjectPropertyBounds(objectDomain *ObjectDomain) error {
	if objectDomain.MinProps < 0 {
		return errors.New("minProperties cannot be negative")
	}

	if objectDomain.MaxProps != nil && *objectDomain.MaxProps < 0 {
		return errors.New("maxProperties cannot be negative")
	}

	return nil
}

// filterObjectEnumsByConstraints retains structurally valid object enum values.
func filterObjectEnumsByConstraints(objectDomain *ObjectDomain) error {
	if objectDomain.Enum == nil {
		return nil
	}

	filtered := make([]types.Enum, 0, len(objectDomain.Enum))
	for _, enumValue := range objectDomain.Enum {
		if string(enumValue) == "null" {
			filtered = append(filtered, enumValue)

			continue
		}

		allowed, err := objectEnumAllowed(objectDomain, enumValue)
		if err != nil {
			return err
		}

		if allowed {
			filtered = append(filtered, enumValue)
		}
	}

	if len(filtered) == 0 {
		return errors.New("enum has no values compatible with object constraints")
	}

	objectDomain.Enum = filtered

	return nil
}

// objectEnumAllowed reports whether an object enum satisfies structural constraints.
func objectEnumAllowed(objectDomain *ObjectDomain, enumValue types.Enum) (bool, error) {
	properties := make(map[string]json.RawMessage)
	if err := json.Unmarshal(enumValue, &properties); err != nil {
		return false, err
	}

	propertyCount := len(properties)
	if propertyCount < objectDomain.MinProps {
		return false, nil
	}

	if objectDomain.MaxProps != nil && propertyCount > *objectDomain.MaxProps {
		return false, nil
	}

	if !objectEnumHasRequiredProperties(objectDomain.Properties, properties) {
		return false, nil
	}

	return objectEnumPropertyValuesAllowed(objectDomain, properties)
}

// objectEnumHasRequiredProperties checks every required property name.
func objectEnumHasRequiredProperties(properties []Property, enumProperties map[string]json.RawMessage) bool {
	for _, property := range properties {
		if !property.Required {
			continue
		}

		if _, ok := enumProperties[property.Key]; !ok {
			return false
		}
	}

	return true
}

// objectEnumPropertyValuesAllowed validates declared and additional property values.
func objectEnumPropertyValuesAllowed(
	objectDomain *ObjectDomain,
	enumProperties map[string]json.RawMessage,
) (bool, error) {
	declared := propertiesByKey(objectDomain.Properties)

	for _, propertyName := range sortedJSONKeys(JSONKV(enumProperties)) {
		propertyValue := enumProperties[propertyName]

		property, ok := declared[propertyName]
		if ok {
			allowed, err := declaredPropertyValueAllowed(property, propertyValue)
			if err != nil || !allowed {
				return allowed, err
			}

			continue
		}

		allowed, err := additionalPropertyValueAllowed(objectDomain, propertyValue)
		if err != nil || !allowed {
			return allowed, err
		}
	}

	return true, nil
}

// declaredPropertyValueAllowed validates a present declared property.
func declaredPropertyValueAllowed(property Property, value json.RawMessage) (bool, error) {
	if property.Domain == nil {
		return true, nil
	}

	allowed, err := domainAllowsJSONValue(property.Domain, value)
	if err != nil {
		return false, fmt.Errorf("property %q: %w", property.Key, err)
	}

	return allowed, nil
}

// additionalPropertyValueAllowed applies the additionalProperties value policy.
func additionalPropertyValueAllowed(objectDomain *ObjectDomain, value json.RawMessage) (bool, error) {
	switch objectDomain.AdditionalPropertyKind {
	case AdditionalTrue:
		return true, nil
	case AdditionalFalse:
		return false, nil
	case AdditionalSchema:
		allowed, err := domainAllowsJSONValue(objectDomain.AdditionalPropertyDomain, value)
		if err != nil {
			return false, fmt.Errorf("additional property: %w", err)
		}

		return allowed, nil
	default:
		return false, errors.New("unknown additionalProperties kind")
	}
}

// validateObjectSatisfiability rejects object constraints with no valid value.
func validateObjectSatisfiability(objectDomain *ObjectDomain) error {
	allowsNull, err := enumAllowsNull(objectDomain.Nullable, objectDomain.Enum)
	if err != nil {
		return err
	}

	if allowsNull {
		return nil
	}

	return validateObjectPropertyCounts(objectDomain)
}

// validateObjectPropertyCounts checks whether at least one object can satisfy the property counts.
func validateObjectPropertyCounts(objectDomain *ObjectDomain) error {
	if objectDomain.MaxProps != nil && objectDomain.MinProps > *objectDomain.MaxProps {
		return errors.New("minProperties cannot be greater than maxProperties")
	}

	requiredCount := 0

	for _, property := range objectDomain.Properties {
		if property.Required {
			requiredCount++
		}
	}

	if objectDomain.MaxProps != nil && requiredCount > *objectDomain.MaxProps {
		return errors.New("required property count cannot be greater than maxProperties")
	}

	if objectDomain.AdditionalPropertyKind == AdditionalFalse && objectDomain.MinProps > len(objectDomain.Properties) {
		return errors.New("minProperties exceeds the number of allowed properties")
	}

	return nil
}

// rejectUnsupportedObjectKeys rejects fields outside the supported object subset.
func rejectUnsupportedObjectKeys(jsonKV JSONKV) error {
	if len(jsonKV) == 0 {
		return nil
	}

	keys := make([]string, 0, len(jsonKV))
	for key := range jsonKV {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return fmt.Errorf("unsupported object schema keys: %v", keys)
}
