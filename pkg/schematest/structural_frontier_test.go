package schematest

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestArrayLengthCursorSkipsDuplicateNamedPhases proves named collisions do not end the domain.
func TestArrayLengthCursorSkipsDuplicateNamedPhases(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array","items":{},"minItems":2,"maxItems":5
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	view := rowProjectionView{sources: []rowSchemaSource{{
		node: model.root, occurrence: model.root.occurrence,
	}}}
	cursor, err := newRowArrayLengthCursor(view, []requirement{{
		tag: requirementExactCount, occurrence: model.root.occurrence, count: model.root.minItems,
	}})
	require.NoError(t, err)

	var lengths []uint64

	for {
		length, ok, nextErr := cursor.Next()
		require.NoError(t, nextErr)

		if !ok {
			break
		}

		require.False(t, length.beyond)
		lengths = append(lengths, length.value)
	}

	require.Equal(t, []uint64{2, 5, 0, 1, 3, 4}, lengths)
}

// TestFiniteArrayLengthCursorEmitsEveryInteriorCount proves finite numeric ranks are complete.
func TestFiniteArrayLengthCursorEmitsEveryInteriorCount(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array","items":{},"minItems":0,"maxItems":5
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	view := rowProjectionView{sources: []rowSchemaSource{{
		node: model.root, occurrence: model.root.occurrence,
	}}}
	cursor, err := newRowArrayLengthCursor(view, nil)
	require.NoError(t, err)

	var lengths []uint64

	for {
		length, ok, nextErr := cursor.Next()
		require.NoError(t, nextErr)

		if !ok {
			break
		}

		lengths = append(lengths, length.value)
	}

	require.Equal(t, []uint64{0, 5, 1, 2, 3, 4}, lengths)
}

// TestArrayStructureSkipsInfeasibleProjection proves contradictory masks consume no length rank.
func TestArrayStructureSkipsInfeasibleProjection(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array","items":{},"anyOf":[
			{"minItems":2,"maxItems":1},
			{"maxItems":0}
		]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1000}

	var selected *jsonValue

	complete, err := searchState.walkArray(
		model.root, model.root.occurrence, nil, rowSearchContext{},
		func(value *jsonValue) (bool, error) {
			selected = value

			return true, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.NotNil(t, selected)
	require.Empty(t, selected.array)
}

// TestArrayChildRanksAdvanceDiagonally locks fair position order.
func TestArrayChildRanksAdvanceDiagonally(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array","items":{"enum":[false,true]},"minItems":2,"maxItems":2
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	view := rowProjectionView{sources: []rowSchemaSource{{
		node: model.root, occurrence: model.root.occurrence,
	}}}
	structure := rankedArrayStructure{view: view, length: rowArrayCount{value: 2}}
	searchState := &search{model: model, maxSteps: 1000}

	var tuples [][]bool

	for rank := uint64(0); rank < 3; rank++ {
		values, ok, usable, _, childErr := searchState.rowArrayChildrenAt(
			structure, nil, rowSearchContext{}, rank,
		)
		require.NoError(t, childErr)
		require.True(t, ok)
		require.True(t, usable)

		tuples = append(tuples, []bool{values[0].boolean, values[1].boolean})
	}

	require.Equal(t, [][]bool{{false, false}, {false, true}, {true, false}}, tuples)
}

// TestUnsatisfiableWildcardDoesNotStarveLaterProjection proves mask fairness.
func TestUnsatisfiableWildcardDoesNotStarveLaterProjection(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object","anyOf":[
			{"minProperties":1,"additionalProperties":{
				"type":"string","allOf":[{"pattern":"^a+$"},{"pattern":"^b+$"}]
			}},
			{"maxProperties":0}
		]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 100}
	found := false
	complete, err := searchState.walkObject(
		model.root, model.root.occurrence, nil, rowSearchContext{},
		func(value *jsonValue) (bool, error) {
			if len(value.object) == 0 {
				found = true

				return true, nil
			}

			return false, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.True(t, found)
}

// TestFiniteObjectFrontierExhausts proves genuine finite endpoints stop without MaxSteps.
func TestFiniteObjectFrontierExhausts(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object","required":["x"],"additionalProperties":false,
		"properties":{"x":{"enum":["z"]}}
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1000}

	var rows int

	complete, err := searchState.walkObject(
		model.root, model.root.occurrence, nil, rowSearchContext{},
		func(*jsonValue) (bool, error) {
			rows++

			return false, nil
		},
	)
	require.NoError(t, err)
	require.False(t, complete)
	require.Positive(t, rows)
	require.Less(t, searchState.steps, searchState.maxSteps)
}

