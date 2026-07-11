//nolint:godoclint,paralleltest,wsl_v5 // Generated CasePlans share one immutable compiled suite.
package suite

import (
	"encoding/json"
	"strconv"
	"testing"

	//nolint:depguard // Generator tests inspect exact private JSON numbers.
	"decode_and_validate_generator/pkg/test_generator/internal/jsonvalue"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestRapidGeneratorBuilderMemoizesDomains(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: array
minItems: 2
maxItems: 4
items: {type: boolean}`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	builder := NewRapidGeneratorBuilder(compiled.Domains, compiled.SchemaUses)
	first, err := builder.Generator(compiled.Root)
	require.NoError(t, err)
	second, err := builder.Generator(compiled.Root)
	require.NoError(t, err)
	require.Same(t, first, second)
}

func TestCompileSuiteGeneratorsConstructEveryPlannedDomain(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: object
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
  type: integer`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)
	require.NotEmpty(t, compiled.Cases)

	for index, plannedCase := range compiled.Cases {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			domain := mustDomain(t, compiled.Domains, plannedCase.Values)
			domain.String.Patterns = nil
			domain.String.Formats = nil
			rapid.Check(t, func(rt *rapid.T) {
				value := plannedCase.Generator.Draw(rt, "value")
				matches, fitErr := compiler.valueFitsDomain(value, domain)
				require.NoError(rt, fitErr)
				require.True(rt, matches)

				body, marshalErr := value.MarshalJSON()
				require.NoError(rt, marshalErr)
				require.True(rt, json.Valid(body))
			})
		})
	}
}

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
