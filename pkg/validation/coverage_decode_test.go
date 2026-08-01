//nolint:godoclint // Internal white-box coverage matrices test private codec failures.
package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"testing"

	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/stretchr/testify/require"
)

type failingWriter struct {
	remaining int
}

func (writer *failingWriter) Write(value []byte) (int, error) {
	if writer.remaining == 0 {
		return 0, errors.New("write failed")
	}

	writer.remaining--

	return len(value), nil
}

func TestPathDecodeInternalFailureMatrix(t *testing.T) {
	t.Parallel()

	validation := new(Validation)
	tooManyCaptures := &PathDecoder{
		operationID: "path",
		segments:    []*regexp.Regexp{regexp.MustCompile(`^(.*)(.*)$`)},
		parameters:  []pathParameter{{name: "one", wire: pathWireSimplePrimitive, scalarType: "string"}},
		validation:  validation,
	}
	_, err := tooManyCaptures.decodePathParams(&url.URL{Path: "x"})
	require.Error(t, err)

	tooFewCaptures := &PathDecoder{
		operationID: "path",
		segments:    []*regexp.Regexp{regexp.MustCompile(`^x$`)},
		parameters:  []pathParameter{{name: "one", wire: pathWireSimplePrimitive, scalarType: "string"}},
		validation:  validation,
	}
	_, err = tooFewCaptures.decodePathParams(&url.URL{Path: "x"})
	require.Error(t, err)

	invalidResult := &PathDecoder{
		operationID: "path",
		segments:    []*regexp.Regexp{regexp.MustCompile(`^(.*)$`)},
		parameters:  []pathParameter{{name: string([]byte{0xff}), wire: pathWireJSONContent}},
		validation:  validation,
	}
	_, err = invalidResult.decodePathParams(&url.URL{Path: "1"})
	require.Error(t, err)
}

func TestPathParameterCodecFailureMatrix(t *testing.T) {
	t.Parallel()

	for _, parameter := range []*pathParameter{
		{wire: pathWireKind(255)},
		{wire: pathWireJSONContent},
		{wire: pathWireLabelPrimitive, scalarType: "string"},
		{wire: pathWireMatrixPrimitive, name: "id", scalarType: "string"},
		{wire: pathWireLabelArray},
		{wire: pathWireMatrixArray, name: "id", explode: true},
		{wire: pathWireKind(10)},
		{wire: pathWireLabelObject},
		{wire: pathWireMatrixObject, name: "id", explode: true},
		{wire: pathWireKind(11)},
	} {
		_, err := parameter.decodePathValue("invalid%zz")
		require.Error(t, err)
	}

	matrix := &pathParameter{name: "id", wire: pathWireMatrixArray, explode: true}
	_, err := matrix.matrixArrayValues("invalid")
	require.Error(t, err)
	_, err = matrix.matrixArrayValues(";other=1")
	require.Error(t, err)

	for _, test := range []struct {
		parameter pathParameter
		raw       string
	}{
		{parameter: pathParameter{wire: pathWireLabelObject, explode: true}, raw: ".bad"},
		{parameter: pathParameter{wire: pathWireLabelObject}, raw: ".a"},
		{parameter: pathParameter{wire: pathWireMatrixObject, explode: true}, raw: ";bad"},
		{parameter: pathParameter{wire: pathWireMatrixObject, name: "id"}, raw: ";id=a"},
	} {
		_, err = test.parameter.decodePathObject(test.raw)
		require.Error(t, err)
	}

	_, err = splitPackedPathObject("a")
	require.Error(t, err)
	_, err = splitExplodedPathObject("a", ",")
	require.Error(t, err)

	object := &pathParameter{dynamicType: "integer", propertyByName: map[string]int{}}
	for _, pairs := range [][][2]string{
		{{"", "1"}},
		{{"a", "1"}, {"a", "2"}},
		{{"a", "invalid"}},
		{{"%zz", "1"}},
		{{"a", "%zz"}},
	} {
		_, err = object.encodePathObjectValue(pairs)
		require.Error(t, err)
	}

	_, err = encodePathArray("integer", []string{"invalid"})
	require.Error(t, err)
	_, err = encodePathArray("string", []string{"%zz"})
	require.Error(t, err)
	_, err = encodePathObject(
		[]pathParameter{{name: "value"}},
		[]jsontext.Value{jsontext.Value(`invalid`)},
	)
	require.Error(t, err)
}

