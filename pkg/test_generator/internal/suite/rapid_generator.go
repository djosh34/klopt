//nolint:cyclop,gocognit,godoclint,mnd,nlreturn,wsl_v5 // Constructive generation keeps JSON family rules together.
package suite

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	//nolint:depguard // Internal suite generation intentionally uses exact internal JSON values.
	"decode_and_validate_generator/pkg/test_generator/internal/jsonvalue"
	"pgregory.net/rapid"
)

const generatedCollectionSlack = 4

var errNoTrustedStringExample = errors.New("pattern or format Domain has no trusted valid example in its length range")

// RapidGeneratorBuilder links canonical Domains to shared constructive Rapid generators.
type RapidGeneratorBuilder struct {
	domains        *DomainRegistry
	generators     map[DomainID]*rapid.Generator[jsonvalue.Value]
	stringExamples map[string][]jsonvalue.Value
}

// NewRapidGeneratorBuilder creates a generator builder for one compiled Domain graph.
func NewRapidGeneratorBuilder(domains *DomainRegistry, uses []SchemaUse) *RapidGeneratorBuilder {
	builder := &RapidGeneratorBuilder{
		domains:        domains,
		generators:     make(map[DomainID]*rapid.Generator[jsonvalue.Value]),
		stringExamples: make(map[string][]jsonvalue.Value),
	}

	for _, use := range uses {
		domain, ok := domains.Domain(use.Domain)
		if !ok || len(domain.String.Patterns) == 0 && len(domain.String.Formats) == 0 {
			continue
		}

		key := stringLanguageKey(domain.String)
		for _, example := range use.Examples.Valid {
			if example.Kind == jsonvalue.KindString && !jsonValuesContain(builder.stringExamples[key], example) {
				builder.stringExamples[key] = append(builder.stringExamples[key], cloneJSONValue(example))
			}
		}
	}

	return builder
}

// Generator returns the memoized constructive generator for id.
func (builder *RapidGeneratorBuilder) Generator(id DomainID) (*rapid.Generator[jsonvalue.Value], error) {
	if builder == nil || builder.domains == nil {
		return nil, errors.New("build Rapid generator: Domain registry is nil")
	}
	if generator, ok := builder.generators[id]; ok {
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
		builder.generators[id] = generator

		return generator, nil
	}

	domain, ok := builder.domains.Domain(id)
	if !ok {
		return nil, fmt.Errorf("build Rapid generator: Domain %d does not exist", id)
	}
	if domain.Status != DomainProductive {
		return nil, fmt.Errorf("build Rapid generator: Domain %d is not productive", id)
	}

	generator, err := builder.domainGenerator(domain)
	if err != nil {
		return nil, fmt.Errorf("build Rapid generator for Domain %d: %w", id, err)
	}
	builder.generators[id] = generator

	return generator, nil
}

func (builder *RapidGeneratorBuilder) domainGenerator(domain Domain) (*rapid.Generator[jsonvalue.Value], error) {
	if domain.Enum != nil {
		return rapid.SampledFrom(cloneJSONValues(domain.Enum.Values)), nil
	}

	generators := make([]*rapid.Generator[jsonvalue.Value], 0, 6)
	var firstErr error
	if domain.Null != KindExcluded {
		generators = append(generators, rapid.Just(jsonvalue.Null()))
	}
	if domain.Boolean != KindExcluded {
		generators = append(generators, rapid.Map(rapid.Bool(), jsonvalue.Bool))
	}
	if domain.Number.State != KindExcluded {
		generator, err := numberGenerator(domain.Number)
		if err != nil {
			firstErr = err
		} else {
			generators = append(generators, generator)
		}
	}
	if domain.String.State != KindExcluded {
		generator, err := builder.stringGenerator(domain.String)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else {
			generators = append(generators, generator)
		}
	}
	if domain.Array.State != KindExcluded {
		generator, err := builder.arrayGenerator(domain.Array)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else {
			generators = append(generators, generator)
		}
	}
	if domain.Object.State != KindExcluded {
		generator, err := builder.objectGenerator(domain.Object)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else {
			generators = append(generators, generator)
		}
	}
	if len(generators) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, errors.New("productive Domain has no reachable JSON kind")
	}

	return rapid.OneOf(generators...), nil
}

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

func latticeNumberGenerator(constraints NumberConstraints) (*rapid.Generator[jsonvalue.Value], error) {
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

	factors := []*big.Int{new(big.Int).Set(minimum), new(big.Int).Set(maximum)}
	if minimum.Sign() <= 0 && maximum.Sign() >= 0 {
		factors = append(factors, new(big.Int))
	}
	values := make([]jsonvalue.Value, 0, len(factors))
	for _, factor := range factors {
		number, numberErr := exactJSONNumberFromRat(new(big.Rat).Mul(step, new(big.Rat).SetInt(factor)))
		if numberErr != nil {
			return nil, numberErr
		}
		values = append(values, jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: *number})
	}

	return rapid.SampledFrom(values), nil
}

