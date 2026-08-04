//nolint:godoclint,lll // String-oracle tables keep clean applicability and format boundaries together.
package schematest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluateStringLengthsCountRunesAndIgnoreWrongKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   string
		value    string
		valid    bool
		observed []string
		failures []string
	}{
		{
			name:     "unicode length is two runes",
			schema:   `{"type":"string","minLength":2,"maxLength":2}`,
			value:    `"😀a"`,
			valid:    true,
			observed: []string{"string", "valid", "valid"},
		},
		{
			name:     "unicode short value",
			schema:   `{"type":"string","minLength":2}`,
			value:    `"😀"`,
			valid:    false,
			observed: []string{"string"},
			failures: []string{"minLength"},
		},
		{
			name:     "unicode long value",
			schema:   `{"type":"string","maxLength":2}`,
			value:    `"😀ab"`,
			valid:    false,
			observed: []string{"string"},
			failures: []string{"maxLength"},
		},
		{
			name:     "wrong kind is inapplicable",
			schema:   `{"minLength":2,"maxLength":2}`,
			value:    "123",
			valid:    true,
			observed: []string{"number"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, test.schema, test.value)
			require.NoError(t, result.err)
			require.Equal(t, test.valid, result.valid)
			require.Equal(t, test.observed, observedLevels(result.observed))
			require.Equal(t, test.failures, failureRules(result.failures))
		})
	}
}

func TestEvaluateEveryStringRuleKeepsItsOwnVerdictAndIdentity(t *testing.T) {
	t.Parallel()

	schema := `{"type":"string","minLength":3,"maxLength":4,"pattern":"^a","format":"date"}`
	first := evaluateSchemaValue(t, schema, `"b"`)
	second := evaluateSchemaValue(t, schema, `"b"`)

	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.Equal(t, first, second)
	require.False(t, first.valid)
	require.Equal(t, []string{"type", "minLength", "maxLength", "pattern", "format"}, applicableRules(first.applicable))
	require.Equal(t, []string{"string", "valid"}, observedLevels(first.observed))
	require.Equal(t, []string{"minLength", "pattern", "format"}, failureRules(first.failures))
	require.Equal(t, first.applicable[1].String(), first.failures[0].String())
	require.Equal(t, first.applicable[3].String(), first.failures[1].String())
	require.Equal(t, first.applicable[4].String(), first.failures[2].String())
}

func TestEvaluatePatternsRemainIndependentWhenOneFails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pattern  string
		value    string
		valid    bool
		observed []string
		failures []string
	}{
		{name: "positive match", pattern: `^a`, value: `"abc"`, valid: true, observed: []string{"string", "valid"}},
		{name: "negative match", pattern: `^a`, value: `"xbc"`, valid: false, observed: []string{"string"}, failures: []string{"pattern"}},
		{name: "unicode literal", pattern: `^😀$`, value: `"😀"`, valid: true, observed: []string{"string", "valid"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, fmt.Sprintf(`{"type":"string","pattern":%q}`, test.pattern), test.value)
			require.NoError(t, result.err)
			require.Equal(t, test.valid, result.valid)
			require.Equal(t, test.observed, observedLevels(result.observed))
			require.Equal(t, test.failures, failureRules(result.failures))
		})
	}
}

