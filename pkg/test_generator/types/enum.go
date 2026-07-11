package types

import (
	"crypto/sha256"
	"encoding/json"
	"errors"

	//nolint:depguard // Exact JSON semantics are shared within test_generator.
	"decode_and_validate_generator/pkg/test_generator/internal/jsonvalue"
)

// Enum is a canonical JSON value used by an enum constraint.
type Enum json.RawMessage

// enumHashJSON separates enum hashes from hashes of other domain values.
type enumHashJSON struct {
	Type  string `json:"type"`
	Value Enum   `json:"value"`
}

var _ Hasher = Enum{}

// CanonicalEnum returns a deterministic representation of a JSON enum value.
func CanonicalEnum(raw json.RawMessage) (Enum, error) {
	if raw == nil {
		return nil, errors.New("enum raw value cannot be nil")
	}

	value, err := jsonvalue.Parse(raw)
	if err != nil {
		return nil, err
	}

	canonical, err := value.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return Enum(canonical), nil
}

// GenerateHash returns a hash of the enum's semantic JSON value.
func (e Enum) GenerateHash() (Hash, error) {
	jsonBytes, err := json.Marshal(enumHashJSON{Type: "enum", Value: e})
	if err != nil {
		return Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}

// MarshalJSON returns the enum as canonical JSON.
func (e Enum) MarshalJSON() ([]byte, error) {
	if e == nil {
		return nil, errors.New("enum raw value cannot be nil")
	}

	canonical, err := CanonicalEnum(json.RawMessage(e))
	if err != nil {
		return nil, err
	}

	return canonical, nil
}
