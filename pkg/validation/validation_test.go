package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/stretchr/testify/require"
)

// TestValidationSupportedKeywordsAtRootNestedAndAllOf covers every runtime rule at each schema shape.
func TestValidationSupportedKeywordsAtRootNestedAndAllOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		valid   string
		invalid string
		keyword string
	}{
		{name: "type", schema: `{"type":"boolean"}`, valid: `true`, invalid: `0`, keyword: "type"},
		{name: "integer", schema: `{"type":"integer"}`, valid: `9007199254740993`, invalid: `1.5`, keyword: "type"},
		{name: "nullable", schema: `{"type":"string","nullable":true}`, valid: `null`, invalid: `1`, keyword: "type"},
		{name: "enum", schema: `{"enum":[1,{"a":2}]}`, valid: `1.0`, invalid: `2`, keyword: "enum"},
		{name: "minimum", schema: `{"minimum":1}`, valid: `1`, invalid: `0`, keyword: "minimum"},
		{
			name: "exclusiveMinimum", schema: `{"minimum":1,"exclusiveMinimum":true}`,
			valid: `2`, invalid: `1`, keyword: "exclusiveMinimum",
		},
		{name: "maximum", schema: `{"maximum":1}`, valid: `1`, invalid: `2`, keyword: "maximum"},
		{
			name: "exclusiveMaximum", schema: `{"maximum":1,"exclusiveMaximum":true}`,
			valid: `0`, invalid: `1`, keyword: "exclusiveMaximum",
		},
		{name: "multipleOf", schema: `{"multipleOf":0.1}`, valid: `0.3`, invalid: `0.31`, keyword: "multipleOf"},
		{name: "minLength", schema: `{"minLength":2}`, valid: `"λx"`, invalid: `"λ"`, keyword: "minLength"},
		{name: "maxLength", schema: `{"maxLength":1}`, valid: `"λ"`, invalid: `"λx"`, keyword: "maxLength"},
		{name: "pattern", schema: `{"pattern":"^a+$"}`, valid: `"aa"`, invalid: `"b"`, keyword: "pattern"},
		{name: "format", schema: `{"format":"date"}`, valid: `"2026-07-14"`, invalid: `"2026-02-30"`, keyword: "format"},
		{name: "minItems", schema: `{"minItems":1}`, valid: `[0]`, invalid: `[]`, keyword: "minItems"},
		{name: "maxItems", schema: `{"maxItems":1}`, valid: `[0]`, invalid: `[0,1]`, keyword: "maxItems"},
		{name: "items", schema: `{"items":{"type":"integer"}}`, valid: `[1]`, invalid: `[1.5]`, keyword: "type"},
		{name: "minProperties", schema: `{"minProperties":1}`, valid: `{"a":1}`, invalid: `{}`, keyword: "minProperties"},
		{
			name: "maxProperties", schema: `{"maxProperties":1}`,
			valid: `{"a":1}`, invalid: `{"a":1,"b":2}`, keyword: "maxProperties",
		},
		{name: "required", schema: `{"required":["a"]}`, valid: `{"a":1}`, invalid: `{}`, keyword: "required"},
		{
			name: "properties", schema: `{"properties":{"a":{"type":"string"}}}`,
			valid: `{"a":"x"}`, invalid: `{"a":1}`, keyword: "type",
		},
		{
			name: "additionalPropertiesFalse", schema: `{"additionalProperties":false}`,
			valid: `{}`, invalid: `{"a":1}`, keyword: "additionalProperties",
		},
		{
			name: "additionalPropertiesSchema", schema: `{"additionalProperties":{"type":"string"}}`,
			valid: `{"a":"x"}`, invalid: `{"a":1}`, keyword: "type",
		},
	}

	shapes := []struct {
		name        string
		wrapSchema  func(string) string
		wrapBody    func(string) string
		wantPointer string
	}{
		{name: "root", wrapSchema: identity, wrapBody: identity, wantPointer: "instance #"},
		{
			name: "nested",
			wrapSchema: func(schema string) string {
				return fmt.Sprintf(`{"type":"object","required":["value"],"properties":{"value":%s}}`, schema)
			},
			wrapBody:    func(body string) string { return fmt.Sprintf(`{"value":%s}`, body) },
			wantPointer: "instance #/value",
		},
		{
			name:        "allOf",
			wrapSchema:  func(schema string) string { return fmt.Sprintf(`{"allOf":[%s]}`, schema) },
			wrapBody:    identity,
			wantPointer: "schema #/paths/~1things/post/requestBody/content/application~1json/schema/allOf/0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for _, shape := range shapes {
				t.Run(shape.name, func(t *testing.T) {
					t.Parallel()

					parsed := mustParseSchema(t, shape.wrapSchema(test.schema), "")
					require.Empty(t, parsed.Validate(json.RawMessage(shape.wrapBody(test.valid))))

					errs := parsed.Validate(json.RawMessage(shape.wrapBody(test.invalid)))
					require.NotEmpty(t, errs)
					require.Contains(t, errors.Join(errs...).Error(), "keyword "+test.keyword)
					require.Contains(t, errors.Join(errs...).Error(), shape.wantPointer)
				})
			}
		})
	}
}

// TestValidationLocksNullabilityAndTypeSpecificApplicability names every null rule outcome.
func TestValidationLocksNullabilityAndTypeSpecificApplicability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		schema       string
		body         string
		valid        bool
		wantType     string
		wantNullable bool
		wantKeywords []string
	}{
		{name: "typeless nullable absent admits null", schema: `{}`, body: `null`, valid: true},
		{name: "typeless nullable false admits null", schema: `{"nullable":false}`, body: `null`, valid: true},
		{name: "typeless nullable true admits null", schema: `{"nullable":true}`, body: `null`, valid: true},
		{
			name: "typeless enum containing null admits null", schema: `{"enum":[null,false]}`,
			body: `null`, valid: true,
		},
		{
			name: "typeless enum excluding null rejects by enum", schema: `{"enum":[false]}`,
			body: `null`, wantKeywords: []string{"enum"},
		},
		{
			name: "explicit nonnullable boolean rejects by type", schema: `{"type":"boolean"}`,
			body: `null`, wantType: "boolean", wantKeywords: []string{"type"},
		},
		{
			name: "explicit nonnullable integer rejects by type", schema: `{"type":"integer"}`,
			body: `null`, wantType: "integer", wantKeywords: []string{"type"},
		},
		{
			name: "explicit nonnullable number rejects by type", schema: `{"type":"number"}`,
			body: `null`, wantType: "number", wantKeywords: []string{"type"},
		},
		{
			name: "explicit nonnullable string rejects by type", schema: `{"type":"string"}`,
			body: `null`, wantType: "string", wantKeywords: []string{"type"},
		},
		{
			name: "explicit nonnullable array rejects by type", schema: `{"type":"array","items":{}}`,
			body: `null`, wantType: "array", wantKeywords: []string{"type"},
		},
		{
			name: "explicit nonnullable object rejects by type", schema: `{"type":"object"}`,
			body: `null`, wantType: "object", wantKeywords: []string{"type"},
		},
		{
			name: "explicit nullable boolean admits null", schema: `{"type":"boolean","nullable":true}`,
			body: `null`, valid: true, wantType: "boolean", wantNullable: true,
		},
		{
			name: "explicit nullable integer admits null", schema: `{"type":"integer","nullable":true}`,
			body: `null`, valid: true, wantType: "integer", wantNullable: true,
		},
		{
			name: "explicit nullable number admits null", schema: `{"type":"number","nullable":true}`,
			body: `null`, valid: true, wantType: "number", wantNullable: true,
		},
		{
			name: "explicit nullable string admits null", schema: `{"type":"string","nullable":true}`,
			body: `null`, valid: true, wantType: "string", wantNullable: true,
		},
		{
			name: "explicit nullable array admits null", schema: `{"type":"array","items":{},"nullable":true}`,
			body: `null`, valid: true, wantType: "array", wantNullable: true,
		},
		{
			name: "explicit nullable object admits null", schema: `{"type":"object","nullable":true}`,
			body: `null`, valid: true, wantType: "object", wantNullable: true,
		},
		{
			name: "nullable keeps enum active", schema: `{"type":"string","nullable":true,"enum":["x"]}`,
			body: `null`, wantType: "string", wantNullable: true, wantKeywords: []string{"enum"},
		},
		{
			name: "nullable does not infer type", schema: `{"nullable":true,"minLength":2}`,
			body: `1`, valid: true,
		},
		{
			name: "string rules ignore numbers", schema: `{"minLength":2,"pattern":"^x+$","format":"date"}`,
			body: `1`, valid: true,
		},
		{
			name: "number rules ignore strings", schema: `{"minimum":2,"multipleOf":2,"format":"int32"}`,
			body: `"not a number"`, valid: true,
		},
		{
			name: "array rules ignore booleans", schema: `{"minItems":2,"items":{"type":"string"}}`,
			body: `true`, valid: true,
		},
		{
			name: "object rules ignore null",
			schema: `{"minProperties":1,"required":["value"],` +
				`"properties":{"value":{"type":"string"}}}`,
			body: `null`, valid: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parsed := mustParseSchema(t, test.schema, "")
			require.Equal(t, test.wantType, parsed.KindValidation.Type)
			require.Equal(t, test.wantNullable, parsed.KindValidation.Nullable)

			errs := parsed.Validate(json.RawMessage(test.body))
			if test.valid {
				require.Empty(t, errs)

				return
			}

			require.Equal(t, test.wantKeywords, validationErrorKeywords(errs))
		})
	}
}

