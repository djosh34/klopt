package suite

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
	"reflect"
	"sort"

	"decode_and_validate_generator/pkg/internal/jsonvalue"
)

// DomainRegistry owns canonical Domains and their semantic identity.
type DomainRegistry struct {
	Domains             []Domain
	HashByDomainID      map[DomainID]jsonvalue.Hash
	IDsByHash           map[jsonvalue.Hash][]DomainID
	IntersectionResults map[DomainPair]DomainID
}

// NewDomainRegistry creates a registry with stable unrestricted and empty Domains.
func NewDomainRegistry() *DomainRegistry {
	registry := &DomainRegistry{
		Domains:             make([]Domain, 1, initialDomainCapacity),
		HashByDomainID:      make(map[DomainID]jsonvalue.Hash),
		IDsByHash:           make(map[jsonvalue.Hash][]DomainID),
		IntersectionResults: make(map[DomainPair]DomainID),
	}
	registry.FindOrAddEquivalentDomain(anyJSONDomain())
	registry.FindOrAddEquivalentDomain(emptyDomain())

	return registry
}

// FindOrAddEquivalentDomain normalizes and reuses a semantically equivalent Domain.
func (registry *DomainRegistry) FindOrAddEquivalentDomain(candidate Domain) DomainID {
	normalized := registry.normalizeDomain(candidate)
	hash := registry.semanticDomainHash(normalized)

	for _, id := range registry.IDsByHash[hash] {
		if domainsAreEquivalent(registry.Domains[id], normalized) {
			return id
		}
	}

	id := DomainID(len(registry.Domains))
	registry.Domains = append(registry.Domains, normalized)
	registry.HashByDomainID[id] = hash
	registry.IDsByHash[hash] = append(registry.IDsByHash[hash], id)

	return id
}

// Domain returns a deep copy of one canonical Domain.
func (registry *DomainRegistry) Domain(id DomainID) (Domain, bool) {
	if id == NoDomain || int(id) >= len(registry.Domains) {
		return Domain{}, false
	}

	return cloneDomain(registry.Domains[id]), true
}

const (
	// initialDomainCapacity reserves the built-in Domains and the next compiled Domain.
	initialDomainCapacity = 3
	// minimumDistinctStrings is the shortest slice that can contain a duplicate.
	minimumDistinctStrings = 2
	// domainHashCapacity avoids growing the common semantic Domain encoding.
	domainHashCapacity = 256
)

// anyJSONDomain returns the unrestricted Domain containing every JSON value.
func anyJSONDomain() Domain {
	return Domain{
		Null:    KindUnrestricted,
		Boolean: KindUnrestricted,
		Number:  NumberConstraints{State: KindUnrestricted},
		String:  StringConstraints{State: KindUnrestricted},
		Array: ArrayConstraints{
			State: KindUnrestricted,
			Items: AnyJSONDomainID,
		},
		Object: ObjectConstraints{
			State: KindUnrestricted,
			Additional: AdditionalProperties{
				Values: AnyJSONDomainID,
			},
		},
		Status: DomainProductive,
	}
}

// emptyDomain returns the Domain containing no JSON values.
func emptyDomain() Domain {
	return Domain{Status: DomainEmpty}
}

// normalizeDomain removes irrelevant state before semantic comparison.
func (registry *DomainRegistry) normalizeDomain(domain Domain) Domain {
	domain = cloneDomain(domain)
	if domain.Status == DomainEmpty {
		return emptyDomain()
	}

	if domain.Status == DomainUnsupported || domain.Status == DomainUnconstructible {
		return domain
	}

	if allKindsExcluded(domain) {
		return emptyDomain()
	}

	domain.Status = DomainProductive
	normalizeNumber(&domain.Number)
	normalizeString(&domain.String)
	normalizeArray(&domain.Array)
	normalizeObject(&domain.Object)
	registry.normalizeChildProductivity(&domain)
	normalizeEnum(domain.Enum)

	if allKindsExcluded(domain) {
		return emptyDomain()
	}

	return domain
}

// normalizeNumber removes irrelevant numeric constraints.
func normalizeNumber(number *NumberConstraints) {
	if number.State == KindExcluded {
		*number = NumberConstraints{State: KindExcluded}

		return
	}

	if number.State == KindUnrestricted || number.State == KindRestricted &&
		!number.IntegersOnly && number.Minimum == nil && number.Maximum == nil &&
		number.MultipleOf == nil {
		*number = NumberConstraints{State: KindUnrestricted}
	}
}

