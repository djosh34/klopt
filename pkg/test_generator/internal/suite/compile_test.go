package suite

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"decode_and_validate_generator/pkg/internal/jsonvalue"
	"decode_and_validate_generator/pkg/internal/oas"
	"github.com/stretchr/testify/require"
)

// TestCompileSuiteKeepsOracleCasesOccurrenceScoped verifies that trusted generation cases
// compose only at their exact Schema Object occurrence, including recursive and referenced uses.
func TestCompileSuiteKeepsOracleCasesOccurrenceScoped(t *testing.T) {
	t.Parallel()

	t.Run("same occurrence enum certifies opaque and modeled siblings without validation", func(t *testing.T) {
		t.Parallel()

		compiler := NewCompiler(parseSchemaSource(t, `type: string
pattern: '^never$'
format: email
enum: [trusted]`, "", "create"))
		compiled, err := compiler.CompileSuite(MustHaveAllXValidCases)
		require.NoError(t, err)

		checkAcceptedCases(t, compiled, func(t require.TestingT, body []byte) {
			require.JSONEq(t, `"trusted"`, string(body))
		})
	})

	t.Run("local enum and valid examples are equivalent case sources", func(t *testing.T) {
		t.Parallel()

		compiler := NewCompiler(parseSchemaSource(t, `allOf:
  - type: string
    pattern: '^never$'
    enum: [from-enum, from-example]
    x-valid-examples: [from-example]
  - type: string
    format: email
    x-valid-examples: [from-example]`, "", "create"))
		compiled, err := compiler.CompileSuite(MustHaveAllXValidCases)
		require.NoError(t, err)

		checkAcceptedCases(t, compiled, func(t require.TestingT, body []byte) {
			require.JSONEq(t, `"from-example"`, string(body))
		})
	})

	t.Run("separate opaque branches require their own shared evidence", func(t *testing.T) {
		t.Parallel()

		for _, schema := range []string{
			`allOf:
  - pattern: '^A'
    x-valid-examples: [A1]
  - format: email`,
			`allOf:
  - enum: [A1]
  - pattern: '^A'`,
			`allOf:
  - pattern: '^A'
    x-valid-examples: [A1]
  - format: email
    x-valid-examples: [other]`,
		} {
			_, err := NewCompiler(parseSchemaSource(t, schema, "", "create")).CompileSuite(MustHaveAllXValidCases)
			require.ErrorContains(t, err, "unconstructible")
		}
	})

	t.Run("three branch intersection is semantic and order independent", func(t *testing.T) {
		t.Parallel()

		branches := []string{
			`enum: [1, [x], {a: 1}, gone]`,
			`enum: [1.0, [x], {a: 1.0}, other]`,
			`enum: [1.00, [x], {a: 1}, last]`,
		}
		orders := [][]int{{0, 1, 2}, {2, 0, 1}, {1, 2, 0}}

		for _, order := range orders {
			schema := "allOf:\n"
			for _, index := range order {
				schema += "  - " + strings.ReplaceAll(branches[index], "\n", "\n    ") + "\n"
			}

			compiler := NewCompiler(parseSchemaSource(t, schema, "", "create"))
			compiled, err := compiler.CompileSuite(MustHaveAllXValidCases)
			require.NoError(t, err)

			domain := mustDomain(t, compiled.Domains, compiled.Root)
			require.Len(t, domain.Enum.Values, 3)
			require.True(t, jsonValuesContain(domain.Enum.Values, mustJSONValue(t, `1`)))
			require.True(t, jsonValuesContain(domain.Enum.Values, mustJSONValue(t, `["x"]`)))
			require.True(t, jsonValuesContain(domain.Enum.Values, mustJSONValue(t, `{"a":1}`)))
		}
	})

	t.Run("nested referenced item evidence reaches the exact merged occurrence", func(t *testing.T) {
		t.Parallel()

		compiler := NewCompiler(parseSchemaSource(t, `type: array
minItems: 1
maxItems: 1
items:
  allOf:
    - {$ref: '#/components/schemas/StartsA'}
    - {$ref: '#/components/schemas/EndsOne'}`, `
components:
  schemas:
    StartsA:
      type: string
      pattern: '^A'
      x-valid-examples: [A1]
    EndsOne:
      type: string
      format: email
      x-valid-examples: [A1]
`, "create"))
		compiled, err := compiler.CompileSuite(MustHaveAllXValidCases)
		require.NoError(t, err)

		checkAcceptedCases(t, compiled, func(t require.TestingT, body []byte) {
			var values []string
			require.NoError(t, json.Unmarshal(body, &values))
			require.Equal(t, []string{"A1"}, values)
		})
	})

	t.Run("array branch item evidence survives structural meet", func(t *testing.T) {
		t.Parallel()

		compiler := NewCompiler(parseSchemaSource(t, `minItems: 1
maxItems: 1
allOf:
  - type: array
    items:
      type: string
      pattern: '^A'
      x-valid-examples: [A1]
  - type: array
    items:
      type: string
      format: email
      x-valid-examples: [A1]`, "", "create"))
		compiled, err := compiler.CompileSuite(MustHaveAllXValidCases)
		require.NoError(t, err)

		checkAcceptedCases(t, compiled, func(t require.TestingT, body []byte) {
			require.JSONEq(t, `["A1"]`, string(body))
		})
	})

	t.Run("chained reference evidence stays on the resolved occurrence", func(t *testing.T) {
		t.Parallel()

		compiler := NewCompiler(parseSchemaSource(t, `type: array
minItems: 1
maxItems: 1
items: {$ref: '#/components/schemas/First'}`, `
components:
  schemas:
    First: {$ref: '#/components/schemas/Second'}
    Second:
      type: string
      pattern: '^trusted$'
      x-valid-examples: [not-trusted]
`, "create"))
		compiled, err := compiler.CompileSuite(MustHaveAllXValidCases)
		require.NoError(t, err)

		checkAcceptedCases(t, compiled, func(t require.TestingT, body []byte) {
			require.JSONEq(t, `["not-trusted"]`, string(body))
		})
	})

	t.Run("additional property conjunction uses its own shared evidence", func(t *testing.T) {
		t.Parallel()

		compiler := NewCompiler(parseSchemaSource(t, `type: object
minProperties: 1
maxProperties: 1
additionalProperties:
  allOf:
    - type: string
      pattern: '^A'
      x-valid-examples: [A1]
    - type: string
      format: email
      x-valid-examples: [A1]`, "", "create"))
		compiled, err := compiler.CompileSuite(MustHaveAllXValidCases)
		require.NoError(t, err)

		checkAcceptedCases(t, compiled, func(t require.TestingT, body []byte) {
			var object map[string]string
			require.NoError(t, json.Unmarshal(body, &object))
			require.Len(t, object, 1)

			for _, value := range object {
				require.Equal(t, "A1", value)
			}
		})
	})

	t.Run("undeclared required property uses additional policy evidence", func(t *testing.T) {
		t.Parallel()

		compiler := NewCompiler(parseSchemaSource(t, `type: object
required: [value]
maxProperties: 1
additionalProperties:
  type: string
  pattern: '^trusted$'
  x-valid-examples: [not-trusted]`, "", "create"))
		compiled, err := compiler.CompileSuite(MustHaveAllXValidCases)
		require.NoError(t, err)

		checkAcceptedCases(t, compiled, func(t require.TestingT, body []byte) {
			require.JSONEq(t, `{"value":"not-trusted"}`, string(body))
		})
	})

	t.Run("object branch child evidence survives structural meet", func(t *testing.T) {
		t.Parallel()

		schemas := []string{
			`minProperties: 1
maxProperties: 1
allOf:
  - type: object
    required: [value]
    properties:
      value: {type: string, pattern: '^A', x-valid-examples: [A1]}
  - type: object
    required: [value]
    properties:
      value: {type: string, format: email, x-valid-examples: [A1]}`,
			`minProperties: 1
maxProperties: 1
allOf:
  - type: object
    additionalProperties: {type: string, pattern: '^A', x-valid-examples: [A1]}
  - type: object
    additionalProperties: {type: string, format: email, x-valid-examples: [A1]}`,
		}

		for _, schema := range schemas {
			compiler := NewCompiler(parseSchemaSource(t, schema, "", "create"))
			compiled, err := compiler.CompileSuite(MustHaveAllXValidCases)
			require.NoError(t, err)

			checkAcceptedCases(t, compiled, func(t require.TestingT, body []byte) {
				var object map[string]string
				require.NoError(t, json.Unmarshal(body, &object))
				require.Len(t, object, 1)

				for _, value := range object {
					require.Equal(t, "A1", value)
				}
			})
		}
	})

	t.Run("unrelated equal languages cannot borrow cases", func(t *testing.T) {
		t.Parallel()

		_, err := NewCompiler(parseSchemaSource(t, `type: object
required: [first, second]
properties:
  first:
    pattern: '^same$'
    x-valid-examples: [first]
  second:
    pattern: '^same$'`, "", "create")).CompileSuite(MustHaveAllXValidCases)
		require.ErrorContains(t, err, "unconstructible")
	})
}

