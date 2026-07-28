package suite

import (
	"strings"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/stretchr/testify/require"
)

// compose only at their exact Schema Object occurrence, including recursive and referenced uses.
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
	require.Len(t, integerDomain.Enum.Values, 1)
	require.True(t, enumContains(integerDomain.Enum, mustJSONValue(t, "1")))
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
func TestCompilerAcceptsDiscriminatorAsAnInertHint(t *testing.T) {
	t.Parallel()

	for _, discriminator := range []string{
		"discriminator: {propertyName: kind}",
		`discriminator: {propertyName: ""}`,
		"discriminator:\n  propertyName: kind\n  mapping: {cat: 'missing.yaml#/Cat'}",
	} {
		t.Run(discriminator, func(t *testing.T) {
			t.Parallel()

			_, _ = compileSchemaYAML(t, "type: object\n"+discriminator, "")
		})
	}

	compiler, rootID := compileSchemaYAML(t, `
allOf:
  - {type: string, minLength: 2}
discriminator: {propertyName: kind}
`, "")
	root, ok := compiler.Domains.Domain(rootID)
	require.True(t, ok)
	require.Equal(t, 2, root.String.MinLength)
}

// TestCompilerRejectsMalformedDiscriminatorAtExactPointers verifies independent
// generated-suite shape checking agrees with runtime compilation.
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

// TestCompileSuiteFiltersLocalEnumMembersThroughOrdinaryConstraints verifies finite candidate filtering.
func TestCompileSuiteFiltersLocalEnumMembersThroughOrdinaryConstraints(t *testing.T) {
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
			values: []string{`1`},
		},
		{
			name: "length",
			schema: `type: string
minLength: 3
enum: [x, long]`,
			values: []string{`"long"`},
		},
		{
			name: "nested property",
			schema: `type: object
properties: {value: {enum: [1]}}
enum: [{value: 1}, {value: 2}]`,
			values: []string{`{"value":1}`},
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
	_, err := NewCompiler(source).CompileSuite()
	require.ErrorContains(t, err, "unconstructible")
}

// TestCompilerValidatesAdjustedOpenAPIFields verifies malformed and required adjusted fields.
func TestCompilerValidatesAdjustedOpenAPIFields(t *testing.T) {
	t.Parallel()

	for _, schema := range []string{
		"type: array",
		"type: boolean\nformat: 7",
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

// TestCompilerRejectsMalformedReadOnlyAndWriteOnlyAtEverySchemaShape verifies recursive shape checking.
func TestCompilerRejectsMalformedReadOnlyAndWriteOnlyAtEverySchemaShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		extra   string
		pointer string
		keyword string
	}{
		{
			name: "root readOnly", schema: "readOnly: yes",
			pointer: "#/paths/~1things/post/requestBody/content/application~1json/schema", keyword: "readOnly",
		},
		{
			name: "property writeOnly", schema: "properties:\n  value: {writeOnly: null}",
			pointer: "#/paths/~1things/post/requestBody/content/application~1json/schema/properties/value",
			keyword: "writeOnly",
		},
		{
			name: "resolved reference readOnly", schema: "$ref: '#/components/schemas/Value'",
			extra:   "components:\n  schemas:\n    Value: {readOnly: []}",
			pointer: "#/components/schemas/Value", keyword: "readOnly",
		},
		{
			name: "allOf writeOnly", schema: "allOf:\n  - {writeOnly: 1}",
			pointer: "#/paths/~1things/post/requestBody/content/application~1json/schema/allOf/0",
			keyword: "writeOnly",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewCompiler(parseSchemaSource(t, test.schema, test.extra, "create")).Compile()
			require.Error(t, err)

			var compileError *Error
			require.ErrorAs(t, err, &compileError)
			require.Equal(t, "compile", compileError.Phase)
			require.Equal(t, "malformed", compileError.Code)
			require.Equal(t, test.pointer, compileError.Pointer)
			require.ErrorContains(t, err, test.keyword+" must be a boolean")
		})
	}
}

