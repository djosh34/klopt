//nolint:godoclint,lll // Internal white-box coverage matrices use compact malformed-state literals.
package validation

import (
	"encoding/json"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/stretchr/testify/require"
)

func testCompiler(document string) *schemaCompiler {
	return &schemaCompiler{
		source:   oas.Source{Document: json.RawMessage(document)},
		bySchema: make(map[string]*Validation),
		active:   make(map[string]struct{}),
	}
}

func TestRawParameterCompilerFailureMatrix(t *testing.T) {
	t.Parallel()

	compiler := testCompiler(`{}`)
	for _, parameter := range []oas.LocatedSchema{
		{Raw: json.RawMessage(`1`), Pointer: "#/parameter"},
		{Raw: json.RawMessage(`{"name":"p","in":1,"schema":{}}`), Pointer: "#/parameter"},
		{Raw: json.RawMessage(`{"name":"p","in":"unknown","schema":{}}`), Pointer: "#/parameter"},
		{Raw: json.RawMessage(`{"name":1,"in":"header","schema":{}}`), Pointer: "#/parameter"},
	} {
		require.Error(t, validateRawParameter(parameter, compiler))
	}

	_, err := isReservedHeaderParameter(map[string]json.RawMessage{"name": json.RawMessage(`1`)})
	require.Error(t, err)

	parameter := oas.LocatedSchema{Raw: json.RawMessage(`{}`), Pointer: "#/parameter"}
	for _, members := range []map[string]json.RawMessage{
		{"name": json.RawMessage(`1`)},
		{"name": json.RawMessage(`"p"`), "content": json.RawMessage(`1`)},
		{"name": json.RawMessage(`"p"`), "schema": json.RawMessage(`{"type":"bad"}`)},
		{"name": json.RawMessage(`"p"`), "schema": json.RawMessage(`{}`), "in": json.RawMessage(`1`)},
	} {
		require.Error(t, compileIgnoredParameterSchema(parameter, members, compiler))
	}

	for _, members := range []map[string]json.RawMessage{
		{"content": json.RawMessage(`1`)},
		{"content": json.RawMessage(`{"application/json":1}`)},
	} {
		_, err = ignoredParameterSchema(parameter, members)
		require.Error(t, err)
	}

	require.Error(t, validateIgnoredParameterSerialization(
		"p", "header", map[string]json.RawMessage{"style": json.RawMessage(`1`)},
	))
}

func TestDirectQueryParameterCompilerFailureMatrix(t *testing.T) {
	t.Parallel()

	compiler := testCompiler(`{}`)
	for _, raw := range []string{
		`1`,
		`{"name":1,"in":"query","schema":{}}`,
		`{"name":"q","in":"query","required":"yes","schema":{}}`,
		`{"name":"q","in":"query","allowEmptyValue":"yes","schema":{}}`,
		`{"name":"q","in":"query","allowReserved":"yes","schema":{}}`,
		`{"name":"q","in":"query","schema":{},"content":{}}`,
		`{"name":"q","in":"query","content":1}`,
		`{"name":"q","in":"query","content":null}`,
		`{"name":"q","in":"query","content":{}}`,
		`{"name":"q","in":"query","content":{"bad":{}}}`,
		`{"name":"q","in":"query","content":{"application/json; charset":{}}}`,
		`{"name":"q","in":"query","content":{"application/json":1}}`,
		`{"name":"q","in":"query","content":{"application/json":null}}`,
		`{"name":"q","in":"query","content":{"application/json":{}},"allowReserved":true}`,
		`{"name":"q","in":"query","content":{"application/json":{}},"style":"form"}`,
		`{"name":"q","in":"query","content":{"application/json":{}},"explode":true}`,
		`{"name":"q","in":"query","schema":{"type":"string"},"style":1}`,
		`{"name":"q","in":"query","schema":{"type":"string"},"explode":1}`,
		`{"name":"q","in":"query","schema":{"type":"string"},"style":"invalid"}`,
		`{"name":"q","in":"query","schema":{"anyOf":[{"type":"string"}]},"style":"spaceDelimited"}`,
	} {
		_, err := compileQueryParameter(oas.LocatedSchema{Raw: json.RawMessage(raw), Pointer: "#/parameter"}, compiler)
		require.Error(t, err, raw)
	}
}

