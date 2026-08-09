package schematest

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
)

// makePlan compiles every stable valid and isolated-fault obligation without
// constructing a JSON row or retaining a scalar witness.
//
//nolint:cyclop // Canonical compilation, keying, schedules, and catalog assembly are one boundary.
func makePlan(model *schemaModel) (*searchPlan, error) {
	if model == nil || model.root == nil || model.root.schemaShape == nil {
		return nil, errors.New("schema model has no root")
	}

	builder := planBuilder{}

	compiled, err := builder.compileNode(
		model.root,
		model.root.occurrence,
		nil,
		nil,
		make(map[*schemaNode]bool),
	)
	if err != nil {
		return nil, err
	}

	if validationErr := validatePlanOccurrences(compiled); validationErr != nil {
		return nil, validationErr
	}

	if cacheErr := cachePlanOrderKeys(compiled.valid, compiled.faults); cacheErr != nil {
		return nil, cacheErr
	}

	sort.SliceStable(compiled.valid, func(left, right int) bool {
		return comparePlanOrderKeys(
			compiled.valid[left].obligation.orderKey,
			compiled.valid[right].obligation.orderKey,
		) < 0
	})
	sort.SliceStable(compiled.faults, func(left, right int) bool {
		return comparePlanOrderKeys(
			compiled.faults[left].obligation.orderKey,
			compiled.faults[right].obligation.orderKey,
		) < 0
	})

	stringObjectives := compileStringObjectiveOrder(compiled.valid)

	validSchedule, err := compileValidSchedule(compiled.valid, stringObjectives)
	if err != nil {
		return nil, err
	}

	validSchedule, err = appendFormatBoundaryRequests(validSchedule, compiled.valid, stringObjectives)
	if err != nil {
		return nil, err
	}

	faultExecution := compileFaultExecution(compiled.faults)

	obligations := make([]obligation, 0, len(compiled.valid)+len(compiled.faults))
	for _, target := range compiled.valid {
		obligations = append(obligations, target.obligation)
	}

	for _, target := range compiled.faults {
		obligations = append(obligations, target.obligation)
	}

	sort.SliceStable(obligations, func(left, right int) bool {
		return comparePlanOrderKeys(obligations[left].orderKey, obligations[right].orderKey) < 0
	})

	if err := rejectDuplicateObligations(obligations); err != nil {
		return nil, err
	}

	return &searchPlan{
		validCatalog:     compiled.valid,
		validSchedule:    validSchedule,
		stringObjectives: stringObjectives,
		faultSchedule:    compiled.faults,
		faultExecution:   faultExecution,
		obligations:      obligations,
	}, nil
}

// compileStringObjectiveOrder keeps scalar execution independent of report order.
func compileStringObjectiveOrder(catalog []validIntent) []levelIdentity {
	objectives := make([]levelIdentity, 0)

	for _, rule := range []string{
		oracleRulePattern, oracleRuleFormat, oracleRuleMinLength, oracleRuleMaxLength,
	} {
		for _, target := range catalog {
			if target.expected.rule != rule || rule == oracleRuleFormat && !validTargetRequiresString(target) {
				continue
			}

			objectives = append(objectives, target.expected)
		}
	}

	return objectives
}

// validTargetRequiresString reports whether a format level applies to strings.
func validTargetRequiresString(target validIntent) bool {
	for _, requirement := range target.requirements {
		if requirement.hasKind && requirement.kind == jsonString {
			return true
		}
	}

	return false
}

// appendFormatBoundaryRequests routes every registry boundary through Build.
func appendFormatBoundaryRequests(
	schedule []validRequest,
	catalog []validIntent,
	objectives []levelIdentity,
) ([]validRequest, error) {
	if len(schedule) == 0 {
		return schedule, nil
	}

	baseline := schedule[0]

	for _, target := range catalog {
		if target.expected.rule != oracleRuleFormat || !validTargetRequiresString(target) {
			continue
		}

		specification, exists := stringFormatSpecificationFor(target.stringFormat)
		if !exists {
			return nil, fmt.Errorf("format boundary target %s has no registry entry", target.obligation.String())
		}

		if specification.inert {
			continue
		}

		for index := 0; index < int(specification.objectiveCount); index++ {
			boundary := &formatBoundaryObjective{
				identity: target.expected,
				format:   target.stringFormat,
				boundary: specification.objectives[index],
			}
			request := baseline
			request.stringObjectives = validRequestStringObjectives(
				baseline.targets, catalogTargetIndex(baseline.targets, target.expected), objectives,
			)
			request.formatBoundary = boundary
			schedule = append(schedule, request)
		}
	}

	return schedule, nil
}

// catalogTargetIndex finds one objective in the selected baseline vector.
func catalogTargetIndex(targets []validIntent, identity levelIdentity) int {
	for index := range targets {
		if targets[index].expected == identity {
			return index
		}
	}

	return -1
}

// compileFaultExecution gives string faults their locked pattern, format, then length order.
func compileFaultExecution(faults []faultProgram) []int {
	execution := make([]int, len(faults))
	for index := range faults {
		execution[index] = index
	}

	sort.SliceStable(execution, func(left, right int) bool {
		return stringFaultExecutionRank(faults[execution[left]].obligation.rule) <
			stringFaultExecutionRank(faults[execution[right]].obligation.rule)
	})

	return execution
}

// stringFaultExecutionRank orders string faults after other canonical faults.
func stringFaultExecutionRank(rule string) uint8 {
	const (
		nonStringRank uint8 = iota
		patternRank
		formatRank
		minLengthRank
		maxLengthRank
	)

	switch rule {
	case oracleRulePattern:
		return patternRank
	case oracleRuleFormat:
		return formatRank
	case oracleRuleMinLength:
		return minLengthRank
	case oracleRuleMaxLength:
		return maxLengthRank
	default:
		return nonStringRank
	}
}

// cachePlanOrderKeys parses ordering metadata once before any sort comparison.
func cachePlanOrderKeys(valid []validIntent, faults []faultProgram) error {
	for index := range valid {
		key, err := makePlanOrderKey(valid[index].obligation)
		if err != nil {
			return fmt.Errorf("cache valid obligation order: %w", err)
		}

		valid[index].obligation.orderKey = key
	}

	for index := range faults {
		key, err := makePlanOrderKey(faults[index].obligation)
		if err != nil {
			return fmt.Errorf("cache fault obligation order: %w", err)
		}

		faults[index].obligation.orderKey = key
	}

	return nil
}

// compileValidSchedule emits one baseline and one request per noncanonical level.
func compileValidSchedule(catalog []validIntent, stringObjectives []levelIdentity) ([]validRequest, error) {
	if len(catalog) == 0 {
		return nil, nil
	}

	groups := make([][]validIntent, 0)
	for _, target := range catalog {
		if len(groups) == 0 || !samePlanRule(groups[len(groups)-1][0], target) {
			groups = append(groups, []validIntent{target})

			continue
		}

		groups[len(groups)-1] = append(groups[len(groups)-1], target)
	}

	baselineTargets := make([]validIntent, len(groups))
	for index, group := range groups {
		baselineTargets[index] = group[0]
	}

	baseline := makeValidRequest(baselineTargets, -1, stringObjectives)
	schedule := []validRequest{baseline}

	for groupIndex, group := range groups {
		for _, alternative := range group[1:] {
			targets := append([]validIntent(nil), baselineTargets...)
			targets[groupIndex] = alternative

			request := makeFocusedValidRequest(baseline, targets, groupIndex, stringObjectives)
			schedule = append(schedule, request)
		}
	}

	return schedule, nil
}

