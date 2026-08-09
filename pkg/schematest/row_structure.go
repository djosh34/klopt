package schematest

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// rowMember describes one object member choice and its best clean schema occurrence.
type rowMember struct {
	name       string
	schemas    rowSchemaConjunction
	occurrence schemaOccurrence
	required   bool
}

const (
	// arrayLengthDirect emits authored array witness lengths.
	arrayLengthDirect uint8 = iota
	// arrayLengthExact emits the directed exact count.
	arrayLengthExact
	// arrayLengthMinimum emits the effective minimum.
	arrayLengthMinimum
	// arrayLengthMaximum emits the effective maximum.
	arrayLengthMaximum
	// arrayLengthRemaining emits the numeric count domain.
	arrayLengthRemaining
)

// rowArrayCount is one exact pending count; huge authored counts remain exact.
type rowArrayCount struct {
	exact  *exactCount
	value  uint64
	beyond bool
}

// rowArrayLengthDomain is the directly addressable authored and numeric length domain.
type rowArrayLengthDomain struct {
	view       rowProjectionView
	exact      rowArrayCount
	minimum    rowArrayCount
	maximum    rowArrayCount
	hasExact   bool
	hasMaximum bool
	infeasible bool
}

// rowArrayLengthCursor lazily enumerates authored lengths, objectives, bounds, and numeric counts.
type rowArrayLengthCursor struct {
	rowArrayLengthDomain

	directSource uint64
	directMember uint64
	remaining    uint64
	phase        uint8
	finiteEnd    bool
}

// newRowArrayLengthDomain intersects active counts without narrowing authored values.
//
//nolint:cyclop,gocognit,gocyclo // Count intersection and directed objectives form one domain.
func newRowArrayLengthDomain(view rowProjectionView, requirements []requirement) (rowArrayLengthDomain, error) {
	domain := rowArrayLengthDomain{view: view}

	var minimum, maximum *exactCount

	for _, source := range view.sources {
		if source.node == nil || source.node.schemaShape == nil {
			return rowArrayLengthDomain{}, errors.New("schematest: projected array source has no shape")
		}

		if source.node.minItems != nil {
			if minimum == nil {
				minimum = source.node.minItems
			} else if comparison, err := source.node.minItems.number.compare(minimum.number); err != nil {
				return rowArrayLengthDomain{}, err
			} else if comparison > 0 {
				minimum = source.node.minItems
			}
		}

		if source.node.maxItems != nil {
			if maximum == nil {
				maximum = source.node.maxItems
			} else if comparison, err := source.node.maxItems.number.compare(maximum.number); err != nil {
				return rowArrayLengthDomain{}, err
			} else if comparison < 0 {
				maximum = source.node.maxItems
			}
		}

		for _, requirement := range requirements {
			if requirement.tag != requirementExactCount || requirement.count == nil ||
				!rowOccurrenceMatches(requirement.occurrence, source.occurrence) {
				continue
			}

			candidate, err := rowArrayCountFromExact(requirement.count)
			if err != nil {
				return rowArrayLengthDomain{}, err
			}

			if domain.hasExact {
				equal, equalErr := rowArrayCountsEqual(domain.exact, candidate)
				if equalErr != nil {
					return rowArrayLengthDomain{}, equalErr
				}

				domain.infeasible = domain.infeasible || !equal
			} else {
				domain.exact, domain.hasExact = candidate, true
			}
		}
	}

	var err error

	domain.minimum, err = rowArrayCountFromExact(minimum)
	if err != nil {
		return rowArrayLengthDomain{}, err
	}

	if maximum != nil {
		domain.maximum, err = rowArrayCountFromExact(maximum)
		if err != nil {
			return rowArrayLengthDomain{}, err
		}

		domain.hasMaximum = true
	}

	for _, source := range view.sources {
		item, exists := rowChildSchemaSource(source.node, source.occurrence, rowChildItems, "")
		if !exists {
			continue
		}

		presence, found := rowPresenceRequirementDetails(requirements, item.occurrence)
		if found && !presence.canonical && presence.presence == requirementPresent &&
			!domain.minimum.beyond && domain.minimum.value < 1 {
			domain.minimum = rowArrayCount{value: 1}
		}
	}

	if domain.hasMaximum {
		comparison, compareErr := rowArrayCountsCompare(domain.minimum, domain.maximum)
		if compareErr != nil {
			return rowArrayLengthDomain{}, compareErr
		}

		domain.infeasible = domain.infeasible || comparison > 0
	}

	if domain.hasExact {
		comparison, compareErr := rowArrayCountsCompare(domain.exact, domain.minimum)
		if compareErr != nil {
			return rowArrayLengthDomain{}, compareErr
		}

		domain.infeasible = domain.infeasible || comparison < 0
		if domain.hasMaximum {
			comparison, compareErr = rowArrayCountsCompare(domain.exact, domain.maximum)
			if compareErr != nil {
				return rowArrayLengthDomain{}, compareErr
			}

			domain.infeasible = domain.infeasible || comparison > 0
		}
	}

	return domain, nil
}

