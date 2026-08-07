//nolint:godoclint // Private product-search vocabulary is documented at its execution seams.
package schematest

import (
	"errors"
	"fmt"
)

type stringRuleKind uint8

const (
	stringRuleMinLength stringRuleKind = iota
	stringRuleMaxLength
)

type stringConstraintKind uint8

const (
	stringConstraintEnum stringConstraintKind = iota
	stringConstraintPattern
	stringConstraintFormat
	stringConstraintLength
)

type stringConstraintRef struct {
	kind  stringConstraintKind
	index int
}

type stringObjectiveOwner struct {
	node       *schemaNode
	occurrence schemaOccurrence
	source     *jsonValue
}

type stringEnumConstraint struct {
	identity ruleIdentity
	owner    stringObjectiveOwner
	members  []*jsonValue
}

type stringPatternAssertionConstraint struct {
	positive bool
	graph    *stringPatternGraph
}

type stringPatternConstraint struct {
	identity   ruleIdentity
	owner      stringObjectiveOwner
	pattern    *patternAST
	graph      *stringPatternGraph
	assertions []stringPatternAssertionConstraint

	finite  bool
	maximum uint64
}

type stringFormatConstraint struct {
	identity ruleIdentity
	owner    stringObjectiveOwner
	format   schemaFormat
}

type stringLengthConstraint struct {
	identity ruleIdentity
	owner    stringObjectiveOwner
	kind     stringRuleKind
	bound    *exactCount
}

type stringPinnedLength struct {
	value uint64
	kind  stringObjectiveKind
	index int
}

type stringProduct struct {
	enums    []stringEnumConstraint
	patterns []stringPatternConstraint
	formats  []stringFormatConstraint
	lengths  []stringLengthConstraint

	truth        stringTruthExpression
	owners       map[schemaOccurrence]stringObjectiveOwner
	defaultOwner stringObjectiveOwner
	enumValues   []*jsonValue
	fixedLengths []stringPinnedLength
	seedUnits    []uint16
}

type stringObjectiveKind uint8

const (
	stringObjectiveAllTrue stringObjectiveKind = iota
	stringObjectivePatternFalse
	stringObjectiveFormatFalse
	stringObjectiveLengthFalse
)

type stringObjective struct {
	kind    stringObjectiveKind
	index   int
	rule    string
	level   string
	owner   stringObjectiveOwner
	term    *stringTruthTerm
	closure []failureIdentity
}

type stringProductRuntime struct {
	patterns []stringPatternRuntime
}

type stringProductTransition struct {
	interval         stringUnitInterval
	targets          [][]int
	assertionTargets [][][]int
}

type stringTruthKind uint8

const (
	stringTruthFalse stringTruthKind = iota
	stringTruthTrue
	stringTruthAtom
	stringTruthAnd
	stringTruthOr
	stringTruthNot
)

type stringTruthExpression struct {
	kind       stringTruthKind
	constraint stringConstraintRef
	children   []stringTruthExpression
}

type stringTruthAssignment struct {
	constraint stringConstraintRef
	truth      bool
}

type stringTruthTerm struct {
	assignments []stringTruthAssignment
}

type stringBranchPin struct {
	constrained bool
	truth       bool
}

// buildStringProduct collects every string rule active in one target state.
func buildStringProduct(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
) (stringProduct, error) {
	if node == nil || node.schemaShape == nil {
		return stringProduct{}, errors.New("string product has no schema shape")
	}

	owner, err := newStringObjectiveOwner(node, occurrence)
	if err != nil {
		return stringProduct{}, err
	}

	product := stringProduct{
		owners:       make(map[schemaOccurrence]stringObjectiveOwner),
		defaultOwner: owner,
	}

	truth, err := collectStringProductRules(&product, node, occurrence, pins, make(map[*schemaNode]bool))
	if err != nil {
		return stringProduct{}, err
	}

	product.truth = truth
	if err := compileStringProductPatterns(&product); err != nil {
		return stringProduct{}, err
	}

	addStringProductFormatFrontier(&product)
	addStringProductPatternLengths(&product)
	product.fixedLengths = uniqueStringPinnedLengths(product.fixedLengths)

	return product, nil
}