// requirementDimensionsConflict detects only contradictory selected constraint dimensions.
//
//nolint:cyclop // Each declarative requirement dimension has one direct conflict rule.
func requirementDimensionsConflict(left, right []requirement) bool {
	for _, existing := range left {
		for _, candidate := range right {
			sameInstance := instanceTemplateMatches(
				existing.occurrence.instanceTemplate, candidate.occurrence.instanceTemplate,
			) || instanceTemplateMatches(
				candidate.occurrence.instanceTemplate, existing.occurrence.instanceTemplate,
			)
			switch {
			case existing.hasBranch && candidate.hasBranch &&
				samePlanRequirementOccurrence(existing, candidate) && existing.truth != candidate.truth:
				return true
			case existing.hasKind && candidate.hasKind && sameInstance && existing.kind != candidate.kind:
				return true
			case existing.presence != requirementNoPresence && candidate.presence != requirementNoPresence &&
				sameInstance && existing.presence != candidate.presence && !existing.canonical && !candidate.canonical:
				return true
			case existing.enumMember != nil && candidate.enumMember != nil && sameInstance &&
				existing.enumMember != candidate.enumMember:
				return true
			case existing.count != nil && candidate.count != nil && sameInstance && existing.count != candidate.count:
				return true
			}
		}
	}

	return false
}

// samePlanRule identifies one radix in the valid catalog.
func samePlanRule(left, right validIntent) bool {
	return left.expected.rule == right.expected.rule &&
		left.expected.occurrence.usePointer == right.expected.occurrence.usePointer &&
		left.expected.occurrence.targetPointer == right.expected.occurrence.targetPointer &&
		left.expected.occurrence.instanceTemplate == right.expected.occurrence.instanceTemplate &&
		left.expected.occurrence.reference == right.expected.occurrence.reference
}

// makeValidRequest synthesizes one complete selected vector from declarative dimensions.
func makeValidRequest(targets []validIntent, focus int, objectives []levelIdentity) validRequest {
	if focus >= 0 && focus < len(targets) {
		baseline := makeValidRequest(targets, -1, objectives)

		return makeFocusedValidRequest(baseline, targets, focus, objectives)
	}

	request := validRequest{
		targets:          append([]validIntent(nil), targets...),
		components:       make([][]requirement, len(targets)),
		stringObjectives: validRequestStringObjectives(targets, focus, objectives),
		focus:            focus,
	}

	for _, index := range validRequestTargetOrder(targets, focus) {
		component := copyPlanRequirements(targets[index].requirements)
		if requirementDimensionsConflict(request.requirements, component) {
			component = validTargetActiveRequirements(targets[index])
		}

		request.components[index] = component
		request.requirements = appendPlanRequirements(request.requirements, component...)
	}

	return request
}

// makeFocusedValidRequest copies the baseline and replaces one radix's selected component.
func makeFocusedValidRequest(
	baseline validRequest,
	targets []validIntent,
	focus int,
	objectives []levelIdentity,
) validRequest {
	components := make([][]requirement, len(baseline.components))
	for index := range baseline.components {
		components[index] = copyPlanRequirements(baseline.components[index])
	}

	oldFocus := components[focus]

	newFocus := copyPlanRequirements(targets[focus].requirements)
	for index := range components {
		if index == focus || !requirementDimensionsConflict(oldFocus, targets[index].requirements) ||
			requirementDimensionsConflict(newFocus, targets[index].requirements) {
			continue
		}

		components[index] = copyPlanRequirements(targets[index].requirements)
	}

	components[focus] = newFocus

	request := validRequest{
		targets:          append([]validIntent(nil), targets...),
		components:       make([][]requirement, len(targets)),
		stringObjectives: validRequestStringObjectives(targets, focus, objectives),
		focus:            focus,
	}
	for _, index := range validRequestTargetOrder(targets, focus) {
		component := components[index]
		if requirementDimensionsConflict(request.requirements, component) {
			component = validTargetActiveRequirements(targets[index])
		}

		request.components[index] = component
		request.requirements = appendPlanRequirements(request.requirements, component...)
	}

	return request
}

// validRequestTargetOrder gives the focus priority, then its matching branch dimensions.
//
//nolint:cyclop // Focus, matching branch components, and composition components have fixed tiers.
func validRequestTargetOrder(targets []validIntent, focus int) []int {
	ordered := make([]int, 0, len(targets))
	if focus < 0 || focus >= len(targets) {
		for index := range targets {
			ordered = append(ordered, index)
		}

		return ordered
	}

	if focus >= 0 && focus < len(targets) {
		ordered = append(ordered, focus)
		for index, target := range targets {
			if index != focus && target.expected.rule != oracleRuleAnyOf &&
				target.expected.rule != oracleRuleAllOf &&
				validTargetSharesSelectedBranch(target, targets[focus]) {
				ordered = append(ordered, index)
			}
		}
	}

	for index, target := range targets {
		if index == focus || target.expected.rule == oracleRuleAnyOf || target.expected.rule == oracleRuleAllOf ||
			containsInt(ordered, index) {
			continue
		}

		ordered = append(ordered, index)
	}

	for index, target := range targets {
		if index != focus && (target.expected.rule == oracleRuleAnyOf || target.expected.rule == oracleRuleAllOf) {
			ordered = append(ordered, index)
		}
	}

	return ordered
}

// validTargetSharesSelectedBranch reports a direct matching branch-truth dimension.
func validTargetSharesSelectedBranch(target, selected validIntent) bool {
	for _, candidate := range target.requirements {
		if !candidate.hasBranch {
			continue
		}

		for _, branch := range selected.requirements {
			if branch.hasBranch && samePlanRequirementOccurrence(candidate, branch) && candidate.truth == branch.truth {
				return true
			}
		}
	}

	return false
}

// validTargetActiveRequirements preserves neutral context when direct dimensions conflict.
func validTargetActiveRequirements(target validIntent) []requirement {
	result := make([]requirement, 0)

	for _, candidate := range target.requirements {
		if candidate.tag == requirementActiveRules {
			result = append(result, candidate)
		}
	}

	return result
}

// containsInt reports whether a small request-order prefix already contains an index.
func containsInt(values []int, wanted int) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}

	return false
}

// validRequestStringObjectives puts an applicable focused objective before the independent sequence.
func validRequestStringObjectives(
	targets []validIntent,
	focus int,
	objectives []levelIdentity,
) []levelIdentity {
	ordered := make([]levelIdentity, 0, len(objectives)+1)
	if focus >= 0 && focus < len(targets) && isStringObjective(targets[focus].expected) {
		ordered = append(ordered, targets[focus].expected)
	}

	for _, objective := range objectives {
		if len(ordered) > 0 && ordered[0] == objective {
			continue
		}

		ordered = append(ordered, objective)
	}

	return ordered
}

// isStringObjective reports whether one level belongs to string-product execution.
func isStringObjective(identity levelIdentity) bool {
	switch identity.rule {
	case oracleRuleMinLength, oracleRuleMaxLength, oracleRulePattern:
		return true
	case oracleRuleFormat:
		return identity.level == oracleScalarValidLevel
	default:
		return false
	}
}

