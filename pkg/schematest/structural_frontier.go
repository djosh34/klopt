package schematest

import "errors"

// rankedArrayStructure is one transient projection/length choice.
type rankedArrayStructure struct {
	view   rowProjectionView
	active []requirement
	length rowArrayCount
}

// liveProjectionFrontier owns the one resumable projection traversal for a structural search.
type liveProjectionFrontier struct {
	cursor     *rowProjectionCursor
	ordinal    uint64
	finiteSize uint64
	exhausted  bool
}

// newLiveProjectionFrontier starts one structural projection stream.
func newLiveProjectionFrontier(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
) *liveProjectionFrontier {
	return &liveProjectionFrontier{cursor: newRowProjectionCursor(node, occurrence, requirements)}
}

// Next advances exactly once and records the endpoint only on natural exhaustion.
func (frontier *liveProjectionFrontier) Next() (rowProjectionView, bool, error) {
	if frontier == nil || frontier.cursor == nil {
		return rowProjectionView{}, false, errors.New("schematest: live projection frontier is not initialized")
	}

	if frontier.exhausted {
		return rowProjectionView{}, false, nil
	}

	view, ok, err := frontier.cursor.Next()
	if err != nil {
		return rowProjectionView{}, false, err
	}

	if !ok {
		frontier.exhausted = true
		frontier.finiteSize = frontier.ordinal

		return rowProjectionView{}, false, nil
	}

	frontier.ordinal++

	return view, true, nil
}

// Close releases the suspended traversal.
func (frontier *liveProjectionFrontier) Close() {
	if frontier != nil && frontier.cursor != nil {
		frontier.cursor.Close()
	}
}

// directRankTupleDecoder resumes one directly addressed canonical tuple component by component.
type directRankTupleDecoder struct {
	dimensions int
	dimension  int
	remaining  uint64
	ordinal    uint64
}

// newDirectRankTupleDecoder directly addresses one tuple without replaying earlier tuples.
//
//nolint:mnd // Saturating exponential and binary searches decode one ordinal without prefix replay.
func newDirectRankTupleDecoder(dimensions int, wanted uint64) (*directRankTupleDecoder, bool) {
	if dimensions <= 0 {
		return nil, false
	}

	diagonal := uint64(0)

	low, high := uint64(0), uint64(1)
	for saturatedRankTupleCount(high, dimensions) <= wanted && high < ^uint64(0) {
		low = high + 1
		if high > (^uint64(0)-1)/2 {
			high = ^uint64(0)
		} else {
			high = high*2 + 1
		}
	}

	for low <= high {
		middle := low + (high-low)/2
		if saturatedRankTupleCount(middle, dimensions) <= wanted {
			low = middle + 1
		} else {
			diagonal = middle
			if middle == 0 {
				break
			}

			high = middle - 1
		}
	}

	before := uint64(0)
	if diagonal > 0 {
		before = saturatedRankTupleCount(diagonal-1, dimensions)
	}

	return &directRankTupleDecoder{
		dimensions: dimensions,
		remaining:  diagonal,
		ordinal:    wanted - before,
	}, true
}

// Next returns the next component of the directly addressed tuple.
func (decoder *directRankTupleDecoder) Next() (uint64, bool) {
	if decoder == nil || decoder.dimension >= decoder.dimensions {
		return 0, false
	}

	if decoder.dimension+1 == decoder.dimensions {
		decoder.dimension++

		return decoder.remaining, true
	}

	for rank := uint64(0); rank <= decoder.remaining; rank++ {
		count := saturatedWeakCompositionCount(
			decoder.remaining-rank, decoder.dimensions-decoder.dimension-1,
		)
		if decoder.ordinal < count {
			decoder.remaining -= rank
			decoder.dimension++

			return rank, true
		}

		decoder.ordinal -= count
	}

	return 0, false
}

// saturatedRankTupleCount counts tuples through one diagonal without overflow.
func saturatedRankTupleCount(diagonal uint64, dimensions int) uint64 {
	if diagonal > ^uint64(0)-uint64(dimensions) {
		return ^uint64(0)
	}

	return saturatedBinomial(diagonal+uint64(dimensions), uint64(dimensions))
}

