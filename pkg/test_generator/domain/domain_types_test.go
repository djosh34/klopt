package domain

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewDomainTypesImplementDomain(t *testing.T) {
	require.Implements(t, (*types.Domain)(nil), new(BoolDomain))
	require.Implements(t, (*types.Domain)(nil), new(NumberDomain))
	require.Implements(t, (*types.Domain)(nil), new(IntegerDomain))
	require.Implements(t, (*types.Domain)(nil), new(ArrayDomain))
	require.Implements(t, (*types.Domain)(nil), new(AllOfDomain))
}

func TestBoolDomainMarshalJSONZeroValueIncludesAllFields(t *testing.T) {
	jsonBytes, err := json.Marshal(BoolDomain{})
	require.NoError(t, err)

	require.JSONEq(t, `{"nullable":false,"enum":null}`, string(jsonBytes))
}

func TestNumberDomainMarshalJSONZeroValueIncludesAllFields(t *testing.T) {
	jsonBytes, err := json.Marshal(NumberDomain{})
	require.NoError(t, err)

	require.JSONEq(t, `{"nullable":false,"enum":null,"minimum":null,"maximum":null,"exclusiveMinimum":false,"exclusiveMaximum":false,"multipleOf":null,"format":null}`, string(jsonBytes))
}

func TestIntegerDomainMarshalJSONZeroValueIncludesAllFields(t *testing.T) {
	jsonBytes, err := json.Marshal(IntegerDomain{})
	require.NoError(t, err)

	require.JSONEq(t, `{"nullable":false,"enum":null,"minimum":null,"maximum":null,"exclusiveMinimum":false,"exclusiveMaximum":false,"multipleOf":null,"format":null}`, string(jsonBytes))
}

func TestArrayDomainMarshalJSONZeroValueIncludesAllFields(t *testing.T) {
	jsonBytes, err := json.Marshal(ArrayDomain{})
	require.NoError(t, err)

	require.JSONEq(t, `{"nullable":false,"items":null,"minItems":0,"maxItems":null}`, string(jsonBytes))
}