// TestValidationLocksSemanticEnumEquality verifies JSON Schema enum equality at the runtime seam.
func TestValidationLocksSemanticEnumEquality(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		valid   string
		invalid string
	}{
		{
			name:    "exact decimal and exponent numbers",
			schema:  `{"enum":[9007199254740993.00]}`,
			valid:   `90071992547409930e-1`,
			invalid: `9007199254740992`,
		},
		{
			name:    "decoded strings",
			schema:  `{"enum":["\u0061"]}`,
			valid:   `"a"`,
			invalid: `"b"`,
		},
		{
			name:    "ordered arrays",
			schema:  `{"enum":[[1.0,{"name":"\u0061"}]]}`,
			valid:   `[1e0,{"name":"a"}]`,
			invalid: `[{"name":"a"},1]`,
		},
		{
			name:    "unordered nested objects",
			schema:  `{"enum":[{"z":{"b":2,"a":"\u0061"},"items":[1,2]}]}`,
			valid:   `{"items":[1.0,2e0],"z":{"a":"a","b":2}}`,
			invalid: `{"z":{"a":"a","b":2},"items":[2,1]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parsed := mustParseSchema(t, test.schema, "")
			require.Empty(t, parsed.Validate(json.RawMessage(test.valid)))
			require.Equal(t, []string{"enum"}, validationErrorKeywords(
				parsed.Validate(json.RawMessage(test.invalid)),
			))
		})
	}
}

// TestParseDeduplicatesSemanticEnumMembersPreservingFirstAuthoredValue locks enum order.
func TestParseDeduplicatesSemanticEnumMembersPreservingFirstAuthoredValue(t *testing.T) {
	t.Parallel()

	parsed := mustParseSchema(t, `{"enum":[1.0,1,{"first":true,"second":2},{"second":2,"first":true},null,null]}`, "")

	require.Equal(t, []string{
		`1.0`,
		`{"first":true,"second":2}`,
		`null`,
	}, rawMessages(parsed.EnumValidation.Values))

	for _, body := range []string{`1e0`, `{"second":2,"first":true}`, `null`} {
		require.Empty(t, parsed.Validate(json.RawMessage(body)))
	}
}

// rawMessages makes compiled enum source values readable in assertions.
func rawMessages(values []json.RawMessage) []string {
	messages := make([]string, len(values))
	for index, value := range values {
		messages[index] = string(value)
	}

	return messages
}

// TestValidationReportsSiblingFailuresInStableRuleOrder locks public error identity and order.
func TestValidationReportsSiblingFailuresInStableRuleOrder(t *testing.T) {
	t.Parallel()

	parsed := mustParseSchema(t, `{
		"type":"string",
		"enum":["x"],
		"minLength":2,
		"pattern":"^x+$"
	}`, "")

	errs := parsed.Validate(json.RawMessage(`null`))
	require.Equal(t, []string{"type", "enum"}, validationErrorKeywords(errs))
	require.Equal(t, []string{
		"instance # schema #/paths/~1things/post/requestBody/content/application~1json/schema " +
			"keyword type: got null, want string",
		"instance # schema #/paths/~1things/post/requestBody/content/application~1json/schema " +
			"keyword enum: value is not an allowed member",
	}, errorStrings(errs))
}

// TestParseValidatesDefaultsBySameObjectTypeAndLeavesThemRuntimeInert locks default semantics.
func TestParseValidatesDefaultsBySameObjectTypeAndLeavesThemRuntimeInert(t *testing.T) {
	t.Parallel()

	validDefaults := []struct {
		name   string
		schema string
	}{
		{name: "boolean", schema: `{"type":"boolean","default":false}`},
		{name: "integer", schema: `{"type":"integer","default":1.0}`},
		{name: "number", schema: `{"type":"number","default":1.5}`},
		{name: "string", schema: `{"type":"string","default":"fallback"}`},
		{name: "array", schema: `{"type":"array","items":{},"default":[]}`},
		{name: "object", schema: `{"type":"object","default":{}}`},
		{name: "nullable null", schema: `{"type":"string","nullable":true,"default":null}`},
	}
	for _, test := range validDefaults {
		t.Run("accepts "+test.name, func(t *testing.T) {
			t.Parallel()

			mustParseSchema(t, test.schema, "")
		})
	}

	invalidDefaults := []struct {
		name   string
		schema string
	}{
		{name: "boolean", schema: `{"type":"boolean","default":0}`},
		{name: "integer", schema: `{"type":"integer","default":1.5}`},
		{name: "number", schema: `{"type":"number","default":"1"}`},
		{name: "string", schema: `{"type":"string","default":null}`},
		{name: "array", schema: `{"type":"array","items":{},"default":{}}`},
		{name: "object", schema: `{"type":"object","default":[]}`},
	}
	for _, test := range invalidDefaults {
		t.Run("rejects "+test.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := Parse(openAPISpec(test.schema, "", false))
			require.Nil(t, parsed)
			require.ErrorContains(t, err, "/default")
			require.ErrorContains(t, err, "must conform to type")
		})
	}

	parsed := mustParseSchema(t, `{"type":"string","default":"x","minLength":2}`, "")
	require.Empty(t, parsed.Validate(nil))
	require.Empty(t, parsed.Validate(json.RawMessage(`"ok"`)))
	require.Equal(t, []string{"minLength"}, validationErrorKeywords(parsed.Validate(json.RawMessage(`"x"`))))

	required := mustParseSchemaWithRequired(t, `{"type":"string","default":"fallback"}`, "", true)
	require.Equal(t, []string{"requestBody"}, validationErrorKeywords(required.Validate(nil)))
}

// validationErrorKeywords returns the stable keyword identity of each validation error.
func validationErrorKeywords(errs []error) []string {
	keywords := make([]string, len(errs))
	for index, err := range errs {
		parts := strings.SplitN(err.Error(), " keyword ", 2)
		if len(parts) != 2 {
			keywords[index] = err.Error()

			continue
		}

		keywords[index] = strings.SplitN(parts[1], ":", 2)[0]
	}

	return keywords
}

// TestParseAppliesPatternOptionOncePerPattern preserves caller-owned option state.
func TestParseAppliesPatternOptionOncePerPattern(t *testing.T) {
	t.Parallel()

	calls := 0
	option := func(*patternvalidator.PatternValidation) error {
		calls++

		return nil
	}

	parsed, err := Parse(openAPISpec(`{"type":"string","pattern":"^a$"}`, "", false), option)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Empty(t, parsed["checkThing"].Body.Validate(json.RawMessage(`"a"`)))
}

// TestParseRetainsLeadingLookaheadPattern covers the authoritative pattern matcher seam.
func TestParseRetainsLeadingLookaheadPattern(t *testing.T) {
	t.Parallel()

	parsed, err := Parse(openAPISpec(`{"type":"string","pattern":"^(?=a)a"}`, "", false))
	require.NoError(t, err)

	validation := parsed["checkThing"].Body
	require.Empty(t, validation.Validate(json.RawMessage(`"ab"`)))
	require.ErrorContains(t, errors.Join(validation.Validate(json.RawMessage(`"ba"`))...), "keyword pattern")
}

// TestParseAdmitsRestrictedLeadingAssertionsWithConsumingRemainders pins both assertion polarities.
func TestParseAdmitsRestrictedLeadingAssertionsWithConsumingRemainders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		valid   string
		invalid string
	}{
		{name: "positive", pattern: `^(?=ab)a`, valid: "ab", invalid: "ac"},
		{name: "negative", pattern: `^(?!ab)a`, valid: "ac", invalid: "ab"},
		{name: "positive remainder mismatch", pattern: `^(?=a)ab`, valid: "ab", invalid: "aa"},
		{name: "negative remainder mismatch", pattern: `^(?!ab)ac`, valid: "ac", invalid: "ad"},
		{name: "consecutive mixed assertions", pattern: `^(?=a)(?!ab)a`, valid: "ac", invalid: "ab"},
		{name: "negative then positive", pattern: `^(?!ab)(?=a)a`, valid: "ac", invalid: "ab"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := Parse(openAPISpec(
				`{"type":"string","pattern":`+strconv.Quote(test.pattern)+`}`,
				"",
				false,
			))
			require.NoError(t, err)

			validation := parsed["checkThing"].Body
			for range 3 {
				require.Empty(t, validation.Validate(json.RawMessage(strconv.Quote(test.valid))))
				require.ErrorContains(
					t,
					errors.Join(validation.Validate(json.RawMessage(strconv.Quote(test.invalid)))...),
					"keyword pattern",
				)
			}
		})
	}
}

// TestParseRejectsUnsupportedLeadingAssertionPlacement keeps lookahead admission narrow.
func TestParseRejectsUnsupportedLeadingAssertionPlacement(t *testing.T) {
	t.Parallel()

	patterns := []string{
		`^(?=(?=a))a`,
		`^(?=a)`,
		`^(?!a)`,
		`^(?=a)$`,
		`^(?=a)\b`,
		`^(?=a)(?:)`,
		`^(?=a)a{0}`,
		`^(?=a)+a`,
		`a(?=b)b`,
		`(?=a)a`,
		`^(?=a)a|b`,
		`^(?=a)(?:b(?=c))`,
	}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(openAPISpec(
				`{"type":"string","pattern":`+strconv.Quote(pattern)+`}`,
				"",
				false,
			))
			require.Error(t, err)
			require.ErrorContains(t, err, "/pattern")
		})
	}
}

// TestParseEnforcesExactPatternResourceLimits pins every production Parse boundary.
func TestParseEnforcesExactPatternResourceLimits(t *testing.T) {
	t.Parallel()

	// The matcher fixtures are measured after translation: the expression wrapper costs
	// 4 bytes, each \S costs 174 bytes, each literal a costs 6 bytes, and each dot
	// costs 23 bytes. The first fixture is exactly 1,048,576 bytes; the second is
	// exactly 1,048,577 bytes.
	matcherAtLimit := strings.Repeat(`\S`, 5_835) + strings.Repeat("a", 3_661) + strings.Repeat(".", 492)
	matcherOverLimit := strings.Repeat(`\S`, 5_835) + strings.Repeat("a", 3_665) + strings.Repeat(".", 491)

	tests := []struct {
		name     string
		pattern  string
		wantErr  bool
		limit    string
		maximum  uint64
		observed uint64
	}{
		// Source is counted as UTF-8 bytes, including the surrounding class brackets.
		{
			name: "source at limit", pattern: "[" + strings.Repeat("a", 65_536-2) + "]",
		},
		{
			name: "source first overflow", pattern: "[" + strings.Repeat("a", 65_537-2) + "]",
			wantErr: true, limit: "source bytes", maximum: 65_536, observed: 65_537,
		},
		// Nesting is the maximum number of simultaneously open groups.
		{
			name: "nesting at limit", pattern: strings.Repeat("(", 100) + "a" + strings.Repeat(")", 100),
		},
		{
			name: "nesting first overflow", pattern: strings.Repeat("(", 101) + "a" + strings.Repeat(")", 101),
			wantErr: true, limit: "nesting depth", maximum: 100, observed: 101,
		},
		// AST nodes count every appended expression, alternative, atom, group,
		// lookahead, or repeat node; this literal fixture has one node per a,
		// plus one expression and one alternative node.
		{
			name: "AST nodes at limit", pattern: strings.Repeat("a", 10_000-2),
		},
		{
			name: "AST nodes first overflow", pattern: strings.Repeat("a", 10_000-1),
			wantErr: true, limit: "AST nodes", maximum: 10_000, observed: 10_001,
		},
		// Leading assertions count consecutive top-level lookahead nodes after ^.
		{
			name: "leading assertions at limit", pattern: "^" + strings.Repeat("(?=a)", 64) + "a",
		},
		{
			name: "leading assertions first overflow", pattern: "^" + strings.Repeat("(?=a)", 65) + "a",
			wantErr: true, limit: "leading assertions", maximum: 64, observed: 65,
		},
		// A counted endpoint is its parsed decimal value, not its source width.
		{
			name: "counted endpoint at limit", pattern: "a{1000}",
		},
		{
			name: "counted endpoint first overflow", pattern: "a{1001}",
			wantErr: true, limit: "repeat endpoint", maximum: 1_000, observed: 1_001,
		},
		// Nested-repeat product is the largest product of counted factors on one
		// nesting path, using bounded maxima or unbounded minima; the inner and
		// outer factors here are 10 and 100.
		{
			name: "nested counted-repeat product at limit", pattern: "(?:a{10}){100}",
		},
		{
			name: "nested counted-repeat product first overflow", pattern: "(?:a{10}){101}",
			wantErr: true, limit: "cumulative nested repeat product", maximum: 1_000, observed: 1_010,
		},
		{
			name: "translated matcher source at limit", pattern: matcherAtLimit,
		},
		{
			name: "translated matcher source first overflow", pattern: matcherOverLimit,
			wantErr: true, limit: "generated Go regexp bytes", maximum: 1_048_576, observed: 1_048_577,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := Parse(openAPISpec(
				`{"type":"string","pattern":`+strconv.Quote(test.pattern)+`}`,
				"", false,
			))
			if !test.wantErr {
				require.NoError(t, err)
				require.NotNil(t, parsed["checkThing"].Body)

				return
			}

			require.Error(t, err)
			require.Nil(t, parsed)
			require.ErrorContains(t, err, "/pattern")
			require.ErrorIs(t, err, patternvalidator.ErrTooComplex)

			var complexity *patternvalidator.ComplexityError
			require.ErrorAs(t, err, &complexity)
			require.Equal(t, test.limit, complexity.Limit)
			require.Equal(t, test.maximum, complexity.Maximum)
			require.Equal(t, test.observed, complexity.Observed)

			repeated, repeatedErr := Parse(openAPISpec(
				`{"type":"string","pattern":`+strconv.Quote(test.pattern)+`}`,
				"", false,
			))
			require.Nil(t, repeated)
			require.EqualError(t, repeatedErr, err.Error())
		})
	}
}

// TestParseAdmitsNamedECMAScript51PatternFamilies pins the production Parse seam.
func TestParseAdmitsNamedECMAScript51PatternFamilies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		valid   string
		invalid string
	}{
		{name: "literal", pattern: `^é$`, valid: "é", invalid: "e"},
		{name: "UTF-16 literal units", pattern: `^..$`, valid: "😀", invalid: "x"},
		{name: "dot", pattern: `^.$`, valid: "x", invalid: "\n"},
		{name: "anchors", pattern: `^a$`, valid: "a", invalid: "aa"},
		{name: "alternation", pattern: `^(?:cat|dog)$`, valid: "dog", invalid: "cow"},
		{name: "groups", pattern: `^(a)(?:b)$`, valid: "ab", invalid: "a"},
		{name: "classes", pattern: `^[^a-c][a-z-]$`, valid: "z-", invalid: "ab"},
		{name: "empty and universal classes", pattern: `^(?:[]|[^])$`, valid: "x", invalid: "xx"},
		{
			name:    "shorthand classes and ES5 whitespace",
			pattern: `^\d\D\w\W\s\S$`,
			valid:   "7a_-\u180ex",
			invalid: "7a_-\u205fx",
		},
		{name: "word boundaries", pattern: `^\bcat\Bdog$`, valid: "catdog", invalid: "scatterdog"},
		{name: "class backspace", pattern: `^[\b]$`, valid: "\b", invalid: "x"},
		{
			name:    "escapes",
			pattern: `^\f\n\r\t\v\0\x41\u0042\cC\ca\/\-\#\,$`,
			valid:   "\f\n\r\t\v\x00AB\x03\x01/-#,",
			invalid: "\f\n\r\t\v\x00AB\x03\x01/-#.",
		},
		{
			name:    "greedy and lazy quantifiers",
			pattern: `^a*?b+?c??d{0}e{1,}f{2,3}?$`,
			valid:   "beff",
			invalid: "bef",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := Parse(openAPISpec(
				`{"type":"string","pattern":`+strconv.Quote(test.pattern)+`}`,
				"",
				false,
			))
			require.NoError(t, err)

			validation := parsed["checkThing"].Body
			validBody, err := json.Marshal(test.valid)
			require.NoError(t, err)
			invalidBody, err := json.Marshal(test.invalid)
			require.NoError(t, err)
			require.Empty(t, validation.Validate(validBody))
			require.NotEmpty(t, validation.Validate(invalidBody))
		})
	}
}

