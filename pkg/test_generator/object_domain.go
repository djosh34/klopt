package testgenerator

import (
	"encoding/json"
	"fmt"
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

func (p *Property) GenerateHash() (Hash, error) {
	//TODO implement me
	panic("implement me")
}

type ObjectDomain struct {
	Enum []*Hash

	Properties []*Hash

	AdditionalPropertyKind
	AdditionalPropertyDomain *Hash

	MinProps int
	MaxProps *int
}

func (o *ObjectDomain) GenerateHash() (Hash, error) {
	//TODO implement me
	panic("implement me")
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
		if dc.domainStore == nil {
			dc.domainStore = make(map[Hash]Domain)
		}

		for _, enumValue := range jsonObject.Enum {
			enumDomain, enumErr := NewEnumFromJSON(&enumValue)
			if enumErr != nil {
				return ObjectDomain{}, enumErr
			}

			enumHash, enumHashErr := enumDomain.GenerateHash()
			if enumHashErr != nil {
				return ObjectDomain{}, enumHashErr
			}

			dc.domainStore[enumHash] = &enumDomain
			objectDomain.Enum = append(objectDomain.Enum, &enumHash)
		}

		return objectDomain, nil
	}

	properties := make(map[string]Property, len(jsonObject.Properties))

	// Parse Properties
	if _, propertiesOk := jsonKV["properties"]; propertiesOk {
		delete(jsonKV, "properties")

		for propertyKey, propertyValue := range jsonObject.Properties {
			if _, propertyOk := properties[propertyKey]; propertyOk {
				return objectDomain, &PropertyAlreadyExistsError{
					Key: propertyKey,
				}
			}

			propertyHash, propertyErr := dc.Parse(&propertyValue)
			if propertyErr != nil {
				return ObjectDomain{}, propertyErr
			}

			property := Property{
				Key:  propertyKey,
				Hash: propertyHash,
			}

			properties[propertyKey] = property
		}

	}

	// Parse required
	if _, requiredOk := jsonKV["required"]; requiredOk {
		delete(jsonKV, "required")

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

	// Parse AdditionalProperties

	// Parse MinProps, MaxProps

	// Reject if any other keys are left in node?

	return objectDomain, nil
}