func TestEvaluateStringFormatsUseNativeApplicabilityAndBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		format  string
		valid   string
		invalid string
	}{
		{name: "byte", format: "byte", valid: `"YQ=="`, invalid: `"YQ="`},
		{name: "date", format: "date", valid: `"2024-02-29"`, invalid: `"2024-02-30"`},
		{name: "date-time", format: "date-time", valid: `"2026-07-14T12:30:00Z"`, invalid: `"2026-07-14T24:00:00Z"`},
		{name: "email", format: "email", valid: `"a@example.com"`, invalid: `"a..b@example.com"`},
		{name: "ipv4", format: "ipv4", valid: `"192.0.2.1"`, invalid: `"192.0.2.01"`},
		{name: "uuid", format: "uuid", valid: `"a1234567-1234-4234-9234-123456789abc"`, invalid: `"a1234567-1234-1234-9234-123456789abc"`},
		{name: "uuidv4", format: "uuidv4", valid: `"a1234567-1234-4234-9234-123456789abc"`, invalid: `"a1234567-1234-4234-7234-123456789abc"`},
		{name: "uuid-v4", format: "uuid-v4", valid: `"a1234567-1234-4234-9234-123456789abc"`, invalid: `"a1234567-1234-4234-7234-123456789abc"`},
		{name: "cidr", format: "cidr", valid: `"192.0.2.7/24"`, invalid: `"192.0.2.7/33"`},
		{name: "ipv4-cidr", format: "ipv4-cidr", valid: `"192.0.2.7/24"`, invalid: `"192.0.2.7/33"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for _, valueTest := range []struct {
				name  string
				value string
				valid bool
			}{
				{name: "valid", value: test.valid, valid: true},
				{name: "invalid", value: test.invalid},
			} {
				t.Run(valueTest.name, func(t *testing.T) {
					t.Parallel()

					result := evaluateSchemaValue(t, fmt.Sprintf(`{"type":"string","format":%q}`, test.format), valueTest.value)
					require.NoError(t, result.err)
					require.Equal(t, valueTest.valid, result.valid)
					require.Equal(t, []string{"type", "format"}, applicableRules(result.applicable))

					if valueTest.valid {
						require.Equal(t, []string{"string", "valid"}, observedLevels(result.observed))
						require.Empty(t, result.failures)
					} else {
						require.Equal(t, []string{"string"}, observedLevels(result.observed))
						require.Equal(t, []string{"format"}, failureRules(result.failures))
					}
				})
			}
		})
	}
}

func TestEvaluateCleanPatternFamiliesUseIndependentMatchingState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		value   string
		valid   bool
	}{
		{name: "search", pattern: "a", value: `"xxa"`, valid: true},
		{name: "alternation", pattern: "a|b", value: `"xxb"`, valid: true},
		{name: "class", pattern: `[a-c]+`, value: `"xxbc"`, valid: true},
		{name: "negated class", pattern: `[^a]+`, value: `"aaa"`, valid: false},
		{name: "counted repeat", pattern: `^a{2,3}$`, value: `"aaa"`, valid: true},
		{name: "lazy repeat", pattern: `^a+?$`, value: `"aaa"`, valid: true},
		{name: "dot excludes line terminator", pattern: `^.$`, value: `"\n"`, valid: false},
		{name: "word boundary", pattern: `\bword\b`, value: `"a word!"`, valid: true},
		{name: "positive leading assertion", pattern: `^(?=a)a`, value: `"abc"`, valid: true},
		{name: "negative leading assertion", pattern: `^(?!b)a`, value: `"abc"`, valid: true},
		{name: "failed negative leading assertion", pattern: `^(?!a)a`, value: `"abc"`, valid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, fmt.Sprintf(`{"type":"string","pattern":%q}`, test.pattern), test.value)
			require.NoError(t, result.err)
			require.Equal(t, test.valid, result.valid)
		})
	}
}

func TestEvaluateStringFormatBoundariesRemainDeterministic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		format  string
		valid   []string
		invalid []string
	}{
		{
			name: "byte", format: "byte",
			valid:   []string{``, `YQ==`, `YWI=`, `YWJj`, `+///`},
			invalid: []string{`%%%`, `YQ`, `YQ=`, `YR==`, `YWJ=`},
		},
		{
			name: "date", format: "date",
			valid:   []string{"0000-02-29", "1904-02-29", "2000-02-29", "2024-02-29"},
			invalid: []string{"1900-02-29", "2024-02-30", "2026-7-14", "2026-07-14Z"},
		},
		{
			name: "date-time", format: "date-time",
			valid:   []string{"2026-07-14T12:30:00Z", "2024-02-29T23:59:59.123Z", "2026-07-14T12:30:00+23:59"},
			invalid: []string{"2026-07-14Z", "2024-02-30T12:00:00Z", "2026-07-14t12:30:00z", "2026-07-14T12:30:00,5+02:30"},
		},
		{
			name: "email", format: "email",
			valid:   []string{"first.last@example.com", `"John Doe"@example.com`, "postmaster@[192.0.2.1]", "postmaster@[IPv6:2001:db8::1]", "postmaster@[TAG:value]", "postmaster@[ABCD:value]"},
			invalid: []string{"a..b@example.com", ".a@example.com", "a@-example.com", "a@[256.0.0.1]", "a@[IPv6:2001:::1]", `"a".example.com`},
		},
		{
			name: "ipv4", format: "ipv4",
			valid:   []string{"0.0.0.0", "10.0.0.1", "255.255.255.255"},
			invalid: []string{"010.0.0.1", "256.0.0.1", "1.2.3", "1.2.3.4.5"},
		},
		{
			name: "cidr", format: "cidr",
			valid:   []string{"0.0.0.0/0", "192.0.2.0/24", "192.0.2.7/24", "255.255.255.255/32"},
			invalid: []string{"192.0.2.7", "192.0.2.7/33", "192.0.02.7/24"},
		},
		{
			name: "uuid", format: "uuid",
			valid:   []string{"a1234567-1234-4234-8234-123456789abc", "A1234567-1234-4234-B234-123456789ABC"},
			invalid: []string{"a1234567-1234-1234-8234-123456789abc", "a1234567-1234-4234-7234-123456789abc", "{a1234567-1234-4234-8234-123456789abc}"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for _, value := range test.valid {
				result := evaluateSchemaValue(t, fmt.Sprintf(`{"type":"string","format":%q}`, test.format), fmt.Sprintf(`%q`, value))
				require.NoError(t, result.err)
				require.True(t, result.valid, value)
			}

			for _, value := range test.invalid {
				result := evaluateSchemaValue(t, fmt.Sprintf(`{"type":"string","format":%q}`, test.format), fmt.Sprintf(`%q`, value))
				require.NoError(t, result.err)
				require.False(t, result.valid, value)
			}
		})
	}
}

