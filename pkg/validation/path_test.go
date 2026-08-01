//nolint:godoclint,lll // Path behavior tables use complete public definitions and literal expected values.
package validation_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/validation"
	"github.com/stretchr/testify/require"
)

func TestGeneratedPathDecoderDecodesEveryStyleShapeCell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wire       uint8
		explode    bool
		path       string
		validation *validation.Validation
		scalarType string
		properties []validation.PathPropertyDefinition
		expected   string
	}{
		{name: "simple primitive", wire: 0, path: "/blue", validation: pathStringValidation(), scalarType: "string", expected: `{"color":"blue"}`},
		{name: "simple primitive explode", wire: 0, explode: true, path: "/blue", validation: pathStringValidation(), scalarType: "string", expected: `{"color":"blue"}`},
		{name: "simple array", wire: 1, path: "/blue,black,brown", validation: pathArrayValidation(), scalarType: "string", expected: `{"color":["blue","black","brown"]}`},
		{name: "simple array explode", wire: 1, explode: true, path: "/blue,black,brown", validation: pathArrayValidation(), scalarType: "string", expected: `{"color":["blue","black","brown"]}`},
		{name: "simple object", wire: 2, path: "/R,100,G,200,B,150", validation: pathObjectValidation(), properties: pathObjectProperties(), expected: `{"color":{"R":100,"G":200,"B":150}}`},
		{name: "simple object explode", wire: 2, explode: true, path: "/R=100,G=200,B=150", validation: pathObjectValidation(), properties: pathObjectProperties(), expected: `{"color":{"R":100,"G":200,"B":150}}`},
		{name: "label primitive", wire: 3, path: "/.blue", validation: pathStringValidation(), scalarType: "string", expected: `{"color":"blue"}`},
		{name: "label primitive explode", wire: 3, explode: true, path: "/.blue", validation: pathStringValidation(), scalarType: "string", expected: `{"color":"blue"}`},
		{name: "label array", wire: 4, path: "/.blue,black,brown", validation: pathArrayValidation(), scalarType: "string", expected: `{"color":["blue","black","brown"]}`},
		{name: "label array explode", wire: 4, explode: true, path: "/.blue.black.brown", validation: pathArrayValidation(), scalarType: "string", expected: `{"color":["blue","black","brown"]}`},
		{name: "label object", wire: 5, path: "/.R,100,G,200,B,150", validation: pathObjectValidation(), properties: pathObjectProperties(), expected: `{"color":{"R":100,"G":200,"B":150}}`},
		{name: "label object explode", wire: 5, explode: true, path: "/.R=100.G=200.B=150", validation: pathObjectValidation(), properties: pathObjectProperties(), expected: `{"color":{"R":100,"G":200,"B":150}}`},
		{name: "matrix primitive", wire: 6, path: "/;color=blue", validation: pathStringValidation(), scalarType: "string", expected: `{"color":"blue"}`},
		{name: "matrix primitive explode", wire: 6, explode: true, path: "/;color=blue", validation: pathStringValidation(), scalarType: "string", expected: `{"color":"blue"}`},
		{name: "matrix array", wire: 7, path: "/;color=blue,black,brown", validation: pathArrayValidation(), scalarType: "string", expected: `{"color":["blue","black","brown"]}`},
		{name: "matrix array explode", wire: 7, explode: true, path: "/;color=blue;color=black;color=brown", validation: pathArrayValidation(), scalarType: "string", expected: `{"color":["blue","black","brown"]}`},
		{name: "matrix object", wire: 8, path: "/;color=R,100,G,200,B,150", validation: pathObjectValidation(), properties: pathObjectProperties(), expected: `{"color":{"R":100,"G":200,"B":150}}`},
		{name: "matrix object explode", wire: 8, explode: true, path: "/;R=100;G=200;B=150", validation: pathObjectValidation(), properties: pathObjectProperties(), expected: `{"color":{"R":100,"G":200,"B":150}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
				OperationID: "colors", PathTemplate: "/{color}",
				Parameters: []validation.PathParameterDefinition{{
					Name: "color", Wire: test.wire, Explode: test.explode, Validation: test.validation,
					ScalarType: test.scalarType, Properties: test.properties,
				}},
			})
			require.NoError(t, err)

			actual, err := decoder.DecodePathParams(&url.URL{Path: test.path})
			require.NoError(t, err)
			require.JSONEq(t, test.expected, string(actual))
		})
	}
}

func TestGeneratedPathDecoderUsesDeclaredShapeForEmptyCaptures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wire       uint8
		explode    bool
		path       string
		validation *validation.Validation
		scalarType string
		expected   string
	}{
		{name: "simple empty string", wire: 0, path: "/", validation: pathStringValidation(), scalarType: "string", expected: `{"p":""}`},
		{name: "simple exploded empty string", wire: 0, explode: true, path: "/", validation: pathStringValidation(), scalarType: "string", expected: `{"p":""}`},
		{name: "simple empty array", wire: 1, path: "/", validation: pathArrayValidation(), scalarType: "string", expected: `{"p":[]}`},
		{name: "simple exploded empty array", wire: 1, explode: true, path: "/", validation: pathArrayValidation(), scalarType: "string", expected: `{"p":[]}`},
		{name: "label framed empty array", wire: 4, path: "/.", validation: pathArrayValidation(), scalarType: "string", expected: `{"p":[]}`},
		{name: "label exploded framed empty array", wire: 4, explode: true, path: "/.", validation: pathArrayValidation(), scalarType: "string", expected: `{"p":[]}`},
		{name: "matrix framed empty array", wire: 7, path: "/;p", validation: pathArrayValidation(), scalarType: "string", expected: `{"p":[]}`},
		{name: "matrix exploded framed empty array", wire: 7, explode: true, path: "/;p", validation: pathArrayValidation(), scalarType: "string", expected: `{"p":[]}`},
		{name: "simple empty object", wire: 2, path: "/", validation: pathOpenObjectValidation(), expected: `{"p":{}}`},
		{name: "simple exploded empty object", wire: 2, explode: true, path: "/", validation: pathOpenObjectValidation(), expected: `{"p":{}}`},
		{name: "label framed empty object", wire: 5, path: "/.", validation: pathOpenObjectValidation(), expected: `{"p":{}}`},
		{name: "label exploded framed empty object", wire: 5, explode: true, path: "/.", validation: pathOpenObjectValidation(), expected: `{"p":{}}`},
		{name: "matrix framed empty object", wire: 8, path: "/;p", validation: pathOpenObjectValidation(), expected: `{"p":{}}`},
		{name: "matrix exploded framed empty object", wire: 8, explode: true, path: "/;p", validation: pathOpenObjectValidation(), expected: `{"p":{}}`},
		{name: "label framed empty string", wire: 3, path: "/.", validation: pathStringValidation(), scalarType: "string", expected: `{"p":""}`},
		{name: "label exploded framed empty string", wire: 3, explode: true, path: "/.", validation: pathStringValidation(), scalarType: "string", expected: `{"p":""}`},
		{name: "matrix framed empty string", wire: 6, path: "/;p=", validation: pathStringValidation(), scalarType: "string", expected: `{"p":""}`},
		{name: "matrix exploded framed empty string", wire: 6, explode: true, path: "/;p=", validation: pathStringValidation(), scalarType: "string", expected: `{"p":""}`},
		{name: "matrix undefined string", wire: 6, path: "/;p", validation: pathStringValidation(), scalarType: "string", expected: `{"p":""}`},
		{name: "matrix exploded undefined string", wire: 6, explode: true, path: "/;p", validation: pathStringValidation(), scalarType: "string", expected: `{"p":""}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
				OperationID: "empty", PathTemplate: "/{p}",
				Parameters: []validation.PathParameterDefinition{{
					Name: "p", Wire: test.wire, Explode: test.explode,
					Validation: test.validation, ScalarType: test.scalarType,
				}},
			})
			require.NoError(t, err)

			actual, err := decoder.DecodePathParams(&url.URL{Path: test.path})
			require.NoError(t, err)
			require.JSONEq(t, test.expected, string(actual))
		})
	}
}

