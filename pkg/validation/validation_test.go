package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidationSupportedKeywordsAtRootNestedAndAllOf covers every runtime rule at each schema shape.
func TestValidationSupportedKeywordsAtRootNestedAndAllOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		valid   string
		invalid string
		keyword string
	}{
		{name: "type", schema: `{"type":"boolean"}`, valid: `true`, invalid: `0`, keyword: "type"},
		{name: "integer", schema: `{"type":"integer"}`, valid: `9007199254740993`, invalid: `1.5`, keyword: "type"},
		{name: "nullable", schema: `{"type":"string","nullable":true}`, valid: `null`, invalid: `1`, keyword: "type"},
		{name: "enum", schema: `{"enum":[1,{"a":2}]}`, valid: `1.0`, invalid: `2`, keyword: "enum"},
		{name: "minimum", schema: `{"minimum":1}`, valid: `1`, invalid: `0`, keyword: "minimum"},
		{
			name: "exclusiveMinimum", schema: `{"minimum":1,"exclusiveMinimum":true}`,
			valid: `2`, invalid: `1`, keyword: "exclusiveMinimum",
		},
		{name: "maximum", schema: `{"maximum":1}`, valid: `1`, invalid: `2`, keyword: "maximum"},
		{
			name: "exclusiveMaximum", schema: `{"maximum":1,"exclusiveMaximum":true}`,
			valid: `0`, invalid: `1`, keyword: "exclusiveMaximum",
		},
		{name: "multipleOf", schema: `{"multipleOf":0.1}`, valid: `0.3`, invalid: `0.31`, keyword: "multipleOf"},
		{name: "minLength", schema: `{"minLength":2}`, valid: `"λx"`, invalid: `"λ"`, keyword: "minLength"},
		{name: "maxLength", schema: `{"maxLength":1}`, valid: `"λ"`, invalid: `"λx"`, keyword: "maxLength"},
		{name: "pattern", schema: `{"pattern":"^a+$"}`, valid: `"aa"`, invalid: `"b"`, keyword: "pattern"},
		{name: "format", schema: `{"format":"date"}`, valid: `"2026-07-14"`, invalid: `"2026-02-30"`, keyword: "format"},
		{name: "minItems", schema: `{"minItems":1}`, valid: `[0]`, invalid: `[]`, keyword: "minItems"},
		{name: "maxItems", schema: `{"maxItems":1}`, valid: `[0]`, invalid: `[0,1]`, keyword: "maxItems"},
		{name: "items", schema: `{"items":{"type":"integer"}}`, valid: `[1]`, invalid: `[1.5]`, keyword: "type"},
		{name: "uniqueItems", schema: `{"uniqueItems":true}`, valid: `[1,2]`, invalid: `[1,1.0]`, keyword: "uniqueItems"},
		{name: "minProperties", schema: `{"minProperties":1}`, valid: `{"a":1}`, invalid: `{}`, keyword: "minProperties"},
		{
			name: "maxProperties", schema: `{"maxProperties":1}`,
			valid: `{"a":1}`, invalid: `{"a":1,"b":2}`, keyword: "maxProperties",
		},
		{name: "required", schema: `{"required":["a"]}`, valid: `{"a":1}`, invalid: `{}`, keyword: "required"},
		{
			name: "properties", schema: `{"properties":{"a":{"type":"string"}}}`,
			valid: `{"a":"x"}`, invalid: `{"a":1}`, keyword: "type",
		},
		{
			name: "additionalPropertiesFalse", schema: `{"additionalProperties":false}`,
			valid: `{}`, invalid: `{"a":1}`, keyword: "additionalProperties",
		},
		{
			name: "additionalPropertiesSchema", schema: `{"additionalProperties":{"type":"string"}}`,
			valid: `{"a":"x"}`, invalid: `{"a":1}`, keyword: "type",
		},
	}

	shapes := []struct {
		name        string
		wrapSchema  func(string) string
		wrapBody    func(string) string
		wantPointer string
	}{
		{name: "root", wrapSchema: identity, wrapBody: identity, wantPointer: "instance #"},
		{
			name: "nested",
			wrapSchema: func(schema string) string {
				return fmt.Sprintf(`{"type":"object","required":["value"],"properties":{"value":%s}}`, schema)
			},
			wrapBody:    func(body string) string { return fmt.Sprintf(`{"value":%s}`, body) },
			wantPointer: "instance #/value",
		},
		{
			name:        "allOf",
			wrapSchema:  func(schema string) string { return fmt.Sprintf(`{"allOf":[%s]}`, schema) },
			wrapBody:    identity,
			wantPointer: "schema #/paths/~1things/post/requestBody/content/application~1json/schema/allOf/0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for _, shape := range shapes {
				t.Run(shape.name, func(t *testing.T) {
					t.Parallel()

					parsed := mustParseSchema(t, shape.wrapSchema(test.schema), "")
					require.Empty(t, parsed.Validate(json.RawMessage(shape.wrapBody(test.valid))))

					errs := parsed.Validate(json.RawMessage(shape.wrapBody(test.invalid)))
					require.NotEmpty(t, errs)
					require.Contains(t, errors.Join(errs...).Error(), "keyword "+test.keyword)
					require.Contains(t, errors.Join(errs...).Error(), shape.wantPointer)
				})
			}
		})
	}
}

