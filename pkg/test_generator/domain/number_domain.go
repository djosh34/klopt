package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"decode_and_validate_generator/pkg/test_generator/types"
)

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
		return nil, errors.New("domain is not NumberDomain")
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

	if err := n.mergeFormat(otherNumber, &merged); err != nil {
		return nil, err
	}

	return &merged, nil
}

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

func (n *NumberDomain) mergeNullable(otherNumber *NumberDomain, merged *NumberDomain) {
	merged.Nullable = n.Nullable && otherNumber.Nullable
}

func (n *NumberDomain) mergeEnum(otherNumber *NumberDomain, merged *NumberDomain) error {
	enums, err := mergeEnums(n.Enum, otherNumber.Enum)
	if err != nil {
		return err
	}

	merged.Enum = enums

	return nil
}

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

func mergeMinimumFromOther(otherNumber *NumberDomain, merged *NumberDomain) error {
	if err := validateNumberLexeme(otherNumber.Minimum); err != nil {
		return err
	}

	merged.Minimum = otherNumber.Minimum
	merged.ExclusiveMinimum = otherNumber.Minimum != nil && otherNumber.ExclusiveMinimum

	return nil
}

func mergeMinimumFromLeft(leftNumber *NumberDomain, merged *NumberDomain) error {
	if err := validateNumberLexeme(leftNumber.Minimum); err != nil {
		return err
	}

	merged.Minimum = leftNumber.Minimum
	merged.ExclusiveMinimum = leftNumber.ExclusiveMinimum

	return nil
}

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

func mergeMaximumFromOther(otherNumber *NumberDomain, merged *NumberDomain) error {
	if err := validateNumberLexeme(otherNumber.Maximum); err != nil {
		return err
	}

	merged.Maximum = otherNumber.Maximum
	merged.ExclusiveMaximum = otherNumber.Maximum != nil && otherNumber.ExclusiveMaximum

	return nil
}

func mergeMaximumFromLeft(leftNumber *NumberDomain, merged *NumberDomain) error {
	if err := validateNumberLexeme(leftNumber.Maximum); err != nil {
		return err
	}

	merged.Maximum = leftNumber.Maximum
	merged.ExclusiveMaximum = leftNumber.ExclusiveMaximum

	return nil
}

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

func validateNumberLexeme(number *Number) error {
	if number == nil {
		return nil
	}

	_, err := compareNumbers(*number, Number("0"))

	return err
}

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

func mergeMultipleOfFromOther(otherNumber *NumberDomain, merged *NumberDomain) error {
	if err := validatePositiveMultipleOf(otherNumber.MultipleOf); err != nil {
		return err
	}

	merged.MultipleOf = otherNumber.MultipleOf

	return nil
}

func mergeMultipleOfFromLeft(leftNumber *NumberDomain, merged *NumberDomain) error {
	if err := validatePositiveMultipleOf(leftNumber.MultipleOf); err != nil {
		return err
	}

	merged.MultipleOf = leftNumber.MultipleOf

	return nil
}

func mergeMultipleOfs(leftNumber *NumberDomain, rightNumber *NumberDomain, merged *NumberDomain) error {
	multipleOf, err := mergeMultipleOf(*leftNumber.MultipleOf, *rightNumber.MultipleOf)
	if err != nil {
		return err
	}

	merged.MultipleOf = &multipleOf

	return nil
}

func validatePositiveMultipleOf(number *Number) error {
	if number == nil {
		return nil
	}

	comparison, err := compareNumbers(*number, Number("0"))
	if err != nil {
		return err
	}

	if comparison <= 0 {
		return errors.New("multipleOf must be positive")
	}

	return nil
}

func (n *NumberDomain) mergeFormat(otherNumber *NumberDomain, merged *NumberDomain) error {
	switch {
	case n.Format == nil:
		merged.Format = otherNumber.Format
	case otherNumber.Format == nil:
		merged.Format = n.Format
	case *n.Format != *otherNumber.Format:
		return errors.New("number formats cannot be merged")
	default:
		merged.Format = n.Format
	}

	return validateMergedNumberFormat(merged)
}