func TestDirectQueryCompiledStateFailures(t *testing.T) {
	t.Parallel()

	compiler := testCompiler(`{}`)
	_, err := compileQueryParameter(oas.LocatedSchema{
		Raw:     json.RawMessage(`{"name":"q","in":"query","schema":{"anyOf":[{"type":"string"}]},"style":"spaceDelimited"}`),
		Pointer: "#/anyof-parameter",
	}, compiler)
	require.Error(t, err)

	malformed := &Validation{EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{{
		Kind:   jsonvalue.KindNumber,
		Number: jsonvalue.Number{Lexeme: "invalid"},
	}}}}
	compiler.bySchema["#/cached/schema"] = malformed
	_, err = compileQueryParameter(oas.LocatedSchema{
		Raw:     json.RawMessage(`{"name":"q","in":"query","schema":{}}`),
		Pointer: "#/cached",
	}, compiler)
	require.Error(t, err)
}

func TestDirectPathParameterCompilerFailureMatrix(t *testing.T) {
	t.Parallel()

	compiler := testCompiler(`{}`)
	for _, raw := range []string{
		`1`,
		`{"name":1,"in":"path","required":true,"schema":{}}`,
		`{"name":"p","in":"path","required":1,"schema":{}}`,
		`{"name":"p","in":"path","required":true}`,
		`{"name":"p","in":"path","required":true,"schema":{},"content":{}}`,
		`{"name":"p","in":"path","required":true,"schema":null}`,
		`{"name":"p","in":"path","required":true,"schema":{"type":"string"},"style":1}`,
		`{"name":"p","in":"path","required":true,"schema":{"type":"string"},"explode":1}`,
		`{"name":"p","in":"path","required":true,"schema":{"type":"string"},"style":"invalid"}`,
		`{"name":"p","in":"path","required":true,"content":1}`,
		`{"name":"p","in":"path","required":true,"content":{"application/json":1}}`,
		`{"name":"p","in":"path","required":true,"content":{"application/json":{}},"style":"simple"}`,
	} {
		_, err := compilePathParameter(oas.LocatedSchema{Raw: json.RawMessage(raw), Pointer: "#/parameter"}, compiler)
		require.Error(t, err, raw)
	}
}

func TestDirectPathMetadataFailureMatrix(t *testing.T) {
	t.Parallel()

	invalidNumber := jsonvalue.Value{
		Kind:   jsonvalue.KindNumber,
		Number: jsonvalue.Number{Lexeme: "invalid"},
	}
	for _, validation := range []*Validation{
		{EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{invalidNumber}}},
		{KindValidation: KindValidation{Type: "array"}, ArrayValidation: ArrayValidation{Items: &Validation{EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{invalidNumber}}}}},
		{KindValidation: KindValidation{Type: "object"}, ObjectValidation: ObjectValidation{Properties: []PropertyValidation{{Name: "p", Validation: &Validation{EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{invalidNumber}}}}}}},
	} {
		_, err := compileSchemaPathMetadata("p", pathWireSimplePrimitive, false, validation)
		require.Error(t, err)
	}

	compiler := testCompiler(`{}`)
	located := oas.LocatedSchema{Raw: json.RawMessage(`{"content":{"application/json":{"schema":{}}}}`), Pointer: "#/parameter"}
	_, err := compileJSONPathParameter(
		"p",
		located,
		map[string]json.RawMessage{
			"content":         json.RawMessage(`{"application/json":{"schema":{}}}`),
			"allowEmptyValue": json.RawMessage(`false`),
		},
		compiler,
	)
	require.Error(t, err)
}

