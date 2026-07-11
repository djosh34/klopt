//nolint:cyclop,gocyclo,gocognit,godoclint,lll,maintidx,mnd,nestif,nlreturn,revive,wsl_v5 // Exhaustive JSON planning.
package suite

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"

	//nolint:depguard // Internal suite architecture intentionally depends on internal/jsonvalue.
	"decode_and_validate_generator/pkg/test_generator/internal/jsonvalue"
)

// CompileSuite compiles, plans, and links the request schema to Rapid generators.
func (compiler *Compiler) CompileSuite() (*CompiledSuite, error) {
	root, err := compiler.Compile()
	if err != nil {
		return nil, err
	}

	planner := &CasePlanner{Domains: compiler.Domains, LocalDomains: compiler.LocalDomainByPointer}
	cases, err := planner.Plan(root, compiler.SchemaUses)
	if err != nil {
		return nil, err
	}

	generators := NewRapidGeneratorBuilder(compiler.Domains, compiler.SchemaUses)
	linked := make([]CasePlan, 0, len(cases))
	for index := range cases {
		generator, generatorErr := generators.Generator(cases[index].Values)
		if errors.Is(generatorErr, errNoTrustedStringExample) {
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
	for index := range planner.Constraints {
		constraint := &planner.Constraints[index]
		if constraint.Outcome != ObligationPlanned || hasRejectedCase(linked, constraint.Source) {
			continue
		}
		constraint.Outcome = ObligationUnconstructible
		constraint.Reason = "isolated failure has no trusted pattern or format example"
	}
	if rootDomain, ok := compiler.Domains.Domain(root); ok && rootDomain.Status == DomainProductive &&
		!hasAcceptedCase(linked) {
		return nil, compiler.failure(
			"generate",
			"unconstructible",
			compiler.Source.RequestSchema.Pointer,
			"",
			errNoTrustedStringExample,
		)
	}

	return &CompiledSuite{
		Root:        root,
		Domains:     compiler.Domains,
		SchemaUses:  append([]SchemaUse(nil), compiler.SchemaUses...),
		Constraints: append([]ConstraintPlan(nil), planner.Constraints...),
		Cases:       linked,
	}, nil
}

func hasRejectedCase(cases []CasePlan, source ConstraintSource) bool {
	for _, plannedCase := range cases {
		if plannedCase.Expect == ExpectRejected && plannedCase.Source == source {
			return true
		}
	}
	return false
}

func hasAcceptedCase(cases []CasePlan) bool {
	for _, plannedCase := range cases {
		if plannedCase.Expect == ExpectAccepted {
			return true
		}
	}
	return false
}

// Plan builds aggregate-valid, valid-partition, and isolated invalid CasePlans.
func (planner *CasePlanner) Plan(root DomainID, uses []SchemaUse) ([]CasePlan, error) {
	if planner == nil || planner.Domains == nil {
		return nil, errors.New("plan cases: Domain registry is nil")
	}

	rootDomain, ok := planner.Domains.Domain(root)
	if !ok {
		return nil, fmt.Errorf("plan cases: root Domain %d does not exist", root)
	}

	rootUse := rootSchemaUse(root, uses)
	constraints, err := planner.constraintPlans(rootUse, uses)
	if err != nil {
		return nil, err
	}

	planner.Constraints = constraints
	result := newCaseSet()
	if rootDomain.Status == DomainProductive {
		result.add(CasePlan{
			Name:   caseName("valid aggregate", rootUse.Pointer, ""),
			Expect: ExpectAccepted,
			Values: root,
			Source: ConstraintSource{Pointer: rootUse.Pointer},
		})
	}

	if err := planner.addIsolatedFailures(result); err != nil {
		return nil, err
	}

	if rootDomain.Status == DomainProductive {
		if err := planner.addValidPartitions(result, root, rootUse, uses, make(map[DomainID]bool)); err != nil {
			return nil, err
		}
	}

	return result.cases, nil
}

// rootSchemaUse returns the request occurrence, which compilation records last among equivalent roots.
func rootSchemaUse(root DomainID, uses []SchemaUse) SchemaUse {
	var requestUse SchemaUse
	for _, use := range uses {
		if use.Domain == root && strings.Contains(use.Pointer, "/requestBody/") &&
			(requestUse.Domain == NoDomain || len(use.Pointer) < len(requestUse.Pointer)) {
			requestUse = use
		}
	}
	if requestUse.Domain != NoDomain {
		return requestUse
	}

	for _, use := range uses {
		if use.Domain == root {
			return use
		}
	}

	return SchemaUse{Domain: root}
}

// constraintPlans creates atomic pass/fail Domains while retaining allOf source provenance.
func (planner *CasePlanner) constraintPlans(root SchemaUse, uses []SchemaUse) ([]ConstraintPlan, error) {
	plans := make([]ConstraintPlan, 0, len(root.Constraints))
	seen := make(map[ConstraintSource]struct{})

	for _, source := range root.Constraints {
		if _, duplicate := seen[source]; duplicate {
			continue
		}
		seen[source] = struct{}{}

		use := schemaUseByPointer(uses, source.Pointer)
		if use.Domain == NoDomain {
			use = root
		}
		if local, ok := planner.LocalDomains[source.Pointer]; ok {
			use.Domain = local
		}
		if (source.Keyword == "pattern" || source.Keyword == "format") && len(use.Examples.Invalid) == 0 {
			use.Examples.Invalid = cloneJSONValues(root.Examples.Invalid)
		}

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

func schemaUseByPointer(uses []SchemaUse, pointer string) SchemaUse {
	for _, use := range uses {
		if use.Pointer == pointer {
			return use
		}
	}
	return SchemaUse{}
}

// atomicConstraint constructs an applicability-correct atomic rule.
// Passing Domains leave unrelated JSON kinds unrestricted; failing Domains contain only the failing kind.
func (planner *CasePlanner) atomicConstraint(source ConstraintSource, use SchemaUse) (ConstraintPlan, bool, error) {
	domain, ok := planner.Domains.Domain(use.Domain)
	if !ok {
		return ConstraintPlan{}, false, nil
	}

	plan := ConstraintPlan{Source: source, Pass: AnyJSONDomainID, Outcome: ObligationUnconstructible}
	pass := anyJSONDomain()
	var fails []DomainID

	switch source.Keyword {
	case "allOf", "items", "properties":
		return ConstraintPlan{}, false, nil
	case "type":
		pass = kindMaskDomain(domain)
		for _, kind := range excludedKinds(domain) {
			fails = append(fails, planner.Domains.FindOrAddEquivalentDomain(singleKindDomain(kind)))
		}
		if domain.Number.State != KindExcluded && domain.Number.IntegersOnly {
			pass.Number.IntegersOnly = true
			pass.Number.State = KindRestricted
			fails = append(fails, planner.finiteNumberFailures(fractionalCandidates())...)
		}
	case "nullable":
		if domain.Null == KindExcluded {
			fails = append(fails, planner.Domains.FindOrAddEquivalentDomain(singleKindDomain(jsonvalue.KindNull)))
		} else {
			return ConstraintPlan{}, false, nil
		}
	case "enum":
		pass = domain
		for _, candidate := range outsiderCandidates(domain.Enum) {
			fails = append(fails, planner.Domains.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{candidate})))
		}
	case "minimum":
		if domain.Number.Minimum == nil {
			return ConstraintPlan{}, false, nil
		}
		minimum := cloneBound(domain.Number.Minimum)
		minimum.Exclusive = false
		pass = numberRuleDomain(NumberConstraints{State: KindRestricted, Minimum: minimum})
		failure := cloneBound(minimum)
		failure.Exclusive = true
		fails = []DomainID{planner.numberDomain(NumberConstraints{State: KindRestricted, Maximum: failure})}
	case "exclusiveMinimum":
		if domain.Number.Minimum == nil || !domain.Number.Minimum.Exclusive {
			return ConstraintPlan{}, false, nil
		}
		pass = numberRuleDomain(NumberConstraints{State: KindRestricted, Minimum: cloneBound(domain.Number.Minimum)})
		equal := cloneBound(domain.Number.Minimum)
		equal.Exclusive = false
		fails = []DomainID{planner.numberDomain(NumberConstraints{
			State: KindRestricted, Minimum: equal, Maximum: equal,
		})}
	case "maximum":
		if domain.Number.Maximum == nil {
			return ConstraintPlan{}, false, nil
		}
		maximum := cloneBound(domain.Number.Maximum)
		maximum.Exclusive = false
		pass = numberRuleDomain(NumberConstraints{State: KindRestricted, Maximum: maximum})
		failure := cloneBound(maximum)
		failure.Exclusive = true
		fails = []DomainID{planner.numberDomain(NumberConstraints{State: KindRestricted, Minimum: failure})}
	case "exclusiveMaximum":
		if domain.Number.Maximum == nil || !domain.Number.Maximum.Exclusive {
			return ConstraintPlan{}, false, nil
		}
		pass = numberRuleDomain(NumberConstraints{State: KindRestricted, Maximum: cloneBound(domain.Number.Maximum)})
		equal := cloneBound(domain.Number.Maximum)
		equal.Exclusive = false
		fails = []DomainID{planner.numberDomain(NumberConstraints{
			State: KindRestricted, Minimum: equal, Maximum: equal,
		})}
	case "multipleOf":
		if domain.Number.MultipleOf == nil {
			return ConstraintPlan{}, false, nil
		}
		pass = numberRuleDomain(NumberConstraints{State: KindRestricted, MultipleOf: cloneNumber(domain.Number.MultipleOf)})
		candidates, err := nonMultipleCandidates(domain.Number.MultipleOf)
		if err != nil {
			return ConstraintPlan{}, false, err
		}
		fails = planner.finiteNumberFailures(candidates)
	case "minLength":
		pass = stringRuleDomain(StringConstraints{State: KindRestricted, MinLength: domain.String.MinLength})
		if domain.String.MinLength > 0 {
			fails = []DomainID{planner.stringDomain(StringConstraints{
				State: KindRestricted, MaxLength: new(domain.String.MinLength - 1),
			})}
		}
	case "maxLength":
		if domain.String.MaxLength == nil {
			return ConstraintPlan{}, false, nil
		}
		pass = stringRuleDomain(StringConstraints{State: KindRestricted, MaxLength: new(*domain.String.MaxLength)})
		fails = []DomainID{planner.stringDomain(StringConstraints{
			State: KindRestricted, MinLength: *domain.String.MaxLength + 1,
		})}
	case "pattern", "format":
		pass = anyJSONDomain()
		pass.String = StringConstraints{State: KindRestricted}
		if source.Keyword == "pattern" {
			pass.String.Patterns = append([]string(nil), domain.String.Patterns...)
		} else {
			pass.String.Formats = append([]string(nil), domain.String.Formats...)
		}
		for _, example := range use.Examples.Invalid {
			if example.Kind == jsonvalue.KindString {
				fails = append(fails, planner.Domains.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{example})))
			}
		}
	case "minItems":
		pass = arrayRuleDomain(ArrayConstraints{
			State: KindRestricted, Items: AnyJSONDomainID, MinItems: domain.Array.MinItems,
		})
		if domain.Array.MinItems > 0 {
			fails = []DomainID{planner.arrayDomain(ArrayConstraints{
				State: KindRestricted, Items: AnyJSONDomainID, MaxItems: new(domain.Array.MinItems - 1),
			})}
		}
	case "maxItems":
		if domain.Array.MaxItems == nil {
			return ConstraintPlan{}, false, nil
		}
		pass = arrayRuleDomain(ArrayConstraints{
			State: KindRestricted, Items: AnyJSONDomainID, MaxItems: new(*domain.Array.MaxItems),
		})
		fails = []DomainID{planner.arrayDomain(ArrayConstraints{
			State: KindRestricted, Items: AnyJSONDomainID, MinItems: *domain.Array.MaxItems + 1,
		})}
	case "minProperties":
		pass = objectRuleDomain(ObjectConstraints{
			State: KindRestricted, Additional: AdditionalProperties{Values: AnyJSONDomainID}, MinProps: domain.Object.MinProps,
		})
		if domain.Object.MinProps > 0 {
			fails = []DomainID{planner.objectDomain(ObjectConstraints{
				State: KindRestricted, Additional: AdditionalProperties{Values: AnyJSONDomainID},
				MaxProps: new(domain.Object.MinProps - 1),
			})}
		}
	case "maxProperties":
		if domain.Object.MaxProps == nil {
			return ConstraintPlan{}, false, nil
		}
		pass = objectRuleDomain(ObjectConstraints{
			State: KindRestricted, Additional: AdditionalProperties{Values: AnyJSONDomainID}, MaxProps: new(*domain.Object.MaxProps),
		})
		fails = []DomainID{planner.objectDomain(ObjectConstraints{
			State: KindRestricted, Additional: AdditionalProperties{Values: AnyJSONDomainID},
			MinProps: *domain.Object.MaxProps + 1,
		})}
	case "required":
		pass = objectRuleDomain(requiredRule(domain.Object))
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
	case "additionalProperties":
		policy := additionalPropertyRule(domain.Object)
		pass = objectRuleDomain(policy)
		if domain.Object.Additional.Values == EmptyDomainID {
			failure := policy
			failure.Properties = append(failure.Properties, NamedProperty{
				Name: unusedPropertyName(domain.Object), Required: true,
				State: PropertyAllowed, Values: AnyJSONDomainID,
			})
			fails = []DomainID{planner.objectDomain(failure)}
		}
	default:
		return ConstraintPlan{}, false, nil
	}

	plan.Pass = planner.Domains.FindOrAddEquivalentDomain(pass)
	plan.Fail = compactDomainIDs(fails)
	if len(plan.Fail) == 0 {
		plan.Reason = "no constructive failing partition"
	}

	return plan, true, nil
}

