//nolint:godoclint,lll // Stage-specific tests use inline OAS declarations at the private compiler seam.
package validation

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/stretchr/testify/require"
)

func TestCompilePathDecoderBuildsSimpleStringMetadata(t *testing.T) {
	t.Parallel()

	decoder, err := compilePathDecoderForTest(t, `
      parameters:
        - {name: id, in: path, required: true, schema: {type: string}}
`)
	require.NoError(t, err)
	require.Equal(t, PathDecoderDefinition{
		OperationID: "path", PathTemplate: "/items/{id}",
		Parameters: []PathParameterDefinition{{
			Name: "id", Wire: uint8(pathWireSimplePrimitive),
			Validation: decoder.parameters[0].validation, ScalarType: "string",
			Properties: []PathPropertyDefinition{},
		}},
	}, decoder.Definition())

	actual, err := decoder.DecodePathParams(&url.URL{Path: "/items/value"})
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"value"}`, string(actual))
}

func TestCompilePathDecoderDerivesStyleMetadataFromCompiledValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		parameter       string
		path            string
		expectedWire    pathWireKind
		expectedExplode bool
		expectedScalar  string
		expectedDynamic string
		expectedProps   []PathPropertyDefinition
		expectedJSON    string
	}{
		{
			name: "array item allOf and enum", path: "/items/.1.2",
			parameter: `{name: id, in: path, required: true, style: label, explode: true,
          schema: {allOf: [{type: array, items: {allOf: [{type: number}, {enum: [1, 2]}]}}]}}`,
			expectedWire: pathWireLabelArray, expectedExplode: true, expectedScalar: "integer",
			expectedProps: []PathPropertyDefinition{}, expectedJSON: `{"id":[1,2]}`,
		},
		{
			name: "object slots", path: "/items/;count=2;flag=true;other=3.0",
			parameter: `{name: id, in: path, required: true, style: matrix, explode: true,
          schema: {allOf: [{type: object, properties: {count: {enum: [1, 2]}, flag: {}},
            additionalProperties: {allOf: [{type: number}, {type: integer}]}}]}}`,
			expectedWire: pathWireMatrixObject, expectedExplode: true, expectedDynamic: "integer",
			expectedProps: []PathPropertyDefinition{{Name: "count", ScalarType: "integer"}, {Name: "flag", ScalarType: "string"}},
			expectedJSON:  `{"id":{"count":2,"flag":"true","other":3}}`,
		},
		{
			name: "mixed enum root uses string", path: "/items/x",
			parameter:    `{name: id, in: path, required: true, schema: {enum: [1, x]}}`,
			expectedWire: pathWireSimplePrimitive, expectedScalar: "string",
			expectedProps: []PathPropertyDefinition{}, expectedJSON: `{"id":"x"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := compilePathDecoderForTest(t, "\n      parameters:\n        - "+test.parameter+"\n")
			require.NoError(t, err)

			definition := decoder.Definition().Parameters[0]
			require.Equal(t, uint8(test.expectedWire), definition.Wire)
			require.Equal(t, test.expectedExplode, definition.Explode)
			require.Equal(t, test.expectedScalar, definition.ScalarType)
			require.Equal(t, test.expectedDynamic, definition.DynamicType)
			require.Equal(t, test.expectedProps, definition.Properties)

			actual, err := decoder.DecodePathParams(&url.URL{Path: test.path})
			require.NoError(t, err)
			require.JSONEq(t, test.expectedJSON, string(actual))
		})
	}
}

func TestCompilePathDecoderSupportsSchemaLessJSONContent(t *testing.T) {
	t.Parallel()

	decoder, err := compilePathDecoderForTest(t, `
      parameters:
        - {name: id, in: path, required: true, content: {application/json: {}}}
`)
	require.NoError(t, err)
	require.Equal(t, uint8(pathWireJSONContent), decoder.Definition().Parameters[0].Wire)

	actual, err := decoder.DecodePathParams(&url.URL{Path: "/items/null"})
	require.NoError(t, err)
	require.JSONEq(t, `{"id":null}`, string(actual))
}

