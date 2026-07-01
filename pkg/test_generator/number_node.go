package testgenerator

import "encoding/json"

var _ Caseable = new(NumberNode)

type NumberNode struct {
	BaseNode `yaml:",inline"`
}

func (n *NumberNode) ValidCases() []Case {
	cases := []Case{
		{Name: "zero", Value: json.RawMessage(`0`)},
		{Name: "positive decimal", Value: json.RawMessage(`123.45`)},
		{Name: "negative decimal", Value: json.RawMessage(`-123.45`)},
		{Name: "int32 max", Value: json.RawMessage(`2147483647`)},
		{Name: "above int32 max", Value: json.RawMessage(`2147483648`)},
		{Name: "int32 min", Value: json.RawMessage(`-2147483648`)},
		{Name: "below int32 min", Value: json.RawMessage(`-2147483649`)},
		{Name: "above float32 max", Value: json.RawMessage(`3.4028236e38`)},
		{Name: "below negative float32 max", Value: json.RawMessage(`-3.4028236e38`)},
		{Name: "below float32 normal min", Value: json.RawMessage(`1e-39`)},
	}

	return append(cases, n.BaseNode.ValidCases()...)
}

func (n *NumberNode) InvalidCases() []Case {
	cases := append([]Case{}, n.BaseNode.InvalidCases()...)
	return append(cases,
		Case{Name: "string", Value: json.RawMessage(`"not-number"`)},
		Case{Name: "numeric string", Value: json.RawMessage(`"123"`)},
		Case{Name: "boolean", Value: json.RawMessage(`true`)},
		Case{Name: "object", Value: json.RawMessage(`{}`)},
		Case{Name: "array", Value: json.RawMessage(`[]`)},
	)
}

func (n *NumberNode) Merge(SchemaNode) (SchemaNode, error) {
	panic("TODO implement NumberNode.Merge")
}
