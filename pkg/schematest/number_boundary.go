//nolint:godoclint // Private numeric address types are documented at their search seams.
package schematest

import (
	"errors"
	"math/big"
	"sort"
)

const (
	// numberRuleValueCount is the count of scale-bearing numeric rule values.
	numberRuleValueCount = 3
	// numberEdgeTermCount is the maximum compact terms in one edge comparison.
	numberEdgeTermCount = 4

	numberBoundaryEdgeCount  = 3
	numberMultipleEdgeCount  = 4
	numberFormatEdgeCount    = 4
	numberRuleEdgeCount      = numberBoundaryEdgeCount*2 + numberMultipleEdgeCount + numberFormatEdgeCount
	numberSecondDirectedEdge = 2
)

type numberEdgeFamily uint8

const (
	numberEdgeEnum numberEdgeFamily = iota
	numberEdgeMinimum
	numberEdgeMaximum
	numberEdgeZero
	numberEdgeMultiple
	numberEdgeFormat
)

// numberSchedule is the exact numeric conjunction for one constrained composition view.
type numberSchedule struct {
	rules   []activeNumberRule
	quantum *exactNumber
	seeded  bool
	hasEnum bool
}

// numberEdge describes one exact choice without constructing a boundary neighbor.
type numberEdge struct {
	base      *exactNumber
	increment *exactNumber
	sign      int64
}

// numberEdgeAddress directly identifies one deterministic schedule slot.
type numberEdgeAddress struct {
	ruleIndex uint64
	family    numberEdgeFamily
	local     uint64
}

// newNumberSchedule compiles one active conjunction and its shared quantum.
//
//nolint:cyclop // Active kind, scale, enum, and frontier policy are compiled in one pass.
func newNumberSchedule(rules []activeNumberRule) (numberSchedule, error) {
	if len(rules) == 0 {
		return numberSchedule{}, errors.New("schematest: number schedule has no active schemas")
	}

	integer := false
	seeded := false
	hasEnum := false
	numbers := make([]*exactNumber, 0, len(rules)*numberRuleValueCount)

	for _, rule := range rules {
		if rule.node == nil || rule.node.schemaShape == nil {
			return numberSchedule{}, errors.New("schematest: number boundary schema has no shape")
		}

		integer = integer || rule.node.kind == schemaInteger
		seeded = seeded || nodeHasNumberSearchRules(rule.node)

		hasEnum = hasEnum || rule.node.enum != nil

		for _, number := range []*exactNumber{rule.node.minimum, rule.node.maximum, rule.node.multipleOf} {
			if number != nil {
				numbers = append(numbers, number)
			}
		}
	}

	var (
		quantum *exactNumber
		err     error
	)
	if integer || len(numbers) == 0 {
		quantum, err = parseExactNumber("1")
	} else {
		quantum, err = exactQuantum(numbers...)
	}

	if err != nil {
		return numberSchedule{}, err
	}

	return numberSchedule{
		rules: rules, quantum: quantum, seeded: seeded && !hasEnum, hasEnum: hasEnum,
	}, nil
}

// materialize constructs one selected exact edge after its assignment is charged.
func (edge numberEdge) materialize() (*exactNumber, error) {
	if edge.base == nil {
		return nil, errors.New("schematest: numeric edge has no base")
	}

	if edge.increment == nil {
		return edge.base, nil
	}

	return addSignedExactNumbers(edge.base, edge.increment, edge.sign)
}

// exactDecimalTerm is one compact signed term in an edge-equality equation.
type exactDecimalTerm struct {
	coefficient *big.Int
	exponent    *big.Int
}

// numberEdgesEqual compares descriptors without constructing either neighbor.
func numberEdgesEqual(left, right numberEdge) (bool, error) {
	terms := make([]exactDecimalTerm, 0, numberEdgeTermCount)

	var err error

	terms, err = appendExactDecimalTerm(terms, left.base, 1)
	if err != nil {
		return false, err
	}

	terms, err = appendExactDecimalTerm(terms, left.increment, left.sign)
	if err != nil {
		return false, err
	}

	terms, err = appendExactDecimalTerm(terms, right.base, -1)
	if err != nil {
		return false, err
	}

	terms, err = appendExactDecimalTerm(terms, right.increment, -right.sign)
	if err != nil {
		return false, err
	}

	return exactDecimalTermsSumToZero(terms), nil
}