// planBuilder owns deterministic insertion order while compiling one model.
type planBuilder struct {
	nextOrder uint64
}

// compiledNodePlan contains targets collected below one schema occurrence.
type compiledNodePlan struct {
	valid  []validIntent
	faults []faultProgram
}

// anyOfBranchPlan contains one branch's declarative fault alternatives.
type anyOfBranchPlan struct {
	faults []faultProgram
}

// compileNode compiles one occurrence with separate valid and fault context.
//
//nolint:cyclop // The compilation phases must remain in canonical order.
func (builder *planBuilder) compileNode(
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []requirement,
	faultInherited []requirement,
	visiting map[*schemaNode]bool,
) (compiledNodePlan, error) {
	if node == nil || node.schemaShape == nil {
		return compiledNodePlan{}, errors.New("schema occurrence has no shape")
	}

	if visiting[node] {
		return compiledNodePlan{}, fmt.Errorf("recursive schema occurrence at %s", occurrence.usePointer)
	}

	visiting[node] = true
	defer delete(visiting, node)

	activeRules := requirement{
		tag:        requirementActiveRules,
		occurrence: occurrence,
		active:     node,
	}
	validRequirements := appendPlanRequirements(defaultPlanRequirements(validInherited, node, occurrence), activeRules)
	faultRequirements := appendPlanRequirements(defaultPlanRequirements(faultInherited, node, occurrence), activeRules)
	result := compiledNodePlan{}

	if err := builder.compileTypeRules(&result, node, occurrence, validRequirements, faultRequirements); err != nil {
		return compiledNodePlan{}, err
	}

	if err := builder.compileEnumRules(&result, node, occurrence, validRequirements, faultRequirements); err != nil {
		return compiledNodePlan{}, err
	}

	if err := builder.compileNumberRules(&result, node, occurrence, validRequirements, faultRequirements); err != nil {
		return compiledNodePlan{}, err
	}

	if err := builder.compileStringRules(&result, node, occurrence, validRequirements, faultRequirements); err != nil {
		return compiledNodePlan{}, err
	}

	if err := builder.compileArrayRules(&result, node, occurrence, validRequirements, faultRequirements); err != nil {
		return compiledNodePlan{}, err
	}

	if err := builder.compileObjectRules(&result, node, occurrence, validRequirements, faultRequirements); err != nil {
		return compiledNodePlan{}, err
	}

	allOfIdentity := makeRuleIdentity(occurrence, oracleRuleAllOf)
	if len(node.allOf) > 0 {
		builder.addValid(
			&result,
			allOfIdentity,
			planLevelAllTrue,
			appendPlanRequirements(validRequirements, allOfValidRequirements(occurrence, len(node.allOf))...),
		)
	}

	anyOfIdentity := makeRuleIdentity(occurrence, oracleRuleAnyOf)

	if len(node.anyOf) > 0 {
		one := big.NewInt(1)

		limit := new(big.Int).Lsh(one, uint(len(node.anyOf)))
		for mask := big.NewInt(1); mask.Cmp(limit) < 0; mask.Add(mask, one) {
			builder.addValid(
				&result,
				anyOfIdentity,
				planLevelMask+mask.String(),
				appendPlanRequirements(validRequirements, anyOfMaskRequirements(occurrence, len(node.anyOf), mask)...),
			)
		}
	}

	if err := builder.compileChildren(
		&result,
		node,
		occurrence,
		validRequirements,
		faultRequirements,
		visiting,
	); err != nil {
		return compiledNodePlan{}, err
	}

	if len(node.anyOf) > 0 {
		if err := builder.compileAnyOfChildren(
			&result,
			node,
			occurrence,
			validRequirements,
			faultRequirements,
			visiting,
			anyOfIdentity,
		); err != nil {
			return compiledNodePlan{}, err
		}
	}

	return result, nil
}

// compileAnyOfChildren compiles branch targets and symbolic branch-local
// closure domains. It never selects or multiplies representatives.
func (builder *planBuilder) compileAnyOfChildren(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []requirement,
	faultInherited []requirement,
	visiting map[*schemaNode]bool,
	anyOfIdentity ruleIdentity,
) error {
	inheritedFaultCount := len(result.faults)
	branches := make([]anyOfBranchPlan, len(node.anyOf))

	for index, child := range node.anyOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/anyOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		childPlan, err := builder.compileNode(
			child,
			childOccurrence,
			appendPlanRequirements(validInherited, anyOfValidRequirements(occurrence, index)...),
			faultInherited,
			visiting,
		)
		if err != nil {
			return err
		}

		result.valid = append(result.valid, childPlan.valid...)
		branches[index] = anyOfBranchPlan{faults: childPlan.faults}
	}

	for index := range result.faults[:inheritedFaultCount] {
		result.faults[index].alternatives = appendClosurePrograms(
			result.faults[index].alternatives,
			optionalAnyOfClosure(branches, anyOfIdentity),
		)
	}

	parentFaultRequirements := anyOfFaultRequirements(occurrence, len(node.anyOf))

	for index, branch := range branches {
		for _, candidate := range branch.faults {
			closure := appendFailureIdentity(candidate.expected, failureIdentity(anyOfIdentity))
			alternatives := appendClosurePrograms(
				candidate.alternatives,
				closureDomainsExcept(branches, index),
			)
			builder.addCompiledFault(
				result,
				candidate.obligation.ruleIdentity,
				appendPlanRequirements(candidate.requirements, parentFaultRequirements...),
				closure,
				alternatives,
				obligationRuleRank(candidate.obligation),
			)
		}
	}

	builder.addCompiledFault(
		result,
		anyOfIdentity,
		appendPlanRequirements(faultInherited, parentFaultRequirements...),
		failureSet{failureIdentity(anyOfIdentity)},
		closureDomainsExcept(branches, -1),
		planRuleRank(anyOfIdentity.rule),
	)

	return nil
}

// optionalAnyOfClosure allows an enclosing direct fault either to preserve the
// composition or to expose one complete aggregate closure.
func optionalAnyOfClosure(branches []anyOfBranchPlan, identity ruleIdentity) *faultClosureProgram {
	preserved := &faultClosureAlternative{}
	closed := &faultClosureAlternative{
		requirements: anyOfFaultRequirements(identity.occurrence, len(branches)),
		expected:     failureSet{failureIdentity(identity)},
		closure:      closureDomainsExcept(branches, -1),
	}
	preserved.next = closed

	return &faultClosureProgram{alternatives: preserved}
}

// closureDomainsExcept links one alternative domain per selected branch.
func closureDomainsExcept(branches []anyOfBranchPlan, excluded int) *faultClosureProgram {
	var (
		first *faultClosureProgram
		last  *faultClosureProgram
	)

	for index, branch := range branches {
		if index == excluded {
			continue
		}

		domain := &faultClosureProgram{alternatives: closureAlternatives(branch.faults)}
		if first == nil {
			first = domain
		} else {
			last.next = domain
		}

		last = domain
	}

	return first
}

// closureAlternatives links branch-local fault descriptors in canonical order.
func closureAlternatives(faults []faultProgram) *faultClosureAlternative {
	var (
		first *faultClosureAlternative
		last  *faultClosureAlternative
	)

	for _, fault := range faults {
		alternative := &faultClosureAlternative{
			requirements: fault.requirements,
			expected:     fault.expected,
			closure:      fault.alternatives,
		}
		if first == nil {
			first = alternative
		} else {
			last.next = alternative
		}

		last = alternative
	}

	return first
}

