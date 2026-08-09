package schematest

import (
	"fmt"
	"sort"
	"strings"
)

// rowMember describes one object member choice and its best clean schema occurrence.
type rowMember struct {
	name         string
	node         *schemaNode
	occurrence   schemaOccurrence
	required     bool
	alternatives []rowMember
}

// rowAdditionalPropertySource identifies a wildcard schema and its declaring object.
type rowAdditionalPropertySource struct {
	source rowSchemaSource
	owner  *schemaNode
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

// walkObject assigns members in canonical UTF-8 name order.
func (s *search) walkObject(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	context rowSearchContext,
	visit rowVisit,
) (bool, error) {
	members, err := rowObjectMembers(node, occurrence, requirements)
	if err != nil {
		return false, err
	}

	values := make(map[string]*jsonValue, len(members))

	return s.walkObjectMembers(node, occurrence, requirements, context, members, values, 0, visit)
}

// walkObjectMembers performs deterministic presence and value backtracking.
func (s *search) walkObjectMembers(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	context rowSearchContext,
	members []rowMember,
	values map[string]*jsonValue,
	index int,
	visit rowVisit,
) (bool, error) {
	if index == len(members) {
		return visit(&jsonValue{kind: jsonObject, object: values})
	}

	member := members[index]
	presence, presenceConstrained := rowPresenceRequirementDetails(requirements, member.occurrence)

	choices, err := rowMemberPresenceChoices(
		node, occurrence, requirements, members, index, member, presence, presenceConstrained,
	)
	if err != nil {
		return false, err
	}

	for _, present := range choices {
		if err := s.assign(); err != nil {
			return false, err
		}

		if !present {
			delete(values, member.name)

			complete, err := s.walkObjectMembers(
				node, occurrence, requirements, context, members, values, index+1, visit,
			)
			if err != nil || complete {
				return complete, err
			}

			continue
		}

		walkValue := func(value *jsonValue) (bool, error) {
			values[member.name] = value

			return s.walkObjectMembers(node, occurrence, requirements, context, members, values, index+1, visit)
		}

		complete, err := s.walkRowMemberValues(member, requirements, context, walkValue)
		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}

// walkRowMemberValues tries the merged property schema and any inactive-branch alternatives.
//
//nolint:cyclop // Schema alternatives and child recursion are one DFS phase.
func (s *search) walkRowMemberValues(
	member rowMember,
	requirements []requirement,
	context rowSearchContext,
	visit rowVisit,
) (bool, error) {
	candidates := make([]rowMember, 0, 1+len(member.alternatives))
	candidates = append(candidates, member)
	candidates = append(candidates, member.alternatives...)

	for _, candidate := range candidates {
		if len(candidates) > 1 {
			if err := s.assign(); err != nil {
				return false, err
			}
		}

		if candidate.node == nil {
			complete, err := s.walkGenericValue(requirements, visit)
			if err != nil || complete {
				return complete, err
			}

			continue
		}

		complete, err := s.walkNode(
			candidate.node, candidate.occurrence, requirements, context,
			func(value *jsonValue) (bool, error) {
				usable, err := s.rowChildValueUsable(candidate.node, candidate.occurrence, requirements, value)
				if err != nil {
					return false, err
				}

				if !usable {
					return false, nil
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

// rowMemberPresenceChoices puts the canonical assignment first and repairs second.
func rowMemberPresenceChoices(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	members []rowMember,
	index int,
	member rowMember,
	constrained requirement,
	presenceConstrained bool,
) ([]bool, error) {
	if presenceConstrained && !constrained.canonical {
		if constrained.presence == requirementPresent {
			return []bool{true}, nil
		}

		return []bool{false}, nil
	}

	if member.required {
		return []bool{true}, nil
	}

	if presenceConstrained {
		if constrained.presence == requirementPresent {
			return []bool{true, false}, nil
		}

		return []bool{false, true}, nil
	}

	present, err := rowCanonicalMemberPresence(node, occurrence, requirements, members, index)
	if err != nil {
		return nil, err
	}

	if present {
		return []bool{true, false}, nil
	}

	return []bool{false, true}, nil
}

// rowObjectMinimumProperties returns the active local and composition lower bound.
func rowObjectMinimumProperties(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
) (uint64, bool, error) {
	return rowNestedObjectMinimumProperties(node, occurrence, requirements, make(map[*schemaNode]bool))
}

// rowNestedObjectMinimumProperties carries object lower bounds through active composition.
//
//nolint:cyclop,gocognit // Local, allOf, and constrained anyOf bounds are one recursive calculation.
func rowNestedObjectMinimumProperties(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	visiting map[*schemaNode]bool,
) (uint64, bool, error) {
	if node == nil || node.schemaShape == nil || !nodeCanHaveKind(node, jsonObject) {
		return 0, false, nil
	}

	if visiting[node] {
		return 0, false, fmt.Errorf("schematest: recursive row object bounds at %s", occurrence.usePointer)
	}

	visiting[node] = true
	defer delete(visiting, node)

	minimum := uint64(0)
	fits := false
	consider := func(candidate *exactCount) error {
		count, candidateFits, err := exactCountUint64(candidate)
		if err != nil {
			return err
		}

		if candidateFits && (!fits || count > minimum) {
			minimum = count
			fits = true
		}

		return nil
	}

	if err := consider(node.minProperties); err != nil {
		return 0, false, err
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		childMinimum, childFits, err := rowNestedObjectMinimumProperties(
			child, childOccurrence, requirements, visiting,
		)
		if err != nil {
			return 0, false, err
		}

		if childFits && (!fits || childMinimum > minimum) {
			minimum = childMinimum
			fits = true
		}
	}

	states, constrained := rowCompositionTruthStates(requirements, occurrence, "anyOf", len(node.anyOf))
	if constrained {
		for index, child := range node.anyOf {
			if !states[index] {
				continue
			}

			childOccurrence := rebasePlanOccurrence(
				child,
				occurrence,
				occurrence.usePointer+"/anyOf/"+itoa(index),
				occurrence.instanceTemplate,
			)

			childMinimum, childFits, err := rowNestedObjectMinimumProperties(
				child, childOccurrence, requirements, visiting,
			)
			if err != nil {
				return 0, false, err
			}

			if childFits && (!fits || childMinimum > minimum) {
				minimum = childMinimum
				fits = true
			}
		}
	}

	return minimum, fits, nil
}

// rowCanonicalMemberPresence supplies required members and lower-bound members first.
func rowCanonicalMemberPresence(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	members []rowMember,
	index int,
) (bool, error) {
	member := members[index]
	if member.required {
		return true, nil
	}

	minimum, fits, err := rowObjectMinimumProperties(node, occurrence, requirements)
	if err != nil {
		return false, err
	}

	if !fits {
		return false, nil
	}

	present := 0

	for prior := 0; prior < index; prior++ {
		priorPresent, err := rowCanonicalMemberPresence(node, occurrence, requirements, members, prior)
		if err != nil {
			return false, err
		}

		if members[prior].required || priorPresent {
			present++
		}
	}

	return uint64(present) < minimum, nil
}

// rowObjectMembers collects direct, composed, required, and constrained member names.
//
//nolint:cyclop,gocognit // Direct, composed, additional, and requirement selection share one canonical pass.
func rowObjectMembers(node *schemaNode, occurrence schemaOccurrence, requirements []requirement) ([]rowMember, error) {
	specs := make(map[string][]rowMember)

	required := make(map[string]bool)
	if err := collectRowObjectMembers(
		node, occurrence, specs, required, requirements, true, make(map[*schemaNode]bool),
	); err != nil {
		return nil, err
	}

	for _, requirement := range requirements {
		if name, ok := rowChildName(occurrence.instanceTemplate, requirement.occurrence.instanceTemplate); ok {
			if name == "*" && strings.HasSuffix(requirement.occurrence.usePointer, "/additionalProperties") {
				continue
			}

			if requirement.presence != requirementNoPresence || requirement.hasKind {
				if _, exists := specs[name]; !exists {
					specs[name] = nil
				}
			}
		}
	}

	additionalSourceSets, err := rowAdditionalPropertySources(node, occurrence, requirements)
	if err != nil {
		return nil, err
	}

	additionalMembersForName := func(name string) [][]rowMember {
		result := make([][]rowMember, 0, len(additionalSourceSets))

		for _, sourceSet := range additionalSourceSets {
			members := make([]rowMember, 0, len(sourceSet))
			for _, additional := range sourceSet {
				if additional.owner != nil {
					if _, declared := additional.owner.properties[name]; declared {
						continue
					}
				}

				source := additional.source
				members = append(members, rowMember{
					name: name,
					node: source.node,
					occurrence: rebasePlanOccurrence(
						source.node,
						source.occurrence,
						source.occurrence.usePointer,
						appendInstanceToken(occurrence.instanceTemplate, name),
					),
				})
			}

			result = append(result, members)
		}

		return result
	}

	extraNames, err := rowAdditionalMemberNames(node, specs, requirements, occurrence)
	if err != nil {
		return nil, err
	}

	for _, name := range extraNames {
		if _, exists := specs[name]; !exists {
			specs[name] = nil
		}
	}

	names := make([]string, 0, len(specs))
	for name := range specs {
		if _, declared := node.properties[name]; !declared &&
			!node.allowAdditionalProperties && len(specs[name]) == 0 {
			continue
		}

		names = append(names, name)
	}

	sort.Strings(names)

	members := make([]rowMember, 0, len(names))
	for _, name := range names {
		member, err := composeRowMemberAlternatives(
			name,
			specs[name],
			additionalMembersForName(name),
			required[name],
			requirements,
		)
		if err != nil {
			return nil, err
		}

		members = append(members, member)
	}

	return members, nil
}

// composeRowMemberAlternatives keeps mutually exclusive wildcard schemas separate.
func composeRowMemberAlternatives(
	name string,
	base []rowMember,
	sourceSets [][]rowMember,
	required bool,
	requirements []requirement,
) (rowMember, error) {
	if len(sourceSets) == 0 {
		sourceSets = [][]rowMember{{}}
	} else {
		hasSource := false

		for _, sources := range sourceSets {
			if len(sources) > 0 {
				hasSource = true

				break
			}
		}

		if !hasSource {
			sourceSets = [][]rowMember{{}}
		}
	}

	composed := make([]rowMember, 0, len(sourceSets))
	for _, sources := range sourceSets {
		candidates := make([]rowMember, 0, len(base)+len(sources))
		candidates = append(candidates, base...)
		candidates = append(candidates, sources...)

		if len(candidates) == 0 {
			candidates = append(candidates, rowMember{name: name, required: required})
		}

		member, err := composeRowMember(name, candidates, required, requirements)
		if err != nil {
			return rowMember{}, err
		}

		composed = append(composed, member)
	}

	member := composed[0]
	member.alternatives = append(member.alternatives, composed[1:]...)

	return member, nil
}

// rowAdditionalPropertySources returns active direct and composed wildcard alternatives.
//
//nolint:cyclop,gocognit // Direct, allOf, and anyOf wildcard sources share one recursive pass.
func rowAdditionalPropertySources(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
) ([][]rowAdditionalPropertySource, error) {
	visiting := make(map[*schemaNode]bool)

	var collect func(*schemaNode, schemaOccurrence) ([][]rowAdditionalPropertySource, error)

	collect = func(current *schemaNode, currentOccurrence schemaOccurrence) ([][]rowAdditionalPropertySource, error) {
		if current == nil || current.schemaShape == nil {
			return [][]rowAdditionalPropertySource{{}}, nil
		}

		if visiting[current] {
			return nil, fmt.Errorf("schematest: recursive row additional-properties shape at %s", currentOccurrence.usePointer)
		}

		visiting[current] = true
		defer delete(visiting, current)

		common := [][]rowAdditionalPropertySource{{}}

		if current.additionalProperties != nil {
			additionalOccurrence := rebasePlanOccurrence(
				current.additionalProperties,
				currentOccurrence,
				currentOccurrence.usePointer+"/additionalProperties",
				appendInstanceToken(currentOccurrence.instanceTemplate, "*"),
			)
			common[0] = append(common[0], rowAdditionalPropertySource{
				source: rowSchemaSource{
					node:       current.additionalProperties,
					occurrence: additionalOccurrence,
				},
				owner: current,
			})
		}

		for index, child := range current.allOf {
			childOccurrence := rebasePlanOccurrence(
				child,
				currentOccurrence,
				currentOccurrence.usePointer+"/allOf/"+itoa(index),
				currentOccurrence.instanceTemplate,
			)

			childSets, err := collect(child, childOccurrence)
			if err != nil {
				return nil, err
			}

			common = combineRowAdditionalPropertySourceSets(common, childSets)
		}

		if len(current.anyOf) == 0 {
			return common, nil
		}

		states, constrained := rowCompositionTruthStates(requirements, currentOccurrence, "anyOf", len(current.anyOf))
		if constrained {
			selected := common

			for index, child := range current.anyOf {
				if !states[index] {
					continue
				}

				childOccurrence := rebasePlanOccurrence(
					child,
					currentOccurrence,
					currentOccurrence.usePointer+"/anyOf/"+itoa(index),
					currentOccurrence.instanceTemplate,
				)

				childSets, err := collect(child, childOccurrence)
				if err != nil {
					return nil, err
				}

				selected = combineRowAdditionalPropertySourceSets(selected, childSets)
			}

			return selected, nil
		}

		alternatives := make([][]rowAdditionalPropertySource, 0, len(current.anyOf))
		for index, child := range current.anyOf {
			childOccurrence := rebasePlanOccurrence(
				child,
				currentOccurrence,
				currentOccurrence.usePointer+"/anyOf/"+itoa(index),
				currentOccurrence.instanceTemplate,
			)

			childSets, err := collect(child, childOccurrence)
			if err != nil {
				return nil, err
			}

			alternatives = append(
				alternatives,
				combineRowAdditionalPropertySourceSets(common, childSets)...,
			)
		}

		return alternatives, nil
	}

	return collect(node, occurrence)
}

// combineRowAdditionalPropertySourceSets computes the allOf product of wildcard alternatives.
func combineRowAdditionalPropertySourceSets(
	left [][]rowAdditionalPropertySource,
	right [][]rowAdditionalPropertySource,
) [][]rowAdditionalPropertySource {
	result := make([][]rowAdditionalPropertySource, 0, len(left)*len(right))

	for _, leftSources := range left {
		for _, rightSources := range right {
			sources := make([]rowAdditionalPropertySource, 0, len(leftSources)+len(rightSources))
			sources = append(sources, leftSources...)
			sources = append(sources, rightSources...)
			result = append(result, sources)
		}
	}

	return result
}

// rowAdditionalMemberNames adds wildcard members needed by lower bounds or target requirements.
func rowAdditionalMemberNames(
	node *schemaNode,
	specified map[string][]rowMember,
	requirements []requirement,
	occurrence schemaOccurrence,
) ([]string, error) {
	if !node.allowAdditionalProperties {
		return nil, nil
	}

	minimum, fits, err := rowObjectMinimumProperties(node, occurrence, requirements)
	if err != nil {
		return nil, err
	}

	needed := 0

	if fits && minimum > uint64(len(specified)) {
		if minimum-uint64(len(specified)) > uint64(^uint(0)>>1) {
			return nil, nil
		}

		needed = int(minimum) - len(specified)
	}

	if rowAdditionalPresenceConstrained(occurrence, requirements) && needed == 0 {
		needed = 1
	}

	if needed == 0 {
		return nil, nil
	}

	result := make([]string, 0, needed)
	for index := 0; index < needed; index++ {
		result = append(result, rowAdditionalMemberName(node, specified, result, index))
	}

	return result, nil
}

// rowAdditionalMemberName picks an extra key that cannot collide with authored keys.
func rowAdditionalMemberName(
	node *schemaNode,
	specified map[string][]rowMember,
	chosen []string,
	index int,
) string {
	base := additionalPropertyWitnessName(node)
	if index == 0 && !containsString(chosen, base) {
		if _, exists := specified[base]; !exists {
			return base
		}
	}

	for suffix := index; ; suffix++ {
		candidate := fmt.Sprintf("%s_%d", base, suffix)
		if !containsString(chosen, candidate) {
			if _, exists := specified[candidate]; !exists {
				return candidate
			}
		}
	}
}

// collectRowObjectMembers recursively collects property schemas from compositions.
//
//nolint:cyclop // Direct, allOf, and anyOf requiredness share one recursive pass.
func collectRowObjectMembers(
	node *schemaNode,
	occurrence schemaOccurrence,
	specs map[string][]rowMember,
	required map[string]bool,
	requirements []requirement,
	requiredAllowed bool,
	visiting map[*schemaNode]bool,
) error {
	if node == nil || node.schemaShape == nil {
		return nil
	}

	if visiting[node] {
		return fmt.Errorf("schematest: recursive row object shape at %s", occurrence.usePointer)
	}

	visiting[node] = true
	defer delete(visiting, node)

	for _, name := range sortedSchemaPropertyNames(node.properties) {
		property := node.properties[name]
		propertyOccurrence := rebasePlanOccurrence(
			property,
			occurrence,
			occurrence.usePointer+"/properties/"+escapePointerToken(name),
			appendInstanceToken(occurrence.instanceTemplate, name),
		)
		specs[name] = append(specs[name], rowMember{
			name: name, node: property, occurrence: propertyOccurrence,
		})
	}

	if requiredAllowed {
		for _, name := range node.required {
			required[name] = true
		}
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if err := collectRowObjectMembers(
			child, childOccurrence, specs, required, requirements, requiredAllowed, visiting,
		); err != nil {
			return err
		}
	}

	for index, child := range node.anyOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/anyOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		branchRequired := requiredAllowed && rowCompositionRequirementTruth(requirements, childOccurrence, "anyOf", index)
		if err := collectRowObjectMembers(
			child, childOccurrence, specs, required, requirements, branchRequired, visiting,
		); err != nil {
			return err
		}
	}

	return nil
}

// rowCompositionRequirementTruth returns one constrained branch truth, defaulting to false.
func rowCompositionRequirementTruth(
	requirements []requirement,
	occurrence schemaOccurrence,
	composition string,
	branch int,
) bool {
	for _, requirement := range requirements {
		if requirement.hasBranch && requirement.composition == composition && requirement.branch == branch &&
			rowOccurrenceMatches(requirement.occurrence, occurrence) {
			return requirement.truth
		}
	}

	return false
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

// rowAdditionalPresenceConstrained reports whether the target explicitly asks for an extra member.
func rowAdditionalPresenceConstrained(occurrence schemaOccurrence, requirements []requirement) bool {
	wantedTemplate := appendInstanceToken(occurrence.instanceTemplate, "*")

	for _, requirement := range requirements {
		if requirement.canonical || requirement.presence != requirementPresent ||
			!strings.HasSuffix(requirement.occurrence.usePointer, "/additionalProperties") {
			continue
		}

		if requirement.occurrence.instanceTemplate == wantedTemplate {
			return true
		}
	}

	return false
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
