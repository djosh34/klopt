package schematest

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
)

// applyCompositionFault searches for the smallest exact aggregate edit relative
// to the regenerated parent. Rejected candidates are transient.
func applyCompositionFault(parent *jsonValue, fault faultProgram, s *search) (*jsonValue, error) {
	if parent == nil {
		return nil, errors.New("schematest: nil composition fault parent")
	}

	if s == nil || s.model == nil || s.model.root == nil {
		return nil, errors.New("schematest: composition fault application has no model")
	}

	derivative, found, err := findCompositionFaultDerivative(parent, fault, s)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, fmt.Errorf("%w: %s", errFaultNotFound, fault.obligation.String())
	}

	return derivative, nil
}

// compositionEdit is one parent-relative aggregate mutation.
type compositionEdit struct {
	path        []string
	replacement *jsonValue
	remove      bool
	append      bool
}

// findCompositionFaultDerivative uses complete-schema assignments only as a
// deterministic source of edits. It applies the smallest useful subset to the
// current parent, so unrelated parent paths are retained.
func findCompositionFaultDerivative(
	parent *jsonValue,
	fault faultProgram,
	s *search,
) (*jsonValue, bool, error) {
	derivative, found, err := applyCompositionRequirementEdits(parent, fault, s)
	if err != nil || found {
		return derivative, found, err
	}

	visit := func(value *jsonValue) (bool, error) {
		return visitCompositionEditSizes(
			compositionDifference(parent, value, nil), s,
			func(selected []compositionEdit) (bool, error) {
				candidate, matched, candidateErr := tryCompositionEdits(parent, fault, selected, s)
				if candidateErr != nil || !matched {
					return false, candidateErr
				}

				derivative = candidate

				return true, nil
			},
		)
	}

	found, err = s.walkNode(
		s.model.root,
		s.model.root.occurrence,
		fault.requirements,
		rowSearchContext{},
		visit,
	)

	return derivative, found, err
}

// compositionAssignmentMachine owns the resumable subset continuations for direct row addresses.
type compositionAssignmentMachine struct {
	search *search

	direct             *compositionRankedSubsetCursor
	assignment         *compositionRankedSubsetCursor
	assignmentViewRank uint64
	assignmentRowRank  uint64
	assignmentSet      bool
}

// compositionFaultAttemptAtRank is the standalone composition-assignment adapter.
func compositionFaultAttemptAtRank(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
	s *search,
) (*jsonValue, bool, bool, error) {
	return compositionFaultAttemptWithMachine(
		parent, fault, rank, &compositionAssignmentMachine{search: s},
	)
}

// compositionFaultAttemptWithMachine advances one bounded direct or assignment coordinate.
//
//nolint:cyclop // Prospective, direct, and assignment sources meet at one ranked attempt.
func compositionFaultAttemptWithMachine(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
	assignments *compositionAssignmentMachine,
) (*jsonValue, bool, bool, error) {
	if parent == nil {
		return nil, false, false, errors.New("schematest: nil composition fault parent")
	}

	if assignments == nil || assignments.search == nil ||
		assignments.search.model == nil || assignments.search.model.root == nil {
		return nil, false, false, errors.New("schematest: composition assignment machine has no model")
	}

	createdEdits, prospectiveSource, prospectiveErr := prospectiveCompositionEditsAtRank(parent, fault, rank)
	if prospectiveErr != nil {
		return nil, false, false, prospectiveErr
	}

	if prospectiveSource && len(createdEdits) > 0 {
		candidate, matched, err := tryCompositionEdits(
			parent, fault, createdEdits, assignments.search,
		)
		if err != nil || matched {
			return candidate, true, false, err
		}
	}

	if assignments.direct == nil {
		assignments.direct = newCompositionRankedSubsetCursor(
			compositionDirectEdits(parent, fault.requirements), assignments.search,
		)
	}

	candidate, attempted, err := assignments.direct.AttemptAtRank(parent, fault, rank)
	if err != nil || attempted {
		return candidate, attempted, false, err
	}

	return assignments.AttemptAtRank(parent, fault, rank)
}