func compileStringProductPatterns(product *stringProduct) error {
	for index := range product.patterns {
		pattern := &product.patterns[index]

		graph, err := compileStringPatternGraph(pattern.pattern)
		if err != nil {
			return fmt.Errorf("compile string pattern %s: %w", pattern.identity, err)
		}

		pattern.graph = graph
		for _, assertion := range pattern.pattern.leadingAssertions {
			assertionGraph, assertionErr := compileStringPatternExpressionGraph(assertion.expression, true)
			if assertionErr != nil {
				return fmt.Errorf("compile string assertion %s: %w", pattern.identity, assertionErr)
			}

			pattern.assertions = append(pattern.assertions, stringPatternAssertionConstraint{
				positive: assertion.positive,
				graph:    assertionGraph,
			})
		}

		product.seedUnits = appendUniqueUint16(product.seedUnits, stringPatternSeedUnits(pattern.pattern)...)
	}

	return nil
}

func addStringProductFormatFrontier(product *stringProduct) {
	for index, format := range product.formats {
		for _, length := range stringFormatPinnedLengths(format.format) {
			product.fixedLengths = append(product.fixedLengths, stringPinnedLength{
				value: length,
				kind:  stringObjectiveFormatFalse,
				index: index,
			})
		}

		product.seedUnits = appendUniqueUint16(product.seedUnits, stringFormatSeedUnits(format.format)...)
	}
}

func addStringProductPatternLengths(product *stringProduct) {
	for index, pattern := range product.patterns {
		if maximum, finite := stringPatternMaximumRuneLength(pattern.pattern); finite {
			product.patterns[index].finite = true
			product.patterns[index].maximum = maximum
		}

		lengths, exact := stringPatternExactRuneLengths(pattern.pattern)
		if !exact {
			continue
		}

		for _, length := range lengths {
			product.fixedLengths = append(product.fixedLengths, stringPinnedLength{
				value: length,
				kind:  stringObjectivePatternFalse,
				index: index,
			})
		}
	}
}

func newStringObjectiveOwner(node *schemaNode, occurrence schemaOccurrence) (stringObjectiveOwner, error) {
	if node == nil || node.schemaShape == nil || node.source == nil {
		return stringObjectiveOwner{}, errors.New("schematest invariant: string objective has no owner schema")
	}

	if occurrence.usePointer == "" {
		return stringObjectiveOwner{}, errors.New("schematest invariant: string objective owner has no occurrence pointer")
	}

	return stringObjectiveOwner{node: node, occurrence: occurrence, source: node.source}, nil
}

func collectStringProductRules(
	product *stringProduct,
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	visiting map[*schemaNode]bool,
) (stringTruthExpression, error) {
	if node == nil || node.schemaShape == nil {
		return stringTruthExpression{}, errors.New("string product child has no schema shape")
	}

	if visiting[node] {
		return stringTruthExpression{}, fmt.Errorf("recursive string product at %s", occurrence.usePointer)
	}

	owner, err := newStringObjectiveOwner(node, occurrence)
	if err != nil {
		return stringTruthExpression{}, err
	}

	product.owners[occurrence] = owner

	if !nodeCanHaveKind(node, jsonString) {
		return stringTruthExpression{kind: stringTruthFalse}, nil
	}

	visiting[node] = true
	defer delete(visiting, node)

	parts, err := collectDirectStringProductRules(product, node, occurrence, owner)
	if err != nil {
		return stringTruthExpression{}, err
	}

	allOfParts, err := collectStringAllOfTruth(product, node, occurrence, pins, visiting)
	if err != nil {
		return stringTruthExpression{}, err
	}

	parts = append(parts, allOfParts...)

	if len(node.anyOf) > 0 {
		anyOfTruth, anyOfErr := collectStringAnyOfTruth(product, node, occurrence, pins, visiting)
		if anyOfErr != nil {
			return stringTruthExpression{}, anyOfErr
		}

		parts = append(parts, anyOfTruth)
	}

	return stringTruthExpression{kind: stringTruthAnd, children: parts}, nil
}

func collectStringAllOfTruth(
	product *stringProduct,
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	visiting map[*schemaNode]bool,
) ([]stringTruthExpression, error) {
	parts := make([]stringTruthExpression, 0, len(node.allOf))
	for index, child := range node.allOf {
		childOccurrence := stringProductAllOfOccurrence(node, child, occurrence, index)

		truth, err := collectStringProductRules(product, child, childOccurrence, pins, visiting)
		if err != nil {
			return nil, err
		}

		parts = append(parts, truth)
	}

	return parts, nil
}

