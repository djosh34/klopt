package suite

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"decode_and_validate_generator/pkg/internal/jsonvalue"
)

// decodeEnumMembers validates and decodes the enum array.
func decodeEnumMembers(raw json.RawMessage) ([]json.RawMessage, error) {
	var members []json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return nil, fmt.Errorf("enum must be an array: %w", err)
	}

	if len(members) == 0 {
		return nil, errors.New("enum must contain at least one value")
	}

	return members, nil
}

// finiteDomain returns a Domain containing exactly the supplied enum values.
func finiteDomain(values []jsonvalue.Value) Domain {
	if len(values) == 0 {
		return emptyDomain()
	}

	domain := emptyDomain()
	domain.Status = DomainProductive

	domain.Enum = &EnumSet{Values: values}
	for _, value := range values {
		switch value.Kind {
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
	}

	return domain
}

// valueFitsDomain reports whether a JSON value satisfies a Domain.
func (compiler *Compiler) valueFitsDomain(value jsonvalue.Value, domain Domain) (bool, error) {
	if !domainCanContainValue(domain, value) {
		return false, nil
	}

	switch value.Kind {
	case jsonvalue.KindNull:
		return domain.Null != KindExcluded, nil
	case jsonvalue.KindBoolean:
		return domain.Boolean != KindExcluded, nil
	case jsonvalue.KindNumber:
		return numberFits(value.Number, domain.Number)
	case jsonvalue.KindString:
		return stringFits(value.String, domain.String)
	case jsonvalue.KindArray:
		return compiler.arrayFits(value.Array, domain.Array)
	case jsonvalue.KindObject:
		return compiler.objectFits(value.Object, domain.Object)
	default:
		return false, errors.New("invalid enum value kind")
	}
}

// domainCanContainValue checks whole-Domain constraints shared by all JSON kinds.
func domainCanContainValue(domain Domain, value jsonvalue.Value) bool {
	return domain.Status != DomainEmpty && (domain.Enum == nil || enumContains(domain.Enum, value))
}

// enumContains reports semantic membership in a finite enum Domain.
func enumContains(enum *EnumSet, candidate jsonvalue.Value) bool {
	for _, value := range enum.Values {
		if value.Equal(candidate) {
			return true
		}
	}

	return false
}

// jsonValuesContain reports semantic JSON membership.
func jsonValuesContain(values []jsonvalue.Value, candidate jsonvalue.Value) bool {
	for _, value := range values {
		if value.Equal(candidate) {
			return true
		}
	}

	return false
}

// numberFits reports whether a JSON number satisfies number constraints.
func numberFits(value jsonvalue.Number, constraints NumberConstraints) (bool, error) {
	if constraints.State == KindExcluded || !fitsIntegerConstraint(value, constraints) {
		return false, nil
	}

	matches, err := fitsNumberBounds(value, constraints)
	if err != nil || !matches {
		return matches, err
	}

	return fitsMultipleOf(value, constraints.MultipleOf)
}

// fitsIntegerConstraint reports whether a value satisfies an integer-only constraint.
func fitsIntegerConstraint(value jsonvalue.Number, constraints NumberConstraints) bool {
	if !constraints.IntegersOnly {
		return true
	}

	return value.IsInteger()
}

// fitsNumberBounds reports whether a value is inside its minimum and maximum bounds.
func fitsNumberBounds(value jsonvalue.Number, constraints NumberConstraints) (bool, error) {
	matches, err := fitsMinimum(value, constraints.Minimum)
	if err != nil || !matches {
		return matches, err
	}

	return fitsMaximum(value, constraints.Maximum)
}

// fitsMinimum reports whether a value satisfies a minimum bound.
func fitsMinimum(value jsonvalue.Number, bound *NumberBound) (bool, error) {
	if bound == nil {
		return true, nil
	}

	comparison := value.Compare(bound.Value)

	return comparison > 0 || comparison == 0 && !bound.Exclusive, nil
}

// fitsMaximum reports whether a value satisfies a maximum bound.
func fitsMaximum(value jsonvalue.Number, bound *NumberBound) (bool, error) {
	if bound == nil {
		return true, nil
	}

	comparison := value.Compare(bound.Value)

	return comparison < 0 || comparison == 0 && !bound.Exclusive, nil
}

// fitsMultipleOf reports whether a value is an exact multiple of its divisor.
func fitsMultipleOf(value jsonvalue.Number, multipleOf *jsonvalue.Number) (bool, error) {
	if multipleOf == nil {
		return true, nil
	}

	return value.IsMultipleOf(*multipleOf), nil
}

// stringFits reports whether a JSON string satisfies string constraints.
func stringFits(value string, constraints StringConstraints) (bool, error) {
	if constraints.State == KindExcluded {
		return false, nil
	}

	length := utf8.RuneCountInString(value)
	if length < constraints.MinLength || constraints.MaxLength != nil && length > *constraints.MaxLength {
		return false, nil
	}

	if len(constraints.Patterns) > 0 || len(constraints.Formats) > 0 {
		return false, errOpaqueStringMembership
	}

	return true, nil
}

// errOpaqueStringMembership marks an enum membership check that needs occurrence evidence.
var errOpaqueStringMembership = fmt.Errorf(
	"%w: enum with pattern or format needs trusted compatible examples",
	errUnconstructible,
)

// childMembership aggregates recursive checks without letting opaque membership
// hide a definite modeled failure.
type childMembership struct {
	opaque bool
	failed bool
}

// add records one child result while propagating hard errors immediately.
func (membership *childMembership) add(matches bool, err error) error {
	if err == nil {
		membership.failed = membership.failed || !matches

		return nil
	}

	if errors.Is(err, errOpaqueStringMembership) {
		membership.opaque = true

		return nil
	}

	return err
}

// result applies hard error > false > opaque > true precedence.
func (membership childMembership) result() (bool, error) {
	if membership.failed {
		return false, nil
	}

	if membership.opaque {
		return false, errOpaqueStringMembership
	}

	return true, nil
}

// arrayFits reports whether a JSON array satisfies array constraints.
func (compiler *Compiler) arrayFits(values []jsonvalue.Value, constraints ArrayConstraints) (bool, error) {
	if constraints.State == KindExcluded || len(values) < constraints.MinItems ||
		constraints.MaxItems != nil && len(values) > *constraints.MaxItems {
		return false, nil
	}

	child, ok := compiler.Domains.Domain(constraints.Items)
	if !ok && len(values) > 0 {
		return false, errors.New("array item Domain does not exist")
	}

	var membership childMembership

	for _, value := range values {
		matches, err := compiler.valueFitsDomain(value, child)

		if addErr := membership.add(matches, err); addErr != nil {
			return false, addErr
		}
	}

	return membership.result()
}

// objectFits reports whether a JSON object satisfies object constraints.
func (compiler *Compiler) objectFits(members []jsonvalue.Member, constraints ObjectConstraints) (bool, error) {
	if !fitsObjectSize(members, constraints) {
		return false, nil
	}

	byName := membersByName(members)
	properties := propertyConstraintsByName(constraints.Properties)

	if !hasRequiredProperties(byName, properties) {
		return false, nil
	}

	return compiler.objectMembersFit(members, properties, constraints.Additional)
}

// fitsObjectSize reports whether an object has an allowed number of members.
func fitsObjectSize(members []jsonvalue.Member, constraints ObjectConstraints) bool {
	return constraints.State != KindExcluded && len(members) >= constraints.MinProps &&
		(constraints.MaxProps == nil || len(members) <= *constraints.MaxProps)
}

// membersByName indexes object members by name.
func membersByName(members []jsonvalue.Member) map[string]jsonvalue.Value {
	result := make(map[string]jsonvalue.Value, len(members))
	for _, member := range members {
		result[member.Name] = member.Value
	}

	return result
}

// propertyConstraintsByName indexes named property constraints by name.
func propertyConstraintsByName(properties []NamedProperty) map[string]NamedProperty {
	result := make(map[string]NamedProperty, len(properties))
	for _, property := range properties {
		result[property.Name] = property
	}

	return result
}

// hasRequiredProperties reports whether all required named properties are present.
func hasRequiredProperties(members map[string]jsonvalue.Value, properties map[string]NamedProperty) bool {
	for name, property := range properties {
		if property.Required {
			if _, ok := members[name]; !ok {
				return false
			}
		}
	}

	return true
}

// objectMembersFit reports whether each object member satisfies its selected child Domain.
func (compiler *Compiler) objectMembersFit(
	members []jsonvalue.Member,
	properties map[string]NamedProperty,
	additional AdditionalProperties,
) (bool, error) {
	last := make(map[string]int, len(members))
	for index, member := range members {
		last[member.Name] = index
	}

	var membership childMembership

	for index, member := range members {
		if last[member.Name] != index {
			continue
		}

		childID, allowed := propertyDomain(member.Name, properties, additional)
		if !allowed {
			if addErr := membership.add(false, nil); addErr != nil {
				return false, addErr
			}

			continue
		}

		child, ok := compiler.Domains.Domain(childID)
		if !ok {
			return false, errors.New("object property Domain does not exist")
		}

		matches, err := compiler.valueFitsDomain(member.Value, child)

		if addErr := membership.add(matches, err); addErr != nil {
			return false, addErr
		}
	}

	return membership.result()
}

// propertyDomain returns the child Domain and permission for one object member name.
func propertyDomain(
	name string,
	properties map[string]NamedProperty,
	additional AdditionalProperties,
) (DomainID, bool) {
	property, named := properties[name]
	if !named {
		return additional.Values, true
	}

	if property.State == PropertyForbidden {
		return NoDomain, false
	}

	return property.Values, true
}
