package schematest

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
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

	if err := stablePlanSort(compiled.valid, func(left, right validTarget) (int, error) {
		return comparePlanObligations(left.obligation, right.obligation)
	}); err != nil {
		return nil, fmt.Errorf("sort valid obligations: %w", err)
	}

	if err := stablePlanSort(compiled.faults, func(left, right faultTarget) (int, error) {
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
		validTargets: compiled.valid,
		faultTargets: compiled.faults,
		obligations:  obligations,
	}, nil
}

// planBuilder owns deterministic insertion order while compiling one model.
type planBuilder struct {
	nextOrder uint64
}

// compiledNodePlan contains targets collected below one schema occurrence.
type compiledNodePlan struct {
	valid  []validTarget
	faults []faultTarget
}

// anyOfBranchPlan contains branch faults that can represent an exact closure.
type anyOfBranchPlan struct {
	node            *schemaNode
	occurrence      schemaOccurrence
	representatives []faultTarget
}

// compileNode compiles one occurrence with separate valid and fault context.
//
//nolint:cyclop // The compilation phases must remain in canonical order.
func (builder *planBuilder) compileNode(
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
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

	validPins := defaultPlanPins(validInherited, node, occurrence)
	faultPins := defaultPlanPins(faultInherited, node, occurrence)
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
		allOfPins, realizable, err := builder.validAnyOfPins(node, occurrence, validPins)
		if err != nil {
			return compiledNodePlan{}, err
		}

		if realizable {
			builder.addValid(
				&result,
				allOfIdentity,
				planLevelAllTrue,
				appendPlanPins(allOfPins, allOfValidPins(occurrence, len(node.allOf))...),
			)
		}
	}

	anyOfIdentity := makeRuleIdentity(occurrence, oracleRuleAnyOf)

	if len(node.anyOf) > 0 {
		masks, err := realizableAnyOfMasks(node)
		if err != nil {
			return compiledNodePlan{}, err
		}

		for _, mask := range masks {
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

// compileAnyOfChildren compiles branch targets and complete anyOf fault closures.
//
//nolint:cyclop,gocognit // Branch compilation and exact closure assembly are one canonical phase.
func (builder *planBuilder) compileAnyOfChildren(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
	visiting map[*schemaNode]bool,
	anyOfIdentity ruleIdentity,
) error {
	branches := make([]anyOfBranchPlan, len(node.anyOf))

	for index, child := range node.anyOf {
		childOccurrence := rebasePlanOccurrence(
			child,
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

		representatives, candidateErr := realizableFaultCandidates(child, childOccurrence, childPlan.faults)
		if candidateErr != nil {
			return candidateErr
		}

		branches[index] = anyOfBranchPlan{
			node:            child,
			occurrence:      childOccurrence,
			representatives: representatives,
		}
	}

	representatives, parentKind, exists := selectAnyOfRepresentativesForParent(branches, node)
	if !exists {
		return nil
	}

	parentFaultPins := anyOfFaultPins(occurrence, len(node.anyOf))

	for index, branch := range branches {
		for _, candidate := range branch.representatives {
			compatible := true

			for siblingIndex, representative := range representatives {
				if siblingIndex != index && !planPinsCompatible(candidate.pins, representative.pins) {
					compatible = false

					break
				}
			}

			if !compatible {
				continue
			}

			pins := appendPlanPins(candidate.pins, parentFaultPins...)
			closure := append([]failureIdentity(nil), candidate.closure...)

			for siblingIndex, representative := range representatives {
				if siblingIndex == index {
					continue
				}

				pins = appendPlanPins(pins, representative.pins...)
				closure = append(closure, representative.closure...)
			}

			closure = append(closure, failureIdentity(anyOfIdentity))

			if err := builder.addFaultAtRank(
				result,
				candidate.obligation.ruleIdentity,
				pins,
				closure,
				obligationRuleRank(candidate.obligation),
			); err != nil {
				return err
			}
		}
	}

	aggregatePins := appendPlanPins(faultInherited, parentFaultPins...)
	aggregatePins = appendPlanPins(aggregatePins, kindPin(occurrence, parentKind))
	closure := make([]failureIdentity, 0)

	for _, representative := range representatives {
		aggregatePins = appendPlanPins(aggregatePins, representative.pins...)
		closure = append(closure, representative.closure...)
	}

	closure = append(closure, failureIdentity(anyOfIdentity))

	return builder.addFault(result, anyOfIdentity, aggregatePins, closure)
}

// compileChildren compiles items, properties, additional schemas, and allOf branches.
func (builder *planBuilder) compileChildren(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
	visiting map[*schemaNode]bool,
) error {
	shape := node.schemaShape
	if shape.items != nil {
		itemOccurrence := rebasePlanOccurrence(
			shape.items,
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
	validInherited, faultInherited []applicabilityPin,
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

	presence := presencePin(childOccurrence, planPinPresent)

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
	validInherited, faultInherited []applicabilityPin,
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
//
//nolint:nestif // Type-fault witness selection must remain beside type-level compilation.
func (builder *planBuilder) compileTypeRules(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
) error {
	identity := makeRuleIdentity(occurrence, oracleRuleType)
	for _, kind := range orderedTypeKinds(node) {
		pins, realizable, err := builder.validPinsForKind(validInherited, node, occurrence, kind)
		if err != nil {
			return err
		}

		if !realizable {
			continue
		}

		builder.addValid(result, identity, jsonKindName(kind), pins)
	}

	if node.kind != schemaAny {
		typeFaultRealizable, err := typeFaultHasWitness(node)
		if err != nil {
			return err
		}

		if !typeFaultRealizable {
			return nil
		}

		pins, realizable, err := builder.faultPinsForType(faultInherited, node, occurrence)
		if err != nil {
			return err
		}

		if realizable {
			if err := builder.addFault(result, identity, pins, []failureIdentity{identity}); err != nil {
				return err
			}
		}
	}

	return nil
}

// typeFaultHasWitness reports whether enum admits a wrong-kind witness for the declared type.
func typeFaultHasWitness(node *schemaNode) (bool, error) {
	if node.enum == nil {
		return true, nil
	}

	for _, member := range node.enum {
		matches, err := valueMatchesNodeKind(member, node.kind, node.nullable)
		if err != nil {
			return false, err
		}

		if !matches {
			return true, nil
		}
	}

	return false, nil
}

// compileEnumRules compiles semantic enum members and the enum fault.
//
//nolint:cyclop // Enum validity and faultability are separate canonical decisions.
func (builder *planBuilder) compileEnumRules(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
) error {
	if node.enum == nil {
		return nil
	}

	members, err := canonicalPlanEnum(node.enum)
	if err != nil {
		return fmt.Errorf("%s/enum: %w", occurrence.targetPointer, err)
	}

	identity := makeRuleIdentity(occurrence, oracleRuleEnum)

	for index, member := range members {
		matches, matchErr := valueMatchesNodeKind(member, node.kind, node.nullable)
		if matchErr != nil {
			return matchErr
		}

		if !matches {
			continue
		}

		pins, realizable, pinErr := builder.validPinsForKind(validInherited, node, occurrence, member.kind)
		if pinErr != nil {
			return pinErr
		}

		if !realizable {
			continue
		}

		authoredIndex := index
		if index < len(node.enumIndices) {
			authoredIndex = node.enumIndices[index]
		}

		builder.addValid(result, identity, "member:"+itoa(authoredIndex), pins)
	}

	if !enumExhaustsNonNullableBoolean(node) {
		pins, realizable, pinErr := builder.faultPinsForEnum(faultInherited, node, occurrence)
		if pinErr != nil {
			return pinErr
		}

		if realizable {
			if err := builder.addFault(result, identity, pins, []failureIdentity{identity}); err != nil {
				return err
			}
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
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
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
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
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
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
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
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
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

	for _, name := range sortedStrings(node.required) {
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
				appendPlanPins(validPins, presencePin(presenceOccurrence, planPinPresent)),
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
				appendPlanPins(faultPins, presencePin(presenceOccurrence, planPinAbsent)),
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
				appendPlanPins(pins, presencePin(identity.occurrence, planPinPresent)),
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
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
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

	return builder.addFaultAtRank(result, identity, faultPins, []failureIdentity{identity}, ruleRank)
}

// addValid appends one valid target with a deterministic insertion number.
func (builder *planBuilder) addValid(
	result *compiledNodePlan,
	identity ruleIdentity,
	level string,
	pins []applicabilityPin,
) {
	builder.addValidAtRank(result, identity, level, pins, planRuleRank(identity.rule))
}

// addValidAtRank appends one valid target with an applicability-family rank.
func (builder *planBuilder) addValidAtRank(
	result *compiledNodePlan,
	identity ruleIdentity,
	level string,
	pins []applicabilityPin,
	ruleRank int,
) {
	validObligation := makeLevelObligation(identity, level)
	validObligation.ruleRank = encodedPlanRuleRank(ruleRank)
	validObligation.order = builder.nextOrder
	builder.nextOrder++

	result.valid = append(result.valid, validTarget{
		obligation: validObligation,
		expected:   makeLevelIdentity(identity, level),
		pins:       copyPlanPins(pins),
	})
}

// addFault appends one fault target with a copied exact failure closure.
func (builder *planBuilder) addFault(
	result *compiledNodePlan,
	identity ruleIdentity,
	pins []applicabilityPin,
	closure []failureIdentity,
) error {
	return builder.addFaultAtRank(result, identity, pins, closure, planRuleRank(identity.rule))
}

// addFaultAtRank appends one fault target with an applicability-family rank.
func (builder *planBuilder) addFaultAtRank(
	result *compiledNodePlan,
	identity ruleIdentity,
	pins []applicabilityPin,
	closure []failureIdentity,
	ruleRank int,
) error {
	canonical, err := canonicalFailureClosure(closure)
	if err != nil {
		return err
	}

	faultObligation := makeFaultObligation(identity, identity.rule)
	faultObligation.ruleRank = encodedPlanRuleRank(ruleRank)
	faultObligation.order = builder.nextOrder
	builder.nextOrder++

	result.faults = append(result.faults, faultTarget{
		obligation: faultObligation,
		pins:       copyPlanPins(pins),
		closure:    canonical,
	})

	return nil
}

// requiredPresenceOccurrence identifies the property slot used by requiredness pins.
func requiredPresenceOccurrence(node *schemaNode, occurrence schemaOccurrence, name string) schemaOccurrence {
	if property, exists := node.properties[name]; exists {
		return rebasePlanOccurrence(
			property,
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
func defaultPlanPins(inherited []applicabilityPin, node *schemaNode, occurrence schemaOccurrence) []applicabilityPin {
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
) ([]applicabilityPin, error) {
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
func defaultArrayPresencePins(node *schemaNode, occurrence schemaOccurrence) ([]applicabilityPin, error) {
	if node.items == nil {
		return nil, nil
	}

	presence := planPinAbsent

	positive, err := exactCountIsPositive(node.minItems)
	if err != nil {
		return nil, err
	}

	if positive {
		presence = planPinPresent
	}

	itemOccurrence := rebasePlanOccurrence(
		node.items,
		occurrence.usePointer+"/items",
		appendInstanceToken(occurrence.instanceTemplate, "*"),
	)

	return []applicabilityPin{presencePin(itemOccurrence, presence)}, nil
}

// defaultObjectPresencePins chooses required members, enough lower-bound members, and no extras.
//
//nolint:cyclop // Required and lower-bound presence decisions share one canonical pass.
func defaultObjectPresencePins(node *schemaNode, occurrence schemaOccurrence) ([]applicabilityPin, error) {
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

	pins := make([]applicabilityPin, 0, len(sortedNames)+1)
	presentCount := 0

	for _, name := range sortedNames {
		presence := planPinAbsent
		if containsString(shape.required, name) {
			presence = planPinPresent
		} else {
			needsMember, err := objectMinimumNeedsMember(shape.minProperties, presentCount)
			if err != nil {
				return nil, err
			}

			if needsMember {
				presence = planPinPresent
			}
		}

		if presence == planPinPresent {
			presentCount++
		}

		pins = append(pins, presencePin(requiredPresenceOccurrence(node, occurrence, name), presence))
	}

	if shape.additionalProperties != nil {
		presence := planPinAbsent

		needsMember, err := objectMinimumNeedsMember(shape.minProperties, presentCount)
		if err != nil {
			return nil, err
		}

		if needsMember {
			presence = planPinPresent
		}

		additionalOccurrence := rebasePlanOccurrence(
			shape.additionalProperties,
			occurrence.usePointer+"/additionalProperties",
			appendInstanceToken(occurrence.instanceTemplate, "*"),
		)
		pins = append(pins, presencePin(additionalOccurrence, presence))
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

// validPinsForKind adds local defaults and the exact parent anyOf state.
func (builder *planBuilder) validPinsForKind(
	inherited []applicabilityPin,
	node *schemaNode,
	occurrence schemaOccurrence,
	kind jsonKind,
) ([]applicabilityPin, bool, error) {
	pins := appendPlanPins(inherited)

	if node != nil && !nodeAcceptsKindForTarget(node, kind) {
		return nil, false, nil
	}

	if node != nil {
		defaults, err := defaultPresencePinsForKind(node, occurrence, kind)
		if err != nil {
			return nil, false, err
		}

		pins = appendPlanPins(pins, defaults...)
	}

	pins = appendPlanPins(pins, kindPin(occurrence, kind))
	if node == nil || len(node.anyOf) == 0 {
		return pins, true, nil
	}

	mask, realizable, err := anyOfMaskForKind(node, kind)
	if err != nil {
		return nil, false, err
	}

	if !realizable {
		return nil, false, nil
	}

	return appendPlanPins(pins, anyOfMaskPins(occurrence, len(node.anyOf), mask)...), true, nil
}

// faultPinsForKind adds the local kind and the exact parent anyOf state.
func (builder *planBuilder) faultPinsForKind(
	inherited []applicabilityPin,
	node *schemaNode,
	occurrence schemaOccurrence,
	kind jsonKind,
) ([]applicabilityPin, bool, error) {
	pins := appendPlanPins(inherited, kindPin(occurrence, kind))
	if node == nil || len(node.anyOf) == 0 {
		return pins, true, nil
	}

	mask, realizable, err := anyOfMaskForKind(node, kind)
	if err != nil {
		return nil, false, err
	}

	if !realizable {
		return nil, false, nil
	}

	return appendPlanPins(pins, anyOfMaskPins(occurrence, len(node.anyOf), mask)...), true, nil
}

// validAnyOfPins chooses one nonempty parent state for an untyped local target.
func (builder *planBuilder) validAnyOfPins(
	node *schemaNode,
	occurrence schemaOccurrence,
	inherited []applicabilityPin,
) ([]applicabilityPin, bool, error) {
	if len(node.anyOf) == 0 {
		return appendPlanPins(inherited), true, nil
	}

	mask, realizable, err := anyOfMaskForAny(node)
	if err != nil {
		return nil, false, err
	}

	if !realizable {
		return nil, false, nil
	}

	return appendPlanPins(inherited, anyOfMaskPins(occurrence, len(node.anyOf), mask)...), true, nil
}

// faultPinsForAny chooses one nonempty parent state for a fault outside this node.
func (builder *planBuilder) faultPinsForAny(
	inherited []applicabilityPin,
	node *schemaNode,
	occurrence schemaOccurrence,
) ([]applicabilityPin, bool, error) {
	return builder.validAnyOfPins(node, occurrence, inherited)
}

// faultPinsForType chooses an anyOf state for a wrong-kind type fault.
func (builder *planBuilder) faultPinsForType(
	inherited []applicabilityPin,
	node *schemaNode,
	occurrence schemaOccurrence,
) ([]applicabilityPin, bool, error) {
	pins := appendPlanPins(inherited)
	if node == nil || len(node.anyOf) == 0 {
		return pins, true, nil
	}

	mask, realizable, err := anyOfMaskForTypeFault(node, occurrence)
	if err != nil {
		return nil, false, err
	}

	if !realizable {
		return nil, false, nil
	}

	return appendPlanPins(pins, anyOfMaskPins(occurrence, len(node.anyOf), mask)...), true, nil
}

// faultPinsForRule chooses an anyOf state for one local scalar fault.
func (builder *planBuilder) faultPinsForRule(
	inherited []applicabilityPin,
	node *schemaNode,
	occurrence schemaOccurrence,
	kind jsonKind,
	rule string,
) ([]applicabilityPin, bool, error) {
	pins := appendPlanPins(inherited, kindPin(occurrence, kind))
	if node == nil || len(node.anyOf) == 0 {
		return pins, true, nil
	}

	mask, realizable, err := anyOfMaskForFaultRule(node, occurrence, kind, rule)
	if err != nil {
		return nil, false, err
	}

	if !realizable {
		return nil, false, nil
	}

	return appendPlanPins(pins, anyOfMaskPins(occurrence, len(node.anyOf), mask)...), true, nil
}

// faultPinsForRequired chooses an anyOf state while omitting one required member.
func (builder *planBuilder) faultPinsForRequired(
	inherited []applicabilityPin,
	node *schemaNode,
	occurrence schemaOccurrence,
	name string,
) ([]applicabilityPin, bool, error) {
	pins := appendPlanPins(inherited, kindPin(occurrence, jsonObject))

	if node == nil {
		return nil, false, errors.New("schema occurrence has no shape")
	}

	if len(node.anyOf) == 0 {
		return pins, true, nil
	}

	mask, realizable, err := anyOfMaskForRequiredFault(node, occurrence, name)
	if err != nil {
		return nil, false, err
	}

	if !realizable {
		return nil, false, nil
	}

	return appendPlanPins(pins, anyOfMaskPins(occurrence, len(node.anyOf), mask)...), true, nil
}

// faultPinsForAdditional chooses an anyOf state for one undeclared member.
func (builder *planBuilder) faultPinsForAdditional(
	inherited []applicabilityPin,
	node *schemaNode,
	occurrence schemaOccurrence,
) ([]applicabilityPin, bool, error) {
	pins := appendPlanPins(inherited, kindPin(occurrence, jsonObject))

	if node == nil {
		return nil, false, errors.New("schema occurrence has no shape")
	}

	if len(node.anyOf) == 0 {
		return pins, true, nil
	}

	mask, realizable, err := anyOfMaskForAdditionalFault(node, occurrence)
	if err != nil {
		return nil, false, err
	}

	if !realizable {
		return nil, false, nil
	}

	return appendPlanPins(pins, anyOfMaskPins(occurrence, len(node.anyOf), mask)...), true, nil
}

// faultPinsForEnum chooses an anyOf state while allowing this node's enum to fail.
func (builder *planBuilder) faultPinsForEnum(
	inherited []applicabilityPin,
	node *schemaNode,
	occurrence schemaOccurrence,
) ([]applicabilityPin, bool, error) {
	pins := appendPlanPins(inherited)

	if node == nil {
		return nil, false, errors.New("schema occurrence has no shape")
	}

	if faultKind, hasKind := enumFaultKind(node); hasKind {
		pins = appendPlanPins(pins, kindPin(occurrence, faultKind))
	}

	if len(node.anyOf) == 0 {
		return pins, true, nil
	}

	mask, faultKind, realizable, err := anyOfMaskForEnumFaultWithKind(node)
	if err != nil {
		return nil, false, err
	}

	if !realizable {
		return nil, false, nil
	}

	pins = appendPlanPins(pins, kindPin(occurrence, faultKind))

	return appendPlanPins(pins, anyOfMaskPins(occurrence, len(node.anyOf), mask)...), true, nil
}

// anyOfMaskForTypeFault chooses a branch mask for a wrong-kind witness.
//
//nolint:cyclop // Wrong-kind candidates and structural fallback are one ordered search.
func anyOfMaskForTypeFault(node *schemaNode, occurrence schemaOccurrence) (*big.Int, bool, error) {
	withoutType := schemaNodeWithoutLocalRule(node, oracleRuleType)
	identity := makeRuleIdentity(occurrence, oracleRuleType)

	for _, kind := range canonicalJSONKinds() {
		if !typeFaultCanUseKind(node, kind) {
			continue
		}

		witnesses, err := canonicalAnyOfWitnesses(withoutType, kind)
		if err != nil {
			return nil, false, err
		}

		for _, witness := range witnesses {
			base := evaluateNode(withoutType, witness, occurrence)
			if base.err != nil {
				return nil, false, fmt.Errorf("evaluate type-fault witness: %w", base.err)
			}

			if !base.valid {
				continue
			}

			actual := evaluateNode(node, witness, occurrence)
			if actual.err != nil {
				return nil, false, fmt.Errorf("evaluate type-fault candidate: %w", actual.err)
			}

			if !containsFailureIdentity(actual.failures, identity) {
				continue
			}

			mask, exists := anyOfEvaluationMask(base, occurrence)
			if exists {
				return mask, true, nil
			}
		}
	}

	for _, kind := range canonicalJSONKinds() {
		if !typeFaultCanUseKind(node, kind) {
			continue
		}

		mask, realizable, err := structuralAnyOfMaskForKind(node, kind)
		if err != nil {
			return nil, false, err
		}

		if realizable {
			return mask, true, nil
		}
	}

	return nil, false, nil
}

// typeFaultCanUseKind reports whether one JSON kind can fail an explicit type.
func typeFaultCanUseKind(node *schemaNode, kind jsonKind) bool {
	if node.kind == schemaInteger && kind == jsonNumber {
		return true
	}

	return !nodeAcceptsKindForTarget(node, kind)
}

// anyOfMaskForFaultRule chooses a branch mask for a directed local scalar fault.
func anyOfMaskForFaultRule(
	node *schemaNode,
	occurrence schemaOccurrence,
	kind jsonKind,
	rule string,
) (*big.Int, bool, error) {
	withoutRule := schemaNodeWithoutLocalRule(node, rule)

	witnesses, err := canonicalAnyOfWitnesses(withoutRule, kind)
	if err != nil {
		return nil, false, err
	}

	identity := makeRuleIdentity(occurrence, rule)
	for _, witness := range witnesses {
		base := evaluateNode(withoutRule, witness, occurrence)
		if base.err != nil {
			return nil, false, fmt.Errorf("evaluate %s-fault witness: %w", rule, base.err)
		}

		if !base.valid {
			continue
		}

		actual := evaluateNode(node, witness, occurrence)
		if actual.err != nil {
			return nil, false, fmt.Errorf("evaluate %s-fault candidate: %w", rule, actual.err)
		}

		if !containsFailureIdentity(actual.failures, identity) {
			continue
		}

		mask, exists := anyOfEvaluationMask(base, occurrence)
		if exists {
			return mask, true, nil
		}
	}

	return structuralAnyOfMaskForKind(node, kind)
}

// anyOfMaskForAdditionalFault chooses a branch mask for an undeclared member.
func anyOfMaskForAdditionalFault(node *schemaNode, occurrence schemaOccurrence) (*big.Int, bool, error) {
	withoutAdditional := schemaNodeWithoutAdditional(node)

	witnesses, err := canonicalAnyOfWitnesses(withoutAdditional, jsonObject)
	if err != nil {
		return nil, false, err
	}

	extra := "__schematest_extra__"
	identity := makeRuleIdentity(appendObjectMemberOccurrence(occurrence, extra), oracleRuleAdditionalProperties)

	for _, witness := range witnesses {
		if witness == nil || witness.kind != jsonObject {
			continue
		}

		candidate := copyObjectWithMember(witness, extra, &jsonValue{kind: jsonBoolean})

		base := evaluateNode(withoutAdditional, candidate, occurrence)
		if base.err != nil {
			return nil, false, fmt.Errorf("evaluate additional-property fault witness: %w", base.err)
		}

		if !base.valid {
			continue
		}

		actual := evaluateNode(node, candidate, occurrence)
		if actual.err != nil {
			return nil, false, fmt.Errorf("evaluate additional-property fault candidate: %w", actual.err)
		}

		if !containsFailureRuleAtUse(actual.failures, identity) {
			continue
		}

		mask, exists := anyOfEvaluationMask(base, occurrence)
		if exists {
			return mask, true, nil
		}
	}

	return structuralAnyOfMaskForKind(node, jsonObject)
}

// schemaNodeWithoutAdditional allows one undeclared member without changing children.
func schemaNodeWithoutAdditional(node *schemaNode) *schemaNode {
	shape := *node.schemaShape
	shape.allowAdditionalProperties = true
	shape.additionalProperties = nil

	return &schemaNode{schemaShape: &shape, occurrence: node.occurrence}
}

// copyObjectWithMember copies one object witness and adds a concrete member.
func copyObjectWithMember(witness *jsonValue, name string, value *jsonValue) *jsonValue {
	members := make(map[string]*jsonValue, len(witness.object)+1)
	for key, member := range witness.object {
		members[key] = member
	}

	members[name] = value

	return &jsonValue{kind: jsonObject, object: members}
}

// containsFailureRuleAtUse matches a wildcard planner identity to one concrete member.
func containsFailureRuleAtUse(failures []failureIdentity, wanted failureIdentity) bool {
	for _, failure := range failures {
		if failure.rule == wanted.rule && failure.occurrence.usePointer == wanted.occurrence.usePointer {
			return true
		}
	}

	return false
}

// anyOfMaskForRequiredFault chooses a branch mask for an omitted member.
//
//nolint:cyclop // Candidate filtering and exact closure checks are one ordered search.
func anyOfMaskForRequiredFault(node *schemaNode, occurrence schemaOccurrence, name string) (*big.Int, bool, error) {
	withoutRequired := schemaNodeWithoutRequired(node, name)

	witnesses, err := canonicalAnyOfWitnesses(withoutRequired, jsonObject)
	if err != nil {
		return nil, false, err
	}

	identity := makeRuleIdentity(appendObjectMemberOccurrence(occurrence, name), oracleRuleRequired)

	for _, witness := range witnesses {
		if witness == nil || witness.kind != jsonObject {
			continue
		}

		if _, present := witness.object[name]; present {
			continue
		}

		base := evaluateNode(withoutRequired, witness, occurrence)
		if base.err != nil {
			return nil, false, fmt.Errorf("evaluate required-fault witness: %w", base.err)
		}

		if !base.valid {
			continue
		}

		actual := evaluateNode(node, witness, occurrence)
		if actual.err != nil {
			return nil, false, fmt.Errorf("evaluate required-fault candidate: %w", actual.err)
		}

		if !containsFailureIdentity(actual.failures, identity) {
			continue
		}

		mask, exists := anyOfEvaluationMask(base, occurrence)
		if exists {
			return mask, true, nil
		}
	}

	return structuralAnyOfMaskForKind(node, jsonObject)
}

// schemaNodeWithoutRequired removes one required member without changing children.
func schemaNodeWithoutRequired(node *schemaNode, name string) *schemaNode {
	shape := *node.schemaShape

	shape.required = make([]string, 0, len(node.required))
	for _, required := range node.required {
		if required != name {
			shape.required = append(shape.required, required)
		}
	}

	return &schemaNode{schemaShape: &shape, occurrence: node.occurrence}
}

// schemaNodeWithoutLocalRule disables one local rule without changing children.
//
//nolint:cyclop // Every supported local rule has one explicit clone operation.
func schemaNodeWithoutLocalRule(node *schemaNode, rule string) *schemaNode {
	shape := *node.schemaShape

	switch rule {
	case oracleRuleType:
		shape.kind = schemaAny
		shape.nullable = false
	case oracleRuleEnum:
		shape.enum = nil
		shape.enumIndices = nil
	case oracleRuleMinimum, oracleRuleExclusiveMinimum:
		shape.minimum = nil
		shape.exclusiveMinimum = false
	case oracleRuleMaximum, oracleRuleExclusiveMaximum:
		shape.maximum = nil
		shape.exclusiveMaximum = false
	case oracleRuleMultipleOf:
		shape.multipleOf = nil
	case oracleRuleFormat:
		shape.format = schemaFormatNone
	case oracleRuleMinLength:
		shape.minLength = nil
	case oracleRuleMaxLength:
		shape.maxLength = nil
	case oracleRulePattern:
		shape.pattern = nil
	case oracleRuleMinItems:
		shape.minItems = nil
	case oracleRuleMaxItems:
		shape.maxItems = nil
	case oracleRuleMinProperties:
		shape.minProperties = nil
	case oracleRuleMaxProperties:
		shape.maxProperties = nil
	}

	return &schemaNode{schemaShape: &shape, occurrence: node.occurrence}
}

// containsFailureIdentity reports whether one evaluation contains a failure identity.
func containsFailureIdentity(failures []failureIdentity, wanted failureIdentity) bool {
	for _, failure := range failures {
		if failure == wanted {
			return true
		}
	}

	return false
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
			if member != nil && member.kind != jsonNull && allowed[member.kind] {
				return member.kind, true
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

// enumExhaustsNonNullableBoolean reports whether enum leaves no valid boolean value.
func enumExhaustsNonNullableBoolean(node *schemaNode) bool {
	if node.kind != schemaBoolean || node.nullable || node.enum == nil {
		return false
	}

	seenTrue := false
	seenFalse := false

	for _, member := range node.enum {
		if member == nil || member.kind != jsonBoolean {
			continue
		}

		if member.boolean {
			seenTrue = true
		} else {
			seenFalse = true
		}
	}

	return seenTrue && seenFalse
}

// branchKindMatches reports whether one branch type admits the requested JSON kind.
func branchKindMatches(node *schemaNode, kind jsonKind) bool {
	if node.kind == schemaAny {
		return true
	}

	if kind == jsonNull {
		return node.nullable
	}

	return schemaNodeJSONKind(node.kind) == kind
}

// branchEnumAcceptsKind reports whether one branch enum contains the requested kind.
func branchEnumAcceptsKind(node *schemaNode, kind jsonKind) (bool, error) {
	if node.enum == nil {
		return true, nil
	}

	for _, member := range node.enum {
		if member == nil {
			return false, errors.New("JSON enum member is nil")
		}

		matches, err := valueMatchesNodeKind(member, node.kind, node.nullable)
		if err != nil {
			return false, err
		}

		if matches && member.kind == kind {
			return true, nil
		}
	}

	return false, nil
}

// branchAllOfAcceptsKind reports whether every allOf child admits the requested kind.
func branchAllOfAcceptsKind(node *schemaNode, kind jsonKind, visiting map[*schemaNode]bool) (bool, error) {
	for _, child := range node.allOf {
		accepted, err := branchCanAcceptKind(child, kind, visiting)
		if err != nil {
			return false, err
		}

		if !accepted {
			return false, nil
		}
	}

	return true, nil
}

// branchAnyOfAcceptsKind reports whether at least one anyOf child admits the requested kind.
func branchAnyOfAcceptsKind(node *schemaNode, kind jsonKind, visiting map[*schemaNode]bool) (bool, error) {
	if len(node.anyOf) == 0 {
		return true, nil
	}

	for _, child := range node.anyOf {
		accepted, err := branchCanAcceptKind(child, kind, visiting)
		if err != nil {
			return false, err
		}

		if accepted {
			return true, nil
		}
	}

	return false, nil
}

// branchCanAcceptKind reports whether an anyOf branch can admit one JSON kind.
func branchCanAcceptKind(node *schemaNode, kind jsonKind, visiting map[*schemaNode]bool) (bool, error) {
	if node == nil || node.schemaShape == nil {
		return false, errors.New("anyOf branch has no shape")
	}

	if visiting[node] {
		return false, fmt.Errorf("recursive anyOf branch at %s", node.occurrence.usePointer)
	}

	visiting[node] = true
	defer delete(visiting, node)

	if !branchKindMatches(node, kind) {
		return false, nil
	}

	accepted, err := branchEnumAcceptsKind(node, kind)
	if err != nil || !accepted {
		return accepted, err
	}

	accepted, err = branchAllOfAcceptsKind(node, kind, visiting)
	if err != nil || !accepted {
		return accepted, err
	}

	return branchAnyOfAcceptsKind(node, kind, visiting)
}

// anyOfAlwaysAcceptsKind reports whether one nested anyOf cannot fail for a kind.
func anyOfAlwaysAcceptsKind(node *schemaNode, kind jsonKind, visiting map[*schemaNode]bool) (bool, error) {
	for _, child := range node.anyOf {
		always, err := branchAlwaysAcceptsKind(child, kind, visiting)
		if err != nil {
			return false, err
		}

		if always {
			return true, nil
		}
	}

	return false, nil
}

// branchAlwaysAcceptsKind reports whether a branch cannot fail for one JSON kind.
func branchAlwaysAcceptsKind(node *schemaNode, kind jsonKind, visiting map[*schemaNode]bool) (bool, error) {
	accepted, err := branchCanAcceptKind(node, kind, visiting)
	if err != nil || !accepted {
		return false, err
	}

	if node.enum != nil || len(node.allOf) > 0 {
		return false, nil
	}

	if len(node.anyOf) > 0 {
		return anyOfAlwaysAcceptsKind(node, kind, visiting)
	}

	if node.kind == schemaInteger && kind == jsonNumber {
		return false, nil
	}

	return !nodeHasSiblingRuleForKind(node, kind), nil
}

// anyOfMaskForKind chooses one realizable truth mask for a JSON kind.
func anyOfMaskForKind(node *schemaNode, kind jsonKind) (*big.Int, bool, error) {
	if node == nil || len(node.anyOf) == 0 {
		return nil, false, errors.New("schema has no anyOf branches")
	}

	exactMasks, err := realizableAnyOfMasksForKind(node, kind)
	if err != nil {
		return nil, false, err
	}

	if len(exactMasks) > 0 {
		return exactMasks[0], true, nil
	}

	return structuralAnyOfMaskForKind(node, kind)
}

// structuralAnyOfMaskForKind chooses a nonempty mask from branch kind applicability.
func structuralAnyOfMaskForKind(node *schemaNode, kind jsonKind) (*big.Int, bool, error) {
	mask := new(big.Int)

	applicable := make([]int, 0, len(node.anyOf))
	for index, child := range node.anyOf {
		accepted, err := branchCanAcceptKind(child, kind, make(map[*schemaNode]bool))
		if err != nil {
			return nil, false, err
		}

		if !accepted {
			continue
		}

		applicable = append(applicable, index)

		always, err := branchAlwaysAcceptsKind(child, kind, make(map[*schemaNode]bool))
		if err != nil {
			return nil, false, err
		}

		if always {
			mask.SetBit(mask, index, 1)
		}
	}

	if len(applicable) == 0 {
		return nil, false, nil
	}

	if mask.Sign() == 0 {
		mask.SetBit(mask, applicable[0], 1)
	}

	return mask, true, nil
}

// branchCanAcceptInteger reports whether a branch can admit an integer JSON number.
//
//nolint:cyclop,gocognit,nestif // Numeric subtype applicability follows composition recursively.
func branchCanAcceptInteger(node *schemaNode, visiting map[*schemaNode]bool) (bool, error) {
	if node == nil || node.schemaShape == nil {
		return false, errors.New("anyOf branch has no shape")
	}

	if visiting[node] {
		return false, fmt.Errorf("recursive anyOf branch at %s", node.occurrence.usePointer)
	}

	visiting[node] = true
	defer delete(visiting, node)

	switch node.kind {
	case schemaAny, schemaInteger, schemaNumber:
	default:
		return false, nil
	}

	if node.enum != nil {
		matched := false

		for _, member := range node.enum {
			if member == nil {
				return false, errors.New("JSON enum member is nil")
			}

			if member.kind != jsonNumber {
				continue
			}

			integer, err := member.number.isInteger()
			if err != nil {
				return false, err
			}

			matches, err := valueMatchesNodeKind(member, node.kind, node.nullable)
			if err != nil {
				return false, err
			}

			if integer && matches {
				matched = true

				break
			}
		}

		if !matched {
			return false, nil
		}
	}

	for _, child := range node.allOf {
		accepted, err := branchCanAcceptInteger(child, visiting)
		if err != nil {
			return false, err
		}

		if !accepted {
			return false, nil
		}
	}

	if len(node.anyOf) == 0 {
		return true, nil
	}

	for _, child := range node.anyOf {
		accepted, err := branchCanAcceptInteger(child, visiting)
		if err != nil {
			return false, err
		}

		if accepted {
			return true, nil
		}
	}

	return false, nil
}

// branchAlwaysAcceptsInteger reports whether a branch always accepts integers.
func branchAlwaysAcceptsInteger(node *schemaNode, visiting map[*schemaNode]bool) (bool, error) {
	accepted, err := branchCanAcceptInteger(node, visiting)
	if err != nil || !accepted {
		return false, err
	}

	if node.enum != nil || len(node.allOf) > 0 {
		return false, nil
	}

	if len(node.anyOf) > 0 {
		for _, child := range node.anyOf {
			always, err := branchAlwaysAcceptsInteger(child, visiting)
			if err != nil {
				return false, err
			}

			if always {
				return true, nil
			}
		}

		return false, nil
	}

	return !nodeHasSiblingRuleForKind(node, jsonNumber), nil
}

// integerAnyOfMask chooses the canonical truth mask for integer JSON numbers.
//
//nolint:cyclop // Integer subtype applicability follows the same branch decision phases.
func integerAnyOfMask(node *schemaNode) (*big.Int, bool, error) {
	if node == nil || len(node.anyOf) == 0 {
		return nil, false, nil
	}

	mask := new(big.Int)
	firstApplicable := -1

	for index, child := range node.anyOf {
		accepted, err := branchCanAcceptInteger(child, make(map[*schemaNode]bool))
		if err != nil {
			return nil, false, err
		}

		if !accepted {
			continue
		}

		if firstApplicable < 0 {
			firstApplicable = index
		}

		always, err := branchAlwaysAcceptsInteger(child, make(map[*schemaNode]bool))
		if err != nil {
			return nil, false, err
		}

		if always {
			mask.SetBit(mask, index, 1)
		}
	}

	if firstApplicable < 0 {
		return nil, false, nil
	}

	if mask.Sign() == 0 {
		mask.SetBit(mask, firstApplicable, 1)
	}

	return mask, true, nil
}

// anyOfMaskForAny chooses the first realizable mask in canonical JSON-kind order.
func anyOfMaskForAny(node *schemaNode) (*big.Int, bool, error) {
	for _, kind := range canonicalJSONKinds() {
		mask, realizable, err := anyOfMaskForKind(node, kind)
		if err != nil {
			return nil, false, err
		}

		if realizable {
			return mask, true, nil
		}
	}

	return nil, false, nil
}

// anyOfMaskForEnumFault keeps the enum-fault mask helper's compact private seam.
func anyOfMaskForEnumFault(node *schemaNode) (*big.Int, bool, error) {
	mask, _, realizable, err := anyOfMaskForEnumFaultWithKind(node)

	return mask, realizable, err
}

// anyOfMaskForEnumFaultWithKind chooses a mask and witness kind for an enum fault.
//
//nolint:cyclop // Enum-fault witness filtering has explicit canonical phases.
func anyOfMaskForEnumFaultWithKind(node *schemaNode) (*big.Int, jsonKind, bool, error) {
	if node == nil || len(node.anyOf) == 0 {
		return nil, jsonNull, false, errors.New("schema has no anyOf branches")
	}

	withoutEnum := schemaNodeWithoutEnum(node)
	for _, kind := range enumFaultKinds(node) {
		witnesses, err := canonicalAnyOfWitnesses(node, kind)
		if err != nil {
			return nil, jsonNull, false, err
		}

		for _, witness := range witnesses {
			matches, err := enumContainsValue(node, witness)
			if err != nil {
				return nil, jsonNull, false, err
			}

			if matches {
				continue
			}

			result := evaluateNode(withoutEnum, witness, node.occurrence)
			if result.err != nil {
				return nil, jsonNull, false, fmt.Errorf("evaluate enum-fault anyOf %s witness: %w", jsonKindName(kind), result.err)
			}

			if !result.valid {
				continue
			}

			mask, exists := anyOfEvaluationMask(result, node.occurrence)
			if exists {
				return mask, kind, true, nil
			}
		}
	}

	return nil, jsonNull, false, nil
}

// enumFaultKind chooses the simplest kind for an isolated enum fault.
func enumFaultKind(node *schemaNode) (jsonKind, bool) {
	if node.kind == schemaAny {
		return jsonNull, false
	}

	if node.kind == schemaBoolean && node.nullable && enumContainsBothBooleans(node) {
		return jsonNull, true
	}

	return schemaNodeJSONKind(node.kind), true
}

// enumContainsBothBooleans reports whether both non-null boolean values are authored.
func enumContainsBothBooleans(node *schemaNode) bool {
	seenTrue := false
	seenFalse := false

	for _, member := range node.enum {
		if member == nil || member.kind != jsonBoolean {
			continue
		}

		if member.boolean {
			seenTrue = true
		} else {
			seenFalse = true
		}
	}

	return seenTrue && seenFalse
}

// enumFaultKinds returns kinds that can fail enum without also failing type.
func enumFaultKinds(node *schemaNode) []jsonKind {
	if node.kind == schemaAny {
		return canonicalJSONKinds()
	}

	kind := schemaNodeJSONKind(node.kind)

	result := []jsonKind{kind}
	if node.nullable {
		result = append(result, jsonNull)
	}

	return result
}

// schemaNodeWithoutEnum shallow-copies one node while disabling only its enum.
func schemaNodeWithoutEnum(node *schemaNode) *schemaNode {
	shape := *node.schemaShape
	shape.enum = nil
	shape.enumIndices = nil

	return &schemaNode{schemaShape: &shape, occurrence: node.occurrence}
}

// enumContainsValue reports whether one value is semantically listed by an enum.
func enumContainsValue(node *schemaNode, value *jsonValue) (bool, error) {
	for _, member := range node.enum {
		equal, err := jsonSemanticEqual(member, value)
		if err != nil {
			return false, err
		}

		if equal {
			return true, nil
		}
	}

	return false, nil
}

// realizableAnyOfMasks returns distinct truth masks reachable by complete-parent witnesses.
func realizableAnyOfMasks(node *schemaNode) ([]*big.Int, error) {
	if node == nil || len(node.anyOf) == 0 {
		return nil, errors.New("schema has no anyOf branches")
	}

	masks := make([]*big.Int, 0)

	for _, kind := range canonicalJSONKinds() {
		kindMasks, err := realizableAnyOfMasksForKind(node, kind)
		if err != nil {
			return nil, err
		}

		for _, mask := range kindMasks {
			masks = appendUniqueAnyOfMask(masks, mask)
		}
	}

	sort.Slice(masks, func(left, right int) bool {
		return masks[left].Cmp(masks[right]) < 0
	})

	return masks, nil
}

// realizableAnyOfMasksForKind evaluates every canonical witness for one JSON kind.
func realizableAnyOfMasksForKind(node *schemaNode, kind jsonKind) ([]*big.Int, error) {
	if node == nil || len(node.anyOf) == 0 {
		return nil, errors.New("schema has no anyOf branches")
	}

	if !nodeAcceptsKindForTarget(node, kind) {
		return nil, nil
	}

	witnesses, err := canonicalAnyOfWitnesses(node, kind)
	if err != nil {
		return nil, err
	}

	masks := make([]*big.Int, 0)

	for _, witness := range witnesses {
		result := evaluateNode(node, witness, node.occurrence)
		if result.err != nil {
			return nil, fmt.Errorf("evaluate anyOf %s witness: %w", jsonKindName(kind), result.err)
		}

		if !result.valid {
			continue
		}

		mask, exists := anyOfEvaluationMask(result, node.occurrence)
		if !exists {
			return nil, errors.New("valid anyOf witness has no parent truth vector")
		}

		masks = appendUniqueAnyOfMask(masks, mask)
	}

	sort.Slice(masks, func(left, right int) bool {
		return masks[left].Cmp(masks[right]) < 0
	})

	return masks, nil
}

// anyOfEvaluationMask extracts one parent anyOf truth vector as a low-bit mask.
func anyOfEvaluationMask(result evaluation, occurrence schemaOccurrence) (*big.Int, bool) {
	identity := makeRuleIdentity(occurrence, oracleRuleAnyOf)
	for _, truth := range result.anyOf {
		if truth.ruleIdentity != identity {
			continue
		}

		mask := new(big.Int)

		for index, branch := range truth.branches {
			if branch {
				mask.SetBit(mask, index, 1)
			}
		}

		return mask, mask.Sign() != 0
	}

	return nil, false
}

// appendUniqueAnyOfMask keeps one copy of a numeric truth mask.
func appendUniqueAnyOfMask(masks []*big.Int, candidate *big.Int) []*big.Int {
	for _, existing := range masks {
		if existing.Cmp(candidate) == 0 {
			return masks
		}
	}

	return append(masks, candidate)
}

// canonicalAnyOfWitnesses returns authored and simple canonical values for one kind.
func canonicalAnyOfWitnesses(node *schemaNode, kind jsonKind) ([]*jsonValue, error) {
	witnesses := make([]*jsonValue, 0)
	if err := collectAnyOfWitnesses(node, kind, make(map[*schemaNode]bool), &witnesses); err != nil {
		return nil, err
	}

	canonical, err := canonicalKindWitnesses(kind)
	if err != nil {
		return nil, err
	}

	for _, witness := range canonical {
		var appendErr error

		witnesses, appendErr = appendUniqueJSONWitness(witnesses, witness)
		if appendErr != nil {
			return nil, appendErr
		}
	}

	return witnesses, nil
}

// collectAnyOfWitnesses collects values authored by the parent or its compositions.
//
//nolint:cyclop,gocognit,nestif // One recursive pass collects all complete-parent witness sources.
func collectAnyOfWitnesses(
	node *schemaNode,
	kind jsonKind,
	visiting map[*schemaNode]bool,
	witnesses *[]*jsonValue,
) error {
	if node == nil || node.schemaShape == nil {
		return errors.New("anyOf branch has no shape")
	}

	if visiting[node] {
		return fmt.Errorf("recursive anyOf witness schema at %s", node.occurrence.usePointer)
	}

	visiting[node] = true
	defer delete(visiting, node)

	for _, member := range node.enum {
		if member == nil {
			return errors.New("JSON enum member is nil")
		}

		if member.kind == kind {
			var err error

			*witnesses, err = appendUniqueJSONWitness(*witnesses, member)
			if err != nil {
				return err
			}
		}
	}

	if node.defaultValue != nil && node.defaultValue.kind == kind {
		var err error

		*witnesses, err = appendUniqueJSONWitness(*witnesses, node.defaultValue)
		if err != nil {
			return err
		}
	}

	if kind == jsonString {
		minimumWitnesses, err := canonicalStringMinLengthWitness(node.minLength)
		if err != nil {
			return err
		}

		for _, witness := range minimumWitnesses {
			*witnesses, err = appendUniqueJSONWitness(*witnesses, witness)
			if err != nil {
				return err
			}
		}
	}

	if kind == jsonNumber {
		for _, bound := range []*exactNumber{node.minimum, node.maximum, node.multipleOf} {
			if bound == nil {
				continue
			}

			value := &jsonValue{kind: jsonNumber, number: bound}

			var appendErr error

			*witnesses, appendErr = appendUniqueJSONWitness(*witnesses, value)
			if appendErr != nil {
				return appendErr
			}
		}

		for _, candidateSpec := range []struct {
			bound *exactNumber
			delta int64
		}{
			{bound: node.minimum, delta: 1},
			{bound: node.maximum, delta: -1},
		} {
			candidate, exists, shiftErr := shiftedExactNumber(candidateSpec.bound, candidateSpec.delta)
			if shiftErr != nil {
				return shiftErr
			}

			if !exists {
				continue
			}

			var appendErr error

			*witnesses, appendErr = appendUniqueJSONWitness(
				*witnesses,
				&jsonValue{kind: jsonNumber, number: candidate},
			)
			if appendErr != nil {
				return appendErr
			}
		}
	}

	for _, child := range node.allOf {
		if err := collectAnyOfWitnesses(child, kind, visiting, witnesses); err != nil {
			return err
		}
	}

	for _, child := range node.anyOf {
		if err := collectAnyOfWitnesses(child, kind, visiting, witnesses); err != nil {
			return err
		}
	}

	return nil
}

const (
	// maxPlanStringWitnessLength bounds transient planner witness allocation.
	maxPlanStringWitnessLength uint64 = 4096
	// maxPlanNumberExponent bounds transient decimal-power expansion.
	maxPlanNumberExponent uint64 = 4096
)

// shiftedExactNumber returns one exact decimal value at an integer offset.
func shiftedExactNumber(number *exactNumber, delta int64) (*exactNumber, bool, error) {
	if number == nil {
		return nil, false, nil
	}

	if err := number.validate(); err != nil {
		return nil, false, err
	}

	if !number.exponent.IsInt64() || number.exponent.BitLen() > 13 {
		return nil, false, nil
	}

	exponent := number.exponent.Int64()

	absoluteExponent := exponent
	if absoluteExponent < 0 {
		absoluteExponent = -absoluteExponent
	}

	if uint64(absoluteExponent) > maxPlanNumberExponent {
		return nil, false, nil
	}

	numerator := new(big.Int).Set(number.numerator)
	denominator := new(big.Int).Set(number.denominator)

	if exponent >= 0 {
		numerator.Mul(numerator, integerPower(decimalRadix, uint64(exponent)))
	} else {
		denominator.Mul(denominator, integerPower(decimalRadix, uint64(-exponent)))
	}

	numerator.Add(numerator, new(big.Int).Mul(big.NewInt(delta), denominator))

	value, err := newExactRational(numerator, denominator)
	if err != nil {
		return nil, false, err
	}

	return value, true, nil
}

// canonicalStringMinLengthWitness derives a bounded valid-length string witness.
func canonicalStringMinLengthWitness(minimum *exactCount) ([]*jsonValue, error) {
	if minimum == nil || minimum.number == nil {
		return []*jsonValue{}, nil
	}

	length, fits, err := exactCountUint64(minimum)
	if err != nil {
		return nil, err
	}

	if !fits || length > maxPlanStringWitnessLength {
		return []*jsonValue{}, nil
	}

	maxInt := uint64(^uint(0) >> 1)
	if length > maxInt {
		return nil, fmt.Errorf("canonicalize minLength witness: length %d overflows int", length)
	}

	return []*jsonValue{{kind: jsonString, text: strings.Repeat("a", int(length))}}, nil
}

// exactCountUint64 converts a non-negative integral count without parsing a display lexeme.
//
//nolint:cyclop // Exact representation validation and bounded conversion are one phase.
func exactCountUint64(count *exactCount) (uint64, bool, error) {
	if count == nil || count.number == nil {
		return 0, false, nil
	}

	integer, err := count.number.isInteger()
	if err != nil {
		return 0, false, err
	}

	if !integer || count.number.numerator.Sign() < 0 {
		return 0, false, nil
	}

	if count.number.exponent.Sign() > 0 {
		if !count.number.exponent.IsUint64() || count.number.exponent.Uint64() > 20 {
			return 0, false, nil
		}
	}

	coefficient, exponent, err := count.number.finiteDecimal()
	if err != nil {
		return 0, false, err
	}

	if coefficient.Sign() < 0 || exponent.Sign() < 0 || !exponent.IsUint64() || exponent.Uint64() > 20 {
		return 0, false, nil
	}

	value := new(big.Int).Set(coefficient)
	if exponent.Sign() > 0 {
		value.Mul(value, integerPower(decimalRadix, exponent.Uint64()))
	}

	if !value.IsUint64() {
		return 0, false, nil
	}

	return value.Uint64(), true, nil
}

// appendUniqueJSONWitness retains semantic distinctness without changing authored values.
func appendUniqueJSONWitness(witnesses []*jsonValue, candidate *jsonValue) ([]*jsonValue, error) {
	for _, existing := range witnesses {
		equal, err := jsonSemanticEqual(existing, candidate)
		if err != nil {
			return nil, err
		}

		if equal {
			return witnesses, nil
		}
	}

	return append(witnesses, candidate), nil
}

// canonicalKindWitnesses returns small deterministic values for each JSON kind.
//
//nolint:cyclop // Each JSON kind has an explicit canonical witness family.
func canonicalKindWitnesses(kind jsonKind) ([]*jsonValue, error) {
	switch kind {
	case jsonNull:
		return []*jsonValue{{kind: jsonNull}}, nil
	case jsonBoolean:
		return []*jsonValue{{kind: jsonBoolean}, {kind: jsonBoolean, boolean: true}}, nil
	case jsonNumber:
		sources := []string{"-1", "0", "0.5", "1", "2", "3"}
		values := make([]*jsonValue, 0, len(sources))

		for _, source := range sources {
			number, err := parseExactNumber(source)
			if err != nil {
				return nil, err
			}

			values = append(values, &jsonValue{kind: jsonNumber, number: number})
		}

		return values, nil
	case jsonString:
		return []*jsonValue{
			{kind: jsonString, text: ""},
			{kind: jsonString, text: "a"},
			{kind: jsonString, text: "b"},
			{kind: jsonString, text: "text"},
		}, nil
	case jsonArray:
		number, err := parseExactNumber("0")
		if err != nil {
			return nil, err
		}

		return []*jsonValue{
			{kind: jsonArray, array: []*jsonValue{}},
			{kind: jsonArray, array: []*jsonValue{{kind: jsonBoolean}}},
			{kind: jsonArray, array: []*jsonValue{{kind: jsonString, text: "a"}}},
			{kind: jsonArray, array: []*jsonValue{{kind: jsonNumber, number: number}}},
		}, nil
	case jsonObject:
		return []*jsonValue{
			{kind: jsonObject, object: map[string]*jsonValue{}},
			{kind: jsonObject, object: map[string]*jsonValue{
				"a": {kind: jsonString, text: "a"},
			}},
		}, nil
	default:
		return nil, fmt.Errorf("unknown JSON kind %d", kind)
	}
}

// canonicalJSONKinds returns the locked JSON kind order.
func canonicalJSONKinds() []jsonKind {
	return []jsonKind{jsonNull, jsonBoolean, jsonNumber, jsonString, jsonArray, jsonObject}
}

// memberKindPin pins an enum target to its member's JSON kind.
func memberKindPin(occurrence schemaOccurrence, value *jsonValue) applicabilityPin {
	return kindPin(occurrence, value.kind)
}

// kindPin pins one schema occurrence to a JSON kind.
func kindPin(occurrence schemaOccurrence, kind jsonKind) applicabilityPin {
	return applicabilityPin{occurrence: occurrence, kind: kind, hasKind: true}
}

// presencePin pins one child occurrence to present or absent.
func presencePin(occurrence schemaOccurrence, presence pinPresence) applicabilityPin {
	return applicabilityPin{occurrence: occurrence, presence: presence}
}

// allOfValidPins pins every allOf branch true.
func allOfValidPins(occurrence schemaOccurrence, count int) []applicabilityPin {
	return compositionPins(occurrence, "allOf", count, -1, true)
}

// allOfFaultPins pins one allOf branch false and all sibling branches true.
func allOfFaultPins(occurrence schemaOccurrence, count, selected int) []applicabilityPin {
	return compositionFaultPins(occurrence, "allOf", count, selected)
}

// anyOfValidPins pins the selected anyOf branch true without constraining siblings.
func anyOfValidPins(occurrence schemaOccurrence, selected int) []applicabilityPin {
	branchOccurrence := schemaOccurrence{
		usePointer:       occurrence.usePointer + "/anyOf/" + itoa(selected),
		targetPointer:    occurrence.targetPointer,
		instanceTemplate: occurrence.instanceTemplate,
	}

	return []applicabilityPin{{
		occurrence:  branchOccurrence,
		composition: "anyOf",
		branch:      selected,
		truth:       true,
		hasBranch:   true,
	}}
}

// anyOfFaultPins pins every authored anyOf branch false.
func anyOfFaultPins(occurrence schemaOccurrence, count int) []applicabilityPin {
	return compositionPins(occurrence, "anyOf", count, -1, false)
}

// compositionPins creates one truth pin for every branch.
func compositionPins(
	occurrence schemaOccurrence,
	composition string,
	count, selected int,
	truth bool,
) []applicabilityPin {
	pins := make([]applicabilityPin, 0, count)
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

		pins = append(pins, applicabilityPin{
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
func compositionFaultPins(occurrence schemaOccurrence, composition string, count, selected int) []applicabilityPin {
	pins := make([]applicabilityPin, 0, count)
	for index := 0; index < count; index++ {
		branchOccurrence := schemaOccurrence{
			usePointer:       occurrence.usePointer + "/" + composition + "/" + itoa(index),
			targetPointer:    occurrence.targetPointer,
			instanceTemplate: occurrence.instanceTemplate,
		}
		pins = append(pins, applicabilityPin{
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
func anyOfMaskPins(occurrence schemaOccurrence, count int, mask *big.Int) []applicabilityPin {
	pins := make([]applicabilityPin, 0, count)
	for index := 0; index < count; index++ {
		pins = append(pins, applicabilityPin{
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
func rebasePlanOccurrence(child *schemaNode, usePointer, instanceTemplate string) schemaOccurrence {
	occurrence := child.occurrence
	occurrence.usePointer = usePointer
	occurrence.instanceTemplate = instanceTemplate

	return occurrence
}

// appendPlanPins merges later pins over earlier pins for the same dimension.
func appendPlanPins(base []applicabilityPin, pins ...applicabilityPin) []applicabilityPin {
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
func samePlanPinOccurrence(left, right applicabilityPin) bool {
	return left.occurrence.usePointer == right.occurrence.usePointer &&
		left.occurrence.instanceTemplate == right.occurrence.instanceTemplate &&
		left.composition == right.composition &&
		left.hasBranch == right.hasBranch &&
		(!left.hasBranch || left.branch == right.branch)
}

// mergePlanPins lets an explicit later pin override one earlier dimension.
func mergePlanPins(left, right applicabilityPin) applicabilityPin {
	merged := left
	if right.hasKind {
		merged.occurrence = right.occurrence
		merged.kind = right.kind
		merged.hasKind = true
	}

	if right.presence != planPinNoPresence {
		merged.occurrence = right.occurrence
		merged.presence = right.presence
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
func copyPlanPins(pins []applicabilityPin) []applicabilityPin {
	if len(pins) == 0 {
		return nil
	}

	return append([]applicabilityPin(nil), pins...)
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

// sortedStrings returns a copied UTF-8 byte-ordered string list.
func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)

	return result
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

// canonicalPlanEnum deduplicates semantic enum values while keeping first-authored order.
func canonicalPlanEnum(values []*jsonValue) ([]*jsonValue, error) {
	members := make([]*jsonValue, 0, len(values))
	for _, candidate := range values {
		if candidate == nil {
			return nil, errors.New("JSON enum member is nil")
		}

		duplicate := false

		for _, existing := range members {
			equal, err := jsonSemanticEqual(candidate, existing)
			if err != nil {
				return nil, err
			}

			if equal {
				duplicate = true

				break
			}
		}

		if !duplicate {
			members = append(members, candidate)
		}
	}

	return members, nil
}

// firstCanonicalFault selects the first fault in canonical obligation order.
func firstCanonicalFault(faults []faultTarget) (faultTarget, bool, error) {
	if len(faults) == 0 {
		return faultTarget{}, false, nil
	}

	best := faults[0]
	for _, candidate := range faults[1:] {
		comparison, err := comparePlanObligations(candidate.obligation, best.obligation)
		if err != nil {
			return faultTarget{}, false, err
		}

		if comparison < 0 {
			best = candidate
		}
	}

	return best, true, nil
}

// realizableFaultCandidates returns canonical branch faults with reachable exact closures.
func realizableFaultCandidates(
	node *schemaNode,
	occurrence schemaOccurrence,
	faults []faultTarget,
) ([]faultTarget, error) {
	remaining := append([]faultTarget(nil), faults...)

	candidates := make([]faultTarget, 0, len(faults))
	for len(remaining) > 0 {
		candidate, exists, err := firstCanonicalFault(remaining)
		if err != nil {
			return nil, err
		}

		if !exists {
			break
		}

		if faultTargetIsRealizable(node, occurrence, candidate) {
			candidates = append(candidates, candidate)
		}

		removed := false

		for index, current := range remaining {
			if current.obligation == candidate.obligation {
				remaining = append(remaining[:index], remaining[index+1:]...)
				removed = true

				break
			}
		}

		if !removed {
			return nil, errors.New("canonical fault selection lost its candidate")
		}
	}

	return candidates, nil
}

// firstRealizableFault selects the first canonical fault with a reachable exact closure.
func firstRealizableFault(
	node *schemaNode,
	occurrence schemaOccurrence,
	faults []faultTarget,
) (faultTarget, bool, error) {
	candidates, err := realizableFaultCandidates(node, occurrence, faults)
	if err != nil {
		return faultTarget{}, false, err
	}

	if len(candidates) == 0 {
		return faultTarget{}, false, nil
	}

	return candidates[0], true, nil
}

// selectAnyOfRepresentatives chooses compatible representatives for every branch.
func selectAnyOfRepresentatives(branches []anyOfBranchPlan) ([]faultTarget, bool) {
	return selectAnyOfRepresentativesAt(branches, 0, nil, nil)
}

// selectAnyOfRepresentativesForParent chooses representatives under one parent kind.
func selectAnyOfRepresentativesForParent(
	branches []anyOfBranchPlan,
	parent *schemaNode,
) ([]faultTarget, jsonKind, bool) {
	for _, kind := range canonicalJSONKinds() {
		if !nodeAcceptsKindForTarget(parent, kind) {
			continue
		}

		selected, exists := selectAnyOfRepresentativesAtKind(branches, kind, 0, nil, nil)
		if exists {
			return selected, kind, true
		}
	}

	return nil, jsonNull, false
}

// selectAnyOfRepresentativesAtKind backtracks over faults compatible with one kind.
func selectAnyOfRepresentativesAtKind(
	branches []anyOfBranchPlan,
	kind jsonKind,
	index int,
	selected []faultTarget,
	pins []applicabilityPin,
) ([]faultTarget, bool) {
	if index == len(branches) {
		return append([]faultTarget(nil), selected...), true
	}

	branch := branches[index]
	for _, candidate := range branch.representatives {
		if !faultCandidateSupportsKind(candidate, branch.node, branch.occurrence, kind) ||
			!planPinsCompatible(pins, candidate.pins) {
			continue
		}

		nextSelected := append(append([]faultTarget(nil), selected...), candidate)
		nextPins := appendPlanPins(pins, candidate.pins...)

		result, exists := selectAnyOfRepresentativesAtKind(
			branches, kind, index+1, nextSelected, nextPins,
		)
		if exists {
			return result, true
		}
	}

	return nil, false
}

// faultCandidateSupportsKind reports whether a branch fault can use one parent kind.
func faultCandidateSupportsKind(
	candidate faultTarget,
	branch *schemaNode,
	branchOccurrence schemaOccurrence,
	kind jsonKind,
) bool {
	for _, pin := range candidate.pins {
		if !pin.hasKind || pin.occurrence.usePointer != branchOccurrence.usePointer ||
			pin.occurrence.instanceTemplate != branchOccurrence.instanceTemplate {
			continue
		}

		return pin.kind == kind
	}

	if candidate.obligation.rule == oracleRuleType {
		return typeFaultCanUseKind(branch, kind)
	}

	return true
}

// selectAnyOfRepresentativesAt backtracks over canonical branch representatives.
func selectAnyOfRepresentativesAt(
	branches []anyOfBranchPlan,
	index int,
	selected []faultTarget,
	pins []applicabilityPin,
) ([]faultTarget, bool) {
	if index == len(branches) {
		return append([]faultTarget(nil), selected...), true
	}

	for _, candidate := range branches[index].representatives {
		if !planPinsCompatible(pins, candidate.pins) {
			continue
		}

		nextSelected := append(append([]faultTarget(nil), selected...), candidate)
		nextPins := appendPlanPins(pins, candidate.pins...)
		result, exists := selectAnyOfRepresentativesAt(branches, index+1, nextSelected, nextPins)

		if exists {
			return result, true
		}
	}

	return nil, false
}

// planPinsCompatible reports whether two pin sets can describe one instance value.
//
//nolint:cyclop // The three independent pin dimensions must be checked pairwise.
func planPinsCompatible(left, right []applicabilityPin) bool {
	for _, leftPin := range left {
		for _, rightPin := range right {
			if leftPin.occurrence.instanceTemplate != rightPin.occurrence.instanceTemplate {
				continue
			}

			if leftPin.hasKind && rightPin.hasKind && leftPin.kind != rightPin.kind {
				return false
			}

			if leftPin.presence != planPinNoPresence && rightPin.presence != planPinNoPresence &&
				leftPin.presence != rightPin.presence {
				return false
			}

			if leftPin.hasBranch && rightPin.hasBranch &&
				leftPin.occurrence.usePointer == rightPin.occurrence.usePointer &&
				leftPin.composition == rightPin.composition && leftPin.branch == rightPin.branch &&
				leftPin.truth != rightPin.truth {
				return false
			}
		}
	}

	return true
}

// faultTargetIsRealizable reports whether a branch fault can keep its other local rules clean.
func faultTargetIsRealizable(node *schemaNode, occurrence schemaOccurrence, target faultTarget) bool {
	if node == nil || node.schemaShape == nil {
		return false
	}

	if target.obligation.occurrence.usePointer != occurrence.usePointer {
		return true
	}

	if target.obligation.rule == oracleRuleType && node.enum != nil && node.kind != schemaAny {
		expected := schemaNodeJSONKind(node.kind)
		for _, member := range node.enum {
			if member != nil && member.kind != expected {
				return true
			}
		}

		return false
	}

	return true
}

// canonicalFailureClosure sorts and deduplicates one expected failure set.
func canonicalFailureClosure(closure []failureIdentity) ([]failureIdentity, error) {
	if len(closure) == 0 {
		return nil, nil
	}

	result := append([]failureIdentity(nil), closure...)
	if err := stablePlanSort(result, compareRuleIdentities); err != nil {
		return nil, err
	}

	unique := result[:0]
	for _, identity := range result {
		if len(unique) == 0 {
			unique = append(unique, identity)

			continue
		}

		comparison, err := compareRuleIdentities(unique[len(unique)-1], identity)
		if err != nil {
			return nil, err
		}

		if comparison != 0 {
			unique = append(unique, identity)
		}
	}

	return unique, nil
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

		for _, pin := range target.pins {
			if err := validate(pin.occurrence); err != nil {
				return err
			}
		}
	}

	for _, target := range plan.faults {
		if err := validate(target.obligation.occurrence); err != nil {
			return err
		}

		for _, pin := range target.pins {
			if err := validate(pin.occurrence); err != nil {
				return err
			}
		}

		for _, failure := range target.closure {
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
