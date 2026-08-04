package schematest

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
)

// makePlan compiles every stable valid and isolated-fault obligation without
// constructing a JSON row or retaining a scalar witness.
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

	sort.SliceStable(compiled.valid, func(left, right int) bool {
		return comparePlanObligations(compiled.valid[left].obligation, compiled.valid[right].obligation) < 0
	})
	sort.SliceStable(compiled.faults, func(left, right int) bool {
		return comparePlanObligations(compiled.faults[left].obligation, compiled.faults[right].obligation) < 0
	})

	obligations := make([]obligation, 0, len(compiled.valid)+len(compiled.faults))
	for _, target := range compiled.valid {
		obligations = append(obligations, target.obligation)
	}

	for _, target := range compiled.faults {
		obligations = append(obligations, target.obligation)
	}

	sort.SliceStable(obligations, func(left, right int) bool {
		return comparePlanObligations(obligations[left], obligations[right]) < 0
	})

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

	builder.compileNumberRules(&result, node, occurrence, validPins, faultPins)
	builder.compileStringRules(&result, node, occurrence, validPins, faultPins)
	builder.compileArrayRules(&result, node, occurrence, validPins, faultPins)
	builder.compileObjectRules(&result, node, occurrence, validPins, faultPins)

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
		for mask := big.NewInt(1); mask.Cmp(anyOfMaskLimit(len(node.anyOf))) < 0; mask.Add(mask, big.NewInt(1)) {
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

// compileAnyOfChildren compiles branch targets and the canonical aggregate fault.
func (builder *planBuilder) compileAnyOfChildren(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
	visiting map[*schemaNode]bool,
	anyOfIdentity ruleIdentity,
) error {
	closure := make([]failureIdentity, 0)
	aggregatePins := appendPlanPins(faultInherited)

	for index, child := range node.anyOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/anyOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		childPlan, err := builder.compileNode(
			child,
			childOccurrence,
			appendPlanPins(validInherited, anyOfValidPins(occurrence, len(node.anyOf), index)...),
			appendPlanPins(faultInherited, anyOfFaultPins(occurrence, len(node.anyOf), index)...),
			visiting,
		)
		if err != nil {
			return err
		}

		result.valid = append(result.valid, childPlan.valid...)
		result.faults = append(result.faults, childPlan.faults...)

		representative, exists := firstCanonicalFault(childPlan.faults)
		if exists {
			closure = append(closure, representative.closure...)
			aggregatePins = appendPlanPins(aggregatePins, representative.pins...)
		}
	}

	closure = append(closure, failureIdentity(anyOfIdentity))
	closure = canonicalFailureClosure(closure)
	aggregatePins = appendPlanPins(
		aggregatePins,
		anyOfMaskPins(occurrence, len(node.anyOf), big.NewInt(0))...,
	)
	builder.addFault(result, anyOfIdentity, aggregatePins, closure)

	return nil
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

		childPlan, err := builder.compileNode(
			shape.items,
			itemOccurrence,
			appendPlanPins(
				validInherited,
				kindPin(occurrence, jsonArray),
				presencePin(itemOccurrence, planPinPresent),
			),
			appendPlanPins(
				faultInherited,
				kindPin(occurrence, jsonArray),
				presencePin(itemOccurrence, planPinPresent),
			),
			visiting,
		)
		if err != nil {
			return err
		}

		result.valid = append(result.valid, childPlan.valid...)
		result.faults = append(result.faults, childPlan.faults...)
	}

	for _, name := range sortedSchemaPropertyNames(shape.properties) {
		property := shape.properties[name]
		propertyOccurrence := rebasePlanOccurrence(
			property,
			occurrence.usePointer+"/properties/"+escapePointerToken(name),
			appendInstanceToken(occurrence.instanceTemplate, name),
		)

		childPlan, err := builder.compileNode(
			property,
			propertyOccurrence,
			appendPlanPins(
				validInherited,
				kindPin(occurrence, jsonObject),
				presencePin(propertyOccurrence, planPinPresent),
			),
			appendPlanPins(
				faultInherited,
				kindPin(occurrence, jsonObject),
				presencePin(propertyOccurrence, planPinPresent),
			),
			visiting,
		)
		if err != nil {
			return err
		}

		result.valid = append(result.valid, childPlan.valid...)
		result.faults = append(result.faults, childPlan.faults...)
	}

	if shape.additionalProperties != nil {
		additionalOccurrence := rebasePlanOccurrence(
			shape.additionalProperties,
			occurrence.usePointer+"/additionalProperties",
			appendInstanceToken(occurrence.instanceTemplate, "*"),
		)

		childPlan, err := builder.compileNode(
			shape.additionalProperties,
			additionalOccurrence,
			appendPlanPins(
				validInherited,
				kindPin(occurrence, jsonObject),
				presencePin(additionalOccurrence, planPinPresent),
			),
			appendPlanPins(
				faultInherited,
				kindPin(occurrence, jsonObject),
				presencePin(additionalOccurrence, planPinPresent),
			),
			visiting,
		)
		if err != nil {
			return err
		}

		result.valid = append(result.valid, childPlan.valid...)
		result.faults = append(result.faults, childPlan.faults...)
	}

	for index, child := range shape.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		childPlan, err := builder.compileNode(
			child,
			childOccurrence,
			appendPlanPins(validInherited, allOfValidPins(occurrence, len(shape.allOf))...),
			appendPlanPins(faultInherited, allOfFaultPins(occurrence, len(shape.allOf), index)...),
			visiting,
		)
		if err != nil {
			return err
		}

		result.valid = append(result.valid, childPlan.valid...)
		result.faults = append(result.faults, childPlan.faults...)
	}

	return nil
}