func latticeFactorBounds(constraints NumberConstraints, step *big.Rat) (*big.Int, *big.Int, error) {
	minimum := big.NewInt(-math.MaxInt32)
	maximum := big.NewInt(math.MaxInt32)

	if constraints.Minimum != nil {
		if constraints.Minimum.Value.Rational == nil {
			return nil, nil, errors.New("minimum is too large to generate exactly")
		}
		minimum = ceilRat(new(big.Rat).Quo(constraints.Minimum.Value.Rational, step))
		if constraints.Minimum.Exclusive && new(big.Rat).Mul(new(big.Rat).SetInt(minimum), step).
			Cmp(constraints.Minimum.Value.Rational) == 0 {
			minimum.Add(minimum, big.NewInt(1))
		}
	}
	if constraints.Maximum != nil {
		if constraints.Maximum.Value.Rational == nil {
			return nil, nil, errors.New("maximum is too large to generate exactly")
		}
		maximum = floorRat(new(big.Rat).Quo(constraints.Maximum.Value.Rational, step))
		if constraints.Maximum.Exclusive && new(big.Rat).Mul(new(big.Rat).SetInt(maximum), step).
			Cmp(constraints.Maximum.Value.Rational) == 0 {
			maximum.Sub(maximum, big.NewInt(1))
		}
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

func boundedNumberCandidates(constraints NumberConstraints) ([]jsonvalue.Value, error) {
	rationals := make([]*big.Rat, 0, 3)
	if constraints.Minimum != nil && constraints.Minimum.Value.Rational == nil ||
		constraints.Maximum != nil && constraints.Maximum.Value.Rational == nil {
		return nil, errors.New("number bound is too large to generate exactly")
	}

	switch {
	case constraints.Minimum != nil && constraints.Maximum != nil:
		minimum := constraints.Minimum.Value.Rational
		maximum := constraints.Maximum.Value.Rational
		if !constraints.Minimum.Exclusive {
			rationals = append(rationals, new(big.Rat).Set(minimum))
		}
		if minimum.Cmp(maximum) != 0 {
			difference := new(big.Rat).Sub(maximum, minimum)
			rationals = append(
				rationals,
				new(big.Rat).Add(minimum, new(big.Rat).Quo(new(big.Rat).Set(difference), big.NewRat(4, 1))),
				new(big.Rat).Add(minimum, new(big.Rat).Quo(new(big.Rat).Set(difference), big.NewRat(2, 1))),
				new(big.Rat).Add(minimum, new(big.Rat).Mul(new(big.Rat).Set(difference), big.NewRat(3, 4))),
			)
		}
		if !constraints.Maximum.Exclusive {
			rationals = append(rationals, new(big.Rat).Set(maximum))
		}
	case constraints.Minimum != nil:
		minimum := constraints.Minimum.Value.Rational
		if !constraints.Minimum.Exclusive {
			rationals = append(rationals, new(big.Rat).Set(minimum))
		}
		rationals = append(
			rationals,
			new(big.Rat).Add(minimum, big.NewRat(1, 2)),
			new(big.Rat).Add(minimum, big.NewRat(1, 1)),
		)
	case constraints.Maximum != nil:
		maximum := constraints.Maximum.Value.Rational
		if !constraints.Maximum.Exclusive {
			rationals = append(rationals, new(big.Rat).Set(maximum))
		}
		rationals = append(
			rationals,
			new(big.Rat).Sub(maximum, big.NewRat(1, 2)),
			new(big.Rat).Sub(maximum, big.NewRat(1, 1)),
		)
	}

	values := make([]jsonvalue.Value, 0, len(rationals))
	for _, rational := range rationals {
		number, err := exactJSONNumberFromRat(rational)
		if err != nil {
			return nil, err
		}
		values = append(values, jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: *number})
	}
	if len(values) == 0 {
		return nil, errors.New("number constraints have no constructive value")
	}

	return values, nil
}

func mustGeneratedNumber(t *rapid.T, rational *big.Rat) jsonvalue.Value {
	t.Helper()
	number, err := exactJSONNumberFromRat(rational)
	if err != nil {
		t.Fatalf("encode exact generated number: %v", err)
	}

	return jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: *number}
}