// TestCompilerIgnoresReadOnlyAndWriteOnlyOutsidePropertyContext verifies direction is property-only.
func TestCompilerIgnoresReadOnlyAndWriteOnlyOutsidePropertyContext(t *testing.T) {
	t.Parallel()

	for _, schema := range []string{
		"readOnly: true\nwriteOnly: true",
		"allOf:\n  - {readOnly: true, writeOnly: true}",
	} {
		t.Run(schema, func(t *testing.T) {
			t.Parallel()

			_, err := NewCompiler(parseSchemaSource(t, schema, "", "create")).Compile()
			require.NoError(t, err)
		})
	}
}

// TestCompilerAppliesReadOnlyRequirednessForRequests verifies required read-only properties remain optional.
func TestCompilerAppliesReadOnlyRequirednessForRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		property string
		extra    string
	}{
		{name: "direct", property: "{type: string, minLength: 2, readOnly: true}"},
		{
			name: "resolved reference", property: "{$ref: '#/components/schemas/Identifier'}",
			extra: "components:\n  schemas:\n    Identifier: {type: string, minLength: 2, readOnly: true}",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			compiler, id := compileSchemaYAML(t, `type: object
required: [id, name]
properties:
  id: `+test.property+`
  name: {type: string}`, test.extra)
			domain := mustDomain(t, compiler.Domains, id)
			properties := propertiesByName(domain.Object.Properties)
			require.False(t, properties["id"].Required)
			require.True(t, properties["name"].Required)

			identifier := mustDomain(t, compiler.Domains, properties["id"].Values)
			require.Equal(t, 2, identifier.String.MinLength)
		})
	}
}

// TestCompilerKeepsWriteOnlyRequiredAndAllOfDirectionBranchLocal verifies non-transforming request cases.
func TestCompilerKeepsWriteOnlyRequiredAndAllOfDirectionBranchLocal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		property string
	}{
		{name: "writeOnly", property: "{type: string, writeOnly: true}"},
		{
			name: "allOf",
			property: `
allOf:
  - {type: string, readOnly: true}
  - {minLength: 2, writeOnly: true}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			compiler, id := compileSchemaYAML(t, `type: object
required: [value]
properties:
  value:
`+indent(test.property, 4), "")
			domain := mustDomain(t, compiler.Domains, id)
			require.True(t, propertiesByName(domain.Object.Properties)["value"].Required)
		})
	}
}

// TestCompilerRejectsReadOnlyAndWriteOnlyTogetherOnRequestProperties verifies resolved property context.
func TestCompilerRejectsReadOnlyAndWriteOnlyTogetherOnRequestProperties(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		extra   string
		pointer string
	}{
		{
			name:    "direct",
			schema:  "properties:\n  value: {readOnly: true, writeOnly: true}",
			pointer: "#/paths/~1things/post/requestBody/content/application~1json/schema/properties/value",
		},
		{
			name:    "resolved reference",
			schema:  "properties:\n  value: {$ref: '#/components/schemas/Value'}",
			extra:   "components:\n  schemas:\n    Value: {readOnly: true, writeOnly: true}",
			pointer: "#/components/schemas/Value",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewCompiler(parseSchemaSource(t, test.schema, test.extra, "create")).Compile()
			require.Error(t, err)

			var compileError *Error
			require.ErrorAs(t, err, &compileError)
			require.Equal(t, "compile", compileError.Phase)
			require.Equal(t, "malformed", compileError.Code)
			require.Equal(t, test.pointer, compileError.Pointer)
			require.Equal(t, "readOnly", compileError.Keyword)
			require.ErrorContains(t, err, "readOnly and writeOnly must not both be true")
		})
	}
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
	sources, err := oas.Parse([]byte(spec))
	require.NoError(t, err)

	source, ok := sources[operation]
	require.True(t, ok, "operationId %q is missing", operation)

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
