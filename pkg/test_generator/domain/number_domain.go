package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.
)

// NumberDomain describes the values accepted by an OpenAPI numeric schema.
type NumberDomain struct {
	Type     string       `json:"type"`
	Nullable bool         `json:"nullable"`
	Enum     []types.Enum `json:"enum"`

	Minimum          *Number `json:"minimum"`
	Maximum          *Number `json:"maximum"`
	ExclusiveMinimum bool    `json:"exclusiveMinimum"`
	ExclusiveMaximum bool    `json:"exclusiveMaximum"`
	MultipleOf       *Number `json:"multipleOf"`
	Format           *string `json:"format"`
}

// AllOfMerge intersects the numeric domain with another domain.
func (n *NumberDomain) AllOfMerge(domain types.Domain) (types.Domain, error) {
	if n == nil {
		return nil, errors.New("number domain cannot be nil")
	}

	if allOfDomain, ok := domain.(*AllOfDomain); ok {
		mergedAllOf := &AllOfDomain{Domains: []types.Domain{n}, MergedDomain: n}

		return mergedAllOf.AllOfMerge(allOfDomain)
	}

	otherNumber, ok := domain.(*NumberDomain)
	if !ok || otherNumber == nil {
		return mergeNumberTypeMismatch(n, domain)
	}

	merged := *n
	if err := n.mergeType(otherNumber, &merged); err != nil {
		return nil, err
	}

	n.mergeNullable(otherNumber, &merged)

	if err := n.mergeEnum(otherNumber, &merged); err != nil {
		return nil, err
	}

	if err := n.mergeMinimum(otherNumber, &merged); err != nil {
		return nil, err
	}

	if err := n.mergeMaximum(otherNumber, &merged); err != nil {
		return nil, err
	}

	if err := n.mergeMultipleOf(otherNumber, &merged); err != nil {
		return nil, err
	}

	n.mergeFormat(otherNumber, &merged)

	return finalizeMergedNumberDomain(&merged)
}

// mergeNumberTypeMismatch retains a shared null value across concrete types.
func mergeNumberTypeMismatch(number *NumberDomain, domain types.Domain) (types.Domain, error) {
	nullOnly, merged, err := mergeDomainsAsNullOnly(number, domain)
	if err != nil {
		return nil, err
	}

	if merged {
		return nullOnly, nil
	}

	return nil, errors.New("domain is not NumberDomain")
}

// finalizeMergedNumberDomain validates a merge while preserving nil-on-error behavior.
func finalizeMergedNumberDomain(merged *NumberDomain) (types.Domain, error) {
	if err := finalizeNumberDomain(merged); err != nil {
		return nil, err
	}

	return merged, nil
}

// GenerateHash returns a deterministic hash of the numeric domain.
func (n *NumberDomain) GenerateHash() (types.Hash, error) {
	if n == nil {
		return types.Hash{}, errors.New("domain of number cannot be nil")
	}

	hashType := n.Type
	if hashType == "" {
		hashType = "number"
	}

	value := *n

	value.Type = hashType
	if err := finalizeNumberDomain(&value); err != nil {
		return types.Hash{}, err
	}

	return generateHash(hashType, value)
}

// finalizeNumberDomain rejects numeric constraints with no permitted value.
func finalizeNumberDomain(domain *NumberDomain) error {
	if err := validateNumberDomainConstraints(domain); err != nil {
		return err
	}

	if err := canonicalizeNumberConstraints(domain); err != nil {
		return err
	}

	if domain.Enum != nil {
		return filterNumberEnumsByConstraints(domain)
	}

	hasNumericValue, err := numberDomainHasNumericValue(domain)
	if err != nil {
		return err
	}

	if hasNumericValue || domain.Nullable {
		return nil
	}

	return errors.New("number domain has no valid values")
}

