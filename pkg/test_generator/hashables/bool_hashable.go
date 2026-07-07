package hashables

import (
	"crypto/sha256"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
)

type BoolHashable struct {
	Nullable bool   `json:"nullable"`
	Enum     []bool `json:"enum"`
}

type boolHashableHashJSON struct {
	Type  string       `json:"type"`
	Value BoolHashable `json:"value"`
}

var _ types.Hasher = new(BoolHashable)

func (b *BoolHashable) GenerateHash() (types.Hash, error) {
	if b == nil {
		return types.Hash{}, errors.New("bool hashable cannot be nil")
	}

	jsonBytes, err := json.Marshal(boolHashableHashJSON{Type: "bool", Value: *b})
	if err != nil {
		return types.Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}