// compileTypeRules compiles kind levels and an explicit-type fault.
func (builder *planBuilder) compileTypeRules(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
) error {
	identity := makeRuleIdentity(occurrence, oracleRuleType)
	for _, kind := range orderedTypeKinds(node) {
		builder.addValid(
			result,
			identity,
			jsonKindName(kind),
			appendPlanPins(validInherited, kindPin(occurrence, kind)),
		)
	}

	if node.kind != schemaAny {
		builder.addFault(result, identity, faultInherited, []failureIdentity{identity})
	}

	return nil
}

// compileEnumRules compiles semantic enum members and the enum fault.
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
		builder.addValid(
			result,
			identity,
			"member:"+itoa(index),
			appendPlanPins(validInherited, memberKindPin(occurrence, member)),
		)
	}

	builder.addFault(result, identity, faultInherited, []failureIdentity{identity})

	return nil
}

// compileNumberRules compiles applicable numeric rules in canonical order.
func (builder *planBuilder) compileNumberRules(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
) {
	if !nodeCanHaveKind(node, jsonNumber) {
		return
	}

	if node.minimum != nil {
		rule := oracleRuleMinimum
		if node.exclusiveMinimum {
			rule = oracleRuleExclusiveMinimum
		}

		builder.addScalarRule(result, occurrence, rule, validInherited, faultInherited, jsonNumber)
	}

	if node.maximum != nil {
		rule := oracleRuleMaximum
		if node.exclusiveMaximum {
			rule = oracleRuleExclusiveMaximum
		}

		builder.addScalarRule(result, occurrence, rule, validInherited, faultInherited, jsonNumber)
	}

	if node.multipleOf != nil {
		builder.addScalarRule(result, occurrence, oracleRuleMultipleOf, validInherited, faultInherited, jsonNumber)
	}

	if isNumericSchemaFormat(node.format) {
		builder.addScalarRule(result, occurrence, oracleRuleFormat, validInherited, faultInherited, jsonNumber)
	}
}

// compileStringRules compiles applicable string rules in canonical order.
func (builder *planBuilder) compileStringRules(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
) {
	if !nodeCanHaveKind(node, jsonString) {
		return
	}

	if node.minLength != nil {
		builder.addScalarRule(result, occurrence, oracleRuleMinLength, validInherited, faultInherited, jsonString)
	}

	if node.maxLength != nil {
		builder.addScalarRule(result, occurrence, oracleRuleMaxLength, validInherited, faultInherited, jsonString)
	}

	if node.pattern != nil {
		builder.addScalarRule(result, occurrence, oracleRulePattern, validInherited, faultInherited, jsonString)
	}

	if isStringSchemaFormat(node.format) {
		builder.addScalarRule(result, occurrence, oracleRuleFormat, validInherited, faultInherited, jsonString)
	}
}

