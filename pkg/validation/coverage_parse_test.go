//nolint:godoclint,lll // Internal white-box coverage matrices use compact malformed-state literals.
package validation

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/stretchr/testify/require"
)

func TestSchemaAdmissionFailureMatrix(t *testing.T) {
	t.Parallel()

	schemas := []string{
		`{"type":1}`,
		`{"type":"unknown"}`,
		`{"type":"string","nullable":null}`,
		`{"title":1}`,
		`{"description":null}`,
		`{"readOnly":1}`,
		`{"writeOnly":null}`,
		`{"deprecated":"yes"}`,
		`{"type":"string","default":1}`,
		`{"xml":null}`,
		`{"xml":{"name":1}}`,
		`{"xml":{"namespace":"relative"}}`,
		`{"xml":{"attribute":1}}`,
		`{"xml":{"unknown":true}}`,
		`{"externalDocs":null}`,
		`{"externalDocs":{}}`,
		`{"externalDocs":{"url":""}}`,
		`{"externalDocs":{"url":1}}`,
		`{"externalDocs":{"url":"%"}}`,
		`{"externalDocs":{"url":"ok","unknown":true}}`,
		`{"discriminator":null}`,
		`{"discriminator":{}}`,
		`{"discriminator":{"propertyName":1}}`,
		`{"discriminator":{"propertyName":"kind","mapping":null}}`,
		`{"discriminator":{"propertyName":"kind","mapping":{"a/b~c":1}}}`,
		`{"enum":null}`,
		`{"enum":[]}`,
		`{"minimum":null}`,
		`{"maximum":"x"}`,
		`{"exclusiveMinimum":null}`,
		`{"exclusiveMaximum":1}`,
		`{"multipleOf":null}`,
		`{"multipleOf":0}`,
		`{"minLength":null}`,
		`{"maxLength":1.5}`,
		`{"pattern":1}`,
		`{"pattern":"["}`,
		`{"format":1}`,
		`{"type":"string","format":"int32"}`,
		`{"type":"integer","format":"float"}`,
		`{"type":"number","format":"password"}`,
		`{"type":"number","format":"email"}`,
		`{"format":"legal-but-unsupported"}`,
		`{"minItems":null}`,
		`{"maxItems":-1}`,
		`{"type":"array"}`,
		`{"items":null}`,
		`{"minProperties":null}`,
		`{"maxProperties":-1}`,
		`{"required":null}`,
		`{"required":[]}`,
		`{"required":["a","a"]}`,
		`{"properties":null}`,
		`{"properties":{"a":null}}`,
		`{"additionalProperties":null}`,
		`{"properties":{"a":{"readOnly":null}}}`,
		`{"properties":{"a":{"writeOnly":null}}}`,
		`{"properties":{"a":{"readOnly":true,"writeOnly":true}}}`,
		`{"allOf":null}`,
		`{"allOf":[]}`,
		`{"allOf":[null]}`,
		`{"anyOf":null}`,
		`{"anyOf":[]}`,
		`{"anyOf":[null]}`,
	}

	for _, schema := range schemas {
		_, err := Parse(openAPISpec(schema, "", false))
		require.Error(t, err, schema)
	}
}

