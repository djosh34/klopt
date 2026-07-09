package domain

import (
	"crypto/sha256"
	"encoding/json"

	//nolint:depguard // Domain contracts are shared within test_generator.
	"decode_and_validate_generator/pkg/test_generator/types"
)

// domainHashJSON separates hashes belonging to different domain types.
type domainHashJSON struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// generateHash returns a deterministic hash of a typed domain value.
func generateHash(hashType string, value any) (types.Hash, error) {
	jsonBytes, err := json.Marshal(domainHashJSON{Type: hashType, Value: value})
	if err != nil {
		return types.Hash{}, err
	}

	return sha256.Sum256(jsonBytes), nil
}