func validateMergedNumberFormat(merged *NumberDomain) error {
	if merged.Type == "integer" && merged.Format != nil && (*merged.Format == "float" || *merged.Format == "double") {
		return errors.New("integer cannot use number format")
	}

	return nil
}

func (n *NumberDomain) GenerateHash() (types.Hash, error) {
	if n == nil {
		return types.Hash{}, errors.New("domain of number cannot be nil")
	}

	hashType := n.Type
	if hashType == "" {
		hashType = "number"
	}

	return generateHash(hashType, *n)
}

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

func (dc *DomainContext) ParseNumber(node *json.RawMessage) (NumberDomain, error) {
	jsonKV, schema, err := parseNumberNode(node)
	if err != nil {
		return NumberDomain{}, err
	}

	schemaType, err := parseNumberType(jsonKV, schema.Type)
	if err != nil {
		return NumberDomain{}, err
	}

	domain := NumberDomain{Type: schemaType}
	if err := parseNumberNullable(jsonKV, schema.Nullable, &domain); err != nil {
		return NumberDomain{}, err
	}

	if err := parseNumberEnums(jsonKV, &domain); err != nil {
		return NumberDomain{}, err
	}

	minimumRat, maximumRat, err := parseNumberBounds(jsonKV, schema, schemaType, &domain)
	if err != nil {
		return NumberDomain{}, err
	}

	if err := parseNumberExclusives(jsonKV, schema, &domain); err != nil {
		return NumberDomain{}, err
	}

	if err := validateNumberRange(&domain, minimumRat, maximumRat); err != nil {
		return NumberDomain{}, err
	}

	if err := parseNumberMultipleOf(jsonKV, schema, schemaType, &domain); err != nil {
		return NumberDomain{}, err
	}

	if err := parseNumberFormat(jsonKV, schema, schemaType, &domain); err != nil {
		return NumberDomain{}, err
	}

	if err := validateNumberSchemaFields(jsonKV); err != nil {
		return NumberDomain{}, err
	}

	return domain, nil
}

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

func parseNumberEnums(jsonKV JSONKV, domain *NumberDomain) error {
	enums, _, err := parseEnums(jsonKV)
	if err != nil {
		return err
	}

	domain.Enum = enums

	return nil
}

func parseNumberBounds(jsonKV JSONKV, schema numberSchema, schemaType string, domain *NumberDomain) (*big.Rat, *big.Rat, error) {
	minimumRat, err := parseNumberMinimum(jsonKV, schema.Minimum, schemaType, domain)
	if err != nil {
		return nil, nil, err
	}

	maximumRat, err := parseNumberMaximum(jsonKV, schema.Maximum, schemaType, domain)
	if err != nil {
		return nil, nil, err
	}

	return minimumRat, maximumRat, nil
}

func parseNumberMinimum(jsonKV JSONKV, minimum *json.Number, schemaType string, domain *NumberDomain) (*big.Rat, error) {
	if _, ok := jsonKV["minimum"]; !ok {
		return nil, nil
	}

	if minimum == nil {
		return nil, errors.New("minimum cannot be null")
	}

	number, rat, err := parseSchemaNumber(*minimum, schemaType, "minimum")
	if err != nil {
		return nil, err
	}

	domain.Minimum = &number

	return rat, nil
}

func parseNumberMaximum(jsonKV JSONKV, maximum *json.Number, schemaType string, domain *NumberDomain) (*big.Rat, error) {
	if _, ok := jsonKV["maximum"]; !ok {
		return nil, nil
	}

	if maximum == nil {
		return nil, errors.New("maximum cannot be null")
	}

	number, rat, err := parseSchemaNumber(*maximum, schemaType, "maximum")
	if err != nil {
		return nil, err
	}

	domain.Maximum = &number

	return rat, nil
}

func parseNumberExclusives(jsonKV JSONKV, schema numberSchema, domain *NumberDomain) error {
	if err := parseNumberExclusiveMinimum(jsonKV, schema.ExclusiveMinimum, domain); err != nil {
		return err
	}

	return parseNumberExclusiveMaximum(jsonKV, schema.ExclusiveMaximum, domain)
}

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

