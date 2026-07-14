package suite

import (
	"errors"
	"fmt"
	"sort"

	"decode_and_validate_generator/pkg/test_generator/internal/jsonvalue"
)

// CompileOption configures a Compiler.
type CompileOption func(*Compiler)

// MustHaveAllXValidCases rejects allOf string merges without a shared trusted valid example.
func MustHaveAllXValidCases(compiler *Compiler) {
	compiler.mustHaveAllXValidCases = true
}

// CompileSuite compiles, plans, and links the request schema to Rapid generators.
func (compiler *Compiler) CompileSuite(options ...CompileOption) (*CompiledSuite, error) {
	for _, option := range options {
		option(compiler)
	}

	root, err := compiler.Compile()
	if err != nil {
		return nil, err
	}

	planner := &CasePlanner{Domains: compiler.Domains}

	cases, err := planner.Plan(compiler.rootUse)
	if err != nil {
		return nil, err
	}

	linked, err := compiler.linkCases(root, cases)
	if err != nil {
		return nil, err
	}

	planner.markUnconstructibleConstraints(linked)

	if err := compiler.requireAcceptedCase(root, linked); err != nil {
		return nil, err
	}

	return &CompiledSuite{
		Root:        root,
		Domains:     compiler.Domains,
		Constraints: append([]ConstraintPlan(nil), planner.Constraints...),
		Cases:       linked,
	}, nil
}

// linkCases assigns a Rapid generator to each constructible CasePlan.
func (compiler *Compiler) linkCases(root DomainID, cases []CasePlan) ([]CasePlan, error) {
	generators := NewRapidGeneratorBuilder(compiler.Domains, compiler.rootUse)
	linked := make([]CasePlan, 0, len(cases))

	for index := range cases {
		generator, generatorErr := generators.Generator(cases[index].Values)
		if errors.Is(generatorErr, errNoTrustedStringExample) &&
			(cases[index].Expect == ExpectRejected || cases[index].Values != root) {
			continue
		}

		if generatorErr != nil {
			return nil, compiler.failure(
				"generate",
				"unconstructible",
				cases[index].Source.Pointer,
				cases[index].Source.Keyword,
				generatorErr,
			)
		}

		cases[index].Generator = generator
		linked = append(linked, cases[index])
	}

	return linked, nil
}

// markUnconstructibleConstraints records isolated failures without a linked generator.
func (planner *CasePlanner) markUnconstructibleConstraints(cases []CasePlan) {
	for index := range planner.Constraints {
		constraint := &planner.Constraints[index]
		if constraint.Outcome != ObligationPlanned {
			continue
		}

		if hasRejectedCase(cases, constraint.Source) {
			continue
		}

		constraint.Outcome = ObligationUnconstructible
		constraint.Reason = "isolated failure has no trusted pattern or format example"
	}
}

// requireAcceptedCase rejects a productive root that has no linked accepted case.
func (compiler *Compiler) requireAcceptedCase(root DomainID, cases []CasePlan) error {
	rootDomain, ok := compiler.Domains.Domain(root)
	if !ok {
		return compiler.failure(
			"generate", "malformed", compiler.Source.RequestSchema.Pointer, "", errors.New("root Domain does not exist"),
		)
	}

	if rootDomain.Status == DomainEmpty {
		return compiler.failure(
			"generate", "empty", compiler.Source.RequestSchema.Pointer, "", errors.New("request schema accepts no JSON value"),
		)
	}

	if rootDomain.Status != DomainProductive {
		return nil
	}

	if hasAcceptedCase(cases) {
		return nil
	}

	return compiler.failure(
		"generate",
		"unconstructible",
		compiler.Source.RequestSchema.Pointer,
		"",
		errNoTrustedStringExample,
	)
}

// hasRejectedCase reports whether cases include a rejected case for source.
func hasRejectedCase(cases []CasePlan, source ConstraintSource) bool {
	for _, plannedCase := range cases {
		if plannedCase.Expect == ExpectRejected && plannedCase.Source == source {
			return true
		}
	}

	return false
}

