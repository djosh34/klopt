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
	node       *schemaNode
	occurrence schemaOccurrence
	required   bool
}

const (
	// rowArrayLengthExactPhase emits a directed exact-count requirement.
	rowArrayLengthExactPhase uint8 = iota
	// rowArrayLengthMinimumPhase emits the effective composed minimum.
	rowArrayLengthMinimumPhase
	// rowArrayLengthMaximumPhase emits the effective composed maximum.
	rowArrayLengthMaximumPhase
	// rowArrayLengthRemainingPhase emits the remaining numeric count domain.
	rowArrayLengthRemainingPhase
)

// rowArrayLengthCursor yields exact structural counts without retaining a frontier.
type rowArrayLengthCursor struct {
	exact        uint64
	minimum      uint64
	maximum      uint64
	hasExact     bool
	hasMaximum   bool
	phase        uint8
	remaining    uint64
	remainingEnd bool
}

// newRowArrayLengthCursor intersects the active projection's count rules.
//
//nolint:cyclop,gocognit // Exact requirements, active minima, and maxima are one cursor initialization.
func newRowArrayLengthCursor(
	view rowProjectionView,
	requirements []requirement,
) (*rowArrayLengthCursor, error) {
	cursor := &rowArrayLengthCursor{}

	var cursorErr error

	view.eachSource(func(source rowSchemaSource) bool {
		minimum, minimumFits, err := exactCountUint64(source.node.minItems)
		if err != nil {
			cursorErr = err

			return false
		}

		if minimumFits && minimum > cursor.minimum {
			cursor.minimum = minimum
		}

		if item, exists := rowChildSchemaSource(source.node, source.occurrence, rowChildItems, ""); exists {
			presence, constrained := rowPresenceRequirementDetails(requirements, item.occurrence)
			if constrained && !presence.canonical && presence.presence == requirementPresent && cursor.minimum < 1 {
				cursor.minimum = 1
			}
		}

		maximum, maximumFits, err := exactCountUint64(source.node.maxItems)
		if err != nil {
			cursorErr = err

			return false
		}

		if maximumFits && (!cursor.hasMaximum || maximum < cursor.maximum) {
			cursor.maximum = maximum
			cursor.hasMaximum = true
		}

		if cursor.hasExact {
			return true
		}

		for _, requirement := range requirements {
			if requirement.tag != requirementExactCount || requirement.count == nil ||
				!rowOccurrenceMatches(requirement.occurrence, source.occurrence) {
				continue
			}

			exact, fits, exactErr := exactCountUint64(requirement.count)
			if exactErr != nil {
				cursorErr = exactErr

				return false
			}

			if fits {
				cursor.exact = exact
				cursor.hasExact = true
			}

			break
		}

		return true
	})

	if cursorErr != nil {
		return nil, cursorErr
	}

	return cursor, nil
}

// Next returns the next first-occurrence count. Open cursors never exhaust locally.
func (cursor *rowArrayLengthCursor) Next() (uint64, bool) {
	if candidate, ok := cursor.nextNamed(); ok {
		return candidate, true
	}

	for !cursor.remainingEnd {
		candidate := cursor.remaining
		if cursor.hasMaximum && candidate == cursor.maximum {
			cursor.remainingEnd = true
		} else {
			cursor.remaining++
		}

		if !cursor.namedBefore(candidate, rowArrayLengthRemainingPhase) {
			return candidate, true
		}
	}

	return 0, false
}

// nextNamed yields only the exact requirement and effective boundaries.
func (cursor *rowArrayLengthCursor) nextNamed() (uint64, bool) {
	for cursor.phase < rowArrayLengthRemainingPhase {
		phase := cursor.phase
		cursor.phase++

		switch phase {
		case rowArrayLengthExactPhase:
			if cursor.hasExact {
				return cursor.exact, true
			}
		case rowArrayLengthMinimumPhase:
			if !cursor.namedBefore(cursor.minimum, rowArrayLengthMinimumPhase) {
				return cursor.minimum, true
			}
		case rowArrayLengthMaximumPhase:
			if cursor.hasMaximum && !cursor.namedBefore(cursor.maximum, rowArrayLengthMaximumPhase) {
				return cursor.maximum, true
			}
		}
	}

	return 0, false
}

// namedBefore reports whether an earlier named phase already yielded one count.
func (cursor *rowArrayLengthCursor) namedBefore(candidate uint64, phase uint8) bool {
	return phase > rowArrayLengthExactPhase && cursor.hasExact && candidate == cursor.exact ||
		phase > rowArrayLengthMinimumPhase && candidate == cursor.minimum ||
		phase > rowArrayLengthMaximumPhase && cursor.hasMaximum && candidate == cursor.maximum
}