// canonicalizeNumberConstraints normalizes equivalent JSON number spellings.
func canonicalizeNumberConstraints(domain *NumberDomain) error {
	var err error

	domain.Minimum, err = canonicalNumberConstraint(domain.Minimum)
	if err != nil {
		return err
	}

	if domain.Minimum == nil {
		domain.ExclusiveMinimum = false
	}

	domain.Maximum, err = canonicalNumberConstraint(domain.Maximum)
	if err != nil {
		return err
	}

	if domain.Maximum == nil {
		domain.ExclusiveMaximum = false
	}

	domain.MultipleOf, err = canonicalNumberConstraint(domain.MultipleOf)

	return err
}

// canonicalNumberConstraint normalizes one optional numeric constraint.
func canonicalNumberConstraint(number *Number) (*Number, error) {
	if number == nil {
		return nil, nil //nolint:nilnil // Nil means the optional constraint is absent.
	}

	canonical, err := types.CanonicalEnum(json.RawMessage(*number))
	if err != nil {
		return nil, err
	}

	canonicalNumber := Number(canonical)

	return &canonicalNumber, nil
}

// validateNumberDomainConstraints validates numeric type and constraint lexemes.
func validateNumberDomainConstraints(domain *NumberDomain) error {
	if domain == nil {
		return errors.New("number domain cannot be nil")
	}

	if domain.Type != "number" && domain.Type != "integer" {
		return errors.New("number domain type must be number or integer")
	}

	if _, err := numberToRat(domain.Minimum); err != nil {
		return err
	}

	if _, err := numberToRat(domain.Maximum); err != nil {
		return err
	}

	if err := validatePositiveMultipleOf(domain.MultipleOf); err != nil {
		return err
	}

	return nil
}

// filterNumberEnumsByConstraints retains enum values satisfying every numeric constraint.
func filterNumberEnumsByConstraints(domain *NumberDomain) error {
	enums, err := filterEnumsByType(domain.Enum, domain.Type, domain.Nullable)
	if err != nil {
		return err
	}

	filtered := make([]types.Enum, 0, len(enums))
	for _, enumValue := range enums {
		if string(enumValue) == "null" {
			filtered = append(filtered, enumValue)

			continue
		}

		number := Number(enumValue)

		value, valueErr := numberToRat(&number)
		if valueErr != nil {
			return valueErr
		}

		allowed, allowedErr := numberValueAllowed(domain, value)
		if allowedErr != nil {
			return allowedErr
		}

		if allowed {
			filtered = append(filtered, enumValue)
		}
	}

	if len(filtered) == 0 {
		return errors.New("enum has no values compatible with numeric constraints")
	}

	domain.Enum = filtered

	return nil
}

// mergeType intersects number and integer instance types.
func (n *NumberDomain) mergeType(otherNumber *NumberDomain, merged *NumberDomain) error {
	if (n.Type != "number" && n.Type != "integer") || (otherNumber.Type != "number" && otherNumber.Type != "integer") {
		return errors.New("number domain type must be number or integer")
	}

	if n.Type == "integer" || otherNumber.Type == "integer" {
		merged.Type = "integer"

		return nil
	}

	merged.Type = "number"

	return nil
}

// mergeNullable retains null only when both domains allow it.
func (n *NumberDomain) mergeNullable(otherNumber *NumberDomain, merged *NumberDomain) {
	merged.Nullable = n.Nullable && otherNumber.Nullable
}

// mergeEnum intersects the optional enum constraints.
func (n *NumberDomain) mergeEnum(otherNumber *NumberDomain, merged *NumberDomain) error {
	enums, err := mergeEnumsByType(n.Enum, otherNumber.Enum, merged.Type, merged.Nullable)
	if err != nil {
		return err
	}

	merged.Enum = enums

	return nil
}

// mergeMinimum selects the tighter lower bound.
func (n *NumberDomain) mergeMinimum(otherNumber *NumberDomain, merged *NumberDomain) error {
	switch {
	case n.Minimum == nil:
		return mergeMinimumFromOther(otherNumber, merged)
	case otherNumber.Minimum == nil:
		return mergeMinimumFromLeft(n, merged)
	default:
		return mergeMinimums(n, otherNumber, merged)
	}
}

