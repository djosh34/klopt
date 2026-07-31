//nolint:godoclint // Package-private tests document the valid Decode contract.
package testgenerator

import (
	"fmt"
	"testing"

	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/stretchr/testify/require"
)

func TestDecodeValidEmptyAndShortTapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		want   string
	}{
		{name: "empty string", schema: `{"type":"string"}`, want: `""`},
		{name: "empty array", schema: `{"type":"array","items":{"type":"integer"}}`, want: `[]`},
		{name: "empty object", schema: `{"type":"object"}`, want: `{}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			generator := mustCompileGenerator(t, test.schema)
			for _, tape := range [][]byte{nil, {0}} {
				sample, status, err := generator.Decode(tape)
				require.NoError(t, err)
				require.Equal(t, Generated, status)
				require.Equal(t, "request", sample.OperationID)
				require.True(t, sample.ExpectValid)
				require.Equal(t, test.want, string(sample.Body))
			}
		})
	}
}

func TestDecodeValidAnyOfKeepsLocalRules(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{
		"type":"string",
		"minLength":1,
		"anyOf":[{"enum":["a"]},{"enum":["b"]}]
	}`)

	sample, status, err := generator.Decode(nil)
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.Equal(t, `"a"`, string(sample.Body))

	holds, err := expressionHolds(generator.operations[0].root, mustValue(t, sample.Body))
	require.NoError(t, err)
	require.True(t, holds)
}

func TestDecodeValidAllOfPassesEveryChild(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{
		"type":"number",
		"allOf":[{"minimum":0},{"maximum":10}]
	}`)

	sample, status, err := generator.Decode(nil)
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.Equal(t, `0`, string(sample.Body))
}

func TestDecodeValidShortTapeHonorsRequiredMinimums(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{
		"type":"object",
		"minProperties":2,
		"required":["name"],
		"properties":{"name":{"type":"string"},"enabled":{"type":"boolean"}},
		"additionalProperties":false
	}`)

	for _, tape := range [][]byte{nil, {0}} {
		sample, status, err := generator.Decode(tape)
		require.NoError(t, err)
		require.Equal(t, Generated, status)
		require.JSONEq(t, `{"name":"","enabled":false}`, string(sample.Body))
	}
}

func TestDecodeValidZeroTailStopsOptionalOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		want   string
	}{
		{name: "string", schema: `{"type":"string"}`, want: `""`},
		{name: "array", schema: `{"type":"array","items":{"type":"integer"}}`, want: `[]`},
		{
			name:   "known properties",
			schema: `{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"boolean"}},"additionalProperties":false}`,
			want:   `{}`,
		},
		{name: "additional properties", schema: `{"type":"object"}`, want: `{}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			generator := mustCompileGenerator(t, test.schema)
			sample, status, err := generator.Decode(nil)
			require.NoError(t, err)
			require.Equal(t, Generated, status)
			require.Equal(t, test.want, string(sample.Body))
		})
	}
}

func TestDecodeSameValidTapeIsDeterministic(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{
		"type":"object",
		"properties":{"value":{"type":"boolean"}},
		"additionalProperties":false
	}`)
	tape := []byte{1, 2, 3, 4, 5}

	first, firstStatus, firstErr := generator.Decode(tape)
	second, secondStatus, secondErr := generator.Decode(tape)

	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.Equal(t, firstStatus, secondStatus)
	require.Equal(t, first, second)
}

func mustCompileGenerator(t *testing.T, schema string) *Generator {
	t.Helper()

	document := []byte(fmt.Sprintf(`{
		"openapi":"3.0.4",
		"info":{"title":"test","version":"1"},
		"paths":{"/things":{"post":{
			"operationId":"request",
			"requestBody":{"content":{"application/json":{"schema":%s}}},
			"responses":{"204":{"description":"ok"}}
		}}}
	}`, schema))

	generator, err := Compile(document)
	require.NoError(t, err)

	return generator
}

func mustValue(t *testing.T, raw []byte) jsonvalue.Value {
	t.Helper()

	value, err := jsonvalue.Parse(raw)
	require.NoError(t, err)

	return value
}
