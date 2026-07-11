package suite

import (
	"fmt"
	"strings"

	//nolint:depguard // Internal suite partition planning intentionally depends on internal/jsonvalue.
	"decode_and_validate_generator/pkg/test_generator/internal/jsonvalue"
)

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
		kindDomain := planner.Domains.FindOrAddEquivalentDomain(singleKindDomain(kind))

		partition, err := planner.Domains.IntersectDomains(id, kindDomain)
		if err != nil {
			return err
		}

		result.add(CasePlan{
			Name:   caseName("valid kind "+kindName(kind), use.Pointer, ""),
			Expect: ExpectAccepted,
			Values: partition,
			Source: source,
		})
	}

	if domain.Enum != nil {
		for index, value := range domain.Enum.Values {
			member := planner.Domains.FindOrAddEquivalentDomain(finiteDomain([]jsonvalue.Value{value}))
			result.add(CasePlan{
				Name:   caseName(fmt.Sprintf("valid enum member %d", index+1), use.Pointer, "enum"),
				Expect: ExpectAccepted,
				Values: member,
				Source: ConstraintSource{
					Pointer: use.Pointer,
					Keyword: "enum",
				},
			})
		}

		return nil
	}

	if err := planner.addScalarValidPartitions(result, id, domain, use); err != nil {
		return err
	}

	if err := planner.addArrayPartitions(result, domain, use, uses, active); err != nil {
		return err
	}

	return planner.addObjectPartitions(result, domain, use, uses, active)
}

// addScalarValidPartitions adds numeric and string boundary partitions.
func (planner *CasePlanner) addScalarValidPartitions(
	result *caseSet,
	root DomainID,
	domain Domain,
	use SchemaUse,
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
	use SchemaUse,
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
				Name:   caseName("valid number "+entry.label+" boundary", use.Pointer, entry.label),
				Expect: ExpectAccepted,
				Values: value,
				Source: ConstraintSource{Pointer: use.Pointer, Keyword: entry.label},
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
	use SchemaUse,
) error {
	if domain.String.State == KindExcluded {
		return nil
	}

	if err := planner.addStringLengthPartitions(result, root, domain.String, use); err != nil {
		return err
	}

	return planner.addTrustedStringPartitions(result, root, use)
}

// addStringLengthPartitions adds accepted partitions at the configured string lengths.
func (planner *CasePlanner) addStringLengthPartitions(
	result *caseSet,
	root DomainID,
	constraints StringConstraints,
	use SchemaUse,
) error {
	lengths := []struct {
		label  string
		length int
	}{
		{label: "minimum", length: constraints.MinLength},
	}
	if constraints.MaxLength != nil {
		lengths = append(lengths, struct {
			label  string
			length int
		}{label: "maximum", length: *constraints.MaxLength})
	}

	for _, length := range lengths {
		candidate := planner.stringDomain(StringConstraints{
			State:     KindRestricted,
			MinLength: length.length,
			MaxLength: new(length.length),
		})

		value, err := planner.Domains.IntersectDomains(root, candidate)
		if err != nil {
			return err
		}

		if value != EmptyDomainID {
			result.add(CasePlan{
				Name: caseName(
					"valid string "+length.label+" length",
					use.Pointer,
					length.label+"Length",
				),
				Expect: ExpectAccepted,
				Values: value,
				Source: ConstraintSource{
					Pointer: use.Pointer,
					Keyword: length.label + "Length",
				},
			})
		}
	}

	return nil
}