func (planner *CasePlanner) numberDomain(number NumberConstraints) DomainID {
	domain := singleKindDomain(jsonvalue.KindNumber)
	domain.Number = number
	return planner.Domains.FindOrAddEquivalentDomain(domain)
}

func (planner *CasePlanner) stringDomain(value StringConstraints) DomainID {
	domain := singleKindDomain(jsonvalue.KindString)
	domain.String = value
	return planner.Domains.FindOrAddEquivalentDomain(domain)
}

func (planner *CasePlanner) arrayDomain(value ArrayConstraints) DomainID {
	domain := singleKindDomain(jsonvalue.KindArray)
	domain.Array = value
	return planner.Domains.FindOrAddEquivalentDomain(domain)
}

func (planner *CasePlanner) objectDomain(value ObjectConstraints) DomainID {
	domain := singleKindDomain(jsonvalue.KindObject)
	domain.Object = value
	return planner.Domains.FindOrAddEquivalentDomain(domain)
}

func numberRuleDomain(number NumberConstraints) Domain {
	domain := anyJSONDomain()
	domain.Number = number
	return domain
}

func stringRuleDomain(value StringConstraints) Domain {
	domain := anyJSONDomain()
	domain.String = value
	return domain
}

func arrayRuleDomain(value ArrayConstraints) Domain {
	domain := anyJSONDomain()
	domain.Array = value
	return domain
}