// AttemptAtRank evaluates one assignment-row/subset coordinate and yields.
//
//nolint:cyclop // Direct row exhaustion and resumable subset state meet here.
func (machine *compositionAssignmentMachine) AttemptAtRank(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
) (*jsonValue, bool, bool, error) {
	projectionRank, rowRank, subsetRank, ok := faultCandidateRanksAtOrdinal(rank)
	if !ok {
		return nil, false, true, nil
	}

	assignment, exists, _, finiteSize, err := faultRowAt(
		machine.search,
		machine.search.model.root,
		machine.search.model.root.occurrence,
		fault.requirements,
		rowSearchContext{},
		projectionRank,
		rowRank,
	)
	if err != nil {
		return nil, false, false, err
	}

	if !exists || assignment == nil {
		exhausted := subsetRank == 0 && (projectionRank == 0 ||
			finiteSize > 0 && rowRank >= finiteSize)

		return nil, false, exhausted, nil
	}

	if !machine.assignmentSet || machine.assignmentViewRank != projectionRank ||
		machine.assignmentRowRank != rowRank {
		machine.assignment = newCompositionRankedSubsetCursor(
			compositionDifference(parent, assignment, nil), machine.search,
		)
		machine.assignmentViewRank = projectionRank
		machine.assignmentRowRank = rowRank
		machine.assignmentSet = true
	}

	candidate, attempted, err := machine.assignment.AttemptAtRank(parent, fault, subsetRank)

	return candidate, attempted, false, err
}

// prospectiveCompositionSource identifies one missing wildcard container.
type prospectiveCompositionSource struct {
	path      []string
	container *jsonValue
}

// prospectiveCompositionEditsAtRank addresses canonical subsets of mutation-created paths.
//
//nolint:cyclop // Source discovery and direct subset decoding share one boundary.
func prospectiveCompositionEditsAtRank(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
) ([]compositionEdit, bool, error) {
	sourceCount, err := prospectiveCompositionSourceCount(parent, fault)
	if err != nil || sourceCount == 0 {
		return nil, false, err
	}

	subsets, finite := parentReplayMaskFiniteSize(sourceCount)
	if !finite {
		return nil, true, nil
	}

	subsetRank, tupleRank, addressed := rowSourceValueRanksAtOrdinal(subsets, rank)
	if !addressed {
		return nil, true, nil
	}

	mask, exists := parentReplayMaskAtOrdinal(sourceCount, new(big.Int).SetUint64(subsetRank))
	if !exists {
		return nil, true, nil
	}

	selected := 0

	for index := range sourceCount {
		if mask.Bit(index) == 1 {
			selected++
		}
	}

	decoder, ok := newDirectRankTupleDecoder(selected*directRowRankDimensions, tupleRank)
	if !ok {
		return nil, true, nil
	}

	edits := make([]compositionEdit, 0, selected)

	for index := range sourceCount {
		if mask.Bit(index) == 0 {
			continue
		}

		source, sourceExists, sourceErr := prospectiveCompositionSourceAt(parent, fault, index)
		if sourceErr != nil || !sourceExists {
			return nil, true, sourceErr
		}

		coordinateRank, _ := decoder.Next()
		valueRank, _ := decoder.Next()

		value, valueExists, _, valueErr := canonicalGenericValueAt(valueRank)
		if valueErr != nil || !valueExists {
			return nil, true, valueErr
		}

		edit, editExists := prospectiveCompositionEdit(source, coordinateRank, value)
		if !editExists {
			return nil, true, nil
		}

		edits = append(edits, edit)
	}

	return edits, true, nil
}

// prospectiveCompositionSourceCount counts eligible identities without retaining coordinates.
func prospectiveCompositionSourceCount(parent *jsonValue, fault faultProgram) (int, error) {
	count := 0

	for index := range fault.expected {
		_, exists, err := prospectiveCompositionSourceForIdentity(parent, fault.expected[index])
		if err != nil {
			return 0, err
		}

		if exists {
			count++
		}
	}

	return count, nil
}

