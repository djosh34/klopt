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
		{name: "string numeric", schema: `{"type":"string","format":"int32"}`, contains: "invalid type/format pair"},
		{name: "string floating", schema: `{"type":"string","format":"double"}`, contains: "invalid type/format pair"},
		{name: "boolean string", schema: `{"type":"boolean","format":"date"}`, contains: "invalid type/format pair"},
		{name: "boolean numeric", schema: `{"type":"boolean","format":"int64"}`, contains: "invalid type/format pair"},
		{name: "integer floating", schema: `{"type":"integer","format":"float"}`, contains: "invalid type/format pair"},
		{name: "integer string", schema: `{"type":"integer","format":"uuid"}`, contains: "invalid type/format pair"},
		{name: "number string", schema: `{"type":"number","format":"email"}`, contains: "invalid type/format pair"},
		{name: "array string", schema: `{"type":"array","format":"email","items":{}}`, contains: "invalid type/format pair"},
		{name: "object numeric", schema: `{"type":"object","format":"int32"}`, contains: "invalid type/format pair"},
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
		{
			format: "uuidv4", valid: `"a1234567-1234-4234-9234-123456789abc"`,
			invalid: `"a1234567-1234-1234-9234-123456789abc"`,
		},
		{
			format: "uuid-v4", valid: `"a1234567-1234-4234-9234-123456789abc"`,
			invalid: `"a1234567-1234-1234-9234-123456789abc"`,
		},
		{format: "ipv4", valid: `"10.0.0.1"`, invalid: `"010.0.0.1"`},
		{format: "cidr", valid: `"192.0.2.7/24"`, invalid: `"192.0.2.7/33"`},
		{format: "ipv4-cidr", valid: `"192.0.2.7/24"`, invalid: `"192.0.2.7/33"`},
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

func TestRuntimeLocksTypelessFormatsToTheirNativeJSONKinds(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		format  string
		valid   string
		invalid string
	}{
		{format: "byte", valid: `"YQ=="`, invalid: `"YQ="`},
		{format: "date", valid: `"2024-02-29"`, invalid: `"2024-02-30"`},
		{format: "date-time", valid: `"2026-07-14T12:30:00Z"`, invalid: `"2026-07-14T12:30:00,5Z"`},
		{format: "email", valid: `"a@example.com"`, invalid: `"a..b@example.com"`},
		{format: "ipv4", valid: `"192.0.2.1"`, invalid: `"192.0.2.01"`},
		{format: "uuid", valid: `"a1234567-1234-4234-9234-123456789abc"`, invalid: `"a1234567-1234-1234-9234-123456789abc"`},
		{
			format: "uuidv4", valid: `"a1234567-1234-4234-9234-123456789abc"`,
			invalid: `"a1234567-1234-1234-9234-123456789abc"`,
		},
		{
			format: "uuid-v4", valid: `"a1234567-1234-4234-9234-123456789abc"`,
			invalid: `"a1234567-1234-1234-9234-123456789abc"`,
		},
		{format: "cidr", valid: `"192.0.2.7/24"`, invalid: `"192.0.2.7/33"`},
		{format: "ipv4-cidr", valid: `"192.0.2.7/24"`, invalid: `"192.0.2.7/33"`},
	} {
		t.Run(test.format, func(t *testing.T) {
			t.Parallel()

			validation := mustParseSchema(t, fmt.Sprintf(`{"format":%q}`, test.format), "")
			require.Empty(t, validation.KindValidation.Type)
			require.Empty(t, validation.Validate(json.RawMessage(test.valid)))
			require.ErrorContains(t, errors.Join(validation.Validate(json.RawMessage(test.invalid))...), "keyword format")

			for _, otherKind := range []string{`null`, `false`, `1`, `[]`, `{}`} {
				require.Empty(t, validation.Validate(json.RawMessage(otherKind)), otherKind)
			}
		})
	}

	for _, test := range []struct {
		format  string
		valid   string
		invalid string
	}{
		{format: "int32", valid: "2147483647", invalid: "2147483648"},
		{format: "int64", valid: "9223372036854775807", invalid: "9223372036854775808"},
		{format: "float", valid: "1", invalid: "1e40"},
		{format: "double", valid: "1e40", invalid: "1e400"},
	} {
		t.Run("number/"+test.format, func(t *testing.T) {
			t.Parallel()

			validation := mustParseSchema(t, fmt.Sprintf(`{"format":%q}`, test.format), "")
			require.Empty(t, validation.KindValidation.Type)
			require.Empty(t, validation.Validate(json.RawMessage(test.valid)))
			require.ErrorContains(t, errors.Join(validation.Validate(json.RawMessage(test.invalid))...), "keyword format")
			require.Empty(t, validation.Validate(json.RawMessage(`"not a number"`)))
		})
	}

	password := mustParseSchema(t, `{"format":"password"}`, "")
	require.Empty(t, password.KindValidation.Type)

	for _, value := range []string{`"anything"`, `""`, `1`, `null`, `{}`} {
		require.Empty(t, password.Validate(json.RawMessage(value)), value)
	}
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
			typeName: "integer", format: "int32",
			valid: []string{"1", "1.0", "-2147483648", "2147483647"}, invalid: []string{"1.5", "2147483648"},
		},
		{
			typeName: "integer", format: "int64",
			valid:   []string{"-9223372036854775808", "9223372036854775807"},
			invalid: []string{"1.5", "9223372036854775808"},
		},
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
		t.Run(test.typeName+"/"+test.format, func(t *testing.T) {
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
