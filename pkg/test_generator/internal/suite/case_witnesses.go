package suite

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"unicode/utf8"

	"decode_and_validate_generator/pkg/internal/jsonvalue"
)

// witnessDecimalRadix keeps derived witnesses representable as JSON decimals.
const witnessDecimalRadix = 10

// witnessMidpointParts divides an exact interval into halves.
const witnessMidpointParts = 2

// Architecture note: witness planning derives exact values from the effective
// occurrence context. The basicNumbers, fractionalCandidates,
// rationalWitnessCandidates, numberWitnessCandidates, nonMultipleCandidates,
// half-step, and outsiderCandidates paths were deleted; implementation LOC now
// points at one context solver. Opaque string languages remain intentionally
// unsupported without a trusted valid oracle. Invalid oracle values establish
// whole-occurrence rejection, never atomic pattern/format blame. Numeric format
// names remain open annotations because the generator and pinned validators do
// not share consensus enforcement semantics.

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
		return planner.enumContextFailuresAt(pass, contextDomain, planner.rootUse)
	}

	return planner.numberContextFailures(
		integerFailure,
		multipleFailure,
		pass.Number.MultipleOf,
		contextDomain,
	)
}

// enumContextFailures creates finite enum outsiders that can satisfy the sibling context.
func (planner *CasePlanner) enumContextFailures(pass Domain, contextDomain Domain) ([]DomainID, error) {
	return planner.enumContextFailuresAt(pass, contextDomain, nil)
}

// enumContextFailuresAt creates outsiders while retaining exact descendant occurrences.
func (planner *CasePlanner) enumContextFailuresAt(
	pass Domain,
	contextDomain Domain,
	use *schemaUse,
) ([]DomainID, error) {
	limit := 1
	if pass.Enum != nil {
		limit += len(pass.Enum.Values)
	}

	values, err := planner.contextValuesAt(contextDomain, limit, make(map[DomainID]bool), use, false)
	if err != nil {
		return nil, err
	}

	for _, value := range values {
		if !enumContains(pass.Enum, value) {
			failure := planner.Domains.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{value}))

			return []DomainID{failure}, nil
		}
	}

	return nil, nil
}

// contextValues enumerates at most limit deterministic values admitted by a
// modeled sibling Domain. Opaque string languages intentionally yield no guess.
func (planner *CasePlanner) contextValues(
	domain Domain,
	limit int,
	active map[DomainID]bool,
) ([]jsonvalue.Value, error) {
	return planner.contextValuesAt(domain, limit, active, nil, false)
}

// contextValuesAt enumerates modeled values with optional exact-occurrence oracle access.
func (planner *CasePlanner) contextValuesAt(
	domain Domain,
	limit int,
	active map[DomainID]bool,
	use *schemaUse,
	useOracle bool,
) ([]jsonvalue.Value, error) {
	if limit <= 0 {
		return nil, nil
	}

	if useOracle && use != nil && use.examples.ValidDeclared &&
		(domain.Enum != nil || hasOpaqueStringConstraints(domain.String)) {
		values := generationExampleValues(use.examples.Valid)

		return values[:min(limit, len(values))], nil
	}

	if domain.Enum != nil {
		return cloneJSONValues(domain.Enum.Values[:min(limit, len(domain.Enum.Values))]), nil
	}

	collector := &contextValueCollector{limit: limit, values: make([]jsonvalue.Value, 0, limit)}
	if err := appendContextScalarValues(collector, domain); err != nil {
		return nil, err
	}

	if err := planner.appendContextContainerValuesAt(collector, domain, active, use); err != nil {
		return nil, err
	}

	return collector.values, nil
}

// generationExampleValues clones retained occurrence values in declaration order.
func generationExampleValues(examples []GenerationExample) []jsonvalue.Value {
	values := make([]jsonvalue.Value, 0, len(examples))
	for _, example := range examples {
		values = append(values, cloneJSONValue(example.Value))
	}

	return values
}

// contextValueCollector appends distinct values up to one fixed bound.
type contextValueCollector struct {
	limit  int
	values []jsonvalue.Value
}

// append records one distinct value while capacity remains.
func (collector *contextValueCollector) append(value jsonvalue.Value) {
	if collector.remaining() > 0 && !jsonValuesContain(collector.values, value) {
		collector.values = append(collector.values, value)
	}
}

// remaining reports the number of values still needed.
func (collector *contextValueCollector) remaining() int {
	return collector.limit - len(collector.values)
}

// appendContextScalarValues adds modeled scalar values in stable kind order.
func appendContextScalarValues(collector *contextValueCollector, domain Domain) error {
	if domain.Null != KindExcluded {
		collector.append(jsonvalue.Null())
	}

	if domain.Boolean != KindExcluded {
		collector.append(jsonvalue.Bool(false))
		collector.append(jsonvalue.Bool(true))
	}

	if domain.Number.State != KindExcluded {
		numbers, err := contextNumbers(domain.Number, collector.remaining())
		if err != nil {
			return err
		}

		for _, number := range numbers {
			collector.append(jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: number})
		}
	}

	if domain.String.State != KindExcluded && !hasOpaqueStringConstraints(domain.String) {
		for _, value := range contextStrings(domain.String, collector.remaining()) {
			collector.append(jsonvalue.String(value))
		}
	}

	return nil
}