// saturatedWeakCompositionCount counts tuples on exactly one diagonal.
func saturatedWeakCompositionCount(sum uint64, dimensions int) uint64 {
	if dimensions == 1 {
		return 1
	}

	addend := uint64(dimensions) - 1
	if sum > ^uint64(0)-addend {
		return ^uint64(0)
	}

	return saturatedBinomial(sum+addend, addend)
}

// saturatedBinomial computes one binomial coefficient without uint64 wraparound.
func saturatedBinomial(total uint64, selected uint64) uint64 {
	if selected > total {
		return 0
	}

	if total-selected < selected {
		selected = total - selected
	}

	result := uint64(1)

	for index := uint64(1); index <= selected; index++ {
		factor := total - selected + index
		if result > ^uint64(0)/factor {
			return ^uint64(0)
		}

		result = result * factor / index
	}

	return result
}

// rowSourceValueRanksAtOrdinal decodes a source-bounded source/value product tuple.
//
//nolint:mnd // Binary search directly locates one source-bounded diagonal.
func rowSourceValueRanksAtOrdinal(sourceCount uint64, wanted uint64) (uint64, uint64, bool) {
	if sourceCount == 0 {
		return 0, 0, false
	}

	low, high := uint64(0), wanted
	for low <= high {
		middle := low + (high-low)/2
		if saturatedSourceValueTupleCount(sourceCount, middle) <= wanted {
			low = middle + 1
		} else if middle == 0 {
			break
		} else {
			high = middle - 1
		}
	}

	diagonal := low

	before := uint64(0)
	if diagonal > 0 {
		before = saturatedSourceValueTupleCount(sourceCount, diagonal-1)
	}

	sourceRank := wanted - before
	if sourceRank >= sourceCount || sourceRank > diagonal {
		return 0, 0, false
	}

	return sourceRank, diagonal - sourceRank, true
}

// saturatedSourceValueTupleCount counts source-bounded tuples through one diagonal.
//
//nolint:mnd // Triangular and rectangular tuple counts use their defining factor.
func saturatedSourceValueTupleCount(sourceCount uint64, diagonal uint64) uint64 {
	if diagonal < sourceCount {
		left := diagonal + 1
		if left > ^uint64(0)/(diagonal+2) {
			return ^uint64(0)
		}

		return left * (diagonal + 2) / 2
	}

	if sourceCount == ^uint64(0) {
		return ^uint64(0)
	}

	if sourceCount > ^uint64(0)/(sourceCount+1) {
		return ^uint64(0)
	}

	base := sourceCount * (sourceCount + 1) / 2

	extra := diagonal - sourceCount + 1
	if extra > (^uint64(0)-base)/sourceCount {
		return ^uint64(0)
	}

	return base + extra*sourceCount
}