func objectRuleDomain(value ObjectConstraints) Domain {
	domain := anyJSONDomain()
	domain.Object = value
	return domain
}

func additionalPropertyRule(source ObjectConstraints) ObjectConstraints {
	properties := make([]NamedProperty, 0, len(source.Properties))
	for _, property := range source.Properties {
		if property.State == PropertyForbidden {
			properties = append(properties, property)
			continue
		}
		properties = append(properties, NamedProperty{
			Name: property.Name, State: PropertyAllowed, Values: AnyJSONDomainID,
		})
	}
	return ObjectConstraints{
		State: KindRestricted, Properties: properties, Additional: source.Additional,
	}
}

func requiredRule(source ObjectConstraints) ObjectConstraints {
	properties := make([]NamedProperty, 0, len(source.Properties))
	for _, property := range source.Properties {
		if property.Required {
			properties = append(properties, NamedProperty{
				Name: property.Name, Required: true, State: PropertyAllowed, Values: AnyJSONDomainID,
			})
		}
	}
	return ObjectConstraints{
		State: KindRestricted, Properties: properties,
		Additional: AdditionalProperties{Values: AnyJSONDomainID},
	}
}

// addIsolatedFailures uses cached prefix/suffix intersections so every candidate passes sibling rules.
func (planner *CasePlanner) addIsolatedFailures(result *caseSet) error {
	prefix := make([]DomainID, len(planner.Constraints)+1)
	prefix[0] = AnyJSONDomainID
	for index, constraint := range planner.Constraints {
		intersection, err := planner.Domains.IntersectDomains(prefix[index], constraint.Pass)
		if err != nil {
			return err
		}
		prefix[index+1] = intersection
	}

	suffix := make([]DomainID, len(planner.Constraints)+1)
	suffix[len(planner.Constraints)] = AnyJSONDomainID
	for index := len(planner.Constraints) - 1; index >= 0; index-- {
		intersection, err := planner.Domains.IntersectDomains(planner.Constraints[index].Pass, suffix[index+1])
		if err != nil {
			return err
		}
		suffix[index] = intersection
	}

	for index := range planner.Constraints {
		constraint := &planner.Constraints[index]
		context, err := planner.Domains.IntersectDomains(prefix[index], suffix[index+1])
		if err != nil {
			return err
		}

		failures := append([]DomainID(nil), constraint.Fail...)
		dynamicFailures, err := planner.contextFailures(*constraint, context)
		if err != nil {
			return err
		}
		failures = compactDomainIDs(append(failures, dynamicFailures...))

		planned := false
		unconstructible := false
		for failIndex, failure := range failures {
			values, intersectErr := planner.Domains.IntersectDomains(context, failure)
			if intersectErr != nil {
				return intersectErr
			}
			if values == EmptyDomainID {
				continue
			}
			domain, ok := planner.Domains.Domain(values)
			if !ok {
				return fmt.Errorf("isolated failure Domain %d does not exist", values)
			}
			if domain.Status == DomainUnconstructible || domain.Status == DomainUnsupported {
				unconstructible = true
				continue
			}
			if domain.Status != DomainProductive {
				continue
			}

			result.add(CasePlan{
				Name:   caseName(fmt.Sprintf("invalid %s %d", constraint.Source.Keyword, failIndex+1), constraint.Source.Pointer, constraint.Source.Keyword),
				Expect: ExpectRejected,
				Values: values,
				Source: constraint.Source,
			})
			planned = true
		}

		if planned {
			constraint.Outcome = ObligationPlanned
			constraint.Reason = ""
		} else if unconstructible {
			constraint.Outcome = ObligationUnconstructible
			constraint.Reason = "isolated failure Domain is unconstructible"
		} else if len(failures) > 0 {
			constraint.Outcome = ObligationDominated
			constraint.Reason = "failing partition is empty while all sibling constraints pass"
		} else {
			constraint.Outcome = ObligationUnconstructible
			if constraint.Reason == "" {
				constraint.Reason = "no constructive failing partition"
			}
		}
	}

	return nil
}

