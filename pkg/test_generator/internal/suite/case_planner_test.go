package suite

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"decode_and_validate_generator/pkg/internal/jsonvalue"
	"decode_and_validate_generator/pkg/internal/oas"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestCasePlannerBuildsCanonicalSemanticPartitions verifies distinct accepted and rejected partitions.
func TestCasePlannerBuildsCanonicalSemanticPartitions(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: string
minLength: 2
maxLength: 4`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)
	require.Equal(t, compiler.Domains, compiled.Domains)
	require.NotEmpty(t, compiled.Cases)

	seen := make(map[string]struct{})
	accepted := 0
	rejected := 0

	for _, plannedCase := range compiled.Cases {
		_, ok := compiled.Domains.Domain(plannedCase.Values)
		require.True(t, ok)
		require.NotEqual(t, EmptyDomainID, plannedCase.Values)

		key := strings.Join([]string{
			plannedCase.Name,
			plannedCase.Source.Pointer,
			plannedCase.Source.Keyword,
		}, "\x00")
		require.NotContains(t, seen, key)
		seen[key] = struct{}{}

		if plannedCase.Expect == ExpectAccepted {
			accepted++
		} else {
			rejected++
		}
	}

	require.Greater(t, accepted, 1)
	require.Greater(t, rejected, 1)
	require.Contains(t, caseNames(compiled.Cases), "valid aggregate")
	require.Contains(t, caseNames(compiled.Cases), "valid string minimum length")
	require.Contains(t, caseNames(compiled.Cases), "valid string maximum length")
}

// TestCompileSuitePlansEveryExactOracleValue verifies explicit evidence remains
// observable as its own linked case for every JSON kind and role.
func TestCompileSuitePlansEveryExactOracleValue(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `pattern: '^x$'
x-valid-examples: [null, false, 1, x, [1], {a: 1}]
x-invalid-examples: [true, 2, y, [2], {a: 2}]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	want := map[string]ExpectedResult{
		`null`: ExpectAccepted, `false`: ExpectAccepted, `1`: ExpectAccepted,
		`"x"`: ExpectAccepted, `[1]`: ExpectAccepted, `{"a":1}`: ExpectAccepted,
		`true`: ExpectRejected, `2`: ExpectRejected, `"y"`: ExpectRejected,
		`[2]`: ExpectRejected, `{"a":2}`: ExpectRejected,
	}
	found := make(map[string]ExpectedResult, len(want))

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Source.Keyword != "x-valid-examples" &&
			plannedCase.Source.Keyword != "x-invalid-examples" {
			continue
		}

		domain := mustDomain(t, compiled.Domains, plannedCase.Values)
		require.NotNil(t, domain.Enum)
		require.Len(t, domain.Enum.Values, 1)
		body, marshalErr := domain.Enum.Values[0].MarshalJSON()
		require.NoError(t, marshalErr)

		found[string(body)] = plannedCase.Expect
		require.NotNil(t, plannedCase.Generator)

		rapid.Check(t, func(rt *rapid.T) {
			value := plannedCase.Generator.Draw(rt, "value")
			require.True(rt, domain.Enum.Values[0].Equal(value))
		})
	}

	require.Equal(t, want, found)
}

// TestCompileSuiteKeepsEnumMembersAsExactAcceptedCases verifies enum evidence
// retains its own source attribution and does not collapse into an aggregate.
func TestCompileSuiteKeepsEnumMembersAsExactAcceptedCases(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `enum: [true, 1, x]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	want := []jsonvalue.Value{
		jsonvalue.Bool(true),
		mustJSONValue(t, `1`),
		jsonvalue.String("x"),
	}
	found := make([]jsonvalue.Value, 0, len(want))

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect != ExpectAccepted || plannedCase.Source.Keyword != "enum" {
			continue
		}

		domain := mustDomain(t, compiled.Domains, plannedCase.Values)
		require.NotNil(t, domain.Enum)
		require.Len(t, domain.Enum.Values, 1)
		found = append(found, domain.Enum.Values[0])

		rapid.Check(t, func(rt *rapid.T) {
			value := plannedCase.Generator.Draw(rt, "value")
			require.True(rt, domain.Enum.Values[0].Equal(value))
		})
	}

	require.Equal(t, want, found)
}

// TestCompileSuiteDoesNotReplaceSingletonEvidenceWithTheOccurrenceOracle
// covers equal Domain identities for a local singleton enum and its evidence.
func TestCompileSuiteDoesNotReplaceSingletonEvidenceWithTheOccurrenceOracle(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: string
pattern: '^x$'
enum: [x]
x-valid-examples: [x]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)
	assertExactEvidenceBody(t, compiled.Cases, "x-valid-examples", ExpectAccepted, `"x"`)
}

// TestCompileSuiteEvidenceMarkerIsOccurrenceLocal verifies targeting one child
// does not suppress the oracle on an equal-Domain sibling occurrence.
func TestCompileSuiteEvidenceMarkerIsOccurrenceLocal(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: object
required: [left, right]
maxProperties: 2
properties:
  left: {type: string, pattern: '^[a-z]+$', x-valid-examples: [left]}
  right: {type: string, pattern: '^[a-z]+$', x-valid-examples: [right]}
additionalProperties: false`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Source.Keyword != "x-valid-examples" ||
			!strings.HasSuffix(plannedCase.Source.Pointer, "/properties/left") {
			continue
		}

		rapid.Check(t, func(rt *rapid.T) {
			value := plannedCase.Generator.Draw(rt, "value")
			body, marshalErr := value.MarshalJSON()
			require.NoError(rt, marshalErr)
			require.JSONEq(rt, `{"left":"left","right":"right"}`, string(body))
		})

		return
	}

	require.Fail(t, "left evidence case not found")
}

// TestCompileSuiteUsesWrongKindTrustedValueAsTheAggregate verifies a declared
// oracle remains executable even when its JSON kind violates local type.
func TestCompileSuiteUsesWrongKindTrustedValueAsTheAggregate(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: string
pattern: '^x$'
x-valid-examples: [1]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)
	assertExactEvidenceBody(t, compiled.Cases, "x-valid-examples", ExpectAccepted, "1")
}