// TestCompileSuiteMeetsTrustedStringsAtOccurrenceBoundaries verifies local oracle
// truth is unchecked while a separate modeled occurrence still constrains it.
func TestCompileSuiteMeetsTrustedStringsAtOccurrenceBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("local modeled sibling does not filter", func(t *testing.T) {
		t.Parallel()

		compiler := NewCompiler(parseSchemaSource(t, `type: string
minLength: 10
pattern: '^never$'
x-valid-examples: [short]`, "", "create"))
		compiled, err := compiler.CompileSuite(MustHaveAllXValidCases)
		require.NoError(t, err)

		checkAcceptedCases(t, compiled, func(t require.TestingT, body []byte) {
			require.JSONEq(t, `"short"`, string(body))
		})
	})

	t.Run("separate modeled sibling filters", func(t *testing.T) {
		t.Parallel()

		const schema = `allOf:
  - type: string
    pattern: '^never$'
    x-valid-examples: [short]
  - minLength: 10`

		compiler := NewCompiler(parseSchemaSource(t, schema, "", "create"))
		_, err := compiler.Compile()
		require.NoError(t, err)
		require.True(t, compiler.rootUse.examples.ValidDeclared)
		require.Empty(t, compiler.rootUse.examples.Valid)

		_, err = NewCompiler(parseSchemaSource(t, schema, "", "create")).
			CompileSuite(MustHaveAllXValidCases)
		require.Error(t, err)

		var compileError *Error
		require.ErrorAs(t, err, &compileError)
		require.Equal(t, "compile", compileError.Phase)
		require.Equal(t, "unconstructible", compileError.Code)
		require.Equal(t, "allOf", compileError.Keyword)
	})
}