// appendContextContainerValues adds modeled arrays and objects in stable order.
func (planner *CasePlanner) appendContextContainerValues(
	collector *contextValueCollector,
	domain Domain,
	active map[DomainID]bool,
) error {
	return planner.appendContextContainerValuesAt(collector, domain, active, nil)
}

// appendContextContainerValuesAt retains exact child occurrence seams.
func (planner *CasePlanner) appendContextContainerValuesAt(
	collector *contextValueCollector,
	domain Domain,
	active map[DomainID]bool,
	use *schemaUse,
) error {
	if domain.Array.State != KindExcluded {
		arrays, err := planner.contextArraysAt(domain.Array, collector.remaining(), active, use)
		if err != nil {
			return err
		}

		for _, value := range arrays {
			collector.append(value)
		}
	}

	if domain.Object.State != KindExcluded {
		objects, err := planner.contextObjectsAt(domain.Object, collector.remaining(), active, use)
		if err != nil {
			return err
		}

		for _, value := range objects {
			collector.append(value)
		}
	}

	return nil
}

// contextNumbers returns exact values from a numeric interval or lattice.
func contextNumbers(constraints NumberConstraints, limit int) ([]jsonvalue.Number, error) {
	if limit <= 0 {
		return nil, nil
	}

	if constraints.IntegersOnly || constraints.MultipleOf != nil {
		return latticeContextNumbers(constraints, limit)
	}

	return continuousContextNumbers(constraints, limit)
}

// latticeContextNumbers enumerates exact sibling lattice points.
func latticeContextNumbers(constraints NumberConstraints, limit int) ([]jsonvalue.Number, error) {
	step, err := latticeStep(constraints)
	if err != nil {
		return nil, err
	}

	minimum, maximum, err := latticeFactorBounds(constraints, step)
	if err != nil {
		return nil, err
	}

	result := make([]jsonvalue.Number, 0, limit)
	for _, factor := range contextLatticeFactors(minimum, maximum, limit) {
		numbers, numberErr := numberFromRational(new(big.Rat).Mul(step, new(big.Rat).SetInt(factor)))
		if numberErr != nil {
			return nil, numberErr
		}

		result = append(result, numbers[0])
	}

	return result, nil
}

// contextLatticeFactors selects bounded factors nearest zero first.
func contextLatticeFactors(minimum *big.Int, maximum *big.Int, limit int) []*big.Int {
	if minimum.Sign() > 0 {
		return ascendingLatticeFactors(minimum, maximum, limit)
	}

	if maximum.Sign() < 0 {
		return descendingLatticeFactors(minimum, maximum, limit)
	}

	return zeroCenteredLatticeFactors(minimum, maximum, limit)
}

// ascendingLatticeFactors enumerates a positive bounded interval.
func ascendingLatticeFactors(minimum *big.Int, maximum *big.Int, limit int) []*big.Int {
	result := make([]*big.Int, 0, limit)
	for factor := new(big.Int).Set(minimum); factor.Cmp(maximum) <= 0 && len(result) < limit; {
		result = append(result, new(big.Int).Set(factor))
		factor.Add(factor, big.NewInt(1))
	}

	return result
}

// descendingLatticeFactors enumerates a negative bounded interval nearest zero first.
func descendingLatticeFactors(minimum *big.Int, maximum *big.Int, limit int) []*big.Int {
	result := make([]*big.Int, 0, limit)
	for factor := new(big.Int).Set(maximum); factor.Cmp(minimum) >= 0 && len(result) < limit; {
		result = append(result, new(big.Int).Set(factor))
		factor.Sub(factor, big.NewInt(1))
	}

	return result
}

// zeroCenteredLatticeFactors alternates positive and negative magnitudes.
func zeroCenteredLatticeFactors(minimum *big.Int, maximum *big.Int, limit int) []*big.Int {
	result := make([]*big.Int, 0, limit)

	result = append(result, new(big.Int))
	for magnitude := big.NewInt(1); len(result) < limit; magnitude.Add(magnitude, big.NewInt(1)) {
		if magnitude.Cmp(maximum) <= 0 {
			result = append(result, new(big.Int).Set(magnitude))
		}

		if len(result) == limit {
			break
		}

		negative := new(big.Int).Neg(magnitude)
		if negative.Cmp(minimum) >= 0 {
			result = append(result, negative)
		}

		if magnitude.Cmp(maximum) > 0 && negative.Cmp(minimum) < 0 {
			break
		}
	}

	return result
}

// continuousContextNumbers enumerates exact points in a non-lattice interval.
func continuousContextNumbers(constraints NumberConstraints, limit int) ([]jsonvalue.Number, error) {
	base, err := interiorNumber(constraints)
	if err != nil {
		return nil, err
	}

	first, err := exactJSONNumberFromRat(base)
	if err != nil {
		return nil, err
	}

	result := []jsonvalue.Number{*first}
	if exactSingletonInterval(constraints) {
		return result, nil
	}

	denominator := big.NewInt(witnessDecimalRadix)
	for len(result) < limit {
		delta := new(big.Rat).Inv(new(big.Rat).SetInt(denominator))
		for _, sign := range []int64{1, -1} {
			candidate := new(big.Rat).Add(base, new(big.Rat).Mul(delta, big.NewRat(sign, 1)))

			number, numberErr := exactJSONNumberFromRat(candidate)
			if numberErr != nil {
				return nil, numberErr
			}

			fits, fitErr := numberFits(*number, constraints)
			if fitErr != nil {
				return nil, fitErr
			}

			if fits {
				result = append(result, *number)
				if len(result) == limit {
					break
				}
			}
		}

		denominator.Mul(denominator, big.NewInt(witnessDecimalRadix))
	}

	return result, nil
}

