package suite

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // Required shared module; the plan forbids changing lint config.
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"pgregory.net/rapid"
)

// generatedCollectionSlack limits unbounded generated collections to a small range above their minimum.
const generatedCollectionSlack = 4

// halfDenominator divides exact bounded intervals without binary floating point.
const halfDenominator = 2

// decimalBase grows exact terminating decimal denominators.
const decimalBase = 10

// maximumDecimalDenominator bounds constructive residue search.
const maximumDecimalDenominator = 1_000_000

// jsonBooleanValueCount is the size of the finite JSON boolean domain.
const jsonBooleanValueCount = 2

// RapidGeneratorBuilder links canonical Domains to shared constructive Rapid generators.
type RapidGeneratorBuilder struct {
	domains         *DomainRegistry
	patternOption   patternvalidator.Option
	generators      map[generatorKey]*rapid.Generator[jsonvalue.Value]
	stringLanguages map[stringLanguageSetKey][]stringlanguage.Language
}

// generatorKey identifies one Domain at one exact occurrence.
type generatorKey struct {
	domain                  DomainID
	use                     *schemaUse
	stringLanguageSignature string
}

// stringLanguageSetKey identifies one ordered list of original pattern and format occurrences.
type stringLanguageSetKey struct {
	use       *schemaUse
	signature string
}

// NewRapidGeneratorBuilder creates a generator builder for one compiled Domain graph.
func NewRapidGeneratorBuilder(
	domains *DomainRegistry,
	options ...patternvalidator.Option,
) *RapidGeneratorBuilder {
	if len(options) > 1 {
		panic("suite: NewRapidGeneratorBuilder accepts at most one pattern option")
	}

	patternOption := patternvalidator.Option(func(*patternvalidator.PatternValidation) {})
	if len(options) == 1 {
		patternOption = options[0]
	}

	return &RapidGeneratorBuilder{
		domains:         domains,
		patternOption:   patternOption,
		generators:      make(map[generatorKey]*rapid.Generator[jsonvalue.Value]),
		stringLanguages: make(map[stringLanguageSetKey][]stringlanguage.Language),
	}
}

// Generator returns the memoized constructive generator for one exact occurrence.
func (builder *RapidGeneratorBuilder) Generator(
	id DomainID,
	use *schemaUse,
) (*rapid.Generator[jsonvalue.Value], error) {
	return builder.generator(id, use)
}

// expression renders one recursive generation expression.
func (builder *RapidGeneratorBuilder) expression(
	expression generationExpression,
) (*rapid.Generator[jsonvalue.Value], error) {
	if expression.term != nil {
		return builder.term(expression.term)
	}

	if expression.choice == nil || len(expression.choice.branches) == 0 {
		return nil, errors.New("build Rapid generator: generation expression is empty")
	}

	branches := make([]*rapid.Generator[jsonvalue.Value], 0, len(expression.choice.branches))
	for _, branch := range expression.choice.branches {
		generator, err := builder.expression(branch)
		if err != nil {
			return nil, err
		}

		branches = append(branches, generator)
	}

	return rapid.OneOf(branches...), nil
}

// term renders one constructive conjunction.
func (builder *RapidGeneratorBuilder) term(
	term *generationTerm,
) (*rapid.Generator[jsonvalue.Value], error) {
	if term == nil {
		return nil, errors.New("build Rapid generator: generation term is nil")
	}

	if term.items == nil && len(term.properties) == 0 && term.additional == nil &&
		len(term.excludedValues) == 0 && len(term.numberFailures) == 0 {
		return builder.generatorWithStringLanguages(term.domain, term.use, term.stringLanguages)
	}

	domain, ok := builder.domains.Domain(term.domain)
	if !ok || domain.Status != DomainProductive {
		return nil, fmt.Errorf("build Rapid generator: term Domain %d is not productive", term.domain)
	}

	return builder.expressionDomainGenerator(domain, term)
}

// expressionDomainGenerator renders every reachable JSON kind in one term.
func (builder *RapidGeneratorBuilder) expressionDomainGenerator(
	domain Domain,
	term *generationTerm,
) (*rapid.Generator[jsonvalue.Value], error) {
	return builder.buildDomainGenerator(domain, term)
}

// expressionArrayGenerator renders an array with an expression-backed item generator.
func (builder *RapidGeneratorBuilder) expressionArrayGenerator(
	constraints ArrayConstraints,
	term *generationTerm,
) (*rapid.Generator[jsonvalue.Value], error) {
	if term.items == nil {
		return builder.arrayGenerator(constraints, term.use, term.stringLanguages)
	}

	itemsExpression, err := meet(
		generationExpression{term: &generationTerm{
			domain: constraints.Items, use: term.use.items, stringLanguages: term.stringLanguages,
		}},
		*term.items,
	)
	if err != nil {
		return nil, fmt.Errorf("array items: %w", err)
	}

	items, err := builder.expression(itemsExpression)
	if err != nil {
		return nil, fmt.Errorf("array items: %w", err)
	}

	maximum := generatedCollectionMaximum(constraints.MinItems, constraints.MaxItems)

	return rapid.Map(rapid.SliceOfN(items, constraints.MinItems, maximum), jsonvalue.Array), nil
}

// expressionObjectGenerator renders expression-backed object children.
//
//nolint:cyclop // Required, optional, and additional properties share one builder.
func (builder *RapidGeneratorBuilder) expressionObjectGenerator(
	constraints ObjectConstraints,
	term *generationTerm,
) (*rapid.Generator[jsonvalue.Value], error) {
	if len(term.properties) == 0 && term.additional == nil {
		return builder.objectGenerator(constraints, term.use, term.stringLanguages)
	}

	required := make([]objectPropertyGenerator, 0)
	optional := make([]objectPropertyGenerator, 0)

	for _, property := range constraints.Properties {
		if property.State == PropertyForbidden {
			continue
		}

		var (
			generator *rapid.Generator[jsonvalue.Value]
			err       error
		)

		if expression, ok := term.properties[property.Name]; ok {
			propertyExpression, meetErr := meet(
				generationExpression{term: &generationTerm{
					domain:          property.Values,
					use:             term.use.property(property.Name),
					stringLanguages: term.stringLanguages,
				}},
				expression,
			)
			if meetErr != nil {
				return nil, fmt.Errorf("object property %q: %w", property.Name, meetErr)
			}

			generator, err = builder.expression(propertyExpression)
		} else {
			generator, err = builder.generatorWithStringLanguages(
				property.Values, term.use.property(property.Name), term.stringLanguages,
			)
		}

		if err != nil {
			if property.Required {
				return nil, err
			}

			continue
		}

		entry := objectPropertyGenerator{name: property.Name, values: generator}
		if property.Required {
			required = append(required, entry)
		} else {
			optional = append(optional, entry)
		}
	}

	var (
		additional    *rapid.Generator[jsonvalue.Value]
		additionalErr error
	)
	if term.additional != nil {
		additionalExpression, meetErr := meet(
			generationExpression{term: &generationTerm{
				domain:          constraints.Additional.Values,
				use:             term.use.additional,
				stringLanguages: term.stringLanguages,
			}},
			*term.additional,
		)
		if meetErr != nil {
			return nil, fmt.Errorf("additional object property: %w", meetErr)
		}

		additional, additionalErr = builder.expression(additionalExpression)
	} else {
		additional, additionalErr = builder.generatorWithStringLanguages(
			constraints.Additional.Values, term.use.additional, term.stringLanguages,
		)
	}

	minimum, maximum, err := objectPropertyCountRange(
		constraints, len(required), len(optional), additionalErr == nil,
	)
	if err != nil {
		return nil, err
	}

	return rapid.Custom(func(t *rapid.T) jsonvalue.Value {
		return drawGeneratedObject(t, constraints.Properties, required, optional, additional, minimum, maximum)
	}), nil
}

// generator builds one exact schema occurrence.
func (builder *RapidGeneratorBuilder) generator(
	id DomainID,
	use *schemaUse,
) (*rapid.Generator[jsonvalue.Value], error) {
	return builder.generatorWithStringLanguages(id, use, nil)
}

