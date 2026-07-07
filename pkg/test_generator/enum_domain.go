package testgenerator

import "encoding/json"

var _ Hasher = new(EnumDomain)

// nil == null
type EnumDomain struct {
	Value *json.RawMessage
}

func (e *EnumDomain) GenerateHash() (Hash, error) {
	//TODO implement me
	panic("implement me")
}

func NewEnumFromJSON(node *json.RawMessage) (EnumDomain, error) {
	return EnumDomain{Value: node}, nil
}
