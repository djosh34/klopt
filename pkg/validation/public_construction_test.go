//nolint:dupl,godoclint // Query and path malformed-definition matrices intentionally mirror the public seams.
package validation_test

import (
	"encoding/json"
	"errors"
	"net/url"
	"testing"

	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/validation"

	"github.com/stretchr/testify/require"
)

func queryDefinitionForTest(t *testing.T, decoder *validation.QueryDecoder) validation.QueryDecoderDefinition {
	t.Helper()

	definition, err := decoder.Definition()
	require.NoError(t, err)

	return definition
}

func pathDefinitionForTest(t *testing.T, decoder *validation.PathDecoder) validation.PathDecoderDefinition {
	t.Helper()

	definition, err := decoder.Definition()
	require.NoError(t, err)

	return definition
}

// TestPublicCompiledFieldsSupportDirectConstruction verifies the external construction API.
func TestPublicCompiledFieldsSupportDirectConstruction(t *testing.T) {
	t.Parallel()

	one, err := jsonvalue.Parse(json.RawMessage(`1`))
	require.NoError(t, err)

	minimum, err := jsonvalue.ParseNumber("1")
	require.NoError(t, err)

	multipleOf, err := jsonvalue.ParseNumber("0.5")
	require.NoError(t, err)

	count, err := jsonvalue.ParseNumber("2")
	require.NoError(t, err)

	tests := []struct {
		name       string
		validation *validation.Validation
		valid      json.RawMessage
		invalid    json.RawMessage
	}{
		{
			name: "enum",
			validation: &validation.Validation{EnumValidation: validation.EnumValidation{
				Values:      []json.RawMessage{json.RawMessage(`1`)},
				ExactValues: []jsonvalue.Value{one},
			}},
			valid: json.RawMessage(`1.0`), invalid: json.RawMessage(`2`),
		},
		{
			name: "number",
			validation: &validation.Validation{NumberValidation: validation.NumberValidation{
				Minimum:         &validation.NumberBound{Value: "1", ExactValue: minimum},
				MultipleOf:      "0.5",
				ExactMultipleOf: &multipleOf,
			}},
			valid: json.RawMessage(`1.5`), invalid: json.RawMessage(`0.75`),
		},
		{
			name: "count",
			validation: &validation.Validation{StringValidation: validation.StringValidation{
				MinLength: &validation.CountBound{Value: "2", ExactValue: count},
			}},
			valid: json.RawMessage(`"ab"`), invalid: json.RawMessage(`"a"`),
		},
		{
			name: "pattern",
			validation: &validation.Validation{StringValidation: validation.StringValidation{
				Pattern:         "^a+$",
				CompiledPattern: patternvalidator.MustParse("^a+$"),
			}},
			valid: json.RawMessage(`"aa"`), invalid: json.RawMessage(`"b"`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Empty(t, test.validation.Validate(test.valid))
			require.NotEmpty(t, test.validation.Validate(test.invalid))
		})
	}
}

// TestPatternOptionsComposeAndPreserveSealing verifies public option propagation.
func TestPatternOptionsComposeAndPreserveSealing(t *testing.T) {
	t.Parallel()

	composite := validation.PatternOptions(
		patternvalidator.RejectNonASCII,
		patternvalidator.UseRE2,
	)

	unsealed := new(patternvalidator.PatternValidation)
	require.NoError(t, composite(unsealed))
	require.True(t, unsealed.RejectsNonASCII())
	require.True(t, unsealed.UsesRE2())

	spec := []byte(`{
		"openapi":"3.0.3",
		"info":{"title":"options","version":"1"},
		"paths":{"/request":{"post":{
			"operationId":"request",
			"requestBody":{"content":{"application/json":{"schema":{
				"type":"string","pattern":"^[a-z]+$"
			}}}},
			"responses":{"204":{"description":"empty"}}
		}}}
	}`)

	parsed, err := validation.Parse(spec, composite)
	require.NoError(t, err)

	compiled := parsed["request"].Body.StringValidation.CompiledPattern
	require.True(t, compiled.RejectsNonASCII())
	require.True(t, compiled.UsesRE2())
	require.Error(t, composite(compiled))
	require.True(t, compiled.RejectsNonASCII())
	require.True(t, compiled.UsesRE2())

	nilComposite := validation.PatternOptions(nil)
	require.Error(t, nilComposite(new(patternvalidator.PatternValidation)))

	customErr := errors.New("custom option failed")
	failing := validation.PatternOptions(func(*patternvalidator.PatternValidation) error { return customErr })
	require.ErrorIs(t, failing(new(patternvalidator.PatternValidation)), customErr)
	_, err = validation.Parse(spec, failing)
	require.ErrorIs(t, err, customErr)

	_, err = validation.Parse(spec, nil)
	require.Error(t, err)
}