// hasAcceptedCase reports whether cases include an accepted case.
func hasAcceptedCase(cases []CasePlan) bool {
	for _, plannedCase := range cases {
		if plannedCase.Expect == ExpectAccepted {
			return true
		}
	}

	return false
}

// Plan builds aggregate-valid, valid-partition, and isolated invalid CasePlans.
func (planner *CasePlanner) Plan(rootUse *schemaUse) ([]CasePlan, error) {
	if planner == nil || planner.Domains == nil {
		return nil, errors.New("plan cases: Domain registry is nil")
	}

	if rootUse == nil {
		return nil, errors.New("plan cases: root occurrence is nil")
	}

	rootDomain, ok := planner.Domains.Domain(rootUse.domain)
	if !ok {
		return nil, fmt.Errorf("plan cases: root Domain %d does not exist", rootUse.domain)
	}

	constraints, err := planner.constraintPlans(rootUse)
	if err != nil {
		return nil, err
	}

	planner.Constraints = constraints

	result := newCaseSet()
	if rootDomain.Status == DomainProductive {
		result.add(CasePlan{
			Name:   caseName("valid aggregate", rootUse.pointer, ""),
			Expect: ExpectAccepted,
			Values: rootUse.domain,
			Source: ConstraintSource{Pointer: rootUse.pointer},
		})
	}

	if err := planner.addIsolatedFailures(result); err != nil {
		return nil, err
	}

	if rootDomain.Status == DomainProductive {
		if err := planner.addValidPartitions(result, rootUse, make(map[DomainID]bool)); err != nil {
			return nil, err
		}
	}

	return result.cases, nil
}

// constraintPlans creates atomic pass/fail Domains while retaining allOf source provenance.
func (planner *CasePlanner) constraintPlans(root *schemaUse) ([]ConstraintPlan, error) {
	plans := make([]ConstraintPlan, 0, len(root.constraints))
	seen := make(map[ConstraintSource]struct{})

	for _, source := range root.constraints {
		if _, duplicate := seen[source]; duplicate {
			continue
		}

		seen[source] = struct{}{}

		use := planner.constraintUse(source, root)

		plan, include, err := planner.atomicConstraint(source, use)
		if err != nil {
			return nil, fmt.Errorf("plan %s at %s: %w", source.Keyword, source.Pointer, err)
		}

		if include {
			plans = append(plans, plan)
		}
	}

	sort.SliceStable(plans, func(left int, right int) bool {
		if plans[left].Source.Pointer != plans[right].Source.Pointer {
			return plans[left].Source.Pointer < plans[right].Source.Pointer
		}

		return plans[left].Source.Keyword < plans[right].Source.Keyword
	})

	return plans, nil
}

// constraintUse resolves the occurrence that supplies source's local rule and examples.
func (planner *CasePlanner) constraintUse(source ConstraintSource, root *schemaUse) *schemaUse {
	use := root.find(source.Pointer)
	if use == nil {
		use = root
	}

	if (source.Keyword == "pattern" || source.Keyword == "format") && len(use.examples.Invalid) == 0 {
		copyUse := *use
		copyUse.examples.Invalid = cloneJSONValues(root.examples.Invalid)
		use = &copyUse
	}

	return use
}

