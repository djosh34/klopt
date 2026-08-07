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
	// rowLengthCandidateCapacity is the small transient array-length frontier.
	rowLengthCandidateCapacity = 8
	// rowSecondRepairLength is the second canonical repair length.
	rowSecondRepairLength = 2
)

// walkArray assigns an array length and recursively assigns every item.
func (s *search) walkArray(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	context rowSearchContext,
	visit rowVisit,
) (bool, error) {
	lengths, err := rowArrayLengths(node, occurrence, pins)
	if err != nil {
		return false, err
	}

	for _, length := range lengths {
		if err := s.assign(); err != nil {
			return false, err
		}

		elements := make([]*jsonValue, length)

		itemChoices, err := rowChildSchemaChoices(node, occurrence, pins, rowChildItems, "")
		if err != nil {
			return false, err
		}

		for _, itemChoice := range itemChoices {
			if len(itemChoices) > 1 {
				if err := s.assign(); err != nil {
					return false, err
				}
			}

			complete, err := s.walkArrayElements(
				itemChoice.node,
				itemChoice.occurrence,
				pins,
				context,
				elements,
				0,
				visit,
			)
			if err != nil || complete {
				return complete, err
			}
		}
	}

	return false, nil
}

// walkArrayElements assigns item values without retaining completed arrays.
func (s *search) walkArrayElements(
	item *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	context rowSearchContext,
	elements []*jsonValue,
	index int,
	visit rowVisit,
) (bool, error) {
	if index == len(elements) {
		return visit(&jsonValue{kind: jsonArray, array: elements})
	}

	if item == nil {
		return s.walkGenericValue(pins, func(value *jsonValue) (bool, error) {
			elements[index] = value

			return s.walkArrayElements(item, occurrence, pins, context, elements, index+1, visit)
		})
	}

	return s.walkNode(item, occurrence, pins, context, func(value *jsonValue) (bool, error) {
		usable, err := s.rowChildValueUsable(item, occurrence, pins, value)
		if err != nil {
			return false, err
		}

		if !usable {
			usable, err = directedStringChildUsable(context.stringObjective, item, occurrence, value)
			if err != nil || !usable {
				return false, err
			}
		}

		elements[index] = value

		return s.walkArrayElements(item, occurrence, pins, context, elements, index+1, visit)
	})
}