// TestCompileSuiteLiftsExactEvidenceBodies verifies child evidence remains an
// exact whole-request case through each structural and reference location.
func TestCompileSuiteLiftsExactEvidenceBodies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		extra   string
		valid   string
		invalid string
	}{
		{
			name: "property",
			schema: `type: object
required: [value]
maxProperties: 1
properties:
  value: {pattern: '^x$', x-valid-examples: [1], x-invalid-examples: [2]}
additionalProperties: false`,
			valid: `{"value":1}`, invalid: `{"value":2}`,
		},
		{
			name: "items",
			schema: `type: array
minItems: 1
maxItems: 1
items: {pattern: '^x$', x-valid-examples: [null], x-invalid-examples: [false]}`,
			valid: `[null]`, invalid: `[false]`,
		},
		{
			name: "additional properties",
			schema: `type: object
minProperties: 1
maxProperties: 1
additionalProperties: {pattern: '^x$', x-valid-examples: [1], x-invalid-examples: [2]}`,
			valid: `{"additional":1}`, invalid: `{"additional":2}`,
		},
		{
			name: "reference",
			schema: `type: object
required: [value]
maxProperties: 1
properties: {value: {$ref: '#/components/schemas/Evidence'}}
additionalProperties: false`,
			extra: `
components:
  schemas:
    Evidence: {pattern: '^x$', x-valid-examples: [1], x-invalid-examples: [2]}
`,
			valid: `{"value":1}`, invalid: `{"value":2}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			compiler := NewCompiler(parseSchemaSource(t, tt.schema, tt.extra, "create"))
			compiled, err := compiler.CompileSuite()
			require.NoError(t, err)
			assertExactEvidenceBody(t, compiled.Cases, "x-valid-examples", ExpectAccepted, tt.valid)
			assertExactEvidenceBody(t, compiled.Cases, "x-invalid-examples", ExpectRejected, tt.invalid)
		})
	}
}

// TestCompileSuiteKeepsWholeOccurrenceInvalidEvidenceSeparateFromOpaqueRules
// verifies invalid evidence gets one exact case and never claims atomic blame.
func TestCompileSuiteKeepsWholeOccurrenceInvalidEvidenceSeparateFromOpaqueRules(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: string
pattern: '^x$'
format: email
x-valid-examples: [x]
x-invalid-examples: [wrong]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)
	assertExactEvidenceBody(t, compiled.Cases, "x-invalid-examples", ExpectRejected, `"wrong"`)

	for _, keyword := range []string{"pattern", "format"} {
		constraint := constraintPlanAt(t, compiled.Constraints, ConstraintSource{
			Pointer: compiler.Source.RequestSchema.Pointer, Keyword: keyword,
		})
		require.Equal(t, ObligationUnconstructible, constraint.Outcome)
		require.NotEmpty(t, constraint.Reason)
	}
}

// TestCompileSuiteHandlesEmptyEvidenceArrays verifies empty declarations retain
// their declared-set meaning without blocking constructible non-string kinds or
// fabricating exact cases.
func TestCompileSuiteHandlesEmptyEvidenceArrays(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `pattern: '^x$'
x-valid-examples: []`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	accepted := 0

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Source.Keyword == "x-valid-examples" {
			require.Fail(t, "empty evidence declaration produced an exact case")
		}

		if plannedCase.Expect != ExpectAccepted {
			continue
		}

		accepted++

		rapid.Check(t, func(rt *rapid.T) {
			value := plannedCase.Generator.Draw(rt, "value")
			require.NotEqual(rt, jsonvalue.KindString, value.Kind)
		})
	}

	require.Positive(t, accepted)

	compiler = NewCompiler(parseSchemaSource(t, `pattern: '^x$'
enum: [x]
x-valid-examples: []
x-invalid-examples: []`, "", "create"))
	compiled, err = compiler.CompileSuite()
	require.NoError(t, err)
	require.NotEmpty(t, compiled.Cases)

	for _, plannedCase := range compiled.Cases {
		require.NotEqual(t, "x-invalid-examples", plannedCase.Source.Keyword)
	}
}

// TestCompileSuitePlanIsDeterministic compares every ordered observable plan
// field and obligation outcome across identical compiles.
func TestCompileSuitePlanIsDeterministic(t *testing.T) {
	t.Parallel()

	schema := `type: object
required: [amount]
properties:
  label: {pattern: '^x$', x-valid-examples: [x], x-invalid-examples: [wrong]}
  amount:
    allOf:
      - {type: number, multipleOf: 0.2}
      - {multipleOf: 0.3}
additionalProperties: false`
	compile := func() (*CompiledSuite, error) {
		compiler := NewCompiler(parseSchemaSource(t, schema, "", "create"))

		return compiler.CompileSuite()
	}

	first, err := compile()
	require.NoError(t, err)
	second, err := compile()
	require.NoError(t, err)
	require.Equal(t, first.Constraints, second.Constraints)
	require.Equal(t, observableCases(first.Cases), observableCases(second.Cases))
}

// observableCases drops only runtime generator identity from an ordered plan.
func observableCases(cases []CasePlan) []CasePlan {
	result := append([]CasePlan(nil), cases...)
	for index := range result {
		result[index].Generator = nil
		result[index].evidenceUse = nil
	}

	return result
}

// assertExactEvidenceBody requires every draw from the named evidence case to
// be the exact canonical request body.
func assertExactEvidenceBody(
	t *testing.T,
	cases []CasePlan,
	keyword string,
	expect ExpectedResult,
	want string,
) {
	t.Helper()

	for _, plannedCase := range cases {
		if plannedCase.Source.Keyword != keyword || plannedCase.Expect != expect {
			continue
		}

		rapid.Check(t, func(rt *rapid.T) {
			value := plannedCase.Generator.Draw(rt, "value")
			body, err := value.MarshalJSON()
			require.NoError(rt, err)
			require.Equal(rt, want, string(body))
		})

		return
	}

	require.Fail(t, "exact evidence case not found", keyword)
}

// TestCasePlannerRecordsAllOfDominanceAndSourceProvenance verifies allOf obligation provenance.
func TestCasePlannerRecordsAllOfDominanceAndSourceProvenance(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: string
maxLength: 5
allOf:
  - minLength: 2
  - minLength: 3`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	root := compiler.Source.RequestSchema.Pointer
	weaker := constraintPlanAt(t, compiled.Constraints, ConstraintSource{
		Pointer: root + "/allOf/0", Keyword: "minLength",
	})
	stronger := constraintPlanAt(t, compiled.Constraints, ConstraintSource{
		Pointer: root + "/allOf/1", Keyword: "minLength",
	})
	require.Equal(t, ObligationDominated, weaker.Outcome)
	require.Equal(t, ObligationPlanned, stronger.Outcome)

	foundStrongFailure := false

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect == ExpectRejected && plannedCase.Source == stronger.Source {
			foundStrongFailure = true

			require.Contains(t, plannedCase.Name, root+"/allOf/1")
		}
	}

	require.True(t, foundStrongFailure)
}

// TestCasePlannerIsolatesBoundedIntegerMultipleAndEnumFailures verifies bounded dynamic failures.
func TestCasePlannerIsolatesBoundedIntegerMultipleAndEnumFailures(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"type":       "type: integer\nminimum: 10\nmaximum: 20",
		"multipleOf": "type: integer\nminimum: 10\nmaximum: 20\nmultipleOf: 3",
		"enum":       "type: integer\nminimum: 10\nmaximum: 20\nenum: [12, 15, 18]",
	}
	for keyword, schema := range tests {
		compiler := NewCompiler(parseSchemaSource(t, schema, "", "create"))
		compiled, err := compiler.CompileSuite()
		require.NoError(t, err)

		found := false

		for _, plannedCase := range compiled.Cases {
			if plannedCase.Expect == ExpectRejected && plannedCase.Source.Keyword == keyword {
				found = true
			}
		}

		require.True(t, found, keyword)
	}
}

// TestCompileSuiteGeneratesNonRationalIntegerEnumTypeFailure verifies an exact
// finite enum member remains a type witness when its exponent is too large to
// materialize as a rational number.
func TestCompileSuiteGeneratesNonRationalIntegerEnumTypeFailure(t *testing.T) {
	t.Parallel()

	const lexeme = "1e-100001"

	schemaSource := oas.Source{RequestSchema: oas.LocatedSchema{
		Raw:     json.RawMessage(`{"allOf":[{"type":"integer"},{"enum":[0,1e-100001]}]}`),
		Pointer: "#/schema",
	}}
	compiler := NewCompiler(schemaSource)
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	source := ConstraintSource{
		Pointer: compiler.Source.RequestSchema.Pointer + "/allOf/0",
		Keyword: "type",
	}
	require.Equal(t, ObligationPlanned, constraintPlanAt(t, compiled.Constraints, source).Outcome)

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect != ExpectRejected || plannedCase.Source != source {
			continue
		}

		rapid.Check(t, func(rt *rapid.T) {
			value := plannedCase.Generator.Draw(rt, "value")
			body, marshalErr := value.MarshalJSON()
			require.NoError(rt, marshalErr)
			require.Equal(rt, lexeme, string(body))
		})

		return
	}

	require.Fail(t, "exact non-rational integer type witness not found")
}