// generatorWithStringLanguages builds one occurrence with explicit signed string requirements.
func (builder *RapidGeneratorBuilder) generatorWithStringLanguages(
	id DomainID,
	use *schemaUse,
	targets []*stringLanguageOccurrence,
) (*rapid.Generator[jsonvalue.Value], error) {
	if builder == nil || builder.domains == nil {
		return nil, errors.New("build Rapid generator: Domain registry is nil")
	}

	key := generatorKey{domain: id, use: use, stringLanguageSignature: stringLanguageSignature(targets)}
	if generator, ok := builder.generators[key]; ok {
		return generator, nil
	}

	if id == AnyJSONDomainID {
		generator := rapid.OneOf(
			rapid.Just(jsonvalue.Null()),
			rapid.Map(rapid.Bool(), jsonvalue.Bool),
			rapid.Map(rapid.Int64(), func(value int64) jsonvalue.Value {
				return jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: jsonvalue.Number{
					Lexeme: strconv.FormatInt(value, 10), Rational: new(big.Rat).SetInt64(value),
				}}
			}),
			rapid.Map(rapid.String(), jsonvalue.String),
			rapid.Just(jsonvalue.Array(nil)),
			rapid.Just(jsonvalue.Value{Kind: jsonvalue.KindObject, Object: []jsonvalue.Member{}}),
		)
		builder.generators[key] = generator

		return generator, nil
	}

	domain, ok := builder.domains.Domain(id)
	if !ok {
		return nil, fmt.Errorf("build Rapid generator: Domain %d does not exist", id)
	}

	if domain.Status != DomainProductive {
		return nil, fmt.Errorf("build Rapid generator: Domain %d is not productive", id)
	}

	generator, err := builder.domainGenerator(domain, use, targets)
	if err != nil {
		return nil, fmt.Errorf("build Rapid generator for Domain %d: %w", id, err)
	}

	builder.generators[key] = generator

	return generator, nil
}

// stringLanguageSignature identifies one signed language conjunction for memoization.
func stringLanguageSignature(targets []*stringLanguageOccurrence) string {
	parts := make([]string, 0, len(targets))

	for _, occurrence := range targets {
		if occurrence != nil {
			parts = append(parts, strconv.FormatUint(occurrence.id, 10))
		}
	}

	return strings.Join(parts, ",")
}

// domainGenerator builds a generator from every reachable JSON kind in domain.
func (builder *RapidGeneratorBuilder) domainGenerator(
	domain Domain,
	use *schemaUse,
	targets []*stringLanguageOccurrence,
) (*rapid.Generator[jsonvalue.Value], error) {
	return builder.buildDomainGenerator(domain, &generationTerm{use: use, stringLanguages: targets})
}

// buildDomainGenerator dispatches every JSON kind once for ordinary and expression terms.
//
//nolint:cyclop,gocognit // Each JSON kind contributes one constructive generator.
func (builder *RapidGeneratorBuilder) buildDomainGenerator(
	domain Domain,
	term *generationTerm,
) (*rapid.Generator[jsonvalue.Value], error) {
	if len(term.excludedValues) != 0 {
		values, finite, err := finiteDomainValues(builder.domains, term.domain)
		if err != nil {
			return nil, err
		}

		if finite {
			values = removeExcludedValues(values, term.excludedValues)
			if len(values) == 0 {
				return nil, &stringlanguage.EmptyError{}
			}

			return rapid.SampledFrom(values), nil
		}
	}

	if domain.Enum != nil {
		values := removeExcludedValues(domain.Enum.Values, term.excludedValues)
		if len(values) == 0 {
			return nil, &stringlanguage.EmptyError{}
		}

		domain.Enum.Values = values

		return builder.enumGenerator(domain, term.use, term.stringLanguages)
	}

	var (
		generators []*rapid.Generator[jsonvalue.Value]
		firstErr   error
	)

	if domain.Null != KindExcluded && !jsonValuesContain(term.excludedValues, jsonvalue.Null()) {
		generators = append(generators, rapid.Just(jsonvalue.Null()))
	}

	if domain.Boolean != KindExcluded {
		values := make([]jsonvalue.Value, 0, jsonBooleanValueCount)

		for _, value := range []jsonvalue.Value{jsonvalue.Bool(false), jsonvalue.Bool(true)} {
			if !jsonValuesContain(term.excludedValues, value) {
				values = append(values, value)
			}
		}

		if len(values) != 0 {
			generators = append(generators, rapid.SampledFrom(values))
		}
	}

	if domain.Number.State != KindExcluded {
		generator, err := numberGeneratorForTerm(domain.Number, term)

		generators, firstErr = appendConstructiveGenerator(generators, firstErr, generator, err)
	}

	if domain.String.State != KindExcluded {
		generator, err := builder.stringGeneratorForTerm(domain.String, term)

		generators, firstErr = appendConstructiveGenerator(generators, firstErr, generator, err)
	}

	if domain.Array.State != KindExcluded {
		var (
			generator *rapid.Generator[jsonvalue.Value]
			err       error
		)
		if hasExcludedKind(term.excludedValues, jsonvalue.KindArray) {
			generator, err = builder.arrayGeneratorExcluding(domain.Array, term)
		} else if term.items != nil {
			generator, err = builder.expressionArrayGenerator(domain.Array, term)
		} else {
			generator, err = builder.arrayGenerator(domain.Array, term.use, term.stringLanguages)
		}

		generators, firstErr = appendConstructiveGenerator(generators, firstErr, generator, err)
	}

	if domain.Object.State != KindExcluded {
		var (
			generator *rapid.Generator[jsonvalue.Value]
			err       error
		)
		if hasExcludedKind(term.excludedValues, jsonvalue.KindObject) {
			generator, err = builder.objectGeneratorExcluding(domain.Object, term)
		} else if len(term.properties) != 0 || term.additional != nil {
			generator, err = builder.expressionObjectGenerator(domain.Object, term)
		} else {
			generator, err = builder.objectGenerator(domain.Object, term.use, term.stringLanguages)
		}

		generators, firstErr = appendConstructiveGenerator(generators, firstErr, generator, err)
	}

	if len(generators) > 0 {
		return rapid.OneOf(generators...), nil
	}

	if firstErr != nil {
		return nil, firstErr
	}

	return nil, errors.New("productive Domain has no reachable JSON kind")
}

// objectGeneratorExcluding constructively differs from every reachable excluded object.
//
//nolint:cyclop // Required properties are tried independently before the structural seam.
func (builder *RapidGeneratorBuilder) objectGeneratorExcluding(
	constraints ObjectConstraints,
	term *generationTerm,
) (*rapid.Generator[jsonvalue.Value], error) {
	excluded := structurallyPossibleExcludedObjects(constraints, term.excludedValues)

	clean := cloneGenerationTermWithoutKind(term, jsonvalue.KindObject)
	if len(excluded) == 0 {
		return builder.expressionObjectGenerator(constraints, &clean)
	}

	for _, property := range constraints.Properties {
		if !property.Required || property.State == PropertyForbidden {
			continue
		}

		values, presentInAll := objectPropertyValues(excluded, property.Name)
		if !presentInAll {
			continue
		}

		complement := generationExpression{term: &generationTerm{
			domain:         AnyJSONDomainID,
			use:            term.use.property(property.Name),
			excludedValues: values,
		}}
		if existing, ok := clean.properties[property.Name]; ok {
			var err error

			complement, err = meet(existing, complement)
			if err != nil {
				return nil, err
			}
		}

		candidate := clean
		candidate.properties = clonePropertyExpressions(clean.properties)
		candidate.properties[property.Name] = complement
		generator, err := builder.expressionObjectGenerator(constraints, &candidate)

		var empty *stringlanguage.EmptyError

		if err == nil {
			return generator, nil
		}

		if !errors.As(err, &empty) {
			return nil, err
		}
	}

	for index, property := range constraints.Properties {
		if property.Required || property.State == PropertyForbidden ||
			!objectPropertyAppearsInAll(excluded, property.Name) {
			continue
		}

		forced := constraints
		forced.Properties = append([]NamedProperty(nil), constraints.Properties...)
		forced.Properties[index].State = PropertyForbidden

		generator, err := builder.expressionObjectGenerator(forced, &clean)

		var empty *stringlanguage.EmptyError

		if err == nil {
			return generator, nil
		}

		if !errors.As(err, &empty) {
			return nil, err
		}
	}

	forced, ok := forceObjectPropertyAbsentFromAll(constraints, excluded)
	if ok {
		return builder.expressionObjectGenerator(forced, &clean)
	}

	forced, ok = forceObjectPropertyCountAboveAll(constraints, excluded)
	if ok {
		return builder.expressionObjectGenerator(forced, &clean)
	}

	return nil, errors.New("cannot construct exact complement of excluded object enum values")
}

// structurallyPossibleExcludedObjects removes enum objects impossible under outer shape rules.
//
//nolint:cyclop // Every object shape rule is an independent rejection condition.
func structurallyPossibleExcludedObjects(
	constraints ObjectConstraints,
	values []jsonvalue.Value,
) [][]jsonvalue.Member {
	result := make([][]jsonvalue.Member, 0, len(values))
	for _, value := range values {
		if value.Kind != jsonvalue.KindObject ||
			len(value.Object) < constraints.MinProps ||
			constraints.MaxProps != nil && len(value.Object) > *constraints.MaxProps {
			continue
		}

		members := membersByName(value.Object)
		possible := true

		for _, property := range constraints.Properties {
			_, present := members[property.Name]
			if property.Required && !present || property.State == PropertyForbidden && present {
				possible = false

				break
			}
		}

		if possible {
			result = append(result, value.Object)
		}
	}

	return result
}