func TestValidationRejectsMalformedCompiledPropertyNames(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		properties    []validation.PropertyValidation
		errorContains string
	}{
		{
			name: "empty", properties: []validation.PropertyValidation{{Name: "", Validation: new(validation.Validation)}},
			errorContains: "empty name",
		},
		{name: "unsorted", properties: []validation.PropertyValidation{
			{Name: "z", Validation: new(validation.Validation)},
			{Name: "a", Validation: new(validation.Validation)},
		}, errorContains: "not strictly increasing"},
		{name: "duplicate", properties: []validation.PropertyValidation{
			{Name: "a", Validation: new(validation.Validation)},
			{Name: "a", Validation: new(validation.Validation)},
		}, errorContains: "not strictly increasing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			compiled := &validation.Validation{ObjectValidation: validation.ObjectValidation{
				Properties: test.properties,
			}}

			decoder, err := validation.NewQueryDecoderFromGenerated(validation.QueryDecoderDefinition{
				OperationID: "propertyNames",
				Parameters: []validation.QueryParameterDefinition{{
					Name: "q", Wire: 7, Validation: compiled,
				}},
			})
			require.Nil(t, decoder)
			require.ErrorContains(t, err, test.errorContains)
		})
	}
}

func TestDecoderNilAndMalformedDefinitionFailuresReturnErrors(t *testing.T) {
	t.Parallel()

	var query *validation.QueryDecoder

	_, err := query.Definition()
	require.Error(t, err)
	_, err = query.Decode(&url.URL{})
	require.Error(t, err)

	var path *validation.PathDecoder

	_, err = path.Definition()
	require.Error(t, err)
	_, err = path.DecodePathParams(&url.URL{})
	require.Error(t, err)

	_, err = validation.NewQueryDecoderFromGenerated(validation.QueryDecoderDefinition{OperationID: "query"})
	require.Error(t, err)
	_, err = validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{OperationID: "path"})
	require.Error(t, err)

	malformedValidation := &validation.Validation{
		NumberValidation: validation.NumberValidation{
			Minimum: &validation.NumberBound{Value: "1", ExactValue: jsonvalue.Number{Lexeme: "invalid"}},
		},
	}
	_, err = validation.NewQueryDecoderFromGenerated(validation.QueryDecoderDefinition{
		OperationID: "query",
		Parameters: []validation.QueryParameterDefinition{{
			Name: "q", Validation: malformedValidation, ScalarType: "string",
		}},
	})
	require.Error(t, err)
	_, err = validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
		OperationID: "path", PathTemplate: "/{p}",
		Parameters: []validation.PathParameterDefinition{{
			Name: "p", Validation: malformedValidation, ScalarType: "string",
		}},
	})
	require.Error(t, err)
}

func TestGeneratedQueryDecoderRejectsMetadataInconsistentWithValidation(t *testing.T) {
	t.Parallel()

	stringValidation := &validation.Validation{KindValidation: validation.KindValidation{Type: "string"}}
	arrayValidation := &validation.Validation{
		KindValidation:  validation.KindValidation{Type: "array"},
		ArrayValidation: validation.ArrayValidation{Items: stringValidation},
	}
	objectValidation := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "object"},
		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed:    true,
			AdditionalPropertiesValidation: &validation.Validation{KindValidation: validation.KindValidation{Type: "integer"}},
			Properties:                     []validation.PropertyValidation{{Name: "known", Validation: stringValidation}},
		},
	}

	for _, test := range []struct {
		name      string
		parameter validation.QueryParameterDefinition
	}{
		{
			name: "primitive scalar type",
			parameter: validation.QueryParameterDefinition{
				Name: "q", Wire: 0, Validation: stringValidation, ScalarType: "integer",
			},
		},
		{
			name: "array shape",
			parameter: validation.QueryParameterDefinition{
				Name: "q", Wire: 0, Validation: arrayValidation, ScalarType: "string",
			},
		},
		{
			name: "array item type",
			parameter: validation.QueryParameterDefinition{
				Name: "q", Wire: 1, Validation: arrayValidation, ScalarType: "integer",
			},
		},
		{
			name: "object shape",
			parameter: validation.QueryParameterDefinition{
				Name: "q", Wire: 0, Validation: objectValidation, ScalarType: "string",
			},
		},
		{
			name: "object property",
			parameter: validation.QueryParameterDefinition{
				Name: "q", Wire: 6, Validation: objectValidation, DynamicType: "integer",
				Properties: []validation.QueryPropertyDefinition{{Name: "known", ScalarType: "boolean"}},
			},
		},
		{
			name: "object dynamic type",
			parameter: validation.QueryParameterDefinition{
				Name: "q", Wire: 6, Validation: objectValidation, DynamicType: "number",
				Properties: []validation.QueryPropertyDefinition{{Name: "known", ScalarType: "string"}},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := validation.NewQueryDecoderFromGenerated(validation.QueryDecoderDefinition{
				OperationID: "query", Parameters: []validation.QueryParameterDefinition{test.parameter},
			})
			require.Nil(t, decoder)
			require.ErrorContains(t, err, "inconsistent with validation")
		})
	}
}

