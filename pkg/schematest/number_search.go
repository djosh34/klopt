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

	candidates := make([]*jsonValue, 0)
	hasNumberRules := false

	for _, rule := range rules {
		if !nodeHasNumberCandidateRules(rule.node) {
			continue
		}

		hasNumberRules = true

		edges, err := numberDeterministicCandidates(rule.node)
		if err != nil {
			return false, err
		}

		for _, edge := range edges {
			candidates, err = appendUniqueJSONWitness(candidates, edge)
			if err != nil {
				return false, err
			}
		}
	}

	for _, candidate := range candidates {
		if err := s.assign(); err != nil {
			return false, err
		}

		complete, err := visit(candidate)
		if err != nil || complete {
			return complete, err
		}
	}

	if !hasNumberRules {
		return false, nil
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

	return s.walkSeededNumberFrontier(
		searchSeed(seedPointer, canonicalSchemaJSON, rule, level),
		visit,
	)
}

// nodeHasNumberCandidateRules reports whether a node contributes an authored numeric edge.
func nodeHasNumberCandidateRules(node *schemaNode) bool {
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

// walkSeededNumberFrontier diagonally explores coefficient length and exponent.
func (s *search) walkSeededNumberFrontier(seed uint64, visit rowVisit) (bool, error) {
	for complexity := uint64(1); ; complexity++ {
		for digitLength := uint64(1); digitLength <= complexity; digitLength++ {
			for exponentRadius := uint64(0); exponentRadius < complexity; exponentRadius++ {
				if digitLength != complexity && exponentRadius+1 != complexity {
					continue
				}

				if err := s.assign(); err != nil {
					return false, err
				}

				complete, err := s.walkSeededNumberExponents(
					digitLength, exponentRadius, seed, visit,
				)
				if err != nil || complete {
					return complete, err
				}
			}
		}

		if complexity == ^uint64(0) {
			return false, nil
		}
	}
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
