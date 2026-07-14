package suite

import (
	"errors"
	"fmt"
	"math/big"
	"sort"

	"decode_and_validate_generator/pkg/internal/jsonvalue"
)

// decimalPrimeFive is the non-binary prime in finite decimal denominators.
const decimalPrimeFive = 5

// IntersectDomains returns the canonical constructive intersection of two Domains.
func (registry *DomainRegistry) IntersectDomains(leftID DomainID, rightID DomainID) (DomainID, error) {
	left, leftOK := registry.Domain(leftID)

	right, rightOK := registry.Domain(rightID)
	if !leftOK || !rightOK {
		return NoDomain, fmt.Errorf("intersect Domains %d and %d: Domain does not exist", leftID, rightID)
	}

	pair := canonicalDomainPair(leftID, rightID)
	if cached, ok := registry.IntersectionResults[pair]; ok {
		return cached, nil
	}

	result, err := registry.intersectDomains(leftID, left, rightID, right)
	if err != nil {
		return NoDomain, err
	}

	registry.IntersectionResults[pair] = result

	return result, nil
}

// canonicalDomainPair orders an unordered pair for intersection caching.
func canonicalDomainPair(left DomainID, right DomainID) DomainPair {
	if left > right {
		left, right = right, left
	}

	return DomainPair{First: left, Second: right}
}

// intersectDomains combines every productive kind after handling status identities.
func (registry *DomainRegistry) intersectDomains(
	leftID DomainID,
	left Domain,
	rightID DomainID,
	right Domain,
) (DomainID, error) {
	if domainIntersectionIsEmpty(leftID, left, rightID, right) {
		return EmptyDomainID, nil
	}

	if identity, ok := intersectDomainIdentity(leftID, rightID); ok {
		return identity, nil
	}

	if status, ok := registry.intersectExceptionalDomainStatuses(left, right); ok {
		return status, nil
	}

	result := Domain{
		Null:    intersectKindState(left.Null, right.Null),
		Boolean: intersectKindState(left.Boolean, right.Boolean),
		Status:  DomainProductive,
	}

	var err error

	result.Number, err = intersectNumbers(left.Number, right.Number)
	if err != nil {
		return NoDomain, err
	}

	result.String = intersectStrings(left.String, right.String)

	result.Array, err = registry.intersectArrays(left.Array, right.Array)
	if err != nil {
		return NoDomain, err
	}

	result.Object, err = registry.intersectObjects(left.Object, right.Object)
	if err != nil {
		return NoDomain, err
	}

	if err := eliminateContradictoryKinds(&result); err != nil {
		return NoDomain, fmt.Errorf("%w: %w", errUnconstructible, err)
	}

	if result.Status == DomainEmpty {
		return EmptyDomainID, nil
	}

	if err := registry.intersectEnums(&result, left.Enum, right.Enum); err != nil {
		return NoDomain, err
	}

	return registry.FindOrAddEquivalentDomain(result), nil
}

// domainIntersectionIsEmpty reports whether either side has Empty Domain semantics.
func domainIntersectionIsEmpty(leftID DomainID, left Domain, rightID DomainID, right Domain) bool {
	return leftID == EmptyDomainID || rightID == EmptyDomainID ||
		left.Status == DomainEmpty || right.Status == DomainEmpty
}

// intersectDomainIdentity applies the equal and Any JSON intersection identities.
func intersectDomainIdentity(leftID DomainID, rightID DomainID) (DomainID, bool) {
	if leftID == rightID {
		return leftID, true
	}

	if leftID == AnyJSONDomainID {
		return rightID, true
	}

	if rightID == AnyJSONDomainID {
		return leftID, true
	}

	return NoDomain, false
}

// intersectExceptionalDomainStatuses propagates unsupported and unconstructible statuses.
func (registry *DomainRegistry) intersectExceptionalDomainStatuses(left Domain, right Domain) (DomainID, bool) {
	if left.Status == DomainUnsupported || right.Status == DomainUnsupported {
		return registry.FindOrAddEquivalentDomain(Domain{Status: DomainUnsupported}), true
	}

	if left.Status == DomainUnconstructible || right.Status == DomainUnconstructible {
		return registry.FindOrAddEquivalentDomain(Domain{Status: DomainUnconstructible}), true
	}

	return NoDomain, false
}