// normalizeString canonicalizes string constraints.
func normalizeString(stringConstraints *StringConstraints) {
	if stringConstraints.State == KindExcluded {
		*stringConstraints = StringConstraints{State: KindExcluded}

		return
	}

	sort.Strings(stringConstraints.Patterns)
	stringConstraints.Patterns = compactStrings(stringConstraints.Patterns)
	sort.Strings(stringConstraints.Formats)
	stringConstraints.Formats = compactStrings(stringConstraints.Formats)

	if stringConstraints.State == KindUnrestricted || stringConstraints.State == KindRestricted &&
		stringConstraints.MinLength == 0 && stringConstraints.MaxLength == nil &&
		len(stringConstraints.Patterns) == 0 && len(stringConstraints.Formats) == 0 {
		*stringConstraints = StringConstraints{State: KindUnrestricted}
	}
}

// normalizeArray canonicalizes array constraints.
func normalizeArray(array *ArrayConstraints) {
	if array.State == KindExcluded {
		*array = ArrayConstraints{State: KindExcluded}

		return
	}

	if array.Items == NoDomain {
		array.Items = AnyJSONDomainID
	}

	if array.State == KindUnrestricted || array.State == KindRestricted &&
		array.Items == AnyJSONDomainID && array.MinItems == 0 && array.MaxItems == nil {
		*array = ArrayConstraints{State: KindUnrestricted, Items: AnyJSONDomainID}
	}
}

// normalizeObject canonicalizes object constraints.
func normalizeObject(object *ObjectConstraints) {
	if object.State == KindExcluded {
		*object = ObjectConstraints{State: KindExcluded}

		return
	}

	if object.Additional.Values == NoDomain {
		object.Additional.Values = AnyJSONDomainID
	}

	sort.Slice(object.Properties, func(left int, right int) bool {
		return object.Properties[left].Name < object.Properties[right].Name
	})

	if object.State == KindUnrestricted || object.State == KindRestricted &&
		len(object.Properties) == 0 && object.Additional.Values == AnyJSONDomainID &&
		object.MinProps == 0 && object.MaxProps == nil {
		*object = ObjectConstraints{
			State:      KindUnrestricted,
			Additional: AdditionalProperties{Values: AnyJSONDomainID},
		}
	}
}

// normalizeChildProductivity canonicalizes empty child Domains and excludes impossible containers.
func (registry *DomainRegistry) normalizeChildProductivity(domain *Domain) {
	if domain.Array.State != KindExcluded && registry.domainIsEmpty(domain.Array.Items) {
		domain.Array.MaxItems = new(0)
		if domain.Array.MinItems > 0 {
			domain.Array = ArrayConstraints{State: KindExcluded}
		}
	}

	if domain.Object.State == KindExcluded {
		return
	}

	for index := range domain.Object.Properties {
		property := &domain.Object.Properties[index]
		if property.State == PropertyForbidden || registry.domainIsEmpty(property.Values) {
			if property.Required {
				domain.Object = ObjectConstraints{State: KindExcluded}

				return
			}

			property.State = PropertyForbidden
			property.Values = EmptyDomainID
		}
	}

	if !registry.objectConstraintsAreProductive(domain.Object) {
		domain.Object = ObjectConstraints{State: KindExcluded}
	}
}

// normalizeEnum orders enum values by their JSON encodings.
func normalizeEnum(enum *EnumSet) {
	if enum == nil {
		return
	}

	sort.Slice(enum.Values, func(left int, right int) bool {
		leftJSON, leftErr := enum.Values[left].MarshalJSON()

		rightJSON, rightErr := enum.Values[right].MarshalJSON()
		if leftErr != nil || rightErr != nil {
			return leftErr == nil
		}

		return bytes.Compare(leftJSON, rightJSON) < 0
	})

	if len(enum.Values) < minimumDistinctStrings {
		return
	}

	values := enum.Values[:1]
	for _, value := range enum.Values[1:] {
		if !value.Equal(values[len(values)-1]) {
			values = append(values, value)
		}
	}

	enum.Values = values
}

// compactStrings removes adjacent duplicate strings from a sorted slice.
func compactStrings(values []string) []string {
	if len(values) < minimumDistinctStrings {
		return values
	}

	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}

	return result
}

// allKindsExcluded reports whether a Domain excludes every JSON kind.
func allKindsExcluded(domain Domain) bool {
	return domain.Null == KindExcluded && domain.Boolean == KindExcluded &&
		domain.Number.State == KindExcluded && domain.String.State == KindExcluded &&
		domain.Array.State == KindExcluded && domain.Object.State == KindExcluded
}

