//nolint:godoclint // Behavior tests use descriptive test names as their specification.
package suite

import (
	"testing"

	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestCompilerPreservesOrderedAnyOfOccurrences(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `
type: string
minLength: 1
anyOf:
  - pattern: '^a'
  - pattern: 'z$'
`, "", "create"))
	_, err := compiler.Compile()
	require.NoError(t, err)
	require.Len(t, compiler.rootUse.anyOf, 2)
	require.Contains(t, compiler.rootUse.anyOf[0].pointer, "/anyOf/0")
	require.Contains(t, compiler.rootUse.anyOf[1].pointer, "/anyOf/1")
}

func TestCompileSuiteBuildsOneFullValidAndExactInvalidAnyOfCase(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `
anyOf:
  - {type: string, minLength: 2}
  - {type: integer, minimum: 5}
`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	var validCases, invalidCases []CasePlan

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Source.Keyword != "anyOf" {
			continue
		}

		if plannedCase.Expect == ExpectAccepted {
			validCases = append(validCases, plannedCase)
		} else {
			invalidCases = append(invalidCases, plannedCase)
		}
	}

	require.Len(t, validCases, 1)
	require.Len(t, invalidCases, 1)

	rapid.Check(t, func(rt *rapid.T) {
		valid := validCases[0].Generator.Draw(rt, "valid")
		require.True(rt, matchesStringOrLargeInteger(valid))

		invalid := invalidCases[0].Generator.Draw(rt, "invalid")
		require.False(rt, matchesStringOrLargeInteger(invalid))
	})
}

func matchesStringOrLargeInteger(value jsonvalue.Value) bool {
	if value.Kind == jsonvalue.KindString {
		return len([]rune(value.String)) >= 2
	}

	if value.Kind != jsonvalue.KindNumber || !value.Number.IsInteger() {
		return false
	}

	minimum, err := jsonvalue.ParseNumber("5")
	if err != nil {
		panic(err)
	}

	return value.Number.Compare(minimum) >= 0
}

func TestCompileSuiteCarriesAnyOfExpressionsThroughContainers(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
		child  func(jsonvalue.Value) (jsonvalue.Value, bool)
	}{
		{
			name: "array items",
			schema: `type: array
minItems: 1
maxItems: 3
items:
  anyOf: [{type: string, minLength: 2}, {type: integer, minimum: 5}]`,
			child: func(value jsonvalue.Value) (jsonvalue.Value, bool) {
				if len(value.Array) == 0 {
					return jsonvalue.Value{}, false
				}

				return value.Array[0], true
			},
		},
		{
			name: "declared property",
			schema: `type: object
required: [value]
additionalProperties: false
properties:
  value:
    anyOf: [{type: string, minLength: 2}, {type: integer, minimum: 5}]`,
			child: func(value jsonvalue.Value) (jsonvalue.Value, bool) {
				for _, member := range value.Object {
					if member.Name == "value" {
						return member.Value, true
					}
				}

				return jsonvalue.Value{}, false
			},
		},
		{
			name: "additional property",
			schema: `type: object
minProperties: 1
additionalProperties:
  anyOf: [{type: string, minLength: 2}, {type: integer, minimum: 5}]`,
			child: func(value jsonvalue.Value) (jsonvalue.Value, bool) {
				if len(value.Object) == 0 {
					return jsonvalue.Value{}, false
				}

				return value.Object[0].Value, true
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			compiler := NewCompiler(parseSchemaSource(t, test.schema, "", "create"))
			compiled, err := compiler.CompileSuite()
			require.NoError(t, err)

			compositionCases := 0

			for _, plannedCase := range compiled.Cases {
				if plannedCase.Source.Keyword != "anyOf" {
					continue
				}

				compositionCases++

				rapid.Check(t, func(rt *rapid.T) {
					value := plannedCase.Generator.Draw(rt, "value")
					child, ok := test.child(value)
					require.True(rt, ok)

					if plannedCase.Expect == ExpectAccepted {
						require.True(rt, matchesStringOrLargeInteger(child))
					} else {
						require.False(rt, matchesStringOrLargeInteger(child))
					}
				})
			}

			require.Equal(t, 2, compositionCases)
		})
	}
}

func TestCompileSuiteAnyOfUsesSignedPatternComplements(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `
type: string
anyOf:
  - {pattern: '^a'}
  - {pattern: 'z$'}
	`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	compositionCases := 0

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Source.Keyword != "anyOf" {
			continue
		}

		compositionCases++

		rapid.Check(t, func(rt *rapid.T) {
			value := plannedCase.Generator.Draw(rt, "value")

			matches := len(value.String) > 0 && (value.String[0] == 'a' || value.String[len(value.String)-1] == 'z')
			if plannedCase.Expect == ExpectAccepted {
				require.True(rt, matches)
			} else {
				require.False(rt, matches)
			}
		})
	}

	require.Equal(t, 2, compositionCases)
}

