package schematest

import "errors"

// rankedArrayStructure is one transient projection/length choice.
type rankedArrayStructure struct {
	view           rowProjectionView
	active         []requirement
	length         rowArrayCount
	projectionRank uint64
	lengthRank     uint64
}

// rowProjectionAt rebuilds one projection ordinal and reports its finite endpoint.
func rowProjectionAt(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	wanted uint64,
) (rowProjectionView, bool, uint64, error) {
	cursor := newRowProjectionCursor(node, occurrence, requirements)
	defer cursor.Close()

	for ordinal := uint64(0); ; ordinal++ {
		view, ok, err := cursor.Next()
		if err != nil {
			return rowProjectionView{}, false, 0, err
		}

		if !ok {
			return rowProjectionView{}, false, ordinal, nil
		}

		if ordinal == wanted {
			view.sources = append([]rowSchemaSource(nil), view.sources...)

			return view, true, 0, nil
		}
	}
}

// rowArrayLengthAt rebuilds one emitted length ordinal through the production cursor.
func rowArrayLengthAt(
	view rowProjectionView,
	requirements []requirement,
	wanted uint64,
) (rowArrayCount, bool, uint64, error) {
	cursor, err := newRowArrayLengthCursor(view, requirements)
	if err != nil {
		return rowArrayCount{}, false, 0, err
	}

	if cursor.infeasible {
		return rowArrayCount{}, false, 0, nil
	}

	if cursor.hasExact {
		if wanted == 0 {
			return cursor.exact, true, 0, nil
		}

		return rowArrayCount{}, false, 1, nil
	}

	for ordinal := uint64(0); ; ordinal++ {
		length, ok, nextErr := cursor.Next()
		if nextErr != nil {
			return rowArrayCount{}, false, 0, nextErr
		}

		if !ok {
			return rowArrayCount{}, false, ordinal, nil
		}

		if ordinal == wanted {
			return length, true, 0, nil
		}
	}
}

// rowArrayLengthsFiniteAt proves a shared rank is beyond every finite projection length domain.
func rowArrayLengthsFiniteAt(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	projectionSize uint64,
	wanted uint64,
) (bool, error) {
	for projectionRank := uint64(0); projectionRank < projectionSize; projectionRank++ {
		view, ok, _, err := rowProjectionAt(node, occurrence, requirements, projectionRank)
		if err != nil {
			return false, err
		}

		if !ok {
			return false, errors.New("schematest: projection endpoint changed")
		}

		if !rowProjectionAcceptsKind(view, jsonArray) ||
			!rowProjectionRequirementsAcceptKind(view, requirements, jsonArray) {
			continue
		}

		_, ok, size, err := rowArrayLengthAt(view, requirements, wanted)
		if err != nil {
			return false, err
		}

		if ok || size > wanted {
			return false, nil
		}
	}

	return true, nil
}

// rowProjectionRequirementsAcceptKind checks directed kinds on every active same-instance source.
func rowProjectionRequirementsAcceptKind(
	view rowProjectionView,
	requirements []requirement,
	kind jsonKind,
) bool {
	for _, source := range view.sources {
		for _, requirement := range requirements {
			if requirement.hasKind && rowOccurrenceMatches(requirement.occurrence, source.occurrence) &&
				requirement.kind != kind {
				return false
			}
		}
	}

	return true
}