func TestGeneratedPathDecoderRequiresStyleFramingForEmptyAggregates(t *testing.T) {
	t.Parallel()

	for _, wire := range []uint8{4, 5, 7, 8} {
		for _, explode := range []bool{false, true} {
			t.Run(fmt.Sprintf("wire %d explode %t", wire, explode), func(t *testing.T) {
				t.Parallel()

				parameterValidation := pathArrayValidation()
				scalarType := "string"

				if wire == 5 || wire == 8 {
					parameterValidation = pathOpenObjectValidation()
					scalarType = ""
				}

				decoder, err := validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
					OperationID: "empty", PathTemplate: "/{p}",
					Parameters: []validation.PathParameterDefinition{{
						Name: "p", Wire: wire, Explode: explode,
						Validation: parameterValidation, ScalarType: scalarType,
					}},
				})
				require.NoError(t, err)

				actual, err := decoder.DecodePathParams(&url.URL{Path: "/"})
				require.Nil(t, actual)
				require.Error(t, err)
			})
		}
	}
}

func TestGeneratedPathDecoderMatchesEscapedMatrixParameterNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wire       uint8
		explode    bool
		path       string
		validation *validation.Validation
		scalarType string
		expected   string
	}{
		{name: "primitive", wire: 6, path: "/;x%20y=blue", validation: pathStringValidation(), scalarType: "string", expected: `{"x y":"blue"}`},
		{name: "array", wire: 7, path: "/;x%20y=blue,black", validation: pathArrayValidation(), scalarType: "string", expected: `{"x y":["blue","black"]}`},
		{name: "exploded array", wire: 7, explode: true, path: "/;x%20y=blue;x%20y=black", validation: pathArrayValidation(), scalarType: "string", expected: `{"x y":["blue","black"]}`},
		{name: "object", wire: 8, path: "/;x%20y=key,value", validation: pathOpenObjectValidation(), expected: `{"x y":{"key":"value"}}`},
		{name: "undefined primitive", wire: 6, path: "/;x%20y", validation: pathStringValidation(), scalarType: "string", expected: `{"x y":""}`},
		{name: "undefined array", wire: 7, path: "/;x%20y", validation: pathArrayValidation(), scalarType: "string", expected: `{"x y":[]}`},
		{name: "undefined object", wire: 8, path: "/;x%20y", validation: pathOpenObjectValidation(), expected: `{"x y":{}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
				OperationID: "escaped", PathTemplate: "/{x y}",
				Parameters: []validation.PathParameterDefinition{{
					Name: "x y", Wire: test.wire, Explode: test.explode,
					Validation: test.validation, ScalarType: test.scalarType,
				}},
			})
			require.NoError(t, err)

			input, err := url.Parse(test.path)
			require.NoError(t, err)
			actual, err := decoder.DecodePathParams(input)
			require.NoError(t, err)
			require.JSONEq(t, test.expected, string(actual))
		})
	}
}

func TestGeneratedPathDecoderMatchesSegmentsAndExpressionOrder(t *testing.T) {
	t.Parallel()

	decoder, err := validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
		OperationID:  "matchPath",
		PathTemplate: "/literal.+/%2F/{z}-{a}//",
		Parameters: []validation.PathParameterDefinition{
			{Name: "a", Wire: 0, Validation: pathStringValidation(), ScalarType: "string"},
			{Name: "z", Wire: 0, Validation: pathStringValidation(), ScalarType: "string"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"z", "a"}, []string{
		pathDefinitionForTest(t, decoder).Parameters[0].Name,
		pathDefinitionForTest(t, decoder).Parameters[1].Name,
	})

	input, err := url.Parse("/literal.+/%2F/a%2Db-c//")
	require.NoError(t, err)
	actual, err := decoder.DecodePathParams(input)
	require.NoError(t, err)
	require.JSONEq(t, `{"z":"a-b","a":"c"}`, string(actual))

	for _, path := range []string{
		"/literalX+/slash/a-b//",
		"/literal.+/%2f/a-b//",
		"/literal.+/%2F/a/b//",
		"/literal.+/%2F/a-b/",
	} {
		mismatch, parseErr := url.Parse(path)
		require.NoError(t, parseErr)

		result, decodeErr := decoder.DecodePathParams(mismatch)
		require.Nil(t, result)
		require.Error(t, decodeErr)
		require.Equal(t, 1, strings.Count(decodeErr.Error(), `operation "matchPath"`))
	}
}

func TestGeneratedPathDecoderDecodesJSONContent(t *testing.T) {
	t.Parallel()

	decoder, err := validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
		OperationID: "jsonPath", PathTemplate: "/{value}",
		Parameters: []validation.PathParameterDefinition{{
			Name: "value", Wire: 9, Validation: pathAnyValidation(),
		}},
	})
	require.NoError(t, err)

	input, err := url.Parse(`/%7B%22number%22%3A1e100%2C%22nested%22%3A%5Btrue%2Cnull%5D%2C%22%22%3A%22a%2Fb%22%7D`)
	require.NoError(t, err)
	actual, err := decoder.DecodePathParams(input)
	require.NoError(t, err)
	require.Equal(t, `{"value":{"number":1e100,"nested":[true,null],"":"a/b"}}`, string(actual))

	nullURL, err := url.Parse(`/null`)
	require.NoError(t, err)
	actual, err = decoder.DecodePathParams(nullURL)
	require.NoError(t, err)
	require.JSONEq(t, `{"value":null}`, string(actual))

	rejecting, err := validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
		OperationID: "jsonString", PathTemplate: "/{value}",
		Parameters: []validation.PathParameterDefinition{{
			Name: "value", Wire: 9, Validation: pathStringValidation(),
		}},
	})
	require.NoError(t, err)
	actual, err = rejecting.DecodePathParams(nullURL)
	require.Nil(t, actual)
	require.ErrorContains(t, err, "got null, want string")

	for _, rawPath := range []string{"/", "/%7B", "/true%20false", "/%2522x%2522", "/%FF"} {
		malformed, parseErr := url.Parse(rawPath)
		require.NoError(t, parseErr)

		actual, decodeErr := decoder.DecodePathParams(malformed)
		require.Nil(t, actual)
		require.Error(t, decodeErr)
	}
}

func TestParsedPathDecoderInfersEnumOnlyContainerScalarSlots(t *testing.T) {
	t.Parallel()

	requests, err := validation.Parse([]byte(`openapi: 3.0.3
paths:
  /array/{value}:
    get:
      operationId: enumArray
      parameters:
        - {name: value, in: path, required: true, schema: {enum: [[1, 2]]}}
  /object/{value}:
    get:
      operationId: enumObject
      parameters:
        - {name: value, in: path, required: true, explode: true, schema: {enum: [{count: 2, enabled: true}]}}
  /partial-object/{value}:
    get:
      operationId: partialEnumObject
      parameters:
        - name: value
          in: path
          required: true
          explode: true
          schema:
            type: object
            properties:
              count: {}
            enum: [{count: 2}]
`))
	require.NoError(t, err)

	array, err := requests["enumArray"].Path.DecodePathParams(&url.URL{Path: "/array/1,2"})
	require.NoError(t, err)
	require.JSONEq(t, `{"value":[1,2]}`, string(array))

	object, err := requests["enumObject"].Path.DecodePathParams(
		&url.URL{Path: "/object/count=2,enabled=true"},
	)
	require.NoError(t, err)
	require.JSONEq(t, `{"value":{"count":2,"enabled":true}}`, string(object))

	partialObject, err := requests["partialEnumObject"].Path.DecodePathParams(
		&url.URL{Path: "/partial-object/count=2"},
	)
	require.NoError(t, err)
	require.JSONEq(t, `{"value":{"count":2}}`, string(partialObject))
}

func TestGeneratedPathDecoderDefinitionRoundTripAndCopiesMetadata(t *testing.T) {
	t.Parallel()

	definition := validation.PathDecoderDefinition{
		OperationID: "roundTrip", PathTemplate: "/{object}",
		Parameters: []validation.PathParameterDefinition{{
			Name: "object", Wire: 8, Explode: true, Validation: pathOpenObjectValidation(),
			DynamicType: "number",
			Properties:  []validation.PathPropertyDefinition{{Name: "known", ScalarType: "boolean"}},
		}},
	}
	decoder, err := validation.NewPathDecoderFromGenerated(definition)
	require.NoError(t, err)
	require.Equal(t, definition, pathDefinitionForTest(t, decoder))
	require.Same(t, definition.Parameters[0].Validation, pathDefinitionForTest(t, decoder).Parameters[0].Validation)

	definition.PathTemplate = "/changed"
	definition.Parameters[0].Name = "changed"
	definition.Parameters[0].Properties[0].Name = "changed"
	restored := pathDefinitionForTest(t, decoder)
	require.Equal(t, "/{object}", restored.PathTemplate)
	require.Equal(t, "object", restored.Parameters[0].Name)
	require.Equal(t, "known", restored.Parameters[0].Properties[0].Name)

	restored.Parameters[0].Properties[0].Name = "alsoChanged"
	require.Equal(t, "known", pathDefinitionForTest(t, decoder).Parameters[0].Properties[0].Name)
}

func TestGeneratedPathDecoderRejectsInvalidDefinitions(t *testing.T) {
	t.Parallel()

	nullable := &validation.Validation{KindValidation: validation.KindValidation{Type: "string", Nullable: true}}
	composedNullable := &validation.Validation{AllOfValidations: []*validation.Validation{nullable}}
	mentionedButRejected := &validation.Validation{
		KindValidation:   validation.KindValidation{Type: "string", Nullable: true},
		AllOfValidations: []*validation.Validation{{KindValidation: validation.KindValidation{Type: "string"}}},
	}

	validStyle := validation.PathParameterDefinition{
		Name: "p", Wire: 0, Validation: pathStringValidation(), ScalarType: "string",
	}
	validContent := validation.PathParameterDefinition{Name: "p", Wire: 9, Validation: nullable}

	tests := []struct {
		name       string
		definition validation.PathDecoderDefinition
	}{
		{name: "invalid operation ID", definition: validation.PathDecoderDefinition{OperationID: "not valid", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{validStyle}}},
		{name: "missing leading slash", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "{p}", Parameters: []validation.PathParameterDefinition{validStyle}}},
		{name: "unmatched opening brace", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p", Parameters: []validation.PathParameterDefinition{validStyle}}},
		{name: "unmatched closing brace", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/p}", Parameters: []validation.PathParameterDefinition{validStyle}}},
		{name: "empty expression", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{}", Parameters: []validation.PathParameterDefinition{validStyle}}},
		{name: "nested expression", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{{p}", Parameters: []validation.PathParameterDefinition{validStyle}}},
		{name: "adjacent expressions", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}{q}", Parameters: []validation.PathParameterDefinition{validStyle, {Name: "q", Wire: 0, Validation: pathStringValidation(), ScalarType: "string"}}}},
		{name: "repeated expression", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}/{p}", Parameters: []validation.PathParameterDefinition{validStyle}}},
		{name: "missing declaration", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{q}", Parameters: []validation.PathParameterDefinition{validStyle}}},
		{name: "unused declaration", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/literal", Parameters: []validation.PathParameterDefinition{validStyle}}},
		{name: "duplicate declaration", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{validStyle, validStyle}}},
		{name: "empty parameter name", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{{Wire: 0, Validation: pathStringValidation(), ScalarType: "string"}}}},
		{name: "slash parameter name", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p/q}", Parameters: []validation.PathParameterDefinition{{Name: "p/q", Wire: 0, Validation: pathStringValidation(), ScalarType: "string"}}}},
		{name: "nil validation", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{{Name: "p", Wire: 0, ScalarType: "string"}}}},
		{name: "wire out of range", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{{Name: "p", Wire: 10, Validation: pathStringValidation()}}}},
		{name: "primitive missing scalar", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{{Name: "p", Wire: 0, Validation: pathStringValidation()}}}},
		{name: "array with properties", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{{Name: "p", Wire: 1, Validation: pathArrayValidation(), ScalarType: "string", Properties: []validation.PathPropertyDefinition{{Name: "x", ScalarType: "string"}}}}}},
		{name: "object with root scalar", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{{Name: "p", Wire: 2, Validation: pathOpenObjectValidation(), ScalarType: "string"}}}},
		{name: "object invalid dynamic scalar", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{{Name: "p", Wire: 2, Validation: pathOpenObjectValidation(), DynamicType: "object"}}}},
		{name: "object empty property", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{{Name: "p", Wire: 2, Validation: pathOpenObjectValidation(), Properties: []validation.PathPropertyDefinition{{ScalarType: "string"}}}}}},
		{name: "object duplicate property", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{{Name: "p", Wire: 2, Validation: pathOpenObjectValidation(), Properties: []validation.PathPropertyDefinition{{Name: "x", ScalarType: "string"}, {Name: "x", ScalarType: "string"}}}}}},
		{name: "JSON content explode", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{{Name: "p", Wire: 9, Explode: true, Validation: nullable}}}},
		{name: "JSON content style metadata", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{{Name: "p", Wire: 9, Validation: nullable, ScalarType: "string"}}}},
		{name: "direct nullable style", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{{Name: "p", Wire: 0, Validation: nullable, ScalarType: "string"}}}},
		{name: "composed nullable style", definition: validation.PathDecoderDefinition{OperationID: "op", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{{Name: "p", Wire: 0, Validation: composedNullable, ScalarType: "string"}}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := validation.NewPathDecoderFromGenerated(test.definition)
			require.Nil(t, decoder)
			require.Error(t, err)
		})
	}

	_, err := validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
		OperationID: "composed", PathTemplate: "/{p}",
		Parameters: []validation.PathParameterDefinition{{
			Name: "p", Wire: 0, Validation: mentionedButRejected, ScalarType: "string",
		}},
	})
	require.NoError(t, err)

	_, err = validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
		OperationID: "content", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{validContent},
	})
	require.NoError(t, err)

	_, err = validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
		OperationID: "not valid", PathTemplate: "/{p}", Parameters: []validation.PathParameterDefinition{validStyle},
	})
	require.True(t, errors.Is(err, oas.ErrInvalidOperationID))
}

func TestGeneratedPathDecoderSplitsBeforeUnescaping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		parameter validation.PathParameterDefinition
		rawPath   string
		expected  string
	}{
		{
			name: "simple string punctuation", rawPath: "/a%2Cb%2Fc%3Bd%3De%2Ef%252F",
			parameter: validation.PathParameterDefinition{
				Name: "p", Wire: 0, Validation: pathStringValidation(), ScalarType: "string",
			},
			expected: `{"p":"a,b/c;d=e.f%2F"}`,
		},
		{
			name: "simple array encoded comma", rawPath: "/a%2Cb,%2F,,z",
			parameter: validation.PathParameterDefinition{
				Name: "p", Wire: 1, Validation: pathArrayValidation(), ScalarType: "string",
			},
			expected: `{"p":["a,b","/","","z"]}`,
		},
		{
			name: "exploded label encoded dot", rawPath: "/.a%2Eb..z",
			parameter: validation.PathParameterDefinition{
				Name: "p", Wire: 4, Explode: true, Validation: pathArrayValidation(), ScalarType: "string",
			},
			expected: `{"p":["a.b","","z"]}`,
		},
		{
			name: "exploded matrix encoded semicolon and equals", rawPath: "/;p=a%3Bb;p;p=%3D",
			parameter: validation.PathParameterDefinition{
				Name: "p", Wire: 7, Explode: true, Validation: pathArrayValidation(), ScalarType: "string",
			},
			expected: `{"p":["a;b","","="]}`,
		},
		{
			name: "exploded object encoded structure", rawPath: "/a%3Db=x%2Cy,%61%2Cb=z%3Dw",
			parameter: validation.PathParameterDefinition{
				Name: "p", Wire: 2, Explode: true, Validation: pathOpenObjectValidation(), DynamicType: "string",
			},
			expected: `{"p":{"a=b":"x,y","a,b":"z=w"}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder := mustGeneratedPathDecoder(t, "decode", "/{p}", test.parameter)
			input, err := url.Parse(test.rawPath)
			require.NoError(t, err)
			actual, err := decoder.DecodePathParams(input)
			require.NoError(t, err)
			require.JSONEq(t, test.expected, string(actual))
		})
	}
}