func TestPathDecoderAnyOfUsesFirstCompleteSchemaStyleConversion(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		schema   string
		expected string
	}{
		{name: "integer first", schema: `{anyOf: [{type: integer}, {type: string}]}`, expected: `{"id":7}`},
		{name: "string first", schema: `{anyOf: [{type: string}, {type: integer}]}`, expected: `{"id":"7"}`},
		{
			name:     "later branch after validation failure",
			schema:   `{anyOf: [{type: integer, minimum: 10}, {type: string, pattern: '^7$'}]}`,
			expected: `{"id":"7"}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := compilePathDecoderForTest(t, `
      parameters:
        - {name: id, in: path, required: true, schema: `+test.schema+`}
`)
			require.NoError(t, err)

			actual, err := decoder.DecodePathParams(&url.URL{Path: "/items/7"})
			require.NoError(t, err)
			require.JSONEq(t, test.expected, string(actual))
		})
	}
}

func TestPathDecoderAnyOfConvertsNestedSchemaStyleValues(t *testing.T) {
	t.Parallel()

	choice := `{anyOf: [{type: integer}, {type: string}]}`
	for _, test := range []struct {
		name      string
		parameter string
		path      string
		expected  string
	}{
		{
			name: "array items", parameter: `{name: id, in: path, required: true, schema: {type: array, items: ` + choice + `}}`,
			path: "/items/7,x", expected: `{"id":[7,"x"]}`,
		},
		{
			name: "declared property", parameter: `{name: id, in: path, required: true, explode: true, schema: {type: object, additionalProperties: false, properties: {value: ` + choice + `}}}`,
			path: "/items/value=7", expected: `{"id":{"value":7}}`,
		},
		{
			name: "additional property", parameter: `{name: id, in: path, required: true, explode: true, schema: {type: object, additionalProperties: ` + choice + `}}`,
			path: "/items/value=7", expected: `{"id":{"value":7}}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := compilePathDecoderForTest(t, "\n      parameters:\n        - "+test.parameter+"\n")
			require.NoError(t, err)
			actual, err := decoder.DecodePathParams(&url.URL{Path: test.path})
			require.NoError(t, err)
			require.JSONEq(t, test.expected, string(actual))
		})
	}
}