// rowArrayStructureAt enumerates projection/length tuples with the shared rank product.
//
//nolint:cyclop,gocognit,mnd,nestif // Dependent finite endpoints are proven at the shared product seam.
func (s *search) rowArrayStructureAt(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	wanted uint64,
) (rankedArrayStructure, bool, uint64, error) {
	frontier, err := newRankProductCursor(2)
	if err != nil {
		return rankedArrayStructure{}, false, 0, err
	}

	var (
		emitted        uint64
		projectionSize uint64
		projectionDone bool
	)

	for {
		ranks, ok := frontier.Next()
		if !ok {
			return rankedArrayStructure{}, false, emitted, nil
		}

		view, exists, finiteSize, viewErr := rowProjectionAt(node, occurrence, requirements, ranks[1])
		if viewErr != nil {
			return rankedArrayStructure{}, false, 0, viewErr
		}

		if !exists {
			projectionSize, projectionDone = finiteSize, true
			if setErr := frontier.SetFinite(1, finiteSize); setErr != nil {
				return rankedArrayStructure{}, false, 0, setErr
			}

			continue
		}

		active, activeErr := view.appendBranchRequirements(
			append([]requirement(nil), requirements...), s.assign,
		)
		if activeErr != nil {
			return rankedArrayStructure{}, false, 0, activeErr
		}

		if !rowProjectionAcceptsKind(view, jsonArray) ||
			!rowProjectionRequirementsAcceptKind(view, requirements, jsonArray) {
			if projectionDone {
				finite, finiteErr := rowArrayLengthsFiniteAt(
					node, occurrence, requirements, projectionSize, ranks[0],
				)
				if finiteErr != nil {
					return rankedArrayStructure{}, false, 0, finiteErr
				}

				if finite {
					if setErr := frontier.SetFinite(0, ranks[0]); setErr != nil {
						return rankedArrayStructure{}, false, 0, setErr
					}
				}
			}

			continue
		}

		length, exists, _, lengthErr := rowArrayLengthAt(view, active, ranks[0])
		if lengthErr != nil {
			return rankedArrayStructure{}, false, 0, lengthErr
		}

		if !exists {
			if projectionDone {
				finite, finiteErr := rowArrayLengthsFiniteAt(
					node, occurrence, requirements, projectionSize, ranks[0],
				)
				if finiteErr != nil {
					return rankedArrayStructure{}, false, 0, finiteErr
				}

				if finite {
					if setErr := frontier.SetFinite(0, ranks[0]); setErr != nil {
						return rankedArrayStructure{}, false, 0, setErr
					}
				}
			}

			continue
		}

		if emitted == wanted {
			return rankedArrayStructure{
				view: view, active: active, length: length,
				projectionRank: ranks[1], lengthRank: ranks[0],
			}, true, 0, nil
		}

		emitted++
	}
}

// rowSourceValueAt returns one raw source candidate before conjunction validation.
//
//nolint:cyclop // Generic, scalar-priority, and structural sources share one adapter.
func (s *search) rowSourceValueAt(
	source *rowSchemaSource,
	requirements []requirement,
	context rowSearchContext,
	wanted uint64,
) (*jsonValue, bool, uint64, error) {
	var (
		ordinal  uint64
		selected *jsonValue
	)

	visit := func(value *jsonValue) (bool, error) {
		if ordinal != wanted {
			ordinal++

			return false, nil
		}

		var err error

		selected, err = cloneJSONValue(value)

		return err == nil, err
	}

	var (
		complete bool
		err      error
	)
	if source != nil && source.node.kind == schemaString && source.node.enum == nil && wanted == 0 &&
		(source.node.format != schemaFormatNone || source.node.minLength != nil ||
			source.node.maxLength != nil || source.node.pattern != nil) {
		complete, err = s.walkActiveStringRules(
			source.node, source.occurrence, requirements, context.validRequest, visit,
		)
	} else if source != nil && wanted == 0 &&
		(source.node.kind != schemaString || len(source.node.allOf) <= 0) {
		complete, err = s.walkNode(
			source.node, source.occurrence, requirements, context,
			func(value *jsonValue) (bool, error) {
				usable, usableErr := s.rowChildValueUsable(
					source.node, source.occurrence, requirements, value,
				)
				if usableErr != nil || !usable {
					return false, usableErr
				}

				return visit(value)
			},
		)
	} else if source == nil {
		complete, err = s.walkGenericValue(requirements, visit)
	} else {
		complete, err = s.walkNode(source.node, source.occurrence, requirements, context, visit)
	}

	if err != nil {
		return nil, false, 0, err
	}

	if complete {
		return selected, true, 0, nil
	}

	return nil, false, ordinal, nil
}