// mergeMinimumFromOther copies and validates the right lower bound.
func mergeMinimumFromOther(otherNumber *NumberDomain, merged *NumberDomain) error {
	if err := validateNumberLexeme(otherNumber.Minimum); err != nil {
		return err
	}

	merged.Minimum = otherNumber.Minimum
	merged.ExclusiveMinimum = otherNumber.Minimum != nil && otherNumber.ExclusiveMinimum

	return nil
}

// mergeMinimumFromLeft copies and validates the left lower bound.
func mergeMinimumFromLeft(leftNumber *NumberDomain, merged *NumberDomain) error {
	if err := validateNumberLexeme(leftNumber.Minimum); err != nil {
		return err
	}

	merged.Minimum = leftNumber.Minimum
	merged.ExclusiveMinimum = leftNumber.ExclusiveMinimum

	return nil
}

// mergeMinimums selects the numerically greater lower bound.
func mergeMinimums(leftNumber *NumberDomain, rightNumber *NumberDomain, merged *NumberDomain) error {
	comparison, err := compareNumbers(*leftNumber.Minimum, *rightNumber.Minimum)
	if err != nil {
		return err
	}

	if comparison < 0 {
		merged.Minimum = rightNumber.Minimum
		merged.ExclusiveMinimum = rightNumber.ExclusiveMinimum

		return nil
	}

	merged.Minimum = leftNumber.Minimum

	merged.ExclusiveMinimum = leftNumber.ExclusiveMinimum
	if comparison == 0 {
		merged.ExclusiveMinimum = leftNumber.ExclusiveMinimum || rightNumber.ExclusiveMinimum
	}

	return nil
}

// mergeMaximum selects the tighter upper bound.
func (n *NumberDomain) mergeMaximum(otherNumber *NumberDomain, merged *NumberDomain) error {
	switch {
	case n.Maximum == nil:
		return mergeMaximumFromOther(otherNumber, merged)
	case otherNumber.Maximum == nil:
		return mergeMaximumFromLeft(n, merged)
	default:
		return mergeMaximums(n, otherNumber, merged)
	}
}

// mergeMaximumFromOther copies and validates the right upper bound.
func mergeMaximumFromOther(otherNumber *NumberDomain, merged *NumberDomain) error {
	if err := validateNumberLexeme(otherNumber.Maximum); err != nil {
		return err
	}

	merged.Maximum = otherNumber.Maximum
	merged.ExclusiveMaximum = otherNumber.Maximum != nil && otherNumber.ExclusiveMaximum

	return nil
}

// mergeMaximumFromLeft copies and validates the left upper bound.
func mergeMaximumFromLeft(leftNumber *NumberDomain, merged *NumberDomain) error {
	if err := validateNumberLexeme(leftNumber.Maximum); err != nil {
		return err
	}

	merged.Maximum = leftNumber.Maximum
	merged.ExclusiveMaximum = leftNumber.ExclusiveMaximum

	return nil
}

// mergeMaximums selects the numerically smaller upper bound.
func mergeMaximums(leftNumber *NumberDomain, rightNumber *NumberDomain, merged *NumberDomain) error {
	comparison, err := compareNumbers(*leftNumber.Maximum, *rightNumber.Maximum)
	if err != nil {
		return err
	}

	if comparison > 0 {
		merged.Maximum = rightNumber.Maximum
		merged.ExclusiveMaximum = rightNumber.ExclusiveMaximum

		return nil
	}

	merged.Maximum = leftNumber.Maximum

	merged.ExclusiveMaximum = leftNumber.ExclusiveMaximum
	if comparison == 0 {
		merged.ExclusiveMaximum = leftNumber.ExclusiveMaximum || rightNumber.ExclusiveMaximum
	}

	return nil
}

// validateNumberLexeme verifies that an optional Number is parseable.
func validateNumberLexeme(number *Number) error {
	if number == nil {
		return nil
	}

	_, err := compareNumbers(*number, Number("0"))

	return err
}