// atomicConstraint constructs an applicability-correct atomic rule.
// Passing Domains leave unrelated JSON kinds unrestricted; failing Domains contain only the failing kind.
func (planner *CasePlanner) atomicConstraint(source ConstraintSource, use *schemaUse) (ConstraintPlan, bool, error) {
	domain, ok := planner.atomicConstraintDomain(source, use)
	if !ok {
		return ConstraintPlan{}, false, nil
	}

	switch source.Keyword {
	case "allOf", "items", "properties":
		return ConstraintPlan{}, false, nil
	case "type", "nullable", "enum":
		return planner.atomicPrimitiveConstraint(source, domain)
	case "minimum", "exclusiveMinimum", "maximum", "exclusiveMaximum", "multipleOf":
		return planner.atomicNumberConstraint(source, domain)
	case "minLength", "maxLength":
		return planner.atomicStringConstraint(source, domain)
	case "pattern", "format":
		return planner.atomicStringExampleConstraint(source, domain, use)
	case "minItems", "maxItems":
		return planner.atomicArrayConstraint(source, domain)
	case "minProperties", "maxProperties", "required", "additionalProperties":
		return planner.atomicObjectConstraint(source, domain)
	default:
		return ConstraintPlan{}, false, nil
	}
}

// atomicConstraintDomain returns the source-local Domain before whole-schema enum filtering.
func (planner *CasePlanner) atomicConstraintDomain(source ConstraintSource, use *schemaUse) (Domain, bool) {
	if atomic, ok := use.atomic[source.Keyword]; ok {
		return planner.Domains.Domain(atomic)
	}

	return planner.Domains.Domain(use.localDomain)
}

// atomicPrimitiveConstraint dispatches type, nullable, and enum rules.
func (planner *CasePlanner) atomicPrimitiveConstraint(
	source ConstraintSource,
	domain Domain,
) (ConstraintPlan, bool, error) {
	switch source.Keyword {
	case "type":
		return planner.atomicTypeConstraint(source, domain)
	case "nullable":
		return planner.atomicNullableConstraint(source, domain)
	case "enum":
		return planner.atomicEnumConstraint(source, domain)
	default:
		return ConstraintPlan{}, false, nil
	}
}

// atomicTypeConstraint builds the valid kind mask and invalid excluded kind partitions.
func (planner *CasePlanner) atomicTypeConstraint(source ConstraintSource, domain Domain) (ConstraintPlan, bool, error) {
	pass := kindMaskDomain(domain)
	excluded := excludedKinds(domain)
	fails := make([]DomainID, 0, len(excluded))

	for _, kind := range excluded {
		failure := planner.Domains.FindOrAddEquivalentDomain(singleKindDomain(kind))
		fails = append(fails, failure)
	}

	if domain.Number.State != KindExcluded && domain.Number.IntegersOnly {
		pass.Number.IntegersOnly = true
		pass.Number.State = KindRestricted
		integerFailures := planner.finiteNumberFailures(fractionalCandidates())

		fails = append(fails, integerFailures...)
	}

	return planner.plannedAtomicConstraint(source, pass, fails)
}

// atomicNullableConstraint builds the null partition when null is excluded.
func (planner *CasePlanner) atomicNullableConstraint(
	source ConstraintSource,
	domain Domain,
) (ConstraintPlan, bool, error) {
	if domain.Null != KindExcluded {
		return ConstraintPlan{}, false, nil
	}

	failure := planner.Domains.FindOrAddEquivalentDomain(singleKindDomain(jsonvalue.KindNull))

	return planner.plannedAtomicConstraint(source, anyJSONDomain(), []DomainID{failure})
}

// atomicEnumConstraint builds finite partitions for values outside the enum.
func (planner *CasePlanner) atomicEnumConstraint(source ConstraintSource, domain Domain) (ConstraintPlan, bool, error) {
	fails := make([]DomainID, 0)

	for _, candidate := range outsiderCandidates(domain.Enum) {
		failure := planner.Domains.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{candidate}))
		fails = append(fails, failure)
	}

	return planner.plannedAtomicConstraint(source, domain, fails)
}

// atomicNumberConstraint dispatches numeric bound and multiple-of rules.
func (planner *CasePlanner) atomicNumberConstraint(
	source ConstraintSource,
	domain Domain,
) (ConstraintPlan, bool, error) {
	switch source.Keyword {
	case "minimum", "maximum":
		return planner.atomicInclusiveNumberConstraint(source, domain)
	case "exclusiveMinimum", "exclusiveMaximum":
		return planner.atomicExclusiveNumberConstraint(source, domain)
	case "multipleOf":
		return planner.atomicMultipleOfConstraint(source, domain)
	default:
		return ConstraintPlan{}, false, nil
	}
}

