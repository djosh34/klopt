package suite

import (
	"fmt"
	"sort"
	"strings"

	"decode_and_validate_generator/pkg/internal/jsonvalue"
)

// labeledIntBoundary identifies an integer boundary used to name a partition.
type labeledIntBoundary struct {
	label string
	value int
}

// exactStructuralCollectionLimit bounds deterministic witness materialization.
const exactStructuralCollectionLimit = 4096

// addValidPartitions adds kind/classes and recursively lifts child partitions by DomainID.
func (planner *CasePlanner) addValidPartitions(
	result *caseSet,
	use *schemaUse,
	active map[DomainID]bool,
) error {
	id := use.domain
	if active[id] {
		return nil
	}

	active[id] = true
	defer delete(active, id)

	domain, ok := planner.Domains.Domain(id)
	if !ok || domain.Status != DomainProductive {
		return nil
	}

	source := ConstraintSource{Pointer: use.pointer}
	planner.addExactEvidenceCases(result, use, use.examples.Valid, ExpectAccepted)

	if err := planner.addKindPartitions(result, id, domain, source); err != nil {
		return err
	}

	if domain.Enum != nil {
		return nil
	}

	if err := planner.addScalarValidPartitions(result, id, domain, use); err != nil {
		return err
	}

	if err := planner.addArrayPartitions(result, domain, use, active); err != nil {
		return err
	}

	return planner.addObjectPartitions(result, domain, use, active)
}

// addKindPartitions adds one accepted partition per constructible JSON kind.
func (planner *CasePlanner) addKindPartitions(
	result *caseSet,
	id DomainID,
	domain Domain,
	source ConstraintSource,
) error {
	for _, kind := range reachableKinds(domain) {
		if kind == jsonvalue.KindString && hasOpaqueStringConstraints(domain.String) {
			continue
		}

		kindDomain := planner.Domains.FindOrAddEquivalentDomain(singleKindDomain(kind))

		partition, err := planner.Domains.IntersectDomains(id, kindDomain)
		if err != nil {
			return err
		}

		result.add(CasePlan{
			Name:   caseName("valid kind "+kindName(kind), source.Pointer, ""),
			Expect: ExpectAccepted, Values: partition, Source: source,
		})
	}

	return nil
}

// hasOpaqueStringConstraints reports whether arbitrary construction is unavailable.
func hasOpaqueStringConstraints(constraints StringConstraints) bool {
	return len(constraints.Patterns) > 0 || len(constraints.Formats) > 0
}

// addScalarValidPartitions adds numeric and string boundary partitions.
func (planner *CasePlanner) addScalarValidPartitions(
	result *caseSet,
	root DomainID,
	domain Domain,
	use *schemaUse,
) error {
	if err := planner.addNumberBoundaryPartitions(result, root, domain, use); err != nil {
		return err
	}

	return planner.addStringValidPartitions(result, root, domain, use)
}

// addNumberBoundaryPartitions adds accepted partitions at inclusive numeric bounds.
func (planner *CasePlanner) addNumberBoundaryPartitions(
	result *caseSet,
	root DomainID,
	domain Domain,
	use *schemaUse,
) error {
	if domain.Number.State == KindExcluded {
		return nil
	}

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
			State:   KindRestricted,
			Minimum: cloneBound(entry.bound),
			Maximum: cloneBound(entry.bound),
		}
		candidate := planner.numberDomain(exact)

		value, err := planner.Domains.IntersectDomains(root, candidate)
		if err != nil {
			return err
		}

		if value != EmptyDomainID {
			result.add(CasePlan{
				Name:   caseName("valid number "+entry.label+" boundary", use.pointer, entry.label),
				Expect: ExpectAccepted,
				Values: value,
				Source: ConstraintSource{Pointer: use.pointer, Keyword: entry.label},
			})
		}
	}

	return nil
}

// addStringValidPartitions adds length boundaries and trusted string examples.
func (planner *CasePlanner) addStringValidPartitions(
	result *caseSet,
	root DomainID,
	domain Domain,
	use *schemaUse,
) error {
	if domain.String.State == KindExcluded {
		return nil
	}

	if len(domain.String.Patterns) > 0 || len(domain.String.Formats) > 0 {
		return nil
	}

	return planner.addStringLengthPartitions(result, root, domain.String, use)
}

