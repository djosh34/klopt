package testgenerator

import "encoding/json"

type StringNode struct {
	BaseNode `yaml:",inline"`
}

func (s *StringNode) ValidCases() []Case {
	cases := []Case{
		stringCase(),
	}

	return append(cases, s.BaseNode.ValidCases()...)
}

func (s *StringNode) InvalidCases() []Case {
	return s.BaseNode.InvalidCases()
}

func stringCase() Case {
	return Case{
		GenerateValid: func(valid, invalid map[string]SchemaNode) json.RawMessage {
			return json.RawMessage(`"valid-string"`)
		},
	}
}
