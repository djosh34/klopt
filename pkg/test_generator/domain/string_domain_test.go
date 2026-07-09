package domain

import (
	"encoding/json"
	"fmt"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.

	"github.com/stretchr/testify/require"
)

// TestStringDomainImplementsInterfaces checks the domain contract.
func TestStringDomainImplementsInterfaces(t *testing.T) {
	t.Parallel()

	require.Implements(t, (*types.Domain)(nil), new(StringDomain))
}

// TestStringDomainMarshalJSONZeroValueIncludesAllFields checks stable JSON shape.
func TestStringDomainMarshalJSONZeroValueIncludesAllFields(t *testing.T) {
	t.Parallel()

	jsonBytes, err := json.Marshal(StringDomain{})
	require.NoError(t, err)

	expected := `{"pattern":null,"format":null,"nullable":false,"enum":null,` +
		`"x-valid-examples":null,"x-invalid-examples":null,"minLength":0,"maxLength":null}`
	require.Equal(t, expected, string(jsonBytes))
}

// TestStringDomainMarshalJSONAllCombinations checks every field-state combination.
func TestStringDomainMarshalJSONAllCombinations(t *testing.T) {
	t.Parallel()

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
		{
			name:  "enum set",
			value: []types.Enum{types.Enum("\"alpha\""), types.Enum("\"beta\"")},
			want:  `["alpha","beta"]`,
		},
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

	totalCases := len(nullableCases) * len(enumCases) * len(patternCases) * len(formatCases) *
		len(xValidExamplesCases) * len(xInvalidExamplesCases) * len(minLengthCases) * len(maxLengthCases)
	for caseIndex := range totalCases {
		selector := caseIndex
		nullableCase := nullableCases[selector%len(nullableCases)]
		selector /= len(nullableCases)
		enumCase := enumCases[selector%len(enumCases)]
		selector /= len(enumCases)
		patternCase := patternCases[selector%len(patternCases)]
		selector /= len(patternCases)
		formatCase := formatCases[selector%len(formatCases)]
		selector /= len(formatCases)
		xValidExamplesCase := xValidExamplesCases[selector%len(xValidExamplesCases)]
		selector /= len(xValidExamplesCases)
		xInvalidExamplesCase := xInvalidExamplesCases[selector%len(xInvalidExamplesCases)]
		selector /= len(xInvalidExamplesCases)
		minLengthCase := minLengthCases[selector%len(minLengthCases)]
		selector /= len(minLengthCases)
		maxLengthCase := maxLengthCases[selector%len(maxLengthCases)]

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
			t.Parallel()

			domain := StringDomain{
				Pattern:          patternCase.value,
				Format:           formatCase.value,
				Nullable:         nullableCase.value,
				Enum:             enumCase.value,
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

// TestStringDomainGenerateHash checks deterministic string-domain hashing.
func TestStringDomainGenerateHash(t *testing.T) {
	t.Parallel()

	domain := StringDomain{
		Pattern:          types.Pattern{"x"},
		Format:           types.Format{"email"},
		Nullable:         true,
		Enum:             []types.Enum{types.Enum("\"alpha\"")},
		XValidExamples:   []string{"alpha"},
		XInvalidExamples: []string{"123"},
		MinLength:        1,
		MaxLength:        new(5),
	}

	got, err := domain.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, requireGeneratedHash(t, "string", domain), got)
}

// TestStringDomainGenerateHashNil rejects a nil string domain.
func TestStringDomainGenerateHashNil(t *testing.T) {
	t.Parallel()

	_, err := (*StringDomain)(nil).GenerateHash()
	require.Error(t, err)
}

// TestStringDomainGenerateHashFinalizesConstraints checks hash-time filtering without receiver mutation.
func TestStringDomainGenerateHashFinalizesConstraints(t *testing.T) {
	t.Parallel()

	domain := StringDomain{
		Nullable:         true,
		Enum:             []types.Enum{types.Enum(`"ccc"`), types.Enum(`null`), types.Enum(`"bb"`)},
		XValidExamples:   []string{"bb", "other"},
		XInvalidExamples: []string{"x"},
		MinLength:        2,
		MaxLength:        new(2),
	}
	before := domain

	got, err := domain.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, before, domain)
	require.Equal(t, requireGeneratedHash(t, "string", StringDomain{
		Nullable:         true,
		Enum:             []types.Enum{types.Enum(`"bb"`), types.Enum(`null`)},
		XValidExamples:   []string{"bb"},
		XInvalidExamples: []string{"x"},
		MinLength:        2,
		MaxLength:        new(2),
	}), got)
}

// TestStringDomainGenerateHashRejectsUnsatisfiableConstraints covers hash-time failures.
func TestStringDomainGenerateHashRejectsUnsatisfiableConstraints(t *testing.T) {
	t.Parallel()

	tests := map[string]*StringDomain{
		"contradictory length bounds": {
			MinLength: 2,
			MaxLength: new(1),
		},
		"enum outside length bounds": {
			Enum:      []types.Enum{types.Enum(`"a"`)},
			MinLength: 2,
		},
		"enum outside valid examples": {
			Enum:           []types.Enum{types.Enum(`"a"`)},
			XValidExamples: []string{"b"},
		},
		"nullable enum without null": {
			Nullable:  true,
			Enum:      []types.Enum{types.Enum(`"a"`)},
			MinLength: 2,
			MaxLength: new(1),
		},
	}

	for name, domain := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := domain.GenerateHash()
			require.Error(t, err)
		})
	}
}

// TestStringDomainGenerateHashAllowsNullOnlySatisfiability checks nullable rescue at hash time.
func TestStringDomainGenerateHashAllowsNullOnlySatisfiability(t *testing.T) {
	t.Parallel()

	domain := StringDomain{
		Nullable:  true,
		Enum:      []types.Enum{types.Enum(`"a"`), types.Enum(`null`)},
		MinLength: 2,
		MaxLength: new(1),
	}

	_, err := domain.GenerateHash()
	require.NoError(t, err)
}

// TestEnumCanonicalMarshalingAndHash checks semantic enum normalization.
func TestEnumCanonicalMarshalingAndHash(t *testing.T) {
	t.Parallel()

	left := types.Enum(`{"text":"1","value":1.0,"object":{"b":2,"a":1}}`)
	right := types.Enum(`{"object":{"a":1e0,"b":2.0},"value":1,"text":"\u0031"}`)

	leftJSON, err := json.Marshal(left)
	require.NoError(t, err)
	rightJSON, err := json.Marshal(right)
	require.NoError(t, err)
	require.Equal(t, `{"object":{"a":1,"b":2},"text":"1","value":1}`, string(leftJSON))
	require.Equal(t, leftJSON, rightJSON)

	leftHash, err := left.GenerateHash()
	require.NoError(t, err)
	rightHash, err := right.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, leftHash, rightHash)

	invalid := types.Enum(`true false`)
	_, err = json.Marshal(invalid)
	require.Error(t, err)
	_, err = invalid.GenerateHash()
	require.Error(t, err)
}