// rowConjunctionValueAt returns one raw source/value rank and validates it for free.
//
//nolint:cyclop,gocognit,gocyclo,mnd,nestif // Enum and generated sources share one rank-addressable adapter.
func (s *search) rowConjunctionValueAt(
	conjunction rowSchemaConjunction,
	requirements []requirement,
	context rowSearchContext,
	wanted uint64,
) (*jsonValue, bool, bool, uint64, error) {
	hasEnum := false
	for _, source := range conjunction.sources {
		hasEnum = hasEnum || source.node.enum != nil
	}

	if hasEnum {
		ordinal := uint64(0)

		for _, source := range conjunction.sources {
			if source.node.enum == nil {
				continue
			}

			for _, member := range source.node.enum {
				if member.value == nil {
					return nil, false, false, 0, errors.New("schematest: nil structural child enum value")
				}

				if ordinal != wanted {
					ordinal++

					continue
				}

				if err := s.assign(); err != nil {
					return nil, false, false, 0, err
				}

				value, err := cloneJSONValue(member.value)
				if err != nil {
					return nil, false, false, 0, err
				}

				usable, err := s.rowConjunctionValueUsable(conjunction.sources, requirements, value)
				if err != nil {
					return nil, false, false, 0, err
				}

				return value, true, usable, 0, nil
			}
		}

		return nil, false, false, ordinal, nil
	}

	if wanted == 0 {
		for index := range conjunction.sources {
			value, exists, _, err := s.rowSourceValueAt(
				&conjunction.sources[index], requirements, context, 0,
			)
			if err != nil {
				return nil, false, false, 0, err
			}

			if !exists {
				continue
			}

			usable, err := s.rowConjunctionValueUsable(conjunction.sources, requirements, value)
			if err != nil {
				return nil, false, false, 0, err
			}

			if usable {
				return value, true, true, 0, nil
			}
		}
	}

	sourceCount := len(conjunction.sources)
	if sourceCount == 0 {
		sourceCount = 1
	}

	frontier, err := newRankProductCursor(2)
	if err != nil {
		return nil, false, false, 0, err
	}

	if err := frontier.SetFinite(0, uint64(sourceCount)); err != nil {
		return nil, false, false, 0, err
	}

	var ordinal uint64

	for {
		ranks, ok := frontier.Next()
		if !ok {
			return nil, false, false, ordinal, nil
		}

		if err := s.assign(); err != nil {
			return nil, false, false, 0, err
		}

		var source *rowSchemaSource
		if len(conjunction.sources) > 0 {
			source = &conjunction.sources[ranks[0]]
		}

		value, exists, _, valueErr := s.rowSourceValueAt(
			source, requirements, context, ranks[1],
		)
		if valueErr != nil {
			return nil, false, false, 0, valueErr
		}

		if !exists {
			allFinite := len(conjunction.sources) == 0 || sourceCount == 1
			if sourceCount > 1 {
				allFinite = true

				for index := range conjunction.sources {
					_, candidateOK, size, candidateErr := s.rowSourceValueAt(
						&conjunction.sources[index], requirements, context, ranks[1],
					)
					if candidateErr != nil {
						return nil, false, false, 0, candidateErr
					}

					if candidateOK || size > ranks[1] {
						allFinite = false

						break
					}
				}
			}

			if allFinite {
				if setErr := frontier.SetFinite(1, ranks[1]); setErr != nil {
					return nil, false, false, 0, setErr
				}
			}

			continue
		}

		if ordinal != wanted {
			ordinal++

			continue
		}

		usable, usableErr := s.rowConjunctionValueUsable(conjunction.sources, requirements, value)
		if usableErr != nil {
			return nil, false, false, 0, usableErr
		}

		return value, true, usable, 0, nil
	}
}

// rowArrayChildrenAt rebuilds one diagonal tuple while extending position state only after charge.
//
//nolint:cyclop,gocognit // Component endpoints and incremental transient reconstruction are one operation.
func (s *search) rowArrayChildrenAt(
	structure rankedArrayStructure,
	requirements []requirement,
	context rowSearchContext,
	wanted uint64,
) ([]*jsonValue, bool, bool, uint64, error) {
	if structure.length.beyond {
		for {
			if err := s.assign(); err != nil {
				return nil, false, false, 0, err
			}
		}
	}

	if structure.length.value == 0 {
		if wanted == 0 {
			return []*jsonValue{}, true, true, 0, nil
		}

		return nil, false, false, 1, nil
	}

	items := rowProjectedArrayItems(structure.view, requirements)

	var (
		ranks      []uint64
		diagonal   uint64
		finiteSize uint64
		finite     bool
		ordinal    uint64
	)

	for {
		values := []*jsonValue(nil)
		tupleExists, tupleUsable := true, true

		for position := uint64(0); position < structure.length.value; position++ {
			if err := s.assign(); err != nil {
				return nil, false, false, 0, err
			}

			if position == uint64(len(ranks)) {
				ranks = append(ranks, 0)
			}

			value, exists, usable, size, valueErr := s.rowConjunctionValueAt(
				items, requirements, context, ranks[position],
			)
			if valueErr != nil {
				return nil, false, false, 0, valueErr
			}

			if !exists {
				tupleExists = false

				if finite && finiteSize != size {
					return nil, false, false, 0, errors.New("schematest: array item rank endpoint changed")
				}

				finite, finiteSize = true, size

				break
			}

			tupleUsable = tupleUsable && usable

			values = append(values, value)
		}

		if tupleExists {
			if ordinal == wanted {
				return values, true, tupleUsable, 0, nil
			}

			ordinal++
		}

		if !advanceArrayRankTuple(ranks, &diagonal, finite, finiteSize) {
			return nil, false, false, ordinal, nil
		}
	}
}

