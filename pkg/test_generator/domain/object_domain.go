package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"decode_and_validate_generator/pkg/test_generator/types"
)

type AdditionalPropertyKind int

const (
	AdditionalTrue AdditionalPropertyKind = iota
	AdditionalFalse
	AdditionalSchema
)

type Property struct {
	types.Domain
	Key      string
	Required bool
}

type propertyHashValue struct {
	Key      string
	Hasher   *types.Hash
	Required bool
}

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

type ObjectDomain struct {
	AdditionalPropertyKind

	Nullable bool

	Enum []types.Enum

	Properties []Property

	AdditionalPropertyDomain types.Domain

	MinProps int
	MaxProps *int
}

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
		return nil, errors.New("domain is not ObjectDomain")
	}

	if o.AdditionalPropertyKind == AdditionalSchema && o.AdditionalPropertyDomain == nil {
		return nil, errors.New("additional property schema domain cannot be nil")
	}

	if otherObject.AdditionalPropertyKind == AdditionalSchema && otherObject.AdditionalPropertyDomain == nil {
		return nil, errors.New("additional property schema domain cannot be nil")
	}

	merged := *o
	merged.Nullable = o.Nullable && otherObject.Nullable

	enums, err := mergeEnums(o.Enum, otherObject.Enum)
	if err != nil {
		return nil, err
	}

	merged.Enum = enums
	merged.MinProps = max(o.MinProps, otherObject.MinProps)
	merged.MaxProps = tighterMax(o.MaxProps, otherObject.MaxProps)

	merged.Properties, err = mergeObjectProperties(o, otherObject)
	if err != nil {
		return nil, err
	}

	additionalKind, additionalDomain, err := mergeAdditionalProperties(o, otherObject)
	if err != nil {
		return nil, err
	}

	merged.AdditionalPropertyKind = additionalKind
	merged.AdditionalPropertyDomain = additionalDomain

	return &merged, nil
}

func tighterMax(first *int, second *int) *int {
	if first == nil {
		return second
	}

	if second == nil || *first <= *second {
		return first
	}

	return second
}

