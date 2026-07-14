package suite

import (
	"errors"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/internal/jsonvalue"
	"github.com/stretchr/testify/require"
)

// TestIntersectDomainsObeysAlgebraLawsAndCachesUnorderedPairs verifies canonical domain algebra.
func TestIntersectDomainsObeysAlgebraLawsAndCachesUnorderedPairs(t *testing.T) {
	t.Parallel()

	compiler, a := compileSchemaYAML(t, `minLength: 2`, "")
	b := compileInto(t, compiler, `maxLength: 5`)
	c := compileInto(t, compiler, `type: string
nullable: true`)

	ab, err := compiler.Domains.IntersectDomains(a, b)
	require.NoError(t, err)
	ba, err := compiler.Domains.IntersectDomains(b, a)
	require.NoError(t, err)
	require.Equal(t, ab, ba)
	require.Equal(t, ab, compiler.Domains.IntersectionResults[canonicalDomainPair(a, b)])

	left, err := compiler.Domains.IntersectDomains(ab, c)
	require.NoError(t, err)
	bc, err := compiler.Domains.IntersectDomains(b, c)
	require.NoError(t, err)
	right, err := compiler.Domains.IntersectDomains(a, bc)
	require.NoError(t, err)
	require.Equal(t, left, right)

	aa, err := compiler.Domains.IntersectDomains(a, a)
	require.NoError(t, err)
	require.Equal(t, a, aa)
	withAny, err := compiler.Domains.IntersectDomains(a, AnyJSONDomainID)
	require.NoError(t, err)
	require.Equal(t, a, withAny)
	empty, err := compiler.Domains.IntersectDomains(a, EmptyDomainID)
	require.NoError(t, err)
	require.Equal(t, EmptyDomainID, empty)

	formatA := compileInto(t, compiler, `type: number
format: int32`)
	formatB := compileInto(t, compiler, `type: number
format: int64`)
	formatC := compileInto(t, compiler, `type: number
format: int64`)
	formatAB, err := compiler.Domains.IntersectDomains(formatA, formatB)
	require.NoError(t, err)
	formatLeft, err := compiler.Domains.IntersectDomains(formatAB, formatC)
	require.NoError(t, err)
	formatBC, err := compiler.Domains.IntersectDomains(formatB, formatC)
	require.NoError(t, err)
	formatRight, err := compiler.Domains.IntersectDomains(formatA, formatBC)
	require.NoError(t, err)
	require.Equal(t, formatLeft, formatRight)
}

// TestIntersectDomainsMergesKindsNumbersAndEnums verifies scalar and finite intersections.
func TestIntersectDomainsMergesKindsNumbersAndEnums(t *testing.T) {
	t.Parallel()

	compiler, left := compileSchemaYAML(t, `type: integer
nullable: true
minimum: 1
exclusiveMinimum: true
multipleOf: 1.5
format: int64`, "")
	right := compileInto(t, compiler, `type: number
nullable: true
minimum: 2
maximum: 20
multipleOf: 2.5
format: double`)

	id, err := compiler.Domains.IntersectDomains(left, right)
	require.NoError(t, err)
	domain := mustDomain(t, compiler.Domains, id)
	require.Equal(t, KindUnrestricted, domain.Null)
	require.Equal(t, KindRestricted, domain.Number.State)
	require.True(t, domain.Number.IntegersOnly)
	require.Equal(t, "2", domain.Number.Minimum.Value.Lexeme)
	require.Equal(t, "20", domain.Number.Maximum.Value.Lexeme)
	require.Equal(t, "7.5", domain.Number.MultipleOf.Lexeme)
	require.Equal(t, "int64", *domain.Number.Format)

	stringNullable := compileInto(t, compiler, `type: string
nullable: true`)
	booleanNullable := compileInto(t, compiler, `type: boolean
nullable: true`)
	nullOnly, err := compiler.Domains.IntersectDomains(stringNullable, booleanNullable)
	require.NoError(t, err)
	nullDomain := mustDomain(t, compiler.Domains, nullOnly)
	require.Equal(t, KindUnrestricted, nullDomain.Null)
	require.Equal(t, KindExcluded, nullDomain.Boolean)
	require.Equal(t, KindExcluded, nullDomain.String.State)

	enumCompiler, enumID := compileSchemaYAML(t, `allOf:
  - enum: [null, true, 1, text]
  - enum: [null, false, 1.0, text]`, "")
	enumDomain := mustDomain(t, enumCompiler.Domains, enumID)
	require.Len(t, enumDomain.Enum.Values, 3)
}