// addStringLengthPartitions adds accepted partitions at the configured string lengths.
func (planner *CasePlanner) addStringLengthPartitions(
	result *caseSet,
	root DomainID,
	constraints StringConstraints,
	use *schemaUse,
) error {
	lengths := []labeledIntBoundary{
		{label: "minimum", value: constraints.MinLength},
	}
	if constraints.MaxLength != nil {
		lengths = append(lengths, labeledIntBoundary{label: "maximum", value: *constraints.MaxLength})
	}

	for _, length := range lengths {
		candidate := planner.stringDomain(StringConstraints{
			State:     KindRestricted,
			MinLength: length.value,
			MaxLength: new(length.value),
		})

		value, err := planner.Domains.IntersectDomains(root, candidate)
		if err != nil {
			return err
		}

		if value != EmptyDomainID {
			result.add(CasePlan{
				Name: caseName(
					"valid string "+length.label+" length",
					use.pointer,
					length.label+"Length",
				),
				Expect: ExpectAccepted,
				Values: value,
				Source: ConstraintSource{
					Pointer: use.pointer,
					Keyword: length.label + "Length",
				},
			})
		}
	}

	return nil
}

// addArrayPartitions adds count and lifted item partitions for an array Domain.
func (planner *CasePlanner) addArrayPartitions(
	result *caseSet,
	domain Domain,
	use *schemaUse,
	active map[DomainID]bool,
) error {
	if err := planner.addContradictoryArrayFailure(result, use); err != nil {
		return err
	}

	if domain.Array.State == KindExcluded {
		return nil
	}

	planner.addArrayCountPartitions(result, domain, use)

	return planner.addArrayItemPartitions(result, domain, use, active)
}

// addContradictoryArrayFailure materializes one array that passes occurrence
// sibling rules and reaches an impossible item policy.
func (planner *CasePlanner) addContradictoryArrayFailure(result *caseSet, use *schemaUse) error {
	if use.items == nil || use.items.domain != EmptyDomainID {
		return nil
	}

	count := max(1, occurrenceMinimum(planner, use, "minItems"))
	if count > exactStructuralCollectionLimit {
		return nil
	}

	items := make([]jsonvalue.Value, count)
	for index := range items {
		items[index] = jsonvalue.Null()
	}

	value := jsonvalue.Array(items)

	passes, err := planner.valueFitsStructuralArraySiblings(value, use)
	if err != nil || !passes {
		return err
	}

	planner.addExactStructuralFailure(
		result,
		value,
		ConstraintSource{Pointer: use.pointer, Keyword: "items"},
		"invalid contradictory array items",
	)

	return nil
}

// addArrayCountPartitions adds accepted partitions at the configured array item counts.
func (planner *CasePlanner) addArrayCountPartitions(result *caseSet, domain Domain, use *schemaUse) {
	counts := []labeledIntBoundary{
		{label: "minimum", value: domain.Array.MinItems},
	}
	if domain.Array.MaxItems != nil {
		counts = append(counts, labeledIntBoundary{label: "maximum", value: *domain.Array.MaxItems})
	}

	for _, count := range counts {
		candidate := cloneDomain(domain)
		candidate.Null, candidate.Boolean = KindExcluded, KindExcluded
		candidate.Number.State, candidate.String.State, candidate.Object.State =
			KindExcluded, KindExcluded, KindExcluded
		candidate.Array.MinItems, candidate.Array.MaxItems = count.value, new(count.value)

		value := planner.Domains.FindOrAddEquivalentDomain(candidate)
		if value != EmptyDomainID {
			result.add(CasePlan{
				Name: caseName(
					"valid array "+count.label+" count",
					use.pointer,
					count.label+"Items",
				),
				Expect: ExpectAccepted,
				Values: value,
				Source: ConstraintSource{
					Pointer: use.pointer,
					Keyword: count.label + "Items",
				},
			})
		}
	}
}

