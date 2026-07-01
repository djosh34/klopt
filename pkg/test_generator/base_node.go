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

func (b BaseNode) Merge(SchemaNode) (SchemaNode, error) {
	panic("TODO implement BaseNode.Merge")
}

func nullCase() Case {
	return Case{
		Name:  "null",
		Value: json.RawMessage(`null`),
	}
}
