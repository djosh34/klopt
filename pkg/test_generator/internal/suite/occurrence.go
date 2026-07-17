package suite

import (
	"fmt"
	"sort"
)

// meet pairs the canonical semantic intersection with the exact recursive occurrences that produced it.
func (compiler *Compiler) meet(left *schemaUse, right *schemaUse) (*schemaUse, error) {
	leftDomain, rightDomain, resultDomain, domain, err := compiler.meetDomains(left, right)
	if err != nil {
		return nil, err
	}

	examples, err := compiler.meetGenerationExamples(left.examples, leftDomain, right.examples, rightDomain)
	if err != nil {
		return nil, err
	}

	result := &schemaUse{
		pointer:     left.pointer,
		domain:      domain,
		localDomain: left.localDomain,
		arrayShape:  left.arrayShape,
		objectShape: left.objectShape,
		constraints: append(append([]ConstraintSource(nil), left.constraints...), right.constraints...),
		patterns:    append(append([]patternOccurrence(nil), left.patterns...), right.patterns...),
		examples:    examples,
		atomic:      left.atomic,
		allOf:       append(append([]*schemaUse(nil), left.allOf...), right),
		resolved:    left.resolved,
	}

	if err := compiler.meetChildren(result, left, leftDomain, right, rightDomain); err != nil {
		return nil, err
	}

	if overlap := generationExampleOverlap(result.examples); overlap != nil {
		return nil, &generationOverlapError{Example: *overlap}
	}

	if compiler.mustHaveAllXValidCases && needsTrustedValidCases(resultDomain) &&
		result.examples.ValidDeclared && len(result.examples.Valid) == 0 {
		return nil, fmt.Errorf("%w: allOf merge has no trusted valid generation case", errUnconstructible)
	}

	return result, nil
}

// needsTrustedValidCases reports occurrence languages that remain oracle-backed.
func needsTrustedValidCases(domain Domain) bool {
	return domain.Enum != nil || domain.String.State != KindExcluded && len(domain.String.Formats) > 0
}

// meetDomains intersects semantic Domains and returns their canonical values.
func (compiler *Compiler) meetDomains(
	left *schemaUse,
	right *schemaUse,
) (Domain, Domain, Domain, DomainID, error) {
	if left == nil || right == nil {
		return Domain{}, Domain{}, Domain{}, NoDomain, fmt.Errorf("meet schema occurrences: occurrence is nil")
	}

	domain, err := compiler.Domains.IntersectDomains(left.domain, right.domain)
	if err != nil {
		return Domain{}, Domain{}, Domain{}, NoDomain, err
	}

	leftDomain, leftOK := compiler.Domains.Domain(left.domain)
	rightDomain, rightOK := compiler.Domains.Domain(right.domain)

	resultDomain, resultOK := compiler.Domains.Domain(domain)
	if !leftOK || !rightOK || !resultOK {
		return Domain{}, Domain{}, Domain{}, NoDomain,
			fmt.Errorf("meet schema occurrences: compiled Domain does not exist")
	}

	return leftDomain, rightDomain, resultDomain, domain, nil
}

// meetChildren recursively pairs array, object-property, and additional-property occurrences.
func (compiler *Compiler) meetChildren(
	result *schemaUse,
	left *schemaUse,
	leftDomain Domain,
	right *schemaUse,
	rightDomain Domain,
) error {
	if err := compiler.meetArrayOccurrence(result, left, leftDomain, right, rightDomain); err != nil {
		return err
	}

	return compiler.meetObjectOccurrence(result, left, leftDomain, right, rightDomain)
}

// meetArrayOccurrence composes planning shape and exact item provenance symmetrically.
func (compiler *Compiler) meetArrayOccurrence(
	result *schemaUse,
	left *schemaUse,
	leftDomain Domain,
	right *schemaUse,
	rightDomain Domain,
) error {
	leftArray, leftPresent, err := compiler.occurrenceArrayShape(left, leftDomain)
	if err != nil {
		return err
	}

	rightArray, rightPresent, err := compiler.occurrenceArrayShape(right, rightDomain)
	if err != nil {
		return err
	}

	if !leftPresent || !rightPresent {
		if leftPresent {
			result.arrayShape = left.arrayShape
			result.items = left.items
		} else if rightPresent {
			result.arrayShape = right.arrayShape
			result.items = right.items
		}

		return nil
	}

	leftItemUse, leftItems := occurrenceArrayItemPolicy(left, leftArray)
	rightItemUse, rightItems := occurrenceArrayItemPolicy(right, rightArray)

	items, itemUse, itemPresent, err := compiler.meetArrayItems(
		leftItemUse,
		leftItems,
		rightItemUse,
		rightItems,
	)
	if err != nil {
		return err
	}

	if itemPresent {
		result.items = itemUse
	}

	planning := arrayPlanningShape(leftArray, rightArray, items, itemPresent)
	result.arrayShape = compiler.Domains.FindOrAddEquivalentDomain(arrayRuleDomain(planning))

	return nil
}