// rowArrayLengths returns canonical and repair lengths for one array occurrence.
//
//nolint:cyclop,gocognit,gocyclo // Bound extraction and deterministic repair choices are one structural phase.
func rowArrayLengths(node *schemaNode, occurrence schemaOccurrence, pins []applicabilityPin) ([]int, error) {
	minimum := 0
	maximum := int(^uint(0) >> 1)

	if count, fits, err := exactCountUint64(node.minItems); err != nil {
		return nil, err
	} else if fits {
		if count > uint64(maximum) {
			return nil, nil
		}

		minimum = int(count)
	}

	if count, fits, err := exactCountUint64(node.maxItems); err != nil {
		return nil, err
	} else if fits && count < uint64(maximum) {
		maximum = int(count)
	}

	composedBounds := [][2]int{{minimum, maximum}}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		childBounds, err := rowNestedArrayBounds(child, childOccurrence, pins)
		if err != nil {
			return nil, err
		}

		composedBounds = combineRowArrayBounds(composedBounds, childBounds)
	}

	anyOfStates, anyOfPinned := rowCompositionTruthStates(pins, occurrence, "anyOf", len(node.anyOf))
	if anyOfPinned {
		for index, child := range node.anyOf {
			if !anyOfStates[index] {
				continue
			}

			childOccurrence := rebasePlanOccurrence(
				child,
				occurrence.usePointer+"/anyOf/"+itoa(index),
				occurrence.instanceTemplate,
			)

			childBounds, err := rowNestedArrayBounds(child, childOccurrence, pins)
			if err != nil {
				return nil, err
			}

			composedBounds = combineRowArrayBounds(composedBounds, childBounds)
		}
	} else if len(node.anyOf) > 0 {
		alternativeBounds := make([][2]int, 0, len(node.anyOf))
		for index, child := range node.anyOf {
			childOccurrence := rebasePlanOccurrence(
				child,
				occurrence.usePointer+"/anyOf/"+itoa(index),
				occurrence.instanceTemplate,
			)

			childBounds, err := rowNestedArrayBounds(child, childOccurrence, pins)
			if err != nil {
				return nil, err
			}

			alternativeBounds = append(
				alternativeBounds,
				combineRowArrayBounds(composedBounds, childBounds)...,
			)
		}

		composedBounds = alternativeBounds
	}

	if len(composedBounds) == 1 {
		minimum = composedBounds[0][0]
		maximum = composedBounds[0][1]
	}

	itemPin, itemPinned := rowPresencePinDetails(pins, schemaOccurrence{
		usePointer:       occurrence.usePointer + "/items",
		targetPointer:    occurrence.targetPointer,
		instanceTemplate: appendInstanceToken(occurrence.instanceTemplate, "*"),
	})
	if !itemPinned {
		itemChoices, err := rowChildSchemaChoices(node, occurrence, pins, rowChildItems, "")
		if err != nil {
			return nil, err
		}

		for _, itemChoice := range itemChoices {
			candidatePin, exists := rowPresencePinDetails(pins, itemChoice.occurrence)
			if exists && (!itemPinned || !candidatePin.canonical) {
				itemPin, itemPinned = candidatePin, true
			}
		}
	}

	if itemPinned && !itemPin.canonical && itemPin.presence == planPinPresent && minimum < 1 {
		minimum = 1
	}

	defaultLengths, err := rowComposedArrayDefaultLengths(node, occurrence, pins)
	if err != nil {
		return nil, err
	}

	if minimum > maximum {
		return []int{minimum}, nil
	}

	candidates := make([]int, 0, rowLengthCandidateCapacity)
	appendLength := func(length int) {
		if length < 0 || length > maximum {
			return
		}

		for _, existing := range candidates {
			if existing == length {
				return
			}
		}

		candidates = append(candidates, length)
	}

	appendLength(minimum)
	appendLength(0)
	appendLength(1)
	appendLength(rowSecondRepairLength)
	appendLength(minimum + 1)

	if maximum < int(^uint(0)>>1) {
		appendLength(maximum)
	}

	for _, bounds := range composedBounds {
		appendLength(bounds[0])

		if bounds[1] < int(^uint(0)>>1) {
			appendLength(bounds[1])
		}
	}

	for _, length := range defaultLengths {
		appendLength(length)
	}

	return candidates, nil
}

// rowComposedArrayDefaultLengths preserves active authored array defaults.
func rowComposedArrayDefaultLengths(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
) ([]int, error) {
	return rowComposedArrayDefaultLengthsAt(node, occurrence, pins, make(map[*schemaNode]bool))
}

// rowComposedArrayDefaultLengthsAt recursively collects nested authored defaults.
//
//nolint:cyclop // Direct, allOf, and pinned anyOf defaults share one recursive pass.
func rowComposedArrayDefaultLengthsAt(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	visiting map[*schemaNode]bool,
) ([]int, error) {
	if node == nil || node.schemaShape == nil {
		return nil, nil
	}

	if visiting[node] {
		return nil, fmt.Errorf("schematest: recursive row array default at %s", occurrence.usePointer)
	}

	visiting[node] = true
	defer delete(visiting, node)

	lengths := make([]int, 0, 1)
	if node.defaultValue != nil && node.defaultValue.kind == jsonArray {
		lengths = append(lengths, len(node.defaultValue.array))
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		childLengths, err := rowComposedArrayDefaultLengthsAt(child, childOccurrence, pins, visiting)
		if err != nil {
			return nil, err
		}

		lengths = append(lengths, childLengths...)
	}

	states, pinned := rowCompositionTruthStates(pins, occurrence, "anyOf", len(node.anyOf))
	for index, child := range node.anyOf {
		if pinned && !states[index] {
			continue
		}

		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/anyOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		childLengths, err := rowComposedArrayDefaultLengthsAt(child, childOccurrence, pins, visiting)
		if err != nil {
			return nil, err
		}

		lengths = append(lengths, childLengths...)
	}

	return lengths, nil
}

// rowNestedArrayBounds extracts active local and composed bound alternatives.
func rowNestedArrayBounds(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
) ([][2]int, error) {
	return rowNestedArrayBoundsAt(node, occurrence, pins, make(map[*schemaNode]bool))
}

