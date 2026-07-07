package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"decode_and_validate_generator/pkg/test_generator/hashables"
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
		mergedAllOf := &AllOfDomain{}
		if _, err := mergedAllOf.AllOfMerge(n); err != nil {
			return nil, err
		}

		return mergedAllOf.AllOfMerge(allOfDomain)
	}

	otherNumber, ok := domain.(*NumberDomain)
	if !ok || otherNumber == nil {
		return nil, errors.New("domain is not NumberDomain")
	}

	if (n.Type != "number" && n.Type != "integer") || (otherNumber.Type != "number" && otherNumber.Type != "integer") {
		return nil, errors.New("number domain type must be number or integer")
	}

	merged := *n
	if n.Type == "integer" || otherNumber.Type == "integer" {
		merged.Type = "integer"
	} else {
		merged.Type = "number"
	}

	merged.Nullable = n.Nullable && otherNumber.Nullable

	enums, err := mergeEnums(n.Enum, otherNumber.Enum)
	if err != nil {
		return nil, err
	}

	merged.Enum = enums

	if n.Minimum == nil {
		if otherNumber.Minimum != nil {
			if _, err := compareNumbers(*otherNumber.Minimum, *otherNumber.Minimum); err != nil {
				return nil, err
			}
		}

		merged.Minimum = otherNumber.Minimum
		merged.ExclusiveMinimum = otherNumber.Minimum != nil && otherNumber.ExclusiveMinimum
	} else if otherNumber.Minimum == nil {
		if _, err := compareNumbers(*n.Minimum, *n.Minimum); err != nil {
			return nil, err
		}

		merged.Minimum = n.Minimum
		merged.ExclusiveMinimum = n.ExclusiveMinimum
	} else {
		comparison, err := compareNumbers(*n.Minimum, *otherNumber.Minimum)
		if err != nil {
			return nil, err
		}

		if comparison < 0 {
			merged.Minimum = otherNumber.Minimum
			merged.ExclusiveMinimum = otherNumber.ExclusiveMinimum
		} else if comparison == 0 {
			merged.Minimum = n.Minimum
			merged.ExclusiveMinimum = n.ExclusiveMinimum || otherNumber.ExclusiveMinimum
		} else {
			merged.Minimum = n.Minimum
			merged.ExclusiveMinimum = n.ExclusiveMinimum
		}
	}

	if n.Maximum == nil {
		if otherNumber.Maximum != nil {
			if _, err := compareNumbers(*otherNumber.Maximum, *otherNumber.Maximum); err != nil {
				return nil, err
			}
		}

		merged.Maximum = otherNumber.Maximum
		merged.ExclusiveMaximum = otherNumber.Maximum != nil && otherNumber.ExclusiveMaximum
	} else if otherNumber.Maximum == nil {
		if _, err := compareNumbers(*n.Maximum, *n.Maximum); err != nil {
			return nil, err
		}

		merged.Maximum = n.Maximum
		merged.ExclusiveMaximum = n.ExclusiveMaximum
	} else {
		comparison, err := compareNumbers(*n.Maximum, *otherNumber.Maximum)
		if err != nil {
			return nil, err
		}

		if comparison > 0 {
			merged.Maximum = otherNumber.Maximum
			merged.ExclusiveMaximum = otherNumber.ExclusiveMaximum
		} else if comparison == 0 {
			merged.Maximum = n.Maximum
			merged.ExclusiveMaximum = n.ExclusiveMaximum || otherNumber.ExclusiveMaximum
		} else {
			merged.Maximum = n.Maximum
			merged.ExclusiveMaximum = n.ExclusiveMaximum
		}
	}

	if n.MultipleOf == nil {
		if otherNumber.MultipleOf != nil {
			comparison, err := compareNumbers(*otherNumber.MultipleOf, Number("0"))
			if err != nil {
				return nil, err
			}

			if comparison <= 0 {
				return nil, errors.New("multipleOf must be positive")
			}
		}

		merged.MultipleOf = otherNumber.MultipleOf
	} else if otherNumber.MultipleOf == nil {
		comparison, err := compareNumbers(*n.MultipleOf, Number("0"))
		if err != nil {
			return nil, err
		}

		if comparison <= 0 {
			return nil, errors.New("multipleOf must be positive")
		}

		merged.MultipleOf = n.MultipleOf
	} else {
		multipleOf, err := mergeMultipleOf(*n.MultipleOf, *otherNumber.MultipleOf)
		if err != nil {
			return nil, err
		}

		merged.MultipleOf = &multipleOf
	}

	if n.Format == nil {
		merged.Format = otherNumber.Format
	} else if otherNumber.Format == nil {
		merged.Format = n.Format
	} else if *n.Format != *otherNumber.Format {
		return nil, errors.New("number formats cannot be merged")
	} else {
		merged.Format = n.Format
	}

	if merged.Type == "integer" && merged.Format != nil && (*merged.Format == "float" || *merged.Format == "double") {
		return nil, errors.New("integer cannot use number format")
	}

	return &merged, nil
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
	if node == nil {
		return NumberDomain{}, errors.New("schema node is nil")
	}

	jsonKV := JSONKV{}
	if err := json.Unmarshal(*node, &jsonKV); err != nil {
		return NumberDomain{}, err
	}

	schema := numberSchema{}
	if err := json.Unmarshal(*node, &schema); err != nil {
		return NumberDomain{}, err
	}

	domain := NumberDomain{}

	schemaType, err := requiredSchemaType(jsonKV, schema.Type)
	if err != nil {
		return NumberDomain{}, err
	}

	if schemaType != "number" && schemaType != "integer" {
		return NumberDomain{}, fmt.Errorf("number domain type must be number or integer, got %q", schemaType)
	}

	domain.Type = schemaType

	if _, ok := jsonKV["nullable"]; ok {
		if schema.Nullable == nil {
			return NumberDomain{}, errors.New("nullable must be boolean")
		}

		domain.Nullable = *schema.Nullable
	}

	enums, _, err := parseEnums(jsonKV)
	if err != nil {
		return NumberDomain{}, err
	}

	domain.Enum = enums

	if _, ok := jsonKV["minimum"]; ok {
		if schema.Minimum == nil {
			return NumberDomain{}, errors.New("minimum cannot be null")
		}

		number, err := parseSchemaNumber(*schema.Minimum, schemaType, "minimum")
		if err != nil {
			return NumberDomain{}, err
		}

		domain.Minimum = &number
	}

	if _, ok := jsonKV["maximum"]; ok {
		if schema.Maximum == nil {
			return NumberDomain{}, errors.New("maximum cannot be null")
		}

		number, err := parseSchemaNumber(*schema.Maximum, schemaType, "maximum")
		if err != nil {
			return NumberDomain{}, err
		}

		domain.Maximum = &number
	}

	if _, ok := jsonKV["exclusiveMinimum"]; ok {
		if schema.ExclusiveMinimum == nil {
			return NumberDomain{}, errors.New("exclusiveMinimum must be boolean")
		}

		domain.ExclusiveMinimum = *schema.ExclusiveMinimum
	}

	if _, ok := jsonKV["exclusiveMaximum"]; ok {
		if schema.ExclusiveMaximum == nil {
			return NumberDomain{}, errors.New("exclusiveMaximum must be boolean")
		}

		domain.ExclusiveMaximum = *schema.ExclusiveMaximum
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

	if _, ok := jsonKV["multipleOf"]; ok {
		if schema.MultipleOf == nil {
			return NumberDomain{}, errors.New("multipleOf cannot be null")
		}

		number, err := parseSchemaNumber(*schema.MultipleOf, schemaType, "multipleOf")
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

	if _, ok := jsonKV["format"]; ok {
		if schema.Format == nil {
			return NumberDomain{}, errors.New("format must be string")
		}

		format := *schema.Format
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

func parseSchemaNumber(value json.Number, schemaType string, field string) (Number, error) {
	lexeme := value.String()
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
	if scale == 0 {
		return digits
	}

	if len(digits) <= scale {
		digits = strings.Repeat("0", scale-len(digits)+1) + digits
	}

	point := len(digits) - scale

	return strings.TrimRight(digits[:point]+"."+digits[point:], "0")
}
