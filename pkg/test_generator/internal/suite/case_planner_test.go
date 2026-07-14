package suite

import (
	"fmt"
	"strings"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/internal/jsonvalue"
	"github.com/stretchr/testify/require"
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
