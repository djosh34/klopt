//nolint:godoclint,lll // Pattern fixtures keep exact grammar and complexity boundaries inline.
package schematest

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseInputModelsAdmittedECMAPatternFamilies(t *testing.T) {
	t.Parallel()

	patterns := []string{
		`^a.$`,
		`(?:a|b(c))`,
		`[][^][a-z]`,
		`\d\D\w\W\s\S\b\B[\b]`,
		`\f\n\r\t\v\0\x41\u0042\cC\ca\-\]\.`,
		`a*?b+?c??d{0}e{1,}f{2,3}?`,
		`^(?=a)(?!b)a$`,
		`😀+`,
		`'`,
	}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			t.Parallel()

			documents := map[string]string{
				"json": documentWithJSONSchema(`{"type":"string","minLength":1,"maxLength":20,"pattern":` + strconv.Quote(pattern) + `}`),
				"yaml": documentWithYAMLSchema(
					"type: string\nminLength: 1\nmaxLength: 20\npattern: " + yamlSingleQuoted(pattern),
				),
			}

			for encoding, document := range documents {
				t.Run(encoding, func(t *testing.T) {
					t.Parallel()

					model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
					require.NoError(t, err)
					require.NotNil(t, model.root.pattern)
					require.Equal(t, pattern, model.root.pattern.source)
					requireCountEqual(t, "1", model.root.minLength)
					requireCountEqual(t, "20", model.root.maxLength)
				})
			}
		})
	}
}

func TestParseInputEnforcesExactECMAPatternLimits(t *testing.T) {
	t.Parallel()

	// A translated expression costs 4 bytes; each \S costs 174, each a costs 6,
	// and each dot costs 23. These fixtures total exactly 1,048,576 and 1,048,577.
	matcherAtLimit := strings.Repeat(`\S`, 5835) + strings.Repeat("a", 3661) + strings.Repeat(".", 492)
	matcherOverLimit := strings.Repeat(`\S`, 5835) + strings.Repeat("a", 3665) + strings.Repeat(".", 491)
	leadingMatcherAtLimit := "^(?=" + strings.Repeat(`\S`, 2942) + ")(?=" +
		strings.Repeat(`\S`, 2943) + strings.Repeat("a", 4077) + strings.Repeat(".", 4) + ")a"
	leadingMatcherOverLimit := "^(?=" + strings.Repeat(`\S`, 2942) + ")(?=" +
		strings.Repeat(`\S`, 2943) + strings.Repeat("a", 4081) + strings.Repeat(".", 3) + ")a"

	tests := []struct {
		name    string
		pattern string
		accept  bool
		reason  string
	}{
		{name: "source_at_limit", pattern: "[" + strings.Repeat("a", patternSourceByteLimit-2) + "]", accept: true},
		{
			name: "source_over_limit", pattern: "[" + strings.Repeat("a", patternSourceByteLimit-1) + "]",
			reason: "pattern source exceeds 65536 bytes",
		},
		{name: "nesting_at_limit", pattern: strings.Repeat("(", patternNestingLimit) + "a" + strings.Repeat(")", patternNestingLimit), accept: true},
		{
			name:    "nesting_over_limit",
			pattern: strings.Repeat("(", patternNestingLimit+1) + "a" + strings.Repeat(")", patternNestingLimit+1),
			reason:  "pattern nesting exceeds 100",
		},
		{name: "nodes_at_limit", pattern: strings.Repeat("a", patternNodeLimit-2), accept: true},
		{name: "nodes_over_limit", pattern: strings.Repeat("a", patternNodeLimit-1), reason: "pattern AST exceeds 10000"},
		{name: "astral_nodes_at_limit", pattern: strings.Repeat("😀", patternNodeLimit-2), accept: true},
		{
			name: "astral_nodes_over_limit", pattern: strings.Repeat("😀", patternNodeLimit-1),
			reason: "pattern AST exceeds 10000",
		},
		{name: "assertions_at_limit", pattern: "^" + strings.Repeat("(?=a)", patternLeadingAssertionLimit) + "a", accept: true},
		{
			name:    "assertions_over_limit",
			pattern: "^" + strings.Repeat("(?=a)", patternLeadingAssertionLimit+1) + "a",
			reason:  "leading assertions exceed 64",
		},
		{name: "endpoint_at_limit", pattern: "a{1000}", accept: true},
		{name: "endpoint_over_limit", pattern: "a{1001}", reason: "counted-repeat endpoint exceeds 1000"},
		{name: "nested_repeat_at_limit", pattern: "(?:a{10}){100}", accept: true},
		{
			name: "nested_repeat_over_limit", pattern: "(?:a{10}){101}",
			reason: "nested counted-repeat product exceeds 1000",
		},
		{name: "nested_unbounded_repeat_at_limit", pattern: "(?:a{10,}){100}", accept: true},
		{
			name: "nested_unbounded_repeat_over_limit", pattern: "(?:a{10,}){101}",
			reason: "nested counted-repeat product exceeds 1000",
		},
		{name: "matcher_at_limit", pattern: matcherAtLimit, accept: true},
		{name: "matcher_over_limit", pattern: matcherOverLimit, reason: "translated matcher source exceeds 1048576 bytes"},
		{name: "leading_matcher_at_limit", pattern: leadingMatcherAtLimit, accept: true},
		{
			name: "leading_matcher_over_limit", pattern: leadingMatcherOverLimit,
			reason: "translated matcher source exceeds 1048576 bytes",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			documents := map[string]string{
				"json": documentWithJSONSchema(`{"type":"string","pattern":` + strconv.Quote(test.pattern) + `}`),
				"yaml": documentWithYAMLSchema("type: string\npattern: " + yamlSingleQuoted(test.pattern)),
			}

			for encoding, document := range documents {
				t.Run(encoding, func(t *testing.T) {
					t.Parallel()

					model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
					if test.accept {
						require.NoError(t, err)
						require.NotNil(t, model.root.pattern)

						if test.name == "matcher_at_limit" || test.name == "leading_matcher_at_limit" {
							require.Equal(t, patternMatcherByteLimit, model.root.pattern.matcherBytes)
						}

						return
					}

					require.Error(t, err)
					require.Contains(t, err.Error(), "/pattern")
					require.Contains(t, err.Error(), test.reason)
				})
			}
		})
	}
}