// atomicInclusiveNumberConstraint builds partitions for inclusive minimum and maximum bounds.
func (planner *CasePlanner) atomicInclusiveNumberConstraint(
	source ConstraintSource,
	domain Domain,
) (ConstraintPlan, bool, error) {
	switch source.Keyword {
	case "minimum":
		if domain.Number.Minimum == nil {
			return ConstraintPlan{}, false, nil
		}

		minimum := cloneBound(domain.Number.Minimum)
		minimum.Exclusive = false
		pass := numberRuleDomain(NumberConstraints{State: KindRestricted, Minimum: minimum})
		failure := cloneBound(minimum)
		failure.Exclusive = true
		fails := []DomainID{planner.numberDomain(NumberConstraints{
			State:   KindRestricted,
			Maximum: failure,
		})}

		return planner.plannedAtomicConstraint(source, pass, fails)
	case "maximum":
		if domain.Number.Maximum == nil {
			return ConstraintPlan{}, false, nil
		}

		maximum := cloneBound(domain.Number.Maximum)
		maximum.Exclusive = false
		pass := numberRuleDomain(NumberConstraints{State: KindRestricted, Maximum: maximum})
		failure := cloneBound(maximum)
		failure.Exclusive = true
		fails := []DomainID{planner.numberDomain(NumberConstraints{
			State:   KindRestricted,
			Minimum: failure,
		})}

		return planner.plannedAtomicConstraint(source, pass, fails)
	default:
		return ConstraintPlan{}, false, nil
	}
}

// atomicExclusiveNumberConstraint builds partitions for exclusive minimum and maximum bounds.
func (planner *CasePlanner) atomicExclusiveNumberConstraint(
	source ConstraintSource,
	domain Domain,
) (ConstraintPlan, bool, error) {
	switch source.Keyword {
	case "exclusiveMinimum":
		if domain.Number.Minimum == nil || !domain.Number.Minimum.Exclusive {
			return ConstraintPlan{}, false, nil
		}

		pass := numberRuleDomain(NumberConstraints{
			State:   KindRestricted,
			Minimum: cloneBound(domain.Number.Minimum),
		})
		equal := cloneBound(domain.Number.Minimum)
		equal.Exclusive = false
		fails := []DomainID{planner.numberDomain(NumberConstraints{
			State:   KindRestricted,
			Minimum: equal,
			Maximum: equal,
		})}

		return planner.plannedAtomicConstraint(source, pass, fails)
	case "exclusiveMaximum":
		if domain.Number.Maximum == nil || !domain.Number.Maximum.Exclusive {
			return ConstraintPlan{}, false, nil
		}

		pass := numberRuleDomain(NumberConstraints{
			State:   KindRestricted,
			Maximum: cloneBound(domain.Number.Maximum),
		})
		equal := cloneBound(domain.Number.Maximum)
		equal.Exclusive = false
		fails := []DomainID{planner.numberDomain(NumberConstraints{
			State:   KindRestricted,
			Minimum: equal,
			Maximum: equal,
		})}

		return planner.plannedAtomicConstraint(source, pass, fails)
	default:
		return ConstraintPlan{}, false, nil
	}
}

// atomicMultipleOfConstraint builds partitions for the multipleOf keyword.
func (planner *CasePlanner) atomicMultipleOfConstraint(
	source ConstraintSource,
	domain Domain,
) (ConstraintPlan, bool, error) {
	if domain.Number.MultipleOf == nil {
		return ConstraintPlan{}, false, nil
	}

	pass := numberRuleDomain(NumberConstraints{
		State:      KindRestricted,
		MultipleOf: cloneNumber(domain.Number.MultipleOf),
	})

	candidates, err := nonMultipleCandidates(domain.Number.MultipleOf)
	if err != nil {
		return ConstraintPlan{}, false, err
	}

	fails := planner.finiteNumberFailures(candidates)

	return planner.plannedAtomicConstraint(source, pass, fails)
}