// TestCompileSuiteEnforcesOracleOptionAfterCompilerReuse verifies cached occurrence
// graphs do not bypass recursive option checks.
func TestCompileSuiteEnforcesOracleOptionAfterCompilerReuse(t *testing.T) {
	t.Parallel()

	source := parseSchemaSource(t, `type: object
properties:
  optional: {$ref: '#/components/schemas/Disjoint'}`, `
components:
  schemas:
    Disjoint:
      allOf:
        - type: string
          pattern: '^A$'
          x-valid-examples: [A]
        - type: string
          format: email
          x-valid-examples: [B]
`, "create")
	compiler := NewCompiler(source)

	_, err := compiler.Compile()
	require.NoError(t, err)

	_, err = compiler.CompileSuite(MustHaveAllXValidCases)
	require.Error(t, err)

	var cachedError *Error
	require.ErrorAs(t, err, &cachedError)

	_, err = NewCompiler(source).CompileSuite(MustHaveAllXValidCases)
	require.Error(t, err)

	var freshError *Error
	require.ErrorAs(t, err, &freshError)
	require.Equal(t, freshError.Phase, cachedError.Phase)
	require.Equal(t, freshError.Code, cachedError.Code)
	require.Equal(t, freshError.Keyword, cachedError.Keyword)
	require.Equal(t, freshError.Pointer, cachedError.Pointer)
	require.Equal(t, "compile", cachedError.Phase)
	require.Equal(t, "unconstructible", cachedError.Code)
	require.Equal(t, "allOf", cachedError.Keyword)
	require.Contains(t, cachedError.Pointer, "/components/schemas/Disjoint")
}

// TestMustHaveAllXValidCasesInvalidatesOccurrenceCacheOnce verifies direct option
// configuration after CompileSchema retains Domains and repeated enabling is idempotent.
func TestMustHaveAllXValidCasesInvalidatesOccurrenceCacheOnce(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: boolean`, "", "create"))
	registry := compiler.Domains

	_, err := compiler.CompileSchema(compiler.Source.RequestSchema)
	require.NoError(t, err)
	require.NotEmpty(t, compiler.usesByPointer)

	MustHaveAllXValidCases(compiler)
	require.Same(t, registry, compiler.Domains)
	require.Empty(t, compiler.usesByPointer)
	require.Nil(t, compiler.rootUse)

	_, err = compiler.Compile()
	require.NoError(t, err)
	require.NotNil(t, compiler.rootUse)
	require.NotEmpty(t, compiler.usesByPointer)

	rootUse := compiler.rootUse
	useCount := len(compiler.usesByPointer)
	MustHaveAllXValidCases(compiler)
	require.Same(t, rootUse, compiler.rootUse)
	require.Len(t, compiler.usesByPointer, useCount)
	require.Same(t, registry, compiler.Domains)
}

// TestCompileSuiteKeepsCachedStructuralMeetLocations verifies synthesized child
// meets retain the parent allOf boundary that fresh compilation reports.
func TestCompileSuiteKeepsCachedStructuralMeetLocations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
	}{
		{
			name: "property",
			schema: `allOf:
  - type: object
    properties:
      value: {type: string, pattern: '^A$', x-valid-examples: [A]}
  - type: object
    properties:
      value: {type: string, format: email, x-valid-examples: [B]}`,
		},
		{
			name: "array items",
			schema: `allOf:
  - type: array
    items: {type: string, pattern: '^A$', x-valid-examples: [A]}
  - type: array
    items: {type: string, format: email, x-valid-examples: [B]}`,
		},
		{
			name: "additional properties",
			schema: `allOf:
  - type: object
    additionalProperties: {type: string, pattern: '^A$', x-valid-examples: [A]}
  - type: object
    additionalProperties: {type: boolean, pattern: '^B$', x-valid-examples: [B]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source := parseSchemaSource(t, test.schema, "", "create")
			compiler := NewCompiler(source)

			_, err := compiler.Compile()
			require.NoError(t, err)

			_, err = compiler.CompileSuite(MustHaveAllXValidCases)
			require.Error(t, err)

			var cachedError *Error
			require.ErrorAs(t, err, &cachedError)

			_, err = NewCompiler(source).CompileSuite(MustHaveAllXValidCases)
			require.Error(t, err)

			var freshError *Error
			require.ErrorAs(t, err, &freshError)
			require.Equal(t, freshError.Phase, cachedError.Phase)
			require.Equal(t, freshError.Code, cachedError.Code)
			require.Equal(t, freshError.Keyword, cachedError.Keyword)
			require.Equal(t, freshError.Pointer, cachedError.Pointer)
			require.Equal(t, source.RequestSchema.Pointer, cachedError.Pointer)
		})
	}
}

