//nolint:godoclint // Behavior tests use descriptive test names as their specification.
package validation

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAndValidateAnyOfWithActiveSiblings(t *testing.T) {
	t.Parallel()

	validation := mustParseSchema(t, `{
		"type":"string",
		"minLength":2,
		"allOf":[{"maxLength":5}],
		"anyOf":[{"pattern":"^a"},{"pattern":"z$"}]
	}`, "")

	require.Len(t, validation.AnyOfValidations, 2)
	require.Empty(t, validation.Validate(json.RawMessage(`"ab"`)))
	require.Empty(t, validation.Validate(json.RawMessage(`"zz"`)))

	for _, body := range []string{`"x"`, `"abcdefz"`, `7`} {
		require.NotEmpty(t, validation.Validate(json.RawMessage(body)), body)
	}
}

func TestAnyOfAllFailDiagnosticsFollowLocalAllOfThenCompositionOrder(t *testing.T) {
	t.Parallel()

	validation := mustParseSchema(t, `{
		"type":"string",
		"minLength":3,
		"allOf":[{"maxLength":1}],
		"anyOf":[{"pattern":"^a"},{"pattern":"z$"}]
	}`, "")

	errs := validation.Validate(json.RawMessage(`"xy"`))
	require.Len(t, errs, 3)
	require.ErrorContains(t, errs[0], "keyword minLength")
	require.ErrorContains(t, errs[1], "/allOf/0")
	require.ErrorContains(t, errs[1], "keyword maxLength")
	require.ErrorContains(t, errs[2], "keyword anyOf")
}

func TestAnyOfWorksRecursivelyAndThroughReferences(t *testing.T) {
	t.Parallel()

	validation := mustParseSchema(t, `{
		"type":"object",
		"properties":{"items":{"type":"array","items":{"$ref":"#/components/schemas/Choice"}}},
		"additionalProperties":{"$ref":"#/components/schemas/Choice"}
	}`, `,"components":{"schemas":{"Choice":{"anyOf":[{"type":"integer"},{"type":"string"}]}}}`)

	require.Empty(t, validation.Validate(json.RawMessage(`{"items":[1,"x"],"extra":2}`)))
	errs := validation.Validate(json.RawMessage(`{"items":[true],"extra":false}`))
	require.Len(t, errs, 2)
	require.Contains(t, errors.Join(errs...).Error(), "instance #/items/0")
	require.Contains(t, errors.Join(errs...).Error(), "instance #/extra")
}

func TestParseRejectsMalformedAnyOfAtExactPointer(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
		want   string
	}{
		{name: "null", schema: `{"anyOf":null}`, want: "/anyOf"},
		{name: "object", schema: `{"anyOf":{}}`, want: "/anyOf"},
		{name: "empty", schema: `{"anyOf":[]}`, want: "/anyOf"},
		{name: "branch scalar", schema: `{"anyOf":[1]}`, want: "/anyOf/0"},
		{
			name: "nested unsupported", schema: `{"anyOf":[{"properties":{"x":{"not":{}}}}]}`,
			want: "/anyOf/0/properties/x/not",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(openAPISpec(test.schema, "", false))
			require.ErrorContains(t, err, test.want)
		})
	}
}
