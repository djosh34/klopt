package testgenerator

import "encoding/json"

type StringNode struct {
	BaseNode `yaml:",inline"`
}

func (s *StringNode) GenerateValid() json.RawMessage {
	//TODO implement me
	panic("implement me")
}