// compileArrayRules compiles applicable array-count rules in canonical order.
func (builder *planBuilder) compileArrayRules(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
) {
	if !nodeCanHaveKind(node, jsonArray) {
		return
	}

	if node.minItems != nil {
		builder.addScalarRule(result, occurrence, oracleRuleMinItems, validInherited, faultInherited, jsonArray)
	}

	if node.maxItems != nil {
		builder.addScalarRule(result, occurrence, oracleRuleMaxItems, validInherited, faultInherited, jsonArray)
	}
}

// compileObjectRules compiles object counts, required properties, and extras.
func (builder *planBuilder) compileObjectRules(
	result *compiledNodePlan,
	node *schemaNode,
	occurrence schemaOccurrence,
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
) {
	if !nodeCanHaveKind(node, jsonObject) {
		return
	}

	if node.minProperties != nil {
		builder.addScalarRule(result, occurrence, oracleRuleMinProperties, validInherited, faultInherited, jsonObject)
	}

	if node.maxProperties != nil {
		builder.addScalarRule(result, occurrence, oracleRuleMaxProperties, validInherited, faultInherited, jsonObject)
	}

	for _, name := range sortedStrings(node.required) {
		identity := makeRuleIdentity(
			appendObjectMemberOccurrence(occurrence, name),
			oracleRuleRequired,
		)
		presenceOccurrence := requiredPresenceOccurrence(node, occurrence, name)

		builder.addValid(result, identity, oracleRequiredPresentLevel, validInherited)
		builder.addFault(
			result,
			identity,
			appendPlanPins(
				faultInherited,
				kindPin(occurrence, jsonObject),
				presencePin(presenceOccurrence, planPinAbsent),
			),
			[]failureIdentity{identity},
		)
	}

	if node.additionalProperties == nil && !node.allowAdditionalProperties {
		identity := makeRuleIdentity(
			appendObjectMemberOccurrence(occurrence, "*"),
			oracleRuleAdditionalProperties,
		)
		builder.addFault(
			result,
			identity,
			appendPlanPins(
				faultInherited,
				kindPin(occurrence, jsonObject),
				presencePin(identity.occurrence, planPinPresent),
			),
			[]failureIdentity{identity},
		)
	}
}

// addScalarRule adds one valid scalar target and one exact scalar fault.
func (builder *planBuilder) addScalarRule(
	result *compiledNodePlan,
	occurrence schemaOccurrence,
	rule string,
	validInherited []applicabilityPin,
	faultInherited []applicabilityPin,
	kind jsonKind,
) {
	identity := makeRuleIdentity(occurrence, rule)
	ruleRank := planRuleRankForKind(rule, kind)
	builder.addValidAtRank(
		result,
		identity,
		oracleScalarValidLevel,
		appendPlanPins(validInherited, kindPin(occurrence, kind)),
		ruleRank,
	)
	builder.addFaultAtRank(result, identity, faultInherited, []failureIdentity{identity}, ruleRank)
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
) {
	builder.addFaultAtRank(result, identity, pins, closure, planRuleRank(identity.rule))
}