// objectPropertyValues returns one property's values when every object contains it.
func objectPropertyValues(objects [][]jsonvalue.Member, name string) ([]jsonvalue.Value, bool) {
	values := make([]jsonvalue.Value, 0, len(objects))
	for _, object := range objects {
		value, ok := membersByName(object)[name]
		if !ok {
			return nil, false
		}

		values = append(values, value)
	}

	return values, true
}

// forceObjectPropertyAbsentFromAll requires an allowed property absent from every exclusion.
func forceObjectPropertyAbsentFromAll(
	constraints ObjectConstraints,
	excluded [][]jsonvalue.Member,
) (ObjectConstraints, bool) {
	required := 0

	for index, property := range constraints.Properties {
		if property.Required && property.State != PropertyForbidden {
			required++
		}

		if property.Required || property.State == PropertyForbidden || objectPropertyAppears(excluded, property.Name) {
			continue
		}

		if constraints.MaxProps != nil && required+1 > *constraints.MaxProps {
			continue
		}

		forced := constraints
		forced.Properties = append([]NamedProperty(nil), constraints.Properties...)
		forced.Properties[index].Required = true

		return forced, true
	}

	return ObjectConstraints{}, false
}

// objectPropertyAppears reports whether any excluded object contains name.
func objectPropertyAppears(objects [][]jsonvalue.Member, name string) bool {
	for _, object := range objects {
		if _, ok := membersByName(object)[name]; ok {
			return true
		}
	}

	return false
}

// objectPropertyAppearsInAll reports whether every excluded object contains name.
func objectPropertyAppearsInAll(objects [][]jsonvalue.Member, name string) bool {
	for _, object := range objects {
		if _, ok := membersByName(object)[name]; !ok {
			return false
		}
	}

	return len(objects) != 0
}

// forceObjectPropertyCountAboveAll requires a size distinct from every exclusion.
func forceObjectPropertyCountAboveAll(
	constraints ObjectConstraints,
	excluded [][]jsonvalue.Member,
) (ObjectConstraints, bool) {
	minimum := constraints.MinProps
	for _, object := range excluded {
		minimum = max(minimum, len(object)+1)
	}

	if constraints.MaxProps != nil && minimum > *constraints.MaxProps {
		return ObjectConstraints{}, false
	}

	forced := constraints
	forced.MinProps = minimum

	return forced, true
}

// cloneGenerationTermWithoutKind removes exclusions handled by a structured generator.
func cloneGenerationTermWithoutKind(term *generationTerm, kind jsonvalue.Kind) generationTerm {
	result := *term

	result.excludedValues = make([]jsonvalue.Value, 0, len(term.excludedValues))
	for _, value := range term.excludedValues {
		if value.Kind != kind {
			result.excludedValues = append(result.excludedValues, value)
		}
	}

	result.properties = clonePropertyExpressions(term.properties)

	return result
}

// clonePropertyExpressions returns an independently mutable property-expression map.
func clonePropertyExpressions(values map[string]generationExpression) map[string]generationExpression {
	result := make(map[string]generationExpression, len(values))
	for name, value := range values {
		result[name] = value
	}

	return result
}

// arrayGeneratorExcluding subtracts excluded arrays by length and tuple value.
//
//nolint:cyclop // Length partitioning and empty-expression handling are the exact array complement.
func (builder *RapidGeneratorBuilder) arrayGeneratorExcluding(
	constraints ArrayConstraints,
	term *generationTerm,
) (*rapid.Generator[jsonvalue.Value], error) {
	itemExpression := generationExpression{term: &generationTerm{
		domain: constraints.Items, use: term.use.items, stringLanguages: term.stringLanguages,
	}}
	if term.items != nil {
		var err error

		itemExpression, err = meet(itemExpression, *term.items)
		if err != nil {
			return nil, err
		}
	}

	if generationExpressionEmpty(itemExpression) && constraints.MinItems > 0 {
		return nil, &stringlanguage.EmptyError{}
	}

	excludedByLength := make(map[int][][]jsonvalue.Value)
	maximumExcluded := constraints.MinItems

	for _, value := range term.excludedValues {
		if value.Kind != jsonvalue.KindArray {
			continue
		}

		excludedByLength[len(value.Array)] = append(excludedByLength[len(value.Array)], value.Array)
		maximumExcluded = max(maximumExcluded, len(value.Array))
	}

	maximum := generatedCollectionMaximum(constraints.MinItems, constraints.MaxItems)
	if constraints.MaxItems == nil {
		maximum = max(maximum, maximumExcluded+1)
	}

	generators := make([]*rapid.Generator[jsonvalue.Value], 0, maximum-constraints.MinItems+1)
	for count := constraints.MinItems; count <= maximum; count++ {
		generator, err := builder.arrayTupleGenerator(itemExpression, count, excludedByLength[count])

		var empty *stringlanguage.EmptyError
		if errors.As(err, &empty) {
			continue
		}

		if err != nil {
			return nil, err
		}

		generators = append(generators, rapid.Map(generator, jsonvalue.Array))
	}

	if len(generators) == 0 {
		return nil, &stringlanguage.EmptyError{}
	}

	return rapid.OneOf(generators...), nil
}

// arrayTupleGenerator subtracts finite tuples using recursive head/tail decomposition.
//
//nolint:cyclop,gocognit // Head/tail decomposition directly implements finite tuple subtraction.
func (builder *RapidGeneratorBuilder) arrayTupleGenerator(
	items generationExpression,
	count int,
	excluded [][]jsonvalue.Value,
) (*rapid.Generator[[]jsonvalue.Value], error) {
	if count == 0 {
		if len(excluded) != 0 {
			return nil, &stringlanguage.EmptyError{}
		}

		return rapid.Just([]jsonvalue.Value{}), nil
	}

	itemGenerator, err := builder.expression(items)
	if err != nil {
		return nil, err
	}

	if len(excluded) == 0 {
		return rapid.SliceOfN(itemGenerator, count, count), nil
	}

	heads := make([]jsonvalue.Value, 0, len(excluded))

	groups := make([][][]jsonvalue.Value, 0, len(excluded))
	for _, tuple := range excluded {
		if len(tuple) != count {
			continue
		}

		group := -1

		for index, head := range heads {
			if head.Equal(tuple[0]) {
				group = index

				break
			}
		}

		if group == -1 {
			heads = append(heads, tuple[0])
			groups = append(groups, nil)
			group = len(groups) - 1
		}

		groups[group] = append(groups[group], tuple[1:])
	}

	branches := make([]*rapid.Generator[[]jsonvalue.Value], 0, len(heads)+1)

	outside, err := meet(items, generationExpression{term: &generationTerm{
		domain: AnyJSONDomainID, use: expressionUse(items), excludedValues: heads,
	}})
	if err != nil {
		return nil, err
	}

	if !generationExpressionEmpty(outside) {
		outsideHead, headErr := builder.expression(outside)

		var empty *stringlanguage.EmptyError
		if headErr == nil {
			tail := rapid.SliceOfN(itemGenerator, count-1, count-1)
			branches = append(branches, prependArrayValue(outsideHead, tail))
		} else if !errors.As(headErr, &empty) {
			return nil, headErr
		}
	}

	for index, head := range heads {
		equality, meetErr := meet(items, generationExpression{term: &generationTerm{
			domain: builder.domains.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{head})),
			use:    expressionUse(items),
		}})
		if meetErr != nil {
			return nil, meetErr
		}

		if generationExpressionEmpty(equality) {
			continue
		}

		if _, equalityErr := builder.expression(equality); equalityErr != nil {
			var empty *stringlanguage.EmptyError
			if errors.As(equalityErr, &empty) {
				continue
			}

			return nil, equalityErr
		}

		tail, tailErr := builder.arrayTupleGenerator(items, count-1, groups[index])

		var empty *stringlanguage.EmptyError
		if errors.As(tailErr, &empty) {
			continue
		}

		if tailErr != nil {
			return nil, tailErr
		}

		branches = append(branches, prependArrayValue(rapid.Just(head), tail))
	}

	if len(branches) == 0 {
		return nil, &stringlanguage.EmptyError{}
	}

	return rapid.OneOf(branches...), nil
}

// expressionUse returns the occurrence owned by an expression's first constructive term.
func expressionUse(expression generationExpression) *schemaUse {
	if expression.term != nil {
		return expression.term.use
	}

	if expression.choice != nil {
		for _, branch := range expression.choice.branches {
			if use := expressionUse(branch); use != nil {
				return use
			}
		}
	}

	return nil
}

// prependArrayValue combines one constructive head with a generated tail.
func prependArrayValue(
	head *rapid.Generator[jsonvalue.Value],
	tail *rapid.Generator[[]jsonvalue.Value],
) *rapid.Generator[[]jsonvalue.Value] {
	return rapid.Custom(func(t *rapid.T) []jsonvalue.Value {
		result := []jsonvalue.Value{head.Draw(t, "head")}
		result = append(result, tail.Draw(t, "tail")...)

		return result
	})
}