func stringProductAllOfOccurrence(
	node, child *schemaNode,
	occurrence schemaOccurrence,
	index int,
) schemaOccurrence {
	if index < len(node.syntheticAllOfOccurrences) {
		source := node.syntheticAllOfOccurrences[index]
		if source.usePointer != "" {
			source.instanceTemplate = occurrence.instanceTemplate

			return source
		}
	}

	return rebasePlanOccurrence(
		child,
		occurrence.usePointer+"/allOf/"+itoa(index),
		occurrence.instanceTemplate,
	)
}

func collectDirectStringProductRules(
	product *stringProduct,
	node *schemaNode,
	occurrence schemaOccurrence,
	owner stringObjectiveOwner,
) ([]stringTruthExpression, error) {
	parts := make([]stringTruthExpression, 0)
	appendAtom := func(constraint stringConstraintRef) {
		parts = append(parts, stringTruthExpression{kind: stringTruthAtom, constraint: constraint})
	}

	if node.enum != nil {
		constraint, err := collectStringEnumConstraint(product, node.enum, occurrence, owner)
		if err != nil {
			return nil, err
		}

		product.enums = append(product.enums, constraint)
		appendAtom(stringConstraintRef{kind: stringConstraintEnum, index: len(product.enums) - 1})
	}

	if node.minLength != nil {
		product.lengths = append(product.lengths, stringLengthConstraint{
			identity: makeRuleIdentity(occurrence, oracleRuleMinLength),
			owner:    owner,
			kind:     stringRuleMinLength,
			bound:    node.minLength,
		})
		appendAtom(stringConstraintRef{kind: stringConstraintLength, index: len(product.lengths) - 1})
	}

	if node.maxLength != nil {
		product.lengths = append(product.lengths, stringLengthConstraint{
			identity: makeRuleIdentity(occurrence, oracleRuleMaxLength),
			owner:    owner,
			kind:     stringRuleMaxLength,
			bound:    node.maxLength,
		})
		appendAtom(stringConstraintRef{kind: stringConstraintLength, index: len(product.lengths) - 1})
	}

	if node.pattern != nil {
		product.patterns = append(product.patterns, stringPatternConstraint{
			identity: makeRuleIdentity(occurrence, oracleRulePattern),
			owner:    owner,
			pattern:  node.pattern,
		})
		appendAtom(stringConstraintRef{kind: stringConstraintPattern, index: len(product.patterns) - 1})
	}

	if isStringSchemaFormat(node.format) && node.format != schemaFormatPassword {
		product.formats = append(product.formats, stringFormatConstraint{
			identity: makeRuleIdentity(occurrence, oracleRuleFormat),
			owner:    owner,
			format:   node.format,
		})
		appendAtom(stringConstraintRef{kind: stringConstraintFormat, index: len(product.formats) - 1})
	}

	return parts, nil
}

func collectStringEnumConstraint(
	product *stringProduct,
	values []*jsonValue,
	occurrence schemaOccurrence,
	owner stringObjectiveOwner,
) (stringEnumConstraint, error) {
	constraint := stringEnumConstraint{
		identity: makeRuleIdentity(occurrence, oracleRuleEnum),
		owner:    owner,
	}

	for _, value := range values {
		if value == nil {
			return stringEnumConstraint{}, errors.New("string product enum value is nil")
		}

		if value.kind != jsonString {
			continue
		}

		members, err := appendUniqueJSONWitness(constraint.members, value)
		if err != nil {
			return stringEnumConstraint{}, fmt.Errorf("collect string enum members: %w", err)
		}

		constraint.members = members

		productValues, err := appendUniqueJSONWitness(product.enumValues, value)
		if err != nil {
			return stringEnumConstraint{}, fmt.Errorf("collect string product enum: %w", err)
		}

		product.enumValues = productValues
	}

	return constraint, nil
}