// contextStrings returns distinct modeled strings within exact length bounds.
func contextStrings(constraints StringConstraints, limit int) []string {
	if !contextStringRangeIsUsable(constraints, limit) {
		return nil
	}

	length := constraints.MinLength
	if length == 0 {
		if constraints.MaxLength != nil && *constraints.MaxLength == 0 {
			return []string{""}
		}

		length = 1
	}

	result := make([]string, 0, limit)

	const preferred = "abcdefghijklmnopqrstuvwxyz0123456789"
	for _, character := range preferred[:min(limit, len(preferred))] {
		result = append(result, strings.Repeat(string(character), length))
	}

	for character := rune(0); character <= utf8.MaxRune && len(result) < limit; character++ {
		if !utf8.ValidRune(character) || strings.ContainsRune(preferred, character) {
			continue
		}

		result = append(result, strings.Repeat(string(character), length))
	}

	return result
}

// contextStringRangeIsUsable bounds deterministic string materialization.
func contextStringRangeIsUsable(constraints StringConstraints, limit int) bool {
	return limit > 0 && constraints.MinLength <= 4096 &&
		(constraints.MaxLength == nil || constraints.MinLength <= *constraints.MaxLength)
}

// contextArrays enumerates bounded array shapes and child combinations.
func (planner *CasePlanner) contextArrays(
	constraints ArrayConstraints,
	limit int,
	active map[DomainID]bool,
) ([]jsonvalue.Value, error) {
	return planner.contextArraysAt(constraints, limit, active, nil)
}

// contextArraysAt enumerates arrays using the exact item occurrence when available.
func (planner *CasePlanner) contextArraysAt(
	constraints ArrayConstraints,
	limit int,
	active map[DomainID]bool,
	use *schemaUse,
) ([]jsonvalue.Value, error) {
	if !contextArrayRangeIsUsable(constraints, limit, active) {
		return nil, nil
	}

	result := make([]jsonvalue.Value, 0, limit)
	if constraints.MinItems == 0 {
		result = append(result, jsonvalue.Array(nil))
		if arrayOnlyAllowsEmpty(constraints) || len(result) == limit {
			return result, nil
		}
	}

	children, err := planner.contextArrayChildrenAt(constraints.Items, limit, active, childItemsUse(use))
	if err != nil || len(children) == 0 {
		return result, err
	}

	return enumerateContextArrays(result, children, constraints, limit), nil
}

// childItemsUse returns the exact item occurrence.
func childItemsUse(use *schemaUse) *schemaUse {
	if use == nil {
		return nil
	}

	return use.items
}

// contextArrayRangeIsUsable rejects recursive, empty, and unsafe materialization ranges.
func contextArrayRangeIsUsable(
	constraints ArrayConstraints,
	limit int,
	active map[DomainID]bool,
) bool {
	if limit <= 0 || active[constraints.Items] || constraints.MinItems > exactStructuralCollectionLimit {
		return false
	}

	return constraints.MaxItems == nil || constraints.MinItems <= *constraints.MaxItems
}

// arrayOnlyAllowsEmpty reports an exact zero maximum.
func arrayOnlyAllowsEmpty(constraints ArrayConstraints) bool {
	return constraints.MaxItems != nil && *constraints.MaxItems == 0
}

// contextArrayChildren enumerates exact item values without recursive loops.
func (planner *CasePlanner) contextArrayChildren(
	items DomainID,
	limit int,
	active map[DomainID]bool,
) ([]jsonvalue.Value, error) {
	return planner.contextArrayChildrenAt(items, limit, active, nil)
}

// contextArrayChildrenAt enumerates one exact item occurrence.
func (planner *CasePlanner) contextArrayChildrenAt(
	items DomainID,
	limit int,
	active map[DomainID]bool,
	use *schemaUse,
) ([]jsonvalue.Value, error) {
	child, ok := planner.Domains.Domain(items)
	if !ok {
		return nil, fmt.Errorf("array item Domain %d does not exist", items)
	}

	active[items] = true
	children, err := planner.contextValuesAt(child, limit, active, use, true)
	delete(active, items)

	return children, err
}

// enumerateContextArrays combines bounded counts and child alternatives.
func enumerateContextArrays(
	result []jsonvalue.Value,
	children []jsonvalue.Value,
	constraints ArrayConstraints,
	limit int,
) []jsonvalue.Value {
	minimumCount := max(1, constraints.MinItems)

	additionalCounts := limit - len(result) - 1

	maximumCount := minimumCount
	if additionalCounts > 0 {
		maximumCount = exactStructuralCollectionLimit
		if additionalCounts <= exactStructuralCollectionLimit-minimumCount {
			maximumCount = minimumCount + additionalCounts
		}
	}

	if constraints.MaxItems != nil && maximumCount > *constraints.MaxItems {
		maximumCount = *constraints.MaxItems
	}

	maximumCount = min(maximumCount, exactStructuralCollectionLimit)

	for count := minimumCount; count <= maximumCount && len(result) < limit; count++ {
		result = append(result, contextArrayVariations(children, count, limit-len(result))...)
	}

	return result
}

