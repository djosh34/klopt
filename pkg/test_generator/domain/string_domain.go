package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.
)

// StringDomain describes the values accepted by an OpenAPI string schema.
type StringDomain struct {
	types.Pattern `json:"pattern"`
	types.Format  `json:"format"`

	Nullable bool `json:"nullable"`

	Enum []types.Enum `json:"enum"`

	XValidExamples   []string `json:"x-valid-examples"`
	XInvalidExamples []string `json:"x-invalid-examples"`

	MinLength int  `json:"minLength"`
	MaxLength *int `json:"maxLength"`
}

// AllOfMerge intersects the string domain with another domain.
func (domain *StringDomain) AllOfMerge(otherDomain types.Domain) (types.Domain, error) {
	if domain == nil {
		return nil, errors.New("string domain cannot be nil")
	}

	if allOfDomain, ok := otherDomain.(*AllOfDomain); ok {
		mergedAllOf := &AllOfDomain{Domains: []types.Domain{domain}, MergedDomain: domain}

		return mergedAllOf.AllOfMerge(allOfDomain)
	}

	otherString, ok := otherDomain.(*StringDomain)
	if !ok || otherString == nil {
		nullOnly, merged, err := mergeDomainsAsNullOnly(domain, otherDomain)
		if err != nil {
			return nil, err
		}

		if merged {
			return nullOnly, nil
		}

		return nil, errors.New("domain is not StringDomain")
	}

	merged := *domain
	merged.Nullable = domain.Nullable && otherString.Nullable

	enums, err := mergeEnumsByType(domain.Enum, otherString.Enum, "string", merged.Nullable)
	if err != nil {
		return nil, err
	}

	merged.Enum = enums

	merged.Pattern = append(append(types.Pattern(nil), domain.Pattern...), otherString.Pattern...)
	merged.Format = append(append(types.Format(nil), domain.Format...), otherString.Format...)

	merged.XValidExamples = mergeStringIntersections(domain.XValidExamples, otherString.XValidExamples)
	merged.XInvalidExamples = mergeStringUnion(domain.XInvalidExamples, otherString.XInvalidExamples)

	mergeStringBounds(domain, otherString, &merged)

	if err := finalizeStringDomain(&merged); err != nil {
		return nil, err
	}

	return &merged, nil
}

// mergeStringBounds selects the tighter length limits.
func mergeStringBounds(left *StringDomain, right *StringDomain, merged *StringDomain) {
	if right.MinLength > merged.MinLength {
		merged.MinLength = right.MinLength
	}

	if left.MaxLength == nil || (right.MaxLength != nil && *right.MaxLength < *left.MaxLength) {
		merged.MaxLength = right.MaxLength
	}
}

// intersectStringEnumExamples keeps string enum values present in the valid examples.
func intersectStringEnumExamples(domain *StringDomain) error {
	if domain.Enum == nil || domain.XValidExamples == nil {
		return nil
	}

	enums := make([]types.Enum, 0, len(domain.Enum))
	examples := make([]string, 0, len(domain.XValidExamples))

	for _, enumValue := range domain.Enum {
		if string(enumValue) == "null" {
			enums = append(enums, enumValue)

			continue
		}

		stringValue, err := unmarshalJSONString(enumValue)
		if err != nil {
			return err
		}

		for _, example := range domain.XValidExamples {
			if stringValue == example {
				enums = append(enums, enumValue)
				examples = append(examples, example)

				break
			}
		}
	}

	if len(enums) == 0 {
		return errors.New("enum and valid examples intersection is empty")
	}

	domain.Enum = enums
	domain.XValidExamples = examples

	return nil
}

// unmarshalJSONString decodes a canonical string enum value.
func unmarshalJSONString(value types.Enum) (string, error) {
	var stringValue string
	if err := json.Unmarshal(value, &stringValue); err != nil {
		return "", err
	}

	return stringValue, nil
}

// GenerateHash returns a deterministic hash of the string domain.
func (domain *StringDomain) GenerateHash() (types.Hash, error) {
	if domain == nil {
		return types.Hash{}, errors.New("domain of string cannot be nil")
	}

	value := *domain
	if err := finalizeStringDomain(&value); err != nil {
		return types.Hash{}, err
	}

	return generateHash("string", value)
}

// finalizeStringDomain filters enum values and rejects unsatisfiable length bounds.
func finalizeStringDomain(domain *StringDomain) error {
	if err := validateStringConstraintValues(domain); err != nil {
		return err
	}

	enums, err := filterEnumsByType(domain.Enum, "string", domain.Nullable)
	if err != nil {
		return err
	}

	domain.Enum = enums
	if domain.Enum != nil {
		if filterErr := filterStringEnumsByLength(domain); filterErr != nil {
			return filterErr
		}

		if exampleErr := intersectStringEnumExamples(domain); exampleErr != nil {
			return exampleErr
		}
	}

	return validateStringSatisfiability(domain)
}