func TestPathDecoderAnyOfConvertsRootObjectsPerBranch(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		schema   string
		path     string
		expected string
	}{
		{
			name: "branch-only declared integer property",
			schema: `{anyOf: [
              {type: object, required: [count], additionalProperties: false, properties: {count: {type: integer}}},
              {type: object, required: [label], additionalProperties: false, properties: {label: {type: string}}}
            ]}`,
			path: "/items/count,7", expected: `{"id":{"count":7}}`,
		},
		{
			name: "later declared-property branch",
			schema: `{anyOf: [
              {type: object, required: [count], additionalProperties: false, properties: {count: {type: integer}}},
              {type: object, required: [label], additionalProperties: false, properties: {label: {type: string}}}
            ]}`,
			path: "/items/label,x", expected: `{"id":{"label":"x"}}`,
		},
		{
			name: "ordered additional property conversion",
			schema: `{anyOf: [
              {type: object, additionalProperties: {type: integer}},
              {type: object, additionalProperties: {type: string}}
            ]}`,
			path: "/items/value,7", expected: `{"id":{"value":7}}`,
		},
		{
			name: "reversed additional property conversion",
			schema: `{anyOf: [
              {type: object, additionalProperties: {type: string}},
              {type: object, additionalProperties: {type: integer}}
            ]}`,
			path: "/items/value,7", expected: `{"id":{"value":"7"}}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := compilePathDecoderForTest(t, `
      parameters:
        - {name: id, in: path, required: true, schema: `+test.schema+`}
`)
			require.NoError(t, err)
			actual, err := decoder.DecodePathParams(&url.URL{Path: test.path})
			require.NoError(t, err)
			require.JSONEq(t, test.expected, string(actual))
		})
	}
}

func TestPathDecoderAnyOfInfersEnumOnlyArrayItems(t *testing.T) {
	t.Parallel()

	decoder, err := compilePathDecoderForTest(t, `
      parameters:
        - {name: id, in: path, required: true, schema: {anyOf: [{enum: [[1, 2]]}]}}
`)
	require.NoError(t, err)
	actual, err := decoder.DecodePathParams(&url.URL{Path: "/items/1,2"})
	require.NoError(t, err)
	require.JSONEq(t, `{"id":[1,2]}`, string(actual))
}

func TestCompilePathDecoderRejectsAnyOfWithUnrepresentableNestedSlots(t *testing.T) {
	t.Parallel()

	decoder, err := compilePathDecoderForTest(t, `
      parameters:
        - {name: id, in: path, required: true, schema: {anyOf: [{type: array, items: {type: object}}]}}
`)
	require.Nil(t, decoder)
	require.ErrorContains(t, err, "/anyOf/0/items")
	require.ErrorContains(t, err, "primitive type")
}

func TestCompilePathDecoderRejectsNestedAnyOfWithUnrepresentableAlternatives(t *testing.T) {
	t.Parallel()

	for _, schema := range []string{
		`{type: array, items: {anyOf: [{type: object}, {type: string}]}}`,
		`{type: object, properties: {value: {anyOf: [{type: object}, {type: string}]}}}`,
		`{type: object, additionalProperties: {anyOf: [{type: object}, {type: string}]}}`,
	} {
		decoder, err := compilePathDecoderForTest(t, `
      parameters:
        - {name: id, in: path, required: true, schema: `+schema+`}
`)
		require.Nil(t, decoder)
		require.Error(t, err)
		require.ErrorContains(t, err, "anyOf")
	}
}

func TestPathDecoderAnyOfReportsNoMatchingAlternative(t *testing.T) {
	t.Parallel()

	decoder, err := compilePathDecoderForTest(t, `
      parameters:
        - {name: id, in: path, required: true, schema: {anyOf: [{type: integer, minimum: 10}, {type: boolean}]}}
`)
	require.NoError(t, err)
	actual, err := decoder.DecodePathParams(&url.URL{Path: "/items/7"})
	require.Nil(t, actual)
	require.Error(t, err)
}

func TestPathDecoderAnyOfPreservesNestedChoiceOrderAndShape(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		schema   string
		path     string
		expected string
	}{
		{
			name: "anyOf inside allOf", schema: `{allOf: [{anyOf: [{type: integer}, {type: string}]}]}`,
			path: "/items/7", expected: `{"id":7}`,
		},
		{
			name: "nested anyOf", schema: `{anyOf: [{anyOf: [{type: integer}, {type: string}]}, {type: boolean}]}`,
			path: "/items/7", expected: `{"id":7}`,
		},
		{
			name: "array alternatives", schema: `{anyOf: [{type: array, items: {type: integer}}, {type: array, items: {type: string}}]}`,
			path: "/items/7,8", expected: `{"id":[7,8]}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := compilePathDecoderForTest(t, `
      parameters:
        - {name: id, in: path, required: true, schema: `+test.schema+`}
`)
			require.NoError(t, err)
			actual, err := decoder.DecodePathParams(&url.URL{Path: test.path})
			require.NoError(t, err)
			require.JSONEq(t, test.expected, string(actual))
		})
	}
}

