//nolint:godoclint // Private record vocabulary stays behind the clean oracle.
package schematest

import (
	"iter"
	"strings"
)

// occurrenceTransform rebases evaluated use-site and instance paths without changing targets.
type occurrenceTransform struct {
	fromUsePointer       string
	toUsePointer         string
	fromInstanceTemplate string
	toInstanceTemplate   string
}

// evaluationRecordKind identifies the single fact carried by an evaluation record.
type evaluationRecordKind uint8

const (
	evaluationRecordApplicable evaluationRecordKind = iota
	evaluationRecordObserved
	evaluationRecordComposition
	evaluationRecordFailure
)

// evaluationRecord carries one fact and its complete rule occurrence identity.
type evaluationRecord struct {
	kind     evaluationRecordKind
	identity ruleIdentity
	level    string
	branches []bool
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
	parts []evaluationRecordPart
	count int
}

// newEvaluationRecords creates an empty logical record sequence.
func newEvaluationRecords() *evaluationRecords {
	return &evaluationRecords{}
}

// append adds one local fact and returns its construction-time record.
func (records *evaluationRecords) append(record evaluationRecord) *evaluationRecord {
	records.parts = append(records.parts, evaluationRecordPart{
		records: []evaluationRecord{record},
		count:   1,
	})
	records.count = addEvaluationRecordCount(records.count, 1)

	return &records.parts[len(records.parts)-1].records[0]
}

// appendRecords shares one complete child sequence.
func (records *evaluationRecords) appendRecords(child *evaluationRecords) {
	records.appendFiltered(child, evaluationRecordsAll)
}

// appendNonFailures shares every child fact except failures.
func (records *evaluationRecords) appendNonFailures(child *evaluationRecords) {
	records.appendFiltered(child, evaluationRecordsWithoutFailures)
}

func (records *evaluationRecords) appendFiltered(child *evaluationRecords, filter evaluationRecordFilter) {
	if child == nil || child.count == 0 {
		return
	}

	count := child.countMatching(filter)
	if count == 0 {
		return
	}

	records.parts = append(records.parts, evaluationRecordPart{
		nested: child,
		filter: filter,
		count:  count,
	})
	records.count = addEvaluationRecordCount(records.count, count)
}

// addEvaluationRecordCount saturates logical record counts without affecting record storage.
func addEvaluationRecordCount(current, added int) int {
	maximum := int(^uint(0) >> 1)
	if current == maximum || added > maximum-current {
		return maximum
	}

	return current + added
}

// rebased returns the same logical sequence viewed from another occurrence.
func (records *evaluationRecords) rebased(from, to schemaOccurrence) *evaluationRecords {
	if records == nil {
		return nil
	}

	transform := occurrenceTransform{
		fromUsePointer:       from.usePointer,
		toUsePointer:         to.usePointer,
		fromInstanceTemplate: from.instanceTemplate,
		toInstanceTemplate:   to.instanceTemplate,
	}
	if transform.empty() || records.count == 0 {
		return records
	}

	return &evaluationRecords{
		parts: []evaluationRecordPart{{
			nested:    records,
			transform: transform,
			filter:    evaluationRecordsAll,
			count:     records.count,
		}},
		count: records.count,
	}
}