func TestEvaluateTypelessStringFormatDoesNotInferType(t *testing.T) {
	t.Parallel()

	stringValue := evaluateSchemaValue(t, `{"format":"date"}`, `"2024-02-29"`)
	numberValue := evaluateSchemaValue(t, `{"format":"date"}`, "1")

	require.NoError(t, stringValue.err)
	require.NoError(t, numberValue.err)
	require.True(t, stringValue.valid)
	require.True(t, numberValue.valid)
	require.Equal(t, []string{"type", "format"}, applicableRules(stringValue.applicable))
	require.Equal(t, []string{"type"}, applicableRules(numberValue.applicable))
	require.Equal(t, []string{"string", "valid"}, observedLevels(stringValue.observed))
	require.Equal(t, []string{"number"}, observedLevels(numberValue.observed))
}

func TestEvaluatePasswordFormatIsInert(t *testing.T) {
	t.Parallel()

	for _, value := range []string{`"anything"`, `""`, `"😀"`} {
		result := evaluateSchemaValue(t, `{"type":"string","format":"password"}`, value)
		require.NoError(t, result.err)
		require.True(t, result.valid)
		require.Equal(t, []string{"type"}, applicableRules(result.applicable))
		require.Equal(t, []string{"string"}, observedLevels(result.observed))
		require.Empty(t, result.failures)
	}
}

func TestEvaluateStringRuleTablesAreDeterministic(t *testing.T) {
	t.Parallel()

	schema := `{"type":"string","minLength":2,"maxLength":3,"pattern":"^a","format":"date"}`
	values := []string{`"a"`, `"abc"`, `"2024-02-29"`, `"b"`, `1`, `null`}
	first := make([]evaluation, 0, len(values))
	second := make([]evaluation, 0, len(values))

	for _, value := range values {
		first = append(first, evaluateSchemaValue(t, schema, value))
		second = append(second, evaluateSchemaValue(t, schema, value))
	}

	require.Equal(t, first, second)
	require.Equal(t, "minLength,format", strings.Join(failureRules(first[0].failures), ","))
}