// addArrayItemPartitions lifts child partitions when the array can contain an item.
func (planner *CasePlanner) addArrayItemPartitions(
	result *caseSet,
	domain Domain,
	use *schemaUse,
	active map[DomainID]bool,
) error {
	if domain.Array.Items == EmptyDomainID {
		return nil
	}

	if domain.Array.Items == AnyJSONDomainID {
		return nil
	}

	if domain.Array.MaxItems != nil && *domain.Array.MaxItems == 0 {
		return nil
	}

	if use.items == nil {
		return fmt.Errorf("plan array items at %s: schema occurrence is missing", use.pointer)
	}

	childCases, err := planner.childPartitions(use.items, active)
	if err != nil {
		return err
	}

	planner.addLiftedArrayChildPartitions(result, domain, use, childCases)

	return nil
}

// addLiftedArrayChildPartitions lifts each non-aggregate child partition into the array Domain.
func (planner *CasePlanner) addLiftedArrayChildPartitions(
	result *caseSet,
	domain Domain,
	use *schemaUse,
	childCases []CasePlan,
) {
	for _, child := range childCases {
		if isAggregateChildCase(child, domain.Array.Items) {
			continue
		}

		lifted := cloneDomain(domain)
		lifted.Null, lifted.Boolean = KindExcluded, KindExcluded
		lifted.Number.State, lifted.String.State, lifted.Object.State =
			KindExcluded, KindExcluded, KindExcluded
		lifted.Array.Items = child.Values

		lifted.Array.MinItems = max(1, lifted.Array.MinItems)
		if lifted.Array.MaxItems != nil && lifted.Array.MinItems > *lifted.Array.MaxItems {
			continue
		}

		values := planner.Domains.FindOrAddEquivalentDomain(lifted)
		result.add(CasePlan{
			Name:        caseName(expectName(child.Expect)+" array item / "+child.Name, use.pointer, "items"),
			Expect:      child.Expect,
			Values:      values,
			Source:      child.Source,
			evidenceUse: child.evidenceUse,
		})
	}
}

// addObjectPartitions adds count, declared-property, and additional-property partitions.
func (planner *CasePlanner) addObjectPartitions(
	result *caseSet,
	domain Domain,
	use *schemaUse,
	active map[DomainID]bool,
) error {
	planning := domain

	if use.objectShape != NoDomain {
		shape, ok := planner.Domains.Domain(use.objectShape)
		if !ok {
			return fmt.Errorf("plan object at %s: preserved shape Domain does not exist", use.pointer)
		}

		planning.Object = shape.Object
	}

	if err := planner.addContradictoryObjectFailures(result, planning, use); err != nil {
		return err
	}

	if domain.Object.State == KindExcluded {
		return nil
	}

	planner.addObjectCountPartitions(result, domain, use)

	if err := planner.addDeclaredPropertyPartitions(result, domain, use, active); err != nil {
		return err
	}

	return planner.addAdditionalPropertyPartitions(result, domain, use, active)
}

// addContradictoryObjectFailures plans impossible child seams before the
// effective object kind can disappear through productivity normalization.
func (planner *CasePlanner) addContradictoryObjectFailures(
	result *caseSet,
	domain Domain,
	use *schemaUse,
) error {
	if domain.Object.State == KindExcluded {
		return nil
	}

	for _, property := range domain.Object.Properties {
		child := use.property(property.Name)
		if child == nil || child.domain != EmptyDomainID {
			continue
		}

		_, _, err := planner.addContradictoryPropertyFailure(result, domain, use, property)
		if err != nil {
			return err
		}
	}

	if use.additional != nil && use.additional.domain == EmptyDomainID {
		return planner.addContradictoryAdditionalFailure(result, domain, use)
	}

	return nil
}

