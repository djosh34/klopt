package schematest

import "strings"

// occurrenceTransform rebases evaluated use-site and instance paths without changing targets.
type occurrenceTransform struct {
	fromUsePointer       string
	toUsePointer         string
	fromInstanceTemplate string
	toInstanceTemplate   string
}

// evaluationRecordPart is one local or shared segment of a deterministic record sequence.
type evaluationRecordPart[T any] struct {
	values    []T
	nested    *evaluationRecordList[T]
	transform occurrenceTransform
}

// evaluationRecordList stores logical records as a compact persistent sequence.
type evaluationRecordList[T any] struct {
	parts []evaluationRecordPart[T]
	count int
}

// evaluationRecords stores every logical evaluation record without expanding shared subtrees.
type evaluationRecords struct {
	applicable evaluationRecordList[ruleIdentity]
	observed   evaluationRecordList[levelIdentity]
	allOf      evaluationRecordList[compositionTruth]
	anyOf      evaluationRecordList[compositionTruth]
	failures   evaluationRecordList[failureIdentity]
}

// newEvaluationRecords creates an empty logical record set.
func newEvaluationRecords() *evaluationRecords {
	return &evaluationRecords{}
}

// ensureEvaluationRecords creates the record set for a manually initialized evaluation.
func ensureEvaluationRecords(result *evaluation) {
	if result.records == nil {
		result.records = newEvaluationRecords()
	}
}

// appendApplicable records one applicable rule in both evaluation views.
func appendApplicable(result *evaluation, identity ruleIdentity) {
	result.applicable = append(result.applicable, identity)
	ensureEvaluationRecords(result)
	result.records.applicable.append(identity)
}

// appendObserved records one observed level in both evaluation views.
func appendObserved(result *evaluation, identity levelIdentity) {
	result.observed = append(result.observed, identity)
	ensureEvaluationRecords(result)
	result.records.observed.append(identity)
}

// appendAllOfTruth records one allOf truth vector in both evaluation views.
func appendAllOfTruth(result *evaluation, truth compositionTruth) {
	result.allOf = append(result.allOf, truth)
	ensureEvaluationRecords(result)
	result.records.allOf.append(truth)
}

// appendAnyOfTruth records one anyOf truth vector in both evaluation views.
func appendAnyOfTruth(result *evaluation, truth compositionTruth) {
	result.anyOf = append(result.anyOf, truth)
	ensureEvaluationRecords(result)
	result.records.anyOf.append(truth)
}

// append adds one local record to a sequence.
func (list *evaluationRecordList[T]) append(value T) {
	list.parts = append(list.parts, evaluationRecordPart[T]{values: []T{value}})
	list.count++
}

// appendList adds a shared sequence with an optional occurrence transform.
func (list *evaluationRecordList[T]) appendList(other *evaluationRecordList[T], transform occurrenceTransform) {
	if other == nil || other.count == 0 {
		return
	}

	list.parts = append(list.parts, evaluationRecordPart[T]{
		nested:    other,
		transform: transform,
	})
	list.count += other.count
}

// rebased returns a shared sequence viewed from one evaluated use site.
func (list *evaluationRecordList[T]) rebased(transform occurrenceTransform) *evaluationRecordList[T] {
	if list == nil || list.count == 0 || transform.empty() {
		return list
	}

	return &evaluationRecordList[T]{
		parts: []evaluationRecordPart[T]{{nested: list, transform: transform}},
		count: list.count,
	}
}

// at returns one logical record without materializing sibling subtrees.
func (list *evaluationRecordList[T]) at(index int) (T, bool) {
	return list.atWithTransforms(index, nil)
}

// atWithTransforms returns one record after applying outer occurrence transforms.
func (list *evaluationRecordList[T]) atWithTransforms(
	index int,
	outer []occurrenceTransform,
) (T, bool) {
	var zero T
	if list == nil || index < 0 || index >= list.count {
		return zero, false
	}

	for _, part := range list.parts {
		partCount := len(part.values)
		if part.nested != nil {
			partCount = part.nested.count
		}

		if index >= partCount {
			index -= partCount

			continue
		}

		if part.nested == nil {
			value := part.values[index]
			for _, transform := range outer {
				value = rebaseEvaluationRecord(value, transform)
			}

			return value, true
		}

		transforms := make([]occurrenceTransform, 0, len(outer)+1)
		if !part.transform.empty() {
			transforms = append(transforms, part.transform)
		}

		transforms = append(transforms, outer...)

		return part.nested.atWithTransforms(index, transforms)
	}

	return zero, false
}