// appendClosurePrograms joins immutable closure programs by copying only the
// left domain spine. Alternatives and nested domains remain shared.
func appendClosurePrograms(left, right *faultClosureProgram) *faultClosureProgram {
	if left == nil {
		return right
	}

	result := &faultClosureProgram{alternatives: left.alternatives}

	last := result
	for domain := left.next; domain != nil; domain = domain.next {
		last.next = &faultClosureProgram{alternatives: domain.alternatives}
		last = last.next
	}

	last.next = right

	return result
}

// appendFailureIdentity returns an independently owned identity set.
func appendFailureIdentity(expected failureSet, identity failureIdentity) failureSet {
	result := append(failureSet(nil), expected...)

	return append(result, identity)
}

// compileChildren compiles items, properties, additional schemas, and allOf branches.
func (builder *planBuilder) compileChildren(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []requirement,
	faultInherited []requirement,
	visiting map[*schemaNode]bool,
) error {
	shape := node.schemaShape
	if shape.items != nil {
		itemOccurrence := rebasePlanOccurrence(
			shape.items,
			occurrence,
			occurrence.usePointer+"/items",
			appendInstanceToken(occurrence.instanceTemplate, "*"),
		)
		if err := builder.compileDirectChild(
			result, node, occurrence, shape.items, itemOccurrence, jsonArray,
			validInherited, faultInherited, visiting,
		); err != nil {
			return err
		}
	}

	for _, name := range sortedSchemaPropertyNames(shape.properties) {
		property := shape.properties[name]

		propertyOccurrence := rebasePlanOccurrence(
			property,
			occurrence,
			occurrence.usePointer+"/properties/"+escapePointerToken(name),
			appendInstanceToken(occurrence.instanceTemplate, name),
		)
		if err := builder.compileDirectChild(
			result, node, occurrence, property, propertyOccurrence, jsonObject,
			validInherited, faultInherited, visiting,
		); err != nil {
			return err
		}
	}

	if shape.additionalProperties != nil {
		additionalOccurrence := rebasePlanOccurrence(
			shape.additionalProperties,
			occurrence,
			occurrence.usePointer+"/additionalProperties",
			appendInstanceToken(occurrence.instanceTemplate, "*"),
		)
		if err := builder.compileDirectChild(
			result, node, occurrence, shape.additionalProperties, additionalOccurrence, jsonObject,
			validInherited, faultInherited, visiting,
		); err != nil {
			return err
		}
	}

	for index, child := range shape.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if err := builder.compileAllOfChild(
			result, node, occurrence, child, childOccurrence, index,
			validInherited, faultInherited, visiting,
		); err != nil {
			return err
		}
	}

	return nil
}

// compileDirectChild compiles one item or object-member child with container context.
func (builder *planBuilder) compileDirectChild(
	result *compiledNodePlan,
	parent *schemaNode,
	parentOccurrence schemaOccurrence,
	child *schemaNode,
	childOccurrence schemaOccurrence,
	kind jsonKind,
	validInherited, faultInherited []requirement,
	visiting map[*schemaNode]bool,
) error {
	validParent, err := builder.validRequirementsForKind(validInherited, parent, parentOccurrence, kind)
	if err != nil {
		return err
	}

	faultParent := builder.faultRequirementsForKind(faultInherited, parentOccurrence, kind)

	faultDefaults, err := defaultPresenceRequirementsForKind(parent, parentOccurrence, kind)
	if err != nil {
		return err
	}

	presence := presenceRequirement(childOccurrence, requirementPresent)

	childPlan, err := builder.compileNode(
		child,
		childOccurrence,
		appendPlanRequirements(validParent, presence),
		appendPlanRequirements(appendPlanRequirements(faultParent, faultDefaults...), presence),
		visiting,
	)
	if err != nil {
		return err
	}

	result.valid = append(result.valid, childPlan.valid...)
	result.faults = append(result.faults, childPlan.faults...)

	return nil
}

// compileAllOfChild compiles one allOf branch with its parent anyOf context.
func (builder *planBuilder) compileAllOfChild(
	result *compiledNodePlan,
	parent *schemaNode,
	parentOccurrence schemaOccurrence,
	child *schemaNode,
	childOccurrence schemaOccurrence,
	index int,
	validInherited, faultInherited []requirement,
	visiting map[*schemaNode]bool,
) error {
	validParent := builder.validAnyOfRequirements(validInherited)
	faultParent := builder.faultRequirementsForAny(faultInherited)

	childPlan, err := builder.compileNode(
		child,
		childOccurrence,
		appendPlanRequirements(validParent, allOfValidRequirements(parentOccurrence, len(parent.allOf))...),
		appendPlanRequirements(faultParent, allOfFaultRequirements(parentOccurrence, len(parent.allOf), index)...),
		visiting,
	)
	if err != nil {
		return err
	}

	result.valid = append(result.valid, childPlan.valid...)
	result.faults = append(result.faults, childPlan.faults...)

	return nil
}

// compileTypeRules compiles kind levels and an explicit-type fault.
func (builder *planBuilder) compileTypeRules(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []requirement,
	faultInherited []requirement,
) error {
	identity := makeRuleIdentity(occurrence, oracleRuleType)
	for _, kind := range orderedTypeKinds(node) {
		requirements, err := builder.validRequirementsForKind(validInherited, node, occurrence, kind)
		if err != nil {
			return err
		}

		builder.addValid(result, identity, jsonKindName(kind), requirements)
	}

	if node.kind != schemaAny {
		builder.addFault(result, identity, faultInherited, failureSet{failureIdentity(identity)})
	}

	return nil
}

// compileEnumRules compiles semantic enum members and the enum fault.
func (builder *planBuilder) compileEnumRules(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []requirement,
	faultInherited []requirement,
) error {
	if node.enum == nil {
		return nil
	}

	identity := makeRuleIdentity(occurrence, oracleRuleEnum)

	for index := range node.enum {
		member := node.enum[index]

		requirements, err := builder.validRequirementsForKind(validInherited, node, occurrence, member.value.kind)
		if err != nil {
			return err
		}

		memberRequirement := requirement{
			tag:        requirementExactEnumMember,
			occurrence: occurrence,
			enumMember: &node.enum[index],
		}
		builder.addValid(
			result,
			identity,
			"member:"+itoa(member.authoredIndex),
			appendPlanRequirements(requirements, memberRequirement),
		)
	}

	requirements := builder.faultRequirementsForEnum(faultInherited)
	builder.addFault(result, identity, requirements, []failureIdentity{identity})

	return nil
}

// compileNumberRules compiles applicable numeric rules in canonical order.
//
//nolint:cyclop // Numeric rules must remain in their authored canonical sequence.
func (builder *planBuilder) compileNumberRules(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []requirement,
	faultInherited []requirement,
) error {
	if !nodeCanHaveKind(node, jsonNumber) {
		return nil
	}

	if node.minimum != nil {
		rule := oracleRuleMinimum
		if node.exclusiveMinimum {
			rule = oracleRuleExclusiveMinimum
		}

		if err := builder.addScalarRule(
			result, node, occurrence, rule, validInherited, faultInherited, jsonNumber, true,
		); err != nil {
			return err
		}
	}

	if node.maximum != nil {
		rule := oracleRuleMaximum
		if node.exclusiveMaximum {
			rule = oracleRuleExclusiveMaximum
		}

		if err := builder.addScalarRule(
			result, node, occurrence, rule, validInherited, faultInherited, jsonNumber, true,
		); err != nil {
			return err
		}
	}

	if node.multipleOf != nil {
		if err := builder.addScalarRule(
			result, node, occurrence, oracleRuleMultipleOf, validInherited, faultInherited, jsonNumber, true,
		); err != nil {
			return err
		}
	}

	if isNumericSchemaFormat(node.format) {
		if err := builder.addScalarRule(
			result, node, occurrence, oracleRuleFormat, validInherited, faultInherited, jsonNumber, true,
		); err != nil {
			return err
		}
	}

	return nil
}