func TestGeneratedPathDecoderConvertsDeclaredScalarTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		typeName   string
		path       string
		validation *validation.Validation
		expected   string
	}{
		{name: "boolean true", typeName: "boolean", path: "/true", validation: &validation.Validation{KindValidation: validation.KindValidation{Type: "boolean"}}, expected: `{"p":true}`},
		{name: "boolean false", typeName: "boolean", path: "/false", validation: &validation.Validation{KindValidation: validation.KindValidation{Type: "boolean"}}, expected: `{"p":false}`},
		{name: "integer", typeName: "integer", path: "/-42", validation: &validation.Validation{KindValidation: validation.KindValidation{Type: "integer"}}, expected: `{"p":-42}`},
		{name: "number", typeName: "number", path: "/1.25e2", validation: &validation.Validation{KindValidation: validation.KindValidation{Type: "number"}}, expected: `{"p":125}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder := mustGeneratedPathDecoder(t, "scalar", "/{p}", validation.PathParameterDefinition{
				Name: "p", Wire: 0, Validation: test.validation, ScalarType: test.typeName,
			})
			actual, err := decoder.DecodePathParams(&url.URL{Path: test.path})
			require.NoError(t, err)
			require.Equal(t, test.expected, string(actual))
		})
	}

	unknown := mustGeneratedPathDecoder(t, "unknown", "/{p}", validation.PathParameterDefinition{
		Name: "p", Wire: 1, Validation: pathArrayValidation(), ScalarType: "string",
	})
	actual, err := unknown.DecodePathParams(&url.URL{Path: "/123,true,false,null,1.5"})
	require.NoError(t, err)
	require.Equal(t, `{"p":["123","true","false","null","1.5"]}`, string(actual))
}

func TestGeneratedPathDecoderRejectsMalformedPathValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		parameter validation.PathParameterDefinition
		rawPath   string
	}{
		{name: "label missing marker", rawPath: "/value", parameter: validation.PathParameterDefinition{Name: "p", Wire: 3, Validation: pathStringValidation(), ScalarType: "string"}},
		{name: "matrix wrong parameter", rawPath: "/;q=value", parameter: validation.PathParameterDefinition{Name: "p", Wire: 6, Validation: pathStringValidation(), ScalarType: "string"}},
		{name: "packed object odd terms", rawPath: "/a,1,b", parameter: validation.PathParameterDefinition{Name: "p", Wire: 2, Validation: pathOpenObjectValidation(), DynamicType: "string"}},
		{name: "exploded object missing equals", rawPath: "/a=1,b", parameter: validation.PathParameterDefinition{Name: "p", Wire: 2, Explode: true, Validation: pathOpenObjectValidation(), DynamicType: "string"}},
		{name: "duplicate decoded key", rawPath: "/a=1,%61=2", parameter: validation.PathParameterDefinition{Name: "p", Wire: 2, Explode: true, Validation: pathOpenObjectValidation(), DynamicType: "string"}},
		{name: "empty dynamic key", rawPath: "/=value", parameter: validation.PathParameterDefinition{Name: "p", Wire: 2, Explode: true, Validation: pathOpenObjectValidation(), DynamicType: "string"}},
		{name: "strict boolean", rawPath: "/True", parameter: validation.PathParameterDefinition{Name: "p", Wire: 0, Validation: &validation.Validation{KindValidation: validation.KindValidation{Type: "boolean"}}, ScalarType: "boolean"}},
		{name: "strict integer", rawPath: "/1.5", parameter: validation.PathParameterDefinition{Name: "p", Wire: 0, Validation: &validation.Validation{KindValidation: validation.KindValidation{Type: "integer"}}, ScalarType: "integer"}},
		{name: "empty number", rawPath: "/", parameter: validation.PathParameterDefinition{Name: "p", Wire: 0, Validation: &validation.Validation{KindValidation: validation.KindValidation{Type: "number"}}, ScalarType: "number"}},
		{name: "invalid UTF-8", rawPath: "/%FF", parameter: validation.PathParameterDefinition{Name: "p", Wire: 0, Validation: pathStringValidation(), ScalarType: "string"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder := mustGeneratedPathDecoder(t, "reject", "/{p}", test.parameter)
			input, err := url.Parse(test.rawPath)
			require.NoError(t, err)
			actual, err := decoder.DecodePathParams(input)
			require.Nil(t, actual)
			require.Error(t, err)
			require.Equal(t, 1, strings.Count(err.Error(), `operation "reject"`))
		})
	}

	decoder := mustGeneratedPathDecoder(t, "nilURL", "/{p}", validation.PathParameterDefinition{
		Name: "p", Wire: 0, Validation: pathStringValidation(), ScalarType: "string",
	})
	actual, err := decoder.DecodePathParams(nil)
	require.Nil(t, actual)
	require.ErrorContains(t, err, "nil URL")
}

func TestGeneratedPathDecoderValidatesOneCompleteObject(t *testing.T) {
	t.Parallel()

	minimumOne, err := jsonvalue.ParseNumber("1")
	require.NoError(t, err)

	emptyTests := []struct {
		name      string
		parameter validation.PathParameterDefinition
		keyword   string
	}{
		{
			name: "empty array does not retry", keyword: "minItems",
			parameter: validation.PathParameterDefinition{
				Name: "p", Wire: 1, ScalarType: "string",
				Validation: &validation.Validation{
					KindValidation: validation.KindValidation{Type: "array"},
					ArrayValidation: validation.ArrayValidation{
						MinItems: &validation.CountBound{Value: "1", ExactValue: minimumOne},
						Items:    pathStringValidation(),
					},
				},
			},
		},
		{
			name: "empty object does not retry", keyword: "minProperties",
			parameter: validation.PathParameterDefinition{
				Name: "p", Wire: 2,
				Validation: &validation.Validation{
					KindValidation: validation.KindValidation{Type: "object"},
					ObjectValidation: validation.ObjectValidation{
						MinProperties:               &validation.CountBound{Value: "1", ExactValue: minimumOne},
						AdditionalPropertiesAllowed: true,
					},
				},
			},
		},
		{
			name: "empty string does not retry", keyword: "minLength",
			parameter: validation.PathParameterDefinition{
				Name: "p", Wire: 0, ScalarType: "string",
				Validation: &validation.Validation{
					KindValidation: validation.KindValidation{Type: "string"},
					StringValidation: validation.StringValidation{
						MinLength: &validation.CountBound{Value: "1", ExactValue: minimumOne},
					},
				},
			},
		},
	}

	for _, test := range emptyTests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder := mustGeneratedPathDecoder(t, "constraint", "/{p}", test.parameter)
			actual, decodeErr := decoder.DecodePathParams(&url.URL{Path: "/"})
			require.Nil(t, actual)
			require.ErrorContains(t, decodeErr, test.keyword)
		})
	}

	closedObject := &validation.Validation{KindValidation: validation.KindValidation{Type: "object"}}
	decoder := mustGeneratedPathDecoder(t, "closed", "/{p}", validation.PathParameterDefinition{
		Name: "p", Wire: 2, Validation: closedObject,
	})
	actual, err := decoder.DecodePathParams(&url.URL{Path: "/extra,value"})
	require.Nil(t, actual)
	require.ErrorContains(t, err, "additionalProperties")

	joined, err := validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
		OperationID: "joined", PathTemplate: "/{z}/{a}",
		Parameters: []validation.PathParameterDefinition{
			{Name: "z", Wire: 0, Validation: &validation.Validation{KindValidation: validation.KindValidation{Type: "number"}}, ScalarType: "string"},
			{Name: "a", Wire: 0, Validation: &validation.Validation{KindValidation: validation.KindValidation{Type: "boolean"}}, ScalarType: "string"},
		},
	})
	require.NoError(t, err)
	actual, err = joined.DecodePathParams(&url.URL{Path: "/value/other"})
	require.Nil(t, actual)
	require.ErrorContains(t, err, "#/a")
	require.ErrorContains(t, err, "#/z")
}

func TestGeneratedPathDecoderUsesOperationRelativeEscapedPath(t *testing.T) {
	t.Parallel()

	decoder := mustGeneratedPathDecoder(t, "pet", "/pets/{id}", validation.PathParameterDefinition{
		Name: "id", Wire: 0, Validation: pathStringValidation(), ScalarType: "string",
	})

	tests := []struct {
		name     string
		input    *url.URL
		expected string
	}{
		{name: "operation relative", input: &url.URL{Path: "/pets/42", RawQuery: "ignored=true"}, expected: `{"id":"42"}`},
		{name: "encoded slash is data", input: &url.URL{Path: "/pets/a/b", RawPath: "/pets/a%2Fb"}, expected: `{"id":"a/b"}`},
		{name: "empty RawPath", input: &url.URL{Path: "/pets/a b"}, expected: `{"id":"a b"}`},
		{name: "malformed RawPath falls back", input: &url.URL{Path: "/pets/a b", RawPath: "/pets/%zz"}, expected: `{"id":"a b"}`},
		{name: "inconsistent RawPath falls back", input: &url.URL{Path: "/pets/value", RawPath: "/pets/other"}, expected: `{"id":"value"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := decoder.DecodePathParams(test.input)
			require.NoError(t, err)
			require.JSONEq(t, test.expected, string(actual))
		})
	}

	actual, err := decoder.DecodePathParams(&url.URL{Path: "/api/v1/pets/42"})
	require.Nil(t, actual)
	require.ErrorContains(t, err, "segment count")
}