// contextArrayVariations enumerates one fixed-size Cartesian item product.
func contextArrayVariations(children []jsonvalue.Value, count int, limit int) []jsonvalue.Value {
	indexes := make([]int, count)
	result := make([]jsonvalue.Value, 0, limit)

	for len(result) < limit {
		items := make([]jsonvalue.Value, count)
		for index, childIndex := range indexes {
			items[index] = children[childIndex]
		}

		result = append(result, jsonvalue.Array(items))

		position := 0
		for position < len(indexes) {
			indexes[position]++
			if indexes[position] < len(children) {
				break
			}

			indexes[position] = 0
			position++
		}

		if position == len(indexes) {
			break
		}
	}

	return result
}

// contextObjects enumerates required/minimum object shapes and child combinations.
func (planner *CasePlanner) contextObjects(
	constraints ObjectConstraints,
	limit int,
	active map[DomainID]bool,
) ([]jsonvalue.Value, error) {
	return planner.contextObjectsAt(constraints, limit, active, nil)
}

// contextObjectsAt enumerates objects using exact property occurrences when available.
func (planner *CasePlanner) contextObjectsAt(
	constraints ObjectConstraints,
	limit int,
	active map[DomainID]bool,
	use *schemaUse,
) ([]jsonvalue.Value, error) {
	if limit <= 0 {
		return nil, nil
	}

	if constraints.MinProps > exactStructuralCollectionLimit {
		return nil, nil
	}

	shape := &contextObjectShape{constraints: constraints, limit: limit, use: use}
	if err := planner.collectContextObjectProperties(shape, active); err != nil {
		return nil, err
	}

	if err := planner.fillContextObjectMinimum(shape, active); err != nil {
		return nil, err
	}

	if !shape.isFeasible() {
		return nil, nil
	}

	return planner.contextObjectVariations(shape, active)
}

// contextObjectChoice pairs a property with its deterministic child values.
type contextObjectChoice struct {
	property NamedProperty
	values   []jsonvalue.Value
}

// contextObjectShape stores one feasible base shape and its variation seams.
type contextObjectShape struct {
	constraints ObjectConstraints
	limit       int
	use         *schemaUse
	members     []jsonvalue.Member
	variants    [][]jsonvalue.Value
	optional    []contextObjectChoice
	blocked     bool
}

// collectContextObjectProperties selects required and minimum declared properties.
func (planner *CasePlanner) collectContextObjectProperties(
	shape *contextObjectShape,
	active map[DomainID]bool,
) error {
	properties := append([]NamedProperty(nil), shape.constraints.Properties...)
	sort.Slice(properties, func(left int, right int) bool { return properties[left].Name < properties[right].Name })

	if err := planner.collectRequiredContextProperties(shape, properties, active); err != nil || shape.blocked {
		return err
	}

	return planner.collectOptionalContextProperties(shape, properties, active)
}

// collectRequiredContextProperties adds every required declared property.
func (planner *CasePlanner) collectRequiredContextProperties(
	shape *contextObjectShape,
	properties []NamedProperty,
	active map[DomainID]bool,
) error {
	for _, property := range properties {
		if property.State == PropertyForbidden || !property.Required {
			continue
		}

		choice, err := planner.contextObjectChoice(
			property,
			shape.limit,
			active,
			contextPropertyUse(shape.use, property.Name),
		)
		if err != nil {
			return err
		}

		if len(choice.values) == 0 {
			shape.blocked = true

			return nil
		}

		shape.addPresent(choice)
	}

	return nil
}

// collectOptionalContextProperties fills the minimum then records optional seams.
func (planner *CasePlanner) collectOptionalContextProperties(
	shape *contextObjectShape,
	properties []NamedProperty,
	active map[DomainID]bool,
) error {
	for _, property := range properties {
		if property.State == PropertyForbidden || property.Required {
			continue
		}

		choice, err := planner.contextObjectChoice(
			property,
			shape.limit,
			active,
			contextPropertyUse(shape.use, property.Name),
		)
		if err != nil {
			return err
		}

		if len(choice.values) == 0 {
			continue
		}

		if len(shape.members) < shape.constraints.MinProps {
			shape.addPresent(choice)
		} else {
			shape.optional = append(shape.optional, choice)
		}
	}

	return nil
}

// contextObjectChoice enumerates one declared property's values.
func (planner *CasePlanner) contextObjectChoice(
	property NamedProperty,
	limit int,
	active map[DomainID]bool,
	use *schemaUse,
) (contextObjectChoice, error) {
	values, err := planner.contextChildValuesAt(property.Values, limit, active, use)

	return contextObjectChoice{property: property, values: values}, err
}

// contextPropertyUse returns the exact declared or additional property occurrence.
func contextPropertyUse(use *schemaUse, name string) *schemaUse {
	if use == nil {
		return nil
	}

	propertyUse := use.property(name)
	if propertyUse != nil {
		return propertyUse
	}

	return use.additional
}

// addPresent adds a property to the base object and its variation list.
func (shape *contextObjectShape) addPresent(choice contextObjectChoice) {
	shape.members = append(shape.members, jsonvalue.Member{
		Name: choice.property.Name, Value: choice.values[0],
	})
	shape.variants = append(shape.variants, choice.values)
}

// contextChildValues enumerates one child Domain without recursive loops.
func (planner *CasePlanner) contextChildValues(
	id DomainID,
	limit int,
	active map[DomainID]bool,
) ([]jsonvalue.Value, error) {
	return planner.contextChildValuesAt(id, limit, active, nil)
}

