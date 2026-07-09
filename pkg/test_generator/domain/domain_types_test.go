package domain

import (
	"encoding/json"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Tests assert the shared domain contract.

	"github.com/stretchr/testify/require"
)

// TestNewDomainTypesImplementDomain verifies the compile-time behavior expected by the parser.
func TestNewDomainTypesImplementDomain(t *testing.T) {
	t.Parallel()

	require.Implements(t, (*types.Domain)(nil), new(BoolDomain))
	require.Implements(t, (*types.Domain)(nil), new(NumberDomain))
	require.Implements(t, (*types.Domain)(nil), new(ArrayDomain))
	require.Implements(t, (*types.Domain)(nil), new(AllOfDomain))
}

// TestBoolDomainMarshalJSONZeroValueIncludesAllFields locks down deterministic zero-value hashing input.
func TestBoolDomainMarshalJSONZeroValueIncludesAllFields(t *testing.T) {
	t.Parallel()

	jsonBytes, err := json.Marshal(BoolDomain{})
	require.NoError(t, err)

	require.JSONEq(t, `{"nullable":false,"enum":null}`, string(jsonBytes))
}

// TestNumberDomainMarshalJSONZeroValueIncludesAllFields locks down deterministic zero-value hashing input.
func TestNumberDomainMarshalJSONZeroValueIncludesAllFields(t *testing.T) {
	t.Parallel()

	jsonBytes, err := json.Marshal(NumberDomain{})
	require.NoError(t, err)

	require.JSONEq(t, `{
		"type": "",
		"nullable": false,
		"enum": null,
		"minimum": null,
		"maximum": null,
		"exclusiveMinimum": false,
		"exclusiveMaximum": false,
		"multipleOf": null,
		"format": null
	}`, string(jsonBytes))
}

// TestArrayDomainMarshalJSONZeroValueIncludesAllFields locks down deterministic zero-value hashing input.
func TestArrayDomainMarshalJSONZeroValueIncludesAllFields(t *testing.T) {
	t.Parallel()

	jsonBytes, err := json.Marshal(ArrayDomain{})
	require.NoError(t, err)

	require.JSONEq(t, `{"nullable":false,"enum":null,"items":null,"minItems":0,"maxItems":null}`, string(jsonBytes))
}
