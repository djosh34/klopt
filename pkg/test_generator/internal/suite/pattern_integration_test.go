package suite

import (
	"errors"
	"testing"
	"unicode/utf8"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // Required shared module; the plan forbids changing lint config.
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/stretchr/testify/require"
)

// TestPatternSuiteConstructsEqualChildLanguagesIndependently covers occurrence-local object generation.
func TestPatternSuiteLiftsIsolatedChildFailures(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
		check  func(t *testing.T, value jsonvalue.Value)
	}{
		{
			name: "object property",
			schema: `type: object
required: [value]
maxProperties: 1
additionalProperties: false
properties:
  value: {type: string, pattern: '^x$'}`,
			check: func(t *testing.T, value jsonvalue.Value) {
				t.Helper()
				require.Equal(t, jsonvalue.KindObject, value.Kind)
				require.Len(t, value.Object, 1)
				require.Equal(t, jsonvalue.KindString, value.Object[0].Value.Kind)
				require.NotEqual(t, "x", value.Object[0].Value.String)
			},
		},
		{
			name: "array item",
			schema: `type: array
minItems: 1
maxItems: 1
items: {type: string, pattern: '^x$'}`,
			check: func(t *testing.T, value jsonvalue.Value) {
				t.Helper()
				require.Equal(t, jsonvalue.KindArray, value.Kind)
				require.Len(t, value.Array, 1)
				require.Equal(t, jsonvalue.KindString, value.Array[0].Kind)
				require.NotEqual(t, "x", value.Array[0].String)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			compiled, err := NewCompiler(parseSchemaSource(t, test.schema, "", "create")).CompileSuite()
			require.NoError(t, err)

			patternCases := 0

			for _, plannedCase := range compiled.Cases {
				if plannedCase.Expect != ExpectRejected || plannedCase.Source.Keyword != "pattern" {
					continue
				}

				patternCases++

				for seed := range 20 {
					test.check(t, plannedCase.Generator.Example(seed))
				}
			}

			require.Equal(t, 1, patternCases)
		})
	}
}

// TestPatternSuitePreservesProvenanceAndConstructsEverySignedCase covers merged valid and isolated-invalid cases.
func TestPatternSuitePreservesProvenanceAndConstructsEverySignedCase(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `
type: string
minLength: 3
pattern: ^A
allOf:
  - maxLength: 4
    pattern: Z$
  - allOf:
      - pattern: '[0-9]'
  - $ref: '#/components/schemas/Base'
`, `
components:
  schemas:
    Base:
      type: string
      pattern: '^[A-Z0-9]+$'
`, "create"))

	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	root := compiler.Source.RequestSchema.Pointer
	require.Equal(t, []ConstraintSource{
		{Pointer: root, Keyword: "pattern"},
		{Pointer: root + "/allOf/0", Keyword: "pattern"},
		{Pointer: root + "/allOf/1/allOf/0", Keyword: "pattern"},
		{Pointer: "#/components/schemas/Base", Keyword: "pattern"},
	}, stringLanguageSources(compiler.rootUse.stringLanguages))

	invalidPatterns := make(map[ConstraintSource]bool)

	for _, plannedCase := range compiled.Cases {
		for seed := range 20 {
			value := plannedCase.Generator.Example(seed)
			if value.Kind != jsonvalue.KindString {
				continue
			}

			length := utf8.RuneCountInString(value.String)
			if plannedCase.Expect == ExpectAccepted || plannedCase.Source.Keyword == "pattern" {
				require.GreaterOrEqual(t, length, 3, plannedCase.Name)
				require.LessOrEqual(t, length, 4, plannedCase.Name)
			}

			for _, pattern := range compiler.rootUse.stringLanguages {
				want := true
				if plannedCase.Expect == ExpectRejected && plannedCase.Source.Keyword == "pattern" &&
					plannedCase.Source == pattern.source {
					want = false
					invalidPatterns[pattern.source] = true
				}

				require.Equal(
					t,
					want,
					patternvalidator.MustParse(pattern.value).Validate(value.String),
					"%s: %q against %s",
					plannedCase.Name,
					value.String,
					pattern.source.Pointer,
				)
			}
		}
	}

	require.Len(t, invalidPatterns, len(compiler.rootUse.stringLanguages))
}

