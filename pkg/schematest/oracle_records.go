//nolint:godoclint // Private record vocabulary stays behind the clean oracle.
package schematest

import (
	"fmt"
	"iter"
	"slices"
	"strings"
)

// evaluationPointer is one parsed canonical RFC 6901 fragment path.
type evaluationPointer struct {
	tokens []string
}

// evaluationOccurrencePaths is the sole structured occurrence identity retained by a record.
type evaluationOccurrencePaths struct {
	use        evaluationPointer
	target     evaluationPointer
	instance   evaluationPointer
	targetRoot evaluationPointer
	reference  bool
}

// evaluationRecordIdentity is the authoritative structured rule identity.
type evaluationRecordIdentity struct {
	occurrence evaluationOccurrencePaths
	rule       string
}

func newEvaluationRecordIdentity(identity ruleIdentity) evaluationRecordIdentity {
	if identity.structured == nil {
		identity.structured = structuredEvaluationOccurrence(identity.occurrence).structured
	}

	return evaluationRecordIdentity{occurrence: *identity.structured, rule: identity.rule}
}

func (identity evaluationRecordIdentity) project() ruleIdentity {
	return ruleIdentity{
		occurrence: schemaOccurrence{
			usePointer:       identity.occurrence.use.String(),
			targetPointer:    identity.occurrence.target.String(),
			instanceTemplate: identity.occurrence.instance.String(),
			reference:        identity.occurrence.reference,
		},
		rule: identity.rule,
	}
}

func cloneEvaluationRecordIdentity(identity evaluationRecordIdentity) evaluationRecordIdentity {
	identity.occurrence.use.tokens = slices.Clone(identity.occurrence.use.tokens)
	identity.occurrence.target.tokens = slices.Clone(identity.occurrence.target.tokens)
	identity.occurrence.instance.tokens = slices.Clone(identity.occurrence.instance.tokens)
	identity.occurrence.targetRoot.tokens = slices.Clone(identity.occurrence.targetRoot.tokens)

	return identity
}

// occurrenceTransform carries the complete source and destination provenance for one shared view.
type occurrenceTransform struct {
	from evaluationOccurrencePaths
	to   evaluationOccurrencePaths
}

// evaluationRecordKind identifies the single fact carried by an evaluation record.
type evaluationRecordKind uint8

const (
	evaluationRecordApplicable evaluationRecordKind = iota
	evaluationRecordObserved
	evaluationRecordComposition
	evaluationRecordFailure
)

// evaluationRecord carries one fact and its complete structured rule occurrence identity.
type evaluationRecord struct {
	kind     evaluationRecordKind
	identity evaluationRecordIdentity
	level    string
	branches []bool
}

func makeEvaluationRecord(kind evaluationRecordKind, identity ruleIdentity) evaluationRecord {
	return evaluationRecord{kind: kind, identity: newEvaluationRecordIdentity(identity)}
}

// evaluationRecordFilter selects a structural view without copying its records.
type evaluationRecordFilter uint8

const (
	evaluationRecordsAll evaluationRecordFilter = iota
	evaluationRecordsWithoutFailures
)

// evaluationRecordPart is one local or shared segment of a deterministic record sequence.
type evaluationRecordPart struct {
	records   []evaluationRecord
	nested    *evaluationRecords
	transform occurrenceTransform
	filter    evaluationRecordFilter
	count     int
}

// evaluationRecords is one heterogeneous persistent logical record sequence.
type evaluationRecords struct {
	parts           []evaluationRecordPart
	count           int
	nonFailureCount int
}

func newEvaluationRecords() *evaluationRecords {
	return &evaluationRecords{}
}

func (records *evaluationRecords) append(record evaluationRecord) {
	if record.kind == evaluationRecordComposition {
		record.branches = slices.Clone(record.branches)
	}

	records.parts = append(records.parts, evaluationRecordPart{records: []evaluationRecord{record}, count: 1})

	records.count = addEvaluationRecordCount(records.count, 1)
	if record.kind != evaluationRecordFailure {
		records.nonFailureCount = addEvaluationRecordCount(records.nonFailureCount, 1)
	}
}

func (records *evaluationRecords) appendRecords(child *evaluationRecords) {
	records.appendFiltered(child, evaluationRecordsAll)
}

func (records *evaluationRecords) appendNonFailures(child *evaluationRecords) {
	records.appendFiltered(child, evaluationRecordsWithoutFailures)
}

// appendFiltered composes count metadata in O(1); records are visited only by consumers.
func (records *evaluationRecords) appendFiltered(child *evaluationRecords, filter evaluationRecordFilter) {
	if child == nil {
		return
	}

	count := child.count
	if filter == evaluationRecordsWithoutFailures {
		count = child.nonFailureCount
	}

	if count == 0 {
		return
	}

	records.parts = append(records.parts, evaluationRecordPart{nested: child, filter: filter, count: count})
	records.count = addEvaluationRecordCount(records.count, count)
	records.nonFailureCount = addEvaluationRecordCount(records.nonFailureCount, child.nonFailureCount)
}

