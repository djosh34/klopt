package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRowProjectionCursorYieldsNestedMasksInCanonicalOrder locks one shared composition order.
func TestRowProjectionCursorYieldsNestedMasksInCanonicalOrder(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"allOf":[{"anyOf":[{"title":"a"},{"title":"b"}]}],
		"anyOf":[{"title":"c"},{"title":"d"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	cursor := newRowProjectionCursor(model.root, model.root.occurrence, nil)
	defer cursor.Close()

	var alternatives [][]string

	for {
		view, ok, nextErr := cursor.Next()
		require.NoError(t, nextErr)

		if !ok {
			break
		}

		alternatives = append(alternatives, projectionBranchPointers(view))
	}

	root := model.root.occurrence.usePointer
	require.Equal(t, [][]string{
		{root + "/allOf/0/anyOf/0", root + "/anyOf/0"},
		{root + "/allOf/0/anyOf/0", root + "/anyOf/1"},
		{root + "/allOf/0/anyOf/0", root + "/anyOf/0", root + "/anyOf/1"},
		{root + "/allOf/0/anyOf/1", root + "/anyOf/0"},
		{root + "/allOf/0/anyOf/1", root + "/anyOf/1"},
		{root + "/allOf/0/anyOf/1", root + "/anyOf/0", root + "/anyOf/1"},
		{root + "/allOf/0/anyOf/0", root + "/allOf/0/anyOf/1", root + "/anyOf/0"},
		{root + "/allOf/0/anyOf/0", root + "/allOf/0/anyOf/1", root + "/anyOf/1"},
		{root + "/allOf/0/anyOf/0", root + "/allOf/0/anyOf/1", root + "/anyOf/0", root + "/anyOf/1"},
	}, alternatives)
}

// TestRowProjectionCursorHonorsOnlyExplicitPins keeps false branches out of the active view.
func TestRowProjectionCursorHonorsOnlyExplicitPins(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{"enum":["a"]},{"enum":["b"]},{"enum":["c"]}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	requirements := []requirement{
		branchRequirement(model.root, 0, true),
		branchRequirement(model.root, 1, false),
	}

	cursor := newRowProjectionCursor(model.root, model.root.occurrence, requirements)
	defer cursor.Close()

	var alternatives [][]string

	for {
		view, ok, nextErr := cursor.Next()
		require.NoError(t, nextErr)

		if !ok {
			break
		}

		alternatives = append(alternatives, projectionBranchPointers(view))
	}

	root := model.root.occurrence.usePointer
	require.Equal(t, [][]string{
		{root + "/anyOf/0"},
		{root + "/anyOf/0", root + "/anyOf/2"},
	}, alternatives)
}

// TestRowProjectionCursorIsArbitraryPrecisionAndLazy proves no fixed-width mask or mask corpus.
func TestRowProjectionCursorIsArbitraryPrecisionAndLazy(t *testing.T) {
	t.Parallel()

	branches := make([]*schemaNode, 65)
	for index := range branches {
		branches[index] = &schemaNode{schemaShape: &schemaShape{}}
	}

	root := &schemaNode{
		schemaShape: &schemaShape{anyOf: branches},
		occurrence:  schemaOccurrence{usePointer: "#/schema", targetPointer: "#/schema", instanceTemplate: "#"},
	}

	cursor := newRowProjectionCursor(root, root.occurrence, nil)
	defer cursor.Close()

	first, ok, err := cursor.Next()
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []string{"#/schema/anyOf/0"}, projectionBranchPointers(first))

	second, ok, err := cursor.Next()
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []string{"#/schema/anyOf/1"}, projectionBranchPointers(second))
}

// TestScalarSearchConsumesEveryProjectionMask proves the migration seam is live.
func TestScalarSearchConsumesEveryProjectionMask(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"anyOf":[{"type":"string"},{"type":"string"}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	searchState := &search{model: model, maxSteps: 100}

	var masks [][]bool

	complete, err := searchState.walkActiveScalarRequirementAlternatives(
		model.root,
		model.root.occurrence,
		nil,
		func(requirements []requirement) (bool, error) {
			states, constrained := rowCompositionTruthStates(
				requirements, model.root.occurrence, "anyOf", len(model.root.anyOf),
			)
			require.True(t, constrained)

			masks = append(masks, states)

			return false, nil
		},
	)
	require.NoError(t, err)
	require.False(t, complete)
	require.Equal(t, [][]bool{{true, false}, {false, true}, {true, true}}, masks)
}

// TestRowProjectionViewKeepsCompleteValuesAndOccurrenceIdentity locks the yielded-view contract.
func TestRowProjectionViewKeepsCompleteValuesAndOccurrenceIdentity(t *testing.T) {
	t.Parallel()

	model, err := parseInput(Input{OpenAPI: []byte(documentWithJSONSchema(`{
		"allOf":[{"enum":[[true,true,true]]}],
		"anyOf":[{"default":{"x":1}},{"default":{"x":2}}]
	}`)), OperationID: "selected"})
	require.NoError(t, err)

	cursor := newRowProjectionCursor(model.root, model.root.occurrence, nil)
	defer cursor.Close()

	view, ok, err := cursor.Next()
	require.NoError(t, err)
	require.True(t, ok)

	var values []string

	err = view.eachDirectValue(func(source rowSchemaSource, value *jsonValue) bool {
		encoded, marshalErr := marshalStrict(value)
		require.NoError(t, marshalErr)

		values = append(values, source.occurrence.usePointer+"="+string(encoded))

		return true
	})
	require.NoError(t, err)

	root := model.root.occurrence.usePointer
	require.Equal(t, []string{
		root + "/allOf/0=[true,true,true]",
		root + `/anyOf/0={"x":1}`,
	}, values)
}

// projectionBranchPointers returns active anyOf branch sources for assertions.
func projectionBranchPointers(view rowProjectionView) []string {
	var pointers []string

	view.eachSource(func(source rowSchemaSource) bool {
		if _, nested := rowAnyOfParentUsePointer(source.occurrence.usePointer); nested {
			pointers = append(pointers, source.occurrence.usePointer)
		}

		return true
	})

	return pointers
}

// branchRequirement creates one explicit root anyOf pin for projection tests.
func branchRequirement(parent *schemaNode, branch int, truth bool) requirement {
	child := parent.anyOf[branch]

	return requirement{
		tag: requirementBranchTruth,
		occurrence: rebasePlanOccurrence(
			child,
			parent.occurrence,
			parent.occurrence.usePointer+"/anyOf/"+itoa(branch),
			parent.occurrence.instanceTemplate,
		),
		composition: "anyOf",
		branch:      branch,
		truth:       truth,
		hasBranch:   true,
	}
}