func TestGeneratedPathDecoderRejectsMetadataInconsistentWithValidation(t *testing.T) {
	t.Parallel()

	stringValidation := &validation.Validation{KindValidation: validation.KindValidation{Type: "string"}}
	arrayValidation := &validation.Validation{
		KindValidation:  validation.KindValidation{Type: "array"},
		ArrayValidation: validation.ArrayValidation{Items: stringValidation},
	}
	objectValidation := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "object"},
		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed:    true,
			AdditionalPropertiesValidation: &validation.Validation{KindValidation: validation.KindValidation{Type: "integer"}},
			Properties:                     []validation.PropertyValidation{{Name: "known", Validation: stringValidation}},
		},
	}

	for _, test := range []struct {
		name      string
		parameter validation.PathParameterDefinition
	}{
		{
			name: "primitive scalar type",
			parameter: validation.PathParameterDefinition{
				Name: "p", Wire: 0, Validation: stringValidation, ScalarType: "integer",
			},
		},
		{
			name: "array shape",
			parameter: validation.PathParameterDefinition{
				Name: "p", Wire: 0, Validation: arrayValidation, ScalarType: "string",
			},
		},
		{
			name: "array item type",
			parameter: validation.PathParameterDefinition{
				Name: "p", Wire: 1, Validation: arrayValidation, ScalarType: "integer",
			},
		},
		{
			name: "object shape",
			parameter: validation.PathParameterDefinition{
				Name: "p", Wire: 0, Validation: objectValidation, ScalarType: "string",
			},
		},
		{
			name: "object property",
			parameter: validation.PathParameterDefinition{
				Name: "p", Wire: 2, Validation: objectValidation, DynamicType: "integer",
				Properties: []validation.PathPropertyDefinition{{Name: "known", ScalarType: "boolean"}},
			},
		},
		{
			name: "object dynamic type",
			parameter: validation.PathParameterDefinition{
				Name: "p", Wire: 2, Validation: objectValidation, DynamicType: "number",
				Properties: []validation.PathPropertyDefinition{{Name: "known", ScalarType: "string"}},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder, err := validation.NewPathDecoderFromGenerated(validation.PathDecoderDefinition{
				OperationID: "path", PathTemplate: "/{p}",
				Parameters: []validation.PathParameterDefinition{test.parameter},
			})
			require.Nil(t, decoder)
			require.ErrorContains(t, err, "inconsistent with validation")
		})
	}
}

func TestMustCompileStringFormatAdvertisesItsPanicBoundary(t *testing.T) {
	t.Parallel()

	require.NotNil(t, validation.MustCompileStringFormat("date"))
	require.Panics(t, func() { validation.MustCompileStringFormat("unknown") })
}

func TestParseReturnsMalformedPatternErrors(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		spec string
	}{
		{
			name: "body",
			spec: `openapi: 3.0.3
paths:
  /body:
    post:
      operationId: body
      requestBody:
        content:
          application/json:
            schema: {type: string, pattern: '['}
`,
		},
		{
			name: "path",
			spec: `openapi: 3.0.3
paths:
  /{id}:
    get:
      operationId: path
      parameters:
        - {name: id, in: path, required: true, schema: {type: string, pattern: '['}}
`,
		},
		{
			name: "query",
			spec: `openapi: 3.0.3
paths:
  /query:
    get:
      operationId: query
      parameters:
        - {name: q, in: query, schema: {type: string, pattern: '['}}
`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := validation.Parse([]byte(test.spec))
			require.Error(t, err)
			require.Nil(t, parsed)
		})
	}
}

func TestParseRetainsPatternAdmissionAndValidation(t *testing.T) {
	t.Parallel()

	spec := []byte(`{
		"openapi":"3.0.4",
		"info":{"title":"pattern","version":"1"},
		"paths":{"/request":{"post":{
			"operationId":"request",
			"requestBody":{"content":{"application/json":{"schema":{
				"type":"string","pattern":"^a+$"
			}}}},
			"responses":{"204":{"description":"empty"}}
		}}}
	}`)

	parsed, err := validation.Parse(spec)
	require.NoError(t, err)

	compiled := parsed["request"].Body.StringValidation
	require.NotNil(t, compiled.CompiledPattern)
	require.Empty(t, parsed["request"].Body.Validate([]byte(`"aa"`)))
	require.NotEmpty(t, parsed["request"].Body.Validate([]byte(`"b"`)))
}