// meetArrayItems intersects item policies and composes their exact occurrences.
func (compiler *Compiler) meetArrayItems(
	leftUse *schemaUse,
	leftValues DomainID,
	rightUse *schemaUse,
	rightValues DomainID,
) (DomainID, *schemaUse, bool, error) {
	items, err := compiler.Domains.IntersectDomains(leftValues, rightValues)
	if err != nil {
		return NoDomain, nil, false, fmt.Errorf("meet array item Domains: %w", err)
	}

	use, present, err := compiler.meetChild(leftUse, leftValues, rightUse, rightValues, items)
	if err != nil {
		return NoDomain, nil, false, fmt.Errorf("meet array items: %w", err)
	}

	return items, use, present, nil
}

// arrayPlanningShape retains counts while opening only a proven contradictory item seam.
func arrayPlanningShape(
	left ArrayConstraints,
	right ArrayConstraints,
	items DomainID,
	itemPresent bool,
) ArrayConstraints {
	planningItems := items
	if items == EmptyDomainID && itemPresent {
		planningItems = AnyJSONDomainID
	}

	planning := ArrayConstraints{
		State:    KindRestricted,
		Items:    planningItems,
		MinItems: max(left.MinItems, right.MinItems),
		MaxItems: smallerInt(left.MaxItems, right.MaxItems),
	}
	if items == EmptyDomainID && !itemPresent {
		planning.MaxItems = new(0)
	}

	return planning
}

// occurrenceArrayItemPolicy restores an exact contradictory seam relaxed in the planning shape.
func occurrenceArrayItemPolicy(use *schemaUse, shape ArrayConstraints) (*schemaUse, DomainID) {
	if use.items != nil {
		return use.items, use.items.domain
	}

	return nil, shape.Items
}

// occurrenceArrayShape returns one canonical planning policy without DomainID provenance lookup.
func (compiler *Compiler) occurrenceArrayShape(use *schemaUse, semantic Domain) (ArrayConstraints, bool, error) {
	if use.arrayShape != NoDomain {
		shape, ok := compiler.Domains.Domain(use.arrayShape)
		if !ok {
			return ArrayConstraints{}, false, fmt.Errorf("array planning Domain %d does not exist", use.arrayShape)
		}

		return shape.Array, shape.Array.State != KindExcluded, nil
	}

	return semantic.Array, semantic.Array.State != KindExcluded, nil
}

// meetObjectOccurrence composes planning shape and exact property provenance symmetrically.
func (compiler *Compiler) meetObjectOccurrence(
	result *schemaUse,
	left *schemaUse,
	leftDomain Domain,
	right *schemaUse,
	rightDomain Domain,
) error {
	leftObject, leftPresent, err := compiler.occurrenceObjectShape(left, leftDomain)
	if err != nil {
		return err
	}

	rightObject, rightPresent, err := compiler.occurrenceObjectShape(right, rightDomain)
	if err != nil {
		return err
	}

	if !leftPresent || !rightPresent {
		if leftPresent {
			result.objectShape = left.objectShape
			result.properties = append([]schemaPropertyUse(nil), left.properties...)
			result.additional = left.additional
		} else if rightPresent {
			result.objectShape = right.objectShape
			result.properties = append([]schemaPropertyUse(nil), right.properties...)
			result.additional = right.additional
		}

		return nil
	}

	leftAdditionalUse, leftAdditional := occurrenceAdditionalPolicy(left, leftObject)
	rightAdditionalUse, rightAdditional := occurrenceAdditionalPolicy(right, rightObject)

	additional, additionalUse, additionalPresent, err := compiler.meetAdditionalOccurrence(
		leftAdditionalUse,
		leftAdditional,
		rightAdditionalUse,
		rightAdditional,
	)
	if err != nil {
		return err
	}

	if additionalPresent {
		result.additional = additionalUse
	}

	properties, err := compiler.meetObjectPlanningProperties(result, left, leftObject, right, rightObject)
	if err != nil {
		return err
	}

	planning := ObjectConstraints{
		State:      KindRestricted,
		Properties: properties,
		Additional: AdditionalProperties{Values: planningAdditionalPolicy(additional, additionalPresent)},
		MinProps:   max(leftObject.MinProps, rightObject.MinProps),
		MaxProps:   smallerInt(leftObject.MaxProps, rightObject.MaxProps),
	}
	result.objectShape = compiler.Domains.FindOrAddEquivalentDomain(objectRuleDomain(planning))

	return nil
}