// compileStringRules compiles applicable string rules in canonical order.
//
//nolint:cyclop // String rules must remain in their authored canonical sequence.
func (builder *planBuilder) compileStringRules(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []requirement,
	faultInherited []requirement,
) error {
	if !nodeCanHaveKind(node, jsonString) {
		return nil
	}

	if node.minLength != nil {
		positive, err := exactCountIsPositive(node.minLength)
		if err != nil {
			return err
		}

		if err := builder.addScalarRule(
			result, node, occurrence, oracleRuleMinLength, validInherited, faultInherited, jsonString, positive,
		); err != nil {
			return err
		}
	}

	if node.maxLength != nil {
		if err := builder.addScalarRule(
			result, node, occurrence, oracleRuleMaxLength, validInherited, faultInherited, jsonString, true,
		); err != nil {
			return err
		}
	}

	if node.pattern != nil {
		if err := builder.addScalarRule(
			result, node, occurrence, oracleRulePattern, validInherited, faultInherited, jsonString, true,
		); err != nil {
			return err
		}
	}

	if isStringSchemaFormat(node.format) {
		if err := builder.addScalarRule(
			result, node, occurrence, oracleRuleFormat, validInherited, faultInherited, jsonString, true,
		); err != nil {
			return err
		}

		result.valid[len(result.valid)-1].stringFormat = node.format
	}

	return nil
}

// compileArrayRules compiles applicable array-count rules in canonical order.
func (builder *planBuilder) compileArrayRules(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []requirement,
	faultInherited []requirement,
) error {
	if !nodeCanHaveKind(node, jsonArray) {
		return nil
	}

	if node.minItems != nil {
		positive, err := exactCountIsPositive(node.minItems)
		if err != nil {
			return err
		}

		if err := builder.addScalarRule(
			result, node, occurrence, oracleRuleMinItems, validInherited, faultInherited, jsonArray, positive,
		); err != nil {
			return err
		}
	}

	if node.maxItems != nil {
		if err := builder.addScalarRule(
			result, node, occurrence, oracleRuleMaxItems, validInherited, faultInherited, jsonArray, true,
		); err != nil {
			return err
		}
	}

	return nil
}

// compileObjectRules compiles object counts, required properties, and extras.
//
//nolint:cyclop // Object rules must remain in their authored canonical sequence.
func (builder *planBuilder) compileObjectRules(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []requirement,
	faultInherited []requirement,
) error {
	if !nodeCanHaveKind(node, jsonObject) {
		return nil
	}

	if node.minProperties != nil {
		positive, err := exactCountIsPositive(node.minProperties)
		if err != nil {
			return err
		}

		if err := builder.addScalarRule(
			result, node, occurrence, oracleRuleMinProperties, validInherited, faultInherited, jsonObject, positive,
		); err != nil {
			return err
		}
	}

	if node.maxProperties != nil {
		if err := builder.addScalarRule(
			result, node, occurrence, oracleRuleMaxProperties, validInherited, faultInherited, jsonObject, true,
		); err != nil {
			return err
		}
	}

	for _, name := range node.required {
		identity := makeRuleIdentity(
			appendObjectMemberOccurrence(occurrence, name),
			oracleRuleRequired,
		)
		presenceOccurrence := requiredPresenceOccurrence(node, occurrence, name)

		validRequirements, err := builder.validRequirementsForKind(validInherited, node, occurrence, jsonObject)
		if err != nil {
			return err
		}

		builder.addValid(
			result,
			identity,
			oracleRequiredPresentLevel,
			appendPlanRequirements(validRequirements, presenceRequirement(presenceOccurrence, requirementPresent)),
		)

		faultRequirements := builder.faultRequirementsForRequired(faultInherited, node, occurrence, name)
		builder.addFault(
			result,
			identity,
			appendPlanRequirements(faultRequirements, presenceRequirement(presenceOccurrence, requirementAbsent)),
			[]failureIdentity{identity},
		)
	}

	if node.additionalProperties == nil && !node.allowAdditionalProperties {
		identity := makeRuleIdentity(
			appendObjectMemberOccurrence(occurrence, "*"),
			oracleRuleAdditionalProperties,
		)

		requirements := builder.faultRequirementsForAdditional(faultInherited, occurrence)
		builder.addFault(
			result,
			identity,
			appendPlanRequirements(requirements, presenceRequirement(identity.occurrence, requirementPresent)),
			[]failureIdentity{identity},
		)
	}

	return nil
}

// addScalarRule adds one valid scalar target and, when possible, one exact scalar fault.
func (builder *planBuilder) addScalarRule(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	rule string,
	validInherited []requirement,
	faultInherited []requirement,
	kind jsonKind,
	faultAllowed bool,
) error {
	identity := makeRuleIdentity(occurrence, rule)
	ruleRank := planRuleRankForKind(rule, kind)

	validRequirements, err := builder.validRequirementsForKind(validInherited, node, occurrence, kind)
	if err != nil {
		return err
	}

	if count := countRequirementForRule(node, occurrence, rule); count != nil {
		validRequirements = appendPlanRequirements(validRequirements, *count)
	}

	builder.addValidAtRank(result, identity, oracleScalarValidLevel, validRequirements, ruleRank)

	if !faultAllowed {
		return nil
	}

	faultRequirements := builder.faultRequirementsForRule(faultInherited, occurrence, kind)

	if count := countRequirementForRule(node, occurrence, rule); count != nil {
		faultRequirements = appendPlanRequirements(faultRequirements, *count)
	}

	builder.addFaultAtRank(result, identity, faultRequirements, []failureIdentity{identity}, ruleRank)

	return nil
}

// countRequirementForRule returns one authored count constraint when applicable.
func countRequirementForRule(
	node *schemaNode,
	occurrence schemaOccurrence,
	rule string,
) *requirement {
	var count *exactCount

	switch rule {
	case oracleRuleMinLength:
		count = node.minLength
	case oracleRuleMaxLength:
		count = node.maxLength
	case oracleRuleMinItems:
		count = node.minItems
	case oracleRuleMaxItems:
		count = node.maxItems
	case oracleRuleMinProperties:
		count = node.minProperties
	case oracleRuleMaxProperties:
		count = node.maxProperties
	}

	if count == nil {
		return nil
	}

	return &requirement{
		tag:        requirementExactCount,
		occurrence: occurrence,
		count:      count,
	}
}

// addValid appends one valid target with a deterministic insertion number.
func (builder *planBuilder) addValid(
	result *compiledNodePlan,
	identity ruleIdentity,
	level string,
	requirements []requirement,
) {
	builder.addValidAtRank(result, identity, level, requirements, planRuleRank(identity.rule))
}