// forEach visits logical records in canonical sequence order.
func (records *evaluationRecords) forEach(visit func(evaluationRecord) bool) {
	if records == nil {
		return
	}

	records.forEachWithTransforms(nil, evaluationRecordsAll, visit)
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

func combineEvaluationRecordFilters(
	outer evaluationRecordFilter,
	inner evaluationRecordFilter,
) evaluationRecordFilter {
	if outer == evaluationRecordsWithoutFailures || inner == evaluationRecordsWithoutFailures {
		return evaluationRecordsWithoutFailures
	}

	return evaluationRecordsAll
}

func evaluationRecordMatches(record evaluationRecord, filter evaluationRecordFilter) bool {
	return filter == evaluationRecordsAll || record.kind != evaluationRecordFailure
}

func (records *evaluationRecords) countMatching(filter evaluationRecordFilter) int {
	if records == nil {
		return 0
	}

	if filter == evaluationRecordsAll {
		return records.count
	}

	count := 0

	records.forEach(func(record evaluationRecord) bool {
		if evaluationRecordMatches(record, filter) {
			count = addEvaluationRecordCount(count, 1)
		}

		return true
	})

	return count
}

// select returns a lazy typed view over the authoritative record sequence.
func selectEvaluationRecords[T any](
	records *evaluationRecords,
	project func(evaluationRecord) (T, bool),
) iter.Seq[T] {
	return func(yield func(T) bool) {
		records.forEach(func(record evaluationRecord) bool {
			value, ok := project(record)
			if !ok {
				return true
			}

			return yield(value)
		})
	}
}

func (result evaluation) applicableRecords() iter.Seq[ruleIdentity] {
	return selectEvaluationRecords(result.records, func(record evaluationRecord) (ruleIdentity, bool) {
		return record.identity, record.kind == evaluationRecordApplicable
	})
}

func (result evaluation) observedRecords() iter.Seq[levelIdentity] {
	return selectEvaluationRecords(result.records, func(record evaluationRecord) (levelIdentity, bool) {
		return levelIdentity{ruleIdentity: record.identity, level: record.level}, record.kind == evaluationRecordObserved
	})
}

func (result evaluation) compositionRecords(rule string) iter.Seq[compositionTruth] {
	return selectEvaluationRecords(result.records, func(record evaluationRecord) (compositionTruth, bool) {
		return compositionTruth{
			ruleIdentity: record.identity,
			branches:     record.branches,
		}, record.kind == evaluationRecordComposition && record.identity.rule == rule
	})
}

func (result evaluation) failureRecords() iter.Seq[failureIdentity] {
	return selectEvaluationRecords(result.records, func(record evaluationRecord) (failureIdentity, bool) {
		return record.identity, record.kind == evaluationRecordFailure
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
	if sequence == nil {
		return true
	}

	for range sequence {
		return false
	}

	return true
}

// appendEvaluation appends a child evaluation in traversal order.
func appendEvaluation(result *evaluation, child evaluation) {
	result.records.appendRecords(child.records)
	result.failed = result.failed || child.failed

	if child.err != nil {
		result.err = child.err
	}
}

// appendEvaluationNonFailures appends only non-failure child records in traversal order.
func appendEvaluationNonFailures(result *evaluation, child evaluation) {
	result.records.appendNonFailures(child.records)
}

// appendApplicable records one applicable rule.
func appendApplicable(result *evaluation, identity ruleIdentity) {
	result.records.append(evaluationRecord{kind: evaluationRecordApplicable, identity: identity})
}

// appendObserved records one observed level.
func appendObserved(result *evaluation, identity levelIdentity) {
	result.records.append(evaluationRecord{
		kind:     evaluationRecordObserved,
		identity: identity.ruleIdentity,
		level:    identity.level,
	})
}

// appendAllOfTruth records one allOf truth vector.
func appendAllOfTruth(result *evaluation, truth compositionTruth) *evaluationRecord {
	return result.records.append(evaluationRecord{
		kind:     evaluationRecordComposition,
		identity: truth.ruleIdentity,
		branches: truth.branches,
	})
}

// appendAnyOfTruth records one anyOf truth vector.
func appendAnyOfTruth(result *evaluation, truth compositionTruth) *evaluationRecord {
	return result.records.append(evaluationRecord{
		kind:     evaluationRecordComposition,
		identity: truth.ruleIdentity,
		branches: truth.branches,
	})
}

// empty reports whether a transform leaves every path unchanged.
func (transform occurrenceTransform) empty() bool {
	return transform.fromUsePointer == transform.toUsePointer &&
		transform.fromInstanceTemplate == transform.toInstanceTemplate
}

// rebaseEvaluationRecord changes only evaluated paths and keeps authored targets intact.
func rebaseEvaluationRecord(record evaluationRecord, transform occurrenceTransform) evaluationRecord {
	if transform.empty() {
		return record
	}

	record.identity.occurrence = rebaseEvaluationOccurrence(record.identity.occurrence, transform)

	return record
}

// rebaseEvaluationOccurrence changes use and instance paths while preserving target metadata.
func rebaseEvaluationOccurrence(occurrence schemaOccurrence, transform occurrenceTransform) schemaOccurrence {
	occurrence.usePointer = replaceOccurrencePrefix(
		occurrence.usePointer,
		transform.fromUsePointer,
		transform.toUsePointer,
	)
	occurrence.instanceTemplate = replaceOccurrencePrefix(
		occurrence.instanceTemplate,
		transform.fromInstanceTemplate,
		transform.toInstanceTemplate,
	)

	return occurrence
}

// replaceOccurrencePrefix replaces one canonical path prefix and preserves relative descendants.
func replaceOccurrencePrefix(value, from, to string) string {
	if from == to || value == from {
		return to
	}

	if strings.HasPrefix(value, from+"/") {
		return to + strings.TrimPrefix(value, from)
	}

	return value
}