// rowArrayLengthForOrdinal directly decodes one emitted length rank from finite named guidance.
//
//nolint:cyclop,gocognit,gocyclo // Named guidance deduplication and numeric unranking form one decoder.
func rowArrayLengthForOrdinal(
	view rowProjectionView,
	requirements []requirement,
	wanted uint64,
) (rowArrayCount, bool, uint64, error) {
	cursor, err := newRowArrayLengthCursor(view, requirements)
	if err != nil || cursor.infeasible {
		return rowArrayCount{}, false, 0, err
	}

	emitted := uint64(0)
	namedCount := uint64(0)

	visitNamed := func(candidate rowArrayCount, phase uint8, directRank uint64) (bool, error) {
		cursor.directRank = directRank

		seen, seenErr := cursor.seenBefore(candidate, phase)
		if seenErr != nil || seen {
			return false, seenErr
		}

		namedCount++

		if emitted == wanted {
			return true, nil
		}

		emitted++

		return false, nil
	}

	for rank := uint64(0); ; rank++ {
		candidate, exists, directErr := rowDirectArrayLengthAt(view, rank)
		if directErr != nil {
			return rowArrayCount{}, false, 0, directErr
		}

		if !exists {
			break
		}

		selected, visitErr := visitNamed(candidate, arrayLengthDirect, rank+1)
		if visitErr != nil {
			return rowArrayCount{}, false, 0, visitErr
		}

		if selected {
			return candidate, true, 0, nil
		}
	}

	fixed := []struct {
		phase uint8
		count rowArrayCount
		set   bool
	}{
		{arrayLengthExact, cursor.exact, cursor.hasExact},
		{arrayLengthMinimum, cursor.minimum, true},
		{arrayLengthMaximum, cursor.maximum, cursor.hasMaximum},
	}
	for _, candidate := range fixed {
		if !candidate.set {
			continue
		}

		selected, visitErr := visitNamed(candidate.count, candidate.phase, ^uint64(0))
		if visitErr != nil {
			return rowArrayCount{}, false, 0, visitErr
		}

		if selected {
			return candidate.count, true, 0, nil
		}
	}

	numericRank := wanted - emitted
	candidateValue := numericRank

	for iteration := uint64(0); iteration <= namedCount; iteration++ {
		beforeOrEqual := uint64(0)
		countNamed := func(candidate rowArrayCount, phase uint8, directRank uint64) error {
			cursor.directRank = directRank

			seen, seenErr := cursor.seenBefore(candidate, phase)
			if seenErr != nil || seen || candidate.beyond || candidate.value > candidateValue {
				return seenErr
			}

			beforeOrEqual++

			return nil
		}

		for rank := uint64(0); ; rank++ {
			direct, exists, directErr := rowDirectArrayLengthAt(view, rank)
			if directErr != nil {
				return rowArrayCount{}, false, 0, directErr
			}

			if !exists {
				break
			}

			if countErr := countNamed(direct, arrayLengthDirect, rank+1); countErr != nil {
				return rowArrayCount{}, false, 0, countErr
			}
		}

		for _, named := range fixed {
			if named.set {
				if countErr := countNamed(named.count, named.phase, ^uint64(0)); countErr != nil {
					return rowArrayCount{}, false, 0, countErr
				}
			}
		}

		if ^uint64(0)-numericRank < beforeOrEqual {
			return rowArrayCount{}, false, 0, errors.New("schematest: array length rank overflow")
		}

		next := numericRank + beforeOrEqual
		if next == candidateValue {
			break
		}

		candidateValue = next
	}

	candidate := rowArrayCount{value: candidateValue}
	if cursor.hasMaximum {
		comparison, compareErr := rowArrayCountsCompare(candidate, cursor.maximum)
		if compareErr != nil {
			return rowArrayCount{}, false, 0, compareErr
		}

		if comparison > 0 {
			return rowArrayCount{}, false, wanted, nil
		}
	}

	return candidate, true, 0, nil
}

// rowProjectionHasExactCount reports whether this view has directed array count guidance.
func rowProjectionHasExactCount(view rowProjectionView, requirements []requirement) bool {
	for _, source := range view.sources {
		for _, requirement := range requirements {
			if requirement.tag == requirementExactCount &&
				rowOccurrenceMatches(requirement.occurrence, source.occurrence) {
				return true
			}
		}
	}

	return false
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
//nolint:cyclop,gocognit,nestif // Enum and generated sources share one rank-addressable adapter.
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

	sourceRank, valueRank, ok := rowSourceValueRanksAtOrdinal(uint64(sourceCount), wanted)
	if !ok {
		return nil, false, false, wanted, nil
	}

	if err := s.assign(); err != nil {
		return nil, false, false, 0, err
	}

	var source *rowSchemaSource
	if len(conjunction.sources) > 0 {
		source = &conjunction.sources[sourceRank]
	}

	value, exists, finiteSize, valueErr := s.rowSourceValueAt(
		source, requirements, context, valueRank,
	)
	if valueErr != nil || !exists {
		return nil, false, false, finiteSize, valueErr
	}

	usable, usableErr := s.rowConjunctionValueUsable(conjunction.sources, requirements, value)
	if usableErr != nil {
		return nil, false, false, 0, usableErr
	}

	return value, true, usable, 0, nil
}