// appendExactDecimalTerm appends one compact finite-decimal term.
func appendExactDecimalTerm(
	terms []exactDecimalTerm,
	number *exactNumber,
	sign int64,
) ([]exactDecimalTerm, error) {
	if number == nil || number.numerator.Sign() == 0 || sign == 0 {
		return terms, nil
	}

	coefficient, exponent, err := number.finiteDecimal()
	if err != nil {
		return nil, err
	}

	if sign < 0 {
		coefficient.Neg(coefficient)
	}

	return append(terms, exactDecimalTerm{coefficient: coefficient, exponent: exponent}), nil
}

// exactDecimalTermsSumToZero folds sparse decimal terms upward without expanding exponent gaps.
func exactDecimalTermsSumToZero(terms []exactDecimalTerm) bool {
	if len(terms) == 0 {
		return true
	}

	sort.Slice(terms, func(left, right int) bool {
		return terms[left].exponent.Cmp(terms[right].exponent) < 0
	})

	coefficient := new(big.Int).Set(terms[0].coefficient)
	exponent := new(big.Int).Set(terms[0].exponent)

	for _, term := range terms[1:] {
		if term.exponent.Cmp(exponent) == 0 {
			coefficient.Add(coefficient, term.coefficient)

			continue
		}

		if coefficient.Sign() != 0 {
			gap := new(big.Int).Sub(term.exponent, exponent)

			zeros := decimalTrailingZeros(new(big.Int).Abs(coefficient))
			if gap.Cmp(new(big.Int).SetUint64(zeros)) > 0 {
				return false
			}

			coefficient.Quo(coefficient, decimalPower(gap.Uint64()))
		}

		coefficient.Add(coefficient, term.coefficient)
		exponent.Set(term.exponent)
	}

	return coefficient.Sign() == 0
}

// edgeAddressAt decodes one finite deterministic slot in schedule order.
//
//nolint:cyclop,gocognit // The enum and five ordered edge families have explicit address blocks.
func (schedule numberSchedule) edgeAddressAt(wanted uint64) (numberEdgeAddress, bool, uint64, error) {
	ruleCount := uint64(len(schedule.rules))

	if schedule.hasEnum {
		maxMembers := uint64(0)
		for _, rule := range schedule.rules {
			if uint64(len(rule.node.enum)) > maxMembers {
				maxMembers = uint64(len(rule.node.enum))
			}
		}

		if maxMembers != 0 && ruleCount > ^uint64(0)/maxMembers {
			return numberEdgeAddress{}, false, 0, errors.New("schematest: numeric enum edge domain overflows")
		}

		domain := ruleCount * maxMembers
		if wanted >= domain || maxMembers == 0 {
			return numberEdgeAddress{}, false, domain, nil
		}

		return numberEdgeAddress{
			ruleIndex: wanted / maxMembers,
			family:    numberEdgeEnum,
			local:     wanted % maxMembers,
		}, true, domain, nil
	}

	if ruleCount > (^uint64(0)-1)/numberRuleEdgeCount {
		return numberEdgeAddress{}, false, 0, errors.New("schematest: numeric edge domain overflows")
	}

	var hasMinimum, hasMaximum, hasMultiple, hasFormat bool
	for _, rule := range schedule.rules {
		hasMinimum = hasMinimum || rule.node.minimum != nil
		hasMaximum = hasMaximum || rule.node.maximum != nil
		hasMultiple = hasMultiple || rule.node.multipleOf != nil
		hasFormat = hasFormat || numberFormatHasEdges(rule.node.format)
	}

	domain := uint64(1)
	if hasMinimum {
		domain += ruleCount * numberBoundaryEdgeCount
	}

	if hasMaximum {
		domain += ruleCount * numberBoundaryEdgeCount
	}

	if hasMultiple {
		domain += ruleCount * numberMultipleEdgeCount
	}

	if hasFormat {
		domain += ruleCount * numberFormatEdgeCount
	}

	if wanted >= domain {
		return numberEdgeAddress{}, false, domain, nil
	}

	minimumSize := ruleCount * numberBoundaryEdgeCount
	if hasMinimum {
		if wanted < minimumSize {
			return numberEdgeAddress{
				ruleIndex: wanted / numberBoundaryEdgeCount,
				family:    numberEdgeMinimum,
				local:     wanted % numberBoundaryEdgeCount,
			}, true, domain, nil
		}

		wanted -= minimumSize
	}

	if hasMaximum {
		if wanted < minimumSize {
			return numberEdgeAddress{
				ruleIndex: wanted / numberBoundaryEdgeCount,
				family:    numberEdgeMaximum,
				local:     wanted % numberBoundaryEdgeCount,
			}, true, domain, nil
		}

		wanted -= minimumSize
	}

	if wanted == 0 {
		return numberEdgeAddress{family: numberEdgeZero}, true, domain, nil
	}

	wanted--

	multipleSize := ruleCount * numberMultipleEdgeCount
	if hasMultiple {
		if wanted < multipleSize {
			return numberEdgeAddress{
				ruleIndex: wanted / numberMultipleEdgeCount,
				family:    numberEdgeMultiple,
				local:     wanted % numberMultipleEdgeCount,
			}, true, domain, nil
		}

		wanted -= multipleSize
	}

	return numberEdgeAddress{
		ruleIndex: wanted / numberFormatEdgeCount,
		family:    numberEdgeFormat,
		local:     wanted % numberFormatEdgeCount,
	}, true, domain, nil
}

