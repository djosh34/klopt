package types

import (
	"crypto/sha256"
	"encoding/json"
)

type Enum json.RawMessage

type enumHashJSON struct {
	Type  string `json:"type"`
	Value Enum   `json:"value"`
}

var _ Hasher = Enum{}
var _ ToHasher = Enum{}

func (e Enum) ToHasher() (Hasher, error) {
	return e, nil
}

func (e Enum) GenerateHash() (Hash, error) {
	jsonBytes, err := json.Marshal(enumHashJSON{Type: "enum", Value: e})
	if err != nil {
		return Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}

func (e Enum) MarshalJSON() ([]byte, error) {
	if e == nil {
		return []byte("null"), nil
	}
	return e, nil
}
