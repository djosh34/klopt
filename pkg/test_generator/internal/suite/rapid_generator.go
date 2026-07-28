package suite

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage"
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"pgregory.net/rapid"
)

// generatedCollectionSlack limits unbounded generated collections to a small range above their minimum.
const generatedCollectionSlack = 4

// halfDenominator divides exact bounded intervals without binary floating point.
const halfDenominator = 2

// RapidGeneratorBuilder links canonical Domains to shared constructive Rapid generators.
type RapidGeneratorBuilder struct {
	domains          *DomainRegistry
	patternOption    patternvalidator.Option
	pattern          *patternOccurrence
	generators       map[generatorKey]*rapid.Generator[jsonvalue.Value]
	patternLanguages map[patternSetKey][]stringlanguage.Language
}

// generatorKey identifies one Domain at one exact occurrence.
type generatorKey struct {
	domain    DomainID
	use       *schemaUse
	patternID uint64
}

// patternSetKey identifies one ordered list of original pattern occurrences.
type patternSetKey struct {
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
		domains:          domains,
		patternOption:    patternOption,
		generators:       make(map[generatorKey]*rapid.Generator[jsonvalue.Value]),
		patternLanguages: make(map[patternSetKey][]stringlanguage.Language),
	}
}

// Generator returns the memoized constructive generator for one exact occurrence.
func (builder *RapidGeneratorBuilder) Generator(
	id DomainID,
	use *schemaUse,
) (*rapid.Generator[jsonvalue.Value], error) {
	return builder.generator(id, use)
}

// generator builds one exact schema occurrence.
func (builder *RapidGeneratorBuilder) generator(
	id DomainID,
	use *schemaUse,
) (*rapid.Generator[jsonvalue.Value], error) {
	if builder == nil || builder.domains == nil {
		return nil, errors.New("build Rapid generator: Domain registry is nil")
	}

	key := generatorKey{domain: id, use: use, patternID: builder.patternID()}
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

	generator, err := builder.domainGenerator(domain, use)
	if err != nil {
		return nil, fmt.Errorf("build Rapid generator for Domain %d: %w", id, err)
	}

	builder.generators[key] = generator

	return generator, nil
}

// patternID returns the selected isolated pattern occurrence or the aggregate sentinel.
func (builder *RapidGeneratorBuilder) patternID() uint64 {
	if builder.pattern == nil {
		return 0
	}

	return builder.pattern.id
}