// addFaultAtRank appends one fault target with an applicability-family rank.
func (builder *planBuilder) addFaultAtRank(
	result *compiledNodePlan,
	identity ruleIdentity,
	pins []applicabilityPin,
	closure []failureIdentity,
	ruleRank int,
) {
	faultObligation := makeFaultObligation(identity, identity.rule)
	faultObligation.ruleRank = encodedPlanRuleRank(ruleRank)
	faultObligation.order = builder.nextOrder
	builder.nextOrder++

	result.faults = append(result.faults, faultTarget{
		obligation: faultObligation,
		pins:       copyPlanPins(pins),
		closure:    canonicalFailureClosure(closure),
	})
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

// defaultPlanPins adds structural defaults for one local schema occurrence.
func defaultPlanPins(inherited []applicabilityPin, node *schemaNode, occurrence schemaOccurrence) []applicabilityPin {
	pins := appendPlanPins(inherited, defaultPresencePins(node, occurrence)...)
	if len(node.allOf) > 0 {
		pins = appendPlanPins(pins, allOfValidPins(occurrence, len(node.allOf))...)
	}

	if len(node.anyOf) > 0 {
		pins = appendPlanPins(pins, anyOfMaskPins(occurrence, len(node.anyOf), big.NewInt(1))...)
	}

	return pins
}

// defaultPresencePins chooses required children and omits optional children.
func defaultPresencePins(node *schemaNode, occurrence schemaOccurrence) []applicabilityPin {
	shape := node.schemaShape
	pins := make([]applicabilityPin, 0, len(shape.properties))

	if shape.items != nil {
		itemOccurrence := rebasePlanOccurrence(
			shape.items,
			occurrence.usePointer+"/items",
			appendInstanceToken(occurrence.instanceTemplate, "*"),
		)
		pins = append(pins, presencePin(itemOccurrence, planPinAbsent))
	}

	for _, name := range sortedSchemaPropertyNames(shape.properties) {
		property := shape.properties[name]
		propertyOccurrence := rebasePlanOccurrence(
			property,
			occurrence.usePointer+"/properties/"+escapePointerToken(name),
			appendInstanceToken(occurrence.instanceTemplate, name),
		)

		presence := planPinAbsent
		if containsString(shape.required, name) {
			presence = planPinPresent
		}

		pins = append(pins, presencePin(propertyOccurrence, presence))
	}

	if shape.additionalProperties != nil {
		additionalOccurrence := rebasePlanOccurrence(
			shape.additionalProperties,
			occurrence.usePointer+"/additionalProperties",
			appendInstanceToken(occurrence.instanceTemplate, "*"),
		)
		pins = append(pins, presencePin(additionalOccurrence, planPinAbsent))
	}

	return pins
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

// firstSiblingCompatibleKind selects the first authored or constrained kind.
func firstSiblingCompatibleKind(node *schemaNode, allowed map[jsonKind]bool) (jsonKind, bool) {
	if node.enum != nil {
		for _, member := range node.enum {
			if member != nil && allowed[member.kind] {
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

// anyOfValidPins pins one anyOf branch true and every other branch false.
func anyOfValidPins(occurrence schemaOccurrence, count, selected int) []applicabilityPin {
	return compositionPins(occurrence, "anyOf", count, selected, false)
}

// anyOfFaultPins pins one anyOf branch false and every sibling branch true.
func anyOfFaultPins(occurrence schemaOccurrence, count, selected int) []applicabilityPin {
	return compositionFaultPins(occurrence, "anyOf", count, selected)
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

// anyOfMaskLimit returns one greater than the largest valid nonzero mask.
func anyOfMaskLimit(count int) *big.Int {
	return new(big.Int).Lsh(big.NewInt(1), uint(count))
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
func firstCanonicalFault(faults []faultTarget) (faultTarget, bool) {
	if len(faults) == 0 {
		return faultTarget{}, false
	}

	best := faults[0]
	for _, candidate := range faults[1:] {
		if comparePlanObligations(candidate.obligation, best.obligation) < 0 {
			best = candidate
		}
	}

	return best, true
}

// canonicalFailureClosure sorts and deduplicates one expected failure set.
func canonicalFailureClosure(closure []failureIdentity) []failureIdentity {
	if len(closure) == 0 {
		return nil
	}

	result := append([]failureIdentity(nil), closure...)
	sort.SliceStable(result, func(left, right int) bool {
		return compareRuleIdentities(result[left], result[right]) < 0
	})

	unique := result[:0]
	for _, identity := range result {
		if len(unique) == 0 || compareRuleIdentities(unique[len(unique)-1], identity) != 0 {
			unique = append(unique, identity)
		}
	}

	return unique
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