func collectStringAnyOfTruth(
	product *stringProduct,
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	visiting map[*schemaNode]bool,
) (stringTruthExpression, error) {
	branchPins, err := stringAnyOfBranchPins(pins, occurrence, len(node.anyOf))
	if err != nil {
		return stringTruthExpression{}, err
	}

	branches := make([]stringTruthExpression, 0, len(node.anyOf))
	for index, child := range node.anyOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/anyOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		childTruth, childErr := collectStringProductRules(product, child, childOccurrence, pins, visiting)
		if childErr != nil {
			return stringTruthExpression{}, childErr
		}

		branches = append(branches, childTruth)
	}

	requirements := make([]stringTruthExpression, 0, len(branches)+1)
	hasTrue := false

	for index, pin := range branchPins {
		if !pin.constrained {
			continue
		}

		if pin.truth {
			hasTrue = true

			requirements = append(requirements, branches[index])

			continue
		}

		requirements = append(requirements, stringTruthExpression{
			kind:     stringTruthNot,
			children: []stringTruthExpression{branches[index]},
		})
	}

	if !hasTrue {
		requirements = append(requirements, stringTruthExpression{kind: stringTruthOr, children: branches})
	}

	return stringTruthExpression{kind: stringTruthAnd, children: requirements}, nil
}

func stringAnyOfBranchPins(
	pins []applicabilityPin,
	occurrence schemaOccurrence,
	count int,
) ([]stringBranchPin, error) {
	branches := make([]stringBranchPin, count)
	for index := range branches {
		branchPointer := occurrence.usePointer + "/anyOf/" + itoa(index)
		for _, pin := range pins {
			if !pin.hasBranch || pin.composition != "anyOf" || pin.branch != index ||
				pin.occurrence.usePointer != branchPointer ||
				!instanceTemplateMatches(pin.occurrence.instanceTemplate, occurrence.instanceTemplate) {
				continue
			}

			if branches[index].constrained && branches[index].truth != pin.truth {
				return nil, fmt.Errorf("conflicting anyOf branch pins at %s", branchPointer)
			}

			branches[index] = stringBranchPin{constrained: true, truth: pin.truth}
		}
	}

	return branches, nil
}

func (product stringProduct) ownerFor(target ruleIdentity) (stringObjectiveOwner, error) {
	if owner, exists := product.owners[target.occurrence]; exists {
		return validateStringObjectiveOwner(owner)
	}

	if product.defaultOwner.occurrence.usePointer == target.occurrence.usePointer {
		owner := product.defaultOwner
		owner.occurrence = target.occurrence

		return validateStringObjectiveOwner(owner)
	}

	owner, found, err := product.ownerForConstraint(target)
	if err != nil {
		return stringObjectiveOwner{}, err
	}

	if found {
		owner.occurrence = target.occurrence

		return validateStringObjectiveOwner(owner)
	}

	if !stringRuleOwnsStringObjective(target.rule) {
		return validateStringObjectiveOwner(product.defaultOwner)
	}

	return stringObjectiveOwner{}, fmt.Errorf(
		"schematest invariant: string objective owner is missing for %s at %s",
		target.rule,
		target.occurrence.usePointer,
	)
}

func stringRuleOwnsStringObjective(rule string) bool {
	switch rule {
	case oracleRuleEnum, oracleRulePattern, oracleRuleFormat, oracleRuleMinLength, oracleRuleMaxLength:
		return true
	default:
		return false
	}
}

func validateStringObjectiveOwner(owner stringObjectiveOwner) (stringObjectiveOwner, error) {
	if owner.node == nil || owner.node.schemaShape == nil || owner.source == nil || owner.occurrence.usePointer == "" {
		return stringObjectiveOwner{}, fmt.Errorf(
			"schematest invariant: string objective owner is incomplete at %s",
			owner.occurrence.usePointer,
		)
	}

	return owner, nil
}

func (product stringProduct) ownerForConstraint(
	target ruleIdentity,
) (stringObjectiveOwner, bool, error) {
	switch target.rule {
	case oracleRuleEnum:
		return findStringConstraintOwner(product.enums, target, stringEnumConstraintDetails)
	case oracleRulePattern:
		return findStringConstraintOwner(product.patterns, target, stringPatternConstraintDetails)
	case oracleRuleFormat:
		return findStringConstraintOwner(product.formats, target, stringFormatConstraintDetails)
	case oracleRuleMinLength, oracleRuleMaxLength:
		return findStringConstraintOwner(product.lengths, target, stringLengthConstraintDetails)
	default:
		return stringObjectiveOwner{}, false, nil
	}
}

func findStringConstraintOwner[T any](
	values []T,
	target ruleIdentity,
	details func(T) (ruleIdentity, stringObjectiveOwner),
) (stringObjectiveOwner, bool, error) {
	var owner stringObjectiveOwner

	found := false

	for _, value := range values {
		identity, candidate := details(value)
		if !sameResolvedStringRuleIdentity(target, identity) {
			continue
		}

		if found {
			return stringObjectiveOwner{}, false, fmt.Errorf(
				"schematest invariant: string objective owner is ambiguous at %s",
				target.occurrence.usePointer,
			)
		}

		owner = candidate
		found = true
	}

	return owner, found, nil
}