// addObjectCountPartitions adds accepted partitions at the configured object property counts.
func (planner *CasePlanner) addObjectCountPartitions(result *caseSet, domain Domain, use *schemaUse) {
	counts := []labeledIntBoundary{
		{label: "minimum", value: domain.Object.MinProps},
	}
	if domain.Object.MaxProps != nil {
		counts = append(counts, labeledIntBoundary{label: "maximum", value: *domain.Object.MaxProps})
	}

	for _, count := range counts {
		candidate := objectOnly(domain)
		candidate.Object.MinProps = count.value
		candidate.Object.MaxProps = new(count.value)

		values := planner.Domains.FindOrAddEquivalentDomain(candidate)
		if values != EmptyDomainID {
			result.add(CasePlan{
				Name: caseName(
					"valid object "+count.label+" count",
					use.pointer,
					count.label+"Properties",
				),
				Expect: ExpectAccepted,
				Values: values,
				Source: ConstraintSource{
					Pointer: use.pointer,
					Keyword: count.label + "Properties",
				},
			})
		}
	}
}

// addDeclaredPropertyPartitions adds partitions for each declared object property.
func (planner *CasePlanner) addDeclaredPropertyPartitions(
	result *caseSet,
	domain Domain,
	use *schemaUse,
	active map[DomainID]bool,
) error {
	for _, property := range domain.Object.Properties {
		if property.State == PropertyForbidden {
			if err := planner.addForbiddenPropertyFailure(result, domain, use, property); err != nil {
				return err
			}

			continue
		}

		planner.addOptionalPropertyPartitions(result, domain, use, property)

		if err := planner.addPropertyChildPartitions(result, domain, use, active, property); err != nil {
			return err
		}
	}

	return nil
}

// addForbiddenPropertyFailure adds a rejected partition that makes a forbidden property present.
func (planner *CasePlanner) addForbiddenPropertyFailure(
	result *caseSet,
	domain Domain,
	use *schemaUse,
	property NamedProperty,
) error {
	source, stop, err := planner.addContradictoryPropertyFailure(result, domain, use, property)
	if err != nil || stop {
		return err
	}

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
		result.add(CasePlan{
			Name: caseName(
				"invalid forbidden property "+property.Name,
				use.pointer,
				"additionalProperties",
			),
			Expect: ExpectRejected,
			Values: values,
			Source: source,
		})
	}

	return nil
}

// addContradictoryPropertyFailure adds an exact body for an impossible optional child.
func (planner *CasePlanner) addContradictoryPropertyFailure(
	result *caseSet,
	domain Domain,
	use *schemaUse,
	property NamedProperty,
) (ConstraintSource, bool, error) {
	source := ConstraintSource{Pointer: use.pointer, Keyword: "additionalProperties"}

	child := use.property(property.Name)
	if child == nil || child.domain != EmptyDomainID {
		return source, false, nil
	}

	source.Keyword = "properties"

	value, constructible, err := planner.contradictoryObjectWitness(domain.Object, property.Name, false)
	if err != nil || !constructible {
		return source, false, err
	}

	passes, err := planner.valueFitsRelaxedObject(value, domain, property.Name, false)
	if err != nil || !passes {
		return source, !passes, err
	}

	planner.addExactStructuralFailure(
		result, value, source, "invalid contradictory optional property "+property.Name,
	)

	return source, false, nil
}

// addOptionalPropertyPartitions adds present and absent partitions for an optional property.
func (planner *CasePlanner) addOptionalPropertyPartitions(
	result *caseSet,
	domain Domain,
	use *schemaUse,
	property NamedProperty,
) {
	if property.Required {
		return
	}

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
					use.pointer,
					"properties",
				),
				Expect: ExpectAccepted,
				Values: values,
				Source: ConstraintSource{
					Pointer: use.pointer,
					Keyword: "properties",
				},
			})
		}
	}
}