func validateNumberRange(domain *NumberDomain, minimumRat *big.Rat, maximumRat *big.Rat) error {
	if domain.Minimum == nil || domain.Maximum == nil {
		return nil
	}

	comparison := minimumRat.Cmp(maximumRat)
	if comparison > 0 || (comparison == 0 && (domain.ExclusiveMinimum || domain.ExclusiveMaximum)) {
		return errors.New("minimum and maximum produce impossible range")
	}

	return nil
}

func parseNumberMultipleOf(jsonKV JSONKV, schema numberSchema, schemaType string, domain *NumberDomain) error {
	if _, ok := jsonKV["multipleOf"]; !ok {
		return nil
	}

	if schema.MultipleOf == nil {
		return errors.New("multipleOf cannot be null")
	}

	number, rat, err := parseSchemaNumber(*schema.MultipleOf, schemaType, "multipleOf")
	if err != nil {
		return err
	}

	if rat.Sign() <= 0 {
		return errors.New("multipleOf must be positive")
	}

	domain.MultipleOf = &number

	return nil
}

func parseNumberFormat(jsonKV JSONKV, schema numberSchema, schemaType string, domain *NumberDomain) error {
	if _, ok := jsonKV["format"]; !ok {
		return nil
	}

	if schema.Format == nil {
		return errors.New("format must be string")
	}

	format := *schema.Format
	if schemaType == "number" && format != "float" && format != "double" {
		return fmt.Errorf("unsupported number format %q", format)
	}

	if schemaType == "integer" && format != "int32" && format != "int64" {
		return fmt.Errorf("unsupported integer format %q", format)
	}

	domain.Format = &format

	return nil
}

func validateNumberSchemaFields(jsonKV JSONKV) error {
	deleteAllowableKeys(jsonKV)

	for _, key := range []string{"enum", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf", "format"} {
		delete(jsonKV, key)
	}

	if len(jsonKV) == 0 {
		return nil
	}

	for key := range jsonKV {
		return fmt.Errorf("unsupported number schema field %q", key)
	}

	return nil
}

func parseSchemaNumber(value json.Number, schemaType string, field string) (Number, *big.Rat, error) {
	lexeme := value.String()
	if schemaType == "integer" && strings.ContainsAny(lexeme, ".eE") {
		return nil, nil, fmt.Errorf("%s must be an integer", field)
	}

	rat, ok := new(big.Rat).SetString(lexeme)
	if !ok {
		return nil, nil, fmt.Errorf("%s must be a number", field)
	}

	return Number(lexeme), rat, nil
}

func compareNumbers(a Number, b Number) (int, error) {
	aRat, ok := new(big.Rat).SetString(string(a))
	if !ok {
		return 0, fmt.Errorf("invalid number %q", string(a))
	}

	bRat, ok := new(big.Rat).SetString(string(b))
	if !ok {
		return 0, fmt.Errorf("invalid number %q", string(b))
	}

	return aRat.Cmp(bRat), nil
}

func mergeMultipleOf(left Number, right Number) (Number, error) {
	leftRat, ok := new(big.Rat).SetString(string(left))
	if !ok {
		return nil, fmt.Errorf("invalid number %q", string(left))
	}

	rightRat, ok := new(big.Rat).SetString(string(right))
	if !ok {
		return nil, fmt.Errorf("invalid number %q", string(right))
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

func formatRatDecimal(rat *big.Rat) string {
	num := new(big.Int).Set(rat.Num())

	den := new(big.Int).Set(rat.Denom())
	if den.Cmp(big.NewInt(1)) == 0 {
		return num.String()
	}

	two := big.NewInt(2)
	five := big.NewInt(5)
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

	pow10 := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	scaled := new(big.Int).Mul(num, pow10)
	scaled.Div(scaled, rat.Denom())

	digits := scaled.String()
	if len(digits) <= scale {
		digits = strings.Repeat("0", scale-len(digits)+1) + digits
	}

	point := len(digits) - scale

	return strings.TrimRight(digits[:point]+"."+digits[point:], "0")
}