func sameResolvedStringRuleIdentity(left, right ruleIdentity) bool {
	return left.rule == right.rule && left.occurrence.targetPointer == right.occurrence.targetPointer &&
		left.occurrence.instanceTemplate == right.occurrence.instanceTemplate &&
		left.occurrence.reference == right.occurrence.reference
}

func (product stringProduct) hasStringRules() bool {
	return len(product.enums) > 0 || len(product.patterns) > 0 || len(product.formats) > 0 || len(product.lengths) > 0
}

func (product stringProduct) constraintDetails(
	constraint stringConstraintRef,
) (ruleIdentity, stringObjectiveOwner, error) {
	switch constraint.kind {
	case stringConstraintEnum:
		return stringConstraintDetailsFor(product.enums, constraint, stringEnumConstraintDetails)
	case stringConstraintPattern:
		return stringConstraintDetailsFor(product.patterns, constraint, stringPatternConstraintDetails)
	case stringConstraintFormat:
		return stringConstraintDetailsFor(product.formats, constraint, stringFormatConstraintDetails)
	case stringConstraintLength:
		return stringConstraintDetailsFor(product.lengths, constraint, stringLengthConstraintDetails)
	default:
		return missingStringConstraintDetails(constraint)
	}
}

func stringConstraintDetailsFor[T any](
	values []T,
	constraint stringConstraintRef,
	details func(T) (ruleIdentity, stringObjectiveOwner),
) (ruleIdentity, stringObjectiveOwner, error) {
	if constraint.index < 0 || constraint.index >= len(values) {
		return missingStringConstraintDetails(constraint)
	}

	identity, owner := details(values[constraint.index])

	return identity, owner, nil
}

func stringEnumConstraintDetails(value stringEnumConstraint) (ruleIdentity, stringObjectiveOwner) {
	return value.identity, value.owner
}

func stringPatternConstraintDetails(value stringPatternConstraint) (ruleIdentity, stringObjectiveOwner) {
	return value.identity, value.owner
}

func stringFormatConstraintDetails(value stringFormatConstraint) (ruleIdentity, stringObjectiveOwner) {
	return value.identity, value.owner
}

func stringLengthConstraintDetails(value stringLengthConstraint) (ruleIdentity, stringObjectiveOwner) {
	return value.identity, value.owner
}

func missingStringConstraintDetails(
	constraint stringConstraintRef,
) (ruleIdentity, stringObjectiveOwner, error) {
	return ruleIdentity{}, stringObjectiveOwner{}, fmt.Errorf(
		"string constraint is missing for %d:%d",
		constraint.kind,
		constraint.index,
	)
}

func stringObjectiveConstraintTruth(
	objective stringObjective,
	constraint stringConstraintRef,
) (bool, bool) {
	if objective.term != nil {
		for _, assignment := range objective.term.assignments {
			if assignment.constraint == constraint {
				return assignment.truth, true
			}
		}

		if objective.kind == stringObjectiveAllTrue {
			return false, false
		}
	}

	if objective.kind == stringObjectiveAllTrue {
		return true, true
	}

	if stringObjectiveDirectedConstraint(objective) == constraint {
		return false, true
	}

	return true, true
}

func stringObjectiveDirectedConstraint(objective stringObjective) stringConstraintRef {
	switch objective.kind {
	case stringObjectivePatternFalse:
		return stringConstraintRef{kind: stringConstraintPattern, index: objective.index}
	case stringObjectiveFormatFalse:
		return stringConstraintRef{kind: stringConstraintFormat, index: objective.index}
	case stringObjectiveLengthFalse:
		return stringConstraintRef{kind: stringConstraintLength, index: objective.index}
	default:
		return stringConstraintRef{kind: stringConstraintEnum, index: -1}
	}
}

func stringObjectiveFalseConstraint(objective stringObjective, constraint stringConstraintRef) bool {
	truth, constrained := stringObjectiveConstraintTruth(objective, constraint)

	return constrained && !truth
}

func stringObjectiveRequiresConstraint(objective stringObjective, constraint stringConstraintRef) bool {
	truth, constrained := stringObjectiveConstraintTruth(objective, constraint)

	return constrained && truth
}

