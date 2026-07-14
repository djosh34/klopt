package suite

import (
	"encoding/json"
	"strconv"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/internal/jsonvalue"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestRapidGeneratorBuilderMemoizesDomains verifies that each Domain generator is built once.
func TestRapidGeneratorBuilderMemoizesDomains(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: array
minItems: 2
maxItems: 4
items: {type: boolean}`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	builder := NewRapidGeneratorBuilder(compiled.Domains, compiler.rootUse)
	first, err := builder.Generator(compiled.Root)
	require.NoError(t, err)
	second, err := builder.Generator(compiled.Root)
	require.NoError(t, err)
	require.Same(t, first, second)
}

// TestCompileSuiteGeneratorsConstructEveryPlannedDomain verifies every planned generator against its Domain.
func TestCompileSuiteGeneratorsConstructEveryPlannedDomain(t *testing.T) {
	t.Parallel()

	schema := `type: object
minProperties: 3
maxProperties: 5
required: [name, score, flags]
properties:
  name:
    type: string
    minLength: 2
    maxLength: 4
  score:
    type: number
    minimum: 0.25
    exclusiveMinimum: true
    maximum: 10.25
    multipleOf: 0.5
  flags:
    type: array
    minItems: 1
    maxItems: 3
    items: {type: boolean}
  note:
    type: string
    minLength: 1
additionalProperties:
  type: integer`
	compiler := NewCompiler(parseSchemaSource(t, schema, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)
	require.NotEmpty(t, compiled.Cases)

	for index := range compiled.Cases {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			t.Parallel()

			caseCompiler := NewCompiler(parseSchemaSource(t, schema, "", "create"))
			caseSuite, caseErr := caseCompiler.CompileSuite()
			require.NoError(t, caseErr)

			plannedCase := caseSuite.Cases[index]
			domain := mustDomain(t, caseSuite.Domains, plannedCase.Values)
			domain.String.Patterns = nil
			domain.String.Formats = nil

			rapid.Check(t, func(rt *rapid.T) {
				value := plannedCase.Generator.Draw(rt, "value")
				matches, fitErr := caseCompiler.valueFitsDomain(value, domain)
				require.NoError(rt, fitErr)
				require.True(rt, matches)

				body, marshalErr := value.MarshalJSON()
				require.NoError(rt, marshalErr)
				require.True(rt, json.Valid(body))
			})
		})
	}
}

// TestNumberGeneratorIncludesExactFractionalValues verifies bounded number coverage beyond integers.
func TestNumberGeneratorIncludesExactFractionalValues(t *testing.T) {
	t.Parallel()

	minimum, err := jsonvalue.ParseNumber("0")
	require.NoError(t, err)
	maximum, err := jsonvalue.ParseNumber("2")
	require.NoError(t, err)

	values, err := boundedNumberCandidates(NumberConstraints{
		State:   KindRestricted,
		Minimum: &NumberBound{Value: minimum},
		Maximum: &NumberBound{Value: maximum},
	})
	require.NoError(t, err)

	fractional := false

	for _, value := range values {
		if !value.Number.Rational.IsInt() {
			fractional = true
		}
	}

	require.True(t, fractional)
}

// TestAdditionalPropertyNamesNeverCollide verifies ordinal names skip every declared property once.
func TestAdditionalPropertyNamesNeverCollide(t *testing.T) {
	t.Parallel()

	properties := []NamedProperty{{Name: "additional0"}, {Name: "additional2"}}
	require.Equal(t, "additional1", additionalPropertyName(properties, 0))
	require.Equal(t, "additional3", additionalPropertyName(properties, 1))
	require.Equal(t, "additional4", additionalPropertyName(properties, 2))
}

// TestStringLanguageKeysAreUnambiguous verifies trusted example caches cannot cross language sets.
func TestStringLanguageKeysAreUnambiguous(t *testing.T) {
	t.Parallel()

	one := stringLanguageKey(StringConstraints{Patterns: []string{"a\x00b"}})
	two := stringLanguageKey(StringConstraints{Patterns: []string{"a", "b"}})
	require.NotEqual(t, one, two)
}

// TestCompileSuiteRejectsEmptyRoot verifies the public checker cannot silently execute zero cases.
func TestCompileSuiteRejectsEmptyRoot(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: string
minLength: 2
maxLength: 1`, "", "create"))
	_, err := compiler.CompileSuite()
	require.ErrorContains(t, err, "accepts no JSON value")
}

// TestCompileSuiteRejectsMissingExamplesForAnyReachableStringKind verifies mixed schemas do not omit a partition.
func TestCompileSuiteRejectsMissingExamplesForAnyReachableStringKind(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `pattern: '^ok$'`, "", "create"))
	_, err := compiler.CompileSuite()
	require.ErrorContains(t, err, "no trusted valid example")
}

// TestCompileSuiteRequiresTrustedPatternAndFormatExamples verifies constrained strings need trusted examples.
func TestCompileSuiteRequiresTrustedPatternAndFormatExamples(t *testing.T) {
	t.Parallel()

	for _, schema := range []string{
		`type: string
pattern: '^ok$'`,
		`type: string
format: email`,
	} {
		compiler := NewCompiler(parseSchemaSource(t, schema, "", "create"))
		_, err := compiler.CompileSuite()
		require.ErrorContains(t, err, "no trusted valid example")
	}
}