func numberFormatHasEdges(format schemaFormat) bool {
	switch format {
	case schemaFormatInt32, schemaFormatInt64, schemaFormatFloat, schemaFormatDouble:
		return true
	default:
		return false
	}
}

// edgeAtAddress evaluates only the selected deterministic schedule slot.
// evaluate is test instrumentation and is never called while inspecting deduplication metadata.
//
//nolint:cyclop // Each explicit edge family delegates to its direct local lookup.
func (schedule numberSchedule) edgeAtAddress(
	address numberEdgeAddress,
	evaluate func(numberEdgeAddress),
) (numberEdge, bool, error) {
	if evaluate != nil {
		evaluate(address)
	}

	if address.family == numberEdgeZero {
		zero, err := parseExactNumber("0")

		return numberEdge{base: zero}, err == nil, err
	}

	if address.ruleIndex >= uint64(len(schedule.rules)) {
		return numberEdge{}, false, nil
	}

	rule := schedule.rules[address.ruleIndex]

	switch address.family {
	case numberEdgeEnum:
		if address.local >= uint64(len(rule.node.enum)) {
			return numberEdge{}, false, nil
		}

		member := rule.node.enum[address.local]
		if member.value == nil {
			return numberEdge{}, false, errors.New("schematest: nil numeric enum member")
		}

		if member.value.kind != jsonNumber {
			return numberEdge{}, false, nil
		}

		return numberEdge{base: member.value.number}, true, nil
	case numberEdgeMinimum:
		return numberBoundaryEdgeAt(rule.node.minimum, schedule.quantum, true, address.local)
	case numberEdgeMaximum:
		return numberBoundaryEdgeAt(rule.node.maximum, schedule.quantum, false, address.local)
	case numberEdgeMultiple:
		return numberMultipleEdgeAt(rule.node.multipleOf, schedule.quantum, address.local)
	case numberEdgeFormat:
		return numberFormatEdgeAt(rule.node.format, address.local)
	default:
		return numberEdge{}, false, errors.New("schematest: unknown numeric edge family")
	}
}

func numberBoundaryEdgeAt(
	bound, quantum *exactNumber,
	minimum bool,
	local uint64,
) (numberEdge, bool, error) {
	if bound == nil || local >= numberBoundaryEdgeCount {
		return numberEdge{}, false, nil
	}

	if local == 0 {
		return numberEdge{base: bound}, true, nil
	}

	firstSign := int64(1)
	if !minimum {
		firstSign = -1
	}

	if local == numberSecondDirectedEdge {
		firstSign = -firstSign
	}

	return numberEdge{base: bound, increment: quantum, sign: firstSign}, true, nil
}