// advanceArrayRankTuple preserves rankProductCursor order without authored-length allocation.
//
//nolint:cyclop,gocognit // Incrementing, finite bounds, and tuple-fit checks are one cursor operation.
func advanceArrayRankTuple(ranks []uint64, diagonal *uint64, finite bool, finiteSize uint64) bool {
	if len(ranks) == 0 || finite && finiteSize == 0 {
		return false
	}

	for {
		last := len(ranks) - 1
		advanced := false

		for index := last - 1; index >= 0; index-- {
			var suffix uint64
			for _, rank := range ranks[index+1:] {
				suffix += rank
			}

			if suffix == 0 {
				continue
			}

			ranks[index]++
			clear(ranks[index+1 : last])
			ranks[last] = suffix - 1
			advanced = true

			break
		}

		if !advanced {
			if *diagonal == ^uint64(0) {
				return false
			}

			if finite && finiteSize > 0 && *diagonal >= uint64(len(ranks))*(finiteSize-1) {
				return false
			}

			*diagonal++

			clear(ranks)
			ranks[last] = *diagonal
		}

		fits := true

		if finite {
			for _, rank := range ranks {
				if rank >= finiteSize {
					fits = false

					break
				}
			}
		}

		if fits {
			return true
		}
	}
}

// rowArrayChildrenFiniteAt proves one child-product rank is exhausted for every structure.
func (s *search) rowArrayChildrenFiniteAt(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	context rowSearchContext,
	structureSize uint64,
	wanted uint64,
) (uint64, bool, error) {
	var maximum uint64

	for structureRank := uint64(0); structureRank < structureSize; structureRank++ {
		structure, ok, _, err := s.rowArrayStructureAt(node, occurrence, requirements, structureRank)
		if err != nil {
			return 0, false, err
		}

		if !ok {
			return 0, false, errors.New("schematest: array structure endpoint changed")
		}

		candidateValues, ok, candidateUsable, size, err := s.rowArrayChildrenAt(
			structure, structure.active, context, wanted,
		)
		clear(candidateValues)

		_ = candidateUsable

		if err != nil {
			return 0, false, err
		}

		if ok || size > wanted {
			return 0, false, nil
		}

		if size > maximum {
			maximum = size
		}
	}

	return maximum, true, nil
}

// rowDirectArrayValueAt returns the authored full witness paired with one direct length rank.
//
//nolint:cyclop,nestif // Enum/default model order is traversed without a retained candidate slice.
func rowDirectArrayValueAt(view rowProjectionView, wanted uint64) (*jsonValue, bool, error) {
	var ordinal uint64

	for _, source := range view.sources {
		if source.node == nil || source.node.schemaShape == nil {
			return nil, false, errors.New("schematest: projected array source has no shape")
		}

		if source.node.enum != nil {
			for _, member := range source.node.enum {
				if member.value == nil {
					return nil, false, errors.New("schematest: nil projected enum value")
				}

				if member.value.kind != jsonArray {
					continue
				}

				if ordinal == wanted {
					return member.value, true, nil
				}

				ordinal++
			}
		} else if source.node.defaultValue != nil && source.node.defaultValue.kind == jsonArray {
			if ordinal == wanted {
				return source.node.defaultValue, true, nil
			}

			ordinal++
		}
	}

	return nil, false, nil
}

