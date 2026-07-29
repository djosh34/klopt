//nolint:godoclint // Behavior tests use descriptive test names as their specification.
package suite

import (
	"strings"
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

func TestCompileSuiteRejectsExcessiveConjunctiveAnyOfChoices(t *testing.T) {
	t.Parallel()

	var groups strings.Builder
	for range 9 {
		_, err := groups.WriteString("  - anyOf: [{minLength: 0}, {maxLength: 10}]\n")
		require.NoError(t, err)
	}

	schema := "type: string\nallOf:\n" + groups.String()
	_, err := NewCompiler(parseSchemaSource(t, schema, "", "create")).CompileSuite()
	require.ErrorContains(t, err, "at most 256 conjunctive anyOf generation profiles")
}

func TestCompileSuitePreservesAnyOfNestedUnderAllOfReference(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `$ref: '#/components/schemas/Choice'`, `
components:
  schemas:
    Choice:
      allOf:
        - anyOf:
            - {type: string}
            - {type: integer}
`, "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	valid := anyOfCase(t, compiled.Cases, ExpectAccepted)
	invalid := anyOfCase(t, compiled.Cases, ExpectRejected)

	for seed := 0; seed < 100; seed++ {
		accepted := valid.Generator.Example(seed)
		require.True(t, accepted.Kind == jsonvalue.KindString ||
			accepted.Kind == jsonvalue.KindNumber && accepted.Number.IsInteger())

		rejected := invalid.Generator.Example(seed)
		require.False(t, rejected.Kind == jsonvalue.KindString ||
			rejected.Kind == jsonvalue.KindNumber && rejected.Number.IsInteger())
	}
}

func TestCompileSuiteKeepsIndependentAnyOfGroupFailuresDisjunctive(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `
type: number
allOf:
  - anyOf: [{minimum: 0}]
  - anyOf: [{maximum: 0}]
`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	invalid := anyOfCase(t, compiled.Cases, ExpectRejected)
	for seed := 0; seed < 100; seed++ {
		value := invalid.Generator.Example(seed)
		require.Equal(t, jsonvalue.KindNumber, value.Kind)
		require.NotZero(t, value.Number.Rational.Sign())
	}
}

func TestCompileSuiteSkipsFocusedFailuresInsideNestedAnyOfBranches(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `
type: object
required: [value]
properties:
  value:
    type: string
    anyOf:
      - {pattern: '^a'}
      - {}
`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	propertyUse := compiler.rootUse.property("value")
	require.NotNil(t, propertyUse)
	require.NotEmpty(t, propertyUse.anyOf)
	require.True(t, sourceIsInsideAnyOfBranch(
		ConstraintSource{Pointer: propertyUse.anyOf[0].pointer, Keyword: "pattern"},
		compiler.rootUse,
	))

	for _, plannedCase := range compiled.Cases {
		require.False(t, plannedCase.Expect == ExpectRejected && plannedCase.Source.Keyword == "pattern")
	}
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

func TestCompileSuiteUsesOneFullValidPropertyForAnyOfWithSiblingBoundaries(t *testing.T) {
	t.Parallel()

	compiled, err := NewCompiler(parseSchemaSource(t, `
type: string
minLength: 2
maxLength: 5
anyOf:
  - {pattern: '^a'}
  - {pattern: 'z$'}
`, "", "create")).CompileSuite()
	require.NoError(t, err)

	accepted := make([]CasePlan, 0)

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect == ExpectAccepted {
			accepted = append(accepted, plannedCase)
		}
	}

	require.Len(t, accepted, 1)
	require.Equal(t, "anyOf", accepted[0].Source.Keyword)

	for seed := range 100 {
		value := accepted[0].Generator.Example(seed)
		require.Equal(t, jsonvalue.KindString, value.Kind)
		require.GreaterOrEqual(t, len([]rune(value.String)), 2)
		require.LessOrEqual(t, len([]rune(value.String)), 5)
		require.True(t, strings.HasPrefix(value.String, "a") || strings.HasSuffix(value.String, "z"))
	}
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

func TestCompileSuiteAnyOfConstructsNumericFormatComplements(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `
type: number
anyOf:
  - {format: float}
`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	invalid := anyOfCase(t, compiled.Cases, ExpectRejected)
	format := numericFormatConstraintsForTest(t, "float")
	distinct := make(map[string]struct{})

	for seed := 0; seed < 100; seed++ {
		value := invalid.Generator.Example(seed)
		require.Equal(t, jsonvalue.KindNumber, value.Kind)
		matches, fitErr := numberFits(value.Number, format)
		require.NoError(t, fitErr)
		require.False(t, matches)

		distinct[value.Number.Lexeme] = struct{}{}
	}

	require.Greater(t, len(distinct), 2)
}

func TestCompileSuiteAnyOfExcludesObjectEnumValuesConstructively(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `
type: object
required: [value]
additionalProperties: false
properties:
  value: {type: string}
anyOf:
  - {enum: [{value: blocked}]}
`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	invalid := anyOfCase(t, compiled.Cases, ExpectRejected)
	distinct := make(map[string]struct{})

	for seed := 0; seed < 100; seed++ {
		value := invalid.Generator.Example(seed)
		require.Equal(t, jsonvalue.KindObject, value.Kind)
		require.Len(t, value.Object, 1)
		require.Equal(t, "value", value.Object[0].Name)
		require.NotEqual(t, "blocked", value.Object[0].Value.String)
		distinct[value.Object[0].Value.String] = struct{}{}
	}

	require.Greater(t, len(distinct), 2)
}

func TestCompileSuiteAnyOfObjectEnumComplementChangesPropertyPresence(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		schema   string
		excluded string
		check    func(*testing.T, jsonvalue.Value)
	}{
		{
			name: "open empty object",
			schema: `type: object
anyOf: [{enum: [{}]}]`,
			excluded: `{}`,
			check: func(t *testing.T, value jsonvalue.Value) {
				t.Helper()
				require.NotEmpty(t, value.Object)
			},
		},
		{
			name: "closed optional property",
			schema: `type: object
additionalProperties: false
properties: {a: {type: string}}
anyOf: [{enum: [{a: x}]}]`,
			excluded: `{"a":"x"}`,
			check: func(t *testing.T, value jsonvalue.Value) {
				t.Helper()
				require.Empty(t, value.Object)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			compiled, err := NewCompiler(parseSchemaSource(t, test.schema, "", "create")).CompileSuite()
			require.NoError(t, err)
			invalid := anyOfCase(t, compiled.Cases, ExpectRejected)
			excluded := mustJSONValue(t, test.excluded)

			for seed := range 20 {
				value := invalid.Generator.Example(seed)
				require.Equal(t, jsonvalue.KindObject, value.Kind)
				require.False(t, value.Equal(excluded))
				test.check(t, value)
			}
		})
	}
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

func TestCompileSuiteOuterPatternFailureReachesExpressionBackedObjectChildren(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
		child  func(jsonvalue.Value) jsonvalue.Value
	}{
		{
			name: "declared property",
			schema: `type: object
required: [value]
maxProperties: 1
additionalProperties: false
properties:
  value:
    type: string
    pattern: '^x$'
    anyOf: [{}]`,
			child: func(value jsonvalue.Value) jsonvalue.Value {
				return value.Object[0].Value
			},
		},
		{
			name: "additional property",
			schema: `type: object
minProperties: 1
maxProperties: 1
additionalProperties:
  type: string
  pattern: '^x$'
  anyOf: [{}]`,
			child: func(value jsonvalue.Value) jsonvalue.Value {
				return value.Object[0].Value
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			compiled, err := NewCompiler(parseSchemaSource(t, test.schema, "", "create")).CompileSuite()
			require.NoError(t, err)

			var patternFailure *CasePlan

			for index := range compiled.Cases {
				if compiled.Cases[index].Expect == ExpectRejected &&
					compiled.Cases[index].Source.Keyword == "pattern" {
					patternFailure = &compiled.Cases[index]

					break
				}
			}

			require.NotNil(t, patternFailure)

			for seed := range 20 {
				value := patternFailure.Generator.Example(seed)
				require.Equal(t, jsonvalue.KindObject, value.Kind)
				require.Len(t, value.Object, 1)

				child := test.child(value)
				require.Equal(t, jsonvalue.KindString, child.Kind)
				require.NotEqual(t, "x", child.String)
			}
		})
	}
}

func TestCompileSuiteDistinguishesEmptyAndUnsupportedAnyOfComplements(t *testing.T) {
	t.Parallel()

	emptyCompiler := NewCompiler(parseSchemaSource(t, `anyOf: [{}, {type: string}]`, "", "create"))
	empty, err := emptyCompiler.CompileSuite()
	require.NoError(t, err)

	for _, plannedCase := range empty.Cases {
		require.NotEqual(t, ExpectRejected, plannedCase.Expect)
	}

	arrayEnumCompiler := NewCompiler(parseSchemaSource(t, `
anyOf:
  - type: array
    items: {type: boolean}
    maxItems: 1
    enum: [[], [false], [true]]
`, "", "create"))
	arrayEnum, err := arrayEnumCompiler.CompileSuite()
	require.NoError(t, err)

	invalid := anyOfCase(t, arrayEnum.Cases, ExpectRejected)
	rapid.Check(t, func(rt *rapid.T) {
		value := invalid.Generator.Draw(rt, "invalid")
		for _, raw := range []string{`[]`, `[false]`, `[true]`} {
			require.False(rt, value.Equal(mustJSONValue(t, raw)))
		}
	})
}

func TestCompileSuiteAnyOfEnumComplementsDoNotCollapseToDifferentWitnesses(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `anyOf: [{enum: [null]}, {enum: [false]}]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	invalid := anyOfCase(t, compiled.Cases, ExpectRejected)
	seen := make(map[string]struct{})

	for seed := range 100 {
		value := invalid.Generator.Example(seed)
		encoded, marshalErr := value.MarshalJSON()
		require.NoError(t, marshalErr)

		seen[string(encoded)] = struct{}{}
	}

	require.Greater(t, len(seen), 2)

	rapid.Check(t, func(rt *rapid.T) {
		value := invalid.Generator.Draw(rt, "invalid")
		require.NotEqual(rt, jsonvalue.KindNull, value.Kind)
		require.False(rt, value.Kind == jsonvalue.KindBoolean && !value.Boolean)
	})
}

func TestCompileSuiteProvesExactContainerEnumComplementsEmpty(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
	}{
		{
			name: "array",
			schema: `type: array
items: {type: boolean}
maxItems: 1
anyOf: [{enum: [[], [false], [true]]}]`,
		},
		{
			name: "object",
			schema: `type: object
required: [flag]
additionalProperties: false
properties: {flag: {type: boolean}}
anyOf: [{enum: [{flag: false}, {flag: true}]}]`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			compiler := NewCompiler(parseSchemaSource(t, test.schema, "", "create"))
			compiled, err := compiler.CompileSuite()
			require.NoError(t, err)

			for _, plannedCase := range compiled.Cases {
				require.False(t, plannedCase.Source.Keyword == "anyOf" && plannedCase.Expect == ExpectRejected)
			}
		})
	}
}

func TestCompileSuiteExactComplementIncludesValuesFailingMultipleSiblingRules(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `anyOf: [
  {type: number, minimum: 10, maximum: 0},
  {type: boolean}
]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)
	invalid := anyOfCase(t, compiled.Cases, ExpectRejected)

	foundMiddle := false
	zero, err := jsonvalue.ParseNumber("0")
	require.NoError(t, err)
	ten, err := jsonvalue.ParseNumber("10")
	require.NoError(t, err)

	for seed := range 200 {
		value := invalid.Generator.Example(seed)
		if value.Kind == jsonvalue.KindNumber && value.Number.Compare(zero) > 0 && value.Number.Compare(ten) < 0 {
			foundMiddle = true

			break
		}
	}

	require.True(t, foundMiddle)
}

func anyOfCase(t *testing.T, cases []CasePlan, expect ExpectedResult) CasePlan {
	t.Helper()

	for _, plannedCase := range cases {
		if plannedCase.Source.Keyword == "anyOf" && plannedCase.Expect == expect {
			return plannedCase
		}
	}

	require.FailNow(t, "missing anyOf case")

	return CasePlan{}
}

func TestCompileSuiteKeepsFocusedFailureForPropertyNamedAnyOf(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `
type: object
required: [anyOf]
properties:
  anyOf: {type: string, allOf: [{minLength: 2}]}
anyOf:
  - {type: object}
`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	found := false

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect == ExpectRejected && plannedCase.Source.Keyword == "minLength" &&
			strings.Contains(plannedCase.Source.Pointer, "/properties/anyOf") {
			found = true
		}
	}

	require.True(t, found)
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

	for seed := range 1_000 {
		value := generator.Example(seed)
		if value.Kind == jsonvalue.KindBoolean {
			singleDraws++
		} else {
			nestedDraws++
		}
	}

	require.InDelta(t, 500, nestedDraws, 100)
	require.InDelta(t, 500, singleDraws, 100)
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