// TestParseRejectsNamedOutsideProfileECMAScript51Patterns pins Parse failures at /pattern.
func TestParseRejectsNamedOutsideProfileECMAScript51Patterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
	}{
		{name: "backreferences", pattern: `(a)\1`},
		{name: "surrogate escapes", pattern: `\uD800`},
		{name: "lookbehind", pattern: `(?<=a)b`},
		{name: "named groups", pattern: `(?<name>a)`},
		{name: "named references", pattern: `\k<name>`},
		{name: "inline modes", pattern: `(?i:a)`},
		{name: "atomic groups", pattern: `(?>a)`},
		{name: "control verbs", pattern: `(*FAIL)`},
		{name: "Unicode property escapes", pattern: `\p{L}`},
		{name: "Unicode code-point escapes", pattern: `\u{41}`},
		{name: "POSIX classes", pattern: `[[:alpha:]]`},
		{name: "character-class set operations", pattern: `[a&&b]`},
		{name: "possessive quantifiers", pattern: `a*+`},
		{name: "malformed range", pattern: `[z-a]`},
		{name: "malformed group", pattern: `(`},
		{name: "unmatched group close", pattern: `)`},
		{name: "malformed class", pattern: `[`},
		{name: "malformed escape", pattern: `\x0g`},
		{name: "decimal escape after zero", pattern: `\01`},
		{name: "malformed quantifier", pattern: `a{2,1}`},
		{name: "malformed repeated quantifier", pattern: `a**`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(openAPISpec(
				`{"type":"string","pattern":`+strconv.Quote(test.pattern)+`}`,
				"",
				false,
			))
			require.Error(t, err)
			require.ErrorContains(t, err, "/pattern")
		})
	}

	t.Run("malformed UTF-8", func(t *testing.T) {
		t.Parallel()

		pattern := string([]byte{'a', 0xff})
		_, err := Parse(openAPISpec(
			`{"type":"string","pattern":"`+pattern+`"}`,
			"",
			false,
		))
		require.Error(t, err)
		require.ErrorContains(t, err, "UTF-8")
		require.ErrorContains(t, err, "/pattern")
	})

	t.Run("unpaired JSON surrogate escape", func(t *testing.T) {
		t.Parallel()

		_, err := Parse(openAPISpec(`{"type":"string","pattern":"\ud800"}`, "", false))
		require.Error(t, err)
		require.ErrorContains(t, err, "surrogate")
		require.ErrorContains(t, err, "/pattern")
	})
}

// TestParseExposesCompiledGraphAndCopiesInput covers the supported construction seam.
func TestParseExposesCompiledGraphAndCopiesInput(t *testing.T) {
	t.Parallel()

	spec := openAPISpec(`{
		"type":"object","required":["value"],
		"properties":{"value":{"type":"string","minLength":1}},
		"additionalProperties":false,
		"allOf":[{"maxProperties":1}]
	}`, "", true)
	parsedByOperation, err := Parse(spec)
	require.NoError(t, err)

	parsed := parsedByOperation["checkThing"].Body

	for index := range spec {
		spec[index] = ' '
	}

	require.True(t, parsed.BodyRequired)
	require.Equal(t, "object", parsed.KindValidation.Type)
	require.Equal(t, []string{"value"}, parsed.ObjectValidation.Required)
	require.Len(t, parsed.ObjectValidation.Properties, 1)
	require.Equal(t, "value", parsed.ObjectValidation.Properties[0].Name)
	require.Equal(t, "string", parsed.ObjectValidation.Properties[0].Validation.KindValidation.Type)
	require.False(t, parsed.ObjectValidation.AdditionalPropertiesAllowed)
	require.Len(t, parsed.AllOfValidations, 1)
	require.Empty(t, parsed.Validate(json.RawMessage(`{"value":"x"}`)))
}

// TestParseCompilesIndependentOperationGraphs verifies the document-wide map and per-operation compiler state.
func TestParseCompilesIndependentOperationGraphs(t *testing.T) {
	t.Parallel()

	spec := []byte(`{
		"openapi":"3.0.3",
		"paths":{
			"/required":{"post":{
				"operationId":"RequiredBody",
				"requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Body"}}}}
			}},
			"/optional":{"put":{
				"operationId":"optionalBody",
				"requestBody":{"content":{"application/*":{"schema":{"$ref":"#/components/schemas/Body"}}}}
			}},
			"/plain":{"post":{"operationId":"plain","requestBody":{"content":{"text/plain":{"schema":{"type":"string"}}}}}},
			"/bodyless":{"get":{"operationId":"bodyless"}}
		},
		"components":{"schemas":{"Body":{"type":"string"}}}
	}`)

	parsed, err := Parse(spec)
	require.NoError(t, err)
	require.Equal(t, []string{"RequiredBody", "bodyless", "optionalBody", "plain"}, slices.Sorted(maps.Keys(parsed)))
	require.True(t, parsed["RequiredBody"].Body.BodyRequired)
	require.False(t, parsed["optionalBody"].Body.BodyRequired)
	require.NotSame(t, parsed["RequiredBody"].Body, parsed["optionalBody"].Body)
	require.Equal(t, "#/components/schemas/Body", parsed["RequiredBody"].Body.SchemaPointer)
	require.Equal(t, "#/components/schemas/Body", parsed["optionalBody"].Body.SchemaPointer)
	require.Nil(t, parsed["plain"].Body)
	require.Equal(t, RequestValidation{}, parsed["bodyless"])

	for index := range spec {
		spec[index] = ' '
	}

	require.Empty(t, parsed["RequiredBody"].Body.Validate(json.RawMessage(`"still compiled"`)))
}

// TestParseRetainsRequestBodiesForEveryOperationMethod prevents method filtering.
func TestParseRetainsRequestBodiesForEveryOperationMethod(t *testing.T) {
	t.Parallel()

	for _, method := range []string{"get", "head", "delete", "options", "trace", "post", "put", "patch"} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			spec := fmt.Appendf(nil, `openapi: 3.0.3
paths:
  /request:
    %s:
      operationId: request
      requestBody:
        required: true
        content:
          application/json:
            schema: {type: string}
`, method)
			requests, err := Parse(spec)
			require.NoError(t, err)
			require.NotNil(t, requests["request"].Body)
			require.Empty(t, requests["request"].Body.Validate(json.RawMessage(`"body"`)))
		})
	}
}

// TestParseBuildsOneRequestValidationPerOperation covers the atomic public Parse result.
func TestParseBuildsOneRequestValidationPerOperation(t *testing.T) {
	t.Parallel()

	requests, err := Parse([]byte(`openapi: 3.0.3
paths:
  /all/{id}:
    post:
      operationId: all
      parameters:
        - {name: id, in: path, required: true, schema: {type: integer}}
        - {name: q, in: query, schema: {type: string}}
      requestBody:
        content:
          application/json:
            schema: {type: object}
  /none:
    get: {operationId: none}
`))
	require.NoError(t, err)

	require.Equal(t, []string{"all", "none"}, slices.Sorted(maps.Keys(requests)))
	require.NotNil(t, requests["all"].Body)
	require.NotNil(t, requests["all"].Query)
	require.NotNil(t, requests["all"].Path)
	require.Equal(t, RequestValidation{}, requests["none"])

	path, err := requests["all"].Path.DecodePathParams(&url.URL{Path: "/all/42"})
	require.NoError(t, err)
	require.JSONEq(t, `{"id":42}`, string(path))
}