// walkArrayFrontier makes projection, emitted length, and child tuples one fair product.
//
//nolint:cyclop,gocognit,mnd,nestif // Selection, charging, finite proof, and mutation share one frontier.
func (s *search) walkArrayFrontier(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	context rowSearchContext,
	visit rowVisit,
) (bool, error) {
	frontier, err := newRankProductCursor(2)
	if err != nil {
		return false, err
	}

	var structureSize uint64

	for {
		ranks, ok := frontier.Next()
		if !ok {
			return false, nil
		}

		structure, exists, finiteSize, structureErr := s.rowArrayStructureAt(
			node, occurrence, requirements, ranks[0],
		)
		if structureErr != nil {
			return false, structureErr
		}

		if !exists {
			structureSize = finiteSize
			if setErr := frontier.SetFinite(0, finiteSize); setErr != nil {
				return false, setErr
			}

			continue
		}

		directExists := false

		if structure.lengthRank == 0 {
			direct, directOK, directErr := rowDirectArrayValueAt(structure.view, ranks[1])
			if directErr != nil {
				return false, directErr
			}

			if directOK {
				directExists = true

				if assignErr := s.assign(); assignErr != nil {
					return false, assignErr
				}

				owned, cloneErr := cloneJSONValue(direct)
				if cloneErr != nil {
					return false, cloneErr
				}

				complete, visitErr := visit(owned)
				if visitErr != nil || complete {
					return complete, visitErr
				}
			}
		}

		if assignErr := s.assign(); assignErr != nil {
			return false, assignErr
		}

		values, childOK, childUsable, _, childErr := s.rowArrayChildrenAt(
			structure, structure.active, context, ranks[1],
		)
		if childErr != nil {
			return false, childErr
		}

		if !childOK {
			if directExists {
				continue
			}

			if structureSize > 0 {
				finiteSize, finite, finiteErr := s.rowArrayChildrenFiniteAt(
					node, occurrence, requirements, context, structureSize, ranks[1],
				)
				if finiteErr != nil {
					return false, finiteErr
				}

				if finite {
					if setErr := frontier.SetFinite(1, finiteSize); setErr != nil {
						return false, setErr
					}
				}
			}

			continue
		}

		if !childUsable {
			continue
		}

		array := &jsonValue{kind: jsonArray, array: make([]*jsonValue, 0)}

		for _, value := range values {
			if assignErr := s.assign(); assignErr != nil {
				return false, assignErr
			}

			array.array = append(array.array, value)
		}

		complete, visitErr := visit(array)
		if visitErr != nil || complete {
			return complete, visitErr
		}
	}
}

// rankedObjectStructure is one transient projection and complete presence state.
type rankedObjectStructure struct {
	view           rowProjectionView
	shape          *rowProjectedObject
	active         []requirement
	present        []bool
	extraCount     uint64
	projectionRank uint64
	presenceRank   uint64
}

// rowObjectPresenceAt enumerates one forward presence state without recursive prefixes.
//
//nolint:cyclop // Forward feasibility and finite choice endpoints form one presence adapter.
func rowObjectPresenceAt(
	shape *rowProjectedObject,
	requirements []requirement,
	wanted uint64,
) ([]bool, uint64, bool, uint64, error) {
	if len(shape.members) == 0 {
		if wanted > 0 {
			return nil, 0, false, 1, nil
		}

		extras, feasible := projectedObjectExtraCount(shape, 0)

		return []bool{}, extras, feasible, 0, nil
	}

	frontier, err := newRankProductCursor(len(shape.members))
	if err != nil {
		return nil, 0, false, 0, err
	}

	var emitted uint64

	for {
		ranks, ok := frontier.Next()
		if !ok {
			return nil, 0, false, emitted, nil
		}

		present := make([]bool, len(shape.members))
		presentCount := uint64(0)
		remainingRequired := shape.requiredFrom(0)
		usable := true

		for index, member := range shape.members {
			if member.required {
				remainingRequired--
			}

			constraint, constrained := rowPresenceRequirementDetails(requirements, member.occurrence)

			choices := projectedMemberPresenceChoices(
				shape, member, constraint, constrained, presentCount, remainingRequired,
			)
			if ranks[index] >= uint64(len(choices)) {
				usable = false

				if setErr := frontier.SetFinite(index, uint64(len(choices))); setErr != nil {
					return nil, 0, false, 0, setErr
				}

				break
			}

			choice := choices[ranks[index]]
			if !projectedPresenceFeasible(shape, index, choice, presentCount, remainingRequired) {
				usable = false

				break
			}

			present[index] = choice
			if choice {
				presentCount++
			}
		}

		extras, feasible := projectedObjectExtraCount(shape, presentCount)
		if !usable || !feasible {
			continue
		}

		if emitted == wanted {
			return present, extras, true, 0, nil
		}

		emitted++
	}
}

