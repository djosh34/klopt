package domain

import (
	"bytes"
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	if allOfDomain, ok := otherDomain.(*AllOfDomain); ok {
		return allOfDomain.AllOfMerge(domain)
	}
	if _, ok := otherDomain.(*StringDomain); !ok {
		return nil, errors.New("domain is not StringDomain")
	}

	return nil, errors.New("NOT IMPLEMENTED")
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

func (dc *DomainContext) ParseString(node *json.RawMessage) (StringDomain, error) {
	if node == nil {
		return StringDomain{}, errors.New("schema node is nil")
	}

	decoder := json.NewDecoder(bytes.NewReader(*node))
	decoder.UseNumber()
	jsonKV := JSONKV{}
	if err := decoder.Decode(&jsonKV); err != nil {
		return StringDomain{}, err
	}

	var raw map[string]any
	decoder = json.NewDecoder(bytes.NewReader(*node))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return StringDomain{}, err
	}

	schemaType, err := requiredString(raw, "type")
	if err != nil {
		return StringDomain{}, err
	}
	if schemaType != "string" {
		return StringDomain{}, fmt.Errorf("string domain type must be string, got %q", schemaType)
	}

	domain := StringDomain{}
	if value, ok := raw["nullable"]; ok {
		nullable, ok := value.(bool)
		if !ok {
			return StringDomain{}, errors.New("nullable must be boolean")
		}
		domain.Nullable = nullable
	}

	if enumRaw, enumOk := jsonKV["enum"]; enumOk {
		var enumValues []json.RawMessage
		if err := json.Unmarshal(enumRaw, &enumValues); err != nil {
			return StringDomain{}, errors.New("enum must be array")
		}
		if enumValues == nil {
			return StringDomain{}, errors.New("enum cannot be null")
		}
		if len(enumValues) == 0 {
			return StringDomain{}, errors.New("enum cannot be empty")
		}
		seen := map[string]struct{}{}
		for _, enumValue := range enumValues {
			var stringValue string
			if err := json.Unmarshal(enumValue, &stringValue); err != nil {
				return StringDomain{}, errors.New("enum values must be strings")
			}
			key := string(enumValue)
			if _, ok := seen[key]; ok {
				return StringDomain{}, errors.New("enum values must be unique")
			}
			seen[key] = struct{}{}
			enumDomain := types.Enum(enumValue)
			domain.Enum = append(domain.Enum, enumDomain)
		}
	}

	if value, ok := raw["minLength"]; ok {
		minLength, err := parseNonNegativeInteger(value, "minLength")
		if err != nil {
			return StringDomain{}, err
		}
		domain.MinLength = minLength
	}
	if value, ok := raw["maxLength"]; ok {
		maxLength, err := parseNonNegativeInteger(value, "maxLength")
		if err != nil {
			return StringDomain{}, err
		}
		domain.MaxLength = &maxLength
	}
	if domain.MaxLength != nil && domain.MinLength > *domain.MaxLength {
		return StringDomain{}, errors.New("minLength cannot exceed maxLength")
	}

	if value, ok := raw["pattern"]; ok {
		pattern, ok := value.(string)
		if !ok {
			return StringDomain{}, errors.New("pattern must be string")
		}
		domain.Pattern = types.Pattern{pattern}
	}
	if value, ok := raw["format"]; ok {
		format, ok := value.(string)
		if !ok {
			return StringDomain{}, errors.New("format must be string")
		}
		domain.Format = types.Format{format}
	}

	if value, ok := raw["x-valid-examples"]; ok {
		examples, err := parseStringExamples(value, "x-valid-examples")
		if err != nil {
			return StringDomain{}, err
		}
		domain.XValidExamples = examples
	}
	if value, ok := raw["x-invalid-examples"]; ok {
		examples, err := parseStringExamples(value, "x-invalid-examples")
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

func parseStringExamples(value any, field string) ([]string, error) {
	values, ok := value.([]any)
	if !ok || values == nil {
		return nil, fmt.Errorf("%s must be array", field)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("%s cannot be empty", field)
	}
	examples := make([]string, 0, len(values))
	for _, item := range values {
		stringValue, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s values must be strings", field)
		}
		examples = append(examples, stringValue)
	}
	return examples, nil
}

func parseNonNegativeInteger(value any, field string) (int, error) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, fmt.Errorf("%s must be an integer", field)
	}
	if strings.ContainsAny(number.String(), ".eE") {
		return 0, fmt.Errorf("%s must be an integer", field)
	}
	integer, err := number.Int64()
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", field, err)
	}
	if integer < 0 {
		return 0, fmt.Errorf("%s cannot be negative", field)
	}
	return int(integer), nil
}