// addPropertyChildPartitions lifts the property value's child partitions into the object Domain.
func (planner *CasePlanner) addPropertyChildPartitions(
	result *caseSet,
	domain Domain,
	use *schemaUse,
	active map[DomainID]bool,
	property NamedProperty,
) error {
	if property.Values == AnyJSONDomainID || property.Values == EmptyDomainID {
		return nil
	}

	childCases, err := planner.propertyChildPartitions(use, property.Name, active)
	if err != nil {
		return err
	}

	for _, child := range childCases {
		if isAggregateChildCase(child, property.Values) {
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
		result.add(CasePlan{
			Name: caseName(
				expectName(expect)+" property "+property.Name+" / "+child.Name,
				use.pointer,
				"properties",
			),
			Expect:      expect,
			Values:      values,
			Source:      child.Source,
			evidenceUse: child.evidenceUse,
		})
	}

	return nil
}

// isAggregateChildCase identifies the synthetic child aggregate, not evidence.
func isAggregateChildCase(child CasePlan, values DomainID) bool {
	return child.Expect == ExpectAccepted && child.Values == values && child.Source.Keyword == ""
}

// propertyChildPartitions plans one exact declared-property occurrence.
func (planner *CasePlanner) propertyChildPartitions(
	use *schemaUse,
	name string,
	active map[DomainID]bool,
) ([]CasePlan, error) {
	childUse := use.property(name)
	if childUse == nil {
		childUse = use.additional
	}

	if childUse == nil {
		return nil, fmt.Errorf("plan property policy %q at %s: schema occurrence is missing", name, use.pointer)
	}

	return planner.childPartitions(childUse, active)
}

// addAdditionalPropertyPartitions adds partitions for the object's additional-property policy.
func (planner *CasePlanner) addAdditionalPropertyPartitions(
	result *caseSet,
	domain Domain,
	use *schemaUse,
	active map[DomainID]bool,
) error {
	if domain.Object.Additional.Values == AnyJSONDomainID {
		lifted := objectOnly(domain)
		lifted.Object.Properties = append(lifted.Object.Properties, NamedProperty{
			Name:     unusedPropertyName(domain.Object),
			Required: true,
			State:    PropertyAllowed,
			Values:   AnyJSONDomainID,
		})

		values := planner.Domains.FindOrAddEquivalentDomain(lifted)
		if values != EmptyDomainID {
			result.add(CasePlan{
				Name:   caseName("valid additional property", use.pointer, "additionalProperties"),
				Expect: ExpectAccepted,
				Values: values,
				Source: ConstraintSource{
					Pointer: use.pointer,
					Keyword: "additionalProperties",
				},
			})
		}

		return nil
	}

	if domain.Object.Additional.Values == EmptyDomainID {
		return planner.addContradictoryAdditionalFailure(result, domain, use)
	}

	if use.additional == nil {
		return fmt.Errorf("plan additional properties at %s: schema occurrence is missing", use.pointer)
	}

	childCases, err := planner.childPartitions(use.additional, active)
	if err != nil {
		return err
	}

	name := unusedPropertyName(domain.Object)

	for _, child := range childCases {
		lifted := objectOnly(domain)
		lifted.Object.Properties = append(lifted.Object.Properties, NamedProperty{
			Name:     name,
			Required: true,
			State:    PropertyAllowed,
			Values:   child.Values,
		})

		values := planner.Domains.FindOrAddEquivalentDomain(lifted)
		if values == EmptyDomainID {
			continue
		}

		result.add(CasePlan{
			Name: caseName(
				expectName(child.Expect)+" additional property / "+child.Name,
				use.pointer,
				"additionalProperties",
			),
			Expect:      child.Expect,
			Values:      values,
			Source:      child.Source,
			evidenceUse: child.evidenceUse,
		})
	}

	return nil
}

// addContradictoryAdditionalFailure adds a reachable exact additional value.
func (planner *CasePlanner) addContradictoryAdditionalFailure(
	result *caseSet,
	domain Domain,
	use *schemaUse,
) error {
	name := unusedPropertyName(domain.Object)

	if use.additional == nil ||
		!occurrenceAllowsPositiveCount(planner, use, "maxProperties") {
		return nil
	}

	value, constructible, err := planner.contradictoryObjectWitness(domain.Object, name, true)
	if err != nil || !constructible {
		return err
	}

	passes, err := planner.valueFitsRelaxedObject(value, domain, name, true)
	if err != nil || !passes {
		return err
	}

	planner.addExactStructuralFailure(
		result,
		value,
		ConstraintSource{Pointer: use.pointer, Keyword: "additionalProperties"},
		"invalid contradictory additional property",
	)

	return nil
}

// contradictoryObjectWitness constructs the smallest object that satisfies
// every modeled sibling shape rule while making one contradictory child present.
func (planner *CasePlanner) contradictoryObjectWitness(
	object ObjectConstraints,
	target string,
	targetAdditional bool,
) (jsonvalue.Value, bool, error) {
	if !objectWitnessCardinalityIsSafe(object) {
		return jsonvalue.Value{}, false, nil
	}

	properties := append([]NamedProperty(nil), object.Properties...)
	sort.Slice(properties, func(left int, right int) bool {
		return properties[left].Name < properties[right].Name
	})

	members, constructible, err := planner.buildObjectWitnessMembers(
		object,
		properties,
		target,
		targetAdditional,
	)
	if err != nil || !constructible {
		return jsonvalue.Value{}, false, err
	}

	if !objectWitnessFitsMaximum(object, members) {
		return jsonvalue.Value{}, false, nil
	}

	value, err := jsonvalue.Object(members)
	if err != nil {
		return jsonvalue.Value{}, false, err
	}

	return value, true, nil
}

// objectWitnessCardinalityIsSafe rejects impossible or oversized materialization.
func objectWitnessCardinalityIsSafe(object ObjectConstraints) bool {
	if object.MinProps > exactStructuralCollectionLimit {
		return false
	}

	return object.MaxProps == nil || *object.MaxProps >= 1
}

// objectWitnessFitsMaximum checks the completed property count.
func objectWitnessFitsMaximum(object ObjectConstraints, members []jsonvalue.Member) bool {
	return object.MaxProps == nil || len(members) <= *object.MaxProps
}

// buildObjectWitnessMembers fills required, optional, and additional siblings.
func (planner *CasePlanner) buildObjectWitnessMembers(
	object ObjectConstraints,
	properties []NamedProperty,
	target string,
	targetAdditional bool,
) ([]jsonvalue.Member, bool, error) {
	members := []jsonvalue.Member{{Name: target, Value: jsonvalue.Null()}}

	members, constructible, err := planner.appendRequiredWitnessMembers(members, properties, target)
	if err != nil || !constructible || len(members) > exactStructuralCollectionLimit {
		return nil, false, err
	}

	members, err = planner.appendOptionalWitnessMembers(members, properties, target, object.MinProps)
	if err != nil {
		return nil, false, err
	}

	return planner.fillObjectWitnessMinimum(members, object, targetAdditional)
}

// appendRequiredWitnessMembers adds every required sibling with one exact value.
func (planner *CasePlanner) appendRequiredWitnessMembers(
	members []jsonvalue.Member,
	properties []NamedProperty,
	target string,
) ([]jsonvalue.Member, bool, error) {
	for _, property := range properties {
		if property.Name == target || !property.Required {
			continue
		}

		value, constructible, err := planner.firstContextValue(property.Values)
		if err != nil {
			return nil, false, err
		}

		if !constructible {
			return nil, false, nil
		}

		members = append(members, jsonvalue.Member{Name: property.Name, Value: value})
	}

	return members, true, nil
}

// appendOptionalWitnessMembers fills the minimum from declared optional siblings.
func (planner *CasePlanner) appendOptionalWitnessMembers(
	members []jsonvalue.Member,
	properties []NamedProperty,
	target string,
	minimum int,
) ([]jsonvalue.Member, error) {
	for _, property := range properties {
		if len(members) >= minimum {
			break
		}

		if property.Name == target || property.Required || property.State == PropertyForbidden {
			continue
		}

		value, constructible, err := planner.firstContextValue(property.Values)
		if err != nil {
			return nil, err
		}

		if constructible {
			members = append(members, jsonvalue.Member{Name: property.Name, Value: value})
		}
	}

	return members, nil
}

// fillObjectWitnessMinimum adds additional names until the minimum is met.
func (planner *CasePlanner) fillObjectWitnessMinimum(
	members []jsonvalue.Member,
	object ObjectConstraints,
	targetAdditional bool,
) ([]jsonvalue.Member, bool, error) {
	for len(members) < object.MinProps {
		if len(members) >= exactStructuralCollectionLimit {
			return nil, false, nil
		}

		value, constructible, err := planner.additionalWitnessValue(object, targetAdditional)
		if err != nil || !constructible {
			return nil, false, err
		}

		members = append(members, jsonvalue.Member{
			Name: unusedMemberName(object.Properties, members), Value: value,
		})
	}

	return members, true, nil
}

// additionalWitnessValue selects the target null or one valid sibling value.
func (planner *CasePlanner) additionalWitnessValue(
	object ObjectConstraints,
	targetAdditional bool,
) (jsonvalue.Value, bool, error) {
	if targetAdditional {
		return jsonvalue.Null(), true, nil
	}

	return planner.firstContextValue(object.Additional.Values)
}

// occurrenceMinimum returns the strongest source-local minimum rule.
func occurrenceMinimum(planner *CasePlanner, use *schemaUse, keyword string) int {
	minimum := 0

	for _, source := range use.constraints {
		if source.Keyword != keyword {
			continue
		}

		sourceUse := use.find(source.Pointer)
		if sourceUse == nil {
			continue
		}

		atomic, ok := sourceUse.atomic[keyword]
		if !ok {
			continue
		}

		domain, ok := planner.Domains.Domain(atomic)
		if ok && keyword == "minItems" {
			minimum = max(minimum, domain.Array.MinItems)
		}
	}

	return minimum
}

// valueFitsStructuralArraySiblings proves an exact array against every modeled
// parent constraint while excluding only the contradictory item subtree.
func (planner *CasePlanner) valueFitsStructuralArraySiblings(
	value jsonvalue.Value,
	use *schemaUse,
) (bool, error) {
	context := AnyJSONDomainID

	for _, constraint := range planner.Constraints {
		if use.find(constraint.Source.Pointer) == nil ||
			constraint.Source.Pointer == use.items.pointer ||
			strings.HasPrefix(constraint.Source.Pointer, use.items.pointer+"/") {
			continue
		}

		intersection, err := planner.Domains.IntersectDomains(context, constraint.Pass)
		if err != nil {
			return false, err
		}

		context = intersection
	}

	domain, ok := planner.Domains.Domain(context)
	if !ok {
		return false, fmt.Errorf("array sibling Domain %d does not exist", context)
	}

	domain.Array.Items = AnyJSONDomainID

	return (&Compiler{Domains: planner.Domains}).valueFitsDomain(value, domain)
}

// valueFitsRelaxedObject proves an exact object after relaxing only the child
// seam intentionally made contradictory by the schema occurrence.
func (planner *CasePlanner) valueFitsRelaxedObject(
	value jsonvalue.Value,
	domain Domain,
	target string,
	targetAdditional bool,
) (bool, error) {
	relaxed := cloneDomain(domain)
	if targetAdditional {
		relaxed.Object.Additional.Values = AnyJSONDomainID
	} else {
		for index := range relaxed.Object.Properties {
			if relaxed.Object.Properties[index].Name == target {
				relaxed.Object.Properties[index].State = PropertyAllowed
				relaxed.Object.Properties[index].Values = AnyJSONDomainID
			}
		}
	}

	return (&Compiler{Domains: planner.Domains}).valueFitsDomain(value, relaxed)
}

// firstContextValue selects one exact modeled value for a sibling child.
func (planner *CasePlanner) firstContextValue(id DomainID) (jsonvalue.Value, bool, error) {
	values, err := planner.contextChildValues(id, 1, make(map[DomainID]bool))
	if err != nil || len(values) == 0 {
		return jsonvalue.Value{}, false, err
	}

	return values[0], true, nil
}

// unusedMemberName returns a name absent from declared and already selected members.
func unusedMemberName(properties []NamedProperty, members []jsonvalue.Member) string {
	used := make(map[string]struct{}, len(properties)+len(members))
	for _, property := range properties {
		used[property.Name] = struct{}{}
	}

	for _, member := range members {
		used[member.Name] = struct{}{}
	}

	for suffix := 0; ; suffix++ {
		name := "additional"
		if suffix > 0 {
			name = fmt.Sprintf("additional%d", suffix)
		}

		if _, exists := used[name]; !exists {
			return name
		}
	}
}

// addExactStructuralFailure adds one deterministic whole-occurrence rejected body.
func (planner *CasePlanner) addExactStructuralFailure(
	result *caseSet,
	value jsonvalue.Value,
	source ConstraintSource,
	name string,
) {
	values := planner.Domains.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{value}))
	result.add(CasePlan{
		Name:   caseName(name, source.Pointer, source.Keyword),
		Expect: ExpectRejected,
		Values: values,
		Source: source,
	})
}

