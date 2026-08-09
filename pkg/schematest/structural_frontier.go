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

// rowArrayLengthForOrdinal directly addresses one raw named occurrence or numeric offset.
// Duplicate named and numeric occurrences are holes, so no earlier emitted rank is replayed.
//
//nolint:cyclop // Direct, fixed, and numeric phases form one address decoder.
func rowArrayLengthForOrdinal(
	view rowProjectionView,
	requirements []requirement,
	wanted uint64,
) (rowArrayCount, bool, uint64, error) {
	domain, err := newRowArrayLengthDomain(view, requirements)
	if err != nil || domain.infeasible {
		return rowArrayCount{}, false, 0, err
	}

	directCount, err := rowDirectArrayLengthCount(view)
	if err != nil {
		return rowArrayCount{}, false, 0, err
	}

	if wanted < directCount {
		candidate, exists, directErr := rowDirectArrayLengthAt(view, wanted)
		if directErr != nil || !exists {
			return rowArrayCount{}, false, 0, directErr
		}

		seen, seenErr := domain.seenBefore(candidate, arrayLengthDirect, wanted+1)

		return candidate, !seen, 0, seenErr
	}

	fixed := [...]struct {
		phase uint8
		count rowArrayCount
		set   bool
	}{
		{arrayLengthExact, domain.exact, domain.hasExact},
		{arrayLengthMinimum, domain.minimum, true},
		{arrayLengthMaximum, domain.maximum, domain.hasMaximum},
	}
	fixedRank := wanted - directCount
	fixedCount := uint64(0)

	for _, candidate := range fixed {
		if !candidate.set {
			continue
		}

		seen, seenErr := domain.seenBefore(candidate.count, candidate.phase, directCount)
		if seenErr != nil {
			return rowArrayCount{}, false, 0, seenErr
		}

		if seen {
			continue
		}

		if fixedRank == fixedCount {
			return candidate.count, true, 0, nil
		}

		fixedCount++
	}

	numericRank := fixedRank - fixedCount

	candidateValue, numericSize, exists, err := rowArrayNumericValueForRank(domain, numericRank)
	if err != nil || !exists {
		return rowArrayCount{}, false, directCount + fixedCount + numericSize, err
	}

	return rowArrayCount{value: candidateValue}, true, 0, nil
}

// rowArrayNumericValueForRank directly unranks the numeric domain around finite named exclusions.
//
//nolint:mnd // Binary search halves the directly addressed numeric domain.
func rowArrayNumericValueForRank(
	domain rowArrayLengthDomain,
	wanted uint64,
) (uint64, uint64, bool, error) {
	if domain.hasMaximum && !domain.maximum.beyond {
		excluded, err := rowArrayNamedCountAtMost(domain, domain.maximum.value)
		if err != nil {
			return 0, 0, false, err
		}

		size := domain.maximum.value + 1 - excluded
		if wanted >= size {
			return 0, size, false, nil
		}
	}

	namedCount, err := rowArrayNamedCountAtMost(domain, ^uint64(0))
	if err != nil {
		return 0, 0, false, err
	}

	high := wanted + namedCount
	if high < wanted {
		return 0, 0, false, errors.New("schematest: array length rank overflow")
	}

	low := wanted
	for low < high {
		middle := low + (high-low)/2

		excluded, countErr := rowArrayNamedCountAtMost(domain, middle)
		if countErr != nil {
			return 0, 0, false, countErr
		}

		if middle+1-excluded > wanted {
			high = middle
		} else {
			low = middle + 1
		}
	}

	return low, 0, true, nil
}