// atomicStringConstraint builds the pass and fail partitions for string lengths.
func (planner *CasePlanner) atomicStringConstraint(
	source ConstraintSource,
	domain Domain,
) (ConstraintPlan, bool, error) {
	var pass Domain

	var fails []DomainID

	switch source.Keyword {
	case "minLength":
		pass = stringRuleDomain(StringConstraints{
			State:     KindRestricted,
			MinLength: domain.String.MinLength,
		})
		if domain.String.MinLength > 0 {
			fails = []DomainID{planner.stringDomain(StringConstraints{
				State:     KindRestricted,
				MaxLength: new(domain.String.MinLength - 1),
			})}
		}
	case "maxLength":
		if domain.String.MaxLength == nil {
			return ConstraintPlan{}, false, nil
		}

		pass = stringRuleDomain(StringConstraints{
			State:     KindRestricted,
			MaxLength: new(*domain.String.MaxLength),
		})
		if next, ok := incrementInt(*domain.String.MaxLength); ok {
			fails = []DomainID{planner.stringDomain(StringConstraints{
				State:     KindRestricted,
				MinLength: next,
			})}
		}
	default:
		return ConstraintPlan{}, false, nil
	}

	return planner.plannedAtomicConstraint(source, pass, fails)
}

// atomicStringExampleConstraint builds pattern and format partitions from invalid string examples.
func (planner *CasePlanner) atomicStringExampleConstraint(
	source ConstraintSource,
	domain Domain,
	use *schemaUse,
) (ConstraintPlan, bool, error) {
	pass := anyJSONDomain()

	pass.String = StringConstraints{State: KindRestricted}
	if source.Keyword == "pattern" {
		pass.String.Patterns = append([]string(nil), domain.String.Patterns...)
	} else {
		pass.String.Formats = append([]string(nil), domain.String.Formats...)
	}

	fails := make([]DomainID, 0)

	for _, example := range use.examples.Invalid {
		if example.Kind != jsonvalue.KindString {
			continue
		}

		failure := planner.Domains.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{example}))
		fails = append(fails, failure)
	}

	return planner.plannedAtomicConstraint(source, pass, fails)
}

// atomicArrayConstraint builds the pass and fail partitions for array lengths.
func (planner *CasePlanner) atomicArrayConstraint(
	source ConstraintSource,
	domain Domain,
) (ConstraintPlan, bool, error) {
	var pass Domain

	var fails []DomainID

	switch source.Keyword {
	case "minItems":
		pass = arrayRuleDomain(ArrayConstraints{
			State:    KindRestricted,
			Items:    domain.Array.Items,
			MinItems: domain.Array.MinItems,
		})
		if domain.Array.MinItems > 0 {
			fails = []DomainID{planner.arrayDomain(ArrayConstraints{
				State:    KindRestricted,
				Items:    domain.Array.Items,
				MaxItems: new(domain.Array.MinItems - 1),
			})}
		}
	case "maxItems":
		if domain.Array.MaxItems == nil {
			return ConstraintPlan{}, false, nil
		}

		pass = arrayRuleDomain(ArrayConstraints{
			State:    KindRestricted,
			Items:    domain.Array.Items,
			MaxItems: new(*domain.Array.MaxItems),
		})
		if next, ok := incrementInt(*domain.Array.MaxItems); ok {
			fails = []DomainID{planner.arrayDomain(ArrayConstraints{
				State:    KindRestricted,
				Items:    domain.Array.Items,
				MinItems: next,
			})}
		}
	default:
		return ConstraintPlan{}, false, nil
	}

	return planner.plannedAtomicConstraint(source, pass, fails)
}

