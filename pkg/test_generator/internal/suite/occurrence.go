package suite

import "fmt"

// meet pairs the canonical semantic intersection with the exact recursive occurrences that produced it.
func (compiler *Compiler) meet(left *schemaUse, right *schemaUse) (*schemaUse, error) {
	leftDomain, rightDomain, resultDomain, domain, err := compiler.meetDomains(left, right)
	if err != nil {
		return nil, err
	}

	result := &schemaUse{
		pointer:     left.pointer,
		domain:      domain,
		localDomain: left.localDomain,
		constraints: append(append([]ConstraintSource(nil), left.constraints...), right.constraints...),
		examples:    mergeGenerationExamples(left.examples, right.examples),
		atomic:      left.atomic,
		allOf:       append(append([]*schemaUse(nil), left.allOf...), right),
		resolved:    left.resolved,
	}

	if err := compiler.meetChildren(result, left, leftDomain, right, rightDomain, resultDomain); err != nil {
		return nil, err
	}

	return result, nil
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
	resultDomain Domain,
) error {
	items, present, err := compiler.meetChild(
		left.items,
		leftDomain.Array.Items,
		right.items,
		rightDomain.Array.Items,
		resultDomain.Array.Items,
	)
	if err != nil {
		return fmt.Errorf("meet array items: %w", err)
	}

	if present {
		result.items = items
	}

	additional, present, err := compiler.meetChild(
		left.additional,
		leftDomain.Object.Additional.Values,
		right.additional,
		rightDomain.Object.Additional.Values,
		resultDomain.Object.Additional.Values,
	)
	if err != nil {
		return fmt.Errorf("meet additional properties: %w", err)
	}

	if present {
		result.additional = additional
	}

	return compiler.meetProperties(result, left, leftDomain.Object, right, rightDomain.Object, resultDomain.Object)
}

// meetProperties recursively pairs every schema-valued property policy.
func (compiler *Compiler) meetProperties(
	result *schemaUse,
	left *schemaUse,
	leftObject ObjectConstraints,
	right *schemaUse,
	rightObject ObjectConstraints,
	resultObject ObjectConstraints,
) error {
	leftProperties := propertyConstraintsByName(leftObject.Properties)
	rightProperties := propertyConstraintsByName(rightObject.Properties)

	for _, property := range resultObject.Properties {
		leftUse, leftValues := occurrencePropertyPolicy(
			left,
			leftProperties,
			leftObject.Additional,
			property.Name,
		)
		rightUse, rightValues := occurrencePropertyPolicy(
			right,
			rightProperties,
			rightObject.Additional,
			property.Name,
		)

		use, present, childErr := compiler.meetChild(leftUse, leftValues, rightUse, rightValues, property.Values)
		if childErr != nil {
			return fmt.Errorf("meet property %q: %w", property.Name, childErr)
		}

		if present {
			result.properties = append(result.properties, schemaPropertyUse{name: property.Name, use: use})
		}
	}

	return nil
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
		return nil, false, fmt.Errorf("metadata Domain %d differs from semantic Domain %d", result.domain, resultDomain)
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
		return nil, false, fmt.Errorf("metadata Domain %d differs from semantic Domain %d", result.domain, EmptyDomainID)
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
		use.examples = GenerationExamples{}

		return
	}

	use.pointer = source.pointer
	use.localDomain = source.localDomain
	use.constraints = append([]ConstraintSource(nil), source.constraints...)
	use.examples = GenerationExamples{
		Valid:   cloneJSONValues(source.examples.Valid),
		Invalid: cloneJSONValues(source.examples.Invalid),
	}
	use.atomic = source.atomic
	use.resolved = source.resolved
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
		return use.additional, additional.Values
	}

	return propertyUse, property.Values
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

// walk visits each reachable occurrence once.
func (use *schemaUse) walk(visit func(*schemaUse), seen map[*schemaUse]struct{}) {
	if use == nil {
		return
	}

	if _, ok := seen[use]; ok {
		return
	}

	seen[use] = struct{}{}
	visit(use)
	use.resolved.walk(visit, seen)

	for _, member := range use.allOf {
		member.walk(visit, seen)
	}

	use.items.walk(visit, seen)

	for _, property := range use.properties {
		property.use.walk(visit, seen)
	}

	use.additional.walk(visit, seen)
}