// addValidPartitions adds kind/classes and recursively lifts child partitions by DomainID.
func (planner *CasePlanner) addValidPartitions(
	result *caseSet,
	id DomainID,
	use SchemaUse,
	uses []SchemaUse,
	active map[DomainID]bool,
) error {
	if active[id] {
		return nil
	}
	active[id] = true
	defer delete(active, id)

	domain, ok := planner.Domains.Domain(id)
	if !ok || domain.Status != DomainProductive {
		return nil
	}

	source := ConstraintSource{Pointer: use.Pointer}
	for _, kind := range reachableKinds(domain) {
		partition, err := planner.Domains.IntersectDomains(id, planner.Domains.FindOrAddEquivalentDomain(singleKindDomain(kind)))
		if err != nil {
			return err
		}
		result.add(CasePlan{Name: caseName("valid kind "+kindName(kind), use.Pointer, ""), Expect: ExpectAccepted, Values: partition, Source: source})
	}

	if domain.Enum != nil {
		for index, value := range domain.Enum.Values {
			member := planner.Domains.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{value}))
			result.add(CasePlan{Name: caseName(fmt.Sprintf("valid enum member %d", index+1), use.Pointer, "enum"), Expect: ExpectAccepted, Values: member, Source: ConstraintSource{Pointer: use.Pointer, Keyword: "enum"}})
		}
		return nil
	}

	if err := planner.addScalarValidPartitions(result, id, domain, use); err != nil {
		return err
	}
	if err := planner.addArrayPartitions(result, id, domain, use, uses, active); err != nil {
		return err
	}
	return planner.addObjectPartitions(result, id, domain, use, uses, active)
}

