package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.
)

// parseEnums parses and canonicalizes an optional enum constraint.
func parseEnums(jsonKV JSONKV) ([]types.Enum, bool, error) {
	enumRaw, ok := jsonKV["enum"]
	if !ok {
		return nil, false, nil
	}

	var enumValues []json.RawMessage
	if err := json.Unmarshal(enumRaw, &enumValues); err != nil {
		return nil, true, errors.New("enum must be array")
	}

	if enumValues == nil {
		return nil, true, errors.New("enum cannot be null")
	}

	if len(enumValues) == 0 {
		return nil, true, errors.New("enum cannot be empty")
	}

	enums := make([]types.Enum, 0, len(enumValues))
	for _, enumValue := range enumValues {
		enums = append(enums, types.Enum(enumValue))
	}

	canonical, err := canonicalEnums(enums)
	if err != nil {
		return nil, true, err
	}

	return canonical, true, nil
}

// parseEnumsByType parses an enum constraint and filters it by type and nullable.
func parseEnumsByType(jsonKV JSONKV, schemaType string, nullable bool) ([]types.Enum, error) {
	enums, _, err := parseEnums(jsonKV)
	if err != nil {
		return nil, err
	}

	return filterEnumsByType(enums, schemaType, nullable)
}

// mergeEnums intersects two optional enum constraints by semantic JSON value.
func mergeEnums(left []types.Enum, right []types.Enum) ([]types.Enum, error) {
	canonicalLeft, err := canonicalEnums(left)
	if err != nil {
		return nil, err
	}

	canonicalRight, err := canonicalEnums(right)
	if err != nil {
		return nil, err
	}

	if canonicalLeft == nil {
		return canonicalRight, nil
	}

	if canonicalRight == nil {
		return canonicalLeft, nil
	}

	return intersectEnums(canonicalLeft, canonicalRight)
}

// mergeEnumsByType intersects enum constraints and filters them by type and nullable.
func mergeEnumsByType(left []types.Enum, right []types.Enum, schemaType string, nullable bool) ([]types.Enum, error) {
	enums, err := mergeEnums(left, right)
	if err != nil {
		return nil, err
	}

	return filterEnumsByType(enums, schemaType, nullable)
}

// canonicalEnums validates and canonicalizes every raw enum value.
func canonicalEnums(enums []types.Enum) ([]types.Enum, error) {
	if enums == nil {
		return nil, nil
	}

	if len(enums) == 0 {
		return nil, errors.New("enum cannot be empty")
	}

	canonical := make([]types.Enum, 0, len(enums))
	for _, enumValue := range enums {
		value, err := types.CanonicalEnum(json.RawMessage(enumValue))
		if err != nil {
			return nil, err
		}

		canonical = append(canonical, value)
	}

	sort.Slice(canonical, func(first int, second int) bool {
		return bytes.Compare(canonical[first], canonical[second]) < 0
	})

	unique := canonical[:0]
	for _, value := range canonical {
		if len(unique) == 0 || !bytes.Equal(unique[len(unique)-1], value) {
			unique = append(unique, value)
		}
	}

	return unique, nil
}

// intersectEnums retains the canonical values present in both constraints.
func intersectEnums(left []types.Enum, right []types.Enum) ([]types.Enum, error) {
	intersection := make([]types.Enum, 0, min(len(left), len(right)))
	for leftIndex, rightIndex := 0, 0; leftIndex < len(left) && rightIndex < len(right); {
		switch comparison := bytes.Compare(left[leftIndex], right[rightIndex]); {
		case comparison < 0:
			leftIndex++
		case comparison > 0:
			rightIndex++
		default:
			intersection = append(intersection, left[leftIndex])
			leftIndex++
			rightIndex++
		}
	}

	if len(intersection) == 0 {
		return nil, errors.New("enum intersection is empty")
	}

	return intersection, nil
}

