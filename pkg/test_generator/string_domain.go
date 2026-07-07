package testgenerator

import (
	"crypto/sha256"
	"encoding/json"
)

type StringDomain struct {
	Nullable bool `json:"nullable"`

	Enum []string `json:"enum"`

	Pattern *string `json:"pattern"`
	Format  *string `json:"format"`

	MinLength int  `json:"minLength"`
	MaxLength *int `json:"maxLength"`
}

var _ Hasher = new(StringDomain)

func (domain *StringDomain) Hash() (Hash, error) {
	jsonBytes, err := json.Marshal(domain)
	if err != nil {
		return Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}
