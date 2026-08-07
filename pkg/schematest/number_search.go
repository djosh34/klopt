package schematest

import (
	"errors"
	"fmt"
	"math/big"
)

// activeNumberRule is one numeric schema in the selected composition view.
type activeNumberRule struct {
	node       *schemaNode
	occurrence schemaOccurrence
}

// walkActiveNumberRules tries all exact authored edges, then the fair decimal frontier.
//
//nolint:cyclop // Edge collection, seed selection, and the unbounded handoff form one search seam.
func (s *search) walkActiveNumberRules(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	target *validTarget,
	visit rowVisit,
) (bool, error) {
	rules := make([]activeNumberRule, 0)
	if err := collectActiveNumberRules(node, occurrence, pins, &rules); err != nil {
		return false, err
	}

	schedule, err := newNumberSchedule(rules)
	if err != nil {
		return false, err
	}

	emitter := numberCandidateEmitter{search: s, visit: visit}

	complete, err := emitter.walkDeterministic(schedule)
	if err != nil || complete || !schedule.seeded {
		return complete, err
	}

	if s.steps == s.maxSteps {
		return false, errMaxSteps
	}

	seedNode := node
	seedPointer := occurrence.usePointer
	rule := oracleRuleType
	level := jsonKindName(jsonNumber)

	if target != nil {
		if found, ok := scalarTargetNode(node, occurrence, target.expected.occurrence); ok {
			seedNode = found
			seedPointer = target.expected.occurrence.usePointer
			rule = target.expected.rule
			level = target.expected.level
		}
	}

	if seedNode.schemaJSON == nil {
		return false, errors.New("schematest: number search schema has no canonical JSON")
	}

	canonicalSchemaJSON, err := marshalStrict(seedNode.schemaJSON)
	if err != nil {
		return false, fmt.Errorf("schematest: canonicalize number search schema: %w", err)
	}

	seededVisit := func(value *jsonValue) (bool, error) {
		for _, earlier := range emitter.seen {
			comparison, compareErr := value.number.compare(earlier)
			if compareErr != nil {
				return false, compareErr
			}

			if comparison == 0 {
				return false, nil
			}
		}

		return visit(value)
	}

	return s.walkSeededNumberFrontier(
		searchSeed(seedPointer, canonicalSchemaJSON, rule, level),
		seededVisit,
	)
}

// nodeHasNumberSearchRules reports whether a node requires the fair decimal frontier.
func nodeHasNumberSearchRules(node *schemaNode) bool {
	return node.minimum != nil || node.maximum != nil || node.multipleOf != nil ||
		isNumericSchemaFormat(node.format)
}

// collectActiveNumberRules follows allOf and the selected anyOf truth view.
func collectActiveNumberRules(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	rules *[]activeNumberRule,
) error {
	if node == nil || node.schemaShape == nil {
		return errors.New("schematest: active number schema has no shape")
	}

	*rules = append(*rules, activeNumberRule{node: node, occurrence: occurrence})

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child, occurrence.usePointer+"/allOf/"+itoa(index), occurrence.instanceTemplate,
		)
		if err := collectActiveNumberRules(child, childOccurrence, pins, rules); err != nil {
			return err
		}
	}

	states, pinned := rowCompositionTruthStates(pins, occurrence, "anyOf", len(node.anyOf))
	for index, child := range node.anyOf {
		if pinned && !states[index] {
			continue
		}

		childOccurrence := rebasePlanOccurrence(
			child, occurrence.usePointer+"/anyOf/"+itoa(index), occurrence.instanceTemplate,
		)
		if err := collectActiveNumberRules(child, childOccurrence, pins, rules); err != nil {
			return err
		}
	}

	return nil
}

// walkSeededNumberFrontier diagonally explores seeded coefficient-length/exponent shells.
func (s *search) walkSeededNumberFrontier(seed uint64, visit rowVisit) (bool, error) {
	for complexity := uint64(1); ; complexity++ {
		complete, err := s.walkSeededNumberShell(complexity, seed, visit)
		if err != nil || complete {
			return complete, err
		}

		if complexity == ^uint64(0) {
			return false, nil
		}
	}
}