func (builder *RapidGeneratorBuilder) stringGenerator(
	constraints StringConstraints,
) (*rapid.Generator[jsonvalue.Value], error) {
	if len(constraints.Patterns) > 0 || len(constraints.Formats) > 0 {
		examples := builder.stringExamples[stringLanguageKey(constraints)]
		values := make([]jsonvalue.Value, 0, len(examples))
		for _, example := range examples {
			length := utf8.RuneCountInString(example.String)
			if length >= constraints.MinLength && (constraints.MaxLength == nil || length <= *constraints.MaxLength) {
				values = append(values, cloneJSONValue(example))
			}
		}
		if len(values) == 0 {
			return nil, errNoTrustedStringExample
		}

		return rapid.SampledFrom(values), nil
	}

	maximum := constraints.MinLength + generatedCollectionSlack
	if maximum < constraints.MinLength {
		maximum = constraints.MinLength
	}
	if constraints.MaxLength != nil && maximum > *constraints.MaxLength {
		maximum = *constraints.MaxLength
	}
	generator := rapid.StringN(constraints.MinLength, maximum, -1)

	return rapid.Map(generator, jsonvalue.String), nil
}

func stringLanguageKey(constraints StringConstraints) string {
	return strings.Join(constraints.Patterns, "\x00") + "\x01" + strings.Join(constraints.Formats, "\x00")
}

func (builder *RapidGeneratorBuilder) arrayGenerator(
	constraints ArrayConstraints,
) (*rapid.Generator[jsonvalue.Value], error) {
	items, err := builder.Generator(constraints.Items)
	if err != nil {
		if constraints.MinItems == 0 {
			return rapid.Just(jsonvalue.Array(nil)), nil
		}
		return nil, fmt.Errorf("array items: %w", err)
	}

	maximum := constraints.MinItems + generatedCollectionSlack
	if maximum < constraints.MinItems {
		maximum = constraints.MinItems
	}
	if constraints.MaxItems != nil && maximum > *constraints.MaxItems {
		maximum = *constraints.MaxItems
	}

	return rapid.Map(rapid.SliceOfN(items, constraints.MinItems, maximum), jsonvalue.Array), nil
}

func (builder *RapidGeneratorBuilder) objectGenerator(
	constraints ObjectConstraints,
) (*rapid.Generator[jsonvalue.Value], error) {
	required := make([]objectPropertyGenerator, 0, len(constraints.Properties))
	optional := make([]objectPropertyGenerator, 0, len(constraints.Properties))
	for _, property := range constraints.Properties {
		if property.State == PropertyForbidden {
			continue
		}
		values, err := builder.Generator(property.Values)
		if err != nil {
			if property.Required {
				return nil, fmt.Errorf("object property %q: %w", property.Name, err)
			}
			continue
		}
		entry := objectPropertyGenerator{name: property.Name, values: values}
		if property.Required {
			required = append(required, entry)
		} else {
			optional = append(optional, entry)
		}
	}

	additional, additionalErr := builder.Generator(constraints.Additional.Values)
	additionalAllowed := additionalErr == nil
	minimum := max(constraints.MinProps, len(required))
	maximum := minimum + generatedCollectionSlack
	if additionalAllowed {
		maximum = max(maximum, len(required)+len(optional))
	} else {
		maximum = len(required) + len(optional)
	}
	if constraints.MaxProps != nil && maximum > *constraints.MaxProps {
		maximum = *constraints.MaxProps
	}
	if minimum > maximum {
		return nil, errors.New("object has no feasible property count")
	}

	return rapid.Custom(func(t *rapid.T) jsonvalue.Value {
		target := rapid.IntRange(minimum, maximum).Draw(t, "property count")
		members := make([]jsonvalue.Member, 0, target)
		for _, property := range required {
			members = append(members, jsonvalue.Member{
				Name: property.name, Value: property.values.Draw(t, "required "+property.name),
			})
		}

		if len(optional) > 0 {
			permuted := rapid.Permutation(optional).Draw(t, "optional properties")
			optionalCount := min(target-len(members), len(permuted))
			for _, property := range permuted[:optionalCount] {
				members = append(members, jsonvalue.Member{
					Name: property.name, Value: property.values.Draw(t, "optional "+property.name),
				})
			}
		}
		for index := 0; len(members) < target; index++ {
			name := additionalPropertyName(constraints.Properties, index)
			members = append(members, jsonvalue.Member{
				Name: name, Value: additional.Draw(t, "additional "+name),
			})
		}

		value, err := jsonvalue.Object(members)
		if err != nil {
			t.Fatalf("construct generated object: %v", err)
		}
		return value
	}), nil
}

type objectPropertyGenerator struct {
	name   string
	values *rapid.Generator[jsonvalue.Value]
}

func additionalPropertyName(properties []NamedProperty, index int) string {
	names := make(map[string]struct{}, len(properties))
	for _, property := range properties {
		names[property.Name] = struct{}{}
	}
	for {
		name := fmt.Sprintf("additional%d", index)
		if _, exists := names[name]; !exists {
			return name
		}
		index++
	}
}
