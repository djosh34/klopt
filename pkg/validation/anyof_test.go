//nolint:godoclint // Behavior tests use descriptive test names as their specification.
package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
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

func TestSharedFailingAnyOfGraphValidatesOncePerNode(t *testing.T) {
	t.Parallel()

	shared := &Validation{KindValidation: KindValidation{Type: "string"}}
	for range 24 {
		shared = &Validation{AnyOfValidations: []*Validation{shared, shared}}
	}

	errs := shared.Validate(json.RawMessage(`false`))
	require.Len(t, errs, 1)
	require.ErrorContains(t, errs[0], "keyword anyOf")
}

//nolint:paralleltest // Per-process allocation counts must run without concurrent tests.
func TestAnyOfDecodesEachLargeInstanceOnce(t *testing.T) {
	body := json.RawMessage(`{"payload":"` + strings.Repeat("x", 1<<18) + `"}`)

	allocatedBytes := func(branches int) int64 {
		children := make([]*Validation, branches)
		for index := range children {
			children[index] = &Validation{KindValidation: KindValidation{Type: "string"}}
		}

		validation := &Validation{
			ObjectValidation: ObjectValidation{AdditionalPropertiesAllowed: true},
			AnyOfValidations: children,
		}
		result := testing.Benchmark(func(benchmark *testing.B) {
			for benchmark.Loop() {
				if errs := validation.Validate(body); len(errs) != 1 {
					panic(fmt.Sprintf("got %d diagnostics, want 1", len(errs)))
				}
			}
		})

		return result.AllocedBytesPerOp()
	}

	single := allocatedBytes(1)
	many := allocatedBytes(8)
	require.Less(t, many, single*2)
}

func TestAnyOfMemoKeysDoNotRetainRawRequestValues(t *testing.T) {
	t.Parallel()

	_, retainsRaw := reflect.TypeFor[validationMemoKey]().FieldByName("raw")
	require.False(t, retainsRaw)
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