// walkSeededNumberShell visits every pair whose max(length, radius+1) is complexity.
//
//nolint:cyclop // Both seeded shell arms use the same charged pair traversal.
func (s *search) walkSeededNumberShell(
	complexity,
	seed uint64,
	visit rowVisit,
) (bool, error) {
	walkPair := func(digitLength, exponentRadius uint64) (bool, error) {
		if err := s.assign(); err != nil {
			return false, err
		}

		return s.walkSeededNumberExponents(digitLength, exponentRadius, seed, visit)
	}

	walkRadiusArm := func() (bool, error) {
		rotation := (seed ^ complexity) % complexity
		for offset := uint64(0); offset < complexity; offset++ {
			complete, err := walkPair(complexity, (rotation+offset)%complexity)
			if err != nil || complete {
				return complete, err
			}
		}

		return false, nil
	}

	walkLengthArm := func() (bool, error) {
		if complexity == 1 {
			return false, nil
		}

		count := complexity - 1

		rotation := (seed>>8 ^ complexity) % count
		for offset := uint64(0); offset < count; offset++ {
			length := 1 + (rotation+offset)%count

			complete, err := walkPair(length, complexity-1)
			if err != nil || complete {
				return complete, err
			}
		}

		return false, nil
	}

	arms := []func() (bool, error){walkRadiusArm, walkLengthArm}
	if seed>>16^complexity&1 != 0 {
		arms[0], arms[1] = arms[1], arms[0]
	}

	for _, walk := range arms {
		complete, err := walk()
		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}

// walkSeededNumberExponents assigns both signs of one exponent radius.
func (s *search) walkSeededNumberExponents(
	digitLength,
	radius,
	seed uint64,
	visit rowVisit,
) (bool, error) {
	exponents := []*big.Int{new(big.Int).SetUint64(radius)}
	if radius > 0 {
		exponents = append(exponents, new(big.Int).Neg(new(big.Int).SetUint64(radius)))
		if seed&1 != 0 {
			exponents[0], exponents[1] = exponents[1], exponents[0]
		}
	}

	for _, exponent := range exponents {
		if err := s.assign(); err != nil {
			return false, err
		}

		complete, err := s.walkSeededNumberSigns(digitLength, exponent, seed, visit)
		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}

// walkSeededNumberSigns assigns positive and negative coefficients in seeded order.
func (s *search) walkSeededNumberSigns(
	digitLength uint64,
	exponent *big.Int,
	seed uint64,
	visit rowVisit,
) (bool, error) {
	signs := []int64{1, -1}
	if seed>>1&1 != 0 {
		signs[0], signs[1] = signs[1], signs[0]
	}

	for _, sign := range signs {
		if err := s.assign(); err != nil {
			return false, err
		}

		complete, err := s.walkSeededNumberDigits(
			nil, digitLength, exponent, sign, seed, visit,
		)
		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}

// walkSeededNumberDigits performs the charged depth-first coefficient assignment.
//
//nolint:cyclop // Completion, canonicality, charging, and recursive backtracking are one DFS step.
func (s *search) walkSeededNumberDigits(
	digits []byte,
	length uint64,
	exponent *big.Int,
	sign int64,
	seed uint64,
	visit rowVisit,
) (bool, error) {
	if uint64(len(digits)) == length {
		if len(digits) > 1 && digits[len(digits)-1] == '0' {
			return false, nil
		}

		coefficient := new(big.Int)
		if _, ok := coefficient.SetString(string(digits), decimalRadix); !ok {
			return false, errors.New("schematest: seeded number has invalid digits")
		}

		if sign < 0 {
			coefficient.Neg(coefficient)
		}

		number, err := newExactNumber(coefficient, big.NewInt(1), exponent, big.NewInt(0))
		if err != nil {
			return false, err
		}

		return visit(&jsonValue{kind: jsonNumber, number: number})
	}

	first := len(digits) == 0
	count := uint64(decimalRadix)
	base := byte('0')

	if first {
		count = 9
		base = '1'
	}

	rotation := seed % count
	for offset := uint64(0); offset < count; offset++ {
		if err := s.assign(); err != nil {
			return false, err
		}

		digit := base + byte((rotation+offset)%count)

		complete, err := s.walkSeededNumberDigits(
			append(digits, digit), length, exponent, sign, seed, visit,
		)
		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}
