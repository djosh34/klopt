package hashables

import (
	"crypto/sha256"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
)

type StringHashable struct {
	Nullable bool `json:"nullable"`

	Enum []types.Enum `json:"enum"`

	types.Pattern `json:"pattern"`
	types.Format  `json:"format"`

	XValidExamples   []string `json:"x-valid-examples"`
	XInvalidExamples []string `json:"x-invalid-examples"`

	MinLength int  `json:"minLength"`
	MaxLength *int `json:"maxLength"`
}

type stringHashableHashJSON struct {
	Type  string         `json:"type"`
	Value StringHashable `json:"value"`
}

var _ types.Hasher = new(StringHashable)

func (s *StringHashable) GenerateHash() (types.Hash, error) {
	if s == nil {
		return types.Hash{}, errors.New("hashable of string cannot be nil")
	}

	jsonBytes, err := json.Marshal(stringHashableHashJSON{Type: "string", Value: *s})
	if err != nil {
		return types.Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}