func addEvaluationRecordCount(current, added int) int {
	maximum := int(^uint(0) >> 1)
	if current == maximum || added > maximum-current {
		return maximum
	}

	return current + added
}

func (records *evaluationRecords) rebased(from, to schemaOccurrence) *evaluationRecords {
	if records == nil {
		return nil
	}

	from = structuredEvaluationOccurrence(from)
	to = structuredEvaluationOccurrence(to)

	transform := occurrenceTransform{from: *from.structured, to: *to.structured}
	if transform.empty() || records.count == 0 {
		return records
	}

	return &evaluationRecords{
		parts: []evaluationRecordPart{{
			nested: records, transform: transform, filter: evaluationRecordsAll, count: records.count,
		}},
		count: records.count, nonFailureCount: records.nonFailureCount,
	}
}

func (records *evaluationRecords) forEach(visit func(evaluationRecord) bool) {
	if records != nil {
		records.forEachWithTransforms(nil, evaluationRecordsAll, visit)
	}
}

func (records *evaluationRecords) forEachWithTransforms(
	outer []occurrenceTransform,
	filter evaluationRecordFilter,
	visit func(evaluationRecord) bool,
) bool {
	for _, part := range records.parts {
		combinedFilter := combineEvaluationRecordFilters(filter, part.filter)
		if part.nested == nil {
			for _, record := range part.records {
				if !evaluationRecordMatches(record, combinedFilter) {
					continue
				}

				for _, transform := range outer {
					record = rebaseEvaluationRecord(record, transform)
				}

				record.identity = cloneEvaluationRecordIdentity(record.identity)
				if record.kind == evaluationRecordComposition {
					record.branches = slices.Clone(record.branches)
				}

				if !visit(record) {
					return false
				}
			}

			continue
		}

		transforms := make([]occurrenceTransform, 0, len(outer)+1)
		if !part.transform.empty() {
			transforms = append(transforms, part.transform)
		}

		transforms = append(transforms, outer...)
		if !part.nested.forEachWithTransforms(transforms, combinedFilter, visit) {
			return false
		}
	}

	return true
}

func combineEvaluationRecordFilters(outer, inner evaluationRecordFilter) evaluationRecordFilter {
	if outer == evaluationRecordsWithoutFailures || inner == evaluationRecordsWithoutFailures {
		return evaluationRecordsWithoutFailures
	}

	return evaluationRecordsAll
}

func evaluationRecordMatches(record evaluationRecord, filter evaluationRecordFilter) bool {
	return filter == evaluationRecordsAll || record.kind != evaluationRecordFailure
}

func selectEvaluationRecords[T any](records *evaluationRecords, project func(evaluationRecord) (T, bool)) iter.Seq[T] {
	return func(yield func(T) bool) {
		records.forEach(func(record evaluationRecord) bool {
			value, ok := project(record)

			return !ok || yield(value)
		})
	}
}

func (result evaluation) applicableRecords() iter.Seq[ruleIdentity] {
	return selectEvaluationRecords(result.records, func(record evaluationRecord) (ruleIdentity, bool) {
		return record.identity.project(), record.kind == evaluationRecordApplicable
	})
}

func (result evaluation) observedRecords() iter.Seq[levelIdentity] {
	return selectEvaluationRecords(result.records, func(record evaluationRecord) (levelIdentity, bool) {
		identity := levelIdentity{ruleIdentity: record.identity.project(), level: record.level}

		return identity, record.kind == evaluationRecordObserved
	})
}

func (result evaluation) compositionRecords(rule string) iter.Seq[compositionTruth] {
	return selectEvaluationRecords(result.records, func(record evaluationRecord) (compositionTruth, bool) {
		return compositionTruth{ruleIdentity: record.identity.project(), branches: record.branches},
			record.kind == evaluationRecordComposition && record.identity.rule == rule
	})
}

func (result evaluation) failureRecords() iter.Seq[failureIdentity] {
	return selectEvaluationRecords(result.records, func(record evaluationRecord) (failureIdentity, bool) {
		return record.identity.project(), record.kind == evaluationRecordFailure
	})
}

func evaluationRecordSequenceCount[T any](sequence iter.Seq[T]) int {
	if sequence == nil {
		return 0
	}

	count := 0
	for range sequence {
		count = addEvaluationRecordCount(count, 1)
	}

	return count
}

func evaluationRecordSequenceAt[T any](sequence iter.Seq[T], index int) (T, bool) {
	var found T
	if sequence == nil || index < 0 {
		return found, false
	}

	current := 0
	for value := range sequence {
		if current == index {
			return value, true
		}

		current++
	}

	return found, false
}

func evaluationRecordSequenceEmpty[T any](sequence iter.Seq[T]) bool {
	if sequence != nil {
		for range sequence {
			return false
		}
	}

	return true
}

func appendEvaluation(result *evaluation, child evaluation) {
	result.records.appendRecords(child.records)

	result.failed = result.failed || child.failed
	if child.err != nil {
		result.err = child.err
	}
}

func appendEvaluationNonFailures(result *evaluation, child evaluation) {
	result.records.appendNonFailures(child.records)
}

