//nolint:godoclint,lll // Parameter interface fixtures keep complete OpenAPI cases together.
package oas

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMergesResolvedQueryParameters(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items/{id}:
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
	require.Equal(t, "#/paths/~1items~1{id}/get/parameters/0", parameters[0].Pointer)
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

func TestParseValidatesPathItemParametersWithoutOperations(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items/{id}:
    parameters:
      - {name: id, in: path, required: false, schema: {type: string}}
`))
	require.Nil(t, sources)
	require.ErrorContains(t, err, `path parameter "id"`)
	require.ErrorContains(t, err, `required must be true`)
}

func TestParseValidatesPathItemParameterCorrespondenceWithoutOperations(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"other", "a/b"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sources, err := Parse(fmt.Appendf(nil, `openapi: 3.0.3
paths:
  /items/{id}:
    parameters:
      - {name: %q, in: path, required: true, schema: {type: string}}
`, name))
			require.Nil(t, sources)
			require.ErrorContains(t, err, `path parameter "`+name+`"`)
		})
	}
}

func TestParseRejectsPathParameterWithoutMatchingExpression(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items/{id}:
    get:
      operationId: getItem
      parameters:
        - {name: other, in: path, required: true, schema: {type: string}}
`))
	require.Nil(t, sources)
	require.ErrorContains(t, err, `path template "/items/{id}" expression "id" has no parameter declaration`)
}

func TestParseValidatesIgnoredHeaderParameter(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items:
    get:
      operationId: getItems
      parameters:
        - {name: X-Filter, in: header, required: nope, schema: {type: string}}
`))
	require.Nil(t, sources)
	require.ErrorContains(t, err, `header parameter "X-Filter"`)
	require.ErrorContains(t, err, `required must be a boolean`)
}

func TestParseValidatesIgnoredCookieParameterContentMediaType(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items:
    get:
      operationId: getItems
      parameters:
        - name: filter
          in: cookie
          content:
            invalid: {}
`))
	require.Nil(t, sources)
	require.ErrorContains(t, err, `cookie parameter "filter"`)
	require.ErrorContains(t, err, `media type "invalid" is malformed`)
}

func TestParseMergesQueryAndPathParametersByExactIdentity(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items/{id}/{slug}:
    parameters:
      - {name: id, in: path, required: true, schema: {type: string}}
      - {name: q, in: query, schema: {type: string}}
      - {name: X-Ignored, in: header, schema: {type: string}}
    get:
      operationId: getItem
      parameters:
        - {name: id, in: path, required: true, schema: {type: integer}}
        - {name: slug, in: path, required: true, schema: {type: string}}
`))
	require.NoError(t, err)

	source := sources["getItem"]
	require.Equal(t, "/items/{id}/{slug}", source.PathTemplate)
	require.Equal(t, []string{"q"}, parameterNames(t, source.QueryParameters))
	require.Equal(t, []string{"id", "slug"}, parameterNames(t, source.PathParameters))
	require.Equal(t, "#/paths/~1items~1{id}~1{slug}/get/parameters/0", source.PathParameters[0].Pointer)
}

func TestParseRejectsInvalidOverriddenPathItemParameter(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items/{id}:
    parameters:
      - {name: id, in: path, required: false, schema: {type: string}}
    get:
      operationId: getItem
      parameters:
        - {name: id, in: path, required: true, schema: {type: integer}}
`))
	require.Nil(t, sources)
	require.ErrorContains(t, err, `path parameter "id"`)
	require.ErrorContains(t, err, "required must be true")
}

func TestParseValidatesOperationParametersBeforeOperationID(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items:
    get:
      operationId: not-valid-
      parameters:
        - {name: q, in: query, required: nope, schema: {type: string}}
`))
	require.Nil(t, sources)
	require.ErrorContains(t, err, `query parameter "q"`)
	require.ErrorContains(t, err, "required must be a boolean")
	require.NotContains(t, err.Error(), "invalid operation ID")
}

func TestParseRejectsMutuallyExclusiveParameterExamples(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items:
    get:
      operationId: getItems
      parameters:
        - name: q
          in: query
          schema: {type: string}
          example: value
          examples: {named: {value: value}}
`))
	require.Nil(t, sources)
	require.ErrorContains(t, err, `parameter "q"`)
	require.ErrorContains(t, err, "example and examples are mutually exclusive")
}

func TestParseRejectsUnknownParameterFieldsAndAllowsExtensions(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items:
    get:
      operationId: getItems
      parameters:
        - {name: q, in: query, unexpected: true, schema: {type: string}}
`))
	require.Nil(t, sources)
	require.ErrorContains(t, err, `parameter "q"`)
	require.ErrorContains(t, err, `unknown field "unexpected"`)

	sources, err = Parse([]byte(`openapi: 3.0.3
paths:
  /items:
    get:
      operationId: getItems
      parameters:
        - {name: q, in: query, x-owner: team, schema: {type: string}}
`))
	require.NoError(t, err)
	require.Contains(t, sources, "getItems")
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
		{name: "in invalid", opParams: `[{name: q, in: matrix, schema: {type: string}}]`, contains: "in must be one of"},
		{name: "in wrong shape", opParams: `[{name: q, in: 1, schema: {type: string}}]`, contains: "in must be"},
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