func mergeObjectProperties(leftObject *ObjectDomain, rightObject *ObjectDomain) ([]Property, error) {
	leftProperties := propertiesByKey(leftObject.Properties)
	rightProperties := propertiesByKey(rightObject.Properties)
	mergedProperties := make(map[string]Property, len(leftObject.Properties)+len(rightObject.Properties))

	for key, leftProperty := range leftProperties {
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

	for key, rightProperty := range rightProperties {
		if _, exists := leftProperties[key]; exists {
			continue
		}

		if mergeErr := mergeUnmatchedProperty(key, rightProperty, leftObject, mergedProperties); mergeErr != nil {
			return nil, mergeErr
		}
	}

	return sortedProperties(mergedProperties), nil
}

func propertiesByKey(properties []Property) map[string]Property {
	mappedProperties := make(map[string]Property, len(properties))
	for _, property := range properties {
		mappedProperties[property.Key] = property
	}

	return mappedProperties
}

func mergeMatchedProperty(key string, leftProperty Property, rightProperty Property) (Property, error) {
	property := Property{Key: key, Required: leftProperty.Required || rightProperty.Required}
	if leftProperty.Domain != nil && rightProperty.Domain != nil {
		domain, mergeErr := leftProperty.AllOfMerge(rightProperty.Domain)
		if mergeErr != nil {
			return Property{}, mergeErr
		}

		property.Domain = domain
	} else if leftProperty.Domain != nil {
		property.Domain = leftProperty.Domain
	} else {
		property.Domain = rightProperty.Domain
	}

	return property, nil
}

func mergeUnmatchedProperty(key string, property Property, otherObject *ObjectDomain, propertiesByKey map[string]Property) error {
	mergedProperty, keep, mergeErr := mergePropertyWithAdditional(property, otherObject)
	if mergeErr != nil {
		return mergeErr
	}

	if keep {
		propertiesByKey[key] = mergedProperty
	}

	return nil
}

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

func mergePropertyWithAdditional(property Property, otherObject *ObjectDomain) (Property, bool, error) {
	switch otherObject.AdditionalPropertyKind {
	case AdditionalTrue:
		return property, true, nil
	case AdditionalFalse:
		if property.Required {
			return Property{}, false, errors.New("required property is forbidden by additionalProperties false")
		}

		return Property{}, false, nil
	case AdditionalSchema:
		if otherObject.AdditionalPropertyDomain == nil {
			return Property{}, false, errors.New("additional property schema domain cannot be nil")
		}

		if property.Domain == nil {
			property.Domain = otherObject.AdditionalPropertyDomain

			return property, true, nil
		}

		domain, mergeErr := property.AllOfMerge(otherObject.AdditionalPropertyDomain)
		if mergeErr != nil {
			return Property{}, false, mergeErr
		}

		property.Domain = domain

		return property, true, nil
	default:
		return Property{}, false, errors.New("unknown additionalProperties kind")
	}
}

func mergeAdditionalProperties(first *ObjectDomain, second *ObjectDomain) (AdditionalPropertyKind, types.Domain, error) {
	if first.AdditionalPropertyKind == AdditionalFalse || second.AdditionalPropertyKind == AdditionalFalse {
		return AdditionalFalse, nil, nil
	}

	if first.AdditionalPropertyKind == AdditionalSchema && second.AdditionalPropertyKind == AdditionalSchema {
		domain, mergeErr := first.AdditionalPropertyDomain.AllOfMerge(second.AdditionalPropertyDomain)
		if mergeErr != nil {
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

type objectHashValue struct {
	AdditionalPropertyKind

	Nullable bool

	Enum []types.Enum

	Properties []*types.Hash

	AdditionalPropertyDomain *types.Hash

	MinProps int
	MaxProps *int
}

func (o *ObjectDomain) GenerateHash() (types.Hash, error) {
	if o == nil {
		return types.Hash{}, errors.New("object domain cannot be nil")
	}

	propertyHashes := make([]*types.Hash, 0, len(o.Properties))
	for _, property := range o.Properties {
		hash, err := property.GenerateHash()
		if err != nil {
			return types.Hash{}, err
		}

		propertyHashes = append(propertyHashes, &hash)
	}

	if o.AdditionalPropertyKind == AdditionalSchema && o.AdditionalPropertyDomain == nil {
		return types.Hash{}, errors.New("additional property schema domain cannot be nil")
	}

	var additionalPropertyHash *types.Hash

	if o.AdditionalPropertyDomain != nil {
		hash, err := o.AdditionalPropertyDomain.GenerateHash()
		if err != nil {
			return types.Hash{}, err
		}

		additionalPropertyHash = &hash
	}

	return generateHash("object", objectHashValue{
		Nullable:                 o.Nullable,
		Enum:                     o.Enum,
		Properties:               propertyHashes,
		AdditionalPropertyKind:   o.AdditionalPropertyKind,
		AdditionalPropertyDomain: additionalPropertyHash,
		MinProps:                 o.MinProps,
		MaxProps:                 o.MaxProps,
	})
}

type JSONObject struct {
	Type                 string           `json:"type"`
	Nullable             bool             `json:"nullable"`
	Required             []string         `json:"required"`
	Properties           JSONKV           `json:"properties"`
	AdditionalProperties *json.RawMessage `json:"additionalProperties"`
	MinProperties        *int             `json:"minProperties"`
	MaxProperties        *int             `json:"maxProperties"`
}

type PropertyAlreadyExistsError struct {
	Key string
}

func (p *PropertyAlreadyExistsError) Error() string {
	return fmt.Sprintf("property %q already exists in object", p.Key)
}

func (dc *DomainContext) ParseObject(node *json.RawMessage) (objectDomain ObjectDomain, err error) {
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

	jsonKV := make(JSONKV)

	decodeKVErr := json.Unmarshal(*node, &jsonKV)
	if decodeKVErr != nil {
		return ObjectDomain{}, decodeKVErr
	}

	jsonObject := JSONObject{}

	decodeErr := json.Unmarshal(*node, &jsonObject)
	if decodeErr != nil {
		return ObjectDomain{}, decodeErr
	}

	if jsonObject.Type != "object" {
		return ObjectDomain{}, fmt.Errorf("object schema type must be object, got %q", jsonObject.Type)
	}

	objectDomain = ObjectDomain{}

	// Parse Enums early, and if it exists, return early.
	enums, hasEnums, err := parseEnums(jsonKV)
	if err != nil {
		return ObjectDomain{}, err
	}

	if hasEnums {
		objectDomain.Enum = enums

		return objectDomain, nil
	}

	objectDomain.Nullable = jsonObject.Nullable

	properties := make(map[string]Property, len(jsonObject.Properties))

	// Parse Properties
	if _, propertiesOk := jsonKV["properties"]; propertiesOk {
		delete(jsonKV, "properties")

		if jsonObject.Properties == nil {
			return ObjectDomain{}, errors.New("properties must be an object")
		}

		for propertyKey, propertyValue := range jsonObject.Properties {
			propertyJSONKV := make(JSONKV)

			propertyJSONKVErr := json.Unmarshal(propertyValue, &propertyJSONKV)
			if propertyJSONKVErr != nil {
				return ObjectDomain{}, propertyJSONKVErr
			}

			if propertyJSONKV == nil {
				return ObjectDomain{}, fmt.Errorf("property %q schema must be an object", propertyKey)
			}

			if _, readOnlyOk := propertyJSONKV["readOnly"]; readOnlyOk {
				return ObjectDomain{}, errors.New("readOnly is not allowed in object properties")
			}

			if _, writeOnlyOk := propertyJSONKV["writeOnly"]; writeOnlyOk {
				return ObjectDomain{}, errors.New("writeOnly is not allowed in object properties")
			}

			propertyDomain, propertyErr := dc.Parse(&propertyValue)
			if propertyErr != nil {
				return ObjectDomain{}, propertyErr
			}

			property := Property{
				Key:    propertyKey,
				Domain: propertyDomain,
			}

			properties[propertyKey] = property
		}
	}

	// Parse required
	if _, requiredOk := jsonKV["required"]; requiredOk {
		delete(jsonKV, "required")

		if len(jsonObject.Required) == 0 {
			return ObjectDomain{}, errors.New("required cannot be empty")
		}

		requiredKeys := make(map[string]struct{}, len(jsonObject.Required))
		for _, requiredKey := range jsonObject.Required {
			if _, requiredKeyOk := requiredKeys[requiredKey]; requiredKeyOk {
				return ObjectDomain{}, fmt.Errorf("required property %q listed more than once", requiredKey)
			}

			requiredKeys[requiredKey] = struct{}{}

			property, propertyOk := properties[requiredKey]
			if !propertyOk {
				property = Property{
					Key:      requiredKey,
					Required: true,
				}
			} else {
				property.Required = true
			}

			properties[requiredKey] = property
		}
	}

	// Convert properties map to array (sorted by key), and add their hashes to dc
	propertyKeys := make([]string, 0, len(properties))
	for propertyKey := range properties {
		propertyKeys = append(propertyKeys, propertyKey)
	}

	sort.Strings(propertyKeys)

	for _, propertyKey := range propertyKeys {
		property := properties[propertyKey]
		dc.AddDomain(&property)

		objectDomain.Properties = append(objectDomain.Properties, property)
	}

	// Parse AdditionalProperties
	if _, additionalPropertiesOk := jsonKV["additionalProperties"]; additionalPropertiesOk {
		delete(jsonKV, "additionalProperties")

		additionalProperties := jsonObject.AdditionalProperties
		if additionalProperties == nil {
			return ObjectDomain{}, errors.New("additionalProperties cannot be null")
		}

		var additionalPropertiesKV JSONKV
		if additionalPropertiesKVErr := json.Unmarshal(*additionalProperties, &additionalPropertiesKV); additionalPropertiesKVErr == nil && len(additionalPropertiesKV) == 0 {
			objectDomain.AdditionalPropertyKind = AdditionalTrue
		} else {
			var boolValue bool
			if boolErr := json.Unmarshal(*additionalProperties, &boolValue); boolErr == nil {
				if boolValue {
					objectDomain.AdditionalPropertyKind = AdditionalTrue
				} else {
					objectDomain.AdditionalPropertyKind = AdditionalFalse
				}
			} else {
				additionalPropertyDomain, additionalPropertyErr := dc.Parse(additionalProperties)
				if additionalPropertyErr != nil {
					return ObjectDomain{}, additionalPropertyErr
				}

				objectDomain.AdditionalPropertyKind = AdditionalSchema
				objectDomain.AdditionalPropertyDomain = additionalPropertyDomain
			}
		}
	}

	// Parse MinProps, MaxProps
	if _, minPropertiesOk := jsonKV["minProperties"]; minPropertiesOk {
		delete(jsonKV, "minProperties")

		if jsonObject.MinProperties == nil {
			return ObjectDomain{}, errors.New("minProperties cannot be null")
		}

		if *jsonObject.MinProperties < 0 {
			return ObjectDomain{}, errors.New("minProperties cannot be negative")
		}

		objectDomain.MinProps = *jsonObject.MinProperties
	}

	if _, maxPropertiesOk := jsonKV["maxProperties"]; maxPropertiesOk {
		delete(jsonKV, "maxProperties")

		if jsonObject.MaxProperties == nil {
			return ObjectDomain{}, errors.New("maxProperties cannot be null")
		}

		if *jsonObject.MaxProperties < 0 {
			return ObjectDomain{}, errors.New("maxProperties cannot be negative")
		}

		objectDomain.MaxProps = jsonObject.MaxProperties
	}

	if objectDomain.MaxProps != nil && objectDomain.MinProps > *objectDomain.MaxProps {
		return ObjectDomain{}, errors.New("minProperties cannot be greater than maxProperties")
	}

	deleteAllowableKeys(jsonKV)

	// Reject if any other keys are left in node?
	if len(jsonKV) != 0 {
		keys := make([]string, 0, len(jsonKV))
		for key := range jsonKV {
			keys = append(keys, key)
		}

		sort.Strings(keys)

		return ObjectDomain{}, fmt.Errorf("unsupported object schema keys: %v", keys)
	}

	return objectDomain, nil
}
