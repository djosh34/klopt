package hashables

import (
	"crypto/sha256"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
)

type Number []byte

type NumberHashable struct {
	Nullable bool     `json:"nullable"`
	Enum     []Number `json:"enum"`

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

	jsonBytes, err := json.Marshal(numberHashableHashJSON{Type: "number", Value: *n})
	if err != nil {
		return types.Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}

type IntegerHashable NumberHashable

type integerHashableHashJSON struct {
	Type  string          `json:"type"`
	Value IntegerHashable `json:"value"`
}

var _ types.Hasher = new(IntegerHashable)

func (i *IntegerHashable) GenerateHash() (types.Hash, error) {
	if i == nil {
		return types.Hash{}, errors.New("integer hashable cannot be nil")
	}

	jsonBytes, err := json.Marshal(integerHashableHashJSON{Type: "integer", Value: *i})
	if err != nil {
		return types.Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}