// TestParseRejectsInvalidOverriddenParameterDeclarations keeps invalid Path Item input visible through overrides.
func TestParseRejectsInvalidOverriddenParameterDeclarations(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		parameter string
		contains  string
	}{
		{
			name: "schema", parameter: `{name: id, in: path, required: true, schema: {oneOf: [{type: string}]}}`,
			contains: "/parameters/0/schema/oneOf",
		},
		{
			name: "content", parameter: `{name: id, in: path, required: true, content: {text/plain: {}}}`,
			contains: "only application/json",
		},
		{
			name: "style", parameter: `{name: id, in: path, required: true, style: form, schema: {type: string}}`,
			contains: `style "form" is unsupported`,
		},
		{
			name: "path field", parameter: `{name: id, in: path, required: true, allowReserved: false, schema: {type: string}}`,
			contains: "cannot declare allowReserved",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			requests, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items/{id}:
    parameters:
      - ` + test.parameter + `
    get:
      operationId: getItem
      parameters:
        - {name: id, in: path, required: true, schema: {type: integer}}
`))
			require.Nil(t, requests)
			require.ErrorContains(t, err, test.contains)
		})
	}
}

// TestParseRejectsInvalidIgnoredParameterDeclarations validates header and cookie inputs before filtering.
func TestParseRejectsInvalidIgnoredParameterDeclarations(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		parameter string
		contains  string
	}{
		{
			name: "header schema reference", parameter: `{name: X-Filter, in: header, schema: {$ref: '#/missing'}}`,
			contains: "resolve reference",
		},
		{
			name: "header style", parameter: `{name: X-Filter, in: header, style: matrix, schema: {type: string}}`,
			contains: `header parameter "X-Filter" style "matrix" is invalid`,
		},
		{
			name:      "header allow reserved",
			parameter: `{name: X-Filter, in: header, allowReserved: false, schema: {type: string}}`,
			contains:  `header parameter "X-Filter" cannot declare allowReserved`,
		},
		{
			name: "cookie style", parameter: `{name: session, in: cookie, style: simple, schema: {type: string}}`,
			contains: `cookie parameter "session" style "simple" is invalid`,
		},
		{
			name: "cookie allow empty", parameter: `{name: session, in: cookie, allowEmptyValue: false, schema: {type: string}}`,
			contains: `cookie parameter "session" cannot declare allowEmptyValue`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			requests, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items:
    get:
      operationId: getItems
      parameters:
        - ` + test.parameter + `
`))
			require.Nil(t, requests)
			require.ErrorContains(t, err, test.contains)
		})
	}
}

// TestParseIgnoresReservedHeaderParameterDefinitions preserves OpenAPI's reserved-header exception.
func TestParseIgnoresReservedHeaderParameterDefinitions(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"Accept", "content-type", "AUTHORIZATION"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			requests, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items:
    get:
      operationId: getItems
      parameters:
        - {name: ` + name + `, in: header, schema: {oneOf: [{type: string}]}}
`))
			require.NoError(t, err)
			require.Contains(t, requests, "getItems")
		})
	}
}

// TestParseAllowsParameterExamplesWithContent keeps Parameter Object examples independent of schema/content.
func TestParseAllowsParameterExamplesWithContent(t *testing.T) {
	t.Parallel()

	parameters := []string{
		`{name: q, in: query, %s, content: {application/json: {}}}`,
		`{name: value, in: path, required: true, %s, content: {application/json: {}}}`,
		`{name: X-Value, in: header, %s, content: {application/json: {}}}`,
		`{name: value, in: cookie, %s, content: {application/json: {}}}`,
	}

	for _, field := range []string{`example: false`, `examples: {sample: {value: false}}`} {
		for index, parameter := range parameters {
			t.Run(fmt.Sprintf("%s location %d", field, index), func(t *testing.T) {
				t.Parallel()

				path := "/items"
				if index == 1 {
					path = "/items/{value}"
				}

				requests, err := Parse([]byte(`openapi: 3.0.3
paths:
  ` + path + `:
    get:
      operationId: getItems
      parameters:
        - ` + fmt.Sprintf(parameter, field) + `
`))
				require.NoError(t, err)
				require.Contains(t, requests, "getItems")
			})
		}
	}
}

// TestParseReportsPathSchemaErrorsBeforePathFieldMisuse preserves compiler error precedence.
func TestParseReportsPathSchemaErrorsBeforePathFieldMisuse(t *testing.T) {
	t.Parallel()

	for _, field := range []string{"allowEmptyValue", "allowReserved"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()

			requests, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items/{id}:
    get:
      operationId: getItem
      parameters:
        - name: id
          in: path
          required: true
          ` + field + `: false
          schema: {oneOf: [{type: string}]}
`))
			require.Nil(t, requests)
			require.ErrorContains(t, err, "/schema/oneOf")
			require.ErrorContains(t, err, "unsupported keyword")
			require.NotContains(t, err.Error(), "cannot declare "+field)
		})
	}
}

// TestParseChecksOperationStructureBeforeOrdinaryParameterSchemaProfile preserves admission phases.
func TestParseChecksOperationStructureBeforeOrdinaryParameterSchemaProfile(t *testing.T) {
	t.Parallel()

	requests, err := Parse([]byte(`openapi: 3.0.3
paths:
  /items:
    get:
      operationId: not-valid-
      parameters:
        - {name: q, in: query, schema: {oneOf: [{type: string}]}}
`))
	require.Nil(t, requests)
	require.ErrorIs(t, err, oas.ErrInvalidOperationID)
	require.NotContains(t, err.Error(), "/parameters/0/schema/oneOf")
}

// TestParseCompilesOperationsInSortedIDOrder verifies deterministic atomic failure selection.
func TestParseCompilesOperationsInSortedIDOrder(t *testing.T) {
	t.Parallel()

	_, err := Parse([]byte(`{
		"openapi":"3.0.3",
		"paths":{
			"/first":{"post":{"operationId":"zulu","requestBody":{"content":{"application/json":{"schema":{"not":{}}}}}}},
			"/second":{"post":{"operationId":"alpha","requestBody":{"content":{"application/json":{"schema":{"oneOf":[{}]}}}}}}
		}
	}`))
	require.ErrorContains(t, err, `compile operationId "alpha"`)
	require.ErrorContains(t, err, "/oneOf")
}

// TestParseReturnsNilMapsAfterLateCompilationFailure verifies atomic return values.
func TestParseReturnsNilMapsAfterLateCompilationFailure(t *testing.T) {
	t.Parallel()

	requestValidations, err := Parse([]byte(`{
		"openapi":"3.0.3",
		"paths":{
			"/first":{"post":{
				"operationId":"alpha",
				"requestBody":{"content":{"application/json":{"schema":{"type":"string"}}}}
			}},
			"/second":{"post":{"operationId":"zulu","requestBody":{"content":{"application/json":{"schema":{"not":{}}}}}}}
		}
	}`))
	require.Nil(t, requestValidations)
	require.ErrorContains(t, err, `compile operationId "zulu"`)
}

// TestValidationStringFormats covers the original native format examples and strict policy.
func TestValidationStringFormats(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		format  string
		valid   string
		invalid string
	}{
		{format: "byte", valid: `"YWJj"`, invalid: `"%%%"`},
		{format: "date", valid: `"2026-07-14"`, invalid: `"2026-02-30"`},
		{format: "date-time", valid: `"2026-07-14T12:30:00Z"`, invalid: `"2026-07-14"`},
		{format: "email", valid: `"a@example.com"`, invalid: `"not-an-email"`},
	} {
		t.Run(test.format, func(t *testing.T) {
			t.Parallel()

			parsed := mustParseSchema(t, fmt.Sprintf(`{"type":"string","format":%q}`, test.format), "")
			require.Empty(t, parsed.Validate(json.RawMessage(test.valid)))
			require.Contains(t, errors.Join(parsed.Validate(json.RawMessage(test.invalid))...).Error(), "keyword format")
		})
	}

	_, err := Parse(openAPISpec(`{"type":"string","format":"vendor-string"}`, "", false))
	require.ErrorContains(t, err, "legal OpenAPI but unsupported by this tool")
}

// TestValidationStrictJSONAndBodyPresence covers transport-independent raw-body rules.
func TestValidationStrictJSONAndBodyPresence(t *testing.T) {
	t.Parallel()

	optional := mustParseSchema(t, `{}`, "")
	require.Nil(t, optional.Validate(nil))
	require.NotEmpty(t, optional.Validate(json.RawMessage("   ")))

	required := mustParseSchemaWithRequired(t, `{}`, "", true)
	require.Contains(t, errors.Join(required.Validate(nil)...).Error(), "required body is absent")
	require.Nil(t, required.Validate(json.RawMessage(`null`)))

	invalidBodies := []json.RawMessage{
		{0xff},
		json.RawMessage(`true false`),
		json.RawMessage(`{"a":1,"a":2}`),
		json.RawMessage(`{"a":{"b":1,"b":2}}`),
		json.RawMessage(`"\ud800"`),
		json.RawMessage(`"\udc00"`),
	}
	for _, body := range invalidBodies {
		require.NotEmpty(t, optional.Validate(body), "%q", body)
	}
}

// TestParseRejectsMalformedJSONRequestMediaAndSchemasAtPointers covers selected content shapes.
func TestParseRejectsMalformedJSONRequestMediaAndSchemasAtPointers(t *testing.T) {
	t.Parallel()

	const (
		mediaPointer  = "#/paths/~1things/post/requestBody/content/application~1json; charset=utf-8"
		schemaPointer = mediaPointer + "/schema"
	)

	tests := []struct {
		name       string
		mediaType  string
		pointer    string
		objectName string
	}{
		{name: "null media type", mediaType: `null`, pointer: mediaPointer, objectName: "Media Type Object"},
		{name: "scalar media type", mediaType: `1`, pointer: mediaPointer, objectName: "Media Type Object"},
		{name: "array media type", mediaType: `[]`, pointer: mediaPointer, objectName: "Media Type Object"},
		{name: "null schema", mediaType: `{"schema":null}`, pointer: schemaPointer, objectName: "Schema Object"},
		{name: "scalar schema", mediaType: `{"schema":1}`, pointer: schemaPointer, objectName: "Schema Object"},
		{name: "array schema", mediaType: `{"schema":[]}`, pointer: schemaPointer, objectName: "Schema Object"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			spec := fmt.Appendf(nil, `{
				"openapi":"3.0.3",
				"paths":{"/things":{"post":{
					"operationId":"checkThing",
					"requestBody":{"content":{"application/json; charset=utf-8":%s}}
				}}}
			}`, test.mediaType)
			_, err := Parse(spec)
			require.Error(t, err)
			require.ErrorContains(t, err, test.objectName)
			require.ErrorContains(t, err, test.pointer)
		})
	}
}