// occurrenceAllowsPositiveCount rejects fabricated structural failures when a
// source-local maximum explicitly prevents a non-empty container.
func occurrenceAllowsPositiveCount(planner *CasePlanner, use *schemaUse, keyword string) bool {
	for _, source := range use.constraints {
		if source.Keyword != keyword {
			continue
		}

		sourceUse := use.find(source.Pointer)
		if sourceUse == nil {
			continue
		}

		atomic := sourceUse.localDomain
		if value, ok := sourceUse.atomic[keyword]; ok {
			atomic = value
		}

		domain, ok := planner.Domains.Domain(atomic)
		if !ok {
			continue
		}

		if occurrenceMaximumIsZero(domain, keyword) {
			return false
		}
	}

	return true
}

// occurrenceMaximumIsZero reports an explicit zero container maximum.
func occurrenceMaximumIsZero(domain Domain, keyword string) bool {
	if keyword == "maxItems" {
		return domain.Array.MaxItems != nil && *domain.Array.MaxItems == 0
	}

	return keyword == "maxProperties" && domain.Object.MaxProps != nil && *domain.Object.MaxProps == 0
}

// childPartitions plans accepted and isolated rejected cases for a nested Domain.
func (planner *CasePlanner) childPartitions(
	use *schemaUse,
	active map[DomainID]bool,
) ([]CasePlan, error) {
	children := newCaseSet()
	children.add(CasePlan{
		Name:   caseName("valid aggregate", use.pointer, ""),
		Expect: ExpectAccepted,
		Values: use.domain,
		Source: ConstraintSource{Pointer: use.pointer},
	})
	planner.addExactEvidenceCases(children, use, use.examples.Invalid, ExpectRejected)

	if err := planner.addValidPartitions(children, use, active); err != nil {
		return nil, err
	}

	childPlanner := &CasePlanner{Domains: planner.Domains, rootUse: use}

	constraints, err := childPlanner.constraintPlans(use)
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

// objectOnly returns domain restricted to the object JSON kind.
func objectOnly(domain Domain) Domain {
	result := cloneDomain(domain)
	result.Null, result.Boolean = KindExcluded, KindExcluded
	result.Number.State, result.String.State, result.Array.State =
		KindExcluded, KindExcluded, KindExcluded
	result.Enum = nil

	return result
}

// expectName returns the name prefix for expect.
func expectName(expect ExpectedResult) string {
	if expect == ExpectRejected {
		return "invalid"
	}

	return "valid"
}

// unusedPropertyName returns an additional property name not declared in object.
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

// kindMaskDomain returns the unrestricted Domain limited to source's reachable JSON kinds.
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

// singleKindDomain returns the unrestricted Domain for one JSON kind.
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
		domain.Object = ObjectConstraints{
			State: KindUnrestricted,
			Additional: AdditionalProperties{
				Values: AnyJSONDomainID,
			},
		}
	}

	return domain
}

// reachableKinds returns the JSON kinds not excluded by domain.
func reachableKinds(domain Domain) []jsonvalue.Kind {
	result := make([]jsonvalue.Kind, 0)
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

// excludedKinds returns the JSON kinds excluded by domain.
func excludedKinds(domain Domain) []jsonvalue.Kind {
	result := make([]jsonvalue.Kind, 0)

	for _, kind := range [...]jsonvalue.Kind{
		jsonvalue.KindNull,
		jsonvalue.KindBoolean,
		jsonvalue.KindNumber,
		jsonvalue.KindString,
		jsonvalue.KindArray,
		jsonvalue.KindObject,
	} {
		if !kindReachable(domain, kind) {
			result = append(result, kind)
		}
	}

	return result
}

// kindReachable reports whether kind is not excluded by domain.
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

// kindName returns the display name for kind.
func kindName(kind jsonvalue.Kind) string {
	return [...]string{"null", "boolean", "number", "string", "array", "object"}[kind]
}
