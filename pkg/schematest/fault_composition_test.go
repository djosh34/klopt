//nolint:godoclint // Focused private composition-fault tests use behavior names.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompositionDifferenceReusesBoundedSubtreeMetadata(t *testing.T) {
	t.Parallel()

	parent := &jsonValue{kind: jsonObject, object: map[string]*jsonValue{}}
	assignment := &jsonValue{kind: jsonObject, object: map[string]*jsonValue{}}
	parentCursor := parent
	assignmentCursor := assignment

	for index := 0; index < 100; index++ {
		parentChild := &jsonValue{kind: jsonObject, object: map[string]*jsonValue{}}
		assignmentChild := &jsonValue{kind: jsonObject, object: map[string]*jsonValue{}}
		parentCursor.object["nested"] = parentChild
		assignmentCursor.object["nested"] = assignmentChild
		parentCursor = parentChild
		assignmentCursor = assignmentChild
	}

	parentCursor.object["leaf"] = &jsonValue{kind: jsonBoolean}
	assignmentCursor.object["leaf"] = &jsonValue{kind: jsonBoolean, boolean: true}

	searchState := &search{maxSteps: 10}
	source := compositionDifference(parent, assignment, nil)
	count, err := source.Count(searchState)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, uint64(1), searchState.steps)

	for range 2 {
		edit, exists, err := source.At(0, searchState)
		require.NoError(t, err)
		require.True(t, exists)
		require.Len(t, edit.path, 101)
	}

	require.Equal(t, uint64(3), searchState.steps)
}

func TestAllOfFaultKeepsSiblingBranchesTrue(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"object",
		"allOf":[
			{"required":["a"],"properties":{"a":{"type":"string"}}},
			{"required":["b"],"properties":{"b":{"type":"string"}}}
		]
	}`)
	fault := findFaultTarget(t, plan, "/allOf/0|#/a|required|fault:required")
	searchState := &search{model: model, maxSteps: 100_000}

	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, `{"a":"","b":""}`, string(marshalFaultTestValue(t, parent)))

	derivative, err := applyFault(parent, fault, searchState)
	require.NoError(t, err)
	require.Equal(t, `{"b":""}`, string(marshalFaultTestValue(t, derivative)))
	require.Equal(t, `{"a":"","b":""}`, string(marshalFaultTestValue(t, parent)))

	result := evaluate(model, derivative)
	require.Equal(t, identityStrings(fault.expected), identityStrings(result.failureRecords()))
	require.Equal(t, [][]bool{{false, true}}, compositionTruthVectorsForTest(result.compositionRecords(oracleRuleAllOf)))
}

func TestAggregateFaultConcretizesFailureAtInsertedItem(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"array",
		"items":{},
		"anyOf":[
			{"maxItems":0},
			{"items":{"type":"string"}}
		]
	}`)
	fault := findFaultTarget(t, plan, "|anyOf|fault:anyOf")
	searchState := &search{model: model, maxSteps: 1_000_000}
	selected, exists, exhausted, err := faultClosureAtRank(fault, 0, searchState)
	require.NoError(t, err)
	require.True(t, exists)
	require.False(t, exhausted)

	parent := &jsonValue{kind: jsonArray, array: []*jsonValue{}}
	selected, exists, err = faultAtOccurrenceRank(parent, selected, 0, searchState)
	require.NoError(t, err)
	require.True(t, exists)

	value, attempted, exhausted, err := compositionFaultAttemptAtRank(
		parent, selected, 0, searchState,
	)
	require.NoError(t, err)
	require.True(t, attempted)
	require.False(t, exhausted)
	require.NotNil(t, value)

	result := evaluate(model, value)
	require.Equal(t, []string{"maxItems", "type", "anyOf"}, failureRules(result.failureRecords()))

	for failure := range result.failureRecords() {
		if failure.rule == oracleRuleType {
			require.Equal(t, "#/0", failure.occurrence.instanceTemplate)
		}
	}
}