func TestParseParameterAndOptionFailureMatrix(t *testing.T) {
	t.Parallel()

	parameters := []struct {
		definition string
		valid      bool
	}{
		{definition: `{"name":"Accept","in":"header","schema":{}}`, valid: true},
		{definition: `{"name":1,"in":"header","schema":{}}`},
		{definition: `{"name":"x","in":"header","content":{"application/json":null}}`},
		{definition: `{"name":"x","in":"header","schema":{"type":"unknown"}}`},
		{definition: `{"name":"x","in":"header","schema":{},"style":1}`},
		{definition: `{"name":"x","in":"header","schema":{},"style":"form"}`},
		{definition: `{"name":"x","in":"header","schema":{},"allowReserved":false}`},
		{definition: `{"name":"x","in":"cookie","schema":{},"style":"simple"}`},
	}
	for _, parameter := range parameters {
		spec := []byte(`{"openapi":"3.0.3","paths":{"/x":{"get":{"operationId":"x","parameters":[` + parameter.definition + `]}}}}`)

		_, err := Parse(spec)
		if parameter.valid {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}

	option := PatternOptions(nil)
	require.Error(t, option(new(patternvalidator.PatternValidation)))

	custom := errors.New("custom")
	option = PatternOptions(func(*patternvalidator.PatternValidation) error { return custom })
	require.ErrorIs(t, option(new(patternvalidator.PatternValidation)), custom)

	_, err := compileStringFormat("unknown")
	require.Error(t, err)
	require.PanicsWithError(t, err.Error(), func() { MustCompileStringFormat("unknown") })
}

func TestQueryAndPathParameterAdmissionFailureMatrix(t *testing.T) {
	t.Parallel()

	for _, parameter := range []string{
		`{"name":"q","in":"query","required":"yes","schema":{}}`,
		`{"name":"q","in":"query","allowReserved":"yes","schema":{}}`,
		`{"name":"q","in":"query","schema":{},"content":{"application/json":{}}}`,
		`{"name":"q","in":"query","content":1}`,
		`{"name":"q","in":"query","content":null}`,
		`{"name":"q","in":"query","content":{"application/json":{},"text/json":{}}}`,
		`{"name":"q","in":"query","content":{"application/json; charset":{}}}`,
		`{"name":"q","in":"query","content":{"application/json":1}}`,
		`{"name":"q","in":"query","content":{"application/json":null}}`,
		`{"name":"q","in":"query","content":{"application/json":{}},"style":"form"}`,
		`{"name":"q","in":"query","content":{"application/json":{}},"explode":true}`,
		`{"name":"q","in":"query","content":{"application/json":{}},"allowReserved":true}`,
		`{"name":"q","in":"query","schema":{"type":"string"},"style":1}`,
		`{"name":"q","in":"query","schema":{"type":"string"},"explode":1}`,
		`{"name":"q","in":"query","schema":{"type":"string"},"style":"deepObject"}`,
		`{"name":"q","in":"query","schema":{"type":"array","items":{"type":"string"}},"style":"spaceDelimited","explode":true}`,
		`{"name":"q","in":"query","schema":{"type":"object"},"style":"deepObject","explode":false}`,
	} {
		spec := []byte(`{"openapi":"3.0.3","paths":{"/x":{"get":{"operationId":"query","parameters":[` + parameter + `]}}}}`)
		_, err := Parse(spec)
		require.Error(t, err, parameter)
	}

	for _, parameter := range []string{
		`{"name":1,"in":"path","required":true,"schema":{}}`,
		`{"name":"a/b","in":"path","required":true,"schema":{}}`,
		`{"name":"p","in":"path","required":false,"schema":{}}`,
		`{"name":"p","in":"path","required":true,"schema":{},"content":{"application/json":{}}}`,
		`{"name":"p","in":"path","required":true,"schema":{"type":"string","format":"binary"}}`,
		`{"name":"p","in":"path","required":true,"content":1}`,
		`{"name":"p","in":"path","required":true,"content":{"application/json":1}}`,
		`{"name":"p","in":"path","required":true,"content":{"application/json":{}},"style":"simple"}`,
		`{"name":"p","in":"path","required":true,"schema":{"type":"string"},"style":1}`,
		`{"name":"p","in":"path","required":true,"schema":{"type":"string"},"explode":1}`,
	} {
		spec := []byte(`{"openapi":"3.0.3","paths":{"/{p}":{"get":{"operationId":"path","parameters":[` + parameter + `]}}}}`)
		_, err := Parse(spec)
		require.Error(t, err, parameter)
	}
}

func TestOperationDecoderAggregationFailures(t *testing.T) {
	t.Parallel()

	calls := 0
	_, err := Parse([]byte(`openapi: 3.0.3
paths:
  /{p}:
    get:
      operationId: path
      parameters:
        - name: p
          in: path
          required: true
          schema: {type: string, pattern: a}
`), func(*patternvalidator.PatternValidation) error {
		calls++
		if calls > 2 {
			return errors.New("second compilation failed")
		}

		return nil
	})
	require.Error(t, err)

	_, err = Parse([]byte(`openapi: 3.0.3
paths:
  /x:
    get:
      operationId: query
      parameters:
        - name: object
          in: query
          schema:
            type: object
            additionalProperties: false
            properties: {x: {type: string}}
        - {name: x, in: query, schema: {type: string}}
`))
	require.Error(t, err)
}

func TestPrimitiveKeywordDecoderFailures(t *testing.T) {
	t.Parallel()

	for _, raw := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`"x"`)} {
		_, err := decodeNumber(raw, "number")
		require.Error(t, err)
		_, err = decodeBoolean(raw, "boolean")
		require.Error(t, err)
	}

	for _, raw := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`1`)} {
		_, err := decodeString(raw, "string")
		require.Error(t, err)
	}

	_, err := decodeOptionalNumber(map[string]json.RawMessage{"number": json.RawMessage(`null`)}, "number")
	require.Error(t, err)
	_, err = decodeOptionalNonNegativeInteger(
		map[string]json.RawMessage{"count": json.RawMessage(`1.5`)},
		"count",
	)
	require.Error(t, err)

	_, err = schemaMembers(oas.LocatedSchema{Raw: json.RawMessage(`null`), Pointer: "#/schema"})
	require.Error(t, err)
	_, err = schemaMembers(oas.LocatedSchema{Raw: json.RawMessage(`[`), Pointer: "#/schema"})
	require.Error(t, err)
}
