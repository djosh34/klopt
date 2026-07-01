package testgenerator

import (
	"encoding/json"
)

var _ Generatable = new(ObjectNode)

type ObjectNode struct {
	BaseNode             `yaml:",inline"`
	Required             []string                 `yaml:"required"`
	AdditionalProperties AdditionalPropertiesNode `yaml:"additionalProperties"`
	Properties           map[string]SchemaNode    `yaml:"properties"`
}

func (o *ObjectNode) GenerateValid() json.RawMessage {
	//TODO implement me
	panic("implement me")
}

type AdditionalPropertiesNode struct {
	Allowed *bool
	Schema  *SchemaNode
}