// atomicObjectConstraint builds the pass and fail partitions for object keywords.
func (planner *CasePlanner) atomicObjectConstraint(
	source ConstraintSource,
	domain Domain,
) (ConstraintPlan, bool, error) {
	var pass Domain

	var fails []DomainID

	switch source.Keyword {
	case "minProperties":
		passing := objectValueRule(domain.Object)
		passing.MinProps = domain.Object.MinProps
		pass = objectRuleDomain(passing)

		if domain.Object.MinProps > 0 {
			failure := objectValueRule(domain.Object)
			failure.MaxProps = new(domain.Object.MinProps - 1)
			fails = []DomainID{planner.objectDomain(failure)}
		}
	case "maxProperties":
		if domain.Object.MaxProps == nil {
			return ConstraintPlan{}, false, nil
		}

		passing := objectValueRule(domain.Object)
		passing.MaxProps = new(*domain.Object.MaxProps)
		pass = objectRuleDomain(passing)

		if next, ok := incrementInt(*domain.Object.MaxProps); ok {
			failure := objectValueRule(domain.Object)
			failure.MinProps = next
			fails = []DomainID{planner.objectDomain(failure)}
		}
	case "required":
		return planner.atomicRequiredConstraint(source, domain)
	case "additionalProperties":
		return planner.atomicAdditionalPropertiesConstraint(source, domain)
	default:
		return ConstraintPlan{}, false, nil
	}

	return planner.plannedAtomicConstraint(source, pass, fails)
}

// atomicRequiredConstraint builds a failure partition for each required property.
func (planner *CasePlanner) atomicRequiredConstraint(
	source ConstraintSource,
	domain Domain,
) (ConstraintPlan, bool, error) {
	pass := objectRuleDomain(requiredRule(domain.Object))
	fails := make([]DomainID, 0)

	for _, property := range domain.Object.Properties {
		if !property.Required {
			continue
		}

		failure := requiredRule(domain.Object)
		for index := range failure.Properties {
			if failure.Properties[index].Name == property.Name {
				failure.Properties[index].Required = false
				failure.Properties[index].State = PropertyForbidden
				failure.Properties[index].Values = EmptyDomainID
			}
		}

		fails = append(fails, planner.objectDomain(failure))
	}

	return planner.plannedAtomicConstraint(source, pass, fails)
}

// atomicAdditionalPropertiesConstraint builds the forbidden additional-property partition.
func (planner *CasePlanner) atomicAdditionalPropertiesConstraint(
	source ConstraintSource,
	domain Domain,
) (ConstraintPlan, bool, error) {
	policy := additionalPropertyRule(domain.Object)

	pass := objectRuleDomain(policy)
	if domain.Object.Additional.Values != EmptyDomainID {
		return planner.plannedAtomicConstraint(source, pass, nil)
	}

	failure := policy
	failure.Properties = append(failure.Properties, NamedProperty{
		Name:     unusedPropertyName(domain.Object),
		Required: true,
		State:    PropertyAllowed,
		Values:   AnyJSONDomainID,
	})
	fails := []DomainID{planner.objectDomain(failure)}

	return planner.plannedAtomicConstraint(source, pass, fails)
}

// plannedAtomicConstraint records a completed atomic pass/fail rule.
func (planner *CasePlanner) plannedAtomicConstraint(
	source ConstraintSource,
	pass Domain,
	fails []DomainID,
) (ConstraintPlan, bool, error) {
	plan := ConstraintPlan{
		Source:  source,
		Pass:    planner.Domains.FindOrAddEquivalentDomain(pass),
		Outcome: ObligationUnconstructible,
	}

	plan.Fail = compactDomainIDs(fails)
	if len(plan.Fail) == 0 {
		plan.Reason = "no constructive failing partition"
	}

	return plan, true, nil
}

// numberDomain registers number as a number-only Domain.
func (planner *CasePlanner) numberDomain(number NumberConstraints) DomainID {
	domain := singleKindDomain(jsonvalue.KindNumber)
	domain.Number = number

	return planner.Domains.FindOrAddEquivalentDomain(domain)
}

