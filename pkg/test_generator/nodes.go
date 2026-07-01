package testgenerator

var _ Caseable = new(SchemaNode)

type SchemaNode struct {
	Type   string `yaml:"type"`
	Array  *ArrayNode
	Bool   *BoolNode
	Number *NumberNode
	Object *ObjectNode
	String *StringNode
}

func (s *SchemaNode) ValidCases() []Case {
	switch {
	case s.Array != nil:
		return s.Array.ValidCases()
	case s.Bool != nil:
		return s.Bool.ValidCases()
	case s.Number != nil:
		return s.Number.ValidCases()
	case s.Object != nil:
		return s.Object.ValidCases()
	case s.String != nil:
		return s.String.ValidCases()
	default:
		return nil
	}
}

func (s *SchemaNode) InvalidCases() []Case {
	switch {
	case s.Array != nil:
		return s.Array.InvalidCases()
	case s.Bool != nil:
		return s.Bool.InvalidCases()
	case s.Number != nil:
		return s.Number.InvalidCases()
	case s.Object != nil:
		return s.Object.InvalidCases()
	case s.String != nil:
		return s.String.InvalidCases()
	default:
		return nil
	}
}

func (s SchemaNode) Merge(SchemaNode) (SchemaNode, error) {
	panic("TODO implement SchemaNode.Merge")
}

type BaseNode struct {
	Nullable bool `yaml:"nullable"`
}