// meetAdditionalOccurrence intersects the policy and composes exact schema provenance.
func (compiler *Compiler) meetAdditionalOccurrence(
	leftUse *schemaUse,
	leftValues DomainID,
	rightUse *schemaUse,
	rightValues DomainID,
) (DomainID, *schemaUse, bool, error) {
	additional, err := compiler.Domains.IntersectDomains(leftValues, rightValues)
	if err != nil {
		return NoDomain, nil, false, fmt.Errorf("meet additional property Domains: %w", err)
	}

	use, present, err := compiler.meetChild(leftUse, leftValues, rightUse, rightValues, additional)
	if err != nil {
		return NoDomain, nil, false, fmt.Errorf("meet additional properties: %w", err)
	}

	return additional, use, present, nil
}

// planningAdditionalPolicy opens only an Empty policy backed by a schema occurrence.
func planningAdditionalPolicy(additional DomainID, present bool) DomainID {
	if additional == EmptyDomainID && present {
		return AnyJSONDomainID
	}

	return additional
}

// occurrenceAdditionalPolicy restores an exact contradictory seam relaxed in the planning shape.
func occurrenceAdditionalPolicy(use *schemaUse, shape ObjectConstraints) (*schemaUse, DomainID) {
	if use.additional != nil {
		return use.additional, use.additional.domain
	}

	return nil, shape.Additional.Values
}

// occurrenceObjectShape returns one canonical planning policy without DomainID provenance lookup.
func (compiler *Compiler) occurrenceObjectShape(use *schemaUse, semantic Domain) (ObjectConstraints, bool, error) {
	if use.objectShape != NoDomain {
		shape, ok := compiler.Domains.Domain(use.objectShape)
		if !ok {
			return ObjectConstraints{}, false, fmt.Errorf("object planning Domain %d does not exist", use.objectShape)
		}

		return shape.Object, shape.Object.State != KindExcluded, nil
	}

	return semantic.Object, semantic.Object.State != KindExcluded, nil
}

// meetObjectPlanningProperties composes every explicit-or-additional property policy.
func (compiler *Compiler) meetObjectPlanningProperties(
	result *schemaUse,
	left *schemaUse,
	leftObject ObjectConstraints,
	right *schemaUse,
	rightObject ObjectConstraints,
) ([]NamedProperty, error) {
	leftProperties := propertyConstraintsByName(leftObject.Properties)
	rightProperties := propertyConstraintsByName(rightObject.Properties)
	names := objectPropertyNames(leftProperties, rightProperties)

	orderedNames := make([]string, 0, len(names))
	for name := range names {
		orderedNames = append(orderedNames, name)
	}

	sort.Strings(orderedNames)

	properties := make([]NamedProperty, 0, len(orderedNames))
	for _, name := range orderedNames {
		property, propertyUse, err := compiler.meetObjectPlanningProperty(
			left,
			leftProperties,
			leftObject.Additional,
			right,
			rightProperties,
			rightObject.Additional,
			name,
		)
		if err != nil {
			return nil, err
		}

		properties = append(properties, property)
		if propertyUse != nil {
			result.properties = append(result.properties, schemaPropertyUse{name: name, use: propertyUse})
		}
	}

	return properties, nil
}

// meetObjectPlanningProperty composes one named policy and its exact child seam.
func (compiler *Compiler) meetObjectPlanningProperty(
	left *schemaUse,
	leftProperties map[string]NamedProperty,
	leftAdditional AdditionalProperties,
	right *schemaUse,
	rightProperties map[string]NamedProperty,
	rightAdditional AdditionalProperties,
	name string,
) (NamedProperty, *schemaUse, error) {
	leftUse, leftValues := occurrencePropertyPolicy(left, leftProperties, leftAdditional, name)
	rightUse, rightValues := occurrencePropertyPolicy(right, rightProperties, rightAdditional, name)

	values, err := compiler.Domains.IntersectDomains(leftValues, rightValues)
	if err != nil {
		return NamedProperty{}, nil, fmt.Errorf("meet property %q Domains: %w", name, err)
	}

	use, present, err := compiler.meetChild(leftUse, leftValues, rightUse, rightValues, values)
	if err != nil {
		return NamedProperty{}, nil, fmt.Errorf("meet property %q: %w", name, err)
	}

	required := leftProperties[name].Required || rightProperties[name].Required

	if values == EmptyDomainID {
		if present {
			return NamedProperty{
				Name: name, Required: required, State: PropertyAllowed, Values: AnyJSONDomainID,
			}, use, nil
		}

		return NamedProperty{
			Name: name, Required: required, State: PropertyForbidden, Values: EmptyDomainID,
		}, nil, nil
	}

	property := NamedProperty{Name: name, Required: required, State: PropertyAllowed, Values: values}
	if present {
		return property, use, nil
	}

	return property, nil, nil
}