// TestCompileSuiteKeepsCachedNestedMeetLocations verifies dependency-first
// validation reports an inner allOf before its enclosing synthesized result.
func TestCompileSuiteKeepsCachedNestedMeetLocations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		extra  string
	}{
		{
			name: "direct",
			schema: `allOf:
  - allOf:
      - {type: string, pattern: '^A$', x-valid-examples: [A]}
      - {type: string, format: email, x-valid-examples: [B]}`,
		},
		{
			name: "referenced",
			schema: `allOf:
  - {$ref: '#/components/schemas/Nested'}`,
			extra: `
components:
  schemas:
    Nested:
      allOf:
        - {type: string, pattern: '^A$', x-valid-examples: [A]}
        - {type: string, format: email, x-valid-examples: [B]}
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source := parseSchemaSource(t, test.schema, test.extra, "create")
			compiler := NewCompiler(source)

			_, err := compiler.Compile()
			require.NoError(t, err)

			_, err = compiler.CompileSuite(MustHaveAllXValidCases)
			require.Error(t, err)

			var cachedError *Error
			require.ErrorAs(t, err, &cachedError)

			_, err = NewCompiler(source).CompileSuite(MustHaveAllXValidCases)
			require.Error(t, err)

			var freshError *Error
			require.ErrorAs(t, err, &freshError)
			require.Equal(t, freshError.Phase, cachedError.Phase)
			require.Equal(t, freshError.Code, cachedError.Code)
			require.Equal(t, freshError.Keyword, cachedError.Keyword)
			require.Equal(t, freshError.Pointer, cachedError.Pointer)
			require.NotEqual(t, source.RequestSchema.Pointer, cachedError.Pointer)
		})
	}
}

// TestCompileSuiteKeepsFirstFoldShortCircuitAfterReuse verifies recompilation
// reports an outer fold before a later nested branch that fresh compilation never reaches.
func TestCompileSuiteKeepsFirstFoldShortCircuitAfterReuse(t *testing.T) {
	t.Parallel()

	source := parseSchemaSource(t, `type: string
pattern: '^root$'
x-valid-examples: [root]
allOf:
  - type: string
    format: email
    x-valid-examples: [first]
  - allOf:
      - {type: string, pattern: '^A$', x-valid-examples: [A]}
      - {type: string, format: email, x-valid-examples: [B]}`, "", "create")
	compiler := NewCompiler(source)

	_, err := compiler.Compile()
	require.NoError(t, err)

	_, err = compiler.CompileSuite(MustHaveAllXValidCases)
	require.Error(t, err)

	var cachedError *Error
	require.ErrorAs(t, err, &cachedError)

	_, err = NewCompiler(source).CompileSuite(MustHaveAllXValidCases)
	require.Error(t, err)

	var freshError *Error
	require.ErrorAs(t, err, &freshError)
	require.Equal(t, freshError.Phase, cachedError.Phase)
	require.Equal(t, freshError.Code, cachedError.Code)
	require.Equal(t, freshError.Keyword, cachedError.Keyword)
	require.Equal(t, freshError.Pointer, cachedError.Pointer)
	require.Equal(t, source.RequestSchema.Pointer, cachedError.Pointer)
}

// TestCompileSuiteIgnoresEmptyLocalOracleForUnreachableString verifies the
// allOf option does not invent a meet for a purely local declaration.
func TestCompileSuiteIgnoresEmptyLocalOracleForUnreachableString(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: boolean
pattern: '^never$'
x-valid-examples: []`, "", "create"))

	_, err := compiler.Compile()
	require.NoError(t, err)

	compiled, err := compiler.CompileSuite(MustHaveAllXValidCases)
	require.NoError(t, err)
	require.NotNil(t, compiled)
}