// mergeMultipleOf intersects the optional divisibility constraints.
func (n *NumberDomain) mergeMultipleOf(otherNumber *NumberDomain, merged *NumberDomain) error {
	switch {
	case n.MultipleOf == nil:
		return mergeMultipleOfFromOther(otherNumber, merged)
	case otherNumber.MultipleOf == nil:
		return mergeMultipleOfFromLeft(n, merged)
	default:
		return mergeMultipleOfs(n, otherNumber, merged)
	}
}

// mergeMultipleOfFromOther copies and validates the right divisor.
func mergeMultipleOfFromOther(otherNumber *NumberDomain, merged *NumberDomain) error {
	if err := validatePositiveMultipleOf(otherNumber.MultipleOf); err != nil {
		return err
	}

	merged.MultipleOf = otherNumber.MultipleOf

	return nil
}

// mergeMultipleOfFromLeft copies and validates the left divisor.
func mergeMultipleOfFromLeft(leftNumber *NumberDomain, merged *NumberDomain) error {
	if err := validatePositiveMultipleOf(leftNumber.MultipleOf); err != nil {
		return err
	}

	merged.MultipleOf = leftNumber.MultipleOf

	return nil
}

// mergeMultipleOfs computes the least common positive rational divisor.
func mergeMultipleOfs(leftNumber *NumberDomain, rightNumber *NumberDomain, merged *NumberDomain) error {
	multipleOf, err := mergeMultipleOf(*leftNumber.MultipleOf, *rightNumber.MultipleOf)
	if err != nil {
		return err
	}

	merged.MultipleOf = &multipleOf

	return nil
}

// validatePositiveMultipleOf verifies that an optional divisor is positive.
func validatePositiveMultipleOf(number *Number) error {
	if number == nil {
		return nil
	}

	value, err := numberToRat(number)
	if err != nil {
		return err
	}

	if value.Sign() <= 0 {
		return errors.New("multipleOf must be positive")
	}

	return nil
}

// mergeFormat retains only a compatible shared format annotation.
func (n *NumberDomain) mergeFormat(otherNumber *NumberDomain, merged *NumberDomain) {
	switch {
	case n.Format == nil:
		merged.Format = otherNumber.Format
	case otherNumber.Format == nil:
		merged.Format = n.Format
	case *n.Format != *otherNumber.Format:
		merged.Format = nil
	default:
		merged.Format = n.Format
	}

	if merged.Type == "integer" && merged.Format != nil && isNumberOnlyFormat(*merged.Format) {
		merged.Format = nil
	}
}

// isNumberOnlyFormat reports whether a standard format excludes integers.
func isNumberOnlyFormat(format string) bool {
	return format == "float" || format == "double"
}

// numberSchema contains the supported numeric Schema Object fields.
type numberSchema struct {
	Type             *string      `json:"type"`
	Nullable         *bool        `json:"nullable"`
	Minimum          *json.Number `json:"minimum"`
	Maximum          *json.Number `json:"maximum"`
	ExclusiveMinimum *bool        `json:"exclusiveMinimum"`
	ExclusiveMaximum *bool        `json:"exclusiveMaximum"`
	MultipleOf       *json.Number `json:"multipleOf"`
	Format           *string      `json:"format"`
}

// ParseNumber parses an OpenAPI number or integer Schema Object.
func (dc *Context) ParseNumber(node *json.RawMessage) (NumberDomain, error) {
	jsonKV, schema, err := parseNumberNode(node)
	if err != nil {
		return NumberDomain{}, err
	}

	schemaType, err := parseNumberType(jsonKV, schema.Type)
	if err != nil {
		return NumberDomain{}, err
	}

	domain := NumberDomain{Type: schemaType}
	if nullableErr := parseNumberNullable(jsonKV, schema.Nullable, &domain); nullableErr != nil {
		return NumberDomain{}, nullableErr
	}

	if enumErr := parseNumberEnums(jsonKV, &domain); enumErr != nil {
		return NumberDomain{}, enumErr
	}

	if boundsErr := parseNumberBounds(jsonKV, schema, &domain); boundsErr != nil {
		return NumberDomain{}, boundsErr
	}

	if exclusiveErr := parseNumberExclusives(jsonKV, schema, &domain); exclusiveErr != nil {
		return NumberDomain{}, exclusiveErr
	}

	if remainingErr := parseRemainingNumberFields(jsonKV, schema, &domain); remainingErr != nil {
		return NumberDomain{}, remainingErr
	}

	if finalizeErr := finalizeNumberDomain(&domain); finalizeErr != nil {
		return NumberDomain{}, finalizeErr
	}

	return domain, nil
}

