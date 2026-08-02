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
	}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			t.Parallel()

			documents := map[string]string{
				"json": documentWithJSONSchema(`{"type":"string","minLength":1,"maxLength":20,"pattern":` + strconv.Quote(pattern) + `}`),
				"yaml": documentWithYAMLSchema("type: string\nminLength: 1\nmaxLength: 20\npattern: '" + pattern + "'"),
			}

			for encoding, document := range documents {
				t.Run(encoding, func(t *testing.T) {
					t.Parallel()

					model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
					require.NoError(t, err)
					require.NotNil(t, model.root.pattern)
					require.Equal(t, pattern, model.root.pattern.source)
					requireBigIntEqual(t, "1", model.root.minLength)
					requireBigIntEqual(t, "20", model.root.maxLength)
				})
			}
		})
	}
}

func TestParseInputEnforcesExactECMAPatternLimits(t *testing.T) {
	t.Parallel()

	matcherAtLimit := strings.Repeat(`\S`, 5855) + strings.Repeat("a", 3840) + strings.Repeat(".", 294)
	matcherOverLimit := strings.Repeat(`\S`, 5855) + strings.Repeat("a", 3839) + strings.Repeat(".", 295)

	tests := []struct {
		name    string
		pattern string
		accept  bool
	}{
		{name: "source_at_limit", pattern: "[" + strings.Repeat("a", patternSourceByteLimit-2) + "]", accept: true},
		{name: "source_over_limit", pattern: "[" + strings.Repeat("a", patternSourceByteLimit-1) + "]"},
		{name: "nesting_at_limit", pattern: strings.Repeat("(", patternNestingLimit) + "a" + strings.Repeat(")", patternNestingLimit), accept: true},
		{name: "nesting_over_limit", pattern: strings.Repeat("(", patternNestingLimit+1) + "a" + strings.Repeat(")", patternNestingLimit+1)},
		{name: "nodes_at_limit", pattern: strings.Repeat("a", patternNodeLimit-2), accept: true},
		{name: "nodes_over_limit", pattern: strings.Repeat("a", patternNodeLimit-1)},
		{name: "astral_nodes_at_limit", pattern: strings.Repeat("😀", patternNodeLimit-2), accept: true},
		{name: "astral_nodes_over_limit", pattern: strings.Repeat("😀", patternNodeLimit-1)},
		{name: "assertions_at_limit", pattern: "^" + strings.Repeat("(?=a)", patternLeadingAssertionLimit) + "a", accept: true},
		{name: "assertions_over_limit", pattern: "^" + strings.Repeat("(?=a)", patternLeadingAssertionLimit+1) + "a"},
		{name: "endpoint_at_limit", pattern: "a{1000}", accept: true},
		{name: "endpoint_over_limit", pattern: "a{1001}"},
		{name: "nested_repeat_at_limit", pattern: "(?:a{10}){100}", accept: true},
		{name: "nested_repeat_over_limit", pattern: "(?:a{10}){101}"},
		{name: "nested_unbounded_repeat_at_limit", pattern: "(?:a{10,}){100}", accept: true},
		{name: "nested_unbounded_repeat_over_limit", pattern: "(?:a{10,}){101}"},
		{name: "matcher_at_limit", pattern: matcherAtLimit, accept: true},
		{name: "matcher_over_limit", pattern: matcherOverLimit},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			documents := map[string]string{
				"json": documentWithJSONSchema(`{"type":"string","pattern":` + strconv.Quote(test.pattern) + `}`),
				"yaml": documentWithYAMLSchema("type: string\npattern: '" + test.pattern + "'"),
			}

			for encoding, document := range documents {
				t.Run(encoding, func(t *testing.T) {
					t.Parallel()

					model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
					if test.accept {
						require.NoError(t, err)
						require.NotNil(t, model.root.pattern)

						if test.name == "matcher_at_limit" {
							require.Equal(t, patternMatcherByteLimit, model.root.pattern.matcherBytes)
						}

						return
					}

					require.Error(t, err)
					require.Contains(t, err.Error(), "/pattern")
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
		`(`,
	}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			t.Parallel()

			documents := map[string]string{
				"json": documentWithJSONSchema(`{"type":"string","pattern":` + strconv.Quote(pattern) + `}`),
				"yaml": documentWithYAMLSchema("type: string\npattern: '" + pattern + "'"),
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
