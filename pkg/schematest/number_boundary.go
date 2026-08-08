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
)

// errNumberEdgeStop ends metadata replay without exposing a search error.
var errNumberEdgeStop = errors.New("schematest: stop numeric edge metadata")

// numberSchedule is the exact numeric conjunction for one pinned composition view.
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

// eachEdge streams schedule metadata in exact required order.
//
//nolint:cyclop,gocognit // Enum and the five deterministic phases are deliberately explicit.
func (schedule numberSchedule) eachEdge(visit func(numberEdge) (bool, error)) error {
	if schedule.hasEnum {
		for _, rule := range schedule.rules {
			for _, member := range rule.node.enum {
				if member.value == nil {
					return errors.New("schematest: nil numeric enum member")
				}

				if member.value.kind != jsonNumber {
					continue
				}

				stop, err := visit(numberEdge{base: member.value.number})
				if err != nil || stop {
					return err
				}
			}
		}

		return nil
	}

	for _, rule := range schedule.rules {
		err := eachNumberBoundaryEdge(rule.node.minimum, schedule.quantum, true, visit)
		if errors.Is(err, errNumberEdgeStop) {
			return nil
		}

		if err != nil {
			return err
		}
	}

	for _, rule := range schedule.rules {
		err := eachNumberBoundaryEdge(rule.node.maximum, schedule.quantum, false, visit)
		if errors.Is(err, errNumberEdgeStop) {
			return nil
		}

		if err != nil {
			return err
		}
	}

	zero, err := parseExactNumber("0")
	if err != nil {
		return err
	}

	if stop, visitErr := visit(numberEdge{base: zero}); visitErr != nil || stop {
		return visitErr
	}

	for _, rule := range schedule.rules {
		err := eachNumberMultipleEdge(rule.node.multipleOf, schedule.quantum, visit)
		if errors.Is(err, errNumberEdgeStop) {
			return nil
		}

		if err != nil {
			return err
		}
	}

	for _, rule := range schedule.rules {
		err := eachNumberFormatEdge(rule.node.format, visit)
		if errors.Is(err, errNumberEdgeStop) {
			return nil
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// eachNumberBoundaryEdge streams one bound and its directed neighbors as metadata.
func eachNumberBoundaryEdge(
	bound, quantum *exactNumber,
	minimum bool,
	visit func(numberEdge) (bool, error),
) error {
	if bound == nil {
		return nil
	}

	if stop, err := visit(numberEdge{base: bound}); err != nil {
		return err
	} else if stop {
		return errNumberEdgeStop
	}

	firstSign := int64(1)
	if !minimum {
		firstSign = -1
	}

	for _, sign := range []int64{firstSign, -firstSign} {
		stop, err := visit(numberEdge{base: bound, increment: quantum, sign: sign})
		if err != nil {
			return err
		}

		if stop {
			return errNumberEdgeStop
		}
	}

	return nil
}

// eachNumberMultipleEdge streams the divisor and its directed edges as metadata.
func eachNumberMultipleEdge(
	divisor, quantum *exactNumber,
	visit func(numberEdge) (bool, error),
) error {
	if divisor == nil {
		return nil
	}

	negative, err := negateExactNumber(divisor)
	if err != nil {
		return err
	}

	edges := []numberEdge{
		{base: divisor},
		{base: negative},
		{base: divisor, increment: quantum, sign: 1},
		{base: divisor, increment: quantum, sign: -1},
	}
	for _, edge := range edges {
		stop, visitErr := visit(edge)
		if visitErr != nil {
			return visitErr
		}

		if stop {
			return errNumberEdgeStop
		}
	}

	return nil
}

// eachNumberFormatEdge streams exact format-edge metadata.
func eachNumberFormatEdge(format schemaFormat, visit func(numberEdge) (bool, error)) error {
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
		return eachNumberFloatFormatEdge(format, visit)
	default:
		return nil
	}

	for _, source := range sources {
		number, err := parseExactNumber(source)
		if err != nil {
			return err
		}

		if stop, visitErr := visit(numberEdge{base: number}); visitErr != nil {
			return visitErr
		} else if stop {
			return errNumberEdgeStop
		}
	}

	return nil
}

// eachNumberFloatFormatEdge streams exact finite-overflow metadata.
func eachNumberFloatFormatEdge(
	format schemaFormat,
	visit func(numberEdge) (bool, error),
) error {
	limit, err := exactBinaryFloatOverflowLimit(format)
	if err != nil {
		return err
	}

	negativeLimit, err := negateExactNumber(limit)
	if err != nil {
		return err
	}

	one, err := parseExactNumber("1")
	if err != nil {
		return err
	}

	edges := []numberEdge{
		{base: negativeLimit, increment: one, sign: 1},
		{base: negativeLimit},
		{base: limit, increment: one, sign: -1},
		{base: limit},
	}
	for _, edge := range edges {
		stop, visitErr := visit(edge)
		if visitErr != nil {
			return visitErr
		}

		if stop {
			return errNumberEdgeStop
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

// numberDeterministicCandidates collects the finite schedule for focused golden tests.
func numberDeterministicCandidates(node *schemaNode) ([]*jsonValue, error) {
	schedule, err := newNumberSchedule([]activeNumberRule{{node: node}})
	if err != nil {
		return nil, err
	}

	state := &search{maxSteps: ^uint64(0)}
	candidates := make([]*jsonValue, 0)

	_, err = state.walkNumberDeterministic(schedule, func(value *jsonValue) (bool, error) {
		candidates = append(candidates, value)

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return candidates, nil
}

// numberBoundaryQuantum returns one for integer schemas and the finest authored numeric unit otherwise.
func numberBoundaryQuantum(node *schemaNode) (*exactNumber, error) {
	schedule, err := newNumberSchedule([]activeNumberRule{{node: node}})
	if err != nil {
		return nil, err
	}

	return schedule.quantum, nil
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