// filterEnumsByType retains enum values allowed by a schema's type and nullable fields.
func filterEnumsByType(enums []types.Enum, schemaType string, nullable bool) ([]types.Enum, error) {
	canonical, err := canonicalEnums(enums)
	if err != nil || canonical == nil {
		return canonical, err
	}

	if !isSupportedEnumSchemaType(schemaType) {
		return nil, fmt.Errorf("unsupported enum schema type %q", schemaType)
	}

	filtered := make([]types.Enum, 0, len(canonical))
	for _, value := range canonical {
		if enumMatchesType(value, schemaType, nullable) {
			filtered = append(filtered, value)
		}
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("enum has no values compatible with %s schema", schemaType)
	}

	return filtered, nil
}

// enumAllowsNull reports whether nullable and enum constraints both permit null.
func enumAllowsNull(nullable bool, enums []types.Enum) (bool, error) {
	canonical, err := canonicalEnums(enums)
	if err != nil {
		return false, err
	}

	if !nullable {
		return false, nil
	}

	if canonical == nil {
		return true, nil
	}

	index := sort.Search(len(canonical), func(index int) bool {
		return bytes.Compare(canonical[index], types.Enum("null")) >= 0
	})

	return index < len(canonical) && bytes.Equal(canonical[index], types.Enum("null")), nil
}

// enumAllowsJSONValue reports whether an optional enum contains a canonical JSON value.
func enumAllowsJSONValue(enums []types.Enum, value types.Enum) (bool, error) {
	canonical, err := canonicalEnums(enums)
	if err != nil {
		return false, err
	}

	if canonical == nil {
		return true, nil
	}

	index := sort.Search(len(canonical), func(index int) bool {
		return bytes.Compare(canonical[index], value) >= 0
	})

	return index < len(canonical) && bytes.Equal(canonical[index], value), nil
}

// domainAllowsJSONValue reports whether domain accepts one JSON value.
func domainAllowsJSONValue(domain types.Domain, value json.RawMessage) (bool, error) {
	if domainIsNil(domain) {
		return false, errors.New("domain cannot be nil")
	}

	canonical, err := types.CanonicalEnum(value)
	if err != nil {
		return false, err
	}

	return domainAllowsCanonicalJSONValue(domain, canonical)
}

// domainAllowsCanonicalJSONValue validates an already canonical JSON value.
func domainAllowsCanonicalJSONValue(domain types.Domain, value types.Enum) (bool, error) {
	if allOfDomain, ok := domain.(*AllOfDomain); ok {
		if domainIsNil(allOfDomain.MergedDomain) {
			return false, errors.New("allOf merged domain cannot be nil")
		}

		return domainAllowsCanonicalJSONValue(allOfDomain.MergedDomain, value)
	}

	if bytes.Equal(value, types.Enum("null")) {
		return domainAllowsNull(domain)
	}

	switch concrete := domain.(type) {
	case *ArrayDomain:
		return arrayDomainAllowsJSONValue(concrete, value)
	case *BoolDomain:
		return boolDomainAllowsJSONValue(concrete, value)
	case *NumberDomain:
		return numberDomainAllowsJSONValue(concrete, value)
	case *ObjectDomain:
		return objectDomainAllowsJSONValue(concrete, value)
	case *StringDomain:
		return stringDomainAllowsJSONValue(concrete, value)
	default:
		return false, fmt.Errorf("unsupported domain type %T", domain)
	}
}

// arrayDomainAllowsJSONValue validates one non-null array value.
func arrayDomainAllowsJSONValue(domain *ArrayDomain, value types.Enum) (bool, error) {
	if err := validateArrayConstraintValues(domain); err != nil {
		return false, err
	}

	allowed, err := enumAllowsJSONValue(domain.Enum, value)
	if err != nil || !allowed {
		return allowed, err
	}

	if !enumMatchesNonNullType(value, "array") {
		return false, nil
	}

	var items []json.RawMessage
	if err := json.Unmarshal(value, &items); err != nil {
		return false, err
	}

	if !arrayLengthAllowed(domain, len(items)) {
		return false, nil
	}

	if domain.Items == nil {
		return true, nil
	}

	return arrayItemsAllowed(domain.Items, items)
}