// walkProjectedDirectArrays offers complete composed array witnesses before repair search.
//
//nolint:cyclop // Projection discovery, charging, cloning, and visiting are one lazy source operation.
func (s *search) walkProjectedDirectArrays(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	visit rowVisit,
) (bool, error) {
	cursor := newRowProjectionCursor(node, occurrence, requirements)
	defer cursor.Close()

	for {
		view, ok, err := cursor.Next()
		if err != nil {
			return false, err
		}

		if !ok {
			return false, nil
		}

		var candidates bool

		err = view.eachDirectValue(func(_ rowSchemaSource, candidate *jsonValue) bool {
			if candidate.kind == jsonArray {
				candidates = true
			}

			return !candidates
		})
		if err != nil {
			return false, err
		}

		if !candidates {
			continue
		}

		if _, activeErr := view.appendBranchRequirements(
			append([]requirement(nil), requirements...), s.assign,
		); activeErr != nil {
			return false, activeErr
		}

		var (
			complete bool
			visitErr error
		)

		err = view.eachDirectValue(func(_ rowSchemaSource, candidate *jsonValue) bool {
			if candidate.kind != jsonArray {
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

		if visitErr != nil || complete {
			return complete, visitErr
		}
	}
}

// walkArray fairly advances every active projection across the open count frontier.
//
//nolint:cyclop // Projection rounds and the sole global cutoff share one lazy traversal boundary.
func (s *search) walkArray(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	context rowSearchContext,
	visit rowVisit,
) (bool, error) {
	complete, err := s.walkProjectedDirectArrays(node, occurrence, requirements, visit)
	if err != nil || complete {
		return complete, err
	}

	for round := uint64(0); ; round++ {
		cursor := newRowProjectionCursor(node, occurrence, requirements)
		attempted := false

		for {
			view, ok, err := cursor.Next()
			if err != nil {
				cursor.Close()

				return false, err
			}

			if !ok {
				break
			}

			complete, viewAttempted, err := s.walkProjectedArray(
				view, requirements, context, round, visit,
			)
			attempted = attempted || viewAttempted

			if err != nil || complete {
				cursor.Close()

				return complete, err
			}
		}

		cursor.Close()

		if round > 0 && !attempted {
			return false, nil
		}

		if round == ^uint64(0) {
			return false, nil
		}
	}
}

// walkProjectedArray constructs the named boundaries or one fair remaining count.
//
//nolint:cyclop // Named boundaries and one fair open count share the charged assignment operation.
func (s *search) walkProjectedArray(
	view rowProjectionView,
	requirements []requirement,
	context rowSearchContext,
	round uint64,
	visit rowVisit,
) (bool, bool, error) {
	lengths, err := newRowArrayLengthCursor(view, requirements)
	if err != nil {
		return false, false, err
	}

	item, err := rowProjectedArrayItem(view, requirements)
	if err != nil {
		return false, false, err
	}

	walkLength := func(length uint64) (bool, error) {
		activeRequirements, activeErr := view.appendBranchRequirements(
			append([]requirement(nil), requirements...), s.assign,
		)
		if activeErr != nil {
			return false, activeErr
		}

		if assignErr := s.assign(); assignErr != nil {
			return false, assignErr
		}

		return s.walkArrayElements(
			item.node, item.occurrence, activeRequirements, context, []*jsonValue{}, 0, length, visit,
		)
	}

	if round == 0 {
		attempted := false

		for {
			length, ok := lengths.nextNamed()
			if !ok {
				return false, attempted, nil
			}

			attempted = true

			complete, walkErr := walkLength(length)
			if walkErr != nil || complete {
				return complete, true, walkErr
			}
		}
	}

	length := round - 1
	if lengths.hasMaximum && length > lengths.maximum ||
		lengths.namedBefore(length, rowArrayLengthRemainingPhase) {
		return false, false, nil
	}

	complete, err := walkLength(length)

	return complete, true, err
}

// rowProjectedArrayItem merges only the item schemas in the current active view.
func rowProjectedArrayItem(
	view rowProjectionView,
	requirements []requirement,
) (rowSchemaChoice, error) {
	var (
		sources  []rowSchemaSource
		fallback schemaOccurrence
	)

	view.eachSource(func(source rowSchemaSource) bool {
		if fallback.usePointer == "" {
			fallback = rowChildOccurrence(source.node, source.occurrence, rowChildItems, "")
		}

		if item, exists := rowChildSchemaSource(source.node, source.occurrence, rowChildItems, ""); exists {
			sources = append(sources, item)
		}

		return true
	})

	ordered := rowPreferredSchemaSources(sources, requirements)
	for index, source := range ordered {
		if source.node.enum == nil && source.node.defaultValue == nil {
			continue
		}

		if index > 0 {
			selected := ordered[index]
			copy(ordered[1:index+1], ordered[:index])
			ordered[0] = selected
		}

		break
	}

	choice, exists, err := mergeRowSchemaSources(ordered)
	if err != nil {
		return rowSchemaChoice{}, err
	}

	if !exists {
		choice.occurrence = fallback
	}

	return choice, nil
}

// walkArrayElements appends one independently owned item after its charged assignment.
func (s *search) walkArrayElements(
	item *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	context rowSearchContext,
	elements []*jsonValue,
	index, length uint64,
	visit rowVisit,
) (bool, error) {
	if index == length {
		return visit(&jsonValue{kind: jsonArray, array: elements})
	}

	walkValue := func(value *jsonValue) (bool, error) {
		owned, err := cloneJSONValue(value)
		if err != nil {
			return false, err
		}

		elements = append(elements, owned)
		complete, err := s.walkArrayElements(
			item, occurrence, requirements, context, elements, index+1, length, visit,
		)
		elements = elements[:len(elements)-1]

		return complete, err
	}

	if item == nil {
		return s.walkGenericValue(requirements, walkValue)
	}

	return s.walkNode(item, occurrence, requirements, context, func(value *jsonValue) (bool, error) {
		usable, err := s.rowChildValueUsable(item, occurrence, requirements, value)
		if err != nil || !usable {
			return false, err
		}

		return walkValue(value)
	})
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
}

// walkObject offers complete projected witnesses, then constructs each active shape lazily.
//
//nolint:cyclop // Cursor advancement, shape compilation, and DFS handoff are one lazy boundary.
func (s *search) walkObject(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	context rowSearchContext,
	visit rowVisit,
) (bool, error) {
	complete, err := s.walkProjectedDirectObjects(node, occurrence, requirements, visit)
	if err != nil || complete {
		return complete, err
	}

	cursor := newRowProjectionCursor(node, occurrence, requirements)
	defer cursor.Close()

	for {
		view, ok, cursorErr := cursor.Next()
		if cursorErr != nil {
			return false, cursorErr
		}

		if !ok {
			return false, nil
		}

		active, activeErr := view.appendBranchRequirements(
			append([]requirement(nil), requirements...), s.assign,
		)
		if activeErr != nil {
			return false, activeErr
		}

		shape, shapeErr := newRowProjectedObject(view, active, occurrence)
		if shapeErr != nil {
			return false, shapeErr
		}

		if !shape.feasible() {
			continue
		}

		values := make(map[string]*jsonValue)

		complete, err = s.walkProjectedObjectMembers(
			shape, active, context, values, 0, 0, 0, shape.requiredFrom(0), visit,
		)
		if err != nil || complete {
			return complete, err
		}
	}
}

// walkProjectedDirectObjects offers authored object enum/default values without retaining them.
//
//nolint:cyclop // Cursor, source, assignment, clone, and visit errors remain explicit.
func (s *search) walkProjectedDirectObjects(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	visit rowVisit,
) (bool, error) {
	cursor := newRowProjectionCursor(node, occurrence, requirements)
	defer cursor.Close()

	for {
		view, ok, err := cursor.Next()
		if err != nil {
			return false, err
		}

		if !ok {
			return false, nil
		}

		var (
			complete bool
			visitErr error
		)

		err = view.eachDirectValue(func(_ rowSchemaSource, candidate *jsonValue) bool {
			if candidate.kind != jsonObject {
				return true
			}

			if _, visitErr = view.appendBranchRequirements(
				append([]requirement(nil), requirements...), s.assign,
			); visitErr != nil {
				return false
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

		if visitErr != nil || complete {
			return complete, visitErr
		}
	}
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
				return shape, nil
			}

			continue
		}

		ordered := rowPreferredSchemaSources(sources, requirements)

		choice, exists, err := mergeRowSchemaSources(ordered)
		if err != nil {
			return nil, err
		}

		member := rowMember{name: name}
		if exists {
			member.node = choice.node
			member.occurrence = choice.occurrence
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
	shape.requiresExtra = projectedAdditionalRequired(requirements, owners)

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
func projectedAdditionalRequired(requirements []requirement, owners []rowSchemaSource) bool {
	for _, requirement := range requirements {
		if requirement.presence != requirementPresent && !requirement.hasKind {
			continue
		}

		for _, owner := range owners {
			if owner.node.additionalProperties != nil &&
				requirement.occurrence.usePointer == owner.occurrence.usePointer+"/additionalProperties" &&
				instanceTemplateMatches(requirement.occurrence.instanceTemplate,
					appendInstanceToken(owner.occurrence.instanceTemplate, "*")) {
				return true
			}
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
	if shape.countConflict {
		return false
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

// walkProjectedObjectMembers advances presence with incremental present/required state.
//
//nolint:cyclop // Pruning, presence assignment, and child DFS form one backtracking operation.
func (s *search) walkProjectedObjectMembers(
	shape *rowProjectedObject,
	requirements []requirement,
	context rowSearchContext,
	values map[string]*jsonValue,
	index int,
	present uint64,
	extras uint64,
	remainingRequired uint64,
	visit rowVisit,
) (bool, error) {
	if shape.hasMaximum && present > shape.maximum {
		return false, nil
	}

	if index == len(shape.members) {
		return s.walkProjectedObjectExtras(shape, requirements, context, values, present, extras, visit)
	}

	member := shape.members[index]

	nextRequired := remainingRequired
	if member.required {
		nextRequired--
	}

	constrained, constrainedPresence := rowPresenceRequirementDetails(requirements, member.occurrence)

	choices := projectedMemberPresenceChoices(
		shape, member, constrained, constrainedPresence, present, nextRequired,
	)
	for _, choice := range choices {
		if err := s.assign(); err != nil {
			return false, err
		}

		if !choice {
			delete(values, member.name)

			complete, err := s.walkProjectedObjectMembers(
				shape, requirements, context, values, index+1, present, extras, nextRequired, visit,
			)
			if err != nil || complete {
				return complete, err
			}

			continue
		}

		walkValue := func(value *jsonValue) (bool, error) {
			values[member.name] = value

			complete, err := s.walkProjectedObjectMembers(
				shape, requirements, context, values, index+1, present+1, extras, nextRequired, visit,
			)
			if !complete {
				delete(values, member.name)
			}

			return complete, err
		}

		complete, err := s.walkRowMemberValues(member, requirements, context, walkValue)
		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
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

// walkProjectedObjectExtras generates and assigns only the next needed synthetic member.
//
//nolint:cyclop // Bound checks, name assignment, wildcard merge, and child DFS stay atomic.
func (s *search) walkProjectedObjectExtras(
	shape *rowProjectedObject,
	requirements []requirement,
	context rowSearchContext,
	values map[string]*jsonValue,
	present uint64,
	extras uint64,
	visit rowVisit,
) (bool, error) {
	target, beyond := shape.targetCount()
	if !beyond && present >= target && (!shape.requiresExtra || extras > 0) {
		return visit(&jsonValue{kind: jsonObject, object: values})
	}

	if !shape.allowsExtra || shape.hasMaximum && present >= shape.maximum {
		return false, nil
	}

	if err := s.assign(); err != nil {
		return false, err
	}

	name := projectedAdditionalMemberName(shape.declared, values)

	member, allowed, err := projectedAdditionalMember(shape, name, requirements)
	if err != nil || !allowed {
		return false, err
	}

	walkValue := func(value *jsonValue) (bool, error) {
		values[name] = value

		complete, err := s.walkProjectedObjectExtras(
			shape, requirements, context, values, present+1, extras+1, visit,
		)
		if !complete {
			delete(values, name)
		}

		return complete, err
	}

	return s.walkRowMemberValues(member, requirements, context, walkValue)
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

	choice, exists, err := mergeRowSchemaSources(ordered)
	if err != nil {
		return rowMember{}, false, err
	}

	member := rowMember{
		name: name,
		occurrence: schemaOccurrence{
			usePointer:       shape.rootOccurrence.usePointer + "/additionalProperties",
			targetPointer:    shape.rootOccurrence.targetPointer,
			instanceTemplate: appendInstanceToken(shape.rootOccurrence.instanceTemplate, name),
		},
	}
	if exists {
		member.node = choice.node
		member.occurrence = choice.occurrence
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
	if member.node == nil {
		return s.walkGenericValue(requirements, visit)
	}

	return s.walkNode(
		member.node, member.occurrence, requirements, context,
		func(value *jsonValue) (bool, error) {
			usable, err := s.rowChildValueUsable(member.node, member.occurrence, requirements, value)
			if err != nil || !usable {
				return false, err
			}

			return visit(value)
		},
	)
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

		choice, exists, mergeErr := mergeRowSchemaSources(rowPreferredSchemaSources(sources, requirements))
		if mergeErr != nil {
			return nil, mergeErr
		}

		member := rowMember{name: name, occurrence: requirement.occurrence}
		if exists {
			member.node = choice.node
			member.occurrence = choice.occurrence
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