// contextChildValuesAt enumerates one exact child occurrence.
func (planner *CasePlanner) contextChildValuesAt(
	id DomainID,
	limit int,
	active map[DomainID]bool,
	use *schemaUse,
) ([]jsonvalue.Value, error) {
	child, ok := planner.Domains.Domain(id)
	if !ok {
		return nil, fmt.Errorf("child Domain %d does not exist", id)
	}

	if active[id] {
		return nil, nil
	}

	active[id] = true
	values, err := planner.contextValuesAt(child, limit, active, use, true)
	delete(active, id)

	return values, err
}

// fillContextObjectMinimum adds schema-valued additional properties as needed.
func (planner *CasePlanner) fillContextObjectMinimum(
	shape *contextObjectShape,
	active map[DomainID]bool,
) error {
	for len(shape.members) < shape.constraints.MinProps &&
		shape.constraints.Additional.Values != EmptyDomainID {
		values, err := planner.contextChildValuesAt(
			shape.constraints.Additional.Values,
			shape.limit,
			active,
			contextAdditionalUse(shape.use),
		)
		if err != nil {
			return err
		}

		if len(values) == 0 {
			shape.blocked = true

			return nil
		}

		shape.members = append(shape.members, jsonvalue.Member{
			Name: unusedMemberName(shape.constraints.Properties, shape.members), Value: values[0],
		})
		shape.variants = append(shape.variants, values)
	}

	return nil
}

// isFeasible reports whether the base shape satisfies object counts.
func (shape *contextObjectShape) isFeasible() bool {
	return !shape.blocked && len(shape.members) >= shape.constraints.MinProps &&
		(shape.constraints.MaxProps == nil || len(shape.members) <= *shape.constraints.MaxProps)
}

// contextObjectVariations builds the base, optional, additional, and value variants.
func (planner *CasePlanner) contextObjectVariations(
	shape *contextObjectShape,
	active map[DomainID]bool,
) ([]jsonvalue.Value, error) {
	base, err := jsonvalue.Object(shape.members)
	if err != nil {
		return nil, err
	}

	result := []jsonvalue.Value{base}

	result, err = appendOptionalContextObjects(result, shape)
	if err != nil {
		return nil, err
	}

	result, err = planner.appendAdditionalContextObjects(result, shape, active)
	if err != nil {
		return nil, err
	}

	result, err = appendRemainingOptionalContextObjects(result, shape)
	if err != nil {
		return nil, err
	}

	result, err = appendCombinedOptionalContextObjects(result, shape)
	if err != nil {
		return nil, err
	}

	return appendContextObjectValueVariants(result, shape)
}

// appendOptionalContextObjects adds one optional property at a time.
func appendOptionalContextObjects(
	result []jsonvalue.Value,
	shape *contextObjectShape,
) ([]jsonvalue.Value, error) {
	if shape.constraints.MaxProps != nil && len(shape.members) >= *shape.constraints.MaxProps {
		return result, nil
	}

	for _, entry := range shape.optional {
		value, err := optionalContextObject(shape.members, entry, entry.values[0])
		if err != nil {
			return nil, err
		}

		result = append(result, value)
		if len(result) == shape.limit {
			return result, nil
		}
	}

	return result, nil
}

// appendRemainingOptionalContextObjects adds later values after structural seams.
func appendRemainingOptionalContextObjects(
	result []jsonvalue.Value,
	shape *contextObjectShape,
) ([]jsonvalue.Value, error) {
	if len(result) >= shape.limit ||
		shape.constraints.MaxProps != nil && len(shape.members) >= *shape.constraints.MaxProps {
		return result, nil
	}

	for _, entry := range shape.optional {
		for _, propertyValue := range entry.values[1:] {
			value, err := optionalContextObject(shape.members, entry, propertyValue)
			if err != nil {
				return nil, err
			}

			result = append(result, value)
			if len(result) == shape.limit {
				return result, nil
			}
		}
	}

	return result, nil
}

// optionalContextObject builds one optional-property variation.
func optionalContextObject(
	members []jsonvalue.Member,
	entry contextObjectChoice,
	propertyValue jsonvalue.Value,
) (jsonvalue.Value, error) {
	candidate := append([]jsonvalue.Member(nil), members...)
	candidate = append(candidate, jsonvalue.Member{Name: entry.property.Name, Value: propertyValue})

	return jsonvalue.Object(candidate)
}

// appendCombinedOptionalContextObjects enumerates bounded multi-property products.
func appendCombinedOptionalContextObjects(
	result []jsonvalue.Value,
	shape *contextObjectShape,
) ([]jsonvalue.Value, error) {
	maximum := min(len(shape.optional), exactStructuralCollectionLimit-len(shape.members))
	if shape.constraints.MaxProps != nil {
		maximum = min(maximum, *shape.constraints.MaxProps-len(shape.members))
	}

	for count := 2; count <= maximum && len(result) < shape.limit; count++ {
		propertyIndexes := make([]int, count)
		for index := range propertyIndexes {
			propertyIndexes[index] = index
		}

		for {
			var err error

			result, err = appendOptionalContextProduct(result, shape, propertyIndexes)
			if err != nil || len(result) == shape.limit {
				return result, err
			}

			if !advanceContextCombination(propertyIndexes, len(shape.optional)) {
				break
			}
		}
	}

	return result, nil
}