// projectedObjectExtraCount computes only the selected transient repair count.
func projectedObjectExtraCount(shape *rowProjectedObject, present uint64) (uint64, bool) {
	target, beyond := shape.targetCount()
	if beyond {
		return 0, false
	}

	extras := uint64(0)
	if present < target {
		extras = target - present
	}

	if shape.requiresExtra && extras == 0 {
		extras = 1
	}

	if extras > 0 && !shape.allowsExtra || shape.hasMaximum && present+extras > shape.maximum {
		return 0, false
	}

	return extras, true
}

// rowObjectPresencesFiniteAt proves one rank is beyond every finite projection presence domain.
//
//nolint:cyclop // Kind, feasibility, and finite endpoint checks are one domain proof.
func rowObjectPresencesFiniteAt(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	projectionSize uint64,
	wanted uint64,
) (bool, error) {
	for projectionRank := uint64(0); projectionRank < projectionSize; projectionRank++ {
		view, ok, _, err := rowProjectionAt(node, occurrence, requirements, projectionRank)
		if err != nil {
			return false, err
		}

		if !ok {
			return false, errors.New("schematest: projection endpoint changed")
		}

		if !rowProjectionAcceptsKind(view, jsonObject) ||
			!rowProjectionRequirementsAcceptKind(view, requirements, jsonObject) {
			continue
		}

		active, err := view.appendBranchRequirements(append([]requirement(nil), requirements...), func() error {
			return nil
		})
		if err != nil {
			return false, err
		}

		shape, err := newRowProjectedObject(view, active, occurrence)
		if err != nil {
			return false, err
		}

		if !shape.feasible() {
			continue
		}

		_, _, ok, size, err := rowObjectPresenceAt(shape, active, wanted)
		if err != nil {
			return false, err
		}

		if ok || size > wanted {
			return false, nil
		}
	}

	return true, nil
}

// rowObjectStructureAt enumerates projection/presence tuples with the shared rank product.
//
//nolint:cyclop,gocognit,mnd,nestif // Dependent finite endpoints are proven at the shared product seam.
func (s *search) rowObjectStructureAt(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	wanted uint64,
) (rankedObjectStructure, bool, uint64, error) {
	frontier, err := newRankProductCursor(2)
	if err != nil {
		return rankedObjectStructure{}, false, 0, err
	}

	var (
		emitted        uint64
		projectionSize uint64
		projectionDone bool
	)

	for {
		ranks, ok := frontier.Next()
		if !ok {
			return rankedObjectStructure{}, false, emitted, nil
		}

		view, exists, finiteSize, viewErr := rowProjectionAt(node, occurrence, requirements, ranks[1])
		if viewErr != nil {
			return rankedObjectStructure{}, false, 0, viewErr
		}

		if !exists {
			projectionSize, projectionDone = finiteSize, true
			if setErr := frontier.SetFinite(1, finiteSize); setErr != nil {
				return rankedObjectStructure{}, false, 0, setErr
			}

			continue
		}

		active, activeErr := view.appendBranchRequirements(
			append([]requirement(nil), requirements...), s.assign,
		)
		if activeErr != nil {
			return rankedObjectStructure{}, false, 0, activeErr
		}

		if !rowProjectionAcceptsKind(view, jsonObject) ||
			!rowProjectionRequirementsAcceptKind(view, requirements, jsonObject) {
			if projectionDone {
				finite, finiteErr := rowObjectPresencesFiniteAt(
					node, occurrence, requirements, projectionSize, ranks[0],
				)
				if finiteErr != nil {
					return rankedObjectStructure{}, false, 0, finiteErr
				}

				if finite {
					if setErr := frontier.SetFinite(0, ranks[0]); setErr != nil {
						return rankedObjectStructure{}, false, 0, setErr
					}
				}
			}

			continue
		}

		shape, shapeErr := newRowProjectedObject(view, active, occurrence)
		if shapeErr != nil {
			return rankedObjectStructure{}, false, 0, shapeErr
		}

		if !shape.feasible() {
			if projectionDone {
				finite, finiteErr := rowObjectPresencesFiniteAt(
					node, occurrence, requirements, projectionSize, ranks[0],
				)
				if finiteErr != nil {
					return rankedObjectStructure{}, false, 0, finiteErr
				}

				if finite {
					if setErr := frontier.SetFinite(0, ranks[0]); setErr != nil {
						return rankedObjectStructure{}, false, 0, setErr
					}
				}
			}

			continue
		}

		present, extras, exists, _, presenceErr := rowObjectPresenceAt(shape, active, ranks[0])
		if presenceErr != nil {
			return rankedObjectStructure{}, false, 0, presenceErr
		}

		if !exists {
			if projectionDone {
				finite, finiteErr := rowObjectPresencesFiniteAt(
					node, occurrence, requirements, projectionSize, ranks[0],
				)
				if finiteErr != nil {
					return rankedObjectStructure{}, false, 0, finiteErr
				}

				if finite {
					if setErr := frontier.SetFinite(0, ranks[0]); setErr != nil {
						return rankedObjectStructure{}, false, 0, setErr
					}
				}
			}

			continue
		}

		if emitted == wanted {
			return rankedObjectStructure{
				view: view, shape: shape, active: active, present: present, extraCount: extras,
				projectionRank: ranks[1], presenceRank: ranks[0],
			}, true, 0, nil
		}

		emitted++
	}
}