// validateStringConstraintValues rejects invalid length keyword values.
func validateStringConstraintValues(domain *StringDomain) error {
	if domain.MinLength < 0 {
		return errors.New("minLength cannot be negative")
	}

	if domain.MaxLength != nil && *domain.MaxLength < 0 {
		return errors.New("maxLength cannot be negative")
	}

	return nil
}

// validateStringSatisfiability permits contradictory bounds only when null remains valid.
func validateStringSatisfiability(domain *StringDomain) error {
	if domain.MaxLength == nil || domain.MinLength <= *domain.MaxLength {
		return nil
	}

	allowsNull, err := enumAllowsNull(domain.Nullable, domain.Enum)
	if err != nil {
		return err
	}

	if allowsNull {
		return nil
	}

	return errors.New("minLength cannot exceed maxLength")
}

// filterStringEnumsByLength retains enum strings satisfying Unicode character-count bounds.
func filterStringEnumsByLength(domain *StringDomain) error {
	filtered := make([]types.Enum, 0, len(domain.Enum))

	for _, enumValue := range domain.Enum {
		if string(enumValue) == "null" {
			filtered = append(filtered, enumValue)

			continue
		}

		stringValue, err := unmarshalJSONString(enumValue)
		if err != nil {
			return err
		}

		length := utf8.RuneCountInString(stringValue)
		if length >= domain.MinLength && (domain.MaxLength == nil || length <= *domain.MaxLength) {
			filtered = append(filtered, enumValue)
		}
	}

	if len(filtered) == 0 {
		return errors.New("enum has no values compatible with string length constraints")
	}

	domain.Enum = filtered

	return nil
}

// stringSchema contains the supported string Schema Object fields.
type stringSchema struct {
	Type             *string  `json:"type"`
	Nullable         *bool    `json:"nullable"`
	MinLength        *int     `json:"minLength"`
	MaxLength        *int     `json:"maxLength"`
	Pattern          *string  `json:"pattern"`
	Format           *string  `json:"format"`
	XValidExamples   []string `json:"x-valid-examples"`
	XInvalidExamples []string `json:"x-invalid-examples"`
}

// ParseString parses an OpenAPI string Schema Object.
func (dc *Context) ParseString(node *json.RawMessage) (StringDomain, error) {
	jsonKV, schema, err := parseStringNode(node)
	if err != nil {
		return StringDomain{}, err
	}

	if typeErr := validateStringType(jsonKV, schema.Type); typeErr != nil {
		return StringDomain{}, typeErr
	}

	domain := StringDomain{}
	if nullableErr := parseStringNullable(jsonKV, schema.Nullable, &domain); nullableErr != nil {
		return StringDomain{}, nullableErr
	}

	enums, err := parseEnumsByType(jsonKV, "string", domain.Nullable)
	if err != nil {
		return StringDomain{}, err
	}

	domain.Enum = enums

	if fieldErr := parseStringDomainFields(jsonKV, schema, &domain); fieldErr != nil {
		return StringDomain{}, fieldErr
	}

	return domain, nil
}

// parseStringDomainFields parses and validates the supported string constraints.
func parseStringDomainFields(jsonKV JSONKV, schema stringSchema, domain *StringDomain) error {
	if err := parseStringLengths(jsonKV, schema, domain); err != nil {
		return err
	}

	if err := parseStringConstraints(jsonKV, schema, domain); err != nil {
		return err
	}

	if err := parseStringExampleFields(jsonKV, schema, domain); err != nil {
		return err
	}

	if err := validateStringExampleUsage(domain); err != nil {
		return err
	}

	if err := validateStringSchemaFields(jsonKV); err != nil {
		return err
	}

	return finalizeStringDomain(domain)
}

// validateStringType checks the required string type declaration.
func validateStringType(jsonKV JSONKV, schemaTypeValue *string) error {
	schemaType, err := requiredSchemaType(jsonKV, schemaTypeValue)
	if err != nil {
		return err
	}

	if schemaType != "string" {
		return fmt.Errorf("string domain type must be string, got %q", schemaType)
	}

	return nil
}

// parseStringNode decodes a string Schema Object into keyed and typed forms.
func parseStringNode(node *json.RawMessage) (JSONKV, stringSchema, error) {
	if node == nil {
		return nil, stringSchema{}, errors.New("schema node is nil")
	}

	jsonKV := JSONKV{}
	if err := json.Unmarshal(*node, &jsonKV); err != nil {
		return nil, stringSchema{}, err
	}

	schema := stringSchema{}
	if err := json.Unmarshal(*node, &schema); err != nil {
		return nil, stringSchema{}, err
	}

	return jsonKV, schema, nil
}