func newStringDirectedObjective(
	product stringProduct,
	kind stringObjectiveKind,
	index int,
) (stringObjective, error) {
	objective := stringObjective{kind: kind, index: index, level: "false"}
	constraint := stringObjectiveDirectedConstraint(objective)

	identity, owner, err := product.constraintDetails(constraint)
	if err != nil {
		return stringObjective{}, err
	}

	if owner.node == nil || owner.node.schemaShape == nil || owner.source == nil || owner.occurrence.usePointer == "" {
		return stringObjective{}, errors.New("schematest invariant: directed string objective has no owner")
	}

	objective.rule = identity.rule
	objective.owner = owner
	objective.closure = []failureIdentity{identity}

	return objective, nil
}

// runStringObjectiveSchedule lazily visits valid truth terms followed by directed negatives.
func (s *search) runStringObjectiveSchedule(
	product stringProduct,
	owner stringObjectiveOwner,
	rule, level string,
	runDirected bool,
	visit func(stringObjective, *jsonValue) (bool, error),
) (bool, error) {
	if err := validateStringObjectiveSchedule(owner, visit); err != nil {
		return false, err
	}

	found, err := s.searchStringValidTerms(product, owner, rule, level, runDirected, visit)
	if err != nil || !runDirected {
		return found, err
	}

	if err := s.searchStringDirectedFamily(
		product,
		stringObjectivePatternFalse,
		len(product.patterns),
		visit,
	); err != nil {
		return false, err
	}

	if err := s.searchStringDirectedFamily(
		product,
		stringObjectiveFormatFalse,
		len(product.formats),
		visit,
	); err != nil {
		return false, err
	}

	if err := s.searchStringDirectedFamily(
		product,
		stringObjectiveLengthFalse,
		len(product.lengths),
		visit,
	); err != nil {
		return false, err
	}

	return found, nil
}

func validateStringObjectiveSchedule(
	owner stringObjectiveOwner,
	visit func(stringObjective, *jsonValue) (bool, error),
) error {
	if visit == nil {
		return errors.New("nil string objective callback")
	}

	if owner.node == nil || owner.node.schemaShape == nil || owner.source == nil || owner.occurrence.usePointer == "" {
		return errors.New("schematest invariant: string objective schedule has no owner")
	}

	return nil
}

func (s *search) searchStringValidTerms(
	product stringProduct,
	owner stringObjectiveOwner,
	rule, level string,
	runDirected bool,
	visit func(stringObjective, *jsonValue) (bool, error),
) (bool, error) {
	found := false
	_, err := enumerateStringTruth(product.truth, true, &stringTruthTerm{}, func(term *stringTruthTerm) (bool, error) {
		objective := stringObjective{
			kind:  stringObjectiveAllTrue,
			rule:  rule,
			level: level,
			owner: owner,
			term:  term,
		}

		complete, searchErr := s.searchStringObjective(product, objective, func(value *jsonValue) (bool, error) {
			return visit(objective, value)
		})
		if searchErr != nil || !complete {
			return false, searchErr
		}

		found = true

		return !runDirected, nil
	})

	return found, err
}

func (s *search) searchStringDirectedFamily(
	product stringProduct,
	kind stringObjectiveKind,
	count int,
	visit func(stringObjective, *jsonValue) (bool, error),
) error {
	for index := range count {
		objective, err := newStringDirectedObjective(product, kind, index)
		if err != nil {
			return err
		}

		if _, err := s.searchStringObjective(product, objective, func(value *jsonValue) (bool, error) {
			return visit(objective, value)
		}); err != nil {
			return err
		}
	}

	return nil
}

func enumerateStringTruth(
	expression stringTruthExpression,
	wanted bool,
	term *stringTruthTerm,
	visit func(*stringTruthTerm) (bool, error),
) (bool, error) {
	switch expression.kind {
	case stringTruthFalse, stringTruthTrue:
		return enumerateStringTruthConstant(expression.kind, wanted, term, visit)
	case stringTruthAtom:
		return enumerateStringTruthAtom(expression.constraint, wanted, term, visit)
	case stringTruthNot:
		return enumerateStringTruthNot(expression.children, wanted, term, visit)
	case stringTruthAnd, stringTruthOr:
		return enumerateStringTruthBoolean(expression.kind, expression.children, wanted, term, visit)
	default:
		return false, fmt.Errorf("unknown string truth kind %d", expression.kind)
	}
}