// appendOptionalContextProduct enumerates the values for one property subset.
func appendOptionalContextProduct(
	result []jsonvalue.Value,
	shape *contextObjectShape,
	propertyIndexes []int,
) ([]jsonvalue.Value, error) {
	valueIndexes := make([]int, len(propertyIndexes))

	for len(result) < shape.limit {
		candidate := append([]jsonvalue.Member(nil), shape.members...)
		for index, propertyIndex := range propertyIndexes {
			entry := shape.optional[propertyIndex]
			candidate = append(candidate, jsonvalue.Member{
				Name: entry.property.Name, Value: entry.values[valueIndexes[index]],
			})
		}

		value, err := jsonvalue.Object(candidate)
		if err != nil {
			return nil, err
		}

		result = append(result, value)

		position := 0
		for position < len(valueIndexes) {
			valueIndexes[position]++

			entry := shape.optional[propertyIndexes[position]]
			if valueIndexes[position] < len(entry.values) {
				break
			}

			valueIndexes[position] = 0
			position++
		}

		if position == len(valueIndexes) {
			break
		}
	}

	return result, nil
}

// advanceContextCombination advances one increasing property-index subset.
func advanceContextCombination(indexes []int, total int) bool {
	for position := len(indexes) - 1; position >= 0; position-- {
		if indexes[position] >= total-len(indexes)+position {
			continue
		}

		indexes[position]++
		for next := position + 1; next < len(indexes); next++ {
			indexes[next] = indexes[next-1] + 1
		}

		return true
	}

	return false
}

// appendAdditionalContextObjects adds distinct additional names and values.
func (planner *CasePlanner) appendAdditionalContextObjects(
	result []jsonvalue.Value,
	shape *contextObjectShape,
	active map[DomainID]bool,
) ([]jsonvalue.Value, error) {
	if len(result) >= shape.limit || shape.constraints.Additional.Values == EmptyDomainID ||
		shape.constraints.MaxProps != nil && len(shape.members) >= *shape.constraints.MaxProps {
		return result, nil
	}

	values, err := planner.contextChildValuesAt(
		shape.constraints.Additional.Values,
		shape.limit-len(result),
		active,
		contextAdditionalUse(shape.use),
	)
	if err != nil {
		return nil, err
	}

	for _, value := range values {
		candidate := append([]jsonvalue.Member(nil), shape.members...)
		candidate = append(candidate, jsonvalue.Member{
			Name: unusedMemberName(shape.constraints.Properties, candidate), Value: value,
		})

		object, objectErr := jsonvalue.Object(candidate)
		if objectErr != nil {
			return nil, objectErr
		}

		result = append(result, object)
		if len(result) == shape.limit {
			break
		}
	}

	return result, nil
}

// contextAdditionalUse returns the exact additional-property occurrence.
func contextAdditionalUse(use *schemaUse) *schemaUse {
	if use == nil {
		return nil
	}

	return use.additional
}

// appendContextObjectValueVariants varies one present property's value at a time.
func appendContextObjectValueVariants(
	result []jsonvalue.Value,
	shape *contextObjectShape,
) ([]jsonvalue.Value, error) {
	for memberIndex, values := range shape.variants {
		for valueIndex := 1; valueIndex < len(values) && len(result) < shape.limit; valueIndex++ {
			candidate := append([]jsonvalue.Member(nil), shape.members...)
			candidate[memberIndex].Value = values[valueIndex]

			value, err := jsonvalue.Object(candidate)
			if err != nil {
				return nil, err
			}

			result = append(result, value)
		}
	}

	return result, nil
}

// numberContextFailures creates numeric witnesses that violate an integer or multiple-of rule.
func (planner *CasePlanner) numberContextFailures(
	integerFailure bool,
	multipleFailure bool,
	multipleOf *jsonvalue.Number,
	context Domain,
) ([]DomainID, error) {
	selected, err := exactNumberFailures(context, integerFailure, multipleFailure, multipleOf)
	if err != nil {
		return nil, err
	}

	return planner.finiteNumberFailures(selected), nil
}

// exactNumberFailures derives a witness from the complete sibling context.
func exactNumberFailures(
	context Domain,
	integerFailure bool,
	multipleFailure bool,
	multipleOf *jsonvalue.Number,
) ([]jsonvalue.Number, error) {
	if context.Enum != nil {
		for _, value := range context.Enum.Values {
			if value.Kind != jsonvalue.KindNumber {
				continue
			}

			violates, err := numberViolatesRule(value.Number, integerFailure, multipleFailure, multipleOf)
			if err != nil {
				return nil, err
			}

			if violates {
				return []jsonvalue.Number{value.Number}, nil
			}
		}

		return nil, nil
	}

	constraints := context.Number
	if constraints.State == KindExcluded {
		return nil, nil
	}

	if constraints.IntegersOnly || constraints.MultipleOf != nil {
		return latticeRuleFailure(constraints, integerFailure, multipleFailure, multipleOf)
	}

	return continuousRuleFailure(constraints, integerFailure, multipleFailure, multipleOf)
}