// TestCasePlannerNestedAllOfUsesEachLocalConstraint verifies nested allOf source selection.
func TestCasePlannerNestedAllOfUsesEachLocalConstraint(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: string
allOf:
  - minLength: 2
    allOf:
      - minLength: 3`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	root := compiler.Source.RequestSchema.Pointer
	outer := constraintPlanAt(t, compiled.Constraints, ConstraintSource{
		Pointer: root + "/allOf/0", Keyword: "minLength",
	})
	inner := constraintPlanAt(t, compiled.Constraints, ConstraintSource{
		Pointer: root + "/allOf/0/allOf/0", Keyword: "minLength",
	})
	require.Equal(t, ObligationDominated, outer.Outcome)
	require.Equal(t, ObligationPlanned, inner.Outcome)
}

// TestCasePlannerAdditionalFailureKeepsDeclaredRequiredProperties verifies additional failures retain requirements.
func TestCasePlannerAdditionalFailureKeepsDeclaredRequiredProperties(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: object
required: [id]
properties:
  id: {type: string}
additionalProperties: false`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	found := false

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect != ExpectRejected || plannedCase.Source.Keyword != "additionalProperties" {
			continue
		}

		found = true
		domain := mustDomain(t, compiled.Domains, plannedCase.Values)
		properties := propertiesByName(domain.Object.Properties)
		root := mustDomain(t, compiled.Domains, compiled.Root)
		require.True(t, properties["id"].Required)
		require.Equal(t, propertiesByName(root.Object.Properties)["id"].Values, properties["id"].Values)
		require.True(t, properties["additional"].Required)
	}

	require.True(t, found)
}

// TestCasePlannerLiftsChildPartitionsAdditivelyByDomainReference verifies independent property lifting.
func TestCasePlannerLiftsChildPartitionsAdditivelyByDomainReference(t *testing.T) {
	t.Parallel()

	oneCompiler := NewCompiler(parseSchemaSource(t, `type: object
required: [left, sibling]
properties:
  left: {type: string, minLength: 2, maxLength: 4}
  sibling: {type: boolean}
additionalProperties: false`, "", "create"))
	one, err := oneCompiler.CompileSuite()
	require.NoError(t, err)

	twoCompiler := NewCompiler(parseSchemaSource(t, `type: object
required: [left, right, sibling]
properties:
  left: {type: string, minLength: 2, maxLength: 4}
  right: {type: string, minLength: 2, maxLength: 4}
  sibling: {type: boolean}
additionalProperties: false`, "", "create"))
	two, err := twoCompiler.CompileSuite()
	require.NoError(t, err)

	oneLifted := liftedPropertyCaseCount(one.Cases)
	twoLifted := liftedPropertyCaseCount(two.Cases)

	require.Greater(t, oneLifted, 0)
	require.Equal(t, oneLifted*2, twoLifted)

	root := mustDomain(t, two.Domains, two.Root)
	rootProperties := propertiesByName(root.Object.Properties)

	for _, plannedCase := range two.Cases {
		if !strings.Contains(plannedCase.Name, "property left /") {
			continue
		}

		partition := mustDomain(t, two.Domains, plannedCase.Values)
		properties := propertiesByName(partition.Object.Properties)
		require.Equal(t, rootProperties["right"].Values, properties["right"].Values)
		require.Equal(t, rootProperties["sibling"].Values, properties["sibling"].Values)
	}
}

// TestCompileSuiteKeepsLiftedChildMetadataOccurrenceLocal verifies equivalent child Domains do not share provenance.
func TestCompileSuiteKeepsLiftedChildMetadataOccurrenceLocal(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		targetSchema  string
		extra         string
		targetBranch  int
		sourcePointer string
	}{
		"direct target first": {
			targetSchema: "type: string\n        minLength: 2",
			targetBranch: 0,
		},
		"direct target second": {
			targetSchema: "type: string\n        minLength: 2",
			targetBranch: 1,
		},
		"referenced target first": {
			targetSchema:  "$ref: '#/components/schemas/Text'",
			targetBranch:  0,
			sourcePointer: "#/components/schemas/Text",
			extra: `
components:
  schemas:
    Text:
      type: string
      minLength: 2
`,
		},
		"referenced target second": {
			targetSchema:  "$ref: '#/components/schemas/Text'",
			targetBranch:  1,
			sourcePointer: "#/components/schemas/Text",
			extra: `
components:
  schemas:
    Text:
      type: string
      minLength: 2
`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			branches := []string{
				"type: object\n    properties:\n      target:\n        " + tt.targetSchema,
				"type: object",
			}
			if tt.targetBranch == 1 {
				branches[0], branches[1] = branches[1], branches[0]
			}

			schema := `type: object
properties:
  borrowed:
    type: string
    minLength: 2
allOf:
  - ` + branches[0] + `
  - ` + branches[1]

			firstCompiler := NewCompiler(parseSchemaSource(t, schema, tt.extra, "create"))
			first, err := firstCompiler.CompileSuite()
			require.NoError(t, err)

			secondCompiler := NewCompiler(parseSchemaSource(t, schema, tt.extra, "create"))
			second, err := secondCompiler.CompileSuite()
			require.NoError(t, err)
			require.Equal(t, caseNames(first.Cases), caseNames(second.Cases))

			root := firstCompiler.Source.RequestSchema.Pointer
			targetPointer := root + fmt.Sprintf("/allOf/%d/properties/target", tt.targetBranch)

			expectedSource := tt.sourcePointer
			if expectedSource == "" {
				expectedSource = targetPointer
			}

			found := false

			for _, plannedCase := range first.Cases {
				if !strings.Contains(plannedCase.Name, "property target /") ||
					plannedCase.Source.Keyword != "minLength" {
					continue
				}

				found = true

				require.Equal(t, expectedSource, plannedCase.Source.Pointer)
				require.NotContains(t, plannedCase.Name, root+"/properties/borrowed")
			}

			require.True(t, found)
		})
	}
}

