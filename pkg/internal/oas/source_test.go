package oas

import (
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseLocatesRequestSchemaThroughDocumentReferences verifies document-aware selection.
func TestParseLocatesRequestSchemaThroughDocumentReferences(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`
openapi: 3.0.3
paths:
  /things:
    $ref: '#/x-path-items/Things'
x-path-items:
  Things:
    post:
      operationId: createThing
      requestBody:
        $ref: '#/components/requestBodies/CreateThing'
        required: false
components:
  requestBodies:
    CreateThing:
      required: true
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Thing'
            description: ignored
  schemas:
    Thing:
      type: object
      properties:
        child:
          $ref: '#/components/schemas/ChildAlias'
          nullable: true
    ChildAlias:
      $ref: '#/components/schemas/Child'
    Child:
      type: string
`))
	require.NoError(t, err)

	source := sources["createThing"]
	require.True(t, source.RequestBodyRequired)
	require.Equal(
		t,
		"#/components/requestBodies/CreateThing/content/application~1json/schema",
		source.RequestSchema.Pointer,
	)
	require.JSONEq(t, `{"$ref":"#/components/schemas/Thing","description":"ignored"}`, string(source.RequestSchema.Raw))

	rootSchema, err := source.Resolve(source.RequestSchema)
	require.NoError(t, err)
	require.Equal(t, "#/components/schemas/Thing", rootSchema.Pointer)

	child, err := source.Child(rootSchema, "properties", "child")
	require.NoError(t, err)
	resolvedChild, err := source.Resolve(child)
	require.NoError(t, err)
	require.Equal(t, "#/components/schemas/Child", resolvedChild.Pointer)
	require.JSONEq(t, `{"type":"string"}`, string(resolvedChild.Raw))

	// Resolution is lazy: neither root Schema Object nor its Reference Object is replaced.
	var document map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source.Document, &document))
	require.Contains(t, string(source.Document), `"nullable": true`)
	require.Contains(t, string(source.Document), `"$ref": "#/components/schemas/ChildAlias"`)
}

// TestNestedSchemaPositionsResolveWithCanonicalPointers verifies properties, items, and allOf positions.
func TestNestedSchemaPositionsResolveWithCanonicalPointers(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`
openapi: 3.0.3
paths:
  /escaped/~things:
    post:
      operationId: nestedThing
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                a/b:
                  type: array
                  items:
                    $ref: '#/components/schemas/Alias'
              allOf:
                - $ref: '#/components/schemas/Base'
              additionalProperties:
                $ref: '#/components/schemas/A~1B~0C'
components:
  schemas:
    Alias:
      $ref: '#/components/schemas/Base'
    Base:
      type: integer
    A/B~C:
      type: boolean
`))
	require.NoError(t, err)

	source := sources["nestedThing"]
	require.Equal(
		t,
		"#/paths/~1escaped~1~0things/post/requestBody/content/application~1json/schema",
		source.RequestSchema.Pointer,
	)

	item, err := source.Child(source.RequestSchema, "properties", "a/b", "items")
	require.NoError(t, err)
	require.Contains(t, item.Pointer, "/properties/a~1b/items")
	item, err = source.Resolve(item)
	require.NoError(t, err)
	require.Equal(t, "#/components/schemas/Base", item.Pointer)

	allOfChild, err := source.Child(source.RequestSchema, "allOf", "0")
	require.NoError(t, err)
	allOfChild, err = source.Resolve(allOfChild)
	require.NoError(t, err)
	require.Equal(t, "#/components/schemas/Base", allOfChild.Pointer)

	additional, err := source.Child(source.RequestSchema, "additionalProperties")
	require.NoError(t, err)
	additional, err = source.Resolve(additional)
	require.NoError(t, err)
	require.Equal(t, "#/components/schemas/A~1B~0C", additional.Pointer)
	require.JSONEq(t, `{"type":"boolean"}`, string(additional.Raw))
}

