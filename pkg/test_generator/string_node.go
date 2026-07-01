package testgenerator

import "encoding/json"

type StringNode struct {
	BaseNode `yaml:",inline"`
	Format   string `yaml:"format"`
}

func (s *StringNode) ValidCases() []Case {
	cases := []Case{
		s.stringCase(),
	}

	return append(cases, s.BaseNode.ValidCases()...)
}

func (s *StringNode) InvalidCases() []Case {
	cases := append([]Case{}, s.BaseNode.InvalidCases()...)
	if s.Format == "date-time" {
		cases = append(cases, Case{Name: "invalid date-time", Value: json.RawMessage(`"not-date-time"`)})
	}

	return append(cases,
		Case{Name: "number", Value: json.RawMessage(`123`)},
		Case{Name: "boolean", Value: json.RawMessage(`true`)},
		Case{Name: "object", Value: json.RawMessage(`{}`)},
		Case{Name: "array", Value: json.RawMessage(`[]`)},
	)
}

func (s *StringNode) Merge(SchemaNode) (SchemaNode, error) {
	panic("TODO implement StringNode.Merge")
}

func (s *StringNode) stringCase() Case {
	if s.Format == "date-time" {
		return Case{
			Name:  "date-time",
			Value: json.RawMessage(`"2026-07-01T12:34:56Z"`),
		}
	}

	return Case{
		Name:  "string",
		Value: json.RawMessage(`"valid-string"`),
	}
}