// stringDomain registers value as a string-only Domain.
func (planner *CasePlanner) stringDomain(value StringConstraints) DomainID {
	domain := singleKindDomain(jsonvalue.KindString)
	domain.String = value

	return planner.Domains.FindOrAddEquivalentDomain(domain)
}

// arrayDomain registers value as an array-only Domain.
func (planner *CasePlanner) arrayDomain(value ArrayConstraints) DomainID {
	domain := singleKindDomain(jsonvalue.KindArray)
	domain.Array = value

	return planner.Domains.FindOrAddEquivalentDomain(domain)
}

// objectDomain registers value as an object-only Domain.
func (planner *CasePlanner) objectDomain(value ObjectConstraints) DomainID {
	domain := singleKindDomain(jsonvalue.KindObject)
	domain.Object = value

	return planner.Domains.FindOrAddEquivalentDomain(domain)
}

// numberRuleDomain returns an all-kind Domain with number restrictions.
func numberRuleDomain(number NumberConstraints) Domain {
	domain := anyJSONDomain()
	domain.Number = number

	return domain
}

// stringRuleDomain returns an all-kind Domain with string restrictions.
func stringRuleDomain(value StringConstraints) Domain {
	domain := anyJSONDomain()
	domain.String = value

	return domain
}

// arrayRuleDomain returns an all-kind Domain with array restrictions.
func arrayRuleDomain(value ArrayConstraints) Domain {
	domain := anyJSONDomain()
	domain.Array = value

	return domain
}

// objectRuleDomain returns an all-kind Domain with object restrictions.
func objectRuleDomain(value ObjectConstraints) Domain {
	domain := anyJSONDomain()
	domain.Object = value

	return domain
}

// additionalPropertyRule permits declared properties while retaining the additional-property policy.
func additionalPropertyRule(source ObjectConstraints) ObjectConstraints {
	properties := make([]NamedProperty, 0, len(source.Properties))
	for _, property := range source.Properties {
		if property.State == PropertyForbidden {
			properties = append(properties, property)
		} else {
			properties = append(properties, NamedProperty{
				Name: property.Name, State: PropertyAllowed, Values: property.Values,
			})
		}
	}

	return ObjectConstraints{
		State:      KindRestricted,
		Properties: properties,
		Additional: source.Additional,
	}
}

// requiredRule retains only required properties with unconstrained allowed values.
func requiredRule(source ObjectConstraints) ObjectConstraints {
	result := objectValueRule(source)
	for index := range result.Properties {
		result.Properties[index].Required = source.Properties[index].Required
	}

	return result
}

// objectValueRule retains child value rules while leaving presence and count unconstrained.
func objectValueRule(source ObjectConstraints) ObjectConstraints {
	properties := cloneDomain(Domain{Object: source}).Object.Properties
	for index := range properties {
		properties[index].Required = false
	}

	return ObjectConstraints{
		State:      KindRestricted,
		Properties: properties,
		Additional: AdditionalProperties{Values: AnyJSONDomainID},
	}
}

// incrementInt returns value+1 without wrapping.
func incrementInt(value int) (int, bool) {
	if value == int(^uint(0)>>1) {
		return 0, false
	}

	return value + 1, true
}

// addIsolatedFailures uses cached prefix/suffix intersections so every candidate passes sibling rules.
func (planner *CasePlanner) addIsolatedFailures(result *caseSet) error {
	prefix, suffix, err := planner.isolationBounds()
	if err != nil {
		return err
	}

	for index := range planner.Constraints {
		constraint := &planner.Constraints[index]

		context, intersectErr := planner.Domains.IntersectDomains(prefix[index], suffix[index+1])
		if intersectErr != nil {
			return intersectErr
		}

		if err := planner.addConstraintFailures(result, constraint, context); err != nil {
			return err
		}
	}

	return nil
}

