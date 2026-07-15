//nolint:godoclint,lll // Coverage tests name private failure branches directly.
package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sync"
	"testing"

	"github.com/djosh34/decode_and_validate_generator/pkg/internal/oas"
	"github.com/djosh34/decode_and_validate_generator/pkg/jsonvalue"
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

func TestQueryDecoderDefinitionIsDetachedDuringConcurrentDecode(t *testing.T) {
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

	definition.Parameters[0].Validation.ObjectValidation.Properties[0].Validation.KindValidation.Type = "number"
	definition.Parameters[0].Properties[0].Name = "changed"

	requireDetachedDecoderResult(t, decoder)

	snapshot := decoder.Definition()
	snapshot.Parameters[0].Validation.ObjectValidation.Properties[0].Validation.KindValidation.Type = "boolean"
	snapshot.Parameters[0].Properties[0].Name = "changed"

	requireDetachedDecoderResult(t, decoder)

	const goroutines = 32

	errs := make(chan error, goroutines)

	var wait sync.WaitGroup
	for range goroutines {
		wait.Add(1)
		go func() {
			defer wait.Done()

			for range 100 {
				concurrentSnapshot := decoder.Definition()
				concurrentSnapshot.Parameters[0].Validation.
					ObjectValidation.Properties[0].Validation.KindValidation.Type = "integer"

				actual, decodeErr := decoder.Decode(&url.URL{RawQuery: `filter[value]=ok`})
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

func requireDetachedDecoderResult(t *testing.T, decoder *QueryDecoder) {
	t.Helper()

	actual, err := decoder.Decode(&url.URL{RawQuery: `filter[value]=ok`})
	require.NoError(t, err)
	require.JSONEq(t, `{"filter":{"value":"ok"}}`, string(actual))
}

//nolint:funlen // One complete Validation graph proves every mutable subtree is copied.
func TestCloneValidationCopiesEveryMutableField(t *testing.T) {
	t.Parallel()

	number, err := jsonvalue.ParseNumber("2")
	require.NoError(t, err)

	leaf := &Validation{KindValidation: KindValidation{Type: "string"}}
	original := &Validation{
		SchemaPointer:  "#/root",
		KindValidation: KindValidation{Type: "object"},
		EnumValidation: EnumValidation{
			Values: []json.RawMessage{json.RawMessage(`{"n":[2]}`)},
			ExactValues: []jsonvalue.Value{{
				Kind: jsonvalue.KindObject,
				Object: []jsonvalue.Member{{
					Name: "n",
					Value: jsonvalue.Value{
						Kind:  jsonvalue.KindArray,
						Array: []jsonvalue.Value{{Kind: jsonvalue.KindNumber, Number: number}},
					},
				}},
			}},
		},
		NumberValidation: NumberValidation{
			Minimum:         &NumberBound{Value: "2", ExactValue: number},
			Maximum:         &NumberBound{Value: "2", Exclusive: true, ExactValue: number},
			MultipleOf:      "2",
			ExactMultipleOf: &number,
		},
		StringValidation: StringValidation{
			MinLength:       &CountBound{Value: "2", ExactValue: number},
			MaxLength:       &CountBound{Value: "2", ExactValue: number},
			Pattern:         "^x$",
			CompiledPattern: regexp.MustCompile("^x$"),
		},
		ArrayValidation: ArrayValidation{
			MinItems: &CountBound{Value: "2", ExactValue: number},
			MaxItems: &CountBound{Value: "2", ExactValue: number},
			Items:    leaf,
		},
		ObjectValidation: ObjectValidation{
			MinProperties:                  &CountBound{Value: "2", ExactValue: number},
			MaxProperties:                  &CountBound{Value: "2", ExactValue: number},
			Required:                       []string{"value"},
			Properties:                     []PropertyValidation{{Name: "value", Validation: leaf}},
			AdditionalPropertiesValidation: leaf,
		},
		AllOfValidations: []*Validation{leaf},
	}

	cloned := cloneValidation(original)
	require.Equal(t, original, cloned)
	require.NotSame(t, original, cloned)
	require.NotSame(t, original.EnumValidation.ExactValues[0].Object[0].Value.Array[0].Number.Rational,
		cloned.EnumValidation.ExactValues[0].Object[0].Value.Array[0].Number.Rational)
	require.NotSame(t, original.StringValidation.CompiledPattern, cloned.StringValidation.CompiledPattern)
	require.NotSame(t, leaf, cloned.ArrayValidation.Items)
	require.Same(t, cloned.ArrayValidation.Items, cloned.ObjectValidation.Properties[0].Validation)
	require.Same(t, cloned.ArrayValidation.Items, cloned.ObjectValidation.AdditionalPropertiesValidation)
	require.Same(t, cloned.ArrayValidation.Items, cloned.AllOfValidations[0])

	original.EnumValidation.Values[0][0] = '['
	original.EnumValidation.ExactValues[0].Object[0].Value.Array[0].Number.Rational.SetInt64(7)
	original.ObjectValidation.Required[0] = "changed"
	leaf.KindValidation.Type = "number"

	require.Equal(t, byte('{'), cloned.EnumValidation.Values[0][0])
	require.Equal(t, "2/1", cloned.EnumValidation.ExactValues[0].Object[0].Value.Array[0].Number.Rational.String())
	require.Equal(t, "value", cloned.ObjectValidation.Required[0])
	require.Equal(t, "string", cloned.ArrayValidation.Items.KindValidation.Type)

	require.Nil(t, cloneValidation(nil))
	require.Nil(t, cloneQueryParameters(nil))
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