// TestParseAppliesJSONRequestOperationPolicy verifies document-wide inclusion and failures.
func TestParseAppliesJSONRequestOperationPolicy(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		spec      string
		wantError string
	}{
		"duplicate operation": {
			spec: `
openapi: 3.0.3
paths:
  /first:
    get:
      operationId: duplicate
      requestBody:
        content:
          application/json:
            schema: {type: string}
  /second:
    post:
      operationId: duplicate
      requestBody:
        content:
          application/json:
            schema: {type: string}
`,
			wantError: `operationId "duplicate" is duplicated at #/paths/~1first/get and #/paths/~1second/post`,
		},
		"duplicate operation through shared path item reference": {
			spec: `
openapi: 3.0.3
paths:
  /first:
    $ref: '#/x-path-items/Shared'
  /second:
    $ref: '#/x-path-items/Shared'
x-path-items:
  Shared:
    post:
      operationId: duplicate
      requestBody:
        content:
          application/json:
            schema: {type: string}
`,
			wantError: `operationId "duplicate" is duplicated at #/paths/~1first/post and #/paths/~1second/post`,
		},
		"duplicate operation with non-JSON body": {
			spec: `
openapi: 3.0.3
paths:
  /first:
    post:
      operationId: duplicate
      requestBody:
        content:
          application/json:
            schema: {type: string}
  /second:
    post:
      operationId: duplicate
      requestBody:
        content:
          text/plain:
            schema: {type: string}
`,
			wantError: `operationId "duplicate" is duplicated at #/paths/~1first/post and #/paths/~1second/post`,
		},
		"missing operation ID": {
			spec: `
openapi: 3.0.3
paths:
  /things:
    post:
      requestBody:
        content:
          application/json:
            schema: {type: string}
`,
			wantError: "operationId must be a non-empty string",
		},
		"non-string operation ID": {
			spec: `
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: 7
      requestBody:
        content:
          application/json:
            schema: {type: string}
`,
			wantError: "operationId must be a non-empty string",
		},
		"empty operation ID": {
			spec: `
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: ''
      requestBody:
        content:
          application/json:
            schema: {type: string}
`,
			wantError: "operationId must be a non-empty string",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse([]byte(tt.spec))
			require.ErrorContains(t, err, tt.wantError)
		})
	}

	sources, err := Parse([]byte(`
openapi: 3.0.3
paths:
  /bodyless:
    get: {operationId: bodyless}
  /plain:
    post:
      operationId: plain
      requestBody:
        content:
          text/plain:
            schema: {type: string}
  /json:
    post:
      operationId: exactID
      requestBody:
        content:
          application/json:
            schema: {type: string}
`))
	require.NoError(t, err)
	require.Equal(t, []string{"bodyless", "exactID", "plain"}, slices.Sorted(maps.Keys(sources)))
}

// TestParseTreatsMissingRequestBodySchemaAsEmpty verifies optional Media Type schema parity.
func TestParseTreatsMissingRequestBodySchemaAsEmpty(t *testing.T) {
	t.Parallel()

	for name, mediaType := range map[string]string{
		"absent":   "{}",
		"explicit": "{schema: {}}",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /things:
    post:
      operationId: create
      requestBody:
        content:
          application/json: ` + mediaType))
			require.NoError(t, err)

			schema := sources["create"].RequestSchema
			require.Equal(t, "#/paths/~1things/post/requestBody/content/application~1json/schema", schema.Pointer)
			require.JSONEq(t, `{}`, string(schema.Raw))
		})
	}
}

// TestParseRejectsMalformedSelectedJSONMediaType verifies the actual selected media-key pointer.
func TestParseRejectsMalformedSelectedJSONMediaType(t *testing.T) {
	t.Parallel()

	for name, mediaType := range map[string]string{
		"null":    "null",
		"boolean": "true",
		"number":  "7",
		"string":  `"invalid"`,
		"array":   "[]",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse([]byte(`openapi: 3.0.3
paths:
  /things:
    post:
      operationId: create
      requestBody:
        content:
          'application/json; charset=utf-8': ` + mediaType))
			require.ErrorContains(
				t,
				err,
				"parse operation at #/paths/~1things/post request body:",
			)
			require.ErrorContains(
				t,
				err,
				"#/paths/~1things/post/requestBody/content/application~1json; charset=utf-8",
			)
		})
	}
}

// TestParseSelectsApplicationJSONMediaRangesBySpecificity verifies request content matching.
func TestParseSelectsApplicationJSONMediaRangesBySpecificity(t *testing.T) {
	t.Parallel()

	for name, content := range map[string]string{
		"application wildcard": "application/*: {schema: {type: string}}",
		"global wildcard":      "'*/*': {schema: {type: boolean}}",
		"exact wins":           "'*/*': {schema: {type: boolean}}\n          application/json: {schema: {type: string}}",
		"parameterized exact":  "'application/json; charset=utf-8': {schema: {type: string}}",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /things:
    post:
      operationId: create
      requestBody:
        content:
          ` + content))
			require.NoError(t, err)
			require.Contains(t, sources, "create")

			source := sources["create"]

			if name == "exact wins" {
				require.JSONEq(t, `{"type":"string"}`, string(source.RequestSchema.Raw))
			}
		})
	}
}

