package types

import (
	"encoding/json"
	"errors"
)

type Pattern []string

func (p *Pattern) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return errors.New("pattern must be string")
	}

	*p = Pattern{value}

	return nil
}

type Format []string

func (f *Format) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return errors.New("format must be string")
	}

	*f = Format{value}

	return nil
}