// domainGenerator builds a generator from every reachable JSON kind in domain.
func (builder *RapidGeneratorBuilder) domainGenerator(
	domain Domain,
	use *schemaUse,
) (*rapid.Generator[jsonvalue.Value], error) {
	if domain.Enum != nil {
		return builder.enumGenerator(domain, use)
	}

	var (
		generators []*rapid.Generator[jsonvalue.Value]
		firstErr   error
	)

	if domain.Null != KindExcluded {
		generators = append(generators, rapid.Just(jsonvalue.Null()))
	}

	if domain.Boolean != KindExcluded {
		generators = append(generators, rapid.Map(rapid.Bool(), jsonvalue.Bool))
	}

	if domain.Number.State != KindExcluded {
		generator, err := numberGenerator(domain.Number)

		generators, firstErr = appendConstructiveGenerator(generators, firstErr, generator, err)
	}

	if domain.String.State != KindExcluded {
		generator, err := builder.stringGenerator(domain.String, use)

		generators, firstErr = appendConstructiveGenerator(generators, firstErr, generator, err)
	}

	if domain.Array.State != KindExcluded {
		generator, err := builder.arrayGenerator(domain.Array, use)

		generators, firstErr = appendConstructiveGenerator(generators, firstErr, generator, err)
	}

	if domain.Object.State != KindExcluded {
		generator, err := builder.objectGenerator(domain.Object, use)

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

// enumGenerator samples the effective occurrence cases without changing Domain identity.
func (builder *RapidGeneratorBuilder) enumGenerator(
	domain Domain,
	use *schemaUse,
) (*rapid.Generator[jsonvalue.Value], error) {
	values := cloneJSONValues(domain.Enum.Values)

	patterns := occurrencePatterns(use, domain.String.Patterns)
	if len(patterns) == 0 {
		if len(values) == 0 {
			return nil, errors.New("enum conjunction accepts no value")
		}

		return rapid.SampledFrom(values), nil
	}

	matching, err := builder.matchingPatternEnumValues(domain, use, patterns, values)
	if err != nil {
		return nil, err
	}

	if len(matching) == 0 {
		return nil, newPatternConstructionError(patterns, use, builder.pattern, &stringlanguage.EmptyError{})
	}

	return rapid.SampledFrom(matching), nil
}

// matchingPatternEnumValues computes the finite enum/pattern conjunction before drawing.
func (builder *RapidGeneratorBuilder) matchingPatternEnumValues(
	domain Domain,
	use *schemaUse,
	patterns []patternOccurrence,
	values []jsonvalue.Value,
) ([]jsonvalue.Value, error) {
	wantMatches := make([]bool, 0, len(patterns))
	for _, pattern := range patterns {
		wantMatches = append(wantMatches, builder.pattern == nil || builder.pattern.id != pattern.id)
	}

	set, err := builder.patternSet(patterns, use, wantMatches, stringlanguage.Length{
		Min: domain.String.MinLength,
		Max: domain.String.MaxLength,
	})
	if err != nil {
		return nil, newPatternConstructionError(patterns, use, builder.pattern, err)
	}

	matching := make([]jsonvalue.Value, 0, len(values))
	for _, value := range values {
		if builder.patternEnumValueMatches(set, value) {
			matching = append(matching, value)
		}
	}

	return matching, nil
}

// patternEnumValueMatches checks one finite candidate with the independent ASCII DFA.
func (builder *RapidGeneratorBuilder) patternEnumValueMatches(
	set *stringlanguage.Set,
	value jsonvalue.Value,
) bool {
	if value.Kind != jsonvalue.KindString {
		return builder.pattern == nil
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
) (*rapid.Generator[jsonvalue.Value], error) {
	if len(constraints.Patterns) > 0 || len(constraints.Formats) > 0 || use != nil && len(use.patterns) > 0 {
		return builder.patternStringGenerator(constraints, use)
	}

	maximum := generatedCollectionMaximum(constraints.MinLength, constraints.MaxLength)
	generator := rapid.StringN(constraints.MinLength, maximum, -1)

	return rapid.Map(generator, jsonvalue.String), nil
}

// patternStringGenerator constructs the exact signed conjunction for one schema occurrence.
func (builder *RapidGeneratorBuilder) patternStringGenerator(
	constraints StringConstraints,
	use *schemaUse,
) (*rapid.Generator[jsonvalue.Value], error) {
	patterns := occurrencePatterns(use, constraints.Patterns)
	wantMatches := make([]bool, 0, len(patterns))

	for _, pattern := range patterns {
		wantMatches = append(wantMatches, builder.pattern == nil || builder.pattern.id != pattern.id)
	}

	set, err := builder.patternSet(patterns, use, wantMatches, stringlanguage.Length{
		Min: constraints.MinLength,
		Max: constraints.MaxLength,
	})
	if err != nil {
		return nil, newPatternConstructionError(patterns, use, builder.pattern, err)
	}

	return rapid.Map(rapid.Uint64(), func(seed uint64) jsonvalue.Value {
		return jsonvalue.String(set.Generate(seed))
	}), nil
}

// patternSet compiles one exact signed request from cached occurrence languages.
func (builder *RapidGeneratorBuilder) patternSet(
	patterns []patternOccurrence,
	use *schemaUse,
	wantMatches []bool,
	length stringlanguage.Length,
) (*stringlanguage.Set, error) {
	languages, err := builder.languages(patterns, use)
	if err != nil {
		return nil, err
	}

	if len(wantMatches) != len(languages) {
		return nil, fmt.Errorf(
			"compile pattern set: got %d signed requirements for %d patterns",
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
	patterns []patternOccurrence,
	use *schemaUse,
) ([]stringlanguage.Language, error) {
	signature := ""

	for _, pattern := range patterns {
		signature += strconv.Itoa(len(pattern.value)) + ":" + pattern.value
	}

	key := patternSetKey{use: use, signature: signature}
	if languages, ok := builder.patternLanguages[key]; ok {
		return languages, nil
	}

	languages := make([]stringlanguage.Language, 0, len(patterns))
	for index, pattern := range patterns {
		var (
			language stringlanguage.Language
			err      error
		)
		if pattern.source.Keyword == "format" {
			language, err = stringlanguage.Format(pattern.value)
		} else {
			language, err = stringlanguage.Pattern(pattern.value, builder.patternOption)
		}

		if err != nil {
			return nil, &patternLanguageError{index: index, err: err}
		}

		languages = append(languages, language)
	}

	builder.patternLanguages[key] = languages

	return languages, nil
}

// patternLanguageError retains the occurrence index for source attribution.
type patternLanguageError struct {
	index int
	err   error
}

// Error reports the underlying language compilation failure.
func (languageError *patternLanguageError) Error() string {
	return languageError.err.Error()
}

// Unwrap exposes the language compilation failure.
func (languageError *patternLanguageError) Unwrap() error {
	return languageError.err
}

// occurrencePatterns returns exact provenance when available and semantic patterns otherwise.
func occurrencePatterns(use *schemaUse, patterns []string) []patternOccurrence {
	if use != nil && len(use.patterns) > 0 {
		return append([]patternOccurrence(nil), use.patterns...)
	}

	result := make([]patternOccurrence, 0, len(patterns))
	for index, pattern := range patterns {
		result = append(result, patternOccurrence{id: uint64(index + 1), value: pattern})
	}

	return result
}

// patternConstructionError retains the schema source responsible for construction failure.
type patternConstructionError struct {
	source ConstraintSource
	cause  error
}

// Error reports the underlying pattern construction failure.
func (constructionError *patternConstructionError) Error() string {
	return constructionError.cause.Error()
}

// Unwrap exposes the pattern generator failure.
func (constructionError *patternConstructionError) Unwrap() error {
	return constructionError.cause
}

// newPatternConstructionError maps a backend requirement index to its exact schema declaration.
func newPatternConstructionError(
	patterns []patternOccurrence,
	use *schemaUse,
	target *patternOccurrence,
	err error,
) *patternConstructionError {
	source := ConstraintSource{Keyword: "pattern"}
	if use != nil {
		source.Pointer = use.pointer
	}

	var languageError *patternLanguageError
	if errors.As(err, &languageError) && languageError.index >= 0 && languageError.index < len(patterns) {
		source = patterns[languageError.index].source
	} else if target != nil {
		source = target.source
	} else if len(patterns) > 0 {
		source = patterns[0].source
	}

	return &patternConstructionError{source: source, cause: err}
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
) (*rapid.Generator[jsonvalue.Value], error) {
	var itemsUse *schemaUse
	if use != nil {
		itemsUse = use.items
	}

	items, err := builder.generator(constraints.Items, itemsUse)
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
) (*rapid.Generator[jsonvalue.Value], error) {
	required, optional, err := builder.objectPropertyGenerators(
		constraints.Properties,
		use,
	)
	if err != nil {
		return nil, err
	}

	var additionalUse *schemaUse
	if use != nil {
		additionalUse = use.additional
	}

	additional, additionalErr := builder.generator(
		constraints.Additional.Values,
		additionalUse,
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

		values, err := builder.generator(property.Values, propertyUse)
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
		return 0, 0, errors.New("object has no feasible property count")
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