// TestParseExposesCompiledGraphAndCopiesInput covers the supported construction seam.
func TestParseExposesCompiledGraphAndCopiesInput(t *testing.T) {
	t.Parallel()

	spec := openAPISpec(`{
		"type":"object","required":["value"],
		"properties":{"value":{"type":"string","minLength":1}},
		"additionalProperties":false,
		"allOf":[{"maxProperties":1}]
	}`, "", true)
	parsedByOperation, _, err := Parse(spec)
	require.NoError(t, err)

	parsed := parsedByOperation["checkThing"]

	for index := range spec {
		spec[index] = ' '
	}

	require.True(t, parsed.BodyRequired)
	require.Equal(t, "object", parsed.KindValidation.Type)
	require.Equal(t, []string{"value"}, parsed.ObjectValidation.Required)
	require.Len(t, parsed.ObjectValidation.Properties, 1)
	require.Equal(t, "value", parsed.ObjectValidation.Properties[0].Name)
	require.Equal(t, "string", parsed.ObjectValidation.Properties[0].Validation.KindValidation.Type)
	require.False(t, parsed.ObjectValidation.AdditionalPropertiesAllowed)
	require.Len(t, parsed.AllOfValidations, 1)
	require.Empty(t, parsed.Validate(json.RawMessage(`{"value":"x"}`)))
}

// TestParseCompilesIndependentOperationGraphs verifies the document-wide map and per-operation compiler state.
func TestParseCompilesIndependentOperationGraphs(t *testing.T) {
	t.Parallel()

	spec := []byte(`{
		"openapi":"3.0.3",
		"paths":{
			"/required":{"post":{
				"operationId":"RequiredBody",
				"requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Body"}}}}
			}},
			"/optional":{"put":{
				"operationId":"optionalBody",
				"requestBody":{"content":{"application/*":{"schema":{"$ref":"#/components/schemas/Body"}}}}
			}},
			"/plain":{"post":{"requestBody":{"content":{"text/plain":{"schema":{"type":"string"}}}}}},
			"/bodyless":{"get":{}}
		},
		"components":{"schemas":{"Body":{"type":"string"}}}
	}`)

	parsed, _, err := Parse(spec)
	require.NoError(t, err)
	require.Equal(t, []string{"RequiredBody", "optionalBody"}, slices.Sorted(maps.Keys(parsed)))
	require.True(t, parsed["RequiredBody"].BodyRequired)
	require.False(t, parsed["optionalBody"].BodyRequired)
	require.NotSame(t, parsed["RequiredBody"], parsed["optionalBody"])
	require.Equal(t, "#/components/schemas/Body", parsed["RequiredBody"].SchemaPointer)
	require.Equal(t, "#/components/schemas/Body", parsed["optionalBody"].SchemaPointer)

	for index := range spec {
		spec[index] = ' '
	}

	require.Empty(t, parsed["RequiredBody"].Validate(json.RawMessage(`"still compiled"`)))
}

// TestParseCompilesOperationsInSortedIDOrder verifies deterministic atomic failure selection.
func TestParseCompilesOperationsInSortedIDOrder(t *testing.T) {
	t.Parallel()

	_, _, err := Parse([]byte(`{
		"openapi":"3.0.3",
		"paths":{
			"/first":{"post":{"operationId":"zulu","requestBody":{"content":{"application/json":{"schema":{"not":{}}}}}}},
			"/second":{"post":{"operationId":"alpha","requestBody":{"content":{"application/json":{"schema":{"oneOf":[{}]}}}}}}
		}
	}`))
	require.ErrorContains(t, err, `compile operationId "alpha"`)
	require.ErrorContains(t, err, "/oneOf")
}

// TestValidationStringFormats covers every agreed format and unknown-format fallback.
func TestValidationStringFormats(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		format  string
		valid   string
		invalid string
	}{
		{format: "byte", valid: `"YWJj"`, invalid: `"%%%"`},
		{format: "date", valid: `"2026-07-14"`, invalid: `"2026-02-30"`},
		{format: "date-time", valid: `"2026-07-14T12:30:00Z"`, invalid: `"2026-07-14"`},
		{format: "email", valid: `"a@example.com"`, invalid: `"not-an-email"`},
	} {
		t.Run(test.format, func(t *testing.T) {
			t.Parallel()

			parsed := mustParseSchema(t, fmt.Sprintf(`{"type":"string","format":%q}`, test.format), "")
			require.Empty(t, parsed.Validate(json.RawMessage(test.valid)))
			require.Contains(t, errors.Join(parsed.Validate(json.RawMessage(test.invalid))...).Error(), "keyword format")
		})
	}

	unknown := mustParseSchema(t, `{"type":"string","format":"vendor-string"}`, "")
	require.Empty(t, unknown.Validate(json.RawMessage(`"anything"`)))
}

// TestValidationStrictJSONAndBodyPresence covers transport-independent raw-body rules.
func TestValidationStrictJSONAndBodyPresence(t *testing.T) {
	t.Parallel()

	optional := mustParseSchema(t, `{}`, "")
	require.Nil(t, optional.Validate(nil))
	require.NotEmpty(t, optional.Validate(json.RawMessage("   ")))

	required := mustParseSchemaWithRequired(t, `{}`, "", true)
	require.Contains(t, errors.Join(required.Validate(nil)...).Error(), "required body is absent")
	require.Nil(t, required.Validate(json.RawMessage(`null`)))

	invalidBodies := []json.RawMessage{
		{0xff},
		json.RawMessage(`true false`),
		json.RawMessage(`{"a":1,"a":2}`),
		json.RawMessage(`{"a":{"b":1,"b":2}}`),
		json.RawMessage(`"\ud800"`),
		json.RawMessage(`"\udc00"`),
	}
	for _, body := range invalidBodies {
		require.NotEmpty(t, optional.Validate(body), "%q", body)
	}
}

