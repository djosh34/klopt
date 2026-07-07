package domain

import (
	"encoding/json"
	"fmt"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"

	"github.com/stretchr/testify/require"
)

func TestStringDomainImplementsInterfaces(t *testing.T) {
	require.Implements(t, (*types.Domain)(nil), new(StringDomain))
}

func TestStringDomainMarshalJSONZeroValueIncludesAllFields(t *testing.T) {
	jsonBytes, err := json.Marshal(StringDomain{})
	require.NoError(t, err)

	require.Equal(t, `{"nullable":false,"enum":null,"pattern":null,"format":null,"x-valid-examples":null,"x-invalid-examples":null,"minLength":0,"maxLength":null}`, string(jsonBytes))
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
		value []types.Enum
		want  string
	}{
		{name: "enum nil", value: nil, want: "null"},
		{name: "enum empty", value: []types.Enum{}, want: "[]"},
		{name: "enum set", value: []types.Enum{types.Enum("\"alpha\""), types.Enum("\"beta\"")}, want: `["alpha","beta"]`},
	}
	patternCases := []struct {
		name  string
		value types.Pattern
		want  string
	}{
		{name: "pattern nil", value: nil, want: "null"},
		{name: "pattern set", value: types.Pattern{"^[a-z]+$"}, want: `["^[a-z]+$"]`},
	}
	formatCases := []struct {
		name  string
		value types.Format
		want  string
	}{
		{name: "format nil", value: nil, want: "null"},
		{name: "format set", value: types.Format{"email"}, want: `["email"]`},
	}
	xValidExamplesCases := []struct {
		name  string
		value []string
		want  string
	}{
		{name: "x-valid-examples nil", value: nil, want: "null"},
		{name: "x-valid-examples empty", value: []string{}, want: "[]"},
		{name: "x-valid-examples set", value: []string{"alpha"}, want: `["alpha"]`},
	}
	xInvalidExamplesCases := []struct {
		name  string
		value []string
		want  string
	}{
		{name: "x-invalid-examples nil", value: nil, want: "null"},
		{name: "x-invalid-examples empty", value: []string{}, want: "[]"},
		{name: "x-invalid-examples set", value: []string{"123"}, want: `["123"]`},
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
					for _, xValidExamplesCase := range xValidExamplesCases {
						for _, xInvalidExamplesCase := range xInvalidExamplesCases {
							for _, minLengthCase := range minLengthCases {
								for _, maxLengthCase := range maxLengthCases {
									name := fmt.Sprintf(
										"%s/%s/%s/%s/%s/%s/%s/%s",
										nullableCase.name,
										enumCase.name,
										patternCase.name,
										formatCase.name,
										xValidExamplesCase.name,
										xInvalidExamplesCase.name,
										minLengthCase.name,
										maxLengthCase.name,
									)

									t.Run(name, func(t *testing.T) {
										domain := StringDomain{
											Nullable:         nullableCase.value,
											Enum:             enumCase.value,
											Pattern:          patternCase.value,
											Format:           formatCase.value,
											XValidExamples:   xValidExamplesCase.value,
											XInvalidExamples: xInvalidExamplesCase.value,
											MinLength:        minLengthCase.value,
											MaxLength:        maxLengthCase.value,
										}

										jsonBytes, err := json.Marshal(domain)
										require.NoError(t, err)

										var fields map[string]json.RawMessage

										err = json.Unmarshal(jsonBytes, &fields)
										require.NoError(t, err)

										require.Len(t, fields, 8)
										require.Contains(t, fields, "nullable")
										require.Contains(t, fields, "enum")
										require.Contains(t, fields, "pattern")
										require.Contains(t, fields, "format")
										require.Contains(t, fields, "x-valid-examples")
										require.Contains(t, fields, "x-invalid-examples")
										require.Contains(t, fields, "minLength")
										require.Contains(t, fields, "maxLength")

										require.Equal(t, nullableCase.want, string(fields["nullable"]))
										require.Equal(t, enumCase.want, string(fields["enum"]))
										require.Equal(t, patternCase.want, string(fields["pattern"]))
										require.Equal(t, formatCase.want, string(fields["format"]))
										require.Equal(t, xValidExamplesCase.want, string(fields["x-valid-examples"]))
										require.Equal(t, xInvalidExamplesCase.want, string(fields["x-invalid-examples"]))
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
	}
}

func TestStringDomainToHasher(t *testing.T) {
	domain := StringDomain{Nullable: true, Enum: []types.Enum{types.Enum("\"alpha\"")}, Pattern: types.Pattern{"x"}, Format: types.Format{"email"}, XValidExamples: []string{"alpha"}, XInvalidExamples: []string{"123"}, MinLength: 1, MaxLength: new(5)}

	hasher, err := domain.ToHasher()
	require.NoError(t, err)
	require.Equal(t, &hashables.StringHashable{Nullable: true, Enum: []types.Enum{types.Enum("\"alpha\"")}, Pattern: types.Pattern{"x"}, Format: types.Format{"email"}, XValidExamples: []string{"alpha"}, XInvalidExamples: []string{"123"}, MinLength: 1, MaxLength: new(5)}, hasher)
}

func TestStringDomainToHasherNil(t *testing.T) {
	_, err := (*StringDomain)(nil).ToHasher()
	require.Error(t, err)
}
