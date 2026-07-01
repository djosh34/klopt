package testgenerator

import "encoding/json"

var _ Caseable = new(BoolNode)

type BoolNode struct {
	BaseNode `yaml:",inline"`
}

func (b *BoolNode) ValidCases() []Case {
	cases := []Case{
		{Name: "true", Value: json.RawMessage(`true`)},
		{Name: "false", Value: json.RawMessage(`false`)},
	}

	return append(cases, b.BaseNode.ValidCases()...)
}

func (b *BoolNode) InvalidCases() []Case {
	cases := append([]Case{}, b.BaseNode.InvalidCases()...)
	return append(cases,
		Case{Name: "string", Value: json.RawMessage(`"not-boolean"`)},
		Case{Name: "string true", Value: json.RawMessage(`"true"`)},
		Case{Name: "string false", Value: json.RawMessage(`"false"`)},
		Case{Name: "number", Value: json.RawMessage(`123`)},
		Case{Name: "zero", Value: json.RawMessage(`0`)},
		Case{Name: "object", Value: json.RawMessage(`{}`)},
		Case{Name: "array", Value: json.RawMessage(`[]`)},
	)
}

func (b *BoolNode) Merge(SchemaNode) (SchemaNode, error) {
	panic("TODO implement BoolNode.Merge")
}