// meetChild combines child provenance when both policies are schema-valued.
func (compiler *Compiler) meetChild(
	left *schemaUse,
	leftDomain DomainID,
	right *schemaUse,
	rightDomain DomainID,
	resultDomain DomainID,
) (*schemaUse, bool, error) {
	if resultDomain == EmptyDomainID {
		return compiler.meetEmptyChild(left, leftDomain, right, rightDomain)
	}

	if resultDomain == NoDomain || resultDomain == AnyJSONDomainID {
		return nil, false, nil
	}

	if left == nil {
		return existingChild(right, resultDomain, "left", leftDomain)
	}

	if right == nil {
		return existingChild(left, resultDomain, "right", rightDomain)
	}

	result, err := compiler.meet(left, right)
	if err != nil {
		return nil, false, err
	}

	if result.domain != resultDomain {
		return nil, false, fmt.Errorf(
			"%w: metadata Domain %d differs from semantic Domain %d",
			errUnconstructible,
			result.domain,
			resultDomain,
		)
	}

	result.preserveChildPlanningParity(left, right)

	return result, true, nil
}

// meetEmptyChild retains the contradictory occurrences that made a child policy impossible.
func (compiler *Compiler) meetEmptyChild(
	left *schemaUse,
	leftDomain DomainID,
	right *schemaUse,
	rightDomain DomainID,
) (*schemaUse, bool, error) {
	if left == nil && right == nil {
		return nil, false, nil
	}

	if left == nil {
		if leftDomain == AnyJSONDomainID {
			return existingChild(right, EmptyDomainID, "left", leftDomain)
		}

		return nil, false, nil
	}

	if right == nil {
		if rightDomain == AnyJSONDomainID {
			return existingChild(left, EmptyDomainID, "right", rightDomain)
		}

		return nil, false, nil
	}

	result, err := compiler.meet(left, right)
	if err != nil {
		return nil, false, err
	}

	if result.domain != EmptyDomainID {
		return nil, false, fmt.Errorf(
			"%w: metadata Domain %d differs from semantic Domain %d",
			errUnconstructible,
			result.domain,
			EmptyDomainID,
		)
	}

	result.preserveChildPlanningParity(left, right)

	return result, true, nil
}

// preserveChildPlanningParity keeps current child obligations while all contributors remain in allOf provenance.
func (use *schemaUse) preserveChildPlanningParity(left *schemaUse, right *schemaUse) {
	var source *schemaUse
	if left.domain == use.domain {
		source = left
	} else if right.domain == use.domain {
		source = right
	}

	if source == nil {
		return
	}

	use.pointer = source.pointer
	use.localDomain = source.localDomain
	use.atomic = source.atomic
	use.resolved = source.resolved
}

// generationOverlapError preserves the invalid declaration source across a meet.
type generationOverlapError struct {
	Example GenerationExample
}

// Error reports the contradictory oracle declaration.
func (overlap *generationOverlapError) Error() string {
	return "trusted value is declared both valid and invalid"
}

// existingChild returns the schema-valued side of an intersection with an implicit policy.
func existingChild(
	use *schemaUse,
	resultDomain DomainID,
	missingSide string,
	missingDomain DomainID,
) (*schemaUse, bool, error) {
	if use != nil && use.domain == resultDomain {
		return use, true, nil
	}

	return nil, false, fmt.Errorf("%s Domain %d has no schema occurrence", missingSide, missingDomain)
}

// occurrencePropertyPolicy returns the explicit or additional policy for one property name.
func occurrencePropertyPolicy(
	use *schemaUse,
	properties map[string]NamedProperty,
	additional AdditionalProperties,
	name string,
) (*schemaUse, DomainID) {
	property, ok := properties[name]
	if !ok {
		return use.additional, additional.Values
	}

	if property.State == PropertyForbidden {
		return use.property(name), EmptyDomainID
	}

	propertyUse := use.property(name)
	if propertyUse == nil {
		if use.additional != nil {
			return use.additional, use.additional.domain
		}

		return nil, additional.Values
	}

	return propertyUse, propertyUse.domain
}

// property returns the exact declared-property occurrence.
func (use *schemaUse) property(name string) *schemaUse {
	if use == nil {
		return nil
	}

	for _, property := range use.properties {
		if property.name == name {
			return property.use
		}
	}

	return nil
}

// find returns the exact occurrence at pointer without using semantic Domain identity.
func (use *schemaUse) find(pointer string) *schemaUse {
	if use == nil {
		return nil
	}

	if use.pointer == pointer {
		return use
	}

	if found := use.resolved.find(pointer); found != nil {
		return found
	}

	for _, member := range use.allOf {
		if found := member.find(pointer); found != nil {
			return found
		}
	}

	if found := use.items.find(pointer); found != nil {
		return found
	}

	for _, property := range use.properties {
		if found := property.use.find(pointer); found != nil {
			return found
		}
	}

	return use.additional.find(pointer)
}