// intersectKindState intersects one unconstrained scalar kind.
func intersectKindState(left KindState, right KindState) KindState {
	if left == KindExcluded || right == KindExcluded {
		return KindExcluded
	}

	return KindUnrestricted
}

// intersectNumbers combines exact numeric constraints.
func intersectNumbers(left NumberConstraints, right NumberConstraints) (NumberConstraints, error) {
	if left.State == KindExcluded || right.State == KindExcluded {
		return NumberConstraints{State: KindExcluded}, nil
	}

	result := NumberConstraints{
		State:        KindRestricted,
		IntegersOnly: left.IntegersOnly || right.IntegersOnly,
	}

	var err error

	result.Minimum, err = stricterMinimum(left.Minimum, right.Minimum)
	if err != nil {
		return NumberConstraints{}, err
	}

	result.Maximum, err = stricterMaximum(left.Maximum, right.Maximum)
	if err != nil {
		return NumberConstraints{}, err
	}

	result.MultipleOf, err = intersectMultiples(left.MultipleOf, right.MultipleOf)
	if err != nil {
		return NumberConstraints{}, err
	}

	productive, err := numberConstraintsAreProductive(result)
	if err != nil {
		return NumberConstraints{}, err
	}

	if !productive {
		return NumberConstraints{State: KindExcluded}, nil
	}

	return result, nil
}

// stricterMinimum returns the greater exact lower bound.
func stricterMinimum(left *NumberBound, right *NumberBound) (*NumberBound, error) {
	if left == nil {
		return cloneBound(right), nil
	}

	if right == nil {
		return cloneBound(left), nil
	}

	comparison := left.Value.Compare(right.Value)

	if comparison > 0 {
		return cloneBound(left), nil
	}

	if comparison < 0 {
		return cloneBound(right), nil
	}

	return &NumberBound{Value: left.Value, Exclusive: left.Exclusive || right.Exclusive}, nil
}

// stricterMaximum returns the smaller exact upper bound.
func stricterMaximum(left *NumberBound, right *NumberBound) (*NumberBound, error) {
	if left == nil {
		return cloneBound(right), nil
	}

	if right == nil {
		return cloneBound(left), nil
	}

	comparison := left.Value.Compare(right.Value)

	if comparison < 0 {
		return cloneBound(left), nil
	}

	if comparison > 0 {
		return cloneBound(right), nil
	}

	return &NumberBound{Value: left.Value, Exclusive: left.Exclusive || right.Exclusive}, nil
}

// intersectMultiples computes the exact rational least common multiple.
func intersectMultiples(left *jsonvalue.Number, right *jsonvalue.Number) (*jsonvalue.Number, error) {
	if left == nil {
		return cloneNumber(right), nil
	}

	if right == nil {
		return cloneNumber(left), nil
	}

	if left.Rational == nil || right.Rational == nil {
		return nil, fmt.Errorf("%w: multipleOf values are too large to intersect", errUnconstructible)
	}

	leftNumerator := new(big.Int).Abs(left.Rational.Num())
	rightNumerator := new(big.Int).Abs(right.Rational.Num())
	numeratorGCD := new(big.Int).GCD(nil, nil, leftNumerator, rightNumerator)
	numerator := new(big.Int).Mul(leftNumerator, rightNumerator)
	numerator.Quo(numerator, numeratorGCD)

	denominator := new(big.Int).GCD(nil, nil, left.Rational.Denom(), right.Rational.Denom())
	multiple := new(big.Rat).SetFrac(numerator, denominator)

	return exactJSONNumberFromRat(multiple)
}

// exactJSONNumberFromRat encodes a finite decimal rational exactly.
func exactJSONNumberFromRat(value *big.Rat) (*jsonvalue.Number, error) {
	denominator := new(big.Int).Set(value.Denom())
	twos := 0

	for denominator.Bit(0) == 0 {
		denominator.Rsh(denominator, 1)

		twos++
	}

	fives := 0
	five := big.NewInt(decimalPrimeFive)
	zero := new(big.Int)

	for {
		quotient, remainder := new(big.Int).QuoRem(denominator, five, new(big.Int))
		if remainder.Cmp(zero) != 0 {
			break
		}

		denominator = quotient
		fives++
	}

	if denominator.Cmp(big.NewInt(1)) != 0 {
		return nil, fmt.Errorf("%w: multipleOf intersection is not a finite JSON decimal", errUnconstructible)
	}

	places := max(twos, fives)

	parsed, err := jsonvalue.ParseNumber(value.FloatString(places))
	if err != nil {
		return nil, fmt.Errorf("encode multipleOf intersection: %w", err)
	}

	return &parsed, nil
}