// parseStringNullable applies the optional nullable field.
func parseStringNullable(jsonKV JSONKV, nullable *bool, domain *StringDomain) error {
	if _, ok := jsonKV["nullable"]; ok {
		if nullable == nil {
			return errors.New("nullable must be boolean")
		}

		domain.Nullable = *nullable
	}

	return nil
}

// parseStringLengths applies and validates minLength and maxLength.
func parseStringLengths(jsonKV JSONKV, schema stringSchema, domain *StringDomain) error {
	if _, ok := jsonKV["minLength"]; ok {
		if schema.MinLength == nil {
			return errors.New("minLength cannot be null")
		}

		if *schema.MinLength < 0 {
			return errors.New("minLength cannot be negative")
		}

		domain.MinLength = *schema.MinLength
	}

	if _, ok := jsonKV["maxLength"]; ok {
		if schema.MaxLength == nil {
			return errors.New("maxLength cannot be null")
		}

		if *schema.MaxLength < 0 {
			return errors.New("maxLength cannot be negative")
		}

		domain.MaxLength = schema.MaxLength
	}

	return nil
}

// parseStringConstraints applies the optional pattern and format fields.
func parseStringConstraints(jsonKV JSONKV, schema stringSchema, domain *StringDomain) error {
	if _, ok := jsonKV["pattern"]; ok {
		if schema.Pattern == nil {
			return errors.New("pattern must be string")
		}

		domain.Pattern = types.Pattern{*schema.Pattern}
	}

	if _, ok := jsonKV["format"]; ok {
		if schema.Format == nil {
			return errors.New("format must be string")
		}

		domain.Format = types.Format{*schema.Format}
	}

	return nil
}

// parseStringExampleFields applies the generator-specific example extensions.
func parseStringExampleFields(jsonKV JSONKV, schema stringSchema, domain *StringDomain) error {
	if _, ok := jsonKV["x-valid-examples"]; ok {
		if schema.XValidExamples == nil {
			return errors.New("x-valid-examples must be array")
		}

		examples, err := parseStringExamples(schema.XValidExamples, "x-valid-examples")
		if err != nil {
			return err
		}

		domain.XValidExamples = examples
	}

	if _, ok := jsonKV["x-invalid-examples"]; ok {
		if schema.XInvalidExamples == nil {
			return errors.New("x-invalid-examples must be array")
		}

		examples, err := parseStringExamples(schema.XInvalidExamples, "x-invalid-examples")
		if err != nil {
			return err
		}

		domain.XInvalidExamples = examples
	}

	return nil
}

// validateStringExampleUsage ensures formats and patterns have generation examples.
func validateStringExampleUsage(domain *StringDomain) error {
	usesExamples := len(domain.Pattern) != 0 || len(domain.Format) != 0
	if usesExamples && (len(domain.XValidExamples) == 0 || len(domain.XInvalidExamples) == 0) {
		return errors.New("pattern and format require x-valid-examples and x-invalid-examples")
	}

	if !usesExamples && (len(domain.XValidExamples) != 0 || len(domain.XInvalidExamples) != 0) {
		return errors.New("x-valid-examples and x-invalid-examples require pattern or format")
	}

	return nil
}

// validateStringSchemaFields rejects unsupported string Schema Object fields.
func validateStringSchemaFields(jsonKV JSONKV) error {
	if err := deleteAllowableKeys(jsonKV); err != nil {
		return err
	}

	supportedKeys := []string{
		"enum",
		"minLength",
		"maxLength",
		"pattern",
		"format",
		"x-valid-examples",
		"x-invalid-examples",
	}
	for _, key := range supportedKeys {
		delete(jsonKV, key)
	}

	if len(jsonKV) != 0 {
		keys := sortedJSONKeys(jsonKV)

		return fmt.Errorf("unsupported string schema field %q", keys[0])
	}

	return nil
}

// mergeStringIntersections intersects two optional string lists.
func mergeStringIntersections(left []string, right []string) []string {
	if left == nil && right == nil {
		return nil
	}

	if left == nil {
		return append([]string(nil), right...)
	}

	if right == nil {
		return append([]string(nil), left...)
	}

	merged := make([]string, 0, len(left))
	for _, leftValue := range left {
		for _, rightValue := range right {
			if leftValue == rightValue {
				merged = append(merged, leftValue)

				break
			}
		}
	}

	return merged
}

// mergeStringUnion combines two string lists without duplicate values.
func mergeStringUnion(left []string, right []string) []string {
	if left == nil && right == nil {
		return nil
	}

	merged := append([]string(nil), left...)

	for _, rightValue := range right {
		found := false

		for _, leftValue := range merged {
			if leftValue == rightValue {
				found = true

				break
			}
		}

		if !found {
			merged = append(merged, rightValue)
		}
	}

	return merged
}

// parseStringExamples validates and copies a generator example list.
func parseStringExamples(values []string, field string) ([]string, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("%s cannot be empty", field)
	}

	return append([]string(nil), values...), nil
}
