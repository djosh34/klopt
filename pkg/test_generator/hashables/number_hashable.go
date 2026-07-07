package hashables

import (
	"crypto/sha256"
	"encoding/json"
	"errors"

	"decode_and_validate_generator/pkg/test_generator/types"
)

type Number []byte

type NumberHashable struct {
	Type     string       `json:"type"`
	Nullable bool         `json:"nullable"`
	Enum     []types.Enum `json:"enum"`

	Minimum          *Number `json:"minimum"`
	Maximum          *Number `json:"maximum"`
	ExclusiveMinimum bool    `json:"exclusiveMinimum"`
	ExclusiveMaximum bool    `json:"exclusiveMaximum"`
	MultipleOf       *Number `json:"multipleOf"`
	Format           *string `json:"format"`
}

type numberHashableHashJSON struct {
	Type  string         `json:"type"`
	Value NumberHashable `json:"value"`
}

var _ types.Hasher = new(NumberHashable)

func (n *NumberHashable) GenerateHash() (types.Hash, error) {
	if n == nil {
		return types.Hash{}, errors.New("number hashable cannot be nil")
	}

	hashType := n.Type
	if hashType == "" {
		hashType = "number"
	}

	jsonBytes, err := json.Marshal(numberHashableHashJSON{Type: hashType, Value: *n})
	if err != nil {
		return types.Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}
