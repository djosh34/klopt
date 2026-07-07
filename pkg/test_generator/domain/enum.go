package domain

import (
	"decode_and_validate_generator/pkg/test_generator/types"
	"encoding/json"
	"errors"
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
