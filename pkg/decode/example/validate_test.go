package example

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/djosh34/klopt/pkg/validation"

	"github.com/stretchr/testify/require"
)

// TestGeneratedMustDecoderHelpersAdvertiseTheirPanicBoundaries tests deliberate generated Must helpers.
func TestGeneratedMustDecoderHelpersAdvertiseTheirPanicBoundaries(t *testing.T) {
	t.Parallel()

	queryDefinition := validation.QueryDecoderDefinition{OperationID: "query"}
	_, queryErr := validation.NewQueryDecoderFromGenerated(queryDefinition)
	require.Error(t, queryErr)
	require.PanicsWithError(t, queryErr.Error(), func() { mustQueryDecoder(queryDefinition) })

	pathDefinition := validation.PathDecoderDefinition{OperationID: "path"}
	_, pathErr := validation.NewPathDecoderFromGenerated(pathDefinition)
	require.Error(t, pathErr)
	require.PanicsWithError(t, pathErr.Error(), func() { mustPathDecoder(pathDefinition) })
}

// TestRequestValidationBodies is the hand-maintained behavior table for the generated fixture.
func TestRequestValidationBodies(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		operationID string
		body        json.RawMessage
		valid       bool
	}{
		{
			name: "allOf valid", operationID: "allOfObject",
			body: json.RawMessage(`{"first":"x","second":true,"last":1}`), valid: true,
		},
		{
			name: "allOf missing branch property", operationID: "allOfObject",
			body: json.RawMessage(`{"first":"x","second":true}`),
		},
		{name: "composite rejects null", operationID: "compositeObject", body: json.RawMessage(`null`)},
		{name: "optional body absent", operationID: "optionalArrayNullable", valid: true},
		{name: "optional array", operationID: "optionalArrayNullable", body: json.RawMessage(`["x"]`), valid: true},
		{name: "nullable array", operationID: "arrayNullable", body: json.RawMessage(`null`), valid: true},
		{name: "non-nullable array rejects null", operationID: "arrayNotNullable", body: json.RawMessage(`null`)},
		{name: "non-nullable array", operationID: "arrayNotNullable", body: json.RawMessage(`["x"]`), valid: true},
		{
			name:        "closed object",
			operationID: "objectKeysAdditionalPropertiesFalse",
			body:        json.RawMessage(`{"requiredNullableString":null,"requiredNotNullableString":"x"}`),
			valid:       true,
		},
		{
			name:        "closed object rejects extra property",
			operationID: "objectKeysAdditionalPropertiesFalse",
			body:        json.RawMessage(`{"requiredNullableString":null,"requiredNotNullableString":"x","extra":true}`),
		},
		{
			name: "nullable closed object", operationID: "nullableObjectKeysAdditionalPropertiesFalse",
			body: json.RawMessage(`null`), valid: true,
		},
		{name: "nullable string", operationID: "stringNoFormatNullable", body: json.RawMessage(`null`), valid: true},
		{
			name: "non-nullable string rejects null", operationID: "stringNoFormatNotNullable",
			body: json.RawMessage(`null`),
		},
		{
			name: "non-nullable string", operationID: "stringNoFormatNotNullable",
			body: json.RawMessage(`"value"`), valid: true,
		},
		{
			name: "referenced object", operationID: "refObject",
			body: json.RawMessage(`{"refRequiredString":"value"}`), valid: true,
		},
		{name: "referenced object missing required", operationID: "refObject", body: json.RawMessage(`{}`)},
		{name: "reference stress rejects null", operationID: "refStressObject", body: json.RawMessage(`null`)},
		{name: "reference stress put rejects absent body", operationID: "refStressObjectPut"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request, ok := RequestValidations[test.operationID]
			require.True(t, ok)
			require.NotNil(t, request.Body)

			errs := request.Body.Validate(test.body)
			require.Equal(t, test.valid, len(errs) == 0, "%v", errs)
		})
	}
}

// TestAnyOfBodyAndParameters freezes the generated fixture's retained anyOf scope.
func TestAnyOfBodyAndParameters(t *testing.T) {
	t.Parallel()

	request, ok := RequestValidations["anyOfBodyAndParameters"]
	require.True(t, ok)
	require.NotNil(t, request.Body)
	require.NotNil(t, request.Path)
	require.NotNil(t, request.Query)

	for _, test := range []struct {
		name  string
		body  json.RawMessage
		valid bool
	}{
		{name: "first alternative", body: json.RawMessage(`"ab"`), valid: true},
		{name: "later alternative", body: json.RawMessage(`"zz"`), valid: true},
		{name: "all alternatives fail", body: json.RawMessage(`"x"`)},
		{name: "allOf remains active", body: json.RawMessage(`"xz"`)},
		{name: "local type remains active", body: json.RawMessage(`7`)},
	} {
		t.Run("body "+test.name, func(t *testing.T) {
			t.Parallel()

			err := request.Body.Validate(test.body)
			require.Equal(t, test.valid, len(err) == 0, "%v", err)
		})
	}

	for _, test := range []struct {
		name          string
		path          string
		query         string
		expectedPath  string
		expectedQuery string
		valid         bool
	}{
		{
			name: "later string alternative", path: "/any-of/7", query: "q=7",
			expectedPath: `{"id":"7"}`, expectedQuery: `{"q":"7"}`, valid: true,
		},
		{
			name: "first integer alternative", path: "/any-of/12", query: "q=12",
			expectedPath: `{"id":12}`, expectedQuery: `{"q":12}`, valid: true,
		},
		{name: "all alternatives fail", path: "/any-of/8", query: "q=8"},
	} {
		t.Run("parameters "+test.name, func(t *testing.T) {
			t.Parallel()

			path, pathErr := request.Path.DecodePathParams(&url.URL{Path: test.path})

			query, queryErr := request.Query.Decode(&url.URL{RawQuery: test.query})
			if !test.valid {
				require.Error(t, pathErr)
				require.Error(t, queryErr)

				return
			}

			require.NoError(t, pathErr)
			require.NoError(t, queryErr)
			require.JSONEq(t, test.expectedPath, string(path))
			require.JSONEq(t, test.expectedQuery, string(query))
		})
	}
}
