package testgenerator

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringDomainImplementsInterfaces(t *testing.T) {
	require.Implements(t, (*Hasher)(nil), new(StringDomain))
}

func TestStringDomainMarshalJSONZeroValueIncludesAllFields(t *testing.T) {
	jsonBytes, err := json.Marshal(StringDomain{})
	require.NoError(t, err)

	require.Equal(t, `{"nullable":false,"enum":null,"pattern":null,"format":null,"minLength":0,"maxLength":null}`, string(jsonBytes))
}

func TestStringDomainMarshalJSONAllCombinations(t *testing.T) {
	nullableCases := []struct {
		name  string
		value bool
		want  string
	}{
		{name: "nullable false", value: false, want: "false"},
		{name: "nullable true", value: true, want: "true"},
	}
	enumCases := []struct {
		name  string
		value []string
		want  string
	}{
		{name: "enum nil", value: nil, want: "null"},
		{name: "enum empty", value: []string{}, want: "[]"},
		{name: "enum set", value: []string{"alpha", "beta"}, want: `["alpha","beta"]`},
	}
	patternCases := []struct {
		name  string
		value *string
		want  string
	}{
		{name: "pattern nil", value: nil, want: "null"},
		{name: "pattern set", value: new("^[a-z]+$"), want: `"^[a-z]+$"`},
	}
	formatCases := []struct {
		name  string
		value *string
		want  string
	}{
		{name: "format nil", value: nil, want: "null"},
		{name: "format set", value: new("email"), want: `"email"`},
	}
	minLengthCases := []struct {
		name  string
		value int
		want  string
	}{
		{name: "minLength zero", value: 0, want: "0"},
		{name: "minLength set", value: 3, want: "3"},
	}
	maxLengthCases := []struct {
		name  string
		value *int
		want  string
	}{
		{name: "maxLength nil", value: nil, want: "null"},
		{name: "maxLength set", value: new(9), want: "9"},
	}

	for _, nullableCase := range nullableCases {
		for _, enumCase := range enumCases {
			for _, patternCase := range patternCases {
				for _, formatCase := range formatCases {
					for _, minLengthCase := range minLengthCases {
						for _, maxLengthCase := range maxLengthCases {
							name := fmt.Sprintf(
								"%s/%s/%s/%s/%s/%s",
								nullableCase.name,
								enumCase.name,
								patternCase.name,
								formatCase.name,
								minLengthCase.name,
								maxLengthCase.name,
							)

							t.Run(name, func(t *testing.T) {
								domain := StringDomain{
									Nullable:  nullableCase.value,
									Enum:      enumCase.value,
									Pattern:   patternCase.value,
									Format:    formatCase.value,
									MinLength: minLengthCase.value,
									MaxLength: maxLengthCase.value,
								}

								jsonBytes, err := json.Marshal(domain)
								require.NoError(t, err)

								var fields map[string]json.RawMessage
								err = json.Unmarshal(jsonBytes, &fields)
								require.NoError(t, err)

								require.Len(t, fields, 6)
								require.Contains(t, fields, "nullable")
								require.Contains(t, fields, "enum")
								require.Contains(t, fields, "pattern")
								require.Contains(t, fields, "format")
								require.Contains(t, fields, "minLength")
								require.Contains(t, fields, "maxLength")

								require.Equal(t, nullableCase.want, string(fields["nullable"]))
								require.Equal(t, enumCase.want, string(fields["enum"]))
								require.Equal(t, patternCase.want, string(fields["pattern"]))
								require.Equal(t, formatCase.want, string(fields["format"]))
								require.Equal(t, minLengthCase.want, string(fields["minLength"]))
								require.Equal(t, maxLengthCase.want, string(fields["maxLength"]))
							})
						}
					}
				}
			}
		}
	}
}

func TestStringDomainHashUsesSHA256OfJSON(t *testing.T) {
	domain := StringDomain{
		Nullable:  true,
		Enum:      []string{"alpha", "beta"},
		Pattern:   new("^[a-z]+$"),
		Format:    new("email"),
		MinLength: 2,
		MaxLength: new(5),
	}

	const domainJSON = `{"nullable":true,"enum":["alpha","beta"],"pattern":"^[a-z]+$","format":"email","minLength":2,"maxLength":5}`
	expectedHash := Hash(sha256.Sum256([]byte(domainJSON)))

	jsonBytes, err := json.Marshal(domain)
	require.NoError(t, err)
	require.Equal(t, domainJSON, string(jsonBytes))
	gotHash, err := domain.Hash()
	require.NoError(t, err)
	require.Equal(t, expectedHash, gotHash)
}