// forEach visits logical records in canonical sequence order.
func (list *evaluationRecordList[T]) forEach(visit func(T)) {
	if list == nil {
		return
	}

	list.forEachWithTransforms(nil, visit)
}

// forEachWithTransforms visits one sequence after applying outer occurrence transforms.
func (list *evaluationRecordList[T]) forEachWithTransforms(
	outer []occurrenceTransform,
	visit func(T),
) {
	for _, part := range list.parts {
		if part.nested == nil {
			for _, value := range part.values {
				for _, transform := range outer {
					value = rebaseEvaluationRecord(value, transform)
				}

				visit(value)
			}

			continue
		}

		transforms := make([]occurrenceTransform, 0, len(outer)+1)
		if !part.transform.empty() {
			transforms = append(transforms, part.transform)
		}

		transforms = append(transforms, outer...)
		part.nested.forEachWithTransforms(transforms, visit)
	}
}

// appendNonFailures appends all child record classes except failures.
func (records *evaluationRecords) appendNonFailures(child *evaluationRecords) {
	if child == nil {
		return
	}

	records.applicable.appendList(&child.applicable, occurrenceTransform{})
	records.observed.appendList(&child.observed, occurrenceTransform{})
	records.allOf.appendList(&child.allOf, occurrenceTransform{})
	records.anyOf.appendList(&child.anyOf, occurrenceTransform{})
}

// recordsMaterialized reports whether compatibility slices already contain every logical record.
func (result *evaluation) recordsMaterialized() bool {
	if result.records == nil {
		return true
	}

	return len(result.applicable) == result.records.applicable.count &&
		len(result.observed) == result.records.observed.count &&
		len(result.allOf) == result.records.allOf.count &&
		len(result.anyOf) == result.records.anyOf.count &&
		len(result.failures) == result.records.failures.count
}

// materializeRecords copies the logical record graph into compatibility slices.
func (result *evaluation) materializeRecords() {
	ensureEvaluationRecords(result)
	result.applicable = nil
	result.observed = nil
	result.allOf = nil
	result.anyOf = nil
	result.failures = nil

	result.records.applicable.forEach(func(identity ruleIdentity) {
		result.applicable = append(result.applicable, identity)
	})
	result.records.observed.forEach(func(identity levelIdentity) {
		result.observed = append(result.observed, identity)
	})
	result.records.allOf.forEach(func(truth compositionTruth) {
		result.allOf = append(result.allOf, truth)
	})
	result.records.anyOf.forEach(func(truth compositionTruth) {
		result.anyOf = append(result.anyOf, truth)
	})
	result.records.failures.forEach(func(identity failureIdentity) {
		result.failures = append(result.failures, identity)
	})
	result.materialized = true
}

// appendAll appends every child record class.
func (records *evaluationRecords) appendAll(child *evaluationRecords) {
	if child == nil {
		return
	}

	records.appendNonFailures(child)
	records.failures.appendList(&child.failures, occurrenceTransform{})
}

// rebased returns all records viewed from another evaluated occurrence.
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
	if transform.empty() {
		return records
	}

	return &evaluationRecords{
		applicable: *records.applicable.rebased(transform),
		observed:   *records.observed.rebased(transform),
		allOf:      *records.allOf.rebased(transform),
		anyOf:      *records.anyOf.rebased(transform),
		failures:   *records.failures.rebased(transform),
	}
}

// empty reports whether a transform leaves every path unchanged.
func (transform occurrenceTransform) empty() bool {
	return transform.fromUsePointer == transform.toUsePointer &&
		transform.fromInstanceTemplate == transform.toInstanceTemplate
}

// rebaseEvaluationRecord changes only evaluated paths and keeps authored targets intact.
func rebaseEvaluationRecord[T any](record T, transform occurrenceTransform) T {
	if transform.empty() {
		return record
	}

	switch typed := any(record).(type) {
	case ruleIdentity:
		typed.occurrence = rebaseEvaluationOccurrence(typed.occurrence, transform)

		converted, ok := any(typed).(T)
		if ok {
			return converted
		}
	case levelIdentity:
		typed.occurrence = rebaseEvaluationOccurrence(typed.occurrence, transform)

		converted, ok := any(typed).(T)
		if ok {
			return converted
		}
	case compositionTruth:
		typed.occurrence = rebaseEvaluationOccurrence(typed.occurrence, transform)

		converted, ok := any(typed).(T)
		if ok {
			return converted
		}
	default:
		return record
	}

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
