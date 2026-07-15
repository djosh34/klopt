//nolint:godoclint,lll // Query interface fixtures keep complete OpenAPI cases together.
package oas

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMergesResolvedQueryParameters(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items:
    parameters:
      - $ref: '#/components/parameters/PathQ'
      - {name: id, in: path, required: true, schema: {type: string}}
      - {name: keep, in: query, schema: {type: boolean}}
    get:
      operationId: query
      parameters:
        - {name: q, in: query, schema: {type: integer}}
        - {name: appended, in: query, schema: {type: number}}
components:
  parameters:
    PathQ: {name: q, in: query, schema: {type: string}}
`))
	require.NoError(t, err)
	require.Contains(t, sources, "query")
	parameters := sources["query"].QueryParameters
	require.Len(t, parameters, 3)
	require.Equal(t, []string{"q", "keep", "appended"}, parameterNames(t, parameters))
	require.Equal(t, "#/paths/~1items/get/parameters/0", parameters[0].Pointer)
}

func TestParseRejectsDuplicateParameterIdentityWithinOneLevel(t *testing.T) {
	t.Parallel()

	_, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items:
    get:
      operationId: query
      parameters:
        - {name: q, in: query, schema: {type: string}}
        - {name: q, in: query, schema: {type: number}}
`))
	require.ErrorContains(t, err, `parameter ("q", "query") is duplicated`)
}

func TestParseRejectsMalformedParameterListsAndIdentities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pathParams string
		opParams   string
		contains   string
	}{
		{name: "path list shape", pathParams: `{}`, contains: "path item parameters"},
		{name: "operation list shape", opParams: `{}`, contains: "operation parameters"},
		{name: "null list", opParams: `null`, contains: "must be an array"},
		{name: "bad reference", opParams: `[{$ref: '#/missing'}]`, contains: "resolve reference"},
		{name: "parameter scalar", opParams: `[1]`, contains: "must be an object"},
		{name: "name absent", opParams: `[{in: query, schema: {type: string}}]`, contains: "name must be"},
		{name: "in absent", opParams: `[{name: q, schema: {type: string}}]`, contains: "in must be"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			spec := `openapi: 3.0.3
paths:
  /items:
`
			if test.pathParams != "" {
				spec += "    parameters: " + test.pathParams + "\n"
			}

			spec += `    get:
      operationId: query
`
			if test.opParams != "" {
				spec += "      parameters: " + test.opParams + "\n"
			}

			_, err := Parse([]byte(spec))
			require.ErrorContains(t, err, test.contains)
		})
	}
}

func TestParameterListRejectsNonObjectParent(t *testing.T) {
	t.Parallel()

	_, err := (Source{}).parameterList(LocatedSchema{Raw: json.RawMessage(`[]`), Pointer: "#/parent"})
	require.ErrorContains(t, err, "parse object")
}

func TestParseRejectsMalformedRequestBodyBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		operation string
		contains  string
	}{
		{name: "operation scalar", operation: `1`, contains: "parse operation"},
		{name: "operation null", operation: `null`, contains: "operation must be an object"},
		{name: "body reference", operation: `{operationId: query, requestBody: {$ref: '#/missing'}}`, contains: "request body"},
		{name: "body scalar", operation: `{operationId: query, requestBody: 1}`, contains: "parse operation"},
		{name: "body null", operation: `{operationId: query, requestBody: null}`, contains: "must be an object"},
		{name: "content absent", operation: `{operationId: query, requestBody: {}}`, contains: "content does not exist"},
		{name: "content scalar", operation: `{operationId: query, requestBody: {content: 1}}`, contains: "request body content"},
		{name: "content null", operation: `{operationId: query, requestBody: {content: null}}`, contains: "content must be an object"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			spec := []byte("openapi: 3.0.3\npaths:\n  /items:\n    get: " + test.operation + "\n")
			_, err := Parse(spec)
			require.ErrorContains(t, err, test.contains)
		})
	}
}

func parameterNames(t *testing.T, parameters []LocatedSchema) []string {
	t.Helper()

	names := make([]string, len(parameters))
	for index, parameter := range parameters {
		var members map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(parameter.Raw, &members))
		require.NoError(t, json.Unmarshal(members["name"], &names[index]))
	}

	return names
}