func numberMultipleEdgeAt(
	divisor, quantum *exactNumber,
	local uint64,
) (numberEdge, bool, error) {
	if divisor == nil || local >= numberMultipleEdgeCount {
		return numberEdge{}, false, nil
	}

	switch local {
	case 0:
		return numberEdge{base: divisor}, true, nil
	case 1:
		negative, err := negateExactNumber(divisor)

		return numberEdge{base: negative}, err == nil, err
	case numberSecondDirectedEdge:
		return numberEdge{base: divisor, increment: quantum, sign: 1}, true, nil
	default:
		return numberEdge{base: divisor, increment: quantum, sign: -1}, true, nil
	}
}

func numberFormatEdgeAt(format schemaFormat, local uint64) (numberEdge, bool, error) {
	if local >= numberFormatEdgeCount {
		return numberEdge{}, false, nil
	}

	var sources []string

	switch format {
	case schemaFormatInt32:
		sources = []string{"-2147483648", "-2147483649", "2147483647", "2147483648"}
	case schemaFormatInt64:
		sources = []string{
			"-9223372036854775808", "-9223372036854775809",
			"9223372036854775807", "9223372036854775808",
		}
	case schemaFormatFloat, schemaFormatDouble:
		limit, err := exactBinaryFloatOverflowLimit(format)
		if err != nil {
			return numberEdge{}, false, err
		}

		negativeLimit, err := negateExactNumber(limit)
		if err != nil {
			return numberEdge{}, false, err
		}

		one, err := parseExactNumber("1")
		if err != nil {
			return numberEdge{}, false, err
		}

		edges := []numberEdge{
			{base: negativeLimit, increment: one, sign: 1},
			{base: negativeLimit},
			{base: limit, increment: one, sign: -1},
			{base: limit},
		}

		return edges[local], true, nil
	default:
		return numberEdge{}, false, nil
	}

	number, err := parseExactNumber(sources[local])

	return numberEdge{base: number}, err == nil, err
}

// edgeAddressDuplicate separately checks first-occurrence metadata before one selected address.
func (schedule numberSchedule) edgeAddressDuplicate(current numberEdge, wanted uint64) (bool, error) {
	for earlierOrdinal := uint64(0); earlierOrdinal < wanted; earlierOrdinal++ {
		address, addressed, _, err := schedule.edgeAddressAt(earlierOrdinal)
		if err != nil {
			return false, err
		}

		if !addressed {
			break
		}

		earlier, exists, err := schedule.edgeAtAddress(address, nil)
		if err != nil {
			return false, err
		}

		if !exists {
			continue
		}

		equal, err := numberEdgesEqual(current, earlier)
		if err != nil {
			return false, err
		}

		if equal {
			return true, nil
		}
	}

	return false, nil
}

// eachEdge streams schedule metadata in exact address order.
func (schedule numberSchedule) eachEdge(visit func(numberEdge) (bool, error)) error {
	_, _, domain, err := schedule.edgeAddressAt(0)
	if err != nil {
		return err
	}

	for ordinal := uint64(0); ordinal < domain; ordinal++ {
		address, addressed, _, addressErr := schedule.edgeAddressAt(ordinal)
		if addressErr != nil {
			return addressErr
		}

		if !addressed {
			return errors.New("schematest: numeric edge address is outside its domain")
		}

		edge, exists, edgeErr := schedule.edgeAtAddress(address, nil)
		if edgeErr != nil {
			return edgeErr
		}

		if !exists {
			continue
		}

		stop, visitErr := visit(edge)
		if visitErr != nil || stop {
			return visitErr
		}
	}

	return nil
}

// edgeDuplicate reports exact first-occurrence replay without retaining witnesses.
func (schedule numberSchedule) edgeDuplicate(current numberEdge, currentIndex uint64) (bool, error) {
	var (
		index     uint64
		duplicate bool
	)

	err := schedule.eachEdge(func(earlier numberEdge) (bool, error) {
		if index == currentIndex {
			return true, nil
		}

		index++

		equal, compareErr := numberEdgesEqual(current, earlier)
		if compareErr != nil {
			return false, compareErr
		}

		duplicate = equal

		return duplicate, nil
	})

	return duplicate, err
}

