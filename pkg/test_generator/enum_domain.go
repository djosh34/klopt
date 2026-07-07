package testgenerator

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
)

var _ Hasher = new(EnumDomain)

// nil == null
type EnumDomain struct {
	*json.RawMessage
}

type enumDomainHashJson struct {
	Type  string     `json:"type"`
	Value EnumDomain `json:"value"`
}

func (e *EnumDomain) GenerateHash() (Hash, error) {
	if e == nil {
		return Hash{}, errors.New("domain of enum cannot be nil")
	}

	edJson := enumDomainHashJson{
		Type:  "enum",
		Value: *e,
	}

	jsonBytes, err := json.Marshal(&edJson)
	if err != nil {
		return Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}

func NewEnumFromJSON(node *json.RawMessage) (EnumDomain, error) {
	return EnumDomain{node}, nil
}
