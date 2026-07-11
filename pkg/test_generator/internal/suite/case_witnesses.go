package suite

import (
	"fmt"
	"math/big"
	"strings"

	//nolint:depguard // Internal suite witness planning intentionally depends on internal/jsonvalue.
	"decode_and_validate_generator/pkg/test_generator/internal/jsonvalue"
)

// contextFailures creates dynamic failing partitions that depend on sibling passing rules.
func (planner *CasePlanner) contextFailures(
	constraint ConstraintPlan,
	context DomainID,
) ([]DomainID, error) {
	pass, ok := planner.Domains.Domain(constraint.Pass)
	if !ok {
		return nil, fmt.Errorf("constraint passing Domain %d does not exist", constraint.Pass)
	}

	integerFailure := constraint.Source.Keyword == "type" && pass.Number.IntegersOnly
	multipleFailure := constraint.Source.Keyword == "multipleOf" && pass.Number.MultipleOf != nil

	enumFailure := constraint.Source.Keyword == "enum" && pass.Enum != nil
	if !integerFailure && !multipleFailure && !enumFailure {
		return nil, nil
	}

	contextDomain, ok := planner.Domains.Domain(context)
	if !ok {
		return nil, fmt.Errorf("constraint context Domain %d does not exist", context)
	}

	if enumFailure {
		return planner.enumContextFailures(pass, contextDomain)
	}

	return planner.numberContextFailures(
		integerFailure,
		multipleFailure,
		pass.Number.MultipleOf,
		contextDomain.Number,
	)
}

// enumContextFailures creates finite enum outsiders that can satisfy the sibling context.
func (planner *CasePlanner) enumContextFailures(pass Domain, contextDomain Domain) ([]DomainID, error) {
	candidates, err := numberWitnessCandidates(contextDomain.Number)
	if err != nil {
		return nil, err
	}

	values := outsiderCandidates(pass.Enum)

	for _, candidate := range candidates {
		value := jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: candidate}
		if !enumContains(pass.Enum, value) {
			values = append(values, value)
		}
	}

	if contextDomain.String.State != KindExcluded && contextDomain.String.MinLength <= 1024 {
		values = append(values, jsonvalue.String(strings.Repeat("a", contextDomain.String.MinLength)))
	}

	result := make([]DomainID, 0, len(values))

	for _, value := range values {
		if !enumContains(pass.Enum, value) {
			failure := finiteDomain([]jsonvalue.Value{value})
			result = append(result, planner.Domains.FindOrAddEquivalentDomain(failure))
		}
	}

	return compactDomainIDs(result), nil
}

// numberContextFailures creates numeric witnesses that violate an integer or multiple-of rule.
func (planner *CasePlanner) numberContextFailures(
	integerFailure bool,
	multipleFailure bool,
	multipleOf *jsonvalue.Number,
	constraints NumberConstraints,
) ([]DomainID, error) {
	candidates, err := numberWitnessCandidates(constraints)
	if err != nil {
		return nil, err
	}

	selected := make([]jsonvalue.Number, 0, len(candidates))
	for _, candidate := range candidates {
		if integerFailure && candidate.Rational != nil && !candidate.Rational.IsInt() {
			selected = append(selected, candidate)
		} else if multipleFailure {
			fits, fitErr := fitsMultipleOf(candidate, multipleOf)
			if fitErr != nil {
				return nil, fitErr
			}

			if !fits {
				selected = append(selected, candidate)
			}
		}
	}

	return planner.finiteNumberFailures(selected), nil
}

// halfDenominator is the denominator for half-step witness values.
const halfDenominator = 2

// threeHalvesNumerator is the numerator for the 1.5 witness value.
const threeHalvesNumerator = 3

// numberWitnessCandidates returns basic and boundary-adjacent numeric witness candidates.
func numberWitnessCandidates(constraints NumberConstraints) ([]jsonvalue.Number, error) {
	result := append(basicNumbers(), fractionalCandidates()...)
	rationals := rationalWitnessCandidates(constraints)

	for _, rational := range rationals {
		candidate, err := exactJSONNumberFromRat(rational)
		if err != nil {
			return nil, err
		}

		result = append(result, *candidate)
	}

	return result, nil
}

// rationalWitnessCandidates returns rational values at and around numeric boundaries.
func rationalWitnessCandidates(constraints NumberConstraints) []*big.Rat {
	rationals := make([]*big.Rat, 0)

	if constraints.Minimum != nil && constraints.Minimum.Value.Rational != nil {
		minimum := constraints.Minimum.Value.Rational
		rationals = append(rationals,
			new(big.Rat).Set(minimum),
			new(big.Rat).Add(minimum, big.NewRat(1, halfDenominator)),
			new(big.Rat).Add(minimum, big.NewRat(1, 1)),
		)
	}

	if constraints.Maximum != nil && constraints.Maximum.Value.Rational != nil {
		maximum := constraints.Maximum.Value.Rational
		rationals = append(rationals,
			new(big.Rat).Set(maximum),
			new(big.Rat).Sub(maximum, big.NewRat(1, halfDenominator)),
			new(big.Rat).Sub(maximum, big.NewRat(1, 1)),
		)
	}

	if constraints.Minimum != nil && constraints.Maximum != nil &&
		constraints.Minimum.Value.Rational != nil && constraints.Maximum.Value.Rational != nil {
		midpoint := new(big.Rat).Add(
			constraints.Minimum.Value.Rational,
			constraints.Maximum.Value.Rational,
		)
		midpoint.Quo(midpoint, big.NewRat(halfDenominator, 1))
		rationals = append(rationals, midpoint, new(big.Rat).Add(midpoint, big.NewRat(1, halfDenominator)))
	}

	return rationals
}