// TestParseRejectsMalformedJSONRequestMediaAndSchemasAtPointers covers selected content shapes.
func TestParseRejectsMalformedJSONRequestMediaAndSchemasAtPointers(t *testing.T) {
	t.Parallel()

	const (
		mediaPointer  = "#/paths/~1things/post/requestBody/content/application~1json; charset=utf-8"
		schemaPointer = mediaPointer + "/schema"
	)

	tests := []struct {
		name       string
		mediaType  string
		pointer    string
		objectName string
	}{
		{name: "null media type", mediaType: `null`, pointer: mediaPointer, objectName: "Media Type Object"},
		{name: "scalar media type", mediaType: `1`, pointer: mediaPointer, objectName: "Media Type Object"},
		{name: "array media type", mediaType: `[]`, pointer: mediaPointer, objectName: "Media Type Object"},
		{name: "null schema", mediaType: `{"schema":null}`, pointer: schemaPointer, objectName: "Schema Object"},
		{name: "scalar schema", mediaType: `{"schema":1}`, pointer: schemaPointer, objectName: "Schema Object"},
		{name: "array schema", mediaType: `{"schema":[]}`, pointer: schemaPointer, objectName: "Schema Object"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			spec := fmt.Appendf(nil, `{
				"openapi":"3.0.3",
				"paths":{"/things":{"post":{
					"operationId":"checkThing",
					"requestBody":{"content":{"application/json; charset=utf-8":%s}}
				}}}
			}`, test.mediaType)
			_, _, err := Parse(spec)
			require.Error(t, err)
			require.ErrorContains(t, err, test.objectName)
			require.ErrorContains(t, err, test.pointer)
		})
	}
}

// TestSchemaLessJSONRequestBodyRuntimeSemantics covers JSON values, presence, and strict decoding.
func TestSchemaLessJSONRequestBodyRuntimeSemantics(t *testing.T) {
	t.Parallel()

	validations, _, err := Parse([]byte(`openapi: 3.0.3
paths:
  /optional:
    post:
      operationId: optionalBody
      requestBody:
        content:
          application/json: {}
  /required:
    post:
      operationId: requiredBody
      requestBody:
        required: true
        content:
          application/json: {}
  /defaulted-optional:
    post:
      operationId: defaultedOptionalBody
      requestBody:
        content:
          application/json:
            schema: {type: string, minLength: 2, default: fallback}
  /defaulted-required:
    post:
      operationId: defaultedRequiredBody
      requestBody:
        required: true
        content:
          application/json:
            schema: {type: string, default: fallback}
`))
	require.NoError(t, err)

	for _, body := range []json.RawMessage{
		json.RawMessage(`null`),
		json.RawMessage(`true`),
		json.RawMessage(`1.25`),
		json.RawMessage(`"value"`),
		json.RawMessage(`[1,true]`),
		json.RawMessage(`{"value":1}`),
		json.RawMessage(" \n\tnull\r "),
	} {
		require.Empty(t, validations["optionalBody"].Validate(body), "%q", body)
		require.Empty(t, validations["requiredBody"].Validate(body), "%q", body)
	}

	for _, absent := range []json.RawMessage{nil, {}} {
		require.Empty(t, validations["optionalBody"].Validate(absent))
		require.ErrorContains(t, errors.Join(validations["requiredBody"].Validate(absent)...), "required body is absent")
	}

	for _, invalid := range []json.RawMessage{
		json.RawMessage(" \n\t "),
		json.RawMessage(`{"value":`),
		json.RawMessage(`true false`),
	} {
		require.NotEmpty(t, validations["optionalBody"].Validate(invalid), "%q", invalid)
	}

	require.Empty(t, validations["defaultedOptionalBody"].Validate(nil))
	require.NotEmpty(t, validations["defaultedOptionalBody"].Validate(json.RawMessage(`"x"`)))
	require.Empty(t, validations["defaultedOptionalBody"].Validate(json.RawMessage(`"ok"`)))
	require.ErrorContains(
		t,
		errors.Join(validations["defaultedRequiredBody"].Validate(nil)...),
		"required body is absent",
	)
}

// TestSchemaLessJSONRequestBodyPreservesMediaSelection verifies specificity before schema compilation.
func TestSchemaLessJSONRequestBodyPreservesMediaSelection(t *testing.T) {
	t.Parallel()

	validations, _, err := Parse([]byte(`openapi: 3.0.3
paths:
  /exact:
    post:
      operationId: exact
      requestBody:
        content:
          '*/*': {schema: {type: number}}
          application/*: {schema: {type: boolean}}
          application/json: {}
  /application-wildcard:
    post:
      operationId: applicationWildcard
      requestBody:
        content:
          '*/*': {schema: {type: boolean}}
          application/*: {}
  /global-wildcard:
    post:
      operationId: globalWildcard
      requestBody:
        content:
          '*/*': {}
  /parameterized-exact:
    post:
      operationId: parameterizedExact
      requestBody:
        content:
          application/*: {schema: {type: boolean}}
          'application/json; charset=utf-8': {}
`))
	require.NoError(t, err)

	for _, operationID := range []string{"exact", "applicationWildcard", "globalWildcard", "parameterizedExact"} {
		require.Empty(t, validations[operationID].Validate(json.RawMessage(`"schema-less winner"`)))
	}
}