// numberContextImpliesRule proves a finite, singleton, or lattice context is a subset.
func numberContextImpliesRule(
	context Domain,
	integerTarget bool,
	multipleOf *jsonvalue.Number,
) bool {
	if context.Enum != nil {
		return finiteNumbersImplyRule(context.Enum.Values, integerTarget, multipleOf)
	}

	constraints := context.Number
	if constraints.State == KindExcluded {
		return true
	}

	if exactSingletonInterval(constraints) {
		return exactNumberImpliesRule(constraints.Minimum.Value, integerTarget, multipleOf)
	}

	if !constraints.IntegersOnly && constraints.MultipleOf == nil {
		return false
	}

	step, err := latticeStep(constraints)
	if err != nil {
		return false
	}

	target := big.NewRat(1, 1)

	if !integerTarget {
		if multipleOf == nil || multipleOf.Rational == nil {
			return false
		}

		target.Set(multipleOf.Rational)
	}

	return new(big.Rat).Quo(step, target).Denom().Cmp(big.NewInt(1)) == 0
}

// finiteNumbersImplyRule checks every numeric member of a finite context.
func finiteNumbersImplyRule(
	values []jsonvalue.Value,
	integerTarget bool,
	multipleOf *jsonvalue.Number,
) bool {
	for _, value := range values {
		if value.Kind == jsonvalue.KindNumber && !exactNumberImpliesRule(value.Number, integerTarget, multipleOf) {
			return false
		}
	}

	return true
}

// exactNumberImpliesRule checks one exact numeric value without losing errors.
func exactNumberImpliesRule(
	value jsonvalue.Number,
	integerTarget bool,
	multipleOf *jsonvalue.Number,
) bool {
	violates, err := numberViolatesRule(value, integerTarget, !integerTarget, multipleOf)

	return err == nil && !violates
}

// latticeRuleFailure proves whether the sibling lattice has a point outside the target lattice.
func latticeRuleFailure(
	constraints NumberConstraints,
	integerFailure bool,
	multipleFailure bool,
	multipleOf *jsonvalue.Number,
) ([]jsonvalue.Number, error) {
	step, err := latticeStep(constraints)
	if err != nil {
		return nil, err
	}

	target := big.NewRat(1, 1)

	if multipleFailure {
		if multipleOf == nil || multipleOf.Rational == nil {
			return nil, errors.New("multipleOf target is too large to solve exactly")
		}

		target.Set(multipleOf.Rational)
	} else if !integerFailure {
		return nil, nil
	}

	period := new(big.Rat).Quo(step, target).Denom()
	if period.Cmp(big.NewInt(1)) == 0 {
		return nil, nil
	}

	minimum, maximum, err := latticeFactorBounds(constraints, step)
	if err != nil {
		return nil, err
	}

	factor := nearestNonMultipleFactor(minimum, maximum, period)
	if factor == nil {
		return nil, nil
	}

	return numberFromRational(new(big.Rat).Mul(step, new(big.Rat).SetInt(factor)))
}

// nearestNonMultipleFactor selects the smallest-magnitude admitted factor
// outside the target period.
func nearestNonMultipleFactor(minimum *big.Int, maximum *big.Int, period *big.Int) *big.Int {
	if minimum.Sign() <= 0 && maximum.Sign() >= 0 {
		return nonzeroFactorAroundZero(minimum, maximum)
	}

	if maximum.Sign() < 0 {
		return directionalNonMultipleFactor(maximum, minimum, big.NewInt(-1), period)
	}

	return directionalNonMultipleFactor(minimum, maximum, big.NewInt(1), period)
}

// nonzeroFactorAroundZero returns the nearest admitted nonzero factor.
func nonzeroFactorAroundZero(minimum *big.Int, maximum *big.Int) *big.Int {
	one := big.NewInt(1)
	if one.Cmp(maximum) <= 0 {
		return one
	}

	negativeOne := big.NewInt(-1)
	if negativeOne.Cmp(minimum) >= 0 {
		return negativeOne
	}

	return nil
}

// directionalNonMultipleFactor advances once when the nearest endpoint is on-period.
func directionalNonMultipleFactor(
	start *big.Int,
	limit *big.Int,
	direction *big.Int,
	period *big.Int,
) *big.Int {
	factor := new(big.Int).Set(start)
	if new(big.Int).Mod(new(big.Int).Set(factor), period).Sign() == 0 {
		factor.Add(factor, direction)
	}

	beyond := direction.Sign() > 0 && factor.Cmp(limit) > 0

	beyond = beyond || direction.Sign() < 0 && factor.Cmp(limit) < 0
	if beyond {
		return nil
	}

	return factor
}

// continuousRuleFailure chooses a point inside the exact interval and perturbs
// a target-lattice point by a finite-decimal fraction of the target step.
func continuousRuleFailure(
	constraints NumberConstraints,
	integerFailure bool,
	multipleFailure bool,
	multipleOf *jsonvalue.Number,
) ([]jsonvalue.Number, error) {
	base, err := interiorNumber(constraints)
	if err != nil {
		return nil, err
	}

	direct, violates, err := exactRuleCandidate(base, integerFailure, multipleFailure, multipleOf)
	if err != nil || violates {
		return direct, err
	}

	unit, err := targetRuleStep(multipleFailure, multipleOf)
	if err != nil {
		return nil, err
	}

	if exactSingletonInterval(constraints) {
		return nil, nil
	}

	perturbed, ok, err := decimalPerturbation(base, unit, constraints)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, nil
	}

	return numberFromRational(perturbed)
}