func TestQueryCodecFailureMatrix(t *testing.T) {
	t.Parallel()

	decoder := &QueryDecoder{
		operationID: "query",
		parameters:  []queryParameter{{name: "deep", wire: wireDeepObject, dynamicType: "string"}},
		owners:      map[string]queryClaim{"plain[]": {parameter: 0}},
		openForm:    -1,
		validation:  new(Validation),
	}
	for _, rawQuery := range []string{
		"plain%5b%5d=value",
		"deep=value",
		"deep%5Bchild=value",
		"deep%5Bchild%5D%5Bnested%5D=value",
	} {
		_, err := decoder.Decode(&url.URL{RawQuery: rawQuery})
		require.Error(t, err, rawQuery)
	}

	for _, parameter := range []*queryParameter{
		{wire: wirePrimitive, scalarType: "string"},
		{wire: wireDelimitedArray, scalarType: "string", separator: ","},
		{wire: wireFormObjectNamed, separator: ",", propertyByName: map[string]int{}},
		{wire: wireJSONContent},
		{wire: wireKind(255)},
	} {
		var output bytes.Buffer

		encoder := jsontext.NewEncoder(&output)
		err := parameter.writeValue(encoder, []rawPair{{decodedValue: "a"}, {decodedValue: "b"}})
		require.Error(t, err)
	}

	pairs := []rawPair{{rawValue: "a", decodedValue: "a"}}
	parameter := &queryParameter{wire: wireDelimitedArray, scalarType: "integer", separator: ","}

	var output bytes.Buffer

	err := parameter.writeValue(jsontext.NewEncoder(&output), pairs)
	require.Error(t, err)

	parameter = &queryParameter{wire: wireFormObjectNamed, separator: ",", propertyByName: map[string]int{}}

	for _, pair := range []rawPair{
		{rawValue: "a", decodedValue: "a"},
		{rawValue: "unknown,1", decodedValue: "unknown,1"},
		{rawValue: "a,1,a,2", decodedValue: "a,1,a,2"},
	} {
		output.Reset()
		err = parameter.writeNamedObject(jsontext.NewEncoder(&output), pair)
		require.Error(t, err)
	}

	parameter = &queryParameter{
		properties:  []queryProperty{{name: "scalar", scalarType: "string"}},
		dynamicType: "integer",
	}

	for _, occurrences := range [][]rawPair{
		{{property: 0, decodedValue: "a"}, {property: 0, decodedValue: "b"}},
		{{property: -1, childName: "x", decodedValue: "1"}, {property: -1, childName: "x", decodedValue: "2"}},
		{{property: -1, childName: "x", decodedValue: "invalid"}},
	} {
		output.Reset()
		err = parameter.writeExplodedObject(jsontext.NewEncoder(&output), occurrences)
		require.Error(t, err)
	}

	_, err = splitStyleValue(rawPair{rawValue: "a|b", decodedValue: "a|b"}, "|")
	require.Error(t, err)
	_, err = splitStyleValue(rawPair{rawValue: "%zz"}, ",")
	require.Error(t, err)

	output.Reset()
	require.Error(t, writeParsedNumber(
		jsontext.NewEncoder(&output),
		"integer",
		jsonvalue.Number{Lexeme: "invalid"},
	))

	for _, test := range []struct {
		typeName   string
		value      string
		allowEmpty bool
	}{
		{typeName: "string"},
		{typeName: "boolean", value: "invalid", allowEmpty: true},
		{typeName: "integer", value: "1.5", allowEmpty: true},
		{typeName: "number", value: "invalid", allowEmpty: true},
		{typeName: "invalid", value: "x", allowEmpty: true},
	} {
		output.Reset()
		err = writeScalar(jsontext.NewEncoder(&output), test.typeName, test.value, test.allowEmpty)
		require.Error(t, err)
	}
}

func TestJSONTextEncodingBranches(t *testing.T) {
	t.Parallel()

	encoded := appendJSONString(nil, "\"\\\b\f\n\r\t\x01é")
	require.True(t, json.Valid(encoded))

	decoder := &QueryDecoder{
		operationID: "query",
		parameters: []queryParameter{{
			name: "q", wire: wirePrimitive, scalarType: "string",
			defaultValue: jsontext.Value(`invalid`), validation: new(Validation),
		}},
		owners: map[string]queryClaim{}, openForm: -1, validation: new(Validation),
	}
	_, err := decoder.Decode(&url.URL{})
	require.Error(t, err)

	invalidName := string([]byte{0xff})

	var output bytes.Buffer

	parameter := &queryParameter{
		separator: ",", dynamicType: "string",
		propertyByName: map[string]int{invalidName: 0},
		properties:     []queryProperty{{name: invalidName, scalarType: "string"}},
	}
	err = parameter.writeNamedObject(
		jsontext.NewEncoder(&output),
		rawPair{rawValue: invalidName + ",x", decodedValue: invalidName + ",x"},
	)
	require.Error(t, err)
	output.Reset()
	err = parameter.writeExplodedObject(
		jsontext.NewEncoder(&output),
		[]rawPair{{property: 0, decodedValue: "x"}},
	)
	require.Error(t, err)
}

func TestCodecWriterErrorsPropagate(t *testing.T) {
	t.Parallel()

	for remaining := 0; remaining < 8; remaining++ {
		writer := &failingWriter{remaining: remaining}
		encoder := jsontext.NewEncoder(writer)
		parameter := &queryParameter{
			wire:           wireFormArrayRepeated,
			scalarType:     "string",
			properties:     []queryProperty{{name: "a", scalarType: "string", array: true}},
			propertyByName: map[string]int{"a": 0},
		}
		err := parameter.writeValue(encoder, []rawPair{{decodedValue: "a", property: 0}})
		require.True(t, err != nil || remaining > 0)

		writer = &failingWriter{remaining: remaining}
		encoder = jsontext.NewEncoder(writer)
		err = parameter.writeNamedObject(encoder, rawPair{rawValue: "a,b", decodedValue: "a,b"})
		require.True(t, err != nil || remaining > 0)

		writer = &failingWriter{remaining: remaining}
		encoder = jsontext.NewEncoder(writer)
		err = parameter.writeExplodedObject(encoder, []rawPair{{decodedValue: "a", property: 0}})
		require.True(t, err != nil || remaining > 0)
	}
}