// TestValidationExactNumbers covers values beyond float64 and arbitrary exponent materialization.
func TestValidationExactNumbers(t *testing.T) {
	t.Parallel()

	parsed := mustParseSchema(t, `{
		"type":"number",
		"minimum":9007199254740993,
		"maximum":9007199254740993,
		"multipleOf":0.1
	}`, "")
	require.Empty(t, parsed.Validate(json.RawMessage(`9007199254740993`)))
	require.NotEmpty(t, parsed.Validate(json.RawMessage(`9007199254740992`)))
	require.NotEmpty(t, parsed.Validate(json.RawMessage(`9007199254740994`)))

	spelling := mustParseSchema(t, `{"minimum":1,"maximum":1}`, "")
	for _, body := range []json.RawMessage{json.RawMessage(`1`), json.RawMessage(`1.0`), json.RawMessage(`1e0`)} {
		require.Empty(t, spelling.Validate(body))
	}

	zero := mustParseSchema(t, `{"minimum":0,"maximum":0}`, "")
	require.Empty(t, zero.Validate(json.RawMessage(`-0`)))

	huge := mustParseSchema(t, `{"minimum":1e400,"maximum":1e400}`, "")
	require.Empty(t, huge.Validate(json.RawMessage(`1e400`)))
	require.NotEmpty(t, huge.Validate(json.RawMessage(`9e399`)))

	hugeExponent := mustParseSchema(t, `{"multipleOf":3e-100001}`, "")
	require.Empty(t, hugeExponent.Validate(json.RawMessage(`9e-100001`)))
	require.NotEmpty(t, hugeExponent.Validate(json.RawMessage(`1e-100001`)))

	integer := mustParseSchema(t, `{"type":"integer"}`, "")
	require.Empty(t, integer.Validate(json.RawMessage(`1e100001`)))
	require.NotEmpty(t, integer.Validate(json.RawMessage(`1e-100001`)))
}

// TestValidationNestedUniqueItemsAndAllOf covers finite nesting and composition behavior directly.
func TestValidationNestedUniqueItemsAndAllOf(t *testing.T) {
	t.Parallel()

	components := `,"components":{"schemas":{"Node":{"type":"object","required":["value"],"properties":{
		"value":{"type":"integer"},"child":{"$ref":"#/components/schemas/Child"}
	},"additionalProperties":false},"Child":{"type":"object","required":["value"],"properties":{
		"value":{"type":"integer"}
	},"additionalProperties":false}}}`
	nested := mustParseSchema(t, `{"$ref":"#/components/schemas/Node"}`, components)
	require.Empty(t, nested.Validate(json.RawMessage(`{"value":1,"child":{"value":2}}`)))
	errs := nested.Validate(json.RawMessage(`{"value":1,"child":{"value":2.5}}`))
	require.Contains(t, errors.Join(errs...).Error(), "instance #/child/value")

	unique := mustParseSchema(t, `{"type":"array","items":{},"uniqueItems":true}`, "")
	require.NotEmpty(t, unique.Validate(json.RawMessage(`[{"a":1},{"a":1.0}]`)))

	allOf := mustParseSchema(t, `{"allOf":[{"minimum":1},{"maximum":2}]}`, "")
	require.Empty(t, allOf.Validate(json.RawMessage(`1.5`)))
	errs = allOf.Validate(json.RawMessage(`3`))
	require.Contains(t, errors.Join(errs...).Error(), "/allOf/1")
}

// TestParseRejectsUnsupportedAndMalformedReachableSchemas covers every parse-time rejection.
func TestParseRejectsUnsupportedAndMalformedReachableSchemas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		components string
		want       string
	}{
		{name: "oneOf", schema: `{"oneOf":[{}]}`, want: "oneOf"},
		{name: "anyOf", schema: `{"anyOf":[{}]}`, want: "anyOf"},
		{name: "not", schema: `{"not":{}}`, want: "not"},
		{name: "externalRef", schema: `{"$ref":"other.yaml#/Thing"}`, want: "external reference"},
		{name: "unsupportedPattern", schema: `{"pattern":"x(?=a)"}`, want: "unsupported"},
		{name: "unknownKeyword", schema: `{"const":1}`, want: "unsupported Schema Object keyword"},
		{name: "malformedBound", schema: `{"minItems":-1}`, want: "minItems"},
		{name: "arrayWithoutItems", schema: `{"type":"array"}`, want: "items"},
		{name: "malformedMultiple", schema: `{"multipleOf":0}`, want: "greater than zero"},
		{name: "malformedRequired", schema: `{"required":["a","a"]}`, want: "unique strings"},
		{name: "wrongDefaultType", schema: `{"type":"string","default":1}`, want: "must conform to type"},
		{name: "fractionalIntegerDefault", schema: `{"type":"integer","default":1.5}`, want: "must conform to type"},
		{name: "externalDocsWithoutURL", schema: `{"externalDocs":{"description":"docs"}}`, want: "url is required"},
		{
			name: "externalDocsWrongDescription", schema: `{"externalDocs":{"url":"/docs","description":1}}`,
			want: "description",
		},
		{
			name: "externalDocsUnknownField", schema: `{"externalDocs":{"url":"/docs","other":true}}`,
			want: "unsupported field",
		},
		{name: "xmlWrongName", schema: `{"xml":{"name":1}}`, want: "name"},
		{name: "xmlRelativeNamespace", schema: `{"xml":{"namespace":"/relative"}}`, want: "absolute URI"},
		{name: "xmlUnknownField", schema: `{"xml":{"other":true}}`, want: "unsupported field"},
		{
			name:       "nestedUnsupported",
			schema:     `{"properties":{"value":{"$ref":"#/components/schemas/Bad"}}}`,
			components: `,"components":{"schemas":{"Bad":{"oneOf":[{}]}}}`,
			want:       "oneOf",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := Parse(openAPISpec(test.schema, test.components, false))
			require.ErrorContains(t, err, test.want)
		})
	}

	parsed, _, err := Parse([]byte(`{"openapi":"3.0.3","paths":{"/a":{"post":{"operationId":"unused"}}}}`))
	require.NoError(t, err)
	require.Empty(t, parsed)
}