// arrayLengthAllowed reports whether an item count satisfies array bounds.
func arrayLengthAllowed(domain *ArrayDomain, length int) bool {
	return length >= domain.MinItems && (domain.MaxItems == nil || length <= *domain.MaxItems)
}

// arrayItemsAllowed validates every item against the shared item domain.
func arrayItemsAllowed(itemsDomain types.Domain, items []json.RawMessage) (bool, error) {
	for index, item := range items {
		itemAllowed, itemErr := domainAllowsJSONValue(itemsDomain, item)
		if itemErr != nil {
			return false, fmt.Errorf("array item %d: %w", index, itemErr)
		}

		if !itemAllowed {
			return false, nil
		}
	}

	return true, nil
}

// boolDomainAllowsJSONValue validates one non-null boolean value.
func boolDomainAllowsJSONValue(domain *BoolDomain, value types.Enum) (bool, error) {
	allowed, err := enumAllowsJSONValue(domain.Enum, value)
	if err != nil || !allowed {
		return allowed, err
	}

	return enumIsBoolean(value), nil
}

// numberDomainAllowsJSONValue validates one non-null numeric value.
func numberDomainAllowsJSONValue(domain *NumberDomain, value types.Enum) (bool, error) {
	validatedDomain := *domain
	if validatedDomain.Type == "" {
		validatedDomain.Type = "number"
	}

	if err := validateNumberDomainConstraints(&validatedDomain); err != nil {
		return false, err
	}

	allowed, err := enumAllowsJSONValue(validatedDomain.Enum, value)
	if err != nil || !allowed {
		return allowed, err
	}

	if !enumIsNumber(value) {
		return false, nil
	}

	number := Number(value)

	rational, err := numberToRat(&number)
	if err != nil {
		return false, err
	}

	return numberValueAllowed(&validatedDomain, rational)
}

// objectDomainAllowsJSONValue validates one non-null object value.
func objectDomainAllowsJSONValue(domain *ObjectDomain, value types.Enum) (bool, error) {
	validatedDomain := *domain
	if err := validateObjectPropertyBounds(&validatedDomain); err != nil {
		return false, err
	}

	if err := validateAdditionalPropertySchema(&validatedDomain); err != nil {
		return false, err
	}

	if err := canonicalizeObjectProperties(&validatedDomain); err != nil {
		return false, err
	}

	allowed, err := enumAllowsJSONValue(validatedDomain.Enum, value)
	if err != nil || !allowed {
		return allowed, err
	}

	if !enumMatchesNonNullType(value, "object") {
		return false, nil
	}

	return objectEnumAllowed(&validatedDomain, value)
}

// stringDomainAllowsJSONValue validates one non-null string value.
func stringDomainAllowsJSONValue(domain *StringDomain, value types.Enum) (bool, error) {
	if err := validateStringConstraintValues(domain); err != nil {
		return false, err
	}

	allowed, err := enumAllowsJSONValue(domain.Enum, value)
	if err != nil || !allowed {
		return allowed, err
	}

	if !enumMatchesNonNullType(value, "string") {
		return false, nil
	}

	stringValue, err := unmarshalJSONString(value)
	if err != nil {
		return false, err
	}

	if !stringLengthAllowed(domain, stringValue) {
		return false, nil
	}

	return stringMatchesTrustedConstraints(domain, stringValue), nil
}

// stringLengthAllowed reports whether a string's rune count satisfies its bounds.
func stringLengthAllowed(domain *StringDomain, value string) bool {
	length := utf8.RuneCountInString(value)

	return length >= domain.MinLength && (domain.MaxLength == nil || length <= *domain.MaxLength)
}

// stringMatchesTrustedConstraints applies generation examples to pattern and format values.
func stringMatchesTrustedConstraints(domain *StringDomain, value string) bool {
	if len(domain.Pattern) == 0 && len(domain.Format) == 0 {
		return true
	}

	for _, example := range domain.XValidExamples {
		if value == example {
			return true
		}
	}

	return false
}