// prospectiveCompositionSourceAt directly addresses one eligible identity.
func prospectiveCompositionSourceAt(
	parent *jsonValue,
	fault faultProgram,
	wanted int,
) (prospectiveCompositionSource, bool, error) {
	for index := range fault.expected {
		source, exists, err := prospectiveCompositionSourceForIdentity(parent, fault.expected[index])
		if err != nil {
			return prospectiveCompositionSource{}, false, err
		}

		if !exists {
			continue
		}

		if wanted == 0 {
			return source, true, nil
		}

		wanted--
	}

	return prospectiveCompositionSource{}, false, nil
}

// prospectiveCompositionSourceForIdentity resolves one missing wildcard container.
//
//nolint:cyclop // Identity filtering and wildcard resolution are one direct lookup.
func prospectiveCompositionSourceForIdentity(
	parent *jsonValue,
	expected evaluationRecordIdentity,
) (prospectiveCompositionSource, bool, error) {
	projected := expected.project()
	if projected.rule != oracleRuleType ||
		!strings.Contains(projected.occurrence.instanceTemplate, "*") ||
		matchingValuePathCount(parent, projected.occurrence.instanceTemplate) > 0 {
		return prospectiveCompositionSource{}, false, nil
	}

	tokens, ok := rowPointerTokens(projected.occurrence.instanceTemplate)
	if !ok {
		return prospectiveCompositionSource{}, false, errors.New("schematest: invalid prospective composition path")
	}

	wildcard := -1

	for index, token := range tokens {
		if token == "*" {
			wildcard = index

			break
		}
	}

	if wildcard < 0 || wildcard+1 != len(tokens) {
		return prospectiveCompositionSource{}, false, nil
	}

	path, exists := matchingValuePathAt(parent, pointerFromTokens(tokens[:wildcard]), 0)
	if !exists {
		return prospectiveCompositionSource{}, false, nil
	}

	container := valueAtPath(parent, path)
	if container == nil || container.kind != jsonArray && container.kind != jsonObject {
		return prospectiveCompositionSource{}, false, nil
	}

	return prospectiveCompositionSource{path: pathCopy(path), container: container}, true, nil
}

// prospectiveCompositionEdit creates one addressed insertion.
func prospectiveCompositionEdit(
	source prospectiveCompositionSource,
	coordinateRank uint64,
	value *jsonValue,
) (compositionEdit, bool) {
	switch source.container.kind {
	case jsonArray:
		if coordinateRank > 0 {
			return compositionEdit{}, false
		}

		return compositionEdit{
			path:        append(pathCopy(source.path), strconv.Itoa(len(source.container.array))),
			replacement: value, append: true,
		}, true
	case jsonObject:
		name := freshObjectMutationName(coordinateRank)
		if _, collision := source.container.object[name]; collision {
			return compositionEdit{}, false
		}

		return compositionEdit{path: append(pathCopy(source.path), name), replacement: value}, true
	default:
		return compositionEdit{}, false
	}
}

// compositionRankedSubsetCursor resumes canonical subsets across increasing ranks.
type compositionRankedSubsetCursor struct {
	source   compositionEditSource
	stream   *compositionEditSubsetMachine
	search   *search
	observed uint64
}

// newCompositionRankedSubsetCursor starts one rank-addressed subset continuation.
func newCompositionRankedSubsetCursor(
	source compositionEditSource,
	s *search,
) *compositionRankedSubsetCursor {
	return &compositionRankedSubsetCursor{
		source: source, stream: newCompositionEditSubsetMachine(source, s), search: s,
	}
}

// AttemptAtRank advances the existing subset stream instead of replaying its prefix.
func (cursor *compositionRankedSubsetCursor) AttemptAtRank(
	parent *jsonValue,
	fault faultProgram,
	rank uint64,
) (*jsonValue, bool, error) {
	if rank < cursor.observed {
		cursor.stream = newCompositionEditSubsetMachine(cursor.source, cursor.search)
		cursor.observed = 0
	}

	for {
		selected, ready, exhausted, err := cursor.stream.Advance()
		if err != nil || exhausted {
			return nil, false, err
		}

		if !ready {
			continue
		}

		if cursor.observed < rank {
			cursor.observed++

			continue
		}

		cursor.observed++
		candidate, _, err := tryCompositionEdits(parent, fault, selected, cursor.search)

		return candidate, true, err
	}
}

