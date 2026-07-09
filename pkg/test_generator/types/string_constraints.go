package types

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Pattern contains every regular-expression constraint applied to a string domain.
type Pattern []string

// UnmarshalJSON decodes one OpenAPI pattern string.
func (p *Pattern) UnmarshalJSON(data []byte) error {
	var value *string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("pattern must be string: %w", err)
	}

	if value == nil {
		return errors.New("pattern must be string")
	}

	*p = Pattern{*value}

	return nil
}

// Format contains every format constraint applied to a string domain.
type Format []string

// UnmarshalJSON decodes one OpenAPI format string.
func (f *Format) UnmarshalJSON(data []byte) error {
	var value *string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("format must be string: %w", err)
	}

	if value == nil {
		return errors.New("format must be string")
	}

	*f = Format{*value}

	return nil
}