// newRowArrayLengthCursor creates the standalone cursor used by length-cursor callers.
func newRowArrayLengthCursor(
	view rowProjectionView,
	requirements []requirement,
) (*rowArrayLengthCursor, error) {
	domain, err := newRowArrayLengthDomain(view, requirements)
	if err != nil {
		return nil, err
	}

	return &rowArrayLengthCursor{rowArrayLengthDomain: domain}, nil
}

// rowArrayCountFromExact preserves one admitted exact count without narrowing.
func rowArrayCountFromExact(count *exactCount) (rowArrayCount, error) {
	if count == nil {
		return rowArrayCount{}, nil
	}

	value, fits, err := exactCountUint64(count)
	if err != nil {
		return rowArrayCount{}, err
	}

	return rowArrayCount{exact: count, value: value, beyond: !fits}, nil
}

// rowArrayCountsCompare orders admitted exact counts without narrowing.
func rowArrayCountsCompare(left, right rowArrayCount) (int, error) {
	if left.exact != nil && right.exact != nil && (left.beyond || right.beyond) {
		return left.exact.number.compare(right.exact.number)
	}

	if left.beyond {
		return 1, nil
	}

	if right.beyond {
		return -1, nil
	}

	if left.value < right.value {
		return -1, nil
	}

	if left.value > right.value {
		return 1, nil
	}

	return 0, nil
}

// rowArrayCountsEqual compares admitted exact counts without narrowing.
func rowArrayCountsEqual(left, right rowArrayCount) (bool, error) {
	comparison, err := rowArrayCountsCompare(left, right)

	return comparison == 0, err
}

// Next returns one first-occurrence count. Open cursors never exhaust locally.
//
//nolint:cyclop // Named and numeric phases form one cursor transition.
func (cursor *rowArrayLengthCursor) Next() (rowArrayCount, bool, error) {
	if cursor.infeasible {
		return rowArrayCount{}, false, nil
	}

	for cursor.phase < arrayLengthRemaining {
		candidate, ok, err := cursor.nextNamed()
		if err != nil || ok {
			return candidate, ok, err
		}
	}

	for !cursor.finiteEnd {
		candidate := rowArrayCount{value: cursor.remaining}
		if cursor.hasMaximum && !cursor.maximum.beyond && cursor.remaining == cursor.maximum.value {
			cursor.finiteEnd = true
		} else if cursor.remaining == ^uint64(0) {
			return rowArrayCount{}, false, errors.New("schematest: array length rank overflow")
		} else {
			cursor.remaining++
		}

		seen, err := cursor.seenBefore(candidate, arrayLengthRemaining)
		if err != nil {
			return rowArrayCount{}, false, err
		}

		if !seen {
			return candidate, true, nil
		}
	}

	return rowArrayCount{}, false, nil
}

// nextNamed advances the direct and fixed named phases.
//
//nolint:cyclop,gocognit // Direct and fixed named phases form one cursor transition.
func (cursor *rowArrayLengthCursor) nextNamed() (rowArrayCount, bool, error) {
	for cursor.phase < arrayLengthRemaining {
		phase := cursor.phase
		switch phase {
		case arrayLengthDirect:
			for cursor.directSource < uint64(len(cursor.view.sources)) {
				sourceIndex := cursor.directSource
				source := &cursor.view.sources[sourceIndex]

				memberCount := uint64(1)
				if source.node != nil && source.node.enum != nil {
					memberCount = uint64(len(source.node.enum))
				}

				if cursor.directMember >= memberCount {
					cursor.directSource++
					cursor.directMember = 0

					continue
				}

				memberIndex := cursor.directMember
				cursor.directMember++

				candidate, ok, err := rowDirectArrayLengthAt(cursor.view, sourceIndex, memberIndex)
				if err != nil {
					return rowArrayCount{}, false, err
				}

				if !ok {
					continue
				}

				seen, seenErr := rowDirectArrayLengthSeenBefore(
					cursor.view, candidate, sourceIndex, memberIndex,
				)
				if seenErr != nil || !seen {
					return candidate, !seen, seenErr
				}
			}

			cursor.phase++
		case arrayLengthExact:
			cursor.phase++
			if cursor.hasExact {
				seen, err := cursor.seenBefore(cursor.exact, phase)

				return cursor.exact, !seen, err
			}
		case arrayLengthMinimum:
			cursor.phase++
			seen, err := cursor.seenBefore(cursor.minimum, phase)

			return cursor.minimum, !seen, err
		case arrayLengthMaximum:
			cursor.phase++
			if cursor.hasMaximum {
				seen, err := cursor.seenBefore(cursor.maximum, phase)

				return cursor.maximum, !seen, err
			}
		}
	}

	return rowArrayCount{}, false, nil
}

