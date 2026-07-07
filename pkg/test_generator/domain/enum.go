package domain

import (
	"bytes"
	"encoding/json"
	"errors"

	"decode_and_validate_generator/pkg/test_generator/types"
)

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

	return enums, true, nil
}

func mergeEnums(left []types.Enum, right []types.Enum) ([]types.Enum, error) {
	if left == nil && right == nil {
		return nil, nil
	}

	if err := validateEnums(left); err != nil {
		return nil, err
	}

	if err := validateEnums(right); err != nil {
		return nil, err
	}

	if left == nil {
		return append([]types.Enum(nil), right...), nil
	}

	if right == nil {
		return append([]types.Enum(nil), left...), nil
	}

	merged := make([]types.Enum, 0, len(left))
	for _, leftEnum := range left {
		for _, rightEnum := range right {
			if bytes.Equal(leftEnum, rightEnum) {
				merged = append(merged, leftEnum)

				break
			}
		}
	}

	if len(merged) == 0 {
		return nil, errors.New("enum intersection is empty")
	}

	return merged, nil
}

func validateEnums(enums []types.Enum) error {
	for _, enumValue := range enums {
		if enumValue == nil {
			return errors.New("enum raw value cannot be nil")
		}

		if !json.Valid(enumValue) {
			return errors.New("enum raw value must be valid JSON")
		}
	}

	return nil
}
