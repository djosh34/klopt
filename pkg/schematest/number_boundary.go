package schematest

import (
	"errors"
	"math/big"
)

// numberRuleValueCount is the count of scale-bearing numeric rule values.
const numberRuleValueCount = 3

// numberSchedule is the exact numeric conjunction for one pinned composition view.
type numberSchedule struct {
	rules   []activeNumberRule
	quantum *exactNumber
	seeded  bool
}

// newNumberSchedule compiles one active conjunction and its shared quantum.
//
//nolint:cyclop // Active kind, scale, and frontier policy are compiled in one pass.
func newNumberSchedule(rules []activeNumberRule) (numberSchedule, error) {
	if len(rules) == 0 {
		return numberSchedule{}, errors.New("schematest: number schedule has no active schemas")
	}

	integer := false
	seeded := false

	numbers := make([]*exactNumber, 0, len(rules)*numberRuleValueCount)
	for _, rule := range rules {
		if rule.node == nil || rule.node.schemaShape == nil {
			return numberSchedule{}, errors.New("schematest: number boundary schema has no shape")
		}

		integer = integer || rule.node.kind == schemaInteger

		seeded = seeded || nodeHasNumberSearchRules(rule.node)
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

	return numberSchedule{rules: rules, quantum: quantum, seeded: seeded}, nil
}

// numberCandidateEmitter incrementally deduplicates and assigns exact scalar choices.
type numberCandidateEmitter struct {
	search *search
	visit  rowVisit
	seen   []*exactNumber
}

// emit charges and visits one first-occurrence exact edge.
func (emitter *numberCandidateEmitter) emit(number *exactNumber) (bool, error) {
	for _, earlier := range emitter.seen {
		equal, err := number.compare(earlier)
		if err != nil {
			return false, err
		}

		if equal == 0 {
			return false, nil
		}
	}

	if err := emitter.search.assign(); err != nil {
		return false, err
	}

	emitter.seen = append(emitter.seen, number)

	return emitter.visit(&jsonValue{kind: jsonNumber, number: number})
}

// makeAndEmit checks the global stop before constructing the next exact neighbor.
func (emitter *numberCandidateEmitter) makeAndEmit(
	makeNumber func() (*exactNumber, error),
) (bool, error) {
	if emitter.search.steps == emitter.search.maxSteps {
		return false, errMaxSteps
	}

	number, err := makeNumber()
	if err != nil {
		return false, err
	}

	return emitter.emit(number)
}

// walkNumberDeterministic emits global boundary phases in canonical active-rule order.
func (s *search) walkNumberDeterministic(
	schedule numberSchedule,
	visit rowVisit,
) (bool, error) {
	emitter := numberCandidateEmitter{search: s, visit: visit}

	return emitter.walkDeterministic(schedule)
}

// walkDeterministic emits all finite phases in their required global order.
//
//nolint:cyclop // The required numeric phases are deliberately explicit and ordered.
func (emitter *numberCandidateEmitter) walkDeterministic(
	schedule numberSchedule,
) (bool, error) {
	for _, rule := range schedule.rules {
		complete, err := emitter.walkBoundary(rule.node.minimum, schedule.quantum, true)
		if err != nil || complete {
			return complete, err
		}
	}

	for _, rule := range schedule.rules {
		complete, err := emitter.walkBoundary(rule.node.maximum, schedule.quantum, false)
		if err != nil || complete {
			return complete, err
		}
	}

	complete, err := emitter.makeAndEmit(func() (*exactNumber, error) {
		return parseExactNumber("0")
	})
	if err != nil || complete {
		return complete, err
	}

	for _, rule := range schedule.rules {
		complete, err = emitter.walkMultiple(rule.node.multipleOf, schedule.quantum)
		if err != nil || complete {
			return complete, err
		}
	}

	for _, rule := range schedule.rules {
		complete, err = emitter.walkFormat(rule.node.format)
		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}

// walkBoundary emits a bound followed by its directed exact neighbors.
func (emitter *numberCandidateEmitter) walkBoundary(
	bound, quantum *exactNumber,
	minimum bool,
) (bool, error) {
	if bound == nil {
		return false, nil
	}

	complete, err := emitter.emit(bound)
	if err != nil || complete {
		return complete, err
	}

	firstSign := int64(1)
	if !minimum {
		firstSign = -1
	}

	for _, sign := range []int64{firstSign, -firstSign} {
		complete, err = emitter.makeAndEmit(func() (*exactNumber, error) {
			return addSignedExactNumbers(bound, quantum, sign)
		})
		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}

// walkMultiple emits the divisor, its negative, and directed neighbors.
func (emitter *numberCandidateEmitter) walkMultiple(
	divisor, quantum *exactNumber,
) (bool, error) {
	if divisor == nil {
		return false, nil
	}

	factories := []func() (*exactNumber, error){
		func() (*exactNumber, error) { return divisor, nil },
		func() (*exactNumber, error) { return negateExactNumber(divisor) },
		func() (*exactNumber, error) { return addSignedExactNumbers(divisor, quantum, 1) },
		func() (*exactNumber, error) { return addSignedExactNumbers(divisor, quantum, -1) },
	}
	for _, factory := range factories {
		complete, err := emitter.makeAndEmit(factory)
		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}

// walkFormat emits the active format's exact inside and outside edges.
func (emitter *numberCandidateEmitter) walkFormat(format schemaFormat) (bool, error) {
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
		return emitter.walkFloatFormat(format)
	default:
		return false, nil
	}

	for _, source := range sources {
		value := source

		complete, err := emitter.makeAndEmit(func() (*exactNumber, error) {
			return parseExactNumber(value)
		})
		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}

// walkFloatFormat emits exact finite-overflow edges incrementally.
func (emitter *numberCandidateEmitter) walkFloatFormat(format schemaFormat) (bool, error) {
	if emitter.search.steps == emitter.search.maxSteps {
		return false, errMaxSteps
	}

	limit, err := exactBinaryFloatOverflowLimit(format)
	if err != nil {
		return false, err
	}

	factories := []func() (*exactNumber, error){
		func() (*exactNumber, error) {
			negativeLimit, negateErr := negateExactNumber(limit)
			if negateErr != nil {
				return nil, negateErr
			}

			one, oneErr := parseExactNumber("1")
			if oneErr != nil {
				return nil, oneErr
			}

			return addSignedExactNumbers(negativeLimit, one, 1)
		},
		func() (*exactNumber, error) { return negateExactNumber(limit) },
		func() (*exactNumber, error) {
			one, oneErr := parseExactNumber("1")
			if oneErr != nil {
				return nil, oneErr
			}

			return addSignedExactNumbers(limit, one, -1)
		},
		func() (*exactNumber, error) { return limit, nil },
	}
	for _, factory := range factories {
		complete, walkErr := emitter.makeAndEmit(factory)
		if walkErr != nil || complete {
			return complete, walkErr
		}
	}

	return false, nil
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
