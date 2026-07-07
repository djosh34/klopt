package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
)

type StringDomain struct {
	Nullable bool `json:"nullable"`

	Enum []types.Enum `json:"enum"`

	types.Pattern `json:"pattern"`
	types.Format  `json:"format"`

	XValidExamples   []string `json:"x-valid-examples"`
	XInvalidExamples []string `json:"x-invalid-examples"`

	MinLength int  `json:"minLength"`
	MaxLength *int `json:"maxLength"`
}

func (domain *StringDomain) AllOfMerge(otherDomain types.Domain) (types.Domain, error) {
	if domain == nil {
		return nil, errors.New("string domain cannot be nil")
	}

	if allOfDomain, ok := otherDomain.(*AllOfDomain); ok {
		mergedAllOf := &AllOfDomain{}
		if _, err := mergedAllOf.AllOfMerge(domain); err != nil {
			return nil, err
		}

		return mergedAllOf.AllOfMerge(allOfDomain)
	}

	otherString, ok := otherDomain.(*StringDomain)
	if !ok || otherString == nil {
		return nil, errors.New("domain is not StringDomain")
	}

	merged := *domain
	merged.Nullable = domain.Nullable && otherString.Nullable

	enums, err := mergeEnums(domain.Enum, otherString.Enum)
	if err != nil {
		return nil, err
	}

	merged.Enum = enums

	merged.Pattern = append(append(types.Pattern(nil), domain.Pattern...), otherString.Pattern...)
	merged.Format = append(append(types.Format(nil), domain.Format...), otherString.Format...)

	merged.XValidExamples = mergeStringIntersections(domain.XValidExamples, otherString.XValidExamples)
	merged.XInvalidExamples = mergeStringUnion(domain.XInvalidExamples, otherString.XInvalidExamples)

	if merged.Enum != nil && merged.XValidExamples != nil {
		newEnums := make([]types.Enum, 0, len(merged.Enum))

		newExamples := make([]string, 0, len(merged.XValidExamples))
		for _, enumValue := range merged.Enum {
			trimmedEnumValue := strings.TrimSpace(string(enumValue))
			if trimmedEnumValue == "" || trimmedEnumValue[0] != '"' {
				continue
			}

			var stringValue string
			if err := json.Unmarshal(enumValue, &stringValue); err != nil {
				return nil, err
			}

			for _, example := range merged.XValidExamples {
				if stringValue == example {
					newEnums = append(newEnums, enumValue)
					newExamples = append(newExamples, example)

					break
				}
			}
		}

		if len(newEnums) == 0 {
			return nil, errors.New("enum and valid examples intersection is empty")
		}

		merged.Enum = newEnums
		merged.XValidExamples = newExamples
	}

	if otherString.MinLength > merged.MinLength {
		merged.MinLength = otherString.MinLength
	}

	if merged.MaxLength == nil || (otherString.MaxLength != nil && *otherString.MaxLength < *merged.MaxLength) {
		merged.MaxLength = otherString.MaxLength
	}

	return &merged, nil
}

func (domain *StringDomain) ToHasher() (types.Hasher, error) {
	if domain == nil {
		return nil, errors.New("domain of string cannot be nil")
	}

	return &hashables.StringHashable{
		Nullable:         domain.Nullable,
		Enum:             domain.Enum,
		Pattern:          domain.Pattern,
		Format:           domain.Format,
		XValidExamples:   domain.XValidExamples,
		XInvalidExamples: domain.XInvalidExamples,
		MinLength:        domain.MinLength,
		MaxLength:        domain.MaxLength,
	}, nil
}

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