func (planner *CasePlanner) addScalarValidPartitions(
	result *caseSet,
	root DomainID,
	domain Domain,
	use SchemaUse,
) error {
	if domain.Number.State != KindExcluded {
		bounds := []struct {
			label string
			bound *NumberBound
		}{
			{label: "minimum", bound: domain.Number.Minimum},
			{label: "maximum", bound: domain.Number.Maximum},
		}
		for _, entry := range bounds {
			if entry.bound == nil || entry.bound.Exclusive {
				continue
			}
			exact := NumberConstraints{
				State: KindRestricted, Minimum: cloneBound(entry.bound), Maximum: cloneBound(entry.bound),
			}
			candidate := planner.numberDomain(exact)
			value, err := planner.Domains.IntersectDomains(root, candidate)
			if err != nil {
				return err
			}
			if value != EmptyDomainID {
				result.add(CasePlan{
					Name:   caseName("valid number "+entry.label+" boundary", use.Pointer, entry.label),
					Expect: ExpectAccepted,
					Values: value,
					Source: ConstraintSource{Pointer: use.Pointer, Keyword: entry.label},
				})
			}
		}
	}

	if domain.String.State != KindExcluded {
		lengths := []struct {
			label  string
			length int
		}{{"minimum", domain.String.MinLength}}
		if domain.String.MaxLength != nil {
			lengths = append(lengths, struct {
				label  string
				length int
			}{"maximum", *domain.String.MaxLength})
		}
		for _, length := range lengths {
			candidate := planner.stringDomain(StringConstraints{
				State: KindRestricted, MinLength: length.length, MaxLength: new(length.length),
			})
			value, err := planner.Domains.IntersectDomains(root, candidate)
			if err != nil {
				return err
			}
			if value != EmptyDomainID {
				result.add(CasePlan{
					Name: caseName(
						"valid string "+length.label+" length", use.Pointer, length.label+"Length",
					),
					Expect: ExpectAccepted,
					Values: value,
					Source: ConstraintSource{Pointer: use.Pointer, Keyword: length.label + "Length"},
				})
			}
		}
		for index, example := range use.Examples.Valid {
			if example.Kind != jsonvalue.KindString {
				continue
			}
			candidate := planner.Domains.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{example}))
			value, err := planner.Domains.IntersectDomains(root, candidate)
			if err != nil {
				return err
			}
			if value != EmptyDomainID {
				result.add(CasePlan{
					Name: caseName(
						fmt.Sprintf("valid trusted string example %d", index+1),
						use.Pointer,
						"pattern/format",
					),
					Expect: ExpectAccepted,
					Values: value,
					Source: ConstraintSource{Pointer: use.Pointer, Keyword: "pattern/format"},
				})
			}
		}
	}

	return nil
}

func (planner *CasePlanner) addArrayPartitions(result *caseSet, root DomainID, domain Domain, use SchemaUse, uses []SchemaUse, active map[DomainID]bool) error {
	if domain.Array.State == KindExcluded {
		return nil
	}

	counts := []struct {
		label string
		value int
	}{{"minimum", domain.Array.MinItems}}
	if domain.Array.MaxItems != nil {
		counts = append(counts, struct {
			label string
			value int
		}{"maximum", *domain.Array.MaxItems})
	}
	for _, count := range counts {
		candidate := cloneDomain(domain)
		candidate.Null, candidate.Boolean = KindExcluded, KindExcluded
		candidate.Number.State, candidate.String.State, candidate.Object.State = KindExcluded, KindExcluded, KindExcluded
		candidate.Array.MinItems, candidate.Array.MaxItems = count.value, new(count.value)
		value := planner.Domains.FindOrAddEquivalentDomain(candidate)
		if value != EmptyDomainID {
			result.add(CasePlan{Name: caseName("valid array "+count.label+" count", use.Pointer, count.label+"Items"), Expect: ExpectAccepted, Values: value, Source: ConstraintSource{Pointer: use.Pointer, Keyword: count.label + "Items"}})
		}
	}

	if domain.Array.Items == AnyJSONDomainID || domain.Array.Items == EmptyDomainID || domain.Array.MaxItems != nil && *domain.Array.MaxItems == 0 {
		return nil
	}
	childUse := childSchemaUse(uses, use.Pointer+"/items", domain.Array.Items)
	childCases, err := planner.childPartitions(domain.Array.Items, childUse, uses, active)
	if err != nil {
		return err
	}

	for _, child := range childCases {
		if child.Values == domain.Array.Items && child.Expect == ExpectAccepted {
			continue
		}
		lifted := cloneDomain(domain)
		lifted.Null, lifted.Boolean = KindExcluded, KindExcluded
		lifted.Number.State, lifted.String.State, lifted.Object.State = KindExcluded, KindExcluded, KindExcluded
		lifted.Array.Items = child.Values
		lifted.Array.MinItems = max(1, lifted.Array.MinItems)
		if lifted.Array.MaxItems != nil && lifted.Array.MinItems > *lifted.Array.MaxItems {
			continue
		}
		values := planner.Domains.FindOrAddEquivalentDomain(lifted)
		result.add(CasePlan{
			Name:   caseName(expectName(child.Expect)+" array item / "+child.Name, use.Pointer, "items"),
			Expect: child.Expect,
			Values: values,
			Source: child.Source,
		})
	}
	return nil
}