// TestValidationAcceptsDiscriminatorAsAnInertHint verifies the intentional permissive
// placement deviation and ensures discriminator metadata does not affect validation.
func TestValidationAcceptsDiscriminatorAsAnInertHint(t *testing.T) {
	t.Parallel()

	for _, discriminator := range []string{
		`{"propertyName":"kind"}`,
		`{"propertyName":""}`,
		`{"propertyName":"kind","mapping":{"cat":"#/components/schemas/Cat"}}`,
	} {
		t.Run(discriminator, func(t *testing.T) {
			t.Parallel()

			validation := mustParseSchema(t, `{
				"type":"object",
				"properties":{"name":{"type":"string"}},
				"discriminator":`+discriminator+`
			}`, `,"components":{"schemas":{"Cat":{"oneOf":[{}]}}}`)
			require.Empty(t, validation.Validate(json.RawMessage(`{}`)))
			require.Empty(t, validation.Validate(json.RawMessage(`{"kind":"unknown"}`)))
		})
	}

	validation := mustParseSchema(t, `{
		"allOf":[{"type":"string","minLength":2}],
		"discriminator":{"propertyName":"kind"}
	}`, "")
	require.Empty(t, validation.Validate(json.RawMessage(`"ok"`)))
	require.NotEmpty(t, validation.Validate(json.RawMessage(`"x"`)))
}

// TestParseRejectsMalformedDiscriminatorAtExactPointers verifies discriminator
// metadata is shape-checked even though its placement and values are otherwise inert.
func TestParseRejectsMalformedDiscriminatorAtExactPointers(t *testing.T) {
	t.Parallel()

	const root = "#/paths/~1things/post/requestBody/content/application~1json/schema/discriminator"

	tests := []struct {
		name       string
		schema     string
		components string
		pointer    string
	}{
		{name: "null", schema: `{"discriminator":null}`, pointer: root},
		{name: "array", schema: `{"discriminator":[]}`, pointer: root},
		{name: "missing propertyName", schema: `{"discriminator":{}}`, pointer: root + "/propertyName"},
		{name: "null propertyName", schema: `{"discriminator":{"propertyName":null}}`, pointer: root + "/propertyName"},
		{name: "non-string propertyName", schema: `{"discriminator":{"propertyName":1}}`, pointer: root + "/propertyName"},
		{
			name: "null mapping", schema: `{"discriminator":{"propertyName":"kind","mapping":null}}`,
			pointer: root + "/mapping",
		},
		{
			name: "non-object mapping", schema: `{"discriminator":{"propertyName":"kind","mapping":[]}}`,
			pointer: root + "/mapping",
		},
		{
			name: "non-string mapping value", schema: `{"discriminator":{"propertyName":"kind","mapping":{"a/b~c":1}}}`,
			pointer: root + "/mapping/a~1b~0c",
		},
		{
			name: "resolved reference", schema: `{"$ref":"#/components/schemas/Bad"}`,
			components: `,"components":{"schemas":{"Bad":{"discriminator":{"propertyName":false}}}}`,
			pointer:    "#/components/schemas/Bad/discriminator/propertyName",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := Parse(openAPISpec(test.schema, test.components, false))
			require.Error(t, err)
			require.ErrorContains(t, err, "compile schema at "+test.pointer)
		})
	}
}

// TestParseAcceptsBooleanReadOnlyAndWriteOnlyAtTheRoot verifies annotation shape without property semantics.
func TestParseAcceptsBooleanReadOnlyAndWriteOnlyAtTheRoot(t *testing.T) {
	t.Parallel()

	for _, schema := range []string{
		`{"readOnly":false}`,
		`{"readOnly":true}`,
		`{"writeOnly":false}`,
		`{"writeOnly":true}`,
		`{"readOnly":true,"writeOnly":true}`,
		`{"allOf":[{"readOnly":true,"writeOnly":true}]}`,
	} {
		t.Run(schema, func(t *testing.T) {
			t.Parallel()

			mustParseSchema(t, schema, "")
		})
	}
}

// TestValidationKeepsRequiredWriteOnlyRequestPropertiesRequired verifies request requiredness is unchanged.
func TestValidationKeepsRequiredWriteOnlyRequestPropertiesRequired(t *testing.T) {
	t.Parallel()

	validation := mustParseSchema(t, `{
		"type":"object","required":["secret"],
		"properties":{"secret":{"type":"string","writeOnly":true}}
	}`, "")
	errs := validation.Validate(json.RawMessage(`{}`))
	require.NotEmpty(t, errs)
	require.Contains(t, errors.Join(errs...).Error(), "keyword required")
	require.Empty(t, validation.Validate(json.RawMessage(`{"secret":"kept"}`)))
}

