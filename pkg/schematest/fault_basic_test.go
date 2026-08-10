//nolint:godoclint // Focused private fault tests use behavior names.
package schematest

import (
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegenerateParentAndApplyBasicTypeFault(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"type":"string"}`)),
		OperationID: "selected",
	})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)

	fault := findFaultTarget(t, plan, "|type|fault:type")
	searchState := &search{model: model, maxSteps: 10}

	parent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, `""`, string(marshalFaultTestValue(t, parent)))
	require.Equal(t, uint64(2), searchState.steps)

	secondParent, found, err := regenerateParent(plan, fault, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.NotSame(t, parent, secondParent)
	require.Equal(t, uint64(4), searchState.steps)

	derivative, err := applyFault(secondParent, fault, searchState)
	require.NoError(t, err)
	require.Equal(t, `null`, string(marshalFaultTestValue(t, derivative)))
	require.Equal(t, `""`, string(marshalFaultTestValue(t, secondParent)))
	require.Equal(t, uint64(7), searchState.steps)

	result := evaluate(model, derivative)
	require.False(t, result.valid)
	require.Equal(t, identityStrings(fault.expected), identityStrings(result.failureRecords()))
}

func TestRegenerateParentAtRankPreservesAndAdvancesAnyOfPins(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"type":"object",
		"required":["x"],
		"properties":{"x":{"type":"string"}},
		"anyOf":[
			{"properties":{"x":{"enum":["a"]}}},
			{"properties":{"x":{"enum":["b"]}}}
		]
	}`)
	fault := findFaultTarget(t, plan, "|anyOf|fault:anyOf")

	searchState := &search{model: model, maxSteps: 100_000}
	first, found, exhausted, err := regenerateParentAtRank(plan, fault, 0, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, exhausted)
	require.JSONEq(t, `{"x":"a"}`, string(marshalFaultTestValue(t, first)))

	second, found, exhausted, err := regenerateParentAtRank(plan, fault, 1, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, exhausted)
	require.JSONEq(t, `{"x":"b"}`, string(marshalFaultTestValue(t, second)))
}

func TestParentReplayYieldsPastUnproductiveFirstAnyOfMask(t *testing.T) {
	t.Parallel()

	model, plan := compositionFaultModel(t, `{
		"anyOf":[
			{"type":"string","minLength":2,"maxLength":1},
			{"enum":[0]}
		]
	}`)
	fault := findFaultTarget(t, plan, "|anyOf|fault:anyOf")
	searchState := &search{model: model, maxSteps: 1_000}

	parent, found, exhausted, err := regenerateParentAtRank(plan, fault, 0, searchState)
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, exhausted)
	require.Equal(t, `0`, string(marshalFaultTestValue(t, parent)))
}

func TestParentReplayMaskAtRankReachesBranchesBeyondUint64Bits(t *testing.T) {
	t.Parallel()

	for rank := uint64(0); rank <= 64; rank++ {
		mask, exists := parentReplayMaskAtOrdinal(65, new(big.Int).SetUint64(rank))
		require.True(t, exists)
		require.Equal(t, 1, mask.BitLen()-int(rank))
		require.Equal(t, uint(1), mask.Bit(int(rank)))
	}

	_, finite := parentReplayMaskFiniteSize(65)
	require.False(t, finite)

	beyondUint64 := new(big.Int).Lsh(big.NewInt(1), 64)
	mask, exists := parentReplayMaskAtOrdinal(65, beyondUint64)
	require.True(t, exists)
	require.Positive(t, mask.BitLen())

	cursor := &parentReplayMaskCursor{
		branches: 65,
		selected: 64,
		indexes:  append([]int(nil), integerRange(1, 65)...),
		started:  true,
	}
	mask, exists = cursor.Next()
	require.True(t, exists)
	require.Equal(t, strings.Repeat("1", 65), mask.Text(2))
}

func integerRange(first, end int) []int {
	values := make([]int, 0, end-first)
	for value := first; value < end; value++ {
		values = append(values, value)
	}

	return values
}