func TestGeneratedPathDecoderIsConcurrent(t *testing.T) {
	t.Parallel()

	decoder := mustGeneratedPathDecoder(t, "concurrent", "/items/{id}", validation.PathParameterDefinition{
		Name: "id", Wire: 0, Validation: pathStringValidation(), ScalarType: "string",
	})

	const goroutines = 32

	errResults := make(chan error, goroutines)

	var wait sync.WaitGroup
	for range goroutines {
		wait.Add(1)
		go func() {
			defer wait.Done()

			for range 100 {
				actual, err := decoder.DecodePathParams(&url.URL{Path: "/items/value"})
				if err != nil || string(actual) != `{"id":"value"}` {
					errResults <- fmt.Errorf("decode %s: %w", actual, err)

					return
				}

				definition, definitionErr := decoder.Definition()
				if definitionErr != nil {
					errResults <- definitionErr

					return
				}

				if definition.PathTemplate != "/items/{id}" {
					errResults <- errors.New("definition changed")

					return
				}
			}
		}()
	}

	wait.Wait()
	close(errResults)

	for err := range errResults {
		require.NoError(t, err)
	}
}

func TestGeneratedPathDecoderNamedRobustnessPaths(t *testing.T) {
	t.Parallel()

	decoder, err := validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
		OperationID: "robustPath", PathTemplate: "/prefix/{x}-{y}",
		Parameters: []validation.PathParameterDefinition{
			{Name: "x", Wire: 0, Validation: pathStringValidation(), ScalarType: "string"},
			{Name: "y", Wire: 0, Validation: pathStringValidation(), ScalarType: "string"},
		},
	})
	require.NoError(t, err)

	for _, test := range []struct {
		name string
		path string
	}{
		{name: "normal", path: "/prefix/a-b"},
		{name: "encoded delimiter", path: "/prefix/a%2Db-c"},
		{name: "empty components", path: "/prefix/-"},
		{name: "wrong prefix", path: "/other/a-b"},
		{name: "extra slash", path: "/prefix/a/b-c"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			input := &url.URL{Path: test.path}
			first, firstErr := decoder.DecodePathParams(input)
			second, secondErr := decoder.DecodePathParams(input)
			require.Equal(t, string(first), string(second))
			require.Equal(t, fmt.Sprint(firstErr), fmt.Sprint(secondErr))

			if firstErr != nil {
				require.Nil(t, first)

				return
			}

			require.True(t, json.Valid(first))
		})
	}
}

