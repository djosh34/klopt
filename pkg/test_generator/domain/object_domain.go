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

	Enum []types.Domain

	Properties []types.Domain

	AdditionalPropertyKind
	AdditionalPropertyDomain types.Domain

	MinProps int
	MaxProps *int
}

func (o *ObjectDomain) ToHasher() (types.Hasher, error) {
	if o == nil {
		return nil, errors.New("object domain cannot be nil")
	}

	enumHashers := make([]types.Hasher, 0, len(o.Enum))
	for _, enumDomain := range o.Enum {
		hasher, err := enumDomain.ToHasher()
		if err != nil {
			return nil, err
		}
		enumHashers = append(enumHashers, hasher)
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
		Enum:                     enumHashers,
		Properties:               propertyHashers,
		AdditionalPropertyKind:   hashables.AdditionalPropertyKind(o.AdditionalPropertyKind),
		AdditionalPropertyDomain: additionalPropertyHasher,
		MinProps:                 o.MinProps,
		MaxProps:                 o.MaxProps,
	}, nil
}

type JSONKV map[string]json.RawMessage
type JSONObject struct {
	Type                 string            `json:"type"`
	Nullable             bool              `json:"nullable"`
	Required             []string          `json:"required"`
	Properties           JSONKV            `json:"properties"`
	AdditionalProperties *json.RawMessage  `json:"additionalProperties"`
	MinProperties        *int              `json:"minProperties"`
	MaxProperties        *int              `json:"maxProperties"`
	Enum                 []json.RawMessage `json:"enum"`
}

type PropertyAlreadyExistsError struct {
	Key string
}

func (p *PropertyAlreadyExistsError) Error() string {
	return fmt.Sprintf("property %q already exists in object", p.Key)
}

func (dc *DomainContext) ParseObject(node *json.RawMessage) (ObjectDomain, error) {
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

	objectDomain := ObjectDomain{}

	// Parse Enums early, and if it exists, return early (we will not check that enum is valid, and only populate enum field of ObjectDomain)
	if _, enumOk := jsonKV["enum"]; enumOk {
		for _, enumValue := range jsonObject.Enum {
			enumDomain, enumErr := NewEnumFromJSON(&enumValue)
			if enumErr != nil {
				return ObjectDomain{}, enumErr
			}

			domainErr := dc.AddDomain(&enumDomain)
			if domainErr != nil {
				return ObjectDomain{}, domainErr
			}

			objectDomain.Enum = append(objectDomain.Enum, &enumDomain)
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
		domainErr := dc.AddDomain(&property)
		if domainErr != nil {
			return ObjectDomain{}, domainErr
		}

		objectDomain.Properties = append(objectDomain.Properties, &property)
	}

	// Parse AdditionalProperties
	if _, additionalPropertiesOk := jsonKV["additionalProperties"]; additionalPropertiesOk {
		delete(jsonKV, "additionalProperties")

		additionalProperties := jsonObject.AdditionalProperties
		if additionalProperties == nil {
			return ObjectDomain{}, errors.New("additionalProperties cannot be null")
		}

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

	delete(jsonKV, "type")
	delete(jsonKV, "nullable")
	delete(jsonKV, "title")
	delete(jsonKV, "description")

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
