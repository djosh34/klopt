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
//nolint:cyclop // Canonical compilation, validation, and sorting are separate required phases.
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

	if err := validatePlanOccurrences(compiled); err != nil {
		return nil, err
	}

	if err := stablePlanSort(compiled.valid, func(left, right validIntent) (int, error) {
		return comparePlanObligations(left.obligation, right.obligation)
	}); err != nil {
		return nil, fmt.Errorf("sort valid obligations: %w", err)
	}

	if err := stablePlanSort(compiled.faults, func(left, right faultProgram) (int, error) {
		return comparePlanObligations(left.obligation, right.obligation)
	}); err != nil {
		return nil, fmt.Errorf("sort fault obligations: %w", err)
	}

	obligations := make([]obligation, 0, len(compiled.valid)+len(compiled.faults))
	for _, target := range compiled.valid {
		obligations = append(obligations, target.obligation)
	}

	for _, target := range compiled.faults {
		obligations = append(obligations, target.obligation)
	}

	if err := stablePlanSort(obligations, comparePlanObligations); err != nil {
		return nil, fmt.Errorf("sort obligations: %w", err)
	}

	if err := rejectDuplicateObligations(obligations); err != nil {
		return nil, err
	}

	return &searchPlan{
		validSchedule: compiled.valid,
		faultSchedule: compiled.faults,
		obligations:   obligations,
	}, nil
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
	validPins := appendPlanPins(defaultPlanPins(validInherited, node, occurrence), activeRules)
	faultPins := appendPlanPins(defaultPlanPins(faultInherited, node, occurrence), activeRules)
	result := compiledNodePlan{}

	if err := builder.compileTypeRules(&result, node, occurrence, validPins, faultPins); err != nil {
		return compiledNodePlan{}, err
	}

	if err := builder.compileEnumRules(&result, node, occurrence, validPins, faultPins); err != nil {
		return compiledNodePlan{}, err
	}

	if err := builder.compileNumberRules(&result, node, occurrence, validPins, faultPins); err != nil {
		return compiledNodePlan{}, err
	}

	if err := builder.compileStringRules(&result, node, occurrence, validPins, faultPins); err != nil {
		return compiledNodePlan{}, err
	}

	if err := builder.compileArrayRules(&result, node, occurrence, validPins, faultPins); err != nil {
		return compiledNodePlan{}, err
	}

	if err := builder.compileObjectRules(&result, node, occurrence, validPins, faultPins); err != nil {
		return compiledNodePlan{}, err
	}

	allOfIdentity := makeRuleIdentity(occurrence, oracleRuleAllOf)
	if len(node.allOf) > 0 {
		builder.addValid(
			&result,
			allOfIdentity,
			planLevelAllTrue,
			appendPlanPins(validPins, allOfValidPins(occurrence, len(node.allOf))...),
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
				appendPlanPins(validPins, anyOfMaskPins(occurrence, len(node.anyOf), mask)...),
			)
		}
	}

	if err := builder.compileChildren(
		&result,
		node,
		occurrence,
		validPins,
		faultPins,
		visiting,
	); err != nil {
		return compiledNodePlan{}, err
	}

	if len(node.anyOf) > 0 {
		if err := builder.compileAnyOfChildren(
			&result,
			node,
			occurrence,
			validPins,
			faultPins,
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
			appendPlanPins(validInherited, anyOfValidPins(occurrence, index)...),
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

	parentFaultPins := anyOfFaultPins(occurrence, len(node.anyOf))

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
				appendPlanPins(candidate.requirements, parentFaultPins...),
				closure,
				alternatives,
				obligationRuleRank(candidate.obligation),
			)
		}
	}

	builder.addCompiledFault(
		result,
		anyOfIdentity,
		appendPlanPins(faultInherited, parentFaultPins...),
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
		expected: failureSet{failureIdentity(identity)},
		closure:  closureDomainsExcept(branches, -1),
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
	validParent, validRealizable, err := builder.validPinsForKind(validInherited, parent, parentOccurrence, kind)
	if err != nil {
		return err
	}

	faultParent, faultRealizable, err := builder.faultPinsForKind(faultInherited, parent, parentOccurrence, kind)
	if err != nil {
		return err
	}

	if !validRealizable || !faultRealizable {
		return nil
	}

	faultDefaults, err := defaultPresencePinsForKind(parent, parentOccurrence, kind)
	if err != nil {
		return err
	}

	presence := presencePin(childOccurrence, requirementPresent)

	childPlan, err := builder.compileNode(
		child,
		childOccurrence,
		appendPlanPins(validParent, presence),
		appendPlanPins(appendPlanPins(faultParent, faultDefaults...), presence),
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
	validParent, validRealizable, err := builder.validAnyOfPins(parent, parentOccurrence, validInherited)
	if err != nil {
		return err
	}

	faultParent, faultRealizable, err := builder.faultPinsForAny(faultInherited, parent, parentOccurrence)
	if err != nil {
		return err
	}

	if !validRealizable || !faultRealizable {
		return nil
	}

	childPlan, err := builder.compileNode(
		child,
		childOccurrence,
		appendPlanPins(validParent, allOfValidPins(parentOccurrence, len(parent.allOf))...),
		appendPlanPins(faultParent, allOfFaultPins(parentOccurrence, len(parent.allOf), index)...),
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
		pins, _, err := builder.validPinsForKind(validInherited, node, occurrence, kind)
		if err != nil {
			return err
		}

		builder.addValid(result, identity, jsonKindName(kind), pins)
	}

	if node.kind != schemaAny {
		return builder.addFault(result, identity, faultInherited, failureSet{failureIdentity(identity)})
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

		pins, realizable, pinErr := builder.validPinsForKind(validInherited, node, occurrence, member.value.kind)
		if pinErr != nil {
			return pinErr
		}

		if !realizable {
			continue
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
			appendPlanPins(pins, memberRequirement),
		)
	}

	pins, realizable, pinErr := builder.faultPinsForEnum(faultInherited, node, occurrence)
	if pinErr != nil {
		return pinErr
	}

	if realizable {
		if err := builder.addFault(result, identity, pins, []failureIdentity{identity}); err != nil {
			return err
		}
	}

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

		validPins, realizable, err := builder.validPinsForKind(validInherited, node, occurrence, jsonObject)
		if err != nil {
			return err
		}

		if realizable {
			builder.addValid(
				result,
				identity,
				oracleRequiredPresentLevel,
				appendPlanPins(validPins, presencePin(presenceOccurrence, requirementPresent)),
			)
		}

		faultPins, realizable, err := builder.faultPinsForRequired(
			faultInherited, node, occurrence, name,
		)
		if err != nil {
			return err
		}

		if realizable {
			if err := builder.addFault(
				result,
				identity,
				appendPlanPins(faultPins, presencePin(presenceOccurrence, requirementAbsent)),
				[]failureIdentity{identity},
			); err != nil {
				return err
			}
		}
	}

	if node.additionalProperties == nil && !node.allowAdditionalProperties {
		identity := makeRuleIdentity(
			appendObjectMemberOccurrence(occurrence, "*"),
			oracleRuleAdditionalProperties,
		)

		pins, realizable, err := builder.faultPinsForAdditional(
			faultInherited, node, occurrence,
		)
		if err != nil {
			return err
		}

		if realizable {
			if err := builder.addFault(
				result,
				identity,
				appendPlanPins(pins, presencePin(identity.occurrence, requirementPresent)),
				[]failureIdentity{identity},
			); err != nil {
				return err
			}
		}
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

	validPins, realizable, err := builder.validPinsForKind(validInherited, node, occurrence, kind)
	if err != nil {
		return err
	}

	if !realizable {
		return nil
	}

	if count := countRequirementForRule(node, occurrence, rule); count != nil {
		validPins = appendPlanPins(validPins, *count)
	}

	builder.addValidAtRank(result, identity, oracleScalarValidLevel, validPins, ruleRank)

	if !faultAllowed {
		return nil
	}

	faultPins, realizable, err := builder.faultPinsForRule(
		faultInherited, node, occurrence, kind, rule,
	)
	if err != nil {
		return err
	}

	if !realizable {
		return nil
	}

	if count := countRequirementForRule(node, occurrence, rule); count != nil {
		faultPins = appendPlanPins(faultPins, *count)
	}

	return builder.addFaultAtRank(result, identity, faultPins, []failureIdentity{identity}, ruleRank)
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
	pins []requirement,
) {
	builder.addValidAtRank(result, identity, level, pins, planRuleRank(identity.rule))
}

// addValidAtRank appends one valid target with an applicability-family rank.
func (builder *planBuilder) addValidAtRank(
	result *compiledNodePlan,
	identity ruleIdentity,
	level string,
	pins []requirement,
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
		requirements: appendPlanPins(pins, requirement{
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
	pins []requirement,
	expected failureSet,
) error {
	builder.addCompiledFault(result, identity, pins, expected, nil, planRuleRank(identity.rule))

	return nil
}

// addFaultAtRank appends one fault target with an applicability-family rank.
func (builder *planBuilder) addFaultAtRank(
	result *compiledNodePlan,
	identity ruleIdentity,
	pins []requirement,
	expected failureSet,
	ruleRank int,
) error {
	builder.addCompiledFault(result, identity, pins, expected, nil, ruleRank)

	return nil
}

// addCompiledFault appends one symbolic fault without selecting closure alternatives.
func (builder *planBuilder) addCompiledFault(
	result *compiledNodePlan,
	identity ruleIdentity,
	pins []requirement,
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
		requirements: copyPlanPins(pins),
		expected:     append(failureSet(nil), expected...),
		alternatives: alternatives,
	})
}

// requiredPresenceOccurrence identifies the property slot used by requiredness pins.
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

// defaultPlanPins adds composition defaults for one local schema occurrence.
func defaultPlanPins(inherited []requirement, node *schemaNode, occurrence schemaOccurrence) []requirement {
	pins := appendPlanPins(inherited)
	if len(node.allOf) > 0 {
		pins = appendPlanPins(pins, allOfValidPins(occurrence, len(node.allOf))...)
	}

	return pins
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

// defaultPresencePinsForKind chooses structural defaults for one applicable JSON kind.
func defaultPresencePinsForKind(
	node *schemaNode,
	occurrence schemaOccurrence,
	kind jsonKind,
) ([]requirement, error) {
	if node == nil || node.schemaShape == nil {
		return nil, errors.New("schema occurrence has no shape")
	}

	switch kind {
	case jsonArray:
		return defaultArrayPresencePins(node, occurrence)
	case jsonObject:
		return defaultObjectPresencePins(node, occurrence)
	default:
		return nil, nil
	}
}

// defaultArrayPresencePins chooses the smallest item presence satisfying minItems.
func defaultArrayPresencePins(node *schemaNode, occurrence schemaOccurrence) ([]requirement, error) {
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

	return []requirement{canonicalPresencePin(itemOccurrence, presence)}, nil
}

// defaultObjectPresencePins chooses required members, enough lower-bound members, and no extras.
//
//nolint:cyclop // Required and lower-bound presence decisions share one canonical pass.
func defaultObjectPresencePins(node *schemaNode, occurrence schemaOccurrence) ([]requirement, error) {
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

	pins := make([]requirement, 0, len(sortedNames)+1)
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

		pins = append(pins, canonicalPresencePin(requiredPresenceOccurrence(node, occurrence, name), presence))
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
		pins = append(pins, canonicalPresencePin(additionalOccurrence, presence))
	}

	return pins, nil
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

// validPinsForKind records the target kind and local structural defaults.
// Composition truth is added only when the target itself names a composition
// level or lies on a branch activation path.
func (builder *planBuilder) validPinsForKind(
	inherited []requirement,
	node *schemaNode,
	occurrence schemaOccurrence,
	kind jsonKind,
) ([]requirement, bool, error) {
	pins := appendPlanPins(inherited)

	if node != nil {
		defaults, err := defaultPresencePinsForKind(node, occurrence, kind)
		if err != nil {
			return nil, false, err
		}

		pins = appendPlanPins(pins, defaults...)
	}

	return appendPlanPins(pins, kindPin(occurrence, kind)), true, nil
}

// faultPinsForKind records only local applicability. Composition closure is
// represented by the containing fault program, never inferred from a value.
func (builder *planBuilder) faultPinsForKind(
	inherited []requirement,
	_ *schemaNode,
	occurrence schemaOccurrence,
	kind jsonKind,
) ([]requirement, bool, error) {
	return appendPlanPins(inherited, kindPin(occurrence, kind)), true, nil
}

// validAnyOfPins preserves inherited requirements for an untyped local target.
func (builder *planBuilder) validAnyOfPins(
	_ *schemaNode,
	_ schemaOccurrence,
	inherited []requirement,
) ([]requirement, bool, error) {
	return appendPlanPins(inherited), true, nil
}

// faultPinsForAny preserves inherited applicability without choosing a branch.
func (builder *planBuilder) faultPinsForAny(
	inherited []requirement,
	_ *schemaNode,
	_ schemaOccurrence,
) ([]requirement, bool, error) {
	return appendPlanPins(inherited), true, nil
}

// faultPinsForRule records the rule's applicable JSON kind.
func (builder *planBuilder) faultPinsForRule(
	inherited []requirement,
	_ *schemaNode,
	occurrence schemaOccurrence,
	kind jsonKind,
	_ string,
) ([]requirement, bool, error) {
	return appendPlanPins(inherited, kindPin(occurrence, kind)), true, nil
}

// faultPinsForRequired records the changed member role and unaffected required siblings.
func (builder *planBuilder) faultPinsForRequired(
	inherited []requirement,
	node *schemaNode,
	occurrence schemaOccurrence,
	name string,
) ([]requirement, bool, error) {
	if node == nil {
		return nil, false, errors.New("schema occurrence has no shape")
	}

	pins := appendPlanPins(inherited, kindPin(occurrence, jsonObject))

	return appendPlanPins(pins, requiredFaultSiblingPins(node, occurrence, name)...), true, nil
}

// faultPinsForAdditional records the object occurrence whose undeclared member role changes.
func (builder *planBuilder) faultPinsForAdditional(
	inherited []requirement,
	node *schemaNode,
	occurrence schemaOccurrence,
) ([]requirement, bool, error) {
	if node == nil {
		return nil, false, errors.New("schema occurrence has no shape")
	}

	return appendPlanPins(inherited, kindPin(occurrence, jsonObject)), true, nil
}

// faultPinsForEnum retains sibling applicability without choosing a witness kind.
func (builder *planBuilder) faultPinsForEnum(
	inherited []requirement,
	node *schemaNode,
	_ schemaOccurrence,
) ([]requirement, bool, error) {
	if node == nil {
		return nil, false, errors.New("schema occurrence has no shape")
	}

	return appendPlanPins(inherited), true, nil
}

// requiredFaultSiblingPins pins unaffected required members present.
func requiredFaultSiblingPins(node *schemaNode, occurrence schemaOccurrence, omitted string) []requirement {
	pins := make([]requirement, 0, len(node.required))

	for _, name := range node.required {
		if name == omitted {
			continue
		}

		pins = append(pins, presencePin(requiredPresenceOccurrence(node, occurrence, name), requirementPresent))
	}

	return pins
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

	allowed := make(map[jsonKind]bool)
	for _, kind := range canonicalJSONKinds() {
		allowed[kind] = true
	}

	ordered := make([]jsonKind, 0, len(allowed))
	if first, ok := firstSiblingCompatibleKind(node, allowed); ok {
		ordered = append(ordered, first)
		delete(allowed, first)
	}

	for _, kind := range canonicalJSONKinds() {
		if allowed[kind] {
			ordered = append(ordered, kind)
		}
	}

	return ordered
}

// firstSiblingCompatibleKind selects the first authored or constrained non-null kind.
//
//nolint:cyclop // Authored, constrained, and canonical kind priorities are explicit.
func firstSiblingCompatibleKind(node *schemaNode, allowed map[jsonKind]bool) (jsonKind, bool) {
	if node.enum != nil {
		for _, member := range node.enum {
			if member.value != nil && member.value.kind != jsonNull && allowed[member.value.kind] {
				return member.value.kind, true
			}
		}
	}

	for _, kind := range []jsonKind{jsonNumber, jsonString, jsonArray, jsonObject} {
		if allowed[kind] && nodeHasSiblingRuleForKind(node, kind) {
			return kind, true
		}
	}

	for _, kind := range canonicalJSONKinds() {
		if allowed[kind] {
			return kind, true
		}
	}

	return jsonNull, false
}

// nodeHasSiblingRuleForKind reports whether a type-specific keyword guides a kind.
//
//nolint:cyclop // Each supported JSON kind has an explicit keyword family.
func nodeHasSiblingRuleForKind(node *schemaNode, kind jsonKind) bool {
	switch kind {
	case jsonNumber:
		return node.minimum != nil || node.maximum != nil || node.multipleOf != nil || isNumericSchemaFormat(node.format)
	case jsonString:
		return node.minLength != nil || node.maxLength != nil || node.pattern != nil || isStringSchemaFormat(node.format)
	case jsonArray:
		return node.minItems != nil || node.maxItems != nil || node.items != nil
	case jsonObject:
		return node.minProperties != nil || node.maxProperties != nil || len(node.required) > 0 ||
			len(node.properties) > 0 || node.additionalProperties != nil || !node.allowAdditionalProperties
	default:
		return false
	}
}

// canonicalJSONKinds returns the locked JSON kind order.
func canonicalJSONKinds() []jsonKind {
	return []jsonKind{jsonNull, jsonBoolean, jsonNumber, jsonString, jsonArray, jsonObject}
}

// memberKindPin pins an enum target to its member's JSON kind.
func memberKindPin(occurrence schemaOccurrence, value *jsonValue) requirement {
	return kindPin(occurrence, value.kind)
}

// kindPin pins one schema occurrence to a JSON kind.
func kindPin(occurrence schemaOccurrence, kind jsonKind) requirement {
	return requirement{
		tag:        requirementJSONKind,
		occurrence: occurrence,
		kind:       kind,
		hasKind:    true,
	}
}

// presencePin pins one child occurrence to present or absent.
func presencePin(occurrence schemaOccurrence, presence requirementPresence) requirement {
	return requirement{
		tag:        requirementPresenceState,
		occurrence: occurrence,
		presence:   presence,
	}
}

// canonicalPresencePin records the first structural assignment without making it a hard target precondition.
func canonicalPresencePin(occurrence schemaOccurrence, presence requirementPresence) requirement {
	return requirement{
		tag:        requirementPresenceState,
		occurrence: occurrence,
		presence:   presence,
		canonical:  true,
	}
}

// allOfValidPins pins every allOf branch true.
func allOfValidPins(occurrence schemaOccurrence, count int) []requirement {
	return compositionPins(occurrence, "allOf", count, -1, true)
}

// allOfFaultPins pins one allOf branch false and all sibling branches true.
func allOfFaultPins(occurrence schemaOccurrence, count, selected int) []requirement {
	return compositionFaultPins(occurrence, "allOf", count, selected)
}

// anyOfValidPins pins the selected anyOf branch true without constraining siblings.
func anyOfValidPins(occurrence schemaOccurrence, selected int) []requirement {
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

// anyOfFaultPins pins every authored anyOf branch false.
func anyOfFaultPins(occurrence schemaOccurrence, count int) []requirement {
	return compositionPins(occurrence, "anyOf", count, -1, false)
}

// compositionPins creates one truth pin for every branch.
func compositionPins(
	occurrence schemaOccurrence,
	composition string,
	count, selected int,
	truth bool,
) []requirement {
	pins := make([]requirement, 0, count)
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

		pins = append(pins, requirement{
			tag:         requirementBranchTruth,
			occurrence:  branchOccurrence,
			composition: composition,
			branch:      index,
			truth:       branchTruth,
			hasBranch:   true,
		})
	}

	return pins
}

// compositionFaultPins creates a branch-failure context for one composition child.
func compositionFaultPins(occurrence schemaOccurrence, composition string, count, selected int) []requirement {
	pins := make([]requirement, 0, count)
	for index := 0; index < count; index++ {
		branchOccurrence := schemaOccurrence{
			usePointer:       occurrence.usePointer + "/" + composition + "/" + itoa(index),
			targetPointer:    occurrence.targetPointer,
			instanceTemplate: occurrence.instanceTemplate,
		}
		pins = append(pins, requirement{
			tag:         requirementBranchTruth,
			occurrence:  branchOccurrence,
			composition: composition,
			branch:      index,
			truth:       index != selected,
			hasBranch:   true,
		})
	}

	return pins
}

// anyOfMaskPins pins the complete authored anyOf truth mask.
func anyOfMaskPins(occurrence schemaOccurrence, count int, mask *big.Int) []requirement {
	pins := make([]requirement, 0, count)
	for index := 0; index < count; index++ {
		pins = append(pins, requirement{
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

	return pins
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

// appendPlanPins merges later pins over earlier pins for the same dimension.
func appendPlanPins(base []requirement, pins ...requirement) []requirement {
	result := copyPlanPins(base)

	for _, pin := range pins {
		merged := false

		for index := range result {
			if !samePlanPinOccurrence(result[index], pin) {
				continue
			}

			result[index] = mergePlanPins(result[index], pin)
			merged = true

			break
		}

		if !merged {
			result = append(result, pin)
		}
	}

	return result
}

// samePlanPinOccurrence identifies pins that describe one occurrence and composition branch.
func samePlanPinOccurrence(left, right requirement) bool {
	return left.tag == right.tag &&
		left.occurrence.usePointer == right.occurrence.usePointer &&
		left.occurrence.instanceTemplate == right.occurrence.instanceTemplate &&
		left.composition == right.composition &&
		left.hasBranch == right.hasBranch &&
		(!left.hasBranch || left.branch == right.branch)
}

// mergePlanPins lets an explicit later pin override one earlier dimension.
func mergePlanPins(left, right requirement) requirement {
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

	if right.presence != requirementNoPresence {
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

// copyPlanPins copies pins without retaining a caller-owned backing array.
func copyPlanPins(pins []requirement) []requirement {
	if len(pins) == 0 {
		return nil
	}

	return append([]requirement(nil), pins...)
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

		for _, pin := range target.requirements {
			if err := validate(pin.occurrence); err != nil {
				return err
			}
		}
	}

	for _, target := range plan.faults {
		if err := validate(target.obligation.occurrence); err != nil {
			return err
		}

		for _, pin := range target.requirements {
			if err := validate(pin.occurrence); err != nil {
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