// TestRowArrayLengthAtKeepsDirectGuidanceAheadOfExact proves exact counts do not bypass authored lengths.
func TestRowArrayLengthAtKeepsDirectGuidanceAheadOfExact(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array","enum":[[false,false]],"items":{},"minItems":1,"maxItems":3
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	view := rowProjectionView{sources: []rowSchemaSource{{node: model.root, occurrence: model.root.occurrence}}}
	requirements := []requirement{{
		tag: requirementExactCount, occurrence: model.root.occurrence, count: model.root.minItems,
	}}

	first, ok, _, err := rowArrayLengthAt(view, requirements, 0)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, uint64(2), first.value)

	second, ok, _, err := rowArrayLengthAt(view, requirements, 1)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, uint64(1), second.value)
}

// TestLiveProjectionFrontierRecordsOnlyNaturalExhaustion locks monotonic cursor ownership.
func TestLiveProjectionFrontierRecordsOnlyNaturalExhaustion(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{"type":"array","items":{}},{"type":"array","items":{}}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	frontier := newLiveProjectionFrontier(model.root, model.root.occurrence, nil)
	defer frontier.Close()

	var alternatives [][]string

	for {
		view, ok, nextErr := frontier.Next()
		require.NoError(t, nextErr)

		if !ok {
			break
		}

		alternatives = append(alternatives, projectionBranchPointers(view))
	}

	require.Equal(t, [][]string{
		{model.root.occurrence.usePointer + "/anyOf/0"},
		{model.root.occurrence.usePointer + "/anyOf/1"},
		{model.root.occurrence.usePointer + "/anyOf/0", model.root.occurrence.usePointer + "/anyOf/1"},
	}, alternatives)
	require.True(t, frontier.exhausted)
	require.Equal(t, uint64(3), frontier.finiteSize)
}