// removeExcludedValues subtracts exact JSON values from a finite candidate set.
func removeExcludedValues(values []jsonvalue.Value, excluded []jsonvalue.Value) []jsonvalue.Value {
	result := make([]jsonvalue.Value, 0, len(values))
	for _, value := range values {
		if !jsonValuesContain(excluded, value) {
			result = append(result, value)
		}
	}

	return result
}

// hasExcludedKind reports whether structured subtraction is needed for kind.
func hasExcludedKind(values []jsonvalue.Value, kind jsonvalue.Kind) bool {
	for _, value := range values {
		if value.Kind == kind {
			return true
		}
	}

	return false
}

// numberGeneratorForTerm dispatches arithmetic failures and exact value exclusions.
func numberGeneratorForTerm(
	constraints NumberConstraints,
	term *generationTerm,
) (*rapid.Generator[jsonvalue.Value], error) {
	if len(term.numberFailures) != 0 {
		return numberFailureGenerator(constraints, term.numberFailures)
	}

	excluded := make([]jsonvalue.Number, 0)

	for _, value := range term.excludedValues {
		if value.Kind == jsonvalue.KindNumber {
			excluded = append(excluded, value.Number)
		}
	}

	return numberGeneratorExcluding(constraints, excluded)
}

// numberGeneratorExcluding subtracts exact points by splitting the allowed interval.
//
//nolint:cyclop // Ordered interval splitting constructively subtracts every excluded number.
func numberGeneratorExcluding(
	constraints NumberConstraints,
	excluded []jsonvalue.Number,
) (*rapid.Generator[jsonvalue.Value], error) {
	if len(excluded) == 0 {
		return numberGenerator(constraints)
	}

	slices.SortFunc(excluded, func(left jsonvalue.Number, right jsonvalue.Number) int {
		return left.Compare(right)
	})

	segments := make([]*rapid.Generator[jsonvalue.Value], 0, len(excluded)+1)

	minimum := cloneBound(constraints.Minimum)
	for _, value := range excluded {
		if fits, err := numberFits(value, constraints); err != nil {
			return nil, err
		} else if !fits {
			continue
		}

		segment := constraints
		segment.Minimum = cloneBound(minimum)

		segment.Maximum = &NumberBound{Value: value, Exclusive: true}
		if productive, err := numberConstraintsAreProductive(segment); err != nil {
			return nil, err
		} else if productive {
			generator, err := numberGenerator(segment)
			if err != nil {
				return nil, err
			}

			segments = append(segments, generator)
		}

		minimum = &NumberBound{Value: value, Exclusive: true}
	}

	segment := constraints

	segment.Minimum = cloneBound(minimum)
	if productive, err := numberConstraintsAreProductive(segment); err != nil {
		return nil, err
	} else if productive {
		generator, err := numberGenerator(segment)
		if err != nil {
			return nil, err
		}

		segments = append(segments, generator)
	}

	if len(segments) == 0 {
		return nil, &stringlanguage.EmptyError{}
	}

	return rapid.OneOf(segments...), nil
}

// numberFailureGenerator builds numbers violating every requested arithmetic predicate.
func numberFailureGenerator(
	constraints NumberConstraints,
	failures []numberFailure,
) (*rapid.Generator[jsonvalue.Value], error) {
	targets := make([]*big.Rat, 0, len(failures))
	for _, failure := range failures {
		switch {
		case failure.integer:
			targets = append(targets, big.NewRat(1, 1))
		case failure.multipleOf != nil && failure.multipleOf.Rational != nil:
			targets = append(targets, new(big.Rat).Set(failure.multipleOf.Rational))
		default:
			return nil, errors.New("numeric complement target is not exactly representable")
		}
	}

	if constraints.IntegersOnly || constraints.MultipleOf != nil {
		return latticeNumberFailureGenerator(constraints, targets)
	}

	return continuousNumberFailureGenerator(constraints, targets)
}

// latticeNumberFailureGenerator selects residue classes that fail every target divisor.
//
//nolint:cyclop // Exact residue construction handles bounded and unbounded numeric lattices.
func latticeNumberFailureGenerator(
	constraints NumberConstraints,
	targets []*big.Rat,
) (*rapid.Generator[jsonvalue.Value], error) {
	step, err := latticeStep(constraints)
	if err != nil {
		return nil, err
	}

	periods := make([]*big.Int, 0, len(targets))
	period := big.NewInt(1)

	for _, target := range targets {
		ratio := new(big.Rat).Quo(step, target)

		divisor := new(big.Int).Set(ratio.Denom())
		if divisor.Cmp(big.NewInt(1)) == 0 {
			return nil, &stringlanguage.EmptyError{}
		}

		periods = append(periods, divisor)

		period = leastCommonMultipleInt(period, divisor)
		if !period.IsInt64() || period.Int64() > 10_000 {
			return nil, errors.New("numeric complement residue period is too large")
		}
	}

	minimum, maximum, err := latticeFactorBounds(constraints, step)
	if err != nil {
		return nil, err
	}

	generators := make([]*rapid.Generator[jsonvalue.Value], 0)

	periodValue := period.Int64()
	for residue := int64(0); residue < periodValue; residue++ {
		allowed := true

		for _, divisor := range periods {
			if new(big.Int).Mod(big.NewInt(residue), divisor).Sign() == 0 {
				allowed = false

				break
			}
		}

		if !allowed {
			continue
		}

		minimumN := ceilRat(new(big.Rat).SetFrac(
			new(big.Int).Sub(minimum, big.NewInt(residue)), period,
		))

		maximumN := floorRat(new(big.Rat).SetFrac(
			new(big.Int).Sub(maximum, big.NewInt(residue)), period,
		))
		if minimumN.Cmp(maximumN) > 0 || !minimumN.IsInt64() || !maximumN.IsInt64() {
			continue
		}

		residueCopy := residue

		generators = append(generators, rapid.Custom(func(t *rapid.T) jsonvalue.Value {
			n := rapid.Int64Range(minimumN.Int64(), maximumN.Int64()).Draw(t, "factor period")
			factor := new(big.Int).Add(big.NewInt(residueCopy), new(big.Int).Mul(big.NewInt(n), period))

			return mustGeneratedNumber(t, new(big.Rat).Mul(step, new(big.Rat).SetInt(factor)))
		}))
	}

	if len(generators) == 0 {
		return nil, &stringlanguage.EmptyError{}
	}

	return rapid.OneOf(generators...), nil
}

// continuousNumberFailureGenerator selects exact decimals outside every target lattice.
//
//nolint:cyclop // Decimal denominator construction avoids every requested arithmetic lattice.
func continuousNumberFailureGenerator(
	constraints NumberConstraints,
	targets []*big.Rat,
) (*rapid.Generator[jsonvalue.Value], error) {
	period := new(big.Rat).Set(targets[0])
	for _, target := range targets[1:] {
		period = rationalLeastCommonMultiple(period, target)
	}

	denominator := big.NewInt(decimalBase)

	for {
		works := true

		for _, target := range targets {
			multiple := new(big.Rat).Quo(period, target)
			if !multiple.IsInt() || new(big.Int).Mod(multiple.Num(), denominator).Sign() == 0 {
				works = false

				break
			}
		}

		if works {
			break
		}

		denominator.Mul(denominator, big.NewInt(decimalBase))

		if denominator.Cmp(big.NewInt(maximumDecimalDenominator)) > 0 {
			return nil, errors.New("numeric complement offset is too complex")
		}
	}

	offset := new(big.Rat).Quo(period, new(big.Rat).SetInt(denominator))

	minimum, maximum := big.NewInt(-math.MaxInt32), big.NewInt(math.MaxInt32)
	if constraints.Minimum != nil {
		minimum = ceilRat(new(big.Rat).Quo(
			new(big.Rat).Sub(constraints.Minimum.Value.Rational, offset), period,
		))
		if constraints.Minimum.Exclusive && new(big.Rat).Add(
			offset, new(big.Rat).Mul(period, new(big.Rat).SetInt(minimum)),
		).Cmp(constraints.Minimum.Value.Rational) == 0 {
			minimum.Add(minimum, big.NewInt(1))
		}
	}

	if constraints.Maximum != nil {
		maximum = floorRat(new(big.Rat).Quo(
			new(big.Rat).Sub(constraints.Maximum.Value.Rational, offset), period,
		))
		if constraints.Maximum.Exclusive && new(big.Rat).Add(
			offset, new(big.Rat).Mul(period, new(big.Rat).SetInt(maximum)),
		).Cmp(constraints.Maximum.Value.Rational) == 0 {
			maximum.Sub(maximum, big.NewInt(1))
		}
	}

	if minimum.Cmp(maximum) > 0 || !minimum.IsInt64() || !maximum.IsInt64() {
		return nil, &stringlanguage.EmptyError{}
	}

	return rapid.Custom(func(t *rapid.T) jsonvalue.Value {
		factor := rapid.Int64Range(minimum.Int64(), maximum.Int64()).Draw(t, "factor")
		value := new(big.Rat).Add(offset, new(big.Rat).Mul(period, new(big.Rat).SetInt64(factor)))

		return mustGeneratedNumber(t, value)
	}), nil
}