// TestParseRejectsNullRequestBodyRequired verifies null is not decoded as false.
func TestParseRejectsNullRequestBodyRequired(t *testing.T) {
	t.Parallel()

	_, err := Parse([]byte(`openapi: 3.0.3
paths:
  /things:
    post:
      operationId: create
      requestBody:
        required: null
        content:
          application/json: {schema: {type: string}}`))
	require.ErrorContains(t, err, "required must be a boolean")
}

// TestResolveReportsExternalAndCyclicReferences verifies clear reference failures.
func TestResolveReportsExternalAndCyclicReferences(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		ref       string
		aliases   string
		wantError string
	}{
		"external": {
			ref:       "other.yaml#/Thing",
			wantError: "external reference",
		},
		"cycle": {
			ref: "#/components/schemas/A",
			aliases: `
    A: {$ref: '#/components/schemas/B'}
    B: {$ref: '#/components/schemas/A'}
`,
			wantError: "reference cycle",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			spec := []byte(`
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: create
      requestBody:
        content:
          application/json:
            schema: {$ref: '` + tt.ref + `'}
components:
  schemas:
` + tt.aliases)
			sources, err := Parse(spec)
			require.NoError(t, err)

			source := sources["create"]

			_, err = source.Resolve(source.RequestSchema)
			require.ErrorContains(t, err, tt.wantError)

			var referenceError *ReferenceError
			require.True(t, errors.As(err, &referenceError))
			require.NotEmpty(t, referenceError.Referrer)
			require.NotEmpty(t, referenceError.Reference)
		})
	}
}

// TestRequestBodyRequiredDefaultsFalse verifies the OpenAPI default.
func TestRequestBodyRequiredDefaultsFalse(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: create
      requestBody:
        content:
          application/json:
            schema: {type: boolean}
`))
	require.NoError(t, err)

	source := sources["create"]
	require.False(t, source.RequestBodyRequired)
}

// TestParseRejectsDuplicateJSONRootNames verifies the shared root-object preflight.
func TestParseRejectsDuplicateJSONRootNames(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`{"openapi":"3.0.3","openapi":"3.0.4","paths":{}}`))
	require.Nil(t, sources)
	require.ErrorContains(t, err, `duplicate object name "openapi"`)
}

// TestParseWrapsInvalidOperationIDSentinel verifies acquisition preserves the public sentinel.
func TestParseWrapsInvalidOperationIDSentinel(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /things:
    get:
      operationId: bad.operation
`))
	require.Nil(t, sources)
	require.ErrorIs(t, err, ErrInvalidOperationID)
}