// TestCompilerKeepsOracleEvidenceOutOfDomainIdentity verifies canonical schema
// semantics do not depend on occurrence-local trusted values.
func TestCompilerKeepsOracleEvidenceOutOfDomainIdentity(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: object
properties:
  first:
    type: string
    pattern: '^same$'
    enum: [enum-only]
    x-valid-examples: [first-only]
  second:
    type: string
    pattern: '^same$'
    enum: [enum-only]
    x-valid-examples: [second-only]`, "", "create"))

	_, err := compiler.Compile()
	require.NoError(t, err)

	first := compiler.rootUse.property("first")
	second := compiler.rootUse.property("second")

	require.NotNil(t, first)
	require.NotNil(t, second)
	require.Equal(t, first.domain, second.domain)
	require.True(t, generationExamplesContain(first.examples.Valid, mustJSONValue(t, `"first-only"`)))
	require.False(t, generationExamplesContain(first.examples.Valid, mustJSONValue(t, `"second-only"`)))
	require.True(t, generationExamplesContain(second.examples.Valid, mustJSONValue(t, `"second-only"`)))
	require.False(t, generationExamplesContain(second.examples.Valid, mustJSONValue(t, `"first-only"`)))
}

// TestCompileSuiteUnionsLocalEnumAndValidExamples verifies both local oracle
// keywords contribute exact accepted cases before a later occurrence intersection.
func TestCompileSuiteUnionsLocalEnumAndValidExamples(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `allOf:
  - type: string
    pattern: '^first$'
    enum: [enum-only]
    x-valid-examples: [shared]
  - type: string
    pattern: '^second$'
    x-valid-examples: [shared]`, "", "create"))
	compiled, err := compiler.CompileSuite(MustHaveAllXValidCases)
	require.NoError(t, err)

	checkAcceptedCases(t, compiled, func(t require.TestingT, body []byte) {
		require.JSONEq(t, `"shared"`, string(body))
	})

	foundSource := false

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect == ExpectAccepted && plannedCase.Source.Keyword == "x-valid-examples" {
			require.Contains(t, plannedCase.Source.Pointer, "/allOf/0")

			foundSource = true
		}
	}

	require.True(t, foundSource)
}

// TestCompilerRejectsInvalidOraclePlacementAndOverlap verifies the extension contract is
// diagnosed at the declaring occurrence instead of being repaired from an outer occurrence.
func TestCompilerRejectsInvalidOraclePlacementAndOverlap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		code    string
		pointer string
		keyword string
	}{
		{
			name:    "valid extension without local opaque rule",
			schema:  "type: string\nx-valid-examples: [x]",
			code:    "malformed",
			keyword: "x-valid-examples",
		},
		{
			name: "outer example cannot certify child",
			schema: `x-valid-examples: [x]
allOf:
  - pattern: '^x$'`,
			code:    "malformed",
			keyword: "x-valid-examples",
		},
		{
			name:    "local overlap",
			schema:  "pattern: '^x$'\nx-valid-examples: [x]\nx-invalid-examples: [x]",
			code:    "malformed",
			keyword: "x-invalid-examples",
		},
		{
			name: "merged overlap",
			schema: `allOf:
  - pattern: '^x$'
    x-valid-examples: [x]
  - format: email
    x-valid-examples: [x]
    x-invalid-examples: [x]`,
			code:    "malformed",
			pointer: "/allOf/1",
			keyword: "x-invalid-examples",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			compiler := NewCompiler(parseSchemaSource(t, tt.schema, "", "create"))
			_, err := compiler.CompileSuite(MustHaveAllXValidCases)
			require.Error(t, err)

			var compileError *Error
			require.ErrorAs(t, err, &compileError)
			require.Equal(t, tt.code, compileError.Code)
			require.Equal(t, compiler.Source.RequestSchema.Pointer+tt.pointer, compileError.Pointer)
			require.Equal(t, tt.keyword, compileError.Keyword)
		})
	}
}

// mustJSONValue parses one exact JSON test value.
func mustJSONValue(t *testing.T, raw string) jsonvalue.Value {
	t.Helper()

	value, err := jsonvalue.Parse([]byte(raw))
	require.NoError(t, err)

	return value
}

// TestCompilerExposesIssueTwoReachableKinds verifies compilation across independent JSON kinds.
func TestCompilerExposesIssueTwoReachableKinds(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		schema string
		check  func(*testing.T, Domain)
	}{
		"optional type and mixed families": {
			schema: `minLength: 3
minimum: 10
minProperties: 2`,
			check: func(t *testing.T, domain Domain) {
				t.Helper()
				require.Equal(t, KindUnrestricted, domain.Null)
				require.Equal(t, KindUnrestricted, domain.Boolean)
				require.Equal(t, KindRestricted, domain.Number.State)
				require.Equal(t, "10", domain.Number.Minimum.Value.Lexeme)
				require.Equal(t, KindRestricted, domain.String.State)
				require.Equal(t, 3, domain.String.MinLength)
				require.Equal(t, KindUnrestricted, domain.Array.State)
				require.Equal(t, KindRestricted, domain.Object.State)
				require.Equal(t, 2, domain.Object.MinProps)
			},
		},
		"explicit type leaves unrelated family inert": {
			schema: `type: string
minLength: 3
minProperties: 2`,
			check: func(t *testing.T, domain Domain) {
				t.Helper()
				require.Equal(t, KindRestricted, domain.String.State)
				require.Equal(t, 3, domain.String.MinLength)
				require.Equal(t, KindExcluded, domain.Object.State)
				require.Equal(t, KindExcluded, domain.Number.State)
			},
		},
		"same node nullable": {
			schema: `type: boolean
nullable: true`,
			check: func(t *testing.T, domain Domain) {
				t.Helper()
				require.Equal(t, KindUnrestricted, domain.Null)
				require.Equal(t, KindUnrestricted, domain.Boolean)
				require.Equal(t, KindExcluded, domain.String.State)
			},
		},
		"nullable without type is inert": {
			schema: `nullable: true`,
			check: func(t *testing.T, domain Domain) {
				t.Helper()
				require.Equal(t, KindUnrestricted, domain.Null)
				require.Equal(t, KindUnrestricted, domain.Boolean)
				require.Equal(t, KindUnrestricted, domain.Number.State)
				require.Equal(t, KindUnrestricted, domain.String.State)
				require.Equal(t, KindUnrestricted, domain.Array.State)
				require.Equal(t, KindUnrestricted, domain.Object.State)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			compiler, id := compileSchemaYAML(t, tt.schema, "")
			domain, ok := compiler.Domains.Domain(id)
			require.True(t, ok)
			tt.check(t, domain)
		})
	}
}

// TestCompilerDistinguishesEmptyDomainsFromExcludedKinds verifies contradiction reachability.
func TestCompilerDistinguishesEmptyDomainsFromExcludedKinds(t *testing.T) {
	t.Parallel()

	_, typedID := compileSchemaYAML(t, `type: string
minLength: 2
maxLength: 1`, "")
	require.Equal(t, EmptyDomainID, typedID)

	compiler, untypedID := compileSchemaYAML(t, `minLength: 2
maxLength: 1`, "")
	require.NotEqual(t, EmptyDomainID, untypedID)
	domain, ok := compiler.Domains.Domain(untypedID)
	require.True(t, ok)
	require.Equal(t, KindExcluded, domain.String.State)
	require.Equal(t, KindUnrestricted, domain.Boolean)
	require.Equal(t, KindUnrestricted, domain.Object.State)
}

// TestCompilerCanonicalizesMixedEnums verifies exact enum value canonicalization.
func TestCompilerCanonicalizesMixedEnums(t *testing.T) {
	t.Parallel()

	compiler, id := compileSchemaYAML(t, `
enum:
  - 1
  - 1.0
  - true
  - text
  - null
  - {b: 2, a: 1}
`, "")
	domain, ok := compiler.Domains.Domain(id)
	require.True(t, ok)
	require.NotNil(t, domain.Enum)
	require.Len(t, domain.Enum.Values, 5)
	require.Equal(t, KindUnrestricted, domain.Null)
	require.Equal(t, KindUnrestricted, domain.Boolean)
	require.Equal(t, KindUnrestricted, domain.Number.State)
	require.Equal(t, KindUnrestricted, domain.String.State)
	require.Equal(t, KindUnrestricted, domain.Object.State)
	require.Equal(t, KindExcluded, domain.Array.State)

	integerCompiler, integerOnly := compileSchemaYAML(t, `type: integer
enum: [1.0, 2.5, "1"]`, "")
	integerDomain, ok := integerCompiler.Domains.Domain(integerOnly)
	require.True(t, ok)
	require.Len(t, integerDomain.Enum.Values, 3)
}

// TestCompilerReusesEquivalentNestedAndReferencedSchemas verifies canonical nested Domain reuse.
func TestCompilerReusesEquivalentNestedAndReferencedSchemas(t *testing.T) {
	t.Parallel()

	compiler, rootID := compileSchemaYAML(t, `
type: object
properties:
  first: {$ref: '#/components/schemas/Text'}
  second: {$ref: '#/components/schemas/Text'}
  recreated: {type: string, minLength: 2}
  nested:
    type: array
    items: {$ref: '#/components/schemas/Text'}
`, `
components:
  schemas:
    Text:
      type: string
      minLength: 2
`)
	root, ok := compiler.Domains.Domain(rootID)
	require.True(t, ok)

	properties := propertiesByName(root.Object.Properties)
	require.Equal(t, properties["first"].Values, properties["second"].Values)
	require.Equal(t, properties["first"].Values, properties["recreated"].Values)

	nested, ok := compiler.Domains.Domain(properties["nested"].Values)
	require.True(t, ok)
	require.Equal(t, properties["first"].Values, nested.Array.Items)
}

// TestCompilerKeepsExamplesOutOfDomainIdentity verifies examples do not affect Domain identity.
func TestCompilerKeepsExamplesOutOfDomainIdentity(t *testing.T) {
	t.Parallel()

	compiler, rootID := compileSchemaYAML(t, `
type: object
properties:
  first:
    type: string
    pattern: same
    x-valid-examples: [first]
  second:
    type: string
    pattern: same
    x-valid-examples: [second]
`, "")
	root, ok := compiler.Domains.Domain(rootID)
	require.True(t, ok)

	properties := propertiesByName(root.Object.Properties)
	require.Equal(t, properties["first"].Values, properties["second"].Values)

	firstUse := schemaUseAt(t, compiler.rootUse, compiler.Source.RequestSchema.Pointer+"/properties/first")
	secondUse := schemaUseAt(t, compiler.rootUse, compiler.Source.RequestSchema.Pointer+"/properties/second")
	require.Equal(t, "first", firstUse.examples.Valid[0].Value.String)
	require.Equal(t, "second", secondUse.examples.Valid[0].Value.String)
}

// TestCompilerReportsMalformedUnsupportedAndRecursiveSchemas verifies stable compiler failures.
func TestCompilerReportsMalformedUnsupportedAndRecursiveSchemas(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		schema string
		extra  string
		code   string
		text   string
	}{
		"malformed bound": {
			schema: `minLength: -1`,
			code:   "malformed",
			text:   "must not be negative",
		},
		"malformed examples": {
			schema: "pattern: x\nx-valid-examples: nope",
			code:   "malformed",
			text:   "must be an array",
		},
		"unsupported composition": {
			schema: `oneOf: [{type: string}]`,
			code:   "unsupported",
			text:   "oneOf is unsupported",
		},
		"recursive reference": {
			schema: `type: object
properties:
  child: {$ref: '#/components/schemas/Node'}`,
			extra: `
components:
  schemas:
    Node:
      type: object
      properties:
        child: {$ref: '#/components/schemas/Node'}
`,
			code: "unsupported",
			text: "recursive schema references are unsupported",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			source := parseSchemaSource(t, tt.schema, tt.extra, "create")
			_, err := NewCompiler(source).Compile()
			require.ErrorContains(t, err, tt.text)

			var compileError *Error
			require.True(t, errors.As(err, &compileError))
			require.Equal(t, tt.code, compileError.Code)
		})
	}
}

// TestCompilerDoesNotValidateEnumOraclesThroughNestedDomains verifies local enum values remain unchecked.
func TestCompilerDoesNotValidateEnumOraclesThroughNestedDomains(t *testing.T) {
	t.Parallel()

	compiler, id := compileSchemaYAML(t, `
type: object
properties:
  value: {enum: [1]}
enum:
  - {value: 1}
  - {value: 2}
`, "")
	domain, ok := compiler.Domains.Domain(id)
	require.True(t, ok)
	require.Len(t, domain.Enum.Values, 2)

	expected, err := jsonvalue.Parse([]byte(`{"value":2}`))
	require.NoError(t, err)
	require.True(t, jsonValuesContain(domain.Enum.Values, expected))
}

// TestCompilerRejectsNullTypedKeywords verifies JSON null is not silently decoded as a zero value.
func TestCompilerRejectsNullTypedKeywords(t *testing.T) {
	t.Parallel()

	for _, schema := range []string{
		"minLength: null",
		"nullable: null",
		"additionalProperties: null",
		"x-valid-examples: null",
		"uniqueItems: null",
	} {
		t.Run(schema, func(t *testing.T) {
			t.Parallel()

			source := parseSchemaSource(t, schema, "", "create")
			_, err := NewCompiler(source).Compile()
			require.Error(t, err)

			var compileError *Error
			require.True(t, errors.As(err, &compileError))
			require.Equal(t, "malformed", compileError.Code)
		})
	}
}

// TestCompilerAcceptsEmptyRequiredPropertyName verifies all JSON object names remain legal.
func TestCompilerAcceptsEmptyRequiredPropertyName(t *testing.T) {
	t.Parallel()

	compiler, id := compileSchemaYAML(t, `type: object
required: [""]`, "")
	domain, ok := compiler.Domains.Domain(id)
	require.True(t, ok)
	require.Equal(t, "", domain.Object.Properties[0].Name)
	require.True(t, domain.Object.Properties[0].Required)
}

// TestDomainRegistryNormalizesNoOpConstraints verifies semantic reuse after normalization.
func TestDomainRegistryNormalizesNoOpConstraints(t *testing.T) {
	t.Parallel()

	compiler, rootID := compileSchemaYAML(t, `
type: object
properties:
  plain: {}
  stringNoOp: {minLength: 0}
  arrayPlain: {type: array, items: {}}
  arrayNoOp: {type: array, minItems: 0, items: {}}
  objectPlain: {type: object}
  objectNoOp: {type: object, minProperties: 0, additionalProperties: true}
`, "")
	root, ok := compiler.Domains.Domain(rootID)
	require.True(t, ok)

	properties := propertiesByName(root.Object.Properties)
	require.Equal(t, properties["plain"].Values, properties["stringNoOp"].Values)
	require.Equal(t, properties["arrayPlain"].Values, properties["arrayNoOp"].Values)
	require.Equal(t, properties["objectPlain"].Values, properties["objectNoOp"].Values)
}

// TestDomainRegistryUsesSemanticEnumEquality verifies object member order does not affect Domain identity.
func TestDomainRegistryUsesSemanticEnumEquality(t *testing.T) {
	t.Parallel()

	compiler, rootID := compileSchemaYAML(t, `
type: object
properties:
  first: {enum: [{a: 1, b: 2}]}
  second: {enum: [{b: 2, a: 1.0}]}
`, "")
	root, ok := compiler.Domains.Domain(rootID)
	require.True(t, ok)

	properties := propertiesByName(root.Object.Properties)
	require.Equal(t, properties["first"].Values, properties["second"].Values)
}

// TestDomainRegistryVerifiesHashCollisions verifies hash collisions do not merge distinct Domains.
func TestDomainRegistryVerifiesHashCollisions(t *testing.T) {
	t.Parallel()

	registry := NewDomainRegistry()
	first := registry.FindOrAddEquivalentDomain(Domain{
		String: StringConstraints{State: KindRestricted, MinLength: 1},
		Status: DomainProductive,
	})
	secondCandidate := Domain{
		String: StringConstraints{State: KindRestricted, MinLength: 2},
		Status: DomainProductive,
	}
	secondHash := registry.semanticDomainHash(registry.normalizeDomain(secondCandidate))
	registry.IDsByHash[secondHash] = append(registry.IDsByHash[secondHash], first)

	second := registry.FindOrAddEquivalentDomain(secondCandidate)
	require.NotEqual(t, first, second)
	require.Equal(t, second, registry.FindOrAddEquivalentDomain(secondCandidate))
}

// TestCompilerDoesNotValidateLocalEnumOraclesAgainstSiblingRules verifies local oracle trust.
func TestCompilerDoesNotValidateLocalEnumOraclesAgainstSiblingRules(t *testing.T) {
	t.Parallel()

	compiler, id := compileSchemaYAML(t, `type: integer
pattern: '^x$'
x-valid-examples: [x]
enum: [1, x]`, "")
	domain := mustDomain(t, compiler.Domains, id)
	require.Len(t, domain.Enum.Values, 2)

	_, productive := compileSchemaYAML(t, `type: string
minLength: 3
pattern: '^x$'
x-valid-examples: [x]
enum: [x]`, "")
	require.NotEqual(t, EmptyDomainID, productive)
}

// TestCompileSuiteExecutesUncheckedLocalEnumMembers verifies same-occurrence sibling
// rules never filter trusted enum partitions, including recursively constrained values.
func TestCompileSuiteExecutesUncheckedLocalEnumMembers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		values []string
	}{
		{
			name: "type",
			schema: `type: integer
enum: [1, text]`,
			values: []string{`1`, `"text"`},
		},
		{
			name: "length",
			schema: `type: string
minLength: 3
enum: [x, long]`,
			values: []string{`"x"`, `"long"`},
		},
		{
			name: "nested property",
			schema: `type: object
properties: {value: {enum: [1]}}
enum: [{value: 1}, {value: 2}]`,
			values: []string{`{"value":1}`, `{"value":2}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			compiler := NewCompiler(parseSchemaSource(t, tt.schema, "", "create"))
			compiled, err := compiler.CompileSuite()
			require.NoError(t, err)

			for _, expected := range tt.values {
				value := mustJSONValue(t, expected)
				found := false

				for _, plannedCase := range compiled.Cases {
					caseDomain := mustDomain(t, compiled.Domains, plannedCase.Values)
					if plannedCase.Expect == ExpectAccepted && caseDomain.Enum != nil &&
						len(caseDomain.Enum.Values) == 1 && enumContains(caseDomain.Enum, value) {
						found = true

						break
					}
				}

				require.True(t, found, "missing exact accepted enum partition for %s", expected)
			}

			checkAcceptedCases(t, compiled, func(t require.TestingT, body []byte) {
				require.Contains(t, tt.values, string(body))
			})
		})
	}
}