// visitCompositionEditSizes traverses edit subsets in increasing size and source order.
func visitCompositionEditSizes(
	edits compositionEditSource,
	s *search,
	visit func([]compositionEdit) (bool, error),
) (bool, error) {
	machine := newCompositionEditSubsetMachine(edits, s)

	for {
		selected, ready, exhausted, err := machine.Advance()
		if err != nil || exhausted {
			return false, err
		}

		if !ready {
			continue
		}

		stopped, err := visit(selected)
		if err != nil || stopped {
			return stopped, err
		}
	}
}

// compositionAssignmentEditVisitor converts each complete assignment to parent-relative edits.
func compositionAssignmentEditVisitor(
	parent *jsonValue,
	s *search,
	visit func([]compositionEdit) (bool, error),
) rowVisit {
	return func(value *jsonValue) (bool, error) {
		return visitCompositionEditSizes(compositionDifference(parent, value, nil), s, visit)
	}
}

// compositionEditCursor retains only active source traversal state.
type compositionEditCursor interface {
	Next(s *search) (compositionEdit, bool, error)
	Clone() compositionEditCursor
}

// compositionEditSource starts independent cursors without retaining an edit corpus.
type compositionEditSource struct {
	cursor func() compositionEditCursor
}

// Cursor starts one independent source traversal.
func (source compositionEditSource) Cursor() compositionEditCursor {
	return source.cursor()
}

// compositionDirectEditCursor resumes represented removals in requirement order.
type compositionDirectEditCursor struct {
	parent           *jsonValue
	requirements     []requirement
	requirementIndex int
	pathRank         uint64
}

// Next resumes at the next represented removal.
func (cursor *compositionDirectEditCursor) Next(s *search) (compositionEdit, bool, error) {
	for cursor.requirementIndex < len(cursor.requirements) {
		current := cursor.requirements[cursor.requirementIndex]
		if current.canonical || current.presence != requirementAbsent {
			cursor.requirementIndex++
			cursor.pathRank = 0

			continue
		}

		if err := s.assign(); err != nil {
			return compositionEdit{}, false, err
		}

		path, exists := matchingValuePathAt(
			cursor.parent, current.occurrence.instanceTemplate, cursor.pathRank,
		)
		if !exists {
			cursor.requirementIndex++
			cursor.pathRank = 0

			continue
		}

		cursor.pathRank++
		if len(path) == 0 || valueAtPath(cursor.parent, path) == nil {
			continue
		}

		return compositionEdit{path: pathCopy(path), remove: true}, true, nil
	}

	return compositionEdit{}, false, nil
}

// Clone copies only the current direct source coordinates.
func (cursor *compositionDirectEditCursor) Clone() compositionEditCursor {
	clone := *cursor

	return &clone
}

// compositionDirectEdits starts represented-removal traversal.
func compositionDirectEdits(parent *jsonValue, requirements []requirement) compositionEditSource {
	return compositionEditSource{cursor: func() compositionEditCursor {
		return &compositionDirectEditCursor{parent: parent, requirements: requirements}
	}}
}

// applyCompositionRequirementEdits applies directly represented absent-presence requirements.
func applyCompositionRequirementEdits(
	parent *jsonValue,
	fault faultProgram,
	s *search,
) (*jsonValue, bool, error) {
	var derivative *jsonValue

	found, err := visitCompositionEditSizes(
		compositionDirectEdits(parent, fault.requirements), s,
		func(selected []compositionEdit) (bool, error) {
			candidate, matched, candidateErr := tryCompositionEdits(parent, fault, selected, s)
			if candidateErr != nil || !matched {
				return false, candidateErr
			}

			derivative = candidate

			return true, nil
		},
	)

	return derivative, found, err
}