// semanticDomainHash hashes the normalized semantic Domain representation.
func (registry *DomainRegistry) semanticDomainHash(domain Domain) jsonvalue.Hash {
	encoded := make([]byte, 0, domainHashCapacity)
	encoded = append(encoded, byte(domain.Status), byte(domain.Null), byte(domain.Boolean))
	encoded = registry.appendNumber(encoded, domain.Number)
	encoded = registry.appendString(encoded, domain.String)
	encoded = registry.appendArray(encoded, domain.Array)
	encoded = registry.appendObject(encoded, domain.Object)
	encoded = appendEnum(encoded, domain.Enum)

	return sha256.Sum256(encoded)
}

// appendNumber appends the semantic numeric constraint encoding.
func (registry *DomainRegistry) appendNumber(encoded []byte, number NumberConstraints) []byte {
	encoded = append(encoded, byte(number.State))
	encoded = appendBool(encoded, number.IntegersOnly)
	encoded = appendBound(encoded, number.Minimum)
	encoded = appendBound(encoded, number.Maximum)

	return appendNumberValue(encoded, number.MultipleOf)
}

// appendString appends the semantic string constraint encoding.
func (registry *DomainRegistry) appendString(encoded []byte, value StringConstraints) []byte {
	encoded = append(encoded, byte(value.State))
	encoded = binary.AppendUvarint(encoded, uint64(value.MinLength))
	encoded = appendOptionalInt(encoded, value.MaxLength)
	encoded = appendStrings(encoded, value.Patterns)

	return appendStrings(encoded, value.Formats)
}

// appendArray appends the semantic array constraint encoding.
func (registry *DomainRegistry) appendArray(encoded []byte, array ArrayConstraints) []byte {
	encoded = append(encoded, byte(array.State))
	encoded = registry.appendChildHash(encoded, array.Items)
	encoded = binary.AppendUvarint(encoded, uint64(array.MinItems))

	return appendOptionalInt(encoded, array.MaxItems)
}

// appendObject appends the semantic object constraint encoding.
func (registry *DomainRegistry) appendObject(encoded []byte, object ObjectConstraints) []byte {
	encoded = append(encoded, byte(object.State))

	encoded = binary.AppendUvarint(encoded, uint64(len(object.Properties)))
	for _, property := range object.Properties {
		encoded = appendBytes(encoded, []byte(property.Name))
		encoded = appendBool(encoded, property.Required)
		encoded = append(encoded, byte(property.State))
		encoded = registry.appendChildHash(encoded, property.Values)
	}

	encoded = registry.appendChildHash(encoded, object.Additional.Values)
	encoded = binary.AppendUvarint(encoded, uint64(object.MinProps))

	return appendOptionalInt(encoded, object.MaxProps)
}

// appendChildHash appends a child Domain hash or a stable missing-Domain marker.
func (registry *DomainRegistry) appendChildHash(encoded []byte, id DomainID) []byte {
	if id == AnyJSONDomainID {
		return appendBytes(encoded, []byte("builtin:any-json"))
	}

	if id == EmptyDomainID {
		return appendBytes(encoded, []byte("builtin:empty"))
	}

	hash, ok := registry.HashByDomainID[id]
	if !ok {
		return appendBytes(encoded, []byte(fmt.Sprintf("missing:%d", id)))
	}

	return append(encoded, hash[:]...)
}

// appendEnum appends the finite enum encoding.
func appendEnum(encoded []byte, enum *EnumSet) []byte {
	if enum == nil {
		return appendBool(encoded, false)
	}

	encoded = appendBool(encoded, true)

	encoded = binary.AppendUvarint(encoded, uint64(len(enum.Values)))
	for _, value := range enum.Values {
		valueJSON, err := value.MarshalJSON()
		if err != nil {
			encoded = appendBytes(encoded, []byte("invalid:"+err.Error()))

			continue
		}

		encoded = appendBytes(encoded, valueJSON)
	}

	return encoded
}

// domainsAreEquivalent reports whether normalized Domains have the same representation.
func domainsAreEquivalent(left Domain, right Domain) bool {
	leftEnum := left.Enum
	rightEnum := right.Enum
	left.Enum = nil

	right.Enum = nil
	if !reflect.DeepEqual(left, right) {
		return false
	}

	if leftEnum == nil || rightEnum == nil {
		return leftEnum == nil && rightEnum == nil
	}

	if len(leftEnum.Values) != len(rightEnum.Values) {
		return false
	}

	for index, value := range leftEnum.Values {
		if !value.Equal(rightEnum.Values[index]) {
			return false
		}
	}

	return true
}