func enumerateStringTruthConstant(
	kind stringTruthKind,
	wanted bool,
	term *stringTruthTerm,
	visit func(*stringTruthTerm) (bool, error),
) (bool, error) {
	if (kind == stringTruthTrue) != wanted {
		return false, nil
	}

	return visit(term)
}

func enumerateStringTruthAtom(
	constraint stringConstraintRef,
	wanted bool,
	term *stringTruthTerm,
	visit func(*stringTruthTerm) (bool, error),
) (bool, error) {
	length := len(term.assignments)
	if !appendStringTruthAssignment(term, constraint, wanted) {
		return false, nil
	}

	stopped, err := visit(term)
	term.assignments = term.assignments[:length]

	return stopped, err
}

func enumerateStringTruthNot(
	children []stringTruthExpression,
	wanted bool,
	term *stringTruthTerm,
	visit func(*stringTruthTerm) (bool, error),
) (bool, error) {
	if len(children) != 1 {
		return false, errors.New("string truth negation has wrong arity")
	}

	return enumerateStringTruth(children[0], !wanted, term, visit)
}

func enumerateStringTruthBoolean(
	kind stringTruthKind,
	children []stringTruthExpression,
	wanted bool,
	term *stringTruthTerm,
	visit func(*stringTruthTerm) (bool, error),
) (bool, error) {
	if (kind == stringTruthAnd) == wanted {
		return enumerateStringTruthConjunction(children, wanted, term, visit)
	}

	for _, child := range children {
		stopped, err := enumerateStringTruth(child, wanted, term, visit)
		if err != nil || stopped {
			return stopped, err
		}
	}

	return false, nil
}

func enumerateStringTruthConjunction(
	children []stringTruthExpression,
	wanted bool,
	term *stringTruthTerm,
	visit func(*stringTruthTerm) (bool, error),
) (bool, error) {
	var enumerate func(int) (bool, error)

	enumerate = func(index int) (bool, error) {
		if index == len(children) {
			return visit(term)
		}

		return enumerateStringTruth(children[index], wanted, term, func(*stringTruthTerm) (bool, error) {
			return enumerate(index + 1)
		})
	}

	return enumerate(0)
}

func appendStringTruthAssignment(term *stringTruthTerm, constraint stringConstraintRef, truth bool) bool {
	for _, assignment := range term.assignments {
		if assignment.constraint == constraint {
			return assignment.truth == truth
		}
	}

	term.assignments = append(term.assignments, stringTruthAssignment{constraint: constraint, truth: truth})

	return true
}

func stringProductTargetIsLast(product stringProduct, target ruleIdentity) (bool, error) {
	later, err := product.visitConstraintIdentities(func(identity ruleIdentity) (bool, error) {
		comparison, compareErr := compareRuleIdentities(target, identity)
		if compareErr != nil {
			return false, compareErr
		}

		return comparison < 0, nil
	})

	return !later, err
}

func (product stringProduct) visitConstraintIdentities(
	visit func(ruleIdentity) (bool, error),
) (bool, error) {
	stopped, err := visitStringConstraintIdentities(product.enums, stringEnumIdentity, visit)
	if err != nil || stopped {
		return stopped, err
	}

	stopped, err = visitStringConstraintIdentities(product.patterns, stringPatternIdentity, visit)
	if err != nil || stopped {
		return stopped, err
	}

	stopped, err = visitStringConstraintIdentities(product.formats, stringFormatIdentity, visit)
	if err != nil || stopped {
		return stopped, err
	}

	return visitStringConstraintIdentities(product.lengths, stringLengthIdentity, visit)
}

func visitStringConstraintIdentities[T any](
	values []T,
	identity func(T) ruleIdentity,
	visit func(ruleIdentity) (bool, error),
) (bool, error) {
	for _, value := range values {
		stopped, err := visit(identity(value))
		if err != nil || stopped {
			return stopped, err
		}
	}

	return false, nil
}

func stringEnumIdentity(value stringEnumConstraint) ruleIdentity {
	return value.identity
}

func stringPatternIdentity(value stringPatternConstraint) ruleIdentity {
	return value.identity
}

func stringFormatIdentity(value stringFormatConstraint) ruleIdentity {
	return value.identity
}

func stringLengthIdentity(value stringLengthConstraint) ruleIdentity {
	return value.identity
}