// addValidAtRank appends one valid target with an applicability-family rank.
func (builder *planBuilder) addValidAtRank(
	result *compiledNodePlan,
	identity ruleIdentity,
	level string,
	requirements []requirement,
	ruleRank int,
) {
	validObligation := makeLevelObligation(identity, level)
	validObligation.ruleRank = encodedPlanRuleRank(ruleRank)
	validObligation.order = builder.nextOrder
	builder.nextOrder++

	expected := makeLevelIdentity(identity, level)
	result.valid = append(result.valid, validIntent{
		obligation: validObligation,
		expected:   expected,
		requirements: appendPlanRequirements(requirements, requirement{
			tag:        requirementTargetLevel,
			occurrence: identity.occurrence,
			target:     expected,
		}),
	})
}

// addFault appends one fault target with an immutable directed identity set.
func (builder *planBuilder) addFault(
	result *compiledNodePlan,
	identity ruleIdentity,
	requirements []requirement,
	expected failureSet,
) {
	builder.addCompiledFault(result, identity, requirements, expected, nil, planRuleRank(identity.rule))
}

// addFaultAtRank appends one fault target with an applicability-family rank.
func (builder *planBuilder) addFaultAtRank(
	result *compiledNodePlan,
	identity ruleIdentity,
	requirements []requirement,
	expected failureSet,
	ruleRank int,
) {
	builder.addCompiledFault(result, identity, requirements, expected, nil, ruleRank)
}

// addCompiledFault appends one symbolic fault without selecting closure alternatives.
func (builder *planBuilder) addCompiledFault(
	result *compiledNodePlan,
	identity ruleIdentity,
	requirements []requirement,
	expected failureSet,
	alternatives *faultClosureProgram,
	ruleRank int,
) {
	faultObligation := makeFaultObligation(identity, identity.rule)
	faultObligation.ruleRank = encodedPlanRuleRank(ruleRank)
	faultObligation.order = builder.nextOrder
	builder.nextOrder++

	result.faults = append(result.faults, faultProgram{
		obligation:   faultObligation,
		requirements: copyPlanRequirements(requirements),
		expected:     append(failureSet(nil), expected...),
		alternatives: alternatives,
	})
}

// requiredPresenceOccurrence identifies the property slot used by requiredness requirements.
func requiredPresenceOccurrence(node *schemaNode, occurrence schemaOccurrence, name string) schemaOccurrence {
	if property, exists := node.properties[name]; exists {
		return rebasePlanOccurrence(
			property,
			occurrence,
			occurrence.usePointer+"/properties/"+escapePointerToken(name),
			appendInstanceToken(occurrence.instanceTemplate, name),
		)
	}

	return schemaOccurrence{
		usePointer:       occurrence.usePointer + "/properties/" + escapePointerToken(name),
		targetPointer:    occurrence.targetPointer,
		instanceTemplate: appendInstanceToken(occurrence.instanceTemplate, name),
	}
}

// defaultPlanRequirements adds composition defaults for one local schema occurrence.
func defaultPlanRequirements(inherited []requirement, node *schemaNode, occurrence schemaOccurrence) []requirement {
	requirements := appendPlanRequirements(inherited)
	if len(node.allOf) > 0 {
		requirements = appendPlanRequirements(requirements, allOfValidRequirements(occurrence, len(node.allOf))...)
	}

	return requirements
}

// exactCountIsPositive reports whether a parsed lower bound is greater than zero.
func exactCountIsPositive(count *exactCount) (bool, error) {
	if count == nil {
		return false, nil
	}

	zero, err := parseExactNumber("0")
	if err != nil {
		return false, err
	}

	comparison, err := count.number.compare(zero)
	if err != nil {
		return false, err
	}

	return comparison > 0, nil
}

// defaultPresenceRequirementsForKind chooses structural defaults for one applicable JSON kind.
func defaultPresenceRequirementsForKind(
	node *schemaNode,
	occurrence schemaOccurrence,
	kind jsonKind,
) ([]requirement, error) {
	if node == nil || node.schemaShape == nil {
		return nil, errors.New("schema occurrence has no shape")
	}

	switch kind {
	case jsonArray:
		return defaultArrayPresenceRequirements(node, occurrence)
	case jsonObject:
		return defaultObjectPresenceRequirements(node, occurrence)
	default:
		return nil, nil
	}
}

// defaultArrayPresenceRequirements chooses the smallest item presence satisfying minItems.
func defaultArrayPresenceRequirements(node *schemaNode, occurrence schemaOccurrence) ([]requirement, error) {
	if node.items == nil {
		return nil, nil
	}

	presence := requirementAbsent

	positive, err := exactCountIsPositive(node.minItems)
	if err != nil {
		return nil, err
	}

	if positive {
		presence = requirementPresent
	}

	itemOccurrence := rebasePlanOccurrence(
		node.items,
		occurrence,
		occurrence.usePointer+"/items",
		appendInstanceToken(occurrence.instanceTemplate, "*"),
	)

	return []requirement{canonicalPresenceRequirement(itemOccurrence, presence)}, nil
}

// defaultObjectPresenceRequirements chooses required members, enough lower-bound members, and no extras.
//
//nolint:cyclop // Required and lower-bound presence decisions share one canonical pass.
func defaultObjectPresenceRequirements(node *schemaNode, occurrence schemaOccurrence) ([]requirement, error) {
	shape := node.schemaShape

	names := make(map[string]bool, len(shape.properties)+len(shape.required))

	for name := range shape.properties {
		names[name] = true
	}

	for _, name := range shape.required {
		names[name] = true
	}

	sortedNames := make([]string, 0, len(names))
	for name := range names {
		sortedNames = append(sortedNames, name)
	}

	sort.Strings(sortedNames)

	requirements := make([]requirement, 0, len(sortedNames)+1)
	presentCount := 0

	for _, name := range sortedNames {
		presence := requirementAbsent
		if containsString(shape.required, name) {
			presence = requirementPresent
		} else {
			needsMember, err := objectMinimumNeedsMember(shape.minProperties, presentCount)
			if err != nil {
				return nil, err
			}

			if needsMember {
				presence = requirementPresent
			}
		}

		if presence == requirementPresent {
			presentCount++
		}

		requirements = append(
			requirements,
			canonicalPresenceRequirement(requiredPresenceOccurrence(node, occurrence, name), presence),
		)
	}

	if shape.additionalProperties != nil {
		presence := requirementAbsent

		needsMember, err := objectMinimumNeedsMember(shape.minProperties, presentCount)
		if err != nil {
			return nil, err
		}

		if needsMember {
			presence = requirementPresent
		}

		additionalOccurrence := rebasePlanOccurrence(
			shape.additionalProperties,
			occurrence,
			occurrence.usePointer+"/additionalProperties",
			appendInstanceToken(occurrence.instanceTemplate, "*"),
		)
		requirements = append(requirements, canonicalPresenceRequirement(additionalOccurrence, presence))
	}

	return requirements, nil
}

// objectMinimumNeedsMember reports whether one more default member is needed.
func objectMinimumNeedsMember(minimum *exactCount, presentCount int) (bool, error) {
	if minimum == nil {
		return false, nil
	}

	actual, err := parseExactNumber(fmt.Sprintf("%d", presentCount))
	if err != nil {
		return false, err
	}

	comparison, err := actual.compare(minimum.number)
	if err != nil {
		return false, err
	}

	return comparison < 0, nil
}