//nolint:gocognit,cyclop // Object shape and child lifting are intentionally kept at one planning seam.
func (planner *CasePlanner) addObjectPartitions(result *caseSet, root DomainID, domain Domain, use SchemaUse, uses []SchemaUse, active map[DomainID]bool) error {
	if domain.Object.State == KindExcluded {
		return nil
	}

	counts := []struct {
		label string
		value int
	}{{label: "minimum", value: domain.Object.MinProps}}
	if domain.Object.MaxProps != nil {
		counts = append(counts, struct {
			label string
			value int
		}{label: "maximum", value: *domain.Object.MaxProps})
	}
	for _, count := range counts {
		candidate := objectOnly(domain)
		candidate.Object.MinProps = count.value
		candidate.Object.MaxProps = new(count.value)
		values := planner.Domains.FindOrAddEquivalentDomain(candidate)
		if values != EmptyDomainID {
			result.add(CasePlan{
				Name: caseName(
					"valid object "+count.label+" count", use.Pointer, count.label+"Properties",
				),
				Expect: ExpectAccepted,
				Values: values,
				Source: ConstraintSource{Pointer: use.Pointer, Keyword: count.label + "Properties"},
			})
		}
	}

	for _, property := range domain.Object.Properties {
		if property.State == PropertyForbidden {
			failure := objectOnly(domain)
			for index := range failure.Object.Properties {
				if failure.Object.Properties[index].Name == property.Name {
					failure.Object.Properties[index].State = PropertyAllowed
					failure.Object.Properties[index].Required = true
					failure.Object.Properties[index].Values = AnyJSONDomainID
				}
			}
			values := planner.Domains.FindOrAddEquivalentDomain(failure)
			if values != EmptyDomainID {
				result.add(CasePlan{Name: caseName("invalid forbidden property "+property.Name, use.Pointer, "additionalProperties"), Expect: ExpectRejected, Values: values, Source: ConstraintSource{Pointer: use.Pointer, Keyword: "additionalProperties"}})
			}
			continue
		}

		if !property.Required {
			present := objectOnly(domain)
			absent := objectOnly(domain)
			for index := range present.Object.Properties {
				if present.Object.Properties[index].Name == property.Name {
					present.Object.Properties[index].Required = true
					absent.Object.Properties[index].State = PropertyForbidden
					absent.Object.Properties[index].Values = EmptyDomainID
				}
			}
			shapes := []struct {
				label     string
				candidate Domain
			}{
				{label: "present", candidate: present},
				{label: "absent", candidate: absent},
			}
			for _, shape := range shapes {
				values := planner.Domains.FindOrAddEquivalentDomain(shape.candidate)
				if values != EmptyDomainID {
					result.add(CasePlan{
						Name: caseName(
							"valid optional property "+property.Name+" "+shape.label,
							use.Pointer,
							"properties",
						),
						Expect: ExpectAccepted,
						Values: values,
						Source: ConstraintSource{Pointer: use.Pointer, Keyword: "properties"},
					})
				}
			}
		}

		if property.Values == AnyJSONDomainID || property.Values == EmptyDomainID {
			continue
		}
		childUse := childSchemaUse(uses, use.Pointer+"/properties/"+escapePointerToken(property.Name), property.Values)
		childCases, err := planner.childPartitions(property.Values, childUse, uses, active)
		if err != nil {
			return err
		}
		for _, child := range childCases {
			if child.Expect == ExpectAccepted && child.Values == property.Values {
				continue
			}
			lifted := objectOnly(domain)
			for index := range lifted.Object.Properties {
				if lifted.Object.Properties[index].Name == property.Name {
					lifted.Object.Properties[index].Required = true
					lifted.Object.Properties[index].Values = child.Values
				}
			}
			values := planner.Domains.FindOrAddEquivalentDomain(lifted)
			if values == EmptyDomainID {
				continue
			}
			expect := child.Expect
			result.add(CasePlan{Name: caseName(expectName(expect)+" property "+property.Name+" / "+child.Name, use.Pointer, "properties"), Expect: expect, Values: values, Source: child.Source})
		}
	}

	if domain.Object.Additional.Values == AnyJSONDomainID {
		lifted := objectOnly(domain)
		lifted.Object.Properties = append(lifted.Object.Properties, NamedProperty{
			Name: unusedPropertyName(domain.Object), Required: true, State: PropertyAllowed, Values: AnyJSONDomainID,
		})
		values := planner.Domains.FindOrAddEquivalentDomain(lifted)
		if values != EmptyDomainID {
			result.add(CasePlan{
				Name:   caseName("valid additional property", use.Pointer, "additionalProperties"),
				Expect: ExpectAccepted,
				Values: values,
				Source: ConstraintSource{Pointer: use.Pointer, Keyword: "additionalProperties"},
			})
		}
	}

	if domain.Object.Additional.Values != AnyJSONDomainID && domain.Object.Additional.Values != EmptyDomainID {
		childUse := childSchemaUse(uses, use.Pointer+"/additionalProperties", domain.Object.Additional.Values)
		childCases, err := planner.childPartitions(domain.Object.Additional.Values, childUse, uses, active)
		if err != nil {
			return err
		}
		name := unusedPropertyName(domain.Object)
		for _, child := range childCases {
			lifted := objectOnly(domain)
			lifted.Object.Properties = append(lifted.Object.Properties, NamedProperty{Name: name, Required: true, State: PropertyAllowed, Values: child.Values})
			values := planner.Domains.FindOrAddEquivalentDomain(lifted)
			if values == EmptyDomainID {
				continue
			}
			result.add(CasePlan{Name: caseName(expectName(child.Expect)+" additional property / "+child.Name, use.Pointer, "additionalProperties"), Expect: child.Expect, Values: values, Source: child.Source})
		}
	}
	return nil
}