// seenBefore reports whether an earlier named rank yielded the exact count.
//

//nolint:godoclint // The receiver supplies the private domain context.
func (domain rowArrayLengthDomain) seenBefore(candidate rowArrayCount, phase uint8) (bool, error) {
	seen, err := rowDirectArrayLengthSeenBefore(
		domain.view, candidate, uint64(len(domain.view.sources)), 0,
	)
	if err != nil || seen {
		return seen, err
	}

	fixed := []struct {
		phase uint8
		count rowArrayCount
		set   bool
	}{
		{arrayLengthExact, domain.exact, domain.hasExact},
		{arrayLengthMinimum, domain.minimum, true},
		{arrayLengthMaximum, domain.maximum, domain.hasMaximum},
	}
	for _, prior := range fixed {
		if phase <= prior.phase || !prior.set {
			continue
		}

		equal, err := rowArrayCountsEqual(candidate, prior.count)
		if err != nil || equal {
			return equal, err
		}
	}

	return false, nil
}

// rowDirectArrayLengthSeenBefore checks authored occurrences in one model pass.
//
//nolint:cyclop // Enum/default sources share one authored-occurrence scan.
func rowDirectArrayLengthSeenBefore(
	view rowProjectionView,
	candidate rowArrayCount,
	selectedSource uint64,
	selectedMember uint64,
) (bool, error) {
	for sourceIndex, source := range view.sources {
		if source.node == nil || source.node.schemaShape == nil {
			return false, errors.New("schematest: projected array source has no shape")
		}

		check := func(value *jsonValue, memberIndex uint64) (bool, error) {
			if value == nil {
				return false, errors.New("schematest: nil projected enum value")
			}

			if uint64(sourceIndex) > selectedSource ||
				uint64(sourceIndex) == selectedSource && memberIndex >= selectedMember {
				return false, nil
			}

			if value.kind != jsonArray {
				return false, nil
			}

			return rowArrayCountsEqual(candidate, rowArrayCount{value: uint64(len(value.array))})
		}

		if source.node.enum != nil {
			for memberIndex, member := range source.node.enum {
				seen, err := check(member.value, uint64(memberIndex))
				if err != nil || seen {
					return seen, err
				}
			}
		} else if source.node.defaultValue != nil {
			seen, err := check(source.node.defaultValue, 0)
			if err != nil || seen {
				return seen, err
			}
		}
	}

	return false, nil
}

// rowDirectArrayLengthAt directly indexes one authored source and enum/default member.
func rowDirectArrayLengthAt(
	view rowProjectionView,
	sourceIndex uint64,
	memberIndex uint64,
) (rowArrayCount, bool, error) {
	if sourceIndex >= uint64(len(view.sources)) {
		return rowArrayCount{}, false, nil
	}

	source := &view.sources[sourceIndex]
	if source.node == nil || source.node.schemaShape == nil {
		return rowArrayCount{}, false, errors.New("schematest: projected array source has no shape")
	}

	var value *jsonValue

	if source.node.enum != nil {
		if memberIndex >= uint64(len(source.node.enum)) {
			return rowArrayCount{}, false, nil
		}

		value = source.node.enum[memberIndex].value
		if value == nil {
			return rowArrayCount{}, false, errors.New("schematest: nil projected enum value")
		}
	} else {
		if memberIndex != 0 {
			return rowArrayCount{}, false, nil
		}

		value = source.node.defaultValue
	}

	if value == nil || value.kind != jsonArray {
		return rowArrayCount{}, false, nil
	}

	return rowArrayCount{value: uint64(len(value.array))}, true, nil
}