// numberConstraintsAreProductive solves exact bound and lattice feasibility.
func numberConstraintsAreProductive(number NumberConstraints) (bool, error) {
	boundsAreProductive, err := numberBoundsAreProductive(number.Minimum, number.Maximum)
	if err != nil || !boundsAreProductive {
		return boundsAreProductive, err
	}

	if !number.IntegersOnly && number.MultipleOf == nil {
		return true, nil
	}

	if number.Minimum == nil || number.Maximum == nil {
		return true, nil
	}

	return boundedNumberLatticeIsProductive(number)
}

// numberBoundsAreProductive checks whether exact lower and upper bounds overlap.
func numberBoundsAreProductive(minimum *NumberBound, maximum *NumberBound) (bool, error) {
	if minimum == nil || maximum == nil {
		return true, nil
	}

	comparison := minimum.Value.Compare(maximum.Value)

	if comparison > 0 || comparison == 0 && (minimum.Exclusive || maximum.Exclusive) {
		return false, nil
	}

	return true, nil
}

// boundedNumberLatticeIsProductive checks for a permitted lattice point within both bounds.
func boundedNumberLatticeIsProductive(number NumberConstraints) (bool, error) {
	step, err := numberLatticeStep(number)
	if err != nil {
		return false, err
	}

	minimumFactor := minimumNumberLatticeFactor(number.Minimum, step)
	maximumFactor := maximumNumberLatticeFactor(number.Maximum, step)

	return minimumFactor.Cmp(maximumFactor) <= 0, nil
}

// numberLatticeStep returns the exact step satisfying integer and multiple constraints.
func numberLatticeStep(number NumberConstraints) (*big.Rat, error) {
	if number.Minimum.Value.Rational == nil || number.Maximum.Value.Rational == nil ||
		number.MultipleOf != nil && number.MultipleOf.Rational == nil {
		return nil, fmt.Errorf("%w: bounded numeric lattice is too large to solve", errUnconstructible)
	}

	step := big.NewRat(1, 1)
	if number.MultipleOf != nil {
		step.Set(number.MultipleOf.Rational)
	}

	if number.IntegersOnly && !step.IsInt() {
		step.SetInt(new(big.Int).Abs(step.Num()))
	}

	return step, nil
}

// minimumNumberLatticeFactor returns the first factor permitted by the lower bound.
func minimumNumberLatticeFactor(bound *NumberBound, step *big.Rat) *big.Int {
	factor := ceilRat(new(big.Rat).Quo(bound.Value.Rational, step))
	if bound.Exclusive && new(big.Rat).Mul(new(big.Rat).SetInt(factor), step).
		Cmp(bound.Value.Rational) == 0 {
		factor.Add(factor, big.NewInt(1))
	}

	return factor
}

// maximumNumberLatticeFactor returns the last factor permitted by the upper bound.
func maximumNumberLatticeFactor(bound *NumberBound, step *big.Rat) *big.Int {
	factor := floorRat(new(big.Rat).Quo(bound.Value.Rational, step))
	if bound.Exclusive && new(big.Rat).Mul(new(big.Rat).SetInt(factor), step).
		Cmp(bound.Value.Rational) == 0 {
		factor.Sub(factor, big.NewInt(1))
	}

	return factor
}

// floorRat returns the mathematical floor of an exact rational.
func floorRat(value *big.Rat) *big.Int {
	quotient := new(big.Int).Quo(value.Num(), value.Denom())
	if value.Sign() < 0 && new(big.Int).Rem(value.Num(), value.Denom()).Sign() != 0 {
		quotient.Sub(quotient, big.NewInt(1))
	}

	return quotient
}

// ceilRat returns the mathematical ceiling of an exact rational.
func ceilRat(value *big.Rat) *big.Int {
	floor := floorRat(value)
	if new(big.Rat).SetInt(floor).Cmp(value) < 0 {
		floor.Add(floor, big.NewInt(1))
	}

	return floor
}