func appendApplicable(result *evaluation, identity ruleIdentity) {
	result.records.append(makeEvaluationRecord(evaluationRecordApplicable, identity))
}

func appendObserved(result *evaluation, identity levelIdentity) {
	record := makeEvaluationRecord(evaluationRecordObserved, identity.ruleIdentity)
	record.level = identity.level
	result.records.append(record)
}

func appendCompositionTruth(result *evaluation, truth compositionTruth) {
	record := makeEvaluationRecord(evaluationRecordComposition, truth.ruleIdentity)
	record.branches = truth.branches
	result.records.append(record)
}

func appendAllOfTruth(result *evaluation, truth compositionTruth) {
	appendCompositionTruth(result, truth)
}

func appendAnyOfTruth(result *evaluation, truth compositionTruth) {
	appendCompositionTruth(result, truth)
}

func (transform occurrenceTransform) empty() bool {
	return transform.from.reference == transform.to.reference &&
		transform.from.use.equal(transform.to.use) &&
		transform.from.target.equal(transform.to.target) &&
		transform.from.instance.equal(transform.to.instance) &&
		transform.from.targetRoot.equal(transform.to.targetRoot)
}

// rebaseEvaluationRecord rebases only targets owned by this transform's provenance root.
func rebaseEvaluationRecord(record evaluationRecord, transform occurrenceTransform) evaluationRecord {
	if transform.empty() {
		return record
	}

	occurrence := record.identity.occurrence
	localRoot := occurrence.targetRoot.equal(transform.from.targetRoot)
	localRule := localRoot && occurrence.use.equal(transform.from.use)
	occurrence.use = occurrence.use.rebased(transform.from.use, transform.to.use)

	occurrence.instance = occurrence.instance.rebased(transform.from.instance, transform.to.instance)
	if localRoot {
		occurrence.target = occurrence.target.rebased(transform.from.target, transform.to.target)
		occurrence.targetRoot = transform.to.targetRoot
	}

	if localRule {
		occurrence.reference = transform.to.reference
	}

	record.identity.occurrence = occurrence

	return record
}

func structuredEvaluationOccurrence(occurrence schemaOccurrence) schemaOccurrence {
	if occurrence.structured != nil {
		return occurrence
	}

	paths := mustParseEvaluationOccurrence(occurrence, occurrence.targetPointer)
	occurrence.structured = &paths

	return occurrence
}

func mustParseEvaluationOccurrence(occurrence schemaOccurrence, targetRoot string) evaluationOccurrencePaths {
	parse := func(name, pointer string) evaluationPointer {
		parsed, err := parseEvaluationPointer(pointer)
		if err != nil {
			panic(fmt.Sprintf("invalid oracle %s pointer %q: %v", name, pointer, err))
		}

		return parsed
	}

	if targetRoot == "" {
		targetRoot = occurrence.targetPointer
	}

	return evaluationOccurrencePaths{
		use: parse("use", occurrence.usePointer), target: parse("target", occurrence.targetPointer),
		instance: parse("instance template", occurrence.instanceTemplate), targetRoot: parse("target root", targetRoot),
		reference: occurrence.reference,
	}
}

func parseEvaluationPointer(pointer string) (evaluationPointer, error) {
	if pointer == "" || pointer == "#" {
		return evaluationPointer{}, nil
	}

	if !strings.HasPrefix(pointer, "#/") {
		return evaluationPointer{}, fmt.Errorf("must be a canonical fragment JSON Pointer")
	}

	encoded := strings.Split(pointer[2:], "/")

	tokens := make([]string, len(encoded))
	for index, token := range encoded {
		decoded, err := unescapePointerToken(token)
		if err != nil {
			return evaluationPointer{}, fmt.Errorf("token %d: %w", index, err)
		}

		if escapePointerToken(decoded) != token {
			return evaluationPointer{}, fmt.Errorf("token %d is not canonical", index)
		}

		tokens[index] = decoded
	}

	return evaluationPointer{tokens: tokens}, nil
}

func (pointer evaluationPointer) String() string {
	if len(pointer.tokens) == 0 {
		return "#"
	}

	encoded := make([]string, len(pointer.tokens))
	for index, token := range pointer.tokens {
		encoded[index] = escapePointerToken(token)
	}

	return "#/" + strings.Join(encoded, "/")
}

func (pointer evaluationPointer) equal(other evaluationPointer) bool {
	return slices.Equal(pointer.tokens, other.tokens)
}

func (pointer evaluationPointer) rebased(from, to evaluationPointer) evaluationPointer {
	if from.equal(to) || !pointer.hasPrefix(from) {
		return pointer
	}

	tokens := make([]string, 0, len(to.tokens)+len(pointer.tokens)-len(from.tokens))
	tokens = append(tokens, to.tokens...)
	tokens = append(tokens, pointer.tokens[len(from.tokens):]...)

	return evaluationPointer{tokens: tokens}
}

func (pointer evaluationPointer) hasPrefix(prefix evaluationPointer) bool {
	return len(pointer.tokens) >= len(prefix.tokens) && slices.Equal(pointer.tokens[:len(prefix.tokens)], prefix.tokens)
}