// leastCommonMultipleInt returns the positive integer least common multiple.
func leastCommonMultipleInt(left *big.Int, right *big.Int) *big.Int {
	gcd := new(big.Int).GCD(nil, nil, left, right)

	return new(big.Int).Quo(new(big.Int).Mul(left, right), gcd)
}

// rationalLeastCommonMultiple returns the smallest positive rational multiple of both inputs.
func rationalLeastCommonMultiple(left *big.Rat, right *big.Rat) *big.Rat {
	numerator := leastCommonMultipleInt(
		new(big.Int).Abs(left.Num()), new(big.Int).Abs(right.Num()),
	)
	denominator := new(big.Int).GCD(nil, nil, left.Denom(), right.Denom())

	return new(big.Rat).SetFrac(numerator, denominator)
}

// stringGeneratorForTerm combines exact enum subtraction with signed string languages.
func (builder *RapidGeneratorBuilder) stringGeneratorForTerm(
	constraints StringConstraints,
	term *generationTerm,
) (*rapid.Generator[jsonvalue.Value], error) {
	excluded := make([]string, 0)

	for _, value := range term.excludedValues {
		if value.Kind == jsonvalue.KindString {
			excluded = append(excluded, value.String)
		}
	}

	if len(excluded) == 0 {
		return builder.stringGenerator(constraints, term.use, term.stringLanguages)
	}

	parts := make([]string, len(excluded))
	for index, value := range excluded {
		parts[index] = regexp.QuoteMeta(value)
	}

	pattern := "^(?:" + strings.Join(parts, "|") + ")$"

	language, err := stringlanguage.Pattern(pattern, builder.patternOption)
	if err != nil {
		return nil, err
	}

	requirements := []stringlanguage.Requirement{{Language: language, WantMatch: false}}
	occurrences := occurrenceStringLanguages(term.use, constraints.Patterns)

	languages, err := builder.languages(occurrences, term.use)
	if err != nil {
		return nil, err
	}

	for index, occurrence := range occurrences {
		requirements = append(requirements, stringlanguage.Requirement{
			Language: languages[index], WantMatch: wantStringLanguageMatch(term.stringLanguages, occurrence),
		})
	}

	set, err := stringlanguage.Compile(requirements, stringlanguage.Length{
		Min: constraints.MinLength, Max: constraints.MaxLength,
	})
	if err != nil {
		return nil, err
	}

	return rapid.Map(rapid.Uint64(), func(seed uint64) jsonvalue.Value {
		return jsonvalue.String(set.Generate(seed))
	}), nil
}

// enumGenerator samples the effective occurrence cases without changing Domain identity.
func (builder *RapidGeneratorBuilder) enumGenerator(
	domain Domain,
	use *schemaUse,
	targets []*stringLanguageOccurrence,
) (*rapid.Generator[jsonvalue.Value], error) {
	values := cloneJSONValues(domain.Enum.Values)

	occurrences := occurrenceStringLanguages(use, domain.String.Patterns)
	if len(occurrences) == 0 {
		if len(values) == 0 {
			return nil, errors.New("enum conjunction accepts no value")
		}

		return rapid.SampledFrom(values), nil
	}

	matching, err := builder.matchingStringLanguageEnumValues(domain, use, occurrences, values, targets)
	if err != nil {
		return nil, err
	}

	if len(matching) == 0 {
		return nil, newStringLanguageConstructionError(
			occurrences,
			use,
			firstTargetStringLanguage(targets),
			&stringlanguage.EmptyError{},
		)
	}

	return rapid.SampledFrom(matching), nil
}

// matchingStringLanguageEnumValues computes the finite enum/string-language conjunction before drawing.
func (builder *RapidGeneratorBuilder) matchingStringLanguageEnumValues(
	domain Domain,
	use *schemaUse,
	occurrences []stringLanguageOccurrence,
	values []jsonvalue.Value,
	targets []*stringLanguageOccurrence,
) ([]jsonvalue.Value, error) {
	wantMatches := make([]bool, 0, len(occurrences))
	for _, occurrence := range occurrences {
		wantMatches = append(
			wantMatches,
			wantStringLanguageMatch(targets, occurrence),
		)
	}

	set, err := builder.stringLanguageSet(occurrences, use, wantMatches, stringlanguage.Length{
		Min: domain.String.MinLength,
		Max: domain.String.MaxLength,
	})
	if err != nil {
		return nil, newStringLanguageConstructionError(occurrences, use, firstTargetStringLanguage(targets), err)
	}

	matching := make([]jsonvalue.Value, 0, len(values))
	for _, value := range values {
		if stringLanguageEnumValueMatches(set, value, targets) {
			matching = append(matching, value)
		}
	}

	return matching, nil
}

// stringLanguageEnumValueMatches checks one finite candidate with the independent ASCII DFA.
func stringLanguageEnumValueMatches(
	set *stringlanguage.Set,
	value jsonvalue.Value,
	targets []*stringLanguageOccurrence,
) bool {
	if value.Kind != jsonvalue.KindString {
		return firstTargetStringLanguage(targets) == nil
	}

	return set.Matches(value.String)
}

// appendConstructiveGenerator records a reachable generator or preserves the first construction error.
func appendConstructiveGenerator(
	generators []*rapid.Generator[jsonvalue.Value],
	firstErr error,
	generator *rapid.Generator[jsonvalue.Value],
	generatorErr error,
) ([]*rapid.Generator[jsonvalue.Value], error) {
	if generatorErr == nil {
		return append(generators, generator), firstErr
	}

	if firstErr == nil {
		return generators, generatorErr
	}

	return generators, firstErr
}

// numberGenerator builds a generator for numeric constraints.
func numberGenerator(constraints NumberConstraints) (*rapid.Generator[jsonvalue.Value], error) {
	if !constraints.IntegersOnly && constraints.MultipleOf == nil {
		if slices.Contains(constraints.Formats, "float") {
			return float32NumberGenerator(constraints)
		}

		if slices.Contains(constraints.Formats, "double") {
			return float64NumberGenerator(constraints)
		}
	}

	if constraints.IntegersOnly || constraints.MultipleOf != nil {
		return latticeNumberGenerator(constraints)
	}

	if constraints.Minimum == nil && constraints.Maximum == nil {
		return rapid.Custom(func(t *rapid.T) jsonvalue.Value {
			numerator := rapid.Int64().Draw(t, "numerator")
			scale := rapid.SampledFrom([]int64{1, 10}).Draw(t, "decimal scale")

			return mustGeneratedNumber(t, new(big.Rat).SetFrac64(numerator, scale))
		}), nil
	}

	candidates, err := boundedNumberCandidates(constraints)
	if err != nil {
		return nil, err
	}

	return rapid.SampledFrom(candidates), nil
}

// float32NumberGenerator generates finite binary32 values inside exact schema bounds.
func float32NumberGenerator(constraints NumberConstraints) (*rapid.Generator[jsonvalue.Value], error) {
	minimum, err := float32Minimum(constraints.Minimum)
	if err != nil {
		return nil, err
	}

	maximum, err := float32Maximum(constraints.Maximum)
	if err != nil {
		return nil, err
	}

	if minimum > maximum {
		return nil, errors.New("float constraints contain no representable float32")
	}

	return rapid.Custom(func(t *rapid.T) jsonvalue.Value {
		value := rapid.Float32Range(minimum, maximum).Draw(t, "float32")

		return mustGeneratedFloat(t, float64(value), suiteFloat32BitSize)
	}), nil
}

// float64NumberGenerator generates finite binary64 values inside exact schema bounds.
func float64NumberGenerator(constraints NumberConstraints) (*rapid.Generator[jsonvalue.Value], error) {
	minimum, err := float64Minimum(constraints.Minimum)
	if err != nil {
		return nil, err
	}

	maximum, err := float64Maximum(constraints.Maximum)
	if err != nil {
		return nil, err
	}

	if minimum > maximum {
		return nil, errors.New("double constraints contain no representable float64")
	}

	return rapid.Custom(func(t *rapid.T) jsonvalue.Value {
		value := rapid.Float64Range(minimum, maximum).Draw(t, "float64")

		return mustGeneratedFloat(t, value, suiteFloat64BitSize)
	}), nil
}

