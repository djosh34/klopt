package schematest

import (
	"errors"
	"math/big"
	"unicode/utf16"
)

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

// rowDirectWitnessUpperBound counts directly authored witnesses anywhere below one node.
//
//nolint:cyclop // Enum/default phases and saturating recursion share one metadata count.
func rowDirectWitnessUpperBound(node *schemaNode, kind jsonKind) uint64 {
	if node == nil {
		return 0
	}

	count := uint64(0)

	if node.enum != nil {
		for _, member := range node.enum {
			if member.value != nil && member.value.kind == kind {
				if count == ^uint64(0) {
					return count
				}

				count++
			}
		}
	} else if node.defaultValue != nil && node.defaultValue.kind == kind {
		count++
	}

	for _, child := range node.allOf {
		childCount := rowDirectWitnessUpperBound(child, kind)
		if childCount > ^uint64(0)-count {
			return ^uint64(0)
		}

		count += childCount
	}

	for _, child := range node.anyOf {
		childCount := rowDirectWitnessUpperBound(child, kind)
		if childCount > ^uint64(0)-count {
			return ^uint64(0)
		}

		count += childCount
	}

	return count
}

// rowArrayLengthForOrdinal decodes explicit phase/source/member-or-offset components.
// Authored addresses form one source/member rectangle; holes are never compressed or replayed.
//
//nolint:cyclop // Authored, fixed, and numeric tuple phases are intentionally explicit.
func rowArrayLengthForOrdinal(
	view rowProjectionView,
	requirements []requirement,
	wanted uint64,
) (rowArrayCount, bool, uint64, error) {
	domain, err := newRowArrayLengthDomain(view, requirements)
	if err != nil || domain.infeasible {
		return rowArrayCount{}, false, 0, err
	}

	sourceCount, memberCount, authored, err := rowDirectArrayLengthAddressShape(view)
	if err != nil {
		return rowArrayCount{}, false, 0, err
	}

	directSize := uint64(0)

	if authored {
		if memberCount > ^uint64(0)/sourceCount {
			return rowArrayCount{}, false, 0, errors.New("schematest: array length address overflow")
		}

		directSize = sourceCount * memberCount
	}

	if wanted < directSize {
		sourceIndex := wanted % sourceCount
		memberIndex := wanted / sourceCount

		return rowArrayLengthForAddress(
			view, requirements, uint64(arrayLengthDirect), sourceIndex, memberIndex,
		)
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
	fixedRank := wanted - directSize
	fixedCount := uint64(0)

	for _, address := range fixed {
		if !address.set {
			continue
		}

		seen, seenErr := domain.seenBefore(address.count, address.phase)
		if seenErr != nil {
			return rowArrayCount{}, false, 0, seenErr
		}

		if seen {
			continue
		}

		if fixedRank == fixedCount {
			return address.count, true, 0, nil
		}

		fixedCount++
	}

	return rowArrayLengthForAddress(
		view, requirements, uint64(arrayLengthRemaining), 0, fixedRank-fixedCount,
	)
}

// rowDirectArrayLengthAddressShape returns the finite authored source/member rectangle.
func rowDirectArrayLengthAddressShape(view rowProjectionView) (uint64, uint64, bool, error) {
	sourceCount := uint64(len(view.sources))
	memberCount := uint64(0)
	authored := false

	for _, source := range view.sources {
		if source.node == nil || source.node.schemaShape == nil {
			return 0, 0, false, errors.New("schematest: projected array source has no shape")
		}

		if source.node.enum != nil {
			memberCount = max(memberCount, uint64(len(source.node.enum)))
			for _, member := range source.node.enum {
				if member.value == nil {
					return 0, 0, false, errors.New("schematest: nil projected enum value")
				}

				authored = authored || member.value.kind == jsonArray
			}
		} else if source.node.defaultValue != nil {
			memberCount = max(memberCount, 1)
			authored = authored || source.node.defaultValue.kind == jsonArray
		}
	}

	return sourceCount, memberCount, authored, nil
}

// rowArrayLengthForAddress evaluates one explicit authored or numeric length address.
//
//nolint:cyclop // The explicit phase cases are the address model.
func rowArrayLengthForAddress(
	view rowProjectionView,
	requirements []requirement,
	phase uint64,
	sourceIndex uint64,
	memberOrOffset uint64,
) (rowArrayCount, bool, uint64, error) {
	domain, err := newRowArrayLengthDomain(view, requirements)
	if err != nil || domain.infeasible {
		return rowArrayCount{}, false, 0, err
	}

	if phase > uint64(arrayLengthRemaining) {
		return rowArrayCount{}, false, uint64(arrayLengthRemaining) + 1, nil
	}

	if phase == uint64(arrayLengthDirect) {
		candidate, exists, directErr := rowDirectArrayLengthAt(view, sourceIndex, memberOrOffset)
		if directErr != nil || !exists {
			return rowArrayCount{}, false, uint64(len(view.sources)), directErr
		}

		seen, seenErr := rowDirectArrayLengthSeenBefore(
			view, candidate, sourceIndex, memberOrOffset,
		)

		return candidate, !seen, uint64(len(view.sources)), seenErr
	}

	if sourceIndex != 0 {
		return rowArrayCount{}, false, 1, nil
	}

	var (
		candidate rowArrayCount
		set       bool
	)

	switch uint8(phase) {
	case arrayLengthExact:
		candidate, set = domain.exact, domain.hasExact
	case arrayLengthMinimum:
		candidate, set = domain.minimum, true
	case arrayLengthMaximum:
		candidate, set = domain.maximum, domain.hasMaximum
	case arrayLengthRemaining:
		candidate, set = rowArrayCount{value: memberOrOffset}, true
	}

	if !set {
		return rowArrayCount{}, false, 0, nil
	}

	if uint8(phase) == arrayLengthRemaining {
		if domain.hasMaximum && !domain.maximum.beyond && candidate.value > domain.maximum.value {
			return rowArrayCount{}, false, domain.maximum.value + 1, nil
		}

		seen, seenErr := domain.seenBefore(candidate, arrayLengthRemaining)

		return candidate, !seen, 0, seenErr
	}

	if memberOrOffset != 0 {
		return rowArrayCount{}, false, 1, nil
	}

	seen, seenErr := domain.seenBefore(candidate, uint8(phase))

	return candidate, !seen, 1, seenErr
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
		return canonicalGenericValueAt(0)
	case source.node.kind == schemaString && source.node.enum == nil &&
		(source.node.format != schemaFormatNone || source.node.minLength != nil ||
			source.node.maxLength != nil || source.node.pattern != nil):
		direct, exists, directErr := s.rowGeneratedStringValueAt(source, requirements, context, 0)

		return direct, exists, 0, directErr
	case source.node.kind == schemaNumber || source.node.kind == schemaInteger:
		direct, exists, directErr := s.rowGeneratedNumberValueAt(source, requirements, context, 0)

		return direct, exists, 0, directErr
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
//
//nolint:cyclop // Canonical and generated scalar phases share one rank boundary.
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

	if kind != jsonString && kind != jsonNumber {
		return nil, false, finiteSize, nil
	}

	if kind == jsonString && !nodeHasStringSearchRules(source.node) ||
		kind == jsonNumber && !nodeHasNumberObjective(source.node) {
		return nil, false, finiteSize, nil
	}

	generatedRank := wanted - finiteSize
	if kind == jsonNumber || kind == jsonString && len(source.node.allOf) == 0 &&
		(source.node.format != schemaFormatNone || source.node.minLength != nil ||
			source.node.maxLength != nil || source.node.pattern != nil) {
		generatedRank++
		if generatedRank == 0 {
			return nil, false, 0, errors.New("schematest: generated scalar rank overflow")
		}
	}

	if kind == jsonString {
		generatedCandidate, generated, generatedErr := s.rowGeneratedStringValueAt(
			source, requirements, context, generatedRank,
		)

		return generatedCandidate, generated, 0, generatedErr
	}

	generatedCandidate, generated, generatedErr := s.rowGeneratedNumberValueAt(
		source, requirements, context, generatedRank,
	)

	return generatedCandidate, generated, 0, generatedErr
}

// nodeHasStringSearchRules reports whether a nested scalar source owns generated string work.
func nodeHasStringSearchRules(node *schemaNode) bool {
	if node == nil {
		return false
	}

	if node.format != schemaFormatNone || node.minLength != nil || node.maxLength != nil || node.pattern != nil {
		return true
	}

	for _, child := range node.allOf {
		if nodeHasStringSearchRules(child) {
			return true
		}
	}

	for _, child := range node.anyOf {
		if nodeHasStringSearchRules(child) {
			return true
		}
	}

	return false
}

// rowGeneratedStringValueAt directly evaluates one length/unit/transition tuple.
//
//nolint:cyclop,gocognit,gocyclo,mnd // Compilation and exact primitive tuple evaluation share one adapter.
func (s *search) rowGeneratedStringValueAt(
	source *rowSchemaSource,
	requirements []requirement,
	context rowSearchContext,
	wanted uint64,
) (*jsonValue, bool, error) {
	rules, err := activeStringRulesFor(source.node, source.occurrence, requirements, nil)
	if err != nil || !rules.supported {
		return nil, false, err
	}

	patterns := make([]*patternAST, 0, len(rules.patterns))
	for _, pattern := range rules.patterns {
		patterns = append(patterns, pattern.pattern)
	}

	lengths, err := basicStringLengthsFromActive(rules.lengths)
	if err != nil {
		return nil, false, err
	}

	if len(patterns) == 0 && len(rules.formats) == 0 && len(lengths.boundaries) == 0 {
		return nil, false, nil
	}

	product, err := newBasicStringProduct(patterns)
	if err != nil {
		return nil, false, err
	}

	if addErr := product.addFormats(rules.formats, -1); addErr != nil {
		return nil, false, addErr
	}

	maximumSamples := uint64(0)
	for _, format := range rules.formats {
		maximumSamples = max(
			maximumSamples, uint64(len(simpleStringFormatWitnesses(format.format, true))),
		)
	}

	formatSampleSize := uint64(len(rules.formats)) * maximumSamples
	if wanted < formatSampleSize {
		formatRank := wanted % uint64(len(rules.formats))
		sampleRank := wanted / uint64(len(rules.formats))

		samples := simpleStringFormatWitnesses(rules.formats[formatRank].format, true)
		if sampleRank >= uint64(len(samples)) {
			return nil, false, nil
		}

		if assignErr := s.assign(); assignErr != nil {
			return nil, false, assignErr
		}

		return &jsonValue{kind: jsonString, text: samples[sampleRank]}, true, nil
	}

	wanted -= formatSampleSize

	seedNode := source.node
	seedPointer := source.occurrence.usePointer
	rule := oracleRulePattern
	level := oracleStringValidLevel

	if objective := validStringObjective(context.validRequest, source.node, source.occurrence); objective != nil {
		seedNode = objective.node
		seedPointer = objective.identity.occurrence.usePointer
		rule = objective.identity.rule
		level = objective.identity.level
	}

	canonicalSchemaJSON, err := marshalStrict(seedNode.schemaJSON)
	if err != nil {
		return nil, false, err
	}

	decoder, ok := newDirectRankTupleDecoder(3, wanted)
	if !ok {
		return nil, false, nil
	}

	lengthRank, _ := decoder.Next()
	unitOffset, _ := decoder.Next()
	transitionCode, _ := decoder.Next()

	runeLength, exists := rowStringLengthAt(lengths, product, lengthRank)
	if !exists || !lengths.allows(runeLength) || !product.formatsAllowLength(runeLength) {
		return nil, false, nil
	}

	if assignErr := s.assign(); assignErr != nil {
		return nil, false, assignErr
	}

	maximumUnits := runeLength
	if product.hasSurrogate {
		if runeLength > uint64(^uint(0)>>1)/basicStringUnitsPerRune {
			return nil, false, nil
		}

		maximumUnits = runeLength * basicStringUnitsPerRune
	}

	unitLength := runeLength + unitOffset
	if unitLength < runeLength || unitLength > maximumUnits || unitLength > uint64(^uint(0)>>1) {
		return nil, false, nil
	}

	seed := searchSeed(seedPointer, canonicalSchemaJSON, rule, level)
	state := product.start(int(unitLength))
	units := make([]uint16, 0, int(unitLength))
	includePadding := product.needsPadding || !product.unbounded && unitLength > product.maxUnits

	for position := uint64(0); position < unitLength; position++ {
		count := uint64(0)

		product.eachTransition(state, seed, includePadding, func(uint16) bool {
			count++

			return false
		})

		if count == 0 {
			return nil, false, nil
		}

		choice := transitionCode % count
		transitionCode /= count

		index := uint64(0)
		selected := uint16(0)
		found := false

		product.eachTransition(state, seed, includePadding, func(unit uint16) bool {
			if index == choice {
				selected = unit
				found = true

				return true
			}

			index++

			return false
		})

		if !found {
			return nil, false, nil
		}

		if assignErr := s.assign(); assignErr != nil {
			return nil, false, assignErr
		}

		state = product.advance(state, selected, int(position), int(unitLength))
		if !product.viable(state) {
			return nil, false, nil
		}

		units = append(units, selected)
	}

	if transitionCode != 0 || !product.accepting(state) || !validBasicStringUnits(units) ||
		uint64(len(utf16.Decode(units))) != runeLength {
		return nil, false, nil
	}

	candidate := string(utf16.Decode(units))

	return &jsonValue{kind: jsonString, text: candidate}, true, nil
}

// rowStringLengthAt directly indexes a boundary occurrence or numeric length.
//
//nolint:cyclop // Boundary and numeric phases share direct first-occurrence checks.
func rowStringLengthAt(
	lengths basicStringLengths,
	product *basicStringProduct,
	wanted uint64,
) (uint64, bool) {
	if wanted < uint64(len(lengths.boundaries)) {
		candidate := lengths.boundaries[wanted]
		for _, earlier := range lengths.boundaries[:wanted] {
			if earlier == candidate {
				return 0, false
			}
		}

		return candidate, true
	}

	candidate := wanted - uint64(len(lengths.boundaries))
	for _, boundary := range lengths.boundaries {
		if boundary == candidate {
			return 0, false
		}
	}

	maximum, bounded := uint64(0), false
	if !product.unbounded {
		maximum, bounded = product.maxUnits, true
	}

	if lengths.hasMaximum && (!bounded || lengths.maximum < maximum) {
		maximum, bounded = lengths.maximum, true
	}

	return candidate, !bounded || candidate <= maximum
}

// rowGeneratedNumberValueAt directly evaluates one deterministic edge or seeded primitive tuple.
//
//nolint:cyclop // Deterministic and seeded phases form one direct adapter.
func (s *search) rowGeneratedNumberValueAt(
	source *rowSchemaSource,
	requirements []requirement,
	context rowSearchContext,
	wanted uint64,
) (*jsonValue, bool, error) {
	rules := make([]activeNumberRule, 0)

	falseBranchObjective := false
	if err := collectActiveNumberRules(
		source.node, source.occurrence, requirements, &rules, &falseBranchObjective,
	); err != nil {
		return nil, false, err
	}

	schedule, err := newNumberSchedule(rules)
	if err != nil {
		return nil, false, err
	}

	schedule.seeded = schedule.seeded || falseBranchObjective && !schedule.hasEnum

	edge, exists, edgeCount, err := rowNumberEdgeAt(schedule, wanted, nil)
	if err != nil {
		return nil, false, err
	}

	if exists {
		if assignErr := s.assign(); assignErr != nil {
			return nil, false, assignErr
		}

		number, materializeErr := edge.materialize()
		if materializeErr != nil {
			return nil, false, materializeErr
		}

		return &jsonValue{kind: jsonNumber, number: number}, true, nil
	}

	if !schedule.seeded || wanted < edgeCount {
		return nil, false, nil
	}

	seedNode := source.node
	seedPointer := source.occurrence.usePointer
	rule := oracleRuleType
	level := jsonKindName(jsonNumber)

	if target := validNumberObjective(context.validRequest, source.node, source.occurrence); target != nil {
		seedNode = target.node
		seedPointer = target.identity.occurrence.usePointer
		rule = target.identity.rule
		level = target.identity.level
	}

	if seedNode.schemaJSON == nil {
		return nil, false, errors.New("schematest: number search schema has no canonical JSON")
	}

	canonicalSchemaJSON, err := marshalStrict(seedNode.schemaJSON)
	if err != nil {
		return nil, false, err
	}

	candidate, exists, err := s.rowSeededNumberValueAt(
		searchSeed(seedPointer, canonicalSchemaJSON, rule, level), wanted-edgeCount,
	)
	if err != nil || !exists {
		return nil, false, err
	}

	replayed, err := schedule.containsNumber(candidate.number)
	if err != nil || replayed {
		return nil, false, err
	}

	return candidate, true, nil
}

// rowNumberEdgeAt directly selects one first-occurrence deterministic address.
func rowNumberEdgeAt(
	schedule numberSchedule,
	wanted uint64,
	evaluate func(numberEdgeAddress),
) (numberEdge, bool, uint64, error) {
	address, addressed, domain, err := schedule.edgeAddressAt(wanted)
	if err != nil || !addressed {
		return numberEdge{}, false, domain, err
	}

	selected, exists, err := schedule.edgeAtAddress(address, evaluate)
	if err != nil || !exists {
		return numberEdge{}, false, domain, err
	}

	duplicate, err := schedule.edgeAddressDuplicate(selected, wanted)
	if err != nil || duplicate {
		return numberEdge{}, false, domain, err
	}

	return selected, true, domain, nil
}

// rowSeededNumberValueAt directly decodes length, exponent, signs, and coefficient digits.
//
//nolint:cyclop,mnd // The five primitive numeric choices are evaluated together.
func (s *search) rowSeededNumberValueAt(seed uint64, wanted uint64) (*jsonValue, bool, error) {
	decoder, ok := newDirectRankTupleDecoder(5, wanted)
	if !ok {
		return nil, false, nil
	}

	lengthOffset, _ := decoder.Next()
	radius, _ := decoder.Next()
	exponentChoice, _ := decoder.Next()
	signChoice, _ := decoder.Next()
	digitsCode, _ := decoder.Next()

	if exponentChoice > 1 || radius == 0 && exponentChoice != 0 || signChoice > 1 {
		return nil, false, nil
	}

	length := lengthOffset + 1
	if length == 0 || length > uint64(^uint(0)>>1) {
		return nil, false, nil
	}

	if assignErr := s.assign(); assignErr != nil {
		return nil, false, assignErr
	}

	exponent := new(big.Int).SetUint64(radius)
	if radius > 0 && (exponentChoice == 1) != (seed&1 == 1) {
		exponent.Neg(exponent)
	}

	if assignErr := s.assign(); assignErr != nil {
		return nil, false, assignErr
	}

	sign := int64(1)
	if (signChoice == 1) != (seed>>1&1 == 1) {
		sign = -1
	}

	digits := make([]byte, int(length))
	for index := range digits {
		count := uint64(decimalRadix)
		base := byte('0')

		if index == 0 {
			count = 9
			base = '1'
		}

		choice := digitsCode % count
		digitsCode /= count
		digits[index] = base + byte((seed%count+choice)%count)

		if assignErr := s.assign(); assignErr != nil {
			return nil, false, assignErr
		}
	}

	if digitsCode != 0 || len(digits) > 1 && digits[len(digits)-1] == '0' {
		return nil, false, nil
	}

	coefficient := new(big.Int)
	if _, ok := coefficient.SetString(string(digits), decimalRadix); !ok {
		return nil, false, errors.New("schematest: seeded number has invalid digits")
	}

	if sign < 0 {
		coefficient.Neg(coefficient)
	}

	number, err := newExactNumber(coefficient, big.NewInt(1), exponent, big.NewInt(0))
	if err != nil {
		return nil, false, err
	}

	return &jsonValue{kind: jsonNumber, number: number}, true, nil
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
	hasEnum := false

	for _, source := range conjunction.sources {
		if source.node == nil || source.node.schemaShape == nil {
			return nil, false, false, 0, errors.New("schematest: structural child source has no shape")
		}

		hasEnum = hasEnum || source.node.enum != nil
	}

	if hasEnum {
		sourceRank, memberRank, ok := rowSourceValueRanksAtOrdinal(
			uint64(len(conjunction.sources)), wanted,
		)
		if !ok {
			return nil, false, false, 0, nil
		}

		selected := &conjunction.sources[sourceRank]

		maximumMembers := uint64(0)
		for index := range conjunction.sources {
			maximumMembers = max(maximumMembers, uint64(len(conjunction.sources[index].node.enum)))
		}

		if selected.node.enum == nil || memberRank >= uint64(len(selected.node.enum)) {
			lastDiagonal := uint64(len(conjunction.sources)) - 1 + maximumMembers - 1

			finiteSize := saturatedSourceValueTupleCount(
				uint64(len(conjunction.sources)), lastDiagonal,
			)
			if wanted < finiteSize {
				return &jsonValue{kind: jsonNull}, true, false, finiteSize, nil
			}

			return nil, false, false, finiteSize, nil
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
	if err != nil {
		return nil, false, false, finiteSize, err
	}

	if !exists {
		return nil, false, false, finiteSize, nil
	}

	if value == nil {
		return nil, false, false, 0, errors.New("schematest: direct structural child rank returned nil")
	}

	usable, err := s.rowConjunctionValueUsable(conjunction.sources, requirements, value)

	return value, true, usable, finiteSize, err
}

// rowArrayItemsHaveEnum reports whether one active item source has an enum.
func rowArrayItemsHaveEnum(view rowProjectionView, requirements []requirement) bool {
	items := rowProjectedArrayItems(view, requirements)
	for index := range items.sources {
		if items.sources[index].node.enum != nil {
			return true
		}
	}

	return false
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

	itemsHaveEnum := rowArrayItemsHaveEnum(view, active)
	if !rowProjectionAcceptsKind(view, jsonArray) ||
		!rowProjectionRequirementsAcceptKind(view, requirements, jsonArray) ||
		rowProjectionHasExactCount(view, active) &&
			(lengthRank != 0 || childRank != 0 && !itemsHaveEnum) {
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

// rowArrayLengthUsable checks the selected count against the active intersection.
func rowArrayLengthUsable(
	view rowProjectionView,
	requirements []requirement,
	candidate rowArrayCount,
) (bool, error) {
	domain, err := newRowArrayLengthDomain(view, requirements)
	if err != nil || domain.infeasible {
		return false, err
	}

	comparison, err := rowArrayCountsCompare(candidate, domain.minimum)
	if err != nil || comparison < 0 {
		return false, err
	}

	if domain.hasMaximum {
		comparison, err = rowArrayCountsCompare(candidate, domain.maximum)
		if err != nil || comparison > 0 {
			return false, err
		}
	}

	if domain.hasExact {
		return rowArrayCountsEqual(candidate, domain.exact)
	}

	return true, nil
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

	lengthUsable, err := rowArrayLengthUsable(view, active, length)
	if err != nil {
		return nil, false, false, err
	}

	usable = usable && lengthUsable

	array := &jsonValue{kind: jsonArray, array: make([]*jsonValue, 0)}

	for _, value := range values {
		if err := s.assign(); err != nil {
			return nil, false, false, err
		}

		array.array = append(array.array, value)
	}

	return array, true, usable, nil
}

// rowPackedArrayFrontierCeiling returns the last potentially live packed length diagonal.
//
//nolint:cyclop,nestif // Finite authored and numeric endpoints share one conservative ceiling.
func rowPackedArrayFrontierCeiling(view rowProjectionView, requirements []requirement) (uint64, error) {
	domain, err := newRowArrayLengthDomain(view, requirements)
	if err != nil {
		return 0, err
	}

	sourceCount, memberCount, authored, err := rowDirectArrayLengthAddressShape(view)
	if err != nil {
		return 0, err
	}

	directSize := uint64(0)

	if authored {
		if memberCount > ^uint64(0)/sourceCount {
			return ^uint64(0), nil
		}

		directSize = sourceCount * memberCount
	}

	if domain.hasExact {
		if domain.exact.beyond {
			return ^uint64(0), nil
		}

		if domain.exact.value == 0 {
			return max(directSize, 1), nil
		}

		items := rowProjectedArrayItems(view, requirements)
		maximumMembers := uint64(0)

		hasEnum := false
		for index := range items.sources {
			hasEnum = hasEnum || items.sources[index].node.enum != nil
			maximumMembers = max(maximumMembers, uint64(len(items.sources[index].node.enum)))
		}

		if !hasEnum {
			return max(directSize, 1), nil
		}

		lastDiagonal := uint64(len(items.sources)) - 1 + maximumMembers - 1
		childValueSize := saturatedSourceValueTupleCount(uint64(len(items.sources)), lastDiagonal)

		childDiagonal := domain.exact.value * (childValueSize - 1)
		if childValueSize == 0 || childDiagonal/domain.exact.value != childValueSize-1 {
			return ^uint64(0), nil
		}

		if domain.exact.value > uint64(^uint(0)>>1) {
			return ^uint64(0), nil
		}

		childCeiling := saturatedRankTupleCount(childDiagonal, int(domain.exact.value))

		return max(directSize, childCeiling), nil
	}

	if !domain.hasMaximum || domain.maximum.beyond {
		return ^uint64(0), nil
	}

	fixedCount := uint64(arrayLengthMaximum-arrayLengthExact) + 1
	if directSize > ^uint64(0)-fixedCount ||
		directSize+fixedCount > ^uint64(0)-domain.maximum.value {
		return ^uint64(0), nil
	}

	lengthCeiling := directSize + fixedCount + domain.maximum.value

	return max(directSize, lengthCeiling+1), nil
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

	if err := frontier.SetFinite(2, max(1, rowDirectWitnessUpperBound(node, jsonArray))); err != nil {
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

		ceiling, ceilingErr := rowPackedArrayFrontierCeiling(view, active)
		if ceilingErr != nil {
			return false, ceilingErr
		}

		if ranks[4] == 0 && diagonal < ceiling {
			diagonalLive = true
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
			itemsHaveEnum := rowArrayItemsHaveEnum(view, active)
			if ranks[2] != 0 || rowProjectionHasExactCount(view, active) &&
				(ranks[1] != 0 || ranks[0] != 0 && !itemsHaveEnum) {
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

	if err := frontier.SetFinite(2, max(1, rowDirectWitnessUpperBound(node, jsonObject))); err != nil {
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