// tryCompositionEdits charges and verifies one transient edit set.
//
//nolint:cyclop // Atomic charging, application, and exact verification meet here.
func tryCompositionEdits(
	parent *jsonValue,
	fault faultProgram,
	edits []compositionEdit,
	s *search,
) (*jsonValue, bool, error) {
	if assignErr := s.assign(); assignErr != nil {
		return nil, false, assignErr
	}

	selectedFault, concretizeErr := concretizeProspectiveCompositionOccurrences(parent, fault, edits, s)
	if concretizeErr != nil {
		return nil, false, concretizeErr
	}

	candidate, copyErr := cloneJSONValue(parent)
	if copyErr != nil {
		return nil, false, fmt.Errorf("clone composition fault parent: %w", copyErr)
	}

	for _, edit := range edits {
		if chargeErr := chargeCompositionEdit(candidate, edit, s); chargeErr != nil {
			return nil, false, chargeErr
		}

		if applyErr := applyCompositionEdit(candidate, edit); applyErr != nil {
			if errors.Is(applyErr, errCompositionEditInapplicable) {
				return nil, false, nil
			}

			return nil, false, applyErr
		}
	}

	result := evaluate(s.model, candidate)
	if result.err != nil {
		return nil, false, fmt.Errorf("evaluate composition fault derivative: %w", result.err)
	}

	matches, matchErr := faultFailureClosureMatches(result, selectedFault)
	if matchErr != nil {
		return nil, false, fmt.Errorf("compare composition fault expected: %w", matchErr)
	}

	if result.valid || !matches {
		return nil, false, nil
	}

	return candidate, true, nil
}

// concretizeProspectiveCompositionOccurrences ranks paths created by the selected edit.
func concretizeProspectiveCompositionOccurrences(
	parent *jsonValue,
	fault faultProgram,
	edits []compositionEdit,
	s *search,
) (faultProgram, error) {
	selected := fault
	selected.expected = append(faultClosure(nil), fault.expected...)

	for index := range selected.expected {
		template := selected.expected[index].project().occurrence.instanceTemplate
		if !strings.Contains(template, "*") || matchingValuePathCount(parent, template) > 0 {
			continue
		}

		var createdPath []string

		for _, edit := range edits {
			path, exists := prospectiveCompositionPath(edit, template)
			if exists {
				createdPath = path

				break
			}
		}

		if createdPath == nil {
			continue
		}

		if err := s.assign(); err != nil {
			return faultProgram{}, err
		}

		identity := cloneEvaluationRecordIdentity(selected.expected[index])
		identity.occurrence.instance.tokens = pathCopy(createdPath)
		selected.expected[index] = identity
	}

	return selected, nil
}

// prospectiveCompositionPath resolves one mutation-created template without oracle output.
func prospectiveCompositionPath(edit compositionEdit, template string) ([]string, bool) {
	if edit.remove || edit.replacement == nil {
		return nil, false
	}

	tokens, ok := rowPointerTokens(template)
	if !ok || len(tokens) < len(edit.path) {
		return nil, false
	}

	for index := range edit.path {
		if tokens[index] != "*" && tokens[index] != edit.path[index] {
			return nil, false
		}
	}

	relativeTemplate := pointerFromTokens(tokens[len(edit.path):])

	relative, exists := matchingValuePathAt(edit.replacement, relativeTemplate, 0)
	if !exists {
		return nil, false
	}

	return append(pathCopy(edit.path), relative...), true
}

// chargeCompositionEdit charges every atomic part of one selected edit.
//
//nolint:cyclop // Presence, index, and replacement charges are intentionally explicit.
func chargeCompositionEdit(candidate *jsonValue, edit compositionEdit, s *search) error {
	if err := s.assign(); err != nil { // selected path
		return err
	}

	if len(edit.path) == 0 {
		return s.assign() // root replacement
	}

	parent := valueAtPath(candidate, edit.path[:len(edit.path)-1])
	if parent == nil {
		return nil
	}

	if edit.remove || edit.append {
		if err := s.assign(); err != nil { // presence
			return err
		}
	}

	if parent.kind == jsonArray {
		if err := s.assign(); err != nil { // selected index
			return err
		}
	}

	if !edit.remove {
		if err := s.assign(); err != nil { // replacement value
			return err
		}
	}

	return nil
}