// float32Minimum returns the first encoded binary32 value allowed by bound.
func float32Minimum(bound *NumberBound) (float32, error) {
	if bound == nil {
		return -math.MaxFloat32, nil
	}

	value, err := strconv.ParseFloat(bound.Value.Lexeme, suiteFloat32BitSize)
	if err != nil {
		return 0, fmt.Errorf("parse float32 minimum: %w", err)
	}

	result := float32(value)
	for {
		satisfies, satisfiesErr := generatedFloatSatisfiesMinimum(
			float64(result),
			suiteFloat32BitSize,
			bound,
		)
		if satisfiesErr != nil {
			return 0, satisfiesErr
		}

		if satisfies {
			break
		}

		result = math.Nextafter32(result, float32(math.Inf(1)))
		if math.IsInf(float64(result), 1) {
			return 0, errors.New("float constraints contain no representable float32 minimum")
		}
	}

	return result, nil
}

// float32Maximum returns the last encoded binary32 value allowed by bound.
func float32Maximum(bound *NumberBound) (float32, error) {
	if bound == nil {
		return math.MaxFloat32, nil
	}

	value, err := strconv.ParseFloat(bound.Value.Lexeme, suiteFloat32BitSize)
	if err != nil {
		return 0, fmt.Errorf("parse float32 maximum: %w", err)
	}

	result := float32(value)
	for {
		satisfies, satisfiesErr := generatedFloatSatisfiesMaximum(
			float64(result),
			suiteFloat32BitSize,
			bound,
		)
		if satisfiesErr != nil {
			return 0, satisfiesErr
		}

		if satisfies {
			break
		}

		result = math.Nextafter32(result, float32(math.Inf(-1)))
		if math.IsInf(float64(result), -1) {
			return 0, errors.New("float constraints contain no representable float32 maximum")
		}
	}

	return result, nil
}

// float64Minimum returns the first encoded binary64 value allowed by bound.
func float64Minimum(bound *NumberBound) (float64, error) {
	if bound == nil {
		return -math.MaxFloat64, nil
	}

	result, err := strconv.ParseFloat(bound.Value.Lexeme, suiteFloat64BitSize)
	if err != nil {
		return 0, fmt.Errorf("parse float64 minimum: %w", err)
	}

	for {
		satisfies, satisfiesErr := generatedFloatSatisfiesMinimum(result, suiteFloat64BitSize, bound)
		if satisfiesErr != nil {
			return 0, satisfiesErr
		}

		if satisfies {
			break
		}

		result = math.Nextafter(result, math.Inf(1))
		if math.IsInf(result, 1) {
			return 0, errors.New("double constraints contain no representable float64 minimum")
		}
	}

	return result, nil
}

// float64Maximum returns the last encoded binary64 value allowed by bound.
func float64Maximum(bound *NumberBound) (float64, error) {
	if bound == nil {
		return math.MaxFloat64, nil
	}

	result, err := strconv.ParseFloat(bound.Value.Lexeme, suiteFloat64BitSize)
	if err != nil {
		return 0, fmt.Errorf("parse float64 maximum: %w", err)
	}

	for {
		satisfies, satisfiesErr := generatedFloatSatisfiesMaximum(result, suiteFloat64BitSize, bound)
		if satisfiesErr != nil {
			return 0, satisfiesErr
		}

		if satisfies {
			break
		}

		result = math.Nextafter(result, math.Inf(-1))
		if math.IsInf(result, -1) {
			return 0, errors.New("double constraints contain no representable float64 maximum")
		}
	}

	return result, nil
}

// generatedFloatSatisfiesMinimum checks the emitted decimal against an exact lower bound.
func generatedFloatSatisfiesMinimum(value float64, bitSize int, bound *NumberBound) (bool, error) {
	number, err := jsonvalue.ParseNumber(strconv.FormatFloat(value, 'g', -1, bitSize))
	if err != nil {
		return false, fmt.Errorf("encode float%d minimum candidate: %w", bitSize, err)
	}

	comparison := number.Compare(bound.Value)

	return comparison > 0 || comparison == 0 && !bound.Exclusive, nil
}

// generatedFloatSatisfiesMaximum checks the emitted decimal against an exact upper bound.
func generatedFloatSatisfiesMaximum(value float64, bitSize int, bound *NumberBound) (bool, error) {
	number, err := jsonvalue.ParseNumber(strconv.FormatFloat(value, 'g', -1, bitSize))
	if err != nil {
		return false, fmt.Errorf("encode float%d maximum candidate: %w", bitSize, err)
	}

	comparison := number.Compare(bound.Value)

	return comparison < 0 || comparison == 0 && !bound.Exclusive, nil
}

// mustGeneratedFloat encodes one generated finite value with the requested native width.
func mustGeneratedFloat(t *rapid.T, value float64, bitSize int) jsonvalue.Value {
	t.Helper()

	number, err := jsonvalue.ParseNumber(strconv.FormatFloat(value, 'g', -1, bitSize))
	if err != nil {
		t.Fatalf("encode generated float%d: %v", bitSize, err)
	}

	return jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: number}
}

// latticeNumberGenerator builds numbers from integer factors of an exact step.
func latticeNumberGenerator(constraints NumberConstraints) (*rapid.Generator[jsonvalue.Value], error) {
	step, err := latticeStep(constraints)
	if err != nil {
		return nil, err
	}

	minimum, maximum, err := latticeFactorBounds(constraints, step)
	if err != nil {
		return nil, err
	}

	if minimum.IsInt64() && maximum.IsInt64() {
		return rapid.Custom(func(t *rapid.T) jsonvalue.Value {
			factor := rapid.Int64Range(minimum.Int64(), maximum.Int64()).Draw(t, "factor")

			return mustGeneratedNumber(t, new(big.Rat).Mul(step, new(big.Rat).SetInt64(factor)))
		}), nil
	}

	values, err := largeLatticeValues(step, minimum, maximum)
	if err != nil {
		return nil, err
	}

	return rapid.SampledFrom(values), nil
}

// latticeStep returns the exact step for integer or multipleOf generation.
func latticeStep(constraints NumberConstraints) (*big.Rat, error) {
	step := big.NewRat(1, 1)

	if constraints.MultipleOf != nil {
		if constraints.MultipleOf.Rational == nil {
			return nil, errors.New("multipleOf is too large to generate exactly")
		}

		step.Set(constraints.MultipleOf.Rational)
	}

	if constraints.IntegersOnly && !step.IsInt() {
		step.SetInt(new(big.Int).Abs(step.Num()))
	}

	return step, nil
}

// largeLatticeValues returns representative values when the factor range exceeds int64.
func largeLatticeValues(step *big.Rat, minimum *big.Int, maximum *big.Int) ([]jsonvalue.Value, error) {
	factors := []*big.Int{new(big.Int).Set(minimum), new(big.Int).Set(maximum)}
	if minimum.Sign() <= 0 && maximum.Sign() >= 0 {
		factors = append(factors, new(big.Int))
	}

	values := make([]jsonvalue.Value, 0, len(factors))
	for _, factor := range factors {
		number, err := exactJSONNumberFromRat(new(big.Rat).Mul(step, new(big.Rat).SetInt(factor)))
		if err != nil {
			return nil, err
		}

		values = append(values, jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: *number})
	}

	return values, nil
}

// latticeFactorBounds returns the inclusive factor range allowed by numeric bounds.
func latticeFactorBounds(constraints NumberConstraints, step *big.Rat) (*big.Int, *big.Int, error) {
	minimum, err := minimumLatticeFactor(constraints.Minimum, step)
	if err != nil {
		return nil, nil, err
	}

	maximum, err := maximumLatticeFactor(constraints.Maximum, step)
	if err != nil {
		return nil, nil, err
	}

	if constraints.Maximum == nil && minimum.Cmp(maximum) > 0 {
		maximum = new(big.Int).Add(minimum, big.NewInt(math.MaxInt32))
	}

	if constraints.Minimum == nil && maximum.Cmp(minimum) < 0 {
		minimum = new(big.Int).Sub(maximum, big.NewInt(math.MaxInt32))
	}

	if minimum.Cmp(maximum) > 0 {
		return nil, nil, errors.New("numeric lattice is empty")
	}

	return minimum, maximum, nil
}

// minimumLatticeFactor returns the first factor admitted by a lower bound.
func minimumLatticeFactor(bound *NumberBound, step *big.Rat) (*big.Int, error) {
	if bound == nil {
		return big.NewInt(-math.MaxInt32), nil
	}

	if bound.Value.Rational == nil {
		return nil, errors.New("minimum is too large to generate exactly")
	}

	minimum := ceilRat(new(big.Rat).Quo(bound.Value.Rational, step))
	if bound.Exclusive && new(big.Rat).Mul(new(big.Rat).SetInt(minimum), step).Cmp(bound.Value.Rational) == 0 {
		minimum.Add(minimum, big.NewInt(1))
	}

	return minimum, nil
}