// addTrustedStringPartitions adds accepted partitions for valid string examples.
func (planner *CasePlanner) addTrustedStringPartitions(result *caseSet, root DomainID, use SchemaUse) error {
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
				Source: ConstraintSource{
					Pointer: use.Pointer,
					Keyword: "pattern/format",
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
	use SchemaUse,
	uses []SchemaUse,
	active map[DomainID]bool,
) error {
	if domain.Array.State == KindExcluded {
		return nil
	}

	planner.addArrayCountPartitions(result, domain, use)

	return planner.addArrayItemPartitions(result, domain, use, uses, active)
}

// addArrayCountPartitions adds accepted partitions at the configured array item counts.
func (planner *CasePlanner) addArrayCountPartitions(result *caseSet, domain Domain, use SchemaUse) {
	counts := []struct {
		label string
		value int
	}{
		{label: "minimum", value: domain.Array.MinItems},
	}
	if domain.Array.MaxItems != nil {
		counts = append(counts, struct {
			label string
			value int
		}{label: "maximum", value: *domain.Array.MaxItems})
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
					use.Pointer,
					count.label+"Items",
				),
				Expect: ExpectAccepted,
				Values: value,
				Source: ConstraintSource{
					Pointer: use.Pointer,
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
	use SchemaUse,
	uses []SchemaUse,
	active map[DomainID]bool,
) error {
	if domain.Array.Items == AnyJSONDomainID || domain.Array.Items == EmptyDomainID {
		return nil
	}

	if domain.Array.MaxItems != nil && *domain.Array.MaxItems == 0 {
		return nil
	}

	childUse := childSchemaUse(uses, use.Pointer+"/items", domain.Array.Items)

	childCases, err := planner.childPartitions(domain.Array.Items, childUse, uses, active)
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
	use SchemaUse,
	childCases []CasePlan,
) {
	for _, child := range childCases {
		if child.Values == domain.Array.Items && child.Expect == ExpectAccepted {
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
			Name:   caseName(expectName(child.Expect)+" array item / "+child.Name, use.Pointer, "items"),
			Expect: child.Expect,
			Values: values,
			Source: child.Source,
		})
	}
}

// addObjectPartitions adds count, declared-property, and additional-property partitions.
func (planner *CasePlanner) addObjectPartitions(
	result *caseSet,
	domain Domain,
	use SchemaUse,
	uses []SchemaUse,
	active map[DomainID]bool,
) error {
	if domain.Object.State == KindExcluded {
		return nil
	}

	planner.addObjectCountPartitions(result, domain, use)

	if err := planner.addDeclaredPropertyPartitions(result, domain, use, uses, active); err != nil {
		return err
	}

	return planner.addAdditionalPropertyPartitions(result, domain, use, uses, active)
}

// addObjectCountPartitions adds accepted partitions at the configured object property counts.
func (planner *CasePlanner) addObjectCountPartitions(result *caseSet, domain Domain, use SchemaUse) {
	counts := []struct {
		label string
		value int
	}{
		{label: "minimum", value: domain.Object.MinProps},
	}
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
					"valid object "+count.label+" count",
					use.Pointer,
					count.label+"Properties",
				),
				Expect: ExpectAccepted,
				Values: values,
				Source: ConstraintSource{
					Pointer: use.Pointer,
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
	use SchemaUse,
	uses []SchemaUse,
	active map[DomainID]bool,
) error {
	for _, property := range domain.Object.Properties {
		if property.State == PropertyForbidden {
			planner.addForbiddenPropertyFailure(result, domain, use, property)

			continue
		}

		planner.addOptionalPropertyPartitions(result, domain, use, property)

		if err := planner.addPropertyChildPartitions(result, domain, use, uses, active, property); err != nil {
			return err
		}
	}

	return nil
}

// addForbiddenPropertyFailure adds a rejected partition that makes a forbidden property present.
func (planner *CasePlanner) addForbiddenPropertyFailure(
	result *caseSet,
	domain Domain,
	use SchemaUse,
	property NamedProperty,
) {
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
				use.Pointer,
				"additionalProperties",
			),
			Expect: ExpectRejected,
			Values: values,
			Source: ConstraintSource{
				Pointer: use.Pointer,
				Keyword: "additionalProperties",
			},
		})
	}
}

// addOptionalPropertyPartitions adds present and absent partitions for an optional property.
func (planner *CasePlanner) addOptionalPropertyPartitions(
	result *caseSet,
	domain Domain,
	use SchemaUse,
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
					use.Pointer,
					"properties",
				),
				Expect: ExpectAccepted,
				Values: values,
				Source: ConstraintSource{
					Pointer: use.Pointer,
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
	use SchemaUse,
	uses []SchemaUse,
	active map[DomainID]bool,
	property NamedProperty,
) error {
	if property.Values == AnyJSONDomainID || property.Values == EmptyDomainID {
		return nil
	}

	childUse := childSchemaUse(
		uses,
		use.Pointer+"/properties/"+escapePointerToken(property.Name),
		property.Values,
	)

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
		result.add(CasePlan{
			Name: caseName(
				expectName(expect)+" property "+property.Name+" / "+child.Name,
				use.Pointer,
				"properties",
			),
			Expect: expect,
			Values: values,
			Source: child.Source,
		})
	}

	return nil
}

// addAdditionalPropertyPartitions adds partitions for the object's additional-property policy.
func (planner *CasePlanner) addAdditionalPropertyPartitions(
	result *caseSet,
	domain Domain,
	use SchemaUse,
	uses []SchemaUse,
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
				Name:   caseName("valid additional property", use.Pointer, "additionalProperties"),
				Expect: ExpectAccepted,
				Values: values,
				Source: ConstraintSource{
					Pointer: use.Pointer,
					Keyword: "additionalProperties",
				},
			})
		}

		return nil
	}

	if domain.Object.Additional.Values == EmptyDomainID {
		return nil
	}

	childUse := childSchemaUse(uses, use.Pointer+"/additionalProperties", domain.Object.Additional.Values)

	childCases, err := planner.childPartitions(domain.Object.Additional.Values, childUse, uses, active)
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
				use.Pointer,
				"additionalProperties",
			),
			Expect: child.Expect,
			Values: values,
			Source: child.Source,
		})
	}

	return nil
}

// childPartitions plans accepted and isolated rejected cases for a nested Domain.
func (planner *CasePlanner) childPartitions(
	id DomainID,
	use SchemaUse,
	uses []SchemaUse,
	active map[DomainID]bool,
) ([]CasePlan, error) {
	children := newCaseSet()
	children.add(CasePlan{
		Name:   caseName("valid aggregate", use.Pointer, ""),
		Expect: ExpectAccepted,
		Values: id,
		Source: ConstraintSource{Pointer: use.Pointer},
	})

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

// childSchemaUse returns the exact child occurrence or an occurrence with the requested Domain.
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

// escapePointerToken escapes token for use in a JSON Pointer.
func escapePointerToken(token string) string {
	return strings.ReplaceAll(strings.ReplaceAll(token, "~", "~0"), "/", "~1")
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