// TestSchemaLessJSONRequestBodyRuntimeSemantics covers JSON values, presence, and strict decoding.
func TestSchemaLessJSONRequestBodyRuntimeSemantics(t *testing.T) {
	t.Parallel()

	requestValidations, err := Parse([]byte(`openapi: 3.0.3
paths:
  /optional:
    post:
      operationId: optionalBody
      requestBody:
        content:
          application/json: {}
  /required:
    post:
      operationId: requiredBody
      requestBody:
        required: true
        content:
          application/json: {}
  /defaulted-optional:
    post:
      operationId: defaultedOptionalBody
      requestBody:
        content:
          application/json:
            schema: {type: string, minLength: 2, default: fallback}
  /defaulted-required:
    post:
      operationId: defaultedRequiredBody
      requestBody:
        required: true
        content:
          application/json:
            schema: {type: string, default: fallback}
`))
	require.NoError(t, err)

	for _, body := range []json.RawMessage{
		json.RawMessage(`null`),
		json.RawMessage(`true`),
		json.RawMessage(`1.25`),
		json.RawMessage(`"value"`),
		json.RawMessage(`[1,true]`),
		json.RawMessage(`{"value":1}`),
		json.RawMessage(" \n\tnull\r "),
	} {
		require.Empty(t, requestValidations["optionalBody"].Body.Validate(body), "%q", body)
		require.Empty(t, requestValidations["requiredBody"].Body.Validate(body), "%q", body)
	}

	for _, absent := range []json.RawMessage{nil, {}} {
		require.Empty(t, requestValidations["optionalBody"].Body.Validate(absent))
		require.ErrorContains(
			t,
			errors.Join(requestValidations["requiredBody"].Body.Validate(absent)...),
			"required body is absent",
		)
	}

	for _, invalid := range []json.RawMessage{
		json.RawMessage(" \n\t "),
		json.RawMessage(`{"value":`),
		json.RawMessage(`true false`),
	} {
		require.NotEmpty(t, requestValidations["optionalBody"].Body.Validate(invalid), "%q", invalid)
	}

	require.Empty(t, requestValidations["defaultedOptionalBody"].Body.Validate(nil))
	require.NotEmpty(t, requestValidations["defaultedOptionalBody"].Body.Validate(json.RawMessage(`"x"`)))
	require.Empty(t, requestValidations["defaultedOptionalBody"].Body.Validate(json.RawMessage(`"ok"`)))
	require.ErrorContains(
		t,
		errors.Join(requestValidations["defaultedRequiredBody"].Body.Validate(nil)...),
		"required body is absent",
	)
}

// TestSchemaLessJSONRequestBodyPreservesMediaSelection verifies specificity before schema compilation.
func TestSchemaLessJSONRequestBodyPreservesMediaSelection(t *testing.T) {
	t.Parallel()

	requestValidations, err := Parse([]byte(`openapi: 3.0.3
paths:
  /exact:
    post:
      operationId: exact
      requestBody:
        content:
          '*/*': {schema: {type: number}}
          application/*: {schema: {type: boolean}}
          application/json: {}
  /application-wildcard:
    post:
      operationId: applicationWildcard
      requestBody:
        content:
          '*/*': {schema: {type: boolean}}
          application/*: {}
  /global-wildcard:
    post:
      operationId: globalWildcard
      requestBody:
        content:
          '*/*': {}
  /parameterized-exact:
    post:
      operationId: parameterizedExact
      requestBody:
        content:
          application/*: {schema: {type: boolean}}
          'application/json; charset=utf-8': {}
`))
	require.NoError(t, err)

	for _, operationID := range []string{"exact", "applicationWildcard", "globalWildcard", "parameterizedExact"} {
		require.Empty(t, requestValidations[operationID].Body.Validate(json.RawMessage(`"schema-less winner"`)))
	}
}

// TestValidationExactNumbers covers values beyond float64 and arbitrary exponent materialization.
func TestValidationExactNumbers(t *testing.T) {
	t.Parallel()

	parsed := mustParseSchema(t, `{
		"type":"number",
		"minimum":9007199254740993,
		"maximum":9007199254740993,
		"multipleOf":0.1
	}`, "")
	require.Empty(t, parsed.Validate(json.RawMessage(`9007199254740993`)))
	require.NotEmpty(t, parsed.Validate(json.RawMessage(`9007199254740992`)))
	require.NotEmpty(t, parsed.Validate(json.RawMessage(`9007199254740994`)))

	spelling := mustParseSchema(t, `{"minimum":1,"maximum":1}`, "")
	for _, body := range []json.RawMessage{json.RawMessage(`1`), json.RawMessage(`1.0`), json.RawMessage(`1e0`)} {
		require.Empty(t, spelling.Validate(body))
	}

	zero := mustParseSchema(t, `{"minimum":0,"maximum":0}`, "")
	require.Empty(t, zero.Validate(json.RawMessage(`-0`)))

	huge := mustParseSchema(t, `{"minimum":1e400,"maximum":1e400}`, "")
	require.Empty(t, huge.Validate(json.RawMessage(`1e400`)))
	require.NotEmpty(t, huge.Validate(json.RawMessage(`9e399`)))

	hugeExponent := mustParseSchema(t, `{"multipleOf":3e-100001}`, "")
	require.Empty(t, hugeExponent.Validate(json.RawMessage(`9e-100001`)))
	require.NotEmpty(t, hugeExponent.Validate(json.RawMessage(`1e-100001`)))

	integer := mustParseSchema(t, `{"type":"integer"}`, "")
	require.Empty(t, integer.Validate(json.RawMessage(`1e100001`)))
	require.NotEmpty(t, integer.Validate(json.RawMessage(`1e-100001`)))
}

// TestValidationNestedAndAllOf covers finite nesting and composition behavior directly.
func TestValidationNestedAndAllOf(t *testing.T) {
	t.Parallel()

	components := `,"components":{"schemas":{"Node":{"type":"object","required":["value"],"properties":{
		"value":{"type":"integer"},"child":{"$ref":"#/components/schemas/Child"}
	},"additionalProperties":false},"Child":{"type":"object","required":["value"],"properties":{
		"value":{"type":"integer"}
	},"additionalProperties":false}}}`
	nested := mustParseSchema(t, `{"$ref":"#/components/schemas/Node"}`, components)
	require.Empty(t, nested.Validate(json.RawMessage(`{"value":1,"child":{"value":2}}`)))
	errs := nested.Validate(json.RawMessage(`{"value":1,"child":{"value":2.5}}`))
	require.Contains(t, errors.Join(errs...).Error(), "instance #/child/value")

	allOf := mustParseSchema(t, `{"allOf":[{"minimum":1},{"maximum":2}]}`, "")
	require.Empty(t, allOf.Validate(json.RawMessage(`1.5`)))
	errs = allOf.Validate(json.RawMessage(`3`))
	require.Contains(t, errors.Join(errs...).Error(), "/allOf/1")
}

// TestValidateDefaultRejectsMalformedUntypedValue keeps syntax validation independent of a declared type.
func TestValidateDefaultRejectsMalformedUntypedValue(t *testing.T) {
	t.Parallel()

	require.ErrorContains(t, validateDefault(json.RawMessage(`invalid`), KindValidation{}), "must be a valid value")
	require.NoError(t, validateDefault(json.RawMessage(`null`), KindValidation{}))
}

// TestParseSelectsExternalDocsErrorsDeterministically fixes lexical error precedence.
func TestParseSelectsExternalDocsErrorsDeterministically(t *testing.T) {
	t.Parallel()

	spec := openAPISpec(`{
		"externalDocs":{"url":"/docs","description":1,"other":true}
	}`, "", false)
	for range 100 {
		_, err := Parse(spec)
		require.ErrorContains(t, err, "description must be a string")
	}
}

// TestParseRejectsUniqueItemsAtItsSourcePointer covers every authored value shape.
func TestParseRejectsUniqueItemsAtItsSourcePointer(t *testing.T) {
	t.Parallel()

	const pointer = "#/paths/~1things/post/requestBody/content/application~1json/schema/uniqueItems"

	for _, value := range []string{"true", "false", "null", `"yes"`, "1", `[]`, `{}`} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			parsed, err := Parse(openAPISpec(`{"uniqueItems":`+value+`}`, "", false))
			require.Nil(t, parsed)
			require.Error(t, err)
			require.ErrorContains(t, err, pointer)
		})
	}
}

// TestParseRejectsUniqueItemsAcrossAuthoredSchemaLocations covers reachable and otherwise ignored schemas.
func TestParseRejectsUniqueItemsAcrossAuthoredSchemaLocations(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		spec    string
		pointer string
	}{
		{
			name: "referenced schema",
			spec: string(openAPISpec(
				`{"$ref":"#/components/schemas/Target"}`,
				`,"components":{"schemas":{"Target":{"uniqueItems":false}}}`,
				false,
			)),
			pointer: "#/components/schemas/Target/uniqueItems",
		},
		{
			name:    "unreachable escaped component",
			spec:    `{"openapi":"3.0.3","paths":{},"components":{"schemas":{"a/b~c":{"uniqueItems":false}}}}`,
			pointer: "#/components/schemas/a~1b~0c/uniqueItems",
		},
		{
			name: "response media type",
			spec: `{"openapi":"3.0.3","paths":{"/things":{"get":{"operationId":"things","responses":` +
				`{"200":{"description":"ok","content":{"application/json":{"schema":{"uniqueItems":false}}}}}}}}}`,
			pointer: "#/paths/~1things/get/responses/200/content/application~1json/schema/uniqueItems",
		},
		{
			name: "callback request body",
			spec: `{"openapi":"3.0.3","paths":{"/things":{"post":{"operationId":"things","callbacks":{"event":` +
				`{"{$request.body#/url}":{"post":{"operationId":"callback","requestBody":{"content":` +
				`{"application/json":{"schema":{"uniqueItems":false}}}}}}}}}}}}`,
			pointer: "#/paths/~1things/post/callbacks/event/{$request.body#~1url}/post/requestBody/" +
				"content/application~1json/schema/uniqueItems",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := Parse([]byte(test.spec))
			require.Nil(t, parsed)
			require.Error(t, err)
			require.ErrorContains(t, err, test.pointer)
		})
	}
}

// TestParsePropagatesUnreachableReferenceErrors covers complete traversal resolution failures.
func TestParsePropagatesUnreachableReferenceErrors(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		spec string
		want string
	}{
		{
			name: "schema component",
			spec: `{"openapi":"3.0.3","paths":{},"components":{"schemas":{
				"Broken":{"$ref":"#/components/schemas/Missing"}
			}}}`,
			want: "#/components/schemas/Missing",
		},
		{
			name: "response schema",
			spec: `{"openapi":"3.0.3","paths":{},"components":{"responses":{
				"Broken":{"description":"broken","content":{"application/json":{
					"schema":{"$ref":"#/components/schemas/Missing"}
				}}}
			}}}`,
			want: "#/components/schemas/Missing",
		},
		{
			name: "parameter component",
			spec: `{"openapi":"3.0.3","paths":{},"components":{"parameters":{
				"Broken":{"$ref":"#/components/parameters/Missing"}
			}}}`,
			want: "#/components/parameters/Missing",
		},
		{
			name: "request body component",
			spec: `{"openapi":"3.0.3","paths":{},"components":{"requestBodies":{
				"Broken":{"$ref":"#/components/requestBodies/Missing"}
			}}}`,
			want: "#/components/requestBodies/Missing",
		},
		{
			name: "response component",
			spec: `{"openapi":"3.0.3","paths":{},"components":{"responses":{
				"Broken":{"$ref":"#/components/responses/Missing"}
			}}}`,
			want: "#/components/responses/Missing",
		},
		{
			name: "header component",
			spec: `{"openapi":"3.0.3","paths":{},"components":{"headers":{
				"Broken":{"$ref":"#/components/headers/Missing"}
			}}}`,
			want: "#/components/headers/Missing",
		},
		{
			name: "callback component",
			spec: `{"openapi":"3.0.3","paths":{},"components":{"callbacks":{
				"Broken":{"$ref":"#/components/callbacks/Missing"}
			}}}`,
			want: "#/components/callbacks/Missing",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := Parse([]byte(test.spec))
			require.Nil(t, parsed)

			var referenceErr *oas.ReferenceError
			require.ErrorAs(t, err, &referenceErr)
			require.ErrorContains(t, err, test.want)
		})
	}
}

