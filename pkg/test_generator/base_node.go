package testgenerator

import "encoding/json"

var _ Caseable = BaseNode{}

func (b BaseNode) ValidCases() []Case {
	if !b.Nullable {
		return nil
	}

	return []Case{nullCase()}
}

func (b BaseNode) InvalidCases() []Case {
	if b.Nullable {
		return nil
	}

	return []Case{nullCase()}
}

func nullCase() Case {
	return Case{
		GenerateValid: func(valid, invalid map[string]SchemaNode) json.RawMessage {
			return json.RawMessage(`null`)
		},
	}
}