func TestSchemaCompilerMismatchedRawChildrenReturnErrors(t *testing.T) {
	t.Parallel()

	compiler := testCompiler(`{}`)
	schema := oas.LocatedSchema{Raw: json.RawMessage(`{}`), Pointer: "#/schema"}

	validation := new(Validation)
	require.Error(t, compiler.compileArray(validation, schema, map[string]json.RawMessage{
		"items": json.RawMessage(`{}`),
	}))
	require.Error(t, compiler.compileObject(validation, schema, map[string]json.RawMessage{
		"additionalProperties": json.RawMessage(`{}`),
	}))
	require.Error(t, compiler.compileObjectProperties(validation, schema, map[string]json.RawMessage{
		"properties": json.RawMessage(`{"p":{}}`),
	}))
	_, err := compiler.compileSchemaArray(schema, map[string]json.RawMessage{
		"allOf": json.RawMessage(`[{}]`),
	}, "allOf")
	require.Error(t, err)

	_, err = compiler.requestPropertyReadOnly(oas.LocatedSchema{
		Raw: json.RawMessage(`{"$ref":"#/missing"}`), Pointer: "#/property",
	})
	require.Error(t, err)
	_, err = compiler.requestPropertyReadOnly(oas.LocatedSchema{Raw: json.RawMessage(`null`), Pointer: "#/property"})
	require.Error(t, err)
	_, err = compiler.requestPropertyReadOnly(oas.LocatedSchema{Raw: json.RawMessage(`{"readOnly":1}`), Pointer: "#/property"})
	require.Error(t, err)
	_, err = compiler.requestPropertyReadOnly(oas.LocatedSchema{Raw: json.RawMessage(`{"writeOnly":1}`), Pointer: "#/property"})
	require.Error(t, err)
}

func TestDocumentationAndNumberHelperBranches(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateDefault(json.RawMessage(`invalid`), KindValidation{}))
	require.Error(t, validateDefault(json.RawMessage(`invalid`), KindValidation{Type: "string"}))
	require.NoError(t, validateDefault(json.RawMessage(`null`), KindValidation{Type: "string", Nullable: true}))
	require.Error(t, validateParsedDefault(jsonvalue.Value{
		Kind:   jsonvalue.KindNumber,
		Number: jsonvalue.Number{Lexeme: "invalid"},
	}, KindValidation{Type: "number"}))

	require.Error(t, validateXML(json.RawMessage(`{"namespace":1}`)))
	require.Error(t, validateExternalDocs(json.RawMessage(`{"url":null}`)))

	validation := new(Validation)
	err := compileEnum(validation, "#/schema", map[string]json.RawMessage{
		"enum": json.RawMessage(`["\ud800"]`),
	})
	require.Error(t, err)

	compiler := testCompiler(`{}`)
	err = compiler.compileString(validation, "#/schema", map[string]json.RawMessage{
		"pattern": json.RawMessage(`"^(?=a)a"`),
	})
	require.Error(t, err)
	require.Error(t, compileValidationStringFormat(validation, "#/schema", "unknown"))
}

func TestDirectDecoderCompilerAggregationErrors(t *testing.T) {
	t.Parallel()

	compiler := testCompiler(`{}`)
	_, err := compilePathDecoder("path", oas.Source{
		PathTemplate: "/{missing}",
		PathParameters: []oas.LocatedSchema{{
			Raw:     json.RawMessage(`{"name":"p","in":"path","required":true,"schema":{"type":"string"}}`),
			Pointer: "#/parameter",
		}},
	}, compiler)
	require.Error(t, err)

	_, err = compileQueryDecoder("query", oas.Source{
		QueryParameters: []oas.LocatedSchema{{Raw: json.RawMessage(`1`), Pointer: "#/parameter"}},
	}, compiler)
	require.Error(t, err)

	for _, schema := range []oas.LocatedSchema{
		{Raw: json.RawMessage(`{"$ref":"#/missing"}`), Pointer: "#/schema"},
		{Raw: json.RawMessage(`null`), Pointer: "#/schema"},
		{Raw: json.RawMessage(`{"type":"bad"}`), Pointer: "#/schema"},
	} {
		_, _, err = compileQueryParameterSchema("q", schema, compiler)
		require.Error(t, err)
	}
}

func TestZeroDecoderDefinitionsReturnErrors(t *testing.T) {
	t.Parallel()

	_, err := new(QueryDecoder).Definition()
	require.Error(t, err)
	_, err = new(PathDecoder).Definition()
	require.Error(t, err)
}