func TestAggregateFaultConcretizesFailureAtInsertedProperty(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"object",
		"anyOf":[
			{"additionalProperties":false},
			{"additionalProperties":{"type":"string"}}
		]
	}`)
	fault := findFaultTarget(t, plan, "|anyOf|fault:anyOf")
	searchState := &search{model: model, maxSteps: 1_000_000}

	var generated Case

	err := streamFault(plan, fault, searchState, make(map[string]bool), func(testCase Case) error {
		generated = testCase

		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, generated.JSON)

	value, err := parseStrictJSON(generated.JSON)
	require.NoError(t, err)

	result := evaluate(model, value)
	require.Equal(
		t,
		[]string{"additionalProperties", "type", "anyOf"},
		failureRules(result.failureRecords()),
	)

	for failure := range result.failureRecords() {
		if failure.rule == oracleRuleType {
			require.NotEqual(t, "#/*", failure.occurrence.instanceTemplate)
		}
	}
}

//nolint:cyclop // Closure selection and the complete aggregate assertion belong together.
func TestAggregateFaultCombinesProspectiveCoordinates(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"object","required":["a","b"],
		"properties":{"a":{"type":"array","items":{}},"b":{"type":"array","items":{}}},
		"anyOf":[
			{"properties":{"a":{"maxItems":0}}},
			{"properties":{"a":{"items":{"type":"string"}}}},
			{"properties":{"b":{"items":{"type":"string"}}}}
		]
	}`)
	fault := findFaultTarget(t, plan, "|anyOf|fault:anyOf")
	searchState := &search{model: model, maxSteps: 1_000_000}

	var selected faultProgram

	for rank := uint64(0); ; rank++ {
		candidate, exists, exhausted, err := faultClosureAtRank(fault, rank, searchState)
		require.NoError(t, err)

		if exhausted {
			break
		}

		if !exists {
			continue
		}

		hasA := false
		hasB := false

		for _, expected := range candidate.expected {
			template := expected.project().occurrence.instanceTemplate
			hasA = hasA || template == "#/a/*"
			hasB = hasB || template == "#/b/*"
		}

		if hasA && hasB {
			selected = candidate

			break
		}
	}

	require.NotEmpty(t, selected.expected)

	parent := &jsonValue{kind: jsonObject, object: map[string]*jsonValue{
		"a": {kind: jsonArray, array: []*jsonValue{}},
		"b": {kind: jsonArray, array: []*jsonValue{}},
	}}
	selected, exists, err := faultAtOccurrenceRank(parent, selected, 0, searchState)
	require.NoError(t, err)
	require.True(t, exists)

	var derivative *jsonValue
	for rank := uint64(0); rank < 20 && derivative == nil; rank++ {
		derivative, _, _, err = compositionFaultAttemptAtRank(parent, selected, rank, searchState)
		require.NoError(t, err)
	}

	require.NotNil(t, derivative)
	require.JSONEq(t, `{"a":[null],"b":[null]}`, string(marshalFaultTestValue(t, derivative)))
}

func TestBuildCompositionFaultGoldenStream(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"properties":{"a":{"type":"string"},"b":{"type":"string"}},
		"anyOf":[{"required":["a"]},{"required":["b"]}]
	}`))
	collect := func(maxSteps uint64) ([]Case, Report) {
		var cases []Case

		report, err := Build(
			Input{OpenAPI: document, OperationID: "selected", MaxSteps: maxSteps},
			func(testCase Case) error {
				cases = append(cases, testCase)

				return nil
			},
		)
		require.NoError(t, err)

		return cases, report
	}

	cases, report := collect(1_000_000)
	require.Equal(t, []Case{
		{JSON: []byte(`{"a":"","b":""}`), Valid: true},
		{JSON: []byte(`{"a":"","b":""}`), Valid: true},
		{JSON: []byte(`null`), Valid: false},
		{JSON: []byte(`{}`), Valid: false},
		{JSON: []byte(`{}`), Valid: false},
		{JSON: []byte(`{}`), Valid: false},
		{JSON: []byte(`{"a":null}`), Valid: false},
		{JSON: []byte(`{"a":"","b":null}`), Valid: false},
	}, cases)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Equal(t, []string{"mask:3"}, anyOfReportMasks(report.Covered))
	require.True(t, reportIdentityHasSuffix(report.Covered, "|anyOf|fault:anyOf"))
}

func TestBuildAllOfCompositionFaultGoldenStream(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"allOf":[
			{"required":["a"],"properties":{"a":{"type":"string"}}},
			{"required":["b"],"properties":{"b":{"type":"string"}}}
		]
	}`))
	collect := func(maxSteps uint64) ([]Case, Report) {
		var cases []Case

		report, err := Build(Input{
			OpenAPI: document, OperationID: "selected", MaxSteps: maxSteps,
		}, func(testCase Case) error {
			cases = append(cases, testCase)

			return nil
		})
		require.NoError(t, err)

		return cases, report
	}

	cases, report := collect(1_000_000)
	require.Len(t, cases, 8)

	for _, testCase := range cases[:3] {
		require.Equal(t, Case{JSON: []byte(`{"a":"","b":""}`), Valid: true}, testCase)
	}

	require.Equal(t, []Case{
		{JSON: []byte(`null`), Valid: false},
		{JSON: []byte(`{"b":""}`), Valid: false},
		{JSON: []byte(`{"a":null,"b":""}`), Valid: false},
		{JSON: []byte(`{"a":""}`), Valid: false},
		{JSON: []byte(`{"a":"","b":null}`), Valid: false},
	}, cases[3:])
	require.Equal(t, SpaceExhausted, report.Stop)
	require.True(t, reportIdentityHasSuffix(report.Covered, "|allOf|level:all-true"))
}

func compositionFaultModel(t *testing.T, schema string) (*schemaModel, *searchPlan) {
	t.Helper()

	model, err := parseInput(Input{
		OpenAPI: []byte(documentWithJSONSchema(schema)), OperationID: "selected",
	})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	return model, plan
}