// TestIntersectDomainsHandlesArrayAndObjectProductivity verifies recursive container feasibility.
func TestIntersectDomainsHandlesArrayAndObjectProductivity(t *testing.T) {
	t.Parallel()

	arrayCompiler, arrayID := compileSchemaYAML(t, `allOf:
  - {type: array, items: {type: string}}
  - {type: array, items: {type: boolean}}`, "")
	array := mustDomain(t, arrayCompiler.Domains, arrayID)
	require.Equal(t, KindRestricted, array.Array.State)
	require.Equal(t, EmptyDomainID, array.Array.Items)
	require.Equal(t, 0, *array.Array.MaxItems)

	_, impossibleArray := compileSchemaYAML(t, `allOf:
  - {type: array, minItems: 1, items: {type: string}}
  - {type: array, items: {type: boolean}}`, "")
	require.Equal(t, EmptyDomainID, impossibleArray)

	objectCompiler, objectID := compileSchemaYAML(t, `allOf:
  - type: object
    properties: {a: {type: string}}
    additionalProperties: false
  - type: object
    properties: {b: {type: integer}}
    additionalProperties: false`, "")
	object := mustDomain(t, objectCompiler.Domains, objectID)
	properties := propertiesByName(object.Object.Properties)
	require.Equal(t, PropertyForbidden, properties["a"].State)
	require.Equal(t, PropertyForbidden, properties["b"].State)
	require.Equal(t, EmptyDomainID, object.Object.Additional.Values)

	_, requiredImpossible := compileSchemaYAML(t, `allOf:
  - {type: object, required: [a], properties: {a: {type: string}}}
  - {type: object, additionalProperties: false}`, "")
	require.Equal(t, EmptyDomainID, requiredImpossible)

	_, countImpossible := compileSchemaYAML(t, `allOf:
  - {type: object, minProperties: 1, properties: {a: {type: string}}, additionalProperties: false}
  - {type: object, properties: {a: {type: string}}, additionalProperties: false}`, "")
	require.NotEqual(t, EmptyDomainID, countImpossible)
}

// TestCompilerFoldsNestedAllOfSiblingsAndPreservesProvenance verifies allOf source metadata.
func TestCompilerFoldsNestedAllOfSiblingsAndPreservesProvenance(t *testing.T) {
	t.Parallel()

	compiler, id := compileSchemaYAML(t, `type: string
maxLength: 8
x-valid-examples: [trusted]
x-invalid-examples: [outer-invalid]
allOf:
  - minLength: 2
    x-valid-examples: [child]
    x-invalid-examples: [child-invalid]
  - allOf:
      - maxLength: 5
      - minLength: 3`, "")
	domain := mustDomain(t, compiler.Domains, id)
	require.Equal(t, 3, domain.String.MinLength)
	require.Equal(t, 5, *domain.String.MaxLength)

	use := schemaUseAt(t, compiler.rootUse, compiler.Source.RequestSchema.Pointer)
	require.Equal(t, "trusted", use.examples.Valid[0].String)
	require.Len(t, use.examples.Invalid, 2)
	require.Contains(t, use.constraints, ConstraintSource{
		Pointer: compiler.Source.RequestSchema.Pointer,
		Keyword: "allOf",
	})
	require.Contains(t, use.constraints, ConstraintSource{
		Pointer: compiler.Source.RequestSchema.Pointer + "/allOf/0", Keyword: "minLength",
	})
}

// TestCompilerUsesTrustedExamplesForPatternAndFormatConjunctions verifies opaque string languages.
func TestCompilerUsesTrustedExamplesForPatternAndFormatConjunctions(t *testing.T) {
	t.Parallel()

	compiler, id := compileSchemaYAML(t, `allOf:
  - pattern: '^a$'
    x-valid-examples: [not-a]
  - format: email
    x-valid-examples: [not-a]`, "")
	domain := mustDomain(t, compiler.Domains, id)
	require.Equal(t, []string{"^a$"}, domain.String.Patterns)
	require.Equal(t, []string{"email"}, domain.String.Formats)
	use := schemaUseAt(t, compiler.rootUse, compiler.Source.RequestSchema.Pointer)
	require.Equal(t, "not-a", use.examples.Valid[0].String)

	outerCompiler, outerID := compileSchemaYAML(t, `x-valid-examples: [outer-trusted]
allOf:
  - pattern: first
  - format: email`, "")
	require.NotEqual(t, EmptyDomainID, outerID)
	outerUse := schemaUseAt(t, outerCompiler.rootUse, outerCompiler.Source.RequestSchema.Pointer)
	require.Equal(t, "outer-trusted", outerUse.examples.Valid[0].String)

	source := parseSchemaSource(t, `allOf:
  - pattern: '^a$'
    x-valid-examples: [a]
  - format: email
    x-valid-examples: [b]`, "", "create")
	_, err := NewCompiler(source).CompileSuite(MustHaveAllXValidCases)
	require.Error(t, err)

	var compileError *Error
	require.True(t, errors.As(err, &compileError))
	require.Equal(t, "unconstructible", compileError.Code)
}