// rowObjectMembersForStructure creates only the selected and already charged transient members.
func (s *search) rowObjectMembersForStructure(structure rankedObjectStructure) ([]rowMember, error) {
	members := make([]rowMember, 0)
	values := make(map[string]*jsonValue)

	for index, present := range structure.present {
		if err := s.assign(); err != nil {
			return nil, err
		}

		member := structure.shape.members[index]

		if err := s.assign(); err != nil {
			return nil, err
		}

		if present {
			members = append(members, member)
			values[member.name] = nil
		}
	}

	for extra := uint64(0); extra < structure.extraCount; extra++ {
		if err := s.assign(); err != nil {
			return nil, err
		}

		name := projectedAdditionalMemberName(structure.shape.declared, values)

		if err := s.assign(); err != nil {
			return nil, err
		}

		member, allowed, err := projectedAdditionalMember(structure.shape, name, structure.active)
		if err != nil {
			return nil, err
		}

		if !allowed {
			return nil, errors.New("schematest: selected additional object member is not allowed")
		}

		members = append(members, member)
		values[name] = nil
	}

	return members, nil
}

// rowObjectChildrenAt rebuilds one diagonal tuple in canonical member occurrence order.
//
//nolint:cyclop // Component endpoints and transient reconstruction form one product operation.
func (s *search) rowObjectChildrenAt(
	members []rowMember,
	requirements []requirement,
	context rowSearchContext,
	wanted uint64,
) ([]*jsonValue, bool, bool, uint64, error) {
	if len(members) == 0 {
		if wanted == 0 {
			return []*jsonValue{}, true, true, 0, nil
		}

		return nil, false, false, 1, nil
	}

	frontier, err := newRankProductCursor(len(members))
	if err != nil {
		return nil, false, false, 0, err
	}

	var ordinal uint64

	for {
		ranks, ok := frontier.Next()
		if !ok {
			return nil, false, false, ordinal, nil
		}

		values := make([]*jsonValue, 0, len(members))
		tupleExists, tupleUsable := true, true

		for index, rank := range ranks {
			if err := s.assign(); err != nil {
				return nil, false, false, 0, err
			}

			value, exists, usable, finiteSize, valueErr := s.rowConjunctionValueAt(
				members[index].schemas, requirements, context, rank,
			)
			if valueErr != nil {
				return nil, false, false, 0, valueErr
			}

			if !exists {
				tupleExists = false

				if setErr := frontier.SetFinite(index, finiteSize); setErr != nil {
					return nil, false, false, 0, setErr
				}

				break
			}

			tupleUsable = tupleUsable && usable

			values = append(values, value)
		}

		if !tupleExists {
			continue
		}

		if ordinal == wanted {
			return values, true, tupleUsable, 0, nil
		}

		ordinal++
	}
}

