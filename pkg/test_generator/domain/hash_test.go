package domain

import (
	"crypto/sha256"
	"encoding/json"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Tests assert the shared hash contract.

	"github.com/stretchr/testify/require"
)

// requireGeneratedHash computes the expected hash independently from the production helper.
func requireGeneratedHash(t *testing.T, hashType string, value any) types.Hash {
	t.Helper()

	expectedHashInput := struct {
		Type  string `json:"type"`
		Value any    `json:"value"`
	}{
		Type:  hashType,
		Value: value,
	}

	jsonBytes, err := json.Marshal(expectedHashInput)
	require.NoError(t, err)

	return sha256.Sum256(jsonBytes)
}