func (planner *CasePlanner) childPartitions(id DomainID, use SchemaUse, uses []SchemaUse, active map[DomainID]bool) ([]CasePlan, error) {
	children := newCaseSet()
	children.add(CasePlan{Name: caseName("valid aggregate", use.Pointer, ""), Expect: ExpectAccepted, Values: id, Source: ConstraintSource{Pointer: use.Pointer}})
	if err := planner.addValidPartitions(children, id, use, uses, active); err != nil {
		return nil, err
	}

	childPlanner := &CasePlanner{Domains: planner.Domains, LocalDomains: planner.LocalDomains}
	constraints, err := childPlanner.constraintPlans(use, uses)
	if err != nil {
		return nil, err
	}
	childPlanner.Constraints = constraints
	if err := childPlanner.addIsolatedFailures(children); err != nil {
		return nil, err
	}
	planner.Constraints = append(planner.Constraints, childPlanner.Constraints...)
	return children.cases, nil
}

func childSchemaUse(uses []SchemaUse, pointer string, id DomainID) SchemaUse {
	if exact := schemaUseByPointer(uses, pointer); exact.Domain != NoDomain {
		return exact
	}
	for _, use := range uses {
		if use.Domain == id {
			return use
		}
	}
	return SchemaUse{Pointer: pointer, Domain: id}
}

func objectOnly(domain Domain) Domain {
	result := cloneDomain(domain)
	result.Null, result.Boolean = KindExcluded, KindExcluded
	result.Number.State, result.String.State, result.Array.State = KindExcluded, KindExcluded, KindExcluded
	result.Enum = nil
	return result
}

func expectName(expect ExpectedResult) string {
	if expect == ExpectRejected {
		return "invalid"
	}
	return "valid"
}

func unusedPropertyName(object ObjectConstraints) string {
	names := propertyConstraintsByName(object.Properties)
	name := "additional"
	for suffix := 1; ; suffix++ {
		if _, used := names[name]; !used {
			return name
		}
		name = fmt.Sprintf("additional%d", suffix)
	}
}

func escapePointerToken(token string) string {
	return strings.ReplaceAll(strings.ReplaceAll(token, "~", "~0"), "/", "~1")
}

func kindMaskDomain(source Domain) Domain {
	result := emptyDomain()
	result.Status = DomainProductive
	result.Null, result.Boolean = source.Null, source.Boolean
	result.Number.State = source.Number.State
	result.String.State = source.String.State
	result.Array.State, result.Array.Items = source.Array.State, AnyJSONDomainID
	result.Object.State = source.Object.State
	result.Object.Additional.Values = AnyJSONDomainID
	return result
}

func singleKindDomain(kind jsonvalue.Kind) Domain {
	domain := emptyDomain()
	domain.Status = DomainProductive
	switch kind {
	case jsonvalue.KindNull:
		domain.Null = KindUnrestricted
	case jsonvalue.KindBoolean:
		domain.Boolean = KindUnrestricted
	case jsonvalue.KindNumber:
		domain.Number.State = KindUnrestricted
	case jsonvalue.KindString:
		domain.String.State = KindUnrestricted
	case jsonvalue.KindArray:
		domain.Array = ArrayConstraints{State: KindUnrestricted, Items: AnyJSONDomainID}
	case jsonvalue.KindObject:
		domain.Object = ObjectConstraints{State: KindUnrestricted, Additional: AdditionalProperties{Values: AnyJSONDomainID}}
	}
	return domain
}

func reachableKinds(domain Domain) []jsonvalue.Kind {
	result := make([]jsonvalue.Kind, 0, 6)
	if domain.Null != KindExcluded {
		result = append(result, jsonvalue.KindNull)
	}
	if domain.Boolean != KindExcluded {
		result = append(result, jsonvalue.KindBoolean)
	}
	if domain.Number.State != KindExcluded {
		result = append(result, jsonvalue.KindNumber)
	}
	if domain.String.State != KindExcluded {
		result = append(result, jsonvalue.KindString)
	}
	if domain.Array.State != KindExcluded {
		result = append(result, jsonvalue.KindArray)
	}
	if domain.Object.State != KindExcluded {
		result = append(result, jsonvalue.KindObject)
	}
	return result
}

func excludedKinds(domain Domain) []jsonvalue.Kind {
	result := make([]jsonvalue.Kind, 0, 6)
	for _, kind := range []jsonvalue.Kind{jsonvalue.KindNull, jsonvalue.KindBoolean, jsonvalue.KindNumber, jsonvalue.KindString, jsonvalue.KindArray, jsonvalue.KindObject} {
		if !kindReachable(domain, kind) {
			result = append(result, kind)
		}
	}
	return result
}