func TestFaultClosureAtRankEnumeratesNestedAlternativesWithoutTuples(t *testing.T) {
	t.Parallel()

	identity := func(pointer, rule string) failureIdentity {
		return makeRuleIdentity(schemaOccurrence{
			usePointer: pointer, targetPointer: pointer, instanceTemplate: "#",
		}, rule)
	}
	firstA := &faultClosureAlternative{expected: faultClosure{
		newEvaluationRecordIdentity(identity("#/a", oracleRuleMinimum)),
	}}
	firstB := &faultClosureAlternative{expected: faultClosure{
		newEvaluationRecordIdentity(identity("#/a", oracleRuleMaximum)),
	}}
	firstA.next = firstB
	secondA := &faultClosureAlternative{expected: faultClosure{
		newEvaluationRecordIdentity(identity("#/b", oracleRulePattern)),
	}}
	secondB := &faultClosureAlternative{expected: faultClosure{
		newEvaluationRecordIdentity(identity("#/b", oracleRuleFormat)),
	}}
	secondA.next = secondB
	program := &faultClosureProgram{alternatives: firstA, next: &faultClosureProgram{alternatives: secondA}}
	fault := faultProgram{expected: faultClosure{
		newEvaluationRecordIdentity(identity("#", oracleRuleAnyOf)),
	}, alternatives: program}

	var got [][]string

	for rank := uint64(0); ; rank++ {
		searchState := &search{maxSteps: 100}
		selected, exists, exhausted, err := faultClosureAtRank(fault, rank, searchState)
		require.NoError(t, err)

		if exhausted {
			break
		}

		require.True(t, exists)
		require.Nil(t, selected.alternatives)
		require.Equal(t, uint64(2), searchState.steps)

		got = append(got, identityStrings(selected.expected))
	}

	require.Equal(t, [][]string{
		{"#|#|anyOf", "#/a|#|minimum", "#/b|#|pattern"},
		{"#|#|anyOf", "#/a|#|minimum", "#/b|#|format"},
		{"#|#|anyOf", "#/a|#|maximum", "#/b|#|pattern"},
		{"#|#|anyOf", "#/a|#|maximum", "#/b|#|format"},
	}, got)
}

func TestFaultClosureWideRankZeroHonorsTinyCutoff(t *testing.T) {
	t.Parallel()

	var first *faultClosureProgram

	for index := 63; index >= 0; index-- {
		left := &faultClosureAlternative{}
		left.next = &faultClosureAlternative{}
		first = &faultClosureProgram{alternatives: left, next: first}
	}

	searchState := &search{maxSteps: 1}
	selected, exists, exhausted, err := faultClosureAtRank(
		faultProgram{alternatives: first}, 0, searchState,
	)
	require.ErrorIs(t, err, errMaxSteps)
	require.Equal(t, faultProgram{}, selected)
	require.False(t, exists)
	require.False(t, exhausted)
	require.Equal(t, uint64(1), searchState.steps)
}

