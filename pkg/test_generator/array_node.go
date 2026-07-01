package testgenerator

import (
	"encoding/json"
)

var _ Caseable = new(ArrayNode)

type ArrayNode struct {
	BaseNode `yaml:",inline"`
	Items    SchemaNode `yaml:"items"`
}

func (a *ArrayNode) ValidCases() []Case {
	cases := append([]Case{}, a.BaseNode.ValidCases()...)
	cases = append(cases, Case{Name: "empty array", Value: json.RawMessage(`[]`)})

	validCases := a.Items.ValidCases()
	for _, validCase := range validCases {
		cases = append(cases, arrayCase("item "+validCase.Name, validCase.Value))
	}

	if len(validCases) != 0 {
		secondItem := validCases[0].Value
		if len(validCases) > 1 {
			secondItem = validCases[1].Value
		}

		cases = append(cases, arrayCase("multiple items", validCases[0].Value, secondItem))
	}

	return cases
}

func (a *ArrayNode) InvalidCases() []Case {
	cases := append([]Case{}, a.BaseNode.InvalidCases()...)
	cases = append(cases,
		Case{Name: "string", Value: json.RawMessage(`"not-array"`)},
		Case{Name: "number", Value: json.RawMessage(`123`)},
		Case{Name: "boolean", Value: json.RawMessage(`true`)},
		Case{Name: "object", Value: json.RawMessage(`{}`)},
	)

	validCases := a.Items.ValidCases()
	for _, invalidCase := range a.Items.InvalidCases() {
		cases = append(cases, arrayCase("invalid item "+invalidCase.Name, invalidCase.Value))

		if len(validCases) != 0 {
			cases = append(cases, arrayCase("invalid second item "+invalidCase.Name, validCases[0].Value, invalidCase.Value))
		}
	}

	return cases
}

func (a *ArrayNode) Merge(SchemaNode) (SchemaNode, error) {
	panic("TODO implement ArrayNode.Merge")
}

func arrayCase(name string, items ...json.RawMessage) Case {
	value, err := json.Marshal(items)
	if err != nil {
		panic(err)
	}

	return Case{
		Name:  name,
		Value: value,
	}
}