// TestArrayFrontierReachesLaterProjectionWitnessRanks proves child exhaustion cannot close direct ranks.
func TestArrayFrontierReachesLaterProjectionWitnessRanks(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array","items":{"enum":[false]},"minItems":1,"maxItems":1,
		"anyOf":[{"enum":[[1]]},{"enum":[[2],[3],[4]]}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1000}
	found := false
	complete, err := searchState.walkArray(
		model.root, model.root.occurrence, nil, rowSearchContext{},
		func(value *jsonValue) (bool, error) {
			encoded, marshalErr := marshalStrict(value)
			if marshalErr != nil {
				return false, marshalErr
			}

			found = string(encoded) == `[4]`

			return found, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.True(t, found)
}

// TestObjectFrontierReachesLaterProjectionWitnessRanks proves object direct ranks are independent.
func TestObjectFrontierReachesLaterProjectionWitnessRanks(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object","additionalProperties":false,
		"anyOf":[{"enum":[{"x":1}]},{"enum":[{"x":2},{"x":3},{"x":4}]}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1000}
	found := false
	complete, err := searchState.walkObject(
		model.root, model.root.occurrence, nil, rowSearchContext{},
		func(value *jsonValue) (bool, error) {
			encoded, marshalErr := marshalStrict(value)
			if marshalErr != nil {
				return false, marshalErr
			}

			found = string(encoded) == `{"x":4}`

			return found, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.True(t, found)
}

// TestArrayFrontierStreamsSameLengthDirectWitnesses proves witness rank is independent of length.
func TestArrayFrontierStreamsSameLengthDirectWitnesses(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array","enum":[[false],[true]],"items":{},"minItems":1,"maxItems":1
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1000}

	var witnesses []bool

	complete, err := searchState.walkArray(
		model.root, model.root.occurrence, nil, rowSearchContext{},
		func(value *jsonValue) (bool, error) {
			if len(value.array) == 1 && value.array[0].kind == jsonBoolean &&
				(len(witnesses) == 0 || witnesses[len(witnesses)-1] != value.array[0].boolean) {
				witnesses = append(witnesses, value.array[0].boolean)
			}

			return len(witnesses) == 2, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.Equal(t, []bool{false, true}, witnesses)
}

// TestObjectFrontierStreamsEveryDirectWitness proves object witnesses have their own rank.
func TestObjectFrontierStreamsEveryDirectWitness(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object","enum":[{"x":false},{"x":true}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1000}

	var witnesses []bool

	complete, err := searchState.walkObject(
		model.root, model.root.occurrence, nil, rowSearchContext{},
		func(value *jsonValue) (bool, error) {
			member, exists := value.object["x"]
			if exists && member.kind == jsonBoolean {
				witnesses = append(witnesses, member.boolean)
			}

			return len(witnesses) == 2, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.Equal(t, []bool{false, true}, witnesses)
}

// TestSharedArrayFrontierBuildsAllOfItems proves explicit constructed ranks keep composed children.
func TestSharedArrayFrontierBuildsAllOfItems(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array","minItems":1,"items":{"type":"string"},
		"allOf":[{"items":{"enum":["z"]}}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1000}
	found := false
	complete, err := searchState.walkArray(
		model.root, model.root.occurrence, nil, rowSearchContext{},
		func(value *jsonValue) (bool, error) {
			encoded, marshalErr := marshalStrict(value)
			if marshalErr != nil {
				return false, marshalErr
			}

			found = string(encoded) == `["z"]`

			return found, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.True(t, found)
}

// TestExactArrayCountKeepsEveryDirectWitnessRank proves constructed guidance cannot retire direct values.
func TestExactArrayCountKeepsEveryDirectWitnessRank(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array","enum":[[1],[2],[3]],"items":{},"minItems":1,"maxItems":1
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1000}
	found := false
	complete, err := searchState.walkArray(
		model.root,
		model.root.occurrence,
		[]requirement{{
			tag: requirementExactCount, occurrence: model.root.occurrence, count: model.root.minItems,
		}},
		rowSearchContext{},
		func(value *jsonValue) (bool, error) {
			encoded, marshalErr := marshalStrict(value)
			if marshalErr != nil {
				return false, marshalErr
			}

			found = string(encoded) == `[3]`

			return found, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.True(t, found)
}

// TestArrayProjectionResumesAfterLaterMask proves later masks do not retire an earlier source.
func TestArrayProjectionResumesAfterLaterMask(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"array","items":{},"minItems":1,"maxItems":1,
		"anyOf":[
			{"items":{"enum":[false,true]}},
			{"items":{"enum":[2]}}
		]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1000}
	laterMaskSeen := false
	found := false
	complete, err := searchState.walkArray(
		model.root, model.root.occurrence, nil, rowSearchContext{},
		func(value *jsonValue) (bool, error) {
			encoded, marshalErr := marshalStrict(value)
			if marshalErr != nil {
				return false, marshalErr
			}

			laterMaskSeen = laterMaskSeen || string(encoded) == `[2]`
			found = laterMaskSeen && string(encoded) == `[true]`

			return found, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.True(t, found)
}

// TestObjectProjectionResumesDirectRanksAfterLaterMask proves object sources remain independent.
func TestObjectProjectionResumesDirectRanksAfterLaterMask(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object","additionalProperties":false,
		"anyOf":[
			{"enum":[{"x":1},{"x":2},{"x":3}]},
			{"enum":[{"y":1}]}
		]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 1000}
	laterMaskSeen := false
	found := false
	complete, err := searchState.walkObject(
		model.root, model.root.occurrence, nil, rowSearchContext{},
		func(value *jsonValue) (bool, error) {
			encoded, marshalErr := marshalStrict(value)
			if marshalErr != nil {
				return false, marshalErr
			}

			laterMaskSeen = laterMaskSeen || string(encoded) == `{"y":1}`
			found = laterMaskSeen && string(encoded) == `{"x":3}`

			return found, nil
		},
	)
	require.NoError(t, err)
	require.True(t, complete)
	require.True(t, found)
}

// TestObjectMemberSelectionAndPresenceChargeSeparately locks the two mutation boundaries.
func TestObjectMemberSelectionAndPresenceChargeSeparately(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"type":"object","properties":{"x":{}},"additionalProperties":false
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	cursor := newRowProjectionCursor(model.root, model.root.occurrence, nil)
	view, ok, err := cursor.Next()
	cursor.Close()
	require.NoError(t, err)
	require.True(t, ok)

	shape, err := newRowProjectedObject(view, nil, model.root.occurrence)
	require.NoError(t, err)

	structure := rankedObjectStructure{shape: shape, present: []bool{false}}

	selectionOnly := &search{model: model, maxSteps: 1}
	_, err = selectionOnly.rowObjectMembersForStructure(structure)
	require.ErrorIs(t, err, errMaxSteps)
	require.Equal(t, uint64(1), selectionOnly.steps)

	withPresence := &search{model: model, maxSteps: 2}
	members, err := withPresence.rowObjectMembersForStructure(structure)
	require.NoError(t, err)
	require.Empty(t, members)
	require.Equal(t, uint64(2), withPresence.steps)
}

// TestBuildRequiredMembersMayExceedDirectedObjectTarget proves exact guidance is not a ceiling.
func TestBuildRequiredMembersMayExceedDirectedObjectTarget(t *testing.T) {
	t.Parallel()

	document := []byte(documentWithJSONSchema(`{
		"type":"object",
		"minProperties":1,
		"required":["x","y"],
		"properties":{
			"x":{"type":"boolean","enum":[false]},
			"y":{"type":"boolean","enum":[false]}
		}
	}`))

	stop := errors.New("stop after required object")
	found := false
	_, err := Build(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 10000},
		func(testCase Case) error {
			if testCase.Valid && string(testCase.JSON) == `{"x":false,"y":false}` {
				found = true

				return stop
			}

			return nil
		},
	)
	require.ErrorIs(t, err, stop)
	require.True(t, found)
}
