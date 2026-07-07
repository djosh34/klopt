package testgenerator

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
)

type StringDomain struct {
	Nullable bool `json:"nullable"`

	Enum []string `json:"enum"`

	Pattern *string `json:"pattern"`
	Format  *string `json:"format"`

	MinLength int  `json:"minLength"`
	MaxLength *int `json:"maxLength"`
}

type stringDomainHashJson struct {
	Type  string       `json:"type"`
	Value StringDomain `json:"value"`
}

var _ Hasher = new(StringDomain)

func (domain *StringDomain) GenerateHash() (Hash, error) {
	if domain == nil {
		return Hash{}, errors.New("domain of string cannot be nil")
	}

	sdJson := stringDomainHashJson{
		Type:  "string",
		Value: *domain,
	}

	jsonBytes, err := json.Marshal(&sdJson)
	if err != nil {
		return Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}
