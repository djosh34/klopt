package names

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRequestValidationRejectsMalformedOperationID verifies the public sentinel.
func TestRequestValidationRejectsMalformedOperationID(t *testing.T) {
	t.Parallel()

	name, err := RequestValidation("not valid")
	require.Empty(t, name)
	require.ErrorIs(t, err, ErrInvalidOperationID)
	require.True(t, errors.Is(err, ErrInvalidOperationID))
}

// TestRequestValidationUsesInjectiveOperationIDMapping covers every character conversion.
func TestRequestValidationUsesInjectiveOperationIDMapping(t *testing.T) {
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

			actual, err := RequestValidation(operationID)
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		})
	}
}

// TestRequestValidationRejectsEveryOutOfGrammarShape covers grammar boundaries.
func TestRequestValidationRejectsEveryOutOfGrammarShape(t *testing.T) {
	t.Parallel()

	for _, operationID := range []string{"", "1get", "_get", "get.pet", "get-", "get--pet", "get//pet", "gét"} {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()

			actual, err := RequestValidation(operationID)
			require.Empty(t, actual)
			require.ErrorIs(t, err, ErrInvalidOperationID)
		})
	}
}

// TestRequestValidationPrefixesEveryCompilationConflict locks the exact conflict set.
func TestRequestValidationPrefixesEveryCompilationConflict(t *testing.T) {
	t.Parallel()

	conflicts := []string{
		"break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough", "for",
		"func", "go", "goto", "if", "import", "interface", "map", "package", "range", "return", "select",
		"struct", "switch", "type", "var", "init", "RequestValidations", "mustQueryDecoder", "mustPathDecoder",
		"openAPI", "TestValidations", "json", "jsonvalue", "patternvalidator", "validation", "string", "error",
		"byte", "int", "nil", "true", "panic",
	}
	require.Len(t, conflicts, 42)

	for _, operationID := range conflicts {
		actual, err := RequestValidation(operationID)
		require.NoError(t, err)
		require.Equal(t, "_x"+operationID, actual)
	}
}