// validRequirementsForKind records the target kind and local structural defaults.
// Composition truth is added only when the target itself names a composition
// level or lies on a branch activation path.
func (builder *planBuilder) validRequirementsForKind(
	inherited []requirement,
	node *schemaNode,
	occurrence schemaOccurrence,
	kind jsonKind,
) ([]requirement, error) {
	requirements := appendPlanRequirements(inherited)

	if node != nil {
		defaults, err := defaultPresenceRequirementsForKind(node, occurrence, kind)
		if err != nil {
			return nil, err
		}

		requirements = appendPlanRequirements(requirements, defaults...)
	}

	return appendPlanRequirements(requirements, kindRequirement(occurrence, kind)), nil
}

// faultRequirementsForKind records only local applicability. Composition closure is
// represented by the containing fault program, never inferred from a value.
func (builder *planBuilder) faultRequirementsForKind(
	inherited []requirement,
	occurrence schemaOccurrence,
	kind jsonKind,
) []requirement {
	return appendPlanRequirements(inherited, kindRequirement(occurrence, kind))
}

// validAnyOfRequirements preserves inherited requirements for an untyped local target.
func (builder *planBuilder) validAnyOfRequirements(inherited []requirement) []requirement {
	return appendPlanRequirements(inherited)
}

// faultRequirementsForAny preserves inherited applicability without choosing a branch.
func (builder *planBuilder) faultRequirementsForAny(inherited []requirement) []requirement {
	return appendPlanRequirements(inherited)
}

// faultRequirementsForRule records the rule's applicable JSON kind.
func (builder *planBuilder) faultRequirementsForRule(
	inherited []requirement,
	occurrence schemaOccurrence,
	kind jsonKind,
) []requirement {
	return appendPlanRequirements(inherited, kindRequirement(occurrence, kind))
}

// faultRequirementsForRequired records the changed member role and unaffected required siblings.
func (builder *planBuilder) faultRequirementsForRequired(
	inherited []requirement,
	node *schemaNode,
	occurrence schemaOccurrence,
	name string,
) []requirement {
	requirements := appendPlanRequirements(inherited, kindRequirement(occurrence, jsonObject))

	return appendPlanRequirements(requirements, requiredFaultSiblingRequirements(node, occurrence, name)...)
}

// faultRequirementsForAdditional records the object occurrence whose undeclared member role changes.
func (builder *planBuilder) faultRequirementsForAdditional(
	inherited []requirement,
	occurrence schemaOccurrence,
) []requirement {
	return appendPlanRequirements(inherited, kindRequirement(occurrence, jsonObject))
}

// faultRequirementsForEnum retains sibling applicability without choosing a witness kind.
func (builder *planBuilder) faultRequirementsForEnum(inherited []requirement) []requirement {
	return appendPlanRequirements(inherited)
}

// requiredFaultSiblingRequirements requirements unaffected required members present.
func requiredFaultSiblingRequirements(node *schemaNode, occurrence schemaOccurrence, omitted string) []requirement {
	requirements := make([]requirement, 0, len(node.required))

	for _, name := range node.required {
		if name == omitted {
			continue
		}

		requirements = append(
			requirements,
			presenceRequirement(requiredPresenceOccurrence(node, occurrence, name), requirementPresent),
		)
	}

	return requirements
}

// nodeAcceptsKindForTarget reports whether a valid target can use one JSON kind.
func nodeAcceptsKindForTarget(node *schemaNode, kind jsonKind) bool {
	if node.kind == schemaAny {
		return true
	}

	if kind == jsonNull {
		return node.nullable
	}

	return schemaNodeJSONKind(node.kind) == kind
}

// nodeCanHaveKind reports whether a type-specific rule can apply to a JSON kind.
func nodeCanHaveKind(node *schemaNode, kind jsonKind) bool {
	if node.kind == schemaAny {
		return kind != jsonNull
	}

	return schemaNodeJSONKind(node.kind) == kind
}

// schemaNodeJSONKind translates a schema kind to its clean JSON kind.
func schemaNodeJSONKind(kind schemaKind) jsonKind {
	switch kind {
	case schemaBoolean:
		return jsonBoolean
	case schemaInteger, schemaNumber:
		return jsonNumber
	case schemaString:
		return jsonString
	case schemaArray:
		return jsonArray
	case schemaObject:
		return jsonObject
	default:
		return jsonNull
	}
}

// orderedTypeKinds returns canonical type levels for one schema occurrence.
func orderedTypeKinds(node *schemaNode) []jsonKind {
	if node.kind != schemaAny {
		kind := schemaNodeJSONKind(node.kind)

		ordered := []jsonKind{kind}
		if node.nullable {
			ordered = append(ordered, jsonNull)
		}

		return ordered
	}

	return []jsonKind{jsonBoolean, jsonNull, jsonNumber, jsonString, jsonArray, jsonObject}
}

// canonicalJSONKinds returns the locked JSON kind order.
func canonicalJSONKinds() []jsonKind {
	return []jsonKind{jsonNull, jsonBoolean, jsonNumber, jsonString, jsonArray, jsonObject}
}

// memberKindRequirement requirements an enum target to its member's JSON kind.
func memberKindRequirement(occurrence schemaOccurrence, value *jsonValue) requirement {
	return kindRequirement(occurrence, value.kind)
}

// kindRequirement requirements one schema occurrence to a JSON kind.
func kindRequirement(occurrence schemaOccurrence, kind jsonKind) requirement {
	return requirement{
		tag:        requirementJSONKind,
		occurrence: occurrence,
		kind:       kind,
		hasKind:    true,
	}
}

// presenceRequirement requirements one child occurrence to present or absent.
func presenceRequirement(occurrence schemaOccurrence, presence requirementPresence) requirement {
	return requirement{
		tag:        requirementPresenceState,
		occurrence: occurrence,
		presence:   presence,
	}
}

// canonicalPresenceRequirement records the first structural assignment without making it a hard target precondition.
func canonicalPresenceRequirement(occurrence schemaOccurrence, presence requirementPresence) requirement {
	return requirement{
		tag:        requirementPresenceState,
		occurrence: occurrence,
		presence:   presence,
		canonical:  true,
	}
}

// allOfValidRequirements requirements every allOf branch true.
func allOfValidRequirements(occurrence schemaOccurrence, count int) []requirement {
	return compositionRequirements(occurrence, "allOf", count, -1, true)
}

// allOfFaultRequirements requirements one allOf branch false and all sibling branches true.
func allOfFaultRequirements(occurrence schemaOccurrence, count, selected int) []requirement {
	return compositionFaultRequirements(occurrence, "allOf", count, selected)
}

// anyOfValidRequirements requirements the selected anyOf branch true without constraining siblings.
func anyOfValidRequirements(occurrence schemaOccurrence, selected int) []requirement {
	branchOccurrence := schemaOccurrence{
		usePointer:       occurrence.usePointer + "/anyOf/" + itoa(selected),
		targetPointer:    occurrence.targetPointer,
		instanceTemplate: occurrence.instanceTemplate,
	}

	return []requirement{{
		tag:         requirementBranchTruth,
		occurrence:  branchOccurrence,
		composition: "anyOf",
		branch:      selected,
		truth:       true,
		hasBranch:   true,
	}}
}

// anyOfFaultRequirements requirements every authored anyOf branch false.
func anyOfFaultRequirements(occurrence schemaOccurrence, count int) []requirement {
	return compositionRequirements(occurrence, "anyOf", count, -1, false)
}