// walkProjectedDirectValues visits complete authored witnesses from one charged projection.
func (s *search) walkProjectedDirectValues(
	view rowProjectionView,
	kind jsonKind,
	visit rowVisit,
) (bool, error) {
	var (
		complete bool
		visitErr error
	)

	err := view.eachDirectValue(func(_ rowSchemaSource, candidate *jsonValue) bool {
		if candidate.kind != kind {
			return true
		}

		if visitErr = s.assign(); visitErr != nil {
			return false
		}

		owned, cloneErr := cloneJSONValue(candidate)
		if cloneErr != nil {
			visitErr = cloneErr

			return false
		}

		complete, visitErr = visit(owned)

		return visitErr == nil && !complete
	})
	if err != nil {
		return false, err
	}

	return complete, visitErr
}

// walkArray advances projection, emitted length, and child ranks through one fair frontier.
func (s *search) walkArray(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	context rowSearchContext,
	visit rowVisit,
) (bool, error) {
	return s.walkArrayFrontier(node, occurrence, requirements, context, visit)
}

// rowProjectionAcceptsKind reports whether every active source admits one structural kind.
func rowProjectionAcceptsKind(view rowProjectionView, kind jsonKind) bool {
	for _, source := range view.sources {
		if source.node.enum != nil {
			accepted := false
			for _, member := range source.node.enum {
				accepted = accepted || member.value != nil && member.value.kind == kind
			}

			if !accepted {
				return false
			}
		}

		accepted := false
		for _, candidate := range orderedTypeKinds(source.node) {
			accepted = accepted || candidate == kind
		}

		if !accepted {
			return false
		}
	}

	return true
}

// rowSchemaConjunction keeps every child schema at its authored occurrence.
type rowSchemaConjunction struct {
	sources  []rowSchemaSource
	fallback schemaOccurrence
}

// rowProjectedArrayItems retains every active authored item occurrence in conjunction order.
func rowProjectedArrayItems(view rowProjectionView, requirements []requirement) rowSchemaConjunction {
	var conjunction rowSchemaConjunction

	view.eachSource(func(source rowSchemaSource) bool {
		if conjunction.fallback.usePointer == "" {
			conjunction.fallback = rowChildOccurrence(source.node, source.occurrence, rowChildItems, "")
		}

		if item, exists := rowChildSchemaSource(source.node, source.occurrence, rowChildItems, ""); exists {
			conjunction.sources = append(conjunction.sources, item)
		}

		return true
	})

	conjunction.sources = rowPreferredSchemaSources(conjunction.sources, requirements)

	return conjunction
}

