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

			return s.walkArrayElements(item, occurrence, pins, elements, index+1, visit)
		})
	}

	return s.walkNode(item, occurrence, pins, func(value *jsonValue) (bool, error) {
		usable, err := rowChildValueUsable(item, occurrence, pins, value)
		if err != nil || !usable {
			return false, err
		}

		elements[index] = value

		return s.walkArrayElements(item, occurrence, pins, elements, index+1, visit)
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

	for _, child := range node.allOf {
		childMinimum, childMaximum, err := rowNestedArrayBounds(child)
		if err != nil {
			return nil, err
		}

		if childMinimum > minimum {
			minimum = childMinimum
		}

		if childMaximum < maximum {
			maximum = childMaximum
		}
	}

	anyOfBounds := make([][2]int, 0, len(node.anyOf))

	anyOfStates, anyOfPinned := rowCompositionTruthStates(pins, occurrence, "anyOf", len(node.anyOf))
	for index, child := range node.anyOf {
		childMinimum, childMaximum, err := rowNestedArrayBounds(child)
		if err != nil {
			return nil, err
		}

		if anyOfPinned {
			if !anyOfStates[index] {
				continue
			}

			if childMinimum > minimum {
				minimum = childMinimum
			}

			if childMaximum < maximum {
				maximum = childMaximum
			}

			continue
		}

		anyOfBounds = append(anyOfBounds, [2]int{childMinimum, childMaximum})
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

	for _, bounds := range anyOfBounds {
		appendLength(bounds[0])

		if bounds[1] < int(^uint(0)>>1) {
			appendLength(bounds[1])
		}
	}

	return candidates, nil
}

// rowNestedArrayBounds extracts safe local bounds from a composition branch.
//
//nolint:cyclop // Local and allOf bound intersections are one structural phase.
func rowNestedArrayBounds(node *schemaNode) (int, int, error) {
	minimum := 0

	maximum := int(^uint(0) >> 1)
	if node == nil {
		return minimum, maximum, nil
	}

	if count, fits, err := exactCountUint64(node.minItems); err != nil {
		return 0, 0, err
	} else if fits {
		if count > uint64(maximum) {
			return maximum, maximum, nil
		}

		minimum = int(count)
	}

	if count, fits, err := exactCountUint64(node.maxItems); err != nil {
		return 0, 0, err
	} else if fits && count < uint64(maximum) {
		maximum = int(count)
	}

	for _, child := range node.allOf {
		childMinimum, childMaximum, err := rowNestedArrayBounds(child)
		if err != nil {
			return 0, 0, err
		}

		if childMinimum > minimum {
			minimum = childMinimum
		}

		if childMaximum < maximum {
			maximum = childMaximum
		}
	}

	return minimum, maximum, nil
}

// walkObject assigns members in canonical UTF-8 name order.
func (s *search) walkObject(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	visit rowVisit,
) (bool, error) {
	members, err := rowObjectMembers(node, occurrence, pins)
	if err != nil {
		return false, err
	}

	values := make(map[string]*jsonValue, len(members))

	return s.walkObjectMembers(node, occurrence, pins, members, values, 0, visit)
}

// walkObjectMembers performs deterministic presence and value backtracking.
func (s *search) walkObjectMembers(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
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

			complete, err := s.walkObjectMembers(node, occurrence, pins, members, values, index+1, visit)
			if err != nil || complete {
				return complete, err
			}

			continue
		}

		walkValue := func(value *jsonValue) (bool, error) {
			values[member.name] = value

			return s.walkObjectMembers(node, occurrence, pins, members, values, index+1, visit)
		}

		complete, err := s.walkRowMemberValues(member, pins, walkValue)
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

		complete, err := s.walkNode(candidate.node, candidate.occurrence, pins, func(value *jsonValue) (bool, error) {
			usable, err := rowChildValueUsable(candidate.node, candidate.occurrence, pins, value)
			if err != nil || !usable {
				return false, err
			}

			return visit(value)
		})
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
//
//nolint:cyclop // Local, allOf, and pinned anyOf bounds are one calculation.
func rowObjectMinimumProperties(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
) (uint64, bool, error) {
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

	if node == nil || node.schemaShape == nil {
		return 0, false, nil
	}

	if err := consider(node.minProperties); err != nil {
		return 0, false, err
	}

	for _, child := range node.allOf {
		if !nodeCanHaveKind(child, jsonObject) {
			continue
		}

		if err := consider(child.minProperties); err != nil {
			return 0, false, err
		}
	}

	states, pinned := rowCompositionTruthStates(pins, occurrence, "anyOf", len(node.anyOf))
	if pinned {
		for index, child := range node.anyOf {
			if states[index] && nodeCanHaveKind(child, jsonObject) {
				if err := consider(child.minProperties); err != nil {
					return 0, false, err
				}
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

	extraNames, err := rowAdditionalMemberNames(node, specs, pins, occurrence)
	if err != nil {
		return nil, err
	}

	for _, name := range extraNames {
		member := rowMember{name: name}
		if node.additionalProperties != nil {
			member.node = node.additionalProperties
			member.occurrence = rebasePlanOccurrence(
				node.additionalProperties,
				occurrence.usePointer+"/additionalProperties",
				appendInstanceToken(occurrence.instanceTemplate, name),
			)
		} else {
			member.occurrence = schemaOccurrence{
				usePointer:       occurrence.usePointer + "/additionalProperties",
				targetPointer:    occurrence.targetPointer,
				instanceTemplate: appendInstanceToken(occurrence.instanceTemplate, name),
			}
		}

		specs[name] = append(specs[name], member)
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
		candidates := specs[name]

		var member rowMember

		if len(candidates) > 0 {
			var err error

			member, err = composeRowMember(name, candidates, required[name], pins)
			if err != nil {
				return nil, err
			}
		} else {
			member = rowMember{name: name, required: required[name]}
			if node.additionalProperties != nil {
				member.node = node.additionalProperties
				member.occurrence = rebasePlanOccurrence(
					node.additionalProperties,
					occurrence.usePointer+"/additionalProperties",
					appendInstanceToken(occurrence.instanceTemplate, name),
				)
			}
		}

		members = append(members, member)
	}

	return members, nil
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
	wanted := schemaOccurrence{
		usePointer:       occurrence.usePointer + "/additionalProperties",
		instanceTemplate: appendInstanceToken(occurrence.instanceTemplate, "*"),
	}

	pin, exists := rowPresencePinDetails(pins, wanted)

	return exists && !pin.canonical && pin.presence == planPinPresent
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