// TestCompilerFiltersEnumOnlyAcrossSeparateAllOfOccurrences verifies the meet seam,
// rather than the enum occurrence itself, applies sibling constraints.
func TestCompilerFiltersEnumOnlyAcrossSeparateAllOfOccurrences(t *testing.T) {
	t.Parallel()

	compiler, id := compileSchemaYAML(t, `allOf:
  - enum: [-1, 1, 3]
  - {type: integer, minimum: 0, maximum: 2}`, "")
	domain := mustDomain(t, compiler.Domains, id)
	require.Len(t, domain.Enum.Values, 1)
	require.Equal(t, "1", domain.Enum.Values[0].Number.Lexeme)
}

// TestCompilerDoesNotUseEnumBranchExamplesAsPatternProof verifies allOf branch-local trust.
func TestCompilerDoesNotUseEnumBranchExamplesAsPatternProof(t *testing.T) {
	t.Parallel()

	source := parseSchemaSource(t, `allOf:
  - enum: [bad]
  - pattern: '^ok$'`, "", "create")
	_, err := NewCompiler(source).Compile()
	require.ErrorContains(t, err, "unconstructible")
}

// TestCompilerValidatesAdjustedOpenAPIFields verifies malformed and required adjusted fields.
func TestCompilerValidatesAdjustedOpenAPIFields(t *testing.T) {
	t.Parallel()

	for _, schema := range []string{
		"type: array",
		"type: boolean\nformat: 7",
		"readOnly: yes",
		"readOnly: true\nwriteOnly: true",
	} {
		t.Run(schema, func(t *testing.T) {
			t.Parallel()

			_, err := NewCompiler(parseSchemaSource(t, schema, "", "create")).Compile()
			require.Error(t, err)

			var compileError *Error
			require.ErrorAs(t, err, &compileError)
			require.Equal(t, "malformed", compileError.Code)
		})
	}
}

