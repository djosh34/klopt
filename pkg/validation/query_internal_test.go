//nolint:godoclint,lll // Coverage tests name private failure branches directly.
package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/oas"
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

func TestParseSharesCompiledValidationWithQueryDecoderDefinition(t *testing.T) {
	t.Parallel()

	validations, decoders, err := Parse([]byte(`openapi: 3.0.3
paths:
  /shared:
    post:
      operationId: shared
      parameters:
        - name: value
          in: query
          schema: {$ref: '#/components/schemas/Value'}
      requestBody:
        content:
          application/json:
            schema: {$ref: '#/components/schemas/Value'}
components:
  schemas:
    Value: {type: string}
`))
	require.NoError(t, err)
	require.Same(t, validations["shared"], decoders["shared"].Definition().Parameters[0].Validation)
}

func TestQueryDecoderDefinitionSharesValidation(t *testing.T) {
	t.Parallel()

	definition := QueryDecoderDefinition{
		OperationID: "query",
		Parameters: []QueryParameterDefinition{{
			Name: "filter", Wire: uint8(wireDeepObject),
			Validation: &Validation{
				KindValidation: KindValidation{Type: "object"},
				ObjectValidation: ObjectValidation{
					Required: []string{"value"},
					Properties: []PropertyValidation{{
						Name: "value", Validation: &Validation{KindValidation: KindValidation{Type: "string"}},
					}},
				},
			},
			Properties: []QueryPropertyDefinition{{Name: "value", ScalarType: "string"}},
		}},
	}

	decoder, err := NewQueryDecoderFromGenerated(definition)
	require.NoError(t, err)
	require.Same(t, definition.Parameters[0].Validation, decoder.Definition().Parameters[0].Validation)

	actual, err := decoder.Decode(&url.URL{RawQuery: `filter%5Bvalue%5D=ok`})
	require.NoError(t, err)
	require.JSONEq(t, `{"filter":{"value":"ok"}}`, string(actual))

	definition.Parameters[0].Validation.ObjectValidation.Properties[0].Validation.KindValidation.Type = "number"
	_, err = decoder.Decode(&url.URL{RawQuery: `filter%5Bvalue%5D=ok`})
	require.ErrorContains(t, err, "got string, want number")

	decoder.Definition().Parameters[0].Validation.
		ObjectValidation.Properties[0].Validation.KindValidation.Type = "string"
	actual, err = decoder.Decode(&url.URL{RawQuery: `filter%5Bvalue%5D=ok`})
	require.NoError(t, err)
	require.JSONEq(t, `{"filter":{"value":"ok"}}`, string(actual))
}

func TestQueryDecoderConcurrentDefinitionDecodeAndValidate(t *testing.T) {
	t.Parallel()

	definition := QueryDecoderDefinition{
		OperationID: "query",
		Parameters: []QueryParameterDefinition{{
			Name: "filter", Wire: uint8(wireDeepObject),
			Validation: &Validation{
				KindValidation: KindValidation{Type: "object"},
				ObjectValidation: ObjectValidation{
					Required: []string{"value"},
					Properties: []PropertyValidation{{
						Name: "value", Validation: &Validation{KindValidation: KindValidation{Type: "string"}},
					}},
				},
			},
			Properties: []QueryPropertyDefinition{{Name: "value", ScalarType: "string"}},
		}},
	}

	decoder, err := NewQueryDecoderFromGenerated(definition)
	require.NoError(t, err)

	const goroutines = 32

	errs := make(chan error, goroutines)

	var wait sync.WaitGroup
	for range goroutines {
		wait.Add(1)
		go func() {
			defer wait.Done()

			for range 100 {
				concurrentDefinition := decoder.Definition()
				if validationErrs := concurrentDefinition.Parameters[0].Validation.
					Validate(json.RawMessage(`{"value":"ok"}`)); len(validationErrs) != 0 {
					errs <- fmt.Errorf("validate: %w", errors.Join(validationErrs...))

					return
				}

				actual, decodeErr := decoder.Decode(&url.URL{RawQuery: `filter%5Bvalue%5D=ok`})
				if decodeErr != nil || string(actual) != `{"filter":{"value":"ok"}}` {
					errs <- fmt.Errorf("decode %s: %w", actual, decodeErr)

					return
				}
			}
		}()
	}

	wait.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
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

func TestSyntheticQueryValidationEscapesOperationID(t *testing.T) {
	t.Parallel()

	validation := syntheticQueryValidation("a/b~c", nil)
	require.Equal(t, "#/operations/a~1b~0c/query", validation.SchemaPointer)
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