// rowArrayNamedCountAtMost counts unique finite named values without retaining them.
//
//nolint:cyclop,gocognit,nestif // Authored and fixed phases share one first-occurrence model pass.
func rowArrayNamedCountAtMost(domain rowArrayLengthDomain, maximum uint64) (uint64, error) {
	var (
		count      uint64
		directRank uint64
	)

	visit := func(candidate rowArrayCount, phase uint8, limit uint64) error {
		seen, err := domain.seenBefore(candidate, phase, limit)
		if err != nil || seen || candidate.beyond || candidate.value > maximum {
			return err
		}

		count++

		return nil
	}

	for _, source := range domain.view.sources {
		if source.node.enum != nil {
			for _, member := range source.node.enum {
				if member.value == nil {
					return 0, errors.New("schematest: nil projected enum value")
				}

				if member.value.kind != jsonArray {
					continue
				}

				directRank++
				if err := visit(
					rowArrayCount{value: uint64(len(member.value.array))}, arrayLengthDirect, directRank,
				); err != nil {
					return 0, err
				}
			}
		} else if source.node.defaultValue != nil && source.node.defaultValue.kind == jsonArray {
			directRank++
			if err := visit(
				rowArrayCount{value: uint64(len(source.node.defaultValue.array))},
				arrayLengthDirect,
				directRank,
			); err != nil {
				return 0, err
			}
		}
	}

	fixed := [...]struct {
		phase uint8
		count rowArrayCount
		set   bool
	}{
		{arrayLengthExact, domain.exact, domain.hasExact},
		{arrayLengthMinimum, domain.minimum, true},
		{arrayLengthMaximum, domain.maximum, domain.hasMaximum},
	}
	for _, candidate := range fixed {
		if candidate.set {
			if err := visit(candidate.count, candidate.phase, directRank); err != nil {
				return 0, err
			}
		}
	}

	return count, nil
}