func TestCompileSuiteOuterFocusedFailureStillSatisfiesAnyOf(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `
type: integer
minimum: 10
anyOf:
  - {multipleOf: 2}
  - {multipleOf: 3}
`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	var minimumFailure *CasePlan

	for index := range compiled.Cases {
		plannedCase := &compiled.Cases[index]
		if plannedCase.Expect == ExpectRejected && plannedCase.Source.Keyword == "minimum" {
			minimumFailure = plannedCase

			break
		}
	}

	require.NotNil(t, minimumFailure)

	minimum, err := jsonvalue.ParseNumber("10")
	require.NoError(t, err)
	rapid.Check(t, func(rt *rapid.T) {
		value := minimumFailure.Generator.Draw(rt, "value")
		require.Equal(rt, jsonvalue.KindNumber, value.Kind)
		require.Less(rt, value.Number.Compare(minimum), 0)

		two, parseErr := jsonvalue.ParseNumber("2")
		require.NoError(rt, parseErr)
		three, parseErr := jsonvalue.ParseNumber("3")
		require.NoError(rt, parseErr)
		require.True(rt, value.Number.IsMultipleOf(two) || value.Number.IsMultipleOf(three))
	})
}

func TestCompileSuiteDistinguishesEmptyAndUnsupportedAnyOfComplements(t *testing.T) {
	t.Parallel()

	emptyCompiler := NewCompiler(parseSchemaSource(t, `anyOf: [{}, {type: string}]`, "", "create"))
	empty, err := emptyCompiler.CompileSuite()
	require.NoError(t, err)

	for _, plannedCase := range empty.Cases {
		require.NotEqual(t, ExpectRejected, plannedCase.Expect)
	}

	unsupportedCompiler := NewCompiler(parseSchemaSource(t, `
anyOf:
  - type: array
    items: {type: boolean}
    maxItems: 1
    enum: [[], [false], [true]]
`, "", "create"))
	_, err = unsupportedCompiler.CompileSuite()
	require.ErrorContains(t, err, "cannot construct exact complement")
}

func TestCompileSuiteAnyOfEnumComplementRejectsEveryBranch(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `anyOf: [{enum: [1, 2]}, {enum: [2, 3]}]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect != ExpectRejected {
			continue
		}

		rapid.Check(t, func(rt *rapid.T) {
			value := plannedCase.Generator.Draw(rt, "value")
			for _, raw := range []string{"1", "2", "3"} {
				require.False(rt, value.Equal(mustJSONValue(t, raw)))
			}
		})
	}
}

func TestCompileSuiteSupportsNestedAnyOfComplements(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `
anyOf:
  - anyOf:
      - {type: string}
      - {type: integer}
  - {type: boolean}
`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	for _, plannedCase := range compiled.Cases {
		rapid.Check(t, func(rt *rapid.T) {
			value := plannedCase.Generator.Draw(rt, "value")

			accepted := value.Kind == jsonvalue.KindString || value.Kind == jsonvalue.KindBoolean ||
				value.Kind == jsonvalue.KindNumber && value.Number.IsInteger()
			if plannedCase.Expect == ExpectAccepted {
				require.True(rt, accepted)
			} else {
				require.False(rt, accepted)
			}
		})
	}
}

func TestGenerationChoiceKeepsImmediateBranchesUniform(t *testing.T) {
	t.Parallel()

	registry := NewDomainRegistry()
	use := &schemaUse{domains: registry, pointer: "#/choice"}
	term := func(raw string) generationExpression {
		id := registry.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{mustJSONValue(t, raw)}))

		return generationExpression{term: &generationTerm{domain: id, use: use}}
	}
	nested := choose(term(`"a"`), term(`"b"`), term(`"c"`))
	expression := choose(nested, term(`true`))
	require.Len(t, expression.choice.branches, 2)
	require.Len(t, expression.choice.branches[0].choice.branches, 3)

	builder := NewRapidGeneratorBuilder(registry)
	generator, err := builder.expression(expression)
	require.NoError(t, err)

	var nestedDraws, singleDraws int

	rapid.Check(t, func(rt *rapid.T) {
		value := generator.Draw(rt, "value")
		if value.Kind == jsonvalue.KindBoolean {
			singleDraws++
		} else {
			nestedDraws++
		}
	})
	require.Greater(t, nestedDraws, 25)
	require.Greater(t, singleDraws, 25)
}

func TestCompilerRejectsMalformedAnyOfAtExactPointer(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
		want   string
	}{
		{name: "null", schema: `anyOf: null`, want: "/anyOf"},
		{name: "object", schema: `anyOf: {}`, want: "/anyOf"},
		{name: "empty", schema: `anyOf: []`, want: "/anyOf"},
		{name: "scalar branch", schema: `anyOf: [1]`, want: "/anyOf/0"},
		{name: "unsupported branch", schema: `anyOf: [{not: {}}]`, want: "/anyOf/0/not"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			compiler := NewCompiler(parseSchemaSource(t, test.schema, "", "create"))
			_, err := compiler.Compile()
			require.ErrorContains(t, err, test.want)
		})
	}
}

func TestCompilerStillRejectsOneOfAndNot(t *testing.T) {
	t.Parallel()

	for _, schema := range []string{`oneOf: [{}]`, `not: {}`} {
		compiler := NewCompiler(parseSchemaSource(t, schema, "", "create"))
		_, err := compiler.Compile()
		require.ErrorContains(t, err, "unsupported")
	}
}