// rowNestedArrayBoundsAt carries nested composition alternatives through bounds.
//
//nolint:cyclop // Bound alternatives are combined in authored composition order.
func rowNestedArrayBoundsAt(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	visiting map[*schemaNode]bool,
) ([][2]int, error) {
	maximum := int(^uint(0) >> 1)
	if node == nil {
		return [][2]int{{0, maximum}}, nil
	}

	if visiting[node] {
		return nil, fmt.Errorf("schematest: recursive row array bounds at %s", occurrence.usePointer)
	}

	visiting[node] = true
	defer delete(visiting, node)

	minimum := 0

	if count, fits, err := exactCountUint64(node.minItems); err != nil {
		return nil, err
	} else if fits {
		if count > uint64(maximum) {
			return [][2]int{{maximum, maximum}}, nil
		}

		minimum = int(count)
	}

	if count, fits, err := exactCountUint64(node.maxItems); err != nil {
		return nil, err
	} else if fits && count < uint64(maximum) {
		maximum = int(count)
	}

	bounds := [][2]int{{minimum, maximum}}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		childBounds, err := rowNestedArrayBoundsAt(child, childOccurrence, pins, visiting)
		if err != nil {
			return nil, err
		}

		bounds = combineRowArrayBounds(bounds, childBounds)
	}

	if len(node.anyOf) == 0 {
		return bounds, nil
	}

	states, pinned := rowCompositionTruthStates(pins, occurrence, "anyOf", len(node.anyOf))
	if pinned {
		for index, child := range node.anyOf {
			if !states[index] {
				continue
			}

			childOccurrence := rebasePlanOccurrence(
				child,
				occurrence.usePointer+"/anyOf/"+itoa(index),
				occurrence.instanceTemplate,
			)

			childBounds, err := rowNestedArrayBoundsAt(child, childOccurrence, pins, visiting)
			if err != nil {
				return nil, err
			}

			bounds = combineRowArrayBounds(bounds, childBounds)
		}

		return bounds, nil
	}

	alternatives := make([][2]int, 0, len(node.anyOf))
	for index, child := range node.anyOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/anyOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		childBounds, err := rowNestedArrayBoundsAt(child, childOccurrence, pins, visiting)
		if err != nil {
			return nil, err
		}

		alternatives = append(alternatives, combineRowArrayBounds(bounds, childBounds)...)
	}

	return alternatives, nil
}

// combineRowArrayBounds intersects every left/right bound alternative.
func combineRowArrayBounds(left, right [][2]int) [][2]int {
	result := make([][2]int, 0, len(left)*len(right))
	for _, leftBound := range left {
		for _, rightBound := range right {
			minimum := leftBound[0]
			if rightBound[0] > minimum {
				minimum = rightBound[0]
			}

			maximum := leftBound[1]
			if rightBound[1] < maximum {
				maximum = rightBound[1]
			}

			result = append(result, [2]int{minimum, maximum})
		}
	}

	return result
}

// walkObject assigns members in canonical UTF-8 name order.
func (s *search) walkObject(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	context rowSearchContext,
	visit rowVisit,
) (bool, error) {
	members, err := rowObjectMembers(node, occurrence, pins)
	if err != nil {
		return false, err
	}

	values := make(map[string]*jsonValue, len(members))

	return s.walkObjectMembers(node, occurrence, pins, context, members, values, 0, visit)
}

