package domain

import (
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

type AdditionalPropertyKind int

const (
	AdditionalTrue AdditionalPropertyKind = iota
	AdditionalFalse
	AdditionalSchema
)

type Property struct {
	Key string
	types.Domain
	Required bool
}

func (p *Property) ToHasher() (types.Hasher, error) {
	if p == nil {
		return nil, errors.New("property cannot be nil")
	}

	var propertyHasher types.Hasher
	if p.Domain != nil {
		hasher, err := p.Domain.ToHasher()
		if err != nil {
			return nil, err
		}
		propertyHasher = hasher
	}

	return &hashables.PropertyHashable{
		Key:      p.Key,
		Hasher:   propertyHasher,
		Required: p.Required,
	}, nil
}

type ObjectDomain struct {
	Nullable bool

	Enum []types.Enum

	Properties []types.Domain

	AdditionalPropertyKind
	AdditionalPropertyDomain types.Domain

	MinProps int
	MaxProps *int
}

func (o *ObjectDomain) AllOfMerge(domain types.Domain) (types.Domain, error) {
	if o == nil {
		return nil, errors.New("object domain cannot be nil")
	}
	if allOfDomain, ok := domain.(*AllOfDomain); ok {
		return allOfDomain.AllOfMerge(o)
	}
	otherObject, ok := domain.(*ObjectDomain)
	if !ok {
		return nil, errors.New("domain is not ObjectDomain")
	}

	merged := *o
	merged.Nullable = o.Nullable || otherObject.Nullable
	if o.Enum != nil || otherObject.Enum != nil {
		merged.Enum = append(append([]types.Enum{}, o.Enum...), otherObject.Enum...)
	}
	merged.MinProps = max(o.MinProps, otherObject.MinProps)
	merged.MaxProps = tighterMaxProps(o.MaxProps, otherObject.MaxProps)

	propertiesByKey := make(map[string]types.Domain, len(o.Properties)+len(otherObject.Properties))
	for _, propertyDomain := range o.Properties {
		property, ok := propertyDomain.(*Property)
		if !ok {
			return nil, errors.New("object property domain is not Property")
		}
		propertiesByKey[property.Key] = propertyDomain
	}
	for _, propertyDomain := range otherObject.Properties {
		property, ok := propertyDomain.(*Property)
		if !ok {
			return nil, errors.New("object property domain is not Property")
		}
		if _, exists := propertiesByKey[property.Key]; exists {
			return nil, &PropertyAlreadyExistsError{Key: property.Key}
		}
		propertiesByKey[property.Key] = propertyDomain
	}

	propertyKeys := make([]string, 0, len(propertiesByKey))
	for propertyKey := range propertiesByKey {
		propertyKeys = append(propertyKeys, propertyKey)
	}
	sort.Strings(propertyKeys)
	merged.Properties = make([]types.Domain, 0, len(propertyKeys))
	for _, propertyKey := range propertyKeys {
		merged.Properties = append(merged.Properties, propertiesByKey[propertyKey])
	}

	additionalKind, additionalDomain, err := mergeAdditionalProperties(o, otherObject)
	if err != nil {
		return nil, err
	}
	merged.AdditionalPropertyKind = additionalKind
	merged.AdditionalPropertyDomain = additionalDomain

	return &merged, nil
}

func tighterMaxProps(first *int, second *int) *int {
	if first == nil {
		return second
	}
	if second == nil || *first <= *second {
		return first
	}
	return second
}

func mergeAdditionalProperties(first *ObjectDomain, second *ObjectDomain) (AdditionalPropertyKind, types.Domain, error) {
	if first.AdditionalPropertyKind == AdditionalFalse || second.AdditionalPropertyKind == AdditionalFalse {
		return AdditionalFalse, nil, nil
	}
	if first.AdditionalPropertyKind == AdditionalSchema && second.AdditionalPropertyKind == AdditionalSchema {
		if first.AdditionalPropertyDomain == second.AdditionalPropertyDomain {
			return AdditionalSchema, first.AdditionalPropertyDomain, nil
		}
		return AdditionalSchema, nil, errors.New("cannot merge different additionalProperties schemas")
	}
	if first.AdditionalPropertyKind == AdditionalSchema {
		return AdditionalSchema, first.AdditionalPropertyDomain, nil
	}
	if second.AdditionalPropertyKind == AdditionalSchema {
		return AdditionalSchema, second.AdditionalPropertyDomain, nil
	}
	return AdditionalTrue, nil, nil
}

func (o *ObjectDomain) ToHasher() (types.Hasher, error) {
	if o == nil {
		return nil, errors.New("object domain cannot be nil")
	}

	propertyHashers := make([]types.Hasher, 0, len(o.Properties))
	for _, propertyDomain := range o.Properties {
		hasher, err := propertyDomain.ToHasher()
		if err != nil {
			return nil, err
		}
		propertyHashers = append(propertyHashers, hasher)
	}

	if o.AdditionalPropertyKind == AdditionalSchema && o.AdditionalPropertyDomain == nil {
		return nil, errors.New("additional property schema domain cannot be nil")
	}

	var additionalPropertyHasher types.Hasher
	if o.AdditionalPropertyDomain != nil {
		hasher, err := o.AdditionalPropertyDomain.ToHasher()
		if err != nil {
			return nil, err
		}
		additionalPropertyHasher = hasher
	}

	return &hashables.ObjectHashable{
		Nullable:                 o.Nullable,
		Enum:                     o.Enum,
		Properties:               propertyHashers,
		AdditionalPropertyKind:   hashables.AdditionalPropertyKind(o.AdditionalPropertyKind),
		AdditionalPropertyDomain: additionalPropertyHasher,
		MinProps:                 o.MinProps,
		MaxProps:                 o.MaxProps,
	}, nil
}

type JSONObject struct {
	Type                 string            `json:"type"`
	Nullable             bool              `json:"nullable"`
	Required             []string          `json:"required"`
	Properties           JSONKV            `json:"properties"`
	AdditionalProperties *json.RawMessage  `json:"additionalProperties"`
	MinProperties        *int              `json:"minProperties"`
	MaxProperties        *int              `json:"maxProperties"`
	EnumRaw              []json.RawMessage `json:"enum"`
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

	if _, typeOk := jsonKV["type"]; typeOk && jsonObject.Type != "object" {
		return ObjectDomain{}, fmt.Errorf("object schema type must be object, got %q", jsonObject.Type)
	}

	objectDomain = ObjectDomain{}

	// Parse Enums early, and if it exists, return early (we will not check that enum is valid, and only populate enum field of ObjectDomain)
	if _, enumOk := jsonKV["enum"]; enumOk {
		for _, enumValue := range jsonObject.EnumRaw {
			enumDomain := types.Enum(enumValue)

			objectDomain.Enum = append(objectDomain.Enum, enumDomain)
		}

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

			if _, propertyOk := properties[propertyKey]; propertyOk {
				return ObjectDomain{}, &PropertyAlreadyExistsError{
					Key: propertyKey,
				}
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

		objectDomain.Properties = append(objectDomain.Properties, &property)
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