// parseRemainingNumberFields applies independent divisor, format, and field checks.
func parseRemainingNumberFields(jsonKV JSONKV, schema numberSchema, domain *NumberDomain) error {
	if err := parseNumberMultipleOf(jsonKV, schema, domain); err != nil {
		return err
	}

	if err := parseNumberFormat(jsonKV, schema, domain); err != nil {
		return err
	}

	return validateNumberSchemaFields(jsonKV)
}

// parseNumberNode decodes a numeric Schema Object into keyed and typed forms.
func parseNumberNode(node *json.RawMessage) (JSONKV, numberSchema, error) {
	if node == nil {
		return nil, numberSchema{}, errors.New("schema node is nil")
	}

	jsonKV := JSONKV{}
	if err := json.Unmarshal(*node, &jsonKV); err != nil {
		return nil, numberSchema{}, err
	}

	schema := numberSchema{}
	if err := json.Unmarshal(*node, &schema); err != nil {
		return nil, numberSchema{}, err
	}

	return jsonKV, schema, nil
}

// parseNumberType validates the required numeric type.
func parseNumberType(jsonKV JSONKV, schemaTypeValue *string) (string, error) {
	schemaType, err := requiredSchemaType(jsonKV, schemaTypeValue)
	if err != nil {
		return "", err
	}

	if schemaType != "number" && schemaType != "integer" {
		return "", fmt.Errorf("number domain type must be number or integer, got %q", schemaType)
	}

	return schemaType, nil
}

// parseNumberNullable applies the optional nullable field.
func parseNumberNullable(jsonKV JSONKV, nullable *bool, domain *NumberDomain) error {
	if _, ok := jsonKV["nullable"]; !ok {
		return nil
	}

	if nullable == nil {
		return errors.New("nullable must be boolean")
	}

	domain.Nullable = *nullable

	return nil
}

// parseNumberEnums applies the optional enum constraint.
func parseNumberEnums(jsonKV JSONKV, domain *NumberDomain) error {
	enums, err := parseEnumsByType(jsonKV, domain.Type, domain.Nullable)
	if err != nil {
		return err
	}

	domain.Enum = enums

	return nil
}

// parseNumberBounds applies minimum and maximum without restricting their numeric form.
func parseNumberBounds(jsonKV JSONKV, schema numberSchema, domain *NumberDomain) error {
	if err := parseNumberMinimum(jsonKV, schema.Minimum, domain); err != nil {
		return err
	}

	return parseNumberMaximum(jsonKV, schema.Maximum, domain)
}

// parseNumberMinimum applies the optional minimum constraint.
func parseNumberMinimum(jsonKV JSONKV, minimum *json.Number, domain *NumberDomain) error {
	if _, ok := jsonKV["minimum"]; !ok {
		return nil
	}

	if minimum == nil {
		return errors.New("minimum cannot be null")
	}

	number, _, err := parseSchemaNumber(jsonKV["minimum"], "minimum")
	if err != nil {
		return err
	}

	domain.Minimum = &number

	return nil
}

// parseNumberMaximum applies the optional maximum constraint.
func parseNumberMaximum(jsonKV JSONKV, maximum *json.Number, domain *NumberDomain) error {
	if _, ok := jsonKV["maximum"]; !ok {
		return nil
	}

	if maximum == nil {
		return errors.New("maximum cannot be null")
	}

	number, _, err := parseSchemaNumber(jsonKV["maximum"], "maximum")
	if err != nil {
		return err
	}

	domain.Maximum = &number

	return nil
}