// walkObjectMembers performs deterministic presence and value backtracking.
func (s *search) walkObjectMembers(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
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
	presence, presencePinned := rowPresencePinDetails(pins, member.occurrence)

	choices, err := rowMemberPresenceChoices(
		node, occurrence, pins, members, index, member, presence, presencePinned,
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
				node, occurrence, pins, context, members, values, index+1, visit,
			)
			if err != nil || complete {
				return complete, err
			}

			continue
		}

		walkValue := func(value *jsonValue) (bool, error) {
			values[member.name] = value

			return s.walkObjectMembers(node, occurrence, pins, context, members, values, index+1, visit)
		}

		complete, err := s.walkRowMemberValues(member, pins, context, walkValue)
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
	pins []applicabilityPin,
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
			complete, err := s.walkGenericValue(pins, visit)
			if err != nil || complete {
				return complete, err
			}

			continue
		}

		complete, err := s.walkNode(
			candidate.node, candidate.occurrence, pins, context,
			func(value *jsonValue) (bool, error) {
				usable, err := s.rowChildValueUsable(candidate.node, candidate.occurrence, pins, value)
				if err != nil {
					return false, err
				}

				if !usable {
					usable, err = directedStringChildUsable(
						context.stringObjective, candidate.node, candidate.occurrence, value,
					)
					if err != nil || !usable {
						return false, err
					}
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
	pins []applicabilityPin,
	members []rowMember,
	index int,
	member rowMember,
	pinned applicabilityPin,
	presencePinned bool,
) ([]bool, error) {
	if presencePinned && !pinned.canonical {
		if pinned.presence == planPinPresent {
			return []bool{true}, nil
		}

		return []bool{false}, nil
	}

	if member.required {
		return []bool{true}, nil
	}

	if presencePinned {
		if pinned.presence == planPinPresent {
			return []bool{true, false}, nil
		}

		return []bool{false, true}, nil
	}

	present, err := rowCanonicalMemberPresence(node, occurrence, pins, members, index)
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
	pins []applicabilityPin,
) (uint64, bool, error) {
	return rowNestedObjectMinimumProperties(node, occurrence, pins, make(map[*schemaNode]bool))
}

// rowNestedObjectMinimumProperties carries object lower bounds through active composition.
//
//nolint:cyclop,gocognit // Local, allOf, and pinned anyOf bounds are one recursive calculation.
func rowNestedObjectMinimumProperties(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
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
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		childMinimum, childFits, err := rowNestedObjectMinimumProperties(
			child, childOccurrence, pins, visiting,
		)
		if err != nil {
			return 0, false, err
		}

		if childFits && (!fits || childMinimum > minimum) {
			minimum = childMinimum
			fits = true
		}
	}

	states, pinned := rowCompositionTruthStates(pins, occurrence, "anyOf", len(node.anyOf))
	if pinned {
		for index, child := range node.anyOf {
			if !states[index] {
				continue
			}

			childOccurrence := rebasePlanOccurrence(
				child,
				occurrence.usePointer+"/anyOf/"+itoa(index),
				occurrence.instanceTemplate,
			)

			childMinimum, childFits, err := rowNestedObjectMinimumProperties(
				child, childOccurrence, pins, visiting,
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
	pins []applicabilityPin,
	members []rowMember,
	index int,
) (bool, error) {
	member := members[index]
	if member.required {
		return true, nil
	}

	minimum, fits, err := rowObjectMinimumProperties(node, occurrence, pins)
	if err != nil {
		return false, err
	}

	if !fits {
		return false, nil
	}

	present := 0

	for prior := 0; prior < index; prior++ {
		priorPresent, err := rowCanonicalMemberPresence(node, occurrence, pins, members, prior)
		if err != nil {
			return false, err
		}

		if members[prior].required || priorPresent {
			present++
		}
	}

	return uint64(present) < minimum, nil
}

// rowObjectMembers collects direct, composed, required, and pinned member names.
//
//nolint:cyclop,gocognit // Direct, composed, additional, and pin selection share one canonical pass.
func rowObjectMembers(node *schemaNode, occurrence schemaOccurrence, pins []applicabilityPin) ([]rowMember, error) {
	specs := make(map[string][]rowMember)

	required := make(map[string]bool)
	if err := collectRowObjectMembers(
		node, occurrence, specs, required, pins, true, make(map[*schemaNode]bool),
	); err != nil {
		return nil, err
	}

	for _, pin := range pins {
		if name, ok := rowChildName(occurrence.instanceTemplate, pin.occurrence.instanceTemplate); ok {
			if name == "*" && strings.HasSuffix(pin.occurrence.usePointer, "/additionalProperties") {
				continue
			}

			if pin.presence != planPinNoPresence || pin.hasKind {
				if _, exists := specs[name]; !exists {
					specs[name] = nil
				}
			}
		}
	}

	additionalSourceSets, err := rowAdditionalPropertySources(node, occurrence, pins)
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
						source.occurrence.usePointer,
						appendInstanceToken(occurrence.instanceTemplate, name),
					),
				})
			}

			result = append(result, members)
		}

		return result
	}

	extraNames, err := rowAdditionalMemberNames(node, specs, pins, occurrence)
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
			pins,
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
	pins []applicabilityPin,
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

		member, err := composeRowMember(name, candidates, required, pins)
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
	pins []applicabilityPin,
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

		states, pinned := rowCompositionTruthStates(pins, currentOccurrence, "anyOf", len(current.anyOf))
		if pinned {
			selected := common

			for index, child := range current.anyOf {
				if !states[index] {
					continue
				}

				childOccurrence := rebasePlanOccurrence(
					child,
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

// rowAdditionalMemberNames adds wildcard members needed by lower bounds or target pins.
func rowAdditionalMemberNames(
	node *schemaNode,
	specified map[string][]rowMember,
	pins []applicabilityPin,
	occurrence schemaOccurrence,
) ([]string, error) {
	if !node.allowAdditionalProperties {
		return nil, nil
	}

	minimum, fits, err := rowObjectMinimumProperties(node, occurrence, pins)
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

	if rowAdditionalPresencePinned(occurrence, pins) && needed == 0 {
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
	pins []applicabilityPin,
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
			child, occurrence.usePointer+"/allOf/"+itoa(index), occurrence.instanceTemplate,
		)
		if err := collectRowObjectMembers(
			child, childOccurrence, specs, required, pins, requiredAllowed, visiting,
		); err != nil {
			return err
		}
	}

	for index, child := range node.anyOf {
		childOccurrence := rebasePlanOccurrence(
			child, occurrence.usePointer+"/anyOf/"+itoa(index), occurrence.instanceTemplate,
		)

		branchRequired := requiredAllowed && rowCompositionPinTruth(pins, childOccurrence, "anyOf", index)
		if err := collectRowObjectMembers(
			child, childOccurrence, specs, required, pins, branchRequired, visiting,
		); err != nil {
			return err
		}
	}

	return nil
}

// rowCompositionPinTruth returns one pinned branch truth, defaulting to false.
func rowCompositionPinTruth(
	pins []applicabilityPin,
	occurrence schemaOccurrence,
	composition string,
	branch int,
) bool {
	for _, pin := range pins {
		if pin.hasBranch && pin.composition == composition && pin.branch == branch &&
			rowOccurrenceMatches(pin.occurrence, occurrence) {
			return pin.truth
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

// rowPresencePinDetails prefers a hard target pin over a canonical starting assignment.
func rowPresencePinDetails(pins []applicabilityPin, occurrence schemaOccurrence) (applicabilityPin, bool) {
	var canonical applicabilityPin

	canonicalFound := false

	for _, pin := range pins {
		if pin.presence == planPinNoPresence || !rowOccurrenceMatches(pin.occurrence, occurrence) {
			continue
		}

		if !pin.canonical {
			return pin, true
		}

		if !canonicalFound {
			canonical = pin
			canonicalFound = true
		}
	}

	return canonical, canonicalFound
}

// rowAdditionalPresencePinned reports whether the target explicitly asks for an extra member.
func rowAdditionalPresencePinned(occurrence schemaOccurrence, pins []applicabilityPin) bool {
	wantedTemplate := appendInstanceToken(occurrence.instanceTemplate, "*")

	for _, pin := range pins {
		if pin.canonical || pin.presence != planPinPresent ||
			!strings.HasSuffix(pin.occurrence.usePointer, "/additionalProperties") {
			continue
		}

		if pin.occurrence.instanceTemplate == wantedTemplate {
			return true
		}
	}

	return false
}

// walkGenericValue assigns a small complete value where no child schema is available.
func (s *search) walkGenericValue(_ []applicabilityPin, visit rowVisit) (bool, error) {
	for _, kind := range canonicalJSONKinds() {
		if err := s.assign(); err != nil {
			return false, err
		}

		candidates, err := canonicalKindWitnesses(kind)
		if err != nil {
			return false, err
		}

		for _, candidate := range candidates {
			if err := s.assign(); err != nil {
				return false, err
			}

			complete, visitErr := visit(candidate)
			if visitErr != nil || complete {
				return complete, visitErr
			}
		}
	}

	return false, nil
}