// exactRuleCandidate checks one exact continuous-context value.
func exactRuleCandidate(
	value *big.Rat,
	integerFailure bool,
	multipleFailure bool,
	multipleOf *jsonvalue.Number,
) ([]jsonvalue.Number, bool, error) {
	candidate, err := exactJSONNumberFromRat(value)
	if err != nil {
		return nil, false, err
	}

	violates, err := numberViolatesRule(*candidate, integerFailure, multipleFailure, multipleOf)
	if err != nil || !violates {
		return nil, false, err
	}

	return []jsonvalue.Number{*candidate}, true, nil
}

// targetRuleStep returns the exact target lattice step.
func targetRuleStep(multipleFailure bool, multipleOf *jsonvalue.Number) (*big.Rat, error) {
	if !multipleFailure {
		return big.NewRat(1, 1), nil
	}

	if multipleOf == nil || multipleOf.Rational == nil {
		return nil, errors.New("multipleOf target is too large to solve exactly")
	}

	return new(big.Rat).Set(multipleOf.Rational), nil
}

// exactSingletonInterval reports a one-point inclusive numeric context.
func exactSingletonInterval(constraints NumberConstraints) bool {
	return constraints.Minimum != nil && constraints.Maximum != nil &&
		!constraints.Minimum.Exclusive && !constraints.Maximum.Exclusive &&
		constraints.Minimum.Value.Rational != nil && constraints.Maximum.Value.Rational != nil &&
		constraints.Minimum.Value.Rational.Cmp(constraints.Maximum.Value.Rational) == 0
}

// interiorNumber returns one exact number admitted by a continuous interval.
func interiorNumber(constraints NumberConstraints) (*big.Rat, error) {
	if constraints.Minimum != nil && constraints.Minimum.Value.Rational == nil ||
		constraints.Maximum != nil && constraints.Maximum.Value.Rational == nil {
		return nil, errors.New("number bounds are too large to solve exactly")
	}

	if constraints.Minimum != nil && constraints.Maximum != nil {
		return twoSidedInteriorNumber(constraints.Minimum, constraints.Maximum)
	}

	if constraints.Minimum != nil {
		return new(big.Rat).Add(constraints.Minimum.Value.Rational, big.NewRat(1, 1)), nil
	}

	if constraints.Maximum != nil {
		return new(big.Rat).Sub(constraints.Maximum.Value.Rational, big.NewRat(1, 1)), nil
	}

	return new(big.Rat), nil
}

// twoSidedInteriorNumber selects a midpoint or the sole inclusive point.
func twoSidedInteriorNumber(minimum *NumberBound, maximum *NumberBound) (*big.Rat, error) {
	comparison := minimum.Value.Rational.Cmp(maximum.Value.Rational)
	if comparison == 0 {
		if minimum.Exclusive || maximum.Exclusive {
			return nil, errors.New("number interval has no interior value")
		}

		return new(big.Rat).Set(minimum.Value.Rational), nil
	}

	if comparison > 0 {
		return nil, errors.New("number interval is reversed")
	}

	return new(big.Rat).Quo(
		new(big.Rat).Add(minimum.Value.Rational, maximum.Value.Rational),
		big.NewRat(witnessMidpointParts, 1),
	), nil
}

// decimalPerturbation moves within bounds by target/10^n, which cannot remain
// on the target lattice and always has a finite JSON decimal representation.
func decimalPerturbation(
	base *big.Rat,
	target *big.Rat,
	constraints NumberConstraints,
) (*big.Rat, bool, error) {
	denominator := big.NewInt(witnessDecimalRadix)
	for {
		delta := new(big.Rat).Quo(target, new(big.Rat).SetInt(denominator))
		for _, sign := range []int64{1, -1} {
			candidate := new(big.Rat).Add(base, new(big.Rat).Mul(delta, big.NewRat(sign, 1)))

			number, err := exactJSONNumberFromRat(candidate)
			if err != nil {
				return nil, false, err
			}

			fits, err := numberFits(*number, constraints)
			if err != nil {
				return nil, false, err
			}

			if fits {
				return candidate, true, nil
			}
		}

		denominator.Mul(denominator, big.NewInt(witnessDecimalRadix))
	}
}

// numberViolatesRule checks the one atomic numeric rule being isolated.
func numberViolatesRule(
	value jsonvalue.Number,
	integerFailure bool,
	multipleFailure bool,
	multipleOf *jsonvalue.Number,
) (bool, error) {
	if integerFailure {
		return !fitsIntegerConstraint(value, NumberConstraints{IntegersOnly: true}), nil
	}

	if !multipleFailure {
		return false, nil
	}

	fits, err := fitsMultipleOf(value, multipleOf)

	return !fits, err
}

// numberFromRational encodes one exact finite-decimal witness.
func numberFromRational(value *big.Rat) ([]jsonvalue.Number, error) {
	candidate, err := exactJSONNumberFromRat(value)
	if err != nil {
		return nil, err
	}

	return []jsonvalue.Number{*candidate}, nil
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

// caseSet collects CasePlans with unique observable plan keys.
type caseSet struct {
	cases []CasePlan
	seen  map[string]struct{}
}

// newCaseSet returns an empty set of unique CasePlans.
func newCaseSet() *caseSet {
	return &caseSet{
		seen: make(map[string]struct{}),
	}
}

// add records plan unless its exact observable plan key already exists.
func (set *caseSet) add(plan CasePlan) {
	if plan.Values == NoDomain || plan.Values == EmptyDomainID {
		return
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