// TestCompilerDistinguishesMalformedUnsupportedUnconstructibleAndEmptyAllOf verifies outcomes.
func TestCompilerDistinguishesMalformedUnsupportedUnconstructibleAndEmptyAllOf(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		schema string
		code   string
	}{
		"malformed":   {schema: `allOf: []`, code: "malformed"},
		"unsupported": {schema: `allOf: [{anyOf: [{type: string}]}]`, code: "unsupported"},
		"mixed enum cannot be narrowed": {schema: `pattern: a
x-valid-examples: [other]
enum: [1, a]`, code: "unconstructible"},
	}
	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := NewCompiler(parseSchemaSource(t, testCase.schema, "", "create")).Compile()
			require.Error(t, err)

			var compileError *Error
			require.True(t, errors.As(err, &compileError))
			require.Equal(t, testCase.code, compileError.Code)
		})
	}

	_, empty := compileSchemaYAML(t, `allOf: [{type: string}, {type: boolean}]`, "")
	require.Equal(t, EmptyDomainID, empty)

	_, emptyInteger := compileSchemaYAML(t, `type: integer
minimum: 0.1
maximum: 0.9`, "")
	require.Equal(t, EmptyDomainID, emptyInteger)

	_, emptyMultiple := compileSchemaYAML(t, `type: number
minimum: 1
maximum: 1
multipleOf: 2`, "")
	require.Equal(t, EmptyDomainID, emptyMultiple)

	huge, err := jsonvalue.ParseNumber("1e100001")
	require.NoError(t, err)
	productive, err := numberConstraintsAreProductive(NumberConstraints{
		State:   KindRestricted,
		Minimum: &NumberBound{Value: huge},
	})
	require.NoError(t, err)
	require.True(t, productive)

	largerHuge, err := jsonvalue.ParseNumber("2e100001")
	require.NoError(t, err)

	comparison, ok := compareExactNumbers(huge, largerHuge)
	require.True(t, ok)
	require.Negative(t, comparison)
}

// compileInto copies one separately compiled test Domain graph into a registry.
func compileInto(t *testing.T, compiler *Compiler, schema string) DomainID {
	t.Helper()

	source := parseSchemaSource(t, schema, "", "other")
	other := NewCompiler(source)
	id, err := other.Compile()
	require.NoError(t, err)
	domain := mustDomain(t, other.Domains, id)

	return copyDomainGraph(t, compiler.Domains, other.Domains, domain)
}

// copyDomainGraph recursively canonicalizes one test Domain graph in another registry.
func copyDomainGraph(t *testing.T, target *DomainRegistry, source *DomainRegistry, domain Domain) DomainID {
	t.Helper()

	if domain.Array.State != KindExcluded && domain.Array.Items > EmptyDomainID {
		child := mustDomain(t, source, domain.Array.Items)
		domain.Array.Items = copyDomainGraph(t, target, source, child)
	}

	if domain.Object.State != KindExcluded {
		for index := range domain.Object.Properties {
			if domain.Object.Properties[index].Values > EmptyDomainID {
				child := mustDomain(t, source, domain.Object.Properties[index].Values)
				domain.Object.Properties[index].Values = copyDomainGraph(t, target, source, child)
			}
		}

		if domain.Object.Additional.Values > EmptyDomainID {
			child := mustDomain(t, source, domain.Object.Additional.Values)
			domain.Object.Additional.Values = copyDomainGraph(t, target, source, child)
		}
	}

	return target.FindOrAddEquivalentDomain(domain)
}

// mustDomain returns an existing test Domain.
func mustDomain(t *testing.T, registry *DomainRegistry, id DomainID) Domain {
	t.Helper()

	domain, ok := registry.Domain(id)
	require.True(t, ok)

	return domain
}

// TestIntersectDomainsRejectsMissingDomainIDs verifies invalid registry input is malformed.
func TestIntersectDomainsRejectsMissingDomainIDs(t *testing.T) {
	t.Parallel()

	_, err := NewDomainRegistry().IntersectDomains(DomainID(100), AnyJSONDomainID)
	require.Error(t, err)
}

// TestIntersectDomainsUsesSemanticEnumEquality verifies exact finite set identity.
func TestIntersectDomainsUsesSemanticEnumEquality(t *testing.T) {
	t.Parallel()

	leftValue, err := jsonvalue.Parse([]byte(`{"a":1,"b":2}`))
	require.NoError(t, err)
	rightValue, err := jsonvalue.Parse([]byte(`{"b":2.0,"a":1e0}`))
	require.NoError(t, err)

	registry := NewDomainRegistry()
	left := registry.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{leftValue}))
	right := registry.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{rightValue}))
	merged, err := registry.IntersectDomains(left, right)
	require.NoError(t, err)
	require.NotEqual(t, EmptyDomainID, merged)
}
