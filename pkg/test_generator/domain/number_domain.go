package domain

import (
	"bytes"
	"decode_and_validate_generator/pkg/test_generator/hashables"
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
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
	if allOfDomain, ok := domain.(*AllOfDomain); ok {
		return allOfDomain.AllOfMerge(n)
	}
	if _, ok := domain.(*NumberDomain); !ok {
		return nil, errors.New("domain is not NumberDomain")
	}

	return nil, errors.New("NOT IMPLEMENTED")
}

func (n *NumberDomain) ToHasher() (types.Hasher, error) {
	if n == nil {
		return nil, errors.New("domain of number cannot be nil")
	}

	return &hashables.NumberHashable{
		Type:             n.Type,
		Nullable:         n.Nullable,
		Enum:             n.Enum,
		Minimum:          toHashableNumberPtr(n.Minimum),
		Maximum:          toHashableNumberPtr(n.Maximum),
		ExclusiveMinimum: n.ExclusiveMinimum,
		ExclusiveMaximum: n.ExclusiveMaximum,
		MultipleOf:       toHashableNumberPtr(n.MultipleOf),
		Format:           n.Format,
	}, nil
}

func (dc *DomainContext) ParseNumber(node *json.RawMessage) (NumberDomain, error) {
	if node == nil {
		return NumberDomain{}, errors.New("schema node is nil")
	}

	decoder := json.NewDecoder(bytes.NewReader(*node))
	decoder.UseNumber()
	jsonKV := JSONKV{}
	if err := decoder.Decode(&jsonKV); err != nil {
		return NumberDomain{}, err
	}

	var raw map[string]any
	decoder = json.NewDecoder(bytes.NewReader(*node))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return NumberDomain{}, err
	}

	domain := NumberDomain{}
	schemaType, err := requiredString(raw, "type")
	if err != nil {
		return NumberDomain{}, err
	}
	if schemaType != "number" && schemaType != "integer" {
		return NumberDomain{}, fmt.Errorf("number domain type must be number or integer, got %q", schemaType)
	}
	domain.Type = schemaType

	if value, ok := raw["nullable"]; ok {
		nullable, ok := value.(bool)
		if !ok {
			return NumberDomain{}, errors.New("nullable must be boolean")
		}
		domain.Nullable = nullable
	}

	if enumRaw, enumOk := jsonKV["enum"]; enumOk {
		var enumValues []json.RawMessage
		if err := json.Unmarshal(enumRaw, &enumValues); err != nil {
			return NumberDomain{}, errors.New("enum must be array")
		}
		if enumValues == nil {
			return NumberDomain{}, errors.New("enum cannot be null")
		}
		if len(enumValues) == 0 {
			return NumberDomain{}, errors.New("enum cannot be empty")
		}
		seen := map[string]struct{}{}
		for _, enumValue := range enumValues {
			var numberValue json.Number
			if err := json.Unmarshal(enumValue, &numberValue); err != nil {
				return NumberDomain{}, err
			}
			number, err := parseSchemaNumber(numberValue, schemaType, "enum")
			if err != nil {
				return NumberDomain{}, err
			}
			key := string(number)
			if _, ok := seen[key]; ok {
				return NumberDomain{}, errors.New("enum values must be unique")
			}
			seen[key] = struct{}{}
			enumDomain := types.Enum(enumValue)
			domain.Enum = append(domain.Enum, enumDomain)
		}
	}

	if value, ok := raw["minimum"]; ok {
		number, err := parseSchemaNumber(value, schemaType, "minimum")
		if err != nil {
			return NumberDomain{}, err
		}
		domain.Minimum = &number
	}
	if value, ok := raw["maximum"]; ok {
		number, err := parseSchemaNumber(value, schemaType, "maximum")
		if err != nil {
			return NumberDomain{}, err
		}
		domain.Maximum = &number
	}
	if value, ok := raw["exclusiveMinimum"]; ok {
		boolValue, ok := value.(bool)
		if !ok {
			return NumberDomain{}, errors.New("exclusiveMinimum must be boolean")
		}
		domain.ExclusiveMinimum = boolValue
	}
	if value, ok := raw["exclusiveMaximum"]; ok {
		boolValue, ok := value.(bool)
		if !ok {
			return NumberDomain{}, errors.New("exclusiveMaximum must be boolean")
		}
		domain.ExclusiveMaximum = boolValue
	}
	if domain.Minimum != nil && domain.Maximum != nil {
		comparison, err := compareNumbers(*domain.Minimum, *domain.Maximum)
		if err != nil {
			return NumberDomain{}, err
		}
		if comparison > 0 || (comparison == 0 && (domain.ExclusiveMinimum || domain.ExclusiveMaximum)) {
			return NumberDomain{}, errors.New("minimum and maximum produce impossible range")
		}
	}

	if value, ok := raw["multipleOf"]; ok {
		number, err := parseSchemaNumber(value, schemaType, "multipleOf")
		if err != nil {
			return NumberDomain{}, err
		}
		comparison, err := compareNumbers(number, Number("0"))
		if err != nil {
			return NumberDomain{}, err
		}
		if comparison <= 0 {
			return NumberDomain{}, errors.New("multipleOf must be positive")
		}
		domain.MultipleOf = &number
	}

	if value, ok := raw["format"]; ok {
		format, ok := value.(string)
		if !ok {
			return NumberDomain{}, errors.New("format must be string")
		}
		if schemaType == "number" && format != "float" && format != "double" {
			return NumberDomain{}, fmt.Errorf("unsupported number format %q", format)
		}
		if schemaType == "integer" && format != "int32" && format != "int64" {
			return NumberDomain{}, fmt.Errorf("unsupported integer format %q", format)
		}
		domain.Format = &format
	}

	deleteAllowableKeys(jsonKV)
	for _, key := range []string{"enum", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf", "format"} {
		delete(jsonKV, key)
	}
	if len(jsonKV) != 0 {
		for key := range jsonKV {
			return NumberDomain{}, fmt.Errorf("unsupported number schema field %q", key)
		}
	}

	return domain, nil
}

func requiredString(raw map[string]any, key string) (string, error) {
	value, ok := raw[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	stringValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be string", key)
	}
	return stringValue, nil
}

func parseSchemaNumber(value any, schemaType string, field string) (Number, error) {
	jsonNumber, ok := value.(json.Number)
	if !ok {
		return nil, fmt.Errorf("%s must be a number", field)
	}
	lexeme := jsonNumber.String()
	if schemaType == "integer" && strings.ContainsAny(lexeme, ".eE") {
		return nil, fmt.Errorf("%s must be an integer", field)
	}
	if _, ok := new(big.Rat).SetString(lexeme); !ok {
		return nil, fmt.Errorf("%s must be a number", field)
	}
	return Number(lexeme), nil
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