// rowDirectArrayLengthCount returns the finite authored occurrence count without decoding candidates.
func rowDirectArrayLengthCount(view rowProjectionView) (uint64, error) {
	var count uint64

	for _, source := range view.sources {
		if source.node == nil || source.node.schemaShape == nil {
			return 0, errors.New("schematest: projected array source has no shape")
		}

		if source.node.enum != nil {
			for _, member := range source.node.enum {
				if member.value == nil {
					return 0, errors.New("schematest: nil projected enum value")
				}

				if member.value.kind == jsonArray {
					count++
				}
			}
		} else if source.node.defaultValue != nil && source.node.defaultValue.kind == jsonArray {
			count++
		}
	}

	return count, nil
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

// rowSourceValueForRank directly decodes one primitive kind/value choice.
// Generated rule searches occupy one rank and are entered once without discarding a prefix.
//
//nolint:cyclop // Scalar and recursive structural kinds share one direct decoder.
func (s *search) rowSourceValueForRank(
	source *rowSchemaSource,
	requirements []requirement,
	context rowSearchContext,
	wanted uint64,
) (*jsonValue, bool, uint64, error) {
	if wanted == 0 && (source == nil || source.node.kind != schemaString || len(source.node.allOf) == 0) {
		return s.rowSourceFirstValue(source, requirements, context)
	}

	if wanted > 0 {
		wanted--
	}

	if source == nil {
		return canonicalGenericValueAt(wanted)
	}

	if source.node == nil || source.node.schemaShape == nil {
		return nil, false, 0, errors.New("schematest: structural child source has no shape")
	}

	kinds, err := rowKindChoices(source.node, source.occurrence, requirements)
	if err != nil || len(kinds) == 0 {
		return nil, false, 0, err
	}

	kindRank, valueRank, ok := rowSourceValueRanksAtOrdinal(uint64(len(kinds)), wanted)
	if !ok {
		return nil, false, 0, nil
	}

	if err := s.chargeNodeCompositions(source.node, source.occurrence, requirements); err != nil {
		return nil, false, 0, err
	}

	if err := s.assign(); err != nil {
		return nil, false, 0, err
	}

	kind := kinds[kindRank]
	if kindIsScalar(kind) {
		return s.rowScalarValueForRank(source, requirements, context, kind, valueRank)
	}

	switch kind {
	case jsonArray:
		return s.rowArrayValueForRank(source, requirements, context, valueRank)
	case jsonObject:
		return s.rowObjectValueForRank(source, requirements, context, valueRank)
	default:
		return nil, false, 0, errors.New("schematest: unsupported structural child kind")
	}
}

// rowSourceFirstValue resumes one source traversal only for its first value choice.
//
//nolint:cyclop // Generic, scalar-rule, and node sources share one first-choice boundary.
func (s *search) rowSourceFirstValue(
	source *rowSchemaSource,
	requirements []requirement,
	context rowSearchContext,
) (*jsonValue, bool, uint64, error) {
	var selected *jsonValue

	visit := func(value *jsonValue) (bool, error) {
		var err error

		selected, err = cloneJSONValue(value)

		return err == nil, err
	}

	var (
		complete bool
		err      error
	)

	switch {
	case source == nil:
		complete, err = s.walkGenericValue(requirements, visit)
	case source.node.kind == schemaString && source.node.enum == nil &&
		(source.node.format != schemaFormatNone || source.node.minLength != nil ||
			source.node.maxLength != nil || source.node.pattern != nil):
		complete, err = s.walkActiveStringRules(
			source.node, source.occurrence, requirements, context.validRequest, visit,
		)
	default:
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
	}

	if err != nil {
		return nil, false, 0, err
	}

	return selected, complete, 0, nil
}

// rowScalarValueForRank directly selects a canonical scalar or one generated-rule search.
func (s *search) rowScalarValueForRank(
	source *rowSchemaSource,
	requirements []requirement,
	context rowSearchContext,
	kind jsonKind,
	wanted uint64,
) (*jsonValue, bool, uint64, error) {
	candidate, exists, finiteSize, err := canonicalKindValueAt(kind, wanted)
	if err != nil {
		return nil, false, 0, err
	}

	if exists {
		usable, usableErr := rowScalarValueUsable(candidate, source.node, kind)
		if usableErr != nil || !usable {
			return nil, false, finiteSize, usableErr
		}

		owned, cloneErr := cloneJSONValue(candidate)

		return owned, cloneErr == nil, finiteSize, cloneErr
	}

	if wanted != finiteSize || kind != jsonString && kind != jsonNumber {
		return nil, false, finiteSize, nil
	}

	var selected *jsonValue

	visit := func(value *jsonValue) (bool, error) {
		var cloneErr error

		selected, cloneErr = cloneJSONValue(value)

		return cloneErr == nil, cloneErr
	}

	var complete bool
	if kind == jsonString {
		complete, err = s.walkActiveStringRules(
			source.node, source.occurrence, requirements, context.validRequest, visit,
		)
	} else {
		complete, err = s.walkActiveNumberRules(
			source.node, source.occurrence, requirements, context.validRequest, visit,
		)
	}

	if err != nil {
		return nil, false, 0, err
	}

	return selected, complete, finiteSize + 1, nil
}

// canonicalGenericValueAt directly decodes the finite generic kind/witness product.
func canonicalGenericValueAt(wanted uint64) (*jsonValue, bool, uint64, error) {
	kinds := canonicalJSONKinds()

	kindRank, valueRank, ok := rowSourceValueRanksAtOrdinal(uint64(len(kinds)), wanted)
	if !ok {
		return nil, false, 0, nil
	}

	return canonicalKindValueAt(kinds[kindRank], valueRank)
}

// canonicalKindValueAt directly addresses one locked primitive alternative.
//
//nolint:cyclop,mnd // Locked witness counts and indices define this direct decoder.
func canonicalKindValueAt(kind jsonKind, wanted uint64) (*jsonValue, bool, uint64, error) {
	var (
		candidate *jsonValue
		size      uint64
	)

	switch kind {
	case jsonNull:
		size = 1

		if wanted == 0 {
			candidate = &jsonValue{kind: jsonNull}
		}
	case jsonBoolean:
		size = 2
		if wanted < size {
			candidate = &jsonValue{kind: jsonBoolean, boolean: wanted == 1}
		}
	case jsonNumber:
		numbers := [...]string{"-1", "0", "0.5", "1", "2", "3"}

		size = uint64(len(numbers))
		if wanted < size {
			number, err := parseExactNumber(numbers[wanted])
			if err != nil {
				return nil, false, 0, err
			}

			candidate = &jsonValue{kind: jsonNumber, number: number}
		}
	case jsonString:
		strings := [...]string{"", "a", "b", "text"}

		size = uint64(len(strings))
		if wanted < size {
			candidate = &jsonValue{kind: jsonString, text: strings[wanted]}
		}
	case jsonArray:
		size = 4
		if wanted < size {
			candidate = &jsonValue{kind: jsonArray, array: []*jsonValue{}}

			switch wanted {
			case 1:
				candidate.array = []*jsonValue{{kind: jsonBoolean}}
			case 2:
				candidate.array = []*jsonValue{{kind: jsonString, text: "a"}}
			case 3:
				number, err := parseExactNumber("0")
				if err != nil {
					return nil, false, 0, err
				}

				candidate.array = []*jsonValue{{kind: jsonNumber, number: number}}
			}
		}
	case jsonObject:
		size = 2
		if wanted < size {
			candidate = &jsonValue{kind: jsonObject, object: map[string]*jsonValue{}}
			if wanted == 1 {
				candidate.object["a"] = &jsonValue{kind: jsonString, text: "a"}
			}
		}
	default:
		return nil, false, 0, errors.New("schematest: unknown canonical child kind")
	}

	return candidate, candidate != nil, size, nil
}

// rowConjunctionValueAt directly decodes authored source/member indices or a primitive source rank.
//
//nolint:cyclop,gocognit,nestif // Finite enum and generated sources share one selection boundary.
func (s *search) rowConjunctionValueAt(
	conjunction rowSchemaConjunction,
	requirements []requirement,
	context rowSearchContext,
	wanted uint64,
) (*jsonValue, bool, bool, uint64, error) {
	enumSources := uint64(0)

	for _, source := range conjunction.sources {
		if source.node == nil || source.node.schemaShape == nil {
			return nil, false, false, 0, errors.New("schematest: structural child source has no shape")
		}

		if source.node.enum != nil {
			enumSources++
		}
	}

	if enumSources > 0 {
		sourceRank, memberRank, ok := rowSourceValueRanksAtOrdinal(enumSources, wanted)
		if !ok {
			return nil, false, false, 0, nil
		}

		var selected *rowSchemaSource

		for index := range conjunction.sources {
			if conjunction.sources[index].node.enum == nil {
				continue
			}

			if sourceRank == 0 {
				selected = &conjunction.sources[index]

				break
			}

			sourceRank--
		}

		if selected == nil {
			return nil, false, false, 0, errors.New("schematest: enum source rank is out of range")
		}

		if memberRank >= uint64(len(selected.node.enum)) {
			return nil, false, false, uint64(len(selected.node.enum)), nil
		}

		member := selected.node.enum[memberRank]
		if member.value == nil {
			return nil, false, false, 0, errors.New("schematest: nil structural child enum value")
		}

		if err := s.assign(); err != nil {
			return nil, false, false, 0, err
		}

		value, err := cloneJSONValue(member.value)
		if err != nil {
			return nil, false, false, 0, err
		}

		usable, err := s.rowConjunctionValueUsable(conjunction.sources, requirements, value)

		return value, true, usable, uint64(len(selected.node.enum)), err
	}

	if wanted == 0 {
		for index := range conjunction.sources {
			value, exists, _, err := s.rowSourceValueForRank(
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
		return nil, false, false, 0, nil
	}

	if err := s.assign(); err != nil {
		return nil, false, false, 0, err
	}

	var source *rowSchemaSource
	if len(conjunction.sources) > 0 {
		source = &conjunction.sources[sourceRank]
	}

	value, exists, finiteSize, err := s.rowSourceValueForRank(
		source, requirements, context, valueRank,
	)
	if err != nil || !exists {
		return nil, false, false, finiteSize, err
	}

	if value == nil {
		return nil, false, false, 0, errors.New("schematest: direct structural child rank returned nil")
	}

	usable, err := s.rowConjunctionValueUsable(conjunction.sources, requirements, value)

	return value, true, usable, finiteSize, err
}

// rowArrayValueForRank directly evaluates one nested array frontier tuple.
//
//nolint:cyclop,mnd // Three primitive ranks and charged decoding form one operation.
func (s *search) rowArrayValueForRank(
	source *rowSchemaSource,
	requirements []requirement,
	context rowSearchContext,
	wanted uint64,
) (*jsonValue, bool, uint64, error) {
	if source.node.defaultValue != nil && source.node.defaultValue.kind == jsonArray {
		if wanted == 0 {
			if err := s.assign(); err != nil {
				return nil, false, 0, err
			}

			owned, err := cloneJSONValue(source.node.defaultValue)

			return owned, err == nil, 0, err
		}

		wanted--
	}

	decoder, ok := newDirectRankTupleDecoder(3, wanted)
	if !ok {
		return nil, false, 0, nil
	}

	projectionRank, exists := decoder.Next()
	if !exists {
		return nil, false, 0, errors.New("schematest: nested array projection rank ended early")
	}

	lengthRank, exists := decoder.Next()
	if !exists {
		return nil, false, 0, errors.New("schematest: nested array length rank ended early")
	}

	childRank, exists := decoder.Next()
	if !exists {
		return nil, false, 0, errors.New("schematest: nested array child rank ended early")
	}

	view, exists, err := rowProjectionAt(source.node, source.occurrence, requirements, projectionRank)
	if err != nil || !exists {
		return nil, false, 0, err
	}

	active, err := view.appendBranchRequirements(append([]requirement(nil), requirements...), s.assign)
	if err != nil {
		return nil, false, 0, err
	}

	if !rowProjectionAcceptsKind(view, jsonArray) ||
		!rowProjectionRequirementsAcceptKind(view, requirements, jsonArray) ||
		rowProjectionHasExactCount(view, active) && (childRank != 0 || lengthRank != 0) {
		return nil, false, 0, nil
	}

	candidate, candidateExists, _, err := s.rowArrayProjectionCandidate(
		view, active, context, childRank, lengthRank,
	)
	if err != nil || !candidateExists {
		return nil, false, 0, err
	}

	return candidate, true, 0, nil
}

// rowObjectValueForRank directly evaluates one nested object frontier tuple.
//
//nolint:cyclop,mnd // Three primitive ranks and charged decoding form one operation.
func (s *search) rowObjectValueForRank(
	source *rowSchemaSource,
	requirements []requirement,
	context rowSearchContext,
	wanted uint64,
) (*jsonValue, bool, uint64, error) {
	if source.node.defaultValue != nil && source.node.defaultValue.kind == jsonObject {
		if wanted == 0 {
			if err := s.assign(); err != nil {
				return nil, false, 0, err
			}

			owned, err := cloneJSONValue(source.node.defaultValue)

			return owned, err == nil, 0, err
		}

		wanted--
	}

	decoder, ok := newDirectRankTupleDecoder(3, wanted)
	if !ok {
		return nil, false, 0, nil
	}

	projectionRank, exists := decoder.Next()
	if !exists {
		return nil, false, 0, errors.New("schematest: nested object projection rank ended early")
	}

	presenceRank, exists := decoder.Next()
	if !exists {
		return nil, false, 0, errors.New("schematest: nested object presence rank ended early")
	}

	childRank, exists := decoder.Next()
	if !exists {
		return nil, false, 0, errors.New("schematest: nested object child rank ended early")
	}

	view, exists, err := rowProjectionAt(source.node, source.occurrence, requirements, projectionRank)
	if err != nil || !exists {
		return nil, false, 0, err
	}

	active, err := view.appendBranchRequirements(append([]requirement(nil), requirements...), s.assign)
	if err != nil {
		return nil, false, 0, err
	}

	if !rowProjectionAcceptsKind(view, jsonObject) ||
		!rowProjectionRequirementsAcceptKind(view, requirements, jsonObject) {
		return nil, false, 0, nil
	}

	shape, err := newRowProjectedObject(view, active, source.occurrence)
	if err != nil || !shape.feasible() {
		return nil, false, 0, err
	}

	candidate, candidateExists, _, err := s.rowObjectProjectionCandidate(
		view, shape, active, context, childRank, presenceRank,
	)
	if err != nil || !candidateExists {
		return nil, false, 0, err
	}

	return candidate, true, 0, nil
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

	array := &jsonValue{kind: jsonArray, array: make([]*jsonValue, 0)}

	for _, value := range values {
		if err := s.assign(); err != nil {
			return nil, false, false, err
		}

		array.array = append(array.array, value)
	}

	return array, true, usable, nil
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

	object := &jsonValue{kind: jsonObject, object: make(map[string]*jsonValue)}

	for index, member := range members {
		if err := s.assign(); err != nil {
			return nil, false, false, err
		}

		object.object[member.name] = values[index]
	}

	return object, true, usable, nil
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

			if candidateExists {
				diagonalLive = true
			}

			if !candidateExists {
				continue
			}

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
