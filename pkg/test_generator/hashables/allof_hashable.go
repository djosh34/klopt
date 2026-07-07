package hashables

import (
	"crypto/sha256"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
)

type AllOfHashable struct {
	Domains      []types.Hasher
	MergedDomain types.Hasher
}

type allOfHashableHashJSON struct {
	Type  string        `json:"type"`
	Value AllOfHashable `json:"value"`
}

var _ types.Hasher = new(AllOfHashable)

func (a *AllOfHashable) GenerateHash() (types.Hash, error) {
	if a == nil {
		return types.Hash{}, errors.New("allOf hashable cannot be nil")
	}

	jsonBytes, err := json.Marshal(allOfHashableHashJSON{Type: "allOf", Value: *a})
	if err != nil {
		return types.Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}