// cloneDomain returns a deep copy of a Domain.
func cloneDomain(domain Domain) Domain {
	domain.Number.Minimum = cloneBound(domain.Number.Minimum)
	domain.Number.Maximum = cloneBound(domain.Number.Maximum)
	domain.Number.MultipleOf = cloneNumber(domain.Number.MultipleOf)

	if domain.String.MaxLength != nil {
		domain.String.MaxLength = new(*domain.String.MaxLength)
	}

	domain.String.Patterns = append([]string(nil), domain.String.Patterns...)
	domain.String.Formats = append([]string(nil), domain.String.Formats...)

	if domain.Array.MaxItems != nil {
		domain.Array.MaxItems = new(*domain.Array.MaxItems)
	}

	if domain.Object.MaxProps != nil {
		domain.Object.MaxProps = new(*domain.Object.MaxProps)
	}

	domain.Object.Properties = append([]NamedProperty(nil), domain.Object.Properties...)
	if domain.Enum != nil {
		values := make([]jsonvalue.Value, len(domain.Enum.Values))
		for index, value := range domain.Enum.Values {
			values[index] = cloneJSONValue(value)
		}

		domain.Enum = &EnumSet{Values: values}
	}

	return domain
}

// cloneJSONValue returns a deep copy of an exact JSON value.
func cloneJSONValue(value jsonvalue.Value) jsonvalue.Value {
	if value.Number.Rational != nil {
		value.Number.Rational = new(big.Rat).Set(value.Number.Rational)
	}

	if value.Array != nil {
		array := make([]jsonvalue.Value, len(value.Array))
		for index, child := range value.Array {
			array[index] = cloneJSONValue(child)
		}

		value.Array = array
	}

	if value.Object != nil {
		object := make([]jsonvalue.Member, len(value.Object))
		for index, member := range value.Object {
			object[index] = jsonvalue.Member{Name: member.Name, Value: cloneJSONValue(member.Value)}
		}

		value.Object = object
	}

	return value
}

// cloneBound returns a deep copy of a number bound.
func cloneBound(bound *NumberBound) *NumberBound {
	if bound == nil {
		return nil
	}

	return &NumberBound{Value: *cloneNumber(&bound.Value), Exclusive: bound.Exclusive}
}

// cloneNumber returns a deep copy of an exact JSON number.
func cloneNumber(number *jsonvalue.Number) *jsonvalue.Number {
	if number == nil {
		return nil
	}

	result := &jsonvalue.Number{Lexeme: number.Lexeme}
	if number.Rational != nil {
		result.Rational = new(big.Rat).Set(number.Rational)
	}

	return result
}

// appendBound appends an optional number bound encoding.
func appendBound(encoded []byte, bound *NumberBound) []byte {
	if bound == nil {
		return appendBool(encoded, false)
	}

	encoded = appendBool(encoded, true)
	encoded = appendBytes(encoded, []byte(bound.Value.Lexeme))

	return appendBool(encoded, bound.Exclusive)
}

// appendNumberValue appends an optional exact number encoding.
func appendNumberValue(encoded []byte, number *jsonvalue.Number) []byte {
	if number == nil {
		return appendBool(encoded, false)
	}

	encoded = appendBool(encoded, true)

	return appendBytes(encoded, []byte(number.Lexeme))
}

// appendOptionalString appends an optional string encoding.
func appendOptionalString(encoded []byte, value *string) []byte {
	if value == nil {
		return appendBool(encoded, false)
	}

	encoded = appendBool(encoded, true)

	return appendBytes(encoded, []byte(*value))
}

// appendOptionalInt appends an optional integer encoding.
func appendOptionalInt(encoded []byte, value *int) []byte {
	if value == nil {
		return appendBool(encoded, false)
	}

	encoded = appendBool(encoded, true)

	return binary.AppendUvarint(encoded, uint64(*value))
}

// appendStrings appends a string slice encoding.
func appendStrings(encoded []byte, values []string) []byte {
	encoded = binary.AppendUvarint(encoded, uint64(len(values)))
	for _, value := range values {
		encoded = appendBytes(encoded, []byte(value))
	}

	return encoded
}

// appendBytes appends a length-prefixed byte slice.
func appendBytes(encoded []byte, value []byte) []byte {
	encoded = binary.AppendUvarint(encoded, uint64(len(value)))

	return append(encoded, value...)
}

// appendBool appends a stable boolean encoding.
func appendBool(encoded []byte, value bool) []byte {
	if value {
		return append(encoded, 1)
	}

	return append(encoded, 0)
}