// TestUniqueItemsTraversalMarksEveryResolvedSchema prevents repeated suffix resolution.
func TestUniqueItemsTraversalMarksEveryResolvedSchema(t *testing.T) {
	t.Parallel()

	document := json.RawMessage(`{"components":{"schemas":{
		"First":{"$ref":"#/components/schemas/Middle"},
		"Middle":{"$ref":"#/components/schemas/Last"},
		"Last":{"type":"string"}
	}}}`)
	walker := authoredSchemaWalker{
		source:  oas.Source{Document: document},
		visited: make(map[string]struct{}),
	}
	first, err := walker.source.At("#/components/schemas/First")
	require.NoError(t, err)
	require.NoError(t, walker.schema(first.Raw, first.Pointer))

	for _, pointer := range []string{
		"#/components/schemas/First",
		"#/components/schemas/Middle",
		"#/components/schemas/Last",
	} {
		_, visited := walker.visited["schema\x00"+pointer]
		require.True(t, visited, pointer)
	}
}

// TestUniqueItemsTraversalResolvesNonSchemaReferenceChainOnce prevents quadratic suffix traversal.
func TestUniqueItemsTraversalResolvesNonSchemaReferenceChainOnce(t *testing.T) {
	t.Parallel()

	const chainLength = 64

	responses := make(map[string]any, chainLength)
	for index := range chainLength {
		name := fmt.Sprintf("R%02d", index)
		if index == chainLength-1 {
			responses[name] = map[string]any{"description": "terminal"}

			continue
		}

		responses[name] = map[string]any{
			"$ref": fmt.Sprintf("#/components/responses/R%02d", index+1),
		}
	}

	document, err := json.Marshal(map[string]any{
		"components": map[string]any{"responses": responses},
	})
	require.NoError(t, err)

	walker := authoredSchemaWalker{
		source:  oas.Source{Document: document},
		visited: make(map[string]struct{}),
	}
	resolutionCount := 0

	for index := range chainLength {
		pointer := fmt.Sprintf("#/components/responses/R%02d", index)
		response, atErr := walker.source.At(pointer)
		require.NoError(t, atErr)

		_, _, resolved, resolveErr := walker.resolve("response", response.Raw, response.Pointer)
		require.NoError(t, resolveErr)

		if resolved {
			resolutionCount++
		}
	}

	require.Equal(t, 1, resolutionCount)
}

// TestUniqueItemsTraversalAllocationsScaleLinearlyWithInlineDepth bounds whole-tree decoding costs.
//
//nolint:paralleltest // Per-process allocation counts must run without concurrent tests.
func TestUniqueItemsTraversalAllocationsScaleLinearlyWithInlineDepth(t *testing.T) {
	document := func(depth int) json.RawMessage {
		return json.RawMessage(
			`{"components":{"schemas":{"Deep":` +
				strings.Repeat(`{"items":`, depth) + `{}` + strings.Repeat(`}`, depth) +
				`}}}`,
		)
	}

	allocatedBytes := func(depth int) int64 {
		raw := document(depth)
		result := testing.Benchmark(func(benchmark *testing.B) {
			for benchmark.Loop() {
				if err := rejectAuthoredSchemaExclusions(raw); err != nil {
					panic(err)
				}
			}
		})

		return result.AllocedBytesPerOp()
	}

	shallow := allocatedBytes(64)
	deep := allocatedBytes(128)
	require.Less(t, deep, shallow*5/2)
}

// TestUniqueItemsTraversalReusesResolvedSchemaTargets bounds repeated-reference traversal costs.
//
//nolint:paralleltest // Per-process allocation counts must run without concurrent tests.
func TestUniqueItemsTraversalReusesResolvedSchemaTargets(t *testing.T) {
	document := func(references int) json.RawMessage {
		schemas := make(map[string]any, references+1)
		for index := range references {
			schemas[fmt.Sprintf("Alias%02d", index)] = map[string]any{"$ref": "#/components/schemas/ZTarget"}
		}

		var target any = map[string]any{"type": "string"}
		for range 256 {
			target = map[string]any{"items": target}
		}

		schemas["ZTarget"] = target

		raw, err := json.Marshal(map[string]any{"components": map[string]any{"schemas": schemas}})
		require.NoError(t, err)

		return raw
	}

	allocatedBytes := func(references int) int64 {
		raw := document(references)
		result := testing.Benchmark(func(benchmark *testing.B) {
			for benchmark.Loop() {
				if err := rejectAuthoredSchemaExclusions(raw); err != nil {
					panic(err)
				}
			}
		})

		return result.AllocedBytesPerOp()
	}

	few := allocatedBytes(2)
	many := allocatedBytes(16)
	require.Less(t, many, few*5/2)
}

// TestParseIgnoresUniqueItemsSiblingOnReferenceObjects distinguishes Reference and Schema Objects.
func TestParseIgnoresUniqueItemsSiblingOnReferenceObjects(t *testing.T) {
	t.Parallel()

	parsed, err := Parse(openAPISpec(
		`{"$ref":"#/components/schemas/Target","uniqueItems":false}`,
		`,"components":{"schemas":{"Target":{"type":"array","items":{}}}}`,
		false,
	))
	require.NoError(t, err)
	require.Contains(t, parsed, "checkThing")
}

// TestParseRejectsUniqueItemsInEveryNestedSchemaKeyword covers the fixed nested traversal order.
func TestParseRejectsUniqueItemsInEveryNestedSchemaKeyword(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		schema  string
		pointer string
	}{
		{name: "items", schema: `{"items":{"uniqueItems":false}}`, pointer: "/items/uniqueItems"},
		{
			name: "properties", schema: `{"properties":{"a/b~c":{"uniqueItems":false}}}`,
			pointer: "/properties/a~1b~0c/uniqueItems",
		},
		{
			name: "additional properties", schema: `{"additionalProperties":{"uniqueItems":false}}`,
			pointer: "/additionalProperties/uniqueItems",
		},
		{name: "allOf", schema: `{"allOf":[{"uniqueItems":false}]}`, pointer: "/allOf/0/uniqueItems"},
		{name: "anyOf", schema: `{"anyOf":[{"uniqueItems":false}]}`, pointer: "/anyOf/0/uniqueItems"},
		{name: "oneOf", schema: `{"oneOf":[{"uniqueItems":false}]}`, pointer: "/oneOf/0/uniqueItems"},
		{name: "not", schema: `{"not":{"uniqueItems":false}}`, pointer: "/not/uniqueItems"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			spec := fmt.Appendf(nil, `{"openapi":"3.0.3","paths":{},"components":{"schemas":{"Only":%s}}}`, test.schema)
			parsed, err := Parse(spec)
			require.Nil(t, parsed)
			require.Error(t, err)
			require.ErrorContains(t, err, "#/components/schemas/Only"+test.pointer)
		})
	}
}

// TestParseIgnoresUniqueItemsOnIntermediateReferenceObjects covers resolved reference chains.
func TestParseIgnoresUniqueItemsOnIntermediateReferenceObjects(t *testing.T) {
	t.Parallel()

	parsed, err := Parse([]byte(`{
		"openapi":"3.0.3",
		"paths":{
			"/first":{"post":{"operationId":"alpha","requestBody":{"content":{"application/json":{
				"schema":{"$ref":"#/components/schemas/First"}
			}}}}},
			"/later":{"post":{"operationId":"beta","requestBody":{"content":{"application/json":{
				"schema":{}
			}}}}}
		},
		"components":{"schemas":{
			"First":{"$ref":"#/x-chain/middle"},
			"Final":{}
		}},
		"x-chain":{"middle":{"$ref":"#/components/schemas/Final","uniqueItems":false}}
	}`))
	require.NoError(t, err)
	require.Contains(t, parsed, "alpha")
	require.Contains(t, parsed, "beta")
}

// TestParsePreservesEarlierErrorsBeforeCompleteUniqueItemsTraversal locks ordinary Parse precedence.
func TestParsePreservesEarlierErrorsBeforeCompleteUniqueItemsTraversal(t *testing.T) {
	t.Parallel()

	parsed, err := Parse([]byte(`{
		"openapi":"3.0.3",
		"paths":{"not-a-path":{}},
		"components":{"schemas":{"Only":{"uniqueItems":false}}}
	}`))
	require.Nil(t, parsed)
	require.ErrorContains(t, err, "must begin with /")
	require.NotContains(t, err.Error(), "uniqueItems")
}

// TestParseSelectsUniqueItemsDeterministically uses lexical maps and the documented nested keyword order.
func TestParseSelectsUniqueItemsDeterministically(t *testing.T) {
	t.Parallel()

	parsed, err := Parse([]byte(`{
		"openapi":"3.0.3",
		"paths":{},
		"components":{"schemas":{
			"Zulu":{"uniqueItems":false},
			"Alpha":{"properties":{"z":{"uniqueItems":false}},"items":{"uniqueItems":false}}
		}}
	}`))
	require.Nil(t, parsed)
	require.ErrorContains(t, err, "#/components/schemas/Alpha/items/uniqueItems")
}

// TestNumericValidationReusesCompiledBoundsAcrossArrayItems bounds request-time bound parsing.
//
//nolint:paralleltest // Per-process allocation counts must run without concurrent tests.
func TestNumericValidationReusesCompiledBoundsAcrossArrayItems(t *testing.T) {
	digits := strings.Repeat("9", 1<<12)
	minimum := "-" + digits
	maximum := digits
	multipleOf := "0." + strings.Repeat("0", 1<<12) + "1"
	validation := mustParseSchema(t, `{"type":"array","items":{"type":"number","minimum":`+
		minimum+`,"maximum":`+maximum+`,"multipleOf":`+multipleOf+`}}`, "")

	allocatedBytes := func(items int) int64 {
		body := json.RawMessage(`[` + strings.TrimSuffix(strings.Repeat("0,", items), ",") + `]`)
		result := testing.Benchmark(func(benchmark *testing.B) {
			for benchmark.Loop() {
				if errs := validation.Validate(body); len(errs) != 0 {
					panic(errors.Join(errs...))
				}
			}
		})

		return result.AllocedBytesPerOp()
	}

	many := allocatedBytes(16)
	require.Less(t, many, int64((len(minimum)+len(maximum)+len(multipleOf))*64))
}

// TestParseAllowsEmptySchemaPropertyNames covers JSON's empty object member name.
func TestParseAllowsEmptySchemaPropertyNames(t *testing.T) {
	t.Parallel()

	validation := mustParseSchema(t, `{
		"type":"object",
		"required":[""],
		"properties":{"":{"type":"string"}},
		"additionalProperties":false
	}`, "")

	require.Empty(t, validation.Validate(json.RawMessage(`{"":"value"}`)))
	require.NotEmpty(t, validation.Validate(json.RawMessage(`{}`)))
}