// compositionDifference exposes one resumable depth-first edit source.
func compositionDifference(parent, assignment *jsonValue, path []string) compositionEditSource {
	return compositionEditSource{cursor: func() compositionEditCursor {
		return &compositionDifferenceCursor{stack: []compositionDifferenceFrame{{
			parent: parent, assignment: assignment, path: pathCopy(path), arrayIndex: -1,
		}}}
	}}
}

// compositionDifferenceFrame retains one active depth-first frame.
type compositionDifferenceFrame struct {
	parent      *jsonValue
	assignment  *jsonValue
	path        []string
	entered     bool
	stage       uint8
	lastName    string
	hasLastName bool
	arrayIndex  int
}

// compositionDifferenceCursor retains only active traversal frames.
type compositionDifferenceCursor struct {
	stack []compositionDifferenceFrame
}

// Next advances the live depth-first traversal until its next edit.
func (cursor *compositionDifferenceCursor) Next(s *search) (compositionEdit, bool, error) {
	for len(cursor.stack) > 0 {
		if err := s.assign(); err != nil {
			return compositionEdit{}, false, err
		}

		edit, exists := cursor.advance()
		if exists {
			return edit, true, nil
		}
	}

	return compositionEdit{}, false, nil
}

// Clone copies only active source traversal frames and paths.
func (cursor *compositionDifferenceCursor) Clone() compositionEditCursor {
	clone := &compositionDifferenceCursor{stack: make([]compositionDifferenceFrame, len(cursor.stack))}
	copy(clone.stack, cursor.stack)

	for index := range clone.stack {
		clone.stack[index].path = pathCopy(clone.stack[index].path)
	}

	return clone
}

// advance performs one charged source traversal transition.
//
//nolint:cyclop // One charged transition handles every JSON kind.
func (cursor *compositionDifferenceCursor) advance() (compositionEdit, bool) {
	frame := &cursor.stack[len(cursor.stack)-1]
	if !frame.entered {
		frame.entered = true
		if frame.parent == nil || frame.assignment == nil || frame.parent.kind != frame.assignment.kind {
			edit := compositionEdit{path: pathCopy(frame.path), replacement: frame.assignment}
			cursor.stack = cursor.stack[:len(cursor.stack)-1]

			return edit, true
		}

		if frame.parent.kind != jsonObject && frame.parent.kind != jsonArray {
			cursor.stack = cursor.stack[:len(cursor.stack)-1]

			if !jsonValuesEqual(frame.parent, frame.assignment) {
				return compositionEdit{path: pathCopy(frame.path), replacement: frame.assignment}, true
			}
		}

		return compositionEdit{}, false
	}

	switch frame.parent.kind {
	case jsonObject:
		return cursor.advanceObject(frame)
	case jsonArray:
		return cursor.advanceArray(frame)
	default:
		cursor.stack = cursor.stack[:len(cursor.stack)-1]

		return compositionEdit{}, false
	}
}

// advanceObject advances one canonical object member.
func (cursor *compositionDifferenceCursor) advanceObject(
	frame *compositionDifferenceFrame,
) (compositionEdit, bool) {
	if frame.stage == 0 {
		name, exists := nextCompositionObjectName(frame.parent.object, frame.lastName, frame.hasLastName)
		if exists {
			frame.lastName = name
			frame.hasLastName = true

			assigned, assignedExists := frame.assignment.object[name]
			if !assignedExists {
				return compositionEdit{path: append(pathCopy(frame.path), name), remove: true}, true
			}

			cursor.stack = append(cursor.stack, compositionDifferenceFrame{
				parent: frame.parent.object[name], assignment: assigned,
				path: append(pathCopy(frame.path), name), arrayIndex: -1,
			})

			return compositionEdit{}, false
		}

		frame.stage = 1
		frame.lastName = ""
		frame.hasLastName = false

		return compositionEdit{}, false
	}

	name, exists := nextCompositionObjectName(frame.assignment.object, frame.lastName, frame.hasLastName)
	if exists {
		frame.lastName = name

		frame.hasLastName = true
		if _, parentExists := frame.parent.object[name]; parentExists {
			return compositionEdit{}, false
		}

		return compositionEdit{
			path: append(pathCopy(frame.path), name), replacement: frame.assignment.object[name],
		}, true
	}

	cursor.stack = cursor.stack[:len(cursor.stack)-1]

	return compositionEdit{}, false
}