func mustGeneratedPathDecoder(
	t *testing.T,
	operationID string,
	pathTemplate string,
	parameter validation.PathParameterDefinition,
) *validation.PathDecoder {
	t.Helper()

	decoder, err := validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
		OperationID: operationID, PathTemplate: pathTemplate,
		Parameters: []validation.PathParameterDefinition{parameter},
	})
	require.NoError(t, err)

	return decoder
}

func TestGeneratedPathDecoderDecodesSimpleString(t *testing.T) {
	t.Parallel()

	decoder, err := validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
		OperationID:  "getPet",
		PathTemplate: "/pets/{id}",
		Parameters: []validation.PathParameterDefinition{{
			Name: "id", Wire: 0,
			Validation: &validation.Validation{KindValidation: validation.KindValidation{Type: "string"}},
			ScalarType: "string",
		}},
	})
	require.NoError(t, err)

	actual, err := decoder.DecodePathParams(&url.URL{Path: "/pets/a/b", RawPath: "/pets/a%2Fb"})
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"a/b"}`, string(actual))
}

func pathStringValidation() *validation.Validation {
	return &validation.Validation{KindValidation: validation.KindValidation{Type: "string"}}
}

func pathAnyValidation() *validation.Validation {
	return &validation.Validation{
		ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true},
	}
}

func pathArrayValidation() *validation.Validation {
	return &validation.Validation{
		KindValidation:  validation.KindValidation{Type: "array"},
		ArrayValidation: validation.ArrayValidation{Items: pathStringValidation()},
	}
}

func pathObjectValidation() *validation.Validation {
	properties := make([]validation.PropertyValidation, 0, 3)
	for _, name := range []string{"R", "G", "B"} {
		properties = append(properties, validation.PropertyValidation{
			Name: name, Validation: &validation.Validation{KindValidation: validation.KindValidation{Type: "integer"}},
		})
	}

	return &validation.Validation{
		KindValidation: validation.KindValidation{Type: "object"},
		ObjectValidation: validation.ObjectValidation{
			Properties: properties, AdditionalPropertiesAllowed: true,
		},
	}
}

func pathOpenObjectValidation() *validation.Validation {
	return &validation.Validation{
		KindValidation:   validation.KindValidation{Type: "object"},
		ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true},
	}
}

func pathObjectProperties() []validation.PathPropertyDefinition {
	return []validation.PathPropertyDefinition{
		{Name: "R", ScalarType: "integer"},
		{Name: "G", ScalarType: "integer"},
		{Name: "B", ScalarType: "integer"},
	}
}
