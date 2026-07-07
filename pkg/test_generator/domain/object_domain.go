package domain

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

var _ Hasher = new(Property)
var _ Hasher = new(ObjectDomain)

type AdditionalPropertyKind int

const (
	AdditionalTrue AdditionalPropertyKind = iota
	AdditionalFalse
	AdditionalSchema
)

type Property struct {
	Key string
	*Hash
	Required bool
}

type propertyHashJSON struct {
	Type  string   `json:"type"`
	Value Property `json:"value"`
}

func (p *Property) GenerateHash() (Hash, error) {
	if p == nil {
		return Hash{}, errors.New("property cannot be nil")
	}

	jsonBytes, err := json.Marshal(propertyHashJSON{Type: "property", Value: *p})
	if err != nil {
		return Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}

type ObjectDomain struct {
	Enum []*Hash

	Properties []*Hash

	AdditionalPropertyKind
	AdditionalPropertyDomain *Hash

	MinProps int
	MaxProps *int
}

type objectDomainHashJSON struct {
	Type  string       `json:"type"`
	Value ObjectDomain `json:"value"`
}

func (o *ObjectDomain) GenerateHash() (Hash, error) {
	if o == nil {
		return Hash{}, errors.New("object domain cannot be nil")
	}

	jsonBytes, err := json.Marshal(objectDomainHashJSON{Type: "object", Value: *o})
	if err != nil {
		return Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
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

			enumHash, enumHashErr := enumDomain.GenerateHash()
			if enumHashErr != nil {
				return ObjectDomain{}, enumHashErr
			}

			domainErr := dc.AddDomain(&enumDomain)
			if domainErr != nil {
				return ObjectDomain{}, domainErr
			}

			objectDomain.Enum = append(objectDomain.Enum, &enumHash)
		}

		return objectDomain, nil
	}

	properties := make(map[string]Property, len(jsonObject.Properties))

	// Parse Properties
	if _, propertiesOk := jsonKV["properties"]; propertiesOk {
		delete(jsonKV, "properties")

		for propertyKey, propertyValue := range jsonObject.Properties {
			propertyJSONKV := make(JSONKV)
			propertyJSONKVErr := json.Unmarshal(propertyValue, &propertyJSONKV)
			if propertyJSONKVErr != nil {
				return ObjectDomain{}, propertyJSONKVErr
			}
			if _, readOnlyOk := propertyJSONKV["readOnly"]; readOnlyOk {
				return ObjectDomain{}, errors.New("readOnly is not allowed in object properties")
			}
			if _, writeOnlyOk := propertyJSONKV["writeOnly"]; writeOnlyOk {
				return ObjectDomain{}, errors.New("writeOnly is not allowed in object properties")
			}

			if _, propertyOk := properties[propertyKey]; propertyOk {
				return objectDomain, &PropertyAlreadyExistsError{
					Key: propertyKey,
				}
			}

			propertyHash, propertyErr := dc.ParseToHash(&propertyValue)
			if propertyErr != nil {
				return ObjectDomain{}, propertyErr
			}

			property := Property{
				Key:  propertyKey,
				Hash: &propertyHash,
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

		for _, requiredKey := range jsonObject.Required {
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
		propertyHash, propertyHashErr := property.GenerateHash()
		if propertyHashErr != nil {
			return ObjectDomain{}, propertyHashErr
		}

		domainErr := dc.AddDomain(&property)
		if domainErr != nil {
			return ObjectDomain{}, domainErr
		}

		objectDomain.Properties = append(objectDomain.Properties, &propertyHash)
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
			additionalPropertyHash, additionalPropertyErr := dc.ParseToHash(additionalProperties)
			if additionalPropertyErr != nil {
				return ObjectDomain{}, additionalPropertyErr
			}

			objectDomain.AdditionalPropertyKind = AdditionalSchema
			objectDomain.AdditionalPropertyDomain = &additionalPropertyHash
		}
	}

	// Parse MinProps, MaxProps
	if _, minPropertiesOk := jsonKV["minProperties"]; minPropertiesOk {
		delete(jsonKV, "minProperties")
		if jsonObject.MinProperties != nil {
			objectDomain.MinProps = *jsonObject.MinProperties
		}
	}
	if _, maxPropertiesOk := jsonKV["maxProperties"]; maxPropertiesOk {
		delete(jsonKV, "maxProperties")
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