// TestParseRejectsPathKeyWithoutLeadingSlashBeforeAcquisition covers zero-operation Paths members.
func TestParseRejectsPathKeyWithoutLeadingSlashBeforeAcquisition(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  things: {}
`))
	require.Nil(t, sources)
	require.ErrorContains(t, err, `path template "things" must begin with /`)
}

// TestParseRejectsDuplicateDecodedJSONNamesRecursively covers every shared ingestion branch.
func TestParseRejectsDuplicateDecodedJSONNamesRecursively(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"escaped equivalent Paths keys": `{"openapi":"3.0.3","paths":{"/pets":{},"\u002fpets":{}}}`,
		"Paths extension value":         `{"openapi":"3.0.3","paths":{"x-meta":{"a":1,"\u0061":2}}}`,
		"request body": `{"openapi":"3.0.3","paths":{"/pets":{"post":{"operationId":"createPet",` +
			`"requestBody":{"required":true,"required":false,"content":{"application/json":{}}}}}}}`,
		"query parameter": `{"openapi":"3.0.3","paths":{"/pets":{"get":{"operationId":"getPets",` +
			`"parameters":[{"name":"q","\u006eame":"other","in":"query","schema":{}}]}}}}`,
		"schema": `{"openapi":"3.0.3","paths":{"/pets":{"post":{"operationId":"createPet",` +
			`"requestBody":{"content":{"application/json":{"schema":{"type":"string","\u0074ype":"number"}}}}}}}}`,
	}

	for name, spec := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sources, err := Parse([]byte(spec))
			require.Nil(t, sources)
			require.ErrorContains(t, err, "duplicate object name")
		})
	}
}

// TestParseRetainsYAMLDuplicateRejection keeps YAML's existing strict behavior.
func TestParseRetainsYAMLDuplicateRejection(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte("openapi: 3.0.3\nopenapi: 3.0.4\npaths: {}\n"))
	require.Nil(t, sources)
	require.Error(t, err)
}

// TestParseClassifiesPathsExtensionsBeforeAcquisition verifies extensions stay opaque.
func TestParseClassifiesPathsExtensionsBeforeAcquisition(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  x-path-item:
    $ref: other.yaml#/Ignored
    malformed: true
  /pets:
    get: {operationId: getPets}
`))
	require.NoError(t, err)
	require.Equal(t, []string{"getPets"}, slices.Sorted(maps.Keys(sources)))
}

// TestParseValidatesEveryPathBeforeOperationAcquisition covers the complete Paths preflight.
func TestParseValidatesEveryPathBeforeOperationAcquisition(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		paths    string
		contains string
	}{
		"identical hierarchy": {
			paths:    "  /pets/{id}: {}\n  /pets/{name}: {}\n",
			contains: "identical templated hierarchy",
		},
		"unknown Path Item field": {
			paths:    "  /pets:\n    connect: {}\n",
			contains: `unknown Path Item field "connect"`,
		},
		"reached external Path Item": {
			paths:    "  /pets:\n    $ref: other.yaml#/Pets\n",
			contains: "external reference",
		},
		"adjacent expressions": {
			paths:    "  /pets/{id}{name}: {}\n",
			contains: "adjacent expressions",
		},
		"repeated expression": {
			paths:    "  /pets/{id}/{id}: {}\n",
			contains: "repeats expression",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sources, err := Parse([]byte("openapi: 3.0.3\npaths:\n" + test.paths))
			require.Nil(t, sources)
			require.ErrorContains(t, err, test.contains)
		})
	}

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /{entity}/mine: {}
  /pets/{id}: {}
`))
	require.NoError(t, err)
	require.Empty(t, sources)
}

// TestParseDoesNotReachUntouchedOperationBranches verifies reference reachability remains lazy.
func TestParseDoesNotReachUntouchedOperationBranches(t *testing.T) {
	t.Parallel()

	sources, err := Parse([]byte(`openapi: 3.0.3
paths:
  /pets:
    get:
      operationId: getPets
      callbacks:
        ignored:
          '{$request.body#/url}':
            post:
              requestBody: {$ref: other.yaml#/CallbackBody}
      responses:
        default: {$ref: other.yaml#/Response}
components:
  schemas:
    Unused: {$ref: other.yaml#/Schema}
`))
	require.NoError(t, err)
	require.Contains(t, sources, "getPets")
}