// advanceArray advances one canonical array child or tail edit.
func (cursor *compositionDifferenceCursor) advanceArray(
	frame *compositionDifferenceFrame,
) (compositionEdit, bool) {
	common := min(len(frame.parent.array), len(frame.assignment.array))
	if frame.stage == 0 {
		frame.arrayIndex++
		if frame.arrayIndex < common {
			index := frame.arrayIndex
			cursor.stack = append(cursor.stack, compositionDifferenceFrame{
				parent: frame.parent.array[index], assignment: frame.assignment.array[index],
				path: append(pathCopy(frame.path), strconv.Itoa(index)), arrayIndex: -1,
			})

			return compositionEdit{}, false
		}

		frame.stage = 1
		frame.arrayIndex = len(frame.parent.array)
	}

	if frame.stage == 1 {
		frame.arrayIndex--
		if frame.arrayIndex >= len(frame.assignment.array) {
			return compositionEdit{
				path: append(pathCopy(frame.path), strconv.Itoa(frame.arrayIndex)), remove: true,
			}, true
		}

		frame.stage = 2
		frame.arrayIndex = len(frame.parent.array) - 1
	}

	frame.arrayIndex++
	if frame.arrayIndex < len(frame.assignment.array) {
		return compositionEdit{
			path:        append(pathCopy(frame.path), strconv.Itoa(frame.arrayIndex)),
			replacement: frame.assignment.array[frame.arrayIndex], append: true,
		}, true
	}

	cursor.stack = cursor.stack[:len(cursor.stack)-1]

	return compositionEdit{}, false
}

// nextCompositionObjectName selects one canonical key without retaining a key set.
func nextCompositionObjectName(object map[string]*jsonValue, after string, hasAfter bool) (string, bool) {
	var (
		selected string
		found    bool
	)

	for name := range object {
		if hasAfter && name <= after || found && name >= selected {
			continue
		}

		selected = name
		found = true
	}

	return selected, found
}

// compositionEditSubsetCursor retains only one fixed-size combination frontier.
type compositionEditSubsetCursor struct {
	source compositionEditSource
	size   int
	levels []compositionEditSubsetLevel
	s      *search
	seen   int
}

// compositionEditSubsetLevel retains one selected edit and cloned source cursor.
type compositionEditSubsetLevel struct {
	cursor compositionEditCursor
	edit   compositionEdit
}

// newCompositionEditSubsetCursor starts one fixed-size combination frontier.
func newCompositionEditSubsetCursor(
	source compositionEditSource,
	size int,
	s *search,
) *compositionEditSubsetCursor {
	return &compositionEditSubsetCursor{source: source, size: size, s: s}
}

// Next resumes the next canonical combination.
func (cursor *compositionEditSubsetCursor) Next() ([]compositionEdit, bool, bool, error) {
	if cursor.size <= 0 {
		return nil, false, true, nil
	}

	if len(cursor.levels) == 0 {
		cursor.levels = append(cursor.levels, compositionEditSubsetLevel{cursor: cursor.source.Cursor()})
	}

	for len(cursor.levels) > 0 {
		level := &cursor.levels[len(cursor.levels)-1]

		edit, exists, err := level.cursor.Next(cursor.s)
		if err != nil {
			return nil, false, false, err
		}

		if !exists {
			cursor.levels = cursor.levels[:len(cursor.levels)-1]

			continue
		}

		level.edit = edit

		if len(cursor.levels) == 1 {
			cursor.seen++
		}

		if len(cursor.levels) < cursor.size {
			cursor.levels = append(cursor.levels, compositionEditSubsetLevel{
				cursor: level.cursor.Clone(),
			})

			continue
		}

		selected := make([]compositionEdit, len(cursor.levels))
		for index := range cursor.levels {
			selected[index] = cursor.levels[index].edit
		}

		return selected, true, false, nil
	}

	return nil, false, true, nil
}

