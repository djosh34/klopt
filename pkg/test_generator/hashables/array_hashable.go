package hashables

import (
	"crypto/sha256"
	"encoding/json"
	"errors"

	"decode_and_validate_generator/pkg/test_generator/types"
)

type ArrayHashable struct {
	Nullable bool `json:"nullable"`

	Enum []types.Enum `json:"enum"`

	Items types.Hasher `json:"items"`

	MinItems int  `json:"minItems"`
	MaxItems *int `json:"maxItems"`
}

type arrayHashableHashJSON struct {
	Type  string        `json:"type"`
	Value ArrayHashable `json:"value"`
}

var _ types.Hasher = new(ArrayHashable)

func (a *ArrayHashable) GenerateHash() (types.Hash, error) {
	if a == nil {
		return types.Hash{}, errors.New("array hashable cannot be nil")
	}

	jsonBytes, err := json.Marshal(arrayHashableHashJSON{Type: "array", Value: *a})
	if err != nil {
		return types.Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}