// TestPatternSuiteKeepsDuplicateOccurrencesAndSkipsOnlyEmptySignedCases covers exact per-occurrence skipping.
func TestPatternSuiteKeepsDuplicateOccurrencesAndSkipsOnlyEmptySignedCases(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `
type: string
maxLength: 3
allOf:
  - pattern: '^[ab]+$'
  - pattern: '^[ab]+$'
  - pattern: '^a'
`, "", "create"))

	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)
	require.Len(t, compiler.rootUse.stringLanguages, 3)
	require.Equal(t, 2, countCases(compiled.Unavailable, ExpectRejected, "pattern"))
	require.Equal(t, compiler.rootUse.stringLanguages[0].value, compiler.rootUse.stringLanguages[1].value)
	require.NotEqual(t, compiler.rootUse.stringLanguages[0].id, compiler.rootUse.stringLanguages[1].id)

	root := compiler.Source.RequestSchema.Pointer
	wantConstructed := ConstraintSource{Pointer: root + "/allOf/2", Keyword: "pattern"}
	constructed := false

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect != ExpectRejected || plannedCase.Source.Keyword != "pattern" {
			continue
		}

		require.Equal(t, wantConstructed, plannedCase.Source)

		constructed = true

		for seed := range 20 {
			value := plannedCase.Generator.Example(seed)
			require.Equal(t, jsonvalue.KindString, value.Kind)
			require.True(t, patternvalidator.MustParse(`^[ab]+$`).Validate(value.String))
			require.False(t, patternvalidator.MustParse(`^a`).Validate(value.String))
		}
	}

	require.True(t, constructed)

	for _, constraint := range compiled.Constraints {
		if constraint.Source == wantConstructed {
			require.Equal(t, ObligationPlanned, constraint.Outcome)
		} else if constraint.Source.Keyword == "pattern" {
			require.Equal(t, ObligationDominated, constraint.Outcome)
			require.Contains(t, constraint.Reason, "empty over ASCII")
		}
	}
}

// TestPatternSuiteIntersectsEnumCandidatesAndKeepsPatternProvenance covers finite siblings.
func TestPatternSuitePropagatesOptionsAndAttributesConstructionErrors(t *testing.T) {
	t.Parallel()

	t.Run("raw Go option", func(t *testing.T) {
		t.Parallel()

		compiler := NewCompiler(
			parseSchemaSource(t, "type: string\nmaxLength: 3\npattern: '(?m)^a$'", "", "create"),
			patternvalidator.UseRE2,
		)
		compiled, err := compiler.CompileSuite()
		require.NoError(t, err)
		require.True(t, hasAcceptedCase(compiled.Cases))
	})

	for _, test := range []struct {
		name    string
		pattern string
		option  patternvalidator.Option
	}{
		{name: "default foreign syntax", pattern: "(?i)a"},
		{name: "raw capability", pattern: "(?i)a", option: patternvalidator.UseRE2},
		{name: "strict ASCII policy", pattern: "é", option: patternvalidator.RejectNonASCII},
		{name: "strict escaped ASCII policy", pattern: `\u00e9`, option: patternvalidator.RejectNonASCII},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source := parseSchemaSource(t, "type: string\npattern: '"+test.pattern+"'", "", "create")

			compiler := NewCompiler(source)
			if test.option != nil {
				compiler = NewCompiler(source, test.option)
			}

			_, err := compiler.CompileSuite()
			require.Error(t, err)

			var compileError *Error
			require.ErrorAs(t, err, &compileError)
			require.Equal(t, compiler.Source.RequestSchema.Pointer, compileError.Pointer)
			require.Equal(t, "pattern", compileError.Keyword)

			var emptyError *stringlanguage.EmptyError
			require.False(t, errors.As(err, &emptyError))
		})
	}

	t.Run("later allOf source", func(t *testing.T) {
		t.Parallel()

		compiler := NewCompiler(parseSchemaSource(t, `type: string
allOf:
  - pattern: '^x$'
  - pattern: '(?i)y'`, "", "create"))

		_, err := compiler.CompileSuite()
		require.Error(t, err)

		var compileError *Error
		require.ErrorAs(t, err, &compileError)
		require.Equal(t, compiler.Source.RequestSchema.Pointer+"/allOf/1", compileError.Pointer)
		require.Equal(t, "pattern", compileError.Keyword)
	})
}

// countCases returns exact expected-result and source-keyword matches.
func countCases(cases []CasePlan, expect ExpectedResult, keyword string) int {
	count := 0

	for _, plannedCase := range cases {
		if plannedCase.Expect == expect && plannedCase.Source.Keyword == keyword {
			count++
		}
	}

	return count
}

// stringLanguageSources returns declaration sources in composition order.
func stringLanguageSources(occurrences []stringLanguageOccurrence) []ConstraintSource {
	result := make([]ConstraintSource, 0, len(occurrences))
	for _, occurrence := range occurrences {
		result = append(result, occurrence.source)
	}

	return result
}

// constraintBySource returns one required test constraint.
func constraintBySource(t *testing.T, compiled *CompiledSuite, source ConstraintSource) ConstraintPlan {
	t.Helper()

	for _, constraint := range compiled.Constraints {
		if constraint.Source == source {
			return constraint
		}
	}

	require.FailNow(t, "constraint not found", source)

	return ConstraintPlan{}
}