func TestStreamAggregateFaultAdvancesPastImpossibleFirstClosure(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"anyOf":[
			{"minimum":0},
			{"maximum":10,"multipleOf":2}
		]
	}`))

	model, err := parseInput(Input{OpenAPI: document, OperationID: "selected"})
	require.NoError(t, err)

	plan, err := makePlan(model)
	require.NoError(t, err)
	fault := findFaultTarget(t, plan, "|anyOf|fault:anyOf")
	searchState := &search{model: model, maxSteps: 1_000_000}
	covered := make(map[string]bool)

	var aggregate Case

	err = streamFault(plan, fault, searchState, covered, func(generated Case) error {
		aggregate = generated

		return nil
	})
	require.NoError(t, err)
	require.True(t, covered[fault.obligation.String()])
	require.False(t, aggregate.Valid)

	value, err := parseStrictJSON(aggregate.JSON)
	require.NoError(t, err)

	result := evaluate(model, value)
	require.Equal(t, []string{"minimum", "multipleOf", "anyOf"}, failureRules(result.failureRecords()))

	fullSteps := searchState.steps
	require.Positive(t, fullSteps)
	cutoff := &search{model: model, maxSteps: fullSteps - 1}
	cutoffEmitted := false
	err = streamFault(plan, fault, cutoff, make(map[string]bool), func(Case) error {
		cutoffEmitted = true

		return nil
	})
	require.ErrorIs(t, err, errMaxSteps)
	require.False(t, cutoffEmitted)
	require.Equal(t, fullSteps-1, cutoff.steps)

	repeated := &search{model: model, maxSteps: fullSteps}

	var repeatedCase Case

	err = streamFault(plan, fault, repeated, make(map[string]bool), func(generated Case) error {
		repeatedCase = generated

		return nil
	})
	require.NoError(t, err)
	require.Equal(t, fullSteps, repeated.steps)
	require.Equal(t, aggregate, repeatedCase)
}

func TestBuildStreamsBasicTypeFaultAfterValidTargets(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{"type":"string"}`))

	var cases []Case

	report, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 7},
		func(generated Case) error {
			cases = append(cases, generated)

			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, []Case{
		{JSON: []byte(`""`), Valid: true},
		{JSON: []byte(`null`), Valid: false},
	}, cases)
	require.Equal(t, SpaceExhausted, report.Stop)
	require.Equal(t, uint64(6), report.Steps)
	require.Empty(t, report.Uncovered)
}

func TestBuildDiscardsBasicFaultAtCutoff(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{"type":"string"}`))

	for _, maxSteps := range []uint64{3, 4, 5} {
		var cases []Case

		report, err := Build(
			Input{OpenAPI: document, OperationID: "selected", MaxSteps: maxSteps},
			func(generated Case) error {
				cases = append(cases, generated)

				return nil
			},
		)
		require.NoError(t, err)
		require.Equal(t, []Case{{JSON: []byte(`""`), Valid: true}}, cases)
		require.Equal(t, MaxStepsReached, report.Stop)
		require.Equal(t, maxSteps, report.Steps)
		require.Len(t, report.Uncovered, 1)
		require.Contains(t, report.Uncovered[0], "|type|fault:type")
	}
}

func TestRegenerateParentPreservesFaultKindRequirements(t *testing.T) {
	t.Parallel()

	for name, schema := range map[string]string{
		"typeless minLength":       `{"minLength":2}`,
		"mixed enum and minLength": `{"enum":["ok",1],"minLength":2}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			model, plan := compositionFaultModel(t, schema)
			fault := findFaultTarget(t, plan, "|minLength|fault:minLength")
			searchState := &search{model: model, maxSteps: 100_000}

			first, found, err := regenerateParent(plan, fault, searchState)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, jsonString, first.kind)
			firstJSON := marshalFaultTestValue(t, first)

			second, found, err := regenerateParent(plan, fault, searchState)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, firstJSON, marshalFaultTestValue(t, second))
		})
	}
}

func TestBuildVisitsMaximumFaultInsideAnyOfContext(t *testing.T) {
	t.Parallel()

	var cases []Case

	report, err := Build(Input{
		OpenAPI: []byte(documentWithJSONSchema(`{
			"type":"number","maximum":100,"anyOf":[{"type":"number"}]
		}`)),
		OperationID: "selected",
		MaxSteps:    1_000_000,
	}, func(testCase Case) error {
		cases = append(cases, testCase)

		return nil
	})
	require.NoError(t, err)
	require.Equal(t, []Case{
		{JSON: []byte(`100`), Valid: true},
		{JSON: []byte(`null`), Valid: false},
		{JSON: []byte(`101`), Valid: false},
	}, cases)
	require.Equal(t, MaxStepsReached, report.Stop)
	require.Contains(t, report.Covered,
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|maximum|fault:maximum")
	require.Contains(t, report.Uncovered,
		"#/paths/~1/post/requestBody/content/application~1json/schema|#|anyOf|fault:anyOf")
}

func marshalFaultTestValue(t *testing.T, value *jsonValue) []byte {
	t.Helper()

	encoded, err := marshalStrict(value)
	require.NoError(t, err)

	return encoded
}
