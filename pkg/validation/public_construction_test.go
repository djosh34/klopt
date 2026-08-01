//nolint:godoclint // Public construction tests document the retained validation seam.
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

func TestValidationRejectsNilCyclesAndMalformedCompiledState(t *testing.T) {
	t.Parallel()

	var nilValidation *validation.Validation
	require.NotEmpty(t, nilValidation.Validate(json.RawMessage(`null`)))

	nilChildren := []*validation.Validation{
		{ArrayValidation: validation.ArrayValidation{Items: nil}, AllOfValidations: []*validation.Validation{nil}},
		{ObjectValidation: validation.ObjectValidation{Properties: []validation.PropertyValidation{{Name: "value"}}}},
		{AnyOfValidations: []*validation.Validation{nil}},
	}
	for _, compiled := range nilChildren {
		require.NotEmpty(t, compiled.Validate(json.RawMessage(`null`)))
	}

	cycles := []func(*validation.Validation){
		func(root *validation.Validation) { root.ArrayValidation.Items = root },
		func(root *validation.Validation) {
			root.ObjectValidation.Properties = []validation.PropertyValidation{{Name: "value", Validation: root}}
		},
		func(root *validation.Validation) { root.ObjectValidation.AdditionalPropertiesValidation = root },
		func(root *validation.Validation) { root.AllOfValidations = []*validation.Validation{root} },
		func(root *validation.Validation) { root.AnyOfValidations = []*validation.Validation{root} },
	}
	for _, makeCycle := range cycles {
		compiled := new(validation.Validation)
		makeCycle(compiled)
		require.NotEmpty(t, compiled.Validate(json.RawMessage(`null`)))
	}

	malformed := []*validation.Validation{
		{KindValidation: validation.KindValidation{Type: "unknown"}},
		{EnumValidation: validation.EnumValidation{Values: []json.RawMessage{json.RawMessage(`1`)}}},
		{NumberValidation: validation.NumberValidation{Minimum: &validation.NumberBound{Value: "1"}}},
		{StringValidation: validation.StringValidation{MinLength: &validation.CountBound{Value: "1"}}},
		{StringValidation: validation.StringValidation{Pattern: "a"}},
	}
	for _, compiled := range malformed {
		require.NotEmpty(t, compiled.Validate(json.RawMessage(`null`)))
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

func TestMustCompileStringFormatAdvertisesItsPanicBoundary(t *testing.T) {
	t.Parallel()

	require.NotNil(t, validation.MustCompileStringFormat("date"))
	require.Panics(t, func() { validation.MustCompileStringFormat("unknown") })
}

func TestParseReturnsMalformedPatternErrors(t *testing.T) {
	t.Parallel()

	for _, spec := range []string{
		`openapi: 3.0.3
paths:
  /body:
    post:
      operationId: body
      requestBody:
        content:
          application/json:
            schema: {type: string, pattern: '['}
`,
		`openapi: 3.0.3
paths:
  /{id}:
    get:
      operationId: path
      parameters:
        - {name: id, in: path, required: true, schema: {type: string, pattern: '['}}
`,
		`openapi: 3.0.3
paths:
  /query:
    get:
      operationId: query
      parameters:
        - {name: q, in: query, schema: {type: string, pattern: '['}}
`,
	} {
		parsed, err := validation.Parse([]byte(spec))
		require.Error(t, err)
		require.Nil(t, parsed)
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