func (dc *DomainContext) ParseString(node *json.RawMessage) (StringDomain, error) {
	if node == nil {
		return StringDomain{}, errors.New("schema node is nil")
	}

	jsonKV := JSONKV{}
	if err := json.Unmarshal(*node, &jsonKV); err != nil {
		return StringDomain{}, err
	}

	schema := stringSchema{}
	if err := json.Unmarshal(*node, &schema); err != nil {
		return StringDomain{}, err
	}

	schemaType, err := requiredSchemaType(jsonKV, schema.Type)
	if err != nil {
		return StringDomain{}, err
	}

	if schemaType != "string" {
		return StringDomain{}, fmt.Errorf("string domain type must be string, got %q", schemaType)
	}

	domain := StringDomain{}

	if _, ok := jsonKV["nullable"]; ok {
		if schema.Nullable == nil {
			return StringDomain{}, errors.New("nullable must be boolean")
		}

		domain.Nullable = *schema.Nullable
	}

	enums, _, err := parseEnums(jsonKV)
	if err != nil {
		return StringDomain{}, err
	}

	domain.Enum = enums

	if _, ok := jsonKV["minLength"]; ok {
		if schema.MinLength == nil {
			return StringDomain{}, errors.New("minLength cannot be null")
		}

		if *schema.MinLength < 0 {
			return StringDomain{}, errors.New("minLength cannot be negative")
		}

		domain.MinLength = *schema.MinLength
	}

	if _, ok := jsonKV["maxLength"]; ok {
		if schema.MaxLength == nil {
			return StringDomain{}, errors.New("maxLength cannot be null")
		}

		if *schema.MaxLength < 0 {
			return StringDomain{}, errors.New("maxLength cannot be negative")
		}

		domain.MaxLength = schema.MaxLength
	}

	if domain.MaxLength != nil && domain.MinLength > *domain.MaxLength {
		return StringDomain{}, errors.New("minLength cannot exceed maxLength")
	}

	if _, ok := jsonKV["pattern"]; ok {
		if schema.Pattern == nil {
			return StringDomain{}, errors.New("pattern must be string")
		}

		domain.Pattern = types.Pattern{*schema.Pattern}
	}

	if _, ok := jsonKV["format"]; ok {
		if schema.Format == nil {
			return StringDomain{}, errors.New("format must be string")
		}

		domain.Format = types.Format{*schema.Format}
	}

	if _, ok := jsonKV["x-valid-examples"]; ok {
		if schema.XValidExamples == nil {
			return StringDomain{}, errors.New("x-valid-examples must be array")
		}

		examples, err := parseStringExamples(schema.XValidExamples, "x-valid-examples")
		if err != nil {
			return StringDomain{}, err
		}

		domain.XValidExamples = examples
	}

	if _, ok := jsonKV["x-invalid-examples"]; ok {
		if schema.XInvalidExamples == nil {
			return StringDomain{}, errors.New("x-invalid-examples must be array")
		}

		examples, err := parseStringExamples(schema.XInvalidExamples, "x-invalid-examples")
		if err != nil {
			return StringDomain{}, err
		}

		domain.XInvalidExamples = examples
	}

	usesExamples := len(domain.Pattern) != 0 || len(domain.Format) != 0
	if usesExamples && (len(domain.XValidExamples) == 0 || len(domain.XInvalidExamples) == 0) {
		return StringDomain{}, errors.New("pattern and format require x-valid-examples and x-invalid-examples")
	}

	if !usesExamples && (len(domain.XValidExamples) != 0 || len(domain.XInvalidExamples) != 0) {
		return StringDomain{}, errors.New("x-valid-examples and x-invalid-examples require pattern or format")
	}

	deleteAllowableKeys(jsonKV)

	for _, key := range []string{"enum", "minLength", "maxLength", "pattern", "format", "x-valid-examples", "x-invalid-examples"} {
		delete(jsonKV, key)
	}

	if len(jsonKV) != 0 {
		for key := range jsonKV {
			return StringDomain{}, fmt.Errorf("unsupported string schema field %q", key)
		}
	}

	return domain, nil
}

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

func parseStringExamples(values []string, field string) ([]string, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("%s cannot be empty", field)
	}

	return append([]string(nil), values...), nil
}