// rowObjectChildrenFiniteAt proves one child-product rank is exhausted for every structure.
func (s *search) rowObjectChildrenFiniteAt(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	context rowSearchContext,
	structureSize uint64,
	wanted uint64,
) (uint64, bool, error) {
	var maximum uint64

	for structureRank := uint64(0); structureRank < structureSize; structureRank++ {
		structure, ok, _, err := s.rowObjectStructureAt(node, occurrence, requirements, structureRank)
		if err != nil {
			return 0, false, err
		}

		if !ok {
			return 0, false, errors.New("schematest: object structure endpoint changed")
		}

		members, err := s.rowObjectMembersForStructure(structure)
		if err != nil {
			return 0, false, err
		}

		candidateValues, ok, candidateUsable, size, err := s.rowObjectChildrenAt(
			members, structure.active, context, wanted,
		)
		clear(candidateValues)

		_ = candidateUsable

		if err != nil {
			return 0, false, err
		}

		if ok || size > wanted {
			return 0, false, nil
		}

		if size > maximum {
			maximum = size
		}
	}

	return maximum, true, nil
}

// rowDirectObjectValueAt returns one complete authored object witness by rank.
//
//nolint:cyclop,nestif // Enum/default model order is traversed without a retained candidate slice.
func rowDirectObjectValueAt(view rowProjectionView, wanted uint64) (*jsonValue, bool, error) {
	var ordinal uint64

	for _, source := range view.sources {
		if source.node == nil || source.node.schemaShape == nil {
			return nil, false, errors.New("schematest: projected object source has no shape")
		}

		if source.node.enum != nil {
			for _, member := range source.node.enum {
				if member.value == nil {
					return nil, false, errors.New("schematest: nil projected enum value")
				}

				if member.value.kind != jsonObject {
					continue
				}

				if ordinal == wanted {
					return member.value, true, nil
				}

				ordinal++
			}
		} else if source.node.defaultValue != nil && source.node.defaultValue.kind == jsonObject {
			if ordinal == wanted {
				return source.node.defaultValue, true, nil
			}

			ordinal++
		}
	}

	return nil, false, nil
}

// walkObjectFrontier makes projection, presence, wildcard, and child ranks one fair frontier.
//
//nolint:cyclop,gocognit,mnd,nestif // Selection, charging, finite proof, and mutation share one frontier.
func (s *search) walkObjectFrontier(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	context rowSearchContext,
	visit rowVisit,
) (bool, error) {
	frontier, err := newRankProductCursor(2)
	if err != nil {
		return false, err
	}

	var structureSize uint64

	for {
		ranks, ok := frontier.Next()
		if !ok {
			return false, nil
		}

		structure, exists, finiteSize, structureErr := s.rowObjectStructureAt(
			node, occurrence, requirements, ranks[0],
		)
		if structureErr != nil {
			return false, structureErr
		}

		if !exists {
			structureSize = finiteSize
			if setErr := frontier.SetFinite(0, finiteSize); setErr != nil {
				return false, setErr
			}

			continue
		}

		directExists := false

		if structure.presenceRank == 0 {
			direct, directOK, directErr := rowDirectObjectValueAt(structure.view, ranks[1])
			if directErr != nil {
				return false, directErr
			}

			if directOK {
				directExists = true

				if assignErr := s.assign(); assignErr != nil {
					return false, assignErr
				}

				owned, cloneErr := cloneJSONValue(direct)
				if cloneErr != nil {
					return false, cloneErr
				}

				complete, visitErr := visit(owned)
				if visitErr != nil || complete {
					return complete, visitErr
				}
			}
		}

		members, memberErr := s.rowObjectMembersForStructure(structure)
		if memberErr != nil {
			return false, memberErr
		}

		values, childOK, childUsable, _, childErr := s.rowObjectChildrenAt(
			members, structure.active, context, ranks[1],
		)
		if childErr != nil {
			return false, childErr
		}

		if !childOK {
			if directExists {
				continue
			}

			if structureSize > 0 {
				finiteSize, finite, finiteErr := s.rowObjectChildrenFiniteAt(
					node, occurrence, requirements, context, structureSize, ranks[1],
				)
				if finiteErr != nil {
					return false, finiteErr
				}

				if finite {
					if setErr := frontier.SetFinite(1, finiteSize); setErr != nil {
						return false, setErr
					}
				}
			}

			continue
		}

		if !childUsable {
			continue
		}

		object := &jsonValue{kind: jsonObject, object: make(map[string]*jsonValue)}

		for index, member := range members {
			if assignErr := s.assign(); assignErr != nil {
				return false, assignErr
			}

			object.object[member.name] = values[index]
		}

		complete, visitErr := visit(object)
		if visitErr != nil || complete {
			return complete, visitErr
		}
	}
}
