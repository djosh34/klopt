//nolint:godoclint,lll // Coverage tests name private failure branches directly.
package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/djosh34/decode_and_validate_generator/pkg/internal/oas"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/stretchr/testify/require"
)

func TestGeneratedQueryDecoderDefinitionRoundTripAndRejections(t *testing.T) {
	t.Parallel()

	definition := QueryDecoderDefinition{
		OperationID: "query",
		Parameters: []QueryParameterDefinition{{
			Name: "filter", Wire: uint8(wireDeepObject), Required: true, AllowEmpty: true,
			Validation:   &Validation{KindValidation: KindValidation{Type: "object"}},
			DefaultValue: json.RawMessage(`{"key":[]}`),
			Properties:   []QueryPropertyDefinition{{Name: "key", ScalarType: "string", Array: true}},
		}},
	}
	decoder, err := NewQueryDecoderFromGenerated(definition)
	require.NoError(t, err)
	require.Equal(t, definition, decoder.Definition())

	definition.Parameters[0].Validation = nil
	_, err = NewQueryDecoderFromGenerated(definition)
	require.ErrorContains(t, err, "is invalid")

	definition.Parameters[0].Validation = &Validation{}
	definition.Parameters[0].Wire = 255
	_, err = NewQueryDecoderFromGenerated(definition)
	require.ErrorContains(t, err, "is invalid")

	duplicate := QueryDecoderDefinition{OperationID: "query", Parameters: []QueryParameterDefinition{
		{Name: "q", Wire: uint8(wirePrimitive), Validation: &Validation{}},
		{Name: "q", Wire: uint8(wirePrimitive), Validation: &Validation{}},
	}}
	_, err = NewQueryDecoderFromGenerated(duplicate)
	require.ErrorContains(t, err, "ownership")
}

func TestPrivateQueryHelpersRejectImpossibleCompiledInputs(t *testing.T) {
	t.Parallel()

	_, err := parameterMembers(oas.LocatedSchema{Raw: json.RawMessage(`null`), Pointer: "#/parameter"})
	require.Error(t, err)

	compiler := schemaCompiler{source: oas.Source{}, bySchema: make(map[string]*Validation), active: make(map[string]struct{})}
	_, err = compileQueryParameter(oas.LocatedSchema{Raw: json.RawMessage(`null`), Pointer: "#/parameter"}, &compiler)
	require.Error(t, err)
	_, err = compileQueryParameter(oas.LocatedSchema{Raw: json.RawMessage(`{"name":null,"in":"query","schema":{"type":"string"}}`), Pointer: "#/parameter"}, &compiler)
	require.Error(t, err)

	source := oas.Source{Document: json.RawMessage(`{"schema":{"$ref":"#/missing"}}`)}
	_, _, _, err = directSchemaType( //nolint:dogsled // Only the error is relevant to this malformed schema case.
		source, oas.LocatedSchema{Raw: json.RawMessage(`{"$ref":"#/missing"}`), Pointer: "#/schema"},
	)
	require.Error(t, err)
	_, _, _, err = directSchemaType( //nolint:dogsled // Only the error is relevant to this malformed schema case.
		source, oas.LocatedSchema{Raw: json.RawMessage(`[]`), Pointer: "#/schema"},
	)
	require.Error(t, err)

	_, _, err = compileQueryProperties(
		oas.LocatedSchema{Raw: json.RawMessage(`null`), Pointer: "#/schema"}, source, false,
	)
	require.Error(t, err)

	var output bytes.Buffer

	encoder := jsontext.NewEncoder(&output)
	require.NoError(t, encoder.WriteToken(jsontext.BeginArray))
	require.Error(t, writeScalar(encoder, "unknown", "x", false))

	unknown := queryParameter{wire: wireKind(255)}
	require.Error(t, unknown.writeValue(jsontext.NewEncoder(&bytes.Buffer{}), []rawPair{{}}))

	delimited := queryParameter{wire: wireDelimitedArray, separator: ",", scalarType: "string"}
	require.Error(t, delimited.writeValue(jsontext.NewEncoder(&bytes.Buffer{}), []rawPair{{rawValue: "%zz"}}))

	object := queryParameter{wire: wireFormObjectNamed, separator: ","}
	require.Error(t, object.writeValue(jsontext.NewEncoder(&bytes.Buffer{}), []rawPair{{rawValue: "%zz"}}))
	require.Error(t, writeScalar(jsontext.NewEncoder(&bytes.Buffer{}), "string", "", false))

	_, err = splitStyleValue(rawPair{rawValue: "%zz"}, ",")
	require.Error(t, err)
	_, err = splitStyleValue(rawPair{rawValue: "%FF"}, ",")
	require.Error(t, err)
}

func TestPrivateQueryEncoderErrorsAreReturned(t *testing.T) {
	t.Parallel()

	expectingNameEncoder := func(t *testing.T) *jsontext.Encoder {
		t.Helper()

		encoder := jsontext.NewEncoder(&bytes.Buffer{})
		require.NoError(t, encoder.WriteToken(jsontext.BeginObject))

		return encoder
	}

	array := queryParameter{wire: wireFormArrayRepeated, scalarType: "string"}
	require.Error(t, array.writeValue(expectingNameEncoder(t), []rawPair{{decodedValue: "x"}}))

	delimited := queryParameter{wire: wireDelimitedArray, separator: ",", scalarType: "string"}
	require.Error(t, delimited.writeValue(expectingNameEncoder(t), []rawPair{{rawValue: "x", decodedValue: "x"}}))

	object := queryParameter{
		wire: wireFormObjectNamed, separator: ",",
		properties: []queryProperty{{name: "x", scalarType: "string"}}, propertyByName: map[string]int{"x": 0},
	}
	require.Error(t, object.writeValue(expectingNameEncoder(t), []rawPair{{rawValue: "x,y", decodedValue: "x,y"}}))

	exploded := queryParameter{
		wire:       wireFormObjectExploded,
		properties: []queryProperty{{name: "x", scalarType: "string"}},
	}
	require.Error(t, exploded.writeValue(expectingNameEncoder(t), []rawPair{{property: 0, decodedValue: "y"}}))
}

type alwaysFailWriter struct{}

func (alwaysFailWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestScalarEncoderWriteErrorsAreReturned(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		typeName string
		value    string
	}{
		{typeName: "string", value: "x"},
		{typeName: "boolean", value: "true"},
		{typeName: "boolean", value: "false"},
		{typeName: "number", value: "1"},
	} {
		encoder := jsontext.NewEncoder(alwaysFailWriter{})
		require.Error(t, writeScalar(encoder, test.typeName, test.value, false))
	}
}