// compositionEditSubsetMachine traverses canonical subset sizes without a corpus.
type compositionEditSubsetMachine struct {
	source compositionEditSource
	s      *search
	size   int
	count  int
	cursor *compositionEditSubsetCursor
}

// newCompositionEditSubsetMachine starts at singleton subsets.
func newCompositionEditSubsetMachine(
	source compositionEditSource,
	s *search,
) *compositionEditSubsetMachine {
	return &compositionEditSubsetMachine{source: source, s: s, size: 1}
}

// Advance resumes one canonical subset or the finite endpoint.
func (machine *compositionEditSubsetMachine) Advance() ([]compositionEdit, bool, bool, error) {
	if machine.cursor == nil {
		machine.cursor = newCompositionEditSubsetCursor(machine.source, machine.size, machine.s)
	}

	selected, ready, exhausted, err := machine.cursor.Next()
	if err != nil || !exhausted {
		return selected, ready, false, err
	}

	if machine.size == 1 {
		machine.count = machine.cursor.seen
	}

	if machine.size >= machine.count {
		return nil, false, true, nil
	}

	machine.size++
	machine.cursor = newCompositionEditSubsetCursor(machine.source, machine.size, machine.s)

	return machine.Advance()
}

// errCompositionEditInapplicable rejects a structurally incomplete edit subset.
var errCompositionEditInapplicable = errors.New("schematest: composition edit is inapplicable")

// applyCompositionEdit applies one edit to a transient parent copy.
//
//nolint:cyclop // Root, object, and array edits share one mutation boundary.
func applyCompositionEdit(root *jsonValue, edit compositionEdit) error {
	if len(edit.path) == 0 {
		if edit.remove || edit.replacement == nil {
			return errors.New("schematest: cannot remove composition root")
		}

		replacement, err := cloneJSONValue(edit.replacement)
		if err != nil {
			return err
		}

		*root = *replacement

		return nil
	}

	parent := valueAtPath(root, edit.path[:len(edit.path)-1])
	if parent == nil {
		return errors.New("schematest: composition edit parent was not found")
	}

	token := edit.path[len(edit.path)-1]
	if edit.remove {
		switch parent.kind {
		case jsonObject:
			delete(parent.object, token)
		case jsonArray:
			index, parseErr := strconv.Atoi(token)
			if parseErr != nil || index != len(parent.array)-1 {
				return errCompositionEditInapplicable
			}

			parent.array = parent.array[:index]
		default:
			return errors.New("schematest: composition removal parent is not a container")
		}

		return nil
	}

	replacement, err := cloneJSONValue(edit.replacement)
	if err != nil {
		return err
	}

	switch parent.kind {
	case jsonObject:
		parent.object[token] = replacement
	case jsonArray:
		index, parseErr := strconv.Atoi(token)
		if parseErr != nil || index < 0 {
			return errors.New("schematest: composition array edit index is invalid")
		}

		if edit.append {
			if index != len(parent.array) {
				return errCompositionEditInapplicable
			}

			parent.array = append(parent.array, replacement)
		} else {
			if index >= len(parent.array) {
				return errCompositionEditInapplicable
			}

			parent.array[index] = replacement
		}
	default:
		return errors.New("schematest: composition edit parent is not a container")
	}

	return nil
}

// pathCopy returns an independent edit path.
func pathCopy(path []string) []string {
	return append([]string(nil), path...)
}

// jsonValuesEqual compares complete internal JSON values.
func jsonValuesEqual(left, right *jsonValue) bool {
	return reflect.DeepEqual(left, right)
}

// faultNeedsCompositionSearch reports whether one fault must make an anyOf
// closure false atomically. Pure allOf branch faults use the ordinary isolated
// mutation path so unaffected branches remain untouched.
func faultNeedsCompositionSearch(fault faultProgram) bool {
	if fault.obligation.rule == oracleRuleAllOf || fault.obligation.rule == oracleRuleAnyOf {
		return true
	}

	if fault.obligation.rule != oracleRuleRequired && fault.obligation.rule != oracleRuleAdditionalProperties {
		return false
	}

	for _, failure := range fault.expected {
		if failure.rule == oracleRuleAnyOf {
			return true
		}
	}

	return false
}