// TestCasePlannerLiftsArrayValidAndInvalidChildPartitions verifies array child partition lifting.
func TestCasePlannerLiftsArrayValidAndInvalidChildPartitions(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: array
minItems: 1
maxItems: 3
items:
  type: string
  minLength: 2`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	valid := false
	invalid := false

	for _, plannedCase := range compiled.Cases {
		if strings.Contains(plannedCase.Name, "valid array item /") {
			valid = true
		}

		if strings.Contains(plannedCase.Name, "invalid array item /") {
			invalid = true
			partition := mustDomain(t, compiled.Domains, plannedCase.Values)
			require.NotEqual(t, mustDomain(t, compiled.Domains, compiled.Root).Array.Items, partition.Array.Items)
		}
	}

	require.True(t, valid)
	require.True(t, invalid)
}

// TestCasePlannerPreservesOriginalEnumForSiblingIsolation verifies enum filtering does not erase failures.
func TestCasePlannerPreservesOriginalEnumForSiblingIsolation(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: integer
minimum: 2
enum: [1, 2]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	minimum := constraintPlanAt(t, compiled.Constraints, ConstraintSource{
		Pointer: compiler.Source.RequestSchema.Pointer, Keyword: "minimum",
	})
	require.Equal(t, ObligationPlanned, minimum.Outcome)

	foundOne := false

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect != ExpectRejected || plannedCase.Source != minimum.Source {
			continue
		}

		domain := mustDomain(t, compiled.Domains, plannedCase.Values)
		for _, value := range domain.Enum.Values {
			foundOne = foundOne || value.Kind == jsonvalue.KindNumber && value.Number.Lexeme == "1"
		}
	}

	require.True(t, foundOne)
}

// TestCasePlannerBuildsStringEnumOutsiderBeyondAlphabet verifies fixed-length
// enum search continues with distinct strings after exhausting a through z.
func TestCasePlannerBuildsStringEnumOutsiderBeyondAlphabet(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: string
minLength: 1
maxLength: 1
enum: [a, b, c, d, e, f, g, h, i, j, k, l, m, n, o, p, q, r, s, t, u, v, w, x, y, z]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	found, err := hasExactRejectedCase(compiled, "enum", `"0"`)
	require.NoError(t, err)
	require.True(t, found, exactRejectedBodies(t, compiled, "enum"))
}

// TestCasePlannerBuildsObjectEnumOutsider verifies object-only enums receive an invalid partition.
func TestCasePlannerBuildsObjectEnumOutsider(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: object
enum: [{a: 1}]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect != ExpectRejected || plannedCase.Source.Keyword != "enum" {
			continue
		}

		domain := mustDomain(t, compiled.Domains, plannedCase.Values)
		if domain.Enum != nil && len(domain.Enum.Values) == 1 && domain.Enum.Values[0].Kind == jsonvalue.KindObject {
			return
		}
	}

	require.Fail(t, "object enum outsider not found")
}

// TestCasePlannerBuildsContextShapedContainerEnumOutsiders verifies outsiders
// satisfy sibling item, count, required, closed-object, and value policies.
func TestCasePlannerBuildsContextShapedContainerEnumOutsiders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		exact  string
	}{
		{
			name: "array",
			schema: `type: array
minItems: 1
maxItems: 1
items: {type: integer, minimum: 0, maximum: 1}
enum: [[0]]`,
			exact: `[1]`,
		},
		{
			name: "object",
			schema: `type: object
required: [value]
properties:
  value: {type: integer, minimum: 0, maximum: 1}
additionalProperties: false
enum: [{value: 0}]`,
			exact: `{"value":1}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			compiler := NewCompiler(parseSchemaSource(t, tt.schema, "", "create"))
			compiled, err := compiler.CompileSuite()
			require.NoError(t, err)

			found, findErr := hasExactRejectedCase(compiled, "enum", tt.exact)
			require.NoError(t, findErr)
			require.True(t, found)
		})
	}
}

// TestCompileSuiteVariesEveryArrayItemForEnumOutsiders verifies fixed-size
// arrays can construct an outsider that differs after the first item.
func TestCompileSuiteVariesEveryArrayItemForEnumOutsiders(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: array
minItems: 2
maxItems: 2
items: {type: integer, enum: [0, 1]}
enum: [[0, 0], [1, 0]]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	found, err := hasExactRejectedCase(compiled, "enum", `[0,1]`)
	require.NoError(t, err)
	require.True(t, found, exactRejectedBodies(t, compiled, "enum"))
}

// TestCompileSuiteCombinesArrayItemsForEnumOutsiders verifies contextual
// search reaches combinations after every single-position variant is an enum.
func TestCompileSuiteCombinesArrayItemsForEnumOutsiders(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: array
minItems: 2
maxItems: 2
items: {type: integer, enum: [0, 1]}
enum: [[0, 0], [1, 0], [0, 1]]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	found, err := hasExactRejectedCase(compiled, "enum", `[1,1]`)
	require.NoError(t, err)
	require.True(t, found, exactRejectedBodies(t, compiled, "enum"))
}

// TestCompileSuiteVariesOptionalPropertyValuesForEnumOutsiders verifies an
// optional property's later values remain available to contextual enum search.
func TestCompileSuiteVariesOptionalPropertyValuesForEnumOutsiders(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: object
maxProperties: 1
properties:
  value: {type: integer, enum: [0, 1]}
additionalProperties: false
enum: [{}, {value: 0}]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	found, err := hasExactRejectedCase(compiled, "enum", `{"value":1}`)
	require.NoError(t, err)
	require.True(t, found, exactRejectedBodies(t, compiled, "enum"))
}

// TestCompileSuiteCombinesOptionalPropertiesForEnumOutsiders verifies
// contextual search reaches a multi-property shape after every singleton.
func TestCompileSuiteCombinesOptionalPropertiesForEnumOutsiders(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: object
maxProperties: 2
properties:
  a: {type: integer, enum: [0]}
  b: {type: integer, enum: [0]}
additionalProperties: false
enum: [{}, {a: 0}, {b: 0}]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	found, err := hasExactRejectedCase(compiled, "enum", `{"a":0,"b":0}`)
	require.NoError(t, err)
	require.True(t, found, exactRejectedBodies(t, compiled, "enum"))
}

// TestCasePlannerKeepsEmptyArrayContextWhenItemsAreUnconstructible verifies an
// empty array remains a usable enum outsider without guessing an item value.
func TestCasePlannerKeepsEmptyArrayContextWhenItemsAreUnconstructible(t *testing.T) {
	t.Parallel()

	registry := NewDomainRegistry()
	item := singleKindDomain(jsonvalue.KindString)
	item.String = StringConstraints{State: KindRestricted, Patterns: []string{"^x$"}}
	itemID := registry.FindOrAddEquivalentDomain(item)
	maximum := 1
	planner := &CasePlanner{Domains: registry}

	values, err := planner.contextArrays(ArrayConstraints{
		State: KindRestricted, Items: itemID, MaxItems: &maximum,
	}, 2, make(map[DomainID]bool))
	require.NoError(t, err)
	require.Equal(t, []jsonvalue.Value{jsonvalue.Array(nil)}, values)
}

// TestCasePlannerBoundsContextContainerMaterialization verifies bounded enum
// search does not allocate or overflow on hostile container minima.
func TestCasePlannerBoundsContextContainerMaterialization(t *testing.T) {
	t.Parallel()

	planner := &CasePlanner{Domains: NewDomainRegistry()}
	arrays, err := planner.contextArrays(ArrayConstraints{
		State: KindRestricted, Items: AnyJSONDomainID, MinItems: int(^uint(0) >> 1),
	}, 2, make(map[DomainID]bool))
	require.NoError(t, err)
	require.Empty(t, arrays)

	objects, err := planner.contextObjects(ObjectConstraints{
		State: KindRestricted, MinProps: int(^uint(0) >> 1),
	}, 2, make(map[DomainID]bool))
	require.NoError(t, err)
	require.Empty(t, objects)
}

// TestCasePlannerContextAdditionalNamesSkipDeclaredNames verifies synthetic
// enum outsiders cannot accidentally select a declared property policy.
func TestCasePlannerContextAdditionalNamesSkipDeclaredNames(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: object
maxProperties: 1
properties:
  additional: {type: string}
  additional1: {type: string}
additionalProperties: {type: integer, minimum: 0, maximum: 0}
enum: [{}, {additional: a}, {additional1: a}]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	found, err := hasExactRejectedCase(compiled, "enum", `{"additional2":0}`)
	require.NoError(t, err)
	require.True(t, found, exactRejectedBodies(t, compiled, "enum"))
}

