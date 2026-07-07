package hashables

import (
	"crypto/sha256"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
)

type PropertyHashable struct {
	Key string
	types.Hasher
	Required bool
}

type propertyHashableHashJSON struct {
	Type  string           `json:"type"`
	Value PropertyHashable `json:"value"`
}

var _ types.Hasher = new(PropertyHashable)

func (p *PropertyHashable) GenerateHash() (types.Hash, error) {
	if p == nil {
		return types.Hash{}, errors.New("property hashable cannot be nil")
	}

	jsonBytes, err := json.Marshal(propertyHashableHashJSON{Type: "property", Value: *p})
	if err != nil {
		return types.Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}

type AdditionalPropertyKind int

const (
	AdditionalTrue AdditionalPropertyKind = iota
	AdditionalFalse
	AdditionalSchema
)

type ObjectHashable struct {
	Nullable bool

	Enum []types.Enum

	Properties []PropertyHashable

	AdditionalPropertyKind
	AdditionalPropertyDomain types.Hasher

	MinProps int
	MaxProps *int
}

type objectHashableHashJSON struct {
	Type  string         `json:"type"`
	Value ObjectHashable `json:"value"`
}

var _ types.Hasher = new(ObjectHashable)

func (o *ObjectHashable) GenerateHash() (types.Hash, error) {
	if o == nil {
		return types.Hash{}, errors.New("object hashable cannot be nil")
	}

	jsonBytes, err := json.Marshal(objectHashableHashJSON{Type: "object", Value: *o})
	if err != nil {
		return types.Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}