// TestParseRejectsUnsupportedAndMalformedReachableSchemas covers every parse-time rejection.
func TestParseRejectsUnsupportedAndMalformedReachableSchemas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		components string
		want       string
	}{
		{name: "oneOf", schema: `{"oneOf":[{}]}`, want: "oneOf"},
		{name: "not", schema: `{"not":{}}`, want: "not"},
		{name: "externalRef", schema: `{"$ref":"other.yaml#/Thing"}`, want: "external reference"},
		{name: "unsupportedPattern", schema: `{"pattern":"x(?=a)"}`, want: "unsupported"},
		{name: "unknownKeyword", schema: `{"const":1}`, want: "unsupported Schema Object keyword"},
		{name: "malformedBound", schema: `{"minItems":-1}`, want: "minItems"},
		{name: "arrayWithoutItems", schema: `{"type":"array"}`, want: "items"},
		{name: "malformedMultiple", schema: `{"multipleOf":0}`, want: "greater than zero"},
		{name: "malformedRequired", schema: `{"required":["a","a"]}`, want: "unique strings"},
		{name: "wrongDefaultType", schema: `{"type":"string","default":1}`, want: "must conform to type"},
		{name: "fractionalIntegerDefault", schema: `{"type":"integer","default":1.5}`, want: "must conform to type"},
		{name: "externalDocsWithoutURL", schema: `{"externalDocs":{"description":"docs"}}`, want: "url is required"},
		{
			name: "externalDocsWrongDescription", schema: `{"externalDocs":{"url":"/docs","description":1}}`,
			want: "description",
		},
		{
			name: "externalDocsUnknownField", schema: `{"externalDocs":{"url":"/docs","other":true}}`,
			want: "unsupported field",
		},
		{name: "xmlWrongName", schema: `{"xml":{"name":1}}`, want: "name"},
		{name: "xmlRelativeNamespace", schema: `{"xml":{"namespace":"/relative"}}`, want: "absolute URI"},
		{name: "xmlUnknownField", schema: `{"xml":{"other":true}}`, want: "unsupported field"},
		{
			name:       "nestedUnsupported",
			schema:     `{"properties":{"value":{"$ref":"#/components/schemas/Bad"}}}`,
			components: `,"components":{"schemas":{"Bad":{"oneOf":[{}]}}}`,
			want:       "oneOf",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(openAPISpec(test.schema, test.components, false))
			require.ErrorContains(t, err, test.want)
		})
	}

	parsed, err := Parse([]byte(`{"openapi":"3.0.3","paths":{"/a":{"post":{"operationId":"unused"}}}}`))
	require.NoError(t, err)
	require.Equal(t, map[string]RequestValidation{"unused": {}}, parsed)
}

// TestParseAcceptsBooleanReadOnlyAndWriteOnlyAtTheRoot verifies annotation shape without property semantics.
func TestParseAcceptsBooleanReadOnlyAndWriteOnlyAtTheRoot(t *testing.T) {
	t.Parallel()

	for _, schema := range []string{
		`{"readOnly":false}`,
		`{"readOnly":true}`,
		`{"writeOnly":false}`,
		`{"writeOnly":true}`,
		`{"readOnly":true,"writeOnly":true}`,
		`{"allOf":[{"readOnly":true,"writeOnly":true}]}`,
	} {
		t.Run(schema, func(t *testing.T) {
			t.Parallel()

			mustParseSchema(t, schema, "")
		})
	}
}

// TestValidationKeepsRequiredWriteOnlyRequestPropertiesRequired verifies request requiredness is unchanged.
func TestValidationKeepsRequiredWriteOnlyRequestPropertiesRequired(t *testing.T) {
	t.Parallel()

	validation := mustParseSchema(t, `{
		"type":"object","required":["secret"],
		"properties":{"secret":{"type":"string","writeOnly":true}}
	}`, "")
	errs := validation.Validate(json.RawMessage(`{}`))
	require.NotEmpty(t, errs)
	require.Contains(t, errors.Join(errs...).Error(), "keyword required")
	require.Empty(t, validation.Validate(json.RawMessage(`{"secret":"kept"}`)))
}

// TestValidationDoesNotPropagateRequestDirectionAcrossAllOf verifies branch-local annotations remain inert.
func TestValidationDoesNotPropagateRequestDirectionAcrossAllOf(t *testing.T) {
	t.Parallel()

	validation := mustParseSchema(t, `{
		"type":"object","required":["value"],
		"properties":{"value":{"allOf":[
			{"type":"string","readOnly":true},
			{"minLength":2,"writeOnly":true}
		]}}
	}`, "")
	errs := validation.Validate(json.RawMessage(`{}`))
	require.NotEmpty(t, errs)
	require.Contains(t, errors.Join(errs...).Error(), "keyword required")
	require.Empty(t, validation.Validate(json.RawMessage(`{"value":"ok"}`)))
}

// TestParseRejectsMalformedReadOnlyAndWriteOnlyAtEverySchemaShape verifies annotation shape recursively.
func TestParseRejectsMalformedReadOnlyAndWriteOnlyAtEverySchemaShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		components string
		pointer    string
	}{
		{
			name: "root readOnly", schema: `{"readOnly":"yes"}`,
			pointer: "#/paths/~1things/post/requestBody/content/application~1json/schema/readOnly",
		},
		{
			name: "property writeOnly", schema: `{"properties":{"value":{"writeOnly":null}}}`,
			pointer: "#/paths/~1things/post/requestBody/content/application~1json/schema/properties/value/writeOnly",
		},
		{
			name: "resolved reference readOnly", schema: `{"$ref":"#/components/schemas/Value"}`,
			components: `,"components":{"schemas":{"Value":{"readOnly":[]}}}`,
			pointer:    "#/components/schemas/Value/readOnly",
		},
		{
			name: "allOf writeOnly", schema: `{"allOf":[{"writeOnly":1}]}`,
			pointer: "#/paths/~1things/post/requestBody/content/application~1json/schema/allOf/0/writeOnly",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(openAPISpec(test.schema, test.components, false))
			require.Error(t, err)
			require.ErrorContains(t, err, "compile schema at "+test.pointer)
			require.ErrorContains(t, err, "must be a boolean")
		})
	}
}

// TestParseRejectsReadOnlyAndWriteOnlyTogetherOnRequestProperties verifies property-only direction semantics.
func TestParseRejectsReadOnlyAndWriteOnlyTogetherOnRequestProperties(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		components string
		pointer    string
	}{
		{
			name:    "direct",
			schema:  `{"properties":{"value":{"readOnly":true,"writeOnly":true}}}`,
			pointer: "#/paths/~1things/post/requestBody/content/application~1json/schema/properties/value",
		},
		{
			name:       "resolved reference",
			schema:     `{"properties":{"value":{"$ref":"#/components/schemas/Value"}}}`,
			components: `,"components":{"schemas":{"Value":{"readOnly":true,"writeOnly":true}}}`,
			pointer:    "#/components/schemas/Value",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(openAPISpec(test.schema, test.components, false))
			require.Error(t, err)
			require.ErrorContains(t, err, "compile schema at "+test.pointer)
			require.ErrorContains(t, err, "readOnly and writeOnly must not both be true")
		})
	}
}

// TestValidationMakesRequiredReadOnlyRequestPropertiesOptional verifies omission and supplied-value validation.
func TestValidationMakesRequiredReadOnlyRequestPropertiesOptional(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		property   string
		components string
	}{
		{name: "direct", property: `{"type":"string","minLength":2,"readOnly":true}`},
		{
			name:       "resolved reference",
			property:   `{"$ref":"#/components/schemas/Identifier"}`,
			components: `,"components":{"schemas":{"Identifier":{"type":"string","minLength":2,"readOnly":true}}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			validation := mustParseSchema(t, `{"type":"object","required":["id"],"properties":{"id":`+
				test.property+`}}`, test.components)
			require.Empty(t, validation.Validate(json.RawMessage(`{}`)))
			require.Empty(t, validation.Validate(json.RawMessage(`{"id":"ok"}`)))

			errs := validation.Validate(json.RawMessage(`{"id":"x"}`))
			require.NotEmpty(t, errs)
			require.Contains(t, errors.Join(errs...).Error(), "keyword minLength")
		})
	}
}

// TestValidationLocksRequestDirectionCombinations covers absent and supplied properties for every direction pair.
func TestValidationLocksRequestDirectionCombinations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		annotations  string
		absentValid  bool
		parseFailure bool
	}{
		{name: "annotations omitted", absentValid: false},
		{
			name:        "both false",
			annotations: `,"readOnly":false,"writeOnly":false`,
			absentValid: false,
		},
		{
			name:        "read only",
			annotations: `,"readOnly":true,"writeOnly":false`,
			absentValid: true,
		},
		{
			name:        "write only",
			annotations: `,"readOnly":false,"writeOnly":true`,
			absentValid: false,
		},
		{
			name:         "both true",
			annotations:  `,"readOnly":true,"writeOnly":true`,
			parseFailure: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			schema := fmt.Sprintf(
				`{"type":"object","required":["value"],"properties":{"value":{"type":"string","minLength":2%s}}}`,
				test.annotations,
			)

			if test.parseFailure {
				parsed, err := Parse(openAPISpec(schema, "", false))
				require.Nil(t, parsed)
				require.ErrorContains(t, err, "compile schema at "+
					"#/paths/~1things/post/requestBody/content/application~1json/schema/properties/value")
				require.ErrorContains(t, err, "readOnly and writeOnly must not both be true")

				return
			}

			parsed := mustParseSchema(t, schema, "")

			absentErrors := parsed.Validate(json.RawMessage(`{}`))
			if test.absentValid {
				require.Empty(t, absentErrors)
			} else {
				require.Contains(t, errors.Join(absentErrors...).Error(), "keyword required")
			}

			require.Empty(t, parsed.Validate(json.RawMessage(`{"value":"okay"}`)))
			require.Contains(t, errors.Join(parsed.Validate(json.RawMessage(`{"value":"x"}`))...).Error(), "keyword minLength")
		})
	}
}

// TestParseRejectsRecursiveSchemas makes finite, acyclic Parse results an explicit contract.
func TestParseRejectsRecursiveSchemas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		components string
	}{
		{
			name: "objectProperty",
			components: `,"components":{"schemas":{"Loop":{"type":"object","properties":{
				"child":{"$ref":"#/components/schemas/Loop"}
			}}}}`,
		},
		{
			name: "arrayItem",
			components: `,"components":{"schemas":{"Loop":{"type":"array",
				"items":{"$ref":"#/components/schemas/Loop"}
			}}}`,
		},
		{
			name: "allOf",
			components: `,"components":{"schemas":{"Loop":{"allOf":[
				{"$ref":"#/components/schemas/Loop"}
			]}}}`,
		},
		{
			name: "mutual",
			components: `,"components":{"schemas":{
				"Loop":{"type":"object","properties":{"other":{"$ref":"#/components/schemas/Other"}}},
				"Other":{"type":"object","properties":{"loop":{"$ref":"#/components/schemas/Loop"}}}
			}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(openAPISpec(`{"$ref":"#/components/schemas/Loop"}`, test.components, false))
			require.ErrorContains(t, err, `compile operationId "checkThing"`)
			require.ErrorContains(t, err, "recursive schema is unsupported")
			require.ErrorContains(t, err, "#/components/schemas/Loop")
		})
	}
}