// TestCasePlannerContextObjectsSkipUnconstructibleOptionalProperties verifies
// optional opaque children neither block omission nor stop later minimum fills.
func TestCasePlannerContextObjectsSkipUnconstructibleOptionalProperties(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		minimum   int
		maximum   int
		enumValue string
		outsider  string
	}{
		{
			name:      "opaque optional omitted",
			maximum:   1,
			enumValue: `{}`,
			outsider:  `{"bValue":0}`,
		},
		{
			name:      "later optional meets minimum",
			minimum:   1,
			maximum:   1,
			enumValue: `{"bValue":1}`,
			outsider:  `{"bValue":0}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			registry := NewDomainRegistry()
			opaque := singleKindDomain(jsonvalue.KindString)
			opaque.String = StringConstraints{State: KindRestricted, Patterns: []string{"^x$"}}
			opaqueID := registry.FindOrAddEquivalentDomain(opaque)
			numbersID := registry.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{
				mustJSONValue(t, `0`),
				mustJSONValue(t, `1`),
			}))
			planner := &CasePlanner{Domains: registry}

			context := singleKindDomain(jsonvalue.KindObject)
			context.Object = ObjectConstraints{
				State: KindRestricted,
				Properties: []NamedProperty{
					{Name: "aOpaque", State: PropertyAllowed, Values: opaqueID},
					{Name: "bValue", State: PropertyAllowed, Values: numbersID},
				},
				Additional: AdditionalProperties{Values: EmptyDomainID},
				MinProps:   tt.minimum,
				MaxProps:   new(tt.maximum),
			}
			pass := finiteDomain([]jsonvalue.Value{mustJSONValue(t, tt.enumValue)})

			failures, err := planner.enumContextFailures(pass, context)
			require.NoError(t, err)
			require.Len(t, failures, 1)
			failure := mustDomain(t, registry, failures[0])
			require.NotNil(t, failure.Enum)
			require.Equal(t, []jsonvalue.Value{mustJSONValue(t, tt.outsider)}, failure.Enum.Values)
		})
	}
}

// TestCompileSuiteUsesChildOraclesForContainerEnumOutsiders verifies contextual
// traversal constructs opaque descendants from their exact occurrence evidence.
func TestCompileSuiteUsesChildOraclesForContainerEnumOutsiders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		exact  string
	}{
		{
			name: "array items",
			schema: `type: array
minItems: 1
maxItems: 1
items: {type: string, pattern: '^x|y$', x-valid-examples: [x, y]}
enum: [[x]]`,
			exact: `["y"]`,
		},
		{
			name: "required object property",
			schema: `type: object
required: [value]
maxProperties: 1
properties:
  value: {type: string, pattern: '^x|y$', x-valid-examples: [x, y]}
additionalProperties: false
enum: [{value: x}]`,
			exact: `{"value":"y"}`,
		},
		{
			name: "array item oracle overrides local enum",
			schema: `type: array
minItems: 1
maxItems: 1
items: {type: string, enum: [x], pattern: '^x|y$', x-valid-examples: [y]}
enum: [[x]]`,
			exact: `["y"]`,
		},
		{
			name: "object property oracle overrides local enum",
			schema: `type: object
required: [value]
maxProperties: 1
properties:
  value: {type: string, enum: [x], pattern: '^x|y$', x-valid-examples: [y]}
additionalProperties: false
enum: [{value: x}]`,
			exact: `{"value":"y"}`,
		},
		{
			name: "equal domain occurrences keep separate oracles",
			schema: `type: object
required: [value]
maxProperties: 2
properties:
  value: {type: string, pattern: '^[a-z]$', x-valid-examples: [x]}
  other: {type: string, pattern: '^[a-z]$', x-valid-examples: [y, z]}
additionalProperties: false
enum: [{value: x}]`,
			exact: `{"other":"y","value":"x"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			compiler := NewCompiler(parseSchemaSource(t, tt.schema, "", "create"))
			compiled, err := compiler.CompileSuite()
			require.NoError(t, err)

			found, err := hasExactRejectedCase(compiled, "enum", tt.exact)
			require.NoError(t, err)
			require.True(t, found, exactRejectedBodies(t, compiled, "enum"))
		})
	}
}

// TestCompileSuiteDoesNotUseTargetEnumOracleAsOutsider verifies an enum's own
// accepted occurrence evidence cannot become a rejection witness for that enum.
func TestCompileSuiteDoesNotUseTargetEnumOracleAsOutsider(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: string
pattern: '^x|y$'
enum: [x]
x-valid-examples: [y]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	found, err := hasExactRejectedCase(compiled, "enum", `"y"`)
	require.NoError(t, err)
	require.False(t, found, exactRejectedBodies(t, compiled, "enum"))
}

// TestCasePlannerKeepsIncompleteOptionalObjectSearchUnconstructible verifies
// an unmet minimum is not mistaken for proof that an enum dominates context.
func TestCasePlannerKeepsIncompleteOptionalObjectSearchUnconstructible(t *testing.T) {
	t.Parallel()

	registry := NewDomainRegistry()
	opaque := singleKindDomain(jsonvalue.KindString)
	opaque.String = StringConstraints{State: KindRestricted, Patterns: []string{"^x$"}}
	opaqueID := registry.FindOrAddEquivalentDomain(opaque)
	context := anyJSONDomain()
	context.Null = KindExcluded
	context.Boolean = KindExcluded
	context.Number.State = KindExcluded
	context.String.State = KindExcluded
	context.Array.State = KindExcluded
	context.Object = ObjectConstraints{
		State: KindRestricted,
		Properties: []NamedProperty{{
			Name: "opaque", State: PropertyAllowed, Values: opaqueID,
		}},
		Additional: AdditionalProperties{Values: EmptyDomainID},
		MinProps:   1,
		MaxProps:   new(1),
	}
	contextID := registry.FindOrAddEquivalentDomain(context)
	pass := finiteDomain([]jsonvalue.Value{{
		Kind: jsonvalue.KindObject,
		Object: []jsonvalue.Member{{
			Name: "opaque", Value: jsonvalue.String("x"),
		}},
	}})
	passID := registry.FindOrAddEquivalentDomain(pass)
	planner := &CasePlanner{Domains: registry}
	constraint := ConstraintPlan{
		Source: ConstraintSource{Pointer: "#/schema", Keyword: "enum"},
		Pass:   passID,
	}

	failures, err := planner.contextFailures(constraint, contextID)
	require.NoError(t, err)
	require.Empty(t, failures)
	planner.finishUnplannedConstraint(&constraint, contextID, failures)
	require.Equal(t, ObligationUnconstructible, constraint.Outcome)
}

// exactRejectedBodies returns deterministic singleton bodies for diagnostics.
func exactRejectedBodies(t *testing.T, compiled *CompiledSuite, keyword string) []string {
	t.Helper()

	var result []string

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect != ExpectRejected || plannedCase.Source.Keyword != keyword {
			continue
		}

		domain := mustDomain(t, compiled.Domains, plannedCase.Values)
		if domain.Enum == nil || len(domain.Enum.Values) != 1 {
			continue
		}

		body, err := domain.Enum.Values[0].MarshalJSON()
		require.NoError(t, err)

		result = append(result, string(body))
	}

	return result
}

// TestCasePlannerProvesOnlyEmptyContainerEnumsDominated verifies finite closed
// container contexts are classified as exhausted instead of unconstructible.
func TestCasePlannerProvesOnlyEmptyContainerEnumsDominated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
	}{
		{name: "array", schema: `type: array
items: {}
maxItems: 0
enum: [[]]`},
		{name: "object", schema: `type: object
maxProperties: 0
properties: {optional: {type: string}}
additionalProperties: false
enum: [{}]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			compiler := NewCompiler(parseSchemaSource(t, tt.schema, "", "create"))
			compiled, err := compiler.CompileSuite()
			require.NoError(t, err)
			plan := constraintPlanAt(t, compiled.Constraints, ConstraintSource{
				Pointer: compiler.Source.RequestSchema.Pointer,
				Keyword: "enum",
			})
			require.Equal(t, ObligationDominated, plan.Outcome)
		})
	}
}

// TestCasePlannerKeepsBoundedContainerExhaustionUnconstructible verifies a
// bounded search is not promoted to a proof for general container products.
func TestCasePlannerKeepsBoundedContainerExhaustionUnconstructible(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: array
items: {type: boolean}
maxItems: 1
enum: [[], [false], [true]]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)
	plan := constraintPlanAt(t, compiled.Constraints, ConstraintSource{
		Pointer: compiler.Source.RequestSchema.Pointer,
		Keyword: "enum",
	})
	require.Equal(t, ObligationUnconstructible, plan.Outcome)
}

// TestCasePlannerKeepsItemRulesInParentFailures verifies parent count failures are isolated.
func TestCasePlannerKeepsItemRulesInParentFailures(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: array
maxItems: 1
items: {type: string}`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)
	root := mustDomain(t, compiled.Domains, compiled.Root)

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect == ExpectRejected && plannedCase.Source.Keyword == "maxItems" {
			partition := mustDomain(t, compiled.Domains, plannedCase.Values)
			require.Equal(t, root.Array.Items, partition.Array.Items)

			return
		}
	}

	require.Fail(t, "maxItems failure not found")
}

