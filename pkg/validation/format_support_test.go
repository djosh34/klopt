//nolint:godoclint // Public-seam test names are intentionally descriptive.
package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseEnforcesTheClosedFormatTypeContract(t *testing.T) {
	t.Parallel()

	const unsupported = "legal OpenAPI but unsupported by this tool"

	for _, test := range []struct {
		typeName string
		format   string
	}{
		{typeName: "integer", format: "int32"},
		{typeName: "integer", format: "int64"},
		{typeName: "number", format: "int32"},
		{typeName: "number", format: "int64"},
		{typeName: "number", format: "float"},
		{typeName: "number", format: "double"},
		{typeName: "string", format: "uuid"},
		{typeName: "string", format: "uuidv4"},
		{typeName: "string", format: "uuid-v4"},
		{typeName: "string", format: "ipv4"},
		{typeName: "string", format: "cidr"},
		{typeName: "string", format: "ipv4-cidr"},
		{typeName: "string", format: "email"},
		{typeName: "string", format: "byte"},
		{typeName: "string", format: "date"},
		{typeName: "string", format: "date-time"},
		{typeName: "string", format: "password"},
	} {
		t.Run(test.typeName+"/"+test.format, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(openAPISpec(fmt.Sprintf(`{"type":%q,"format":%q}`, test.typeName, test.format), "", false))
			require.NoError(t, err)
		})
	}

	for _, test := range []struct {
		name     string
		schema   string
		contains string
	}{
		{name: "unknown", schema: `{"type":"string","format":"hostname"}`, contains: unsupported},
		{name: "binary", schema: `{"type":"string","format":"binary"}`, contains: unsupported},
		{name: "wrong pair", schema: `{"type":"string","format":"int32"}`, contains: "invalid type/format pair"},
		{name: "wrong scalar", schema: `{"type":"boolean","format":"date"}`, contains: "invalid type/format pair"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(openAPISpec(test.schema, "", false))
			require.Error(t, err)
			require.ErrorContains(t, err, "/format")
			require.ErrorContains(t, err, test.contains)
		})
	}
}

func TestRuntimeEnforcesNativeStringFormatsAndRecursiveAllOf(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		format  string
		valid   string
		invalid string
	}{
		{format: "uuid", valid: `"a1234567-1234-4234-9234-123456789abc"`, invalid: `"a1234567-1234-1234-9234-123456789abc"`},
		{format: "ipv4", valid: `"10.0.0.1"`, invalid: `"010.0.0.1"`},
		{format: "cidr", valid: `"192.0.2.7/24"`, invalid: `"192.0.2.7/33"`},
		{format: "email", valid: `"\"John Doe\"@example.com"`, invalid: `"a..b@example.com"`},
		{format: "byte", valid: `"YQ=="`, invalid: `"%%%"`},
		{format: "date", valid: `"2024-02-29"`, invalid: `"2024-02-30"`},
		{format: "date-time", valid: `"2026-07-14T12:30:00Z"`, invalid: `"2026-07-14Z"`},
	} {
		t.Run(test.format, func(t *testing.T) {
			t.Parallel()
			validation := mustParseSchema(t, fmt.Sprintf(`{"type":"string","allOf":[{"format":%q}]}`, test.format), "")
			require.Empty(t, validation.Validate(json.RawMessage(test.valid)))
			joined := errors.Join(validation.Validate(json.RawMessage(test.invalid))...)
			require.Error(t, joined)
			require.ErrorContains(t, joined, "keyword format")
		})
	}

	password := mustParseSchema(
		t,
		`{"type":"string","format":"password","pattern":"^a$","minLength":1,"maxLength":1,"enum":["a"]}`,
		"",
	)
	require.Empty(t, password.Validate(json.RawMessage(`"a"`)))
	require.NotEmpty(t, password.Validate(json.RawMessage(`"b"`)))

	dateTime := mustParseSchema(t, `{"type":"string","format":"date-time"}`, "")
	commaFraction := errors.Join(dateTime.Validate(json.RawMessage(`"2026-07-14T12:30:00,5+02:30"`))...)
	require.Error(t, commaFraction)
	require.ErrorContains(t, commaFraction, "keyword format")
}

func TestRuntimeEnforcesNumericFormatsExactly(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		typeName string
		format   string
		valid    []string
		invalid  []string
	}{
		{
			typeName: "number", format: "int32",
			valid: []string{"1", "1.0", "-2147483648", "2147483647"}, invalid: []string{"1.5", "2147483648"},
		},
		{
			typeName: "number", format: "int64",
			valid:   []string{"-9223372036854775808", "9223372036854775807"},
			invalid: []string{"1.5", "9223372036854775808"},
		},
		{typeName: "number", format: "float", valid: []string{"0.1"}, invalid: []string{"1e40"}},
		{typeName: "number", format: "double", valid: []string{"0.1", "1e40"}, invalid: []string{"1e400"}},
	} {
		t.Run(test.format, func(t *testing.T) {
			t.Parallel()

			validation := mustParseSchema(t, fmt.Sprintf(`{"type":%q,"format":%q}`, test.typeName, test.format), "")
			for _, value := range test.valid {
				require.Empty(t, validation.Validate(json.RawMessage(value)), value)
			}

			for _, value := range test.invalid {
				joined := errors.Join(validation.Validate(json.RawMessage(value))...)
				require.Error(t, joined, value)
				require.ErrorContains(t, joined, "keyword format", value)
			}
		})
	}
}