// TestCompilerAppliesReadOnlyRequirednessForRequests verifies required read-only properties remain optional.
func TestCompilerAppliesReadOnlyRequirednessForRequests(t *testing.T) {
	t.Parallel()

	compiler, id := compileSchemaYAML(t, `type: object
required: [id, name]
properties:
  id: {type: string, readOnly: true}
  name: {type: string}`, "")
	domain := mustDomain(t, compiler.Domains, id)
	properties := propertiesByName(domain.Object.Properties)
	require.False(t, properties["id"].Required)
	require.True(t, properties["name"].Required)
}

// TestDomainRegistryDeduplicatesSemanticEnumMembers verifies finite-set identity ignores duplicates.
func TestDomainRegistryDeduplicatesSemanticEnumMembers(t *testing.T) {
	t.Parallel()

	one, err := jsonvalue.Parse([]byte("1"))
	require.NoError(t, err)
	oneDecimal, err := jsonvalue.Parse([]byte("1.0"))
	require.NoError(t, err)
	oneExponent, err := jsonvalue.Parse([]byte("1e0"))
	require.NoError(t, err)

	registry := NewDomainRegistry()
	first := registry.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{one}))
	second := registry.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{one, oneDecimal, oneExponent}))
	require.Equal(t, first, second)
}