// TestCasePlannerBuildsNarrowMultipleOfFailure verifies boundary-local witnesses.
func TestCasePlannerBuildsNarrowMultipleOfFailure(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: number
minimum: 100
maximum: 101
multipleOf: 0.5`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)
	plan := constraintPlanAt(t, compiled.Constraints, ConstraintSource{
		Pointer: compiler.Source.RequestSchema.Pointer, Keyword: "multipleOf",
	})
	require.Equal(t, ObligationPlanned, plan.Outcome)
}

// TestCasePlannerSolvesMultipleOfFailuresFromExactSiblingContext verifies each
// direction receives a witness from the other rule's exact lattice or enum.
func TestCasePlannerSolvesMultipleOfFailuresFromExactSiblingContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   string
		expected map[int]string
	}{
		{
			name: "integer lattice two by three",
			schema: `type: number
allOf:
  - {multipleOf: 2}
  - {multipleOf: 3}`,
			expected: map[int]string{0: "3", 1: "2"},
		},
		{
			name: "exact decimal lattice",
			schema: `type: number
allOf:
  - {multipleOf: 0.2}
  - {multipleOf: 0.3}`,
			expected: map[int]string{0: "0.3", 1: "0.2"},
		},
		{
			name: "sibling enum",
			schema: `type: number
allOf:
  - {multipleOf: 2}
  - {enum: [42, 43]}`,
			expected: map[int]string{0: "43"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			compiler := NewCompiler(parseSchemaSource(t, tt.schema, "", "create"))
			compiled, err := compiler.CompileSuite()
			require.NoError(t, err)

			root := compiler.Source.RequestSchema.Pointer
			for branch, exact := range tt.expected {
				source := ConstraintSource{
					Pointer: fmt.Sprintf("%s/allOf/%d", root, branch),
					Keyword: "multipleOf",
				}
				constraint := constraintPlanAt(t, compiled.Constraints, source)
				require.Equal(t, ObligationPlanned, constraint.Outcome, source.Pointer)
				requireExactRejectedSource(t, compiled, source, exact)
			}
		})
	}
}

// requireExactRejectedSource finds one exact singleton witness for a source.
func requireExactRejectedSource(
	t *testing.T,
	compiled *CompiledSuite,
	source ConstraintSource,
	exact string,
) {
	t.Helper()

	want, err := jsonvalue.Parse([]byte(exact))
	require.NoError(t, err)

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect != ExpectRejected || plannedCase.Source != source {
			continue
		}

		domain := mustDomain(t, compiled.Domains, plannedCase.Values)
		if domain.Enum != nil && len(domain.Enum.Values) == 1 && domain.Enum.Values[0].Equal(want) {
			return
		}
	}

	require.Fail(
		t,
		"exact rejected source witness not found",
		"source=%v exact=%s bodies=%v",
		source,
		exact,
		exactRejectedBodies(t, compiled, source.Keyword),
	)
}

// TestCasePlannerSolvesNarrowContinuousNumericContexts verifies exact
// off-lattice and non-integer witnesses without a fixed candidate bag.
func TestCasePlannerSolvesNarrowContinuousNumericContexts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		keyword string
		exact   string
	}{
		{
			name: "integer against narrow number",
			schema: `allOf:
  - {type: integer}
  - {type: number, minimum: 0, maximum: 0.1}`,
			keyword: "type",
		},
		{
			name: "narrow off lattice",
			schema: `type: number
allOf:
  - {multipleOf: 0.1}
  - {minimum: 0.29, maximum: 0.31}`,
			keyword: "multipleOf",
			exact:   "0.31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			compiler := NewCompiler(parseSchemaSource(t, tt.schema, "", "create"))
			compiled, err := compiler.CompileSuite()
			require.NoError(t, err)

			found := false

			for _, constraint := range compiled.Constraints {
				if constraint.Source.Keyword == tt.keyword && constraint.Outcome == ObligationPlanned {
					found = true
				}
			}

			require.True(t, found)

			if tt.exact != "" {
				exactFound, exactErr := hasExactRejectedCase(compiled, tt.keyword, tt.exact)
				require.NoError(t, exactErr)
				require.True(t, exactFound)
			}
		})
	}
}

// hasExactRejectedCase reports an exact singleton rejected case.
func hasExactRejectedCase(compiled *CompiledSuite, keyword string, exact string) (bool, error) {
	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect != ExpectRejected || plannedCase.Source.Keyword != keyword {
			continue
		}

		domain, ok := compiled.Domains.Domain(plannedCase.Values)
		if !ok || domain.Enum == nil || len(domain.Enum.Values) != 1 {
			continue
		}

		body, err := domain.Enum.Values[0].MarshalJSON()
		if err != nil {
			return false, err
		}

		if string(body) == exact {
			return true, nil
		}
	}

	return false, nil
}

// TestCasePlannerMarksFiniteEnumExhaustionDominated verifies exact finite
// implication is distinguished from an incomplete constructive search.
func TestCasePlannerMarksFiniteEnumExhaustionDominated(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `allOf:
  - {enum: [1, 2]}
  - {enum: [1, 2]}`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	for _, constraint := range compiled.Constraints {
		if constraint.Source.Keyword == "enum" {
			require.Equal(t, ObligationDominated, constraint.Outcome)
		}
	}
}

// TestCasePlannerMarksExactNumericImplicationDominated verifies a stronger
// sibling lattice proves dominance instead of reporting an incomplete search.
func TestCasePlannerMarksExactNumericImplicationDominated(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: number
allOf:
  - {multipleOf: 2}
  - {multipleOf: 4}`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	root := compiler.Source.RequestSchema.Pointer
	constraint := constraintPlanAt(t, compiled.Constraints, ConstraintSource{
		Pointer: root + "/allOf/0", Keyword: "multipleOf",
	})
	require.Equal(t, ObligationDominated, constraint.Outcome)
}

// TestCasePlannerDoesNotOverflowMaximumFailureBounds verifies impossible larger collections stay unconstructible.
func TestCasePlannerDoesNotOverflowMaximumFailureBounds(t *testing.T) {
	t.Parallel()

	maxInt := int(^uint(0) >> 1)
	for _, schema := range []string{
		fmt.Sprintf("type: string\nmaxLength: %d", maxInt),
		fmt.Sprintf("type: array\nitems: {}\nmaxItems: %d", maxInt),
		fmt.Sprintf("type: object\nmaxProperties: %d", maxInt),
	} {
		t.Run(schema, func(t *testing.T) {
			t.Parallel()

			compiler := NewCompiler(parseSchemaSource(t, schema, "", "create"))
			_, err := compiler.Compile()
			require.NoError(t, err)

			planner := &CasePlanner{Domains: compiler.Domains}
			_, err = planner.Plan(compiler.rootUse)
			require.NoError(t, err)

			for _, constraint := range planner.Constraints {
				if strings.HasPrefix(constraint.Source.Keyword, "max") {
					require.Equal(t, ObligationUnconstructible, constraint.Outcome)
				}
			}
		})
	}
}