// TestValidationDoesNotPropagateRequestDirectionAcrossAllOf verifies branch-local annotations remain inert.
func TestValidationDoesNotPropagateRequestDirectionAcrossAllOf(t *testing.T) {
	t.Parallel()

	validation := mustParseSchema(t, `{
		"type":"object","required":["value"],
		"properties":{"value":{"allOf":[
			{"type":"string","readOnly":true},
			{"minLength":2,"writeOnly":true}
		]}}
	}`, "")
	errs := validation.Validate(json.RawMessage(`{}`))
	require.NotEmpty(t, errs)
	require.Contains(t, errors.Join(errs...).Error(), "keyword required")
	require.Empty(t, validation.Validate(json.RawMessage(`{"value":"ok"}`)))
}

// TestParseRejectsMalformedReadOnlyAndWriteOnlyAtEverySchemaShape verifies annotation shape recursively.
func TestParseRejectsMalformedReadOnlyAndWriteOnlyAtEverySchemaShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		components string
		pointer    string
	}{
		{
			name: "root readOnly", schema: `{"readOnly":"yes"}`,
			pointer: "#/paths/~1things/post/requestBody/content/application~1json/schema/readOnly",
		},
		{
			name: "property writeOnly", schema: `{"properties":{"value":{"writeOnly":null}}}`,
			pointer: "#/paths/~1things/post/requestBody/content/application~1json/schema/properties/value/writeOnly",
		},
		{
			name: "resolved reference readOnly", schema: `{"$ref":"#/components/schemas/Value"}`,
			components: `,"components":{"schemas":{"Value":{"readOnly":[]}}}`,
			pointer:    "#/components/schemas/Value/readOnly",
		},
		{
			name: "allOf writeOnly", schema: `{"allOf":[{"writeOnly":1}]}`,
			pointer: "#/paths/~1things/post/requestBody/content/application~1json/schema/allOf/0/writeOnly",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := Parse(openAPISpec(test.schema, test.components, false))
			require.Error(t, err)
			require.ErrorContains(t, err, "compile schema at "+test.pointer)
			require.ErrorContains(t, err, "must be a boolean")
		})
	}
}

// TestParseRejectsReadOnlyAndWriteOnlyTogetherOnRequestProperties verifies property-only direction semantics.
func TestParseRejectsReadOnlyAndWriteOnlyTogetherOnRequestProperties(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		components string
		pointer    string
	}{
		{
			name:    "direct",
			schema:  `{"properties":{"value":{"readOnly":true,"writeOnly":true}}}`,
			pointer: "#/paths/~1things/post/requestBody/content/application~1json/schema/properties/value",
		},
		{
			name:       "resolved reference",
			schema:     `{"properties":{"value":{"$ref":"#/components/schemas/Value"}}}`,
			components: `,"components":{"schemas":{"Value":{"readOnly":true,"writeOnly":true}}}`,
			pointer:    "#/components/schemas/Value",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := Parse(openAPISpec(test.schema, test.components, false))
			require.Error(t, err)
			require.ErrorContains(t, err, "compile schema at "+test.pointer)
			require.ErrorContains(t, err, "readOnly and writeOnly must not both be true")
		})
	}
}

// TestValidationMakesRequiredReadOnlyRequestPropertiesOptional verifies omission and supplied-value validation.
func TestValidationMakesRequiredReadOnlyRequestPropertiesOptional(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		property   string
		components string
	}{
		{name: "direct", property: `{"type":"string","minLength":2,"readOnly":true}`},
		{
			name:       "resolved reference",
			property:   `{"$ref":"#/components/schemas/Identifier"}`,
			components: `,"components":{"schemas":{"Identifier":{"type":"string","minLength":2,"readOnly":true}}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			validation := mustParseSchema(t, `{"type":"object","required":["id"],"properties":{"id":`+
				test.property+`}}`, test.components)
			require.Empty(t, validation.Validate(json.RawMessage(`{}`)))
			require.Empty(t, validation.Validate(json.RawMessage(`{"id":"ok"}`)))

			errs := validation.Validate(json.RawMessage(`{"id":"x"}`))
			require.NotEmpty(t, errs)
			require.Contains(t, errors.Join(errs...).Error(), "keyword minLength")
		})
	}
}

// TestParseRejectsRecursiveSchemas makes finite, acyclic Parse results an explicit contract.
func TestParseRejectsRecursiveSchemas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		components string
	}{
		{
			name: "objectProperty",
			components: `,"components":{"schemas":{"Loop":{"type":"object","properties":{
				"child":{"$ref":"#/components/schemas/Loop"}
			}}}}`,
		},
		{
			name: "arrayItem",
			components: `,"components":{"schemas":{"Loop":{"type":"array",
				"items":{"$ref":"#/components/schemas/Loop"}
			}}}`,
		},
		{
			name: "allOf",
			components: `,"components":{"schemas":{"Loop":{"allOf":[
				{"$ref":"#/components/schemas/Loop"}
			]}}}`,
		},
		{
			name: "mutual",
			components: `,"components":{"schemas":{
				"Loop":{"type":"object","properties":{"other":{"$ref":"#/components/schemas/Other"}}},
				"Other":{"type":"object","properties":{"loop":{"$ref":"#/components/schemas/Loop"}}}
			}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := Parse(openAPISpec(`{"$ref":"#/components/schemas/Loop"}`, test.components, false))
			require.ErrorContains(t, err, `compile operationId "checkThing"`)
			require.ErrorContains(t, err, "recursive schema is unsupported")
			require.ErrorContains(t, err, "#/components/schemas/Loop")
		})
	}
}