// maximumLatticeFactor returns the last factor admitted by an upper bound.
func maximumLatticeFactor(bound *NumberBound, step *big.Rat) (*big.Int, error) {
	if bound == nil {
		return big.NewInt(math.MaxInt32), nil
	}

	if bound.Value.Rational == nil {
		return nil, errors.New("maximum is too large to generate exactly")
	}

	maximum := floorRat(new(big.Rat).Quo(bound.Value.Rational, step))
	if bound.Exclusive && new(big.Rat).Mul(new(big.Rat).SetInt(maximum), step).Cmp(bound.Value.Rational) == 0 {
		maximum.Sub(maximum, big.NewInt(1))
	}

	return maximum, nil
}

// boundedNumberCandidates returns representative exact values from a bounded interval.
func boundedNumberCandidates(constraints NumberConstraints) ([]jsonvalue.Value, error) {
	rationals, err := boundedNumberRationals(constraints)
	if err != nil {
		return nil, err
	}

	values := make([]jsonvalue.Value, 0, len(rationals))
	for _, rational := range rationals {
		number, numberErr := exactJSONNumberFromRat(rational)
		if numberErr != nil {
			return nil, numberErr
		}

		values = append(values, jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: *number})
	}

	if len(values) == 0 {
		return nil, errors.New("number constraints have no constructive value")
	}

	return values, nil
}

// boundedNumberRationals selects representative rationals for the configured bounds.
func boundedNumberRationals(constraints NumberConstraints) ([]*big.Rat, error) {
	if constraints.Minimum != nil && constraints.Minimum.Value.Rational == nil ||
		constraints.Maximum != nil && constraints.Maximum.Value.Rational == nil {
		return nil, errors.New("number bound is too large to generate exactly")
	}

	switch {
	case constraints.Minimum != nil && constraints.Maximum != nil:
		return twoSidedBoundedRationals(constraints), nil
	case constraints.Minimum != nil:
		return lowerBoundedRationals(constraints.Minimum), nil
	case constraints.Maximum != nil:
		return upperBoundedRationals(constraints.Maximum), nil
	default:
		return nil, nil
	}
}

// twoSidedBoundedRationals selects interior values and any inclusive endpoints.
func twoSidedBoundedRationals(constraints NumberConstraints) []*big.Rat {
	var rationals []*big.Rat

	minimum := constraints.Minimum.Value.Rational
	maximum := constraints.Maximum.Value.Rational

	if !constraints.Minimum.Exclusive {
		rationals = append(rationals, new(big.Rat).Set(minimum))
	}

	if minimum.Cmp(maximum) != 0 {
		difference := new(big.Rat).Sub(maximum, minimum)
		half := new(big.Rat).Quo(new(big.Rat).Set(difference), big.NewRat(halfDenominator, 1))
		quarter := new(big.Rat).Quo(new(big.Rat).Set(half), big.NewRat(halfDenominator, 1))

		rationals = append(
			rationals,
			new(big.Rat).Add(minimum, quarter),
			new(big.Rat).Add(minimum, half),
			new(big.Rat).Sub(maximum, quarter),
		)
	}

	if !constraints.Maximum.Exclusive {
		rationals = append(rationals, new(big.Rat).Set(maximum))
	}

	return rationals
}

// lowerBoundedRationals selects values at and above a lower bound.
func lowerBoundedRationals(bound *NumberBound) []*big.Rat {
	var rationals []*big.Rat

	minimum := bound.Value.Rational

	if !bound.Exclusive {
		rationals = append(rationals, new(big.Rat).Set(minimum))
	}

	return append(
		rationals,
		new(big.Rat).Add(minimum, big.NewRat(1, halfDenominator)),
		new(big.Rat).Add(minimum, big.NewRat(1, 1)),
	)
}

// upperBoundedRationals selects values at and below an upper bound.
func upperBoundedRationals(bound *NumberBound) []*big.Rat {
	var rationals []*big.Rat

	maximum := bound.Value.Rational

	if !bound.Exclusive {
		rationals = append(rationals, new(big.Rat).Set(maximum))
	}

	return append(
		rationals,
		new(big.Rat).Sub(maximum, big.NewRat(1, halfDenominator)),
		new(big.Rat).Sub(maximum, big.NewRat(1, 1)),
	)
}

// mustGeneratedNumber converts an exact rational or fails the active Rapid check.
func mustGeneratedNumber(t *rapid.T, rational *big.Rat) jsonvalue.Value {
	t.Helper()

	number, err := exactJSONNumberFromRat(rational)
	if err != nil {
		t.Fatalf("encode exact generated number: %v", err)
	}

	return jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: *number}
}

// stringGenerator builds arbitrary strings or constructively satisfies string languages.
func (builder *RapidGeneratorBuilder) stringGenerator(
	constraints StringConstraints,
	use *schemaUse,
	targets []*stringLanguageOccurrence,
) (*rapid.Generator[jsonvalue.Value], error) {
	if len(constraints.Patterns) > 0 || len(constraints.Formats) > 0 ||
		use != nil && len(use.stringLanguages) > 0 {
		return builder.stringLanguageStringGenerator(constraints, use, targets)
	}

	maximum := generatedCollectionMaximum(constraints.MinLength, constraints.MaxLength)
	generator := rapid.StringN(constraints.MinLength, maximum, -1)

	return rapid.Map(generator, jsonvalue.String), nil
}

// stringLanguageStringGenerator constructs the exact signed conjunction for one schema occurrence.
func (builder *RapidGeneratorBuilder) stringLanguageStringGenerator(
	constraints StringConstraints,
	use *schemaUse,
	targets []*stringLanguageOccurrence,
) (*rapid.Generator[jsonvalue.Value], error) {
	occurrences := occurrenceStringLanguages(use, constraints.Patterns)
	wantMatches := make([]bool, 0, len(occurrences))

	for _, occurrence := range occurrences {
		wantMatches = append(
			wantMatches,
			wantStringLanguageMatch(targets, occurrence),
		)
	}

	set, err := builder.stringLanguageSet(occurrences, use, wantMatches, stringlanguage.Length{
		Min: constraints.MinLength,
		Max: constraints.MaxLength,
	})
	if err != nil {
		return nil, newStringLanguageConstructionError(occurrences, use, firstTargetStringLanguage(targets), err)
	}

	return rapid.Map(rapid.Uint64(), func(seed uint64) jsonvalue.Value {
		return jsonvalue.String(set.Generate(seed))
	}), nil
}

// wantStringLanguageMatch reports the sign of one language occurrence.
func wantStringLanguageMatch(
	targets []*stringLanguageOccurrence,
	occurrence stringLanguageOccurrence,
) bool {
	for _, target := range targets {
		if target != nil && target.id == occurrence.id {
			return false
		}
	}

	return true
}

// firstTargetStringLanguage locates diagnostics for a signed conjunction.
func firstTargetStringLanguage(targets []*stringLanguageOccurrence) *stringLanguageOccurrence {
	for _, target := range targets {
		if target != nil {
			return target
		}
	}

	return nil
}

// stringLanguageSet compiles one exact signed request from cached occurrence languages.
func (builder *RapidGeneratorBuilder) stringLanguageSet(
	occurrences []stringLanguageOccurrence,
	use *schemaUse,
	wantMatches []bool,
	length stringlanguage.Length,
) (*stringlanguage.Set, error) {
	languages, err := builder.languages(occurrences, use)
	if err != nil {
		return nil, err
	}

	if len(wantMatches) != len(languages) {
		return nil, fmt.Errorf(
			"compile string-language set: got %d signed requirements for %d languages",
			len(wantMatches),
			len(languages),
		)
	}

	requirements := make([]stringlanguage.Requirement, len(languages))
	for index, language := range languages {
		requirements[index] = stringlanguage.Requirement{
			Language: language, WantMatch: wantMatches[index],
		}
	}

	return stringlanguage.Compile(requirements, length)
}

// languages compiles each original occurrence once and reuses it across signed requests.
func (builder *RapidGeneratorBuilder) languages(
	occurrences []stringLanguageOccurrence,
	use *schemaUse,
) ([]stringlanguage.Language, error) {
	signature := ""

	for _, occurrence := range occurrences {
		keyword := occurrence.source.Keyword
		signature += strconv.Itoa(len(keyword)) + ":" + keyword +
			strconv.Itoa(len(occurrence.value)) + ":" + occurrence.value
	}

	key := stringLanguageSetKey{use: use, signature: signature}
	if languages, ok := builder.stringLanguages[key]; ok {
		return languages, nil
	}

	languages := make([]stringlanguage.Language, 0, len(occurrences))
	for index, occurrence := range occurrences {
		var (
			language stringlanguage.Language
			err      error
		)
		if occurrence.source.Keyword == "format" {
			language, err = stringlanguage.Format(occurrence.value)
		} else {
			language, err = stringlanguage.Pattern(occurrence.value, builder.patternOption)
		}

		if err != nil {
			return nil, &stringLanguageError{index: index, err: err}
		}

		languages = append(languages, language)
	}

	builder.stringLanguages[key] = languages

	return languages, nil
}