// compileSchemaYAML compiles a request schema and optional OpenAPI components.
func compileSchemaYAML(t *testing.T, schema string, extra string) (*Compiler, DomainID) {
	t.Helper()

	source := parseSchemaSource(t, schema, extra, "create")
	compiler := NewCompiler(source)
	id, err := compiler.Compile()
	require.NoError(t, err)

	return compiler, id
}

// parseSchemaSource builds an OpenAPI source containing a request schema.
func parseSchemaSource(t *testing.T, schema string, extra string, operation string) oas.Source {
	t.Helper()

	spec := `
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: ` + operation + `
      requestBody:
        content:
          application/json:
            schema:
` + indent(schema, 14) + "\n" + extra
	source, err := oas.Parse([]byte(spec), operation)
	require.NoError(t, err)

	return source
}

// indent prefixes every non-empty input line with a fixed number of spaces.
func indent(value string, spaces int) string {
	prefix := ""
	for range spaces {
		prefix += " "
	}

	lines := strings.Split(strings.TrimSpace(value), "\n")
	for index := range lines {
		lines[index] = prefix + lines[index]
	}

	return strings.Join(lines, "\n")
}

// propertiesByName indexes test properties by name.
func propertiesByName(properties []NamedProperty) map[string]NamedProperty {
	result := make(map[string]NamedProperty, len(properties))
	for _, property := range properties {
		result[property.Name] = property
	}

	return result
}

// schemaUseAt returns test metadata for a source pointer.
func schemaUseAt(t *testing.T, root *schemaUse, pointer string) *schemaUse {
	t.Helper()

	use := root.find(pointer)
	if use != nil {
		return use
	}

	require.FailNow(t, "schema use not found", pointer)

	return nil
}

// TestFiniteEnumUsesExactSemanticJSON verifies semantic JSON equality removes duplicate enum values.
func TestFiniteEnumUsesExactSemanticJSON(t *testing.T) {
	t.Parallel()

	compiler, id := compileSchemaYAML(t, `enum: [{a: 1, b: [true]}, {b: [true], a: 1.0}]`, "")
	domain, ok := compiler.Domains.Domain(id)
	require.True(t, ok)
	require.Len(t, domain.Enum.Values, 1)

	expected, err := jsonvalue.Parse([]byte(`{"a":1,"b":[true]}`))
	require.NoError(t, err)
	require.True(t, expected.Equal(domain.Enum.Values[0]))
}