func kindReachable(domain Domain, kind jsonvalue.Kind) bool {
	switch kind {
	case jsonvalue.KindNull:
		return domain.Null != KindExcluded
	case jsonvalue.KindBoolean:
		return domain.Boolean != KindExcluded
	case jsonvalue.KindNumber:
		return domain.Number.State != KindExcluded
	case jsonvalue.KindString:
		return domain.String.State != KindExcluded
	case jsonvalue.KindArray:
		return domain.Array.State != KindExcluded
	case jsonvalue.KindObject:
		return domain.Object.State != KindExcluded
	default:
		return false
	}
}

func kindName(kind jsonvalue.Kind) string {
	return [...]string{"null", "boolean", "number", "string", "array", "object"}[kind]
}

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
	candidates, err := numberWitnessCandidates(contextDomain.Number)
	if err != nil {
		return nil, err
	}
	if enumFailure {
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
				result = append(result, planner.Domains.FindOrAddEquivalentDomain(
					finiteDomain([]jsonvalue.Value{value}),
				))
			}
		}
		return compactDomainIDs(result), nil
	}

	selected := make([]jsonvalue.Number, 0, len(candidates))
	for _, candidate := range candidates {
		if integerFailure && candidate.Rational != nil && !candidate.Rational.IsInt() {
			selected = append(selected, candidate)
			continue
		}
		if multipleFailure {
			fits, fitErr := fitsMultipleOf(candidate, pass.Number.MultipleOf)
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

func numberWitnessCandidates(constraints NumberConstraints) ([]jsonvalue.Number, error) {
	result := append(basicNumbers(), fractionalCandidates()...)
	rationals := make([]*big.Rat, 0, 8)
	if constraints.Minimum != nil && constraints.Minimum.Value.Rational != nil {
		minimum := constraints.Minimum.Value.Rational
		rationals = append(rationals,
			new(big.Rat).Set(minimum),
			new(big.Rat).Add(minimum, big.NewRat(1, 2)),
			new(big.Rat).Add(minimum, big.NewRat(1, 1)),
		)
	}
	if constraints.Maximum != nil && constraints.Maximum.Value.Rational != nil {
		maximum := constraints.Maximum.Value.Rational
		rationals = append(rationals,
			new(big.Rat).Set(maximum),
			new(big.Rat).Sub(maximum, big.NewRat(1, 2)),
			new(big.Rat).Sub(maximum, big.NewRat(1, 1)),
		)
	}
	if constraints.Minimum != nil && constraints.Maximum != nil &&
		constraints.Minimum.Value.Rational != nil && constraints.Maximum.Value.Rational != nil {
		midpoint := new(big.Rat).Add(
			constraints.Minimum.Value.Rational,
			constraints.Maximum.Value.Rational,
		)
		midpoint.Quo(midpoint, big.NewRat(2, 1))
		rationals = append(rationals, midpoint, new(big.Rat).Add(midpoint, big.NewRat(1, 2)))
	}
	for _, rational := range rationals {
		candidate, err := exactJSONNumberFromRat(rational)
		if err != nil {
			return nil, err
		}
		result = append(result, *candidate)
	}
	return result, nil
}

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

func nonMultipleCandidates(multiple *jsonvalue.Number) ([]jsonvalue.Number, error) {
	result := make([]jsonvalue.Number, 0, 8)
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
		value := new(big.Rat).Quo(multiple.Rational, big.NewRat(2, 1))
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

func basicNumbers() []jsonvalue.Number {
	return []jsonvalue.Number{
		{Lexeme: "0", Rational: new(big.Rat)},
		{Lexeme: "1", Rational: big.NewRat(1, 1)},
		{Lexeme: "-1", Rational: big.NewRat(-1, 1)},
		{Lexeme: "2", Rational: big.NewRat(2, 1)},
	}
}

func fractionalCandidates() []jsonvalue.Number {
	return []jsonvalue.Number{
		{Lexeme: "0.5", Rational: big.NewRat(1, 2)},
		{Lexeme: "-0.5", Rational: big.NewRat(-1, 2)},
		{Lexeme: "1.5", Rational: big.NewRat(3, 2)},
	}
}

func (planner *CasePlanner) finiteNumberFailures(numbers []jsonvalue.Number) []DomainID {
	result := make([]DomainID, 0, len(numbers))
	for _, number := range numbers {
		result = append(result, planner.Domains.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{{Kind: jsonvalue.KindNumber, Number: number}})))
	}
	return compactDomainIDs(result)
}

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

type caseSet struct {
	cases        []CasePlan
	seen         map[string]struct{}
	seenAccepted map[DomainID]struct{}
}

func newCaseSet() *caseSet {
	return &caseSet{seen: make(map[string]struct{}), seenAccepted: make(map[DomainID]struct{})}
}

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
	key := fmt.Sprintf("%d\x00%d\x00%s\x00%s\x00%s", plan.Expect, plan.Values, plan.Name, plan.Source.Pointer, plan.Source.Keyword)
	if _, duplicate := set.seen[key]; duplicate {
		return
	}
	set.seen[key] = struct{}{}
	set.cases = append(set.cases, plan)
}

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
