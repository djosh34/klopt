package testgenerator

type ObjectNode struct {
	BaseNode             `yaml:",inline"`
	Required             []string                 `yaml:"required"`
	AdditionalProperties AdditionalPropertiesNode `yaml:"additionalProperties"`
	Properties           map[string]SchemaNode    `yaml:"properties"`
}

func (o *ObjectNode) ValidCases() []Case {
	//TODO implement me
	panic("implement me")
}

type AdditionalPropertiesNode struct {
	Allowed *bool
	Schema  *SchemaNode
}