// outsiderCandidates returns basic JSON values not contained in enum.
func outsiderCandidates(enum *EnumSet) []jsonvalue.Value {
	candidates := []jsonvalue.Value{
		jsonvalue.Null(),
		jsonvalue.Bool(false),
		jsonvalue.Bool(true),
		jsonvalue.String(""),
		jsonvalue.String("outsider"),
		jsonvalue.Array(nil),
	}

	for _, number := range basicNumbers() {
		candidates = append(candidates, jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: number})
	}

	result := candidates[:0]

	for _, candidate := range candidates {
		if enum == nil || !enumContains(enum, candidate) {
			result = append(result, candidate)
		}
	}

	return result
}

// nonMultipleCandidates returns basic numeric values that do not fit multiple.
func nonMultipleCandidates(multiple *jsonvalue.Number) ([]jsonvalue.Number, error) {
	result := make([]jsonvalue.Number, 0)

	for _, value := range append(basicNumbers(), fractionalCandidates()...) {
		fits, err := fitsMultipleOf(value, multiple)
		if err != nil {
			return nil, err
		}

		if !fits {
			result = append(result, value)
		}
	}

	if multiple != nil && multiple.Rational != nil {
		value := new(big.Rat).Quo(multiple.Rational, big.NewRat(halfDenominator, 1))

		candidate, err := exactJSONNumberFromRat(value)
		if err != nil {
			return nil, err
		}

		fits, err := fitsMultipleOf(*candidate, multiple)
		if err != nil {
			return nil, err
		}

		if !fits {
			result = append(result, *candidate)
		}
	}

	return result, nil
}

// basicNumbers returns the whole-number candidates used for finite partitions.
func basicNumbers() []jsonvalue.Number {
	return []jsonvalue.Number{
		{Lexeme: "0", Rational: new(big.Rat)},
		{Lexeme: "1", Rational: big.NewRat(1, 1)},
		{Lexeme: "-1", Rational: big.NewRat(-1, 1)},
		{Lexeme: "2", Rational: big.NewRat(halfDenominator, 1)},
	}
}

// fractionalCandidates returns the fractional candidates used for finite partitions.
func fractionalCandidates() []jsonvalue.Number {
	return []jsonvalue.Number{
		{Lexeme: "0.5", Rational: big.NewRat(1, halfDenominator)},
		{Lexeme: "-0.5", Rational: big.NewRat(-1, halfDenominator)},
		{Lexeme: "1.5", Rational: big.NewRat(threeHalvesNumerator, halfDenominator)},
	}
}

// finiteNumberFailures registers finite number-only failure Domains.
func (planner *CasePlanner) finiteNumberFailures(numbers []jsonvalue.Number) []DomainID {
	result := make([]DomainID, 0, len(numbers))

	for _, number := range numbers {
		value := jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: number}
		failure := finiteDomain([]jsonvalue.Value{value})
		id := planner.Domains.FindOrAddEquivalentDomain(failure)
		result = append(result, id)
	}

	return compactDomainIDs(result)
}

// compactDomainIDs removes sentinel and duplicate Domain IDs while preserving order.
func compactDomainIDs(ids []DomainID) []DomainID {
	seen := make(map[DomainID]struct{}, len(ids))

	result := make([]DomainID, 0, len(ids))
	for _, id := range ids {
		if id == NoDomain || id == EmptyDomainID {
			continue
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		result = append(result, id)
	}

	return result
}

// caseSet collects unique CasePlans while coalescing accepted Domains.
type caseSet struct {
	cases        []CasePlan
	seen         map[string]struct{}
	seenAccepted map[DomainID]struct{}
}

// newCaseSet returns an empty set of unique CasePlans.
func newCaseSet() *caseSet {
	return &caseSet{
		seen:         make(map[string]struct{}),
		seenAccepted: make(map[DomainID]struct{}),
	}
}

// add records plan unless it duplicates an existing case or accepted Domain.
func (set *caseSet) add(plan CasePlan) {
	if plan.Values == NoDomain || plan.Values == EmptyDomainID {
		return
	}

	if plan.Expect == ExpectAccepted {
		if _, duplicate := set.seenAccepted[plan.Values]; duplicate {
			return
		}

		set.seenAccepted[plan.Values] = struct{}{}
	}

	key := fmt.Sprintf(
		"%d\x00%d\x00%s\x00%s\x00%s",
		plan.Expect,
		plan.Values,
		plan.Name,
		plan.Source.Pointer,
		plan.Source.Keyword,
	)
	if _, duplicate := set.seen[key]; duplicate {
		return
	}

	set.seen[key] = struct{}{}
	set.cases = append(set.cases, plan)
}

// caseName returns the display name for a planned case.
func caseName(label string, pointer string, keyword string) string {
	name := label
	if pointer != "" {
		name += " at " + pointer
	}

	if keyword != "" {
		name += " / " + keyword
	}

	return name
}