// TestParseAcceptsWellFormedDocumentationFields guards the supported documentation-only shapes.
func TestParseAcceptsWellFormedDocumentationFields(t *testing.T) {
	t.Parallel()

	parsed := mustParseSchema(t, `{
		"type":"string",
		"nullable":true,
		"default":null,
		"title":"Thing",
		"description":"A thing",
		"deprecated":false,
		"xml":{
			"name":"thing",
			"namespace":"https://example.com/things",
			"prefix":"t",
			"attribute":false,
			"wrapped":true,
			"x-extra":1
		},
		"externalDocs":{
			"description":"More details",
			"url":"https://example.com/docs",
			"x-extra":1
		}
	}`, "")

	require.Empty(t, parsed.Validate(json.RawMessage(`null`)))
}

// TestParseAcceptsArbitrarilyLargeCollectionBounds covers the unbounded OAS integer domain.
func TestParseAcceptsArbitrarilyLargeCollectionBounds(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		keyword    string
		body       string
		wantErrors bool
	}{
		{name: "minLength", keyword: "minLength", body: `"x"`, wantErrors: true},
		{name: "maxLength", keyword: "maxLength", body: `"x"`},
		{name: "minItems", keyword: "minItems", body: `[]`, wantErrors: true},
		{name: "maxItems", keyword: "maxItems", body: `[]`},
		{name: "minProperties", keyword: "minProperties", body: `{}`, wantErrors: true},
		{name: "maxProperties", keyword: "maxProperties", body: `{}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parsed := mustParseSchema(t, fmt.Sprintf(`{%q:1e100001}`, test.keyword), "")

			errs := parsed.Validate(json.RawMessage(test.body))
			if test.wantErrors {
				require.Contains(t, errors.Join(errs...).Error(), "keyword "+test.keyword)
			} else {
				require.Empty(t, errs)
			}
		})
	}
}

// TestParseAcceptsCompatibleOpenAPIVersions verifies patch-level compatibility.
func TestParseAcceptsCompatibleOpenAPIVersions(t *testing.T) {
	t.Parallel()

	valid := openAPISpec(`{}`, "", false)

	for _, version := range []string{
		"3.0.0",
		"3.0.4",
		"3.0.10",
		"3.0.4-rc.1",
		"3.0.4+vendor",
		"3.0.4-rc.1+vendor",
	} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			spec := strings.Replace(string(valid), `"3.0.3"`, strconv.Quote(version), 1)
			_, _, err := Parse([]byte(spec))
			require.NoError(t, err)
		})
	}
}

// TestParseRejectsUnsupportedOpenAPIVersions enforces this package's feature-set contract.
func TestParseRejectsUnsupportedOpenAPIVersions(t *testing.T) {
	t.Parallel()

	valid := openAPISpec(`{}`, "", false)

	for _, test := range []struct {
		name        string
		replacement string
		wantError   string
	}{
		{name: "leading zero major", replacement: `"03.0.4"`, wantError: "Semantic Versioning 2.0.0"},
		{name: "leading zero minor", replacement: `"3.00.4"`, wantError: "Semantic Versioning 2.0.0"},
		{name: "leading zero", replacement: `"3.0.04"`, wantError: "Semantic Versioning 2.0.0"},
		{name: "missing patch", replacement: `"3.0"`, wantError: "Semantic Versioning 2.0.0"},
		{name: "leading version marker", replacement: `"v3.0.4"`, wantError: "Semantic Versioning 2.0.0"},
		{name: "leading zero prerelease", replacement: `"3.0.4-01"`, wantError: "Semantic Versioning 2.0.0"},
		{name: "empty prerelease", replacement: `"3.0.4-"`, wantError: "Semantic Versioning 2.0.0"},
		{name: "empty build", replacement: `"3.0.4+"`, wantError: "Semantic Versioning 2.0.0"},
		{name: "unsupported feature set", replacement: `"3.1.0"`, wantError: "feature set must be 3.0"},
		{name: "number", replacement: `3.0`, wantError: "Semantic Versioning 2.0.0"},
		{name: "null", replacement: `null`, wantError: "Semantic Versioning 2.0.0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			spec := strings.Replace(string(valid), `"3.0.3"`, test.replacement, 1)
			_, _, err := Parse([]byte(spec))
			require.ErrorContains(t, err, test.wantError)
		})
	}

	missing := strings.Replace(string(valid), `"openapi":"3.0.3",`, "", 1)
	_, _, err := Parse([]byte(missing))
	require.ErrorContains(t, err, "Semantic Versioning 2.0.0")
}

// TestParsePreservesOpenAPIVersionDecodeError keeps invalid field-type context available to callers.
func TestParsePreservesOpenAPIVersionDecodeError(t *testing.T) {
	t.Parallel()

	valid := openAPISpec(`{}`, "", false)
	spec := strings.Replace(string(valid), `"3.0.3"`, `3.0`, 1)

	_, _, err := Parse([]byte(spec))

	var typeError *json.UnmarshalTypeError
	require.ErrorAs(t, err, &typeError)
}

// TestParseRejectsFirstMalformedOperationDeterministically verifies the whole-document error boundary.
func TestParseRejectsFirstMalformedOperationDeterministically(t *testing.T) {
	t.Parallel()

	spec := []byte(`{
		"openapi":"3.0.3",
		"paths":{
			"/broken-ref":{"$ref":"#/components/pathItems/Missing"},
			"/broken-operation":{"post":false},
			"/things":{"post":{
				"operationId":"checkThing",
				"requestBody":{"content":{"application/json":{"schema":{"type":"string"}}}}
			}}
		}
	}`)

	_, _, err := Parse(spec)
	require.ErrorContains(t, err, "#/paths/~1broken-operation/post")
}

// TestValidationErrorsAreStableAndFresh covers repeatability and caller-owned result slices.
func TestValidationErrorsAreStableAndFresh(t *testing.T) {
	t.Parallel()

	parsed := mustParseSchema(t, `{"type":"array","minItems":2,"items":{"type":"integer"}}`, "")
	body := json.RawMessage(`[1.5]`)
	first := parsed.Validate(body)
	second := parsed.Validate(body)
	require.Equal(t, errorStrings(first), errorStrings(second))
	require.Len(t, first, 2)
	first[0] = errors.New("caller mutation")

	require.Equal(t, errorStrings(second), errorStrings(parsed.Validate(body)))
}

// TestValidationConcurrentHighContention proves one parsed graph is immutable across concurrent calls.
//
//nolint:cyclop // The required barrier, mutation probe, and buffered mismatch reporting are one concurrency scenario.
func TestValidationConcurrentHighContention(t *testing.T) {
	t.Parallel()

	components := `,"components":{"schemas":{"Node":{"type":"object","required":["name","amount","children"],"properties":{
		"name":{"type":"string","pattern":"^[a-z]+$"},
		"amount":{"type":"integer","minimum":9007199254740993,"multipleOf":3},
		"children":{"type":"array","items":{"$ref":"#/components/schemas/Child"}}
	},"additionalProperties":false,"allOf":[{"minProperties":3}]},
	"Child":{"type":"object","required":["name","amount","children"],"properties":{
		"name":{"type":"string","pattern":"^[a-z]+$"},
		"amount":{"type":"integer","minimum":9007199254740993,"multipleOf":3},
		"children":{"type":"array","items":{"type":"string"}}
	},"additionalProperties":false,"allOf":[{"minProperties":3}]}}}`
	parsed := mustParseSchema(t, `{"$ref":"#/components/schemas/Node"}`, components)

	bodies := []json.RawMessage{
		json.RawMessage(`{"name":"root","amount":9007199254740993,"children":[]}`),
		json.RawMessage(`{"name":"BAD","amount":9007199254740992,"children":[]}`),
		json.RawMessage(
			`{"name":"root","amount":9007199254740993,"children":` +
				`[{"name":"child","amount":9007199254740996,"children":[]}]}`,
		),
		json.RawMessage(`{"name":"root","amount":9007199254740994,"children":[],"extra":true}`),
	}

	expected := make([][]string, len(bodies))
	for index, body := range bodies {
		expected[index] = errorStrings(parsed.Validate(body))
	}

	const (
		goroutineCount    = 256
		callsPerGoroutine = 250
	)

	start := make(chan struct{})
	mismatches := make(chan string, goroutineCount)

	var waitGroup sync.WaitGroup
	waitGroup.Add(goroutineCount)

	for worker := range goroutineCount {
		go func() {
			defer waitGroup.Done()

			<-start

			for iteration := range callsPerGoroutine {
				bodyIndex := (worker + iteration) % len(bodies)

				errs := parsed.Validate(bodies[bodyIndex])
				if got := errorStrings(errs); !equalStrings(got, expected[bodyIndex]) {
					select {
					case mismatches <- fmt.Sprintf(
						"worker %d iteration %d: got %v want %v", worker, iteration, got, expected[bodyIndex],
					):
					default:
					}
				}

				for index := range errs {
					errs[index] = errors.New("caller mutation")
				}

				if got := errorStrings(parsed.Validate(bodies[bodyIndex])); !equalStrings(got, expected[bodyIndex]) {
					select {
					case mismatches <- fmt.Sprintf(
						"worker %d iteration %d after mutation: got %v want %v",
						worker, iteration, got, expected[bodyIndex],
					):
					default:
					}
				}
			}
		}()
	}

	close(start)
	waitGroup.Wait()
	close(mismatches)

	for mismatch := range mismatches {
		t.Error(mismatch)
	}
}

// identity returns its input unchanged.
func identity(value string) string {
	return value
}

// mustParseSchema builds one optional-body OpenAPI fixture and requires parse success.
func mustParseSchema(t *testing.T, schema string, components string) *Validation {
	t.Helper()

	return mustParseSchemaWithRequired(t, schema, components, false)
}

// mustParseSchemaWithRequired builds one OpenAPI fixture and requires parse success.
func mustParseSchemaWithRequired(t *testing.T, schema string, components string, required bool) *Validation {
	t.Helper()

	parsedByOperation, _, err := Parse(openAPISpec(schema, components, required))
	require.NoError(t, err)

	return parsedByOperation["checkThing"]
}

// openAPISpec embeds one JSON Schema Object into one selected OpenAPI operation.
func openAPISpec(schema string, components string, required bool) []byte {
	return fmt.Appendf(nil, `{
		"openapi":"3.0.3",
		"info":{"title":"test","version":"1"},
		"paths":{"/things":{"post":{
			"operationId":"checkThing",
			"requestBody":{"required":%t,"content":{"application/json":{"schema":%s}}},
			"responses":{"204":{"description":"ok"}}
		}}}%s
	}`, required, schema, components)
}

// errorStrings copies an error sequence into comparable strings.
func errorStrings(errs []error) []string {
	if len(errs) == 0 {
		return nil
	}

	result := make([]string, len(errs))
	for index, err := range errs {
		result[index] = err.Error()
	}

	return result
}

// equalStrings compares two ordered string slices.
func equalStrings(left []string, right []string) bool {
	return strings.Join(left, "\x00") == strings.Join(right, "\x00")
}
