package suite

import (
	"strings"
	"testing"

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
		require.True(t, properties["id"].Required)
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