func TestCompilePathDecoderLeavesJSONContentNullToValidation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
	}{
		{name: "direct", schema: `{type: string, nullable: true}`},
		{name: "reference", schema: `{$ref: '#/components/schemas/Nullable'}`},
		{name: "allOf", schema: `{allOf: [{type: string, nullable: true}]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := compilePathDecoderSpecForTest(t, pathJSONContentSpec(test.schema))
			require.NoError(t, err)

			actual, err := decoder.DecodePathParams(&url.URL{Path: "/items/null"})
			require.NoError(t, err)
			require.JSONEq(t, `{"id":null}`, string(actual))
		})
	}

	decoder, err := compilePathDecoderSpecForTest(t, pathJSONContentSpec(`{type: string}`))
	require.NoError(t, err)

	actual, err := decoder.DecodePathParams(&url.URL{Path: "/items/null"})
	require.Nil(t, actual)
	require.ErrorContains(t, err, "got null, want string")
}

func pathJSONContentSpec(schema string) string {
	return `openapi: 3.0.3
paths:
  /items/{id}:
    get:
      operationId: path
      parameters:
        - name: id
          in: path
          required: true
          content:
            application/json:
              schema: ` + schema + `
components:
  schemas:
    Nullable: {type: string, nullable: true}
`
}

func TestCompilePathDecoderRejectsPathFieldMisuseAndUnsupportedSerialization(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		parameter string
		contains  string
	}{
		{name: "allow empty present", parameter: `{name: id, in: path, required: true, allowEmptyValue: false, schema: {type: string}}`, contains: "allowEmptyValue"},
		{name: "allow reserved present", parameter: `{name: id, in: path, required: true, allowReserved: false, schema: {type: string}}`, contains: "allowReserved"},
		{name: "query-only style", parameter: `{name: id, in: path, required: true, style: form, schema: {type: string}}`, contains: `style "form" is unsupported`},
		{name: "non-JSON content", parameter: `{name: id, in: path, required: true, content: {text/plain: {schema: {type: string}}}}`, contains: "only application/json"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := compilePathDecoderForTest(t, "\n      parameters:\n        - "+test.parameter+"\n")
			require.Nil(t, decoder)
			require.ErrorContains(t, err, test.contains)
		})
	}
}

func TestCompilePathDecoderAppliesSemanticNullPolicyOnlyToSchemaStyle(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
	}{
		{name: "direct", schema: `{type: string, nullable: true}`},
		{name: "reference", schema: `{$ref: '#/components/schemas/Nullable'}`},
		{name: "allOf", schema: `{allOf: [{type: string, nullable: true}]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := compilePathDecoderSpecForTest(t, `openapi: 3.0.3
paths:
  /items/{id}:
    get:
      operationId: path
      parameters:
        - name: id
          in: path
          required: true
          schema: `+test.schema+`
components:
  schemas:
    Nullable: {type: string, nullable: true}
`)
			require.Nil(t, decoder)
			require.ErrorContains(t, err, `schema-style path parameter "id" accepts JSON null`)
		})
	}

	decoder, err := compilePathDecoderSpecForTest(t, `openapi: 3.0.3
paths:
  /items/{id}:
    get:
      operationId: path
      parameters:
        - name: id
          in: path
          required: true
          schema: {nullable: true, allOf: [{type: string}]}
`)
	require.NoError(t, err)

	actual, err := decoder.DecodePathParams(&url.URL{Path: "/items/value"})
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"value"}`, string(actual))
}

func TestCompilePathDecoderRejectsReachableBinaryFormats(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		parameter string
	}{
		{name: "root", parameter: `{name: id, in: path, required: true, schema: {type: string, format: binary}}`},
		{name: "array item", parameter: `{name: id, in: path, required: true, schema: {type: array, items: {allOf: [{type: string}, {format: binary}]}}}`},
		{name: "object property", parameter: `{name: id, in: path, required: true, schema: {type: object, properties: {value: {type: string, format: binary}}}}`},
		{name: "dynamic property", parameter: `{name: id, in: path, required: true, schema: {type: object, additionalProperties: {type: string, format: binary}}}`},
		{name: "JSON content", parameter: `{name: id, in: path, required: true, content: {application/json: {schema: {type: object, properties: {value: {type: string, format: binary}}}}}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := compilePathDecoderForTest(t, "\n      parameters:\n        - "+test.parameter+"\n")
			require.Nil(t, decoder)
			require.ErrorContains(t, err, `format "binary" is legal OpenAPI but unsupported by this tool`)
		})
	}
}