// compositionRequirements creates one truth requirement for every branch.
func compositionRequirements(
	occurrence schemaOccurrence,
	composition string,
	count, selected int,
	truth bool,
) []requirement {
	requirements := make([]requirement, 0, count)
	for index := 0; index < count; index++ {
		branchOccurrence := schemaOccurrence{
			usePointer:       occurrence.usePointer + "/" + composition + "/" + itoa(index),
			targetPointer:    occurrence.targetPointer,
			instanceTemplate: occurrence.instanceTemplate,
		}

		branchTruth := truth
		if selected >= 0 {
			branchTruth = index == selected
		}

		requirements = append(requirements, requirement{
			tag:         requirementBranchTruth,
			occurrence:  branchOccurrence,
			composition: composition,
			branch:      index,
			truth:       branchTruth,
			hasBranch:   true,
		})
	}

	return requirements
}

// compositionFaultRequirements creates a branch-failure context for one composition child.
func compositionFaultRequirements(occurrence schemaOccurrence, composition string, count, selected int) []requirement {
	requirements := make([]requirement, 0, count)
	for index := 0; index < count; index++ {
		branchOccurrence := schemaOccurrence{
			usePointer:       occurrence.usePointer + "/" + composition + "/" + itoa(index),
			targetPointer:    occurrence.targetPointer,
			instanceTemplate: occurrence.instanceTemplate,
		}
		requirements = append(requirements, requirement{
			tag:         requirementBranchTruth,
			occurrence:  branchOccurrence,
			composition: composition,
			branch:      index,
			truth:       index != selected,
			hasBranch:   true,
		})
	}

	return requirements
}

// anyOfMaskRequirements requirements the complete authored anyOf truth mask.
func anyOfMaskRequirements(occurrence schemaOccurrence, count int, mask *big.Int) []requirement {
	requirements := make([]requirement, 0, count)
	for index := 0; index < count; index++ {
		requirements = append(requirements, requirement{
			tag: requirementBranchTruth,
			occurrence: schemaOccurrence{
				usePointer:       occurrence.usePointer + "/anyOf/" + itoa(index),
				targetPointer:    occurrence.targetPointer,
				instanceTemplate: occurrence.instanceTemplate,
			},
			composition: "anyOf",
			branch:      index,
			truth:       mask.Bit(index) == 1,
			hasBranch:   true,
		})
	}

	return requirements
}

// rebasePlanOccurrence carries a child shape to its use site and instance template.
func rebasePlanOccurrence(
	child *schemaNode,
	parent schemaOccurrence,
	usePointer, instanceTemplate string,
) schemaOccurrence {
	occurrence := rebaseChildOccurrence(child, parent, usePointer, instanceTemplate)
	occurrence.structured = nil

	return occurrence
}

// appendPlanRequirements merges later requirements over earlier requirements for the same dimension.
func appendPlanRequirements(base []requirement, requirements ...requirement) []requirement {
	result := copyPlanRequirements(base)

	for _, requirement := range requirements {
		merged := false

		for index := range result {
			if !samePlanRequirementOccurrence(result[index], requirement) {
				continue
			}

			result[index] = mergePlanRequirements(result[index], requirement)
			merged = true

			break
		}

		if !merged {
			result = append(result, requirement)
		}
	}

	return result
}

// samePlanRequirementOccurrence identifies requirements that describe one occurrence and composition branch.
func samePlanRequirementOccurrence(left, right requirement) bool {
	return left.tag == right.tag &&
		left.occurrence.usePointer == right.occurrence.usePointer &&
		left.occurrence.instanceTemplate == right.occurrence.instanceTemplate &&
		left.composition == right.composition &&
		(left.tag != requirementTargetLevel || left.target.ruleIdentity == right.target.ruleIdentity) &&
		left.hasBranch == right.hasBranch &&
		(!left.hasBranch || left.branch == right.branch)
}

// mergePlanRequirements lets an explicit later requirement override one earlier dimension.
func mergePlanRequirements(left, right requirement) requirement {
	merged := left
	if right.active != nil {
		merged.active = right.active
	}

	if right.target.rule != "" {
		merged.target = right.target
	}

	if right.enumMember != nil {
		merged.enumMember = right.enumMember
	}

	if right.count != nil {
		merged.count = right.count
	}

	if right.hasKind {
		merged.occurrence = right.occurrence
		merged.kind = right.kind
		merged.hasKind = true
	}

	if right.presence != requirementNoPresence && (!right.canonical || left.canonical) {
		merged.occurrence = right.occurrence
		merged.presence = right.presence
		merged.canonical = right.canonical
	}

	if right.hasBranch {
		merged.occurrence = right.occurrence
		merged.composition = right.composition
		merged.branch = right.branch
		merged.truth = right.truth
		merged.hasBranch = true
	}

	return merged
}

// copyPlanRequirements copies requirements without retaining a caller-owned backing array.
func copyPlanRequirements(requirements []requirement) []requirement {
	if len(requirements) == 0 {
		return nil
	}

	return append([]requirement(nil), requirements...)
}

// sortedSchemaPropertyNames returns property names in UTF-8 byte order.
func sortedSchemaPropertyNames(properties map[string]*schemaNode) []string {
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// containsString reports whether a required name is present.
func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}

	return false
}

// validatePlanOccurrences checks every generated pointer before planning can escape.
//
//nolint:cyclop // The validator checks every plan record class at the same seam.
func validatePlanOccurrences(plan compiledNodePlan) error {
	validate := func(occurrence schemaOccurrence) error {
		if _, err := parsePlanPointer(occurrence.usePointer, true); err != nil {
			return err
		}

		if _, err := parsePlanPointer(occurrence.targetPointer, true); err != nil {
			return err
		}

		if _, err := parsePlanPointer(occurrence.instanceTemplate, false); err != nil {
			return err
		}

		return nil
	}

	for _, target := range plan.valid {
		if err := validate(target.obligation.occurrence); err != nil {
			return err
		}

		for _, requirement := range target.requirements {
			if err := validate(requirement.occurrence); err != nil {
				return err
			}
		}
	}

	for _, target := range plan.faults {
		if err := validate(target.obligation.occurrence); err != nil {
			return err
		}

		for _, requirement := range target.requirements {
			if err := validate(requirement.occurrence); err != nil {
				return err
			}
		}

		for _, failure := range target.expected {
			if err := validate(failure.occurrence); err != nil {
				return err
			}
		}
	}

	return nil
}

// rejectDuplicateObligations rejects IDs that alias distinct target metadata.
func rejectDuplicateObligations(obligations []obligation) error {
	seen := make(map[string]schemaOccurrence, len(obligations))
	for _, current := range obligations {
		identity := current.String()

		previous, exists := seen[identity]
		if !exists {
			seen[identity] = current.occurrence

			continue
		}

		if previous.targetPointer != current.occurrence.targetPointer || previous.reference != current.occurrence.reference {
			return fmt.Errorf("obligation identity %q aliases distinct schema targets", identity)
		}

		return fmt.Errorf("duplicate obligation identity %q", identity)
	}

	return nil
}

// itoa formats planner array indices without locale or allocation state.
func itoa(value int) string {
	return fmt.Sprintf("%d", value)
}

// oracleScalarValidLevel is the single valid level for scalar rules.
const oracleScalarValidLevel = "valid"
