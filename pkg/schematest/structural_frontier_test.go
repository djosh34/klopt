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
	structure, ok, _, err := searchState.rowArrayStructureAt(
		model.root, model.root.occurrence, nil, 0,
	)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, uint64(1), structure.projectionRank)
	require.Equal(t, uint64(0), structure.length.value)
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