// parseNumberExclusives applies both exclusive bound flags.
func parseNumberExclusives(jsonKV JSONKV, schema numberSchema, domain *NumberDomain) error {
	if err := parseNumberExclusiveMinimum(jsonKV, schema.ExclusiveMinimum, domain); err != nil {
		return err
	}

	return parseNumberExclusiveMaximum(jsonKV, schema.ExclusiveMaximum, domain)
}

// parseNumberExclusiveMinimum applies the exclusive lower-bound flag.
func parseNumberExclusiveMinimum(jsonKV JSONKV, exclusiveMinimum *bool, domain *NumberDomain) error {
	if _, ok := jsonKV["exclusiveMinimum"]; !ok {
		return nil
	}

	if exclusiveMinimum == nil {
		return errors.New("exclusiveMinimum must be boolean")
	}

	domain.ExclusiveMinimum = *exclusiveMinimum

	return nil
}

// parseNumberExclusiveMaximum applies the exclusive upper-bound flag.
func parseNumberExclusiveMaximum(jsonKV JSONKV, exclusiveMaximum *bool, domain *NumberDomain) error {
	if _, ok := jsonKV["exclusiveMaximum"]; !ok {
		return nil
	}

	if exclusiveMaximum == nil {
		return errors.New("exclusiveMaximum must be boolean")
	}

	domain.ExclusiveMaximum = *exclusiveMaximum

	return nil
}

// parseNumberMultipleOf applies the optional positive multipleOf constraint.
func parseNumberMultipleOf(jsonKV JSONKV, schema numberSchema, domain *NumberDomain) error {
	if _, ok := jsonKV["multipleOf"]; !ok {
		return nil
	}

	if schema.MultipleOf == nil {
		return errors.New("multipleOf cannot be null")
	}

	number, rat, err := parseSchemaNumber(jsonKV["multipleOf"], "multipleOf")
	if err != nil {
		return err
	}

	if rat.Sign() <= 0 {
		return errors.New("multipleOf must be positive")
	}

	domain.MultipleOf = &number

	return nil
}

// parseNumberFormat accepts the OpenAPI format field as an open string value.
func parseNumberFormat(jsonKV JSONKV, schema numberSchema, domain *NumberDomain) error {
	if _, ok := jsonKV["format"]; !ok {
		return nil
	}

	if schema.Format == nil {
		return errors.New("format must be string")
	}

	format := *schema.Format
	domain.Format = &format

	return nil
}

// validateNumberSchemaFields rejects unsupported numeric Schema Object fields.
func validateNumberSchemaFields(jsonKV JSONKV) error {
	if err := deleteAllowableKeys(jsonKV); err != nil {
		return err
	}

	supportedKeys := []string{
		"enum",
		"minimum",
		"maximum",
		"exclusiveMinimum",
		"exclusiveMaximum",
		"multipleOf",
		"format",
	}
	for _, key := range supportedKeys {
		delete(jsonKV, key)
	}

	if len(jsonKV) == 0 {
		return nil
	}

	return fmt.Errorf("unsupported number schema field %q", sortedJSONKeys(jsonKV)[0])
}

// parseSchemaNumber parses any valid JSON number used by a numeric constraint.
func parseSchemaNumber(value json.RawMessage, field string) (Number, *big.Rat, error) {
	number := Number(strings.TrimSpace(string(value)))

	rat, err := numberToRat(&number)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", field, err)
	}

	return number, rat, nil
}

// numberToRat parses an optional JSON number lexeme exactly.
//
//nolint:nilnil // A nil Number represents an absent optional constraint.
func numberToRat(number *Number) (*big.Rat, error) {
	if number == nil {
		return nil, nil
	}

	if !json.Valid(*number) {
		return nil, fmt.Errorf("invalid number %q", string(*number))
	}

	rat, ok := new(big.Rat).SetString(string(*number))
	if !ok {
		return nil, fmt.Errorf("invalid number %q", string(*number))
	}

	return rat, nil
}

