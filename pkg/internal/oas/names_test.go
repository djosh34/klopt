package oas

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRequestValidationNameRejectsMalformedOperationID verifies the shared sentinel.
func TestRequestValidationNameRejectsMalformedOperationID(t *testing.T) {
	t.Parallel()

	name, err := RequestValidationName("not valid")
	require.Empty(t, name)
	require.ErrorIs(t, err, ErrInvalidOperationID)
	require.True(t, errors.Is(err, ErrInvalidOperationID))
}

// TestRequestValidationNameUsesInjectiveOperationIDMapping covers every character conversion.
func TestRequestValidationNameUsesInjectiveOperationIDMapping(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"getPet":              "getPet",
		"get_pet":             "get__pet",
		"get-pet":             "get_0pet",
		"pets/get":            "pets_1get",
		"for":                 "_xfor",
		"RequestValidations":  "_xRequestValidations",
		"mustPathDecoder":     "_xmustPathDecoder",
		"ordinaryIdentifier1": "ordinaryIdentifier1",
	}

	for operationID, expected := range tests {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()

			actual, err := RequestValidationName(operationID)
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		})
	}
}

// TestRequestValidationNameRejectsEveryOutOfGrammarShape covers grammar boundaries.
func TestRequestValidationNameRejectsEveryOutOfGrammarShape(t *testing.T) {
	t.Parallel()

	for _, operationID := range []string{"", "1get", "_get", "get.pet", "get-", "get--pet", "get//pet", "gét"} {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()

			actual, err := RequestValidationName(operationID)
			require.Empty(t, actual)
			require.ErrorIs(t, err, ErrInvalidOperationID)
		})
	}
}

// TestRequestValidationNamePrefixesEveryCompilationConflict locks the exact conflict set.
func TestRequestValidationNamePrefixesEveryCompilationConflict(t *testing.T) {
	t.Parallel()

	conflicts := []string{
		"break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough", "for",
		"func", "go", "goto", "if", "import", "interface", "map", "package", "range", "return", "select",
		"struct", "switch", "type", "var", "init", "RequestValidations", "mustQueryDecoder", "mustPathDecoder",
		"openAPI", "FuzzValidations", "validateBody", "json", "jsonvalue", "patternvalidator", "validation",
		"string", "error", "byte", "int", "nil", "true", "panic",
	}
	require.Len(t, conflicts, 43)

	for _, operationID := range conflicts {
		actual, err := RequestValidationName(operationID)
		require.NoError(t, err)
		require.Equal(t, "_x"+operationID, actual)
	}
}