func TestParseInputRejectsOutsideProfileECMAPatterns(t *testing.T) {
	t.Parallel()

	patterns := []string{
		`(a)\1`,
		`\uD800`,
		`(?<=a)b`,
		`(?<name>a)`,
		`\k<name>`,
		`(?i:a)`,
		`(?>a)`,
		`(*FAIL)`,
		`\p{L}`,
		`\u{41}`,
		`a*+`,
		`[[:alpha:]]`,
		`[a&&b]`,
		`[a--b]`,
		`^(?=(?=a))a`,
		`a(?=b)`,
		`^(?=a)+a`,
		`(?=a)a`,
		`^(?=a)a|b`,
		`^(?=a)`,
		`^(?=a)$`,
		`^(?=a)\b`,
		`^(?=a)(?:)`,
		`^(?=a)a{0}`,
		`\é`,
		`[\é]`,
		`[z-a]`,
		`a{2,1}`,
		`(`,
	}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			t.Parallel()

			documents := map[string]string{
				"json": documentWithJSONSchema(`{"type":"string","pattern":` + strconv.Quote(pattern) + `}`),
				"yaml": documentWithYAMLSchema("type: string\npattern: " + yamlSingleQuoted(pattern)),
			}

			for encoding, document := range documents {
				t.Run(encoding, func(t *testing.T) {
					t.Parallel()

					_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
					require.Error(t, err)
					require.Contains(t, err.Error(), "#/paths/~1/post/requestBody/content/application~1json/schema/pattern")
				})
			}
		})
	}
}

func yamlSingleQuoted(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