// intersectStrings conjoins lengths, patterns, and formats.
func intersectStrings(left StringConstraints, right StringConstraints) StringConstraints {
	if left.State == KindExcluded || right.State == KindExcluded {
		return StringConstraints{State: KindExcluded}
	}

	result := StringConstraints{
		State:     KindRestricted,
		MinLength: max(left.MinLength, right.MinLength),
		MaxLength: smallerInt(left.MaxLength, right.MaxLength),
		Patterns:  append(append([]string(nil), left.Patterns...), right.Patterns...),
		Formats:   append(append([]string(nil), left.Formats...), right.Formats...),
	}
	if result.MaxLength != nil && result.MinLength > *result.MaxLength {
		return StringConstraints{State: KindExcluded}
	}

	return result
}

// intersectArrays combines lengths and recursively intersects item Domains.
func (registry *DomainRegistry) intersectArrays(
	left ArrayConstraints,
	right ArrayConstraints,
) (ArrayConstraints, error) {
	if left.State == KindExcluded || right.State == KindExcluded {
		return ArrayConstraints{State: KindExcluded}, nil
	}

	items, err := registry.IntersectDomains(left.Items, right.Items)
	if err != nil {
		return ArrayConstraints{}, err
	}

	result := ArrayConstraints{
		State:    KindRestricted,
		Items:    items,
		MinItems: max(left.MinItems, right.MinItems),
		MaxItems: smallerInt(left.MaxItems, right.MaxItems),
	}
	if items == EmptyDomainID {
		result.MaxItems = new(0)
	}

	if result.MaxItems != nil && result.MinItems > *result.MaxItems {
		return ArrayConstraints{State: KindExcluded}, nil
	}

	return result, nil
}

// intersectObjects applies both branches independently to every possible property name.
func (registry *DomainRegistry) intersectObjects(
	left ObjectConstraints,
	right ObjectConstraints,
) (ObjectConstraints, error) {
	if left.State == KindExcluded || right.State == KindExcluded {
		return ObjectConstraints{State: KindExcluded}, nil
	}

	additional, err := registry.IntersectDomains(left.Additional.Values, right.Additional.Values)
	if err != nil {
		return ObjectConstraints{}, err
	}

	properties, productive, err := registry.intersectObjectProperties(left, right)
	if err != nil {
		return ObjectConstraints{}, err
	}

	if !productive {
		return ObjectConstraints{State: KindExcluded}, nil
	}

	result := ObjectConstraints{
		State:      KindRestricted,
		Properties: properties,
		Additional: AdditionalProperties{Values: additional},
		MinProps:   max(left.MinProps, right.MinProps),
		MaxProps:   smallerInt(left.MaxProps, right.MaxProps),
	}
	if !registry.objectConstraintsAreProductive(result) {
		return ObjectConstraints{State: KindExcluded}, nil
	}

	return result, nil
}

// intersectObjectProperties intersects each explicit-or-additional property policy.
func (registry *DomainRegistry) intersectObjectProperties(
	left ObjectConstraints,
	right ObjectConstraints,
) ([]NamedProperty, bool, error) {
	leftProperties := propertyConstraintsByName(left.Properties)
	rightProperties := propertyConstraintsByName(right.Properties)
	names := objectPropertyNames(leftProperties, rightProperties)

	orderedNames := make([]string, 0, len(names))
	for name := range names {
		orderedNames = append(orderedNames, name)
	}

	sort.Strings(orderedNames)

	var result []NamedProperty

	for _, name := range orderedNames {
		leftValues := valuesForObjectName(name, leftProperties, left.Additional)
		rightValues := valuesForObjectName(name, rightProperties, right.Additional)

		values, err := registry.IntersectDomains(leftValues, rightValues)
		if err != nil {
			return nil, false, fmt.Errorf("intersect property %q: %w", name, err)
		}

		leftProperty := leftProperties[name]
		rightProperty := rightProperties[name]
		required := leftProperty.Required || rightProperty.Required

		if values == EmptyDomainID {
			if required {
				return nil, false, nil
			}

			result = append(result, NamedProperty{
				Name: name, State: PropertyForbidden, Values: EmptyDomainID,
			})

			continue
		}

		result = append(result, NamedProperty{
			Name: name, Required: required, State: PropertyAllowed, Values: values,
		})
	}

	return result, true, nil
}