// TestParseAcceptsWellFormedDocumentationFields guards the supported documentation-only shapes.
func TestParseAcceptsWellFormedDocumentationFields(t *testing.T) {
	t.Parallel()

	parsed := mustParseSchema(t, `{
		"type":"string",
		"nullable":true,
		"default":null,
		"title":"Thing",
		"description":"A thing",
		"deprecated":false,
		"xml":{
			"name":"thing",
			"namespace":"https://example.com/things",
			"prefix":"t",
			"attribute":false,
			"wrapped":true,
			"x-extra":1
		},
		"externalDocs":{
			"description":"More details",
			"url":"https://example.com/docs",
			"x-extra":1
		}
	}`, "")

	require.Empty(t, parsed.Validate(json.RawMessage(`null`)))
}

// TestParseAcceptsArbitrarilyLargeCollectionBounds covers the unbounded OAS integer domain.
func TestParseAcceptsArbitrarilyLargeCollectionBounds(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		keyword    string
		body       string
		wantErrors bool
	}{
		{name: "minLength", keyword: "minLength", body: `"x"`, wantErrors: true},
		{name: "maxLength", keyword: "maxLength", body: `"x"`},
		{name: "minItems", keyword: "minItems", body: `[]`, wantErrors: true},
		{name: "maxItems", keyword: "maxItems", body: `[]`},
		{name: "minProperties", keyword: "minProperties", body: `{}`, wantErrors: true},
		{name: "maxProperties", keyword: "maxProperties", body: `{}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parsed := mustParseSchema(t, fmt.Sprintf(`{%q:1e100001}`, test.keyword), "")

			errs := parsed.Validate(json.RawMessage(test.body))
			if test.wantErrors {
				require.Contains(t, errors.Join(errs...).Error(), "keyword "+test.keyword)
			} else {
				require.Empty(t, errs)
			}
		})
	}
}

// TestParseAcceptsCompatibleOpenAPIVersions verifies patch-level compatibility.
func TestParseAcceptsCompatibleOpenAPIVersions(t *testing.T) {
	t.Parallel()

	valid := openAPISpec(`{}`, "", false)

	for _, version := range []string{
		"3.0.0",
		"3.0.4",
		"3.0.10",
		"3.0.999",
		"3.0.4-rc.1",
		"3.0.4-0.3.7+001",
		"3.0.4+vendor",
		"3.0.4-rc.1+vendor",
		"3.0.4-x-y-z.--+exp.sha.5114f85",
	} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			spec := strings.Replace(string(valid), `"3.0.3"`, strconv.Quote(version), 1)
			_, err := Parse([]byte(spec))
			require.NoError(t, err)
		})
	}
}

// TestParseRejectsUnsupportedOpenAPIVersions enforces this package's feature-set contract.
func TestParseRejectsUnsupportedOpenAPIVersions(t *testing.T) {
	t.Parallel()

	valid := openAPISpec(`{}`, "", false)

	const (
		versionSyntaxError = "#/openapi: OpenAPI document version must be a Semantic Versioning 2.0.0 version"
		featureSetError    = "#/openapi: OpenAPI document feature set must be 3.0"
	)

	for _, test := range []struct {
		name        string
		replacement string
		wantError   string
	}{
		{name: "leading zero major", replacement: `"03.0.4"`, wantError: versionSyntaxError},
		{name: "leading zero minor", replacement: `"3.00.4"`, wantError: versionSyntaxError},
		{name: "leading zero", replacement: `"3.0.04"`, wantError: versionSyntaxError},
		{name: "missing patch", replacement: `"3.0"`, wantError: versionSyntaxError},
		{name: "leading version marker", replacement: `"v3.0.4"`, wantError: versionSyntaxError},
		{name: "leading zero prerelease", replacement: `"3.0.4-01"`, wantError: versionSyntaxError},
		{name: "empty prerelease", replacement: `"3.0.4-"`, wantError: versionSyntaxError},
		{name: "empty build", replacement: `"3.0.4+"`, wantError: versionSyntaxError},
		{name: "unsupported feature set", replacement: `"3.1.0"`, wantError: featureSetError},
		{name: "number", replacement: `3.0`, wantError: versionSyntaxError},
		{name: "null", replacement: `null`, wantError: versionSyntaxError},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			spec := strings.Replace(string(valid), `"3.0.3"`, test.replacement, 1)
			_, err := Parse([]byte(spec))
			require.ErrorContains(t, err, test.wantError)
		})
	}

	missing := strings.Replace(string(valid), `"openapi":"3.0.3",`, "", 1)
	_, err := Parse([]byte(missing))
	require.ErrorContains(t, err, versionSyntaxError)
}

// TestParsePreservesOpenAPIVersionDecodeError keeps invalid field-type context available to callers.
func TestParsePreservesOpenAPIVersionDecodeError(t *testing.T) {
	t.Parallel()

	valid := openAPISpec(`{}`, "", false)
	spec := strings.Replace(string(valid), `"3.0.3"`, `3.0`, 1)

	_, err := Parse([]byte(spec))

	var typeError *json.UnmarshalTypeError
	require.ErrorAs(t, err, &typeError)
}

// TestParseRejectsFirstMalformedOperationDeterministically verifies the whole-document error boundary.
func TestParseRejectsFirstMalformedOperationDeterministically(t *testing.T) {
	t.Parallel()

	spec := []byte(`{
		"openapi":"3.0.3",
		"paths":{
			"/broken-ref":{"$ref":"#/components/pathItems/Missing"},
			"/broken-operation":{"post":false},
			"/things":{"post":{
				"operationId":"checkThing",
				"requestBody":{"content":{"application/json":{"schema":{"type":"string"}}}}
			}}
		}
	}`)

	_, err := Parse(spec)
	require.ErrorContains(t, err, "#/paths/~1broken-operation/post")
}

// TestValidationErrorsAreStableAndFresh covers repeatability and caller-owned result slices.
func TestValidationErrorsAreStableAndFresh(t *testing.T) {
	t.Parallel()

	parsed := mustParseSchema(t, `{"type":"array","minItems":2,"items":{"type":"integer"}}`, "")
	body := json.RawMessage(`[1.5]`)
	first := parsed.Validate(body)
	second := parsed.Validate(body)
	require.Equal(t, errorStrings(first), errorStrings(second))
	require.Len(t, first, 2)
	first[0] = errors.New("caller mutation")

	require.Equal(t, errorStrings(second), errorStrings(parsed.Validate(body)))
}

// TestValidationConcurrentHighContention proves one parsed graph is immutable across concurrent calls.
//
//nolint:cyclop // The required barrier, mutation probe, and buffered mismatch reporting are one concurrency scenario.
func TestValidationConcurrentHighContention(t *testing.T) {
	t.Parallel()

	components := `,"components":{"schemas":{"Node":{"type":"object","required":["name","amount","children"],"properties":{
		"name":{"type":"string","pattern":"^[a-z]+$"},
		"amount":{"type":"integer","minimum":9007199254740993,"multipleOf":3},
		"children":{"type":"array","items":{"$ref":"#/components/schemas/Child"}}
	},"additionalProperties":false,"allOf":[{"minProperties":3}]},
	"Child":{"type":"object","required":["name","amount","children"],"properties":{
		"name":{"type":"string","pattern":"^[a-z]+$"},
		"amount":{"type":"integer","minimum":9007199254740993,"multipleOf":3},
		"children":{"type":"array","items":{"type":"string"}}
	},"additionalProperties":false,"allOf":[{"minProperties":3}]}}}`
	parsed := mustParseSchema(t, `{"$ref":"#/components/schemas/Node"}`, components)

	bodies := []json.RawMessage{
		json.RawMessage(`{"name":"root","amount":9007199254740993,"children":[]}`),
		json.RawMessage(`{"name":"BAD","amount":9007199254740992,"children":[]}`),
		json.RawMessage(
			`{"name":"root","amount":9007199254740993,"children":` +
				`[{"name":"child","amount":9007199254740996,"children":[]}]}`,
		),
		json.RawMessage(`{"name":"root","amount":9007199254740994,"children":[],"extra":true}`),
	}

	expected := make([][]string, len(bodies))
	for index, body := range bodies {
		expected[index] = errorStrings(parsed.Validate(body))
	}

	const (
		goroutineCount    = 256
		callsPerGoroutine = 250
	)

	start := make(chan struct{})
	mismatches := make(chan string, goroutineCount)

	var waitGroup sync.WaitGroup
	waitGroup.Add(goroutineCount)

	for worker := range goroutineCount {
		go func() {
			defer waitGroup.Done()

			<-start

			for iteration := range callsPerGoroutine {
				bodyIndex := (worker + iteration) % len(bodies)

				errs := parsed.Validate(bodies[bodyIndex])
				if got := errorStrings(errs); !equalStrings(got, expected[bodyIndex]) {
					select {
					case mismatches <- fmt.Sprintf(
						"worker %d iteration %d: got %v want %v", worker, iteration, got, expected[bodyIndex],
					):
					default:
					}
				}

				for index := range errs {
					errs[index] = errors.New("caller mutation")
				}

				if got := errorStrings(parsed.Validate(bodies[bodyIndex])); !equalStrings(got, expected[bodyIndex]) {
					select {
					case mismatches <- fmt.Sprintf(
						"worker %d iteration %d after mutation: got %v want %v",
						worker, iteration, got, expected[bodyIndex],
					):
					default:
					}
				}
			}
		}()
	}

	close(start)
	waitGroup.Wait()
	close(mismatches)

	for mismatch := range mismatches {
		t.Error(mismatch)
	}
}

// identity returns its input unchanged.
func identity(value string) string {
	return value
}

// mustParseSchema builds one optional-body OpenAPI fixture and requires parse success.
func mustParseSchema(t *testing.T, schema string, components string) *Validation {
	t.Helper()

	return mustParseSchemaWithRequired(t, schema, components, false)
}

// mustParseSchemaWithRequired builds one OpenAPI fixture and requires parse success.
func mustParseSchemaWithRequired(t *testing.T, schema string, components string, required bool) *Validation {
	t.Helper()

	parsedByOperation, err := Parse(openAPISpec(schema, components, required))
	require.NoError(t, err)

	return parsedByOperation["checkThing"].Body
}

// openAPISpec embeds one JSON Schema Object into one selected OpenAPI operation.
func openAPISpec(schema string, components string, required bool) []byte {
	return fmt.Appendf(nil, `{
		"openapi":"3.0.3",
		"info":{"title":"test","version":"1"},
		"paths":{"/things":{"post":{
			"operationId":"checkThing",
			"requestBody":{"required":%t,"content":{"application/json":{"schema":%s}}},
			"responses":{"204":{"description":"ok"}}
		}}}%s
	}`, required, schema, components)
}

// errorStrings copies an error sequence into comparable strings.
func errorStrings(errs []error) []string {
	if len(errs) == 0 {
		return nil
	}

	result := make([]string, len(errs))
	for index, err := range errs {
		result[index] = err.Error()
	}

	return result
}

// equalStrings compares two ordered string slices.
func equalStrings(left []string, right []string) bool {
	return strings.Join(left, "\x00") == strings.Join(right, "\x00")
}