// stringLanguageError retains the occurrence index for source attribution.
type stringLanguageError struct {
	index int
	err   error
}

// Error reports the underlying language compilation failure.
func (languageError *stringLanguageError) Error() string {
	return languageError.err.Error()
}

// Unwrap exposes the language compilation failure.
func (languageError *stringLanguageError) Unwrap() error {
	return languageError.err
}

// occurrenceStringLanguages returns exact provenance when available and semantic patterns otherwise.
func occurrenceStringLanguages(use *schemaUse, patterns []string) []stringLanguageOccurrence {
	if use != nil && len(use.stringLanguages) > 0 {
		return append([]stringLanguageOccurrence(nil), use.stringLanguages...)
	}

	result := make([]stringLanguageOccurrence, 0, len(patterns))
	for index, pattern := range patterns {
		result = append(result, stringLanguageOccurrence{id: uint64(index + 1), value: pattern})
	}

	return result
}

// stringLanguageConstructionError retains the schema source responsible for construction failure.
type stringLanguageConstructionError struct {
	source ConstraintSource
	cause  error
}

// Error reports the underlying pattern construction failure.
func (constructionError *stringLanguageConstructionError) Error() string {
	return constructionError.cause.Error()
}

// Unwrap exposes the pattern generator failure.
func (constructionError *stringLanguageConstructionError) Unwrap() error {
	return constructionError.cause
}

// newStringLanguageConstructionError maps a backend requirement index to its exact schema declaration.
func newStringLanguageConstructionError(
	occurrences []stringLanguageOccurrence,
	use *schemaUse,
	target *stringLanguageOccurrence,
	err error,
) *stringLanguageConstructionError {
	source := ConstraintSource{Keyword: "pattern"}
	if use != nil {
		source.Pointer = use.pointer
	}

	var languageError *stringLanguageError
	if errors.As(err, &languageError) && languageError.index >= 0 && languageError.index < len(occurrences) {
		source = occurrences[languageError.index].source
	} else if target != nil {
		source = target.source
	} else if len(occurrences) > 0 {
		source = occurrences[0].source
	}

	return &stringLanguageConstructionError{source: source, cause: err}
}

// generatedCollectionMaximum caps an unbounded collection range above minimum.
func generatedCollectionMaximum(minimum int, configuredMaximum *int) int {
	maximum := minimum
	if minimum <= math.MaxInt-generatedCollectionSlack {
		maximum += generatedCollectionSlack
	}

	if configuredMaximum != nil && maximum > *configuredMaximum {
		maximum = *configuredMaximum
	}

	return maximum
}

// arrayGenerator builds arrays from the generator for their item Domain.
func (builder *RapidGeneratorBuilder) arrayGenerator(
	constraints ArrayConstraints,
	use *schemaUse,
	targets []*stringLanguageOccurrence,
) (*rapid.Generator[jsonvalue.Value], error) {
	var itemsUse *schemaUse
	if use != nil {
		itemsUse = use.items
	}

	items, err := builder.generatorWithStringLanguages(constraints.Items, itemsUse, targets)
	if err != nil {
		if constraints.MinItems == 0 && constraints.MaxItems != nil && *constraints.MaxItems == 0 {
			return rapid.Just(jsonvalue.Array(nil)), nil
		}

		return nil, fmt.Errorf("array items: %w", err)
	}

	maximum := generatedCollectionMaximum(constraints.MinItems, constraints.MaxItems)

	return rapid.Map(rapid.SliceOfN(items, constraints.MinItems, maximum), jsonvalue.Array), nil
}

// objectGenerator builds objects from feasible declared and additional properties.
func (builder *RapidGeneratorBuilder) objectGenerator(
	constraints ObjectConstraints,
	use *schemaUse,
	targets []*stringLanguageOccurrence,
) (*rapid.Generator[jsonvalue.Value], error) {
	required, optional, err := builder.objectPropertyGenerators(
		constraints.Properties,
		use,
		targets,
	)
	if err != nil {
		return nil, err
	}

	var additionalUse *schemaUse
	if use != nil {
		additionalUse = use.additional
	}

	additional, additionalErr := builder.generatorWithStringLanguages(
		constraints.Additional.Values,
		additionalUse,
		targets,
	)

	minimum, maximum, err := objectPropertyCountRange(
		constraints,
		len(required),
		len(optional),
		additionalErr == nil,
	)
	if err != nil {
		return nil, err
	}

	return rapid.Custom(func(t *rapid.T) jsonvalue.Value {
		return drawGeneratedObject(t, constraints.Properties, required, optional, additional, minimum, maximum)
	}), nil
}

// objectPropertyGenerators separates feasible declared properties into required and optional groups.
func (builder *RapidGeneratorBuilder) objectPropertyGenerators(
	properties []NamedProperty,
	use *schemaUse,
	targets []*stringLanguageOccurrence,
) ([]objectPropertyGenerator, []objectPropertyGenerator, error) {
	required := make([]objectPropertyGenerator, 0, len(properties))
	optional := make([]objectPropertyGenerator, 0, len(properties))

	for _, property := range properties {
		if property.State == PropertyForbidden {
			continue
		}

		var propertyUse *schemaUse
		if use != nil {
			propertyUse = use.property(property.Name)
			if propertyUse == nil {
				propertyUse = use.additional
			}
		}

		values, err := builder.generatorWithStringLanguages(property.Values, propertyUse, targets)
		if err != nil && property.Required {
			return nil, nil, fmt.Errorf("object property %q: %w", property.Name, err)
		}

		if err != nil {
			continue
		}

		entry := objectPropertyGenerator{name: property.Name, values: values}
		if property.Required {
			required = append(required, entry)
		} else {
			optional = append(optional, entry)
		}
	}

	return required, optional, nil
}

// objectPropertyCountRange returns the feasible generated property-count range.
func objectPropertyCountRange(
	constraints ObjectConstraints,
	requiredCount int,
	optionalCount int,
	additionalAllowed bool,
) (int, int, error) {
	minimum := max(constraints.MinProps, requiredCount)

	maximum := generatedCollectionMaximum(minimum, nil)
	if additionalAllowed {
		maximum = max(maximum, requiredCount+optionalCount)
	} else {
		maximum = requiredCount + optionalCount
	}

	if constraints.MaxProps != nil && maximum > *constraints.MaxProps {
		maximum = *constraints.MaxProps
	}

	if minimum > maximum {
		return 0, 0, &stringlanguage.EmptyError{}
	}

	return minimum, maximum, nil
}

// drawGeneratedObject draws a feasible property count and constructs the corresponding object.
func drawGeneratedObject(
	t *rapid.T,
	properties []NamedProperty,
	required []objectPropertyGenerator,
	optional []objectPropertyGenerator,
	additional *rapid.Generator[jsonvalue.Value],
	minimum int,
	maximum int,
) jsonvalue.Value {
	t.Helper()

	target := rapid.IntRange(minimum, maximum).Draw(t, "property count")
	members := drawDeclaredObjectMembers(t, target, required, optional)

	for index := 0; len(members) < target; index++ {
		name := additionalPropertyName(properties, index)
		members = append(members, jsonvalue.Member{
			Name: name, Value: additional.Draw(t, "additional "+name),
		})
	}

	value, err := jsonvalue.Object(members)
	if err != nil {
		t.Fatalf("construct generated object: %v", err)
	}

	return value
}

// drawDeclaredObjectMembers draws all required and enough optional declared properties.
func drawDeclaredObjectMembers(
	t *rapid.T,
	target int,
	required []objectPropertyGenerator,
	optional []objectPropertyGenerator,
) []jsonvalue.Member {
	t.Helper()

	members := make([]jsonvalue.Member, 0, target)

	for _, property := range required {
		members = append(members, jsonvalue.Member{
			Name: property.name, Value: property.values.Draw(t, "required "+property.name),
		})
	}

	if len(optional) == 0 {
		return members
	}

	permuted := rapid.Permutation(optional).Draw(t, "optional properties")
	optionalCount := min(target-len(members), len(permuted))

	for _, property := range permuted[:optionalCount] {
		members = append(members, jsonvalue.Member{
			Name: property.name, Value: property.values.Draw(t, "optional "+property.name),
		})
	}

	return members
}

// objectPropertyGenerator associates an object property name with its value generator.
type objectPropertyGenerator struct {
	name   string
	values *rapid.Generator[jsonvalue.Value]
}

// additionalPropertyName returns an indexed name that does not collide with declared properties.
func additionalPropertyName(properties []NamedProperty, index int) string {
	names := make(map[string]struct{}, len(properties))
	for _, property := range properties {
		names[property.Name] = struct{}{}
	}

	for candidate := 0; ; candidate++ {
		name := fmt.Sprintf("additional%d", candidate)
		if _, exists := names[name]; exists {
			continue
		}

		if index == 0 {
			return name
		}

		index--
	}
}