// containsNumber reports whether a seeded candidate replays a deterministic edge.
func (schedule numberSchedule) containsNumber(number *exactNumber) (bool, error) {
	found := false
	err := schedule.eachEdge(func(edge numberEdge) (bool, error) {
		equal, compareErr := numberEdgesEqual(edge, numberEdge{base: number})
		if compareErr != nil {
			return false, compareErr
		}

		found = equal

		return found, nil
	})

	return found, err
}

// numberCandidateEmitter assigns streamed edge metadata without retaining candidates.
type numberCandidateEmitter struct {
	search   *search
	visit    rowVisit
	schedule numberSchedule
	index    uint64
}

// walk charges each first-occurrence edge before materializing it.
func (emitter *numberCandidateEmitter) walk() (bool, error) {
	var (
		complete bool
		walkErr  error
	)

	err := emitter.schedule.eachEdge(func(edge numberEdge) (bool, error) {
		duplicate, duplicateErr := emitter.schedule.edgeDuplicate(edge, emitter.index)
		emitter.index++

		if duplicateErr != nil {
			return false, duplicateErr
		}

		if duplicate {
			return false, nil
		}

		if assignErr := emitter.search.assign(); assignErr != nil {
			return false, assignErr
		}

		number, materializeErr := edge.materialize()
		if materializeErr != nil {
			return false, materializeErr
		}

		complete, walkErr = emitter.visit(&jsonValue{kind: jsonNumber, number: number})

		return complete || walkErr != nil, walkErr
	})
	if err != nil {
		return false, err
	}

	return complete, walkErr
}

// walkNumberDeterministic emits the finite exact schedule.
func (s *search) walkNumberDeterministic(
	schedule numberSchedule,
	visit rowVisit,
) (bool, error) {
	emitter := numberCandidateEmitter{search: s, visit: visit, schedule: schedule}

	return emitter.walk()
}

// negateExactNumber returns the exact additive inverse.
func negateExactNumber(number *exactNumber) (*exactNumber, error) {
	return newExactNumber(
		new(big.Int).Neg(number.numerator), number.denominator, number.exponent, number.scale,
	)
}

// addSignedExactNumbers adds or subtracts right according to sign.
func addSignedExactNumbers(left, right *exactNumber, sign int64) (*exactNumber, error) {
	if sign >= 0 {
		return addExactNumbers(left, right)
	}

	negative, err := negateExactNumber(right)
	if err != nil {
		return nil, err
	}

	return addExactNumbers(left, negative)
}

// addExactNumbers adds scaled rationals without binary floating point or fixed-width exponents.
func addExactNumbers(left, right *exactNumber) (*exactNumber, error) {
	if err := left.validate(); err != nil {
		return nil, err
	}

	if err := right.validate(); err != nil {
		return nil, err
	}

	exponent := new(big.Int).Set(left.exponent)
	if right.exponent.Cmp(exponent) < 0 {
		exponent.Set(right.exponent)
	}

	leftShift := new(big.Int).Sub(left.exponent, exponent)
	rightShift := new(big.Int).Sub(right.exponent, exponent)
	leftNumerator := new(big.Int).Mul(left.numerator, right.denominator)
	leftNumerator.Mul(leftNumerator, new(big.Int).Exp(big.NewInt(decimalRadix), leftShift, nil))
	rightNumerator := new(big.Int).Mul(right.numerator, left.denominator)
	rightNumerator.Mul(rightNumerator, new(big.Int).Exp(big.NewInt(decimalRadix), rightShift, nil))

	numerator := new(big.Int).Add(leftNumerator, rightNumerator)
	denominator := new(big.Int).Mul(left.denominator, right.denominator)

	scale := new(big.Int).Set(left.scale)
	if right.scale.Cmp(scale) > 0 {
		scale.Set(right.scale)
	}

	return newExactNumber(numerator, denominator, exponent, scale)
}