// mergeDomainsAsNullOnly returns the null-only intersection of differently typed domains.
func mergeDomainsAsNullOnly(left types.Domain, right types.Domain) (types.Domain, bool, error) {
	leftAllowsNull, err := domainAllowsNull(left)
	if err != nil {
		return nil, false, err
	}

	rightAllowsNull, err := domainAllowsNull(right)
	if err != nil {
		return nil, false, err
	}

	if !leftAllowsNull || !rightAllowsNull {
		return nil, false, nil
	}

	return &BoolDomain{
		Nullable: true,
		Enum:     []types.Enum{types.Enum("null")},
	}, true, nil
}

// domainAllowsNull reports whether a concrete domain's nullable and enum constraints admit null.
func domainAllowsNull(domain types.Domain) (bool, error) {
	if domainIsNil(domain) {
		return false, nil
	}

	switch concrete := domain.(type) {
	case *AllOfDomain:
		return domainAllowsNull(concrete.MergedDomain)
	case *ArrayDomain:
		return enumAllowsNull(concrete.Nullable, concrete.Enum)
	case *BoolDomain:
		return enumAllowsNull(concrete.Nullable, concrete.Enum)
	case *NumberDomain:
		return enumAllowsNull(concrete.Nullable, concrete.Enum)
	case *ObjectDomain:
		return enumAllowsNull(concrete.Nullable, concrete.Enum)
	case *StringDomain:
		return enumAllowsNull(concrete.Nullable, concrete.Enum)
	default:
		return false, nil
	}
}

// domainIsNil reports whether a domain interface is nil or contains a nil pointer.
func domainIsNil(domain types.Domain) bool {
	value := reflect.ValueOf(domain)

	return !value.IsValid() || value.Kind() == reflect.Pointer && value.IsNil()
}

// isSupportedEnumSchemaType reports whether enum filtering supports schemaType.
func isSupportedEnumSchemaType(schemaType string) bool {
	switch schemaType {
	case "array", "boolean", "integer", "number", "object", "string":
		return true
	default:
		return false
	}
}

// enumMatchesType reports whether one canonical value satisfies type and nullable.
func enumMatchesType(value types.Enum, schemaType string, nullable bool) bool {
	if bytes.Equal(value, types.Enum("null")) {
		return nullable
	}

	if len(value) == 0 {
		return false
	}

	return enumMatchesNonNullType(value, schemaType)
}

// enumMatchesNonNullType reports whether one canonical non-null value satisfies type.
func enumMatchesNonNullType(value types.Enum, schemaType string) bool {
	switch schemaType {
	case "array":
		return value[0] == '['
	case "boolean":
		return enumIsBoolean(value)
	case "integer":
		return enumNumberIsInteger(value)
	case "number":
		return enumIsNumber(value)
	case "object":
		return value[0] == '{'
	case "string":
		return value[0] == '"'
	default:
		return false
	}
}

// enumIsBoolean reports whether a canonical value is a JSON boolean.
func enumIsBoolean(value types.Enum) bool {
	return bytes.Equal(value, types.Enum("false")) || bytes.Equal(value, types.Enum("true"))
}

// enumIsNumber reports whether a canonical value is a JSON number.
func enumIsNumber(value types.Enum) bool {
	return value[0] == '-' || value[0] >= '0' && value[0] <= '9'
}

// enumNumberIsInteger reports whether a canonical numeric enum has no fraction.
func enumNumberIsInteger(value types.Enum) bool {
	if len(value) == 0 || value[0] != '-' && (value[0] < '0' || value[0] > '9') {
		return false
	}

	number := string(value)
	if strings.Contains(number, ".") {
		return false
	}

	exponentIndex := strings.IndexByte(number, 'e')

	return exponentIndex < 0 || number[exponentIndex+1] != '-'
}
