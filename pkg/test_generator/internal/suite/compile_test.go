package suite

import (
	"errors"
	"strings"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/internal/jsonvalue"
	"decode_and_validate_generator/pkg/test_generator/internal/oas"
	"github.com/stretchr/testify/require"
)

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
	require.Len(t, integerDomain.Enum.Values, 1)
	require.Equal(t, "1", integerDomain.Enum.Values[0].Number.Lexeme)
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
    x-valid-examples: [first]
  second:
    type: string
    x-valid-examples: [second]
`, "")
	root, ok := compiler.Domains.Domain(rootID)
	require.True(t, ok)

	properties := propertiesByName(root.Object.Properties)
	require.Equal(t, properties["first"].Values, properties["second"].Values)

	firstUse := schemaUseAt(t, compiler.rootUse, compiler.Source.RequestSchema.Pointer+"/properties/first")
	secondUse := schemaUseAt(t, compiler.rootUse, compiler.Source.RequestSchema.Pointer+"/properties/second")
	require.Equal(t, "first", firstUse.examples.Valid[0].String)
	require.Equal(t, "second", secondUse.examples.Valid[0].String)
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
			schema: `x-valid-examples: nope`,
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

// TestCompilerFiltersEnumsThroughNestedDomains verifies nested enum constraints filter parent enums.
func TestCompilerFiltersEnumsThroughNestedDomains(t *testing.T) {
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
	require.Len(t, domain.Enum.Values, 1)

	expected, err := jsonvalue.Parse([]byte(`{"value":1}`))
	require.NoError(t, err)
	require.True(t, expected.Equal(domain.Enum.Values[0]))
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

// TestCompilerAppliesModeledConstraintsBeforeTrustingStringEnumExamples verifies examples only prove languages.
func TestCompilerAppliesModeledConstraintsBeforeTrustingStringEnumExamples(t *testing.T) {
	t.Parallel()

	compiler, id := compileSchemaYAML(t, `type: integer
pattern: '^x$'
x-valid-examples: [x]
enum: [1, x]`, "")
	domain := mustDomain(t, compiler.Domains, id)
	require.Len(t, domain.Enum.Values, 1)
	require.Equal(t, jsonvalue.KindNumber, domain.Enum.Values[0].Kind)

	_, empty := compileSchemaYAML(t, `type: string
minLength: 3
pattern: '^x$'
x-valid-examples: [x]
enum: [x]`, "")
	require.Equal(t, EmptyDomainID, empty)
}

// TestCompilerDoesNotUseEnumBranchExamplesAsPatternProof verifies allOf branch-local trust.
func TestCompilerDoesNotUseEnumBranchExamplesAsPatternProof(t *testing.T) {
	t.Parallel()

	source := parseSchemaSource(t, `allOf:
  - enum: [bad]
    x-valid-examples: [bad]
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

	registry := NewDomainRegistry()
	first := registry.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{one}))
	second := registry.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{one, oneDecimal}))
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
