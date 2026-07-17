package validation_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/validation"

	"github.com/stretchr/testify/require"
)

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
	composite(unsealed)
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

	parsed, _, err := validation.Parse(spec, composite)
	require.NoError(t, err)

	compiled := parsed["request"].StringValidation.CompiledPattern
	require.True(t, compiled.RejectsNonASCII())
	require.True(t, compiled.UsesRE2())
	require.Panics(t, func() { composite(compiled) })
	require.True(t, compiled.RejectsNonASCII())
	require.True(t, compiled.UsesRE2())

	require.Panics(t, func() { validation.PatternOptions(nil) })

	_, _, err = validation.Parse(spec, nil)
	require.EqualError(t, err, strings.Join([]string{
		"compile operationId \"request\": compile schema at ",
		"#/paths/~1request/post/requestBody/content/application~1json/schema/pattern: ",
		"patternvalidator: nil option",
	}, ""))
}