// rowArrayChildrenForOrdinal rebuilds one diagonal tuple while extending position state only after charge.
//
//nolint:cyclop // Component endpoints and incremental transient reconstruction are one operation.
func (s *search) rowArrayChildrenForOrdinal(
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

	if structure.length.value > uint64(^uint(0)>>1) {
		return nil, false, false, 0, errors.New("schematest: array item rank dimension overflow")
	}

	decoder, ok := newDirectRankTupleDecoder(int(structure.length.value), wanted)
	if !ok {
		return nil, false, false, 0, nil
	}

	items := rowProjectedArrayItems(structure.view, requirements)
	values := []*jsonValue(nil)
	usable := true

	for position := uint64(0); position < structure.length.value; position++ {
		if err := s.assign(); err != nil {
			return nil, false, false, 0, err
		}

		rank, rankExists := decoder.Next()
		if !rankExists {
			return nil, false, false, 0, errors.New("schematest: array item rank tuple ended early")
		}

		value, exists, valueUsable, size, valueErr := s.rowConjunctionValueAt(
			items, requirements, context, rank,
		)
		if valueErr != nil || !exists {
			return nil, false, false, size, valueErr
		}

		usable = usable && valueUsable

		values = append(values, value)
	}

	return values, true, usable, 0, nil
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

// rowArrayProjectionCandidate builds exactly one shared-frontier length and child tuple.
func (s *search) rowArrayProjectionCandidate(
	view rowProjectionView,
	active []requirement,
	context rowSearchContext,
	childRank uint64,
	lengthRank uint64,
) (*jsonValue, bool, bool, error) {
	length, exists, _, err := rowArrayLengthForOrdinal(view, active, lengthRank)
	if err != nil || !exists {
		return nil, false, false, err
	}

	if assignErr := s.assign(); assignErr != nil {
		return nil, false, false, assignErr
	}

	structure := rankedArrayStructure{view: view, active: active, length: length}

	values, childExists, usable, _, err := s.rowArrayChildrenForOrdinal(
		structure, active, context, childRank,
	)
	if err != nil || !childExists {
		return nil, false, false, err
	}

	if !usable {
		clear(values)

		return nil, true, false, nil
	}

	array := &jsonValue{kind: jsonArray, array: make([]*jsonValue, 0)}

	for _, value := range values {
		if err := s.assign(); err != nil {
			return nil, false, false, err
		}

		array.array = append(array.array, value)
	}

	return array, true, true, nil
}

// walkArrayFrontier decodes one ephemeral projection for each shared structural rank tuple.
//
//nolint:cyclop,gocognit,gocyclo,mnd // One frontier owns projection, source, length, and child ranks.
func (s *search) walkArrayFrontier(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	context rowSearchContext,
	visit rowVisit,
) (bool, error) {
	frontier, err := newRankProductCursor(5)
	if err != nil {
		return false, err
	}

	if err := frontier.SetFinite(3, 2); err != nil {
		return false, err
	}

	var diagonal uint64

	diagonalLive := true

	for {
		ranks, ok := frontier.Next()
		if !ok {
			return false, nil
		}

		if frontier.diagonal != diagonal {
			if !diagonalLive {
				return false, nil
			}

			diagonal = frontier.diagonal
			diagonalLive = false
		}

		view, exists, decodeErr := rowProjectionAt(node, occurrence, requirements, ranks[4])
		if decodeErr != nil {
			return false, decodeErr
		}

		if !exists {
			continue
		}

		if ranks[4] == diagonal {
			diagonalLive = true
		}

		directTuple := ranks[3] == 0 && ranks[0] == 0 && ranks[1] == 0

		constructedTuple := ranks[3] == 1 && ranks[2] == 0
		if !directTuple && !constructedTuple {
			continue
		}

		active, activeErr := view.appendBranchRequirements(
			append([]requirement(nil), requirements...), s.assign,
		)
		if activeErr != nil {
			return false, activeErr
		}

		if !rowProjectionAcceptsKind(view, jsonArray) ||
			!rowProjectionRequirementsAcceptKind(view, requirements, jsonArray) {
			continue
		}

		switch ranks[3] {
		case 0:
			if ranks[0] != 0 || ranks[1] != 0 {
				continue
			}

			witness, witnessExists, directErr := rowDirectArrayValueAt(view, ranks[2])
			if directErr != nil {
				return false, directErr
			}

			if !witnessExists {
				continue
			}

			diagonalLive = true

			if err := s.assign(); err != nil {
				return false, err
			}

			owned, cloneErr := cloneJSONValue(witness)
			if cloneErr != nil {
				return false, cloneErr
			}

			complete, visitErr := visit(owned)
			if visitErr != nil || complete {
				return complete, visitErr
			}
		case 1:
			if ranks[2] != 0 || rowProjectionHasExactCount(view, active) && (ranks[0] != 0 || ranks[1] != 0) {
				continue
			}

			candidate, candidateExists, usable, candidateErr := s.rowArrayProjectionCandidate(
				view, active, context, ranks[0], ranks[1],
			)
			if candidateErr != nil {
				return false, candidateErr
			}

			if !candidateExists {
				continue
			}

			diagonalLive = true

			if !usable {
				continue
			}

			complete, visitErr := visit(candidate)
			if visitErr != nil || complete {
				return complete, visitErr
			}
		}
	}
}

// rankedObjectStructure is one transient projection and complete presence state.
type rankedObjectStructure struct {
	view       rowProjectionView
	shape      *rowProjectedObject
	active     []requirement
	present    []bool
	extraCount uint64
}

// rowObjectPresenceForOrdinal enumerates one forward presence state without recursive prefixes.
func rowObjectPresenceForOrdinal(
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

	decoder, ok := newDirectRankTupleDecoder(len(shape.members), wanted)
	if !ok {
		return nil, 0, false, 0, nil
	}

	present := make([]bool, 0)
	presentCount := uint64(0)
	remainingRequired := shape.requiredFrom(0)

	for index, member := range shape.members {
		if member.required {
			remainingRequired--
		}

		constraint, constrained := rowPresenceRequirementDetails(requirements, member.occurrence)

		choices := projectedMemberPresenceChoices(
			shape, member, constraint, constrained, presentCount, remainingRequired,
		)

		rank, rankExists := decoder.Next()
		if !rankExists || rank >= uint64(len(choices)) {
			return nil, 0, false, wanted, nil
		}

		choice := choices[rank]
		if !projectedPresenceFeasible(shape, index, choice, presentCount, remainingRequired) {
			return nil, 0, false, wanted, nil
		}

		present = append(present, choice)
		if choice {
			presentCount++
		}
	}

	extras, feasible := projectedObjectExtraCount(shape, presentCount)

	return present, extras, feasible, 0, nil
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

// rowObjectChildrenForOrdinal rebuilds one diagonal tuple in canonical member occurrence order.
func (s *search) rowObjectChildrenForOrdinal(
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

	decoder, ok := newDirectRankTupleDecoder(len(members), wanted)
	if !ok {
		return nil, false, false, 0, nil
	}

	values := make([]*jsonValue, 0, len(members))
	usable := true

	for index := range members {
		if err := s.assign(); err != nil {
			return nil, false, false, 0, err
		}

		rank, rankExists := decoder.Next()
		if !rankExists {
			return nil, false, false, 0, errors.New("schematest: object child rank tuple ended early")
		}

		value, exists, valueUsable, finiteSize, valueErr := s.rowConjunctionValueAt(
			members[index].schemas, requirements, context, rank,
		)
		if valueErr != nil || !exists {
			return nil, false, false, finiteSize, valueErr
		}

		usable = usable && valueUsable

		values = append(values, value)
	}

	return values, true, usable, 0, nil
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

// rowObjectProjectionCandidate builds exactly one shared-frontier presence and child tuple.
func (s *search) rowObjectProjectionCandidate(
	view rowProjectionView,
	shape *rowProjectedObject,
	active []requirement,
	context rowSearchContext,
	childRank uint64,
	presenceRank uint64,
) (*jsonValue, bool, bool, error) {
	present, extras, exists, _, err := rowObjectPresenceForOrdinal(shape, active, presenceRank)
	if err != nil || !exists {
		return nil, false, false, err
	}

	structure := rankedObjectStructure{
		view: view, shape: shape, active: active, present: present, extraCount: extras,
	}

	members, err := s.rowObjectMembersForStructure(structure)
	if err != nil {
		return nil, false, false, err
	}

	values, childExists, usable, _, err := s.rowObjectChildrenForOrdinal(
		members, active, context, childRank,
	)
	if err != nil || !childExists {
		return nil, false, false, err
	}

	if !usable {
		clear(values)

		return nil, true, false, nil
	}

	object := &jsonValue{kind: jsonObject, object: make(map[string]*jsonValue)}

	for index, member := range members {
		if err := s.assign(); err != nil {
			return nil, false, false, err
		}

		object.object[member.name] = values[index]
	}

	return object, true, true, nil
}

// walkObjectFrontier decodes one ephemeral projection for each shared structural rank tuple.
//
//nolint:cyclop,gocognit,gocyclo,mnd // One frontier owns projection, source, presence, and child ranks.
func (s *search) walkObjectFrontier(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	context rowSearchContext,
	visit rowVisit,
) (bool, error) {
	frontier, err := newRankProductCursor(5)
	if err != nil {
		return false, err
	}

	if err := frontier.SetFinite(3, 2); err != nil {
		return false, err
	}

	var diagonal uint64

	diagonalLive := true

	for {
		ranks, ok := frontier.Next()
		if !ok {
			return false, nil
		}

		if frontier.diagonal != diagonal {
			if !diagonalLive {
				return false, nil
			}

			diagonal = frontier.diagonal
			diagonalLive = false
		}

		view, exists, decodeErr := rowProjectionAt(node, occurrence, requirements, ranks[4])
		if decodeErr != nil {
			return false, decodeErr
		}

		if !exists {
			continue
		}

		if ranks[4] == diagonal {
			diagonalLive = true
		}

		directTuple := ranks[3] == 0 && ranks[0] == 0 && ranks[1] == 0

		constructedTuple := ranks[3] == 1 && ranks[2] == 0
		if !directTuple && !constructedTuple {
			continue
		}

		active, activeErr := view.appendBranchRequirements(
			append([]requirement(nil), requirements...), s.assign,
		)
		if activeErr != nil {
			return false, activeErr
		}

		if !rowProjectionAcceptsKind(view, jsonObject) ||
			!rowProjectionRequirementsAcceptKind(view, requirements, jsonObject) {
			continue
		}

		shape, shapeErr := newRowProjectedObject(view, active, occurrence)
		if shapeErr != nil {
			return false, shapeErr
		}

		if !shape.feasible() {
			continue
		}

		switch ranks[3] {
		case 0:
			if ranks[0] != 0 || ranks[1] != 0 {
				continue
			}

			witness, witnessExists, directErr := rowDirectObjectValueAt(view, ranks[2])
			if directErr != nil {
				return false, directErr
			}

			if !witnessExists {
				continue
			}

			diagonalLive = true

			if err := s.assign(); err != nil {
				return false, err
			}

			owned, cloneErr := cloneJSONValue(witness)
			if cloneErr != nil {
				return false, cloneErr
			}

			complete, visitErr := visit(owned)
			if visitErr != nil || complete {
				return complete, visitErr
			}
		case 1:
			if ranks[2] != 0 {
				continue
			}

			candidate, candidateExists, usable, candidateErr := s.rowObjectProjectionCandidate(
				view, shape, active, context, ranks[0], ranks[1],
			)
			if candidateErr != nil {
				return false, candidateErr
			}

			if !candidateExists {
				continue
			}

			diagonalLive = true

			if !usable {
				continue
			}

			complete, visitErr := visit(candidate)
			if visitErr != nil || complete {
				return complete, visitErr
			}
		}
	}
}