// numberValueAllowed reports whether value satisfies every numeric constraint.
func numberValueAllowed(domain *NumberDomain, value *big.Rat) (bool, error) {
	if value == nil {
		return false, errors.New("number value cannot be nil")
	}

	if domain.Type == "integer" && !value.IsInt() {
		return false, nil
	}

	allowed, err := numberMeetsMinimum(domain, value)
	if err != nil || !allowed {
		return allowed, err
	}

	allowed, err = numberMeetsMaximum(domain, value)
	if err != nil || !allowed {
		return allowed, err
	}

	return numberMeetsMultipleOf(domain, value)
}

// numberMeetsMinimum reports whether value satisfies the lower bound.
func numberMeetsMinimum(domain *NumberDomain, value *big.Rat) (bool, error) {
	minimum, err := numberToRat(domain.Minimum)
	if err != nil {
		return false, err
	}

	if minimum == nil {
		return true, nil
	}

	comparison := value.Cmp(minimum)

	return comparison > 0 || comparison == 0 && !domain.ExclusiveMinimum, nil
}

// numberMeetsMaximum reports whether value satisfies the upper bound.
func numberMeetsMaximum(domain *NumberDomain, value *big.Rat) (bool, error) {
	maximum, err := numberToRat(domain.Maximum)
	if err != nil {
		return false, err
	}

	if maximum == nil {
		return true, nil
	}

	comparison := value.Cmp(maximum)

	return comparison < 0 || comparison == 0 && !domain.ExclusiveMaximum, nil
}

// numberMeetsMultipleOf reports whether value is an integral multiple of the divisor.
func numberMeetsMultipleOf(domain *NumberDomain, value *big.Rat) (bool, error) {
	multipleOf, err := numberToRat(domain.MultipleOf)
	if err != nil {
		return false, err
	}

	if multipleOf == nil {
		return true, nil
	}

	return new(big.Rat).Quo(value, multipleOf).IsInt(), nil
}

// numberDomainHasNumericValue reports whether the numeric constraints have a solution.
func numberDomainHasNumericValue(domain *NumberDomain) (bool, error) {
	step, discrete, err := numberDomainStep(domain)
	if err != nil {
		return false, err
	}

	minimum, err := numberToRat(domain.Minimum)
	if err != nil {
		return false, err
	}

	maximum, err := numberToRat(domain.Maximum)
	if err != nil {
		return false, err
	}

	if !discrete {
		return denseNumberRangeHasValue(domain, minimum, maximum), nil
	}

	candidate := nearestNumberDomainValue(domain, step, minimum, maximum)

	return numberValueAllowed(domain, candidate)
}

// nearestNumberDomainValue returns the first lattice point nearest an available bound.
func nearestNumberDomainValue(
	domain *NumberDomain,
	step *big.Rat,
	minimum *big.Rat,
	maximum *big.Rat,
) *big.Rat {
	candidate := new(big.Rat)
	if minimum != nil {
		quotient := new(big.Rat).Quo(minimum, step)

		factor := ceilRat(quotient)
		if domain.ExclusiveMinimum && quotient.IsInt() {
			factor.Add(factor, big.NewInt(1))
		}

		candidate.Mul(new(big.Rat).SetInt(factor), step)
	} else if maximum != nil {
		quotient := new(big.Rat).Quo(maximum, step)

		factor := floorRat(quotient)
		if domain.ExclusiveMaximum && quotient.IsInt() {
			factor.Sub(factor, big.NewInt(1))
		}

		candidate.Mul(new(big.Rat).SetInt(factor), step)
	}

	return candidate
}

// denseNumberRangeHasValue reports whether a dense numeric interval is nonempty.
func denseNumberRangeHasValue(domain *NumberDomain, minimum *big.Rat, maximum *big.Rat) bool {
	if minimum == nil || maximum == nil {
		return true
	}

	comparison := minimum.Cmp(maximum)

	return comparison < 0 || comparison == 0 && !domain.ExclusiveMinimum && !domain.ExclusiveMaximum
}