// contradictoryChildCase describes one exact structural planning expectation.
type contradictoryChildCase struct {
	name    string
	schema  string
	extra   string
	keyword string
	body    string
	present bool
}

// reachableContradictoryChildCases covers direct and conjoined structural seams.
var reachableContradictoryChildCases = []contradictoryChildCase{
	{
		name: "items reachable",
		schema: `type: array
items:
  allOf:
    - {type: string}
    - {type: boolean}`,
		keyword: "items", body: `[null]`, present: true,
	},
	{
		name: "items blocked by maximum",
		schema: `type: array
maxItems: 0
items:
  allOf:
    - {type: string}
    - {type: boolean}`,
		keyword: "items", body: `[null]`, present: false,
	},
	{
		name: "items satisfy positive minimum",
		schema: `minItems: 2
maxItems: 2
items:
  allOf:
    - {type: string}
    - {type: boolean}`,
		keyword: "items", body: `[null,null]`, present: true,
	},
	{
		name: "items survive one child allOf wrapper",
		schema: `allOf:
  - minItems: 1
    maxItems: 1
    items:
      allOf:
        - {type: string}
        - {type: boolean}`,
		keyword: "items", body: `[null]`, present: true,
	},
	{
		name: "items survive identity first",
		schema: `allOf:
  - {}
  - minItems: 1
    maxItems: 1
    items:
      allOf:
        - {type: string}
        - {type: boolean}`,
		keyword: "items", body: `[null]`, present: true,
	},
	{
		name: "items survive identity last",
		schema: `allOf:
  - minItems: 1
    maxItems: 1
    items:
      allOf:
        - {type: string}
        - {type: boolean}
  - {}`,
		keyword: "items", body: `[null]`, present: true,
	},
	{
		name: "wrapped items remain blocked by zero maximum",
		schema: `allOf:
  - maxItems: 0
    items:
      allOf:
        - {type: string}
        - {type: boolean}`,
		keyword: "items", body: `[null]`, present: false,
	},
	{
		name: "items contradiction composes across branches",
		schema: `allOf:
  - minItems: 1
    maxItems: 1
    items: {type: string}
  - items: {type: boolean}`,
		keyword: "items", body: `[null]`, present: true,
	},
	{
		name: "referenced items survive allOf wrapper",
		schema: `allOf:
  - {$ref: '#/components/schemas/ImpossibleArray'}`,
		extra: `components:
  schemas:
    ImpossibleArray:
      minItems: 1
      maxItems: 1
      items:
        allOf:
          - {type: string}
          - {type: boolean}`,
		keyword: "items", body: `[null]`, present: true,
	},
	{
		name: "additional reachable",
		schema: `type: object
additionalProperties:
  allOf:
    - {type: string}
    - {type: boolean}`,
		keyword: "additionalProperties", body: `{"additional":null}`, present: true,
	},
	{
		name: "additional blocked by maximum",
		schema: `type: object
maxProperties: 0
additionalProperties:
  allOf:
    - {type: string}
    - {type: boolean}`,
		keyword: "additionalProperties", body: `{"additional":null}`, present: false,
	},
	{
		name: "optional property reachable",
		schema: `type: object
properties:
  value:
    allOf:
      - {type: string}
      - {type: boolean}
additionalProperties: false`,
		keyword: "properties", body: `{"value":null}`, present: true,
	},
	{
		name: "optional property retains required and minimum siblings",
		schema: `type: object
minProperties: 3
maxProperties: 4
required: [required]
properties:
  required: {enum: [ok]}
  filler1: {enum: [fill1]}
  filler2: {enum: [fill2]}
  value:
    allOf:
      - {type: string}
      - {type: boolean}
additionalProperties: false`,
		keyword: "properties",
		body:    `{"value":null,"required":"ok","filler1":"fill1"}`,
		present: true,
	},
	{
		name: "untyped positive minimum keeps contradictory optional shape",
		schema: `minProperties: 1
maxProperties: 1
properties:
  value:
    allOf:
      - {type: string}
      - {type: boolean}
additionalProperties: false`,
		keyword: "properties", body: `{"value":null}`, present: true,
	},
	{
		name: "optional shape survives one child allOf wrapper",
		schema: `allOf:
  - minProperties: 1
    maxProperties: 1
    properties:
      value:
        allOf:
          - {type: string}
          - {type: boolean}
    additionalProperties: false`,
		keyword: "properties", body: `{"value":null}`, present: true,
	},
	{
		name: "optional shape survives identity first",
		schema: `allOf:
  - {}
  - minProperties: 1
    maxProperties: 1
    properties:
      value:
        allOf:
          - {type: string}
          - {type: boolean}
    additionalProperties: false`,
		keyword: "properties", body: `{"value":null}`, present: true,
	},
	{
		name: "optional shape survives identity last",
		schema: `allOf:
  - minProperties: 1
    maxProperties: 1
    properties:
      value:
        allOf:
          - {type: string}
          - {type: boolean}
    additionalProperties: false
  - {}`,
		keyword: "properties", body: `{"value":null}`, present: true,
	},
	{
		name: "untyped positive minimum keeps contradictory additional shape",
		schema: `minProperties: 1
maxProperties: 1
additionalProperties:
  allOf:
    - {type: string}
    - {type: boolean}`,
		keyword: "additionalProperties", body: `{"additional":null}`, present: true,
	},
	{
		name: "additional shape survives one child allOf wrapper",
		schema: `allOf:
  - minProperties: 1
    maxProperties: 1
    additionalProperties:
      allOf:
        - {type: string}
        - {type: boolean}`,
		keyword: "additionalProperties", body: `{"additional":null}`, present: true,
	},
	{
		name: "wrapped additional remains blocked by zero maximum",
		schema: `allOf:
  - maxProperties: 0
    additionalProperties:
      allOf:
        - {type: string}
        - {type: boolean}`,
		keyword: "additionalProperties", body: `{"additional":null}`, present: false,
	},
	{
		name: "optional contradiction composes across branches",
		schema: `allOf:
  - minProperties: 2
    maxProperties: 2
    required: [required]
    properties:
      required: {enum: [ok]}
      value: {type: string}
    additionalProperties: false
  - properties:
      required: {enum: [ok]}
      value: {type: boolean}
    additionalProperties: false`,
		keyword: "properties", body: `{"value":null,"required":"ok"}`, present: true,
	},
	{
		name: "optional cross branch contradiction is order symmetric",
		schema: `allOf:
  - properties:
      required: {enum: [ok]}
      value: {type: boolean}
    additionalProperties: false
  - minProperties: 2
    maxProperties: 2
    required: [required]
    properties:
      required: {enum: [ok]}
      value: {type: string}
    additionalProperties: false`,
		keyword: "properties", body: `{"value":null,"required":"ok"}`, present: true,
	},
	{
		name: "referenced optional shape survives allOf wrapper",
		schema: `allOf:
  - {$ref: '#/components/schemas/ImpossibleObject'}`,
		extra: `components:
  schemas:
    ImpossibleObject:
      minProperties: 1
      maxProperties: 1
      properties:
        value:
          allOf:
            - {type: string}
            - {type: boolean}
      additionalProperties: false`,
		keyword: "properties", body: `{"value":null}`, present: true,
	},
	{
		name: "false additional policy is not a contradictory schema seam",
		schema: `allOf:
  - minProperties: 1
    maxProperties: 1
    additionalProperties: false`,
		keyword: "additionalProperties", body: `{"additional":null}`, present: false,
	},
	{
		name: "untyped contradictory optional respects impossible maximum",
		schema: `minProperties: 1
maxProperties: 0
properties:
  value:
    allOf:
      - {type: string}
      - {type: boolean}
additionalProperties: false`,
		keyword: "properties", body: `{"value":null}`, present: false,
	},
	{
		name: "additional retains required and minimum siblings",
		schema: `type: object
minProperties: 3
maxProperties: 4
required: [required]
properties:
  required: {enum: [ok]}
  filler1: {enum: [fill1]}
  filler2: {enum: [fill2]}
additionalProperties:
  allOf:
    - {type: string}
    - {type: boolean}`,
		keyword: "additionalProperties",
		body:    `{"additional":null,"required":"ok","filler1":"fill1"}`,
		present: true,
	},
	{
		name: "optional property blocked when required sibling consumes maximum",
		schema: `type: object
maxProperties: 1
required: [required]
properties:
  required: {enum: [ok]}
  value:
    allOf:
      - {type: string}
      - {type: boolean}
additionalProperties: false`,
		keyword: "properties", body: `{"value":null}`, present: false,
	},
}