// walkRowSchemaConjunction searches each child source at its authored occurrence.
//
//nolint:cyclop,gocognit,nestif // Finite enum intersections and generated sources share one seam.
func (s *search) walkRowSchemaConjunction(
	conjunction rowSchemaConjunction,
	requirements []requirement,
	context rowSearchContext,
	visit rowVisit,
) (bool, error) {
	if len(conjunction.sources) == 0 {
		return s.walkGenericValue(requirements, visit)
	}

	hasEnum := false

	for _, source := range conjunction.sources {
		if source.node == nil || source.node.schemaShape == nil {
			return false, errors.New("schematest: structural child source has no shape")
		}

		hasEnum = hasEnum || source.node.enum != nil
	}

	if hasEnum {
		for index, source := range conjunction.sources {
			if source.node.enum == nil {
				continue
			}

			if index > 0 {
				if err := s.assign(); err != nil {
					return false, err
				}
			}

			for _, member := range source.node.enum {
				if member.value == nil {
					return false, errors.New("schematest: nil structural child enum value")
				}

				if err := s.assign(); err != nil {
					return false, err
				}

				usable, err := s.rowConjunctionValueUsable(conjunction.sources, requirements, member.value)
				if err != nil {
					return false, err
				}

				if !usable {
					continue
				}

				complete, visitErr := visit(member.value)
				if visitErr != nil || complete {
					return complete, visitErr
				}
			}
		}

		return false, nil
	}

	for index, source := range conjunction.sources {
		if index > 0 {
			if err := s.assign(); err != nil {
				return false, err
			}
		}

		complete, err := s.walkNode(
			source.node, source.occurrence, requirements, context,
			func(value *jsonValue) (bool, error) {
				usable, usableErr := s.rowConjunctionValueUsable(
					conjunction.sources, requirements, value,
				)
				if usableErr != nil || !usable {
					return false, usableErr
				}

				return visit(value)
			},
		)
		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}

// rowConjunctionValueUsable checks every source at its original occurrence.
func (s *search) rowConjunctionValueUsable(
	sources []rowSchemaSource,
	requirements []requirement,
	value *jsonValue,
) (bool, error) {
	for _, source := range sources {
		usable, err := s.rowChildValueUsable(source.node, source.occurrence, requirements, value)
		if err != nil || !usable {
			return false, err
		}
	}

	return true, nil
}

// rowProjectedObject is the incrementally constructed shape of one active projection.
type rowProjectedObject struct {
	members        []rowMember
	owners         []rowSchemaSource
	declared       map[string]bool
	rootOccurrence schemaOccurrence
	minimum        uint64
	maximum        uint64
	exact          uint64
	minimumBeyond  bool
	exactBeyond    bool
	countConflict  bool
	hasMaximum     bool
	hasExact       bool
	allowsExtra    bool
	requiresExtra  bool
	infeasible     bool
}

// walkObject advances projection, presence, wildcard, and child ranks through one fair frontier.
func (s *search) walkObject(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	context rowSearchContext,
	visit rowVisit,
) (bool, error) {
	return s.walkObjectFrontier(node, occurrence, requirements, context, visit)
}

// newRowProjectedObject derives bounds, names, requiredness, and wildcard schemas from one view.
//
//nolint:cyclop,gocognit,gocyclo // The active object conjunction is intentionally compiled at one seam.
func newRowProjectedObject(
	view rowProjectionView,
	requirements []requirement,
	rootOccurrence schemaOccurrence,
) (*rowProjectedObject, error) {
	shape := &rowProjectedObject{
		declared: make(map[string]bool), rootOccurrence: rootOccurrence, allowsExtra: true,
	}
	required := make(map[string]schemaOccurrence)
	owners := make([]rowSchemaSource, 0, len(view.sources))

	for _, source := range view.sources {
		if source.node == nil || source.node.schemaShape == nil {
			return nil, errors.New("schematest: projected object source has no shape")
		}

		owners = append(owners, source)

		minimum, fits, err := exactCountUint64(source.node.minProperties)
		if err != nil {
			return nil, err
		}

		if source.node.minProperties != nil && !fits {
			shape.minimumBeyond = true
		} else if fits && minimum > shape.minimum {
			shape.minimum = minimum
		}

		maximum, fits, err := exactCountUint64(source.node.maxProperties)
		if err != nil {
			return nil, err
		}

		if fits && (!shape.hasMaximum || maximum < shape.maximum) {
			shape.maximum = maximum
			shape.hasMaximum = true
		}

		for _, requirement := range requirements {
			if requirement.tag != requirementExactCount || requirement.count == nil ||
				!rowOccurrenceMatches(requirement.occurrence, source.occurrence) {
				continue
			}

			exact, exactFits, exactErr := exactCountUint64(requirement.count)
			if exactErr != nil {
				return nil, exactErr
			}

			if !exactFits {
				if shape.hasExact && !shape.exactBeyond {
					shape.countConflict = true
				}

				shape.exactBeyond = true
				shape.hasExact = true
			} else if shape.hasExact && (shape.exactBeyond || shape.exact != exact) {
				shape.countConflict = true
			} else {
				shape.exact = exact
				shape.hasExact = true
			}
		}

		for _, name := range sortedSchemaPropertyNames(source.node.properties) {
			shape.declared[name] = true
		}

		for _, name := range source.node.required {
			if _, exists := required[name]; !exists {
				required[name] = requiredPresenceOccurrence(source.node, source.occurrence, name)
			}

			shape.declared[name] = true
		}
	}

	for _, requirement := range requirements {
		name, child := rowChildName(rootOccurrence.instanceTemplate, requirement.occurrence.instanceTemplate)
		if !child || name == "*" || !projectedRequirementActive(requirement, owners, rootOccurrence) {
			continue
		}

		if requirement.presence != requirementNoPresence || requirement.hasKind {
			shape.declared[name] = true
		}
	}

	names := make([]string, 0, len(shape.declared))
	for name := range shape.declared {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		allowed := true

		var sources []rowSchemaSource

		for _, owner := range owners {
			if property, exists := rowChildSchemaSource(owner.node, owner.occurrence, rowChildProperty, name); exists {
				sources = append(sources, property)

				continue
			}

			if owner.node.additionalProperties != nil {
				additionalOccurrence := rebasePlanOccurrence(
					owner.node.additionalProperties,
					owner.occurrence,
					owner.occurrence.usePointer+"/additionalProperties",
					appendInstanceToken(rootOccurrence.instanceTemplate, name),
				)
				sources = append(sources, rowSchemaSource{
					node: owner.node.additionalProperties, occurrence: additionalOccurrence,
				})

				continue
			}

			if !owner.node.allowAdditionalProperties {
				allowed = false

				break
			}
		}

		if !allowed {
			if _, mustExist := required[name]; mustExist {
				shape.infeasible = true
			}

			continue
		}

		ordered := rowPreferredSchemaSources(sources, requirements)

		member := rowMember{name: name, schemas: rowSchemaConjunction{sources: ordered}}
		if len(ordered) > 0 {
			member.occurrence = ordered[0].occurrence
			member.schemas.fallback = ordered[0].occurrence
		} else if requiredOccurrence, mustExist := required[name]; mustExist {
			member.occurrence = requiredOccurrence
		} else {
			member.occurrence = schemaOccurrence{
				usePointer:       rootOccurrence.usePointer + "/properties/" + escapePointerToken(name),
				targetPointer:    rootOccurrence.targetPointer,
				instanceTemplate: appendInstanceToken(rootOccurrence.instanceTemplate, name),
			}
		}

		_, member.required = required[name]
		shape.members = append(shape.members, member)
	}

	shape.owners = owners
	shape.allowsExtra = projectedObjectAllowsExtra(owners)
	shape.requiresExtra = projectedAdditionalRequired(requirements, owners, shape.declared)

	return shape, nil
}

// projectedRequirementActive excludes child guidance authored only below inactive branches.
func projectedRequirementActive(
	requirement requirement,
	owners []rowSchemaSource,
	root schemaOccurrence,
) bool {
	localPrefix := root.usePointer + "/properties/"
	if strings.HasPrefix(requirement.occurrence.usePointer, localPrefix) ||
		strings.HasPrefix(requirement.occurrence.usePointer, root.usePointer+"/additionalProperties") {
		return true
	}

	for _, owner := range owners {
		if owner.occurrence.usePointer != root.usePointer &&
			strings.HasPrefix(requirement.occurrence.usePointer, owner.occurrence.usePointer+"/") {
			return true
		}
	}

	return false
}

// projectedAdditionalRequired identifies active wildcard value or presence guidance.
//
//nolint:cyclop // Matching wildcard ownership and declared coverage share one decision.
func projectedAdditionalRequired(
	requirements []requirement,
	owners []rowSchemaSource,
	declared map[string]bool,
) bool {
	for _, requirement := range requirements {
		if requirement.presence != requirementPresent && !requirement.hasKind {
			continue
		}

		for _, owner := range owners {
			if owner.node.additionalProperties == nil ||
				requirement.occurrence.usePointer != owner.occurrence.usePointer+"/additionalProperties" ||
				!instanceTemplateMatches(requirement.occurrence.instanceTemplate,
					appendInstanceToken(owner.occurrence.instanceTemplate, "*")) {
				continue
			}

			coveredByDeclared := false

			for name := range declared {
				if _, explicitlyDeclared := owner.node.properties[name]; !explicitlyDeclared {
					coveredByDeclared = true

					break
				}
			}

			if coveredByDeclared {
				continue
			}

			return true
		}
	}

	return false
}

// projectedObjectAllowsExtra requires every active occurrence to admit an undeclared name.
func projectedObjectAllowsExtra(owners []rowSchemaSource) bool {
	for _, owner := range owners {
		if owner.node.additionalProperties == nil && !owner.node.allowAdditionalProperties {
			return false
		}
	}

	return true
}

// feasible rejects intersected count contradictions without constructing a partial object.
//
//nolint:cyclop // Bounded, unbounded, exact, and intersected count states are explicit.
func (shape *rowProjectedObject) feasible() bool {
	if shape.infeasible || shape.countConflict {
		return false
	}

	required := shape.requiredFrom(0)
	if shape.hasMaximum && required > shape.maximum {
		return false
	}

	if !shape.allowsExtra {
		capacity := uint64(len(shape.members))
		if shape.minimumBeyond || shape.exactBeyond || shape.minimum > capacity ||
			shape.hasExact && shape.exact > capacity || shape.requiresExtra {
			return false
		}
	}

	if shape.minimumBeyond {
		return !shape.hasMaximum && (!shape.hasExact || shape.exactBeyond)
	}

	if shape.exactBeyond {
		return !shape.hasMaximum
	}

	if shape.hasExact && (shape.exact < shape.minimum || shape.hasMaximum && shape.exact > shape.maximum) {
		return false
	}

	return !shape.hasMaximum || shape.minimum <= shape.maximum
}

// requiredFrom counts mandatory declared members without recursive prefix recomputation.
func (shape *rowProjectedObject) requiredFrom(index int) uint64 {
	var count uint64

	for _, member := range shape.members[index:] {
		if member.required {
			count++
		}
	}

	return count
}

// targetCount returns the exact objective or effective active lower bound.
func (shape *rowProjectedObject) targetCount() (uint64, bool) {
	if shape.hasExact {
		return shape.exact, shape.exactBeyond
	}

	return shape.minimum, shape.minimumBeyond
}

// projectedPresenceFeasible prunes impossible prefixes before charging their presence choice.
func projectedPresenceFeasible(
	shape *rowProjectedObject,
	index int,
	choice bool,
	present uint64,
	remainingRequired uint64,
) bool {
	nextPresent := present
	if choice {
		nextPresent++
	}

	if shape.hasMaximum && (nextPresent > shape.maximum || nextPresent+remainingRequired > shape.maximum) {
		return false
	}

	if shape.allowsExtra {
		return true
	}

	remainingCapacity := uint64(len(shape.members) - index - 1)

	maximumPresent := nextPresent + remainingCapacity
	if shape.hasExact {
		return !shape.exactBeyond && maximumPresent >= shape.exact
	}

	return !shape.minimumBeyond && maximumPresent >= shape.minimum
}

// projectedMemberPresenceChoices puts the incremental canonical state first.
func projectedMemberPresenceChoices(
	shape *rowProjectedObject,
	member rowMember,
	constrained requirement,
	presenceConstrained bool,
	present uint64,
	remainingRequired uint64,
) []bool {
	if presenceConstrained && !constrained.canonical {
		return []bool{constrained.presence == requirementPresent}
	}

	if member.required {
		return []bool{true}
	}

	if presenceConstrained {
		preferred := constrained.presence == requirementPresent

		return []bool{preferred, !preferred}
	}

	target, beyond := shape.targetCount()
	preferred := beyond || present+remainingRequired < target

	return []bool{preferred, !preferred}
}

// projectedAdditionalMemberName returns one collision-free key without building a name list.
func projectedAdditionalMemberName(declared map[string]bool, values map[string]*jsonValue) string {
	const base = "__schematest_extra__"
	if !declared[base] {
		if _, used := values[base]; !used {
			return base
		}
	}

	for suffix := 0; ; suffix++ {
		candidate := fmt.Sprintf("%s_%d", base, suffix)
		if declared[candidate] {
			continue
		}

		if _, used := values[candidate]; !used {
			return candidate
		}
	}
}

// projectedAdditionalMember merges wildcard schemas at their exact active occurrences.
func projectedAdditionalMember(
	shape *rowProjectedObject,
	name string,
	requirements []requirement,
) (rowMember, bool, error) {
	sources := make([]rowSchemaSource, 0, len(shape.owners))
	for _, owner := range shape.owners {
		if _, declared := owner.node.properties[name]; declared {
			return rowMember{}, false, nil
		}

		if owner.node.additionalProperties != nil {
			sources = append(sources, rowSchemaSource{
				node: owner.node.additionalProperties,
				occurrence: rebasePlanOccurrence(
					owner.node.additionalProperties,
					owner.occurrence,
					owner.occurrence.usePointer+"/additionalProperties",
					appendInstanceToken(shape.rootOccurrence.instanceTemplate, name),
				),
			})

			continue
		}

		if !owner.node.allowAdditionalProperties {
			return rowMember{}, false, nil
		}
	}

	ordered := rowPreferredSchemaSources(sources, requirements)
	member := rowMember{
		name:    name,
		schemas: rowSchemaConjunction{sources: ordered},
		occurrence: schemaOccurrence{
			usePointer:       shape.rootOccurrence.usePointer + "/additionalProperties",
			targetPointer:    shape.rootOccurrence.targetPointer,
			instanceTemplate: appendInstanceToken(shape.rootOccurrence.instanceTemplate, name),
		},
	}

	member.schemas.fallback = member.occurrence
	if len(ordered) > 0 {
		member.occurrence = ordered[0].occurrence
		member.schemas.fallback = ordered[0].occurrence
	}

	return member, true, nil
}

// walkRowMemberValues walks the one schema conjunction selected by the active projection.
func (s *search) walkRowMemberValues(
	member rowMember,
	requirements []requirement,
	context rowSearchContext,
	visit rowVisit,
) (bool, error) {
	return s.walkRowSchemaConjunction(member.schemas, requirements, context, visit)
}

// rowObjectMembers exposes the first requested active view for fault regeneration.
// Composition-constrained fault requests have exactly one such view.
//
//nolint:cyclop // Active declared members and explicit forbidden fault names share one projection.
func rowObjectMembers(node *schemaNode, occurrence schemaOccurrence, requirements []requirement) ([]rowMember, error) {
	cursor := newRowProjectionCursor(node, occurrence, requirements)
	defer cursor.Close()

	view, ok, err := cursor.Next()
	if err != nil || !ok {
		return nil, err
	}

	shape, err := newRowProjectedObject(view, requirements, occurrence)
	if err != nil {
		return nil, err
	}

	members := append([]rowMember(nil), shape.members...)

	known := make(map[string]bool, len(members))
	for _, member := range members {
		known[member.name] = true
	}

	for _, requirement := range requirements {
		name, child := rowChildName(occurrence.instanceTemplate, requirement.occurrence.instanceTemplate)
		if !child || name == "*" || known[name] || requirement.presence == requirementNoPresence {
			continue
		}

		var sources []rowSchemaSource

		view.eachSource(func(owner rowSchemaSource) bool {
			if property, exists := rowChildSchemaSource(
				owner.node, owner.occurrence, rowChildProperty, name,
			); exists {
				sources = append(sources, property)
			} else if owner.node.additionalProperties != nil {
				sources = append(sources, rowSchemaSource{
					node: owner.node.additionalProperties,
					occurrence: rebasePlanOccurrence(
						owner.node.additionalProperties,
						owner.occurrence,
						owner.occurrence.usePointer+"/additionalProperties",
						appendInstanceToken(occurrence.instanceTemplate, name),
					),
				})
			}

			return true
		})

		ordered := rowPreferredSchemaSources(sources, requirements)

		member := rowMember{
			name: name, occurrence: requirement.occurrence,
			schemas: rowSchemaConjunction{sources: ordered, fallback: requirement.occurrence},
		}
		if len(ordered) > 0 {
			member.occurrence = ordered[0].occurrence
			member.schemas.fallback = ordered[0].occurrence
		}

		members = append(members, member)
		known[name] = true
	}

	sort.Slice(members, func(left, right int) bool { return members[left].name < members[right].name })

	return members, nil
}

// rowChildName extracts one data member token below an instance template.
func rowChildName(parent, child string) (string, bool) {
	parentTokens, parentOK := rowPointerTokens(parent)

	childTokens, childOK := rowPointerTokens(child)
	if !parentOK || !childOK || len(childTokens) != len(parentTokens)+1 {
		return "", false
	}

	for index, token := range parentTokens {
		if token != childTokens[index] && token != "*" {
			return "", false
		}
	}

	return childTokens[len(parentTokens)], true
}

// rowPresenceRequirementDetails prefers a hard target requirement over a canonical starting assignment.
func rowPresenceRequirementDetails(requirements []requirement, occurrence schemaOccurrence) (requirement, bool) {
	var canonical requirement

	canonicalFound := false

	for _, requirement := range requirements {
		if requirement.presence == requirementNoPresence || !rowOccurrenceMatches(requirement.occurrence, occurrence) {
			continue
		}

		if !requirement.canonical {
			return requirement, true
		}

		if !canonicalFound {
			canonical = requirement
			canonicalFound = true
		}
	}

	return canonical, canonicalFound
}

// walkGenericValue assigns a small complete value where no child schema is available.
func (s *search) walkGenericValue(_ []requirement, visit rowVisit) (bool, error) {
	for _, kind := range canonicalJSONKinds() {
		if err := s.assign(); err != nil {
			return false, err
		}

		var (
			complete bool
			visitErr error
		)

		err := walkCanonicalKindWitnesses(kind, func(candidate *jsonValue) bool {
			if visitErr = s.assign(); visitErr != nil {
				return false
			}

			complete, visitErr = visit(candidate)

			return visitErr == nil && !complete
		})
		if err != nil {
			return false, err
		}

		if visitErr != nil || complete {
			return complete, visitErr
		}
	}

	return false, nil
}