func TestCompilePathDecoderRejectsNestedStyleCompositesAndEmptyDeclaredKeys(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		schema   string
		contains string
	}{
		{name: "array item array", schema: `{type: array, items: {type: array, items: {type: string}}}`, contains: `unsupported compiled type "array"`},
		{name: "object property object", schema: `{type: object, properties: {value: {type: object}}}`, contains: `unsupported compiled type "object"`},
		{name: "dynamic object", schema: `{type: object, additionalProperties: {allOf: [{type: object}]}}`, contains: `unsupported compiled type "object"`},
		{name: "empty declared key", schema: `{type: object, properties: {'': {type: string}}}`, contains: "object property"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := compilePathDecoderForTest(t, "\n      parameters:\n        - name: id\n          in: path\n          required: true\n          schema: "+test.schema+"\n")
			require.Nil(t, decoder)
			require.ErrorContains(t, err, test.contains)
		})
	}
}

func TestCompilePathDecoderUsesStringDynamicValuesForEveryAdditionalPropertiesForm(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name                 string
		additionalProperties string
		decodeError          string
	}{
		{name: "omitted"},
		{name: "true", additionalProperties: `, additionalProperties: true`},
		{name: "empty schema", additionalProperties: `, additionalProperties: {}`},
		{name: "false", additionalProperties: `, additionalProperties: false`, decodeError: "additionalProperties"},
		{name: "unknown schema", additionalProperties: `, additionalProperties: {enum: [1, value]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := compilePathDecoderForTest(t, `
      parameters:
        - {name: id, in: path, required: true, explode: true,
           schema: {type: object`+test.additionalProperties+`}}
`)
			require.NoError(t, err)
			require.Equal(t, "string", decoder.Definition().Parameters[0].DynamicType)

			actual, err := decoder.DecodePathParams(&url.URL{Path: "/items/other=value"})
			if test.decodeError != "" {
				require.Nil(t, actual)
				require.ErrorContains(t, err, test.decodeError)

				return
			}

			require.NoError(t, err)
			require.JSONEq(t, `{"id":{"other":"value"}}`, string(actual))
		})
	}
}

func TestCompilePathDecoderMapsEverySchemaStyleAndShapeWire(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		style   string
		schema  string
		wire    pathWireKind
		explode bool
	}{
		{name: "simple primitive", schema: `{type: string}`, wire: pathWireSimplePrimitive},
		{name: "simple array", schema: `{type: array, items: {type: string}}`, wire: pathWireSimpleArray, explode: true},
		{name: "simple object", schema: `{type: object}`, wire: pathWireSimpleObject},
		{name: "label primitive", style: "label", schema: `{type: string}`, wire: pathWireLabelPrimitive, explode: true},
		{name: "label array", style: "label", schema: `{type: array, items: {type: string}}`, wire: pathWireLabelArray},
		{name: "label object", style: "label", schema: `{type: object}`, wire: pathWireLabelObject, explode: true},
		{name: "matrix primitive", style: "matrix", schema: `{type: string}`, wire: pathWireMatrixPrimitive},
		{name: "matrix array", style: "matrix", schema: `{type: array, items: {type: string}}`, wire: pathWireMatrixArray, explode: true},
		{name: "matrix object", style: "matrix", schema: `{type: object}`, wire: pathWireMatrixObject},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			style := ""
			if test.style != "" {
				style = ", style: " + test.style
			}

			decoder, err := compilePathDecoderForTest(t, fmt.Sprintf(`
      parameters:
        - {name: id, in: path, required: true%s, explode: %t, schema: %s}
`, style, test.explode, test.schema))
			require.NoError(t, err)
			require.Equal(t, uint8(test.wire), decoder.Definition().Parameters[0].Wire)
			require.Equal(t, test.explode, decoder.Definition().Parameters[0].Explode)
		})
	}
}

func compilePathDecoderForTest(t *testing.T, operationFields string) (*PathDecoder, error) {
	t.Helper()

	return compilePathDecoderSpecForTest(t, `openapi: 3.0.3
paths:
  /items/{id}:
    get:
      operationId: path
`+operationFields)
}

func compilePathDecoderSpecForTest(t *testing.T, spec string) (*PathDecoder, error) {
	t.Helper()

	sources, err := oas.Parse([]byte(spec))
	require.NoError(t, err)

	source := sources["path"]
	compiler := schemaCompiler{
		source: source, bySchema: make(map[string]*Validation), active: make(map[string]struct{}),
	}

	return compilePathDecoder("path", source, &compiler)
}