// TestCompileSuitePlansReachableContradictoryChildren verifies impossible child
// policies produce deterministic rejected containers only when cardinality permits.
func TestCompileSuitePlansReachableContradictoryChildren(t *testing.T) {
	t.Parallel()

	for _, tt := range reachableContradictoryChildCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			compiler := NewCompiler(parseSchemaSource(t, tt.schema, tt.extra, "create"))
			compiled, err := compiler.CompileSuite()
			require.NoError(t, err)

			if strings.HasPrefix(tt.name, "untyped ") {
				root := mustDomain(t, compiled.Domains, compiled.Root)
				require.NotEqual(t, KindExcluded, root.Null)
				require.NotEqual(t, KindExcluded, root.Boolean)
			}

			found := false
			want, parseErr := jsonvalue.Parse([]byte(tt.body))
			require.NoError(t, parseErr)

			for _, plannedCase := range compiled.Cases {
				if plannedCase.Expect != ExpectRejected || plannedCase.Source.Keyword != tt.keyword {
					continue
				}

				domain := mustDomain(t, compiled.Domains, plannedCase.Values)
				if domain.Enum == nil || len(domain.Enum.Values) != 1 {
					continue
				}

				found = found || domain.Enum.Values[0].Equal(want)
			}

			require.Equal(t, tt.present, found, caseNames(compiled.Cases))
		})
	}
}

// TestCompileSuiteNestedAllOfPlanningKeepsCombinedZeroMaximum verifies equal
// semantic child meets retain union constraints and composed planning shapes.
func TestCompileSuiteNestedAllOfPlanningKeepsCombinedZeroMaximum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		outer      string
		positive   string
		maximum    string
		keyword    string
		body       string
		childUse   func(*schemaUse) *schemaUse
		constraint []string
		indent     int
	}{
		{
			name: "property",
			outer: `allOf:
  - type: object
    required: [outer]
    maxProperties: 1
    properties:
      outer:
%s
    additionalProperties: false
  - type: object
    required: [outer]
    maxProperties: 1
    properties:
      outer:
%s
    additionalProperties: false`,
			positive: `minProperties: 1
properties:
  value:
    allOf:
      - {type: string}
      - {type: boolean}
additionalProperties: false`,
			maximum:    "maxProperties: 0",
			keyword:    "properties",
			body:       `{"outer":{"value":null}}`,
			childUse:   func(use *schemaUse) *schemaUse { return use.property("outer") },
			constraint: []string{"minProperties", "maxProperties"},
			indent:     8,
		},
		{
			name: "item",
			outer: `allOf:
  - type: array
    minItems: 1
    maxItems: 1
    items:
%s
  - type: array
    minItems: 1
    maxItems: 1
    items:
%s`,
			positive: `minItems: 1
items:
  allOf:
    - {type: string}
    - {type: boolean}`,
			maximum:    "maxItems: 0",
			keyword:    "items",
			body:       `[[null]]`,
			childUse:   func(use *schemaUse) *schemaUse { return use.items },
			constraint: []string{"minItems", "maxItems"},
			indent:     6,
		},
	}

	for _, tt := range tests {
		for order, branches := range [][]string{{tt.positive, tt.maximum}, {tt.maximum, tt.positive}} {
			name := fmt.Sprintf("%s/order-%d", tt.name, order)
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				prefix := strings.Repeat(" ", tt.indent)
				first := prefix + strings.ReplaceAll(branches[0], "\n", "\n"+prefix)
				second := prefix + strings.ReplaceAll(branches[1], "\n", "\n"+prefix)
				schema := fmt.Sprintf(tt.outer, first, second)
				compiler := NewCompiler(parseSchemaSource(t, schema, "", "create"))
				compiled, err := compiler.CompileSuite()
				require.NoError(t, err)

				found, err := hasExactRejectedCase(compiled, tt.keyword, tt.body)
				require.NoError(t, err)
				require.False(t, found, exactRejectedBodies(t, compiled, tt.keyword))

				child := tt.childUse(compiler.rootUse)
				require.NotNil(t, child)

				for _, keyword := range tt.constraint {
					require.True(t, constraintSourcesContainKeyword(child.constraints, keyword), child.constraints)
				}
			})
		}
	}
}

// constraintSourcesContainKeyword reports keyword membership without pointer equivalence.
func constraintSourcesContainKeyword(sources []ConstraintSource, keyword string) bool {
	for _, source := range sources {
		if source.Keyword == keyword {
			return true
		}
	}

	return false
}

// TestCasePlannerBoundsContradictoryContainerMaterialization verifies hostile
// cardinalities cannot force huge exact witness allocations during planning.
func TestCasePlannerBoundsContradictoryContainerMaterialization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
	}{
		{
			name: "array",
			schema: `minItems: 5000
items:
  allOf:
    - {type: string}
    - {type: boolean}`,
		},
		{
			name: "object",
			schema: `type: object
minProperties: 5000
properties:
  value:
    allOf:
      - {type: string}
      - {type: boolean}
additionalProperties: true`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			compiler := NewCompiler(parseSchemaSource(t, tt.schema, "", "create"))
			_, err := compiler.Compile()
			require.NoError(t, err)

			planner := &CasePlanner{Domains: compiler.Domains}
			cases, err := planner.Plan(compiler.rootUse)
			require.NoError(t, err)
			require.NotContains(t, caseNames(cases), "invalid contradictory")
		})
	}
}

// caseNames joins the names of cases.
func caseNames(cases []CasePlan) string {
	names := make([]string, 0, len(cases))
	for _, plannedCase := range cases {
		names = append(names, plannedCase.Name)
	}

	return strings.Join(names, "\n")
}

// constraintPlanAt returns the plan matching source.
func constraintPlanAt(t *testing.T, plans []ConstraintPlan, source ConstraintSource) ConstraintPlan {
	t.Helper()

	for _, plan := range plans {
		if plan.Source == source {
			return plan
		}
	}

	require.FailNow(t, "ConstraintPlan not found", source)

	return ConstraintPlan{}
}

// liftedPropertyCaseCount returns the number of lifted left and right property cases.
func liftedPropertyCaseCount(cases []CasePlan) int {
	count := 0

	for _, plannedCase := range cases {
		if strings.Contains(plannedCase.Name, "property left /") || strings.Contains(plannedCase.Name, "property right /") {
			count++
		}
	}

	return count
}