// isolationBounds builds the prefix and suffix passing intersections for all constraints.
func (planner *CasePlanner) isolationBounds() ([]DomainID, []DomainID, error) {
	prefix := make([]DomainID, len(planner.Constraints)+1)
	prefix[0] = AnyJSONDomainID

	for index, constraint := range planner.Constraints {
		intersection, err := planner.Domains.IntersectDomains(prefix[index], constraint.Pass)
		if err != nil {
			return nil, nil, err
		}

		prefix[index+1] = intersection
	}

	suffix := make([]DomainID, len(planner.Constraints)+1)
	suffix[len(planner.Constraints)] = AnyJSONDomainID

	for index := len(planner.Constraints) - 1; index >= 0; index-- {
		intersection, err := planner.Domains.IntersectDomains(planner.Constraints[index].Pass, suffix[index+1])
		if err != nil {
			return nil, nil, err
		}

		suffix[index] = intersection
	}

	return prefix, suffix, nil
}

// addConstraintFailures creates isolated rejected cases and records their obligation outcome.
func (planner *CasePlanner) addConstraintFailures(result *caseSet, constraint *ConstraintPlan, context DomainID) error {
	failures, err := planner.isolatedFailureDomains(*constraint, context)
	if err != nil {
		return err
	}

	planned, unconstructible, err := planner.addFailureCases(result, *constraint, context, failures)
	if err != nil {
		return err
	}

	if planned {
		constraint.Outcome = ObligationPlanned
		constraint.Reason = ""

		return nil
	}

	if unconstructible {
		constraint.Outcome = ObligationUnconstructible
		constraint.Reason = "isolated failure Domain is unconstructible"

		return nil
	}

	if len(failures) > 0 {
		if constraint.Source.Keyword == "pattern" || constraint.Source.Keyword == "format" {
			constraint.Outcome = ObligationUnconstructible
			constraint.Reason = "trusted failing examples do not isolate this constraint"
		} else {
			constraint.Outcome = ObligationDominated
			constraint.Reason = "failing partition is empty while all sibling constraints pass"
		}

		return nil
	}

	constraint.Outcome = ObligationUnconstructible
	if constraint.Reason == "" {
		constraint.Reason = "no constructive failing partition"
	}

	return nil
}

// isolatedFailureDomains combines a constraint's static and context-specific failing partitions.
func (planner *CasePlanner) isolatedFailureDomains(constraint ConstraintPlan, context DomainID) ([]DomainID, error) {
	failures := append([]DomainID(nil), constraint.Fail...)

	dynamicFailures, err := planner.contextFailures(constraint, context)
	if err != nil {
		return nil, err
	}

	return compactDomainIDs(append(failures, dynamicFailures...)), nil
}

// addFailureCases adds productive isolated failures and reports any unconstructible failure partitions.
func (planner *CasePlanner) addFailureCases(
	result *caseSet,
	constraint ConstraintPlan,
	context DomainID,
	failures []DomainID,
) (bool, bool, error) {
	planned := false
	unconstructible := false

	for failIndex, failure := range failures {
		values, err := planner.Domains.IntersectDomains(context, failure)
		if err != nil {
			return false, false, err
		}

		if values == EmptyDomainID {
			continue
		}

		domain, ok := planner.Domains.Domain(values)
		if !ok {
			return false, false, fmt.Errorf("isolated failure Domain %d does not exist", values)
		}

		if domain.Status == DomainUnconstructible || domain.Status == DomainUnsupported {
			unconstructible = true

			continue
		}

		if domain.Status != DomainProductive {
			continue
		}

		result.add(CasePlan{
			Name: caseName(
				fmt.Sprintf("invalid %s %d", constraint.Source.Keyword, failIndex+1),
				constraint.Source.Pointer,
				constraint.Source.Keyword,
			),
			Expect: ExpectRejected,
			Values: values,
			Source: constraint.Source,
		})

		planned = true
	}

	return planned, unconstructible, nil
}
