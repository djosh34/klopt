package domain

import (
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
)

// nil == null
type EnumDomain struct {
	*json.RawMessage
}

func (e *EnumDomain) ToHasher() (types.Hasher, error) {
	if e == nil {
		return nil, errors.New("domain of enum cannot be nil")
	}

	return &hashables.EnumHashable{RawMessage: e.RawMessage}, nil
}

func NewEnumFromJSON(node *json.RawMessage) EnumDomain {
	return EnumDomain{node}
}