// objectPropertyNames returns the union of explicit property names.
func objectPropertyNames(
	leftProperties map[string]NamedProperty,
	rightProperties map[string]NamedProperty,
) map[string]struct{} {
	names := make(map[string]struct{}, len(leftProperties)+len(rightProperties))
	for name := range leftProperties {
		names[name] = struct{}{}
	}

	for name := range rightProperties {
		names[name] = struct{}{}
	}

	return names
}

// valuesForObjectName applies one branch's explicit-or-additional property policy.
func valuesForObjectName(
	name string,
	properties map[string]NamedProperty,
	additional AdditionalProperties,
) DomainID {
	property, ok := properties[name]
	if !ok {
		return additional.Values
	}

	if property.State == PropertyForbidden {
		return EmptyDomainID
	}

	return property.Values
}

// objectConstraintsAreProductive checks required and achievable property counts.
func (registry *DomainRegistry) objectConstraintsAreProductive(object ObjectConstraints) bool {
	if object.MaxProps != nil && object.MinProps > *object.MaxProps {
		return false
	}

	required, availableNamed, requiredAreProductive := registry.productiveObjectPropertyCounts(object.Properties)
	if !requiredAreProductive {
		return false
	}

	if object.MaxProps != nil && required > *object.MaxProps {
		return false
	}

	return !registry.domainIsEmpty(object.Additional.Values) || object.MinProps <= availableNamed
}

// productiveObjectPropertyCounts counts usable required and named properties.
func (registry *DomainRegistry) productiveObjectPropertyCounts(
	properties []NamedProperty,
) (required int, available int, requiredAreProductive bool) {
	for _, property := range properties {
		if property.State == PropertyForbidden || registry.domainIsEmpty(property.Values) {
			if property.Required {
				return 0, 0, false
			}

			continue
		}

		available++

		if property.Required {
			required++
		}
	}

	return required, available, true
}

// domainIsEmpty reports Empty Domain semantics for a child DomainID.
func (registry *DomainRegistry) domainIsEmpty(id DomainID) bool {
	if id == AnyJSONDomainID {
		return false
	}

	if id == NoDomain || id == EmptyDomainID {
		return true
	}

	domain, ok := registry.Domain(id)

	return !ok || domain.Status == DomainEmpty
}

// intersectEnums applies finite semantic set intersection to merged kind constraints.
func (registry *DomainRegistry) intersectEnums(result *Domain, left *EnumSet, right *EnumSet) error {
	if left == nil && right == nil {
		return nil
	}

	candidates := enumCandidates(left, right)
	values := make([]jsonvalue.Value, 0, len(candidates))
	compiler := Compiler{Domains: registry}
	knownConstraints := cloneDomain(*result)
	knownConstraints.String.Patterns = nil
	knownConstraints.String.Formats = nil

	for _, value := range candidates {
		matches, err := compiler.valueFitsDomain(value, knownConstraints)
		if errors.Is(err, errOpaqueStringMembership) {
			// Exact occurrence evidence decides opaque descendants after Domain intersection.
			matches, err = true, nil
		}

		if err != nil {
			return err
		}

		if matches {
			values = append(values, value)
		}
	}

	if len(values) == 0 {
		*result = emptyDomain()

		return nil
	}

	*result = finiteDomain(values)

	return nil
}

// enumCandidates returns the finite candidate set in stable left order.
func enumCandidates(left *EnumSet, right *EnumSet) []jsonvalue.Value {
	if left == nil {
		return cloneJSONValues(right.Values)
	}

	if right == nil {
		return cloneJSONValues(left.Values)
	}

	values := make([]jsonvalue.Value, 0, min(len(left.Values), len(right.Values)))
	for _, leftValue := range left.Values {
		if enumContains(right, leftValue) {
			values = append(values, cloneJSONValue(leftValue))
		}
	}

	return values
}

// smallerInt returns a copy of the smaller optional upper bound.
func smallerInt(left *int, right *int) *int {
	if left == nil {
		if right == nil {
			return nil
		}

		return new(*right)
	}

	if right == nil || *left <= *right {
		return new(*left)
	}

	return new(*right)
}