// numberDomainStep returns the exact step of a discrete numeric domain.
func numberDomainStep(domain *NumberDomain) (*big.Rat, bool, error) {
	multipleOf, err := numberToRat(domain.MultipleOf)
	if err != nil {
		return nil, false, err
	}

	if multipleOf != nil && multipleOf.Sign() <= 0 {
		return nil, false, errors.New("multipleOf must be positive")
	}

	if domain.Type == "number" {
		return multipleOf, multipleOf != nil, nil
	}

	if domain.Type != "integer" {
		return nil, false, errors.New("number domain type must be number or integer")
	}

	if multipleOf == nil {
		return big.NewRat(1, 1), true, nil
	}

	integerStep := new(big.Int).Abs(multipleOf.Num())

	return new(big.Rat).SetInt(integerStep), true, nil
}

// floorRat returns the greatest integer no larger than value.
func floorRat(value *big.Rat) *big.Int {
	return new(big.Int).Div(new(big.Int).Set(value.Num()), value.Denom())
}

// ceilRat returns the smallest integer no smaller than value.
func ceilRat(value *big.Rat) *big.Int {
	return new(big.Int).Neg(floorRat(new(big.Rat).Neg(value)))
}

// compareNumbers compares two exact JSON number lexemes.
func compareNumbers(a Number, b Number) (int, error) {
	aRat, err := numberToRat(&a)
	if err != nil {
		return 0, err
	}

	bRat, err := numberToRat(&b)
	if err != nil {
		return 0, err
	}

	return aRat.Cmp(bRat), nil
}

// mergeMultipleOf returns the least common multiple of two positive rationals.
func mergeMultipleOf(left Number, right Number) (Number, error) {
	leftRat, err := numberToRat(&left)
	if err != nil {
		return nil, err
	}

	rightRat, err := numberToRat(&right)
	if err != nil {
		return nil, err
	}

	if leftRat.Sign() <= 0 || rightRat.Sign() <= 0 {
		return nil, errors.New("multipleOf must be positive")
	}

	leftNum := new(big.Int).Abs(leftRat.Num())
	rightNum := new(big.Int).Abs(rightRat.Num())
	leftDen := leftRat.Denom()
	rightDen := rightRat.Denom()

	gcdNum := new(big.Int).GCD(nil, nil, leftNum, rightNum)
	lcmNum := new(big.Int).Div(new(big.Int).Mul(leftNum, rightNum), gcdNum)
	gcdDen := new(big.Int).GCD(nil, nil, leftDen, rightDen)

	mergedRat := new(big.Rat).SetFrac(lcmNum, gcdDen)

	return Number(formatRatDecimal(mergedRat)), nil
}

// formatRatDecimal formats a finite rational as an exact decimal number.
func formatRatDecimal(rat *big.Rat) string {
	const (
		binaryPrime  = 2
		decimalPrime = 5
		decimalRadix = 10
	)

	num := new(big.Int).Set(rat.Num())

	den := new(big.Int).Set(rat.Denom())
	if den.Cmp(big.NewInt(1)) == 0 {
		return num.String()
	}

	two := big.NewInt(binaryPrime)
	five := big.NewInt(decimalPrime)
	scale := 0

	for new(big.Int).Mod(den, two).Sign() == 0 {
		den.Div(den, two)

		scale++
	}

	for new(big.Int).Mod(den, five).Sign() == 0 {
		den.Div(den, five)

		scale++
	}

	if den.Cmp(big.NewInt(1)) != 0 {
		return rat.RatString()
	}

	pow10 := new(big.Int).Exp(big.NewInt(decimalRadix), big.NewInt(int64(scale)), nil)
	scaled := new(big.Int).Mul(num, pow10)
	scaled.Div(scaled, rat.Denom())

	digits := scaled.String()
	if len(digits) <= scale {
		digits = strings.Repeat("0", scale-len(digits)+1) + digits
	}

	point := len(digits) - scale

	return strings.TrimRight(digits[:point]+"."+digits[point:], "0")
}
